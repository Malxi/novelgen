package rpg

import (
	"encoding/json"
	"fmt"
	"strings"
)

// LLMExtractor 使用LLM智能提取小说实体
type LLMExtractor struct {
	client LLMClient
	prompt string
}

// LLMClient LLM客户端接口
type LLMClient interface {
	Complete(prompt string, text string) (string, error)
}

// LLMExtractionResult LLM提取结果
type LLMExtractionResult struct {
	Characters []LLMCharacter `json:"characters"`
	Items      []LLMItem      `json:"items"`
	Skills     []LLMSkill     `json:"skills"`
	Locations  []LLMLocation  `json:"locations"`
	Events     []LLMEvent     `json:"events"`
	Timeline   []LLMTimeline  `json:"timeline"`
	Analysis   LLMAnalysis    `json:"analysis"`
}

// LLMCharacter LLM提取的角色
type LLMCharacter struct {
	Name          string      `json:"name"`
	Type          string      `json:"type"` // protagonist, antagonist, supporting
	Personality   string      `json:"personality"`
	Background    string      `json:"background"`
	Goals         string      `json:"goals"`
	Relationships interface{} `json:"relationships"` // 可以是字符串或数组
	PowerLevel    string      `json:"power_level"`
	Confidence    float64     `json:"confidence"`
}

// LLMRelationship 角色关系
type LLMRelationship struct {
	Name     string `json:"name"`
	Relation string `json:"relation"` // friend, enemy, master, disciple, family, etc.
}

// LLMItem LLM提取的物品
type LLMItem struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`   // weapon, armor, consumable, treasure, material
	Rarity      string  `json:"rarity"` // common, uncommon, rare, epic, legendary
	Description string  `json:"description"`
	Effects     string  `json:"effects"`
	Owner       string  `json:"owner"`
	Confidence  float64 `json:"confidence"`
}

// LLMSkill LLM提取的技能
type LLMSkill struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"` // combat, cultivation, support, passive, system
	Description string  `json:"description"`
	PowerLevel  string  `json:"power_level"`
	Owner       string  `json:"owner"`
	Confidence  float64 `json:"confidence"`
}

// LLMLocation LLM提取的地点
type LLMLocation struct {
	Name            string      `json:"name"`
	Type            string      `json:"type"` // city, dungeon, sect, wilderness, building
	Description     string      `json:"description"`
	ConnectedTo     interface{} `json:"connected_to"`     // 可以是字符串或数组
	ImportantEvents interface{} `json:"important_events"` // 可以是字符串或数组
	Confidence      float64     `json:"confidence"`
}

// LLMEvent LLM提取的事件
type LLMEvent struct {
	Type         string      `json:"type"`         // battle, breakthrough, death, resurrection, meeting, betrayal, discovery
	Participants interface{} `json:"participants"` // 可以是字符串或数组
	Location     interface{} `json:"location"`     // 可以是字符串或对象
	Time         string      `json:"time"`
	Description  string      `json:"description"`
	Consequences string      `json:"consequences"`
	Confidence   float64     `json:"confidence"`
}

// LLMTimeline 时间线事件
type LLMTimeline struct {
	Day        interface{} `json:"day"`        // 可以是字符串或数字
	Chapter    interface{} `json:"chapter"`    // 可以是字符串或数字
	Events     interface{} `json:"events"`     // 可以是字符串或数组
	Location   string      `json:"location"`
	Characters interface{} `json:"characters"` // 可以是字符串或数组
}

// LLMAnalysis LLM分析结果
type LLMAnalysis struct {
	PlotSummary     string      `json:"plot_summary"`
	PowerSystem     string      `json:"power_system"`
	PotentialIssues interface{} `json:"potential_issues"` // 可以是字符串或数组
}

// NewLLMExtractor 创建LLM提取器
func NewLLMExtractor(client LLMClient) *LLMExtractor {
	return &LLMExtractor{
		client: client,
		prompt: buildExtractionPrompt(),
	}
}

// SetPrompt 设置自定义提示词
func (le *LLMExtractor) SetPrompt(prompt string) {
	le.prompt = prompt
}

