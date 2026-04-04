package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

var (
	writeChapterFlag               string
	writeVolumeFlag                string
	writePartFlag                  string
	writeWordsFlag                 int
	writeAllFlag                   bool
	writeContextFlag               int
	writeConcurrencyFlag           int
	writeMaxRoundsFlag             int
	writeMinScoreFlag              int
	writeBridgeRetriesFlag         int
	writeTeleportFixFlag           bool
	writeCharacterPatchRetriesFlag int
	writeCharacterFixFlag          bool
	writePromptFlag                string
)

var writeCmd = &cobra.Command{
	Use:   "write",
	Short: "Generate final chapter content",
	Long: `Generate polished final chapter content based on drafts.

This command reads draft chapters and generates refined final content,
ensuring continuity with surrounding chapters by including them as context.

Features:
  - Context-aware generation (includes surrounding chapters)
  - State matrix tracking (character states, relationships, items)
  - Consistent voice and style across chapters

Final chapters are saved to the chapters/ directory.

Subcommands:
  gen      - Generate final chapters from drafts
  improve  - Improve final chapters based on review`,
}

var writeGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate final chapter content",
	Long: `Generate final chapter content with continuity from surrounding drafts.

Examples:
  # Generate final content for chapter 1
  novelgen write gen --chapter 1

  # Generate final content for chapters 1 to 5
  novelgen write gen --chapter 1-5

  # Generate final content for all chapters
  novelgen write gen --all

  # Generate with custom word count
  novelgen write gen --chapter 1 --words 2000

  # Generate with 3 chapters of context on each side
  novelgen write gen --chapter 5 --context 3`,
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
  novelgen write improve --volume 1

  # Improve with max 3 rounds
  novelgen write improve --volume 1 --max-rounds 3

  # Only improve chapters with score below 7
  novelgen write improve --volume 1 --min-score 7`,
	RunE: runWriteImprove,
}

var writeReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review final chapter content",
	Long: `Review final chapter content and generate review reports.

This command will:
1. Load existing final chapters
2. Review each chapter for quality
3. Save review results to story/reviews/

Examples:
  # Review chapter 1
  novelgen write review --chapter 1

  # Review all chapters in volume 1
  novelgen write review --volume 1

  # Review all chapters
  novelgen write review --all`,
	RunE: runWriteReview,
}

var writePipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Run complete writing pipeline: gen -> review -> improve -> recap",
	Long: `Run the complete writing pipeline for a chapter or volume.

This command will:
1. Generate final chapter content (if not exists)
2. Review the generated content
3. Improve content based on review (iterative)
4. Generate recap for continuity

