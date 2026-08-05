package cmd

import (
	"path/filepath"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

func TestCanResumeOutlineGenerationWithEmptyVolumes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outline.json")
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{
			{ID: "volume_1", Title: "Done", Chapters: []models.Chapter{{ID: "ch1", Title: "Chapter"}}},
			{ID: "volume_2", Title: "Pending", Chapters: []models.Chapter{}},
		},
	}}}
	if err := savePartialOutline(outline, path); err != nil {
		t.Fatalf("save outline: %v", err)
	}

	if !canResumeOutlineGeneration(path, models.StoryStructure{TargetParts: 1, TargetVolumes: 2, TargetChapters: 10}) {
		t.Fatalf("expected outline with empty volumes to be resumable")
	}
}

func TestOutlineWithGeneratedVolumesAndMergePreserveEmptyVolumes(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{
			{ID: "volume_1", Title: "Generated", Chapters: []models.Chapter{{ID: "ch1", Title: "Old"}}},
			{ID: "volume_2", Title: "Empty", Chapters: []models.Chapter{}},
		},
	}}}

	filtered := outlineWithGeneratedVolumes(outline)
	if got := len(filtered.Parts); got != 1 {
		t.Fatalf("filtered parts = %d, want 1", got)
	}
	if got := len(filtered.Parts[0].Volumes); got != 1 {
		t.Fatalf("filtered volumes = %d, want 1", got)
	}
	filtered.Parts[0].Volumes[0].Chapters[0].Title = "Improved"

	mergeGeneratedVolumes(outline, filtered)
	if got := outline.Parts[0].Volumes[0].Chapters[0].Title; got != "Improved" {
		t.Fatalf("merged generated volume title = %q, want Improved", got)
	}
	if got := len(outline.Parts[0].Volumes[1].Chapters); got != 0 {
		t.Fatalf("empty volume chapters = %d, want 0", got)
	}
}

func TestOutlineWithImproveVolumeSelection(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{
			{ID: "volume_1", Title: "Generated 1", Chapters: []models.Chapter{{ID: "ch1"}}},
			{ID: "volume_2", Title: "Generated 2", Chapters: []models.Chapter{{ID: "ch2"}}},
			{ID: "volume_3", Title: "Empty", Chapters: []models.Chapter{}},
		},
	}}}

	filtered, err := outlineWithImproveVolumeSelection(outline, 2, 0, 0)
	if err != nil {
		t.Fatalf("select volume: %v", err)
	}
	if got := len(filtered.Parts[0].Volumes); got != 1 {
		t.Fatalf("selected volumes = %d, want 1", got)
	}
	if got := filtered.Parts[0].Volumes[0].ID; got != "volume_2" {
		t.Fatalf("selected volume ID = %q, want volume_2", got)
	}

	if _, err := outlineWithImproveVolumeSelection(outline, 3, 0, 0); err == nil {
		t.Fatalf("expected empty selected volume to fail")
	}
}

func TestFilterAgentSDKReviewForPromptBoundaryKeepsOnlyMentionedChapter(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{
				{ID: "P1-V1-C1"},
				{ID: "P1-V1-C2"},
				{ID: "P1-V1-C3"},
			},
		}},
	}}}
	review := &models.ReviewResult{
		Summary: "post-check",
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V1-C1", Category: "logic", Priority: models.PriorityMedium},
			{TargetID: "P1-V1-C2", Category: "transition", Priority: models.PriorityMedium},
			{TargetID: "P1-V1-C3", Category: "logic", Priority: models.PriorityMedium},
		},
	}

	got := filterAgentSDKReviewForPromptBoundary(review, outline, "只修复 P1-V1-C2 的 transition/logic focused check", "test review")
	if len(got.Suggestions) != 1 || got.Suggestions[0].TargetID != "P1-V1-C2" {
		t.Fatalf("filtered suggestions = %#v", got.Suggestions)
	}
	if len(review.Suggestions) != 3 {
		t.Fatalf("filter mutated original review: %#v", review.Suggestions)
	}
}

