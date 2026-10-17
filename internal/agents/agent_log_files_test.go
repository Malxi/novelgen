package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/store"
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
			path, err := (&BaseAgent{}).saveLogStream("prompts", "ComposeAgent", "payload")
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
	path, err := (&BaseAgent{}).saveLogStream("responses", "WriteAgent", "payload")
	if err != nil {
		t.Fatalf("saveLogStream: %v", err)
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

	projectRoot, err := store.Open(root)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	entries, err := projectRoot.ReadUsage()
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

// The web UI's AI-call viewer reads logs/ai_calls/<id>.json, so every call must
// leave one structured record with the model, both prompts, the response and the
// token usage.
func TestBaseAgentExecuteWritesStructuredAICallRecord(t *testing.T) {
	root := t.TempDir()
	prev := logger.Default().ProjectDir()
	logger.Default().SetProjectDir(root)
	t.Cleanup(func() { logger.Default().SetProjectDir(prev) })

	client := &fakeStaticClient{content: `{"name":"ok"}`}
	agent := &BaseAgent{name: "TestAgent", client: client, config: &llm.Config{}}

	var output struct {
		Name string `json:"name"`
	}
	if err := agent.Execute(context.Background(), InvokeParams{Command: "compose skeleton"}, struct{}{}, &output); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	dir := filepath.Join(root, "logs", "ai_calls")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read ai_calls dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d AI call records, want 1", len(entries))
	}
	name := entries[0].Name()
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".json") {
		t.Fatalf("record name = %q, want a bare .json file name", name)
	}

	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var record aiCallRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if record.Agent != "TestAgent" || record.Command != "compose skeleton" {
		t.Fatalf("record identity = %+v", record)
	}
	if strings.TrimSpace(record.SystemPrompt) == "" || strings.TrimSpace(record.UserPrompt) == "" {
		t.Fatalf("record prompts are empty: %+v", record)
	}
	if record.Response != `{"name":"ok"}` {
		t.Fatalf("record response = %q", record.Response)
	}
	if record.Usage.TotalTokens != 10 {
		t.Fatalf("record usage = %+v, want the client's totals", record.Usage)
	}
	if record.CompletedAt.IsZero() || record.StartedAt.IsZero() || record.CompletedAt.Before(record.StartedAt) {
		t.Fatalf("record timestamps = %v → %v", record.StartedAt, record.CompletedAt)
	}
}

// The same switch that turns off the markdown prompt/response logs turns off the
// structured records.
func TestBaseAgentExecuteSkipsStructuredAICallRecordWhenLogsDisabled(t *testing.T) {
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
	if err := agent.Execute(context.Background(), InvokeParams{Command: "compose skeleton"}, struct{}{}, &output); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "logs", "ai_calls")); !os.IsNotExist(err) {
		t.Fatalf("expected no ai_calls directory when logs are disabled, stat err = %v", err)
	}
}

// The record carries the fields the message-level test cannot reach (model,
// skills, token split) straight from the invoke parameters.
func TestRecordAICallWritesModelSkillsAndTokenSplit(t *testing.T) {
	root := t.TempDir()
	prev := logger.Default().ProjectDir()
	logger.Default().SetProjectDir(root)
	t.Cleanup(func() { logger.Default().SetProjectDir(prev) })

	agent := &BaseAgent{name: "CraftAgent"}
	startedAt := time.Date(2026, time.October, 17, 1, 2, 3, 0, time.UTC)
	agent.recordAICall(
		InvokeParams{Command: "craft improve", Skills: []string{"compose", "craft"}},
		"deepseek-v4-pro",
		"system prompt",
		"user prompt",
		"response body",
		llm.Usage{PromptTokens: 1200, CompletionTokens: 800, TotalTokens: 2000},
		startedAt,
	)

	entries, err := os.ReadDir(filepath.Join(root, "logs", "ai_calls"))
	if err != nil {
		t.Fatalf("read ai_calls dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d AI call records, want 1", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(root, "logs", "ai_calls", entries[0].Name()))
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var record aiCallRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	if record.Model != "deepseek-v4-pro" {
		t.Fatalf("record model = %q", record.Model)
	}
	if len(record.Skills) != 2 || record.Skills[0] != "compose" || record.Skills[1] != "craft" {
		t.Fatalf("record skills = %v", record.Skills)
	}
	if record.Usage.PromptTokens != 1200 || record.Usage.CompletionTokens != 800 || record.Usage.TotalTokens != 2000 {
		t.Fatalf("record usage = %+v", record.Usage)
	}
	if !record.StartedAt.Equal(startedAt) {
		t.Fatalf("record started_at = %v, want %v", record.StartedAt, startedAt)
	}
	if record.Response != "response body" || record.SystemPrompt != "system prompt" || record.UserPrompt != "user prompt" {
		t.Fatalf("record payload = %+v", record)
	}
}
