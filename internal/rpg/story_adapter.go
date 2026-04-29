package rpg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// StoryOutline 小说大纲结构
type StoryOutline struct {
	Parts []StoryPart `json:"parts"`
}

// StoryPart 故事部分
type StoryPart struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Volumes []StoryVolume `json:"volumes"`
}

// StoryVolume 故事卷
type StoryVolume struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Summary  string         `json:"summary"`
	Chapters []StoryChapter `json:"chapters"`
}

// StoryChapterTimeline 章节时间线信息
type StoryChapterTimeline struct {
	Anchor      string `json:"anchor,omitempty"`       // 相对于故事开始的时间点
	StartTime   string `json:"start_time,omitempty"`   // 章节开始的具体时间
	EndTime     string `json:"end_time,omitempty"`     // 章节结束的具体时间
	Duration    string `json:"duration,omitempty"`     // 章节内经过的时间
	TimeJump    bool   `json:"time_jump,omitempty"`    // 是否时间跳跃
	PreviousGap string `json:"previous_gap,omitempty"` // 与上一章的时间间隔
	Transition  string `json:"transition,omitempty"`   // 时间过渡说明
}

// StoryStateAnchor 章节开始时的主角状态锚点
type StoryStateAnchor struct {
	Cultivation  string   `json:"cultivation,omitempty"`   // 修炼境界/能力等级
	SpiritStones int      `json:"spirit_stones,omitempty"` // 灵石/资源数量
	Allies       []string `json:"allies,omitempty"`        // 当前盟友/同伴
	Injuries     []string `json:"injuries,omitempty"`      // 当前伤势
	Location     string   `json:"location,omitempty"`      // 章节开始时的位置
	KeyItems     []string `json:"key_items,omitempty"`     // 当前持有的关键物品
	Notes        string   `json:"notes,omitempty"`         // 其他值得追踪的状态
}

// StoryOutlineEnemy 本章出现的敌人
type StoryOutlineEnemy struct {
	Name    string `json:"name"`              // 敌人名称
	Faction string `json:"faction,omitempty"` // 所属阵营
	Tier    string `json:"tier,omitempty"`    // 阵营内等级标识
	Count   int    `json:"count"`             // 出现数量
	Level   int    `json:"level,omitempty"`   // 敌人等级
	IsBoss  bool   `json:"is_boss,omitempty"` // 是否是boss
	BossID  string `json:"boss_id,omitempty"` // boss唯一ID，跨章追踪
	Status  string `json:"status,omitempty"`  // new/engaged/defeated/escaped
	Context string `json:"context,omitempty"` // 出现场景
}

// StoryResourceLedgerEntry 本章资源变化
type StoryResourceLedgerEntry struct {
	Item   string `json:"item"`   // 资源名称
	Start  int    `json:"start"`  // 开始数量
	Delta  int    `json:"delta"`  // 变化量
	End    int    `json:"end"`    // 结束数量
	Reason string `json:"reason"` // 变化原因
}

// StoryOutlineScene 章节内场景
type StoryOutlineScene struct {
	Order      int      `json:"order"`           // 场景序号
	POV        string   `json:"pov"`             // 视角角色
	Goal       string   `json:"goal"`            // 场景目标
	Location   string   `json:"location"`        // 场景地点
	Characters []string `json:"characters"`      // 出场角色
	Words      int      `json:"words,omitempty"` // 建议字数
	Tone       string   `json:"tone,omitempty"`  // 情绪基调
	Beats      []string `json:"beats,omitempty"` // Scene plot beats
}

// StoryMysteryPlanted 新埋下的线索
type StoryMysteryPlanted struct {
	ID   string `json:"id"`
	Clue string `json:"clue"`
}

// StoryMysteryResolved 回收的伏笔
type StoryMysteryResolved struct {
	ID         string `json:"id"`
	Resolution string `json:"resolution"`
}

// StoryChapterMysteries 本章谜题
type StoryChapterMysteries struct {
	Planted  []StoryMysteryPlanted  `json:"planted,omitempty"`
	Resolved []StoryMysteryResolved `json:"resolved,omitempty"`
}

