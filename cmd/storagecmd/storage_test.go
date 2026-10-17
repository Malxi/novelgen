package storagecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runStorageCommand(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := NewCommand()
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(stdin))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return out.String(), err
}

func TestStorageReadListStatAndWrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "story", "compose"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "story", "compose", "outline.json"), []byte(`{"parts":[]}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, err := runStorageCommand(t, "", "read", "--project", dir, "--path", "story/compose/outline.json")
	if err != nil {
		t.Fatalf("read: %v (%s)", err, out)
	}
	var resp response
	if err := json.Unmarshal([]byte(out), &resp); err != nil || !resp.OK || !strings.Contains(resp.Content, "parts") {
		t.Fatalf("read response = %s, %v", out, err)
	}

	out, err = runStorageCommand(t, "", "list", "--project", dir, "--path", "story/compose")
	if err != nil {
		t.Fatalf("list: %v (%s)", err, out)
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil || len(resp.Entries) != 1 || resp.Entries[0].Name != "outline.json" {
		t.Fatalf("list response = %s, %v", out, err)
	}

	out, err = runStorageCommand(t, "hello", "write", "--project", dir, "--path", "story/compose/notes.txt")
	if err != nil {
		t.Fatalf("write: %v (%s)", err, out)
	}
	data, err := os.ReadFile(filepath.Join(dir, "story", "compose", "notes.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("written = %q, %v", data, err)
	}

	out, err = runStorageCommand(t, "", "stat", "--project", dir, "--path", "story/compose/notes.txt")
	if err != nil {
		t.Fatalf("stat: %v (%s)", err, out)
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil || resp.Entry == nil || resp.Entry.Size != 5 {
		t.Fatalf("stat response = %s, %v", out, err)
	}
}

func TestStorageProjectsAndPathTraversal(t *testing.T) {
	workspace := t.TempDir()
	projectDir := filepath.Join(workspace, "one")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "novel.json"), []byte(`{"name":"One"}`), 0o644); err != nil {
		t.Fatalf("novel.json: %v", err)
	}

	out, err := runStorageCommand(t, "", "projects", "--root", workspace)
	if err != nil {
		t.Fatalf("projects: %v (%s)", err, out)
	}
	var resp response
	if err := json.Unmarshal([]byte(out), &resp); err != nil || len(resp.Projects) != 1 || resp.Projects[0].Name != "One" {
		t.Fatalf("projects response = %s, %v", out, err)
	}

	out, err = runStorageCommand(t, "", "read", "--project", projectDir, "--path", "../outside.txt")
	if err == nil {
		t.Fatalf("traversal read succeeded: %s", out)
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil || resp.OK || resp.Code != "invalid_path" {
		t.Fatalf("traversal response = %s, %v", out, err)
	}
}
