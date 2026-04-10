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
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Summary  string        `json:"summary"`
	Volumes  []StoryVolume `json:"volumes"`
}

// StoryVolume 故事卷
type StoryVolume struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Summary   string         `json:"summary"`
	Chapters  []StoryChapter `json:"chapters"`
}

// StoryChapter 故事章节
type StoryChapter struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary"`
	Characters   []string       `json:"characters"`
	Location     string         `json:"location"`
	Events       []StoryEvent   `json:"events"`
	Beats        []string       `json:"beats"`
	OpeningBeat  string         `json:"opening_beat"`
	ClosingBeat  string         `json:"closing_beat"`
	StateChange  string         `json:"state_change"`
	Conflict     string         `json:"conflict"`
	Pacing       string         `json:"pacing"`
}

// StoryEvent 故事事件
type StoryEvent struct {
	Type       string   `json:"type"`
	Characters []string `json:"characters"`
	Subject    string   `json:"subject"`
	Change     string   `json:"change"`
	Details    string   `json:"details"`
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

			// 如果是主角，设置为玩家
			if charName == "林砚" {
				sw.GameWorld.SetPlayer(character)
			}
		}
	}
}

// inferCharacterStats 推断角色属性
func (sw *StoryWorld) inferCharacterStats(name string) (CharacterType, BaseStats) {
	switch name {
	case "林砚":
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
	case "林砚":
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

// inferRarity 推断稀有度
func (sw *StoryWorld) inferRarity(name string) Rarity {
	switch name {
	case "林砚":
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
					ID:          questID,
					Name:        chapter.Title,
					Description: chapter.Summary,
					Type:        sw.inferQuestType(chapter),
					LevelRequired: sw.inferQuestLevel(chapter),
					Objectives:  objectives,
					Rewards:     sw.inferQuestRewards(chapter),
				}

				sw.GameWorld.Quests.AddQuest(quest)
			}
		}
	}
}

// inferQuestObjectives 推断任务目标
func (sw *StoryWorld) inferQuestObjectives(chapter StoryChapter) []QuestObjective {
	objectives := make([]QuestObjective, 0)

	for i, event := range chapter.Events {
		objType := sw.inferObjectiveType(event.Type)
		targetID := ""

		// 尝试找到对应的角色ID
		if len(event.Characters) > 0 {
			if charID, ok := sw.CharacterMap[event.Characters[0]]; ok {
				targetID = charID
			}
		}

		objective := QuestObjective{
			ID:          fmt.Sprintf("%s_obj_%d", chapter.ID, i),
			Type:        objType,
			Description: event.Change,
			TargetID:    targetID,
			TargetCount: 1,
		}

		objectives = append(objectives, objective)
	}

	return objectives
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
		"title":           sw.Outline.Parts[0].Title,
		"parts_count":     len(sw.Outline.Parts),
		"characters":      len(sw.CharacterMap),
		"locations":       len(sw.LocationMap),
		"quests":          len(sw.QuestMap),
		"events":          len(sw.EventMap),
		"player_name":     sw.GameWorld.Player.Name,
		"player_level":    sw.GameWorld.Player.Level,
		"current_map":     sw.GameWorld.Context.CurrentMap,
	}
}

// ExportToJSON 导出为JSON
func (sw *StoryWorld) ExportToJSON() string {
	data := map[string]interface{}{
		"summary":   sw.GetStorySummary(),
		"world":     sw.GameWorld.SaveToJSON(),
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
