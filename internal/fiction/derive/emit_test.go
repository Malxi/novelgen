package derive_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/derive"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/store"
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

	chapters := make([]derive.WriteRPGDSLChapterInput, 0, 2)
	for _, id := range []string{"P1-V1-C1", "P1-V1-C2"} {
		path := filepath.Join(chapterDir, "chapter-"+id+".md")
		if err := os.WriteFile(path, []byte("# "+id+"\n\nprose\n"), 0644); err != nil {
			t.Fatal(err)
		}
		chapters = append(chapters, derive.WriteRPGDSLChapterInput{ChapterID: id, ChapterPath: path})
	}

	cacheDir := filepath.Join(root, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest, err := derive.BuildWriteRPGDSLBatchManifest(store.OSFS{}, "book", 0, 10, chapters, cacheDir)
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
	if err := derive.SaveWriteRPGDSLBatchToCache(store.OSFS{}, manifest, cached); err != nil {
		t.Fatal(err)
	}

	result, combined, err := derive.EmitWriteRPGDSL(context.Background(), derive.WriteRPGDSLEmitInput{
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
	if _, _, err := derive.EmitWriteRPGDSL(context.Background(), derive.WriteRPGDSLEmitInput{}); err == nil {
		t.Fatal("expected an error for an empty chapter list")
	}
}

// TestEmitWriteRPGDSLRunsOnAnInMemoryProject is the writersdl half of the storage
// seam test: chapter inputs, the per-batch cache and the merged output all live
// in a store.MemFS, which only works because every read and write in this
// package goes through the store's FS.
func TestEmitWriteRPGDSLRunsOnAnInMemoryProject(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	chapters := make([]derive.WriteRPGDSLChapterInput, 0, 2)
	for _, id := range []string{"P1-V1-C1", "P1-V1-C2"} {
		path := root.ChapterFile(id)
		if err := root.WriteText(path, "# "+id+"\n\nprose\n"); err != nil {
			t.Fatalf("seed chapter %s: %v", id, err)
		}
		chapters = append(chapters, derive.WriteRPGDSLChapterInput{ChapterID: id, ChapterPath: path})
	}

	cacheDir := root.RPGChapterBatchCacheDir()
	manifest, err := derive.BuildWriteRPGDSLBatchManifest(root.FS(), "book", 0, 10, chapters, cacheDir)
	if err != nil {
		t.Fatalf("BuildWriteRPGDSLBatchManifest: %v", err)
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
	if err := derive.SaveWriteRPGDSLBatchToCache(root.FS(), manifest, cached); err != nil {
		t.Fatalf("SaveWriteRPGDSLBatchToCache: %v", err)
	}

	result, combined, err := derive.EmitWriteRPGDSL(context.Background(), derive.WriteRPGDSLEmitInput{
		BookName:  "book",
		FS:        root.FS(),
		CacheDir:  cacheDir,
		Chapters:  chapters,
		BatchSize: 10,
	})
	if err != nil {
		t.Fatalf("EmitWriteRPGDSL: %v", err)
	}
	if result.ReusedBatches != 1 || result.GeneratedBatches != 0 {
		t.Fatalf("result = %#v, want the in-memory batch cache to be reused", result)
	}
	if combined == nil || combined.Storyline == nil || len(combined.Storyline.Chapters) != 2 {
		t.Fatalf("merged storyline chapters = %#v, want 2", combined)
	}

	memFS := root.FS().(*store.MemFS)
	cachedFiles := 0
	for _, name := range memFS.Names() {
		if !strings.HasPrefix(name, "/book/") {
			t.Fatalf("unexpected path outside the in-memory project: %s", name)
		}
		if strings.Contains(name, "/rpg/cache/") {
			cachedFiles++
		}
	}
	// EmitWriteRPGDSL deliberately does not write the merged DSL (the caller owns
	// that), but it must have stored the batch cache through the seam.
	if cachedFiles == 0 {
		t.Fatalf("no batch cache files in MemFS: %v", memFS.Names())
	}
}

// TestEmitChapterRPGDSLForProjectUsesInjectedRoot covers the wrapper that the
// CLI and tool refresh call. It used to load outline/craft through the ambient
// project, so a MemFS root still read host state.
func TestEmitChapterRPGDSLForProjectUsesInjectedRoot(t *testing.T) {
	root := store.MemRoot()
	config := &models.ProjectConfig{Name: "Memory Book", Language: "en"}
	setup := &models.StorySetup{ProjectName: "Memory Book"}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID:       "P1-V1",
			Chapters: []models.Chapter{{ID: "P1-V1-C1", Title: "One"}},
		}},
	}}}
	if err := root.SaveSetup(setup); err != nil {
		t.Fatalf("SaveSetup: %v", err)
	}
	if err := root.SaveOutline(outline); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	chapterPath := root.ChapterFile("P1-V1-C1")
	if err := root.WriteText(chapterPath, "# One\n\nprose\n"); err != nil {
		t.Fatalf("seed chapter: %v", err)
	}

	chapters := []derive.WriteRPGDSLChapterInput{{ChapterID: "P1-V1-C1", ChapterPath: chapterPath}}
	manifest, err := derive.BuildWriteRPGDSLBatchManifest(root.FS(), "Memory Book", 0, 10, chapters, root.RPGChapterBatchCacheDir())
	if err != nil {
		t.Fatalf("BuildWriteRPGDSLBatchManifest: %v", err)
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
}`
	if err := derive.SaveWriteRPGDSLBatchToCache(root.FS(), manifest, cached); err != nil {
		t.Fatalf("SaveWriteRPGDSLBatchToCache: %v", err)
	}

	ambient := store.MemRoot()
	store.SetDefault(ambient)
	defer store.ResetDefault()

	result, err := derive.EmitChapterRPGDSLForProject(context.Background(), root, "Memory Book", config, &llm.Config{}, setup, nil, 10)
	if err != nil {
		t.Fatalf("EmitChapterRPGDSLForProject: %v", err)
	}
	if result.DSLPath != root.RPGChaptersDSL() {
		t.Fatalf("DSLPath = %q, want %q", result.DSLPath, root.RPGChaptersDSL())
	}
	if _, ok := root.FS().(*store.MemFS).File(result.DSLPath); !ok {
		t.Fatalf("merged DSL was not written to MemFS: %v", root.FS().(*store.MemFS).Names())
	}
	if names := ambient.FS().(*store.MemFS).Names(); len(names) != 0 {
		t.Fatalf("ambient project was touched: %v", names)
	}
}
