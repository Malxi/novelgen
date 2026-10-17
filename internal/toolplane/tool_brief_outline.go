// Outline briefs: parts, volumes and chapters, plus the storyline advances they carry.

package toolplane

import (
	"novelgen/internal/contract/story"
)

func briefOutlineResults(results interface{}, indexOnly bool) interface{} {
	switch values := results.(type) {
	case *models.Outline:
		if values == nil {
			return results
		}
		out := OutlineBrief{Navigation: map[string]interface{}{
			"detail_queries": []string{
				"novelgen tool query outline --type volume --id <volume_id>",
				"novelgen tool query outline --type chapter --id <chapter_id>",
				"novelgen tool query outline --type events --chapter-id <chapter_id>",
			},
		}}
		out.PartCount = len(values.Parts)
		for _, part := range values.Parts {
			out.Parts = append(out.Parts, makePartBrief(part, indexOnly))
			out.VolumeCount += len(part.Volumes)
			for _, volume := range part.Volumes {
				out.ChapterCount += len(volume.Chapters)
			}
		}
		return out
	case []OutlineHit:
		out := make([]OutlineHit, 0, len(values))
		for _, hit := range values {
			if indexOnly {
				hit.Summary = ""
			}
			hit.Object = briefOutlineObject(hit.Object, indexOnly)
			out = append(out, hit)
		}
		return out
	case []EventHit:
		out := make([]EventHitBrief, 0, len(values))
		for _, hit := range values {
			out = append(out, makeEventHitBrief(hit, indexOnly))
		}
		return out
	default:
		return results
	}
}

func briefOutlineObject(value interface{}, indexOnly bool) interface{} {
	switch typed := value.(type) {
	case models.Part:
		return makePartBrief(typed, indexOnly)
	case models.Volume:
		return makeVolumeBrief(typed, indexOnly, true)
	case models.Chapter:
		return makeChapterBrief(typed, indexOnly)
	case *models.Part:
		if typed != nil {
			return makePartBrief(*typed, indexOnly)
		}
	case *models.Volume:
		if typed != nil {
			return makeVolumeBrief(*typed, indexOnly, true)
		}
	case *models.Chapter:
		if typed != nil {
			return makeChapterBrief(*typed, indexOnly)
		}
	}
	return value
}

func makePartBrief(part models.Part, indexOnly bool) PartBrief {
	brief := PartBrief{
		ID:          part.ID,
		Title:       part.Title,
		VolumeCount: len(part.Volumes),
	}
	if !indexOnly {
		brief.Summary = clipToolString(part.Summary, 500)
	}
	for _, volume := range part.Volumes {
		brief.ChapterCount += len(volume.Chapters)
		brief.Volumes = append(brief.Volumes, makeVolumeBrief(volume, indexOnly, !indexOnly))
	}
	return brief
}

func makeVolumeBrief(volume models.Volume, indexOnly bool, includeChapters bool) VolumeBrief {
	brief := VolumeBrief{
		ID:           volume.ID,
		Title:        volume.Title,
		ChapterCount: len(volume.Chapters),
		Navigation: map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id>",
				"novelgen tool query outline --type events --chapter-id <chapter_id>",
			},
		},
	}
	if !indexOnly {
		brief.Summary = clipToolString(volume.Summary, 700)
		if !volume.PayoffContract.IsZero() {
			brief.PayoffContract = volume.PayoffContract
		}
	}
	if !includeChapters {
		return brief
	}
	for _, chapter := range volume.Chapters {
		if indexOnly {
			brief.Chapters = append(brief.Chapters, makeChapterBrief(chapter, true))
		} else {
			brief.Chapters = append(brief.Chapters, makeChapterNavigationBrief(chapter))
		}
	}
	return brief
}

func makeContextVolumeBrief(volume models.Volume, includeChapters bool) VolumeBrief {
	brief := VolumeBrief{
		ID:      volume.ID,
		Title:   volume.Title,
		Summary: clipToolString(volume.Summary, 360),
		Navigation: map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id> --view brief",
				"novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			},
		},
	}
	if !volume.PayoffContract.IsZero() {
		brief.PayoffContract = volume.PayoffContract
	}
	if !includeChapters {
		return brief
	}
	for _, chapter := range volume.Chapters {
		brief.Chapters = append(brief.Chapters, makeContextChapterBrief(chapter))
	}
	return brief
}

func makeContextVolumeRouteMap(volume models.Volume, includeChapters bool) map[string]interface{} {
	brief := map[string]interface{}{
		"id":      volume.ID,
		"title":   volume.Title,
		"summary": clipToolString(volume.Summary, 180),
		"navigation": map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id> --view brief",
				"novelgen tool query outline --type chapter --id <chapter_id> --fields location,timeline,state_anchor --view brief",
				"novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			},
		},
	}
	if !includeChapters {
		return brief
	}
	chapters := make([]map[string]interface{}, 0, len(volume.Chapters))
	for _, chapter := range volume.Chapters {
		chapters = append(chapters, map[string]interface{}{
			"id":            chapter.ID,
			"title":         chapter.Title,
			"characters":    clipToolStrings(chapter.Characters, 6, 40),
			"location":      clipToolString(chapter.Location, 80),
			"event_count":   len(chapter.Events),
			"advance_count": len(chapter.StorylineAdvances),
		})
	}
	brief["chapters"] = chapters
	return brief
}

func makeRepairVolumeBrief(volume models.Volume, check *ToolCheckResult) VolumeBrief {
	brief := makeContextVolumeBrief(volume, false)
	chapterIDs := map[string]bool{}
	if check != nil {
		for _, issue := range check.Issues {
			if _, ok := ToolVolumeIDFromChapterID(issue.TargetID); ok {
				chapterIDs[issue.TargetID] = true
			}
		}
	}
	for _, chapter := range volume.Chapters {
		if len(chapterIDs) == 0 || chapterIDs[chapter.ID] {
			brief.Chapters = append(brief.Chapters, makeContextChapterBrief(chapter))
		}
		if len(chapterIDs) == 0 && len(brief.Chapters) >= 8 {
			break
		}
	}
	return brief
}

func compactStorylineAdvances(advances []models.StorylineAdvance, maxItems int) []models.StorylineAdvance {
	if maxItems <= 0 || len(advances) == 0 {
		return nil
	}
	if len(advances) > maxItems {
		advances = advances[:maxItems]
	}
	result := make([]models.StorylineAdvance, 0, len(advances))
	for _, advance := range advances {
		result = append(result, models.StorylineAdvance{
			StorylineName: clipToolString(advance.StorylineName, 80),
			Stage:         clipToolString(advance.Stage, 50),
			Change:        clipToolString(advance.Change, 180),
			Consequence:   clipToolString(advance.Consequence, 180),
			Pressure:      clipToolString(advance.Pressure, 180),
		})
	}
	return result
}
