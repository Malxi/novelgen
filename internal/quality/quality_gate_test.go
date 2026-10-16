package quality_test

import (
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/quality"
	"novelgen/internal/testfixtures"
)

func TestValidateStorySetupDirectFindsMissingContractFields(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Test",
		CoreCast: []models.CoreCastSeed{{
			Name:          "Hero",
			Role:          "protagonist",
			Importance:    9,
			EntryPhase:    "opening",
			StoryFunction: "drives the main loop",
		}},
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

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "required setup field is empty")
	assertHasIssue(t, suggestions, "important storyline is under-specified")
	assertHasIssue(t, suggestions, "important storyline lacks an arc contract")
	assertHasIssue(t, suggestions, "important storyline lacks a complete appeal_engine")
	assertHasIssue(t, suggestions, "important core cast seed lacks payoff")
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

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "setup has too few progression systems")
}

func TestValidateStorySetupDirectFlagsThinLongFormPlan(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()
	setup.LongFormPlan = &models.LongFormPlan{
		TargetChapters: 1000,
		TargetVolumes:  10,
		MainLoop:       "pressure -> win",
	}

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "long-form plan has too few escalation ladder stages")
	assertHasIssue(t, suggestions, "long-form plan has too few reader promises")
	assertHasIssue(t, suggestions, "long-form plan lacks payoff cadence")
	assertHasIssue(t, suggestions, "long-form plan lacks a usable volume pattern")
	assertHasIssue(t, suggestions, "long-form plan lacks midpoint mutation")
}

func TestValidateStorySetupDirectFlagsThinLongFormStorylineEngine(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()
	setup.Storylines[0].Importance = 9
	setup.Storylines[0].Scope = "series"
	setup.Storylines[0].SetupRole = "main pressure engine"
	setup.Storylines[0].PayoffStyle = "staged_reveal"
	setup.Storylines[0].PressurePoints = []string{"public ranking", "faction challenge"}

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "important long-form storyline lacks repeatable pressure")
	assertHasIssue(t, suggestions, "important long-form storyline lacks payoff cadence")
	assertHasIssue(t, suggestions, "important long-form storyline lacks mutation")
	assertHasIssue(t, suggestions, "important long-form storyline lacks failure mode")
}

func TestValidateStorySetupDirectFlagsOversizedSetupForPromptStability(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()
	setup.Premise = strings.Repeat("A", 901)
	setup.Rules = append(setup.Rules, strings.Repeat("rule", 90))
	setup.Storylines[0].RepeatablePressure = strings.Repeat("pressure", 31)
	setup.CoreCast[0].StoryFunction = strings.Repeat("function", 35)

	for len(setup.CoreCast) < 13 {
		setup.CoreCast = append(setup.CoreCast, models.CoreCastSeed{
			Name:          "Extra Cast",
			Role:          "ally",
			Importance:    5,
			StoryFunction: "background support",
			EntryPhase:    "series",
		})
	}
	for len(setup.Storylines) < 13 {
		setup.Storylines = append(setup.Storylines, models.Storyline{Name: "Extra Arc", Importance: 5})
	}
	for len(setup.Premises) < 9 {
		setup.Premises = append(setup.Premises, models.Premise{
			Name: "Extra System",
			Progression: []models.ProgressionStage{
				{Level: 1, Name: "Tier", Description: "basic"},
			},
		})
	}

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "setup has too many core cast seeds")
	assertHasIssue(t, suggestions, "setup has too many storylines")
	assertHasIssue(t, suggestions, "setup has too many premise systems")
	assertHasIssue(t, suggestions, "setup field is too long for stable prompting")
	assertHasIssue(t, suggestions, "storyline serial engine hint is too long")
}

