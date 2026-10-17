package dsl

import (
	"strings"
	"testing"
)

func TestValidatorRejectsNilDSL(t *testing.T) {
	v := NewValidator()
	if err := v.Validate(nil); err == nil || !strings.Contains(err.Error(), "dsl document is required") {
		t.Fatalf("expected nil DSL error, got %v", err)
	}
	if len(v.GetErrors()) != 1 {
		t.Fatalf("expected one error for nil DSL, got %+v", v.GetErrors())
	}
}

func TestValidatorAcceptsWellFormedDSL(t *testing.T) {
	dsl := newWellFormedValidationDSL()
	v := NewValidator()
	if err := v.Validate(dsl); err != nil {
		t.Fatalf("expected valid DSL, got %v", err)
	}
	if v.HasErrors() {
		t.Fatalf("expected no validation errors, got %+v", v.GetErrors())
	}
	if v.HasWarnings() {
		t.Fatalf("expected no validation warnings, got %+v", v.GetWarnings())
	}
}

func TestValidatorFindsCrossReferencesAndDeltaProblems(t *testing.T) {
	dsl := newWellFormedValidationDSL()
	step := &dsl.Storyline.Chapters[0].Objectives[0].Steps[0]
	step.Event.Type = "move"
	step.Event.Spawn = nil
	step.Event.Move = &MoveEvent{
		Actor: "Hero",
		From:  "loc_start",
		To:    "loc_missing",
	}
	step.Event.Require = &Requirement{Items: []string{"item_missing"}}
	step.Event.OnComplete = &EventResult{Items: []string{"item_missing"}}
	step.Event.StateDeltas = []StateDelta{{
		Kind:  "resource",
		From:  "3",
		To:    "1",
		Delta: -1,
	}}
	dsl.Storyline.Arcs[0].Chapters = append(dsl.Storyline.Arcs[0].Chapters, "chapter_missing")
	dsl.Storyline.Chapters[0].Completion.Items = []string{"item_missing"}
	dsl.Characters.Enemies[0].Drops.Fixed[0].Item = "item_missing"

	v := NewValidator()
	if err := v.Validate(dsl); err == nil {
		t.Fatalf("expected validation failures")
	}

	assertErrorContains(t, v.GetErrors(), "undefined location: loc_missing")
	assertErrorContains(t, v.GetErrors(), "undefined item: item_missing")
	assertErrorContains(t, v.GetErrors(), "delta mismatch")
	assertErrorContains(t, v.GetErrors(), "undefined chapter: chapter_missing")
}

func TestValidatorFlagsSystemDefinitionProblems(t *testing.T) {
	dsl := newWellFormedValidationDSL()
	dsl.Systems = &Systems{
		AttributeSystem: &AttributeSystem{
			ID:   "attr_core",
			Name: "Core Attributes",
			Attributes: []AttributeDef{
				{ID: "qi", Name: "Qi", Type: "resource", BaseValue: 10},
				{ID: "qi", Name: "", Type: "bogus", BaseValue: 0},
			},
		},
		PowerFormula: &PowerFormula{
			ID:   "power_core",
			Name: "Power Core",
			Factors: []Factor{
				{Attribute: "", Name: "", Weight: 0},
			},
		},
	}

	v := NewValidator()
	if err := v.Validate(dsl); err == nil {
		t.Fatalf("expected invalid systems")
	}

	assertErrorContains(t, v.GetErrors(), "duplicate attribute id: qi")
	assertWarningContains(t, v.GetWarnings(), "unknown attribute type: bogus")
	assertWarningContains(t, v.GetWarnings(), "power factor weight is zero")
}

func newWellFormedValidationDSL() *DSL {
	return &DSL{
		Metadata: &Metadata{
			Title:      "Validation Story",
			Genre:      []string{"xianxia"},
			DSLVersion: "0.1.0",
		},
		World: &World{
			Locations: []Location{
				{ID: "loc_start", Name: "Start", Type: "city"},
				{ID: "loc_field", Name: "Field", Type: "outdoor"},
			},
			Items: []Item{
				{ID: "item_key", Name: "Key", Type: "quest", Rarity: "common"},
			},
		},
		Characters: &Characters{
			Player: &Player{
				ID:   "hero",
				Name: "Hero",
				Stats: Stats{
					STR: 10,
					AGI: 10,
					INT: 10,
					VIT: 10,
					HP:  100,
					MP:  20,
				},
			},
			Enemies: []Enemy{
				{
					ID:             "enemy_wolf",
					Name:           "Wolf",
					SpawnLocations: []string{"loc_field"},
					Drops: Drops{
						Fixed: []FixedDrop{{Item: "item_key"}},
					},
				},
			},
			NPCs: []NPC{
				{ID: "npc_guide", Name: "Guide", DefaultLocation: "loc_start"},
			},
		},
		Storyline: &Storyline{
			Arcs: []Arc{
				{ID: "arc_main", Name: "Main Arc", Position: 1, Chapters: []string{"chapter_1"}},
			},
			Chapters: []Chapter{
				{
					ID:       "chapter_1",
					Title:    "Beginning",
					Arc:      "arc_main",
					Position: 1,
					Objectives: []Objective{
						{
							Name: "Open",
							Type: "sequence",
							Steps: []Step{
								{
									Order:       1,
									Description: "The hero arrives",
									Event: Event{
										Type: "spawn",
										Spawn: &SpawnEvent{
											Actor:    "Hero",
											Location: "loc_start",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func assertErrorContains(t *testing.T, errors []ValidationError, needle string) {
	t.Helper()
	for _, err := range errors {
		if strings.Contains(err.Field, needle) || strings.Contains(err.Message, needle) {
			return
		}
	}
	t.Fatalf("expected error containing %q, got %+v", needle, errors)
}

func assertWarningContains(t *testing.T, warnings []ValidationWarning, needle string) {
	t.Helper()
	for _, warning := range warnings {
		if strings.Contains(warning.Field, needle) || strings.Contains(warning.Message, needle) {
			return
		}
	}
	t.Fatalf("expected warning containing %q, got %+v", needle, warnings)
}
