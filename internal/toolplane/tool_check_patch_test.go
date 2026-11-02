package toolplane_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelgen/cmd/writecmd"
	models "novelgen/internal/contract/story"
	"novelgen/internal/contract/testfixtures"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/quality"
	writepipe "novelgen/internal/fiction/pipeline"
	rpgdsl "novelgen/internal/fiction/rpg/dsl"
	"novelgen/internal/toolplane"
)

func decodeToolPatchResult(t *testing.T, raw string) toolplane.ToolPatchResult {
	t.Helper()
	var result toolplane.ToolPatchResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode patch result: %v\n%s", err, raw)
	}
	return result
}

func toolCheckHasIssue(result *toolplane.ToolCheckResult, text string) bool {
	if result == nil {
		return false
	}
	text = strings.ToLower(text)
	for _, issue := range result.Issues {
		if strings.Contains(strings.ToLower(issue.Issue), text) ||
			strings.Contains(strings.ToLower(issue.Suggestion), text) {
			return true
		}
	}
	return false
}

func writeTestJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertToolCheckIssue(t *testing.T, result *toolplane.ToolCheckResult, category, targetID, issueContains string) {
	t.Helper()
	if result == nil {
		t.Fatal("check result is nil")
	}
	for _, issue := range result.Issues {
		if toolplane.NormalizeKey(issue.Category) == toolplane.NormalizeKey(category) &&
			strings.TrimSpace(issue.TargetID) == targetID &&
			strings.Contains(issue.Issue, issueContains) {
			return
		}
	}
	t.Fatalf("missing issue category=%q target=%q contains=%q in %#v", category, targetID, issueContains, result.Issues)
}

func TestToolOutlineCheckQualityUsesScopedTarget(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID:      "P1",
		Title:   "Part One",
		Summary: "Part arc",
		Volumes: []models.Volume{{
			ID:      "P1-V1",
			Title:   "Volume One",
			Summary: "Volume arc",
			Chapters: []models.Chapter{{
				ID:    "P1-V1-C1",
				Title: "Broken",
			}},
		}},
	}}}

	result, err := toolplane.RunToolOutlineCheck("quality", nil, outline, "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "quality" || result.Scope != "chapter" || result.ID != "P1-V1-C1" {
		t.Fatalf("unexpected check identity: %#v", result)
	}
	if result.OK || !result.Blocking {
		t.Fatalf("quality check should block for missing required chapter fields: %#v", result.Summary)
	}
	if result.Summary.High == 0 {
		t.Fatalf("expected high-priority contract issues: %#v", result.Summary)
	}
}

func TestRunToolOutlineCheckAllMergesQualityAndSimulation(t *testing.T) {
	outline := validToolPatchTestOutline()
	result, err := toolplane.RunToolOutlineCheck("all", nil, outline, "volume", "P1-V1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "all" {
		t.Fatalf("kind = %q, want all", result.Kind)
	}
	if result.Summary.Total == 0 {
		t.Fatalf("expected at least simulation/quality feedback for sparse test outline")
	}
}

func TestRunToolOutlineCheckChapterScopeKeepsVolumeContext(t *testing.T) {
	chapter1 := testfixtures.ValidChapterForQualityGate()
	chapter1.ID = "P1-V1-C1"
	chapter1.Location = "Outer Gate"
	chapter1.StateAnchor.Location = "Outer Gate"
	chapter2 := testfixtures.ValidChapterForQualityGate()
	chapter2.ID = "P1-V1-C2"
	chapter2.Location = "Inner Hall"
	chapter2.StateAnchor.Location = "Inner Hall"
	chapter2.Timeline = models.ChapterTimeline{Anchor: "Day 2"}
	outline := &models.Outline{Parts: []models.Part{{
		ID:      "P1",
		Title:   "Part One",
		Summary: "Part arc",
		Volumes: []models.Volume{{
			ID:       "P1-V1",
			Title:    "Volume One",
			Summary:  "Volume arc",
			Chapters: []models.Chapter{chapter1, chapter2},
		}},
	}}}

	result, err := toolplane.RunToolOutlineCheck("all", nil, outline, "chapter", "P1-V1-C2")
	if err != nil {
		t.Fatal(err)
	}
	assertToolCheckIssue(t, result, "transition", "P1-V1-C2", "缺少过渡描述")
}

func TestToolCraftSchemaCheckFlagsUnknownItemOwner(t *testing.T) {
	root := t.TempDir()
	craftDir := filepath.Join(root, "story", "craft")
	if err := os.MkdirAll(craftDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(craftDir, "characters.json"), map[string]interface{}{
		"林野": map[string]interface{}{
			"name":       "林野",
			"role":       "主角",
			"background": "测试角色",
		},
	})
	writeTestJSON(t, filepath.Join(craftDir, "items.json"), map[string]interface{}{
		"survival_kit": map[string]interface{}{
			"name":         "求生装备",
			"type":         "工具套装",
			"description":  "基础生存工具",
			"appearance":   "磨损背包",
			"function":     "提供基础生存支持",
			"significance": "初期物资",
			"owner":        "陈烽",
		},
	})

	result, err := toolplane.RunToolCraftSchemaCheck(root, "item", "survival_kit")
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocking {
		t.Fatalf("unknown owner should be non-blocking consistency feedback: %#v", result)
	}
	if result.Summary.Medium != 1 || len(result.Issues) != 1 {
		t.Fatalf("summary/issues = %#v %#v, want one medium issue", result.Summary, result.Issues)
	}
	if !strings.Contains(result.Issues[0].Issue, "陈烽") || result.Issues[0].Category != "consistency" {
		t.Fatalf("unexpected issue: %#v", result.Issues[0])
	}
}

func TestToolScopedSuggestionsExcludeGlobalIssues(t *testing.T) {
	outline := validToolPatchTestOutline()
	suggestions := []models.ReviewSuggestion{
		{Category: "character", TargetID: "global", Issue: "global issue", Priority: models.PriorityMedium},
		{Category: "character", TargetID: "core_cast", Issue: "setup issue", Priority: models.PriorityMedium},
		{Category: "appeal", TargetID: "P1-V1-C1", Issue: "chapter issue", Priority: models.PriorityMedium},
		{Category: "appeal", TargetID: "P1-V2-C1", Issue: "other chapter issue", Priority: models.PriorityMedium},
	}

	filtered := toolplane.FilterToolScopedSuggestions(suggestions, outline, "volume", "P1-V1")
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1: %#v", len(filtered), filtered)
	}
	if filtered[0].TargetID != "P1-V1-C1" {
		t.Fatalf("kept target = %q, want P1-V1-C1", filtered[0].TargetID)
	}
}

