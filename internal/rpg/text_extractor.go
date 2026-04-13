package rpg

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExtractedEntity 提取的实体
type ExtractedEntity struct {
	Type       string            `json:"type"` // character, item, skill, location, event
	Name       string            `json:"name"`
	Category   string            `json:"category"`    // 子类型
	Attributes map[string]string `json:"attributes"`  // 属性
	Relations  []EntityRelation  `json:"relations"`   // 关系
	SourceText string            `json:"source_text"` // 原文
	Confidence float64           `json:"confidence"`  // 置信度 0-1
	LineNumber int               `json:"line_number"` // 所在行号
}

// EntityRelation 实体关系
type EntityRelation struct {
	Type    string `json:"type"`    // own, use, fight, meet, etc.
	Target  string `json:"target"`  // 目标实体名
	Context string `json:"context"` // 关系上下文
}

// TextExtractor 文本提取器
type TextExtractor struct {
	entities []ExtractedEntity
	rules    []ExtractionRule
}

// ExtractionRule 提取规则
type ExtractionRule struct {
	EntityType string
	Pattern    *regexp.Regexp
	Extractor  func(matches []string, line string, lineNum int) *ExtractedEntity
}

// NewTextExtractor 创建文本提取器
func NewTextExtractor() *TextExtractor {
	te := &TextExtractor{
		entities: make([]ExtractedEntity, 0),
	}
	te.initRules()
	return te
}

// initRules 初始化提取规则
func (te *TextExtractor) initRules() {
	te.rules = []ExtractionRule{
		// 角色提取规则 - 多种匹配模式
		{
			EntityType: "character",
			Pattern:    regexp.MustCompile(`([一-龥]{2,5})(?:说道|说|道|问|答|想|看着|看着|走到|来到|站在|坐在|躺在|死了|复活|醒来)`),
			Extractor:  te.extractCharacter,
		},
		// 角色提取规则2 - 名字+动作
		{
			EntityType: "character",
			Pattern:    regexp.MustCompile(`(?:^|[，。！？\s])([一-龥]{2,5})(?:[，。！？\s])`),
			Extractor:  te.extractCharacterSimple,
		},
		// 修为/等级提取 - 支持更多修仙体系
		{
			EntityType: "cultivation",
			Pattern:    regexp.MustCompile(`(练气|筑基|金丹|元婴|化神|合体|大乘|渡劫|先天|后天|宗师|大宗师|武王|武皇|武帝|武圣|武神|斗者|斗师|大斗师|斗灵|斗王|斗皇|斗宗|斗尊|斗圣|斗帝)([一二三四五六七八九十百千万]+)(?:层|期|阶|重|品|星|段|级)?`),
			Extractor:  te.extractCultivation,
		},
		// 物品提取 - 支持更多物品类型和描述
		{
			EntityType: "item",
			Pattern:    regexp.MustCompile(`([一-龥]{1,8}(?:剑|刀|枪|棍|斧|鞭|锤|爪|环|珠|镜|鼎|塔|扇|琴|笛|葫|瓶|罐|盒|袋|戒|镯|链|佩|甲|衣|袍|靴|帽|盔|盾|符|丹|药|草|花|果|根|石|矿|晶|玉|金|银|铜|铁|木|竹|书|卷|轴|图|谱|诀|术|法|功|技|能|宝|器|具))`),
			Extractor:  te.extractItem,
		},
		// 技能/金手指提取 - 支持多种引号
		{
			EntityType: "skill",
			Pattern:    regexp.MustCompile(`[「""']([^""'」]*(?:复活|系统|面板|空间|签到|抽奖|模拟|功法|剑法|刀法|拳法|掌法|指法|步法|心法|神通|秘术|绝技))[」""']`),
			Extractor:  te.extractSkill,
		},
		// 地点提取 - 支持更多地点类型
		{
			EntityType: "location",
			Pattern:    regexp.MustCompile(`([一-龥]{1,10}(?:场|洞|道|门|城|池|镇|村|庄|岛|山|峰|岭|谷|林|海|湖|河|潭|渊|崖|壁|坡|原|野|漠|泽|沼|地|界|域|境|天|界|宫|殿|阁|楼|台|亭|榭|轩|斋|庵|观|寺|庙|祠|陵|墓|冢|墟|址|迹|境|府|宅|院|园|圃|苑|池|塘|沼|泽))`),
			Extractor:  te.extractLocation,
		},
		// 事件提取 - 带上下文
		{
			EntityType: "event",
			Pattern:    regexp.MustCompile(`(?:被|遭|受|遇|与|和|同).*?(死亡|杀死|击杀|陨落|身亡|毙命|丧命|遇害|阵亡|战死|决斗|战斗|打斗|交战|厮杀|搏斗|对决|比试|切磋|突破|晋升|进阶|升级|晋级|逃跑|逃离|逃脱|撤退|撤离|相遇|遇见|碰到|邂逅|重逢|相聚|会面|相见|背叛|出卖|反叛|倒戈|叛变|变节|投敌|发现|找到|寻得|获得|得到|察觉|发觉|觉察)`),
			Extractor:  te.extractEvent,
		},
		// 数值属性提取
		{
			EntityType: "attribute",
			Pattern:    regexp.MustCompile(`(\d+)(?:年|岁|块|枚|点|层|级|重|倍)`),
			Extractor:  te.extractAttribute,
		},
	}
}

