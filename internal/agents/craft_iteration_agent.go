package agents

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// CraftReviewCharactersInput is the input for character review
type CraftReviewCharactersInput struct {
	StorySetup   models.StorySetup           `md:"story_setup"`
	Outline      string                      `md:"outline"`
	Characters   map[string]models.Character `md:"characters"`
	Iteration    int                         `md:"iteration"`
}

// CraftReviewCharactersOutput is the output for character review
type CraftReviewCharactersOutput struct {
	Result models.ReviewResult `md:"result"`
}

// CraftReviewLocationsInput is the input for location review
type CraftReviewLocationsInput struct {
	StorySetup   models.StorySetup          `md:"story_setup"`
	Outline      string                     `md:"outline"`
	Locations    map[string]models.Location `md:"locations"`
	Iteration    int                        `md:"iteration"`
}

// CraftReviewLocationsOutput is the output for location review
type CraftReviewLocationsOutput struct {
	Result models.ReviewResult `md:"result"`
}

// CraftReviewItemsInput is the input for item review
type CraftReviewItemsInput struct {
	StorySetup   models.StorySetup      `md:"story_setup"`
	Outline      string                 `md:"outline"`
	Items        map[string]models.Item `md:"items"`
	Iteration    int                    `md:"iteration"`
}

// CraftReviewItemsOutput is the output for item review
type CraftReviewItemsOutput struct {
	Result models.ReviewResult `md:"result"`
}

// CraftImproveCharactersInput is the input for character improvement
type CraftImproveCharactersInput struct {
	StorySetup   models.StorySetup           `md:"story_setup"`
	Outline      string                      `md:"outline"`
	Characters   map[string]models.Character `md:"characters"`
	ReviewResult models.ReviewResult         `md:"review_result"`
	CustomPrompt string                      `md:"custom_prompt,omitempty"`
}

// CraftImproveCharactersOutput is the output for character improvement
type CraftImproveCharactersOutput struct {
	Characters map[string]models.Character `md:"characters"`
}

// CraftImproveLocationsInput is the input for location improvement
type CraftImproveLocationsInput struct {
	StorySetup   models.StorySetup          `md:"story_setup"`
	Outline      string                     `md:"outline"`
	Locations    map[string]models.Location `md:"locations"`
	ReviewResult models.ReviewResult        `md:"review_result"`
	CustomPrompt string                     `md:"custom_prompt,omitempty"`
}

// CraftImproveLocationsOutput is the output for location improvement
type CraftImproveLocationsOutput struct {
	Locations map[string]models.Location `md:"locations"`
}

// CraftImproveItemsInput is the input for item improvement
type CraftImproveItemsInput struct {
	StorySetup   models.StorySetup      `md:"story_setup"`
	Outline      string                 `md:"outline"`
	Items        map[string]models.Item `md:"items"`
	ReviewResult models.ReviewResult    `md:"review_result"`
	CustomPrompt string                 `md:"custom_prompt,omitempty"`
}

// CraftImproveItemsOutput is the output for item improvement
type CraftImproveItemsOutput struct {
	Items map[string]models.Item `md:"items"`
}

// CraftIterationAgent handles AI-driven element review and improvement
// It wraps BaseAgent to provide type-safe methods
type CraftIterationAgent struct {
	base    *BaseAgent
	setup   *models.StorySetup
	outline *models.Outline
}

// NewCraftIterationAgent creates a new CraftIterationAgent
func NewCraftIterationAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline) *CraftIterationAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "CraftIterationAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &CraftIterationAgent{
		base:    base,
		setup:   setup,
		outline: outline,
	}
}

