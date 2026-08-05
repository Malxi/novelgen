package agentruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	DefaultRuntimeName = "claude"
	defaultAgentDir    = ".novelgen"
)

var (
	ansiEscapePattern       = regexp.MustCompile("\x1b\\[[0-9;]*m")
	sgrResidueSuffixPattern = regexp.MustCompile(`(?:\[[0-9;]*m\])+$`)
)

// Config is stored at ~/.novelgen/agent_config.json.
type Config struct {
	DefaultRuntime string                   `json:"default_runtime"`
	AgentHome      string                   `json:"agent_home"`
	Runtimes       map[string]RuntimeConfig `json:"runtimes"`
}

// RuntimeConfig describes one concrete backend runner.
type RuntimeConfig struct {
	Type           string            `json:"type"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	APIKey         string            `json:"api_key,omitempty"`
	BaseURL        string            `json:"base_url,omitempty"`
	Model          string            `json:"model,omitempty"`
	Timeout        int               `json:"timeout,omitempty"`
	MaxTurns       int               `json:"max_turns,omitempty"`
	Settings       string            `json:"settings,omitempty"`
	SettingSources []string          `json:"setting_sources,omitempty"`
	SDKSkills      []string          `json:"sdk_skills,omitempty"`
	AddDirs        []string          `json:"add_dirs,omitempty"`
	Tools          []string          `json:"tools,omitempty"`
	AllowedTools   []string          `json:"allowed_tools,omitempty"`
	PermissionMode string            `json:"permission_mode,omitempty"`
	LiveOutput     *bool             `json:"live_output,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
}

// ConfigPath returns the canonical global agent config path.
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "agent_config.json"
	}
	return filepath.Join(home, defaultAgentDir, "agent_config.json")
}

// DefaultAgentHome returns ~/.novelgen/agents.
func DefaultAgentHome() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(defaultAgentDir, "agents")
	}
	return filepath.Join(home, defaultAgentDir, "agents")
}

// LoadConfig loads ~/.novelgen/agent_config.json. If it does not exist, it can
// derive a Claude-compatible runtime from ~/.claude/settings.json.
func LoadConfig() (*Config, error) {
	path := ConfigPath()
	if data, err := os.ReadFile(path); err == nil {
		data = trimUTF8BOM(data)
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		cfg.normalize()
		return &cfg, nil
	}

	if cfg, err := loadFromClaudeSettings(); err == nil {
		cfg.normalize()
		return cfg, nil
	}

	return nil, os.ErrNotExist
}

// Exists reports whether an agent runtime config or Claude settings file exists.
func Exists() bool {
	if _, err := os.Stat(ConfigPath()); err == nil {
		return true
	}
	if _, err := os.Stat(claudeSettingsPath()); err == nil {
		return true
	}
	return false
}

// NewRuntime creates the configured runtime.
func (c *Config) NewRuntime(name string) (Runtime, error) {
	if c == nil {
		return nil, errors.New("agent runtime config is nil")
	}
	c.normalize()
	if strings.TrimSpace(name) == "" {
		name = c.DefaultRuntime
	}
	rc, ok := c.Runtimes[name]
	if !ok {
		return nil, fmt.Errorf("agent runtime %q not found", name)
	}
	switch strings.TrimSpace(rc.Type) {
	case "", "python_process", "process":
		return NewProcessRuntime(c.AgentHome, rc)
	default:
		return nil, fmt.Errorf("unsupported agent runtime type %q", rc.Type)
	}
}

func (c *Config) normalize() {
	if strings.TrimSpace(c.DefaultRuntime) == "" {
		c.DefaultRuntime = DefaultRuntimeName
	}
	if strings.TrimSpace(c.AgentHome) == "" {
		c.AgentHome = DefaultAgentHome()
	}
	c.AgentHome = expandHome(c.AgentHome)
	if c.Runtimes == nil {
		c.Runtimes = map[string]RuntimeConfig{}
	}
	if _, ok := c.Runtimes[DefaultRuntimeName]; !ok {
		c.Runtimes[DefaultRuntimeName] = RuntimeConfig{
			Type:           "python_process",
			Command:        defaultPythonCommand(),
			Args:           []string{defaultClaudeRunnerPath()},
			Model:          "sonnet",
			Timeout:        120,
			MaxTurns:       8,
			SettingSources: []string{"project", "local", "user"},
		}
	}
	for name, runtimeConfig := range c.Runtimes {
		runtimeConfig.Model = sanitizeModelName(runtimeConfig.Model)
		for _, key := range []string{
			"ANTHROPIC_MODEL",
			"ANTHROPIC_DEFAULT_OPUS_MODEL",
			"ANTHROPIC_DEFAULT_SONNET_MODEL",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL",
			"CLAUDE_CODE_SUBAGENT_MODEL",
		} {
			if runtimeConfig.Env != nil {
				runtimeConfig.Env[key] = sanitizeModelName(runtimeConfig.Env[key])
			}
		}
		c.Runtimes[name] = runtimeConfig
	}
}

func loadFromClaudeSettings() (*Config, error) {
	data, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		return nil, err
	}
	data = trimUTF8BOM(data)
	var raw struct {
		Env   map[string]string `json:"env"`
		Model string            `json:"model"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	model := firstNonEmpty(raw.Env["ANTHROPIC_MODEL"], raw.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"], raw.Model, "sonnet")
	return &Config{
		DefaultRuntime: DefaultRuntimeName,
		AgentHome:      DefaultAgentHome(),
		Runtimes: map[string]RuntimeConfig{
			DefaultRuntimeName: {
				Type:           "python_process",
				Command:        defaultPythonCommand(),
				Args:           []string{defaultClaudeRunnerPath()},
				APIKey:         raw.Env["ANTHROPIC_AUTH_TOKEN"],
				BaseURL:        raw.Env["ANTHROPIC_BASE_URL"],
				Model:          model,
				Timeout:        120,
				MaxTurns:       8,
				SettingSources: []string{"project", "local", "user"},
				Env:            raw.Env,
			},
		},
	}, nil
}

func claudeSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".claude", "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func defaultClaudeRunnerPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("internal", "agentruntime", "runners", "claude_runner.py")
	}
	return filepath.Join(filepath.Dir(filename), "runners", "claude_runner.py")
}

func defaultPythonCommand() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~"+string(os.PathSeparator)) || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
		}
	}
	return path
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return sanitizeModelName(value)
		}
	}
	return ""
}

func sanitizeModelName(value string) string {
	value = strings.TrimSpace(value)
	value = ansiEscapePattern.ReplaceAllString(value, "")
	value = sgrResidueSuffixPattern.ReplaceAllString(value, "")
	return strings.TrimSpace(value)
}

func trimUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}
