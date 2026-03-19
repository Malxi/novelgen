package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"nolvegen/internal/agents"
	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/logic"
	"nolvegen/internal/models"

	"github.com/spf13/cobra"
)

var (
	draftChapterFlag     string
	draftVolumeFlag      string
	draftPartFlag        string
	draftWordsFlag       int
	draftAllFlag         bool
	draftConcurrencyFlag int
	draftMaxRoundsFlag   int
	draftMinScoreFlag    int
)

var draftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Generate and improve draft chapters",
	Long: `Generate, review, and improve draft chapters based on outline and story state.

Commands:
  gen      - Generate new drafts
  review   - Review drafts and provide feedback
  improve  - Improve drafts based on review feedback`,
}

var draftGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate draft chapter(s)",
	Long: `Generate draft chapter(s) based on outline and story state.

Examples:
  # Generate draft for chapter 1
  novel draft gen --chapter 1

  # Generate draft for chapters 1 to 5
  novel draft gen --chapter 1-5

  # Generate draft for all chapters
  novel draft gen --all

  # Generate draft for specific chapter by ID
  novel draft gen --chapter chap_1_1_1

  # Generate with custom word count
  novel draft gen --chapter 1 --words 800

  # Generate for first chapter of volume 2
  novel draft gen --volume 2 --chapter 1`,
	RunE: runDraftGen,
}

var draftReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review drafts and provide feedback",
	Long: `Review drafts and generate detailed feedback for improvement.

Examples:
  # Review all drafts in volume 1
  novel draft review --volume 1

  # Review specific chapter
  novel draft review --chapter 1

  # Review with concurrency
  novel draft review --volume 1 --concurrency 3`,
	RunE: runDraftReview,
}

var draftImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve drafts based on review feedback",
	Long: `Improve drafts by regenerating chapters that need revision.

Examples:
  # Improve all chapters in volume 1
  novel draft improve --volume 1

  # Improve with max 3 rounds
  novel draft improve --volume 1 --max-rounds 3

  # Only improve chapters with score below 7
  novel draft improve --volume 1 --min-score 7`,
	RunE: runDraftImprove,
}

func init() {
	draftCmd.AddCommand(draftGenCmd)
	draftCmd.AddCommand(draftReviewCmd)
	draftCmd.AddCommand(draftImproveCmd)

	draftGenCmd.Flags().StringVar(&draftChapterFlag, "chapter", "", "Chapter number(s) to generate (e.g., '1', '1-5', or 'P1-V1-C1')")
	draftGenCmd.Flags().StringVar(&draftVolumeFlag, "volume", "", "Volume number for context (e.g., '1', 'P1-V1')")
	draftGenCmd.Flags().StringVar(&draftPartFlag, "part", "", "Part number for context (e.g., '1', 'P1')")
	draftGenCmd.Flags().IntVar(&draftWordsFlag, "words", 500, "Target word count for the draft")
	draftGenCmd.Flags().BoolVar(&draftAllFlag, "all", false, "Generate drafts for all chapters")
	draftGenCmd.Flags().IntVar(&draftConcurrencyFlag, "concurrency", 1, "Number of concurrent chapter generations")

	draftReviewCmd.Flags().StringVar(&draftChapterFlag, "chapter", "", "Chapter to review (e.g., '1' or 'P1-V1-C1')")
	draftReviewCmd.Flags().StringVar(&draftVolumeFlag, "volume", "", "Volume to review (e.g., '1', 'P1-V1')")
	draftReviewCmd.Flags().StringVar(&draftPartFlag, "part", "", "Part to review (e.g., '1', 'P1')")
	draftReviewCmd.Flags().IntVar(&draftConcurrencyFlag, "concurrency", 1, "Number of concurrent reviews")

	draftImproveCmd.Flags().StringVar(&draftChapterFlag, "chapter", "", "Chapter to improve (e.g., '1' or 'P1-V1-C1')")
	draftImproveCmd.Flags().StringVar(&draftVolumeFlag, "volume", "", "Volume to improve (e.g., '1', 'P1-V1')")
	draftImproveCmd.Flags().StringVar(&draftPartFlag, "part", "", "Part to improve (e.g., '1', 'P1')")
	draftImproveCmd.Flags().IntVar(&draftMaxRoundsFlag, "max-rounds", 1, "Maximum improvement rounds")
	draftImproveCmd.Flags().IntVar(&draftMinScoreFlag, "min-score", 7, "Minimum acceptable score (1-10)")
	draftImproveCmd.Flags().IntVar(&draftConcurrencyFlag, "concurrency", 1, "Number of concurrent improvements")

	rootCmd.AddCommand(draftCmd)
}

func runDraftGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Create draft agent
	agent := agents.NewDraftAgent(client, cfg, &config.LLM, setup, outline, config.Language)

	// Get project root for state matrix manager
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Create state matrix manager
	stateManager := logic.NewStateMatrixManager(root)

	// Get list of chapters to generate
	chapters, err := getChaptersToGenerate(outline, draftChapterFlag, draftVolumeFlag, draftPartFlag, draftAllFlag)
	if err != nil {
		return err
	}

	log.Info("Generating drafts for %d chapter(s) with concurrency %d", len(chapters), draftConcurrencyFlag)

	// Use worker pool for concurrent generation
	concurrency := draftConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	// Create work channel and wait group
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				log.Info("[Worker %d] Generating draft for chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Calculate story state matrix
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Generate draft
				draft, err := agent.GenerateDraft(chapter, stateMatrix, draftWordsFlag)
				if err != nil {
					log.Error("Failed to generate draft for chapter %s: %v", chapter.ID, err)
					continue
				}

				// Save draft
				if err := saveDraft(chapter, draft); err != nil {
					log.Error("Failed to save draft for chapter %s: %v", chapter.ID, err)
					continue
				}

				log.Info("[Worker %d] Draft saved for chapter %s: %d words", workerID, chapter.ID, len(strings.Fields(draft)))
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chapters {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

	log.Info("Draft generation complete")
	return nil
}

// findTargetChapter finds the target chapter based on flags
// getChaptersToGenerate returns list of chapters to generate based on flags
func getChaptersToGenerate(outline *models.Outline, chapterFlag, volumeFlag, partFlag string, allFlag bool) ([]*models.Chapter, error) {
	if outline == nil || len(outline.Parts) == 0 {
		return nil, fmt.Errorf("outline is empty")
	}

	// If --all flag, return all chapters
	if allFlag {
		return getAllChapters(outline), nil
	}

	// If no chapter flag, error
	if chapterFlag == "" {
		return nil, fmt.Errorf("please specify --chapter or --all")
	}

	// Check if it's a range (e.g., "1-5")
	if strings.Contains(chapterFlag, "-") && !strings.Contains(strings.ToUpper(chapterFlag), "P") {
		parts := strings.Split(chapterFlag, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				return getChapterRange(outline, start, end)
			}
		}
	}

	// Single chapter
	chapter, err := findTargetChapter(outline, chapterFlag, volumeFlag, partFlag)
	if err != nil {
		return nil, err
	}
	return []*models.Chapter{chapter}, nil
}

// getAllChapters returns all chapters in order
func getAllChapters(outline *models.Outline) []*models.Chapter {
	idManager := logic.NewIDManager(outline)
	return idManager.GetAllChapters()
}

// getChapterRange returns chapters from start to end (inclusive)
func getChapterRange(outline *models.Outline, start, end int) ([]*models.Chapter, error) {
	allChapters := getAllChapters(outline)

	if start < 1 || start > len(allChapters) {
		return nil, fmt.Errorf("start chapter %d out of range (1-%d)", start, len(allChapters))
	}
	if end < 1 || end > len(allChapters) {
		return nil, fmt.Errorf("end chapter %d out of range (1-%d)", end, len(allChapters))
	}
	if start > end {
		return nil, fmt.Errorf("start chapter %d must be <= end chapter %d", start, end)
	}

	return allChapters[start-1 : end], nil
}

// findTargetChapter finds the target chapter based on flags using IDManager
func findTargetChapter(outline *models.Outline, chapterFlag, volumeFlag, partFlag string) (*models.Chapter, error) {
	if outline == nil || len(outline.Parts) == 0 {
		return nil, fmt.Errorf("outline is empty")
	}

	idManager := logic.NewIDManager(outline)

	// Resolve chapter ID using IDManager
	chapterID, err := idManager.ResolveChapterID(chapterFlag, partFlag, volumeFlag)
	if err != nil {
		return nil, err
	}

	// Find chapter by ID
	chapter, _, _ := idManager.GetChapterByID(chapterID)
	if chapter == nil {
		return nil, fmt.Errorf("chapter not found: %s", chapterID)
	}

	return chapter, nil
}

// saveDraft saves the generated draft to file
func saveDraft(chapter *models.Chapter, draft string) error {
	draftsDir := filepath.Join("drafts")
	if err := os.MkdirAll(draftsDir, 0755); err != nil {
		return fmt.Errorf("failed to create drafts directory: %w", err)
	}

	filename := filepath.Join(draftsDir, chapter.ID+".md")
	content := fmt.Sprintf("# %s\n\n%s\n\n%s\n", chapter.Title, chapter.Summary, draft)

	return os.WriteFile(filename, []byte(content), 0644)
}