// ExtractFromText 从文本中提取实体
func (te *TextExtractor) ExtractFromText(text string) []ExtractedEntity {
	scanner := bufio.NewScanner(strings.NewReader(text))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, rule := range te.rules {
			matches := rule.Pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if entity := rule.Extractor(match, line, lineNum); entity != nil {
					te.entities = append(te.entities, *entity)
				}
			}
		}
	}

	// 去重和合并
	te.deduplicate()

	return te.entities
}

// 提取角色
func (te *TextExtractor) extractCharacter(matches []string, line string, lineNum int) *ExtractedEntity {
	if len(matches) < 2 {
		return nil
	}

	name := strings.TrimSpace(matches[1])
	if len(name) < 2 || len(name) > 5 {
		return nil
	}

	// 过滤常见非人名词和错误提取
	filterWords := []string{
		// 连接词
		"因此", "然后", "因此他", "于是他", "自己", "众人", "对方", "什么", "这个", "那个",
		"这里", "那里", "现在", "当时", "只见", "忽然", "突然", "原来", "虽然", "尽管",
		"因为", "所以", "于是", "但是", "然而", "不过", "只是", "只有", "只要", "如果",
		"假如", "即使", "哪怕", "无论", "不管", "不论", "不仅", "不但", "而且", "并且",
		"或者", "还是", "要么", "与其", "宁可", "宁愿", "除了", "除去", "除非", "关于",
		"对于", "由于", "根据", "按照", "依照", "通过", "经过", "随着", "为了", "为着",
		// 副词
		"可以", "能够", "应该", "需要", "必须", "一定", "也许", "大概", "可能", "似乎",
		"好像", "仿佛", "简直", "根本", "绝对", "完全", "实在", "确实", "的确", "真的",
		"太", "很", "非常", "特别", "十分", "相当", "比较", "稍微", "有点", "一些",
		// 错误提取的常见词
		"暗中观", "因此林", "然后林", "于是林", "自己笔", "正在连", "拥有对",
		"正在采", "正在修", "正在凝", "正在酝", "正在酝", "正在酝",
	}
	for _, word := range filterWords {
		if name == word || strings.HasPrefix(name, word) {
			return nil
		}
	}

	// 过滤包含特定动词的名字（通常是错误提取）
	invalidPatterns := []string{"正在", "拥有", "自己", "笔下", "连载", "观", "对", "意识", "发现", "决定", "开始", "准备", "想要"}
	for _, pattern := range invalidPatterns {
		if strings.Contains(name, pattern) {
			return nil
		}
	}

	// 过滤以特定词开头的名字
	invalidPrefixes := []string{"因此", "然后", "于是", "但是", "所以", "自己", "正在", "拥有"}
	for _, prefix := range invalidPrefixes {
		if strings.HasPrefix(name, prefix) {
			return nil
		}
	}

	return &ExtractedEntity{
		Type:       "character",
		Name:       name,
		Category:   te.inferCharacterCategory(line),
		Attributes: te.extractCharacterAttributes(line),
		SourceText: line,
		Confidence: 0.7,
		LineNumber: lineNum,
	}
}

