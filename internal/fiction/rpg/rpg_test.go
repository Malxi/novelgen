package rpg

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCreateWorld(t *testing.T) {
	world := CreateExampleWorld()

	if world == nil {
		t.Fatal("Failed to create world")
	}

	if world.Player == nil {
		t.Fatal("Player not created")
	}

	t.Logf("World created successfully")
	t.Logf("Player: %s (Level %d)", world.Player.Name, world.Player.Level)
}

func TestCharacterCreation(t *testing.T) {
	world := CreateExampleWorld()

	// 测试创建角色
	char := world.CreateCharacter("template_warrior", "测试角色")
	if char == nil {
		t.Fatal("Failed to create character")
	}

	if char.Name != "测试角色" {
		t.Errorf("Expected name '测试角色', got '%s'", char.Name)
	}

	if char.Level != 1 {
		t.Errorf("Expected level 1, got %d", char.Level)
	}

	t.Logf("Character created: %s, HP: %d/%d", char.Name, char.CurrentStats.HP, char.BaseStats.HP)
}

func TestLevelUp(t *testing.T) {
	world := CreateExampleWorld()
	player := world.Player

	initialLevel := player.Level
	initialHP := player.BaseStats.HP

	// 给予足够经验升级
	leveledUp, _ := player.GainExp(150)

	if !leveledUp {
		t.Error("Expected level up, but didn't happen")
	}

	if player.Level <= initialLevel {
		t.Errorf("Expected level > %d, got %d", initialLevel, player.Level)
	}

	if player.BaseStats.HP <= initialHP {
		t.Errorf("Expected HP > %d after level up, got %d", initialHP, player.BaseStats.HP)
	}

	t.Logf("Level up! %d -> %d, HP: %d -> %d", initialLevel, player.Level, initialHP, player.BaseStats.HP)
}

func TestBattlePower(t *testing.T) {
	world := CreateExampleWorld()
	player := world.Player

	bp := player.CalculateBattlePower()

	if bp.Total <= 0 {
		t.Error("Battle power should be positive")
	}

	t.Logf("Battle Power: %d (Level: %d, Stats: %d, Equip: %d, Skills: %d)",
		bp.Total, bp.LevelPower, bp.StatsPower, bp.EquipPower, bp.SkillPower)
}

func TestSkillUsage(t *testing.T) {
	world := CreateExampleWorld()

	// 创建一个敌人
	enemy := world.CreateCharacter("template_goblin", "测试哥布林")
	if enemy == nil {
		t.Fatal("Failed to create enemy")
	}

	initialHP := enemy.CurrentStats.HP

	// 使用技能
	result := world.UseSkill("skill_slash", []string{enemy.ID})
	if result == nil {
		t.Log("Skill usage returned nil (might be due to cooldown or other restrictions)")
		return
	}

	if result.IsMiss {
		t.Log("Attack missed")
	} else {
		damage := result.Damage[enemy.ID]
		t.Logf("Dealt %d damage", damage)

		if enemy.CurrentStats.HP >= initialHP {
			t.Error("Enemy HP should have decreased")
		}
	}
}

func TestItemUsage(t *testing.T) {
	world := CreateExampleWorld()
	player := world.Player

	// 记录初始HP
	player.CurrentStats.HP = 50 // 降低HP以便测试治疗
	initialHP := player.CurrentStats.HP
	t.Logf("Initial HP: %d/%d", initialHP, player.BaseStats.HP)

	// 检查当前背包
	t.Logf("Player items: %v", player.Items)

	// 确保有药水
	player.AddItem("item_health_potion", 1)
	t.Logf("After adding potion: %v", player.Items)

	// 检查物品是否存在
	item := world.Items.GetItem("item_health_potion")
	if item == nil {
		t.Fatal("Health potion not found in item manager")
	}
	t.Logf("Item found: %s, IsUsable: %v, Effects: %v", item.Name, item.IsUsable, item.Effects)

	// 直接使用 ItemManager.UseItem
	effects := world.Items.UseItem("item_health_potion", player)

	if effects == nil {
		t.Error("Item usage failed - effects is nil")
		return
	}

	if player.CurrentStats.HP <= initialHP {
		t.Errorf("HP should have increased after using potion: %d -> %d", initialHP, player.CurrentStats.HP)
	}

	t.Logf("HP: %d -> %d", initialHP, player.CurrentStats.HP)
}

func TestEquipment(t *testing.T) {
	world := CreateExampleWorld()
	player := world.Player

	// 先卸下武器（因为CreateExampleWorld已经装备了武器）
	world.UnequipItem("weapon")
	player.CalculateCurrentStats(world.Equipments)

	// 记录卸下后的攻击力
	initialAttack := player.CurrentStats.Attack
	t.Logf("Attack after unequip: %d", initialAttack)

	// 装备武器
	success := world.EquipItem("equip_iron_sword")
	if !success {
		t.Error("Failed to equip weapon")
		return
	}

	// 重新计算属性
	player.CalculateCurrentStats(world.Equipments)

	if player.CurrentStats.Attack <= initialAttack {
		t.Errorf("Attack should have increased after equipping weapon: %d -> %d", initialAttack, player.CurrentStats.Attack)
	}

	t.Logf("Attack: %d -> %d", initialAttack, player.CurrentStats.Attack)
}

