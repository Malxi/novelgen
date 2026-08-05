package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/models"
)

type fakeWriteRuntime struct {
	invocation agentruntime.Invocation
	content    string
	contents   []string
	review     *models.ReviewResult
	summary    *agentruntime.LiveSummary
	calls      int
}

func (r *fakeWriteRuntime) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.calls++
	r.invocation = invocation
	content := r.content
	if len(r.contents) >= r.calls {
		content = r.contents[r.calls-1]
	}
	if content == "" {
		content = "Lin opened the sealed door and saw blue light. He kept moving, counted every breath, and chose the only path that matched the old signal."
	}
	var payload []byte
	var err error
	if r.review != nil {
		payload, err = json.Marshal(writeAgentSDKReviewOutput{Result: writeAgentSDKReviewResult{
			OverallScore: r.review.OverallScore,
			Summary:      r.review.Summary,
			Strengths:    r.review.Strengths,
			Weaknesses:   r.review.Weaknesses,
			Suggestions:  r.review.Suggestions,
		}})
	} else {
		payload, err = json.Marshal(WriteGenOutput{Content: content})
	}
	if err != nil {
		return nil, err
	}
	summary := r.summary
	if summary == nil {
		summary = &agentruntime.LiveSummary{
			QueryCalls:          1,
			ContextQueryCalls:   1,
			CheckCalls:          1,
			AllowedToolCommands: append([]string(nil), invocation.ToolAllowlist...),
		}
	}
	return &agentruntime.Result{
		Content:     string(payload),
		Usage:       agentruntime.Usage{TotalTokens: 42},
		LiveSummary: summary,
	}, nil
}

