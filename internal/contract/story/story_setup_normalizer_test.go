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
		LongFormPlan: &LongFormPlan{
			TargetChapters:   1000,
			TargetVolumes:    10,
			MainLoop:         "  pressure -> exploit -> win  ",
			EscalationLadder: []string{" local ", "", "local", " empire "},
			ReaderPromises:   []string{" public wins ", " public wins ", " faction rise "},
			PayoffCadence:    "  small wins every chapter, big wins every volume  ",
			VolumePattern:    []string{" hook ", " exploit ", " win "},
			MidpointMutation: "  local game becomes regional war  ",
			EndgamePromise:   "  final public reversal  ",
		},
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
			RepeatablePressure: "  rival factions keep raising public tests  ",
			PayoffCadence:      "  partial reveal each volume  ",
			Mutation:           "  local rivalry becomes imperial audit  ",
			FailureMode:        "  repetitive tournament brackets  ",
		}},
		CoreCast: []CoreCastSeed{{
			Name:            "  Hero  ",
			Role:            " protagonist ",
			Importance:      10,
			StoryFunction:   "  drives the main power fantasy  ",
			RelationshipArc: "  isolated -> trusted  ",
			EntryPhase:      " opening ",
			Payoff:          "  public victory  ",
			StorylineRefs:   []string{" Main Arc ", "", "Main Arc"},
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
	if setup.LongFormPlan == nil || setup.LongFormPlan.MainLoop != "pressure -> exploit -> win" || len(setup.LongFormPlan.EscalationLadder) != 2 {
		t.Fatalf("long form plan was not normalized: %#v", setup.LongFormPlan)
	}
	if len(setup.Storylines) != 1 || setup.Storylines[0].Name != "Main Arc" {
		t.Fatalf("storyline was not trimmed: %#v", setup.Storylines)
	}
	if setup.Storylines[0].RepeatablePressure != "rival factions keep raising public tests" || setup.Storylines[0].FailureMode != "repetitive tournament brackets" {
		t.Fatalf("storyline serial engine was not trimmed: %#v", setup.Storylines[0])
	}
	if len(setup.CoreCast) != 1 || setup.CoreCast[0].Name != "Hero" || len(setup.CoreCast[0].StorylineRefs) != 1 {
		t.Fatalf("core cast was not normalized: %#v", setup.CoreCast)
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
