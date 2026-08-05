package agents

import (
	"strings"
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

func TestCraftAgentSDKParamsAllowCharacterPatchOnly(t *testing.T) {
	required := []string{`novelgen tool query context --type craft-character --name "林野" --view brief`}
	params := craftAgentSDKParams("generate", "craft-character-workflow", "character", 16, false, []string{"林野"}, required)
	if !params.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if len(params.SDKSkills) != 2 || params.SDKSkills[0] != "novel-tools-core" || params.SDKSkills[1] != "craft-character-workflow" {
		t.Fatalf("SDKSkills = %#v", params.SDKSkills)
	}
	if len(params.ToolAllowlist) != 3 ||
		params.ToolAllowlist[0] != `novelgen tool query context --type craft-character --name "林野" --view brief` ||
		params.ToolAllowlist[1] != `novelgen tool check schema --target craft --scope character --id "林野"` ||
		!strings.HasPrefix(params.ToolAllowlist[2], "novelgen tool patch craft --target character --id ") {
		t.Fatalf("ToolAllowlist = %#v", params.ToolAllowlist)
	}
	for _, item := range params.ToolAllowlist {
		if item == "novelgen tool query" || item == "novelgen tool check" {
			t.Fatalf("ToolAllowlist should use exact craft context/check commands, got %#v", params.ToolAllowlist)
		}
	}
	if params.MaxTurns != 16 {
		t.Fatalf("MaxTurns = %d, want 16", params.MaxTurns)
	}
	if params.ToolEvidence.MinContextQueryCalls != 1 || params.ToolEvidence.MinCheckCalls != 1 || params.ToolEvidence.RequirePatchApplyFollowupCheck {
		t.Fatalf("ToolEvidence = %#v", params.ToolEvidence)
	}
}

func TestCraftAgentSDKParamsCanAllowCharacterPatchApplyExplicitly(t *testing.T) {
	required := []string{`novelgen tool query context --type craft-character --name "林野" --view brief`}
	params := craftAgentSDKParams("generate", "craft-character-workflow", "character", 16, true, []string{"林野"}, required)
	if len(params.ToolAllowlist) != 3 ||
		params.ToolAllowlist[0] != `novelgen tool query context --type craft-character --name "林野" --view brief` ||
		params.ToolAllowlist[1] != `novelgen tool check schema --target craft --scope character --id "林野"` ||
		!strings.HasPrefix(params.ToolAllowlist[2], "novelgen tool patch craft --target character --id ") ||
		!strings.Contains(params.ToolAllowlist[2], " --apply") {
		t.Fatalf("ToolAllowlist = %#v", params.ToolAllowlist)
	}
	if params.ToolEvidence.MinContextQueryCalls != 1 || params.ToolEvidence.MinCheckCalls != 1 ||
		params.ToolEvidence.MinPatchApplyCalls != 1 || !params.ToolEvidence.RequirePatchApplyFollowupCheck {
		t.Fatalf("ToolEvidence = %#v", params.ToolEvidence)
	}
}

func TestCraftAgentSDKParamsCanAllowElementPatchTargets(t *testing.T) {
	itemParams := craftAgentSDKParams("generate", "craft-element-workflow", "item", 16, false, []string{"Star Core"}, []string{`novelgen tool query context --type craft-item --name "Star Core" --view brief`})
	if len(itemParams.ToolAllowlist) != 3 ||
		itemParams.ToolAllowlist[2] != `novelgen tool patch craft --target item --id "Star Core"` {
		t.Fatalf("item ToolAllowlist = %#v", itemParams.ToolAllowlist)
	}

	locationParams := craftAgentSDKParams("generate", "craft-element-workflow", "location", 16, true, []string{"Mine"}, []string{`novelgen tool query context --type craft-location --name "Mine" --view brief`})
	if len(locationParams.ToolAllowlist) != 3 ||
		locationParams.ToolAllowlist[1] != `novelgen tool check schema --target craft --scope location --id "Mine"` ||
		locationParams.ToolAllowlist[2] != `novelgen tool patch craft --target location --id "Mine" --apply` {
		t.Fatalf("location ToolAllowlist = %#v", locationParams.ToolAllowlist)
	}
	if locationParams.ToolEvidence.MinPatchApplyCalls != 1 || !locationParams.ToolEvidence.RequirePatchApplyFollowupCheck {
		t.Fatalf("location ToolEvidence = %#v", locationParams.ToolEvidence)
	}

	orgParams := craftAgentSDKParams("generate", "craft-element-workflow", "organization", 16, false, []string{"Iron Hive"}, []string{`novelgen tool query context --type craft-organization --name "Iron Hive" --view brief`})
	if len(orgParams.SDKSkills) != 2 || orgParams.SDKSkills[1] != "craft-element-workflow" ||
		orgParams.ToolAllowlist[2] != `novelgen tool patch craft --target organization --id "Iron Hive"` {
		t.Fatalf("organization params = %+v", orgParams)
	}
}

func TestCraftAgentSDKToolAllowlistDeduplicatesQueries(t *testing.T) {
	got := craftAgentSDKToolAllowlist("character", []string{"林野", "林野", ""}, []string{
		`novelgen tool query context --type craft-character --name "林野" --view brief`,
		`novelgen tool query context --type craft-character --name "林野" --view brief`,
	}, false)
	if len(got) != 3 {
		t.Fatalf("allowlist = %#v, want 3 entries", got)
	}
	if got[0] != `novelgen tool query context --type craft-character --name "林野" --view brief` ||
		got[1] != `novelgen tool check schema --target craft --scope character --id "林野"` ||
		!strings.HasPrefix(got[2], "novelgen tool patch craft --target character --id ") {
		t.Fatalf("allowlist = %#v", got)
	}
}

func TestBuildCraftAgentSDKCharactersPromptInputUsesRequiredQueries(t *testing.T) {
	got := buildCraftAgentSDKCharactersPromptInput([]string{"林野"}, "focus on voice", false)
	if got.CustomPrompt != "focus on voice" {
		t.Fatalf("CustomPrompt = %q", got.CustomPrompt)
	}
	joined := strings.Join(got.RequiredQueries, "\n")
	for _, want := range []string{
		`novelgen tool query context --type craft-character --name "林野" --view brief`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("required query %q missing from %#v", want, got.RequiredQueries)
		}
	}
	if got.ApplyPatches {
		t.Fatalf("ApplyPatches = true")
	}
	if len(got.Instructions) == 0 || !strings.Contains(strings.Join(got.Instructions, "\n"), "tool patch craft") {
		t.Fatalf("instructions should mention craft patch dry-run: %#v", got.Instructions)
	}
	for _, want := range []string{"personality has 3-6", "skills cover mundane/tactical", "abilities cover special powers", "notes stays concise"} {
		if !strings.Contains(strings.Join(got.Instructions, "\n"), want) {
			t.Fatalf("character field instruction missing %q: %#v", want, got.Instructions)
		}
	}
	if !strings.Contains(strings.Join(got.Instructions, "\n"), "do not use --apply") {
		t.Fatalf("dry-run instructions should explicitly forbid --apply: %#v", got.Instructions)
	}
}

