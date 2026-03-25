package agents

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
)

// ComposeGenInput is the input for outline generation
type ComposeGenInput struct {
	Setup     models.StorySetup     `md:"setup"`
	Structure models.StoryStructure `md:"structure"`
}

// ComposeGenOutput is the output for outline generation
type ComposeGenOutput struct {
	Outline models.Outline `md:"outline"`
}

// ComposeRegenInput is the input for outline regeneration
type ComposeRegenInput struct {
	Outline     models.Outline `md:"outline"`
	ElementType string         `md:"element_type"` // "part", "volume", "chapter"
	ElementID   string         `md:"element_id"`
	Suggestions string         `md:"suggestions"`
}

// ComposeRegenOutput is the output for outline regeneration
type ComposeRegenOutput struct {
	Part    *models.Part    `md:"part,omitempty"`
	Volume  *models.Volume  `md:"volume,omitempty"`
	Chapter *models.Chapter `md:"chapter,omitempty"`
}

// ComposeAgent handles AI generation for story outline
// It wraps BaseAgent to provide type-safe methods
type ComposeAgent struct {
	base *BaseAgent
}

// NewComposeAgent creates a new ComposeAgent
func NewComposeAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *ComposeAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "ComposeAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &ComposeAgent{base: base}
}

// SetLanguage sets the output language
func (a *ComposeAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// GenerateOutlineWithStructure generates a story outline with a predefined structure
// This is the type-safe wrapper around BaseAgent.Execute
func (a *ComposeAgent) GenerateOutlineWithStructure(ctx context.Context, setup *models.StorySetup, structure models.StoryStructure, language string) (*models.Outline, error) {
	logger.Section("COMPOSE AGENT - Outline Generation")
	logger.Info("Project: %s", setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes × %d chapters", structure.TargetParts, structure.TargetVolumes, structure.TargetChapters)
	logger.Info("Language: %s", language)

	// Set language
	a.SetLanguage(language)

	var output ComposeGenOutput
	params := InvokeParams{
		Skills:  []string{"compose-gen"},
		Command: "generate a story outline with the specified structure",
	}

	input := ComposeGenInput{
		Setup:     *setup,
		Structure: structure,
	}

	if err := a.base.Execute(ctx, params, input, &output.Outline); err != nil {
		return nil, err
	}

	// Validate the outline structure
	if err := a.validateOutlineStructure(&output.Outline, structure); err != nil {
		return nil, err
	}

	// Validate chapter anchors and state change mapping
	if err := a.validateOutlineChapters(&output.Outline); err != nil {
		return nil, err
	}

	// Assign IDs to all elements using IDManager
	idManager := logic.NewIDManager(&output.Outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	totalChapters := structure.TotalChapters()
	fmt.Printf("✓ Generated outline with %d part(s), %d volume(s) per part, %d chapter(s) per volume\n",
		len(output.Outline.Parts), structure.TargetVolumes, structure.TargetChapters)
	fmt.Printf("  Total: %d chapters\n", totalChapters)
	fmt.Println()

	return &output.Outline, nil
}

// RegeneratePart regenerates a single part with user suggestions
func (a *ComposeAgent) RegeneratePart(ctx context.Context, part *models.Part, outline *models.Outline, setup *models.StorySetup, language, suggestions string) error {
	fmt.Printf("🤖 Regenerating part with AI...\n")

	// Set language
	a.SetLanguage(language)

	// Build context from surrounding parts
	context := a.buildPartContext(part, outline)

	params := InvokeParams{
		Skills:  []string{"compose-regen"},
		Command: "regenerate a part while maintaining continuity",
	}

	input := ComposeRegenInput{
		Outline:     *outline,
		ElementType: "part",
		ElementID:   part.ID,
		Suggestions: suggestions,
	}

	// Add context to the base agent's execution
	// We need to pass context through the input
	inputWithContext := struct {
		ComposeRegenInput
		Context string `md:"context"`
	}{
		ComposeRegenInput: input,
		Context:           context,
	}

	var newPart models.Part
	if err := a.base.Execute(ctx, params, inputWithContext, &newPart); err != nil {
		return err
	}

	// Update part
	part.Title = newPart.Title
	part.Summary = newPart.Summary

	fmt.Printf("✓ Part regenerated: %s\n", part.Title)
	return nil
}

// RegenerateVolume regenerates a single volume with user suggestions
func (a *ComposeAgent) RegenerateVolume(ctx context.Context, volume *models.Volume, outline *models.Outline, setup *models.StorySetup, language, suggestions string) error {
	fmt.Printf("🤖 Regenerating volume with AI...\n")

	// Set language
	a.SetLanguage(language)

	// Build context
	context := a.buildVolumeContext(volume, outline)

	params := InvokeParams{
		Skills:  []string{"compose-regen"},
		Command: "regenerate a volume while maintaining continuity",
	}

	input := ComposeRegenInput{
		Outline:     *outline,
		ElementType: "volume",
		ElementID:   volume.ID,
		Suggestions: suggestions,
	}

	// Add context to the base agent's execution
	inputWithContext := struct {
		ComposeRegenInput
		Context string `md:"context"`
	}{
		ComposeRegenInput: input,
		Context:           context,
	}

	var newVolume models.Volume
	if err := a.base.Execute(ctx, params, inputWithContext, &newVolume); err != nil {
		return err
	}

	// Update volume (including chapters)
	volume.Title = newVolume.Title
	volume.Summary = newVolume.Summary
	if len(newVolume.Chapters) > 0 {
		volume.Chapters = newVolume.Chapters
	}

	fmt.Printf("✓ Volume regenerated: %s (%d chapters)\n", volume.Title, len(volume.Chapters))
	return nil
}

// RegenerateChapter regenerates a single chapter with user suggestions
func (a *ComposeAgent) RegenerateChapter(ctx context.Context, chapter *models.Chapter, outline *models.Outline, setup *models.StorySetup, language, suggestions string) error {
	fmt.Printf("🤖 Regenerating chapter with AI...\n")

	// Set language
	a.SetLanguage(language)

	// Build context
	context := a.buildChapterContext(chapter, outline)

	params := InvokeParams{
		Skills:  []string{"compose-regen"},
		Command: "regenerate a chapter while maintaining continuity",
	}

	input := ComposeRegenInput{
		Outline:     *outline,
		ElementType: "chapter",
		ElementID:   chapter.ID,
		Suggestions: suggestions,
	}

	// Add context to the base agent's execution
	inputWithContext := struct {
		ComposeRegenInput
		Context string `md:"context"`
	}{
		ComposeRegenInput: input,
		Context:           context,
	}

	var newChapter models.Chapter
	if err := a.base.Execute(ctx, params, inputWithContext, &newChapter); err != nil {
		return err
	}

	if err := a.validateChapterOutput(&newChapter); err != nil {
		return err
	}

	// Update chapter
	chapter.Title = newChapter.Title
	chapter.Summary = newChapter.Summary
	chapter.Characters = newChapter.Characters
	chapter.Location = newChapter.Location
	chapter.Events = newChapter.Events
	chapter.Beats = newChapter.Beats
	chapter.OpeningBeat = newChapter.OpeningBeat
	chapter.ClosingBeat = newChapter.ClosingBeat
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
						context.WriteString(fmt.Sprintf("Events: %s\n", a.formatEvents(prevChap.Events)))
						prevBeats := "None"
						if len(prevChap.Beats) > 0 {
							prevBeats = strings.Join(prevChap.Beats, "; ")
						}
						lastBeat := "None"
						if len(prevChap.Beats) > 0 {
							lastBeat = prevChap.Beats[len(prevChap.Beats)-1]
						}
						prevClosing := prevChap.ClosingBeat
						if prevClosing == "" {
							prevClosing = lastBeat
						}
						context.WriteString(fmt.Sprintf("Beats: %s\n", prevBeats))
						context.WriteString(fmt.Sprintf("Final Beat: %s\n", lastBeat))
						context.WriteString(fmt.Sprintf("Closing Beat: %s\n", prevClosing))
						context.WriteString("\n")
					}
					if i > 1 {
						prev2Chap := vol.Chapters[i-2]
						context.WriteString(fmt.Sprintf("Two Chapters Back (%s): %s\n", prev2Chap.ID, prev2Chap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", prev2Chap.Summary))
						context.WriteString(fmt.Sprintf("Key Events: %s\n\n", a.formatEvents(prev2Chap.Events)))
					}

					// Add next chapter context
					if i < len(vol.Chapters)-1 {
						nextChap := vol.Chapters[i+1]
						context.WriteString("=== NEXT CHAPTER (What This Chapter Must Lead To) ===\n")
						context.WriteString(fmt.Sprintf("Next Chapter (%s): %s\n", nextChap.ID, nextChap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", nextChap.Summary))
						nextFirstBeat := nextChap.OpeningBeat
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
					context.WriteString(fmt.Sprintf("Current Events: %s\n", a.formatEvents(chapter.Events)))

					return context.String()
				}
			}
		}
	}

	return context.String()
}

// formatEvents formats events for context display
func (a *ComposeAgent) formatEvents(events []models.Event) string {
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

// validateOutlineStructure validates the outline matches the expected structure
func (a *ComposeAgent) validateOutlineStructure(outline *models.Outline, structure models.StoryStructure) error {
	if len(outline.Parts) != structure.TargetParts {
		logger.Error("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
		return fmt.Errorf("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
	}

	for i, part := range outline.Parts {
		if len(part.Volumes) != structure.TargetVolumes {
			return fmt.Errorf("part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), structure.TargetVolumes)
		}
		for j, volume := range part.Volumes {
			if len(volume.Chapters) != structure.TargetChapters {
				return fmt.Errorf("volume %d.%d has %d chapters, but %d were requested", i+1, j+1, len(volume.Chapters), structure.TargetChapters)
			}
		}
	}

	return nil
}

// validateOutlineChapters validates all chapters in the outline
func (a *ComposeAgent) validateOutlineChapters(outline *models.Outline) error {
	if outline == nil {
		return fmt.Errorf("outline is nil")
	}
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			for chapIdx := range outline.Parts[partIdx].Volumes[volIdx].Chapters {
				chapter := &outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx]
				if err := a.validateChapterOutput(chapter); err != nil {
					return fmt.Errorf("chapter %d.%d.%d invalid: %w", partIdx+1, volIdx+1, chapIdx+1, err)
				}
			}
		}
	}
	return nil
}

// validateChapterOutput validates a chapter's output
func (a *ComposeAgent) validateChapterOutput(chapter *models.Chapter) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	if len(chapter.Beats) == 0 {
		return fmt.Errorf("beats are required")
	}
	if chapter.OpeningBeat == "" {
		return fmt.Errorf("opening_beat is required")
	}
	if chapter.ClosingBeat == "" {
		return fmt.Errorf("closing_beat is required")
	}
	if chapter.OpeningBeat != chapter.Beats[0] {
		return fmt.Errorf("opening_beat must match beats[0]")
	}
	if chapter.ClosingBeat != chapter.Beats[len(chapter.Beats)-1] {
		return fmt.Errorf("closing_beat must match beats[last]")
	}
	if len(chapter.Events) == 0 {
		return fmt.Errorf("events are required")
	}
	return nil
}
