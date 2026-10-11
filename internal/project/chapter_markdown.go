package project

import (
	"strings"
	"unicode"

	"novelgen/internal/models"
)

// StripChapterTitle removes a leading "# <title>" heading when it matches the
// chapter title, returning the trimmed body.
func StripChapterTitle(content, title string) string {
	clean := strings.TrimSpace(content)
	if title == "" {
		return clean
	}
	if !HasChapterTitle(clean, title) {
		return clean
	}
	_, rest, _ := strings.Cut(clean, "\n")
	return strings.TrimSpace(rest)
}

// HasChapterTitle reports whether content starts with a markdown heading whose
// text matches title (after normalization).
func HasChapterTitle(content, title string) bool {
	clean := strings.TrimSpace(content)
	if title == "" || !strings.HasPrefix(clean, "#") {
		return false
	}
	line, _, _ := strings.Cut(clean, "\n")
	lineTitle := strings.TrimSpace(strings.TrimLeft(line, "# \t"))
	return NormalizeChapterTitle(lineTitle) == NormalizeChapterTitle(title)
}

// HasRepeatedChapterTitlePrefix reports whether content repeats the chapter
// title as a leading prefix (with or without a "#" heading).
func HasRepeatedChapterTitlePrefix(content, title string) bool {
	content = strings.TrimSpace(content)
	title = strings.TrimSpace(title)
	return MatchingChapterTitlePrefixLen(content, title) > 0
}

// StripRepeatedChapterTitlePrefix removes a leading chapter-title prefix and any
// following separator punctuation, returning trimmed content.
func StripRepeatedChapterTitlePrefix(content, title string) string {
	prefixLen := MatchingChapterTitlePrefixLen(strings.TrimSpace(content), title)
	if prefixLen <= 0 {
		return strings.TrimSpace(content)
	}
	rest := strings.TrimSpace(strings.TrimSpace(content)[prefixLen:])
	rest = strings.TrimLeft(rest, " \t\r\n:：-—,，.。")
	return strings.TrimSpace(rest)
}

// MatchingChapterTitlePrefixLen returns the byte length of the leading run of
// content that normalizes to title, or 0 when it does not match.
func MatchingChapterTitlePrefixLen(content, title string) int {
	content = strings.TrimSpace(content)
	title = strings.TrimSpace(title)
	if content == "" || title == "" {
		return 0
	}
	titleRunes := []rune(title)
	contentRunes := []rune(content)
	if len(contentRunes) < len(titleRunes) {
		return 0
	}
	candidate := string(contentRunes[:len(titleRunes)])
	if NormalizeChapterTitle(candidate) != NormalizeChapterTitle(title) {
		return 0
	}
	return len(candidate)
}

// NormalizeChapterTitle normalizes quote/colon punctuation and drops all
// whitespace so titles can be compared across formatting differences.
func NormalizeChapterTitle(value string) string {
	replacer := strings.NewReplacer(
		"“", "\"",
		"”", "\"",
		"‘", "'",
		"’", "'",
		"：", ":",
	)
	value = replacer.Replace(strings.TrimSpace(value))
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
}

// NarrativeUnitCount counts CJK characters when the text is mostly CJK,
// otherwise whitespace-separated words.
func NarrativeUnitCount(content string) int {
	hasCJK := false
	cjkCount := 0
	for _, r := range content {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			hasCJK = true
			cjkCount++
		}
	}
	if hasCJK {
		return cjkCount
	}
	return len(strings.Fields(content))
}

// NormalizeChapterContent produces the canonical stored form of a chapter body:
// exactly one leading "# <title>" heading followed by the deduplicated body.
func NormalizeChapterContent(chapter *models.Chapter, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if chapter == nil || strings.TrimSpace(chapter.Title) == "" {
		return content
	}
	title := strings.TrimSpace(chapter.Title)
	titlePrefix := "# " + title
	if HasChapterTitle(content, title) {
		body := StripChapterTitle(content, title)
		for HasChapterTitle(body, title) {
			body = StripChapterTitle(body, title)
		}
		if cleaned := StripRepeatedChapterTitlePrefix(body, title); cleaned != body {
			return titlePrefix + "\n\n" + cleaned
		}
		if body == "" {
			return titlePrefix
		}
		return titlePrefix + "\n\n" + body
	}
	return titlePrefix + "\n\n" + StripRepeatedChapterTitlePrefix(content, title)
}

// ChapterContentChanged reports whether two chapter bodies differ after
// canonical normalization.
func ChapterContentChanged(chapter *models.Chapter, before, after string) bool {
	before = strings.TrimSpace(NormalizeChapterContent(chapter, before))
	after = strings.TrimSpace(NormalizeChapterContent(chapter, after))
	return before != after
}

// ChapterNarrativeUnits counts narrative units of the body with the chapter
// title stripped, for logging.
func ChapterNarrativeUnits(chapter *models.Chapter, content string) int {
	title := ""
	if chapter != nil {
		title = chapter.Title
	}
	return NarrativeUnitCount(StripChapterTitle(content, title))
}
