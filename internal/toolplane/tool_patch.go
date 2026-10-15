package toolplane

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/quality"
	"novelgen/internal/toolcmd"
	"novelgen/internal/utils"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func RunToolPatch(cmd *cobra.Command, args []string) error {
	section := NormalizeKey(args[0])
	if section != "outline" && section != "craft" && section != "setup" && section != "recap" && section != "chapter" {
		return fmt.Errorf("unsupported patch section %q", section)
	}
	target := NormalizeKey(ToolPatchFlags.Target)
	if section == "outline" && target == "" {
		target = "chapter"
	}
	if section == "outline" && target != "chapter" && target != "volume" {
		return fmt.Errorf("unsupported outline patch target %q", target)
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

	root, err := project.FindRoot()
	if err != nil {
		return err
	}
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

func resolveToolPatchTask(root, section, target, id, taskID string) (string, string, []byte, []string, error) {
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

func RunToolPatchBuffer(cmd *cobra.Command, args []string) error {
	op := NormalizeKey(args[0])
	if !allowedToolPatchBufferOps[op] {
		return fmt.Errorf("unsupported patch-buffer operation %q", op)
	}
	id := strings.TrimSpace(ToolPatchBufferFlags.ID)
	if id == "" {
		return fmt.Errorf("--id is required")
	}
	root, err := project.FindRoot()
	if err != nil {
		return err
	}
	path, err := ToolPatchBufferPath(root, id)
	if err != nil {
		return err
	}
	switch op {
	case "clear":
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clear patch buffer: %w", err)
		}
		return writeJSON(cmd, map[string]interface{}{"ok": true, "id": id, "path": path, "bytes": 0})
	case "append":
		text := ToolPatchBufferFlags.Text
		if ToolPatchBufferFlags.Stdin && text != "" {
			return fmt.Errorf("use either --text or --stdin, not both")
		}
		if text == "" || ToolPatchBufferFlags.Stdin {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read patch buffer stdin: %w", err)
			}
			text = string(utils.StripUTF8BOM(data))
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("buffer text is required via --text or stdin")
		}
		if err := utils.ValidateNoSuspiciousPatchText(map[string]interface{}{"text": text}); err != nil {
			return fmt.Errorf("patch buffer rejected: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create patch buffer directory: %w", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open patch buffer: %w", err)
		}
		if _, err := f.WriteString(text); err != nil {
			_ = f.Close()
			return fmt.Errorf("failed to append patch buffer: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close patch buffer: %w", err)
		}
		info, _ := os.Stat(path)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		return writeJSON(cmd, map[string]interface{}{"ok": true, "id": id, "path": path, "bytes": size})
	case "show":
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read patch buffer: %w", err)
		}
		maxChars := ToolPatchBufferFlags.MaxChars
		if maxChars <= 0 {
			maxChars = 400
		}
		if maxChars > 2000 {
			maxChars = 2000
		}
		runes := []rune(string(data))
		preview := string(runes)
		truncated := false
		if len(runes) > maxChars {
			preview = string(runes[:maxChars])
			truncated = true
		}
		return writeJSON(cmd, map[string]interface{}{
			"ok":        true,
			"id":        id,
			"path":      path,
			"bytes":     len(data),
			"chars":     len(runes),
			"preview":   preview,
			"truncated": truncated,
		})
	}
	return fmt.Errorf("unsupported patch-buffer operation %q", op)
}

func LoadToolPatchBuffer(root, id, section string) ([]byte, error) {
	path, err := ToolPatchBufferPath(root, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read patch buffer %q: %w", id, err)
	}
	content := string(utils.StripUTF8BOM(data))
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("patch buffer %q is empty", id)
	}
	if NormalizeKey(section) == "chapter" {
		payload, err := json.Marshal(map[string]string{"content": content})
		if err != nil {
			return nil, err
		}
		return payload, nil
	}
	return []byte(content), nil
}

func ToolPatchBufferPath(root, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("patch buffer id is required")
	}
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			return "", fmt.Errorf("patch buffer id %q contains unsupported character %q", id, r)
		}
	}
	clean := b.String()
	if clean == "" || clean != id {
		return "", fmt.Errorf("invalid patch buffer id %q", id)
	}
	return filepath.Join(root, ".novelgen", "agent-patches", clean+".txt"), nil
}

