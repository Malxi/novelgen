package agents

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// SetupGenInput is the input for setup generation
type SetupGenInput struct {
	Idea string `json:"idea" md:"idea" desc:"User's story idea or concept"`
}

// SetupGenOutput is the output for setup generation
type SetupGenOutput struct {
	Setup models.StorySetup `json:"setup" md:"setup" desc:"Generated story setup with premise, genres, themes, rules"`
}

// SetupImproveInput is the input for setup improvement
type SetupImproveInput struct {
	ReviewResult  models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	ExistingSetup models.StorySetup   `json:"existing_setup" md:"existing_setup" desc:"Current story setup to improve"`
}

// SetupImproveOutput is the output for setup improvement
type SetupImproveOutput struct {
	Setup models.StorySetup `json:"setup" md:"setup" desc:"Improved story setup"`
}

// SetupReviewInput is the input for setup review
type SetupReviewInput struct {
	ExistingSetup models.StorySetup `json:"existing_setup" md:"existing_setup" desc:"Story setup to review"`
}

// SetupReviewOutput is the output for setup review
type SetupReviewOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
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
	params := InvokeParams{
		Skills:  []string{"setup-gen"},
		Command: "generate a story setup",
	}
	if err := a.base.Execute(ctx, params, input, &output.Setup); err != nil {
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

	var output SetupImproveOutput
	params := InvokeParams{
		Skills:  []string{"setup-improve"},
		Command: "improve the story setup",
	}
	if err := a.base.Execute(ctx, params, input, &output.Setup); err != nil {
		return SetupImproveOutput{}, err
	}

	// Normalize and validate
	a.normalizeSetup(&output.Setup)

	return output, nil
}

// Review reviews an existing story setup and provides improvement suggestions
func (a *SetupAgent) Review(ctx context.Context, input SetupReviewInput) (SetupReviewOutput, error) {
	logger.Section("SETUP AGENT - Story Setup Review")
	logger.Info("Project: %s", input.ExistingSetup.ProjectName)
	logger.Info("Language: %s", a.base.language)

	var output SetupReviewOutput
	params := InvokeParams{
		Skills:  []string{"setup-review"},
		Command: "review the story setup and provide improvement suggestions",
	}
	if err := a.base.Execute(ctx, params, input, &output.Result); err != nil {
		return SetupReviewOutput{}, err
	}

	// Log result
	logger.Section("Setup Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	for _, dim := range output.Result.Dimensions {
		logger.Info("%s: %.1f/%.0f", dim.Name, dim.Score, dim.Max)
	}
	logger.Info("Summary: %s", output.Result.Summary)
	logger.Info("Strengths: %d items", len(output.Result.Strengths))
	logger.Info("Suggestions: %d items", len(output.Result.Suggestions))

	return output, nil
}

// Iterate runs the review-improvement loop for story setup
// It directly uses Review and Improve methods
func (a *SetupAgent) Iterate(ctx context.Context, setup *models.StorySetup, maxIterations int, qualityThreshold float64, forceImprove bool) (*models.StorySetup, *models.ReviewResult, error) {
	logger.Section("SETUP AGENT - Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}

	currentSetup := *setup
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review current setup
		reviewInput := SetupReviewInput{ExistingSetup: currentSetup}
		reviewOutput, err := a.Review(ctx, reviewInput)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}

		finalReview = &reviewOutput.Result

		// Check if quality meets threshold
		scoreMeetsThreshold := reviewOutput.Result.OverallScore >= qualityThreshold
		if scoreMeetsThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
		}

		// Determine if we should improve
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		// Improve the setup with review feedback
		improveInput := SetupImproveInput{
			ExistingSetup: currentSetup,
			ReviewResult:  reviewOutput.Result,
		}
		improveOutput, err := a.Improve(ctx, improveInput)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}

		currentSetup = improveOutput.Setup
		logger.Info("Setup improved based on review suggestions")
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
		logger.Info("✓ Setup improved based on review suggestions")
	}

	return &currentSetup, finalReview, nil
}

// normalizeSetup normalizes and validates the setup fields
func (a *SetupAgent) normalizeSetup(setup *models.StorySetup) {
	setup.ProjectName = strings.TrimSpace(setup.ProjectName)
	setup.Premise = strings.TrimSpace(setup.Premise)
	setup.Theme = strings.TrimSpace(setup.Theme)
	setup.TargetAudience = strings.TrimSpace(setup.TargetAudience)
	setup.Tone = strings.TrimSpace(setup.Tone)
	setup.Tense = strings.TrimSpace(setup.Tense)
	setup.POVStyle = strings.TrimSpace(setup.POVStyle)
	setup.WritingStyle = setup.WritingStyle.CompactReference(len([]rune(setup.WritingStyle.ReferenceExcerpt)))
	for i := range setup.Storylines {
		setup.Storylines[i].Name = strings.TrimSpace(setup.Storylines[i].Name)
		setup.Storylines[i].Description = strings.TrimSpace(setup.Storylines[i].Description)
		setup.Storylines[i].Type = strings.TrimSpace(setup.Storylines[i].Type)
		setup.Storylines[i].Scope = strings.TrimSpace(setup.Storylines[i].Scope)
		setup.Storylines[i].PayoffStyle = strings.TrimSpace(setup.Storylines[i].PayoffStyle)
		setup.Storylines[i].SetupRole = strings.TrimSpace(setup.Storylines[i].SetupRole)
		setup.Storylines[i].Desire = strings.TrimSpace(setup.Storylines[i].Desire)
		setup.Storylines[i].Opposition = strings.TrimSpace(setup.Storylines[i].Opposition)
		setup.Storylines[i].Stakes = strings.TrimSpace(setup.Storylines[i].Stakes)
		setup.Storylines[i].Turn = strings.TrimSpace(setup.Storylines[i].Turn)
		setup.Storylines[i].Payoff = strings.TrimSpace(setup.Storylines[i].Payoff)
		setup.Storylines[i].OpenQuestion = strings.TrimSpace(setup.Storylines[i].OpenQuestion)
		var pressurePoints []string
		for _, point := range setup.Storylines[i].PressurePoints {
			point = strings.TrimSpace(point)
			if point != "" {
				pressurePoints = append(pressurePoints, point)
			}
		}
		setup.Storylines[i].PressurePoints = pressurePoints
	}

	if setup.ProjectName == "" {
		logger.Warn("AI did not generate project name, using default")
		setup.ProjectName = "Untitled Novel"
	}

	// Validate tense (support both English and Chinese)
	validTenses := []string{
		"past", "过去", "过去时",
		"present", "现在", "现在时",
	}
	tenseValid := false
	for _, valid := range validTenses {
		if setup.Tense == valid {
			tenseValid = true
			break
		}
	}
	if setup.Tense != "" && !tenseValid {
		logger.Warn("Invalid tense value '%s', clearing", setup.Tense)
		setup.Tense = ""
	}

	// Validate POV style (support both English and Chinese)
	validPOVStyles := []string{
		"first person", "第一人称",
		"third person limited", "第三人称有限", "第三人称限制", "第三人称有限视角",
		"third person omniscient", "第三人称全知", "第三人称上帝视角", "第三人称全知视角",
	}
	povValid := false
	for _, valid := range validPOVStyles {
		if setup.POVStyle == valid {
			povValid = true
			break
		}
	}
	if setup.POVStyle != "" && !povValid {
		logger.Warn("Invalid POV style value '%s', clearing", setup.POVStyle)
		setup.POVStyle = ""
	}
}
