package benchmark

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ============================================================
// 跨章一致性检测系统
// ============================================================

// CrossChapterChecker 跨章一致性检查器
type CrossChapterChecker struct {
	chapters []ChapterState
	issues   []CrossChapterIssue
}

// ChapterState 章节状态快照
type ChapterState struct {
	ChapterID       string
	ChapterNum      int
	CharacterStates map[string]*CharacterSnapshot
	Timeline        TimelineInfo
	Events          []EventSnapshot
}

// CharacterSnapshot 角色状态快照
type CharacterSnapshot struct {
	Name           string
	HP             int
	MP             int
	MaxHP          int
	MaxMP          int
	Level          string
	Cultivation    string // 修为境界
	PowerLevel     int    // 战力数值
	Location       string
	Status         string // alive, dead, injured, unconscious
	Inventory      []string
	Relationships  map[string]string // 与其他角色的关系
	FirstAppear    int               // 首次出现的章节
	AppearCount    int               // 出现次数
	LastAction     string
}

// TimelineInfo 时间线信息
type TimelineInfo struct {
	AbsoluteTime int    // 绝对时间（天数）
	RelativeTime string // 相对时间描述
	Season       string
	Year         int
}

// CrossChapterIssue 跨章检测发现的问题
type CrossChapterIssue struct {
	Type        string // inconsistency, continuity_error, paradox
	Category    string // character, timeline, power, location
	Severity    string
	ChapterFrom string
	ChapterTo   string
	Target      string
	Description string
	Expected    string
	Actual      string
}

// NewCrossChapterChecker 创建跨章检查器
func NewCrossChapterChecker() *CrossChapterChecker {
	return &CrossChapterChecker{
		chapters: make([]ChapterState, 0),
		issues:   make([]CrossChapterIssue, 0),
	}
}

// AddChapter 添加章节状态
func (cc *CrossChapterChecker) AddChapter(state ChapterState) {
	cc.chapters = append(cc.chapters, state)
}

// CheckConsistency 执行所有跨章一致性检查
func (cc *CrossChapterChecker) CheckConsistency() []CrossChapterIssue {
	cc.issues = make([]CrossChapterIssue, 0)

	if len(cc.chapters) < 2 {
		return cc.issues
	}

	// 执行各种检查
	cc.checkCharacterContinuity()
	cc.CheckTimelineConsistency()
	cc.checkPowerConsistency()
	cc.checkLocationConsistency()
	cc.checkRelationshipConsistency()
	cc.checkInventoryConsistency()

	return cc.issues
}

// checkCharacterContinuity 检查角色连续性
func (cc *CrossChapterChecker) checkCharacterContinuity() {
	for i := 1; i < len(cc.chapters); i++ {
		prev := cc.chapters[i-1]
		curr := cc.chapters[i]

		for charName, prevState := range prev.CharacterStates {
			currState, exists := curr.CharacterStates[charName]
			if !exists {
				// 角色消失检查
				if prevState.Status != "dead" {
					cc.addIssue(CrossChapterIssue{
						Type:        "continuity_error",
						Category:    "character",
						Severity:    "warning",
						ChapterFrom: prev.ChapterID,
						ChapterTo:   curr.ChapterID,
						Target:      charName,
						Description: fmt.Sprintf("角色 %s 在前章出现但在本章消失", charName),
					})
				}
				continue
			}

			// 检查死亡状态
			if prevState.Status == "dead" && currState.Status == "alive" {
				cc.addIssue(CrossChapterIssue{
					Type:        "inconsistency",
					Category:    "character",
					Severity:    "critical",
					ChapterFrom: prev.ChapterID,
					ChapterTo:   curr.ChapterID,
					Target:      charName,
					Description: fmt.Sprintf("角色 %s 在上章死亡但本章复活，无复活过程", charName),
					Expected:    "保持死亡状态或有复活事件",
					Actual:      "突然复活",
				})
			}

			// 检查HP异常恢复
			if prevState.HP < prevState.MaxHP/3 && currState.HP == currState.MaxHP {
				// 从重伤到满血，检查是否有恢复过程
				if !hasRecoveryEvent(prev.Events, charName) {
					cc.addIssue(CrossChapterIssue{
						Type:        "continuity_error",
						Category:    "character",
						Severity:    "warning",
						ChapterFrom: prev.ChapterID,
						ChapterTo:   curr.ChapterID,
						Target:      charName,
						Description: fmt.Sprintf("角色 %s HP从%d/%d 突然恢复到满血，缺乏恢复过程", charName, prevState.HP, prevState.MaxHP),
					})
				}
			}

			// 检查修为倒退
			prevLevel := parseCultivationLevel(prevState.Cultivation)
			currLevel := parseCultivationLevel(currState.Cultivation)
			if currLevel < prevLevel {
				cc.addIssue(CrossChapterIssue{
					Type:        "inconsistency",
					Category:    "power",
					Severity:    "error",
					ChapterFrom: prev.ChapterID,
					ChapterTo:   curr.ChapterID,
					Target:      charName,
					Description: fmt.Sprintf("角色 %s 修为从 %s 倒退到 %s", charName, prevState.Cultivation, currState.Cultivation),
					Expected:    fmt.Sprintf("至少保持 %s", prevState.Cultivation),
					Actual:      currState.Cultivation,
				})
			}

			// 检查战力暴涨
			if currState.PowerLevel > prevState.PowerLevel*5 && currLevel == prevLevel {
				cc.addIssue(CrossChapterIssue{
					Type:        "inconsistency",
					Category:    "power",
					Severity:    "error",
					ChapterFrom: prev.ChapterID,
					ChapterTo:   curr.ChapterID,
					Target:      charName,
					Description: fmt.Sprintf("角色 %s 战力暴涨%d倍但修为未变", charName, currState.PowerLevel/prevState.PowerLevel),
				})
			}
		}
	}
}

