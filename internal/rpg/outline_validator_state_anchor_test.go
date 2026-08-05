package rpg

import (
	"strings"
	"testing"
)

func TestValidateStateAnchorAcceptsGateBreakthroughEvent(t *testing.T) {
	outline := &StoryOutline{Parts: []StoryPart{{
		ID: "P1",
		Volumes: []StoryVolume{{
			ID: "P1-V1",
			Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					StateAnchor: StoryStateAnchor{
						Cultivation: "一级基因适配者",
					},
				},
				{
					ID: "P1-V1-C2",
					StateAnchor: StoryStateAnchor{
						Cultivation: "二级基因适配者（不稳定58%）",
					},
					Events: []StoryEvent{{
						Type:       "gate",
						Characters: []string{"林野"},
						Subject:    "基因适配突破",
						Change:     "completed",
						Details:    "利用中级晶核和升级引导数据完成二级基因适配突破",
						Actor:      "林野",
						Action:     "breakthrough",
						Target:     "二级基因适配者",
						TargetType: "status",
						Context:    "废弃村庄地窖",
						Result:     "成功突破至二级基因适配者但稳定性跌至58%",
					}},
				},
			},
		}},
	}}}

	validator := NewOutlineValidator(outline)
	validator.validateStateAnchor()

	assertNoStateAnchorWarning(t, validator.Warnings, "P1-V1-C2")
}

func TestValidateStateAnchorStillWarnsWhenCultivationChangeHasNoEvent(t *testing.T) {
	outline := &StoryOutline{Parts: []StoryPart{{
		ID: "P1",
		Volumes: []StoryVolume{{
			ID: "P1-V1",
			Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					StateAnchor: StoryStateAnchor{
						Cultivation: "一级基因适配者",
					},
				},
				{
					ID: "P1-V1-C2",
					StateAnchor: StoryStateAnchor{
						Cultivation: "二级基因适配者",
					},
					Events: []StoryEvent{{
						Type:   "item",
						Change: "get",
						Target: "补给包",
					}},
				},
			},
		}},
	}}}

	validator := NewOutlineValidator(outline)
	validator.validateStateAnchor()

	assertHasStateAnchorWarning(t, validator.Warnings, "P1-V1-C2", "突破事件")
}

func assertNoStateAnchorWarning(t *testing.T, warnings []OutlineWarning, location string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Type == "state_anchor" && warning.Location == location {
			t.Fatalf("unexpected state_anchor warning at %s: %#v", location, warning)
		}
	}
}

func assertHasStateAnchorWarning(t *testing.T, warnings []OutlineWarning, location, text string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Type == "state_anchor" && warning.Location == location && strings.Contains(warning.Description, text) {
			return
		}
	}
	t.Fatalf("missing state_anchor warning at %s containing %q: %#v", location, text, warnings)
}
