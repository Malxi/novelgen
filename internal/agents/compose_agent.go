package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
)

// ComposeGenInput is the input for outline generation
type ComposeGenInput struct {
	Setup     models.StorySetup     `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

// ComposeGenOutput is the output for outline generation
type ComposeGenOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Generated complete story outline with parts, volumes, chapters"`
}

// ComposeRegenInput is the input for outline regeneration
type ComposeRegenInput struct {
	Outline     models.Outline `json:"outline" md:"outline" desc:"Existing outline to regenerate from"`
	ElementType string         `json:"element_type" md:"element_type" desc:"Type of element to regenerate: part, volume, or chapter"`
	ElementID   string         `json:"element_id" md:"element_id" desc:"ID of the element to regenerate"`
	Suggestions string         `json:"suggestions" md:"suggestions" desc:"User suggestions for regeneration"`
	Context     string         `json:"context,omitempty" md:"context,omitempty" desc:"Surrounding context for continuity"`
}

// ComposeRegenOutput is the output for outline regeneration
type ComposeRegenOutput struct {
	Part    *models.Part    `json:"part,omitempty" md:"part,omitempty" desc:"Regenerated part (if element_type is part)"`
	Volume  *models.Volume  `json:"volume,omitempty" md:"volume,omitempty" desc:"Regenerated volume (if element_type is volume)"`
	Chapter *models.Chapter `json:"chapter,omitempty" md:"chapter,omitempty" desc:"Regenerated chapter (if element_type is chapter)"`
}

