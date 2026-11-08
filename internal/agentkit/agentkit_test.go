package agentkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallClaudeAndCodex(t *testing.T) {
	root := t.TempDir()
	result, err := Install(root, []string{"claude", "codex"}, Options{Apply: true, AllowHooks: true})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !result.OK {
		t.Fatalf("result not ok: %+v", result)
	}
	for _, rel := range []string{
		".novelgen/workflows/setup.md",
		".novelgen/adapters/claude/.claude-plugin/marketplace.json",
		".novelgen/adapters/claude/plugin/.claude-plugin/plugin.json",
		".novelgen/adapters/claude/plugin/.mcp.json",
		".novelgen/adapters/claude/plugin/hooks/hooks.json",
		".novelgen/adapters/claude/plugin/commands/novelgen-next.md",
		".novelgen/adapters/codex/.codex/config.toml",
		".codex/config.toml",
		".codex/skills/novelgen/SKILL.md",
		"AGENTS.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "<!-- novelgen:begin -->") ||
		!strings.Contains(string(agents), "novelgen tool replace") {
		t.Fatalf("AGENTS.md missing managed block:\n%s", agents)
	}
}

func TestInstallDryRunAndHookGate(t *testing.T) {
	root := t.TempDir()
	result, err := Install(root, []string{"claude"}, Options{Apply: false, AllowHooks: false})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if result.OK == false {
		t.Fatal("dry-run result not ok")
	}
	if _, err := os.Stat(filepath.Join(root, ".novelgen", "adapters", "claude", "plugin")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote files: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected a hook warning")
	}

	if _, err := Install(root, []string{"claude"}, Options{Apply: true, AllowHooks: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".novelgen", "adapters", "claude", "plugin", "hooks", "hooks.json")); err != nil {
		t.Fatalf("hooks not installed: %v", err)
	}
}

func TestInstallCodexHome(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	if _, err := Install(root, []string{"codex"}, Options{Apply: true, CodexHome: home}); err != nil {
		t.Fatalf("install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[mcp_servers.novelgen]") ||
		!strings.Contains(string(data), "mcp\", \"serve") {
		t.Fatalf("codex config:\n%s", data)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, []string{"codex"}, Options{Apply: true}); err != nil {
		t.Fatalf("first: %v", err)
	}
	agentsBefore, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	result, err := Install(root, []string{"codex"}, Options{Apply: true})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	agentsAfter, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(agentsBefore) != string(agentsAfter) {
		t.Fatal("managed AGENTS block changed on second install")
	}
	for _, file := range result.Files {
		if file.Path == "AGENTS.md" && file.Action != "update" {
			t.Fatalf("unexpected AGENTS action: %+v", file)
		}
	}
}

func TestInstallRendersValidAgentConfigs(t *testing.T) {
	root := t.TempDir()
	binary := `D:\Tools\novelgen.exe`
	if _, err := Install(root, []string{"claude", "codex"}, Options{
		Apply:        true,
		AllowHooks:   true,
		NovelgenPath: binary,
	}); err != nil {
		t.Fatalf("install: %v", err)
	}

	mcpData, err := os.ReadFile(filepath.Join(root, ".novelgen", "adapters", "claude", "plugin", ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mcp struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(mcpData, &mcp); err != nil {
		t.Fatalf("rendered .mcp.json is not valid JSON: %v\n%s", err, mcpData)
	}
	entry, ok := mcp.MCPServers["novelgen"]
	if !ok || entry.Command != binary || len(entry.Args) != 2 || entry.Args[0] != "mcp" || entry.Args[1] != "serve" {
		t.Fatalf("unexpected .mcp.json entry: %+v", mcp.MCPServers)
	}

	hooksData, err := os.ReadFile(filepath.Join(root, ".novelgen", "adapters", "claude", "plugin", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hooks struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(hooksData, &hooks); err != nil {
		t.Fatalf("rendered hooks.json is not valid JSON: %v\n%s", err, hooksData)
	}
	if len(hooks.Hooks["PreToolUse"]) == 0 || len(hooks.Hooks["PreToolUse"][0].Hooks) == 0 {
		t.Fatalf("PreToolUse hook missing: %s", hooksData)
	}
	command := hooks.Hooks["PreToolUse"][0].Hooks[0].Command
	if !strings.Contains(command, binary) || !strings.Contains(command, "hook guard-write") {
		t.Fatalf("unexpected PreToolUse command %q", command)
	}

	codexConfig, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codexConfig), `command = "D:\\Tools\\novelgen.exe"`) {
		t.Fatalf("codex config did not escape the Windows path:\n%s", codexConfig)
	}
}
