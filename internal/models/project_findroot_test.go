package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRootWalksUpToNovelJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"name":"x"}`), 0644); err != nil {
		t.Fatalf("write novel.json: %v", err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}

	got, err := FindProjectRoot(deep)
	if err != nil {
		t.Fatalf("FindProjectRoot(%q) error: %v", deep, err)
	}
	if filepath.Clean(got) != filepath.Clean(root) {
		t.Fatalf("FindProjectRoot = %q, want %q", got, root)
	}
}