func runToolOutlinePatch(cmd *cobra.Command, root, target, id string, patchBytes []byte, apply bool, warnings []string) error {
	ctx := ToolProjectContext{Root: root}
	if err := ctx.loadSetup(); err != nil {
		return err
	}
	if err := ctx.loadOutline(); err != nil {
		return err
	}

	beforeOutline := CloneOutline(ctx.Outline)
	var changes []ToolPatchChange
	var err error
	switch target {
	case "chapter":
		changes, err = ApplyChapterPatch(ctx.Outline, id, patchBytes)
	case "volume":
		changes, err = ApplyVolumePatch(ctx.Outline, id, patchBytes)
	}
	if err != nil {
		return err
	}

	scope := target
	check, err := RunToolPatchValidationCheck(ctx.Setup, ctx.Outline, scope, id)
	if err != nil {
		return err
	}
	addToolCheckMeta(check, "project_root", root)
	globalCheck, err := RunToolPatchValidationCheck(ctx.Setup, ctx.Outline, "all", "")
	if err != nil {
		return err
	}
	addToolCheckMeta(globalCheck, "project_root", root)
	if newBlocking := newBlockingPatchIssues(beforeOutline, ctx.Setup, globalCheck); len(newBlocking) > 0 {
		ctx.Outline = beforeOutline
		first := newBlocking[0]
		return fmt.Errorf("patch rejected: global outline check introduced %d new blocking issue(s); first [%s/%s] %s: %s", len(newBlocking), first.Priority, first.Category, first.TargetID, first.Issue)
	}
	if apply && check.Blocking {
		ctx.Outline = beforeOutline
		return fmt.Errorf("patch rejected: quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolPatchCheckpoint(root, beforeOutline)
		if err != nil {
			return err
		}
		OutlinePath := filepath.Join(root, "story", "compose", "outline.json")
		if err := project.SavePartialOutline(ctx.Outline, OutlinePath); err != nil {
			return fmt.Errorf("failed to save patched outline: %w", err)
		}
		mdPath := filepath.Join(root, "story", "compose", "outline.md")
		if err := project.CreateOutlineMarkdown(ctx.Outline, mdPath); err != nil {
			return fmt.Errorf("failed to save outline markdown: %w", err)
		}
		files["outline"] = OutlinePath
		files["markdown"] = mdPath
		files["checkpoint"] = checkpointPath
	} else {
		ctx.Outline = beforeOutline
	}

	result := ToolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      target,
		ID:          id,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, OutlinePatchPostCheckCommand(target, id)),
		Meta: map[string]interface{}{
			"project_root": root,
			"validation":   toolPatchValidationMeta(check),
		},
	}
	return writeJSON(cmd, result)
}

