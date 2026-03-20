package cmd

import (
	"strings"

	"nolvegen/internal/models"
)

func recapGateFeedback(reasons []string, recap *models.ChapterRecap) string {
	var sb strings.Builder
	if len(reasons) > 0 {
		sb.WriteString("Recap minimal gate failed. You MUST fix the following missing/invalid fields:\n")
		for _, r := range reasons {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			sb.WriteString("- " + r + "\n")
		}
		sb.WriteString("\n")
	}
	// Provide explicit directives for the key fields to improve compliance.
	sb.WriteString("Hard requirements:\n")
	sb.WriteString("- location: MUST be the concrete end-of-chapter location (not vague).\n")
	sb.WriteString("- present: MUST list who is present in the final scene.\n")
	sb.WriteString("- last_line: MUST be the last spoken line or last sentence describing the last moment.\n")
	sb.WriteString("- next_opening_hint: MUST be 1–3 sentences that can be used as the next chapter's opening and must continue from last_line.\n")

	_ = recap
	return sb.String()
}
