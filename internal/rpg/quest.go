package rpg

import (
	"encoding/json"
	"fmt"
)

// 任务类型
type QuestType string

const (
	QuestTypeMain        QuestType = "main"        // 主线任务
	QuestTypeSide        QuestType = "side"        // 支线任务
	QuestTypeDaily       QuestType = "daily"       // 日常任务
	QuestTypeWeekly      QuestType = "weekly"      // 周常任务
	QuestTypeChain       QuestType = "chain"       // 连锁任务
	QuestTypeHidden      QuestType = "hidden"      // 隐藏任务
	QuestTypeAchievement QuestType = "achievement" // 成就
)

// 任务状态
type QuestStatus string

const (
	QuestStatusLocked    QuestStatus = "locked"    // 未解锁
	QuestStatusAvailable QuestStatus = "available" // 可接取
	QuestStatusActive    QuestStatus = "active"    // 进行中
	QuestStatusCompleted QuestStatus = "completed" // 已完成
	QuestStatusTurnedIn  QuestStatus = "turned_in" // 已提交
	QuestStatusFailed    QuestStatus = "failed"    // 已失败
)

// 任务目标类型
type QuestObjectiveType string

const (
	ObjectiveKill      QuestObjectiveType = "kill"      // 击杀敌人
	ObjectiveCollect   QuestObjectiveType = "collect"   // 收集物品
	ObjectiveTalk      QuestObjectiveType = "talk"      // 与NPC对话
	ObjectiveReach     QuestObjectiveType = "reach"     // 到达地点
	ObjectiveDefeat    QuestObjectiveType = "defeat"    // 击败特定目标
	ObjectiveEscort    QuestObjectiveType = "escort"    // 护送
	ObjectiveProtect   QuestObjectiveType = "protect"   // 保护
	ObjectiveExplore   QuestObjectiveType = "explore"   // 探索
	ObjectiveCraft     QuestObjectiveType = "craft"     // 制造
	ObjectiveUse       QuestObjectiveType = "use"       // 使用物品
	ObjectiveSkill     QuestObjectiveType = "skill"     // 使用技能
	ObjectiveEvent     QuestObjectiveType = "event"     // 触发事件
	ObjectiveCondition QuestObjectiveType = "condition" // 满足条件
)

// 任务目标
type QuestObjective struct {
	ID           string             `json:"id"`
	Type         QuestObjectiveType `json:"type"`
	Description  string             `json:"description"`
	TargetID     string             `json:"target_id"`     // 目标ID（敌人/NPC/物品等）
	TargetCount  int                `json:"target_count"`  // 目标数量
	CurrentCount int                `json:"current_count"` // 当前进度
	IsOptional   bool               `json:"is_optional"`   // 是否可选
	IsCompleted  bool               `json:"is_completed"`
	LocationID   string             `json:"location_id,omitempty"` // 限定地点
	Conditions   []Condition        `json:"conditions,omitempty"`  // 额外条件
}

// 任务奖励
type QuestReward struct {
	Exp          int                `json:"exp,omitempty"`
	Money        int                `json:"money,omitempty"`
	Items        []RewardItem       `json:"items,omitempty"`
	Equipment    []string           `json:"equipment,omitempty"` // 装备ID
	Skills       []string           `json:"skills,omitempty"`    // 技能ID
	Reputation   []ReputationReward `json:"reputation,omitempty"`
	UnlockQuests []string           `json:"unlock_quests,omitempty"` // 解锁的任务
	UnlockEvents []string           `json:"unlock_events,omitempty"` // 解锁的事件
	UnlockMaps   []string           `json:"unlock_maps,omitempty"`   // 解锁的地图
}

// 奖励物品
type RewardItem struct {
	ItemID string `json:"item_id"`
	Count  int    `json:"count"`
}

// 声望奖励
type ReputationReward struct {
	FactionID string `json:"faction_id"`
	Value     int    `json:"value"`
}

