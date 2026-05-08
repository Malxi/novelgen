package models

import "testing"

func TestNormalizeStorySetupTrimsAndOrdersContracts(t *testing.T) {
	setup := &StorySetup{
		ProjectName:    "  Test Book  ",
		Genres:         []string{"  sci-fi  ", "", "sci-fi", "mecha"},
		Premise:        "  A pilot turns the enemy's rules into a weapon. ",
		Theme:          "  freedom  ",
		Rules:          []string{" rule one ", "rule one", "rule two"},
		TargetAudience: "  genre readers  ",
		Tone:           "  sharp  ",
		Tense:          " present ",
		POVStyle:       "  third person  ",
		Storylines: []Storyline{{
			Name:       "  Main Arc  ",
			Importance: 9,
			AppealEngine: &AppealEngine{
				Appeal:          "  win big  ",
				SurfaceLimit:    " ",
				Exploit:         "  use enemy misreads  ",
				SignatureWin:    "  ",
				UpgradePath:     "  ",
				OpponentMisread: "  ",
				RewardType:      "  reputation  ",
			},
		}},
		Premises: []Premise{{
			Name:        "  Growth System  ",
			Description: "  Growth that scales. ",
			Category:    "  mech  ",
			Progression: []ProgressionStage{
				{Level: 3, Name: "  Third  ", Description: " later "},
				{Level: 1, Name: "  First  ", Description: " early "},
			},
			AppealEngine: &AppealEngine{},
		}},
		WorldTimeline: []WorldTimelineEntry{{
			Year:  "  100  ",
			Event: "  War begins  ",
		}},
		WorldResources: []WorldResource{{
			Name:     "  crystal  ",
			Category: "  energy  ",
			Scarcity: "  rare  ",
		}},
	}

	report := NormalizeStorySetup(setup)

	if !report.Changed() {
		t.Fatal("expected normalization changes")
	}
	if setup.ProjectName != "Test Book" || setup.Premise != "A pilot turns the enemy's rules into a weapon." {
		t.Fatalf("setup strings were not trimmed: %#v", setup)
	}
	if len(setup.Genres) != 2 || setup.Genres[0] != "sci-fi" || setup.Genres[1] != "mecha" {
		t.Fatalf("genres were not compacted: %#v", setup.Genres)
	}
	if len(setup.Rules) != 2 {
		t.Fatalf("rules were not compacted: %#v", setup.Rules)
	}
	if len(setup.Storylines) != 1 || setup.Storylines[0].Name != "Main Arc" {
		t.Fatalf("storyline was not trimmed: %#v", setup.Storylines)
	}
	if setup.Storylines[0].AppealEngine == nil || setup.Storylines[0].AppealEngine.SurfaceLimit != "" {
		t.Fatalf("storyline appeal engine was not normalized: %#v", setup.Storylines[0].AppealEngine)
	}
	if len(setup.Premises) != 1 || setup.Premises[0].Progression[0].Level != 1 || setup.Premises[0].Progression[1].Level != 3 {
		t.Fatalf("premise progression was not ordered: %#v", setup.Premises[0].Progression)
	}
	if setup.Premises[0].AppealEngine != nil {
		t.Fatalf("empty premise appeal engine should be removed: %#v", setup.Premises[0].AppealEngine)
	}
	if len(setup.WorldTimeline) != 1 || setup.WorldTimeline[0].Year != "100" || setup.WorldTimeline[0].Event != "War begins" {
		t.Fatalf("timeline was not trimmed: %#v", setup.WorldTimeline)
	}
	if len(setup.WorldResources) != 1 || setup.WorldResources[0].Name != "crystal" {
		t.Fatalf("resources were not trimmed: %#v", setup.WorldResources)
	}
}
