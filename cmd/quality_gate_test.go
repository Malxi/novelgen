package cmd

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestValidateStorySetupDirectFindsMissingContractFields(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Test",
		Storylines: []models.Storyline{{
			Name:       "Main Arc",
			Importance: 9,
		}},
		Premises: []models.Premise{{
			Name: "Power",
			Progression: []models.ProgressionStage{
				{Level: 2, Name: "Step Two", Description: "later"},
				{Level: 1, Name: "Step One", Description: "earlier"},
			},
		}},
		WorldResources: []models.WorldResource{
			{Name: "crystal", Category: "energy", Scarcity: "rare"},
			{Name: "crystal", Category: "energy", Scarcity: "rare"},
		},
	}

	suggestions := validateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "required setup field is empty")
	assertHasIssue(t, suggestions, "important storyline is under-specified")
	assertHasIssue(t, suggestions, "important storyline lacks an arc contract")
	assertHasIssue(t, suggestions, "important storyline lacks a complete appeal_engine")
	assertHasIssue(t, suggestions, "progression levels are not increasing")
	assertHasIssue(t, suggestions, "duplicate world resource name")
}

func TestValidateStorySetupDirectFlagsThinLongFormSystems(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName:    "Test",
		Genres:         []string{"sci-fi", "mecha"},
		Premise:        "A pilot survives an apocalyptic war through a gene-linked mech.",
		Theme:          "Survival and responsibility",
		Rules:          []string{"Gene locks cost stability", "Enemy hives scale in tiers"},
		TargetAudience: "adult genre readers",
		Tone:           "tense",
		Tense:          "past",
		POVStyle:       "third person limited",
		Storylines: []models.Storyline{{
			Name:           "Main War",
			Type:           "main",
			Importance:     9,
			Scope:          "book",
			SetupRole:      "survival hook",
			PayoffStyle:    "staged_reveal",
			PressurePoints: []string{"hive expansion", "pilot instability"},
		}},
		Premises: []models.Premise{{
			Name: "Pilot Growth",
			Progression: []models.ProgressionStage{
				{Level: 1, Name: "Spark", Description: "Basic activation"},
				{Level: 2, Name: "Forge", Description: "Stable combat"},
			},
		}},
	}

	suggestions := validateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "setup has too few progression systems")
}

func TestValidateOutlineDirectFindsChapterContractIssues(t *testing.T) {
	setup := &models.StorySetup{
		Storylines:     []models.Storyline{{Name: "Main Arc", Importance: 8}},
		WorldResources: []models.WorldResource{{Name: "crystal", Category: "energy", Scarcity: "rare"}},
	}
	outline := &models.Outline{
		Parts: []models.Part{{
			ID:      "P1",
			Title:   "Part",
			Summary: "Part summary",
			Volumes: []models.Volume{{
				ID:      "P1-V1",
				Title:   "Volume",
				Summary: "Volume summary",
				Chapters: []models.Chapter{{
					ID:         "P1-V1-C1",
					Title:      "Chapter",
					Summary:    "Hero fights",
					Characters: []string{"Hero"},
					Location:   "Arena",
					Events: []models.Event{{
						Actor:      "Hero",
						Action:     models.ActionCombat,
						Target:     "Guard",
						TargetType: models.TargetTypeCharacter,
					}},
					Conflict: "Survive the ambush",
					Pacing:   "fast",
					Timeline: models.ChapterTimeline{TimeJump: true},
					ResourceLedger: []models.ResourceLedgerEntry{{
						Item:  "unknown",
						Start: 1,
						Delta: 2,
						End:   9,
					}},
				}},
			}},
		}},
	}

	suggestions := validateOutlineDirect(setup, outline)

	assertHasIssue(t, suggestions, "chapter event count should be 3-5")
	assertHasIssue(t, suggestions, "combat chapter has no enemies")
	assertHasIssue(t, suggestions, "chapter has no scenes")
	assertHasIssue(t, suggestions, "chapter lacks chapter_payoff")
	assertHasIssue(t, suggestions, "volume lacks payoff_contract")
	assertHasIssue(t, suggestions, "time jump lacks transition")
	assertHasIssue(t, suggestions, "resource ledger arithmetic is invalid")
	assertHasIssue(t, suggestions, "is not declared in setup.world_resources")
	assertHasIssue(t, suggestions, "outline never advances setup storylines")
}

func TestQualityGateDeduplicatesSuggestionsAndMarksBlocking(t *testing.T) {
	gate := qualityGateResult{}
	duplicate := qualitySuggestion("plot", "P1-V1-C1", "Chapter", "same issue", "same fix", models.PriorityHigh)
	gate.add(duplicate, duplicate)
	gate.dedup()
	gate.Blocking = hasBlockingSuggestions(gate.Suggestions)

	if len(gate.Suggestions) != 1 {
		t.Fatalf("deduped suggestions = %d, want 1", len(gate.Suggestions))
	}
	if !gate.Blocking {
		t.Fatalf("expected high priority suggestion to be blocking")
	}
}

func assertHasIssue(t *testing.T, suggestions []models.ReviewSuggestion, needle string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion.Issue, needle) {
			return
		}
	}
	t.Fatalf("expected issue containing %q, got %#v", needle, suggestions)
}
