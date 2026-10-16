package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"novelgen/internal/store"
	"novelgen/internal/usage"
)

// The usage ledger's IO moved here with the rest of the project layout:
// internal/usage keeps the record shape and the aggregation, internal/store
// owns logs/usage.jsonl.

func TestUsageLedgerRoundTrip(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	entries := []usage.Entry{
		{Time: base, Agent: "ComposeAgent", Command: "generate outline", Model: "m1", PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, Success: true},
		{Time: base.Add(time.Hour), Agent: "WriteAgent", Model: "m1", TotalTokens: 280, Success: true},
		{Time: base.Add(2 * time.Hour), Agent: "WriteAgent", Model: "m2", Success: false, Error: "AI request failed: 502"},
	}
	for _, entry := range entries {
		if err := root.AppendUsage(entry); err != nil {
			t.Fatalf("AppendUsage: %v", err)
		}
	}

	got, err := root.ReadUsage()
	if err != nil {
		t.Fatalf("ReadUsage: %v", err)
	}
	if len(got) != len(entries) {
		t.Fatalf("read %d entries, want %d", len(got), len(entries))
	}
	if got[0].Agent != "ComposeAgent" || got[2].Error == "" {
		t.Fatalf("unexpected entries: %+v", got)
	}
	if _, ok := root.FS().(*store.MemFS).File(root.UsageLog()); !ok {
		t.Fatalf("ledger was not written to %s", root.UsageLog())
	}
}

func TestReadUsageMissingLedgerIsNotAnError(t *testing.T) {
	t.Parallel()

	entries, err := store.MemRoot().ReadUsage()
	if err != nil {
		t.Fatalf("ReadUsage: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(entries))
	}
}

func TestReadUsageSkipsCorruptLines(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	content := strings.Join([]string{
		`{"agent":"A","total_tokens":1,"success":true}`,
		`not json`,
		`{"agent":"B","total_tokens":2,"success":true}`,
	}, "\n")
	if err := root.WriteText(root.UsageLog(), content); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	entries, err := root.ReadUsage()
	if err != nil {
		t.Fatalf("ReadUsage: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
}

// TestAppendUsageIsSafeUnderConcurrency guards the append-only ledger contract:
// parallel write workers must not interleave partial lines.
func TestAppendUsageIsSafeUnderConcurrency(t *testing.T) {
	t.Parallel()

	root, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(tokens int) {
			defer wg.Done()
			if err := root.AppendUsage(usage.Entry{Time: time.Now(), Agent: "Concurrent", TotalTokens: tokens, Success: true}); err != nil {
				t.Errorf("AppendUsage: %v", err)
			}
		}(i)
	}
	wg.Wait()

	entries, err := root.ReadUsage()
	if err != nil {
		t.Fatalf("ReadUsage: %v", err)
	}
	if len(entries) != 32 {
		t.Fatalf("read %d entries, want 32 (interleaved writes would corrupt lines)", len(entries))
	}
}

// TestAppendUsageCreatesLedgerDirectory covers the fresh-project path.
func TestAppendUsageCreatesLedgerDirectory(t *testing.T) {
	t.Parallel()

	root, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	if err := root.AppendUsage(usage.Entry{Agent: "A", TotalTokens: 1, Success: true}); err != nil {
		t.Fatalf("AppendUsage: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root.Dir(), "logs", "usage.jsonl")); err != nil {
		t.Fatalf("ledger missing: %v", err)
	}
}
