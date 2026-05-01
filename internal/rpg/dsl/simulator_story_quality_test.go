package dsl

import (
	"testing"

	"novelgen/internal/models"
)

func TestEscapeActionIsMovementNotCombatSetupRequirement(t *testing.T) {
	if got := eventTypeFromAction("escape"); got != "move" {
		t.Fatalf("escape should map to move, got %q", got)
	}
}

func TestStateAnchorKeyItemsReplaceCurrentInventory(t *testing.T) {
	sim := NewSimulator(&DSL{})
	sim.Context.Protagonist.Items = []string{"old sword", "spent key"}
	sim.Context.Protagonist.Inventory = Capacity{Type: "pack", Current: 2, Max: 20}

	sim.applyStateDeltas([]StateDelta{{
		Kind:  "item",
		Field: "key_items",
		To:    "火种机甲, 密钥-04",
	}})

	if len(sim.Context.Protagonist.Items) != 2 || sim.Context.Protagonist.Inventory.Current != 2 {
		t.Fatalf("expected key_items to replace current inventory, got %+v", sim.Context.Protagonist)
	}
	if sim.Context.Protagonist.Items[0] != "火种机甲" || sim.Context.Protagonist.Items[1] != "密钥-04" {
		t.Fatalf("unexpected items after replacement: %+v", sim.Context.Protagonist.Items)
	}
}

func TestStateAnchorEmitsStructuredGeneAndMechDeltas(t *testing.T) {
	deltas := buildStateAnchorDeltas(models.Chapter{
		StateAnchor: models.StateAnchor{
			Cultivation: "二级基因适配者（基因稳定性65%）",
			KeyItems: []string{
				"基础版火种机甲（能量40%，左腿护甲受损）",
				"近战模块蓝图（已获得）",
				"基础版火种机甲（已解锁近战武器模块·临时）",
			},
		},
	})

	assertDelta := func(kind, field, to string) {
		t.Helper()
		for _, delta := range deltas {
			if delta.Kind == kind && delta.Field == field && delta.To == to {
				return
			}
		}
		t.Fatalf("missing state_delta kind=%s field=%s to=%s in %+v", kind, field, to, deltas)
	}

	assertDelta("gene", "level", "2")
	assertDelta("gene", "stability", "65")
	assertDelta("mech", "form", "基础版火种机甲")
	assertDelta("mech", "level", "2")
	assertDelta("mech", "energy", "40")
	assertDelta("mech", "damage", "左腿护甲受损")
	assertDelta("mech", "module", "近战武器模块")
	assertDelta("mech", "module_blueprint", "近战模块")
}

func TestParseStateDeltaLevelSupportsSciFiProgression(t *testing.T) {
	early := parseStateDeltaLevel("基因适配者（一阶初级，刚觉醒暂未稳定）")
	mid := parseStateDeltaLevel("基因适配者（一阶中级）")
	later := parseStateDeltaLevel("基因强化者（二阶初级）")

	if early <= 0 {
		t.Fatalf("expected sci-fi progression label to parse, got %d", early)
	}
	if !(early < mid && mid < later) {
		t.Fatalf("expected monotonic sci-fi levels, got early=%d mid=%d later=%d", early, mid, later)
	}
}

func TestSimulatorCombatUsesAlliesEquipmentAndDamagedEnemies(t *testing.T) {
	sim := NewSimulator(&DSL{
		Metadata: &Metadata{Phase: string(PhaseOutline), PowerSystem: "sci-fi"},
		Characters: &Characters{
			Player: &Player{
				ID:   "p",
				Name: "Lin",
				Stats: Stats{
					STR: 10,
					AGI: 8,
					INT: 8,
					VIT: 8,
					HP:  120,
				},
			},
			Enemies: []Enemy{{
				ID:          "blade_bug",
				Name:        "Blade Bug",
				Description: "血量剩余40%，三方混战后受损",
				Template: EnemyTemplate{
					HPFormula: "200",
					StatsPerLevel: map[string]int{
						"str": 20,
						"agi": 12,
						"int": 8,
						"vit": 16,
					},
				},
			}},
		},
		World:     &World{},
		Storyline: &Storyline{},
		Systems:   &Systems{},
	})
	sim.initializeProtagonist()
	sim.applyStateDeltas([]StateDelta{
		{Kind: "ally", To: "Su, Tiger"},
		{Kind: "item", To: "重曙机甲"},
	})

	step := &Step{
		Order:       1,
		Description: "Lin uses 重曙机甲 with terrain ambush in a three-way fight.",
		Event: Event{
			Type: "combat",
			Combat: &CombatEvent{Setup: CombatSetup{Enemies: []EnemySpawn{{
				ID:    "blade_bug",
				Count: 2,
			}}}},
			OnComplete: &EventResult{Narration: "victory", Exp: 1},
		},
	}

	sim.checkCombatEvent("C1", step)
	for _, issue := range sim.Issues {
		if issue.Type == IssueBalance {
			t.Fatalf("expected combat modifiers to avoid balance issue, got %+v", issue)
		}
	}
}

