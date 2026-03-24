package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillLoader loads skill definitions from SKILL.md files
type SkillLoader struct {
	skillsDir string
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
	if prompt, ok := sl.cache[skillName]; ok {
		return prompt, nil
	}

	// Load from file
	skillPath := filepath.Join(sl.skillsDir, skillName, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return "", fmt.Errorf("failed to load skill %s: %w", skillName, err)
	}

	// Extract system prompt from SKILL.md
	// The entire file is treated as the system prompt
	prompt := string(content)

	// Cache it
	sl.cache[skillName] = prompt

	return prompt, nil
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
	sl.cache = make(map[string]string)
}
