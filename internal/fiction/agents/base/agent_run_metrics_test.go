package base

import (
	"context"
	"testing"

	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/runmetrics"
	"novelgen/internal/platform/store"
)

func TestBaseAgentExecuteRecordsRunMetrics(t *testing.T) {
	root := store.MemRoot()
	ctx, run, err := runmetrics.Start(context.Background(), root, "novelgen test")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	agent := &BaseAgent{
		name:        "TestAgent",
		client:      &fakeStaticClient{content: `{"name":"ok"}`},
		config:      &llm.Config{},
		project:     root,
		skillLoader: NewSkillLoader(""),
	}

	var output struct {
		Name string `json:"name"`
	}
	if err := agent.Execute(ctx, InvokeParams{
		Command: "test command",
		Skills:  []string{"write-generate"},
	}, struct{}{}, &output); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	run.Finish("", nil)

	detail, err := runmetrics.Read(root, run.ID())
	if err != nil {
		t.Fatalf("read run metrics: %v", err)
	}
	if detail.Summary.AgentCalls != 1 || detail.Summary.TotalTokens != 10 {
		t.Fatalf("run summary = %#v", detail.Summary)
	}
	if len(detail.Summary.Skills) != 1 || detail.Summary.Skills[0] != "write-generate" {
		t.Fatalf("skills = %#v", detail.Summary.Skills)
	}
}

func TestRepeatedAllowedCommandsNormalizesForStuckDetection(t *testing.T) {
	counts := repeatedAllowedCommands([]string{
		"novelgen tool check all --target chapter --scope chapter --id 'P1-V1-C1'",
		"novelgen tool check all --target chapter --scope chapter --id 'P1-V1-C2'",
		"novelgen tool check all --target chapter --scope chapter --id 'P1-V1-C3'",
	})
	if got := counts["novelgen tool check all --target chapter --scope chapter --id"]; got != 3 {
		t.Fatalf("normalized repeated count = %#v", counts)
	}
}

func TestRunMetricsDistinguishesToolPostFromDenial(t *testing.T) {
	root := store.MemRoot()
	ctx, run, err := runmetrics.Start(context.Background(), root, "novelgen test")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runtime := &fakeRuntime{}
	runtime.toolEvents = []agentruntime.ToolEvent{
		{Source: "pre", Command: "novelgen tool check all --target chapter --id 'P1-V1-C1'", Allowed: true},
		{Source: "post", Command: "novelgen tool check all --target chapter --id 'P1-V1-C1'", DurationMS: 15},
	}
	agent := &BaseAgent{
		name:        "TestAgent",
		runtime:     runtime,
		config:      &llm.Config{},
		project:     root,
		skillLoader: NewSkillLoader(""),
	}

	var output struct {
		Name string `json:"name"`
	}
	if err := agent.Execute(ctx, InvokeParams{Command: "test"}, struct{}{}, &output); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	run.Finish("", nil)
	detail, err := runmetrics.Read(root, run.ID())
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	var post *runmetrics.Event
	for i := range detail.Events {
		if detail.Events[i].Event == runmetrics.EventTool && detail.Events[i].Kind == "post" {
			post = &detail.Events[i]
			break
		}
	}
	if post == nil || post.Status != "completed" || post.Allowed != nil {
		t.Fatalf("post tool event = %#v", post)
	}
}
