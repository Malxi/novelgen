package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/utils"
)

// ChapterFileName returns the canonical final-chapter filename for a chapter ID
// (for example "chapter-P1-V1-C1.md").
func ChapterFileName(chapterID string) string {
	return fmt.Sprintf("chapter-%s.md", chapterID)
}

// SaveFinalChapter writes chapters/chapter-<id>.md with canonical content
// (exactly one "# <title>" heading).
func SaveFinalChapter(chapter *models.Chapter, content string) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	root, err := FindRoot()
	if err != nil {
		return err
	}
	chaptersDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		return fmt.Errorf("failed to create chapters directory: %w", err)
	}
	content = NormalizeChapterContent(chapter, content)
	return os.WriteFile(filepath.Join(chaptersDir, ChapterFileName(chapter.ID)), []byte(content), 0644)
}

// LoadFinalChapterContent reads a chapter's final markdown, trying the full-id
// filename first and the numeric filename as a compatibility fallback. It
// returns "" when the project root or the chapter file is missing.
func LoadFinalChapterContent(chapter *models.Chapter) string {
	if chapter == nil {
		return ""
	}
	root, err := FindRoot()
	if err != nil {
		return ""
	}
	for _, path := range []string{
		filepath.Join(root, "chapters", ChapterFileName(chapter.ID)),
		filepath.Join(root, "chapters", ChapterFileName(utils.ExtractChapterNumber(chapter.ID))),
	} {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return ""
}

// LoadDraftContent reads drafts/<chapterID>.md, returning "" when missing.
func LoadDraftContent(chapterID string) string {
	root, err := FindRoot()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "drafts", chapterID+".md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// SaveDraft writes drafts/<chapterID>.md with a markdown title header. It writes
// relative to the current working directory (historical CLI behavior).
func SaveDraft(chapter *models.Chapter, draft string) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	draftsDir := "drafts"
	if err := os.MkdirAll(draftsDir, 0755); err != nil {
		return fmt.Errorf("failed to create drafts directory: %w", err)
	}
	content := fmt.Sprintf("# %s\n\n%s\n", chapter.Title, draft)
	return os.WriteFile(filepath.Join(draftsDir, chapter.ID+".md"), []byte(content), 0644)
}

// LoadAllDrafts returns every drafts/*.md keyed by chapter ID.
func LoadAllDrafts() map[string]string {
	drafts := make(map[string]string)
	root, err := FindRoot()
	if err != nil {
		return drafts
	}
	draftsDir := filepath.Join(root, "drafts")
	entries, err := os.ReadDir(draftsDir)
	if err != nil {
		return drafts
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		chapterID := strings.TrimSuffix(entry.Name(), ".md")
		if data, err := os.ReadFile(filepath.Join(draftsDir, entry.Name())); err == nil {
			drafts[chapterID] = string(data)
		}
	}
	return drafts
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
