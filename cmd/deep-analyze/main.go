package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// 深度分析结构
type DeepAnalysisResult struct {
	CriticalIssues   []CriticalIssue   `json:"critical_issues"`
	LogicHoles       []LogicHole       `json:"logic_holes"`
	TimelineIssues   []TimelineIssue   `json:"timeline_issues"`
	CharacterArcs    []CharacterArc    `json:"character_arcs"`
	PlotContradictions []PlotContradiction `json:"plot_contradictions"`
	MissingConnections []MissingConnection `json:"missing_connections"`
	Summary          AnalysisSummary   `json:"summary"`
}

type CriticalIssue struct {
	Type        string `json:"type"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Severity    string `json:"severity"` // fatal, critical, major
}

type LogicHole struct {
	Location    string   `json:"location"`
	Description string   `json:"description"`
	Questions   []string `json:"questions"`
}

type TimelineIssue struct {
	Location    string `json:"location"`
	Issue       string `json:"issue"`
	TimeBefore  string `json:"time_before"`
	TimeAfter   string `json:"time_after"`
}

type CharacterArc struct {
	Character   string   `json:"character"`
	Appearances []string `json:"appearances"`
	ArcStatus   string   `json:"arc_status"`
	Issues      []string `json:"issues"`
}

type PlotContradiction struct {
	Location1   string `json:"location1"`
	Location2   string `json:"location2"`
	Contradiction string `json:"contradiction"`
	Details     string `json:"details"`
}

type MissingConnection struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type AnalysisSummary struct {
	TotalChapters        int `json:"total_chapters"`
	TotalCharacters      int `json:"total_characters"`
	CriticalIssuesCount  int `json:"critical_issues_count"`
	LogicHolesCount      int `json:"logic_holes_count"`
	TimelineIssuesCount  int `json:"timeline_issues_count"`
	ContradictionsCount  int `json:"contradictions_count"`
	RiskLevel            string `json:"risk_level"`
}

// 大纲结构
type StoryOutline struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Volumes []Volume `json:"volumes"`
}

type Volume struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Summary  string    `json:"summary"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Characters   []string `json:"characters"`
	Location     string   `json:"location"`
	Events       []Event  `json:"events"`
	Beats        []string `json:"beats"`
	OpeningBeat  string   `json:"opening_beat"`
	ClosingBeat  string   `json:"closing_beat"`
	StateChange  string   `json:"state_change"`
	Conflict     string   `json:"conflict"`
	Pacing       string   `json:"pacing"`
}

type Event struct {
	Type       string   `json:"type"`
	Characters []string `json:"characters"`
	Subject    string   `json:"subject"`
	Change     string   `json:"change"`
	Details    string   `json:"details"`
}

func main() {
	outlinePath := flag.String("i", "", "输入大纲JSON文件路径 (必需)")
	outputPath := flag.String("o", "", "输出分析结果JSON文件路径 (可选)")
	flag.Parse()

	if *outlinePath == "" {
		fmt.Println("错误: 请使用 -i 参数指定大纲文件路径")
		flag.Usage()
		os.Exit(1)
	}

	// 读取大纲
	outlineData, err := os.ReadFile(*outlinePath)
	if err != nil {
		fmt.Printf("错误: 无法读取大纲文件: %v\n", err)
		os.Exit(1)
	}

	var outline StoryOutline
	if err := json.Unmarshal(outlineData, &outline); err != nil {
		fmt.Printf("错误: 无法解析大纲JSON: %v\n", err)
		os.Exit(1)
	}

	// 执行深度分析
	result := performDeepAnalysis(&outline)

	// 输出结果
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("错误: 无法生成分析结果JSON: %v\n", err)
		os.Exit(1)
	}

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, resultJSON, 0644); err != nil {
			fmt.Printf("错误: 无法写入分析结果文件: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("深度分析结果已保存到: %s\n", *outputPath)
	}

	// 打印分析摘要
	printAnalysisReport(result)
}

