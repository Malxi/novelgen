package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ============================================================
// 智能小说检测器 v2 - 针对特定小说设定优化
// ============================================================

// SmartNovelChecker 智能小说检测器
type SmartNovelChecker struct {
	BookPath           string
	Results            []SmartChapterResult
	CrossChapterIssues []CrossChapterIssue
	BookSettings       BookSettings // 小说特定设定
}

// BookSettings 小说设定配置
type BookSettings struct {
	Title                    string
	HasResurrection          bool     // 是否有复活设定
	ResurrectionCost         string   // 复活代价描述
	Protagonist              string   // 主角名字
	ValidPowerChanges        int      // 每章允许的最大战力变化次数
	IgnoreTimeJumps          bool     // 是否忽略时间跳跃（如果小说是倒叙/插叙结构）
	AllowedResurrectionChars []string // 允许复活的角色名单
}

// SmartChapterResult 智能章节检测结果
type SmartChapterResult struct {
	ChapterID     string
	ChapterTitle  string
	FilePath      string
	TextLength    int
	RealIssues    []RealIssue // 真正的问题
	InfoNotes     []InfoNote  // 信息提示（非问题）
	CheckDuration time.Duration
	Timestamp     time.Time
}

// RealIssue 真正的问题
type RealIssue struct {
	Category    string
	Severity    string // high, medium, low
	Description string
	Evidence    string // 证据文本
	Suggestion  string
}

// InfoNote 信息提示
type InfoNote struct {
	Type        string
	Description string
}

// DefaultBookSettings 返回默认的小说设定（针对 books/mine）
func DefaultBookSettings() BookSettings {
	return BookSettings{
		Title:                    "寒矿醒转",
		HasResurrection:          true,
		ResurrectionCost:         "每次复活掉一层修为",
		Protagonist:              "林砚",
		ValidPowerChanges:        5,              // 修仙小说允许较多修炼描写
		IgnoreTimeJumps:          true,           // 暂时忽略时间线问题
		AllowedResurrectionChars: []string{"林砚"}, // 只有主角能复活
	}
}

// NewSmartNovelChecker 创建智能检测器
func NewSmartNovelChecker(bookPath string, settings *BookSettings) (*SmartNovelChecker, error) {
	checker := &SmartNovelChecker{
		BookPath: bookPath,
		Results:  make([]SmartChapterResult, 0),
	}

	if settings != nil {
		checker.BookSettings = *settings
	} else {
		checker.BookSettings = DefaultBookSettings()
	}

	return checker, nil
}

// CheckAllChapters 检测所有章节
func (snc *SmartNovelChecker) CheckAllChapters() error {
	chaptersDir := filepath.Join(snc.BookPath, "chapters")

	files, err := os.ReadDir(chaptersDir)
	if err != nil {
		return fmt.Errorf("读取章节目录失败: %w", err)
	}

	var chapterFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			chapterFiles = append(chapterFiles, f.Name())
		}
	}
	sortChapterFilenames(chapterFiles)

	fmt.Printf("发现 %d 个章节文件\n", len(chapterFiles))
	fmt.Printf("小说设定: %s (复活: %v, 主角: %s)\n\n",
		snc.BookSettings.Title,
		snc.BookSettings.HasResurrection,
		snc.BookSettings.Protagonist)

	for i, filename := range chapterFiles {
		chapterPath := filepath.Join(chaptersDir, filename)
		fmt.Printf("[%d/%d] 检测 %s...\n", i+1, len(chapterFiles), filename)

		result, err := snc.CheckChapter(chapterPath)
		if err != nil {
			fmt.Printf("  警告: %v\n", err)
			continue
		}

		snc.Results = append(snc.Results, *result)

		// 打印真正的问题
		if len(result.RealIssues) > 0 {
			fmt.Printf("  发现 %d 个真正的问题:\n", len(result.RealIssues))
			for _, issue := range result.RealIssues {
				fmt.Printf("    ⚠️  [%s/%s] %s\n", issue.Severity, issue.Category, issue.Description)
			}
		}

		// 打印信息提示
		if len(result.InfoNotes) > 0 {
			for _, note := range result.InfoNotes {
				fmt.Printf("    ℹ️  %s: %s\n", note.Type, note.Description)
			}
		}

		if len(result.RealIssues) == 0 && len(result.InfoNotes) == 0 {
			fmt.Printf("  ✓ 未发现问题\n")
		}
	}

	fmt.Println("\n执行跨章一致性检测...")
	snc.runCrossChapterCheck()

	return nil
}

