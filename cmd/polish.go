package cmd

import (
	"context"
	"fmt"
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
	polishVolumeFlag      string
	polishPartFlag        string
	polishMaxRoundsFlag   int
	polishMinScoreFlag    int
	polishConcurrencyFlag int
	polishPromptFlag      string
)

var polishCmd = &cobra.Command{
	Use:   "polish",
	Short: "Polish a volume of chapters",
	Long: `Polish command performs volume-level review and improvement.

This command will:
1. Load all chapters in the specified volume
2. Perform a holistic volume-level review (AI sees ALL chapters at once)
3. Generate volume-level improvement suggestions for each chapter
4. Improve each chapter based on volume-level feedback
5. Update recaps after improvement

Examples:
  # Polish volume 1
  novelgen polish --volume 1

  # Polish with custom rounds
  novelgen polish --volume 1 --max-rounds 3

  # Polish with specific instructions
  novelgen polish --volume 1 --prompt "加强人物情感描写"`,
	RunE: runPolish,
}

func init() {
	polishCmd.Flags().StringVar(&polishVolumeFlag, "volume", "", "Volume to polish (e.g., '1' or 'P1-V1')")
	polishCmd.Flags().StringVar(&polishPartFlag, "part", "", "Part to polish (e.g., '1' or 'P1')")
	polishCmd.Flags().IntVar(&polishMaxRoundsFlag, "max-rounds", 2, "Maximum improvement rounds")
	polishCmd.Flags().IntVar(&polishMinScoreFlag, "min-score", 8, "Minimum acceptable score (1-10)")
	polishCmd.Flags().IntVar(&polishConcurrencyFlag, "concurrency", 1, "Number of concurrent improvements")
	polishCmd.Flags().StringVar(&polishPromptFlag, "prompt", "", "Additional instructions for volume-level improvement")

	// Register polish command using the plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return polishCmd
	})
}

func runPolish(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load story setup
	setup, err := loadStorySetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
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

	// Get project root
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Initialize agents
	writeAgent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	writeAgent.SetLanguage(config.Language)
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStore(root)

	// Initialize state manager
	stateManager := logic.NewStateMatrixManager(root)

	// Determine target words per chapter
	targetWords := config.ChapterConfig.TargetWordsPerChapter
	if targetWords == 0 {
		targetWords = 3000
	}

	// Get volumes to polish
	volumes := getVolumesForPolish(outline, polishVolumeFlag, polishPartFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to polish")
	}

	log.Info("Polishing %d volume(s)", len(volumes))

	// Process each volume
	for _, volume := range volumes {
		log.Info("Polishing Volume: %s - %s", volume.ID, volume.Title)

		// Get chapters for this volume
		chapters := getChaptersForVolume(outline, volume.ID)
		if len(chapters) == 0 {
			log.Warn("No chapters found in volume %s", volume.ID)
			continue
		}

		log.Info("Found %d chapters in volume %s", len(chapters), volume.ID)

		// Run improvement rounds
		for round := 1; round <= polishMaxRoundsFlag; round++ {
			log.Info("--- Improvement Round %d/%d ---", round, polishMaxRoundsFlag)

			// Step 1: Volume-level review (AI sees all chapters at once)
			log.Info("Performing volume-level review (AI will see all %d chapters)...", len(chapters))
			volumeReviewResult, err := performVolumeReview(ctx, writeAgent, volume, chapters, targetWords)
			if err != nil {
				log.Error("Failed to perform volume review: %v", err)
				continue
			}

			// Check if any chapter needs improvement
			hasAnySuggestions := false
			for _, cr := range volumeReviewResult.ChapterReviews {
				if len(cr.Issues) > 0 || len(cr.Suggestions) > 0 {
					hasAnySuggestions = true
					break
				}
			}
			if len(volumeReviewResult.VolumeLevelIssues) > 0 || len(volumeReviewResult.VolumeLevelSuggestions) > 0 {
				hasAnySuggestions = true
			}

			if !hasAnySuggestions && polishPromptFlag == "" {
				log.Info("No suggestions for any chapter, stopping improvement")
				break
			}

			// Step 2: Improve chapters based on volume review
			log.Info("Improving chapters based on volume review...")
			improvedCount := improveChaptersWithVolumeReview(ctx, writeAgent, recapAgent, recapStore, stateManager, volume, chapters, volumeReviewResult, outline, targetWords)
			log.Info("Round %d complete: %d chapters improved", round, improvedCount)

			// If no chapters were improved, stop
			if improvedCount == 0 {
				log.Info("No chapters improved in this round, stopping")
				break
			}
		}

		log.Info("Volume %s polishing complete", volume.ID)
	}

	log.Info("Polish complete!")
	return nil
}

