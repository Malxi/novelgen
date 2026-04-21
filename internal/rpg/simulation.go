package rpg

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// SimulationEngine 剧情推演引擎
type SimulationEngine struct {
	World       *GameWorld
	History     []SimulationStep
	CurrentStep int
	Random      *rand.Rand

	// 推演配置
	Config SimulationConfig
}

// SimulationConfig 推演配置
type SimulationConfig struct {
	AutoResolveCombat bool    // 自动战斗
	CombatSpeed       float64 // 战斗速度
	EventDelay        int     // 事件延迟（毫秒）
	LogLevel          string  // 日志级别
}

// SimulationStep 推演步骤
type SimulationStep struct {
	StepNumber   int                    `json:"step_number"`
	Type         string                 `json:"type"` // event, combat, dialogue, quest
	Description  string                 `json:"description"`
	Characters   []string               `json:"characters"`
	Location     string                 `json:"location"`
	Actions      []Action               `json:"actions"`
	Results      []ActionResult         `json:"results"`
	StateChanges map[string]interface{} `json:"state_changes"`
	Timestamp    int64                  `json:"timestamp"`
}

// Action 行动
type Action struct {
	Actor      string      `json:"actor"`
	ActionType string      `json:"action_type"` // attack, use_skill, use_item, move, talk
	Target     string      `json:"target,omitempty"`
	SkillID    string      `json:"skill_id,omitempty"`
	ItemID     string      `json:"item_id,omitempty"`
	Parameters interface{} `json:"parameters,omitempty"`
}

// ActionResult 行动结果
type ActionResult struct {
	Success bool     `json:"success"`
	Action  Action   `json:"action"`
	Effects []Effect `json:"effects"`
	Damage  int      `json:"damage,omitempty"`
	Message string   `json:"message"`
}

// StoryScenario 剧情场景
type StoryScenario struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Characters  []string    `json:"characters"` // 参与角色ID
	Location    string      `json:"location"`   // 场景地点
	Events      []string    `json:"events"`     // 触发的事件ID
	Quests      []string    `json:"quests"`     // 相关任务ID
	Conditions  []Condition `json:"conditions"` // 触发条件
}

// NewSimulationEngine 创建推演引擎
func NewSimulationEngine(world *GameWorld) *SimulationEngine {
	return &SimulationEngine{
		World:   world,
		History: make([]SimulationStep, 0),
		Random:  rand.New(rand.NewSource(time.Now().UnixNano())),
		Config: SimulationConfig{
			AutoResolveCombat: true,
			CombatSpeed:       1.0,
			EventDelay:        0,
			LogLevel:          "info",
		},
	}
}

// SimulateChapter 推演单个章节
func (se *SimulationEngine) SimulateChapter(chapterID string) (*SimulationResult, error) {
	// 查找对应的任务
	quest := se.World.Quests.GetQuest("quest_" + chapterID)
	if quest == nil {
		return nil, fmt.Errorf("章节 %s 对应的任务不存在", chapterID)
	}

	result := &SimulationResult{
		ChapterID:   chapterID,
		ChapterName: quest.Name,
		Steps:       make([]SimulationStep, 0),
		StartTime:   time.Now().Unix(),
	}

	// 接取任务
	if !se.World.AcceptQuest(quest.ID) {
		return nil, fmt.Errorf("无法接取任务 %s", quest.ID)
	}

	// 推演每个目标
	for _, objective := range quest.Objectives {
		step := se.SimulateObjective(objective)
		result.Steps = append(result.Steps, step)
		se.History = append(se.History, step)
	}

	// 完成任务
	se.World.TurnInQuest(quest.ID)

	result.EndTime = time.Now().Unix()
	result.Success = true

	return result, nil
}

// SimulateObjective 推演单个目标（公开方法）
func (se *SimulationEngine) SimulateObjective(objective QuestObjective) SimulationStep {
	step := SimulationStep{
		StepNumber:   len(se.History) + 1,
		Type:         string(objective.Type),
		Description:  objective.Description,
		Timestamp:    time.Now().Unix(),
		StateChanges: make(map[string]interface{}),
	}

	switch objective.Type {
	case ObjectiveKill:
		step = se.simulateCombatObjective(step, objective)
	case ObjectiveCollect:
		step = se.simulateCollectObjective(step, objective)
	case ObjectiveTalk:
		step = se.simulateTalkObjective(step, objective)
	case ObjectiveReach:
		step = se.simulateMoveObjective(step, objective)
	default:
		step = se.simulateEventObjective(step, objective)
	}

	return step
}

