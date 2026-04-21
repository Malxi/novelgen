package dsl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEndToEndDSLConversion tests the complete DSL to RPG World conversion
func TestEndToEndDSLConversion(t *testing.T) {
	// Create a complete DSL using JSON format (more reliable for complex structures)
	dslContent := `{
		"metadata": {
			"title": "逐星",
			"genre": ["科幻", "废土求生", "虫族"],
			"power_system": "基因进化",
			"dsl_version": "0.1.0"
		},
		"world": {
			"locations": [
				{
					"id": "loc_base",
					"name": "休眠基地",
					"type": "indoor",
					"description": "废弃的休眠设施"
				},
				{
					"id": "loc_wasteland",
					"name": "废土地表",
					"type": "outdoor",
					"description": "荒芜的废土"
				}
			],
			"items": [
				{
					"id": "item_crystal",
					"name": "虫族晶核",
					"type": "material",
					"rarity": "common"
				}
			]
		},
		"characters": {
			"player": {
				"id": "char_player",
				"name": "陆沉",
				"class": "engineer",
				"stats": {
					"hp": 100,
					"str": 10,
					"agi": 12,
					"int": 15,
					"vit": 10,
					"mp": 50
				},
				"skills": ["skill_analyze"]
			},
			"enemies": [
				{
					"id": "enemy_wasp",
					"name": "虫族工蜂",
					"type": "insect",
					"template": {
						"stats_per_level": {
							"str": 8,
							"agi": 10
						}
					}
				},
				{
					"id": "enemy_warrior",
					"name": "虫族战士",
					"type": "insect",
					"template": {
						"stats_per_level": {
							"str": 12,
							"agi": 8
						}
					}
				}
			]
		},
		"storyline": {
			"chapters": [
				{
					"id": "chap_001",
					"title": "冷舱梦醒",
					"objectives": [
						{
							"name": "逃离休眠基地",
							"steps": [
								{
									"order": 1,
									"description": "从休眠仓中醒来",
									"event": {
										"type": "spawn",
										"spawn": {
											"actor": "char_player",
											"location": "loc_base"
										}
									}
								},
								{
									"order": 2,
									"description": "遭遇虫族工蜂",
									"event": {
										"type": "combat"
									}
								},
								{
									"order": 3,
									"description": "离开基地进入废土",
									"event": {
										"type": "move",
										"move": {
											"actor": "char_player",
											"to": "loc_wasteland"
										}
									}
								}
							]
						}
					]
				},
				{
					"id": "chap_002",
					"title": "废土求生",
					"objectives": [
						{
							"name": "寻找幸存者",
							"steps": [
								{
									"order": 1,
									"description": "探索废土",
									"event": {
										"type": "move",
										"move": {
											"actor": "char_player",
											"to": "loc_wasteland"
										}
									}
								},
								{
									"order": 2,
									"description": "击败虫族战士",
									"event": {
										"type": "combat"
									}
								}
							]
						}
					]
				}
			]
		}
	}`

	// Step 1: Parse DSL
	parser := NewParser(dslContent)
	dslAST, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	// Step 2: Validate DSL
	validator := NewValidator()
	if err := validator.Validate(dslAST); err != nil {
		t.Errorf("Validation failed: %v", err)
		for _, e := range validator.GetErrors() {
			t.Errorf("  Error: %s", e.Error())
		}
	}

	if validator.HasWarnings() {
		t.Logf("Warnings (%d):", len(validator.GetWarnings()))
		for _, w := range validator.GetWarnings() {
			t.Logf("  - %s: %s", w.Field, w.Message)
		}
	}

	// Step 3: Convert to RPG World
	converter := NewConverter()
	world, err := converter.Convert(dslAST)
	if err != nil {
		t.Fatalf("Failed to convert DSL to RPG world: %v", err)
	}

	// Step 4: Verify RPG World

	// Verify metadata
	if world.Context == nil {
		t.Error("World context is nil")
	}

	// Verify maps
	maps := world.Maps.GetAllMaps()
	if len(maps) != 2 {
		t.Errorf("Expected 2 maps, got %d", len(maps))
	}

	baseMap := world.Maps.GetMap("loc_base")
	if baseMap == nil {
		t.Error("Map 'loc_base' not found")
	} else {
		if baseMap.Name != "休眠基地" {
			t.Errorf("Expected map name '休眠基地', got '%s'", baseMap.Name)
		}
	}

	// Verify items
	items := world.Items.GetAllItems()
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}

	crystal := world.Items.GetItem("item_crystal")
	if crystal == nil {
		t.Error("Item 'item_crystal' not found")
	} else {
		if crystal.Name != "虫族晶核" {
			t.Errorf("Expected item name '虫族晶核', got '%s'", crystal.Name)
		}
	}

	// Verify character templates
	enemyTemplate := world.Characters.GetTemplate("enemy_wasp")
	if enemyTemplate == nil {
		t.Error("Enemy template 'enemy_wasp' not found")
	} else {
		if enemyTemplate.Name != "虫族工蜂" {
			t.Errorf("Expected enemy name '虫族工蜂', got '%s'", enemyTemplate.Name)
		}
	}

	// Verify player
	if world.Player == nil {
		t.Fatal("Player is nil")
	}
	if world.Player.Name != "陆沉" {
		t.Errorf("Expected player name '陆沉', got '%s'", world.Player.Name)
	}
	if world.Player.BaseStats.HP != 100 {
		t.Errorf("Expected player HP 100, got %d", world.Player.BaseStats.HP)
	}

	// Verify quests
	quests := world.Quests.GetAllQuests()
	if len(quests) != 2 {
		t.Errorf("Expected 2 quests, got %d", len(quests))
	}

	quest1 := world.Quests.GetQuest("chap_001")
	if quest1 == nil {
		t.Error("Quest 'chap_001' not found")
	} else {
		if quest1.Name != "冷舱梦醒" {
			t.Errorf("Expected quest name '冷舱梦醒', got '%s'", quest1.Name)
		}
		if len(quest1.Objectives) != 3 {
			t.Errorf("Expected 3 objectives, got %d", len(quest1.Objectives))
		}
	}

	t.Log("✅ End-to-end DSL conversion test passed!")
}

