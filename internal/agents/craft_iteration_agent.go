package agents

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// CraftReviewResult represents the review result for craft elements
type CraftReviewResult struct {
	OverallScore     float64                 `json:"overall_score"`
	ConsistencyScore float64                 `json:"consistency_score"`
	DepthScore       float64                 `json:"depth_score"`
	OriginalityScore float64                 `json:"originality_score"`
	Suggestions      []CraftReviewSuggestion `json:"suggestions"`
	Iteration        int                     `json:"iteration"`
}

// CraftReviewSuggestion represents a single improvement suggestion
type CraftReviewSuggestion struct {
	ElementName string `json:"element_name"`
	Issue       string `json:"issue"`
	Suggestion  string `json:"suggestion"`
	Priority    string `json:"priority"` // high, medium, low
}

// CraftIterationAgent handles AI-driven element review and improvement
type CraftIterationAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
	setup      *models.StorySetup
	outline    *models.Outline
	language   string
	log        logger.LoggerInterface
	pm         *prompts.PromptManager
}

// NewCraftIterationAgent creates a new CraftIterationAgent
func NewCraftIterationAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline, language string) *CraftIterationAgent {
	return &CraftIterationAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
		setup:      setup,
		outline:    outline,
		language:   language,
		log:        logger.GetLogger(),
		pm:         prompts.NewPromptManager(),
	}
}

// ReviewCharacters reviews characters and returns improvement suggestions
func (a *CraftIterationAgent) ReviewCharacters(characters map[string]*models.Character, iteration int) (*CraftReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Review Characters")
	logger.Info("Iteration: %d, Characters: %d", iteration, len(characters))

	// Convert characters to JSON
	charsJSON, err := json.MarshalIndent(characters, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal characters: %w", err)
	}

	// Build prompt data
	data := map[string]interface{}{
		"element_type": "characters",
		"elements":     string(charsJSON),
		"story_setup":  prompts.StructToPrompt(a.setup, ""),
		"outline":      a.getOutlineSummary(),
		"iteration":    iteration,
		"language":     a.language,
	}

	// Build prompts
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillCharacterReview, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	if options.MaxTokens < 8000 {
		options.MaxTokens = 8000
	}

	logger.Info("Sending character review request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI review request failed: %w", err)
	}

	// Parse review result
	review, err := a.parseReviewResult(resp.Content, iteration)
	if err != nil {
		logger.Error("Failed to parse review result: %v", err)
		logger.Debug("Raw response: %s", resp.Content)
		return nil, fmt.Errorf("failed to parse review result: %w", err)
	}

	logger.Info("Character Review Score: %.1f/100", review.OverallScore)
	logger.Info("Suggestions: %d", len(review.Suggestions))

	return review, nil
}

// ReviewLocations reviews locations and returns improvement suggestions
func (a *CraftIterationAgent) ReviewLocations(locations map[string]*models.Location, iteration int) (*CraftReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Review Locations")
	logger.Info("Iteration: %d, Locations: %d", iteration, len(locations))

	// Convert locations to JSON
	locsJSON, err := json.MarshalIndent(locations, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal locations: %w", err)
	}

	// Build prompt data
	data := map[string]interface{}{
		"element_type": "locations",
		"elements":     string(locsJSON),
		"story_setup":  prompts.StructToPrompt(a.setup, ""),
		"outline":      a.getOutlineSummary(),
		"iteration":    iteration,
		"language":     a.language,
	}

	// Build prompts
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillLocationReview, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	if options.MaxTokens < 8000 {
		options.MaxTokens = 8000
	}

	logger.Info("Sending location review request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI review request failed: %w", err)
	}

	// Parse review result
	review, err := a.parseReviewResult(resp.Content, iteration)
	if err != nil {
		logger.Error("Failed to parse review result: %v", err)
		return nil, fmt.Errorf("failed to parse review result: %w", err)
	}

	logger.Info("Location Review Score: %.1f/100", review.OverallScore)
	logger.Info("Suggestions: %d", len(review.Suggestions))

	return review, nil
}

