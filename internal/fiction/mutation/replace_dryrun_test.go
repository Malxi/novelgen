package mutation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	models "novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

func newReplaceTestRoot(t *testing.T) *store.Root {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("novel.json", `{"name":"Replace Test","language":"zh","structure":{"target_parts":1,"target_volumes":1,"target_chapters":2}}`)
	write("story/setup/story_setup.json", `{"project_name":"Replace Test","premise":"before"}`)
	write("story/compose/outline.json", `{"parts":[{"id":"P1","title":"Part","volumes":[{"id":"P1-V1","title":"Volume","chapters":[{"id":"P1-V1-C1","title":"Chapter 1"},{"id":"P1-V1-C2","title":"Chapter 2"}]}]}]}`)
	root, err := store.OpenProject(dir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	if err := root.Write(root.ChapterFile("P1-V1-C1"), []byte("# Chapter 1\n\nText."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := root.Write(root.ChapterFile("P1-V1-C2"), []byte("# Chapter 2\n\nText."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := root.Write(root.RecapFile("P1-V1-C2"), []byte(`{"chapter_id":"P1-V1-C2","summary":"s"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestReplaceOutlineDryRunReportsRemovedChaptersWithoutWriting(t *testing.T) {
	root := newReplaceTestRoot(t)
	before, err := os.ReadFile(root.OutlineJSON())
	if err != nil {
		t.Fatal(err)
	}

	candidate := &models.Outline{Parts: []models.Part{{
		ID:    "P1",
		Title: "Part",
		Volumes: []models.Volume{{
			ID:    "P1-V1",
			Title: "Volume",
			Chapters: []models.Chapter{{
				ID:    "P1-V1-C1",
				Title: "Chapter 1",
			}},
		}},
	}}}
	result, err := NewMutationService(root).ReplaceOutline(context.Background(), candidate, MutationOptions{
		DryRun:   true,
		Validate: false,
		Source:   "test",
	})
	if err != nil {
		t.Fatalf("dry-run outline replace: %v", err)
	}
	if result.Applied || !result.DryRun {
		t.Fatalf("dry run result = applied %t, dry-run %t", result.Applied, result.DryRun)
	}
	after, err := os.ReadFile(root.OutlineJSON())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run wrote the outline")
	}
	warnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(warnings, "removes chapter P1-V1-C2") {
		t.Fatalf("missing removed-chapter warning:\n%s", warnings)
	}
	if !strings.Contains(warnings, "saved chapter content and recap exist") {
		t.Fatalf("missing prose/recap detail:\n%s", warnings)
	}
}

func TestReplaceSetupDryRunDoesNotWrite(t *testing.T) {
	root := newReplaceTestRoot(t)
	before, err := os.ReadFile(root.SetupJSON())
	if err != nil {
		t.Fatal(err)
	}

	setup := &models.StorySetup{ProjectName: "Changed"}
	result, err := NewMutationService(root).ReplaceSetup(context.Background(), setup, MutationOptions{
		DryRun:   true,
		Validate: false,
		Source:   "test",
	})
	if err != nil {
		t.Fatalf("dry-run setup replace: %v", err)
	}
	if result.Applied || !result.DryRun {
		t.Fatalf("dry run result = applied %t, dry-run %t", result.Applied, result.DryRun)
	}
	after, err := os.ReadFile(root.SetupJSON())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run wrote the story setup")
	}

	if _, err := NewMutationService(root).ReplaceSetup(context.Background(), setup, MutationOptions{
		Validate: false,
		Source:   "test",
	}); err != nil {
		t.Fatalf("apply setup replace: %v", err)
	}
	applied, err := os.ReadFile(root.SetupJSON())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(applied), `"project_name": "Changed"`) {
		t.Fatalf("setup was not written on apply:\n%s", applied)
	}
}
