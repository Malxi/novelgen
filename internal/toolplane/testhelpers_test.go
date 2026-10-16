package toolplane_test

import (
	"context"
	"testing"

	"novelgen/internal/store"

	"github.com/spf13/cobra"
)

// containsString reports whether values contains want.
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// toolRoot returns the store root for a fixture project directory.
func toolRoot(t *testing.T, dir string) *store.Root {
	t.Helper()
	root, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open project %s: %v", dir, err)
	}
	return root
}

// toolCmd returns a command whose context carries an explicit project root, so
// the tool entry points resolve a fixture project without chdir-ing the process
// into it.
func toolCmd(t *testing.T, dir string) *cobra.Command {
	t.Helper()
	return bindProject(&cobra.Command{}, toolRoot(t, dir))
}

// bindProject attaches a project root to a command.
func bindProject(cmd *cobra.Command, root *store.Root) *cobra.Command {
	cmd.SetContext(store.WithRoot(context.Background(), root))
	return cmd
}
