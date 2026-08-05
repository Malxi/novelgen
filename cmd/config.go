package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"novelgen/internal/agentruntime"
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
  set  - Configure LLM settings interactively
  agent - Configure Claude agent runtime interactively`,
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

		fmt.Println("Agent Runtime Configuration:")
		fmt.Println("============================")
		agentCfg, agentErr := agentruntime.LoadConfig()
		if agentErr != nil {
			fmt.Printf("Agent Config: not found (%s)\n", agentruntime.ConfigPath())
			return nil
		}
		fmt.Printf("Agent Config:    %s\n", agentruntime.ConfigPath())
		fmt.Printf("Agent Home:      %s\n", agentCfg.AgentHome)
		fmt.Printf("Default Runtime: %s\n", agentCfg.DefaultRuntime)
		runtimeNames := make([]string, 0, len(agentCfg.Runtimes))
		for name := range agentCfg.Runtimes {
			runtimeNames = append(runtimeNames, name)
		}
		sort.Strings(runtimeNames)
		for _, name := range runtimeNames {
			runtime := agentCfg.Runtimes[name]
			fmt.Printf("Runtime: %s\n", name)
			fmt.Printf("  Type:    %s\n", runtime.Type)
			fmt.Printf("  Command: %s %s\n", runtime.Command, strings.Join(runtime.Args, " "))
			fmt.Printf("  BaseURL: %s\n", runtime.BaseURL)
			fmt.Printf("  Model:   %s\n", runtime.Model)
			fmt.Printf("  Max Turns: %d\n", agentMaxTurns(runtime))
			fmt.Printf("  Settings: %s\n", runtime.Settings)
			fmt.Printf("  Sources:  %s\n", strings.Join(runtime.SettingSources, ","))
			fmt.Printf("  SDK Skills: %s\n", strings.Join(runtime.SDKSkills, ","))
			fmt.Printf("  Add Dirs: %s\n", strings.Join(runtime.AddDirs, ","))
			fmt.Printf("  Tools: %s\n", strings.Join(runtime.Tools, ","))
			fmt.Printf("  Allowed Tools: %s\n", strings.Join(runtime.AllowedTools, ","))
			fmt.Printf("  Permission Mode: %s\n", runtime.PermissionMode)
			fmt.Printf("  Live Output: %t\n", agentLiveOutputEnabled(runtime))
			fmt.Printf("  API Key: %s\n", maskAPIKey(runtime.APIKey))
		}

		return nil
	},
}

var configAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Set Claude agent runtime configuration interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := agentruntime.LoadConfig()
		if err != nil {
			cfg = &agentruntime.Config{
				DefaultRuntime: agentruntime.DefaultRuntimeName,
				AgentHome:      agentruntime.DefaultAgentHome(),
				Runtimes:       map[string]agentruntime.RuntimeConfig{},
			}
		}
		runtime := cfg.Runtimes[agentruntime.DefaultRuntimeName]
		if runtime.Type == "" {
			runtime.Type = "python_process"
		}
		if runtime.Command == "" {
			runtime.Command = "python"
		}
		if len(runtime.Args) == 0 {
			runtime.Args = []string{"internal/agentruntime/runners/claude_runner.py"}
		}
		if runtime.Timeout == 0 {
			runtime.Timeout = 120
		}
		if runtime.MaxTurns == 0 {
			runtime.MaxTurns = 8
		}
		if len(runtime.SettingSources) == 0 {
			runtime.SettingSources = []string{"project", "local", "user"}
		}

		fmt.Println("Configure Claude Agent Runtime")
		fmt.Println("==============================")

		agentHomePrompt := &survey.Input{
			Message: "Agent home:",
			Default: cfg.AgentHome,
		}
		if err := survey.AskOne(agentHomePrompt, &cfg.AgentHome, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		commandPrompt := &survey.Input{
			Message: "Python command or full path:",
			Default: runtime.Command,
		}
		if err := survey.AskOne(commandPrompt, &runtime.Command, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		baseURLPrompt := &survey.Input{
			Message: "Anthropic-compatible base URL:",
			Default: runtime.BaseURL,
		}
		if err := survey.AskOne(baseURLPrompt, &runtime.BaseURL); err != nil {
			return err
		}

		modelPrompt := &survey.Input{
			Message: "Model:",
			Default: firstNonEmpty(runtime.Model, "sonnet"),
		}
		if err := survey.AskOne(modelPrompt, &runtime.Model, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		maxTurnsPrompt := &survey.Input{
			Message: "Max agent turns:",
			Default: strconv.Itoa(runtime.MaxTurns),
			Help:    "Tool-using workflows need more than one turn; 8 is a safe default.",
		}
		var maxTurnsText string
		if err := survey.AskOne(maxTurnsPrompt, &maxTurnsText, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		maxTurns, err := strconv.Atoi(strings.TrimSpace(maxTurnsText))
		if err != nil || maxTurns <= 0 {
			return fmt.Errorf("max agent turns must be a positive integer")
		}
		runtime.MaxTurns = maxTurns

		settingsPrompt := &survey.Input{
			Message: "Claude settings path:",
			Default: runtime.Settings,
			Help:    "Optional path passed to ClaudeAgentOptions.settings.",
		}
		if err := survey.AskOne(settingsPrompt, &runtime.Settings); err != nil {
			return err
		}

		sourcesPrompt := &survey.Input{
			Message: "Setting sources:",
			Default: strings.Join(runtime.SettingSources, ","),
			Help:    "Comma-separated values such as project,local,user.",
		}
		var sources string
		if err := survey.AskOne(sourcesPrompt, &sources); err != nil {
			return err
		}
		runtime.SettingSources = splitCommaList(sources)

		skillsPrompt := &survey.Input{
			Message: "Claude SDK skills:",
			Default: strings.Join(runtime.SDKSkills, ","),
			Help:    "Optional comma-separated Claude SDK skill names; Novelgen stage skills are separate.",
		}
		var sdkSkills string
		if err := survey.AskOne(skillsPrompt, &sdkSkills); err != nil {
			return err
		}
		runtime.SDKSkills = splitCommaList(sdkSkills)

		liveOutput := agentLiveOutputEnabled(runtime)
		liveOutputPrompt := &survey.Confirm{
			Message: "Write Claude agent live output logs?",
			Default: liveOutput,
		}
		if err := survey.AskOne(liveOutputPrompt, &liveOutput); err != nil {
			return err
		}
		runtime.LiveOutput = &liveOutput

		apiKeyPrompt := &survey.Password{
			Message: "API key:",
		}
		var apiKey string
		if err := survey.AskOne(apiKeyPrompt, &apiKey); err != nil {
			return err
		}
		if strings.TrimSpace(apiKey) != "" {
			runtime.APIKey = apiKey
		}

		cfg.DefaultRuntime = agentruntime.DefaultRuntimeName
		cfg.Runtimes[agentruntime.DefaultRuntimeName] = runtime

		path := agentruntime.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		data, err := jsonMarshalIndent(cfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return fmt.Errorf("failed to save agent config: %w", err)
		}
		fmt.Printf("\nAgent runtime configuration saved to %s\n", path)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func jsonMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func agentLiveOutputEnabled(runtime agentruntime.RuntimeConfig) bool {
	if runtime.LiveOutput == nil {
		return true
	}
	return *runtime.LiveOutput
}

func agentMaxTurns(runtime agentruntime.RuntimeConfig) int {
	if runtime.MaxTurns == 0 {
		return 8
	}
	return runtime.MaxTurns
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configAgentCmd)
	// Register config command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return configCmd
	})
}