func TestFilterAgentSDKReviewForPromptBoundaryLeavesUnboundedPromptAlone(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{
				{ID: "P1-V1-C1"},
				{ID: "P1-V1-C2"},
			},
		}},
	}}}
	review := &models.ReviewResult{Suggestions: []models.ReviewSuggestion{
		{TargetID: "P1-V1-C1", Category: "logic", Priority: models.PriorityMedium},
		{TargetID: "P1-V1-C2", Category: "logic", Priority: models.PriorityMedium},
	}}

	got := filterAgentSDKReviewForPromptBoundary(review, outline, "强化整卷节奏", "test review")
	if len(got.Suggestions) != 2 {
		t.Fatalf("unbounded prompt should keep suggestions: %#v", got.Suggestions)
	}
	if got != review {
		t.Fatalf("unbounded prompt should reuse original review")
	}
}

func TestOutlineGateHasPatchableGlobalIssuesDetectsSetupBackedFactionTier(t *testing.T) {
	gate := qualityGateResult{Suggestions: []models.ReviewSuggestion{{
		Category: "faction_tier",
		TargetID: "zerg",
		Issue:    "missing faction tier ladder",
		Priority: models.PriorityLow,
	}}}
	if !outlineGateHasPatchableGlobalIssues(gate, nil) {
		t.Fatalf("expected setup-backed faction_tier issue to be patchable")
	}
}

func TestOutlineGateHasPatchableGlobalIssuesIgnoresUnpatchableGlobalIssue(t *testing.T) {
	gate := qualityGateResult{Suggestions: []models.ReviewSuggestion{{
		Category: "mysteries",
		TargetID: "global",
		Issue:    "unresolved mystery needs story decision",
		Priority: models.PriorityLow,
	}}}
	if outlineGateHasPatchableGlobalIssues(gate, nil) {
		t.Fatalf("global mystery issue without outline evidence should not trigger apply-mode repair")
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:        "P1-V1-C1",
				Mysteries: models.ChapterMysteries{Planted: []models.MysteryPlanted{{ID: "myst_signal", Clue: "signal"}}},
			}, {
				ID: "P1-V1-C2",
			}},
		}},
	}}}
	if !outlineGateHasPatchableGlobalIssues(gate, outline) {
		t.Fatalf("global mystery issue with a later chapter should trigger patchable repair")
	}
}

func TestValidateSetupOutlineCrossInfersFactionFromPremiseName(t *testing.T) {
	setup := &models.StorySetup{Premises: []models.Premise{{
		Name:        "Zerg Faction Tier Ladder",
		Category:    "faction",
		Description: "zerg ranks: drone, soldier, captain",
	}}}
	outline := &rpg.StoryOutline{Parts: []rpg.StoryPart{{
		Volumes: []rpg.StoryVolume{{
			Chapters: []rpg.StoryChapter{{
				ID: "P1-V1-C1",
				Enemies: []rpg.StoryOutlineEnemy{{
					Name:    "zerg_drone",
					Faction: "zerg",
					Tier:    "drone",
					Count:   1,
				}},
			}},
		}},
	}}}

	issues, warnings := validateSetupOutlineCross(setup, outline)
	if len(issues) != 0 || len(warnings) != 0 {
		t.Fatalf("expected inferred zerg faction tiers to satisfy cross-check, issues=%v warnings=%v", issues, warnings)
	}
}

func TestRunOutlineValidatorOnModelSkipsSetupBackedFactionTierHint(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		Volumes: []models.Volume{{
			Chapters: []models.Chapter{{
				ID: "P1-V1-C1",
				Enemies: []models.OutlineEnemy{{
					Name:    "zerg_drone",
					Faction: "zerg",
					Tier:    "drone",
					Count:   1,
				}, {
					Name:    "zerg_soldier",
					Faction: "zerg",
					Tier:    "soldier",
					Count:   1,
				}},
			}},
		}},
	}}}

	for _, suggestion := range runOutlineValidatorOnModel(outline) {
		if suggestion.Category == "faction_tier" && suggestion.TargetID == "zerg" {
			t.Fatalf("outline-only validator should not emit setup-backed faction_tier hint: %#v", suggestion)
		}
	}
}

func TestOutlinesSemanticallyEqual(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{{
			ID:       "volume_1",
			Title:    "Generated",
			Chapters: []models.Chapter{{ID: "ch1", Title: "Old"}},
		}},
	}}}
	cloned := cloneOutline(outline)

	if !outlinesSemanticallyEqual(outline, cloned) {
		t.Fatalf("cloned outline should be semantically equal")
	}
	cloned.Parts[0].Volumes[0].Chapters[0].Title = "New"
	if outlinesSemanticallyEqual(outline, cloned) {
		t.Fatalf("changed outline should not be semantically equal")
	}
}

