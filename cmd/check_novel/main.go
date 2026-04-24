package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/models"
	"novelgen/internal/rpg/dsl"
)

func main() {
	var (
		bookArg    = flag.String("book", "books/mine", "小说目录路径或书名，例如 books/mine 或 mine")
		output     = flag.String("output", "", "输出文本报告文件路径")
		jsonOutput = flag.String("json", "", "输出 JSON 报告文件路径")
		batchSize  = flag.Int("batch-size", 10, "AI 章节转 DSL 的批大小")
		help       = flag.Bool("h", false, "显示帮助")
	)

	flag.Parse()

	if *help {
		fmt.Println("小说 RPG 问题检测工具 (AI DSL Pipeline)")
		fmt.Println()
		fmt.Println("用法: check_novel [选项]")
		fmt.Println()
		fmt.Println("选项:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("示例:")
		fmt.Println("  check_novel -book=books/mine")
		fmt.Println("  check_novel -book=mine")
		fmt.Println("  check_novel -book=books/mine -output=report.txt")
		fmt.Println("  check_novel -book=books/mine -json=report.json")
		os.Exit(0)
	}

	bookName, bookPath, err := resolveBook(*bookArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	if err := runAIDSLCheck(context.Background(), bookName, bookPath, *output, *jsonOutput, *batchSize); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

type reportSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

type checkNovelAIReport struct {
	Timestamp    string                `json:"timestamp"`
	BookName     string                `json:"book_name"`
	BookPath     string                `json:"book_path"`
	Source       string                `json:"source"`
	ChapterCount int                   `json:"chapter_count"`
	DSLFile      string                `json:"dsl_file"`
	Summary      reportSummary         `json:"summary"`
	Issues       []dsl.SimulationIssue `json:"issues"`
}

func runAIDSLCheck(ctx context.Context, bookName, bookPath, outputPath, jsonOutputPath string, batchSize int) error {
	fmt.Printf("开始 AI DSL Benchmark: %s (%s)\n\n", bookName, bookPath)

	adapter, err := agents.LoadNovelgenProject(bookName)
	if err != nil {
		return fmt.Errorf("加载项目失败: %w", err)
	}

	chapterFiles, err := adapter.FindChapterRecaps()
	if err != nil {
		return fmt.Errorf("查找章节 recap 文件失败: %w", err)
	}
	if len(chapterFiles) == 0 {
		return fmt.Errorf("未找到 recap 文件。请先生成 recaps (story/recaps/*.json)")
	}

	llmConfig, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("加载 LLM 配置失败: %w", err)
	}
	if llmConfig == nil || len(llmConfig.Providers) == 0 {
		return fmt.Errorf("未找到有效 LLM 配置，无法执行 AI -> DSL")
	}

	projectConfig, err := models.LoadProjectConfig(filepath.Join(bookPath, "novel.json"))
	if err != nil {
		return fmt.Errorf("加载 novel.json 失败: %w", err)
	}
	if projectConfig.LLM.Provider == "" || projectConfig.LLM.Model == "" {
		return fmt.Errorf("novel.json 缺少 llm.provider 或 llm.model，无法执行 AI -> DSL")
	}

	projectLLM := &projectConfig.LLM
	client := llmConfig.CreateClient(projectLLM)
	if client == nil {
		return fmt.Errorf("创建 LLM client 失败: provider=%s model=%s", projectLLM.Provider, projectLLM.Model)
	}

	agent := agents.NewChapterToDSLAgent(client, llmConfig, projectLLM, adapter.GetStorySetup())
	agent.SetLanguage("zh")
	agent.SetRequireAI(true)

	rpgDir := filepath.Join(bookPath, "story", "rpg")
	if err := os.MkdirAll(rpgDir, 0755); err != nil {
		return fmt.Errorf("创建 rpg 目录失败: %w", err)
	}

	dslContent, dslData, err := convertChaptersWithBatchCache(
		ctx,
		agent,
		bookName,
		bookPath,
		adapter.GetCharacters(),
		adapter.GetLocations(),
		chapterFiles,
		batchSize,
	)
	if err != nil {
		return err
	}

	applySimulationDefaults(bookName, dslData)

	dslFile := filepath.Join(rpgDir, "01_outline.rpg")
	if err := os.WriteFile(dslFile, []byte(dslContent), 0644); err != nil {
		return fmt.Errorf("保存 DSL 文件失败: %w", err)
	}

	simulator := dsl.NewSimulator(dslData)
	issues := simulator.SimulateAll()
	rules := loadConsistencyRules(bookPath)
	issues = append(issues, buildCrossChapterConsistencyIssues(chapterFiles, rules)...)
	report := buildReport(bookName, bookPath, dslFile, len(chapterFiles), issues)

	printSummary(report)
	printIssues(&dsl.Simulator{Issues: issues})

	if outputPath != "" {
		text := formatTextReport(report)
		if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
			return fmt.Errorf("保存文本报告失败: %w", err)
		}
		fmt.Printf("\n文本报告已保存到: %s\n", outputPath)
	}

	if jsonOutputPath != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON 编码失败: %w", err)
		}
		if err := os.WriteFile(jsonOutputPath, data, 0644); err != nil {
			return fmt.Errorf("保存 JSON 报告失败: %w", err)
		}
		fmt.Printf("JSON 报告已保存到: %s\n", jsonOutputPath)
	}

	return nil
}