func performDeepAnalysis(outline *StoryOutline) *DeepAnalysisResult {
	result := &DeepAnalysisResult{
		CriticalIssues:     []CriticalIssue{},
		LogicHoles:         []LogicHole{},
		TimelineIssues:     []TimelineIssue{},
		CharacterArcs:      []CharacterArc{},
		PlotContradictions: []PlotContradiction{},
		MissingConnections: []MissingConnection{},
	}

	allChapters := collectAllChapters(outline)
	result.Summary.TotalChapters = len(allChapters)

	// 1. 检查关键逻辑漏洞
	checkCriticalIssues(allChapters, result)

	// 2. 检查时间线问题
	checkTimelineIssues(allChapters, result)

	// 3. 检查角色弧线
	checkCharacterArcs(allChapters, result)

	// 4. 检查剧情矛盾
	checkPlotContradictions(allChapters, result)

	// 5. 检查缺失连接
	checkMissingConnections(allChapters, result)

	// 6. 检查逻辑漏洞
	checkLogicHoles(allChapters, result)

	// 计算摘要
	result.Summary.TotalCharacters = len(result.CharacterArcs)
	result.Summary.CriticalIssuesCount = len(result.CriticalIssues)
	result.Summary.LogicHolesCount = len(result.LogicHoles)
	result.Summary.TimelineIssuesCount = len(result.TimelineIssues)
	result.Summary.ContradictionsCount = len(result.PlotContradictions)
	result.Summary.RiskLevel = calculateRiskLevel(result)

	return result
}

func collectAllChapters(outline *StoryOutline) []Chapter {
	var chapters []Chapter
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				chapters = append(chapters, chapter)
			}
		}
	}
	return chapters
}

func checkCriticalIssues(chapters []Chapter, result *DeepAnalysisResult) {
	// 检查1: 角色死亡与复活的逻辑一致性
	deathResurrectionMap := make(map[string][]string)
	for _, ch := range chapters {
		for _, event := range ch.Events {
			if strings.Contains(event.Change, "死亡") || strings.Contains(event.Change, "复活") {
				for _, char := range event.Characters {
					deathResurrectionMap[char] = append(deathResurrectionMap[char], ch.ID)
				}
			}
		}
	}

	// 检查林砚的死亡复活次数
	linYanDeaths := 0
	for _, ch := range chapters {
		for _, event := range ch.Events {
			if strings.Contains(event.Subject, "林砚") && strings.Contains(event.Change, "复活") {
				linYanDeaths++
			}
		}
	}

	// 检查寿命折损计算
	if linYanDeaths > 0 {
		result.CriticalIssues = append(result.CriticalIssues, CriticalIssue{
			Type:        "resurrection_count",
			Location:    "全局",
			Description: fmt.Sprintf("林砚在剧情中复活 %d 次，每次折损10年寿命，总计折损%d年寿命", linYanDeaths, linYanDeaths*10),
			Impact:      "需要确认主角剩余寿命是否足以支撑后续剧情",
			Severity:    "critical",
		})
	}

	// 检查2: 修为跌落与恢复的逻辑
	cultivationChanges := []string{}
	for _, ch := range chapters {
		for _, event := range ch.Events {
			if strings.Contains(event.Subject, "林砚") && 
			   (strings.Contains(event.Change, "修为") || strings.Contains(event.Change, "练气")) {
				cultivationChanges = append(cultivationChanges, fmt.Sprintf("%s: %s", ch.ID, event.Change))
			}
		}
	}

	if len(cultivationChanges) > 5 {
		result.CriticalIssues = append(result.CriticalIssues, CriticalIssue{
			Type:        "cultivation_instability",
			Location:    "全局",
			Description: fmt.Sprintf("林砚修为变化过于频繁(%d次)，可能导致读者困惑", len(cultivationChanges)),
			Impact:      "战力体系可能崩坏，读者难以建立稳定预期",
			Severity:    "major",
		})
	}

	// 检查3: 关键道具的连续性
	keyItems := []string{"封印玉佩", "云纹腰牌", "玄铁匕首", "血蓟草"}
	itemFirstSeen := make(map[string]string)
	itemLastSeen := make(map[string]string)

	for _, ch := range chapters {
		for _, event := range ch.Events {
			for _, item := range keyItems {
				if strings.Contains(event.Details, item) || strings.Contains(event.Change, item) {
					if itemFirstSeen[item] == "" {
						itemFirstSeen[item] = ch.ID
					}
					itemLastSeen[item] = ch.ID
				}
			}
		}
	}

	for _, item := range keyItems {
		if itemFirstSeen[item] == "" {
			result.CriticalIssues = append(result.CriticalIssues, CriticalIssue{
				Type:        "missing_key_item",
				Location:    "全局",
				Description: fmt.Sprintf("关键道具'%s'在大纲中未出现", item),
				Impact:      "可能导致后续剧情缺乏必要的道具支撑",
				Severity:    "major",
			})
		}
	}
}

