package review

import (
	"novelgen/internal/agents/prompt"
	"novelgen/internal/models"
)

// SDKReviewResult is the compact review shape returned by SDK workflow agents.
// Compose and setup both consume it, so it lives beside the shared review
// context rather than in either stage.
type SDKReviewResult struct {
	OverallScore float64               `json:"overall_score" md:"overall_score" desc:"0-100 score for the target volume after the returned patch"`
	Summary      string                `json:"summary" md:"summary" desc:"Compact Chinese summary under 500 characters"`
	Suggestions  []SDKReviewSuggestion `json:"suggestions,omitempty" md:"suggestions" desc:"At most 8 remaining issues or applied patch notes"`
}

// SDKReviewSuggestion is one compact suggestion in an SDK workflow result.
type SDKReviewSuggestion struct {
	Category   string `json:"category,omitempty" md:"category" desc:"Issue category such as logic, structure, state_anchor, resource_ledger"`
	TargetID   string `json:"target_id,omitempty" md:"target_id" desc:"Target volume or chapter ID"`
	TargetName string `json:"target_name,omitempty" md:"target_name" desc:"Short target display name"`
	Issue      string `json:"issue" md:"issue" desc:"Short issue or applied-change description"`
	Suggestion string `json:"suggestion,omitempty" md:"suggestion" desc:"Short fix, or why no patch is needed"`
	Priority   string `json:"priority" md:"priority" desc:"critical, high, medium, or low"`
}

// ToModelReviewResult converts the SDK shape into the canonical model result
// while applying the shared prompt budgets.
func (r SDKReviewResult) ToModelReviewResult() models.ReviewResult {
	result := models.ReviewResult{
		OverallScore: r.OverallScore,
		Summary:      prompt.ClipForPrompt(r.Summary, 500),
	}
	for _, suggestion := range r.Suggestions {
		result.Suggestions = append(result.Suggestions, prompt.ClipReviewSuggestion(models.ReviewSuggestion{
			Category:   suggestion.Category,
			TargetID:   suggestion.TargetID,
			TargetName: suggestion.TargetName,
			Issue:      suggestion.Issue,
			Suggestion: suggestion.Suggestion,
			Priority:   suggestion.Priority,
		}))
		if len(result.Suggestions) >= prompt.MaxReviewSuggestions {
			break
		}
	}
	return result
}
