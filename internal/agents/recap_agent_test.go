package agents

import (
	"context"
	"strings"
	"testing"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
)

type fakeRecapRuntime struct {
	invocation agentruntime.Invocation
	summary    *agentruntime.LiveSummary
	calls      int
}

func (r *fakeRecapRuntime) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.calls++
	r.invocation = invocation
	summary := r.summary
	if summary == nil {
		summary = &agentruntime.LiveSummary{ContextQueryCalls: 1, CheckCalls: 1, PatchApplies: 1}
	}
	return &agentruntime.Result{
		Content: `{
			"recap": {
				"chapter_id": "P1-V1-C1",
				"title": "醒来",
				"location": "坠舰残骸",
				"time": "同夜",
				"present": ["林砚"],
				"plot_beats": ["林砚在残骸中醒来并确认求救信标失效。"],
				"decisions": ["林砚决定沿着蓝色火光寻找出口。"],
				"reveals": ["残骸外存在异常引力潮。"],
				"unresolved": ["蓝色火光来源未知。"],
				"promises": ["林砚承诺带走黑匣子。"],
				"items": ["黑匣子仍在林砚手中。"],
				"status": ["林砚左臂受伤但意识清醒。"],
				"last_line": "他推开舱门。",
				"cliffhanger": "蓝色火光中出现人影。",
				"next_opening_hint": "他推开舱门后，先承接蓝色火光和人影的压迫感。"
			}
		}`,
		Usage:       agentruntime.Usage{TotalTokens: 42},
		LiveSummary: summary,
	}, nil
}

type fakeRecapRuntimeWithContent struct {
	contents   []string
	invocation agentruntime.Invocation
	summary    *agentruntime.LiveSummary
	calls      int
}

func (r *fakeRecapRuntimeWithContent) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.calls++
	r.invocation = invocation
	idx := r.calls - 1
	if idx >= len(r.contents) {
		idx = len(r.contents) - 1
	}
	summary := r.summary
	if summary == nil {
		summary = &agentruntime.LiveSummary{ContextQueryCalls: 1, CheckCalls: 1, PatchApplies: 1}
	}
	return &agentruntime.Result{
		Content:     r.contents[idx],
		Usage:       agentruntime.Usage{TotalTokens: 42},
		LiveSummary: summary,
	}, nil
}

