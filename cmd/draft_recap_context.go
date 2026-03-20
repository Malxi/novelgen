package cmd

import (
	"strings"

	"nolvegen/internal/models"
)

// loadPreviousDraftRecap returns a compact, high-signal recap of the immediately
// previous chapter draft (best-effort). If not available, returns "".
func loadPreviousDraftRecap(outline *models.Outline, targetChapter *models.Chapter) string {
	if outline == nil || targetChapter == nil {
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

	prev := allChapters[targetIndex-1]
	draft := loadDraftContent(prev.ID)
	if strings.TrimSpace(draft) == "" {
		return ""
	}
	return tryExtractRecap(prev, draft)
}