func TestBuildCraftAgentSDKCharactersPromptInputAllowsApplyWhenRequested(t *testing.T) {
	got := buildCraftAgentSDKCharactersPromptInput([]string{"林野"}, "", true)
	if !got.ApplyPatches {
		t.Fatalf("ApplyPatches = false")
	}
	joined := strings.Join(got.Instructions, "\n")
	if !strings.Contains(joined, "--apply") || !strings.Contains(joined, "dry-run") {
		t.Fatalf("apply instructions should require dry-run then --apply: %#v", got.Instructions)
	}
	if !strings.Contains(joined, "tool check schema --target craft") {
		t.Fatalf("apply instructions should require schema check after apply: %#v", got.Instructions)
	}
	if !strings.Contains(joined, "printf '%s' '<compact-json>' | novelgen tool patch craft --target character --id <name>") ||
		!strings.Contains(joined, "do not run Python/Node/PowerShell/help commands") {
		t.Fatalf("apply instructions should prefer stdin-piped Chinese JSON patches: %#v", got.Instructions)
	}
}

func TestBuildCraftAgentSDKElementsPromptInputUsesItemQueries(t *testing.T) {
	got := buildCraftAgentSDKElementsPromptInput("item", "items", []string{"Star Core"}, "focus on usage", false)
	if got.Target != "item" || got.OutputKey != "items" || got.CustomPrompt != "focus on usage" {
		t.Fatalf("prompt input = %+v", got)
	}
	joined := strings.Join(got.RequiredQueries, "\n")
	for _, want := range []string{
		`novelgen tool query context --type craft-item --name "Star Core" --view brief`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("required query %q missing from %#v", want, got.RequiredQueries)
		}
	}
	instructions := strings.Join(got.Instructions, "\n")
	if !strings.Contains(instructions, "tool patch craft --target item") ||
		!strings.Contains(instructions, `"items"`) {
		t.Fatalf("instructions should mention item patch and output key: %#v", got.Instructions)
	}
	if !strings.Contains(instructions, "printf '%s' '<compact-json>' | novelgen tool patch craft --target item --id <name>") {
		t.Fatalf("instructions should prefer stdin-piped item patch JSON: %#v", got.Instructions)
	}
}

func TestBuildCraftAgentSDKElementsPromptInputUsesOrganizationQueries(t *testing.T) {
	got := buildCraftAgentSDKElementsPromptInput("organization", "organizations", []string{"Iron Hive"}, "", false)
	joined := strings.Join(got.RequiredQueries, "\n")
	for _, want := range []string{
		`novelgen tool query context --type craft-organization --name "Iron Hive" --view brief`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("required query %q missing from %#v", want, got.RequiredQueries)
		}
	}
}
