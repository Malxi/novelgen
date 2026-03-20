package cmd

import (
	"encoding/json"
	"strings"

	"nolvegen/internal/logic/continuity/recap"
	"nolvegen/internal/models"
)

// loadPreviousRecapStruct loads a structured recap for the immediately previous chapter (best-effort).
// It prefers persisted recap JSON from story/recaps; if missing, it returns nil.
func loadPreviousRecapStruct(outline *models.Outline, targetChapter *models.Chapter) *models.ChapterRecap {
	if outline == nil || targetChapter == nil {
		return nil
	}

	all := getAllChapters(outline)
	idx := -1
	for i, ch := range all {
		if ch.ID == targetChapter.ID {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return nil
	}

	prev := all[idx-1]
	root, err := findProjectRoot()
	if err != nil || strings.TrimSpace(root) == "" {
		return nil
	}
	store := recap.NewStore(root)
	recap, err := store.Load(prev.ID)
	if err != nil {
		return nil
	}
	return recap
}

// loadPreviousRecapJSON returns the previous chapter recap as compact JSON (best-effort).
// This is convenient for prompts that expect the canonical recap block.
func loadPreviousRecapJSON(outline *models.Outline, targetChapter *models.Chapter) string {
	recap := loadPreviousRecapStruct(outline, targetChapter)
	if recap == nil {
		return ""
	}
	b, err := json.MarshalIndent(recap, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}
