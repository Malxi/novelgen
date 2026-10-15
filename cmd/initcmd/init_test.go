package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
)

// chdirTo switches the process working directory for the rest of the test.
// t.Chdir needs go1.24 while this module targets go1.21, so it is done manually.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func TestCreateProjectStructureCreatesDocumentedLayout(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)

	config := &models.ProjectConfig{
		Name: "test-book",
		LLM:  models.ProjectLLM{Provider: models.ProviderClaude, Model: "sonnet"},
	}
	if err := createProjectStructure(config); err != nil {
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
