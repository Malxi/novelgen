package rpg

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// AIExtractor AI驱动的提取器
type AIExtractor struct {
	textExtractor *TextExtractor
	context       *ExtractionContext
}

// ExtractionContext 提取上下文
type ExtractionContext struct {
	CurrentChapter    string
	CurrentSection    string
	CurrentCharacters []string
	CurrentLocation   string
	Timeline          map[int]*TimelineEvent
	CharacterArcs     map[string]*CharacterArc
	PowerSystem       *PowerSystemContext
}

// TimelineEvent 时间线事件
type TimelineEvent struct {
	Time         int            `json:"time"`
	Day          int            `json:"day"`
	Characters   []string       `json:"characters"`
	Location     string         `json:"location"`
	Events       []string       `json:"events"`
	PowerChanges map[string]int `json:"power_changes"`
}

// CharacterArc 角色弧线
type CharacterArc struct {
	Name           string   `json:"name"`
	FirstAppear    int      `json:"first_appear"`
	AppearCount    int      `json:"appear_count"`
	DeathCount     int      `json:"death_count"`
	ResurrectCount int      `json:"resurrect_count"`
	PowerChanges   []int    `json:"power_changes"`
	Relationships  []string `json:"relationships"`
	KeyEvents      []string `json:"key_events"`
}

// PowerSystemContext 战力系统上下文
type PowerSystemContext struct {
	Levels            []string       `json:"levels"`
	LevelOrder        map[string]int `json:"level_order"`
	PowerChanges      int            `json:"power_changes"`
	ResurrectionCount int            `json:"resurrection_count"`
	Inconsistencies   []string       `json:"inconsistencies"`
}

// NewAIExtractor 创建AI提取器
func NewAIExtractor() *AIExtractor {
	return &AIExtractor{
		textExtractor: NewTextExtractor(),
		context: &ExtractionContext{
			Timeline:      make(map[int]*TimelineEvent),
			CharacterArcs: make(map[string]*CharacterArc),
			PowerSystem: &PowerSystemContext{
				Levels: []string{
					"练气", "筑基", "金丹", "元婴", "化神", "合体", "大乘", "渡劫",
				},
				LevelOrder: map[string]int{
					"练气": 1, "筑基": 2, "金丹": 3, "元婴": 4,
					"化神": 5, "合体": 6, "大乘": 7, "渡劫": 8,
				},
				PowerChanges:      0,
				ResurrectionCount: 0,
				Inconsistencies:   make([]string, 0),
			},
		},
	}
}

// ExtractFromNovel 从小说文本中提取完整RPG数据
func (ai *AIExtractor) ExtractFromNovel(text string) *NovelRPGData {
	// 1. 基础实体提取
	entities := ai.textExtractor.ExtractFromText(text)

	// 2. 语义分析 - 理解上下文
	ai.analyzeSemantics(text)

	// 3. 构建时间线
	ai.buildTimeline(text)

	// 4. 分析角色弧线
	ai.analyzeCharacterArcs(entities)

	// 5. 检测战力系统问题
	ai.analyzePowerSystem(entities)

	// 6. 转换为RPG数据
	return ai.convertToRPGData(entities)
}

// NovelRPGData 小说RPG数据
type NovelRPGData struct {
	Characters       []*CharacterTemplate `json:"characters"`
	Items            []*Item              `json:"items"`
	Skills           []*Skill             `json:"skills"`
	Locations        []*Map               `json:"locations"`
	Events           []*Event             `json:"events"`
	Quests           []*Quest             `json:"quests"`
	Timeline         []*TimelineEvent     `json:"timeline"`
	ValidationIssues []ValidationIssue    `json:"validation_issues"`
}

// ValidationIssue 验证问题
type ValidationIssue struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"` // fatal, critical, warning, info
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
	Location   string `json:"location"`
}

// analyzeSemantics 语义分析
func (ai *AIExtractor) analyzeSemantics(text string) {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		// 检测章节标题
		if ai.isChapterTitle(line) {
			ai.context.CurrentChapter = strings.TrimSpace(line)
			continue
		}

		// 检测小节标题
		if ai.isSectionTitle(line) {
			ai.context.CurrentSection = strings.TrimSpace(line)
			continue
		}

		// 提取当前场景的角色
		chars := ai.extractCharactersFromLine(line)
		if len(chars) > 0 {
			ai.context.CurrentCharacters = chars
		}

		// 提取当前地点
		if loc := ai.extractLocationFromLine(line); loc != "" {
			ai.context.CurrentLocation = loc
		}

		// 记录时间线事件
		ai.recordTimelineEvent(i, line)
	}
}

