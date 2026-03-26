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
	Context     string         `md:"context,omitempty"` // Surrounding context for continuity
}

// ComposeRegenOutput is the output for outline regeneration
type ComposeRegenOutput struct {
	Part    *models.Part    `md:"part,omitempty"`
	Volume  *models.Volume  `md:"volume,omitempty"`
	Chapter *models.Chapter `md:"chapter,omitempty"`
}

// ComposeReviewInput is the input for outline review
type ComposeReviewInput struct {
	ExistingOutline models.Outline `md:"existing_outline"`
}

// ComposeReviewOutput is the output for outline review
type ComposeReviewOutput struct {
	Result models.ReviewResult `md:"result"`
}

// ComposeImproveInput is the input for outline improvement
type ComposeImproveInput struct {
	ExistingOutline models.Outline      `md:"existing_outline"`
	ReviewResult    models.ReviewResult `md:"review_result,omitempty"` // Optional: for improvement based on review
}

// ComposeImproveOutput is the output for outline improvement
type ComposeImproveOutput struct {
	Outline models.Outline `md:"outline"`
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

// Generate creates a story outline from setup and structure
// This is the type-safe wrapper around BaseAgent.Execute
func (a *ComposeAgent) Generate(ctx context.Context, input ComposeGenInput) (ComposeGenOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Generation")
	logger.Info("Project: %s", input.Setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes × %d chapters",
		input.Structure.TargetParts, input.Structure.TargetVolumes, input.Structure.TargetChapters)
	logger.Info("Language: %s", a.base.language)

	var output ComposeGenOutput
	params := InvokeParams{
		Skills:  []string{"compose-gen"},
		Command: "generate a story outline with the specified structure",
	}

	if err := a.base.Execute(ctx, params, input, &output.Outline); err != nil {
		return ComposeGenOutput{}, err
	}

	// Validate the outline structure
	if err := a.validateOutlineStructure(&output.Outline, input.Structure); err != nil {
		return ComposeGenOutput{}, err
	}

	// Validate chapter anchors and state change mapping
	if err := a.validateOutlineChapters(&output.Outline); err != nil {
		return ComposeGenOutput{}, err
	}

	// Assign IDs to all elements using IDManager
	idManager := logic.NewIDManager(&output.Outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	totalChapters := input.Structure.TotalChapters()
	logger.Info("Generated outline with %d part(s), %d volume(s) per part, %d chapter(s) per volume",
		len(output.Outline.Parts), input.Structure.TargetVolumes, input.Structure.TargetChapters)
	logger.Info("Total: %d chapters", totalChapters)

	return output, nil
}

// Regenerate regenerates a story outline element (part, volume, or chapter)
func (a *ComposeAgent) Regenerate(ctx context.Context, input ComposeRegenInput) (ComposeRegenOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Regeneration")
	logger.Info("Element Type: %s", input.ElementType)
	logger.Info("Element ID: %s", input.ElementID)
	logger.Info("Language: %s", a.base.language)

	params := InvokeParams{
		Skills:  []string{"compose-regen"},
		Command: fmt.Sprintf("regenerate a %s while maintaining continuity", input.ElementType),
	}

	var output ComposeRegenOutput

	switch input.ElementType {
	case "part":
		var part models.Part
		if err := a.base.Execute(ctx, params, input, &part); err != nil {
			return ComposeRegenOutput{}, err
		}
		output.Part = &part
		logger.Info("✓ Part regenerated: %s", part.Title)

	case "volume":
		var volume models.Volume
		if err := a.base.Execute(ctx, params, input, &volume); err != nil {
			return ComposeRegenOutput{}, err
		}
		output.Volume = &volume
		logger.Info("✓ Volume regenerated: %s (%d chapters)", volume.Title, len(volume.Chapters))

	case "chapter":
		var chapter models.Chapter
		if err := a.base.Execute(ctx, params, input, &chapter); err != nil {
			return ComposeRegenOutput{}, err
		}
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeRegenOutput{}, fmt.Errorf("validation failed: %w", err)
		}
		output.Chapter = &chapter
		logger.Info("✓ Chapter regenerated: %s", chapter.Title)

	default:
		return ComposeRegenOutput{}, fmt.Errorf("invalid element type: %s", input.ElementType)
	}

	return output, nil
}

// Review reviews an existing outline and provides improvement suggestions
func (a *ComposeAgent) Review(ctx context.Context, input ComposeReviewInput) (ComposeReviewOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Review")
	logger.Info("Language: %s", a.base.language)

	var output ComposeReviewOutput
	params := InvokeParams{
		Skills:  []string{"compose-review"},
		Command: "review the story outline and provide improvement suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Result); err != nil {
		return ComposeReviewOutput{}, err
	}

	// Log result
	logger.Section("Outline Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	for _, dim := range output.Result.Dimensions {
		logger.Info("%s: %.1f/%.0f", dim.Name, dim.Score, dim.Max)
	}
	logger.Info("Summary: %s", output.Result.Summary)
	logger.Info("Strengths: %d items", len(output.Result.Strengths))
	logger.Info("Suggestions: %d items", len(output.Result.Suggestions))

	return output, nil
}

// Improve improves an existing outline
func (a *ComposeAgent) Improve(ctx context.Context, input ComposeImproveInput) (ComposeImproveOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Improvement")
	logger.Info("Language: %s", a.base.language)

	var output ComposeImproveOutput
	params := InvokeParams{
		Skills:  []string{"compose-improve"},
		Command: "improve the story outline",
	}

	if err := a.base.Execute(ctx, params, input, &output.Outline); err != nil {
		return ComposeImproveOutput{}, err
	}

	// Validate the improved outline
	if err := a.validateOutlineChapters(&output.Outline); err != nil {
		return ComposeImproveOutput{}, err
	}

	return output, nil
}

// Iterate runs the review-improvement loop for outline
func (a *ComposeAgent) Iterate(ctx context.Context, outline *models.Outline, maxIterations int, qualityThreshold float64) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT - Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)

	currentOutline := *outline
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review current outline
		reviewInput := ComposeReviewInput{ExistingOutline: currentOutline}
		reviewOutput, err := a.Review(ctx, reviewInput)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}

		finalReview = &reviewOutput.Result

		// Check if quality meets threshold
		if reviewOutput.Result.OverallScore >= qualityThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
			break
		}

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}

		// Improve the outline with review feedback
		improveInput := ComposeImproveInput{
			ExistingOutline: currentOutline,
			ReviewResult:    reviewOutput.Result,
		}
		improveOutput, err := a.Improve(ctx, improveInput)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}

		currentOutline = improveOutput.Outline
		logger.Info("✓ Outline improved, continuing to next iteration")
	}

	return &currentOutline, finalReview, nil
}

// BuildPartContext builds context for part regeneration
func (a *ComposeAgent) BuildPartContext(part *models.Part, outline *models.Outline) string {
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

// BuildVolumeContext builds context for volume regeneration
func (a *ComposeAgent) BuildVolumeContext(volume *models.Volume, outline *models.Outline) string {
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

// BuildChapterContext builds context for chapter regeneration
func (a *ComposeAgent) BuildChapterContext(chapter *models.Chapter, outline *models.Outline) string {
	var context strings.Builder

	// Find chapter in outline
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i, chap := range vol.Chapters {
				if chap.ID == chapter.ID {
					// Add part and volume context
					context.WriteString("=== CURRENT LOCATION IN STORY ===\n")
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
