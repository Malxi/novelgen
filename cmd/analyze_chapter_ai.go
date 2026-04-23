package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/models"
)

var analyzeChapterAICmd = &cobra.Command{
	Use:   "analyze-chapter-ai [chapter-file]",
	Short: "使用AI分析章节RPG数据",
	Long: `使用AI Agent分析小说章节，提取角色、事件、战力变化等RPG数据。

示例:
  novelgen analyze-chapter-ai books/mine/chapters/chapter-P1-V1-C1.md
  novelgen analyze-chapter-ai books/mine/chapters/chapter-P1-V1-C1.md --output=analysis.json`,
	Args: cobra.ExactArgs(1),
	RunE: runAnalyzeChapterAI,
}

func init() {
	analyzeChapterAICmd.Flags().StringP("output", "o", "", "输出文件路径 (JSON格式)")
	analyzeChapterAICmd.Flags().BoolP("verbose", "v", false, "详细输出")

	// Register command
	RegisterCommand(func() *cobra.Command {
		return analyzeChapterAICmd
	})
}

func runAnalyzeChapterAI(cmd *cobra.Command, args []string) error {
	chapterFile := args[0]
	outputPath, _ := cmd.Flags().GetString("output")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// 检查文件是否存在
	if _, err := os.Stat(chapterFile); os.IsNotExist(err) {
		return fmt.Errorf("章节文件不存在: %s", chapterFile)
	}

	// 读取章节内容
	content, err := os.ReadFile(chapterFile)
	if err != nil {
		return fmt.Errorf("读取章节文件失败: %w", err)
	}

	// 提取章节信息
	chapterID := extractChapterIDFromFilename(chapterFile)
	chapterTitle := extractChapterTitle(string(content))

	fmt.Printf("📖 分析章节: %s - %s\n", chapterID, chapterTitle)
	fmt.Printf("📝 内容长度: %d 字符\n\n", len(content))

	// 获取书籍目录（从章节文件路径推断）
	bookPath := filepath.Dir(filepath.Dir(chapterFile))

	// 初始化 LLM 客户端
	client, config, projectLLM, err := initLLMForAnalysis(bookPath)
	if err != nil {
		return fmt.Errorf("初始化LLM失败: %w", err)
	}

	// 创建 AI Agent
	agent := agents.NewChapterAnalysisAgent(client, config, projectLLM)

	// 准备输入
	input := agents.ChapterAnalysisInput{
		ChapterID:      chapterID,
		ChapterTitle:   chapterTitle,
		ChapterContent: string(content),
		StorySetup:     "主角有复活金手指，每次复活掉一层修为", // 可以从配置文件读取
	}

	// 执行分析
	ctx := context.Background()
	output, err := agent.Analyze(ctx, input)
	if err != nil {
		return fmt.Errorf("AI分析失败: %w", err)
	}

	// 打印结果
	printAnalysisResult(output, verbose)

	// 保存到文件
	if outputPath != "" {
		if err := saveAnalysisResult(output, outputPath); err != nil {
			return fmt.Errorf("保存结果失败: %w", err)
		}
		fmt.Printf("\n💾 结果已保存到: %s\n", outputPath)
	}

	return nil
}

// initLLMForAnalysis 初始化LLM客户端
func initLLMForAnalysis(bookPath string) (llm.Client, *llm.Config, *models.ProjectLLM, error) {
	// 1. 从 novel.json 读取项目配置
	novelJSONPath := filepath.Join(bookPath, "novel.json")
	var projectLLM *models.ProjectLLM
	
	if data, err := os.ReadFile(novelJSONPath); err == nil {
		var novelConfig struct {
			LLM struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
			} `json:"llm"`
		}
		if err := json.Unmarshal(data, &novelConfig); err == nil && novelConfig.LLM.Provider != "" {
			projectLLM = &models.ProjectLLM{
				Provider: novelConfig.LLM.Provider,
				Model:    novelConfig.LLM.Model,
			}
			fmt.Printf("📚 从 novel.json 读取配置: provider=%s, model=%s\n", projectLLM.Provider, projectLLM.Model)
		}
	}
	
	if projectLLM == nil {
		return nil, nil, nil, fmt.Errorf("未在 novel.json 中找到 LLM 配置")
	}
	
	// 2. 加载 llm_config.json 获取完整配置（包括 API key）
	config, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载 LLM 配置失败: %w", err)
	}
	
	// 3. 获取活跃 provider 和 model
	provider, model := config.GetActiveModel(projectLLM)
	if provider == nil {
		return nil, nil, nil, fmt.Errorf("provider 未找到: %s", projectLLM.Provider)
	}
	if model == nil {
		return nil, nil, nil, fmt.Errorf("model 未找到: %s", projectLLM.Model)
	}
	
	fmt.Printf("🔑 使用配置: provider=%s, model=%s, base_url=%s\n", provider.Name, model.Name, provider.BaseURL)
	
	// 4. 创建客户端
	client := llm.NewOpenAIClient(&llm.OpenAIConfig{
		APIKey:  provider.APIKey,
		BaseURL: provider.BaseURL,
		Model:   model.Name,
		Timeout: provider.Timeout,
	})
	
	return client, config, projectLLM, nil
}