// extractCharacterSimple 简单角色提取（用于独立名字匹配）
func (te *TextExtractor) extractCharacterSimple(matches []string, line string, lineNum int) *ExtractedEntity {
	if len(matches) < 2 {
		return nil
	}

	name := strings.TrimSpace(matches[1])
	if len(name) < 2 || len(name) > 5 {
		return nil
	}

	// 更严格的过滤词列表
	filterWords := []string{"因此", "然后", "因此他", "然后他", "自己", "众人", "对方", "什么", "这个", "那个", "这里", "那里", "现在", "当时", "只见", "忽然", "突然", "原来", "虽然", "尽管", "因为", "所以", "于是", "但是", "然而", "不过", "只是", "只有", "只要", "如果", "假如", "即使", "哪怕", "无论", "不管", "不论", "不仅", "不但", "而且", "并且", "或者", "还是", "要么", "与其", "宁可", "宁愿", "除了", "除去", "除非", "关于", "对于", "由于", "根据", "按照", "依照", "通过", "经过", "随着", "为了", "为着", "可以", "能够", "应该", "需要", "必须", "一定", "也许", "大概", "可能", "似乎", "好像", "仿佛", "简直", "根本", "绝对", "完全", "实在", "确实", "的确", "真的"}
	for _, word := range filterWords {
		if name == word {
			return nil
		}
	}

	// 检查是否可能是人名（简单启发式：常见姓氏）
	commonSurnames := []string{"李", "王", "张", "刘", "陈", "杨", "赵", "黄", "周", "吴", "徐", "孙", "胡", "朱", "高", "林", "何", "郭", "马", "罗", "梁", "宋", "郑", "谢", "韩", "唐", "冯", "于", "董", "萧", "程", "曹", "袁", "邓", "许", "傅", "沈", "曾", "彭", "吕", "苏", "卢", "蒋", "蔡", "贾", "丁", "魏", "薛", "叶", "阎", "余", "潘", "杜", "戴", "夏", "钟", "汪", "田", "任", "姜", "范", "方", "石", "姚", "谭", "廖", "邹", "熊", "金", "陆", "郝", "孔", "白", "崔", "康", "毛", "邱", "秦", "江", "史", "顾", "侯", "邵", "孟", "龙", "万", "段", "雷", "钱", "汤", "尹", "黎", "易", "常", "武", "乔", "贺", "赖", "龚", "文"}
	hasSurname := false
	for _, surname := range commonSurnames {
		if strings.HasPrefix(name, surname) {
			hasSurname = true
			break
		}
	}

	// 如果没有常见姓氏，降低置信度
	confidence := 0.5
	if hasSurname {
		confidence = 0.6
	}

	return &ExtractedEntity{
		Type:       "character",
		Name:       name,
		Category:   te.inferCharacterCategory(line),
		Attributes: te.extractCharacterAttributes(line),
		SourceText: line,
		Confidence: confidence,
		LineNumber: lineNum,
	}
}

// 推断角色类别
func (te *TextExtractor) inferCharacterCategory(line string) string {
	categories := map[string]string{
		"监工":  "antagonist",
		"矿奴":  "npc",
		"长老":  "npc",
		"宗主":  "npc",
		"弟子":  "npc",
		"主角":  "protagonist",
		"穿越":  "protagonist",
		"金手指": "protagonist",
	}

	for keyword, category := range categories {
		if strings.Contains(line, keyword) {
			return category
		}
	}

	return "unknown"
}

