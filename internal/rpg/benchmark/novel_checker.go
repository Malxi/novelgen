package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"novelgen/internal/rpg"
)

// ============================================================
// 小说章节检测工具
// ============================================================

// NovelChecker 小说检测器
type NovelChecker struct {
	BookPath           string
	RPGData            *rpg.NovelRPGData
	Results            []ChapterCheckResult
	CrossChapterIssues []CrossChapterIssue
}

// ChapterCheckResult 章节检测结果
type ChapterCheckResult struct {
	ChapterID     string
	ChapterTitle  string
	FilePath      string
	TextLength    int
	Issues        []DetectedIssue
	Violations    []rpg.ConstraintViolation
	CheckDuration time.Duration
	Timestamp     time.Time
}

// NewNovelChecker 创建小说检测器
func NewNovelChecker(bookPath string) (*NovelChecker, error) {
	checker := &NovelChecker{
		BookPath: bookPath,
		Results:  make([]ChapterCheckResult, 0),
	}

	// 加载 RPG 数据
	rpgDataPath := filepath.Join(bookPath, "rpg_data.json")
	if _, err := os.Stat(rpgDataPath); err == nil {
		data, err := os.ReadFile(rpgDataPath)
		if err != nil {
			return nil, fmt.Errorf("读取 RPG 数据失败: %w", err)
		}
		// 解析 RPG 数据
		var rpgData rpg.NovelRPGData
		if err := json.Unmarshal(data, &rpgData); err != nil {
			// 尝试解析为通用格式
			checker.RPGData = extractRPGDataFromJSON(data)
		} else {
			checker.RPGData = &rpgData
		}
	}

	return checker, nil
}

// CheckAllChapters 检测所有章节
func (nc *NovelChecker) CheckAllChapters() error {
	chaptersDir := filepath.Join(nc.BookPath, "chapters")

	// 读取所有章节文件
	files, err := os.ReadDir(chaptersDir)
	if err != nil {
		return fmt.Errorf("读取章节目录失败: %w", err)
	}

	// 按章节自然顺序排序（例如 C2 在 C10 前）
	var chapterFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			chapterFiles = append(chapterFiles, f.Name())
		}
	}
	sortChapterFilenames(chapterFiles)

	fmt.Printf("发现 %d 个章节文件\n", len(chapterFiles))

	// 逐个检测章节
	for i, filename := range chapterFiles {
		chapterPath := filepath.Join(chaptersDir, filename)
		fmt.Printf("[%d/%d] 检测 %s...\n", i+1, len(chapterFiles), filename)

		result, err := nc.CheckChapter(chapterPath)
		if err != nil {
			fmt.Printf("  警告: %v\n", err)
			continue
		}

		nc.Results = append(nc.Results, *result)

		// 打印发现的问题
		if len(result.Issues) > 0 {
			fmt.Printf("  发现 %d 个问题:\n", len(result.Issues))
			for _, issue := range result.Issues {
				fmt.Printf("    - [%s] %s: %s\n", issue.Severity, issue.Category, issue.Description)
			}
		}
	}

	// 跨章一致性检测
	fmt.Println("\n执行跨章一致性检测...")
	nc.runCrossChapterCheck()

	return nil
}

// CheckChapter 检测单个章节
func (nc *NovelChecker) CheckChapter(chapterPath string) (*ChapterCheckResult, error) {
	start := time.Now()

	// 读取章节内容
	content, err := os.ReadFile(chapterPath)
	if err != nil {
		return nil, err
	}

	text := string(content)
	filename := filepath.Base(chapterPath)

	// 提取章节标题
	title := extractChapterTitle(text)

	result := &ChapterCheckResult{
		ChapterID:    extractChapterID(filename),
		ChapterTitle: title,
		FilePath:     chapterPath,
		TextLength:   len(text),
		Issues:       make([]DetectedIssue, 0),
		Violations:   make([]rpg.ConstraintViolation, 0),
		Timestamp:    time.Now(),
	}

	// 1. 使用约束系统检测
	if nc.RPGData != nil {
		world := nc.buildGameWorld()
		cs := rpg.NewConstraintSystem(world)

		violations := cs.ValidateChapter(result.ChapterID, text)
		result.Violations = violations

		// 转换为 DetectedIssue
		for _, v := range violations {
			result.Issues = append(result.Issues, DetectedIssue{
				Category:    v.Type,
				Target:      v.Target,
				Description: v.Issue,
				Severity:    v.Severity,
			})
		}
	}

	// 2. 使用文本提取器检测
	extractor := rpg.NewTextExtractor()
	entities := extractor.ExtractFromText(text)

	// 检测潜在问题
	issues := nc.detectIssuesFromEntities(entities, text)
	result.Issues = append(result.Issues, issues...)

	result.CheckDuration = time.Since(start)

	return result, nil
}