func TestToolScopedChapterSuggestionsExcludeSiblingIssues(t *testing.T) {
	outline := validToolPatchTestOutline()
	suggestions := []models.ReviewSuggestion{
		{Category: "appeal", TargetID: "P1-V1-C1", Issue: "target chapter issue", Priority: models.PriorityMedium},
		{Category: "appeal", TargetID: "P1-V1-C2", Issue: "sibling chapter issue", Priority: models.PriorityMedium},
		{Category: "plot", TargetID: "global", Issue: "global issue", Priority: models.PriorityMedium},
	}

	filtered := toolplane.FilterToolScopedSuggestions(suggestions, outline, "chapter", "P1-V1-C1")
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1: %#v", len(filtered), filtered)
	}
	if filtered[0].TargetID != "P1-V1-C1" {
		t.Fatalf("kept target = %q, want P1-V1-C1", filtered[0].TargetID)
	}
}

func TestApplyToolCheckIssueFiltersKeepsSummaryAndLimitsReturnedIssues(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Summary: toolplane.ToolCheckSummary{Total: 4, Critical: 1, High: 1, Medium: 1, Low: 1},
		Issues: []models.ReviewSuggestion{
			{TargetID: "low", Priority: models.PriorityLow},
			{TargetID: "medium", Priority: models.PriorityMedium},
			{TargetID: "high", Priority: models.PriorityHigh},
			{TargetID: "critical", Priority: models.PriorityCritical},
		},
		Meta: map[string]interface{}{"project_root": "root"},
	}

	if err := toolplane.ApplyToolCheckIssueFilters(result, "medium", "", 2); err != nil {
		t.Fatal(err)
	}
	if result.Summary.Total != 4 || len(result.Issues) != 2 {
		t.Fatalf("summary/issues = %#v %#v", result.Summary, result.Issues)
	}
	if result.Issues[0].TargetID != "medium" || result.Issues[1].TargetID != "high" {
		t.Fatalf("unexpected filtered issues: %#v", result.Issues)
	}
	filter, ok := result.Meta["issue_filter"].(map[string]interface{})
	if !ok || filter["original_issues"] != 4 || filter["returned_issues"] != 2 {
		t.Fatalf("missing filter meta: %#v", result.Meta)
	}
}

func TestApplyToolCheckIssueFiltersFiltersByCategory(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Summary: toolplane.ToolCheckSummary{Total: 4, Critical: 1, High: 1, Medium: 1, Low: 1},
		Issues: []models.ReviewSuggestion{
			{TargetID: "logic-low", Category: "logic", Priority: models.PriorityLow},
			{TargetID: "plot-medium", Category: "plot", Priority: models.PriorityMedium},
			{TargetID: "logic-high", Category: "logic", Priority: models.PriorityHigh},
			{TargetID: "structure-critical", Category: "structure", Priority: models.PriorityCritical},
		},
		Meta: map[string]interface{}{},
	}

	if err := toolplane.ApplyToolCheckIssueFilters(result, "medium", " logic,structure ", 0); err != nil {
		t.Fatal(err)
	}
	if result.Summary.Total != 4 || len(result.Issues) != 2 {
		t.Fatalf("summary/issues = %#v %#v", result.Summary, result.Issues)
	}
	if result.Issues[0].TargetID != "logic-high" || result.Issues[1].TargetID != "structure-critical" {
		t.Fatalf("unexpected filtered issues: %#v", result.Issues)
	}
	filter, ok := result.Meta["issue_filter"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing filter meta: %#v", result.Meta)
	}
	categories, ok := filter["category"].([]string)
	if !ok || len(categories) != 2 || categories[0] != "logic" || categories[1] != "structure" {
		t.Fatalf("unexpected category filter meta: %#v", filter["category"])
	}
}

func TestToolCheckResultJSONAddsIssueNavigation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "volume",
		ID:     "P1-V1",
		Issues: []models.ReviewSuggestion{{
			Category: "logic",
			TargetID: "P1-V1-C3",
			Issue:    "章节开场可能缺少与上一章的过渡",
			Priority: models.PriorityLow,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			TargetID   string                 `json:"target_id"`
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
		Meta        map[string]interface{}     `json:"meta"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Issues) != 1 || decoded.Issues[0].TargetID != "P1-V1-C3" {
		t.Fatalf("unexpected issues JSON: %s", string(data))
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "chapter" || nav["volume_id"] != "P1-V1" {
		t.Fatalf("unexpected navigation: %#v", nav)
	}
	detail, ok := nav["detail_queries"].([]interface{})
	if !ok || len(detail) != 2 || !strings.Contains(detail[0].(string), "P1-V1-C3") {
		t.Fatalf("missing detail queries: %#v", nav["detail_queries"])
	}
	if !strings.Contains(nav["focused_check_query"].(string), "--category logic") {
		t.Fatalf("missing focused check query: %#v", nav["focused_check_query"])
	}
	if !strings.Contains(nav["repair_route_query"].(string), `outline-repair --id 'P1-V1-C3' --name 'logic' --view index`) {
		t.Fatalf("missing repair route query: %#v", nav["repair_route_query"])
	}
	if !strings.Contains(nav["repair_context_query"].(string), `outline-repair --id 'P1-V1-C3' --name 'logic'`) {
		t.Fatalf("missing repair context query: %#v", nav["repair_context_query"])
	}
	if !strings.Contains(nav["patch_query"].(string), "--target volume") {
		t.Fatalf("missing patch query: %#v", nav["patch_query"])
	}
	patchShape, ok := nav["patch_shape"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing patch shape: %#v", nav["patch_shape"])
	}
	changedChapters, ok := patchShape["changed_chapters"].([]interface{})
	if !ok || len(changedChapters) != 1 {
		t.Fatalf("missing changed chapter patch shape: %#v", patchShape)
	}
	chapterPatch, ok := changedChapters[0].(map[string]interface{})
	if !ok || chapterPatch["id"] != "P1-V1-C3" {
		t.Fatalf("unexpected chapter patch shape: %#v", changedChapters[0])
	}
	if openingBeat, ok := chapterPatch["opening_beat"].(string); !ok || !strings.Contains(openingBeat, "来到/前往/到达") {
		t.Fatalf("opening beat repair hint missing: %#v", chapterPatch)
	}
	if len(decoded.NextActions) < 4 ||
		decoded.NextActions[0].Action != "repair_first_patchable_issue" ||
		decoded.NextActions[1].Action != "query_repair_route" ||
		!strings.Contains(decoded.NextActions[1].Command, `outline-repair --id 'P1-V1-C3' --name 'logic' --view index`) {
		t.Fatalf("unexpected check next_actions: %#v", decoded.NextActions)
	}
	budget, ok := decoded.Meta["check_budget"].(map[string]interface{})
	if !ok || budget["strategy"] != "patchable_issue_navigation_first" {
		t.Fatalf("missing check_budget: %#v", decoded.Meta)
	}
	avoid, ok := budget["avoid"].([]interface{})
	if !ok || !toolTestInterfaceSliceContains(avoid, "full_outline") || !toolTestInterfaceSliceContains(avoid, "source_code_search") {
		t.Fatalf("unexpected check_budget avoid: %#v", budget["avoid"])
	}
}

func TestToolCheckResultJSONCleanCheckTellsAgentToStop(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "volume",
		ID:     "P1-V1",
		OK:     true,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
		Meta        map[string]interface{}     `json:"meta"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.NextActions) != 1 || decoded.NextActions[0].Action != "return_final_json" {
		t.Fatalf("clean check should tell agent to stop: %#v", decoded.NextActions)
	}
	budget, ok := decoded.Meta["check_budget"].(map[string]interface{})
	if !ok || budget["strategy"] != "patchable_issue_navigation_first" || budget["current_issue_count"].(float64) != 0 {
		t.Fatalf("missing clean check budget: %#v", decoded.Meta)
	}
}

