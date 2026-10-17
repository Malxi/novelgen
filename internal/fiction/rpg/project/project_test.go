package rpgproject

import (
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

// TestLoadNovelgenProjectFromPlainProjectDir pins the fix for the RPG tools'
// workspace assumption: a project is loaded from its own directory, so it works
// outside <workspace>/books/<name> too (for example when selected with
// `novelgen --dir`).
func TestLoadNovelgenProjectFromPlainProjectDir(t *testing.T) {
	t.Parallel()

	bookDir := t.TempDir()
	writeAdapterFixture(t, bookDir, "novel.json",
		`{"name":"Configured Book","language":"zh"}`)
	// A hand-edited craft file may carry a UTF-8 BOM; the adapter strips it like
	// the CLI's craft loader does.
	writeAdapterFixture(t, bookDir, filepath.Join("story", "craft", "characters.json"),
		"\uFEFF{\"Chen\":{\"name\":\"Chen\",\"role_in_story\":\"protagonist\",\"motivation\":\"escape the mine\"}}")
	writeAdapterFixture(t, bookDir, filepath.Join("story", "craft", "items.json"),
		`{"Pickaxe":{"name":"Pickaxe","type":"tool"}}`)
	writeAdapterFixture(t, bookDir, filepath.Join("story", "craft", "locations.json"),
		`{"Mine":{"name":"Mine","description":"a cold shaft"}}`)
	writeAdapterFixture(t, bookDir, filepath.Join("story", "craft", "organizations.json"),
		`{"Bureau":{"name":"Bureau"}}`)
	writeAdapterFixture(t, bookDir, filepath.Join("story", "compose", "outline.json"),
		`{"parts":[{"id":"P1","title":"Part","volumes":[{"id":"P1-V1","title":"Volume","chapters":[{"id":"P1-V1-C1","title":"Chapter","characters":["Chen"],"location":"Mine"}]}]}]}`)

	project, err := LoadNovelgenProject(bookDir)
	if err != nil {
		t.Fatalf("LoadNovelgenProject(%q): %v", bookDir, err)
	}
	if got := project.BookName; got != "Configured Book" {
		t.Fatalf("BookName = %q, want the name from novel.json", got)
	}
	if len(project.Characters) != 1 || project.Characters["Chen"].Motivation == "" {
		t.Fatalf("characters = %#v", project.Characters)
	}
	if len(project.Items) != 1 || len(project.Locations) != 1 || len(project.Organizations) != 1 {
		t.Fatalf("elements were not all loaded: items=%d locations=%d organizations=%d",
			len(project.Items), len(project.Locations), len(project.Organizations))
	}
	if len(project.Outline.Parts) != 1 {
		t.Fatalf("outline parts = %d, want 1", len(project.Outline.Parts))
	}
}

func TestLoadNovelgenProjectAtReadsMemFS(t *testing.T) {
	root := store.MemRoot()
	if err := root.SaveConfig(&models.ProjectConfig{Name: "Memory Book"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if err := root.SaveOutline(&models.Outline{Parts: []models.Part{{ID: "P1"}}}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := root.WriteJSON(root.CraftFile(store.CraftCharacters), map[string]map[string]interface{}{
		"Lin": {"name": "Lin", "role_in_story": "protagonist"},
	}); err != nil {
		t.Fatalf("Save characters: %v", err)
	}
	if err := root.WriteJSON(root.CraftFile(store.CraftItems), map[string]map[string]interface{}{}); err != nil {
		t.Fatalf("Save items: %v", err)
	}
	if err := root.WriteJSON(root.CraftFile(store.CraftLocations), map[string]map[string]interface{}{}); err != nil {
		t.Fatalf("Save locations: %v", err)
	}

	project, err := LoadNovelgenProjectAt(root)
	if err != nil {
		t.Fatalf("LoadNovelgenProjectAt: %v", err)
	}
	if project.Root != root {
		t.Fatalf("project root = %p, want %p", project.Root, root)
	}
	if project.BookName != "Memory Book" {
		t.Fatalf("book name = %q, want Memory Book", project.BookName)
	}
	if _, ok := project.Characters["Lin"]; !ok {
		t.Fatalf("characters = %#v, want Lin", project.Characters)
	}
}

// TestLoadNovelgenProjectFallsBackToDirName covers a project whose novel.json is
// missing or nameless: the directory name is the best available book name.
func TestLoadNovelgenProjectFallsBackToDirName(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	bookDir := filepath.Join(parent, "unnamed-book")
	writeAdapterFixture(t, bookDir, filepath.Join("story", "craft", "characters.json"),
		`{"Chen":{"name":"Chen"}}`)
	writeAdapterFixture(t, bookDir, filepath.Join("story", "craft", "items.json"), `{}`)
	writeAdapterFixture(t, bookDir, filepath.Join("story", "craft", "locations.json"), `{}`)
	writeAdapterFixture(t, bookDir, filepath.Join("story", "compose", "outline.json"), `{"parts":[]}`)

	project, err := LoadNovelgenProject(bookDir)
	if err != nil {
		t.Fatalf("LoadNovelgenProject: %v", err)
	}
	if project.BookName != "unnamed-book" {
		t.Fatalf("BookName = %q, want the directory name", project.BookName)
	}
}

// TestLoadNovelgenProjectReportsMissingElements keeps the failure message
// actionable when a project has not been crafted yet.
func TestLoadNovelgenProjectReportsMissingElements(t *testing.T) {
	t.Parallel()

	_, err := LoadNovelgenProject(t.TempDir())
	if err == nil {
		t.Fatal("expected an error for an empty directory")
	}
}

func writeAdapterFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relative, err)
	}
}