// StorylineAdvance is an optional note for how a chapter moves a setup storyline.
type StorylineAdvance struct {
	StorylineName string `json:"storyline_name"`
	Stage         string `json:"stage,omitempty"`
	Change        string `json:"change"`
	Consequence   string `json:"consequence,omitempty"`
	Pressure      string `json:"pressure,omitempty"`
}

// StoryChapter 故事章节
type StoryChapter struct {
	ID                string                     `json:"id"`
	Title             string                     `json:"title"`
	Summary           string                     `json:"summary"`
	Characters        []string                   `json:"characters"`
	Location          string                     `json:"location"`
	Events            []StoryEvent               `json:"events"`
	Beats             []string                   `json:"beats"`
	StateChange       string                     `json:"state_change"`
	Conflict          string                     `json:"conflict"`
	Pacing            string                     `json:"pacing"`
	Timeline          StoryChapterTimeline       `json:"timeline,omitempty"`           // 时间线信息
	StateAnchor       StoryStateAnchor           `json:"state_anchor,omitempty"`       // 状态锚点
	Enemies           []StoryOutlineEnemy        `json:"enemies,omitempty"`            // 敌人清单
	ResourceLedger    []StoryResourceLedgerEntry `json:"resource_ledger,omitempty"`    // 资源账本
	Scenes            []StoryOutlineScene        `json:"scenes,omitempty"`             // 场景拆分
	Mysteries         StoryChapterMysteries      `json:"mysteries,omitempty"`          // 伏笔/谜题
	StorylineAdvances []StorylineAdvance         `json:"storyline_advances,omitempty"` // 故事线推进提示
}

// StoryEvent 故事事件
type StoryEvent struct {
	// 旧字段（向后兼容）
	Type       string   `json:"type"`
	Characters []string `json:"characters"`
	Subject    string   `json:"subject"`
	Change     string   `json:"change"`
	Details    string   `json:"details"`

	// 新字段（推荐，语义更清晰）
	Actor      string `json:"actor,omitempty"`
	Action     string `json:"action,omitempty"`
	Target     string `json:"target,omitempty"`
	TargetType string `json:"target_type,omitempty"`
	Context    string `json:"context,omitempty"`
	Result     string `json:"result,omitempty"`

	// 战斗事件特殊格式
	Enemies []EnemyInfo `json:"enemies,omitempty"`
	Allies  []string    `json:"allies,omitempty"`
}

// EnemyInfo 敌人信息（用于战斗事件）
type EnemyInfo struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Level  int    `json:"level"`
	IsBoss bool   `json:"is_boss,omitempty"`
}

// GetActor 获取执行者（优先新格式，回退旧格式）
func (e *StoryEvent) GetActor() string {
	if e.Actor != "" {
		return e.Actor
	}
	if len(e.Characters) > 0 {
		return e.Characters[0]
	}
	return ""
}

// GetAction 获取动作（优先新格式，回退旧格式）
func (e *StoryEvent) GetAction() string {
	if e.Action != "" {
		return e.Action
	}
	// 从旧格式推断
	return inferActionFromOldFormat(e.Type, e.Change)
}

// GetTarget 获取目标（优先新格式，回退旧格式）
func (e *StoryEvent) GetTarget() string {
	if e.Target != "" {
		return e.Target
	}
	return e.Subject
}

// GetTargetType 获取目标类型（优先新格式，回退旧格式）
func (e *StoryEvent) GetTargetType() string {
	if e.TargetType != "" {
		return e.TargetType
	}
	return inferTargetTypeFromOldFormat(e.Type)
}

// inferActionFromOldFormat 从旧格式推断动作
func inferActionFromOldFormat(eventType, change string) string {
	switch change {
	case "acquired", "get", "获得":
		return "acquire"
	case "lost", "失去":
		return "lose"
	case "used", "使用":
		return "use"
	case "discovered", "发现":
		return "discover"
	case "awakened", "awaken", "觉醒":
		return "awaken"
	case "upgraded", "升级":
		return "upgrade"
	case "completed", "完成":
		return "achieve"
	case "started", "开始":
		return "set"
	case "progressed", "推进":
		return "progress"
	default:
		switch eventType {
		case "item":
			return "acquire"
		case "status":
			return "transform"
		case "premise":
			return "awaken"
		case "combat":
			return "combat"
		default:
			return "discover"
		}
	}
}

