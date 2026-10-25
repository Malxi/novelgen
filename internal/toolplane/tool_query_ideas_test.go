package toolplane

import (
	"strings"
	"testing"

	"novelgen/internal/fiction/domain/creativity"
)

func TestQueryIdeasReturnsReplayableProvocations(t *testing.T) {
	resp, err := QueryIdeas("axiom", "仙侠", "", "wild", 42, 3, 0, "", "")
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
	if _, err := QueryIdeas("nonsense", "", "", "", 1, 1, 0, "", ""); err == nil {
		t.Fatal("expected unknown ideas type error")
	}
}

func TestQueryIdeasLawReturnsSystemicContract(t *testing.T) {
	resp, err := QueryIdeas("law", "仙侠", "", "wild", 42, 3, 0, "", "")
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
	resp, err := QueryIdeas("scenario", "科幻", "", "", 12, 3, 0, "", "")
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
	resp, err := QueryIdeas("scenario", "科幻", "", "", 12, 10, 3, "", "")
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

func TestQueryIdeasWorldPackReturnsCompatibleBundle(t *testing.T) {
	resp, err := QueryIdeas("worldpack", "科幻", "", "", 42, 3, 0, "星战", "")
	if err != nil {
		t.Fatalf("QueryIdeas worldpack: %v", err)
	}
	packs, ok := resp.Results.([]creativity.WorldPack)
	if !ok || len(packs) != 3 {
		t.Fatalf("results = %#v", resp.Results)
	}
	for _, pack := range packs {
		if !pack.Compatibility.Compatible {
			t.Fatalf("incompatible pack: %#v", pack.Compatibility)
		}
		if issues := creativity.ValidateWorldPack(pack); len(issues) > 0 {
			t.Fatalf("invalid pack %s: %v", pack.ID, issues)
		}
		if pack.Reference.ID != "starwars" {
			t.Fatalf("reference = %q, want starwars", pack.Reference.ID)
		}
		for _, resource := range pack.Resources {
			if resource.ReferenceID != "starwars" {
				t.Fatalf("resource reference = %q, want starwars", resource.ReferenceID)
			}
		}
	}
	if got := resp.Meta["packs"]; got != 3 {
		t.Fatalf("meta packs = %#v, want 3", got)
	}
	replay, _ := resp.Meta["replay_command"].(string)
	if !strings.Contains(replay, `--source "星战"`) {
		t.Fatalf("replay command lost source: %q", replay)
	}
}

func TestQueryIdeasWorldDecksSharePackIDs(t *testing.T) {
	packIDs := func(resp ToolResponse) map[string]bool {
		out := map[string]bool{}
		switch items := resp.Results.(type) {
		case []creativity.PremiseCard:
			for _, item := range items {
				out[item.PackID] = true
			}
		case []creativity.StorylineCard:
			for _, item := range items {
				out[item.PackID] = true
			}
		case []creativity.CastCard:
			for _, item := range items {
				out[item.PackID] = true
			}
		case []creativity.ResourceCard:
			for _, item := range items {
				out[item.PackID] = true
			}
		}
		return out
	}

	want := map[string]bool{}
	for _, queryType := range []string{"premise", "storyline", "cast", "resource"} {
		resp, err := QueryIdeas(queryType, "科幻", "", "", 42, 3, 0, "", "")
		if err != nil {
			t.Fatalf("QueryIdeas %s: %v", queryType, err)
		}
		got := packIDs(resp)
		if len(got) != 3 {
			t.Fatalf("%s pack ids = %#v, want 3", queryType, got)
		}
		if len(want) == 0 {
			want = got
			continue
		}
		if len(got) != len(want) {
			t.Fatalf("%s pack count differs", queryType)
		}
		for id := range want {
			if !got[id] {
				t.Fatalf("%s missing pack id %s", queryType, id)
			}
		}
	}
}

func TestQueryIdeasEngineReturnsScaffold(t *testing.T) {
	resp, err := QueryIdeas("engine", "都市", "", "", 7, 2, 0, "", "")
	if err != nil {
		t.Fatalf("QueryIdeas engine: %v", err)
	}
	engines, ok := resp.Results.([]creativity.Engine)
	if !ok || len(engines) != 2 {
		t.Fatalf("results = %#v", resp.Results)
	}
	for _, engine := range engines {
		if engine.PremiseScaffold == "" || len(engine.StorylineSeeds) < 3 || len(engine.CastSeeds) < 4 {
			t.Fatalf("incomplete engine: %#v", engine)
		}
	}
}

func TestQueryIdeasReferenceReturnsResearchedPatterns(t *testing.T) {
	resp, err := QueryIdeas("reference", "科幻", "", "", 0, 5, 0, "", "")
	if err != nil {
		t.Fatalf("QueryIdeas reference: %v", err)
	}
	cards, ok := resp.Results.([]creativity.ReferenceCard)
	if !ok || len(cards) < 5 {
		t.Fatalf("results = %#v", resp.Results)
	}
	want := map[string]bool{
		"stellaris":    false,
		"warhammer40k": false,
		"alien":        false,
		"starwars":     false,
		"nomanssky":    false,
	}
	for _, card := range cards {
		if card.SourceURL == "" || card.CorePattern == "" || len(card.StructuralMechanics) < 3 {
			t.Fatalf("incomplete reference card: %#v", card)
		}
		if _, ok := want[card.ID]; ok {
			want[card.ID] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Fatalf("missing reference card %s", id)
		}
	}
}

func TestQueryIdeasReferenceAliasAndUnknownSource(t *testing.T) {
	resp, err := QueryIdeas("reference", "科幻", "", "", 0, 1, 0, "无人深空", "")
	if err != nil {
		t.Fatalf("QueryIdeas reference alias: %v", err)
	}
	cards, ok := resp.Results.([]creativity.ReferenceCard)
	if !ok || len(cards) != 1 || cards[0].ID != "nomanssky" {
		t.Fatalf("results = %#v", resp.Results)
	}
	if _, err := QueryIdeas("worldpack", "科幻", "", "", 1, 1, 0, "not-a-setting", ""); err == nil {
		t.Fatal("expected unknown reference source error")
	}
}

func TestQueryIdeasNamecardReturnsRoleplayCards(t *testing.T) {
	resp, err := QueryIdeas("namecard", "科幻", "", "", 42, 5, 0, "星战", "")
	if err != nil {
		t.Fatalf("QueryIdeas namecard: %v", err)
	}
	cards, ok := resp.Results.([]creativity.NameCard)
	if !ok || len(cards) != 5 {
		t.Fatalf("results = %#v", resp.Results)
	}
	for _, card := range cards {
		if card.Name == "" || card.Archetype == "" || len(card.Personality) == 0 ||
			card.ReferenceID != "starwars" {
			t.Fatalf("incomplete namecard: %#v", card)
		}
	}
	sources, ok := resp.Meta["name_sources"].([]map[string]string)
	if !ok || len(sources) < 2 {
		t.Fatalf("name sources = %#v", resp.Meta["name_sources"])
	}
	replay, _ := resp.Meta["replay_command"].(string)
	if !strings.Contains(replay, `--source "星战"`) {
		t.Fatalf("replay command lost source: %q", replay)
	}
}

func TestQueryIdeasItemcardFiltersCategoryAndReplays(t *testing.T) {
	resp, err := QueryIdeas("itemcard", "科幻", "", "", 42, 4, 0, "战锤", "装备")
	if err != nil {
		t.Fatalf("QueryIdeas itemcard: %v", err)
	}
	items, ok := resp.Results.([]creativity.ItemCard)
	if !ok || len(items) != 4 {
		t.Fatalf("results = %#v", resp.Results)
	}
	for _, item := range items {
		if item.Category != "equipment" || item.ReferenceID != "warhammer40k" {
			t.Fatalf("unexpected item: %#v", item)
		}
	}
	categories, ok := resp.Meta["categories"].([]string)
	if !ok || len(categories) < 7 {
		t.Fatalf("categories = %#v", resp.Meta["categories"])
	}
	replay, _ := resp.Meta["replay_command"].(string)
	if !strings.Contains(replay, `--source "战锤"`) || !strings.Contains(replay, `--category "装备"`) {
		t.Fatalf("replay command = %q", replay)
	}
}

func TestQueryIdeasItemcardRejectsUnknownCategory(t *testing.T) {
	if _, err := QueryIdeas("itemcard", "科幻", "", "", 1, 1, 0, "", "不存在的分类"); err == nil {
		t.Fatal("expected unknown item category error")
	}
}

func TestQueryIdeasDefaultReplayCommandUsesResolvedLevel(t *testing.T) {
	resp, err := QueryIdeas("", "悬疑", "event", "wild", 9, 1, 0, "", "")
	if err != nil {
		t.Fatalf("QueryIdeas: %v", err)
	}
	replay, ok := resp.Meta["replay_command"].(string)
	if !ok || !strings.Contains(replay, "--type event") {
		t.Fatalf("replay command = %#v", resp.Meta["replay_command"])
	}
}
