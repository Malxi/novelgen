package agents

import (
	"os"
	"testing"

	"novelgen/internal/store"
)

func TestBaseAgentUsesBoundProjectRootForWorkspaceAndLogs(t *testing.T) {
	bound, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open bound project: %v", err)
	}
	ambient, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open ambient project: %v", err)
	}
	previous, _ := store.Default()
	store.SetDefault(ambient)
	t.Cleanup(func() {
		if previous != nil {
			store.SetDefault(previous)
			return
		}
		store.ResetDefault()
	})

	agent := NewBaseAgent(BaseAgentConfig{Name: "TestAgent"})
	agent.SetProjectRoot(bound)
	if got := agent.workspaceRoot(); got != bound.Dir() {
		t.Fatalf("workspace root = %q, want %q", got, bound.Dir())
	}
	path, err := agent.saveLogStream("prompts", "TestAgent", "payload")
	if err != nil {
		t.Fatalf("save log stream: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log file was not written under bound root: %v", err)
	}
	if _, err := os.Stat(ambient.LogsDir()); err == nil {
		t.Fatalf("agent log was written under the ambient project: %s", ambient.LogsDir())
	}
}
