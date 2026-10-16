// The outline and chapter query sections: outline parts/volumes/chapters, entity refs, events and
// saved chapter manuscript lookups.

package toolplane

import (
	"fmt"
	"novelgen/internal/models"
	"strings"
)

func queryOutline(ctx ToolProjectContext, queryType, id, entityType, name, chapterID, volumeID string) ToolResponse {
	switch normalizeOutlineType(queryType) {
	case "", "all":
		count := 0
		if ctx.Outline != nil {
			count = len(ctx.Outline.Parts)
		}
		return ToolResponse{OK: true, Count: count, Results: ctx.Outline}
	case "part":
		return QueryOutlineNode(ctx.Outline, "part", id)
	case "volume":
		return QueryOutlineNode(ctx.Outline, "volume", id)
	case "chapter":
		return QueryOutlineNode(ctx.Outline, "chapter", id)
	case "refs":
		return QueryOutlineRefs(ctx.Outline, entityType, name, false)
	case "events":
		return QueryEvents(ctx.Outline, chapterID, volumeID, entityType, name)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported outline type %q", queryType)}}
	}
}

func queryChapterSection(ctx ToolProjectContext, queryType, id, entityType, name, chapterID, volumeID string, includeContent bool) ToolResponse {
	if strings.TrimSpace(id) != "" {
		return queryChapter(ctx, id, includeContent)
	}
	switch normalizeChapterType(queryType) {
	case "", "all":
		if strings.TrimSpace(entityType) == "" && strings.TrimSpace(name) == "" {
			return queryAllChapters(ctx, includeContent)
		}
		return queryChaptersByEntity(ctx, entityType, name, includeContent)
	case "refs":
		return queryChaptersByEntity(ctx, entityType, name, includeContent)
	case "events":
		return QueryEvents(ctx.Outline, chapterID, volumeID, entityType, name)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported chapter type %q", queryType)}}
	}
}

func queryAllChapters(ctx ToolProjectContext, includeContent bool) ToolResponse {
	chapters := make([]chapterHit, 0)
	for _, item := range flattenChapters(ctx.Outline) {
		hit := chapterHit{
			ID:      item.Chapter.ID,
			Title:   item.Chapter.Title,
			Summary: item.Chapter.Summary,
			Path:    item.Path,
			Outline: item.Chapter,
		}
		if includeContent {
			content, path, err := ctx.Root.LoadChapterContent(item.Chapter.ID)
			if err != nil {
				return ToolResponse{OK: false, Section: "chapter", Count: 0, Warnings: []string{err.Error()}}
			}
			hit.Content, hit.ContentPath = content, path
		}
		chapters = append(chapters, hit)
	}
	return ToolResponse{OK: true, Section: "chapter", Count: len(chapters), Results: chapters}
}

func QueryOutlineNode(outline *models.Outline, nodeType, id string) ToolResponse {
	hits := make([]OutlineHit, 0)
	for _, part := range outline.Parts {
		partPath := OutlinePath{PartID: part.ID, PartTitle: part.Title}
		if nodeType == "part" && idMatches(part.ID, id) {
			hits = append(hits, OutlineHit{Type: "part", ID: part.ID, Title: part.Title, Summary: part.Summary, Path: partPath, Object: part})
		}
		for _, volume := range part.Volumes {
			volumePath := partPath
			volumePath.VolumeID = volume.ID
			volumePath.VolumeTitle = volume.Title
			if nodeType == "volume" && idMatches(volume.ID, id) {
				hits = append(hits, OutlineHit{Type: "volume", ID: volume.ID, Title: volume.Title, Summary: volume.Summary, Path: volumePath, Object: volume})
			}
			for _, chapter := range volume.Chapters {
				chapterPath := volumePath
				chapterPath.ChapterID = chapter.ID
				chapterPath.ChapterTitle = chapter.Title
				if nodeType == "chapter" && idMatches(chapter.ID, id) {
					hits = append(hits, OutlineHit{Type: "chapter", ID: chapter.ID, Title: chapter.Title, Summary: chapter.Summary, Path: chapterPath, Object: chapter})
				}
			}
		}
	}
	return ToolResponse{OK: true, Section: "outline", Count: len(hits), Results: hits}
}

func queryChapter(ctx ToolProjectContext, id string, includeContent bool) ToolResponse {
	if strings.TrimSpace(id) == "" {
		return ToolResponse{OK: false, Count: 0, Warnings: []string{"--id is required for chapter queries"}}
	}
	for _, chapter := range flattenChapters(ctx.Outline) {
		if !idMatches(chapter.Chapter.ID, id) {
			continue
		}
		hit := chapterHit{
			ID:      chapter.Chapter.ID,
			Title:   chapter.Chapter.Title,
			Summary: chapter.Chapter.Summary,
			Path:    chapter.Path,
			Outline: chapter.Chapter,
		}
		if includeContent {
			content, path, err := ctx.Root.LoadChapterContent(chapter.Chapter.ID)
			if err != nil {
				return ToolResponse{OK: false, Count: 0, Warnings: []string{err.Error()}}
			}
			hit.Content, hit.ContentPath = content, path
		}
		return ToolResponse{OK: true, Count: 1, Results: []chapterHit{hit}}
	}
	return ToolResponse{OK: true, Count: 0, Results: []chapterHit{}}
}

