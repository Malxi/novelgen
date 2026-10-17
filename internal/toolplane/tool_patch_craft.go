// Craft patches: target/key resolution and the typed normalization of a single craft element.

package toolplane

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/toolcmd"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/models"
	"novelgen/internal/quality"
	"novelgen/internal/store"
	"novelgen/internal/utils"
)

func RunToolCraftPatch(cmd *cobra.Command, root *store.Root, target, id string, patchBytes []byte, apply bool, warnings []string) error {
	result, err := runCraftMutation(root, target, id, patchBytes, MutationOptions{
		Apply:    apply,
		Validate: true,
		Mode:     MutationMerge,
		Source:   "tool-cli",
	}, warnings)
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func runCraftMutation(root *store.Root, target, id string, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	var result *ToolPatchResult
	err := root.WithMutationLock(func() error {
		var err error
		result, err = runCraftMutationLocked(root, target, id, patchBytes, opts, warnings)
		return err
	})
	return result, err
}

func runCraftMutationLocked(root *store.Root, target, id string, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	opts = opts.normalized()
	filename, err := craftPatchFilename(target)
	if err != nil {
		return nil, err
	}
	kind, ok := store.CraftKindForFile(filename)
	if !ok {
		return nil, fmt.Errorf("unknown craft file %q", filename)
	}
	values, err := loadRawCraftMap(root, filename)
	if err != nil {
		return nil, err
	}
	if values == nil {
		values = map[string]map[string]interface{}{}
	}

	beforeValues := cloneRawCraftMap(values)
	var key, action string
	var changes []ToolPatchChange
	if opts.Mode == MutationReplace {
		key, action, changes, err = ApplyCraftElementReplace(values, target, id, patchBytes)
	} else {
		key, action, changes, err = ApplyCraftElementPatch(values, target, id, patchBytes)
	}
	if err != nil {
		return nil, err
	}
	warnings = appendNoChangeWarning(warnings, changes)

	gate := quality.QualityGateResult{}
	validateCraftSchemaObject(&gate, target, key, values[key])
	if normalizeCraftPatchTarget(target) == "item" {
		validateCraftConsistencyObject(&gate, target, key, values[key], loadCraftCharacterNames(root))
	}
	validateCraftQualityObject(&gate, target, key, values[key])
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	check := MakeToolCheckResult("schema", "craft", target, key, gate)
	addToolCheckMeta(check, "project_root", root.Dir())

	files := map[string]string{}
	if opts.Apply && opts.Validate && check.Blocking {
		return nil, fmt.Errorf("patch rejected: craft schema check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}
	if opts.Apply {
		base := strings.TrimSuffix(filename, filepath.Ext(filename))
		checkpointPath, err := saveToolCheckpointJSON(root, store.ScopeCraft, base, beforeValues)
		if err != nil {
			return nil, err
		}
		path := root.CraftFile(kind)
		if err := root.WriteJSON(path, values); err != nil {
			return nil, err
		}
		files["craft"] = path
		files["checkpoint"] = checkpointPath
	}

	ok = !check.Blocking || !opts.Validate
	result := ToolPatchResult{
		OK:          ok,
		Applied:     opts.Apply,
		DryRun:      !opts.Apply,
		Target:      "craft." + target,
		ID:          key,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(opts.Apply, check, toolcmd.Check(toolcmd.CheckOptions{Kind: "schema", Target: "craft", Scope: target, ID: key})),
		Meta: map[string]interface{}{
			"project_root": root.Dir(),
			"action":       action,
			"mode":         string(opts.Mode),
			"source":       opts.Source,
		},
	}
	return &result, nil
}

// runCraftBulkMutation upserts a generated batch of elements while preserving
// unrelated entries in the same craft file. It is the host-pipeline counterpart
// of the single-element `tool patch craft` command.
func runCraftBulkMutation(root *store.Root, kind store.CraftKind, elements map[string]map[string]interface{}, opts MutationOptions, warnings []string, replace bool) (*ToolPatchResult, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	if !kind.Valid() {
		return nil, fmt.Errorf("unknown craft kind %q", kind)
	}
	opts = opts.normalized()
	if len(elements) == 0 {
		return &ToolPatchResult{
			OK:       true,
			Applied:  false,
			DryRun:   !opts.Apply,
			Target:   "craft." + strings.TrimSuffix(string(kind), "s"),
			Warnings: append(warnings, "no craft elements to write"),
			Meta: map[string]interface{}{
				"project_root": root.Dir(),
				"mode":         string(opts.Mode),
				"source":       opts.Source,
				"count":        0,
			},
		}, nil
	}
	var result *ToolPatchResult
	err := root.WithMutationLock(func() error {
		diskValues, err := root.LoadCraftRaw(kind)
		if err != nil {
			return err
		}
		if diskValues == nil {
			diskValues = map[string]map[string]interface{}{}
		}
		beforeValues := cloneRawCraftMap(diskValues)
		values := diskValues
		if replace {
			values = map[string]map[string]interface{}{}
		}
		target := strings.TrimSuffix(string(kind), "s")
		var changes []ToolPatchChange
		keys := make([]string, 0, len(elements))
		for key := range elements {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			raw := elements[key]
			normalized, err := NormalizeCraftPatchObject(target, key, raw)
			if err != nil {
				return err
			}
			before := copyStringInterfaceMap(values[key])
			values[key] = normalized
			action := "update"
			if _, exists := beforeValues[key]; !exists {
				action = "create"
			}
			beforeChangeCount := len(changes)
			changes = append(changes, diffJSONMaps("craft."+target+"."+key, before, normalized)...)
			if len(changes) == beforeChangeCount && action == "create" {
				changes = append(changes, ToolPatchChange{Path: "craft." + target + "." + key, Action: action})
			}
		}
		if replace {
			for key := range beforeValues {
				if _, exists := elements[key]; exists {
					continue
				}
				changes = append(changes, ToolPatchChange{Path: "craft." + target + "." + key, Action: "delete"})
			}
			sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
		}
		warnings = appendNoChangeWarning(warnings, changes)

		gate := quality.QualityGateResult{}
		knownCharacters := loadCraftCharacterNames(root)
		for _, key := range keys {
			validateCraftSchemaObject(&gate, target, key, values[key])
			if target == "item" {
				validateCraftConsistencyObject(&gate, target, key, values[key], knownCharacters)
			}
			validateCraftQualityObject(&gate, target, key, values[key])
		}
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
		check := MakeToolCheckResult("schema", "craft", target, "", gate)
		addToolCheckMeta(check, "project_root", root.Dir())
		if opts.Apply && opts.Validate && check.Blocking {
			return fmt.Errorf("patch rejected: craft schema check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
		}

		files := map[string]string{}
		if opts.Apply {
			base := strings.TrimSuffix(kind.FileName(), filepath.Ext(kind.FileName()))
			checkpointPath, err := saveToolCheckpointJSON(root, store.ScopeCraft, base, beforeValues)
			if err != nil {
				return err
			}
			path := root.CraftFile(kind)
			if err := root.WriteJSON(path, values); err != nil {
				return err
			}
			files["craft"] = path
			files["checkpoint"] = checkpointPath
		}
		ok := !check.Blocking || !opts.Validate
		result = &ToolPatchResult{
			OK:       ok,
			Applied:  opts.Apply,
			DryRun:   !opts.Apply,
			Target:   "craft." + target,
			Changed:  changes,
			Check:    check,
			Files:    files,
			Warnings: warnings,
			Meta: map[string]interface{}{
				"project_root": root.Dir(),
				"mode":         string(opts.Mode),
				"source":       opts.Source,
				"count":        len(keys),
			},
		}
		return nil
	})
	return result, err
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

func ApplyCraftElementReplace(values map[string]map[string]interface{}, kind, id string, patchBytes []byte) (string, string, []ToolPatchChange, error) {
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
	raw := copyStringInterfaceMap(patch)
	if _, ok := raw["name"]; !ok {
		raw["name"] = key
	}
	normalized, err := NormalizeCraftPatchObject(kind, key, raw)
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
		return store.CraftCharacters.FileName(), nil
	case "item":
		return store.CraftItems.FileName(), nil
	case "location":
		return store.CraftLocations.FileName(), nil
	case "organization":
		return store.CraftOrganizations.FileName(), nil
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
