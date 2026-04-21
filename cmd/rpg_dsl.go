package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/rpg"
	"novelgen/internal/rpg/dsl"
)

// rpgDSLCmd represents the rpg-dsl command
var rpgDSLCmd = &cobra.Command{
	Use:   "rpg-dsl",
	Short: "DSL-RPG 工具集",
	Long:  `用于处理 RPG-DSL 文件的命令行工具，支持验证、合并、转换等功能。`,
}

// validateCmd validates DSL files
var validateCmd = &cobra.Command{
	Use:   "validate -b <book_name>",
	Short: "验证 DSL 文件",
	Long:  `验证指定项目的 DSL 文件语法和语义是否正确。`,
	Example: `  novelgen rpg-dsl validate -b fire-galaxy
  novelgen rpg-dsl validate -b fire-galaxy --verbose`,
	RunE: runValidate,
}

// mergeCmd merges DSL fragments
var mergeCmd = &cobra.Command{
	Use:   "merge -b <book_name>",
	Short: "合并 DSL 片段",
	Long:  `合并 outline、craft 等阶段的 DSL 片段为完整 DSL。`,
	Example: `  novelgen rpg-dsl merge -b fire-galaxy
  novelgen rpg-dsl merge -b fire-galaxy -o final.rpg`,
	RunE: runMerge,
}

// convertCmd converts novelgen project to DSL
var convertCmd = &cobra.Command{
	Use:   "convert -b <book_name>",
	Short: "转换项目为 DSL",
	Long:  `将 novelgen 项目数据（outline/craft）转换为 DSL 格式。`,
	Example: `  novelgen rpg-dsl convert -b fire-galaxy
  novelgen rpg-dsl convert -b fire-galaxy --phase outline
  novelgen rpg-dsl convert -b fire-galaxy --phase craft`,
	RunE: runConvert,
}

// dslExportCmd exports DSL to various formats
var dslExportCmd = &cobra.Command{
	Use:   "export-dsl -b <book_name> -f <format>",
	Short: "导出 DSL 为其他格式",
	Long:  `将 DSL 导出为 JSON、YAML 或其他格式。`,
	Example: `  novelgen rpg-dsl export-dsl -b fire-galaxy -f json
  novelgen rpg-dsl export-dsl -b fire-galaxy -f json -o rpg_data.json`,
	RunE: runExport,
}

// checkCmd checks for missing placeholders
var checkCmd = &cobra.Command{
	Use:   "check -b <book_name>",
	Short: "检查未填充的元素",
	Long:  `检查 DSL 中还有哪些 placeholder 未被填充，并生成 AI Prompt。`,
	Example: `  novelgen rpg-dsl check -b fire-galaxy
  novelgen rpg-dsl check -b fire-galaxy --generate-prompt`,
	RunE: runCheck,
}

// simulateCmd simulates a chapter
var simulateCmd = &cobra.Command{
	Use:   "simulate -b <book_name> -c <chapter_id>",
	Short: "推演章节",
	Long:  `使用 DSL 数据推演指定章节的游戏流程。`,
	Example: `  novelgen rpg-dsl simulate -b fire-galaxy -c ch1_awakening
  novelgen rpg-dsl simulate -b fire-galaxy --all`,
	RunE: runSimulate,
}

var (
	bookName       string
	outputFile     string
	phase          string
	format         string
	chapterID      string
	simulateAll    bool
	verbose        bool
	generatePrompt bool
)

func init() {
	rootCmd.AddCommand(rpgDSLCmd)
	rpgDSLCmd.AddCommand(validateCmd)
	rpgDSLCmd.AddCommand(mergeCmd)
	rpgDSLCmd.AddCommand(convertCmd)
	rpgDSLCmd.AddCommand(dslExportCmd)
	rpgDSLCmd.AddCommand(checkCmd)
	rpgDSLCmd.AddCommand(simulateCmd)

	// Common flags
	for _, cmd := range []*cobra.Command{validateCmd, mergeCmd, convertCmd, dslExportCmd, checkCmd, simulateCmd} {
		cmd.Flags().StringVarP(&bookName, "book", "b", "", "项目名称 (必填)")
		cmd.MarkFlagRequired("book")
	}

	// Output flags
	mergeCmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出文件路径")
	dslExportCmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出文件路径")
	dslExportCmd.Flags().StringVarP(&format, "format", "f", "json", "导出格式 (json/yaml)")

	// Phase flag
	convertCmd.Flags().StringVar(&phase, "phase", "all", "转换阶段 (outline/craft/all)")

	// Simulation flags
	simulateCmd.Flags().StringVarP(&chapterID, "chapter", "c", "", "章节 ID")
	simulateCmd.Flags().BoolVar(&simulateAll, "all", false, "推演所有章节")

	// Other flags
	validateCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细信息")
	checkCmd.Flags().BoolVar(&generatePrompt, "generate-prompt", false, "生成 AI Prompt")
}

