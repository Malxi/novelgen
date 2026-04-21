package dsl

import (
	"testing"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
	if len(v.errors) != 0 {
		t.Error("New validator should have no errors")
	}
}

func TestValidateEmptyDSL(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{}

	err := v.Validate(dsl)
	if err == nil {
		t.Error("Expected validation error for empty DSL")
	}

	if !v.HasErrors() {
		t.Error("Expected HasErrors to return true")
	}
}

func TestValidateMinimalDSL(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title:      "Test",
			DSLVersion: "0.1.0",
		},
		Characters: &Characters{
			Player: &Player{
				ID:   "char_player",
				Name: "Hero",
				Stats: Stats{
					HP:  100,
					STR: 10,
					AGI: 10,
					INT: 10,
					VIT: 10,
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "Chapter 1",
					Objectives: []Objective{
						{
							Name: "Objective 1",
							Type: "sequence",
							Steps: []Step{
								{
									Order: 1,
									Event: Event{
										Type: "spawn",
										Spawn: &SpawnEvent{
											Actor:    "char_player",
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

	err := v.Validate(dsl)
	if err != nil {
		t.Errorf("Expected no errors for valid DSL, got: %v", err)
	}

	if v.HasErrors() {
		t.Errorf("Expected no errors, got %d errors", len(v.GetErrors()))
		for _, e := range v.GetErrors() {
			t.Logf("Error: %s", e.Error())
		}
	}
}

func TestValidateMissingMetadata(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Characters: &Characters{
			Player: &Player{
				Name: "Hero",
				Stats: Stats{
					HP: 100,
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "Chapter 1",
				},
			},
		},
	}

	err := v.Validate(dsl)
	if err == nil {
		t.Error("Expected error for missing metadata")
	}

	errors := v.GetErrors()
	found := false
	for _, e := range errors {
		if e.Field == "metadata" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'metadata' error")
	}
}

func TestValidateMissingPlayer(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title:      "Test",
			DSLVersion: "0.1.0",
		},
		Characters: &Characters{
			Enemies: []Enemy{
				{ID: "enemy_001", Name: "Enemy"},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "Chapter 1",
				},
			},
		},
	}

	err := v.Validate(dsl)
	if err == nil {
		t.Error("Expected error for missing player")
	}

	errors := v.GetErrors()
	found := false
	for _, e := range errors {
		if e.Field == "characters.player" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'characters.player' error")
	}
}

func TestValidateDuplicateLocationID(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title:      "Test",
			DSLVersion: "0.1.0",
		},
		World: &World{
			Locations: []Location{
				{ID: "loc_001", Name: "Location 1"},
				{ID: "loc_001", Name: "Location 2"}, // Duplicate ID
			},
		},
		Characters: &Characters{
			Player: &Player{
				Name: "Hero",
				Stats: Stats{
					HP: 100,
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "Chapter 1",
				},
			},
		},
	}

	v.Validate(dsl)

	errors := v.GetErrors()
	found := false
	for _, e := range errors {
		if e.Field == "world.locations[1].id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for duplicate location ID")
	}
}

func TestValidateMissingChapterTitle(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title:      "Test",
			DSLVersion: "0.1.0",
		},
		Characters: &Characters{
			Player: &Player{
				Name: "Hero",
				Stats: Stats{
					HP: 100,
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "", // Empty title
				},
			},
		},
	}

	v.Validate(dsl)

	errors := v.GetErrors()
	found := false
	for _, e := range errors {
		if e.Field == "storyline.chapters[0].title" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for missing chapter title")
	}
}

func TestValidateMissingEventType(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title:      "Test",
			DSLVersion: "0.1.0",
		},
		Characters: &Characters{
			Player: &Player{
				Name: "Hero",
				Stats: Stats{
					HP: 100,
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "Chapter 1",
					Objectives: []Objective{
						{
							Name: "Objective 1",
							Steps: []Step{
								{
									Order: 1,
									Event: Event{
										Type: "", // Empty type
									},
								},
							},
						},
					},
				},
			},
		},
	}

	v.Validate(dsl)

	errors := v.GetErrors()
	found := false
	for _, e := range errors {
		if e.Field == "storyline.chapters[0].objectives[0].steps[0].event.type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for missing event type")
	}
}

