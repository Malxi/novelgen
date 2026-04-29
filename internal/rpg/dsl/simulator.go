package dsl

import (
	"fmt"
	"strconv"
	"strings"
)

// SimulationIssue 表示模拟中发现的问题
type SimulationIssue struct {
	Type        IssueType       `json:"type"`
	Severity    SeverityLevel   `json:"severity"`
	Chapter     string          `json:"chapter,omitempty"`
	Step        int             `json:"step,omitempty"`
	Description string          `json:"description"`
	Suggestion  string          `json:"suggestion,omitempty"`
	Evidence    []IssueEvidence `json:"evidence,omitempty"`
}

// IssueEvidence points to the DSL facts or chapter observations behind an issue.
type IssueEvidence struct {
	Chapter string `json:"chapter,omitempty"`
	Step    int    `json:"step,omitempty"`
	Source  string `json:"source,omitempty"`
	Text    string `json:"text,omitempty"`
}

// IssueType 问题类型 - 针对小说剧情检测
type IssueType string

const (
	IssueLogic       IssueType = "logic"        // 逻辑矛盾
	IssueCharacter   IssueType = "character"    // 角色行为不一致
	IssueContinuity  IssueType = "continuity"   // 情节连续性
	IssuePacing      IssueType = "pacing"       // 节奏问题
	IssuePlotHole    IssueType = "plot_hole"    // 剧情漏洞
	IssueMissingInfo IssueType = "missing_info" // 信息缺失
	IssueConflict    IssueType = "conflict"     // 冲突设置
	IssueDescription IssueType = "description"  // 描述完整性
	IssueBalance     IssueType = "balance"      // 战斗/难度平衡
	IssueGrowth      IssueType = "growth"       // 角色成长
	IssueEquipment   IssueType = "equipment"    // 装备/道具
)

// SeverityLevel 严重程度
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical" // 严重
	SeverityWarning  SeverityLevel = "warning"  // 警告
	SeverityInfo     SeverityLevel = "info"     // 信息
)

// Simulator 剧情模拟器
type Simulator struct {
	DSL          *DSL
	Issues       []SimulationIssue
	Context      *SimulationContext
	characterArc map[string][]string // 记录角色出现的章节
	eventLog     []EventLog          // 事件日志
}

// SimulationContext 模拟上下文 - 针对小说
type SimulationContext struct {
	CurrentChapter   string
	CurrentLocation  string
	ActiveCharacters map[string]bool     // 当前在场的角色
	KnownInfo        map[string]bool     // 主角已知道的信息
	CompletedEvents  map[string]bool     // 已完成的事件
	ChapterEvents    map[string][]string // 每个章节的事件列表

	// 主角状态跟踪（根据小说类型）
	Protagonist ProtagonistState

	// 战力历史（每章记录一次）
	PowerHistory []PowerSnapshot

	// 可选进度规则（从 DSL Systems 读取）
	ProgressionRules ProgressionRules

	// 伏笔追踪
	PlotThreadsRaised   int
	PlotThreadsResolved int
}

// ProtagonistState 主角状态
type ProtagonistState struct {
	Name      string
	Level     int // 等级/境界（如：练气期、筑基期、金丹期...）
	Power     int // 战斗力/修为
	HP        int // 生命/体力
	MaxHP     int
	MP        int // 法力/精神力/能量
	MaxMP     int
	Skills    []string // 已掌握的技能/功法
	Items     []string // 拥有的道具/法宝/装备
	Allies    []string // 盟友/伙伴
	Inventory Capacity // 储物空间
}

// Capacity 储物空间
type Capacity struct {
	Type      string // 储物袋、空间背包、系统空间、纳戒等
	Current   int
	Max       int
	CanExpand bool // 是否可以扩容
}

// EventLog 事件日志
type EventLog struct {
	Chapter     string
	Step        int
	EventType   string
	Description string
	Characters  []string
	Location    string
}

// PowerSnapshot records the protagonist's power state at a chapter boundary.
type PowerSnapshot struct {
	ChapterID           string
	Power               int
	Level               int
	HasBreakthrough     bool // true if this chapter contains a breakthrough/evolution event
	BreakthroughDetails string
}

// ProgressionRules defines optional constraints read from DSL Systems.
// All fields are optional — zero/empty means "no constraint".
type ProgressionRules struct {
	MaxPowerIncreaseRatio float64 // e.g. 2.0 = power may at most double in one chapter without a breakthrough
}

// NewSimulator 创建新的模拟器
func NewSimulator(dsl *DSL) *Simulator {
	return &Simulator{
		DSL:    dsl,
		Issues: make([]SimulationIssue, 0),
		Context: &SimulationContext{
			ActiveCharacters: make(map[string]bool),
			KnownInfo:        make(map[string]bool),
			CompletedEvents:  make(map[string]bool),
			ChapterEvents:    make(map[string][]string),
			Protagonist:      ProtagonistState{},
		},
		characterArc: make(map[string][]string),
		eventLog:     make([]EventLog, 0),
	}
}

