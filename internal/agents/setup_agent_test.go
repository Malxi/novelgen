package agents

import (
	"context"
	"strings"
	"testing"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/models"
)

type fakeSetupRuntime struct {
	content    string
	summary    *agentruntime.LiveSummary
	invocation agentruntime.Invocation
	calls      int
}

func (r *fakeSetupRuntime) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.calls++
	r.invocation = invocation
	summary := r.summary
	if summary == nil {
		summary = &agentruntime.LiveSummary{QueryCalls: 1, CheckCalls: 1}
	}
	return &agentruntime.Result{
		Content:     r.content,
		Usage:       agentruntime.Usage{TotalTokens: 11},
		LiveSummary: summary,
	}, nil
}

func TestSetupAgentSDKParamsAllowApplyOnlyWhenRequested(t *testing.T) {
	dryRun := setupAgentSDKParams(false)
	if !dryRun.RequireSDK {
		t.Fatalf("RequireSDK = false, want true")
	}
	if got := dryRun.SDKSkills; len(got) != 2 || got[0] != "novel-tools" || got[1] != "setup-improve-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if got := dryRun.ToolAllowlist; !hasExactString(got, "novelgen tool query story-setup --view brief") ||
		!hasExactString(got, "novelgen tool query story-setup --type search") ||
		!hasExactString(got, "novelgen tool query story-setup --type core-cast") ||
		!hasExactString(got, "novelgen tool query story-setup --type storyline") ||
		!hasExactString(got, "novelgen tool query story-setup --type premise") ||
		!hasExactString(got, "novelgen tool query story-setup --type resource") ||
		!hasExactString(got, "novelgen tool query story-setup --type timeline") ||
		!hasExactString(got, "novelgen tool query logs --type prompts --view index") ||
		!hasExactString(got, "novelgen tool query logs --id") ||
		!hasExactString(got, "novelgen tool check all --target setup --min-priority medium --max-issues 12") ||
		!hasExactString(got, "novelgen tool check all --target setup --category") ||
		!hasExactString(got, "novelgen tool patch setup") ||
		hasExactString(got, "novelgen tool query") ||
		hasExactString(got, "novelgen tool check") ||
		hasExactString(got, "novelgen tool patch setup --apply") {
		t.Fatalf("dry-run allowlist = %#v", got)
	}

	apply := setupAgentSDKParams(true)
	if got := apply.ToolAllowlist; !hasExactString(got, "novelgen tool query story-setup --view brief") ||
		!hasExactString(got, "novelgen tool query story-setup --type search") ||
		!hasExactString(got, "novelgen tool query story-setup --type core-cast") ||
		!hasExactString(got, "novelgen tool query story-setup --type storyline") ||
		!hasExactString(got, "novelgen tool query story-setup --type premise") ||
		!hasExactString(got, "novelgen tool query story-setup --type resource") ||
		!hasExactString(got, "novelgen tool query story-setup --type timeline") ||
		!hasExactString(got, "novelgen tool query logs --type agent-live --view brief") ||
		!hasExactString(got, "novelgen tool query logs --id") ||
		!hasExactString(got, "novelgen tool check all --target setup --min-priority medium --max-issues 12") ||
		!hasExactString(got, "novelgen tool check all --target setup --category") ||
		!hasExactString(got, "novelgen tool patch setup --apply") ||
		hasExactString(got, "novelgen tool query") ||
		hasExactString(got, "novelgen tool check") ||
		hasExactString(got, "novelgen tool patch setup") {
		t.Fatalf("apply allowlist = %#v", got)
	}
	if apply.MaxTurns != 22 || apply.Timeout != 300 {
		t.Fatalf("MaxTurns/Timeout = %d/%d, want 22/300", apply.MaxTurns, apply.Timeout)
	}
	if apply.ToolEvidence.MinQueryCalls != 1 || apply.ToolEvidence.MinCheckCalls != 1 ||
		apply.ToolEvidence.MinPatchApplyCalls != 1 || !apply.ToolEvidence.RequirePatchApplyFollowupCheck {
		t.Fatalf("ToolEvidence = %#v", apply.ToolEvidence)
	}
}

