// Package craft owns deterministic normalization for generated craft objects.
// The tool patch plane and the schema checks both consume these helpers, so the
// rules live in the fiction domain rather than in a CLI adapter.
package craft

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

// NormalizeTarget maps the craft target spellings accepted by commands and
// agents to the four canonical kinds.
func NormalizeTarget(target string) string {
	switch utils.NormalizeKey(target) {
	case "character", "characters", "char", "chars":
		return "character"
	case "item", "items":
		return "item"
	case "location", "locations", "loc", "locs":
		return "location"
	case "organization", "organizations", "org", "orgs":
		return "organization"
	default:
		return utils.NormalizeKey(target)
	}
}

// Filename returns the canonical craft file name for a target kind.
func Filename(kind string) (string, error) {
	switch NormalizeTarget(kind) {
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

// ResolveKey finds the stored key for an id or display name.
func ResolveKey(values map[string]map[string]interface{}, id string) (string, bool) {
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

// NormalizePatchObject merges a raw patch object onto the typed craft model and
// returns the canonical map that should be stored.
func NormalizePatchObject(kind, key string, raw map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	switch NormalizeTarget(kind) {
	case "character":
		if err := normalizeRPGStatsPatch(raw); err != nil {
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
		out, err := mergeNormalizedObject(raw, value)
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
		out, err := mergeNormalizedObject(raw, value)
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
		out, err := mergeNormalizedObject(raw, value)
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
		return mergeNormalizedObject(raw, value)
	default:
		return nil, fmt.Errorf("unsupported craft patch target %q", kind)
	}
}

func normalizeRPGStatsPatch(raw map[string]interface{}) error {
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
		canonical, ok := normalizeRPGStatKey(key)
		if !ok {
			return fmt.Errorf("unsupported craft character rpg_stats key %q; use supported keys: str, agi, int, vit, hp, mp, level", key)
		}
		if _, exists := normalized[canonical]; exists {
			return fmt.Errorf("duplicate craft character rpg_stats key %q after normalization", canonical)
		}
		number, err := rpgStatNumber(value)
		if err != nil {
			return fmt.Errorf("craft character rpg_stats.%s: %w", key, err)
		}
		normalized[canonical] = number
	}
	raw["rpg_stats"] = normalized
	return nil
}

func normalizeRPGStatKey(key string) (string, bool) {
	switch utils.NormalizeKey(key) {
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

func rpgStatNumber(value interface{}) (int, error) {
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

func mergeNormalizedObject(raw map[string]interface{}, normalized interface{}) (map[string]interface{}, error) {
	out := CopyMap(raw)
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

// CopyMap returns a shallow copy of a raw craft object.
func CopyMap(value map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for key, item := range value {
		out[key] = item
	}
	return out
}
