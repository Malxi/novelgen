package creativity

import (
	"reflect"
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
		if len(pack.Storylines) != 3 || len(pack.Cast) != 4 || len(pack.Resources) != 2 || len(pack.Items) != 5 {
			t.Fatalf("incomplete decks for %s: %#v", pack.ID, pack)
		}
		categories := map[string]bool{}
		for _, item := range pack.Items {
			categories[item.Category] = true
			if item.ReferenceID != pack.Reference.ID || item.PackID != pack.ID {
				t.Fatalf("foreign item: %#v", item)
			}
		}
		for _, category := range []string{"resource", "equipment", "consumable", "prop", "credential"} {
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

func TestReferenceCardsHaveSourcesAndAliases(t *testing.T) {
	cards, err := ReferenceCards("")
	if err != nil {
		t.Fatalf("ReferenceCards: %v", err)
	}
	if len(cards) != 5 {
		t.Fatalf("reference cards = %d, want 5", len(cards))
	}
	for _, card := range cards {
		if card.SourceURL == "" || card.AccessedOn == "" || len(card.StructuralMechanics) < 3 {
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