func resolveBook(bookArg string) (bookName string, bookPath string, err error) {
	trimmed := strings.TrimSpace(bookArg)
	if trimmed == "" {
		return "", "", fmt.Errorf("-book 不能为空")
	}

	if st, statErr := os.Stat(trimmed); statErr == nil && st.IsDir() {
		novelPath := filepath.Join(trimmed, "novel.json")
		if _, err := os.Stat(novelPath); err == nil {
			absPath, _ := filepath.Abs(trimmed)
			return filepath.Base(absPath), absPath, nil
		}
		return "", "", fmt.Errorf("目录存在但缺少 novel.json: %s", trimmed)
	}

	candidate := filepath.Join("books", trimmed)
	if st, statErr := os.Stat(candidate); statErr == nil && st.IsDir() {
		novelPath := filepath.Join(candidate, "novel.json")
		if _, err := os.Stat(novelPath); err == nil {
			absPath, _ := filepath.Abs(candidate)
			return trimmed, absPath, nil
		}
	}

	return "", "", fmt.Errorf("找不到有效书目录: %s", trimmed)
}

func buildReport(bookName, bookPath, dslFile string, chapterCount int, issues []dsl.SimulationIssue) checkNovelAIReport {
	s := reportSummary{Total: len(issues)}
	for _, issue := range issues {
		switch issue.Severity {
		case dsl.SeverityCritical:
			s.Critical++
		case dsl.SeverityWarning:
			s.Warning++
		case dsl.SeverityInfo:
			s.Info++
		}
	}

	return checkNovelAIReport{
		Timestamp:    time.Now().Format(time.RFC3339),
		BookName:     bookName,
		BookPath:     bookPath,
		Source:       "chapters(recap) -> AI(batch/cache) -> DSL -> simulate",
		ChapterCount: chapterCount,
		DSLFile:      dslFile,
		Summary:      s,
		Issues:       issues,
	}
}

func printSummary(report checkNovelAIReport) {
	fmt.Println("========================================")
	fmt.Println("AI DSL Benchmark Summary")
	fmt.Println("========================================")
	fmt.Printf("Book: %s\n", report.BookName)
	fmt.Printf("Chapters: %d\n", report.ChapterCount)
	fmt.Printf("DSL: %s\n", report.DSLFile)
	fmt.Printf("Issues: total=%d critical=%d warning=%d info=%d\n",
		report.Summary.Total, report.Summary.Critical, report.Summary.Warning, report.Summary.Info)
}

func printIssues(simulator *dsl.Simulator) {
	if simulator == nil || len(simulator.Issues) == 0 {
		fmt.Println("\n未发现明显问题")
		return
	}

	fmt.Println("\nDetailed Issues")
	fmt.Println(strings.Repeat("-", 60))

	for _, issue := range simulator.GetIssuesBySeverity(dsl.SeverityCritical) {
		fmt.Println(dsl.FormatIssue(issue))
		fmt.Println()
	}
	for _, issue := range simulator.GetIssuesBySeverity(dsl.SeverityWarning) {
		fmt.Println(dsl.FormatIssue(issue))
		fmt.Println()
	}
	for _, issue := range simulator.GetIssuesBySeverity(dsl.SeverityInfo) {
		fmt.Println(dsl.FormatIssue(issue))
		fmt.Println()
	}
}

func formatTextReport(report checkNovelAIReport) string {
	var b strings.Builder
	b.WriteString("AI DSL Benchmark Report\n")
	b.WriteString("========================================\n")
	b.WriteString(fmt.Sprintf("Timestamp: %s\n", report.Timestamp))
	b.WriteString(fmt.Sprintf("Book: %s\n", report.BookName))
	b.WriteString(fmt.Sprintf("BookPath: %s\n", report.BookPath))
	b.WriteString(fmt.Sprintf("Source: %s\n", report.Source))
	b.WriteString(fmt.Sprintf("ChapterCount: %d\n", report.ChapterCount))
	b.WriteString(fmt.Sprintf("DSL: %s\n", report.DSLFile))
	b.WriteString(fmt.Sprintf("Issues: total=%d critical=%d warning=%d info=%d\n\n",
		report.Summary.Total, report.Summary.Critical, report.Summary.Warning, report.Summary.Info))

	for i, issue := range report.Issues {
		b.WriteString(fmt.Sprintf("%d. [%s/%s] %s\n",
			i+1, strings.ToUpper(string(issue.Severity)), strings.ToUpper(string(issue.Type)), issue.Description))
		if issue.Chapter != "" {
			if issue.Step > 0 {
				b.WriteString(fmt.Sprintf("   Location: %s step %d\n", issue.Chapter, issue.Step))
			} else {
				b.WriteString(fmt.Sprintf("   Location: %s\n", issue.Chapter))
			}
		}
		if issue.Suggestion != "" {
			b.WriteString(fmt.Sprintf("   Suggestion: %s\n", issue.Suggestion))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func applySimulationDefaults(bookName string, dslData *dsl.DSL) {
	if dslData == nil {
		return
	}
	if dslData.Metadata == nil {
		dslData.Metadata = &dsl.Metadata{}
	}
	if strings.TrimSpace(dslData.Metadata.Title) == "" {
		dslData.Metadata.Title = bookName
	}
	if strings.TrimSpace(dslData.Metadata.DSLVersion) == "" {
		dslData.Metadata.DSLVersion = "0.1.0"
	}
	if dslData.Characters == nil {
		dslData.Characters = &dsl.Characters{}
	}
	if dslData.Characters.Player == nil {
		playerName := "主角"
		if len(dslData.Characters.NPCs) > 0 && strings.TrimSpace(dslData.Characters.NPCs[0].Name) != "" {
			playerName = dslData.Characters.NPCs[0].Name
		}
		dslData.Characters.Player = &dsl.Player{
			ID:   "char_player",
			Name: playerName,
		}
	}
}
