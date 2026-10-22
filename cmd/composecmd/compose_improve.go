// Code in this file is part of the compose command implementation.
// Split from compose.go; keep one command/workflow concern per file.

package composecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
	"novelgen/internal/toolplane"
)

func runComposeImprove(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN COMPOSE IMPROVE")
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}

	session, err := openComposeSession(cmd)
	if err != nil {
		return err
	}
	book := session.Book()
	projectConfig := session.Config()
	cfg, client, err := session.LoadLLM()
	if err != nil {
		return err
	}

	// Check if story_setup.json exists
	setupPath := book.Root().SetupJSON()
	if !book.Root().HasSetup() {
		return fmt.Errorf("story setup not found at %s. Run 'novelgen setup gen' first", setupPath)
	}

	// Load story setup
	setup, err := session.RequireSetup()
	if err != nil {
		return err
	}

	// Load existing outline
	outlinePath := book.Root().OutlineJSON()
	outline, err := session.RequireOutline()
	if err != nil {
		return err
	}
	logger.Info("Loaded existing outline for improvement")
	originalOutline := toolplane.CloneOutline(outline)
	if countGeneratedVolumes(outline) == 0 {
		return fmt.Errorf("outline has no generated volumes to improve; run 'novelgen compose gen' first")
	}

	// Backup the current outline before any improvement pass mutates it. Both
	// legacy and Agent SDK improve paths overwrite outline.json in place; the
	// in-memory clone above only guards against a no-op run, so a disk backup
	// is the only way to recover the pre-improve outline after a bad iteration.
	if err := backupOutlineFiles(book); err != nil {
		logger.Error("Failed to backup outline before improve: %v", err)
		return err
	}
	logger.Info("Backed up outline before improvement")

	if composeHierarchicalFlag && composeOneShotFlag {
		return fmt.Errorf("--hierarchical and --one-shot cannot be used together")
	}
	if err := validateComposeAgentSDKOption(composeAgentSDKFlag, composeOneShotFlag); err != nil {
		return err
	}
	if err := validateComposeAgentApplyOption(composeAgentSDKFlag, composeAgentApplyFlag); err != nil {
		return err
	}

	// Default to hierarchical partial improvement for stability. --one-shot is
	// still available for smaller outlines and falls back to hierarchical if it
	// cannot produce valid output.
	useHierarchical := !composeOneShotFlag || composeHierarchicalFlag

	improveOutline := outline
	selectedImprove := composeImproveVolume > 0 || composeImproveFromVol > 0 || composeImproveToVol > 0
	if selectedImprove {
		improveOutline, err = outlineWithImproveVolumeSelection(outline, composeImproveVolume, composeImproveFromVol, composeImproveToVol)
		if err != nil {
			return err
		}
		logger.Info("Improving selected generated volumes only")
	} else if countEmptyVolumes(outline) > 0 {
		logger.Info("Outline contains empty volumes; improving only generated volumes")
		improveOutline = outlineWithGeneratedVolumes(outline)
	}

	// Run improvement
	if composeAgentSDKFlag {
		// A fresh run (no resume checkpoint) starts an empty improvement report.
		// A resume run keeps the existing report file so already-completed
		// volumes are not re-appended after a transient failure.
		progressPath := book.Root().ComposeFile("outline_improve_progress.json")
		keepReport := resumeProgressMatchesRun(book.Root(), progressPath, improveOutline, composeMaxRoundsFlag)
		if !keepReport {
			_ = book.Root().Remove(book.Root().ComposeFile("outline_improve_report.jsonl"))
		}
		agentSDKForceImprove := composeForceImproveFlag || selectedImprove
		var suggestions []models.ReviewSuggestion
		if strings.TrimSpace(composeSuggestionsFlag) != "" {
			suggestions, err = loadReviewSuggestionReports(book, composeSuggestionsFlag)
			if err != nil {
				return err
			}
		}
		// When the run is scoped to selected volume(s), the iteration outline
		// only contains those volumes. Pass the full outline as the
		// cross-volume context so apply mode can patch adjacent volumes in the
		// same session.
		var improveCrossOutline *models.Outline
		if selectedImprove {
			improveCrossOutline = outline
		}
		err = runAgentSDKImproveWithResume(cmd.Context(), book, improveOutline, setup, projectConfig, composeMaxRoundsFlag, agentSDKForceImprove, composePromptFlag, composeAgentApplyFlag, improveOutline != outline, suggestions, composeModelFlag, improveCrossOutline)
	} else {
		err = iterateOutlineImprovement(cmd.Context(), book, improveOutline, setup, projectConfig, cfg, client, composeMaxRoundsFlag, composeConcurrencyFlag, useHierarchical, composeForceImproveFlag, composePromptFlag)
	}
	if err != nil {
		logger.Error("Improvement failed: %v", err)
		return fmt.Errorf("improvement failed: %w", err)
	}
	var gate quality.QualityGateResult
	if improveOutline != outline {
		if composeAgentSDKFlag {
			var repairCrossOutline *models.Outline
			if selectedImprove {
				repairCrossOutline = outline
			}
			improveOutline, gate, err = repairOutlineWithQualityGateAgentSDK(book, cmd.Context(), improveOutline, setup, projectConfig, cfg, client, "improvement", composeAgentApplyFlag, true, composePromptFlag, repairCrossOutline)
		} else {
			improveOutline, gate, err = repairOutlineWithQualityGate(book, cmd.Context(), improveOutline, setup, projectConfig, cfg, client, "improvement", true)
		}
		if err != nil {
			return err
		}
		quality.LogQualityGateResult("outline", gate)
		mergeGeneratedVolumes(outline, improveOutline)
	} else {
		if composeAgentSDKFlag {
			outline, gate, err = repairOutlineWithQualityGateAgentSDK(book, cmd.Context(), outline, setup, projectConfig, cfg, client, "improvement", composeAgentApplyFlag, false, composePromptFlag, nil)
		} else {
			outline, gate, err = repairOutlineWithQualityGate(book, cmd.Context(), outline, setup, projectConfig, cfg, client, "improvement", false)
		}
		if err != nil {
			return err
		}
		quality.LogQualityGateResult("outline", gate)
	}

	// Save improved outline. In Agent SDK selected-volume mode the agent returns
	// a bounded patch; avoid whole-outline normalization here so unselected
	// volumes are not rewritten as a side effect of saving.
	if composeAgentSDKFlag && selectedImprove && outlinesSemanticallyEqual(originalOutline, outline) {
		logger.Info("Agent SDK selected-volume improve made no effective outline changes; skipping outline.json and outline.md rewrite")
		if reportErr := writeComposeImproveReport(book, -1); reportErr != nil {
			logger.Warn("Failed to write compose improve report: %v", reportErr)
		}
		fmt.Printf("\n[ok] Outline checked; no effective changes were applied to %s\n", outlinePath)
		return nil
	}
	if composeAgentSDKFlag && selectedImprove {
		err = saveOutlineMutationRaw(cmd.Context(), book, outline)
	} else {
		err = saveOutlineMutation(cmd.Context(), book, outline)
	}
	if err != nil {
		return fmt.Errorf("failed to save improved outline: %w", err)
	}

	// Update markdown version
	if _, err := book.SaveOutlineMarkdown(outline); err != nil {
		return fmt.Errorf("failed to save outline markdown: %w", err)
	}

	if composeAgentSDKFlag {
		remainingGateIssues := len(quality.FilterMediumOrHigherOutlineTargetSuggestions(gate.Suggestions))
		if reportErr := writeComposeImproveReport(book, remainingGateIssues); reportErr != nil {
			logger.Warn("Failed to write compose improve report: %v", reportErr)
		}
	}

	fmt.Printf("\n[ok] Outline improved and saved to %s\n", outlinePath)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/compose/outline.json to refine your outline")
	fmt.Println("  - Run 'novelgen craft' to create world elements")

	return nil
}

