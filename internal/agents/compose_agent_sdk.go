// Code in this file is part of the ComposeAgent implementation.
// Split from compose_agent.go; keep the stage concern in one file.

package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"novelgen/internal/toolcmd"
	"reflect"
	"strings"

	"novelgen/internal/agentruntime"
	"novelgen/internal/envutil"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/utils"
)

type composeAgentSDKSkeletonPromptInput struct {
	Structure       models.StoryStructure `json:"structure" md:"structure" desc:"Expected parts, volumes, and chapters per volume"`
	RequiredQueries []string              `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions    []string              `json:"instructions" md:"instructions" desc:"Workflow constraints for the SDK agent"`
}

type composeAgentSDKChaptersPromptInput struct {
	TargetPartID     string   `json:"target_part_id" md:"target_part_id" desc:"Part ID containing the target volume"`
	TargetVolumeID   string   `json:"target_volume_id" md:"target_volume_id" desc:"Volume ID to generate"`
	TargetVolumeName string   `json:"target_volume_name" md:"target_volume_name" desc:"Current target volume title for display only"`
	VolumeIndex      int      `json:"volume_index" md:"volume_index" desc:"1-based global volume index"`
	TotalVolumes     int      `json:"total_volumes" md:"total_volumes" desc:"Total volume count"`
	ChaptersPerVol   int      `json:"chapters_per_volume" md:"chapters_per_volume" desc:"Exact number of chapters to return"`
	PreviousVolumeID string   `json:"previous_volume_id,omitempty" md:"previous_volume_id" desc:"Previous volume ID for continuity queries"`
	RequiredQueries  []string `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions     []string `json:"instructions" md:"instructions" desc:"Workflow constraints for the SDK agent"`
}

type composeAgentSDKImproveVolumePromptInput struct {
	TargetPartID     string                            `json:"target_part_id" md:"target_part_id" desc:"Part ID containing the target volume"`
	TargetVolumeID   string                            `json:"target_volume_id" md:"target_volume_id" desc:"Only this volume may be reviewed and returned"`
	TargetVolumeName string                            `json:"target_volume_name" md:"target_volume_name" desc:"Current target volume title for display only"`
	ChapterCount     int                               `json:"chapter_count" md:"chapter_count" desc:"Exact number of chapters that volume.chapters must contain"`
	ReviewResult     composeAgentSDKPromptReviewResult `json:"review_result,omitempty" md:"review_result" desc:"Optional quality gate or user review feedback relevant to this volume, including focused navigation queries"`
	UserPrompt       string                            `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	ApplyPatches     bool                              `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use tool patch outline --apply after a successful dry-run"`
	ForceIssueRepair bool                              `json:"force_issue_repair" md:"force_issue_repair" desc:"When true, focused review_result suggestions are an explicit repair task list, including directly fixable low-priority issues"`
	RequiredQueries  []string                          `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions     []string                          `json:"instructions" md:"instructions" desc:"Workflow constraints for the SDK agent"`
}

type composeAgentSDKGlobalRepairPromptInput struct {
	ReviewResult     composeAgentSDKPromptReviewResult `json:"review_result" md:"review_result" desc:"Global outline check issues, including navigation when available"`
	ApplyPatches     bool                              `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use validated setup/outline patch --apply"`
	ForceIssueRepair bool                              `json:"force_issue_repair" md:"force_issue_repair" desc:"Global issues are explicit repair tasks when they have patch_query and patch_shape"`
	RequiredQueries  []string                          `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions     []string                          `json:"instructions" md:"instructions" desc:"Workflow constraints for global repair"`
}

type composeAgentSDKPromptReviewResult struct {
	OverallScore float64                                 `json:"overall_score,omitempty" md:"overall_score" desc:"0-100 score from deterministic checks or prior review"`
	Summary      string                                  `json:"summary,omitempty" md:"summary" desc:"Compact review/check summary"`
	Suggestions  []composeAgentSDKPromptReviewSuggestion `json:"suggestions,omitempty" md:"suggestions" desc:"Focused issues for this target volume"`
}

type composeAgentSDKPromptReviewSuggestion struct {
	Category   string                               `json:"category,omitempty" md:"category" desc:"Issue category such as logic, structure, state_anchor, resource_ledger"`
	TargetID   string                               `json:"target_id,omitempty" md:"target_id" desc:"Target volume or chapter ID"`
	TargetName string                               `json:"target_name,omitempty" md:"target_name" desc:"Short target display name"`
	Issue      string                               `json:"issue" md:"issue" desc:"Short issue description"`
	Suggestion string                               `json:"suggestion,omitempty" md:"suggestion" desc:"Short suggested fix"`
	Priority   string                               `json:"priority" md:"priority" desc:"critical, high, medium, or low"`
	Navigation *composeAgentSDKSuggestionNavigation `json:"navigation,omitempty" md:"navigation" desc:"Focused commands to gather the smallest useful context for this issue"`
}

type composeAgentSDKSuggestionNavigation struct {
	TargetKind         string                 `json:"target_kind,omitempty" md:"target_kind" desc:"chapter, volume, or outline"`
	VolumeID           string                 `json:"volume_id,omitempty" md:"volume_id" desc:"Volume ID to patch when repairing this issue"`
	DetailQueries      []string               `json:"detail_queries,omitempty" md:"detail_queries" desc:"Read-only queries to inspect before broadening context"`
	FocusedCheckQuery  string                 `json:"focused_check_query,omitempty" md:"focused_check_query" desc:"Smallest useful check command for this issue"`
	RepairRouteQuery   string                 `json:"repair_route_query,omitempty" md:"repair_route_query" desc:"Index-sized route query with next_actions for this issue"`
	RepairContextQuery string                 `json:"repair_context_query,omitempty" md:"repair_context_query" desc:"Smallest useful context bundle for this issue"`
	PatchQuery         string                 `json:"patch_query,omitempty" md:"patch_query" desc:"Patch command prefix for dry-run/apply"`
	PatchShape         map[string]interface{} `json:"patch_shape,omitempty" md:"patch_shape" desc:"Minimal JSON patch shape hint"`
}

type ComposeAgentSDKImproveVolumeOutput struct {
	ReviewResult models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result for the target volume"`
	Volume       models.Volume       `json:"volume" md:"volume" desc:"Improved target volume with chapters"`
}

type composeAgentSDKOutlineReviewPromptInput struct {
	TargetVolumeID  string   `json:"target_volume_id,omitempty" md:"target_volume_id" desc:"Optional volume scope to review; empty means the whole outline"`
	VolumeCount     int      `json:"volume_count,omitempty" md:"volume_count" desc:"Total number of volumes in scope"`
	ChapterCount    int      `json:"chapter_count,omitempty" md:"chapter_count" desc:"Total number of chapters in scope"`
	UserPrompt      string   `json:"user_prompt,omitempty" md:"user_prompt" desc:"Optional user review focus; the highest-priority review task when present"`
	ReviewFocus     string   `json:"review_focus,omitempty" md:"review_focus" desc:"Review dimensions to cover when no user prompt is given"`
	RequiredQueries []string `json:"required_queries" md:"required_queries" desc:"Exact tool commands to run first, in order"`
	Instructions    []string `json:"instructions,omitempty" md:"instructions" desc:"Workflow rules for this review run"`
}

type composeAgentSDKImprovePatchOutput struct {
	ReviewResult composeAgentSDKReviewResult `json:"review_result" md:"review_result" desc:"Compact review result for the target volume. Keep it short and focused on applied or remaining issues."`
	VolumePatch  composeVolumePatch          `json:"volume_patch" md:"volume_patch" desc:"Patch for the target volume. Return only changed volume fields and complete changed chapters; Go will merge it into the original volume."`
}

type composeAgentSDKApplyOutput struct {
	ReviewResult      composeAgentSDKReviewResult `json:"review_result" md:"review_result" desc:"Compact review result for the target volume after any applied patches"`
	AppliedPatches    bool                        `json:"applied_patches" md:"applied_patches" desc:"True only if the workflow successfully used tool patch outline --apply"`
	AppliedPatchCount int                         `json:"applied_patch_count,omitempty" md:"applied_patch_count" desc:"Number of successful apply calls, if any"`
	FinalCheck        string                      `json:"final_check,omitempty" md:"final_check" desc:"Compact final tool check summary, or why no patch/check was needed"`
}

type composeAgentSDKGlobalRepairOutput struct {
	ReviewResult      composeAgentSDKReviewResult `json:"review_result" md:"review_result" desc:"Compact review result for global outline issues after any applied patches"`
	AppliedPatches    bool                        `json:"applied_patches" md:"applied_patches" desc:"True only if the workflow successfully used a validated tool patch --apply"`
	AppliedPatchCount int                         `json:"applied_patch_count,omitempty" md:"applied_patch_count" desc:"Number of successful apply calls, if any"`
	FinalCheck        string                      `json:"final_check,omitempty" md:"final_check" desc:"Compact final tool check summary, or why no patch/check was needed"`
}

type composeAgentSDKReviewResult struct {
	OverallScore float64                           `json:"overall_score" md:"overall_score" desc:"0-100 score for the target volume after the returned patch"`
	Summary      string                            `json:"summary" md:"summary" desc:"Compact Chinese summary under 500 characters"`
	Suggestions  []composeAgentSDKReviewSuggestion `json:"suggestions,omitempty" md:"suggestions" desc:"At most 8 remaining issues or applied patch notes"`
}

type composeAgentSDKReviewSuggestion struct {
	Category   string `json:"category,omitempty" md:"category" desc:"Issue category such as logic, structure, state_anchor, resource_ledger"`
	TargetID   string `json:"target_id,omitempty" md:"target_id" desc:"Target volume or chapter ID"`
	TargetName string `json:"target_name,omitempty" md:"target_name" desc:"Short target display name"`
	Issue      string `json:"issue" md:"issue" desc:"Short issue or applied-change description"`
	Suggestion string `json:"suggestion,omitempty" md:"suggestion" desc:"Short fix, or why no patch is needed"`
	Priority   string `json:"priority" md:"priority" desc:"critical, high, medium, or low"`
}

func composeAgentSDKParams(command, workflowSkill string, maxTurns int, toolAllowlist []string) InvokeParams {
	if maxTurns <= 0 {
		maxTurns = 12
	}
	toolAllowlist = append(append([]string(nil), toolAllowlist...), agentSDKLogToolAllowlist()...)
	return agentSDKToolParams(
		[]string{"novel-tools-core", workflowSkill},
		command,
		dedupeToolAllowlist(toolAllowlist),
		ToolEvidenceRequirement{},
		maxTurns,
		900,
	)
}

func AgentSDKImproveBoundaryChapterIDs(volume models.Volume, prompt string) []string {
	return append([]string(nil), composeImproveBoundaryChapterIDs(volume, prompt)...)
}

