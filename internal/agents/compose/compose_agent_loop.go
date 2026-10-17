// Code in this file is part of the ComposeAgent implementation.
// Split from compose_agent.go; keep the stage concern in one file.

package compose

import (
	"context"
	"fmt"
	agentbase "novelgen/internal/agents/base"
	"novelgen/internal/agents/prompt"
	"novelgen/internal/agents/review"
	"sort"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/store"
)

// ImproveProgress tracks the progress of hierarchical improvement
type ImproveProgress struct {
	Iteration        int                 `json:"iteration"`          // Current iteration number
	TotalIterations  int                 `json:"total_iterations"`   // Total iterations planned
	CurrentVolumeIdx int                 `json:"current_volume_idx"` // Index of next volume to improve (0-based)
	TotalVolumes     int                 `json:"total_volumes"`      // Total volumes to improve
	TargetVolumes    []string            `json:"target_volumes,omitempty"`
	CompletedVolumes []string            `json:"completed_volumes"` // List of completed volume IDs
	Outline          models.Outline      `json:"outline"`           // Current state of outline
	ReviewResult     models.ReviewResult `json:"review_result"`     // Review result for current iteration
}

func composeGlobalRepairToolAllowlist(applyPatches bool) []string {
	allowlist := []string{
		"novelgen tool query context --type outline-global-repair",
		"novelgen tool query story-setup --type search",
		"novelgen tool check all --target outline",
		"novelgen tool check all --target setup",
		"novelgen tool patch setup",
		"novelgen tool patch outline --target volume --id",
	}
	if applyPatches {
		allowlist = append(allowlist, "novelgen tool patch setup --apply")
		allowlist = append(allowlist, "novelgen tool patch outline --target volume --id --apply")
	}
	return agentbase.DedupeToolAllowlist(allowlist)
}

func composeGlobalRepairToolAllowlistForReview(review models.ReviewResult, applyPatches bool) []string {
	allowlist := []string{
		"novelgen tool query context --type outline-global-repair",
		"novelgen tool query story-setup --type search",
		"novelgen tool patch setup",
		"novelgen tool patch outline --target volume --id",
	}
	checks := composeGlobalRepairRequiredPostPatchChecks(review)
	if len(checks) == 0 {
		allowlist = append(allowlist,
			"novelgen tool check all --target outline",
			"novelgen tool check all --target setup",
		)
	} else {
		allowlist = append(allowlist, checks...)
	}
	if applyPatches {
		allowlist = append(allowlist, "novelgen tool patch setup --apply")
		allowlist = append(allowlist, "novelgen tool patch outline --target volume --id --apply")
	}
	return agentbase.DedupeToolAllowlist(allowlist)
}

// IterateSkeleton runs review/improve loops for a parts/volumes skeleton.
func (a *ComposeAgent) IterateSkeleton(ctx context.Context, outline *models.Outline, setup *models.StorySetup, structure models.StoryStructure, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT - Skeleton Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}

	if err := validateOutlineSkeleton(outline, structure); err != nil {
		return nil, nil, err
	}

	currentOutline := *outline
	var finalReview *models.ReviewResult
	session := review.NewRevisionSession("compose-skeleton", "Improve only outline parts/volumes/payoff contracts before chapter generation.")
	if strings.TrimSpace(userPrompt) != "" {
		session.AddUserGuidance(0, userPrompt)
	}

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Skeleton iteration %d/%d ===", i, maxIterations)
		reviewOutput, err := a.ReviewSkeleton(ctx, ComposeSkeletonReviewInput{
			ExistingOutline: currentOutline,
			Setup:           *setup,
			Structure:       structure,
			UserPrompt:      userPrompt,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("skeleton review failed at iteration %d: %w", i, err)
		}
		reviewOutput.Result.Iteration = i
		session.AddReview(i, reviewOutput.Result)
		finalReview = &reviewOutput.Result

		scoreMeetsThreshold := reviewOutput.Result.OverallScore >= qualityThreshold
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
		}
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing skeleton improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		improveOutput, err := a.ImproveSkeleton(ctx, ComposeSkeletonImproveInput{
			ExistingOutline: currentOutline,
			ReviewResult:    reviewOutput.Result,
			UserPrompt:      userPrompt,
			Setup:           *setup,
			Structure:       structure,
			RevisionContext: session.Prompt(),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("skeleton improvement failed at iteration %d: %w", i, err)
		}
		currentOutline = improveOutput.Outline
		session.AddImprove(i, "Applied skeleton review feedback while preserving part/volume IDs and existing chapters.")

		if i == maxIterations {
			logger.Warn("Max skeleton iterations reached, stopping iteration loop")
			break
		}
	}

	models.EnsureVolumeTitleOrdinals(&currentOutline)
	return &currentOutline, finalReview, nil
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
	session := review.NewRevisionSession("compose", "Improve the outline through review feedback while preserving existing structure and continuity.")
	if strings.TrimSpace(userPrompt) != "" {
		session.AddUserGuidance(0, userPrompt)
	}

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

		reviewOutput.Result.Iteration = i
		session.AddReview(i, reviewOutput.Result)
		finalReview = &reviewOutput.Result

		// Check if quality meets threshold
		scoreMeetsThreshold := reviewOutput.Result.OverallScore >= qualityThreshold
		if scoreMeetsThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
		}

		// Determine if we should improve. Blocking suggestions override the
		// numeric score because they represent issues that must still be fixed.
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		// Improve the outline with review feedback
		improveInput := ComposeImproveInput{
			ExistingOutline: currentOutline,
			ReviewResult:    prompt.CompactReviewForPrompt(reviewOutput.Result),
			UserPrompt:      userPrompt,
			RevisionContext: session.Prompt(),
		}
		if setup != nil {
			improveInput.Setup = *setup
		}
		improveOutput, err := a.Improve(ctx, improveInput)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}

		currentOutline = improveOutput.Outline
		session.AddImprove(i, "Applied the previous outline review feedback; next review should verify the repaired chapters and avoid undoing preserved structure.")
		logger.Info("✓ Outline improved based on review suggestions")

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
	}

	return &currentOutline, finalReview, nil
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

		// Determine if we should improve. Blocking suggestions override the
		// numeric score because they represent issues that must still be fixed.
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		// Step 2: Identify volumes that need improvement from suggestions
		logger.Section("Step 2: Improving Volumes")
		volumesToImprove := a.identifyVolumesToImprove(&currentOutline, &reviewOutput.Result)

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

