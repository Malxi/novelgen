package agentruntime

import (
	"encoding/json"
	"testing"
)

func TestSummarizeToolEventsCountsQueriesChecksAndApplyFollowup(t *testing.T) {
	events := []ToolEvent{
		{Source: "permission", Command: "novelgen tool query context --type outline-volume --id 'P1-V1' --view brief", Allowed: true},
		{Source: "pre", Command: "novelgen tool query context --type outline-volume --id 'P1-V1' --view brief", Allowed: true},
		{Source: "pre", Command: "novelgen tool check all --target outline --category logic --min-priority medium", Allowed: true},
		{Source: "pre", Command: "novelgen tool patch outline --target volume --id 'P1-V1' --apply", Allowed: true, PatchKey: "volume:P1-V1", PatchApply: true},
		{Source: "post", Command: "novelgen tool check all --target outline --category logic --min-priority medium", DurationMS: 12},
	}
	summary := SummarizeToolEvents(events)
	if summary == nil {
		t.Fatal("summary is nil")
	}
	if summary.QueryCalls != 1 || summary.ContextQueryCalls != 1 || summary.CheckCalls != 1 {
		t.Fatalf("query/context/check counts = %d/%d/%d, want 1/1/1", summary.QueryCalls, summary.ContextQueryCalls, summary.CheckCalls)
	}
	if summary.PatchApplies != 1 {
		t.Fatalf("patch applies = %d, want 1", summary.PatchApplies)
	}
	if summary.ApplyWithoutFollowupCheck != 0 {
		t.Fatalf("apply without followup = %d, want 0", summary.ApplyWithoutFollowupCheck)
	}
	if summary.SlowestToolDurationMS != 12 {
		t.Fatalf("slowest duration = %d, want 12", summary.SlowestToolDurationMS)
	}
}

func TestSummarizeToolEventsRecordsDeniedCommands(t *testing.T) {
	summary := SummarizeToolEvents([]ToolEvent{
		{Source: "pre", Command: "novelgen tool query outline --type volume --id 'P1-V1'", Allowed: false, Reason: "outside allowlist"},
	})
	if summary == nil || summary.ToolDenied != 1 || len(summary.DeniedToolCommands) != 1 {
		t.Fatalf("denied summary = %#v", summary)
	}
}

func TestResultDecodesStructuredToolEvents(t *testing.T) {
	var result Result
	if err := json.Unmarshal([]byte(`{"content":"ok","tool_events":[{"source":"pre","command":"novelgen tool query outline --view brief","allowed":true}]}`), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(result.ToolEvents) != 1 || result.ToolEvents[0].Source != "pre" || !result.ToolEvents[0].Allowed {
		t.Fatalf("tool events = %#v", result.ToolEvents)
	}
}
