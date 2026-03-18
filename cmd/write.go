package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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
	writeChapterFlag     string
	writeVolumeFlag      string
	writePartFlag        string
	writeWordsFlag       int
	writeAllFlag         bool
	writeContextFlag     int
	writeConcurrencyFlag int
	writeMaxRoundsFlag   int
	writeMinScoreFlag    int
)

var writeCmd = &cobra.Command{
	Use:   "write",
	Short: "Generate final chapter content",
	Long: `Generate final chapter content based on drafts with context continuity.

The write command reads draft chapters and generates polished final content,
ensuring continuity with surrounding chapters by including them as context.`,
}

var writeGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate final chapter content",
	Long: `Generate final chapter content with continuity from surrounding drafts.

Examples:
  # Generate final content for chapter 1
  novel write gen --chapter 1

  # Generate final content for chapters 1 to 5
  novel write gen --chapter 1-5

  # Generate final content for all chapters
  novel write gen --all

  # Generate with custom word count
  novel write gen --chapter 1 --words 2000

  # Generate with 3 chapters of context on each side
  novel write gen --chapter 5 --context 3`,
	RunE: runWriteGen,
}

var writeImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve final chapters based on review",
	Long: `Improve final chapters by reviewing and regenerating content that doesn't meet quality standards.

This command will:
1. Load existing reviews for the specified chapters/volumes
2. Identify chapters that need improvement (below min-score)
3. Regenerate those chapters with improvement suggestions
4. Repeat for the specified number of rounds

Examples:
  # Improve all chapters in volume 1
  novel write improve --volume 1

  # Improve with max 3 rounds
  novel write improve --volume 1 --max-rounds 3

  # Only improve chapters with score below 7
  novel write improve --volume 1 --min-score 7`,
	RunE: runWriteImprove,
}

func init() {
	writeCmd.AddCommand(writeGenCmd)
	writeCmd.AddCommand(writeImproveCmd)

	writeGenCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter number(s) to generate (e.g., '1', '1-5', or 'P1-V1-C1')")
	writeGenCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume number for context (e.g., '1', 'P1-V1')")
	writeGenCmd.Flags().StringVar(&writePartFlag, "part", "", "Part number for context (e.g., '1', 'P1')")
	writeGenCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	writeGenCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Generate content for all chapters")
	writeGenCmd.Flags().IntVar(&writeContextFlag, "context", 2, "Number of surrounding chapters to include as context")
	writeGenCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent chapter generations")

	writeImproveCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to improve (e.g., '1' or 'P1-V1-C1')")
	writeImproveCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to improve (e.g., '1', 'P1-V1')")
	writeImproveCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to improve (e.g., '1', 'P1')")
	writeImproveCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 1, "Maximum improvement rounds")
	writeImproveCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 7, "Minimum acceptable score (1-10)")
	writeImproveCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent improvements")

	rootCmd.AddCommand(writeCmd)
}

func runWriteGen(cmd *cobra.Command, args []string) error {
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

	// Create write agent
	agent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline, config.Language)

	// Get project root for state matrix manager
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Create state matrix manager
	stateManager := logic.NewStateMatrixManager(root)

	// Get list of chapters to generate
	chapters, err := getChaptersToGenerate(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if err != nil {
		return err
	}

	log.Info("Generating final content for %d chapter(s) with concurrency %d", len(chapters), writeConcurrencyFlag)

	// Use worker pool for concurrent generation
	concurrency := writeConcurrencyFlag
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
				log.Info("[Worker %d] Generating content for chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Load context drafts (previous and next chapters)
				context := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate story state matrix
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Generate final content
				content, err := agent.GenerateChapter(chapter, context, stateMatrix, writeWordsFlag)
				if err != nil {
					log.Error("Failed to generate content for chapter %s: %v", chapter.ID, err)
					continue
				}

				// Save final content
				if err := saveFinalChapter(chapter, content); err != nil {
					log.Error("Failed to save content for chapter %s: %v", chapter.ID, err)
					continue
				}

				log.Info("[Worker %d] Content saved for chapter %s: %d words", workerID, chapter.ID, len(strings.Fields(content)))
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

	log.Info("Chapter writing complete")
	return nil
}

// loadChapterContext loads surrounding chapter drafts for context
func loadChapterContext(outline *models.Outline, targetChapter *models.Chapter, contextCount int) *agents.ChapterContext {
	context := &agents.ChapterContext{
		Current:  targetChapter,
		Previous: make([]*agents.ContextChapter, 0),
		Next:     make([]*agents.ContextChapter, 0),
	}

	allChapters := getAllChapters(outline)

	// Find target chapter index
	targetIndex := -1
	for i, ch := range allChapters {
		if ch.ID == targetChapter.ID {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		return context
	}

	// Load previous chapters
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex - i
		if idx >= 0 {
			draft := loadDraftContent(allChapters[idx].ID)
			if draft != "" {
				context.Previous = append([]*agents.ContextChapter{{
					Chapter: allChapters[idx],
					Content: draft,
				}}, context.Previous...)
			}
		}
	}

	// Load next chapters
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex + i
		if idx < len(allChapters) {
			draft := loadDraftContent(allChapters[idx].ID)
			if draft != "" {
				context.Next = append(context.Next, &agents.ContextChapter{
					Chapter: allChapters[idx],
					Content: draft,
				})
			}
		}
	}

	return context
}

