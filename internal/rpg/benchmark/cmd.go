package benchmark

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// ============================================================
// 命令行接口
// ============================================================

// RunCommand 运行 benchmark 命令
func RunCommand(args []string) error {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)

	// 定义命令行参数
	var (
		mode           = fs.String("mode", "standard", "运行模式: standard, fuzz, stress, all")
		difficulty     = fs.String("difficulty", "", "按难度过滤: easy, medium, hard, expert")
		tag            = fs.String("tag", "", "按标签过滤")
		category       = fs.String("category", "", "按类别过滤: power, resurrection, timeline, character, pacing, plot")
		output         = fs.String("output", "", "输出报告到文件")
		jsonOutput     = fs.String("json", "", "输出 JSON 格式报告到文件")
		perfTest       = fs.Bool("perf", false, "运行性能测试")
		stressTest     = fs.Bool("stress", false, "运行压力测试")
		scalability    = fs.Bool("scalability", false, "运行可扩展性测试")
		list           = fs.Bool("list", false, "列出所有测试用例")
		verbose        = fs.Bool("v", false, "详细输出")
		seed           = fs.Int64("seed", time.Now().UnixNano(), "模糊测试随机种子")
		fuzzCount      = fs.Int("fuzz-count", 50, "模糊测试用例数量")
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	// 列出测试用例
	if *list {
		return listTestCases(*difficulty, *tag, *category)
	}

	// 运行性能测试
	if *perfTest {
		return runPerformanceTest(output)
	}

	// 运行压力测试
	if *stressTest {
		return runStressTest(output)
	}

	// 运行可扩展性测试
	if *scalability {
		return runScalabilityTest(output)
	}

	// 根据模式运行测试
	switch *mode {
	case "standard":
		return runStandardBenchmark(*difficulty, *tag, *category, *output, *jsonOutput, *verbose)
	case "fuzz":
		return runFuzzBenchmark(*seed, *fuzzCount, *output, *jsonOutput)
	case "stress":
		return runStressTest(output)
	case "all":
		return runAllBenchmarks(*output, *jsonOutput)
	default:
		return fmt.Errorf("未知模式: %s", *mode)
	}
}

// ============================================================
// 各种运行模式
// ============================================================

