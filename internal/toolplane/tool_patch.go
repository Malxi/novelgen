// The tool patch entry point: argument dispatch, patch-task resolution, patch-bytes validation and
// the generic JSON merge/diff helpers. Project files are written only behind apply.

package toolplane

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"
	"novelgen/internal/utils"
)

type ToolPatchResult struct {
	OK          bool                   `json:"ok"`
	Applied     bool                   `json:"applied"`
	DryRun      bool                   `json:"dry_run"`
	Target      string                 `json:"target"`
	ID          string                 `json:"id"`
	Changed     []ToolPatchChange      `json:"changed,omitempty"`
	Check       *ToolCheckResult       `json:"check,omitempty"`
	Files       map[string]string      `json:"files,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	NextActions []ToolNextAction       `json:"next_actions,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type ToolPatchChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

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
		target = normalizeCraftPatchTarget(target)
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

	switch section {
	case "outline":
		return runToolOutlinePatch(cmd, root, target, id, patchBytes, apply, warnings)
	case "craft":
		return RunToolCraftPatch(cmd, root, target, id, patchBytes, apply, warnings)
	case "setup":
		return RunToolSetupPatch(cmd, root, patchBytes, apply, warnings)
	case "recap":
		return RunToolRecapPatch(cmd, root, id, patchBytes, apply, warnings)
	case "chapter":
		return runToolChapterPatch(cmd, root, id, patchBytes, apply, warnings)
	default:
		return fmt.Errorf("unsupported patch section %q", section)
	}
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
	if err := ctx.loadSetup(); err != nil {
		return "", "", nil, nil, err
	}
	if err := ctx.loadOutline(); err != nil {
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

func ValidateToolPatchBytes(patchBytes []byte) error {
	return utils.ValidatePatchJSONText(patchBytes)
}

func RunToolPatchValidationCheck(setup *models.StorySetup, outline *models.Outline, scope, id string) (*ToolCheckResult, error) {
	return RunToolOutlineCheck("all", setup, outline, scope, id)
}

func toolPatchValidationMeta(check *ToolCheckResult) map[string]interface{} {
	if check == nil {
		return nil
	}
	meta := map[string]interface{}{
		"kind":     check.Kind,
		"target":   check.Target,
		"scope":    check.Scope,
		"id":       check.ID,
		"blocking": check.Blocking,
		"score":    check.Score,
	}
	if coverage, ok := check.Meta["coverage"]; ok {
		meta["coverage"] = coverage
	}
	return meta
}

func MergeJSONPatchObject[T any](original T, patchBytes []byte) (T, error) {
	var base map[string]interface{}
	data, err := json.Marshal(original)
	if err != nil {
		return original, err
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return original, err
	}
	var patch map[string]interface{}
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return original, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	for key, value := range patch {
		if key == "id" {
			continue
		}
		mergeJSONPatchValue(base, key, value)
	}
	mergedBytes, err := json.Marshal(base)
	if err != nil {
		return original, err
	}
	var merged T
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return original, fmt.Errorf("patch does not match target schema: %w", err)
	}
	return merged, nil
}

func mergeJSONPatchValue(base map[string]interface{}, key string, value interface{}) {
	if value == nil {
		base[key] = nil
		return
	}
	patchMap, ok := value.(map[string]interface{})
	if !ok {
		base[key] = value
		return
	}
	baseMap, ok := base[key].(map[string]interface{})
	if !ok || baseMap == nil {
		base[key] = value
		return
	}
	for childKey, childValue := range patchMap {
		if childKey == "id" {
			continue
		}
		mergeJSONPatchValue(baseMap, childKey, childValue)
	}
	base[key] = baseMap
}

func writeJSON(cmd *cobra.Command, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
