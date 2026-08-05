package models

import (
	"strings"
	"testing"
)

func TestNormalizeOutlineSyncsEventAndSceneCharacters(t *testing.T) {
	outline := &Outline{
		Parts: []Part{{
			ID: "P1",
			Volumes: []Volume{{
				ID: "P1-V1",
				Chapters: []Chapter{{
					ID:         "P1-V1-C1",
					Characters: []string{"Lin", "Lin", " "},
					Events: []Event{{
						Characters: []string{"Squad", "Lin"},
					}},
					Scenes: []OutlineScene{{
						Characters: []string{"Survivors", "Squad"},
					}},
				}},
			}},
		}},
	}

	report := NormalizeOutline(outline)
	chapter := outline.Parts[0].Volumes[0].Chapters[0]

	want := []string{"Lin", "Squad", "Survivors"}
	if len(chapter.Characters) != len(want) {
		t.Fatalf("characters length = %d, want %d: %#v", len(chapter.Characters), len(want), chapter.Characters)
	}
	for i := range want {
		if chapter.Characters[i] != want[i] {
			t.Fatalf("characters[%d] = %q, want %q", i, chapter.Characters[i], want[i])
		}
	}

	if !hasNormalizationChange(report, "sync_event_character", "Squad") {
		t.Fatalf("missing event character sync change: %#v", report.Changes)
	}
	if !hasNormalizationChange(report, "sync_scene_character", "Survivors") {
		t.Fatalf("missing scene character sync change: %#v", report.Changes)
	}
}

func TestNormalizeOutlineRestoresVolumeTitleOrdinals(t *testing.T) {
	outline := &Outline{Parts: []Part{{
		ID: "P1",
		Volumes: []Volume{
			{ID: "P1-V1", Title: "杂役白烬，所有人的新手任务"},
			{ID: "P1-V2", Title: "第二卷：回档者的十三次失败"},
			{ID: "P1-V3", Title: "第3卷：青岚宗小比"},
			{ID: "P1-V4", Title: "三十六宗大比"},
		},
	}}}

	report := NormalizeOutline(outline)

	want := []string{
		"第一卷：杂役白烬，所有人的新手任务",
		"第二卷：回档者的十三次失败",
		"第3卷：青岚宗小比",
		"第四卷：三十六宗大比",
	}
	for i, volume := range outline.Parts[0].Volumes {
		if volume.Title != want[i] {
			t.Fatalf("volume %d title = %q, want %q", i+1, volume.Title, want[i])
		}
	}
	if !hasNormalizationChange(report, "restore_volume_title_ordinal", "第一卷：杂役白烬，所有人的新手任务") {
		t.Fatalf("missing title ordinal normalization change: %#v", report.Changes)
	}
}

func TestNormalizeOutlineCanonicalizesStateAnchorAlliesAndInjuries(t *testing.T) {
	outline := &Outline{
		Parts: []Part{{
			ID: "P1",
			Volumes: []Volume{{
				ID: "P1-V1",
				Chapters: []Chapter{{
					ID: "P1-V1-C1",
					StateAnchor: StateAnchor{
						Allies: []string{
							"苏晴（据点内）",
							"苏晴",
							"老周（后方）",
						},
						Injuries: []string{
							"左手背轻微划伤（已愈合）",
							"右手轻微扭伤（已包扎）",
						},
						Notes: "原有备注",
					},
				}}}},
		}},
	}

	report := NormalizeOutline(outline)
	anchor := outline.Parts[0].Volumes[0].Chapters[0].StateAnchor

	wantAllies := []string{"苏晴", "老周"}
	if len(anchor.Allies) != len(wantAllies) {
		t.Fatalf("allies length = %d, want %d: %#v", len(anchor.Allies), len(wantAllies), anchor.Allies)
	}
	for i := range wantAllies {
		if anchor.Allies[i] != wantAllies[i] {
			t.Fatalf("allies[%d] = %q, want %q", i, anchor.Allies[i], wantAllies[i])
		}
	}

	wantInjuries := []string{"右手轻微扭伤"}
	if len(anchor.Injuries) != len(wantInjuries) {
		t.Fatalf("injuries length = %d, want %d: %#v", len(anchor.Injuries), len(wantInjuries), anchor.Injuries)
	}
	if anchor.Injuries[0] != wantInjuries[0] {
		t.Fatalf("injuries[0] = %q, want %q", anchor.Injuries[0], wantInjuries[0])
	}

	for _, want := range []string{"盟友备注：苏晴（据点内）", "盟友备注：老周（后方）", "伤势已恢复：左手背轻微划伤（已愈合）", "伤势备注：右手轻微扭伤（已包扎）"} {
		if !strings.Contains(anchor.Notes, want) {
			t.Fatalf("notes missing %q: %s", want, anchor.Notes)
		}
	}

	if len(report.Changes) < 5 {
		t.Fatalf("changes = %d, want at least 5: %#v", len(report.Changes), report.Changes)
	}
}