// RepairByReview applies an existing review result through bounded volume-level
// repairs. It is intended for validator/DSL feedback after the main review pass,
// where re-running a full-outline rewrite would be unnecessarily fragile.
func (a *ComposeAgent) RepairByReview(ctx context.Context, outline *models.Outline, reviewResult *models.ReviewResult, setup *models.StorySetup) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	if reviewResult == nil {
		return outline, nil
	}

	volumesToImprove := a.identifyVolumesToImprove(outline, reviewResult)
	if len(volumesToImprove) == 0 {
		logger.Info("No specific volume targets in review; repairing generated volumes only")
		volumesToImprove = generatedVolumeIndices(outline)
	} else {
		volumesToImprove = filterGeneratedVolumeIndices(outline, volumesToImprove)
	}
	if len(volumesToImprove) == 0 {
		logger.Info("No generated volumes to repair")
		return outline, nil
	}

	logger.Info("Repairing %d volume(s) from enriched review feedback", len(volumesToImprove))
	return a.improveVolumesWithCheckpoint(ctx, outline, volumesToImprove, reviewResult, 0, 1, setup)
}

// improveVolumesWithCheckpoint improves volumes with checkpoint/resume support
func (a *ComposeAgent) improveVolumesWithCheckpoint(ctx context.Context, outline *models.Outline, volumesToImprove [][2]int, reviewResult *models.ReviewResult, currentIteration int, totalIterations int, setup *models.StorySetup) (*models.Outline, error) {
	currentOutline := *outline
	root, progressPath, err := a.composeImproveProgress()
	if err != nil {
		return nil, err
	}
	targetVolumes := improveTargetVolumeIDs(outline, volumesToImprove)

	// Try to load existing progress
	var progress *ImproveProgress
	if root.Exists(progressPath) {
		loadedProgress, err := a.loadImproveProgress(root, progressPath)
		if err == nil && loadedProgress.Iteration == currentIteration && equalStringSlices(loadedProgress.TargetVolumes, targetVolumes) {
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
			TargetVolumes:    targetVolumes,
			CompletedVolumes: []string{},
			Outline:          currentOutline,
			ReviewResult:     *reviewResult,
		}
		// Save initial progress
		if err := a.saveImproveProgress(root, progress, progressPath); err != nil {
			logger.Warn("Failed to save initial progress: %v", err)
		}
	}

	// Improve remaining volumes
	for idx := progress.CurrentVolumeIdx; idx < len(volumesToImprove); idx++ {
		indices := volumesToImprove[idx]
		partIdx, volIdx := indices[0], indices[1]
		if partIdx < 0 || partIdx >= len(currentOutline.Parts) || volIdx < 0 || volIdx >= len(currentOutline.Parts[partIdx].Volumes) {
			logger.Warn("Skipping invalid volume repair target part=%d volume=%d", partIdx+1, volIdx+1)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}
		part := &currentOutline.Parts[partIdx]
		volume := &part.Volumes[volIdx]
		if len(volume.Chapters) == 0 {
			logger.Warn("Skipping empty future volume repair target %s", volume.ID)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}

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
			if saveErr := a.saveImproveProgress(root, progress, progressPath); saveErr != nil {
				logger.Warn("Failed to save progress on error: %v", saveErr)
			}
			return nil, fmt.Errorf("failed to improve volume %d.%d: %w", partIdx+1, volIdx+1, err)
		}

		// Update the volume in the outline, preserving stable identity fields.
		improvedVolume := improveOutput.Volume
		preserveImprovedVolumeIdentity(volume, &improvedVolume)
		part.Volumes[volIdx] = improvedVolume
		progress.CompletedVolumes = append(progress.CompletedVolumes, volume.ID)
		progress.CurrentVolumeIdx = idx + 1
		progress.Outline = currentOutline

		// Save progress after each volume
		if err := a.saveImproveProgress(root, progress, progressPath); err != nil {
			logger.Warn("Failed to save progress: %v", err)
		} else {
			logger.Info("[save] Progress saved (%d/%d volumes completed)", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
	}

	// Remove progress file on successful completion of this iteration
	_ = root.Remove(progressPath)
	logger.Info("[ok] Iteration %d complete. Progress file removed.", currentIteration)

	return &currentOutline, nil
}

// composeImproveProgress returns the project the improve loop checkpoints into
// and the progress file's path. The path used to be spelled relative to the SDK
// process working directory, which only coincided with the project by accident.
func (a *ComposeAgent) composeImproveProgress() (*store.Root, string, error) {
	root, err := a.composeImproveRoot()
	if err != nil {
		return nil, "", err
	}
	return root, root.ComposeFile("outline_improve_progress.json"), nil
}

func countMediumOrHigherReviewSuggestions(review models.ReviewResult) int {
	count := 0
	for _, suggestion := range review.Suggestions {
		switch strings.ToLower(strings.TrimSpace(suggestion.Priority)) {
		case models.PriorityCritical, models.PriorityHigh, models.PriorityMedium:
			count++
		}
	}
	return count
}

func isEmptyReviewResult(review *models.ReviewResult) bool {
	if review == nil {
		return true
	}
	return review.OverallScore == 0 &&
		len(review.Dimensions) == 0 &&
		strings.TrimSpace(review.Summary) == "" &&
		len(review.Strengths) == 0 &&
		len(review.Weaknesses) == 0 &&
		len(review.Suggestions) == 0 &&
		len(review.ContinuityIssues) == 0
}

func generatedVolumeIndices(outline *models.Outline) [][2]int {
	if outline == nil {
		return nil
	}
	var result [][2]int
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			if len(outline.Parts[partIdx].Volumes[volIdx].Chapters) == 0 {
				continue
			}
			result = append(result, [2]int{partIdx, volIdx})
		}
	}
	return result
}

