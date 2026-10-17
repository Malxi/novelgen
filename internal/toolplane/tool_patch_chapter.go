// Chapter patches: prose validation (length and structure) around the saved chapter markdown.

package toolplane

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"
	"novelgen/internal/toolcmd"
	"novelgen/internal/utils"
)

type toolChapterPatch struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content"`
}

func runToolChapterPatch(cmd *cobra.Command, root *store.Root, id string, patchBytes []byte, apply bool, warnings []string) error {
	result, err := runChapterMutation(cmd.Context(), root, id, patchBytes, MutationOptions{
		Apply:          apply,
		Validate:       true,
		Mode:           MutationMerge,
		TargetWords:    ToolPatchFlags.TargetWords,
		RefreshDerived: ToolPatchFlags.RefreshDerived,
		Source:         "tool-cli",
	}, warnings)
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func runChapterMutation(ctx context.Context, root *store.Root, id string, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	return runChapterMutationForChapter(ctx, root, id, nil, patchBytes, opts, warnings)
}

func runChapterMutationForChapter(ctx context.Context, root *store.Root, id string, target *models.Chapter, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	var result *ToolPatchResult
	err := root.WithMutationLock(func() error {
		var err error
		result, err = runChapterMutationLocked(ctx, root, id, target, patchBytes, opts, warnings)
		return err
	})
	return result, err
}

func runChapterMutationLocked(ctx context.Context, root *store.Root, id string, target *models.Chapter, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	opts = opts.normalized()
	var chapter *models.Chapter
	if target != nil {
		chapter = target
		if chapter.ID != "" && chapter.ID != id {
			return nil, fmt.Errorf("chapter %q does not match target chapter %q", chapter.ID, id)
		}
	} else {
		outline, err := root.LoadOutline()
		if err != nil {
			return nil, fmt.Errorf("failed to load outline: %w", err)
		}
		chapter = outline.GetChapterByID(id)
		if chapter == nil {
			return nil, fmt.Errorf("chapter %q not found in outline", id)
		}
	}
	var patch toolChapterPatch
	if err := decodeStrictToolPatch(patchBytes, &patch); err != nil {
		return nil, fmt.Errorf("failed to parse chapter patch: %w", err)
	}
	if strings.TrimSpace(patch.ID) != "" && strings.TrimSpace(patch.ID) != id {
		return nil, fmt.Errorf("patch id %q does not match target chapter %q", patch.ID, id)
	}
	content := NormalizeChapterPatchContent(chapter, patch.Content)
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("chapter patch content is required")
	}
	if err := utils.ValidateNoSuspiciousPatchText(content); err != nil {
		return nil, fmt.Errorf("chapter patch rejected: %w", err)
	}
	targetWords := opts.TargetWords
	if targetWords <= 0 {
		targetWords = toolChapterTargetWords(root)
	}
	beforeContent, beforePath, err := root.LoadChapterContent(id)
	if err != nil {
		return nil, fmt.Errorf("failed to read chapter %s: %w", id, err)
	}
	if opts.Validate {
		if err := ValidateToolChapterPatchLength(chapter, content, targetWords, beforeContent); err != nil {
			return nil, fmt.Errorf("chapter patch rejected: %w", err)
		}
	}

	changes := []ToolPatchChange{{
		Path:   "chapter." + id + ".content",
		Action: "update",
	}}
	if beforePath == "" {
		changes[0].Action = "create"
	}
	if beforePath != "" && content == beforeContent {
		warnings = append(warnings, "chapter content is unchanged")
	}

	var qualityCheck *ToolCheckResult
	if opts.Validate {
		qualityCheck = MakeToolCheckResult("quality", "chapter", "chapter", id, RunToolChapterQualityGate(chapter, content, targetWords))
		addToolCheckMeta(qualityCheck, "project_root", root.Dir())
		addToolCheckMeta(qualityCheck, "mode", "pre_apply")
		addToolCheckMeta(qualityCheck, "target_words", targetWords)
		if opts.Apply && qualityCheck.Blocking {
			return nil, fmt.Errorf("patch rejected: chapter quality check found blocking issues (critical=%d high=%d total=%d)", qualityCheck.Summary.Critical, qualityCheck.Summary.High, qualityCheck.Summary.Total)
		}
	}

	files := map[string]string{}
	check := qualityCheck
	refreshedDerived := false
	nextActions := BuildChapterPatchNextActions(id, opts.Apply, false, check)
	postApplyError := ""
	if opts.Apply {
		checkpointPath, err := saveToolChapterPatchCheckpoint(root, id, beforePath, beforeContent)
		if err != nil {
			return nil, err
		}
		path := toolFinalChapterPathForID(root, id)
		// root.WriteText is the atomic write (sibling temp file + rename), so an
		// interrupted apply cannot leave a half-written chapter behind.
		if err := root.WriteText(path, content); err != nil {
			return nil, fmt.Errorf("failed to save patched chapter: %w", err)
		}
		files["chapter"] = path
		files["checkpoint"] = checkpointPath
		if opts.RefreshDerived {
			var refresh *toolRefreshResult
			func() {
				restoreLogger := SuppressToolRefreshLogs()
				defer restoreLogger()
				refresh, err = RefreshToolChapterDSL(ctx, root, id, 0)
			}()
			if err != nil {
				postApplyError = fmt.Sprintf("derived RPG DSL refresh failed: %v", err)
				warnings = append(warnings, "chapter patch applied but "+postApplyError)
				nextActions = []ToolNextAction{
					{
						Step:    1,
						Action:  "retry_refresh_derived",
						Command: toolcmd.ChapterDSLRefresh(id),
						Purpose: "The chapter was written, but derived RPG DSL refresh failed; retry the refresh before simulation checks.",
					},
					{
						Step:    2,
						Action:  "post_refresh_check",
						Command: toolcmd.ChapterCheck(id, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
						Purpose: "Run the all check after a successful refresh.",
					},
				}
				check = nil
			} else {
				refreshedDerived = true
				if refresh.DSLPath != "" {
					files["chapter_dsl"] = refresh.DSLPath
				}
				if refresh.CacheDir != "" {
					files["chapter_dsl_cache"] = refresh.CacheDir
				}
				postCheck, postErr := runToolChapterCheckAt(root, "all", "chapter", id, targetWords)
				if postErr != nil {
					postApplyError = fmt.Sprintf("post-refresh check failed: %v", postErr)
					warnings = append(warnings, "chapter patch applied and DSL refreshed but "+postApplyError)
					check = nil
					nextActions = []ToolNextAction{{
						Step:    1,
						Action:  "post_refresh_check",
						Command: toolcmd.ChapterCheck(id, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
						Purpose: "The chapter and DSL were written; rerun the focused all check because the automatic post-refresh check failed to run.",
					}}
				} else {
					addToolCheckMeta(postCheck, "project_root", root.Dir())
					addToolCheckMeta(postCheck, "mode", "post_apply_refresh_all")
					addToolCheckMeta(postCheck, "derived_dsl_refreshed", true)
					check = postCheck
					nextActions = BuildChapterPatchNextActions(id, opts.Apply, refreshedDerived, check)
				}
			}
		} else if opts.Validate {
			postCheck, err := RunToolChapterCheckWithTargetWordsAt(root, "quality", "chapter", id, targetWords)
			if err != nil {
				return nil, err
			}
			addToolCheckMeta(postCheck, "project_root", root.Dir())
			addToolCheckMeta(postCheck, "mode", "post_apply_quality")
			addToolCheckMeta(postCheck, "simulation_requires_derived_dsl_refresh", true)
			check = postCheck
			nextActions = BuildChapterPatchNextActions(id, opts.Apply, refreshedDerived, check)
		}
	}

	ok := postApplyError == ""
	if ok && opts.Validate && check != nil {
		ok = !check.Blocking
	}
	result := ToolPatchResult{
		OK:          ok,
		Applied:     opts.Apply,
		DryRun:      !opts.Apply,
		Target:      "chapter",
		ID:          id,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: nextActions,
		Meta: map[string]interface{}{
			"project_root":           root.Dir(),
			"previous_content_path":  beforePath,
			"previous_content_bytes": len(beforeContent),
			"content_bytes":          len(content),
			"refresh_derived":        refreshedDerived,
			"mode":                   string(opts.Mode),
			"source":                 opts.Source,
		},
	}
	if postApplyError != "" {
		result.Meta["post_apply_error"] = postApplyError
	}
	return &result, nil
}

func BuildChapterPatchNextActions(chapterID string, applied bool, refreshedDerived bool, check *ToolCheckResult) []ToolNextAction {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return nil
	}
	if !applied {
		actions := []ToolNextAction{{
			Step:    1,
			Action:  "apply_validated_patch",
			Purpose: "If this dry-run patch is acceptable and the workflow allows writes, repeat the same patch command with --apply.",
			When:    "Only after check.blocking is false and apply is explicitly allowed.",
		}}
		if check != nil && check.Blocking {
			actions[0] = ToolNextAction{
				Step:    1,
				Action:  "repair_patch_content",
				Purpose: "Use check.issues navigation to repair the proposed chapter content before trying apply.",
				When:    "The dry-run check is blocking.",
			}
		}
		return actions
	}
	if refreshedDerived {
		if check != nil && check.Blocking {
			return []ToolNextAction{{
				Step:    1,
				Action:  "repair_remaining_issues",
				Purpose: "Patch applied and derived RPG DSL was refreshed, but the post-refresh all check is blocking; repair the returned check.issues one target at a time.",
				Command: toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
			}}
		}
		return []ToolNextAction{{
			Step:    1,
			Action:  "return_final_json",
			Purpose: "Patch applied, derived RPG DSL refreshed, and post-refresh all check is clean for blocking issues.",
		}}
	}
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "refresh_derived_dsl",
			Purpose: "Chapter prose changed; regenerate derived RPG DSL before trusting simulation results.",
			Command: toolcmd.ChapterDSLRefresh(chapterID),
		},
		{
			Step:    2,
			Action:  "post_refresh_check",
			Purpose: "Run deterministic quality and simulation checks after derived DSL refresh.",
			Command: toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
		},
	}
}