func TestRecapAgentExtractWithAgentSDKUsesWorkflowSkill(t *testing.T) {
	runtime := &fakeRecapRuntime{}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	got, err := agent.ExtractWithAgentSDK(context.Background(), "P1-V1-C1", "醒来", "他推开舱门，蓝色火光扑面而来。")
	if err != nil {
		t.Fatalf("ExtractWithAgentSDK() error = %v", err)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if got.ChapterID != "P1-V1-C1" || got.Title != "醒来" {
		t.Fatalf("recap identity = %s/%s", got.ChapterID, got.Title)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if got := runtime.invocation.SDKSkills; len(got) != 1 || got[0] != "recap-extract-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if !runtime.invocation.CompactOutputSchema {
		t.Fatalf("CompactOutputSchema = false")
	}
	if !runtime.invocation.DisableSDKOutputFormat {
		t.Fatalf("DisableSDKOutputFormat = false")
	}
	if got := runtime.invocation.Skills; len(got) != 1 || got[0] != "recap-extract" {
		t.Fatalf("Skills = %#v", got)
	}
	if len(runtime.invocation.Tools) != 0 || len(runtime.invocation.AllowedTools) != 0 || len(runtime.invocation.ToolAllowlist) != 0 {
		t.Fatalf("recap workflow should not grant tools: tools=%#v allowed=%#v allowlist=%#v",
			runtime.invocation.Tools, runtime.invocation.AllowedTools, runtime.invocation.ToolAllowlist)
	}
	if runtime.invocation.Options.MaxTurns != 3 || runtime.invocation.Options.Timeout != 300 {
		t.Fatalf("MaxTurns/Timeout = %d/%d, want 3/300", runtime.invocation.Options.MaxTurns, runtime.invocation.Options.Timeout)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "蓝色火光") {
		t.Fatalf("UserPrompt did not include chapter text")
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "next_opening_hint directly continue from last_line") {
		t.Fatalf("UserPrompt missing direct continuation instruction")
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "plot_beats <= 8") ||
		!strings.Contains(runtime.invocation.UserPrompt, "no prose summary or validation checklist") {
		t.Fatalf("UserPrompt missing compact recap instruction")
	}
	if runtime.invocation.OutputJSONSchema["type"] != "object" {
		t.Fatalf("OutputJSONSchema type = %v, want object", runtime.invocation.OutputJSONSchema["type"])
	}
}

func TestRecapAgentSDKMaxTurnsScalesWithChapterLength(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "short", text: strings.Repeat("短", 500), want: 3},
		{name: "medium", text: strings.Repeat("中", 1000), want: 5},
		{name: "long", text: strings.Repeat("长", 1400), want: 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := recapAgentSDKMaxTurns(tc.text); got != tc.want {
				t.Fatalf("recapAgentSDKMaxTurns() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRecapAgentExtractWithAgentSDKApplyAllowsValidatedPatchTools(t *testing.T) {
	runtime := &fakeRecapRuntime{}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	got, err := agent.ExtractWithAgentSDKApply(context.Background(), "P1-V1-C1", "醒来", "他推开舱门，蓝色火光扑面而来。", true)
	if err != nil {
		t.Fatalf("ExtractWithAgentSDKApply() error = %v", err)
	}
	if got.ChapterID != "P1-V1-C1" {
		t.Fatalf("ChapterID = %q", got.ChapterID)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if got := runtime.invocation.SDKSkills; len(got) != 2 || got[0] != "novel-tools-core" || got[1] != "recap-extract-workflow" {
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
	if got := runtime.invocation.ToolAllowlist; !containsAllStrings(got,
		`novelgen tool query context --type recap-repair --id "P1-V1-C1" --view brief`,
		`novelgen tool check quality --target recap --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 8`,
		`novelgen tool patch recap --id "P1-V1-C1" --apply`) {
		t.Fatalf("ToolAllowlist = %#v", got)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "tool query context --type recap-repair") ||
		!strings.Contains(runtime.invocation.UserPrompt, "tool check quality --target recap") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Apply Patches") {
		t.Fatalf("UserPrompt missing required recap apply workflow guidance:\n%s", runtime.invocation.UserPrompt)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "printf '%s' '<compact-json>' | novelgen tool patch recap --id <chapter_id>") ||
		!strings.Contains(runtime.invocation.UserPrompt, "do not run Python/Node/PowerShell/help commands") {
		t.Fatalf("UserPrompt should prefer stdin-piped recap patch JSON:\n%s", runtime.invocation.UserPrompt)
	}
}

func TestRecapAgentApplyRejectsMissingCheckEvidence(t *testing.T) {
	runtime := &fakeRecapRuntime{summary: &agentruntime.LiveSummary{
		ContextQueryCalls: 1,
		CheckCalls:        0,
	}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	_, err := agent.ExtractWithAgentSDKApply(context.Background(), "P1-V1-C1", "醒来", "他推开舱门，蓝色火光扑面而来。", true)
	if err == nil || !strings.Contains(err.Error(), "check calls=0") {
		t.Fatalf("expected missing check evidence error, got %v", err)
	}
}

func TestRecapAgentApplyRejectsMissingPatchApplyEvidence(t *testing.T) {
	runtime := &fakeRecapRuntime{summary: &agentruntime.LiveSummary{
		ContextQueryCalls: 1,
		CheckCalls:        1,
		PatchApplies:      0,
	}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	_, err := agent.ExtractWithAgentSDKApply(context.Background(), "P1-V1-C1", "醒来", "他推开舱门，蓝色火光扑面而来。", true)
	if err == nil || !strings.Contains(err.Error(), "patch apply calls=0") {
		t.Fatalf("expected missing patch apply evidence error, got %v", err)
	}
}

func TestRecapAgentApplyDoesNotRetryConsistencyWarning(t *testing.T) {
	runtime := &fakeRecapRuntimeWithContent{contents: []string{
		`{"recap":{"chapter_id":"P1-V1-C1","title":"醒来","location":"残骸","time":"同夜","present":["林砚"],"plot_beats":["林砚醒来。"],"decisions":[],"reveals":[],"unresolved":[],"promises":[],"items":[],"status":[],"last_line":"他推开舱门。","cliffhanger":"","next_opening_hint":"远处传来新的警报声。"}}`,
	}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	got, err := agent.ExtractWithAgentSDKApply(context.Background(), "P1-V1-C1", "醒来", "正文", true)
	if err != nil {
		t.Fatalf("ExtractWithAgentSDKApply() error = %v", err)
	}
	if got.Location != "残骸" {
		t.Fatalf("Location = %q", got.Location)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
}

func TestRecapAgentRejectsMinimalInvalidAfterRetry(t *testing.T) {
	runtime := &fakeRecapRuntimeWithContent{contents: []string{
		`{"recap":{"chapter_id":"P1-V1-C1","title":"醒来","location":"","time":"","present":[],"plot_beats":[],"decisions":[],"reveals":[],"unresolved":[],"promises":[],"items":[],"status":[],"last_line":"","cliffhanger":"","next_opening_hint":""}}`,
		`{"recap":{"chapter_id":"P1-V1-C1","title":"醒来","location":"","time":"","present":[],"plot_beats":[],"decisions":[],"reveals":[],"unresolved":[],"promises":[],"items":[],"status":[],"last_line":"","cliffhanger":"","next_opening_hint":""}}`,
	}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	_, err := agent.ExtractWithAgentSDK(context.Background(), "P1-V1-C1", "醒来", "正文")
	if err == nil {
		t.Fatalf("expected minimal validation failure")
	}
	if !strings.Contains(err.Error(), "recap failed minimal validation") || !strings.Contains(err.Error(), "location 为空") {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime.calls != 2 {
		t.Fatalf("runtime calls = %d, want retry count 2", runtime.calls)
	}
}

func TestRecapAgentNormalizesIdentityFromTargetChapter(t *testing.T) {
	runtime := &fakeRecapRuntimeWithContent{contents: []string{
		`{"recap":{"chapter_id":"WRONG-ID","title":"错误标题","location":"残骸","time":"同夜","present":["林砚"],"plot_beats":["林砚醒来。"],"decisions":[],"reveals":[],"unresolved":[],"promises":[],"items":[],"status":[],"last_line":"他推开舱门。","cliffhanger":"","next_opening_hint":"他推开舱门后，先看见蓝色火光。"}}`,
	}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	got, err := agent.ExtractWithAgentSDK(context.Background(), "P1-V1-C1", "醒来", "正文")
	if err != nil {
		t.Fatalf("ExtractWithAgentSDK() error = %v", err)
	}
	if got.ChapterID != "P1-V1-C1" || got.Title != "醒来" {
		t.Fatalf("identity was not normalized: %s/%s", got.ChapterID, got.Title)
	}
}

func TestRecapAgentCompactsOversizedRecap(t *testing.T) {
	content := `{"recap":{"chapter_id":"WRONG-ID","title":"错误标题","location":"残骸","time":"同夜","present":["林砚"],"plot_beats":["一","二","三","四","五","六","七","八","九"],"decisions":[],"reveals":["` + strings.Repeat("线索", 100) + `"],"unresolved":[],"promises":[],"items":[],"status":[],"last_line":"他推开舱门。","cliffhanger":"","next_opening_hint":"他推开舱门后，蓝色火光照进残骸。"}}`
	runtime := &fakeRecapRuntimeWithContent{contents: []string{content}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	got, err := agent.ExtractWithAgentSDK(context.Background(), "P1-V1-C1", "醒来", "正文")
	if err != nil {
		t.Fatalf("ExtractWithAgentSDK() error = %v", err)
	}
	if len(got.PlotBeats) != 8 {
		t.Fatalf("PlotBeats len = %d, want 8", len(got.PlotBeats))
	}
	if got.Decisions == nil || got.Items == nil {
		t.Fatalf("empty recap lists should be normalized to empty slices, got decisions=%#v items=%#v", got.Decisions, got.Items)
	}
	if len([]rune(got.Reveals[0])) > 140 {
		t.Fatalf("Reveal was not clipped: %d runes", len([]rune(got.Reveals[0])))
	}
	if got.ChapterID != "P1-V1-C1" || got.Title != "醒来" {
		t.Fatalf("identity was not normalized: %s/%s", got.ChapterID, got.Title)
	}
}

func TestRecapAgentRepairsNextOpeningHintContinuation(t *testing.T) {
	runtime := &fakeRecapRuntimeWithContent{contents: []string{
		`{"recap":{"chapter_id":"P1-V1-C1","title":"醒来","location":"残骸","time":"同夜","present":["林砚"],"plot_beats":["林砚醒来。"],"decisions":[],"reveals":[],"unresolved":[],"promises":[],"items":[],"status":[],"last_line":"而猎物踏入陷阱的那一刻，才会发现网早就被另一双眼睛看穿了。","cliffhanger":"","next_opening_hint":"次日午后，灵泉方向的煞气尚未涌动。"}}`,
	}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	got, err := agent.ExtractWithAgentSDK(context.Background(), "P1-V1-C1", "醒来", "正文")
	if err != nil {
		t.Fatalf("ExtractWithAgentSDK() error = %v", err)
	}
	if !strings.Contains(got.NextOpeningHint, "猎物踏入陷阱") {
		t.Fatalf("NextOpeningHint was not anchored to last line: %q", got.NextOpeningHint)
	}
}

func TestRecapAgentAllowsConsistencyWarningWhenMinimalValid(t *testing.T) {
	runtime := &fakeRecapRuntimeWithContent{contents: []string{
		`{"recap":{"chapter_id":"P1-V1-C1","title":"醒来","location":"残骸","time":"同夜","present":["林砚"],"plot_beats":["林砚醒来。"],"decisions":[],"reveals":[],"unresolved":[],"promises":[],"items":[],"status":[],"last_line":"他推开舱门。","cliffhanger":"","next_opening_hint":"远处传来新的警报声。"}}`,
	}}
	agent := &RecapAgent{base: NewBaseAgent(BaseAgentConfig{
		Name:     "RecapAgent",
		Runtime:  runtime,
		Config:   &llm.Config{},
		Language: "zh",
	})}

	got, err := agent.ExtractWithAgentSDK(context.Background(), "P1-V1-C1", "醒来", "正文")
	if err != nil {
		t.Fatalf("ExtractWithAgentSDK() error = %v", err)
	}
	if got.Location != "残骸" {
		t.Fatalf("Location = %q", got.Location)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want no retry for consistency warning", runtime.calls)
	}
}