// runCrossChapterCheck 运行跨章一致性检测
func (nc *NovelChecker) runCrossChapterCheck() {
	checker := NewCrossChapterChecker()

	// 构建章节状态
	for _, result := range nc.Results {
		content, _ := os.ReadFile(result.FilePath)
		state := ExtractChapterStateFromText(result.ChapterID, 0, string(content))
		checker.AddChapter(state)
	}

	// 执行检查
	nc.CrossChapterIssues = checker.CheckConsistency()

	if len(nc.CrossChapterIssues) > 0 {
		fmt.Printf("发现 %d 个跨章一致性问题:\n", len(nc.CrossChapterIssues))
		for _, issue := range nc.CrossChapterIssues {
			fmt.Printf("  - [%s/%s] %s: %s\n",
				issue.Category, issue.Severity, issue.Target, issue.Description)
		}
	} else {
		fmt.Println("跨章一致性良好，未发现明显问题")
	}
}

// buildGameWorld 从 RPG 数据构建游戏世界
func (nc *NovelChecker) buildGameWorld() *rpg.GameWorld {
	world := rpg.NewGameWorld()

	// 如果加载了 RPG 数据，填充世界
	if nc.RPGData != nil {
		// 添加角色
		for _, charTemplate := range nc.RPGData.Characters {
			world.Characters.AddTemplate(charTemplate)
		}

		// 添加物品
		for _, item := range nc.RPGData.Items {
			world.Items.AddItem(item)
		}

		// 添加技能
		for _, skill := range nc.RPGData.Skills {
			world.Skills.AddSkill(skill)
		}

		// 添加地图位置
		for _, m := range nc.RPGData.Locations {
			world.Maps.AddMap(m)
		}
	}

	return world
}

// detectIssuesFromEntities 从提取的实体中检测问题
func (nc *NovelChecker) detectIssuesFromEntities(entities []rpg.ExtractedEntity, text string) []DetectedIssue {
	issues := make([]DetectedIssue, 0)

	// 统计各类实体
	characterCount := 0
	cultivationCount := 0
	deathCount := 0
	resurrectionCount := 0

	for _, entity := range entities {
		switch entity.Type {
		case "character":
			characterCount++
		case "cultivation":
			cultivationCount++
		case "event":
			if entity.Name == "死亡" || entity.Name == "被杀" {
				deathCount++
			}
			if entity.Name == "复活" {
				resurrectionCount++
			}
		}
	}

	// 检测战力崩坏（过多修为变化）
	if cultivationCount > 3 {
		issues = append(issues, DetectedIssue{
			Category:    "power",
			SubCategory: "too_frequent",
			Description: fmt.Sprintf("章节内检测到 %d 次修为/境界变化", cultivationCount),
			Severity:    "warning",
		})
	}

	// 检测复活滥用
	if resurrectionCount > 0 {
		issues = append(issues, DetectedIssue{
			Category:    "resurrection",
			SubCategory: "detected",
			Description: fmt.Sprintf("检测到 %d 次复活事件", resurrectionCount),
			Severity:    "info",
		})
	}

	// 检测死亡频率
	if deathCount > 2 {
		issues = append(issues, DetectedIssue{
			Category:    "plot",
			SubCategory: "high_death_rate",
			Description: fmt.Sprintf("章节内死亡事件较多: %d 次", deathCount),
			Severity:    "warning",
		})
	}

	// 检测时间跳跃
	timeJumps := detectTimeJumps(text)
	if len(timeJumps) > 3 {
		issues = append(issues, DetectedIssue{
			Category:    "timeline",
			SubCategory: "excessive_jumps",
			Description: fmt.Sprintf("章节内时间跳跃 %d 次", len(timeJumps)),
			Severity:    "warning",
		})
	}

	return issues
}

