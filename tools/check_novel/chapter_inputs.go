package main

import (
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/rpg"
	"novelgen/internal/store"
	"novelgen/internal/writersdl"
)

// findCheckNovelChapterInputs resolves the chapter markdown (and its recap when
// one exists) for every chapter in the RPG outline read from the book
// directory. Unlike writersdl.FindProjectChapterInputs it works from an explicit
// book path instead of assuming the process working directory is the novel
// project.
func findCheckNovelChapterInputs(bookPath string, outline *rpg.StoryOutline) ([]writersdl.WriteRPGDSLChapterInput, error) {
	book := store.At(bookPath)
	chapterFiles := map[string]string{}
	if outline != nil {
		for _, part := range outline.Parts {
			for _, volume := range part.Volumes {
				for _, chapter := range volume.Chapters {
					for _, path := range book.ChapterCandidates(chapter.ID) {
						if _, err := os.Stat(path); err == nil {
							chapterFiles[chapter.ID] = path
							break
						}
					}
				}
			}
		}
	}
	if len(chapterFiles) == 0 {
		pattern := filepath.Join(book.ChaptersDir(), "chapter-*.md")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			chapterID := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "chapter-"), filepath.Ext(file))
			chapterFiles[chapterID] = file
		}
	}

	inputs := make([]writersdl.WriteRPGDSLChapterInput, 0, len(chapterFiles))
	for chapterID, chapterPath := range chapterFiles {
		input := writersdl.WriteRPGDSLChapterInput{
			ChapterID:   chapterID,
			ChapterPath: chapterPath,
		}
		recapPath := book.RecapFile(chapterID)
		if _, err := os.Stat(recapPath); err == nil {
			input.RecapPath = recapPath
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}
