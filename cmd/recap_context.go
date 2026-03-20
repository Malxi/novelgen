package cmd

import (
	"encoding/json"
	"strings"

	"nolvegen/internal/logic/continuity/recap"
	"nolvegen/internal/models"
)

// tryExtractRecap attempts to extract a compact, high-signal recap from a chapter
// draft text. This is a best-effort helper: if it fails, we simply return "" and
// generation proceeds with the existing full-context approach.
//
// NOTE: We intentionally avoid making this depend on LLM calls here to keep
// tests fast/offline and the change reversible.
func tryExtractRecap(ch *models.Chapter, draft string) string {
	if ch == nil {
		return ""
	}
	text := strings.TrimSpace(draft)
	if text == "" {
		return ""
	}

	// Best-effort continuity recap strategy:
	// 1) Prefer a persisted recap JSON for this chapter (if it exists).
	// 2) Fallback to a minimal offline recap derived from outline + last line.
	root, err := findProjectRoot()
	if err == nil {
		store := recap.NewStore(root)
		if saved, err := store.Load(ch.ID); err == nil && saved != nil {
			if b, err := json.MarshalIndent(saved, "", "  "); err == nil {
				// Return raw JSON only. Prompts already wrap recap blocks; double-wrapping
				// reduces model compliance and makes the recap section noisy.
				return string(b)
			}
		}
	}

	recap := buildOfflineRecap(ch, text)
	b, err := json.MarshalIndent(recap, "", "  ")
	if err != nil {
		return ""
	}

	// Return raw JSON only. Prompts already wrap recap blocks; double-wrapping
	// reduces model compliance and makes the recap section noisy.
	return string(b)
}

// buildOfflineRecap creates a small, deterministic recap without any LLM calls.
// This keeps tests fast/offline while still giving the next chapter a concrete
// scene-anchor via LastLine/NextOpeningHint.
func buildOfflineRecap(ch *models.Chapter, text string) *models.ChapterRecap {
	lastLine := extractLastNonEmptyLine(text)
	recap := &models.ChapterRecap{
		ChapterID: ch.ID,
		Title:     ch.Title,
		Location:  ch.Location,
		Time:      "",
		Present:   ch.Characters,
		PlotBeats: []string{strings.TrimSpace(ch.Summary)},
		LastLine:  lastLine,
		// For the offline fallback, we treat the last line as the best available
		// cliffhanger signal.
		Cliffhanger: lastLine,
	}
	if strings.TrimSpace(lastLine) != "" {
		recap.Unresolved = []string{lastLine}
		// NextOpeningHint should be 1–3 sentences that can be pasted as the next chapter's opening.
		// Keep it in the same language as the draft (usually Chinese) and ensure it visibly
		// overlaps the last line for continuity checks.
		recap.NextOpeningHint = lastLine + "（紧接上一章最后一刻，地点与时间不变。）"
	}
	return recap
}

// persistOfflineRecap saves an offline recap for the given chapter.
// Best-effort: failures are ignored by callers.
func persistOfflineRecap(ch *models.Chapter, text string) {
	if ch == nil {
		return
	}
	clean := strings.TrimSpace(text)
	if clean == "" {
		return
	}
	root, err := findProjectRoot()
	if err != nil {
		return
	}
	store := recap.NewStore(root)
	_ = store.Save(buildOfflineRecap(ch, clean))
}

// persistOfflineRecapIfMissing saves an offline recap only when there is no
// persisted recap JSON yet. This is useful for chapter-to-chapter continuity
// when generating out of order or in environments where recap extraction hasn't
// run.
func persistOfflineRecapIfMissing(ch *models.Chapter, text string) {
	if ch == nil {
		return
	}
	clean := strings.TrimSpace(text)
	if clean == "" {
		return
	}
	root, err := findProjectRoot()
	if err != nil {
		return
	}
	store := recap.NewStore(root)
	if saved, err := store.Load(ch.ID); err == nil && saved != nil {
		return
	}
	_ = store.Save(buildOfflineRecap(ch, clean))
}
