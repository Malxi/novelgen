package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/store"
)

func TestRelativeProjectIO(t *testing.T) {
	root := store.MemRoot()
	if err := root.WriteRelative("story/compose/custom.json", []byte(`{"ok":true}`), store.FileMode); err != nil {
		t.Fatalf("WriteRelative: %v", err)
	}
	data, err := root.ReadRelative("story/compose/custom.json")
	if err != nil || string(data) != `{"ok":true}` {
		t.Fatalf("ReadRelative = %q, %v", data, err)
	}
	if info, err := root.StatRelative("story/compose/custom.json"); err != nil || info.IsDir() {
		t.Fatalf("StatRelative = %#v, %v", info, err)
	}
	entries, err := root.ListRelative("story/compose")
	if err != nil || len(entries) != 1 || entries[0].Name() != "custom.json" {
		t.Fatalf("ListRelative = %#v, %v", entries, err)
	}
	if _, err := root.ReadRelative("../secret.txt"); err == nil {
		t.Fatal("ReadRelative accepted traversal")
	}
	if err := root.WriteRelative("story/../../secret.txt", []byte("x"), store.FileMode); err == nil {
		t.Fatal("WriteRelative accepted traversal")
	}
}

func TestHostBackendListProjects(t *testing.T) {
	workspace := t.TempDir()
	valid := filepath.Join(workspace, "valid")
	if err := os.MkdirAll(valid, 0o755); err != nil {
		t.Fatalf("mkdir valid: %v", err)
	}
	if err := os.WriteFile(filepath.Join(valid, "novel.json"), []byte(`{"name":"Configured"}`), 0o644); err != nil {
		t.Fatalf("write novel.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "empty"), 0o755); err != nil {
		t.Fatalf("mkdir empty: %v", err)
	}

	projects, err := (store.HostBackend{}).ListProjects(workspace)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("projects = %#v, want 2 entries", projects)
	}
	var found bool
	for _, project := range projects {
		if project.Exists {
			found = true
			if project.Name != "Configured" {
				t.Fatalf("project name = %q, want Configured", project.Name)
			}
		}
	}
	if !found {
		t.Fatal("ListProjects did not mark the valid project as existing")
	}
}

func TestRelativePathRejectsSymlinkEscape(t *testing.T) {
	rootDir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	link := filepath.Join(rootDir, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	root := store.At(rootDir)
	if _, err := root.ReadRelative("link/secret.txt"); err == nil {
		t.Fatal("ReadRelative followed a symlink outside the project")
	}
}
