package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestLoadFinalChapterContentPrefersFullIDThenNumericFallback(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "novel.json"), `{"name":"demo"}`)
	writeFile(t, filepath.Join(root, "chapters", "chapter-P1-V1-C1.md"), "# full id")
	book := openBook(t, root)

	got := book.LoadFinalChapterContent(&models.Chapter{ID: "P1-V1-C1"})
	if got != "# full id" {
		t.Fatalf("content = %q, want full-id file", got)
	}

	// Underscore IDs reduce to their trailing segment, matching the historical
	// numeric-fallback naming (chapter-3.md).
	writeFile(t, filepath.Join(root, "chapters", "chapter-3.md"), "# numeric")
	if got := book.LoadFinalChapterContent(&models.Chapter{ID: "chap_1_2_3"}); got != "# numeric" {
		t.Fatalf("numeric fallback content = %q, want %q", got, "# numeric")
	}

	if got := book.LoadFinalChapterContent(&models.Chapter{ID: "missing"}); got != "" {
		t.Fatalf("missing chapter content = %q, want empty", got)
	}
	if got := book.LoadFinalChapterContent(nil); got != "" {
		t.Fatalf("nil chapter content = %q, want empty", got)
	}
}

func TestLoadDraftContentAndSaveDraft(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "novel.json"), `{"name":"demo"}`)
	writeFile(t, filepath.Join(root, "drafts", "P1-V1-C1.md"), "draft body")
	book := openBook(t, root)

	if got := book.LoadDraftContent("P1-V1-C1"); got != "draft body" {
		t.Fatalf("LoadDraftContent = %q", got)
	}
	if got := book.LoadDraftContent("nope"); got != "" {
		t.Fatalf("missing draft = %q, want empty", got)
	}

	chapter := &models.Chapter{ID: "P1-V1-C2", Title: "Chapter Two"}
	if err := book.SaveDraft(chapter, "new body"); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "drafts", "P1-V1-C2.md"))
	if err != nil {
		t.Fatalf("read saved draft: %v", err)
	}
	if !strings.HasPrefix(string(data), "# Chapter Two\n\nnew body") {
		t.Fatalf("saved draft = %q", string(data))
	}
}
