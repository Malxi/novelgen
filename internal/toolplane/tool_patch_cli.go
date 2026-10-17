package toolplane

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/fiction/domain/craft"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/mutation"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

// RunToolPatch is the CLI adapter over fiction/mutation. It owns flag parsing,
// patch-source selection and JSON output; merge, normalization, validation,
// checkpoints and atomic writes live in the mutation service.
func RunToolPatch(cmd *cobra.Command, args []string) error {
	section := NormalizeKey(args[0])
	if section != "outline" && section != "craft" && section != "setup" && section != "recap" && section != "chapter" {
		return fmt.Errorf("unsupported patch section %q; valid sections: %s",
			section, strings.Join(toolPatchSections, ", "))
	}
	target := NormalizeKey(ToolPatchFlags.Target)
	if section == "outline" && target == "" {
		target = "chapter"
	}
	if section == "outline" && target != "chapter" && target != "volume" {
		return fmt.Errorf("unsupported outline patch --target %q; valid targets: chapter, volume", target)
	}
	if section == "craft" {
		target = craft.NormalizeTarget(target)
		if target == "" {
			return fmt.Errorf("--target is required for craft patch (character/item/location/organization)")
		}
	}
	id := strings.TrimSpace(ToolPatchFlags.ID)
	taskID := strings.TrimSpace(ToolPatchFlags.Task)
	idRepaired := false
	if section != "setup" {
		repairedID := utils.RepairLikelyMojibakeText(id)
		idRepaired = repairedID != id
		id = repairedID
	}
	if id == "" && section != "setup" && taskID == "" {
		return fmt.Errorf("--id is required")
	}
	warnings := []string{}
	if idRepaired {
		warnings = append(warnings, fmt.Sprintf("repaired possible mojibake in --id from %q to %q", ToolPatchFlags.ID, id))
	}
	apply := ToolPatchFlags.Apply && !ToolPatchFlags.DryRun
	if ToolPatchFlags.Apply && ToolPatchFlags.DryRun {
		warnings = append(warnings, "--dry-run forced a dry run; the patch was not applied")
	}
	if section == "setup" && strings.TrimSpace(ToolPatchFlags.ID) != "" {
		warnings = append(warnings, "--id is not used by setup patches and was ignored")
	}
	if (section == "setup" || section == "recap" || section == "chapter") && target != "" {
		warnings = append(warnings, fmt.Sprintf("--target %q is not used by %s patches and was ignored", target, section))
	}
	if section == "chapter" && ToolPatchFlags.RefreshDerived && !apply {
		warnings = append(warnings, "--refresh-derived only takes effect with --apply; this dry run did not refresh derived DSL")
	}

	explicitSources := 0
	if taskID != "" {
		explicitSources++
	}
	if strings.TrimSpace(ToolPatchFlags.PatchBuffer) != "" {
		explicitSources++
	}
	if strings.TrimSpace(ToolPatchFlags.PatchJSON) != "" {
		explicitSources++
	}
	if explicitSources > 1 {
		return fmt.Errorf("use only one patch source: --task, --patch-buffer, or --patch-json")
	}

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Root
	var patchBytes []byte
	if taskID != "" {
		resolvedTarget, resolvedID, resolvedPatchBytes, taskWarnings, err := resolveToolPatchTask(root, section, target, id, taskID)
		if err != nil {
			return err
		}
		target = resolvedTarget
		id = resolvedID
		patchBytes = resolvedPatchBytes
		warnings = append(warnings, taskWarnings...)
	} else if strings.TrimSpace(ToolPatchFlags.PatchBuffer) != "" {
		patchBytes, err = LoadToolPatchBuffer(root, ToolPatchFlags.PatchBuffer, section)
		if err != nil {
			return err
		}
	} else if strings.TrimSpace(ToolPatchFlags.PatchJSON) != "" {
		patchBytes = []byte(ToolPatchFlags.PatchJSON)
	} else {
		patchBytes, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("failed to read patch stdin: %w", err)
		}
	}
	patchBytes = utils.StripUTF8BOM(patchBytes)
	if len(strings.TrimSpace(string(patchBytes))) == 0 {
		return fmt.Errorf("patch JSON is required on stdin")
	}
	if err := ValidateToolPatchBytes(patchBytes); err != nil {
		return fmt.Errorf("patch rejected: %w", err)
	}

	var refreshChapter mutation.ChapterRefresher
	if section == "chapter" {
		refreshChapter = func(ctx context.Context, root *store.Root, id string, batchSize int) (*mutation.DerivedRefreshResult, error) {
			restoreLogger := SuppressToolRefreshLogs()
			defer restoreLogger()
			refresh, err := RefreshToolChapterDSL(ctx, root, id, batchSize)
			if err != nil {
				return nil, err
			}
			return &mutation.DerivedRefreshResult{DSLPath: refresh.DSLPath, CacheDir: refresh.CacheDir}, nil
		}
	}

	result, err := NewMutationService(root).Patch(cmd.Context(), PatchRequest{
		Section:  section,
		Target:   target,
		ID:       id,
		Mode:     MutationMerge,
		Patch:    patchBytes,
		Warnings: warnings,
		Options: MutationOptions{
			Apply:          apply,
			Validate:       true,
			Mode:           MutationMerge,
			TargetWords:    ToolPatchFlags.TargetWords,
			RefreshDerived: ToolPatchFlags.RefreshDerived,
			RefreshChapter: refreshChapter,
			Source:         "tool-cli",
		},
	})
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func resolveToolPatchTask(root *store.Root, section, target, id, taskID string) (string, string, []byte, []string, error) {
	if section != "outline" {
		return "", "", nil, nil, fmt.Errorf("--task is currently supported only for outline patches")
	}
	const prefix = "outline-global-repair:"
	if !strings.HasPrefix(NormalizeKey(taskID), prefix) {
		return "", "", nil, nil, fmt.Errorf("unsupported patch task %q", taskID)
	}
	name := strings.TrimPrefix(NormalizeKey(taskID), prefix)
	if name == "" {
		return "", "", nil, nil, fmt.Errorf("patch task category is required")
	}
	ctx := ToolProjectContext{Root: root}
	if err := ctx.LoadSetup(); err != nil {
		return "", "", nil, nil, err
	}
	if err := ctx.LoadOutline(); err != nil {
		return "", "", nil, nil, err
	}
	resp := QueryOutlineGlobalRepairContext(ctx, name)
	if !resp.OK {
		return "", "", nil, nil, fmt.Errorf("failed to resolve patch task %q", taskID)
	}
	bundle, ok := resp.Results.(OutlineGlobalRepairContext)
	if !ok || bundle.PatchTask == nil {
		return "", "", nil, nil, fmt.Errorf("patch task %q has no patchable target", taskID)
	}
	task := bundle.PatchTask
	resolvedTarget := extractNovelgenCommandFlag(task.PatchQuery, "--target")
	resolvedID := extractNovelgenCommandFlag(task.PatchQuery, "--id")
	if resolvedTarget == "" || resolvedID == "" {
		return "", "", nil, nil, fmt.Errorf("patch task %q has incomplete patch query %q", taskID, task.PatchQuery)
	}
	if target != "" && target != "chapter" && NormalizeKey(target) != NormalizeKey(resolvedTarget) {
		return "", "", nil, nil, fmt.Errorf("--target %q does not match patch task target %q", target, resolvedTarget)
	}
	if id != "" && id != resolvedID {
		return "", "", nil, nil, fmt.Errorf("--id %q does not match patch task id %q", id, resolvedID)
	}
	patchBytes, err := json.Marshal(task.PatchShape)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to encode patch task %q: %w", taskID, err)
	}
	return NormalizeKey(resolvedTarget), resolvedID, patchBytes, []string{fmt.Sprintf("using patch task %s", taskID)}, nil
}

func extractNovelgenCommandFlag(command, flag string) string {
	fields := strings.Fields(command)
	for i := 0; i < len(fields); i++ {
		if fields[i] == flag && i+1 < len(fields) {
			return strings.Trim(fields[i+1], `"'`)
		}
		prefix := flag + "="
		if strings.HasPrefix(fields[i], prefix) {
			return strings.Trim(strings.TrimPrefix(fields[i], prefix), `"'`)
		}
	}
	return ""
}

func writeJSON(cmd *cobra.Command, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
