package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/store"
)

func TestGoalToolAllowlistSeparatesReadAndWriteCapabilities(t *testing.T) {
	readOnly := strings.Join(goalToolAllowlist(false), "\n")
	if strings.Contains(readOnly, "tool patch") || strings.Contains(readOnly, "tool refresh") {
		t.Fatalf("read-only allowlist contains write tools:\n%s", readOnly)
	}
	if !strings.Contains(readOnly, "novelgen tool skill") || !strings.Contains(readOnly, "novelgen tool query") {
		t.Fatalf("read-only allowlist is missing discovery tools:\n%s", readOnly)
	}

	writable := strings.Join(goalToolAllowlist(true), "\n")
	for _, want := range []string{"novelgen tool patch chapter", "novelgen tool patch chapter --apply --refresh-derived", "novelgen tool patch-buffer"} {
		if !strings.Contains(writable, want) {
			t.Errorf("writable allowlist missing %q:\n%s", want, writable)
		}
	}
}

func TestDecodeGoalOutputAcceptsFencedJSON(t *testing.T) {
	var output agentGoalOutput
	err := decodeGoalOutput("Here is the result:\n```json\n{\"status\":\"completed\",\"summary\":\"done\"}\n```", &output)
	if err != nil {
		t.Fatalf("decodeGoalOutput() error = %v", err)
	}
	if output.Status != "completed" || output.Summary != "done" {
		t.Fatalf("decoded output = %#v", output)
	}
}

func TestValidateGoalReportRejectsUnverifiedCompletion(t *testing.T) {
	report := &AgentGoalReport{
		Status:         "completed",
		Summary:        "done",
		ChangedTargets: []string{"chapter:P1-V1-C1"},
		AllowWrite:     true,
		ToolSummary:    &agentruntime.LiveSummary{ToolAllowed: 1},
	}
	if err := validateGoalReport(report, map[string]bool{"goal-agent-core": true}); err == nil {
		t.Fatal("expected changed target without patch apply to fail")
	}
	report.ToolSummary.PatchApplies = 1
	if err := validateGoalReport(report, map[string]bool{"goal-agent-core": true}); err == nil {
		t.Fatal("expected patch without follow-up check to fail")
	}
	report.ToolSummary.CheckCalls = 1
	report.SkillsUsed = []string{"goal-agent-core"}
	if err := validateGoalReport(report, map[string]bool{"goal-agent-core": true}); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
}

func TestValidateGoalReportRejectsWriteClaimInReadOnlyMode(t *testing.T) {
	report := &AgentGoalReport{
		Status:         "completed",
		Summary:        "done",
		ChangedTargets: []string{"setup"},
		AllowWrite:     false,
		ToolSummary:    &agentruntime.LiveSummary{ToolAllowed: 1, PatchApplies: 1, CheckCalls: 1},
	}
	if err := validateGoalReport(report, map[string]bool{"goal-agent-core": true}); err == nil {
		t.Fatal("expected read-only report with changed target to fail")
	}
}

type fakeGoalRuntime struct {
	invocations []agentruntime.Invocation
	results     []*agentruntime.Result
	result      *agentruntime.Result
}

func (f *fakeGoalRuntime) Invoke(_ context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	f.invocations = append(f.invocations, invocation)
	if len(f.results) > 0 {
		index := len(f.invocations) - 1
		if index >= len(f.results) {
			index = len(f.results) - 1
		}
		return f.results[index], nil
	}
	return f.result, nil
}

