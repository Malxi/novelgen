package continuity

import (
	"strings"
	"testing"
)

func TestFormatReasonsSkipsBlankEntries(t *testing.T) {
	if got := formatReasons(nil); got != "" {
		t.Fatalf("formatReasons(nil) = %q, want empty", got)
	}
	if got := formatReasons([]string{"first", "  ", "second"}); got != "- first\n- second\n" {
		t.Fatalf("formatReasons = %q", got)
	}
}

func TestFixSummaryString(t *testing.T) {
	got := FixSummary{TeleportAttempted: true, TeleportAttempts: 2, TeleportPassed: true, CharacterPassed: true}.String()
	for _, want := range []string{"teleport_fix=true", "attempts=2", "passed=true", "character_fix=false"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FixSummary.String() missing %q: %s", want, got)
		}
	}
}

func TestBuildCharacterPresencePatchRequest(t *testing.T) {
	empty := BuildCharacterPresencePatchRequest(nil, nil)
	if !strings.Contains(empty, "CHARACTER_PRESENCE_PATCH") || strings.Contains(empty, "必须补出场的角色") {
		t.Fatalf("empty request should only carry the template: %q", empty)
	}

	got := BuildCharacterPresencePatchRequest([]string{"Mentor"}, []string{"Rival"})
	if !strings.Contains(got, "必须补出场的角色：Mentor") {
		t.Fatalf("request missing the missing-character list: %q", got)
	}
	if !strings.Contains(got, "Rival") {
		t.Fatalf("request missing the unexpected-character list: %q", got)
	}
}

func TestTransitionFixFeedback(t *testing.T) {
	if got := TransitionFixFeedback(nil); got != "" {
		t.Fatalf("TransitionFixFeedback(nil) = %q, want empty", got)
	}
	got := TransitionFixFeedback([]string{"opening teleports", "  "})
	for _, want := range []string{"复检失败", "- opening teleports", "重新生成 TRANSITION_BRIDGE"} {
		if !strings.Contains(got, want) {
			t.Fatalf("feedback missing %q: %s", want, got)
		}
	}
}
