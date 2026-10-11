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
		outlinePath = flag.String("i", "", "输入大纲JSON文件路径 (必需)")
		outputPath  = flag.String("o", "", "输出检测结果JSON文件路径 (可选)")
		showDetail  = flag.Bool("d", true, "显示详细信息")
	)
	flag.Parse()

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

	// 执行RPG检测
	checker := rpg.NewOutlineRPGChecker()
	result := checker.Check(&outline)

	// 输出结果
	if *outputPath != "" {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(*outputPath, resultJSON, 0644); err != nil {
			fmt.Printf("警告: 无法保存结果文件: %v\n", err)
		} else {
			fmt.Printf("检测结果已保存到: %s\n", *outputPath)
		}
	}

	// 打印报告
	if *showDetail {
		printRPGReport(result)
	} else {
		printRPGSummary(result)
	}
}

func printRPGReport(result *rpg.RPGCheckResult) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("🎮 RPG大纲健康度检测报告")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// 总评分
	gradeEmoji := getGradeEmoji(result.Grade)
	fmt.Printf("%s 总评分: %d/100 (评级: %s)\n", gradeEmoji, result.TotalScore, result.Grade)
	fmt.Println()

	// 诊断
	fmt.Println("【诊断结果】")
	fmt.Println(result.Diagnosis)
	fmt.Println()

	// 属性面板
	fmt.Println("【属性面板】")
	printStatBar("结构完整性", result.Stats.StructureIntegrity)
	printStatBar("逻辑一致性", result.Stats.LogicConsistency)
	printStatBar("角色平衡性", result.Stats.CharacterBalance)
	printStatBar("剧情连贯性", result.Stats.PlotCoherence)
	printStatBar("节奏质量", result.Stats.PacingQuality)
	fmt.Println()

	// 战斗属性
	fmt.Println("【战斗属性】")
	printStatBar("战力系统防御", result.Stats.PowerSystemDefense)
	printStatBar("时间线稳定性", result.Stats.TimelineStability)
	printStatBar("角色聚焦度", result.Stats.CharacterFocus)
	printStatBar("冲突强度", result.Stats.ConflictIntensity)
	fmt.Println()

	// 资源属性
	fmt.Println("【资源属性】")
	printStatBar("寿命/可持续性", result.Stats.LifeSpan)
	printStatBar("剧情护甲", result.Stats.PlotArmor)
	printStatBar("可信度", result.Stats.SuspensionOfDisbelief)
	fmt.Println()

	// 负面状态
	if len(result.Debuffs) > 0 {
		fmt.Println("【负面状态】")
		for i, debuff := range result.Debuffs {
			severityEmoji := getSeverityEmoji(debuff.Severity)
			fmt.Printf("\n%d. %s %s (严重度: %d/10)\n", i+1, severityEmoji, debuff.Name, debuff.Severity)
			fmt.Printf("   📋 %s\n", debuff.Description)
			fmt.Printf("   ⚡ 效果: %s\n", debuff.Effect)
		}
		fmt.Println()
	}

	// BOSS战
	if len(result.Bosses) > 0 {
		fmt.Println("【BOSS战 - 需要击败的严重问题】")
		for i, boss := range result.Bosses {
			fmt.Printf("\n%d. 👹 %s\n", i+1, boss.Name)
			fmt.Printf("   HP: %d/%d | 攻击: %d | 防御: %d\n", boss.HP, boss.MaxHP, boss.Attack, boss.Defense)
			fmt.Printf("   📋 %s\n", boss.Description)
			fmt.Printf("   🎯 弱点: %v\n", boss.Weaknesses)
			fmt.Printf("   🎁 击败掉落: %v\n", boss.Drops)
		}
		fmt.Println()
	}

	// 治疗方案
	if len(result.Prescription) > 0 {
		fmt.Println("【治疗方案】")
		for i, prescription := range result.Prescription {
			fmt.Printf("%d. %s\n", i+1, prescription)
		}
		fmt.Println()
	}
}

func printRPGSummary(result *rpg.RPGCheckResult) {
	gradeEmoji := getGradeEmoji(result.Grade)
	fmt.Printf("%s 大纲评级: %s (评分: %d/100)\n", gradeEmoji, result.Grade, result.TotalScore)
	fmt.Println(result.Diagnosis)

	if len(result.Bosses) > 0 {
		fmt.Printf("\n⚠️ 发现 %d 个BOSS级问题需要处理\n", len(result.Bosses))
	}

	if len(result.Debuffs) > 0 {
		fmt.Printf("⚡ 发现 %d 个负面状态\n", len(result.Debuffs))
	}
}

func printStatBar(name string, value int) {
	barLength := 20
	filled := value * barLength / 100
	empty := barLength - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	color := getStatColor(value)

	fmt.Printf("  %-12s [%s] %s%d%%%s\n", name, bar, color, value, "\033[0m")
}

func getStatColor(value int) string {
	if value >= 80 {
		return "\033[32m" // 绿色
	} else if value >= 60 {
		return "\033[33m" // 黄色
	} else if value >= 40 {
		return "\033[33m" // 橙色
	}
	return "\033[31m" // 红色
}

func getGradeEmoji(grade string) string {
	switch grade {
	case "S":
		return "🏆"
	case "A":
		return "🥇"
	case "B":
		return "🥈"
	case "C":
		return "🥉"
	case "D":
		return "⚠️"
	case "F":
		return "💀"
	}
	return "❓"
}

func getSeverityEmoji(severity int) string {
	if severity >= 8 {
		return "🔴"
	} else if severity >= 5 {
		return "🟠"
	} else if severity >= 3 {
		return "🟡"
	}
	return "🟢"
}

func printUsage() {
	fmt.Println()
	fmt.Println("用法: rpg-check -i <大纲文件> [选项]")
	fmt.Println()
	fmt.Println("必需参数:")
	fmt.Println("  -i string    输入大纲JSON文件路径")
	fmt.Println()
	fmt.Println("可选参数:")
	fmt.Println("  -o string    输出检测结果JSON文件路径")
	fmt.Println("  -d bool      显示详细信息 (默认: true)")
	fmt.Println("  -h           显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  rpg-check -i outline.json")
	fmt.Println("  rpg-check -i outline.json -o result.json")
	fmt.Println()
}
