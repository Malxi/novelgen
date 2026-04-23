package main

import (
	"flag"
	"fmt"
	"os"

	"novelgen/internal/rpg/benchmark"
)

func main() {
	var (
		bookPath   = flag.String("book", "books/mine", "小说目录路径")
		output     = flag.String("output", "", "输出报告文件路径")
		jsonOutput = flag.String("json", "", "输出 JSON 报告文件路径")
		help       = flag.Bool("h", false, "显示帮助")
	)

	flag.Parse()

	if *help {
		fmt.Println("小说 RPG 问题检测工具")
		fmt.Println()
		fmt.Println("用法: check_novel [选项]")
		fmt.Println()
		fmt.Println("选项:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("示例:")
		fmt.Println("  check_novel -book=books/mine")
		fmt.Println("  check_novel -book=books/mine -output=report.txt")
		fmt.Println("  check_novel -book=books/mine -json=report.json")
		os.Exit(0)
	}

	// 检查目录是否存在
	if _, err := os.Stat(*bookPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "错误: 目录不存在: %s\n", *bookPath)
		os.Exit(1)
	}

	// 执行检测
	if err := benchmark.CheckNovelCommand(*bookPath, *output, *jsonOutput); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
