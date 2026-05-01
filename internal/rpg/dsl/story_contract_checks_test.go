package dsl

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestParserPreservesMetadataPhase(t *testing.T) {
	parsed, err := NewParser(`metadata { title = "T" phase = "setup" dsl_version = "0.2.0" }`).Parse()
	if err != nil {
		t.Fatalf("parse DSL: %v", err)
	}
	if parsed.Metadata == nil || parsed.Metadata.Phase != "setup" {
		t.Fatalf("expected metadata phase to survive parse, got %#v", parsed.Metadata)
	}
}

func TestOutlinePhaseDowngradesBalanceCriticalToWarning(t *testing.T) {
	sim := &Simulator{DSL: &DSL{Metadata: &Metadata{Phase: string(PhaseOutline)}}}
	sim.addIssue(IssueBalance, SeverityCritical, "C1", 1, "combat balance is inferred", "repair if useful")
	if len(sim.Issues) != 1 || sim.Issues[0].Severity != SeverityWarning {
		t.Fatalf("expected outline balance issue to be warning, got %#v", sim.Issues)
	}
}

func TestSetupPhaseStoryContractFeedbackDoesNotRequireOutlineChapters(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Contract Test",
		Storylines: []models.Storyline{
			{Name: "Main Arc", Type: "main", Importance: 9, Desire: "win"},
		},
	}

	issues, err := NewModelAdapter(setup, nil, nil, nil, nil).Simulate(PhaseSetup)
	if err != nil {
		t.Fatalf("simulate setup: %v", err)
	}

	for _, issue := range issues {
		if issue.Severity == SeverityCritical {
			t.Fatalf("setup contract simulation should not emit critical issue: %+v", issue)
		}
	}
	if !hasIssueContaining(issues, "under-specified") {
		t.Fatalf("expected under-specified storyline contract feedback, got %#v", issues)
	}
}

func TestOutlinePhaseStorylineContractFindsMissingAdvancement(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Outline Contract Test",
		Storylines: []models.Storyline{
			{
				Name:         "Lost Signal",
				Type:         "main",
				Importance:   9,
				Desire:       "find the source",
				Opposition:   "the signal corrupts navigation",
				Stakes:       "the fleet loses its route home",
				Payoff:       "the source is decoded",
				OpenQuestion: "who sent the signal",
			},
		},
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Chapters: []models.Chapter{
				{ID: "P1-V1-C1", Title: "Launch", Events: []models.Event{{Action: models.ActionMove, Target: "hangar"}}},
				{ID: "P1-V1-C2", Title: "Fight", Events: []models.Event{{Action: models.ActionCombat, Target: "raiders"}}},
			},
		}},
	}}}

	issues, err := NewModelAdapter(setup, outline, nil, nil, nil).Simulate(PhaseOutline)
	if err != nil {
		t.Fatalf("simulate outline: %v", err)
	}

	if !hasIssueContaining(issues, "not advanced") {
		t.Fatalf("expected missing storyline advancement feedback, got %#v", issues)
	}
	if !hasIssueContaining(issues, "durable story state change") {
		t.Fatalf("expected durable state-change feedback, got %#v", issues)
	}
}

func TestOutlinePhaseStorylineContractMatchesSanitizedNames(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Sanitized Contract Test",
		Storylines: []models.Storyline{{
			Name:         "Lost Signal",
			Type:         "main",
			Importance:   9,
			Desire:       "find the source",
			Opposition:   "the signal corrupts navigation",
			Stakes:       "the fleet loses its route home",
			OpenQuestion: "who sent the signal",
		}},
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Chapters: []models.Chapter{{
				ID: "P1-V1-C1", Title: "Signal",
				StorylineAdvances: []models.StorylineAdvance{{
					StorylineName: "lost_signal",
					Stage:         "pressure",
					Change:        "the signal starts steering the fleet",
					Consequence:   "the route home becomes less reliable",
				}},
			}},
		}},
	}}}

	issues, err := NewModelAdapter(setup, outline, nil, nil, nil).Simulate(PhaseOutline)
	if err != nil {
		t.Fatalf("simulate outline: %v", err)
	}
	if hasIssueContaining(issues, "not advanced") {
		t.Fatalf("expected sanitized storyline name to match setup contract, got %#v", issues)
	}
}

