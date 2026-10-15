package export

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

func TestCleanMarkdownForTXTStripsFormatting(t *testing.T) {
	input := "\n\n# Chapter One\n\n**bold** and *italic* and __underlined__\n\n---\n\nplain line\n"

	got := cleanMarkdownForTXT(input)

	for _, want := range []string{"Chapter One", "bold and italic and underlined", "plain line"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"#", "**", "__", "\n---"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("output still contains markdown %q:\n%s", unwanted, got)
		}
	}
	if strings.HasPrefix(got, "\n") {
		t.Fatalf("leading blank lines should be dropped:\n%q", got)
	}
}

func TestCleanMarkdownForTXTKeepsParagraphBreaks(t *testing.T) {
	got := cleanMarkdownForTXT("first paragraph\n\nsecond paragraph\n")
	if got != "first paragraph\n\nsecond paragraph" {
		t.Fatalf("paragraph separation must survive TXT cleanup, got %q", got)
	}
}

func TestCleanMarkdownForTXTDropsRulesButNotText(t *testing.T) {
	got := cleanMarkdownForTXT("before\n\n***\n\nafter\n")
	if got != "before\n\nafter" {
		t.Fatalf("horizontal rule handling = %q", got)
	}
}

func TestBuildNovelContentFormats(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"name":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chapterDir, "chapter-P1-V1-C1.md"), []byte("# Opening\n\nBody text.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	chdirTo(t, root)

	config := &models.ProjectConfig{Name: "test"}
	setup := &models.StorySetup{ProjectName: "My Novel"}
	chapters := []*models.Chapter{
		{ID: "P1-V1-C1", Title: "Opening"},
		{ID: "P1-V1-C2", Title: "Missing"}, // no file on disk -> skipped
	}
	outline := &models.Outline{}

	markdown, err := buildNovelContent(config, setup, outline, chapters, "md")
	if err != nil {
		t.Fatalf("markdown build: %v", err)
	}
	for _, want := range []string{"# My Novel", "## 第1章 Opening", "Body text."} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "Missing") {
		t.Fatalf("chapters without a file must be skipped:\n%s", markdown)
	}

	plain, err := buildNovelContent(config, setup, outline, chapters, "txt")
	if err != nil {
		t.Fatalf("txt build: %v", err)
	}
	if !strings.HasPrefix(plain, "My Novel\n\n") {
		t.Fatalf("txt should start with the plain title:\n%q", plain)
	}
	if strings.Contains(plain, "#") || !strings.Contains(plain, "第1章 Opening") {
		t.Fatalf("txt should not carry markdown markers:\n%s", plain)
	}
	if !strings.Contains(plain, "Body text.") {
		t.Fatalf("txt should include the cleaned chapter body:\n%s", plain)
	}
}
