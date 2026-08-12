package rpg

import (
	"strings"
	"testing"
)

func TestValidateEventSemantics(t *testing.T) {
	cases := []struct {
		name      string
		events    []StoryEvent
		wantIssue bool
	}{
		{
			name: "combat with controlled result change is accepted",
			events: []StoryEvent{
				{Type: "combat", Actor: "李侑", Action: "combat", Change: "defeated", Target: "林渊", TargetType: "character"},
			},
			wantIssue: false,
		},
		{
			name: "combat with prose change raises issue",
			events: []StoryEvent{
				{Type: "combat", Actor: "李侑", Action: "combat", Change: "冷却期内的兑换计划被截胡", Target: "林渊"},
			},
			wantIssue: true,
		},
		{
			name: "acquire with empty target raises issue",
			events: []StoryEvent{
				{Type: "item", Actor: "李侑", Action: "acquire", Change: "acquired"},
			},
			wantIssue: true,
		},
		{
			name: "use of consumable item is accepted",
			events: []StoryEvent{
				{Type: "item", Actor: "李侑", Action: "use", Change: "consumed", Target: "爆灵符"},
			},
			wantIssue: false,
		},
		{
			name: "use of non-consumable target raises issue",
			events: []StoryEvent{
				{Type: "item", Actor: "李侑", Action: "use", Change: "progressed", Target: "深度扫描"},
			},
			wantIssue: true,
		},
		{
			name: "combat with simulated death context is accepted",
			events: []StoryEvent{
				{Type: "combat", Actor: "顾恒", Action: "combat", Change: "模拟死亡", Target: "陆青禾"},
			},
			wantIssue: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ov := &OutlineValidator{
				Outline: &StoryOutline{
					Parts: []StoryPart{
						{Volumes: []StoryVolume{
							{Chapters: []StoryChapter{
								{ID: "P1-V1-C1", Events: tc.events},
							}},
						}},
					},
				},
			}
			ov.validateEventSemantics()
			got := false
			for _, iss := range ov.Issues {
				if iss.Type == "event_semantics" {
					got = true
					break
				}
			}
			if got != tc.wantIssue {
				t.Errorf("want issue=%v, got %v (issues: %d)", tc.wantIssue, got, len(ov.Issues))
			}
		})
	}
}

func TestEventSemanticsProseChangeExcerpt(t *testing.T) {
	// 确保受控词表里有 defeated/injured/escaped/completed 等结果语义
	all := strings.Join(outlineCombatResultChanges, ",")
	for _, want := range []string{"defeated", "injured", "escaped", "completed", "temporary_death"} {
		if !strings.Contains(all, want) {
			t.Errorf("outlineCombatResultChanges missing %q", want)
		}
	}
}