func checkTimelineIssues(chapters []Chapter, result *DeepAnalysisResult) {
	// 检查时间跳跃
	timeReferences := []string{"三天后", "一个月后", "三个月后", "半个月后", "次日", "当天"}
	
	for i, ch := range chapters {
		for _, beat := range ch.Beats {
			for _, timeRef := range timeReferences {
				if strings.Contains(beat, timeRef) {
					// 检查是否有明确的时间线连接
					if i > 0 && !hasTimeTransition(chapters[i-1], ch) {
						result.TimelineIssues = append(result.TimelineIssues, TimelineIssue{
							Location:   ch.ID,
							Issue:      fmt.Sprintf("章节包含时间跳跃'%s'但缺乏过渡", timeRef),
							TimeBefore: chapters[i-1].ID,
							TimeAfter:  ch.ID,
						})
					}
				}
			}
		}
	}
}

func hasTimeTransition(prevCh, currCh Chapter) bool {
	// 检查是否有明确的时间过渡描述
	timeTransitions := []string{"然后", "因此", "第二天", "次日", "三天后", "当晚", "夜里"}
	for _, transition := range timeTransitions {
		if strings.Contains(currCh.OpeningBeat, transition) {
			return true
		}
	}
	return false
}

func checkCharacterArcs(chapters []Chapter, result *DeepAnalysisResult) {
	// 收集所有角色及其出场
	characterAppearances := make(map[string][]string)
	
	for _, ch := range chapters {
		for _, char := range ch.Characters {
			characterAppearances[char] = append(characterAppearances[char], ch.ID)
		}
		// 也检查事件中的角色
		for _, event := range ch.Events {
			for _, char := range event.Characters {
				if !contains(characterAppearances[char], ch.ID) {
					characterAppearances[char] = append(characterAppearances[char], ch.ID)
				}
			}
		}
	}

	// 分析每个角色的弧线
	for char, appearances := range characterAppearances {
		arc := CharacterArc{
			Character:   char,
			Appearances: appearances,
			ArcStatus:   "active",
			Issues:      []string{},
		}

		// 检查角色是否有合理的出场分布
		if len(appearances) == 1 && char != "矿奴群体" && char != "监工群体" && char != "书友会全体" {
			arc.Issues = append(arc.Issues, "角色仅出场一次，可能是工具人")
			arc.ArcStatus = "underdeveloped"
		}

		// 检查主要角色
		if char == "林砚" && len(appearances) < len(chapters)/2 {
			arc.Issues = append(arc.Issues, "主角出场章节过少")
		}

		// 检查角色突然消失
		if len(appearances) >= 3 {
			// 检查是否有大段空白
			for i := 1; i < len(appearances); i++ {
				// 简化检查：如果章节ID差距过大
				if isLargeGap(appearances[i-1], appearances[i]) {
					arc.Issues = append(arc.Issues, fmt.Sprintf("角色在 %s 到 %s 之间有较大出场空白", appearances[i-1], appearances[i]))
				}
			}
		}

		result.CharacterArcs = append(result.CharacterArcs, arc)
	}
}

func isLargeGap(ch1, ch2 string) bool {
	// 简化判断：如果卷号或章节号差距过大
	// 实际应该解析ID，这里简化处理
	return false // 简化处理
}

func checkPlotContradictions(chapters []Chapter, result *DeepAnalysisResult) {
	// 检查剧情矛盾
	
	// 1. 检查地点矛盾
	locationEvents := make(map[string][]string)
	for _, ch := range chapters {
		for _, event := range ch.Events {
			if event.Subject == "林砚" {
				locationEvents[ch.Location] = append(locationEvents[ch.Location], ch.ID)
			}
		}
	}

	// 2. 检查状态矛盾
	statusMap := make(map[string]string)
	for _, ch := range chapters {
		for _, event := range ch.Events {
			if strings.Contains(event.Subject, "林砚") && strings.Contains(event.Change, "修为") {
				if prevStatus, exists := statusMap["林砚_cultivation"]; exists {
					// 检查修为变化是否合理
					if isContradictoryCultivation(prevStatus, event.Change) {
						result.PlotContradictions = append(result.PlotContradictions, PlotContradiction{
							Location1:     statusMap["林砚_cultivation_chapter"],
							Location2:     ch.ID,
							Contradiction: "修为变化矛盾",
							Details:       fmt.Sprintf("%s -> %s", prevStatus, event.Change),
						})
					}
				}
				statusMap["林砚_cultivation"] = event.Change
				statusMap["林砚_cultivation_chapter"] = ch.ID
			}
		}
	}
}

