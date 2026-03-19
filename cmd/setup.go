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

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create story setup using AI",
	Long: `Create or update the story setup for your novel using AI generation.

This command generates story_setup.json with genre, premise, rules, theme, tone, POV style, etc.`,
}

var setupGenCmd = &cobra.Command{
	Use:   "gen [prompt]",
	Short: "Generate story setup from a prompt",
	Long:  `Generate story setup using AI based on your story idea prompt.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSetupGen,
}

func init() {
	setupCmd.AddCommand(setupGenCmd)
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
	fmt.Println("  - Run 'novel compose' to generate the story outline")

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
