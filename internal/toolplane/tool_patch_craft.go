// Craft patches: target/key resolution and the typed normalization of a single craft element.

package toolplane

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/toolcmd"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/models"
	"novelgen/internal/quality"
	"novelgen/internal/store"
	"novelgen/internal/utils"
)

func RunToolCraftPatch(cmd *cobra.Command, root *store.Root, target, id string, patchBytes []byte, apply bool, warnings []string) error {
	filename, err := craftPatchFilename(target)
	if err != nil {
		return err
	}
	kind, ok := store.CraftKindForFile(filename)
	if !ok {
		return fmt.Errorf("unknown craft file %q", filename)
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
	if apply && check.Blocking {
		return fmt.Errorf("patch rejected: craft schema check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}
	if apply {
		base := strings.TrimSuffix(filename, filepath.Ext(filename))
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

	result := ToolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "craft." + target,
		ID:          key,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, toolcmd.Check(toolcmd.CheckOptions{Kind: "schema", Target: "craft", Scope: target, ID: key})),
		Meta: map[string]interface{}{
			"project_root": root.Dir(),
			"action":       action,
		},
	}
	return writeJSON(cmd, result)
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