func TestToolCheckResultJSONRoutesGlobalOutlineIssueConservatively(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category: "mysteries",
			Issue:    "unresolved mysteries",
			Priority: models.PriorityLow,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Issues) != 1 {
		t.Fatalf("unexpected issues JSON: %s", string(data))
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "outline.global" {
		t.Fatalf("unexpected global navigation: %#v", nav)
	}
	if nav["repair_route_query"] != `novelgen tool query context --type outline-global-repair --name 'mysteries' --view index` {
		t.Fatalf("global outline route should stay index-sized: %#v", nav)
	}
	if _, ok := nav["repair_context_query"]; ok {
		t.Fatalf("unpatchable global outline route should not request brief context: %#v", nav)
	}
	if !strings.Contains(nav["focused_check_query"].(string), "tool check all --target outline --category mysteries") {
		t.Fatalf("missing global focused check: %#v", nav["focused_check_query"])
	}
	if len(decoded.NextActions) < 4 ||
		decoded.NextActions[1].Action != "query_repair_route" ||
		decoded.NextActions[1].Command != `novelgen tool query context --type outline-global-repair --name 'mysteries' --view index` ||
		decoded.NextActions[2].Action != "focused_recheck" ||
		decoded.NextActions[3].Action != "return_final_json" {
		t.Fatalf("unexpected global next_actions: %#v", decoded.NextActions)
	}
	for _, action := range decoded.NextActions {
		if action.Action == "query_repair_context_if_needed" {
			t.Fatalf("unpatchable global next_actions should not fetch brief context: %#v", decoded.NextActions)
		}
	}
}

func TestToolCheckResultJSONPrefersPatchableIssueForNextActions(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category: "mysteries",
			Issue:    "unresolved mysteries",
			Priority: models.PriorityLow,
		}, {
			Category: "faction_tier",
			TargetID: "zerg",
			Issue:    "tier ladder is scattered",
			Priority: models.PriorityLow,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.NextActions) < 5 {
		t.Fatalf("expected patchable next_actions, got %#v", decoded.NextActions)
	}
	if decoded.NextActions[0].Action != "repair_first_patchable_issue" ||
		!strings.Contains(decoded.NextActions[0].Purpose, "issue #2") ||
		decoded.NextActions[0].When != "Selected category: faction_tier" {
		t.Fatalf("first action should identify selected patchable issue: %#v", decoded.NextActions[0])
	}
	if !strings.Contains(decoded.NextActions[1].Command, `story-setup --type search --name 'zerg' --view index`) ||
		decoded.NextActions[3].Command != "novelgen tool patch setup" ||
		!strings.Contains(decoded.NextActions[4].Command, "tool check all --target outline --category faction_tier") {
		t.Fatalf("next_actions should route to setup patch: %#v", decoded.NextActions)
	}
}

func TestToolIssueNavigationRoutesVolumeRedundancyToChangedEvents(t *testing.T) {
	nav := toolplane.ToolIssueNavigation("all", "outline", "volume", "P1-V1", models.ReviewSuggestion{
		Category: "redundancy",
		TargetID: "P1-V1",
		Issue:    "event type repeated",
		Priority: models.PriorityMedium,
	}, 0)
	if got := fmt.Sprint(nav["patch_query"]); got != `novelgen tool patch outline --target volume --id 'P1-V1'` {
		t.Fatalf("patch query = %q", got)
	}
	detailQueries, ok := nav["detail_queries"].([]string)
	if !ok {
		t.Fatalf("detail_queries type = %T", nav["detail_queries"])
	}
	if !containsString(detailQueries, `novelgen tool query outline --type events --volume-id 'P1-V1' --fields event_index,chapter_id,type,action,actor,target,target_type,details,result --view brief`) {
		t.Fatalf("detail_queries missing volume events query: %#v", detailQueries)
	}
	shape, ok := nav["patch_shape"].(map[string]interface{})
	if !ok {
		t.Fatalf("patch_shape type = %T", nav["patch_shape"])
	}
	if _, ok := shape["changed_events"]; !ok {
		t.Fatalf("patch_shape should suggest changed_events: %#v", shape)
	}
}

func TestToolCheckResultJSONDoesNotCallUnpatchableGlobalIssuePatchable(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category: "mysteries",
			Issue:    "unresolved mysteries",
			Priority: models.PriorityLow,
		}, {
			Category: "structure",
			TargetID: "global",
			Issue:    "protagonist background is thin",
			Priority: models.PriorityMedium,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.NextActions) == 0 {
		t.Fatalf("expected route next_actions, got none")
	}
	if decoded.NextActions[0].Action != "inspect_priority_issue_route" ||
		!strings.Contains(decoded.NextActions[0].Purpose, "issue #2") ||
		decoded.NextActions[0].When != "Selected category: structure" {
		t.Fatalf("unpatchable routed issue should not be called patchable: %#v", decoded.NextActions[0])
	}
	for _, action := range decoded.NextActions {
		if action.Action == "patch_dry_run" {
			t.Fatalf("unpatchable global issue should not request patch dry-run: %#v", decoded.NextActions)
		}
	}
}

