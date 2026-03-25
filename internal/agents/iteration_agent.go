package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/prompts"
)

// Constants for iteration control
const (
	// QualityThreshold is the minimum overall score to stop iteration
	QualityThreshold = 85
	// HighPriority is the priority level for critical issues
	HighPriority = "high"
)

// IterationAgent handles AI-driven outline review and improvement
type IterationAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
}

// NewIterationAgent creates a new IterationAgent
func NewIterationAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *IterationAgent {
	return &IterationAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
	}
}

// ReviewResult wraps prompts.ReviewResult with additional metadata
type ReviewResult struct {
	*prompts.ReviewResult
	Iteration int
}

// ReviewOutline reviews an outline and returns improvement suggestions
func (a *IterationAgent) ReviewOutline(outline *models.Outline, setup *models.StorySetup, iteration int) (*ReviewResult, error) {
	logger.Section("ITERATION AGENT - Review Outline")
	logger.Info("Iteration: %d", iteration)

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Convert outline and setup to JSON strings
	outlineJSON, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal outline: %w", err)
	}
	setupJSON, err := json.MarshalIndent(setup, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal setup: %w", err)
	}

	// Build review data
	data := prompts.BuildOutlineReviewData(string(outlineJSON), string(setupJSON), iteration)

	// Build prompts
	systemPrompt, userPrompt, err := pm.Build(prompts.SkillOutlineReview, "default", data)
	if err != nil {
		logger.Error("Failed to build review prompt: %v", err)
		return nil, fmt.Errorf("failed to build review prompt: %w", err)
	}

	// Log prompts
	logger.Prompt(string(prompts.SkillOutlineReview), "default", systemPrompt, userPrompt)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)

	logger.Info("Sending review request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		logger.Error("AI review request failed: %v", err)
		return nil, fmt.Errorf("AI review request failed: %w", err)
	}

	logger.Info("Received review response (%d tokens used)", resp.Usage.TotalTokens)

	// Parse the JSON response
	var reviewResult prompts.ReviewResult
	if err := json.Unmarshal([]byte(resp.Content), &reviewResult); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &reviewResult); err != nil {
			logger.Error("Failed to parse AI review response as JSON: %v", err)
			logger.Debug("Raw response: %s", resp.Content)
			return nil, fmt.Errorf("failed to parse AI review response as JSON: %w", err)
		}
	}

	// Log review results
	logger.Section("Review Results")
	logger.Info("Overall Score: %d/100", reviewResult.OverallScore)
	logger.Info("Summary: %s", reviewResult.Summary)
	logger.Info("Number of Suggestions: %d", len(reviewResult.Suggestions))

	for i, suggestion := range reviewResult.Suggestions {
		logger.Info("Suggestion %d: [%s] %s (%s) - %s", i+1, suggestion.Priority, suggestion.ID, suggestion.Title, suggestion.Issue)
	}

	return &ReviewResult{
		ReviewResult: &reviewResult,
		Iteration:    iteration,
	}, nil
}

// ApplyImprovements applies improvements based on review suggestions
func (a *IterationAgent) ApplyImprovements(outline *models.Outline, review *ReviewResult, setup *models.StorySetup, language string, concurrency int) error {
	logger.Section("ITERATION AGENT - Apply Improvements")

	// Filter high priority suggestions
	var highPrioritySuggestions []prompts.ReviewSuggestion
	for _, s := range review.Suggestions {
		if s.Priority == HighPriority {
			highPrioritySuggestions = append(highPrioritySuggestions, s)
		}
	}

	if len(highPrioritySuggestions) == 0 {
		logger.Info("No high priority suggestions to apply")
		return nil
	}

	logger.Info("Applying %d high priority improvements", len(highPrioritySuggestions))

	// Group suggestions by volume for efficient regeneration
	volumeGroups := a.groupSuggestionsByVolume(outline, highPrioritySuggestions)

	// Process each volume's suggestions
	for volumeID, suggestions := range volumeGroups {
		if err := a.processVolumeSuggestions(outline, volumeID, suggestions, setup, language); err != nil {
			logger.Error("Failed to process suggestions for volume %s: %v", volumeID, err)
			// Continue with other volumes even if one fails
		}
	}

	return nil
}

