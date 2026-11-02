package writecmd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"novelgen/internal/app"
	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	novelcont "novelgen/internal/fiction/domain/continuity"
	"novelgen/internal/fiction/domain/logic"
	"novelgen/internal/fiction/domain/logic/continuity/recap"
	"novelgen/internal/fiction/domain/logic/style"
	"novelgen/internal/fiction/domain/project"
	reviewstore "novelgen/internal/fiction/domain/review"
	writepipe "novelgen/internal/fiction/pipeline"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/workerpool"

	"github.com/spf13/cobra"
)

func runWriteGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()
	if err := ValidateWriteAgentHistoryOption(writeAgentSDKFlag, writeAgentHistoryFlag); err != nil {
		return err
	}
	if err := ValidateWriteBestOfFlag(); err != nil {
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
	agent, err := session.NewWriteAgent()
	if err != nil {
		return err
	}
	agent.SetNoRecapContext(writeNoRecapContextFlag)

	setupWriteRunLogging(book.Root(), "write gen")

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilderAt(book.Root())
	// Create recap agent + store (auto-persist recaps for continuity)
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
	var errc writepipe.WriteErrorCollector

	// Get list of chapters to generate
	chapters, err := project.SelectChapters(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if err != nil {
		return err
	}

	log.Info("Generating final content for %d chapter(s) with concurrency %d", len(chapters), writeConcurrencyFlag)
	if WriteBestOfFlag > 1 {
		log.Info("Best-of mode enabled: %d independent copies per chapter, scored and best saved", WriteBestOfFlag)
	}
	if writeAgentSDKFlag {
		log.Info("Chapter generation will use Agent SDK; Go still validates and saves final markdown")
		if writeAgentHistoryFlag {
			log.Info("Agent history enabled: SDK must inspect queryable logs before chapter generation")
		}
	}
	if writeRecapAgentSDKFlag {
		log.Info("Automatic recap extraction will use Agent SDK; Go still validates and saves recap JSON")
	}

	// Use worker pool for concurrent generation
	concurrency := writeConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	_, _ = workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (struct{}, error) {
		log.Info("[Worker %d] Generating content for chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load context drafts (previous and next chapters)
		context := loadWriteChapterContext(book, outline, chapter)

		// Calculate chapter continuity
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Generate final content (optionally best-of-N with AI scoring)
		content, err := GenerateChapterContentForRun(ctx, agent, scorer, chapter, context, continuity, targetWords, WriteBestOfFlag, writeAgentSDKFlag, writeAgentHistoryFlag)
		if err != nil {
			log.Error("Failed to generate content for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: generate failed: %w", chapter.ID, err)
			return struct{}{}, nil
		}

		// Save final content
		if err := writepipe.SaveFinalChapter(ctx, book, chapter, content); err != nil {
			log.Error("Failed to save content for chapter %s: %v", chapter.ID, err)
			errc.Addf("%s: save final chapter failed: %w", chapter.ID, err)
			return struct{}{}, nil
		}
		savedContent, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("Failed to read back chapter %s: %v", chapter.ID, readErr)
			errc.Addf("%s: read back final chapter failed: %w", chapter.ID, readErr)
			return struct{}{}, nil
		}
		if strings.TrimSpace(savedContent) != "" {
			content = savedContent
		}
		if writeAgentSDKFlag {
			if result, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords); err != nil {
				log.Warn("[Worker %d] Agent SDK post-save check failed for %s: %v", workerID, chapter.ID, err)
			} else {
				writepipe.LogAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, result)
				if result.Blocking {
					log.Warn("[Worker %d] Agent SDK generated chapter %s still has blocking post-save issues; attempting one validated agent repair", workerID, chapter.ID)
					repaired, repairedCheck, repairErr := repairAgentSDKGeneratedChapterPostSave(ctx, book, log, workerID, agent, chapter, context, continuity, targetWords, content, result, writeAgentHistoryFlag)
					if repairErr != nil {
						log.Error("[Worker %d] Agent SDK post-save repair failed for %s: %v", workerID, chapter.ID, repairErr)
						errc.Addf("%s: post-save repair failed: %w", chapter.ID, repairErr)
						return struct{}{}, nil
					}
					content = repaired
					writepipe.LogAgentSDKChapterPostSaveCheck(log, workerID, chapter.ID, repairedCheck)
					if repairedCheck != nil && repairedCheck.Blocking {
						errc.Addf("%s: post-save repair still has blocking issues", chapter.ID)
						return struct{}{}, nil
					}
				}
			}
		}

		// Auto-extract + persist recap for this final chapter (best-effort)
		if err := writepipe.ExtractAndSaveRecapWithGate(ctx, recapAgent, recapStore, chapter, content, workerID, writeRecapAgentSDKFlag); err != nil {
			log.Warn("[Worker %d] Recap gate failed for %s: %v", workerID, chapter.ID, err)
		}

		log.Info("[Worker %d] Content saved for chapter %s: %d narrative units", workerID, chapter.ID, writepipe.ChapterNarrativeUnitsForLog(chapter, content))
		return struct{}{}, nil
	})

	if err := errc.Err(); err != nil {
		return err
	}

	log.Info("Chapter writing complete")
	return nil
}