func TestToolCheckResultJSONPrefersHigherPriorityPatchableIssue(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category: "faction_tier",
			TargetID: "zerg",
			Issue:    "low priority setup patch",
			Priority: models.PriorityLow,
		}, {
			Category: "logic",
			TargetID: "P1-V1-C3",
			Issue:    "medium priority chapter patch",
			Priority: models.PriorityMedium,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.NextActions) < 4 {
		t.Fatalf("expected patchable next_actions, got %#v", decoded.NextActions)
	}
	if decoded.NextActions[0].Action != "repair_first_patchable_issue" ||
		!strings.Contains(decoded.NextActions[0].Purpose, "issue #2") ||
		decoded.NextActions[0].When != "Selected category: logic" {
		t.Fatalf("first action should identify higher-priority patchable issue: %#v", decoded.NextActions[0])
	}
	if !strings.Contains(decoded.NextActions[1].Command, `outline-repair --id 'P1-V1-C3' --name 'logic' --view index`) ||
		!strings.Contains(decoded.NextActions[3].Command, `tool patch outline --target volume --id 'P1-V1'`) {
		t.Fatalf("next_actions should route to the higher-priority chapter patch: %#v", decoded.NextActions)
	}
}

func toolTestInterfaceSliceContains(values []interface{}, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestToolCheckResultJSONRoutesVolumeIssueThroughContextNavigation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "volume",
		ID:     "P1-V1",
		Issues: []models.ReviewSuggestion{{
			Category: "logic",
			TargetID: "P1-V1",
			Issue:    "needs volume context",
			Priority: models.PriorityMedium,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `tool query context --type outline-volume`) {
		t.Fatalf("volume issue navigation should use context query: %s", text)
	}
	if strings.Contains(text, `tool query outline --type volume`) {
		t.Fatalf("volume issue navigation should not use legacy volume query: %s", text)
	}
}

func TestToolCheckResultJSONRoutesFactionTierIssueThroughSetupNavigation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category:   "faction_tier",
			TargetID:   "zerg",
			TargetName: "zerg",
			Issue:      "tier ladder is scattered",
			Priority:   models.PriorityLow,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Issues) != 1 {
		t.Fatalf("unexpected issues JSON: %s", string(data))
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "setup.faction_tier" {
		t.Fatalf("unexpected faction tier navigation: %#v", nav)
	}
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type search --name 'zerg' --view index`) ||
		!strings.Contains(nav["repair_context_query"].(string), `story-setup --type search --name 'zerg' --view brief`) {
		t.Fatalf("faction tier should route through setup search: %#v", nav)
	}
	if nav["patch_query"] != "novelgen tool patch setup" {
		t.Fatalf("faction tier should patch setup: %#v", nav)
	}
	shape, ok := nav["patch_shape"].(map[string]interface{})
	if !ok {
		t.Fatalf("faction tier should include patch_shape: %#v", nav)
	}
	premises, ok := shape["premises"].([]interface{})
	if !ok || len(premises) != 1 {
		t.Fatalf("patch_shape should include one premise hint: %#v", shape)
	}
	premise, ok := premises[0].(map[string]interface{})
	if !ok || premise["category"] != "zerg" {
		t.Fatalf("patch_shape should name target faction category: %#v", shape)
	}
	if !strings.Contains(nav["focused_check_query"].(string), "tool check all --target outline --category faction_tier") {
		t.Fatalf("missing faction tier focused check: %#v", nav["focused_check_query"])
	}
	if len(decoded.NextActions) < 4 ||
		decoded.NextActions[1].Command != nav["repair_route_query"] ||
		decoded.NextActions[3].Command != nav["patch_query"] {
		t.Fatalf("next_actions should follow setup navigation: %#v", decoded.NextActions)
	}
}

func TestToolCheckCraftSchemaJSONAddsNavigation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "schema",
		Target: "craft",
		Scope:  "character",
		ID:     "林野",
		Issues: []models.ReviewSuggestion{{
			Category: "schema",
			TargetID: "林野",
			Issue:    "bad schema",
			Priority: models.PriorityHigh,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "tool query craft") ||
		!strings.Contains(text, "tool query context --type craft-character") ||
		!strings.Contains(text, `repair_route_query`) ||
		!strings.Contains(text, `craft-character --name '林野' --view index`) ||
		!strings.Contains(text, `repair_context_query`) ||
		!strings.Contains(text, `craft-character --name '林野' --view brief`) ||
		!strings.Contains(string(data), "tool patch craft") ||
		!strings.Contains(string(data), "tool check schema") {
		t.Fatalf("craft navigation missing: %s", text)
	}
}

func TestToolCheckRecapJSONAddsNavigation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "quality",
		Target: "recap",
		Scope:  "chapter",
		ID:     "P1-V1-C1",
		Issues: []models.ReviewSuggestion{{
			Category: "recap",
			TargetID: "P1-V1-C1",
			Issue:    "last_line 为空",
			Priority: models.PriorityHigh,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "tool check quality --target recap") ||
		!strings.Contains(text, "tool query context --type recap-repair") ||
		!strings.Contains(text, `repair_route_query`) ||
		!strings.Contains(text, `recap-repair --id 'P1-V1-C1' --view index`) ||
		!strings.Contains(text, `repair_context_query`) ||
		!strings.Contains(text, `recap-repair --id 'P1-V1-C1' --view brief`) ||
		!strings.Contains(text, "novelgen recap gen --chapter") {
		t.Fatalf("recap navigation missing: %s", text)
	}
}

func TestToolCheckSetupJSONAddsRouteNavigation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "setup",
		Scope:  "all",
		ID:     "",
		Issues: []models.ReviewSuggestion{{
			Category: "structure",
			TargetID: "premise",
			Issue:    "premise is too vague",
			Priority: models.PriorityHigh,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`tool query story-setup --view index`,
		`tool query story-setup --view brief`,
		`story-setup --type search --name 'premise' --view index`,
		`repair_route_query`,
		`repair_context_query`,
		`tool patch setup`,
		`patch_shape`,
		`post_patch_check_query`,
		`tool check all --target setup --category structure`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("setup navigation missing %q: %s", want, text)
		}
	}
}

func TestToolCheckSetupJSONRoutesIndexedStorylineByTargetName(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "setup",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category:   "plot",
			TargetID:   "storylines[0]",
			TargetName: "Signal War",
			Issue:      "important long-form storyline lacks repeatable pressure",
			Priority:   models.PriorityMedium,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "setup.storyline" {
		t.Fatalf("unexpected indexed storyline navigation: %#v", nav)
	}
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type storyline --name 'Signal War' --view index`) ||
		strings.Contains(nav["repair_route_query"].(string), "storylines[0]") {
		t.Fatalf("storyline route should use target_name, not index: %#v", nav)
	}
	shape, ok := nav["patch_shape"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing patch shape: %#v", nav["patch_shape"])
	}
	storylines, ok := shape["storylines"].([]interface{})
	if !ok || len(storylines) != 1 {
		t.Fatalf("missing storyline upsert shape: %#v", shape)
	}
	storyline := storylines[0].(map[string]interface{})
	if storyline["name"] != "Signal War" || storyline["repeatable_pressure"] == nil {
		t.Fatalf("unexpected storyline patch shape: %#v", storyline)
	}
	if len(decoded.NextActions) < 4 ||
		decoded.NextActions[1].Command != nav["repair_route_query"] ||
		decoded.NextActions[3].Command != "novelgen tool patch setup" {
		t.Fatalf("next_actions should follow indexed setup navigation: %#v", decoded.NextActions)
	}
}