// SimulateAll 模拟所有章节
func (s *Simulator) SimulateAll() []SimulationIssue {
	// 初始化主角状态
	s.initializeProtagonist()

	// 初始化
	s.initialize()

	if s.DSL.Metadata != nil && s.DSL.Metadata.Phase == string(PhaseSetup) {
		s.checkStoryContractQuality()
		return s.Issues
	}

	// 检查整体结构
	s.checkStoryStructure()

	// 检查角色设定
	s.checkCharacterSetup()

	// 检查地点设定
	s.checkLocationSetup()

	// 模拟每个章节
	for _, chapter := range s.DSL.Storyline.Chapters {
		s.simulateChapter(&chapter)
	}

	s.checkNarrativeStateConsistency()

	// 检查整体一致性
	s.checkOverallConsistency()

	// 通用检查（从 DSL Systems 读取规则，不绑定小说类型）
	s.checkPowerProgression()
	s.checkStoryContractQuality()
	s.checkPlotThreads()

	return s.Issues
}

// SimulateChapter 模拟单个章节
func (s *Simulator) SimulateChapter(chapterID string) []SimulationIssue {
	chapter := s.findChapter(chapterID)
	if chapter == nil {
		s.addIssue(IssueLogic, SeverityCritical, "", 0,
			fmt.Sprintf("章节 '%s' 不存在", chapterID),
			"检查章节 ID 是否正确")
		return s.Issues
	}

	s.simulateChapter(chapter)
	return s.Issues
}

// 初始化主角状态
func (s *Simulator) initializeProtagonist() {
	if s.DSL.Characters.Player == nil {
		return
	}

	player := s.DSL.Characters.Player
	s.Context.Protagonist.Name = player.Name
	s.Context.Protagonist.Level = 1
	s.Context.Protagonist.Skills = player.Skills
	s.Context.Protagonist.Items = make([]string, 0)

	// 根据小说类型推断储物空间类型
	powerSystem := strings.ToLower(s.DSL.Metadata.PowerSystem)
	switch {
	case strings.Contains(powerSystem, "cultivation"), strings.Contains(powerSystem, "修仙"), strings.Contains(powerSystem, "修真"):
		s.Context.Protagonist.Inventory = Capacity{Type: "储物袋", Current: 0, Max: 20, CanExpand: true}
	case strings.Contains(powerSystem, "system"), strings.Contains(powerSystem, "系统"):
		s.Context.Protagonist.Inventory = Capacity{Type: "系统空间", Current: 0, Max: 50, CanExpand: true}
	case strings.Contains(powerSystem, "sci-fi"), strings.Contains(powerSystem, "科幻"):
		s.Context.Protagonist.Inventory = Capacity{Type: "空间背包", Current: 0, Max: 30, CanExpand: false}
	case strings.Contains(powerSystem, "magic"), strings.Contains(powerSystem, "魔法"):
		s.Context.Protagonist.Inventory = Capacity{Type: "魔法行囊", Current: 0, Max: 25, CanExpand: false}
	default:
		s.Context.Protagonist.Inventory = Capacity{Type: "背包", Current: 0, Max: 20, CanExpand: false}
	}

	// 根据属性计算战斗力
	s.Context.Protagonist.HP = player.Stats.HP
	s.Context.Protagonist.MaxHP = player.Stats.HP
	s.Context.Protagonist.MP = player.Stats.MP
	s.Context.Protagonist.MaxMP = player.Stats.MP
	s.Context.Protagonist.Power = s.calculatePower(player.Stats)
}

// 计算战斗力 - 使用DSL中定义的公式
func (s *Simulator) calculatePower(stats Stats) int {
	// 优先使用DSL中定义的战力公式
	if s.DSL.Systems != nil && s.DSL.Systems.PowerFormula != nil {
		formula := s.DSL.Systems.PowerFormula
		power := formula.BasePower

		// 根据factors计算
		for _, factor := range formula.Factors {
			value := s.getAttributeValue(factor.Attribute, stats)
			power += int(float64(value) * factor.Weight)
		}

		return power
	}

	// 使用默认计算
	return stats.STR*2 + stats.AGI*1 + stats.INT*2 + stats.VIT*1 + stats.HP/10
}

// getAttributeValue 获取属性值（支持自定义属性）
func (s *Simulator) getAttributeValue(attrID string, stats Stats) int {
	// 标准属性
	switch attrID {
	case "str", "STR":
		return stats.STR
	case "agi", "AGI":
		return stats.AGI
	case "int", "INT":
		return stats.INT
	case "vit", "VIT":
		return stats.VIT
	case "hp", "HP":
		return stats.HP
	case "mp", "MP":
		return stats.MP
	}

	// 自定义属性 - 从AttributeSystem获取
	if s.DSL.Systems != nil && s.DSL.Systems.AttributeSystem != nil {
		for _, attr := range s.DSL.Systems.AttributeSystem.Attributes {
			if attr.ID == attrID || attr.Name == attrID {
				return attr.BaseValue
			}
		}
	}

	return 0
}

// 初始化
func (s *Simulator) initialize() {
	// 初始化角色弧线追踪
	for _, npc := range s.DSL.Characters.NPCs {
		s.characterArc[npc.ID] = make([]string, 0)
	}

	// 从 DSL Systems 读取可选的进度规则
	if s.DSL.Systems != nil {
		s.Context.ProgressionRules = s.parseProgressionRules()
	}
}

