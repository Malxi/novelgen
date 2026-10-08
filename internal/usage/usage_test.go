package usage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAppendAndReadRoundTrip(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)

	entries := []Entry{
		{Time: base, Agent: "ComposeAgent", Command: "generate outline", Model: "m1", PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, Success: true},
		{Time: base.Add(time.Hour), Agent: "WriteAgent", Command: "write chapter", Model: "m1", PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280, Success: true},
		{Time: base.Add(2 * time.Hour), Agent: "WriteAgent", Command: "write chapter", Model: "m2", TotalTokens: 0, Success: false, Error: "AI request failed: 502"},
	}
	for _, entry := range entries {
		if err := Append(root, entry); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	got, err := Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != len(entries) {
		t.Fatalf("read %d entries, want %d", len(got), len(entries))
	}
	if got[0].Agent != "ComposeAgent" || got[2].Error == "" {
		t.Fatalf("unexpected entries: %+v", got)
	}
}

func TestReadMissingLogIsNotAnError(t *testing.T) {
	entries, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(entries))
	}
}

func TestSummarizeGroupsAndTotals(t *testing.T) {
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	summary := Summarize([]Entry{
		{Time: base, Agent: "ComposeAgent", Model: "m1", PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, Success: true},
		{Time: base.Add(time.Minute), Agent: "ComposeAgent", Model: "m1", PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, Success: true},
		{Time: base.Add(24 * time.Hour), Agent: "WriteAgent", Model: "m2", PromptTokens: 100, CompletionTokens: 1, TotalTokens: 101, Success: true},
		{Time: base.Add(25 * time.Hour), Agent: "WriteAgent", Model: "m2", Success: false, Error: "boom"},
	})

	if summary.Calls != 4 || summary.FailedCalls != 1 {
		t.Fatalf("calls=%d failed=%d, want 4/1", summary.Calls, summary.FailedCalls)
	}
	if summary.TotalTokens != 146 {
		t.Fatalf("total tokens = %d, want 146", summary.TotalTokens)
	}
	if len(summary.ByAgent) != 2 || summary.ByAgent[0].Key != "WriteAgent" {
		t.Fatalf("by agent = %+v, want WriteAgent first (most tokens)", summary.ByAgent)
	}
	if len(summary.ByDay) != 2 {
		t.Fatalf("by day = %+v, want two days", summary.ByDay)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Key != "boom" {
		t.Fatalf("failures = %+v", summary.Failures)
	}
	if summary.FirstCall != base.Format(time.RFC3339) {
		t.Fatalf("first call = %q, want %q", summary.FirstCall, base.Format(time.RFC3339))
	}
}

func TestFilterSince(t *testing.T) {
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	entries := []Entry{
		{Time: base},
		{Time: base.Add(time.Hour)},
	}
	filtered := FilterSince(entries, base.Add(30*time.Minute))
	if len(filtered) != 1 || !filtered[0].Time.Equal(base.Add(time.Hour)) {
		t.Fatalf("filtered = %+v", filtered)
	}
}

func TestAppendIsSafeUnderConcurrency(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Append(root, Entry{Time: time.Now(), Agent: "Concurrent", TotalTokens: i, Success: true}); err != nil {
				t.Errorf("Append: %v", err)
			}
		}(i)
	}
	wg.Wait()

	entries, err := Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(entries) != 32 {
		t.Fatalf("read %d entries, want 32 (interleaved writes would corrupt lines)", len(entries))
	}
}

func TestReadSkipsCorruptLines(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`{"agent":"A","total_tokens":1,"success":true}`,
		`not json`,
		`{"agent":"B","total_tokens":2,"success":true}`,
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(RelativeLogPath)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(root)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
}