func TestWriteAgentGenerateChapterWithAgentSDKUsesFocusedWorkflow(t *testing.T) {
	runtime := &fakeWriteRuntime{}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin opens the sealed door.",
	}

	content, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 0)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK() error = %v", err)
	}
	if !strings.Contains(content, "sealed door") {
		t.Fatalf("content = %q", content)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if got := runtime.invocation.SDKSkills; len(got) != 1 || got[0] != "write-chapter-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if got := runtime.invocation.ToolAllowlist; !containsAllStrings(got,
		writeGenRequiredContextQuery("P1-V1-C1")) {
		t.Fatalf("ToolAllowlist = %#v", got)
	}
	if len(runtime.invocation.ToolAllowlist) != 1 {
		t.Fatalf("write generation should grant exactly one required query: %#v", runtime.invocation.ToolAllowlist)
	}
	if containsAllStrings(runtime.invocation.ToolAllowlist, "novelgen tool query") ||
		containsAllStrings(runtime.invocation.ToolAllowlist, "novelgen tool check") {
		t.Fatalf("write generation should not grant broad query/check tools: %#v", runtime.invocation.ToolAllowlist)
	}
	if strings.Contains(strings.Join(runtime.invocation.ToolAllowlist, " "), "patch") {
		t.Fatalf("write generation should not grant patch tools: %#v", runtime.invocation.ToolAllowlist)
	}
	if runtime.invocation.Options.MaxTurns != 18 || runtime.invocation.Options.Timeout != 600 {
		t.Fatalf("MaxTurns/Timeout = %d/%d, want 18/600", runtime.invocation.Options.MaxTurns, runtime.invocation.Options.Timeout)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "Opening") || !strings.Contains(runtime.invocation.UserPrompt, "Lin opens the sealed door") {
		t.Fatalf("UserPrompt did not include chapter title/summary")
	}
	if runtime.invocation.OutputJSONSchema["type"] != "object" {
		t.Fatalf("OutputJSONSchema type = %v, want object", runtime.invocation.OutputJSONSchema["type"])
	}
	schemaBytes, err := json.Marshal(runtime.invocation.OutputJSONSchema)
	if err != nil {
		t.Fatalf("marshal output schema: %v", err)
	}
	schemaText := string(schemaBytes)
	for _, unwanted := range []string{"dimensions", "continuity_issues", "iteration"} {
		if strings.Contains(schemaText, unwanted) {
			t.Fatalf("agent-sdk write review schema should be compact and omit %q: %s", unwanted, schemaText)
		}
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKHistoryRequiresLogQuery(t *testing.T) {
	runtime := &fakeWriteRuntime{
		summary: &agentruntime.LiveSummary{
			QueryCalls:        2,
			ContextQueryCalls: 1,
			AllowedToolCommands: []string{
				writeGenRequiredContextQuery("P1-V1-C1"),
				`novelgen tool query logs --view index --limit 5`,
			},
		},
	}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "System Log", Premise: "A protagonist can read host logs."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin reads the first system log."}

	_, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 0, true)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK(history) error = %v", err)
	}
	if got := runtime.invocation.ToolAllowlist; !containsAllStrings(got,
		writeGenRequiredContextQuery("P1-V1-C1"),
		"novelgen tool query logs --view index",
		"novelgen tool query logs --view index --limit 5",
		"novelgen tool query logs --id") {
		t.Fatalf("history ToolAllowlist = %#v", got)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "历史续写已启用") ||
		!strings.Contains(runtime.invocation.Command, "History continuation is requested") {
		t.Fatalf("history mode not passed into prompt/command: command=%q prompt=%q", runtime.invocation.Command, runtime.invocation.UserPrompt)
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKAddsInitialLengthBudget(t *testing.T) {
	runtime := &fakeWriteRuntime{
		content: strings.Repeat("Lin reads the system log and turns the warning into action. ", 80),
		summary: &agentruntime.LiveSummary{
			QueryCalls:          1,
			ContextQueryCalls:   1,
			AllowedToolCommands: []string{writeGenRequiredContextQuery("P1-V1-C1")},
		},
	}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "System Log", Premise: "A protagonist can read host logs."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin reads the first system log."}

	_, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 800)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK() error = %v", err)
	}
	for _, want := range []string{
		"target_words=800",
		"preferred range 760-840",
		"initial hard max 960",
		"absolute validation max 1200",
		"4-6 paragraphs",
		"content field",
	} {
		if !strings.Contains(runtime.invocation.Command, want) {
			t.Fatalf("command missing %q: %s", want, runtime.invocation.Command)
		}
		if !strings.Contains(runtime.invocation.UserPrompt, want) {
			t.Fatalf("prompt missing %q: %s", want, runtime.invocation.UserPrompt)
		}
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKRejectsLargeOvershoot(t *testing.T) {
	runtime := &fakeWriteRuntime{content: strings.Repeat("word ", 700)}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin opens the sealed door.",
	}

	_, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 200)
	if err == nil {
		t.Fatalf("GenerateChapterWithAgentSDK() error = nil, want overshoot error")
	}
	if !strings.Contains(err.Error(), "agent-sdk returned too much content") {
		t.Fatalf("error = %v, want overshoot error", err)
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKRetriesLargeOvershoot(t *testing.T) {
	runtime := &fakeWriteRuntime{contents: []string{
		strings.Repeat("word ", 2000),
		strings.Repeat("Lin reads the system log and chooses a safer route. ", 90),
	}}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A log grants information advantage."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin reads the first system log.",
	}

	content, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 800)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK() retry error = %v", err)
	}
	if runtime.calls != 2 {
		t.Fatalf("runtime calls = %d, want 2", runtime.calls)
	}
	if !strings.Contains(content, "safer route") {
		t.Fatalf("content = %q", content)
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKRetriesSoftOvershoot(t *testing.T) {
	runtime := &fakeWriteRuntime{contents: []string{
		strings.Repeat("word ", 1120),
		strings.Repeat("Lin reads the system log and chooses the safer route. ", 88),
	}}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A log grants information advantage."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin reads the first system log."}

	content, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 800)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK() soft retry error = %v", err)
	}
	if runtime.calls != 2 {
		t.Fatalf("runtime calls = %d, want 2", runtime.calls)
	}
	if !strings.Contains(content, "safer route") {
		t.Fatalf("content = %q", content)
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKRetriesSoftShortfall(t *testing.T) {
	runtime := &fakeWriteRuntime{contents: []string{
		strings.Repeat("word ", 650),
		strings.Repeat("Lin reads the hidden pattern and acts. ", 125),
	}}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A log grants information advantage."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin reads the first system log."}

	content, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 800)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK() shortfall retry error = %v", err)
	}
	if runtime.calls != 2 {
		t.Fatalf("runtime calls = %d, want 2", runtime.calls)
	}
	if !strings.Contains(content, "hidden pattern") {
		t.Fatalf("content = %q", content)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "Length Shortfall Retry") ||
		!strings.Contains(runtime.invocation.UserPrompt, "scene-level texture") {
		t.Fatalf("shortfall retry prompt missing expansion instruction: %s", runtime.invocation.UserPrompt)
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKRetriesSecondTimeWhenStillTooLong(t *testing.T) {
	runtime := &fakeWriteRuntime{contents: []string{
		strings.Repeat("word ", 2000),
		strings.Repeat("word ", 1300),
		strings.Repeat("Lin reads the system log and acts with care. ", 85),
	}}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A log grants information advantage."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin reads the first system log.",
	}

	content, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 800)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK() second retry error = %v", err)
	}
	if runtime.calls != 3 {
		t.Fatalf("runtime calls = %d, want 3", runtime.calls)
	}
	if !strings.Contains(content, "acts with care") {
		t.Fatalf("content = %q", content)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "final compression attempt") ||
		!strings.Contains(runtime.invocation.UserPrompt, "fail-fast ceiling") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Do not run any Bash command") {
		t.Fatalf("final retry prompt did not include compressed instruction: %s", runtime.invocation.UserPrompt)
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKRetriesShortfallAfterCompression(t *testing.T) {
	runtime := &fakeWriteRuntime{contents: []string{
		strings.Repeat("word ", 2000),
		strings.Repeat("word ", 1300),
		strings.Repeat("word ", 650),
		strings.Repeat("Lin follows the log and gives each beat texture. ", 90),
	}}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A log grants information advantage."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin reads the first system log."}

	content, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 800)
	if err != nil {
		t.Fatalf("GenerateChapterWithAgentSDK() compression shortfall retry error = %v", err)
	}
	if runtime.calls != 4 {
		t.Fatalf("runtime calls = %d, want 4", runtime.calls)
	}
	if !strings.Contains(content, "beat texture") {
		t.Fatalf("content = %q", content)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "Length Shortfall Retry") {
		t.Fatalf("final retry prompt missing shortfall instruction: %s", runtime.invocation.UserPrompt)
	}
}

