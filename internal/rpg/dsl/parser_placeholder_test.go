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
