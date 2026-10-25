package project

import "testing"

func TestSplitChapterParagraphsReturnsStableOneBasedBlocks(t *testing.T) {
	content := "# Opening\r\n\r\nFirst paragraph.\r\ncontinues here.\r\n\r\n\r\nThird paragraph.\r\n"
	paragraphs := SplitChapterParagraphs(content)
	if len(paragraphs) != 3 {
		t.Fatalf("paragraph count = %d, want 3: %#v", len(paragraphs), paragraphs)
	}
	if paragraphs[0].Index != 1 || paragraphs[0].Kind != "heading" || paragraphs[0].Text != "# Opening" {
		t.Fatalf("unexpected heading: %#v", paragraphs[0])
	}
	if paragraphs[1].Index != 2 || paragraphs[1].Text != "First paragraph.\ncontinues here." {
		t.Fatalf("unexpected second paragraph: %#v", paragraphs[1])
	}
	if paragraphs[2].Index != 3 || paragraphs[2].SHA256 == "" {
		t.Fatalf("unexpected third paragraph: %#v", paragraphs[2])
	}
}

func TestChapterContentSHA256CanonicalizesLineEndings(t *testing.T) {
	lf := "# Opening\n\nA paragraph.\n"
	crlf := "# Opening\r\n\r\nA paragraph.\r\n"
	if got, want := ChapterContentSHA256(crlf), ChapterContentSHA256(lf); got != want {
		t.Fatalf("CRLF hash = %s, LF hash = %s", got, want)
	}
}
