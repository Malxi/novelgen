package dsl

import (
	"fmt"
	"testing"
)

// TestDSLMerger_FireGalaxy demonstrates merging outline and craft DSL
func TestDSLMerger_FireGalaxy(t *testing.T) {
	fmt.Println("=== Fire-Galaxy DSL Merger Demo ===")
	fmt.Println()

	// Step 1: Create outline DSL (basic framework from outline.json)
	outlineDSL := createOutlineDSL()
	fmt.Printf("1. Outline DSL created:\n")
	fmt.Printf("   - Title: %s\n", outlineDSL.Metadata.Title)
	fmt.Printf("   - Placeholders: %v\n\n", outlineDSL.CountPlaceholders())

	// Step 2: Create craft DSL (detailed info from craft/*.json)
	craftDSL := createCraftDSL()
	fmt.Printf("2. Craft DSL created:\n")
	fmt.Printf("   - Characters: %d\n", len(craftDSL.Characters.Enemies)+len(craftDSL.Characters.NPCs)+1)
	fmt.Printf("   - Locations: %d\n\n", len(craftDSL.World.Locations))

	// Step 3: Create merger and add fragments
	logger := NewConsoleLogger(WithMinLevel(LogLevelInfo))
	merger := NewDSLMerger(logger)

	merger.AddFragment(outlineDSL, PhaseOutline, "01_outline.rpg")
	merger.AddFragment(craftDSL, PhaseCraft, "02_craft.rpg")

	// Step 4: Merge!
	result, err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	fmt.Printf("3. Merge Result:\n")
	fmt.Printf("   - Phases merged: %v\n", result.PhasesMerged)
	fmt.Printf("   - Placeholders remaining: %d\n", len(result.Placeholders))
	fmt.Printf("   - Conflicts: %d\n", len(result.Conflicts))
	fmt.Printf("   - Warnings: %d\n\n", len(result.Warnings))

	// Step 5: Verify merged DSL
	mergedDSL := result.DSL

	fmt.Printf("4. Merged DSL Content:\n")
	fmt.Printf("   - Player: %s (HP %d)\n", 
		mergedDSL.Characters.Player.Name,
		mergedDSL.Characters.Player.Stats.HP)
	fmt.Printf("   - Player Background: %.50s...\n", 
		mergedDSL.Characters.Player.Background)

	// Check if placeholder was filled
	if !mergedDSL.Characters.Player.IsPlaceholder {
		fmt.Printf("   ✓ Player placeholder FILLED\n")
	}

	// Check enemies
	fmt.Printf("\n   - Enemies (%d):\n", len(mergedDSL.Characters.Enemies))
	for _, enemy := range mergedDSL.Characters.Enemies {
		placeholderStatus := "✓"
		if enemy.IsPlaceholder {
			placeholderStatus = "✗ (placeholder)"
		}
		fmt.Printf("     %s %s - HP:%d %s\n", 
			placeholderStatus, enemy.Name, 
			enemy.Template.StatsPerLevel["hp"],
			enemy.Description)
	}

	// Check locations
	fmt.Printf("\n   - Locations (%d):\n", len(mergedDSL.World.Locations))
	for _, loc := range mergedDSL.World.Locations {
		placeholderStatus := "✓"
		if loc.IsPlaceholder {
			placeholderStatus = "✗ (placeholder)"
		}
		desc := loc.Description
		if len(desc) > 40 {
			desc = desc[:40] + "..."
		}
		fmt.Printf("     %s %s - %s\n", 
			placeholderStatus, loc.Name, desc)
	}

	// Step 6: Check for unfilled placeholders
	if merger.HasUnfilledPlaceholders(result) {
		fmt.Printf("\n5. ⚠️  Unfilled Placeholders:\n")
		fmt.Print(merger.GetUnfilledPlaceholdersSummary(result))
		fmt.Printf("\n6. AI Prompt for missing details:\n")
		fmt.Print(merger.GeneratePromptForPlaceholders(result))
	} else {
		fmt.Printf("\n5. ✓ All placeholders filled!\n")
	}
}

