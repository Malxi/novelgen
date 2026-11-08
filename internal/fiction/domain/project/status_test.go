package project

import (
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/platform/store"
)

func writeProjectTestFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	target := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openStatusTestProject(t *testing.T, outline string) *store.Root {
	t.Helper()
	dir := t.TempDir()
	writeProjectTestFile(t, dir, "novel.json", `{"name":"Status Test","language":"zh","structure":{"target_parts":1,"target_volumes":1,"target_chapters":2}}`)
	writeProjectTestFile(t, dir, "story/setup/story_setup.json", `{"project_name":"Status Test"}`)
	writeProjectTestFile(t, dir, "story/compose/outline.json", outline)
	root, err := store.OpenProject(dir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	return root
}

func TestBookStatusNextStage(t *testing.T) {
	t.Run("missing prose points at write", func(t *testing.T) {
		root := openStatusTestProject(t, `{"parts":[{"id":"P1","title":"Part","volumes":[{"id":"P1-V1","title":"Volume","chapters":[{"id":"P1-V1-C1","title":"Chapter 1"},{"id":"P1-V1-C2","title":"Chapter 2"}]}]}]}`)
		if err := root.Write(root.ChapterFile("P1-V1-C1"), []byte("# Chapter 1\n\nText."), 0o644); err != nil {
			t.Fatal(err)
		}

		status, err := Open(root).Status()
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if status.Outline.Chapters != 2 || len(status.Chapters) != 2 {
			t.Fatalf("outline chapters = %d, chapters = %d", status.Outline.Chapters, len(status.Chapters))
		}
		if !status.Chapters[0].Written || status.Chapters[1].Written {
			t.Fatalf("written flags = %t, %t", status.Chapters[0].Written, status.Chapters[1].Written)
		}
		if status.Next.Stage != "write" || status.Next.TargetID != "chapter:P1-V1-C2" {
			t.Fatalf("next = %+v", status.Next)
		}
	})

	t.Run("empty volume points at outline", func(t *testing.T) {
		root := openStatusTestProject(t, `{"parts":[{"id":"P1","title":"Part","volumes":[]}]}`)
		status, err := Open(root).Status()
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if status.Next.Stage != "outline" {
			t.Fatalf("next = %+v", status.Next)
		}
		if status.Chapters == nil {
			t.Fatal("chapters should be an empty slice, not nil")
		}
	})
}
