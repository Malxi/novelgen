// validate-outline.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/rpg"

	"github.com/spf13/cobra"
)

var (
	validateOutputPath string
	validateFormat     string
	validateStrict     bool
)

func init() {
	validateOutlineCmd.Flags().StringVarP(&validateOutputPath, "output", "o", "", "验证报告输出路径")
	validateOutlineCmd.Flags().StringVarP(&validateFormat, "format", "f", "text", "输出格式 (text, json)")
	validateOutlineCmd.Flags().BoolVarP(&validateStrict, "strict", "s", false, "严格模式：警告也视为失败")
}

var validateOutlineCmd = cobra.Command{
	Use:   "validate-outline [outline-file]",
	Short: "验证大纲结构（支持 ComposeOutput 格式）",
	Long: `验证大纲的结构完整性、叙事流、角色一致性等。

支持以下格式：
  - ComposeOutput 格式 (story/compose/*.json)
  - RPG Outline 格式

示例：
  novelgen validate-outline story/compose/outline.json
  novelgen validate-outline story/compose/outline.json -o validation.json -f json
  novelgen validate-outline story/compose/outline.json --strict`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 确定项目路径和大纲文件路径
		projectPath := "."
		if projectFlag != "" {
			projectPath = projectFlag
		}

		var outlinePath string
		if len(args) > 0 {
			outlinePath = args[0]
		} else {
			// 默认查找 compose 输出
			composeDir := filepath.Join(projectPath, "story", "compose")
			entries, err := os.ReadDir(composeDir)
			if err != nil || len(entries) == 0 {
				return fmt.Errorf("未找到大纲文件，请指定文件路径")
			}

			// 使用最新的 compose 文件
			var latestFile string
			var latestModTime int64
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
					info, err := entry.Info()
					if err != nil {
						continue
					}
					if info.ModTime().Unix() > latestModTime {
						latestModTime = info.ModTime().Unix()
						latestFile = entry.Name()
					}
				}
			}

			if latestFile == "" {
				return fmt.Errorf("未找到有效的 compose 文件")
			}

			outlinePath = filepath.Join(composeDir, latestFile)
		}

		// 读取并解析大纲文件
		data, err := os.ReadFile(outlinePath)
		if err != nil {
			return fmt.Errorf("读取大纲文件失败: %w", err)
		}

		// 尝试解析为 ComposeOutput
		var composeOutput models.ComposeOutput
		var useComposeOutput bool

		if err := json.Unmarshal(data, &composeOutput); err == nil && len(composeOutput.Arcs) > 0 {
			useComposeOutput = true
			fmt.Printf("✓ 检测到 ComposeOutput 格式\n")
		} else {
			fmt.Printf("✓ 检测到 RPG Outline 格式\n")
		}

		// 执行验证
		rules := rpg.GetDefaultValidationRules()
		var report *rpg.ValidationReport

		if useComposeOutput {
			// 执行分析
			analysis := rpg.AnalyzeComposeOutput(&composeOutput)
			fmt.Printf("\n📊 大纲分析:\n")
			fmt.Printf("   情节弧线: %d\n", analysis.TotalArcs)
			fmt.Printf("   总章节数: %d\n", analysis.TotalChapters)
			fmt.Printf("   总场景数: %d\n", analysis.TotalScenes)

			report, err = rpg.ValidateComposeOutput(&composeOutput, rules)
		} else {
			report, err = rpg.ValidateOutlineFile(outlinePath, rules)
		}

		if err != nil {
			return fmt.Errorf("验证失败: %w", err)
		}

		// 输出验证结果
		if validateFormat == "json" {
			return outputJSONReport(report, validateOutputPath)
		}

		return outputTextReport(report, validateOutputPath, validateStrict)
	},
}

func outputJSONReport(report *rpg.ValidationReport, outputPath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化报告失败: %w", err)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("保存报告失败: %w", err)
		}
		fmt.Printf("✅ 验证报告已保存到: %s\n", outputPath)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func outputTextReport(report *rpg.ValidationReport, outputPath string, strict bool) error {
	var output strings.Builder

	// 总体结果
	if report.Valid && (!strict || report.Summary.Warnings == 0) {
		output.WriteString("✅ 验证通过\n")
	} else {
		output.WriteString("❌ 验证未通过\n")
	}

	output.WriteString(fmt.Sprintf("\n📊 总结:\n"))
	output.WriteString(fmt.Sprintf("   总问题数: %d\n", report.Summary.TotalIssues))
	output.WriteString(fmt.Sprintf("   关键问题: %d\n", report.Summary.CriticalIssues))
	output.WriteString(fmt.Sprintf("   警告: %d\n", report.Summary.Warnings))
	output.WriteString(fmt.Sprintf("   建议: %d\n", report.Summary.Suggestions))

	// 问题详情
	if len(report.Issues) > 0 {
		output.WriteString(fmt.Sprintf("\n📝 问题详情:\n"))
		for _, issue := range report.Issues {
			icon := "ℹ️"
			if issue.Type == rpg.IssueTypeWarning {
				icon = "⚠️"
			} else if issue.Type == rpg.IssueTypeError {
				icon = "❌"
			}

			output.WriteString(fmt.Sprintf("\n   %s [%s] %s\n", icon, issue.Category, issue.Message))
			if issue.Suggestion != "" {
				output.WriteString(fmt.Sprintf("      💡 %s\n", issue.Suggestion))
			}
			output.WriteString(fmt.Sprintf("      📍 %s\n", issue.Location))
		}
	}

	// 建议
	if len(report.Suggestions) > 0 {
		output.WriteString(fmt.Sprintf("\n💡 改进建议:\n"))
		for i, suggestion := range report.Suggestions {
			output.WriteString(fmt.Sprintf("   %d. %s\n", i+1, suggestion))
		}
	}

	result := output.String()

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(result), 0644); err != nil {
			return fmt.Errorf("保存报告失败: %w", err)
		}
		fmt.Printf("✅ 验证报告已保存到: %s\n", outputPath)
	} else {
		fmt.Print(result)
	}

	return nil
}