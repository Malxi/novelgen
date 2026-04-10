package rpg

import (
	"encoding/json"
)

// GameWorld 游戏世界 - 整合所有系统
type GameWorld struct {
	// 各个管理器
	Characters  *CharacterManager
	Items       *ItemManager
	Skills      *SkillManager
	Classes     *ClassManager
	Equipments  *EquipmentManager
	Maps        *MapManager
	Events      *EventManager
	Quests      *QuestManager
	
	// 世界上下文
	Context     *World
	
	// 玩家数据
	Player      *Character
	QuestLog    *QuestLog
	
	// 世界状态
	WorldFlags  map[string]interface{} `json:"world_flags"`
	GameTime    int64                  `json:"game_time"`
	DayNight    int                    `json:"day_night"` // 0-23
	Weather     string                 `json:"weather"`
}

// NewGameWorld 创建新的游戏世界
func NewGameWorld() *GameWorld {
	// 创建世界上下文
	ctx := &World{
		CharacterMgr: nil,
		ItemMgr:      nil,
		SkillMgr:     nil,
		ClassMgr:     nil,
		EquipmentMgr: nil,
		MapMgr:       nil,
		QuestMgr:     nil,
		Player:       nil,
		CurrentMap:   "",
	}
	
	// 创建执行器
	executor := NewEventExecutor(ctx)
	
	// 创建世界
	world := &GameWorld{
		Characters:  NewCharacterManager(),
		Items:       NewItemManager(),
		Skills:      NewSkillManager(),
		Classes:     NewClassManager(),
		Equipments:  NewEquipmentManager(),
		Maps:        NewMapManager(),
		Events:      NewEventManager(executor),
		Quests:      NewQuestManager(executor),
		Context:     ctx,
		WorldFlags:  make(map[string]interface{}),
		GameTime:    0,
		DayNight:    12,
		Weather:     "clear",
	}
	
	// 设置上下文引用
	ctx.CharacterMgr = world.Characters
	ctx.ItemMgr = world.Items
	ctx.SkillMgr = world.Skills
	ctx.ClassMgr = world.Classes
	ctx.EquipmentMgr = world.Equipments
	ctx.MapMgr = world.Maps
	ctx.QuestMgr = world.Quests
	
	return world
}

// SetPlayer 设置玩家
func (w *GameWorld) SetPlayer(player *Character) {
	w.Player = player
	w.Context.Player = player
	
	// 将玩家添加到角色管理器
	if player != nil {
		w.Characters.AddCharacterInstance(player)
	}
	
	w.QuestLog = &QuestLog{
		ActiveQuests:    make([]string, 0),
		CompletedQuests: make([]string, 0),
		FailedQuests:    make([]string, 0),
		QuestInstances:  make(map[string]*QuestInstance),
	}
}

// 创建角色
func (w *GameWorld) CreateCharacter(templateID, name string) *Character {
	char := w.Characters.CreateCharacter(templateID, name)
	if char != nil {
		// 应用职业属性
		if char.ClassID != "" {
			char.BaseStats = w.Classes.ApplyClassStats(char.ClassID, char.BaseStats, char.Level)
			char.CalculateCurrentStats(w.Equipments)
		}
	}
	return char
}

// 移动玩家
func (w *GameWorld) MovePlayerTo(mapID string, x, y float64) bool {
	if w.Player == nil {
		return false
	}
	
	// 检查地图是否存在
	m := w.Maps.GetMap(mapID)
	if m == nil {
		return false
	}
	
	// 更新玩家位置
	w.Player.Position.MapID = mapID
	w.Player.Position.X = x
	w.Player.Position.Y = y
	w.Context.CurrentMap = mapID
	
	return true
}

// 触发地图事件
func (w *GameWorld) TriggerMapEvent(x, y float64, triggerType EventTriggerType) *EventResult {
	if w.Player == nil {
		return nil
	}
	
	// 获取当前地图
	m := w.Maps.GetMap(w.Context.CurrentMap)
	if m == nil {
		return nil
	}
	
	// 查找位置上的实体
	entities := w.Maps.GetEntitiesAt(w.Context.CurrentMap, x, y, 1.0)
	
	for _, entity := range entities {
		if entity.Type == "event" && entity.IsActive {
			// 获取事件
			event := w.Events.GetEvent(entity.EntityID)
			if event != nil {
				// 找到匹配触发类型的页面
				for i, page := range event.Pages {
					if page.Trigger == triggerType {
						return w.Events.TriggerEvent(event.ID, i)
					}
				}
			}
		}
	}
	
	return nil
}

