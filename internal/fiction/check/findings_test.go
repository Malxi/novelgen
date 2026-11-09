package check

import (
	"encoding/json"
	"testing"

	"novelgen/internal/contract/story"
)

func TestToolCheckJSONExposesFindingsAlias(t *testing.T) {
	result := ToolCheckResult{
		Kind:   "quality",
		Target: "outline",
		Scope:  "all",
		Summary: ToolCheckSummary{
			Total: 1,
			High:  1,
		},
		Issues: []models.ReviewSuggestion{{
			Category:   "item_usage_continuity",
			Priority:   "high",
			Issue:      "item used before acquisition",
			Suggestion: "add an acquisition scene",
		}},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload struct {
		Issues   []map[string]interface{} `json:"issues"`
		Findings []map[string]interface{} `json:"findings"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Issues) != 1 || len(payload.Findings) != 1 {
		t.Fatalf("issues/findings = %d/%d, want 1/1\n%s", len(payload.Issues), len(payload.Findings), data)
	}
	if payload.Findings[0]["severity"] != "high" {
		t.Fatalf("severity = %v, want high", payload.Findings[0]["severity"])
	}
	if payload.Findings[0]["rule_id"] != "item_usage_continuity" {
		t.Fatalf("rule_id = %v", payload.Findings[0]["rule_id"])
	}
}
