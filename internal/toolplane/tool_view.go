package toolplane

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func ApplyToolView(resp *ToolResponse, view string) {
	if resp == nil {
		return
	}
	view = normalizeToolView(view)
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["view"] = view
	applyToolContextBudget(resp, view)
	if view == "full" || resp.Results == nil {
		return
	}
	// Explicit scene/beats field requests project from the full object: the
	// brief/index summaries strip scenes, so trimming first would make
	// ApplyToolFields return "no requested fields found". Skipping the trim
	// keeps the payload bounded because ApplyToolFields projects only the
	// requested fields.
	if FieldsRequestScenes(resp.Query["fields"]) {
		return
	}
	switch view {
	case "index":
		resp.Results = indexToolResults(resp.Section, resp.Results)
	case "brief":
		resp.Results = briefToolResults(resp.Section, resp.Results)
	}
}

func FieldsRequestScenes(rawFields string) bool {
	for _, field := range parseToolFields(rawFields) {
		if field == "scenes" || strings.HasPrefix(field, "scenes.") {
			return true
		}
	}
	return false
}

func applyToolContextBudget(resp *ToolResponse, view string) {
	if resp == nil || NormalizeKey(resp.Section) != "context" {
		return
	}
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	budget := map[string]interface{}{
		"strategy":           "index_first_then_brief",
		"current_view":       view,
		"avoid":              []string{"full_project_setup", "full_outline", "all_chapters", "source_code_search", "rpg_file_refresh_unless_next_actions_requires_it"},
		"max_extra_queries":  3,
		"prefer_commands_in": []string{"next_actions", "navigation", "workflow"},
	}
	switch view {
	case "index":
		budget["context_level"] = "route_only"
		budget["next_view"] = "brief"
		budget["upgrade_when"] = []string{
			"before composing a non-empty patch",
			"when check/navigation says target facts or excerpts are needed",
			"when current route identifies a specific entity that must be verified",
		}
		budget["stop_when"] = []string{
			"required focused check is clean",
			"next_actions do not request more context",
		}
		if contextIndexHasPatchTask(resp.Results) {
			budget["strategy"] = "patch_task_only"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 0
			budget["prefer_commands_in"] = []string{"patch_task", "next_actions"}
			budget["upgrade_when"] = []string{
				"do not fetch brief context; use patch_task.dry_run_command and patch_task.apply_command directly",
			}
			budget["stop_when"] = []string{
				"patch_task post_patch_check_query has been run",
				"next_actions request post_patch_check or return_final_json",
			}
		} else if contextIndexHasUnpatchableGlobalOnly(resp.Results) {
			budget["strategy"] = "route_only_classify_unpatchable"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 1
			budget["upgrade_when"] = []string{
				"do not fetch brief context unless a later focused check returns a patchable target",
			}
			budget["stop_when"] = []string{
				"issue_context has no patch_query plus patch_shape",
				"next_actions request return_final_json",
			}
		} else if contextIndexHasCleanFocusedCheck(resp.Results) {
			budget["strategy"] = "route_only_clean_focused_check"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 0
			budget["upgrade_when"] = []string{
				"do not fetch brief context when the scoped check returned zero issues",
			}
			budget["stop_when"] = []string{
				"focused check summary total is zero",
				"next_actions request return_final_json",
			}
		} else if contextIndexHasRefreshFirst(resp.Results) {
			budget["strategy"] = "route_only_refresh_derived_first"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 0
			budget["upgrade_when"] = []string{
				"do not fetch brief context before running the refresh and post-refresh check",
			}
			budget["stop_when"] = []string{
				"post-refresh focused check is clean",
				"next_actions request return_final_json",
			}
		}
	case "brief":
		budget["context_level"] = "focused_facts"
		budget["next_view"] = "targeted_query_only"
		budget["upgrade_when"] = []string{
			"only query exact chapter/entity/event IDs named by this bundle",
			"only use full view for one precise small object when brief is insufficient",
		}
		budget["stop_when"] = []string{
			"patch dry-run succeeds and workflow does not allow apply",
			"post-apply focused check is clean",
		}
	case "full":
		budget["context_level"] = "full_facts"
		budget["next_view"] = "none"
		budget["upgrade_when"] = []string{
			"do not broaden further; switch to patch/check or a narrower exact query",
		}
		budget["stop_when"] = []string{
			"target object is understood enough to patch or return final JSON",
		}
	default:
		budget["context_level"] = "focused_facts"
	}
	resp.Meta["context_budget"] = budget
}

func contextIndexHasPatchTask(results interface{}) bool {
	switch typed := results.(type) {
	case OutlineGlobalRepairContext:
		return typed.PatchTask != nil
	case map[string]interface{}:
		task, ok := typed["patch_task"]
		if !ok || task == nil {
			return false
		}
		switch value := task.(type) {
		case *outlinePatchTask:
			return value != nil
		case outlinePatchTask:
			return strings.TrimSpace(value.PatchQuery) != ""
		case map[string]interface{}:
			return strings.TrimSpace(fmt.Sprint(value["patch_query"])) != "" && value["patch_shape"] != nil
		default:
			return true
		}
	default:
		return false
	}
}

func contextIndexHasRefreshFirst(results interface{}) bool {
	switch typed := results.(type) {
	case ChapterRepairContext:
		return toolCheckResultNeedsRefreshFirst(typed.Check)
	case OutlineRepairContext:
		return toolCheckResultNeedsRefreshFirst(typed.Check)
	case RecapRepairContext:
		return toolCheckResultNeedsRefreshFirst(typed.Check)
	case map[string]interface{}:
		actions, ok := typed["next_actions"].([]ToolNextAction)
		if !ok || len(actions) == 0 {
			return false
		}
		return actions[0].Action == "refresh_derived_dsl_first"
	default:
		return false
	}
}