func runValidate(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔍 验证 DSL 文件: %s\n\n", bookName)

	// Load and merge DSL
	dslData, err := loadAndMergeDSL(bookName)
	if err != nil {
		return fmt.Errorf("加载 DSL 失败: %w", err)
	}

	// Validate
	validator := dsl.NewValidator()
	if err := validator.Validate(dslData); err != nil {
		fmt.Printf("❌ 验证失败:\n%v\n", err)
		return nil
	}

	errors := validator.GetErrors()
	warnings := validator.GetWarnings()

	if len(errors) == 0 && len(warnings) == 0 {
		fmt.Println("✅ DSL 验证通过！")
	} else {
		if len(errors) > 0 {
			fmt.Printf("❌ 发现 %d 个错误:\n", len(errors))
			for _, e := range errors {
				fmt.Printf("  - [%s] %s\n", e.Field, e.Message)
			}
		}
		if len(warnings) > 0 {
			fmt.Printf("⚠️  发现 %d 个警告:\n", len(warnings))
			for _, w := range warnings {
				fmt.Printf("  - [%s] %s\n", w.Field, w.Message)
			}
		}
	}

	// Show statistics
	if verbose {
		fmt.Println("\n📊 统计信息:")
		fmt.Printf("  - 角色: %d\n", len(dslData.Characters.NPCs)+1)
		fmt.Printf("  - 地点: %d\n", len(dslData.World.Locations))
		fmt.Printf("  - 物品: %d\n", len(dslData.World.Items))
		fmt.Printf("  - 章节: %d\n", len(dslData.Storyline.Chapters))
	}

	return nil
}

func runMerge(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔀 合并 DSL 片段: %s\n\n", bookName)

	// Load DSL fragments
	fragments, err := loadDSLFragments(bookName)
	if err != nil {
		return fmt.Errorf("加载 DSL 片段失败: %w", err)
	}

	if len(fragments) == 0 {
		return fmt.Errorf("未找到 DSL 片段文件")
	}

	fmt.Printf("找到 %d 个 DSL 片段:\n", len(fragments))
	for _, f := range fragments {
		fmt.Printf("  - %s (%s)\n", f.FilePath, f.Phase)
	}
	fmt.Println()

	// Merge
	logger := dsl.NewConsoleLogger(dsl.WithMinLevel(dsl.LogLevelInfo))
	merger := dsl.NewDSLMerger(logger)

	for _, f := range fragments {
		merger.AddFragment(f.DSL, f.Phase, f.FilePath)
	}

	result, err := merger.Merge()
	if err != nil {
		return fmt.Errorf("合并失败: %w", err)
	}

	// Output
	if outputFile == "" {
		outputFile = filepath.Join(getBookRPGDir(bookName), "final.rpg")
	}

	if err := result.DSL.WriteToFile(outputFile); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	fmt.Printf("✅ 合并完成!\n")
	fmt.Printf("   输出: %s\n", outputFile)
	fmt.Printf("   阶段: %v\n", result.PhasesMerged)
	fmt.Printf("   未填充: %d\n", len(result.Placeholders))
	fmt.Printf("   冲突: %d\n", len(result.Conflicts))

	return nil
}

func runConvert(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔄 转换项目为 DSL: %s\n\n", bookName)

	// Load novelgen project
	project, err := loadNovelgenProject(bookName)
	if err != nil {
		return fmt.Errorf("加载项目失败: %w", err)
	}

	// Create adapter
	logger := dsl.NewConsoleLogger(dsl.WithMinLevel(dsl.LogLevelInfo))
	adapter := dsl.NewNovelgenAdapter(project, logger)

	outputDir := getBookRPGDir(bookName)
	os.MkdirAll(outputDir, 0755)

	// Convert based on phase
	switch phase {
	case "outline", "all":
		fmt.Println("生成 Outline DSL...")
		outlineDSL, err := adapter.ToDSL(dsl.PhaseOutline)
		if err != nil {
			return fmt.Errorf("生成 outline DSL 失败: %w", err)
		}
		outlinePath := filepath.Join(outputDir, "01_outline.rpg")
		if err := outlineDSL.WriteToFile(outlinePath); err != nil {
			return fmt.Errorf("写入 outline DSL 失败: %w", err)
		}
		fmt.Printf("  ✓ %s\n", outlinePath)

		if phase == "outline" {
			return nil
		}
		fallthrough

	case "craft":
		fmt.Println("生成 Craft DSL...")
		craftDSL, err := adapter.ToDSL(dsl.PhaseCraft)
		if err != nil {
			return fmt.Errorf("生成 craft DSL 失败: %w", err)
		}
		craftPath := filepath.Join(outputDir, "02_craft.rpg")
		if err := craftDSL.WriteToFile(craftPath); err != nil {
			return fmt.Errorf("写入 craft DSL 失败: %w", err)
		}
		fmt.Printf("  ✓ %s\n", craftPath)

	default:
		return fmt.Errorf("未知阶段: %s", phase)
	}

	fmt.Println("\n✅ 转换完成!")
	return nil
}

