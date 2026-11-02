package writecmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"novelgen/internal/app"
	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	novelcont "novelgen/internal/fiction/domain/continuity"
	"novelgen/internal/fiction/domain/logic"
	"novelgen/internal/fiction/domain/project"
	reviewstore "novelgen/internal/fiction/domain/review"
	writepipe "novelgen/internal/fiction/pipeline"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/workerpool"

	"github.com/spf13/cobra"
)

// runWriteReview reviews final chapters and saves review results
func runWriteReview(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

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

	setupWriteRunLogging(book.Root(), "write review")
	if writeAgentSDKFlag {
		log.Info("Chapter review will use Agent SDK; Go still saves review JSON")
	}

	// Create continuity builder
	continuityBuilder := logic.NewChapterContinuityBuilderAt(book.Root())

	// Get chapters to review based on flags
	chaptersToReview := getChaptersToReview(outline, writeChapterFlag, writeVolumeFlag, writePartFlag, writeAllFlag)
	if len(chaptersToReview) == 0 {
		return fmt.Errorf("no chapters found to review")
	}

	log.Info("Reviewing %d chapter(s)", len(chaptersToReview))

	// Group chapters by volume for saving reviews
	volumeReviews := make(map[string]*agents.VolumeReview)
	volumeContents := make(map[string]map[string]string)
	var errc writepipe.WriteErrorCollector

	// Review chapters concurrently
	concurrency := writeConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chaptersToReview) {
		concurrency = len(chaptersToReview)
	}

	type volumeReviewResult struct {
		volumeID    string
		volumeTitle string
		chapterID   string
		content     string
		review      agents.DraftReview
	}

	reviewResults, _ := workerpool.Map(ctx, concurrency, chaptersToReview, func(ctx context.Context, workerID int, chapter *models.Chapter) (*volumeReviewResult, error) {
		log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load final chapter content
		content, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return nil, nil
		}
		if content == "" {
			log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
			errc.Addf("%s: no final chapter content found for review", chapter.ID)
			return nil, nil
		}

		// Load context with recap for continuity checking
		chapterContext := loadWriteChapterContext(book, outline, chapter)

		// Calculate continuity snapshot for continuity checking
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Review chapter
		var reviewResult models.ReviewResult
		var err error
		if writeAgentSDKFlag {
			reviewResult, err = writeAgent.ReviewChapterWithAgentSDKFocus(ctx, chapter, chapterContext, continuity, content, targetWords, 1, writeFocusFlag)
		} else {
			reviewResult, err = writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
		}
		if err != nil {
			log.Error("[Worker %d] Failed to review chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: review failed: %w", chapter.ID, err)
			return nil, nil
		}
		if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
			log.Warn("[Worker %d] Failed to save full write review for %s: %v", workerID, chapter.ID, err)
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
			OverallScore:  ReviewScorePercentInt(reviewResult.OverallScore),
			NeedsRevision: ReviewResultRequiresRevision(reviewResult),
			Suggestions:   suggestionStrings,
		}

		// Get volume ID for this chapter
		volumeID := getVolumeIDFromChapter(chapter.ID)
		volumeTitle := ""
		if volume := outline.GetVolumeByID(volumeID); volume != nil {
			volumeTitle = volume.Title
		}

		log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
		return &volumeReviewResult{
			volumeID:    volumeID,
			volumeTitle: volumeTitle,
			chapterID:   chapter.ID,
			content:     content,
			review:      draftReview,
		}, nil
	})

	// Merge in input order so review lists are deterministic.
	for _, result := range reviewResults {
		if result == nil {
			continue
		}
		if _, exists := volumeContents[result.volumeID]; !exists {
			volumeContents[result.volumeID] = make(map[string]string)
		}
		volumeContents[result.volumeID][result.chapterID] = result.content
		if _, exists := volumeReviews[result.volumeID]; !exists {
			volumeReviews[result.volumeID] = &agents.VolumeReview{
				VolumeID:    result.volumeID,
				VolumeTitle: result.volumeTitle,
				Reviews:     make([]agents.DraftReview, 0),
			}
		}
		volumeReviews[result.volumeID].Reviews = append(volumeReviews[result.volumeID].Reviews, result.review)
	}

	// Save all volume reviews
	for volumeID, review := range volumeReviews {
		if existing, err := reviewstore.LoadVolumeReviewAt(book, volumeID); err == nil {
			review = MergeVolumeReviewByChapter(existing, review)
		} else if !os.IsNotExist(err) {
			log.Warn("Failed to load existing review for volume %s before merge: %v", volumeID, err)
		}
		if volume := outline.GetVolumeByID(volumeID); volume != nil {
			contentByID, err := loadFinalChapterContentsForVolume(book, volume)
			if err != nil {
				log.Warn("Failed to load chapter contents for volume %s; skipping heuristic checks: %v", volumeID, err)
			} else {
				for chapterID, content := range volumeContents[volumeID] {
					contentByID[chapterID] = content
				}
				novelcont.ApplyHeuristicTransitionChecks(book, volume, contentByID, review)
				applyHumanizeChecksToVolume(book, volume, contentByID, review)
			}
		}
		if err := reviewstore.SaveVolumeReviewAt(book, review); err != nil {
			log.Error("Failed to save review for volume %s: %v", volumeID, err)
			errc.Addf("%s: save volume review failed: %w", volumeID, err)
			continue
		}
		log.Info("Review saved for volume %s: %d chapters reviewed", volumeID, len(review.Reviews))
	}

	if err := errc.Err(); err != nil {
		return err
	}

	log.Info("Review process complete")
	return nil
}