func TestNormalizeOutlineMigratesLegacyChapterShape(t *testing.T) {
	outline := &Outline{
		Parts: []Part{{
			ID: "P1",
			Volumes: []Volume{{
				ID: "P1-V1",
				Chapters: []Chapter{{
					ID:          "P1-V1-C1",
					Title:       "旧章",
					Summary:     "林砚在矿道发现禁术线索",
					Characters:  []string{"林砚"},
					Location:    "黑风矿旧矿道",
					LegacyBeats: []string{"林砚进入旧矿道", "他发现禁术残痕", "他决定回收线索"},
					OpeningBeat: "林砚进入旧矿道",
					ClosingBeat: "他决定回收线索",
					StateChange: "林砚获得禁术线索",
					Conflict:    "旧矿道随时可能塌方",
					Pacing:      "normal",
					Events: []Event{{
						Type:       "premise",
						Characters: []string{"林砚"},
						Subject:    "禁术线索",
						Change:     "发现旧矿道残痕",
					}},
				}},
			}},
		}},
	}

	report := NormalizeOutline(outline)
	chapter := outline.Parts[0].Volumes[0].Chapters[0]

	if !report.Changed() {
		t.Fatal("expected normalization changes")
	}
	if len(chapter.Scenes) != 2 {
		t.Fatalf("scenes = %d, want 2: %#v", len(chapter.Scenes), chapter.Scenes)
	}
	for i, scene := range chapter.Scenes {
		if scene.Order != i+1 {
			t.Fatalf("scene %d order = %d", i, scene.Order)
		}
		if scene.POV != "林砚" || scene.Goal == "" || scene.Location != "黑风矿旧矿道" || len(scene.Beats) == 0 {
			t.Fatalf("scene %d not fully populated: %#v", i, scene)
		}
	}
	if chapter.Timeline.Anchor == "" {
		t.Fatal("timeline anchor was not filled")
	}
	if chapter.StateAnchor.Location != "黑风矿旧矿道" {
		t.Fatalf("state anchor location = %q", chapter.StateAnchor.Location)
	}
	if len(chapter.Events) < 3 {
		t.Fatalf("events = %d, want at least 3: %#v", len(chapter.Events), chapter.Events)
	}
	for i, event := range chapter.Events {
		if event.GetActor() == "" || event.GetAction() == "" || event.GetTarget() == "" {
			t.Fatalf("event %d not normalized for quality gate: %#v", i, event)
		}
	}
	if len(chapter.LegacyBeats) != 3 {
		t.Fatalf("legacy beats were not preserved: %#v", chapter.LegacyBeats)
	}
}

func TestNormalizeOutlineSyncsOpeningAndClosingBeatIntoExistingScenes(t *testing.T) {
	outline := &Outline{
		Parts: []Part{{
			ID: "P1",
			Volumes: []Volume{{
				ID: "P1-V1",
				Chapters: []Chapter{{
					ID:          "P1-V1-C2",
					Title:       "承接章",
					Summary:     "林砚从旧矿道前往执法堂继续追查。",
					Characters:  []string{"林砚"},
					Location:    "执法堂",
					OpeningBeat: "紧接着上一章结尾，林砚前往执法堂核验禁术线索。",
					ClosingBeat: "林砚在执法堂确认线索已经指向内门。",
					Scenes: []OutlineScene{{
						Order:      1,
						POV:        "林砚",
						Goal:       "旧目标",
						Location:   "执法堂",
						Characters: []string{"林砚"},
						Beats:      []string{"旧开场", "中段推进", "旧收束"},
					}},
				}},
			}},
		}},
	}

	report := NormalizeOutline(outline)
	chapter := outline.Parts[0].Volumes[0].Chapters[0]

	if !report.Changed() {
		t.Fatal("expected normalization changes")
	}
	if got := chapter.Scenes[0].Beats[0]; got != chapter.OpeningBeat {
		t.Fatalf("opening scene beat = %q, want %q", got, chapter.OpeningBeat)
	}
	if got := chapter.Scenes[0].Beats[len(chapter.Scenes[0].Beats)-1]; got != chapter.ClosingBeat {
		t.Fatalf("closing scene beat = %q, want %q", got, chapter.ClosingBeat)
	}
}