// 提取角色属性
func (te *TextExtractor) extractCharacterAttributes(line string) map[string]string {
	attrs := make(map[string]string)

	// 提取修为等级
	cultPattern := regexp.MustCompile(`(练气|筑基|金丹|元婴|化神|合体|大乘|渡劫)([一二三四五六七八九十百千万]+)`)
	if match := cultPattern.FindStringSubmatch(line); len(match) >= 3 {
		attrs["cultivation_level"] = match[1] + match[2]
	}

	// 提取年龄/寿命
	agePattern := regexp.MustCompile(`(\d+)(?:年|岁)`)
	if match := agePattern.FindStringSubmatch(line); len(match) >= 2 {
		attrs["age"] = match[1]
	}

	// 提取状态
	statusKeywords := map[string]string{
		"死亡": "dead",
		"复活": "resurrected",
		"受伤": "injured",
		"昏迷": "unconscious",
		"修炼": "cultivating",
		"战斗": "fighting",
		"逃跑": "fleeing",
	}

	for keyword, status := range statusKeywords {
		if strings.Contains(line, keyword) {
			attrs["status"] = status
			break
		}
	}

	return attrs
}

// 提取修为
func (te *TextExtractor) extractCultivation(matches []string, line string, lineNum int) *ExtractedEntity {
	if len(matches) < 3 {
		return nil
	}

	return &ExtractedEntity{
		Type:     "cultivation",
		Name:     matches[1] + matches[2],
		Category: matches[1],
		Attributes: map[string]string{
			"level": matches[2],
			"raw":   matches[0],
		},
		SourceText: line,
		Confidence: 0.9,
		LineNumber: lineNum,
	}
}

// 提取物品
func (te *TextExtractor) extractItem(matches []string, line string, lineNum int) *ExtractedEntity {
	if len(matches) < 2 {
		return nil
	}

	name := matches[1]
	category := "misc"

	// 分类物品
	if strings.Contains(name, "灵石") {
		category = "currency"
	} else if strings.Contains(name, "丹药") {
		category = "consumable"
	} else if strings.Contains(name, "法宝") || strings.Contains(name, "武器") {
		category = "equipment"
	} else if strings.Contains(name, "秘籍") || strings.Contains(name, "功法") {
		category = "skill_book"
	}

	// 提取数量
	quantity := 1
	qtyPattern := regexp.MustCompile(`(\d+)(?:块|枚|颗|瓶|本|件|个)`)
	if match := qtyPattern.FindStringSubmatch(line); len(match) >= 2 {
		if q, err := strconv.Atoi(match[1]); err == nil {
			quantity = q
		}
	}

	return &ExtractedEntity{
		Type:     "item",
		Name:     name,
		Category: category,
		Attributes: map[string]string{
			"quantity": fmt.Sprintf("%d", quantity),
		},
		SourceText: line,
		Confidence: 0.8,
		LineNumber: lineNum,
	}
}

// 提取技能
func (te *TextExtractor) extractSkill(matches []string, line string, lineNum int) *ExtractedEntity {
	name := ""
	for i := 1; i < len(matches); i++ {
		if matches[i] != "" {
			name = matches[i]
			break
		}
	}

	if name == "" {
		return nil
	}

	// 判断技能类型
	skillType := "active"
	if strings.Contains(line, "被动") || strings.Contains(line, "天赋") {
		skillType = "passive"
	} else if strings.Contains(line, "金手指") || strings.Contains(line, "系统") {
		skillType = "cheat"
	}

	return &ExtractedEntity{
		Type:     "skill",
		Name:     name,
		Category: skillType,
		Attributes: map[string]string{
			"trigger_condition": te.extractTriggerCondition(line),
		},
		SourceText: line,
		Confidence: 0.85,
		LineNumber: lineNum,
	}
}