// simulateCombatObjective 推演战斗目标
func (se *SimulationEngine) simulateCombatObjective(step SimulationStep, objective QuestObjective) SimulationStep {
	// 获取目标敌人
	enemy := se.World.Characters.GetCharacter(objective.TargetID)
	if enemy == nil {
		step.Results = append(step.Results, ActionResult{
			Success: false,
			Message: "敌人不存在",
		})
		return step
	}

	step.Characters = []string{se.World.Player.ID, enemy.ID}
	step.Location = se.World.Player.Position.MapID

	// 创建战斗场景
	battle := &BattleScene{
		Enemies:  []*Character{enemy},
		Allies:   []*Character{se.World.Player},
		IsActive: true,
	}

	turnCount := 0
	maxTurns := 20

	// 战斗循环
	for battle.IsActive && turnCount < maxTurns {
		turnCount++

		// 玩家回合
		if se.World.Player.State != CharacterStateDead {
			action := se.decidePlayerAction(enemy)
			actionResult := se.executeAction(action, battle)
			step.Actions = append(step.Actions, action)
			step.Results = append(step.Results, actionResult)
		}

		// 检查战斗结束
		battle.CheckBattleEnd()
		if !battle.IsActive {
			break
		}

		// 敌人回合
		if enemy.State != CharacterStateDead {
			action := se.decideEnemyAction(enemy, se.World.Player)
			actionResult := se.executeAction(action, battle)
			step.Actions = append(step.Actions, action)
			step.Results = append(step.Results, actionResult)
		}

		battle.CheckBattleEnd()
	}

	// 记录战斗结果
	if battle.Winner == "player" {
		expReward := enemy.Level * 20
		leveledUp, _ := se.World.Player.GainExp(expReward)
		step.StateChanges["enemy_defeated"] = enemy.ID
		step.StateChanges["exp_gained"] = expReward
		if leveledUp {
			step.StateChanges["level_up"] = se.World.Player.Level
		}
		step.StateChanges["player_level"] = se.World.Player.Level
		step.StateChanges["player_exp"] = se.World.Player.Exp
		step.Description = fmt.Sprintf("战斗胜利！击败了 %s", enemy.Name)
	} else {
		step.Description = fmt.Sprintf("战斗失败，被 %s 击败", enemy.Name)
	}

	return step
}

// simulateCollectObjective 推演收集目标
func (se *SimulationEngine) simulateCollectObjective(step SimulationStep, objective QuestObjective) SimulationStep {
	// 添加玩家到角色列表
	step.Characters = append(step.Characters, se.World.Player.ID)
	step.Location = se.World.Player.Position.MapID

	// 模拟收集物品
	item := se.World.Items.GetItem(objective.TargetID)
	if item != nil {
		se.World.AddItemToPlayer(item.ID, objective.TargetCount)
		step.StateChanges["item_collected"] = item.ID
		step.StateChanges["count"] = objective.TargetCount
		step.Description = fmt.Sprintf("收集了 %s x%d", item.Name, objective.TargetCount)
	}

	// 添加经验值奖励
	expReward := 5 + se.Random.Intn(15)
	leveledUp, _ := se.World.Player.GainExp(expReward)
	step.StateChanges["exp_gained"] = expReward
	if leveledUp {
		step.StateChanges["level_up"] = se.World.Player.Level
	}
	step.StateChanges["player_level"] = se.World.Player.Level
	step.StateChanges["player_exp"] = se.World.Player.Exp

	return step
}

