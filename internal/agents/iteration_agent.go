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

	// Convert outline to JSON
	outlineJSON, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal outline: %v", err)
		return nil, fmt.Errorf("failed to marshal outline: %w", err)
	}

	// Use StructToPrompt to convert setup to formatted string
	setupPrompt := prompts.StructToPrompt(setup, "")

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts
	data := prompts.BuildOutlineReviewData(string(outlineJSON), setupPrompt, iteration)
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
// Now operates at volume level - all suggestions for a volume are processed together
func (a *IterationAgent) ApplyImprovements(outline *models.Outline, review *ReviewResult, setup *models.StorySetup, language string, concurrency int) error {
	logger.Section("ITERATION AGENT - Apply Improvements")
	logger.Info("Processing %d suggestions", len(review.Suggestions))
	if concurrency <= 0 {
		concurrency = 3 // Default concurrency
	}

	// Normalize IDs and filter high priority suggestions
	highPrioritySugs := []prompts.ReviewSuggestion{}
	for _, s := range review.Suggestions {
		if s.Priority == HighPriority {
			s.ID = normalizeSuggestionID(outline, s)
			highPrioritySugs = append(highPrioritySugs, s)
		}
	}

	if len(highPrioritySugs) == 0 {
		logger.Info("No high-priority suggestions to apply")
		return nil
	}

	logger.Info("Applying %d high-priority suggestions at volume level", len(highPrioritySugs))

	// Group suggestions by volume
	volumeSuggestions := a.groupSuggestionsByVolume(outline, highPrioritySugs)
	logger.Info("Found suggestions for %d volumes", len(volumeSuggestions))

	// Process each volume concurrently
	executor := logic.NewExecutor(concurrency)
	appliedCount := 0

	for volumeID, suggestions := range volumeSuggestions {
		logger.Info("Volume %s: %d suggestions", volumeID, len(suggestions))

		taskData := &volumeTaskData{
			outline:     outline,
			volumeID:    volumeID,
			suggestions: suggestions,
			setup:       setup,
			language:    language,
			agent:       a,
		}

		task := &logic.Task{
			ID:      volumeID,
			Data:    taskData,
			Execute: executeVolumeTask,
		}
		executor.AddTask(task)
	}

	// Execute all volume tasks concurrently
	ctx := context.Background()
	results := executor.Execute(ctx)

	// Process results
	for _, result := range results {
		if result.Error != nil {
			logger.Error("Failed to apply suggestions for volume %s: %v", result.TaskID, result.Error)
		} else {
			appliedCount++
		}
	}

	logger.Info("Applied improvements to %d volumes", appliedCount)
	return nil
}

// groupSuggestionsByVolume groups suggestions by their parent volume
func (a *IterationAgent) groupSuggestionsByVolume(outline *models.Outline, suggestions []prompts.ReviewSuggestion) map[string][]prompts.ReviewSuggestion {
	groups := make(map[string][]prompts.ReviewSuggestion)

	for _, s := range suggestions {
		targetType := determineTargetTypeFromID(outline, s.ID)
		var volumeID string

		switch targetType {
		case "part":
			// Part-level suggestions apply to all volumes in the part
			for _, part := range outline.Parts {
				if part.ID == s.ID {
					for _, vol := range part.Volumes {
						groups[vol.ID] = append(groups[vol.ID], s)
					}
					break
				}
			}
			continue
		case "volume":
			volumeID = s.ID
		case "chapter":
			// Find parent volume for chapter
			for _, part := range outline.Parts {
				for _, vol := range part.Volumes {
					for _, chap := range vol.Chapters {
						if chap.ID == s.ID {
							volumeID = vol.ID
							break
						}
					}
					if volumeID != "" {
						break
					}
				}
				if volumeID != "" {
					break
				}
			}
		}

		if volumeID != "" {
			groups[volumeID] = append(groups[volumeID], s)
		}
	}

	return groups
}

// volumeTaskData holds data for volume-level execution
type volumeTaskData struct {
	outline     *models.Outline
	volumeID    string
	suggestions []prompts.ReviewSuggestion
	setup       *models.StorySetup
	language    string
	agent       *IterationAgent
}

