// The tool patch entry point: argument dispatch, patch-task resolution, patch-bytes validation and
// the generic JSON merge/diff helpers. Project files are written only behind apply.

package mutation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

type ToolPatchResult struct {
	OK          bool                   `json:"ok"`
	Applied     bool                   `json:"applied"`
	DryRun      bool                   `json:"dry_run"`
	Target      string                 `json:"target"`
	ID          string                 `json:"id"`
	Changed     []ToolPatchChange      `json:"changed,omitempty"`
	Check       *ToolCheckResult       `json:"check,omitempty"`
	Files       map[string]string      `json:"files,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	NextActions []ToolNextAction       `json:"next_actions,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type ToolPatchChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

func ValidateToolPatchBytes(patchBytes []byte) error {
	data := utils.StripUTF8BOM(patchBytes)
	if len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("patch JSON is required")
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	object, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("patch must be a JSON object with at least one field")
	}
	if len(object) == 0 {
		return fmt.Errorf("patch must contain at least one field")
	}
	return utils.ValidateNoSuspiciousPatchText(value)
}

// decodeStrictToolPatch decodes one patch object and rejects fields the target
// type does not declare. The tool plane used to merge patches through a
// map[string]interface{}, where a typo such as "sumary" was silently dropped
// and the command still reported success.
func decodeStrictToolPatch(data []byte, out interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			value := reflect.ValueOf(out)
			if value.Kind() == reflect.Ptr && !value.IsNil() {
				allowed := structJSONFields(value.Elem().Interface())
				if len(allowed) > 0 {
					names := make([]string, 0, len(allowed))
					for name := range allowed {
						names = append(names, name)
					}
					sort.Strings(names)
					return fmt.Errorf("%w; valid fields: %s", err, strings.Join(names, ", "))
				}
			}
		}
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("patch must contain exactly one JSON object")
		}
		return err
	}
	return nil
}

// structJSONFields returns the JSON field names a struct accepts. It is used for
// setup patches, whose top-level object also accepts two virtual array-edit
// fields that are consumed by the tool plane and never stored.
func structJSONFields(value interface{}) map[string]bool {
	fields := map[string]bool{}
	collectStructJSONFields(reflect.TypeOf(value), fields)
	return fields
}

func collectStructJSONFields(t reflect.Type, out map[string]bool) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			collectStructJSONFields(field.Type, out)
			continue
		}
		if field.PkgPath != "" {
			continue
		}
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "-" {
			continue
		}
		if tag == "" {
			tag = field.Name
		}
		out[tag] = true
	}
}

func unknownPatchFieldsError(kind string, provided map[string]json.RawMessage, allowed map[string]bool) error {
	var unknown []string
	for key := range provided {
		if !allowed[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	valid := make([]string, 0, len(allowed))
	for key := range allowed {
		valid = append(valid, key)
	}
	sort.Strings(valid)
	return fmt.Errorf("%s patch has unknown field(s): %s; valid fields: %s",
		kind, strings.Join(unknown, ", "), strings.Join(valid, ", "))
}

// appendNoChangeWarning makes a dry-run/apply that changed nothing explicit.
// Agents should not have to infer success from an empty `changed` array.
func appendNoChangeWarning(warnings []string, changes []ToolPatchChange) []string {
	if len(changes) == 0 {
		warnings = append(warnings, "patch produced no changes; check field names and values against the target shape")
	}
	return warnings
}

func RunToolPatchValidationCheck(setup *models.StorySetup, outline *models.Outline, scope, id string) (*ToolCheckResult, error) {
	return RunToolPatchValidationCheckAt(nil, setup, outline, scope, id)
}

// RunToolPatchValidationCheckAt validates a patch against an explicit project
// root so outline simulation uses the same craft elements as the project.
func RunToolPatchValidationCheckAt(root *store.Root, setup *models.StorySetup, outline *models.Outline, scope, id string) (*ToolCheckResult, error) {
	return RunToolOutlineCheckAt(root, "all", setup, outline, scope, id)
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
