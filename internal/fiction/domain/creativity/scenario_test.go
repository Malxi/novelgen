package creativity

import (
	"reflect"
	"testing"
)

func TestGenerateScenariosIsDeterministicAndValid(t *testing.T) {
	req := ScenarioRequest{Genre: "科幻", Seed: 12, Count: 4}
	first, err := GenerateScenarios(req)
	if err != nil {
		t.Fatalf("GenerateScenarios: %v", err)
	}
	second, err := GenerateScenarios(req)
	if err != nil {
		t.Fatalf("GenerateScenarios: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed produced different scenario cards")
	}
	if len(first.Cards) != 4 {
		t.Fatalf("count = %d, want 4", len(first.Cards))
	}
	for _, card := range first.Cards {
		if err := card.Validate(); err != nil {
			t.Fatalf("invalid card: %v", err)
		}
		if card.ElementRoles["星战"] == "" || card.ElementRoles["虫族"] == "" {
			t.Fatalf("missing element roles: %#v", card.ElementRoles)
		}
	}
}

func TestGenerateScenariosRejectsNonSciFiGenre(t *testing.T) {
	if _, err := GenerateScenarios(ScenarioRequest{Genre: "仙侠", Seed: 1, Count: 1}); err == nil {
		t.Fatal("expected non-sci-fi genre to be rejected")
	}
}

func TestSelectScenarioItemsKeepsFamiliesDistinct(t *testing.T) {
	items := []ScenarioPlanItem{
		{Card: ScenarioCard{ID: "a"}, Family: "f1", Novelty: 90},
		{Card: ScenarioCard{ID: "b"}, Family: "f1", Novelty: 80},
		{Card: ScenarioCard{ID: "c"}, Family: "f2", Novelty: 70},
		{Card: ScenarioCard{ID: "d"}, Family: "f3", Novelty: 60},
	}
	selected := SelectScenarioItems(items, 2)
	if len(selected) != 2 {
		t.Fatalf("selected = %#v", selected)
	}
	if selected[0].Card.ID != "a" || selected[1].Card.ID != "c" {
		t.Fatalf("selected = %#v", selected)
	}
}