func (s *Simulator) parseProgressionRules() ProgressionRules {
	rules := ProgressionRules{MaxPowerIncreaseRatio: 2.0} // default: warn if power doubles in one chapter
	for _, sys := range s.DSL.Systems.ProgressionSystems {
		if sys.ID == "progression_rules" || strings.Contains(strings.ToLower(sys.Name), "progression") {
			for _, lvl := range sys.Levels {
				switch strings.ToLower(lvl.Name) {
				case "max_power_increase_ratio":
					if v, err := strconv.ParseFloat(strings.TrimSpace(lvl.Requirements), 64); err == nil && v > 0 {
						rules.MaxPowerIncreaseRatio = v
					}
				}
			}
		}
	}
	return rules
}

// 检查故事结构
func (s *Simulator) checkStoryStructure() {
	chapters := s.DSL.Storyline.Chapters

	// 检查章节数量
	if len(chapters) == 0 {
		s.addIssue(IssueMissingInfo, SeverityCritical, "", 0,
			"故事没有章节",
			"至少添加一个章节")
		return
	}

	// 检查章节顺序是否合理
	if len(chapters) > 1 {
		// 检查是否有明确的开始和结束
		hasIntro := false
		hasConclusion := false

		for _, chapter := range chapters {
			if len(chapter.Objectives) > 0 {
				firstObj := chapter.Objectives[0]
				// 简单的启发式判断
				if strings.Contains(strings.ToLower(firstObj.Name), "开始") ||
					strings.Contains(strings.ToLower(firstObj.Name), "序章") ||
					strings.Contains(strings.ToLower(firstObj.Name), "苏醒") {
					hasIntro = true
				}
				if strings.Contains(strings.ToLower(firstObj.Name), "结局") ||
					strings.Contains(strings.ToLower(firstObj.Name), "最终") ||
					strings.Contains(strings.ToLower(firstObj.Name), "尾声") {
					hasConclusion = true
				}
			}
		}

		if !hasIntro {
			s.addIssue(IssuePacing, SeverityInfo, "", 0,
				"建议添加明确的起始章节（序章/苏醒等）",
				"让读者更容易进入故事")
		}
		if !hasConclusion {
			s.addIssue(IssuePacing, SeverityInfo, "", 0,
				"建议添加明确的结尾章节",
				"给故事一个完整的收束")
		}
	}

	// 检查章节长度分布
	totalSteps := 0
	chapterStepCounts := make(map[string]int)
	for _, chapter := range chapters {
		steps := 0
		for _, obj := range chapter.Objectives {
			steps += len(obj.Steps)
		}
		chapterStepCounts[chapter.ID] = steps
		totalSteps += steps
	}

	if totalSteps > 0 {
		avgSteps := totalSteps / len(chapters)
		for chapterID, steps := range chapterStepCounts {
			if steps > avgSteps*3 {
				s.addIssue(IssuePacing, SeverityWarning, chapterID, 0,
					fmt.Sprintf("章节步骤数(%d)远超过平均值(%d)，可能导致节奏拖沓", steps, avgSteps),
					"考虑将章节拆分为多个小章节")
			}
			if steps < 2 && len(chapters) > 3 {
				s.addIssue(IssuePacing, SeverityInfo, chapterID, 0,
					fmt.Sprintf("章节步骤数(%d)较少，可能过于简短", steps),
					"考虑添加更多情节发展或与其他章节合并")
			}
		}
	}
}

// 检查角色设定
func (s *Simulator) checkCharacterSetup() {
	// 检查主角
	if s.DSL.Characters.Player == nil {
		s.addIssue(IssueMissingInfo, SeverityCritical, "", 0,
			"缺少主角设定",
			"在 characters 块中定义 player")
	} else {
		player := s.DSL.Characters.Player
		if strings.TrimSpace(player.Background) == "" {
			s.addIssue(IssueDescription, SeverityWarning, "", 0,
				"主角缺少背景故事",
				"添加主角的背景故事，让读者更容易产生共鸣")
		}
		if strings.TrimSpace(player.Motivation) == "" && len(player.Personality) == 0 {
			s.addIssue(IssueCharacter, SeverityWarning, "", 0,
				"主角缺少性格或动机描述",
				"定义主角的性格特点和行动动机")
		}

		// 检查主角初始能力
		if player.Class == "" {
			s.addIssue(IssueMissingInfo, SeverityInfo, "", 0,
				"主角缺少职业/修炼体系设定",
				"定义主角的修炼体系或职业类型")
		}
	}

	// 检查NPC
	if len(s.DSL.Characters.NPCs) == 0 {
		s.addIssue(IssueConflict, SeverityWarning, "", 0,
			"故事中没有NPC",
			"添加一些NPC来推动剧情发展")
	}

	// 检查角色定位
	roles := make(map[string]int)
	for _, npc := range s.DSL.Characters.NPCs {
		roles[npc.Role]++
	}

	// 检查是否有足够的角色类型
	if roles["antagonist"] == 0 && roles["villain"] == 0 {
		s.addIssue(IssueConflict, SeverityInfo, "", 0,
			"缺少明确的反派角色",
			"考虑添加一个反派角色来增加冲突")
	}

	// 检查是否有导师/盟友角色
	if roles["mentor"] == 0 && roles["guide"] == 0 {
		s.addIssue(IssueCharacter, SeverityInfo, "", 0,
			"缺少导师/引路人角色",
			"考虑添加一个导师角色来帮助主角成长")
	}
}

