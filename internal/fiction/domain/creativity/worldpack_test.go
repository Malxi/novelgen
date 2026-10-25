package creativity

import (
	"reflect"
	"strings"
	"testing"
)

func TestGenerateWorldPacksIsDeterministicAndCompatible(t *testing.T) {
	req := EngineRequest{Genre: "科幻", Seed: 42, Count: 3}
	first, err := GenerateWorldPacks(req)
	if err != nil {
		t.Fatalf("GenerateWorldPacks: %v", err)
	}
	second, err := GenerateWorldPacks(req)
	if err != nil {
		t.Fatalf("GenerateWorldPacks: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed produced different world packs")
	}
	if len(first.Packs) != 3 {
		t.Fatalf("packs = %d, want 3", len(first.Packs))
	}
	for _, pack := range first.Packs {
		if issues := ValidateWorldPack(pack); len(issues) > 0 {
			t.Fatalf("invalid world pack %s: %v", pack.ID, issues)
		}
		if !pack.Compatibility.Compatible {
			t.Fatalf("pack %s marked incompatible: %v", pack.ID, pack.Compatibility.Warnings)
		}
		if len(pack.Storylines) != 3 || len(pack.Cast) != 4 || len(pack.Resources) != 2 || len(pack.Items) != 7 {
			t.Fatalf("incomplete decks for %s: %#v", pack.ID, pack)
		}
		categories := map[string]bool{}
		for _, item := range pack.Items {
			categories[item.Category] = true
			if item.ReferenceID != pack.Reference.ID || item.PackID != pack.ID {
				t.Fatalf("foreign item: %#v", item)
			}
		}
		for _, category := range itemCategories {
			if !categories[category] {
				t.Fatalf("pack %s missing item category %s", pack.ID, category)
			}
		}
	}
}

func TestWorldPackCardsShareOneEngine(t *testing.T) {
	result, err := GenerateWorldPacks(EngineRequest{Genre: "都市", Seed: 77, Count: 2})
	if err != nil {
		t.Fatalf("GenerateWorldPacks: %v", err)
	}
	for _, pack := range result.Packs {
		if pack.Engine.ID != pack.ID || pack.Premise.PackID != pack.ID {
			t.Fatalf("engine/premise pack mismatch: %#v", pack)
		}
		for _, storyline := range pack.Storylines {
			if storyline.PackID != pack.ID {
				t.Fatalf("foreign storyline pack: %#v", storyline)
			}
		}
		for _, cast := range pack.Cast {
			if cast.PackID != pack.ID {
				t.Fatalf("foreign cast pack: %#v", cast)
			}
		}
		for _, resource := range pack.Resources {
			if resource.PackID != pack.ID {
				t.Fatalf("foreign resource pack: %#v", resource)
			}
		}
	}
}

func TestValidateWorldPackRejectsMixedPackIDs(t *testing.T) {
	result, err := GenerateWorldPacks(EngineRequest{Genre: "科幻", Seed: 9, Count: 1})
	if err != nil {
		t.Fatalf("GenerateWorldPacks: %v", err)
	}
	pack := result.Packs[0]
	pack.Cast[0].PackID = "foreign-pack"
	issues := ValidateWorldPack(pack)
	if len(issues) == 0 {
		t.Fatal("expected mixed pack_id validation error")
	}
}

func TestWorldPackCastNamesAreUnique(t *testing.T) {
	result, err := GenerateWorldPacks(EngineRequest{Genre: "悬疑", Seed: 18, Count: 4})
	if err != nil {
		t.Fatalf("GenerateWorldPacks: %v", err)
	}
	for _, pack := range result.Packs {
		seen := make(map[string]bool)
		for _, cast := range pack.Cast {
			if seen[cast.Name] {
				t.Fatalf("duplicate cast name %q in %s", cast.Name, pack.ID)
			}
			seen[cast.Name] = true
		}
	}
}