// inferTargetTypeFromOldFormat 从旧格式推断目标类型
func inferTargetTypeFromOldFormat(eventType string) string {
	switch eventType {
	case "item":
		return "item"
	case "status":
		return "status"
	case "relationship":
		return "relationship"
	case "goal":
		return "goal"
	case "premise":
		return "premise"
	case "storyline":
		return "storyline"
	default:
		return ""
	}
}

// StoryWorld 故事世界 - 将小说大纲转换为RPG数据
type StoryWorld struct {
	GameWorld    *GameWorld
	Outline      *StoryOutline
	CharacterMap map[string]string // 小说角色名 -> RPG角色ID
	LocationMap  map[string]string // 小说地点名 -> RPG地图ID
	QuestMap     map[string]string // 章节ID -> RPG任务ID
	EventMap     map[string]string // 章节ID -> RPG事件ID
}

// NewStoryWorld 从大纲创建故事世界
func NewStoryWorld(outlinePath string) (*StoryWorld, error) {
	// 读取大纲文件
	data, err := ioutil.ReadFile(outlinePath)
	if err != nil {
		return nil, fmt.Errorf("读取大纲文件失败: %v", err)
	}

	var outline StoryOutline
	if err := json.Unmarshal(data, &outline); err != nil {
		return nil, fmt.Errorf("解析大纲JSON失败: %v", err)
	}

	// 创建基础游戏世界
	gameWorld := NewGameWorld()

	storyWorld := &StoryWorld{
		GameWorld:    gameWorld,
		Outline:      &outline,
		CharacterMap: make(map[string]string),
		LocationMap:  make(map[string]string),
		QuestMap:     make(map[string]string),
		EventMap:     make(map[string]string),
	}

	// 转换大纲数据到RPG系统
	storyWorld.ConvertOutlineToRPG()

	return storyWorld, nil
}

// ConvertOutlineToRPG 将大纲转换为RPG数据
func (sw *StoryWorld) ConvertOutlineToRPG() {
	// 0. 初始化技能
	InitDefaultSkills(sw.GameWorld.Skills)

	// 1. 转换地点
	sw.convertLocations()

	// 2. 转换角色
	sw.convertCharacters()

	// 3. 转换任务（章节）
	sw.convertQuests()

	// 4. 转换事件
	sw.convertEvents()
}

// convertLocations 转换地点
func (sw *StoryWorld) convertLocations() {
	locationSet := make(map[string]bool)

	// 收集所有地点
	for _, part := range sw.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				if chapter.Location != "" {
					locationSet[chapter.Location] = true
				}
			}
		}
	}

	// 为每个地点创建地图
	for location := range locationSet {
		mapID := sw.generateMapID(location)
		sw.LocationMap[location] = mapID

		// 根据地点名称判断地图类型
		mapType := sw.inferMapType(location)

		gameMap := &Map{
			ID:          mapID,
			Name:        location,
			Description: location,
			Type:        mapType,
			Width:       20,
			Height:      15,
			TileSize:    32,
			Entities:    make([]MapEntity, 0),
			Teleports:   make([]TeleportPoint, 0),
			Regions:     make([]MapRegion, 0),
			Connections: make([]MapConnection, 0),
			LightLevel:  100,
		}

		sw.GameWorld.Maps.AddMap(gameMap)
	}
}

// inferMapType 根据地点名称推断地图类型
func (sw *StoryWorld) inferMapType(location string) MapType {
	switch {
	case stringContainsAny(location, []string{"矿", "洞", "穴"}):
		return MapTypeCave
	case stringContainsAny(location, []string{"城", "镇", "村"}):
		return MapTypeTown
	case stringContainsAny(location, []string{"林", "森", "木"}):
		return MapTypeForest
	case stringContainsAny(location, []string{"山", "峰", "岭"}):
		return MapTypeMountain
	default:
		return MapTypeField
	}
}