// 检查地点设定
func (s *Simulator) checkLocationSetup() {
	locations := s.DSL.World.Locations

	if len(locations) == 0 {
		s.addIssue(IssueMissingInfo, SeverityWarning, "", 0,
			"故事中没有地点设定",
			"添加至少2-3个关键地点")
		return
	}

	// 检查地点描述
	for _, loc := range locations {
		if loc.IsPlaceholder {
			continue
		}
		if strings.TrimSpace(loc.Description) == "" {
			s.addIssue(IssueDescription, SeverityWarning, "", 0,
				fmt.Sprintf("地点 '%s' 缺少描述", loc.Name),
				"添加地点的详细描述，帮助读者想象场景")
		}
		if strings.TrimSpace(loc.Atmosphere) == "" {
			s.addIssue(IssueDescription, SeverityInfo, "", 0,
				fmt.Sprintf("地点 '%s' 缺少氛围描述", loc.Name),
				"添加氛围描述，增强场景感")
		}
	}

	// 检查地点连通性
	locationIDs := make(map[string]bool)
	for _, loc := range locations {
		locationIDs[loc.ID] = true
	}

	// 检查连接是否指向存在的地点
	for _, loc := range locations {
		for _, conn := range loc.Connections {
			if !locationIDs[conn.To] {
				s.addIssue(IssueLogic, SeverityWarning, "", 0,
					fmt.Sprintf("地点 '%s' 连接到不存在的地点 '%s'", loc.Name, conn.To),
					"检查连接目标是否正确")
			}
		}
	}
}

// 模拟章节
func (s *Simulator) simulateChapter(chapter *Chapter) {
	s.Context.CurrentChapter = chapter.ID
	s.Context.ChapterEvents[chapter.ID] = make([]string, 0)

	// 记录章节中的事件
	for _, objective := range chapter.Objectives {
		for _, step := range objective.Steps {
			s.simulateStep(chapter.ID, &step)
		}
	}

	// 检查章节内部的逻辑
	s.checkChapterLogic(chapter)

	// 记录主角战力快照（用于跨章节对比）
	s.recordPowerSnapshot(chapter)
	s.trackPlotThreads(chapter)
}

// 模拟步骤
func (s *Simulator) simulateStep(chapterID string, step *Step) {
	// 检查步骤描述
	if strings.TrimSpace(step.Description) == "" {
		s.addIssue(IssueDescription, SeverityWarning, chapterID, step.Order,
			fmt.Sprintf("步骤 %d 缺少描述", step.Order),
			"添加详细的步骤描述")
		return
	}

	// 记录事件
	eventLog := EventLog{
		Chapter:     chapterID,
		Step:        step.Order,
		EventType:   step.Event.Type,
		Description: step.Description,
	}
	s.eventLog = append(s.eventLog, eventLog)
	s.Context.ChapterEvents[chapterID] = append(s.Context.ChapterEvents[chapterID], step.Description)

	// 根据事件类型检查
	switch step.Event.Type {
	case "combat":
		s.checkCombatEvent(chapterID, step)
	case "location":
		s.checkLocationEvent(chapterID, step)
	case "acquire":
		s.checkAcquireEvent(chapterID, step)
	case "knowledge":
		s.checkKnowledgeEvent(chapterID, step)
	case "relationship":
		s.checkRelationshipEvent(chapterID, step)
	}

	// 检查事件结果
	if step.Event.OnComplete != nil && step.Event.OnComplete.Narration != "" {
		// 有 narration 是好的
		// 检查是否有成长奖励
		if step.Event.OnComplete.Exp > 0 {
			s.checkGrowthReward(chapterID, step)
		}
	} else if step.Event.OnComplete != nil && step.Event.OnComplete.Result == "" {
		// 没有结果说明
		s.addIssue(IssueDescription, SeverityInfo, chapterID, step.Order,
			"事件完成后缺少结果描述",
			"添加事件的 narrative 结果")
	}
}

// 检查战斗事件
func (s *Simulator) checkCombatEvent(chapterID string, step *Step) {
	if step.Event.Combat == nil {
		s.addIssue(IssueLogic, SeverityWarning, chapterID, step.Order,
			"战斗事件缺少配置",
			"添加战斗的具体配置（敌人、场景等）")
		return
	}

	if len(step.Event.Combat.Setup.Enemies) == 0 {
		s.addIssue(IssueMissingInfo, SeverityWarning, chapterID, step.Order,
			"战斗事件没有指定敌人",
			"添加具体的敌人信息（名称、等级、能力等）")
		return
	}

	// 检查战斗平衡性 - 主角能否获胜？
	enemyTotalPower := 0
	hasEnemyInfo := false
	for _, enemy := range step.Event.Combat.Setup.Enemies {
		// 查找敌人信息
		enemyInfo := s.findEnemy(enemy.ID)
		if enemyInfo != nil {
			hasEnemyInfo = true
			enemyPower := s.calculateEnemyPower(enemyInfo)
			enemyTotalPower += enemyPower * enemy.Count
		}
	}

	if hasEnemyInfo {
		protagonistPower := s.Context.Protagonist.Power
		allyPower := s.calculateAllyPower()
		totalPower := protagonistPower + allyPower

		// 检查战斗难度
		if enemyTotalPower > totalPower*2 {
			s.addIssue(IssueBalance, SeverityCritical, chapterID, step.Order,
				fmt.Sprintf("战斗难度过高！主角战力(%d)+%s支援(%d)远低于敌人总战力(%d)",
					protagonistPower, s.getAllyDescription(), allyPower, enemyTotalPower),
				"降低敌人等级/数量，或给主角增加技能、道具、盟友支援")
		} else if enemyTotalPower > totalPower*3/2 {
			s.addIssue(IssueBalance, SeverityWarning, chapterID, step.Order,
				fmt.Sprintf("战斗难度较高。建议给主角准备：技能(%d个)、道具(%d个)、盟友(%d个)",
					len(s.Context.Protagonist.Skills), len(s.Context.Protagonist.Items), len(s.Context.Protagonist.Allies)),
				"增加战斗前的准备：修炼突破、获得新技能、找到强力道具、获得盟友帮助")
		}
	}

	// 检查战斗是否有叙事结果
	if step.Event.OnComplete == nil || step.Event.OnComplete.Narration == "" {
		s.addIssue(IssueDescription, SeverityInfo, chapterID, step.Order,
			"战斗事件缺少叙事性结果描述",
			"添加战斗结束后发生了什么（受伤、领悟、获得战利品等）")
	}

	// 检查战斗是否有成长奖励
	if step.Event.OnComplete == nil || step.Event.OnComplete.Exp == 0 {
		s.addIssue(IssueGrowth, SeverityInfo, chapterID, step.Order,
			"战斗后缺少成长/收获",
			"添加战斗后的感悟、突破、获得经验/技能/道具等")
	}
}