// 任务定义
type Quest struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        QuestType `json:"type"`

	// 任务信息
	LevelRequired    int `json:"level_required"`
	LevelRecommended int `json:"level_recommended,omitempty"`

	// 前置条件
	Prerequisites QuestPrerequisites `json:"prerequisites"`

	// 任务链
	PreviousQuest string   `json:"previous_quest,omitempty"` // 前置任务
	NextQuests    []string `json:"next_quests,omitempty"`    // 后续任务

	// 任务内容
	StartNPC   string           `json:"start_npc,omitempty"` // 起始NPC
	EndNPC     string           `json:"end_npc,omitempty"`   // 结束NPC
	Objectives []QuestObjective `json:"objectives"`

	// 奖励
	Rewards      QuestReward   `json:"rewards"`
	BonusRewards []QuestReward `json:"bonus_rewards,omitempty"` // 额外奖励（可选目标完成）

	// 时间限制
	TimeLimit int `json:"time_limit,omitempty"` // 时间限制（分钟）

	// 重复性
	IsRepeatable   bool `json:"is_repeatable"`
	RepeatCooldown int  `json:"repeat_cooldown,omitempty"` // 重复冷却（分钟）

	// 任务标记
	IsTracked bool `json:"is_tracked,omitempty"` // 是否追踪
	Priority  int  `json:"priority,omitempty"`   // 优先级

	// 剧情相关
	StoryChapter string   `json:"story_chapter,omitempty"`
	Cutscenes    []string `json:"cutscenes,omitempty"` // 相关剧情动画

	Tags []string `json:"tags,omitempty"`
}

// 任务前置条件
type QuestPrerequisites struct {
	Level      int                    `json:"level,omitempty"`
	Quests     []string               `json:"quests,omitempty"`     // 需要完成的任务
	Items      []string               `json:"items,omitempty"`      // 需要的物品
	Skills     []string               `json:"skills,omitempty"`     // 需要的技能
	Reputation map[string]int         `json:"reputation,omitempty"` // 需要的声望
	Switches   map[string]bool        `json:"switches,omitempty"`   // 需要的开关
	Variables  map[string]interface{} `json:"variables,omitempty"`
}

// 任务实例（玩家任务状态）
type QuestInstance struct {
	QuestID          string           `json:"quest_id"`
	Status           QuestStatus      `json:"status"`
	StartTime        string           `json:"start_time,omitempty"`
	CompleteTime     string           `json:"complete_time,omitempty"`
	Objectives       []QuestObjective `json:"objectives"`
	CurrentStep      int              `json:"current_step"`           // 当前步骤
	IsBonusCompleted bool             `json:"is_bonus_completed"`     // 是否完成额外目标
	RepeatCount      int              `json:"repeat_count,omitempty"` // 重复次数
}

// 任务日志
type QuestLog struct {
	ActiveQuests    []string                  `json:"active_quests"`    // 进行中的任务ID
	CompletedQuests []string                  `json:"completed_quests"` // 已完成的任务ID
	FailedQuests    []string                  `json:"failed_quests"`    // 失败的任务ID
	QuestInstances  map[string]*QuestInstance `json:"quest_instances"`
	TrackedQuest    string                    `json:"tracked_quest,omitempty"`
}

// 任务管理器
type QuestManager struct {
	quests    map[string]*Quest
	instances map[string]*QuestInstance // key: questID
	executor  *EventExecutor
}

func NewQuestManager(executor *EventExecutor) *QuestManager {
	return &QuestManager{
		quests:    make(map[string]*Quest),
		instances: make(map[string]*QuestInstance),
		executor:  executor,
	}
}

func (qm *QuestManager) AddQuest(quest *Quest) {
	qm.quests[quest.ID] = quest
}

func (qm *QuestManager) GetQuest(id string) *Quest {
	return qm.quests[id]
}

func (qm *QuestManager) GetAllQuests() []*Quest {
	result := make([]*Quest, 0, len(qm.quests))
	for _, quest := range qm.quests {
		result = append(result, quest)
	}
	return result
}

func (qm *QuestManager) GetQuestsByType(questType QuestType) []*Quest {
	result := make([]*Quest, 0)
	for _, quest := range qm.quests {
		if quest.Type == questType {
			result = append(result, quest)
		}
	}
	return result
}

