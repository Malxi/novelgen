package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nolvegen/internal/agents"
	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
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
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOLVEGEN INIT")

	// Check if novel.json already exists
	if _, err := os.Stat("novel.json"); err == nil {
		logger.Error("A novel project already exists in this directory (novel.json found)")
		return fmt.Errorf("a novel project already exists in this directory (novel.json found)")
	}

	var setup *models.StorySetup
	var err error

	if genPrompt != "" {
		// AI generation mode
		logger.Info("AI generation mode with prompt: %s", genPrompt)
		setup, err = generateStorySetupWithAI(genPrompt)
		if err != nil {
			logger.Error("Failed to generate story setup with AI: %v", err)
			return fmt.Errorf("failed to generate story setup with AI: %w", err)
		}
	} else {
		// Interactive mode
		logger.Info("Interactive mode")
		setup, err = interactiveStorySetup()
		if err != nil {
			logger.Error("Failed to get story setup: %v", err)
			return fmt.Errorf("failed to get story setup: %w", err)
		}
	}

	// Get story structure configuration
	structure, err := interactiveStoryStructure()
	if err != nil {
		return fmt.Errorf("failed to get story structure: %w", err)
	}

	// Get language
	language, err := interactiveLanguage()
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	// Use default LLM config (no interactive prompts)
	llmConfig := models.DefaultProjectLLM()

	// Create project config
	config := &models.ProjectConfig{
		Name:          setup.ProjectName,
		Version:       "1.0.0",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Language:      language,
		Structure:     structure,
		ChapterConfig: models.DefaultChapterConfig(),
		LLM:           llmConfig,
	}

	// Create project directory structure
	if err := createProjectStructure(setup, config); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	fmt.Printf("\n✓ Novel project '%s' initialized successfully!\n", setup.ProjectName)
	fmt.Printf("\n📊 Story Structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		structure.TargetParts, structure.TargetVolumes, structure.TargetChapters,
		structure.TotalChapters())
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
		Options: []string{"first_person", "third_person_limited", "third_person_omniscient"},
		Default: "third_person_limited",
	}
	if err := survey.AskOne(povPrompt, &setup.POVStyle); err != nil {
		return nil, err
	}

	return setup, nil
}

func interactiveStoryStructure() (models.StoryStructure, error) {
	fmt.Println("\n📖 Story Structure Configuration")
	fmt.Println("=================================")
	fmt.Println()

	structure := models.DefaultStoryStructure()

	// Number of parts
	partsPrompt := &survey.Input{
		Message: "Number of parts (部):",
		Help:    "How many major parts will your story have?",
		Default: "3",
	}
	var partsStr string
	if err := survey.AskOne(partsPrompt, &partsStr, survey.WithValidator(survey.Required)); err != nil {
		return structure, err
	}
	if parts, err := strconv.Atoi(partsStr); err == nil && parts > 0 {
		structure.TargetParts = parts
	}

	// Number of volumes per part
	volumesPrompt := &survey.Input{
		Message: "Number of volumes per part (卷):",
		Help:    "How many volumes will each part have?",
		Default: "2",
	}
	var volumesStr string
	if err := survey.AskOne(volumesPrompt, &volumesStr, survey.WithValidator(survey.Required)); err != nil {
		return structure, err
	}
	if volumes, err := strconv.Atoi(volumesStr); err == nil && volumes > 0 {
		structure.TargetVolumes = volumes
	}

	// Number of chapters per volume
	chaptersPrompt := &survey.Input{
		Message: "Number of chapters per volume (章):",
		Help:    "How many chapters will each volume have?",
		Default: "3",
	}
	var chaptersStr string
	if err := survey.AskOne(chaptersPrompt, &chaptersStr, survey.WithValidator(survey.Required)); err != nil {
		return structure, err
	}
	if chapters, err := strconv.Atoi(chaptersStr); err == nil && chapters > 0 {
		structure.TargetChapters = chapters
	}

	totalChapters := structure.TotalChapters()
	fmt.Printf("\n📊 Total chapters: %d (%d parts × %d volumes × %d chapters)\n",
		totalChapters, structure.TargetParts, structure.TargetVolumes, structure.TargetChapters)

	return structure, nil
}

func interactiveLanguage() (string, error) {
	fmt.Println("\n🌐 Language Configuration")
	fmt.Println("=========================")

	languagePrompt := &survey.Select{
		Message: "Select the story language:",
		Options: []string{"中文 (Chinese)", "English", "日本語 (Japanese)", "Español (Spanish)", "Français (French)", "Deutsch (German)"},
		Default: "中文 (Chinese)",
	}

	var languageStr string
	if err := survey.AskOne(languagePrompt, &languageStr); err != nil {
		return "zh", err
	}

	// Extract language code
	switch {
	case strings.Contains(languageStr, "中文"):
		return "zh", nil
	case strings.Contains(languageStr, "English"):
		return "en", nil
	case strings.Contains(languageStr, "日本語"):
		return "ja", nil
	case strings.Contains(languageStr, "Español"):
		return "es", nil
	case strings.Contains(languageStr, "Français"):
		return "fr", nil
	case strings.Contains(languageStr, "Deutsch"):
		return "de", nil
	default:
		return "zh", nil
	}
}

func generateStorySetupWithAI(prompt string) (*models.StorySetup, error) {
	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Use default project LLM settings for init
	projectLLM := models.DefaultProjectLLM()

	provider, model := cfg.GetActiveModel(&projectLLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Println()

	// Create LLM client and agent
	client := cfg.CreateClient(&projectLLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewInitAgent(client, cfg, &projectLLM)

	return agent.GenerateStorySetup(prompt)
}

func createProjectStructure(setup *models.StorySetup, config *models.ProjectConfig) error {
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

	// Save novel.json
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

func formatList(items []string) string {
	if len(items) == 0 {
		return "None"
	}
	var result strings.Builder
	for _, item := range items {
		result.WriteString(fmt.Sprintf("- %s\n", item))
	}
	return result.String()
}
