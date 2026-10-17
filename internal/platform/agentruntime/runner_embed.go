package agentruntime

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed runners/claude_runner.py
var embeddedClaudeRunner []byte

//go:embed skills/*/SKILL.md
var embeddedSkillFiles embed.FS

// EmbeddedSkillNames returns the workflow skill names bundled into this build.
func EmbeddedSkillNames() ([]string, error) {
	matches, err := embeddedSkillFiles.ReadDir("skills")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded skills: %w", err)
	}
	names := make([]string, 0, len(matches))
	for _, entry := range matches {
		if entry.IsDir() && strings.TrimSpace(entry.Name()) != "" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func resolveClaudeRunnerPath(agentHome, configured string) (string, error) {
	configured = expandHome(configured)
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
	}

	targetHome := expandHome(agentHome)
	if targetHome == "" {
		targetHome = DefaultAgentHome()
	}
	target := filepath.Join(targetHome, "runners", "claude_runner.py")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", fmt.Errorf("failed to create embedded runner directory: %w", err)
	}
	if current, err := os.ReadFile(target); err == nil && string(current) == string(embeddedClaudeRunner) {
		return target, nil
	}
	if err := os.WriteFile(target, embeddedClaudeRunner, 0644); err != nil {
		return "", fmt.Errorf("failed to write embedded Claude runner: %w", err)
	}
	return target, nil
}

func isClaudeRunnerArg(arg string) bool {
	return filepath.Base(expandHome(arg)) == "claude_runner.py"
}

func materializeEmbeddedSkills(agentHome string) (string, error) {
	targetHome := expandHome(agentHome)
	if targetHome == "" {
		targetHome = DefaultAgentHome()
	}
	targetRoot := filepath.Join(targetHome, "skills")
	matches, err := embeddedSkillFiles.ReadDir("skills")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded skills: %w", err)
	}
	for _, entry := range matches {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		content, err := embeddedSkillFiles.ReadFile(filepath.ToSlash(filepath.Join("skills", name, "SKILL.md")))
		if err != nil {
			return "", fmt.Errorf("failed to read embedded skill %s: %w", name, err)
		}
		target := filepath.Join(targetRoot, name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", fmt.Errorf("failed to create embedded skill directory: %w", err)
		}
		if current, err := os.ReadFile(target); err == nil && string(current) == string(content) {
			continue
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			return "", fmt.Errorf("failed to write embedded skill %s: %w", name, err)
		}
	}
	return targetRoot, nil
}
