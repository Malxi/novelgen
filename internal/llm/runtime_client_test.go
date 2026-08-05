package llm

import (
	"context"
	"testing"

	"novelgen/internal/agentruntime"
)

type fakeRuntime struct {
	invocation agentruntime.Invocation
}

func (r *fakeRuntime) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.invocation = invocation
	return &agentruntime.Result{
		Content: `{"ok":true}`,
		Model:   invocation.Options.Model,
		Usage:   agentruntime.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
	}, nil
}

func TestRuntimeClientChatCompletionUsesRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	client := NewRuntimeBackedClient("claude", runtime, nil)

	resp, err := client.ChatCompletion(context.Background(), []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "user prompt"},
	}, &ChatOptions{Model: "sonnet", Temperature: 0.2, MaxTokens: 123})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Content != `{"ok":true}` {
		t.Fatalf("Content = %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("TotalTokens = %d, want 5", resp.Usage.TotalTokens)
	}
	if runtime.invocation.SystemPrompt != "system prompt" {
		t.Fatalf("SystemPrompt = %q", runtime.invocation.SystemPrompt)
	}
	if runtime.invocation.UserPrompt != "user prompt" {
		t.Fatalf("UserPrompt = %q", runtime.invocation.UserPrompt)
	}
	if runtime.invocation.Options.Model != "sonnet" {
		t.Fatalf("Model = %q", runtime.invocation.Options.Model)
	}
	if runtime.invocation.Options.MaxTokens != 123 {
		t.Fatalf("MaxTokens = %d, want 123", runtime.invocation.Options.MaxTokens)
	}
}
