package toolplane_test

import (
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/store"
	"novelgen/internal/testfixtures"
	"novelgen/internal/toolplane"

	"github.com/spf13/cobra"
)

// TestToolPlaneRunsOnAnInMemoryProject pins the storage seam end to end: the
// agent-facing mutation paths — setup patch, craft patch with checkpoint, patch
// buffer — work on a project that exists only inside a store.MemFS.
//
// This is the assertion the seam exists for. Before project IO was routed through
// internal/store, these paths called os.* on the paths store handed out, so an
// in-memory project was unreachable and every tool-plane fixture had to be built
// on disk.
func TestToolPlaneRunsOnAnInMemoryProject(t *testing.T) {
	// Not parallel: the patch-buffer flags are package-level state, like the
	// other tool-plane tests that set them.
	root := store.MemRoot()
	memFS := root.FS().(*store.MemFS)

	if err := root.SaveConfig(&models.ProjectConfig{Name: "Memory Project"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if err := root.SaveSetup(testfixtures.ValidLongFormSetup()); err != nil {
		t.Fatalf("SaveSetup: %v", err)
	}
	if err := root.SaveOutline(validToolPatchTestOutline()); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := root.WriteJSON(root.CraftFile(store.CraftCharacters), map[string]map[string]interface{}{
		"Lin": {"name": "Lin", "role_in_story": "protagonist"},
	}); err != nil {
		t.Fatalf("seed craft: %v", err)
	}

	// 1. Setup patch: apply writes story_setup.json and the derived markdown.
	setupCmd := &cobra.Command{}
	var setupOut strings.Builder
	setupCmd.SetOut(&setupOut)
	if err := toolplane.RunToolSetupPatch(setupCmd, root, []byte(`{"theme":"updated by setup patch"}`), true, nil); err != nil {
		t.Fatalf("RunToolSetupPatch: %v", err)
	}
	saved, err := root.LoadSetup()
	if err != nil {
		t.Fatalf("LoadSetup: %v", err)
	}
	if saved.Theme != "updated by setup patch" {
		t.Fatalf("setup patch was not written into MemFS: theme = %q", saved.Theme)
	}
	if _, ok := memFS.File(root.SetupMarkdown()); !ok {
		t.Fatalf("setup markdown was not rendered through the store")
	}

	// 2. Craft patch: apply writes the craft file and takes a checkpoint, which
	//    means the pattern that allocates a unique name runs over the seam too.
	craftCmd := &cobra.Command{}
	craftCmd.SetOut(&strings.Builder{})
	if err := toolplane.RunToolCraftPatch(craftCmd, root, "character", "Lin", []byte(`{"notes":"updated by tool"}`), true, nil); err != nil {
		t.Fatalf("RunToolCraftPatch: %v", err)
	}
	values, err := root.LoadCraftRaw(store.CraftCharacters)
	if err != nil {
		t.Fatalf("LoadCraftRaw: %v", err)
	}
	if values["Lin"]["notes"] != "updated by tool" {
		t.Fatalf("craft patch was not written into MemFS: %#v", values["Lin"])
	}
	var checkpoints []string
	for _, name := range memFS.Names() {
		if strings.Contains(name, "/checkpoints/") && strings.HasPrefix(filepath.Base(name), "characters") {
			checkpoints = append(checkpoints, name)
		}
	}
	if len(checkpoints) != 1 {
		t.Fatalf("craft checkpoints in MemFS = %v, want exactly one", checkpoints)
	}

	// 3. Patch buffer: the agent scratch plane appends and reads back through the
	//    store as well.
	originalBufferFlags := toolplane.ToolPatchBufferFlags
	defer func() { toolplane.ToolPatchBufferFlags = originalBufferFlags }()
	toolplane.ToolPatchBufferFlags.ID = "seam"
	toolplane.ToolPatchBufferFlags.Text = "patch text"

	// The patch-buffer entry point resolves its project from the command context
	// (that is how cmd/root.go installs it for a real invocation), so the root has
	// to be bound here too.
	bufferCmd := bindProject(&cobra.Command{}, root)
	bufferCmd.SetOut(&strings.Builder{})
	if err := toolplane.RunToolPatchBuffer(bufferCmd, []string{"append"}); err != nil {
		t.Fatalf("RunToolPatchBuffer: %v", err)
	}
	buffered, err := toolplane.LoadToolPatchBuffer(root, "seam", "setup")
	if err != nil {
		t.Fatalf("LoadToolPatchBuffer: %v", err)
	}
	if string(buffered) != "patch text" {
		t.Fatalf("patch buffer = %q, want %q", buffered, "patch text")
	}

	// Nothing may be left behind on disk-shaped assumptions: every artifact the
	// run produced lives in the in-memory filesystem, with no temp file debris.
	for _, name := range memFS.Names() {
		if strings.Contains(name, ".tmp") {
			t.Fatalf("temp file left in MemFS: %s", name)
		}
	}
	if names := memFS.Names(); len(names) == 0 {
		t.Fatal("MemFS recorded no files at all; the seam test is not exercising anything")
	}
}
