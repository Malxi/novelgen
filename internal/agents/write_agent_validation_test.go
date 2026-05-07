package agents

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestValidateWriteContentRejectsNonProseResponses(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1"}

	if err := validateWriteContent(chapter, `{"content":"still wrapped"}`, 0); err == nil {
		t.Fatal("expected JSON-as-prose to be rejected")
	}
	if err := validateWriteContent(chapter, "```json\n{\"content\":\"x\"}\n```", 0); err == nil {
		t.Fatal("expected fenced response to be rejected")
	}
}

func TestNarrativeUnitCountUsesCJKCharacters(t *testing.T) {
	content := strings.Repeat("章", 150)
	if err := validateWriteContent(&models.Chapter{ID: "P1-V1-C1"}, content, 200); err != nil {
		t.Fatalf("expected CJK prose to pass minimum length gate: %v", err)
	}
	if got := narrativeUnitCount(content); got != 150 {
		t.Fatalf("narrativeUnitCount = %d, want 150", got)
	}
}

func TestToCompactCarriesWritingStyleWithTruncatedReference(t *testing.T) {
	setup := &models.StorySetup{
		Genres: []string{"悬疑"},
		WritingStyle: models.WritingStyle{
			Name:             "冷峻克制",
			Principles:       []string{"少解释"},
			ReferenceExcerpt: strings.Repeat("风", compactStyleReferenceRuneLimit+20),
		},
	}

	compact := ToCompact(setup)
	if compact.WritingStyle.Name != "冷峻克制" {
		t.Fatalf("style name = %q", compact.WritingStyle.Name)
	}
	if len(compact.WritingStyle.Principles) != 1 || compact.WritingStyle.Principles[0] != "少解释" {
		t.Fatalf("principles = %#v", compact.WritingStyle.Principles)
	}
	if got := len([]rune(compact.WritingStyle.ReferenceExcerpt)); got != compactStyleReferenceRuneLimit+3 {
		t.Fatalf("reference excerpt runes = %d, want %d", got, compactStyleReferenceRuneLimit+3)
	}
	if !strings.HasSuffix(compact.WritingStyle.ReferenceExcerpt, "...") {
		t.Fatalf("reference excerpt should end with truncation marker")
	}
}