// checkTimelineConsistency 检查时间线一致性
func (cc *CrossChapterChecker) CheckTimelineConsistency() {
	for i := 1; i < len(cc.chapters); i++ {
		prev := cc.chapters[i-1]
		curr := cc.chapters[i]

		// 检查绝对时间倒退
		if curr.Timeline.AbsoluteTime < prev.Timeline.AbsoluteTime {
			cc.addIssue(CrossChapterIssue{
				Type:        "paradox",
				Category:    "timeline",
				Severity:    "critical",
				ChapterFrom: prev.ChapterID,
				ChapterTo:   curr.ChapterID,
				Target:      "时间线",
				Description: "时间线倒流：本章时间早于前章",
				Expected:    fmt.Sprintf("时间 >= 第%d天", prev.Timeline.AbsoluteTime),
				Actual:      fmt.Sprintf("第%d天", curr.Timeline.AbsoluteTime),
			})
		}

		// 检查时间跳跃幅度
		timeJump := curr.Timeline.AbsoluteTime - prev.Timeline.AbsoluteTime
		if timeJump > 365 {
			cc.addIssue(CrossChapterIssue{
				Type:        "continuity_error",
				Category:    "timeline",
				Severity:    "warning",
				ChapterFrom: prev.ChapterID,
				ChapterTo:   curr.ChapterID,
				Target:      "时间线",
				Description: fmt.Sprintf("两章之间跳跃了%d天（约%.1f年），缺乏过渡", timeJump, float64(timeJump)/365),
			})
		}

		// 检查相对时间与绝对时间矛盾
		if prev.Timeline.RelativeTime != "" && curr.Timeline.RelativeTime != "" {
			prevRel := parseRelativeTime(prev.Timeline.RelativeTime)
			currRel := parseRelativeTime(curr.Timeline.RelativeTime)
			if currRel < prevRel && curr.Timeline.AbsoluteTime > prev.Timeline.AbsoluteTime {
				cc.addIssue(CrossChapterIssue{
					Type:        "paradox",
					Category:    "timeline",
					Severity:    "critical",
					ChapterFrom: prev.ChapterID,
					ChapterTo:   curr.ChapterID,
					Target:      "时间线",
					Description: fmt.Sprintf("时间描述矛盾：绝对时间前进但相对时间倒退（%s -> %s）", prev.Timeline.RelativeTime, curr.Timeline.RelativeTime),
				})
			}
		}
	}
}

// checkPowerConsistency 检查战力系统一致性
func (cc *CrossChapterChecker) checkPowerConsistency() {
	// 检查跨章战力膨胀
	for i := 1; i < len(cc.chapters); i++ {
		prev := cc.chapters[i-1]
		curr := cc.chapters[i]

		for charName, prevState := range prev.CharacterStates {
			currState, exists := curr.CharacterStates[charName]
			if !exists {
				continue
			}

			// 检查跨级战斗
			if currState.LastAction == "combat_win" && prevState.PowerLevel > 0 {
				// 如果本章战胜了明显更强的对手
				enemyPower := estimateEnemyPower(curr.Events, charName)
				if enemyPower > currState.PowerLevel*3 {
					cc.addIssue(CrossChapterIssue{
						Type:        "inconsistency",
						Category:    "power",
						Severity:    "critical",
						ChapterFrom: prev.ChapterID,
						ChapterTo:   curr.ChapterID,
						Target:      charName,
						Description: fmt.Sprintf("战力不合理：%s（战力%d）战胜了明显更强的敌人（估计战力%d）", charName, currState.PowerLevel, enemyPower),
					})
				}
			}
		}
	}
}

