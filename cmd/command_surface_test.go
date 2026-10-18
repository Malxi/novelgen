package cmd

import (
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// commandSurfaceOnce makes sure the shared root command is populated exactly
// once per test binary. RegisterAllCommands appends to the global rootCmd, so
// calling it from several tests would otherwise register duplicates.
var commandSurfaceOnce sync.Once

// fullCommandTree returns the real command tree (registry + commands attached
// directly in init()) after registering every command once.
func fullCommandTree() *cobra.Command {
	commandSurfaceOnce.Do(RegisterAllCommands)
	return rootCmd
}

func childNames(cmd *cobra.Command) []string {
	names := make([]string, 0, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		names = append(names, child.Name())
	}
	sort.Strings(names)
	return names
}

func findChild(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	t.Fatalf("command %q has no subcommand %q", parent.Name(), name)
	return nil
}

// TestCommandSurfaceTopLevel pins the set of top-level CLI commands. This is the
// guardrail for the planned cmd/ packaging refactor: moving command files must
// not add, drop, or rename a user-facing command.
func TestCommandSurfaceTopLevel(t *testing.T) {
	root := fullCommandTree()

	got := childNames(root)
	want := []string{
		"analyze-ai-rpg",
		"analyze-chapter-ai",
		"compose",
		"config",
		"craft",
		"draft",
		"export",
		"init",
		"polish",
		"project",
		"recap",
		"rpg",
		"rpg-dsl",
		"setup",
		"simulate-dsl",
		"storage",
		"tool",
		"translate",
		"write",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("top-level commands changed:\n got=%v\nwant=%v", got, want)
	}
}

// TestCommandSurfaceSubcommands pins the subcommand set of the multi-command
// groups. Keep this in sync intentionally: a diff here means the public CLI
// changed and callers (including SDK skills) may need updating.
func TestCommandSurfaceSubcommands(t *testing.T) {
	root := fullCommandTree()
	want := map[string][]string{
		"compose": {"check", "gen", "improve", "normalize", "pipeline", "regen", "review", "skeleton-improve", "skeleton-review"},
		"config":  {"agent", "set", "show"},
		"craft":   {"gen", "improve"},
		"draft":   {"gen", "improve", "review"},
		"export":  {"novel"},
		"project": {"clone", "doctor", "rename", "usage"},
		"recap":   {"gen"},
		"rpg":     {"bench", "export", "gen", "simulate"},
		"rpg-dsl": {"check", "convert", "convert-chapters", "export-dsl", "merge", "simulate", "validate"},
		"setup":   {"check", "explore", "gen", "import", "improve", "regen"},
		"storage": {"list", "projects", "read", "replace", "stat", "write"},
		"tool":    {"check", "patch", "patch-buffer", "query", "refresh"},
		"write":   {"gen", "improve", "pipeline", "review"},
	}
	for name, expected := range want {
		parent := findChild(t, root, name)
		got := childNames(parent)
		if strings.Join(got, ",") != strings.Join(expected, ",") {
			t.Errorf("%s subcommands changed:\n got=%v\nwant=%v", name, got, expected)
		}
	}
}

// TestCommandSurfaceKeyFlags pins the flags that skills and docs depend on.
func TestCommandSurfaceKeyFlags(t *testing.T) {
	root := fullCommandTree()
	cases := []struct {
		path  []string
		flags []string
	}{
		{[]string{"craft", "gen"}, []string{"agent-sdk", "agent-apply"}},
		{[]string{"craft", "improve"}, []string{"agent-sdk", "agent-apply"}},
		{[]string{"setup", "gen"}, []string{"agent-sdk", "random", "random-seed", "random-level", "random-mode", "random-genre", "scenario", "scenario-seed", "scenario-genre", "scenario-index"}},
		{[]string{"setup", "explore"}, []string{"agent-sdk", "count", "seed", "level", "mode", "random-genre", "apply-best", "strict", "scenario", "select", "plan-only", "predict", "predict-limit"}},
		{[]string{"setup", "regen"}, []string{"agent-sdk", "agent-apply"}},
		{[]string{"setup", "improve"}, []string{"agent-sdk", "agent-apply"}},
		{[]string{"write", "improve"}, []string{"agent-sdk", "agent-apply"}},
		{[]string{"polish"}, []string{"agent-sdk"}},
		{[]string{"tool", "patch"}, []string{"patch-json", "apply"}},
		{[]string{"tool", "query"}, []string{"view"}},
	}
	for _, tc := range cases {
		cmd := root
		for _, part := range tc.path {
			cmd = findChild(t, cmd, part)
		}
		for _, flag := range tc.flags {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%s is missing required flag --%s", strings.Join(tc.path, " "), flag)
			}
		}
	}
}

// TestCommandRegistrationNamesAreUnique guards the self-registration mechanism:
// two files registering the same top-level name would produce duplicate
// commands that can be masked by ordering.
func TestCommandRegistrationNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, registrar := range commandRegistry {
		cmd := registrar()
		if cmd == nil {
			continue
		}
		if seen[cmd.Name()] {
			t.Fatalf("duplicate command registration for %q", cmd.Name())
		}
		seen[cmd.Name()] = true
		if strings.TrimSpace(cmd.Use) == "" || strings.TrimSpace(cmd.Short) == "" {
			t.Errorf("command %q must keep a non-empty Use and Short", cmd.Name())
		}
	}
}