func TestBuildSetupAgentSDKImprovePromptInput(t *testing.T) {
	review := models.ReviewResult{
		OverallScore: 91,
		Summary:      "needs direct repair",
		Suggestions: []models.ReviewSuggestion{{
			Category:   "setup",
			TargetID:   "theme",
			Issue:      "theme is generic",
			Suggestion: "make theme specific",
			Priority:   models.PriorityLow,
		}},
	}

	dryRun := BuildSetupAgentSDKImprovePromptInput(review, "tighten the setup", false, true)
	if dryRun.ApplyPatches {
		t.Fatalf("ApplyPatches = true, want false")
	}
	if !dryRun.ForceIssueRepair || dryRun.UserPrompt != "tighten the setup" {
		t.Fatalf("prompt flags = force %v prompt %q", dryRun.ForceIssueRepair, dryRun.UserPrompt)
	}
	if !hasExactString(dryRun.RequiredQueries, "novelgen tool query story-setup --view brief") {
		t.Fatalf("missing story-setup query: %#v", dryRun.RequiredQueries)
	}
	if !containsSubstring(dryRun.Instructions, "Do not use --apply") {
		t.Fatalf("dry-run instructions missing no-apply guard: %#v", dryRun.Instructions)
	}
	if !containsSubstring(dryRun.Instructions, "force_issue_repair=true") {
		t.Fatalf("instructions missing force repair guidance: %#v", dryRun.Instructions)
	}

	apply := BuildSetupAgentSDKImprovePromptInput(review, "", true, false)
	if !apply.ApplyPatches {
		t.Fatalf("ApplyPatches = false, want true")
	}
	if !containsSubstring(apply.Instructions, "novelgen tool patch setup ... --apply") {
		t.Fatalf("apply instructions missing patch apply guard: %#v", apply.Instructions)
	}
	if !containsSubstring(apply.Instructions, "novelgen tool check all --target setup") {
		t.Fatalf("apply instructions missing setup check: %#v", apply.Instructions)
	}
	if !containsSubstring(apply.Instructions, "printf '%s' '<compact-json>' | novelgen tool patch setup") ||
		!containsSubstring(apply.Instructions, "do not run Python/Node/PowerShell/help commands") {
		t.Fatalf("apply instructions should prefer stdin-piped Chinese JSON patches: %#v", apply.Instructions)
	}
}

func TestSetupAgentImproveWithAgentSDKParsesPatchAndInvocation(t *testing.T) {
	runtime := &fakeSetupRuntime{content: `{
		"review_result": {
			"overall_score": 95,
			"summary": "setup improved",
			"suggestions": []
		},
		"setup_patch": {
			"theme": "specific theme"
		}
	}`, summary: &agentruntime.LiveSummary{
		QueryCalls:   1,
		CheckCalls:   1,
		PatchApplies: 1,
	}}
	agent := &SetupAgent{base: &BaseAgent{
		name:    "SetupAgent",
		runtime: runtime,
		config:  &llm.Config{},
	}}

	output, err := agent.ImproveWithAgentSDK(context.Background(), SetupAgentSDKImproveInput{ApplyPatches: true})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if got := runtime.invocation.SDKSkills; len(got) != 2 || got[1] != "setup-improve-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false, want true")
	}
	if !hasExactString(runtime.invocation.ToolAllowlist, "novelgen tool query story-setup --view brief") ||
		!hasExactString(runtime.invocation.ToolAllowlist, "novelgen tool query story-setup --type search") ||
		!hasExactString(runtime.invocation.ToolAllowlist, "novelgen tool query story-setup --type core-cast") ||
		!hasExactString(runtime.invocation.ToolAllowlist, "novelgen tool query logs --type prompts --view index") ||
		!hasExactString(runtime.invocation.ToolAllowlist, "novelgen tool check all --target setup --min-priority medium --max-issues 12") ||
		!hasExactString(runtime.invocation.ToolAllowlist, "novelgen tool check all --target setup --category") ||
		!hasExactString(runtime.invocation.ToolAllowlist, "novelgen tool patch setup --apply") {
		t.Fatalf("allowlist = %#v", runtime.invocation.ToolAllowlist)
	}
	if output.ReviewResult.OverallScore != 95 {
		t.Fatalf("review score = %.1f, want 95", output.ReviewResult.OverallScore)
	}
	if output.SetupPatch["theme"] != "specific theme" {
		t.Fatalf("setup patch = %#v", output.SetupPatch)
	}
}

func TestSetupAgentImproveWithAgentSDKRejectsMissingCheckEvidence(t *testing.T) {
	runtime := &fakeSetupRuntime{
		content: `{"review_result":{"overall_score":95,"summary":"ok"},"setup_patch":{}}`,
		summary: &agentruntime.LiveSummary{
			QueryCalls: 1,
			CheckCalls: 0,
		},
	}
	agent := &SetupAgent{base: &BaseAgent{
		name:    "SetupAgent",
		runtime: runtime,
		config:  &llm.Config{},
	}}

	_, err := agent.ImproveWithAgentSDK(context.Background(), SetupAgentSDKImproveInput{ApplyPatches: true})
	if err == nil || !strings.Contains(err.Error(), "check calls=0") {
		t.Fatalf("expected missing check evidence error, got %v", err)
	}
}

func TestSetupAgentImproveWithAgentSDKRejectsMissingPatchApplyEvidence(t *testing.T) {
	runtime := &fakeSetupRuntime{
		content: `{"review_result":{"overall_score":95,"summary":"ok"},"setup_patch":{"theme":"specific"}}`,
		summary: &agentruntime.LiveSummary{
			QueryCalls:   1,
			CheckCalls:   1,
			PatchApplies: 0,
		},
	}
	agent := &SetupAgent{base: &BaseAgent{
		name:    "SetupAgent",
		runtime: runtime,
		config:  &llm.Config{},
	}}

	_, err := agent.ImproveWithAgentSDK(context.Background(), SetupAgentSDKImproveInput{ApplyPatches: true})
	if err == nil || !strings.Contains(err.Error(), "patch apply calls=0") {
		t.Fatalf("expected missing patch apply evidence error, got %v", err)
	}
}

func hasExactString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsSubstring(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