// ReviewItems reviews items and returns improvement suggestions
func (a *CraftIterationAgent) ReviewItems(items map[string]*models.Item, iteration int) (*CraftReviewResult, error) {
	logger.Section("CRAFT ITERATION AGENT - Review Items")
	logger.Info("Iteration: %d, Items: %d", iteration, len(items))

	// Convert items to JSON
	itemsJSON, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	// Build prompt data
	data := map[string]interface{}{
		"element_type": "items",
		"elements":     string(itemsJSON),
		"story_setup":  prompts.StructToPrompt(a.setup, ""),
		"outline":      a.getOutlineSummary(),
		"iteration":    iteration,
		"language":     a.language,
	}

	// Build prompts
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillItemReview, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	if options.MaxTokens < 8000 {
		options.MaxTokens = 8000
	}

	logger.Info("Sending item review request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI review request failed: %w", err)
	}

	// Parse review result
	review, err := a.parseReviewResult(resp.Content, iteration)
	if err != nil {
		logger.Error("Failed to parse review result: %v", err)
		return nil, fmt.Errorf("failed to parse review result: %w", err)
	}

	logger.Info("Item Review Score: %.1f/100", review.OverallScore)
	logger.Info("Suggestions: %d", len(review.Suggestions))

	return review, nil
}

// ImproveCharacters applies improvements to characters based on review
func (a *CraftIterationAgent) ImproveCharacters(characters map[string]*models.Character, review *CraftReviewResult, customPrompt string) (map[string]*models.Character, error) {
	logger.Section("CRAFT ITERATION AGENT - Improve Characters")
	logger.Info("Processing %d suggestions", len(review.Suggestions))

	// Sort suggestions by priority
	sortedSuggestions := a.sortSuggestionsByPriority(review.Suggestions)

	// Filter high priority suggestions
	highPrioritySuggestions := []CraftReviewSuggestion{}
	for _, s := range sortedSuggestions {
		if s.Priority == "high" {
			highPrioritySuggestions = append(highPrioritySuggestions, s)
		}
	}

	if len(highPrioritySuggestions) == 0 {
		logger.Info("No high priority suggestions to apply")
		return characters, nil
	}

	logger.Info("Applying %d high priority improvements", len(highPrioritySuggestions))

	// Convert characters to JSON
	charsJSON, err := json.MarshalIndent(characters, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal characters: %w", err)
	}

	// Build improvement prompt
	data := map[string]interface{}{
		"elements":      string(charsJSON),
		"suggestions":   highPrioritySuggestions,
		"story_setup":   prompts.StructToPrompt(a.setup, ""),
		"outline":       a.getOutlineSummary(),
		"custom_prompt": customPrompt,
		"language":      a.language,
	}

	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillCharacterImprovement, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build improvement prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	if options.MaxTokens < 12000 {
		options.MaxTokens = 12000
	}

	logger.Info("Sending character improvement request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI improvement request failed: %w", err)
	}

	// Parse improved characters
	var improved map[string]*models.Character
	if err := json.Unmarshal([]byte(resp.Content), &improved); err != nil {
		// Try to extract JSON from markdown
		jsonStr := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(jsonStr), &improved); err != nil {
			return nil, fmt.Errorf("failed to parse improved characters: %w", err)
		}
	}

	logger.Info("Successfully improved %d characters", len(improved))
	return improved, nil
}

// ImproveLocations applies improvements to locations based on review
func (a *CraftIterationAgent) ImproveLocations(locations map[string]*models.Location, review *CraftReviewResult, customPrompt string) (map[string]*models.Location, error) {
	logger.Section("CRAFT ITERATION AGENT - Improve Locations")
	logger.Info("Processing %d suggestions", len(review.Suggestions))

	// Sort suggestions by priority
	sortedSuggestions := a.sortSuggestionsByPriority(review.Suggestions)

	// Filter high priority suggestions
	highPrioritySuggestions := []CraftReviewSuggestion{}
	for _, s := range sortedSuggestions {
		if s.Priority == "high" {
			highPrioritySuggestions = append(highPrioritySuggestions, s)
		}
	}

	if len(highPrioritySuggestions) == 0 {
		logger.Info("No high priority suggestions to apply")
		return locations, nil
	}

	logger.Info("Applying %d high priority improvements", len(highPrioritySuggestions))

	// Convert locations to JSON
	locsJSON, err := json.MarshalIndent(locations, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal locations: %w", err)
	}

	// Build improvement prompt
	data := map[string]interface{}{
		"elements":      string(locsJSON),
		"suggestions":   highPrioritySuggestions,
		"story_setup":   prompts.StructToPrompt(a.setup, ""),
		"outline":       a.getOutlineSummary(),
		"custom_prompt": customPrompt,
		"language":      a.language,
	}

	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillLocationImprovement, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build improvement prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	if options.MaxTokens < 12000 {
		options.MaxTokens = 12000
	}

	logger.Info("Sending location improvement request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI improvement request failed: %w", err)
	}

	// Parse improved locations
	var improved map[string]*models.Location
	if err := json.Unmarshal([]byte(resp.Content), &improved); err != nil {
		jsonStr := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(jsonStr), &improved); err != nil {
			return nil, fmt.Errorf("failed to parse improved locations: %w", err)
		}
	}

	logger.Info("Successfully improved %d locations", len(improved))
	return improved, nil
}