// checkLocationConsistency 检查位置一致性
func (cc *CrossChapterChecker) checkLocationConsistency() {
	for i := 1; i < len(cc.chapters); i++ {
		prev := cc.chapters[i-1]
		curr := cc.chapters[i]

		for charName, prevState := range prev.CharacterStates {
			currState, exists := curr.CharacterStates[charName]
			if !exists {
				continue
			}

			// 检查位置跳跃
			if prevState.Location != "" && currState.Location != "" &&
				prevState.Location != currState.Location {
				distance := estimateDistance(prevState.Location, currState.Location)
				timeAvailable := curr.Timeline.AbsoluteTime - prev.Timeline.AbsoluteTime

				// 如果距离过远但时间不足
				if distance > 1000 && timeAvailable < 1 {
					cc.addIssue(CrossChapterIssue{
						Type:        "inconsistency",
						Category:    "location",
						Severity:    "error",
						ChapterFrom: prev.ChapterID,
						ChapterTo:   curr.ChapterID,
						Target:      charName,
						Description: fmt.Sprintf("位置跳跃不合理：%s 从 %s 瞬间移动到 %s（距离约%d里）", charName, prevState.Location, currState.Location, distance),
					})
				}
			}
		}
	}
}

// checkRelationshipConsistency 检查关系一致性
func (cc *CrossChapterChecker) checkRelationshipConsistency() {
	for i := 1; i < len(cc.chapters); i++ {
		prev := cc.chapters[i-1]
		curr := cc.chapters[i]

		for charName, prevState := range prev.CharacterStates {
			currState, exists := curr.CharacterStates[charName]
			if !exists {
				continue
			}

			// 检查关系突变
			for otherChar, prevRel := range prevState.Relationships {
				currRel, exists := currState.Relationships[otherChar]
				if !exists {
					continue
				}

				// 检查敌对到友好的突变
				if isHostile(prevRel) && isFriendly(currRel) {
					cc.addIssue(CrossChapterIssue{
						Type:        "inconsistency",
						Category:    "character",
						Severity:    "error",
						ChapterFrom: prev.ChapterID,
						ChapterTo:   curr.ChapterID,
						Target:      fmt.Sprintf("%s-%s", charName, otherChar),
						Description: fmt.Sprintf("关系突变：%s 与 %s 的关系从 %s 突然变为 %s，缺乏铺垫", charName, otherChar, prevRel, currRel),
					})
				}
			}
		}
	}
}

// checkInventoryConsistency 检查物品连续性
func (cc *CrossChapterChecker) checkInventoryConsistency() {
	for i := 1; i < len(cc.chapters); i++ {
		prev := cc.chapters[i-1]
		curr := cc.chapters[i]

		for charName, prevState := range prev.CharacterStates {
			currState, exists := curr.CharacterStates[charName]
			if !exists {
				continue
			}

			// 检查重要物品消失
			for _, item := range prevState.Inventory {
				if isImportantItem(item) && !contains(currState.Inventory, item) {
					// 检查是否有丢弃或使用事件
					if !hasItemEvent(curr.Events, charName, item, "lose") {
						cc.addIssue(CrossChapterIssue{
							Type:        "continuity_error",
							Category:    "character",
							Severity:    "warning",
							ChapterFrom: prev.ChapterID,
							ChapterTo:   curr.ChapterID,
							Target:      charName,
							Description: fmt.Sprintf("物品消失：%s 失去了 %s 但没有相关事件", charName, item),
						})
					}
				}
			}
		}
	}
}

// addIssue 添加问题（去重）
func (cc *CrossChapterChecker) addIssue(issue CrossChapterIssue) {
	// 简单去重
	for _, existing := range cc.issues {
		if existing.Type == issue.Type &&
			existing.Target == issue.Target &&
			existing.ChapterFrom == issue.ChapterFrom &&
			existing.ChapterTo == issue.ChapterTo {
			return
		}
	}
	cc.issues = append(cc.issues, issue)
}

// ============================================================
// 辅助函数
// ============================================================