func toolCheckResultNeedsRefreshFirst(check *ToolCheckResult) bool {
	if check == nil || len(check.Issues) == 0 {
		return false
	}
	actions := toolCheckNextActions(*check)
	return len(actions) > 0 && actions[0].Action == "refresh_derived_dsl_first"
}

func contextIndexHasCleanFocusedCheck(results interface{}) bool {
	switch typed := results.(type) {
	case ChapterRepairContext:
		return toolCheckResultIsClean(typed.Check)
	case OutlineRepairContext:
		return toolCheckResultIsClean(typed.Check)
	case RecapRepairContext:
		return toolCheckResultIsClean(typed.Check)
	case map[string]interface{}:
		raw, ok := typed["check"].(map[string]interface{})
		if !ok {
			return false
		}
		return toolCheckIndexMapIsClean(raw)
	default:
		return false
	}
}

func toolCheckResultIsClean(check *ToolCheckResult) bool {
	if check == nil {
		return false
	}
	return check.Summary.Total == 0 && len(check.Issues) == 0
}

func toolCheckIndexMapIsClean(check map[string]interface{}) bool {
	if check == nil {
		return false
	}
	summary, ok := check["summary"].(map[string]interface{})
	if !ok {
		return false
	}
	return numericMapInt(summary, "total") == 0
}

func numericMapInt(values map[string]interface{}, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := strconv.Atoi(value.String())
		return n
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func contextIndexHasUnpatchableGlobalOnly(results interface{}) bool {
	switch typed := results.(type) {
	case OutlineGlobalRepairContext:
		if len(typed.IssueContext) == 0 {
			return false
		}
		return !outlineRepairIssuesHavePatchRoute(typed.IssueContext)
	case map[string]interface{}:
		if _, ok := typed["issue_category"]; !ok {
			return false
		}
		issues, ok := typed["issue_context"]
		if !ok {
			return false
		}
		return !outlineIssueContextHasPatchRoute(issues)
	default:
		return false
	}
}

func outlineIssueContextHasPatchRoute(issues interface{}) bool {
	switch typed := issues.(type) {
	case []map[string]interface{}:
		if len(typed) == 0 {
			return true
		}
		for _, issue := range typed {
			patchQuery, _ := issue["patch_query"].(string)
			if strings.TrimSpace(patchQuery) != "" && issue["patch_shape"] != nil {
				return true
			}
		}
		return false
	case []interface{}:
		if len(typed) == 0 {
			return true
		}
		for _, raw := range typed {
			issue, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			patchQuery, _ := issue["patch_query"].(string)
			if strings.TrimSpace(patchQuery) != "" && issue["patch_shape"] != nil {
				return true
			}
		}
		return false
	case []OutlineRepairIssue:
		return outlineRepairIssuesHavePatchRoute(typed)
	default:
		return true
	}
}

func ApplyToolFields(resp *ToolResponse, rawFields string) {
	if resp == nil || resp.Results == nil {
		return
	}
	fields := parseToolFields(rawFields)
	if len(fields) == 0 {
		return
	}
	projected, ok := projectToolFields(resp.Results, fields)
	if !ok {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("no requested fields found: %s", strings.Join(fields, ",")))
	}
	resp.Results = projected
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["fields"] = fields
}

func parseToolFields(raw string) []string {
	seen := map[string]bool{}
	fields := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		field := normalizeToolFieldPath(item)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		fields = append(fields, field)
	}
	return fields
}

func normalizeToolFieldPath(value string) string {
	parts := strings.Split(strings.TrimSpace(value), ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		key := NormalizeKey(part)
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return strings.Join(out, ".")
}

func projectToolFields(value interface{}, fields []string) (interface{}, bool) {
	data, err := json.Marshal(value)
	if err != nil {
		return value, false
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return value, false
	}
	return projectToolFieldValue(raw, fields)
}

func projectToolFieldValue(value interface{}, fields []string) (interface{}, bool) {
	switch typed := value.(type) {
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		foundAny := false
		for _, item := range typed {
			projected, found := projectToolFieldValue(item, fields)
			foundAny = foundAny || found
			out = append(out, projected)
		}
		return out, foundAny
	case map[string]interface{}:
		return projectToolFieldMap(typed, fields)
	default:
		return value, false
	}
}

func projectToolFieldMap(value map[string]interface{}, fields []string) (map[string]interface{}, bool) {
	out := map[string]interface{}{}
	for _, key := range []string{"type", "id", "title", "summary", "chapter_id", "chapter_title", "path", "reasons", "navigation", "stats"} {
		if v, ok := value[key]; ok {
			out[key] = v
		}
	}

	foundAny := false
	for _, field := range fields {
		if v, ok := getToolFieldPath(value, field); ok {
			out[toolFieldOutputKey(field)] = v
			foundAny = true
			continue
		}
		for _, nested := range []string{"object", "outline", "event", "target_volume", "previous_volume", "next_volume"} {
			child, ok := value[nested].(map[string]interface{})
			if !ok {
				continue
			}
			if v, ok := getToolFieldPath(child, field); ok {
				out[toolFieldOutputKey(field)] = v
				foundAny = true
				break
			}
		}
	}
	return out, foundAny
}

func getToolFieldPath(value map[string]interface{}, path string) (interface{}, bool) {
	current := interface{}(value)
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		next, ok := m[part]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func toolFieldOutputKey(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func normalizeToolView(view string) string {
	switch NormalizeKey(view) {
	case "", "brief", "summary":
		return "brief"
	case "full", "raw":
		return "full"
	case "index", "list":
		return "index"
	default:
		return "brief"
	}
}
