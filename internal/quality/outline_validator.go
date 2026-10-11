package quality

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

// RunOutlineValidatorOnModel converts models.Outline to rpg.StoryOutline, runs
// the outline validator, and returns issues as ReviewSuggestions.
func RunOutlineValidatorOnModel(outline *models.Outline) []models.ReviewSuggestion {
	if outline == nil {
		return nil
	}
	// Convert via JSON roundtrip (models.Outline -> rpg.StoryOutline)
	data, err := json.Marshal(outline)
	if err != nil {
		return nil
	}
	var storyOutline rpg.StoryOutline
	if err := json.Unmarshal(data, &storyOutline); err != nil {
		return nil
	}

	validator := rpg.NewOutlineValidator(&storyOutline)
	result := validator.Validate()

	var suggestions []models.ReviewSuggestion

	for _, issue := range result.Issues {
		suggestions = append(suggestions, models.ReviewSuggestion{
			Category:   issue.Type,
			TargetID:   issue.Location,
			TargetName: issue.Location,
			Issue:      issue.Description,
			Suggestion: issue.Fix,
			Priority:   SeverityToPriorityStr(issue.Severity),
		})
	}
	for _, w := range result.Warnings {
		suggestions = append(suggestions, models.ReviewSuggestion{
			Category:   w.Type,
			TargetID:   w.Location,
			TargetName: w.Location,
			Issue:      w.Description,
			Suggestion: w.Suggestion,
			Priority:   models.PriorityMedium,
		})
	}
	for _, s := range result.Suggestions {
		if s.Type == "faction_tier" {
			// The outline-only validator cannot see story_setup.json, so its
			// "define faction tiers in setup" hint is not self-verifying. The
			// setup+outline cross-check owns that contract.
			continue
		}
		suggestions = append(suggestions, OutlineSuggestionToReviewSuggestion(s))
	}
	return suggestions
}

// OutlineSuggestionToReviewSuggestion maps one RPG outline suggestion to the
// shared ReviewSuggestion shape.
func OutlineSuggestionToReviewSuggestion(s rpg.OutlineSuggestion) models.ReviewSuggestion {
	issue := strings.TrimSpace(s.Reason)
	if issue == "" {
		issue = OutlineSuggestionIssueFromType(s.Type)
	}
	if current := ShortEvidence(s.Current, 120); current != "" {
		issue = fmt.Sprintf("%s；当前表现：%s", issue, current)
	}

	suggestion := strings.TrimSpace(s.Suggested)
	if suggestion == "" {
		suggestion = "按问题原因补充一个具体、可执行的章节调整。"
	}

	return models.ReviewSuggestion{
		Category:   s.Type,
		TargetID:   s.Location,
		TargetName: s.Location,
		Issue:      issue,
		Suggestion: suggestion,
		Priority:   models.PriorityLow,
	}
}

func OutlineSuggestionIssueFromType(issueType string) string {
	switch strings.TrimSpace(issueType) {
	case "logic":
		return "章节逻辑需要更明确的因果或阶段性结果"
	case "conflict":
		return "章节冲突强度或落点不足"
	case "pacing":
		return "章节节奏需要调整"
	case "timeline":
		return "时间线锚点或过渡不够清晰"
	case "state_anchor":
		return "章节状态锚点缺少可追踪的变化依据"
	case "storyline_texture":
		return "故事线推进记录缺少戏剧压力、后果或具体变化"
	default:
		return "大纲存在可优化项"
	}
}

func ShortEvidence(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "缺少") || strings.HasPrefix(value, "chapter has") || strings.Contains(value, "entry has no") {
		return value
	}
	runes := []rune(value)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return value
}

func SeverityToPriorityStr(severity string) string {
	switch severity {
	case "critical":
		return models.PriorityHigh
	case "major":
		return models.PriorityHigh
	case "minor":
		return models.PriorityMedium
	default:
		return models.PriorityLow
	}
}
