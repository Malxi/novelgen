package agents

import (
	"context"
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
	base       *BaseAgent
}

type TranslateAgentSDKInput struct {
	Content    string `json:"content" md:"content" desc:"Source text to translate"`
	SourceLang string `json:"source_lang" md:"source_lang" desc:"Source language code or name"`
	TargetLang string `json:"target_lang" md:"target_lang" desc:"Target language code or name"`
}

type TranslateAgentSDKOutput struct {
	Translation string `json:"translation" md:"translation" desc:"Translated text preserving formatting and narrative style"`
}

// NewTranslateAgent creates a new TranslateAgent
func NewTranslateAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *TranslateAgent {
	agent := &TranslateAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
		language:   "zh",
	}
	agent.base = NewBaseAgent(BaseAgentConfig{
		Name:       "TranslateAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   agent.language,
	})
	return agent
}

// SetLanguage sets the output language
func (a *TranslateAgent) SetLanguage(language string) {
	a.language = language
	if a.base != nil {
		a.base.SetLanguage(language)
	}
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

	response, err := a.client.ChatCompletion(context.Background(), messages, nil)
	if err != nil {
		return "", fmt.Errorf("translation request failed: %w", err)
	}

	logger.Info("Translation completed successfully")
	return response.Content, nil
}

// TranslateWithAgentSDK translates file content through the Claude Agent SDK.
// The SDK agent may not read or write files; Go owns input/output file IO.
func (a *TranslateAgent) TranslateWithAgentSDK(ctx context.Context, content, sourceLang, targetLang string) (string, error) {
	logger.Section("TRANSLATE AGENT SDK - Translation")
	if a.base == nil {
		a.base = NewBaseAgent(BaseAgentConfig{
			Name:       "TranslateAgent",
			Client:     a.client,
			Config:     a.config,
			ProjectLLM: a.projectLLM,
			Language:   a.language,
		})
	}
	input := TranslateAgentSDKInput{
		Content:    content,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	}
	var output TranslateAgentSDKOutput
	params := InvokeParams{
		SDKSkills:      []string{"translate-workflow"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		MaxTurns:       4,
		Timeout:        900,
		Command:        "translate the provided text without reading or writing files",
	}
	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}
	if output.Translation == "" {
		return "", fmt.Errorf("agent-sdk returned empty translation")
	}
	logger.Info("[ok] Agent SDK translated %d source characters to %d target characters", len(content), len(output.Translation))
	return output.Translation, nil
}