// simulateTalkObjective 推演对话目标
func (se *SimulationEngine) simulateTalkObjective(step SimulationStep, objective QuestObjective) SimulationStep {
	// 添加玩家到角色列表
	step.Characters = append(step.Characters, se.World.Player.ID)
	step.Location = se.World.Player.Position.MapID

	// 获取NPC
	npc := se.World.Characters.GetCharacter(objective.TargetID)
	if npc != nil {
		action := Action{
			Actor:      se.World.Player.ID,
			ActionType: "talk",
			Target:     npc.ID,
		}

		// 触发NPC的对话事件
		eventResult := se.World.TalkToNPC(npc.ID)

		step.Actions = append(step.Actions, action)
		step.Results = append(step.Results, ActionResult{
			Success: eventResult != nil,
			Action:  action,
			Message: fmt.Sprintf("与 %s 对话", npc.Name),
		})

		step.Characters = append(step.Characters, npc.ID)
		step.StateChanges["npc_talked"] = npc.ID
		step.Description = fmt.Sprintf("与 %s 进行了对话", npc.Name)
	}

	// 添加经验值奖励
	expReward := 5 + se.Random.Intn(10)
	leveledUp, _ := se.World.Player.GainExp(expReward)
	step.StateChanges["exp_gained"] = expReward
	if leveledUp {
		step.StateChanges["level_up"] = se.World.Player.Level
	}
	step.StateChanges["player_level"] = se.World.Player.Level
	step.StateChanges["player_exp"] = se.World.Player.Exp

	return step
}

