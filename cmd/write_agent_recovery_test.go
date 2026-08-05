package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestRecoverAgentAppliedChapterContentAfterErrorUsesSavedPatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Recover Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	previous := "# Opening\n\nold draft"
	if err := os.MkdirAll(filepath.Join(root, "chapters"), 0755); err != nil {
		t.Fatal(err)
	}
	saved := "# Opening\n\n" + strings.Repeat("Lin repairs the scene. ", 80)
	if err := os.WriteFile(filepath.Join(root, "chapters", "chapter-P1-V1-C1.md"), []byte(saved), 0644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatal(err)
		}
	}()

	got, ok := recoverAgentAppliedChapterContentAfterError(chapter, previous, 200, true)
	if !ok || got != saved {
		t.Fatalf("recovered = %t content len=%d, want saved len=%d", ok, len(got), len(saved))
	}
}

func TestRecoverAgentAppliedChapterContentAfterErrorRejectsOvershoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Recover Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	if err := os.MkdirAll(filepath.Join(root, "chapters"), 0755); err != nil {
		t.Fatal(err)
	}
	saved := "# Opening\n\n" + strings.Repeat("word ", 500)
	if err := os.WriteFile(filepath.Join(root, "chapters", "chapter-P1-V1-C1.md"), []byte(saved), 0644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatal(err)
		}
	}()

	if got, ok := recoverAgentAppliedChapterContentAfterError(chapter, "# Opening\n\nold draft", 100, true); ok {
		t.Fatalf("overshoot should not recover, got len=%d", len(got))
	}
}

func TestSaveFinalChapterStripsRepeatedTitleAfterHeading(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Save Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatal(err)
		}
	}()

	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "第1章：倒计时"}
	content := "# 第1章：倒计时\n\n第1章：倒计时\n\n李侑醒来时，先闻到潮湿木板的霉味。"
	if err := saveFinalChapter(chapter, content); err != nil {
		t.Fatalf("saveFinalChapter() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "chapters", "chapter-P1-V1-C1.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "\n\n第1章：倒计时\n\n") {
		t.Fatalf("saved chapter still repeats title: %q", got)
	}
	if !strings.HasPrefix(got, "# 第1章：倒计时\n\n李侑醒来") {
		t.Fatalf("saved chapter prefix = %q", got)
	}
}

func TestEffectiveAgentSDKImproveTargetWordsPreservesExistingLengthForFocusedRepair(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	content := "# Opening\n\n" + strings.Repeat("word ", 1800)

	got := effectiveAgentSDKImproveTargetWords(chapter, 3000, content, "repair one low-priority structure issue", true)
	if got != 1800 {
		t.Fatalf("effective target = %d, want current length 1800", got)
	}

	shortCurrent := "# Opening\n\n" + strings.Repeat("word ", 1100)
	got = effectiveAgentSDKImproveTargetWords(chapter, 3000, shortCurrent, "minimum revision: tighten repeated explanation", true)
	if got != 1100 {
		t.Fatalf("short effective target = %d, want current length 1100", got)
	}

	longCurrent := "# Opening\n\n" + strings.Repeat("word ", 1800)
	got = effectiveAgentSDKImproveTargetWords(chapter, 1200, longCurrent, "remove formatting artifacts", true)
	if got != 1200 {
		t.Fatalf("long effective target = %d, want requested target 1200", got)
	}

	got = effectiveAgentSDKImproveTargetWords(chapter, 3000, content, "chapter is too short; expand the scene", true)
	if got != 3000 {
		t.Fatalf("length expansion target = %d, want original target 3000", got)
	}
}