// ComposeReviewInput is the input for outline review
type ComposeReviewInput struct {
	ExistingOutline models.Outline    `json:"existing_outline" md:"existing_outline" desc:"Existing outline to review"`
	Setup           models.StorySetup `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	UserPrompt      string            `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

// ComposeReviewOutput is the output for outline review
type ComposeReviewOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeImproveInput is the input for outline improvement
type ComposeImproveInput struct {
	ExistingOutline models.Outline      `json:"existing_outline" md:"existing_outline" desc:"Existing outline to improve"`
	ReviewResult    models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup           models.StorySetup   `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
}

// ComposeImproveOutput is the output for outline improvement
type ComposeImproveOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Improved story outline"`
}

// ComposeSkeletonInput is the input for generating outline skeleton (parts and volumes only)
type ComposeSkeletonInput struct {
	Setup     models.StorySetup     `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

// ComposeSkeletonOutput is the output for outline skeleton generation
type ComposeSkeletonOutput struct {
	Parts []models.Part `json:"parts" md:"parts" desc:"Generated story parts with volumes"`
}

// ComposeChaptersInput is the input for generating chapters for a volume
type ComposeChaptersInput struct {
	Setup          models.StorySetup `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Part           models.Part       `json:"part" md:"part" desc:"Current part information"`
	Volume         models.Volume     `json:"volume" md:"volume" desc:"Current volume information"`
	VolumeIndex    int               `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes   int               `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
	ChaptersPerVol int               `json:"chapters_per_volume" md:"chapters_per_volume" desc:"Number of chapters per volume"`
	PreviousVolume *models.Volume    `json:"previous_volume,omitempty" md:"previous_volume,omitempty" desc:"Previous volume for continuity"`
	OutlineContext string            `json:"outline_context" md:"outline_context" desc:"Context from previously generated outline"`
}

// ComposeChaptersOutput is the output for chapter generation
type ComposeChaptersOutput struct {
	Chapters []models.Chapter `json:"chapters" md:"chapters" desc:"Generated chapters for the volume"`
}

// ComposeReviewVolumeInput is the input for reviewing a specific volume
type ComposeReviewVolumeInput struct {
	Outline      models.Outline `json:"outline" md:"outline" desc:"Complete story outline"`
	Part         models.Part    `json:"part" md:"part" desc:"Current part information"`
	Volume       models.Volume  `json:"volume" md:"volume" desc:"Volume to review"`
	VolumeIndex  int            `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes int            `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
}

// ComposeReviewVolumeOutput is the output for volume review
type ComposeReviewVolumeOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeImproveVolumeInput is the input for improving a specific volume
type ComposeImproveVolumeInput struct {
	Outline      models.Outline      `json:"outline" md:"outline" desc:"Complete story outline"`
	Part         models.Part         `json:"part" md:"part" desc:"Current part information"`
	Volume       models.Volume       `json:"volume" md:"volume" desc:"Volume to improve"`
	ReviewResult models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result for improvement guidance"`
	UserPrompt   string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup        models.StorySetup   `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
}

// ComposeImproveVolumeOutput is the output for volume improvement
type ComposeImproveVolumeOutput struct {
	Volume models.Volume `json:"volume" md:"volume" desc:"Improved volume with chapters"`
}

// ImproveProgress tracks the progress of hierarchical improvement
type ImproveProgress struct {
	Iteration        int                 `json:"iteration"`          // Current iteration number
	TotalIterations  int                 `json:"total_iterations"`   // Total iterations planned
	CurrentVolumeIdx int                 `json:"current_volume_idx"` // Index of next volume to improve (0-based)
	TotalVolumes     int                 `json:"total_volumes"`      // Total volumes to improve
	CompletedVolumes []string            `json:"completed_volumes"`  // List of completed volume IDs
	Outline          models.Outline      `json:"outline"`            // Current state of outline
	ReviewResult     models.ReviewResult `json:"review_result"`      // Review result for current iteration
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
func (a *ComposeAgent) Iterate(ctx context.Context, outline *models.Outline, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT - Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}

	currentOutline := *outline
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review current outline
		reviewInput := ComposeReviewInput{ExistingOutline: currentOutline}
		if setup != nil {
			reviewInput.Setup = *setup
		}
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
		shouldImprove := !scoreMeetsThreshold || forceImprove
		if !shouldImprove {
			break
		}

		// Improve the outline with review feedback
		improveInput := ComposeImproveInput{
			ExistingOutline: currentOutline,
			ReviewResult:    reviewOutput.Result,
			UserPrompt:      userPrompt,
		}
		if setup != nil {
			improveInput.Setup = *setup
		}
		improveOutput, err := a.Improve(ctx, improveInput)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}

		currentOutline = improveOutput.Outline
		logger.Info("✓ Outline improved based on review suggestions")

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
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

// ImproveVolume improves a specific volume based on review feedback
func (a *ComposeAgent) ImproveVolume(ctx context.Context, input ComposeImproveVolumeInput) (ComposeImproveVolumeOutput, error) {
	logger.Section("COMPOSE AGENT - Volume Improvement")
	logger.Info("Volume: %s", input.Volume.Title)
	logger.Info("Language: %s", a.base.language)

	var output ComposeImproveVolumeOutput
	params := InvokeParams{
		Skills:  []string{"compose-improve-volume"},
		Command: "improve the chapters in this volume based on review feedback",
	}

	if err := a.base.Execute(ctx, params, input, &output.Volume); err != nil {
		return ComposeImproveVolumeOutput{}, err
	}

	// Validate the improved volume
	if len(output.Volume.Chapters) == 0 {
		return ComposeImproveVolumeOutput{}, fmt.Errorf("improved volume has no chapters")
	}

	// Validate each chapter
	for i, chapter := range output.Volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}

	// Preserve original volume ID and chapter IDs to maintain consistency
	originalVolumeID := input.Volume.ID
	output.Volume.ID = originalVolumeID

	// Preserve original chapter IDs
	for i := range output.Volume.Chapters {
		if i < len(input.Volume.Chapters) {
			output.Volume.Chapters[i].ID = input.Volume.Chapters[i].ID
		}
	}

	logger.Info("✓ Volume improved: %s (%d chapters)", output.Volume.Title, len(output.Volume.Chapters))

	return output, nil
}

// IterateHierarchical runs the hierarchical review-improvement loop with checkpoint support
// 1. Review entire outline
// 2. Identify volumes that need improvement
// 3. Improve each volume individually
// Supports resuming from checkpoint if interrupted
func (a *ComposeAgent) IterateHierarchical(ctx context.Context, outline *models.Outline, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT - Hierarchical Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}
	logger.Info("This will review the entire outline, then improve volumes individually")

	currentOutline := *outline
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Step 1: Review entire outline
		logger.Section("Step 1: Reviewing Entire Outline")
		reviewInput := ComposeReviewInput{
			ExistingOutline: currentOutline,
			UserPrompt:      userPrompt,
		}
		if setup != nil {
			reviewInput.Setup = *setup
		}
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
		shouldImprove := !scoreMeetsThreshold || forceImprove
		if !shouldImprove {
			break
		}

		// Step 2: Identify volumes that need improvement from suggestions
		logger.Section("Step 2: Improving Volumes")
		volumesToImprove := a.identifyVolumesToImprove(&reviewOutput.Result)

		if len(volumesToImprove) == 0 {
			logger.Info("No specific volumes identified for improvement, improving all volumes")
			// Improve all volumes
			for partIdx := range currentOutline.Parts {
				for volIdx := range currentOutline.Parts[partIdx].Volumes {
					volumesToImprove = append(volumesToImprove, [2]int{partIdx, volIdx})
				}
			}
		}

		// Step 3: Improve each identified volume with checkpoint support
		// Note: userPrompt is passed to review, not to improve (review generates suggestions based on userPrompt)
		improvedOutline, err := a.improveVolumesWithCheckpoint(ctx, &currentOutline, volumesToImprove, &reviewOutput.Result, i, maxIterations, setup)
		if err != nil {
			return nil, nil, fmt.Errorf("iteration %d failed: %w", i, err)
		}
		currentOutline = *improvedOutline

		logger.Info("✓ All volumes improved, continuing to next iteration")

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
	}

	return &currentOutline, finalReview, nil
}

// improveVolumesWithCheckpoint improves volumes with checkpoint/resume support
func (a *ComposeAgent) improveVolumesWithCheckpoint(ctx context.Context, outline *models.Outline, volumesToImprove [][2]int, reviewResult *models.ReviewResult, currentIteration int, totalIterations int, setup *models.StorySetup) (*models.Outline, error) {
	currentOutline := *outline
	progressPath := "story/compose/outline_improve_progress.json"

	// Try to load existing progress
	var progress *ImproveProgress
	if _, err := os.Stat(progressPath); err == nil {
		loadedProgress, err := a.loadImproveProgress(progressPath)
		if err == nil && loadedProgress.Iteration == currentIteration {
			logger.Info("📂 Found existing progress for iteration %d, resuming...", currentIteration)
			progress = loadedProgress
			currentOutline = progress.Outline
			logger.Info("✓ Resumed from checkpoint: %d/%d volumes completed", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
	}

	// Initialize progress if not resuming
	if progress == nil {
		progress = &ImproveProgress{
			Iteration:        currentIteration,
			TotalIterations:  totalIterations,
			CurrentVolumeIdx: 0,
			TotalVolumes:     len(volumesToImprove),
			CompletedVolumes: []string{},
			Outline:          currentOutline,
			ReviewResult:     *reviewResult,
		}
		// Save initial progress
		if err := a.saveImproveProgress(progress, progressPath); err != nil {
			logger.Warn("Failed to save initial progress: %v", err)
		}
	}

	// Improve remaining volumes
	for idx := progress.CurrentVolumeIdx; idx < len(volumesToImprove); idx++ {
		indices := volumesToImprove[idx]
		partIdx, volIdx := indices[0], indices[1]
		part := &currentOutline.Parts[partIdx]
		volume := &part.Volumes[volIdx]

		logger.Info("Improving Volume %d.%d: %s (%d/%d)", partIdx+1, volIdx+1, volume.Title, idx+1, len(volumesToImprove))

		// Filter suggestions for this volume
		volumeReview := a.filterReviewForVolume(reviewResult, volume.ID)

		improveInput := ComposeImproveVolumeInput{
			Outline:      currentOutline,
			Part:         *part,
			Volume:       *volume,
			ReviewResult: volumeReview,
			// Note: UserPrompt is not passed here - it's used in review to generate suggestions
		}
		if setup != nil {
			improveInput.Setup = *setup
		}

		improveOutput, err := a.ImproveVolume(ctx, improveInput)
		if err != nil {
			// Save progress before returning error
			progress.CurrentVolumeIdx = idx
			progress.Outline = currentOutline
			if saveErr := a.saveImproveProgress(progress, progressPath); saveErr != nil {
				logger.Warn("Failed to save progress on error: %v", saveErr)
			}
			return nil, fmt.Errorf("failed to improve volume %d.%d: %w", partIdx+1, volIdx+1, err)
		}

		// Update the volume in the outline
		part.Volumes[volIdx] = improveOutput.Volume
		progress.CompletedVolumes = append(progress.CompletedVolumes, volume.ID)
		progress.CurrentVolumeIdx = idx + 1
		progress.Outline = currentOutline

		// Save progress after each volume
		if err := a.saveImproveProgress(progress, progressPath); err != nil {
			logger.Warn("Failed to save progress: %v", err)
		} else {
			logger.Info("💾 Progress saved (%d/%d volumes completed)", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
	}

	// Remove progress file on successful completion of this iteration
	os.Remove(progressPath)
	logger.Info("✓ Iteration %d complete! Progress file removed.", currentIteration)

	return &currentOutline, nil
}

// loadImproveProgress loads improvement progress from file
func (a *ComposeAgent) loadImproveProgress(path string) (*ImproveProgress, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read progress file: %w", err)
	}

	var progress ImproveProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to parse progress file: %w", err)
	}

	return &progress, nil
}

// saveImproveProgress saves improvement progress to file
func (a *ComposeAgent) saveImproveProgress(progress *ImproveProgress, path string) error {
	if progress == nil {
		return fmt.Errorf("cannot save nil progress")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	// Write to temporary file first, then rename for atomic operation
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename progress file: %w", err)
	}

	return nil
}

// identifyVolumesToImprove identifies which volumes need improvement based on suggestions
func (a *ComposeAgent) identifyVolumesToImprove(review *models.ReviewResult) [][2]int {
	volumeMap := make(map[string][2]int)

	// Parse suggestion target IDs to identify volumes
	for _, suggestion := range review.Suggestions {
		// Target ID format: P1-V1-C1 or P1-V1
		parts := strings.Split(suggestion.TargetID, "-")
		if len(parts) >= 2 {
			// Extract volume ID (e.g., "P1-V1")
			volumeID := parts[0] + "-" + parts[1]
			if _, exists := volumeMap[volumeID]; !exists {
				// Parse part and volume indices from ID
				var partIdx, volIdx int
				n, err := fmt.Sscanf(volumeID, "P%d-V%d", &partIdx, &volIdx)
				if err != nil || n != 2 {
					logger.Warn("Failed to parse volume ID '%s', skipping", volumeID)
					continue
				}
				// Validate indices are positive
				if partIdx <= 0 || volIdx <= 0 {
					logger.Warn("Invalid volume ID '%s' (part=%d, vol=%d), skipping", volumeID, partIdx, volIdx)
					continue
				}
				volumeMap[volumeID] = [2]int{partIdx - 1, volIdx - 1} // Convert to 0-based
			}
		}
	}

	// Convert map to slice
	var result [][2]int
	for _, indices := range volumeMap {
		result = append(result, indices)
	}

	return result
}

// filterReviewForVolume filters review results for a specific volume
func (a *ComposeAgent) filterReviewForVolume(review *models.ReviewResult, volumeID string) models.ReviewResult {
	filtered := models.ReviewResult{
		OverallScore: review.OverallScore,
		Dimensions:   review.Dimensions,
		Summary:      review.Summary,
		Strengths:    review.Strengths,
		Weaknesses:   review.Weaknesses,
	}

	// Filter suggestions for this volume
	for _, suggestion := range review.Suggestions {
		if strings.HasPrefix(suggestion.TargetID, volumeID) {
			filtered.Suggestions = append(filtered.Suggestions, suggestion)
		}
	}

	return filtered
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
						if len(prevChap.GetBeats()) > 0 {
							prevBeats = strings.Join(prevChap.GetBeats(), "; ")
						}
						lastBeat := "None"
						if len(prevChap.GetBeats()) > 0 {
							lastBeat = prevChap.GetBeats()[len(prevChap.GetBeats())-1]
						}
						prevClosing := getClosingBeat(prevChap)
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
						nextFirstBeat := getOpeningBeat(nextChap)
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
	if len(chapter.GetBeats()) == 0 {
		return fmt.Errorf("beats are required")
	}
	if len(chapter.Events) == 0 {
		return fmt.Errorf("events are required")
	}
	return nil
}

// GenerateSkeleton generates the outline skeleton (parts and volumes without chapters)
func (a *ComposeAgent) GenerateSkeleton(ctx context.Context, input ComposeSkeletonInput) (ComposeSkeletonOutput, error) {
	logger.Section("COMPOSE AGENT - Generating Outline Skeleton")
	logger.Info("Project: %s", input.Setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes",
		input.Structure.TargetParts, input.Structure.TargetVolumes)
	logger.Info("Language: %s", a.base.language)

	var output ComposeSkeletonOutput
	params := InvokeParams{
		Skills:  []string{"compose-skeleton"},
		Command: "generate the story outline skeleton with parts and volumes only",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return ComposeSkeletonOutput{}, err
	}

	// Validate the skeleton structure
	if len(output.Parts) != input.Structure.TargetParts {
		return ComposeSkeletonOutput{}, fmt.Errorf("AI generated %d parts, but %d were requested",
			len(output.Parts), input.Structure.TargetParts)
	}

	for i, part := range output.Parts {
		if len(part.Volumes) != input.Structure.TargetVolumes {
			return ComposeSkeletonOutput{}, fmt.Errorf("part %d has %d volumes, but %d were requested",
				i+1, len(part.Volumes), input.Structure.TargetVolumes)
		}
	}

	logger.Info("Generated skeleton with %d part(s), %d volume(s) per part",
		len(output.Parts), input.Structure.TargetVolumes)

	return output, nil
}

// GenerateChaptersForVolume generates chapters for a specific volume
func (a *ComposeAgent) GenerateChaptersForVolume(ctx context.Context, input ComposeChaptersInput) (ComposeChaptersOutput, error) {
	logger.Section("COMPOSE AGENT - Generating Chapters for Volume")
	logger.Info("Volume: %s (%d/%d)", input.Volume.Title, input.VolumeIndex, input.TotalVolumes)
	logger.Info("Chapters to generate: %d", input.ChaptersPerVol)
	logger.Info("Language: %s", a.base.language)

	var output ComposeChaptersOutput
	params := InvokeParams{
		Skills:  []string{"compose-chapters"},
		Command: fmt.Sprintf("generate %d chapters for this volume with proper continuity", input.ChaptersPerVol),
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return ComposeChaptersOutput{}, err
	}

	// Validate chapter count
	if len(output.Chapters) != input.ChaptersPerVol {
		return ComposeChaptersOutput{}, fmt.Errorf("AI generated %d chapters, but %d were requested",
			len(output.Chapters), input.ChaptersPerVol)
	}

	// Validate each chapter
	for i, chapter := range output.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeChaptersOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}

	logger.Info("Generated %d chapters for volume", len(output.Chapters))

	return output, nil
}

// GenerateOutlineHierarchical generates a complete outline using hierarchical approach
// First generates skeleton (parts/volumes), then generates chapters for each volume
func (a *ComposeAgent) GenerateOutlineHierarchical(ctx context.Context, setup models.StorySetup, structure models.StoryStructure) (*models.Outline, error) {
	logger.Section("COMPOSE AGENT - Hierarchical Outline Generation")
	logger.Info("This will generate the outline in two phases:")
	logger.Info("  Phase 1: Generate skeleton (parts and volumes)")
	logger.Info("  Phase 2: Generate chapters for each volume")

	// Phase 1: Generate skeleton
	skeletonInput := ComposeSkeletonInput{
		Setup:     setup,
		Structure: structure,
	}
	skeletonOutput, err := a.GenerateSkeleton(ctx, skeletonInput)
	if err != nil {
		return nil, fmt.Errorf("failed to generate skeleton: %w", err)
	}

	// Phase 2: Generate chapters for each volume
	outline := &models.Outline{
		Parts: skeletonOutput.Parts,
	}

	return a.GenerateChaptersHierarchical(ctx, setup, structure, outline, nil)
}

// GenerateChaptersHierarchical generates chapters for each volume in hierarchical mode
// Supports incremental generation with save callback
func (a *ComposeAgent) GenerateChaptersHierarchical(ctx context.Context, setup models.StorySetup, structure models.StoryStructure, outline *models.Outline, onVolumeComplete func(*models.Outline, int, int, int)) (*models.Outline, error) {
	totalVolumes := structure.TargetParts * structure.TargetVolumes
	volumeCount := 0

	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			volumeCount++
			volume := &outline.Parts[partIdx].Volumes[volIdx]

			// Skip if already has chapters (resumed generation)
			if len(volume.Chapters) > 0 {
				logger.Info("✓ Volume %d.%d: %s - already has %d chapters (skipping)",
					partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))
				continue
			}

			logger.Info("Generating chapters for Volume %d.%d: %s",
				partIdx+1, volIdx+1, volume.Title)

			// Build context from previous volume (for continuity)
			var outlineContext string
			if partIdx > 0 || volIdx > 0 {
				outlineContext = a.buildHierarchicalContext(outline, partIdx, volIdx)
			}

			// Get previous volume for continuity
			var previousVolume *models.Volume
			if volIdx > 0 {
				previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
			} else if partIdx > 0 {
				prevPart := outline.Parts[partIdx-1]
				if len(prevPart.Volumes) > 0 {
					previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
				}
			}

			chaptersInput := ComposeChaptersInput{
				Setup:          setup,
				Part:           outline.Parts[partIdx],
				Volume:         *volume,
				VolumeIndex:    volumeCount,
				TotalVolumes:   totalVolumes,
				ChaptersPerVol: structure.TargetChapters,
				PreviousVolume: previousVolume,
				OutlineContext: outlineContext,
			}

			chaptersOutput, err := a.GenerateChaptersForVolume(ctx, chaptersInput)
			if err != nil {
				return outline, fmt.Errorf("failed to generate chapters for volume %d.%d: %w",
					partIdx+1, volIdx+1, err)
			}

			// Assign chapters to volume
			volume.Chapters = chaptersOutput.Chapters

			logger.Info("✓ Volume %d.%d: %s - %d chapters generated",
				partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))

			// Call save callback if provided
			if onVolumeComplete != nil {
				onVolumeComplete(outline, partIdx, volIdx, volumeCount)
			}
		}
	}

	// Assign IDs to all elements
	idManager := logic.NewIDManager(outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	totalChapters := 0
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			totalChapters += len(vol.Chapters)
		}
	}
	logger.Info("Generated complete outline with %d part(s), %d volume(s), %d chapter(s)",
		len(outline.Parts), structure.TargetVolumes*structure.TargetParts, totalChapters)

	return outline, nil
}

// buildHierarchicalContext builds context for hierarchical generation
func (a *ComposeAgent) buildHierarchicalContext(outline *models.Outline, partIdx, volIdx int) string {
	var context strings.Builder

	context.WriteString("=== STORY CONTEXT ===\n\n")

	// Add previous volumes summary
	context.WriteString("Previous Volumes Summary:\n")
	for p := 0; p <= partIdx; p++ {
		for v := 0; v < len(outline.Parts[p].Volumes); v++ {
			if p < partIdx || v < volIdx {
				vol := outline.Parts[p].Volumes[v]
				context.WriteString(fmt.Sprintf("- %s: %s\n", vol.Title, vol.Summary))
				if len(vol.Chapters) > 0 {
					lastChap := vol.Chapters[len(vol.Chapters)-1]
					context.WriteString(fmt.Sprintf("  Last chapter: %s - %s\n", lastChap.Title, lastChap.Summary))
					context.WriteString(fmt.Sprintf("  Closing beat: %s\n", getClosingBeat(lastChap)))
				}
				context.WriteString("\n")
			}
		}
	}

	// Add current part context
	currentPart := outline.Parts[partIdx]
	context.WriteString(fmt.Sprintf("=== CURRENT PART ===\n"))
	context.WriteString(fmt.Sprintf("Part: %s\n", currentPart.Title))
	context.WriteString(fmt.Sprintf("Summary: %s\n\n", currentPart.Summary))

	// Add current volume context
	currentVolume := currentPart.Volumes[volIdx]
	context.WriteString(fmt.Sprintf("=== CURRENT VOLUME ===\n"))
	context.WriteString(fmt.Sprintf("Volume: %s\n", currentVolume.Title))
	context.WriteString(fmt.Sprintf("Summary: %s\n", currentVolume.Summary))
	context.WriteString("This volume needs chapters that:\n")
	context.WriteString("1. Follow from the previous volume's ending\n")
	context.WriteString("2. Build toward this volume's summary\n")
	context.WriteString("3. Set up for the next volume (if any)\n")

	return context.String()
}

// getOpeningBeat returns beats[0] or empty string.
func getOpeningBeat(chapter models.Chapter) string {
	if len(chapter.GetBeats()) > 0 {
		return chapter.GetBeats()[0]
	}
	return ""
}

// getClosingBeat returns beats[last] or empty string.
func getClosingBeat(chapter models.Chapter) string {
	if len(chapter.GetBeats()) > 0 {
		return chapter.GetBeats()[len(chapter.GetBeats())-1]
	}
	return ""
}