// parseCultivationLevel 解析修为等级为数值
func parseCultivationLevel(level string) int {
	// 修仙体系等级映射
	levels := map[string]int{
		"练气": 1, "筑基": 2, "金丹": 3, "元婴": 4,
		"化神": 5, "合体": 6, "大乘": 7, "渡劫": 8,
		"后天": 1, "先天": 2, "宗师": 3, "大宗师": 4,
		"武王": 5, "武皇": 6, "武帝": 7, "武圣": 8, "武神": 9,
		"斗者": 1, "斗师": 2, "大斗师": 3, "斗灵": 4,
		"斗王": 5, "斗皇": 6, "斗宗": 7, "斗尊": 8, "斗圣": 9, "斗帝": 10,
	}

	for key, val := range levels {
		if strings.Contains(level, key) {
			// 尝试提取层数
			re := regexp.MustCompile(`(\d+)`)
			if match := re.FindString(level); match != "" {
				if num, err := strconv.Atoi(match); err == nil {
					return val*100 + num
				}
			}
			return val * 100
		}
	}
	return 0
}

// parseRelativeTime 解析相对时间为数值（天数）
func parseRelativeTime(desc string) int {
	// 简单解析
	if strings.Contains(desc, "前") {
		return -1
	}
	if strings.Contains(desc, "后") {
		return 1
	}
	return 0
}

// hasRecoveryEvent 检查是否有恢复事件
func hasRecoveryEvent(events []EventSnapshot, charName string) bool {
	recoveryKeywords := []string{"疗伤", "恢复", "治疗", "丹药", "修养", "闭关", " healed"}
	for _, event := range events {
		if contains(event.Characters, charName) {
			for _, keyword := range recoveryKeywords {
				if strings.Contains(event.Details, keyword) || strings.Contains(event.Action, keyword) {
					return true
				}
			}
		}
	}
	return false
}

// estimateEnemyPower 估计敌人战力
func estimateEnemyPower(events []EventSnapshot, winnerName string) int {
	for _, event := range events {
		if event.Type == "combat" && contains(event.Characters, winnerName) {
			// 从事件描述中估计
			if strings.Contains(event.Details, "元婴") || strings.Contains(event.Details, "斗尊") {
				return 5000
			}
			if strings.Contains(event.Details, "金丹") || strings.Contains(event.Details, "斗皇") {
				return 3000
			}
			if strings.Contains(event.Details, "筑基") || strings.Contains(event.Details, "斗王") {
				return 2000
			}
		}
	}
	return 1000
}

// estimateDistance 估计两地距离
func estimateDistance(from, to string) int {
	// 简单距离估计
	if strings.Contains(from, "宗门") && strings.Contains(to, "矿场") {
		return 3000
	}
	if strings.Contains(from, "矿场") && strings.Contains(to, "宗门") {
		return 3000
	}
	if strings.Contains(from, "城") && strings.Contains(to, "城") {
		return 2000
	}
	return 500
}

// isHostile 判断关系是否敌对
func isHostile(rel string) bool {
	hostile := []string{"enemy", "hostile", "仇恨", "敌对", "追杀", "死敌"}
	for _, h := range hostile {
		if strings.Contains(rel, h) {
			return true
		}
	}
	return false
}

// isFriendly 判断关系是否友好
func isFriendly(rel string) bool {
	friendly := []string{"friend", "ally", "friendly", "盟友", "朋友", "效忠", "忠诚", "师徒"}
	for _, f := range friendly {
		if strings.Contains(rel, f) {
			return true
		}
	}
	return false
}

// isImportantItem 判断物品是否重要
func isImportantItem(item string) bool {
	important := []string{"法宝", "灵器", "神器", "秘籍", "传承", "宝物"}
	for _, i := range important {
		if strings.Contains(item, i) {
			return true
		}
	}
	return false
}

// hasItemEvent 检查是否有物品相关事件
func hasItemEvent(events []EventSnapshot, charName, item, eventType string) bool {
	for _, event := range events {
		if contains(event.Characters, charName) {
			if strings.Contains(event.Details, item) || strings.Contains(event.Action, item) {
				if eventType == "lose" && (strings.Contains(event.Action, "丢") || strings.Contains(event.Action, "失去")) {
					return true
				}
				if eventType == "use" && (strings.Contains(event.Action, "用") || strings.Contains(event.Action, "使用")) {
					return true
				}
			}
		}
	}
	return false
}

// contains 检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ============================================================
// 从文本提取章节状态
// ============================================================

