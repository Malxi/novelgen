package rpg

import (
	"strings"
	"testing"
)

func TestValidateCharacterDeathContinuity(t *testing.T) {
	tests := []struct {
		name          string
		outline       *StoryOutline
		wantIssue     bool
		issueContains string
	}{
		{
			name: "death then revive allows later appearance",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:         "P1-V1-C1",
					Characters: []string{"林跃", "妖狼"},
					Events: []StoryEvent{{
						Action: "combat", Actor: "林跃", Target: "妖狼", TargetType: "character",
						Details: "林跃一剑斩杀妖狼", Result: "妖狼已死",
					}},
				},
				{
					ID:         "P1-V1-C2",
					Characters: []string{"林跃", "妖狼"},
					Events: []StoryEvent{{
						Action: "revive", Actor: "林跃", Target: "妖狼", TargetType: "character",
						Result: "妖狼借宝物复活",
					}},
				},
				{
					ID:         "P1-V1-C3",
					Characters: []string{"林跃", "妖狼"},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "dead character reappears in later chapter",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:         "P1-V1-C1",
					Characters: []string{"林跃", "妖狼"},
					Events: []StoryEvent{{
						Action: "combat", Actor: "林跃", Target: "妖狼", TargetType: "character",
						Result: "妖狼已死",
					}},
				},
				{
					ID:         "P1-V1-C2",
					Characters: []string{"林跃", "妖狼"},
				},
			}}}}}},
			wantIssue:     true,
			issueContains: "妖狼",
		},
		{
			name: "death detected from combat result text",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:         "P1-V1-C1",
					Characters: []string{"林跃", "王铁柱"},
					Events: []StoryEvent{{
						Action: "combat", Actor: "林跃", Target: "王铁柱", TargetType: "character",
						Details: "进入矿道深处，发现王铁柱已死", Result: "确认王铁柱已死",
					}},
				},
				{
					ID:         "P1-V1-C2",
					Characters: []string{"林跃", "王铁柱"},
				},
			}}}}}},
			wantIssue:     true,
			issueContains: "王铁柱",
		},
		{
			name: "devour action marks target dead",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:         "P1-V1-C1",
					Characters: []string{"林跃", "黑风兽"},
					Events: []StoryEvent{{
						Action: "devour", Actor: "林跃", Target: "黑风兽", TargetType: "character",
					}},
				},
				{
					ID:         "P1-V1-C2",
					Characters: []string{"林跃", "黑风兽"},
					Enemies:    []StoryOutlineEnemy{{Name: "黑风兽"}},
				},
			}}}}}},
			wantIssue:     true,
			issueContains: "黑风兽",
		},
		{
			name: "structured change temporary_death kills target",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:         "P1-V1-C1",
					Characters: []string{"林跃", "妖狼"},
					Events: []StoryEvent{{
						Action: "combat", Actor: "林跃", Target: "妖狼", TargetType: "character",
						Change: "temporary_death",
					}},
				},
				{
					ID:         "P1-V1-C2",
					Characters: []string{"林跃", "妖狼"},
				},
			}}}}}},
			wantIssue:     true,
			issueContains: "妖狼",
		},
		{
			name: "devour ability name in details with progress action is not death",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:         "P1-V1-C1",
					Characters: []string{"李侑", "林渊"},
					Events: []StoryEvent{{
						Action: "progress", Actor: "李侑", Target: "主线推进", TargetType: "storyline",
						Details: "林渊面对吞噬系统的终极诱惑，选择把王座砸碎而非坐上去",
					}},
				},
				{
					ID:         "P1-V1-C2",
					Characters: []string{"李侑", "林渊"},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "simulated death in combat is not real death",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:         "P1-V1-C1",
					Characters: []string{"林跃", "妖狼"},
					Events: []StoryEvent{{
						Action: "combat", Actor: "林跃", Target: "妖狼", TargetType: "character",
						Details: "模拟器中演练击杀妖狼，并非真实战斗", Result: "模拟结束",
					}},
				},
				{
					ID:         "P1-V1-C2",
					Characters: []string{"林跃", "妖狼"},
				},
			}}}}}},
			wantIssue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewOutlineValidator(tt.outline)
			validator.validateCharacterDeathContinuity()

			issue := issueByType(validator.Issues, "death_continuity")
			if tt.wantIssue {
				if issue == nil {
					t.Fatalf("missing death_continuity issue: %#v", validator.Issues)
				}
				if tt.issueContains != "" && !strings.Contains(issue.Description, tt.issueContains) {
					t.Fatalf("issue description %q does not contain %q", issue.Description, tt.issueContains)
				}
			} else if issue != nil {
				t.Fatalf("unexpected death_continuity issue: %#v", issue)
			}
		})
	}
}

