package agents

import (
	"context"
	"strings"
	"testing"

	"novelgen/internal/llm"
	"novelgen/internal/models"
)

type promptCaptureClient struct {
	lastMessages []llm.Message
	response     string
}

func (c *promptCaptureClient) ChatCompletion(ctx context.Context, messages []llm.Message, options *llm.ChatOptions) (*llm.ChatResponse, error) {
	c.lastMessages = append([]llm.Message(nil), messages...)
	if c.response == "" {
		c.response = `{"project_name":"Test","genres":["fantasy"],"premise":"p","theme":"t","rules":["r"],"target_audience":"a","tone":"爽","tense":"past","pov_style":"third person limited"}`
	}
	return &llm.ChatResponse{Content: c.response, Usage: llm.Usage{TotalTokens: 12}}, nil
}

func TestRevisionSessionPromptIncludesHistory(t *testing.T) {
	session := NewRevisionSession("setup", "Make the setup sharper.")
	session.AddUserGuidance(0, "Add more爽点.")
	session.AddReview(1, models.ReviewResult{
		Summary: "The setup is close but the appeal is thin.",
		Suggestions: []models.ReviewSuggestion{
			{Priority: models.PriorityHigh, Category: models.CategoryAppeal, TargetName: "premise", Issue: "No obvious win pattern", Suggestion: "Add a concrete exploit path."},
		},
		Strengths: []string{"core premise is clear"},
	})
	session.AddImprove(1, "Expanded the appeal engine and preserved the premise.")

	prompt := session.Prompt()
	for _, want := range []string{"Revision Session", "setup", "Make the setup sharper.", "No obvious win pattern", "Expanded the appeal engine"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}

func TestSetupImproveCarriesRevisionContextIntoPrompt(t *testing.T) {
	client := &promptCaptureClient{}
	agent := NewSetupAgent(client, &llm.Config{}, &models.ProjectLLM{})

	input := SetupImproveInput{
		ExistingSetup: models.StorySetup{
			ProjectName:    "Old",
			Genres:         []string{"fantasy"},
			Premise:        "old premise",
			Theme:          "old theme",
			Rules:          []string{"old rule"},
			TargetAudience: "readers",
			Tone:           "bright",
			Tense:          "past",
			POVStyle:       "third person limited",
		},
		ReviewResult: models.ReviewResult{
			Summary: "Need more of a payoff engine.",
			Suggestions: []models.ReviewSuggestion{
				{Priority: models.PriorityHigh, Category: models.CategoryAppeal, Issue: "thin appeal", Suggestion: "Add a stronger exploit pattern."},
			},
		},
		RevisionContext: "Revision Session\n- goal: make the setup more爽\n- history:\n  - round 1 review\n    suggestion[high/appeal/global]: thin appeal -> Add a stronger exploit pattern.\n",
	}

	_, err := agent.Improve(context.Background(), input)
	if err != nil {
		t.Fatalf("Improve() returned error: %v", err)
	}
	if len(client.lastMessages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(client.lastMessages))
	}
	userPrompt := client.lastMessages[1].Content
	for _, want := range []string{"Revision Session", "goal: make the setup more爽", "thin appeal", "Add a stronger exploit pattern"} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("user prompt missing %q: %s", want, userPrompt)
		}
	}
}
