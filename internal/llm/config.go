package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"novelgen/internal/agentruntime"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// ModelConfig represents configuration for a specific model
type ModelConfig struct {
	Name      string  `json:"name"`
	Context   int     `json:"context"`    // Model's context window size
	MaxTokens int     `json:"max_tokens"` // Max tokens for generation
	Temp      float32 `json:"temp"`       // Temperature for generation
}

// ProviderConfig represents configuration for a provider with multiple models
type ProviderConfig struct {
	Name         string                  `json:"name"`
	APIKey       string                  `json:"api_key"`
	BaseURL      string                  `json:"base_url"`
	AgentBaseURL string                  `json:"agent_base_url,omitempty"` // Anthropic-compatible base URL for the agent runtime
	Timeout      int                     `json:"timeout"`                  // seconds
	Models       map[string]*ModelConfig `json:"models"`                   // Map of model name to config
}

// Config represents the LLM configuration with multiple providers
type Config struct {
	Providers       map[string]*ProviderConfig `json:"providers"`
	DefaultProvider string                     `json:"default_provider"`
	DefaultModel    string                     `json:"default_model"`
}

// DefaultConfig returns a default configuration for local Ollama
func DefaultConfig() *Config {
	return &Config{
		Providers: map[string]*ProviderConfig{
			"claude": defaultClaudeProvider(),
			"ollama": {
				Name:    "ollama",
				APIKey:  "local-llama",
				BaseURL: "http://127.0.0.1:11434/v1",
				Timeout: 120,
				Models: map[string]*ModelConfig{
					"qwen3.5:4b": {
						Name:      "qwen3.5:4b",
						Context:   32000,
						MaxTokens: 8000,
						Temp:      0.8,
					},
				},
			},
		},
		DefaultProvider: "claude",
		DefaultModel:    "sonnet",
	}
}

func defaultClaudeProvider() *ProviderConfig {
	return &ProviderConfig{
		Name:    "claude",
		APIKey:  "",
		BaseURL: "",
		Timeout: 120,
		Models: map[string]*ModelConfig{
			"sonnet": {
				Name:      "sonnet",
				Context:   200000,
				MaxTokens: 8000,
				Temp:      0.8,
			},
		},
	}
}

func init() {
	agentruntime.SetProviderResolver(resolveProviderCredentials)
}

// Provider returns the named provider, matching case-insensitively.
func (c *Config) Provider(name string) *ProviderConfig {
	if c == nil || name == "" {
		return nil
	}
	if provider, ok := c.Providers[name]; ok {
		return provider
	}
	for key, provider := range c.Providers {
		if strings.EqualFold(key, name) {
			return provider
		}
	}
	return nil
}

// resolveProviderCredentials maps an llm_config.json provider into agent
// runtime credentials. Claude Code appends /v1/messages itself, so an
// OpenAI-style trailing /v1 is stripped from base_url unless the provider pins
// agent_base_url explicitly.
func resolveProviderCredentials(providerName string) (agentruntime.ProviderCredentials, error) {
	return resolveProviderCredentialsFrom(GetConfigPath(), providerName)
}

func resolveProviderCredentialsFrom(path, providerName string) (agentruntime.ProviderCredentials, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return agentruntime.ProviderCredentials{}, err
	}
	provider := cfg.Provider(providerName)
	if provider == nil {
		return agentruntime.ProviderCredentials{}, fmt.Errorf("provider %q not found in %s", providerName, path)
	}
	return providerCredentials(provider), nil
}

func providerCredentials(provider *ProviderConfig) agentruntime.ProviderCredentials {
	base := strings.TrimSpace(provider.AgentBaseURL)
	if base == "" {
		base = anthropicBaseURL(provider.BaseURL)
	}
	return agentruntime.ProviderCredentials{
		APIKey:  strings.TrimSpace(provider.APIKey),
		BaseURL: base,
		Model:   firstProviderModel(provider.Models),
		Timeout: provider.Timeout,
	}
}

func anthropicBaseURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	return base
}