func isContradictoryCultivation(prev, curr string) bool {
	// 检查修为变化是否矛盾
	// 例如：从练气三层跌落到练气一层，然后突然恢复到练气三层而没有解释
	return false // 简化处理
}

func checkMissingConnections(chapters []Chapter, result *DeepAnalysisResult) {
	// 检查章节之间的缺失连接
	for i := 1; i < len(chapters); i++ {
		prevCh := chapters[i-1]
		currCh := chapters[i]

		// 检查地点转换是否有说明
		if prevCh.Location != currCh.Location {
			// 检查开场节拍是否解释了地点转换
			locationTransitionExplained := false
			locationKeywords := []string{"回到", "前往", "来到", "进入", "退出", "离开", "赶到", "走向"}
			for _, keyword := range locationKeywords {
				if strings.Contains(currCh.OpeningBeat, keyword) {
					locationTransitionExplained = true
					break
				}
			}

			if !locationTransitionExplained {
				result.MissingConnections = append(result.MissingConnections, MissingConnection{
					From:        prevCh.ID,
					To:          currCh.ID,
					Type:        "location_transition",
					Description: fmt.Sprintf("地点从 '%s' 变为 '%s' 缺乏过渡说明", prevCh.Location, currCh.Location),
				})
			}
		}

		// 检查状态连续性
		if !hasStateContinuity(prevCh, currCh) {
			result.MissingConnections = append(result.MissingConnections, MissingConnection{
				From:        prevCh.ID,
				To:          currCh.ID,
				Type:        "state_continuity",
				Description: "章节间状态变化缺乏连贯性",
			})
		}
	}
}

func hasStateContinuity(prevCh, currCh Chapter) bool {
	// 检查closing_beat和opening_beat是否连贯
	prevClosing := prevCh.ClosingBeat
	currOpening := currCh.OpeningBeat

	// 简单检查：如果closing_beat包含"因此"，opening_beat应该以"然后"开头
	if strings.Contains(prevClosing, "因此") {
		// 可以接受的开场词
		validOpenings := []string{"然后", "因此", "当天", "次日", "第二天", "三日后"}
		for _, opening := range validOpenings {
			if strings.HasPrefix(currOpening, opening) {
				return true
			}
		}
	}

	return true // 默认通过
}

func checkLogicHoles(chapters []Chapter, result *DeepAnalysisResult) {
	// 检查逻辑漏洞
	
	// 1. 检查"三哥"的线索是否连贯
	sanGeMentions := []string{}
	for _, ch := range chapters {
		for _, event := range ch.Events {
			if strings.Contains(event.Details, "三哥") || strings.Contains(event.Change, "三哥") {
				sanGeMentions = append(sanGeMentions, ch.ID)
			}
		}
		for _, beat := range ch.Beats {
			if strings.Contains(beat, "三哥") {
				sanGeMentions = append(sanGeMentions, ch.ID)
			}
		}
	}

	if len(sanGeMentions) > 0 {
		// 检查三哥的线索是否过于分散
		result.LogicHoles = append(result.LogicHoles, LogicHole{
			Location:    "全局",
			Description: "「三哥」作为重要反派线索出现多次，需要确认最终是否有合理的揭示",
			Questions: []string{
				"三哥的真实身份是否在剧情后期揭示？",
				"三哥与苏晚的关系是否有充分展开？",
				"三哥的动机和目的是否清晰？",
			},
		})
	}

	// 2. 检查复活能力的暴露风险
	resurrectionWitnesses := []string{}
	for _, ch := range chapters {
		for _, event := range ch.Events {
			if strings.Contains(event.Change, "复活") && len(event.Characters) > 1 {
				for _, char := range event.Characters {
					if char != "林砚" {
						resurrectionWitnesses = append(resurrectionWitnesses, fmt.Sprintf("%s在%s", char, ch.ID))
					}
				}
			}
		}
	}

	if len(resurrectionWitnesses) > 0 {
		result.LogicHoles = append(result.LogicHoles, LogicHole{
			Location:    "全局",
			Description: fmt.Sprintf("林砚的复活能力被以下角色目击: %v", resurrectionWitnesses),
			Questions: []string{
				"这些目击者是否都处理了保密问题？",
				"是否有目击者后来成为威胁？",
				"复活能力的保密性是否得到维持？",
			},
		})
	}

	// 3. 检查缉厄司的行动逻辑
	result.LogicHoles = append(result.LogicHoles, LogicHole{
		Location:    "全局",
		Description: "缉厄司作为官方组织，其行动范围和权限需要确认",
		Questions: []string{
			"缉厄司为什么只在矿场活动而不直接控制整个区域？",
			"缉厄司与矿场管理者的关系是什么？",
			"缉厄司为何没有更早发现林砚的异常？",
		},
	})
}