// 检查是否可以接取任务
func (qm *QuestManager) CanAcceptQuest(questID string, character *Character, questLog *QuestLog) (bool, string) {
	quest := qm.quests[questID]
	if quest == nil {
		return false, "任务不存在"
	}

	instance := qm.instances[questID]
	if instance != nil && instance.Status == QuestStatusActive {
		return false, "任务已在进行中"
	}

	if instance != nil && instance.Status == QuestStatusCompleted && !quest.IsRepeatable {
		return false, "任务已完成"
	}

	// 检查前置任务
	for _, preQuestID := range quest.Prerequisites.Quests {
		preInstance := qm.instances[preQuestID]
		if preInstance == nil || preInstance.Status != QuestStatusCompleted {
			return false, "前置任务未完成"
		}
	}

	// 检查前置开关
	for switchID, required := range quest.Prerequisites.Switches {
		if qm.executor.GetSwitch(switchID) != required {
			return false, "条件未满足"
		}
	}

	return true, ""
}

// 接取任务
func (qm *QuestManager) AcceptQuest(questID string, character *Character) *QuestInstance {
	quest := qm.quests[questID]
	if quest == nil {
		return nil
	}

	instance := &QuestInstance{
		QuestID:     questID,
		Status:      QuestStatusActive,
		StartTime:   fmt.Sprintf("%d", getCurrentTime()),
		Objectives:  make([]QuestObjective, len(quest.Objectives)),
		CurrentStep: 0,
	}

	// 复制目标
	copy(instance.Objectives, quest.Objectives)

	qm.instances[questID] = instance

	// 触发任务开始事件
	if qm.executor != nil {
		qm.executor.SetSwitch(fmt.Sprintf("quest_%s_started", questID), true)
	}

	return instance
}

// 更新任务进度
func (qm *QuestManager) UpdateObjective(questID, objectiveID string, progress int) bool {
	instance := qm.instances[questID]
	if instance == nil || instance.Status != QuestStatusActive {
		return false
	}

	for i := range instance.Objectives {
		if instance.Objectives[i].ID == objectiveID && !instance.Objectives[i].IsCompleted {
			instance.Objectives[i].CurrentCount += progress
			if instance.Objectives[i].CurrentCount >= instance.Objectives[i].TargetCount {
				instance.Objectives[i].CurrentCount = instance.Objectives[i].TargetCount
				instance.Objectives[i].IsCompleted = true

				// 检查是否所有必需目标完成
				qm.checkQuestCompletion(instance)
			}
			return true
		}
	}

	return false
}

// 增加任务进度（通过类型）
func (qm *QuestManager) AddProgress(objectiveType QuestObjectiveType, targetID string, amount int) {
	for _, instance := range qm.instances {
		if instance.Status != QuestStatusActive {
			continue
		}

		for i := range instance.Objectives {
			obj := &instance.Objectives[i]
			if obj.Type == objectiveType && obj.TargetID == targetID && !obj.IsCompleted {
				obj.CurrentCount += amount
				if obj.CurrentCount >= obj.TargetCount {
					obj.CurrentCount = obj.TargetCount
					obj.IsCompleted = true
					qm.checkQuestCompletion(instance)
				}
			}
		}
	}
}

// 检查任务完成
func (qm *QuestManager) checkQuestCompletion(instance *QuestInstance) {
	quest := qm.quests[instance.QuestID]
	if quest == nil {
		return
	}

	allCompleted := true
	bonusCompleted := true

	for _, obj := range instance.Objectives {
		if !obj.IsOptional && !obj.IsCompleted {
			allCompleted = false
			break
		}
		if obj.IsOptional && !obj.IsCompleted {
			bonusCompleted = false
		}
	}

	if allCompleted {
		instance.Status = QuestStatusCompleted
		instance.CompleteTime = fmt.Sprintf("%d", getCurrentTime())
		instance.IsBonusCompleted = bonusCompleted

		// 触发任务完成事件
		if qm.executor != nil {
			qm.executor.SetSwitch(fmt.Sprintf("quest_%s_completed", instance.QuestID), true)
		}
	}
}

