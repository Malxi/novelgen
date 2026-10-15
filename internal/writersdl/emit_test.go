package writersdl_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/writersdl"
)

// TestEmitWriteRPGDSLReusesBatchCacheWithoutAgent pins the shared entry point
// used by the standalone tools: when the per-batch cache is already populated,
// emission must reuse it and never touch the conversion agent (nil here).
func TestEmitWriteRPGDSLReusesBatchCacheWithoutAgent(t *testing.T) {
	root := t.TempDir()
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}

	chapters := make([]writersdl.WriteRPGDSLChapterInput, 0, 2)
	for _, id := range []string{"P1-V1-C1", "P1-V1-C2"} {
		path := filepath.Join(chapterDir, "chapter-"+id+".md")
		if err := os.WriteFile(path, []byte("# "+id+"\n\nprose\n"), 0644); err != nil {
			t.Fatal(err)
		}
		chapters = append(chapters, writersdl.WriteRPGDSLChapterInput{ChapterID: id, ChapterPath: path})
	}

	cacheDir := filepath.Join(root, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest, err := writersdl.BuildWriteRPGDSLBatchManifest("book", 0, 10, chapters, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	cached := `storyline {
  chapter "Chapter 1" {
    id = "P1-V1-C1"
    objective "one" {
      step 1 {
        description = "a"
      }
    }
  }
  chapter "Chapter 2" {
    id = "P1-V1-C2"
    objective "two" {
      step 1 {
        description = "b"
      }
    }
  }
}`
	if err := writersdl.SaveWriteRPGDSLBatchToCache(manifest, cached); err != nil {
		t.Fatal(err)
	}

	result, combined, err := writersdl.EmitWriteRPGDSL(context.Background(), writersdl.WriteRPGDSLEmitInput{
		BookName:  "book",
		CacheDir:  cacheDir,
		Chapters:  chapters,
		BatchSize: 10,
	})
	if err != nil {
		t.Fatalf("EmitWriteRPGDSL: %v", err)
	}
	if result.ReusedBatches != 1 || result.GeneratedBatches != 0 {
		t.Fatalf("result = %#v, want 1 reused batch and 0 generated", result)
	}
	if result.ChapterCount != 2 {
		t.Fatalf("ChapterCount = %d, want 2", result.ChapterCount)
	}
	if combined == nil || combined.Storyline == nil || len(combined.Storyline.Chapters) != 2 {
		t.Fatalf("merged storyline chapters = %#v, want 2", combined)
	}
	if got := combined.Storyline.Chapters[1].ID; got != "P1-V1-C2" {
		t.Fatalf("second chapter ID = %q, want P1-V1-C2", got)
	}
}

func TestEmitWriteRPGDSLRejectsEmptyChapters(t *testing.T) {
	if _, _, err := writersdl.EmitWriteRPGDSL(context.Background(), writersdl.WriteRPGDSLEmitInput{}); err == nil {
		t.Fatal("expected an error for an empty chapter list")
	}
}