// createOutlineDSL creates the outline phase DSL
func createOutlineDSL() *DSL {
	return &DSL{
		Metadata: &Metadata{
			Title:        "火银河",
			Subtitle:     "从休眠者到星际战士",
			Genre:        []string{"科幻", "废土", "虫族", "基因进化"},
			PowerSystem:  "基因进化",
			Tone:         "史诗",
			DSLVersion:   "0.2.0",
		},
		Characters: &Characters{
			Player: &Player{
				ID:                "char_luxingmian",
				Name:              "陆星眠",
				IsPlaceholder:     true,
				PlaceholderSource: "outline",
			},
			Enemies: []Enemy{
				{
					ID:                "enemy_insect_worker",
					Name:              "虫族工蜂",
					Type:              "insect",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
				{
					ID:                "enemy_insect_scythe",
					Name:              "低阶镰虫",
					Type:              "insect",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
			},
			NPCs: []NPC{
				{
					ID:                "npc_chenye",
					Name:              "陈野",
					Role:              "拾荒队长",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
				{
					ID:                "npc_suxiao",
					Name:              "苏晓",
					Role:              "队医",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
			},
		},
		World: &World{
			Locations: []Location{
				{
					ID:                "loc_cryo_center",
					Name:              "西昌航天发射场地下休眠中心",
					Type:              "dungeon",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
				{
					ID:                "loc_rustwall",
					Name:              "锈墙幸存者据点",
					Type:              "city",
					IsPlaceholder:     true,
					PlaceholderSource: "outline",
				},
			},
		},
		Storyline: &Storyline{
			Chapters: []Chapter{
				{
					ID:       "ch1_awakening",
					Title:    "休眠仓的冷光",
					Objectives: []Objective{
						{
							ID:   "obj_awakening",
							Name: "苏醒并探索",
							Type: "sequence",
							Steps: []Step{
								{
									Order:       1,
									Description: "从休眠中苏醒",
								},
								{
									Order:       2,
									Description: "遭遇虫族",
								},
							},
						},
					},
				},
			},
		},
	}
}

// createCraftDSL creates the craft phase DSL with detailed info
func createCraftDSL() *DSL {
	return &DSL{
		Metadata: &Metadata{
			Phase: "craft",
		},
		Characters: &Characters{
			Player: &Player{
				ID:          "char_luxingmian",
				Name:        "陆星眠",
				IsPlaceholder: false,
				Description: "来自21世纪的航天工程师...",
				Age:         28,
				Gender:      "男",
				Race:        "人类",
				Background:  "华国西昌航天发射场的年轻工程师...",
				Personality: []string{"理性", "好奇", "坚韧"},
				Motivation:  "在这个危险的世界生存下去...",
				Class:       "adaptable_survivor",
				Stats: Stats{
					STR: 12, AGI: 15, INT: 16, VIT: 10,
					HP: 100, MP: 50,
				},
				Skills: []string{"工程学", "快速学习", "适应环境"},
			},
			Enemies: []Enemy{
				{
					ID:            "enemy_insect_worker",
					Name:          "虫族工蜂",
					IsPlaceholder: false,
					Description:   "虫族最基础的单位...",
					Type:          "insect",
					Level:         1,
					Appearance:    "半人高的昆虫，黄褐色外壳...",
					Abilities:     []string{"酸性唾液", "群体攻击"},
					Template: EnemyTemplate{
						StatsPerLevel: map[string]int{
							"hp": 40, "mp": 10,
							"str": 8, "agi": 12, "vit": 6,
						},
					},
					Drops: Drops{
						Random: []RandomDrop{
							{Item: "虫族甲壳", Chance: 30.0},
						},
					},
				},
				{
					ID:            "enemy_insect_scythe",
					Name:          "低阶镰虫",
					IsPlaceholder: false,
					Description:   "虫族基础战斗单位...",
					Type:          "insect",
					Level:         2,
					Appearance:    "约1.5米高的虫族...",
					Abilities:     []string{"镰刀斩击", "跳跃攻击"},
					Template: EnemyTemplate{
						StatsPerLevel: map[string]int{
							"hp": 60, "mp": 15,
							"str": 14, "agi": 10, "vit": 8,
						},
					},
				},
			},
			NPCs: []NPC{
				{
					ID:            "npc_chenye",
					Name:          "陈野",
					IsPlaceholder: false,
					Description:   "锈墙据点的拾荒队长...",
					Age:           35,
					Gender:        "男",
					Role:          "scavenger_captain",
					Appearance:    "身高一米八的壮汉...",
					Background:    "废土土著，从小在据点长大...",
					Personality:   []string{"豪爽", "务实", "保护欲强"},
					Affiliations:  []string{"锈墙据点", "拾荒者公会"},
				},
				{
					ID:            "npc_suxiao",
					Name:          "苏晓",
					IsPlaceholder: false,
					Description:   "锈墙据点的年轻队医...",
					Age:           24,
					Gender:        "女",
					Role:          "medic",
					Appearance:    "清秀的年轻女性...",
					Background:    "自学成才的医者...",
					Personality:   []string{"温柔", "细心", "坚韧"},
					Affiliations:  []string{"锈墙据点", "医疗站"},
				},
			},
		},
		World: &World{
			Locations: []Location{
				{
					ID:             "loc_cryo_center",
					Name:           "西昌航天发射场地下休眠中心",
					IsPlaceholder:  false,
					Type:           "dungeon",
					Description:    "位于西昌航天发射场地下300米的休眠设施...",
					Appearance:     "巨大的圆形大厅，中央排列着数十个休眠仓...",
					Atmosphere:     "阴暗潮湿，布满灰尘...",
					History:        "建于2247年，用于深空探索人员的长期休眠实验...",
					Secrets:        "中心深处可能还有其他幸存的休眠者...",
					SensoryDetails: map[string][]string{
						"visual": {"布满灰尘的休眠仓", "闪烁的红色警示灯"},
						"audio":  {"冷却系统的低鸣", "远处传来的嘶鸣声"},
						"smell":  {"霉味", "金属锈蚀的味道"},
					},
					Inhabitants: []string{"虫族工蜂"},
					Events:      []string{"休眠仓故障", "虫族入侵"},
				},
				{
					ID:            "loc_rustwall",
					Name:          "锈墙幸存者据点",
					IsPlaceholder: false,
					Type:          "city",
					Description:   "人类在废墟中建立的临时避难所...",
					Appearance:    "由废弃车辆、集装箱和铁板围成的圆形堡垒...",
					Atmosphere:    "拥挤但充满生机...",
					History:       "建于虫族入侵后第50年...",
				},
			},
		},
		Systems: &Systems{
			Counters: []CounterSystem{
				{
					Name:        "insect_kills",
					Track:       "虫族击杀数",
					Description: "累计击杀虫族数量",
					Milestones: []CounterMilestone{
						{
							Value: 5,
							Reward: Reward{
								Title:       "初级虫族猎人",
								Description: "你已经能够应对基础的虫族威胁",
								Exp:         100,
							},
						},
						{
							Value: 25,
							Reward: Reward{
								Title:       "虫族克星",
								Description: "虫族见到你都会颤抖",
								Exp:         500,
								Items:       []string{"chitin_armor"},
							},
						},
					},
				},
			},
		},
	}
}
