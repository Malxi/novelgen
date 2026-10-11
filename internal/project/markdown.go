package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/models"
)

// CreateStorySetupMarkdown renders story_setup.md from a typed StorySetup.
func CreateStorySetupMarkdown(setup *models.StorySetup, path string) error {
	content := fmt.Sprintf(`# %s

## Story Setup

### Genre(s)
%s

### Core Premise
%s

### Theme
%s

### Story Rules/Constraints
%s

### Target Audience
%s

### Tone/Style
%s

### Narrative Tense
%s

### POV Style
%s

### Writing Style
%s

## Long Form Plan
%s

## Core Cast
%s

## Storylines
%s

## Premises
%s
`,
		setup.ProjectName,
		FormatList(setup.Genres),
		setup.Premise,
		setup.Theme,
		FormatList(setup.Rules),
		setup.TargetAudience,
		setup.Tone,
		setup.Tense,
		setup.POVStyle,
		FormatWritingStyle(setup.WritingStyle),
		FormatLongFormPlan(setup.LongFormPlan),
		FormatCoreCast(setup.CoreCast),
		FormatStorylines(setup.Storylines),
		FormatPremises(setup.Premises),
	)
	return os.WriteFile(path, []byte(content), 0644)
}

// CreateOutlineMarkdown renders outline.md from a typed Outline.
func CreateOutlineMarkdown(outline *models.Outline, path string) error {
	return os.WriteFile(path, []byte(outline.ToMarkdown()), 0644)
}

// SavePartialOutline writes an outline as indented JSON, creating directories.
func SavePartialOutline(outline *models.Outline, path string) error {
	if outline == nil {
		return fmt.Errorf("cannot save nil outline")
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outline: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}
	return nil
}

// FormatLongFormPlan renders the long-form plan block.
func FormatLongFormPlan(plan *models.LongFormPlan) string {
	if plan == nil || plan.IsZero() {
		return "No long-form plan defined."
	}
	var result strings.Builder
	if plan.TargetChapters > 0 {
		result.WriteString(fmt.Sprintf("- **Target Chapters**: %d\n", plan.TargetChapters))
	}
	if plan.TargetVolumes > 0 {
		result.WriteString(fmt.Sprintf("- **Target Volumes**: %d\n", plan.TargetVolumes))
	}
	if plan.MainLoop != "" {
		result.WriteString(fmt.Sprintf("- **Main Loop**: %s\n", plan.MainLoop))
	}
	if len(plan.EscalationLadder) > 0 {
		result.WriteString(fmt.Sprintf("- **Escalation Ladder**: %s\n", strings.Join(plan.EscalationLadder, "; ")))
	}
	if len(plan.ReaderPromises) > 0 {
		result.WriteString(fmt.Sprintf("- **Reader Promises**: %s\n", strings.Join(plan.ReaderPromises, "; ")))
	}
	if plan.PayoffCadence != "" {
		result.WriteString(fmt.Sprintf("- **Payoff Cadence**: %s\n", plan.PayoffCadence))
	}
	if len(plan.VolumePattern) > 0 {
		result.WriteString(fmt.Sprintf("- **Volume Pattern**: %s\n", strings.Join(plan.VolumePattern, "; ")))
	}
	if plan.MidpointMutation != "" {
		result.WriteString(fmt.Sprintf("- **Midpoint Mutation**: %s\n", plan.MidpointMutation))
	}
	if plan.EndgamePromise != "" {
		result.WriteString(fmt.Sprintf("- **Endgame Promise**: %s\n", plan.EndgamePromise))
	}
	return result.String()
}

// FormatCoreCast renders the core-cast block.
func FormatCoreCast(seeds []models.CoreCastSeed) string {
	if len(seeds) == 0 {
		return "No core cast seeds defined."
	}
	var result strings.Builder
	for _, seed := range seeds {
		result.WriteString(fmt.Sprintf("\n### %s\n", seed.Name))
		if seed.ID != "" {
			result.WriteString(fmt.Sprintf("- **ID**: %s\n", seed.ID))
		}
		result.WriteString(fmt.Sprintf("- **Role**: %s\n", seed.Role))
		result.WriteString(fmt.Sprintf("- **Importance**: %d/10\n", seed.Importance))
		result.WriteString(fmt.Sprintf("- **Story Function**: %s\n", seed.StoryFunction))
		if seed.RelationshipToLead != "" {
			result.WriteString(fmt.Sprintf("- **Relationship To Lead**: %s\n", seed.RelationshipToLead))
		}
		if seed.RelationshipArc != "" {
			result.WriteString(fmt.Sprintf("- **Relationship Arc**: %s\n", seed.RelationshipArc))
		}
		if seed.EntryPhase != "" {
			result.WriteString(fmt.Sprintf("- **Entry Phase**: %s\n", seed.EntryPhase))
		}
		if seed.Payoff != "" {
			result.WriteString(fmt.Sprintf("- **Payoff**: %s\n", seed.Payoff))
		}
		if len(seed.StorylineRefs) > 0 {
			result.WriteString(fmt.Sprintf("- **Storyline Refs**: %s\n", strings.Join(seed.StorylineRefs, "; ")))
		}
	}
	return result.String()
}