func buildPatchNextActions(applied bool, check *ToolCheckResult, postApplyCheckCommand string) []ToolNextAction {
	postApplyCheckCommand = strings.TrimSpace(postApplyCheckCommand)
	if !applied {
		action := ToolNextAction{
			Step:    1,
			Action:  "apply_validated_patch",
			Purpose: "If this dry-run patch is acceptable and the workflow allows writes, repeat the same patch command with --apply.",
			When:    "Only after check.blocking is false and apply is explicitly allowed.",
		}
		if check != nil && check.Blocking {
			action = ToolNextAction{
				Step:    1,
				Action:  "repair_patch_content",
				Purpose: "Use check.issues navigation to repair the proposed patch before trying apply.",
				When:    "The dry-run check is blocking.",
			}
		}
		return []ToolNextAction{action}
	}
	if postApplyCheckCommand == "" {
		return []ToolNextAction{{
			Step:    1,
			Action:  "inspect_patch_check",
			Purpose: "Use the check object returned by this patch result before deciding whether another repair is needed.",
		}}
	}
	return []ToolNextAction{{
		Step:    1,
		Action:  "post_apply_check",
		Purpose: "Verify the saved project state after the validated patch write.",
		Command: postApplyCheckCommand,
	}}
}

func NormalizeChapterPatchContent(chapter *models.Chapter, content string) string {
	return project.NormalizeChapterContent(chapter, content)
}

