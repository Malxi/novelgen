package agents

import (
	"context"
	"strings"
	"testing"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/models"
)

type fakeTranslateRuntime struct {
	invocation agentruntime.Invocation
}

func (r *fakeTranslateRuntime) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.invocation = invocation
	return &agentruntime.Result{
		Content: `{"translation":"The fire woke in the ruins."}`,
		Usage:   agentruntime.Usage{TotalTokens: 12},
	}, nil
}

func TestTranslateWithAgentSDKUsesNoProjectTools(t *testing.T) {
	runtime := &fakeTranslateRuntime{}
	projectLLM := &models.ProjectLLM{Provider: "claude", Model: "deepseek-v4-flash"}
	agent := NewTranslateAgent(nil, &llm.Config{}, projectLLM)
	agent.base = NewBaseAgent(BaseAgentConfig{
		Name:       "TranslateAgent",
		Runtime:    runtime,
		Config:     &llm.Config{},
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	got, err := agent.TranslateWithAgentSDK(context.Background(), "废墟里的火醒了。", "zh", "en")
	if err != nil {
		t.Fatalf("TranslateWithAgentSDK() error = %v", err)
	}
	if got != "The fire woke in the ruins." {
		t.Fatalf("translation = %q", got)
	}
	if !runtime.invocation.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if len(runtime.invocation.SDKSkills) != 1 || runtime.invocation.SDKSkills[0] != "translate-workflow" {
		t.Fatalf("SDKSkills = %#v", runtime.invocation.SDKSkills)
	}
	if len(runtime.invocation.Tools) != 0 || len(runtime.invocation.AllowedTools) != 0 || len(runtime.invocation.ToolAllowlist) != 0 {
		t.Fatalf("translate SDK should not expose tools: tools=%#v allowed=%#v allowlist=%#v",
			runtime.invocation.Tools, runtime.invocation.AllowedTools, runtime.invocation.ToolAllowlist)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "废墟里的火醒了") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Source Lang") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Target Lang") {
		t.Fatalf("UserPrompt missing translation input fields: %s", runtime.invocation.UserPrompt)
	}
}
