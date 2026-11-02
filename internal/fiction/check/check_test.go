package check

import (
	"encoding/json"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/quality"
)

func testIssue(category, priority string) models.ReviewSuggestion {
	return models.ReviewSuggestion{
		Category:   category,
		TargetID:   "P1-V1-C1",
		TargetName: "chapter one",
		Issue:      "issue in " + category,
		Suggestion: "fix " + category,
		Priority:   priority,
	}
}

func TestApplyToolCheckIssueFiltersCombinesPriorityCategoryAndLimit(t *testing.T) {
	result := &ToolCheckResult{Issues: []models.ReviewSuggestion{
		testIssue("logic", models.PriorityCritical),
		testIssue("logic", models.PriorityMedium),
		testIssue("plot", models.PriorityHigh),
		testIssue("pacing", models.PriorityLow),
	}}

	if err := ApplyToolCheckIssueFilters(result, "high", "logic", 0); err != nil {
		t.Fatalf("ApplyToolCheckIssueFilters() error = %v", err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Priority != models.PriorityCritical {
		t.Fatalf("high+logic issues = %#v", result.Issues)
	}

	result = &ToolCheckResult{Issues: []models.ReviewSuggestion{
		testIssue("logic", models.PriorityCritical),
		testIssue("logic", models.PriorityHigh),
	}}
	if err := ApplyToolCheckIssueFilters(result, "", "", 1); err != nil {
		t.Fatalf("max issues filter error = %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("max_issues did not cap the list: %#v", result.Issues)
	}
	filter, ok := result.Meta["issue_filter"].(map[string]interface{})
	if !ok {
		t.Fatalf("issue_filter meta missing: %#v", result.Meta)
	}
	if filter["original_issues"].(int) != 2 || filter["returned_issues"].(int) != 1 {
		t.Fatalf("issue_filter = %#v", filter)
	}
}

func TestApplyToolCheckIssueFiltersRejectsUnknownPriority(t *testing.T) {
	result := &ToolCheckResult{Issues: []models.ReviewSuggestion{testIssue("logic", models.PriorityHigh)}}
	if err := ApplyToolCheckIssueFilters(result, "urgent", "", 0); err == nil {
		t.Fatal("unknown priority should be an error")
	}
	if len(result.Issues) != 1 {
		t.Fatalf("failed filter must not mutate issues: %#v", result.Issues)
	}
}

func TestApplyToolCheckIssueFiltersMarksNoMatches(t *testing.T) {
	result := &ToolCheckResult{Issues: []models.ReviewSuggestion{testIssue("logic", models.PriorityLow)}}
	if err := ApplyToolCheckIssueFilters(result, "critical", "", 0); err != nil {
		t.Fatalf("filter error = %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("issues = %#v, want none", result.Issues)
	}
	meta := result.Meta["issue_filter"].(map[string]interface{})
	if meta["no_matches"] != true {
		t.Fatalf("no_matches = %#v", meta)
	}
	actions := ToolCheckNextActions(*result)
	if len(actions) == 0 || actions[0].Action != "fix_check_filter" {
		t.Fatalf("next actions = %#v, want fix_check_filter", actions)
	}
}

func TestMakeToolCheckResultCountsPrioritiesAndMarshalsNavigation(t *testing.T) {
	gate := quality.QualityGateResult{
		Blocking: true,
		Suggestions: []models.ReviewSuggestion{
			testIssue("logic", models.PriorityHigh),
			testIssue("plot", models.PriorityLow),
		},
	}
	result := MakeToolCheckResult("quality", "outline", "chapter", "P1-V1-C1", gate)
	if result.OK || !result.Blocking {
		t.Fatalf("OK/Blocking = %v/%v, want false/true", result.OK, result.Blocking)
	}
	if result.Summary.Total != 2 || result.Summary.High != 1 || result.Summary.Low != 1 {
		t.Fatalf("summary = %#v", result.Summary)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	var payload struct {
		Issues []struct {
			Category   string                 `json:"category"`
			Navigation map[string]interface{} `json:"navigation"`
		} `json:"issues"`
		NextActions []ToolNextAction       `json:"next_actions"`
		Meta        map[string]interface{} `json:"meta"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if len(payload.Issues) != 2 {
		t.Fatalf("issues = %#v", payload.Issues)
	}
	for _, issue := range payload.Issues {
		if issue.Navigation == nil || len(issue.Navigation) == 0 {
			t.Fatalf("issue %q has no navigation: %#v", issue.Category, issue)
		}
	}
	if len(payload.NextActions) == 0 {
		t.Fatal("next_actions missing from the check result")
	}
	budget, ok := payload.Meta["check_budget"].(map[string]interface{})
	if !ok || budget["max_repair_issues"] == nil {
		t.Fatalf("check_budget meta missing: %#v", payload.Meta)
	}
}

func TestToolCheckNextActionsForCleanResult(t *testing.T) {
	actions := ToolCheckNextActions(ToolCheckResult{Kind: "quality", Target: "outline", Scope: "chapter"})
	if len(actions) != 1 || actions[0].Action != "return_final_json" {
		t.Fatalf("actions = %#v, want return_final_json", actions)
	}
}

func TestChapterAndVolumeIDHelpers(t *testing.T) {
	if volume, ok := ToolVolumeIDFromChapterID("P1-V2-C3"); !ok || volume != "P1-V2" {
		t.Fatalf("ToolVolumeIDFromChapterID() = %q, %v", volume, ok)
	}
	if _, ok := ToolVolumeIDFromChapterID("P1-V2"); ok {
		t.Fatal("volume id should not parse as a chapter id")
	}
	if !ToolLooksLikeVolumeID("P1-V2") || ToolLooksLikeVolumeID("P1-V2-C3") {
		t.Fatal("ToolLooksLikeVolumeID misclassified ids")
	}
}
