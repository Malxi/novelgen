package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nolvegen/internal/agents"
	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"

	"github.com/spf13/cobra"
)

var (
	setupRegenPrompt string
	setupMaxRounds   int
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create story setup",
	Long: `Create or update the story setup for your novel.

This command generates story/setup/story_setup.json containing:
  - Genre(s) and subgenres
  - Core premise and logline
  - Story rules and mechanics
  - Themes and motifs
  - Tone and atmosphere
  - POV style and narrative voice

Subcommands:
  gen     - Generate story setup using AI from a prompt
  regen   - Regenerate story setup with optional guidance
  improve - Improve existing story setup through AI review`,
}

var setupGenCmd = &cobra.Command{
	Use:   "gen [prompt]",
	Short: "Generate story setup from a prompt",
	Long: `Generate story setup using AI based on your story idea prompt.

Examples:
  novel setup gen "一个关于太空探险的故事"
  novel setup gen "赛博朋克背景下的侦探故事"`,
	Args: cobra.ExactArgs(1),
	RunE: runSetupGen,
}

var setupRegenCmd = &cobra.Command{
	Use:   "regen",
	Short: "Regenerate story setup",
	Long: `Regenerate the story setup with optional guidance.

This command reads the existing story setup and regenerates it
based on the optional prompt guidance.

Examples:
  novel setup regen                      # Regenerate with current setup
  novel setup regen --prompt "增加更多悬疑元素"
  novel setup regen --prompt "改为喜剧风格"`,
	RunE: runSetupRegen,
}

var setupImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Improve story setup through AI review",
	Long: `Improve the existing story setup through AI review and refinement.

This command analyzes the current story setup and suggests improvements
to make it more compelling, coherent, and complete.

Examples:
  novel setup improve                    # Improve with 1 round
  novel setup improve --max-rounds 3     # Improve with up to 3 rounds`,
	RunE: runSetupImprove,
}

func init() {
	setupCmd.AddCommand(setupGenCmd)
	setupCmd.AddCommand(setupRegenCmd)
	setupCmd.AddCommand(setupImproveCmd)

	// Regen flags
	setupRegenCmd.Flags().StringVar(&setupRegenPrompt, "prompt", "", "Guidance for regeneration")

	// Improve flags
	setupImproveCmd.Flags().IntVar(&setupMaxRounds, "max-rounds", 1, "Maximum improvement rounds")
}

func runSetupGen(cmd *cobra.Command, args []string) error {
	prompt := args[0]

	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOLVEGEN SETUP")

	// Check if we're in a project directory
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novel init <book_name>' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// AI generation mode
	logger.Info("AI generation mode with prompt: %s", prompt)
	logger.Info("Using language: %s", projectConfig.Language)
	setup, err := generateStorySetupWithAI(prompt, projectConfig.Language, &projectConfig.LLM)
	if err != nil {
		logger.Error("Failed to generate story setup with AI: %v", err)
		return fmt.Errorf("failed to generate story setup with AI: %w", err)
	}

	// Save story setup
	if err := saveStorySetup(setup); err != nil {
		return fmt.Errorf("failed to save story setup: %w", err)
	}

	fmt.Printf("\n✓ Story setup created successfully!\n")
	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Edit story/setup/story_setup.json to refine your story setup")
	fmt.Println("  - Run 'novel compose gen' to generate the story outline")

	return nil
}

func runSetupRegen(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOLVEGEN SETUP REGEN")

	// Check if we're in a project directory
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novel init <book_name>' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Load existing story setup
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	existingSetup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		logger.Error("Failed to load existing story setup: %v", err)
		return fmt.Errorf("failed to load existing story setup: %w", err)
	}
	logger.Info("Loaded existing story setup")

	// Build prompt for regeneration
	prompt := existingSetup.Premise
	if setupRegenPrompt != "" {
		prompt = fmt.Sprintf("%s\n\nAdditional guidance: %s", prompt, setupRegenPrompt)
		logger.Info("Using regeneration guidance: %s", setupRegenPrompt)
	}

	// Regenerate story setup
	logger.Info("Regenerating story setup...")
	setup, err := generateStorySetupWithAI(prompt, projectConfig.Language, &projectConfig.LLM)
	if err != nil {
		logger.Error("Failed to regenerate story setup: %v", err)
		return fmt.Errorf("failed to regenerate story setup: %w", err)
	}

	// Save story setup
	if err := saveStorySetup(setup); err != nil {
		return fmt.Errorf("failed to save story setup: %w", err)
	}

	fmt.Printf("\n✓ Story setup regenerated successfully!\n")
	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)

	return nil
}

