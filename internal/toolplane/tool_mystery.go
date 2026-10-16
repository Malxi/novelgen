// The mystery-thread ledger behind outline global repair: collection, filtering and patchable routing.

package toolplane

import (
	"fmt"
	"novelgen/internal/models"
	"novelgen/internal/toolcmd"
	"strings"
)

func CollectOutlineMysteryThreads(outline *models.Outline) []outlineMysteryThread {
	chapters := flattenChapters(outline)
	if len(chapters) == 0 {
		return nil
	}
	byID := map[string]int{}
	threads := make([]outlineMysteryThread, 0)
	for idx, chapter := range chapters {
		if chapter.Chapter == nil {
			continue
		}
		for _, planted := range chapter.Chapter.Mysteries.Planted {
			id := strings.TrimSpace(planted.ID)
			if id == "" {
				continue
			}
			if existing, ok := byID[id]; ok {
				if threads[existing].Clue == "" {
					threads[existing].Clue = clipToolString(planted.Clue, 220)
				}
				continue
			}
			thread := outlineMysteryThread{
				ID:                    id,
				Clue:                  clipToolString(planted.Clue, 220),
				Horizon:               planted.Horizon,
				Status:                planted.Status,
				PlantedChapterID:      chapter.Chapter.ID,
				PlantedChapterTitle:   chapter.Chapter.Title,
				PlantedChapterSummary: clipToolString(chapter.Chapter.Summary, 220),
				PlantedVolumeID:       chapter.Path.VolumeID,
				PlantedVolumeTitle:    chapter.Path.VolumeTitle,
			}
			if candidate := suggestedMysteryResolutionChapter(chapters, idx); candidate != nil && candidate.Chapter != nil {
				thread.SuggestedResolutionChapterID = candidate.Chapter.ID
				thread.SuggestedResolutionChapterTitle = candidate.Chapter.Title
				thread.SuggestedResolutionChapterSummary = clipToolString(candidate.Chapter.Summary, 260)
				thread.SuggestedResolutionVolumeID = candidate.Path.VolumeID
				thread.SuggestedResolutionVolumeTitle = candidate.Path.VolumeTitle
				thread.SuggestedResolutionHint = mysteryResolutionHint(id, planted.Clue, candidate.Chapter)
				thread.RepairStrategy = "resolve_in_later_chapter"
				thread.PatchQuery = toolcmd.PatchOutlineVolume(candidate.Path.VolumeID)
				thread.PatchShape = mysteryResolutionPatchShape(id, thread.SuggestedResolutionHint, candidate)
				thread.PostPatchCheckQuery = `novelgen tool check all --target outline --scope all --category mysteries --min-priority low --max-issues 12`
			} else {
				thread.RepairStrategy = "mark_deferred_or_extend_outline"
			}
			byID[id] = len(threads)
			threads = append(threads, thread)
		}
		for _, resolved := range chapter.Chapter.Mysteries.Resolved {
			id := strings.TrimSpace(resolved.ID)
			if id == "" {
				continue
			}
			if existing, ok := byID[id]; ok {
				threads[existing].ResolvedChapterIDs = append(threads[existing].ResolvedChapterIDs, chapter.Chapter.ID)
				continue
			}
			thread := outlineMysteryThread{
				ID:                 id,
				ResolvedChapterIDs: []string{chapter.Chapter.ID},
				RepairStrategy:     "resolved_without_known_planted_context",
			}
			byID[id] = len(threads)
			threads = append(threads, thread)
		}
	}
	return threads
}

func suggestedMysteryResolutionChapter(chapters []flatChapter, plantedIndex int) *flatChapter {
	for i := plantedIndex + 1; i < len(chapters); i++ {
		if chapters[i].Chapter != nil && strings.TrimSpace(chapters[i].Chapter.ID) != "" {
			return &chapters[i]
		}
	}
	return nil
}

func mysteryResolutionHint(mysteryID, clue string, candidate *models.Chapter) string {
	if candidate == nil {
		return fmt.Sprintf("Reveal or confirm %s using an already planned later beat.", mysteryID)
	}
	parts := []string{fmt.Sprintf("Resolve %s", mysteryID)}
	if clue = strings.TrimSpace(clue); clue != "" {
		parts = append(parts, "from clue: "+clipToolString(clue, 120))
	}
	if summary := strings.TrimSpace(candidate.Summary); summary != "" {
		parts = append(parts, "inside chapter beat: "+clipToolString(summary, 160))
	}
	return strings.Join(parts, "; ")
}

func mysteryResolutionPatchShape(mysteryID, hint string, candidate *flatChapter) map[string]interface{} {
	if candidate == nil || candidate.Chapter == nil {
		return nil
	}
	resolved := make([]map[string]string, 0, len(candidate.Chapter.Mysteries.Resolved)+1)
	for _, existing := range candidate.Chapter.Mysteries.Resolved {
		if strings.TrimSpace(existing.ID) == "" {
			continue
		}
		resolved = append(resolved, map[string]string{
			"id":         existing.ID,
			"resolution": existing.Resolution,
		})
	}
	resolution := strings.TrimSpace(hint)
	if resolution == "" {
		resolution = "<concise in-story answer revealed or confirmed in this chapter>"
	}
	resolved = append(resolved, map[string]string{
		"id":         mysteryID,
		"resolution": resolution,
	})
	return map[string]interface{}{
		"changed_chapters": []map[string]interface{}{{
			"id": candidate.Chapter.ID,
			"mysteries": map[string]interface{}{
				"resolved": resolved,
			},
		}},
	}
}

func filterOutlineMysteryThreadsForCategory(threads []outlineMysteryThread, category string) []outlineMysteryThread {
	if NormalizeKey(category) != "mysteries" {
		return nil
	}
	if len(threads) > 12 {
		return threads[:12]
	}
	return threads
}

func FirstPatchableOutlineMysteryThread(threads []outlineMysteryThread) *outlineMysteryThread {
	for i := range threads {
		if len(threads[i].ResolvedChapterIDs) > 0 {
			continue
		}
		if strings.TrimSpace(threads[i].PatchQuery) == "" || threads[i].PatchShape == nil {
			continue
		}
		return &threads[i]
	}
	return nil
}

func countUnresolvedOutlineMysteryThreads(threads []outlineMysteryThread) int {
	count := 0
	for _, thread := range threads {
		if strings.TrimSpace(thread.PlantedChapterID) == "" || len(thread.ResolvedChapterIDs) > 0 {
			continue
		}
		count++
	}
	return count
}
