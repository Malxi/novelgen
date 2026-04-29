package cmd

import (
	"strings"
	"testing"

	"novelgen/internal/rpg"
)

func TestOutlineSuggestionToReviewSuggestionUsesReasonAsIssue(t *testing.T) {
	longBeat := strings.Repeat("主角完成一连串战斗与发现，", 20)
	suggestion := outlineSuggestionToReviewSuggestion(rpg.OutlineSuggestion{
		Type:      "logic",
		Location:  "P1-V1-C7",
		Current:   longBeat,
		Suggested: longBeat + "并明确冲突得到升级",
		Reason:    "确保冲突有明确的阶段性结果",
	})

	if !strings.Contains(suggestion.Issue, "确保冲突有明确的阶段性结果") {
		t.Fatalf("expected reason in issue, got %q", suggestion.Issue)
	}
	if len([]rune(suggestion.Issue)) > 170 {
		t.Fatalf("issue should carry short evidence, got %d runes: %q", len([]rune(suggestion.Issue)), suggestion.Issue)
	}
	if !strings.Contains(suggestion.Suggestion, "明确冲突得到升级") {
		t.Fatalf("expected suggested action to be preserved, got %q", suggestion.Suggestion)
	}
}
