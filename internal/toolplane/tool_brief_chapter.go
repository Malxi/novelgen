// Chapter and event briefs, including the saved manuscript excerpt loaded for repair prompts.

package toolplane

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/store"
)

func makeChapterBrief(chapter models.Chapter, indexOnly bool) ChapterBrief {
	brief := ChapterBrief{
		ID:           chapter.ID,
		Title:        chapter.Title,
		Summary:      clipToolString(chapter.Summary, 500),
		Characters:   chapter.Characters,
		Location:     chapter.Location,
		StateChange:  clipToolString(chapter.StateChange, 180),
		EventCount:   len(chapter.Events),
		AdvanceCount: len(chapter.StorylineAdvances),
	}
	if indexOnly {
		return brief
	}
	brief.OpeningBeat = clipToolString(chapter.OpeningBeat, 180)
	brief.ClosingBeat = clipToolString(chapter.ClosingBeat, 180)
	brief.Conflict = clipToolString(chapter.Conflict, 220)
	brief.Pacing = clipToolString(chapter.Pacing, 180)
	brief.Timeline = chapter.Timeline
	brief.StateAnchor = chapter.StateAnchor
	brief.StorylineAdvances = chapter.StorylineAdvances
	brief.ChapterPayoff = chapter.ChapterPayoff
	for _, event := range chapter.Events {
		brief.Events = append(brief.Events, makeEventBrief(event))
	}
	return brief
}

func makeEventBrief(event models.Event) EventBrief {
	return EventBrief{
		Type:       event.Type,
		Characters: event.Characters,
		Subject:    clipToolString(event.Subject, 120),
		Change:     clipToolString(event.Change, 140),
		Details:    clipToolString(event.Details, 180),
		Actor:      event.GetActor(),
		Action:     event.GetAction(),
		Target:     event.GetTarget(),
		TargetType: event.GetTargetType(),
		Context:    clipToolString(event.Context, 140),
		Result:     clipToolString(event.Result, 160),
	}
}

func makeEventIndexBrief(event models.Event) EventBrief {
	return EventBrief{
		Type:       event.Type,
		Subject:    clipToolString(event.Subject, 80),
		Change:     clipToolString(event.Change, 80),
		Actor:      event.GetActor(),
		Action:     event.GetAction(),
		Target:     event.GetTarget(),
		TargetType: event.GetTargetType(),
	}
}

func makeEventHitBrief(hit EventHit, indexOnly bool) EventHitBrief {
	brief := EventHitBrief{
		ChapterID:    hit.ChapterID,
		ChapterTitle: hit.ChapterTitle,
		EventIndex:   hit.EventIndex,
		Reasons:      hit.Reasons,
	}
	if indexOnly {
		brief.Event = makeEventIndexBrief(hit.Event)
		return brief
	}
	brief.Path = &hit.Path
	brief.Event = makeEventBrief(hit.Event)
	return brief
}

func briefChapterResults(results interface{}, indexOnly bool) interface{} {
	values, ok := results.([]chapterHit)
	if !ok {
		return results
	}
	out := make([]map[string]interface{}, 0, len(values))
	for _, hit := range values {
		item := map[string]interface{}{
			"id":      hit.ID,
			"title":   hit.Title,
			"summary": hit.Summary,
			"path":    hit.Path,
		}
		if len(hit.Reasons) > 0 {
			item["reasons"] = hit.Reasons
		}
		if hit.Outline != nil {
			brief := makeChapterBrief(*hit.Outline, indexOnly)
			item["outline"] = brief
		}
		if hit.ContentPath != "" && !indexOnly {
			item["content_path"] = hit.ContentPath
		}
		out = append(out, item)
	}
	return out
}

func loadChapterExcerptForTool(root *store.Root, chapterID string) (map[string]string, []string) {
	chapterID = strings.TrimSpace(chapterID)
	if root.Dir() == "" || chapterID == "" {
		return nil, []string{"project root or chapter id missing; chapter excerpt unavailable"}
	}
	for _, path := range root.ChapterCandidates(chapterID) {
		data, err := root.Read(path)
		if err != nil {
			continue
		}
		content := string(data)
		return map[string]string{
			"content_path": path,
			"opening":      clipToolString(content, 320),
			"closing":      clipToolTailString(content, 420),
			"last_line":    clipToolString(lastNonEmptyLine(content), 220),
		}, nil
	}
	return nil, []string{fmt.Sprintf("chapter manuscript not found for %s", chapterID)}
}
