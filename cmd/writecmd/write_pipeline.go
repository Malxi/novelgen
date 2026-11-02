package writecmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"novelgen/internal/app"
	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/derive"
	novelcont "novelgen/internal/fiction/domain/continuity"
	"novelgen/internal/fiction/domain/logic"
	"novelgen/internal/fiction/domain/logic/continuity/recap"
	"novelgen/internal/fiction/domain/project"
	reviewstore "novelgen/internal/fiction/domain/review"
	writepipe "novelgen/internal/fiction/pipeline"
	"novelgen/internal/fiction/rpg/dsl"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"

	"github.com/spf13/cobra"
)

// runWritePipeline runs the complete writing pipeline: gen -> review -> improve -> recap
func runWritePipeline(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()
	if err := validateWriteMinScoreFlag(); err != nil {
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
	setup, err := session.RequireSetup()
	if err != nil {
		return err
	}
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

	setupWriteRunLogging(book.Root(), "write pipeline")

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilderAt(book.Root())

	// Get chapters to process
	chaptersToProcess := getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if len(chaptersToProcess) == 0 {
		return fmt.Errorf("no chapters found to process")
	}

	log.Info("=== WRITE PIPELINE ===")
	log.Info("Processing %d chapter(s)", len(chaptersToProcess))
	log.Info("RPG DSL export: enabled=%t batch_size=%d output=%s", writeEmitRPGDSLFlag, writeRPGBatchSizeFlag, book.Root().RPGChaptersDSL())
	minScorePercent := writeMinScorePercent()
	log.Info("Minimum acceptable score: %.0f/100", minScorePercent)
	if WriteBestOfFlag > 1 {
		log.Info("Best-of mode enabled: %d independent copies per chapter, scored and best saved", WriteBestOfFlag)
	}
	if writeAgentSDKFlag {
		log.Info("Chapter generation/review/improvement will use Agent SDK; Go still validates and saves final markdown and review JSON")
		if writeAgentHistoryFlag {
			log.Info("Agent history enabled: SDK must inspect queryable logs before writing/improving")
		}
	}
	if writeAgentApplyFlag {
		log.Info("Agent apply enabled: SDK may write final markdown through validated chapter patch tools")
	}
	usePipelineRecapAgentSDK := writeRecapAgentSDKFlag || writeAgentSDKFlag
	if usePipelineRecapAgentSDK {
		log.Info("Final recap extraction will use Agent SDK; Go still validates and saves recap JSON")
	}

	// Create recap agent for all chapters
	recapAgent, err := session.NewRecapAgent()
	if err != nil {
		return err
	}
	recapStore := recap.NewStoreAt(book.Root())
	// Create lightweight best-of scorer (used only when --best-of is enabled)
	scorer, err := session.NewChapterScorerAgent()
	if err != nil {
		return err
	}

	// RPG DSL issues are refreshed after chapter content changes. Running this
	// before generation would analyze stale or missing chapter files.
	var rpgDSLIssues []dsl.SimulationIssue
	knownChars := project.KnownCharacterNames(outline)
	var errc writepipe.WriteErrorCollector

	// Process each chapter completely before moving to the next
	for chapterIdx, chapter := range chaptersToProcess {
		log.Info("\n%s", strings.Repeat("=", 60))
		log.Info("Processing Chapter %d/%d: %s - %s", chapterIdx+1, len(chaptersToProcess), chapter.ID, chapter.Title)
		log.Info("%s", strings.Repeat("=", 60))

		// Step 1: Generate chapter (if not exists)
		log.Info("\n[Step 1/4] Generating chapter...")
		content, chapterPath, readErr := book.Root().LoadChapterContent(chapter.ID)
		if readErr != nil {
			log.Error("Failed to read chapter %s: %v", chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			continue
		}
		if content == "" {
			log.Info("Generating chapter: %s - %s", chapter.ID, chapter.Title)
			chapterContext := loadWriteChapterContext(book, outline, chapter)
			continuity := continuityBuilder.BuildBefore(outline, chapter)
			generatedContent, err := GenerateChapterContentForRun(ctx, writeAgent, scorer, chapter, chapterContext, continuity, targetWords, WriteBestOfFlag, writeAgentSDKFlag, writeAgentHistoryFlag)
			if err != nil {
				log.Error("Failed to generate chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: generate failed: %w", chapter.ID, err)
				continue
			}
			if err := writepipe.SaveFinalChapter(ctx, book, chapter, generatedContent); err != nil {
				log.Error("Failed to save chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save generated chapter failed: %w", chapter.ID, err)
				continue
			}
			content, chapterPath, readErr = book.Root().LoadChapterContent(chapter.ID)
			if readErr != nil {
				log.Error("Failed to read back chapter %s: %v", chapter.ID, readErr)
				errc.Addf("%s: read back final chapter failed: %w", chapter.ID, readErr)
				continue
			}
			if strings.TrimSpace(content) == "" {
				content = generatedContent
			}
			log.Info("Generated chapter output: %s", finalChapterPath(book.Root(), chapter))
		} else {
			log.Info("Chapter %s already exists, using: %s", chapter.ID, finalChapterPath(book.Root(), chapter))
		}

		volumeID := getVolumeIDFromChapter(chapter.ID)
		volume := outline.GetVolumeByID(volumeID)
		reviewPath := book.Root().VolumeReviewFile(volumeID)
		volumeReview, chapterReview, reviewReused := LoadReusableWriteReview(book, chapter, chapterPath)
		var reviewResult models.ReviewResult
		recapAlreadyValid, _, _ := recap.CheckQualityAt(book.Root(), chapter.ID)
		if reviewReused {
			if PipelineChapterComplete(book, chapter, chapterPath, chapterReview, writeMinScoreFlag, writeEmitRPGDSLFlag) {
				log.Info("Chapter %s is already complete (fresh review, valid recap, derived DSL); skipping", chapter.ID)
				continue
			}
			log.Info("Reusing fresh review for %s from %s", chapter.ID, reviewPath)
		}

		// Step 2: Review chapter
		log.Info("\n[Step 2/4] Reviewing chapter...")
		if !reviewReused {
			chapterContext := loadWriteChapterContext(book, outline, chapter)
			continuity := continuityBuilder.BuildBefore(outline, chapter)
			var err error
			if writeAgentSDKFlag {
				reviewResult, err = writeAgent.ReviewChapterWithAgentSDK(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
			} else {
				reviewResult, err = writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
			}
			if err != nil {
				log.Error("Failed to review chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: review failed: %w", chapter.ID, err)
				continue
			}
			if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
				log.Warn("Failed to save full write review for %s: %v", chapter.ID, err)
			}

			var suggestionStrings []string
			for _, s := range reviewResult.Suggestions {
				suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
			}
			draftReview := agents.DraftReview{
				ChapterID:     chapter.ID,
				ChapterTitle:  chapter.Title,
				OverallScore:  ReviewScorePercentInt(reviewResult.OverallScore),
				NeedsRevision: ReviewResultRequiresRevision(reviewResult),
				Suggestions:   suggestionStrings,
			}
			var existingReview agents.VolumeReview
			if err := book.Root().ReadJSON(reviewPath, &existingReview); err == nil {
				volumeReview = &existingReview
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
			if volume != nil {
				volumeContents, err := loadFinalChapterContentsForVolume(book, volume)
				if err != nil {
					log.Warn("Failed to load chapter contents for volume %s; skipping heuristic checks: %v", volume.ID, err)
				} else {
					novelcont.ApplyHeuristicTransitionChecks(book, volume, volumeContents, volumeReview)
					applyHumanizeChecksToVolume(book, volume, volumeContents, volumeReview)
				}
			}
			if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
				log.Error("Failed to save review for chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save volume review failed: %w", chapter.ID, err)
			}
			log.Info("Review output: %s", reviewPath)
			log.Info("Reviewed chapter %s: score %.1f", chapter.ID, reviewResult.OverallScore)
			chapterReview = findDraftReview(volumeReview, chapter.ID)
		}
		if chapterReview == nil {
			log.Error("No usable review found for chapter %s", chapter.ID)
			errc.Addf("%s: no usable review found", chapter.ID)
			continue
		}

		if writeEmitRPGDSLFlag && writeMaxRoundsFlag > 0 {
			result, err := refreshWriteRPGDSL(ctx, book.Root(), config, session.LLMConfig(), setup, session.Client(), writeRPGBatchSizeFlag, "before improvement")
			if err != nil {
				log.Error("Failed to refresh RPG DSL before improving chapter %s: %v", chapter.ID, err)
			} else {
				rpgDSLIssues = result.Issues
			}
		}

		// Step 3: Improve chapter (iterative)
		log.Info("\n[Step 3/4] Improving chapter...")
		currentContent := content
		forceChapterImprovement := strings.TrimSpace(writeChapterFlag) != ""
		for round := 1; round <= writeMaxRoundsFlag; round++ {
			// Check for RPG DSL issues for this chapter
			chapterRPGIssues := derive.GetChapterSpecificSuggestions(rpgDSLIssues, chapter.ID)
			hasRPGIssues := chapterRPGIssues != ""

			if !forceChapterImprovement && !DraftReviewNeedsImprovement(*chapterReview, writeMinScoreFlag) && !hasRPGIssues {
				log.Info("Chapter %s: no actionable review findings, skipping improvement", chapter.ID)
				break
			}

			log.Info("Improvement round %d/%d (score: %d, RPG issues: %v)", round, writeMaxRoundsFlag, chapterReview.OverallScore, hasRPGIssues)
			suggestions := reviewstore.BuildImprovementSuggestions(chapterReview)

			// Append RPG DSL issues as suggestions if any
			if hasRPGIssues {
				suggestions += "\n\n" + chapterRPGIssues
			}
			if writeAgentSDKFlag {
				if checkSuggestions := writepipe.BuildAgentSDKChapterCheckSuggestions(book, chapter, targetWords); checkSuggestions != "" {
					suggestions += "\n\n" + checkSuggestions
				}
			}
			chapterContext := loadWriteChapterContext(book, outline, chapter)
			continuity := continuityBuilder.BuildBefore(outline, chapter)
			improvedContent, err := writepipe.GenerateImprovedChapter(book, ctx, writeAgent, chapter, chapterContext, continuity, targetWords, currentContent, suggestions, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
			if err != nil {
				log.Error("Failed to improve chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: improve round %d failed: %w", chapter.ID, round, err)
				break
			}
			improvedContent, agentApplied := writepipe.ResolveAgentAppliedChapterContent(log, book, 0, chapter, currentContent, improvedContent, writeAgentSDKFlag, writeAgentApplyFlag)

			fixedContent, sum := novelcont.ApplyImproveFixesWrite(
				log,
				book,
				0,
				chapter,
				outline,
				improvedContent,
				suggestions,
				writeTeleportFixFlag,
				writeBridgeRetriesFlag,
				func(s string) (string, error) {
					return writepipe.GenerateImprovedChapter(book, ctx, writeAgent, chapter, chapterContext, continuity, targetWords, improvedContent, s, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
				},
				writeCharacterFixFlag,
				writeCharacterPatchRetriesFlag,
				knownChars,
				func(s string) (string, error) {
					return writepipe.GenerateImprovedChapter(book, ctx, writeAgent, chapter, chapterContext, continuity, targetWords, improvedContent, s, writeAgentSDKFlag, writeAgentApplyFlag, writeAgentHistoryFlag, round)
				},
			)
			improvedContent = fixedContent
			log.Info("Fix summary for %s: %s", chapter.ID, sum.String())

			if !agentApplied && !FinalChapterContentChanged(chapter, currentContent, improvedContent) {
				log.Info("No content changes for chapter %s; skipping save and post-save check", chapter.ID)
				break
			}

			savedContent, readErr := book.LoadFinalChapterContent(chapter)
			if readErr != nil {
				log.Error("Failed to read chapter %s: %v", chapter.ID, readErr)
				errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
				break
			}
			if agentApplied && strings.TrimSpace(savedContent) == strings.TrimSpace(improvedContent) {
				log.Info("Agent patch already saved chapter %s through tool patch chapter --apply", chapter.ID)
			} else if err := writepipe.SaveFinalChapter(ctx, book, chapter, improvedContent); err != nil {
				log.Error("Failed to save improved chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save improved chapter failed: %w", chapter.ID, err)
				break
			}
			if writeAgentSDKFlag {
				postCheck, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords)
				if err != nil {
					log.Warn("Agent SDK post-save check failed for %s: %v", chapter.ID, err)
				} else {
					writepipe.LogAgentSDKChapterPostSaveCheck(log, 0, chapter.ID, postCheck)
					AppendAgentSDKPostSaveReviewSuggestions(chapterReview, postCheck)
					if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
						log.Warn("Failed to save Agent SDK post-save review suggestions for %s: %v", chapter.ID, err)
					}
				}
			}
			currentContent = improvedContent
			log.Info("Improved chapter output: %s", finalChapterPath(book.Root(), chapter))

			// Persist a review for the new content even on the final round so a
			// resumed pipeline can recognize the chapter as complete.
			log.Info("Re-reviewing chapter %s after improvement...", chapter.ID)
			if writeAgentSDKFlag {
				reviewResult, err = writeAgent.ReviewChapterWithAgentSDK(ctx, chapter, chapterContext, continuity, currentContent, targetWords, 1)
			} else {
				reviewResult, err = writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, currentContent, targetWords, 1)
			}
			if err != nil {
				log.Error("Failed to re-review chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: re-review after improvement failed: %w", chapter.ID, err)
				break
			}
			if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
				log.Warn("Failed to save full write review for %s: %v", chapter.ID, err)
			}
			var suggestionStrings []string
			for _, s := range reviewResult.Suggestions {
				suggestionStrings = append(suggestionStrings, fmt.Sprintf("[%s] %s: %s", s.Priority, s.Category, s.Suggestion))
			}
			chapterReview.OverallScore = ReviewScorePercentInt(reviewResult.OverallScore)
			chapterReview.NeedsRevision = ReviewResultRequiresRevision(reviewResult)
			chapterReview.Suggestions = suggestionStrings
			if volume != nil {
				volumeContents, err := loadFinalChapterContentsForVolume(book, volume)
				if err != nil {
					log.Warn("Failed to load chapter contents for volume %s; skipping heuristic checks: %v", volume.ID, err)
				} else {
					novelcont.ApplyHeuristicTransitionChecks(book, volume, volumeContents, volumeReview)
					applyHumanizeChecksToVolume(book, volume, volumeContents, volumeReview)
					if updated := findDraftReview(volumeReview, chapter.ID); updated != nil {
						chapterReview = updated
					}
				}
			}
			// Save updated review
			if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
				log.Error("Failed to save updated review for chapter %s: %v", chapter.ID, err)
				errc.Addf("%s: save updated review failed: %w", chapter.ID, err)
			}
			// Scores are diagnostic; stop only when the re-review no longer
			// reports concrete medium-or-higher findings.
			if !DraftReviewNeedsImprovement(*chapterReview, writeMinScoreFlag) {
				log.Info("No actionable review findings remain (score %.1f), stopping improvement", reviewResult.OverallScore)
				break
			}
		}

		// Step 4: Generate recap
		log.Info("\n[Step 4/4] Generating recap...")
		contentChanged := FinalChapterContentChanged(chapter, content, currentContent)
		if recapAlreadyValid && !contentChanged {
			log.Info("Recap for %s is already current; skipping extraction", chapter.ID)
		} else if err := writepipe.ExtractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, currentContent, 0, usePipelineRecapAgentSDK); err != nil {
			log.Error("Failed to generate/save recap for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: final recap failed: %w", chapter.ID, err)
		} else {
			log.Info("Recap output: %s", book.Root().RecapFile(chapter.ID))
		}

		if writeEmitRPGDSLFlag {
			result, err := refreshWriteRPGDSL(ctx, book.Root(), config, session.LLMConfig(), setup, session.Client(), writeRPGBatchSizeFlag, "after final recap")
			if err != nil {
				log.Error("Failed to refresh final RPG DSL after chapter %s recap: %v", chapter.ID, err)
			} else {
				rpgDSLIssues = result.Issues
			}
		}

		log.Info("\nCompleted chapter: %s", chapter.ID)
	}

	if err := errc.Err(); err != nil {
		log.Info("\n=== PIPELINE COMPLETE WITH ERRORS ===")
		return err
	}

	log.Info("\n=== PIPELINE COMPLETE ===")
	log.Info("All phases completed successfully!")
	return nil
}

func refreshWriteRPGDSL(
	ctx context.Context,
	root *store.Root,
	config *models.ProjectConfig,
	cfg *llm.Config,
	setup *models.StorySetup,
	client llm.Client,
	batchSize int,
	reason string,
) (derive.WriteRPGDSLResult, error) {
	log := logger.GetLogger()
	log.Info("\n[RPG DSL] Refreshing chapter DSL (%s)...", reason)
	result, err := derive.EmitChapterRPGDSLForProject(ctx, root, filepath.Base(root.Dir()), config, cfg, setup, client, batchSize)
	if err != nil {
		return result, err
	}
	log.Info("[RPG DSL] Output: %s", result.DSLPath)
	log.Info("[RPG DSL] Batches: %d reused, %d generated", result.ReusedBatches, result.GeneratedBatches)
	log.Info("[RPG DSL] Issues detected: %d total, %d important", len(result.Issues), len(derive.FilterImportantIssues(result.Issues)))
	return result, nil
}
