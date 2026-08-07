package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"novelgen/internal/agentruntime"
)

func TestAnthropicBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://opencode.ai/zen/go/v1":      "https://opencode.ai/zen/go",
		"https://opencode.ai/zen/go/v1/":     "https://opencode.ai/zen/go",
		"https://api.deepseek.com":           "https://api.deepseek.com",
		"https://api.deepseek.com/":          "https://api.deepseek.com",
		"https://api.deepseek.com/anthropic": "https://api.deepseek.com/anthropic",
		"":                                   "",
	}
	for in, want := range cases {
		if got := anthropicBaseURL(in); got != want {
			t.Fatalf("anthropicBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProviderCredentialsUsesAgentBaseURLOverride(t *testing.T) {
	provider := &ProviderConfig{
		Name:         "opencode",
		APIKey:       "key",
		BaseURL:      "https://opencode.ai/zen/go/v1",
		AgentBaseURL: "https://custom.example/zen",
		Timeout:      1800,
		Models: map[string]*ModelConfig{
			"deepseek-v4-pro":   {Name: "deepseek-v4-pro"},
			"deepseek-v4-flash": {Name: "deepseek-v4-flash"},
		},
	}
	creds := providerCredentials(provider)
	if creds.BaseURL != "https://custom.example/zen" {
		t.Fatalf("BaseURL = %q, want agent_base_url override", creds.BaseURL)
	}
	if creds.APIKey != "key" || creds.Timeout != 1800 {
		t.Fatalf("creds = %+v", creds)
	}
	if creds.Model != "deepseek-v4-flash" {
		t.Fatalf("Model = %q, want first sorted model", creds.Model)
	}
}

func TestProviderCredentialsStripsTrailingV1(t *testing.T) {
	provider := &ProviderConfig{
		Name:    "opencode",
		APIKey:  "key",
		BaseURL: "https://opencode.ai/zen/go/v1",
		Models:  map[string]*ModelConfig{"deepseek-v4-flash": {Name: "deepseek-v4-flash"}},
	}
	creds := providerCredentials(provider)
	if creds.BaseURL != "https://opencode.ai/zen/go" {
		t.Fatalf("BaseURL = %q, want trailing /v1 stripped", creds.BaseURL)
	}
}

func TestResolveProviderCredentialsFrom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm_config.json")
	cfg := &Config{
		Providers: map[string]*ProviderConfig{
			"opencode": {
				Name:    "opencode",
				APIKey:  "provider-key",
				BaseURL: "https://opencode.ai/zen/go/v1",
				Timeout: 1800,
				Models:  map[string]*ModelConfig{"deepseek-v4-flash": {Name: "deepseek-v4-flash"}},
			},
		},
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	creds, err := resolveProviderCredentialsFrom(path, "opencode")
	if err != nil {
		t.Fatalf("resolveProviderCredentialsFrom: %v", err)
	}
	if creds.APIKey != "provider-key" ||
		creds.BaseURL != "https://opencode.ai/zen/go" ||
		creds.Model != "deepseek-v4-flash" ||
		creds.Timeout != 1800 {
		t.Fatalf("creds = %+v", creds)
	}
	if _, err := resolveProviderCredentialsFrom(path, "missing"); err == nil {
		t.Fatalf("resolveProviderCredentialsFrom with missing provider should fail")
	}
}

func TestConfigProviderCaseInsensitive(t *testing.T) {
	cfg := &Config{Providers: map[string]*ProviderConfig{"Opencode": {Name: "Opencode"}}}
	if got := cfg.Provider("opencode"); got == nil || got.Name != "Opencode" {
		t.Fatalf("case-insensitive provider lookup failed")
	}
	if cfg.Provider("nope") != nil {
		t.Fatalf("missing provider should return nil")
	}
}

// TestAgentRuntimeFromRealProvider exercises the full provider-resolution
// chain against the real user config: agent_config.json -> llm_config.json
// provider -> auto-generated Claude flag settings. Skipped by default because
// it reads ~/.novelgen; run with NOVELGEN_TEST_REAL_CONFIG=1 to verify a
// machine's actual setup.
func TestAgentRuntimeFromRealProvider(t *testing.T) {
	if os.Getenv("NOVELGEN_TEST_REAL_CONFIG") == "" {
		t.Skip("set NOVELGEN_TEST_REAL_CONFIG=1 to run against the real user config")
	}
	cfg, err := agentruntime.LoadConfig()
	if err != nil {
		t.Fatalf("agentruntime.LoadConfig() = %v", err)
	}
	runtime, err := cfg.NewRuntime("")
	if err != nil {
		t.Fatalf("cfg.NewRuntime() = %v", err)
	}
	processRuntime, ok := runtime.(*agentruntime.ProcessRuntime)
	if !ok {
		t.Fatalf("runtime type = %T", runtime)
	}
	if processRuntime.Config().APIKey == "" {
		t.Fatalf("runtime API key is empty; provider was not resolved")
	}
	if processRuntime.Config().Settings == "" {
		t.Fatalf("runtime settings were not auto-generated")
	}
	data, err := os.ReadFile(processRuntime.Config().Settings)
	if err != nil {
		t.Fatalf("read generated settings: %v", err)
	}
	t.Logf("resolved base_url=%s model=%s timeout=%d settings=%s",
		processRuntime.Config().BaseURL, processRuntime.Config().Model, processRuntime.Config().Timeout, processRuntime.Config().Settings)
	var settings struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse generated settings: %v", err)
	}
	keys := make([]string, 0, len(settings.Env))
	for key := range settings.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	t.Logf("generated settings env keys: %v", keys)
	if _, ok := settings.Env["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Fatalf("generated settings must not contain ANTHROPIC_AUTH_TOKEN")
	}
	if got := settings.Env["ANTHROPIC_MODEL"]; got != processRuntime.Config().Model {
		t.Fatalf("generated ANTHROPIC_MODEL = %q, want %q", got, processRuntime.Config().Model)
	}
}