func TestRunAgentGoalValidatesAndRecordsUsage(t *testing.T) {
	root := goalTestRoot(t)
	runtime := &fakeGoalRuntime{result: &agentruntime.Result{
		Content:   `{"status":"completed","summary":"checked outline","skills_used":["outline-review-workflow"]}`,
		Model:     "test-model",
		SessionID: "session-123",
		Plan:      []agentruntime.PlanItem{{ID: "inspect", Text: "Inspect outline", Status: "completed"}},
		Usage:     agentruntime.Usage{TotalTokens: 42},
		LiveSummary: &agentruntime.LiveSummary{
			ToolAllowed: 2,
			QueryCalls:  1,
			CheckCalls:  1,
		},
	}}
	report, err := RunAgentGoal(context.Background(), AgentGoalOptions{
		Goal:          "review the outline",
		Root:          root,
		AllowedSkills: []string{"outline-review-workflow"},
		Runtime:       runtime,
	})
	if err != nil {
		t.Fatalf("RunAgentGoal() error = %v", err)
	}
	if report.Status != "completed" || report.Summary != "checked outline" {
		t.Fatalf("report = %#v", report)
	}
	if report.SessionID != "session-123" {
		t.Fatalf("SessionID = %q, want session-123", report.SessionID)
	}
	if len(report.Plan) != 1 || report.Plan[0].ID != "inspect" {
		t.Fatalf("Plan = %#v", report.Plan)
	}
	if len(runtime.invocations) != 1 || runtime.invocations[0].Metadata["mode"] != "goal-agent" {
		t.Fatalf("invocations = %#v", runtime.invocations)
	}
	if runtime.invocations[0].NativeTools == nil || !runtime.invocations[0].NativeTools.Enabled {
		t.Fatalf("native tools were not enabled: %#v", runtime.invocations[0].NativeTools)
	}
	if !strings.Contains(runtime.invocations[0].SystemPrompt, "outline-review-workflow") {
		t.Fatalf("system prompt missing selected skill catalog:\n%s", runtime.invocations[0].SystemPrompt)
	}
	if strings.Contains(runtime.invocations[0].SystemPrompt, "write-chapter-workflow") {
		t.Fatalf("system prompt leaked a skill outside --skill:\n%s", runtime.invocations[0].SystemPrompt)
	}
	entries, err := root.ReadUsage()
	if err != nil {
		t.Fatalf("ReadUsage() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Agent != "GoalAgent" || entries[0].TotalTokens != 42 {
		t.Fatalf("usage entries = %#v", entries)
	}
}

func TestRunAgentGoalRetriesValidationFailureReadOnly(t *testing.T) {
	root := goalTestRoot(t)
	runtime := &fakeGoalRuntime{results: []*agentruntime.Result{
		{
			Content: `{"status":"completed","summary":"changed chapter","changed_targets":["chapter:P1-V1-C1"]}`,
			LiveSummary: &agentruntime.LiveSummary{
				ToolAllowed: 1,
			},
		},
		{
			Content: `{"status":"completed","summary":"verified existing project state","skills_used":["write-chapter-workflow"]}`,
			LiveSummary: &agentruntime.LiveSummary{
				ToolAllowed: 1,
				CheckCalls:  1,
			},
		},
	}}
	report, err := RunAgentGoal(context.Background(), AgentGoalOptions{
		Goal:          "repair the chapter",
		Root:          root,
		AllowedSkills: []string{"write-chapter-workflow"},
		AllowWrite:    true,
		MaxAttempts:   2,
		Runtime:       runtime,
	})
	if err != nil {
		t.Fatalf("RunAgentGoal() error = %v", err)
	}
	if len(runtime.invocations) != 2 {
		t.Fatalf("invocations = %d, want 2", len(runtime.invocations))
	}
	if strings.Contains(strings.Join(runtime.invocations[1].ToolAllowlist, "\n"), "tool patch") {
		t.Fatalf("retry allowlist should be read-only: %#v", runtime.invocations[1].ToolAllowlist)
	}
	if runtime.invocations[1].NativeTools == nil || runtime.invocations[1].NativeTools.AllowWrite {
		t.Fatalf("retry native tools should be read-only: %#v", runtime.invocations[1].NativeTools)
	}
	if !strings.Contains(runtime.invocations[1].UserPrompt, "This retry is read-only") {
		t.Fatalf("retry prompt missing read-only correction instruction:\n%s", runtime.invocations[1].UserPrompt)
	}
	if report.Status != "completed" || report.Summary != "verified existing project state" {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunAgentGoalPassesResumeSessionOnFirstAttempt(t *testing.T) {
	root := goalTestRoot(t)
	runtime := &fakeGoalRuntime{result: &agentruntime.Result{
		Content:   `{"status":"completed","summary":"continued","skills_used":["goal-agent-core"]}`,
		SessionID: "session-456",
		LiveSummary: &agentruntime.LiveSummary{
			ToolAllowed: 1,
		},
	}}
	report, err := RunAgentGoal(context.Background(), AgentGoalOptions{
		Goal:          "continue the previous goal",
		Root:          root,
		ResumeSession: "session-123",
		Runtime:       runtime,
	})
	if err != nil {
		t.Fatalf("RunAgentGoal() error = %v", err)
	}
	if runtime.invocations[0].ResumeSessionID != "session-123" {
		t.Fatalf("ResumeSessionID = %q", runtime.invocations[0].ResumeSessionID)
	}
	if report.SessionID != "session-456" {
		t.Fatalf("report SessionID = %q", report.SessionID)
	}
}

func TestRunAgentGoalRestoresAndPersistsPlanAcrossResume(t *testing.T) {
	root := goalTestRoot(t)
	initialPlan := []agentruntime.PlanItem{{ID: "old", Text: "Old step", Status: "completed"}}
	if err := root.WriteJSON(root.AgentGoalPlanFile("session-old"), initialPlan); err != nil {
		t.Fatal(err)
	}
	runtime := &fakeGoalRuntime{result: &agentruntime.Result{
		Content:     `{"status":"completed","summary":"continued","skills_used":["goal-agent-core"]}`,
		SessionID:   "session-new",
		Plan:        []agentruntime.PlanItem{{ID: "next", Text: "Next step", Status: "in_progress"}},
		PlanUpdated: true,
		LiveSummary: &agentruntime.LiveSummary{
			ToolAllowed: 1,
		},
	}}
	report, err := RunAgentGoal(context.Background(), AgentGoalOptions{
		Goal:          "continue",
		Root:          root,
		ResumeSession: "session-old",
		Runtime:       runtime,
	})
	if err != nil {
		t.Fatalf("RunAgentGoal() error = %v", err)
	}
	if len(runtime.invocations[0].InitialPlan) != 1 || runtime.invocations[0].InitialPlan[0].ID != "old" {
		t.Fatalf("InitialPlan = %#v", runtime.invocations[0].InitialPlan)
	}
	var persisted []agentruntime.PlanItem
	if err := root.ReadJSON(root.AgentGoalPlanFile(report.SessionID), &persisted); err != nil {
		t.Fatalf("read persisted plan: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != "next" {
		t.Fatalf("persisted plan = %#v", persisted)
	}
}

func goalTestRoot(t *testing.T) *store.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "novel.json"), []byte(`{"name":"Goal Test","language":"zh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := store.OpenProject(dir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	return root
}
