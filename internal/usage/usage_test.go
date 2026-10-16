package usage

import (
	"testing"
	"time"
)

// Reading and appending logs/usage.jsonl is covered by internal/store
// (Root.AppendUsage / Root.ReadUsage); this package only shapes and aggregates
// the records.

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