// SetLanguage sets the output language
func (a *CraftIterationAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// ReviewCharacters reviews characters and returns improvement suggestions
func (a *CraftIterationAgent) ReviewCharacters(ctx context.Context, characters map[string]models.Character, iteration int) (models.ReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Character Review")
	logger.Info("Iteration: %d, Characters: %d", iteration, len(characters))
	logger.Info("Language: %s", a.base.language)

	input := CraftReviewCharactersInput{
		StorySetup: *a.setup,
		Outline:    a.getOutlineSummary(),
		Characters: characters,
		Iteration:  iteration,
	}

	var output CraftReviewCharactersOutput
	params := InvokeParams{
		Skills:  []string{"craft-review-characters"},
		Command: "review characters and provide improvement suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Result); err != nil {
		return models.ReviewResult{}, err
	}

	logger.Section("Character Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	logger.Info("Suggestions: %d", len(output.Result.Suggestions))

	return output.Result, nil
}

// ReviewLocations reviews locations and returns improvement suggestions
func (a *CraftIterationAgent) ReviewLocations(ctx context.Context, locations map[string]models.Location, iteration int) (models.ReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Location Review")
	logger.Info("Iteration: %d, Locations: %d", iteration, len(locations))
	logger.Info("Language: %s", a.base.language)

	input := CraftReviewLocationsInput{
		StorySetup: *a.setup,
		Outline:    a.getOutlineSummary(),
		Locations:  locations,
		Iteration:  iteration,
	}

	var output CraftReviewLocationsOutput
	params := InvokeParams{
		Skills:  []string{"craft-review-locations"},
		Command: "review locations and provide improvement suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Result); err != nil {
		return models.ReviewResult{}, err
	}

	logger.Section("Location Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	logger.Info("Suggestions: %d", len(output.Result.Suggestions))

	return output.Result, nil
}

// ReviewItems reviews items and returns improvement suggestions
func (a *CraftIterationAgent) ReviewItems(ctx context.Context, items map[string]models.Item, iteration int) (models.ReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Item Review")
	logger.Info("Iteration: %d, Items: %d", iteration, len(items))
	logger.Info("Language: %s", a.base.language)

	input := CraftReviewItemsInput{
		StorySetup: *a.setup,
		Outline:    a.getOutlineSummary(),
		Items:      items,
		Iteration:  iteration,
	}

	var output CraftReviewItemsOutput
	params := InvokeParams{
		Skills:  []string{"craft-review-items"},
		Command: "review items and provide improvement suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Result); err != nil {
		return models.ReviewResult{}, err
	}

	logger.Section("Item Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	logger.Info("Suggestions: %d", len(output.Result.Suggestions))

	return output.Result, nil
}

// ImproveCharacters applies improvements to characters based on review
func (a *CraftIterationAgent) ImproveCharacters(ctx context.Context, characters map[string]models.Character, review models.ReviewResult, customPrompt string) (map[string]models.Character, error) {
	logger.Section("CRAFT ITERATION AGENT - Character Improvement")
	logger.Info("Characters: %d, Suggestions: %d", len(characters), len(review.Suggestions))
	logger.Info("Language: %s", a.base.language)

	input := CraftImproveCharactersInput{
		StorySetup:   *a.setup,
		Outline:      a.getOutlineSummary(),
		Characters:   characters,
		ReviewResult: review,
		CustomPrompt: customPrompt,
	}

	var output CraftImproveCharactersOutput
	params := InvokeParams{
		Skills:  []string{"craft-improve-characters"},
		Command: "improve characters based on review suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Characters); err != nil {
		return nil, err
	}

	logger.Info("✓ Improved %d characters", len(output.Characters))
	return output.Characters, nil
}

// ImproveLocations applies improvements to locations based on review
func (a *CraftIterationAgent) ImproveLocations(ctx context.Context, locations map[string]models.Location, review models.ReviewResult, customPrompt string) (map[string]models.Location, error) {
	logger.Section("CRAFT ITERATION AGENT - Location Improvement")
	logger.Info("Locations: %d, Suggestions: %d", len(locations), len(review.Suggestions))
	logger.Info("Language: %s", a.base.language)

	input := CraftImproveLocationsInput{
		StorySetup:   *a.setup,
		Outline:      a.getOutlineSummary(),
		Locations:    locations,
		ReviewResult: review,
		CustomPrompt: customPrompt,
	}

	var output CraftImproveLocationsOutput
	params := InvokeParams{
		Skills:  []string{"craft-improve-locations"},
		Command: "improve locations based on review suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Locations); err != nil {
		return nil, err
	}

	logger.Info("✓ Improved %d locations", len(output.Locations))
	return output.Locations, nil
}

// ImproveItems applies improvements to items based on review
func (a *CraftIterationAgent) ImproveItems(ctx context.Context, items map[string]models.Item, review models.ReviewResult, customPrompt string) (map[string]models.Item, error) {
	logger.Section("CRAFT ITERATION AGENT - Item Improvement")
	logger.Info("Items: %d, Suggestions: %d", len(items), len(review.Suggestions))
	logger.Info("Language: %s", a.base.language)

	input := CraftImproveItemsInput{
		StorySetup:   *a.setup,
		Outline:      a.getOutlineSummary(),
		Items:        items,
		ReviewResult: review,
		CustomPrompt: customPrompt,
	}

	var output CraftImproveItemsOutput
	params := InvokeParams{
		Skills:  []string{"craft-improve-items"},
		Command: "improve items based on review suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Items); err != nil {
		return nil, err
	}

	logger.Info("✓ Improved %d items", len(output.Items))
	return output.Items, nil
}

// IterateCharacters runs the review-improvement loop for characters
func (a *CraftIterationAgent) IterateCharacters(ctx context.Context, characters map[string]models.Character, maxIterations int, qualityThreshold float64, customPrompt string) (map[string]models.Character, *models.ReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Character Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)

	currentChars := characters
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review
		review, err := a.ReviewCharacters(ctx, currentChars, i)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}
		finalReview = &review

		// Check if quality meets threshold
		if review.OverallScore >= qualityThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", review.OverallScore, qualityThreshold)
			break
		}

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}

		// Check if there are high priority suggestions
		hasHighPriority := false
		for _, s := range review.Suggestions {
			if s.Priority == "high" {
				hasHighPriority = true
				break
			}
		}
		if !hasHighPriority {
			logger.Info("No high priority issues, stopping iteration")
			break
		}

		// Improve
		improved, err := a.ImproveCharacters(ctx, currentChars, review, customPrompt)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}
		currentChars = improved
	}

	return currentChars, finalReview, nil
}

