package rpg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EnhancedSimulationResult 增强版模拟结果
type EnhancedSimulationResult struct {
	SimulationResult
	PlayerStats    PlayerStatsLog `json:"player_stats"`
	BattleReports  []BattleReport `json:"battle_reports,omitempty"`
	Rewards        []RewardLog    `json:"rewards"`
	FullLog        []LogEntry     `json:"full_log"`
	TotalExpGained int            `json:"total_exp_gained"`
	FinalLevel     int            `json:"final_level"`
}

// PlayerStatsLog 玩家状态日志
type PlayerStatsLog struct {
	Initial LevelStats `json:"initial"`
	Final   LevelStats `json:"final"`
}

// LevelStats 等级状态
type LevelStats struct {
	Level   int `json:"level"`
	HP      int `json:"hp"`
	MaxHP   int `json:"max_hp"`
	MP      int `json:"mp"`
	MaxMP   int `json:"max_mp"`
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
	Exp     int `json:"exp"`
}

// BattleReport 战斗报告
type BattleReport struct {
	EnemyName   string       `json:"enemy_name"`
	EnemyLevel  int          `json:"enemy_level"`
	Turns       int          `json:"turns"`
	Victory     bool         `json:"victory"`
	DamageDealt int          `json:"damage_dealt"`
	DamageTaken int          `json:"damage_taken"`
	TurnLog     []BattleTurn `json:"turn_log"`
}

