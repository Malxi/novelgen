package cmd

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func TestParseStorySetupMarkdownPreservesStorylineContractHints(t *testing.T) {
	setup, err := parseStorySetupFromMarkdown(`# Test

## Basic Information
- **Genres**: 科幻
- **Premise**: premise
- **Theme**: theme
- **Rules**: rule
- **Target Audience**: readers
- **Tone**: tone
- **Tense**: 过去时
- **POV Style**: 第三人称有限视角

## Storylines

### Signal Arc
- **Type**: main
- **Importance**: 9/10
- **Scope**: series
- **Payoff Style**: staged_reveal
- **Setup Role**: long mystery engine
- **Description**: A signal bends the war.
- **Desire**: decode it
- **Opposition**: corrupted archives
`)
	if err != nil {
		t.Fatalf("parse setup markdown: %v", err)
	}
	if len(setup.Storylines) != 1 {
		t.Fatalf("storylines = %d, want 1", len(setup.Storylines))
	}
	storyline := setup.Storylines[0]
	if storyline.Scope != "series" || storyline.PayoffStyle != "staged_reveal" || storyline.SetupRole != "long mystery engine" {
		t.Fatalf("storyline hints not preserved: %#v", storyline)
	}
}

func TestFormatStorylinesIncludesContractHints(t *testing.T) {
	text := formatStorylines([]models.Storyline{modelsStorylineForTest()})
	for _, want := range []string{
		"- **Scope**: series",
		"- **Payoff Style**: staged_reveal",
		"- **Setup Role**: long mystery engine",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted storyline missing %q:\n%s", want, text)
		}
	}
}

func modelsStorylineForTest() models.Storyline {
	return models.Storyline{
		Name:        "Signal Arc",
		Type:        "main",
		Importance:  9,
		Scope:       "series",
		PayoffStyle: "staged_reveal",
		SetupRole:   "long mystery engine",
		Description: "A signal bends the war.",
	}
}
