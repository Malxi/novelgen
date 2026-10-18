package setup

import (
	"context"
	agentbase "novelgen/internal/fiction/agents/base"
	"strings"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/creativity"
	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/utils"
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
		!hasExactString(got, "novelgen tool query ideas --type axiom") ||
		!hasExactString(got, "novelgen tool query ideas --type event") ||
		!hasExactString(got, "novelgen tool query ideas --type law") ||
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
		!hasExactString(got, "novelgen tool query ideas --type axiom") ||
		!hasExactString(got, "novelgen tool query ideas --type event") ||
		!hasExactString(got, "novelgen tool query ideas --type law") ||
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
	if apply.MaxTurns != 22 || apply.Timeout != 900 {
		t.Fatalf("MaxTurns/Timeout = %d/%d, want 22/900", apply.MaxTurns, apply.Timeout)
	}
	if apply.ToolEvidence.MinQueryCalls != 1 || apply.ToolEvidence.MinCheckCalls != 1 ||
		apply.ToolEvidence.MinPatchApplyCalls != 1 || !apply.ToolEvidence.RequirePatchApplyFollowupCheck {
		t.Fatalf("ToolEvidence = %#v", apply.ToolEvidence)
	}
}

func TestSetupGenInputSerializesCreativeContract(t *testing.T) {
	input := SetupGenInput{
		Idea: "一个关于宗门继承的故事",
		CreativeProvocation: &creativity.Provocation{
			ID:                    "axiom-42-01",
			Sentence:              "每年冬至，宗主的辞职信由一只乌鸦代写。",
			RequiredRole:          "必须成为继承制度",
			MustAnswer:            []string{"谁执行这条规则"},
			ForbiddenExplanations: []string{"梦"},
			OutputContract:        []string{"给出三条 canon 事实"},
		},
	}
	text := utils.StructToMarkdown(input, 0)
	for _, want := range []string{"Creative Provocation", "每年冬至", "谁执行这条规则", "给出三条 canon 事实"} {
		if !strings.Contains(text, want) {
			t.Fatalf("serialized setup input missing %q:\n%s", want, text)
		}
	}
}

func TestSetupGenInputSerializesCreativeScenario(t *testing.T) {
	input := SetupGenInput{
		Idea: "一个星际大逃杀故事",
		CreativeScenario: &creativity.ScenarioCard{
			ID:            "scenario-12-01",
			Arena:         "虫洞边缘的废弃航道",
			Scarcity:      "反应堆冷却剂",
			Hazard:        "引力剪切",
			PremiseKernel: "冷却剂是唯一硬通货",
			ElementRoles: map[string]string{
				"虫洞": "arena",
				"虫族": "threat",
			},
		},
	}
	text := utils.StructToMarkdown(input, 0)
	for _, want := range []string{"Creative Scenario", "虫洞边缘", "反应堆冷却剂", "引力剪切"} {
		if !strings.Contains(text, want) {
			t.Fatalf("serialized setup input missing %q:\n%s", want, text)
		}
	}
}

func TestSetupCausalReviewRejected(t *testing.T) {
	if (SetupCausalReviewOutput{AxiomCount: 1, Verdict: "keep"}).Rejected() {
		t.Fatal("one-axiom clean review should not be rejected")
	}
	cases := []SetupCausalReviewOutput{
		{AxiomCount: 2},
		{Verdict: "reject"},
		{OrphanRules: []string{"真话税无法从长耳朵推导"}},
		{MissingBeneficiary: []string{"钟楼没有预算"}},
		{ZeroCostBypasses: []string{"市民可以不去钟楼"}},
		{UnseededTwists: []string{"建城契约伪造没有线索"}},
	}
	for _, review := range cases {
		if !review.Rejected() {
			t.Fatalf("review should be rejected: %#v", review)
		}
	}
	issues := (SetupCausalReviewOutput{OrphanRules: []string{"a"}, UnseededTwists: []string{"b"}}).Issues()
	if len(issues) != 2 || !strings.Contains(issues[0], "orphan_rule") || !strings.Contains(issues[1], "unseeded_twist") {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestSetupAgentGenerateWithAgentSDKUsesGenerateWorkflow(t *testing.T) {
	runtime := &fakeSetupRuntime{content: `{
		"project_name": "测试书",
		"genres": ["仙侠"],
		"premise": "身份必须用继承权支付。",
		"theme": "身份与债务",
		"rules": ["身份到期必须续费"],
		"target_audience": "成年读者",
		"tone": "冷峻，克制",
		"tense": "过去时",
		"pov_style": "第三人称有限视角"
	}`}
	agent := &SetupAgent{base: agentbase.NewBaseAgent(agentbase.BaseAgentConfig{Name: "SetupAgent", Runtime: runtime, Config: &llm.Config{}})}

	output, err := agent.GenerateWithAgentSDK(context.Background(), SetupGenInput{Idea: "一个关于身份的故事"})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatal("RequireSDK = false, want true")
	}
	if got := runtime.invocation.SDKSkills; len(got) != 1 || got[0] != "setup-generate-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if output.Setup.ProjectName != "测试书" {
		t.Fatalf("project name = %q", output.Setup.ProjectName)
	}
}