// convertCharacters 转换角色
func (sw *StoryWorld) convertCharacters() {
	characterSet := make(map[string]bool)
	enemySet := make(map[string]bool)

	// 收集所有角色
	for _, part := range sw.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, char := range chapter.Characters {
					characterSet[char] = true
				}
				for _, event := range chapter.Events {
					for _, char := range event.Characters {
						characterSet[char] = true
					}
					// 收集战斗目标（敌人）- 优先使用新格式
					action := event.GetAction()
					if action == "combat" || action == "defeat" || action == "kill" {
						// 新格式：使用 enemies 字段
						if len(event.Enemies) > 0 {
							for _, enemy := range event.Enemies {
								enemySet[enemy.Name] = true
							}
						} else if event.Target != "" && event.TargetType == "character" {
							// 旧格式：使用 target 字段
							enemySet[event.Target] = true
						}
					}
				}
			}
		}
	}

	// 为每个角色创建模板
	for charName := range characterSet {
		templateID := sw.generateCharacterTemplateID(charName)
		charID := sw.generateCharacterID(charName)
		sw.CharacterMap[charName] = charID

		// 根据角色名称判断类型和属性
		charType, baseStats := sw.inferCharacterStats(charName)

		template := &CharacterTemplate{
			ID:          templateID,
			Name:        charName,
			Type:        charType,
			BaseStats:   baseStats,
			GrowthStats: sw.inferGrowthStats(charName),
			Rarity:      sw.inferRarity(charName),
		}

		sw.GameWorld.Characters.AddTemplate(template)

		// 创建角色实例
		character := sw.GameWorld.CreateCharacter(templateID, charName)
		if character != nil {
			// 设置初始位置
			if len(sw.Outline.Parts) > 0 && len(sw.Outline.Parts[0].Volumes) > 0 &&
				len(sw.Outline.Parts[0].Volumes[0].Chapters) > 0 {
				firstChapter := sw.Outline.Parts[0].Volumes[0].Chapters[0]
				if firstChapter.Location != "" {
					if mapID, ok := sw.LocationMap[firstChapter.Location]; ok {
						character.Position.MapID = mapID
						character.Position.X = 5
						character.Position.Y = 5
					}
				}
			}

			// 如果是主角，设置为玩家并添加技能
			if charName == "林砚" || charName == "林跃" {
				sw.GameWorld.SetPlayer(character)
				// 给主角添加基础技能
				character.Skills = []string{"skill_quick_strike", "skill_power_strike"}
			}
		}
	}

	// 为敌人创建角色
	for enemyName := range enemySet {
		// 跳过已存在的角色
		if _, exists := sw.CharacterMap[enemyName]; exists {
			continue
		}

		templateID := sw.generateCharacterTemplateID(enemyName)
		charID := sw.generateCharacterID(enemyName)
		sw.CharacterMap[enemyName] = charID

		// 敌人类型和属性
		charType, baseStats := sw.inferEnemyStats(enemyName)

		template := &CharacterTemplate{
			ID:          templateID,
			Name:        enemyName,
			Type:        charType,
			BaseStats:   baseStats,
			GrowthStats: sw.inferEnemyGrowthStats(enemyName),
			Rarity:      sw.inferRarity(enemyName),
		}

		sw.GameWorld.Characters.AddTemplate(template)

		// 手动创建角色实例，使用我们指定的ID
		character := &Character{
			ID:           charID,
			TemplateID:   templateID,
			Name:         enemyName,
			Type:         charType,
			Level:        1,
			Exp:          0,
			ExpToNext:    100,
			BaseStats:    baseStats,
			CurrentStats: baseStats,
			GrowthStats:  sw.inferEnemyGrowthStats(enemyName),
			State:        CharacterStateNormal,
			Skills:       sw.getEnemySkills(enemyName),
			Flags:        make(map[string]interface{}),
		}

		// 设置初始位置（随机地图）
		allMaps := sw.GameWorld.Maps.GetAllMaps()
		if len(allMaps) > 0 {
			character.Position.MapID = allMaps[0].ID
			character.Position.X = 10
			character.Position.Y = 10
		}

		sw.GameWorld.Characters.AddCharacterInstance(character)
	}
}

