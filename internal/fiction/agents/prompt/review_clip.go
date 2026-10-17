package prompt

import (
	"strings"

	"novelgen/internal/contract/story"
)

// Prompt-length budgets for review content. Every stage that turns an Agent SDK
// review into a model result, or compacts a review back into a prompt, has to
// apply the same clamps; keeping them here means one path cannot silently leak
// unbounded text into a prompt.
const (
	MaxReviewTargetNameRunes = 120
	MaxReviewIssueRunes      = 360
	MaxReviewSuggestionRunes = 420
	MaxReviewSuggestions     = 8
	MaxReviewStrengths       = 4
	MaxReviewWeaknesses      = 4
	MaxReviewStrengthRunes   = 180
	MaxReviewWeaknessRunes   = 220
)

// ClipForPrompt trims a value and clamps it to maxRunes, appending an ellipsis
// when it had to cut. It is the single prompt-budget helper for every stage.
func ClipForPrompt(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || value == "" {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

// NonEmpty returns fallback when value is blank.
func NonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// CompactReviewForPrompt keeps the review fields that matter for the next
// review/improve round and clamps every free-text field to the shared budget.
func CompactReviewForPrompt(review models.ReviewResult) models.ReviewResult {
	compact := models.ReviewResult{
		OverallScore: review.OverallScore,
		Dimensions:   review.Dimensions,
		Summary:      ClipForPrompt(review.Summary, 700),
		Iteration:    review.Iteration,
	}

	for _, strength := range review.Strengths {
		compact.Strengths = append(compact.Strengths, ClipForPrompt(strength, 220))
	}
	for _, weakness := range review.Weaknesses {
		compact.Weaknesses = append(compact.Weaknesses, ClipForPrompt(weakness, 260))
	}
	for _, suggestion := range review.Suggestions {
		compact.Suggestions = append(compact.Suggestions, ClipReviewSuggestion(suggestion))
	}
	for _, issue := range review.ContinuityIssues {
		compact.ContinuityIssues = append(compact.ContinuityIssues, models.ContinuityIssue{
			Type:        issue.Type,
			Location:    ClipForPrompt(issue.Location, 160),
			Description: ClipForPrompt(issue.Description, 320),
			Reason:      ClipForPrompt(issue.Reason, 260),
			Suggestion:  ClipForPrompt(issue.Suggestion, 360),
			Severity:    issue.Severity,
		})
	}

	return compact
}

// ClipReviewSuggestion trims the identifying fields and clamps the free-text
// fields of one review suggestion to the shared prompt budget.
func ClipReviewSuggestion(suggestion models.ReviewSuggestion) models.ReviewSuggestion {
	suggestion.Category = strings.TrimSpace(suggestion.Category)
	suggestion.TargetID = strings.TrimSpace(suggestion.TargetID)
	suggestion.TargetName = ClipForPrompt(suggestion.TargetName, MaxReviewTargetNameRunes)
	suggestion.Issue = ClipForPrompt(suggestion.Issue, MaxReviewIssueRunes)
	suggestion.Suggestion = ClipForPrompt(suggestion.Suggestion, MaxReviewSuggestionRunes)
	suggestion.Priority = strings.TrimSpace(suggestion.Priority)
	return suggestion
}

// ClipReviewStringList clamps a list of free-text review entries, dropping blank
// ones and stopping at maxItems.
func ClipReviewStringList(values []string, maxItems int, maxRunes int) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = ClipForPrompt(value, maxRunes)
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}
