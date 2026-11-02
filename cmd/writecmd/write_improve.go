package writecmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"novelgen/internal/app"
	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	novelcont "novelgen/internal/fiction/domain/continuity"
	"novelgen/internal/fiction/domain/logic"
	"novelgen/internal/fiction/domain/logic/continuity/recap"
	"novelgen/internal/fiction/domain/project"
	reviewstore "novelgen/internal/fiction/domain/review"
	writepipe "novelgen/internal/fiction/pipeline"
	"novelgen/internal/fiction/rpg/dsl"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/workerpool"

	"github.com/spf13/cobra"
)

// runWriteImprove improves final chapters based on review
func runWriteImprove(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()
	if err := validateWriteMinScoreFlag(); err != nil {
		return err
	}
	if err := ValidateWriteBestOfFlag(); err != nil {
		return err
	}
	if err := ValidateWriteAgentApplyOption(writeAgentSDKFlag, writeAgentApplyFlag); err != nil {
		return err
	}
	if err := ValidateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	setupWriteAgentHistoryCutoff(writeAgentHistoryFlag)

	session, err := app.Open(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	config := session.Config()
	outline, err := session.RequireOutline()
	if err != nil {
		return err
	}

	targetWords := writeTargetWords(cmd, config)

	// Create write agent
	writeAgent, err := session.NewWriteAgent()
	if err != nil {
		return err
	}
	writeAgent.SetNoRecapContext(writeNoRecapContextFlag)

	setupWriteRunLogging(book.Root(), "write improve")
	if writeAgentSDKFlag {
		log.Info("Chapter improvement will use Agent SDK; Go still validates and saves final markdown")
	}
	if writeAgentApplyFlag {
		log.Info("Agent apply enabled: SDK may write final markdown through validated chapter patch tools")
	}

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilderAt(book.Root())
	recapAgent, err := session.NewRecapAgent()
	if err != nil {
		return err
	}
	recapStore := recap.NewStoreAt(book.Root())

	// Get volumes to improve
	volumes, err := project.VolumesForSelection(outline, writeVolumeFlag, writeChapterFlag)
	if err != nil {
		return err
	}
	if len(volumes) == 0 {
		return fmt.Errorf("no volumes found to improve")
	}

	forcedAgentSDKImprove := writeAgentSDKFlag && strings.TrimSpace(writeChapterFlag) != ""

	// Auto-review flag - set to true if any volume needs review
	autoReviewNeeded := false

	// Check whether selected volumes have complete, fresh reviews.
	for _, volume := range volumes {
		if VolumeReviewNeedsRefresh(book, volume) {
			if forcedAgentSDKImprove {
				log.Info("Review for volume %s is missing or stale; Agent SDK forced improvement will use focused chapter checks instead of auto-review", volume.ID)
			} else {
				log.Info("Review for volume %s is missing or stale; will auto-review", volume.ID)
				autoReviewNeeded = true
			}
		}
	}

	// Auto-review if needed
	if autoReviewNeeded {
		log.Info("=== Auto-Review Phase ===")
		log.Info("Some volumes have missing or stale reviews, running auto-review first...")

		// Run review for volumes that need it
		for _, volume := range volumes {
			if VolumeReviewNeedsRefresh(book, volume) {
				log.Info("Auto-reviewing volume: %s", volume.ID)
				if err := reviewVolumeWithWriteAgent(ctx, book, writeAgent, continuityBuilder, volume, outline, targetWords, writeConcurrencyFlag); err != nil {
					log.Error("Failed to auto-review volume %s: %v", volume.ID, err)
					return err
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
			review, err := reviewstore.LoadVolumeReviewAt(book, volume.ID)
			if err != nil {
				if forcedAgentSDKImprove {
					log.Warn("No review found for volume %s; continuing forced Agent SDK improvement with focused check suggestions", volume.ID)
					review = &agents.VolumeReview{
						VolumeID:    volume.ID,
						VolumeTitle: volume.Title,
						Reviews:     []agents.DraftReview{},
					}
				} else {
					log.Warn("No review found for volume %s, skipping", volume.ID)
					continue
				}
			}

			// Get chapters that need improvement. An explicit chapter remains a
			// forced target, but a prompt scoped to a volume must not turn every
			// chapter into a rewrite target.
			var chaptersToImprove []*models.Chapter
			if writeChapterFlag != "" {
				chaptersToImprove = getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, false)
				log.Info("Volume %s: Force improving %d specified chapter(s)", volume.ID, len(chaptersToImprove))
			} else {
				chaptersToImprove = GetWriteChaptersNeedingImprovement(review, outline, writeMinScoreFlag)
				if len(chaptersToImprove) == 0 {
					log.Info("Volume %s: No chapters have actionable review findings", volume.ID)
					continue
				}
				if strings.TrimSpace(writePromptFlag) != "" {
					log.Info("Volume %s: Applying --prompt only to %d chapter(s) with actionable review findings", volume.ID, len(chaptersToImprove))
				} else {
					log.Info("Volume %s: Improving %d chapters", volume.ID, len(chaptersToImprove))
				}
			}

			// Improve chapters concurrently
			improved, err := improveChaptersWithWriteAgent(ctx, book, writeAgent, recapAgent, recapStore, chaptersToImprove, review.Reviews, outline, continuityBuilder, writeConcurrencyFlag, targetWords, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag)
			if err != nil {
				log.Error("Failed to improve some chapters in volume %s: %v", volume.ID, err)
				return err
			}
			improvedCount += improved
		}

		log.Info("Round %d complete: %d chapters improved", round, improvedCount)

		if improvedCount == 0 {
			log.Info("No more chapters need improvement")
			break
		}

		// Persist reviews for the new content after every improvement round,
		// including the last one. The next `write improve` can then use the
		// persisted findings instead of acting on a pre-improvement review.
		log.Info("Re-reviewing after improvements...")
		if err := runWriteReview(cmd, args); err != nil {
			return fmt.Errorf("re-review after improvements: %w", err)
		}
	}

	log.Info("Improvement process complete")
	return nil
}

func ValidateWriteAgentApplyOption(useAgentSDK, agentApply bool) error {
	if agentApply && !useAgentSDK {
		return fmt.Errorf("--agent-apply requires --agent-sdk")
	}
	return nil
}

func ValidateWriteAgentHistoryOption(useAgentSDK, agentHistory bool) error {
	if agentHistory && !useAgentSDK {
		return fmt.Errorf("--agent-history requires --agent-sdk")
	}
	return nil
}

func setupWriteAgentHistoryCutoff(agentHistory bool) {
	if !agentHistory {
		return
	}
	_ = os.Setenv("NOVELGEN_LOG_HISTORY_CUTOFF", time.Now().Format(time.RFC3339Nano))
}

// improveChaptersWithWriteAgent improves chapters using the write agent
func improveChaptersWithWriteAgent(ctx context.Context, book *project.Book, agent *agents.WriteAgent, recapAgent writepipe.RecapExtractor, recapStore *recap.Store, chapters []*models.Chapter, reviews []agents.DraftReview, outline *models.Outline, continuityBuilder *logic.ChapterContinuityBuilder, concurrency int, targetWords int, useAgentSDK bool, agentApply bool, useAgentHistory bool) (int, error) {
	log := logger.GetLogger()
	var errc writepipe.WriteErrorCollector

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

	// Create review map for quick lookup
	reviewMap := make(map[string]*agents.DraftReview)
	for i := range reviews {
		reviewMap[reviews[i].ChapterID] = &reviews[i]
	}

	improved, _ := workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (bool, error) {
		review := reviewMap[chapter.ID]
		if review == nil {
			log.Warn("[Worker %d] No existing review for forced chapter %s, using focused check suggestions", workerID, chapter.ID)
			review = fallbackDraftReviewForForcedImprove(chapter)
		}

		log.Info("[Worker %d] Improving chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load current chapter content
		currentContent, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return false, nil
		}
		if currentContent == "" {
			if useAgentSDK && agentApply {
				log.Warn("[Worker %d] No existing content for chapter %s; Agent SDK apply may create it through validated chapter patch", workerID, chapter.ID)
			} else {
				log.Error("[Worker %d] No existing content for chapter %s, skipping improvement", workerID, chapter.ID)
				errc.Addf("%s: no existing content for improvement", chapter.ID)
				return false, nil
			}
		}
		applyHumanizeCheckToReview(chapter, currentContent, review)

		// Build improvement suggestions
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

		// Append user prompt if provided
		if writePromptFlag != "" {
			suggestions += "\n\n## 用户要求\n\n" + writePromptFlag
		}
		if useAgentSDK {
			if checkSuggestions := writepipe.BuildAgentSDKChapterCheckSuggestions(book, chapter, targetWords); checkSuggestions != "" {
				suggestions += "\n\n" + checkSuggestions
			}
		}

		// Load context drafts
		context := loadWriteChapterContext(book, outline, chapter)

		// Calculate chapter continuity
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Generate improved content with suggestions
		content, err := writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, currentContent, suggestions, useAgentSDK, agentApply, useAgentHistory)
		if err != nil {
			log.Error("[Worker %d] Failed to improve chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: improve failed: %w", chapter.ID, err)
			return false, nil
		}
		content, agentApplied := writepipe.ResolveAgentAppliedChapterContent(log, book, workerID, chapter, currentContent, content, useAgentSDK, agentApply)

		// Apply enabled minimal-change fixers (teleport bridge, character presence)
		knownChars := project.KnownCharacterNames(outline)
		fixed, sum := novelcont.ApplyImproveFixesWrite(
			log,
			book,
			workerID,
			chapter,
			outline,
			content,
			suggestions,
			writeTeleportFixFlag,
			writeBridgeRetriesFlag,
			func(s string) (string, error) {
				return writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, content, s, useAgentSDK, agentApply, useAgentHistory)
			},
			writeCharacterFixFlag,
			writeCharacterPatchRetriesFlag,
			knownChars,
			func(s string) (string, error) {
				return writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, content, s, useAgentSDK, agentApply, useAgentHistory)
			},
		)
		content = fixed
		log.Info("[Worker %d] Fix summary for %s: %s", workerID, chapter.ID, sum.String())

		if !agentApplied && !FinalChapterContentChanged(chapter, currentContent, content) {
			log.Info("[Worker %d] No content changes for chapter %s; skipping save, post-save check, and recap refresh", workerID, chapter.ID)
			return false, nil
		}

		// Save improved content
		savedContent, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return false, nil
		}
		if agentApplied && strings.TrimSpace(savedContent) == strings.TrimSpace(content) {
			log.Info("[Worker %d] Agent patch already saved chapter %s through tool patch chapter --apply", workerID, chapter.ID)
		} else if err := writepipe.SaveFinalChapter(ctx, book, chapter, content); err != nil {
			log.Error("[Worker %d] Failed to save improved chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: save improved chapter failed: %w", chapter.ID, err)
			return false, nil
		}
		if useAgentSDK {
			postCheck, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords)
			if err != nil {
				log.Warn("[Worker %d] Agent SDK post-save check failed for %s: %v", workerID, chapter.ID, err)
			} else {
				writepipe.LogAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, postCheck)
				AppendAgentSDKPostSaveReviewSuggestions(review, postCheck)
			}
		}
		savedContent, readErr = book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return false, nil
		}
		if strings.TrimSpace(savedContent) != "" {
			content = savedContent
		}
		if err := writepipe.ExtractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, content, workerID, useAgentSDK); err != nil {
			log.Warn("[Worker %d] Recap refresh failed for improved chapter %s: %v", workerID, chapter.ID, err)
		} else {
			log.Info("[Worker %d] Updated recap for improved chapter %s", workerID, chapter.ID)
		}

		log.Info("[Worker %d] Improved chapter saved: %s", workerID, chapter.ID)
		return true, nil
	})

	improvedCount := 0
	for _, ok := range improved {
		if ok {
			improvedCount++
		}
	}

	return improvedCount, errc.Err()
}
