package agents

import (
	"encoding/json"
	"fmt"
	"sort"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
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

	// Convert outline to JSON
	outlineJSON, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal outline: %v", err)
		return nil, fmt.Errorf("failed to marshal outline: %w", err)
	}

	// Build story setup data
	setupData := map[string]interface{}{
		"project_name": setup.ProjectName,
		"genres":       setup.Genres,
		"premise":      setup.Premise,
		"theme":        setup.Theme,
		"rules":        setup.Rules,
		"tone":         setup.Tone,
		"tense":        setup.Tense,
		"pov":          setup.POVStyle,
	}

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts
	data := prompts.BuildOutlineReviewData(string(outlineJSON), setupData, iteration)
	systemPrompt, userPrompt, err := pm.Build(prompts.SkillOutlineReview, "default", data)
	if err != nil {
		logger.Error("Failed to build prompt: %v", err)
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

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

	// Parse review result
	reviewResult, err := prompts.ParseReviewResult(resp.Content)
	if err != nil {
		logger.Error("Failed to parse review result: %v", err)
		logger.Debug("Raw response: %s", resp.Content)
		return nil, fmt.Errorf("failed to parse review result: %w", err)
	}

	// Log review summary
	logger.Section("Review Summary")
	logger.Info("Overall Score: %.1f/100", reviewResult.OverallScore)
	logger.Info("Logic Score: %.1f/100", reviewResult.LogicScore)
	logger.Info("Engagement Score: %.1f/100", reviewResult.EngagementScore)
	logger.Info("Pacing Score: %.1f/100", reviewResult.PacingScore)
	logger.Info("Coherence Score: %.1f/100", reviewResult.CoherenceScore)
	logger.Info("Suggestions: %d", len(reviewResult.Suggestions))

	// Log high priority suggestions
	highPriorityCount := 0
	for _, s := range reviewResult.Suggestions {
		if s.Priority == "high" {
			highPriorityCount++
		}
	}
	if highPriorityCount > 0 {
		logger.Warn("High priority issues: %d", highPriorityCount)
	}

	return &ReviewResult{
		ReviewResult: reviewResult,
		Iteration:    iteration,
	}, nil
}

// ApplyImprovements applies review suggestions to improve the outline
func (a *IterationAgent) ApplyImprovements(outline *models.Outline, review *ReviewResult, setup *models.StorySetup, language string) error {
	logger.Section("ITERATION AGENT - Apply Improvements")
	logger.Info("Processing %d suggestions", len(review.Suggestions))

	// Sort suggestions by priority (high first)
	sortedSuggestions := make([]prompts.ReviewSuggestion, len(review.Suggestions))
	copy(sortedSuggestions, review.Suggestions)
	sort.Slice(sortedSuggestions, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return priorityOrder[sortedSuggestions[i].Priority] < priorityOrder[sortedSuggestions[j].Priority]
	})

	// Group suggestions by type
	partSuggestions := []prompts.ReviewSuggestion{}
	volumeSuggestions := []prompts.ReviewSuggestion{}
	chapterSuggestions := []prompts.ReviewSuggestion{}

	for _, s := range sortedSuggestions {
		switch s.Type {
		case "part":
			partSuggestions = append(partSuggestions, s)
		case "volume":
			volumeSuggestions = append(volumeSuggestions, s)
		case "chapter":
			chapterSuggestions = append(chapterSuggestions, s)
		}
	}

	logger.Info("Part suggestions: %d", len(partSuggestions))
	logger.Info("Volume suggestions: %d", len(volumeSuggestions))
	logger.Info("Chapter suggestions: %d", len(chapterSuggestions))

	// Apply improvements using regeneration
	// For now, we apply high priority suggestions only
	appliedCount := 0
	for _, s := range sortedSuggestions {
		if s.Priority != "high" {
			continue // Skip medium/low priority for now
		}

		logger.Info("Applying suggestion for %s %s: %s", s.Type, s.ID, s.Suggestion)

		var err error
		switch s.Type {
		case "part":
			err = a.regeneratePart(outline, s, setup, language)
		case "volume":
			err = a.regenerateVolume(outline, s, setup, language)
		case "chapter":
			err = a.regenerateChapter(outline, s, setup, language)
		}

		if err != nil {
			logger.Error("Failed to apply suggestion for %s %s: %v", s.Type, s.ID, err)
			continue
		}
		appliedCount++
	}

	logger.Info("Applied %d high-priority improvements", appliedCount)
	return nil
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
	userPrompt := buildReviewContext(suggestion)

	// Regenerate the part
	part := &outline.Parts[partIndex]
	if err := composeAgent.RegeneratePart(part, outline, setup, language, userPrompt); err != nil {
		return err
	}

	return nil
}

// regenerateVolume regenerates a volume based on review suggestion
func (a *IterationAgent) regenerateVolume(outline *models.Outline, suggestion prompts.ReviewSuggestion, setup *models.StorySetup, language string) error {
	// Find the volume
	for i, part := range outline.Parts {
		for j, vol := range part.Volumes {
			if vol.ID == suggestion.ID {
				// Create compose agent for regeneration
				composeAgent := NewComposeAgent(a.client, a.config, a.projectLLM)

				userPrompt := buildReviewContext(suggestion)
				if err := composeAgent.RegenerateVolume(&vol, outline, setup, language, userPrompt); err != nil {
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
	for i, part := range outline.Parts {
		for j, vol := range part.Volumes {
			for k, ch := range vol.Chapters {
				if ch.ID == suggestion.ID {
					// Create compose agent for regeneration
					composeAgent := NewComposeAgent(a.client, a.config, a.projectLLM)

					userPrompt := buildReviewContext(suggestion)
					if err := composeAgent.RegenerateChapter(&ch, outline, setup, language, userPrompt); err != nil {
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

// buildReviewContext builds context string from review suggestion
func buildReviewContext(suggestion prompts.ReviewSuggestion) string {
	return fmt.Sprintf("Issue: %s\nSuggestion: %s\nPriority: %s",
		suggestion.Issue, suggestion.Suggestion, suggestion.Priority)
}

// ShouldContinueIteration determines if we should continue iterating
func ShouldContinueIteration(review *ReviewResult, iteration int, maxIterations int) bool {
	// Stop if we've reached max iterations
	if iteration >= maxIterations {
		logger.Info("Reached maximum iterations (%d)", maxIterations)
		return false
	}

	// Stop if overall score is good enough (>= 85)
	if review.OverallScore >= 85 {
		logger.Info("Outline quality is good (score: %.1f), stopping iteration", review.OverallScore)
		return false
	}

	// Stop if no high priority suggestions
	hasHighPriority := false
	for _, s := range review.Suggestions {
		if s.Priority == "high" {
			hasHighPriority = true
			break
		}
	}
	if !hasHighPriority {
		logger.Info("No high priority issues remaining, stopping iteration")
		return false
	}

	return true
}
