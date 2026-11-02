package writecmd

import (
	"context"
	"fmt"
	"strings"

	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	novelcont "novelgen/internal/fiction/domain/continuity"
	"novelgen/internal/fiction/domain/project"
	writepipe "novelgen/internal/fiction/pipeline"
	"novelgen/internal/platform/logger"
	"novelgen/internal/toolplane"
)

func fallbackDraftReviewForForcedImprove(chapter *models.Chapter) *agents.DraftReview {
	review := &agents.DraftReview{
		OverallScore:  7,
		NeedsRevision: true,
		Suggestions: []string{
			"User selected this chapter for focused improvement. Preserve the established outline facts and repair only issues found by deterministic chapter checks.",
		},
	}
	if chapter != nil {
		review.ChapterID = strings.TrimSpace(chapter.ID)
		review.ChapterTitle = strings.TrimSpace(chapter.Title)
	}
	return review
}

func repairAgentSDKGeneratedChapterPostSave(ctx context.Context, book *project.Book, log logger.LoggerInterface, workerID int, agent *agents.WriteAgent, chapter *models.Chapter, context *agents.ChapterContext, continuity *models.ChapterContinuity, targetWords int, currentContent string, check *toolplane.ToolCheckResult, useAgentHistory bool) (string, *toolplane.ToolCheckResult, error) {
	if agent == nil {
		return "", nil, fmt.Errorf("write agent is nil")
	}
	if chapter == nil {
		return "", nil, fmt.Errorf("chapter is nil")
	}
	suggestions := "## Agent SDK Post-save Repair\n\n" +
		"The just-generated chapter was saved, but deterministic post-save checks still found blocking issues. " +
		"Repair only the returned check issues. Use validated `tool patch chapter` dry-run and then the matching `--apply --refresh-derived` command. " +
		"Keep the existing chapter shape and prose volume unless the check issue is explicitly about length.\n\n" +
		writepipe.FormatAgentSDKChapterCheckSuggestions(check)

	repaired, err := writepipe.GenerateImprovedChapter(book, ctx, agent, chapter, context, continuity, targetWords, currentContent, suggestions, true, true, useAgentHistory, 1)
	if err != nil {
		return "", nil, err
	}
	repaired, agentApplied := writepipe.ResolveAgentAppliedChapterContent(log, book, workerID, chapter, currentContent, repaired, true, true)
	if !agentApplied {
		if err := writepipe.SaveFinalChapter(ctx, book, chapter, repaired); err != nil {
			return "", nil, fmt.Errorf("save repaired final chapter: %w", err)
		}
		saved, readErr := book.LoadFinalChapterContent(chapter)
		if readErr != nil {
			return "", nil, fmt.Errorf("read repaired final chapter: %w", readErr)
		}
		if strings.TrimSpace(saved) != "" {
			repaired = saved
		}
	}
	recheck, err := writepipe.RunAgentSDKChapterPostSaveCheck(ctx, book, chapter, targetWords)
	if err != nil {
		return repaired, nil, err
	}
	return repaired, recheck, nil
}

func AppendAgentSDKPostSaveReviewSuggestions(review *agents.DraftReview, result *toolplane.ToolCheckResult) {
	if review == nil || result == nil {
		return
	}
	if result.Blocking {
		review.NeedsRevision = true
	}
	for _, issue := range result.Issues {
		text := formatAgentSDKPostSaveIssueSuggestion(result.Kind, result.Target, result.Scope, issue)
		if text != "" && !novelcont.ContainsStr(review.Suggestions, text) {
			review.Suggestions = append(review.Suggestions, text)
		}
	}
}

