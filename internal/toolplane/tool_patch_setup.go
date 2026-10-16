// Setup patches: merge semantics for the typed story setup (string lists, long-form plan, writing
// style and keyed item lists).

package toolplane

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"
	"novelgen/internal/utils"
)

type setupStringListPatchItem struct {
	Index int    `json:"index"`
	Value string `json:"value"`
}

func RunToolSetupPatch(cmd *cobra.Command, root *store.Root, patchBytes []byte, apply bool, warnings []string) error {
	setupPath := root.SetupJSON()
	setup, err := root.LoadSetup()
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
	addToolCheckMeta(check, "project_root", root.Dir())
	if apply && check.Blocking {
		return fmt.Errorf("patch rejected: setup quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolCheckpointJSON(root, store.ScopeSetup, "story_setup", before)
		if err != nil {
			return err
		}
		// SaveSetup is the store's atomic, normalizing write; the patch plane used
		// to call models.StorySetup.Save here, which wrote the canonical file with
		// a plain os.WriteFile and could leave a truncated story_setup.json.
		if err := root.SaveSetup(&merged); err != nil {
			return fmt.Errorf("failed to save patched story setup: %w", err)
		}
		mdPath, err := project.Open(root).SaveStorySetupMarkdown(&merged)
		if err != nil {
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
			"project_root": root.Dir(),
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
