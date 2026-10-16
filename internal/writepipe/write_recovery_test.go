package writepipe_test

import (
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"
	"novelgen/internal/writepipe"
)

// memBook is an in-memory project for the write pipeline. The pipeline takes an
// explicit *project.Book now, so its tests no longer point the ambient project
// root at a temp directory — and these tests passing is evidence that the
// pipeline reads and writes only through the store.
func memBook(t *testing.T) *project.Book {
	t.Helper()
	book := project.Open(store.MemRoot())
	if err := book.SaveConfig(&models.ProjectConfig{Name: "Recovery Test"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	return book
}

func TestRecoverAgentAppliedChapterContentAfterErrorUsesSavedPatch(t *testing.T) {
	t.Parallel()

	book := memBook(t)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	previous := "# Opening\n\nold draft"
	saved := "# Opening\n\n" + strings.Repeat("Lin repairs the scene. ", 80)
	if err := book.SaveChapterContent(chapter.ID, saved); err != nil {
		t.Fatalf("SaveChapterContent: %v", err)
	}

	got, ok := writepipe.RecoverAgentAppliedChapterContentAfterError(book, chapter, previous, 200, true)
	if !ok || got != saved {
		t.Fatalf("recovered = %t content len=%d, want saved len=%d", ok, len(got), len(saved))
	}
}

func TestRecoverAgentAppliedChapterContentAfterErrorRejectsOvershoot(t *testing.T) {
	t.Parallel()

	book := memBook(t)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	saved := "# Opening\n\n" + strings.Repeat("word ", 500)
	if err := book.SaveChapterContent(chapter.ID, saved); err != nil {
		t.Fatalf("SaveChapterContent: %v", err)
	}

	if got, ok := writepipe.RecoverAgentAppliedChapterContentAfterError(book, chapter, "# Opening\n\nold draft", 100, true); ok {
		t.Fatalf("overshoot should not recover, got len=%d", len(got))
	}
}

func TestSaveFinalChapterStripsRepeatedTitleAfterHeading(t *testing.T) {
	t.Parallel()

	book := memBook(t)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "第1章：倒计时"}
	content := "# 第1章：倒计时\n\n第1章：倒计时\n\n李侑醒来时，先闻到潮湿木板的霉味。"
	if err := writepipe.SaveFinalChapter(book, chapter, content); err != nil {
		t.Fatalf("writepipe.SaveFinalChapter() error = %v", err)
	}

	got, _, err := book.LoadChapterContent(chapter.ID)
	if err != nil {
		t.Fatalf("LoadChapterContent: %v", err)
	}
	if strings.Contains(got, "\n\n第1章：倒计时\n\n") {
		t.Fatalf("saved chapter still repeats title: %q", got)
	}
	if !strings.HasPrefix(got, "# 第1章：倒计时\n\n李侑醒来") {
		t.Fatalf("saved chapter prefix = %q", got)
	}
}

func TestEffectiveAgentSDKImproveTargetWordsPreservesExistingLengthForFocusedRepair(t *testing.T) {
	t.Parallel()

	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	content := "# Opening\n\n" + strings.Repeat("word ", 1800)

	got := writepipe.EffectiveAgentSDKImproveTargetWords(chapter, 3000, content, "repair one low-priority structure issue", true)
	if got != 1800 {
		t.Fatalf("effective target = %d, want current length 1800", got)
	}

	shortCurrent := "# Opening\n\n" + strings.Repeat("word ", 1100)
	got = writepipe.EffectiveAgentSDKImproveTargetWords(chapter, 3000, shortCurrent, "minimum revision: tighten repeated explanation", true)
	if got != 1100 {
		t.Fatalf("short effective target = %d, want current length 1100", got)
	}

	longCurrent := "# Opening\n\n" + strings.Repeat("word ", 1800)
	got = writepipe.EffectiveAgentSDKImproveTargetWords(chapter, 1200, longCurrent, "remove formatting artifacts", true)
	if got != 1200 {
		t.Fatalf("long effective target = %d, want requested target 1200", got)
	}

	got = writepipe.EffectiveAgentSDKImproveTargetWords(chapter, 3000, content, "chapter is too short; expand the scene", true)
	if got != 3000 {
		t.Fatalf("length expansion target = %d, want original target 3000", got)
	}
}
