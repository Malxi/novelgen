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
		bookArg           = flag.String("book", "books/mine", "灏忚鐩綍璺緞鎴栦功鍚嶏紝渚嬪 books/mine 鎴?mine")
		output            = flag.String("output", "", "杈撳嚭鏂囨湰鎶ュ憡鏂囦欢璺緞")
		jsonOutput        = flag.String("json", "", "杈撳嚭 JSON 鎶ュ憡鏂囦欢璺緞")
		batchSize         = flag.Int("batch-size", 10, "AI 绔犺妭杞?DSL 鐨勬壒澶у皬")
		minSeverity       = flag.String("min-severity", "warning", "鎶ュ憡鏈€灏忎弗閲嶇骇鍒? critical, warning, info")
		includeDSLHygiene = flag.Bool("include-dsl-hygiene", false, "include DSL extraction hygiene issues in report")
		goldenPath        = flag.String("golden", "", "golden benchmark 鏂囦欢璺緞锛涗负绌烘椂鑷姩浣跨敤 story/rpg/expected_issues.json")
		help              = flag.Bool("h", false, "鏄剧ず甯姪")
	)

	flag.Parse()

	if *help {
		fmt.Println("灏忚 RPG 闂妫€娴嬪伐鍏?(AI DSL Pipeline)")
		fmt.Println()
		fmt.Println("鐢ㄦ硶: check_novel [閫夐」]")
		fmt.Println()
		fmt.Println("閫夐」:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("绀轰緥:")
		fmt.Println("  check_novel -book=books/mine")
		fmt.Println("  check_novel -book=mine")
		fmt.Println("  check_novel -book=books/mine -output=report.txt")
		fmt.Println("  check_novel -book=books/mine -json=report.json")
		os.Exit(0)
	}

	bookName, bookPath, err := resolveBook(*bookArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "閿欒: %v\n", err)
		os.Exit(1)
	}

	threshold, err := parseMinSeverity(*minSeverity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "閿欒: %v\n", err)
		os.Exit(1)
	}

	if err := runAIDSLCheck(context.Background(), bookName, bookPath, *output, *jsonOutput, *batchSize, threshold, *includeDSLHygiene, *goldenPath); err != nil {
		fmt.Fprintf(os.Stderr, "閿欒: %v\n", err)
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
	Timestamp         string                `json:"timestamp"`
	BookName          string                `json:"book_name"`
	BookPath          string                `json:"book_path"`
	Source            string                `json:"source"`
	ChapterCount      int                   `json:"chapter_count"`
	DSLFile           string                `json:"dsl_file"`
	MinSeverity       string                `json:"min_severity"`
	IncludeDSLHygiene bool                  `json:"include_dsl_hygiene"`
	Summary           reportSummary         `json:"summary"`
	AllSummary        reportSummary         `json:"all_summary"`
	FilteredOut       int                   `json:"filtered_out"`
	Golden            *goldenEvaluation     `json:"golden,omitempty"`
	Issues            []dsl.SimulationIssue `json:"issues"`
}

func runAIDSLCheck(ctx context.Context, bookName, bookPath, outputPath, jsonOutputPath string, batchSize int, minSeverity dsl.SeverityLevel, includeDSLHygiene bool, goldenPath string) error {
	fmt.Printf("寮€濮?AI DSL Benchmark: %s (%s)\n\n", bookName, bookPath)

	adapter, err := agents.LoadNovelgenProject(bookName)
	if err != nil {
		return fmt.Errorf("鍔犺浇椤圭洰澶辫触: %w", err)
	}

	chapterFiles, err := findCheckNovelChapterInputs(bookPath, adapter.GetOutline())
	if err != nil {
		return fmt.Errorf("查找章节正文失败: %w", err)
	}
	if len(chapterFiles) == 0 {
		return fmt.Errorf("未找到章节正文。请先生成 chapters/*.md")
	}

	llmConfig, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("鍔犺浇 LLM 閰嶇疆澶辫触: %w", err)
	}
	if llmConfig == nil || len(llmConfig.Providers) == 0 {
		return fmt.Errorf("鏈壘鍒版湁鏁?LLM 閰嶇疆锛屾棤娉曟墽琛?AI -> DSL")
	}

	projectConfig, err := models.LoadProjectConfig(filepath.Join(bookPath, "novel.json"))
	if err != nil {
		return fmt.Errorf("鍔犺浇 novel.json 澶辫触: %w", err)
	}
	if projectConfig.LLM.Provider == "" || projectConfig.LLM.Model == "" {
		return fmt.Errorf("novel.json 缂哄皯 llm.provider 鎴?llm.model锛屾棤娉曟墽琛?AI -> DSL")
	}

	projectLLM := &projectConfig.LLM
	client := llmConfig.CreateClient(projectLLM)
	if client == nil {
		return fmt.Errorf("鍒涘缓 LLM client 澶辫触: provider=%s model=%s", projectLLM.Provider, projectLLM.Model)
	}

	agent := agents.NewChapterToDSLAgent(client, llmConfig, projectLLM, adapter.GetStorySetup())
	agent.SetLanguage("zh")
	agent.SetRequireAI(true)

	rpgDir := filepath.Join(bookPath, "story", "rpg")
	if err := os.MkdirAll(rpgDir, 0755); err != nil {
		return fmt.Errorf("鍒涘缓 rpg 鐩綍澶辫触: %w", err)
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

	dslFile := filepath.Join(rpgDir, "04_chapters.rpg")
	if err := os.WriteFile(dslFile, []byte(dslContent), 0644); err != nil {
		return fmt.Errorf("淇濆瓨 DSL 鏂囦欢澶辫触: %w", err)
	}

	simulator := dsl.NewSimulator(dslData)
	issues := simulator.SimulateAll()
	report := buildReport(bookName, bookPath, dslFile, len(chapterFiles), issues, minSeverity, includeDSLHygiene)
	golden, err := maybeEvaluateGolden(bookPath, goldenPath, report.Issues)
	if err != nil {
		return err
	}
	report.Golden = golden

	printSummary(report)
	printIssues(&dsl.Simulator{Issues: report.Issues})

	if outputPath != "" {
		text := formatTextReport(report)
		if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
			return fmt.Errorf("淇濆瓨鏂囨湰鎶ュ憡澶辫触: %w", err)
		}
		fmt.Printf("\n鏂囨湰鎶ュ憡宸蹭繚瀛樺埌: %s\n", outputPath)
	}

	if jsonOutputPath != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON 缂栫爜澶辫触: %w", err)
		}
		if err := os.WriteFile(jsonOutputPath, data, 0644); err != nil {
			return fmt.Errorf("淇濆瓨 JSON 鎶ュ憡澶辫触: %w", err)
		}
		fmt.Printf("JSON 鎶ュ憡宸蹭繚瀛樺埌: %s\n", jsonOutputPath)
	}

	return nil
}