// GenerateReport 生成检测报告
func (nc *NovelChecker) GenerateReport() string {
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║              小说章节 RPG 问题检测报告                      ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════╝\n\n")

	// 统计信息
	totalIssues := 0
	criticalCount := 0
	errorCount := 0
	warningCount := 0

	for _, result := range nc.Results {
		totalIssues += len(result.Issues)
		for _, issue := range result.Issues {
			switch issue.Severity {
			case "critical":
				criticalCount++
			case "error":
				errorCount++
			case "warning":
				warningCount++
			}
		}
	}

	sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│                      统计概览                            │\n")
	sb.WriteString("├─────────────────────────────────────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ 检测章节数:     %3d                                    │\n", len(nc.Results)))
	sb.WriteString(fmt.Sprintf("│ 发现问题总数:   %3d                                    │\n", totalIssues))
	sb.WriteString(fmt.Sprintf("│   - Critical:   %3d                                    │\n", criticalCount))
	sb.WriteString(fmt.Sprintf("│   - Error:      %3d                                    │\n", errorCount))
	sb.WriteString(fmt.Sprintf("│   - Warning:    %3d                                    │\n", warningCount))
	sb.WriteString(fmt.Sprintf("│ 跨章问题数:     %3d                                    │\n", len(nc.CrossChapterIssues)))
	sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

	// 章节详细报告
	if totalIssues > 0 {
		sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
		sb.WriteString("│                    章节详细问题                          │\n")
		sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

		for _, result := range nc.Results {
			if len(result.Issues) == 0 {
				continue
			}

			sb.WriteString(fmt.Sprintf("【%s】%s\n", result.ChapterID, result.ChapterTitle))
			sb.WriteString(fmt.Sprintf("  文件: %s\n", result.FilePath))
			sb.WriteString(fmt.Sprintf("  字数: %d, 检测耗时: %s\n", result.TextLength, result.CheckDuration))
			sb.WriteString("  问题列表:\n")

			for _, issue := range result.Issues {
				sb.WriteString(fmt.Sprintf("    - [%s/%s] ", issue.Severity, issue.Category))
				if issue.Target != "" {
					sb.WriteString(fmt.Sprintf("(%s) ", issue.Target))
				}
				sb.WriteString(issue.Description + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// 跨章问题
	if len(nc.CrossChapterIssues) > 0 {
		sb.WriteString("┌─────────────────────────────────────────────────────────┐\n")
		sb.WriteString("│                    跨章一致性问题                        │\n")
		sb.WriteString("└─────────────────────────────────────────────────────────┘\n\n")

		for _, issue := range nc.CrossChapterIssues {
			sb.WriteString(fmt.Sprintf("【%s → %s】\n", issue.ChapterFrom, issue.ChapterTo))
			sb.WriteString(fmt.Sprintf("  类型: %s/%s\n", issue.Category, issue.Type))
			sb.WriteString(fmt.Sprintf("  目标: %s\n", issue.Target))
			sb.WriteString(fmt.Sprintf("  问题: %s\n", issue.Description))
			if issue.Expected != "" {
				sb.WriteString(fmt.Sprintf("  期望: %s, 实际: %s\n", issue.Expected, issue.Actual))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ExportJSON 导出 JSON 格式报告
func (nc *NovelChecker) ExportJSON(filename string) error {
	report := map[string]interface{}{
		"timestamp":            time.Now().Format(time.RFC3339),
		"book_path":            nc.BookPath,
		"total_chapters":       len(nc.Results),
		"results":              nc.Results,
		"cross_chapter_issues": nc.CrossChapterIssues,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// ============================================================
// 辅助函数
// ============================================================

func extractChapterTitle(text string) string {
	// 提取 Markdown 标题
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
		if strings.HasPrefix(line, "## ") {
			return strings.TrimPrefix(line, "## ")
		}
	}
	return "未知标题"
}

func extractChapterID(filename string) string {
	// 从文件名提取章节 ID，如 chapter-P1-V1-C1.md -> P1-V1-C1
	re := regexp.MustCompile(`chapter-(.+)\.md`)
	if matches := re.FindStringSubmatch(filename); len(matches) > 1 {
		return matches[1]
	}
	return filename
}

func detectTimeJumps(text string) []string {
	patterns := []string{
		`三天后`, `一周后`, `一个月后`, `三个月后`, `半年后`, `一年后`,
		`三年后`, `十年后`, `数年后`, `春去秋来`, `斗转星移`,
	}

	var jumps []string
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			jumps = append(jumps, pattern)
		}
	}
	return jumps
}

func extractRPGDataFromJSON(data []byte) *rpg.NovelRPGData {
	// 尝试从通用 JSON 格式提取 RPG 数据
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return &rpg.NovelRPGData{}
	}

	// 这里可以根据实际的 JSON 结构进行解析
	return &rpg.NovelRPGData{
		Characters: make([]*rpg.CharacterTemplate, 0),
		Items:      make([]*rpg.Item, 0),
		Skills:     make([]*rpg.Skill, 0),
		Locations:  make([]*rpg.Map, 0),
	}
}

// ============================================================
// 命令行入口
// ============================================================

// CheckNovelCommand 检测小说命令
func CheckNovelCommand(bookPath string, outputPath string, jsonOutput string) error {
	fmt.Printf("开始检测小说: %s\n\n", bookPath)

	checker, err := NewNovelChecker(bookPath)
	if err != nil {
		return fmt.Errorf("创建检测器失败: %w", err)
	}

	// 执行检测
	if err := checker.CheckAllChapters(); err != nil {
		return fmt.Errorf("检测失败: %w", err)
	}

	// 生成报告
	report := checker.GenerateReport()
	fmt.Println(report)

	// 保存文本报告
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(report), 0644); err != nil {
			return fmt.Errorf("保存报告失败: %w", err)
		}
		fmt.Printf("报告已保存到: %s\n", outputPath)
	}

	// 保存 JSON 报告
	if jsonOutput != "" {
		if err := checker.ExportJSON(jsonOutput); err != nil {
			return fmt.Errorf("导出 JSON 失败: %w", err)
		}
		fmt.Printf("JSON 报告已保存到: %s\n", jsonOutput)
	}

	return nil
}
