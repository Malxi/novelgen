package cmd

import (
	"fmt"
	"sort"

	"novelgen/internal/llm"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage LLM configuration",
	Long: `Configure the LLM provider settings for AI generation features.

This command manages the global LLM configuration stored in ~/.novelgen/llm_config.json.
You can configure multiple providers (OpenAI, Ollama, etc.) and switch between them.

Subcommands:
  show - Display current LLM configuration
  set  - Configure LLM settings interactively`,
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
		fmt.Printf("Default Provider: %s\n", config.DefaultProvider)
		fmt.Printf("Default Model:    %s\n", config.DefaultModel)
		fmt.Println()

		// Sort providers for consistent display
		providerNames := make([]string, 0, len(config.Providers))
		for name := range config.Providers {
			providerNames = append(providerNames, name)
		}
		sort.Strings(providerNames)

		for _, name := range providerNames {
			provider := config.Providers[name]
			fmt.Printf("Provider: %s\n", provider.Name)
			fmt.Printf("  Base URL: %s\n", provider.BaseURL)
			fmt.Printf("  Timeout:  %d seconds\n", provider.Timeout)
			fmt.Printf("  API Key:  %s\n", maskAPIKey(provider.APIKey))
			fmt.Println("  Models:")

			// Sort models for consistent display
			modelNames := make([]string, 0, len(provider.Models))
			for name := range provider.Models {
				modelNames = append(modelNames, name)
			}
			sort.Strings(modelNames)

			for _, modelName := range modelNames {
				model := provider.Models[modelName]
				fmt.Printf("    - %s (context: %d, max_tokens: %d, temp: %.1f)\n",
					model.Name, model.Context, model.MaxTokens, model.Temp)
			}
			fmt.Println()
		}

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

		// Get provider names
		providerNames := make([]string, 0, len(config.Providers))
		for name := range config.Providers {
			providerNames = append(providerNames, name)
		}

		// Select or create provider
		var selectedProvider string
		providerPrompt := &survey.Select{
			Message: "Select provider to configure:",
			Options: append(providerNames, "<new provider>"),
			Default: config.DefaultProvider,
		}
		if err := survey.AskOne(providerPrompt, &selectedProvider); err != nil {
			return err
		}

		// Create new provider if selected
		if selectedProvider == "<new provider>" {
			newProviderPrompt := &survey.Input{
				Message: "Provider name:",
				Help:    "e.g., openai, ollama, custom",
			}
			if err := survey.AskOne(newProviderPrompt, &selectedProvider, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			config.Providers[selectedProvider] = &llm.ProviderConfig{
				Name:   selectedProvider,
				Models: make(map[string]*llm.ModelConfig),
			}
		}

		provider := config.Providers[selectedProvider]

		// Base URL
		baseURLPrompt := &survey.Input{
			Message: "Base URL:",
			Help:    "The base URL for the API (e.g., http://127.0.0.1:11434/v1 for Ollama)",
			Default: provider.BaseURL,
		}
		if err := survey.AskOne(baseURLPrompt, &provider.BaseURL, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		// API Key
		apiKeyPrompt := &survey.Input{
			Message: "API Key:",
			Help:    "Your API key (use 'local-llama' for Ollama)",
			Default: provider.APIKey,
		}
		if err := survey.AskOne(apiKeyPrompt, &provider.APIKey); err != nil {
			return err
		}

		// Timeout
		timeoutPrompt := &survey.Input{
			Message: "Timeout (seconds):",
			Default: fmt.Sprintf("%d", provider.Timeout),
		}
		var timeoutStr string
		if err := survey.AskOne(timeoutPrompt, &timeoutStr); err != nil {
			return err
		}
		fmt.Sscanf(timeoutStr, "%d", &provider.Timeout)

		// Get model names for this provider
		modelNames := make([]string, 0, len(provider.Models))
		for name := range provider.Models {
			modelNames = append(modelNames, name)
		}

		// Select or create model
		var selectedModel string
		modelPrompt := &survey.Select{
			Message: "Select model to configure:",
			Options: append(modelNames, "<new model>"),
		}
		if len(modelNames) > 0 {
			modelPrompt.Default = modelNames[0]
		}
		if err := survey.AskOne(modelPrompt, &selectedModel); err != nil {
			return err
		}

		// Create new model if selected
		if selectedModel == "<new model>" {
			newModelPrompt := &survey.Input{
				Message: "Model name:",
				Help:    "e.g., qwen3.5:4b, gpt-4, etc.",
			}
			if err := survey.AskOne(newModelPrompt, &selectedModel, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}

		model := provider.Models[selectedModel]
		if model == nil {
			model = &llm.ModelConfig{Name: selectedModel}
			provider.Models[selectedModel] = model
		}

		// Context window
		contextPrompt := &survey.Input{
			Message: "Context window size:",
			Help:    "Model's context window size (e.g., 32000, 128000)",
			Default: fmt.Sprintf("%d", model.Context),
		}
		var contextStr string
		if err := survey.AskOne(contextPrompt, &contextStr); err != nil {
			return err
		}
		fmt.Sscanf(contextStr, "%d", &model.Context)

		// Max tokens
		maxTokensPrompt := &survey.Input{
			Message: "Max tokens for generation:",
			Help:    "Maximum tokens for generation (e.g., 8000, 4000)",
			Default: fmt.Sprintf("%d", model.MaxTokens),
		}
		var maxTokensStr string
		if err := survey.AskOne(maxTokensPrompt, &maxTokensStr); err != nil {
			return err
		}
		fmt.Sscanf(maxTokensStr, "%d", &model.MaxTokens)

		// Temperature
		tempPrompt := &survey.Input{
			Message: "Temperature:",
			Help:    "Temperature for generation (0.0-1.0, default 0.8)",
			Default: fmt.Sprintf("%.1f", model.Temp),
		}
		var tempStr string
		if err := survey.AskOne(tempPrompt, &tempStr); err != nil {
			return err
		}
		fmt.Sscanf(tempStr, "%f", &model.Temp)

		// Set as default
		setDefaultPrompt := &survey.Confirm{
			Message: "Set as default provider/model?",
			Default: false,
		}
		var setDefault bool
		if err := survey.AskOne(setDefaultPrompt, &setDefault); err != nil {
			return err
		}
		if setDefault {
			config.DefaultProvider = selectedProvider
			config.DefaultModel = selectedModel
		}

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
	// Register config command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return configCmd
	})
}