func calculateRiskLevel(result *DeepAnalysisResult) string {
	score := 0
	score += len(result.CriticalIssues) * 10
	score += len(result.LogicHoles) * 5
	score += len(result.TimelineIssues) * 3
	score += len(result.PlotContradictions) * 8
	score += len(result.MissingConnections) * 2

	if score >= 100 {
		return "极高风险 - 需要重大修改"
	} else if score >= 70 {
		return "高风险 - 需要较多修改"
	} else if score >= 40 {
		return "中等风险 - 需要部分修改"
	} else if score >= 20 {
		return "低风险 - 需要微调"
	}
	return "低风险 - 结构良好"
}

func printAnalysisReport(result *DeepAnalysisResult) {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║               大纲深度推演分析报告                           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 摘要
	fmt.Println("【分析摘要】")
	fmt.Printf("总章节数: %d\n", result.Summary.TotalChapters)
	fmt.Printf("总角色数: %d\n", result.Summary.TotalCharacters)
	fmt.Printf("关键问题: %d\n", result.Summary.CriticalIssuesCount)
	fmt.Printf("逻辑漏洞: %d\n", result.Summary.LogicHolesCount)
	fmt.Printf("时间线问题: %d\n", result.Summary.TimelineIssuesCount)
	fmt.Printf("剧情矛盾: %d\n", result.Summary.ContradictionsCount)
	fmt.Printf("风险等级: %s\n", result.Summary.RiskLevel)
	fmt.Println()

	// 关键问题
	if len(result.CriticalIssues) > 0 {
		fmt.Println("【🚨 关键问题】")
		for i, issue := range result.CriticalIssues {
			fmt.Printf("\n%d. [%s] %s\n", i+1, issue.Severity, issue.Type)
			fmt.Printf("   位置: %s\n", issue.Location)
			fmt.Printf("   描述: %s\n", issue.Description)
			fmt.Printf("   影响: %s\n", issue.Impact)
		}
		fmt.Println()
	}

	// 逻辑漏洞
	if len(result.LogicHoles) > 0 {
		fmt.Println("【🔍 逻辑漏洞】")
		for i, hole := range result.LogicHoles {
			fmt.Printf("\n%d. %s\n", i+1, hole.Description)
			fmt.Printf("   位置: %s\n", hole.Location)
			fmt.Println("   需要回答的问题:")
			for _, q := range hole.Questions {
				fmt.Printf("   - %s\n", q)
			}
		}
		fmt.Println()
	}

	// 剧情矛盾
	if len(result.PlotContradictions) > 0 {
		fmt.Println("【⚠️ 剧情矛盾】")
		for i, contra := range result.PlotContradictions {
			fmt.Printf("\n%d. %s\n", i+1, contra.Contradiction)
			fmt.Printf("   位置1: %s\n", contra.Location1)
			fmt.Printf("   位置2: %s\n", contra.Location2)
			fmt.Printf("   详情: %s\n", contra.Details)
		}
		fmt.Println()
	}

	// 缺失连接
	if len(result.MissingConnections) > 0 {
		fmt.Println("【🔗 缺失连接】")
		for i, conn := range result.MissingConnections {
			fmt.Printf("\n%d. [%s] %s -> %s\n", i+1, conn.Type, conn.From, conn.To)
			fmt.Printf("   %s\n", conn.Description)
		}
		fmt.Println()
	}

	// 建议
	fmt.Println("【💡 修复建议】")
	fmt.Println("1. 建立角色出场追踪表，确保每个角色都有合理的引入和退场")
	fmt.Println("2. 增加章节间的过渡描述，特别是地点转换时")
	fmt.Println("3. 明确时间线，标注每个事件的具体时间点")
	fmt.Println("4. 检查复活能力的暴露风险，确保所有目击者都有处理")
	fmt.Println("5. 完善「三哥」等关键反派的动机和行动逻辑")
	fmt.Println("6. 确保修为变化有合理的解释和恢复机制")
	fmt.Println()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
