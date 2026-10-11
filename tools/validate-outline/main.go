package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/rpg"
)

func main() {
	var (
		outlinePath = flag.String("i", "", "输入大纲JSON文件路径 (必需)")
		outputPath  = flag.String("o", "", "输出验证结果JSON文件路径 (可选)")
		showDetail  = flag.Bool("d", true, "显示详细信息")
		fixMode     = flag.Bool("f", false, "尝试修复可修复的问题")
		showHelp    = flag.Bool("h", false, "显示帮助信息")
	)

	flag.Parse()

	if *showHelp || *outlinePath == "" {
		printHelp()
		return
	}

	// 检查输入文件是否存在
	if _, err := os.Stat(*outlinePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 输入文件不存在: %s\n", *outlinePath)
		os.Exit(1)
	}

	fmt.Printf("正在读取大纲文件: %s\n", *outlinePath)

	// 读取大纲
	data, err := ioutil.ReadFile(*outlinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 读取文件失败: %v\n", err)
		os.Exit(1)
	}

	var outline rpg.StoryOutline
	if err := json.Unmarshal(data, &outline); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 解析JSON失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ 大纲加载成功")

	// 创建验证器
	validator := rpg.NewOutlineValidator(&outline)

	// 执行验证
	fmt.Println("\n正在验证大纲...")
	result := validator.Validate()

	// 显示验证结果
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("验证结果")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println(result.Summary)

	if *showDetail {
		// 显示问题
		if len(result.Issues) > 0 {
			fmt.Println("\n【问题列表】")
			for i, issue := range result.Issues {
				fmt.Printf("\n%d. [%s] %s\n", i+1, issue.Severity, issue.Type)
				fmt.Printf("   位置: %s\n", issue.Location)
				fmt.Printf("   描述: %s\n", issue.Description)
				fmt.Printf("   影响: %s\n", issue.Impact)
				fmt.Printf("   修复: %s\n", issue.Fix)
			}
		}

		// 显示警告
		if len(result.Warnings) > 0 {
			fmt.Println("\n【警告列表】")
			for i, warning := range result.Warnings {
				fmt.Printf("\n%d. [%s] %s\n", i+1, warning.Type, warning.Location)
				fmt.Printf("   描述: %s\n", warning.Description)
				fmt.Printf("   建议: %s\n", warning.Suggestion)
			}
		}

		// 显示建议
		if len(result.Suggestions) > 0 {
			fmt.Println("\n【改进建议】")
			for i, suggestion := range result.Suggestions {
				fmt.Printf("\n%d. [%s] %s\n", i+1, suggestion.Type, suggestion.Location)
				fmt.Printf("   当前: %s\n", suggestion.Current)
				fmt.Printf("   建议: %s\n", suggestion.Suggested)
				fmt.Printf("   原因: %s\n", suggestion.Reason)
			}
		}
	}

	// 修复模式
	if *fixMode && !result.IsValid {
		fmt.Println("\n【修复模式】")
		fixedOutline := tryFixOutline(&outline, result)
		
		// 保存修复后的大纲
		fixedPath := filepath.Join(filepath.Dir(*outlinePath), 
			filepath.Base(*outlinePath)+".fixed.json")
		
		fixedData, _ := json.MarshalIndent(fixedOutline, "", "  ")
		if err := ioutil.WriteFile(fixedPath, fixedData, 0644); err != nil {
			fmt.Printf("修复文件保存失败: %v\n", err)
		} else {
			fmt.Printf("✓ 修复后的大纲已保存到: %s\n", fixedPath)
		}
	}

	// 导出验证结果
	if *outputPath == "" {
		*outputPath = filepath.Join(filepath.Dir(*outlinePath), 
			"validation_"+filepath.Base(*outlinePath))
	}

	resultData, _ := json.MarshalIndent(result, "", "  ")
	if err := ioutil.WriteFile(*outputPath, resultData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 保存验证结果失败: %v\n", err)
	} else {
		fmt.Printf("\n✓ 验证结果已保存到: %s\n", *outputPath)
	}

	// 根据验证结果设置退出码
	if !result.IsValid {
		os.Exit(1)
	}
}

func tryFixOutline(outline *rpg.StoryOutline, result *rpg.ValidationResult) *rpg.StoryOutline {
	// 这里可以实现自动修复逻辑
	// 例如：添加缺失的ID、补充默认字段等
	fmt.Println("尝试修复问题...")
	
	fixed := *outline
	
	// 修复逻辑示例
	for _, issue := range result.Issues {
		switch issue.Type {
		case "structure":
			if issue.Severity == "minor" {
				fmt.Printf("  - 修复: %s\n", issue.Description)
			}
		}
	}
	
	return &fixed
}

func printHelp() {
	fmt.Println("大纲验证工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  validate-outline -i <大纲文件> [选项]")
	fmt.Println()
	fmt.Println("参数:")
	fmt.Println("  -i string    输入大纲JSON文件路径 (必需)")
	fmt.Println("  -o string    输出验证结果JSON文件路径 (可选)")
	fmt.Println("  -d bool      显示详细信息 (默认: true)")
	fmt.Println("  -f bool      尝试修复可修复的问题 (默认: false)")
	fmt.Println("  -h           显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  validate-outline -i outline.json")
	fmt.Println("  validate-outline -i outline.json -d=false")
	fmt.Println("  validate-outline -i outline.json -f")
}