func TestValidateStorySetupDirectFlagsThinLongFormCoreCastCapacity(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()
	setup.CoreCast = []models.CoreCastSeed{
		{
			Name:          "Hero",
			Role:          "supporting",
			Importance:    9,
			StoryFunction: "survives the first arc",
			EntryPhase:    "opening",
			Payoff:        "earns a public win",
		},
		{
			Name:          "Rival",
			Role:          "rival",
			Importance:    8,
			StoryFunction: "tests the hero",
			EntryPhase:    "early",
			Payoff:        "forces the hero to reveal a trick",
		},
		{
			Name:          "Ally",
			Role:          "ally",
			Importance:    8,
			StoryFunction: "opens the faction route",
			EntryPhase:    "early",
			Payoff:        "brings the hero into a larger game",
		},
	}

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "core cast has no protagonist")
	assertHasIssue(t, suggestions, "long-form setup has too few important core cast seeds")
	assertHasIssue(t, suggestions, "core cast entry phases are front-loaded")
	assertHasIssue(t, suggestions, "important core cast seed lacks relationship arc")
	assertHasIssue(t, suggestions, "important core cast seed lacks storyline_refs")
}

func TestValidateStorySetupDirectFlagsUnknownCoreCastStorylineRef(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()
	setup.CoreCast[0].StorylineRefs = []string{"Missing Arc"}

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertHasIssue(t, suggestions, "core cast seed references unknown storyline")
}

func TestValidateStorySetupDirectAcceptsRichLongFormCoreCastCapacity(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()

	suggestions := quality.ValidateStorySetupDirect(setup)

	assertNoIssue(t, suggestions, "core cast has no protagonist")
	assertNoIssue(t, suggestions, "long-form setup has too few important core cast seeds")
	assertNoIssue(t, suggestions, "core cast entry phases are front-loaded")
	assertNoIssue(t, suggestions, "important core cast seed lacks relationship arc")
	assertNoIssue(t, suggestions, "important core cast seed lacks storyline_refs")
	assertNoIssue(t, suggestions, "core cast seed references unknown storyline")
	assertNoIssue(t, suggestions, "core cast role diversity is too low")
}

func TestValidateOutlineDirectFlagsLongFormPlanMismatch(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Summary: "Part summary",
		Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Summary: "Volume summary",
			Chapters: []models.Chapter{testfixtures.ValidChapterForQualityGate()},
		}},
	}}}

	suggestions := quality.ValidateOutlineDirect(setup, outline)

	assertHasIssue(t, suggestions, "outline chapter count is far below long_form_plan target")
	assertHasIssue(t, suggestions, "outline volume count is far below long_form_plan target")
	assertHasIssue(t, suggestions, "long-form outline has too many volumes without payoff_contract")
}

func TestRunScopedOutlineQualityGateSkipsLongFormScaleMismatch(t *testing.T) {
	setup := testfixtures.ValidLongFormSetup()
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Summary: "Part summary",
		Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Summary: "Volume summary",
			Chapters: []models.Chapter{testfixtures.ValidChapterForQualityGate()},
		}},
	}}}

	gate := quality.RunScopedOutlineQualityGate(setup, outline)

	assertNoIssue(t, gate.Suggestions, "outline chapter count is far below long_form_plan target")
	assertNoIssue(t, gate.Suggestions, "outline volume count is far below long_form_plan target")
	assertHasIssue(t, gate.Suggestions, "long-form outline has too many volumes without payoff_contract")
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

	suggestions := quality.ValidateOutlineDirect(setup, outline)

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
	gate := quality.QualityGateResult{}
	duplicate := quality.QualitySuggestion("plot", "P1-V1-C1", "Chapter", "same issue", "same fix", models.PriorityHigh)
	gate.Add(duplicate, duplicate)
	gate.Dedup()
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)

	if len(gate.Suggestions) != 1 {
		t.Fatalf("deduped suggestions = %d, want 1", len(gate.Suggestions))
	}
	if !gate.Blocking {
		t.Fatalf("expected high priority suggestion to be blocking")
	}
}