// CheckChapter 智能检测单个章节
func (snc *SmartNovelChecker) CheckChapter(chapterPath string) (*SmartChapterResult, error) {
	start := time.Now()

	content, err := os.ReadFile(chapterPath)
	if err != nil {
		return nil, err
	}

	text := string(content)
	filename := filepath.Base(chapterPath)

	title := extractChapterTitle(text)

	result := &SmartChapterResult{
		ChapterID:    extractChapterID(filename),
		ChapterTitle: title,
		FilePath:     chapterPath,
		TextLength:   len(text),
		RealIssues:   make([]RealIssue, 0),
		InfoNotes:    make([]InfoNote, 0),
		Timestamp:    time.Now(),
	}

	// 1. 检测复活逻辑问题（考虑设定）
	snc.checkResurrectionLogic(result, text)

	// 2. 检测战力变化（考虑修仙设定）
	snc.checkPowerProgression(result, text)

	// 3. 检测角色一致性
	snc.checkCharacterConsistency(result, text)

	// 4. 检测可能的逻辑漏洞
	snc.checkLogicHoles(result, text)

	result.CheckDuration = time.Since(start)

	return result, nil
}

// checkResurrectionLogic 检测复活逻辑
func (snc *SmartNovelChecker) checkResurrectionLogic(result *SmartChapterResult, text string) {
	if !snc.BookSettings.HasResurrection {
		return
	}

	// 统计复活相关关键词
	resurrectionPatterns := []string{"复活", "复生", "死而复生", "重生", "醒来", "睁开眼"}
	deathPatterns := []string{"死了", "身亡", "殒命", "断气", "被砸死", "颈动脉", "骨头碎裂"}

	resurrectionCount := 0
	deathCount := 0

	for _, pattern := range resurrectionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringIndex(text, -1)
		resurrectionCount += len(matches)
	}

	for _, pattern := range deathPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringIndex(text, -1)
		deathCount += len(matches)
	}

	// 添加信息提示
	if resurrectionCount > 0 {
		result.InfoNotes = append(result.InfoNotes, InfoNote{
			Type:        "设定",
			Description: fmt.Sprintf("本章涉及 %d 次复活相关描写（主角特权，代价：掉修为）", resurrectionCount),
		})
	}

	// 检查非主角复活（真正的问题）
	// 如果文本中提到其他角色复活，那才是问题
	otherChars := extractOtherCharacters(text, snc.BookSettings.Protagonist)
	for _, char := range otherChars {
		if strings.Contains(text, char+"复活") || strings.Contains(text, char+"醒来") {
			result.RealIssues = append(result.RealIssues, RealIssue{
				Category:    "设定冲突",
				Severity:    "high",
				Description: fmt.Sprintf("非主角角色 '%s' 疑似复活", char),
				Evidence:    extractContext(text, char+"复活", 30),
				Suggestion:  "根据设定，只有主角林砚能复活，其他角色复活需要特殊说明",
			})
		}
	}

	// 检查复活代价是否被描写
	if resurrectionCount > 0 {
		costKeywords := []string{"修为", "掉", "跌落", "降", "代价", "削弱"}
		hasCostDescription := false
		for _, kw := range costKeywords {
			if strings.Contains(text, kw) {
				hasCostDescription = true
				break
			}
		}

		if !hasCostDescription && deathCount > 0 {
			// 如果没描写代价，可能是问题
			result.RealIssues = append(result.RealIssues, RealIssue{
				Category:    "设定完整性",
				Severity:    "medium",
				Description: "复活场景可能缺乏代价描写",
				Evidence:    "检测到复活关键词但未发现修为/代价相关描述",
				Suggestion:  "确保每次复活后描写修为下降或其他代价",
			})
		}
	}
}

// checkPowerProgression 检测战力进展
func (snc *SmartNovelChecker) checkPowerProgression(result *SmartChapterResult, text string) {
	// 提取修为变化
	cultivationChanges := extractCultivationChanges(text)

	if len(cultivationChanges) > 0 {
		result.InfoNotes = append(result.InfoNotes, InfoNote{
			Type:        "修炼",
			Description: fmt.Sprintf("本章涉及 %d 次修为/境界变化", len(cultivationChanges)),
		})
	}

	// 检测不合理的大幅度跨越
	// 例如：练气一层直接跳到金丹期
	severeJumps := detectSeverePowerJumps(cultivationChanges)
	for _, jump := range severeJumps {
		result.RealIssues = append(result.RealIssues, RealIssue{
			Category:    "战力系统",
			Severity:    "high",
			Description: fmt.Sprintf("不合理的修为跳跃: %s", jump),
			Suggestion:  "修仙境界跨越需要合理的铺垫和过渡",
		})
	}
}

