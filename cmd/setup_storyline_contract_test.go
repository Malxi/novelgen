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
- **Repeatable Pressure**: every faction tries to weaponize the signal
- **Payoff Cadence**: partial reveal every volume
- **Mutation**: the signal changes from mystery to public arms race
- **Failure Mode**: repeating clue hunts without consequences
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
	if storyline.RepeatablePressure == "" || storyline.PayoffCadence == "" || storyline.Mutation == "" || storyline.FailureMode == "" {
		t.Fatalf("storyline serial engine hints not preserved: %#v", storyline)
	}
}

func TestFormatStorylinesIncludesContractHints(t *testing.T) {
	text := formatStorylines([]models.Storyline{modelsStorylineForTest()})
	for _, want := range []string{
		"- **Scope**: series",
		"- **Payoff Style**: staged_reveal",
		"- **Setup Role**: long mystery engine",
		"- **Repeatable Pressure**: every faction tests the signal",
		"- **Payoff Cadence**: one partial reveal per volume",
		"- **Mutation**: mystery becomes arms race",
		"- **Failure Mode**: clue loop without consequences",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted storyline missing %q:\n%s", want, text)
		}
	}
}

func TestParseStorySetupMarkdownPreservesCoreCast(t *testing.T) {
	setup, err := parseStorySetupFromMarkdown(`# Test

## Core Cast

### Hero
- **Role**: protagonist
- **Importance**: 10/10
- **Story Function**: drives the main loop
- **Relationship To Lead**: self
- **Relationship Arc**: outsider -> leader
- **Entry Phase**: opening
- **Payoff**: public win
- **Storyline Refs**: Main Arc; Rivalry Arc
`)
	if err != nil {
		t.Fatalf("parse setup markdown: %v", err)
	}
	if len(setup.CoreCast) != 1 {
		t.Fatalf("core cast = %d, want 1", len(setup.CoreCast))
	}
	seed := setup.CoreCast[0]
	if seed.Name != "Hero" || seed.Role != "protagonist" || seed.Importance != 10 || len(seed.StorylineRefs) != 2 {
		t.Fatalf("core cast seed not preserved: %#v", seed)
	}
}

func TestFormatCoreCastIncludesRoleAndPayoff(t *testing.T) {
	text := formatCoreCast([]models.CoreCastSeed{{
		Name:            "Hero",
		Role:            "protagonist",
		Importance:      10,
		StoryFunction:   "drives the main loop",
		EntryPhase:      "opening",
		Payoff:          "public win",
		StorylineRefs:   []string{"Main Arc"},
		RelationshipArc: "outsider -> leader",
	}})
	for _, want := range []string{
		"### Hero",
		"- **Role**: protagonist",
		"- **Importance**: 10/10",
		"- **Story Function**: drives the main loop",
		"- **Relationship Arc**: outsider -> leader",
		"- **Payoff**: public win",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted core cast missing %q:\n%s", want, text)
		}
	}
}

func TestParseStorySetupMarkdownPreservesLongFormPlan(t *testing.T) {
	setup, err := parseStorySetupFromMarkdown(`# Test

## Long Form Plan
- **Target Chapters**: 1000
- **Target Volumes**: 10
- **Main Loop**: pressure -> exploit -> win -> reward -> bigger game
- **Escalation Ladder**: village; city; sect; empire
- **Reader Promises**: upgrades; public wins; faction rise
- **Payoff Cadence**: small wins every chapter, big wins every volume
- **Volume Pattern**: hook; pressure; misread; exploit; big win; reward; next gate
- **Midpoint Mutation**: local ranking game becomes faction war
- **Endgame Promise**: the hero overturns the system in public
`)
	if err != nil {
		t.Fatalf("parse setup markdown: %v", err)
	}
	if setup.LongFormPlan == nil {
		t.Fatalf("long form plan was not parsed")
	}
	plan := setup.LongFormPlan
	if plan.TargetChapters != 1000 || plan.TargetVolumes != 10 || len(plan.EscalationLadder) != 4 || len(plan.VolumePattern) != 7 {
		t.Fatalf("long form plan not preserved: %#v", plan)
	}
}