func TestToolCheckOutlineGlobalIssueRoutesThroughGlobalRepairContext(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "outline",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category: "structure",
			TargetID: "global",
			Issue:    "global structure issue",
			Priority: models.PriorityMedium,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`context --type outline-global-repair --name 'structure' --view index`,
		`repair_route_query`,
		`classification_rule`,
		`return_final_json`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("global outline navigation missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{
		`context --type outline-global-repair --name 'structure' --view brief`,
		`repair_context_query`,
		`tool query outline --view index`,
		`tool query story-setup --view index`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("global outline navigation should not use broad query %q: %s", forbidden, text)
		}
	}
}

func TestToolCheckSetupJSONRoutesIndexedPremiseByTargetName(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "setup",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category:   "appeal",
			TargetID:   "premises[6]",
			TargetName: "虫族阶层与天敌体系",
			Issue:      "core premise lacks appeal_engine",
			Priority:   models.PriorityMedium,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "setup.premise" {
		t.Fatalf("unexpected indexed premise navigation: %#v", nav)
	}
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type premise --name '虫族阶层与天敌体系' --view index`) ||
		strings.Contains(nav["repair_route_query"].(string), "premises[6]") {
		t.Fatalf("premise route should use target_name, not index: %#v", nav)
	}
	shape := nav["patch_shape"].(map[string]interface{})
	premises := shape["premises"].([]interface{})
	premise := premises[0].(map[string]interface{})
	if premise["name"] != "虫族阶层与天敌体系" || premise["appeal_engine"] == nil {
		t.Fatalf("unexpected premise patch shape: %#v", premise)
	}
}

func TestToolCheckSetupJSONRoutesIndexedRuleToPatchField(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "setup",
		Scope:  "all",
		Issues: []models.ReviewSuggestion{{
			Category:   "setup",
			TargetID:   "rules[4]",
			TargetName: "rules[4]",
			Issue:      "setup field is too long for stable prompting",
			Priority:   models.PriorityLow,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "setup.rule" {
		t.Fatalf("unexpected indexed rule navigation: %#v", nav)
	}
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type search --name 'rules[4]' --view index`) {
		t.Fatalf("rule route should stay focused on target id: %#v", nav)
	}
	shape := nav["patch_shape"].(map[string]interface{})
	items := shape["rules_patch"].([]interface{})
	item := items[0].(map[string]interface{})
	if item["index"].(float64) != 4 || item["value"] != "<shortened rule text>" {
		t.Fatalf("unexpected rules_patch shape: %#v", item)
	}
}

func TestToolChapterQualityCheckRejectsRawAgentJSON(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	content := "# Opening\n\n{\"content\":\"still wrapped\"}"

	result := toolplane.RunToolChapterQualityGate(chapter, content, 200)
	if !result.Blocking {
		t.Fatalf("chapter quality gate should block raw JSON body: %#v", result)
	}
	if len(result.Suggestions) == 0 {
		t.Fatalf("expected chapter quality issues")
	}
	foundJSON := false
	foundShort := false
	for _, issue := range result.Suggestions {
		if strings.Contains(issue.Issue, "raw JSON") && issue.Priority == models.PriorityHigh {
			foundJSON = true
		}
		if strings.Contains(issue.Issue, "too short") && issue.Priority == models.PriorityHigh {
			foundShort = true
		}
	}
	if !foundJSON || !foundShort {
		t.Fatalf("missing expected JSON/length issues: %#v", result.Suggestions)
	}
}

func TestToolChapterQualityCheckFlagsRepeatedTitleAfterHeading(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "第1章：倒计时"}
	content := "# 第1章：倒计时\n\n第1章：倒计时李侑恢复意识时，鼻子里先钻进一股霉味。" + strings.Repeat("他开始整理眼前的处境。", 60)

	result := toolplane.RunToolChapterQualityGate(chapter, content, 100)
	found := false
	for _, issue := range result.Suggestions {
		if strings.Contains(issue.Issue, "body repeats the outline title") && issue.Priority == models.PriorityMedium {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected repeated title issue: %#v", result.Suggestions)
	}
}

func TestToolChapterQualityCheckFlagsInlineFormattingArtifacts(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C6", Title: "第6章：寻找新韭菜"}
	content := "# 第6章：寻找新韭菜\n\n" +
		"搜索结果弹出两组数据。<br><br>\n**第一条：后山朱果。**\n" +
		strings.Repeat("李侑继续观察现场并压下心中的吐槽。", 80)

	result := toolplane.RunToolChapterQualityGate(chapter, content, 1200)

	foundBR := false
	foundBold := false
	for _, issue := range result.Suggestions {
		if strings.Contains(issue.Issue, "HTML line break") && issue.Priority == models.PriorityMedium {
			foundBR = true
		}
		if strings.Contains(issue.Issue, "inline bold markdown") && issue.Priority == models.PriorityMedium {
			foundBold = true
		}
	}
	if !foundBR || !foundBold {
		t.Fatalf("expected HTML/bold markdown issues, got %#v", result.Suggestions)
	}
}

