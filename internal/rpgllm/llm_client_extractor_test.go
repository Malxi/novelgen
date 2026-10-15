package rpgllm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"novelgen/internal/llm"
	"novelgen/internal/rpg"
)

type fakeLLMClient struct {
	messages []llm.Message
	options  *llm.ChatOptions
	response *llm.ChatResponse
	err      error
}

func (f *fakeLLMClient) ChatCompletion(_ context.Context, messages []llm.Message, options *llm.ChatOptions) (*llm.ChatResponse, error) {
	f.messages = messages
	f.options = options
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func TestCompleteMapsPromptAndTextToMessages(t *testing.T) {
	options := &llm.ChatOptions{Model: "test-model"}
	client := &fakeLLMClient{response: &llm.ChatResponse{Content: "{}"}}
	adapter := &NovelgenLLMClient{client: client, options: options}

	content, err := adapter.Complete("system prompt", "user text")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if content != "{}" {
		t.Fatalf("content = %q", content)
	}
	if len(client.messages) != 2 || client.messages[0].Role != "system" || client.messages[0].Content != "system prompt" {
		t.Fatalf("system message = %#v", client.messages)
	}
	if client.messages[1].Role != "user" || client.messages[1].Content != "user text" {
		t.Fatalf("user message = %#v", client.messages)
	}
	if client.options != options {
		t.Fatal("options should be forwarded unchanged")
	}
}

func TestCompleteWrapsClientErrors(t *testing.T) {
	client := &fakeLLMClient{err: errors.New("boom")}
	adapter := &NovelgenLLMClient{client: client}

	if _, err := adapter.Complete("p", "t"); err == nil || !strings.Contains(err.Error(), "LLM request failed") {
		t.Fatalf("error should be wrapped, got %v", err)
	}
}

func TestMergeExtractResultsDedupesByName(t *testing.T) {
	first := &rpg.NovelRPGData{
		Characters:       []*rpg.CharacterTemplate{{Name: "Hero", Description: "first"}},
		Items:            []*rpg.Item{{Name: "Blade"}},
		Timeline:         []*rpg.TimelineEvent{{Location: "t1"}},
		ValidationIssues: []rpg.ValidationIssue{{Type: "issue-1"}},
	}
	second := &rpg.NovelRPGData{
		Characters: []*rpg.CharacterTemplate{{Name: "Hero", Description: "second"}, {Name: "Mentor"}},
		Items:      []*rpg.Item{{Name: "Blade"}, {Name: "Shield"}},
		Timeline:   []*rpg.TimelineEvent{{Location: "t2"}},
	}

	merged := MergeExtractResults([]*rpg.NovelRPGData{first, second})

	if len(merged.Characters) != 2 {
		t.Fatalf("characters = %#v", merged.Characters)
	}
	for _, character := range merged.Characters {
		if character.Name == "Hero" && character.Description != "second" {
			t.Fatalf("later result should win for duplicate names: %#v", character)
		}
	}
	if len(merged.Items) != 2 {
		t.Fatalf("items = %#v", merged.Items)
	}
	if len(merged.Timeline) != 2 {
		t.Fatalf("timeline should concatenate: %#v", merged.Timeline)
	}
	if len(merged.ValidationIssues) != 1 {
		t.Fatalf("validation issues should concatenate: %#v", merged.ValidationIssues)
	}
}

func TestSplitTextIntoChunksReconstructsInput(t *testing.T) {
	text := strings.Repeat("line of prose\n", 40)

	chunks := splitTextIntoChunks(text, 64)

	if len(chunks) < 2 {
		t.Fatalf("expected several chunks, got %d", len(chunks))
	}
	if got := strings.Join(chunks, ""); got != text {
		t.Fatalf("chunks must reconstruct the input:\n%q\n%q", got, text)
	}
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			t.Fatalf("empty chunk produced: %#v", chunks)
		}
	}

	if got := splitTextIntoChunks("short text", 1024); len(got) != 1 || got[0] != "short text\n" {
		t.Fatalf("small input should stay in one chunk: %#v", got)
	}
}