func TestValidateSpawnEventMissingActor(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title:      "Test",
			DSLVersion: "0.1.0",
		},
		Characters: &Characters{
			Player: &Player{
				Name: "Hero",
				Stats: Stats{
					HP: 100,
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "Chapter 1",
					Objectives: []Objective{
						{
							Name: "Objective 1",
							Steps: []Step{
								{
									Order: 1,
									Event: Event{
										Type: "spawn",
										Spawn: &SpawnEvent{
											Actor:    "", // Empty actor
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

	v.Validate(dsl)

	errors := v.GetErrors()
	found := false
	for _, e := range errors {
		if e.Field == "storyline.chapters[0].objectives[0].steps[0].event.actor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for missing spawn actor")
	}
}

func TestValidateWarnings(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title: "Test",
			// Missing dsl_version - should generate warning
		},
		Characters: &Characters{
			Player: &Player{
				Name: "Hero",
				Stats: Stats{
					// Missing HP - should generate warning
					STR: 10,
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "Chapter 1",
					// No objectives - should generate warning
				},
			},
		},
	}

	v.Validate(dsl)

	if !v.HasWarnings() {
		t.Error("Expected warnings for incomplete DSL")
	}

	warnings := v.GetWarnings()
	if len(warnings) == 0 {
		t.Error("Expected at least one warning")
	}
}

func TestValidateCompleteStory(t *testing.T) {
	v := NewValidator()
	dsl := &DSL{
		Metadata: &Metadata{
			Title:       "逐星",
			Genre:       []string{"科幻", "废土"},
			PowerSystem: "基因进化",
			DSLVersion:  "0.1.0",
		},
		World: &World{
			Locations: []Location{
				{
					ID:          "loc_base",
					Name:        "休眠基地",
					Type:        "indoor",
					Description: "废弃的休眠设施",
				},
				{
					ID:   "loc_wasteland",
					Name: "废土",
					Type: "outdoor",
				},
			},
		},
		Characters: &Characters{
			Player: &Player{
				ID:    "char_player",
				Name:  "陆沉",
				Class: "engineer",
				Stats: Stats{
					STR: 10,
					AGI: 12,
					INT: 15,
					VIT: 10,
					HP:  100,
					MP:  50,
				},
			},
			Enemies: []Enemy{
				{
					ID:   "enemy_wasp",
					Name: "虫族工蜂",
					Type: "insect",
					Template: EnemyTemplate{
						BaseLevel: 1,
						HPFormula: "50 + level * 10",
						StatsPerLevel: map[string]int{
							"str": 2,
							"agi": 3,
						},
					},
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:    "chap_001",
					Title: "冷舱梦醒",
					Objectives: []Objective{
						{
							Name: "逃离休眠基地",
							Type: "sequence",
							Steps: []Step{
								{
									Order:       1,
									Description: "从休眠仓中醒来",
									Event: Event{
										Type: "spawn",
										Spawn: &SpawnEvent{
											Actor:    "char_player",
											Location: "loc_base",
										},
										OnComplete: &EventResult{
											Narration: "你醒来时，休眠仓的指示灯闪烁着红光...",
										},
									},
								},
								{
									Order:       2,
									Description: "离开基地",
									Event: Event{
										Type: "move",
										Move: &MoveEvent{
											Actor: "char_player",
											To:    "loc_wasteland",
										},
										OnComplete: &EventResult{
											Narration: "废土的风沙扑面而来...",
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

	err := v.Validate(dsl)
	if err != nil {
		t.Errorf("Expected no errors for complete story, got: %v", err)
		for _, e := range v.GetErrors() {
			t.Logf("Error: %s", e.Error())
		}
	}

	if v.HasErrors() {
		t.Errorf("Expected no errors, got %d errors", len(v.GetErrors()))
	}
}