func newBlockingPatchIssues(before *models.Outline, setup *models.StorySetup, afterCheck *ToolCheckResult) []models.ReviewSuggestion {
	if before == nil || setup == nil || afterCheck == nil {
		return nil
	}
	beforeCheck, err := RunToolPatchValidationCheck(setup, before, "all", "")
	if err != nil {
		return nil
	}
	beforeBlocking := map[string]bool{}
	for _, issue := range beforeCheck.Issues {
		if toolPatchIssueIsBlocking(issue) {
			beforeBlocking[toolPatchIssueSignature(issue)] = true
		}
	}
	var out []models.ReviewSuggestion
	for _, issue := range afterCheck.Issues {
		if !toolPatchIssueIsBlocking(issue) {
			continue
		}
		if beforeBlocking[toolPatchIssueSignature(issue)] {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func toolPatchIssueIsBlocking(issue models.ReviewSuggestion) bool {
	rank, ok := reviewPriorityRank(issue.Priority)
	return ok && rank >= 3
}

func toolPatchIssueSignature(issue models.ReviewSuggestion) string {
	return strings.Join([]string{
		NormalizeKey(issue.Category),
		strings.ToLower(strings.TrimSpace(issue.TargetID)),
		strings.ToLower(strings.TrimSpace(issue.TargetName)),
		strings.ToLower(strings.TrimSpace(issue.Issue)),
	}, "\x00")
}

func RunToolCraftPatch(cmd *cobra.Command, root, target, id string, patchBytes []byte, apply bool, warnings []string) error {
	filename, err := craftPatchFilename(target)
	if err != nil {
		return err
	}
	values, err := loadRawCraftMap(root, filename)
	if err != nil {
		return err
	}
	if values == nil {
		values = map[string]map[string]interface{}{}
	}

	beforeValues := cloneRawCraftMap(values)
	key, action, changes, err := ApplyCraftElementPatch(values, target, id, patchBytes)
	if err != nil {
		return err
	}

	gate := quality.QualityGateResult{}
	validateCraftSchemaObject(&gate, target, key, values[key])
	if normalizeCraftPatchTarget(target) == "item" {
		validateCraftConsistencyObject(&gate, target, key, values[key], loadCraftCharacterNames(root))
	}
	validateCraftQualityObject(&gate, target, key, values[key])
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	check := MakeToolCheckResult("schema", "craft", target, key, gate)
	addToolCheckMeta(check, "project_root", root)

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolCraftPatchCheckpoint(root, filename, beforeValues)
		if err != nil {
			return err
		}
		path := filepath.Join(root, "story", "craft", filename)
		if err := SaveRawCraftMap(path, values); err != nil {
			return err
		}
		files["craft"] = path
		files["checkpoint"] = checkpointPath
	}

	result := ToolPatchResult{
		OK:          true,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "craft." + target,
		ID:          key,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", target, key)),
		Meta: map[string]interface{}{
			"project_root": root,
			"action":       action,
		},
	}
	return writeJSON(cmd, result)
}

func RunToolSetupPatch(cmd *cobra.Command, root string, patchBytes []byte, apply bool, warnings []string) error {
	setupPath := filepath.Join(root, "story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}
	before := CloneStorySetup(setup)
	merged, setupPatchWarnings, err := ApplySetupPatchObject(*setup, patchBytes)
	if err != nil {
		return err
	}
	warnings = append(warnings, setupPatchWarnings...)
	if err := utils.ValidateNoSuspiciousPatchText(merged); err != nil {
		return fmt.Errorf("setup patch rejected: %w", err)
	}
	models.NormalizeStorySetup(&merged)

	check, err := RunToolSetupCheck("all", &merged)
	if err != nil {
		return err
	}
	addToolCheckMeta(check, "project_root", root)
	if apply && check.Blocking {
		return fmt.Errorf("patch rejected: setup quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := SaveToolSetupPatchCheckpoint(root, before)
		if err != nil {
			return err
		}
		if err := merged.Save(setupPath); err != nil {
			return fmt.Errorf("failed to save patched story setup: %w", err)
		}
		mdPath := filepath.Join(root, "story", "setup", "story_setup.md")
		if err := project.CreateStorySetupMarkdown(&merged, mdPath); err != nil {
			return fmt.Errorf("failed to save story setup markdown: %w", err)
		}
		files["setup"] = setupPath
		files["markdown"] = mdPath
		files["checkpoint"] = checkpointPath
	}

	result := ToolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "setup",
		ID:          "story_setup",
		Changed:     DiffStructFields("setup", before, merged),
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, "novelgen tool check all --target setup --min-priority medium --max-issues 12"),
		Meta: map[string]interface{}{
			"project_root": root,
		},
	}
	return writeJSON(cmd, result)
}

func ApplySetupPatchObject(setup models.StorySetup, patchBytes []byte) (models.StorySetup, []string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchBytes, &raw); err != nil {
		return setup, nil, fmt.Errorf("failed to parse setup patch JSON: %w", err)
	}
	scalarPatch := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		switch key {
		case "core_cast", "storylines", "premises", "world_timeline", "world_resources", "long_form_plan", "writing_style", "rules", "genres", "rules_patch", "genres_patch":
			continue
		default:
			scalarPatch[key] = value
		}
	}

	merged := setup
	if len(scalarPatch) > 0 {
		data, err := json.Marshal(scalarPatch)
		if err != nil {
			return setup, nil, err
		}
		mergedValue, mergeErr := MergeJSONPatchObject(setup, data)
		if mergeErr != nil {
			return setup, nil, mergeErr
		}
		merged = mergedValue
	}

	var upserted []string
	var err error
	if value, ok := raw["genres"]; ok {
		merged.Genres, err = upsertSetupStringList(merged.Genres, value, "genres")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "genres")
	}
	if value, ok := raw["rules"]; ok {
		merged.Rules, err = upsertSetupStringList(merged.Rules, value, "rules")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "rules")
	}
	if value, ok := raw["genres_patch"]; ok {
		merged.Genres, err = applySetupStringListPatch(merged.Genres, value, "genres_patch")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "genres_patch")
	}
	if value, ok := raw["rules_patch"]; ok {
		merged.Rules, err = applySetupStringListPatch(merged.Rules, value, "rules_patch")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "rules_patch")
	}
	if value, ok := raw["core_cast"]; ok {
		merged.CoreCast, err = upsertSetupPatchItems(merged.CoreCast, value, setupCoreCastPatchKey, "core_cast")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "core_cast")
	}
	if value, ok := raw["storylines"]; ok {
		merged.Storylines, err = upsertSetupPatchItems(merged.Storylines, value, func(item models.Storyline) string { return item.Name }, "storylines")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "storylines")
	}
	if value, ok := raw["premises"]; ok {
		merged.Premises, err = upsertSetupPatchItems(merged.Premises, value, func(item models.Premise) string { return item.Name }, "premises")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "premises")
	}
	if value, ok := raw["world_timeline"]; ok {
		merged.WorldTimeline, err = upsertSetupPatchItems(merged.WorldTimeline, value, setupTimelinePatchKey, "world_timeline")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "world_timeline")
	}
	if value, ok := raw["world_resources"]; ok {
		merged.WorldResources, err = upsertSetupPatchItems(merged.WorldResources, value, func(item models.WorldResource) string { return item.Name }, "world_resources")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "world_resources")
	}
	if value, ok := raw["long_form_plan"]; ok {
		merged.LongFormPlan, err = mergeSetupLongFormPlan(merged.LongFormPlan, value)
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "long_form_plan")
	}
	if value, ok := raw["writing_style"]; ok {
		merged.WritingStyle, err = mergeSetupWritingStyle(merged.WritingStyle, value)
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "writing_style")
	}

	var warnings []string
	if len(upserted) > 0 {
		warnings = append(warnings, "setup patch used incremental merge/upsert semantics for field(s): "+strings.Join(upserted, ", "))
	}
	return merged, warnings, nil
}

func upsertSetupStringList(existing []string, patchRaw json.RawMessage, path string) ([]string, error) {
	if isJSONNull(patchRaw) {
		return nil, nil
	}
	var items []string
	if err := json.Unmarshal(patchRaw, &items); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of strings: %w", path, err)
	}
	return appendUniqueSetupStrings(existing, items), nil
}

func applySetupStringListPatch(existing []string, patchRaw json.RawMessage, path string) ([]string, error) {
	if isJSONNull(patchRaw) {
		return existing, nil
	}
	var items []setupStringListPatchItem
	if err := json.Unmarshal(patchRaw, &items); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of {index,value}: %w", path, err)
	}
	out := append([]string(nil), existing...)
	for _, item := range items {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, fmt.Errorf("setup patch %s index %d out of range 0..%d", path, item.Index, len(out)-1)
		}
		value := strings.TrimSpace(item.Value)
		if value == "" {
			return nil, fmt.Errorf("setup patch %s index %d value is empty", path, item.Index)
		}
		out[item.Index] = value
	}
	return out, nil
}

