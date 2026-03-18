package agents

import (
	"encoding/json"
	"fmt"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// SetupAgent handles AI generation for story setup
type SetupAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
	language   string
}

// NewSetupAgent creates a new SetupAgent
func NewSetupAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *SetupAgent {
	return &SetupAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
		language:   "zh", // default to Chinese
	}
}

// SetLanguage sets the output language
func (a *SetupAgent) SetLanguage(language string) {
	a.language = language
}

// GenerateStorySetup generates a story setup from a prompt
func (a *SetupAgent) GenerateStorySetup(idea string) (*models.StorySetup, error) {
	logger.Section("SETUP AGENT - Story Setup Generation")
	logger.Info("Idea: %s", idea)
	logger.Info("Language: %s", a.language)

	// Build prompts manually with language support
	systemPrompt := prompts.GetStorySetupSystemPrompt(a.language)
	userPrompt := fmt.Sprintf("Create a story setup based on this idea: %s", idea)

	// Add output requirements
	outputRequirements := fmt.Sprintf(`

=== OUTPUT REQUIREMENTS ===
Format: json
Language: All content MUST be in %s
Structure:
%s
=== END REQUIREMENTS ===`, prompts.GetLanguageName(a.language), prompts.StructToJSONSchema(models.StorySetup{}, "  "))

	fullSystemPrompt := systemPrompt + outputRequirements

	// Log prompts
	logger.Prompt(string(prompts.SkillStorySetup), "default", fullSystemPrompt, userPrompt)

	messages := []llm.Message{
		{Role: "system", Content: fullSystemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)

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
