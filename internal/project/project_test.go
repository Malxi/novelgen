package project

import (
	"os"
	"path/filepath"
	"testing"
)

// chdir switches to dir for the duration of the test and always restores the
// original working directory. (t.Chdir is Go 1.24+, and this module targets
// 1.21.)
func chdir(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
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
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "novel.json"), `{"name":"demo"}`)
	nested := filepath.Join(root, "story", "compose")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	chdir(t, nested)

	got, err := FindRoot()
	if err != nil {
		t.Fatalf("FindRoot error: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(root) {
		t.Fatalf("FindRoot = %q, want %q", got, root)
	}
}

func TestLoadAllElementsNormalizesAndSkipsMissingFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "novel.json"), `{"name":"demo"}`)
	// characters.json carries a UTF-8 BOM and a null entry that must be dropped.
	writeFile(t, filepath.Join(root, "story", "craft", "characters.json"),
		"\uFEFF{\"A\":{\"name\":\"A\"},\"nil\":null}")
	writeFile(t, filepath.Join(root, "story", "craft", "organizations.json"),
		`{"Org":null}`)
	chdir(t, root)

	characters, locations, items, organizations, err := LoadAllElements()
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