func runAgentSDKImproveWithResume(
	ctx context.Context,
	book *project.Book,
	improveOutline *models.Outline,
	setup *models.StorySetup,
	projectConfig *models.ProjectConfig,
	maxRounds int,
	forceImprove bool,
	userPrompt string,
	applyPatches bool,
	scoped bool,
	suggestions []models.ReviewSuggestion,
	modelOverride string,
	crossOutline *models.Outline,
) error {
	return retryAgentSDKImproveWithResume(composeAgentSDKImproveResumeAttempts, func() error {
		return iterateOutlineImprovementAgentSDK(ctx, book, improveOutline, setup, projectConfig, maxRounds, forceImprove, userPrompt, applyPatches, scoped, suggestions, modelOverride, crossOutline)
	})
}

// loadReviewSuggestionReports reads comma-separated ReviewResult JSON report
// paths and returns their combined suggestions. Both compose review and
// compose check --suggestions-out emit this shape, so any combination of AI
// review and deterministic check reports can seed an Agent SDK improve run.
func loadReviewSuggestionReports(book *project.Book, raw string) ([]models.ReviewSuggestion, error) {
	var out []models.ReviewSuggestion
	for _, part := range strings.Split(raw, ",") {
		path := strings.TrimSpace(part)
		if path == "" {
			continue
		}
		data, err := book.Root().Read(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read suggestion report %s: %w", path, err)
		}
		var report models.ReviewResult
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, fmt.Errorf("failed to parse suggestion report %s: %w", path, err)
		}
		out = append(out, report.Suggestions...)
	}
	return out, nil
}