// 检查成长奖励
func (s *Simulator) checkGrowthReward(chapterID string, step *Step) {
	if step.Event.OnComplete == nil || step.Event.OnComplete.Exp <= 0 {
		return
	}

	exp := step.Event.OnComplete.Exp
	currentLevel := s.Context.Protagonist.Level

	// 根据经验值判断成长幅度
	if exp >= 100 {
		s.addIssue(IssueGrowth, SeverityInfo, chapterID, step.Order,
			fmt.Sprintf("获得大量成长(%d经验/感悟值)，建议有相应的突破描写", exp),
			"描写主角的顿悟、境界突破、能力觉醒等成长过程")
	}

	// 模拟成长
	if exp >= currentLevel*50 {
		s.Context.Protagonist.Level++
		s.Context.Protagonist.Power += 10
		s.Context.Protagonist.MaxHP += 20
		s.Context.Protagonist.HP = s.Context.Protagonist.MaxHP
	}
}

// 检查位置事件
func (s *Simulator) checkLocationEvent(chapterID string, step *Step) {
	if step.Event.Move == nil || step.Event.Move.To == "" {
		return
	}

	newLocation := step.Event.Move.To
	if !s.locationExists(newLocation) {
		s.addIssue(IssueLogic, SeverityCritical, chapterID, step.Order,
			fmt.Sprintf("移动到未定义的地点: %s", newLocation),
			"在 world 块中定义此地点")
		return
	}

	// 记录位置变化
	if s.Context.CurrentLocation != "" && s.Context.CurrentLocation != newLocation {
		// 检查场景切换是否平滑
		if !s.hasTransitionDescription(step.Description) {
			s.addIssue(IssueContinuity, SeverityInfo, chapterID, step.Order,
				"场景切换建议添加过渡描述",
				"描述角色如何从一个地点移动到另一个地点")
		}
	}
	s.Context.CurrentLocation = newLocation
}

// 检查获取事件
func (s *Simulator) checkAcquireEvent(chapterID string, step *Step) {
	// 检查是否有具体的获取内容
	if step.Event.OnComplete == nil {
		s.addIssue(IssueDescription, SeverityInfo, chapterID, step.Order,
			"获取事件建议添加结果描述",
			"描述获得了什么以及获得后的感受")
		return
	}

	// 检查储物空间是否足够
	if s.Context.Protagonist.Inventory.Current >= s.Context.Protagonist.Inventory.Max {
		s.addIssue(IssueEquipment, SeverityWarning, chapterID, step.Order,
			fmt.Sprintf("%s已满(%d/%d)，无法获得更多物品",
				s.Context.Protagonist.Inventory.Type,
				s.Context.Protagonist.Inventory.Current,
				s.Context.Protagonist.Inventory.Max),
			"清理空间、扩容、或使用物品")
	} else {
		s.Context.Protagonist.Inventory.Current++
	}

	// 记录获得的物品
	if step.Event.OnComplete.Items != nil {
		s.Context.Protagonist.Items = append(s.Context.Protagonist.Items, step.Event.OnComplete.Items...)
	}
}

// 检查知识事件
func (s *Simulator) checkKnowledgeEvent(chapterID string, step *Step) {
	// 检查知识获取是否重复
	infoKey := fmt.Sprintf("%s:%s", chapterID, step.Description)
	if s.Context.KnownInfo[infoKey] {
		s.addIssue(IssueContinuity, SeverityWarning, chapterID, step.Order,
			"重复获取相同信息",
			"检查是否是故意重复还是逻辑错误")
	}
	s.Context.KnownInfo[infoKey] = true

	// 知识获取也是一种成长
	if step.Event.OnComplete != nil && step.Event.OnComplete.Exp > 0 {
		s.checkGrowthReward(chapterID, step)
	}
}