func filterGeneratedVolumeIndices(outline *models.Outline, indices [][2]int) [][2]int {
	if outline == nil {
		return nil
	}
	var result [][2]int
	for _, index := range indices {
		partIdx, volIdx := index[0], index[1]
		if partIdx < 0 || partIdx >= len(outline.Parts) || volIdx < 0 || volIdx >= len(outline.Parts[partIdx].Volumes) {
			continue
		}
		if len(outline.Parts[partIdx].Volumes[volIdx].Chapters) == 0 {
			continue
		}
		result = append(result, index)
	}
	return result
}

func improveTargetVolumeIDs(outline *models.Outline, indices [][2]int) []string {
	if outline == nil {
		return nil
	}
	targets := make([]string, 0, len(indices))
	for _, index := range indices {
		partIdx, volIdx := index[0], index[1]
		if partIdx < 0 || partIdx >= len(outline.Parts) || volIdx < 0 || volIdx >= len(outline.Parts[partIdx].Volumes) {
			targets = append(targets, fmt.Sprintf("invalid:%d:%d", partIdx, volIdx))
			continue
		}
		volumeID := strings.TrimSpace(outline.Parts[partIdx].Volumes[volIdx].ID)
		if volumeID == "" {
			volumeID = fmt.Sprintf("P%d-V%d", partIdx+1, volIdx+1)
		}
		targets = append(targets, volumeID)
	}
	return targets
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// loadImproveProgress loads improvement progress through the project store.
func (a *ComposeAgent) loadImproveProgress(root *store.Root, path string) (*ImproveProgress, error) {
	var progress ImproveProgress
	if err := root.ReadJSON(path, &progress); err != nil {
		return nil, fmt.Errorf("failed to read progress file: %w", err)
	}
	return &progress, nil
}

// saveImproveProgress saves improvement progress through the store's atomic
// write. It used to hand-roll a temp-file-plus-rename of its own next to the
// store's version, which meant two implementations of the same guarantee.
func (a *ComposeAgent) saveImproveProgress(root *store.Root, progress *ImproveProgress, path string) error {
	if progress == nil {
		return fmt.Errorf("cannot save nil progress")
	}
	if err := root.WriteJSON(path, progress); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}
	return nil
}