func mergeSetupLongFormPlan(existing *models.LongFormPlan, patchRaw json.RawMessage) (*models.LongFormPlan, error) {
	if isJSONNull(patchRaw) {
		return nil, nil
	}
	base := models.LongFormPlan{}
	if existing != nil {
		base = *existing
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchRaw, &raw); err != nil {
		return nil, fmt.Errorf("setup patch long_form_plan must be an object or null: %w", err)
	}
	patchCopy := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		switch key {
		case "escalation_ladder", "reader_promises", "volume_pattern":
			continue
		default:
			patchCopy[key] = value
		}
	}
	merged := base
	if len(patchCopy) > 0 {
		data, err := json.Marshal(patchCopy)
		if err != nil {
			return nil, err
		}
		mergedValue, err := MergeJSONPatchObject(base, data)
		if err != nil {
			return nil, fmt.Errorf("setup patch long_form_plan does not match schema: %w", err)
		}
		merged = mergedValue
	}
	var err error
	if value, ok := raw["escalation_ladder"]; ok {
		merged.EscalationLadder, err = upsertSetupStringList(base.EscalationLadder, value, "long_form_plan.escalation_ladder")
		if err != nil {
			return nil, err
		}
	}
	if value, ok := raw["reader_promises"]; ok {
		merged.ReaderPromises, err = upsertSetupStringList(base.ReaderPromises, value, "long_form_plan.reader_promises")
		if err != nil {
			return nil, err
		}
	}
	if value, ok := raw["volume_pattern"]; ok {
		merged.VolumePattern, err = upsertSetupStringList(base.VolumePattern, value, "long_form_plan.volume_pattern")
		if err != nil {
			return nil, err
		}
	}
	return &merged, nil
}

func mergeSetupWritingStyle(existing models.WritingStyle, patchRaw json.RawMessage) (models.WritingStyle, error) {
	if isJSONNull(patchRaw) {
		return models.WritingStyle{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchRaw, &raw); err != nil {
		return existing, fmt.Errorf("setup patch writing_style must be an object or null: %w", err)
	}
	patchCopy := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		switch key {
		case "principles", "avoid":
			continue
		default:
			patchCopy[key] = value
		}
	}
	merged := existing
	if len(patchCopy) > 0 {
		data, err := json.Marshal(patchCopy)
		if err != nil {
			return existing, err
		}
		mergedValue, err := MergeJSONPatchObject(existing, data)
		if err != nil {
			return existing, fmt.Errorf("setup patch writing_style does not match schema: %w", err)
		}
		merged = mergedValue
	}
	var err error
	if value, ok := raw["principles"]; ok {
		merged.Principles, err = upsertSetupStringList(existing.Principles, value, "writing_style.principles")
		if err != nil {
			return existing, err
		}
	}
	if value, ok := raw["avoid"]; ok {
		merged.Avoid, err = upsertSetupStringList(existing.Avoid, value, "writing_style.avoid")
		if err != nil {
			return existing, err
		}
	}
	return merged, nil
}

func appendUniqueSetupStrings(existing, patch []string) []string {
	out := append([]string(nil), existing...)
	seen := map[string]bool{}
	for _, item := range out {
		key := normalizeSetupPatchKey(item)
		if key != "" {
			seen[key] = true
		}
	}
	for _, item := range patch {
		item = strings.TrimSpace(item)
		key := normalizeSetupPatchKey(item)
		if key == "" || seen[key] {
			continue
		}
		out = append(out, item)
		seen[key] = true
	}
	return out
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.EqualFold(strings.TrimSpace(string(raw)), "null")
}

func upsertSetupPatchItems[T any](existing []T, patchRaw json.RawMessage, keyFn func(T) string, path string) ([]T, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(patchRaw, &raws); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of objects: %w", path, err)
	}
	out := append([]T(nil), existing...)
	for _, raw := range raws {
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("setup patch %s item does not match schema: %w", path, err)
		}
		key := normalizeSetupPatchKey(keyFn(item))
		if key == "" {
			out = append(out, item)
			continue
		}
		found := false
		for i := range out {
			if normalizeSetupPatchKey(keyFn(out[i])) != key {
				continue
			}
			merged, err := MergeJSONPatchObject(out[i], raw)
			if err != nil {
				return nil, fmt.Errorf("setup patch %s item %q does not match schema: %w", path, keyFn(item), err)
			}
			out[i] = merged
			found = true
			break
		}
		if !found {
			out = append(out, item)
		}
	}
	return out, nil
}

func setupCoreCastPatchKey(item models.CoreCastSeed) string {
	if strings.TrimSpace(item.ID) != "" {
		return "id:" + item.ID
	}
	return "name:" + item.Name
}

func setupTimelinePatchKey(item models.WorldTimelineEntry) string {
	if strings.TrimSpace(item.RelatedMystery) != "" {
		return "mystery:" + item.RelatedMystery
	}
	if strings.TrimSpace(item.Year) != "" || strings.TrimSpace(item.Event) != "" {
		return "time:" + item.Year + "|" + item.Event
	}
	return ""
}

func normalizeSetupPatchKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func RunToolRecapPatch(cmd *cobra.Command, root, id string, patchBytes []byte, apply bool, warnings []string) error {
	store := recap.NewStore(root)
	before, err := store.Load(id)
	if err != nil {
		before = &models.ChapterRecap{ChapterID: id}
		warnings = append(warnings, fmt.Sprintf("recap file not found; patch will create %s", id))
	}
	var outlineChapter *models.Chapter
	if outline, err := models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json")); err == nil && outline != nil {
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
	addToolCheckMeta(check, "project_root", root)
	if apply && check.Blocking {
		return fmt.Errorf("patch rejected: recap check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolRecapPatchCheckpoint(root, id, before)
		if err != nil {
			return err
		}
		if err := store.Save(&merged); err != nil {
			return fmt.Errorf("failed to save patched recap: %w", err)
		}
		files["recap"] = filepath.Join(root, "story", "recaps", id+".json")
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
		NextActions: buildPatchNextActions(apply, check, fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", id)),
		Meta: map[string]interface{}{
			"project_root": root,
		},
	}
	return writeJSON(cmd, result)
}

func runToolChapterPatch(cmd *cobra.Command, root, id string, patchBytes []byte, apply bool, warnings []string) error {
	outline, err := models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json"))
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}
	chapter := outline.GetChapterByID(id)
	if chapter == nil {
		return fmt.Errorf("chapter %q not found in outline", id)
	}
	var patch toolChapterPatch
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return fmt.Errorf("failed to parse chapter patch: %w", err)
	}
	if strings.TrimSpace(patch.ID) != "" && strings.TrimSpace(patch.ID) != id {
		return fmt.Errorf("patch id %q does not match target chapter %q", patch.ID, id)
	}
	content := NormalizeChapterPatchContent(chapter, patch.Content)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("chapter patch content is required")
	}
	if err := utils.ValidateNoSuspiciousPatchText(content); err != nil {
		return fmt.Errorf("chapter patch rejected: %w", err)
	}
	targetWords := ToolPatchFlags.TargetWords
	if targetWords <= 0 {
		targetWords = toolChapterTargetWords()
	}
	beforePath, beforeContent := loadFinalChapterContentWithPath(root, id)
	if err := ValidateToolChapterPatchLength(chapter, content, targetWords, beforeContent); err != nil {
		return fmt.Errorf("chapter patch rejected: %w", err)
	}

	changes := []ToolPatchChange{{
		Path:   "chapter." + id + ".content",
		Action: "update",
	}}
	if beforePath == "" {
		changes[0].Action = "create"
	}

	qualityCheck := MakeToolCheckResult("quality", "chapter", "chapter", id, RunToolChapterQualityGate(chapter, content, targetWords))
	addToolCheckMeta(qualityCheck, "project_root", root)
	addToolCheckMeta(qualityCheck, "mode", "pre_apply")
	addToolCheckMeta(qualityCheck, "target_words", targetWords)
	if apply && qualityCheck.Blocking {
		return fmt.Errorf("patch rejected: chapter quality check found blocking issues (critical=%d high=%d total=%d)", qualityCheck.Summary.Critical, qualityCheck.Summary.High, qualityCheck.Summary.Total)
	}

	files := map[string]string{}
	check := qualityCheck
	refreshedDerived := false
	if apply {
		checkpointPath, err := saveToolChapterPatchCheckpoint(root, id, beforePath, beforeContent)
		if err != nil {
			return err
		}
		path := toolFinalChapterPathForID(root, id)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to save patched chapter: %w", err)
		}
		files["chapter"] = path
		files["checkpoint"] = checkpointPath
		if ToolPatchFlags.RefreshDerived {
			var refresh *toolRefreshResult
			func() {
				restoreLogger := SuppressToolRefreshLogs()
				defer restoreLogger()
				refresh, err = RefreshToolChapterDSL(cmd.Context(), root, id, 0)
			}()
			if err != nil {
				return fmt.Errorf("chapter patch applied but derived RPG DSL refresh failed: %w", err)
			}
			refreshedDerived = true
			if refresh.DSLPath != "" {
				files["chapter_dsl"] = refresh.DSLPath
			}
			if refresh.CacheDir != "" {
				files["chapter_dsl_cache"] = refresh.CacheDir
			}
			postCheck, err := RunToolChapterCheckWithTargetWords(root, "all", "chapter", id, targetWords)
			if err != nil {
				return err
			}
			addToolCheckMeta(postCheck, "project_root", root)
			addToolCheckMeta(postCheck, "mode", "post_apply_refresh_all")
			addToolCheckMeta(postCheck, "derived_dsl_refreshed", true)
			check = postCheck
		} else {
			postCheck, err := RunToolChapterCheckWithTargetWords(root, "quality", "chapter", id, targetWords)
			if err != nil {
				return err
			}
			addToolCheckMeta(postCheck, "project_root", root)
			addToolCheckMeta(postCheck, "mode", "post_apply_quality")
			addToolCheckMeta(postCheck, "simulation_requires_derived_dsl_refresh", true)
			check = postCheck
		}
	}

	result := ToolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "chapter",
		ID:          id,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: BuildChapterPatchNextActions(id, apply, refreshedDerived, check),
		Meta: map[string]interface{}{
			"project_root":           root,
			"previous_content_path":  beforePath,
			"previous_content_bytes": len(beforeContent),
			"content_bytes":          len(content),
			"refresh_derived":        refreshedDerived,
		},
	}
	return writeJSON(cmd, result)
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

