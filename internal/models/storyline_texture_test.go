package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStorylineTextureJSONRoundTrip(t *testing.T) {
	setup := StorySetup{
		ProjectName: "Fire Galaxy",
		Storylines: []Storyline{{
			Name:           "Signal War",
			Description:    "A dormant signal wakes old enemies.",
			Type:           "main",
			Importance:     9,
			Scope:          "series",
			PayoffStyle:    "staged_reveal",
			SetupRole:      "long mystery engine",
			Desire:         "Find who sent the signal.",
			Opposition:     "The fleet wants to bury the evidence.",
			Stakes:         "The colony falls if the signal spreads.",
			Turn:           "The signal is coming from an ally ship.",
			Payoff:         "The first mystery reframes the war.",
			OpenQuestion:   "Who benefits from waking the swarm?",
			PressurePoints: []string{"lost telemetry", "fleet quarantine"},
		}},
	}

	data, err := json.Marshal(setup)
	if err != nil {
		t.Fatalf("marshal setup: %v", err)
	}

	var got StorySetup
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal setup: %v", err)
	}

	storyline := got.Storylines[0]
	if storyline.Desire != setup.Storylines[0].Desire {
		t.Fatalf("desire = %q, want %q", storyline.Desire, setup.Storylines[0].Desire)
	}
	if storyline.Scope != "series" || storyline.PayoffStyle != "staged_reveal" || storyline.SetupRole != "long mystery engine" {
		t.Fatalf("storyline contract hints not preserved: %#v", storyline)
	}
	if len(storyline.PressurePoints) != 2 || storyline.PressurePoints[1] != "fleet quarantine" {
		t.Fatalf("pressure_points = %#v", storyline.PressurePoints)
	}
}

func TestStorylineTextureOmitEmptyAndMarkdown(t *testing.T) {
	emptyData, err := json.Marshal(Storyline{Name: "Quiet Arc"})
	if err != nil {
		t.Fatalf("marshal empty storyline: %v", err)
	}
	if strings.Contains(string(emptyData), "desire") || strings.Contains(string(emptyData), "pressure_points") {
		t.Fatalf("empty optional fields should be omitted: %s", emptyData)
	}

	outline := Outline{Parts: []Part{{
		Title:   "Part One",
		Summary: "A signal wakes.",
		Volumes: []Volume{{
			Title:   "Volume One",
			Summary: "The first chase.",
			Chapters: []Chapter{{
				Title:   "Wake",
				Summary: "A pilot finds the signal.",
				StorylineAdvances: []StorylineAdvance{{
					StorylineName: "Signal War",
					Stage:         "reveal",
					Change:        "The signal is proven artificial.",
					Pressure:      "The enemy can trace the receiver.",
				}},
			}},
		}},
	}}}

	md := outline.ToMarkdown()
	if !strings.Contains(md, "Storyline Advances") {
		t.Fatalf("markdown missing storyline advances section:\n%s", md)
	}
	if !strings.Contains(md, "Signal War") || !strings.Contains(md, "pressure: The enemy can trace the receiver.") {
		t.Fatalf("markdown missing advance details:\n%s", md)
	}
}