func runStandardBenchmark(difficulty, tag, category, output, jsonOutput string, verbose bool) error {
	fmt.Println("正在运行标准 Benchmark...")

	// 获取测试用例
	testCases := getFilteredTestCases(difficulty, tag, category)
	if len(testCases) == 0 {
		return fmt.Errorf("没有找到匹配的测试用例")
	}

	fmt.Printf("加载了 %d 个测试用例\n\n", len(testCases))

	// 运行测试
	runner := NewBenchmarkRunner()
	runner.TestCases = testCases
	runner.Run()

	// 计算指标（包含难度信息）
	metrics := CalculateMetricsWithDifficulty(runner.Results, testCases)

	// 生成报告
	report := runner.GenerateReport()
	metricsReport := FormatMetrics(metrics)

	// 输出到控制台
	fmt.Println(metricsReport)
	if verbose {
		fmt.Println(report)
	}

	// 输出到文件
	if output != "" {
		fullReport := metricsReport + "\n" + report
		if err := os.WriteFile(output, []byte(fullReport), 0644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
		fmt.Printf("报告已保存到: %s\n", output)
	}

	// 输出 JSON
	if jsonOutput != "" {
		if err := exportJSON(metrics, runner.Results, jsonOutput); err != nil {
			return fmt.Errorf("导出 JSON 失败: %w", err)
		}
		fmt.Printf("JSON 报告已保存到: %s\n", jsonOutput)
	}

	// 返回结果状态
	if metrics.OverallScore < 60 {
		return fmt.Errorf("benchmark 未通过: 总体评分 %.1f < 60", metrics.OverallScore)
	}

	return nil
}

func runFuzzBenchmark(seed int64, count int, output, jsonOutput string) error {
	fmt.Printf("正在运行模糊测试 (seed=%d, count=%d)...\n\n", seed, count)

	// 生成模糊测试用例
	gen := NewFuzzGenerator()
	gen.SetSeed(seed)
	testCases := gen.GenerateTestCases(count)

	// 添加边界测试用例
	boundaryCases := gen.GenerateBoundaryTestCases()
	testCases = append(testCases, boundaryCases...)

	// 添加对抗性测试用例
	adversarialCases := gen.GenerateAdversarialTestCases()
	testCases = append(testCases, adversarialCases...)

	fmt.Printf("生成了 %d 个模糊测试用例\n", len(testCases))

	// 运行测试
	runner := NewBenchmarkRunner()
	runner.TestCases = testCases
	runner.Run()

	// 计算指标
	metrics := CalculateMetricsWithDifficulty(runner.Results, testCases)

	// 输出报告
	report := FormatMetrics(metrics)
	fmt.Println(report)

	// 保存报告
	if output != "" {
		if err := os.WriteFile(output, []byte(report), 0644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
	}

	if jsonOutput != "" {
		if err := exportJSON(metrics, runner.Results, jsonOutput); err != nil {
			return fmt.Errorf("导出 JSON 失败: %w", err)
		}
	}

	return nil
}

func runPerformanceTest(output *string) error {
	fmt.Println("正在运行性能测试...")
	fmt.Println("注意：使用 'go test -bench' 运行完整性能测试")

	// 简单的性能测试
	testCases := StandardTestCases()
	start := Now()

	runner := NewBenchmarkRunner()
	runner.TestCases = testCases
	runner.Run()

	duration := Since(start)

	var sb strings.Builder
	sb.WriteString("╔══════════════════════════════════════════════════╗\n")
	sb.WriteString("║              性能测试报告                         ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════╝\n\n")
	sb.WriteString(fmt.Sprintf("总耗时: %s\n", duration))
	sb.WriteString(fmt.Sprintf("用例数: %d\n", len(testCases)))
	sb.WriteString(fmt.Sprintf("平均/用例: %s\n", duration/time.Duration(len(testCases))))

	report := sb.String()
	fmt.Println(report)

	if output != nil && *output != "" {
		if err := os.WriteFile(*output, []byte(report), 0644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
	}

	return nil
}

func runStressTest(output *string) error {
	fmt.Println("正在运行压力测试...")
	fmt.Println("配置: 100个用例，每个5000字符，3个问题")

	// 生成压力测试用例
	gen := NewFuzzGenerator()
	gen.SetSeed(42)
	testCases := gen.GenerateTestCases(100)

	start := Now()
	runner := NewBenchmarkRunner()
	runner.TestCases = testCases
	runner.Run()
	duration := Since(start)

	var sb strings.Builder
	sb.WriteString("╔══════════════════════════════════════════════════╗\n")
	sb.WriteString("║              压力测试报告                         ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════╝\n\n")
	sb.WriteString(fmt.Sprintf("总耗时: %s\n", duration))
	sb.WriteString(fmt.Sprintf("用例数: %d\n", len(testCases)))
	sb.WriteString(fmt.Sprintf("平均/用例: %s\n", duration/time.Duration(len(testCases))))
	if duration.Seconds() > 0 {
		sb.WriteString(fmt.Sprintf("吞吐量: %.2f 用例/秒\n", float64(len(testCases))/duration.Seconds()))
	}

	report := sb.String()
	fmt.Println(report)

	if output != nil && *output != "" {
		if err := os.WriteFile(*output, []byte(report), 0644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
	}

	return nil
}

func runScalabilityTest(output *string) error {
	fmt.Println("正在运行可扩展性测试...")

	gen := NewFuzzGenerator()
	gen.SetSeed(42)

	counts := []int{10, 50, 100, 200}
	var results []string

	for _, count := range counts {
		testCases := gen.GenerateTestCases(count)
		start := Now()
		runner := NewBenchmarkRunner()
		runner.TestCases = testCases
		runner.Run()
		duration := Since(start)

		throughput := float64(0)
		if duration.Seconds() > 0 {
			throughput = float64(count) / duration.Seconds()
		}
		results = append(results, fmt.Sprintf("用例 %4d: %10s (%.2f 用例/秒)", count, duration, throughput))
	}

	var sb strings.Builder
	sb.WriteString("╔══════════════════════════════════════════════════╗\n")
	sb.WriteString("║              可扩展性测试报告                      ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════╝\n\n")
	for _, r := range results {
		sb.WriteString(r + "\n")
	}

	report := sb.String()
	fmt.Println(report)

	if output != nil && *output != "" {
		if err := os.WriteFile(*output, []byte(report), 0644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
	}

	return nil
}

// Now 返回当前时间（用于测试时可以被 mock）
var Now = time.Now

// Since 返回时间差（用于测试时可以被 mock）
var Since = time.Since

func runAllBenchmarks(output, jsonOutput string) error {
	fmt.Println("运行所有 Benchmark 模式...\n")

	// 标准测试
	fmt.Println("=== 标准测试 ===")
	if err := runStandardBenchmark("", "", "", "", "", false); err != nil {
		fmt.Printf("标准测试警告: %v\n", err)
	}

	fmt.Println("\n=== 模糊测试 ===")
	if err := runFuzzBenchmark(42, 30, "", ""); err != nil {
		fmt.Printf("模糊测试警告: %v\n", err)
	}

	fmt.Println("\n=== 性能测试 ===")
	if err := runPerformanceTest(nil); err != nil {
		return err
	}

	return nil
}

// ============================================================
// 辅助函数
// ============================================================

func listTestCases(difficulty, tag, category string) error {
	testCases := getFilteredTestCases(difficulty, tag, category)

	fmt.Printf("共 %d 个测试用例:\n\n", len(testCases))
	fmt.Printf("%-40s %-10s %-10s %-30s\n", "名称", "难度", "问题数", "描述")
	fmt.Println(strings.Repeat("-", 90))

	for _, tc := range testCases {
		diffStr := tc.Difficulty.String()
		fmt.Printf("%-40s %-10s %-10d %-30s\n",
			tc.Name,
			diffStr,
			len(tc.KnownIssues),
			truncateString(tc.Description, 28))
	}

	return nil
}

func getFilteredTestCases(difficulty, tag, category string) []BenchmarkTestCase {
	var testCases []BenchmarkTestCase

	// 先获取基础测试用例
	if difficulty != "" {
		switch difficulty {
		case "easy":
			testCases = GetTestCasesByDifficulty(DifficultyEasy)
		case "medium":
			testCases = GetTestCasesByDifficulty(DifficultyMedium)
		case "hard":
			testCases = GetTestCasesByDifficulty(DifficultyHard)
		case "expert":
			testCases = GetTestCasesByDifficulty(DifficultyExpert)
		default:
			testCases = StandardTestCases()
		}
	} else if tag != "" {
		testCases = GetTestCasesByTag(tag)
	} else if category != "" {
		testCases = GetTestCasesByCategory(category)
	} else {
		testCases = StandardTestCases()
	}

	return testCases
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============================================================
// JSON 导出
// ============================================================

// BenchmarkReport 完整的 Benchmark 报告
type BenchmarkReport struct {
	Timestamp   string         `json:"timestamp"`
	Metrics     Metrics        `json:"metrics"`
	Results     []MatchResult  `json:"results"`
	Summary     ReportSummary  `json:"summary"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalTestCases int                `json:"total_test_cases"`
	Passed         int                `json:"passed"`
	Failed         int                `json:"failed"`
	Categories     []string           `json:"categories"`
	DifficultyDist map[string]int     `json:"difficulty_distribution"`
}

func exportJSON(metrics Metrics, results []MatchResult, filename string) error {
	report := BenchmarkReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Metrics:   metrics,
		Results:   results,
		Summary: ReportSummary{
			TotalTestCases: len(results),
			Categories:     getCategories(results),
			DifficultyDist: getDifficultyDistribution(results),
		},
	}

	// 计算通过/失败
	for _, r := range results {
		if r.KnownIssues == 0 && r.FalsePositives == 0 {
			report.Summary.Passed++
		} else if r.KnownIssues > 0 && float64(r.TruePositives)/float64(r.KnownIssues) >= 0.8 {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
		}
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func getCategories(results []MatchResult) []string {
	categorySet := make(map[string]bool)
	for _, r := range results {
		for _, issue := range r.MatchedKnown {
			categorySet[issue.Category] = true
		}
		for _, issue := range r.MissedKnown {
			categorySet[issue.Category] = true
		}
	}

	categories := make([]string, 0, len(categorySet))
	for cat := range categorySet {
		categories = append(categories, cat)
	}
	return categories
}

func getDifficultyDistribution(results []MatchResult) map[string]int {
	// 这个函数需要根据测试用例的难度信息
	// 这里简化处理，返回空 map
	return make(map[string]int)
}

// ============================================================
// 使用示例
// ============================================================

// ExampleUsage 使用示例
func ExampleUsage() {
	// 基本用法
	// RunCommand([]string{"-mode=standard"})

	// 按难度运行
	// RunCommand([]string{"-difficulty=easy"})

	// 模糊测试
	// RunCommand([]string{"-mode=fuzz", "-fuzz-count=100", "-seed=12345"})

	// 性能测试
	// RunCommand([]string{"-perf"})

	// 导出 JSON
	// RunCommand([]string{"-json=report.json"})
}