// inferCharacterStats 推断角色属性
func (sw *StoryWorld) inferCharacterStats(name string) (CharacterType, BaseStats) {
	switch name {
	case "林砚", "林跃":
		// 主角 - 平衡型
		return CharacterTypePlayer, BaseStats{
			HP: 100, MP: 50, Attack: 12, Defense: 10,
			Magic: 8, Resistance: 8, Speed: 10, Luck: 10,
		}
	case "矿监周虎":
		// 监工 - 物理型
		return CharacterTypeEnemy, BaseStats{
			HP: 80, MP: 20, Attack: 15, Defense: 8,
			Magic: 3, Resistance: 5, Speed: 8, Luck: 5,
		}
	default:
		// 默认NPC
		return CharacterTypeNPC, BaseStats{
			HP: 50, MP: 30, Attack: 8, Defense: 6,
			Magic: 6, Resistance: 6, Speed: 7, Luck: 5,
		}
	}
}

// inferGrowthStats 推断成长属性
func (sw *StoryWorld) inferGrowthStats(name string) GrowthStats {
	switch name {
	case "林砚", "林跃":
		return GrowthStats{
			HP: 10, MP: 5, Attack: 2, Defense: 1.5,
			Magic: 1.5, Resistance: 1.5, Speed: 2, Luck: 1,
		}
	default:
		return GrowthStats{
			HP: 5, MP: 2, Attack: 1, Defense: 0.8,
			Magic: 0.8, Resistance: 0.8, Speed: 1, Luck: 0.5,
		}
	}
}

// inferEnemyStats 推断敌人属性
func (sw *StoryWorld) inferEnemyStats(name string) (CharacterType, BaseStats) {
	// 根据敌人名称推断属性
	switch {
	case stringContainsAny(name, []string{"虫族", "虫", "蜂", "兽"}):
		// 虫族/野兽 - 高攻击低防御
		return CharacterTypeEnemy, BaseStats{
			HP: 60, MP: 10, Attack: 15, Defense: 5,
			Magic: 2, Resistance: 3, Speed: 12, Luck: 3,
		}
	case stringContainsAny(name, []string{"首领", "王", "将", "帅"}):
		// Boss级敌人 - 高属性
		return CharacterTypeEnemy, BaseStats{
			HP: 150, MP: 50, Attack: 20, Defense: 15,
			Magic: 12, Resistance: 10, Speed: 8, Luck: 5,
		}
	default:
		// 普通敌人
		return CharacterTypeEnemy, BaseStats{
			HP: 40, MP: 15, Attack: 10, Defense: 8,
			Magic: 5, Resistance: 5, Speed: 7, Luck: 4,
		}
	}
}

// inferEnemyGrowthStats 推断敌人成长属性
func (sw *StoryWorld) inferEnemyGrowthStats(name string) GrowthStats {
	switch {
	case stringContainsAny(name, []string{"虫族", "虫", "蜂", "兽"}):
		return GrowthStats{
			HP: 8, MP: 1, Attack: 2, Defense: 0.5,
			Magic: 0.3, Resistance: 0.3, Speed: 1.5, Luck: 0.2,
		}
	case stringContainsAny(name, []string{"首领", "王", "将", "帅"}):
		return GrowthStats{
			HP: 15, MP: 5, Attack: 3, Defense: 2,
			Magic: 1.5, Resistance: 1.2, Speed: 1, Luck: 0.5,
		}
	default:
		return GrowthStats{
			HP: 5, MP: 1.5, Attack: 1.2, Defense: 0.8,
			Magic: 0.5, Resistance: 0.5, Speed: 0.8, Luck: 0.3,
		}
	}
}

// inferRarity 推断稀有度
func (sw *StoryWorld) inferRarity(name string) Rarity {
	switch name {
	case "林砚", "林跃":
		return RarityLegendary
	case "矿监周虎":
		return RarityCommon
	default:
		return RarityCommon
	}
}