func firstProviderModel(models map[string]*ModelConfig) string {
	names := make([]string, 0, len(models))
	for name := range models {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// Save writes the config to a file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadConfig reads the config from a file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigPath returns the path to the config file
// Priority: global config > local config
func GetConfigPath() string {
	// First check for global config in user's home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfigPath := filepath.Join(homeDir, ".novelgen", "llm_config.json")
		if _, err := os.Stat(globalConfigPath); err == nil {
			return globalConfigPath
		}
	}

	// Then check for local config
	if _, err := os.Stat("llm_config.json"); err == nil {
		return "llm_config.json"
	}

	// Return global path as default (for error messages)
	if homeDir != "" {
		return filepath.Join(homeDir, ".novelgen", "llm_config.json")
	}
	return "llm_config.json"
}

// LoadOrCreateConfig loads the config or returns an error if not found
// It will NOT automatically create a config file - user must configure it manually
func LoadOrCreateConfig() (*Config, error) {
	path := GetConfigPath()

	if _, err := os.Stat(path); err == nil {
		return LoadConfig(path)
	}
	if agentruntime.Exists() {
		return DefaultConfig(), nil
	}

	// Config not found - provide helpful error message
	homeDir, _ := os.UserHomeDir()
	globalConfigPath := filepath.Join(homeDir, ".novelgen", "llm_config.json")

	return nil, fmt.Errorf("AI configuration not found.\n\n"+
		"For the default Claude agent runtime, create:\n"+
		"  1. Agent:  %s\n"+
		"     Run: novelgen config agent\n\n"+
		"For legacy OpenAI-compatible providers, create one of:\n"+
		"  2. Global: %s\n"+
		"  3. Local:  llm_config.json (in current directory)\n\n"+
		"Example legacy LLM configuration:\n"+
		"{\n"+
		"  \"providers\": {\n"+
		"    \"ollama\": {\n"+
		"      \"name\": \"ollama\",\n"+
		"      \"api_key\": \"local-llama\",\n"+
		"      \"base_url\": \"http://127.0.0.1:11434/v1\",\n"+
		"      \"timeout\": 120,\n"+
		"      \"models\": {\n"+
		"        \"qwen3.5:4b\": {\n"+
		"          \"name\": \"qwen3.5:4b\",\n"+
		"          \"context\": 32000,\n"+
		"          \"max_tokens\": 8000,\n"+
		"          \"temp\": 0.8\n"+
		"        }\n"+
		"      }\n"+
		"    }\n"+
		"  },\n"+
		"  \"default_provider\": \"ollama\",\n"+
		"  \"default_model\": \"qwen3.5:4b\"\n"+
		"}", agentruntime.ConfigPath(), globalConfigPath)
}

// GetActiveProvider returns the active provider config based on project settings
func (c *Config) GetActiveProvider(projectLLM *models.ProjectLLM) *ProviderConfig {
	log := logger.GetLogger()
	providerName := c.DefaultProvider
	if projectLLM.Provider != "" {
		providerName = projectLLM.Provider
	}

	log.Debug("Looking for provider: %s", providerName)

	if provider, ok := c.Providers[providerName]; ok {
		log.Debug("Found provider: %s", providerName)
		return provider
	}
	if strings.EqualFold(providerName, "claude") {
		log.Debug("Using built-in claude provider")
		return defaultClaudeProvider()
	}
	// Fall back to default provider
	log.Warn("Provider not found: %s, falling back to default: %s", providerName, c.DefaultProvider)
	if provider, ok := c.Providers[c.DefaultProvider]; ok {
		return provider
	}
	return nil
}

// GetActiveModel returns the active model config based on project settings
func (c *Config) GetActiveModel(projectLLM *models.ProjectLLM) (*ProviderConfig, *ModelConfig) {
	log := logger.GetLogger()
	provider := c.GetActiveProvider(projectLLM)
	if provider == nil {
		log.Error("No provider available")
		return nil, nil
	}

	modelName := c.DefaultModel
	if projectLLM.Model != "" {
		modelName = projectLLM.Model
	}

	log.Info("Looking for model: %s in provider: %s", modelName, provider.Name)

	if model, ok := provider.Models[modelName]; ok {
		log.Info("Using model: %s (provider: %s)", modelName, provider.Name)
		return provider, model
	}
	if strings.EqualFold(provider.Name, "claude") && modelName != "" {
		log.Info("Using claude runtime model: %s", modelName)
		return provider, &ModelConfig{
			Name:      modelName,
			Context:   200000,
			MaxTokens: 8000,
			Temp:      0.8,
		}
	}

	// Fall back to first available model
	log.Warn("Model not found: %s, falling back to first available model", modelName)
	for _, model := range provider.Models {
		log.Warn("Using fallback model: %s (provider: %s)", model.Name, provider.Name)
		return provider, model
	}

	return provider, nil
}

// CreateClient creates an LLM client from the config based on project settings
func (c *Config) CreateClient(projectLLM *models.ProjectLLM) Client {
	provider, model := c.GetActiveModel(projectLLM)
	if provider == nil || model == nil {
		return nil
	}
	if strings.EqualFold(provider.Name, "claude") {
		runtime, err := newAgentRuntime(provider.Name)
		return NewRuntimeBackedClient(provider.Name, runtime, err)
	}

	return NewOpenAIClient(&OpenAIConfig{
		APIKey:  provider.APIKey,
		BaseURL: provider.BaseURL,
		Model:   model.Name,
		Timeout: provider.Timeout,
	})
}

func newAgentRuntime(providerName string) (agentruntime.Runtime, error) {
	cfg, err := agentruntime.LoadConfig()
	if err != nil {
		return nil, err
	}
	runtime, err := cfg.NewRuntime(providerName)
	if err == nil {
		return runtime, nil
	}
	return cfg.NewRuntime(cfg.DefaultRuntime)
}

// GetChatOptions returns ChatOptions from the config based on project settings
func (c *Config) GetChatOptions(projectLLM *models.ProjectLLM) *ChatOptions {
	_, model := c.GetActiveModel(projectLLM)
	if model == nil {
		return &ChatOptions{
			Temperature: 0.8,
			MaxTokens:   8000,
			Model:       "",
		}
	}

	return &ChatOptions{
		Temperature: float64(model.Temp),
		MaxTokens:   model.MaxTokens,
		Model:       model.Name,
	}
}

// GetActiveConfig loads llm_config.json and returns the active configuration
// based on the project's provider and model selection
func GetActiveConfig(projectLLM *models.ProjectLLM) (*ProviderConfig, *ModelConfig, error) {
	cfg, err := LoadOrCreateConfig()
	if err != nil {
		return nil, nil, err
	}

	provider, model := cfg.GetActiveModel(projectLLM)
	if provider == nil {
		return nil, nil, fmt.Errorf("provider not found: %s", projectLLM.Provider)
	}
	if model == nil {
		return nil, nil, fmt.Errorf("model not found: %s", projectLLM.Model)
	}

	return provider, model, nil
}
