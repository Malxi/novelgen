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

func TestInstallWorkbuddyMergesMCPAndUsesExistingGuidance(t *testing.T) {
	root := t.TempDir()
	binary := `D:\Tools\novelgen.exe`
	write := func(rel, body string) {
		t.Helper()
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".mcp.json", `{"mcpServers":{"other":{"command":"other.exe","args":["x"]}}}`)
	write("AGENTS.md", "# Existing guidance\n\nKeep this.\n")

	result, err := Install(root, []string{"workbuddy"}, Options{Apply: true, NovelgenPath: binary})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !result.OK || len(result.Warnings) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MCPServers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Cwd     string   `json:"cwd"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf(".mcp.json is not valid JSON: %v\n%s", err, data)
	}
	if _, ok := config.MCPServers["other"]; !ok {
		t.Fatalf("existing MCP server was dropped:\n%s", data)
	}
	novelgen, ok := config.MCPServers["novelgen"]
	if !ok || novelgen.Type != "stdio" || novelgen.Command != binary ||
		len(novelgen.Args) != 2 || novelgen.Args[0] != "mcp" || novelgen.Args[1] != "serve" ||
		novelgen.Cwd != "${CODEBUDDY_PROJECT_DIR}" {
		t.Fatalf("unexpected novelgen MCP entry: %+v", novelgen)
	}

	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "Keep this.") ||
		!strings.Contains(string(agents), "<!-- novelgen-workbuddy:begin -->") ||
		!strings.Contains(string(agents), "mcp__novelgen__") {
		t.Fatalf("AGENTS.md block not merged correctly:\n%s", agents)
	}
	if _, err := os.Stat(filepath.Join(root, "CODEBUDDY.md")); !os.IsNotExist(err) {
		t.Fatalf("CODEBUDDY.md should not shadow the existing AGENTS.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".codebuddy", "skills", "novelgen", "SKILL.md")); err != nil {
		t.Fatalf("project skill missing: %v", err)
	}

	second, err := Install(root, []string{"workbuddy"}, Options{Apply: true, NovelgenPath: binary})
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if action := changeAction(second.Files, ".mcp.json"); action != "unchanged" {
		t.Fatalf(".mcp.json action = %q, want unchanged", action)
	}
}

func TestInstallWorkbuddyCreatesGuidanceFile(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, []string{"workbuddy"}, Options{Apply: true, NovelgenPath: "novelgen"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "CODEBUDDY.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<!-- novelgen-workbuddy:begin -->") {
		t.Fatalf("CODEBUDDY.md missing managed block:\n%s", data)
	}
}

func TestInstallWorkbuddySkipMCPKeepsGuidance(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, []string{"workbuddy"}, Options{
		Apply:        true,
		SkipMCP:      true,
		NovelgenPath: "novelgen",
	}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("SkipMCP should not write .mcp.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "CODEBUDDY.md")); err != nil {
		t.Fatalf("guidance missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".codebuddy", "skills", "novelgen", "SKILL.md")); err != nil {
		t.Fatalf("project skill missing: %v", err)
	}
}

func TestInstallWorkbuddyUserSkill(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	if _, err := Install(root, []string{"workbuddy"}, Options{
		Apply:        true,
		SkipMCP:      true,
		UserSkill:    true,
		UserHome:     home,
		NovelgenPath: "novelgen",
	}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codebuddy", "skills", "novelgen", "SKILL.md")); err != nil {
		t.Fatalf("codebuddy user skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".workbuddy", "skills", "novelgen", "SKILL.md")); err != nil {
		t.Fatalf("workbuddy user skill missing: %v", err)
	}
}

func changeAction(files []FileChange, path string) string {
	for _, file := range files {
		if file.Path == path {
			return file.Action
		}
	}
	return ""
}
