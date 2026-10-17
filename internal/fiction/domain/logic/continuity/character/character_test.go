package character

import (
	"strings"
	"testing"

	"novelgen/internal/contract/story"
)

func TestCheckCharacterPresenceFlagsMissingExpected(t *testing.T) {
	chapter := &models.Chapter{Characters: []string{"Hero", "Mentor"}}

	got := CheckCharacterPresence(chapter, "Hero walks alone.", []string{"Hero", "Mentor"})

	if !got.HasIssue || len(got.Issues) != 1 || !strings.Contains(got.Issues[0], "Mentor") {
		t.Fatalf("expected a single missing-character issue: %#v", got)
	}
	if !strings.Contains(got.Suggestions[0], "CHARACTER_PRESENCE_PATCH") {
		t.Fatalf("suggestion should request a presence patch: %#v", got.Suggestions)
	}
}

func TestCheckCharacterPresenceIgnoresNilOrBlankInputs(t *testing.T) {
	if got := CheckCharacterPresence(nil, "text", nil); got.HasIssue {
		t.Fatalf("nil chapter must not flag: %#v", got)
	}
	if got := CheckCharacterPresence(&models.Chapter{Characters: []string{"Hero"}}, "   ", nil); got.HasIssue {
		t.Fatalf("blank draft must not flag: %#v", got)
	}
}

func TestCheckCharacterPresenceFlagsUnexpectedOpeningCharacter(t *testing.T) {
	chapter := &models.Chapter{Characters: []string{"Hero"}}
	known := []string{"Hero", "Rival"}

	oneMention := CheckCharacterPresence(chapter, "Rival watches. Hero acts.", known)
	if oneMention.HasIssue {
		t.Fatalf("a single opening mention should stay below the threshold: %#v", oneMention)
	}

	twoMentions := CheckCharacterPresence(chapter, "Rival watches. Rival waits. Hero acts.", known)
	if !twoMentions.HasIssue || !strings.Contains(twoMentions.Issues[0], "Rival") {
		t.Fatalf("two opening mentions should flag: %#v", twoMentions)
	}
}

func TestCheckCharacterPresenceDetailedReportsStructuredDetails(t *testing.T) {
	chapter := &models.Chapter{Characters: []string{"Hero", "Mentor"}}
	_, details := CheckCharacterPresenceDetailed(chapter, "Rival blocks. Rival strikes. Hero dodges.", []string{"Hero", "Rival"})

	if len(details.MissingExpected) != 1 || details.MissingExpected[0] != "Mentor" {
		t.Fatalf("MissingExpected = %#v", details.MissingExpected)
	}
	if len(details.UnexpectedInOpen) != 1 || details.UnexpectedInOpen[0] != "Rival" {
		t.Fatalf("UnexpectedInOpen = %#v", details.UnexpectedInOpen)
	}
}

func TestExtractInsertAndValidatePresencePatch(t *testing.T) {
	text := "<CHARACTER_PRESENCE_PATCH>\n  补丁正文  \n</CHARACTER_PRESENCE_PATCH>"
	patch, ok := ExtractCharacterPresencePatch(text)
	if !ok || patch != "补丁正文" {
		t.Fatalf("ExtractCharacterPresencePatch = %q/%v", patch, ok)
	}
	if _, ok := ExtractCharacterPresencePatch("<CHARACTER_PRESENCE_PATCH></CHARACTER_PRESENCE_PATCH>"); ok {
		t.Fatal("empty patch block should not extract")
	}

	if got := InsertCharacterPresencePatch("正文", "补丁"); got != "补丁\n\n正文" {
		t.Fatalf("InsertCharacterPresencePatch = %q", got)
	}
	if got := InsertCharacterPresencePatch("正文", "  "); got != "正文" {
		t.Fatalf("blank patch must not change the text: %q", got)
	}

	if ok, _ := ValidateCharacterPresencePatchOutput("no patch"); ok {
		t.Fatal("missing block should fail validation")
	}
	if ok, reasons := ValidateCharacterPresencePatchOutput(text); !ok || len(reasons) != 0 {
		t.Fatalf("well-formed block should validate: %v %#v", ok, reasons)
	}
}

func TestFirstNRunesIsRuneSafe(t *testing.T) {
	if got := firstNRunes("你好世界", 2); got != "你好" {
		t.Fatalf("firstNRunes = %q", got)
	}
	if got := firstNRunes("短", 10); got != "短" {
		t.Fatalf("short input should pass through: %q", got)
	}
}
