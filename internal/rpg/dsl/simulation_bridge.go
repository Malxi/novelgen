package dsl

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
)

// SimulationBridge converts DSL SimulationIssues into models.ReviewSuggestion
// and provides merge/format helpers for integrate improve steps.
type SimulationBridge struct{}

// NewSimulationBridge creates a new SimulationBridge.
func NewSimulationBridge() *SimulationBridge {
	return &SimulationBridge{}
}

// severityToPriority maps DSL severity to ReviewSuggestion priority.
func severityToPriority(s SeverityLevel) string {
	switch s {
	case SeverityCritical:
		return models.PriorityCritical
	case SeverityWarning:
		return models.PriorityMedium
	case SeverityInfo:
		return models.PriorityLow
	default:
		return models.PriorityLow
	}
}

// issueTypeToCategory maps DSL issue type to ReviewSuggestion category.
func issueTypeToCategory(t IssueType) string {
	switch t {
	case IssueLogic:
		return models.CategoryLogic
	case IssueCharacter:
		return models.CategoryCharacter
	case IssueContinuity:
		return models.CategoryConsistency
	case IssuePacing:
		return "pacing"
	case IssuePlotHole:
		return models.CategoryPlot
	case IssueMissingInfo:
		return models.CategoryStructure
	case IssueConflict:
		return models.CategoryPlot
	case IssueDescription:
		return models.CategoryStructure
	case IssueBalance:
		return models.CategoryLogic
	case IssueGrowth:
		return models.CategoryCharacter
	case IssueEquipment:
		return models.CategoryStructure
	default:
		return models.CategoryConsistency
	}
}

// ConvertIssueToSuggestion maps a single SimulationIssue to ReviewSuggestion.
func (b *SimulationBridge) ConvertIssueToSuggestion(issue SimulationIssue) models.ReviewSuggestion {
	targetName := issue.Chapter
	if targetName == "" {
		targetName = "global"
	}

	return models.ReviewSuggestion{
		Category:   issueTypeToCategory(issue.Type),
		TargetID:   issue.Chapter,
		TargetName: targetName,
		Issue:      issue.Description,
		Suggestion: issue.Suggestion,
		Priority:   severityToPriority(issue.Severity),
	}
}

// ConvertIssuesToSuggestions converts a slice of SimulationIssues to ReviewSuggestions.
func (b *SimulationBridge) ConvertIssuesToSuggestions(issues []SimulationIssue) []models.ReviewSuggestion {
	result := make([]models.ReviewSuggestion, 0, len(issues))
	for _, issue := range issues {
		result = append(result, b.ConvertIssueToSuggestion(issue))
	}
	return result
}

// MergeIntoReview appends DSL simulation issues into an existing ReviewResult.
// Deduplicates by (TargetID, Issue) to avoid repeating findings from both
// AI review and DSL simulation.
func (b *SimulationBridge) MergeIntoReview(issues []SimulationIssue, review *models.ReviewResult) {
	if review == nil || len(issues) == 0 {
		return
	}

	// Build dedup set from existing suggestions
	seen := make(map[string]bool)
	for _, s := range review.Suggestions {
		key := s.TargetID + "|" + s.Issue
		seen[key] = true
	}

	for _, issue := range issues {
		suggestion := b.ConvertIssueToSuggestion(issue)
		key := suggestion.TargetID + "|" + suggestion.Issue
		if seen[key] {
			continue
		}
		seen[key] = true
		review.Suggestions = append(review.Suggestions, suggestion)

		// Also add critical DSL issues as weaknesses
		if issue.Severity == SeverityCritical {
			review.Weaknesses = append(review.Weaknesses,
				fmt.Sprintf("[DSL:%s] %s", issue.Type, issue.Description))
		}
	}

	// Recalculate overall score to reflect new issues
	if len(review.Suggestions) > 0 {
		criticalCount := 0
		for _, issue := range issues {
			if issue.Severity == SeverityCritical {
				criticalCount++
			}
		}
		if criticalCount > 0 {
			penalty := float64(criticalCount * 3)
			if penalty > 20 {
				penalty = 20
			}
			review.OverallScore = max(0, review.OverallScore-penalty)
		}
	}
}

// FormatAsString formats issues as a markdown text block for commands that
// use string-based suggestions (draft and write improve steps).
func (b *SimulationBridge) FormatAsString(issues []SimulationIssue) string {
	if len(issues) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 系统模拟反馈 (RPG DSL Simulation)\n\n")

	// Group by severity
	for _, severity := range []SeverityLevel{SeverityCritical, SeverityWarning, SeverityInfo} {
		var group []SimulationIssue
		for _, issue := range issues {
			if issue.Severity == severity {
				group = append(group, issue)
			}
		}
		if len(group) == 0 {
			continue
		}

		label := severityLabel(severity)
		sb.WriteString(fmt.Sprintf("### %s\n", label))
		for _, issue := range group {
			loc := issue.Chapter
			if issue.Step > 0 {
				loc = fmt.Sprintf("%s step %d", loc, issue.Step)
			}
			sb.WriteString(fmt.Sprintf("- **[%s]** (%s) %s\n", issue.Type, loc, issue.Description))
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("  - 建议: %s\n", issue.Suggestion))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// IssuesForChapter filters issues relevant to a specific chapter.
// Includes issues with matching Chapter ID and issues with empty Chapter (global).
func (b *SimulationBridge) IssuesForChapter(issues []SimulationIssue, chapterID string) []SimulationIssue {
	var result []SimulationIssue
	for _, issue := range issues {
		if issue.Chapter == "" || issue.Chapter == chapterID {
			result = append(result, issue)
		}
	}
	return result
}

func severityLabel(s SeverityLevel) string {
	switch s {
	case SeverityCritical:
		return "严重问题"
	case SeverityWarning:
		return "需关注"
	case SeverityInfo:
		return "参考信息"
	default:
		return string(s)
	}
}
