package projectcmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"novelgen/internal/platform/store"
	"novelgen/internal/platform/usage"
)

// useProjectRoot points the ambient project root at dir for the rest of the
// test, so project commands read a fixture without a process chdir.
func useProjectRoot(t *testing.T, dir string) {
	t.Helper()
	root, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open %s: %v", dir, err)
	}
	previous, _ := store.Default()
	store.SetDefault(root)
	t.Cleanup(func() {
		if previous != nil {
			store.SetDefault(previous)
			return
		}
		store.ResetDefault()
	})
}

func TestProjectUsageSummarizesRecordedTokens(t *testing.T) {
	root := writeCloneProjectFixture(t)
	projectRoot, err := store.Open(root)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	for _, entry := range []usage.Entry{
		{Time: base, Agent: "ComposeAgent", Model: "deepseek-v4-flash", PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500, Success: true},
		{Time: base.Add(time.Minute), Agent: "WriteAgent", Model: "deepseek-v4-flash", PromptTokens: 2000, CompletionTokens: 800, TotalTokens: 2800, Success: true},
	} {
		if err := projectRoot.AppendUsage(entry); err != nil {
			t.Fatalf("append usage: %v", err)
		}
	}

	useProjectRoot(t, root)

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
	projectRoot, err := store.Open(root)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	useProjectRoot(t, root)

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

	if err := projectRoot.AppendUsage(usage.Entry{
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

	if _, err := os.Stat(projectRoot.UsageLog()); err != nil {
		t.Fatalf("usage log missing: %v", err)
	}
}
