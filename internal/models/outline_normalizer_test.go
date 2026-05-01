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

	if len(report.Changes) != 2 {
		t.Fatalf("changes = %d, want 2: %#v", len(report.Changes), report.Changes)
	}
	if report.Changes[0].Action != "sync_event_character" || report.Changes[0].Value != "Squad" {
		t.Fatalf("first change = %#v", report.Changes[0])
	}
	if report.Changes[1].Action != "sync_scene_character" || report.Changes[1].Value != "Survivors" {
		t.Fatalf("second change = %#v", report.Changes[1])
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