func runExport(cmd *cobra.Command, args []string) error {
	fmt.Printf("📤 导出 DSL: %s\n\n", bookName)

	// Load DSL
	dslData, err := loadAndMergeDSL(bookName)
	if err != nil {
		return fmt.Errorf("加载 DSL 失败: %w", err)
	}

	// Determine output file
	if outputFile == "" {
		ext := format
		if ext == "" {
			ext = "json"
		}
		outputFile = filepath.Join(getBookRPGDir(bookName), fmt.Sprintf("rpg_data.%s", ext))
	}

	// Export based on format
	switch strings.ToLower(format) {
	case "json":
		if err := exportToJSON(dslData, outputFile); err != nil {
			return fmt.Errorf("导出 JSON 失败: %w", err)
		}
	case "yaml", "yml":
		if err := exportToYAML(dslData, outputFile); err != nil {
			return fmt.Errorf("导出 YAML 失败: %w", err)
		}
	default:
		return fmt.Errorf("不支持的格式: %s", format)
	}

	fmt.Printf("✅ 导出完成: %s\n", outputFile)
	return nil
}

func runCheck(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔍 检查 DSL: %s\n\n", bookName)

	// Load and merge DSL
	dslData, err := loadAndMergeDSL(bookName)
	if err != nil {
		return fmt.Errorf("加载 DSL 失败: %w", err)
	}

	// Check placeholders
	placeholders := dslData.GetPlaceholderList()

	if len(placeholders) == 0 {
		fmt.Println("✅ 所有元素已填充！")
		return nil
	}

	fmt.Printf("⚠️  发现 %d 个未填充的元素:\n\n", len(placeholders))

	grouped := make(map[string][]dsl.PlaceholderInfo)
	for _, p := range placeholders {
		grouped[p.Type] = append(grouped[p.Type], p)
	}

	for typ, items := range grouped {
		fmt.Printf("[%s] %d 个:\n", typ, len(items))
		for _, p := range items {
			fmt.Printf("  - %s (id: %s)\n", p.Name, p.ID)
		}
		fmt.Println()
	}

	if generatePrompt {
		merger := dsl.NewDSLMerger(nil)
		result := &dsl.MergeResult{
			Placeholders: placeholders,
		}
		prompt := merger.GeneratePromptForPlaceholders(result)
		fmt.Println("📝 AI Prompt:")
		fmt.Println("---")
		fmt.Println(prompt)
		fmt.Println("---")
	}

	return nil
}

