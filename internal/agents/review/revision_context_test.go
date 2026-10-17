package review

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestRevisionSessionPromptIncludesHistory(t *testing.T) {
	session := NewRevisionSession("setup", "Make the setup sharper.")
	session.AddUserGuidance(0, "Add more爽点.")
	session.AddReview(1, models.ReviewResult{
		Summary: "The setup is close but the appeal is thin.",
		Suggestions: []models.ReviewSuggestion{
			{Priority: models.PriorityHigh, Category: models.CategoryAppeal, TargetName: "premise", Issue: "No obvious win pattern", Suggestion: "Add a concrete exploit path."},
		},
		Strengths: []string{"core premise is clear"},
	})
	session.AddImprove(1, "Expanded the appeal engine and preserved the premise.")

	prompt := session.Prompt()
	for _, want := range []string{"Revision Session", "setup", "Make the setup sharper.", "No obvious win pattern", "Expanded the appeal engine"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}
