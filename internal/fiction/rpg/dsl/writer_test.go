package dsl

import (
	"strings"
	"testing"
)

func TestDSLStringPreservesEventResults(t *testing.T) {
	input := &DSL{
		Storyline: &Storyline{Chapters: []Chapter{{
			ID:    "P1-V1-C1",
			Title: "Opening",
			Objectives: []Objective{{
				Name: "Survive",
				Steps: []Step{{
					Order:       1,
					Description: "Hero wins the fight.",
					Event: Event{
						Type: "combat",
						Combat: &CombatEvent{Setup: CombatSetup{Enemies: []EnemySpawn{{
							ID:    "enemy_worker_bee",
							Count: 1,
							Level: 1,
						}}}},
						OnComplete: &EventResult{
							Narration: "Hero defeats the worker bee.",
							Exp:       5,
						},
					},
				}},
			}},
		}}},
	}

	text := input.String()
	for _, want := range []string{
		"on_complete {",
		`narration = "Hero defeats the worker bee."`,
		"exp = 5",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("serialized DSL missing %q:\n%s", want, text)
		}
	}

	parsed, err := NewParser(text).Parse()
	if err != nil {
		t.Fatalf("round-trip parse failed: %v\n%s", err, text)
	}
	got := parsed.Storyline.Chapters[0].Objectives[0].Steps[0].Event.OnComplete
	if got == nil {
		t.Fatalf("OnComplete was not preserved")
	}
	if got.Narration != "Hero defeats the worker bee." || got.Exp != 5 {
		t.Fatalf("OnComplete = %#v", got)
	}
}