// convertQuests 转换任务
func (sw *StoryWorld) convertQuests() {
	for _, part := range sw.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				questID := sw.generateQuestID(chapter.ID)
				sw.QuestMap[chapter.ID] = questID

				// 从章节信息推断任务目标
				objectives := sw.inferQuestObjectives(chapter)

				quest := &Quest{
					ID:            questID,
					Name:          chapter.Title,
					Description:   chapter.Summary,
					Type:          sw.inferQuestType(chapter),
					LevelRequired: sw.inferQuestLevel(chapter),
					Objectives:    objectives,
					Rewards:       sw.inferQuestRewards(chapter),
				}

				sw.GameWorld.Quests.AddQuest(quest)
			}
		}
	}
}

// inferQuestObjectives 推断任务目标
func (sw *StoryWorld) inferQuestObjectives(chapter StoryChapter) []QuestObjective {
	objectives := make([]QuestObjective, 0)

	// 获取章节位置信息
	locationID := ""
	if chapter.Location != "" {
		if mapID, ok := sw.LocationMap[chapter.Location]; ok {
			locationID = mapID
		}
	}

	for i, event := range chapter.Events {
		// 优先使用新格式（action），回退到旧格式（type）
		objType := sw.inferObjectiveTypeFromEvent(event)
		targetID := ""
		targetCount := 1

		// 对于战斗/击败类型的事件，优先使用 Enemies 字段
		if objType == ObjectiveDefeat || objType == ObjectiveKill {
			// 新格式：使用 enemies 字段，选择 Boss 或第一个敌人
			if len(event.Enemies) > 0 {
				// 优先选择 Boss 敌人
				bossEnemy := ""
				firstEnemy := ""
				totalCount := 0
				for _, enemy := range event.Enemies {
					totalCount += enemy.Count
					if firstEnemy == "" {
						firstEnemy = enemy.Name
					}
					if enemy.IsBoss {
						bossEnemy = enemy.Name
					}
				}

				// 使用 Boss 或第一个敌人作为主要目标
				enemyName := bossEnemy
				if enemyName == "" {
					enemyName = firstEnemy
				}

				if charID, ok := sw.CharacterMap[enemyName]; ok {
					targetID = charID
				}
				targetCount = totalCount
			} else if event.Target != "" {
				// 旧格式：使用 target 字段
				if charID, ok := sw.CharacterMap[event.Target]; ok {
					targetID = charID
				}
			}
		}

		// 如果没有找到目标，尝试使用 Actor 或 Characters[0]
		if targetID == "" {
			actor := event.GetActor()
			if actor != "" {
				if charID, ok := sw.CharacterMap[actor]; ok {
					targetID = charID
				}
			} else if len(event.Characters) > 0 {
				if charID, ok := sw.CharacterMap[event.Characters[0]]; ok {
					targetID = charID
				}
			}
		}

		// 获取描述（优先使用 Result，回退到 Details/Change）
		description := event.Result
		if description == "" {
			description = event.Details
		}
		if description == "" {
			// 如果只有 change，构建更丰富的描述
			description = sw.buildDescriptionFromEvent(event)
		}

		// 获取事件上下文位置
		eventLocation := locationID
		if event.Context != "" {
			if ctxMapID, ok := sw.LocationMap[event.Context]; ok {
				eventLocation = ctxMapID
			}
		}

		objective := QuestObjective{
			ID:          fmt.Sprintf("%s_obj_%d", chapter.ID, i),
			Type:        objType,
			Description: description,
			TargetID:    targetID,
			TargetCount: targetCount,
			LocationID:  eventLocation,
		}

		objectives = append(objectives, objective)
	}

	return objectives
}

// buildDescriptionFromEvent 从事件构建描述
func (sw *StoryWorld) buildDescriptionFromEvent(event StoryEvent) string {
	// 优先使用新格式构建描述
	actor := event.GetActor()
	action := event.GetAction()
	target := event.GetTarget()
	context := event.Context

	if actor != "" && action != "" {
		// 构建动作描述
		desc := fmt.Sprintf("%s %s", actor, action)
		if target != "" {
			desc += fmt.Sprintf(" %s", target)
		}
		if context != "" {
			desc += fmt.Sprintf(" (在 %s)", context)
		}
		return desc
	}

	// 回退到旧格式
	if event.Subject != "" && event.Change != "" {
		return fmt.Sprintf("%s: %s", event.Subject, event.Change)
	}

	// 最后回退
	return event.Change
}