// GenerateSkeletonWithAgentSDK asks the Claude Agent SDK workflow to query
// project facts and return an outline skeleton. Go still validates and saves.
func (a *ComposeAgent) GenerateSkeletonWithAgentSDK(ctx context.Context, input ComposeSkeletonInput) (ComposeSkeletonOutput, error) {
	logger.Section("COMPOSE AGENT SDK - Generating Outline Skeleton")
	var output ComposeSkeletonOutput
	promptInput := composeAgentSDKSkeletonPromptInput{
		Structure: input.Structure,
		RequiredQueries: []string{
			"novelgen tool query story-setup --view brief",
		},
		Instructions: []string{
			"Do not use the setup_brief shortcut; query the project facts with the required command.",
			"Use --view brief or --view index before asking for full detail; avoid broad full-output queries.",
			"Return only the parts/volumes skeleton. Chapters must be empty arrays.",
		},
	}
	params := composeAgentSDKParams("generate the story outline skeleton using read-only project queries", "outline-compose-skeleton-workflow", 10, composeRequiredQueryAllowlist(promptInput.RequiredQueries))
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeSkeletonOutput{}, err
	}
	if len(output.Parts) != input.Structure.TargetParts {
		return ComposeSkeletonOutput{}, fmt.Errorf("agent SDK generated %d parts, but %d were requested", len(output.Parts), input.Structure.TargetParts)
	}
	for i, part := range output.Parts {
		if len(part.Volumes) != input.Structure.TargetVolumes {
			return ComposeSkeletonOutput{}, fmt.Errorf("agent SDK part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), input.Structure.TargetVolumes)
		}
	}
	return output, nil
}

// GenerateChaptersForVolumeWithAgentSDK asks the SDK workflow to fill one
// volume. Only returned chapters are accepted; volume identity stays in Go.
func (a *ComposeAgent) GenerateChaptersForVolumeWithAgentSDK(ctx context.Context, input ComposeChaptersInput) (ComposeImproveVolumeOutput, error) {
	logger.Section("COMPOSE AGENT SDK - Generating Chapters for Volume")
	logger.Info("Volume: %s (%d/%d)", input.Volume.Title, input.VolumeIndex, input.TotalVolumes)

	var output ComposeImproveVolumeOutput
	promptInput := buildAgentSDKChaptersPromptInput(input)
	params := composeAgentSDKParams("generate chapters for this volume using read-only project queries", "outline-compose-volume-workflow", 24, composeRequiredQueryAllowlist(promptInput.RequiredQueries))
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeImproveVolumeOutput{}, err
	}
	volume, err := prepareAgentSDKReturnedVolume(input.Volume, output.Volume, input.ChaptersPerVol)
	if err != nil {
		return ComposeImproveVolumeOutput{}, err
	}
	for i, chapter := range volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}
	output.Volume = volume
	return output, nil
}

// ImproveVolumeWithAgentSDK lets the SDK workflow query context, review a
// target volume, and return an improved volume plus review metadata.
func (a *ComposeAgent) ImproveVolumeWithAgentSDK(ctx context.Context, input ComposeImproveVolumeInput, applyPatches bool, forceIssueRepair bool) (ComposeAgentSDKImproveVolumeOutput, error) {
	logger.Section("COMPOSE AGENT SDK - Volume Review/Improvement")
	crossVolume := len(input.CrossVolumeIDs) > 0
	if crossVolume {
		logger.Info("Cross-volume mode: target=%s cross=%v", input.Volume.ID, input.CrossVolumeIDs)
	} else {
		logger.Info("Volume: %s (single-volume mode, CrossVolumeIDs=%d)", input.Volume.Title, len(input.CrossVolumeIDs))
	}
	if applyPatches {
		logger.Info("Agent apply enabled: SDK may write through validated outline patch tools")
	}
	if forceIssueRepair {
		logger.Info("Focused review issues are treated as repair tasks, including directly fixable low-priority issues")
	}

	promptInput := buildAgentSDKImproveVolumePromptInput(input, applyPatches, forceIssueRepair)
	allowlist := composeImproveToolAllowlist(input.Volume, promptInput, applyPatches)
	if crossVolume {
		// Cross-volume mode: allow query/check/patch on every target volume in one session.
		for _, vid := range input.CrossVolumeIDs {
			vid = strings.TrimSpace(vid)
			if vid == "" {
				continue
			}
			for _, vol := range input.Outline.Parts {
				for _, v := range vol.Volumes {
					if strings.EqualFold(strings.TrimSpace(v.ID), vid) {
						allowlist = append(allowlist, composeImproveVolumeToolAllowlist(v, promptInput, applyPatches)...)
					}
				}
			}
		}
		allowlist = append(allowlist,
			"novelgen tool query outline --type refs --entity-type storyline --name",
			"novelgen tool query outline --type refs --entity-type character --name",
			"novelgen tool query outline --type refs --entity-type item --name",
			"novelgen tool query outline --type refs --entity-type location --name",
		)
	}
	allowlist = append(allowlist, composeAdjacentVolumeQueryAllowlist(input.Outline, input.Volume.ID)...)
	params := composeAgentSDKParams("review and improve this outline volume using project query/check/patch tools", "outline-improve-volume-workflow", 28, dedupeToolAllowlist(allowlist))
	params.ToolEvidence = composeImproveToolEvidence(promptInput, applyPatches)
	// An explicit user prompt makes this an explicit task list: clean scoped
	// checks must not close targets or stop the agent before the patch cycle.
	params.UserPromptDriven = strings.TrimSpace(input.UserPrompt) != ""
	if crossVolume {
		// Cross-volume sessions cover 2-3 volumes (target + adjacent) with up to
		// 45 turns; the default 900s timeout is too tight and caused TimeoutError
		// mid-run. Give them a 30-minute budget.
		params.MaxTurns = 45
		params.Timeout = 1800
	}
	if applyPatches {
		params.ToolEvidence.RequirePatchApplyFollowupCheck = true
	}

	if applyPatches {
		var applyOutput composeAgentSDKApplyOutput
		runtimeResult, err := a.base.ExecuteWithRuntimeResult(ctx, params, promptInput, &applyOutput)
		if err != nil {
			if recovered, recoverErr := a.recoverAppliedVolumeAfterAgentSDKError(input.Volume, runtimeResult, err); recoverErr == nil {
				return recovered, nil
			}
			return ComposeAgentSDKImproveVolumeOutput{}, err
		}
		if err := validateComposeImproveAgentSDKPatchApplyEvidence(runtimeResult, applyOutput.AppliedPatches, applyOutput.AppliedPatchCount); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, err
		}
		if err := validateAgentSDKApplyOutput(applyOutput); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, err
		}
		reviewResult := applyOutput.ReviewResult.toModelReviewResult()
		reviewResult.NormalizeScoreScale()
		volume := input.Volume
		if applyOutput.AppliedPatches {
			var err error
			volume, err = a.loadAgentSDKAppliedOutlineVolume(input.Volume, true)
			if err != nil {
				return ComposeAgentSDKImproveVolumeOutput{}, err
			}
		}
		for i, chapter := range volume.Chapters {
			if err := a.validateChapterOutput(&chapter); err != nil {
				return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
			}
		}
		return ComposeAgentSDKImproveVolumeOutput{
			ReviewResult: reviewResult,
			Volume:       volume,
		}, nil
	}

	var patchOutput composeAgentSDKImprovePatchOutput
	runtimeResult, err := a.base.ExecuteWithRuntimeResult(ctx, params, promptInput, &patchOutput)
	if err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	if err := validateComposeImproveAgentSDKPatchApplyEvidence(runtimeResult, false, 0); err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	if err := validateAgentSDKImprovePatchOutput(patchOutput, input.Volume.ID); err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	if err := utils.ValidateNoSuspiciousPatchText(patchOutput.VolumePatch); err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("agent SDK volume patch rejected: %w", err)
	}
	reviewResult := patchOutput.ReviewResult.toModelReviewResult()
	reviewResult.NormalizeScoreScale()
	volume, err := applyAgentSDKVolumePatch(input.Volume, patchOutput.VolumePatch, len(input.Volume.Chapters))
	if err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	for i, chapter := range volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}
	return ComposeAgentSDKImproveVolumeOutput{
		ReviewResult: reviewResult,
		Volume:       volume,
	}, nil
}

// ReviewOutlineWithAgentSDK reviews the outline (whole book or one volume)
// through the read-only Agent SDK workflow and returns a ReviewResult. The
// review is deliberately orthogonal to deterministic checks: it reports
// open-ended issues (pacing, motivation, theme, information warfare) that
// rule-based checks cannot judge, and never patches project state.
func (a *ComposeAgent) ReviewOutlineWithAgentSDK(ctx context.Context, input ComposeOutlineReviewInput) (models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Outline Review")
	scope := "whole outline"
	if input.CrossVolume && input.FromVolumeIndex > 0 {
		scope = fmt.Sprintf("volumes %d-%d (cross-volume)", input.FromVolumeIndex, input.ToVolumeIndex)
	} else if strings.TrimSpace(input.VolumeID) != "" {
		scope = "volume " + strings.TrimSpace(input.VolumeID)
	}
	logger.Info("Scope: %s", scope)

	if input.CrossVolume {
		return a.reviewOutlineCrossVolumeAgentSDK(ctx, input)
	}

	promptInput := buildAgentSDKOutlineReviewPromptInput(input)
	params := composeOutlineReviewAgentSDKParams("review the story outline and provide focused improvement suggestions", input.Outline, input.VolumeID)
	params.UserPromptDriven = strings.TrimSpace(input.UserPrompt) != ""
	var output models.ReviewResult
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return models.ReviewResult{}, err
	}
	output.NormalizeScoreScale()
	clipOutlineReviewResult(&output)
	logger.Section("Outline Agent SDK Review Result")
	logger.Info("Overall Score: %.1f/100", output.OverallScore)
	logger.Info("Suggestions: %d", len(output.Suggestions))
	return output, nil
}

