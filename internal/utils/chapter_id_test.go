package utils

import "testing"

func TestExtractChapterNumberPreservesHistoricalNaming(t *testing.T) {
	cases := map[string]string{
		"chap_1_2_3": "3",        // underscore form: trailing segment
		"P1-V1-C2":   "P1-V1-C2", // hyphen form: returned unchanged (historical behavior)
		"single":     "single",   // no separator: returned unchanged
		"":           "",         // empty input
	}
	for in, want := range cases {
		if got := ExtractChapterNumber(in); got != want {
			t.Errorf("ExtractChapterNumber(%q) = %q, want %q", in, got, want)
		}
	}
}
