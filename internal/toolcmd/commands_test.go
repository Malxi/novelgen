package toolcmd

import "testing"

func TestChapterCheckCanonicalFlagOrder(t *testing.T) {
	got := ChapterCheck("P1-V1-C1", ChapterCheckOptions{
		Kind:        "quality",
		Category:    "continuity",
		MinPriority: "low",
		MaxIssues:   8,
		TargetWords: 1200,
	})
	want := `novelgen tool check quality --target chapter --scope chapter --id 'P1-V1-C1' --category continuity --min-priority low --max-issues 8 --target-words 1200`
	if got != want {
		t.Fatalf("ChapterCheck = %q, want %q", got, want)
	}
}

func TestChapterCheckDefaultsToAllAndOmitsEmptyFlags(t *testing.T) {
	got := ChapterCheck("P1-V1-C1", ChapterCheckOptions{MinPriority: "low", MaxIssues: 12})
	want := `novelgen tool check all --target chapter --scope chapter --id 'P1-V1-C1' --min-priority low --max-issues 12`
	if got != want {
		t.Fatalf("ChapterCheck = %q, want %q", got, want)
	}
}

func TestChapterDSLRefresh(t *testing.T) {
	got := ChapterDSLRefresh("P1-V1-C1")
	want := `novelgen tool refresh chapter-dsl --id 'P1-V1-C1'`
	if got != want {
		t.Fatalf("ChapterDSLRefresh = %q, want %q", got, want)
	}
}
