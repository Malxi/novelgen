package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"nolvegen/internal/llm"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage LLM configuration",
	Long:  `Configure the LLM provider settings for AI generation features.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current LLM configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := llm.LoadOrCreateConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Println("Current LLM Configuration:")
		fmt.Println("==========================")
		fmt.Printf("Provider: %s\n", config.Provider)
		fmt.Printf("Base URL: %s\n", config.BaseURL)
		fmt.Printf("Model:    %s\n", config.Model)
		fmt.Printf("Timeout:  %d seconds\n", config.Timeout)
		fmt.Printf("API Key:  %s\n", maskAPIKey(config.APIKey))

		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set LLM configuration interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := llm.LoadOrCreateConfig()
		if err != nil {
			config = llm.DefaultConfig()
		}

		fmt.Println("Configure LLM Settings")
		fmt.Println("======================")
		fmt.Println()

		// Provider
		providerPrompt := &survey.Select{
			Message: "Provider:",
			Options: []string{"ollama", "openai", "custom"},
			Default: config.Provider,
		}
		if err := survey.AskOne(providerPrompt, &config.Provider); err != nil {
			return err
		}

		// Base URL
		baseURLPrompt := &survey.Input{
			Message: "Base URL:",
			Help:    "The base URL for the API (e.g., http://127.0.0.1:11434/v1 for Ollama)",
			Default: config.BaseURL,
		}
		if err := survey.AskOne(baseURLPrompt, &config.BaseURL, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		// Model
		modelPrompt := &survey.Input{
			Message: "Model:",
			Help:    "The model name (e.g., qwen3.5:4b, gpt-4, etc.)",
			Default: config.Model,
		}
		if err := survey.AskOne(modelPrompt, &config.Model, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		// API Key
		apiKeyPrompt := &survey.Input{
			Message: "API Key:",
			Help:    "Your API key (use 'local-llama' for Ollama)",
			Default: config.APIKey,
		}
		if err := survey.AskOne(apiKeyPrompt, &config.APIKey); err != nil {
			return err
		}

		// Timeout
		timeoutPrompt := &survey.Input{
			Message: "Timeout (seconds):",
			Default: fmt.Sprintf("%d", config.Timeout),
		}
		var timeoutStr string
		if err := survey.AskOne(timeoutPrompt, &timeoutStr); err != nil {
			return err
		}
		fmt.Sscanf(timeoutStr, "%d", &config.Timeout)

		// Save config
		path := llm.GetConfigPath()
		if err := config.Save(path); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println()
		fmt.Printf("✓ Configuration saved to %s\n", path)

		return nil
	},
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}