// reviewOutlineCrossVolumeAgentSDK 跨卷审查: 一次 session 内按"线"审查多卷之间的连续性问题。
func (a *ComposeAgent) reviewOutlineCrossVolumeAgentSDK(ctx context.Context, input ComposeOutlineReviewInput) (models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Cross-Volume Outline Review")

	// 收集范围内的卷
	var inScope []models.Volume
	for _, part := range input.Outline.Parts {
		for _, volume := range part.Volumes {
			idx := globalVolumeIndex(input.Outline, volume.ID)
			if idx >= input.FromVolumeIndex && (input.ToVolumeIndex <= 0 || idx <= input.ToVolumeIndex) {
				inScope = append(inScope, volume)
			}
		}
	}
	if len(inScope) == 0 {
		return models.ReviewResult{}, fmt.Errorf("cross-volume review: no volumes in range %d-%d", input.FromVolumeIndex, input.ToVolumeIndex)
	}
	volCount, chapCount := 0, 0
	for _, v := range inScope {
		volCount++
		chapCount += len(v.Chapters)
	}
	logger.Info("Cross-volume scope: %d volumes, %d chapters", volCount, chapCount)

	instructions := []string{
		"Run the required first query exactly as given before anything else. It returns the overall structure of the review scope.",
		"This is a read-only cross-volume review: do not patch, do not run check, do not write files, do not read source/RPG/Claude tool-results.",
		"Focus on CROSS-VOLUME issues only: foreshadowing lifecycle, setup consistency drift, repeated tropes across volumes, macro pacing, volume-end hook handoff, character arc continuity. Single-volume issues are out of scope.",
		"Every suggestion must cite facts from at least TWO different volumes/chapters in scope. Base every claim on facts visible in tool query results; do not assert an unresolved mystery or contradiction unless the queried facts show it.",
		"Use `novelgen tool query outline --type refs --entity-type storyline|character|item --name \"<name>\" --view brief` to trace a line across volumes. Cross-check setup book via story-setup queries for promise-vs-delivery gaps.",
		"Every suggestion target_id must exist in the review scope. For cross-volume issues not tied to one target, point at the most relevant volume/chapter or omit target_id.",
		"Keep review_result compact: summary under 300 Chinese characters, at most 10 suggestions, at most 4 strengths and 4 weaknesses.",
		"Return only the JSON object; no Markdown, no code fences, no extra text.",
	}
	focus := "按跨卷维度自由审查：伏笔生命周期、设定一致性漂移、跨卷重复套路、宏观节奏、卷间钩子衔接、角色跨卷弧线。"
	if strings.TrimSpace(input.UserPrompt) != "" {
		instructions = append(instructions,
			"user_prompt 是本次审查的最高优先级任务清单。围绕它逐条分析，再补充你发现的跨卷问题。",
		)
		focus = "以 user_prompt 为最高优先级；在此基础上补充伏笔生命周期、设定一致性漂移、跨卷重复套路、宏观节奏、卷间钩子衔接、角色跨卷弧线等跨卷维度。"
	}
	if input.SurfaceOnly {
		instructions = append(instructions,
			"SURFACE-ONLY REVIEW: you can ONLY see reader-visible fields (volume summary, chapter title/summary/opening_beat/scenes). Internal structure fields (events, payoff_contract, state_anchor, state_change, conflicts) are NOT available and MUST NOT be assumed or guessed. Judge the outline the way a reader experiences it: pacing, hooks, character motivation, emotional beats, foreshadowing as it reads — not as data-structure panels. Do not report issues that would be invisible to a reader (e.g. a forced data-slot value, a field format, an engine check artifact).",
		)
	}

	promptInput := composeAgentSDKOutlineReviewPromptInput{
		TargetVolumeID:  fmt.Sprintf("%d-%d (cross-volume)", input.FromVolumeIndex, input.ToVolumeIndex),
		VolumeCount:     volCount,
		ChapterCount:    chapCount,
		UserPrompt:      strings.TrimSpace(input.UserPrompt),
		ReviewFocus:     focus,
		RequiredQueries: []string{"novelgen tool query outline --type all --view index"},
		Instructions:    instructions,
	}

	allowlist := []string{
		"novelgen tool query story-setup --type search",
		"novelgen tool query story-setup --type index",
		"novelgen tool query story-setup --type all",
		"novelgen tool query story-setup --type core-cast",
		"novelgen tool query story-setup --type storyline",
		"novelgen tool query story-setup --type premise",
		"novelgen tool query story-setup --type resource",
		"novelgen tool query story-setup --type timeline",
		"novelgen tool query outline --type all --view index",
		"novelgen tool query outline --type all --view brief",
		"novelgen tool query outline --type part --id",
		"novelgen tool query outline --type refs --entity-type storyline --name",
		"novelgen tool query outline --type refs --entity-type character --name",
		"novelgen tool query outline --type refs --entity-type item --name",
		"novelgen tool query outline --type refs --entity-type location --name",
	}
	for _, v := range inScope {
		vid := strings.TrimSpace(v.ID)
		if vid == "" {
			continue
		}
		if input.SurfaceOnly {
			// Reader-visible fields only: title/summary/opening_beat/scenes.
			// No events, no payoff_contract, no state_anchor — the AI-flavor
			// review judges what a reader actually sees, not the author's
			// internal structure panels (which are routinely misread as
			// 'AI flavor' — e.g. the forced opponent-misread slot).
			allowlist = append(allowlist,
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "volume", ID: vid, Fields: "summary", View: "brief"}),
			)
			for _, chapter := range v.Chapters {
				cid := strings.TrimSpace(chapter.ID)
				if cid == "" {
					continue
				}
				allowlist = append(allowlist,
					toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: cid, Fields: "title,summary,opening_beat,scenes", View: "brief"}),
				)
			}
			continue
		}
		allowlist = append(allowlist,
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "volume", ID: vid, View: "brief"}),
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", VolumeID: vid, View: "brief"}),
		)
		for _, chapter := range v.Chapters {
			cid := strings.TrimSpace(chapter.ID)
			if cid == "" {
				continue
			}
			allowlist = append(allowlist,
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: cid, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: cid, View: "brief"}),
			)
		}
	}

	params := agentSDKToolParams(
		[]string{"novel-tools-core", "outline-review-workflow", "cross-volume-review-workflow"},
		"review the cross-volume continuity of the story outline and provide focused improvement suggestions",
		dedupeToolAllowlist(allowlist),
		ToolEvidenceRequirement{
			MinQueryCalls:        2,
			RequireNoDeniedTools: true,
			RequiredToolCommands: []string{"novelgen tool query outline --type all --view index"},
		},
		composeCrossVolumeReviewAgentSDKMaxTurns(),
		1800,
	)
	params.UserPromptDriven = strings.TrimSpace(input.UserPrompt) != ""

	var output models.ReviewResult
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return models.ReviewResult{}, err
	}
	output.NormalizeScoreScale()
	clipOutlineReviewResult(&output)
	logger.Section("Cross-Volume Agent SDK Review Result")
	logger.Info("Overall Score: %.1f/100", output.OverallScore)
	logger.Info("Suggestions: %d", len(output.Suggestions))
	return output, nil
}

func buildAgentSDKOutlineReviewPromptInput(input ComposeOutlineReviewInput) composeAgentSDKOutlineReviewPromptInput {
	volumeCount, chapterCount := countOutlineScope(input.Outline, input.VolumeID)
	instructions := []string{
		"Run the required first query exactly as given before anything else. It returns the overall structure of the review scope.",
		"This is a read-only review: do not patch, do not run check, do not write files, do not read source/RPG/Claude tool-results.",
		"Base every claim on facts visible in tool query results. Do not assert an unresolved mystery or contradiction unless the queried facts show it.",
		"Every suggestion target_id must exist in the review scope. If a suggestion is not tied to a concrete volume/chapter, omit target_id.",
		"Keep review_result compact: summary under 300 Chinese characters, at most 8 suggestions, at most 4 strengths and 4 weaknesses.",
		"Return only the JSON object; no Markdown, no code fences, no extra text.",
	}
	if strings.TrimSpace(input.VolumeID) != "" && !input.CrossVolume {
		instructions = append(instructions,
			"Cross-volume context available: the immediately adjacent volumes (previous + next) can be queried read-only (volume/events/chapter queries are allowlisted). When an issue involves a volume-end hook, a next-volume payoff, setup planted elsewhere, or continuity drift, query the adjacent volumes and cite the cross-volume evidence instead of guessing. Keep the review focused on the target volume; adjacent volumes are evidence context, not the review target.",
		)
	}
	focus := "按以下维度自由审查：结构、节奏、连贯性、人物动机、情节逻辑、信息差博弈。"
	if strings.TrimSpace(input.UserPrompt) != "" {
		instructions = append(instructions,
			"user_prompt 是本次审查的最高优先级任务清单。围绕它逐条分析，再补充你发现的其他开放性问题。",
		)
		focus = "以 user_prompt 为最高优先级；在此基础上补充结构、节奏、连贯性、人物动机、情节逻辑、信息差博弈等维度的开放性问题。"
	}
	return composeAgentSDKOutlineReviewPromptInput{
		TargetVolumeID:  strings.TrimSpace(input.VolumeID),
		VolumeCount:     volumeCount,
		ChapterCount:    chapterCount,
		UserPrompt:      strings.TrimSpace(input.UserPrompt),
		ReviewFocus:     focus,
		RequiredQueries: composeOutlineReviewRequiredQueries(input.Outline, input.VolumeID),
		Instructions:    instructions,
	}
}

func composeOutlineReviewAgentSDKParams(command string, outline models.Outline, volumeID string) InvokeParams {
	allowlist := composeOutlineReviewToolAllowlist(outline, volumeID)
	return agentSDKToolParams(
		[]string{"novel-tools-core", "outline-review-workflow"},
		command,
		dedupeToolAllowlist(allowlist),
		ToolEvidenceRequirement{
			MinQueryCalls:        1,
			RequireNoDeniedTools: true,
			RequiredToolCommands: composeOutlineReviewRequiredQueries(outline, volumeID),
		},
		composeOutlineReviewAgentSDKMaxTurns(),
		composeOutlineReviewAgentSDKTimeout(),
	)
}

// composeCrossVolumeReviewAgentSDKMaxTurns 返回跨卷 review 的 agent 回合上限。
// 默认 60：原值 45 对"一次审三卷"（3 卷 × ~30 章正文级大纲 + setup 承诺表）不够，
// 实测撞 Reached maximum number of turns (45)、无产出。可用 NOVELGEN_CROSS_REVIEW_MAX_TURNS 覆盖；
// 卷数更多时建议拆成逐个 focus 串行跑，而不是无上限加回合（回合越多越容易空转）。
func composeCrossVolumeReviewAgentSDKMaxTurns() int {
	const defaultTurns = 60
	return envutil.PositiveInt("NOVELGEN_CROSS_REVIEW_MAX_TURNS", defaultTurns)
}

// composeOutlineReviewAgentSDKTimeout 返回单次 outline review 的 agent 超时（秒）。
// 默认 1800（与 improve 路径对齐）：600 秒对本项目 10 章的大卷不够——agent 要读完本卷
// 全部章节 + setup 三张承诺表 + 相邻卷，实测每次跑到 ~9 分钟就撞 TimeoutError、无产出。
// 可用 NOVELGEN_REVIEW_TIMEOUT 覆盖。
func composeOutlineReviewAgentSDKTimeout() int {
	const defaultSeconds = 1800
	return envutil.PositiveInt("NOVELGEN_REVIEW_TIMEOUT", defaultSeconds)
}

