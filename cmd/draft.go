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

	draftGenCmd.Flags().StringVar(&draftChapterFlag, "chapter", "", "Chapter number(s) to generate (e.g., '1', '1-5', or 'chap_1_1_1')")
	draftGenCmd.Flags().StringVar(&draftVolumeFlag, "volume", "", "Volume number for context (e.g., '1')")
	draftGenCmd.Flags().StringVar(&draftPartFlag, "part", "", "Part number for context (e.g., '1')")
	draftGenCmd.Flags().IntVar(&draftWordsFlag, "words", 500, "Target word count for the draft")
	draftGenCmd.Flags().BoolVar(&draftAllFlag, "all", false, "Generate drafts for all chapters")
	draftGenCmd.Flags().IntVar(&draftConcurrencyFlag, "concurrency", 1, "Number of concurrent chapter generations")

	draftReviewCmd.Flags().StringVar(&draftChapterFlag, "chapter", "", "Chapter to review (e.g., '1' or 'chap_1_1_1')")
	draftReviewCmd.Flags().StringVar(&draftVolumeFlag, "volume", "", "Volume to review (e.g., '1')")
	draftReviewCmd.Flags().StringVar(&draftPartFlag, "part", "", "Part to review (e.g., '1')")
	draftReviewCmd.Flags().IntVar(&draftConcurrencyFlag, "concurrency", 1, "Number of concurrent reviews")

	draftImproveCmd.Flags().StringVar(&draftChapterFlag, "chapter", "", "Chapter to improve (e.g., '1' or 'chap_1_1_1')")
	draftImproveCmd.Flags().StringVar(&draftVolumeFlag, "volume", "", "Volume to improve (e.g., '1')")
	draftImproveCmd.Flags().StringVar(&draftPartFlag, "part", "", "Part to improve (e.g., '1')")
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
				stateMatrix := calculateStateMatrix(outline, chapter)

				// Generate draft
				draft, err := agent.GenerateDraft(chapter, &models.StateMatrix{
					Characters:    stateMatrix.Characters,
					Locations:     stateMatrix.Locations,
					Items:         stateMatrix.Items,
					Relationships: stateMatrix.Relationships,
					Storylines:    stateMatrix.Storylines,
					Premises:      stateMatrix.Premises,
				}, draftWordsFlag)
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
	if strings.Contains(chapterFlag, "-") {
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
	var chapters []*models.Chapter
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i := range vol.Chapters {
				chapters = append(chapters, &vol.Chapters[i])
			}
		}
	}
	return chapters
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

// findTargetChapter finds the target chapter based on flags
func findTargetChapter(outline *models.Outline, chapterFlag, volumeFlag, partFlag string) (*models.Chapter, error) {
	if outline == nil || len(outline.Parts) == 0 {
		return nil, fmt.Errorf("outline is empty")
	}

	// If chapter flag contains "chap_", treat it as ID
	if strings.HasPrefix(chapterFlag, "chap_") {
		return findChapterByID(outline, chapterFlag)
	}

	// Parse chapter number
	chapterNum, err := strconv.Atoi(chapterFlag)
	if err != nil {
		return nil, fmt.Errorf("invalid chapter number: %s", chapterFlag)
	}

	// If volume specified, find chapter in that volume
	if volumeFlag != "" {
		volNum, err := strconv.Atoi(volumeFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid volume number: %s", volumeFlag)
		}
		return findChapterInVolume(outline, volNum, chapterNum)
	}

	// Otherwise find by global chapter number
	return findChapterByNumber(outline, chapterNum)
}

// findChapterByID finds a chapter by its ID
func findChapterByID(outline *models.Outline, chapterID string) (*models.Chapter, error) {
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				if ch.ID == chapterID {
					return &ch, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("chapter not found: %s", chapterID)
}

// findChapterInVolume finds a chapter by volume and chapter number
func findChapterInVolume(outline *models.Outline, volNum, chapterNum int) (*models.Chapter, error) {
	volIdx := volNum - 1
	chapterIdx := chapterNum - 1

	for _, part := range outline.Parts {
		if volIdx >= 0 && volIdx < len(part.Volumes) {
			vol := part.Volumes[volIdx]
			if chapterIdx >= 0 && chapterIdx < len(vol.Chapters) {
				return &vol.Chapters[chapterIdx], nil
			}
		}
	}
	return nil, fmt.Errorf("chapter %d in volume %d not found", chapterNum, volNum)
}

// findChapterByNumber finds a chapter by global chapter number
func findChapterByNumber(outline *models.Outline, chapterNum int) (*models.Chapter, error) {
	currentNum := 0
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i := range vol.Chapters {
				currentNum++
				if currentNum == chapterNum {
					return &vol.Chapters[i], nil
				}
			}
		}
	}
	return nil, fmt.Errorf("chapter %d not found", chapterNum)
}

// calculateStateMatrix calculates the story state up to the target chapter
func calculateStateMatrix(outline *models.Outline, targetChapter *models.Chapter) *models.StateMatrix {
	state := &models.StateMatrix{
		Characters:    make(map[string]*models.Character),
		Locations:     make(map[string]*models.Location),
		Items:         make(map[string]*models.Item),
		Relationships: make(map[string]string),
		Storylines:    make(map[string]string),
		Premises:      make(map[string]string),
	}

	// Load all generated elements
	loadElementsIntoState(state)

	// Apply events from all chapters up to target
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				// Stop when we reach target chapter
				if ch.ID == targetChapter.ID {
					return state
				}

				// Apply events from this chapter
				for _, event := range ch.Events {
					applyEvent(state, event)
				}
			}
		}
	}

	return state
}