// 与NPC对话
func (w *GameWorld) TalkToNPC(npcID string) *EventResult {
	if w.Player == nil {
		return nil
	}
	
	// 获取NPC
	npc := w.Characters.GetCharacter(npcID)
	if npc == nil || npc.Type != CharacterTypeNPC {
		return nil
	}
	
	// 检查距离
	dx := npc.Position.X - w.Player.Position.X
	dy := npc.Position.Y - w.Player.Position.Y
	if dx*dx+dy*dy > 4.0 { // 距离大于2
		return nil
	}
	
	// 触发NPC的对话事件
	if npc.TemplateID != "" {
		template := w.Characters.GetTemplate(npc.TemplateID)
		if template != nil && template.DialogueID != "" {
			event := w.Events.GetEvent(template.DialogueID)
			if event != nil {
				return w.Events.TriggerEvent(event.ID, 0)
			}
		}
	}
	
	return nil
}

// 接取任务
func (w *GameWorld) AcceptQuest(questID string) bool {
	if w.Player == nil {
		return false
	}
	
	canAccept, _ := w.Quests.CanAcceptQuest(questID, w.Player, w.QuestLog)
	if !canAccept {
		return false
	}
	
	instance := w.Quests.AcceptQuest(questID, w.Player)
	if instance != nil {
		w.QuestLog.ActiveQuests = append(w.QuestLog.ActiveQuests, questID)
		w.QuestLog.QuestInstances[questID] = instance
		return true
	}
	
	return false
}

// 提交任务
func (w *GameWorld) TurnInQuest(questID string) bool {
	if w.Player == nil {
		return false
	}
	
	rewards := w.Quests.TurnInQuest(questID, w.Player)
	if rewards == nil {
		return false
	}
	
	// 更新任务日志
	instance := w.QuestLog.QuestInstances[questID]
	if instance != nil {
		instance.Status = QuestStatusTurnedIn
		
		// 从活跃列表移除
		for i, id := range w.QuestLog.ActiveQuests {
			if id == questID {
				w.QuestLog.ActiveQuests = append(w.QuestLog.ActiveQuests[:i], w.QuestLog.ActiveQuests[i+1:]...)
				break
			}
		}
		
		// 添加到完成列表
		w.QuestLog.CompletedQuests = append(w.QuestLog.CompletedQuests, questID)
	}
	
	return true
}

// 使用技能
func (w *GameWorld) UseSkill(skillID string, targetIDs []string) *SkillResult {
	if w.Player == nil {
		return nil
	}
	
	// 检查是否可以使用
	canUse, _ := w.Skills.CanUseSkill(skillID, w.Player)
	if !canUse {
		return nil
	}
	
	// 获取目标
	targets := make([]*Character, 0)
	for _, targetID := range targetIDs {
		target := w.Characters.GetCharacter(targetID)
		if target != nil {
			targets = append(targets, target)
		}
	}
	
	if len(targets) == 0 {
		return nil
	}
	
	return w.Skills.UseSkill(skillID, w.Player, targets)
}

// 使用物品
func (w *GameWorld) UseItem(itemID string, targetID string) []Effect {
	if w.Player == nil {
		return nil
	}
	
	// 检查是否有该物品
	hasItem := false
	for _, item := range w.Player.Items {
		if item.ItemID == itemID && item.Count > 0 {
			hasItem = true
			break
		}
	}
	
	if !hasItem {
		return nil
	}
	
	// 获取目标
	target := w.Characters.GetCharacter(targetID)
	if target == nil {
		target = w.Player
	}
	
	// 使用物品
	effects := w.Items.UseItem(itemID, target)
	
	// 消耗物品
	if effects != nil {
		w.Player.RemoveItem(itemID, 1)
	}
	
	return effects
}

// 装备物品
func (w *GameWorld) EquipItem(equipmentID string) bool {
	if w.Player == nil {
		return false
	}
	
	// 检查是否可以装备
	canEquip, _ := w.Equipments.CanEquip(equipmentID, w.Player)
	if !canEquip {
		return false
	}
	
	// 获取装备
	equip := w.Equipments.GetEquipment(equipmentID)
	if equip == nil {
		return false
	}
	
	// 根据类型装备到对应槽位
	switch equip.Type {
	case EquipTypeWeapon:
		w.Player.Equipment.Weapon = equipmentID
	case EquipTypeArmor:
		w.Player.Equipment.Armor = equipmentID
	case EquipTypeHelmet:
		w.Player.Equipment.Helmet = equipmentID
	case EquipTypeAccessory:
		if w.Player.Equipment.Accessory1 == "" {
			w.Player.Equipment.Accessory1 = equipmentID
		} else {
			w.Player.Equipment.Accessory2 = equipmentID
		}
	}
	
	// 重新计算属性
	w.Player.CalculateCurrentStats(w.Equipments)
	
	return true
}

