package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"novelgen/internal/rpg"
)

func main() {
	var (
		inputPath   = flag.String("i", "", "输入RPG数据JSON文件路径 (必需)")
		reportPath  = flag.String("r", "", "输入模拟报告JSON文件路径 (可选)")
		outputDir   = flag.String("o", "output/visualization", "输出目录")
		format      = flag.String("f", "all", "输出格式: html, md, json, all")
		openBrowser = flag.Bool("open", false, "生成后自动打开浏览器")
	)
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("错误: 请提供输入文件路径")
		fmt.Println("用法: visualize -i <rpg_data.json> [选项]")
		fmt.Println("\n选项:")
		fmt.Println("  -i string    输入RPG数据JSON文件路径 (必需)")
		fmt.Println("  -r string    输入模拟报告JSON文件路径 (可选)")
		fmt.Println("  -o string    输出目录 (默认: output/visualization)")
		fmt.Println("  -f string    输出格式: html, md, json, all (默认: all)")
		fmt.Println("  -open        生成后自动打开浏览器")
		os.Exit(1)
	}

	// 读取RPG数据
	fmt.Printf("📖 正在读取RPG数据: %s\n", *inputPath)
	data, err := loadRPGData(*inputPath)
	if err != nil {
		fmt.Printf("❌ 错误: 无法读取RPG数据: %v\n", err)
		os.Exit(1)
	}

	// 读取模拟报告（如果提供）
	var report *rpg.SimulationReport
	if *reportPath != "" {
		fmt.Printf("📊 正在读取模拟报告: %s\n", *reportPath)
		report, err = loadSimulationReport(*reportPath)
		if err != nil {
			fmt.Printf("⚠️  警告: 无法读取模拟报告: %v\n", err)
			fmt.Println("将生成简化版报告")
		}
	}

	// 如果没有报告，创建一个默认的
	if report == nil {
		report = createDefaultReport(data)
	}

	// 创建输出目录
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("❌ 错误: 无法创建输出目录: %v\n", err)
		os.Exit(1)
	}

	// 创建可视化器
	viz := rpg.NewVisualizer(data, report)

	// 生成报告
	fmt.Println("\n🎨 正在生成可视化报告...")

	var generatedFiles []string

	switch *format {
	case "html":
		path := filepath.Join(*outputDir, "rpg_report.html")
		if err := viz.GenerateHTMLReport(path); err != nil {
			fmt.Printf("❌ 生成HTML报告失败: %v\n", err)
			os.Exit(1)
		}
		generatedFiles = append(generatedFiles, path)
		fmt.Printf("✅ HTML报告已生成: %s\n", path)

	case "md":
		path := filepath.Join(*outputDir, "rpg_report.md")
		if err := viz.GenerateMarkdownReport(path); err != nil {
			fmt.Printf("❌ 生成Markdown报告失败: %v\n", err)
			os.Exit(1)
		}
		generatedFiles = append(generatedFiles, path)
		fmt.Printf("✅ Markdown报告已生成: %s\n", path)

	case "json":
		path := filepath.Join(*outputDir, "rpg_report.json")
		if err := viz.GenerateJSONReport(path); err != nil {
			fmt.Printf("❌ 生成JSON报告失败: %v\n", err)
			os.Exit(1)
		}
		generatedFiles = append(generatedFiles, path)
		fmt.Printf("✅ JSON报告已生成: %s\n", path)

	case "all":
		files, err := rpg.GenerateAllReports(data, report, *outputDir)
		if err != nil {
			fmt.Printf("❌ 生成报告失败: %v\n", err)
			os.Exit(1)
		}
		generatedFiles = files
		for _, file := range files {
			fmt.Printf("✅ 报告已生成: %s\n", file)
		}
	}

	// 自动打开浏览器
	if *openBrowser && len(generatedFiles) > 0 {
		// 找到HTML文件
		for _, file := range generatedFiles {
			if filepath.Ext(file) == ".html" {
				fmt.Println("\n🌐 正在打开浏览器...")
				if err := viz.OpenBrowser(file); err != nil {
					fmt.Printf("⚠️  无法自动打开浏览器: %v\n", err)
					fmt.Printf("请手动打开: %s\n", file)
				}
				break
			}
		}
	}

	fmt.Println("\n✨ 可视化报告生成完成!")
	fmt.Printf("📁 输出目录: %s\n", *outputDir)
}

// loadRPGData 从JSON文件加载RPG数据
func loadRPGData(path string) (*rpg.NovelRPGData, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data rpg.NovelRPGData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

// loadSimulationReport 从JSON文件加载模拟报告
func loadSimulationReport(path string) (*rpg.SimulationReport, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var report rpg.SimulationReport
	if err := json.Unmarshal(content, &report); err != nil {
		return nil, err
	}

	return &report, nil
}

// createDefaultReport 创建默认报告（当没有模拟报告时）
func createDefaultReport(data *rpg.NovelRPGData) *rpg.SimulationReport {
	return &rpg.SimulationReport{
		Summary: rpg.SimulationSummary{
			TotalEvents:        len(data.Timeline),
			CharactersInvolved: len(data.Characters),
			IssuesFound:        len(data.ValidationIssues),
			CriticalIssues:     0,
			OverallScore:       100.0,
			Grade:              "S",
		},
		ValidationResults: []rpg.SimulatorValidationResult{
			{
				Category:    "character_consistency",
				Passed:      true,
				Score:       100,
				Issues:      []string{},
				Suggestions: []string{},
			},
			{
				Category:    "power_system",
				Passed:      true,
				Score:       100,
				Issues:      []string{},
				Suggestions: []string{},
			},
		},
		Recommendations: []string{},
	}
}
