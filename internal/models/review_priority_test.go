package models

import "testing"

func TestReviewSuggestionPriorityRankOrdersSeverity(t *testing.T) {
	order := []string{PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow}
	for want, priority := range order {
		if got := ReviewSuggestionPriorityRank(priority); got != want {
			t.Fatalf("ReviewSuggestionPriorityRank(%q) = %d, want %d", priority, got, want)
		}
	}
}

func TestReviewSuggestionPriorityRankNormalizesInput(t *testing.T) {
	cases := map[string]int{
		"  HIGH ":  1,
		"Critical": 0,
		"严重":       0,
		"高":        1,
		"中优先级":     2,
		"低":        3,
		"fatal":    4, // not a known alias for this ordering
		"":         4,
	}
	for input, want := range cases {
		if got := ReviewSuggestionPriorityRank(input); got != want {
			t.Errorf("ReviewSuggestionPriorityRank(%q) = %d, want %d", input, got, want)
		}
	}
}