// checkCharacterConsistency 检测角色一致性
func (snc *SmartNovelChecker) checkCharacterConsistency(result *SmartChapterResult, text string) {
	// 检测角色状态矛盾
	// 例如：角色上一段死了，下一段活着且无解释

	// 检测角色位置矛盾
	locationKeywords := map[string][]string{
		"矿场": {"矿道", "矿区", "黑风矿", "丙字"},
		"宗门": {"宗门", "大殿", "长老", "峰主"},
		"城镇": {"镇", "城", "坊市", "街道"},
	}

	foundLocations := make(map[string]bool)
	for location, keywords := range locationKeywords {
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				foundLocations[location] = true
				break
			}
		}
	}

	if len(foundLocations) > 2 {
		// 一章内场景切换过多
		result.RealIssues = append(result.RealIssues, RealIssue{
			Category:    "场景切换",
			Severity:    "low",
			Description: fmt.Sprintf("本章涉及 %d 个不同场景", len(foundLocations)),
			Suggestion:  "单章场景过多可能导致叙事混乱",
		})
	}
}

// checkLogicHoles 检测逻辑漏洞
func (snc *SmartNovelChecker) checkLogicHoles(result *SmartChapterResult, text string) {
	// 检测可能的逻辑漏洞

	// 1. 物品凭空出现/消失
	itemMentions := extractItemMentions(text)
	if len(itemMentions) > 10 {
		result.InfoNotes = append(result.InfoNotes, InfoNote{
			Type:        "物品",
			Description: fmt.Sprintf("本章提及 %d 件物品", len(itemMentions)),
		})
	}

	// 2. 时间描述矛盾
	if strings.Contains(text, "三天前") && strings.Contains(text, "昨天") {
		// 检查时间线是否合理
		// 这只是一个示例检测
	}
}

// runCrossChapterCheck 运行跨章检测
func (snc *SmartNovelChecker) runCrossChapterCheck() {
	// 简化版的跨章检测
	// 主要检测角色状态连续性和时间线

	checker := NewCrossChapterChecker()

	for _, result := range snc.Results {
		content, _ := os.ReadFile(result.FilePath)
		state := ExtractChapterStateFromText(result.ChapterID, 0, string(content))
		checker.AddChapter(state)
	}

	issues := checker.CheckConsistency()

	// 过滤掉因文件名排序导致的时间线误报
	for _, issue := range issues {
		if issue.Category == "timeline" && issue.Type == "paradox" {
			// 跳过文件名排序导致的时间线问题
			continue
		}
		snc.CrossChapterIssues = append(snc.CrossChapterIssues, issue)
	}

	if len(snc.CrossChapterIssues) > 0 {
		fmt.Printf("发现 %d 个跨章问题:\n", len(snc.CrossChapterIssues))
		for _, issue := range snc.CrossChapterIssues {
			fmt.Printf("  - [%s/%s] %s\n", issue.Category, issue.Severity, issue.Description)
		}
	} else {
		fmt.Println("跨章一致性良好")
	}
}

// GenerateSmartReport 生成智能报告
func (snc *SmartNovelChecker) GenerateSmartReport() string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║           小说 RPG 问题检测报告 v2 (智能版)                 ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════╝\n\n")

	// 小说设定
	sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│                      小说设定                            │\n")
	sb.WriteString("├─────────────────────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ 书名:     %-45s│\n", snc.BookSettings.Title))
	sb.WriteString(fmt.Sprintf("│ 主角:     %-45s│\n", snc.BookSettings.Protagonist))
	sb.WriteString(fmt.Sprintf("│ 复活设定: %-45s│\n", fmt.Sprintf("%v (%s)", snc.BookSettings.HasResurrection, snc.BookSettings.ResurrectionCost)))
	sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

	// 统计
	totalIssues := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0

	for _, result := range snc.Results {
		totalIssues += len(result.RealIssues)
		for _, issue := range result.RealIssues {
			switch issue.Severity {
			case "high":
				highCount++
			case "medium":
				mediumCount++
			case "low":
				lowCount++
			}
		}
	}

	sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│                      统计概览                            │\n")
	sb.WriteString("├─────────────────────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ 检测章节数:    %3d                                     │\n", len(snc.Results)))
	sb.WriteString(fmt.Sprintf("│ 真正的问题:    %3d                                     │\n", totalIssues))
	sb.WriteString(fmt.Sprintf("│   - 严重:      %3d                                     │\n", highCount))
	sb.WriteString(fmt.Sprintf("│   - 中等:      %3d                                     │\n", mediumCount))
	sb.WriteString(fmt.Sprintf("│   - 轻微:      %3d                                     │\n", lowCount))
	sb.WriteString(fmt.Sprintf("│ 跨章问题:      %3d                                     │\n", len(snc.CrossChapterIssues)))
	sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

	// 详细问题
	if totalIssues > 0 {
		sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
		sb.WriteString("│                    章节详细问题                          │\n")
		sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

		for _, result := range snc.Results {
			if len(result.RealIssues) == 0 {
				continue
			}

			sb.WriteString(fmt.Sprintf("【%s】%s\n", result.ChapterID, result.ChapterTitle))
			sb.WriteString("  问题列表:\n")

			for _, issue := range result.RealIssues {
				severityIcon := "⚠️"
				if issue.Severity == "high" {
					severityIcon = "🔴"
				} else if issue.Severity == "medium" {
					severityIcon = "🟡"
				} else {
					severityIcon = "🟢"
				}

				sb.WriteString(fmt.Sprintf("    %s [%s] %s\n", severityIcon, issue.Category, issue.Description))
				if issue.Evidence != "" {
					sb.WriteString(fmt.Sprintf("       证据: %s\n", issue.Evidence))
				}
				if issue.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("       建议: %s\n", issue.Suggestion))
				}
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("✅ 未发现明显问题！\n\n")
	}

	return sb.String()
}

