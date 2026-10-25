// Setup patches: merge semantics for the typed story setup (string lists, long-form plan, writing
// style and keyed item lists).

package mutation

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

type setupStringListPatchItem struct {
	Index int    `json:"index"`
	Value string `json:"value"`
}

func runSetupMutation(root *store.Root, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	var result *ToolPatchResult
	err := root.WithMutationLock(func() error {
		var err error
		result, err = runSetupMutationLocked(root, patchBytes, opts, warnings)
		return err
	})
	return result, err
}

func runSetupMutationLocked(root *store.Root, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	opts = opts.normalized()
	setupPath := root.SetupJSON()
	exists, err := root.ExistsErr(setupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect story setup: %w", err)
	}
	before := models.StorySetup{}
	current := models.StorySetup{}
	if exists {
		setup, err := root.LoadSetup()
		if err != nil {
			return nil, fmt.Errorf("failed to load story setup: %w", err)
		}
		before = CloneStorySetup(setup)
		current = CloneStorySetup(setup)
	} else if opts.Mode != MutationReplace {
		return nil, fmt.Errorf("failed to load story setup: story setup does not exist")
	}

	var merged models.StorySetup
	if opts.Mode == MutationReplace {
		if err := decodeStrictToolPatch(patchBytes, &merged); err != nil {
			return nil, fmt.Errorf("failed to parse story setup: %w", err)
		}
	} else {
		var setupPatchWarnings []string
		merged, setupPatchWarnings, err = ApplySetupPatchObject(current, patchBytes)
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, setupPatchWarnings...)
	}
	if err := utils.ValidateNoSuspiciousPatchText(merged); err != nil {
		return nil, fmt.Errorf("setup patch rejected: %w", err)
	}
	models.NormalizeStorySetup(&merged)

	var check *ToolCheckResult
	if opts.Validate {
		check, err = RunToolSetupCheck("all", &merged)
		if err != nil {
			return nil, err
		}
		addToolCheckMeta(check, "project_root", root.Dir())
		if opts.Apply && check.Blocking {
			return nil, fmt.Errorf("patch rejected: setup quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
		}
	}

	files := map[string]string{}
	changes := DiffStructFields("setup", before, merged)
	warnings = appendNoChangeWarning(warnings, changes)
	if opts.Apply {
		checkpointPath := ""
		if exists {
			checkpointPath, err = saveToolCheckpointJSON(root, store.ScopeSetup, "story_setup", before)
			if err != nil {
				return nil, err
			}
		}
		// SaveSetup is the store's atomic, normalizing write; the patch plane used
		// to call models.StorySetup.Save here, which wrote the canonical file with
		// a plain os.WriteFile and could leave a truncated story_setup.json.
		if err := root.SaveSetup(&merged); err != nil {
			return nil, fmt.Errorf("failed to save patched story setup: %w", err)
		}
		mdPath, err := project.Open(root).SaveStorySetupMarkdown(&merged)
		if err != nil {
			return nil, fmt.Errorf("failed to save story setup markdown: %w", err)
		}
		files["setup"] = setupPath
		files["markdown"] = mdPath
		if checkpointPath != "" {
			files["checkpoint"] = checkpointPath
		}
		warnings = markMemoryDirtyAfterWrite(root, "setup", warnings)
	}

	ok := true
	if check != nil {
		ok = !check.Blocking
	}
	result := ToolPatchResult{
		OK:          ok,
		Applied:     opts.Apply,
		DryRun:      !opts.Apply,
		Target:      "setup",
		ID:          "story_setup",
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(opts.Apply, check, "novelgen tool check all --target setup --min-priority medium --max-issues 12"),
		Meta: map[string]interface{}{
			"project_root": root.Dir(),
			"mode":         string(opts.Mode),
			"source":       opts.Source,
		},
	}
	return &result, nil
}