func retryAgentSDKImproveWithResume(attempts int, run func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = run()
		if lastErr == nil {
			return nil
		}
		if attempt < attempts {
			logger.Warn("Agent SDK outline improvement failed (attempt %d/%d): %v; resuming from saved checkpoint", attempt, attempts, lastErr)
		}
	}
	return lastErr
}

// repairAgentSDKWithTransientRetry runs the Agent SDK repair pass, retrying up
// to maxRepairAttempts when the failure looks like a transient CLI/SDK
// transport error (e.g. the CLI control-channel "Stream closed" crash family).
// The repair pass writes its own per-iteration checkpoint, so each retry
// resumes from the last completed volume instead of restarting the whole
// batch. Permanent failures return immediately, and a leftover repair
// checkpoint is removed so a later invocation never resumes stale work.
func repairAgentSDKWithTransientRetry(book *project.Book, repair func() (*models.Outline, error)) (*models.Outline, error) {
	const maxRepairAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxRepairAttempts; attempt++ {
		output, err := repair()
		if err == nil {
			return output, nil
		}
		lastErr = err
		if attempt < maxRepairAttempts && isTransientAgentSDKError(err) {
			logger.Warn("Agent SDK repair pass failed with transient CLI/SDK error (attempt %d/%d): %v; resuming from repair checkpoint", attempt, maxRepairAttempts, err)
			continue
		}
		break
	}
	// The repair pass's checkpoint (iteration 0) is only removed on success;
	// clear it on permanent failure so the next invocation starts clean.
	if removeErr := book.Root().Remove(book.Root().ComposeFile("outline_improve_progress.json")); removeErr != nil {
		logger.Warn("Failed to remove stale Agent SDK repair checkpoint: %v", removeErr)
	}
	return nil, lastErr
}

// resumeProgressMatchesRun reports whether the on-disk improvement progress
// belongs to this command invocation (same iteration window and same target
// volumes). Only then is the improvement report jsonl kept across a restart.
func resumeProgressMatchesRun(root *store.Root, progressPath string, outline *models.Outline, maxRounds int) bool {
	data, err := root.Read(progressPath)
	if err != nil {
		return false
	}
	var progress composeImproveResumeProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return false
	}
	if progress.Iteration < 1 || progress.Iteration > utils.MaxInt(1, maxRounds) {
		return false
	}
	targets := make(map[string]bool, len(progress.TargetVolumes))
	for _, target := range progress.TargetVolumes {
		targets[target] = true
	}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if !targets[volume.ID] {
				return false
			}
		}
	}
	return len(progress.TargetVolumes) == outlineVolumeCount(outline)
}

