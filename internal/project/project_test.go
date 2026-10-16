package project

import (
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/store"
)

// openBook returns the project view for a fixture directory. Tests hold an
// explicit root instead of switching the process working directory, which is
// what lets them run in parallel.
func openBook(t *testing.T, dir string) *Book {
	t.Helper()
	root, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	return Open(root)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func TestFindRootWalksUpFromWorkingDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "novel.json"), `{"name":"demo"}`)
	nested := filepath.Join(root, "story", "compose")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	// Locating from a nested directory is what FindRoot wraps; the walk-up
	// itself is asserted in internal/store.
	located, err := store.Locate(nested)
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	if filepath.Clean(located.Dir()) != filepath.Clean(root) {
		t.Fatalf("Locate = %q, want %q", located.Dir(), root)
	}
}

func TestLoadAllElementsNormalizesAndSkipsMissingFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "novel.json"), `{"name":"demo"}`)
	// characters.json carries a UTF-8 BOM and a null entry that must be dropped.
	writeFile(t, filepath.Join(root, "story", "craft", "characters.json"),
		"\uFEFF{\"A\":{\"name\":\"A\"},\"nil\":null}")
	writeFile(t, filepath.Join(root, "story", "craft", "organizations.json"),
		`{"Org":null}`)
	book := openBook(t, root)

	characters, locations, items, organizations, err := book.LoadElements()
	if err != nil {
		t.Fatalf("LoadAllElements error: %v", err)
	}
	if len(characters) != 1 || characters["A"] == nil {
		t.Fatalf("characters = %#v, want only A", characters)
	}
	if characters["A"].Name != "A" {
		t.Fatalf("character A name = %q, want A (normalized)", characters["A"].Name)
	}
	if len(locations) != 0 || len(items) != 0 {
		t.Fatalf("missing files should yield empty maps, got locations=%v items=%v", locations, items)
	}
	if len(organizations) != 0 {
		t.Fatalf("null organization should be dropped, got %#v", organizations)
	}
}