// TestDSLToSimulation tests converting DSL and running a simulation
func TestDSLToSimulation(t *testing.T) {
	// Simple DSL for simulation test
	dslContent := `metadata {
	title = "测试故事"
	dsl_version = "0.1.0"
}

world {
	location "起点" {
		id = "loc_start"
		type = "outdoor"
	}
}

characters {
	player "测试玩家" {
		id = "char_player"
		class = "warrior"
		str = 10
		hp = 100
	}
	
	enemy "测试敌人" {
		id = "enemy_test"
		str = 5
		hp = 30
	}
}

storyline {
	chapter "测试章节" {
		id = "chap_test"
		
		objective "测试目标" {
			step 1 {
				description = "生成玩家"
				event {
					type = "spawn"
					actor = "char_player"
					location = "loc_start"
				}
			}
			
			step 2 {
				description = "战斗"
				event {
					type = "combat"
					enemies = [
						{ id = "enemy_test", count = 1, level = 1 }
					]
				}
			}
		}
	}
}`

	// Parse and convert
	parser := NewParser(dslContent)
	dslAST, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	validator := NewValidator()
	if err := validator.Validate(dslAST); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	converter := NewConverter()
	world, err := converter.Convert(dslAST)
	if err != nil {
		t.Fatalf("Failed to convert: %v", err)
	}

	// Verify the world is ready for simulation
	if world.Player == nil {
		t.Fatal("Player not created")
	}

	quest := world.Quests.GetQuest("chap_test")
	if quest == nil {
		t.Fatal("Quest not created")
	}

	// Try to accept the quest
	questInstance := world.Quests.AcceptQuest(quest.ID, world.Player)
	if questInstance == nil {
		t.Error("Failed to accept quest")
	}

	t.Log("✅ DSL to simulation test passed!")
}

// TestFileBasedIntegration tests importing DSL from a file
func TestFileBasedIntegration(t *testing.T) {
	// Create temporary DSL file
	tmpDir := t.TempDir()
	dslFile := filepath.Join(tmpDir, "test.dsl")

	dslContent := `metadata {
	title = "文件测试"
	dsl_version = "0.1.0"
}

world {
	location "测试地点" {
		id = "loc_test"
		type = "indoor"
	}
}

characters {
	player "测试玩家" {
		id = "char_player"
		class = "tester"
		str = 10
		hp = 100
	}
}

storyline {
	chapter "测试章节" {
		id = "chap_test"
		
		objective "测试目标" {
			step 1 {
				description = "测试步骤"
				event {
					type = "spawn"
					actor = "char_player"
					location = "loc_test"
				}
			}
		}
	}
}`

	if err := os.WriteFile(dslFile, []byte(dslContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Read and parse from file
	content, err := os.ReadFile(dslFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	parser := NewParser(string(content))
	dslAST, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Validate and convert
	validator := NewValidator()
	if err := validator.Validate(dslAST); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	converter := NewConverter()
	world, err := converter.Convert(dslAST)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Verify
	if world.Player == nil {
		t.Error("Player not created")
	}
	if world.Maps.GetMap("loc_test") == nil {
		t.Error("Map not created")
	}

	t.Log("✅ File-based integration test passed!")
}

// TestJSONFormatIntegration tests JSON format DSL
func TestJSONFormatIntegration(t *testing.T) {
	jsonContent := `{
		"metadata": {
			"title": "JSON测试",
			"dsl_version": "0.1.0"
		},
		"world": {
			"locations": [
				{
					"id": "loc_json",
					"name": "JSON地点",
					"type": "outdoor"
				}
			]
		},
		"characters": {
			"player": {
				"id": "char_player",
				"name": "JSON玩家",
				"class": "coder",
				"stats": {
					"hp": 100,
					"str": 10
				}
			}
		},
		"storyline": {
			"chapters": [
				{
					"id": "chap_json",
					"title": "JSON章节",
					"objectives": [
						{
							"name": "JSON目标",
							"steps": [
								{
									"order": 1,
									"description": "JSON步骤",
									"event": {
										"type": "spawn",
										"spawn": {
											"actor": "char_player",
											"location": "loc_json"
										}
									}
								}
							]
						}
					]
				}
			]
		}
	}`

	parser := NewParser(jsonContent)
	dslAST, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if dslAST.Metadata.Title != "JSON测试" {
		t.Errorf("Expected title 'JSON测试', got '%s'", dslAST.Metadata.Title)
	}

	validator := NewValidator()
	if err := validator.Validate(dslAST); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	converter := NewConverter()
	world, err := converter.Convert(dslAST)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	if world.Player.Name != "JSON玩家" {
		t.Errorf("Expected player name 'JSON玩家', got '%s'", world.Player.Name)
	}

	t.Log("✅ JSON format integration test passed!")
}