func outlineVolumeCount(outline *models.Outline) int {
	count := 0
	if outline == nil {
		return count
	}
	for _, part := range outline.Parts {
		count += len(part.Volumes)
	}
	return count
}

func loadComposeImproveReportEntries(root *store.Root, path string) ([]agents.ComposeImproveReportEntry, error) {
	data, err := root.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entriesByKey := make(map[string]agents.ComposeImproveReportEntry)
	order := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry agents.ComposeImproveReportEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			logger.Warn("Skipping malformed improve report line: %v", err)
			continue
		}
		key := fmt.Sprintf("%d:%s", entry.Iteration, entry.VolumeID)
		if _, seen := entriesByKey[key]; !seen {
			order = append(order, key)
		}
		entriesByKey[key] = entry
	}
	entries := make([]agents.ComposeImproveReportEntry, 0, len(order))
	for _, key := range order {
		entries = append(entries, entriesByKey[key])
	}
	return entries, nil
}

// writeComposeImproveReport aggregates the per-volume report entries recorded
// by the Agent SDK improvement run into a Markdown file under logs/. Pass a
// negative remainingGateIssues when no gate result is available.
func writeComposeImproveReport(book *project.Book, remainingGateIssues int) error {
	entries, err := loadComposeImproveReportEntries(book.Root(), book.Root().ComposeFile("outline_improve_report.jsonl"))
	if err != nil {
		return err
	}
	if len(entries) == 0 && remainingGateIssues < 0 {
		return nil
	}
	now := time.Now()
	reportPath := book.Root().LogFile(fmt.Sprintf("compose_improve_report_%s.md", now.Format("20060102_150405")))

	var builder strings.Builder
	builder.WriteString("# Compose Improve 报告\n\n")
	fmt.Fprintf(&builder, "- 生成时间: %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&builder, "- 改进卷数: %d\n\n", len(entries))

	for index, entry := range entries {
		status := "已应用修复"
		if entry.Skipped {
			status = "跳过（预检无中高优问题）"
		} else if !entry.Changed {
			status = "评审通过（无有效改动）"
		}
		fmt.Fprintf(&builder, "## %d. %s（%s）\n\n", index+1, entry.VolumeTitle, entry.VolumeID)
		fmt.Fprintf(&builder, "- 状态: %s\n", status)
		if !entry.Skipped {
			fmt.Fprintf(&builder, "- 评分: %.1f\n", entry.Score)
			fmt.Fprintf(&builder, "- 剩余中高优问题: %d\n", entry.RemainingMediumPlus)
		}
		if summary := strings.TrimSpace(entry.Summary); summary != "" {
			fmt.Fprintf(&builder, "- 摘要: %s\n", summary)
		}
		builder.WriteString("\n")
	}
	if remainingGateIssues >= 0 {
		fmt.Fprintf(&builder, "## 门禁修复后\n\n- 剩余中高优问题: %d\n", remainingGateIssues)
	}

	if err := book.Root().WriteText(reportPath, builder.String()); err != nil {
		return err
	}
	logger.Info("Compose improve report written: %s", reportPath)
	fmt.Printf("\n[report] 改进报告: %s\n", reportPath)
	return nil
}

func repairOutlineWithQualityGate(book *project.Book, ctx context.Context, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client, stage string, scoped bool) (*models.Outline, quality.QualityGateResult, error) {
	applyOutlineNormalization(book, outline, "pre_gate_"+stage)
	gate := runOutlineQualityGateForScope(setup, outline, scoped)
	if !gate.Blocking {
		return outline, gate, nil
	}
	if outline == nil {
		return nil, gate, fmt.Errorf("outline quality gate failed: outline is nil")
	}

	logger.Info("Outline quality gate found blocking issues after %s; running one bounded repair pass", stage)
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	agent.SetProjectRoot(book.Root())
	review := quality.QualityGateReviewResult("Repair outline quality gate findings before saving project state.", gate)
	repaired, err := agent.RepairByReview(ctx, outline, review, setup)
	if err != nil {
		return outline, gate, fmt.Errorf("outline quality gate repair failed: %w", err)
	}
	applyOutlineNormalization(book, repaired, "post_gate_"+stage)
	finalGate := runOutlineQualityGateForScope(setup, repaired, scoped)
	return repaired, finalGate, nil
}

func repairOutlineWithQualityGateAgentSDK(book *project.Book, ctx context.Context, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client, stage string, agentApply bool, scoped bool, userPrompt string, crossOutline *models.Outline) (*models.Outline, quality.QualityGateResult, error) {
	applyOutlineNormalization(book, outline, "pre_gate_"+stage)
	gate := runOutlineQualityGateForScope(setup, outline, scoped)
	repairGate := filterQualityGateForAgentSDKPromptBoundary(gate, outline, userPrompt, "quality-gate repair")
	if !gate.Blocking {
		if agentApply && outlineGateHasPatchableGlobalIssues(repairGate, outline) {
			repairedOutline, repairedGate, err := repairOutlineGlobalIssuesAgentSDK(book, ctx, outline, setup, projectConfig, cfg, client, repairGate, stage)
			if err != nil {
				return outline, gate, err
			}
			return repairedOutline, repairedGate, nil
		}
		return outline, gate, nil
	}
	if outline == nil {
		return nil, gate, fmt.Errorf("outline quality gate failed: outline is nil")
	}

	logger.Info("Outline quality gate found blocking issues after %s; running one Agent SDK repair pass", stage)
	if !repairGate.Blocking {
		logger.Info("Skipping Agent SDK blocking repair after %s: user prompt boundary excludes blocking issue(s)", stage)
		return outline, gate, nil
	}
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	agent.SetProjectRoot(book.Root())
	review := quality.QualityGateReviewResult("Repair outline quality gate findings before saving project state.", repairGate)
	repaired, err := agent.RepairByReviewAgentSDK(ctx, outline, review, setup, agentApply, crossOutline, composeCrossVolumeAllFlag)
	if err != nil {
		return outline, gate, fmt.Errorf("outline quality gate Agent SDK repair failed: %w", err)
	}
	applyOutlineNormalization(book, repaired, "post_gate_"+stage)
	finalGate := runOutlineQualityGateForScope(setup, repaired, scoped)
	return repaired, finalGate, nil
}

func repairOutlineGlobalIssuesAgentSDK(book *project.Book, ctx context.Context, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client, gate quality.QualityGateResult, stage string) (*models.Outline, quality.QualityGateResult, error) {
	logger.Info("Outline quality gate found patchable global issues after %s; running one Agent SDK global repair pass", stage)
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	agent.SetProjectRoot(book.Root())
	review := quality.QualityGateReviewResult("Repair patchable global outline quality findings before saving project state.", gate)
	if _, err := agent.RepairGlobalIssuesWithAgentSDK(ctx, *review, true); err != nil {
		return outline, gate, fmt.Errorf("global outline quality gate Agent SDK repair failed: %w", err)
	}
	reloadedSetup := setup
	if loaded, loadErr := book.LoadSetup(); loadErr == nil {
		reloadedSetup = loaded
	} else {
		logger.Warn("Failed to reload story setup after global repair: %v", loadErr)
	}
	reloadedOutline := outline
	if loaded, loadErr := book.LoadOutline(); loadErr == nil {
		reloadedOutline = loaded
	} else {
		logger.Warn("Failed to reload outline after global repair; using in-memory outline: %v", loadErr)
	}
	return reloadedOutline, quality.RunOutlineQualityGate(reloadedSetup, reloadedOutline), nil
}

// iterateOutlineImprovement runs the review-improvement loop
func iterateOutlineImprovement(ctx context.Context, book *project.Book, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client, maxIterations int, concurrency int, hierarchical bool, forceImprove bool, userPrompt string) error {
	logger.Section("Outline Iteration Improvement")
	logger.Info("Maximum iterations: %d", maxIterations)
	if hierarchical {
		logger.Info("Mode: Hierarchical (review entire outline, improve volumes individually)")
	} else {
		logger.Info("Mode: One-shot (review and improve entire outline at once)")
	}
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}

	// Create compose agent
	agent := agents.NewComposeAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	agent.SetProjectRoot(book.Root())

	applyOutlineNormalization(book, outline, "pre_improve")

	var improvedOutline *models.Outline
	var review *models.ReviewResult
	var err error

	if hierarchical {
		// Use hierarchical iteration
		improvedOutline, review, err = agent.IterateHierarchical(ctx, outline, maxIterations, 80.0, forceImprove, userPrompt, setup)
	} else {
		// Use one-shot iteration
		improvedOutline, review, err = agent.Iterate(ctx, outline, maxIterations, 80.0, forceImprove, userPrompt, setup)
		if err != nil {
			logger.Warn("One-shot outline improve failed; falling back to hierarchical partial improve: %v", err)
			improvedOutline, review, err = agent.IterateHierarchical(ctx, outline, maxIterations, 80.0, forceImprove, userPrompt, setup)
		}
	}

	if err != nil {
		return fmt.Errorf("iteration failed: %w", err)
	}

	// Update the outline with improved version
	applyOutlineNormalization(book, improvedOutline, "post_improve")
	*outline = *improvedOutline

	// Enrich with direct model checks + DSL simulation + outline validator feedback.
	if review != nil {
		gate := quality.RunOutlineQualityGate(setup, improvedOutline)
		if len(gate.Suggestions) > 0 {
			review.Suggestions = append(review.Suggestions, gate.Suggestions...)
			logger.Info("Quality gate added %d suggestion(s) to review", len(gate.Suggestions))
		}

		// Feed DSL simulation feedback back into outline improve. Critical
		// findings force repair; softer story-contract findings get one direct
		// repair pass so the simulation loop can actually improve the outline.
		if gate.Blocking && maxIterations > 0 {
			logger.Info("Quality gate blocking feedback detected, running partial repair pass with enriched review")
			repairOutput, repairErr := agent.RepairByReview(ctx, improvedOutline, review, setup)
			if repairErr == nil {
				improvedOutline = repairOutput
				applyOutlineNormalization(book, improvedOutline, "post_repair")
				*outline = *improvedOutline
				logger.Info("Partial compose repair pass completed for DSL/validator feedback")
			} else {
				logger.Warn("Partial compose repair pass failed: %v", repairErr)
			}
		}
	}

	// Save intermediate result
	outlinePath := book.Root().ComposeFile(fmt.Sprintf("outline_iter_%d.json", maxIterations))
	if err := book.Root().SaveOutlineAt(outlinePath, outline); err != nil {
		logger.Error("Failed to save intermediate outline: %v", err)
	} else {
		logger.Info("Saved intermediate outline to %s", outlinePath)
	}

	logger.Section("Iteration Complete")
	if review != nil {
		logger.Info("Final Review Score: %.1f/100", review.OverallScore)
	}
	return nil
}

