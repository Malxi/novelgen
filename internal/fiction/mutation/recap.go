// Recap patches: identity-preserving writes of one chapter recap.

package mutation

import (
	"errors"
	"fmt"
	"io/fs"
	"novelgen/internal/contract/toolcmd"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/logic/continuity/recap"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

func runRecapMutation(root *store.Root, id string, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	opts = opts.normalized()
	var result *ToolPatchResult
	err := root.WithMutationLock(func() error {
		var err error
		result, err = runRecapMutationLocked(root, id, patchBytes, opts, warnings)
		return err
	})
	return result, err
}

func runRecapMutationLocked(root *store.Root, id string, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	var validated models.ChapterRecap
	if err := decodeStrictToolPatch(patchBytes, &validated); err != nil {
		return nil, fmt.Errorf("recap patch does not match the recap shape: %w", err)
	}
	recaps := recap.NewStoreAt(root)
	before, err := recaps.Load(id)
	exists := true
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("failed to read recap %s: %w", id, err)
		}
		exists = false
		before = &models.ChapterRecap{ChapterID: id}
		warnings = append(warnings, fmt.Sprintf("recap file not found; patch will create %s", id))
	}
	var outlineChapter *models.Chapter
	if outline, outlineErr := root.LoadOutline(); outlineErr == nil && outline != nil {
		outlineChapter = outline.GetChapterByID(id)
	} else if outlineErr != nil && !errors.Is(outlineErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to load outline while patching recap %s: %w", id, outlineErr)
	}

	var merged models.ChapterRecap
	if opts.Mode == MutationReplace {
		merged = validated
		if merged.ChapterID == "" {
			merged.ChapterID = id
		}
	} else {
		merged, err = MergeJSONPatchObject(*before, patchBytes)
		if err != nil {
			return nil, err
		}
	}
	patchIdentityWarnings := normalizeRecapPatchIdentity(&merged, id, outlineChapter, patchBytes)
	warnings = append(warnings, patchIdentityWarnings...)
	if err := utils.ValidateNoSuspiciousPatchText(merged); err != nil {
		return nil, fmt.Errorf("recap patch rejected: %w", err)
	}

	var check *ToolCheckResult
	if opts.Validate {
		check = runToolRecapCheckForValue("all", "chapter", id, &merged, outlineChapter)
		addToolCheckMeta(check, "project_root", root.Dir())
		if opts.Apply && check.Blocking {
			return nil, fmt.Errorf("patch rejected: recap check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
		}
	}

	files := map[string]string{}
	changes := DiffStructFields("recap."+id, *before, merged)
	warnings = appendNoChangeWarning(warnings, changes)
	if opts.Apply {
		checkpointPath := ""
		if exists {
			checkpointPath, err = saveToolCheckpointJSON(root, store.ScopeRecaps, id, before)
			if err != nil {
				return nil, err
			}
		}
		if err := recaps.Save(&merged); err != nil {
			return nil, fmt.Errorf("failed to save patched recap: %w", err)
		}
		files["recap"] = root.RecapFile(id)
		if checkpointPath != "" {
			files["checkpoint"] = checkpointPath
		}
	}

	ok := !opts.Validate || check == nil || !check.Blocking
	result := ToolPatchResult{
		OK:          ok,
		Applied:     opts.Apply,
		DryRun:      !opts.Apply,
		Target:      "recap.chapter",
		ID:          id,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(opts.Apply, check, toolcmd.Check(toolcmd.CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: id, MinPriority: "low", MaxIssues: 8})),
		Meta: map[string]interface{}{
			"project_root": root.Dir(),
			"mode":         string(opts.Mode),
			"source":       opts.Source,
		},
	}
	return &result, nil
}
