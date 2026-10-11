package cmd

import (
	"strings"
	"testing"

	"novelgen/cmd/writecmd"
	"novelgen/internal/models"
)

func TestOrganizationWriteContextOnlyIncludesRelevantOrganizations(t *testing.T) {
	chapter := &models.Chapter{
		ID:      "P1-V1-C1",
		Title:   "Mine Ambush",
		Summary: "Chen Dong faces an Iron Hive patrol.",
		Enemies: []models.OutlineEnemy{{
			Name:    "Drone Guard",
			Faction: "Iron Hive",
		}},
	}
	organizations := map[string]*models.Organization{
		"iron_hive": {
			Name:        "Iron Hive",
			Type:        "faction",
			Description: "A machine cult controlling the lower mine shafts.",
			Goals:       []string{"Hold the mine"},
		},
		"moon_court": {
			Name:        "Moon Court",
			Type:        "faction",
			Description: "A distant aristocratic house.",
			Goals:       []string{"Win court influence"},
		},
	}

	got := writecmd.BuildOrganizationWriteContext(chapter, organizations)
	if !strings.Contains(got, "Iron Hive") {
		t.Fatalf("expected relevant organization in context, got %q", got)
	}
	if strings.Contains(got, "Moon Court") {
		t.Fatalf("unmatched organization should not be injected into write context: %q", got)
	}
}

func TestOrganizationWriteContextReturnsEmptyWithoutMatches(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Quiet Camp"}
	organizations := map[string]*models.Organization{
		"moon_court": {Name: "Moon Court", Type: "faction"},
	}

	if got := writecmd.BuildOrganizationWriteContext(chapter, organizations); got != "" {
		t.Fatalf("expected no organization context for unrelated chapter, got %q", got)
	}
}
