package agents

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/usage"
)

func TestWriteAgentLogFileDoesNotOverwriteConcurrentWrites(t *testing.T) {
	root := t.TempDir()
	prev := logger.Default().ProjectDir()
	logger.Default().SetProjectDir(root)
	t.Cleanup(func() { logger.Default().SetProjectDir(prev) })

	const writers = 24
	paths := make([]string, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path, err := writeAgentLogFile("prompts", "ComposeAgent", "payload")
			if err != nil {
				t.Errorf("writeAgentLogFile: %v", err)
				return
			}
			paths[i] = path
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" {
			t.Fatalf("got empty log path")
		}
		if seen[path] {
			t.Fatalf("duplicate log path allocated: %s", path)
		}
		seen[path] = true
	}

	entries, err := os.ReadDir(filepath.Join(root, "logs", "prompts"))
	if err != nil {
		t.Fatalf("read prompts dir: %v", err)
	}
	if len(entries) != writers {
		t.Fatalf("wrote %d files, want %d", len(entries), writers)
	}
}

func TestWriteAgentLogFileCanBeDisabled(t *testing.T) {
	root := t.TempDir()
	prev := logger.Default().ProjectDir()
	logger.Default().SetProjectDir(root)
	t.Cleanup(func() { logger.Default().SetProjectDir(prev) })

	t.Setenv(AgentLogsDisabledEnv, "1")
	path, err := writeAgentLogFile("responses", "WriteAgent", "payload")
	if err != nil {
		t.Fatalf("writeAgentLogFile: %v", err)
	}
	if path != "" {
		t.Fatalf("expected no path when logs are disabled, got %q", path)
	}
	if _, err := os.Stat(filepath.Join(root, "logs")); !os.IsNotExist(err) {
		t.Fatalf("expected no logs directory, stat err = %v", err)
	}
}

// Every agent call must leave a usage record so long runs can be costed.
func TestBaseAgentExecuteRecordsUsage(t *testing.T) {
	root := t.TempDir()
	prev := logger.Default().ProjectDir()
	logger.Default().SetProjectDir(root)
	t.Cleanup(func() { logger.Default().SetProjectDir(prev) })
	t.Setenv(AgentLogsDisabledEnv, "1")

	client := &fakeStaticClient{content: `{"name":"ok"}`}
	agent := &BaseAgent{name: "TestAgent", client: client, config: &llm.Config{}}

	var output struct {
		Name string `json:"name"`
	}
	if err := agent.Execute(context.Background(), InvokeParams{Command: "record usage"}, struct{}{}, &output); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	entries, err := usage.Read(root)
	if err != nil {
		t.Fatalf("read usage: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("recorded %d usage entries, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Agent != "TestAgent" || entry.Command != "record usage" || entry.Kind != "chat" {
		t.Fatalf("unexpected usage entry: %+v", entry)
	}
	if entry.TotalTokens != 10 || !entry.Success {
		t.Fatalf("unexpected usage totals: %+v", entry)
	}
}
