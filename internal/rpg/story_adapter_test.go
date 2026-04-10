package rpg

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
)

func TestStoryWorldConversion(t *testing.T) {
	// 创建测试用的大纲文件
	outline := StoryOutline{
		Parts: []StoryPart{
			{
				ID:      "P1",
				Title:   "第一部分：矿场囚笼与弈局开端",
				Summary: "故事开篇，讲述林砚穿越为灵石矿场的底层矿奴",
				Volumes: []StoryVolume{
					{
						ID:      "P1-V1",
						Title:   "卷一：死而复生的矿奴",
						Summary: "林砚在矿场中触发复活能力",
						Chapters: []StoryChapter{
							{
								ID:         "P1-V1-C1",
								Title:      "寒矿醒转，首死触发复生",
								Summary:    "林砚穿越为矿奴，首次死亡后触发复活",
								Characters: []string{"林砚", "矿监周虎"},
								Location:   "黑风灵石矿丙字三号矿道",
								Events: []StoryEvent{
									{
										Type:       "premise",
										Characters: []string{"林砚"},
										Subject:    "林砚",
										Change:     "穿越为黑风灵石矿场练气二层矿奴",
									},
									{
										Type:       "premise",
										Characters: []string{"林砚"},
										Subject:    "林砚",
										Change:     "首次触发复活能力",
									},
								},
								Beats: []string{
									"林砚在刺骨的疼痛中醒转，发现自己身处潮湿阴暗的矿洞",
									"被监工周虎一鞭子抽在背上，逼着他立刻下矿挖灵石",
									"头顶的矿岩突然松动坍塌，落石瞬间砸中他的头颅",
									"林砚在矿道的另一个角落醒转，震惊地发现自己死而复生",
								},
								OpeningBeat: "林砚在刺骨的疼痛中醒转，发现自己身处潮湿阴暗的矿洞",
								ClosingBeat: "林砚在矿道的另一个角落醒转，震惊地发现自己死而复生",
								StateChange: "林砚确认自身穿越到修仙界成为矿奴，且拥有死而复生的特殊能力",
								Conflict:    "刚穿越就遭遇致命矿难，完全未知的生存环境带来强烈的生存危机",
								Pacing:      "fast",
							},
							{
								ID:         "P1-V1-C2",
								Title:      "二度殒命，始知复生有耗",
								Summary:    "林砚再次死亡，发现复活会损耗修为",
								Characters: []string{"林砚", "矿监周虎"},
								Location:   "黑风灵石矿丙字三号矿道",
								Events: []StoryEvent{
									{
										Type:       "status",
										Characters: []string{"林砚"},
										Subject:    "林砚",
										Change:     "修为从练气二层跌落至练气一层",
									},
								},
								Beats: []string{
									"刚复活的林砚意识混沌，误以为刚才的矿难只是濒死的幻觉",
									"被监工周虎踹了一脚，再次被赶回丙字三号矿道挖矿",
									"同一位置再次发生小型坍塌，林砚被落石砸中右腿失血过多死亡",
									"林砚在矿道出口处复活，检查自身状态时发现修为跌至练气一层",
								},
								OpeningBeat: "刚复活的林砚意识混沌，误以为刚才的矿难只是濒死的幻觉",
								ClosingBeat: "林砚在矿道出口处复活，检查自身状态时发现修为跌至练气一层",
								StateChange: "林砚确认复活能力真实存在，且复活会损耗自身修为",
								Conflict:    "反复死亡的痛苦与未知的能力代价让林砚陷入恐慌",
								Pacing:      "normal",
							},
						},
					},
				},
			},
		},
	}

	// 保存测试大纲到临时文件
	tmpFile, err := ioutil.TempFile("", "test_outline_*.json")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	outlineJSON, _ := json.MarshalIndent(outline, "", "  ")
	if _, err := tmpFile.Write(outlineJSON); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	tmpFile.Close()

	// 测试转换
	storyWorld, err := NewStoryWorld(tmpFile.Name())
	if err != nil {
		t.Fatalf("创建故事世界失败: %v", err)
	}

	// 验证转换结果
	if storyWorld == nil {
		t.Fatal("故事世界为空")
	}

	// 验证角色
	if len(storyWorld.CharacterMap) == 0 {
		t.Error("没有转换角色")
	}

	// 验证林砚是否被创建并设置为玩家
	if storyWorld.GameWorld.Player == nil {
		t.Error("玩家（林砚）未设置")
	} else if storyWorld.GameWorld.Player.Name != "林砚" {
		t.Errorf("玩家名称错误，期望'林砚'，实际'%s'", storyWorld.GameWorld.Player.Name)
	}

	t.Logf("玩家: %s (等级 %d)", storyWorld.GameWorld.Player.Name, storyWorld.GameWorld.Player.Level)
	t.Logf("玩家属性: HP=%d/%d, MP=%d/%d", 
		storyWorld.GameWorld.Player.CurrentStats.HP, storyWorld.GameWorld.Player.BaseStats.HP,
		storyWorld.GameWorld.Player.CurrentStats.MP, storyWorld.GameWorld.Player.BaseStats.MP)

	// 验证地点
	if len(storyWorld.LocationMap) == 0 {
		t.Error("没有转换地点")
	}

	// 验证地图是否正确创建
	if len(storyWorld.GameWorld.Maps.GetAllMaps()) == 0 {
		t.Error("没有创建地图")
	}

	for location, mapID := range storyWorld.LocationMap {
		gameMap := storyWorld.GameWorld.Maps.GetMap(mapID)
		if gameMap == nil {
			t.Errorf("地图未找到: %s -> %s", location, mapID)
		} else {
			t.Logf("地点: %s -> 地图: %s (类型: %s)", location, gameMap.Name, gameMap.Type)
		}
	}

	// 验证任务
	if len(storyWorld.QuestMap) == 0 {
		t.Error("没有转换任务")
	}

	for chapterID, questID := range storyWorld.QuestMap {
		quest := storyWorld.GameWorld.Quests.GetQuest(questID)
		if quest == nil {
			t.Errorf("任务未找到: %s -> %s", chapterID, questID)
		} else {
			t.Logf("章节: %s -> 任务: %s (目标数: %d)", chapterID, quest.Name, len(quest.Objectives))
		}
	}

	// 验证事件
	if len(storyWorld.EventMap) == 0 {
		t.Error("没有转换事件")
	}

	for chapterID, eventID := range storyWorld.EventMap {
		event := storyWorld.GameWorld.Events.GetEvent(eventID)
		if event == nil {
			t.Errorf("事件未找到: %s -> %s", chapterID, eventID)
		} else {
			t.Logf("章节: %s -> 事件: %s (页数: %d)", chapterID, event.Name, len(event.Pages))
		}
	}

	// 验证摘要
	summary := storyWorld.GetStorySummary()
	t.Logf("故事摘要: %+v", summary)

	// 测试导出
	exportJSON := storyWorld.ExportToJSON()
	if exportJSON == "" {
		t.Error("导出JSON失败")
	}
	t.Logf("导出JSON长度: %d 字符", len(exportJSON))
}