// groupSuggestionsByVolume groups suggestions by their parent volume
func (a *IterationAgent) groupSuggestionsByVolume(outline *models.Outline, suggestions []prompts.ReviewSuggestion) map[string][]prompts.ReviewSuggestion {
	groups := make(map[string][]prompts.ReviewSuggestion)
	idManager := logic.NewIDManager(outline)

	for _, suggestion := range suggestions {
		var volumeID string

		switch suggestion.Type {
		case "part":
			// Find which volume contains this part's chapters that need fixing
			part := idManager.GetPartByID(suggestion.ID)
			if part != nil && len(part.Volumes) > 0 {
				// Use the first volume as the target for part-level issues
				volumeID = part.Volumes[0].ID
			}
		case "volume":
			volumeID = suggestion.ID
		case "chapter":
			// Find the parent volume of this chapter
			_, volume, _ := idManager.GetChapterByID(suggestion.ID)
			if volume != nil {
				volumeID = volume.ID
			}
		}

		if volumeID != "" {
			groups[volumeID] = append(groups[volumeID], suggestion)
		}
	}

	return groups
}

// processVolumeSuggestions processes all suggestions for a single volume
func (a *IterationAgent) processVolumeSuggestions(
	outline *models.Outline,
	volumeID string,
	suggestions []prompts.ReviewSuggestion,
	setup *models.StorySetup,
	language string,
) error {
	logger.Info("Processing %d suggestions for volume %s", len(suggestions), volumeID)

	// Find the volume
	idManager := logic.NewIDManager(outline)
	volume, _ := idManager.GetVolumeByID(volumeID)
	if volume == nil {
		return fmt.Errorf("volume %s not found", volumeID)
	}

	// Build combined prompt from all suggestions
	var combinedPrompt strings.Builder
	combinedPrompt.WriteString(fmt.Sprintf("Improve volume '%s' based on the following issues:\n\n", volume.Title))

	for i, suggestion := range suggestions {
		combinedPrompt.WriteString(fmt.Sprintf("%d. [%s] %s (%s): %s\n", i+1, suggestion.Priority, suggestion.ID, suggestion.Title, suggestion.Issue))
		combinedPrompt.WriteString(fmt.Sprintf("   Suggestion: %s\n\n", suggestion.Suggestion))
	}

	// Regenerate the volume with combined suggestions
	return a.regenerateVolumeWithSuggestions(outline, volume, setup, language, combinedPrompt.String(), suggestions)
}

// regeneratePart regenerates a part based on review suggestion
func (a *IterationAgent) regeneratePart(outline *models.Outline, suggestion prompts.ReviewSuggestion, setup *models.StorySetup, language string) error {
	// Find the part
	partIndex := -1
	for i, p := range outline.Parts {
		if p.ID == suggestion.ID {
			partIndex = i
			break
		}
	}
	if partIndex == -1 {
		return fmt.Errorf("part %s not found", suggestion.ID)
	}

	// Create compose agent for regeneration
	composeAgent := NewComposeAgent(a.client, a.config, a.projectLLM)

	// Build context for regeneration
	userPrompt := buildReviewContext(outline, suggestion)

	// Regenerate the part
	ctx := context.Background()
	part := &outline.Parts[partIndex]
	if err := composeAgent.RegeneratePart(ctx, part, outline, setup, language, userPrompt); err != nil {
		return err
	}

	return nil
}

// regenerateVolume regenerates a volume based on review suggestion
func (a *IterationAgent) regenerateVolume(outline *models.Outline, suggestion prompts.ReviewSuggestion, setup *models.StorySetup, language string) error {
	// Find the volume
	ctx := context.Background()
	for i, part := range outline.Parts {
		for j, vol := range part.Volumes {
			if vol.ID == suggestion.ID {
				// Create compose agent for regeneration
				composeAgent := NewComposeAgent(a.client, a.config, a.projectLLM)

				userPrompt := buildReviewContext(outline, suggestion)
				if err := composeAgent.RegenerateVolume(ctx, &vol, outline, setup, language, userPrompt); err != nil {
					return err
				}

				outline.Parts[i].Volumes[j] = vol
				return nil
			}
		}
	}
	return fmt.Errorf("volume %s not found", suggestion.ID)
}

