package models

import "testing"

func TestReviewResultHasBlockingSuggestions(t *testing.T) {
	tests := []struct {
		name   string
		review *ReviewResult
		want   bool
	}{
		{
			name:   "nil review",
			review: nil,
			want:   false,
		},
		{
			name: "critical priority",
			review: &ReviewResult{Suggestions: []ReviewSuggestion{
				{Priority: "critical"},
			}},
			want: true,
		},
		{
			name: "high priority is blocking",
			review: &ReviewResult{Suggestions: []ReviewSuggestion{
				{Priority: " high "},
			}},
			want: true,
		},
		{
			name: "medium priority is not blocking",
			review: &ReviewResult{Suggestions: []ReviewSuggestion{
				{Priority: "medium"},
			}},
			want: false,
		},
		{
			name: "fatal continuity issue",
			review: &ReviewResult{ContinuityIssues: []ContinuityIssue{
				{Severity: "fatal"},
			}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.review.HasBlockingSuggestions(); got != tt.want {
				t.Fatalf("HasBlockingSuggestions() = %v, want %v", got, tt.want)
			}
		})
	}
}