// IterateLocations runs the review-improvement loop for locations
func (a *CraftIterationAgent) IterateLocations(ctx context.Context, locations map[string]models.Location, maxIterations int, qualityThreshold float64, customPrompt string) (map[string]models.Location, *models.ReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Location Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)

	currentLocs := locations
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review
		review, err := a.ReviewLocations(ctx, currentLocs, i)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}
		finalReview = &review

		// Check if quality meets threshold
		if review.OverallScore >= qualityThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", review.OverallScore, qualityThreshold)
			break
		}

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}

		// Check if there are high priority suggestions
		hasHighPriority := false
		for _, s := range review.Suggestions {
			if s.Priority == "high" {
				hasHighPriority = true
				break
			}
		}
		if !hasHighPriority {
			logger.Info("No high priority issues, stopping iteration")
			break
		}

		// Improve
		improved, err := a.ImproveLocations(ctx, currentLocs, review, customPrompt)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}
		currentLocs = improved
	}

	return currentLocs, finalReview, nil
}

// IterateItems runs the review-improvement loop for items
func (a *CraftIterationAgent) IterateItems(ctx context.Context, items map[string]models.Item, maxIterations int, qualityThreshold float64, customPrompt string) (map[string]models.Item, *models.ReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Item Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)

	currentItems := items
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review
		review, err := a.ReviewItems(ctx, currentItems, i)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}
		finalReview = &review

		// Check if quality meets threshold
		if review.OverallScore >= qualityThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", review.OverallScore, qualityThreshold)
			break
		}

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}

		// Check if there are high priority suggestions
		hasHighPriority := false
		for _, s := range review.Suggestions {
			if s.Priority == "high" {
				hasHighPriority = true
				break
			}
		}
		if !hasHighPriority {
			logger.Info("No high priority issues, stopping iteration")
			break
		}

		// Improve
		improved, err := a.ImproveItems(ctx, currentItems, review, customPrompt)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}
		currentItems = improved
	}

	return currentItems, finalReview, nil
}

// getOutlineSummary returns a summary of the outline for context
func (a *CraftIterationAgent) getOutlineSummary() string {
	if a.outline == nil || len(a.outline.Parts) == 0 {
		return "No outline available"
	}

	var sb strings.Builder
	sb.WriteString("Story Outline Summary:\n")

	for _, part := range a.outline.Parts {
		sb.WriteString(fmt.Sprintf("\nPart: %s\n", part.Title))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", part.Summary))

		for _, vol := range part.Volumes {
			sb.WriteString(fmt.Sprintf("  Volume: %s\n", vol.Title))
			for _, ch := range vol.Chapters {
				sb.WriteString(fmt.Sprintf("    Chapter %s: %s\n", ch.ID, ch.Title))
			}
		}
	}

	return sb.String()
}
