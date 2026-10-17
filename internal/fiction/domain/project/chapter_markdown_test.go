package project

import (
	"testing"

	"novelgen/internal/contract/story"
)

func TestNormalizeChapterContentProducesSingleTitleHeading(t *testing.T) {
	t.Parallel()

	chapter := &models.Chapter{Title: "第一章 寒矿"}
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"no title gets one", "正文内容", "# 第一章 寒矿\n\n正文内容"},
		{"duplicate heading collapsed", "# 第一章 寒矿\n\n# 第一章 寒矿\n\n正文", "# 第一章 寒矿\n\n正文"},
		{"repeated plain prefix stripped", "第一章 寒矿：正文", "# 第一章 寒矿\n\n正文"},
		{"empty stays empty", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeChapterContent(chapter, tc.input); got != tc.want {
				t.Fatalf("NormalizeChapterContent = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChapterTitleHelpers(t *testing.T) {
	if !HasChapterTitle("# 标题\n\nbody", "标题") {
		t.Fatal("HasChapterTitle should match leading heading")
	}
	if HasChapterTitle("body only", "标题") {
		t.Fatal("HasChapterTitle should not match body without heading")
	}
	if got := StripChapterTitle("# 标题\n\nbody", "标题"); got != "body" {
		t.Fatalf("StripChapterTitle = %q", got)
	}
	// Quote/space normalization makes titles compare equal.
	if NormalizeChapterTitle("“标题”") != NormalizeChapterTitle("\"标题\"") {
		t.Fatal("NormalizeChapterTitle should normalize quote characters")
	}
}

func TestNarrativeUnitCountCountsCJKCharsElseWords(t *testing.T) {
	if got := NarrativeUnitCount("一二三四五"); got != 5 {
		t.Fatalf("CJK count = %d, want 5", got)
	}
	if got := NarrativeUnitCount("one two   three"); got != 3 {
		t.Fatalf("word count = %d, want 3", got)
	}
}

func TestChapterContentChangedIgnoresTitleDuplication(t *testing.T) {
	chapter := &models.Chapter{Title: "T"}
	if ChapterContentChanged(chapter, "# T\n\nbody", "T\n\nbody") {
		t.Fatal("normalized-equal content should not count as changed")
	}
	if !ChapterContentChanged(chapter, "body", "different") {
		t.Fatal("different content should count as changed")
	}
}
