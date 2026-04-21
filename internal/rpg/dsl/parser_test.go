package dsl

import (
	"testing"
)

func TestNewParser(t *testing.T) {
	content := `metadata { title = "Test" }`
	parser := NewParser(content)
	if parser == nil {
		t.Fatal("NewParser returned nil")
	}
	if parser.content != content {
		t.Error("Parser content not set correctly")
	}
}

func TestParseJSON(t *testing.T) {
	jsonContent := `{
		"metadata": {
			"title": "Test Story",
			"dsl_version": "0.1.0"
		},
		"characters": {
			"player": {
				"id": "char_player",
				"name": "Hero",
				"class": "warrior",
				"stats": {
					"hp": 100,
					"str": 10
				}
			}
		},
		"storyline": {
			"chapters": [
				{
					"id": "chap_001",
					"title": "Chapter 1"
				}
			]
		}
	}`

	parser := NewParser(jsonContent)
	dsl, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if dsl.Metadata.Title != "Test Story" {
		t.Errorf("Expected title 'Test Story', got '%s'", dsl.Metadata.Title)
	}

	if dsl.Characters.Player.Name != "Hero" {
		t.Errorf("Expected player name 'Hero', got '%s'", dsl.Characters.Player.Name)
	}

	if len(dsl.Storyline.Chapters) != 1 {
		t.Errorf("Expected 1 chapter, got %d", len(dsl.Storyline.Chapters))
	}
}

func TestParseDSLMetadata(t *testing.T) {
	dslContent := `
metadata {
	title = "Test Story"
	subtitle = "A test story"
	power_system = "magic"
	tone = "dark"
	dsl_version = "0.1.0"
	genre = ["fantasy", "adventure"]
}`

	parser := NewParser(dslContent)
	dsl, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	if dsl.Metadata == nil {
		t.Fatal("Metadata is nil")
	}

	if dsl.Metadata.Title != "Test Story" {
		t.Errorf("Expected title 'Test Story', got '%s'", dsl.Metadata.Title)
	}

	if dsl.Metadata.Subtitle != "A test story" {
		t.Errorf("Expected subtitle 'A test story', got '%s'", dsl.Metadata.Subtitle)
	}

	if dsl.Metadata.PowerSystem != "magic" {
		t.Errorf("Expected power_system 'magic', got '%s'", dsl.Metadata.PowerSystem)
	}

	if len(dsl.Metadata.Genre) != 2 {
		t.Errorf("Expected 2 genres, got %d", len(dsl.Metadata.Genre))
	}
}

func TestParseDSLPlayer(t *testing.T) {
	dslContent := `
characters {
	player "Hero" {
		id = "char_player"
		class = "warrior"
		str = 15
		agi = 12
		hp = 120
		skills = ["slash", "block"]
	}
}`

	parser := NewParser(dslContent)
	dsl, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	if dsl.Characters == nil {
		t.Fatal("Characters is nil")
	}

	if dsl.Characters.Player == nil {
		t.Fatal("Player is nil")
	}

	if dsl.Characters.Player.Name != "Hero" {
		t.Errorf("Expected name 'Hero', got '%s'", dsl.Characters.Player.Name)
	}

	if dsl.Characters.Player.Class != "warrior" {
		t.Errorf("Expected class 'warrior', got '%s'", dsl.Characters.Player.Class)
	}

	if dsl.Characters.Player.Stats.STR != 15 {
		t.Errorf("Expected STR 15, got %d", dsl.Characters.Player.Stats.STR)
	}

	if dsl.Characters.Player.Stats.HP != 120 {
		t.Errorf("Expected HP 120, got %d", dsl.Characters.Player.Stats.HP)
	}

	if len(dsl.Characters.Player.Skills) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(dsl.Characters.Player.Skills))
	}
}

func TestParseDSLEnemy(t *testing.T) {
	dslContent := `
characters {
	enemy "Goblin" {
		id = "enemy_goblin"
		type = "humanoid"
		str = 8
		agi = 10
		hp = 50
	}
}`

	parser := NewParser(dslContent)
	dsl, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	if len(dsl.Characters.Enemies) != 1 {
		t.Fatalf("Expected 1 enemy, got %d", len(dsl.Characters.Enemies))
	}

	enemy := dsl.Characters.Enemies[0]
	if enemy.Name != "Goblin" {
		t.Errorf("Expected name 'Goblin', got '%s'", enemy.Name)
	}

	if enemy.ID != "enemy_goblin" {
		t.Errorf("Expected id 'enemy_goblin', got '%s'", enemy.ID)
	}
}

