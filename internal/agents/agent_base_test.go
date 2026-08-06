package agents

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
)

type fakeRepairClient struct {
	calls int
}

func (c *fakeRepairClient) ChatCompletion(ctx context.Context, messages []llm.Message, options *llm.ChatOptions) (*llm.ChatResponse, error) {
	c.calls++
	if c.calls == 1 {
		return &llm.ChatResponse{Content: `{"name":"broken",}`, Usage: llm.Usage{TotalTokens: 10}}, nil
	}
	return &llm.ChatResponse{Content: `{"name":"fixed"}`, Usage: llm.Usage{TotalTokens: 10}}, nil
}

func TestBaseAgentExecuteRepairsMalformedJSON(t *testing.T) {
	client := &fakeRepairClient{}
	agent := &BaseAgent{
		name:   "TestAgent",
		client: client,
		config: &llm.Config{},
	}

	var output struct {
		Name string `json:"name"`
	}
	err := agent.Execute(context.Background(), InvokeParams{Command: "return a test object"}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if output.Name != "fixed" {
		t.Fatalf("Name = %q, want fixed", output.Name)
	}
	if client.calls != 2 {
		t.Fatalf("calls = %d, want 2", client.calls)
	}
}

func TestClipResponsePreviewIsRuneSafe(t *testing.T) {
	text := "模式检查误报：每章events均含完整叙事性结果"
	got := clipResponsePreview(text, 5)
	if !utf8.ValidString(got) {
		t.Fatalf("preview is not valid UTF-8: %q", got)
	}
	if got != "模式检查误..." {
		t.Fatalf("preview = %q", got)
	}
}

func TestValidateAgentRuntimeToolEvidenceRequiresSpecificCommand(t *testing.T) {
	result := &agentruntime.Result{LiveSummary: &agentruntime.LiveSummary{
		ContextQueryCalls: 1,
		CheckCalls:        1,
		AllowedToolCommands: []string{
			`novelgen tool query context --type outline-volume --id P1-V1 --view index`,
			`novelgen tool check all --target outline --scope chapter --id "P1-V1-C2" --category logic --min-priority low --max-issues 8`,
		},
	}}

	err := validateAgentRuntimeToolEvidence("TestAgent", ToolEvidenceRequirement{
		MinContextQueryCalls: 1,
		MinCheckCalls:        1,
		RequiredToolCommands: []string{
			`novelgen tool check all --target outline --scope chapter --id "P1-V1-C2" --category logic`,
		},
	}, result)
	if err != nil {
		t.Fatalf("required command should pass: %v", err)
	}

	err = validateAgentRuntimeToolEvidence("TestAgent", ToolEvidenceRequirement{
		RequiredToolCommands: []string{
			`novelgen tool check all --target outline --scope chapter --id "P1-V1-C2" --category transition`,
		},
	}, result)
	if err == nil || !strings.Contains(err.Error(), "required tool command not observed") {
		t.Fatalf("expected missing required command error, got %v", err)
	}
}

func TestDeterministicJSONRepairEscapesUnescapedInnerQuotes(t *testing.T) {
	var output struct {
		Summary string `json:"summary"`
		Count   int    `json:"count"`
	}
	raw := `{"summary":"完成了"荒骨苏醒"和"青藤据点"两条故事线","count":"2"}`

	repaired, err := tryParseDeterministicallyRepairedJSON(raw, &output)
	if err != nil {
		t.Fatalf("tryParseDeterministicallyRepairedJSON() error = %v", err)
	}
	if output.Summary != `完成了"荒骨苏醒"和"青藤据点"两条故事线` {
		t.Fatalf("Summary = %q", output.Summary)
	}
	if output.Count != 2 {
		t.Fatalf("Count = %d, want 2", output.Count)
	}
	if repaired == "" {
		t.Fatalf("repair summary is empty")
	}
}

type fakeRepairClientWithNumericStrings struct {
	calls int
}

func (c *fakeRepairClientWithNumericStrings) ChatCompletion(ctx context.Context, messages []llm.Message, options *llm.ChatOptions) (*llm.ChatResponse, error) {
	c.calls++
	if c.calls == 1 {
		return &llm.ChatResponse{Content: `{"enemies":[{"name":"broken","count":,}]}`, Usage: llm.Usage{TotalTokens: 10}}, nil
	}
	return &llm.ChatResponse{Content: `{
		"enemies": [
			{"name": "魔门刺客", "count": "3", "level": "7"}
		]
	}`, Usage: llm.Usage{TotalTokens: 10}}, nil
}

func TestBaseAgentExecuteDeterministicallyRepairsJSONRepairResponse(t *testing.T) {
	client := &fakeRepairClientWithNumericStrings{}
	agent := &BaseAgent{
		name:   "TestAgent",
		client: client,
		config: &llm.Config{},
	}

	var output struct {
		Enemies []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
			Level int    `json:"level"`
		} `json:"enemies"`
	}
	err := agent.Execute(context.Background(), InvokeParams{Command: "return a test object"}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if len(output.Enemies) != 1 {
		t.Fatalf("len(Enemies) = %d, want 1", len(output.Enemies))
	}
	if output.Enemies[0].Count != 3 || output.Enemies[0].Level != 7 {
		t.Fatalf("enemy numeric fields = count %d level %d, want count 3 level 7", output.Enemies[0].Count, output.Enemies[0].Level)
	}
	if client.calls != 2 {
		t.Fatalf("calls = %d, want 2", client.calls)
	}
}

type fakeStaticClient struct {
	content string
	calls   int
}

func (c *fakeStaticClient) ChatCompletion(ctx context.Context, messages []llm.Message, options *llm.ChatOptions) (*llm.ChatResponse, error) {
	c.calls++
	return &llm.ChatResponse{Content: c.content, Usage: llm.Usage{TotalTokens: 10}}, nil
}

func TestBaseAgentExecuteDeterministicallyRepairsNumericStrings(t *testing.T) {
	client := &fakeStaticClient{
		content: `{
			"enemies": [
				{"name": "魔门刺客", "count": "3", "level": "7"}
			]
		}`,
	}
	agent := &BaseAgent{
		name:   "TestAgent",
		client: client,
		config: &llm.Config{},
	}

	var output struct {
		Enemies []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
			Level int    `json:"level"`
		} `json:"enemies"`
	}
	err := agent.Execute(context.Background(), InvokeParams{Command: "return a test object"}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if len(output.Enemies) != 1 {
		t.Fatalf("len(Enemies) = %d, want 1", len(output.Enemies))
	}
	if output.Enemies[0].Count != 3 || output.Enemies[0].Level != 7 {
		t.Fatalf("enemy numeric fields = count %d level %d, want count 3 level 7", output.Enemies[0].Count, output.Enemies[0].Level)
	}
	if client.calls != 1 {
		t.Fatalf("calls = %d, want 1", client.calls)
	}
}

func TestBaseAgentExecuteDeterministicallyRepairsResourceLedgerNumbers(t *testing.T) {
	client := &fakeStaticClient{
		content: `{
			"resource_ledger": [
				{"item": "记忆锚残片", "start": 0.7, "delta": -0.3, "end": 0.7, "reason": "模型误写成比例"}
			]
		}`,
	}
	agent := &BaseAgent{
		name:   "TestAgent",
		client: client,
		config: &llm.Config{},
	}

	var output struct {
		ResourceLedger []struct {
			Item   string `json:"item"`
			Start  int    `json:"start"`
			Delta  int    `json:"delta"`
			End    int    `json:"end"`
			Reason string `json:"reason"`
		} `json:"resource_ledger"`
	}
	err := agent.Execute(context.Background(), InvokeParams{Command: "return a test object"}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if len(output.ResourceLedger) != 1 {
		t.Fatalf("len(ResourceLedger) = %d, want 1", len(output.ResourceLedger))
	}
	entry := output.ResourceLedger[0]
	if entry.Start != 1 || entry.Delta != 0 || entry.End != 1 {
		t.Fatalf("resource ledger = start %d delta %d end %d, want start 1 delta 0 end 1", entry.Start, entry.Delta, entry.End)
	}
	if client.calls != 1 {
		t.Fatalf("calls = %d, want 1", client.calls)
	}
}

type fakeRuntime struct {
	invocation agentruntime.Invocation
	summary    *agentruntime.LiveSummary
	calls      int
}

func (r *fakeRuntime) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.calls++
	r.invocation = invocation
	return &agentruntime.Result{
		Content:     `{"name":"runtime"}`,
		Usage:       agentruntime.Usage{TotalTokens: 7},
		LiveSummary: r.summary,
	}, nil
}

func TestBaseAgentExecuteUsesRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	agent := &BaseAgent{
		name:    "TestAgent",
		runtime: runtime,
		config:  &llm.Config{},
	}

	var output struct {
		Name string `json:"name"`
	}
	err := agent.Execute(context.Background(), InvokeParams{Command: "return a test object"}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if output.Name != "runtime" {
		t.Fatalf("Name = %q, want runtime", output.Name)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if runtime.invocation.AgentName != "TestAgent" {
		t.Fatalf("AgentName = %q, want TestAgent", runtime.invocation.AgentName)
	}
	if runtime.invocation.OutputJSONSchema["type"] != "object" {
		t.Fatalf("OutputJSONSchema type = %v, want object", runtime.invocation.OutputJSONSchema["type"])
	}
}

func TestBaseAgentExecutePassesPerCallAgentSDKOptions(t *testing.T) {
	runtime := &fakeRuntime{summary: &agentruntime.LiveSummary{
		QueryCalls:        5,
		ContextQueryCalls: 4,
		CheckCalls:        3,
		PatchApplies:      4,
		AllowedToolCommands: []string{
			"novelgen tool check all --target outline --scope volume --id p1-v1",
		},
	}}
	agent := &BaseAgent{
		name:    "TestAgent",
		runtime: runtime,
		config:  &llm.Config{},
	}

	var output struct {
		Name string `json:"name"`
	}
	err := agent.Execute(context.Background(), InvokeParams{
		Command:        "return a test object",
		SDKSkills:      []string{"novel-tools", "outline-compose-volume-workflow"},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  []string{"novelgen tool query"},
		ToolEvidence: ToolEvidenceRequirement{
			MinQueryCalls:        1,
			MinContextQueryCalls: 2,
			MinCheckCalls:        3,
			MinPatchApplyCalls:   4,
			RequiredToolCommands: []string{"novelgen tool check all --target outline --scope volume --id P1-V1"},
			RequireNoDeniedTools: true,
		},
		MaxTurns:       10,
		Timeout:        900,
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if got := runtime.invocation.SDKSkills; len(got) != 2 || got[1] != "outline-compose-volume-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if got := runtime.invocation.Tools; len(got) != 1 || got[0] != "Bash" {
		t.Fatalf("Tools = %#v", got)
	}
	if got := runtime.invocation.AllowedTools; len(got) != 1 || got[0] != "Bash" {
		t.Fatalf("AllowedTools = %#v", got)
	}
	if runtime.invocation.PermissionMode != "dontAsk" {
		t.Fatalf("PermissionMode = %q", runtime.invocation.PermissionMode)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false, want true")
	}
	if got := runtime.invocation.ToolAllowlist; len(got) != 1 || got[0] != "novelgen tool query" {
		t.Fatalf("ToolAllowlist = %#v", got)
	}
	if got := runtime.invocation.ToolEvidence; got.MinQueryCalls != 1 || got.MinContextQueryCalls != 2 || got.MinCheckCalls != 3 || got.MinPatchApplyCalls != 4 {
		t.Fatalf("ToolEvidence minimums = %#v", got)
	}
	if got := runtime.invocation.ToolEvidence.RequiredToolCommands; len(got) != 1 || got[0] != "novelgen tool check all --target outline --scope volume --id P1-V1" {
		t.Fatalf("ToolEvidence.RequiredToolCommands = %#v", got)
	}
	if !runtime.invocation.ToolEvidence.RequireNoDeniedTools {
		t.Fatalf("ToolEvidence.RequireNoDeniedTools = false, want true")
	}
	if runtime.invocation.Options.MaxTurns != 10 {
		t.Fatalf("MaxTurns = %d, want 10", runtime.invocation.Options.MaxTurns)
	}
	if runtime.invocation.Options.Timeout != 900 {
		t.Fatalf("Timeout = %d, want 900", runtime.invocation.Options.Timeout)
	}
}

func TestBaseAgentExecuteWithRuntimeResultReturnsSummary(t *testing.T) {
	runtime := &fakeRuntime{}
	agent := &BaseAgent{
		name:    "TestAgent",
		runtime: runtime,
		config:  &llm.Config{},
	}

	var output struct {
		Name string `json:"name"`
	}
	result, err := agent.ExecuteWithRuntimeResult(context.Background(), InvokeParams{
		Command: "return a test object",
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("ExecuteWithRuntimeResult() returned error: %v", err)
	}
	if output.Name != "runtime" {
		t.Fatalf("Name = %q, want runtime", output.Name)
	}
	if result == nil || result.Usage.TotalTokens != 7 {
		t.Fatalf("runtime result = %#v", result)
	}
}

func TestBaseAgentExecuteVerifiesRuntimeToolEvidence(t *testing.T) {
	runtime := &fakeRuntime{summary: &agentruntime.LiveSummary{
		QueryCalls:        1,
		ContextQueryCalls: 1,
		CheckCalls:        1,
		PatchApplies:      1,
	}}
	agent := &BaseAgent{
		name:    "TestAgent",
		runtime: runtime,
		config:  &llm.Config{},
	}

	var output struct {
		Name string `json:"name"`
	}
	err := agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls: 1,
			MinQueryCalls:        1,
			MinCheckCalls:        1,
			MinPatchApplyCalls:   1,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	runtime.summary.CheckCalls = 0
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls: 1,
			MinCheckCalls:        1,
		},
	}, struct{}{}, &output)
	if err == nil || !strings.Contains(err.Error(), "check calls=0") {
		t.Fatalf("expected missing check evidence error, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		QueryCalls:                1,
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ApplyWithoutFollowupCheck: 1,
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
		},
	}, struct{}{}, &output)
	if err == nil || !strings.Contains(err.Error(), "without follow-up check=1") {
		t.Fatalf("expected missing follow-up check evidence error, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:  1,
		CheckCalls:         1,
		ToolDenied:         1,
		DeniedToolCommands: []string{"novelgen tool query outline --view full"},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls: 1,
			MinCheckCalls:        1,
			RequireNoDeniedTools: true,
		},
	}, struct{}{}, &output)
	if err == nil || !strings.Contains(err.Error(), "denied tool calls=1") || !strings.Contains(err.Error(), "outline --view full") {
		t.Fatalf("expected denied tool evidence error, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`novelgen tool patch outline --target volume --id "P1-V1" --apply`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("successful apply retry denial should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`novelgen tool patch outline --target volume --id "P1-V1"`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("successful apply follow-up patch-cycle denial should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`novelgen tool patch-buffer clear --id "P1-V1-C1-draft"`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("successful apply after patch-buffer precheck denial should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`type C:\Users\me\.claude\projects\run\tool-results-123.txt`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("denied Claude tool-results read after successful apply should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`powershell -Command "Get-Content 'C:\Users\me\AppData\Local\Temp\claude\project\tasks\abc.output' -Tail 20 -Wait" 2>$null`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("denied Claude temp task output read after successful apply should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`novelgen tool query chapter --id "P1-V1-C1" --content --view full`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("denied post-apply full chapter inspection should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`novelgen tool query chapter --id "P1-V1-C1" --content --view brief`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("denied post-apply brief chapter inspection should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`novelgen tool refresh chapter-dsl --id "P1-V1-C1"`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("denied redundant post-apply refresh should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`novelgen tool query context --type chapter-repair --id "P1-V1-C1" --name "logic" --view brief`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("denied post-apply repair context expansion should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:         1,
		CheckCalls:                1,
		PatchApplies:              1,
		ToolDenied:                1,
		ApplyWithoutFollowupCheck: 0,
		DeniedToolCommands:        []string{`powershell -Command "Get-Content 'story\compose\outline.json'"`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:           1,
			MinCheckCalls:                  1,
			MinPatchApplyCalls:             1,
			RequirePatchApplyFollowupCheck: true,
			RequireNoDeniedTools:           true,
		},
	}, struct{}{}, &output)
	if err == nil || !strings.Contains(err.Error(), "denied tool calls=1") {
		t.Fatalf("expected project file Get-Content denial to remain blocking, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls:          1,
		CheckCalls:                 1,
		ToolDenied:                 1,
		ApplyWithoutFollowupCheck:  0,
		WorkflowDeniedToolCommands: []string{`novelgen tool patch outline --target volume --id "P1-V3"`},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls: 1,
			MinCheckCalls:        1,
			RequireNoDeniedTools: true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("workflow-enforced denial should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		ContextQueryCalls: 1,
		CheckCalls:        1,
		ToolDenied:        1,
		DenialsResolved:   true,
		DeniedToolCommands: []string{
			`novelgen tool query outline --type chapter --id "P1-V3-C1" --fields chapter_payoff --view brief`,
		},
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls: 1,
			MinCheckCalls:        1,
			RequireNoDeniedTools: true,
		},
	}, struct{}{}, &output)
	if err != nil {
		t.Fatalf("denial resolved by a later allowed tool call should be tolerated, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		QueryCalls:        2,
		ContextQueryCalls: 2,
		CheckCalls:        1,
		QueryBriefCalls:   1,
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinContextQueryCalls:    1,
			MinCheckCalls:           1,
			MaxQueryCalls:           1,
			MaxContextQueryCalls:    1,
			DisallowQueryBriefCalls: true,
		},
	}, struct{}{}, &output)
	if err == nil || !strings.Contains(err.Error(), "query calls=2") {
		t.Fatalf("expected query budget evidence error, got %v", err)
	}

	runtime.summary = &agentruntime.LiveSummary{
		QueryCalls:      1,
		CheckCalls:      1,
		QueryBriefCalls: 1,
	}
	err = agent.Execute(context.Background(), InvokeParams{
		Command: "return a test object",
		ToolEvidence: ToolEvidenceRequirement{
			MinCheckCalls:           1,
			DisallowQueryBriefCalls: true,
		},
	}, struct{}{}, &output)
	if err == nil || !strings.Contains(err.Error(), "brief query calls=1") {
		t.Fatalf("expected brief query budget evidence error, got %v", err)
	}
}

func TestAgentRuntimeModelPassesConfiguredModelForRequiredSDK(t *testing.T) {
	if got := agentRuntimeModel(InvokeParams{RequireSDK: true}, "deepseek-v4-flash"); got != "deepseek-v4-flash" {
		t.Fatalf("agentRuntimeModel with required SDK = %q", got)
	}
	if got := agentRuntimeModel(InvokeParams{RequireSDK: true}, "claude-sonnet-4"); got != "claude-sonnet-4" {
		t.Fatalf("agentRuntimeModel with Claude model = %q", got)
	}
	if got := agentRuntimeModel(InvokeParams{}, "gpt-5.2"); got != "gpt-5.2" {
		t.Fatalf("agentRuntimeModel without required SDK = %q", got)
	}
}
