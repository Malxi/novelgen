package rpg

// CreateExampleWorld 创建一个示例游戏世界
func CreateExampleWorld() *GameWorld {
	world := NewGameWorld()

	// ========== 创建职业 ==========
	warrior := &Class{
		ID:          "class_warrior",
		Name:        "战士",
		Description: "近战物理输出，拥有强大的攻击力和防御力",
		Type:        ClassTypeWarrior,
		BaseStats: BaseStats{
			HP:         120,
			MP:         30,
			Attack:     15,
			Defense:    12,
			Magic:      3,
			Resistance: 5,
			Speed:      8,
			Luck:       5,
		},
		GrowthStats: GrowthStats{
			HP:         15,
			MP:         2,
			Attack:     3,
			Defense:    2.5,
			Magic:      0.5,
			Resistance: 1,
			Speed:      1,
			Luck:       0.5,
		},
		StatModifier: ClassStatModifier{
			HP: 1.2, Attack: 1.15, Defense: 1.1,
		},
		DefaultSkills: []string{"skill_slash", "skill_defend"},
		WeaponTypes:   []string{"sword", "axe", "spear", "mace"},
		ArmorTypes:    []string{"plate", "mail", "leather"},
		Traits: []ClassTrait{
			{ID: "trait_rage", Name: "狂怒", Description: "HP低于30%时攻击力提升20%", Level: 10, Effect: Effect{Type: EffectBuff, Value: "rage"}},
		},
	}

	mage := &Class{
		ID:          "class_mage",
		Name:        "法师",
		Description: "远程魔法输出，拥有强大的魔法攻击力",
		Type:        ClassTypeMage,
		BaseStats: BaseStats{
			HP:         70,
			MP:         100,
			Attack:     5,
			Defense:    4,
			Magic:      18,
			Resistance: 12,
			Speed:      7,
			Luck:       6,
		},
		GrowthStats: GrowthStats{
			HP:         5,
			MP:         10,
			Attack:     0.5,
			Defense:    0.5,
			Magic:      4,
			Resistance: 2.5,
			Speed:      0.8,
			Luck:       0.5,
		},
		StatModifier: ClassStatModifier{
			MP: 1.3, Magic: 1.2, Resistance: 1.15,
		},
		DefaultSkills: []string{"skill_fireball", "skill_heal"},
		WeaponTypes:   []string{"staff", "wand"},
		ArmorTypes:    []string{"robe", "cloth"},
	}

	world.Classes.AddClass(warrior)
	world.Classes.AddClass(mage)

	// ========== 创建技能 ==========
	fireball := &Skill{
		ID:            "skill_fireball",
		Name:          "火球术",
		Description:   "发射火球攻击敌人",
		Type:          SkillTypeActive,
		Element:       ElementFire,
		LevelRequired: 1,
		Cost:          SkillCost{MP: 10},
		Cooldown:      0,
		Target:        SkillTargetSingle,
		Damage: &SkillDamage{
			Type:          DamageTypeMagic,
			Power:         30,
			ScalingStat:   "magic",
			ScalingFactor: 1.2,
			Element:       ElementFire,
		},
		Effects: []SkillEffect{
			{Type: EffectDebuff, Chance: 0.2, Duration: 3, Value: "burn"},
		},
	}

	slash := &Skill{
		ID:            "skill_slash",
		Name:          "斩击",
		Description:   "用武器斩击敌人",
		Type:          SkillTypeActive,
		LevelRequired: 1,
		Cost:          SkillCost{MP: 5},
		Target:        SkillTargetSingle,
		Damage: &SkillDamage{
			Type:          DamageTypePhysical,
			Power:         25,
			ScalingStat:   "attack",
			ScalingFactor: 1.0,
		},
	}

	heal := &Skill{
		ID:            "skill_heal",
		Name:          "治疗术",
		Description:   "恢复目标生命值",
		Type:          SkillTypeActive,
		Element:       ElementLight,
		LevelRequired: 3,
		Cost:          SkillCost{MP: 15},
		Target:        SkillTargetAlly,
		Damage: &SkillDamage{
			Type:          DamageTypeHeal,
			Power:         40,
			ScalingStat:   "magic",
			ScalingFactor: 0.8,
		},
	}

	world.Skills.AddSkill(fireball)
	world.Skills.AddSkill(slash)
	world.Skills.AddSkill(heal)

	// ========== 创建物品 ==========
	healthPotion := &Item{
		ID:          "item_health_potion",
		Name:        "生命药水",
		Description: "恢复50点生命值",
		Type:        ItemTypeConsumable,
		Rarity:      RarityCommon,
		Weight:      0.1,
		MaxStack:    99,
		Value:       50,
		IsUsable:    true,
		Effects: []ConsumableEffect{
			{Type: ConsumableEffectHealHP, Value: 50, Target: "self"},
		},
	}

	manaPotion := &Item{
		ID:          "item_mana_potion",
		Name:        "魔法药水",
		Description: "恢复30点魔法值",
		Type:        ItemTypeConsumable,
		Rarity:      RarityCommon,
		Weight:      0.1,
		MaxStack:    99,
		Value:       40,
		IsUsable:    true,
		Effects: []ConsumableEffect{
			{Type: ConsumableEffectHealMP, Value: 30, Target: "self"},
		},
	}

	world.Items.AddItem(healthPotion)
	world.Items.AddItem(manaPotion)

	// ========== 创建装备 ==========
	ironSword := &Weapon{
		Equipment: Equipment{
			ID:            "equip_iron_sword",
			Name:          "铁剑",
			Description:   "普通的铁制长剑",
			Type:          EquipTypeWeapon,
			Rarity:        RarityCommon,
			LevelRequired: 1,
			Stats:         EquipmentStats{Attack: 10},
			Durability:    100,
			Value:         100,
		},
		WeaponType: WeaponTypeSword,
		WeaponStats: WeaponStats{
			MinDamage:   8,
			MaxDamage:   12,
			AttackSpeed: 1.0,
		},
	}

	leatherArmor := &Equipment{
		ID:            "equip_leather_armor",
		Name:          "皮甲",
		Description:   "用皮革制成的护甲",
		Type:          EquipTypeArmor,
		Rarity:        RarityCommon,
		LevelRequired: 1,
		Stats:         EquipmentStats{Defense: 8, HP: 20},
		Durability:    80,
		Value:         80,
	}

	world.Equipments.AddWeapon(ironSword)
	world.Equipments.AddEquipment(leatherArmor)

	// ========== 创建角色模板 ==========
	warriorTemplate := &CharacterTemplate{
		ID:            "template_warrior",
		Name:          "战士模板",
		Type:          CharacterTypePlayer,
		ClassID:       "class_warrior",
		BaseStats: BaseStats{
			HP: 100, MP: 30, Attack: 12, Defense: 10,
			Magic: 3, Resistance: 5, Speed: 8, Luck: 5,
		},
		GrowthStats: GrowthStats{
			HP: 12, MP: 2, Attack: 2.5, Defense: 2,
			Magic: 0.5, Resistance: 1, Speed: 1, Luck: 0.5,
		},
		DefaultSkills: []string{"skill_slash"},
		Rarity:        RarityCommon,
	}

	goblinTemplate := &CharacterTemplate{
		ID:          "template_goblin",
		Name:        "哥布林",
		Type:        CharacterTypeEnemy,
		BaseStats: BaseStats{
			HP: 40, MP: 0, Attack: 8, Defense: 3,
			Magic: 0, Resistance: 2, Speed: 10, Luck: 3,
		},
		GrowthStats: GrowthStats{
			HP: 5, Attack: 1, Defense: 0.5, Speed: 1,
		},
		DropItems: []DropItem{
			{ItemID: "item_health_potion", Chance: 0.3, MinCount: 1, MaxCount: 1},
		},
		Rarity: RarityCommon,
	}

	merchantTemplate := &CharacterTemplate{
		ID:         "template_merchant",
		Name:       "商人",
		Type:       CharacterTypeNPC,
		DialogueID: "event_merchant_talk",
		Rarity:     RarityCommon,
	}

	world.Characters.AddTemplate(warriorTemplate)
	world.Characters.AddTemplate(goblinTemplate)
	world.Characters.AddTemplate(merchantTemplate)

	// ========== 创建地图 ==========
	startingVillage := CreateGridMap("map_starting_village", "新手村", 20, 15, TerrainNormal)
	startingVillage.Type = MapTypeTown
	startingVillage.Music = "town_theme"
	startingVillage.Description = "冒险者的起点，一个宁静的小村庄"

	// 添加传送点
	startingVillage.Teleports = append(startingVillage.Teleports, TeleportPoint{
		ID:          "tp_village_to_forest",
		Name:        "前往森林",
		TargetMapID: "map_forest",
		TargetX:     5,
		TargetY:     5,
		X:           19,
		Y:           7,
	})

	// 添加NPC
	startingVillage.Entities = append(startingVillage.Entities, MapEntity{
		ID:        "entity_merchant_1",
		Type:      "npc",
		EntityID:  "npc_merchant_1",
		Name:      "商人汤姆",
		X:         10,
		Y:         8,
		IsVisible: true,
		IsActive:  true,
	})

	forest := CreateGridMap("map_forest", "迷雾森林", 30, 25, TerrainForest)
	forest.Type = MapTypeForest
	forest.Music = "forest_theme"
	forest.Description = "被迷雾笼罩的神秘森林"
	forest.EncounterRate = 0.15
	forest.EncounterEnemies = []string{"template_goblin"}

	world.Maps.AddMap(startingVillage)
	world.Maps.AddMap(forest)

	// ========== 创建事件 ==========
	merchantTalkEvent := &Event{
		ID:   "event_merchant_talk",
		Name: "商人对话",
		X:    10, Y: 8,
		Pages: []EventPage{
			{
				ID:      0,
				Trigger: EventTriggerAction,
				Graphic: EventGraphic{CharacterName: "merchant", CharacterIndex: 0},
				List: []EventCommand{
					{Code: CmdShowText, Parameters: []interface{}{"欢迎光临！需要买点什么吗？"}},
					{Code: CmdShowChoices, Parameters: []interface{}{"购买物品", "出售物品", "离开"}},
				},
			},
		},
	}

	chestEvent := &Event{
		ID:   "event_treasure_chest",
		Name: "宝箱",
		Pages: []EventPage{
			{
				ID:      0,
				Trigger: EventTriggerAction,
				List: []EventCommand{
					{Code: CmdShowText, Parameters: []interface{}{"发现了一个宝箱！"}},
					{Code: CmdChangeItem, Parameters: []interface{}{"item_health_potion", 3}},
					{Code: CmdShowText, Parameters: []interface{}{"获得了3瓶生命药水！"}},
					{Code: CmdControlSelfSwitch, Parameters: []interface{}{"A", true}},
				},
			},
			{
				ID: 1,
				Conditions: EventConditions{
					SelfSwitchValid: true,
					SelfSwitchCh:    "A",
				},
				Trigger: EventTriggerAction,
				List: []EventCommand{
					{Code: CmdShowText, Parameters: []interface{}{"宝箱是空的。"}},
				},
			},
		},
	}

	world.Events.AddEvent(merchantTalkEvent)
	world.Events.AddEvent(chestEvent)

	// ========== 创建任务 ==========
	mainQuest := &Quest{
		ID:          "quest_main_1",
		Name:        "初出茅庐",
		Description: "击败5只哥布林，证明你的实力",
		Type:        QuestTypeMain,
		LevelRequired: 1,
		StartNPC:    "npc_merchant_1",
		Objectives: []QuestObjective{
			{
				ID:          "obj_kill_goblins",
				Type:        ObjectiveKill,
				Description: "击败哥布林",
				TargetID:    "template_goblin",
				TargetCount: 5,
			},
		},
		Rewards: QuestReward{
			Exp:   100,
			Money: 200,
			Items: []RewardItem{
				{ItemID: "item_health_potion", Count: 5},
			},
			Equipment: []string{"equip_iron_sword"},
		},
		NextQuests: []string{"quest_main_2"},
	}

	sideQuest := &Quest{
		ID:          "quest_side_1",
		Name:        "收集草药",
		Description: "收集10个草药交给药剂师",
		Type:        QuestTypeSide,
		LevelRequired: 1,
		Objectives: []QuestObjective{
			{
				ID:          "obj_collect_herbs",
				Type:        ObjectiveCollect,
				Description: "收集草药",
				TargetID:    "item_herb",
				TargetCount: 10,
			},
		},
		Rewards: QuestReward{
			Exp:   50,
			Money: 100,
			Items: []RewardItem{
				{ItemID: "item_mana_potion", Count: 3},
			},
		},
	}

	world.Quests.AddQuest(mainQuest)
	world.Quests.AddQuest(sideQuest)

	// ========== 创建玩家 ==========
	player := world.CreateCharacter("template_warrior", "勇者")
	if player != nil {
		// 给予初始物品
		player.AddItem("item_health_potion", 5)
		player.AddItem("item_mana_potion", 3)

		// 装备初始装备
		player.Equipment.Weapon = "equip_iron_sword"
		player.Equipment.Armor = "equip_leather_armor"
		player.CalculateCurrentStats(world.Equipments)

		// 设置初始位置
		player.Position.MapID = "map_starting_village"
		player.Position.X = 5
		player.Position.Y = 5

		world.SetPlayer(player)
		world.MovePlayerTo("map_starting_village", 5, 5)
	}

	// 创建NPC实例
	merchant := world.CreateCharacter("template_merchant", "商人汤姆")
	if merchant != nil {
		merchant.Position.MapID = "map_starting_village"
		merchant.Position.X = 10
		merchant.Position.Y = 8
	}

	return world
}

