package dsl

import "testing"

func TestSimulatorSkipsPlaceholderLocationDescriptionChecks(t *testing.T) {
	sim := NewSimulator(&DSL{
		Metadata: &Metadata{PowerSystem: "sci-fi"},
		World: &World{
			Locations: []Location{
				{ID: "placeholder_loc", Name: "Placeholder", IsPlaceholder: true},
			},
		},
		Characters: &Characters{
			Player: &Player{
				ID:         "p",
				Name:       "Protagonist",
				Class:      "pilot",
				Background: "background",
				Motivation: "motivation",
			},
			NPCs: []NPC{
				{ID: "mentor", Name: "Mentor", Role: "mentor"},
				{ID: "villain", Name: "Villain", Role: "villain"},
			},
		},
		Storyline: &Storyline{},
		Systems:   &Systems{},
	})

	sim.SimulateAll()

	for _, issue := range sim.Issues {
		if issue.Type == IssueDescription && issue.Description != "" {
			t.Fatalf("placeholder location should not emit description issue: %+v", issue)
		}
	}
}