func maybeEvaluateGolden(bookPath, requestedPath string, issues []dsl.SimulationIssue) (*goldenEvaluation, error) {
	path := strings.TrimSpace(requestedPath)
	explicit := path != ""
	if path == "" {
		path = filepath.Join(bookPath, "story", "rpg", "expected_issues.json")
	}
	if _, err := os.Stat(path); err != nil {
		if explicit {
			return nil, fmt.Errorf("鍔犺浇 golden benchmark 澶辫触: %w", err)
		}
		return nil, nil
	}
	spec, err := loadGoldenSpec(path)
	if err != nil {
		return nil, fmt.Errorf("鍔犺浇 golden benchmark 澶辫触: %w", err)
	}
	eval := evaluateGolden(path, spec, issues)
	return &eval, nil
}

func resolveBook(bookArg string) (bookName string, bookPath string, err error) {
	trimmed := strings.TrimSpace(bookArg)
	if trimmed == "" {
		return "", "", fmt.Errorf("-book 涓嶈兘涓虹┖")
	}

	if st, statErr := os.Stat(trimmed); statErr == nil && st.IsDir() {
		novelPath := filepath.Join(trimmed, "novel.json")
		if _, err := os.Stat(novelPath); err == nil {
			absPath, _ := filepath.Abs(trimmed)
			return filepath.Base(absPath), absPath, nil
		}
		return "", "", fmt.Errorf("鐩綍瀛樺湪浣嗙己灏?novel.json: %s", trimmed)
	}

	candidate := filepath.Join("books", trimmed)
	if st, statErr := os.Stat(candidate); statErr == nil && st.IsDir() {
		novelPath := filepath.Join(candidate, "novel.json")
		if _, err := os.Stat(novelPath); err == nil {
			absPath, _ := filepath.Abs(candidate)
			return trimmed, absPath, nil
		}
	}

	return "", "", fmt.Errorf("鎵句笉鍒版湁鏁堜功鐩綍: %s", trimmed)
}

func buildReport(bookName, bookPath, dslFile string, chapterCount int, issues []dsl.SimulationIssue, minSeverity dsl.SeverityLevel, includeDSLHygiene bool) checkNovelAIReport {
	allSummary := summarizeIssues(issues)
	reportedIssues := filterIssuesByMinSeverity(issues, minSeverity)
	if !includeDSLHygiene {
		reportedIssues = filterDSLHygieneIssues(reportedIssues)
	}
	reportedSummary := summarizeIssues(reportedIssues)

	return checkNovelAIReport{
		Timestamp:         time.Now().Format(time.RFC3339),
		BookName:          bookName,
		BookPath:          bookPath,
		Source:            "chapters(md)+recaps(optional) -> AI(batch/cache) -> DSL -> simulate",
		ChapterCount:      chapterCount,
		DSLFile:           dslFile,
		MinSeverity:       string(minSeverity),
		IncludeDSLHygiene: includeDSLHygiene,
		Summary:           reportedSummary,
		AllSummary:        allSummary,
		FilteredOut:       allSummary.Total - reportedSummary.Total,
		Issues:            reportedIssues,
	}
}