// composeOutlineReviewAgentSDKMaxTurns 返回单次 outline review 的 agent 回合上限。
// 默认 30：review 要跨读本卷全部章节 + setup（storylines/premises/main_loop 三张承诺表）
// + 相邻卷，章节内容变厚后 20 回合常在"查证收敛"阶段被掐断（报 Reached maximum number of turns）。
// 可用 NOVELGEN_REVIEW_MAX_TURNS 覆盖。
func composeOutlineReviewAgentSDKMaxTurns() int {
	const defaultTurns = 30
	return envutil.PositiveInt("NOVELGEN_REVIEW_MAX_TURNS", defaultTurns)
}

func (a *ComposeAgent) RepairGlobalIssuesWithAgentSDK(ctx context.Context, review models.ReviewResult, applyPatches bool) (models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Global Outline Repair")
	if applyPatches {
		logger.Info("Agent apply enabled: SDK may write global fixes through validated setup/outline patch tools")
	}
	promptInput := composeAgentSDKGlobalRepairPromptInput{
		ReviewResult:     compactReviewForAgentSDKPrompt(review, ""),
		ApplyPatches:     applyPatches,
		ForceIssueRepair: true,
		RequiredQueries: []string{
			"novelgen tool query context --type outline-global-repair --view index",
		},
		Instructions: []string{
			"Use the required outline-global-repair index query first. It returns check summary, issue_context, patch_task, navigation, workflow, next_actions, and stats without full setup/outline.",
			"If patch_task is present, use patch_task.dry_run_command and patch_task.apply_command exactly. Do not run repair_context_query, outline-volume, outline-repair, chapter-repair, query outline, query chapter, source/RPG reads, or Claude tool-results reads.",
			"Follow patch_task exactly: run dry_run_command once, run apply_command once only if dry-run succeeds, then patch_task.post_patch_check_query exactly.",
			"If patch_task is absent, fall back to issue_context. Patch only entries that have both patch_query and patch_shape; patch_query may target setup or a specific outline volume.",
			"When patch_task is absent, run at most one repair_context_query, and only the exact repair_context_query from the first patchable issue_context item. Do not invent combined category names such as logic,plot,structure.",
			"Do not patch broad global issues that lack patch_shape. Return them as remaining diagnostics or route them to a smaller workflow.",
			"Patch at most one item in this invocation. When patch_task has dry_run_command/apply_command, do not reconstruct the command; run those exact commands. If falling back to issue_context, dry-run exactly once with `printf '%s' '<compact-json>' | <patch_query>` using a concrete compact JSON object; do not run placeholder text such as `<json>`.",
			"For global outline/setup repair, do not use --patch-json. For Chinese/non-ASCII patch JSON, do not run Python/Node/PowerShell/help commands to encode it.",
			"If apply_patches=true and dry-run succeeds, repeat the same stdin-piped patch with --apply, then run the provided post_patch_check_query exactly; do not replace it with a volume-level or medium-only check.",
			"Do not inspect source code, RPG files, full setup, full outline, or all chapters.",
		},
	}
	requiredPostPatchChecks := composeGlobalRepairRequiredPostPatchChecks(review)
	if len(requiredPostPatchChecks) > 0 {
		promptInput.Instructions = append(promptInput.Instructions, "Required post-patch checks before final JSON: "+strings.Join(requiredPostPatchChecks, "; "))
	}
	params := composeAgentSDKParams("repair global outline issues using project query/check/patch tools", "outline-global-repair-workflow", 18, composeGlobalRepairToolAllowlistForReview(review, applyPatches))
	params.ToolEvidence = ToolEvidenceRequirement{
		MinContextQueryCalls:   1,
		MinCheckCalls:          1,
		MaxContextQueryCalls:   2,
		DisallowQueryFullCalls: true,
		RequireNoDeniedTools:   true,
		RequiredToolCommands:   requiredPostPatchChecks,
	}
	if applyPatches {
		params.ToolEvidence.RequirePatchApplyFollowupCheck = true
	}

	var output composeAgentSDKGlobalRepairOutput
	runtimeResult, err := a.base.ExecuteWithRuntimeResult(ctx, params, promptInput, &output)
	if err != nil {
		if recovered, recoverErr := recoverAppliedGlobalAfterAgentSDKError(runtimeResult, err, requiredPostPatchChecks); recoverErr == nil {
			logger.Warn("Agent SDK global repair ended with an error after applying a validated patch; recovered saved project state: %v", err)
			return recovered, nil
		}
		return models.ReviewResult{}, err
	}
	if err := validateComposeImproveAgentSDKPatchApplyEvidence(runtimeResult, output.AppliedPatches, output.AppliedPatchCount); err != nil {
		return models.ReviewResult{}, err
	}
	if output.ReviewResult.OverallScore == 0 &&
		strings.TrimSpace(output.ReviewResult.Summary) == "" &&
		len(output.ReviewResult.Suggestions) == 0 {
		return models.ReviewResult{}, fmt.Errorf("agent SDK global repair output has empty review_result")
	}
	result := output.ReviewResult.toModelReviewResult()
	result.NormalizeScoreScale()
	return result, nil
}

func recoverAppliedGlobalAfterAgentSDKError(runtimeResult *agentruntime.Result, invokeErr error, requiredPostPatchChecks []string) (models.ReviewResult, error) {
	if runtimeResult == nil || runtimeResult.LiveSummary == nil {
		return models.ReviewResult{}, invokeErr
	}
	summary := runtimeResult.LiveSummary
	if summary.PatchApplies == 0 || summary.CheckCalls == 0 || summary.ApplyWithoutFollowupCheck > 0 {
		return models.ReviewResult{}, invokeErr
	}
	if len(requiredPostPatchChecks) > 0 && !liveSummaryHasAnyToolCommand(summary, requiredPostPatchChecks) {
		return models.ReviewResult{}, invokeErr
	}
	return models.ReviewResult{
		OverallScore: 80,
		Summary:      fmt.Sprintf("Agent SDK applied %d validated global patch(es) and completed follow-up checks, but ended with tool evidence diagnostics; recovered saved project state.", summary.PatchApplies),
		Suggestions: []models.ReviewSuggestion{{
			Category:   "agent_sdk_recovery",
			Issue:      "Agent SDK attempted disallowed or extra tools after a validated global patch.",
			Suggestion: "Recovered the saved project state because patch apply and follow-up check evidence were present.",
			Priority:   models.PriorityLow,
		}},
	}, nil
}

func validateComposeImproveAgentSDKPatchApplyEvidence(result *agentruntime.Result, appliedPatches bool, appliedPatchCount int) error {
	if result == nil || result.LiveSummary == nil {
		return nil
	}
	summary := result.LiveSummary
	if appliedPatches && summary.PatchApplies == 0 {
		return fmt.Errorf("agent SDK reported applied outline patches but live log has no patch apply call")
	}
	if appliedPatchCount > summary.PatchApplies {
		return fmt.Errorf("agent SDK reported %d applied outline patches but live log has %d patch apply call(s)", appliedPatchCount, summary.PatchApplies)
	}
	return nil
}

func (a *ComposeAgent) recoverAppliedVolumeAfterAgentSDKError(original models.Volume, runtimeResult *agentruntime.Result, invokeErr error) (ComposeAgentSDKImproveVolumeOutput, error) {
	if runtimeResult == nil || runtimeResult.LiveSummary == nil {
		return ComposeAgentSDKImproveVolumeOutput{}, invokeErr
	}
	summary := runtimeResult.LiveSummary
	if summary.PatchApplies == 0 || summary.ApplyWithoutFollowupCheck > 0 {
		return ComposeAgentSDKImproveVolumeOutput{}, invokeErr
	}
	volume, err := a.loadAgentSDKAppliedOutlineVolume(original, true)
	if err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("recover applied Agent SDK outline patch: %w; original error: %v", err, invokeErr)
	}
	for i, chapter := range volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("recovered chapter %d invalid: %w; original error: %v", i+1, err, invokeErr)
		}
	}
	review := models.ReviewResult{
		OverallScore: 80,
		Summary:      fmt.Sprintf("Agent SDK applied %d validated outline patch(es) and completed follow-up checks, but ended before final JSON; recovered saved outline.", summary.PatchApplies),
		Suggestions: []models.ReviewSuggestion{{
			Category:   "agent_sdk_recovery",
			Issue:      "Agent SDK ended before returning final JSON.",
			Suggestion: "Recovered the saved outline because patch apply and follow-up check evidence were present.",
			Priority:   models.PriorityLow,
		}},
	}
	return ComposeAgentSDKImproveVolumeOutput{ReviewResult: review, Volume: volume}, nil
}

func validateAgentSDKApplyOutput(output composeAgentSDKApplyOutput) error {
	if output.ReviewResult.OverallScore == 0 &&
		strings.TrimSpace(output.ReviewResult.Summary) == "" &&
		len(output.ReviewResult.Suggestions) == 0 {
		return fmt.Errorf("agent SDK apply output has empty review_result")
	}
	if output.AppliedPatchCount < 0 {
		return fmt.Errorf("agent SDK apply output has negative applied_patch_count")
	}
	if output.AppliedPatchCount > 1 {
		return fmt.Errorf("agent SDK apply output reports applied_patch_count=%d, want at most 1 per invocation", output.AppliedPatchCount)
	}
	if !output.AppliedPatches && output.AppliedPatchCount > 0 {
		return fmt.Errorf("agent SDK apply output reports applied_patch_count=%d while applied_patches=false", output.AppliedPatchCount)
	}
	return nil
}

func (a *ComposeAgent) loadAgentSDKAppliedOutlineVolume(original models.Volume, requireApplied bool) (models.Volume, error) {
	storage := a.project
	if storage == nil {
		if requireApplied {
			return models.Volume{}, fmt.Errorf("agent SDK applied outline patch but project root is unknown")
		}
		return original, nil
	}
	outlinePath := storage.OutlineJSON()
	outline, err := storage.LoadOutline()
	if err != nil {
		if requireApplied {
			return models.Volume{}, fmt.Errorf("agent SDK applied outline patch but reloading %s failed: %w", outlinePath, err)
		}
		return original, nil
	}
	volume := outline.GetVolumeByID(original.ID)
	if volume == nil {
		if requireApplied {
			return models.Volume{}, fmt.Errorf("agent SDK applied outline patch but target volume %q is missing from %s", original.ID, outlinePath)
		}
		return original, nil
	}
	loaded, err := cloneVolumeForAgentSDKPatch(*volume)
	if err != nil {
		return models.Volume{}, err
	}
	preserveImprovedVolumeIdentity(&original, &loaded)
	return loaded, nil
}

func validateAgentSDKImprovePatchOutput(output composeAgentSDKImprovePatchOutput, targetVolumeID string) error {
	targetVolumeID = strings.TrimSpace(targetVolumeID)
	if targetVolumeID == "" {
		return fmt.Errorf("target volume id is empty")
	}
	if strings.TrimSpace(output.VolumePatch.ID) == "" {
		return fmt.Errorf("agent SDK improve output missing volume_patch.id for target volume %q", targetVolumeID)
	}
	if strings.TrimSpace(output.VolumePatch.ID) != targetVolumeID {
		return fmt.Errorf("agent SDK improve output volume_patch.id %q does not match target volume %q", output.VolumePatch.ID, targetVolumeID)
	}
	if output.ReviewResult.OverallScore == 0 &&
		strings.TrimSpace(output.ReviewResult.Summary) == "" &&
		len(output.ReviewResult.Suggestions) == 0 {
		return fmt.Errorf("agent SDK improve output has empty review_result")
	}
	return nil
}

