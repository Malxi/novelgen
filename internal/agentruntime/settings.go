package agentruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// claudeFlagSettings mirrors the subset of Claude Code settings that the agent
// runtime needs as the "flag settings" layer (the highest-priority
// user-controlled settings layer, equivalent to `claude --settings <path>`).
type claudeFlagSettings struct {
	Env map[string]string `json:"env"`
}

// generatedSettingsEnvKeys are always emitted when auto-generating runtime
// settings. The runner intentionally does not emit ANTHROPIC_AUTH_TOKEN:
// Claude Code sends that variable as `Authorization: Bearer`, which wins over
// ANTHROPIC_API_KEY (`x-api-key`) and breaks x-api-key-only endpoints.
var generatedSettingsEnvKeys = []string{
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"CLAUDE_CODE_SUBAGENT_MODEL",
}

// ensureRuntimeSettings returns rc with Settings pointing at an auto-generated
// Claude flag-settings file whenever the runtime carries concrete
// Anthropic-compatible credentials and no explicit settings path was given.
func (c *Config) ensureRuntimeSettings(name string, rc RuntimeConfig) (RuntimeConfig, error) {
	if strings.TrimSpace(rc.Settings) != "" {
		return rc, nil
	}
	if strings.TrimSpace(rc.BaseURL) == "" || strings.TrimSpace(rc.APIKey) == "" || strings.TrimSpace(rc.Model) == "" {
		return rc, nil
	}

	env := make(map[string]string, len(generatedSettingsEnvKeys)+len(rc.Env))
	for _, key := range generatedSettingsEnvKeys {
		switch key {
		case "ANTHROPIC_BASE_URL":
			env[key] = strings.TrimSpace(rc.BaseURL)
		case "ANTHROPIC_API_KEY":
			env[key] = strings.TrimSpace(rc.APIKey)
		default:
			env[key] = strings.TrimSpace(rc.Model)
		}
	}
	// Runtime Env entries are merged into the flag settings too, but the
	// bearer-token form is never generated (see generatedSettingsEnvKeys).
	for key, value := range rc.Env {
		if strings.EqualFold(key, "ANTHROPIC_AUTH_TOKEN") {
			continue
		}
		env[key] = value
	}

	settingsDir := filepath.Join(expandHome(c.AgentHome), "settings")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return rc, fmt.Errorf("failed to create generated settings directory: %w", err)
	}
	path := filepath.Join(settingsDir, sanitizeLogName(name)+".json")
	data, err := json.MarshalIndent(claudeFlagSettings{Env: env}, "", "  ")
	if err != nil {
		return rc, fmt.Errorf("failed to marshal generated Claude settings: %w", err)
	}
	data = append(data, '\n')
	if current, err := os.ReadFile(path); err == nil && string(current) == string(data) {
		rc.Settings = path
		return rc, nil
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return rc, fmt.Errorf("failed to write generated Claude settings %s: %w", path, err)
	}
	rc.Settings = path
	return rc, nil
}