// getChaptersToReview returns chapters to review based on flags
func getChaptersToReview(outline *models.Outline, chapterFlag, volumeFlag, partFlag string, allFlag bool) []*models.Chapter {
	// If --all flag, return all chapters
	if allFlag {
		return project.AllChapters(outline)
	}

	// If chapter flag specified, return specific chapter(s)
	if chapterFlag != "" {
		chapters, err := project.SelectChapters(outline, chapterFlag, volumeFlag, partFlag, false)
		if err == nil {
			return chapters
		}
	}

	// If volume flag specified, return all chapters in volume
	if volumeFlag != "" {
		var chapters []*models.Chapter
		volumeID := writepipe.ResolveVolumeID(outline, volumeFlag, partFlag)
		for _, ch := range project.AllChapters(outline) {
			if strings.HasPrefix(ch.ID, volumeID) {
				chapters = append(chapters, ch)
			}
		}
		return chapters
	}

	// Default: return all chapters
	return project.AllChapters(outline)
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

// reviewVolumeWithWriteAgent reviews all chapters in a volume using the write agent
func reviewVolumeWithWriteAgent(ctx context.Context, book *project.Book, writeAgent *agents.WriteAgent, continuityBuilder *logic.ChapterContinuityBuilder, volume *models.Volume, outline *models.Outline, targetWords int, concurrency int) error {
	log := logger.GetLogger()

	// Get all chapters in this volume
	var chapters []*models.Chapter
	for _, ch := range project.AllChapters(outline) {
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
	volumeContents := make(map[string]string)
	var errc writepipe.WriteErrorCollector

	// Review chapters concurrently
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	type chapterReviewResult struct {
		chapterID string
		content   string
		review    agents.DraftReview
	}

	reviewResults, _ := workerpool.Map(ctx, concurrency, chapters, func(ctx context.Context, workerID int, chapter *models.Chapter) (*chapterReviewResult, error) {
		log.Info("[Worker %d] Reviewing chapter: %s - %s", workerID, chapter.ID, chapter.Title)

		// Load final chapter content
		content, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			log.Error("[Worker %d] Failed to read chapter %s: %v", workerID, chapter.ID, readErr)
			errc.Addf("%s: read final chapter failed: %w", chapter.ID, readErr)
			return nil, nil
		}
		if content == "" {
			log.Warn("[Worker %d] No final content found for chapter %s, skipping", workerID, chapter.ID)
			errc.Addf("%s: no final chapter content found for review", chapter.ID)
			return nil, nil
		}

		// Load context with recap for continuity checking
		chapterContext := loadWriteChapterContext(book, outline, chapter)

		// Calculate continuity snapshot for continuity checking
		continuity := continuityBuilder.BuildBefore(outline, chapter)

		// Review chapter
		reviewResult, err := writeAgent.ReviewChapter(ctx, chapter, chapterContext, continuity, content, targetWords, 1)
		if err != nil {
			log.Error("[Worker %d] Failed to review chapter %s: %v", workerID, chapter.ID, err)
			errc.Addf("%s: review failed: %w", chapter.ID, err)
			return nil, nil
		}
		if err := saveWriteReviewResult(book, chapter, reviewResult); err != nil {
			log.Warn("[Worker %d] Failed to save full write review for %s: %v", workerID, chapter.ID, err)
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
			OverallScore:  ReviewScorePercentInt(reviewResult.OverallScore),
			NeedsRevision: ReviewResultRequiresRevision(reviewResult),
			Suggestions:   suggestionStrings,
		}

		log.Info("[Worker %d] Reviewed chapter %s: score %.1f", workerID, chapter.ID, reviewResult.OverallScore)
		return &chapterReviewResult{chapterID: chapter.ID, content: content, review: draftReview}, nil
	})

	// Merge in input order so the saved review list is deterministic.
	for _, result := range reviewResults {
		if result == nil {
			continue
		}
		volumeContents[result.chapterID] = result.content
		volumeReview.Reviews = append(volumeReview.Reviews, result.review)
	}

	// Save volume review
	novelcont.ApplyHeuristicTransitionChecks(book, volume, volumeContents, volumeReview)
	applyHumanizeChecksToVolume(book, volume, volumeContents, volumeReview)
	if err := reviewstore.SaveVolumeReviewAt(book, volumeReview); err != nil {
		return fmt.Errorf("failed to save review for volume %s: %w", volume.ID, err)
	}
	if err := errc.Err(); err != nil {
		return err
	}

	log.Info("Review saved for volume %s: %d chapters reviewed", volume.ID, len(volumeReview.Reviews))
	return nil
}
