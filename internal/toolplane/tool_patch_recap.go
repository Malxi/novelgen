// Recap patches: identity-preserving writes of one chapter recap.

package toolplane

import (
	"fmt"
	"novelgen/internal/toolcmd"

	"github.com/spf13/cobra"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/store"
	"novelgen/internal/utils"
)

func RunToolRecapPatch(cmd *cobra.Command, root *store.Root, id string, patchBytes []byte, apply bool, warnings []string) error {
	recaps := recap.NewStoreAt(root)
	before, err := recaps.Load(id)
	if err != nil {
		before = &models.ChapterRecap{ChapterID: id}
		warnings = append(warnings, fmt.Sprintf("recap file not found; patch will create %s", id))
	}
	var outlineChapter *models.Chapter
	if outline, err := root.LoadOutline(); err == nil && outline != nil {
		outlineChapter = outline.GetChapterByID(id)
	}

	merged, err := MergeJSONPatchObject(*before, patchBytes)
	if err != nil {
		return err
	}
	patchIdentityWarnings := normalizeRecapPatchIdentity(&merged, id, outlineChapter, patchBytes)
	warnings = append(warnings, patchIdentityWarnings...)
	if err := utils.ValidateNoSuspiciousPatchText(merged); err != nil {
		return fmt.Errorf("recap patch rejected: %w", err)
	}

	check := runToolRecapCheckForValue("all", "chapter", id, &merged, outlineChapter)
	addToolCheckMeta(check, "project_root", root.Dir())
	if apply && check.Blocking {
		return fmt.Errorf("patch rejected: recap check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolCheckpointJSON(root, store.ScopeRecaps, id, before)
		if err != nil {
			return err
		}
		if err := recaps.Save(&merged); err != nil {
			return fmt.Errorf("failed to save patched recap: %w", err)
		}
		files["recap"] = root.RecapFile(id)
		files["checkpoint"] = checkpointPath
	}

	result := ToolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "recap.chapter",
		ID:          id,
		Changed:     DiffStructFields("recap."+id, *before, merged),
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, toolcmd.Check(toolcmd.CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: id, MinPriority: "low", MaxIssues: 8})),
		Meta: map[string]interface{}{
			"project_root": root.Dir(),
		},
	}
	return writeJSON(cmd, result)
}
