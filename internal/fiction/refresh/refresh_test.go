package refresh

import (
	"context"
	"testing"

	"novelgen/internal/platform/store"
)

func TestRefreshChapterDSLRequiresProjectRoot(t *testing.T) {
	if _, err := RefreshChapterDSL(context.Background(), nil, "P1-V1-C1", 0); err == nil {
		t.Fatal("nil root should be rejected")
	}
}

func TestRefreshChapterDSLRejectsUnsafeChapterID(t *testing.T) {
	root := store.MemRoot()
	for _, id := range []string{"", "../escape", "P1/V1", ".."} {
		if _, err := RefreshChapterDSL(context.Background(), root, id, 0); err == nil {
			t.Fatalf("chapter id %q should be rejected before any IO", id)
		}
	}
}

func TestFirstExistingPathUsesProjectFilesystem(t *testing.T) {
	root := store.MemRoot()
	const existing = "story/chapters/P1-V1-C1.md"
	if err := root.FS().WriteFile(existing, []byte("# chapter"), store.FileMode); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got := FirstExistingPath(root.FS(), []string{"story/missing.md", existing})
	if got != existing {
		t.Fatalf("FirstExistingPath() = %q, want %q", got, existing)
	}
	if got := FirstExistingPath(root.FS(), []string{"story/missing.md"}); got != "" {
		t.Fatalf("FirstExistingPath() = %q, want empty", got)
	}
}
