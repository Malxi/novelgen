package cmd

import (
	"testing"

	"novelgen/internal/agents"
	"novelgen/internal/models"
)

func TestDraftReviewScorePercentSupportsLegacyAndPercentScores(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "legacy ten point", in: 7, want: 70},
		{name: "percent", in: 76, want: 76},
		{name: "zero", in: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := draftReviewScorePercent(agents.DraftReview{OverallScore: tt.in})
			if got != tt.want {
				t.Fatalf("draftReviewScorePercent(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestReviewScorePercentIntClampsAndRounds(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want int
	}{
		{name: "rounds", in: 74.6, want: 75},
		{name: "clamps low", in: -1, want: 0},
		{name: "clamps high", in: 101, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reviewScorePercentInt(tt.in)
			if got != tt.want {
				t.Fatalf("reviewScorePercentInt(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestGetWriteChaptersNeedingImprovementUsesPercentThreshold(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{
					{ID: "P1-V1-C1", Title: "Legacy 7"},
					{ID: "P1-V1-C2", Title: "Percent 74"},
					{ID: "P1-V1-C3", Title: "Percent 85"},
				},
			}},
		}},
	}
	review := &agents.VolumeReview{
		Reviews: []agents.DraftReview{
			{ChapterID: "P1-V1-C1", OverallScore: 7},
			{ChapterID: "P1-V1-C2", OverallScore: 74},
			{ChapterID: "P1-V1-C3", OverallScore: 85},
		},
	}

	chapters := getWriteChaptersNeedingImprovement(review, outline, 75)
	if len(chapters) != 2 {
		t.Fatalf("chapters needing improvement = %d, want 2", len(chapters))
	}
	if chapters[0].ID != "P1-V1-C1" || chapters[1].ID != "P1-V1-C2" {
		t.Fatalf("unexpected chapters needing improvement: %#v", chapters)
	}
}