func TestFormatLongFormPlanIncludesCadenceAndVolumePattern(t *testing.T) {
	text := formatLongFormPlan(&models.LongFormPlan{
		TargetChapters:   1000,
		TargetVolumes:    10,
		MainLoop:         "pressure -> exploit -> win",
		EscalationLadder: []string{"village", "city", "empire"},
		ReaderPromises:   []string{"upgrades", "public wins"},
		PayoffCadence:    "small wins every chapter",
		VolumePattern:    []string{"hook", "pressure", "win"},
		MidpointMutation: "local to regional",
		EndgamePromise:   "final public reversal",
	})
	for _, want := range []string{
		"- **Target Chapters**: 1000",
		"- **Main Loop**: pressure -> exploit -> win",
		"- **Payoff Cadence**: small wins every chapter",
		"- **Volume Pattern**: hook; pressure; win",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted long form plan missing %q:\n%s", want, text)
		}
	}
}

func TestParseStorySetupMarkdownPreservesWritingStyle(t *testing.T) {
	setup, err := parseStorySetupFromMarkdown(`# Test

## Story Setup

### Tone/Style
冷峻、克制

### POV Style
第三人称有限视角

### Writing Style
- **Name**: 冷峻克制
- **Description**: 用短句推进危机，少解释，多动作细节。
- **Principles**:
  - 少用总结性形容词
  - 对话留白
- **Avoid**:
  - 不要复制参考文原句
- **Reference Excerpt**:
雨停了。巷口的灯还亮着，水迹贴在石阶上，像一层薄薄的旧伤。
他说：别回头。
`)
	if err != nil {
		t.Fatalf("parse setup markdown: %v", err)
	}
	if setup.Tone != "冷峻、克制" {
		t.Fatalf("tone = %q", setup.Tone)
	}
	if setup.POVStyle != "第三人称有限视角" {
		t.Fatalf("pov_style = %q", setup.POVStyle)
	}
	if setup.WritingStyle.Name != "冷峻克制" {
		t.Fatalf("writing style name = %q", setup.WritingStyle.Name)
	}
	if len(setup.WritingStyle.Principles) != 2 || setup.WritingStyle.Principles[0] != "少用总结性形容词" {
		t.Fatalf("principles not parsed: %#v", setup.WritingStyle.Principles)
	}
	if len(setup.WritingStyle.Avoid) != 1 || !strings.Contains(setup.WritingStyle.ReferenceExcerpt, "巷口的灯") || !strings.Contains(setup.WritingStyle.ReferenceExcerpt, "别回头") {
		t.Fatalf("writing style details not parsed: %#v", setup.WritingStyle)
	}
}

func TestFormatWritingStyleIncludesReferenceExcerpt(t *testing.T) {
	text := formatWritingStyle(models.WritingStyle{
		Name:             "轻喜剧",
		Principles:       []string{"短句快节奏"},
		Avoid:            []string{"不要堆热梗"},
		ReferenceExcerpt: "她把门推开，先听见锅盖在唱歌。",
	})
	for _, want := range []string{
		"- **Name**: 轻喜剧",
		"- **Principles**:",
		"- **Avoid**:",
		"- **Reference Excerpt**:",
		"她把门推开",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted writing style missing %q:\n%s", want, text)
		}
	}
}

func modelsStorylineForTest() models.Storyline {
	return models.Storyline{
		Name:               "Signal Arc",
		Type:               "main",
		Importance:         9,
		Scope:              "series",
		PayoffStyle:        "staged_reveal",
		SetupRole:          "long mystery engine",
		RepeatablePressure: "every faction tests the signal",
		PayoffCadence:      "one partial reveal per volume",
		Mutation:           "mystery becomes arms race",
		FailureMode:        "clue loop without consequences",
		Description:        "A signal bends the war.",
	}
}
