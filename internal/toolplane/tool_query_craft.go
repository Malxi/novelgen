// The craft query section: typed and raw craft element lookup plus the compact projections used by
// the craft context bundles.

package toolplane

import (
	"fmt"
	"novelgen/internal/store"
	"sort"
	"strings"
)

func QueryCraft(ctx ToolProjectContext, queryType, name string) ToolResponse {
	switch normalizeCraftType(queryType) {
	case "", "all":
		results := map[string]interface{}{
			"characters":    rawOrTyped(ctx.RawCharacters, ctx.Characters),
			"items":         rawOrTyped(ctx.RawItems, ctx.Items),
			"locations":     rawOrTyped(ctx.RawLocations, ctx.Locations),
			"organizations": rawOrTyped(ctx.RawOrgs, ctx.Organizations),
		}
		return ToolResponse{OK: true, Count: len(ctx.Characters) + len(ctx.Items) + len(ctx.Locations) + len(ctx.Organizations), Results: results}
	case "character":
		if ctx.RawCharacters != nil {
			return queryNamedRawElement("character", name, ctx.RawCharacters)
		}
		return queryNamedElement("character", name, ctx.Characters)
	case "item":
		if ctx.RawItems != nil {
			return queryNamedRawElement("item", name, ctx.RawItems)
		}
		return queryNamedElement("item", name, ctx.Items)
	case "location":
		if ctx.RawLocations != nil {
			return queryNamedRawElement("location", name, ctx.RawLocations)
		}
		return queryNamedElement("location", name, ctx.Locations)
	case "organization":
		if ctx.RawOrgs != nil {
			return queryNamedRawElement("organization", name, ctx.RawOrgs)
		}
		return queryNamedElement("organization", name, ctx.Organizations)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported craft type %q", queryType)}}
	}
}

// loadRawCraftMap reads one craft element file as the string-keyed map the patch
// plane and the schema checks work with. Missing files yield an empty map, so a
// patch against a not-yet-crafted element creates it.
func loadRawCraftMap(root *store.Root, filename string) (map[string]map[string]interface{}, error) {
	kind, ok := store.CraftKindForFile(filename)
	if !ok {
		return nil, fmt.Errorf("unknown craft file %q", filename)
	}
	values, err := root.LoadCraftRaw(kind)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}
	return values, nil
}

func queryNamedElement[T any](kind, name string, values map[string]*T) ToolResponse {
	hits := make([]map[string]interface{}, 0)
	for key, value := range values {
		if value == nil {
			continue
		}
		if name == "" || namedObjectMatches(key, value, name) {
			hits = append(hits, map[string]interface{}{
				"key":    key,
				"object": value,
			})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		return fmt.Sprint(hits[i]["key"]) < fmt.Sprint(hits[j]["key"])
	})
	return ToolResponse{OK: true, Section: "story-setup", Count: len(hits), Results: hits}
}

func rawObjectMatches(key string, value map[string]interface{}, name string) bool {
	if strings.TrimSpace(name) == "" {
		return true
	}
	return textMatches(key, name) || objectNameMatches(value, name)
}

func queryNamedRawElement(kind, name string, values map[string]map[string]interface{}) ToolResponse {
	hits := make([]map[string]interface{}, 0)
	for key, value := range values {
		if value == nil {
			continue
		}
		if name == "" || rawObjectMatches(key, value, name) {
			hits = append(hits, map[string]interface{}{
				"type":   kind,
				"key":    key,
				"object": value,
			})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		return fmt.Sprint(hits[i]["key"]) < fmt.Sprint(hits[j]["key"])
	})
	return ToolResponse{OK: true, Section: "craft", Count: len(hits), Results: hits}
}

func compactCraftObject(raw map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, key := range []string{
		"name", "aliases", "role_in_story", "rpg_role", "combat_role",
		"type", "description", "appearance", "personality", "background",
		"motivation", "skills", "abilities", "voice", "function", "significance",
		"atmosphere", "goals", "rarity", "power_level", "danger_level",
		"resource_tags", "encounter_tags", "members", "allies", "enemies",
		"dsl_tags", "affiliations", "notes",
	} {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			out[key] = clipToolString(typed, 260)
		case []interface{}:
			out[key] = clipInterfaceStrings(typed, 6, 120)
		default:
			out[key] = value
		}
	}
	out["available_fields"] = sortedMapKeys(raw)
	return out
}

func clipInterfaceStrings(values []interface{}, maxItems, maxRunes int) []interface{} {
	if maxItems > 0 && len(values) > maxItems {
		values = values[:maxItems]
	}
	out := make([]interface{}, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, clipToolString(text, maxRunes))
			continue
		}
		out = append(out, value)
	}
	return out
}

func sortedMapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func rawOrTyped[T any](raw map[string]map[string]interface{}, typed map[string]*T) interface{} {
	if raw != nil {
		return raw
	}
	return typed
}
