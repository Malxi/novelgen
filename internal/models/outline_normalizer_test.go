package models

import "testing"

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
