package llm

import (
	"encoding/json"
	"os"
	"path/filepath"

	"nolvegen/internal/models"
)

// Config represents the LLM configuration
type Config struct {
	Provider  string  `json:"provider"` // "openai", "ollama", etc.
	APIKey    string  `json:"api_key"`
	BaseURL   string  `json:"base_url"`
	Model     string  `json:"model"`
	Timeout   int     `json:"timeout"`    // seconds
	MaxTokens int     `json:"max_tokens"` // max tokens for generation
	Temp      float32 `json:"temp"`       // temperature for generation
}

// DefaultConfig returns a default configuration for local Ollama
func DefaultConfig() *Config {
	return &Config{
		Provider:  "ollama",
		APIKey:    "local-llama",
		BaseURL:   "http://127.0.0.1:11434/v1",
		Model:     "qwen3.5:4b",
		Timeout:   120,
		MaxTokens: 50000,
		Temp:      0.8,
	}
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
func GetConfigPath() string {
	// First check for local config
	if _, err := os.Stat("llm_config.json"); err == nil {
		return "llm_config.json"
	}

	// Then check for global config in user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "llm_config.json"
	}

	globalConfigPath := filepath.Join(homeDir, ".nolvegen", "llm_config.json")
	if _, err := os.Stat(globalConfigPath); err == nil {
		return globalConfigPath
	}

	return "llm_config.json"
}

// LoadOrCreateConfig loads the config or creates a default one
func LoadOrCreateConfig() (*Config, error) {
	path := GetConfigPath()

	if _, err := os.Stat(path); err == nil {
		return LoadConfig(path)
	}

	// Create default config
	config := DefaultConfig()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return config, err
		}
	}

	// Save default config
	if err := config.Save(path); err != nil {
		return config, err
	}

	return config, nil
}

// CreateClient creates an LLM client from the config
func (c *Config) CreateClient() Client {
	return NewOpenAIClient(&OpenAIConfig{
		APIKey:  c.APIKey,
		BaseURL: c.BaseURL,
		Model:   c.Model,
		Timeout: c.Timeout,
	})
}

// GetChatOptions returns ChatOptions from the config
func (c *Config) GetChatOptions() *ChatOptions {
	return &ChatOptions{
		Temperature: float64(c.Temp),
		MaxTokens:   c.MaxTokens,
		Model:       c.Model,
	}
}

// ConfigFromProjectConfig creates LLM Config from ProjectConfig LLM settings
func ConfigFromProjectConfig(llmConfig *models.LLMConfig) *Config {
	return &Config{
		Provider:  "ollama", // Default provider
		APIKey:    "local-llama",
		BaseURL:   "http://127.0.0.1:11434/v1",
		Model:     llmConfig.Model,
		Timeout:   120,
		MaxTokens: llmConfig.MaxTokens,
		Temp:      0.8,
	}
}