// GenerateChapterContentForRun produces chapter content either through the
// existing single-copy path or, when --best-of is enabled, through the
// parallel best-of-N generation + AI scoring flow.
func GenerateChapterContentForRun(
	ctx context.Context,
	agent *agents.WriteAgent,
	scorer *agents.ChapterScorerAgent,
	chapter *models.Chapter,
	context *agents.ChapterContext,
	continuity *models.ChapterContinuity,
	targetWords int,
	bestOf int,
	useAgentSDK bool,
	useHistory bool,
) (string, error) {
	if bestOf <= 1 {
		if useAgentSDK {
			return agent.GenerateChapterWithAgentSDK(ctx, chapter, context, continuity, targetWords, useHistory)
		}
		return agent.GenerateChapter(ctx, chapter, context, continuity, targetWords)
	}
	return GenerateChapterBestOf(ctx, agent, scorer, chapter, context, continuity, targetWords, bestOf, useAgentSDK, useHistory)
}

// GenerateChapterBestOf generates N independent copies of the same chapter in
// parallel, scores each successful copy with the lightweight reader scorer,
// and returns the highest-scoring content. Failures are isolated per copy:
// a failed copy is skipped, and if every scoring attempt fails the first
// successfully generated copy is used so generation can still proceed.
func GenerateChapterBestOf(
	ctx context.Context,
	agent *agents.WriteAgent,
	scorer *agents.ChapterScorerAgent,
	chapter *models.Chapter,
	context *agents.ChapterContext,
	continuity *models.ChapterContinuity,
	targetWords int,
	bestOf int,
	useAgentSDK bool,
	useHistory bool,
) (string, error) {
	log := logger.GetLogger()

	type generationResult struct {
		index   int
		content string
		err     error
	}
	results := make([]generationResult, bestOf)
	var genWg sync.WaitGroup
	for i := 0; i < bestOf; i++ {
		genWg.Add(1)
		go func(index int) {
			defer genWg.Done()
			log.Info("[BestOf] Generating copy %d/%d for chapter %s", index+1, bestOf, chapter.ID)
			var content string
			var err error
			if useAgentSDK {
				content, err = agent.GenerateChapterWithAgentSDK(ctx, chapter, context, continuity, targetWords, useHistory)
			} else {
				content, err = agent.GenerateChapter(ctx, chapter, context, continuity, targetWords)
			}
			results[index] = generationResult{index: index, content: content, err: err}
		}(i)
	}
	genWg.Wait()

	type scoredResult struct {
		index   int
		content string
		score   int
		reason  string
		err     error
	}
	copies := make([]scoredResult, 0, bestOf)
	for _, result := range results {
		if result.err != nil {
			log.Error("[BestOf] Copy %d/%d generation failed for chapter %s: %v", result.index+1, bestOf, chapter.ID, result.err)
			continue
		}
		copies = append(copies, scoredResult{index: result.index, content: result.content})
	}
	if len(copies) == 0 {
		return "", fmt.Errorf("all %d best-of copies failed to generate for chapter %s", bestOf, chapter.ID)
	}

	// Score all successful copies in parallel; each scoring call is one cheap
	// LLM request.
	var scoreWg sync.WaitGroup
	for i := range copies {
		scoreWg.Add(1)
		go func(c *scoredResult) {
			defer scoreWg.Done()
			output, err := scorer.ScoreChapter(ctx, BuildChapterScoreInput(chapter, c.content, context))
			if err != nil {
				c.err = err
				return
			}
			c.score = int(output.Score)
			c.reason = output.Reason
		}(&copies[i])
	}
	scoreWg.Wait()

	scored := make([]agents.ScoredChapterCopy, 0, len(copies))
	for _, c := range copies {
		if c.err != nil {
			log.Error("[BestOf] Scoring copy %d/%d failed for chapter %s: %v", c.index+1, bestOf, chapter.ID, c.err)
			continue
		}
		scored = append(scored, agents.ScoredChapterCopy{
			CopyIndex: c.index,
			Content:   c.content,
			Score:     c.score,
			Reason:    c.reason,
		})
	}

	if len(scored) == 0 {
		log.Warn("[BestOf] All scoring attempts failed for chapter %s; falling back to copy %d/%d", chapter.ID, copies[0].index+1, bestOf)
		return copies[0].content, nil
	}

	best, err := agents.SelectBestScoredChapter(scored)
	if err != nil {
		return "", fmt.Errorf("best-of selection failed for chapter %s: %w", chapter.ID, err)
	}

	log.Info("[BestOf] Chapter %s copy score comparison:", chapter.ID)
	for _, c := range scored {
		marker := " "
		if c.CopyIndex == best.CopyIndex {
			marker = "*"
		}
		log.Info("[BestOf]   %s copy %d/%d: score %d/100 - %s", marker, c.CopyIndex+1, bestOf, c.Score, c.Reason)
	}
	log.Info("[BestOf] Selected copy %d/%d (score %d/100) for chapter %s", best.CopyIndex+1, bestOf, best.Score, chapter.ID)
	return best.Content, nil
}