func iterateOutlineImprovementAgentSDK(ctx context.Context, book *project.Book, outline *models.Outline, setup *models.StorySetup, projectConfig *models.ProjectConfig, maxIterations int, forceImprove bool, userPrompt string, agentApply bool, scoped bool, suggestions []models.ReviewSuggestion, modelOverride string, crossOutline *models.Outline) error {
	logger.Section("Outline Agent SDK Iteration Improvement")
	logger.Info("Maximum iterations: %d", maxIterations)
	logger.Info("Mode: Agent SDK hierarchical per-volume review/improve")
	if forceImprove {
		logger.Info("Force improve enabled: will apply SDK volume output even if score meets threshold")
	}
	if agentApply {
		logger.Info("Agent apply enabled: SDK may write outline patches through validated tool apply")
	}

	agent, err := newComposeAgentForProject(book.Root(), projectConfig)
	if err != nil {
		return err
	}
	agent.SetLanguage(projectConfig.Language)
	if strings.TrimSpace(modelOverride) != "" {
		agent.SetModelOverride(modelOverride)
	}

	beforeOutline := toolplane.CloneOutline(outline)
	applyOutlineNormalization(book, outline, "pre_agent_sdk_improve")

	preGate := quality.RunOutlineCombinedGateForScope(book.Root(), setup, outline, scoped)
	var seedReview *models.ReviewResult
	if len(preGate.Suggestions) > 0 {
		seedReview = quality.QualityGateReviewResult("Pre-check issues for targeted Agent SDK outline improvement.", preGate)
		seedReview = filterAgentSDKReviewForPromptBoundary(seedReview, outline, userPrompt, "pre-check seed review")
		logger.Info("Pre-check supplied %d quality/simulation issue(s) to Agent SDK", len(seedReview.Suggestions))
	}
	if len(suggestions) > 0 {
		merged := quality.QualityGateResult{Suggestions: append([]models.ReviewSuggestion(nil), suggestions...)}
		if seedReview != nil {
			merged.Suggestions = append(merged.Suggestions, seedReview.Suggestions...)
		}
		merged.Dedup()
		seedReview = &models.ReviewResult{
			OverallScore: quality.ScoreFromGate(merged),
			Summary:      fmt.Sprintf("Agent SDK outline improvement seeded by %d suggestion report item(s).", len(merged.Suggestions)),
			Suggestions:  merged.Suggestions,
		}
		seedReview = filterAgentSDKReviewForPromptBoundary(seedReview, outline, userPrompt, "suggestion report seed")
		logger.Info("Suggestion report(s) provided %d issue(s) to Agent SDK", len(merged.Suggestions))
	}

	improvedOutline, review, err := agent.IterateHierarchicalAgentSDK(ctx, outline, crossOutline, seedReview, maxIterations, 80.0, forceImprove, userPrompt, setup, agentApply, composeCrossVolumeAllFlag)
	if err != nil {
		return fmt.Errorf("iteration failed: %w", err)
	}

	applyOutlineNormalization(book, improvedOutline, "post_agent_sdk_improve")
	*outline = *improvedOutline

	if review != nil {
		gate := quality.RunOutlineCombinedGateForScope(book.Root(), setup, improvedOutline, scoped)
		if len(gate.Suggestions) > 0 {
			review.Suggestions = append(review.Suggestions, gate.Suggestions...)
			logger.Info("Quality/simulation gate added %d suggestion(s) to Agent SDK review", len(gate.Suggestions))
		}
		repairReview := quality.QualityGateMediumReviewResult("Post-check medium-or-higher issues for Agent SDK targeted repair.", gate)
		repairReview = filterAgentSDKReviewForPromptBoundary(repairReview, improvedOutline, userPrompt, "post-check repair pass")
		if len(repairReview.Suggestions) > 0 && maxIterations > 0 {
			repairBatch := quality.LimitReviewSuggestionsForAgentSDKRepair(repairReview, composeRepairBudgetFlag)
			logger.Info("Quality/simulation gate found %d targetable medium-or-higher issue(s), running Agent SDK repair pass on %d issue(s)", len(repairReview.Suggestions), len(repairBatch.Suggestions))
			repairOutput, repairErr := repairAgentSDKWithTransientRetry(book, func() (*models.Outline, error) {
				return agent.RepairByReviewAgentSDK(ctx, improvedOutline, repairBatch, setup, agentApply, crossOutline, composeCrossVolumeAllFlag)
			})
			if repairErr == nil {
				improvedOutline = repairOutput
				applyOutlineNormalization(book, improvedOutline, "post_agent_sdk_repair")
				*outline = *improvedOutline
				repairGate := quality.RunOutlineCombinedGateForScope(book.Root(), setup, improvedOutline, scoped)
				remaining := quality.FilterMediumOrHigherOutlineTargetSuggestions(repairGate.Suggestions)
				if len(remaining) > 0 {
					logger.Warn("Agent SDK repair completed, but %d targetable medium-or-higher quality/simulation issue(s) remain", len(remaining))
				} else {
					logger.Info("Agent SDK partial compose repair pass completed; no targetable medium-or-higher quality/simulation issues remain")
				}
			} else {
				logger.Warn("Agent SDK partial compose repair pass failed: %v", repairErr)
			}
		}
	}

	if outlinesSemanticallyEqual(beforeOutline, outline) {
		logger.Info("Agent SDK iteration made no effective outline changes; skipping intermediate outline_iter_%d.json", maxIterations)
	} else {
		outlinePath := book.Root().ComposeFile(fmt.Sprintf("outline_iter_%d.json", maxIterations))
		if err := book.Root().SaveOutlineAt(outlinePath, outline); err != nil {
			logger.Error("Failed to save intermediate outline: %v", err)
		} else {
			logger.Info("Saved intermediate outline to %s", outlinePath)
		}
	}

	logger.Section("Agent SDK Iteration Complete")
	if review != nil {
		logger.Info("Final Agent SDK Review Score: %.1f/100", review.OverallScore)
	}
	return nil
}

