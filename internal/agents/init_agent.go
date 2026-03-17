package agents

import (
	"encoding/json"
	"fmt"

	"nolvegen/internal/llm"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// InitAgent handles AI generation for story setup
type InitAgent struct {
	client llm.Client
	config *llm.Config
}

// NewInitAgent creates a new InitAgent
func NewInitAgent(client llm.Client, config *llm.Config) *InitAgent {
	return &InitAgent{
		client: client,
		config: config,
	}
}

// GenerateStorySetup generates a story setup from a prompt
func (a *InitAgent) GenerateStorySetup(idea string) (*models.StorySetup, error) {
	fmt.Println("🤖 Generating story setup with AI...")
	fmt.Println()

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts using the prompt manager
	data := prompts.BuildStorySetupData(idea)
	systemPrompt, userPrompt, err := pm.Build(prompts.SkillStorySetup, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions()
	// Use smaller max tokens for story setup generation
	options.MaxTokens = 2000

	fmt.Println("Sending request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	fmt.Printf("Received response (%d tokens used)\n", resp.Usage.TotalTokens)
	fmt.Println()

	// Parse the JSON response
	var setup models.StorySetup
	if err := json.Unmarshal([]byte(resp.Content), &setup); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &setup); err != nil {
			return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, resp.Content)
		}
	}

	// Validate required fields
	if setup.ProjectName == "" {
		setup.ProjectName = "Untitled Novel"
	}
	if setup.Premise == "" {
		return nil, fmt.Errorf("AI did not generate a premise")
	}

	fmt.Println("✓ Story setup generated successfully!")
	fmt.Println()

	return &setup, nil
}
