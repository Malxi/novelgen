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

	all, err := VolumesForSelection(outline, "", "")
	if err != nil {
		t.Fatalf("all volumes: %v", err)
	}
	if got := volumeIDs(all); got != "P1-V1,P1-V2" {
		t.Fatalf("all volumes = %q", got)
	}
	selectedVolume, err := VolumesForSelection(outline, "2", "")
	if err != nil {
		t.Fatalf("volume flag: %v", err)
	}
	if got := volumeIDs(selectedVolume); got != "P1-V2" {
		t.Fatalf("volume flag = %q", got)
	}
	selectedChapter, err := VolumesForSelection(outline, "", "P1-V1-C2")
	if err != nil {
		t.Fatalf("chapter flag: %v", err)
	}
	if got := volumeIDs(selectedChapter); got != "P1-V1" {
		t.Fatalf("chapter flag = %q", got)
	}
	if _, err := VolumesForSelection(outline, "999", ""); err == nil {
		t.Fatal("missing volume should be an error, not a silent fallback to every volume")
	}
	if _, err := VolumesForSelection(outline, "", "P9-V9-C9"); err == nil {
		t.Fatal("missing chapter should be an error, not a silent fallback to every volume")
	}
}