// 提取触发条件
func (te *TextExtractor) extractTriggerCondition(line string) string {
	conditions := []string{}

	if strings.Contains(line, "死亡") {
		conditions = append(conditions, "on_death")
	}
	if strings.Contains(line, "战斗") {
		conditions = append(conditions, "on_combat")
	}
	if strings.Contains(line, "修炼") {
		conditions = append(conditions, "on_cultivation")
	}
	if strings.Contains(line, "签到") {
		conditions = append(conditions, "on_checkin")
	}

	if len(conditions) == 0 {
		return "unknown"
	}
	return strings.Join(conditions, ",")
}

// 提取地点
func (te *TextExtractor) extractLocation(matches []string, line string, lineNum int) *ExtractedEntity {
	if len(matches) < 2 {
		return nil
	}

	name := matches[1]

	// 过滤过长或过短的地点名
	if len(name) < 2 || len(name) > 15 {
		return nil
	}

	// 过滤包含动词或错误模式的地点名
	invalidPatterns := []string{"正在", "拥有", "自己", "对", "中", "了", "的", "着", "将", "发现", "意识到", "快步", "逃离", "地下", "隐藏", "凶", "下", "原", "瞥见", "当场", "知道", "调虎", "离山", "崩天", "大半个", "推断", "回望", "应付", "锁定", "仅用", "看到", "应付过", "入城", "当天", "遇到", "遭遇", "偶然", "听到", "讹到", "被", "吓得", "委托", "他去", "坐镇", "和之前", "小石头", "专门留给"}
	for _, pattern := range invalidPatterns {
		if strings.Contains(name, pattern) {
			return nil
		}
	}

	// 过滤以动词开头的地点名
	verbPrefixes := []string{"将", "发现", "意识到", "快步", "逃离", "前往", "来到", "进入", "瞥见", "知道"}
	for _, prefix := range verbPrefixes {
		if strings.HasPrefix(name, prefix) {
			return nil
		}
	}

	// 过滤纯动词或常见错误提取
	invalidNames := []string{"知道", "认为", "觉得", "看到", "听到", "想到", "发现", "决定", "开始", "准备", "想要", "可以", "能够", "推断", "回望", "应付", "锁定", "仅用", "入城", "当天", "第一天", "第二天", "第三天", "次", "地", "城"}
	for _, invalidName := range invalidNames {
		if name == invalidName || strings.HasPrefix(name, invalidName) {
			return nil
		}
	}

	// 过滤包含数字或量词的地名
	if matched, _ := regexp.MatchString(`[\d三四五六七八九十半]`, name); matched {
		// 但保留合理的地名如"三天门"
		if strings.Contains(name, "三天") || strings.Contains(name, "五天") {
			return nil
		}
	}

	// 过滤纯虚词组合
	virtualWords := []string{"因此", "然后", "于是", "但是", "所以", "因为", "虽然", "尽管"}
	for _, word := range virtualWords {
		if strings.HasPrefix(name, word) {
			return nil
		}
	}

	locType := "area"

	if strings.Contains(name, "矿道") || strings.Contains(name, "矿洞") {
		locType = "dungeon"
	} else if strings.Contains(name, "矿场") {
		locType = "instance"
	} else if strings.Contains(name, "宗门") || strings.Contains(name, "城池") {
		locType = "city"
	}

	return &ExtractedEntity{
		Type:       "location",
		Name:       name,
		Category:   locType,
		Attributes: make(map[string]string),
		SourceText: line,
		Confidence: 0.75,
		LineNumber: lineNum,
	}
}

