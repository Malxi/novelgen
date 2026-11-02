package runmetrics

import (
	"context"
	"strings"
	"testing"

	"novelgen/internal/platform/store"
)

func TestRunLedgerSummarizesCallsToolsSkillsAndStuck(t *testing.T) {
	root := store.MemRoot()
	ctx, run, err := Start(context.Background(), root, "novelgen write improve --chapter 1")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if run == nil || FromContext(ctx) != run {
		t.Fatal("run was not attached to context")
	}
	allowed := true
	if err := run.Emit(Event{
		Event:        EventSkillLoad,
		Agent:        "WriteAgent",
		Skills:       []string{"write-improve"},
		LoadedSkills: []string{"write-improve-workflow"},
	}); err != nil {
		t.Fatalf("emit skill event: %v", err)
	}
	if err := run.Emit(Event{
		Event:             EventTool,
		Agent:             "WriteAgent",
		Command:           "novelgen tool patch chapter --id C1 --patch-json {\"content\":\"secret\"}",
		ToolName:          "Bash",
		Allowed:           &allowed,
		DurationMS:        42,
		NormalizedCommand: "novelgen tool patch chapter --id --patch-json <redacted>",
	}); err != nil {
		t.Fatalf("emit tool event: %v", err)
	}
	run.Emit(Event{Event: EventStuck, Agent: "WriteAgent", Reason: "repeated_command:check (3)"})
	run.Emit(Event{
		Event:            EventAgentCall,
		Agent:            "WriteAgent",
		Model:            "deepseek",
		Command:          "improve",
		Status:           "ok",
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	})
	run.Finish("", nil)

	runs, err := List(root, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	got := runs[0]
	if got.Status != StatusCompleted || got.AgentCalls != 1 || got.ToolCalls != 1 || got.TotalTokens != 30 {
		t.Fatalf("summary = %#v", got)
	}
	if got.StuckEvents != 1 || len(got.Skills) != 2 {
		t.Fatalf("stuck/skills = %#v", got)
	}
	if strings.Contains(got.Command, "secret") {
		t.Fatalf("run summary leaked command payload: %q", got.Command)
	}

	detail, err := Read(root, run.ID())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(detail.Events) != 6 {
		t.Fatalf("events = %d, want 6", len(detail.Events))
	}
}

func TestMaskCommandRedactsPayloadFlags(t *testing.T) {
	got := MaskCommand(`novelgen tool patch chapter --patch-json "{\"content\":\"private\"}" --apply --prompt secret`)
	if strings.Contains(got, "private") || strings.Contains(got, "secret") {
		t.Fatalf("MaskCommand leaked payload: %q", got)
	}
	if !strings.Contains(got, "--patch-json <redacted>") || !strings.Contains(got, "--apply") {
		t.Fatalf("MaskCommand lost structure: %q", got)
	}
}

func TestListKeepsReadableRunsWhenLedgerHasMalformedTail(t *testing.T) {
	root := store.MemRoot()
	_, run, err := Start(context.Background(), root, "novelgen test")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	run.Finish("", nil)
	if err := root.Append(root.RunMetricsFile(run.ID()), []byte(`{"broken":`), store.FileMode); err != nil {
		t.Fatalf("append malformed tail: %v", err)
	}

	runs, err := List(root, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(runs) != 1 || runs[0].Status != StatusCompleted {
		t.Fatalf("runs = %#v", runs)
	}
	if len(runs[0].Errors) == 0 {
		t.Fatalf("malformed tail should be surfaced as an error event: %#v", runs[0])
	}
}

func TestSummaryDetectsRepeatedAllowedCommandsAcrossCalls(t *testing.T) {
	root := store.MemRoot()
	_, run, err := Start(context.Background(), root, "novelgen write")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	allowed := true
	for i := 0; i < 3; i++ {
		if err := run.Emit(Event{
			Event:   EventTool,
			Agent:   "WriteAgent",
			Kind:    "pre",
			Command: "novelgen tool check all --target chapter --scope chapter --id 'P1-V1-C1'",
			Allowed: &allowed,
		}); err != nil {
			t.Fatalf("emit tool event: %v", err)
		}
	}
	run.Finish("", nil)

	runs, err := List(root, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(runs) != 1 || runs[0].StuckEvents == 0 {
		t.Fatalf("repeated command was not marked stuck: %#v", runs)
	}
	if !strings.Contains(runs[0].Stuck[0].Reason, "repeated_command:") {
		t.Fatalf("stuck reasons = %#v", runs[0].Stuck)
	}
}