// inferObjectiveTypeFromEvent 从事件推断目标类型（支持新旧格式）
func (sw *StoryWorld) inferObjectiveTypeFromEvent(event StoryEvent) QuestObjectiveType {
	// 优先使用新格式的 action 字段
	action := event.GetAction()
	if action != "" {
		switch action {
		case "combat", "defeat", "escape", "defend":
			return ObjectiveDefeat
		case "acquire", "discover", "reveal", "craft":
			return ObjectiveCollect
		case "meet", "befriend", "betray", "reconcile":
			return ObjectiveTalk
		case "move", "enter", "leave", "teleport":
			return ObjectiveReach
		case "learn", "awaken", "upgrade", "master":
			return ObjectiveEvent // 能力觉醒/升级
		case "use", "lose", "transform", "recover", "afflict":
			return ObjectiveEvent // 状态变化
		case "set", "progress", "achieve", "abandon":
			return ObjectiveEvent // 目标变化
		}
	}

	// 回退到旧格式的 type 字段
	return sw.inferObjectiveType(event.Type)
}

// inferObjectiveType 推断目标类型
func (sw *StoryWorld) inferObjectiveType(eventType string) QuestObjectiveType {
	switch eventType {
	case "combat", "battle":
		return ObjectiveDefeat
	case "discovery", "find":
		return ObjectiveCollect
	case "dialogue", "talk":
		return ObjectiveTalk
	case "travel", "move":
		return ObjectiveReach
	default:
		return ObjectiveEvent
	}
}

// inferQuestType 推断任务类型
func (sw *StoryWorld) inferQuestType(chapter StoryChapter) QuestType {
	// 根据章节ID判断
	if len(chapter.ID) > 0 && chapter.ID[0] == 'P' && len(chapter.ID) > 1 {
		if chapter.ID[1] == '1' {
			return QuestTypeMain
		}
	}
	return QuestTypeSide
}

// inferQuestLevel 推断任务等级
func (sw *StoryWorld) inferQuestLevel(chapter StoryChapter) int {
	// 根据章节ID推断等级
	// P1-V1-C1 -> 等级1, P1-V1-C2 -> 等级2, etc.
	level := 1
	if len(chapter.ID) > 0 {
		// 简单推断：根据章节顺序
		for _, part := range sw.Outline.Parts {
			for _, volume := range part.Volumes {
				for i, ch := range volume.Chapters {
					if ch.ID == chapter.ID {
						return level + i
					}
				}
			}
		}
	}
	return level
}

// inferQuestRewards 推断任务奖励
func (sw *StoryWorld) inferQuestRewards(chapter StoryChapter) QuestReward {
	level := sw.inferQuestLevel(chapter)

	return QuestReward{
		Exp:   level * 50,
		Money: level * 20,
		Items: []RewardItem{
			{ItemID: "item_health_potion", Count: level},
		},
	}
}

// convertEvents 转换事件
func (sw *StoryWorld) convertEvents() {
	for _, part := range sw.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				eventID := sw.generateEventID(chapter.ID)
				sw.EventMap[chapter.ID] = eventID

				// 创建事件
				event := &Event{
					ID:   eventID,
					Name: chapter.Title,
				}

				// 如果有地点信息，设置位置
				if chapter.Location != "" {
					if mapID, ok := sw.LocationMap[chapter.Location]; ok {
						gameMap := sw.GameWorld.Maps.GetMap(mapID)
						if gameMap != nil {
							event.X = 10
							event.Y = 10
						}
					}
				}

				// 创建事件页
				page := EventPage{
					ID:      0,
					Trigger: EventTriggerAction,
					List:    sw.convertBeatsToCommands(chapter.Beats),
				}

				event.Pages = []EventPage{page}
				sw.GameWorld.Events.AddEvent(event)

				// 将事件添加到地图
				if chapter.Location != "" {
					if mapID, ok := sw.LocationMap[chapter.Location]; ok {
						sw.GameWorld.Maps.AddEntity(mapID, MapEntity{
							ID:        fmt.Sprintf("entity_%s", eventID),
							Type:      "event",
							EntityID:  eventID,
							Name:      chapter.Title,
							X:         10,
							Y:         10,
							IsVisible: true,
							IsActive:  true,
						})
					}
				}
			}
		}
	}
}

