package main

import (
	"flag"
	"fmt"
	"os"

	"novelgen/internal/rpg/benchmark"
)

func main() {
	var (
		bookPath = flag.String("book", "books/mine", "小说目录路径")
		output   = flag.String("output", "", "输出报告文件路径")
		help     = flag.Bool("h", false, "显示帮助")
	)

	flag.Parse()

	if *help {
		fmt.Println("小说 RPG 智能检测工具 v2")
		fmt.Println()
		fmt.Println("此版本针对特定小说设定进行优化：")
		fmt.Println("  - 识别复活是主角特权（有代价）")
		fmt.Println("  - 区分真正的问题和信息提示")
		fmt.Println("  - 减少误报")
		fmt.Println()
		fmt.Println("用法: check_novel_smart [选项]")
		fmt.Println()
		fmt.Println("选项:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if _, err := os.Stat(*bookPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 目录不存在: %s\n", *bookPath)
		os.Exit(1)
	}

	if err := benchmark.CheckNovelSmartCommand(*bookPath, *output); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