func TestNormalizeOutlineFillsLegacyEventTriples(t *testing.T) {
	outline := &Outline{
		Parts: []Part{{
			ID: "P1",
			Volumes: []Volume{{
				ID: "P1-V1",
				Chapters: []Chapter{{
					ID:         "P1-V1-C1",
					Title:      "旧事件",
					Summary:    "林砚确认矿道异常",
					Location:   "黑风矿矿道",
					Characters: []string{"林砚"},
					Events: []Event{{
						Type:    "premise",
						Subject: "复活能力",
						Change:  "首次触发复活能力",
					}},
				}},
			}},
		}},
	}

	report := NormalizeOutline(outline)
	event := outline.Parts[0].Volumes[0].Chapters[0].Events[0]

	if event.Actor != "林砚" {
		t.Fatalf("actor = %q, want 林砚", event.Actor)
	}
	if event.Action == "" {
		t.Fatal("action was not filled")
	}
	if event.Target != "复活能力" {
		t.Fatalf("target = %q, want 复活能力", event.Target)
	}
	if event.Context != "黑风矿矿道" {
		t.Fatalf("context = %q, want 黑风矿矿道", event.Context)
	}
	if event.Result != "首次触发复活能力" {
		t.Fatalf("result = %q", event.Result)
	}
	if !hasNormalizationChange(report, "fill_event_actor", "林砚") {
		t.Fatalf("missing event actor normalization change: %#v", report.Changes)
	}
}

func TestNormalizeOutlineCanonicalizesLegacyIDs(t *testing.T) {
	outline := &Outline{
		Parts: []Part{{
			ID: "P1",
			Volumes: []Volume{
				{ID: "P1-V1"},
				{
					ID: "part1-volume2",
					Chapters: []Chapter{
						{ID: "p1-v2-c1"},
						{ID: "P1-V2-C2"},
					},
				},
			},
		}},
	}

	report := NormalizeOutline(outline)
	volume := outline.Parts[0].Volumes[1]

	if volume.ID != "P1-V2" {
		t.Fatalf("volume id = %q, want P1-V2", volume.ID)
	}
	if volume.Chapters[0].ID != "P1-V2-C1" {
		t.Fatalf("chapter id = %q, want P1-V2-C1", volume.Chapters[0].ID)
	}
	if !hasNormalizationChange(report, "canonicalize_volume_id", "part1-volume2 -> P1-V2") {
		t.Fatalf("missing volume id normalization change: %#v", report.Changes)
	}
	if !hasNormalizationChange(report, "canonicalize_chapter_id", "p1-v2-c1 -> P1-V2-C1") {
		t.Fatalf("missing chapter id normalization change: %#v", report.Changes)
	}
}

func TestNormalizeOutlinePayoffContracts(t *testing.T) {
	outline := &Outline{
		Parts: []Part{{
			ID: "P1",
			Volumes: []Volume{{
				ID: "P1-V1",
				PayoffContract: &VolumePayoffContract{
					VolumeQuestion: "  Can the hero turn the arena rules?  ",
					BigWin:         "  ",
				},
				Chapters: []Chapter{
					{
						ID:            "P1-V1-C1",
						ChapterPayoff: &ChapterPayoff{},
					},
					{
						ID: "P1-V1-C2",
						ChapterPayoff: &ChapterPayoff{
							PayoffMoment: "  The judge realizes the trap was legal.  ",
						},
					},
				},
			}, {
				ID:             "P1-V2",
				PayoffContract: &VolumePayoffContract{},
			}},
		}},
	}

	report := NormalizeOutline(outline)
	volume := outline.Parts[0].Volumes[0]

	if volume.PayoffContract == nil || volume.PayoffContract.VolumeQuestion != "Can the hero turn the arena rules?" {
		t.Fatalf("volume payoff was not trimmed: %#v", volume.PayoffContract)
	}
	if volume.Chapters[0].ChapterPayoff != nil {
		t.Fatalf("empty chapter payoff should be removed: %#v", volume.Chapters[0].ChapterPayoff)
	}
	if volume.Chapters[1].ChapterPayoff == nil || volume.Chapters[1].ChapterPayoff.PayoffMoment != "The judge realizes the trap was legal." {
		t.Fatalf("chapter payoff was not trimmed: %#v", volume.Chapters[1].ChapterPayoff)
	}
	if outline.Parts[0].Volumes[1].PayoffContract != nil {
		t.Fatalf("empty volume payoff should be removed: %#v", outline.Parts[0].Volumes[1].PayoffContract)
	}
	if !hasNormalizationChange(report, "drop_empty_chapter_payoff", "") {
		t.Fatalf("missing empty chapter payoff cleanup: %#v", report.Changes)
	}
	if !hasNormalizationChange(report, "drop_empty_volume_payoff", "") {
		t.Fatalf("missing empty volume payoff cleanup: %#v", report.Changes)
	}
}

func hasNormalizationChange(report OutlineNormalizationReport, action, value string) bool {
	for _, change := range report.Changes {
		if change.Action == action && change.Value == value {
			return true
		}
	}
	return false
}