func ValidateToolChapterPatchLength(chapter *models.Chapter, content string, targetWords int, previousContent ...string) error {
	if targetWords <= 0 {
		return nil
	}
	chapterID := ""
	chapterTitle := ""
	if chapter != nil {
		chapterID = chapter.ID
		chapterTitle = chapter.Title
	}
	body := strings.TrimSpace(stripToolChapterMarkdownTitle(content, chapterTitle))
	count := ToolNarrativeUnitCount(body)
	hardMax := toolChapterLengthHardMax(targetWords)
	if count > hardMax {
		return fmt.Errorf("content is too long for chapter %s: got %d narrative units, target %d, hard max %d; trim the patch and keep only the selected chapter", chapterID, count, targetWords, hardMax)
	}
	if len(previousContent) > 0 {
		previousBody := strings.TrimSpace(stripToolChapterMarkdownTitle(previousContent[0], chapterTitle))
		previousCount := ToolNarrativeUnitCount(previousBody)
		minPreviousForShrinkGuard := targetWords / 2
		if minPreviousForShrinkGuard < 500 {
			minPreviousForShrinkGuard = 500
		}
		if previousCount >= minPreviousForShrinkGuard && previousCount <= hardMax {
			minRetained := int(float64(previousCount) * 0.85)
			if count < minRetained {
				return fmt.Errorf("content removes too much existing prose for chapter %s: got %d narrative units, previous %d, minimum retained %d; keep this as a minimal patch unless the check issue is explicitly about excessive length", chapterID, count, previousCount, minRetained)
			}
		}
		previousParagraphs := toolChapterParagraphCount(previousBody)
		paragraphs := toolChapterParagraphCount(body)
		if previousParagraphs >= 4 {
			minParagraphs := previousParagraphs / 2
			if minParagraphs < 3 {
				minParagraphs = 3
			}
			if minParagraphs > 6 {
				minParagraphs = 6
			}
			if paragraphs < minParagraphs {
				return fmt.Errorf("content collapses chapter %s paragraph structure: got %d prose paragraphs, previous %d, minimum %d; preserve mobile-readable paragraph breaks when patching", chapterID, paragraphs, previousParagraphs, minParagraphs)
			}
		}
	}
	return nil
}

func toolChapterLengthHardMax(targetWords int) int {
	if targetWords <= 0 {
		return 0
	}
	hardMax := int(float64(targetWords) * 1.2)
	if targetWords+250 > hardMax {
		hardMax = targetWords + 250
	}
	return hardMax
}