// 检查关系事件
func (s *Simulator) checkRelationshipEvent(chapterID string, step *Step) {
	// 检查是否涉及角色互动
	hasCharacter := false
	for _, npc := range s.DSL.Characters.NPCs {
		if strings.Contains(step.Description, npc.Name) {
			hasCharacter = true
			// 记录角色出现的章节
			s.characterArc[npc.ID] = append(s.characterArc[npc.ID], chapterID)

			// 检查是否是盟友
			if npc.Role == "ally" || npc.Role == "companion" || npc.Role == "partner" {
				s.Context.Protagonist.Allies = append(s.Context.Protagonist.Allies, npc.Name)
			}
			break
		}
	}

	if !hasCharacter && len(s.DSL.Characters.NPCs) > 0 {
		s.addIssue(IssueCharacter, SeverityInfo, chapterID, step.Order,
			"关系事件建议明确涉及的角色",
			"指明是哪个角色的关系发生变化")
	}

	// 关系进展也是一种成长
	if step.Event.OnComplete != nil && step.Event.OnComplete.Exp > 0 {
		s.checkGrowthReward(chapterID, step)
	}
}

// 检查章节逻辑
func (s *Simulator) checkChapterLogic(chapter *Chapter) {
	// 检查章节内步骤的连贯性
	events := s.Context.ChapterEvents[chapter.ID]
	if len(events) < 2 {
		return
	}

	// 检查是否有足够的变化（不能全是同一种类型的事件）
	eventTypes := make(map[string]int)
	for _, objective := range chapter.Objectives {
		for _, step := range objective.Steps {
			eventTypes[step.Event.Type]++
		}
	}

	if len(eventTypes) == 1 && len(events) > 3 {
		s.addIssue(IssuePacing, SeverityWarning, chapter.ID, 0,
			"章节内事件类型单一，可能导致节奏单调",
			"添加不同类型的事件（对话、探索、冲突等）来增加变化")
	}

	// 检查章节是否有成长
	hasGrowth := false
	for _, objective := range chapter.Objectives {
		for _, step := range objective.Steps {
			if step.Event.OnComplete != nil && step.Event.OnComplete.Exp > 0 {
				hasGrowth = true
				break
			}
		}
		if hasGrowth {
			break
		}
	}

	if !hasGrowth && len(events) > 3 {
		s.addIssue(IssueGrowth, SeverityInfo, chapter.ID, 0,
			"章节内容较多但缺少主角成长",
			"考虑添加修炼突破、技能学习、获得道具、获得盟友等成长元素")
	}
}

// 检查整体一致性
func (s *Simulator) checkOverallConsistency() {
	// 检查角色弧线
	for npcID, chapters := range s.characterArc {
		npc := s.findNPC(npcID)
		if npc == nil {
			continue
		}

		// 检查角色是否只在开头或结尾出现
		if len(chapters) > 0 {
			// 简单检查：如果角色只出现在一个章节
			if len(chapters) == 1 && len(s.DSL.Storyline.Chapters) > 3 {
				s.addIssue(IssueCharacter, SeverityInfo, "", 0,
					fmt.Sprintf("NPC '%s' 只在一个章节中出现", npc.Name),
					"考虑让角色在其他章节中也有出现，增强存在感")
			}

			// 检查角色的出现是否过于集中
			if len(chapters) >= 3 {
				// 检查是否连续出现
				consecutive := true
				for i := 1; i < len(chapters); i++ {
					// 简化的连续性检查（假设章节ID包含顺序信息）
					if !strings.Contains(chapters[i], fmt.Sprintf("C%d", i+1)) {
						consecutive = false
						break
					}
				}
				if consecutive {
					s.addIssue(IssuePacing, SeverityInfo, "", 0,
						fmt.Sprintf("NPC '%s' 连续出现在多个章节", npc.Name),
						"考虑适当减少该角色的戏份，给其他角色更多空间")
				}
			}
		}
	}

	// 检查信息揭示的节奏
	knowledgeCount := 0
	for _, event := range s.eventLog {
		if event.EventType == "knowledge" {
			knowledgeCount++
		}
	}

	if len(s.eventLog) > 0 {
		knowledgeRatio := float64(knowledgeCount) / float64(len(s.eventLog))
		if knowledgeRatio > 0.5 {
			s.addIssue(IssuePacing, SeverityWarning, "", 0,
				"故事中信息揭示类事件占比过高",
				"增加更多动作和对话场景，减少纯信息传递")
		}
	}

	// 检查冲突密度
	conflictCount := 0
	for _, event := range s.eventLog {
		if event.EventType == "combat" || event.EventType == "conflict" {
			conflictCount++
		}
	}

	if len(s.eventLog) > 0 {
		conflictRatio := float64(conflictCount) / float64(len(s.eventLog))
		if conflictRatio < 0.1 && len(s.eventLog) > 10 {
			s.addIssue(IssueConflict, SeverityInfo, "", 0,
				"故事中冲突事件较少",
				"考虑添加更多冲突来增加故事张力")
		}
	}

	// 检查主角成长曲线
	finalLevel := s.Context.Protagonist.Level
	chapterCount := len(s.DSL.Storyline.Chapters)

	if chapterCount > 5 && finalLevel < 3 {
		s.addIssue(IssueGrowth, SeverityInfo, "", 0,
			fmt.Sprintf("故事较长(%d章)但主角成长较少(等级%d)，建议增加突破情节", chapterCount, finalLevel),
			"考虑在中期添加境界突破、能力觉醒等成长高潮")
	}

	// 检查装备/道具使用情况
	if len(s.Context.Protagonist.Items) > s.Context.Protagonist.Inventory.Max {
		s.addIssue(IssueEquipment, SeverityWarning, "", 0,
			fmt.Sprintf("主角拥有的物品(%d)超过%s容量(%d)",
				len(s.Context.Protagonist.Items),
				s.Context.Protagonist.Inventory.Type,
				s.Context.Protagonist.Inventory.Max),
			"清理不需要的物品、扩容储物空间、或使用/消耗物品")
	}
}

