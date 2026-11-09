package toolplane

import "novelgen/internal/fiction/mutation"

// limitToolPatchResult keeps a tool result small enough for an agent context
// while preserving the full summary counts. A max <= 0 leaves the result intact.
func limitToolPatchResult(result *mutation.ToolPatchResult, maxIssues int) {
	if result == nil || maxIssues <= 0 {
		return
	}
	if result.Check != nil && len(result.Check.Issues) > maxIssues {
		total := len(result.Check.Issues)
		result.Check.Issues = result.Check.Issues[:maxIssues]
		if result.Check.Meta == nil {
			result.Check.Meta = map[string]interface{}{}
		}
		result.Check.Meta["issues_truncated"] = true
		result.Check.Meta["issues_total"] = total
	}
	const maxChanges = 40
	if len(result.Changed) > maxChanges {
		total := len(result.Changed)
		result.Changed = result.Changed[:maxChanges]
		if result.Meta == nil {
			result.Meta = map[string]interface{}{}
		}
		result.Meta["changes_truncated"] = true
		result.Meta["changes_total"] = total
	}
}
