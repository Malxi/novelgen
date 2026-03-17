package agents

import (
	"encoding/json"
	"fmt"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
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
	logger.Section("INIT AGENT - Story Setup Generation")
	logger.Info("Idea: %s", idea)

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts using the prompt manager
	data := prompts.BuildStorySetupData(idea)
	systemPrompt, userPrompt, err := pm.Build(prompts.SkillStorySetup, "default", data)
	if err != nil {
		logger.Error("Failed to build prompt: %v", err)
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Log prompts
	logger.Prompt(string(prompts.SkillStorySetup), "default", systemPrompt, userPrompt)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions()

	logger.Info("Sending request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		logger.Error("AI request failed: %v", err)
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	logger.Info("Received response (%d tokens used)", resp.Usage.TotalTokens)

	// Parse the JSON response
	var setup models.StorySetup
	if err := json.Unmarshal([]byte(resp.Content), &setup); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		logger.Debug("Extracted JSON from markdown: %s", content)
		if err := json.Unmarshal([]byte(content), &setup); err != nil {
			logger.Error("Failed to parse AI response as JSON: %v", err)
			logger.Debug("Raw response: %s", resp.Content)
			return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, resp.Content)
		}
	}

	// Validate required fields
	if setup.ProjectName == "" {
		logger.Warn("AI did not generate project name, using default")
		setup.ProjectName = "Untitled Novel"
	}
	if setup.Premise == "" {
		logger.Error("AI did not generate a premise")
		return nil, fmt.Errorf("AI did not generate a premise")
	}

	// Log result
	logger.Section("Story Setup Result")
	logger.Info("Project Name: %s", setup.ProjectName)
	logger.Info("Genres: %v", setup.Genres)
	logger.Info("Theme: %s", setup.Theme)
	logger.Info("Tone: %s", setup.Tone)

	return &setup, nil
}
