package toolplane

import (
	"strings"
	"testing"

	"novelgen/internal/fiction/domain/creativity"
)

func TestQueryIdeasReturnsReplayableProvocations(t *testing.T) {
	resp, err := QueryIdeas("axiom", "仙侠", "", "wild", 42, 3, 0)
	if err != nil {
		t.Fatalf("QueryIdeas: %v", err)
	}
	if !resp.OK || resp.Count != 3 {
		t.Fatalf("response = %#v", resp)
	}
	items, ok := resp.Results.([]creativity.Provocation)
	if !ok {
		t.Fatalf("results type = %T", resp.Results)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	for _, item := range items {
		if item.Seed != int64(42) {
			t.Fatalf("item seed = %d, want 42", item.Seed)
		}
		if item.Sentence == "" || item.RequiredRole == "" {
			t.Fatalf("incomplete item: %#v", item)
		}
	}
	if got := resp.Meta["seed"]; got != int64(42) {
		t.Fatalf("meta seed = %#v, want 42", got)
	}
}

func TestQueryIdeasRejectsUnknownType(t *testing.T) {
	if _, err := QueryIdeas("nonsense", "", "", "", 1, 1, 0); err == nil {
		t.Fatal("expected unknown ideas type error")
	}
}

func TestQueryIdeasLawReturnsSystemicContract(t *testing.T) {
	resp, err := QueryIdeas("law", "仙侠", "", "wild", 42, 3, 0)
	if err != nil {
		t.Fatalf("QueryIdeas law: %v", err)
	}
	items, ok := resp.Results.([]creativity.Provocation)
	if !ok || len(items) != 3 {
		t.Fatalf("results = %#v", resp.Results)
	}
	for _, item := range items {
		if item.Level != creativity.LevelLaw {
			t.Fatalf("level = %q, want law", item.Level)
		}
		if item.AxiomBudget != 1 || len(item.DerivationSteps) == 0 || len(item.RejectIf) == 0 {
			t.Fatalf("missing systemic contract: %#v", item)
		}
	}
}

func TestQueryIdeasScenarioReturnsCausalCards(t *testing.T) {
	resp, err := QueryIdeas("scenario", "科幻", "", "", 12, 3, 0)
	if err != nil {
		t.Fatalf("QueryIdeas scenario: %v", err)
	}
	cards, ok := resp.Results.([]creativity.ScenarioCard)
	if !ok || len(cards) != 3 {
		t.Fatalf("results = %#v", resp.Results)
	}
	for _, card := range cards {
		if err := card.Validate(); err != nil {
			t.Fatalf("invalid card: %v", err)
		}
	}
}

func TestQueryIdeasScenarioSelectReturnsDiversePlan(t *testing.T) {
	resp, err := QueryIdeas("scenario", "科幻", "", "", 12, 10, 3)
	if err != nil {
		t.Fatalf("QueryIdeas scenario plan: %v", err)
	}
	items, ok := resp.Results.([]creativity.ScenarioPlanItem)
	if !ok || len(items) != 3 {
		t.Fatalf("results = %#v", resp.Results)
	}
	families := map[string]bool{}
	for _, item := range items {
		if item.Family == "" || item.Card.ID == "" {
			t.Fatalf("incomplete plan item: %#v", item)
		}
		families[item.Family] = true
	}
	if len(families) != 3 {
		t.Fatalf("families = %#v, want 3 distinct", families)
	}
}

func TestQueryIdeasDefaultReplayCommandUsesResolvedLevel(t *testing.T) {
	resp, err := QueryIdeas("", "悬疑", "event", "wild", 9, 1, 0)
	if err != nil {
		t.Fatalf("QueryIdeas: %v", err)
	}
	replay, ok := resp.Meta["replay_command"].(string)
	if !ok || !strings.Contains(replay, "--type event") {
		t.Fatalf("replay command = %#v", resp.Meta["replay_command"])
	}
}
