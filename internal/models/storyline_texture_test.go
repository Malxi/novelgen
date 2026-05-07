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
			AppealEngine: &AppealEngine{
				Appeal:          "A weak pilot wins by reading signal timing better than the fleet.",
				SurfaceLimit:    "The signal only updates every seven breaths.",
				Exploit:         "He moves during the blind interval.",
				SignatureWin:    "The enemy fires at his old position while he takes the command room.",
				UpgradePath:     "Later he predicts longer signal chains.",
				OpponentMisread: "Enemies assume the signal is real-time surveillance.",
				RewardType:      "reputation",
			},
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
	if storyline.AppealEngine == nil || storyline.AppealEngine.Exploit != "He moves during the blind interval." {
		t.Fatalf("appeal_engine not preserved: %#v", storyline.AppealEngine)
	}
}

func TestStorylineTextureOmitEmptyAndMarkdown(t *testing.T) {
	emptyData, err := json.Marshal(Storyline{Name: "Quiet Arc"})
	if err != nil {
		t.Fatalf("marshal empty storyline: %v", err)
	}
	if strings.Contains(string(emptyData), "desire") || strings.Contains(string(emptyData), "pressure_points") || strings.Contains(string(emptyData), "appeal_engine") {
		t.Fatalf("empty optional fields should be omitted: %s", emptyData)
	}

	outline := Outline{Parts: []Part{{
		Title:   "Part One",
		Summary: "A signal wakes.",
		Volumes: []Volume{{
			Title:   "Volume One",
			Summary: "The first chase.",
			PayoffContract: &VolumePayoffContract{
				VolumeQuestion:      "Can the pilot expose the false command chain?",
				PowerPromise:        "He turns a surveillance weakness into a weapon.",
				MainOpponentMisread: "The admiral thinks he only knows the signal exists.",
				BigWin:              "He hijacks the enemy broadcast in public.",
				VisibleReward:       "A ship and a crew.",
				ReputationShift:     "From suspect to impossible problem.",
				NextBiggerGame:      "The signal source answers back.",
			},
			Chapters: []Chapter{{
				Title:   "Wake",
				Summary: "A pilot finds the signal.",
				ChapterPayoff: &ChapterPayoff{
					Desire:       "Escape the locked bay.",
					Pressure:     "Security drones close every exit.",
					CleverMove:   "He times movement to the signal refresh gap.",
					PayoffMoment: "The drones salute an empty corner while he opens the bay.",
					Reward:       "He gets the access card.",
					SocialProof:  "The guard captain realizes the cameras lied.",
					Hook:         "The card opens a door that should not exist.",
				},
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
	if !strings.Contains(md, "Payoff Contract") || !strings.Contains(md, "The signal source answers back.") {
		t.Fatalf("markdown missing payoff contract:\n%s", md)
	}
	if !strings.Contains(md, "Chapter Payoff") || !strings.Contains(md, "The drones salute an empty corner") {
		t.Fatalf("markdown missing chapter payoff:\n%s", md)
	}
}