// 卸下装备
func (w *GameWorld) UnequipItem(slot string) bool {
	if w.Player == nil {
		return false
	}
	
	switch slot {
	case "weapon":
		w.Player.Equipment.Weapon = ""
	case "armor":
		w.Player.Equipment.Armor = ""
	case "helmet":
		w.Player.Equipment.Helmet = ""
	case "accessory1":
		w.Player.Equipment.Accessory1 = ""
	case "accessory2":
		w.Player.Equipment.Accessory2 = ""
	default:
		return false
	}
	
	// 重新计算属性
	w.Player.CalculateCurrentStats(w.Equipments)
	
	return true
}

// 添加物品到玩家背包
func (w *GameWorld) AddItemToPlayer(itemID string, count int) bool {
	if w.Player == nil {
		return false
	}
	
	w.Player.AddItem(itemID, count)
	return true
}

// 从玩家背包移除物品
func (w *GameWorld) RemoveItemFromPlayer(itemID string, count int) bool {
	if w.Player == nil {
		return false
	}
	
	return w.Player.RemoveItem(itemID, count)
}

// 给予玩家经验
func (w *GameWorld) GivePlayerExp(amount int) bool {
	if w.Player == nil {
		return false
	}
	
	leveledUp := w.Player.GainExp(amount)
	if leveledUp {
		// 重新计算属性
		w.Player.CalculateCurrentStats(w.Equipments)
	}
	
	return leveledUp
}

// 学习技能
func (w *GameWorld) LearnSkill(skillID string) bool {
	if w.Player == nil {
		return false
	}
	
	return w.Player.LearnSkill(skillID)
}

// 获取玩家战力
func (w *GameWorld) GetPlayerBattlePower() BattlePower {
	if w.Player == nil {
		return BattlePower{}
	}
	
	return w.Player.CalculateBattlePower()
}

// 更新游戏时间
func (w *GameWorld) UpdateGameTime(delta int64) {
	w.GameTime += delta
	w.DayNight = int((w.GameTime / 3600) % 24) // 每小时更新
}

// 设置世界标志
func (w *GameWorld) SetWorldFlag(key string, value interface{}) {
	w.WorldFlags[key] = value
}

// 获取世界标志
func (w *GameWorld) GetWorldFlag(key string) interface{} {
	return w.WorldFlags[key]
}

// 保存游戏世界
func (w *GameWorld) SaveToJSON() string {
	data := map[string]interface{}{
		"player":       w.Player,
		"quest_log":    w.QuestLog,
		"world_flags":  w.WorldFlags,
		"game_time":    w.GameTime,
		"day_night":    w.DayNight,
		"weather":      w.Weather,
		"current_map":  w.Context.CurrentMap,
	}
	
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	return string(jsonData)
}

// 加载游戏数据
func (w *GameWorld) LoadFromJSON(data string) error {
	var saveData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &saveData); err != nil {
		return err
	}
	
	// 恢复玩家数据
	if playerData, ok := saveData["player"].(map[string]interface{}); ok {
		playerJSON, _ := json.Marshal(playerData)
		var player Character
		if err := json.Unmarshal(playerJSON, &player); err == nil {
			w.Player = &player
			w.Context.Player = &player
		}
	}
	
	// 恢复其他数据...
	if flags, ok := saveData["world_flags"].(map[string]interface{}); ok {
		w.WorldFlags = flags
	}
	
	if gameTime, ok := saveData["game_time"].(float64); ok {
		w.GameTime = int64(gameTime)
	}
	
	if dayNight, ok := saveData["day_night"].(float64); ok {
		w.DayNight = int(dayNight)
	}
	
	if weather, ok := saveData["weather"].(string); ok {
		w.Weather = weather
	}
	
	if currentMap, ok := saveData["current_map"].(string); ok {
		w.Context.CurrentMap = currentMap
	}
	
	return nil
}

// 获取世界状态摘要
func (w *GameWorld) GetWorldSummary() map[string]interface{} {
	return map[string]interface{}{
		"player_name":    w.Player.Name,
		"player_level":   w.Player.Level,
		"player_class":   w.Player.ClassID,
		"current_map":    w.Context.CurrentMap,
		"active_quests":  len(w.QuestLog.ActiveQuests),
		"completed_quests": len(w.QuestLog.CompletedQuests),
		"game_time":      w.GameTime,
		"day_night":      w.DayNight,
		"weather":        w.Weather,
	}
}
