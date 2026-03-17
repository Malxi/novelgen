package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"nolvegen/internal/models"
)

var genPrompt string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new novel project",
	Long: `Initialize a new novel project and define core story setup.

This command creates a new novel project directory structure and generates
the initial story setup configuration including genre, premise, rules,
theme, tone, POV style, and more.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&genPrompt, "gen", "", "Use AI to generate the story setup based on the prompt")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if novel.json already exists
	if _, err := os.Stat("novel.json"); err == nil {
		return fmt.Errorf("a novel project already exists in this directory (novel.json found)")
	}

	var setup *models.StorySetup
	var err error

	if genPrompt != "" {
		// AI generation mode
		setup, err = generateStorySetupWithAI(genPrompt)
		if err != nil {
			return fmt.Errorf("failed to generate story setup with AI: %w", err)
		}
	} else {
		// Interactive mode
		setup, err = interactiveStorySetup()
		if err != nil {
			return fmt.Errorf("failed to get story setup: %w", err)
		}
	}

	// Create project directory structure
	if err := createProjectStructure(setup); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	fmt.Printf("\n✓ Novel project '%s' initialized successfully!\n", setup.ProjectName)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit config/init/story_setup.json to refine your story setup")
	fmt.Println("  - Run 'novel compose' to generate the story outline")

	return nil
}

func interactiveStorySetup() (*models.StorySetup, error) {
	fmt.Println("📚 Novel Project Initialization")
	fmt.Println("================================")
	fmt.Println()

	setup := &models.StorySetup{}

	// Project name
	namePrompt := &survey.Input{
		Message: "Project name:",
		Help:    "The name of your novel project",
	}
	if err := survey.AskOne(namePrompt, &setup.ProjectName, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	// Genre(s)
	genrePrompt := &survey.Input{
		Message: "Genre(s):",
		Help:    "Comma-separated list of genres (e.g., Fantasy, Adventure, Mystery)",
	}
	var genresStr string
	if err := survey.AskOne(genrePrompt, &genresStr, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}
	setup.Genres = splitAndTrim(genresStr)

	// Core premise
	premisePrompt := &survey.Multiline{
		Message: "Core premise:",
		Help:    "A brief summary of what your story is about",
	}
	if err := survey.AskOne(premisePrompt, &setup.Premise, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	// Theme
	themePrompt := &survey.Input{
		Message: "Core theme:",
		Help:    "The central theme of your story (e.g., 'courage vs power', 'redemption')",
	}
	if err := survey.AskOne(themePrompt, &setup.Theme); err != nil {
		return nil, err
	}

	// Story rules/constraints
	rulesPrompt := &survey.Multiline{
		Message: "Story rules/constraints:",
		Help:    "Enter rules one per line (e.g., 'Magic is rare', 'Dragons are extinct'). Press Ctrl+D when done.",
	}
	var rulesStr string
	if err := survey.AskOne(rulesPrompt, &rulesStr); err != nil {
		return nil, err
	}
	if rulesStr != "" {
		setup.Rules = splitLinesAndTrim(rulesStr)
	}

	// Target audience
	audiencePrompt := &survey.Select{
		Message: "Target audience:",
		Options: []string{"Children", "Young Adult", "Adult", "All Ages"},
		Default: "Young Adult",
	}
	if err := survey.AskOne(audiencePrompt, &setup.TargetAudience); err != nil {
		return nil, err
	}

	// Tone/style
	tonePrompt := &survey.Input{
		Message: "Tone/style:",
		Help:    "The overall tone of your story (e.g., 'Epic, Hopeful', 'Dark, Gritty')",
	}
	if err := survey.AskOne(tonePrompt, &setup.Tone); err != nil {
		return nil, err
	}

	// Narrative tense
	tensePrompt := &survey.Select{
		Message: "Narrative tense:",
		Options: []string{"past", "present"},
		Default: "past",
	}
	if err := survey.AskOne(tensePrompt, &setup.Tense); err != nil {
		return nil, err
	}

	// POV style
	povPrompt := &survey.Select{
		Message: "POV style:",
		Options: []string{
			"first-person",
			"third-person limited",
			"third-person omniscient",
		},
		Default: "third-person limited",
	}
	if err := survey.AskOne(povPrompt, &setup.POVStyle); err != nil {
		return nil, err
	}

	return setup, nil
}

func generateStorySetupWithAI(prompt string) (*models.StorySetup, error) {
	// TODO: Implement AI generation
	// For now, return an error indicating this feature is not yet implemented
	return nil, fmt.Errorf("AI generation is not yet implemented. Please use interactive mode")
}

func createProjectStructure(setup *models.StorySetup) error {
	dirs := []string{
		"config/init",
		"config/compose",
		"config/worldbuilding/characters",
		"config/worldbuilding/weapons",
		"config/worldbuilding/items",
		"config/worldbuilding/locations",
		"config/worldbuilding/factions",
		"config/storyline",
		"config/events",
		"data/chapters",
		"data/snapshots",
		"logs",
		"version",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create novel.json
	config := &models.ProjectConfig{
		Name:      setup.ProjectName,
		Version:   "1.0.0",
		CreatedAt: time.Now(),
	}
	if err := config.Save("novel.json"); err != nil {
		return fmt.Errorf("failed to save novel.json: %w", err)
	}

	// Create story_setup.json in config/init/
	setupPath := filepath.Join("config", "init", "story_setup.json")
	if err := setup.Save(setupPath); err != nil {
		return fmt.Errorf("failed to save story_setup.json: %w", err)
	}

	// Create story_setup.md (markdown version for easier editing)
	mdPath := filepath.Join("config", "init", "story_setup.md")
	if err := createStorySetupMarkdown(setup, mdPath); err != nil {
		return fmt.Errorf("failed to save story_setup.md: %w", err)
	}

	return nil
}

func createStorySetupMarkdown(setup *models.StorySetup, path string) error {
	content := fmt.Sprintf(`# %s

## Story Setup

### Genre(s)
%s

### Core Premise
%s

### Theme
%s

### Story Rules/Constraints
%s

### Target Audience
%s

### Tone/Style
%s

### Narrative Tense
%s

### POV Style
%s
`,
		setup.ProjectName,
		formatList(setup.Genres),
		setup.Premise,
		setup.Theme,
		formatList(setup.Rules),
		setup.TargetAudience,
		setup.Tone,
		setup.Tense,
		setup.POVStyle,
	)

	return os.WriteFile(path, []byte(content), 0644)
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitLinesAndTrim(s string) []string {
	parts := strings.Split(s, "\n")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "- (none)"
	}
	var result []string
	for _, item := range items {
		result = append(result, "- "+item)
	}
	return strings.Join(result, "\n")
}
