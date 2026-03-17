package agents

import (
	"encoding/json"
	"fmt"

	"nolvegen/internal/llm"
	"nolvegen/internal/models"
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
func (a *InitAgent) GenerateStorySetup(prompt string) (*models.StorySetup, error) {
	fmt.Println("🤖 Generating story setup with AI...")
	fmt.Println()

	// Build the prompt
	systemPrompt := `You are a creative writing assistant specializing in novel planning.
Your task is to generate a structured story setup based on the user's idea.

Respond ONLY with a valid JSON object in the following format:
{
  "project_name": "Title of the novel",
  "genres": ["Genre1", "Genre2"],
  "premise": "A compelling description of what the story is about",
  "theme": "The central theme (e.g., 'courage vs power', 'redemption')",
  "rules": ["Story rule 1", "Story rule 2"],
  "target_audience": "Target audience (e.g., Young Adult, Adult)",
  "tone": "Tone/style (e.g., Epic, Hopeful, Dark, Gritty)",
  "tense": "past or present",
  "pov_style": "first-person, third-person limited, or third-person omniscient"
}

Make the story setup creative, coherent, and suitable for a full-length novel.`

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Create a story setup based on this idea: %s", prompt),
		},
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
