package agents

import (
	"testing"

	"novelgen/internal/models"
)

func TestIdentifyVolumesToImproveResolvesActualVolumeIDs(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{
				{ID: "P1-V1"},
				{ID: "P1-V2"},
			},
		}},
	}
	review := &models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V2-C3"},
			{TargetID: "P1-V2"},
			{TargetID: "global"},
		},
	}

	got := (&ComposeAgent{}).identifyVolumesToImprove(outline, review)
	if len(got) != 1 {
		t.Fatalf("expected one volume target, got %v", got)
	}
	if got[0] != [2]int{0, 1} {
		t.Fatalf("expected P1-V2 indices [0 1], got %v", got[0])
	}
}

func TestIdentifyVolumesToImproveToleratesSinglePartMalformedTarget(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{
				{ID: "P1-V1"},
				{ID: "P1-V2"},
			},
		}},
	}
	review := &models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P2-V2-C9"},
		},
	}

	got := (&ComposeAgent{}).identifyVolumesToImprove(outline, review)
	if len(got) != 1 {
		t.Fatalf("expected one volume target, got %v", got)
	}
	if got[0] != [2]int{0, 1} {
		t.Fatalf("expected malformed P2-V2 to resolve to P1-V2 [0 1], got %v", got[0])
	}
}
