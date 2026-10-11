package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"novelgen/internal/rpg"
)

func main() {
	var (
		projectPath = flag.String("p", ".", "novelgen项目路径")
		bookName    = flag.String("b", "", "书籍名称 (必需)")
		outputPath  = flag.String("o", "", "输出RPG数据文件路径")
		simulate    = flag.String("s", "", "要推演的章节ID (可选)")
		showReport  = flag.Bool("r", true, "显示推演报告")
		showHelp    = flag.Bool("h", false, "显示帮助信息")
	)

	flag.Parse()

	if *showHelp || *bookName == "" {
		printHelp()
		return
	}

	// 检查项目路径
	if _, err := os.Stat(*projectPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 项目路径不存在: %s\n", *projectPath)
		os.Exit(1)
	}

	fmt.Printf("正在加载 novelgen 项目: %s/%s\n", *projectPath, *bookName)

	// 加载 novelgen 项目
	project, err := rpg.LoadNovelgenProject(*projectPath, *bookName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 加载项目失败: %v\n", err)
		os.Exit(1)
	}

	// 显示项目摘要
	summary := project.GetProjectSummary()
	fmt.Println("\n========== 项目摘要 ==========")
	fmt.Printf("书籍: %s\n", summary["book_name"])
	fmt.Printf("角色: %d\n", summary["characters"])
	fmt.Printf("物品: %d\n", summary["items"])
	fmt.Printf("地点: %d\n", summary["locations"])
	fmt.Printf("部分: %d\n", summary["parts"])

	// 转换为 RPG 世界
	fmt.Println("\n正在转换为 RPG 世界...")
	world, err := project.ConvertToRPG()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 转换失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ RPG 世界创建成功")

	// 显示角色信息
	fmt.Println("\n========== 角色列表 ==========")
	characters := world.Characters.GetAllCharacters()
	for _, char := range characters {
		charType := ""
		switch char.Type {
		case rpg.CharacterTypePlayer:
			charType = "[主角]"
		case rpg.CharacterTypeNPC:
			charType = "[NPC]"
		case rpg.CharacterTypeEnemy:
			charType = "[敌人]"
		}
		fmt.Printf("%s %s - HP:%d/%d MP:%d/%d 战力:%d\n",
			charType, char.Name,
			char.CurrentStats.HP, char.BaseStats.HP,
			char.CurrentStats.MP, char.BaseStats.MP,
			char.CalculateBattlePower().Total)
	}

	// 显示地图信息
	fmt.Println("\n========== 地图列表 ==========")
	maps := world.Maps.GetAllMaps()
	for _, m := range maps {
		fmt.Printf("[%s] %s\n", m.Type, m.Name)
	}

	// 显示任务信息
	fmt.Println("\n========== 任务列表 ==========")
	quests := world.Quests.GetAllQuests()
	for _, quest := range quests {
		fmt.Printf("[%s] %s - 目标:%d\n", quest.Type, quest.Name, len(quest.Objectives))
	}

	// 如果指定了章节，进行推演
	if *simulate != "" {
		fmt.Printf("\n========== 推演章节: %s ==========\n", *simulate)
		engine := rpg.NewSimulationEngine(world)
		result, err := engine.SimulateChapter(*simulate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "推演出错: %v\n", err)
		} else {
			fmt.Printf("章节: %s\n", result.ChapterName)
			fmt.Printf("步骤数: %d\n", len(result.Steps))
			fmt.Printf("成功: %v\n", result.Success)

			if *showReport {
				fmt.Println("\n推演过程:")
				for _, step := range result.Steps {
					fmt.Printf("\n[%s] %s\n", step.Type, step.Description)
					for _, res := range step.Results {
						if res.Message != "" {
							fmt.Printf("  - %s\n", res.Message)
						}
					}
				}
			}
		}
	}

	// 导出 RPG 数据
	if *outputPath == "" {
		*outputPath = filepath.Join(*projectPath, "books", *bookName, "rpg_data.json")
	}

	fmt.Printf("\n正在导出 RPG 数据到: %s\n", *outputPath)
	if err := project.ExportRPGData(*outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 导出失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ 导出成功")
}

func printHelp() {
	fmt.Println("novelgen-rpg 集成工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  novelgen-rpg -b <书籍名称> [选项]")
	fmt.Println()
	fmt.Println("参数:")
	fmt.Println("  -b string    书籍名称 (必需)")
	fmt.Println("  -p string    项目路径 (默认: 当前目录)")
	fmt.Println("  -o string    输出文件路径 (默认: books/<book_name>/rpg_data.json)")
	fmt.Println("  -s string    要推演的章节ID (可选)")
	fmt.Println("  -r bool      显示推演报告 (默认: true)")
	fmt.Println("  -h           显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  novelgen-rpg -b mine")
	fmt.Println("  novelgen-rpg -b mine -s P1-V1-C1")
	fmt.Println("  novelgen-rpg -p /path/to/project -b mine -o output.json")
}