func TestWriteAgentGenerateChapterWithAgentSDKRejectsFinalHardShortfall(t *testing.T) {
	runtime := &fakeWriteRuntime{contents: []string{
		strings.Repeat("word ", 500),
		strings.Repeat("word ", 500),
		strings.Repeat("word ", 500),
		strings.Repeat("word ", 500),
		strings.Repeat("word ", 500),
	}}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A log grants information advantage."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin reads the first system log."}

	_, err := agent.GenerateChapterWithAgentSDK(context.Background(), chapter, nil, nil, 800)
	if err == nil {
		t.Fatalf("GenerateChapterWithAgentSDK() error = nil, want final hard shortfall error")
	}
	if !strings.Contains(err.Error(), "too little content") || !strings.Contains(err.Error(), "hard min") {
		t.Fatalf("error = %v, want final hard shortfall", err)
	}
	if runtime.calls != 5 {
		t.Fatalf("runtime calls = %d, want 5", runtime.calls)
	}
}

func TestWriteAgentImproveWithAgentSDKUsesFocusedWorkflow(t *testing.T) {
	runtime := &fakeWriteRuntime{content: "Lin tightened the loose cable, stepped through the sealed door, and kept the blue light behind his shoulder."}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin opens the sealed door.",
	}

	content, err := agent.GenerateChapterWithSuggestionsAgentSDK(context.Background(), chapter, nil, nil, 0, "old draft", "tighten the action", false, false, 2)
	if err != nil {
		t.Fatalf("GenerateChapterWithSuggestionsAgentSDK() error = %v", err)
	}
	if !strings.Contains(content, "loose cable") {
		t.Fatalf("content = %q", content)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if got := runtime.invocation.SDKSkills; len(got) != 2 || got[0] != "novel-tools-core" || got[1] != "write-improve-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if got := runtime.invocation.ToolAllowlist; !containsAllStrings(got,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view index`,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief`,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief --fields existing_chapter_excerpt`,
		`novelgen tool query context --type chapter-repair --id "P1-V1-C1"`,
		"novelgen tool query context --type craft-character",
		"novelgen tool query context --type craft-item",
		"novelgen tool query context --type craft-location",
		"novelgen tool query logs --view index",
		"novelgen tool query logs --id",
		`novelgen tool query chapter --id "P1-V1-C1" --view brief`,
		`novelgen tool query chapter --id "P1-V1-C1" --content`,
		`novelgen tool query chapter --id "P1-V1-C1" --content --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief`,
		`novelgen tool check quality --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8`,
		`novelgen tool check simulation --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8`,
		`novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12`,
		`novelgen tool refresh chapter-dsl --id "P1-V1-C1"`,
		`novelgen tool patch-buffer --id "P1-V1-C1-draft"`,
		`novelgen tool patch chapter --id "P1-V1-C1"`,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"`) {
		t.Fatalf("ToolAllowlist = %#v", got)
	}
	if containsAllStrings(runtime.invocation.ToolAllowlist, "novelgen tool query") ||
		containsAllStrings(runtime.invocation.ToolAllowlist, "novelgen tool check") {
		t.Fatalf("write improvement should not grant broad query/check tools: %#v", runtime.invocation.ToolAllowlist)
	}
	if strings.Contains(strings.Join(runtime.invocation.ToolAllowlist, " "), "--apply") {
		t.Fatalf("write improvement should not grant apply patch tools: %#v", runtime.invocation.ToolAllowlist)
	}
	if runtime.invocation.Options.MaxTurns != 32 || runtime.invocation.Options.Timeout != 600 {
		t.Fatalf("MaxTurns/Timeout = %d/%d, want 32/600", runtime.invocation.Options.MaxTurns, runtime.invocation.Options.Timeout)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "old draft") || !strings.Contains(runtime.invocation.UserPrompt, "tighten the action") {
		t.Fatalf("UserPrompt did not include current draft and suggestions")
	}
	if strings.Contains(runtime.invocation.UserPrompt, "Apply Patches") {
		t.Fatalf("UserPrompt should not enable apply patches")
	}
}

