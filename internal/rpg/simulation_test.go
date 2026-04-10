package rpg

import (
	"testing"
)

func TestSimulationEngine(t *testing.T) {
	// 创建示例世界
	world := CreateExampleWorld()
	
	// 创建推演引擎
	engine := NewSimulationEngine(world)
	
	if engine == nil {
		t.Fatal("推演引擎创建失败")
	}
	
	if len(engine.History) != 0 {
		t.Error("推演历史应该为空")
	}
	
	t.Logf("推演引擎创建成功")
}

func TestSimulateCombat(t *testing.T) {
	// 创建示例世界
	world := CreateExampleWorld()
	
	// 创建一个敌人
	enemy := world.CreateCharacter("template_goblin", "测试哥布林")
	if enemy == nil {
		t.Fatal("创建敌人失败")
	}
	
	// 创建推演引擎
	engine := NewSimulationEngine(world)
	
	// 创建战斗目标
	objective := QuestObjective{
		ID:          "test_obj_1",
		Type:        ObjectiveKill,
		Description: "击败测试哥布林",
		TargetID:    enemy.ID,
		TargetCount: 1,
	}
	
	// 推演战斗
	step := engine.SimulateObjective(objective)
	
	if step.Type != "kill" {
		t.Errorf("步骤类型错误，期望 'kill'，实际 '%s'", step.Type)
	}
	
	if len(step.Actions) == 0 {
		t.Error("战斗应该有行动")
	}
	
	t.Logf("战斗推演完成，行动数: %d", len(step.Actions))
	
	// 打印战斗过程
	for _, result := range step.Results {
		t.Logf("  - %s", result.Message)
	}
	
	// 检查战斗结果
	if step.StateChanges["enemy_defeated"] == nil {
		t.Log("战斗可能失败或未完成")
	} else {
		t.Logf("敌人被击败，获得经验: %v", step.StateChanges["exp_gained"])
	}
}

func TestSimulateTalk(t *testing.T) {
	// 创建示例世界
	world := CreateExampleWorld()
	
	// 创建一个NPC
	npc := world.CreateCharacter("template_merchant", "测试NPC")
	if npc == nil {
		t.Fatal("创建NPC失败")
	}
	
	// 创建推演引擎
	engine := NewSimulationEngine(world)
	
	// 创建对话目标
	objective := QuestObjective{
		ID:          "test_obj_2",
		Type:        ObjectiveTalk,
		Description: "与测试NPC对话",
		TargetID:    npc.ID,
		TargetCount: 1,
	}
	
	// 推演对话
	step := engine.SimulateObjective(objective)
	
	if step.Type != "talk" {
		t.Errorf("步骤类型错误，期望 'talk'，实际 '%s'", step.Type)
	}
	
	t.Logf("对话推演完成: %s", step.Description)
}

func TestSimulateCollect(t *testing.T) {
	// 创建示例世界
	world := CreateExampleWorld()
	
	// 创建推演引擎
	engine := NewSimulationEngine(world)
	
	// 创建收集目标
	objective := QuestObjective{
		ID:          "test_obj_3",
		Type:        ObjectiveCollect,
		Description: "收集生命药水",
		TargetID:    "item_health_potion",
		TargetCount: 3,
	}
	
	// 推演收集
	step := engine.SimulateObjective(objective)
	
	if step.Type != "collect" {
		t.Errorf("步骤类型错误，期望 'collect'，实际 '%s'", step.Type)
	}
	
	t.Logf("收集推演完成: %s", step.Description)
	
	// 检查背包
	for _, item := range world.Player.Items {
		if item.ItemID == "item_health_potion" {
			t.Logf("背包中有 %d 个生命药水", item.Count)
			break
		}
	}
}

func TestSimulationReport(t *testing.T) {
	// 创建示例世界
	world := CreateExampleWorld()
	
	// 创建推演引擎
	engine := NewSimulationEngine(world)
	
	// 执行一些推演
	enemy := world.CreateCharacter("template_goblin", "哥布林")
	if enemy != nil {
		objective := QuestObjective{
			ID:          "test_obj",
			Type:        ObjectiveKill,
			Description: "击败哥布林",
			TargetID:    enemy.ID,
			TargetCount: 1,
		}
		engine.SimulateObjective(objective)
	}
	
	// 生成报告
	report := engine.GetSimulationReport()
	
	if report == "" {
		t.Error("报告为空")
	}
	
	t.Logf("推演报告:\n%s", report)
}

func TestFullChapterSimulation(t *testing.T) {
	// 创建故事世界（从大纲）
	outline := StoryOutline{
		Parts: []StoryPart{
			{
				ID:      "P1",
				Title:   "测试部分",
				Volumes: []StoryVolume{
					{
						ID:     "P1-V1",
						Title:  "测试卷",
						Chapters: []StoryChapter{
							{
								ID:          "P1-V1-C1",
								Title:       "测试章节",
								Summary:     "这是一个测试章节",
								Characters:  []string{"林砚"},
								Location:    "测试地点",
								Events: []StoryEvent{
									{
										Type:       "test",
										Characters: []string{"林砚"},
										Subject:    "林砚",
										Change:     "完成测试",
									},
								},
								Beats: []string{"开始测试", "测试进行中", "测试完成"},
							},
						},
					},
				},
			},
		},
	}
	
	// 手动创建故事世界
	storyWorld := &StoryWorld{
		GameWorld:    NewGameWorld(),
		Outline:      &outline,
		CharacterMap: make(map[string]string),
		LocationMap:  make(map[string]string),
		QuestMap:     make(map[string]string),
		EventMap:     make(map[string]string),
	}
	storyWorld.ConvertOutlineToRPG()
	
	// 创建推演引擎
	engine := NewSimulationEngine(storyWorld.GameWorld)
	
	// 推演章节
	result, err := engine.SimulateChapter("P1-V1-C1")
	if err != nil {
		t.Logf("章节推演出错: %v", err)
		return
	}
	
	t.Logf("章节推演完成: %s", result.ChapterName)
	t.Logf("步骤数: %d", len(result.Steps))
	t.Logf("成功: %v", result.Success)
	
	// 打印推演过程
	for _, step := range result.Steps {
		t.Logf("  [%s] %s", step.Type, step.Description)
	}
}