// executeVolumeTask executes regeneration for a volume with all its suggestions
func executeVolumeTask(ctx context.Context, data interface{}) error {
	taskData, ok := data.(*volumeTaskData)
	if !ok {
		return fmt.Errorf("invalid task data type")
	}

	logger.Info("Regenerating volume %s with %d suggestions", taskData.volumeID, len(taskData.suggestions))

	// Build combined user prompt from all suggestions
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Improvements needed for volume %s:\n\n", taskData.volumeID))
	for i, s := range taskData.suggestions {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, s.Type, s.Issue))
		sb.WriteString(fmt.Sprintf("   Suggestion: %s\n\n", s.Suggestion))
	}
	combinedPrompt := sb.String()

	// Find the volume
	for i, part := range taskData.outline.Parts {
		for j, vol := range part.Volumes {
			if vol.ID == taskData.volumeID {
				// Regenerate volume with combined suggestions
				if err := taskData.agent.regenerateVolumeWithSuggestions(
					taskData.outline,
					&taskData.outline.Parts[i].Volumes[j],
					taskData.setup,
					taskData.language,
					combinedPrompt,
					taskData.suggestions,
				); err != nil {
					return fmt.Errorf("failed to regenerate volume %s: %w", taskData.volumeID, err)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("volume %s not found", taskData.volumeID)
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

				userPrompt := buildReviewContext(outline, suggestion)
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

					userPrompt := buildReviewContext(outline, suggestion)
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
	context := a.buildVolumeContextWithSuggestions(volume, outline, suggestions)

	// Regenerate the volume
	if err := composeAgent.RegenerateVolume(volume, outline, setup, language, combinedPrompt+"\n\n"+context); err != nil {
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
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Target Type: %s\n", suggestion.Type))
	if suggestion.ID != "" {
		sb.WriteString(fmt.Sprintf("Target ID: %s\n", suggestion.ID))
	}
	if suggestion.Title != "" {
		sb.WriteString(fmt.Sprintf("Target Title: %s\n", suggestion.Title))
	}
	sb.WriteString(fmt.Sprintf("Issue: %s\n", suggestion.Issue))
	sb.WriteString(fmt.Sprintf("Suggestion: %s\n", suggestion.Suggestion))
	sb.WriteString(fmt.Sprintf("Priority: %s\n", suggestion.Priority))

	if outline != nil && strings.EqualFold(strings.TrimSpace(suggestion.Type), "chapter") {
		if closing, opening := lookupChapterHandoff(outline, suggestion.ID); closing != "" || opening != "" {
			sb.WriteString("Continuity Handoff:\n")
			if closing != "" {
				sb.WriteString(fmt.Sprintf("- Previous closing_beat: %s\n", closing))
			}
			if opening != "" {
				sb.WriteString(fmt.Sprintf("- Next opening_beat: %s\n", opening))
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

func lookupChapterHandoff(outline *models.Outline, chapterID string) (string, string) {
	if outline == nil || chapterID == "" {
		return "", ""
	}
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			chapters := outline.Parts[partIdx].Volumes[volIdx].Chapters
			for chapIdx := range chapters {
				if chapters[chapIdx].ID != chapterID {
					continue
				}
				var previousClosing string
				if chapIdx > 0 {
					prev := chapters[chapIdx-1]
					previousClosing = prev.ClosingBeat
					if previousClosing == "" && len(prev.Beats) > 0 {
						previousClosing = prev.Beats[len(prev.Beats)-1]
					}
				}

				var nextOpening string
				if chapIdx < len(chapters)-1 {
					next := chapters[chapIdx+1]
					nextOpening = next.OpeningBeat
					if nextOpening == "" && len(next.Beats) > 0 {
						nextOpening = next.Beats[0]
					}
				}

				return previousClosing, nextOpening
			}
		}
	}
	return "", ""
}

func normalizeSuggestionID(outline *models.Outline, suggestion prompts.ReviewSuggestion) string {
	id := strings.TrimSpace(suggestion.ID)
	if id == "" || outline == nil {
		return id
	}

	// Determine target type from ID format using IDManager
	targetType := determineTargetTypeFromID(outline, id)

	// Already new-style ID
	upper := strings.ToUpper(id)
	if strings.HasPrefix(upper, "P") {
		switch targetType {
		case "part":
			if strings.Contains(upper, "-V") {
				parts := strings.Split(upper, "-V")
				return parts[0]
			}
			if strings.Contains(upper, "-C") {
				parts := strings.Split(upper, "-C")
				return parts[0]
			}
		case "volume":
			if strings.Contains(upper, "-C") {
				parts := strings.Split(upper, "-C")
				return parts[0]
			}
		}
		return upper
	}

	// Legacy format "1_1_1" or "1_1" or "1"
	parts := strings.Split(id, "_")
	idManager := logic.NewIDManager(outline)
	getNum := func(idx int) string {
		if idx < 0 || idx >= len(parts) {
			return ""
		}
		return extractDigits(parts[idx])
	}

	switch targetType {
	case "part":
		if n := getNum(0); n != "" {
			if resolved, err := idManager.ResolvePartID(n); err == nil {
				return resolved
			}
		}
	case "volume":
		if len(parts) >= 2 {
			partNum := getNum(0)
			volNum := getNum(1)
			if partNum != "" && volNum != "" {
				if resolved, err := idManager.ResolveVolumeID(volNum, partNum); err == nil {
					return resolved
				}
			}
		}
	case "chapter":
		if len(parts) >= 3 {
			partNum := getNum(0)
			volNum := getNum(1)
			chapNum := getNum(2)
			if partNum != "" && volNum != "" && chapNum != "" {
				if resolved, err := idManager.ResolveChapterID(chapNum, partNum, volNum); err == nil {
					return resolved
				}
			}
		}
	}

	return id
}

func extractDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// determineTargetTypeFromID determines the target type (part/volume/chapter) from the ID format
// It uses IDManager to intelligently resolve IDs based on what actually exists in the outline
// ID formats:
//   - Part: "P1", "1"
//   - Volume: "P1-V1", "1_1"
//   - Chapter: "P1-V1-C4", "1_1_4", "C4" (ambiguous, will try to resolve)
func determineTargetTypeFromID(outline *models.Outline, id string) string {
	id = strings.TrimSpace(id)
	if id == "" || outline == nil {
		return ""
	}

	idManager := logic.NewIDManager(outline)
	upper := strings.ToUpper(id)

	// Try to find as chapter first (most specific)
	// Handle formats: P1-V1-C4, P1-V1-C4, C4
	if strings.Contains(upper, "-C") || strings.HasPrefix(upper, "C") {
		// Full chapter ID: P1-V1-C4
		if strings.Contains(upper, "-C") && strings.Contains(upper, "-V") {
			if _, _, _, err := idManager.ParseChapterID(id); err == nil {
				if chapter := findChapterInOutline(outline, id); chapter != nil {
					return "chapter"
				}
			}
		}
		// Ambiguous format like "C4" - try to resolve to full chapter ID
		if strings.HasPrefix(upper, "C") {
			chapNum := strings.TrimPrefix(upper, "C")
			// Try to find chapter in the outline
			for _, part := range outline.Parts {
				for _, vol := range part.Volumes {
					for _, chap := range vol.Chapters {
						if strings.HasSuffix(strings.ToUpper(chap.ID), "-C"+chapNum) {
							return "chapter"
						}
					}
				}
			}
		}
	}

	// Try to find as volume: P1-V1
	if strings.Contains(upper, "-V") {
		if _, _, err := idManager.ParseVolumeID(id); err == nil {
			if volume := findVolumeInOutline(outline, id); volume != nil {
				return "volume"
			}
		}
	}

	// Try to find as part: P1
	if strings.HasPrefix(upper, "P") {
		if _, err := idManager.ParsePartID(id); err == nil {
			if part := findPartInOutline(outline, id); part != nil {
				return "part"
			}
		}
	}

	// Legacy format: 1_1_4 (chapter), 1_1 (volume), 1 (part)
	parts := strings.Split(id, "_")
	switch len(parts) {
	case 3:
		// Try as chapter
		if resolved, err := idManager.ResolveChapterID(parts[2], parts[0], parts[1]); err == nil {
			if chapter := findChapterInOutline(outline, resolved); chapter != nil {
				return "chapter"
			}
		}
	case 2:
		// Try as volume
		if resolved, err := idManager.ResolveVolumeID(parts[1], parts[0]); err == nil {
			if volume := findVolumeInOutline(outline, resolved); volume != nil {
				return "volume"
			}
		}
	case 1:
		// Could be part number or chapter number
		// Try as part first
		if resolved, err := idManager.ResolvePartID(parts[0]); err == nil {
			if part := findPartInOutline(outline, resolved); part != nil {
				return "part"
			}
		}
		// Try as chapter number (e.g., "2" might mean "P1-V1-C2")
		for _, part := range outline.Parts {
			for _, vol := range part.Volumes {
				for _, chap := range vol.Chapters {
					if strings.HasSuffix(chap.ID, "-C"+parts[0]) || strings.HasSuffix(chap.ID, "_"+parts[0]) {
						return "chapter"
					}
				}
			}
		}
	}

	return ""
}

// findPartInOutline finds a part by ID
func findPartInOutline(outline *models.Outline, id string) *models.Part {
	for i := range outline.Parts {
		if outline.Parts[i].ID == id {
			return &outline.Parts[i]
		}
	}
	return nil
}

// findVolumeInOutline finds a volume by ID
func findVolumeInOutline(outline *models.Outline, id string) *models.Volume {
	for _, part := range outline.Parts {
		for i := range part.Volumes {
			if part.Volumes[i].ID == id {
				return &part.Volumes[i]
			}
		}
	}
	return nil
}

// findChapterInOutline finds a chapter by ID
func findChapterInOutline(outline *models.Outline, id string) *models.Chapter {
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i := range vol.Chapters {
				if vol.Chapters[i].ID == id {
					return &vol.Chapters[i]
				}
			}
		}
	}
	return nil
}

// ShouldContinueIteration determines if we should continue iterating
func ShouldContinueIteration(review *ReviewResult, iteration int, maxIterations int) bool {
	// Stop if we've reached max iterations
	if iteration >= maxIterations {
		logger.Info("Reached maximum iterations (%d)", maxIterations)
		return false
	}

	// Stop if overall score is good enough
	if review.OverallScore >= QualityThreshold {
		logger.Info("Outline quality is good (score: %.1f), stopping iteration", review.OverallScore)
		return false
	}

	// Stop if no high priority suggestions
	hasHighPriority := false
	for _, s := range review.Suggestions {
		if s.Priority == HighPriority {
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
