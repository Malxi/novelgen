// The tool patch entry point: argument dispatch, patch-task resolution, patch-bytes validation and
// the generic JSON merge/diff helpers. Project files are written only behind apply.

package toolplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
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
	data := utils.StripUTF8BOM(patchBytes)
	if len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("patch JSON is required")
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	object, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("patch must be a JSON object with at least one field")
	}
	if len(object) == 0 {
		return fmt.Errorf("patch must contain at least one field")
	}
	return utils.ValidateNoSuspiciousPatchText(value)
}

// decodeStrictToolPatch decodes one patch object and rejects fields the target
// type does not declare. The tool plane used to merge patches through a
// map[string]interface{}, where a typo such as "sumary" was silently dropped
// and the command still reported success.
func decodeStrictToolPatch(data []byte, out interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			value := reflect.ValueOf(out)
			if value.Kind() == reflect.Ptr && !value.IsNil() {
				allowed := structJSONFields(value.Elem().Interface())
				if len(allowed) > 0 {
					names := make([]string, 0, len(allowed))
					for name := range allowed {
						names = append(names, name)
					}
					sort.Strings(names)
					return fmt.Errorf("%w; valid fields: %s", err, strings.Join(names, ", "))
				}
			}
		}
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("patch must contain exactly one JSON object")
		}
		return err
	}
	return nil
}

// structJSONFields returns the JSON field names a struct accepts. It is used for
// setup patches, whose top-level object also accepts two virtual array-edit
// fields that are consumed by the tool plane and never stored.
func structJSONFields(value interface{}) map[string]bool {
	fields := map[string]bool{}
	collectStructJSONFields(reflect.TypeOf(value), fields)
	return fields
}

func collectStructJSONFields(t reflect.Type, out map[string]bool) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			collectStructJSONFields(field.Type, out)
			continue
		}
		if field.PkgPath != "" {
			continue
		}
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "-" {
			continue
		}
		if tag == "" {
			tag = field.Name
		}
		out[tag] = true
	}
}

func unknownPatchFieldsError(kind string, provided map[string]json.RawMessage, allowed map[string]bool) error {
	var unknown []string
	for key := range provided {
		if !allowed[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	valid := make([]string, 0, len(allowed))
	for key := range allowed {
		valid = append(valid, key)
	}
	sort.Strings(valid)
	return fmt.Errorf("%s patch has unknown field(s): %s; valid fields: %s",
		kind, strings.Join(unknown, ", "), strings.Join(valid, ", "))
}

// appendNoChangeWarning makes a dry-run/apply that changed nothing explicit.
// Agents should not have to infer success from an empty `changed` array.
func appendNoChangeWarning(warnings []string, changes []ToolPatchChange) []string {
	if len(changes) == 0 {
		warnings = append(warnings, "patch produced no changes; check field names and values against the target shape")
	}
	return warnings
}

func RunToolPatchValidationCheck(setup *models.StorySetup, outline *models.Outline, scope, id string) (*ToolCheckResult, error) {
	return RunToolPatchValidationCheckAt(nil, setup, outline, scope, id)
}

// RunToolPatchValidationCheckAt validates a patch against an explicit project
// root so outline simulation uses the same craft elements as the project.
func RunToolPatchValidationCheckAt(root *store.Root, setup *models.StorySetup, outline *models.Outline, scope, id string) (*ToolCheckResult, error) {
	return RunToolOutlineCheckAt(root, "all", setup, outline, scope, id)
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
