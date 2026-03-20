package cmd

import "strings"

// extractLastNonEmptyLine returns the last non-empty line of text.
// This is used as a best-effort scene-anchor signal for offline recap fallback.
func extractLastNonEmptyLine(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" {
			return l
		}
	}
	return ""
}