// ExtractChapterStateFromText 从章节文本提取状态
func ExtractChapterStateFromText(chapterID string, chapterNum int, text string) ChapterState {
	state := ChapterState{
		ChapterID:       chapterID,
		ChapterNum:      chapterNum,
		CharacterStates: make(map[string]*CharacterSnapshot),
		Events:          make([]EventSnapshot, 0),
	}

	// 简单的角色提取（使用正则）
	characterNames := extractCharacterNames(text)
	for _, name := range characterNames {
		snapshot := &CharacterSnapshot{
			Name:          name,
			Status:        "alive",
			Relationships: make(map[string]string),
		}

		// 检测状态
		if strings.Contains(text, name+"死了") || strings.Contains(text, name+"身亡") {
			snapshot.Status = "dead"
		} else if strings.Contains(text, name+"受伤") {
			snapshot.Status = "injured"
		}

		// 提取修为
		snapshot.Cultivation = extractCultivationFromText(text, name)

		state.CharacterStates[name] = snapshot
	}

	// 提取时间线信息
	state.Timeline = extractTimelineFromChapter(text)

	return state
}

// extractCharacterNames 简单提取角色名
func extractCharacterNames(text string) []string {
	// 常见姓氏
	surnames := []string{"林", "李", "王", "张", "刘", "陈", "杨", "赵", "黄", "周", "吴", "徐", "孙", "马", "朱", "胡"}

	found := make(map[string]bool)
	var names []string

	// 简单匹配：姓氏 + 1-2个字符
	for _, surname := range surnames {
		pattern := surname + "[一-龥]{1,2}"
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(text, -1)
		for _, match := range matches {
			if !found[match] && len(match) >= 2 && len(match) <= 4 {
				found[match] = true
				names = append(names, match)
			}
		}
	}

	return names
}

// extractCultivationFromText 从文本提取修为
func extractCultivationFromText(text, charName string) string {
	cultivations := []string{"练气", "筑基", "金丹", "元婴", "化神", "合体", "大乘", "渡劫"}

	for _, cult := range cultivations {
		if strings.Contains(text, charName+cult) || strings.Contains(text, cult) {
			// 尝试找层数
			re := regexp.MustCompile(cult + `([一二三四五六七八九十百千万]+)`)
			if match := re.FindString(text); match != "" {
				return match
			}
			return cult
		}
	}
	return ""
}

// extractTimelineFromChapter 从章节文本提取时间线
func extractTimelineFromChapter(text string) TimelineInfo {
	info := TimelineInfo{
		AbsoluteTime: 0,
		RelativeTime: "",
	}

	// 检测时间关键词
	timePatterns := []struct {
		pattern string
		days    int
	}{
		{`三天后`, 3},
		{`一周后`, 7},
		{`一个月后`, 30},
		{`三个月后`, 90},
		{`半年后`, 180},
		{`一年后`, 365},
		{`三年后`, 1095},
		{`十年后`, 3650},
	}

	for _, tp := range timePatterns {
		if matched, _ := regexp.MatchString(tp.pattern, text); matched {
			info.AbsoluteTime = tp.days
			info.RelativeTime = tp.pattern
			break
		}
	}

	// 检测季节
	seasons := map[string]string{
		"春": "spring", "夏": "summer", "秋": "autumn", "冬": "winter",
	}
	for cn, en := range seasons {
		if strings.Contains(text, cn) {
			info.Season = en
			break
		}
	}

	return info
}

// ============================================================
// 跨章指标计算
// ============================================================

// CrossChapterMetrics 跨章一致性指标
type CrossChapterMetrics struct {
	TotalChecks       int
	PassedChecks      int
	FailedChecks      int
	IssueCount        int
	CriticalIssues    int
	ErrorIssues       int
	WarningIssues     int
	ConsistencyScore  float64 // 0-100
	CategoryBreakdown map[string]int
}

// CalculateCrossChapterMetrics 计算跨章指标
func CalculateCrossChapterMetrics(issues []CrossChapterIssue) CrossChapterMetrics {
	m := CrossChapterMetrics{
		CategoryBreakdown: make(map[string]int),
	}

	m.IssueCount = len(issues)
	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			m.CriticalIssues++
		case "error":
			m.ErrorIssues++
		case "warning":
			m.WarningIssues++
		}
		m.CategoryBreakdown[issue.Category]++
	}

	// 计算一致性得分（扣分制）
	m.ConsistencyScore = 100
	m.ConsistencyScore -= float64(m.CriticalIssues) * 20
	m.ConsistencyScore -= float64(m.ErrorIssues) * 10
	m.ConsistencyScore -= float64(m.WarningIssues) * 3

	if m.ConsistencyScore < 0 {
		m.ConsistencyScore = 0
	}

	return m
}