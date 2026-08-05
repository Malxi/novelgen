package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestAgentToolsDocsDeclareAgentSDKCommandCoverage(t *testing.T) {
	data, err := os.ReadFile("../docs/AGENT_TOOLS.md")
	if err != nil {
		t.Fatalf("read AGENT_TOOLS.md: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"## Agent SDK Command Coverage",
		"`compose gen --agent-sdk`",
		"`compose improve/pipeline --agent-sdk`",
		"`setup improve/regen --agent-sdk`",
		"`setup regen --agent-sdk`",
		"`craft gen/improve --agent-sdk`",
		"`write gen/review/improve/pipeline --agent-sdk`",
		"`polish --agent-sdk`",
		"`recap gen --agent-sdk`",
		"`translate --agent-sdk`",
		"`novelgen project doctor --json` is read-only",
		"`draft` | Legacy optional workflow",
		"Go remain the writer",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("AGENT_TOOLS.md missing %q", want)
		}
	}
}

func TestStageContractsDeclareSetupRegenAgentSDKContract(t *testing.T) {
	data, err := os.ReadFile("../docs/STAGE_CONTRACTS.md")
	if err != nil {
		t.Fatalf("read STAGE_CONTRACTS.md: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"`setup improve --agent-sdk` and `setup regen --agent-sdk`",
		"`setup-improve-workflow`",
		"`novelgen tool patch setup`",
		"`setup regen --agent-sdk` is a focused regeneration/repair mode",
		"Go remains the writer",
		"`--agent-apply`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("STAGE_CONTRACTS.md missing %q", want)
		}
	}
}

func TestAgentSDKWriteDocsDeclareFocusedRepairAndInfoGrowth(t *testing.T) {
	files := []string{
		"../docs/AGENT_TOOLS.md",
		"../docs/STAGE_CONTRACTS.md",
		"../internal/agentruntime/skills/write-improve-workflow/SKILL.md",
		"../internal/agentruntime/skills/write-chapter-workflow/SKILL.md",
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, want := range []string{
			"信息差",
			"主角成长",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}

	data, err := os.ReadFile("../docs/STAGE_CONTRACTS.md")
	if err != nil {
		t.Fatalf("read STAGE_CONTRACTS.md: %v", err)
	}
	contract := string(data)
	for _, want := range []string{
		"chapter's narrative-unit count",
		"knowledge/insight/clue/information/strategy",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("STAGE_CONTRACTS.md missing %q", want)
		}
	}

	data, err = os.ReadFile("../internal/agentruntime/skills/write-improve-workflow/SKILL.md")
	if err != nil {
		t.Fatalf("read write-improve-workflow skill: %v", err)
	}
	skill := string(data)
	for _, want := range []string{
		"默认做最小修复",
		"不为凑目标字数扩写新场景",
		"获得日志线索",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("write-improve-workflow skill missing %q", want)
		}
	}
}

func TestAgentToolsDocsDeclareAgentLiveSummaryContract(t *testing.T) {
	files := []string{
		"../docs/AGENT_TOOLS.md",
		"../internal/agentruntime/skills/novel-tools-core/SKILL.md",
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, want := range []string{
			"agent-live",
			"summary",
			"model",
			"patch",
			"allowed",
			"denied",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}

	data, err := os.ReadFile("../docs/AGENT_TOOLS.md")
	if err != nil {
		t.Fatalf("read AGENT_TOOLS.md: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"`final_model`",
		"`sdk_skills`",
		"`query_calls`",
		"`check_calls`",
		"`patch_applies`",
		"`allowed_tool_commands`",
		"`denied_tool_commands`",
		"`--stdin <stdin>`",
		"`<claude-temp-tool-output>`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("AGENT_TOOLS.md missing %q", want)
		}
	}
}
