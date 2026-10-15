package quality

import (
	"fmt"
	"sort"
	"strings"

	"novelgen/internal/models"
)

// LimitReviewSuggestionsForAgentSDKRepair caps how many review suggestions one
// Agent SDK repair pass receives, keeping the highest-priority issues first.
// The caller's review value is never mutated.
func LimitReviewSuggestionsForAgentSDKRepair(review *models.ReviewResult, limit int) *models.ReviewResult {
	if review == nil {
		return nil
	}
	if limit <= 0 || len(review.Suggestions) <= limit {
		return review
	}
	limited := *review
	limited.Suggestions = append([]models.ReviewSuggestion(nil), review.Suggestions...)
	sort.SliceStable(limited.Suggestions, func(i, j int) bool {
		left := models.ReviewSuggestionPriorityRank(limited.Suggestions[i].Priority)
		right := models.ReviewSuggestionPriorityRank(limited.Suggestions[j].Priority)
		if left != right {
			return left < right
		}
		leftTarget := strings.TrimSpace(limited.Suggestions[i].TargetID)
		rightTarget := strings.TrimSpace(limited.Suggestions[j].TargetID)
		if leftTarget != rightTarget {
			return leftTarget < rightTarget
		}
		return strings.TrimSpace(limited.Suggestions[i].Category) < strings.TrimSpace(limited.Suggestions[j].Category)
	})
	limited.Suggestions = limited.Suggestions[:limit]
	limited.Summary = fmt.Sprintf("%s Repairing the first %d of %d targetable issues in this pass.", strings.TrimSpace(review.Summary), limit, len(review.Suggestions))
	return &limited
}
