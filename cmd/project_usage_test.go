package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelgen/internal/usage"
)

func TestProjectUsageSummarizesRecordedTokens(t *testing.T) {
	root := writeCloneProjectFixture(t)
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	for _, entry := range []usage.Entry{
		{Time: base, Agent: "ComposeAgent", Model: "deepseek-v4-flash", PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500, Success: true},
		{Time: base.Add(time.Minute), Agent: "WriteAgent", Model: "deepseek-v4-flash", PromptTokens: 2000, CompletionTokens: 800, TotalTokens: 2800, Success: true},
	} {
		if err := usage.Append(root, entry); err != nil {
			t.Fatalf("append usage: %v", err)
		}
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	projectUsageJSON = false
	projectUsageSince = ""
	cmd := *projectUsageCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := runProjectUsage(&cmd, nil); err != nil {
		t.Fatalf("runProjectUsage: %v", err)
	}
	text := out.String()
	for _, want := range []string{"Calls: 2", "Tokens: 4300 total", "By agent", "WriteAgent", "By model", "deepseek-v4-flash"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage report missing %q:\n%s", want, text)
		}
	}
}

func TestProjectUsageSinceFilterAndMissingLog(t *testing.T) {
	root := writeCloneProjectFixture(t)

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	projectUsageJSON = false
	projectUsageSince = ""
	cmd := *projectUsageCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runProjectUsage(&cmd, nil); err != nil {
		t.Fatalf("runProjectUsage on empty log: %v", err)
	}
	if !strings.Contains(out.String(), "No usage recorded yet") {
		t.Fatalf("expected empty-log message, got:\n%s", out.String())
	}

	if err := usage.Append(root, usage.Entry{
		Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Agent: "Old", TotalTokens: 5, Success: true,
	}); err != nil {
		t.Fatalf("append usage: %v", err)
	}
	projectUsageSince = "2030-01-01"
	out.Reset()
	if err := runProjectUsage(&cmd, nil); err != nil {
		t.Fatalf("runProjectUsage: %v", err)
	}
	if !strings.Contains(out.String(), "No usage recorded yet") {
		t.Fatalf("expected --since to filter everything out, got:\n%s", out.String())
	}

	projectUsageSince = "not-a-date"
	if err := runProjectUsage(&cmd, nil); err == nil {
		t.Fatalf("expected an error for an invalid --since value")
	}
	projectUsageSince = ""

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(usage.RelativeLogPath))); err != nil {
		t.Fatalf("usage log missing: %v", err)
	}
}
