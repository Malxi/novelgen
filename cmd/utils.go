package cmd

import (
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/utils"
)

// loadDraftContent loads draft content for a chapter from the drafts directory.
func loadDraftContent(chapterID string) string {
	return project.LoadDraftContent(chapterID)
}

// loadFinalChapterContent loads a chapter's final markdown (project-relative).
func loadFinalChapterContent(chapter *models.Chapter) string {
	return project.LoadFinalChapterContent(chapter)
}

func extractChapterNumber(chapterID string) string {
	return utils.ExtractChapterNumber(chapterID)
}
