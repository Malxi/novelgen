package writecmd_test

import (
	"os"
	"testing"
	"time"

	"novelgen/cmd/writecmd"
	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/project"
	reviewstore "novelgen/internal/fiction/domain/review"
	"novelgen/internal/platform/store"
)

func TestLoadReusableWriteReviewRequiresReviewAfterChapter(t *testing.T) {
	root, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	book := project.Open(root)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	if err := root.SaveChapterContent(chapter.ID, "# Opening\n\nBody.\n"); err != nil {
		t.Fatal(err)
	}
	if err := reviewstore.SaveVolumeReviewAt(book, &agents.VolumeReview{
		VolumeID: "P1-V1",
		Reviews:  []agents.DraftReview{{ChapterID: chapter.ID, OverallScore: 88}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteJSON(root.ReviewFile(chapter.ID+"_write_review.json"), models.ReviewResult{OverallScore: 88}); err != nil {
		t.Fatal(err)
	}
	_, chapterPath, err := root.LoadChapterContent(chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := writecmd.LoadReusableWriteReview(book, chapter, chapterPath); !ok {
		t.Fatal("review written after the chapter should be reusable")
	}

	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(chapterPath, future, future); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := writecmd.LoadReusableWriteReview(book, chapter, chapterPath); ok {
		t.Fatal("a chapter newer than its review must be reviewed again")
	}
}

func TestPipelineChapterCompleteRequiresCleanReviewRecapAndDSL(t *testing.T) {
	root, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	book := project.Open(root)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	if err := root.SaveChapterContent(chapter.ID, "# Opening\n\n林野推开门。\n"); err != nil {
		t.Fatal(err)
	}
	_, chapterPath, err := root.LoadChapterContent(chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	clean := &agents.DraftReview{ChapterID: chapter.ID, OverallScore: 91}
	if writecmd.PipelineChapterComplete(book, chapter, chapterPath, clean, 70, true) {
		t.Fatal("missing recap and DSL must keep the chapter incomplete")
	}
	if err := root.SaveRecap(&models.ChapterRecap{
		ChapterID:       chapter.ID,
		Title:           chapter.Title,
		Location:        "门厅",
		Present:         []string{"林野"},
		LastLine:        "林野推开门。",
		NextOpeningHint: "林野推开门，看见走廊尽头的灯。",
	}); err != nil {
		t.Fatal(err)
	}
	if err := root.Write(root.RPGChaptersDSL(), []byte("chapters {}\n"), store.FileMode); err != nil {
		t.Fatal(err)
	}
	if !writecmd.PipelineChapterComplete(book, chapter, chapterPath, clean, 70, true) {
		t.Fatal("clean review with recap and DSL should be complete")
	}
	dirty := &agents.DraftReview{
		ChapterID:     chapter.ID,
		OverallScore:  96,
		NeedsRevision: true,
		Suggestions:   []string{"[high] continuity: repair the contradiction"},
	}
	if writecmd.PipelineChapterComplete(book, chapter, chapterPath, dirty, 70, true) {
		t.Fatal("actionable review findings must keep the chapter incomplete")
	}
}
