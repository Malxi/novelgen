package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/rpg/dsl"
)

var simulateDSLCmd = &cobra.Command{
	Use:   "simulate-dsl [dsl-file]",
	Short: "运行 DSL 模拟器检测问题",
	Long: `直接运行 DSL 模拟器，检测剧情逻辑问题。

示例:
  novelgen simulate-dsl books/mine/story/rpg/01_outline.rpg
  novelgen simulate-dsl books/mine/story/rpg/01_outline.rpg -o issues.txt`,
	Args: cobra.ExactArgs(1),
	RunE: runSimulateDSL,
}

func init() {
	simulateDSLCmd.Flags().StringP("output", "o", "", "输出报告文件路径")

	// Register command
	RegisterCommand(func() *cobra.Command {
		return simulateDSLCmd
	})
}

func runSimulateDSL(cmd *cobra.Command, args []string) error {
	dslFile := args[0]
	outputPath, _ := cmd.Flags().GetString("output")

	// 检查文件是否存在
	if _, err := os.Stat(dslFile); os.IsNotExist(err) {
		return fmt.Errorf("DSL 文件不存在: %s", dslFile)
	}

	fmt.Printf("📖 DSL 文件: %s\n", dslFile)

	// 读取 DSL 内容
	content, err := os.ReadFile(dslFile)
	if err != nil {
		return fmt.Errorf("读取 DSL 文件失败: %w", err)
	}

	fmt.Printf("📝 DSL 大小: %d 字符\n\n", len(content))

	// 解析 DSL
	fmt.Println("🔍 解析 DSL...")
	parser := dsl.NewParser(string(content))
	dslData, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("解析 DSL 失败: %w", err)
	}

	// 应用默认值
	applySimulationDefaults(filepath.Base(filepath.Dir(filepath.Dir(dslFile))), dslData)

	fmt.Printf("   ✓ 章节数: %d\n", len(dslData.Storyline.Chapters))
	fmt.Printf("   ✓ 角色数: %d\n", len(dslData.Characters.NPCs)+1) // +1 for player
	fmt.Printf("   ✓ 地点数: %d\n\n", len(dslData.World.Locations))

	// 运行模拟器
	fmt.Println("🎮 运行模拟器...")
	simulator := dsl.NewSimulator(dslData)
	issues := simulator.SimulateAll()

	fmt.Printf("   ✓ 模拟完成\n\n")

	// 打印结果
	printSimulateResult(simulator, issues)

	// 保存报告
	if outputPath != "" {
		report := formatSimulateReport(dslFile, issues)
		if err := os.WriteFile(outputPath, []byte(report), 0644); err != nil {
			return fmt.Errorf("保存报告失败: %w", err)
		}
		fmt.Printf("\n💾 报告已保存到: %s\n", outputPath)
	}

	return nil
}

func printSimulateResult(simulator *dsl.Simulator, issues []dsl.SimulationIssue) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("              DSL 模拟器检测报告")
	fmt.Println("═══════════════════════════════════════════════════════════\n")

	// 统计
	criticalCount := 0
	warningCount := 0
	infoCount := 0

	for _, issue := range issues {
		switch issue.Severity {
		case dsl.SeverityCritical:
			criticalCount++
		case dsl.SeverityWarning:
			warningCount++
		case dsl.SeverityInfo:
			infoCount++
		}
	}

	fmt.Printf("📊 问题统计:\n")
	fmt.Printf("   总计: %d\n", len(issues))
	fmt.Printf("   🔴 严重: %d\n", criticalCount)
	fmt.Printf("   🟠 警告: %d\n", warningCount)
	fmt.Printf("   🟢 信息: %d\n\n", infoCount)

	// 详细问题
	if len(issues) == 0 {
		fmt.Println("✅ 未发现问题！")
		return
	}

	fmt.Println("🚨 详细问题列表:\n")

	// 按严重程度排序输出
	for _, issue := range issues {
		if issue.Severity == dsl.SeverityCritical {
			printIssue(issue)
		}
	}
	for _, issue := range issues {
		if issue.Severity == dsl.SeverityWarning {
			printIssue(issue)
		}
	}
	for _, issue := range issues {
		if issue.Severity == dsl.SeverityInfo {
			printIssue(issue)
		}
	}
}

func printIssue(issue dsl.SimulationIssue) {
	severityEmoji := "ℹ️"
	if issue.Severity == dsl.SeverityCritical {
		severityEmoji = "🔴"
	} else if issue.Severity == dsl.SeverityWarning {
		severityEmoji = "🟠"
	}

	fmt.Printf("%s [%s/%s] ", severityEmoji, issue.Severity, issue.Type)
	if issue.Chapter != "" {
		fmt.Printf("%s", issue.Chapter)
		if issue.Step > 0 {
			fmt.Printf(" step %d", issue.Step)
		}
		fmt.Printf(" ")
	}
	fmt.Printf("\n   %s\n", issue.Description)
	if issue.Suggestion != "" {
		fmt.Printf("   💡 建议: %s\n", issue.Suggestion)
	}
	fmt.Println()
}

func formatSimulateReport(dslFile string, issues []dsl.SimulationIssue) string {
	var b strings.Builder

	b.WriteString("DSL Simulator Report\n")
	b.WriteString("====================\n\n")
	b.WriteString(fmt.Sprintf("DSL File: %s\n", dslFile))
	b.WriteString(fmt.Sprintf("Total Issues: %d\n\n", len(issues)))

	for i, issue := range issues {
		b.WriteString(fmt.Sprintf("%d. [%s/%s] ", i+1, issue.Severity, issue.Type))
		if issue.Chapter != "" {
			b.WriteString(fmt.Sprintf("%s", issue.Chapter))
			if issue.Step > 0 {
				b.WriteString(fmt.Sprintf(" step %d", issue.Step))
			}
			b.WriteString(" ")
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("   %s\n", issue.Description))
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