func TestOutlineVolumePositionUsesGlobalVolumeIndex(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{
		{ID: "part_1", Volumes: []models.Volume{{ID: "v1"}, {ID: "v2"}}},
		{ID: "part_2", Volumes: []models.Volume{{ID: "v3"}, {ID: "v4"}}},
	}}

	partIdx, volIdx, err := outlineVolumePosition(outline, 3)
	if err != nil {
		t.Fatalf("locate volume: %v", err)
	}
	if partIdx != 1 || volIdx != 0 {
		t.Fatalf("volume 3 position = (%d,%d), want (1,0)", partIdx, volIdx)
	}

	if _, _, err := outlineVolumePosition(outline, 5); err == nil {
		t.Fatalf("expected out-of-range volume to fail")
	}
}

func TestValidateComposeAgentSDKOptionRejectsOneShot(t *testing.T) {
	if err := validateComposeAgentSDKOption(true, true); err == nil {
		t.Fatalf("expected --agent-sdk with --one-shot to fail")
	}
	if err := validateComposeAgentSDKOption(true, false); err != nil {
		t.Fatalf("agent sdk without one-shot should pass: %v", err)
	}
}

func TestValidateComposeAgentApplyOptionRequiresAgentSDK(t *testing.T) {
	if err := validateComposeAgentApplyOption(false, true); err == nil {
		t.Fatalf("expected --agent-apply without --agent-sdk to fail")
	}
	if err := validateComposeAgentApplyOption(true, true); err != nil {
		t.Fatalf("agent apply with agent sdk should pass: %v", err)
	}
	if err := validateComposeAgentApplyOption(false, false); err != nil {
		t.Fatalf("disabled agent apply should pass: %v", err)
	}
}

func TestFilterQualityGateForAgentSDKPromptBoundaryKeepsOnlyPromptChapter(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V2",
			Chapters: []models.Chapter{
				{ID: "P1-V2-C4", Title: "第四章"},
				{ID: "P1-V2-C5", Title: "第五章"},
			},
		}},
	}}}
	gate := qualityGateResult{
		Blocking: true,
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V2-C4", Issue: "outside prompt", Priority: models.PriorityHigh},
			{TargetID: "global", Issue: "global issue", Priority: models.PriorityLow},
			{TargetID: "P1-V2-C5", Issue: "inside prompt", Priority: models.PriorityMedium},
		},
	}

	filtered := filterQualityGateForAgentSDKPromptBoundary(gate, outline, "只改 P1-V2-C5，不要修改其他章节", "test")

	if len(filtered.Suggestions) != 1 {
		t.Fatalf("filtered suggestions = %d, want 1: %#v", len(filtered.Suggestions), filtered.Suggestions)
	}
	if filtered.Suggestions[0].TargetID != "P1-V2-C5" {
		t.Fatalf("filtered target = %q, want P1-V2-C5", filtered.Suggestions[0].TargetID)
	}
	if filtered.Blocking {
		t.Fatalf("filtered gate should not be blocking after high-priority outside-prompt issue is removed")
	}
}

func TestFilterQualityGateForAgentSDKPromptBoundarySkipsGlobalWhenPromptNamesChapter(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID:       "P1-V2",
			Chapters: []models.Chapter{{ID: "P1-V2-C5", Title: "第五章"}},
		}},
	}}}
	gate := qualityGateResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "global", Issue: "global issue", Priority: models.PriorityLow},
		},
	}

	filtered := filterQualityGateForAgentSDKPromptBoundary(gate, outline, "只改 P1-V2-C5", "test")

	if len(filtered.Suggestions) != 0 {
		t.Fatalf("global suggestion should be outside chapter prompt boundary: %#v", filtered.Suggestions)
	}
}

func TestMissingSetupResources(t *testing.T) {
	setup := &models.StorySetup{
		WorldResources: []models.WorldResource{{Name: "ore"}},
	}
	outline := &rpg.StoryOutline{Parts: []rpg.StoryPart{{
		Volumes: []rpg.StoryVolume{{
			Chapters: []rpg.StoryChapter{{
				ResourceLedger: []rpg.StoryResourceLedgerEntry{
					{Item: "ore"},
					{Item: "crystal"},
					{Item: "battery"},
					{Item: "crystal"},
				},
			}},
		}},
	}}}

	missing := missingSetupResources(setup, outline)
	if len(missing) != 2 || missing[0] != "battery" || missing[1] != "crystal" {
		t.Fatalf("missing resources = %#v, want [battery crystal]", missing)
	}
}
