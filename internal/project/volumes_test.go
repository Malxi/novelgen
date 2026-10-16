package project

import (
	"testing"

	"novelgen/internal/models"
)

func volumesTestOutline() *models.Outline {
	return &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{
			{ID: "P1-V1", Chapters: []models.Chapter{{ID: "P1-V1-C1"}, {ID: "P1-V1-C2"}}},
			{ID: "P1-V2", Chapters: []models.Chapter{{ID: "P1-V2-C1"}}},
		},
	}}}
}

func volumeIDs(volumes []*models.Volume) string {
	out := ""
	for i, v := range volumes {
		if i > 0 {
			out += ","
		}
		out += v.ID
	}
	return out
}

func TestVolumesForSelection(t *testing.T) {
	t.Parallel()

	outline := volumesTestOutline()

	if got := volumeIDs(VolumesForSelection(outline, "", "")); got != "P1-V1,P1-V2" {
		t.Fatalf("all volumes = %q", got)
	}
	if got := volumeIDs(VolumesForSelection(outline, "2", "")); got != "P1-V2" {
		t.Fatalf("volume flag = %q", got)
	}
	if got := volumeIDs(VolumesForSelection(outline, "", "P1-V1-C2")); got != "P1-V1" {
		t.Fatalf("chapter flag = %q", got)
	}
}