func TestToolChapterQualityCheckFlagsSuspiciousTypoArtifacts(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C8", Title: "第8章：系统的报错与数据挖掘"}
	content := "# 第8章：系统的报错与数据挖掘\n\n" +
		"苏瑶兴奋地说，太上长老今天给她安排了十好几个考验。" +
		strings.Repeat("李侑继续翻看报错代码并整理其中的坐标偏移。", 60)

	result := toolplane.RunToolChapterQualityGate(chapter, content, 1200)

	found := false
	for _, issue := range result.Suggestions {
		if strings.Contains(issue.Issue, "suspicious typo artifact") && strings.Contains(issue.Issue, "十好几个") && issue.Priority == models.PriorityMedium {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected suspicious typo issue, got %#v", result.Suggestions)
	}
}

func TestNormalizeChapterPatchContentStripsRepeatedTitle(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "第1章：倒计时"}

	got := toolplane.NormalizeChapterPatchContent(chapter, "# 第1章：倒计时\n\n第1章：倒计时李侑恢复意识时，鼻子里先钻进一股霉味。")
	want := "# 第1章：倒计时\n\n李侑恢复意识时，鼻子里先钻进一股霉味。"
	if got != want {
		t.Fatalf("normalized content = %q, want %q", got, want)
	}

	got = toolplane.NormalizeChapterPatchContent(chapter, "第1章：倒计时：李侑推开木窗。")
	want = "# 第1章：倒计时\n\n李侑推开木窗。"
	if got != want {
		t.Fatalf("normalized bare content = %q, want %q", got, want)
	}

	chapter = &models.Chapter{ID: "P1-V2-C1", Title: "第11章：完美的“分赃”与死忠粉的诞生"}
	got = toolplane.NormalizeChapterPatchContent(chapter, "# 第11章：完美的“分赃”与死忠粉的诞生\n\n# 第11章：完美的\"分赃\"与死忠粉的诞生\n\n清晨卯时，李侑翻开日志。")
	want = "# 第11章：完美的“分赃”与死忠粉的诞生\n\n清晨卯时，李侑翻开日志。"
	if got != want {
		t.Fatalf("normalized quoted title = %q, want %q", got, want)
	}
}

func TestToolChapterQualityCheckRequiresOutlineCharacters(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Characters: []string{"Lin"}}
	content := "# Opening\n\n" + strings.Repeat("A stranger follows the signal through the mine. ", 40)

	result := toolplane.RunToolChapterQualityGate(chapter, content, 100)
	if !result.Blocking {
		t.Fatalf("chapter quality gate should block missing outline character: %#v", result)
	}
	found := false
	for _, issue := range result.Suggestions {
		if issue.Category == "character" && strings.Contains(issue.Issue, "Lin") && issue.Priority == models.PriorityHigh {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing outline character issue: %#v", result.Suggestions)
	}
}

func TestToolChapterQualityCheckAcceptsChineseShortCharacterMention(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C2", Title: "外来者", Characters: []string{"据点守门长老吴伯"}}
	body := strings.Repeat("吴伯站在据点指挥中心里，要求众人放下弩箭，并给外来者三天观察期。", 30)
	content := "# 外来者\n\n" + body

	result := toolplane.RunToolChapterQualityGate(chapter, content, 100)
	if qualityGateHasIssue(result, "character", models.PriorityHigh) {
		t.Fatalf("short Chinese name should satisfy outline character mention: %#v", result.Suggestions)
	}
}

func TestToolChapterQualityCheckUsesTargetWordsOverride(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Characters: []string{"Lin"}}
	body := strings.Repeat("Lin reads the system log, checks Gu Heng's next intervention, and turns the warning into a practical route through the outer sect. ", 55)
	content := "# Opening\n\n" + body

	defaultTarget := toolplane.RunToolChapterQualityGate(chapter, content, 3000)
	if !defaultTarget.Blocking || !qualityGateHasIssue(defaultTarget, "length", models.PriorityHigh) {
		t.Fatalf("large target should flag short generated debug chapter: %#v", defaultTarget)
	}

	overrideTarget := toolplane.RunToolChapterQualityGate(chapter, content, 800)
	if overrideTarget.Blocking || qualityGateHasIssue(overrideTarget, "length", models.PriorityHigh) {
		t.Fatalf("explicit short target should not reuse project default length: %#v", overrideTarget)
	}
}

func TestToolChapterCheckNavigationPreservesTargetWordsOverride(t *testing.T) {
	result := toolplane.ToolCheckResult{
		Kind:     "all",
		Target:   "chapter",
		Scope:    "chapter",
		ID:       "P1-V1-C1",
		Blocking: true,
		Summary:  toolplane.ToolCheckSummary{Total: 1, High: 1},
		Issues: []models.ReviewSuggestion{{
			Category: "length",
			TargetID: "P1-V1-C1",
			Issue:    "final chapter is too short",
			Priority: models.PriorityHigh,
		}},
		Meta: map[string]interface{}{"target_words": 800},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`tool check all --target chapter --scope chapter --id 'P1-V1-C1' --category length --min-priority low --max-issues 8 --target-words 800`,
		`tool patch chapter --id 'P1-V1-C1' --target-words 800`,
		`write improve --agent-sdk --chapter \"P1-V1-C1\" --max-rounds 1 --words 800`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("target words navigation missing %q in %s", want, text)
		}
	}
}

func qualityGateHasIssue(gate quality.QualityGateResult, category, priority string) bool {
	for _, issue := range gate.Suggestions {
		if issue.Category == category && issue.Priority == priority {
			return true
		}
	}
	return false
}

func TestToolCheckChapterJSONAddsNavigation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "quality",
		Target: "chapter",
		Scope:  "chapter",
		ID:     "P1-V1-C1",
		Issues: []models.ReviewSuggestion{{
			Category: "style",
			TargetID: "P1-V1-C1",
			Issue:    "formulaic prose",
			Priority: models.PriorityMedium,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "tool query context --type chapter-write") ||
		!strings.Contains(text, `tool query context --type chapter-repair --id 'P1-V1-C1' --name 'style' --view index`) ||
		!strings.Contains(text, `tool query context --type chapter-repair --id 'P1-V1-C1' --name 'style' --view brief`) ||
		!strings.Contains(text, "tool check quality --target chapter") ||
		!strings.Contains(text, "tool check simulation --target chapter") ||
		!strings.Contains(text, "write improve --agent-sdk") {
		t.Fatalf("chapter navigation missing: %s", text)
	}
}

