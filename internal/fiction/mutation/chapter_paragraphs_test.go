package mutation

import (
	"strings"
	"testing"

	"novelgen/internal/fiction/domain/project"
)

func TestApplyChapterParagraphEditsSupportsMinimalOperations(t *testing.T) {
	content := "# Opening\n\nA.\n\nB.\n\nC."
	paragraphs := project.SplitChapterParagraphs(content)
	edits := []toolChapterParagraphEdit{
		{Op: "insert_before", Index: 2, ExpectedSHA256: paragraphs[1].SHA256, Text: "Before A."},
		{Op: "replace", Index: 3, ExpectedSHA256: paragraphs[2].SHA256, Text: "B first.\n\nB second."},
		{Op: "insert_after", Index: 4, ExpectedSHA256: paragraphs[3].SHA256, Text: "After C."},
		{Op: "delete", Index: 4, ExpectedSHA256: paragraphs[3].SHA256},
	}
	got, changes, err := applyChapterParagraphEdits("P1-V1-C1", content, edits)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Opening\n\nBefore A.\n\nA.\n\nB first.\n\nB second.\n\nAfter C."
	if got != want {
		t.Fatalf("paragraph edit result:\n got %q\nwant %q", got, want)
	}
	if len(changes) != len(edits) {
		t.Fatalf("changes = %#v, want %d entries", changes, len(edits))
	}
}

func TestApplyChapterParagraphEditsRejectsHeadingAndConflictingEdits(t *testing.T) {
	content := "# Opening\n\nA."
	paragraphs := project.SplitChapterParagraphs(content)
	_, _, err := applyChapterParagraphEdits("P1-V1-C1", content, []toolChapterParagraphEdit{{
		Op: "replace", Index: 1, ExpectedSHA256: paragraphs[0].SHA256, Text: "Changed title",
	}})
	if err == nil || !strings.Contains(err.Error(), "chapter heading") {
		t.Fatalf("expected heading rejection, got %v", err)
	}

	_, _, err = applyChapterParagraphEdits("P1-V1-C1", content, []toolChapterParagraphEdit{
		{Op: "replace", Index: 2, ExpectedSHA256: paragraphs[1].SHA256, Text: "Changed."},
		{Op: "delete", Index: 2, ExpectedSHA256: paragraphs[1].SHA256},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting replace/delete") {
		t.Fatalf("expected conflicting edit rejection, got %v", err)
	}
}
