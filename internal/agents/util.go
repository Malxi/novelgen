package agents

import (
	"strings"
)

// extractJSONFromMarkdown extracts JSON from markdown code blocks if present
func extractJSONFromMarkdown(content string) string {
	// Look for JSON in code blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}
	if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + 3
		end := strings.Index(content[start:], "```")
		if end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}
	return content
}