// runDraftReview reviews drafts and provides feedback
func runDraftReview(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load drafts
	drafts := loadAllDrafts()
	if len(drafts) == 0 {
		return fmt.Errorf("no drafts found. Run 'novel draft gen' first")
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Create review agent
	agent := agents.NewReviewAgent(client, cfg, &config.LLM, setup, outline, config.Language)

	// Get volumes to review
	volumes := getVolumesForDraft(outline, draftVolumeFlag, draftChapterFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to review")
	}

	// Review each volume
	for _, volume := range volumes {
		log.Info("Reviewing volume: %s - %s", volume.ID, volume.Title)

		// Filter drafts for this volume
		volumeDrafts := filterDraftsForVolume(drafts, volume)

		review, err := agent.ReviewVolume(volume, volumeDrafts)
		if err != nil {
			log.Error("Failed to review volume %s: %v", volume.ID, err)
			continue
		}

		// Save review
		if err := saveVolumeReview(review); err != nil {
			log.Error("Failed to save review: %v", err)
			continue
		}

		log.Info("Review saved for volume %s", volume.ID)
		log.Info("Summary: %s", review.Summary)

		// Print chapters that need revision
		needsRevision := 0
		for _, r := range review.Reviews {
			if r.NeedsRevision {
				needsRevision++
				log.Info("Chapter %s needs revision (score: %d)", r.ChapterID, r.OverallScore)
			}
		}
		log.Info("Total chapters needing revision: %d", needsRevision)
	}

	return nil
}

// runDraftImprove improves drafts based on review feedback
func runDraftImprove(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Create draft agent
	draftAgent := agents.NewDraftAgent(client, cfg, &config.LLM, setup, outline, config.Language)

	// Get volumes to improve
	volumes := getVolumesForDraft(outline, draftVolumeFlag, draftChapterFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to improve")
	}

	// Run improvement rounds
	for round := 1; round <= draftMaxRoundsFlag; round++ {
		log.Info("=== Improvement Round %d/%d ===", round, draftMaxRoundsFlag)

		improvedCount := 0

		for _, volume := range volumes {
			// Load review for this volume
			review, err := loadVolumeReview(volume.ID)
			if err != nil {
				log.Warn("No review found for volume %s, skipping", volume.ID)
				continue
			}

			// Get chapters that need improvement
			chaptersToImprove := getChaptersNeedingImprovement(review, outline, draftMinScoreFlag)
			if len(chaptersToImprove) == 0 {
				log.Info("Volume %s: All chapters meet quality threshold", volume.ID)
				continue
			}

			log.Info("Volume %s: Improving %d chapters", volume.ID, len(chaptersToImprove))

			// Improve chapters concurrently
			improved := improveChaptersWithAgent(draftAgent, chaptersToImprove, review.Reviews, outline, draftConcurrencyFlag)
			improvedCount += improved
		}

		log.Info("Round %d complete: %d chapters improved", round, improvedCount)

		if improvedCount == 0 {
			log.Info("No more chapters need improvement")
			break
		}

		// Re-review after improvement (if not last round)
		if round < draftMaxRoundsFlag {
			log.Info("Re-reviewing after improvements...")
			if err := runDraftReview(cmd, args); err != nil {
				log.Error("Re-review failed: %v", err)
			}
		}
	}

	log.Info("Improvement process complete")
	return nil
}

// getVolumesForDraft returns volumes based on flags using IDManager
func getVolumesForDraft(outline *models.Outline, volumeFlag, chapterFlag string) []*models.Volume {
	idManager := logic.NewIDManager(outline)

	if chapterFlag != "" {
		// Find volume containing this chapter
		chapterID, err := idManager.ResolveChapterID(chapterFlag, "", "")
		if err == nil {
			_, vol, _ := idManager.GetChapterByID(chapterID)
			if vol != nil {
				return []*models.Volume{vol}
			}
		}
	} else if volumeFlag != "" {
		// Find specific volume
		volumeID, err := idManager.ResolveVolumeID(volumeFlag, "")
		if err == nil {
			vol, _ := idManager.GetVolumeByID(volumeID)
			if vol != nil {
				return []*models.Volume{vol}
			}
		}
	}

	// Return all volumes
	return idManager.GetAllVolumes()
}

// loadAllDrafts loads all draft files
func loadAllDrafts() map[string]string {
	drafts := make(map[string]string)

	root, err := findProjectRoot()
	if err != nil {
		return drafts
	}

	draftsDir := filepath.Join(root, "drafts")
	entries, err := os.ReadDir(draftsDir)
	if err != nil {
		return drafts
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		chapterID := strings.TrimSuffix(entry.Name(), ".md")
		data, err := os.ReadFile(filepath.Join(draftsDir, entry.Name()))
		if err == nil {
			drafts[chapterID] = string(data)
		}
	}

	return drafts
}

// filterDraftsForVolume filters drafts for a specific volume
func filterDraftsForVolume(drafts map[string]string, volume *models.Volume) map[string]string {
	result := make(map[string]string)
	for _, chapter := range volume.Chapters {
		if draft, exists := drafts[chapter.ID]; exists {
			result[chapter.ID] = draft
		}
	}
	return result
}

// saveVolumeReview saves a volume review to file
func saveVolumeReview(review *agents.VolumeReview) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	reviewDir := filepath.Join(root, "story", "reviews")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		return fmt.Errorf("failed to create reviews directory: %w", err)
	}

	reviewPath := filepath.Join(reviewDir, review.VolumeID+"_review.json")
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal review: %w", err)
	}

	return os.WriteFile(reviewPath, data, 0644)
}

