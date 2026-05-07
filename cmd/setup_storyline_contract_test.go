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
		Name:        "Signal Arc",
		Type:        "main",
		Importance:  9,
		Scope:       "series",
		PayoffStyle: "staged_reveal",
		SetupRole:   "long mystery engine",
		Description: "A signal bends the war.",
	}
}