func TestSetupAgentReviewCausalityWithAgentSDKUsesCausalWorkflow(t *testing.T) {
	runtime := &fakeSetupRuntime{content: `{
		"axiom_count": 2,
		"orphan_rules": ["真话税无法从长耳朵推导"],
		"missing_beneficiary": [],
		"zero_cost_bypasses": [],
		"unseeded_twists": [],
		"verdict": "reject",
		"summary": "存在第二条魔法规则"
	}`}
	agent := &SetupAgent{base: agentbase.NewBaseAgent(agentbase.BaseAgentConfig{Name: "SetupAgent", Runtime: runtime, Config: &llm.Config{}})}

	output, err := agent.ReviewCausalityWithAgentSDK(context.Background(), SetupCausalReviewInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got := runtime.invocation.SDKSkills; len(got) != 1 || got[0] != "setup-causal-review" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if !output.Rejected() {
		t.Fatal("expected causal review to be rejected")
	}
}

func TestSetupAgentReviewWithAgentSDKUsesReviewWorkflow(t *testing.T) {
	runtime := &fakeSetupRuntime{content: `{
		"overall_score": 88,
		"summary": "设定可写",
		"strengths": ["根规则清晰"],
		"weaknesses": [],
		"suggestions": []
	}`}
	agent := &SetupAgent{base: agentbase.NewBaseAgent(agentbase.BaseAgentConfig{Name: "SetupAgent", Runtime: runtime, Config: &llm.Config{}})}

	output, err := agent.ReviewWithAgentSDK(context.Background(), SetupReviewInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got := runtime.invocation.SDKSkills; len(got) != 1 || got[0] != "setup-review-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if output.Result.OverallScore != 88 {
		t.Fatalf("overall score = %.1f, want 88", output.Result.OverallScore)
	}
}

func TestSetupAgentPredictScenarioParsesScores(t *testing.T) {
	runtime := &fakeSetupRuntime{content: `{
		"predictability": 72,
		"novelty": 28,
		"obvious_developments": ["抢补给", "关闭反应堆"],
		"summary": "资源垄断链条过于直接"
	}`}
	agent := &SetupAgent{base: agentbase.NewBaseAgent(agentbase.BaseAgentConfig{Name: "SetupAgent", Runtime: runtime, Config: &llm.Config{}})}

	output, err := agent.PredictScenario(context.Background(), ScenarioPredictionInput{
		Idea:     "星际大逃杀",
		Scenario: creativity.ScenarioCard{ID: "scenario-12-01"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if output.Predictability != 72 || output.NormalizedNovelty() != 28 {
		t.Fatalf("prediction = %#v", output)
	}
	if len(output.ObviousDevelopments) != 2 {
		t.Fatalf("developments = %#v", output.ObviousDevelopments)
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
	agent := &SetupAgent{base: agentbase.NewBaseAgent(agentbase.BaseAgentConfig{Name: "SetupAgent", Runtime: runtime, Config: &llm.Config{}})}

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
	agent := &SetupAgent{base: agentbase.NewBaseAgent(agentbase.BaseAgentConfig{Name: "SetupAgent", Runtime: runtime, Config: &llm.Config{}})}

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
	agent := &SetupAgent{base: agentbase.NewBaseAgent(agentbase.BaseAgentConfig{Name: "SetupAgent", Runtime: runtime, Config: &llm.Config{}})}

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
