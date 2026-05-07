package agents

import (
	"testing"

	"novelgen/internal/models"
)

func TestNormalizeGeneratedCharactersDropsUnrequestedNames(t *testing.T) {
	got := normalizeGeneratedCharacters([]string{"Requested"}, map[string]models.Character{
		"Hallucinated": {Name: "Hallucinated"},
	})

	if len(got) != 0 {
		t.Fatalf("expected unrequested character to be dropped, got %+v", got)
	}
}

func TestNormalizeGeneratedOrganizationsKeepsRequestedNames(t *testing.T) {
	got := normalizeGeneratedOrganizations([]string{"Iron Hive"}, map[string]models.Organization{
		"other-key": {Name: "Iron Hive"},
		"Extra":     {Name: "Extra"},
	})

	if len(got) != 1 || got["Iron Hive"].Name != "Iron Hive" {
		t.Fatalf("expected only requested organization, got %+v", got)
	}
}