// CreateExampleBattle 创建示例战斗场景
func CreateExampleBattle(world *GameWorld) *BattleScene {
	// 创建敌人
	enemy := world.CreateCharacter("template_goblin", "哥布林战士")
	enemy.Level = 2
	enemy.BaseStats.HP = 50
	enemy.CurrentStats.HP = 50

	return &BattleScene{
		Enemies:   []*Character{enemy},
		Allies:    []*Character{world.Player},
		Turn:      0,
		IsActive:  true,
	}
}

// BattleScene 战斗场景
type BattleScene struct {
	Enemies  []*Character
	Allies   []*Character
	Turn     int
	IsActive bool
	Winner   string // "player", "enemy", ""
}

// ExecuteTurn 执行回合
func (bs *BattleScene) ExecuteTurn(skillMgr *SkillManager, actor *Character, skillID string, targets []*Character) *SkillResult {
	if !bs.IsActive {
		return nil
	}

	result := skillMgr.UseSkill(skillID, actor, targets)
	if result != nil {
		bs.Turn++
		bs.CheckBattleEnd()
	}

	return result
}

// CheckBattleEnd 检查战斗是否结束
func (bs *BattleScene) CheckBattleEnd() {
	// 检查敌人是否全部死亡
	allEnemiesDead := true
	for _, enemy := range bs.Enemies {
		if enemy.State != CharacterStateDead {
			allEnemiesDead = false
			break
		}
	}

	if allEnemiesDead {
		bs.IsActive = false
		bs.Winner = "player"
		return
	}

	// 检查盟友是否全部死亡
	allAlliesDead := true
	for _, ally := range bs.Allies {
		if ally.State != CharacterStateDead {
			allAlliesDead = false
			break
		}
	}

	if allAlliesDead {
		bs.IsActive = false
		bs.Winner = "enemy"
	}
}

// GetBattleResult 获取战斗结果
func (bs *BattleScene) GetBattleResult() map[string]interface{} {
	return map[string]interface{}{
		"is_active": bs.IsActive,
		"winner":    bs.Winner,
		"turn":      bs.Turn,
		"enemies":   len(bs.Enemies),
		"allies":    len(bs.Allies),
	}
}
