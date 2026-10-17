package setupcmd

import (
	"novelgen/internal/contract/story"
	"novelgen/internal/toolplane"
)

func hasPatchChange(changes []toolplane.ToolPatchChange, path string) bool {
	for _, change := range changes {
		if change.Path == path {
			return true
		}
	}
	return false
}

// validLongFormSetup mirrors the cmd package fixture used by the setup SDK tests.
func validLongFormSetup() *models.StorySetup {
	return &models.StorySetup{
		ProjectName:    "Long Test",
		Genres:         []string{"web novel", "power fantasy"},
		Premise:        "A defeated heir rebuilds a broken frontier over 1000 chapters by exploiting a public ranking system.",
		Theme:          "Power earned through perception, timing, and alliances",
		Rules:          []string{"Ranks can be challenged publicly", "Faction resources change hands after formal wins"},
		TargetAudience: "adult genre readers",
		Tone:           "fast, escalating",
		Tense:          "past",
		POVStyle:       "third person limited",
		LongFormPlan: &models.LongFormPlan{
			TargetChapters:   1000,
			TargetVolumes:    10,
			MainLoop:         "pressure -> opponent misread -> clever exploit -> visible win -> reward -> bigger game",
			EscalationLadder: []string{"frontier town", "city league", "regional faction", "imperial arena"},
			ReaderPromises:   []string{"power growth", "public reversals", "faction rise"},
			PayoffCadence:    "small wins every chapter, medium wins every 10 chapters, major wins every volume",
			VolumePattern:    []string{"hook", "pressure", "misread", "exploit", "big win", "visible reward", "next gate"},
			MidpointMutation: "the ranking game mutates into a faction war",
			EndgamePromise:   "the hero overturns the public ranking system with everyone watching",
		},
		Storylines: []models.Storyline{
			{Name: "Frontier Rise", Importance: 7},
			{Name: "Faction War", Importance: 7},
		},
		Premises: []models.Premise{{
			Name: "Ranking System",
			Progression: []models.ProgressionStage{
				{Level: 1, Name: "Outer Rank", Description: "Entry-level recognition"},
				{Level: 2, Name: "Inner Rank", Description: "Faction-level authority"},
			},
		}},
		CoreCast: []models.CoreCastSeed{
			{
				Name:               "Hero",
				Role:               "protagonist",
				Importance:         10,
				StoryFunction:      "drives the ranking exploit loop",
				RelationshipToLead: "self",
				RelationshipArc:    "isolated survivor to public leader",
				EntryPhase:         "opening",
				Payoff:             "turns humiliation into visible authority",
				StorylineRefs:      []string{"Frontier Rise"},
			},
			{
				Name:               "First Lead",
				Role:               "female_lead",
				Importance:         9,
				StoryFunction:      "anchors trust and information access",
				RelationshipToLead: "uneasy ally",
				RelationshipArc:    "contract ally to trusted partner",
				EntryPhase:         "early",
				Payoff:             "chooses the hero over her faction",
				StorylineRefs:      []string{"Frontier Rise", "Faction War"},
			},
			{
				Name:               "Rival",
				Role:               "rival",
				Importance:         8,
				StoryFunction:      "tests each new public status jump",
				RelationshipToLead: "competitor",
				RelationshipArc:    "mockery to respect",
				EntryPhase:         "early",
				Payoff:             "admits the hero won by skill, not luck",
				StorylineRefs:      []string{"Frontier Rise"},
			},
			{
				Name:               "Hidden Mentor",
				Role:               "mentor",
				Importance:         8,
				StoryFunction:      "reveals old rules without solving fights",
				RelationshipToLead: "suspicious sponsor",
				RelationshipArc:    "transactional help to legacy handoff",
				EntryPhase:         "mid",
				Payoff:             "unlocks the larger faction history",
				StorylineRefs:      []string{"Faction War"},
			},
			{
				Name:               "Late Antagonist",
				Role:               "antagonist",
				Importance:         9,
				StoryFunction:      "turns local wins into a regional war",
				RelationshipToLead: "distant pressure",
				RelationshipArc:    "rumored threat to personal enemy",
				EntryPhase:         "series",
				Payoff:             "forces the hero to defend everything he built",
				StorylineRefs:      []string{"Faction War"},
			},
		},
	}
}
