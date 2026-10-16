// Structural diffing used to report what a patch changed: typed struct fields and raw JSON maps.

package toolplane

import (
	"encoding/json"
	"reflect"
)

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