func TestWriteAgentImproveWithAgentSDKApplyAllowsValidatedChapterApply(t *testing.T) {
	runtime := &fakeWriteRuntime{
		content: "Lin tightened the loose cable, stepped through the sealed door, and kept the blue light behind his shoulder.",
		summary: &agentruntime.LiveSummary{
			QueryCalls:          1,
			ContextQueryCalls:   1,
			CheckCalls:          1,
			PatchCalls:          2,
			PatchApplies:        1,
			AllowedToolCommands: []string{`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived`},
		},
	}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin opens the sealed door.",
	}

	_, err := agent.GenerateChapterWithSuggestionsAgentSDK(context.Background(), chapter, nil, nil, 0, "old draft", "tighten the action", true, false, 2)
	if err != nil {
		t.Fatalf("GenerateChapterWithSuggestionsAgentSDK() error = %v", err)
	}
	if got := runtime.invocation.ToolAllowlist; !containsAllStrings(got,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view index`,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief`,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief --fields existing_chapter_excerpt`,
		`novelgen tool query context --type chapter-repair --id "P1-V1-C1"`,
		"novelgen tool query context --type craft-character",
		"novelgen tool query context --type craft-item",
		"novelgen tool query context --type craft-location",
		"novelgen tool query logs --type responses --view brief",
		"novelgen tool query logs --id",
		`novelgen tool query chapter --id "P1-V1-C1" --view brief`,
		`novelgen tool query chapter --id "P1-V1-C1" --content`,
		`novelgen tool query chapter --id "P1-V1-C1" --content --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V1-C1" --view brief`,
		`novelgen tool patch-buffer --id "P1-V1-C1-draft"`,
		`novelgen tool refresh chapter-dsl --id "P1-V1-C1"`,
		`novelgen tool patch chapter --id "P1-V1-C1"`,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"`,
		`novelgen tool patch chapter --id "P1-V1-C1" --apply --refresh-derived`,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived`) {
		t.Fatalf("ToolAllowlist = %#v, want chapter apply patch", got)
	}
	if containsAllStrings(runtime.invocation.ToolAllowlist,
		`novelgen tool patch chapter --id "P1-V1-C1" --apply`,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply`) {
		t.Fatalf("ToolAllowlist should not allow chapter apply without --refresh-derived: %#v", runtime.invocation.ToolAllowlist)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "Apply Patches") || !strings.Contains(runtime.invocation.UserPrompt, "true") {
		t.Fatalf("UserPrompt did not include apply_patches=true")
	}
	if runtime.invocation.Options.MaxTurns != 64 {
		t.Fatalf("apply MaxTurns = %d, want 64", runtime.invocation.Options.MaxTurns)
	}
	if !strings.Contains(runtime.invocation.Command, "return final JSON without patching") ||
		!strings.Contains(runtime.invocation.Command, "Only patch a clean chapter") ||
		!strings.Contains(runtime.invocation.Command, "--apply --refresh-derived") ||
		!strings.Contains(runtime.invocation.Command, "Never emit plans") ||
		!strings.Contains(runtime.invocation.Command, "do not read task/tool-result temp files") ||
		!strings.Contains(runtime.invocation.Command, "run tool refresh again") ||
		!strings.Contains(runtime.invocation.Command, "When verification fails, return the current draft unchanged") ||
		!strings.Contains(runtime.invocation.Command, "do not repair unrelated length/style/check issues") ||
		!strings.Contains(runtime.invocation.Command, "there is no craft-ability query") ||
		!strings.Contains(runtime.invocation.Command, "Before any patch-buffer or patch command") {
		t.Fatalf("Command did not force patch apply for explicit edit requests: %s", runtime.invocation.Command)
	}
}