func applyAgentSDKVolumePatch(original models.Volume, patch composeVolumePatch, expectedChapters int) (models.Volume, error) {
	if err := utils.ValidateNoSuspiciousPatchText(patch); err != nil {
		return models.Volume{}, fmt.Errorf("agent SDK volume patch rejected: %w", err)
	}
	if id := strings.TrimSpace(patch.ID); id != "" && id != strings.TrimSpace(original.ID) {
		return models.Volume{}, fmt.Errorf("agent SDK volume patch id %q does not match target volume %q", id, original.ID)
	}
	if expectedChapters > 0 && len(original.Chapters) != expectedChapters {
		return models.Volume{}, fmt.Errorf("target volume has %d chapter(s), expected %d", len(original.Chapters), expectedChapters)
	}

	merged, err := cloneVolumeForAgentSDKPatch(original)
	if err != nil {
		return models.Volume{}, err
	}
	if strings.TrimSpace(patch.Title) != "" {
		merged.Title = patch.Title
	}
	if strings.TrimSpace(patch.Summary) != "" {
		merged.Summary = patch.Summary
	}
	if !isEmptyPayoffContract(patch.PayoffContract) {
		merged.PayoffContract = models.MergeVolumePayoffContract(merged.PayoffContract, patch.PayoffContract)
	}

	chapterByID := make(map[string]int, len(merged.Chapters))
	for i, chapter := range merged.Chapters {
		chapterByID[strings.TrimSpace(chapter.ID)] = i
	}
	for _, changed := range patch.Chapters {
		id := strings.TrimSpace(changed.ID)
		if id == "" {
			return models.Volume{}, fmt.Errorf("agent SDK volume patch includes a changed chapter without id")
		}
		idx, ok := chapterByID[id]
		if !ok {
			return models.Volume{}, fmt.Errorf("agent SDK volume patch references unknown chapter id %q", id)
		}
		merged.Chapters[idx] = mergeAgentSDKChapterPatch(merged.Chapters[idx], changed)
	}
	if err := applyAgentSDKEventPatches(&merged, patch.Events); err != nil {
		return models.Volume{}, err
	}
	if expectedChapters > 0 && len(merged.Chapters) != expectedChapters {
		return models.Volume{}, fmt.Errorf("merged volume has %d chapter(s), expected %d", len(merged.Chapters), expectedChapters)
	}
	preserveImprovedVolumeIdentity(&original, &merged)
	return merged, nil
}

func applyAgentSDKEventPatches(volume *models.Volume, patches []composeChangedEventPatch) error {
	if len(patches) == 0 {
		return nil
	}
	chapterByID := make(map[string]int, len(volume.Chapters))
	for i, chapter := range volume.Chapters {
		chapterByID[strings.TrimSpace(chapter.ID)] = i
	}
	for _, patch := range patches {
		chapterID := strings.TrimSpace(patch.ChapterID)
		if chapterID == "" {
			return fmt.Errorf("agent SDK changed_events includes an event without chapter_id")
		}
		chapterIdx, ok := chapterByID[chapterID]
		if !ok {
			return fmt.Errorf("agent SDK changed_events references unknown chapter id %q", chapterID)
		}
		if patch.EventIndex < 0 || patch.EventIndex >= len(volume.Chapters[chapterIdx].Events) {
			return fmt.Errorf("agent SDK changed_events index %d out of range for chapter %q", patch.EventIndex, chapterID)
		}
		volume.Chapters[chapterIdx].Events[patch.EventIndex] = mergeAgentSDKEventPatch(volume.Chapters[chapterIdx].Events[patch.EventIndex], patch)
	}
	return nil
}

func mergeAgentSDKEventPatch(original models.Event, patch composeChangedEventPatch) models.Event {
	merged := original
	if strings.TrimSpace(patch.Type) != "" {
		merged.Type = patch.Type
	}
	if len(patch.Characters) > 0 {
		merged.Characters = patch.Characters
	}
	if strings.TrimSpace(patch.Subject) != "" {
		merged.Subject = patch.Subject
	}
	if strings.TrimSpace(patch.Change) != "" {
		merged.Change = patch.Change
	}
	if strings.TrimSpace(patch.Details) != "" {
		merged.Details = patch.Details
	}
	if strings.TrimSpace(patch.Actor) != "" {
		merged.Actor = patch.Actor
	}
	if strings.TrimSpace(patch.Action) != "" {
		merged.Action = patch.Action
	}
	if strings.TrimSpace(patch.Target) != "" {
		merged.Target = patch.Target
	}
	if strings.TrimSpace(patch.TargetType) != "" {
		merged.TargetType = patch.TargetType
	}
	if strings.TrimSpace(patch.Context) != "" {
		merged.Context = patch.Context
	}
	if strings.TrimSpace(patch.Result) != "" {
		merged.Result = patch.Result
	}
	return merged
}

func mergeAgentSDKChapterPatch(original models.Chapter, patch composeChapterPatch) models.Chapter {
	merged := original
	merged.ID = original.ID
	if strings.TrimSpace(patch.Title) != "" {
		merged.Title = patch.Title
	}
	if strings.TrimSpace(patch.Summary) != "" {
		merged.Summary = patch.Summary
	}
	if len(patch.Characters) > 0 {
		merged.Characters = patch.Characters
	}
	if strings.TrimSpace(patch.Location) != "" {
		merged.Location = patch.Location
	}
	if len(patch.Events) > 0 {
		merged.Events = composeEventPatchesToModelEvents(patch.Events)
	}
	if len(patch.Scenes) > 0 {
		merged.Scenes = mergeOutlineScenesByOrder(merged.Scenes, patch.Scenes)
	}
	if strings.TrimSpace(patch.OpeningBeat) != "" {
		merged.OpeningBeat = patch.OpeningBeat
	}
	if strings.TrimSpace(patch.ClosingBeat) != "" {
		merged.ClosingBeat = patch.ClosingBeat
	}
	if strings.TrimSpace(patch.StateChange) != "" {
		merged.StateChange = patch.StateChange
	}
	if strings.TrimSpace(patch.Conflict) != "" {
		merged.Conflict = patch.Conflict
	}
	if strings.TrimSpace(patch.Pacing) != "" {
		merged.Pacing = patch.Pacing
	}
	if !reflect.DeepEqual(patch.Timeline, models.ChapterTimeline{}) {
		merged.Timeline = patch.Timeline
	}
	if !reflect.DeepEqual(patch.StateAnchor, models.StateAnchor{}) {
		merged.StateAnchor = patch.StateAnchor
	}
	if len(patch.Enemies) > 0 {
		merged.Enemies = patch.Enemies
	}
	if len(patch.ResourceLedger) > 0 {
		merged.ResourceLedger = patch.ResourceLedger
	}
	if !reflect.DeepEqual(patch.Mysteries, models.ChapterMysteries{}) {
		merged.Mysteries = patch.Mysteries
	}
	if len(patch.StorylineAdvances) > 0 {
		merged.StorylineAdvances = patch.StorylineAdvances
	}
	if patch.ChapterPayoff != nil && !patch.ChapterPayoff.IsZero() {
		merged.ChapterPayoff = patch.ChapterPayoff
	}
	return merged
}

func prepareAgentSDKReturnedVolume(original models.Volume, returned models.Volume, expectedChapters int) (models.Volume, error) {
	if expectedChapters > 0 {
		if len(returned.Chapters) > expectedChapters {
			returned.Chapters = returned.Chapters[:expectedChapters]
		}
		if len(returned.Chapters) != expectedChapters {
			return models.Volume{}, fmt.Errorf("agent SDK returned %d chapter(s), expected %d", len(returned.Chapters), expectedChapters)
		}
	}
	merged, err := cloneVolumeForAgentSDKPatch(original)
	if err != nil {
		return models.Volume{}, err
	}
	if strings.TrimSpace(returned.Title) != "" {
		merged.Title = returned.Title
	}
	if strings.TrimSpace(returned.Summary) != "" {
		merged.Summary = returned.Summary
	}
	if !isEmptyPayoffContract(returned.PayoffContract) {
		merged.PayoffContract = returned.PayoffContract
	}
	merged.Chapters = returned.Chapters
	preserveImprovedVolumeIdentity(&original, &merged)
	return merged, nil
}

func cloneVolumeForAgentSDKPatch(volume models.Volume) (models.Volume, error) {
	data, err := json.Marshal(volume)
	if err != nil {
		return models.Volume{}, fmt.Errorf("clone agent SDK volume: %w", err)
	}
	var cloned models.Volume
	if err := json.Unmarshal(data, &cloned); err != nil {
		return models.Volume{}, fmt.Errorf("clone agent SDK volume: %w", err)
	}
	return cloned, nil
}

func buildAgentSDKChaptersPromptInput(input ComposeChaptersInput) composeAgentSDKChaptersPromptInput {
	required := []string{
		toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: input.Volume.ID, View: "brief"}),
	}
	previousID := ""
	if input.PreviousVolume != nil && strings.TrimSpace(input.PreviousVolume.ID) != "" {
		previousID = input.PreviousVolume.ID
	}
	return composeAgentSDKChaptersPromptInput{
		TargetPartID:     input.Part.ID,
		TargetVolumeID:   input.Volume.ID,
		TargetVolumeName: input.Volume.Title,
		VolumeIndex:      input.VolumeIndex,
		TotalVolumes:     input.TotalVolumes,
		ChaptersPerVol:   input.ChaptersPerVol,
		PreviousVolumeID: previousID,
		RequiredQueries:  required,
		Instructions: []string{
			"Use the required context query to fetch compact story setup, target volume, neighboring volumes, entity index, events, and navigation.",
			"After the brief queries, use targeted detail queries only when needed: chapter by id, events by chapter id, craft by exact name.",
			"Do not ask for, inspect, or modify source files.",
			"Return exactly one volume object with exactly chapters_per_volume chapters.",
			"Do not invent or change project-wide facts that are absent from tool query results.",
		},
	}
}

