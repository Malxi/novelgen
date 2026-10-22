package agentruntime

import (
	"strings"
	"testing"
)

func TestListEmbeddedSkillsIncludesGoalAgentCore(t *testing.T) {
	skills, err := ListEmbeddedSkills()
	if err != nil {
		t.Fatalf("ListEmbeddedSkills() error = %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("ListEmbeddedSkills() returned no skills")
	}
	found := false
	for _, skill := range skills {
		if skill.Name == "goal-agent-core" {
			found = true
			if strings.TrimSpace(skill.Description) == "" {
				t.Error("goal-agent-core description is empty")
			}
		}
		if skill.Name == "" {
			t.Error("skill with empty name")
		}
	}
	if !found {
		t.Fatal("goal-agent-core is missing from the skill catalog")
	}
}

func TestReadEmbeddedSkillRejectsTraversal(t *testing.T) {
	if _, err := ReadEmbeddedSkill("../secret"); err == nil {
		t.Fatal("expected traversal skill name to be rejected")
	}
}

func TestSelectSkillsPreservesCatalogOrderAndRejectsUnknown(t *testing.T) {
	catalog := []SkillInfo{
		{Name: "a", Description: "first"},
		{Name: "b", Description: "second"},
		{Name: "c", Description: "third"},
	}
	selected, err := SelectSkills(catalog, []string{"c", "a"})
	if err != nil {
		t.Fatalf("SelectSkills() error = %v", err)
	}
	if len(selected) != 2 || selected[0].Name != "a" || selected[1].Name != "c" {
		t.Fatalf("SelectSkills() = %#v, want catalog order a, c", selected)
	}
	if _, err := SelectSkills(catalog, []string{"missing"}); err == nil {
		t.Fatal("expected unknown skill to be rejected")
	}
}

func TestFormatSkillCatalogIncludesDescriptions(t *testing.T) {
	got := FormatSkillCatalog([]SkillInfo{{Name: "chapter-repair", Description: "fix one chapter"}})
	if !strings.Contains(got, "chapter-repair") || !strings.Contains(got, "fix one chapter") {
		t.Fatalf("FormatSkillCatalog() = %q", got)
	}
}