func TestWorldPackStorylineArchetypesAreDistinctAndLinked(t *testing.T) {
	result, err := GenerateWorldPacks(EngineRequest{Genre: "科幻", Seed: 42, Count: 4, Source: "星战"})
	if err != nil {
		t.Fatalf("GenerateWorldPacks: %v", err)
	}
	seenAcrossPacks := map[string]bool{}
	for _, pack := range result.Packs {
		if len(pack.Storylines) != 3 {
			t.Fatalf("storylines = %d, want 3", len(pack.Storylines))
		}
		castIDs := map[string]bool{}
		for _, cast := range pack.Cast {
			castIDs[cast.ID] = true
		}
		itemIDs := map[string]bool{}
		for _, item := range pack.Items {
			itemIDs[item.ID] = true
		}
		archetypes := map[string]bool{}
		slots := map[string]bool{}
		for i, storyline := range pack.Storylines {
			if storyline.ArchetypeID == "" || archetypes[storyline.ArchetypeID] {
				t.Fatalf("duplicate or empty archetype in %s: %#v", pack.ID, pack.Storylines)
			}
			if storyline.ArcSlot != storylineArcSlots[i] || slots[storyline.ArcSlot] {
				t.Fatalf("unexpected arc slot in %s: %#v", pack.ID, pack.Storylines)
			}
			archetypes[storyline.ArchetypeID] = true
			slots[storyline.ArcSlot] = true
			seenAcrossPacks[storyline.ArchetypeID] = true
			if len(storyline.LinkedCharacters) == 0 || len(storyline.LinkedItems) == 0 {
				t.Fatalf("storyline links are empty: %#v", storyline)
			}
			for _, id := range storyline.LinkedCharacters {
				if !castIDs[id] {
					t.Fatalf("unknown linked cast %s", id)
				}
			}
			for _, id := range storyline.LinkedItems {
				if !itemIDs[id] {
					t.Fatalf("unknown linked item %s", id)
				}
			}
			combined := storyline.Title + storyline.Promise + storyline.Pressure +
				storyline.Opposition + storyline.Turn + storyline.Payoff
			if strings.Contains(combined, "{") || strings.Contains(combined, "}") {
				t.Fatalf("unresolved storyline token: %#v", storyline)
			}
		}
	}
	if len(seenAcrossPacks) < 6 {
		t.Fatalf("storyline archetype variety too low: %#v", seenAcrossPacks)
	}
}

func TestWorldPackAppliesRequestedReferenceAsOneLens(t *testing.T) {
	result, err := GenerateWorldPacks(EngineRequest{Genre: "科幻", Seed: 137, Count: 2, Source: "异形"})
	if err != nil {
		t.Fatalf("GenerateWorldPacks: %v", err)
	}
	for _, pack := range result.Packs {
		if pack.Reference.ID != "alien" {
			t.Fatalf("reference = %q, want alien", pack.Reference.ID)
		}
		if pack.Reference.SourceURL == "" || pack.Reference.CorePattern == "" {
			t.Fatalf("missing researched reference data: %#v", pack.Reference)
		}
		if len(pack.Reference.StoryBias) == 0 {
			t.Fatalf("missing reference storyline biases: %#v", pack.Reference)
		}
		if pack.Premise.ReferenceID != "alien" {
			t.Fatalf("premise reference = %q, want alien", pack.Premise.ReferenceID)
		}
		for _, cast := range pack.Cast {
			if cast.ReferenceID != "alien" {
				t.Fatalf("cast reference = %q, want alien", cast.ReferenceID)
			}
		}
	}
}

func TestReferenceSourceChangesStorylineArchetypes(t *testing.T) {
	starWars, err := GenerateWorldPacks(EngineRequest{Genre: "科幻", Seed: 42, Count: 1, Source: "星战"})
	if err != nil {
		t.Fatalf("GenerateWorldPacks starwars: %v", err)
	}
	alien, err := GenerateWorldPacks(EngineRequest{Genre: "科幻", Seed: 42, Count: 1, Source: "异形"})
	if err != nil {
		t.Fatalf("GenerateWorldPacks alien: %v", err)
	}
	starIDs := make([]string, 0, 3)
	alienIDs := make([]string, 0, 3)
	for _, storyline := range starWars.Packs[0].Storylines {
		starIDs = append(starIDs, storyline.ArchetypeID)
	}
	for _, storyline := range alien.Packs[0].Storylines {
		alienIDs = append(alienIDs, storyline.ArchetypeID)
	}
	if reflect.DeepEqual(starIDs, alienIDs) {
		t.Fatalf("reference source did not change storyline archetypes: %#v", starIDs)
	}
}

func TestReferenceCardsHaveSourcesAndAliases(t *testing.T) {
	cards, err := ReferenceCards("")
	if err != nil {
		t.Fatalf("ReferenceCards: %v", err)
	}
	if len(cards) != 5 {
		t.Fatalf("reference cards = %d, want 5", len(cards))
	}
	for _, card := range cards {
		if card.SourceURL == "" || card.AccessedOn == "" || len(card.StructuralMechanics) < 3 || len(card.StoryBias) == 0 {
			t.Fatalf("incomplete reference card: %#v", card)
		}
	}
	for alias, want := range map[string]string{
		"群星":   "stellaris",
		"战锤":   "warhammer40k",
		"异型":   "alien",
		"星球大战": "starwars",
		"无人深空": "nomanssky",
	} {
		found, err := ReferenceCards(alias)
		if err != nil || len(found) != 1 || found[0].ID != want {
			t.Fatalf("ReferenceCards(%q) = %#v, %v; want %s", alias, found, err, want)
		}
	}
}
