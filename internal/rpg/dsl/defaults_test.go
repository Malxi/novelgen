package dsl

import "testing"

func TestApplySimulationDefaultsFillsMetadataAndPlayer(t *testing.T) {
	doc := &DSL{
		Characters: &Characters{
			NPCs: []NPC{{ID: "npc1", Name: "Lin"}},
		},
	}

	ApplySimulationDefaults("fire-galaxy", doc)

	if doc.Metadata == nil || doc.Metadata.Title != "fire-galaxy" || doc.Metadata.DSLVersion != "0.1.0" {
		t.Fatalf("metadata defaults not applied: %#v", doc.Metadata)
	}
	if doc.Characters.Player == nil {
		t.Fatal("player should be created")
	}
	if doc.Characters.Player.Name != "Lin" || doc.Characters.Player.ID != "char_player" {
		t.Fatalf("player = %#v, want Lin/char_player", doc.Characters.Player)
	}
}

func TestApplySimulationDefaultsKeepsExistingValues(t *testing.T) {
	doc := &DSL{
		Metadata: &Metadata{Title: "Existing", DSLVersion: "9.9"},
		Characters: &Characters{
			Player: &Player{ID: "hero", Name: "Hero"},
			NPCs:   []NPC{{Name: "Lin"}},
		},
	}

	ApplySimulationDefaults("fire-galaxy", doc)

	if doc.Metadata.Title != "Existing" || doc.Metadata.DSLVersion != "9.9" {
		t.Fatalf("existing metadata was overwritten: %#v", doc.Metadata)
	}
	if doc.Characters.Player.ID != "hero" || doc.Characters.Player.Name != "Hero" {
		t.Fatalf("existing player was overwritten: %#v", doc.Characters.Player)
	}
}

func TestApplySimulationDefaultsHandlesNil(t *testing.T) {
	ApplySimulationDefaults("book", nil)
}
