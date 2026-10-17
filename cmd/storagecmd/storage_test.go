package storagecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/store"
	"novelgen/internal/testfixtures"
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

func TestStorageReplaceRoutesSetupThroughMutationService(t *testing.T) {
	dir := t.TempDir()
	setup := testfixtures.ValidLongFormSetup()
	data, err := json.Marshal(setup)
	if err != nil {
		t.Fatalf("marshal setup: %v", err)
	}

	out, err := runStorageCommand(t, string(data), "replace", "--project", dir, "--path", "story/setup/story_setup.json")
	if err != nil {
		t.Fatalf("replace setup: %v (%s)", err, out)
	}

	loaded, err := store.At(dir).LoadSetup()
	if err != nil {
		t.Fatalf("load replaced setup: %v", err)
	}
	if loaded.ProjectName != setup.ProjectName || loaded.Premise != setup.Premise {
		t.Fatalf("setup was not replaced through the mutation service: %#v", loaded)
	}
	if _, err := os.Stat(filepath.Join(dir, "story", "setup", "story_setup.md")); err != nil {
		t.Fatalf("derived setup markdown missing: %v", err)
	}
}

func TestStorageReplaceCraftFileRemovesOmittedElements(t *testing.T) {
	dir := t.TempDir()
	root := store.At(dir)
	if err := root.WriteJSON(root.CraftFile(store.CraftCharacters), map[string]models.Character{
		"Mira": {Name: "Mira", RoleInStory: "ally"},
		"Bo":   {Name: "Bo", RoleInStory: "rival"},
	}); err != nil {
		t.Fatalf("seed characters: %v", err)
	}
	data, err := json.Marshal(map[string]models.Character{
		"Mira": {Name: "Mira", RoleInStory: "ally"},
	})
	if err != nil {
		t.Fatalf("marshal characters: %v", err)
	}

	out, err := runStorageCommand(t, string(data), "replace", "--project", dir, "--path", "story/craft/characters.json")
	if err != nil {
		t.Fatalf("replace characters: %v (%s)", err, out)
	}
	loaded, _, _, _, err := root.LoadElements()
	if err != nil {
		t.Fatalf("load characters: %v", err)
	}
	if _, ok := loaded["Bo"]; ok {
		t.Fatalf("craft replace must remove omitted elements, got %#v", loaded)
	}
	if _, ok := loaded["Mira"]; !ok {
		t.Fatalf("craft replace lost the retained element: %#v", loaded)
	}
}