func TestValidateCultivationContinuity(t *testing.T) {
	tests := []struct {
		name          string
		outline       *StoryOutline
		wantIssue     bool
		wantWarning   bool
		issueContains string
	}{
		{
			name: "cultivation downgrade without explanation raises major issue",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{ID: "P1-V1-C1", Characters: []string{"林跃"}, StateAnchor: StoryStateAnchor{Cultivation: "金丹后期"}},
				{ID: "P1-V1-C2", Characters: []string{"林跃"}, StateAnchor: StoryStateAnchor{Cultivation: "金丹初期"}},
			}}}}}},
			wantIssue:     true,
			issueContains: "金丹后期",
		},
		{
			name: "cultivation downgrade with explanation event is accepted",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:          "P1-V1-C1",
					Characters:  []string{"林跃"},
					StateAnchor: StoryStateAnchor{Cultivation: "筑基大圆满"},
				},
				{
					ID:          "P1-V1-C2",
					Characters:  []string{"林跃"},
					StateAnchor: StoryStateAnchor{Cultivation: "筑基初期"},
					Events: []StoryEvent{{
						Action: "afflict", Actor: "林跃", Target: "林跃", TargetType: "character",
						Details: "修炼反噬，修为跌落，损失一层小境界",
					}},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "cultivation upgrade without breakthrough warns",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{ID: "P1-V1-C1", Characters: []string{"林跃"}, StateAnchor: StoryStateAnchor{Cultivation: "练气"}},
				{ID: "P1-V1-C2", Characters: []string{"林跃"}, StateAnchor: StoryStateAnchor{Cultivation: "筑基"}},
			}}}}}},
			wantWarning: true,
		},
		{
			name: "same-tier layer upgrade with breakthrough event is accepted",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID:          "P1-V1-C1",
					Characters:  []string{"林跃"},
					StateAnchor: StoryStateAnchor{Cultivation: "练气一层后期"},
				},
				{
					ID:          "P1-V1-C2",
					Characters:  []string{"林跃"},
					StateAnchor: StoryStateAnchor{Cultivation: "练气二层"},
					Events: []StoryEvent{{
						Action: "breakthrough", Actor: "林跃", Target: "练气二层", TargetType: "status",
						Result: "成功突破至练气二层",
					}},
				},
			}}}}}},
			wantIssue:   false,
			wantWarning: false,
		},
		{
			name: "state anchor wins over statechange sighting",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{ID: "P1-V1-C1", Characters: []string{"林跃"}, StateAnchor: StoryStateAnchor{Cultivation: "练气四层中期"}},
				{
					ID:          "P1-V1-C2",
					Characters:  []string{"林跃"},
					StateAnchor: StoryStateAnchor{Cultivation: "练气四层中期"},
					StateChange: "林跃获得突破练气五层的关键材料，修炼进度提升",
				},
				{ID: "P1-V1-C3", Characters: []string{"林跃"}, StateAnchor: StoryStateAnchor{Cultivation: "练气四层中期"}},
			}}}}}},
			wantIssue:   false,
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewOutlineValidator(tt.outline)
			validator.validateCultivationContinuity()

			issue := issueByType(validator.Issues, "cultivation_continuity")
			if tt.wantIssue {
				if issue == nil {
					t.Fatalf("missing cultivation_continuity issue: %#v", validator.Issues)
				}
				if tt.issueContains != "" && !strings.Contains(issue.Description, tt.issueContains) {
					t.Fatalf("issue description %q does not contain %q", issue.Description, tt.issueContains)
				}
			} else if issue != nil {
				t.Fatalf("unexpected cultivation_continuity issue: %#v", issue)
			}

			warning := warningByType(validator.Warnings, "cultivation_continuity")
			if tt.wantWarning && warning == nil {
				t.Fatalf("missing cultivation_continuity warning: %#v", validator.Warnings)
			}
			if !tt.wantWarning && warning != nil {
				t.Fatalf("unexpected cultivation_continuity warning: %#v", warning)
			}
		})
	}
}

