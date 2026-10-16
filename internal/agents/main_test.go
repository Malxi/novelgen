package agents

import (
	"os"
	"testing"

	"novelgen/internal/store"
)

// TestMain keeps this package's tests from writing agent logs into the source
// tree. Agent logging falls back to the process working directory when no project
// is pinned, so every test that exercises Execute without caring where the logs
// go used to leave prompts/, responses/, ai_calls/ and usage.jsonl behind in
// internal/agents/logs (gitignored, but still debris — and it made "did the tests
// write anywhere unexpected" impossible to answer). Tests that need a specific
// project still pin it themselves (they install their own default root).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "novelgen-agent-logs-")
	if err != nil {
		panic(err)
	}
	// The package resolves its project as "logger project dir → default root →
	// working directory". Pinning the default root (rather than the logger's
	// directory) keeps that resolution order intact for tests that set their own.
	root, err := store.Open(dir)
	if err != nil {
		panic(err)
	}
	store.SetDefault(root)

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}