// ImproveItems applies improvements to items based on review
func (a *CraftIterationAgent) ImproveItems(items map[string]*models.Item, review *CraftReviewResult, customPrompt string) (map[string]*models.Item, error) {
	logger.Section("CRAFT ITERATION AGENT - Improve Items")
	logger.Info("Processing %d suggestions", len(review.Suggestions))

	// Sort suggestions by priority
	sortedSuggestions := a.sortSuggestionsByPriority(review.Suggestions)

	// Filter high priority suggestions
	highPrioritySuggestions := []CraftReviewSuggestion{}
	for _, s := range sortedSuggestions {
		if s.Priority == "high" {
			highPrioritySuggestions = append(highPrioritySuggestions, s)
		}
	}

	if len(highPrioritySuggestions) == 0 {
		logger.Info("No high priority suggestions to apply")
		return items, nil
	}

	logger.Info("Applying %d high priority improvements", len(highPrioritySuggestions))

	// Convert items to JSON
	itemsJSON, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	// Build improvement prompt
	data := map[string]interface{}{
		"elements":      string(itemsJSON),
		"suggestions":   highPrioritySuggestions,
		"story_setup":   prompts.StructToPrompt(a.setup, ""),
		"outline":       a.getOutlineSummary(),
		"custom_prompt": customPrompt,
		"language":      a.language,
	}

	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillItemImprovement, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build improvement prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	if options.MaxTokens < 12000 {
		options.MaxTokens = 12000
	}

	logger.Info("Sending item improvement request to AI...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI improvement request failed: %w", err)
	}

	// Parse improved items
	var improved map[string]*models.Item
	if err := json.Unmarshal([]byte(resp.Content), &improved); err != nil {
		jsonStr := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(jsonStr), &improved); err != nil {
			return nil, fmt.Errorf("failed to parse improved items: %w", err)
		}
	}

	logger.Info("Successfully improved %d items", len(improved))
	return improved, nil
}

// parseReviewResult parses the AI response into a CraftReviewResult
func (a *CraftIterationAgent) parseReviewResult(content string, iteration int) (*CraftReviewResult, error) {
	// Try to extract JSON from markdown if needed
	jsonStr := extractJSONFromMarkdown(content)

	var result CraftReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal review result: %w", err)
	}

	result.Iteration = iteration
	return &result, nil
}

// sortSuggestionsByPriority sorts suggestions by priority (high first)
func (a *CraftIterationAgent) sortSuggestionsByPriority(suggestions []CraftReviewSuggestion) []CraftReviewSuggestion {
	sorted := make([]CraftReviewSuggestion, len(suggestions))
	copy(sorted, suggestions)

	sort.Slice(sorted, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return priorityOrder[sorted[i].Priority] < priorityOrder[sorted[j].Priority]
	})

	return sorted
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

// ShouldContinueCraftIteration determines if we should continue iterating
func ShouldContinueCraftIteration(review *CraftReviewResult, iteration int, maxIterations int) bool {
	// Stop if we've reached max iterations
	if iteration >= maxIterations {
		logger.Info("Reached maximum iterations (%d)", maxIterations)
		return false
	}

	// Stop if overall score is good enough (85+)
	if review.OverallScore >= 85 {
		logger.Info("Element quality is good (score: %.1f), stopping iteration", review.OverallScore)
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