// 提交任务
func (qm *QuestManager) TurnInQuest(questID string, character *Character) *QuestReward {
	instance := qm.instances[questID]
	if instance == nil || instance.Status != QuestStatusCompleted {
		return nil
	}

	quest := qm.quests[questID]
	if quest == nil {
		return nil
	}

	instance.Status = QuestStatusTurnedIn

	// 发放奖励
	rewards := &quest.Rewards

	// 经验
	if rewards.Exp > 0 {
		character.GainExp(rewards.Exp)
	}

	// 物品
	for _, item := range rewards.Items {
		character.AddItem(item.ItemID, item.Count)
	}

	// 技能
	for _, skillID := range rewards.Skills {
		character.LearnSkill(skillID)
	}

	// 额外奖励
	if instance.IsBonusCompleted && len(quest.BonusRewards) > 0 {
		// 发放额外奖励...
	}

	// 解锁后续任务
	for _, nextQuestID := range quest.NextQuests {
		if qm.executor != nil {
			qm.executor.SetSwitch(fmt.Sprintf("quest_%s_available", nextQuestID), true)
		}
	}

	// 触发任务提交事件
	if qm.executor != nil {
		qm.executor.SetSwitch(fmt.Sprintf("quest_%s_turnedin", questID), true)
	}

	return rewards
}

// 获取任务日志
func (qm *QuestManager) GetQuestLog() *QuestLog {
	log := &QuestLog{
		ActiveQuests:    make([]string, 0),
		CompletedQuests: make([]string, 0),
		FailedQuests:    make([]string, 0),
		QuestInstances:  make(map[string]*QuestInstance),
	}

	for id, instance := range qm.instances {
		log.QuestInstances[id] = instance

		switch instance.Status {
		case QuestStatusActive:
			log.ActiveQuests = append(log.ActiveQuests, id)
		case QuestStatusTurnedIn:
			log.CompletedQuests = append(log.CompletedQuests, id)
		case QuestStatusFailed:
			log.FailedQuests = append(log.FailedQuests, id)
		}
	}

	return log
}

// 获取进行中的任务
func (qm *QuestManager) GetActiveQuests() []*QuestInstance {
	result := make([]*QuestInstance, 0)
	for _, instance := range qm.instances {
		if instance.Status == QuestStatusActive {
			result = append(result, instance)
		}
	}
	return result
}

// 获取可接取的任务
func (qm *QuestManager) GetAvailableQuests(character *Character, questLog *QuestLog) []*Quest {
	result := make([]*Quest, 0)
	for _, quest := range qm.quests {
		canAccept, _ := qm.CanAcceptQuest(quest.ID, character, questLog)
		if canAccept {
			result = append(result, quest)
		}
	}
	return result
}

// 放弃任务
func (qm *QuestManager) AbandonQuest(questID string) bool {
	instance := qm.instances[questID]
	if instance == nil || instance.Status != QuestStatusActive {
		return false
	}

	delete(qm.instances, questID)
	return true
}

// 失败任务
func (qm *QuestManager) FailQuest(questID string) bool {
	instance := qm.instances[questID]
	if instance == nil || instance.Status != QuestStatusActive {
		return false
	}

	instance.Status = QuestStatusFailed
	return true
}

// 重置可重复任务
func (qm *QuestManager) ResetRepeatableQuest(questID string) bool {
	quest := qm.quests[questID]
	if quest == nil || !quest.IsRepeatable {
		return false
	}

	instance := qm.instances[questID]
	if instance == nil {
		return false
	}

	// 检查冷却
	if quest.RepeatCooldown > 0 {
		// 检查是否过了冷却时间...
	}

	// 重置实例
	instance.Status = QuestStatusAvailable
	instance.Objectives = make([]QuestObjective, len(quest.Objectives))
	copy(instance.Objectives, quest.Objectives)
	instance.CurrentStep = 0
	instance.IsBonusCompleted = false
	instance.RepeatCount++

	return true
}

// 辅助函数
func getCurrentTime() int64 {
	return 0 // 简化实现
}

// 序列化
func (q *Quest) ToJSON() string {
	data, _ := json.MarshalIndent(q, "", "  ")
	return string(data)
}

func (qi *QuestInstance) ToJSON() string {
	data, _ := json.MarshalIndent(qi, "", "  ")
	return string(data)
}

func (ql *QuestLog) ToJSON() string {
	data, _ := json.MarshalIndent(ql, "", "  ")
	return string(data)
}

// ExportToMap 导出为map
func (qm *QuestManager) ExportToMap() map[string]interface{} {
	return map[string]interface{}{
		"quests":    qm.quests,
		"instances": qm.instances,
	}
}