// printAnalysisResult 打印分析结果
func printAnalysisResult(output agents.ChapterAnalysisOutput, verbose bool) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("                    AI 章节分析结果")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	// 角色信息
	fmt.Printf("👤 角色 (%d 个):\n", len(output.Characters))
	for _, char := range output.Characters {
		status := "🟢"
		if char.State == "dead" {
			status = "💀"
		} else if char.State == "injured" {
			status = "🩹"
		}
		
		resurrectInfo := ""
		if char.IsResurrected {
			resurrectInfo = " [复活]"
		}
		
		fmt.Printf("   %s %s%s\n", status, char.Name, resurrectInfo)
		if char.Cultivation != "" {
			fmt.Printf("      修为: %s\n", char.Cultivation)
		}
		if char.ResurrectionCost != "" {
			fmt.Printf("      复活代价: %s\n", char.ResurrectionCost)
		}
	}
	fmt.Println()

	// 事件信息
	fmt.Printf("📅 事件 (%d 个):\n", len(output.Events))
	for _, event := range output.Events {
		typeEmoji := "📝"
		switch event.Type {
		case "combat":
			typeEmoji = "⚔️"
		case "death":
			typeEmoji = "💀"
		case "resurrection":
			typeEmoji = "✨"
		case "cultivation":
			typeEmoji = "🧘"
		}
		fmt.Printf("   %s %s\n", typeEmoji, event.Description)
		if verbose && len(event.Characters) > 0 {
			fmt.Printf("      参与: %s\n", strings.Join(event.Characters, ", "))
		}
	}
	fmt.Println()

	// 战力变化
	if len(output.PowerChanges) > 0 {
		fmt.Printf("⚡ 战力变化 (%d 次):\n", len(output.PowerChanges))
		for _, change := range output.PowerChanges {
			legitEmoji := "✅"
			if !change.IsLegitimate {
				legitEmoji = "⚠️"
			}
			fmt.Printf("   %s %s: %s → %s (%s)\n", 
				legitEmoji, change.Character, change.FromLevel, change.ToLevel, change.Reason)
		}
		fmt.Println()
	}

	// 时间线
	fmt.Printf("⏱️  时间线: %s (%s)\n", output.TimelineInfo.Duration, map[bool]string{true: "连续", false: "有跳跃"}[output.TimelineInfo.IsContinuous])
	fmt.Println()

	// 问题检测
	if len(output.Issues) > 0 {
		fmt.Printf("🚨 检测到 %d 个问题:\n", len(output.Issues))
		for _, issue := range output.Issues {
			severityEmoji := "ℹ️"
			switch issue.Severity {
			case "critical":
				severityEmoji = "🔴"
			case "error":
				severityEmoji = "🟠"
			case "warning":
				severityEmoji = "🟡"
			}
			fmt.Printf("   %s [%s] %s: %s\n", severityEmoji, issue.Category, issue.Target, issue.Description)
			if verbose && issue.Evidence != "" {
				fmt.Printf("      证据: %s\n", issue.Evidence)
			}
			if verbose && issue.Suggestion != "" {
				fmt.Printf("      建议: %s\n", issue.Suggestion)
			}
		}
	} else {
		fmt.Println("✅ 未检测到明显问题")
	}
	fmt.Println()

	// 摘要
	fmt.Println("📋 章节摘要:")
	fmt.Printf("   %s\n", output.Summary)
}

// saveAnalysisResult 保存分析结果到文件
func saveAnalysisResult(output agents.ChapterAnalysisOutput, path string) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// extractChapterIDFromFilename 从文件名提取章节ID
func extractChapterIDFromFilename(filename string) string {
	base := filepath.Base(filename)
	base = strings.TrimPrefix(base, "chapter-")
	base = strings.TrimSuffix(base, ".md")
	return base
}

// extractChapterTitle 从内容提取标题
func extractChapterTitle(content string) string {
	lines := strings.Split(content, "\n")
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
