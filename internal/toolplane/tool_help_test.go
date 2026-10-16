package toolplane

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestToolQueryHelpListsEverySectionAndType pins the self-documenting contract of
// `tool query`: an agent that can only read --help must be able to discover the
// sections and, for each one, the --type values it accepts.
func TestToolQueryHelpListsEverySectionAndType(t *testing.T) {
	help := toolQueryLongHelp()
	for _, section := range toolQuerySections {
		if !strings.Contains(help, section) {
			t.Errorf("query help does not mention section %q", section)
		}
		for _, queryType := range toolQuerySectionTypes[section] {
			if !strings.Contains(help, queryType) {
				t.Errorf("query help does not list --type %q for section %q", queryType, section)
			}
		}
	}
	if !strings.Contains(ToolQueryCmd.Use, "context") {
		t.Errorf("query Use line = %q, want it to list the sections", ToolQueryCmd.Use)
	}
}

// TestToolQueryTypeTableMatchesNormalizers keeps the documented per-section
// --type values and the alias normalizers from drifting apart: every value in the
// table must be accepted as-is, and every alias must resolve to a documented
// value.
func TestToolQueryTypeTableMatchesNormalizers(t *testing.T) {
	aliases := map[string][]string{
		"story-setup": {"storylines:storyline", "arc:storyline", "premises:premise", "cast:core-cast", "resources:resource", "world-timeline:timeline", "rule:rules", "longform:long-form-plan"},
		"outline":     {"parts:part", "volumes:volume", "chapters:chapter", "references:refs"},
		"craft":       {"characters:character", "chars:character", "items:item", "locations:location", "orgs:organization"},
		"chapter":     {"event:events", "references:refs"},
		"context":     {"chapter-draft:chapter-write", "repair-outline:outline-repair", "global-repair:outline-global-repair", "recap:recap-repair", "char:craft-character", "org:craft-organization", "volume:outline-volume"},
		"logs":        {"prompt:prompts", "response:responses", "live:agent-live"},
	}
	for section, types := range toolQuerySectionTypes {
		for _, queryType := range types {
			if got := canonicalToolQueryType(section, queryType); got != queryType {
				t.Errorf("canonicalToolQueryType(%q, %q) = %q, want it unchanged", section, queryType, got)
			}
			if !toolQueryTypeKnown(section, queryType) {
				t.Errorf("toolQueryTypeKnown(%q, %q) = false", section, queryType)
			}
		}
	}
	for section, pairs := range aliases {
		for _, pair := range pairs {
			alias, want, _ := strings.Cut(pair, ":")
			got := canonicalToolQueryType(section, alias)
			if got != want {
				t.Errorf("canonicalToolQueryType(%q, %q) = %q, want %q", section, alias, got, want)
			}
			if !toolQueryTypeKnown(section, got) {
				t.Errorf("alias %q resolved to %q, which the %q table does not list", alias, got, section)
			}
		}
	}
}

// TestToolQueryRejectsUnknownTypeWithValidValues checks the failure path an agent
// actually hits: a mistyped --type must say what the section accepts.
func TestToolQueryRejectsUnknownTypeWithValidValues(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})
	ToolQueryFlags.Type = "nonsense"
	defer func() { ToolQueryFlags.Type = "" }()

	err := runToolQuery(cmd, []string{"context"})
	if err == nil {
		t.Fatal("expected an error for an unknown --type")
	}
	for _, want := range []string{"nonsense", "chapter-repair", "outline-repair", "recap-repair"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
}

// TestToolQueryRequiresASectionIncludingLogs pins the section-required message:
// it used to omit logs even though the command accepts it.
func TestToolQueryRequiresASectionIncludingLogs(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})

	err := runToolQuery(cmd, nil)
	if err == nil {
		t.Fatal("expected an error when no section is given")
	}
	for _, section := range toolQuerySections {
		if !strings.Contains(err.Error(), section) {
			t.Errorf("error %q does not mention section %q", err.Error(), section)
		}
	}
}

// TestToolPatchHelpExplainsSectionAndTarget pins the correction of the --target
// trap: the section is the positional argument, --target selects within it, and
// the help must say which sections ignore --target.
func TestToolPatchHelpExplainsSectionAndTarget(t *testing.T) {
	help := ToolPatchCmd.Long
	for _, want := range []string{"--target", "--id", "--patch-buffer", "--task", "--refresh-derived", "setup", "recap", "chapter"} {
		if !strings.Contains(help, want) {
			t.Errorf("patch help does not mention %q", want)
		}
	}
	if target := ToolPatchCmd.Flags().Lookup("target"); target == nil || target.DefValue != "" {
		t.Errorf("--target default = %v, want empty so the help cannot imply it applies to every section", target)
	}
	if task := ToolPatchCmd.Flags().Lookup("task"); task == nil || !strings.Contains(task.Usage, "outline-global-repair") {
		t.Errorf("--task help must name the supported task form, got %v", task)
	}
}
