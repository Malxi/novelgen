package project_test

import (
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/project"
)

func selectionTestOutline() *models.Outline {
	return &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{
				{ID: "P1-V1-C1"},
				{ID: "P1-V1-C2"},
				{ID: "P1-V1-C3"},
			},
		}},
	}}}
}

func chapterIDs(chapters []*models.Chapter) string {
	ids := make([]string, 0, len(chapters))
	for _, chapter := range chapters {
		ids = append(ids, chapter.ID)
	}
	return strings.Join(ids, ",")
}

// TestGetChaptersToGenerate is the regression net for the chapter selection
// shared by the draft/recap/write concurrency loops. The worker refactor must
// not change which chapters are selected, their order, or the error contract.
func TestGetChaptersToGenerate(t *testing.T) {
	t.Parallel()

	outline := selectionTestOutline()

	t.Run("all returns every chapter in order", func(t *testing.T) {
		got, err := project.SelectChapters(outline, "", "", "", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ids := chapterIDs(got); ids != "P1-V1-C1,P1-V1-C2,P1-V1-C3" {
			t.Fatalf("ids = %q", ids)
		}
	})

	t.Run("numeric range is inclusive", func(t *testing.T) {
		got, err := project.SelectChapters(outline, "1-2", "", "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ids := chapterIDs(got); ids != "P1-V1-C1,P1-V1-C2" {
			t.Fatalf("ids = %q", ids)
		}
	})

	t.Run("explicit chapter id selects one chapter", func(t *testing.T) {
		got, err := project.SelectChapters(outline, "P1-V1-C2", "", "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ids := chapterIDs(got); ids != "P1-V1-C2" {
			t.Fatalf("ids = %q", ids)
		}
	})

	t.Run("missing selection is an error", func(t *testing.T) {
		if _, err := project.SelectChapters(outline, "", "", "", false); err == nil {
			t.Fatal("expected an error when neither --chapter nor --all is given")
		}
	})

	t.Run("empty outline is an error", func(t *testing.T) {
		if _, err := project.SelectChapters(&models.Outline{}, "", "", "", true); err == nil {
			t.Fatal("expected an error for an empty outline")
		}
	})

	t.Run("out of range range is an error", func(t *testing.T) {
		if _, err := project.SelectChapters(outline, "1-9", "", "", false); err == nil {
			t.Fatal("expected an out-of-range error")
		}
	})
}