// applyEvent applies a single event to the state matrix
func applyEvent(state *models.StateMatrix, event models.Event) {
	switch event.Type {
	case "relationship":
		// Format: relationship between char1 and char2 changes
		if len(event.Characters) >= 2 {
			key := event.Characters[0] + "_" + event.Characters[1]
			state.Relationships[key] = event.Change
		}
	case "goal":
		// Character goal update
		if len(event.Characters) > 0 {
			charName := event.Characters[0]
			if char, exists := state.Characters[charName]; exists {
				// Update character's goals based on event
				if event.Change != "" {
					char.Goals = append(char.Goals, event.Change)
				}
			}
		}
	case "item":
		// Character gets or loses item
		if len(event.Characters) > 0 && event.Subject != "" {
			charName := event.Characters[0]
			itemName := event.Subject
			if event.Change == "get" {
				if item, exists := state.Items[itemName]; exists {
					item.Owner = charName
				}
			} else if event.Change == "lost" {
				if item, exists := state.Items[itemName]; exists {
					item.Owner = ""
				}
			}
		}
	case "premise":
		// Character premise/progression update
		if len(event.Characters) > 0 {
			key := event.Characters[0] + "_" + event.Subject
			state.Premises[key] = event.Change
		}
	case "storyline":
		// Storyline progression
		if event.Subject != "" {
			state.Storylines[event.Subject] = event.Change
		}
	}
}

// loadElementsIntoState loads generated elements into state matrix
func loadElementsIntoState(state *models.StateMatrix) {
	root, err := findProjectRoot()
	if err != nil {
		return
	}

	// Load characters
	charPath := filepath.Join(root, "config", "craft", "characters.json")
	if data, err := os.ReadFile(charPath); err == nil {
		var chars map[string]*models.Character
		if err := json.Unmarshal(data, &chars); err == nil {
			for name, char := range chars {
				state.Characters[name] = char
			}
		}
	}

	// Load locations
	locPath := filepath.Join(root, "config", "craft", "locations.json")
	if data, err := os.ReadFile(locPath); err == nil {
		var locs map[string]*models.Location
		if err := json.Unmarshal(data, &locs); err == nil {
			for name, loc := range locs {
				state.Locations[name] = loc
			}
		}
	}

	// Load items
	itemPath := filepath.Join(root, "config", "craft", "items.json")
	if data, err := os.ReadFile(itemPath); err == nil {
		var items map[string]*models.Item
		if err := json.Unmarshal(data, &items); err == nil {
			for name, item := range items {
				state.Items[name] = item
			}
		}
	}
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

// getVolumesForDraft returns volumes based on flags
func getVolumesForDraft(outline *models.Outline, volumeFlag, chapterFlag string) []*models.Volume {
	volumes := make([]*models.Volume, 0)

	if chapterFlag != "" {
		// Find volume containing this chapter
		// Support both chapter ID (e.g., "C1", "C12") and chapter number
		for _, part := range outline.Parts {
			for i := range part.Volumes {
				for _, chapter := range part.Volumes[i].Chapters {
					if chapter.ID == chapterFlag || matchChapterNumber(chapter.ID, chapterFlag) {
						return []*models.Volume{&part.Volumes[i]}
					}
				}
			}
		}
	} else if volumeFlag != "" {
		// Find specific volume
		// Support both volume ID (e.g., "V1", "V2") and volume number (e.g., "1", "2")
		for _, part := range outline.Parts {
			for i := range part.Volumes {
				if part.Volumes[i].ID == volumeFlag || matchVolumeNumber(part.Volumes[i].ID, volumeFlag) {
					return []*models.Volume{&part.Volumes[i]}
				}
			}
		}
	} else {
		// Return all volumes
		for _, part := range outline.Parts {
			for i := range part.Volumes {
				volumes = append(volumes, &part.Volumes[i])
			}
		}
	}

	return volumes
}

// matchVolumeNumber checks if volume ID matches the given number
// e.g., "V1" matches "1", "V12" matches "12"
func matchVolumeNumber(volumeID, number string) bool {
	// Remove "V" prefix and compare
	if len(volumeID) > 1 && volumeID[0] == 'V' {
		return volumeID[1:] == number
	}
	return volumeID == number
}

// matchChapterNumber checks if chapter ID matches the given number
// e.g., "C1" matches "1", "C12" matches "12"
func matchChapterNumber(chapterID, number string) bool {
	// Remove "C" prefix and compare
	if len(chapterID) > 1 && chapterID[0] == 'C' {
		return chapterID[1:] == number
	}
	return chapterID == number
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

	reviewDir := filepath.Join(root, "config", "reviews")
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

	reviewPath := filepath.Join(root, "config", "reviews", volumeID+"_review.json")
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
				stateMatrix := calculateStateMatrix(outline, chapter)

				// Generate improved draft with suggestions
				draft, err := agent.GenerateDraftWithSuggestions(chapter, &models.StateMatrix{
					Characters:    stateMatrix.Characters,
					Locations:     stateMatrix.Locations,
					Items:         stateMatrix.Items,
					Relationships: stateMatrix.Relationships,
					Storylines:    stateMatrix.Storylines,
					Premises:      stateMatrix.Premises,
				}, 500, suggestions)

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