// getVolumesForPolish returns volumes to polish based on flags
func getVolumesForPolish(outline *models.Outline, volumeFlag, partFlag string) []*models.Volume {
	var volumes []*models.Volume

	// Get all volumes from outline
	allVolumes := getAllVolumes(outline)

	if volumeFlag != "" {
		// Specific volume
		volumeID := resolveVolumeID(outline, volumeFlag, partFlag)
		for i := range allVolumes {
			if allVolumes[i].ID == volumeID {
				volumes = append(volumes, allVolumes[i])
				break
			}
		}
	} else if partFlag != "" {
		// All volumes in part
		partID := partFlag
		if !strings.HasPrefix(partID, "P") {
			partID = "P" + partID
		}
		for i := range allVolumes {
			if strings.HasPrefix(allVolumes[i].ID, partID+"-") {
				volumes = append(volumes, allVolumes[i])
			}
		}
	} else {
		// All volumes
		volumes = allVolumes
	}

	return volumes
}

// getAllVolumes returns all volumes from outline
func getAllVolumes(outline *models.Outline) []*models.Volume {
	var volumes []*models.Volume
	for i := range outline.Parts {
		for j := range outline.Parts[i].Volumes {
			volumes = append(volumes, &outline.Parts[i].Volumes[j])
		}
	}
	return volumes
}

// getChaptersForVolume returns all chapters in a volume
func getChaptersForVolume(outline *models.Outline, volumeID string) []*models.Chapter {
	var chapters []*models.Chapter

	for i := range outline.Parts {
		for j := range outline.Parts[i].Volumes {
			if outline.Parts[i].Volumes[j].ID == volumeID {
				for k := range outline.Parts[i].Volumes[j].Chapters {
					chapters = append(chapters, &outline.Parts[i].Volumes[j].Chapters[k])
				}
				return chapters
			}
		}
	}

	return chapters
}

// performVolumeReview performs a holistic review of all chapters in a volume
func performVolumeReview(ctx context.Context, writeAgent *agents.WriteAgent, volume *models.Volume, chapters []*models.Chapter, targetWords int) (agents.VolumeReviewResult, error) {
	log := logger.GetLogger()

	// Load all chapter contents
	chapterContents := make(map[string]string)
	for _, chapter := range chapters {
		content := loadFinalChapterContent(chapter)
		if content == "" {
			log.Warn("No content found for chapter %s", chapter.ID)
			continue
		}
		chapterContents[chapter.ID] = content
	}

	if len(chapterContents) == 0 {
		return agents.VolumeReviewResult{}, fmt.Errorf("no chapters with content to review")
	}

	log.Info("Loaded %d chapters for volume review", len(chapterContents))

	// Perform volume-level review (AI sees all chapters at once)
	return writeAgent.ReviewVolume(ctx, volume, chapters, chapterContents, targetWords)
}

