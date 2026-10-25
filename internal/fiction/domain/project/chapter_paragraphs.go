package project

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ChapterParagraph is one blank-line-delimited block in the saved chapter
// manuscript. Index is 1-based so agents can refer to the same paragraph shown
// by the query tool without translating array offsets.
type ChapterParagraph struct {
	Index  int    `json:"index"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Text   string `json:"text"`
}

// ChapterContentSHA256 is the concurrency token for a paragraph patch. Line
// endings are canonicalized so a CRLF checkout and an LF save represent the
// same project state.
func ChapterContentSHA256(content string) string {
	sum := sha256.Sum256([]byte(canonicalChapterLineEndings(content)))
	return fmt.Sprintf("%x", sum[:])
}

// ChapterParagraphSHA256 hashes one paragraph using the same text the query
// tool returns for that paragraph.
func ChapterParagraphSHA256(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(canonicalChapterLineEndings(text))))
	return fmt.Sprintf("%x", sum[:])
}

// SplitChapterParagraphs returns blank-line-delimited Markdown blocks. Markdown
// headings and prose are both blocks; the first block is classified as a
// heading when it starts with "# ". Whitespace-only separators are collapsed,
// which matches the canonical chapter save format.
func SplitChapterParagraphs(content string) []ChapterParagraph {
	content = canonicalChapterLineEndings(content)
	lines := strings.Split(content, "\n")
	blocks := make([]string, 0)
	current := make([]string, 0)
	flush := func() {
		if len(current) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(current, "\n"))
		if text != "" {
			blocks = append(blocks, text)
		}
		current = current[:0]
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()

	paragraphs := make([]ChapterParagraph, 0, len(blocks))
	for i, text := range blocks {
		kind := "prose"
		if strings.HasPrefix(text, "# ") {
			kind = "heading"
		}
		paragraphs = append(paragraphs, ChapterParagraph{
			Index:  i + 1,
			Kind:   kind,
			SHA256: ChapterParagraphSHA256(text),
			Text:   text,
		})
	}
	return paragraphs
}

func canonicalChapterLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}