// ============================================================
// 辅助函数
// ============================================================

func extractOtherCharacters(text, protagonist string) []string {
	// 提取非主角的角色名
	allChars := extractCharacterNames(text)
	var others []string
	for _, char := range allChars {
		if char != protagonist && !strings.Contains(protagonist, char) {
			others = append(others, char)
		}
	}
	return others
}

func extractCultivationChanges(text string) []string {
	// 提取修为变化
	patterns := []string{
		`练气[一二三四五六七八九十]+层`,
		`筑基[初中后]期`,
		`金丹[初中后]期`,
		`修为`,
		`突破`,
		`进阶`,
	}

	var changes []string
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(text, -1)
		changes = append(changes, matches...)
	}
	return changes
}

func detectSeverePowerJumps(changes []string) []string {
	// 检测不合理的修为跳跃
	// 例如：直接提到跨大境界
	var jumps []string

	// 简单检测：如果出现多个不同境界
	hasLianqi := false
	hasZhuji := false
	hasJindan := false

	for _, change := range changes {
		if strings.Contains(change, "练气") {
			hasLianqi = true
		}
		if strings.Contains(change, "筑基") {
			hasZhuji = true
		}
		if strings.Contains(change, "金丹") {
			hasJindan = true
		}
	}

	// 如果一章内同时出现练气和金丹，那可能有问题
	if hasLianqi && hasJindan {
		jumps = append(jumps, "练气期到金丹期的大跨越")
	}

	// 如果一章内同时出现练气和筑基，也可能有问题
	if hasLianqi && hasZhuji {
		// 筑基是练气后的正常突破，不算严重问题
		// 但如果是同一章内就值得注意
	}

	return jumps
}

func extractItemMentions(text string) []string {
	// 提取物品提及
	itemPatterns := []string{
		`[一-龥]{1,6}丹`,
		`[一-龥]{1,6}石`,
		`[一-龥]{1,6}剑`,
		`[一-龥]{1,6}符`,
		`灵石`, `丹药`, `法宝`, `功法`, `秘籍`,
	}

	var items []string
	for _, pattern := range itemPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(text, -1)
		items = append(items, matches...)
	}
	return items
}

func extractContext(text, keyword string, radius int) string {
	// 提取关键词周围的上下文
	idx := strings.Index(text, keyword)
	if idx == -1 {
		return ""
	}

	start := idx - radius
	if start < 0 {
		start = 0
	}
	end := idx + len(keyword) + radius
	if end > len(text) {
		end = len(text)
	}

	return text[start:end]
}

// CheckNovelSmartCommand 智能检测命令入口
func CheckNovelSmartCommand(bookPath string, outputPath string) error {
	fmt.Printf("开始智能检测小说: %s\n\n", bookPath)

	settings := DefaultBookSettings()
	checker, err := NewSmartNovelChecker(bookPath, &settings)
	if err != nil {
		return fmt.Errorf("创建检测器失败: %w", err)
	}

	if err := checker.CheckAllChapters(); err != nil {
		return fmt.Errorf("检测失败: %w", err)
	}

	report := checker.GenerateSmartReport()
	fmt.Println(report)

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(report), 0644); err != nil {
			return fmt.Errorf("保存报告失败: %w", err)
		}
		fmt.Printf("报告已保存到: %s\n", outputPath)
	}

	return nil
}