func TestQualityGateMediumReviewFiltersLowNoise(t *testing.T) {
	gate := quality.QualityGateResult{
		Suggestions: []models.ReviewSuggestion{
			quality.QualitySuggestion("logic", "P1-V1-C1", "C1", "low issue", "low fix", models.PriorityLow),
			quality.QualitySuggestion("logic", "P1-V1-C2", "C2", "medium issue", "medium fix", models.PriorityMedium),
			quality.QualitySuggestion("structure", "global", "global", "global medium issue", "global fix", models.PriorityMedium),
		},
	}
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	if gate.Blocking {
		t.Fatalf("medium-only gate should not use global blocking semantics")
	}
	if !quality.HasMediumOrHigherSuggestions(gate.Suggestions) {
		t.Fatalf("medium issue should be actionable for Agent SDK repair")
	}

	review := quality.QualityGateMediumReviewResult("medium repair", gate)
	if len(review.Suggestions) != 1 {
		t.Fatalf("review suggestions = %#v", review.Suggestions)
	}
	if review.Suggestions[0].Issue != "medium issue" {
		t.Fatalf("low/global issues should be filtered out: %#v", review.Suggestions)
	}
}

func TestLimitReviewSuggestionsForAgentSDKRepairBatchesHighestPriority(t *testing.T) {
	review := &models.ReviewResult{
		Summary: "repair",
		Suggestions: []models.ReviewSuggestion{
			quality.QualitySuggestion("logic", "P1-V1-C5", "C5", "medium late", "fix", models.PriorityMedium),
			quality.QualitySuggestion("logic", "P1-V1-C2", "C2", "high early", "fix", models.PriorityHigh),
			quality.QualitySuggestion("logic", "P1-V1-C4", "C4", "medium middle", "fix", models.PriorityMedium),
			quality.QualitySuggestion("logic", "P1-V1-C1", "C1", "critical first", "fix", models.PriorityCritical),
			quality.QualitySuggestion("logic", "P1-V1-C3", "C3", "medium third", "fix", models.PriorityMedium),
		},
	}

	got := quality.LimitReviewSuggestionsForAgentSDKRepair(review, 4)
	if got == review {
		t.Fatalf("expected a capped copy, got original pointer")
	}
	if len(got.Suggestions) != 4 {
		t.Fatalf("suggestions = %d, want 4", len(got.Suggestions))
	}
	wantIssues := []string{"critical first", "high early", "medium third", "medium middle"}
	for i, want := range wantIssues {
		if got.Suggestions[i].Issue != want {
			t.Fatalf("suggestion %d = %q, want %q; all=%#v", i, got.Suggestions[i].Issue, want, got.Suggestions)
		}
	}
	if !strings.Contains(got.Summary, "first 4 of 5") {
		t.Fatalf("summary should describe capped batch: %q", got.Summary)
	}
}

func TestRunOutlineSimulationGateUsesCraftProtagonistDetails(t *testing.T) {
	t.Parallel()

	characters := map[string]*models.Character{
		"Lin": {
			Name:        "Lin",
			RoleInStory: "protagonist",
			Background:  "old-world engineer",
			Motivation:  "restore humanity",
			Personality: []string{"calm"},
			CombatRole:  "mech pilot",
		},
	}

	gate := quality.RunOutlineSimulationGateWithElements(
		&models.StorySetup{ProjectName: "Fire"},
		&models.Outline{},
		characters,
		nil,
		nil,
		nil,
	)

	assertNoIssue(t, gate.Suggestions, "主角缺少背景故事")
	assertNoIssue(t, gate.Suggestions, "主角缺少性格或动机描述")
	assertNoIssue(t, gate.Suggestions, "主角缺少职业/修炼体系设定")
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

func assertNoIssue(t *testing.T, suggestions []models.ReviewSuggestion, needle string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion.Issue, needle) {
			t.Fatalf("unexpected issue containing %q: %#v", needle, suggestion)
		}
	}
}