// loadVolumeReview loads a volume review from file
func loadVolumeReview(volumeID string) (*agents.VolumeReview, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, err
	}

	reviewPath := filepath.Join(root, "story", "reviews", volumeID+"_review.json")
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return nil, err
	}

	var review agents.VolumeReview
	if err := json.Unmarshal(data, &review); err != nil {
		return nil, err
	}

	return &review, nil
}

// getChaptersNeedingImprovement returns chapters that need improvement
func getChaptersNeedingImprovement(review *agents.VolumeReview, outline *models.Outline, minScore int) []*models.Chapter {
	chapters := make([]*models.Chapter, 0)

	for _, r := range review.Reviews {
		if r.NeedsRevision || r.OverallScore < minScore {
			chapter := outline.GetChapterByID(r.ChapterID)
			if chapter != nil {
				chapters = append(chapters, chapter)
			}
		}
	}

	return chapters
}

// improveChaptersWithAgent improves chapters using the draft agent
func improveChaptersWithAgent(agent *agents.DraftAgent, chapters []*models.Chapter, reviews []agents.DraftReview, outline *models.Outline, concurrency int) int {
	log := logger.GetLogger()

	// Get project root for state matrix manager
	root, err := findProjectRoot()
	if err != nil {
		log.Error("Failed to find project root: %v", err)
		return 0
	}
	stateManager := logic.NewStateMatrixManager(root)

	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	// Create review map for quick lookup
	reviewMap := make(map[string]*agents.DraftReview)
	for i := range reviews {
		reviewMap[reviews[i].ChapterID] = &reviews[i]
	}

	// Create work channel and wait group
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup
	improvedCount := 0
	var mu sync.Mutex

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				review := reviewMap[chapter.ID]
				if review == nil {
					continue
				}

				log.Info("[Worker %d] Improving chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Build improvement prompt
				suggestions := buildImprovementSuggestions(review)

				// Calculate state matrix
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Generate improved draft with suggestions
				draft, err := agent.GenerateDraftWithSuggestions(chapter, stateMatrix, 500, suggestions)

				if err != nil {
					log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				// Save improved draft
				if err := saveDraft(chapter, draft); err != nil {
					log.Error("[Worker %d] Failed to save improved draft for chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				mu.Lock()
				improvedCount++
				mu.Unlock()

				log.Info("[Worker %d] Improved draft saved for chapter %s: %d words", workerID, chapter.ID, len(strings.Fields(draft)))
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chapters {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

	return improvedCount
}

// buildImprovementSuggestions builds improvement suggestions from review
func buildImprovementSuggestions(review *agents.DraftReview) string {
	var sb strings.Builder

	sb.WriteString("## 改进建议\n\n")

	if len(review.PlotCoherence.Suggestions) > 0 {
		sb.WriteString("### 剧情连贯性\n")
		for _, s := range review.PlotCoherence.Suggestions {
			sb.WriteString("- " + s + "\n")
		}
		sb.WriteString("\n")
	}

	if len(review.PlotRationality.Suggestions) > 0 {
		sb.WriteString("### 情节合理性\n")
		for _, s := range review.PlotRationality.Suggestions {
			sb.WriteString("- " + s + "\n")
		}
		sb.WriteString("\n")
	}

	if len(review.CharacterConsistency.Suggestions) > 0 {
		sb.WriteString("### 角色一致性\n")
		for _, s := range review.CharacterConsistency.Suggestions {
			sb.WriteString("- " + s + "\n")
		}
		sb.WriteString("\n")
	}

	if len(review.PacingReview.Suggestions) > 0 {
		sb.WriteString("### 节奏把控\n")
		for _, s := range review.PacingReview.Suggestions {
			sb.WriteString("- " + s + "\n")
		}
		sb.WriteString("\n")
	}

	if len(review.Suggestions) > 0 {
		sb.WriteString("### 总体建议\n")
		for _, s := range review.Suggestions {
			sb.WriteString("- " + s + "\n")
		}
	}

	return sb.String()
}
