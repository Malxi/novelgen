package project

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestKnownCharacterNamesDedupesSortsAndSkipsBlank(t *testing.T) {
	t.Parallel()

	outline := &models.Outline{Parts: []models.Part{{
		Volumes: []models.Volume{{
			Chapters: []models.Chapter{
				{ID: "P1-V1-C1", Characters: []string{"B", "A", " "}},
				{ID: "P1-V1-C2", Characters: []string{"A", "C"}},
			},
		}},
	}}}

	got := KnownCharacterNames(outline)
	if strings.Join(got, ",") != "A,B,C" {
		t.Fatalf("KnownCharacterNames = %v, want [A B C]", got)
	}
	if got := KnownCharacterNames(nil); got != nil {
		t.Fatalf("nil outline should return nil, got %v", got)
	}
}
