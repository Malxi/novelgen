package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelgen/internal/agents"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"

	"github.com/spf13/cobra"
)

func decodeToolPatchResult(t *testing.T, raw string) toolPatchResult {
	t.Helper()
	var result toolPatchResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode patch result: %v\n%s", err, raw)
	}
	return result
}

func toolCheckHasIssue(result *toolCheckResult, text string) bool {
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

func assertToolCheckIssue(t *testing.T, result *toolCheckResult, category, targetID, issueContains string) {
	t.Helper()
	if result == nil {
		t.Fatal("check result is nil")
	}
	for _, issue := range result.Issues {
		if normalizeKey(issue.Category) == normalizeKey(category) &&
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

	result, err := runToolOutlineCheck("quality", nil, outline, "chapter", "P1-V1-C1")
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
	result, err := runToolOutlineCheck("all", nil, outline, "volume", "P1-V1")
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
	chapter1 := validChapterForQualityGate()
	chapter1.ID = "P1-V1-C1"
	chapter1.Location = "Outer Gate"
	chapter1.StateAnchor.Location = "Outer Gate"
	chapter2 := validChapterForQualityGate()
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

	result, err := runToolOutlineCheck("all", nil, outline, "chapter", "P1-V1-C2")
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

	result, err := runToolCraftSchemaCheck(root, "item", "survival_kit")
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

	filtered := filterToolScopedSuggestions(suggestions, outline, "volume", "P1-V1")
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

	filtered := filterToolScopedSuggestions(suggestions, outline, "chapter", "P1-V1-C1")
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1: %#v", len(filtered), filtered)
	}
	if filtered[0].TargetID != "P1-V1-C1" {
		t.Fatalf("kept target = %q, want P1-V1-C1", filtered[0].TargetID)
	}
}

func TestApplyToolCheckIssueFiltersKeepsSummaryAndLimitsReturnedIssues(t *testing.T) {
	result := &toolCheckResult{
		Summary: toolCheckSummary{Total: 4, Critical: 1, High: 1, Medium: 1, Low: 1},
		Issues: []models.ReviewSuggestion{
			{TargetID: "low", Priority: models.PriorityLow},
			{TargetID: "medium", Priority: models.PriorityMedium},
			{TargetID: "high", Priority: models.PriorityHigh},
			{TargetID: "critical", Priority: models.PriorityCritical},
		},
		Meta: map[string]interface{}{"project_root": "root"},
	}

	if err := applyToolCheckIssueFilters(result, "medium", "", 2); err != nil {
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
	result := &toolCheckResult{
		Summary: toolCheckSummary{Total: 4, Critical: 1, High: 1, Medium: 1, Low: 1},
		Issues: []models.ReviewSuggestion{
			{TargetID: "logic-low", Category: "logic", Priority: models.PriorityLow},
			{TargetID: "plot-medium", Category: "plot", Priority: models.PriorityMedium},
			{TargetID: "logic-high", Category: "logic", Priority: models.PriorityHigh},
			{TargetID: "structure-critical", Category: "structure", Priority: models.PriorityCritical},
		},
		Meta: map[string]interface{}{},
	}

	if err := applyToolCheckIssueFilters(result, "medium", " logic,structure ", 0); err != nil {
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction       `json:"next_actions"`
		Meta        map[string]interface{} `json:"meta"`
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
	if !strings.Contains(nav["repair_route_query"].(string), `outline-repair --id "P1-V1-C3" --name "logic" --view index`) {
		t.Fatalf("missing repair route query: %#v", nav["repair_route_query"])
	}
	if !strings.Contains(nav["repair_context_query"].(string), `outline-repair --id "P1-V1-C3" --name "logic"`) {
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
		!strings.Contains(decoded.NextActions[1].Command, `outline-repair --id "P1-V1-C3" --name "logic" --view index`) {
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction       `json:"next_actions"`
		Meta        map[string]interface{} `json:"meta"`
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
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
	if nav["repair_route_query"] != `novelgen tool query context --type outline-global-repair --name "mysteries" --view index` {
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
		decoded.NextActions[1].Command != `novelgen tool query context --type outline-global-repair --name "mysteries" --view index` ||
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
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
	if !strings.Contains(decoded.NextActions[1].Command, `story-setup --type search --name "zerg" --view index`) ||
		decoded.NextActions[3].Command != "novelgen tool patch setup" ||
		!strings.Contains(decoded.NextActions[4].Command, "tool check all --target outline --category faction_tier") {
		t.Fatalf("next_actions should route to setup patch: %#v", decoded.NextActions)
	}
}

func TestToolIssueNavigationRoutesVolumeRedundancyToChangedEvents(t *testing.T) {
	nav := toolIssueNavigation("all", "outline", "volume", "P1-V1", models.ReviewSuggestion{
		Category: "redundancy",
		TargetID: "P1-V1",
		Issue:    "event type repeated",
		Priority: models.PriorityMedium,
	}, 0)
	if got := fmt.Sprint(nav["patch_query"]); got != `novelgen tool patch outline --target volume --id "P1-V1"` {
		t.Fatalf("patch query = %q", got)
	}
	detailQueries, ok := nav["detail_queries"].([]string)
	if !ok {
		t.Fatalf("detail_queries type = %T", nav["detail_queries"])
	}
	if !containsString(detailQueries, `novelgen tool query outline --type events --volume-id "P1-V1" --fields event_index,chapter_id,type,action,actor,target,target_type,details,result --view brief`) {
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
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
	if !strings.Contains(decoded.NextActions[1].Command, `outline-repair --id "P1-V1-C3" --name "logic" --view index`) ||
		!strings.Contains(decoded.NextActions[3].Command, `tool patch outline --target volume --id "P1-V1"`) {
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
	result := &toolCheckResult{
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
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
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type search --name "zerg" --view index`) ||
		!strings.Contains(nav["repair_context_query"].(string), `story-setup --type search --name "zerg" --view brief`) {
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
	result := &toolCheckResult{
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
		!strings.Contains(text, `craft-character --name \"林野\" --view index`) ||
		!strings.Contains(text, `repair_context_query`) ||
		!strings.Contains(text, `craft-character --name \"林野\" --view brief`) ||
		!strings.Contains(string(data), "tool patch craft") ||
		!strings.Contains(string(data), "tool check schema") {
		t.Fatalf("craft navigation missing: %s", text)
	}
}

func TestToolCheckRecapJSONAddsNavigation(t *testing.T) {
	result := &toolCheckResult{
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
		!strings.Contains(text, `recap-repair --id \"P1-V1-C1\" --view index`) ||
		!strings.Contains(text, `repair_context_query`) ||
		!strings.Contains(text, `recap-repair --id \"P1-V1-C1\" --view brief`) ||
		!strings.Contains(text, "novelgen recap gen --chapter") {
		t.Fatalf("recap navigation missing: %s", text)
	}
}

func TestToolCheckSetupJSONAddsRouteNavigation(t *testing.T) {
	result := &toolCheckResult{
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
		`story-setup --type search --name \"premise\" --view index`,
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "setup.storyline" {
		t.Fatalf("unexpected indexed storyline navigation: %#v", nav)
	}
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type storyline --name "Signal War" --view index`) ||
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
	result := &toolCheckResult{
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
		`context --type outline-global-repair --name \"structure\" --view index`,
		`repair_route_query`,
		`classification_rule`,
		`return_final_json`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("global outline navigation missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{
		`context --type outline-global-repair --name \"structure\" --view brief`,
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "setup.premise" {
		t.Fatalf("unexpected indexed premise navigation: %#v", nav)
	}
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type premise --name "虫族阶层与天敌体系" --view index`) ||
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
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	nav := decoded.Issues[0].Navigation
	if nav["target_kind"] != "setup.rule" {
		t.Fatalf("unexpected indexed rule navigation: %#v", nav)
	}
	if !strings.Contains(nav["repair_route_query"].(string), `story-setup --type search --name "rules[4]" --view index`) {
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

	result := runToolChapterQualityGate(chapter, content, 200)
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

	result := runToolChapterQualityGate(chapter, content, 100)
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

	result := runToolChapterQualityGate(chapter, content, 1200)

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

	result := runToolChapterQualityGate(chapter, content, 1200)

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

	got := normalizeChapterPatchContent(chapter, "# 第1章：倒计时\n\n第1章：倒计时李侑恢复意识时，鼻子里先钻进一股霉味。")
	want := "# 第1章：倒计时\n\n李侑恢复意识时，鼻子里先钻进一股霉味。"
	if got != want {
		t.Fatalf("normalized content = %q, want %q", got, want)
	}

	got = normalizeChapterPatchContent(chapter, "第1章：倒计时：李侑推开木窗。")
	want = "# 第1章：倒计时\n\n李侑推开木窗。"
	if got != want {
		t.Fatalf("normalized bare content = %q, want %q", got, want)
	}

	chapter = &models.Chapter{ID: "P1-V2-C1", Title: "第11章：完美的“分赃”与死忠粉的诞生"}
	got = normalizeChapterPatchContent(chapter, "# 第11章：完美的“分赃”与死忠粉的诞生\n\n# 第11章：完美的\"分赃\"与死忠粉的诞生\n\n清晨卯时，李侑翻开日志。")
	want = "# 第11章：完美的“分赃”与死忠粉的诞生\n\n清晨卯时，李侑翻开日志。"
	if got != want {
		t.Fatalf("normalized quoted title = %q, want %q", got, want)
	}
}

func TestToolChapterQualityCheckRequiresOutlineCharacters(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Characters: []string{"Lin"}}
	content := "# Opening\n\n" + strings.Repeat("A stranger follows the signal through the mine. ", 40)

	result := runToolChapterQualityGate(chapter, content, 100)
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

	result := runToolChapterQualityGate(chapter, content, 100)
	if qualityGateHasIssue(result, "character", models.PriorityHigh) {
		t.Fatalf("short Chinese name should satisfy outline character mention: %#v", result.Suggestions)
	}
}

func TestToolChapterQualityCheckUsesTargetWordsOverride(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Characters: []string{"Lin"}}
	body := strings.Repeat("Lin reads the system log, checks Gu Heng's next intervention, and turns the warning into a practical route through the outer sect. ", 55)
	content := "# Opening\n\n" + body

	defaultTarget := runToolChapterQualityGate(chapter, content, 3000)
	if !defaultTarget.Blocking || !qualityGateHasIssue(defaultTarget, "length", models.PriorityHigh) {
		t.Fatalf("large target should flag short generated debug chapter: %#v", defaultTarget)
	}

	overrideTarget := runToolChapterQualityGate(chapter, content, 800)
	if overrideTarget.Blocking || qualityGateHasIssue(overrideTarget, "length", models.PriorityHigh) {
		t.Fatalf("explicit short target should not reuse project default length: %#v", overrideTarget)
	}
}

func TestToolChapterCheckNavigationPreservesTargetWordsOverride(t *testing.T) {
	result := toolCheckResult{
		Kind:     "all",
		Target:   "chapter",
		Scope:    "chapter",
		ID:       "P1-V1-C1",
		Blocking: true,
		Summary:  toolCheckSummary{Total: 1, High: 1},
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
		`tool check all --target chapter --scope chapter --id \"P1-V1-C1\" --category length --min-priority low --max-issues 8 --target-words 800`,
		`tool patch chapter --id \"P1-V1-C1\" --target-words 800`,
		`write improve --agent-sdk --chapter \"P1-V1-C1\" --max-rounds 1 --words 800`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("target words navigation missing %q in %s", want, text)
		}
	}
}

func qualityGateHasIssue(gate qualityGateResult, category, priority string) bool {
	for _, issue := range gate.Suggestions {
		if issue.Category == category && issue.Priority == priority {
			return true
		}
	}
	return false
}

func TestToolCheckChapterJSONAddsNavigation(t *testing.T) {
	result := &toolCheckResult{
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
		!strings.Contains(text, `tool query context --type chapter-repair --id \"P1-V1-C1\" --name \"style\" --view index`) ||
		!strings.Contains(text, `tool query context --type chapter-repair --id \"P1-V1-C1\" --name \"style\" --view brief`) ||
		!strings.Contains(text, "tool check quality --target chapter") ||
		!strings.Contains(text, "tool check simulation --target chapter") ||
		!strings.Contains(text, "write improve --agent-sdk") {
		t.Fatalf("chapter navigation missing: %s", text)
	}
}

func TestToolCheckChapterSimulationNavigationFocusesSimulation(t *testing.T) {
	result := &toolCheckResult{
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
		NextActions []toolNextAction `json:"next_actions"`
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
	result := &toolCheckResult{
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
	result := &toolCheckResult{
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

	text := formatAgentSDKChapterCheckSuggestions(result)
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
	result := &toolCheckResult{
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

	appendAgentSDKPostSaveReviewSuggestions(review, result)
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
	appendAgentSDKPostSaveReviewSuggestions(review, result)
	if len(review.Suggestions) != 1 {
		t.Fatalf("duplicate post-save suggestion appended: %#v", review.Suggestions)
	}
}

func TestAgentSDKLengthRetrySuggestions(t *testing.T) {
	text := appendAgentSDKLengthRetrySuggestions("base suggestions", 3000)
	if !strings.Contains(text, "base suggestions") ||
		!strings.Contains(text, "Agent SDK Length Retry") ||
		!strings.Contains(text, "minimal repair only") ||
		!strings.Contains(text, "preferably 2700-3300") ||
		!strings.Contains(text, "must not exceed 4050") {
		t.Fatalf("length retry suggestions missing strict guidance: %s", text)
	}
	if !isAgentSDKLengthOvershootError(fmt.Errorf("agent-sdk returned too much content for chapter P1-V1-C1")) {
		t.Fatalf("expected overshoot error detection")
	}
	if isAgentSDKLengthOvershootError(fmt.Errorf("some other error")) {
		t.Fatalf("unexpected overshoot detection")
	}
}

func TestToolChapterSimulationCheckReportsMissingDSL(t *testing.T) {
	result := runToolChapterSimulationGate(t.TempDir(), "P1-V1-C1")
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

	result := runToolChapterSimulationGate(root, chapterID)
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

	issue := buildChapterSimulationSignalDiagnostic("P1-V1-C1", dslData, []rpgdsl.SimulationIssue{{
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

	issue := buildChapterSimulationSignalDiagnostic("P1-V1-C1", dslData, []rpgdsl.SimulationIssue{{
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

	check := makeToolRefreshChapterDSLCheck("P1-V1-C1", issues)
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
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()

	originalFlags := toolRefreshFlags
	defer func() { toolRefreshFlags = originalFlags }()
	toolRefreshFlags = struct {
		ID        string
		BatchSize int
	}{}

	err = runToolRefresh(&cobra.Command{}, []string{"chapter-dsl"})
	if err == nil || !strings.Contains(err.Error(), "--id is required") {
		t.Fatalf("expected --id error before LLM setup, got %v", err)
	}
}

func TestToolPatchChapterDryRunAndApply(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	outline := validToolPatchTestOutline()
	if err := writeToolTestOutline(root, outline); err != nil {
		t.Fatal(err)
	}
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	chapterPath := filepath.Join(chapterDir, "chapter-P1-V1-C1.md")
	if err := os.WriteFile(chapterPath, []byte("# Opening\n\nLin starts here."), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()

	content := "# Opening\n\n" + strings.Repeat("Lin repairs the star core inside the mine and keeps the signal stable. ", 140)
	patch := fmt.Sprintf(`{"content":%q}`, content)

	originalFlags := toolPatchFlags
	defer func() { toolPatchFlags = originalFlags }()
	toolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{ID: "P1-V1-C1", PatchJSON: patch}
	var dryOut bytes.Buffer
	dryCmd := &cobra.Command{}
	dryCmd.SetOut(&dryOut)
	if err := runToolPatch(dryCmd, []string{"chapter"}); err != nil {
		t.Fatalf("dry-run chapter patch: %v", err)
	}
	var dryResult toolPatchResult
	if err := json.Unmarshal(dryOut.Bytes(), &dryResult); err != nil {
		t.Fatalf("decode dry-run result: %v\n%s", err, dryOut.String())
	}
	if dryResult.Applied || !dryResult.DryRun || dryResult.Target != "chapter" || dryResult.Check == nil || dryResult.Check.Kind != "quality" {
		t.Fatalf("unexpected dry-run result: %#v", dryResult)
	}
	if len(dryResult.NextActions) == 0 || dryResult.NextActions[0].Action != "apply_validated_patch" {
		t.Fatalf("dry-run should return apply next action: %#v", dryResult.NextActions)
	}
	if data, err := os.ReadFile(chapterPath); err != nil || strings.Contains(string(data), "repairs the star core") {
		t.Fatalf("dry-run should not modify chapter, err=%v content=%s", err, string(data))
	}

	toolPatchFlags.Apply = true
	var applyOut bytes.Buffer
	applyCmd := &cobra.Command{}
	applyCmd.SetOut(&applyOut)
	if err := runToolPatch(applyCmd, []string{"chapter"}); err != nil {
		t.Fatalf("apply chapter patch: %v", err)
	}
	var applyResult toolPatchResult
	if err := json.Unmarshal(applyOut.Bytes(), &applyResult); err != nil {
		t.Fatalf("decode apply result: %v\n%s", err, applyOut.String())
	}
	if !applyResult.Applied || applyResult.DryRun || applyResult.Files["chapter"] != chapterPath || applyResult.Files["checkpoint"] == "" {
		t.Fatalf("unexpected apply result: %#v", applyResult)
	}
	if applyResult.Check == nil || applyResult.Check.Kind != "quality" || applyResult.Check.Meta["mode"] != "post_apply_quality" ||
		applyResult.Check.Meta["simulation_requires_derived_dsl_refresh"] != true {
		t.Fatalf("apply should return post-apply quality check with simulation refresh marker: %#v", applyResult.Check)
	}
	coverage, ok := applyResult.Check.Meta["coverage"].(map[string]interface{})
	if !ok || coverage["target"] != "chapter" || coverage["kind"] != "quality" {
		t.Fatalf("chapter apply check should preserve coverage meta: %#v", applyResult.Check.Meta)
	}
	if len(applyResult.NextActions) != 2 ||
		applyResult.NextActions[0].Action != "refresh_derived_dsl" ||
		!strings.Contains(applyResult.NextActions[0].Command, `tool refresh chapter-dsl --id "P1-V1-C1"`) ||
		applyResult.NextActions[1].Action != "post_refresh_check" ||
		!strings.Contains(applyResult.NextActions[1].Command, `tool check all --target chapter --scope chapter --id "P1-V1-C1"`) {
		t.Fatalf("apply should return refresh/check next actions: %#v", applyResult.NextActions)
	}
	data, err := os.ReadFile(chapterPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repairs the star core") {
		t.Fatalf("apply did not write patched chapter: %s", string(data))
	}
	if _, err := os.Stat(applyResult.Files["checkpoint"]); err != nil {
		t.Fatalf("checkpoint missing: %v", err)
	}
}

func TestToolPatchBufferAppendShowClear(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Buffer Test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()

	originalFlags := toolPatchBufferFlags
	defer func() { toolPatchBufferFlags = originalFlags }()

	toolPatchBufferFlags.ID = "P1-V1-C1-draft"
	toolPatchBufferFlags.Text = "# Opening\n\nLin repairs "
	appendCmd := &cobra.Command{}
	var appendOut bytes.Buffer
	appendCmd.SetOut(&appendOut)
	if err := runToolPatchBuffer(appendCmd, []string{"append"}); err != nil {
		t.Fatalf("append buffer: %v", err)
	}

	toolPatchBufferFlags.Text = "the star core."
	var appendOut2 bytes.Buffer
	appendCmd2 := &cobra.Command{}
	appendCmd2.SetOut(&appendOut2)
	if err := runToolPatchBuffer(appendCmd2, []string{"append"}); err != nil {
		t.Fatalf("append buffer second chunk: %v", err)
	}

	toolPatchBufferFlags.Text = ""
	toolPatchBufferFlags.Stdin = true
	var appendOut3 bytes.Buffer
	appendCmd3 := &cobra.Command{}
	appendCmd3.SetIn(strings.NewReader("\n林野用 stdin 继续追加。"))
	appendCmd3.SetOut(&appendOut3)
	if err := runToolPatchBuffer(appendCmd3, []string{"append"}); err != nil {
		t.Fatalf("append buffer stdin chunk: %v", err)
	}

	toolPatchBufferFlags.Stdin = false
	toolPatchBufferFlags.MaxChars = 18
	var showOut bytes.Buffer
	showCmd := &cobra.Command{}
	showCmd.SetOut(&showOut)
	if err := runToolPatchBuffer(showCmd, []string{"show"}); err != nil {
		t.Fatalf("show buffer: %v", err)
	}
	var shown map[string]interface{}
	if err := json.Unmarshal(showOut.Bytes(), &shown); err != nil {
		t.Fatalf("decode show output: %v\n%s", err, showOut.String())
	}
	if shown["truncated"] != true || !strings.Contains(shown["preview"].(string), "Opening") {
		t.Fatalf("unexpected show output: %#v", shown)
	}

	path, err := toolPatchBufferPath(root, "P1-V1-C1-draft")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Opening\n\nLin repairs the star core.\n林野用 stdin 继续追加。" {
		t.Fatalf("unexpected buffer content: %q", string(data))
	}

	var clearOut bytes.Buffer
	clearCmd := &cobra.Command{}
	clearCmd.SetOut(&clearOut)
	if err := runToolPatchBuffer(clearCmd, []string{"clear"}); err != nil {
		t.Fatalf("clear buffer: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("buffer should be removed, stat err=%v", err)
	}
}

func TestBuildChapterPatchNextActionsAfterRefreshDerived(t *testing.T) {
	clean := &toolCheckResult{Blocking: false}
	actions := buildChapterPatchNextActions("P1-V1-C1", true, true, clean)
	if len(actions) != 1 || actions[0].Action != "return_final_json" ||
		strings.Contains(actions[0].Command, "refresh chapter-dsl") {
		t.Fatalf("refreshed clean patch should stop without another refresh: %#v", actions)
	}

	blocking := &toolCheckResult{Blocking: true}
	actions = buildChapterPatchNextActions("P1-V1-C1", true, true, blocking)
	if len(actions) != 1 || actions[0].Action != "repair_remaining_issues" ||
		!strings.Contains(actions[0].Command, `tool check all --target chapter --scope chapter --id "P1-V1-C1"`) {
		t.Fatalf("refreshed blocking patch should route to repair check: %#v", actions)
	}
}

func TestLoadToolPatchBufferWrapsChapterContent(t *testing.T) {
	root := t.TempDir()
	path, err := toolPatchBufferPath(root, "chapter-draft")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	content := "# Opening\n\n林野完成一次受控修复。"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	payload, err := loadToolPatchBuffer(root, "chapter-draft", "chapter")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode wrapped payload: %v\n%s", err, string(payload))
	}
	if decoded["content"] != content {
		t.Fatalf("content = %q, want %q", decoded["content"], content)
	}
	if _, err := toolPatchBufferPath(root, "../bad"); err == nil {
		t.Fatalf("expected invalid patch buffer id to be rejected")
	}
}

func TestToolPatchBufferShowCapsPreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Buffer Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	path, err := toolPatchBufferPath(root, "long-draft")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("字", 3000)), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()

	originalFlags := toolPatchBufferFlags
	defer func() { toolPatchBufferFlags = originalFlags }()
	toolPatchBufferFlags.ID = "long-draft"
	toolPatchBufferFlags.MaxChars = 999999
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runToolPatchBuffer(cmd, []string{"show"}); err != nil {
		t.Fatal(err)
	}
	var shown map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &shown); err != nil {
		t.Fatalf("decode show output: %v\n%s", err, out.String())
	}
	if len([]rune(shown["preview"].(string))) != 2000 || shown["truncated"] != true {
		t.Fatalf("preview should be capped to 2000 chars: %#v", shown)
	}
}

func TestToolPatchValidationUsesQualityAndSimulation(t *testing.T) {
	outline := validToolPatchTestOutline()
	result, err := runToolPatchValidationCheck(nil, outline, "volume", "P1-V1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "all" {
		t.Fatalf("patch validation kind = %q, want all", result.Kind)
	}
	if result.Summary.Total == 0 {
		t.Fatalf("expected validation to include quality/simulation feedback")
	}
	coverage, ok := result.Meta["coverage"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected coverage meta: %#v", result.Meta)
	}
	if coverage["target"] != "outline" || coverage["scope"] != "volume" || coverage["simulation_phase"] != "outline" ||
		coverage["simulation_backend"] != "in_memory_model_adapter" ||
		coverage["invokes_llm"] != false ||
		coverage["uses_derived_rpg_files"] != false ||
		coverage["refresh_required_before_simulation"] != false {
		t.Fatalf("unexpected outline validation coverage: %#v", coverage)
	}
}

func TestToolOutlinePatchResultExposesValidationCoverage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "story", "setup"), 0755); err != nil {
		t.Fatal(err)
	}
	setup := &models.StorySetup{ProjectName: "Patch Test", Theme: "cause and cost"}
	if err := setup.Save(filepath.Join(root, "story", "setup", "story_setup.json")); err != nil {
		t.Fatal(err)
	}
	if err := writeToolTestOutline(root, validToolPatchTestOutline()); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()

	originalFlags := toolPatchFlags
	defer func() { toolPatchFlags = originalFlags }()
	toolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{
		Target:    "volume",
		ID:        "P1-V1",
		PatchJSON: `{"summary":"Volume arc with a clearer causal spine."}`,
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runToolPatch(cmd, []string{"outline"}); err != nil {
		t.Fatalf("outline patch dry-run: %v", err)
	}
	result := decodeToolPatchResult(t, out.String())
	validation, ok := result.Meta["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing validation meta: %#v", result.Meta)
	}
	if validation["kind"] != "all" || validation["target"] != "outline" || validation["scope"] != "volume" ||
		validation["id"] != "P1-V1" {
		t.Fatalf("unexpected validation identity: %#v", validation)
	}
	coverage, ok := validation["coverage"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing validation coverage: %#v", validation)
	}
	if coverage["simulation_backend"] != "in_memory_model_adapter" ||
		coverage["simulation_phase"] != "outline" ||
		coverage["invokes_llm"] != false ||
		coverage["uses_derived_rpg_files"] != false ||
		coverage["refresh_required_before_simulation"] != false {
		t.Fatalf("unexpected validation coverage: %#v", coverage)
	}
}

func TestToolOutlinePatchRejectsNewGlobalBlockingIssueOnDryRun(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"project_name":"Patch Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "story", "setup"), 0755); err != nil {
		t.Fatal(err)
	}
	setup := &models.StorySetup{ProjectName: "Patch Test", Theme: "cause and cost"}
	if err := setup.Save(filepath.Join(root, "story", "setup", "story_setup.json")); err != nil {
		t.Fatal(err)
	}
	outline := validToolPatchTestOutline()
	volume := &outline.Parts[0].Volumes[0]
	volume.Chapters[0].Mysteries.Planted = []models.MysteryPlanted{{
		ID:      "myst_old_barrier",
		Clue:    "The old barrier still glows.",
		Horizon: "next_volume",
	}}
	second := volume.Chapters[0]
	second.ID = "P1-V1-C2"
	second.Title = "Second"
	second.Mysteries = models.ChapterMysteries{}
	volume.Chapters = append(volume.Chapters, second)
	if err := writeToolTestOutline(root, outline); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()

	originalFlags := toolPatchFlags
	defer func() { toolPatchFlags = originalFlags }()
	toolPatchFlags = struct {
		Target         string
		ID             string
		PatchJSON      string
		PatchBuffer    string
		Task           string
		Apply          bool
		DryRun         bool
		RefreshDerived bool
		TargetWords    int
	}{
		Target:    "volume",
		ID:        "P1-V1",
		PatchJSON: `{"changed_chapters":[{"id":"P1-V1-C2","mysteries":{"planted":[{"id":"myst_old_barrier","clue":"Repeated clue"}]}}]}`,
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err = runToolPatch(cmd, []string{"outline"})
	if err == nil || !strings.Contains(err.Error(), "global outline check introduced") || !strings.Contains(err.Error(), "myst_old_barrier") {
		t.Fatalf("expected global blocking rejection, got %v output=%s", err, out.String())
	}
}

func TestRunToolOutlineCheckUsesGlobalSimulationContextForScopedMysteries(t *testing.T) {
	setup := &models.StorySetup{ProjectName: "Patch Test", Theme: "cause and cost"}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "Plant",
				Summary: "Plant a cross-volume mystery.",
				Mysteries: models.ChapterMysteries{Planted: []models.MysteryPlanted{{
					ID:   "myst_cross_volume",
					Clue: "A signal appears.",
				}}},
			}},
		}, {
			ID: "P1-V2",
			Chapters: []models.Chapter{{
				ID:      "P1-V2-C1",
				Title:   "Resolve",
				Summary: "Resolve the cross-volume mystery.",
				Mysteries: models.ChapterMysteries{Resolved: []models.MysteryResolved{{
					ID:         "myst_cross_volume",
					Resolution: "The signal came from the old relay.",
				}}},
			}},
		}},
	}}}

	check, err := runToolOutlineCheck("all", setup, outline, "volume", "P1-V2")
	if err != nil {
		t.Fatalf("run scoped check: %v", err)
	}
	for _, issue := range check.Issues {
		if normalizeKey(issue.Category) == "mysteries" && strings.Contains(issue.Issue, "从未被planted") {
			t.Fatalf("scoped check should use global mystery context, got issue: %#v", issue)
		}
	}
}

func TestOutlinePatchPostCheckCommandScopesTarget(t *testing.T) {
	if got := outlinePatchPostCheckCommand("volume", "P1-V1"); got != `novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12` {
		t.Fatalf("volume post check = %q", got)
	}
	if got := outlinePatchPostCheckCommand("chapter", "P1-V1-C1"); got != `novelgen tool check all --target outline --scope chapter --id "P1-V1-C1" --min-priority low --max-issues 8` {
		t.Fatalf("chapter post check = %q", got)
	}
	if got := outlinePatchPostCheckCommand("all", ""); got != "" {
		t.Fatalf("unexpected post check = %q", got)
	}
}

func TestApplyChapterPatchPreservesIDAndOverlaysFields(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := []byte(`{"id":"P1-V1-C1","summary":"new summary","characters":["Lin","Mira"]}`)

	changes, err := applyChapterPatch(outline, "P1-V1-C1", patch)
	if err != nil {
		t.Fatal(err)
	}
	chapter := outline.GetChapterByID("P1-V1-C1")
	if chapter.ID != "P1-V1-C1" {
		t.Fatalf("chapter id changed to %q", chapter.ID)
	}
	if chapter.Summary != "new summary" || len(chapter.Characters) != 2 {
		t.Fatalf("patch not applied: %#v", chapter)
	}
	if !hasPatchChange(changes, "outline.chapter.P1-V1-C1.summary") {
		t.Fatalf("summary change not reported: %#v", changes)
	}
}

func TestApplyChapterPatchRejectsIDMismatch(t *testing.T) {
	outline := validToolPatchTestOutline()
	_, err := applyChapterPatch(outline, "P1-V1-C1", []byte(`{"id":"P1-V1-C2","summary":"bad"}`))
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected id mismatch error, got %v", err)
	}
}

func TestApplyChapterPatchRejectsGarbledPatchText(t *testing.T) {
	err := validateToolPatchBytes([]byte("{\"summary\":\"\ufffd\ufffd\ufffd????\"}"))
	if err == nil || !strings.Contains(err.Error(), "suspicious") {
		t.Fatalf("expected garbled text rejection, got %v", err)
	}
}

func TestValidateToolChapterPatchLengthRejectsHardOvershoot(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	okContent := "# Opening\n\n" + strings.Repeat("字", 3600)
	if err := validateToolChapterPatchLength(chapter, okContent, 3000); err != nil {
		t.Fatalf("expected hard max boundary to pass, got %v", err)
	}
	tooLong := "# Opening\n\n" + strings.Repeat("字", 3601)
	err := validateToolChapterPatchLength(chapter, tooLong, 3000)
	if err == nil || !strings.Contains(err.Error(), "too long") || !strings.Contains(err.Error(), "hard max 3600") {
		t.Fatalf("expected hard overshoot error, got %v", err)
	}
}

func TestRunToolChapterQualityGateFlagsTargetOvershoot(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	content := "# Opening\n\n" + strings.Repeat("字", 1501)

	gate := runToolChapterQualityGate(chapter, content, 1200)
	if len(gate.Suggestions) == 0 {
		t.Fatal("expected target overshoot suggestion")
	}
	found := false
	for _, suggestion := range gate.Suggestions {
		if suggestion.Category == "length" && strings.Contains(suggestion.Issue, "much longer than target") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected length overshoot suggestion, got %#v", gate.Suggestions)
	}
}

func TestValidateToolChapterPatchLengthRejectsOverShrink(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C2", Title: "出招"}
	previous := "# 出招\n\n" + strings.Repeat("旧", 1300)
	tooShort := "# 出招\n\n" + strings.Repeat("新", 700)

	err := validateToolChapterPatchLength(chapter, tooShort, 1200, previous)
	if err == nil || !strings.Contains(err.Error(), "removes too much existing prose") || !strings.Contains(err.Error(), "minimum retained 1105") {
		t.Fatalf("expected over-shrink error, got %v", err)
	}

	ok := "# 出招\n\n" + strings.Repeat("新", 1110)
	if err := validateToolChapterPatchLength(chapter, ok, 1200, previous); err != nil {
		t.Fatalf("expected minimal shrink to pass, got %v", err)
	}
}

func TestValidateToolChapterPatchLengthAllowsTrimmingOversizedPrevious(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C2", Title: "出招"}
	previous := "# 出招\n\n" + strings.Repeat("旧", 1900)
	trimmed := "# 出招\n\n" + strings.Repeat("新", 1200)

	if err := validateToolChapterPatchLength(chapter, trimmed, 1200, previous); err != nil {
		t.Fatalf("expected oversized previous content to allow trimming, got %v", err)
	}
}

func TestValidateToolChapterPatchLengthRejectsParagraphCollapse(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C4", Title: "反杀"}
	previous := "# 反杀\n\n" + strings.Join([]string{
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
	}, "\n\n")
	collapsed := "# 反杀\n\n" + strings.Repeat("新", 1000)

	err := validateToolChapterPatchLength(chapter, collapsed, 1200, previous)
	if err == nil || !strings.Contains(err.Error(), "collapses chapter") {
		t.Fatalf("expected paragraph collapse error, got %v", err)
	}

	readable := "# 反杀\n\n" + strings.Join([]string{
		strings.Repeat("新", 230),
		strings.Repeat("新", 230),
		strings.Repeat("新", 230),
		strings.Repeat("新", 230),
	}, "\n\n")
	if err := validateToolChapterPatchLength(chapter, readable, 1200, previous); err != nil {
		t.Fatalf("expected readable paragraph patch to pass, got %v", err)
	}
}

func TestRunToolChapterQualityGateFlagsCollapsedParagraphs(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C4", Title: "反杀"}
	content := "# 反杀\n\n" + strings.Repeat("这一段连续推进剧情没有空行也没有段落拆分。", 80)

	gate := runToolChapterQualityGate(chapter, content, 1200)

	if !qualityGateHasIssue(gate, "structure", models.PriorityMedium) {
		t.Fatalf("expected collapsed paragraph issue, got %+v", gate.Suggestions)
	}
}

func TestApplyVolumePatchRejectsWholeChapterReplacement(t *testing.T) {
	outline := validToolPatchTestOutline()
	_, err := applyVolumePatch(outline, "P1-V1", []byte(`{"chapters":[]}`))
	if err == nil || !strings.Contains(err.Error(), "changed_chapters") {
		t.Fatalf("expected changed_chapters error, got %v", err)
	}
}

func TestApplyVolumePatchChangedChapters(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := map[string]interface{}{
		"id":    "P1-V1",
		"title": "New Volume",
		"changed_chapters": []map[string]interface{}{{
			"id":       "P1-V1-C1",
			"conflict": "new conflict",
		}},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := applyVolumePatch(outline, "P1-V1", data)
	if err != nil {
		t.Fatal(err)
	}
	if got := outline.GetVolumeByID("P1-V1").Title; got != "New Volume" {
		t.Fatalf("volume title = %q", got)
	}
	if got := outline.GetChapterByID("P1-V1-C1").Conflict; got != "new conflict" {
		t.Fatalf("chapter conflict = %q", got)
	}
	if !hasPatchChange(changes, "outline.volume.P1-V1.title") ||
		!hasPatchChange(changes, "outline.chapter.P1-V1-C1.conflict") {
		t.Fatalf("expected volume and chapter changes, got %#v", changes)
	}
}

func TestApplyVolumePatchMergesPayoffContractFields(t *testing.T) {
	outline := validToolPatchTestOutline()
	volume := outline.GetVolumeByID("P1-V1")
	volume.PayoffContract = &models.VolumePayoffContract{
		VolumeQuestion:      "Can Lin survive the first signal?",
		PowerPromise:        "Signal reading creates tactical advantage.",
		MainOpponentMisread: "The rival thinks Lin is blind.",
		VisibleReward:       "Lin gains a map.",
		ReputationShift:     "Lin becomes credible.",
		NextBiggerGame:      "The war expands.",
	}

	changes, err := applyVolumePatch(outline, "P1-V1", []byte(`{"payoff_contract":{"big_win":"Lin wins the first public duel."}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := outline.GetVolumeByID("P1-V1").PayoffContract
	if got == nil {
		t.Fatal("payoff_contract was removed")
	}
	if got.BigWin != "Lin wins the first public duel." ||
		got.VolumeQuestion != "Can Lin survive the first signal?" ||
		got.PowerPromise != "Signal reading creates tactical advantage." ||
		got.MainOpponentMisread != "The rival thinks Lin is blind." ||
		got.VisibleReward != "Lin gains a map." ||
		got.ReputationShift != "Lin becomes credible." ||
		got.NextBiggerGame != "The war expands." {
		t.Fatalf("payoff_contract should be field-merged: %#v", got)
	}
	if !hasPatchChange(changes, "outline.volume.P1-V1.payoff_contract") {
		t.Fatalf("payoff_contract change not reported: %#v", changes)
	}
}

func TestApplyVolumePatchChangedEvents(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := []byte(`{"changed_events":[{"chapter_id":"P1-V1-C1","event_index":1,"type":"plan","action":"plan","details":"Lin chooses the safer route."}]}`)
	changes, err := applyVolumePatch(outline, "P1-V1", patch)
	if err != nil {
		t.Fatal(err)
	}
	events := outline.GetChapterByID("P1-V1-C1").Events
	if len(events) != 3 {
		t.Fatalf("event array length changed: %#v", events)
	}
	if events[1].Type != "plan" || events[1].Action != "plan" || events[1].Details != "Lin chooses the safer route." {
		t.Fatalf("target event not patched: %#v", events[1])
	}
	if events[1].Target != "Camp" || events[0].Target != "Star Core" || events[2].Target != "Signal" {
		t.Fatalf("event patch should preserve target event fields and untouched events: %#v", events)
	}
	if !hasPatchChange(changes, "outline.event.P1-V1-C1.1.type") ||
		!hasPatchChange(changes, "outline.event.P1-V1-C1.1.action") {
		t.Fatalf("event changes not reported: %#v", changes)
	}
}

func TestApplyVolumePatchDeepMergesNestedChapterObjects(t *testing.T) {
	outline := validToolPatchTestOutline()
	chapter := outline.GetChapterByID("P1-V1-C1")
	chapter.ChapterPayoff = &models.ChapterPayoff{
		Desire:       "old desire",
		Pressure:     "old pressure",
		CleverMove:   "old clever move",
		PayoffMoment: "old payoff",
		Reward:       "old reward",
		SocialProof:  "old social proof",
		Hook:         "old hook",
	}

	patch := []byte(`{"changed_chapters":[{"id":"P1-V1-C1","chapter_payoff":{"desire":"new desire","hook":"new hook"}}]}`)
	changes, err := applyVolumePatch(outline, "P1-V1", patch)
	if err != nil {
		t.Fatal(err)
	}
	got := outline.GetChapterByID("P1-V1-C1").ChapterPayoff
	if got == nil {
		t.Fatal("chapter payoff was removed")
	}
	if got.Desire != "new desire" || got.Hook != "new hook" {
		t.Fatalf("patch fields not applied: %#v", got)
	}
	if got.Pressure != "old pressure" ||
		got.CleverMove != "old clever move" ||
		got.PayoffMoment != "old payoff" ||
		got.Reward != "old reward" ||
		got.SocialProof != "old social proof" {
		t.Fatalf("nested payoff fields should be preserved by deep merge: %#v", got)
	}
	if !hasPatchChange(changes, "outline.chapter.P1-V1-C1.chapter_payoff") {
		t.Fatalf("chapter payoff change not reported: %#v", changes)
	}
}

func TestApplyChapterPatchStillReplacesArrays(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := []byte(`{"events":[{"actor":"Lin","action":"discover","target":"Door","target_type":"knowledge"}]}`)

	if _, err := applyChapterPatch(outline, "P1-V1-C1", patch); err != nil {
		t.Fatal(err)
	}
	chapter := outline.GetChapterByID("P1-V1-C1")
	if len(chapter.Events) != 1 || chapter.Events[0].Target != "Door" {
		t.Fatalf("arrays should be replaced, not appended/deep-merged: %#v", chapter.Events)
	}
}

func TestApplyCraftElementPatchPreservesRawFieldsAndNormalizes(t *testing.T) {
	values := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
			"character_arc": "learns the cost of power",
			"power_level":   3,
		},
	}
	patch := []byte(`{"notes":"updated","power_level":-5,"dsl_tags":["lead","lead"]}`)

	key, action, changes, err := applyCraftElementPatch(values, "character", "Lin", patch)
	if err != nil {
		t.Fatal(err)
	}
	if key != "Lin" || action != "update" {
		t.Fatalf("key/action = %q/%q, want Lin/update", key, action)
	}
	got := values["Lin"]
	if got["character_arc"] != "learns the cost of power" {
		t.Fatalf("raw unknown field was not preserved: %#v", got)
	}
	if got["power_level"] != 0 {
		t.Fatalf("power_level = %#v, want normalized 0", got["power_level"])
	}
	tags, ok := got["dsl_tags"].([]interface{})
	if !ok || len(tags) != 1 || tags[0] != "lead" {
		t.Fatalf("dsl_tags not compacted: %#v", got["dsl_tags"])
	}
	if !hasPatchChange(changes, "craft.character.Lin.notes") ||
		!hasPatchChange(changes, "craft.character.Lin.power_level") {
		t.Fatalf("expected notes and power changes, got %#v", changes)
	}
}

func TestApplyCraftElementPatchCreatesElementWithName(t *testing.T) {
	values := map[string]map[string]interface{}{}
	key, action, _, err := applyCraftElementPatch(values, "item", "Star Core", []byte(`{"type":"artifact","description":"Ancient core","function":"stores power","significance":"main reward"}`))
	if err != nil {
		t.Fatal(err)
	}
	if key != "Star Core" || action != "create" {
		t.Fatalf("key/action = %q/%q, want Star Core/create", key, action)
	}
	if values["Star Core"]["name"] != "Star Core" {
		t.Fatalf("created item missing normalized name: %#v", values["Star Core"])
	}
}

func TestRunToolCraftPatchApplyWritesFileAndCheckpoint(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
		},
	}
	if err := saveRawCraftMap(filepath.Join(root, "story", "craft", "characters.json"), initial); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := runToolCraftPatch(cmd, root, "character", "Lin", []byte(`{"notes":"updated by tool"}`), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeToolPatchResult(t, out.String())
	if len(result.NextActions) != 1 ||
		result.NextActions[0].Action != "post_apply_check" ||
		!strings.Contains(result.NextActions[0].Command, `tool check schema --target craft --scope character --id "Lin"`) {
		t.Fatalf("craft apply should return schema check next action: %#v", result.NextActions)
	}

	data, err := os.ReadFile(filepath.Join(root, "story", "craft", "characters.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["Lin"]["notes"] != "updated by tool" {
		t.Fatalf("patch was not written: %#v", saved["Lin"])
	}
	checkpoints, err := filepath.Glob(filepath.Join(root, "story", "craft", "checkpoints", "characters_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint count = %d, want 1", len(checkpoints))
	}
}

func TestRunToolCraftSchemaCheckFlagsProtagonistFieldDistribution(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
			"notes":         strings.Repeat("tactical note ", 120),
		},
	}
	if err := saveRawCraftMap(filepath.Join(root, "story", "craft", "characters.json"), initial); err != nil {
		t.Fatal(err)
	}

	result, err := runToolCraftSchemaCheck(root, "character", "Lin")
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocking {
		t.Fatalf("craft quality suggestions should not block old data: %#v", result)
	}
	if result.Summary.Medium < 3 || result.Summary.Low < 1 {
		t.Fatalf("expected protagonist field distribution suggestions, got summary=%#v issues=%#v", result.Summary, result.Issues)
	}
	if !toolCheckHasIssue(result, "personality") ||
		!toolCheckHasIssue(result, "motivation") ||
		!toolCheckHasIssue(result, "skills or abilities") ||
		!toolCheckHasIssue(result, "notes") {
		t.Fatalf("missing expected craft quality issues: %#v", result.Issues)
	}
}

func TestRunToolCraftPatchDryRunReturnsCraftQualityCheck(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
		},
	}
	if err := saveRawCraftMap(filepath.Join(root, "story", "craft", "characters.json"), initial); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := runToolCraftPatch(cmd, root, "character", "Lin", []byte(`{"notes":"updated only"}`), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeToolPatchResult(t, out.String())
	if result.Check == nil || result.Check.Summary.Total == 0 {
		t.Fatalf("craft patch dry-run should return quality check issues: %#v", result.Check)
	}
	if result.Check.Blocking {
		t.Fatalf("craft quality suggestions should not block dry-run: %#v", result.Check)
	}
	if !toolCheckHasIssue(result.Check, "structured skills or abilities") {
		t.Fatalf("missing structured field issue: %#v", result.Check.Issues)
	}
}

func TestRunToolRecapCheckValidatesSavedRecap(t *testing.T) {
	root := t.TempDir()
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "醒来",
		Location:        "残骸",
		Time:            "同夜",
		Present:         []string{"林砚"},
		PlotBeats:       []string{"林砚醒来。"},
		LastLine:        "他推开舱门。",
		NextOpeningHint: "他推开舱门后，先看见蓝色火光。",
	}); err != nil {
		t.Fatal(err)
	}

	result, err := runToolRecapCheck(root, "quality", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Blocking || result.Summary.Total != 0 {
		t.Fatalf("recap check should pass: %#v", result)
	}
}

func TestRunToolRecapCheckReportsMissingRecap(t *testing.T) {
	result, err := runToolRecapCheck(t.TempDir(), "quality", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !result.Blocking || result.Summary.High != 1 {
		t.Fatalf("missing recap should block: %#v", result)
	}
	if !strings.Contains(result.Issues[0].Issue, "not available") {
		t.Fatalf("unexpected issue: %#v", result.Issues)
	}
}

func TestRunToolRecapCheckReportsMinimalAndConsistencyIssues(t *testing.T) {
	root := t.TempDir()
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "醒来",
		Location:        "",
		Present:         nil,
		LastLine:        "他推开舱门。",
		NextOpeningHint: "远处传来警报。",
	}); err != nil {
		t.Fatal(err)
	}

	result, err := runToolRecapCheck(root, "all", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !result.Blocking || result.Summary.High < 2 || result.Summary.Medium == 0 {
		t.Fatalf("recap check should report high minimal and medium consistency issues: %#v", result)
	}
}

func TestRunToolRecapCheckReportsOutlineTitleMismatch(t *testing.T) {
	root := t.TempDir()
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "旧标题",
		Location:        "残骸",
		Present:         []string{"林砚"},
		PlotBeats:       []string{"林砚醒来。"},
		LastLine:        "他推开舱门。",
		NextOpeningHint: "他推开舱门后，先看见蓝色火光。",
	}); err != nil {
		t.Fatal(err)
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:    "P1-V1-C1",
				Title: "新标题",
			}},
		}},
	}}}
	if err := os.MkdirAll(filepath.Join(root, "story", "compose"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := outline.Save(filepath.Join(root, "story", "compose", "outline.json")); err != nil {
		t.Fatal(err)
	}

	result, err := runToolRecapCheck(root, "quality", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocking || result.Summary.Medium != 1 {
		t.Fatalf("title mismatch should be non-blocking medium issue: %#v", result)
	}
	if !strings.Contains(result.Issues[0].Issue, "does not match outline title") {
		t.Fatalf("unexpected issue: %#v", result.Issues)
	}
}

func TestRunToolRecapPatchApplyWritesValidatedFileAndCheckpoint(t *testing.T) {
	root := t.TempDir()
	chapterID := "P1-V1-C1"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       chapterID,
		Title:           "Old title",
		Location:        "Old place",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Old beat"},
		LastLine:        "Old last line.",
		NextOpeningHint: "Old last line opens the next scene.",
	}); err != nil {
		t.Fatal(err)
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:    chapterID,
				Title: "Outline title",
			}},
		}},
	}}}
	if err := os.MkdirAll(filepath.Join(root, "story", "compose"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := outline.Save(filepath.Join(root, "story", "compose", "outline.json")); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := runToolRecapPatch(cmd, root, chapterID, []byte(`{
		"chapter_id":"WRONG",
		"title":"Wrong title",
		"location":"New place",
		"present":["Lin","Mira"],
		"plot_beats":["New beat"],
		"last_line":"Lin opens the sealed door.",
		"next_opening_hint":"Lin opens the sealed door and Mira sees blue light."
	}`), true, nil)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := recap.NewStore(root).Load(chapterID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ChapterID != chapterID || saved.Title != "Outline title" {
		t.Fatalf("recap identity not fixed by Go: %#v", saved)
	}
	if saved.Location != "New place" || len(saved.Present) != 2 || saved.PlotBeats[0] != "New beat" {
		t.Fatalf("recap patch not applied: %#v", saved)
	}
	checkpoints, err := filepath.Glob(filepath.Join(root, "story", "recaps", "checkpoints", chapterID+"_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint count = %d, want 1", len(checkpoints))
	}
	output := out.String()
	if !strings.Contains(output, `"target": "recap.chapter"`) ||
		!strings.Contains(output, "ignored patch chapter_id") ||
		!strings.Contains(output, "ignored patch title") {
		t.Fatalf("unexpected recap patch output: %s", output)
	}
	result := decodeToolPatchResult(t, output)
	if len(result.NextActions) != 1 ||
		result.NextActions[0].Action != "post_apply_check" ||
		!strings.Contains(result.NextActions[0].Command, `tool check quality --target recap --scope chapter --id "P1-V1-C1"`) {
		t.Fatalf("recap apply should return recap check next action: %#v", result.NextActions)
	}
}

func TestRunToolRecapPatchRejectsBlockingApply(t *testing.T) {
	root := t.TempDir()
	chapterID := "P1-V1-C1"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       chapterID,
		Title:           "Opening",
		Location:        "Mine",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Lin wakes."},
		LastLine:        "Lin opens the sealed door.",
		NextOpeningHint: "Lin opens the sealed door and sees blue light.",
	}); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	err := runToolRecapPatch(cmd, root, chapterID, []byte(`{"location":"","present":[]}`), true, nil)
	if err == nil || !strings.Contains(err.Error(), "patch rejected") {
		t.Fatalf("expected blocking recap patch rejection, got %v", err)
	}
	saved, err := recap.NewStore(root).Load(chapterID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Location != "Mine" || len(saved.Present) != 1 {
		t.Fatalf("blocking apply should not write recap: %#v", saved)
	}
}

func TestRunToolSetupCheckAllMergesQualityAndSimulation(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Thin",
	}
	result, err := runToolSetupCheck("all", setup)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "all" || result.Target != "setup" || result.Scope != "all" {
		t.Fatalf("unexpected setup check identity: %#v", result)
	}
	if result.OK || !result.Blocking {
		t.Fatalf("thin setup should have blocking issues: %#v", result.Summary)
	}
	if result.Summary.Total == 0 {
		t.Fatalf("expected setup check issues")
	}
}

func TestRunToolSetupPatchApplyWritesFilesAndCheckpoint(t *testing.T) {
	root := t.TempDir()
	setup := validLongFormSetup()
	if err := os.MkdirAll(filepath.Join(root, "story", "setup"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := setup.Save(filepath.Join(root, "story", "setup", "story_setup.json")); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := runToolSetupPatch(cmd, root, []byte(`{"theme":"updated by setup patch"}`), true, nil)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := models.LoadStorySetup(filepath.Join(root, "story", "setup", "story_setup.json"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Theme != "updated by setup patch" {
		t.Fatalf("setup patch was not written: %q", saved.Theme)
	}
	if _, err := os.Stat(filepath.Join(root, "story", "setup", "story_setup.md")); err != nil {
		t.Fatalf("setup markdown was not written: %v", err)
	}
	checkpoints, err := filepath.Glob(filepath.Join(root, "story", "setup", "checkpoints", "story_setup_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint count = %d, want 1", len(checkpoints))
	}
	if !strings.Contains(out.String(), `"applied": true`) || !strings.Contains(out.String(), `"target": "setup"`) {
		t.Fatalf("unexpected setup patch output: %s", out.String())
	}
	result := decodeToolPatchResult(t, out.String())
	if len(result.NextActions) != 1 ||
		result.NextActions[0].Action != "post_apply_check" ||
		!strings.Contains(result.NextActions[0].Command, "tool check all --target setup") {
		t.Fatalf("setup apply should return setup check next action: %#v", result.NextActions)
	}
}

func TestApplySetupPatchObjectUpsertsPremises(t *testing.T) {
	setup := *validLongFormSetup()
	setup.Premises = []models.Premise{{
		Name:        "Ranking System",
		Category:    "progression",
		Description: "public rank ladder",
		Progression: []models.ProgressionStage{{
			Level:       1,
			Name:        "Outer Rank",
			Description: "entry rank",
		}},
	}}

	merged, warnings, err := applySetupPatchObject(setup, []byte(`{
		"premises": [{
			"name": "Zerg Tier Ladder",
			"category": "faction",
			"description": "Zerg ranks progress from drone to soldier to captain."
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "upsert") {
		t.Fatalf("expected upsert warning, got %#v", warnings)
	}
	if len(merged.Premises) != 2 {
		t.Fatalf("premises len = %d, want 2: %#v", len(merged.Premises), merged.Premises)
	}
	if merged.Premises[0].Name != "Ranking System" || merged.Premises[0].Progression[0].Name != "Outer Rank" {
		t.Fatalf("existing premise was not preserved: %#v", merged.Premises[0])
	}
	if merged.Premises[1].Name != "Zerg Tier Ladder" || merged.Premises[1].Category != "faction" {
		t.Fatalf("new premise was not appended: %#v", merged.Premises[1])
	}

	merged, _, err = applySetupPatchObject(merged, []byte(`{
		"premises": [{
			"name": "Ranking System",
			"description": "updated public ladder"
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Premises) != 2 {
		t.Fatalf("update should not append duplicate premise: %#v", merged.Premises)
	}
	if merged.Premises[0].Description != "updated public ladder" ||
		merged.Premises[0].Category != "progression" ||
		merged.Premises[0].Progression[0].Name != "Outer Rank" {
		t.Fatalf("existing premise should be field-merged: %#v", merged.Premises[0])
	}
}

func TestApplySetupPatchObjectMergesNestedPlanAndStringLists(t *testing.T) {
	setup := *validLongFormSetup()
	setup.Rules = []string{"Existing rule"}
	setup.LongFormPlan = &models.LongFormPlan{
		TargetChapters:   1000,
		TargetVolumes:    10,
		MainLoop:         "old loop",
		EscalationLadder: []string{"frontier", "city"},
		ReaderPromises:   []string{"visible wins"},
		PayoffCadence:    "old cadence",
		VolumePattern:    []string{"hook", "win"},
		EndgamePromise:   "old endgame",
	}

	merged, warnings, err := applySetupPatchObject(setup, []byte(`{
		"rules": ["Existing rule", "New faction rule"],
		"long_form_plan": {
			"main_loop": "new loop",
			"escalation_ladder": ["city", "region"],
			"reader_promises": ["visible wins", "territory growth"]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "rules") || !strings.Contains(warnings[0], "long_form_plan") {
		t.Fatalf("expected incremental merge warning, got %#v", warnings)
	}
	if len(merged.Rules) != 2 || merged.Rules[0] != "Existing rule" || merged.Rules[1] != "New faction rule" {
		t.Fatalf("rules should append/dedupe, got %#v", merged.Rules)
	}
	if merged.LongFormPlan == nil {
		t.Fatal("long_form_plan should remain present")
	}
	if merged.LongFormPlan.MainLoop != "new loop" ||
		merged.LongFormPlan.TargetChapters != 1000 ||
		merged.LongFormPlan.TargetVolumes != 10 ||
		merged.LongFormPlan.PayoffCadence != "old cadence" ||
		merged.LongFormPlan.EndgamePromise != "old endgame" {
		t.Fatalf("long_form_plan scalar fields were not merged safely: %#v", merged.LongFormPlan)
	}
	if strings.Join(merged.LongFormPlan.EscalationLadder, "|") != "frontier|city|region" {
		t.Fatalf("escalation ladder should append/dedupe: %#v", merged.LongFormPlan.EscalationLadder)
	}
	if strings.Join(merged.LongFormPlan.ReaderPromises, "|") != "visible wins|territory growth" {
		t.Fatalf("reader promises should append/dedupe: %#v", merged.LongFormPlan.ReaderPromises)
	}
	if strings.Join(merged.LongFormPlan.VolumePattern, "|") != "hook|win" {
		t.Fatalf("untouched nested arrays should be preserved: %#v", merged.LongFormPlan.VolumePattern)
	}
}

func TestApplySetupPatchObjectPatchesStringListByIndex(t *testing.T) {
	setup := *validLongFormSetup()
	setup.Genres = []string{"sci-fi", "progression"}
	setup.Rules = []string{"short rule", "this rule is far too verbose and needs to be compacted"}

	merged, warnings, err := applySetupPatchObject(setup, []byte(`{
		"genres_patch": [{"index": 1, "value": "mecha progression"}],
		"rules_patch": [{"index": 1, "value": "Compact rule."}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "genres_patch") || !strings.Contains(warnings[0], "rules_patch") {
		t.Fatalf("expected patch warning, got %#v", warnings)
	}
	if strings.Join(merged.Genres, "|") != "sci-fi|mecha progression" {
		t.Fatalf("genres patch did not replace by index: %#v", merged.Genres)
	}
	if strings.Join(merged.Rules, "|") != "short rule|Compact rule." {
		t.Fatalf("rules patch did not replace by index: %#v", merged.Rules)
	}
}

func TestApplySetupPatchObjectRejectsStringListPatchOutOfRange(t *testing.T) {
	setup := *validLongFormSetup()
	setup.Rules = []string{"short rule"}

	_, _, err := applySetupPatchObject(setup, []byte(`{"rules_patch":[{"index":2,"value":"nope"}]}`))
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected out-of-range error, got %v", err)
	}
}

func TestRunToolPatchSetupDoesNotRequireID(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"title":"Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	setup := validLongFormSetup()
	if err := os.MkdirAll(filepath.Join(root, "story", "setup"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := setup.Save(filepath.Join(root, "story", "setup", "story_setup.json")); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
		toolPatchFlags = struct {
			Target         string
			ID             string
			PatchJSON      string
			PatchBuffer    string
			Task           string
			Apply          bool
			DryRun         bool
			RefreshDerived bool
			TargetWords    int
		}{}
	})

	toolPatchFlags.PatchJSON = `{"tone":"dry-run setup tone"}`
	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	if err := runToolPatch(cmd, []string{"setup"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) || !strings.Contains(out.String(), "setup.tone") {
		t.Fatalf("unexpected setup dry-run output: %s", out.String())
	}
	result := decodeToolPatchResult(t, out.String())
	if len(result.NextActions) == 0 || result.NextActions[0].Action != "apply_validated_patch" {
		t.Fatalf("setup dry-run should return apply next action: %#v", result.NextActions)
	}
	reloaded, err := models.LoadStorySetup(filepath.Join(root, "story", "setup", "story_setup.json"))
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Tone == "dry-run setup tone" {
		t.Fatalf("dry-run setup patch should not write file")
	}
}

func TestRunToolPatchStripsBOMFromStdinPatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"title":"Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
		},
	}
	if err := saveRawCraftMap(filepath.Join(root, "story", "craft", "characters.json"), initial); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
		toolPatchFlags = struct {
			Target         string
			ID             string
			PatchJSON      string
			PatchBuffer    string
			Task           string
			Apply          bool
			DryRun         bool
			RefreshDerived bool
			TargetWords    int
		}{}
	})

	toolPatchFlags.Target = "character"
	toolPatchFlags.ID = "Lin"
	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader(string([]byte{0xEF, 0xBB, 0xBF}) + `{"notes":"stdin patch"}`))

	if err := runToolPatch(cmd, []string{"craft"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) || !strings.Contains(out.String(), "craft.character.Lin.notes") {
		t.Fatalf("unexpected dry-run output: %s", out.String())
	}
}

func TestRunToolCraftSchemaCheckValidatesSavedCharacter(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"appearance":    "lean pilot",
			"personality":   []interface{}{"focused", "risk-aware", "dry humor"},
			"background":    "old world survivor",
			"motivation":    "survive",
			"skills":        []interface{}{"route planning"},
			"abilities":     []interface{}{"system log reading with blind spots"},
			"role_in_story": "lead",
			"voice":         "short, observant, lightly sarcastic",
			"rpg_stats": map[string]interface{}{
				"str": float64(6),
				"agi": float64(5),
			},
		},
	}
	if err := saveRawCraftMap(filepath.Join(root, "story", "craft", "characters.json"), initial); err != nil {
		t.Fatal(err)
	}

	result, err := runToolCraftSchemaCheck(root, "character", "Lin")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Blocking || result.Summary.Total != 0 {
		t.Fatalf("schema check should pass: %#v", result)
	}
}

func TestRunToolCraftSchemaCheckReportsInvalidSavedCharacter(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"appearance":    "lean pilot",
			"personality":   []interface{}{"focused"},
			"background":    "old world survivor",
			"motivation":    "survive",
			"role_in_story": "lead",
			"rpg_stats": map[string]interface{}{
				"perception": float64(6),
			},
		},
	}
	if err := saveRawCraftMap(filepath.Join(root, "story", "craft", "characters.json"), initial); err != nil {
		t.Fatal(err)
	}

	result, err := runToolCraftSchemaCheck(root, "character", "Lin")
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !result.Blocking || result.Summary.High != 1 {
		t.Fatalf("schema check should fail: %#v", result)
	}
	if !strings.Contains(result.Issues[0].Issue, "perception") {
		t.Fatalf("schema issue should mention invalid key: %#v", result.Issues)
	}
}

func TestApplyCraftElementPatchRejectsSchemaMismatch(t *testing.T) {
	values := map[string]map[string]interface{}{
		"Lin": {"name": "Lin", "role_in_story": "protagonist"},
	}
	_, _, _, err := applyCraftElementPatch(values, "character", "Lin", []byte(`{"personality":"should be array"}`))
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("expected schema mismatch, got %v", err)
	}
}

func validToolPatchTestOutline() *models.Outline {
	return &models.Outline{Parts: []models.Part{{
		ID:      "P1",
		Title:   "Part One",
		Summary: "Part arc",
		Volumes: []models.Volume{{
			ID:      "P1-V1",
			Title:   "Volume One",
			Summary: "Volume arc",
			Chapters: []models.Chapter{{
				ID:         "P1-V1-C1",
				Title:      "Opening",
				Summary:    "Lin enters the mine and finds a star core.",
				Characters: []string{"Lin"},
				Location:   "Mine",
				Events: []models.Event{{
					Actor:      "Lin",
					Action:     models.ActionAcquire,
					Target:     "Star Core",
					TargetType: models.TargetTypeItem,
				}, {
					Actor:      "Lin",
					Action:     models.ActionMove,
					Target:     "Camp",
					TargetType: models.TargetTypeLocation,
				}, {
					Actor:      "Lin",
					Action:     models.ActionDiscover,
					Target:     "Signal",
					TargetType: models.TargetTypeKnowledge,
				}},
				StateChange: "Lin gains a star core and a new lead.",
				Conflict:    "The mine is unstable.",
				Pacing:      "normal",
				Timeline:    models.ChapterTimeline{Anchor: "Day 1"},
				StateAnchor: models.StateAnchor{Location: "Mine"},
				Scenes: []models.OutlineScene{{
					Order: 1, POV: "Lin", Goal: "Find supplies", Beats: []string{"Search the mine"},
				}, {
					Order: 2, POV: "Lin", Goal: "Escape", Beats: []string{"Follow the signal"},
				}},
			}},
		}},
	}}}
}

func hasPatchChange(changes []toolPatchChange, path string) bool {
	for _, change := range changes {
		if change.Path == path {
			return true
		}
	}
	return false
}

func saveTestRecap(root string, value models.ChapterRecap) error {
	dir := filepath.Join(root, "story", "recaps")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, value.ChapterID+".json"), data, 0644)
}
