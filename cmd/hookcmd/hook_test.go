package hookcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

func writeTestFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	target := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func newHookProject(t *testing.T) *store.Root {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "novel.json",
		`{"name":"Hook Test","language":"zh","structure":{"target_parts":1,"target_volumes":1,"target_chapters":2}}`)
	writeTestFile(t, dir, "story/setup/story_setup.json", `{"project_name":"Hook Test"}`)
	root, err := store.OpenProject(dir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	return root
}

func writeOutline(t *testing.T, root *store.Root) {
	t.Helper()
	writeTestFile(t, root.Dir(), "story/compose/outline.json",
		`{"parts":[{"id":"P1","title":"Part","volumes":[{"id":"P1-V1","title":"Volume","chapters":[{"id":"P1-V1-C1","title":"Chapter 1"},{"id":"P1-V1-C2","title":"Chapter 2"}]}]}]}`)
}

func TestSessionContextCountsOnlyWrittenChapters(t *testing.T) {
	t.Run("no chapter files", func(t *testing.T) {
		root := newHookProject(t)
		writeOutline(t, root)

		context := sessionContext(project.Open(root))
		if !strings.Contains(context, "2 chapters, 0 written") {
			t.Fatalf("context does not report zero written chapters:\n%s", context)
		}
		if !strings.Contains(context, "\nNext: write\n") {
			t.Fatalf("context should point at writing, not review:\n%s", context)
		}
	})

	t.Run("one of two written", func(t *testing.T) {
		root := newHookProject(t)
		writeOutline(t, root)
		if err := root.Write(root.ChapterFile("P1-V1-C1"), []byte("# Chapter 1\n\nText."), 0o644); err != nil {
			t.Fatalf("write chapter: %v", err)
		}

		context := sessionContext(project.Open(root))
		if !strings.Contains(context, "2 chapters, 1 written") {
			t.Fatalf("context does not report one written chapter:\n%s", context)
		}
		if !strings.Contains(context, "\nNext: write\n") {
			t.Fatalf("context should point at writing the remaining chapter:\n%s", context)
		}
	})

	t.Run("all written", func(t *testing.T) {
		root := newHookProject(t)
		writeOutline(t, root)
		for _, id := range []string{"P1-V1-C1", "P1-V1-C2"} {
			if err := root.Write(root.ChapterFile(id), []byte("# Chapter\n\nText."), 0o644); err != nil {
				t.Fatalf("write chapter %s: %v", id, err)
			}
		}

		context := sessionContext(project.Open(root))
		if !strings.Contains(context, "2 chapters, 2 written") {
			t.Fatalf("context does not report two written chapters:\n%s", context)
		}
		if !strings.Contains(context, "\nNext: review\n") {
			t.Fatalf("context should point at review:\n%s", context)
		}
	})
}

func TestSessionContextOutlineWithoutChapters(t *testing.T) {
	root := newHookProject(t)
	writeTestFile(t, root.Dir(), "story/compose/outline.json", `{"parts":[]}`)

	context := sessionContext(project.Open(root))
	if !strings.Contains(context, "\nNext: outline\n") {
		t.Fatalf("empty outline should point at the outline stage:\n%s", context)
	}
}
