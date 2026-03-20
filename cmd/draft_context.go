package cmd

import (
	"fmt"
	"strings"

	"nolvegen/internal/models"
)

// loadPreviousDraftContext loads FULL previous draft chapters for continuity.
// contextCount = number of previous chapters to include.
func loadPreviousDraftContext(outline *models.Outline, targetChapter *models.Chapter, contextCount int) string {
	if contextCount <= 0 {
		return ""
	}

	allChapters := getAllChapters(outline)
	// Find target chapter index
	targetIndex := -1
	for i, ch := range allChapters {
		if ch.ID == targetChapter.ID {
			targetIndex = i
			break
		}
	}
	if targetIndex <= 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("=== CONTINUITY CONTEXT (PREVIOUS CHAPTERS) ===\n")

	start := targetIndex - contextCount
	if start < 0 {
		start = 0
	}

	for i := start; i < targetIndex; i++ {
		ch := allChapters[i]
		// Reuse existing loader used by write.go
		draft := loadDraftContent(ch.ID)
		if strings.TrimSpace(draft) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", ch.ID, ch.Title))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", ch.Summary))
		sb.WriteString("Draft (full):\n")
		sb.WriteString(draft)
		sb.WriteString("\n")
	}

	sb.WriteString("=== END CONTINUITY CONTEXT ===\n")
	return sb.String()
}
