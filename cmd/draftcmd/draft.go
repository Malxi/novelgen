// Package draftcmd implements the legacy `novelgen draft` command tree.
package draftcmd

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/agents"
	"novelgen/internal/continuity"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/reviewstore"
	"novelgen/internal/rpg/dsl"
	"novelgen/internal/toolplane"
	"novelgen/internal/workerpool"

	"github.com/spf13/cobra"
)

var (
	draftChapterFlag               string
	draftVolumeFlag                string
	draftPartFlag                  string
	draftWordsFlag                 int
	draftAllFlag                   bool
	draftConcurrencyFlag           int
	draftContextFlag               int
	draftMaxRoundsFlag             int
	draftMinScoreFlag              int
	draftBridgeRetriesFlag         int
	draftTeleportFixFlag           bool
	draftCharacterPatchRetriesFlag int
	draftCharacterFixFlag          bool
)

var draftCmd = &cobra.Command{
	Use:        "draft",
	Short:      "Legacy: generate and improve draft chapters",
	Hidden:     true,
	Deprecated: "draft is a legacy optional workflow; use `novelgen write pipeline` for the default direct-to-final flow.",
	Long: `Legacy optional workflow for generating, reviewing, and improving draft chapters.

Drafts are preliminary versions of chapters saved to the drafts/ directory.
They are no longer part of the recommended default workflow. New projects should
write final chapters directly with:

  novelgen write pipeline --chapter P1-V1-C1
  novelgen write pipeline --volume P1-V1

Subcommands:
  gen      - Generate new draft chapters
  review   - Review drafts and provide detailed feedback
  improve  - Improve drafts based on review feedback

The typical workflow:
  1. draft gen     - Generate initial drafts
  2. draft review  - Review and identify issues
  3. draft improve - Fix issues based on review`,
}

var draftGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate draft chapter(s)",
	Long: `Generate draft chapter(s) based on outline and story state.

Examples:
  # Generate draft for chapter 1
  novelgen draft gen --chapter 1

  # Generate draft for chapters 1 to 5
  novelgen draft gen --chapter 1-5

  # Generate draft for all chapters
  novelgen draft gen --all

  # Generate draft for specific chapter by ID
  novelgen draft gen --chapter chap_1_1_1

  # Generate with custom word count
  novelgen draft gen --chapter 1 --words 800

  # Generate for first chapter of volume 2
  novelgen draft gen --volume 2 --chapter 1`,
	RunE: runDraftGen,
}

var draftReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review drafts and provide feedback",
	Long: `Review drafts and generate detailed feedback for improvement.

Examples:
  # Review all drafts in volume 1
  novelgen draft review --volume 1

  # Review specific chapter
  novelgen draft review --chapter 1

  # Review with concurrency
  novelgen draft review --volume 1 --concurrency 3`,
	RunE: runDraftReview,
}

var draftImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve drafts based on review feedback",
	Long: `Improve drafts by regenerating chapters that need revision.

Examples:
  # Improve all chapters in volume 1
  novelgen draft improve --volume 1

  # Improve with max 3 rounds
  novelgen draft improve --volume 1 --max-rounds 3

  # Only improve chapters with score below 7
  novelgen draft improve --volume 1 --min-score 7`,
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
	draftGenCmd.Flags().IntVar(&draftContextFlag, "context", 1, "Number of previous chapters to include as continuity context")
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
	draftImproveCmd.Flags().IntVar(&draftBridgeRetriesFlag, "bridge-retries", 1, "Max retries for teleport transition bridge patch")
	draftImproveCmd.Flags().BoolVar(&draftTeleportFixFlag, "enable-teleport-auto-fix", true, "Enable automatic teleport transition fixes")
	draftImproveCmd.Flags().IntVar(&draftCharacterPatchRetriesFlag, "character-patch-retries", 1, "Max retries for character presence patch")
	draftImproveCmd.Flags().BoolVar(&draftCharacterFixFlag, "enable-character-presence-auto-fix", true, "Enable automatic character presence fixes")
}

// NewCommand returns the `novelgen draft` command tree.
func NewCommand() *cobra.Command {
	return draftCmd
}

func runDraftGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

	// Resolve the project this command operates on once, then read everything through it.
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}

	config, err := book.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := book.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := book.LoadOutline()
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
	agent := agents.NewDraftAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)

	// Create state matrix manager
	stateManager := logic.NewStateMatrixManagerAt(book.Root)
	// Create recap agent + store (auto-persist recaps for continuity)
	recapAgent := agents.NewRecapAgent(client, cfg, &config.LLM)
	recapAgent.SetLanguage(config.Language)
	recapStore := recap.NewStoreAt(book.Root)

	// Get list of chapters to generate
	chapters, err := project.SelectChapters(outline, draftChapterFlag, draftVolumeFlag, draftPartFlag, draftAllFlag)
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

	_, _ = workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (struct{}, error) {
		log.Info("[Worker %d] Generating draft for chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Calculate story state matrix
		stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

		// Load continuity context from previous draft chapters
		contextText := continuity.LoadPreviousDraftContext(book, outline, chapter, draftContextFlag)
		// Extract a compact recap from the immediately previous chapter (if available)
		prevRecap := continuity.LoadPreviousDraftRecap(book, outline, chapter)
		// Load next chapters for foreshadowing (default: 2 chapters ahead)
		nextChapters := continuity.LoadNextChapters(outline, chapter, 2)

		// Generate draft
		draft, err := agent.GenerateDraftWithContext(ctx, chapter, stateMatrix, draftWordsFlag, contextText, prevRecap, nextChapters)
		if err != nil {
			log.Error("Failed to generate draft for chapter %s: %v", chapter.ID, err)
			return struct{}{}, nil
		}

		// Persist a best-effort recap for this chapter so the NEXT chapter can
		// reuse a stable, compact continuity anchor without additional LLM calls.
		continuity.PersistOfflineRecap(book, chapter, draft)

		// Save draft
		if err := saveDraft(book, chapter, draft); err != nil {
			log.Error("Failed to save draft for chapter %s: %v", chapter.ID, err)
			return struct{}{}, nil
		}

		// Auto-extract + persist recap for this chapter (best-effort)
		if recapData, err := recapAgent.Extract(ctx, chapter.ID, chapter.Title, draft); err == nil {
			if ok, reasons := recap.ValidateMinimal(recapData); !ok {
				log.Warn("[Worker %d] Recap minimal validation failed for %s: %v", workerID, chapter.ID, reasons)

				// One retry with explicit feedback to force required fields.
				fb := continuity.RecapGateFeedback(reasons, recapData)
				if recap2, err2 := recapAgent.ExtractWithFeedback(ctx, chapter.ID, chapter.Title, draft, fb); err2 == nil {
					if okR, reasonsR := recap.ValidateMinimal(recap2); okR {
						recapData = recap2
					} else {
						log.Warn("[Worker %d] Recap retry still failed minimal validation for %s: %v", workerID, chapter.ID, reasonsR)
						// Fallback: keep offline recap we already persisted; do not overwrite store with low-quality recap.
						goto recap_done
					}
				} else {
					log.Warn("[Worker %d] Recap retry extract failed for %s: %v", workerID, chapter.ID, err2)
					goto recap_done
				}
			}

			if ok2, reasons2 := recap.ValidateConsistency(recapData); !ok2 {
				log.Warn("[Worker %d] Recap consistency validation warning for %s: %v", workerID, chapter.ID, reasons2)
				// Still save, but note: minimal fields are present; hint may be weak.
			}
			root := recapStore.Root()
			if root == nil {
				log.Warn("[Worker %d] Failed to save recap for %s: project root is missing", workerID, chapter.ID)
			} else if _, err := toolplane.NewMutationService(root).ReplaceRecap(ctx, recapData, toolplane.MutationOptions{
				Validate: false,
				Source:   "draft-command",
			}); err != nil {
				log.Warn("[Worker %d] Failed to save recap for %s: %v", workerID, chapter.ID, err)
			}
		} else {
			log.Warn("[Worker %d] Failed to extract recap for %s: %v", workerID, chapter.ID, err)
		}
	recap_done:

		log.Info("[Worker %d] Draft saved for chapter %s: %d words", workerID, chapter.ID, len(strings.Fields(draft)))
		return struct{}{}, nil
	})

	log.Info("Draft generation complete")
	return nil
}

// saveDraft saves the generated draft into an explicit project.
func saveDraft(book *project.Book, chapter *models.Chapter, draft string) error {
	if book == nil {
		return fmt.Errorf("project is required")
	}
	_, err := toolplane.NewMutationService(book.Root).ReplaceDraft(context.Background(), chapter, draft, toolplane.MutationOptions{
		Source: "draft-command",
	})
	return err
}