// ExtractFromNovel 从小说文本中提取实体
func (le *LLMExtractor) ExtractFromNovel(text string) (*NovelRPGData, error) {
	// 分段处理长文本
	chunks := le.splitText(text, 4000)

	var allResults []LLMExtractionResult
	for i, chunk := range chunks {
		result, err := le.extractChunk(chunk, i+1, len(chunks))
		if err != nil {
			return nil, fmt.Errorf("提取第%d段失败: %w", i+1, err)
		}
		allResults = append(allResults, *result)
	}

	// 合并结果
	merged := le.mergeResults(allResults)

	// 转换为系统格式
	return le.convertToSystemFormat(merged), nil
}

// extractChunk 提取单个文本块
func (le *LLMExtractor) extractChunk(chunk string, chunkNum, totalChunks int) (*LLMExtractionResult, error) {
	fullPrompt := fmt.Sprintf(`%s

这是小说的第%d/%d部分，请提取实体：

%s`, le.prompt, chunkNum, totalChunks, chunk)

	response, err := le.client.Complete(fullPrompt, chunk)
	if err != nil {
		return nil, err
	}

	// 解析JSON响应
	var result LLMExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// 尝试从响应中提取JSON
		jsonStr := extractJSON(response)
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return nil, fmt.Errorf("解析LLM响应失败: %w\n响应: %s", err, response)
		}
	}

	return &result, nil
}