// BuildChapterScoreInput builds the lightweight scorer input from the outline
// chapter and its generated content. Outline info is optional: only the
// chapter's own summary/beats and the immediate next chapter's summary are
// included so the scorer can judge beat coverage and the ending hook.
func BuildChapterScoreInput(chapter *models.Chapter, content string, context *agents.ChapterContext) agents.ChapterScoreInput {
	input := agents.ChapterScoreInput{
		ChapterID:      chapter.ID,
		ChapterTitle:   chapter.Title,
		ChapterContent: content,
		ChapterSummary: chapter.Summary,
		ChapterBeats:   chapter.GetBeats(),
	}
	if context != nil && len(context.Next) > 0 && context.Next[0] != nil && context.Next[0].Chapter != nil {
		input.NextChapterSummary = context.Next[0].Chapter.Summary
	}
	return input
}

func FinalChapterContentChanged(chapter *models.Chapter, before, after string) bool {
	return project.ChapterContentChanged(chapter, before, after)
}

func saveWriteReviewResult(book *project.Book, chapter *models.Chapter, review models.ReviewResult) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	if book == nil {
		return fmt.Errorf("project is required")
	}
	return book.Root().WriteJSON(book.Root().ReviewFile(chapter.ID+"_write_review.json"), review)
}

func loadFinalChapterContentsForVolume(book *project.Book, volume *models.Volume) (map[string]string, error) {
	contents := make(map[string]string)
	if volume == nil {
		return contents, nil
	}
	for i := range volume.Chapters {
		ch := &volume.Chapters[i]
		content, readErr := book.LoadFinalChapterContent(ch)
		if readErr != nil {
			return nil, readErr
		}
		if strings.TrimSpace(content) != "" {
			contents[ch.ID] = content
		}
	}
	return contents, nil
}