// isChapterTitle 判断是否为章节标题
func (ai *AIExtractor) isChapterTitle(line string) bool {
	patterns := []string{
		`^#{1,2}\s*第[一二三四五六七八九十百千万]+[章卷部]`,
		`^#{1,2}\s*第\d+[章卷部]`,
		`^第[一二三四五六七八九十百千万]+[章卷部]`,
		`^第\d+[章卷部]`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			return true
		}
	}
	return false
}

// isSectionTitle 判断是否为小节标题
func (ai *AIExtractor) isSectionTitle(line string) bool {
	patterns := []string{
		`^#{3,4}\s*`,
		`^\*\*.*\*\*$`,
		`^[一-龥]{2,10}[,，、]`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			return true
		}
	}
	return false
}

// extractCharactersFromLine 从行中提取角色
func (ai *AIExtractor) extractCharactersFromLine(line string) []string {
	// 匹配2-4个汉字的名字
	pattern := regexp.MustCompile(`([一-龥]{2,4})(?:说|道|想|看|走|来|去|死|活)`)
	matches := pattern.FindAllStringSubmatch(line, -1)

	chars := make([]string, 0)
	for _, match := range matches {
		if len(match) >= 2 {
			name := match[1]
			// 过滤非人名词
			if !ai.isFilterWord(name) {
				chars = append(chars, name)
			}
		}
	}

	return chars
}

// isFilterWord 是否为过滤词
func (ai *AIExtractor) isFilterWord(word string) bool {
	filterWords := []string{
		"因此", "然后", "因此他", "然后他", "自己", "众人", "对方",
		"什么", "这个", "那个", "这里", "那里", "现在", "当时",
	}
	for _, fw := range filterWords {
		if word == fw {
			return true
		}
	}
	return false
}

// extractLocationFromLine 从行中提取地点
func (ai *AIExtractor) extractLocationFromLine(line string) string {
	// 匹配地点描述
	patterns := []string{
		`在([一-龥]*(?:矿场|矿洞|矿道|宗门|城池|山脉|森林|洞府|秘境))`,
		`位于([一-龥]*(?:矿场|矿洞|矿道|宗门|城池|山脉|森林|洞府|秘境))`,
		`([一-龥]*(?:矿场|矿洞|矿道|宗门|城池|山脉|森林|洞府|秘境))内`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(line); len(match) >= 2 {
			return match[1]
		}
	}

	return ""
}

// recordTimelineEvent 记录时间线事件
func (ai *AIExtractor) recordTimelineEvent(lineNum int, line string) {
	event := &TimelineEvent{
		Time:         lineNum,
		Day:          ai.inferDay(line),
		Characters:   ai.context.CurrentCharacters,
		Location:     ai.context.CurrentLocation,
		Events:       make([]string, 0),
		PowerChanges: make(map[string]int),
	}

	// 检测事件类型
	if strings.Contains(line, "死亡") {
		event.Events = append(event.Events, "death")
	}
	if strings.Contains(line, "复活") {
		event.Events = append(event.Events, "resurrection")
		ai.context.PowerSystem.ResurrectionCount++
	}
	if strings.Contains(line, "突破") || strings.Contains(line, "晋升") {
		event.Events = append(event.Events, "breakthrough")
	}
	if strings.Contains(line, "战斗") || strings.Contains(line, "打斗") {
		event.Events = append(event.Events, "combat")
	}

	// 检测战力变化
	if changes := ai.extractPowerChanges(line); len(changes) > 0 {
		event.PowerChanges = changes
		ai.context.PowerSystem.PowerChanges += len(changes)
	}

	if len(event.Events) > 0 || len(event.PowerChanges) > 0 {
		ai.context.Timeline[lineNum] = event
	}
}

// inferDay 推断天数
func (ai *AIExtractor) inferDay(line string) int {
	// 匹配天数描述
	patterns := []string{
		`第(\d+)天`,
		`(\d+)天后`,
		`过了(\d+)天`,
		`第([一二三四五六七八九十百千万]+)天`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(line); len(match) >= 2 {
			// 尝试解析数字
			if day, err := strconv.Atoi(match[1]); err == nil {
				return day
			}
			// 解析中文数字
			return chineseToNumber(match[1])
		}
	}

	return 0
}

