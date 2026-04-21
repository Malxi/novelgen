package dsl

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/rpg"
)

// TestFullIntegration demonstrates the complete DSL-RPG workflow
func TestFullIntegration(t *testing.T) {
	fmt.Println("=== Full DSL-RPG Integration Test ===")
	fmt.Println()

	// Step 1: Simulate existing novelgen project data
	fmt.Println("1. Creating novelgen project data...")
	project := createSampleNovelgenProject()
	fmt.Printf("   - Book: %s\n", project.BookName)
	fmt.Printf("   - Characters: %d\n", len(project.Characters))
	fmt.Printf("   - Locations: %d\n", len(project.Locations))
	fmt.Printf("   - Outline chapters: %d\n\n", countOutlineChapters(project.Outline))

	// Step 2: Convert to Outline DSL
	fmt.Println("2. Converting to Outline DSL...")
	adapter := NewNovelgenAdapter(project, nil)
	outlineDSL, err := adapter.ToDSL(PhaseOutline)
	if err != nil {
		t.Fatalf("Failed to create outline DSL: %v", err)
	}

	placeholders := outlineDSL.CountPlaceholders()
	fmt.Printf("   ✓ Outline DSL created\n")
	fmt.Printf("   - Placeholders: %v\n\n", placeholders)

	// Step 3: Convert to Craft DSL
	fmt.Println("3. Converting to Craft DSL...")
	craftDSL, err := adapter.ToDSL(PhaseCraft)
	if err != nil {
		t.Fatalf("Failed to create craft DSL: %v", err)
	}

	fmt.Printf("   ✓ Craft DSL created\n")
	fmt.Printf("   - Characters: %d\n", len(craftDSL.Characters.NPCs)+1)
	fmt.Printf("   - Locations: %d\n\n", len(craftDSL.World.Locations))

	// Step 4: Merge DSLs
	fmt.Println("4. Merging Outline + Craft DSL...")
	merger := NewDSLMerger(nil)
	merger.AddFragment(outlineDSL, PhaseOutline, "01_outline.rpg")
	merger.AddFragment(craftDSL, PhaseCraft, "02_craft.rpg")

	mergeResult, err := merger.Merge()
	if err != nil {
		t.Fatalf("Failed to merge DSLs: %v", err)
	}

	mergedDSL := mergeResult.DSL
	fmt.Printf("   ✓ DSLs merged successfully\n")
	fmt.Printf("   - Placeholders remaining: %d\n", len(mergeResult.Placeholders))
	fmt.Printf("   - Conflicts: %d\n\n", len(mergeResult.Conflicts))

	// Step 5: Export DSL to files
	fmt.Println("5. Exporting DSL files...")
	tempDir := t.TempDir()

	outlinePath := filepath.Join(tempDir, "01_outline.rpg")
	if err := outlineDSL.WriteToFile(outlinePath); err != nil {
		t.Fatalf("Failed to write outline DSL: %v", err)
	}

	craftPath := filepath.Join(tempDir, "02_craft.rpg")
	if err := craftDSL.WriteToFile(craftPath); err != nil {
		t.Fatalf("Failed to write craft DSL: %v", err)
	}

	mergedPath := filepath.Join(tempDir, "final.rpg")
	if err := mergedDSL.WriteToFile(mergedPath); err != nil {
		t.Fatalf("Failed to write merged DSL: %v", err)
	}

	fmt.Printf("   ✓ DSL files exported\n")
	fmt.Printf("   - %s\n", outlinePath)
	fmt.Printf("   - %s\n", craftPath)
	fmt.Printf("   - %s\n\n", mergedPath)

	// Step 6: Show sample DSL output
	fmt.Println("6. Sample merged DSL output:")
	fmt.Println("   ---")
	dslContent := mergedDSL.String()
	lines := splitLines(dslContent)
	if len(lines) > 50 {
		lines = lines[:50]
	}
	for _, line := range lines {
		fmt.Printf("   %s\n", line)
	}
	if len(splitLines(dslContent)) > 50 {
		fmt.Println("   ... (truncated)")
	}
	fmt.Println("   ---")

	// Step 7: Verify completeness
	fmt.Println("\n7. Verification:")
	if mergedDSL.Characters.Player != nil {
		fmt.Printf("   ✓ Player: %s\n", mergedDSL.Characters.Player.Name)
		fmt.Printf("     - Stats: HP=%d, STR=%d, AGI=%d\n",
			mergedDSL.Characters.Player.Stats.HP,
			mergedDSL.Characters.Player.Stats.STR,
			mergedDSL.Characters.Player.Stats.AGI)
	}

	fmt.Printf("   ✓ NPCs: %d\n", len(mergedDSL.Characters.NPCs))
	for _, npc := range mergedDSL.Characters.NPCs {
		fmt.Printf("     - %s (%s)\n", npc.Name, npc.Role)
	}

	fmt.Printf("   ✓ Locations: %d\n", len(mergedDSL.World.Locations))
	for _, loc := range mergedDSL.World.Locations {
		fmt.Printf("     - %s (%s)\n", loc.Name, loc.Type)
	}

	fmt.Printf("   ✓ Chapters: %d\n", len(mergedDSL.Storyline.Chapters))
	for _, chapter := range mergedDSL.Storyline.Chapters {
		fmt.Printf("     - %s\n", chapter.Title)
	}

	// Step 8: Check file sizes
	fmt.Println("\n8. File sizes:")
	for _, path := range []string{outlinePath, craftPath, mergedPath} {
		info, err := os.Stat(path)
		if err == nil {
			fmt.Printf("   %s: %d bytes\n", filepath.Base(path), info.Size())
		}
	}

	fmt.Println("\n=== Integration Test Complete ===")
}