// convertBeatsToCommands 将情节节拍转换为事件命令
func (sw *StoryWorld) convertBeatsToCommands(beats []string) []EventCommand {
	commands := make([]EventCommand, 0)

	for _, beat := range beats {
		// 将每个节拍转换为显示文本命令
		cmd := EventCommand{
			Code:       CmdShowText,
			Parameters: []interface{}{beat},
		}
		commands = append(commands, cmd)
	}

	return commands
}

// 辅助方法

func (sw *StoryWorld) generateMapID(location string) string {
	return fmt.Sprintf("map_%s", sanitizeID(location))
}

func (sw *StoryWorld) generateCharacterTemplateID(name string) string {
	return fmt.Sprintf("template_%s", sanitizeID(name))
}

func (sw *StoryWorld) generateCharacterID(name string) string {
	return fmt.Sprintf("char_%s", sanitizeID(name))
}

func (sw *StoryWorld) generateQuestID(chapterID string) string {
	return fmt.Sprintf("quest_%s", chapterID)
}

func (sw *StoryWorld) generateEventID(chapterID string) string {
	return fmt.Sprintf("event_%s", chapterID)
}

func sanitizeID(s string) string {
	// 保留中文字符、英文字母和数字
	result := ""
	for _, c := range s {
		// 保留中文字符 (Unicode范围: 0x4E00-0x9FFF)
		if (c >= 0x4E00 && c <= 0x9FFF) ||
			(c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '-' {
			result += string(c)
		}
	}
	if result == "" {
		return "unknown"
	}
	return result
}

func stringContainsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// GetStorySummary 获取故事摘要
func (sw *StoryWorld) GetStorySummary() map[string]interface{} {
	return map[string]interface{}{
		"title":        sw.Outline.Parts[0].Title,
		"parts_count":  len(sw.Outline.Parts),
		"characters":   len(sw.CharacterMap),
		"locations":    len(sw.LocationMap),
		"quests":       len(sw.QuestMap),
		"events":       len(sw.EventMap),
		"player_name":  sw.GameWorld.Player.Name,
		"player_level": sw.GameWorld.Player.Level,
		"current_map":  sw.GameWorld.Context.CurrentMap,
	}
}

// ExportToJSON 导出为JSON
func (sw *StoryWorld) ExportToJSON() string {
	data := map[string]interface{}{
		"summary": sw.GetStorySummary(),
		"world":   sw.GameWorld.SaveToJSON(),
		"mappings": map[string]interface{}{
			"characters": sw.CharacterMap,
			"locations":  sw.LocationMap,
			"quests":     sw.QuestMap,
			"events":     sw.EventMap,
		},
	}

	jsonData, _ := json.MarshalIndent(data, "", "  ")
	return string(jsonData)
}

// getEnemySkills 根据敌人名称获取技能
func (sw *StoryWorld) getEnemySkills(enemyName string) []string {
	// 根据敌人类型返回不同技能
	switch {
	case containsAny(enemyName, []string{"虫族", "虫", "蜂"}):
		// 虫族技能
		return []string{"skill_insect_claw", "skill_acid_spray"}
	case containsAny(enemyName, []string{"野兽", "狼", "熊"}):
		// 野兽技能
		return []string{"skill_beast_bite", "skill_ferocious_charge"}
	case containsAny(enemyName, []string{"统领", "高阶", "boss"}):
		// Boss级敌人
		return []string{"skill_powerful_strike", "skill_rage", "skill_area_attack"}
	default:
		// 默认敌人技能
		return []string{"skill_basic_attack"}
	}
}

// containsAny 检查字符串是否包含任意一个子串
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
