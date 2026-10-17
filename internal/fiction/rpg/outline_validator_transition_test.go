package rpg

import (
	"strings"
	"testing"
)

func TestValidateTransitionsAcceptsReturnMovementBeat(t *testing.T) {
	outline := &StoryOutline{Parts: []StoryPart{{
		ID: "P1",
		Volumes: []StoryVolume{{
			ID: "P1-V1",
			Chapters: []StoryChapter{
				{
					ID:       "P1-V1-C1",
					Location: "万剑冢弟子房",
					Beats:    []string{"李侑拿到飞剑。"},
				},
				{
					ID:       "P1-V1-C2",
					Location: "外门区域",
					Beats:    []string{"紧接着上一章结尾，李侑从万剑冢弟子房回到外门区域。"},
				},
			},
		}},
	}}}

	validator := NewOutlineValidator(outline)
	validator.validateTransitions()

	assertNoTransitionWarning(t, validator.Warnings, "P1-V1-C2")
}

func TestValidateTransitionsWarnsWhenMovementBeatIsMissing(t *testing.T) {
	outline := &StoryOutline{Parts: []StoryPart{{
		ID: "P1",
		Volumes: []StoryVolume{{
			ID: "P1-V1",
			Chapters: []StoryChapter{
				{
					ID:       "P1-V1-C1",
					Location: "万剑冢弟子房",
					Beats:    []string{"李侑拿到飞剑。"},
				},
				{
					ID:       "P1-V1-C2",
					Location: "外门区域",
					Beats:    []string{"紧接着上一章结尾，李侑意识到名声开始发酵。"},
				},
			},
		}},
	}}}

	validator := NewOutlineValidator(outline)
	validator.validateTransitions()

	assertHasTransitionWarning(t, validator.Warnings, "P1-V1-C2", "缺少过渡描述")
}

func assertNoTransitionWarning(t *testing.T, warnings []OutlineWarning, location string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Type == "transition" && warning.Location == location {
			t.Fatalf("unexpected transition warning at %s: %#v", location, warning)
		}
	}
}

func assertHasTransitionWarning(t *testing.T, warnings []OutlineWarning, location, text string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Type == "transition" && warning.Location == location && strings.Contains(warning.Description, text) {
			return
		}
	}
	t.Fatalf("missing transition warning at %s containing %q: %#v", location, text, warnings)
}
