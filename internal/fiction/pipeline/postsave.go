package writepipe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/check"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/refresh"
	"novelgen/internal/platform/logger"
)

func BuildAgentSDKChapterCheckSuggestions(book *project.Book, chapter *models.Chapter, targetWords int) string {
	if book == nil || chapter == nil || strings.TrimSpace(chapter.ID) == "" {
		return ""
	}
	result, err := check.RunToolChapterCheckWithTargetWordsAt(book.Root, "all", "chapter", chapter.ID, targetWords)
	if err != nil {
		return fmt.Sprintf("## Agent SDK Chapter Check\n\nCould not run focused chapter check before improvement: %v", err)
	}
	if err := check.ApplyToolCheckIssueFilters(result, "low", "", 8); err != nil {
		return fmt.Sprintf("## Agent SDK Chapter Check\n\nCould not filter focused chapter check before improvement: %v", err)
	}
	return FormatAgentSDKChapterCheckSuggestions(result)
}

func FormatAgentSDKChapterCheckSuggestions(result *check.ToolCheckResult) string {
	if result == nil {
		return ""
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("## Agent SDK Chapter Check\n\nCould not encode focused chapter check before improvement: %v", err)
	}
	return "## Agent SDK Chapter Check\n\n" +
		"Treat this deterministic `tool check --target chapter` result as the primary repair task list. " +
		"If an issue contains `navigation.refresh_query`, run it first, then run `navigation.post_refresh_check_query` before deciding whether prose needs repair. " +
		"Before rewriting, execute each remaining issue's `navigation.repair_route_query` when present; it returns an index-sized route and next_actions. " +
		"Only execute `navigation.repair_context_query` when the route says detailed facts or excerpts are needed. " +
		"Do not query full setup, full outline, or full chapter files unless the focused repair context is insufficient.\n\n" +
		"```json\n" + string(data) + "\n```"
}

// RunAgentSDKChapterPostSaveCheck refreshes the chapter RPG DSL and runs the
// deterministic post-save checks that gate further repair.
func RunAgentSDKChapterPostSaveCheck(ctx context.Context, book *project.Book, chapter *models.Chapter, targetWords int) (*check.ToolCheckResult, error) {
	if chapter == nil || strings.TrimSpace(chapter.ID) == "" {
		return nil, fmt.Errorf("chapter is nil or has no ID")
	}
	if book == nil {
		return nil, fmt.Errorf("project is required")
	}
	restoreLogger := refresh.SuppressLogs()
	_, refreshErr := refresh.RefreshChapterDSL(ctx, book.Root, chapter.ID, 0)
	restoreLogger()
	if refreshErr != nil {
		return nil, fmt.Errorf("refresh chapter RPG DSL before post-save check: %w", refreshErr)
	}
	result, err := check.RunToolChapterCheckWithTargetWordsAt(book.Root, "all", "chapter", chapter.ID, targetWords)
	if err != nil {
		return nil, err
	}
	if err := check.ApplyToolCheckIssueFilters(result, "low", "", 12); err != nil {
		return nil, err
	}
	return result, nil
}

func LogAgentSDKChapterPostSaveCheck(log logger.LoggerInterface, workerID int, chapterID string, result *check.ToolCheckResult) {
	if result == nil {
		return
	}
	log.Info("[Worker %d] Agent SDK post-save check for %s: ok=%t blocking=%t score=%.0f issues=%d (critical=%d high=%d medium=%d low=%d)",
		workerID,
		chapterID,
		result.OK,
		result.Blocking,
		result.Score,
		result.Summary.Total,
		result.Summary.Critical,
		result.Summary.High,
		result.Summary.Medium,
		result.Summary.Low,
	)
	if result.Blocking {
		for _, issue := range firstNReviewSuggestions(result.Issues, 3) {
			log.Warn("[Worker %d] Remaining chapter issue for %s: [%s/%s] %s", workerID, chapterID, issue.Priority, issue.Category, issue.Issue)
		}
	}
}

func firstNReviewSuggestions(items []models.ReviewSuggestion, n int) []models.ReviewSuggestion {
	if n < 0 {
		n = 0
	}
	if len(items) <= n {
		return items
	}
	return items[:n]
}