// splitText 将文本分割成块
func (le *LLMExtractor) splitText(text string, chunkSize int) []string {
	var chunks []string
	lines := strings.Split(text, "\n")

	var currentChunk strings.Builder
	for _, line := range lines {
		if currentChunk.Len()+len(line) > chunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// mergeResults 合并多个提取结果
func (le *LLMExtractor) mergeResults(results []LLMExtractionResult) LLMExtractionResult {
	merged := LLMExtractionResult{
		Characters: make([]LLMCharacter, 0),
		Items:      make([]LLMItem, 0),
		Skills:     make([]LLMSkill, 0),
		Locations:  make([]LLMLocation, 0),
		Events:     make([]LLMEvent, 0),
		Timeline:   make([]LLMTimeline, 0),
	}

	// 使用map去重
	charMap := make(map[string]LLMCharacter)
	itemMap := make(map[string]LLMItem)
	skillMap := make(map[string]LLMSkill)
	locMap := make(map[string]LLMLocation)
	eventMap := make(map[string]LLMEvent)

	for _, result := range results {
		for _, char := range result.Characters {
			if existing, ok := charMap[char.Name]; ok {
				// 合并信息
				char = mergeCharacter(existing, char)
			}
			charMap[char.Name] = char
		}

		for _, item := range result.Items {
			if existing, ok := itemMap[item.Name]; ok {
				item = mergeItem(existing, item)
			}
			itemMap[item.Name] = item
		}

		for _, skill := range result.Skills {
			if existing, ok := skillMap[skill.Name]; ok {
				skill = mergeSkill(existing, skill)
			}
			skillMap[skill.Name] = skill
		}

		for _, loc := range result.Locations {
			if existing, ok := locMap[loc.Name]; ok {
				loc = mergeLocation(existing, loc)
			}
			locMap[loc.Name] = loc
		}

		for _, event := range result.Events {
			participants := interfaceToStringSlice(event.Participants)
			key := event.Type + "_" + strings.Join(participants, "_")
			if existing, ok := eventMap[key]; ok {
				event = mergeEvent(existing, event)
			}
			eventMap[key] = event
		}

		merged.Timeline = append(merged.Timeline, result.Timeline...)
	}

	// 转换回slice
	for _, char := range charMap {
		merged.Characters = append(merged.Characters, char)
	}
	for _, item := range itemMap {
		merged.Items = append(merged.Items, item)
	}
	for _, skill := range skillMap {
		merged.Skills = append(merged.Skills, skill)
	}
	for _, loc := range locMap {
		merged.Locations = append(merged.Locations, loc)
	}
	for _, event := range eventMap {
		merged.Events = append(merged.Events, event)
	}

	return merged
}

// convertToSystemFormat 转换为系统格式
func (le *LLMExtractor) convertToSystemFormat(result LLMExtractionResult) *NovelRPGData {
	data := &NovelRPGData{
		Characters:       make([]*CharacterTemplate, 0),
		Items:            make([]*Item, 0),
		Skills:           make([]*Skill, 0),
		Locations:        make([]*Map, 0),
		Events:           make([]*Event, 0),
		Quests:           make([]*Quest, 0),
		Timeline:         make([]*TimelineEvent, 0),
		ValidationIssues: make([]ValidationIssue, 0),
	}

	// 转换角色
	for _, char := range result.Characters {
		charType := CharacterTypeNPC
		switch char.Type {
		case "protagonist":
			charType = CharacterTypePlayer
		case "antagonist":
			charType = CharacterTypeEnemy
		}

		template := &CharacterTemplate{
			ID:          string(NewID("char")),
			Name:        char.Name,
			Description: char.Background + " " + char.Goals,
			Type:        charType,
			BaseStats: BaseStats{
				HP:         100 * parsePowerLevel(char.PowerLevel),
				MP:         100 * parsePowerLevel(char.PowerLevel),
				Attack:     10 * parsePowerLevel(char.PowerLevel),
				Magic:      10 * parsePowerLevel(char.PowerLevel),
				Defense:    5 * parsePowerLevel(char.PowerLevel),
				Resistance: 5 * parsePowerLevel(char.PowerLevel),
				Speed:      10,
				Luck:       5,
			},
			Rarity: RarityCommon,
		}
		data.Characters = append(data.Characters, template)
	}

	// 转换物品
	for _, item := range result.Items {
		itemType := ItemTypeMisc
		switch item.Type {
		case "consumable":
			itemType = ItemTypeConsumable
		case "material":
			itemType = ItemTypeMaterial
		case "quest":
			itemType = ItemTypeQuest
		case "key":
			itemType = ItemTypeKey
		}

		itemObj := &Item{
			ID:          string(NewID("item")),
			Name:        item.Name,
			Type:        itemType,
			Description: item.Description,
			Rarity:      Rarity(fmt.Sprintf("%d", parseRarity(item.Rarity))),
			Weight:      1.0,
			MaxStack:    99,
			Value:       100 * parseRarity(item.Rarity),
			IsUsable:    itemType == ItemTypeConsumable,
			IsDroppable: true,
			IsSellable:  true,
		}
		data.Items = append(data.Items, itemObj)
	}

	// 转换技能
	for _, skill := range result.Skills {
		skillType := SkillTypeActive
		switch skill.Type {
		case "passive":
			skillType = SkillTypePassive
		case "reaction":
			skillType = SkillTypeReaction
		case "ultimate":
			skillType = SkillTypeUltimate
		}

		skillObj := &Skill{
			ID:            string(NewID("skill")),
			Name:          skill.Name,
			Type:          skillType,
			Description:   skill.Description,
			LevelRequired: 1,
			Cost: SkillCost{
				MP: int(parsePowerCost(skill.PowerLevel)),
			},
			Cooldown: 0,
			Target:   SkillTargetSingle,
			MaxLevel: 10,
		}
		data.Skills = append(data.Skills, skillObj)
	}

	// 转换地点
	for _, loc := range result.Locations {
		mapType := MapTypeField
		switch loc.Type {
		case "town":
			mapType = MapTypeTown
		case "dungeon":
			mapType = MapTypeDungeon
		case "castle":
			mapType = MapTypeCastle
		case "forest":
			mapType = MapTypeForest
		case "mountain":
			mapType = MapTypeMountain
		case "cave":
			mapType = MapTypeCave
		case "desert":
			mapType = MapTypeDesert
		case "ocean":
			mapType = MapTypeOcean
		case "special":
			mapType = MapTypeSpecial
		}

		mapObj := &Map{
			ID:          string(NewID("map")),
			Name:        loc.Name,
			Description: loc.Description,
			Type:        mapType,
			Width:       50,
			Height:      50,
			TileSize:    32,
			Entities:    make([]MapEntity, 0),
			Teleports:   make([]TeleportPoint, 0),
			Regions:     make([]MapRegion, 0),
			Connections: make([]MapConnection, 0),
			LightLevel:  100,
		}
		data.Locations = append(data.Locations, mapObj)
	}

	// 转换事件
	for _, event := range result.Events {
		eventObj := &Event{
			ID:   string(NewID("event")),
			Name: event.Type,
			X:    0,
			Y:    0,
			Pages: []EventPage{
				{
					Conditions: EventConditions{},
					List: []EventCommand{
						{
							Code:       CmdShowText,
							Parameters: []interface{}{event.Description},
						},
					},
				},
			},
		}
		data.Events = append(data.Events, eventObj)
	}

	// 转换时间线
	for _, tl := range result.Timeline {
		timelineEvent := &TimelineEvent{
			Day:        interfaceToInt(tl.Day),
			Characters: interfaceToStringSlice(tl.Characters),
			Location:   tl.Location,
			Events:     interfaceToStringSlice(tl.Events),
		}
		data.Timeline = append(data.Timeline, timelineEvent)
	}

	return data
}

// buildExtractionPrompt 构建提取提示词
func buildExtractionPrompt() string {
	return `你是一个专业的小说分析助手，擅长从小说文本中提取RPG游戏实体。

请仔细分析提供的小说文本，提取以下信息并以JSON格式返回：

## 提取要求

### 1. 角色 (characters)
- 识别所有重要角色
- 判断角色类型：protagonist(主角), antagonist(反派), supporting(配角)
- 提取性格特征、背景故事、目标动机
- 识别角色之间的关系
- 标注修为等级/战力水平
- 给出置信度 (0.0-1.0)

### 2. 物品 (items)
- 识别重要物品、装备、道具
- 判断类型：weapon(武器), armor(防具), consumable(消耗品), treasure(宝物), material(材料)
- 标注稀有度：common(普通), uncommon(精良), rare(稀有), epic(史诗), legendary(传说)
- 描述物品效果和用途
- 标注拥有者
- 给出置信度

### 3. 技能 (skills)
- 识别功法、武技、神通、金手指等
- 判断类型：combat(战斗), cultivation(修炼), support(辅助), passive(被动), system(系统)
- 描述技能效果
- 标注威力等级
- 标注拥有者
- 给出置信度

### 4. 地点 (locations)
- 识别重要场景、地点
- 判断类型：city(城市), dungeon(副本), sect(宗门), wilderness(野外), building(建筑)
- 描述地点特征
- 识别关联地点
- 标注重要事件
- 给出置信度

### 5. 事件 (events)
- 识别重要剧情事件
- 判断类型：battle(战斗), breakthrough(突破), death(死亡), resurrection(复活), meeting(相遇), betrayal(背叛), discovery(发现)
- 标注参与者
- 标注地点
- 描述事件经过和后果
- 给出置信度

### 6. 时间线 (timeline)
- 按时间顺序整理事件
- 标注天数/章节
- 列出当天发生的事件
- 标注参与角色

### 7. 分析 (analysis)
- plot_summary: 故事主线概述
- power_system: 战力体系分析
- potential_issues: 潜在问题列表

## 输出格式要求

**重要：必须返回有效的、严格的JSON格式，遵循以下规则：**

1. **所有字符串必须用双引号包裹**
2. **数组和对象的最后一个元素后面不能有加逗号**
3. **不要包含任何注释或说明文字**
4. **确保JSON可以被标准JSON解析器正确解析**

**返回格式示例：**

{
  "characters": [
    {
      "name": "角色名",
      "type": "protagonist",
      "personality": "性格描述",
      "background": "背景故事",
      "goals": "目标",
      "power_level": "练气期",
      "confidence": 0.9
    }
  ],
  "items": [
    {
      "name": "物品名",
      "type": "weapon",
      "rarity": "common",
      "description": "物品描述",
      "owner": "拥有者",
      "confidence": 0.8
    }
  ],
  "skills": [],
  "locations": [],
  "events": [],
  "timeline": [],
  "analysis": {
    "plot_summary": "故事概述",
    "power_system": "战力体系",
    "potential_issues": []
  }
}

**注意：只返回JSON，不要包含 markdown 代码块标记或其他说明文字。**
`
}

// extractJSON 从文本中提取JSON
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 && end > start {
		return text[start : end+1]
	}
	return text
}

// mergeCharacter 合并角色信息
func mergeCharacter(existing, new LLMCharacter) LLMCharacter {
	if new.Confidence > existing.Confidence {
		existing = new
	}
	// 合并关系 - 处理 interface{} 类型
	// 暂时跳过关系合并，因为 relationships 可能是字符串或数组
	return existing
}

// mergeItem 合并物品信息
func mergeItem(existing, new LLMItem) LLMItem {
	if new.Confidence > existing.Confidence {
		return new
	}
	return existing
}

// mergeSkill 合并技能信息
func mergeSkill(existing, new LLMSkill) LLMSkill {
	if new.Confidence > existing.Confidence {
		return new
	}
	return existing
}

// mergeLocation 合并地点信息
func mergeLocation(existing, new LLMLocation) LLMLocation {
	if new.Confidence > existing.Confidence {
		existing = new
	}
	// 合并关联地点 - 暂时跳过，因为 ConnectedTo 是 interface{}
	return existing
}

// mergeEvent 合并事件信息
func mergeEvent(existing, new LLMEvent) LLMEvent {
	if new.Confidence > existing.Confidence {
		return new
	}
	return existing
}

// interfaceToStringSlice 将 interface{} 转换为 []string
func interfaceToStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}

	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		return []string{val}
	default:
		return []string{}
	}
}