// FormatStorylines renders the storylines block.
func FormatStorylines(storylines []models.Storyline) string {
	if len(storylines) == 0 {
		return "No storylines defined."
	}
	var result strings.Builder
	for _, s := range storylines {
		result.WriteString(fmt.Sprintf("\n### %s\n", s.Name))
		result.WriteString(fmt.Sprintf("- **Type**: %s\n", s.Type))
		result.WriteString(fmt.Sprintf("- **Importance**: %d/10\n", s.Importance))
		if s.Scope != "" {
			result.WriteString(fmt.Sprintf("- **Scope**: %s\n", s.Scope))
		}
		if s.PayoffStyle != "" {
			result.WriteString(fmt.Sprintf("- **Payoff Style**: %s\n", s.PayoffStyle))
		}
		if s.SetupRole != "" {
			result.WriteString(fmt.Sprintf("- **Setup Role**: %s\n", s.SetupRole))
		}
		if s.RepeatablePressure != "" {
			result.WriteString(fmt.Sprintf("- **Repeatable Pressure**: %s\n", s.RepeatablePressure))
		}
		if s.PayoffCadence != "" {
			result.WriteString(fmt.Sprintf("- **Payoff Cadence**: %s\n", s.PayoffCadence))
		}
		if s.Mutation != "" {
			result.WriteString(fmt.Sprintf("- **Mutation**: %s\n", s.Mutation))
		}
		if s.FailureMode != "" {
			result.WriteString(fmt.Sprintf("- **Failure Mode**: %s\n", s.FailureMode))
		}
		result.WriteString(fmt.Sprintf("- **Description**: %s\n", s.Description))
		if s.Desire != "" {
			result.WriteString(fmt.Sprintf("- **Desire**: %s\n", s.Desire))
		}
		if s.Opposition != "" {
			result.WriteString(fmt.Sprintf("- **Opposition**: %s\n", s.Opposition))
		}
		if s.Stakes != "" {
			result.WriteString(fmt.Sprintf("- **Stakes**: %s\n", s.Stakes))
		}
		if s.Turn != "" {
			result.WriteString(fmt.Sprintf("- **Turn**: %s\n", s.Turn))
		}
		if s.Payoff != "" {
			result.WriteString(fmt.Sprintf("- **Payoff**: %s\n", s.Payoff))
		}
		if s.OpenQuestion != "" {
			result.WriteString(fmt.Sprintf("- **Open Question**: %s\n", s.OpenQuestion))
		}
		if len(s.PressurePoints) > 0 {
			result.WriteString(fmt.Sprintf("- **Pressure Points**: %s\n", strings.Join(s.PressurePoints, "; ")))
		}
	}
	return result.String()
}

// FormatWritingStyle renders the writing-style block.
func FormatWritingStyle(style models.WritingStyle) string {
	if style.IsZero() {
		return "None"
	}
	var result strings.Builder
	if style.Name != "" {
		result.WriteString(fmt.Sprintf("- **Name**: %s\n", style.Name))
	}
	if style.Description != "" {
		result.WriteString(fmt.Sprintf("- **Description**: %s\n", style.Description))
	}
	if len(style.Principles) > 0 {
		result.WriteString("- **Principles**:\n")
		for _, item := range style.Principles {
			result.WriteString(fmt.Sprintf("  - %s\n", item))
		}
	}
	if len(style.Avoid) > 0 {
		result.WriteString("- **Avoid**:\n")
		for _, item := range style.Avoid {
			result.WriteString(fmt.Sprintf("  - %s\n", item))
		}
	}
	if style.ReferenceExcerpt != "" {
		result.WriteString("- **Reference Excerpt**:\n")
		result.WriteString(style.ReferenceExcerpt)
		if !strings.HasSuffix(style.ReferenceExcerpt, "\n") {
			result.WriteString("\n")
		}
	}
	return strings.TrimRight(result.String(), "\n")
}

// FormatPremises renders the premises block.
func FormatPremises(premises []models.Premise) string {
	if len(premises) == 0 {
		return "No premises defined."
	}
	var result strings.Builder
	for _, p := range premises {
		result.WriteString(fmt.Sprintf("\n### %s (%s)\n", p.Name, p.Category))
		result.WriteString(fmt.Sprintf("%s\n\n", p.Description))
		if len(p.Progression) > 0 {
			result.WriteString("**Progression System:**\n\n")
			for _, stage := range p.Progression {
				result.WriteString(fmt.Sprintf("**Level %d: %s**\n", stage.Level, stage.Name))
				result.WriteString(fmt.Sprintf("- Description: %s\n", stage.Description))
				if stage.Requirements != "" {
					result.WriteString(fmt.Sprintf("- Requirements: %s\n", stage.Requirements))
				}
				result.WriteString("\n")
			}
		}
	}
	return result.String()
}

// FormatList renders a bullet list or "None".
func FormatList(items []string) string {
	if len(items) == 0 {
		return "None"
	}
	var result strings.Builder
	for _, item := range items {
		result.WriteString(fmt.Sprintf("- %s\n", item))
	}
	return result.String()
}
