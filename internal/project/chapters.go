package project

import (
	"fmt"
	"strconv"
	"strings"

	"novelgen/internal/logic"
	"novelgen/internal/models"
)

// ChapterFileName returns the canonical final-chapter filename for a chapter ID
// (for example "chapter-P1-V1-C1.md").
func ChapterFileName(chapterID string) string {
	return fmt.Sprintf("chapter-%s.md", chapterID)
}

// SaveFinalChapter writes chapters/chapter-<id>.md with canonical content
// (exactly one "# <title>" heading).
func (b *Book) SaveFinalChapter(chapter *models.Chapter, content string) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	return b.SaveChapterContent(chapter.ID, NormalizeChapterContent(chapter, content))
}

// LoadFinalChapterContent reads a chapter's final markdown, trying the full-id
// filename first and the numeric filename as a compatibility fallback. It
// returns "" when the project root or the chapter file is missing.
func (b *Book) LoadFinalChapterContent(chapter *models.Chapter) string {
	if chapter == nil {
		return ""
	}
	content, _ := b.LoadChapterContent(chapter.ID)
	return content
}

// LoadDraftContent reads drafts/<chapterID>.md, returning "" when missing.
func (b *Book) LoadDraftContent(chapterID string) string {
	return b.Root.LoadDraftContent(chapterID)
}

// SaveDraft writes drafts/<chapterID>.md with a markdown title header.
//
// This used to write relative to the working directory rather than to the
// project, so running the legacy draft command from a subdirectory scattered
// drafts outside the novel. Drafts now land in the project like every other
// artifact.
func (b *Book) SaveDraft(chapter *models.Chapter, draft string) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	content := fmt.Sprintf("# %s\n\n%s\n", chapter.Title, draft)
	return b.SaveDraftContent(chapter.ID, content)
}

// LoadAllDrafts returns every drafts/*.md keyed by chapter ID.
func (b *Book) LoadAllDrafts() map[string]string {
	return b.Root.LoadAllDrafts()
}

// SaveFinalChapter writes a chapter through the ambient project root.
func SaveFinalChapter(chapter *models.Chapter, content string) error {
	book, err := Default()
	if err != nil {
		return err
	}
	return book.SaveFinalChapter(chapter, content)
}

// LoadFinalChapterContent reads a chapter through the ambient project root.
func LoadFinalChapterContent(chapter *models.Chapter) string {
	book, err := Default()
	if err != nil {
		return ""
	}
	return book.LoadFinalChapterContent(chapter)
}

// LoadDraftContent reads a draft through the ambient project root.
func LoadDraftContent(chapterID string) string {
	book, err := Default()
	if err != nil {
		return ""
	}
	return book.LoadDraftContent(chapterID)
}

// SaveDraft writes a draft through the ambient project root.
func SaveDraft(chapter *models.Chapter, draft string) error {
	book, err := Default()
	if err != nil {
		return err
	}
	return book.SaveDraft(chapter, draft)
}

// LoadAllDrafts reads every draft through the ambient project root.
func LoadAllDrafts() map[string]string {
	book, err := Default()
	if err != nil {
		return map[string]string{}
	}
	return book.LoadAllDrafts()
}

// SelectChapters resolves the chapter-selection flags shared by the draft,
// recap and write commands.
func SelectChapters(outline *models.Outline, chapterFlag, volumeFlag, partFlag string, allFlag bool) ([]*models.Chapter, error) {
	if outline == nil || len(outline.Parts) == 0 {
		return nil, fmt.Errorf("outline is empty")
	}

	if allFlag {
		return AllChapters(outline), nil
	}

	if chapterFlag == "" {
		return nil, fmt.Errorf("please specify --chapter or --all")
	}

	// Range form such as "1-5" (but not a P-prefixed id).
	if strings.Contains(chapterFlag, "-") && !strings.Contains(strings.ToUpper(chapterFlag), "P") {
		parts := strings.Split(chapterFlag, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(parts[0])
			end, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				return chapterRange(outline, start, end)
			}
		}
	}

	chapter, err := resolveTargetChapter(outline, chapterFlag, volumeFlag, partFlag)
	if err != nil {
		return nil, err
	}
	return []*models.Chapter{chapter}, nil
}

func chapterRange(outline *models.Outline, start, end int) ([]*models.Chapter, error) {
	allChapters := AllChapters(outline)
	if start < 1 || start > len(allChapters) {
		return nil, fmt.Errorf("start chapter %d out of range (1-%d)", start, len(allChapters))
	}
	if end < 1 || end > len(allChapters) {
		return nil, fmt.Errorf("end chapter %d out of range (1-%d)", end, len(allChapters))
	}
	if start > end {
		return nil, fmt.Errorf("start chapter %d must be <= end chapter %d", start, end)
	}
	return allChapters[start-1 : end], nil
}

func resolveTargetChapter(outline *models.Outline, chapterFlag, volumeFlag, partFlag string) (*models.Chapter, error) {
	if outline == nil || len(outline.Parts) == 0 {
		return nil, fmt.Errorf("outline is empty")
	}
	idManager := logic.NewIDManager(outline)
	chapterID, err := idManager.ResolveChapterID(chapterFlag, partFlag, volumeFlag)
	if err != nil {
		return nil, err
	}
	chapter, _, _ := idManager.GetChapterByID(chapterID)
	if chapter == nil {
		return nil, fmt.Errorf("chapter not found: %s", chapterID)
	}
	return chapter, nil
}
