package agents

import (
	"fmt"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// init registers the translate agent factory
// This is called automatically when the package is imported
func init() {
	RegisterAgent("translate", func(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) Agent {
		return NewTranslateAgent(client, config, projectLLM)
	})
}

// TranslateAgent handles AI translation
type TranslateAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
	language   string
}

// NewTranslateAgent creates a new TranslateAgent
func NewTranslateAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *TranslateAgent {
	return &TranslateAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
		language:   "zh",
	}
}

// SetLanguage sets the output language
func (a *TranslateAgent) SetLanguage(language string) {
	a.language = language
}

// Translate performs translation using the provided prompts
func (a *TranslateAgent) Translate(systemPrompt, userPrompt string) (string, error) {
	logger.Section("TRANSLATE AGENT - Translation")
	logger.Info("Source language: %s", a.language)

	provider, model := a.config.GetActiveModel(a.projectLLM)
	if provider == nil || model == nil {
		return "", fmt.Errorf("failed to get active LLM configuration")
	}

	logger.Info("Using model: %s/%s", provider.Name, model.Name)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	response, err := a.client.ChatCompletion(messages, nil)
	if err != nil {
		return "", fmt.Errorf("translation request failed: %w", err)
	}

	logger.Info("Translation completed successfully")
	return response.Content, nil
}