// createSampleNovelgenProject creates a sample novelgen project
func createSampleNovelgenProject() *rpg.NovelgenProject {
	return &rpg.NovelgenProject{
		BookName: "火银河",
		Characters: map[string]rpg.NovelgenCharacter{
			"陆星眠": {
				Name:         "陆星眠",
				Age:          "28",
				Gender:       "男",
				Race:         "人类",
				Background:   "21世纪航天工程师，因实验事故进入休眠仓穿越到3024年",
				Personality:  []string{"理性", "好奇", "坚韧"},
				Motivation:   "在这个危险的世界生存下去",
				Abilities:    []string{"快速学习", "适应环境"},
				Skills:       []string{"工程学", "基础格斗"},
				Affiliations: []string{"锈墙据点"},
				RoleInStory:  "主角",
				Voice:        "理性但偶尔迷茫",
			},
			"陈野": {
				Name:         "陈野",
				Age:          "35",
				Gender:       "男",
				Background:   "锈墙据点拾荒队长，性格豪爽",
				Personality:  []string{"豪爽", "务实", "保护欲强"},
				RoleInStory:  "配角",
				Skills:       []string{"拾荒", "领导", "战斗"},
			},
		},
		Locations: map[string]rpg.NovelgenLocation{
			"西昌航天发射场地下休眠中心": {
				Name:           "西昌航天发射场地下休眠中心",
				Description:    "位于地下300米的休眠设施",
				Atmosphere:     "阴暗潮湿，布满灰尘",
				History:        "建于2247年，用于深空探索",
				ConnectedLocs:  []string{"休眠中心入口通道"},
				SensoryDetails: map[string][]string{
					"visual": {"休眠仓", "警示灯"},
					"audio":  {"冷却系统低鸣"},
				},
			},
			"锈墙据点": {
				Name:          "锈墙幸存者据点",
				Description:   "人类在废墟中建立的避难所",
				Atmosphere:    "拥挤但充满生机",
				ConnectedLocs: []string{},
			},
		},
		Items: map[string]rpg.NovelgenItem{
			"联邦民用身份识别手环": {
				Name:         "联邦民用身份识别手环",
				Type:         "剧情道具",
				Description:  "显示当前时间为星元327年的智能手环",
				Powers:       []string{"时间显示", "身份认证"},
				Significance: "核心道具",
			},
		},
		Outline: rpg.StoryOutline{
			Parts: []rpg.StoryPart{
				{
					ID:    "P1",
					Title: "第一卷",
					Volumes: []rpg.StoryVolume{
						{
							ID:       "V1",
							Title:    "第一辑",
							Chapters: []rpg.StoryChapter{
								{
									ID:       "P1-V1-C1",
									Title:    "第一章：苏醒",
									Events: []rpg.StoryEvent{
										{
											Type:    "status",
											Actor:   "陆星眠",
											Context: "休眠仓",
											Result:  "从休眠中苏醒",
										},
										{
											Type:    "combat",
											Actor:   "陆星眠",
											Target:  "虫族工蜂",
											Context: "休眠中心",
											Result:  "击败虫族",
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

func countOutlineChapters(outline rpg.StoryOutline) int {
	count := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			count += len(volume.Chapters)
		}
	}
	return count
}

func splitLines(s string) []string {
	var lines []string
	var current string
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