func buildAgentSDKImproveVolumePromptInput(input ComposeImproveVolumeInput, applyPatches bool, forceIssueRepair bool) composeAgentSDKImproveVolumePromptInput {
	required := []string{
		toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: input.Volume.ID, View: "index"}),
	}
	crossVolume := len(input.CrossVolumeIDs) > 0
	hasFocusedTasks := len(input.ReviewResult.Suggestions) > 0 || strings.TrimSpace(input.UserPrompt) != ""
	instructions := []string{
		"Use the required context query before reviewing. It returns an index-sized target volume route, check summary, navigation, workflow, next_actions, and stats.",
		"If review_result.suggestions contain navigation, execute repair_route_query first when present. It returns an index-sized route and next_actions. Execute repair_context_query only when the route indicates detailed facts are needed. If route/context are absent, execute detail_queries or focused_check_query before broadening context; treat these focused commands as the primary task list.",
		"If review_result contains suggestions without navigation, query only the referenced target IDs unless more context is necessary.",
	}
	if crossVolume {
		instructions = append(instructions,
			fmt.Sprintf("CROSS-VOLUME MODE: this session may query, check, and patch the target volume %s AND all listed cross-volume volumes: %v. Cross-volume issues (e.g. a volume-end hook that must be resolved at the start of the next volume, a setup planted in one volume and paid off in another, consistency drift across volumes) should be fixed consistently across the involved volumes IN THIS SAME SESSION, not deferred. Use `novelgen tool query outline --type refs --entity-type storyline|character|item|location --name \"<name>\" --view brief` to trace lines across volumes when needed. Every patched volume still needs a check command run for it (volume or chapter scope) before finishing.", input.Volume.ID, strings.Join(input.CrossVolumeIDs, ",")),
		)
	} else {
		instructions = append(instructions,
			"Single-volume mode: only the target_volume_id may be queried, checked, and patched. Do not patch other volumes.",
		)
	}
	if strings.TrimSpace(input.UserPrompt) != "" {
		userPromptInstruction := "Treat user_prompt as already scoped to target_volume_id. If it refers to a multi-volume user request, this invocation must execute only the target_volume_id portion. Do not query, check, or patch any other volume."
		if crossVolume {
			userPromptInstruction = "Treat user_prompt as scoped to the target volume plus the listed cross-volume volumes. Multi-volume user requests should be executed across the involved volumes in this session."
		}
		instructions = append(instructions,
			userPromptInstruction,
			"user_prompt requests are explicit tasks. If they describe concrete narrative changes (character motivation, scene beats, foreshadowing, wording, mystery clues), implement them in volume_patch fields (summary/opening_beat/scenes/events/mysteries) even when scoped checks are clean. Do not decline creative changes as 'not check-verifiable'; the user request is the task list.",
		)
		if composePromptRequestsCheckFirstNoop(input.UserPrompt) {
			instructions = append(instructions,
				"The user_prompt asks for a check-first/no-op-if-clean run. After the required index query, run the target_volume_id check before any brief/full detail query. If that check returns zero issues, return final JSON immediately; do not run extra context just to keep reviewing.",
			)
		}
	}
	if !hasFocusedTasks {
		noTaskInstruction := "No focused review suggestions or user repair request were supplied. In this case, run the required context query, at most one same-volume brief context query if needed, and the required medium+ volume check; do not run low-priority broad checks, do not run full context, and do not sweep chapter-by-chapter."
		if applyPatches {
			noTaskInstruction += " If the medium+ check is clean, return applied_patches=false and applied_patch_count=0."
		} else {
			noTaskInstruction += " If the medium+ check is clean, return an empty patch."
		}
		instructions = append(instructions,
			noTaskInstruction,
		)
	}
	if forceIssueRepair {
		instructions = append(instructions,
			"Treat review_result.suggestions as an explicit repair task list even when they are low priority. Build and validate a minimal patch for the directly fixable issues you can verify from focused tool results.",
			"Do not dismiss low-priority focused issues only because the medium+ scoped check passes. If you decide a focused issue is a false positive, cite the exact queried facts in review_result.suggestions and leave it as a remaining false-positive note instead of saying it was fixed.",
			"When apply_patches=true and at least one focused issue is directly fixable, prefer a small dry-run/apply/check cycle over returning an empty patch.",
		)
	}
	detailInstruction := "If a focused issue, user prompt, or failed medium+ check requires more facts, use targeted detail queries only when needed: repair route/context first, then chapter by id, events by chapter id, or craft by exact name."
	fieldSyncInstruction := "FIELD-SYNC RULE: when a change affects a chapter's narrative (a character's fate, a timeline/date, a fight outcome, an object's species/name, a death, a mystery payoff), apply it consistently to EVERY reader-visible field of that chapter and its volume: summary, opening_beat, scenes[*].beats, events[*].details/result, chapter_payoff (clever_move/reward/hook), state_change, state_anchor notes, AND the volume-level summary/payoff_contract if it references the same fact. A chapter whose summary says one thing while its events say another is a continuity bug, not a patch. After patching, re-check the patched chapters' events/beats for stale references to the old fact (e.g. a character who died in the summary but is still alive in events) and fix them in the same session."
	setupRulesInstruction := "SETUP-RULES CHECK: before patching, query story-setup rules (novelgen tool query story-setup --type search with keyword 'rules' or --view brief) and make sure your patch does not violate them. Key iron rules in this book: (1) 日志失效机制=信息差——日志从不失灵/乱码/被屏蔽/低能耗，读取不到信息只可能是对手反推演制造盲区、环境特性、或物理痕迹锚定; 严禁系统自缚(冷却/解锁/权限/能量消耗)与外部物理屏蔽. (2) 能力成长=摸索而非解锁——新能力是主角自己摸索/代价换来, 严禁'系统恰好解锁'. (3) 金手指代价落于关系/资源/时间/暴露, 而非系统惩罚. If your patch would introduce any of these banned mechanisms, rewrite it."
	instructions = append(instructions, fieldSyncInstruction, setupRulesInstruction)
	if hasFocusedTasks {
		detailInstruction = "After the index query, use targeted detail queries only when needed: same target volume via `novelgen tool query context --type outline-volume --id <target_volume_id> --view brief`, repair context, chapter by id, events by chapter id, craft by exact name."
	}
	if !crossVolume {
		if boundaryIDs := composeImproveBoundaryChapterIDs(input.Volume, input.UserPrompt); len(boundaryIDs) > 0 {
			instructions = append(instructions,
				fmt.Sprintf("The user prompt is boundary-scoped. Prefer direct chapter/event detail queries only for these target chapter IDs: %s. Query other chapters only when a check result or next_actions explicitly names them.", strings.Join(boundaryIDs, ", ")),
				"For a boundary-scoped chapter summary/title/beat patch, do not query story-setup, character refs, cross-volume context, or unrelated chapters. Use the target volume index, target volume brief, target chapter detail, and required focused check as sufficient evidence unless next_actions names a narrower repair context.",
				"If the user prompt names a character, item, location, skill, or proper noun that is absent from the target chapter/volume facts, verify it once with an exact refs or story-setup search. If both focused facts and the verification query show no match, do not patch; return a compact review_result note that the user prompt conflicts with project facts.",
			)
			if checks := composeImproveRequiredFocusedChecks(input.Volume, input.UserPrompt); len(checks) > 0 {
				instructions = append(instructions,
					"Required focused checks before final JSON: "+strings.Join(checks, "; "),
				)
			}
		}
	}
	instructions = append(instructions,
		detailInstruction,
		"If continuity facts require cross-volume context, you may read the payoff_contract/summary of the immediately adjacent volumes only (the allowed query list includes them). Do not query non-adjacent volumes and never patch an adjacent volume.",
		"Review and, only when needed, improve target_volume_id. Do not change any other volume.",
		composeAgentSDKPatchInstruction(applyPatches),
		composeAgentSDKCheckInstruction(applyPatches),
		composeAgentSDKReturnInstruction(applyPatches),
		"Keep review_result compact: summary under 500 Chinese characters and at most 8 suggestions, focused on remaining issues or applied patch targets.",
		composeAgentSDKWriteInstruction(applyPatches),
	)
	return composeAgentSDKImproveVolumePromptInput{
		TargetPartID:     input.Part.ID,
		TargetVolumeID:   input.Volume.ID,
		TargetVolumeName: input.Volume.Title,
		ChapterCount:     len(input.Volume.Chapters),
		ReviewResult:     compactReviewForAgentSDKPrompt(input.ReviewResult, input.Volume.ID),
		UserPrompt:       input.UserPrompt,
		ApplyPatches:     applyPatches,
		ForceIssueRepair: forceIssueRepair,
		RequiredQueries:  required,
		Instructions:     instructions,
	}
}

func composeAgentSDKPatchInstruction(applyPatches bool) string {
	const patchCommand = "novelgen tool patch outline --target volume --id <target_volume_id>"
	jsonInput := agentSDKNonASCIIPatchRule(patchCommand, true)
	if applyPatches {
		return "If you build a non-empty candidate patch JSON, first dry-run it with `printf '%s' '<compact-json>' | " + patchCommand + "`; only after a successful dry-run, repeat the same stdin-piped patch command with `--apply`. Apply at most one successful outline volume patch in this invocation. " + jsonInput
	}
	return "If you build a non-empty volume_patch, dry-run it with `printf '%s' '<compact-json>' | " + patchCommand + "` and do not use --apply. " + jsonInput
}

func composeAgentSDKCheckInstruction(applyPatches bool) string {
	if applyPatches {
		return "After a successful --apply, run `novelgen tool check all --target outline --scope volume --id <target_volume_id> --min-priority medium --max-issues 12`, summarize remaining issues, and return final JSON. Do not start another dry-run/apply cycle in the same invocation."
	}
	return "Use the quality/simulation result embedded in patch dry-run output as the validation signal."
}

func composeAgentSDKReturnInstruction(applyPatches bool) string {
	if applyPatches {
		return "Return only review_result, applied_patches, applied_patch_count, and final_check. Do not return volume_patch; Go reloads the saved outline after validated tool apply."
	}
	return "Return review_result and volume_patch. volume_patch.changed_chapters must contain only chapters you actually changed, and may include only changed fields plus the chapter id. Use volume_patch.changed_events for individual event field changes with chapter_id and 0-based event_index. If no changes are needed, return empty changed_chapters/changed_events arrays instead of echoing the whole volume."
}

func composeAgentSDKWriteInstruction(applyPatches bool) string {
	if applyPatches {
		return "Do not inspect source code or write files directly; do not read Claude tool-results temporary files. If you need more detail, rerun a narrower `novelgen tool query/check`. " + agentSDKOnlyAllowedWriteSentence("novelgen tool patch outline --target volume ...")
	}
	return "Do not ask for, inspect, or modify source files. Go will merge and save the returned patch."
}

func compactReviewForAgentSDKPrompt(review models.ReviewResult, volumeID string) composeAgentSDKPromptReviewResult {
	compact := composeAgentSDKPromptReviewResult{
		OverallScore: review.OverallScore,
		Summary:      clipForPrompt(review.Summary, 700),
	}
	for _, suggestion := range review.Suggestions {
		brief := composeAgentSDKPromptReviewSuggestion{
			Category:   strings.TrimSpace(suggestion.Category),
			TargetID:   strings.TrimSpace(suggestion.TargetID),
			TargetName: clipForPrompt(suggestion.TargetName, maxReviewTargetNameRunes),
			Issue:      clipForPrompt(suggestion.Issue, maxReviewIssueRunes),
			Suggestion: clipForPrompt(suggestion.Suggestion, maxReviewSuggestionRunes),
			Priority:   strings.TrimSpace(suggestion.Priority),
			Navigation: buildComposeAgentSDKSuggestionNavigation(volumeID, suggestion),
		}
		compact.Suggestions = append(compact.Suggestions, brief)
	}
	return compact
}

