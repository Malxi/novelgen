package agents

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"novelgen/internal/logger"
)

// embeddedSkills carries a copy of every agent skill inside the binary. Without
// it a released executable could only work from the source tree it was built
// in, because the loader used to resolve skills through runtime.Caller.
//
//go:embed skills/*/SKILL.md
var embeddedSkills embed.FS

// embeddedSkillWarning makes sure the fallback is reported once per process
// instead of once per agent invocation.
var embeddedSkillWarning sync.Once

// SkillLoader loads skill definitions from SKILL.md files
type SkillLoader struct {
	skillsDir string
	mu        sync.RWMutex
	cache     map[string]string
}

// NewSkillLoader creates a new skill loader
func NewSkillLoader(skillsDir string) *SkillLoader {
	return &SkillLoader{
		skillsDir: skillsDir,
		cache:     make(map[string]string),
	}
}

// Load loads a skill's system prompt from its SKILL.md file
func (sl *SkillLoader) Load(skillName string) (string, error) {
	// Check cache first
	sl.mu.RLock()
	if prompt, ok := sl.cache[skillName]; ok {
		sl.mu.RUnlock()
		return prompt, nil
	}
	sl.mu.RUnlock()

	content, source, err := loadSkillSource(sl.skillsDir, skillName)
	if err != nil {
		return "", fmt.Errorf("failed to load skill %s: %w", skillName, err)
	}
	// The entire file is treated as the system prompt.
	prompt := string(content)
	if strings.HasPrefix(source, "embedded:") {
		embeddedSkillWarning.Do(func() {
			logger.Warn("Agent skills are being served from the binary's embedded copy because the skills directory %q is unavailable; using embedded skill %q", sl.skillsDir, skillName)
		})
	}

	// Cache it
	sl.mu.Lock()
	sl.cache[skillName] = prompt
	sl.mu.Unlock()

	return prompt, nil
}

// loadSkillSource reads a skill from the skills directory, falling back to the
// embedded copy when the directory is missing or unreadable.
func loadSkillSource(skillsDir, skillName string) ([]byte, string, error) {
	name := strings.TrimSpace(skillName)
	if name == "" {
		return nil, "", fmt.Errorf("empty skill name")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil, "", fmt.Errorf("invalid skill name %q", skillName)
	}

	if dir := strings.TrimSpace(skillsDir); dir != "" {
		path := filepath.Join(dir, name, "SKILL.md")
		if data, err := os.ReadFile(path); err == nil {
			return data, path, nil
		}
	}

	data, err := embeddedSkills.ReadFile("skills/" + name + "/SKILL.md")
	if err != nil {
		return nil, "", fmt.Errorf("skill %q not found in skills directory or embedded skills", name)
	}
	return data, "embedded:" + name, nil
}

// LoadWithVars loads a skill and replaces template variables
func (sl *SkillLoader) LoadWithVars(skillName string, vars map[string]string) (string, error) {
	prompt, err := sl.Load(skillName)
	if err != nil {
		return "", err
	}

	// Replace template variables {{key}}
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		prompt = strings.ReplaceAll(prompt, placeholder, value)
	}

	return prompt, nil
}

// ClearCache clears the skill cache
func (sl *SkillLoader) ClearCache() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.cache = make(map[string]string)
}