func filterAgentSDKReviewForPromptBoundary(review *models.ReviewResult, outline *models.Outline, userPrompt, purpose string) *models.ReviewResult {
	if review == nil || outline == nil || strings.TrimSpace(userPrompt) == "" {
		return review
	}
	boundaryIDs := collectAgentSDKRepairBoundaryChapterIDs(outline, userPrompt)
	if len(boundaryIDs) == 0 {
		return review
	}
	filtered := make([]models.ReviewSuggestion, 0, len(review.Suggestions))
	for _, suggestion := range review.Suggestions {
		if boundaryIDs[strings.TrimSpace(suggestion.TargetID)] {
			filtered = append(filtered, suggestion)
		}
	}
	limited := *review
	limited.Suggestions = filtered
	limited.Summary = fmt.Sprintf("%s Prompt boundary retained %d of %d targetable issue(s).", strings.TrimSpace(review.Summary), len(filtered), len(review.Suggestions))
	if len(filtered) == 0 && len(review.Suggestions) > 0 {
		if strings.TrimSpace(purpose) == "" {
			purpose = "review"
		}
		logger.Info("Skipping Agent SDK %s: user prompt boundary does not include remaining targetable issue(s)", purpose)
	}
	return &limited
}

func filterQualityGateForAgentSDKPromptBoundary(gate quality.QualityGateResult, outline *models.Outline, userPrompt, purpose string) quality.QualityGateResult {
	review := quality.QualityGateReviewResult("Filter quality gate by Agent SDK prompt boundary.", gate)
	filtered := filterAgentSDKReviewForPromptBoundary(review, outline, userPrompt, purpose)
	if filtered == nil {
		return gate
	}
	limited := gate
	limited.Suggestions = append([]models.ReviewSuggestion(nil), filtered.Suggestions...)
	limited.Blocking = quality.HasBlockingSuggestions(limited.Suggestions)
	return limited
}

func collectAgentSDKRepairBoundaryChapterIDs(outline *models.Outline, userPrompt string) map[string]bool {
	ids := map[string]bool{}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, id := range agents.AgentSDKImproveBoundaryChapterIDs(volume, userPrompt) {
				if id = strings.TrimSpace(id); id != "" {
					ids[id] = true
				}
			}
		}
	}
	return ids
}

func outlinesSemanticallyEqual(left, right *models.Outline) bool {
	if left == nil || right == nil {
		return left == right
	}
	return reflect.DeepEqual(left, right)
}

// applyOutlineNormalization normalizes an outline and records the deterministic
// changes it made. The report is a diagnostics artifact, so a failed write only
// warns: the outline itself is saved by the caller.
func applyOutlineNormalization(book *project.Book, outline *models.Outline, stage string) {
	report := models.NormalizeOutline(outline)
	if !report.Changed() {
		return
	}

	logger.Info("Outline normalizer applied %d changes at %s", len(report.Changes), stage)
	reportPath := book.Root().ComposeFile(fmt.Sprintf("outline_normalization_%s.json", stage))
	if err := book.Root().WriteJSON(reportPath, report); err != nil {
		logger.Warn("Failed to save outline normalization report: %v", err)
	}
}
