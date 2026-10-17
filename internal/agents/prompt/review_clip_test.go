package prompt

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestClipReviewSuggestionTrimsAndClamps(t *testing.T) {
	long := strings.Repeat("字", MaxReviewIssueRunes+50)

	got := ClipReviewSuggestion(models.ReviewSuggestion{
		Category:   "  logic ",
		TargetID:   " P1-V1-C1 ",
		TargetName: strings.Repeat("n", MaxReviewTargetNameRunes+10),
		Issue:      long,
		Suggestion: strings.Repeat("s", MaxReviewSuggestionRunes+10),
		Priority:   " high ",
	})

	if got.Category != "logic" || got.TargetID != "P1-V1-C1" || got.Priority != "high" {
		t.Fatalf("identifying fields should be trimmed: %#v", got)
	}
	// ClipForPrompt keeps maxRunes runes and marks the truncation with "...".
	for name, value := range map[string]string{
		"target_name": got.TargetName,
		"issue":       got.Issue,
		"suggestion":  got.Suggestion,
	} {
		if !strings.HasSuffix(value, "...") {
			t.Fatalf("%s should be marked as truncated: %q", name, value)
		}
	}
	if len([]rune(got.TargetName)) != MaxReviewTargetNameRunes+3 {
		t.Fatalf("target name runes = %d", len([]rune(got.TargetName)))
	}
	if len([]rune(got.Issue)) != MaxReviewIssueRunes+3 {
		t.Fatalf("issue runes = %d", len([]rune(got.Issue)))
	}
	if len([]rune(got.Suggestion)) != MaxReviewSuggestionRunes+3 {
		t.Fatalf("suggestion runes = %d", len([]rune(got.Suggestion)))
	}
}

func TestClipReviewStringListDropsBlanksAndCaps(t *testing.T) {
	got := ClipReviewStringList([]string{"a", "   ", "b", "c", "d", "e"}, 3, 10)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("ClipReviewStringList = %#v", got)
	}
	if got := ClipReviewStringList(nil, 4, 10); len(got) != 0 {
		t.Fatalf("nil input should stay empty: %#v", got)
	}
}