func QueryOutlineRefs(outline *models.Outline, entityType, name string, chaptersOnly bool) ToolResponse {
	hits := make([]OutlineHit, 0)
	for _, chapter := range flattenChapters(outline) {
		reasons := chapterEntityReasons(chapter.Chapter, entityType, name)
		if len(reasons) == 0 {
			continue
		}
		hits = append(hits, OutlineHit{
			Type:    "chapter",
			ID:      chapter.Chapter.ID,
			Title:   chapter.Chapter.Title,
			Summary: chapter.Chapter.Summary,
			Path:    chapter.Path,
			Object:  chapter.Chapter,
			Reasons: reasons,
		})
	}
	if chaptersOnly {
		return ToolResponse{OK: true, Count: len(hits), Results: hits}
	}
	return ToolResponse{OK: true, Count: len(hits), Results: hits}
}

func queryChaptersByEntity(ctx ToolProjectContext, entityType, name string, includeContent bool) ToolResponse {
	outlineResp := QueryOutlineRefs(ctx.Outline, entityType, name, true)
	outlineHits, _ := outlineResp.Results.([]OutlineHit)
	chapters := make([]chapterHit, 0, len(outlineHits))
	for _, hit := range outlineHits {
		chapter := ctx.Outline.GetChapterByID(hit.ID)
		chapterHit := chapterHit{
			ID:      hit.ID,
			Title:   hit.Title,
			Summary: hit.Summary,
			Path:    hit.Path,
			Outline: chapter,
			Reasons: hit.Reasons,
		}
		if includeContent {
			content, path, err := ctx.Root.LoadChapterContent(hit.ID)
			if err != nil {
				return ToolResponse{OK: false, Count: 0, Warnings: []string{err.Error()}}
			}
			chapterHit.Content, chapterHit.ContentPath = content, path
		}
		chapters = append(chapters, chapterHit)
	}
	return ToolResponse{OK: true, Count: len(chapters), Results: chapters}
}

func queryStorylines(ctx ToolProjectContext, name string) ToolResponse {
	type storylineHit struct {
		Source  string      `json:"source"`
		Name    string      `json:"name"`
		Object  interface{} `json:"object"`
		Chapter string      `json:"chapter_id,omitempty"`
		Title   string      `json:"chapter_title,omitempty"`
		Path    OutlinePath `json:"path,omitempty"`
	}
	hits := make([]storylineHit, 0)
	if ctx.Setup != nil {
		for _, storyline := range ctx.Setup.Storylines {
			if name == "" || textMatches(storyline.Name, name) {
				hits = append(hits, storylineHit{Source: "setup", Name: storyline.Name, Object: storyline})
			}
		}
	}
	if ctx.Outline != nil {
		for _, chapter := range flattenChapters(ctx.Outline) {
			for _, advance := range chapter.Chapter.StorylineAdvances {
				if name == "" || textMatches(advance.StorylineName, name) {
					hits = append(hits, storylineHit{
						Source:  "chapter_advance",
						Name:    advance.StorylineName,
						Object:  advance,
						Chapter: chapter.Chapter.ID,
						Title:   chapter.Chapter.Title,
						Path:    chapter.Path,
					})
				}
			}
			for _, event := range chapter.Chapter.Events {
				if event.GetTargetType() != models.TargetTypeStoryline && event.Type != models.EventTypeStoryline {
					continue
				}
				target := event.GetTarget()
				if name == "" || textMatches(target, name) {
					hits = append(hits, storylineHit{
						Source:  "event",
						Name:    target,
						Object:  event,
						Chapter: chapter.Chapter.ID,
						Title:   chapter.Chapter.Title,
						Path:    chapter.Path,
					})
				}
			}
		}
	}
	return ToolResponse{OK: true, Count: len(hits), Results: hits}
}

func QueryEvents(outline *models.Outline, chapterID, volumeID, entityType, name string) ToolResponse {
	hits := make([]EventHit, 0)
	for _, chapter := range flattenChapters(outline) {
		if strings.TrimSpace(volumeID) != "" && !idMatches(chapter.Path.VolumeID, volumeID) {
			continue
		}
		if strings.TrimSpace(chapterID) != "" && !idMatches(chapter.Chapter.ID, chapterID) {
			continue
		}
		for eventIndex, event := range chapter.Chapter.Events {
			reasons := eventReasons(event, entityType, name)
			if strings.TrimSpace(entityType) != "" || strings.TrimSpace(name) != "" {
				if len(reasons) == 0 {
					continue
				}
			}
			hits = append(hits, EventHit{
				ChapterID:    chapter.Chapter.ID,
				ChapterTitle: chapter.Chapter.Title,
				EventIndex:   eventIndex,
				Path:         chapter.Path,
				Event:        event,
				Reasons:      reasons,
			})
		}
	}
	return ToolResponse{OK: true, Count: len(hits), Results: hits}
}