func TestOutlinePhaseLongScopePayoffAcceptsStagedReveal(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Long Scope Contract Test",
		Storylines: []models.Storyline{{
			Name:         "Conspiracy Arc",
			Type:         "subplot",
			Importance:   8,
			Scope:        "series",
			PayoffStyle:  "staged_reveal",
			Desire:       "decode the conspiracy",
			Opposition:   "the archives are controlled",
			Payoff:       "the conspiracy is fully exposed",
			OpenQuestion: "who controls the archives",
		}},
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Chapters: []models.Chapter{{
				ID: "P1-V1-C1", Title: "Signal",
				StorylineAdvances: []models.StorylineAdvance{{
					StorylineName: "Conspiracy Arc",
					Stage:         "reveal",
					Change:        "the first archive is decoded",
					Consequence:   "the conspiracy looks larger",
				}},
			}},
		}},
	}}}

	issues, err := NewModelAdapter(setup, outline, nil, nil, nil).Simulate(PhaseOutline)
	if err != nil {
		t.Fatalf("simulate outline: %v", err)
	}
	if hasIssueContaining(issues, "promises a payoff but outline DSL has no payoff/resolved movement") {
		t.Fatalf("long-scope staged reveal should not require immediate payoff, got %#v", issues)
	}
}

func TestOutlinePhaseImmediatePayoffStillRequiresResolution(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Immediate Contract Test",
		Storylines: []models.Storyline{{
			Name:         "Rescue Episode",
			Type:         "episode",
			Importance:   7,
			Scope:        "volume",
			PayoffStyle:  "immediate",
			Desire:       "rescue the prisoner",
			Opposition:   "the guards lock down the prison",
			Payoff:       "the prisoner is rescued",
			OpenQuestion: "can the team get out alive",
		}},
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Chapters: []models.Chapter{{
				ID: "P1-V1-C1", Title: "Prison",
				StorylineAdvances: []models.StorylineAdvance{{
					StorylineName: "Rescue Episode",
					Stage:         "pressure",
					Change:        "the team reaches the prison",
					Consequence:   "the alarm is raised",
				}},
			}},
		}},
	}}}

	issues, err := NewModelAdapter(setup, outline, nil, nil, nil).Simulate(PhaseOutline)
	if err != nil {
		t.Fatalf("simulate outline: %v", err)
	}
	if !hasIssueContaining(issues, "promises a payoff but outline DSL has no payoff/resolved movement") {
		t.Fatalf("immediate payoff should still require resolution, got %#v", issues)
	}
}

func TestOutlinePhaseStorylineContractFlagsTwistWithoutSetup(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Twist Contract Test",
		Storylines: []models.Storyline{{
			Name:         "Lost Signal",
			Type:         "main",
			Importance:   9,
			Desire:       "find the source",
			Opposition:   "the signal corrupts navigation",
			Stakes:       "the fleet loses its route home",
			Turn:         "the signal is bait",
			OpenQuestion: "who sent the signal",
		}},
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Chapters: []models.Chapter{{
				ID: "P1-V1-C1", Title: "Signal",
				StorylineAdvances: []models.StorylineAdvance{{
					StorylineName: "Lost Signal",
					Stage:         "twist",
					Change:        "the signal is revealed as bait",
					Consequence:   "the fleet is trapped",
				}},
			}},
		}},
	}}}

	issues, err := NewModelAdapter(setup, outline, nil, nil, nil).Simulate(PhaseOutline)
	if err != nil {
		t.Fatalf("simulate outline: %v", err)
	}
	if !hasIssueContaining(issues, "reversal without earlier pressure/reveal setup") {
		t.Fatalf("expected twist without setup feedback, got %#v", issues)
	}
}

func hasIssueContaining(issues []SimulationIssue, needle string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Description, needle) {
			return true
		}
	}
	return false
}
