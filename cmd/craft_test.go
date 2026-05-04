package cmd

import (
	"testing"

	"novelgen/internal/models"
)

func TestElementExtractorUsesRPGRelevantOutlineFields(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		Volumes: []models.Volume{{
			Chapters: []models.Chapter{{
				ID:         "P1-V1-C1",
				Characters: []string{"Chen Dong"},
				Location:   "Mine Gate",
				StateAnchor: models.StateAnchor{
					Allies:   []string{"San Ge"},
					Location: "Deep Shaft: wet and dark",
					KeyItems: []string{"Rusty Pickaxe"},
				},
				Enemies:        []models.OutlineEnemy{{Name: "Drone Guard"}},
				ResourceLedger: []models.ResourceLedgerEntry{{Item: "Spirit Stone", Start: 0, Delta: 3, End: 3}},
				Scenes: []models.OutlineScene{{
					POV:        "San Ge",
					Location:   "Repair Bay",
					Characters: []string{"Mechanic Luo"},
				}},
				Events: []models.Event{
					{Actor: "Chen Dong", Action: models.ActionAcquire, Target: "Blood Token", TargetType: models.TargetTypeItem},
					{Actor: "Drone Guard", Action: models.ActionMove, Target: "Mine Gate", TargetType: models.TargetTypeLocation},
					{Actor: "San Ge", Action: models.ActionMeet, Target: "Hidden Trader", TargetType: models.TargetTypeCharacter},
				},
			}},
		}},
	}}}

	elements := NewElementExtractor(outline, nil).Extract()

	for _, name := range []string{"Chen Dong", "San Ge", "Drone Guard", "Mechanic Luo", "Hidden Trader"} {
		if !containsString(elements.Characters, name) {
			t.Fatalf("missing character %q in %+v", name, elements.Characters)
		}
	}
	for _, name := range []string{"Mine Gate", "Deep Shaft", "Repair Bay"} {
		if !containsString(elements.Locations, name) {
			t.Fatalf("missing location %q in %+v", name, elements.Locations)
		}
	}
	for _, name := range []string{"Rusty Pickaxe", "Spirit Stone", "Blood Token"} {
		if !containsString(elements.Items, name) {
			t.Fatalf("missing item %q in %+v", name, elements.Items)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
