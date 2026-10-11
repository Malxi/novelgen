package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"novelgen/internal/rpg"
)

func main() {
	var (
		inputPath  = flag.String("i", "", "输入大纲JSON文件路径")
		outputPath = flag.String("o", "", "输出RPG数据JSON文件路径")
		showHelp   = flag.Bool("h", false, "显示帮助信息")
	)

	flag.Parse()

	if *showHelp || *inputPath == "" {
		printHelp()
		return
	}

	// 检查输入文件是否存在
	if _, err := os.Stat(*inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 输入文件不存在: %s\n", *inputPath)
		os.Exit(1)
	}

	fmt.Printf("正在读取大纲文件: %s\n", *inputPath)

	// 创建故事世界
	storyWorld, err := rpg.NewStoryWorld(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 创建故事世界失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ 大纲转换成功")

	// 显示摘要
	summary := storyWorld.GetStorySummary()
	fmt.Println("\n========== 故事摘要 ==========")
	fmt.Printf("标题: %s\n", summary["title"])
	fmt.Printf("部分数: %d\n", summary["parts_count"])
	fmt.Printf("角色数: %d\n", summary["characters"])
	fmt.Printf("地点数: %d\n", summary["locations"])
	fmt.Printf("任务数: %d\n", summary["quests"])
	fmt.Printf("事件数: %d\n", summary["events"])
	fmt.Printf("\n主角: %s (等级 %d)\n", summary["player_name"], summary["player_level"])
	fmt.Printf("当前地图: %s\n", summary["current_map"])

	// 显示详细角色信息
	fmt.Println("\n========== 角色列表 ==========")
	characters := storyWorld.GameWorld.Characters.GetAllCharacters()
	for _, char := range characters {
		charType := ""
		switch char.Type {
		case rpg.CharacterTypePlayer:
			charType = "[主角]"
		case rpg.CharacterTypeNPC:
			charType = "[NPC]"
		case rpg.CharacterTypeEnemy:
			charType = "[敌人]"
		default:
			charType = "[其他]"
		}
		fmt.Printf("%s %s - HP:%d/%d MP:%d/%d 战力:%d\n",
			charType, char.Name,
			char.CurrentStats.HP, char.BaseStats.HP,
			char.CurrentStats.MP, char.BaseStats.MP,
			char.CalculateBattlePower().Total)
	}

	// 显示地图信息
	fmt.Println("\n========== 地图列表 ==========")
	maps := storyWorld.GameWorld.Maps.GetAllMaps()
	for _, m := range maps {
		fmt.Printf("[%s] %s - %s\n", m.Type, m.Name, m.Description)
	}

	// 显示任务信息
	fmt.Println("\n========== 任务列表 ==========")
	quests := storyWorld.GameWorld.Quests.GetAllQuests()
	for _, quest := range quests {
		questType := ""
		switch quest.Type {
		case rpg.QuestTypeMain:
			questType = "[主线]"
		case rpg.QuestTypeSide:
			questType = "[支线]"
		default:
			questType = "[其他]"
		}
		fmt.Printf("%s %s - %s (目标:%d)\n", questType, quest.Name, quest.Description, len(quest.Objectives))
	}

	// 导出JSON
	exportJSON := storyWorld.ExportToJSON()

	// 确定输出路径
	if *outputPath == "" {
		// 默认输出到输入文件所在目录
		baseName := filepath.Base(*inputPath)
		ext := filepath.Ext(baseName)
		nameWithoutExt := baseName[:len(baseName)-len(ext)]
		*outputPath = filepath.Join(filepath.Dir(*inputPath), nameWithoutExt+"_rpg.json")
	}

	// 写入输出文件
	if err := ioutil.WriteFile(*outputPath, []byte(exportJSON), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入输出文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ RPG数据已导出到: %s\n", *outputPath)
	fmt.Printf("  文件大小: %d 字节\n", len(exportJSON))
}

func printHelp() {
	fmt.Println("故事大纲转RPG数据工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  story2rpg -i <输入文件> [-o <输出文件>]")
	fmt.Println()
	fmt.Println("参数:")
	fmt.Println("  -i string    输入大纲JSON文件路径 (必需)")
	fmt.Println("  -o string    输出RPG数据JSON文件路径 (可选，默认在输入文件同目录)")
	fmt.Println("  -h           显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  story2rpg -i outline.json")
	fmt.Println("  story2rpg -i outline.json -o rpg_data.json")
}
