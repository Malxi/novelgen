package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"novelgen/internal/rpg/dsl"
)

// DebugLoadDSL 调试用函数
func DebugLoadDSL(bookName string) {
	rpgDir := filepath.Join("books", bookName, "story", "rpg")

	fmt.Printf("RPG Dir: %s\n", rpgDir)
	fmt.Printf("Current Dir: ")
	wd, _ := os.Getwd()
	fmt.Println(wd)

	// Check if dir exists
	info, err := os.Stat(rpgDir)
	if err != nil {
		fmt.Printf("Dir error: %v\n", err)
		return
	}
	fmt.Printf("Dir exists: %v, isDir: %v\n", info != nil, info.IsDir())

	// Try glob
	pattern := filepath.Join(rpgDir, "*.rpg")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("Glob error: %v\n", err)
		return
	}
	fmt.Printf("Glob pattern: %s\n", pattern)
	fmt.Printf("Matches: %v\n", matches)

	// Try to parse each file
	for _, file := range matches {
		fmt.Printf("\nParsing: %s\n", file)
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("  Read error: %v\n", err)
			continue
		}

		parser := dsl.NewParser(string(content))
		dslData, err := parser.Parse()
		if err != nil {
			fmt.Printf("  Parse error: %v\n", err)
			continue
		}

		fmt.Printf("  ✓ Parsed successfully!\n")
		fmt.Printf("    Title: %s\n", dslData.Metadata.Title)
	}
}