func TestQuestSystem(t *testing.T) {
	world := CreateExampleWorld()

	// 尝试接取任务
	success := world.AcceptQuest("quest_main_1")
	if !success {
		t.Error("Failed to accept quest")
		return
	}

	// 检查任务是否在活跃列表
	found := false
	for _, id := range world.QuestLog.ActiveQuests {
		if id == "quest_main_1" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Quest should be in active quests")
	}

	// 更新任务进度
	questMgr := world.Quests
	questMgr.AddProgress(ObjectiveKill, "template_goblin", 3)

	instance := questMgr.GetQuestLog().QuestInstances["quest_main_1"]
	if instance != nil {
		for _, obj := range instance.Objectives {
			if obj.Type == ObjectiveKill {
				t.Logf("Quest progress: %d/%d", obj.CurrentCount, obj.TargetCount)
			}
		}
	}
}

func TestEventSystem(t *testing.T) {
	world := CreateExampleWorld()

	// 触发事件
	event := world.Events.GetEvent("event_merchant_talk")
	if event == nil {
		t.Fatal("Event not found")
	}

	result := world.Events.TriggerEvent("event_merchant_talk", 0)
	if result == nil {
		t.Error("Event execution failed")
		return
	}

	t.Logf("Event executed with %d commands", len(result.Commands))
	for _, cmd := range result.Commands {
		t.Logf("Command: %s", cmd.Command)
		if cmd.Message != "" {
			t.Logf("  Message: %s", cmd.Message)
		}
	}
}

func TestMapSystem(t *testing.T) {
	world := CreateExampleWorld()

	// 获取地图
	village := world.Maps.GetMap("map_starting_village")
	if village == nil {
		t.Fatal("Map not found")
	}

	if village.Name != "新手村" {
		t.Errorf("Expected map name '新手村', got '%s'", village.Name)
	}

	// 测试传送
	success := world.MovePlayerTo("map_starting_village", 10, 10)
	if !success {
		t.Error("Failed to move player")
	}

	if world.Player.Position.X != 10 || world.Player.Position.Y != 10 {
		t.Error("Player position not updated correctly")
	}

	t.Logf("Player moved to: (%f, %f) in %s",
		world.Player.Position.X, world.Player.Position.Y, world.Player.Position.MapID)
}

func TestSerialization(t *testing.T) {
	world := CreateExampleWorld()

	// 序列化玩家
	playerJSON := world.Player.ToJSON()
	if playerJSON == "" {
		t.Error("Failed to serialize player")
		return
	}

	t.Logf("Player JSON length: %d", len(playerJSON))

	// 序列化世界状态
	worldJSON := world.SaveToJSON()
	if worldJSON == "" {
		t.Error("Failed to serialize world")
		return
	}

	t.Logf("World JSON length: %d", len(worldJSON))

	// 验证JSON有效性
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(worldJSON), &data); err != nil {
		t.Errorf("Invalid JSON: %v", err)
	}
}

func TestBattleScene(t *testing.T) {
	world := CreateExampleWorld()
	battle := CreateExampleBattle(world)

	if battle == nil {
		t.Fatal("Failed to create battle scene")
	}

	if len(battle.Enemies) == 0 {
		t.Fatal("No enemies in battle")
	}

	// 执行战斗回合
	player := world.Player
	enemy := battle.Enemies[0]

	result := battle.ExecuteTurn(world.Skills, player, "skill_slash", []*Character{enemy})
	if result != nil {
		t.Logf("Battle turn %d: %s used %s", battle.Turn, player.Name, "skill_slash")
		if !result.IsMiss {
			t.Logf("Dealt %d damage to %s", result.Damage[enemy.ID], enemy.Name)
		}
	}

	battleResult := battle.GetBattleResult()
	t.Logf("Battle status: Active=%v, Winner=%s", battleResult["is_active"], battleResult["winner"])
}

func TestWorldSummary(t *testing.T) {
	world := CreateExampleWorld()

	summary := world.GetWorldSummary()

	expectedKeys := []string{"player_name", "player_level", "current_map", "active_quests"}
	for _, key := range expectedKeys {
		if _, ok := summary[key]; !ok {
			t.Errorf("Missing key in summary: %s", key)
		}
	}

	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	t.Logf("World Summary:\n%s", string(summaryJSON))
}

func ExampleGameWorld() {
	// 创建示例世界
	world := CreateExampleWorld()

	// 显示玩家信息
	fmt.Printf("玩家: %s (等级 %d)\n", world.Player.Name, world.Player.Level)
	fmt.Printf("职业: %s\n", world.Player.ClassID)
	fmt.Printf("HP: %d/%d\n", world.Player.CurrentStats.HP, world.Player.BaseStats.HP)
	fmt.Printf("MP: %d/%d\n", world.Player.CurrentStats.MP, world.Player.BaseStats.MP)

	// 计算战力
	bp := world.GetPlayerBattlePower()
	fmt.Printf("战力: %d\n", bp.Total)

	// 显示任务状态
	fmt.Printf("进行中的任务: %d\n", len(world.QuestLog.ActiveQuests))
	fmt.Printf("已完成的任务: %d\n", len(world.QuestLog.CompletedQuests))

	// 显示当前位置
	fmt.Printf("当前位置: %s (%f, %f)\n",
		world.Player.Position.MapID,
		world.Player.Position.X,
		world.Player.Position.Y)
}
