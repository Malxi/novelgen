// Craft patches: target/key resolution and the typed normalization of a single craft element.

package mutation

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/contract/toolcmd"
	"path/filepath"
	"sort"
	"strings"

	"novelgen/internal/fiction/domain/craft"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/platform/store"
)

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
	return craft.NormalizeTarget(target)
}

func craftPatchFilename(kind string) (string, error) {
	return craft.Filename(kind)
}

func resolveCraftPatchKey(values map[string]map[string]interface{}, id string) (string, bool) {
	return craft.ResolveKey(values, id)
}

func NormalizeCraftPatchObject(kind, key string, raw map[string]interface{}) (map[string]interface{}, error) {
	return craft.NormalizePatchObject(kind, key, raw)
}

func copyStringInterfaceMap(value map[string]interface{}) map[string]interface{} {
	return craft.CopyMap(value)
}
func cloneRawCraftMap(values map[string]map[string]interface{}) map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = copyStringInterfaceMap(value)
	}
	return out
}