func TestParseDSLChapter(t *testing.T) {
	dslContent := `
storyline {
	chapter "The Beginning" {
		id = "chap_001"
		
		objective "Start journey" {
			step 1 {
				description = "Spawn player"
				event {
					type = "spawn"
					actor = "player"
					location = "loc_start"
				}
			}
			
			step 2 {
				description = "First battle"
				event {
					type = "combat"
					enemies = []
				}
			}
		}
	}
}`

	parser := NewParser(dslContent)
	dsl, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	if len(dsl.Storyline.Chapters) != 1 {
		t.Fatalf("Expected 1 chapter, got %d", len(dsl.Storyline.Chapters))
	}

	chapter := dsl.Storyline.Chapters[0]
	if chapter.Title != "The Beginning" {
		t.Errorf("Expected title 'The Beginning', got '%s'", chapter.Title)
	}

	if chapter.ID != "chap_001" {
		t.Errorf("Expected id 'chap_001', got '%s'", chapter.ID)
	}

	if len(chapter.Objectives) != 1 {
		t.Fatalf("Expected 1 objective, got %d", len(chapter.Objectives))
	}

	obj := chapter.Objectives[0]
	if obj.Name != "Start journey" {
		t.Errorf("Expected objective name 'Start journey', got '%s'", obj.Name)
	}

	if len(obj.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(obj.Steps))
	}
}

func TestParseDSLLocation(t *testing.T) {
	dslContent := `
world {
	location "Town Square" {
		id = "loc_town_square"
		type = "outdoor"
		description = "The center of town"
	}
}`

	parser := NewParser(dslContent)
	dsl, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	if len(dsl.World.Locations) != 1 {
		t.Fatalf("Expected 1 location, got %d", len(dsl.World.Locations))
	}

	loc := dsl.World.Locations[0]
	if loc.Name != "Town Square" {
		t.Errorf("Expected name 'Town Square', got '%s'", loc.Name)
	}

	if loc.ID != "loc_town_square" {
		t.Errorf("Expected id 'loc_town_square', got '%s'", loc.ID)
	}

	if loc.Type != "outdoor" {
		t.Errorf("Expected type 'outdoor', got '%s'", loc.Type)
	}
}

func TestParseCompleteDSL(t *testing.T) {
	dslContent := `
metadata {
	title = "逐星"
	genre = ["科幻", "废土"]
	dsl_version = "0.1.0"
}

world {
	location "休眠基地" {
		id = "loc_base"
		type = "indoor"
	}
}

characters {
	player "陆沉" {
		id = "char_player"
		class = "engineer"
		str = 10
		agi = 12
		hp = 100
	}
	
	enemy "虫族工蜂" {
		id = "enemy_wasp"
		str = 8
		hp = 50
	}
}

storyline {
	chapter "冷舱梦醒" {
		id = "chap_001"
		
		objective "逃离" {
			step 1 {
				description = "醒来"
				event {
					type = "spawn"
					actor = "char_player"
					location = "loc_base"
				}
			}
		}
	}
}`

	parser := NewParser(dslContent)
	dsl, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	// Validate metadata
	if dsl.Metadata.Title != "逐星" {
		t.Errorf("Expected title '逐星', got '%s'", dsl.Metadata.Title)
	}

	// Validate world
	if len(dsl.World.Locations) != 1 {
		t.Errorf("Expected 1 location, got %d", len(dsl.World.Locations))
	}

	// Validate characters
	if dsl.Characters.Player.Name != "陆沉" {
		t.Errorf("Expected player name '陆沉', got '%s'", dsl.Characters.Player.Name)
	}

	if len(dsl.Characters.Enemies) != 1 {
		t.Errorf("Expected 1 enemy, got %d", len(dsl.Characters.Enemies))
	}

	// Validate storyline
	if len(dsl.Storyline.Chapters) != 1 {
		t.Errorf("Expected 1 chapter, got %d", len(dsl.Storyline.Chapters))
	}
}