package rpg

import (
	"strings"
	"testing"
)

func TestValidateTimelineMonotonicity(t *testing.T) {
	cases := []struct {
		name     string
		chapters []StoryChapter
		wantN    int
	}{
		{
			name: "monotonic days pass",
			chapters: []StoryChapter{
				{ID: "C1", Title: "第1章", Timeline: StoryChapterTimeline{Anchor: "第1天"}},
				{ID: "C2", Title: "第2章", Timeline: StoryChapterTimeline{Anchor: "第2天"}},
				{ID: "C3", Title: "第3章", Timeline: StoryChapterTimeline{Anchor: "第3天"}},
			},
			wantN: 0,
		},
		{
			name: "day regression flagged",
			chapters: []StoryChapter{
				{ID: "C1", Title: "第1章", Timeline: StoryChapterTimeline{Anchor: "第10天"}},
				{ID: "C2", Title: "第2章", Timeline: StoryChapterTimeline{Anchor: "第12天"}},
				{ID: "C3", Title: "第3章", Timeline: StoryChapterTimeline{Anchor: "第9天"}},
				{ID: "C4", Title: "第4章", Timeline: StoryChapterTimeline{Anchor: "第13天"}},
			},
			wantN: 1,
		},
		{
			name: "flashback allowed with context",
			chapters: []StoryChapter{
				{ID: "C1", Title: "第1章", Timeline: StoryChapterTimeline{Anchor: "第26天上午"}},
				{ID: "C2", Title: "第2章", Timeline: StoryChapterTimeline{Anchor: "第23天", Duration: "回溯顾恒被调查期间的一周经历"}},
			},
			wantN: 0,
		},
		{
			name: "missing anchors skipped",
			chapters: []StoryChapter{
				{ID: "C1", Title: "第1章", Timeline: StoryChapterTimeline{Anchor: ""}},
				{ID: "C2", Title: "第2章", Timeline: StoryChapterTimeline{Anchor: "第1天"}},
				{ID: "C3", Title: "第3章", Timeline: StoryChapterTimeline{Anchor: ""}},
			},
			wantN: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ov := NewOutlineValidator(nil)
			ov.Outline = &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "V1", Chapters: tc.chapters}}}}}
			ov.validateTimelineMonotonicity()
			if len(ov.Issues) != tc.wantN {
				t.Errorf("want %d issues, got %d: %+v", tc.wantN, len(ov.Issues), ov.Issues)
			}
		})
	}
}

func TestValidateTitleUniqueness(t *testing.T) {
	// 无序号后缀的重名
	ov := NewOutlineValidator(nil)
	chs := []StoryChapter{
		{ID: "C1", Title: "第1章：测试标题"},
		{ID: "C2", Title: "第2章：测试标题"},
		{ID: "C3", Title: "第3章：不同标题"},
	}
	ov.Outline = &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "V1", Chapters: chs}}}}}
	ov.validateTitleUniqueness()
	if len(ov.Issues) != 1 {
		t.Errorf("want 1 duplicate issue, got %d", len(ov.Issues))
	}

	// 带序号后缀的不算重复
	ov2 := NewOutlineValidator(nil)
	chs2 := []StoryChapter{
		{ID: "C1", Title: "第1章：测试（一）"},
		{ID: "C2", Title: "第2章：测试（二）"},
	}
	ov2.Outline = &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "V1", Chapters: chs2}}}}}
	ov2.validateTitleUniqueness()
	if len(ov2.Issues) != 0 {
		t.Errorf("suffixed titles should not be duplicates, got %d", len(ov2.Issues))
	}
}

func TestValidateAnchorFormat(t *testing.T) {
	ov := NewOutlineValidator(nil)
	chs := []StoryChapter{
		{ID: "C1", Timeline: StoryChapterTimeline{Anchor: "第21章"}},
		{ID: "C2", Timeline: StoryChapterTimeline{Anchor: "第22天"}},
		{ID: "C3", Timeline: StoryChapterTimeline{Anchor: ""}},
	}
	ov.Outline = &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "V1", Chapters: chs}}}}}
	ov.validateAnchorFormat()
	if len(ov.Issues) != 1 {
		t.Errorf("want 1 anchor format issue, got %d: %+v", len(ov.Issues), ov.Issues)
	}
}

func TestValidateLocationContinuity(t *testing.T) {
	ov := NewOutlineValidator(nil)
	chs := []StoryChapter{
		{ID: "C1", StateAnchor: StoryStateAnchor{Location: "玄云宗·外门"}},
		{ID: "C2", StateAnchor: StoryStateAnchor{Location: "玄云宗·内门"}, Timeline: StoryChapterTimeline{Transition: "离开外门前往内门"}},
		{ID: "C3", StateAnchor: StoryStateAnchor{Location: "须弥秘境·入口"}}, // 跨区域 + transition 空
	}
	ov.Outline = &StoryOutline{Parts: []StoryPart{{ID: "P1", Volumes: []StoryVolume{{ID: "V1", Chapters: chs}}}}}
	ov.validateLocationContinuity()
	if len(ov.Issues) != 1 {
		t.Errorf("want 1 location issue, got %d: %+v", len(ov.Issues), ov.Issues)
	}
	if !strings.Contains(ov.Issues[0].Description, "transition 为空") {
		t.Errorf("issue should mention transition 为空, got: %s", ov.Issues[0].Description)
	}
}
