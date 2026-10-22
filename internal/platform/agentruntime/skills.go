package agentruntime

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// SkillInfo is the metadata an autonomous agent sees before it decides which
// skill to load. The full SKILL.md body is intentionally omitted so a goal
// session does not have to pay for every workflow prompt up front.
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListEmbeddedSkills returns the bundled skill catalog sorted by name.
func ListEmbeddedSkills() ([]SkillInfo, error) {
	matches, err := embeddedSkillFiles.ReadDir("skills")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded skills: %w", err)
	}
	skills := make([]SkillInfo, 0, len(matches))
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
			return nil, fmt.Errorf("failed to read embedded skill %s: %w", name, err)
		}
		skills = append(skills, parseSkillInfo(name, string(content)))
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

// ReadEmbeddedSkill returns the full SKILL.md body for one bundled skill.
func ReadEmbeddedSkill(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid skill name %q", name)
	}
	data, err := embeddedSkillFiles.ReadFile(filepath.ToSlash(filepath.Join("skills", name, "SKILL.md")))
	if err != nil {
		return "", fmt.Errorf("skill %q not found: %w", name, err)
	}
	return string(data), nil
}

// SelectSkills filters a catalog to the requested names, preserving catalog
// order. An empty request means "all skills".
func SelectSkills(catalog []SkillInfo, requested []string) ([]SkillInfo, error) {
	if len(requested) == 0 {
		return append([]SkillInfo(nil), catalog...), nil
	}
	want := map[string]bool{}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		want[name] = true
	}
	if len(want) == 0 {
		return append([]SkillInfo(nil), catalog...), nil
	}
	selected := make([]SkillInfo, 0, len(want))
	for _, skill := range catalog {
		if want[skill.Name] {
			selected = append(selected, skill)
			delete(want, skill.Name)
		}
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for name := range want {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("unknown skill(s): %s", strings.Join(missing, ", "))
	}
	return selected, nil
}

// FormatSkillCatalog renders the compact catalog that is injected into a goal
// prompt. Full instructions are loaded on demand through
// `novelgen tool skill read <name>`.
func FormatSkillCatalog(skills []SkillInfo) string {
	if len(skills) == 0 {
		return "(no skills available)"
	}
	var b strings.Builder
	for _, skill := range skills {
		b.WriteString("- ")
		b.WriteString(skill.Name)
		if description := strings.TrimSpace(skill.Description); description != "" {
			b.WriteString(": ")
			b.WriteString(description)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func parseSkillInfo(fallbackName, content string) SkillInfo {
	info := SkillInfo{Name: fallbackName}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return info
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			if value != "" {
				info.Name = value
			}
		case "description":
			info.Description = value
		}
	}
	return info
}
