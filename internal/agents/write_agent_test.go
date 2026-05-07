package agents

import (
	"strings"
	"testing"
)

func TestFormatChapterContextIncludesCraftContext(t *testing.T) {
	context := &ChapterContext{
		Craft: "ORGANIZATIONS:\n- Ember Guild: goals=control the mine",
	}

	got := formatChapterContext(context)
	if !strings.Contains(got, "CRAFT CONTEXT:") {
		t.Fatalf("expected craft context header, got %q", got)
	}
	if !strings.Contains(got, "Ember Guild") {
		t.Fatalf("expected organization summary, got %q", got)
	}
}
