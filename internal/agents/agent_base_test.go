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
