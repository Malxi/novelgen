package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/rpg"
)

func main() {
	var (
		inputPath  = flag.String("i", "", "输入小说文件路径 (.md 或 .txt) (必需)")
		outputPath = flag.String("o", "", "输出RPG数据JSON文件路径 (可选)")
		reportPath = flag.String("r", "", "输出模拟报告JSON文件路径 (可选)")
		_          = flag.String("f", "text", "输出格式: text(文本), json(JSON)") // 保留以备将来使用
		simulate   = flag.Bool("s", true, "是否执行RPG模拟验证")
		verbose    = flag.Bool("v", false, "显示详细信息")
	)
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("错误: 请提供输入文件路径")
		fmt.Println("用法: extract-simulate -i <小说文件路径> [选项]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 读取小说文件
	fmt.Printf("📖 正在读取小说文件: %s\n", *inputPath)
	content, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Printf("❌ 错误: 无法读取文件: %v\n", err)
		os.Exit(1)
	}

	// 使用AI提取器提取RPG数据
	fmt.Println("🤖 AI正在分析小说内容...")
	extractor := rpg.NewAIExtractor()
	data := extractor.ExtractFromNovel(string(content))

	// 显示提取统计
	stats := extractor.GetExtractionStats()
	printExtractionStats(stats)

	// 执行模拟验证
	var report *rpg.SimulationReport
	if *simulate {
		fmt.Println("\n🎮 正在执行RPG模拟验证...")
		simulator := rpg.NewNovelSimulator(data)
		report = simulator.Simulate()
		printSimulationReport(report)
	}

	// 输出RPG数据
	if *outputPath != "" {
		if err := saveRPGData(data, *outputPath); err != nil {
			fmt.Printf("❌ 保存RPG数据失败: %v\n", err)
		} else {
			fmt.Printf("\n✅ RPG数据已保存: %s\n", *outputPath)
		}
	}

	// 输出模拟报告
	if *simulate && *reportPath != "" {
		if err := saveSimulationReport(report, *reportPath); err != nil {
			fmt.Printf("❌ 保存模拟报告失败: %v\n", err)
		} else {
			fmt.Printf("✅ 模拟报告已保存: %s\n", *reportPath)
		}
	}

	// 生成可视化报告
	fmt.Println("\n🎨 正在生成可视化报告...")
	vizDir := filepath.Join(filepath.Dir(*outputPath), "visualization")
	if err := os.MkdirAll(vizDir, 0755); err == nil {
		files, err := rpg.GenerateAllReports(data, report, vizDir)
		if err != nil {
			fmt.Printf("⚠️  生成可视化报告失败: %v\n", err)
		} else {
			fmt.Printf("✅ 可视化报告已生成:\n")
			for _, file := range files {
				fmt.Printf("   - %s\n", file)
			}
		}
	}

	// 显示验证问题
	if len(data.ValidationIssues) > 0 {
		printValidationIssues(data.ValidationIssues)
	}

	// 显示详细信息
	if *verbose {
		printDetailedInfo(data)
	}
}

// printExtractionStats 打印提取统计
func printExtractionStats(stats map[string]interface{}) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 提取统计")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("时间线事件数: %v\n", stats["timeline_events"])
	fmt.Printf("角色弧线数: %v\n", stats["character_arcs"])
	fmt.Printf("复活次数: %v\n", stats["resurrection_count"])
	fmt.Printf("战力变化次数: %v\n", stats["power_changes"])
	fmt.Printf("不一致问题: %v\n", stats["inconsistencies"])

	if charDetails, ok := stats["character_details"].(map[string]interface{}); ok {
		fmt.Println("\n角色详情:")
		for name, details := range charDetails {
			if d, ok := details.(map[string]interface{}); ok {
				fmt.Printf("  - %s: 出场%v次, 死亡%v次, 复活%v次\n",
					name, d["appearances"], d["deaths"], d["resurrects"])
			}
		}
	}
}

// printSimulationReport 打印模拟报告
func printSimulationReport(report *rpg.SimulationReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎮 RPG模拟验证报告")
	fmt.Println(strings.Repeat("=", 60))

	summary := report.Summary
	fmt.Printf("\n总评分: %.1f/100 (评级: %s)\n", summary.OverallScore, summary.Grade)
	fmt.Printf("模拟事件数: %d\n", summary.TotalEvents)
	fmt.Printf("涉及角色数: %d\n", summary.CharactersInvolved)
	fmt.Printf("发现问题: %d\n", summary.IssuesFound)
	fmt.Printf("严重问题: %d\n", summary.CriticalIssues)

	// 显示各类验证结果
	fmt.Println("\n【验证详情】")
	for _, result := range report.ValidationResults {
		status := "✅"
		if !result.Passed {
			status = "❌"
		}
		fmt.Printf("\n%s %s (得分: %.1f)\n", status, getCategoryName(result.Category), result.Score)

		if len(result.Issues) > 0 {
			fmt.Println("  问题:")
			for _, issue := range result.Issues {
				fmt.Printf("    - %s\n", issue)
			}
		}
	}

	// 显示建议
	if len(report.Recommendations) > 0 {
		fmt.Println("\n【改进建议】")
		for i, rec := range report.Recommendations {
			fmt.Printf("%d. %s\n", i+1, rec)
		}
	}
}