// extractPowerChanges 提取战力变化
func (ai *AIExtractor) extractPowerChanges(line string) map[string]int {
	changes := make(map[string]int)

	// 匹配修为变化
	pattern := regexp.MustCompile(`(练气|筑基|金丹|元婴|化神|合体|大乘|渡劫)([一二三四五六七八九十百千万]+).*?(?:跌落|降至|提升到|突破至).*?(练气|筑基|金丹|元婴|化神|合体|大乘|渡劫)([一二三四五六七八九十百千万]+)`)
	if match := pattern.FindStringSubmatch(line); len(match) >= 5 {
		fromLevel := match[1] + match[2]
		toLevel := match[3] + match[4]
		changes[fromLevel] = ai.context.PowerSystem.LevelOrder[toLevel] - ai.context.PowerSystem.LevelOrder[fromLevel]
	}

	return changes
}

// buildTimeline 构建完整时间线
func (ai *AIExtractor) buildTimeline(text string) {
	// 已经在recordTimelineEvent中构建了基础时间线
	// 这里进行时间线验证和补充

	// 检测时间线矛盾
	ai.detectTimelineInconsistencies()
}

// detectTimelineInconsistencies 检测时间线矛盾
func (ai *AIExtractor) detectTimelineInconsistencies() {
	// 检测同一天发生矛盾事件
	dayEvents := make(map[int][]*TimelineEvent)
	for _, event := range ai.context.Timeline {
		if event.Day > 0 {
			dayEvents[event.Day] = append(dayEvents[event.Day], event)
		}
	}

	// 检测角色在同一时间出现在不同地点
	for day, events := range dayEvents {
		charLocations := make(map[string][]string)
		for _, event := range events {
			for _, char := range event.Characters {
				charLocations[char] = append(charLocations[char], event.Location)
			}
		}

		for char, locations := range charLocations {
			if len(locations) > 1 {
				// 同一角色同一天出现在多个地点
				ai.context.PowerSystem.Inconsistencies = append(
					ai.context.PowerSystem.Inconsistencies,
					fmt.Sprintf("第%d天: 角色%s同时出现在多个地点: %v", day, char, locations),
				)
			}
		}
	}
}

// analyzeCharacterArcs 分析角色弧线
func (ai *AIExtractor) analyzeCharacterArcs(entities []ExtractedEntity) {
	for _, entity := range entities {
		if entity.Type != "character" {
			continue
		}

		name := entity.Name
		if _, exists := ai.context.CharacterArcs[name]; !exists {
			ai.context.CharacterArcs[name] = &CharacterArc{
				Name:          name,
				FirstAppear:   entity.LineNumber,
				PowerChanges:  make([]int, 0),
				Relationships: make([]string, 0),
				KeyEvents:     make([]string, 0),
			}
		}

		arc := ai.context.CharacterArcs[name]
		arc.AppearCount++

		// 记录关键事件
		if entity.Attributes["status"] != "" {
			arc.KeyEvents = append(arc.KeyEvents, entity.Attributes["status"])
		}

		// 统计死亡和复活
		if entity.Attributes["status"] == "dead" {
			arc.DeathCount++
		}
		if entity.Attributes["status"] == "resurrected" {
			arc.ResurrectCount++
		}
	}
}

// analyzePowerSystem 分析战力系统
func (ai *AIExtractor) analyzePowerSystem(entities []ExtractedEntity) {
	// 检测战力系统问题
	if ai.context.PowerSystem.ResurrectionCount > 10 {
		ai.context.PowerSystem.Inconsistencies = append(
			ai.context.PowerSystem.Inconsistencies,
			fmt.Sprintf("复活次数过多: %d次，可能导致战力崩坏", ai.context.PowerSystem.ResurrectionCount),
		)
	}

	if ai.context.PowerSystem.PowerChanges > 20 {
		ai.context.PowerSystem.Inconsistencies = append(
			ai.context.PowerSystem.Inconsistencies,
			fmt.Sprintf("战力变化过于频繁: %d次", ai.context.PowerSystem.PowerChanges),
		)
	}
}

