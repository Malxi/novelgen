package main

import (
	"fmt"
	"log"
	"strings"

	"novelgen/internal/rpg"
)

func main() {
	fmt.Print("=== 剧情推演示例 ===\n")

	// 方法1: 使用示例世界进行推演
	fmt.Println("【方法1】使用示例世界")
	exampleWorldSimulation()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// 方法2: 从大纲创建世界并推演
	fmt.Println("【方法2】从大纲创建世界并推演")
	outlineSimulation()
}

func exampleWorldSimulation() {
	// 创建示例世界
	world := rpg.CreateExampleWorld()

	// 创建推演引擎
	engine := rpg.NewSimulationEngine(world)

	// 创建一个敌人
	enemy := world.CreateCharacter("template_goblin", "哥布林战士")
	if enemy == nil {
		log.Fatal("创建敌人失败")
	}

	fmt.Printf("创建敌人: %s (HP: %d)\n", enemy.Name, enemy.BaseStats.HP)

	// 创建战斗目标
	objective := rpg.QuestObjective{
		ID:          "test_combat",
		Type:        rpg.ObjectiveKill,
		Description: "击败哥布林战士",
		TargetID:    enemy.ID,
		TargetCount: 1,
	}

	// 推演战斗
	fmt.Println("\n开始战斗推演...")
	step := engine.SimulateObjective(objective)

	fmt.Printf("\n战斗结果: %s\n", step.Description)
	fmt.Printf("行动数: %d\n", len(step.Actions))

	// 打印战斗过程
	fmt.Println("\n战斗过程:")
	for i, result := range step.Results {
		fmt.Printf("  %d. %s\n", i+1, result.Message)
		if result.Damage > 0 {
			fmt.Printf("     伤害: %d\n", result.Damage)
		}
	}

	// 显示最终状态
	fmt.Printf("\n最终状态:\n")
	fmt.Printf("  玩家HP: %d/%d\n", world.Player.CurrentStats.HP, world.Player.BaseStats.HP)
	fmt.Printf("  敌人HP: %d/%d\n", enemy.CurrentStats.HP, enemy.BaseStats.HP)
	if enemy.State == rpg.CharacterStateDead {
		fmt.Printf("  敌人状态: 死亡\n")
	}
}

func outlineSimulation() {
	// 创建大纲
	outline := rpg.StoryOutline{
		Parts: []rpg.StoryPart{
			{
				ID:      "P1",
				Title:   "初入修仙界",
				Summary: "主角穿越到修仙世界，开始冒险",
				Volumes: []rpg.StoryVolume{
					{
						ID:      "P1-V1",
						Title:   "觉醒篇",
						Summary: "主角觉醒特殊能力",
						Chapters: []rpg.StoryChapter{
							{
								ID:         "P1-V1-C1",
								Title:      "穿越觉醒",
								Summary:    "主角穿越到修仙世界，觉醒系统",
								Characters: []string{"林凡", "老者"},
								Location:   "青云山脉",
								Events: []rpg.StoryEvent{
									{
										Type:       "awakening",
										Characters: []string{"林凡"},
										Subject:    "林凡",
										Change:     "觉醒修仙系统",
									},
								},
								Beats: []string{
									"林凡醒来发现自己身处陌生山林",
									"脑海中响起系统提示音",
									"获得新手礼包：基础功法、灵石10颗",
									"遇到神秘老者，得知这里是修仙界",
								},
								StateChange: "林凡穿越到修仙界并觉醒系统",
								Conflict:    "身处陌生世界，需要尽快了解环境并生存",
								Pacing:      "normal",
							},
							{
								ID:         "P1-V1-C2",
								Title:      "初次修炼",
								Summary:    "主角开始修炼基础功法",
								Characters: []string{"林凡"},
								Location:   "青云山脉-修炼洞窟",
								Events: []rpg.StoryEvent{
									{
										Type:       "cultivation",
										Characters: []string{"林凡"},
										Subject:    "林凡",
										Change:     "突破到练气一层",
									},
								},
								Beats: []string{
									"找到一处隐蔽洞窟作为修炼场所",
									"按照系统指引开始修炼基础功法",
									"吸收天地灵气，感受体内变化",
									"成功突破到练气一层，实力大增",
								},
								StateChange: "林凡突破到练气一层，正式踏入修仙之路",
								Conflict:    "修炼资源匮乏，需要寻找更多资源",
								Pacing:      "slow",
							},
						},
					},
				},
			},
		},
	}

	// 创建故事世界
	storyWorld := &rpg.StoryWorld{
		GameWorld:    rpg.NewGameWorld(),
		Outline:      &outline,
		CharacterMap: make(map[string]string),
		LocationMap:  make(map[string]string),
		QuestMap:     make(map[string]string),
		EventMap:     make(map[string]string),
	}
	storyWorld.ConvertOutlineToRPG()

	fmt.Printf("故事世界创建成功!\n")
	fmt.Printf("角色数: %d\n", len(storyWorld.CharacterMap))
	fmt.Printf("地点数: %d\n", len(storyWorld.LocationMap))
	fmt.Printf("任务数: %d\n", len(storyWorld.QuestMap))

	// 创建推演引擎
	engine := rpg.NewSimulationEngine(storyWorld.GameWorld)

	// 推演第一个章节
	fmt.Println("\n开始推演章节: P1-V1-C1 (穿越觉醒)")
	result, err := engine.SimulateChapter("P1-V1-C1")
	if err != nil {
		log.Printf("章节推演出错: %v", err)
		return
	}

	fmt.Printf("\n章节推演完成: %s\n", result.ChapterName)
	fmt.Printf("步骤数: %d\n", len(result.Steps))
	fmt.Printf("成功: %v\n", result.Success)

	// 打印推演过程
	fmt.Println("\n推演过程:")
	for _, step := range result.Steps {
		fmt.Printf("\n[%s] %s\n", step.Type, step.Description)
		for _, action := range step.Actions {
			fmt.Printf("  行动: %s (%s -> %s)\n", action.ActionType, action.Actor, action.Target)
		}
		for _, res := range step.Results {
			if res.Message != "" {
				fmt.Printf("  结果: %s\n", res.Message)
			}
		}
	}

	// 生成推演报告
	fmt.Println("\n" + strings.Repeat("-", 50))
	fmt.Println(engine.GetSimulationReport())
}
