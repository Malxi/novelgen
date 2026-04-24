package benchmark

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

var (
	chapterIDPattern = regexp.MustCompile(`^chapter-P(\d+)-V(\d+)-C(\d+)\.md$`)
	simpleIDPattern  = regexp.MustCompile(`^chapter-(\d+)\.md$`)
)

type chapterSortKey struct {
	part      int
	volume    int
	chapter   int
	valid     bool
	simple    bool
	simpleNum int
	name      string
}

func sortChapterFilenames(files []string) {
	sort.Slice(files, func(i, j int) bool {
		a := parseChapterSortKey(files[i])
		b := parseChapterSortKey(files[j])

		// Prefer fully parsed IDs.
		if a.valid && b.valid {
			if a.part != b.part {
				return a.part < b.part
			}
			if a.volume != b.volume {
				return a.volume < b.volume
			}
			return a.chapter < b.chapter
		}

		// Then prefer simple numeric chapter IDs.
		if a.simple && b.simple {
			return a.simpleNum < b.simpleNum
		}
		if a.valid || a.simple {
			return true
		}
		if b.valid || b.simple {
			return false
		}

		// Fallback for unknown formats.
		return a.name < b.name
	})
}

func parseChapterSortKey(filename string) chapterSortKey {
	base := filepath.Base(filename)
	key := chapterSortKey{name: base}

	if m := chapterIDPattern.FindStringSubmatch(base); len(m) == 4 {
		part, err1 := strconv.Atoi(m[1])
		volume, err2 := strconv.Atoi(m[2])
		chapter, err3 := strconv.Atoi(m[3])
		if err1 == nil && err2 == nil && err3 == nil {
			key.part = part
			key.volume = volume
			key.chapter = chapter
			key.valid = true
			return key
		}
	}

	if m := simpleIDPattern.FindStringSubmatch(base); len(m) == 2 {
		num, err := strconv.Atoi(m[1])
		if err == nil {
			key.simple = true
			key.simpleNum = num
		}
	}

	return key
}