// loadDraftContent loads draft content for a chapter
func loadDraftContent(chapterID string) string {
	root, err := findProjectRoot()
	if err != nil {
		return ""
	}

	draftPath := filepath.Join(root, "drafts", chapterID+".md")
	data, err := os.ReadFile(draftPath)
	if err != nil {
		return ""
	}

	return string(data)
}

// saveFinalChapter saves the generated final chapter content
func saveFinalChapter(chapter *models.Chapter, content string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	chaptersDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		return fmt.Errorf("failed to create chapters directory: %w", err)
	}

	// Format: chapter-XXX.md
	chapterNum := extractChapterNumber(chapter.ID)
	filename := filepath.Join(chaptersDir, fmt.Sprintf("chapter-%s.md", chapterNum))

	// Build content with header
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", chapter.Title))
	sb.WriteString(fmt.Sprintf("**章节概要**: %s\n\n", chapter.Summary))
	sb.WriteString("---\n\n")
	sb.WriteString(content)

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

// extractChapterNumber extracts chapter number from chapter ID
// Supports formats like "P1-V1-C1" or "C1"
func extractChapterNumber(chapterID string) string {
	// Handle new format like "P1-V1-C1"
	if strings.Contains(chapterID, "-C") {
		parts := strings.Split(chapterID, "-C")
		if len(parts) >= 2 {
			return parts[len(parts)-1]
		}
	}
	// Handle old format like "chap_1_1_1"
	parts := strings.Split(chapterID, "_")
	if len(parts) >= 4 {
		return parts[3]
	}
	// Handle format like "C1"
	if strings.HasPrefix(strings.ToUpper(chapterID), "C") {
		return strings.TrimPrefix(strings.ToUpper(chapterID), "C")
	}
	return chapterID
}

// runWriteImprove improves final chapters based on review
func runWriteImprove(cmd *cobra.Command, args []string) error {
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

	// Create write agent
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline, config.Language)

	// Get project root for state matrix manager
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Create state matrix manager
	stateManager := logic.NewStateMatrixManager(root)

	// Get volumes to improve
	volumes := getVolumesForDraft(outline, writeVolumeFlag, writeChapterFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to improve")
	}

	// Run improvement rounds
	for round := 1; round <= writeMaxRoundsFlag; round++ {
		log.Info("=== Improvement Round %d/%d ===", round, writeMaxRoundsFlag)

		improvedCount := 0

		for _, volume := range volumes {
			// Load review for this volume
			review, err := loadVolumeReview(volume.ID)
			if err != nil {
				log.Warn("No review found for volume %s, skipping", volume.ID)
				continue
			}

			// Get chapters that need improvement
			chaptersToImprove := getChaptersNeedingImprovement(review, outline, writeMinScoreFlag)
			if len(chaptersToImprove) == 0 {
				log.Info("Volume %s: All chapters meet quality threshold", volume.ID)
				continue
			}

			log.Info("Volume %s: Improving %d chapters", volume.ID, len(chaptersToImprove))

			// Improve chapters concurrently
			improved := improveChaptersWithWriteAgent(writeAgent, chaptersToImprove, review.Reviews, outline, stateManager, writeConcurrencyFlag)
			improvedCount += improved
		}

		log.Info("Round %d complete: %d chapters improved", round, improvedCount)

		if improvedCount == 0 {
			log.Info("No more chapters need improvement")
			break
		}

		// Re-review after improvement (if not last round)
		if round < writeMaxRoundsFlag {
			log.Info("Re-reviewing after improvements...")
			if err := runDraftReview(cmd, args); err != nil {
				log.Error("Re-review failed: %v", err)
			}
		}
	}

	log.Info("Improvement process complete")
	return nil
}

// improveChaptersWithWriteAgent improves chapters using the write agent
func improveChaptersWithWriteAgent(agent *agents.WriteAgent, chapters []*models.Chapter, reviews []agents.DraftReview, outline *models.Outline, stateManager *logic.StateMatrixManager, concurrency int) int {
	log := logger.GetLogger()

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

				// Build improvement suggestions
				suggestions := buildImprovementSuggestions(review)

				// Load context drafts
				context := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate story state matrix
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Generate improved content with suggestions
				content, err := agent.GenerateChapterWithSuggestions(chapter, context, stateMatrix, writeWordsFlag, suggestions)
				if err != nil {
					log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				// Save improved content
				if err := saveFinalChapter(chapter, content); err != nil {
					log.Error("[Worker %d] Failed to save improved chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				mu.Lock()
				improvedCount++
				mu.Unlock()

				log.Info("[Worker %d] Improved chapter saved: %s", workerID, chapter.ID)
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