// printValidationIssues 打印验证问题
func printValidationIssues(issues []rpg.ValidationIssue) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("⚠️  验证问题")
	fmt.Println(strings.Repeat("=", 60))

	// 按严重程度分组
	fatalIssues := make([]rpg.ValidationIssue, 0)
	criticalIssues := make([]rpg.ValidationIssue, 0)
	warningIssues := make([]rpg.ValidationIssue, 0)
	infoIssues := make([]rpg.ValidationIssue, 0)

	for _, issue := range issues {
		switch issue.Severity {
		case "fatal":
			fatalIssues = append(fatalIssues, issue)
		case "critical":
			criticalIssues = append(criticalIssues, issue)
		case "warning":
			warningIssues = append(warningIssues, issue)
		default:
			infoIssues = append(infoIssues, issue)
		}
	}

	printIssueGroup("致命问题", fatalIssues, "🔴")
	printIssueGroup("严重问题", criticalIssues, "🟠")
	printIssueGroup("警告", warningIssues, "🟡")
	printIssueGroup("建议", infoIssues, "🔵")
}

func printIssueGroup(title string, issues []rpg.ValidationIssue, emoji string) {
	if len(issues) == 0 {
		return
	}
	fmt.Printf("\n%s %s (%d):\n", emoji, title, len(issues))
	for _, issue := range issues {
		fmt.Printf("  - [%s] %s\n", issue.Type, issue.Message)
		if issue.Suggestion != "" {
			fmt.Printf("    💡 %s\n", issue.Suggestion)
		}
	}
}

// printDetailedInfo 打印详细信息
func printDetailedInfo(data *rpg.NovelRPGData) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📋 详细RPG数据")
	fmt.Println(strings.Repeat("=", 60))

	// 角色信息
	if len(data.Characters) > 0 {
		fmt.Printf("\n【角色】(%d)\n", len(data.Characters))
		for _, char := range data.Characters {
			fmt.Printf("  - %s (%s)\n", char.Name, char.ClassID)
			fmt.Printf("    HP: %d, MP: %d, 攻击: %d, 防御: %d\n",
				char.BaseStats.HP, char.BaseStats.MP, char.BaseStats.Attack, char.BaseStats.Defense)
		}
	}

	// 物品信息
	if len(data.Items) > 0 {
		fmt.Printf("\n【物品】(%d)\n", len(data.Items))
		for _, item := range data.Items {
			fmt.Printf("  - %s [%s]\n", item.Name, item.Type)
		}
	}

	// 技能信息
	if len(data.Skills) > 0 {
		fmt.Printf("\n【技能】(%d)\n", len(data.Skills))
		for _, skill := range data.Skills {
			fmt.Printf("  - %s [%s]\n", skill.Name, skill.Type)
		}
	}

	// 地点信息
	if len(data.Locations) > 0 {
		fmt.Printf("\n【地点】(%d)\n", len(data.Locations))
		for _, loc := range data.Locations {
			fmt.Printf("  - %s [%s]\n", loc.Name, loc.Type)
		}
	}

	// 时间线
	if len(data.Timeline) > 0 {
		fmt.Printf("\n【时间线】(%d 事件)\n", len(data.Timeline))
		for _, event := range data.Timeline {
			if len(event.Events) > 0 {
				fmt.Printf("  第%d天: %v @ %s\n", event.Day, event.Events, event.Location)
			}
		}
	}
}

// getCategoryName 获取类别名称
func getCategoryName(category string) string {
	names := map[string]string{
		"character_consistency": "角色一致性",
		"power_system":          "战力系统",
		"economy_system":        "经济系统",
		"combat_balance":        "战斗平衡",
		"plot_logic":            "剧情逻辑",
		"pacing":                "节奏控制",
	}

	if name, ok := names[category]; ok {
		return name
	}
	return category
}

// saveRPGData 保存RPG数据
func saveRPGData(data *rpg.NovelRPGData, path string) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0644)
}

// saveSimulationReport 保存模拟报告
func saveSimulationReport(report *rpg.SimulationReport, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0644)
}