// 提取事件
func (te *TextExtractor) extractEvent(matches []string, line string, lineNum int) *ExtractedEntity {
	if len(matches) < 2 {
		return nil
	}

	eventType := matches[1]
	severity := "normal"

	// 判断事件严重程度
	if eventType == "死亡" || eventType == "复活" {
		severity = "critical"
	} else if eventType == "战斗" || eventType == "突破" {
		severity = "major"
	}

	return &ExtractedEntity{
		Type:     "event",
		Name:     eventType,
		Category: severity,
		Attributes: map[string]string{
			"event_type": eventType,
		},
		SourceText: line,
		Confidence: 0.8,
		LineNumber: lineNum,
	}
}

// 提取属性
func (te *TextExtractor) extractAttribute(matches []string, line string, lineNum int) *ExtractedEntity {
	if len(matches) < 2 {
		return nil
	}

	value := matches[1]
	attrType := "unknown"

	if strings.Contains(line, "年") || strings.Contains(line, "岁") {
		attrType = "time/lifespan"
	} else if strings.Contains(line, "块") || strings.Contains(line, "枚") {
		attrType = "quantity"
	} else if strings.Contains(line, "点") {
		attrType = "points"
	} else if strings.Contains(line, "层") || strings.Contains(line, "级") {
		attrType = "level"
	}

	return &ExtractedEntity{
		Type:     "attribute",
		Name:     value,
		Category: attrType,
		Attributes: map[string]string{
			"value": value,
			"unit":  te.extractUnit(line),
		},
		SourceText: line,
		Confidence: 0.9,
		LineNumber: lineNum,
	}
}

// 提取单位
func (te *TextExtractor) extractUnit(line string) string {
	units := []string{"年", "岁", "块", "枚", "颗", "点", "层", "级", "重", "倍"}
	for _, unit := range units {
		if strings.Contains(line, unit) {
			return unit
		}
	}
	return ""
}

// 去重
func (te *TextExtractor) deduplicate() {
	seen := make(map[string]bool)
	unique := make([]ExtractedEntity, 0)

	for _, entity := range te.entities {
		key := fmt.Sprintf("%s:%s:%d", entity.Type, entity.Name, entity.LineNumber)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, entity)
		}
	}

	te.entities = unique
}

// GetEntitiesByType 按类型获取实体
func (te *TextExtractor) GetEntitiesByType(entityType string) []ExtractedEntity {
	result := make([]ExtractedEntity, 0)
	for _, entity := range te.entities {
		if entity.Type == entityType {
			result = append(result, entity)
		}
	}
	return result
}

// GetEntityRelations 获取实体关系
func (te *TextExtractor) GetEntityRelations() []EntityRelation {
	relations := make([]EntityRelation, 0)

	// 分析共现关系
	charEntities := te.GetEntitiesByType("character")
	for i, char1 := range charEntities {
		for j, char2 := range charEntities {
			if i >= j {
				continue
			}
			// 如果出现在同一行，建立关系
			if char1.LineNumber == char2.LineNumber {
				relations = append(relations, EntityRelation{
					Type:    "coappear",
					Target:  char2.Name,
					Context: char1.SourceText,
				})
			}
		}
	}

	return relations
}

// ExtractStats 提取统计信息
func (te *TextExtractor) ExtractStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_entities":     len(te.entities),
		"characters":         len(te.GetEntitiesByType("character")),
		"items":              len(te.GetEntitiesByType("item")),
		"skills":             len(te.GetEntitiesByType("skill")),
		"locations":          len(te.GetEntitiesByType("location")),
		"events":             len(te.GetEntitiesByType("event")),
		"cultivation_levels": len(te.GetEntitiesByType("cultivation")),
	}

	// 统计角色类别
	charCategories := make(map[string]int)
	for _, char := range te.GetEntitiesByType("character") {
		charCategories[char.Category]++
	}
	stats["character_categories"] = charCategories

	// 统计事件严重程度
	eventSeverities := make(map[string]int)
	for _, event := range te.GetEntitiesByType("event") {
		eventSeverities[event.Category]++
	}
	stats["event_severities"] = eventSeverities

	return stats
}
