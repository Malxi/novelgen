// Recap briefs, including the freshness check that keeps a stale recap out of a writing context.

package toolplane

import (
	"fmt"
	"os"
	"strings"

	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/store"
)

func makeRecapBrief(value *models.ChapterRecap) interface{} {
	if value == nil {
		return nil
	}
	return models.ChapterRecap{
		ChapterID:       value.ChapterID,
		Title:           clipToolString(value.Title, 80),
		Location:        clipToolString(value.Location, 120),
		Time:            clipToolString(value.Time, 120),
		Present:         clipToolStrings(value.Present, 12, 50),
		PlotBeats:       clipToolStrings(value.PlotBeats, 5, 160),
		Decisions:       clipToolStrings(value.Decisions, 4, 140),
		Reveals:         clipToolStrings(value.Reveals, 4, 140),
		Unresolved:      clipToolStrings(value.Unresolved, 4, 140),
		Promises:        clipToolStrings(value.Promises, 4, 140),
		Items:           clipToolStrings(value.Items, 5, 140),
		Status:          clipToolStrings(value.Status, 5, 140),
		LastLine:        clipToolString(value.LastLine, 220),
		Cliffhanger:     clipToolString(value.Cliffhanger, 180),
		NextOpeningHint: clipToolString(value.NextOpeningHint, 220),
	}
}

func recapUsableForChapter(value *models.ChapterRecap, chapter *models.Chapter) (bool, string) {
	if value == nil || chapter == nil {
		return false, ""
	}
	recapID := strings.TrimSpace(value.ChapterID)
	chapterID := strings.TrimSpace(chapter.ID)
	if recapID != "" && chapterID != "" && !idMatches(recapID, chapterID) {
		return false, fmt.Sprintf("recap chapter_id %q does not match outline chapter %q", recapID, chapterID)
	}
	recapTitle := strings.TrimSpace(value.Title)
	chapterTitle := strings.TrimSpace(chapter.Title)
	if recapTitle != "" && chapterTitle != "" && !strings.EqualFold(recapTitle, chapterTitle) {
		return false, fmt.Sprintf("recap title %q does not match outline title %q for %s", recapTitle, chapterTitle, chapterID)
	}
	return true, ""
}

func loadUsableToolContextRecap(root *store.Root, store *recap.Store, chapter *models.Chapter) (*models.ChapterRecap, string) {
	if store == nil || chapter == nil {
		return nil, ""
	}
	loaded, err := store.Load(chapter.ID)
	if err != nil {
		return nil, ""
	}
	if ok, warning := recapUsableForChapter(loaded, chapter); !ok {
		return nil, warning
	}
	if ok, warning := recapFreshForChapterMarkdown(root, chapter.ID); !ok {
		return nil, warning
	}
	return loaded, ""
}

func recapFreshForChapterMarkdown(root *store.Root, chapterID string) (bool, string) {
	chapterID = strings.TrimSpace(chapterID)
	if root.Dir() == "" || chapterID == "" {
		return true, ""
	}
	recapPath := root.RecapFile(chapterID)
	chapterPath := firstExistingPath(root.ChapterCandidates(chapterID))
	if chapterPath == "" {
		return true, ""
	}
	recapInfo, recapErr := os.Stat(recapPath)
	chapterInfo, chapterErr := os.Stat(chapterPath)
	if recapErr != nil || chapterErr != nil {
		return true, ""
	}
	if recapInfo.ModTime().Before(chapterInfo.ModTime()) {
		return false, fmt.Sprintf("recap %s is older than chapter markdown %s", recapPath, chapterPath)
	}
	return true, ""
}
