package cmd

import (
	"fmt"
	"os"

	"nolvegen/internal/agents"
	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"

	"github.com/spf13/cobra"
)

// init registers the translate command
// This is called automatically when the package is imported
func init() {
	RegisterCommand(func() *cobra.Command {
		return translateCmd
	})
}

var (
	translateSourceLang string
	translateTargetLang string
	translateOutputFile string
)

var translateCmd = &cobra.Command{
	Use:   "translate [file]",
	Short: "Translate novel content between languages",
	Long: `Translate novel content from one language to another using AI.

This command translates chapters, story setup, or other text files
while preserving the narrative style and formatting.

Examples:
  novel translate story/chapters/chapter_001.txt
  novel translate story/setup/story_setup.md --target-lang en
  novel translate chapter.txt --source-lang zh --target-lang en --output chapter_en.txt`,
	Args: cobra.ExactArgs(1),
	RunE: runTranslate,
}

func init() {
	// These flags are automatically registered when the command is loaded
	translateCmd.Flags().StringVar(&translateSourceLang, "source-lang", "zh", "Source language (auto-detect if not specified)")
	translateCmd.Flags().StringVar(&translateTargetLang, "target-lang", "en", "Target language for translation")
	translateCmd.Flags().StringVar(&translateOutputFile, "output", "", "Output file (default: stdout)")
}

func runTranslate(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOLVEGEN TRANSLATE")

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

	// Read input file
	content, err := os.ReadFile(inputFile)
	if err != nil {
		logger.Error("Failed to read input file: %v", err)
		return fmt.Errorf("failed to read input file %s: %w", inputFile, err)
	}
	logger.Info("Read %d bytes from %s", len(content), inputFile)

	// Create LLM client
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Get or create translate agent using the registry
	var agent *agents.TranslateAgent
	if agents.HasAgent("translate") {
		// Use registered agent factory
		baseAgent := agents.GetAgent("translate", client, cfg, &projectConfig.LLM)
		if ta, ok := baseAgent.(*agents.TranslateAgent); ok {
			agent = ta
		}
	}

	// Fallback: create agent directly
	if agent == nil {
		agent = agents.NewTranslateAgent(client, cfg, &projectConfig.LLM)
	}

	agent.SetLanguage(projectConfig.Language)

	// Perform translation
	logger.Info("Translating from %s to %s...", translateSourceLang, translateTargetLang)

	pm := prompts.NewPromptManager()
	systemPrompt, userPrompt, err := pm.Build(
		prompts.SkillTranslation,
		"default",
		prompts.BuildTranslationData(string(content), translateSourceLang, translateTargetLang),
	)
	if err != nil {
		return fmt.Errorf("failed to build translation prompt: %w", err)
	}

	translated, err := agent.Translate(systemPrompt, userPrompt)
	if err != nil {
		logger.Error("Translation failed: %v", err)
		return fmt.Errorf("translation failed: %w", err)
	}

	// Output result
	if translateOutputFile != "" {
		if err := os.WriteFile(translateOutputFile, []byte(translated), 0644); err != nil {
			logger.Error("Failed to write output file: %v", err)
			return fmt.Errorf("failed to write output file: %w", err)
		}
		logger.Info("Translation saved to: %s", translateOutputFile)
	} else {
		fmt.Println(translated)
	}

	logger.Info("Translation completed!")
	return nil
}