// regenerateChapter regenerates a chapter based on review suggestion
func (a *IterationAgent) regenerateChapter(outline *models.Outline, suggestion prompts.ReviewSuggestion, setup *models.StorySetup, language string) error {
	// Find the chapter
	ctx := context.Background()
	for i, part := range outline.Parts {
		for j, vol := range part.Volumes {
			for k, ch := range vol.Chapters {
				if ch.ID == suggestion.ID {
					// Create compose agent for regeneration
					composeAgent := NewComposeAgent(a.client, a.config, a.projectLLM)

					userPrompt := buildReviewContext(outline, suggestion)
					if err := composeAgent.RegenerateChapter(ctx, &ch, outline, setup, language, userPrompt); err != nil {
						return err
					}

					outline.Parts[i].Volumes[j].Chapters[k] = ch
					return nil
				}
			}
		}
	}
	return fmt.Errorf("chapter %s not found", suggestion.ID)
}

// regenerateVolumeWithSuggestions regenerates a volume with multiple suggestions
func (a *IterationAgent) regenerateVolumeWithSuggestions(
	outline *models.Outline,
	volume *models.Volume,
	setup *models.StorySetup,
	language string,
	combinedPrompt string,
	suggestions []prompts.ReviewSuggestion,
) error {
	logger.Info("Regenerating volume %s with %d suggestions", volume.ID, len(suggestions))

	// Create compose agent for regeneration
	composeAgent := NewComposeAgent(a.client, a.config, a.projectLLM)

	// Build context with volume info and all suggestions
	ctx := context.Background()
	contextStr := a.buildVolumeContextWithSuggestions(volume, outline, suggestions)

	// Regenerate the volume
	if err := composeAgent.RegenerateVolume(ctx, volume, outline, setup, language, combinedPrompt+"\n\n"+contextStr); err != nil {
		return err
	}

	return nil
}

// buildVolumeContextWithSuggestions builds context for volume regeneration with suggestions
func (a *IterationAgent) buildVolumeContextWithSuggestions(volume *models.Volume, outline *models.Outline, suggestions []prompts.ReviewSuggestion) string {
	var context strings.Builder

	// Add part context
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			if vol.ID == volume.ID {
				context.WriteString(fmt.Sprintf("Part: %s\nSummary: %s\n\n", part.Title, part.Summary))
				break
			}
		}
	}

	// Add current volume info
	context.WriteString(fmt.Sprintf("Current Volume: %s\n", volume.Title))
	context.WriteString(fmt.Sprintf("Summary: %s\n", volume.Summary))
	context.WriteString(fmt.Sprintf("Chapters: %d\n\n", len(volume.Chapters)))

	// Add chapter summaries
	context.WriteString("Chapter Overview:\n")
	for _, chap := range volume.Chapters {
		context.WriteString(fmt.Sprintf("- %s: %s\n", chap.ID, chap.Title))
	}
	context.WriteString("\n")

	return context.String()
}

// buildReviewContext builds context string from review suggestion
func buildReviewContext(outline *models.Outline, suggestion prompts.ReviewSuggestion) string {
	var context strings.Builder

	context.WriteString(fmt.Sprintf("Issue: %s\n", suggestion.Issue))
	context.WriteString(fmt.Sprintf("Suggestion: %s\n", suggestion.Suggestion))
	context.WriteString(fmt.Sprintf("Priority: %s\n", suggestion.Priority))

	return context.String()
}

// ShouldContinueIteration determines if iteration should continue
func ShouldContinueIteration(review *ReviewResult, currentIteration, maxIterations int) bool {
	// Stop if we've reached max iterations
	if currentIteration >= maxIterations {
		return false
	}

	// Stop if quality threshold is met
	if review.OverallScore >= QualityThreshold {
		logger.Info("Quality threshold met (score: %d >= %d)", review.OverallScore, QualityThreshold)
		return false
	}

	// Stop if no high priority issues
	hasHighPriority := false
	for _, s := range review.Suggestions {
		if s.Priority == HighPriority {
			hasHighPriority = true
			break
		}
	}

	if !hasHighPriority {
		logger.Info("No high priority issues remaining")
		return false
	}

	return true
}
