package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A released binary must be able to load skills without the source tree, so the
// loader has to fall back to the embedded copy.
func TestSkillLoaderFallsBackToEmbeddedSkills(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "does-not-exist")
	loader := NewSkillLoader(missingDir)

	prompt, err := loader.Load("write-generate")
	if err != nil {
		t.Fatalf("load embedded skill: %v", err)
	}
	if strings.TrimSpace(prompt) == "" {
		t.Fatalf("embedded skill is empty")
	}

	onDisk, err := os.ReadFile(filepath.Join("skills", "write-generate", "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill from disk: %v", err)
	}
	if prompt != string(onDisk) {
		t.Fatalf("embedded skill differs from the on-disk skill")
	}
}

func TestSkillLoaderPrefersDiskCopy(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "custom-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("disk override"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewSkillLoader(dir)
	prompt, err := loader.Load("custom-skill")
	if err != nil {
		t.Fatalf("load disk skill: %v", err)
	}
	if prompt != "disk override" {
		t.Fatalf("prompt = %q, want disk override", prompt)
	}
}

func TestSkillLoaderRejectsUnknownSkill(t *testing.T) {
	loader := NewSkillLoader(t.TempDir())
	if _, err := loader.Load("definitely-not-a-skill"); err == nil {
		t.Fatalf("expected an error for an unknown skill")
	}
	if _, err := loader.Load("../escape"); err == nil {
		t.Fatalf("expected an error for a traversal skill name")
	}
}