// identifyVolumesToImprove identifies which volumes need improvement based on suggestions
func (a *ComposeAgent) identifyVolumesToImprove(outline *models.Outline, review *models.ReviewResult) [][2]int {
	if outline == nil || review == nil {
		return nil
	}

	seen := make(map[[2]int]bool)
	var result [][2]int

	for _, suggestion := range review.Suggestions {
		indices, ok := resolveVolumeTarget(outline, suggestion.TargetID)
		if !ok {
			continue
		}
		if seen[indices] {
			continue
		}
		seen[indices] = true
		result = append(result, indices)
	}

	return result
}

func resolveVolumeTarget(outline *models.Outline, targetID string) ([2]int, bool) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" || strings.EqualFold(targetID, "global") {
		return [2]int{}, false
	}

	for partIdx, part := range outline.Parts {
		for volIdx, volume := range part.Volumes {
			if targetID == volume.ID || strings.HasPrefix(targetID, volume.ID+"-") {
				return [2]int{partIdx, volIdx}, true
			}
		}
	}

	parts := strings.Split(targetID, "-")
	if len(parts) < 2 {
		return [2]int{}, false
	}
	volumeID := parts[0] + "-" + parts[1]

	var partNum, volNum int
	n, err := fmt.Sscanf(volumeID, "P%d-V%d", &partNum, &volNum)
	if err != nil || n != 2 {
		logger.Warn("Failed to parse volume ID '%s', skipping", volumeID)
		return [2]int{}, false
	}
	if partNum <= 0 || volNum <= 0 {
		logger.Warn("Invalid volume ID '%s' (part=%d, vol=%d), skipping", volumeID, partNum, volNum)
		return [2]int{}, false
	}

	partIdx := partNum - 1
	volIdx := volNum - 1
	if partIdx >= 0 && partIdx < len(outline.Parts) && volIdx >= 0 && volIdx < len(outline.Parts[partIdx].Volumes) {
		return [2]int{partIdx, volIdx}, true
	}

	// Single-part projects are often described by users and LLM review as
	// "volume 2"; tolerate malformed targets like P2-V2 by resolving the
	// volume ordinal against the only part instead of silently skipping it.
	if len(outline.Parts) == 1 && volIdx >= 0 && volIdx < len(outline.Parts[0].Volumes) {
		logger.Warn("Interpreting malformed single-part volume target '%s' as '%s'", volumeID, outline.Parts[0].Volumes[volIdx].ID)
		return [2]int{0, volIdx}, true
	}

	logger.Warn("Skipping invalid volume repair target %s", volumeID)
	return [2]int{}, false
}

// filterReviewForVolume filters review results for a specific volume
func (a *ComposeAgent) filterReviewForVolume(review *models.ReviewResult, volumeID string) models.ReviewResult {
	if review == nil {
		return models.ReviewResult{}
	}
	filtered := models.ReviewResult{
		OverallScore: review.OverallScore,
		Dimensions:   review.Dimensions,
		Summary:      review.Summary,
	}

	for _, suggestion := range review.Suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if strings.HasPrefix(targetID, volumeID) || isBlockingGlobalSuggestion(suggestion) {
			filtered.Suggestions = append(filtered.Suggestions, suggestion)
		}
	}
	sort.SliceStable(filtered.Suggestions, func(i, j int) bool {
		left := models.ReviewSuggestionPriorityRank(filtered.Suggestions[i].Priority)
		right := models.ReviewSuggestionPriorityRank(filtered.Suggestions[j].Priority)
		if left != right {
			return left < right
		}
		return strings.TrimSpace(filtered.Suggestions[i].TargetID) < strings.TrimSpace(filtered.Suggestions[j].TargetID)
	})
	if len(filtered.Suggestions) > 12 {
		filtered.Suggestions = filtered.Suggestions[:12]
	}

	return filtered
}

func isBlockingGlobalSuggestion(suggestion models.ReviewSuggestion) bool {
	targetID := strings.TrimSpace(suggestion.TargetID)
	if targetID != "" && !strings.EqualFold(targetID, "global") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(suggestion.Priority)) {
	case models.PriorityCritical, models.PriorityHigh:
		return true
	default:
		return false
	}
}

func hasMediumOrHigherReviewSuggestion(review models.ReviewResult) bool {
	for _, suggestion := range review.Suggestions {
		switch strings.ToLower(strings.TrimSpace(suggestion.Priority)) {
		case models.PriorityCritical, models.PriorityHigh, models.PriorityMedium:
			return true
		}
	}
	return false
}
