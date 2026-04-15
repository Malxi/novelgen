package main

import (
	"flag"
	"fmt"
	"os"

	"novelgen/internal/logger"
	"novelgen/internal/rpg"
)

func main() {
	var (
		bookName    = flag.String("b", "", "Book name (required)")
		projectPath = flag.String("p", ".", "Project path")
		showReport  = flag.Bool("report", false, "Show constraint report only")
	)
	flag.Parse()

	if *bookName == "" {
		fmt.Println("Usage: rpg-write -b <book_name> [-p <project_path>] [-report]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	logger.Section("RPG Constraint System Demo")
	logger.Info("Book: %s", *bookName)
	logger.Info("Project Path: %s", *projectPath)

	// 加载RPG项目
	project, err := rpg.LoadNovelgenProject(*projectPath, *bookName)
	if err != nil {
		logger.Error("Failed to load RPG project: %v", err)
		logger.Info("Make sure the book exists with craft data (characters.json, items.json, locations.json)")
		os.Exit(1)
	}

	logger.Section("Project Summary")
	summary := project.GetProjectSummary()
	logger.Info("Book Name: %s", summary["book_name"])
	logger.Info("Characters: %d", summary["characters"])
	logger.Info("Items: %d", summary["items"])
	logger.Info("Locations: %d", summary["locations"])
	logger.Info("Parts: %d", summary["parts"])

	// 转换为RPG世界
	world, err := project.ConvertToRPG()
	if err != nil {
		logger.Error("Failed to convert to RPG world: %v", err)
		os.Exit(1)
	}

	logger.Section("RPG World Created")
	logger.Info("Characters in world: %d", len(world.Characters.GetAllCharacters()))
	logger.Info("Items in world: %d", len(world.Items.GetAllItems()))
	logger.Info("Maps in world: %d", len(world.Maps.GetAllMaps()))

	// 创建约束系统
	constraintSystem := rpg.NewConstraintSystem(world)
	constraintReport := constraintSystem.BuildFromRPGData()

	logger.Section("Constraint Report")
	logger.Info("Character Constraints: %d", len(constraintReport.CharacterConstraints))
	logger.Info("Plot Constraints: configured")
	logger.Info("Power Constraints: configured")
	logger.Info("Total Suggestions: %d", len(constraintReport.Suggestions))

	// 显示硬性约束
	logger.Section("Hard Constraints (Must Follow)")
	hardCount := 0
	for _, sug := range constraintReport.Suggestions {
		if sug.Type == "hard" {
			hardCount++
			logger.Info("[%s] %s: %s", sug.Category, sug.Target, sug.Constraint)
			logger.Info("  Reason: %s (Priority: %d)", sug.Reason, sug.Priority)
		}
	}

	if hardCount == 0 {
		logger.Info("No hard constraints defined")
	}

	// 显示软性约束
	logger.Section("Soft Constraints (Recommended)")
	softCount := 0
	for _, sug := range constraintReport.Suggestions {
		if sug.Type == "soft" {
			softCount++
			logger.Info("[%s] %s: %s", sug.Category, sug.Target, sug.Constraint)
		}
	}

	if softCount == 0 {
		logger.Info("No soft constraints defined")
	}

	// 显示约束提示词格式
	if *showReport {
		logger.Section("Constraint Prompt Format")
		promptFormat := constraintSystem.ToPromptFormat(constraintReport)
		fmt.Println(promptFormat)
	}

	// 显示系统提示词格式
	logger.Section("System Prompt Format")
	systemPrompt := constraintSystem.ToSystemPrompt(constraintReport)
	fmt.Println(systemPrompt)

	logger.Section("Demo Complete!")
	logger.Info("This demonstrates how RPG constraints can guide AI writing")
	logger.Info("Use these constraints in your WriteAgent to ensure story consistency")
}
