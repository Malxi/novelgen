package agents

import (
	"strings"

	"novelgen/internal/models"
)

// Prompt-length budgets for review content. Every stage that turns an Agent SDK
// review into a model result, or compacts a review back into a prompt, has to
// apply the same clamps; keeping them here means one path cannot silently leak
// unbounded text into a prompt.
const (
	maxReviewTargetNameRunes = 120
	maxReviewIssueRunes      = 360
	maxReviewSuggestionRunes = 420
	maxReviewSuggestions     = 8
	maxReviewStrengths       = 4
	maxReviewWeaknesses      = 4
	maxReviewStrengthRunes   = 180
	maxReviewWeaknessRunes   = 220
)

// clipReviewSuggestion trims the identifying fields and clamps the free-text
// fields of one review suggestion to the shared prompt budget.
func clipReviewSuggestion(suggestion models.ReviewSuggestion) models.ReviewSuggestion {
	suggestion.Category = strings.TrimSpace(suggestion.Category)
	suggestion.TargetID = strings.TrimSpace(suggestion.TargetID)
	suggestion.TargetName = clipForPrompt(suggestion.TargetName, maxReviewTargetNameRunes)
	suggestion.Issue = clipForPrompt(suggestion.Issue, maxReviewIssueRunes)
	suggestion.Suggestion = clipForPrompt(suggestion.Suggestion, maxReviewSuggestionRunes)
	suggestion.Priority = strings.TrimSpace(suggestion.Priority)
	return suggestion
}

// clipReviewStringList clamps a list of free-text review entries, dropping blank
// ones and stopping at maxItems.
func clipReviewStringList(values []string, maxItems int, maxRunes int) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = clipForPrompt(value, maxRunes)
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
