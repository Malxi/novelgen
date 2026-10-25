package creativity

import (
	"reflect"
	"strings"
	"testing"
)

func TestGenerateItemCardsDeterministicAndCoversCategories(t *testing.T) {
	req := ItemRequest{Genre: "科幻", Seed: 42, Count: 7, Source: "星战"}
	first, err := GenerateItemCards(req)
	if err != nil {
		t.Fatalf("GenerateItemCards: %v", err)
	}
	second, err := GenerateItemCards(req)
	if err != nil {
		t.Fatalf("GenerateItemCards: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed produced different item cards")
	}
	seen := map[string]bool{}
	for _, item := range first.Items {
		if item.Name == "" || item.BaseObject == "" || item.Category == "" || item.Function == "" ||
			item.UseCost == "" || item.Limit == "" || item.Counterplay == "" ||
			item.FailureMode == "" || len(item.NarrativeUses) < 2 {
			t.Fatalf("incomplete item card: %#v", item)
		}
		if !worldPackContains(itemPhysicalForms[item.BaseObject], item.PhysicalForm) {
			t.Fatalf("physical form %q does not belong to item %q", item.PhysicalForm, item.BaseObject)
		}
		if item.ReferenceID != "starwars" {
			t.Fatalf("reference = %q, want starwars", item.ReferenceID)
		}
		seen[item.Category] = true
	}
	for _, category := range ItemCategories() {
		if !seen[category] {
			t.Fatalf("missing category %q", category)
		}
	}
}

func TestGenerateItemCardsCategoryAliases(t *testing.T) {
	for alias, want := range map[string]string{
		"装备":        "equipment",
		"消耗品":       "consumable",
		"道具":        "prop",
		"载具":        "vehicle",
		"凭证":        "credential",
		"资源":        "resource",
		"commodity": "commodity",
	} {
		result, err := GenerateItemCards(ItemRequest{Genre: "科幻", Seed: 9, Count: 3, Category: alias})
		if err != nil {
			t.Fatalf("category %q: %v", alias, err)
		}
		for _, item := range result.Items {
			if item.Category != want {
				t.Fatalf("category %q -> %q, want %q", alias, item.Category, want)
			}
		}
	}
	if _, err := GenerateItemCards(ItemRequest{Genre: "科幻", Seed: 1, Count: 1, Category: "spaceship-of-the-gaps"}); err == nil {
		t.Fatal("expected unknown category error")
	}
}

func TestGenerateItemCardsAvoidsDuplicateBaseObjectsInCategory(t *testing.T) {
	result, err := GenerateItemCards(ItemRequest{Genre: "科幻", Seed: 42, Count: 4, Source: "战锤", Category: "equipment"})
	if err != nil {
		t.Fatalf("GenerateItemCards: %v", err)
	}
	seen := make(map[string]bool)
	for _, item := range result.Items {
		if item.Category != "equipment" {
			t.Fatalf("category = %q, want equipment", item.Category)
		}
		if seen[item.BaseObject] {
			t.Fatalf("duplicate base object %q", item.BaseObject)
		}
		seen[item.BaseObject] = true
	}
}

func TestGenerateItemCardsLinksWorldPackEngine(t *testing.T) {
	engine := Engine{
		ID:             "engine-test-01",
		Genre:          "科幻",
		Family:         "salvage",
		Artifact:       "残骸打捞权证",
		Issuer:         "航道打捞公会",
		HiddenFunction: "把沉船变成保险资产",
	}
	result, err := GenerateItemCards(ItemRequest{
		Genre:       engine.Genre,
		Seed:        11,
		Count:       5,
		Source:      "异形",
		PackID:      engine.ID,
		ReferenceID: "alien",
		Engine:      &engine,
	})
	if err != nil {
		t.Fatalf("GenerateItemCards: %v", err)
	}
	if len(result.Items) != 5 {
		t.Fatalf("items = %d, want 5", len(result.Items))
	}
	for _, item := range result.Items {
		if item.PackID != engine.ID || item.ReferenceID != "alien" {
			t.Fatalf("item lost pack/reference: %#v", item)
		}
		if !strings.Contains(item.EngineLink, engine.Artifact) {
			t.Fatalf("item does not reference engine artifact: %#v", item)
		}
	}
}