// convertToRPGData 转换为RPG数据
func (ai *AIExtractor) convertToRPGData(entities []ExtractedEntity) *NovelRPGData {
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
	charMap := make(map[string]*CharacterTemplate)
	for _, entity := range entities {
		if entity.Type == "character" {
			char := ai.convertToCharacter(entity)
			data.Characters = append(data.Characters, char)
			charMap[entity.Name] = char
		}
	}

	// 转换物品
	for _, entity := range entities {
		if entity.Type == "item" {
			item := ai.convertToItem(entity)
			data.Items = append(data.Items, item)
		}
	}

	// 转换技能
	for _, entity := range entities {
		if entity.Type == "skill" {
			skill := ai.convertToSkill(entity)
			data.Skills = append(data.Skills, skill)
		}
	}

	// 转换地点
	for _, entity := range entities {
		if entity.Type == "location" {
			location := ai.convertToLocation(entity)
			data.Locations = append(data.Locations, location)
		}
	}

	// 转换时间线
	for _, event := range ai.context.Timeline {
		data.Timeline = append(data.Timeline, event)
	}

	// 生成验证问题
	data.ValidationIssues = ai.generateValidationIssues()

	return data
}

// convertToCharacter 转换为角色
func (ai *AIExtractor) convertToCharacter(entity ExtractedEntity) *CharacterTemplate {
	char := &CharacterTemplate{
		ID:          string(NewID("char_tpl")),
		Name:        entity.Name,
		Description: entity.SourceText,
		ClassID:     entity.Category,
		BaseStats:   BaseStats{HP: 100, MP: 50, Attack: 10, Defense: 5, Speed: 10, Luck: 5},
		GrowthStats: GrowthStats{HP: 1.0, MP: 1.0, Attack: 1.0, Defense: 1.0, Speed: 1.0, Luck: 1.0},
		Rarity:      RarityCommon,
	}

	// 设置属性
	if level, ok := entity.Attributes["cultivation_level"]; ok {
		// 根据修为等级设置基础属性
		baseStats := ai.calculateStatsFromLevel(level)
		char.BaseStats = baseStats
	}

	return char
}

// convertToItem 转换为物品
func (ai *AIExtractor) convertToItem(entity ExtractedEntity) *Item {
	item := &Item{
		ID:          string(NewID("item")),
		Name:        entity.Name,
		Description: entity.SourceText,
		Type:        ItemTypeConsumable,
		Rarity:      RarityCommon,
		Weight:      0.1,
		MaxStack:    99,
		Value:       10,
		IsUsable:    true,
		IsDroppable: true,
		IsSellable:  true,
	}

	// 根据类别设置类型
	switch entity.Category {
	case "currency":
		item.Type = ItemTypeMisc
		item.Rarity = RarityCommon
		item.MaxStack = 9999
	case "consumable":
		item.Type = ItemTypeConsumable
		item.Rarity = RarityUncommon
	case "equipment":
		item.Type = ItemTypeMaterial
		item.Rarity = RarityRare
	case "skill_book":
		item.Type = ItemTypeMaterial
		item.Rarity = RarityEpic
	}

	return item
}

// convertToSkill 转换为技能
func (ai *AIExtractor) convertToSkill(entity ExtractedEntity) *Skill {
	skill := &Skill{
		ID:          string(NewID("skill")),
		Name:        entity.Name,
		Description: entity.SourceText,
		Type:        SkillTypeActive,
		Target:      SkillTargetSelf,
		Cost: SkillCost{
			MP: 10,
		},
		Cooldown: 1,
		MaxLevel: 10,
	}

	// 根据类别设置类型
	if entity.Category == "passive" {
		skill.Type = SkillTypePassive
	} else if entity.Category == "cheat" {
		skill.Type = SkillTypeUltimate
	}

	return skill
}

// convertToLocation 转换为地点
func (ai *AIExtractor) convertToLocation(entity ExtractedEntity) *Map {
	gameMap := &Map{
		ID:       string(NewID("map")),
		Name:     entity.Name,
		Type:     MapTypeField,
		Width:    50,
		Height:   50,
		TileSize: 32,
		Entities: make([]MapEntity, 0),
	}

	// 根据类别设置类型和大小
	switch entity.Category {
	case "dungeon":
		gameMap.Type = MapTypeDungeon
		gameMap.Width = 30
		gameMap.Height = 30
	case "instance":
		gameMap.Type = MapTypeSpecial
		gameMap.Width = 40
		gameMap.Height = 40
	case "city":
		gameMap.Type = MapTypeTown
		gameMap.Width = 60
		gameMap.Height = 60
	}

	return gameMap
}

