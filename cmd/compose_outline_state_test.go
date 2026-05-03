package cmd

import (
	"path/filepath"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

func TestCanResumeOutlineGenerationWithEmptyVolumes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outline.json")
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{
			{ID: "volume_1", Title: "Done", Chapters: []models.Chapter{{ID: "ch1", Title: "Chapter"}}},
			{ID: "volume_2", Title: "Pending", Chapters: []models.Chapter{}},
		},
	}}}
	if err := savePartialOutline(outline, path); err != nil {
		t.Fatalf("save outline: %v", err)
	}

	if !canResumeOutlineGeneration(path, models.StoryStructure{TargetParts: 1, TargetVolumes: 2, TargetChapters: 10}) {
		t.Fatalf("expected outline with empty volumes to be resumable")
	}
}

func TestOutlineWithGeneratedVolumesAndMergePreserveEmptyVolumes(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{
			{ID: "volume_1", Title: "Generated", Chapters: []models.Chapter{{ID: "ch1", Title: "Old"}}},
			{ID: "volume_2", Title: "Empty", Chapters: []models.Chapter{}},
		},
	}}}

	filtered := outlineWithGeneratedVolumes(outline)
	if got := len(filtered.Parts); got != 1 {
		t.Fatalf("filtered parts = %d, want 1", got)
	}
	if got := len(filtered.Parts[0].Volumes); got != 1 {
		t.Fatalf("filtered volumes = %d, want 1", got)
	}
	filtered.Parts[0].Volumes[0].Chapters[0].Title = "Improved"

	mergeGeneratedVolumes(outline, filtered)
	if got := outline.Parts[0].Volumes[0].Chapters[0].Title; got != "Improved" {
		t.Fatalf("merged generated volume title = %q, want Improved", got)
	}
	if got := len(outline.Parts[0].Volumes[1].Chapters); got != 0 {
		t.Fatalf("empty volume chapters = %d, want 0", got)
	}
}

func TestOutlineWithImproveVolumeSelection(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{
			{ID: "volume_1", Title: "Generated 1", Chapters: []models.Chapter{{ID: "ch1"}}},
			{ID: "volume_2", Title: "Generated 2", Chapters: []models.Chapter{{ID: "ch2"}}},
			{ID: "volume_3", Title: "Empty", Chapters: []models.Chapter{}},
		},
	}}}

	filtered, err := outlineWithImproveVolumeSelection(outline, 2, 0, 0)
	if err != nil {
		t.Fatalf("select volume: %v", err)
	}
	if got := len(filtered.Parts[0].Volumes); got != 1 {
		t.Fatalf("selected volumes = %d, want 1", got)
	}
	if got := filtered.Parts[0].Volumes[0].ID; got != "volume_2" {
		t.Fatalf("selected volume ID = %q, want volume_2", got)
	}

	if _, err := outlineWithImproveVolumeSelection(outline, 3, 0, 0); err == nil {
		t.Fatalf("expected empty selected volume to fail")
	}
}

func TestOutlineVolumePositionUsesGlobalVolumeIndex(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{
		{ID: "part_1", Volumes: []models.Volume{{ID: "v1"}, {ID: "v2"}}},
		{ID: "part_2", Volumes: []models.Volume{{ID: "v3"}, {ID: "v4"}}},
	}}

	partIdx, volIdx, err := outlineVolumePosition(outline, 3)
	if err != nil {
		t.Fatalf("locate volume: %v", err)
	}
	if partIdx != 1 || volIdx != 0 {
		t.Fatalf("volume 3 position = (%d,%d), want (1,0)", partIdx, volIdx)
	}

	if _, _, err := outlineVolumePosition(outline, 5); err == nil {
		t.Fatalf("expected out-of-range volume to fail")
	}
}

func TestMissingSetupResources(t *testing.T) {
	setup := &models.StorySetup{
		WorldResources: []models.WorldResource{{Name: "ore"}},
	}
	outline := &rpg.StoryOutline{Parts: []rpg.StoryPart{{
		Volumes: []rpg.StoryVolume{{
			Chapters: []rpg.StoryChapter{{
				ResourceLedger: []rpg.StoryResourceLedgerEntry{
					{Item: "ore"},
					{Item: "crystal"},
					{Item: "battery"},
					{Item: "crystal"},
				},
			}},
		}},
	}}}

	missing := missingSetupResources(setup, outline)
	if len(missing) != 2 || missing[0] != "battery" || missing[1] != "crystal" {
		t.Fatalf("missing resources = %#v, want [battery crystal]", missing)
	}
}