// 辅助函数

func (s *Simulator) addIssue(issueType IssueType, severity SeverityLevel, chapter string, step int, description, suggestion string) {
	s.addIssueWithEvidence(issueType, severity, chapter, step, description, suggestion, nil)
}

func (s *Simulator) addIssueWithEvidence(issueType IssueType, severity SeverityLevel, chapter string, step int, description, suggestion string, evidence []IssueEvidence) {
	severity = s.normalizeIssueSeverity(issueType, severity)
	s.Issues = append(s.Issues, SimulationIssue{
		Type:        issueType,
		Severity:    severity,
		Chapter:     chapter,
		Step:        step,
		Description: description,
		Suggestion:  suggestion,
		Evidence:    compactIssueEvidence(evidence),
	})
}

func (s *Simulator) normalizeIssueSeverity(issueType IssueType, severity SeverityLevel) SeverityLevel {
	if severity != SeverityCritical || s == nil || s.DSL == nil || s.DSL.Metadata == nil {
		return severity
	}
	if s.DSL.Metadata.Phase == string(PhaseOutline) && issueType == IssueBalance {
		return SeverityWarning
	}
	return severity
}

func compactIssueEvidence(evidence []IssueEvidence) []IssueEvidence {
	if len(evidence) == 0 {
		return nil
	}
	out := make([]IssueEvidence, 0, len(evidence))
	seen := map[string]bool{}
	for _, ev := range evidence {
		ev.Text = strings.TrimSpace(ev.Text)
		if ev.Text == "" {
			continue
		}
		if len([]rune(ev.Text)) > 180 {
			runes := []rune(ev.Text)
			ev.Text = string(runes[:180]) + "..."
		}
		key := fmt.Sprintf("%s|%d|%s|%s", ev.Chapter, ev.Step, ev.Source, ev.Text)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ev)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func (s *Simulator) findChapter(id string) *Chapter {
	for i := range s.DSL.Storyline.Chapters {
		if s.DSL.Storyline.Chapters[i].ID == id {
			return &s.DSL.Storyline.Chapters[i]
		}
	}
	return nil
}

func (s *Simulator) findNPC(id string) *NPC {
	for i := range s.DSL.Characters.NPCs {
		if s.DSL.Characters.NPCs[i].ID == id {
			return &s.DSL.Characters.NPCs[i]
		}
	}
	return nil
}

func (s *Simulator) findEnemy(id string) *Enemy {
	for i := range s.DSL.Characters.Enemies {
		if s.DSL.Characters.Enemies[i].ID == id {
			return &s.DSL.Characters.Enemies[i]
		}
	}
	return nil
}

func (s *Simulator) locationExists(id string) bool {
	for _, loc := range s.DSL.World.Locations {
		if loc.ID == id {
			return true
		}
	}
	return false
}

func (s *Simulator) hasTransitionDescription(description string) bool {
	// 检查描述中是否包含位置移动相关的词汇
	transitionWords := []string{"走", "去", "来到", "到达", "进入", "离开", "前往", "返回"}
	lower := strings.ToLower(description)
	for _, word := range transitionWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func (s *Simulator) calculateEnemyPower(enemy *Enemy) int {
	// 如果有战力公式，使用公式计算
	if s.DSL.Systems != nil && s.DSL.Systems.PowerFormula != nil {
		// 从敌人的stats构建临时Stats对象
		enemyStats := Stats{
			STR: getStatFromMap(enemy.Template.StatsPerLevel, "str"),
			AGI: getStatFromMap(enemy.Template.StatsPerLevel, "agi"),
			INT: getStatFromMap(enemy.Template.StatsPerLevel, "int"),
			VIT: getStatFromMap(enemy.Template.StatsPerLevel, "vit"),
		}
		// 解析HP
		if enemy.Template.HPFormula != "" {
			fmt.Sscanf(enemy.Template.HPFormula, "%d", &enemyStats.HP)
		}
		return s.calculatePower(enemyStats)
	}

	// 根据敌人属性计算战斗力（默认方式）
	power := 0
	for _, stat := range enemy.Template.StatsPerLevel {
		power += stat
	}
	hpBonus := 0
	if enemy.Template.HPFormula != "" {
		fmt.Sscanf(enemy.Template.HPFormula, "%d", &hpBonus)
	}
	return power + hpBonus/10
}

// getStatFromMap 从map中获取属性值
func getStatFromMap(stats map[string]int, key string) int {
	// 尝试小写
	if val, ok := stats[key]; ok {
		return val
	}
	// 尝试大写
	if val, ok := stats[strings.ToUpper(key)]; ok {
		return val
	}
	return 0
}

func (s *Simulator) calculateAllyPower() int {
	// 计算盟友总战力
	power := 0
	for _, ally := range s.Context.Protagonist.Allies {
		// 简单估算，每个盟友提供一定战力
		power += 20
		_ = ally
	}
	return power
}

func (s *Simulator) getAllyDescription() string {
	if len(s.Context.Protagonist.Allies) == 0 {
		return "无盟友"
	}
	return fmt.Sprintf("盟友%d人", len(s.Context.Protagonist.Allies))
}

// recordPowerSnapshot captures the protagonist's power state after a chapter finishes.
func (s *Simulator) recordPowerSnapshot(chapter *Chapter) {
	hasBreakthrough := false
	details := ""
	for _, objective := range chapter.Objectives {
		for _, step := range objective.Steps {
			for _, delta := range step.Event.StateDeltas {
				switch strings.ToLower(delta.Kind) {
				case "breakthrough", "evolution", "cultivation", "power_change":
					if delta.Delta > 0 || delta.To != "" {
						hasBreakthrough = true
						if details == "" && delta.Note != "" {
							details = delta.Note
						}
					}
				}
			}
		}
	}
	s.Context.PowerHistory = append(s.Context.PowerHistory, PowerSnapshot{
		ChapterID:           chapter.ID,
		Power:               s.Context.Protagonist.Power,
		Level:               s.Context.Protagonist.Level,
		HasBreakthrough:     hasBreakthrough,
		BreakthroughDetails: details,
	})
}

// trackPlotThreads counts plot_thread deltas raised/resolved in this chapter.
func (s *Simulator) trackPlotThreads(chapter *Chapter) {
	for _, objective := range chapter.Objectives {
		for _, step := range objective.Steps {
			for _, delta := range step.Event.StateDeltas {
				if strings.ToLower(delta.Kind) != "plot_thread" {
					continue
				}
				switch strings.ToLower(delta.To) {
				case "raised":
					s.Context.PlotThreadsRaised++
				case "resolved":
					s.Context.PlotThreadsResolved++
				}
			}
		}
	}
}

// checkPowerProgression validates that power jumps between chapters are
// accompanied by breakthrough/evolution events. Threshold read from DSL Systems.
func (s *Simulator) checkPowerProgression() {
	if len(s.Context.PowerHistory) < 2 {
		return
	}
	ratio := s.Context.ProgressionRules.MaxPowerIncreaseRatio
	if ratio <= 0 {
		return // no constraint configured
	}
	for i := 1; i < len(s.Context.PowerHistory); i++ {
		prev := s.Context.PowerHistory[i-1]
		curr := s.Context.PowerHistory[i]
		if prev.Power <= 0 {
			continue
		}
		if float64(curr.Power)/float64(prev.Power) > ratio && !curr.HasBreakthrough {
			s.addIssue(IssueGrowth, SeverityWarning, curr.ChapterID, 0,
				fmt.Sprintf("主角战力跳跃过大: 从 %d (C%d) 到 %d (C%d)，增幅 %.1fx，但章节缺少突破/进化事件",
					prev.Power, i, curr.Power, i+1, float64(curr.Power)/float64(prev.Power)),
				"添加对应的突破/进化/觉醒事件说明，或降低单章战力增幅")
		}
	}
}

// checkPlotThreads reports unresolved plot threads at the end of the story.
func (s *Simulator) checkPlotThreads() {
	unresolved := s.Context.PlotThreadsRaised - s.Context.PlotThreadsResolved
	if unresolved > 0 {
		s.addIssue(IssuePlotHole, SeverityInfo, "", 0,
			fmt.Sprintf("存在 %d 个未回收的伏笔/剧情线 (提出 %d, 回收 %d)",
				unresolved, s.Context.PlotThreadsRaised, s.Context.PlotThreadsResolved),
			"确认这些伏笔是否计划在后续章节中回收，或添加对应的 resolved state_delta")
	}
}

// GetIssuesBySeverity 按严重程度获取问题
func (s *Simulator) GetIssuesBySeverity(severity SeverityLevel) []SimulationIssue {
	var result []SimulationIssue
	for _, issue := range s.Issues {
		if issue.Severity == severity {
			result = append(result, issue)
		}
	}
	return result
}

// FormatIssue 格式化问题输出
func FormatIssue(issue SimulationIssue) string {
	var severityIcon string
	switch issue.Severity {
	case SeverityCritical:
		severityIcon = "🔴"
	case SeverityWarning:
		severityIcon = "🟡"
	case SeverityInfo:
		severityIcon = "🔵"
	}

	location := issue.Chapter
	if issue.Step > 0 {
		location = fmt.Sprintf("%s (步骤 %d)", issue.Chapter, issue.Step)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s [%s] %s\n   位置: %s\n   建议: %s",
		severityIcon,
		strings.ToUpper(string(issue.Type)),
		issue.Description,
		location,
		issue.Suggestion,
	))
	if len(issue.Evidence) > 0 {
		b.WriteString("\n   Evidence:")
		for i, ev := range issue.Evidence {
			if i >= 3 {
				break
			}
			loc := ev.Chapter
			if ev.Step > 0 {
				loc = fmt.Sprintf("%s step %d", ev.Chapter, ev.Step)
			}
			if loc == "" {
				loc = "global"
			}
			source := ev.Source
			if source == "" {
				source = "dsl"
			}
			b.WriteString(fmt.Sprintf("\n   - %s / %s: %s", loc, source, ev.Text))
		}
	}
	return b.String()
}
