package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/store"

	"github.com/spf13/cobra"
)

// TestRootPersistentFlags pins the flags every command inherits. --dir is how a
// command is pointed at a project outside the working directory, and the docs
// and skills reference it.
func TestRootPersistentFlags(t *testing.T) {
	root := fullCommandTree()
	for _, flag := range []string{"dir"} {
		if root.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("root command is missing persistent flag --%s", flag)
		}
	}
}

// TestResolveProjectRootInstallsExplicitDir covers the wiring that replaced the
// per-command "am I in the project root?" checks: an explicit directory is
// resolved once, installed as the ambient root, and attached to the command so
// command-scoped lookups see it too.
func TestResolveProjectRootInstallsExplicitDir(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "novel.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatalf("write novel.json: %v", err)
	}

	previous, defaultErr := store.Default()
	hadPrevious := defaultErr == nil
	previousFlag := projectDirFlag
	t.Cleanup(func() {
		projectDirFlag = previousFlag
		if hadPrevious {
			store.SetDefault(previous)
			return
		}
		store.ResetDefault()
	})

	projectDirFlag = projectDir
	cmd := &cobra.Command{}
	if err := resolveProjectRoot(cmd, nil); err != nil {
		t.Fatalf("resolveProjectRoot: %v", err)
	}

	installed, err := store.Default()
	if err != nil {
		t.Fatalf("store.Default: %v", err)
	}
	if installed.Dir() != filepath.Clean(projectDir) {
		t.Fatalf("ambient root = %q, want %q", installed.Dir(), projectDir)
	}
	fromCmd, ok := store.FromContext(cmd.Context())
	if !ok || fromCmd.Dir() != filepath.Clean(projectDir) {
		t.Fatalf("command context root = %v, %v", fromCmd, ok)
	}
}

// TestResolveProjectRootRejectsMissingDir keeps a typo'd --dir from silently
// creating a project somewhere unexpected.
func TestResolveProjectRootRejectsMissingDir(t *testing.T) {
	previousFlag := projectDirFlag
	t.Cleanup(func() { projectDirFlag = previousFlag })

	projectDirFlag = filepath.Join(t.TempDir(), "does-not-exist")
	err := resolveProjectRoot(&cobra.Command{}, nil)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("resolveProjectRoot error = %v, want a missing-directory error", err)
	}
}

// TestResolveProjectRootWithoutProjectIsNotAnError keeps `init` working outside
// any project: resolution is best-effort, and the command reports its own error
// when it actually needs a project. The module root has no novel.json, so the
// walk-up from this test's working directory finds nothing.
func TestResolveProjectRootWithoutProjectIsNotAnError(t *testing.T) {
	previousFlag := projectDirFlag
	t.Cleanup(func() { projectDirFlag = previousFlag })

	projectDirFlag = ""
	if _, err := store.Locate("."); err == nil {
		t.Skip("this checkout has a novel.json above the test directory; the no-project path cannot be exercised")
	}
	if err := resolveProjectRoot(&cobra.Command{}, nil); err != nil {
		t.Fatalf("resolveProjectRoot outside a project should not fail: %v", err)
	}
}