func findDraftReview(review *agents.VolumeReview, chapterID string) *agents.DraftReview {
	if review == nil {
		return nil
	}
	for i := range review.Reviews {
		if review.Reviews[i].ChapterID == chapterID {
			return &review.Reviews[i]
		}
	}
	return nil
}

func MergeVolumeReviewByChapter(existing, incoming *agents.VolumeReview) *agents.VolumeReview {
	if incoming == nil {
		return existing
	}
	if existing == nil || existing.VolumeID == "" {
		return incoming
	}
	if incoming.VolumeID != "" && existing.VolumeID != incoming.VolumeID {
		return incoming
	}

	merged := *existing
	if incoming.VolumeID != "" {
		merged.VolumeID = incoming.VolumeID
	}
	if incoming.VolumeTitle != "" {
		merged.VolumeTitle = incoming.VolumeTitle
	}
	if incoming.Summary != "" {
		merged.Summary = incoming.Summary
	}

	indexByChapter := make(map[string]int, len(merged.Reviews))
	for i := range merged.Reviews {
		indexByChapter[merged.Reviews[i].ChapterID] = i
	}
	for _, review := range incoming.Reviews {
		if i, ok := indexByChapter[review.ChapterID]; ok {
			merged.Reviews[i] = review
			continue
		}
		indexByChapter[review.ChapterID] = len(merged.Reviews)
		merged.Reviews = append(merged.Reviews, review)
	}
	return &merged
}

// LoadReusableWriteReview returns a persisted volume review only when it was
// written after the chapter content it describes. This uses the artifacts
// themselves as the pipeline checkpoint, so an interrupted run resumes at the
// first stale or missing step without adding a second state file.
func LoadReusableWriteReview(book *project.Book, chapter *models.Chapter, chapterPath string) (*agents.VolumeReview, *agents.DraftReview, bool) {
	if book == nil || chapter == nil || strings.TrimSpace(chapterPath) == "" {
		return nil, nil, false
	}
	volumeID := getVolumeIDFromChapter(chapter.ID)
	if volumeID == "" {
		return nil, nil, false
	}
	review, err := reviewstore.LoadVolumeReviewAt(book, volumeID)
	if err != nil {
		return nil, nil, false
	}
	draftReview := findDraftReview(review, chapter.ID)
	if draftReview == nil {
		return nil, nil, false
	}
	chapterInfo, err := book.Root().Stat(chapterPath)
	if err != nil {
		return nil, nil, false
	}
	// The volume review is a compatibility container for every chapter in the
	// volume, so its mtime changes when any sibling is reviewed. The per-chapter
	// full-review file is the precise freshness checkpoint for this chapter.
	chapterReviewPath := book.Root().ReviewFile(chapter.ID + "_write_review.json")
	reviewInfo, err := book.Root().Stat(chapterReviewPath)
	if err != nil {
		return nil, nil, false
	}
	if chapterInfo.ModTime().After(reviewInfo.ModTime()) {
		return nil, nil, false
	}
	return review, draftReview, true
}

// VolumeReviewNeedsRefresh reports whether a selected volume has a chapter
// whose saved markdown is newer than its per-chapter review (or lacks a review
// entirely).
func VolumeReviewNeedsRefresh(book *project.Book, volume *models.Volume) bool {
	if book == nil || volume == nil {
		return true
	}
	if _, err := reviewstore.LoadVolumeReviewAt(book, volume.ID); err != nil {
		return true
	}
	for i := range volume.Chapters {
		chapter := &volume.Chapters[i]
		_, chapterPath, err := book.Root().LoadChapterContent(chapter.ID)
		if err != nil {
			return true
		}
		if strings.TrimSpace(chapterPath) == "" {
			continue
		}
		if _, _, ok := LoadReusableWriteReview(book, chapter, chapterPath); !ok {
			return true
		}
	}
	return false
}

