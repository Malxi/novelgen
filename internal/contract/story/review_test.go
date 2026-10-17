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
			name: "chinese high priority is blocking",
			review: &ReviewResult{Suggestions: []ReviewSuggestion{
				{Priority: "高"},
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

func TestReviewResultNormalizeScoreScale(t *testing.T) {
	t.Run("uses dimension ratio for ten point scores", func(t *testing.T) {
		review := &ReviewResult{
			OverallScore: 9.5,
			Dimensions: []DimensionScore{
				{Name: "structure", Score: 9.5, Max: 10},
				{Name: "payoff", Score: 8.5, Max: 10},
			},
		}

		review.NormalizeScoreScale()

		if review.OverallScore != 90 {
			t.Fatalf("OverallScore = %v, want 90", review.OverallScore)
		}
	})

	t.Run("keeps percentage scores", func(t *testing.T) {
		review := &ReviewResult{
			OverallScore: 86,
			Dimensions: []DimensionScore{
				{Name: "structure", Score: 86, Max: 100},
			},
		}

		review.NormalizeScoreScale()

		if review.OverallScore != 86 {
			t.Fatalf("OverallScore = %v, want 86", review.OverallScore)
		}
	})
}
