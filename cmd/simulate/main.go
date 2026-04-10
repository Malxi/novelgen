package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"novelgen/internal/rpg"
)

func main() {
	var (
		outlinePath = flag.String("i", "", "输入大纲JSON文件路径")
		chapterID   = flag.String("c", "", "要推演的章节ID (如: P1-V1-C1)")
		outputPath  = flag.String("o", "", "输出推演结果JSON文件路径")
		showReport  = flag.Bool("r", true, "显示推演报告")
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

	// 创建故事世界
	storyWorld, err := rpg.NewStoryWorld(*outlinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 创建故事世界失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ 大纲转换成功")

	// 创建推演引擎
	engine := rpg.NewSimulationEngine(storyWorld.GameWorld)

	var result *rpg.SimulationResult

	// 推演单个章节或全部章节
	if *chapterID != "" {
		fmt.Printf("\n正在推演章节: %s\n", *chapterID)
		result, err = engine.SimulateChapter(*chapterID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 章节推演出错: %v\n", err)
			os.Exit(1)
		}
	} else {
		// 推演所有章节
		fmt.Println("\n正在推演所有章节...")
		quests := storyWorld.GameWorld.Quests.GetAllQuests()
		questIDs := make([]string, 0, len(quests))
		for _, quest := range quests {
			// 提取章节ID (去掉 "quest_" 前缀)
			chapterID := strings.TrimPrefix(quest.ID, "quest_")
			questIDs = append(questIDs, chapterID)
		}

		storyResult := engine.SimulateStory(questIDs)
		fmt.Printf("推演完成! 共推演 %d 个章节，总步骤数: %d\n", 
			len(storyResult.Chapters), storyResult.TotalSteps)
		
		if len(storyResult.Errors) > 0 {
			fmt.Printf("错误: %d 个\n", len(storyResult.Errors))
			for _, err := range storyResult.Errors {
				fmt.Printf("  - %s\n", err)
			}
		}
	}

	// 显示推演报告
	if *showReport {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println(engine.GetSimulationReport())
	}

	// 导出推演数据
	exportJSON := engine.ExportSimulation()

	// 确定输出路径
	if *outputPath == "" {
		if *chapterID != "" {
			*outputPath = fmt.Sprintf("simulation_%s.json", *chapterID)
		} else {
			*outputPath = "simulation_full.json"
		}
	}

	// 写入输出文件
	if err := ioutil.WriteFile(*outputPath, []byte(exportJSON), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入输出文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ 推演数据已导出到: %s\n", *outputPath)
	fmt.Printf("  文件大小: %d 字节\n", len(exportJSON))
}

func printHelp() {
	fmt.Println("剧情推演工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  simulate -i <大纲文件> [-c <章节ID>] [-o <输出文件>]")
	fmt.Println()
	fmt.Println("参数:")
	fmt.Println("  -i string    输入大纲JSON文件路径 (必需)")
	fmt.Println("  -c string    要推演的章节ID (可选，默认推演所有章节)")
	fmt.Println("  -o string    输出推演结果JSON文件路径 (可选)")
	fmt.Println("  -r bool      显示推演报告 (默认: true)")
	fmt.Println("  -h           显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  simulate -i outline.json")
	fmt.Println("  simulate -i outline.json -c P1-V1-C1")
	fmt.Println("  simulate -i outline.json -c P1-V1-C1 -o result.json")
}