// runDraftReview reviews drafts and provides feedback
func runDraftReview(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

	// Resolve the project this command operates on once, then read everything through it.
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}

	config, err := book.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := book.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := book.LoadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load drafts
	drafts, err := loadAllDrafts(book)
	if err != nil {
		return fmt.Errorf("failed to load drafts: %w", err)
	}
	if len(drafts) == 0 {
		return fmt.Errorf("no drafts found. Run 'novelgen draft gen' first")
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

	// Create draft agent for review
	agent := agents.NewDraftAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)

	// Get volumes to review
	volumes := project.VolumesForSelection(outline, draftVolumeFlag, draftChapterFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to review")
	}

	// Review each volume
	for _, volume := range volumes {
		log.Info("Reviewing volume: %s - %s", volume.ID, volume.Title)

		// Filter drafts for this volume
		volumeDrafts := filterDraftsForVolume(drafts, volume)

		review, err := agent.ReviewVolume(ctx, volume, volumeDrafts)
		if err != nil {
			log.Error("Failed to review volume %s: %v", volume.ID, err)
			continue
		}

		// Apply deterministic heuristic checks (best-effort) to catch obvious issues
		continuity.ApplyHeuristicTransitionChecks(book, volume, volumeDrafts, review)

		// Save review
		if err := reviewstore.SaveVolumeReviewAt(book, review); err != nil {
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
	ctx := cmd.Context()

	// Resolve the project this command operates on once, then read everything through it.
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}

	config, err := book.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load story setup
	setup, err := book.LoadSetup()
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}

	// Load outline
	outline, err := book.LoadOutline()
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
	draftAgent := agents.NewDraftAgent(client, cfg, &config.LLM, setup, outline)
	draftAgent.SetLanguage(config.Language)

	// Get volumes to improve
	volumes := project.VolumesForSelection(outline, draftVolumeFlag, draftChapterFlag)
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to improve")
	}

	// Run improvement rounds
	for round := 1; round <= draftMaxRoundsFlag; round++ {
		log.Info("=== Improvement Round %d/%d ===", round, draftMaxRoundsFlag)

		improvedCount := 0

		for _, volume := range volumes {
			// Load review for this volume
			review, err := reviewstore.LoadVolumeReviewAt(book, volume.ID)
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
			improved := improveChaptersWithAgent(ctx, book, draftAgent, chaptersToImprove, review.Reviews, outline, draftConcurrencyFlag)
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

// loadAllDrafts loads all draft files from an explicit project.
func loadAllDrafts(book *project.Book) (map[string]string, error) {
	return book.LoadAllDrafts()
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
func improveChaptersWithAgent(ctx context.Context, book *project.Book, agent *agents.DraftAgent, chapters []*models.Chapter, reviews []agents.DraftReview, outline *models.Outline, concurrency int) int {
	log := logger.GetLogger()

	stateManager := logic.NewStateMatrixManagerAt(book.Root)

	// Pre-build DSL simulation for enrichment (shared across goroutines)
	var dslIssues []dsl.SimulationIssue
	dslBridge := dsl.NewSimulationBridge()
	if charModels, locModels, itemModels, orgModels, elErr := book.LoadElements(); elErr == nil {
		setup, _ := book.LoadSetup()
		dslAdapter := dsl.NewModelAdapterWithOrganizations(setup, outline, charModels, locModels, itemModels, orgModels)
		dslIssues, _ = dslAdapter.Simulate(dsl.PhaseCraft)
		if len(dslIssues) > 0 {
			log.Info("DSL simulation loaded %d issues for improvement enrichment", len(dslIssues))
		}
	}

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

	improved, _ := workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (bool, error) {
		review := reviewMap[chapter.ID]
		if review == nil {
			return false, nil
		}

		log.Info("[Worker %d] Improving chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Build improvement prompt
		suggestions := reviewstore.BuildImprovementSuggestions(review)

		// Append DSL simulation feedback for this chapter
		if len(dslIssues) > 0 {
			chapterIssues := dslBridge.IssuesForChapter(dslIssues, chapter.ID)
			if len(chapterIssues) > 0 {
				dslFeedback := dslBridge.FormatAsString(chapterIssues)
				if dslFeedback != "" {
					suggestions += "\n\n" + dslFeedback
				}
			}
		}

		// Calculate state matrix
		stateMatrix := stateManager.CalculateStateMatrix(outline, chapter)

		// Load continuity context from previous draft chapters
		contextText := continuity.LoadPreviousDraftContext(book, outline, chapter, draftContextFlag)
		// Extract a compact recap from the immediately previous chapter (if available)
		prevRecap := continuity.LoadPreviousDraftRecap(book, outline, chapter)
		// Load next chapters for foreshadowing (default: 2 chapters ahead)
		nextChapters := continuity.LoadNextChapters(outline, chapter, 2)

		// Generate improved draft with suggestions
		draft, err := agent.GenerateDraftWithSuggestions(ctx, chapter, stateMatrix, 500, suggestions, contextText, prevRecap, nextChapters)
		if err != nil {
			log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
			return false, nil
		}

		// Apply enabled minimal-change fixers (teleport bridge, character presence)
		knownChars := project.KnownCharacterNames(outline)
		fixed, sum := continuity.ApplyImproveFixesDraft(
			log,
			book,
			workerID,
			chapter,
			outline,
			draft,
			suggestions,
			draftTeleportFixFlag,
			draftBridgeRetriesFlag,
			func(s string) (string, error) {
				return agent.GenerateDraftWithSuggestions(ctx, chapter, stateMatrix, 500, s, contextText, prevRecap, nextChapters)
			},
			draftCharacterFixFlag,
			draftCharacterPatchRetriesFlag,
			knownChars,
			func(s string) (string, error) {
				return agent.GenerateDraftWithSuggestions(ctx, chapter, stateMatrix, 500, s, contextText, prevRecap, nextChapters)
			},
		)
		draft = fixed
		log.Info("[Worker %d] Fix summary for %s: %s", workerID, chapter.ID, sum.String())

		// Save improved draft
		if err := saveDraft(book, chapter, draft); err != nil {
			log.Error("[Worker %d] Failed to save improved draft for chapter %s: %v", workerID, chapter.ID, err)
			return false, nil
		}

		log.Info("[Worker %d] Improved draft saved for chapter %s: %d words", workerID, chapter.ID, len(strings.Fields(draft)))
		return true, nil
	})

	improvedCount := 0
	for _, ok := range improved {
		if ok {
			improvedCount++
		}
	}

	return improvedCount
}
