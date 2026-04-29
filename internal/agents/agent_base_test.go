package agents

import (
	"context"
	"testing"

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
