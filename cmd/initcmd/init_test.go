package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
	"novelgen/internal/projectinit"
)

// testBook returns a project rooted at a temporary directory, so these tests
// never have to switch the process working directory.
func testBook(t *testing.T, dir string) *project.Book {
	t.Helper()
	root, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open %s: %v", dir, err)
	}
	return project.Open(root)
}

func TestCreateProjectStructureCreatesDocumentedLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := projectinit.Create(testBook(t, root), projectinit.Options{
		Name:     "test-book",
		Parts:    1,
		Volumes:  1,
		Chapters: 20,
		Provider: "claude",
		Model:    "sonnet",
		Language: "zh",
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	for _, dir := range []string{
		"story/setup", "story/compose", "story/craft", "story/reviews",
		"chapters", "drafts", "logs",
	} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil || !info.IsDir() {
			t.Errorf("directory %s missing: %v", dir, err)
		}
	}

	placeholder, err := os.ReadFile(filepath.Join(root, "story", "setup", "story_setup.md"))
	if err != nil {
		t.Fatalf("read placeholder: %v", err)
	}
	if !strings.Contains(string(placeholder), "Story Setup") {
		t.Fatalf("placeholder should be a usable template:\n%s", placeholder)
	}
}