func TestStoryWorldWithRealOutline(t *testing.T) {
	// 测试使用真实的大纲文件
	outlinePath := "../../../books/mine/story/compose/outline.json"

	// 检查文件是否存在
	if _, err := os.Stat(outlinePath); os.IsNotExist(err) {
		t.Skip("真实大纲文件不存在，跳过此测试")
	}

	storyWorld, err := NewStoryWorld(outlinePath)
	if err != nil {
		t.Fatalf("从真实大纲创建故事世界失败: %v", err)
	}

	// 验证转换结果
	summary := storyWorld.GetStorySummary()
	t.Logf("故事摘要: %+v", summary)

	// 验证玩家
	if storyWorld.GameWorld.Player != nil {
		t.Logf("主角: %s (等级 %d, 战力 %d)",
			storyWorld.GameWorld.Player.Name,
			storyWorld.GameWorld.Player.Level,
			storyWorld.GameWorld.GetPlayerBattlePower().Total)
	}

	// 验证角色
	characters := storyWorld.GameWorld.Characters.GetAllCharacters()
	t.Logf("角色数量: %d", len(characters))
	for _, char := range characters {
		t.Logf("  - %s (类型: %s, 等级: %d)", char.Name, char.Type, char.Level)
	}

	// 验证地图
	maps := storyWorld.GameWorld.Maps.GetAllMaps()
	t.Logf("地图数量: %d", len(maps))
	for _, m := range maps {
		t.Logf("  - %s (类型: %s)", m.Name, m.Type)
	}

	// 验证任务
	quests := storyWorld.GameWorld.Quests.GetAllQuests()
	t.Logf("任务数量: %d", len(quests))
	for _, quest := range quests {
		t.Logf("  - %s (类型: %s, 目标: %d)", quest.Name, quest.Type, len(quest.Objectives))
	}
}