// improveChaptersWithVolumeReview improves chapters based on volume-level review
func improveChaptersWithVolumeReview(ctx context.Context, writeAgent *agents.WriteAgent, recapAgent *agents.RecapAgent, recapStore *recap.Store, stateManager *logic.StateMatrixManager, volume *models.Volume, chapters []*models.Chapter, volumeReview agents.VolumeReviewResult, outline *models.Outline, targetWords int) int {
	log := logger.GetLogger()

	// Create review map
	reviewMap := make(map[string]*agents.VolumeChapterReview)
	for i := range volumeReview.ChapterReviews {
		reviewMap[volumeReview.ChapterReviews[i].ChapterID] = &volumeReview.ChapterReviews[i]
	}

	// Create work channel
	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup
	improvedCount := 0
	var mu sync.Mutex

	concurrency := polishConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

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

				// Check if improvement is needed
				hasSuggestions := len(review.Issues) > 0 || len(review.Suggestions) > 0 || len(volumeReview.VolumeLevelIssues) > 0 || len(volumeReview.VolumeLevelSuggestions) > 0
				hasUserPrompt := polishPromptFlag != ""

				// Skip if no suggestions and no user prompt
				if !hasSuggestions && !hasUserPrompt {
					log.Info("[Worker %d] Chapter %s: no suggestions, skipping", workerID, chapter.ID)
					continue
				}

				// Skip if score is good enough and no suggestions and no user prompt
				if review.ChapterScore >= float64(polishMinScoreFlag) && !hasSuggestions && !hasUserPrompt {
					log.Info("[Worker %d] Chapter %s score %.1f >= %d, skipping", workerID, chapter.ID, review.ChapterScore, polishMinScoreFlag)
					continue
				}

				log.Info("[Worker %d] Improving chapter: %s - %s (role: %s, score: %.1f)", workerID, chapter.ID, chapter.Title, review.ChapterRole, review.ChapterScore)

				// Load current content
				currentContent := loadFinalChapterContent(chapter)
				if currentContent == "" {
					log.Error("[Worker %d] No content for chapter %s, skipping", workerID, chapter.ID)
					continue
				}

				// Build suggestions from volume review
				var suggestions strings.Builder
				suggestions.WriteString("## 卷级评审建议\n\n")
				suggestions.WriteString(fmt.Sprintf("本章在卷中的定位: %s\n", review.ChapterRole))
				suggestions.WriteString(fmt.Sprintf("与前章衔接: %s\n", review.ContinuityWithPrev))
				suggestions.WriteString(fmt.Sprintf("与后章衔接: %s\n", review.ContinuityWithNext))

				// Add chapter-specific issues and suggestions
				if len(review.Issues) > 0 {
					suggestions.WriteString("\n### 本章问题\n")
					for _, issue := range review.Issues {
						suggestions.WriteString(fmt.Sprintf("- %s\n", issue))
					}
				}

				if len(review.Suggestions) > 0 {
					suggestions.WriteString("\n### 改进建议\n")
					for _, s := range review.Suggestions {
						suggestions.WriteString(fmt.Sprintf("- %s\n", s))
					}
				}

				// Add volume-level issues and suggestions
			if len(volumeReview.VolumeLevelIssues) > 0 {
				suggestions.WriteString("\n### 卷级整体问题（请检查本章是否涉及以下问题，如涉及则必须修正）\n")
				for _, issue := range volumeReview.VolumeLevelIssues {
					suggestions.WriteString(fmt.Sprintf("- %s\n", issue))
				}
			}

			if len(volumeReview.VolumeLevelSuggestions) > 0 {
				suggestions.WriteString("\n### 卷级整体建议（请检查本章是否涉及以下内容，如涉及则按建议修正）\n")
				for _, s := range volumeReview.VolumeLevelSuggestions {
					suggestions.WriteString(fmt.Sprintf("- %s\n", s))
				}
			}

				// Add user prompt if provided
				if polishPromptFlag != "" {
					suggestions.WriteString("\n\n## 用户要求\n\n")
					suggestions.WriteString(polishPromptFlag)
				}

				// Load context
				chapterContext := loadChapterContext(outline, chapter, 2)

				// Calculate state matrix
				stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

				// Generate improved content
				content, err := writeAgent.GenerateChapterWithSuggestions(ctx, chapter, chapterContext, stateMatrix, targetWords, currentContent, suggestions.String())
				if err != nil {
					log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				// Save improved content
				if err := saveFinalChapter(chapter, content); err != nil {
					log.Error("[Worker %d] Failed to save chapter %s: %v", workerID, chapter.ID, err)
					continue
				}

				// Update recap
				chapterRecap, err := recapAgent.Extract(ctx, chapter.ID, chapter.Title, content)
				if err != nil {
					log.Error("[Worker %d] Failed to generate recap for chapter %s: %v", workerID, chapter.ID, err)
				} else {
					if err := recapStore.Save(chapterRecap); err != nil {
						log.Error("[Worker %d] Failed to save recap for chapter %s: %v", workerID, chapter.ID, err)
					} else {
						log.Info("[Worker %d] Updated recap for chapter %s", workerID, chapter.ID)
					}
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

	wg.Wait()

	return improvedCount
}
