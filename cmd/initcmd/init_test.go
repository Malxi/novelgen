package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"
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

	config := &models.ProjectConfig{
		Name: "test-book",
		LLM:  models.ProjectLLM{Provider: models.ProviderClaude, Model: "sonnet"},
	}
	if err := createProjectStructure(testBook(t, root), config); err != nil {
		t.Fatalf("createProjectStructure: %v", err)
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

func TestSplitAndTrimDropsBlanks(t *testing.T) {
	got := splitAndTrim(" fantasy , , wuxia ,")
	if len(got) != 2 || got[0] != "fantasy" || got[1] != "wuxia" {
		t.Fatalf("splitAndTrim = %#v", got)
	}
	if got := splitAndTrim(""); len(got) != 0 {
		t.Fatalf("empty input should yield no entries: %#v", got)
	}
}