func runSetupImprove(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOLVEGEN SETUP IMPROVE")

	// Check if we're in a project directory
	if _, err := os.Stat("novel.json"); err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novel init <book_name>' first")
	}

	// Load project config
	projectConfig, err := models.LoadProjectConfig("novel.json")
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Load existing story setup
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		logger.Error("Failed to load existing story setup: %v", err)
		return fmt.Errorf("failed to load existing story setup: %w", err)
	}
	logger.Info("Loaded existing story setup")

	// Create LLM client and agent
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	provider, model := cfg.GetActiveModel(&projectConfig.LLM)
	if provider == nil || model == nil {
		return fmt.Errorf("failed to get active LLM configuration")
	}

	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	agent := agents.NewSetupAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)

	// Improve story setup
	fmt.Printf("\n🔄 Starting setup improvement (max %d rounds)...\n\n", setupMaxRounds)

	for round := 1; round <= setupMaxRounds; round++ {
		logger.Section(fmt.Sprintf("IMPROVEMENT ROUND %d/%d", round, setupMaxRounds))

		improvedSetup, err := agent.ImproveStorySetup(setup)
		if err != nil {
			logger.Error("Failed to improve story setup: %v", err)
			return fmt.Errorf("failed to improve story setup: %w", err)
		}

		setup = improvedSetup

		// Save after each round
		if err := saveStorySetup(setup); err != nil {
			return fmt.Errorf("failed to save improved story setup: %w", err)
		}

		fmt.Printf("\n✓ Round %d completed\n", round)
	}

	fmt.Printf("\n✓ Story setup improved successfully after %d round(s)!\n", setupMaxRounds)
	fmt.Printf("\n📚 Project: %s\n", setup.ProjectName)
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(setup.Genres, ", "))
	fmt.Printf("📖 Premise: %.100s...\n", setup.Premise)

	return nil
}

func generateStorySetupWithAI(prompt, language string, projectLLM *models.ProjectLLM) (*models.StorySetup, error) {
	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Use project LLM settings from novel.json
	if projectLLM.Provider == "" {
		projectLLM.Provider = "ollama"
	}
	if projectLLM.Model == "" {
		if projectLLM.Provider == "openai" {
			projectLLM.Model = "gpt-4"
		} else {
			projectLLM.Model = "qwen3.5:4b"
		}
	}

	provider, model := cfg.GetActiveModel(projectLLM)
	if provider == nil || model == nil {
		return nil, fmt.Errorf("failed to get active LLM configuration")
	}

	fmt.Printf("Using provider: %s, model: %s at %s\n", provider.Name, model.Name, provider.BaseURL)
	fmt.Printf("Language: %s\n", language)
	fmt.Println()

	// Create LLM client and agent
	client := cfg.CreateClient(projectLLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}
	agent := agents.NewSetupAgent(client, cfg, projectLLM)
	agent.SetLanguage(language)

	return agent.GenerateStorySetup(prompt)
}

func saveStorySetup(setup *models.StorySetup) error {
	// Create story_setup.json in story/setup/
	setupPath := filepath.Join("story", "setup", "story_setup.json")
	if err := setup.Save(setupPath); err != nil {
		return fmt.Errorf("failed to save story_setup.json: %w", err)
	}

	// Create story_setup.md (markdown version for easier editing)
	mdPath := filepath.Join("story", "setup", "story_setup.md")
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

## Storylines
%s

## Premises
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
		formatStorylines(setup.Storylines),
		formatPremises(setup.Premises),
	)

	return os.WriteFile(path, []byte(content), 0644)
}

func formatStorylines(storylines []models.Storyline) string {
	if len(storylines) == 0 {
		return "No storylines defined."
	}
	var result strings.Builder
	for _, s := range storylines {
		result.WriteString(fmt.Sprintf("\n### %s\n", s.Name))
		result.WriteString(fmt.Sprintf("- **Type**: %s\n", s.Type))
		result.WriteString(fmt.Sprintf("- **Importance**: %d/10\n", s.Importance))
		result.WriteString(fmt.Sprintf("- **Description**: %s\n", s.Description))
	}
	return result.String()
}

func formatPremises(premises []models.Premise) string {
	if len(premises) == 0 {
		return "No premises defined."
	}
	var result strings.Builder
	for _, p := range premises {
		result.WriteString(fmt.Sprintf("\n### %s (%s)\n", p.Name, p.Category))
		result.WriteString(fmt.Sprintf("%s\n\n", p.Description))
		if len(p.Progression) > 0 {
			result.WriteString("**Progression System:**\n\n")
			for _, stage := range p.Progression {
				result.WriteString(fmt.Sprintf("**Level %d: %s**\n", stage.Level, stage.Name))
				result.WriteString(fmt.Sprintf("- Description: %s\n", stage.Description))
				if stage.Requirements != "" {
					result.WriteString(fmt.Sprintf("- Requirements: %s\n", stage.Requirements))
				}
				result.WriteString("\n")
			}
		}
	}
	return result.String()
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