func TestValidateItemUsageContinuity(t *testing.T) {
	tests := []struct {
		name          string
		outline       *StoryOutline
		wantIssue     bool
		issueContains string
	}{
		{
			name: "use before acquire raises major issue",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "use", Actor: "林跃", Target: "疗伤丹药", TargetType: "item",
					}},
				},
			}}}}}},
			wantIssue:     true,
			issueContains: "疗伤丹药",
		},
		{
			name: "acquire then use is accepted",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "acquire", Actor: "林跃", Target: "疗伤丹药", TargetType: "item",
					}},
				},
				{
					ID: "P1-V1-C2",
					Events: []StoryEvent{{
						Action: "use", Actor: "林跃", Target: "疗伤丹药", TargetType: "item",
					}},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "craft counts as acquisition",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "craft", Actor: "林跃", Target: "聚灵阵盘", TargetType: "item",
					}},
				},
				{
					ID: "P1-V1-C2",
					Events: []StoryEvent{{
						Action: "use", Actor: "林跃", Target: "聚灵阵盘", TargetType: "item",
					}},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "details acquisition before use is accepted",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "acquire", Actor: "林跃",
						Details: "在废弃仓库中获得了一枚回气丹",
					}},
				},
				{
					ID: "P1-V1-C2",
					Events: []StoryEvent{{
						Action: "use", Actor: "林跃", Target: "回气丹", TargetType: "item",
						Details: "战斗中使用了回气丹恢复灵力",
					}},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "parenthesized acquire target normalizes to base name",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "acquire", Actor: "林跃", Target: "破阵符（签到应急奖励）", TargetType: "item",
					}},
				},
				{
					ID: "P1-V1-C2",
					Events: []StoryEvent{{
						Action: "use", Actor: "林跃", Target: "破阵符", TargetType: "item",
					}},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "count suffix acquisition matches annotated use",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "acquire", Actor: "林跃", Target: "隐息符×2", TargetType: "item",
					}},
				},
				{
					ID: "P1-V1-C2",
					Events: []StoryEvent{{
						Action: "use", Actor: "林跃", Target: "隐息符（最后一张）", TargetType: "item",
					}},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "concept item is not flagged when used before acquired",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "use", Actor: "林跃", Target: "灵石", TargetType: "item",
					}},
				},
			}}}}}},
			wantIssue: false,
		},
		{
			name: "consume counts as use",
			outline: &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "P1-V1", Chapters: []StoryChapter{
				{
					ID: "P1-V1-C1",
					Events: []StoryEvent{{
						Action: "consume", Actor: "林跃", Target: "聚气丹", TargetType: "item",
					}},
				},
			}}}}}},
			wantIssue:     true,
			issueContains: "聚气丹",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewOutlineValidator(tt.outline)
			validator.validateItemUsageContinuity()

			issue := issueByType(validator.Issues, "item_usage_continuity")
			if tt.wantIssue {
				if issue == nil {
					t.Fatalf("missing item_usage_continuity issue: %#v", validator.Issues)
				}
				if tt.issueContains != "" && !strings.Contains(issue.Description, tt.issueContains) {
					t.Fatalf("issue description %q does not contain %q", issue.Description, tt.issueContains)
				}
			} else if issue != nil {
				t.Fatalf("unexpected item_usage_continuity issue: %#v", issue)
			}
		})
	}
}

func issueByType(issues []OutlineIssue, typ string) *OutlineIssue {
	for i := range issues {
		if issues[i].Type == typ {
			return &issues[i]
		}
	}
	return nil
}

func warningByType(warnings []OutlineWarning, typ string) *OutlineWarning {
	for i := range warnings {
		if warnings[i].Type == typ {
			return &warnings[i]
		}
	}
	return nil
}