// calculateStatsFromLevel 根据等级计算属性
func (ai *AIExtractor) calculateStatsFromLevel(level string) BaseStats {
	// 基础属性
	baseStats := BaseStats{
		HP:      100,
		MP:      50,
		Attack:  10,
		Defense: 5,
		Speed:   10,
		Luck:    5,
	}

	// 根据修为等级调整
	levelMultipliers := map[string]float64{
		"练气": 1.0,
		"筑基": 2.0,
		"金丹": 4.0,
		"元婴": 8.0,
		"化神": 16.0,
		"合体": 32.0,
		"大乘": 64.0,
		"渡劫": 128.0,
	}

	for levelName, multiplier := range levelMultipliers {
		if strings.Contains(level, levelName) {
			baseStats.HP = int(float64(baseStats.HP) * multiplier)
			baseStats.MP = int(float64(baseStats.MP) * multiplier)
			baseStats.Attack = int(float64(baseStats.Attack) * multiplier)
			baseStats.Defense = int(float64(baseStats.Defense) * multiplier)
			break
		}
	}

	return baseStats
}

// generateValidationIssues 生成验证问题
func (ai *AIExtractor) generateValidationIssues() []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	// 检查复活次数
	if ai.context.PowerSystem.ResurrectionCount > 5 {
		issues = append(issues, ValidationIssue{
			Type:       "resurrection_abuse",
			Severity:   "critical",
			Message:    fmt.Sprintf("复活次数过多: %d次", ai.context.PowerSystem.ResurrectionCount),
			Suggestion: "建议限制复活次数，每次复活增加代价，或设置复活冷却时间",
		})
	}

	// 检查战力变化频率
	if ai.context.PowerSystem.PowerChanges > 15 {
		issues = append(issues, ValidationIssue{
			Type:       "power_system_instability",
			Severity:   "critical",
			Message:    fmt.Sprintf("战力变化过于频繁: %d次", ai.context.PowerSystem.PowerChanges),
			Suggestion: "建议稳定战力系统，减少突破次数，增加修炼过程的描写",
		})
	}

	// 检查角色弧线
	for name, arc := range ai.context.CharacterArcs {
		if arc.AppearCount == 1 {
			issues = append(issues, ValidationIssue{
				Type:       "tool_character",
				Severity:   "warning",
				Message:    fmt.Sprintf("角色%s只出现1次，可能是工具人", name),
				Suggestion: "建议增加该角色的戏份，或合并到现有角色中",
			})
		}

		if arc.DeathCount > 3 {
			issues = append(issues, ValidationIssue{
				Type:       "death_abuse",
				Severity:   "warning",
				Message:    fmt.Sprintf("角色%s死亡次数过多: %d次", name, arc.DeathCount),
				Suggestion: "建议减少死亡次数，增加死亡的严肃性和代价",
			})
		}
	}

	// 添加时间线不一致问题
	for _, inconsistency := range ai.context.PowerSystem.Inconsistencies {
		issues = append(issues, ValidationIssue{
			Type:       "timeline_inconsistency",
			Severity:   "warning",
			Message:    inconsistency,
			Suggestion: "建议检查时间线逻辑，确保角色不会同时出现在多个地点",
		})
	}

	return issues
}

// chineseToNumber 中文数字转阿拉伯数字
func chineseToNumber(chinese string) int {
	chineseNums := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
		'百': 100, '千': 1000, '万': 10000,
	}

	result := 0
	temp := 0
	_ = 1 // lastUnit placeholder

	for i := len(chinese) - 1; i >= 0; i-- {
		char := rune(chinese[i])
		if num, ok := chineseNums[char]; ok {
			if num >= 10 {
				if temp == 0 {
					temp = 1
				}
				result += temp * num
				temp = 0
			} else {
				temp = temp*10 + num
			}
		}
	}

	result += temp
	return result
}

// GetExtractionStats 获取提取统计
func (ai *AIExtractor) GetExtractionStats() map[string]interface{} {
	stats := map[string]interface{}{
		"timeline_events":    len(ai.context.Timeline),
		"character_arcs":     len(ai.context.CharacterArcs),
		"resurrection_count": ai.context.PowerSystem.ResurrectionCount,
		"power_changes":      ai.context.PowerSystem.PowerChanges,
		"inconsistencies":    len(ai.context.PowerSystem.Inconsistencies),
	}

	// 统计角色弧线
	arcStats := make(map[string]interface{})
	for name, arc := range ai.context.CharacterArcs {
		arcStats[name] = map[string]interface{}{
			"appearances": arc.AppearCount,
			"deaths":      arc.DeathCount,
			"resurrects":  arc.ResurrectCount,
		}
	}
	stats["character_details"] = arcStats

	return stats
}
