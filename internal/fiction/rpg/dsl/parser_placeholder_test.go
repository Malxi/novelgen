package dsl

import (
	"strings"
	"testing"
)

func TestParserPreservesPlaceholderMarkers(t *testing.T) {
	source := &DSL{
		Characters: &Characters{
			Player: &Player{
				ID:                "hero",
				Name:              "Hero",
				IsPlaceholder:     true,
				PlaceholderSource: "outline",
			},
			Enemies: []Enemy{
				{
					ID:                "raider",
					Name:              "Raider",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
			},
			NPCs: []NPC{
				{
					ID:                "mentor",
					Name:              "Mentor",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
			},
		},
		World: &World{
			Locations: []Location{
				{
					ID:                "cryo_facility",
					Name:              "Cryo Facility",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
			},
		},
	}

	var b strings.Builder
	if err := NewDSLWriter(&b).WriteDSL(source); err != nil {
		t.Fatalf("write DSL: %v", err)
	}

	parsed, err := NewParser(b.String()).Parse()
	if err != nil {
		t.Fatalf("parse DSL: %v", err)
	}
	assertPlaceholderMarkers(t, parsed)

	enhanced, err := NewEnhancedParser(b.String()).ParseEnhanced()
	if err != nil {
		t.Fatalf("parse enhanced DSL: %v", err)
	}
	assertPlaceholderMarkers(t, enhanced)
}

func TestParserPreservesPlayerProfileFields(t *testing.T) {
	source := &DSL{
		Characters: &Characters{
			Player: &Player{
				ID:           "lin_ye",
				Name:         "林野",
				Class:        "mecha_pilot",
				Description:  "火种机甲项目核心研发工程师。",
				Age:          29,
				Gender:       "男",
				Race:         "人类",
				Background:   "遭沈氏暗害后休眠百年苏醒。",
				Personality:  []string{"沉稳务实", "重情重义"},
				Motivation:   "带领人类夺回母星。",
				Skills:       []string{"火种机甲同步"},
				Abilities:    []string{"基因适配控制"},
				Affiliations: []string{"青藤据点"},
				RoleInStory:  "protagonist",
				Voice:        "冷静克制",
			},
		},
	}

	var b strings.Builder
	if err := NewDSLWriter(&b).WriteDSL(source); err != nil {
		t.Fatalf("write DSL: %v", err)
	}

	parsed, err := NewParser(b.String()).Parse()
	if err != nil {
		t.Fatalf("parse DSL: %v", err)
	}
	assertPlayerProfile(t, parsed.Characters.Player)

	enhanced, err := NewEnhancedParser(b.String()).ParseEnhanced()
	if err != nil {
		t.Fatalf("parse enhanced DSL: %v", err)
	}
	assertPlayerProfile(t, enhanced.Characters.Player)
}

func assertPlaceholderMarkers(t *testing.T, parsed *DSL) {
	t.Helper()

	if parsed.Characters == nil || parsed.Characters.Player == nil || !parsed.Characters.Player.IsPlaceholder {
		t.Fatalf("player placeholder marker was not preserved: %+v", parsed.Characters)
	}
	if parsed.Characters.Player.PlaceholderSource != "outline" {
		t.Fatalf("player placeholder source was not preserved: %q", parsed.Characters.Player.PlaceholderSource)
	}
	if len(parsed.Characters.Enemies) != 1 || !parsed.Characters.Enemies[0].IsPlaceholder {
		t.Fatalf("enemy placeholder marker was not preserved: %+v", parsed.Characters.Enemies)
	}
	if len(parsed.Characters.NPCs) != 1 || !parsed.Characters.NPCs[0].IsPlaceholder {
		t.Fatalf("npc placeholder marker was not preserved: %+v", parsed.Characters.NPCs)
	}
	if parsed.World == nil || len(parsed.World.Locations) != 1 || !parsed.World.Locations[0].IsPlaceholder {
		t.Fatalf("location placeholder marker was not preserved: %+v", parsed.World)
	}
	if parsed.World.Locations[0].PlaceholderSource != "outline" {
		t.Fatalf("location placeholder source was not preserved: %q", parsed.World.Locations[0].PlaceholderSource)
	}
}

func assertPlayerProfile(t *testing.T, player *Player) {
	t.Helper()

	if player == nil {
		t.Fatal("player was nil")
	}
	if player.Name != "林野" || player.Class != "mecha_pilot" || player.Background == "" || player.Motivation == "" {
		t.Fatalf("player profile fields were not preserved: %+v", player)
	}
	if len(player.Personality) != 2 || len(player.Abilities) != 1 || len(player.Affiliations) != 1 {
		t.Fatalf("player slice fields were not preserved: %+v", player)
	}
	if player.Age != 29 || player.Gender != "男" || player.Race != "人类" || player.RoleInStory != "protagonist" || player.Voice != "冷静克制" {
		t.Fatalf("player scalar fields were not preserved: %+v", player)
	}
}
