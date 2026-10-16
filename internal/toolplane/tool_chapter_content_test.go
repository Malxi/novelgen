package toolplane

import (
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/store"
)

// chapterContentContext builds the context a chapter query reads through, with
// one saved chapter file.
func chapterContentContext(t *testing.T, chapterID, fileName, content string) ToolProjectContext {
	t.Helper()
	dir := t.TempDir()
	book := store.At(dir)
	if err := book.WriteText(book.ChapterFile(fileName), content); err != nil {
		t.Fatalf("write chapter: %v", err)
	}
	if _, err := os.Stat(book.ChapterFile(fileName)); err != nil {
		t.Fatalf("chapter file missing: %v", err)
	}
	return ToolProjectContext{
		Root: book,
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID:       "P1-V1",
				Chapters: []models.Chapter{{ID: chapterID, Title: "Chapter"}},
			}},
		}}},
	}
}

// TestChapterQuerySeparatesContentPathFromContent guards the response shape the
// agent tools document: content_path is the file the manuscript was read from and
// content is the manuscript. loadChapterContent returns (content, path) while the
// chapter hit fields are (ContentPath, Content), so a swapped assignment would
// silently ship the chapter text as the path.
func TestChapterQuerySeparatesContentPathFromContent(t *testing.T) {
	const text = "# Chapter\n\nSaved prose.\n"
	for _, tc := range []struct {
		name       string
		chapterID  string
		fileName   string
		wantSuffix string
	}{
		{name: "canonical name", chapterID: "P1-V1-C1", fileName: "P1-V1-C1", wantSuffix: "chapter-P1-V1-C1.md"},
		{name: "legacy numeric name", chapterID: "chap_1_2_3", fileName: "3", wantSuffix: "chapter-3.md"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := chapterContentContext(t, tc.chapterID, tc.fileName, text)

			resp := queryChapter(ctx, tc.chapterID, true)
			if !resp.OK || resp.Count != 1 {
				t.Fatalf("chapter query = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
			}
			hits, ok := resp.Results.([]chapterHit)
			if !ok || len(hits) != 1 {
				t.Fatalf("results type = %T", resp.Results)
			}
			assertChapterHitContent(t, hits[0].ContentPath, hits[0].Content, tc.wantSuffix, text)
		})
	}
}

// TestChapterQueryWithoutContentOmitsManuscriptText pins that the manuscript is
// only read when the caller asks for it.
func TestChapterQueryWithoutContentOmitsManuscriptText(t *testing.T) {
	ctx := chapterContentContext(t, "P1-V1-C1", "P1-V1-C1", "Saved prose.\n")

	resp := queryChapter(ctx, "P1-V1-C1", false)
	hits, ok := resp.Results.([]chapterHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("results type = %T", resp.Results)
	}
	if hits[0].Content != "" || hits[0].ContentPath != "" {
		t.Fatalf("content = %q path = %q, want both empty without --content", hits[0].Content, hits[0].ContentPath)
	}
}

func assertChapterHitContent(t *testing.T, contentPath, content, wantSuffix, wantText string) {
	t.Helper()
	if filepath.Base(contentPath) != wantSuffix {
		t.Fatalf("content_path = %q, want a path ending in %q", contentPath, wantSuffix)
	}
	if content != wantText {
		t.Fatalf("content = %q, want %q", content, wantText)
	}
}
