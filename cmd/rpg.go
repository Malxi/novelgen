package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"novelgen/internal/logger"
	"novelgen/internal/rpg"

	"github.com/spf13/cobra"
)

var (
	rpgOutputPath string
	rpgSimulate   string
	rpgShowReport bool
)

var rpgCmd = &cobra.Command{
	Use:   "rpg",
	Short: "Generate RPG data from novel elements",
	Long: `Generate RPG game data from the novel's story elements.

This command converts the novel's characters, items, locations, and outline 
into RPG game data that can be used for game development or story simulation.

The RPG data includes:
  - Characters: RPG stats, skills, classes, equipment
  - Items: Consumables, equipment, materials with effects
  - Maps: Locations converted to game maps with connections
  - Quests: Story chapters converted to game quests
  - Events: Story events converted to game events

Generated RPG data is saved to the book's directory as rpg_data.json.`,
}

var rpgGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate RPG data from current novel",
	Long: `Generate RPG data from the current novel project.

This command loads the novel's craft data (characters, items, locations) 
and outline, then converts them into a complete RPG game world.

Examples:
  # Generate RPG data for current book
  novelgen rpg gen

  # Generate with custom output path
  novelgen rpg gen -o books/mybook/rpg_data.json

  # Generate and simulate a specific chapter
  novelgen rpg gen -s P1-V1-C1`,
	RunE: runRPGGen,
}

var rpgSimulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Simulate RPG gameplay for a chapter or volume",
	Long: `Simulate RPG gameplay for a specific chapter or volume.

This command runs a simulation of the RPG gameplay based on the chapter's
story events, showing how the game mechanics would play out.

Examples:
  # Simulate chapter P1-V1-C1
  novelgen rpg simulate P1-V1-C1

  # Simulate volume P1-V1
  novelgen rpg simulate P1-V1

  # Simulate without detailed report
  novelgen rpg simulate P1-V1-C1 --report=false`,
	Args: cobra.ExactArgs(1),
	RunE: runRPGSimulate,
}

var rpgExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export RPG data to novelgen format",
	Long: `Export RPG world data back to novelgen format.

This command converts the RPG game world data (characters, items, locations)
back to novelgen's standard format, allowing AI-generated RPG content
to be applied directly to the novel project.

Examples:
  # Export RPG data to novelgen format
  novelgen rpg export

  # Export to specific directory
  novelgen rpg export -o books/mybook/craft/`,
	RunE: runRPGExport,
}

func init() {
	rpgCmd.AddCommand(rpgGenCmd)
	rpgCmd.AddCommand(rpgSimulateCmd)
	rpgCmd.AddCommand(rpgExportCmd)

	rpgGenCmd.Flags().StringVarP(&rpgOutputPath, "output", "o", "", "Output file path (default: books/<book_name>/rpg_data.json)")
	rpgGenCmd.Flags().StringVarP(&rpgSimulate, "simulate", "s", "", "Chapter ID to simulate after generation (e.g., P1-V1-C1)")
	rpgGenCmd.Flags().BoolVarP(&rpgShowReport, "report", "r", true, "Show simulation report")

	rpgSimulateCmd.Flags().BoolVarP(&rpgShowReport, "report", "r", true, "Show detailed simulation report")

	rpgExportCmd.Flags().StringVarP(&rpgOutputPath, "output", "o", "", "Output directory (default: books/<book_name>/craft/)")

	// Register rpg command
	RegisterCommand(func() *cobra.Command {
		return rpgCmd
	})
}

func runRPGGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	bookName := config.Name
	if bookName == "" {
		return fmt.Errorf("book name not set in project config")
	}

	// Get project root - findProjectRoot returns the book directory
	// We need the parent of books directory as project root
	bookDir, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Get the book folder name (e.g., "mine")
	bookFolder := filepath.Base(bookDir)
	// Get the project root (parent of books directory)
	projectRoot := filepath.Dir(filepath.Dir(bookDir))

	log.Info("Loading novelgen project: %s (book: %s)", bookName, bookFolder)

	// Load novelgen project
	project, err := rpg.LoadNovelgenProject(projectRoot, bookFolder)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Display project summary
	summary := project.GetProjectSummary()
	fmt.Println("\n========== Project Summary ==========")
	fmt.Printf("Book: %s\n", summary["book_name"])
	fmt.Printf("Characters: %d\n", summary["characters"])
	fmt.Printf("Items: %d\n", summary["items"])
	fmt.Printf("Locations: %d\n", summary["locations"])
	fmt.Printf("Parts: %d\n", summary["parts"])

	// Convert to RPG world
	fmt.Println("\nConverting to RPG world...")
	world, err := project.ConvertToRPG()
	if err != nil {
		return fmt.Errorf("failed to convert to RPG: %w", err)
	}

	fmt.Println("✓ RPG world created successfully")

	// Display character info
	fmt.Println("\n========== Characters ==========")
	characters := world.Characters.GetAllCharacters()
	for _, char := range characters {
		charType := ""
		switch char.Type {
		case rpg.CharacterTypePlayer:
			charType = "[Player]"
		case rpg.CharacterTypeNPC:
			charType = "[NPC]"
		case rpg.CharacterTypeEnemy:
			charType = "[Enemy]"
		}
		fmt.Printf("%s %s - HP:%d/%d MP:%d/%d Power:%d\n",
			charType, char.Name,
			char.CurrentStats.HP, char.BaseStats.HP,
			char.CurrentStats.MP, char.BaseStats.MP,
			char.CalculateBattlePower().Total)
	}

	// Display map info
	fmt.Println("\n========== Maps ==========")
	maps := world.Maps.GetAllMaps()
	for _, m := range maps {
		fmt.Printf("[%s] %s\n", m.Type, m.Name)
	}

	// Display quest info
	fmt.Println("\n========== Quests ==========")
	quests := world.Quests.GetAllQuests()
	for _, quest := range quests {
		fmt.Printf("[%s] %s - Objectives:%d\n", quest.Type, quest.Name, len(quest.Objectives))
	}

	// Simulate chapter if specified
	if rpgSimulate != "" {
		fmt.Printf("\n========== Simulating: %s ==========\n", rpgSimulate)
		engine := rpg.NewSimulationEngine(world)
		result, err := engine.SimulateChapter(rpgSimulate)
		if err != nil {
			log.Error("Simulation error: %v", err)
		} else {
			fmt.Printf("Chapter: %s\n", result.ChapterName)
			fmt.Printf("Steps: %d\n", len(result.Steps))
			fmt.Printf("Success: %v\n", result.Success)

			if rpgShowReport {
				fmt.Println("\nSimulation Steps:")
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

	// Determine output path - default to current book directory
	if rpgOutputPath == "" {
		rpgOutputPath = filepath.Join(bookDir, "rpg_data.json")
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(rpgOutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Export RPG data
	fmt.Printf("\nExporting RPG data to: %s\n", rpgOutputPath)
	if err := project.ExportRPGData(rpgOutputPath); err != nil {
		return fmt.Errorf("failed to export RPG data: %w", err)
	}

	// Also export to project output directory for convenience
	outputRPGPath := filepath.Join(projectRoot, "output", "rpg_data.json")
	if err := os.MkdirAll(filepath.Dir(outputRPGPath), 0755); err == nil {
		_ = project.ExportRPGData(outputRPGPath)
	}

	fmt.Println("✓ RPG data exported successfully")
	return nil
}

func runRPGSimulate(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	targetID := args[0]

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	bookName := config.Name
	if bookName == "" {
		return fmt.Errorf("book name not set in project config")
	}

	// Get project root - findProjectRoot returns the book directory
	bookDir, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Get the book folder name (e.g., "mine")
	bookFolder := filepath.Base(bookDir)
	// Get the project root (parent of books directory)
	projectRoot := filepath.Dir(filepath.Dir(bookDir))

	log.Info("Loading RPG data for book: %s", bookName)

	// Try to load existing RPG data from current book directory
	rpgDataPath := filepath.Join(bookDir, "rpg_data.json")

	// If not found, try to generate on the fly
	var world *rpg.GameWorld

	if _, err := os.Stat(rpgDataPath); err == nil {
		// Load existing RPG data
		data, err := os.ReadFile(rpgDataPath)
		if err != nil {
			return fmt.Errorf("failed to read RPG data: %w", err)
		}

		// Generate RPG data on the fly from project (simpler and more reliable)
		fmt.Println("Loading RPG data from project...")
		project, err := rpg.LoadNovelgenProject(projectRoot, bookFolder)
		if err != nil {
			return fmt.Errorf("failed to load project: %w", err)
		}

		world, err = project.ConvertToRPG()
		if err != nil {
			return fmt.Errorf("failed to convert to RPG: %w", err)
		}

		// Suppress unused variable warning
		_ = data
	} else {
		// Generate RPG data on the fly
		fmt.Println("RPG data not found, generating from novel elements...")
		project, err := rpg.LoadNovelgenProject(projectRoot, bookFolder)
		if err != nil {
			return fmt.Errorf("failed to load project: %w", err)
		}

		world, err = project.ConvertToRPG()
		if err != nil {
			return fmt.Errorf("failed to convert to RPG: %w", err)
		}
	}

	// 判断是章节还是卷
	engine := rpg.NewEnhancedSimulationEngine(world)

	if isVolumeID(targetID) {
		// 模拟整个卷
		return simulateVolume(engine, targetID, bookDir)
	}

	// 模拟单个章节
	return simulateChapter(engine, targetID, bookDir)
}

func isVolumeID(id string) bool {
	// 卷ID格式: P1-V1 (不包含 -C)
	return len(id) > 0 && !containsChapter(id)
}

func containsChapter(id string) bool {
	for i := 0; i < len(id)-1; i++ {
		if id[i] == '-' && id[i+1] == 'C' {
			return true
		}
	}
	return false
}

func simulateChapter(engine *rpg.EnhancedSimulationEngine, chapterID string, bookDir string) error {
	fmt.Printf("\n========== Simulating Chapter: %s ==========\n", chapterID)
	result, err := engine.SimulateChapterEnhanced(chapterID)
	if err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	// Display results
	fmt.Printf("Chapter: %s\n", result.ChapterName)
	fmt.Printf("Total Steps: %d\n", len(result.Steps))
	fmt.Printf("Success: %v\n", result.Success)

	if rpgShowReport {
		// 生成并显示增强版文本报告
		textReport := engine.GenerateTextReport(result)
		fmt.Println(textReport)
	}

	// 保存报告到文件
	reportPath := filepath.Join(bookDir, "simulation_reports", fmt.Sprintf("simulation_%s.json", chapterID))
	if err := engine.ExportEnhancedReport(result, reportPath); err != nil {
		fmt.Printf("Warning: Failed to save report: %v\n", err)
	} else {
		fmt.Printf("\n📄 详细报告已保存到: %s\n", reportPath)
	}

	return nil
}

func simulateVolume(engine *rpg.EnhancedSimulationEngine, volumeID string, bookDir string) error {
	fmt.Printf("\n========== Simulating Volume: %s ==========\n", volumeID)

	// 获取该卷下的所有章节
	chapterIDs, err := engine.GetChaptersInVolume(volumeID)
	if err != nil {
		return fmt.Errorf("failed to get chapters in volume: %w", err)
	}

	if len(chapterIDs) == 0 {
		return fmt.Errorf("volume %s has no chapters", volumeID)
	}

	fmt.Printf("Found %d chapters in volume\n\n", len(chapterIDs))

	// 累计统计
	totalSteps := 0
	totalBattles := 0
	totalDamageDealt := 0
	totalDamageTaken := 0
	totalExpGained := 0
	finalLevel := 1

	// 收集章节摘要
	chapterSummaries := make([]rpg.ChapterSummary, 0, len(chapterIDs))

	// 模拟每个章节
	for i, chapterID := range chapterIDs {
		fmt.Printf("\n--- Chapter %d/%d: %s ---\n", i+1, len(chapterIDs), chapterID)

		result, err := engine.SimulateChapterEnhanced(chapterID)
		if err != nil {
			fmt.Printf("  ⚠️ Failed to simulate %s: %v\n", chapterID, err)
			continue
		}

		// 累计统计
		totalSteps += len(result.Steps)
		totalBattles += len(result.BattleReports)
		totalExpGained += result.TotalExpGained
		finalLevel = result.FinalLevel

		for _, battle := range result.BattleReports {
			totalDamageDealt += battle.DamageDealt
			totalDamageTaken += battle.DamageTaken
		}

		// 收集章节摘要
		chapterSummaries = append(chapterSummaries, rpg.ChapterSummary{
			ChapterID:   chapterID,
			ChapterName: result.ChapterName,
			Steps:       len(result.Steps),
			Battles:     len(result.BattleReports),
			ExpGained:   result.TotalExpGained,
			Success:     result.Success,
		})

		// 显示简要结果
		fmt.Printf("  ✅ %s | 步骤: %d | 战斗: %d | 经验: +%d\n",
			result.ChapterName, len(result.Steps), len(result.BattleReports), result.TotalExpGained)
	}

	// 显示卷总结
	fmt.Printf("\n╔══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                    卷模拟总结                                  ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════╝\n\n")

	fmt.Printf("📖 卷: %s\n", volumeID)
	fmt.Printf("📊 章节数: %d\n", len(chapterIDs))
	fmt.Printf("📦 总步骤: %d\n", totalSteps)
	fmt.Printf("⚔️ 总战斗: %d\n", totalBattles)
	fmt.Printf("✨ 总经验: +%d\n", totalExpGained)
	fmt.Printf("📈 最终等级: %d\n", finalLevel)

	if totalBattles > 0 {
		fmt.Printf("💥 总伤害输出: %d\n", totalDamageDealt)
		fmt.Printf("🛡️ 总承受伤害: %d\n", totalDamageTaken)
	}

	// 保存卷报告
	reportPath := filepath.Join(bookDir, "simulation_reports", fmt.Sprintf("simulation_%s.json", volumeID))
	volumeReport := &rpg.VolumeSimulationReport{
		VolumeID:     volumeID,
		ChapterIDs:   chapterIDs,
		TotalSteps:   totalSteps,
		TotalBattles: totalBattles,
		TotalExp:     totalExpGained,
		FinalLevel:   finalLevel,
		Chapters:     chapterSummaries,
	}
	if err := engine.ExportVolumeReport(volumeReport, reportPath); err != nil {
		fmt.Printf("Warning: Failed to save volume report: %v\n", err)
	} else {
		fmt.Printf("\n📄 卷详细报告已保存到: %s\n", reportPath)
	}

	return nil
}

func runRPGExport(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	bookName := config.Name
	if bookName == "" {
		return fmt.Errorf("book name not set in project config")
	}

	// Get project root
	bookDir, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	bookFolder := filepath.Base(bookDir)
	projectRoot := filepath.Dir(filepath.Dir(bookDir))

	log.Info("Loading RPG data for export: %s", bookName)

	// Load or generate RPG world
	project, err := rpg.LoadNovelgenProject(projectRoot, bookFolder)
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	world, err := project.ConvertToRPG()
	if err != nil {
		return fmt.Errorf("failed to convert to RPG: %w", err)
	}

	// 使用共享模型转换器导出
	converter := rpg.NewSharedModelConverter(world)
	export := converter.ExportAllToNovelgen()

	// 确定输出路径
	outputDir := rpgOutputPath
	if outputDir == "" {
		outputDir = filepath.Join(bookDir, "craft")
	}

	// 确保目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 导出角色
	charData, err := json.MarshalIndent(export.Characters, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal characters: %w", err)
	}
	charPath := filepath.Join(outputDir, "characters.json")
	if err := os.WriteFile(charPath, charData, 0644); err != nil {
		return fmt.Errorf("failed to write characters: %w", err)
	}
	fmt.Printf("✓ 导出 %d 个角色到: %s\n", len(export.Characters), charPath)

	// 导出物品
	itemData, err := json.MarshalIndent(export.Items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}
	itemPath := filepath.Join(outputDir, "items.json")
	if err := os.WriteFile(itemPath, itemData, 0644); err != nil {
		return fmt.Errorf("failed to write items: %w", err)
	}
	fmt.Printf("✓ 导出 %d 个物品到: %s\n", len(export.Items), itemPath)

	// 导出地点
	locData, err := json.MarshalIndent(export.Locations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal locations: %w", err)
	}
	locPath := filepath.Join(outputDir, "locations.json")
	if err := os.WriteFile(locPath, locData, 0644); err != nil {
		return fmt.Errorf("failed to write locations: %w", err)
	}
	fmt.Printf("✓ 导出 %d 个地点到: %s\n", len(export.Locations), locPath)

	// 导出完整数据
	fullJSON, err := export.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to generate full export: %w", err)
	}
	fullPath := filepath.Join(outputDir, "rpg_export.json")
	if err := os.WriteFile(fullPath, []byte(fullJSON), 0644); err != nil {
		return fmt.Errorf("failed to write full export: %w", err)
	}
	fmt.Printf("✓ 导出完整数据到: %s\n", fullPath)

	fmt.Println("\n✅ RPG 数据已成功导出到 novelgen 格式！")
	fmt.Println("AI 生成的内容现在可以直接应用到项目中。")

	return nil
}