// PipelineChapterComplete reports whether an unchanged chapter has a clean
// persisted review and a minimally valid recap. When DSL emission is enabled,
// the chapter DSL artifact must also exist.
func PipelineChapterComplete(book *project.Book, chapter *models.Chapter, chapterPath string, review *agents.DraftReview, minScorePercent int, requireDSL bool) bool {
	if book == nil || chapter == nil || review == nil {
		return false
	}
	if review.OverallScore <= 0 {
		return false
	}
	if DraftReviewNeedsImprovement(*review, minScorePercent) {
		return false
	}
	ok, _, _ := recap.CheckQualityAt(book.Root(), chapter.ID)
	if !ok {
		return false
	}
	if requireDSL {
		chapterInfo, chapterErr := book.Root().Stat(chapterPath)
		dslInfo, dslErr := book.Root().Stat(book.Root().RPGChaptersDSL())
		if chapterErr != nil || dslErr != nil || dslInfo.ModTime().Before(chapterInfo.ModTime()) {
			return false
		}
	}
	return true
}

func applyHumanizeCheckToReview(chapter *models.Chapter, content string, review *agents.DraftReview) {
	if !writeHumanizeFlag || review == nil || strings.TrimSpace(content) == "" {
		return
	}
	result := style.CheckAIFlavor(content, writeHumanizeThresholdFlag)
	if len(result.Issues) == 0 && len(result.Suggestions) == 0 {
		return
	}

	if review.PacingReview.Score == 0 {
		review.PacingReview.Score = 8
	}
	for _, issue := range result.Issues {
		msg := fmt.Sprintf("去AI味检查（score=%d）：%s", result.Score, issue)
		if !novelcont.ContainsStr(review.PacingReview.Issues, msg) {
			review.PacingReview.Issues = append(review.PacingReview.Issues, msg)
		}
	}
	for _, suggestion := range result.Suggestions {
		msg := "【去AI味】" + suggestion
		if !novelcont.ContainsStr(review.PacingReview.Suggestions, msg) {
			review.PacingReview.Suggestions = append(review.PacingReview.Suggestions, msg)
		}
	}
	if result.HasIssue {
		review.NeedsRevision = true
		if review.OverallScore == 0 || review.OverallScore > 7 {
			review.OverallScore = 7
		}
		if chapter != nil {
			summary := fmt.Sprintf("[medium] style: 本章%s去AI味得分 %d，建议按“去AI味”条目改写公式化表达。", chapter.ID, result.Score)
			if !novelcont.ContainsStr(review.Suggestions, summary) {
				review.Suggestions = append(review.Suggestions, summary)
			}
		}
	}
}

func applyHumanizeChecksToVolume(book *project.Book, volume *models.Volume, contents map[string]string, review *agents.VolumeReview) {
	if !writeHumanizeFlag || volume == nil || review == nil {
		return
	}
	contentByID := contents
	if contentByID == nil {
		loaded, err := loadFinalChapterContentsForVolume(book, volume)
		if err != nil {
			logger.GetLogger().Warn("Cannot load chapter contents for volume %s; skipping heuristic checks: %v", volume.ID, err)
			loaded = map[string]string{}
		}
		contentByID = loaded
	}
	chapterByID := make(map[string]*models.Chapter)
	for i := range volume.Chapters {
		chapterByID[volume.Chapters[i].ID] = &volume.Chapters[i]
	}
	for i := range review.Reviews {
		r := &review.Reviews[i]
		applyHumanizeCheckToReview(chapterByID[r.ChapterID], contentByID[r.ChapterID], r)
	}
}
