package transition

import (
	"strings"
	"testing"

	"novelgen/internal/contract/story"
)

func TestCheckTeleportOpeningIgnoresNilAndUnchangedLocation(t *testing.T) {
	if got := CheckTeleportOpening(nil, nil, ""); got.HasIssue {
		t.Fatalf("nil inputs must not flag: %#v", got)
	}

	prev := &models.ChapterRecap{Location: "Arena"}
	chapter := &models.Chapter{Location: "Arena", Summary: "Hero trains"}
	if got := CheckTeleportOpening(prev, chapter, "Hero keeps training."); got.HasIssue {
		t.Fatalf("same location must not flag: %#v", got)
	}
}

func TestCheckTeleportOpeningFlagsUndeclaredLocationJump(t *testing.T) {
	prev := &models.ChapterRecap{Location: "Arena"}
	chapter := &models.Chapter{Location: "Market", Summary: "Hero negotiates"}

	got := CheckTeleportOpening(prev, chapter, "Hero walks through the stalls.")

	if !got.HasIssue || len(got.Issues) == 0 {
		t.Fatalf("expected a teleport issue: %#v", got)
	}
	if !strings.Contains(got.Suggestions[0], "TRANSITION_BRIDGE") {
		t.Fatalf("suggestion should request a bridge patch: %#v", got.Suggestions)
	}
}

func TestCheckTeleportOpeningRespectsDeclaredTransitions(t *testing.T) {
	prev := &models.ChapterRecap{Location: "Arena"}

	bySummary := &models.Chapter{Location: "Market", Summary: "翌日来到集市"}
	if got := CheckTeleportOpening(prev, bySummary, "Fresh scene."); got.HasIssue {
		t.Fatalf("summary transition should suppress the flag: %#v", got)
	}

	byOpening := &models.Chapter{Location: "Market", Summary: "Hero negotiates"}
	if got := CheckTeleportOpening(prev, byOpening, "与此同时，另一边正在议价。"); got.HasIssue {
		t.Fatalf("opening transition should suppress the flag: %#v", got)
	}
}

func TestCheckTeleportOpeningFlagsOpeningLocationMarker(t *testing.T) {
	prev := &models.ChapterRecap{Location: "Arena"}
	chapter := &models.Chapter{Location: "", Summary: "Hero negotiates"}

	got := CheckTeleportOpening(prev, chapter, "地点：集市\n\nHero walks in.")

	if !got.HasIssue {
		t.Fatalf("an explicit opening location marker should flag: %#v", got)
	}
}

func TestExtractInsertAndValidateBridge(t *testing.T) {
	text := "前言\n<TRANSITION_BRIDGE>\n  桥段正文  \n</TRANSITION_BRIDGE>\n后记"
	bridge, ok := ExtractTransitionBridge(text)
	if !ok || bridge != "桥段正文" {
		t.Fatalf("ExtractTransitionBridge = %q/%v", bridge, ok)
	}
	if _, ok := ExtractTransitionBridge("<TRANSITION_BRIDGE>   </TRANSITION_BRIDGE>"); ok {
		t.Fatal("empty bridge block should not extract")
	}
	if _, ok := ExtractTransitionBridge("no block here"); ok {
		t.Fatal("missing block should not extract")
	}

	if got := InsertTransitionBridge("正文", "桥段"); got != "桥段\n\n正文" {
		t.Fatalf("InsertTransitionBridge = %q", got)
	}
	if got := InsertTransitionBridge("正文", "   "); got != "正文" {
		t.Fatalf("blank bridge must not change the text: %q", got)
	}
	if got := InsertTransitionBridge("  ", "桥段"); got != "桥段" {
		t.Fatalf("empty original should become the bridge: %q", got)
	}
}

func TestValidateTransitionBridgeOutput(t *testing.T) {
	if ok, reasons := ValidateTransitionBridgeOutput("   "); ok || len(reasons) == 0 {
		t.Fatalf("empty output should fail: %v %#v", ok, reasons)
	}
	if ok, _ := ValidateTransitionBridgeOutput("no patch block"); ok {
		t.Fatal("missing block should fail")
	}
	if ok, _ := ValidateTransitionBridgeOutput("<TRANSITION_BRIDGE>\n\n</TRANSITION_BRIDGE>"); ok {
		t.Fatal("empty block should fail")
	}
	if ok, reasons := ValidateTransitionBridgeOutput("<TRANSITION_BRIDGE>桥段</TRANSITION_BRIDGE>"); !ok || len(reasons) != 0 {
		t.Fatalf("leading block should pass: %v %#v", ok, reasons)
	}

	late := strings.Repeat("前文", 800) + "<TRANSITION_BRIDGE>桥段</TRANSITION_BRIDGE>"
	ok, reasons := ValidateTransitionBridgeOutput(late)
	if ok || len(reasons) == 0 {
		t.Fatalf("late block should be reported: %v %#v", ok, reasons)
	}
}

func TestFirstNCharsIsRuneSafe(t *testing.T) {
	if got := firstNChars("abcdef", 3); got != "abc" {
		t.Fatalf("firstNChars = %q", got)
	}
	if got := firstNChars("短", 5); got != "短" {
		t.Fatalf("short input should pass through: %q", got)
	}
	if got := firstNChars("你好世界", 2); got != "你好" {
		t.Fatalf("CJK truncation should stay on rune boundaries: %q", got)
	}
}