func ApplySetupPatchObject(setup models.StorySetup, patchBytes []byte) (models.StorySetup, []string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchBytes, &raw); err != nil {
		return setup, nil, fmt.Errorf("failed to parse setup patch JSON: %w", err)
	}
	if len(raw) == 0 {
		return setup, nil, fmt.Errorf("setup patch must contain at least one field")
	}
	if err := validateSetupPatchKeys(raw); err != nil {
		return setup, nil, err
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
		merged.CoreCast, err = upsertSetupPatchItems(merged.CoreCast, value, setupCoreCastPatchKeys, "core_cast")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "core_cast")
	}
	if value, ok := raw["storylines"]; ok {
		merged.Storylines, err = upsertSetupPatchItems(merged.Storylines, value, func(item models.Storyline) []string { return setupNamePatchKeys(item.Name) }, "storylines")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "storylines")
	}
	if value, ok := raw["premises"]; ok {
		merged.Premises, err = upsertSetupPatchItems(merged.Premises, value, func(item models.Premise) []string { return setupNamePatchKeys(item.Name) }, "premises")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "premises")
	}
	if value, ok := raw["world_timeline"]; ok {
		merged.WorldTimeline, err = upsertSetupPatchItems(merged.WorldTimeline, value, setupTimelinePatchKeys, "world_timeline")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "world_timeline")
	}
	if value, ok := raw["world_resources"]; ok {
		merged.WorldResources, err = upsertSetupPatchItems(merged.WorldResources, value, func(item models.WorldResource) []string { return setupNamePatchKeys(item.Name) }, "world_resources")
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

func validateSetupPatchKeys(raw map[string]json.RawMessage) error {
	allowed := structJSONFields(models.StorySetup{})
	allowed["rules_patch"] = true
	allowed["genres_patch"] = true
	if err := unknownPatchFieldsError("setup", raw, allowed); err != nil {
		return err
	}
	if value, ok := raw["long_form_plan"]; ok && !isJSONNull(value) {
		var plan models.LongFormPlan
		if err := decodeStrictToolPatch(value, &plan); err != nil {
			return fmt.Errorf("setup patch long_form_plan does not match the schema: %w", err)
		}
	}
	if value, ok := raw["writing_style"]; ok && !isJSONNull(value) {
		var style models.WritingStyle
		if err := decodeStrictToolPatch(value, &style); err != nil {
			return fmt.Errorf("setup patch writing_style does not match the schema: %w", err)
		}
	}
	return nil
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
	var rawItems []json.RawMessage
	if err := json.Unmarshal(patchRaw, &rawItems); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of {index,value}: %w", path, err)
	}
	items := make([]setupStringListPatchItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		var item setupStringListPatchItem
		if err := decodeStrictToolPatch(rawItem, &item); err != nil {
			return nil, fmt.Errorf("setup patch %s item does not match {index,value}: %w", path, err)
		}
		items = append(items, item)
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

func upsertSetupPatchItems[T any](existing []T, patchRaw json.RawMessage, keyFns func(T) []string, path string) ([]T, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(patchRaw, &raws); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of objects: %w", path, err)
	}
	out := append([]T(nil), existing...)
	for _, raw := range raws {
		var item T
		if err := decodeStrictToolPatch(raw, &item); err != nil {
			return nil, fmt.Errorf("setup patch %s item does not match schema: %w", path, err)
		}
		itemKeys := keyFns(item)
		if len(itemKeys) == 0 {
			return nil, fmt.Errorf("setup patch %s item needs a stable key (name, id, year+event, or related_mystery)", path)
		}
		match := -1
		for _, patchKey := range itemKeys {
			if patchKey == "" {
				continue
			}
			matches := make([]int, 0, 1)
			for i := range out {
				if setupPatchItemHasKey(out[i], keyFns, patchKey) {
					matches = append(matches, i)
				}
			}
			if strings.HasPrefix(patchKey, "year:") && len(matches) > 1 {
				return nil, fmt.Errorf("setup patch %s year %q matches multiple timeline entries; include event or related_mystery to disambiguate", path, strings.TrimPrefix(patchKey, "year:"))
			}
			if len(matches) > 0 {
				match = matches[0]
				break
			}
		}
		if match < 0 {
			out = append(out, item)
			continue
		}
		merged, err := MergeJSONPatchObject(out[match], raw)
		if err != nil {
			return nil, fmt.Errorf("setup patch %s item does not match schema: %w", path, err)
		}
		out[match] = merged
	}
	return out, nil
}

func setupPatchItemHasKey[T any](item T, keyFns func(T) []string, want string) bool {
	for _, key := range keyFns(item) {
		if key == want {
			return true
		}
	}
	return false
}

func setupNamePatchKeys(name string) []string {
	key := normalizeSetupPatchKey(name)
	if key == "" {
		return nil
	}
	return []string{"name:" + key}
}

func setupCoreCastPatchKeys(item models.CoreCastSeed) []string {
	keys := make([]string, 0, 2)
	if id := strings.TrimSpace(item.ID); id != "" {
		keys = append(keys, "id:"+normalizeSetupPatchKey(id))
	}
	if name := normalizeSetupPatchKey(item.Name); name != "" {
		keys = append(keys, "name:"+name)
	}
	return keys
}

func setupTimelinePatchKeys(item models.WorldTimelineEntry) []string {
	keys := make([]string, 0, 3)
	if strings.TrimSpace(item.RelatedMystery) != "" {
		keys = append(keys, "mystery:"+normalizeSetupPatchKey(item.RelatedMystery))
	}
	if strings.TrimSpace(item.Year) != "" || strings.TrimSpace(item.Event) != "" {
		keys = append(keys, "time:"+normalizeSetupPatchKey(item.Year)+"|"+normalizeSetupPatchKey(item.Event))
	}
	if strings.TrimSpace(item.Year) != "" {
		keys = append(keys, "year:"+normalizeSetupPatchKey(item.Year))
	}
	return keys
}

func normalizeSetupPatchKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
