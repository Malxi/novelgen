package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"novelgen/internal/llm"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

// loadProjectConfig 从项目目录加载 novel.json
func loadProjectConfig(projectPath string) (*models.ProjectConfig, error) {
	configPath := filepath.Join(projectPath, "novel.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config models.ProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	var (
		inputPath   = flag.String("i", "", "输入小说文件路径 (.md 或 .txt) (必需)")
		outputPath  = flag.String("o", "", "输出JSON文件路径 (可选)")
		verbose     = flag.Bool("v", false, "显示详细信息")
		provider    = flag.String("p", "", "LLM提供商 (覆盖配置文件)")
		model       = flag.String("m", "", "模型名称 (覆盖配置文件)")
		chunkSize   = flag.Int("chunk", 4000, "文本分块大小")
		projectPath = flag.String("project", "books/mine", "项目路径 (包含 novel.json)")
	)
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("错误: 请提供输入文件路径")
		fmt.Println("用法: extract-novel -i <小说文件路径> [选项]")
		fmt.Println("\n选项:")
		fmt.Println("  -i string      输入小说文件路径 (必需)")
		fmt.Println("  -o string      输出JSON文件路径 (可选)")
		fmt.Println("  -v             显示详细信息")
		fmt.Println("  -p string      LLM提供商 (覆盖配置文件)")
		fmt.Println("  -m string      模型名称 (覆盖配置文件)")
		fmt.Println("  -chunk int     文本分块大小 (默认4000)")
		fmt.Println("  -project string 项目路径 (默认: books/mine)")
		fmt.Println("\n示例:")
		fmt.Println("  extract-novel -i novel.md")
		fmt.Println("  extract-novel -i novel.md -o output.json -v")
		fmt.Println("  extract-novel -i novel.md -p ollama -m qwen3.5:4b")
		os.Exit(1)
	}

	// 读取小说文件
	fmt.Printf("📖 正在读取小说文件: %s\n", *inputPath)
	content, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Printf("❌ 错误: 无法读取文件: %v\n", err)
		os.Exit(1)
	}

	// 加载 LLM 配置
	fmt.Println("⚙️  正在加载LLM配置...")
	config, err := llm.LoadOrCreateConfig()
	if err != nil {
		fmt.Printf("❌ 配置错误: %v\n", err)
		fmt.Println("\n请确保已创建 llm_config.json 配置文件")
		os.Exit(1)
	}

	// 加载项目配置
	fmt.Printf("📁 正在加载项目配置: %s/novel.json\n", *projectPath)
	projectConfig, err := loadProjectConfig(*projectPath)
	if err != nil {
		fmt.Printf("⚠️  无法加载项目配置: %v\n", err)
		fmt.Println("将使用命令行参数或默认配置")
		projectConfig = &models.ProjectConfig{
			LLM: models.ProjectLLM{
				Provider: *provider,
				Model:    *model,
			},
		}
	} else {
		fmt.Printf("✅ 已加载项目: %s\n", projectConfig.Name)
		// 如果命令行没有指定，使用项目配置
		if *provider == "" {
			*provider = projectConfig.LLM.Provider
		}
		if *model == "" {
			*model = projectConfig.LLM.Model
		}
	}

	// 创建 projectLLM 配置
	projectLLM := &models.ProjectLLM{
		Provider: *provider,
		Model:    *model,
	}

	// 获取 active 配置
	providerConfig, modelConfig := config.GetActiveModel(projectLLM)
	if providerConfig == nil {
		fmt.Printf("❌ 错误: 未找到提供商配置\n")
		os.Exit(1)
	}
	if modelConfig == nil {
		fmt.Printf("❌ 错误: 未找到模型配置\n")
		os.Exit(1)
	}

	fmt.Printf("✅ 使用提供商: %s, 模型: %s\n", providerConfig.Name, modelConfig.Name)

	// 创建 client
	client := config.CreateClient(projectLLM)
	if client == nil {
		fmt.Printf("❌ 错误: 无法创建LLM客户端\n")
		os.Exit(1)
	}

	options := config.GetChatOptions(projectLLM)

	// 执行提取
	fmt.Println("🤖 LLM正在分析小说内容...")
	fmt.Printf("📊 文本大小: %d 字符, 分块大小: %d\n", len(content), *chunkSize)

	var data *rpg.NovelRPGData
	if len(content) > *chunkSize {
		fmt.Println("📄 文本较大，将分块处理...")
		data, err = rpg.ExtractLargeText(client, options, string(content), *chunkSize)
	} else {
		data, err = rpg.ExtractWithNovelgenClient(client, options, string(content))
	}

	if err != nil {
		fmt.Printf("❌ 提取失败: %v\n", err)
		os.Exit(1)
	}

	// 显示结果
	fmt.Println("\n============================================================")
	fmt.Println("📊 LLM提取结果")
	fmt.Println("============================================================")

	fmt.Printf("\n【角色】(%d)\n", len(data.Characters))
	for _, char := range data.Characters {
		fmt.Printf("  - %s [类型:%s]\n", char.Name, char.Type)
		if *verbose && char.Description != "" {
			fmt.Printf("    描述: %s\n", char.Description)
		}
	}

	fmt.Printf("\n【物品】(%d)\n", len(data.Items))
	for _, item := range data.Items {
		fmt.Printf("  - %s [%s]\n", item.Name, item.Type)
		if *verbose && item.Description != "" {
			fmt.Printf("    描述: %s\n", item.Description)
		}
	}

	fmt.Printf("\n【技能】(%d)\n", len(data.Skills))
	for _, skill := range data.Skills {
		fmt.Printf("  - %s [%s]\n", skill.Name, skill.Type)
		if *verbose && skill.Description != "" {
			fmt.Printf("    描述: %s\n", skill.Description)
		}
	}

	fmt.Printf("\n【地点】(%d)\n", len(data.Locations))
	for _, loc := range data.Locations {
		fmt.Printf("  - %s [%s]\n", loc.Name, loc.Type)
		if *verbose && loc.Description != "" {
			fmt.Printf("    描述: %s\n", loc.Description)
		}
	}

	fmt.Printf("\n【事件】(%d)\n", len(data.Events))
	for _, event := range data.Events {
		fmt.Printf("  - %s\n", event.Name)
	}

	fmt.Printf("\n【时间线】(%d 个时间点)\n", len(data.Timeline))
	for _, tl := range data.Timeline {
		if len(tl.Events) > 0 {
			fmt.Printf("  第%d天: %v @ %s\n", tl.Day, tl.Events, tl.Location)
		}
	}

	if len(data.ValidationIssues) > 0 {
		fmt.Printf("\n⚠️  检测到 %d 个问题:\n", len(data.ValidationIssues))
		for _, issue := range data.ValidationIssues {
			fmt.Printf("  [%s] %s\n", issue.Severity, issue.Message)
		}
	}

	// 保存到文件
	if *outputPath != "" {
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			fmt.Printf("❌ 序列化失败: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(*outputPath, jsonData, 0644); err != nil {
			fmt.Printf("❌ 保存失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n💾 结果已保存到: %s\n", *outputPath)
	}

	if *verbose {
		fmt.Println("\n============================================================")
		fmt.Println("📋 完整JSON输出")
		fmt.Println("============================================================")
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(jsonData))
	}

	fmt.Println("\n✅ LLM提取完成!")
	fmt.Printf("   角色: %d | 物品: %d | 技能: %d | 地点: %d | 事件: %d\n",
		len(data.Characters), len(data.Items), len(data.Skills), len(data.Locations), len(data.Events))
}
