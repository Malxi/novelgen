package cmd

import (
	"testing"

	"novelgen/internal/rpg"
)

func TestSelectOutlineVolume(t *testing.T) {
	project := &rpg.NovelgenProject{
		Outline: rpg.StoryOutline{Parts: []rpg.StoryPart{{
			ID: "part_1",
			Volumes: []rpg.StoryVolume{
				{ID: "volume_1", Title: "One", Chapters: []rpg.StoryChapter{{ID: "v1ch01"}}},
				{ID: "volume_2", Title: "Two", Chapters: []rpg.StoryChapter{{ID: "v2ch01"}}},
				{ID: "volume_3", Title: "Empty"},
			},
		}}},
	}

	if err := selectOutlineVolume(project, 2); err != nil {
		t.Fatalf("select volume: %v", err)
	}
	if got := len(project.Outline.Parts); got != 1 {
		t.Fatalf("parts = %d, want 1", got)
	}
	if got := len(project.Outline.Parts[0].Volumes); got != 1 {
		t.Fatalf("volumes = %d, want 1", got)
	}
	if got := project.Outline.Parts[0].Volumes[0].ID; got != "volume_2" {
		t.Fatalf("selected volume = %q, want volume_2", got)
	}
}

func TestSelectOutlineVolumeRejectsEmptyVolume(t *testing.T) {
	project := &rpg.NovelgenProject{
		Outline: rpg.StoryOutline{Parts: []rpg.StoryPart{{
			ID:      "part_1",
			Volumes: []rpg.StoryVolume{{ID: "volume_1"}},
		}}},
	}

	if err := selectOutlineVolume(project, 1); err == nil {
		t.Fatalf("expected empty volume selection to fail")
	}
}
