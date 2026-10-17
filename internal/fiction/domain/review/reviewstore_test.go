package review

import (
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

func TestBuildImprovementSuggestionsSectionOrderAndTrailingNewline(t *testing.T) {
	t.Parallel()

	review := &models.DraftReview{
		SceneContinuity:      models.SceneContinuityReview{Suggestions: []string{"a"}},
		CharacterPresence:    models.PresenceReview{Suggestions: []string{"b"}},
		PlotCoherence:        models.PlotCoherenceReview{Suggestions: []string{"c"}},
		PlotRationality:      models.RationalityReview{Suggestions: []string{"d"}},
		CharacterConsistency: models.CharacterReview{Suggestions: []string{"e"}},
		PacingReview:         models.PacingReview{Suggestions: []string{"f"}},
		Suggestions:          []string{"g"},
	}

	want := "## 改进建议\n\n" +
		"### 场景/转场连续性\n- a\n\n" +
		"### 角色出场一致性\n- b\n\n" +
		"### 剧情连贯性\n- c\n\n" +
		"### 情节合理性\n- d\n\n" +
		"### 角色一致性\n- e\n\n" +
		"### 节奏把控\n- f\n\n" +
		"### 总体建议\n- g\n"

	if got := BuildImprovementSuggestions(review); got != want {
		t.Fatalf("BuildImprovementSuggestions mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestSaveLoadVolumeReviewRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"name":"x"}`), 0644); err != nil {
		t.Fatalf("write novel.json: %v", err)
	}
	storeRoot, err := store.Open(root)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	book := project.Open(storeRoot)

	review := &models.VolumeReview{VolumeID: "P1-V1", VolumeTitle: "Vol", Reviews: []models.DraftReview{{ChapterID: "C1"}}}
	if err := SaveVolumeReviewAt(book, review); err != nil {
		t.Fatalf("SaveVolumeReview: %v", err)
	}
	got, err := LoadVolumeReviewAt(book, "P1-V1")
	if err != nil {
		t.Fatalf("LoadVolumeReview: %v", err)
	}
	if got.VolumeID != "P1-V1" || len(got.Reviews) != 1 || got.Reviews[0].ChapterID != "C1" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