// simulateMoveObjective 推演移动目标
func (se *SimulationEngine) simulateMoveObjective(step SimulationStep, objective QuestObjective) SimulationStep {
	// 添加玩家到角色列表
	step.Characters = append(step.Characters, se.World.Player.ID)

	// 移动到目标地点
	gameMap := se.World.Maps.GetMap(objective.TargetID)
	if gameMap != nil {
		oldLocation := se.World.Player.Position.MapID
		se.World.MovePlayerTo(objective.TargetID, 10, 10)

		action := Action{
			Actor:      se.World.Player.ID,
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
	}

	// 添加经验值奖励
	expReward := 3 + se.Random.Intn(8)
	leveledUp, _ := se.World.Player.GainExp(expReward)
	step.StateChanges["exp_gained"] = expReward
	if leveledUp {
		step.StateChanges["level_up"] = se.World.Player.Level
	}
	step.StateChanges["player_level"] = se.World.Player.Level
	step.StateChanges["player_exp"] = se.World.Player.Exp

	return step
}

// simulateEventObjective 推演事件目标
func (se *SimulationEngine) simulateEventObjective(step SimulationStep, objective QuestObjective) SimulationStep {
	// 添加玩家到角色列表
	step.Characters = append(step.Characters, se.World.Player.ID)

	// 添加当前位置信息
	step.Location = se.World.Player.Position.MapID

	// 触发事件
	event := se.World.Events.GetEvent(objective.TargetID)
	if event != nil {
		result := se.World.Events.TriggerEvent(event.ID, 0)

		step.StateChanges["event_triggered"] = event.ID

		if result != nil {
			step.StateChanges["commands_executed"] = len(result.Commands)
		}
	}

	// 如果 objective 有描述信息，使用它
	if objective.Description != "" {
		step.Description = objective.Description
	}

	// 如果有目标ID，尝试获取角色信息
	if objective.TargetID != "" {
		char := se.World.Characters.GetCharacter(objective.TargetID)
		if char != nil {
			step.Characters = append(step.Characters, char.ID)
		}
	}

	// 添加经验值奖励
	expReward := 10 + se.Random.Intn(20)
	leveledUp, _ := se.World.Player.GainExp(expReward)
	step.StateChanges["exp_gained"] = expReward

	// 检查是否升级
	if leveledUp {
		step.StateChanges["level_up"] = se.World.Player.Level
	}

	// 记录状态变化
	step.StateChanges["player_hp"] = se.World.Player.BaseStats.HP
	step.StateChanges["player_mp"] = se.World.Player.BaseStats.MP
	step.StateChanges["player_level"] = se.World.Player.Level
	step.StateChanges["player_exp"] = se.World.Player.Exp

	return step
}

// decidePlayerAction 决定玩家行动
func (se *SimulationEngine) decidePlayerAction(enemy *Character) Action {
	// 简单的AI：优先使用技能，其次普通攻击
	if len(se.World.Player.Skills) > 0 && se.Random.Float64() > 0.3 {
		// 筛选出可以使用的技能
		usableSkills := make([]string, 0)
		for _, skillID := range se.World.Player.Skills {
			canUse, _ := se.World.Skills.CanUseSkill(skillID, se.World.Player)
			if canUse {
				usableSkills = append(usableSkills, skillID)
			}
		}
		
		// 如果有可用技能，随机选择一个
		if len(usableSkills) > 0 {
			skillID := usableSkills[se.Random.Intn(len(usableSkills))]
			return Action{
				Actor:      se.World.Player.ID,
				ActionType: "use_skill",
				Target:     enemy.ID,
				SkillID:    skillID,
			}
		}
	}

	// 普通攻击
	return Action{
		Actor:      se.World.Player.ID,
		ActionType: "attack",
		Target:     enemy.ID,
	}
}

// decideEnemyAction 决定敌人行动
func (se *SimulationEngine) decideEnemyAction(enemy, player *Character) Action {
	// 敌人AI：80%概率攻击
	if se.Random.Float64() < 0.8 {
		return Action{
			Actor:      enemy.ID,
			ActionType: "attack",
			Target:     player.ID,
		}
	}

	// 20%概率使用技能（如果有）
	if len(enemy.Skills) > 0 {
		skillID := enemy.Skills[se.Random.Intn(len(enemy.Skills))]
		return Action{
			Actor:      enemy.ID,
			ActionType: "use_skill",
			Target:     player.ID,
			SkillID:    skillID,
		}
	}

	return Action{
		Actor:      enemy.ID,
		ActionType: "attack",
		Target:     player.ID,
	}
}

// executeAction 执行行动
func (se *SimulationEngine) executeAction(action Action, battle *BattleScene) ActionResult {
	result := ActionResult{
		Success: true,
		Action:  action,
	}

	switch action.ActionType {
	case "attack":
		result = se.executeAttack(action, battle)
	case "use_skill":
		result = se.executeSkill(action, battle)
	case "use_item":
		result = se.executeItem(action, battle)
	}

	return result
}

// executeAttack 执行普通攻击
func (se *SimulationEngine) executeAttack(action Action, battle *BattleScene) ActionResult {
	actor := se.World.Characters.GetCharacter(action.Actor)
	target := se.World.Characters.GetCharacter(action.Target)

	if actor == nil || target == nil {
		return ActionResult{
			Success: false,
			Action:  action,
			Message: "角色不存在",
		}
	}

	// 闪避判定（基于速度和幸运）
	dodgeChance := target.CurrentStats.Speed*2 + target.CurrentStats.Luck
	if se.Random.Intn(100) < dodgeChance {
		return ActionResult{
			Success: true,
			Action:  action,
			Damage:  0,
			Message: fmt.Sprintf("%s 攻击了 %s，但 %s 闪避了攻击", actor.Name, target.Name, target.Name),
		}
	}

	// 计算基础伤害
	damage := actor.CurrentStats.Attack - target.CurrentStats.Defense/2
	if damage < 1 {
		damage = 1
	}

	// 暴击判定（基于幸运）
	critChance := actor.CurrentStats.Luck
	isCrit := se.Random.Intn(100) < critChance
	if isCrit {
		damage = int(float64(damage) * 1.5)
	}

	// 应用伤害
	target.CurrentStats.HP -= damage
	if target.CurrentStats.HP <= 0 {
		target.CurrentStats.HP = 0
		target.State = CharacterStateDead
	}

	message := fmt.Sprintf("%s 攻击了 %s，造成 %d 点伤害", actor.Name, target.Name, damage)
	if isCrit {
		message = fmt.Sprintf("%s 暴击攻击了 %s，造成 %d 点伤害！", actor.Name, target.Name, damage)
	}

	return ActionResult{
		Success: true,
		Action:  action,
		Damage:  damage,
		Message: message,
	}
}

// executeSkill 执行技能
func (se *SimulationEngine) executeSkill(action Action, battle *BattleScene) ActionResult {
	if action.SkillID == "" {
		return ActionResult{
			Success: false,
			Action:  action,
			Message: "未指定技能",
		}
	}

	// 使用技能系统执行
	targets := []*Character{}
	if action.Target != "" {
		target := se.World.Characters.GetCharacter(action.Target)
		if target != nil {
			targets = append(targets, target)
		}
	}

	skillResult := se.World.UseSkill(action.SkillID, []string{action.Target})

	if skillResult == nil {
		return ActionResult{
			Success: false,
			Action:  action,
			Message: "技能使用失败",
		}
	}

	actor := se.World.Characters.GetCharacter(action.Actor)
	target := se.World.Characters.GetCharacter(action.Target)

	damage := 0
	if dmg, ok := skillResult.Damage[action.Target]; ok {
		damage = dmg
	}

	return ActionResult{
		Success: !skillResult.IsMiss,
		Action:  action,
		Damage:  damage,
		Effects: skillResult.Effects,
		Message: fmt.Sprintf("%s 对 %s 使用了 %s", actor.Name, target.Name, action.SkillID),
	}
}

// executeItem 执行物品使用
func (se *SimulationEngine) executeItem(action Action, battle *BattleScene) ActionResult {
	if action.ItemID == "" {
		return ActionResult{
			Success: false,
			Action:  action,
			Message: "未指定物品",
		}
	}

	effects := se.World.UseItem(action.ItemID, action.Target)

	return ActionResult{
		Success: effects != nil,
		Action:  action,
		Effects: effects,
		Message: fmt.Sprintf("使用了 %s", action.ItemID),
	}
}

// SimulateStory 推演整个故事
func (se *SimulationEngine) SimulateStory(questIDs []string) *StorySimulationResult {
	result := &StorySimulationResult{
		StartTime:  time.Now().Unix(),
		Chapters:   make([]ChapterSimulationResult, 0),
		FinalState: make(map[string]interface{}),
	}

	for _, questID := range questIDs {
		chapterResult, err := se.SimulateChapter(questID)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.Chapters = append(result.Chapters, ChapterSimulationResult{
			ChapterID:   chapterResult.ChapterID,
			ChapterName: chapterResult.ChapterName,
			Success:     chapterResult.Success,
			StepCount:   len(chapterResult.Steps),
		})
	}

	result.EndTime = time.Now().Unix()
	result.TotalSteps = len(se.History)

	// 记录最终状态
	if se.World.Player != nil {
		result.FinalState["player_level"] = se.World.Player.Level
		result.FinalState["player_hp"] = se.World.Player.CurrentStats.HP
		result.FinalState["player_exp"] = se.World.Player.Exp
	}

	return result
}

// GetSimulationReport 获取推演报告
func (se *SimulationEngine) GetSimulationReport() string {
	report := fmt.Sprintf("=== 剧情推演报告 ===\n\n")
	report += fmt.Sprintf("总步骤数: %d\n", len(se.History))
	report += fmt.Sprintf("当前玩家等级: %d\n", se.World.Player.Level)
	report += fmt.Sprintf("当前玩家HP: %d/%d\n", se.World.Player.CurrentStats.HP, se.World.Player.BaseStats.HP)
	report += fmt.Sprintf("\n详细步骤:\n")

	for _, step := range se.History {
		report += fmt.Sprintf("\n[%d] %s: %s\n", step.StepNumber, step.Type, step.Description)
		if len(step.Results) > 0 {
			for _, res := range step.Results {
				if res.Message != "" {
					report += fmt.Sprintf("  - %s\n", res.Message)
				}
			}
		}
	}

	return report
}

// ExportSimulation 导出推演数据
func (se *SimulationEngine) ExportSimulation() string {
	data := map[string]interface{}{
		"history":     se.History,
		"final_state": se.World.SaveToJSON(),
		"config":      se.Config,
	}

	jsonData, _ := json.MarshalIndent(data, "", "  ")
	return string(jsonData)
}

// SimulationResult 推演结果
type SimulationResult struct {
	ChapterID   string           `json:"chapter_id"`
	ChapterName string           `json:"chapter_name"`
	Steps       []SimulationStep `json:"steps"`
	Success     bool             `json:"success"`
	StartTime   int64            `json:"start_time"`
	EndTime     int64            `json:"end_time"`
}

// StorySimulationResult 故事推演结果
type StorySimulationResult struct {
	Chapters   []ChapterSimulationResult `json:"chapters"`
	TotalSteps int                       `json:"total_steps"`
	StartTime  int64                     `json:"start_time"`
	EndTime    int64                     `json:"end_time"`
	FinalState map[string]interface{}    `json:"final_state"`
	Errors     []string                  `json:"errors,omitempty"`
}

// ChapterSimulationResult 章节推演结果
type ChapterSimulationResult struct {
	ChapterID   string `json:"chapter_id"`
	ChapterName string `json:"chapter_name"`
	Success     bool   `json:"success"`
	StepCount   int    `json:"step_count"`
}