Examples:
  # Run pipeline for chapter 1
  novelgen write pipeline --chapter 1

  # Run pipeline for volume 1 with 3 improvement rounds
  novelgen write pipeline --volume 1 --max-rounds 3

  # Run pipeline with custom target words
  novelgen write pipeline --chapter 1 --words 3000`,
	RunE: runWritePipeline,
}

func init() {
	writeCmd.AddCommand(writeGenCmd)
	writeCmd.AddCommand(writeImproveCmd)
	writeCmd.AddCommand(writeReviewCmd)
	writeCmd.AddCommand(writePipelineCmd)

	writeGenCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter number(s) to generate (e.g., '1', '1-5', or 'P1-V1-C1')")
	writeGenCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume number for context (e.g., '1', 'P1-V1')")
	writeGenCmd.Flags().StringVar(&writePartFlag, "part", "", "Part number for context (e.g., '1', 'P1')")
	writeGenCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	writeGenCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Generate content for all chapters")
	writeGenCmd.Flags().IntVar(&writeContextFlag, "context", 1, "Number of surrounding chapters to include as context")
	writeGenCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent chapter generations")

	writeImproveCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to improve (e.g., '1' or 'P1-V1-C1')")
	writeImproveCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to improve (e.g., '1', 'P1-V1')")
	writeImproveCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to improve (e.g., '1', 'P1')")
	writeImproveCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 1, "Maximum improvement rounds")
	writeImproveCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 7, "Minimum acceptable score (1-10)")
	writeImproveCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent improvements")
	writeImproveCmd.Flags().IntVar(&writeBridgeRetriesFlag, "bridge-retries", 1, "Max retries for teleport transition bridge patch")
	writeImproveCmd.Flags().BoolVar(&writeTeleportFixFlag, "enable-teleport-auto-fix", true, "Enable automatic teleport transition fixes")
	writeImproveCmd.Flags().IntVar(&writeCharacterPatchRetriesFlag, "character-patch-retries", 1, "Max retries for character presence patch")
	writeImproveCmd.Flags().BoolVar(&writeCharacterFixFlag, "enable-character-presence-auto-fix", true, "Enable automatic character presence fixes")
	writeImproveCmd.Flags().StringVar(&writePromptFlag, "prompt", "", "Additional user instructions for improvement")

	writeReviewCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to review (e.g., '1' or 'P1-V1-C1')")
	writeReviewCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to review (e.g., '1', 'P1-V1')")
	writeReviewCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to review (e.g., '1', 'P1')")
	writeReviewCmd.Flags().BoolVar(&writeAllFlag, "all", false, "Review all chapters")
	writeReviewCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent reviews")

	writePipelineCmd.Flags().StringVar(&writeChapterFlag, "chapter", "", "Chapter to process (e.g., '1' or 'P1-V1-C1')")
	writePipelineCmd.Flags().StringVar(&writeVolumeFlag, "volume", "", "Volume to process (e.g., '1', 'P1-V1')")
	writePipelineCmd.Flags().StringVar(&writePartFlag, "part", "", "Part to process (e.g., '1', 'P1')")
	writePipelineCmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "Target word count for the chapter")
	writePipelineCmd.Flags().IntVar(&writeMaxRoundsFlag, "max-rounds", 2, "Maximum improvement rounds")
	writePipelineCmd.Flags().IntVar(&writeMinScoreFlag, "min-score", 7, "Minimum acceptable score (1-10)")
	writePipelineCmd.Flags().IntVar(&writeContextFlag, "context", 1, "Number of surrounding chapters to include as context")
	writePipelineCmd.Flags().IntVar(&writeConcurrencyFlag, "concurrency", 1, "Number of concurrent operations")
	writePipelineCmd.Flags().BoolVar(&writeTeleportFixFlag, "enable-teleport-auto-fix", true, "Enable automatic teleport transition fixes")
	writePipelineCmd.Flags().IntVar(&writeBridgeRetriesFlag, "bridge-retries", 1, "Max retries for teleport transition bridge patch")
	writePipelineCmd.Flags().BoolVar(&writeCharacterFixFlag, "enable-character-presence-auto-fix", true, "Enable automatic character presence fixes")
	writePipelineCmd.Flags().IntVar(&writeCharacterPatchRetriesFlag, "character-patch-retries", 1, "Max retries for character presence patch")

	// Register write command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return writeCmd
	})
}

func runWriteGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

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

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	agent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)

	// Get project root for state matrix manager
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Create state matrix manager
	stateManager := logic.NewStateMatrixManager(root)
	// Create recap agent + store (auto-persist recaps for continuity)
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStore(root)

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
				content, err := agent.GenerateChapter(ctx, chapter, context, stateMatrix, targetWords)
				if err != nil {
					log.Error("Failed to generate content for chapter %s: %v", chapter.ID, err)
					continue
				}

				// Save final content
				if err := saveFinalChapter(chapter, content); err != nil {
					log.Error("Failed to save content for chapter %s: %v", chapter.ID, err)
					continue
				}

				// Auto-extract + persist recap for this final chapter (best-effort)
				if recapData, err := recapAgent.Extract(ctx, chapter.ID, chapter.Title, content); err == nil {
					if ok, reasons := recap.ValidateMinimal(recapData); !ok {
						log.Warn("[Worker %d] Recap minimal validation failed for %s: %v", workerID, chapter.ID, reasons)

						// One retry with explicit feedback to force required fields.
						fb := recapGateFeedback(reasons, recapData)
						if recap2, err2 := recapAgent.ExtractWithFeedback(ctx, chapter.ID, chapter.Title, content, fb); err2 == nil {
							if okR, reasonsR := recap.ValidateMinimal(recap2); okR {
								recapData = recap2
							} else {
								log.Warn("[Worker %d] Recap retry still failed minimal validation for %s: %v", workerID, chapter.ID, reasonsR)
								goto recap_done
							}
						} else {
							log.Warn("[Worker %d] Recap retry extract failed for %s: %v", workerID, chapter.ID, err2)
							goto recap_done
						}
					}

					if ok2, reasons2 := recap.ValidateConsistency(recapData); !ok2 {
						log.Warn("[Worker %d] Recap consistency validation warning for %s: %v", workerID, chapter.ID, reasons2)
					}
					if err := recapStore.Save(recapData); err != nil {
						log.Warn("[Worker %d] Failed to save recap for %s: %v", workerID, chapter.ID, err)
					}
				} else {
					log.Warn("[Worker %d] Failed to extract recap for %s: %v", workerID, chapter.ID, err)
				}
			recap_done:

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

// loadChapterContent loads chapter content, preferring final version over draft
func loadChapterContent(chapterID string) string {
	// Try to load final chapter first
	root, err := findProjectRoot()
	if err != nil {
		return loadDraftContent(chapterID)
	}

	// Check for final chapter
	finalPath := filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", chapterID))
	if data, err := os.ReadFile(finalPath); err == nil {
		return string(data)
	}

	// Fallback to draft
	return loadDraftContent(chapterID)
}

// loadChapterContext loads surrounding chapter content for context
// Prefers final chapters over drafts when available
func loadChapterContext(outline *models.Outline, targetChapter *models.Chapter, contextCount int) *agents.ChapterContext {
	context := &agents.ChapterContext{
		Current:  targetChapter,
		Previous: make([]*agents.ContextChapter, 0),
		Next:     make([]*agents.ContextChapter, 0),
		Recap:    "",
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

	if recapJSON := loadPreviousRecapJSON(outline, targetChapter); strings.TrimSpace(recapJSON) != "" {
		context.Recap = recapJSON
	} else if draftRecap := loadPreviousDraftRecap(outline, targetChapter); strings.TrimSpace(draftRecap) != "" {
		context.Recap = draftRecap
	}

	// Load previous chapters (prefer final, fallback to draft, then outline)
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex - i
		if idx >= 0 {
			ch := allChapters[idx]
			content := loadChapterContent(ch.ID)
			// If no final/draft, build from outline
			if content == "" {
				content = buildChapterContentFromOutline(ch)
			}
			if content != "" {
				// Ensure there's at least an offline recap persisted for the previous
				// chapter so later steps (e.g., transition checks / future drafts) can
				// use it even when generation is out-of-order.
				if i == 1 && content != "" {
					persistOfflineRecapIfMissing(ch, content)
				}

				// Note: Recap extraction is handled separately via recap command
				context.Previous = append([]*agents.ContextChapter{{
					Chapter: ch,
					Content: content,
				}}, context.Previous...)
			}
		}
	}

	// Load next chapters (prefer final, fallback to draft, then outline)
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex + i
		if idx < len(allChapters) {
			ch := allChapters[idx]
			content := loadChapterContent(ch.ID)
			// If no final/draft, build from outline
			if content == "" {
				content = buildChapterContentFromOutline(ch)
			}
			if content != "" {
				context.Next = append(context.Next, &agents.ContextChapter{
					Chapter: ch,
					Content: content,
				})
			}
		}
	}

	return context
}

// buildChapterContentFromOutline builds chapter content from outline data
// Used as fallback when no draft or final content exists
func buildChapterContentFromOutline(chapter *models.Chapter) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Chapter: %s\n", chapter.ID))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", chapter.Summary))
	if len(chapter.Beats) > 0 {
		sb.WriteString("Beats:\n")
		for i, beat := range chapter.Beats {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, beat))
		}
	}
	return sb.String()
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
	sb.WriteString(content)

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

// runWriteImprove improves final chapters based on review
func runWriteImprove(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

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

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	writeAgent.SetLanguage(config.Language)

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

	// Auto-review flag - set to true if any volume needs review
	autoReviewNeeded := false

	// Check if reviews exist for all volumes
	for _, volume := range volumes {
		_, err := loadVolumeReview(volume.ID)
		if err != nil {
			log.Info("No review found for volume %s, will auto-review", volume.ID)
			autoReviewNeeded = true
		}
	}

	// Auto-review if needed
	if autoReviewNeeded {
		log.Info("=== Auto-Review Phase ===")
		log.Info("Some volumes don't have reviews, running auto-review first...")

		// Run review for volumes that need it
		for _, volume := range volumes {
			_, err := loadVolumeReview(volume.ID)
			if err != nil {
				log.Info("Auto-reviewing volume: %s", volume.ID)
				if err := reviewVolumeWithWriteAgent(ctx, writeAgent, stateManager, volume, outline, targetWords, writeConcurrencyFlag); err != nil {
					log.Error("Failed to auto-review volume %s: %v", volume.ID, err)
				}
			}
		}
		log.Info("Auto-review phase complete")
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
			// If user prompt is provided, force improvement on specified chapters regardless of score
			var chaptersToImprove []*models.Chapter
			if writePromptFlag != "" {
				// Force improvement: get chapters specified by flags
				chaptersToImprove = getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, false)
				log.Info("Volume %s: Force improving %d chapters (user prompt provided)", volume.ID, len(chaptersToImprove))
			} else {
				chaptersToImprove = getChaptersNeedingImprovement(review, outline, writeMinScoreFlag)
				if len(chaptersToImprove) == 0 {
					log.Info("Volume %s: All chapters meet quality threshold", volume.ID)
					continue
				}
				log.Info("Volume %s: Improving %d chapters", volume.ID, len(chaptersToImprove))
			}

			// Improve chapters concurrently
			improved := improveChaptersWithWriteAgent(ctx, writeAgent, chaptersToImprove, review.Reviews, outline, stateManager, writeConcurrencyFlag, targetWords)
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
			if err := runWriteReview(cmd, args); err != nil {
				log.Error("Re-review failed: %v", err)
			}
		}
	}

	log.Info("Improvement process complete")
	return nil
}

// improveChaptersWithWriteAgent improves chapters using the write agent
func improveChaptersWithWriteAgent(ctx context.Context, agent *agents.WriteAgent, chapters []*models.Chapter, reviews []agents.DraftReview, outline *models.Outline, stateManager *logic.StateMatrixManager, concurrency int, targetWords int) int {
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

				// Load current chapter content
				currentContent := loadFinalChapterContent(chapter)
				if currentContent == "" {
					log.Error("[Worker %d] No existing content for chapter %s, skipping improvement", workerID, chapter.ID)
					continue
				}

				// Build improvement suggestions
				suggestions := buildImprovementSuggestions(review)

				// Append user prompt if provided
				if writePromptFlag != "" {
					suggestions += "\n\n## 用户要求\n\n" + writePromptFlag
				}

				// Load context drafts
				context := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate story state matrix
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Generate improved content with suggestions
				content, err := agent.GenerateChapterWithSuggestions(ctx, chapter, context, stateMatrix, targetWords, currentContent, suggestions)
				if err != nil {
					log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				// Apply enabled minimal-change fixers (teleport bridge, character presence)
				knownChars := collectKnownCharactersFromOutline(outline)
				fixed, sum := applyImproveFixesWrite(
					log,
					workerID,
					chapter,
					outline,
					content,
					suggestions,
					writeTeleportFixFlag,
					writeBridgeRetriesFlag,
					func(s string) (string, error) {
						return agent.GenerateChapterWithSuggestions(ctx, chapter, context, stateMatrix, targetWords, content, s)
					},
					writeCharacterFixFlag,
					writeCharacterPatchRetriesFlag,
					knownChars,
					func(s string) (string, error) {
						return agent.GenerateChapterWithSuggestions(ctx, chapter, context, stateMatrix, targetWords, content, s)
					},
				)
				content = fixed
				log.Info("[Worker %d] Fix summary for %s: %s", workerID, chapter.ID, sum.String())

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

// runWriteReview reviews final chapters and saves review results
func runWriteReview(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

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

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	writeAgent.SetLanguage(config.Language)

	// Get project root for state matrix manager
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Create state matrix manager
	stateManager := logic.NewStateMatrixManager(root)

	// Get chapters to review based on flags
	chaptersToReview := getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if len(chaptersToReview) == 0 {
		return fmt.Errorf("no chapters found to review")
	}

	log.Info("Reviewing %d chapter(s)", len(chaptersToReview))

	// Group chapters by volume for saving reviews
	volumeReviews := make(map[string]*agents.VolumeReview)

	// Review chapters concurrently
	chapterChan := make(chan *models.Chapter, len(chaptersToReview))
	var wg sync.WaitGroup
	var mu sync.Mutex

	concurrency := writeConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chaptersToReview) {
		concurrency = len(chaptersToReview)
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Load final chapter content
				content := loadFinalChapterContent(chapter)
				if content == "" {
					log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
					continue
				}

				// Load context with recap for continuity checking
				chapterContext := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate state matrix for continuity checking
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Review chapter
				reviewResult, err := writeAgent.ReviewChapter(ctx, chapter, chapterContext, stateMatrix, content, targetWords, 1)
				if err != nil {
					log.Error("[Worker %d] Failed to review chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				// Convert suggestions to strings
				var suggestionStrings []string
				for _, s := range reviewResult.Suggestions {
					suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
				}

				// Convert to DraftReview
				draftReview := agents.DraftReview{
					ChapterID:     chapter.ID,
					ChapterTitle:  chapter.Title,
					OverallScore:  int(reviewResult.OverallScore / 10), // Convert 0-100 to 0-10
					NeedsRevision: reviewResult.OverallScore < 70,
					Suggestions:   suggestionStrings,
				}

				// Get volume ID for this chapter
				volumeID := getVolumeIDFromChapter(chapter.ID)
				volume := outline.GetVolumeByID(volumeID)

				mu.Lock()
				// Create volume review if not exists
				if _, exists := volumeReviews[volumeID]; !exists {
					volumeTitle := ""
					if volume != nil {
						volumeTitle = volume.Title
					}
					volumeReviews[volumeID] = &agents.VolumeReview{
						VolumeID:    volumeID,
						VolumeTitle: volumeTitle,
						Reviews:     make([]agents.DraftReview, 0),
					}
				}
				volumeReviews[volumeID].Reviews = append(volumeReviews[volumeID].Reviews, draftReview)
				mu.Unlock()

				log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
			}
		}(i)
	}

	// Send chapters to workers
	for _, chapter := range chaptersToReview {
		chapterChan <- chapter
	}
	close(chapterChan)

	// Wait for all workers to complete
	wg.Wait()

	// Save all volume reviews
	for volumeID, review := range volumeReviews {
		if err := saveVolumeReview(review); err != nil {
			log.Error("Failed to save review for volume %s: %v", volumeID, err)
			continue
		}
		log.Info("Review saved for volume %s: %d chapters reviewed", volumeID, len(review.Reviews))
	}

	log.Info("Review process complete")
	return nil
}

// getChaptersToReview returns chapters to review based on flags
func getChaptersToReview(outline *models.Outline, chapterFlag, volumeFlag, partFlag string, allFlag bool) []*models.Chapter {
	// If --all flag, return all chapters
	if allFlag {
		return getAllChapters(outline)
	}

	// If chapter flag specified, return specific chapter(s)
	if chapterFlag != "" {
		chapters, err := getChaptersToGenerate(outline, chapterFlag, volumeFlag, partFlag, false)
		if err == nil {
			return chapters
		}
	}

	// If volume flag specified, return all chapters in volume
	if volumeFlag != "" {
		var chapters []*models.Chapter
		volumeID := resolveVolumeID(outline, volumeFlag, partFlag)
		for _, ch := range getAllChapters(outline) {
			if strings.HasPrefix(ch.ID, volumeID) {
				chapters = append(chapters, ch)
			}
		}
		return chapters
	}

	// Default: return all chapters
	return getAllChapters(outline)
}

// getVolumeIDFromChapter extracts volume ID from chapter ID
// e.g., "P1-V1-C1" -> "P1-V1"
func getVolumeIDFromChapter(chapterID string) string {
	parts := strings.Split(chapterID, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], "-")
	}
	return ""
}

// resolveVolumeID resolves volume flag to full volume ID
func resolveVolumeID(outline *models.Outline, volumeFlag, partFlag string) string {
	// If already full ID, return as is
	if strings.Contains(volumeFlag, "-") {
		return volumeFlag
	}

	// Try to resolve with part flag
	partID := partFlag
	if partID == "" {
		partID = "P1" // Default to first part
	}
	if !strings.HasPrefix(partID, "P") {
		partID = "P" + partID
	}

	volumeID := volumeFlag
	if !strings.HasPrefix(volumeID, "V") {
		volumeID = "V" + volumeID
	}

	return partID + "-" + volumeID
}

// reviewVolumeWithWriteAgent reviews all chapters in a volume using the write agent
func reviewVolumeWithWriteAgent(ctx context.Context, writeAgent *agents.WriteAgent, stateManager *logic.StateMatrixManager, volume *models.Volume, outline *models.Outline, targetWords int, concurrency int) error {
	log := logger.GetLogger()

	// Get all chapters in this volume
	var chapters []*models.Chapter
	for _, ch := range getAllChapters(outline) {
		if strings.HasPrefix(ch.ID, volume.ID) {
			chapters = append(chapters, ch)
		}
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found in volume %s", volume.ID)
	}

	// Create volume review
	volumeReview := &agents.VolumeReview{
		VolumeID:    volume.ID,
		VolumeTitle: volume.Title,
		Reviews:     make([]agents.DraftReview, 0),
	}

	// Review chapters concurrently
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup
	var mu sync.Mutex

	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chapter := range chapterChan {
				log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

				// Load final chapter content
				content := loadFinalChapterContent(chapter)
				if content == "" {
					log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
					continue
				}

				// Load context with recap for continuity checking
				chapterContext := loadChapterContext(outline, chapter, writeContextFlag)

				// Calculate state matrix for continuity checking
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Review chapter
				reviewResult, err := writeAgent.ReviewChapter(ctx, chapter, chapterContext, stateMatrix, content, targetWords, 1)
				if err != nil {
					log.Error("[Worker %d] Failed to review chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				// Convert suggestions to strings
				var suggestionStrings []string
				for _, s := range reviewResult.Suggestions {
					suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
				}

				// Convert to DraftReview
				draftReview := agents.DraftReview{
					ChapterID:     chapter.ID,
					ChapterTitle:  chapter.Title,
					OverallScore:  int(reviewResult.OverallScore / 10), // Convert 0-100 to 0-10
					NeedsRevision: reviewResult.OverallScore < 70,
					Suggestions:   suggestionStrings,
				}

				mu.Lock()
				volumeReview.Reviews = append(volumeReview.Reviews, draftReview)
				mu.Unlock()

				log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
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

	// Save volume review
	if err := saveVolumeReview(volumeReview); err != nil {
		return fmt.Errorf("failed to save review for volume %s: %w", volume.ID, err)
	}

	log.Info("Review saved for volume %s: %d chapters reviewed", volume.ID, len(volumeReview.Reviews))
	return nil
}

// runWritePipeline runs the complete writing pipeline: gen -> review -> improve -> recap
func runWritePipeline(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

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

	// Use config target words if flag is default (2000)
	targetWords := writeWordsFlag
	if targetWords == 2000 && config.ChapterConfig.TargetWordsPerChapter > 0 {
		targetWords = config.ChapterConfig.TargetWordsPerChapter
	}

	// Create write agent
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	writeAgent.SetLanguage(config.Language)

	// Get project root for state matrix manager
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Create state matrix manager
	stateManager := logic.NewStateMatrixManager(root)

	// Get chapters to process
	chaptersToProcess := getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if len(chaptersToProcess) == 0 {
		return fmt.Errorf("no chapters found to process")
	}

	log.Info("=== WRITE PIPELINE ===")
	log.Info("Processing %d chapter(s)", len(chaptersToProcess))

	// Create recap agent for all chapters
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStore(root)

	// Process each chapter completely before moving to the next
	for chapterIdx, chapter := range chaptersToProcess {
		log.Info("\n%s", strings.Repeat("=", 60))
		log.Info("Processing Chapter %d/%d: %s - %s", chapterIdx+1, len(chaptersToProcess), chapter.ID, chapter.Title)
		log.Info("%s", strings.Repeat("=", 60))

		// Step 1: Generate chapter (if not exists)
		log.Info("\n[Step 1/4] Generating chapter...")
		content := loadFinalChapterContent(chapter)
		if content == "" {
			log.Info("Generating chapter: %s - %s", chapter.ID, chapter.Title)
			chapterContext := loadChapterContext(outline, chapter, writeContextFlag)
			stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)
			generatedContent, err := writeAgent.GenerateChapter(ctx, chapter, chapterContext, stateMatrix, targetWords)
			if err != nil {
				log.Error("Failed to generate chapter %s: %v", chapter.ID, err)
				continue
			}
			if err := saveFinalChapter(chapter, generatedContent); err != nil {
				log.Error("Failed to save chapter %s: %v", chapter.ID, err)
				continue
			}
			content = generatedContent
			log.Info("✓ Generated and saved chapter: %s", chapter.ID)
		} else {
			log.Info("Chapter %s already exists, skipping generation", chapter.ID)
		}

		// Step 2: Review chapter
		log.Info("\n[Step 2/4] Reviewing chapter...")
		chapterContext := loadChapterContext(outline, chapter, writeContextFlag)
		stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)
		reviewResult, err := writeAgent.ReviewChapter(ctx, chapter, chapterContext, stateMatrix, content, targetWords, 1)
		if err != nil {
			log.Error("Failed to review chapter %s: %v", chapter.ID, err)
			continue
		}

		// Save review
		var suggestionStrings []string
		for _, s := range reviewResult.Suggestions {
			suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
		}
		draftReview := agents.DraftReview{
			ChapterID:     chapter.ID,
			ChapterTitle:  chapter.Title,
			OverallScore:  int(reviewResult.OverallScore / 10),
			NeedsRevision: reviewResult.OverallScore < 70,
			Suggestions:   suggestionStrings,
		}
		volumeID := getVolumeIDFromChapter(chapter.ID)
		volume := outline.GetVolumeByID(volumeID)
		var volumeReview *agents.VolumeReview
		reviewPath := filepath.Join(root, "story", "reviews", volumeID+"_review.json")
		if data, err := os.ReadFile(reviewPath); err == nil {
			var existingReview agents.VolumeReview
			if err := json.Unmarshal(data, &existingReview); err == nil {
				volumeReview = &existingReview
			}
		}
		if volumeReview == nil {
			volumeTitle := ""
			if volume != nil {
				volumeTitle = volume.Title
			}
			volumeReview = &agents.VolumeReview{
				VolumeID:    volumeID,
				VolumeTitle: volumeTitle,
				Reviews:     make([]agents.DraftReview, 0),
			}
		}
		found := false
		for i := range volumeReview.Reviews {
			if volumeReview.Reviews[i].ChapterID == chapter.ID {
				volumeReview.Reviews[i] = draftReview
				found = true
				break
			}
		}
		if !found {
			volumeReview.Reviews = append(volumeReview.Reviews, draftReview)
		}
		if err := saveVolumeReview(volumeReview); err != nil {
			log.Error("Failed to save review for chapter %s: %v", chapter.ID, err)
		}
		log.Info("✓ Reviewed chapter %s: score %.1f", chapter.ID, reviewResult.OverallScore)

		// Step 3: Improve chapter (iterative)
		log.Info("\n[Step 3/4] Improving chapter...")
		currentContent := content
		chapterReview := &draftReview
		for round := 1; round <= writeMaxRoundsFlag; round++ {
			if len(chapterReview.Suggestions) == 0 && !chapterReview.NeedsRevision {
				log.Info("Chapter %s: no suggestions and no revision needed, skipping improvement", chapter.ID)
				break
			}

			log.Info("Improvement round %d/%d (score: %d)", round, writeMaxRoundsFlag, chapterReview.OverallScore)
			suggestions := buildImprovementSuggestions(chapterReview)
			chapterContext := loadChapterContext(outline, chapter, writeContextFlag)
			stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)
			improvedContent, err := writeAgent.GenerateChapterWithSuggestions(ctx, chapter, chapterContext, stateMatrix, targetWords, currentContent, suggestions)
			if err != nil {
				log.Error("Failed to improve chapter %s: %v", chapter.ID, err)
				break
			}
			if err := saveFinalChapter(chapter, improvedContent); err != nil {
				log.Error("Failed to save improved chapter %s: %v", chapter.ID, err)
				break
			}
			currentContent = improvedContent
			log.Info("✓ Improved chapter: %s", chapter.ID)

			// Re-review after improvement (if not last round)
			if round < writeMaxRoundsFlag {
				log.Info("Re-reviewing chapter %s after improvement...", chapter.ID)
				reviewResult, err := writeAgent.ReviewChapter(ctx, chapter, chapterContext, stateMatrix, currentContent, targetWords, 1)
				if err != nil {
					log.Error("Failed to re-review chapter %s: %v", chapter.ID, err)
					break
				}
				var suggestionStrings []string
				for _, s := range reviewResult.Suggestions {
					suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
				}
				chapterReview.OverallScore = int(reviewResult.OverallScore / 10)
				chapterReview.NeedsRevision = reviewResult.OverallScore < 70
				chapterReview.Suggestions = suggestionStrings
				// Save updated review
				if err := saveVolumeReview(volumeReview); err != nil {
					log.Error("Failed to save updated review for chapter %s: %v", chapter.ID, err)
				}
				// Check if quality threshold met
				if reviewResult.OverallScore >= 85 {
					log.Info("Quality threshold met (%.1f >= 85), stopping improvement", reviewResult.OverallScore)
					break
				}
			}
		}

		// Step 4: Generate recap
		log.Info("\n[Step 4/4] Generating recap...")
		chapterRecap, err := recapAgent.Extract(ctx, chapter.ID, chapter.Title, currentContent)
		if err != nil {
			log.Error("Failed to generate recap for chapter %s: %v", chapter.ID, err)
		} else {
			if err := recapStore.Save(chapterRecap); err != nil {
				log.Error("Failed to save recap for chapter %s: %v", chapter.ID, err)
			} else {
				log.Info("✓ Generated and saved recap for chapter: %s", chapter.ID)
			}
		}

		log.Info("\n✓ Completed chapter: %s", chapter.ID)
	}

	log.Info("\n=== PIPELINE COMPLETE ===")
	log.Info("All phases completed successfully!")
	return nil
}