func buildComposeAgentSDKSuggestionNavigation(volumeID string, suggestion models.ReviewSuggestion) *composeAgentSDKSuggestionNavigation {
	volumeID = strings.TrimSpace(volumeID)
	targetID := strings.TrimSpace(suggestion.TargetID)
	category := composeAgentSDKCheckCategory(suggestion.Category)
	if volumeID == "" {
		nav := &composeAgentSDKSuggestionNavigation{
			TargetKind:         "outline.global",
			FocusedCheckQuery:  toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "all", Category: category, MinPriority: "low", MaxIssues: 12}),
			RepairRouteQuery:   toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-global-repair", Name: category, View: "index"}),
			RepairContextQuery: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-global-repair", Name: category, View: "brief"}),
		}
		if category == "faction_tier" && targetID != "" && !strings.EqualFold(targetID, "global") {
			nav.TargetKind = "setup.faction_tier"
			nav.DetailQueries = []string{
				toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: targetID, View: "index"}),
				toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: targetID, View: "brief"}),
			}
			nav.RepairRouteQuery = toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: targetID, View: "index"})
			nav.RepairContextQuery = toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: targetID, View: "brief"})
			nav.PatchQuery = "novelgen tool patch setup"
			nav.PatchShape = map[string]interface{}{
				"premises": []map[string]string{{
					"name":        targetID + " faction tier ladder",
					"category":    targetID,
					"description": "<define stable tier ladder for " + targetID + ">",
				}},
			}
		}
		return nav
	}
	if targetID == "" || strings.EqualFold(targetID, "global") {
		return nil
	}
	if targetID == volumeID {
		return &composeAgentSDKSuggestionNavigation{
			TargetKind: "volume",
			VolumeID:   volumeID,
			DetailQueries: []string{
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: volumeID, View: "brief"}),
			},
			FocusedCheckQuery:  toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: volumeID, Category: category, MinPriority: "low", MaxIssues: 12}),
			RepairRouteQuery:   toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: volumeID, Name: category, View: "index"}),
			RepairContextQuery: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: volumeID, Name: category, View: "brief"}),
			PatchQuery:         toolcmd.PatchOutlineVolume(volumeID),
		}
	}
	if strings.HasPrefix(targetID, volumeID+"-") {
		return &composeAgentSDKSuggestionNavigation{
			TargetKind: "chapter",
			VolumeID:   volumeID,
			DetailQueries: []string{
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: targetID, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: targetID, Fields: "result,details,target,target_type,actor,action", View: "brief"}),
			},
			FocusedCheckQuery:  toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "chapter", ID: targetID, Category: category, MinPriority: "low", MaxIssues: 8}),
			RepairRouteQuery:   toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: targetID, Name: category, View: "index"}),
			RepairContextQuery: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: targetID, Name: category, View: "brief"}),
			PatchQuery:         toolcmd.PatchOutlineVolume(volumeID),
			PatchShape:         composeAgentSDKOutlineChapterPatchShape(targetID, suggestion),
		}
	}
	return nil
}

func composeAgentSDKOutlineChapterPatchShape(targetID string, suggestion models.ReviewSuggestion) map[string]interface{} {
	chapterPatch := map[string]string{"id": targetID}
	if composeAgentSDKIssueNeedsOpeningBeat(suggestion) {
		chapterPatch["opening_beat"] = "<rewrite the chapter opening beat; for continuity include 继续/随后/紧接着, and for location moves include 来到/前往/到达>"
	}
	return map[string]interface{}{
		"changed_chapters": []map[string]string{chapterPatch},
	}
}

func composeAgentSDKIssueNeedsOpeningBeat(suggestion models.ReviewSuggestion) bool {
	category := strings.TrimSpace(strings.ToLower(suggestion.Category))
	text := strings.Join([]string{suggestion.Issue, suggestion.Suggestion}, " ")
	return category == "transition" ||
		strings.Contains(text, "章节开场") ||
		strings.Contains(text, "开场节拍") ||
		strings.Contains(text, "缺少与上一章的过渡") ||
		strings.Contains(text, "缺少过渡描述") ||
		strings.Contains(text, "地点从")
}

func composeAgentSDKCheckCategory(category string) string {
	category = strings.TrimSpace(strings.ToLower(category))
	if category == "" {
		return "all"
	}
	return category
}

// IterateHierarchicalAgentSDK runs the per-volume review/improve loop through
// Claude Agent SDK workflows. The SDK agent can query project facts, while Go
// keeps validation, merge, checkpoint, and writes.
// crossOutline, when non-nil, is the full outline used for cross-volume
// adjacency in apply mode. It can differ from outline when the caller scoped
// the run to a subset of volumes (outline only contains the selected volumes,
// while crossOutline still contains their neighbors).
func (a *ComposeAgent) IterateHierarchicalAgentSDK(ctx context.Context, outline *models.Outline, crossOutline *models.Outline, initialReview *models.ReviewResult, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup, applyPatches bool, allVolumes bool) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Hierarchical Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: SDK workflow will apply targeted fixes even when score passes")
	}
	if applyPatches {
		logger.Info("Agent apply enabled: SDK workflow may write through validated outline patch tools")
	}

	if outline == nil {
		return nil, nil, fmt.Errorf("outline is nil")
	}
	currentOutline := *outline
	finalReview := initialReview

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Agent SDK iteration %d/%d ===", i, maxIterations)

		volumesToImprove := generatedVolumeIndices(&currentOutline)
		if len(volumesToImprove) == 0 {
			logger.Info("No generated volumes to improve")
			break
		}

		improvedOutline, review, improvedCount, err := a.improveVolumesWithCheckpointAgentSDK(ctx, &currentOutline, volumesToImprove, finalReview, i, maxIterations, qualityThreshold, forceImprove, userPrompt, setup, applyPatches, crossOutline, allVolumes)
		if err != nil {
			return nil, finalReview, fmt.Errorf("iteration %d failed: %w", i, err)
		}
		currentOutline = *improvedOutline
		if review != nil {
			finalReview = review
		}
		if improvedCount == 0 && !forceImprove {
			logger.Info("Agent SDK review found no blocking fixes; stopping iteration loop")
			break
		}
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
	}

	return &currentOutline, finalReview, nil
}

// RepairByReviewAgentSDK applies an existing quality-gate review through the
// SDK volume workflow.
func (a *ComposeAgent) RepairByReviewAgentSDK(ctx context.Context, outline *models.Outline, reviewResult *models.ReviewResult, setup *models.StorySetup, applyPatches bool, crossOutline *models.Outline, allVolumes bool) (*models.Outline, error) {
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

	logger.Info("Repairing %d volume(s) with Agent SDK workflow", len(volumesToImprove))
	improved, _, _, err := a.improveVolumesWithCheckpointAgentSDK(ctx, outline, volumesToImprove, reviewResult, 0, 1, 80.0, true, "", setup, applyPatches, crossOutline, allVolumes)
	return improved, err
}

