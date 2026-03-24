package agents

import (
	"context"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// SetupGenInput is the input for setup generation
type SetupGenInput struct {
	Idea string `md:"idea"`
}

// SetupGenOutput is the output for setup generation
type SetupGenOutput struct {
	Setup models.StorySetup `md:"setup"`
}

// SetupImproveInput is the input for setup improvement
type SetupImproveInput struct {
	ExistingSetup models.StorySetup `md:"existing_setup"`
}

// SetupImproveOutput is the output for setup improvement
type SetupImproveOutput struct {
	Setup models.StorySetup `md:"setup"`
}

// SetupAgent handles AI generation for story setup
// It wraps BaseAgent to provide type-safe methods
type SetupAgent struct {
	base *BaseAgent
}

// NewSetupAgent creates a new SetupAgent
func NewSetupAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *SetupAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "SetupAgent",
		Skills:     []string{"setup-gen"},
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &SetupAgent{base: base}
}

// SetLanguage sets the output language
func (a *SetupAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// Generate creates a story setup from an idea
// This is the type-safe wrapper around BaseAgent.Execute
func (a *SetupAgent) Generate(ctx context.Context, input SetupGenInput) (SetupGenOutput, error) {
	logger.Section("SETUP AGENT - Story Setup Generation")
	logger.Info("Idea: %s", input.Idea)
	logger.Info("Language: %s", a.base.language)

	var output SetupGenOutput
	if err := a.base.Execute(ctx, input, &output.Setup); err != nil {
		return SetupGenOutput{}, err
	}

	// Normalize and validate
	a.normalizeSetup(&output.Setup)

	// Log result
	logger.Section("Story Setup Result")
	logger.Info("Project Name: %s", output.Setup.ProjectName)
	logger.Info("Genres: %v", output.Setup.Genres)
	logger.Info("Theme: %s", output.Setup.Theme)
	logger.Info("Tone: %s", output.Setup.Tone)

	return output, nil
}

// Improve improves an existing story setup
func (a *SetupAgent) Improve(ctx context.Context, input SetupImproveInput) (SetupImproveOutput, error) {
	logger.Section("SETUP AGENT - Story Setup Improvement")
	logger.Info("Project: %s", input.ExistingSetup.ProjectName)
	logger.Info("Language: %s", a.base.language)

	// Create a temporary agent with the improve skill
	improveBase := NewBaseAgent(BaseAgentConfig{
		Name:       "SetupAgent",
		Skills:     []string{"setup-improve"},
		Client:     a.base.client,
		Config:     a.base.config,
		ProjectLLM: a.base.projectLLM,
		Language:   a.base.language,
	})

	var output SetupImproveOutput
	if err := improveBase.Execute(ctx, input, &output.Setup); err != nil {
		return SetupImproveOutput{}, err
	}

	// Normalize and validate
	a.normalizeSetup(&output.Setup)

	return output, nil
}

// normalizeSetup normalizes and validates the setup fields
func (a *SetupAgent) normalizeSetup(setup *models.StorySetup) {
	setup.ProjectName = strings.TrimSpace(setup.ProjectName)
	setup.Premise = strings.TrimSpace(setup.Premise)
	setup.Theme = strings.TrimSpace(setup.Theme)
	setup.TargetAudience = strings.TrimSpace(setup.TargetAudience)
	setup.Tone = strings.TrimSpace(setup.Tone)
	setup.Tense = strings.TrimSpace(strings.ToLower(setup.Tense))
	setup.POVStyle = strings.TrimSpace(strings.ToLower(setup.POVStyle))

	if setup.ProjectName == "" {
		logger.Warn("AI did not generate project name, using default")
		setup.ProjectName = "Untitled Novel"
	}

	if setup.Tense != "" && setup.Tense != "past" && setup.Tense != "present" {
		logger.Warn("Invalid tense value '%s', clearing", setup.Tense)
		setup.Tense = ""
	}

	if setup.POVStyle != "" && setup.POVStyle != "first person" &&
		setup.POVStyle != "third person limited" && setup.POVStyle != "third person omniscient" {
		logger.Warn("Invalid POV style value '%s', clearing", setup.POVStyle)
		setup.POVStyle = ""
	}
}