func summarizeIssues(issues []dsl.SimulationIssue) reportSummary {
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
	return s
}

func filterIssuesByMinSeverity(issues []dsl.SimulationIssue, minSeverity dsl.SeverityLevel) []dsl.SimulationIssue {
	filtered := make([]dsl.SimulationIssue, 0, len(issues))
	minRank := severityRank(minSeverity)
	for _, issue := range issues {
		if severityRank(issue.Severity) >= minRank {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func filterDSLHygieneIssues(issues []dsl.SimulationIssue) []dsl.SimulationIssue {
	filtered := make([]dsl.SimulationIssue, 0, len(issues))
	for _, issue := range issues {
		if isDSLHygieneIssue(issue) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func isDSLHygieneIssue(issue dsl.SimulationIssue) bool {
	desc := strings.ToLower(issue.Description)
	if strings.Contains(desc, "missing protagonist background") ||
		strings.Contains(desc, "missing protagonist motivation") ||
		strings.Contains(desc, "battle event missing config") ||
		strings.Contains(desc, "battle event missing enemies") {
		return true
	}
	return false
}

func parseMinSeverity(raw string) (dsl.SeverityLevel, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "warning", "warn":
		return dsl.SeverityWarning, nil
	case "critical", "crit":
		return dsl.SeverityCritical, nil
	case "info", "all":
		return dsl.SeverityInfo, nil
	default:
		return "", fmt.Errorf("invalid min-severity: %s (allowed: critical, warning, info)", raw)
	}
}
func severityRank(severity dsl.SeverityLevel) int {
	switch severity {
	case dsl.SeverityCritical:
		return 3
	case dsl.SeverityWarning:
		return 2
	case dsl.SeverityInfo:
		return 1
	default:
		return 0
	}
}

func printSummary(report checkNovelAIReport) {
	fmt.Println("========================================")
	fmt.Println("AI DSL Benchmark Summary")
	fmt.Println("========================================")
	fmt.Printf("Book: %s\n", report.BookName)
	fmt.Printf("Chapters: %d\n", report.ChapterCount)
	fmt.Printf("DSL: %s\n", report.DSLFile)
	fmt.Printf("Reported issues (min=%s, include_dsl_hygiene=%t): total=%d critical=%d warning=%d info=%d\n",
		report.MinSeverity, report.IncludeDSLHygiene,
		report.Summary.Total, report.Summary.Critical, report.Summary.Warning, report.Summary.Info)
	if report.FilteredOut > 0 {
		fmt.Printf("All issues: total=%d critical=%d warning=%d info=%d (filtered_out=%d)\n",
			report.AllSummary.Total, report.AllSummary.Critical, report.AllSummary.Warning, report.AllSummary.Info, report.FilteredOut)
	}
	if summary := formatGoldenSummary(report.Golden); summary != "" {
		fmt.Println(summary)
		if report.Golden.Missed > 0 {
			fmt.Printf("Golden missed: %s\n", strings.Join(report.Golden.MissingIssues, ", "))
		}
	}
}

func printIssues(simulator *dsl.Simulator) {
	if simulator == nil || len(simulator.Issues) == 0 {
		fmt.Println("\nNo obvious issues found")
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
	b.WriteString(fmt.Sprintf("MinSeverity: %s\n", report.MinSeverity))
	b.WriteString(fmt.Sprintf("IncludeDSLHygiene: %t\n", report.IncludeDSLHygiene))
	b.WriteString(fmt.Sprintf("ReportedIssues: total=%d critical=%d warning=%d info=%d\n",
		report.Summary.Total, report.Summary.Critical, report.Summary.Warning, report.Summary.Info))
	b.WriteString(fmt.Sprintf("AllIssues: total=%d critical=%d warning=%d info=%d filtered_out=%d\n\n",
		report.AllSummary.Total, report.AllSummary.Critical, report.AllSummary.Warning, report.AllSummary.Info, report.FilteredOut))
	if summary := formatGoldenSummary(report.Golden); summary != "" {
		b.WriteString(summary)
		b.WriteString("\n")
		if report.Golden.Missed > 0 {
			b.WriteString(fmt.Sprintf("GoldenMissed: %s\n", strings.Join(report.Golden.MissingIssues, ", ")))
		}
		b.WriteString("\n")
	}

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
		for _, ev := range issue.Evidence {
			location := ev.Chapter
			if ev.Step > 0 {
				location = fmt.Sprintf("%s step %d", ev.Chapter, ev.Step)
			}
			b.WriteString(fmt.Sprintf("   Evidence: %s / %s: %s\n", location, ev.Source, ev.Text))
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
		playerName := "涓昏"
		if len(dslData.Characters.NPCs) > 0 && strings.TrimSpace(dslData.Characters.NPCs[0].Name) != "" {
			playerName = dslData.Characters.NPCs[0].Name
		}
		dslData.Characters.Player = &dsl.Player{
			ID:   "char_player",
			Name: playerName,
		}
	}
}