func TestSimulatorAppliesStructuredGeneAndMechDeltas(t *testing.T) {
	sim := NewSimulator(&DSL{})

	sim.applyStateDeltas([]StateDelta{
		{Kind: "gene", Field: "level", To: "2"},
		{Kind: "gene", Field: "stability", To: "68"},
		{Kind: "mech", Field: "form", To: "改良版火种机甲"},
		{Kind: "mech", Field: "energy", To: "100"},
		{Kind: "mech", Field: "module", To: "远程模块"},
		{Kind: "mech", Field: "damage", To: "左腿护甲受损"},
		{Kind: "mech", Field: "damage", To: "none"},
	})

	if sim.Context.Protagonist.Gene.Level != 2 || sim.Context.Protagonist.Gene.Stability != 68 {
		t.Fatalf("unexpected gene state: %+v", sim.Context.Protagonist.Gene)
	}
	if sim.Context.Protagonist.Mech.Form != "改良版火种机甲" || sim.Context.Protagonist.Mech.Level != 3 || sim.Context.Protagonist.Mech.Energy != 100 {
		t.Fatalf("unexpected mech state: %+v", sim.Context.Protagonist.Mech)
	}
	if len(sim.Context.Protagonist.Mech.Modules) != 1 || sim.Context.Protagonist.Mech.Modules[0] != "远程模块" {
		t.Fatalf("unexpected mech modules: %+v", sim.Context.Protagonist.Mech.Modules)
	}
	if len(sim.Context.Protagonist.Mech.Damage) != 0 {
		t.Fatalf("expected repaired damage to clear active damage, got %+v", sim.Context.Protagonist.Mech.Damage)
	}
}

func TestStructuredGeneAndMechStateFeedsCombatResolver(t *testing.T) {
	sim := NewSimulator(&DSL{
		Metadata: &Metadata{Phase: string(PhaseOutline), PowerSystem: "sci-fi"},
		Characters: &Characters{
			Player: &Player{
				ID:   "p",
				Name: "Lin",
				Stats: Stats{
					STR: 6,
					AGI: 6,
					INT: 6,
					VIT: 6,
					HP:  80,
				},
			},
			Enemies: []Enemy{{
				ID:   "soldier_bug",
				Name: "Soldier Bug",
				Template: EnemyTemplate{
					HPFormula: "120",
					StatsPerLevel: map[string]int{
						"str": 18,
						"agi": 10,
						"int": 4,
						"vit": 12,
					},
				},
			}},
		},
		World:     &World{},
		Storyline: &Storyline{},
		Systems:   &Systems{},
	})
	sim.initializeProtagonist()
	sim.applyStateDeltas([]StateDelta{
		{Kind: "gene", Field: "level", To: "2"},
		{Kind: "gene", Field: "stability", To: "85"},
		{Kind: "mech", Field: "form", To: "改良版火种机甲"},
		{Kind: "mech", Field: "energy", To: "100"},
		{Kind: "mech", Field: "module", To: "远程模块"},
	})

	step := &Step{
		Order:       1,
		Description: "Lin fights with structured mech state.",
		Event: Event{
			Type: "combat",
			Combat: &CombatEvent{Setup: CombatSetup{Enemies: []EnemySpawn{{
				ID:    "soldier_bug",
				Count: 2,
			}}}},
			OnComplete: &EventResult{Narration: "victory", Exp: 1},
		},
	}

	sim.checkCombatEvent("C1", step)
	for _, issue := range sim.Issues {
		if issue.Type == IssueBalance {
			t.Fatalf("expected structured gene/mech state to avoid balance issue, got %+v", issue)
		}
	}
}

func TestPlotThreadDeferredHorizonDoesNotCountAsCurrentHole(t *testing.T) {
	sim := NewSimulator(&DSL{})
	chapter := &Chapter{Objectives: []Objective{{Steps: []Step{{
		Event: Event{StateDeltas: []StateDelta{{
			Target: "myst_long",
			Kind:   "plot_thread",
			Field:  "mystery",
			To:     "raised",
			Unit:   "series",
			Cost:   "deferred",
		}}},
	}}}}}

	sim.trackPlotThreads(chapter)
	sim.checkPlotThreads()

	if sim.Context.PlotThreadsDeferred != 1 {
		t.Fatalf("expected deferred plot thread to be tracked, got %d", sim.Context.PlotThreadsDeferred)
	}
	for _, issue := range sim.Issues {
		if issue.Type == IssuePlotHole {
			t.Fatalf("deferred plot thread should not emit current plot hole, got %+v", issue)
		}
	}
}

func TestPlotThreadRepeatedPressureCountsAsOneOpenThread(t *testing.T) {
	sim := NewSimulator(&DSL{})
	chapter := &Chapter{Objectives: []Objective{{Steps: []Step{
		{Event: Event{StateDeltas: []StateDelta{{
			Target: "main_arc",
			Kind:   "plot_thread",
			Field:  "storyline",
			To:     "raised",
		}}}},
		{Event: Event{StateDeltas: []StateDelta{{
			Target: "main_arc",
			Kind:   "plot_thread",
			Field:  "storyline",
			To:     "raised",
		}}}},
		{Event: Event{StateDeltas: []StateDelta{{
			Target: "main_arc",
			Kind:   "plot_thread",
			Field:  "storyline",
			To:     "resolved",
		}}}},
	}}}}

	sim.trackPlotThreads(chapter)
	sim.checkPlotThreads()

	if sim.Context.PlotThreadsRaised != 1 || sim.Context.PlotThreadsResolved != 1 {
		t.Fatalf("expected repeated pressure to count once, got raised=%d resolved=%d", sim.Context.PlotThreadsRaised, sim.Context.PlotThreadsResolved)
	}
	for _, issue := range sim.Issues {
		if issue.Type == IssuePlotHole {
			t.Fatalf("resolved repeated pressure should not emit plot hole, got %+v", issue)
		}
	}
}