// interfaceToInt 将 interface{} 转换为 int
func interfaceToInt(v interface{}) int {
	if v == nil {
		return 0
	}

	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		// 尝试解析字符串中的数字
		var result int
		fmt.Sscanf(val, "%d", &result)
		return result
	default:
		return 0
	}
}

// parsePowerLevel 解析战力等级为数字
func parsePowerLevel(level string) int {
	levels := map[string]int{
		"练气": 1, "筑基": 2, "金丹": 3, "元婴": 4, "化神": 5,
		"合体": 6, "大乘": 7, "渡劫": 8, "仙人": 9,
		"后天": 1, "先天": 2, "宗师": 3, "大宗师": 4,
		"武王": 5, "武皇": 6, "武帝": 7, "武圣": 8, "武神": 9,
		"斗者": 1, "斗师": 2, "大斗师": 3, "斗灵": 4,
		"斗王": 5, "斗皇": 6, "斗宗": 7, "斗尊": 8, "斗圣": 9, "斗帝": 10,
	}
	for key, val := range levels {
		if strings.Contains(level, key) {
			return val
		}
	}
	return 1
}

// parseRarity 解析稀有度
func parseRarity(rarity string) int {
	switch rarity {
	case "legendary":
		return 5
	case "epic":
		return 4
	case "rare":
		return 3
	case "uncommon":
		return 2
	default:
		return 1
	}
}

// parsePowerCost 解析威力消耗
func parsePowerCost(level string) float64 {
	levels := map[string]float64{
		"低": 10, "中": 50, "高": 100,
		"weak": 10, "normal": 50, "strong": 100,
	}
	for key, val := range levels {
		if strings.Contains(level, key) {
			return val
		}
	}
	return 30
}

// MockLLMClient 模拟LLM客户端（用于测试）
type MockLLMClient struct {
	Response string
}

// Complete 模拟完成请求
func (m *MockLLMClient) Complete(prompt string, text string) (string, error) {
	if m.Response != "" {
		return m.Response, nil
	}

	// 返回示例响应
	return `{
  "characters": [
    {
      "name": "示例角色",
      "type": "protagonist",
      "personality": "坚毅",
      "background": "穿越者",
      "goals": "成为最强",
      "relationships": [],
      "power_level": "练气期",
      "confidence": 0.9
    }
  ],
  "items": [],
  "skills": [],
  "locations": [],
  "events": [],
  "timeline": [],
  "analysis": {
    "plot_summary": "示例故事",
    "power_system": "修仙体系",
    "potential_issues": []
  }
}`, nil
}
