package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"novelgen/internal/rpg"
)

func main() {
	var (
		outlinePath  = flag.String("i", "", "输入大纲JSON文件路径 (必需)")
		configPath   = flag.String("c", "", "分析配置JSON文件路径 (可选，使用内置配置)")
		outputPath   = flag.String("o", "", "输出分析结果JSON文件路径 (可选)")
		analyzerType = flag.String("t", "general", "分析器类型: general(通用), xianxia(修仙), infinite(无限流)")
		showDetail   = flag.Bool("d", true, "显示详细信息")
		exportConfig = flag.String("export-config", "", "导出内置配置到指定路径")
	)
	flag.Parse()

	// 导出配置模式
	if *exportConfig != "" {
		exportBuiltinConfig(*exportConfig, *analyzerType)
		return
	}

	if *outlinePath == "" {
		fmt.Println("错误: 请使用 -i 参数指定大纲文件路径")
		printUsage()
		os.Exit(1)
	}

	// 读取大纲
	outlineData, err := os.ReadFile(*outlinePath)
	if err != nil {
		fmt.Printf("错误: 无法读取大纲文件: %v\n", err)
		os.Exit(1)
	}

	var outline rpg.StoryOutline
	if err := json.Unmarshal(outlineData, &outline); err != nil {
		fmt.Printf("错误: 无法解析大纲JSON: %v\n", err)
		os.Exit(1)
	}

	// 加载或创建配置
	var config *rpg.AnalyzerConfig
	if *configPath != "" {
		// 从文件加载配置
		configData, err := os.ReadFile(*configPath)
		if err != nil {
			fmt.Printf("错误: 无法读取配置文件: %v\n", err)
			os.Exit(1)
		}
		config, err = rpg.LoadAnalyzerConfig(configData)
		if err != nil {
			fmt.Printf("错误: 无法解析配置文件: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已加载自定义配置: %s\n", config.Name)
	} else {
		// 使用内置配置
		config = getBuiltinConfig(*analyzerType)
		fmt.Printf("使用内置配置: %s\n", config.Name)
	}

	// 执行分析
	analyzer := rpg.NewOutlineAnalyzer(config)
	result := analyzer.Analyze(&outline)

	// 输出结果
	if *outputPath != "" {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(*outputPath, resultJSON, 0644); err != nil {
			fmt.Printf("警告: 无法保存结果文件: %v\n", err)
		} else {
			fmt.Printf("分析结果已保存到: %s\n", *outputPath)
		}
	}

	// 打印报告
	if *showDetail {
		printAnalysisReport(result)
	} else {
		printSummary(result)
	}
}

func getBuiltinConfig(analyzerType string) *rpg.AnalyzerConfig {
	switch analyzerType {
	case "xianxia":
		return rpg.GetXianxiaAnalysisConfig()
	case "infinite", "infinite-flow":
		return rpg.GetInfiniteFlowAnalysisConfig()
	default:
		return rpg.GetNovelAnalysisConfig()
	}
}

func exportBuiltinConfig(path, analyzerType string) {
	config := getBuiltinConfig(analyzerType)
	configData, err := rpg.SaveAnalyzerConfig(config)
	if err != nil {
		fmt.Printf("错误: 无法序列化配置: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(path, configData, 0644); err != nil {
		fmt.Printf("错误: 无法写入配置文件: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已导出 '%s' 配置到: %s\n", config.Name, path)
	fmt.Println("\n你可以编辑此配置文件来自定义分析规则，然后使用 -c 参数加载:")
	fmt.Printf("  analyze-outline -i outline.json -c %s\n", path)
}

func printAnalysisReport(result *rpg.AnalysisResult) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("📚 %s\n", result.Config.Name)
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("📝 %s\n", result.Config.Description)
	fmt.Println()

	// 摘要
	fmt.Println("【分析摘要】")
	fmt.Printf("总章节数: %d\n", result.Metrics.TotalChapters)
	fmt.Printf("总角色数: %d\n", result.Metrics.TotalCharacters)
	fmt.Printf("总地点数: %d\n", result.Metrics.TotalLocations)
	fmt.Printf("总事件数: %d\n", result.Metrics.TotalEvents)
	fmt.Println()

	// 风险等级
	riskEmoji := getRiskEmoji(result.RiskLevel)
	fmt.Printf("%s 风险等级: %s\n", riskEmoji, result.RiskLevel)
	fmt.Println()

	// 问题列表
	if len(result.Issues) > 0 {
		// 按严重程度分组
		groupedIssues := groupIssuesBySeverity(result.Issues)

		if fatalIssues, ok := groupedIssues["fatal"]; ok && len(fatalIssues) > 0 {
			printIssueGroup("🚨 致命问题", fatalIssues)
		}
		if criticalIssues, ok := groupedIssues["critical"]; ok && len(criticalIssues) > 0 {
			printIssueGroup("⚠️ 严重问题", criticalIssues)
		}
		if warningIssues, ok := groupedIssues["warning"]; ok && len(warningIssues) > 0 {
			printIssueGroup("⚡ 警告", warningIssues)
		}
		if infoIssues, ok := groupedIssues["info"]; ok && len(infoIssues) > 0 {
			printIssueGroup("💡 建议", infoIssues)
		}
	} else {
		fmt.Println("✅ 未发现明显问题")
	}

	// 统计信息
	fmt.Println()
	fmt.Println("【统计信息】")
	fmt.Println("事件类型分布:")
	for eventType, count := range result.Metrics.EventTypeCounts {
		fmt.Printf("  - %s: %d\n", eventType, count)
	}

	fmt.Println()
	fmt.Println("角色出场频率 (Top 10):")
	sortedChars := getTopCharacters(result.Metrics.CharacterFrequency, 10)
	for _, char := range sortedChars {
		freq := result.Metrics.CharacterFrequency[char]
		fmt.Printf("  - %s: %d次\n", char, freq)
	}
}

func printSummary(result *rpg.AnalysisResult) {
	fmt.Println(result.Summary)
}

func printIssueGroup(title string, issues []rpg.AnalysisIssue) {
	fmt.Println()
	fmt.Printf("%s (%d)\n", title, len(issues))
	fmt.Println(strings.Repeat("-", 50))
	
	for i, issue := range issues {
		fmt.Printf("\n%d. %s\n", i+1, issue.Message)
		if issue.Location != "" {
			fmt.Printf("   📍 位置: %s\n", issue.Location)
		}
		if issue.Suggestion != "" {
			fmt.Printf("   💡 建议: %s\n", issue.Suggestion)
		}
	}
}

func groupIssuesBySeverity(issues []rpg.AnalysisIssue) map[string][]rpg.AnalysisIssue {
	grouped := make(map[string][]rpg.AnalysisIssue)
	for _, issue := range issues {
		grouped[issue.Severity] = append(grouped[issue.Severity], issue)
	}
	return grouped
}

func getRiskEmoji(riskLevel string) string {
	if strings.Contains(riskLevel, "极高") {
		return "🔴"
	} else if strings.Contains(riskLevel, "高") {
		return "🟠"
	} else if strings.Contains(riskLevel, "中等") {
		return "🟡"
	} else if strings.Contains(riskLevel, "低") {
		return "🟢"
	}
	return "⚪"
}

func getTopCharacters(freq map[string]int, n int) []string {
	type charFreq struct {
		name string
		freq int
	}

	var chars []charFreq
	for name, f := range freq {
		chars = append(chars, charFreq{name, f})
	}

	// 简单排序（冒泡）
	for i := 0; i < len(chars); i++ {
		for j := i + 1; j < len(chars); j++ {
			if chars[j].freq > chars[i].freq {
				chars[i], chars[j] = chars[j], chars[i]
			}
		}
	}

	var result []string
	for i := 0; i < n && i < len(chars); i++ {
		result = append(result, chars[i].name)
	}
	return result
}

func printUsage() {
	fmt.Println()
	fmt.Println("用法: analyze-outline -i <大纲文件> [选项]")
	fmt.Println()
	fmt.Println("必需参数:")
	fmt.Println("  -i string    输入大纲JSON文件路径")
	fmt.Println()
	fmt.Println("可选参数:")
	fmt.Println("  -c string    分析配置JSON文件路径")
	fmt.Println("  -o string    输出分析结果JSON文件路径")
	fmt.Println("  -t string    分析器类型: general(通用), xianxia(修仙), infinite(无限流)")
	fmt.Println("  -d bool      显示详细信息 (默认: true)")
	fmt.Println("  -export-config string  导出内置配置到指定路径")
	fmt.Println("  -h           显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  # 使用通用分析器")
	fmt.Println("  analyze-outline -i outline.json")
	fmt.Println()
	fmt.Println("  # 使用修仙小说专用分析器")
	fmt.Println("  analyze-outline -i outline.json -t xianxia")
	fmt.Println()
	fmt.Println("  # 导出配置并自定义")
	fmt.Println("  analyze-outline -export-config my-config.json -t xianxia")
	fmt.Println("  # 编辑 my-config.json 后使用")
	fmt.Println("  analyze-outline -i outline.json -c my-config.json")
	fmt.Println()
}