func TestWriteAgentImproveWithAgentSDKApplyAllowsCleanNoop(t *testing.T) {
	runtime := &fakeWriteRuntime{
		content: "Lin tightened the loose cable, stepped through the sealed door, and kept the blue light behind his shoulder.",
		summary: &agentruntime.LiveSummary{
			QueryCalls:          1,
			ContextQueryCalls:   1,
			CheckCalls:          0,
			PatchCalls:          0,
			PatchApplies:        0,
			AllowedToolCommands: []string{`novelgen tool query context --type chapter-repair --id "P1-V1-C1" --view brief`},
		},
	}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin opens the sealed door."}

	currentDraft := "old draft"
	got, err := agent.GenerateChapterWithSuggestionsAgentSDK(context.Background(), chapter, nil, nil, 0, currentDraft, "keep unchanged when clean", true, false, 2)
	if err != nil {
		t.Fatalf("GenerateChapterWithSuggestionsAgentSDK() clean noop error = %v", err)
	}
	if got != currentDraft {
		t.Fatalf("clean noop returned %q, want current draft", got)
	}
}

func TestWriteAgentImproveWithAgentSDKApplyRequiresRefreshDerivedEvidence(t *testing.T) {
	runtime := &fakeWriteRuntime{
		content: "Lin tightened the loose cable, stepped through the sealed door, and kept the blue light behind his shoulder.",
		summary: &agentruntime.LiveSummary{
			QueryCalls:                1,
			ContextQueryCalls:         1,
			CheckCalls:                1,
			PatchCalls:                2,
			PatchApplies:              1,
			ApplyWithoutFollowupCheck: 1,
			AllowedToolCommands:       []string{`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply`},
		},
	}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin opens the sealed door."}

	_, err := agent.GenerateChapterWithSuggestionsAgentSDK(context.Background(), chapter, nil, nil, 0, "old draft", "tighten the action", true, false, 2)
	if err == nil || !strings.Contains(err.Error(), "patch apply calls without follow-up check") {
		t.Fatalf("expected missing refresh-derived evidence error, got %v", err)
	}
}

func TestWriteAgentImproveWithAgentSDKApplyRejectsDeniedTools(t *testing.T) {
	runtime := &fakeWriteRuntime{
		content: "Lin tightened the loose cable, stepped through the sealed door, and kept the blue light behind his shoulder.",
		summary: &agentruntime.LiveSummary{
			QueryCalls:          1,
			ContextQueryCalls:   1,
			CheckCalls:          1,
			PatchCalls:          2,
			PatchApplies:        1,
			AllowedToolCommands: []string{`novelgen tool patch chapter --id "P1-V1-C1" --apply --refresh-derived`},
			ToolDenied:          1,
			DeniedToolCommands:  []string{`type story\compose\outline.json`},
		},
	}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin opens the sealed door.",
	}

	_, err := agent.GenerateChapterWithSuggestionsAgentSDK(context.Background(), chapter, nil, nil, 0, "old draft", "tighten the action", true, false, 2)
	if err == nil || !strings.Contains(err.Error(), "denied tool calls=1") || !strings.Contains(err.Error(), `type story\compose\outline.json`) {
		t.Fatalf("expected denied tool evidence error, got %v", err)
	}
}

func TestWriteAgentImproveWithAgentSDKRejectsLargeOvershoot(t *testing.T) {
	runtime := &fakeWriteRuntime{content: strings.Repeat("word ", 700)}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin opens the sealed door.",
	}

	_, err := agent.GenerateChapterWithSuggestionsAgentSDK(context.Background(), chapter, nil, nil, 200, "old draft", "tighten the action", false, false)
	if err == nil {
		t.Fatalf("GenerateChapterWithSuggestionsAgentSDK() error = nil, want overshoot error")
	}
	if !strings.Contains(err.Error(), "agent-sdk returned too much content") {
		t.Fatalf("error = %v, want overshoot error", err)
	}
}