// BattleTurn 战斗回合
type BattleTurn struct {
	TurnNumber int    `json:"turn_number"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Damage     int    `json:"damage"`
	SkillUsed  string `json:"skill_used,omitempty"`
	PlayerHP   int    `json:"player_hp"`
	EnemyHP    int    `json:"enemy_hp"`
}

// RewardLog 奖励日志
type RewardLog struct {
	Type     string `json:"type"` // exp, item, gold
	Amount   int    `json:"amount"`
	ItemName string `json:"item_name,omitempty"`
	Reason   string `json:"reason"`
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp int64                  `json:"timestamp"`
	Type      string                 `json:"type"` // info, combat, reward, event
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// EnhancedSimulationEngine 增强版推演引擎
type EnhancedSimulationEngine struct {
	*SimulationEngine
	Log              []LogEntry
	CurrentBattle    *BattleReport
	TotalExpGained   int
	ItemsCollected   []string
	TotalDamageDealt int
	TotalDamageTaken int
	TotalHPHealed    int
	TotalMPHealed    int
	CritCount        int
	DodgeCount       int
	StatusEffects    []StatusEffect
}

// StatusEffect 状态效果
type StatusEffect struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Duration    int    `json:"duration"`
	EffectType  string `json:"effect_type"` // buff, debuff
	Stat        string `json:"stat,omitempty"`
	Value       int    `json:"value,omitempty"`
}

// NewEnhancedSimulationEngine 创建增强版推演引擎
func NewEnhancedSimulationEngine(world *GameWorld) *EnhancedSimulationEngine {
	return &EnhancedSimulationEngine{
		SimulationEngine: NewSimulationEngine(world),
		Log:              make([]LogEntry, 0),
		ItemsCollected:   make([]string, 0),
		StatusEffects:    make([]StatusEffect, 0),
	}
}

// SimulateChapterEnhanced 增强版章节推演
func (ese *EnhancedSimulationEngine) SimulateChapterEnhanced(chapterID string) (*EnhancedSimulationResult, error) {
	// 清空日志和章节级计数器
	ese.Log = make([]LogEntry, 0)
	chapterExpGained := 0

	// 记录初始状态
	initialStats := ese.recordPlayerStats()

	// 查找任务
	quest := ese.World.Quests.GetQuest("quest_" + chapterID)
	if quest == nil {
		return nil, fmt.Errorf("章节 %s 对应的任务不存在", chapterID)
	}

	result := &EnhancedSimulationResult{
		SimulationResult: SimulationResult{
			ChapterID:   chapterID,
			ChapterName: quest.Name,
			Steps:       make([]SimulationStep, 0),
			StartTime:   time.Now().Unix(),
		},
		PlayerStats: PlayerStatsLog{
			Initial: initialStats,
		},
		BattleReports: make([]BattleReport, 0),
		Rewards:       make([]RewardLog, 0),
		FullLog:       make([]LogEntry, 0),
	}

	// 添加开始日志
	ese.addLog("info", fmt.Sprintf("开始模拟章节: %s", quest.Name), map[string]interface{}{
		"chapter_id": chapterID,
		"quest_id":   quest.ID,
	})

	// 接取任务
	if !ese.World.AcceptQuest(quest.ID) {
		return nil, fmt.Errorf("无法接取任务 %s", quest.ID)
	}

	ese.addLog("info", fmt.Sprintf("接取任务: %s", quest.Name), nil)

	// 推演每个目标
	for i, objective := range quest.Objectives {
		step := ese.simulateObjectiveEnhanced(objective, i+1)
		result.Steps = append(result.Steps, step)

		// 复制当前日志到结果，然后清空
		result.FullLog = append(result.FullLog, ese.Log...)
		ese.Log = make([]LogEntry, 0)

		// 标记目标完成
		ese.World.Quests.UpdateObjective(quest.ID, objective.ID, 1)

		// 如果是战斗，添加战斗报告
		if ese.CurrentBattle != nil && (step.Type == "kill" || step.Type == "defeat") {
			result.BattleReports = append(result.BattleReports, *ese.CurrentBattle)
			ese.CurrentBattle = nil
		}
	}

	// 完成任务并获取奖励
	rewards := ese.World.Quests.TurnInQuest(quest.ID, ese.World.Player)
	if rewards != nil {
		// 记录经验奖励（TurnInQuest 已经调用了 GainExp）
		if rewards.Exp > 0 {
			chapterExpGained += rewards.Exp
			result.Rewards = append(result.Rewards, RewardLog{
				Type:   "exp",
				Amount: rewards.Exp,
				Reason: "完成任务",
			})
		}

		// 记录物品奖励
		for _, rewardItem := range rewards.Items {
			item := ese.World.Items.GetItem(rewardItem.ItemID)
			if item != nil {
				result.Rewards = append(result.Rewards, RewardLog{
					Type:     "item",
					ItemName: item.Name,
					Reason:   "任务奖励",
				})
				ese.ItemsCollected = append(ese.ItemsCollected, item.Name)
			}
		}
	}

	result.EndTime = time.Now().Unix()
	result.Success = true
	result.PlayerStats.Final = ese.recordPlayerStats()
	result.TotalExpGained = chapterExpGained
	result.FinalLevel = ese.World.Player.Level

	// 添加完成日志
	ese.addLog("info", fmt.Sprintf("章节模拟完成: %s", quest.Name), map[string]interface{}{
		"success":     true,
		"exp_gained":  result.TotalExpGained,
		"steps_count": len(result.Steps),
	})
	result.FullLog = append(result.FullLog, ese.Log...)

	return result, nil
}

// simulateObjectiveEnhanced 增强版目标推演
func (ese *EnhancedSimulationEngine) simulateObjectiveEnhanced(objective QuestObjective, stepNum int) SimulationStep {
	step := SimulationStep{
		StepNumber:   stepNum,
		Type:         string(objective.Type),
		Description:  objective.Description,
		Timestamp:    time.Now().Unix(),
		StateChanges: make(map[string]interface{}),
	}

	// 如果目标有位置信息，先移动到该位置
	if objective.LocationID != "" && ese.World.Player.Position.MapID != objective.LocationID {
		gameMap := ese.World.Maps.GetMap(objective.LocationID)
		if gameMap != nil {
			oldLocation := ese.World.Player.Position.MapID
			ese.World.MovePlayerTo(objective.LocationID, 10, 10)
			step.Location = objective.LocationID
			step.StateChanges["location_changed"] = objective.LocationID
			ese.addLog("event", fmt.Sprintf("移动: %s → %s", oldLocation, gameMap.Name), nil)
		}
	}

	switch objective.Type {
	case ObjectiveKill, ObjectiveDefeat:
		step = ese.simulateCombatObjectiveEnhanced(step, objective)
	case ObjectiveCollect:
		step = ese.simulateCollectObjectiveEnhanced(step, objective)
	case ObjectiveTalk:
		step = ese.simulateTalkObjectiveEnhanced(step, objective)
	case ObjectiveReach:
		step = ese.simulateMoveObjectiveEnhanced(step, objective)
	default:
		step = ese.simulateEventObjectiveEnhanced(step, objective)
	}

	return step
}

// recordPlayerStats 记录玩家状态
func (ese *EnhancedSimulationEngine) recordPlayerStats() LevelStats {
	if ese.World.Player == nil {
		return LevelStats{}
	}
	return LevelStats{
		Level:   ese.World.Player.Level,
		HP:      ese.World.Player.CurrentStats.HP,
		MaxHP:   ese.World.Player.BaseStats.HP,
		MP:      ese.World.Player.CurrentStats.MP,
		MaxMP:   ese.World.Player.BaseStats.MP,
		Attack:  ese.World.Player.BaseStats.Attack,
		Defense: ese.World.Player.BaseStats.Defense,
		Exp:     ese.World.Player.Exp,
	}
}

// addLog 添加日志
func (ese *EnhancedSimulationEngine) addLog(logType, message string, details map[string]interface{}) {
	ese.Log = append(ese.Log, LogEntry{
		Timestamp: time.Now().Unix(),
		Type:      logType,
		Message:   message,
		Details:   details,
	})
}

// simulateCombatObjectiveEnhanced 增强版战斗目标推演
func (ese *EnhancedSimulationEngine) simulateCombatObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
	// 获取目标敌人
	enemy := ese.World.Characters.GetCharacter(objective.TargetID)
	if enemy == nil {
		step.Results = append(step.Results, ActionResult{
			Success: false,
			Message: "敌人不存在",
		})
		return step
	}

	step.Characters = []string{ese.World.Player.ID, enemy.ID}
	step.Location = ese.World.Player.Position.MapID

	// 创建战斗报告
	ese.CurrentBattle = &BattleReport{
		EnemyName:   enemy.Name,
		EnemyLevel:  enemy.Level,
		Turns:       0,
		Victory:     false,
		DamageDealt: 0,
		DamageTaken: 0,
		TurnLog:     make([]BattleTurn, 0),
	}

	// 创建战斗场景
	battle := &BattleScene{
		Enemies:  []*Character{enemy},
		Allies:   []*Character{ese.World.Player},
		IsActive: true,
	}

	turnCount := 0
	maxTurns := 20

	ese.addLog("combat", fmt.Sprintf("战斗开始: %s vs %s", ese.World.Player.Name, enemy.Name), map[string]interface{}{
		"player_hp":  ese.World.Player.CurrentStats.HP,
		"enemy_hp":   enemy.CurrentStats.HP,
		"player_atk": ese.World.Player.CurrentStats.Attack,
		"enemy_atk":  enemy.CurrentStats.Attack,
	})

	// 战斗循环
	for battle.IsActive && turnCount < maxTurns {
		turnCount++
		ese.CurrentBattle.Turns++

		// 玩家回合
		if ese.World.Player.State != CharacterStateDead {
			action := ese.decidePlayerAction(enemy)
			actionResult := ese.executeAction(action, battle)

			// 记录回合日志
			turn := BattleTurn{
				TurnNumber: turnCount,
				Actor:      ese.World.Player.Name,
				Action:     action.ActionType,
				Target:     enemy.Name,
				Damage:     actionResult.Damage,
				PlayerHP:   ese.World.Player.CurrentStats.HP,
				EnemyHP:    enemy.CurrentStats.HP,
			}
			if action.SkillID != "" {
				turn.SkillUsed = action.SkillID
			}
			ese.CurrentBattle.TurnLog = append(ese.CurrentBattle.TurnLog, turn)
			ese.CurrentBattle.DamageDealt += actionResult.Damage
			ese.TotalDamageDealt += actionResult.Damage

			// 统计暴击
			if strings.Contains(actionResult.Message, "暴击") {
				ese.CritCount++
			}

			step.Actions = append(step.Actions, action)
			step.Results = append(step.Results, actionResult)

			ese.addLog("combat", fmt.Sprintf("[回合%d] %s 攻击 %s，造成 %d 伤害",
				turnCount, ese.World.Player.Name, enemy.Name, actionResult.Damage), nil)
		}

		// 检查战斗结束
		battle.CheckBattleEnd()
		if !battle.IsActive {
			break
		}

		// 敌人回合
		if enemy.State != CharacterStateDead {
			action := ese.decideEnemyAction(enemy, ese.World.Player)
			actionResult := ese.executeAction(action, battle)

			// 记录回合日志
			turn := BattleTurn{
				TurnNumber: turnCount,
				Actor:      enemy.Name,
				Action:     action.ActionType,
				Target:     ese.World.Player.Name,
				Damage:     actionResult.Damage,
				PlayerHP:   ese.World.Player.CurrentStats.HP,
				EnemyHP:    enemy.CurrentStats.HP,
			}
			if action.SkillID != "" {
				turn.SkillUsed = action.SkillID
			}
			ese.CurrentBattle.TurnLog = append(ese.CurrentBattle.TurnLog, turn)
			ese.CurrentBattle.DamageTaken += actionResult.Damage
			ese.TotalDamageTaken += actionResult.Damage

			// 统计闪避
			if strings.Contains(actionResult.Message, "闪避") {
				ese.DodgeCount++
			}

			step.Actions = append(step.Actions, action)
			step.Results = append(step.Results, actionResult)

			ese.addLog("combat", fmt.Sprintf("[回合%d] %s 攻击 %s，造成 %d 伤害",
				turnCount, enemy.Name, ese.World.Player.Name, actionResult.Damage), nil)
		}

		battle.CheckBattleEnd()
	}

	// 记录战斗结果
	if battle.Winner == "player" {
		ese.CurrentBattle.Victory = true
		step.StateChanges["enemy_defeated"] = enemy.ID
		expGained := enemy.Level * 20
		leveledUp, statChanges := ese.World.Player.GainExp(expGained)
		ese.TotalExpGained += expGained
		step.StateChanges["exp_gained"] = expGained
		if leveledUp {
			step.StateChanges["level_up"] = ese.World.Player.Level
			ese.addLog("reward", fmt.Sprintf("升级！等级 %d → %d", ese.World.Player.Level-1, ese.World.Player.Level), nil)
			// 显示属性提升
			if hp, ok := statChanges["hp"]; ok && hp > 0 {
				ese.addLog("reward", fmt.Sprintf("  HP +%d", hp), nil)
			}
			if atk, ok := statChanges["attack"]; ok && atk > 0 {
				ese.addLog("reward", fmt.Sprintf("  攻击 +%d", atk), nil)
			}
			if def, ok := statChanges["defense"]; ok && def > 0 {
				ese.addLog("reward", fmt.Sprintf("  防御 +%d", def), nil)
			}
		}
		step.StateChanges["player_level"] = ese.World.Player.Level
		step.StateChanges["player_exp"] = ese.World.Player.Exp
		step.StateChanges["player_hp"] = ese.World.Player.BaseStats.HP
		step.StateChanges["player_mp"] = ese.World.Player.BaseStats.MP
		step.StateChanges["player_attack"] = ese.World.Player.BaseStats.Attack
		step.StateChanges["player_defense"] = ese.World.Player.BaseStats.Defense
		step.Description = fmt.Sprintf("战斗胜利！击败了 %s (用时 %d 回合)", enemy.Name, turnCount)
		ese.addLog("reward", fmt.Sprintf("战斗胜利！获得经验 %d", expGained), nil)
	} else {
		step.Description = fmt.Sprintf("战斗失败，被 %s 击败", enemy.Name)
		ese.addLog("combat", fmt.Sprintf("战斗失败！"), nil)
	}

	return step
}

// simulateCollectObjectiveEnhanced 增强版收集目标
func (ese *EnhancedSimulationEngine) simulateCollectObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
	step.Characters = append(step.Characters, ese.World.Player.ID)
	step.Location = ese.World.Player.Position.MapID

	item := ese.World.Items.GetItem(objective.TargetID)
	if item != nil {
		se := ese.World.AddItemToPlayer(item.ID, objective.TargetCount)
		step.StateChanges["item_collected"] = item.ID
		step.StateChanges["count"] = objective.TargetCount
		step.Description = fmt.Sprintf("收集了 %s x%d", item.Name, objective.TargetCount)
		ese.addLog("reward", fmt.Sprintf("获得物品: %s x%d", item.Name, objective.TargetCount), nil)
		ese.ItemsCollected = append(ese.ItemsCollected, item.Name)
		_ = se

		// 根据物品类型给予属性增益
		if item.Type == "consumable" {
			// 消耗品：恢复HP/MP
			hpRestore := 10 + ese.Random.Intn(20)
			mpRestore := 5 + ese.Random.Intn(15)
			ese.World.Player.CurrentStats.HP = min(ese.World.Player.BaseStats.HP, ese.World.Player.CurrentStats.HP+hpRestore)
			ese.World.Player.CurrentStats.MP = min(ese.World.Player.BaseStats.MP, ese.World.Player.CurrentStats.MP+mpRestore)
			step.StateChanges["hp_restored"] = hpRestore
			step.StateChanges["mp_restored"] = mpRestore
			ese.TotalHPHealed += hpRestore
			ese.TotalMPHealed += mpRestore
			ese.addLog("event", fmt.Sprintf("使用 %s，恢复 HP+%d MP+%d", item.Name, hpRestore, mpRestore), nil)
		} else if item.Type == "equipment" || item.Type == "weapon" {
			// 装备：增加属性
			atkBonus := 1 + ese.Random.Intn(3)
			defBonus := 1 + ese.Random.Intn(2)
			ese.World.Player.BaseStats.Attack += atkBonus
			ese.World.Player.BaseStats.Defense += defBonus
			step.StateChanges["attack_bonus"] = atkBonus
			step.StateChanges["defense_bonus"] = defBonus
			ese.addLog("reward", fmt.Sprintf("装备 %s，攻击+%d 防御+%d", item.Name, atkBonus, defBonus), nil)
		}
	}

	// 添加经验值奖励
	expReward := 8 + ese.Random.Intn(12)
	leveledUp, _ := ese.World.Player.GainExp(expReward)
	ese.TotalExpGained += expReward
	step.StateChanges["exp_gained"] = expReward
	if leveledUp {
		step.StateChanges["level_up"] = ese.World.Player.Level
		ese.addLog("reward", fmt.Sprintf("升级！等级 %d → %d", ese.World.Player.Level-1, ese.World.Player.Level), nil)
	}
	step.StateChanges["player_level"] = ese.World.Player.Level
	step.StateChanges["player_exp"] = ese.World.Player.Exp

	// 更新玩家当前属性
	step.StateChanges["player_hp"] = ese.World.Player.BaseStats.HP
	step.StateChanges["player_mp"] = ese.World.Player.BaseStats.MP
	step.StateChanges["player_attack"] = ese.World.Player.BaseStats.Attack
	step.StateChanges["player_defense"] = ese.World.Player.BaseStats.Defense

	return step
}

// simulateTalkObjectiveEnhanced 增强版对话目标
func (ese *EnhancedSimulationEngine) simulateTalkObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
	step.Characters = append(step.Characters, ese.World.Player.ID)
	step.Location = ese.World.Player.Position.MapID

	npc := ese.World.Characters.GetCharacter(objective.TargetID)
	if npc != nil {
		action := Action{
			Actor:      ese.World.Player.ID,
			ActionType: "talk",
			Target:     npc.ID,
		}

		eventResult := ese.World.TalkToNPC(npc.ID)

		step.Actions = append(step.Actions, action)
		step.Results = append(step.Results, ActionResult{
			Success: eventResult != nil,
			Action:  action,
			Message: fmt.Sprintf("与 %s 对话", npc.Name),
		})

		step.Characters = append(step.Characters, npc.ID)
		step.StateChanges["npc_talked"] = npc.ID
		step.Description = fmt.Sprintf("与 %s 进行了对话", npc.Name)
		ese.addLog("event", fmt.Sprintf("与 %s 对话", npc.Name), nil)

		// 对话可能带来属性增益（情报、鼓励、技能指导等）
		if ese.Random.Intn(100) < 30 {
			// 30%概率获得临时增益
			bonusType := ese.Random.Intn(3)
			switch bonusType {
			case 0:
				// 获得情报，增加经验
				bonusExp := 5 + ese.Random.Intn(10)
				_, _ = ese.World.Player.GainExp(bonusExp)
				ese.TotalExpGained += bonusExp
				step.StateChanges["bonus_exp"] = bonusExp
				ese.addLog("reward", fmt.Sprintf("从 %s 获得情报，额外经验 +%d", npc.Name, bonusExp), nil)
			case 1:
				// 获得鼓励，恢复MP
				mpRestore := 5 + ese.Random.Intn(10)
				ese.World.Player.CurrentStats.MP = min(ese.World.Player.BaseStats.MP, ese.World.Player.CurrentStats.MP+mpRestore)
				step.StateChanges["mp_restored"] = mpRestore
				ese.TotalMPHealed += mpRestore
				ese.addLog("event", fmt.Sprintf("%s 的鼓励让你恢复了 MP+%d", npc.Name, mpRestore), nil)
			case 2:
				// 获得指导，临时增加攻击
				atkBonus := 1 + ese.Random.Intn(2)
				ese.World.Player.BaseStats.Attack += atkBonus
				step.StateChanges["attack_bonus"] = atkBonus
				ese.addLog("reward", fmt.Sprintf("从 %s 学到技巧，攻击 +%d", npc.Name, atkBonus), nil)
			}
		}
	}

	// 添加经验值奖励
	expReward := 5 + ese.Random.Intn(10)
	leveledUp, _ := ese.World.Player.GainExp(expReward)
	ese.TotalExpGained += expReward
	step.StateChanges["exp_gained"] = expReward
	if leveledUp {
		step.StateChanges["level_up"] = ese.World.Player.Level
		ese.addLog("reward", fmt.Sprintf("升级！等级 %d → %d", ese.World.Player.Level-1, ese.World.Player.Level), nil)
	}
	step.StateChanges["player_level"] = ese.World.Player.Level
	step.StateChanges["player_exp"] = ese.World.Player.Exp

	// 更新玩家当前属性
	step.StateChanges["player_hp"] = ese.World.Player.BaseStats.HP
	step.StateChanges["player_mp"] = ese.World.Player.BaseStats.MP
	step.StateChanges["player_attack"] = ese.World.Player.BaseStats.Attack
	step.StateChanges["player_defense"] = ese.World.Player.BaseStats.Defense

	return step
}

// simulateMoveObjectiveEnhanced 增强版移动目标
func (ese *EnhancedSimulationEngine) simulateMoveObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
	step.Characters = append(step.Characters, ese.World.Player.ID)

	gameMap := ese.World.Maps.GetMap(objective.TargetID)
	if gameMap != nil {
		oldLocation := ese.World.Player.Position.MapID
		ese.World.MovePlayerTo(objective.TargetID, 10, 10)

		action := Action{
			Actor:      ese.World.Player.ID,
			ActionType: "move",
			Parameters: map[string]interface{}{
				"from": oldLocation,
				"to":   objective.TargetID,
			},
		}

		step.Actions = append(step.Actions, action)
		step.Location = objective.TargetID
		step.StateChanges["location_changed"] = objective.TargetID
		step.Description = fmt.Sprintf("移动到了 %s", gameMap.Name)
		ese.addLog("event", fmt.Sprintf("移动: %s → %s", oldLocation, gameMap.Name), nil)

		// 移动时可能触发随机事件
		if ese.Random.Intn(100) < 35 {
			// 35%概率触发随机事件
			eventType := ese.Random.Intn(6)
			switch eventType {
			case 0:
				// 发现隐藏路径
				bonusExp := 8 + ese.Random.Intn(12)
				_, _ = ese.World.Player.GainExp(bonusExp)
				ese.TotalExpGained += bonusExp
				step.StateChanges["bonus_exp"] = bonusExp
				ese.addLog("event", fmt.Sprintf("在 %s 发现了隐藏路径，获得经验 +%d", gameMap.Name, bonusExp), nil)
			case 1:
				// 遭遇环境危险
				damage := 5 + ese.Random.Intn(10)
				ese.World.Player.CurrentStats.HP -= damage
				if ese.World.Player.CurrentStats.HP < 0 {
					ese.World.Player.CurrentStats.HP = 0
				}
				step.StateChanges["environment_damage"] = damage
				ese.addLog("event", fmt.Sprintf("在 %s 遭遇了环境危险，受到 %d 点伤害", gameMap.Name, damage), nil)
			case 2:
				// 发现资源点
				hpRestore := 10 + ese.Random.Intn(15)
				ese.World.Player.CurrentStats.HP = min(ese.World.Player.BaseStats.HP, ese.World.Player.CurrentStats.HP+hpRestore)
				step.StateChanges["hp_restored"] = hpRestore
				ese.TotalHPHealed += hpRestore
				ese.addLog("event", fmt.Sprintf("在 %s 发现了资源点，恢复 HP+%d", gameMap.Name, hpRestore), nil)
			case 3:
				// 获得环境增益
				defBonus := 1 + ese.Random.Intn(2)
				ese.World.Player.BaseStats.Defense += defBonus
				step.StateChanges["defense_bonus"] = defBonus
				ese.addLog("reward", fmt.Sprintf("适应了 %s 的环境，防御 +%d", gameMap.Name, defBonus), nil)
			case 4:
				// 发现敌人踪迹
				ese.addLog("event", fmt.Sprintf("在 %s 发现了敌人踪迹，需要保持警惕", gameMap.Name), nil)
				step.StateChanges["enemy_spotted"] = true
			case 5:
				// 获得灵感
				mpRestore := 5 + ese.Random.Intn(10)
				ese.World.Player.CurrentStats.MP = min(ese.World.Player.BaseStats.MP, ese.World.Player.CurrentStats.MP+mpRestore)
				step.StateChanges["mp_restored"] = mpRestore
				ese.TotalMPHealed += mpRestore
				ese.addLog("event", fmt.Sprintf("在 %s 获得了灵感，恢复 MP+%d", gameMap.Name, mpRestore), nil)
			}
		}
	}

	// 添加经验值奖励
	expReward := 3 + ese.Random.Intn(8)
	leveledUp, _ := ese.World.Player.GainExp(expReward)
	ese.TotalExpGained += expReward
	step.StateChanges["exp_gained"] = expReward
	if leveledUp {
		step.StateChanges["level_up"] = ese.World.Player.Level
		ese.addLog("reward", fmt.Sprintf("升级！等级 %d → %d", ese.World.Player.Level-1, ese.World.Player.Level), nil)
	}
	step.StateChanges["player_level"] = ese.World.Player.Level
	step.StateChanges["player_exp"] = ese.World.Player.Exp

	// 更新玩家当前属性
	step.StateChanges["player_hp"] = ese.World.Player.BaseStats.HP
	step.StateChanges["player_mp"] = ese.World.Player.BaseStats.MP
	step.StateChanges["player_attack"] = ese.World.Player.BaseStats.Attack
	step.StateChanges["player_defense"] = ese.World.Player.BaseStats.Defense

	return step
}

// checkAndTriggerBreakthrough 检测并触发突破事件
func (ese *EnhancedSimulationEngine) checkAndTriggerBreakthrough(step SimulationStep, objective QuestObjective) bool {
	if ese.World.Player == nil {
		return false
	}

	// 检查目标描述中是否包含突破关键词
	description := strings.ToLower(objective.Description)
	targetID := strings.ToLower(objective.TargetID)

	// 基因进化关键词
	geneKeywords := []string{"基因进化", "基因药剂", "进化成功", "进化为", "基因等级"}
	// 修仙突破关键词
	cultivationKeywords := []string{"突破", "筑基", "金丹", "元婴", "化神", "炼虚", "大乘", "渡劫", "炼气"}
	// 超能力觉醒关键词
	superpowerKeywords := []string{"觉醒", "超能力", "能力觉醒", "能力突破"}
	// 武道突破关键词
	martialKeywords := []string{"淬体", "开脉", "凝气", "先天", "宗师", "武神"}

	// 检测基因进化
	if containsAnyKeyword(description, geneKeywords) || containsAnyKeyword(targetID, geneKeywords) {
		// 尝试提取阶段名称
		stageName := extractGeneStage(description, targetID)
		if stageName == "" {
			stageName = "F级" // 默认第一阶段
		}

		success, statChanges, err := ese.World.Player.Breakthrough("gene_evolution", stageName)
		if success {
			step.Type = "breakthrough"
			step.Description = fmt.Sprintf("基因进化突破至 %s", stageName)
			step.StateChanges["breakthrough_system"] = "基因进化"
			step.StateChanges["breakthrough_stage"] = stageName
			step.StateChanges["stat_changes"] = statChanges

			ese.addLog("breakthrough", fmt.Sprintf("基因进化突破！当前阶段: %s", stageName), nil)
			if hp, ok := statChanges["hp"]; ok && hp > 0 {
				ese.addLog("breakthrough", fmt.Sprintf("  HP +%d", hp), nil)
			}
			if atk, ok := statChanges["attack"]; ok && atk > 0 {
				ese.addLog("breakthrough", fmt.Sprintf("  攻击 +%d", atk), nil)
			}
			if def, ok := statChanges["defense"]; ok && def > 0 {
				ese.addLog("breakthrough", fmt.Sprintf("  防御 +%d", def), nil)
			}
			return true
		}
		_ = err // 忽略错误（可能已经突破过了）
	}

	// 检测修仙突破
	if containsAnyKeyword(description, cultivationKeywords) || containsAnyKeyword(targetID, cultivationKeywords) {
		stageName := extractCultivationStage(description, targetID)
		if stageName == "" {
			stageName = "炼气期"
		}

		success, statChanges, err := ese.World.Player.Breakthrough("cultivation", stageName)
		if success {
			step.Type = "breakthrough"
			step.Description = fmt.Sprintf("修仙突破至 %s", stageName)
			step.StateChanges["breakthrough_system"] = "修仙"
			step.StateChanges["breakthrough_stage"] = stageName
			step.StateChanges["stat_changes"] = statChanges

			ese.addLog("breakthrough", fmt.Sprintf("修仙突破！当前境界: %s", stageName), nil)
			return true
		}
		_ = err
	}

	// 检测超能力觉醒
	if containsAnyKeyword(description, superpowerKeywords) || containsAnyKeyword(targetID, superpowerKeywords) {
		stageName := extractSuperpowerStage(description, targetID)
		if stageName == "" {
			stageName = "E级"
		}

		success, statChanges, err := ese.World.Player.Breakthrough("superpower", stageName)
		if success {
			step.Type = "breakthrough"
			step.Description = fmt.Sprintf("超能力觉醒至 %s", stageName)
			step.StateChanges["breakthrough_system"] = "超能力"
			step.StateChanges["breakthrough_stage"] = stageName
			step.StateChanges["stat_changes"] = statChanges

			ese.addLog("breakthrough", fmt.Sprintf("超能力觉醒！当前等级: %s", stageName), nil)
			return true
		}
		_ = err
	}

	// 检测武道突破
	if containsAnyKeyword(description, martialKeywords) || containsAnyKeyword(targetID, martialKeywords) {
		stageName := extractMartialStage(description, targetID)
		if stageName == "" {
			stageName = "淬体境"
		}

		success, statChanges, err := ese.World.Player.Breakthrough("martial_arts", stageName)
		if success {
			step.Type = "breakthrough"
			step.Description = fmt.Sprintf("武道突破至 %s", stageName)
			step.StateChanges["breakthrough_system"] = "武道"
			step.StateChanges["breakthrough_stage"] = stageName
			step.StateChanges["stat_changes"] = statChanges

			ese.addLog("breakthrough", fmt.Sprintf("武道突破！当前境界: %s", stageName), nil)
			return true
		}
		_ = err
	}

	return false
}

// containsAnyKeyword 检查是否包含任意关键词
func containsAnyKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// extractGeneStage 提取基因进化阶段
func extractGeneStage(description, subject string) string {
	text := description + " " + subject

	// 按优先级匹配
	stages := []string{"S级", "A级", "B级", "C级", "D级", "E级", "F级"}
	for _, stage := range stages {
		if strings.Contains(text, stage) {
			return stage
		}
	}
	return ""
}

// extractCultivationStage 提取修仙境界
func extractCultivationStage(description, subject string) string {
	text := description + " " + subject

	stages := []string{"渡劫期", "大乘期", "炼虚期", "化神期", "元婴期", "金丹期", "筑基期", "炼气期"}
	for _, stage := range stages {
		if strings.Contains(text, stage) {
			return stage
		}
	}
	return ""
}

// extractSuperpowerStage 提取超能力等级
func extractSuperpowerStage(description, subject string) string {
	text := description + " " + subject

	stages := []string{"SSS级", "S级", "A级", "B级", "C级", "D级", "E级"}
	for _, stage := range stages {
		if strings.Contains(text, stage) {
			return stage
		}
	}
	return ""
}

// extractMartialStage 提取武道境界
func extractMartialStage(description, subject string) string {
	text := description + " " + subject

	stages := []string{"武神境", "大宗师", "宗师境", "先天境", "凝气境", "开脉境", "淬体境"}
	for _, stage := range stages {
		if strings.Contains(text, stage) {
			return stage
		}
	}
	return ""
}

// simulateEventObjectiveEnhanced 增强版事件目标
func (ese *EnhancedSimulationEngine) simulateEventObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
	step.Characters = append(step.Characters, ese.World.Player.ID)
	step.Location = ese.World.Player.Position.MapID

	// 检测是否是突破事件
	breakthroughDetected := ese.checkAndTriggerBreakthrough(step, objective)
	if breakthroughDetected {
		// 突破事件已处理，直接返回
		return step
	}

	event := ese.World.Events.GetEvent(objective.TargetID)
	if event != nil {
		result := ese.World.Events.TriggerEvent(event.ID, 0)

		step.StateChanges["event_triggered"] = event.ID
		step.Description = fmt.Sprintf("触发了事件: %s", event.Name)

		if result != nil {
			step.StateChanges["commands_executed"] = len(result.Commands)
		}
		ese.addLog("event", fmt.Sprintf("触发事件: %s", event.Name), nil)
	}

	// 事件可能带来随机效果
	if ese.Random.Intn(100) < 25 {
		// 25%概率获得随机增益
		effectType := ese.Random.Intn(4)
		switch effectType {
		case 0:
			// 发现隐藏资源
			bonusExp := 5 + ese.Random.Intn(15)
			_, _ = ese.World.Player.GainExp(bonusExp)
			ese.TotalExpGained += bonusExp
			step.StateChanges["bonus_exp"] = bonusExp
			ese.addLog("reward", fmt.Sprintf("发现隐藏资源，获得经验 +%d", bonusExp), nil)
		case 1:
			// 环境恢复
			hpRestore := 5 + ese.Random.Intn(15)
			ese.World.Player.CurrentStats.HP = min(ese.World.Player.BaseStats.HP, ese.World.Player.CurrentStats.HP+hpRestore)
			step.StateChanges["hp_restored"] = hpRestore
			ese.TotalHPHealed += hpRestore
			ese.addLog("event", fmt.Sprintf("环境中的能量恢复了 HP+%d", hpRestore), nil)
		case 2:
			// 领悟技巧
			defBonus := 1 + ese.Random.Intn(2)
			ese.World.Player.BaseStats.Defense += defBonus
			step.StateChanges["defense_bonus"] = defBonus
			ese.addLog("reward", fmt.Sprintf("领悟了防御技巧，防御 +%d", defBonus), nil)
		case 3:
			// 获得状态效果
			step.StateChanges["status_effect"] = "inspired"
			ese.addLog("event", "获得了'灵感'状态，下次行动经验+50%", nil)
		}
	}

	// 添加经验值奖励
	expReward := 10 + ese.Random.Intn(20)
	leveledUp, _ := ese.World.Player.GainExp(expReward)
	ese.TotalExpGained += expReward
	step.StateChanges["exp_gained"] = expReward
	if leveledUp {
		step.StateChanges["level_up"] = ese.World.Player.Level
		ese.addLog("reward", fmt.Sprintf("升级！等级 %d → %d", ese.World.Player.Level-1, ese.World.Player.Level), nil)
	}
	step.StateChanges["player_level"] = ese.World.Player.Level
	step.StateChanges["player_exp"] = ese.World.Player.Exp

	// 更新玩家当前属性
	step.StateChanges["player_hp"] = ese.World.Player.BaseStats.HP
	step.StateChanges["player_mp"] = ese.World.Player.BaseStats.MP
	step.StateChanges["player_attack"] = ese.World.Player.BaseStats.Attack
	step.StateChanges["player_defense"] = ese.World.Player.BaseStats.Defense

	return step
}

// SimulateMultipleChapters 连续模拟多个章节
func (ese *EnhancedSimulationEngine) SimulateMultipleChapters(chapterIDs []string) *StorySimulationResult {
	result := &StorySimulationResult{
		StartTime:  time.Now().Unix(),
		Chapters:   make([]ChapterSimulationResult, 0),
		FinalState: make(map[string]interface{}),
	}

	for _, chapterID := range chapterIDs {
		enhancedResult, err := ese.SimulateChapterEnhanced(chapterID)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.Chapters = append(result.Chapters, ChapterSimulationResult{
			ChapterID:   enhancedResult.ChapterID,
			ChapterName: enhancedResult.ChapterName,
			Success:     enhancedResult.Success,
			StepCount:   len(enhancedResult.Steps),
		})
	}

	result.EndTime = time.Now().Unix()
	result.TotalSteps = len(ese.History)

	// 记录最终状态
	if ese.World.Player != nil {
		result.FinalState["player_level"] = ese.World.Player.Level
		result.FinalState["player_hp"] = ese.World.Player.CurrentStats.HP
		result.FinalState["player_exp"] = ese.World.Player.Exp
		result.FinalState["total_exp_gained"] = ese.TotalExpGained
	}

	return result
}

// ExportEnhancedReport 导出增强报告到文件
func (ese *EnhancedSimulationEngine) ExportEnhancedReport(result *EnhancedSimulationResult, outputPath string) error {
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(outputPath, jsonData, 0644)
}

// GenerateTextReport 生成文本报告
func (ese *EnhancedSimulationEngine) GenerateTextReport(result *EnhancedSimulationResult) string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    RPG 章节模拟报告                            ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n\n")

	// 章节信息
	sb.WriteString(fmt.Sprintf("📖 章节: %s (%s)\n", result.ChapterName, result.ChapterID))
	sb.WriteString(fmt.Sprintf("✅ 状态: %s\n", func() string {
		if result.Success {
			return "成功"
		}
		return "失败"
	}()))

	// 当前位置
	if ese.World.Player != nil && ese.World.Player.Position.MapID != "" {
		currentMap := ese.World.Maps.GetMap(ese.World.Player.Position.MapID)
		if currentMap != nil {
			sb.WriteString(fmt.Sprintf("📍 当前位置: %s\n", currentMap.Name))
		}
	}
	sb.WriteString("\n")

	// 玩家状态变化
	sb.WriteString("┌─────────────────────────────────────────┐\n")
	sb.WriteString("│           玩家状态变化                   │\n")
	sb.WriteString("├─────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ 等级  %d → %d                           │\n",
		result.PlayerStats.Initial.Level, result.PlayerStats.Final.Level))
	sb.WriteString(fmt.Sprintf("│ HP    %d/%d → %d/%d                   │\n",
		result.PlayerStats.Initial.HP, result.PlayerStats.Initial.MaxHP,
		result.PlayerStats.Final.HP, result.PlayerStats.Final.MaxHP))
	sb.WriteString(fmt.Sprintf("│ MP    %d/%d → %d/%d                   │\n",
		result.PlayerStats.Initial.MP, result.PlayerStats.Initial.MaxMP,
		result.PlayerStats.Final.MP, result.PlayerStats.Final.MaxMP))
	sb.WriteString(fmt.Sprintf("│ 攻击  %d → %d                          │\n",
		result.PlayerStats.Initial.Attack, result.PlayerStats.Final.Attack))
	sb.WriteString(fmt.Sprintf("│ 防御  %d → %d                          │\n",
		result.PlayerStats.Initial.Defense, result.PlayerStats.Final.Defense))
	sb.WriteString(fmt.Sprintf("│ 经验  %d → %d (+%d)                   │\n",
		result.PlayerStats.Initial.Exp, result.PlayerStats.Final.Exp,
		ese.TotalExpGained))
	sb.WriteString("└─────────────────────────────────────────┘\n\n")

	// 模拟摘要统计
	if ese.TotalDamageDealt > 0 || ese.TotalDamageTaken > 0 || ese.TotalHPHealed > 0 ||
		ese.TotalMPHealed > 0 || ese.CritCount > 0 || ese.DodgeCount > 0 {
		sb.WriteString("┌─────────────────────────────────────────┐\n")
		sb.WriteString("│              模拟摘要                    │\n")
		sb.WriteString("├─────────────────────────────────────────┤\n")
		if ese.TotalDamageDealt > 0 {
			sb.WriteString(fmt.Sprintf("│ 总伤害输出: %-30d │\n", ese.TotalDamageDealt))
		}
		if ese.TotalDamageTaken > 0 {
			sb.WriteString(fmt.Sprintf("│ 总承受伤害: %-30d │\n", ese.TotalDamageTaken))
		}
		if ese.TotalHPHealed > 0 {
			sb.WriteString(fmt.Sprintf("│ 总HP恢复:   %-30d │\n", ese.TotalHPHealed))
		}
		if ese.TotalMPHealed > 0 {
			sb.WriteString(fmt.Sprintf("│ 总MP恢复:   %-30d │\n", ese.TotalMPHealed))
		}
		if ese.CritCount > 0 {
			sb.WriteString(fmt.Sprintf("│ 暴击次数:   %-30d │\n", ese.CritCount))
		}
		if ese.DodgeCount > 0 {
			sb.WriteString(fmt.Sprintf("│ 闪避次数:   %-30d │\n", ese.DodgeCount))
		}
		sb.WriteString("└─────────────────────────────────────────┘\n\n")
	}

	// 战斗报告
	if len(result.BattleReports) > 0 {
		sb.WriteString("┌─────────────────────────────────────────┐\n")
		sb.WriteString("│              战斗报告                    │\n")
		sb.WriteString("└─────────────────────────────────────────┘\n")
		for i, battle := range result.BattleReports {
			status := "❌ 失败"
			if battle.Victory {
				status = "✅ 胜利"
			}
			sb.WriteString(fmt.Sprintf("\n  ⚔️ 战斗 %d: vs %s %s\n", i+1, battle.EnemyName, status))
			sb.WriteString(fmt.Sprintf("     敌人等级: %d | 回合数: %d\n", battle.EnemyLevel, battle.Turns))
			sb.WriteString(fmt.Sprintf("     造成伤害: %d | 承受伤害: %d\n", battle.DamageDealt, battle.DamageTaken))

			// 显示战斗回合详情
			if len(battle.TurnLog) > 0 {
				sb.WriteString("\n     战斗过程:\n")
				maxTurns := len(battle.TurnLog)
				if maxTurns > 10 {
					maxTurns = 10
					sb.WriteString("     (仅显示前10回合)\n")
				}
				for j := 0; j < maxTurns; j++ {
					turn := battle.TurnLog[j]
					icon := "⚔️"
					if turn.Action == "use_skill" {
						icon = "✨"
					}
					if turn.Damage > 0 {
						sb.WriteString(fmt.Sprintf("       %s [T%d] %s → %s: %d伤害\n",
							icon, turn.TurnNumber, turn.Actor, turn.Target, turn.Damage))
					} else {
						sb.WriteString(fmt.Sprintf("       💨 [T%d] %s 攻击 %s: 闪避!\n",
							turn.TurnNumber, turn.Actor, turn.Target))
					}
					// 显示HP状态
					if turn.Actor == ese.World.Player.Name {
						sb.WriteString(fmt.Sprintf("          玩家HP: %d | 敌人HP: %d\n",
							turn.PlayerHP, turn.EnemyHP))
					}
				}
			}
		}
		sb.WriteString("\n")
	}

	// 奖励
	if len(result.Rewards) > 0 {
		sb.WriteString("┌─────────────────────────────────────────┐\n")
		sb.WriteString("│              获得奖励                    │\n")
		sb.WriteString("└─────────────────────────────────────────┘\n")
		for _, reward := range result.Rewards {
			if reward.Type == "exp" {
				sb.WriteString(fmt.Sprintf("  ✨ 经验: +%d\n", reward.Amount))
			} else if reward.Type == "item" {
				sb.WriteString(fmt.Sprintf("  📦 %s\n", reward.ItemName))
			}
		}
		sb.WriteString("\n")
	}

	// 详细步骤
	sb.WriteString("┌─────────────────────────────────────────┐\n")
	sb.WriteString("│              任务步骤                    │\n")
	sb.WriteString("└─────────────────────────────────────────┘\n")
	for i, step := range result.Steps {
		typeIcon := "📝"
		switch step.Type {
		case "kill":
			typeIcon = "⚔️"
		case "collect":
			typeIcon = "📦"
		case "talk":
			typeIcon = "💬"
		case "reach":
			typeIcon = "🗺️"
		default:
			typeIcon = "✨"
		}

		sb.WriteString(fmt.Sprintf("\n  %s 步骤 %d: %s\n", typeIcon, i+1, step.Description))

		// 显示位置信息
		if step.Location != "" {
			locationName := step.Location
			gameMap := ese.World.Maps.GetMap(step.Location)
			if gameMap != nil {
				locationName = gameMap.Name
			}
			sb.WriteString(fmt.Sprintf("     📍 位置: %s\n", locationName))
		}

		// 显示参与角色
		if len(step.Characters) > 0 {
			charNames := make([]string, 0, len(step.Characters))
			for _, charID := range step.Characters {
				char := ese.World.Characters.GetCharacter(charID)
				if char != nil {
					charNames = append(charNames, char.Name)
				}
			}
			if len(charNames) > 0 {
				sb.WriteString(fmt.Sprintf("     👥 角色: %s\n", strings.Join(charNames, ", ")))
			}
		}

		// 显示经验获取
		if expGained, ok := step.StateChanges["exp_gained"].(int); ok && expGained > 0 {
			sb.WriteString(fmt.Sprintf("     ✨ 经验: +%d\n", expGained))
		}

		// 显示等级提升
		if levelUp, ok := step.StateChanges["level_up"].(int); ok {
			sb.WriteString(fmt.Sprintf("     🎉 升级! 等级 %d\n", levelUp))
		}
	}

	sb.WriteString("\n╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    模拟完成                                  ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n")

	return sb.String()
}

// GetChaptersInVolume 获取指定卷下的所有章节ID
func (ese *EnhancedSimulationEngine) GetChaptersInVolume(volumeID string) ([]string, error) {
	// 解析卷ID，例如 P1-V1
	parts := strings.Split(volumeID, "-")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "P") || !strings.HasPrefix(parts[1], "V") {
		return nil, fmt.Errorf("invalid volume ID format: %s", volumeID)
	}

	// 遍历所有任务，找到属于该卷的章节任务
	allQuests := ese.World.Quests.GetAllQuests()
	chapterIDs := make([]string, 0)

	for _, q := range allQuests {
		// 章节任务ID格式: quest_P1-V1-C1
		questID := strings.TrimPrefix(q.ID, "quest_")
		// 检查是否属于该卷 (P1-V1-C*)
		if strings.HasPrefix(questID, volumeID+"-C") {
			chapterIDs = append(chapterIDs, questID)
		}
	}

	// 按章节编号排序（提取C后面的数字进行数值排序）
	sort.Slice(chapterIDs, func(i, j int) bool {
		numI := extractChapterNumber(chapterIDs[i])
		numJ := extractChapterNumber(chapterIDs[j])
		return numI < numJ
	})

	return chapterIDs, nil
}

// extractChapterNumber 从章节ID中提取章节编号
func extractChapterNumber(chapterID string) int {
	// 格式: P1-V1-C10
	parts := strings.Split(chapterID, "-C")
	if len(parts) != 2 {
		return 0
	}
	num := 0
	fmt.Sscanf(parts[1], "%d", &num)
	return num
}

// VolumeSimulationReport 卷模拟报告
type VolumeSimulationReport struct {
	VolumeID     string           `json:"volume_id"`
	ChapterIDs   []string         `json:"chapter_ids"`
	TotalSteps   int              `json:"total_steps"`
	TotalBattles int              `json:"total_battles"`
	TotalExp     int              `json:"total_exp"`
	FinalLevel   int              `json:"final_level"`
	Chapters     []ChapterSummary `json:"chapters"`
}

// ChapterSummary 章节摘要
type ChapterSummary struct {
	ChapterID   string `json:"chapter_id"`
	ChapterName string `json:"chapter_name"`
	Steps       int    `json:"steps"`
	Battles     int    `json:"battles"`
	ExpGained   int    `json:"exp_gained"`
	Success     bool   `json:"success"`
}

// ExportVolumeReport 导出卷模拟报告
func (ese *EnhancedSimulationEngine) ExportVolumeReport(report *VolumeSimulationReport, outputPath string) error {
	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, jsonData, 0644)
}