func runSimulate(cmd *cobra.Command, args []string) error {
	fmt.Printf("🎮 推演章节: %s\n\n", bookName)

	// Load DSL
	dslData, err := loadAndMergeDSL(bookName)
	if err != nil {
		return fmt.Errorf("加载 DSL 失败: %w", err)
	}

	// Validate first
	validator := dsl.NewValidator()
	if err := validator.Validate(dslData); err != nil {
		return fmt.Errorf("DSL 验证失败: %w", err)
	}

	// Run simulation
	simulator := dsl.NewSimulator(dslData)
	var issues []dsl.SimulationIssue

	if simulateAll {
		fmt.Println("📊 正在推演所有章节...")
		issues = simulator.SimulateAll()
	} else if chapterID != "" {
		fmt.Printf("📊 正在推演章节: %s\n", chapterID)
		issues = simulator.SimulateChapter(chapterID)
	} else {
		fmt.Println("📊 正在推演所有章节...")
		issues = simulator.SimulateAll()
	}

	// Display results
	fmt.Printf("\n📋 推演结果:\n")
	fmt.Printf("   总问题数: %d\n", len(issues))

	if len(issues) == 0 {
		fmt.Println("\n✅ 推演完成！未发现明显问题。")
		return nil
	}

	// Group by severity
	criticalCount := len(simulator.GetIssuesBySeverity(dsl.SeverityCritical))
	warningCount := len(simulator.GetIssuesBySeverity(dsl.SeverityWarning))
	infoCount := len(simulator.GetIssuesBySeverity(dsl.SeverityInfo))

	fmt.Printf("   🔴 严重: %d\n", criticalCount)
	fmt.Printf("   🟡 警告: %d\n", warningCount)
	fmt.Printf("   🔵 信息: %d\n", infoCount)

	// Display issues
	fmt.Println("\n📝 详细问题列表:")
	fmt.Println(strings.Repeat("-", 60))

	// Critical first
	for _, issue := range simulator.GetIssuesBySeverity(dsl.SeverityCritical) {
		fmt.Println(dsl.FormatIssue(issue))
		fmt.Println()
	}

	// Then warnings
	for _, issue := range simulator.GetIssuesBySeverity(dsl.SeverityWarning) {
		fmt.Println(dsl.FormatIssue(issue))
		fmt.Println()
	}

	// Then info
	for _, issue := range simulator.GetIssuesBySeverity(dsl.SeverityInfo) {
		fmt.Println(dsl.FormatIssue(issue))
		fmt.Println()
	}

	// Summary
	fmt.Println(strings.Repeat("-", 60))
	if criticalCount > 0 {
		fmt.Printf("\n⚠️  发现 %d 个严重问题，建议在生成大纲前修复。\n", criticalCount)
	} else if warningCount > 0 {
		fmt.Printf("\nℹ️  发现 %d 个警告，建议检查但不影响使用。\n", warningCount)
	} else {
		fmt.Println("\n✅ 推演完成！只有信息性提示。")
	}

	return nil
}

// Helper functions

func getBookRPGDir(bookName string) string {
	return filepath.Join("books", bookName, "story", "rpg")
}

func loadNovelgenProject(bookName string) (*rpg.NovelgenProject, error) {
	return rpg.LoadNovelgenProject(".", bookName)
}

func loadDSLFragments(bookName string) ([]*dsl.DSLFragment, error) {
	rpgDir := getBookRPGDir(bookName)

	// Look for DSL files
	patterns := []string{
		"01_outline.rpg",
		"02_craft.rpg",
		"03_systems.rpg",
		"*.rpg",
	}

	var fragments []*dsl.DSLFragment
	seen := make(map[string]bool)
	var parseErrors []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(rpgDir, pattern))
		if err != nil {
			continue
		}

		for _, file := range matches {
			if seen[file] {
				continue
			}
			seen[file] = true

			// Determine phase from filename
			phase := inferPhaseFromFilename(filepath.Base(file))

			// Parse DSL
			content, err := os.ReadFile(file)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("读取 %s 失败: %v", file, err))
				continue
			}

			parser := dsl.NewParser(string(content))
			dslData, err := parser.Parse()
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("解析 %s 失败: %v", file, err))
				continue
			}

			fragments = append(fragments, &dsl.DSLFragment{
				DSL:      dslData,
				Phase:    phase,
				FilePath: file,
			})
		}
	}

	if len(fragments) == 0 && len(parseErrors) > 0 {
		return nil, fmt.Errorf("DSL 解析错误:\n%s", strings.Join(parseErrors, "\n"))
	}

	return fragments, nil
}

func loadAndMergeDSL(bookName string) (*dsl.DSL, error) {
	fragments, err := loadDSLFragments(bookName)
	if err != nil {
		return nil, err
	}

	if len(fragments) == 0 {
		return nil, fmt.Errorf("未找到 DSL 文件")
	}

	// If only one fragment, return it directly
	if len(fragments) == 1 {
		return fragments[0].DSL, nil
	}

	// Merge fragments
	merger := dsl.NewDSLMerger(nil)
	for _, f := range fragments {
		merger.AddFragment(f.DSL, f.Phase, f.FilePath)
	}

	result, err := merger.Merge()
	if err != nil {
		return nil, err
	}

	return result.DSL, nil
}

func inferPhaseFromFilename(filename string) dsl.MergePhase {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "outline") {
		return dsl.PhaseOutline
	}
	if strings.Contains(lower, "craft") {
		return dsl.PhaseCraft
	}
	if strings.Contains(lower, "system") {
		return dsl.PhaseSystems
	}
	return dsl.PhaseFinal
}

func exportToJSON(dslData *dsl.DSL, path string) error {
	// TODO: Implement JSON export
	return fmt.Errorf("JSON export not yet implemented")
}

func exportToYAML(dslData *dsl.DSL, path string) error {
	// TODO: Implement YAML export
	return fmt.Errorf("YAML export not yet implemented")
}