func TestWriteAgentReviewWithAgentSDKUsesFocusedReadOnlyWorkflow(t *testing.T) {
	runtime := &fakeWriteRuntime{review: &models.ReviewResult{
		OverallScore: 82,
		Summary:      "Chapter works but needs sharper pacing.",
		Suggestions: []models.ReviewSuggestion{{
			Category:   "pacing",
			TargetID:   "P1-V1-C1",
			Issue:      "Middle beat is soft.",
			Suggestion: "Tighten the middle action beat.",
			Priority:   models.PriorityMedium,
		}},
	}}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Opening",
		Summary: "Lin opens the sealed door.",
	}

	review, err := agent.ReviewChapterWithAgentSDK(context.Background(), chapter, nil, nil, "final chapter content", 2000, 1)
	if err != nil {
		t.Fatalf("ReviewChapterWithAgentSDK() error = %v", err)
	}
	if review.OverallScore != 82 || len(review.Suggestions) != 1 {
		t.Fatalf("review = %#v", review)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if got := runtime.invocation.SDKSkills; len(got) != 2 || got[0] != "novel-tools-core" || got[1] != "write-review-workflow" {
		t.Fatalf("SDKSkills = %#v", got)
	}
	if got := runtime.invocation.ToolAllowlist; !containsAllStrings(got,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view index`,
		`novelgen tool query context --type chapter-write --id "P1-V1-C1" --view brief`,
		`novelgen tool query context --type chapter-repair --id "P1-V1-C1"`,
		"novelgen tool query context --type craft-character",
		"novelgen tool query context --type craft-item",
		"novelgen tool query context --type craft-location",
		"novelgen tool query logs --type prompts --view index",
		"novelgen tool query logs --id",
		`novelgen tool check quality --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8 --target-words 2000`,
		`novelgen tool check simulation --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8`,
		`novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8 --target-words 2000`,
		`novelgen tool check all --target outline --scope chapter --id "P1-V1-C1" --max-issues 8`,
		`novelgen tool refresh chapter-dsl --id "P1-V1-C1"`) {
		t.Fatalf("ToolAllowlist = %#v", got)
	}
	if strings.Contains(strings.Join(runtime.invocation.ToolAllowlist, " "), "patch") {
		t.Fatalf("write review should not grant patch tools: %#v", runtime.invocation.ToolAllowlist)
	}
	if strings.Contains(strings.Join(runtime.invocation.ToolAllowlist, " "), "tool query chapter") {
		t.Fatalf("write review should not grant final chapter content query; content is typed input: %#v", runtime.invocation.ToolAllowlist)
	}
	if runtime.invocation.Options.MaxTurns != 18 || runtime.invocation.Options.Timeout != 600 {
		t.Fatalf("MaxTurns/Timeout = %d/%d, want 18/600", runtime.invocation.Options.MaxTurns, runtime.invocation.Options.Timeout)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "final chapter content") || !strings.Contains(runtime.invocation.UserPrompt, "Opening") {
		t.Fatalf("UserPrompt did not include chapter content/title")
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "current_narrative_units=") ||
		!strings.Contains(runtime.invocation.UserPrompt, "length_review_rule=trust_these_counts") {
		t.Fatalf("UserPrompt did not include deterministic chapter stats: %s", runtime.invocation.UserPrompt)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, `novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8 --target-words 2000`) ||
		!strings.Contains(runtime.invocation.UserPrompt, "target_words=2000") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Do not run alternate check command shapes") {
		t.Fatalf("UserPrompt did not include exact review check command: %s", runtime.invocation.UserPrompt)
	}
	if runtime.invocation.OutputJSONSchema["type"] != "object" {
		t.Fatalf("OutputJSONSchema type = %v, want object", runtime.invocation.OutputJSONSchema["type"])
	}
}

func TestWriteAgentReviewWithAgentSDKRejectsMissingCheckEvidence(t *testing.T) {
	runtime := &fakeWriteRuntime{
		review: &models.ReviewResult{OverallScore: 90, Summary: "ok"},
		summary: &agentruntime.LiveSummary{
			QueryCalls:        1,
			ContextQueryCalls: 1,
			CheckCalls:        0,
		},
	}
	agent := &WriteAgent{
		base: NewBaseAgent(BaseAgentConfig{
			Name:     "WriteAgent",
			Runtime:  runtime,
			Config:   &llm.Config{},
			Language: "zh",
		}),
		setup: &models.StorySetup{ProjectName: "Test", Premise: "A sealed door hides a signal."},
	}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin opens the sealed door."}

	_, err := agent.ReviewChapterWithAgentSDK(context.Background(), chapter, nil, nil, "Lin opens the sealed door.", 0, 1)
	if err == nil || !strings.Contains(err.Error(), "check calls=0") {
		t.Fatalf("expected missing check evidence error, got %v", err)
	}
}

func TestWriteAgentSDKParamsDeclareToolEvidence(t *testing.T) {
	gen := writeAgentSDKParams("generate", "P1-V1-C1", false)
	if gen.ToolEvidence.MinContextQueryCalls != 1 || gen.ToolEvidence.MinCheckCalls != 0 || !gen.ToolEvidence.RequireNoDeniedTools ||
		!containsAllStrings(gen.ToolEvidence.RequiredToolCommands, writeGenRequiredContextQuery("P1-V1-C1")) {
		t.Fatalf("gen ToolEvidence = %#v", gen.ToolEvidence)
	}
	genHistory := writeAgentSDKParams("generate", "P1-V1-C1", true)
	if genHistory.ToolEvidence.MinQueryCalls != 2 ||
		!containsAllStrings(genHistory.ToolEvidence.RequiredToolCommands, writeGenRequiredContextQuery("P1-V1-C1"), "novelgen tool query logs --view index") ||
		!containsAllStrings(genHistory.ToolAllowlist, writeGenRequiredContextQuery("P1-V1-C1"), "novelgen tool query logs --view index", "novelgen tool query logs --view index --limit 5") {
		t.Fatalf("gen history params = evidence=%#v allowlist=%#v", genHistory.ToolEvidence, genHistory.ToolAllowlist)
	}
	if !strings.Contains(genHistory.Command, "Tool outputs are already returned") ||
		!strings.Contains(genHistory.Command, "Do not read log content") ||
		!strings.Contains(genHistory.Command, "filter it with head/Select-String") ||
		!strings.Contains(genHistory.Command, "must not increase target_words") {
		t.Fatalf("gen history command missing stability/length rules: %q", genHistory.Command)
	}
	genWithBudget := writeAgentSDKParams("generate", "P1-V1-C1", true, 800)
	if !strings.Contains(genWithBudget.Command, "target_words=800") ||
		!strings.Contains(genWithBudget.Command, "initial hard max 960") ||
		!strings.Contains(genWithBudget.Command, "4-6 paragraphs") ||
		!strings.Contains(genWithBudget.Command, "word_count") {
		t.Fatalf("gen command missing initial length budget: %q", genWithBudget.Command)
	}
	finalRetry := agentSDKWriteLengthRetryInstruction(1200, 2)
	if !strings.Contains(finalRetry, "900-1140 narrative units") ||
		!strings.Contains(finalRetry, "1300 as a fail-fast ceiling") ||
		!strings.Contains(finalRetry, "Do not run any Bash command") ||
		!strings.Contains(finalRetry, "printf") {
		t.Fatalf("final retry instruction missing strict compression/shell rules: %q", finalRetry)
	}
	improve := writeImproveAgentSDKParams("improve", "P1-V1-C1", false, false)
	if improve.ToolEvidence.MinQueryCalls != 0 || improve.ToolEvidence.MinContextQueryCalls != 1 || improve.ToolEvidence.MinCheckCalls != 1 ||
		improve.ToolEvidence.MinPatchApplyCalls != 0 || improve.ToolEvidence.RequireNoDeniedTools {
		t.Fatalf("improve ToolEvidence = %#v", improve.ToolEvidence)
	}
	improveHistory := writeImproveAgentSDKParams("improve", "P1-V1-C1", false, true)
	if improveHistory.ToolEvidence.MinQueryCalls != 2 ||
		!containsAllStrings(improveHistory.ToolEvidence.RequiredToolCommands, "novelgen tool query logs --view index") ||
		!strings.Contains(improveHistory.Command, "History continuation is requested") ||
		!strings.Contains(improveHistory.Command, "Do not read log content") ||
		!strings.Contains(improveHistory.Command, "must not increase target_words") {
		t.Fatalf("improve history params = command=%q evidence=%#v", improveHistory.Command, improveHistory.ToolEvidence)
	}
	improveWithBudget := writeImproveAgentSDKParams("improve", "P1-V1-C1", false, true, 800)
	if !strings.Contains(improveWithBudget.Command, "target_words=800") ||
		!strings.Contains(improveWithBudget.Command, "initial hard max 960") ||
		!strings.Contains(improveWithBudget.Command, "4-6 paragraphs") {
		t.Fatalf("improve command missing initial length budget: %q", improveWithBudget.Command)
	}
	if !containsAllStrings(improveWithBudget.ToolAllowlist,
		`novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12 --target-words 800`,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --target-words 800`,
	) {
		t.Fatalf("improve target-word allowlist missing focused commands: %#v", improveWithBudget.ToolAllowlist)
	}
	if containsAllStrings(improveWithBudget.ToolAllowlist,
		`novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 12`,
	) || containsAllStrings(improveWithBudget.ToolAllowlist,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft"`,
	) {
		t.Fatalf("improve target-word allowlist should not include no-target chapter check/patch commands: %#v", improveWithBudget.ToolAllowlist)
	}
	improveApply := writeImproveAgentSDKParams("improve", "P1-V1-C1", true, false)
	if improveApply.ToolEvidence.MinQueryCalls != 0 || improveApply.ToolEvidence.MinContextQueryCalls != 1 || improveApply.ToolEvidence.MinCheckCalls != 0 || improveApply.ToolEvidence.MinPatchApplyCalls != 0 ||
		!improveApply.ToolEvidence.RequirePatchApplyFollowupCheck || !improveApply.ToolEvidence.RequireNoDeniedTools ||
		containsAllStrings(improveApply.ToolEvidence.RequiredToolCommands, "--apply --refresh-derived") {
		t.Fatalf("improve apply ToolEvidence = %#v", improveApply.ToolEvidence)
	}
	improveApplyWithBudget := writeImproveAgentSDKParams("improve", "P1-V1-C1", true, false, 800)
	if !strings.Contains(improveApplyWithBudget.Command, "every `novelgen tool patch chapter` dry-run and apply command must include `--target-words 800`") {
		t.Fatalf("improve apply target-word command missing patch flag rule: %q", improveApplyWithBudget.Command)
	}
	if !containsAllStrings(improveApplyWithBudget.ToolAllowlist,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived --target-words 800`,
	) || containsAllStrings(improveApplyWithBudget.ToolAllowlist,
		`novelgen tool patch chapter --id "P1-V1-C1" --patch-buffer "P1-V1-C1-draft" --apply --refresh-derived`,
	) {
		t.Fatalf("improve apply target-word allowlist = %#v", improveApplyWithBudget.ToolAllowlist)
	}
	review := writeReviewAgentSDKParams("review", "P1-V1-C1")
	if review.ToolEvidence.MinContextQueryCalls != 1 || review.ToolEvidence.MinCheckCalls != 1 || !review.ToolEvidence.RequireNoDeniedTools {
		t.Fatalf("review ToolEvidence = %#v", review.ToolEvidence)
	}
	reviewWithBudget := writeReviewAgentSDKParams("review", "P1-V1-C1", 800)
	if !strings.Contains(reviewWithBudget.Command, "target_words=800") ||
		!containsAllStrings(reviewWithBudget.ToolAllowlist,
			`novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8 --target-words 800`,
			`novelgen tool check quality --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8 --target-words 800`,
		) {
		t.Fatalf("review target-word params missing budgeted checks: command=%q allowlist=%#v", reviewWithBudget.Command, reviewWithBudget.ToolAllowlist)
	}
}

func TestFormatChapterContextIncludesCraftContext(t *testing.T) {
	context := &ChapterContext{
		Craft: "ORGANIZATIONS:\n- Ember Guild: goals=control the mine",
	}

	got := formatChapterContext(context)
	if !strings.Contains(got, "CRAFT CONTEXT:") {
		t.Fatalf("expected craft context header, got %q", got)
	}
	if !strings.Contains(got, "Ember Guild") {
		t.Fatalf("expected organization summary, got %q", got)
	}
}

func containsAllStrings(values []string, wants ...string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, want := range wants {
		if !seen[want] {
			return false
		}
	}
	return true
}