func formatAgentSDKPostSaveIssueSuggestion(kind, target, scope string, issue models.ReviewSuggestion) string {
	parts := []string{"Agent SDK post-save check still reports"}
	if issue.Priority != "" {
		parts = append(parts, "priority="+string(issue.Priority))
	}
	if strings.TrimSpace(issue.Category) != "" {
		parts = append(parts, "category="+strings.TrimSpace(issue.Category))
	}
	if strings.TrimSpace(issue.TargetID) != "" {
		parts = append(parts, "target="+strings.TrimSpace(issue.TargetID))
	}
	if strings.TrimSpace(issue.Issue) != "" {
		parts = append(parts, "issue="+strings.TrimSpace(issue.Issue))
	}
	if strings.TrimSpace(issue.Suggestion) != "" {
		parts = append(parts, "suggestion="+strings.TrimSpace(issue.Suggestion))
	}
	nav := toolplane.ToolIssueNavigation(kind, target, scope, issue.TargetID, issue, 0)
	if route, ok := nav["repair_route_query"].(string); ok && strings.TrimSpace(route) != "" {
		parts = append(parts, "repair_route_query="+route)
	}
	if repair, ok := nav["repair_context_query"].(string); ok && strings.TrimSpace(repair) != "" {
		parts = append(parts, "repair_context_query="+repair)
	}
	if check, ok := nav["focused_check_query"].(string); ok && strings.TrimSpace(check) != "" {
		parts = append(parts, "focused_check_query="+check)
	}
	return strings.Join(parts, "; ")
}

func GetWriteChaptersNeedingImprovement(review *agents.VolumeReview, outline *models.Outline, minScorePercent int) []*models.Chapter {
	if review == nil || outline == nil {
		return nil
	}
	chapters := make([]*models.Chapter, 0)
	for _, r := range review.Reviews {
		if DraftReviewNeedsImprovement(r, minScorePercent) {
			if chapter := outline.GetChapterByID(r.ChapterID); chapter != nil {
				chapters = append(chapters, chapter)
			}
		}
	}
	return chapters
}

// ReviewResultRequiresRevision converts review output into a repair decision.
// Numeric scores are intentionally not sufficient on their own: model scores
// drift between otherwise equivalent review runs. Concrete medium-or-higher
// findings are the stable signal that an improvement pass has work to do.
func ReviewResultRequiresRevision(review models.ReviewResult) bool {
	for _, suggestion := range review.Suggestions {
		if models.ReviewSuggestionPriorityRank(suggestion.Priority) <= 2 {
			return true
		}
	}
	for _, issue := range review.ContinuityIssues {
		if models.ReviewSuggestionPriorityRank(issue.Severity) <= 2 {
			return true
		}
	}
	return false
}

// DraftReviewNeedsImprovement keeps the legacy --min-score threshold as a
// secondary signal, but only when the review also contains concrete feedback.
// A score-only fluctuation must not rewrite a chapter.
func DraftReviewNeedsImprovement(review agents.DraftReview, minScorePercent int) bool {
	if !DraftReviewHasFindings(review) {
		return false
	}
	return review.NeedsRevision || DraftReviewScorePercent(review) < minScorePercent
}

func DraftReviewHasFindings(review agents.DraftReview) bool {
	return len(review.Suggestions) > 0 ||
		len(review.PlotCoherence.Issues) > 0 ||
		len(review.PlotCoherence.Suggestions) > 0 ||
		len(review.PlotRationality.LogicFlaws) > 0 ||
		len(review.PlotRationality.Suggestions) > 0 ||
		len(review.CharacterConsistency.Inconsistencies) > 0 ||
		len(review.CharacterConsistency.Suggestions) > 0 ||
		len(review.PacingReview.Issues) > 0 ||
		len(review.PacingReview.Suggestions) > 0 ||
		len(review.SceneContinuity.Issues) > 0 ||
		len(review.SceneContinuity.Suggestions) > 0 ||
		len(review.CharacterPresence.Issues) > 0 ||
		len(review.CharacterPresence.Suggestions) > 0 ||
		len(review.RecapQuality.Issues) > 0 ||
		len(review.RecapQuality.Suggestions) > 0
}

func DraftReviewScorePercent(review agents.DraftReview) int {
	if review.OverallScore <= 10 {
		return review.OverallScore * 10
	}
	return review.OverallScore
}

func ReviewScorePercentInt(score float64) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return int(score + 0.5)
}
