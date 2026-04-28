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

var analyzeAIRPGCmd = &cobra.Command{
	Use:   "analyze-ai-rpg [chapter-file]",
	Short: "使用AI+RPG Pipeline分析章节",
	Long: `完整流程: chapters -> AI Agent -> RPG DSL -> RPG Simulate -> Issues

示例:
  novelgen analyze-ai-rpg books/mine/chapters/chapter-P1-V1-C1.md
  novelgen analyze-ai-rpg books/mine/chapters/chapter-P1-V1-C1.md -o report.json`,
	Args: cobra.ExactArgs(1),
	RunE: runAnalyzeAIRPG,
}

func init() {
	analyzeAIRPGCmd.Flags().StringP("output", "o", "", "输出文件路径 (JSON格式)")
	analyzeAIRPGCmd.Flags().BoolP("verbose", "v", false, "详细输出")
	analyzeAIRPGCmd.Flags().BoolP("batch", "b", false, "批量处理目录下所有章节")

	// Register command
	RegisterCommand(func() *cobra.Command {
		return analyzeAIRPGCmd
	})
}

func runAnalyzeAIRPG(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	outputPath, _ := cmd.Flags().GetString("output")
	verbose, _ := cmd.Flags().GetBool("verbose")
	batchMode, _ := cmd.Flags().GetBool("batch")

	// 初始化 LLM
	bookPath := filepath.Dir(filepath.Dir(inputPath))
	client, config, projectLLM, err := initLLMForAIRPG(bookPath)
	if err != nil {
		return fmt.Errorf("初始化LLM失败: %w", err)
	}

	// 创建 Pipeline
	pipeline := agents.NewAIRPGPipeline(client, config, projectLLM)
	ctx := context.Background()

	if batchMode {
		// 批量模式
		return runBatchMode(ctx, pipeline, inputPath, outputPath, verbose)
	}

	// 单章节模式
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", inputPath)
	}

	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	chapterID := extractChapterIDFromFilename(inputPath)
	chapterTitle := extractChapterTitle(string(content))

	fmt.Printf("🚀 启动 AI RPG Pipeline\n")
	fmt.Printf("📖 章节: %s - %s\n\n", chapterID, chapterTitle)

	result, err := pipeline.ProcessChapter(ctx, chapterID, chapterTitle, string(content))
	if err != nil {
		return fmt.Errorf("Pipeline执行失败: %w", err)
	}

	// 打印报告
	report := pipeline.FormatPipelineReport(result)
	fmt.Println(report)

	// 保存结果
	if outputPath != "" {
		if err := saveJSON(outputPath, result); err != nil {
			return fmt.Errorf("保存结果失败: %w", err)
		}
		fmt.Printf("💾 结果已保存到: %s\n", outputPath)
	}

	return nil
}

func runBatchMode(ctx context.Context, pipeline *agents.AIRPGPipeline, dirPath, outputPath string, verbose bool) error {
	// 如果是单个文件，获取其目录
	info, err := os.Stat(dirPath)
	if err != nil {
		return err
	}

	chaptersDir := dirPath
	if !info.IsDir() {
		chaptersDir = filepath.Dir(dirPath)
	}

	// 读取所有章节文件
	files, err := os.ReadDir(chaptersDir)
	if err != nil {
		return fmt.Errorf("读取目录失败: %w", err)
	}

	var chapters []agents.ChapterInput
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			content, err := os.ReadFile(filepath.Join(chaptersDir, f.Name()))
			if err != nil {
				continue
			}

			chapters = append(chapters, agents.ChapterInput{
				ID:      extractChapterIDFromFilename(f.Name()),
				Title:   extractChapterTitle(string(content)),
				Content: string(content),
			})
		}
	}

	fmt.Printf("🚀 批量模式: 发现 %d 个章节\n\n", len(chapters))

	results, err := pipeline.ProcessChaptersBatch(ctx, chapters)
	if err != nil {
		return err
	}

	// 汇总报告
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("                    批量分析汇总报告")
	fmt.Print("═══════════════════════════════════════════════════════════\n")

	totalIssues := 0
	for _, result := range results {
		totalIssues += len(result.ValidationIssues)
		fmt.Printf("[%s] %d 个问题\n", result.ChapterID, len(result.ValidationIssues))
	}

	fmt.Printf("\n总计: %d 个章节, %d 个问题\n", len(results), totalIssues)

	if outputPath != "" {
		if err := saveJSON(outputPath, results); err != nil {
			return fmt.Errorf("保存结果失败: %w", err)
		}
		fmt.Printf("\n💾 结果已保存到: %s\n", outputPath)
	}

	return nil
}

func initLLMForAIRPG(bookPath string) (llm.Client, *llm.Config, *models.ProjectLLM, error) {
	// 1. 从 novel.json 读取配置
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

	// 2. 加载 llm_config.json
	config, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载 LLM 配置失败: %w", err)
	}

	// 3. 获取 provider 和 model
	provider, model := config.GetActiveModel(projectLLM)
	if provider == nil {
		return nil, nil, nil, fmt.Errorf("provider 未找到: %s", projectLLM.Provider)
	}
	if model == nil {
		return nil, nil, nil, fmt.Errorf("model 未找到: %s", projectLLM.Model)
	}

	fmt.Printf("🔑 使用配置: provider=%s, model=%s\n", provider.Name, model.Name)

	// 4. 创建客户端
	client := llm.NewOpenAIClient(&llm.OpenAIConfig{
		APIKey:  provider.APIKey,
		BaseURL: provider.BaseURL,
		Model:   model.Name,
		Timeout: provider.Timeout,
	})

	return client, config, projectLLM, nil
}