func TestToolCheckChapterSimulationNavigationFocusesSimulation(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "simulation",
		Target: "chapter",
		Scope:  "chapter",
		ID:     "P1-V1-C1",
		Issues: []models.ReviewSuggestion{{
			Category: "simulation",
			TargetID: "P1-V1-C1",
			Issue:    "chapter RPG DSL is stale",
			Priority: models.PriorityHigh,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []toolplane.ToolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	focused := decoded.Issues[0].Navigation["focused_check_query"].(string)
	if !strings.Contains(focused, "tool check simulation --target chapter") ||
		strings.Contains(focused, "tool check quality --target chapter") {
		t.Fatalf("simulation issue focused query should use simulation check: %s", focused)
	}
	refresh := decoded.Issues[0].Navigation["refresh_query"].(string)
	if !strings.Contains(refresh, "tool refresh chapter-dsl --id") {
		t.Fatalf("simulation issue missing refresh query: %#v", decoded.Issues[0].Navigation)
	}
	postRefresh := decoded.Issues[0].Navigation["post_refresh_check_query"].(string)
	if postRefresh != focused {
		t.Fatalf("post refresh check should match focused simulation check: %#v", decoded.Issues[0].Navigation)
	}
	if len(decoded.NextActions) != 4 ||
		decoded.NextActions[0].Action != "refresh_derived_dsl_first" ||
		decoded.NextActions[1].Action != "refresh_derived_dsl" ||
		!strings.Contains(decoded.NextActions[1].Command, "tool refresh chapter-dsl") ||
		decoded.NextActions[2].Action != "post_refresh_check" ||
		decoded.NextActions[2].Command != focused ||
		decoded.NextActions[3].Action != "return_final_json" {
		t.Fatalf("stale simulation next_actions should refresh before repair: %#v", decoded.NextActions)
	}
	for _, action := range decoded.NextActions {
		if action.Action == "query_repair_context_if_needed" || action.Action == "patch_dry_run" {
			t.Fatalf("stale simulation should not query repair context or patch before refresh: %#v", decoded.NextActions)
		}
	}
}

func TestToolCheckChapterSimulationNavigationDoesNotRefreshForOrdinaryIssues(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "simulation",
		Target: "chapter",
		Scope:  "chapter",
		ID:     "P1-V1-C1",
		Issues: []models.ReviewSuggestion{{
			Category: "logic",
			TargetID: "P1-V1-C1",
			Issue:    "战斗难度过高",
			Priority: models.PriorityCritical,
		}},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Issues []struct {
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded.Issues[0].Navigation["refresh_query"]; ok {
		t.Fatalf("ordinary simulation issue should not force DSL refresh: %#v", decoded.Issues[0].Navigation)
	}
	if _, ok := decoded.Issues[0].Navigation["post_refresh_check_query"]; ok {
		t.Fatalf("ordinary simulation issue should not include post-refresh check: %#v", decoded.Issues[0].Navigation)
	}
}

func TestAgentSDKChapterCheckSuggestionsPreferRepairContext(t *testing.T) {
	result := &toolplane.ToolCheckResult{
		Kind:   "all",
		Target: "chapter",
		Scope:  "chapter",
		ID:     "P1-V1-C1",
		Issues: []models.ReviewSuggestion{{
			Category: "character",
			TargetID: "P1-V1-C1",
			Issue:    "final chapter text does not mention outline character(s): Lin",
			Priority: models.PriorityHigh,
		}},
	}

	text := writepipe.FormatAgentSDKChapterCheckSuggestions(result)
	if !strings.Contains(text, "primary repair task list") ||
		!strings.Contains(text, "navigation.repair_route_query") ||
		!strings.Contains(text, "navigation.repair_context_query") ||
		!strings.Contains(text, "tool query context --type chapter-repair") ||
		!strings.Contains(text, "tool check --target chapter") {
		t.Fatalf("agent-sdk chapter check suggestions missing repair navigation: %s", text)
	}
}

func TestAgentSDKPostSaveCheckAppendsReviewSuggestions(t *testing.T) {
	review := &agents.DraftReview{ChapterID: "P1-V1-C1", OverallScore: 80}
	result := &toolplane.ToolCheckResult{
		Kind:     "all",
		Target:   "chapter",
		Scope:    "chapter",
		ID:       "P1-V1-C1",
		Blocking: true,
		Issues: []models.ReviewSuggestion{{
			Category: "character",
			TargetID: "P1-V1-C1",
			Issue:    "final chapter text does not mention outline character(s): Lin",
			Priority: models.PriorityHigh,
		}},
	}

	writecmd.AppendAgentSDKPostSaveReviewSuggestions(review, result)
	if !review.NeedsRevision {
		t.Fatalf("blocking post-save check should mark review as needing revision")
	}
	if len(review.Suggestions) != 1 {
		t.Fatalf("suggestions len = %d, want 1: %#v", len(review.Suggestions), review.Suggestions)
	}
	got := review.Suggestions[0]
	if !strings.Contains(got, "Agent SDK post-save check still reports") ||
		!strings.Contains(got, "repair_route_query=novelgen tool query context --type chapter-repair") ||
		!strings.Contains(got, "repair_context_query=novelgen tool query context --type chapter-repair") ||
		!strings.Contains(got, "focused_check_query=novelgen tool check all --target chapter") {
		t.Fatalf("post-save suggestion missing repair navigation: %s", got)
	}
	writecmd.AppendAgentSDKPostSaveReviewSuggestions(review, result)
	if len(review.Suggestions) != 1 {
		t.Fatalf("duplicate post-save suggestion appended: %#v", review.Suggestions)
	}
}

func TestAgentSDKLengthRetrySuggestions(t *testing.T) {
	text := writepipe.AppendAgentSDKLengthRetrySuggestions("base suggestions", 3000)
	if !strings.Contains(text, "base suggestions") ||
		!strings.Contains(text, "Agent SDK Length Retry") ||
		!strings.Contains(text, "minimal repair only") ||
		!strings.Contains(text, "preferably 2700-3300") ||
		!strings.Contains(text, "must not exceed 4050") {
		t.Fatalf("length retry suggestions missing strict guidance: %s", text)
	}
	if !writepipe.IsAgentSDKLengthOvershootError(fmt.Errorf("chapter P1-V1-C1: %w", agents.ErrAgentSDKChapterTooLong)) {
		t.Fatalf("expected overshoot error detection")
	}
	if writepipe.IsAgentSDKLengthOvershootError(fmt.Errorf("some other error")) {
		t.Fatalf("unexpected overshoot detection")
	}
}

func TestToolChapterSimulationCheckReportsMissingDSL(t *testing.T) {
	result := toolplane.RunToolChapterSimulationGate(t.TempDir(), "P1-V1-C1")
	if !result.Blocking {
		t.Fatalf("simulation gate should block when RPG DSL is missing: %#v", result)
	}
	if len(result.Suggestions) != 1 {
		t.Fatalf("suggestions len = %d, want 1: %#v", len(result.Suggestions), result.Suggestions)
	}
	issue := result.Suggestions[0]
	if issue.Category != "simulation" || issue.TargetID != "P1-V1-C1" || issue.Priority != models.PriorityHigh {
		t.Fatalf("unexpected missing DSL issue: %#v", issue)
	}
	if !strings.Contains(issue.Suggestion, "tool refresh chapter-dsl") ||
		!strings.Contains(issue.Suggestion, "do not invoke LLM conversion") {
		t.Fatalf("suggestion should explain no LLM conversion: %q", issue.Suggestion)
	}
}

func TestToolChapterSimulationCheckReportsStaleDSL(t *testing.T) {
	root := t.TempDir()
	chapterID := "P1-V1-C1"
	chapterDir := filepath.Join(root, "chapters")
	rpgDir := filepath.Join(root, "story", "rpg")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rpgDir, 0755); err != nil {
		t.Fatal(err)
	}
	dslPath := filepath.Join(rpgDir, "04_chapters.rpg")
	chapterPath := filepath.Join(chapterDir, "chapter-"+chapterID+".md")
	if err := os.WriteFile(dslPath, []byte("invalid old dsl"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(dslPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chapterPath, []byte("# Chapter\n\nnew content"), 0644); err != nil {
		t.Fatal(err)
	}

	result := toolplane.RunToolChapterSimulationGate(root, chapterID)
	if !result.Blocking || len(result.Suggestions) != 1 {
		t.Fatalf("stale DSL should produce one blocking issue: %#v", result)
	}
	issue := result.Suggestions[0]
	if issue.Category != "simulation" || issue.Priority != models.PriorityHigh ||
		!strings.Contains(issue.Issue, "stale") ||
		!strings.Contains(issue.Suggestion, "tool refresh chapter-dsl") ||
		!strings.Contains(issue.Suggestion, "do not invoke LLM conversion") {
		t.Fatalf("unexpected stale issue: %#v", issue)
	}
}

func TestChapterSimulationSignalDiagnosticReportsMissingSignals(t *testing.T) {
	dslData := &rpgdsl.DSL{Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{{
		ID: "P1-V1-C1",
		Objectives: []rpgdsl.Objective{{Steps: []rpgdsl.Step{{
			Order:       1,
			Description: "Lin fights in a narrow terrain ambush.",
			Event: rpgdsl.Event{
				Type: "combat",
				Combat: &rpgdsl.CombatEvent{Setup: rpgdsl.CombatSetup{Enemies: []rpgdsl.EnemySpawn{{
					ID:    "enemy_worker_bee",
					Count: 3,
					Level: 2,
				}}}},
			},
		}}}},
	}}}}

	issue := toolplane.BuildChapterSimulationSignalDiagnostic("P1-V1-C1", dslData, []rpgdsl.SimulationIssue{{
		Chapter:     "P1-V1-C1",
		Type:        rpgdsl.IssueBalance,
		Severity:    rpgdsl.SeverityCritical,
		Description: "战斗难度过高",
	}})
	if issue == nil {
		t.Fatal("expected signal diagnostic")
	}
	text := issue.Issue + " " + issue.Suggestion
	for _, want := range []string{
		"simulation signal diagnostics",
		"enemy_worker_bee(count=3,level=2",
		"combat_result=false",
		"mech=false",
		"tactical_text=true",
		"missing_repair_signals=combat_result_on_complete, power_change, mech, gene, equipment_or_item, ally",
		"refresh chapter-dsl",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("diagnostic missing %q: %s", want, text)
		}
	}
}

func TestChapterSimulationSignalDiagnosticSeesStructuredRepairSignals(t *testing.T) {
	dslData := &rpgdsl.DSL{Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{{
		ID: "P1-V1-C1",
		Objectives: []rpgdsl.Objective{{Steps: []rpgdsl.Step{{
			Order:       1,
			Description: "Lin uses 火种机甲 and terrain.",
			Event: rpgdsl.Event{
				Type: "combat",
				Combat: &rpgdsl.CombatEvent{
					Setup: rpgdsl.CombatSetup{Enemies: []rpgdsl.EnemySpawn{{ID: "enemy_worker_bee", Count: 1}}},
				},
				OnComplete: &rpgdsl.EventResult{Narration: "Lin wins and limps away with combat data."},
				StateDeltas: []rpgdsl.StateDelta{
					{Target: "char_lin", Kind: "power_change", Delta: 50},
					{Target: "char_lin", Kind: "mech", Field: "form", To: "残骸外骨骼形态"},
					{Target: "char_lin", Kind: "gene", Field: "stability", To: "70"},
					{Target: "fire_core", Kind: "equipment", Field: "key_item", To: "火种核心"},
					{Target: "char_lin", Kind: "ally", To: "青藤勘探队"},
				},
			},
		}}}},
	}}}}

	issue := toolplane.BuildChapterSimulationSignalDiagnostic("P1-V1-C1", dslData, []rpgdsl.SimulationIssue{{
		Chapter:     "P1-V1-C1",
		Type:        rpgdsl.IssueBalance,
		Severity:    rpgdsl.SeverityCritical,
		Description: "战斗难度过高",
	}})
	if issue == nil {
		t.Fatal("expected signal diagnostic")
	}
	text := issue.Issue
	for _, want := range []string{
		"combat_result=true",
		"power_change=true",
		"gene=true",
		"mech=true",
		"equipment_or_item=true",
		"ally=true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("diagnostic missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "missing_repair_signals=combat_result_on_complete") ||
		strings.Contains(text, "missing_repair_signals=power_change") {
		t.Fatalf("diagnostic should not report present repair signals as missing: %s", text)
	}
}

func TestToolRefreshChapterDSLCheckMapsScopedSimulationIssues(t *testing.T) {
	issues := []rpgdsl.SimulationIssue{
		{
			Chapter:     "P1-V1-C1",
			Type:        rpgdsl.IssueContinuity,
			Severity:    rpgdsl.SeverityCritical,
			Description: "state delta contradicts prior state",
			Suggestion:  "repair the saved chapter or generated DSL state delta",
		},
		{
			Chapter:     "P1-V1-C2",
			Type:        rpgdsl.IssueLogic,
			Severity:    rpgdsl.SeverityWarning,
			Description: "sibling issue",
			Suggestion:  "ignore for target chapter",
		},
	}

	check := toolplane.MakeToolRefreshChapterDSLCheck("P1-V1-C1", issues)
	if check == nil || check.OK || !check.Blocking || check.Summary.Critical != 1 || check.Summary.Total != 1 {
		t.Fatalf("unexpected refresh check: %#v", check)
	}
	if len(check.Issues) != 1 || check.Issues[0].TargetID != "P1-V1-C1" || !strings.Contains(check.Issues[0].Issue, "state delta") {
		t.Fatalf("unexpected scoped issues: %#v", check.Issues)
	}
}

func TestRunToolRefreshChapterDSLRequiresIDBeforeLLM(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Refresh Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := toolCmd(t, root)

	originalFlags := toolplane.ToolRefreshFlags
	defer func() { toolplane.ToolRefreshFlags = originalFlags }()
	toolplane.ToolRefreshFlags = struct {
		ID        string
		BatchSize int
	}{}

	err := toolplane.RunToolRefresh(cmd, []string{"chapter-dsl"})
	if err == nil || !strings.Contains(err.Error(), "--id is required") {
		t.Fatalf("expected --id error before LLM setup, got %v", err)
	}
}
