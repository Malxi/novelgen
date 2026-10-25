package creativity

import (
	"reflect"
	"strings"
	"testing"
)

func TestGenerateNameCardsIsDeterministic(t *testing.T) {
	req := NameRequest{Genre: "科幻", Seed: 42, Count: 8}
	first, err := GenerateNameCards(req)
	if err != nil {
		t.Fatalf("GenerateNameCards: %v", err)
	}
	second, err := GenerateNameCards(req)
	if err != nil {
		t.Fatalf("GenerateNameCards: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed produced different name cards")
	}
	if len(first.Cards) != 8 {
		t.Fatalf("cards = %d, want 8", len(first.Cards))
	}
	seen := make(map[string]bool)
	seenArchetypes := make(map[string]bool)
	for _, card := range first.Cards {
		if card.Name == "" || card.Surname == "" || card.GivenName == "" || card.Archetype == "" {
			t.Fatalf("incomplete name card: %#v", card)
		}
		if seen[card.Name] {
			t.Fatalf("duplicate name %q", card.Name)
		}
		if seenArchetypes[card.Archetype] {
			t.Fatalf("duplicate archetype %q", card.Archetype)
		}
		seen[card.Name] = true
		seenArchetypes[card.Archetype] = true
		if len(card.Personality) == 0 || card.Contradiction == "" || card.SpeechStyle == "" ||
			card.BodyLanguage == "" || card.BackstoryHook == "" || card.NamingLogic == "" {
			t.Fatalf("incomplete roleplay fields: %#v", card)
		}
		if !strings.Contains(card.NamingLogic, card.Archetype) || !strings.Contains(card.NamingLogic, card.Name) {
			t.Fatalf("naming logic does not explain archetype and name: %q", card.NamingLogic)
		}
	}
}

func TestGenerateNameCardsUsesRoleArchetypesAndReference(t *testing.T) {
	result, err := GenerateNameCards(NameRequest{
		Genre:          "科幻",
		Seed:           7,
		Count:          4,
		Source:         "异形",
		PackID:         "engine-7-01",
		ReferenceID:    "alien",
		ArchetypeRoles: []string{"protagonist", "antagonist", "ally", "wildcard"},
	})
	if err != nil {
		t.Fatalf("GenerateNameCards: %v", err)
	}
	if len(result.Cards) != 4 {
		t.Fatalf("cards = %d, want 4", len(result.Cards))
	}
	roles := []string{"protagonist", "antagonist", "ally", "wildcard"}
	for i, card := range result.Cards {
		if card.PackID != "engine-7-01" || card.ReferenceID != "alien" {
			t.Fatalf("card %d lost pack/reference: %#v", i, card)
		}
		if !worldPackContains(card.RoleTags, roles[i]) {
			t.Fatalf("card %d archetype %q does not support role %q", i, card.Archetype, roles[i])
		}
	}
}

func TestNameResearchSourcesAreCited(t *testing.T) {
	sources := NameResearchSources()
	if len(sources) < 2 {
		t.Fatalf("name sources = %d, want at least 2", len(sources))
	}
	for _, source := range sources {
		if source["url"] == "" || source["accessed_on"] == "" || source["note"] == "" {
			t.Fatalf("incomplete name source: %#v", source)
		}
	}
}
