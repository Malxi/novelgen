package rpg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnhancedSimulationResult 增强版模拟结果
type EnhancedSimulationResult struct {
	SimulationResult
	PlayerStats    PlayerStatsLog    `json:"player_stats"`
	BattleReports  []BattleReport    `json:"battle_reports,omitempty"`
	Rewards        []RewardLog       `json:"rewards"`
	FullLog        []LogEntry        `json:"full_log"`
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
	Log            []LogEntry
	CurrentBattle  *BattleReport
	TotalExpGained int
	ItemsCollected []string
}

// NewEnhancedSimulationEngine 创建增强版推演引擎
func NewEnhancedSimulationEngine(world *GameWorld) *EnhancedSimulationEngine {
	return &EnhancedSimulationEngine{
		SimulationEngine: NewSimulationEngine(world),
		Log:              make([]LogEntry, 0),
		ItemsCollected:   make([]string, 0),
	}
}

// SimulateChapterEnhanced 增强版章节推演
func (ese *EnhancedSimulationEngine) SimulateChapterEnhanced(chapterID string) (*EnhancedSimulationResult, error) {
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
		result.FullLog = append(result.FullLog, ese.Log...)

		// 如果是战斗，添加战斗报告
		if ese.CurrentBattle != nil && step.Type == "kill" {
			result.BattleReports = append(result.BattleReports, *ese.CurrentBattle)
			ese.CurrentBattle = nil
		}
	}

	// 完成任务并获取奖励
	turnInSuccess := ese.World.TurnInQuest(quest.ID)
	if turnInSuccess {
		// 获取任务奖励（从任务定义中获取）
		rewards := ese.World.Quests.TurnInQuest(quest.ID, ese.World.Player)
		if rewards != nil {
			// 记录经验奖励
			if rewards.Exp > 0 {
				result.Rewards = append(result.Rewards, RewardLog{
					Type:   "exp",
					Amount: rewards.Exp,
					Reason: "完成任务",
				})
				ese.TotalExpGained += rewards.Exp

				// 应用经验
				oldLevel := ese.World.Player.Level
				for i := 0; i < rewards.Exp; i++ {
					if ese.World.Player.GainExp(1) {
						// 升级了
						if ese.World.Player.Level > oldLevel {
							ese.addLog("reward", fmt.Sprintf("升级！等级 %d → %d",
								oldLevel, ese.World.Player.Level), nil)
						}
					}
				}
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
	}

	result.EndTime = time.Now().Unix()
	result.Success = true
	result.PlayerStats.Final = ese.recordPlayerStats()

	// 添加完成日志
	ese.addLog("info", fmt.Sprintf("章节模拟完成: %s", quest.Name), map[string]interface{}{
		"success":     true,
		"exp_gained":  ese.TotalExpGained,
		"steps_count": len(result.Steps),
	})

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

	switch objective.Type {
	case ObjectiveKill:
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
		Attack:  ese.World.Player.CurrentStats.Attack,
		Defense: ese.World.Player.CurrentStats.Defense,
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
		"player_hp":   ese.World.Player.CurrentStats.HP,
		"enemy_hp":    enemy.CurrentStats.HP,
		"player_atk":  ese.World.Player.CurrentStats.Attack,
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
		step.StateChanges["exp_gained"] = expGained
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
	item := ese.World.Items.GetItem(objective.TargetID)
	if item != nil {
		se := ese.World.AddItemToPlayer(item.ID, objective.TargetCount)
		step.StateChanges["item_collected"] = item.ID
		step.StateChanges["count"] = objective.TargetCount
		step.Description = fmt.Sprintf("收集了 %s x%d", item.Name, objective.TargetCount)
		ese.addLog("reward", fmt.Sprintf("获得物品: %s x%d", item.Name, objective.TargetCount), nil)
		ese.ItemsCollected = append(ese.ItemsCollected, item.Name)
		_ = se
	}
	return step
}

// simulateTalkObjectiveEnhanced 增强版对话目标
func (ese *EnhancedSimulationEngine) simulateTalkObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
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

		step.StateChanges["npc_talked"] = npc.ID
		step.Description = fmt.Sprintf("与 %s 进行了对话", npc.Name)
		ese.addLog("event", fmt.Sprintf("与 %s 对话", npc.Name), nil)
	}
	return step
}

// simulateMoveObjectiveEnhanced 增强版移动目标
func (ese *EnhancedSimulationEngine) simulateMoveObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
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
		step.StateChanges["location_changed"] = objective.TargetID
		step.Description = fmt.Sprintf("移动到了 %s", gameMap.Name)
		ese.addLog("event", fmt.Sprintf("移动: %s → %s", oldLocation, gameMap.Name), nil)
	}
	return step
}

// simulateEventObjectiveEnhanced 增强版事件目标
func (ese *EnhancedSimulationEngine) simulateEventObjectiveEnhanced(step SimulationStep, objective QuestObjective) SimulationStep {
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
	return step
}

// SimulateMultipleChapters 连续模拟多个章节
func (ese *EnhancedSimulationEngine) SimulateMultipleChapters(chapterIDs []string) *StorySimulationResult {
	result := &StorySimulationResult{
		StartTime: time.Now().Unix(),
		Chapters:  make([]ChapterSimulationResult, 0),
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
			sb.WriteString(fmt.Sprintf("\n  战斗 %d: %s %s\n", i+1, battle.EnemyName, status))
			sb.WriteString(fmt.Sprintf("  敌人等级: %d | 回合数: %d\n", battle.EnemyLevel, battle.Turns))
			sb.WriteString(fmt.Sprintf("  造成伤害: %d | 承受伤害: %d\n", battle.DamageDealt, battle.DamageTaken))

			// 显示战斗回合详情
			if len(battle.TurnLog) > 0 && len(battle.TurnLog) <= 5 {
				sb.WriteString("\n  战斗过程:\n")
				for _, turn := range battle.TurnLog {
					if turn.Damage > 0 {
						sb.WriteString(fmt.Sprintf("    [T%d] %s → %s: %d伤害\n",
							turn.TurnNumber, turn.Actor, turn.Target, turn.Damage))
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
	}

	sb.WriteString("\n╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    模拟完成                                  ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n")

	return sb.String()
}