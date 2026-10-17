package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain doubles as a fake novelgen CLI: when WEBUI_FAKE_NOVELGEN is set the
// test binary records the arguments it was called with and exits, which lets the
// tests assert the exact CLI invocation without shipping a stub binary.
func TestMain(m *testing.M) {
	if os.Getenv("WEBUI_FAKE_NOVELGEN") == "1" {
		runFakeNovelGen()
		return
	}
	previousStorage := runStorage
	runStorage = hostStorageForTests
	code := m.Run()
	runStorage = previousStorage
	os.Exit(code)
}

func runFakeNovelGen() {
	if path := os.Getenv("WEBUI_FAKE_NOVELGEN_ARGS"); path != "" {
		_ = os.WriteFile(path, []byte(strings.Join(os.Args[1:], " ")), 0644)
	}
	if path := os.Getenv("WEBUI_FAKE_NOVELGEN_MD"); path != "" {
		_ = os.WriteFile(path, []byte("# Story Outline\n"), 0644)
	}
	os.Exit(0)
}

func TestSyncDerivedFileAfterSaveRoutesThroughCLI(t *testing.T) {
	root := t.TempDir()
	outlineDir := filepath.Join(root, "story", "compose")
	if err := os.MkdirAll(outlineDir, 0755); err != nil {
		t.Fatalf("mkdir outline dir: %v", err)
	}

	argsPath := filepath.Join(root, "invocation.txt")
	markdownPath := filepath.Join(outlineDir, "outline.md")
	t.Setenv("NOVELGEN_BIN", executableForTest(t))
	t.Setenv("WEBUI_FAKE_NOVELGEN", "1")
	t.Setenv("WEBUI_FAKE_NOVELGEN_ARGS", argsPath)
	t.Setenv("WEBUI_FAKE_NOVELGEN_MD", markdownPath)

	if err := syncDerivedFileAfterSave(root, "story/compose/outline.json"); err != nil {
		t.Fatalf("sync derived file: %v", err)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read recorded invocation: %v", err)
	}
	if got, want := strings.TrimSpace(string(args)), "compose normalize --force"; got != want {
		t.Fatalf("CLI invocation = %q, want %q", got, want)
	}

	if _, err := os.Stat(markdownPath); err != nil {
		t.Fatalf("derived markdown should be produced by the CLI: %v", err)
	}
}

func TestBuildTaskArgsIsDeterministic(t *testing.T) {
	args := map[string]interface{}{
		"volume":      "2",
		"prompt":      "tighten",
		"agent-sdk":   true,
		"max-rounds":  float64(2),
		"subcommand":  "improve",
		"project_dir": "/tmp/book",
	}
	first := strings.Join(buildTaskArgs("compose", args), "\x00")
	second := strings.Join(buildTaskArgs("compose", args), "\x00")
	if first != second {
		t.Fatalf("task argv is not deterministic:\n first=%q\nsecond=%q", first, second)
	}
	want := "compose\x00improve\x00--agent-sdk\x00--max-rounds\x002\x00--prompt\x00tighten\x00--volume\x002"
	if first != want {
		t.Fatalf("task argv = %q, want %q", first, want)
	}
}

func TestSyncDerivedFileAfterSaveIgnoresOtherPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NOVELGEN_BIN", filepath.Join(root, "must-not-run"))

	if err := syncDerivedFileAfterSave(root, "story/setup/story_setup.json"); err != nil {
		t.Fatalf("non-outline paths need no derived sync: %v", err)
	}
}

func TestSyncDerivedFileAfterSaveSurfacesCLIFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NOVELGEN_BIN", filepath.Join(root, "missing-novelgen"))

	if err := syncDerivedFileAfterSave(root, "story/compose/outline.json"); err == nil {
		t.Fatal("expected an error when the CLI is unavailable")
	}
	if _, err := os.Stat(filepath.Join(root, "story", "compose", "outline.md")); err == nil {
		t.Fatal("webui must not write derived files itself")
	}
}

func TestNovelGenNormalizeCommandUsesProjectDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NOVELGEN_BIN", executableForTest(t))

	cmd := novelGenNormalizeCommand(root)
	if got, want := strings.Join(cmd.Args[1:], " "), "compose normalize --force"; got != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
	if cmd.Dir != root {
		t.Fatalf("cmd.Dir = %q, want %q", cmd.Dir, root)
	}
}

func executableForTest(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return exe
}
