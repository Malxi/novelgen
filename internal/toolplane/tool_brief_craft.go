// Craft briefs: one clipped object per craft element and the collection index used to find it.

package toolplane

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/contract/toolcmd"
	"sort"
)

func briefCraftResults(results interface{}, indexOnly bool) interface{} {
	switch typed := results.(type) {
	case []map[string]interface{}:
		out := make([]map[string]interface{}, 0, len(typed))
		for _, hit := range typed {
			if indexOnly {
				out = append(out, indexCraftHit(hit))
			} else {
				out = append(out, briefCraftHit(hit))
			}
		}
		return out
	case map[string]interface{}:
		if indexOnly {
			out := map[string]interface{}{}
			for section, value := range typed {
				out[section] = craftCollectionIndex(section, value)
			}
			return out
		}
		out := map[string]interface{}{}
		for section, value := range typed {
			out[section] = briefCraftCollection(value)
		}
		return out
	default:
		return results
	}
}

func briefCraftCollection(value interface{}) interface{} {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var raw map[string]map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return value
	}
	out := make(map[string]interface{}, len(raw))
	for key, object := range raw {
		out[key] = compactCraftObject(object)
	}
	return out
}

func briefCraftHit(hit map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, key := range []string{"type", "key"} {
		if value, ok := hit[key]; ok {
			out[key] = value
		}
	}
	if raw, ok := hit["object"].(map[string]interface{}); ok {
		out["object"] = compactCraftObject(raw)
	} else if object, ok := hit["object"]; ok {
		out["object"] = object
	}
	return out
}

func indexCraftHit(hit map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	key := fmt.Sprint(hit["key"])
	kind := normalizeCraftType(fmt.Sprint(hit["type"]))
	if key != "" {
		out["key"] = key
	}
	if kind != "" {
		out["type"] = kind
	}
	name := key
	if raw, ok := hit["object"].(map[string]interface{}); ok {
		if objectName := StringMapValue(raw, "name"); objectName != "" {
			name = objectName
			out["name"] = objectName
		}
		for _, field := range []string{"role_in_story", "rpg_role", "combat_role", "type", "category"} {
			if value, ok := raw[field]; ok {
				if text, ok := value.(string); ok {
					out[field] = clipToolString(text, 120)
				} else {
					out[field] = value
				}
			}
		}
	}
	if kind != "" && name != "" {
		out["detail_query"] = toolcmd.QueryCraft(toolcmd.QueryOptions{Type: kind, Name: name, View: "brief"})
	}
	return out
}

func craftCollectionIndex(section string, value interface{}) interface{} {
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]interface{}{"count": 0}
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]interface{}{"count": 0}
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return map[string]interface{}{
		"count":        len(keys),
		"keys":         keys,
		"detail_query": fmt.Sprintf("novelgen tool query craft --type %s --name <name> --view brief", craftCollectionType(section)),
	}
}

func craftCollectionType(section string) string {
	switch NormalizeKey(section) {
	case "characters":
		return "character"
	case "items":
		return "item"
	case "locations":
		return "location"
	case "organizations", "orgs":
		return "organization"
	default:
		return NormalizeKey(section)
	}
}