func (a *ComposeAgent) improveVolumesWithCheckpointAgentSDK(ctx context.Context, outline *models.Outline, volumesToImprove [][2]int, reviewResult *models.ReviewResult, currentIteration int, totalIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup, applyPatches bool, crossOutline *models.Outline, allVolumes bool) (*models.Outline, *models.ReviewResult, int, error) {
	currentOutline := *outline
	root, progressPath, err := a.composeImproveProgress()
	if err != nil {
		return nil, nil, 0, err
	}
	targetVolumes := improveTargetVolumeIDs(outline, volumesToImprove)
	var finalReview *models.ReviewResult

	var progress *ImproveProgress
	if root.Exists(progressPath) {
		loadedProgress, err := a.loadImproveProgress(root, progressPath)
		if err == nil && loadedProgress.Iteration == currentIteration && equalStringSlices(loadedProgress.TargetVolumes, targetVolumes) {
			logger.Info("[resume] Found existing Agent SDK progress for iteration %d, resuming...", currentIteration)
			progress = loadedProgress
			currentOutline = progress.Outline
			if !isEmptyReviewResult(&progress.ReviewResult) {
				finalReview = &progress.ReviewResult
			}
			logger.Info("[ok] Resumed from checkpoint: %d/%d volumes completed", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
	}

	if progress == nil {
		progress = &ImproveProgress{
			Iteration:        currentIteration,
			TotalIterations:  totalIterations,
			CurrentVolumeIdx: 0,
			TotalVolumes:     len(volumesToImprove),
			TargetVolumes:    targetVolumes,
			CompletedVolumes: []string{},
			Outline:          currentOutline,
		}
		if reviewResult != nil {
			progress.ReviewResult = *reviewResult
		}
		if err := a.saveImproveProgress(root, progress, progressPath); err != nil {
			logger.Warn("Failed to save initial Agent SDK progress: %v", err)
		}
	}

	improvedCount := 0
	for idx := progress.CurrentVolumeIdx; idx < len(volumesToImprove); idx++ {
		// Full-book mode hands the whole book-level task to the FIRST volume's
		// session (it may patch every volume); the remaining volumes then run
		// as ordinary per-volume sessions to avoid every session re-fixing the
		// same book-level issues.
		fullBookSession := allVolumes && idx == 0
		indices := volumesToImprove[idx]
		partIdx, volIdx := indices[0], indices[1]
		if partIdx < 0 || partIdx >= len(currentOutline.Parts) || volIdx < 0 || volIdx >= len(currentOutline.Parts[partIdx].Volumes) {
			logger.Warn("Skipping invalid Agent SDK volume target part=%d volume=%d", partIdx+1, volIdx+1)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}
		part := &currentOutline.Parts[partIdx]
		volume := &part.Volumes[volIdx]
		if len(volume.Chapters) == 0 {
			logger.Warn("Skipping empty future volume target %s", volume.ID)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}

		logger.Info("Agent SDK reviewing Volume %d.%d: %s (%d/%d)", partIdx+1, volIdx+1, volume.Title, idx+1, len(volumesToImprove))
		var volumeReview models.ReviewResult
		if reviewResult != nil {
			volumeReview = a.filterReviewForVolume(reviewResult, volume.ID)
			logger.Info("Agent SDK volume %s received %d filtered review issue(s)", volume.ID, len(volumeReview.Suggestions))
		}
		// Skip volumes with no medium/high/critical suggestions: there is
		// nothing for the agent to do. forceImprove (--force or selected
		// volume scope) no longer disables this skip - a volume without
		// suggestions has no tasks regardless of force, so running a full
		// agent session on it only burns tokens.
		if reviewResult != nil && !hasMediumOrHigherReviewSuggestion(volumeReview) {
			logger.Info("[skip] Agent SDK skipping %s: no medium/high/critical volume issues after pre-check", volume.Title)
			progress.CompletedVolumes = append(progress.CompletedVolumes, volume.ID)
			progress.CurrentVolumeIdx = idx + 1
			progress.Outline = currentOutline
			if err := a.saveImproveProgress(root, progress, progressPath); err != nil {
				logger.Warn("Failed to save Agent SDK progress after skip: %v", err)
			}
			if err := a.appendComposeImproveReport(ComposeImproveReportEntry{
				Iteration:   currentIteration,
				VolumeID:    volume.ID,
				VolumeTitle: volume.Title,
				Skipped:     true,
				Summary:     "Pre-check found no medium/high/critical volume issues; skipped.",
			}); err != nil {
				logger.Warn("Failed to append improve report entry: %v", err)
			}
			continue
		}
		improveInput := ComposeImproveVolumeInput{
			Outline:      currentOutline,
			Part:         *part,
			Volume:       *volume,
			ReviewResult: volumeReview,
		}
		if fullBookSession {
			// Full-book mode: keep the user prompt unscoped so the session knows
			// this is a book-level task spanning many volumes, and let the
			// session see ALL suggestions (not just its own target), so a
			// book-level issue (e.g. a cost that inflates across 13 volumes)
			// can be rooted out in one session.
			bookLevelInstruction := "FULL-BOOK CROSS-VOLUME SESSION: you are authorized to query, check, and patch EVERY volume of the outline (P1-V1 through P5-V5), not just the target volume. The suggestions below are book-level issues spanning multiple volumes. For each one: locate ALL affected volumes/chapters, patch them consistently in this session, and run checks for every volume you touch. Do NOT defer issues to another volume's session, do NOT mark book-level suggestions as 'outside this volume's boundary' — the boundary is the whole book. If a suggestion references specific chapter IDs in other volumes, query and patch those too."
			if strings.TrimSpace(userPrompt) != "" {
				improveInput.UserPrompt = bookLevelInstruction + "\n\n" + userPrompt
			} else {
				improveInput.UserPrompt = bookLevelInstruction
			}
			if reviewResult != nil {
				improveInput.ReviewResult = *reviewResult
			}
		} else {
			improveInput.UserPrompt = scopeComposeAgentSDKVolumeUserPrompt(userPrompt, volume.ID, volume.Title)
			improveInput.ReviewResult = volumeReview
		}
		// Cross-volume by default (apply mode only): the session may also patch
		// adjacent volumes (previous + next), so volume-end hooks and next-volume
		// payoffs can be fixed consistently in one session instead of being
		// deferred as "outside this volume's boundary". Non-apply mode merges a
		// single volume_patch in Go, so cross-volume patching is only available
		// when the agent writes through tool patch --apply.
		if applyPatches {
			adjacencyOutline := currentOutline
			if crossOutline != nil {
				adjacencyOutline = *crossOutline
			}
			if fullBookSession {
				// Full-book cross-volume mode: every volume in the outline is
				// patchable in this session, so book-level issues (a golden
				// finger cost that inflates across 13 volumes, a character
				// whose intelligence drifts per-volume) can be rooted out in
				// one session instead of being chased volume by volume.
				allIDs := make([]string, 0)
				for _, part := range adjacencyOutline.Parts {
					for _, v := range part.Volumes {
						if strings.TrimSpace(v.ID) != "" {
							allIDs = append(allIDs, v.ID)
						}
					}
				}
				improveInput.CrossVolumeIDs = allIDs
				improveInput.Outline = adjacencyOutline
			} else if adjacent := composeAdjacentVolumeIDs(adjacencyOutline, volume.ID); len(adjacent) > 0 {
				improveInput.CrossVolumeIDs = adjacent
				// The session must see the full outline so the cross-volume
				// patch/query allowlist expansion can find the adjacent volumes.
				improveInput.Outline = adjacencyOutline
			}
		}
		if setup != nil {
			improveInput.Setup = *setup
		}

		forceIssueRepair := shouldForceAgentSDKIssueRepair(forceImprove, improveInput.UserPrompt, volumeReview)
		improveOutput, err := a.ImproveVolumeWithAgentSDK(ctx, improveInput, applyPatches, forceIssueRepair)
		if err != nil {
			progress.CurrentVolumeIdx = idx
			progress.Outline = currentOutline
			if saveErr := a.saveImproveProgress(root, progress, progressPath); saveErr != nil {
				logger.Warn("Failed to save Agent SDK progress on error: %v", saveErr)
			}
			return nil, finalReview, improvedCount, fmt.Errorf("failed to improve volume %d.%d: %w", partIdx+1, volIdx+1, err)
		}

		review := improveOutput.ReviewResult
		review.Iteration = currentIteration
		review.NormalizeScoreScale()
		finalReview = &review
		progress.ReviewResult = review

		improvedVolume := improveOutput.Volume
		patchChanged := agentSDKVolumeChanged(volume, &improvedVolume)
		if patchChanged {
			part.Volumes[volIdx] = improvedVolume
			improvedCount++
			logger.Info("[ok] Agent SDK applied volume fixes: %s", improvedVolume.Title)
		} else {
			scoreMeetsThreshold := review.OverallScore >= qualityThreshold
			hasBlockingSuggestions := review.HasBlockingSuggestions()
			if scoreMeetsThreshold && !hasBlockingSuggestions && (!forceImprove || !forceIssueRepair) {
				logger.Info("[ok] Agent SDK review passed for %s (%.1f >= %.1f); no effective volume changes", volume.Title, review.OverallScore, qualityThreshold)
			} else {
				logger.Info("[warn] Agent SDK returned no volume changes for %s (score %.1f, blocking=%t)", volume.Title, review.OverallScore, hasBlockingSuggestions)
			}
		}

		progress.CompletedVolumes = append(progress.CompletedVolumes, volume.ID)
		progress.CurrentVolumeIdx = idx + 1
		progress.Outline = currentOutline
		if err := a.saveImproveProgress(root, progress, progressPath); err != nil {
			logger.Warn("Failed to save Agent SDK progress: %v", err)
		} else {
			logger.Info("[save] Agent SDK progress saved (%d/%d volumes completed)", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
		if err := a.appendComposeImproveReport(ComposeImproveReportEntry{
			Iteration:           currentIteration,
			VolumeID:            volume.ID,
			VolumeTitle:         volume.Title,
			Score:               review.OverallScore,
			Changed:             patchChanged,
			Summary:             review.Summary,
			RemainingMediumPlus: countMediumOrHigherReviewSuggestions(review),
		}); err != nil {
			logger.Warn("Failed to append improve report entry: %v", err)
		}
	}

	_ = root.Remove(progressPath)
	logger.Info("[ok] Agent SDK iteration %d complete. Progress file removed.", currentIteration)
	return &currentOutline, finalReview, improvedCount, nil
}

func shouldForceAgentSDKIssueRepair(forceImprove bool, userPrompt string, review models.ReviewResult) bool {
	if !forceImprove {
		return false
	}
	return strings.TrimSpace(userPrompt) != "" || len(review.Suggestions) > 0
}

func scopeComposeAgentSDKVolumeUserPrompt(userPrompt, volumeID, volumeTitle string) string {
	prompt := strings.TrimSpace(userPrompt)
	if prompt == "" {
		return ""
	}
	localized := composeVolumeIDListPattern.ReplaceAllString(prompt, nonEmpty(volumeID, "target_volume_id"))
	localized = composeChineseVolumeListPattern.ReplaceAllString(localized, "当前卷")
	localized = composeChineseVolumePattern.ReplaceAllString(localized, "当前卷")

	target := strings.TrimSpace(volumeID)
	if title := strings.TrimSpace(volumeTitle); title != "" {
		if target != "" {
			target += " / "
		}
		target += title
	}
	if target == "" {
		target = "target_volume_id"
	}
	return fmt.Sprintf("当前调用只处理 %s。已将用户请求局部化到当前卷；如果原始请求包含多个卷，只执行当前卷这一份，不要查询、检查或 patch 其它卷。局部化用户请求：%s", target, localized)
}

func agentSDKVolumeChanged(original *models.Volume, improved *models.Volume) bool {
	if original == nil || improved == nil {
		return false
	}
	preserveImprovedVolumeIdentity(original, improved)
	return !reflect.DeepEqual(*original, *improved)
}

// GenerateChaptersHierarchicalAgentSDK generates chapters one volume at a time
// through SDK workflows and preserves Go-owned IDs/checkpoints.
func (a *ComposeAgent) GenerateChaptersHierarchicalAgentSDK(ctx context.Context, setup models.StorySetup, structure models.StoryStructure, outline *models.Outline, onVolumeComplete func(*models.Outline, int, int, int)) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	totalVolumes := structure.TargetParts * structure.TargetVolumes
	volumeCount := 0

	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			volumeCount++
			volume := &outline.Parts[partIdx].Volumes[volIdx]
			if len(volume.Chapters) > 0 {
				logger.Info("[ok] Volume %d.%d: %s - already has %d chapters (skipping)",
					partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))
				continue
			}

			logger.Info("Agent SDK generating chapters for Volume %d.%d: %s",
				partIdx+1, volIdx+1, volume.Title)

			var outlineContext string
			if partIdx > 0 || volIdx > 0 {
				outlineContext = a.buildHierarchicalContext(outline, partIdx, volIdx)
			}

			var previousVolume *models.Volume
			if volIdx > 0 {
				previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
			} else if partIdx > 0 {
				prevPart := outline.Parts[partIdx-1]
				if len(prevPart.Volumes) > 0 {
					previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
				}
			}

			output, err := a.GenerateChaptersForVolumeWithAgentSDK(ctx, ComposeChaptersInput{
				Setup:          setup,
				Part:           outline.Parts[partIdx],
				Volume:         *volume,
				VolumeIndex:    volumeCount,
				TotalVolumes:   totalVolumes,
				ChaptersPerVol: structure.TargetChapters,
				PreviousVolume: previousVolume,
				OutlineContext: outlineContext,
			})
			if err != nil {
				return outline, fmt.Errorf("failed to generate chapters for volume %d.%d: %w", partIdx+1, volIdx+1, err)
			}

			outline.Parts[partIdx].Volumes[volIdx] = output.Volume
			logic.NewIDManager(outline).AssignIDsToOutline()
			logger.Info("[ok] Agent SDK volume %d.%d: %s - %d chapters generated",
				partIdx+1, volIdx+1, output.Volume.Title, len(output.Volume.Chapters))

			if onVolumeComplete != nil {
				onVolumeComplete(outline, partIdx, volIdx, volumeCount)
			}
		}
	}

	logic.NewIDManager(outline).AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")
	return outline, nil
}
