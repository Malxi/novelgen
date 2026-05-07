package models

import (
	"encoding/json"
	"testing"
)

func TestCraftRPGMetadataRoundTripAndNormalize(t *testing.T) {
	character := Character{
		RPGRole:    "player",
		CombatRole: "striker",
		PowerLevel: -1,
		RPGStats:   &CraftRPGStats{STR: 12, AGI: -3, INT: 14, VIT: 11, HP: 120, MP: 60, Level: 2},
		DSLTags:    []string{"pilot", "pilot", ""},
		StateEffects: []CraftStateEffect{
			{Target: "protagonist", Kind: "status", Field: "identity", To: "awakened"},
			{},
		},
	}
	character.NormalizeForCraft("Chen Dong")

	if character.Name != "Chen Dong" {
		t.Fatalf("expected fallback name, got %q", character.Name)
	}
	if character.PowerLevel != 0 || character.RPGStats.AGI != 0 {
		t.Fatalf("negative values were not clamped: %+v", character)
	}
	if len(character.DSLTags) != 1 || len(character.StateEffects) != 1 {
		t.Fatalf("metadata was not compacted: %+v", character)
	}

	data, err := json.Marshal(character)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Character
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.RPGRole != "player" || decoded.RPGStats == nil || decoded.RPGStats.STR != 12 {
		t.Fatalf("metadata did not round trip: %+v", decoded)
	}
}

func TestOrganizationNormalizeForCraft(t *testing.T) {
	org := Organization{
		Goals:     []string{"control the gate", "control the gate", ""},
		Resources: []string{"spies", ""},
		DSLTags:   []string{"faction", "faction"},
		StateEffects: []CraftStateEffect{
			{Target: "protagonist", Kind: "relationship", Field: "standing", To: "hostile"},
			{},
		},
	}

	org.NormalizeForCraft("Iron Sect")

	if org.Name != "Iron Sect" {
		t.Fatalf("expected fallback name, got %q", org.Name)
	}
	if len(org.Goals) != 1 || len(org.Resources) != 1 || len(org.DSLTags) != 1 || len(org.StateEffects) != 1 {
		t.Fatalf("organization metadata was not compacted: %+v", org)
	}
}