func OutlinePatchPostCheckCommand(target, id string) string {
	target = NormalizeKey(target)
	id = strings.TrimSpace(id)
	switch target {
	case "volume":
		if id != "" {
			return fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --min-priority medium --max-issues 12", id)
		}
	case "chapter":
		if id != "" {
			return fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --min-priority low --max-issues 8", id)
		}
	}
	return ""
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

func loadFinalChapterContentWithPath(root, chapterID string) (string, string) {
	candidates := []string{
		toolFinalChapterPathForID(root, chapterID),
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", utils.ExtractChapterNumber(chapterID))),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return path, string(data)
		}
	}
	return "", ""
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

func ApplyChapterPatch(outline *models.Outline, chapterID string, patchBytes []byte) ([]ToolPatchChange, error) {
	chapter := outline.GetChapterByID(chapterID)
	if chapter == nil {
		return nil, fmt.Errorf("chapter %q not found", chapterID)
	}
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(patchBytes, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if strings.TrimSpace(probe.ID) != "" && probe.ID != chapter.ID {
		return nil, fmt.Errorf("patch id %q does not match target chapter %q", probe.ID, chapter.ID)
	}
	before := *chapter
	merged, err := MergeJSONPatchObject(before, patchBytes)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(merged.ID) != "" && merged.ID != before.ID {
		return nil, fmt.Errorf("patch id %q does not match target chapter %q", merged.ID, before.ID)
	}
	merged.ID = before.ID
	*chapter = merged
	return DiffStructFields("outline.chapter."+chapterID, before, merged), nil
}

func ApplyVolumePatch(outline *models.Outline, volumeID string, patchBytes []byte) ([]ToolPatchChange, error) {
	volume := outline.GetVolumeByID(volumeID)
	if volume == nil {
		return nil, fmt.Errorf("volume %q not found", volumeID)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if _, ok := raw["chapters"]; ok {
		return nil, fmt.Errorf("volume patch must use changed_chapters; replacing chapters is not allowed")
	}
	var patch outlineVolumePatch
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return nil, fmt.Errorf("failed to parse volume patch: %w", err)
	}
	if strings.TrimSpace(patch.ID) != "" && patch.ID != volume.ID {
		return nil, fmt.Errorf("patch id %q does not match target volume %q", patch.ID, volume.ID)
	}

	before := *volume
	if _, ok := raw["title"]; ok {
		volume.Title = patch.Title
	}
	if _, ok := raw["summary"]; ok {
		volume.Summary = patch.Summary
	}
	if _, ok := raw["payoff_contract"]; ok {
		volume.PayoffContract = models.MergeVolumePayoffContract(volume.PayoffContract, patch.PayoffContract)
	}
	changes := DiffStructFields("outline.volume."+volumeID, before, *volume)

	for _, rawChapter := range patch.ChangedChapters {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rawChapter, &probe); err != nil {
			return nil, fmt.Errorf("failed to parse changed chapter patch: %w", err)
		}
		if strings.TrimSpace(probe.ID) == "" {
			return nil, fmt.Errorf("changed_chapters item missing id")
		}
		found := false
		for i := range volume.Chapters {
			if volume.Chapters[i].ID != probe.ID {
				continue
			}
			found = true
			beforeChapter := volume.Chapters[i]
			merged, err := MergeJSONPatchObject(beforeChapter, rawChapter)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(merged.ID) != "" && merged.ID != beforeChapter.ID {
				return nil, fmt.Errorf("patch chapter id %q does not match target chapter %q", merged.ID, beforeChapter.ID)
			}
			merged.ID = beforeChapter.ID
			volume.Chapters[i] = merged
			changes = append(changes, DiffStructFields("outline.chapter."+probe.ID, beforeChapter, merged)...)
			break
		}
		if !found {
			return nil, fmt.Errorf("changed chapter %q is not in target volume %q", probe.ID, volumeID)
		}
	}
	for _, rawEvent := range patch.ChangedEvents {
		eventChanges, err := applyVolumeEventPatch(volume, volumeID, rawEvent)
		if err != nil {
			return nil, err
		}
		changes = append(changes, eventChanges...)
	}
	return changes, nil
}

func applyVolumeEventPatch(volume *models.Volume, volumeID string, rawEvent json.RawMessage) ([]ToolPatchChange, error) {
	var probe struct {
		ChapterID  string `json:"chapter_id"`
		EventIndex *int   `json:"event_index"`
		Index      *int   `json:"index"`
	}
	if err := json.Unmarshal(rawEvent, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse changed event patch: %w", err)
	}
	chapterID := strings.TrimSpace(probe.ChapterID)
	if chapterID == "" {
		return nil, fmt.Errorf("changed_events item missing chapter_id")
	}
	eventIndex := -1
	if probe.EventIndex != nil {
		eventIndex = *probe.EventIndex
	} else if probe.Index != nil {
		eventIndex = *probe.Index
	} else {
		return nil, fmt.Errorf("changed_events item missing event_index")
	}
	if eventIndex < 0 {
		return nil, fmt.Errorf("changed_events item has negative event_index %d", eventIndex)
	}
	var patchFields map[string]json.RawMessage
	if err := json.Unmarshal(rawEvent, &patchFields); err != nil {
		return nil, fmt.Errorf("failed to parse changed event patch fields: %w", err)
	}
	delete(patchFields, "chapter_id")
	delete(patchFields, "event_index")
	delete(patchFields, "index")
	if len(patchFields) == 0 {
		return nil, fmt.Errorf("changed_events item for %s[%d] has no event fields to patch", chapterID, eventIndex)
	}
	patchBytes, err := json.Marshal(patchFields)
	if err != nil {
		return nil, fmt.Errorf("failed to encode changed event patch: %w", err)
	}
	for chapterIdx := range volume.Chapters {
		if volume.Chapters[chapterIdx].ID != chapterID {
			continue
		}
		if eventIndex >= len(volume.Chapters[chapterIdx].Events) {
			return nil, fmt.Errorf("changed_events index %d out of range for chapter %q", eventIndex, chapterID)
		}
		before := volume.Chapters[chapterIdx].Events[eventIndex]
		merged, err := MergeJSONPatchObject(before, patchBytes)
		if err != nil {
			return nil, err
		}
		volume.Chapters[chapterIdx].Events[eventIndex] = merged
		return DiffStructFields(fmt.Sprintf("outline.event.%s.%d", chapterID, eventIndex), before, merged), nil
	}
	return nil, fmt.Errorf("changed event chapter %q is not in target volume %q", chapterID, volumeID)
}

func ApplyCraftElementPatch(values map[string]map[string]interface{}, kind, id string, patchBytes []byte) (string, string, []ToolPatchChange, error) {
	var patch map[string]interface{}
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if len(patch) == 0 {
		return "", "", nil, fmt.Errorf("craft patch must be a non-empty JSON object")
	}
	key, exists := resolveCraftPatchKey(values, id)
	if key == "" {
		key = id
	}
	before := copyStringInterfaceMap(values[key])
	base := copyStringInterfaceMap(before)
	for field, value := range patch {
		if field == "id" {
			continue
		}
		base[field] = value
	}
	if _, ok := base["name"]; !ok {
		base["name"] = key
	}
	normalized, err := NormalizeCraftPatchObject(kind, key, base)
	if err != nil {
		return "", "", nil, err
	}
	values[key] = normalized
	action := "update"
	if !exists {
		action = "create"
	}
	return key, action, diffJSONMaps("craft."+kind+"."+key, before, normalized), nil
}

func normalizeCraftPatchTarget(target string) string {
	switch NormalizeKey(target) {
	case "character", "characters", "char", "chars":
		return "character"
	case "item", "items":
		return "item"
	case "location", "locations", "loc", "locs":
		return "location"
	case "organization", "organizations", "org", "orgs":
		return "organization"
	default:
		return NormalizeKey(target)
	}
}

func craftPatchFilename(kind string) (string, error) {
	switch normalizeCraftPatchTarget(kind) {
	case "character":
		return "characters.json", nil
	case "item":
		return "items.json", nil
	case "location":
		return "locations.json", nil
	case "organization":
		return "organizations.json", nil
	default:
		return "", fmt.Errorf("unsupported craft patch target %q", kind)
	}
}

func resolveCraftPatchKey(values map[string]map[string]interface{}, id string) (string, bool) {
	if _, ok := values[id]; ok {
		return id, true
	}
	lowerID := strings.ToLower(strings.TrimSpace(id))
	for key, value := range values {
		if strings.ToLower(key) == lowerID {
			return key, true
		}
		if rawName, ok := value["name"].(string); ok && strings.ToLower(strings.TrimSpace(rawName)) == lowerID {
			return key, true
		}
	}
	return "", false
}

func NormalizeCraftPatchObject(kind, key string, raw map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	switch normalizeCraftPatchTarget(kind) {
	case "character":
		if err := normalizeCraftRPGStatsPatch(raw); err != nil {
			return nil, err
		}
		data, err = json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		var value models.Character
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft character patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		out, err := mergeNormalizedCraftObject(raw, value)
		if _, ok := raw["power_level"]; ok {
			out["power_level"] = value.PowerLevel
		}
		return out, err
	case "item":
		var value models.Item
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft item patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		out, err := mergeNormalizedCraftObject(raw, value)
		if _, ok := raw["power_level"]; ok {
			out["power_level"] = value.PowerLevel
		}
		return out, err
	case "location":
		var value models.Location
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft location patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		out, err := mergeNormalizedCraftObject(raw, value)
		if _, ok := raw["danger_level"]; ok {
			out["danger_level"] = value.DangerLevel
		}
		return out, err
	case "organization":
		var value models.Organization
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft organization patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		return mergeNormalizedCraftObject(raw, value)
	default:
		return nil, fmt.Errorf("unsupported craft patch target %q", kind)
	}
}

func normalizeCraftRPGStatsPatch(raw map[string]interface{}) error {
	stats, ok := raw["rpg_stats"]
	if !ok || stats == nil {
		return nil
	}
	statsMap, ok := stats.(map[string]interface{})
	if !ok {
		return fmt.Errorf("craft character rpg_stats must be an object with supported keys: str, agi, int, vit, hp, mp, level")
	}
	normalized := map[string]interface{}{}
	for key, value := range statsMap {
		canonical, ok := normalizeCraftRPGStatKey(key)
		if !ok {
			return fmt.Errorf("unsupported craft character rpg_stats key %q; use supported keys: str, agi, int, vit, hp, mp, level", key)
		}
		if _, exists := normalized[canonical]; exists {
			return fmt.Errorf("duplicate craft character rpg_stats key %q after normalization", canonical)
		}
		number, err := craftRPGStatNumber(value)
		if err != nil {
			return fmt.Errorf("craft character rpg_stats.%s: %w", key, err)
		}
		normalized[canonical] = number
	}
	raw["rpg_stats"] = normalized
	return nil
}

func normalizeCraftRPGStatKey(key string) (string, bool) {
	switch NormalizeKey(key) {
	case "str", "strength", "力量":
		return "str", true
	case "agi", "agility", "dex", "dexterity", "敏捷":
		return "agi", true
	case "int", "intelligence", "智力":
		return "int", true
	case "vit", "vitality", "endurance", "stamina", "体质", "耐力":
		return "vit", true
	case "hp", "health", "生命", "生命值":
		return "hp", true
	case "mp", "mana", "energy", "mind", "精神", "能量", "法力":
		return "mp", true
	case "level", "lvl", "等级":
		return "level", true
	default:
		return "", false
	}
}

func craftRPGStatNumber(value interface{}) (int, error) {
	switch typed := value.(type) {
	case float64:
		number := int(typed)
		if typed != float64(number) {
			return 0, fmt.Errorf("must be an integer")
		}
		return number, nil
	case int:
		return typed, nil
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(number), nil
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

func mergeNormalizedCraftObject(raw map[string]interface{}, normalized interface{}) (map[string]interface{}, error) {
	out := copyStringInterfaceMap(raw)
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	var normalizedMap map[string]interface{}
	if err := json.Unmarshal(data, &normalizedMap); err != nil {
		return nil, err
	}
	for key, value := range normalizedMap {
		out[key] = value
	}
	if err := utils.ValidateNoSuspiciousPatchText(out); err != nil {
		return nil, err
	}
	return out, nil
}

func diffJSONMaps(prefix string, before, after map[string]interface{}) []ToolPatchChange {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	changes := make([]ToolPatchChange, 0)
	for key := range keys {
		if reflect.DeepEqual(before[key], after[key]) {
			continue
		}
		action := "replace"
		if _, ok := before[key]; !ok {
			action = "add"
		} else if _, ok := after[key]; !ok {
			action = "remove"
		}
		changes = append(changes, ToolPatchChange{Path: prefix + "." + key, Action: action})
	}
	return changes
}

func copyStringInterfaceMap(value map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for key, item := range value {
		out[key] = item
	}
	return out
}

func cloneRawCraftMap(values map[string]map[string]interface{}) map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = copyStringInterfaceMap(value)
	}
	return out
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

func DiffStructFields(prefix string, before, after interface{}) []ToolPatchChange {
	beforeMap := structJSONMap(before)
	afterMap := structJSONMap(after)
	keys := map[string]bool{}
	for key := range beforeMap {
		keys[key] = true
	}
	for key := range afterMap {
		keys[key] = true
	}
	changes := make([]ToolPatchChange, 0)
	for key := range keys {
		if reflect.DeepEqual(beforeMap[key], afterMap[key]) {
			continue
		}
		action := "replace"
		if _, ok := beforeMap[key]; !ok {
			action = "add"
		} else if _, ok := afterMap[key]; !ok {
			action = "remove"
		}
		changes = append(changes, ToolPatchChange{Path: prefix + "." + key, Action: action})
	}
	return changes
}

func structJSONMap(value interface{}) map[string]interface{} {
	data, _ := json.Marshal(value)
	var out map[string]interface{}
	_ = json.Unmarshal(data, &out)
	return out
}

func CloneOutline(outline *models.Outline) *models.Outline {
	if outline == nil {
		return nil
	}
	data, _ := json.Marshal(outline)
	var cloned models.Outline
	_ = json.Unmarshal(data, &cloned)
	return &cloned
}

func CloneStorySetup(setup *models.StorySetup) models.StorySetup {
	if setup == nil {
		return models.StorySetup{}
	}
	data, _ := json.Marshal(setup)
	var cloned models.StorySetup
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

func saveToolPatchCheckpoint(root string, outline *models.Outline) (string, error) {
	dir := filepath.Join(root, "story", "compose", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "outline_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func SaveToolSetupPatchCheckpoint(root string, setup models.StorySetup) (string, error) {
	dir := filepath.Join(root, "story", "setup", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "story_setup_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(setup, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveToolCraftPatchCheckpoint(root, filename string, values map[string]map[string]interface{}) (string, error) {
	dir := filepath.Join(root, "story", "craft", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	path := filepath.Join(dir, base+"_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveToolRecapPatchCheckpoint(root, chapterID string, value *models.ChapterRecap) (string, error) {
	dir := filepath.Join(root, "story", "recaps", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, chapterID+"_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveToolChapterPatchCheckpoint(root, chapterID, beforePath, beforeContent string) (string, error) {
	dir := filepath.Join(root, "chapters", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, chapterID+"_"+time.Now().Format("20060102_150405")+".md")
	var sb strings.Builder
	sb.WriteString("<!-- novelgen tool patch chapter checkpoint")
	if strings.TrimSpace(beforePath) != "" {
		sb.WriteString(" source=\"")
		sb.WriteString(beforePath)
		sb.WriteString("\"")
	}
	sb.WriteString(" -->\n\n")
	sb.WriteString(beforeContent)
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func SaveRawCraftMap(path string, values map[string]map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func writeJSON(cmd *cobra.Command, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
