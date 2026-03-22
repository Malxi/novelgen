package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/prompts"
)

// ComposeAgent handles AI generation for story outline
type ComposeAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
}

// NewComposeAgent creates a new ComposeAgent
func NewComposeAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *ComposeAgent {
	return &ComposeAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
	}
}

// GenerateOutlineWithStructure generates a story outline with a predefined structure
func (a *ComposeAgent) GenerateOutlineWithStructure(setup *models.StorySetup, structure models.StoryStructure, language string) (*models.Outline, error) {
	logger.Section("COMPOSE AGENT - Outline Generation")
	logger.Info("Project: %s", setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes × %d chapters", structure.TargetParts, structure.TargetVolumes, structure.TargetChapters)
	logger.Info("Language: %s", language)

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts using the prompt manager
	data := prompts.BuildOutlineGenData(structure, setup, language)
	data["language"] = language

	systemPrompt, userPrompt, err := pm.Build(prompts.SkillOutlineGen, "with_structure", data)
	if err != nil {
		logger.Error("Failed to build prompt: %v", err)
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Log prompts
	logger.Prompt(string(prompts.SkillOutlineGen), "with_structure", systemPrompt, userPrompt)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.config.GetChatOptions(a.projectLLM)

	logger.Info("Sending request to AI (this may take a while)...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		logger.Error("AI request failed: %v", err)
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	logger.Info("Received response (%d tokens used)", resp.Usage.TotalTokens)

	// Parse the JSON response
	var outline models.Outline
	if err := json.Unmarshal([]byte(resp.Content), &outline); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		logger.Debug("Extracted JSON from markdown: %s", content)
		if err := json.Unmarshal([]byte(content), &outline); err != nil {
			logger.Error("Failed to parse AI response as JSON: %v", err)
			logger.Debug("Raw response: %s", resp.Content)
			return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, resp.Content)
		}
	}

	// Validate the outline structure
	if len(outline.Parts) != structure.TargetParts {
		logger.Error("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
		return nil, fmt.Errorf("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
	}

	for i, part := range outline.Parts {
		if len(part.Volumes) != structure.TargetVolumes {
			return nil, fmt.Errorf("part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), structure.TargetVolumes)
		}
		for j, volume := range part.Volumes {
			if len(volume.Chapters) != structure.TargetChapters {
				return nil, fmt.Errorf("volume %d.%d has %d chapters, but %d were requested", i+1, j+1, len(volume.Chapters), structure.TargetChapters)
			}
		}
	}

	totalChapters := structure.TotalChapters()

	// Assign IDs to all elements using IDManager
	idManager := logic.NewIDManager(&outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	fmt.Printf("✓ Generated outline with %d part(s), %d volume(s) per part, %d chapter(s) per volume\n",
		len(outline.Parts), structure.TargetVolumes, structure.TargetChapters)
	fmt.Printf("  Total: %d chapters\n", totalChapters)
	fmt.Println()

	return &outline, nil
}

// RegeneratePart regenerates a single part with user suggestions
func (a *ComposeAgent) RegeneratePart(part *models.Part, outline *models.Outline, setup *models.StorySetup, language, userPrompt string) error {
	fmt.Printf("🤖 Regenerating part with AI...\n")

	// Build context from surrounding parts
	context := a.buildPartContext(part, outline)

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts
	data := prompts.BuildOutlineRegenData("part", part.Title, context, setup, language, userPrompt)
	systemPrompt, userPromptText, err := pm.Build(prompts.SkillOutlineRegen, "part", data)
	if err != nil {
		return fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call AI
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPromptText},
	}

	options := a.config.GetChatOptions(a.projectLLM)

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	// Parse response
	var newPart models.Part
	if err := json.Unmarshal([]byte(resp.Content), &newPart); err != nil {
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &newPart); err != nil {
			return fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	// Update part
	part.Title = newPart.Title
	part.Summary = newPart.Summary

	fmt.Printf("✓ Part regenerated: %s\n", part.Title)
	return nil
}

// RegenerateVolume regenerates a single volume with user suggestions
func (a *ComposeAgent) RegenerateVolume(volume *models.Volume, outline *models.Outline, setup *models.StorySetup, language, userPrompt string) error {
	fmt.Printf("🤖 Regenerating volume with AI...\n")

	// Build context
	context := a.buildVolumeContext(volume, outline)

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts
	data := prompts.BuildOutlineRegenData("volume", volume.Title, context, setup, language, userPrompt)
	systemPrompt, userPromptText, err := pm.Build(prompts.SkillOutlineRegen, "volume", data)
	if err != nil {
		return fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call AI
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPromptText},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	// Use smaller max tokens for volume regeneration
	options.MaxTokens = 10000

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	// Parse response
	var newVolume models.Volume
	if err := json.Unmarshal([]byte(resp.Content), &newVolume); err != nil {
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &newVolume); err != nil {
			return fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	// Update volume (keep chapters)
	volume.Title = newVolume.Title
	volume.Summary = newVolume.Summary

	fmt.Printf("✓ Volume regenerated: %s\n", volume.Title)
	return nil
}

// RegenerateChapter regenerates a single chapter with user suggestions
func (a *ComposeAgent) RegenerateChapter(chapter *models.Chapter, outline *models.Outline, setup *models.StorySetup, language, userPrompt string) error {
	fmt.Printf("🤖 Regenerating chapter with AI...\n")

	// Build context
	context := a.buildChapterContext(chapter, outline)

	// Create prompt manager
	pm := prompts.NewPromptManager()

	// Build prompts
	data := prompts.BuildOutlineRegenData("chapter", chapter.Title, context, setup, language, userPrompt)
	systemPrompt, userPromptText, err := pm.Build(prompts.SkillOutlineRegen, "chapter", data)
	if err != nil {
		return fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call AI
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPromptText},
	}

	options := a.config.GetChatOptions(a.projectLLM)
	// Use smaller max tokens for chapter regeneration
	options.MaxTokens = 5000

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	// Parse response
	var newChapter models.Chapter
	if err := json.Unmarshal([]byte(resp.Content), &newChapter); err != nil {
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &newChapter); err != nil {
			return fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	// Update chapter
	chapter.Title = newChapter.Title
	chapter.Summary = newChapter.Summary
	chapter.Characters = newChapter.Characters
	chapter.Location = newChapter.Location
	chapter.Events = newChapter.Events
	chapter.Beats = newChapter.Beats
	chapter.Conflict = newChapter.Conflict
	chapter.Pacing = newChapter.Pacing

	fmt.Printf("✓ Chapter regenerated: %s\n", chapter.Title)
	return nil
}

// buildPartContext builds context for part regeneration
func (a *ComposeAgent) buildPartContext(part *models.Part, outline *models.Outline) string {
	var context strings.Builder

	// Find part index
	partIdx := -1
	for i, p := range outline.Parts {
		if p.ID == part.ID {
			partIdx = i
			break
		}
	}

	if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		context.WriteString(fmt.Sprintf("Previous Part (%s): %s\nSummary: %s\n\n",
			prevPart.ID, prevPart.Title, prevPart.Summary))
	}

	if partIdx < len(outline.Parts)-1 {
		nextPart := outline.Parts[partIdx+1]
		context.WriteString(fmt.Sprintf("Next Part (%s): %s\nSummary: %s\n\n",
			nextPart.ID, nextPart.Title, nextPart.Summary))
	}

	return context.String()
}

// buildVolumeContext builds context for volume regeneration
func (a *ComposeAgent) buildVolumeContext(volume *models.Volume, outline *models.Outline) string {
	var context strings.Builder

	// Find volume in outline
	for _, part := range outline.Parts {
		for i, vol := range part.Volumes {
			if vol.ID == volume.ID {
				// Add part context
				context.WriteString(fmt.Sprintf("Part: %s\nSummary: %s\n\n", part.Title, part.Summary))

				// Add sibling volumes
				if i > 0 {
					prevVol := part.Volumes[i-1]
					context.WriteString(fmt.Sprintf("Previous Volume (%s): %s\nSummary: %s\n\n",
						prevVol.ID, prevVol.Title, prevVol.Summary))
				}
				if i < len(part.Volumes)-1 {
					nextVol := part.Volumes[i+1]
					context.WriteString(fmt.Sprintf("Next Volume (%s): %s\nSummary: %s\n\n",
						nextVol.ID, nextVol.Title, nextVol.Summary))
				}
				return context.String()
			}
		}
	}

	return context.String()
}

// buildChapterContext builds context for chapter regeneration
func (a *ComposeAgent) buildChapterContext(chapter *models.Chapter, outline *models.Outline) string {
	var context strings.Builder

	// Find chapter in outline
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i, chap := range vol.Chapters {
				if chap.ID == chapter.ID {
					// Add part and volume context
					context.WriteString(fmt.Sprintf("=== CURRENT LOCATION IN STORY ===\n"))
					context.WriteString(fmt.Sprintf("Part: %s\nPart Summary: %s\n\n", part.Title, part.Summary))
					context.WriteString(fmt.Sprintf("Volume: %s\nVolume Summary: %s\n\n", vol.Title, vol.Summary))

					// Add previous chapters context (up to 2 chapters back for better continuity)
					context.WriteString("=== PREVIOUS CHAPTERS (For Continuity) ===\n")
					if i > 0 {
						prevChap := vol.Chapters[i-1]
						context.WriteString(fmt.Sprintf("Previous Chapter (%s): %s\n", prevChap.ID, prevChap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", prevChap.Summary))
						context.WriteString(fmt.Sprintf("Events: %s\n", formatEvents(prevChap.Events)))
						prevBeats := "None"
						if len(prevChap.Beats) > 0 {
							prevBeats = strings.Join(prevChap.Beats, "; ")
						}
						lastBeat := "None"
						if len(prevChap.Beats) > 0 {
							lastBeat = prevChap.Beats[len(prevChap.Beats)-1]
						}
						context.WriteString(fmt.Sprintf("Beats: %s\n", prevBeats))
						context.WriteString(fmt.Sprintf("Final Beat: %s\n\n", lastBeat))
					}
					if i > 1 {
						prev2Chap := vol.Chapters[i-2]
						context.WriteString(fmt.Sprintf("Two Chapters Back (%s): %s\n", prev2Chap.ID, prev2Chap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", prev2Chap.Summary))
						context.WriteString(fmt.Sprintf("Key Events: %s\n\n", formatEvents(prev2Chap.Events)))
					}

					// Add next chapter context
					if i < len(vol.Chapters)-1 {
						nextChap := vol.Chapters[i+1]
						context.WriteString("=== NEXT CHAPTER (What This Chapter Must Lead To) ===\n")
						context.WriteString(fmt.Sprintf("Next Chapter (%s): %s\n", nextChap.ID, nextChap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", nextChap.Summary))
						nextFirstBeat := "None"
						if len(nextChap.Beats) > 0 {
							nextFirstBeat = nextChap.Beats[0]
						}
						context.WriteString(fmt.Sprintf("Opening Beat: %s\n", nextFirstBeat))
						context.WriteString(fmt.Sprintf("This chapter MUST set up: %s\n\n", nextChap.Summary))
					}

					// Add current chapter to regenerate
					context.WriteString("=== CURRENT CHAPTER TO REGENERATE ===\n")
					context.WriteString(fmt.Sprintf("Chapter Title: %s\n", chapter.Title))
					context.WriteString(fmt.Sprintf("Current Summary: %s\n", chapter.Summary))
					context.WriteString(fmt.Sprintf("Current Events: %s\n", formatEvents(chapter.Events)))

					return context.String()
				}
			}
		}
	}

	return context.String()
}

// formatEvents formats events for context display
func formatEvents(events []models.Event) string {
	if len(events) == 0 {
		return "None"
	}
	var parts []string
	for _, e := range events {
		part := fmt.Sprintf("[%s: %s - %s]", e.Type, e.Subject, e.Change)
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}
