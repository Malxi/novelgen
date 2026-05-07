package logic

import (
	"testing"

	"novelgen/internal/models"
)

func TestCalculateStateMatrixBeforeExcludesTargetChapterEvents(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{
					{
						ID: "P1-V1-C1",
						Events: []models.Event{{
							Type:       models.EventTypePremise,
							Characters: []string{"Lin"},
							Subject:    "power",
							Change:     "seeded",
						}},
					},
					{
						ID: "P1-V1-C2",
						Events: []models.Event{{
							Type:       models.EventTypePremise,
							Characters: []string{"Lin"},
							Subject:    "rank",
							Change:     "advanced",
						}},
					},
				},
			}},
		}},
	}

	manager := NewStateMatrixManager("")
	beforeFirst := manager.CalculateStateMatrixBefore(outline, &outline.Parts[0].Volumes[0].Chapters[0])
	if got := beforeFirst.Premises["Lin_power"]; got != "" {
		t.Fatalf("before first chapter included target event premise = %q", got)
	}

	beforeSecond := manager.CalculateStateMatrixBefore(outline, &outline.Parts[0].Volumes[0].Chapters[1])
	if got := beforeSecond.Premises["Lin_power"]; got != "seeded" {
		t.Fatalf("before second chapter previous premise = %q, want seeded", got)
	}
	if got := beforeSecond.Premises["Lin_rank"]; got != "" {
		t.Fatalf("before second chapter included target event premise = %q", got)
	}

	afterSecond := manager.CalculateStateMatrix(outline, &outline.Parts[0].Volumes[0].Chapters[1])
	if got := afterSecond.Premises["Lin_rank"]; got != "advanced" {
		t.Fatalf("after second chapter target premise = %q, want advanced", got)
	}
}
