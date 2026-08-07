package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/models"
	"novelgen/internal/utils"
)

type fakeComposeImproveRuntime struct {
	result     *agentruntime.Result
	err        error
	invocation agentruntime.Invocation
}

func (r *fakeComposeImproveRuntime) Invoke(ctx context.Context, invocation agentruntime.Invocation) (*agentruntime.Result, error) {
	r.invocation = invocation
	return r.result, r.err
}

func TestIdentifyVolumesToImproveResolvesActualVolumeIDs(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{
				{ID: "P1-V1"},
				{ID: "P1-V2"},
			},
		}},
	}
	review := &models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V2-C3"},
			{TargetID: "P1-V2"},
			{TargetID: "global"},
		},
	}

	got := (&ComposeAgent{}).identifyVolumesToImprove(outline, review)
	if len(got) != 1 {
		t.Fatalf("expected one volume target, got %v", got)
	}
	if got[0] != [2]int{0, 1} {
		t.Fatalf("expected P1-V2 indices [0 1], got %v", got[0])
	}
}

func TestIdentifyVolumesToImproveToleratesSinglePartMalformedTarget(t *testing.T) {
	outline := &models.Outline{
		Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{
				{ID: "P1-V1"},
				{ID: "P1-V2"},
			},
		}},
	}
	review := &models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P2-V2-C9"},
		},
	}

	got := (&ComposeAgent{}).identifyVolumesToImprove(outline, review)
	if len(got) != 1 {
		t.Fatalf("expected one volume target, got %v", got)
	}
	if got[0] != [2]int{0, 1} {
		t.Fatalf("expected malformed P2-V2 to resolve to P1-V2 [0 1], got %v", got[0])
	}
}

func TestPreserveImprovedVolumeIdentityKeepsStableIDs(t *testing.T) {
	original := &models.Volume{
		ID: "P1-V10",
		Chapters: []models.Chapter{
			{ID: "P1-V10-C1"},
			{ID: "P1-V10-C2"},
		},
	}
	improved := &models.Volume{
		ID: "part1-volume10",
		Chapters: []models.Chapter{
			{ID: "p1-v10-c1", Title: "Rewritten 1"},
			{ID: "p1-v10-c2", Title: "Rewritten 2"},
			{ID: "p1-v10-c3", Title: "Extra chapter"},
		},
	}

	preserveImprovedVolumeIdentity(original, improved)

	if improved.ID != "P1-V10" {
		t.Fatalf("volume id = %q, want P1-V10", improved.ID)
	}
	if improved.Chapters[0].ID != "P1-V10-C1" || improved.Chapters[1].ID != "P1-V10-C2" {
		t.Fatalf("chapter ids were not preserved: %#v", improved.Chapters)
	}
	if improved.Chapters[2].ID != "p1-v10-c3" {
		t.Fatalf("extra chapter id should be untouched, got %q", improved.Chapters[2].ID)
	}
}

func TestAgentSDKVolumeChangedDetectsPatchAfterPreservingIDs(t *testing.T) {
	original := &models.Volume{
		ID:      "P1-V1",
		Title:   "Original",
		Summary: "same",
		Chapters: []models.Chapter{
			{ID: "P1-V1-C1", Title: "Old", Summary: "same"},
		},
	}
	improved := &models.Volume{
		ID:      "sdk-volume",
		Title:   "Original",
		Summary: "same",
		Chapters: []models.Chapter{
			{ID: "sdk-chapter", Title: "New", Summary: "same"},
		},
	}

	if !agentSDKVolumeChanged(original, improved) {
		t.Fatalf("expected changed volume to be applied")
	}
	if improved.ID != "P1-V1" || improved.Chapters[0].ID != "P1-V1-C1" {
		t.Fatalf("stable IDs were not preserved: %#v", improved)
	}

	same := *original
	same.Chapters = append([]models.Chapter(nil), original.Chapters...)
	if agentSDKVolumeChanged(original, &same) {
		t.Fatalf("unchanged volume should be a no-op")
	}
}

func TestPrepareAgentSDKReturnedVolumePreservesStableIDs(t *testing.T) {
	original := models.Volume{
		ID:      "P1-V2",
		Title:   "Original",
		Summary: "Original summary",
		Chapters: []models.Chapter{
			{ID: "P1-V2-C1", Title: "Old 1"},
			{ID: "P1-V2-C2", Title: "Old 2"},
		},
	}
	returned := models.Volume{
		ID:      "sdk-volume",
		Title:   "Improved",
		Summary: "Improved summary",
		Chapters: []models.Chapter{
			{ID: "sdk-c1", Title: "New 1"},
			{ID: "sdk-c2", Title: "New 2"},
		},
	}

	got, err := prepareAgentSDKReturnedVolume(original, returned, 2)
	if err != nil {
		t.Fatalf("prepare Agent SDK volume: %v", err)
	}
	if got.ID != original.ID {
		t.Fatalf("volume id = %q, want %q", got.ID, original.ID)
	}
	if got.Chapters[0].ID != "P1-V2-C1" || got.Chapters[1].ID != "P1-V2-C2" {
		t.Fatalf("chapter ids were not preserved: %#v", got.Chapters)
	}
	if got.Title != "Improved" || got.Chapters[0].Title != "New 1" {
		t.Fatalf("returned content was not applied: %#v", got)
	}
}

func TestApplyAgentSDKVolumePatchChangesOnlyTargetChapters(t *testing.T) {
	original := models.Volume{
		ID:      "P1-V2",
		Title:   "Original",
		Summary: "Original summary",
		Chapters: []models.Chapter{
			{ID: "P1-V2-C1", Title: "Old 1", Summary: "keep", Characters: []string{"A"}, Location: "Loc", Events: []models.Event{{Type: "status", Subject: "state", Change: "same"}}, Conflict: "same"},
			{
				ID:         "P1-V2-C2",
				Title:      "Old 2",
				Summary:    "old",
				Characters: []string{"B"},
				Location:   "Loc",
				Events:     []models.Event{{Type: "status", Subject: "state", Change: "old"}},
				Conflict:   "old",
				Scenes: []models.OutlineScene{{
					Order:      1,
					POV:        "B",
					Goal:       "keep scene",
					Location:   "Loc",
					Characters: []string{"B"},
					Beats:      []string{"keep beat"},
				}},
			},
		},
	}
	patch := composeVolumePatch{
		ID:      "P1-V2",
		Summary: "Patched summary",
		Chapters: []composeChapterPatch{{
			ID:          "P1-V2-C2",
			Title:       "New 2",
			Summary:     "new",
			Characters:  []string{"B"},
			Location:    "Loc",
			Events:      []composeEventPatch{{Type: "status", Subject: "state", Change: "new"}},
			OpeningBeat: "随后，B前往新地点承接上一章压力。",
			ClosingBeat: "B确认新状态已经落地。",
			Conflict:    "new",
		}},
	}

	got, err := applyAgentSDKVolumePatch(original, patch, 2)
	if err != nil {
		t.Fatalf("apply Agent SDK patch: %v", err)
	}
	if original.Summary != "Original summary" || original.Chapters[1].Title != "Old 2" || original.Chapters[1].Summary != "old" {
		t.Fatalf("patch application mutated original volume: %#v", original)
	}
	if got.Summary != "Patched summary" {
		t.Fatalf("summary = %q", got.Summary)
	}
	if got.Chapters[0].Title != "Old 1" || got.Chapters[0].Summary != "keep" {
		t.Fatalf("unchanged chapter was modified: %#v", got.Chapters[0])
	}
	if got.Chapters[1].ID != "P1-V2-C2" || got.Chapters[1].Title != "New 2" {
		t.Fatalf("changed chapter not applied: %#v", got.Chapters[1])
	}
	if got.Chapters[1].OpeningBeat != "随后，B前往新地点承接上一章压力。" || got.Chapters[1].ClosingBeat != "B确认新状态已经落地。" {
		t.Fatalf("chapter beats not applied: %#v", got.Chapters[1])
	}
	if len(got.Chapters[1].Scenes) != 1 || got.Chapters[1].Scenes[0].Goal != "keep scene" {
		t.Fatalf("unchanged heavy chapter fields were not preserved: %#v", got.Chapters[1].Scenes)
	}
}

func TestApplyAgentSDKVolumePatchMergesPayoffContractFields(t *testing.T) {
	original := models.Volume{
		ID: "P1-V2",
		PayoffContract: &models.VolumePayoffContract{
			VolumeQuestion:      "Can Lin survive the first signal?",
			PowerPromise:        "Signal reading creates tactical advantage.",
			MainOpponentMisread: "The rival thinks Lin is blind.",
			VisibleReward:       "Lin gains a map.",
			ReputationShift:     "Lin becomes credible.",
			NextBiggerGame:      "The war expands.",
		},
		Chapters: []models.Chapter{{ID: "P1-V2-C1"}},
	}

	got, err := applyAgentSDKVolumePatch(original, composeVolumePatch{
		ID: "P1-V2",
		PayoffContract: &models.VolumePayoffContract{
			BigWin: "Lin wins the first public duel.",
		},
	}, 1)
	if err != nil {
		t.Fatalf("apply Agent SDK patch: %v", err)
	}
	payoff := got.PayoffContract
	if payoff == nil {
		t.Fatal("payoff contract was removed")
	}
	if payoff.BigWin != "Lin wins the first public duel." ||
		payoff.VolumeQuestion != "Can Lin survive the first signal?" ||
		payoff.PowerPromise != "Signal reading creates tactical advantage." ||
		payoff.MainOpponentMisread != "The rival thinks Lin is blind." ||
		payoff.VisibleReward != "Lin gains a map." ||
		payoff.ReputationShift != "Lin becomes credible." ||
		payoff.NextBiggerGame != "The war expands." {
		t.Fatalf("payoff contract should be field-merged: %#v", payoff)
	}
}

func TestApplyAgentSDKVolumePatchMergesScenes(t *testing.T) {
	original := models.Volume{
		ID: "P1-V2",
		Chapters: []models.Chapter{{
			ID: "P1-V2-C1",
			Scenes: []models.OutlineScene{
				{Order: 1, POV: "Lin", Goal: "old goal 1", Beats: []string{"old beat 1"}},
				{Order: 2, POV: "Lin", Goal: "old goal 2", Beats: []string{"old beat 2"}},
				{Order: 3, POV: "Lin", Goal: "old goal 3", Beats: []string{"old beat 3"}},
			},
		}},
	}
	got, err := applyAgentSDKVolumePatch(original, composeVolumePatch{
		ID: "P1-V2",
		Chapters: []composeChapterPatch{{
			ID: "P1-V2-C1",
			Scenes: []models.OutlineScene{{
				Order: 2,
				POV:   "Lin",
				Goal:  "new goal 2",
				Beats: []string{"new beat 2"},
			}},
		}},
	}, 1)
	if err != nil {
		t.Fatalf("apply Agent SDK patch: %v", err)
	}
	scenes := got.Chapters[0].Scenes
	if len(scenes) != 3 {
		t.Fatalf("partial scene patch truncated the scene list: %#v", scenes)
	}
	if scenes[0].Goal != "old goal 1" || scenes[0].Beats[0] != "old beat 1" {
		t.Fatalf("untouched scene 1 should be preserved: %#v", scenes[0])
	}
	if scenes[1].Goal != "new goal 2" || scenes[1].Beats[0] != "new beat 2" {
		t.Fatalf("scene 2 should be replaced by order: %#v", scenes[1])
	}
	if scenes[2].Goal != "old goal 3" || scenes[2].Beats[0] != "old beat 3" {
		t.Fatalf("untouched scene 3 should be preserved: %#v", scenes[2])
	}
}

func TestApplyAgentSDKVolumePatchChangedEvents(t *testing.T) {
	original := models.Volume{
		ID: "P1-V2",
		Chapters: []models.Chapter{{
			ID: "P1-V2-C1",
			Events: []models.Event{{
				Actor:  "Lin",
				Action: models.ActionAcquire,
				Target: "Star Core",
			}, {
				Actor:  "Lin",
				Action: models.ActionMove,
				Target: "Camp",
			}},
		}},
	}

	got, err := applyAgentSDKVolumePatch(original, composeVolumePatch{
		ID: "P1-V2",
		Events: []composeChangedEventPatch{{
			ChapterID:  "P1-V2-C1",
			EventIndex: 1,
			Type:       "plan",
			Action:     "plan",
			Details:    "Lin chooses a safer route.",
		}},
	}, 1)
	if err != nil {
		t.Fatalf("apply Agent SDK patch: %v", err)
	}
	events := got.Chapters[0].Events
	if len(events) != 2 {
		t.Fatalf("event array length changed: %#v", events)
	}
	if events[1].Type != "plan" || events[1].Action != "plan" || events[1].Details != "Lin chooses a safer route." {
		t.Fatalf("target event not patched: %#v", events[1])
	}
	if events[1].Target != "Camp" || events[0].Target != "Star Core" {
		t.Fatalf("event patch should preserve existing fields and untouched events: %#v", events)
	}
}

func TestApplyAgentSDKVolumePatchRejectsUnknownChapter(t *testing.T) {
	original := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{{ID: "P1-V2-C1"}}}
	_, err := applyAgentSDKVolumePatch(original, composeVolumePatch{
		ID:       "P1-V2",
		Chapters: []composeChapterPatch{{ID: "P1-V2-C99"}},
	}, 1)
	if err == nil {
		t.Fatalf("expected unknown chapter error")
	}
}

func TestApplyAgentSDKVolumePatchRejectsGarbledText(t *testing.T) {
	original := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{{ID: "P1-V2-C1"}}}
	_, err := applyAgentSDKVolumePatch(original, composeVolumePatch{
		ID:       "P1-V2",
		Chapters: []composeChapterPatch{{ID: "P1-V2-C1", Summary: "锟斤拷锟竭????"}},
	}, 1)
	if err == nil || !strings.Contains(err.Error(), "suspicious") {
		t.Fatalf("expected garbled patch rejection, got %v", err)
	}
}

func TestValidateAgentSDKImprovePatchOutputRejectsEmptyStructuredOutput(t *testing.T) {
	err := validateAgentSDKImprovePatchOutput(composeAgentSDKImprovePatchOutput{}, "P1-V1")
	if err == nil || !strings.Contains(err.Error(), "volume_patch.id") {
		t.Fatalf("expected missing volume_patch.id error, got %v", err)
	}
}

func TestValidateAgentSDKImprovePatchOutputRejectsEmptyReviewResult(t *testing.T) {
	err := validateAgentSDKImprovePatchOutput(composeAgentSDKImprovePatchOutput{
		VolumePatch: composeVolumePatch{ID: "P1-V1"},
	}, "P1-V1")
	if err == nil || !strings.Contains(err.Error(), "empty review_result") {
		t.Fatalf("expected empty review_result error, got %v", err)
	}
}

func TestValidateAgentSDKImprovePatchOutputAllowsZeroScoreWithSummary(t *testing.T) {
	err := validateAgentSDKImprovePatchOutput(composeAgentSDKImprovePatchOutput{
		ReviewResult: composeAgentSDKReviewResult{Summary: "目标卷存在阻塞问题。"},
		VolumePatch:  composeVolumePatch{ID: "P1-V1"},
	}, "P1-V1")
	if err != nil {
		t.Fatalf("zero score with explicit summary should be valid: %v", err)
	}
}

func TestComposeVolumePatchSchemaUsesCompactChapterPatch(t *testing.T) {
	schema := utils.StructToJSONSchema(composeAgentSDKImprovePatchOutput{}, "")
	if !strings.Contains(schema, "scenes") {
		t.Fatalf("compact Agent SDK patch schema should expose scene/beat patches (changed_chapters[].scenes):\n%s", schema)
	}
	if strings.Contains(schema, "strengths") || strings.Contains(schema, "weaknesses") || strings.Contains(schema, "continuity_issues") || strings.Contains(schema, "dimensions") {
		t.Fatalf("compact Agent SDK review schema should not expose full review fields:\n%s", schema)
	}
	if strings.Contains(schema, "Event type: relationship") || !strings.Contains(schema, "Narrative result") {
		t.Fatalf("compact Agent SDK event patch schema should use short event descriptions:\n%s", schema)
	}
	if !strings.Contains(schema, "changed_chapters") || !strings.Contains(schema, "resource_ledger") || !strings.Contains(schema, "opening_beat") || !strings.Contains(schema, "closing_beat") {
		t.Fatalf("compact Agent SDK patch schema missing expected patch fields:\n%s", schema)
	}
}

func TestComposeAgentSDKApplySchemaOmitsVolumePatch(t *testing.T) {
	applySchema := utils.StructToJSONSchema(composeAgentSDKApplyOutput{}, "")
	patchSchema := utils.StructToJSONSchema(composeAgentSDKImprovePatchOutput{}, "")
	for _, unwanted := range []string{"volume_patch", "changed_chapters", "storyline_advances", "Narrative result"} {
		if strings.Contains(applySchema, unwanted) {
			t.Fatalf("apply-mode schema should not include patch field %q:\n%s", unwanted, applySchema)
		}
	}
	for _, want := range []string{"review_result", "applied_patches", "applied_patch_count", "final_check"} {
		if !strings.Contains(applySchema, want) {
			t.Fatalf("apply-mode schema missing %q:\n%s", want, applySchema)
		}
	}
	if len(applySchema) >= len(patchSchema)/2 {
		t.Fatalf("apply-mode schema should be substantially smaller: apply=%d patch=%d", len(applySchema), len(patchSchema))
	}
}

func TestValidateAgentSDKApplyOutputRejectsEmptyReview(t *testing.T) {
	err := validateAgentSDKApplyOutput(composeAgentSDKApplyOutput{})
	if err == nil || !strings.Contains(err.Error(), "empty review_result") {
		t.Fatalf("expected empty review_result error, got %v", err)
	}
}

func TestValidateAgentSDKApplyOutputRejectsNegativePatchCount(t *testing.T) {
	err := validateAgentSDKApplyOutput(composeAgentSDKApplyOutput{
		ReviewResult:      composeAgentSDKReviewResult{Summary: "clean"},
		AppliedPatchCount: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "negative applied_patch_count") {
		t.Fatalf("expected negative patch count error, got %v", err)
	}
}

func TestComposeAgentSDKReviewResultConvertsToModelReview(t *testing.T) {
	compact := composeAgentSDKReviewResult{
		OverallScore: 9.2,
		Summary:      strings.Repeat("修", 600),
	}
	for i := 0; i < 10; i++ {
		compact.Suggestions = append(compact.Suggestions, composeAgentSDKReviewSuggestion{
			Category:   "logic",
			TargetID:   "P1-V1-C1",
			TargetName: strings.Repeat("章", 200),
			Issue:      strings.Repeat("问", 500),
			Suggestion: strings.Repeat("改", 500),
			Priority:   models.PriorityMedium,
		})
	}

	got := compact.toModelReviewResult()
	got.NormalizeScoreScale()
	if got.OverallScore != 92 {
		t.Fatalf("score = %v, want 92", got.OverallScore)
	}
	if len([]rune(got.Summary)) > 503 {
		t.Fatalf("summary was not clipped: %d", len([]rune(got.Summary)))
	}
	if len(got.Suggestions) != 8 {
		t.Fatalf("suggestion count = %d, want 8", len(got.Suggestions))
	}
	if len([]rune(got.Suggestions[0].Issue)) > 363 || len([]rune(got.Suggestions[0].Suggestion)) > 423 {
		t.Fatalf("suggestion text was not clipped: %#v", got.Suggestions[0])
	}
}

func TestBuildAgentSDKImproveVolumePromptInputUsesTargetQueries(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part: models.Part{ID: "P1", Title: "Part"},
		Volume: models.Volume{
			ID:      "P1-V2",
			Title:   "Target",
			Summary: strings.Repeat("summary", 100),
			Chapters: []models.Chapter{
				{ID: "P1-V2-C1", Title: "Chapter 1", Summary: strings.Repeat("chapter", 100)},
				{ID: "P1-V2-C2", Title: "Chapter 2"},
			},
		},
		ReviewResult: models.ReviewResult{
			Suggestions: []models.ReviewSuggestion{{TargetID: "P1-V2", Issue: "fix me", Priority: "high"}},
		},
		UserPrompt: "tighten causality",
	}

	got := buildAgentSDKImproveVolumePromptInput(input, false, false)
	if got.TargetVolumeID != "P1-V2" || got.ChapterCount != 2 {
		t.Fatalf("target payload = %#v", got)
	}
	if len(got.RequiredQueries) != 1 || got.RequiredQueries[0] != "novelgen tool query context --type outline-volume --id P1-V2 --view index" {
		t.Fatalf("required queries = %#v", got.RequiredQueries)
	}
	if got.ReviewResult.Suggestions[0].Issue != "fix me" {
		t.Fatalf("review feedback missing: %#v", got.ReviewResult)
	}
	if got.ReviewResult.Suggestions[0].Navigation == nil ||
		got.ReviewResult.Suggestions[0].Navigation.FocusedCheckQuery == "" ||
		!strings.Contains(got.ReviewResult.Suggestions[0].Navigation.PatchQuery, "P1-V2") {
		t.Fatalf("review feedback should include focused navigation: %#v", got.ReviewResult.Suggestions[0])
	}
	if !containsString(got.Instructions, "review_result contains suggestions") {
		t.Fatalf("instructions should prioritize review_result targets: %#v", got.Instructions)
	}
}

func TestBuildAgentSDKImproveVolumePromptInputAddsChapterNavigation(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part:   models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V2", Title: "Target", Chapters: []models.Chapter{{ID: "P1-V2-C1"}}},
		ReviewResult: models.ReviewResult{
			Suggestions: []models.ReviewSuggestion{{
				Category:   "state_anchor",
				TargetID:   "P1-V2-C1",
				TargetName: "Chapter 1",
				Issue:      "state changed without event",
				Priority:   models.PriorityMedium,
			}},
		},
	}

	got := buildAgentSDKImproveVolumePromptInput(input, false, false)
	if len(got.ReviewResult.Suggestions) != 1 {
		t.Fatalf("suggestions = %#v", got.ReviewResult.Suggestions)
	}
	nav := got.ReviewResult.Suggestions[0].Navigation
	if nav == nil {
		t.Fatalf("navigation missing: %#v", got.ReviewResult.Suggestions[0])
	}
	if nav.TargetKind != "chapter" || nav.VolumeID != "P1-V2" {
		t.Fatalf("navigation target = %+v", nav)
	}
	joined := strings.Join(nav.DetailQueries, "\n")
	for _, want := range []string{
		`novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C1" --fields result,details,target,target_type,actor,action --view brief`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("detail query %q missing from %#v", want, nav.DetailQueries)
		}
	}
	if !strings.Contains(nav.FocusedCheckQuery, `--scope chapter --id "P1-V2-C1" --category state_anchor`) {
		t.Fatalf("focused check query = %q", nav.FocusedCheckQuery)
	}
	if !strings.Contains(nav.RepairRouteQuery, `outline-repair --id "P1-V2-C1" --name "state_anchor" --view index`) {
		t.Fatalf("repair route query = %q", nav.RepairRouteQuery)
	}
	if !strings.Contains(nav.RepairContextQuery, `outline-repair --id "P1-V2-C1" --name "state_anchor"`) {
		t.Fatalf("repair context query = %q", nav.RepairContextQuery)
	}
	if nav.PatchShape == nil {
		t.Fatalf("patch shape missing")
	}
}

func TestBuildAgentSDKImprovePromptAllowsApplyOnlyWhenRequested(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part:   models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V1", Title: "Target", Chapters: []models.Chapter{{ID: "P1-V1-C1"}}},
	}

	dryRun := buildAgentSDKImproveVolumePromptInput(input, false, false)
	if dryRun.ApplyPatches {
		t.Fatalf("dry-run prompt ApplyPatches = true")
	}
	dryRunInstructions := strings.Join(dryRun.Instructions, "\n")
	if !strings.Contains(dryRunInstructions, "do not use --apply") {
		t.Fatalf("dry-run instructions should forbid apply: %#v", dryRun.Instructions)
	}

	apply := buildAgentSDKImproveVolumePromptInput(input, true, false)
	if !apply.ApplyPatches {
		t.Fatalf("apply prompt ApplyPatches = false")
	}
	applyInstructions := strings.Join(apply.Instructions, "\n")
	if !strings.Contains(applyInstructions, "repeat the same stdin-piped patch command with `--apply`") ||
		!strings.Contains(applyInstructions, "printf '%s' '<compact-json>' | novelgen tool patch outline") ||
		!strings.Contains(applyInstructions, "tool check all") {
		t.Fatalf("apply instructions should require dry-run/apply/check: %#v", apply.Instructions)
	}
	if !strings.Contains(applyInstructions, "Do not return volume_patch") ||
		!strings.Contains(applyInstructions, "Go reloads the saved outline") {
		t.Fatalf("apply instructions should use reload-based output: %#v", apply.Instructions)
	}
	if strings.Contains(applyInstructions, "Return review_result and volume_patch") {
		t.Fatalf("apply instructions should not ask final output for volume_patch: %#v", apply.Instructions)
	}
}

func TestBuildAgentSDKImprovePromptWithoutFocusedTasksAvoidsLowSweep(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part:   models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V1", Title: "Target", Chapters: []models.Chapter{{ID: "P1-V1-C1"}}},
	}

	got := buildAgentSDKImproveVolumePromptInput(input, true, false)
	instructions := strings.Join(got.Instructions, "\n")
	for _, want := range []string{
		"No focused review suggestions or user repair request were supplied",
		"do not run low-priority broad checks",
		"do not sweep chapter-by-chapter",
		"return applied_patches=false",
	} {
		if !strings.Contains(instructions, want) {
			t.Fatalf("instructions missing %q: %#v", want, got.Instructions)
		}
	}

	dryRun := buildAgentSDKImproveVolumePromptInput(input, false, false)
	if !strings.Contains(strings.Join(dryRun.Instructions, "\n"), "return an empty patch") {
		t.Fatalf("dry-run instructions should still request an empty returned patch: %#v", dryRun.Instructions)
	}
}

func TestBuildAgentSDKImprovePromptScopesMultiVolumeUserPrompt(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part:   models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V2", Title: "Target", Chapters: []models.Chapter{{ID: "P1-V2-C1"}}},
		UserPrompt: scopeComposeAgentSDKVolumeUserPrompt(
			`请先分别对第1卷和第2卷运行 outline volume focused check。如果 check 返回 0 issue，不要 patch。`,
			"P1-V2",
			"Target",
		),
	}

	if strings.Contains(input.UserPrompt, "第1卷") || strings.Contains(input.UserPrompt, "第2卷") {
		t.Fatalf("scoped prompt should remove cross-volume ordinal wording: %s", input.UserPrompt)
	}
	if !strings.Contains(input.UserPrompt, "当前卷") || !strings.Contains(input.UserPrompt, "P1-V2") {
		t.Fatalf("scoped prompt should name current volume: %s", input.UserPrompt)
	}

	got := buildAgentSDKImproveVolumePromptInput(input, true, true)
	instructions := strings.Join(got.Instructions, "\n")
	for _, want := range []string{
		"already scoped to target_volume_id",
		"Do not query, check, or patch any other volume",
		"check-first/no-op-if-clean",
		"return final JSON immediately",
	} {
		if !strings.Contains(instructions, want) {
			t.Fatalf("scoped prompt instructions missing %q: %#v", want, got.Instructions)
		}
	}
}

func TestBuildAgentSDKImprovePromptForceIssueRepairTreatsLowAsTasks(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part:   models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V1", Title: "Target", Chapters: []models.Chapter{{ID: "P1-V1-C1"}}},
		ReviewResult: models.ReviewResult{
			Suggestions: []models.ReviewSuggestion{{
				Category: "logic",
				TargetID: "P1-V1-C1",
				Issue:    "low issue still needs a concrete result",
				Priority: models.PriorityLow,
			}},
		},
	}

	normal := buildAgentSDKImproveVolumePromptInput(input, true, false)
	if normal.ForceIssueRepair {
		t.Fatalf("normal prompt ForceIssueRepair = true")
	}
	if strings.Contains(strings.Join(normal.Instructions, "\n"), "low priority") {
		t.Fatalf("normal prompt should not force low-priority repairs: %#v", normal.Instructions)
	}

	forced := buildAgentSDKImproveVolumePromptInput(input, true, true)
	if !forced.ForceIssueRepair {
		t.Fatalf("forced prompt ForceIssueRepair = false")
	}
	instructions := strings.Join(forced.Instructions, "\n")
	for _, want := range []string{
		"even when they are low priority",
		"directly fixable issues",
		"Do not dismiss low-priority focused issues only because the medium+ scoped check passes",
		"prefer a small dry-run/apply/check cycle",
	} {
		if !strings.Contains(instructions, want) {
			t.Fatalf("forced instructions missing %q: %#v", want, forced.Instructions)
		}
	}
}

func TestShouldForceAgentSDKIssueRepairRequiresFocusedTask(t *testing.T) {
	if shouldForceAgentSDKIssueRepair(false, "tighten", models.ReviewResult{Suggestions: []models.ReviewSuggestion{{TargetID: "P1-V1-C1"}}}) {
		t.Fatalf("force repair should be disabled when forceImprove=false")
	}
	if shouldForceAgentSDKIssueRepair(true, "", models.ReviewResult{}) {
		t.Fatalf("force repair should not be enabled without suggestions or user prompt")
	}
	if !shouldForceAgentSDKIssueRepair(true, "tighten causal chain", models.ReviewResult{}) {
		t.Fatalf("user prompt should enable force repair under forceImprove")
	}
	if !shouldForceAgentSDKIssueRepair(true, "", models.ReviewResult{Suggestions: []models.ReviewSuggestion{{TargetID: "P1-V1-C1"}}}) {
		t.Fatalf("focused suggestions should enable force repair under forceImprove")
	}
}

func TestFilterReviewForVolumeDropsNonBlockingGlobalNoise(t *testing.T) {
	review := &models.ReviewResult{
		OverallScore: 79,
		Summary:      "precheck",
		Strengths:    []string{"keep out"},
		Weaknesses:   []string{"keep out"},
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V1-C1", Issue: "target issue", Priority: models.PriorityLow},
			{TargetID: "P1-V2-C1", Issue: "other volume", Priority: models.PriorityHigh},
			{TargetID: "global", Issue: "global low", Priority: models.PriorityLow},
			{TargetID: "", Issue: "global high", Priority: models.PriorityHigh},
		},
	}

	got := (&ComposeAgent{}).filterReviewForVolume(review, "P1-V1")
	if len(got.Suggestions) != 2 {
		t.Fatalf("suggestions = %#v", got.Suggestions)
	}
	if got.Suggestions[0].Issue != "global high" || got.Suggestions[1].Issue != "target issue" {
		t.Fatalf("unexpected filtered suggestions: %#v", got.Suggestions)
	}
	if len(got.Strengths) != 0 || len(got.Weaknesses) != 0 {
		t.Fatalf("strengths/weaknesses should be omitted from targeted review: %#v %#v", got.Strengths, got.Weaknesses)
	}
}

func TestFilterReviewForVolumePrioritizesBlockingIssues(t *testing.T) {
	review := &models.ReviewResult{}
	for i := 0; i < 20; i++ {
		review.Suggestions = append(review.Suggestions, models.ReviewSuggestion{
			TargetID: "P1-V1-C1",
			Issue:    "low",
			Priority: models.PriorityLow,
		})
	}
	review.Suggestions = append(review.Suggestions, models.ReviewSuggestion{
		TargetID: "P1-V1-C9",
		Issue:    "medium",
		Priority: models.PriorityMedium,
	})

	got := (&ComposeAgent{}).filterReviewForVolume(review, "P1-V1")
	if len(got.Suggestions) != 12 {
		t.Fatalf("suggestions = %d, want capped 12", len(got.Suggestions))
	}
	if got.Suggestions[0].Issue != "medium" {
		t.Fatalf("medium issue should be first, got %#v", got.Suggestions[:2])
	}
}

func TestHasMediumOrHigherReviewSuggestion(t *testing.T) {
	lowOnly := models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V1-C1", Priority: models.PriorityLow},
		},
	}
	if hasMediumOrHigherReviewSuggestion(lowOnly) {
		t.Fatalf("low-only review should not require Agent SDK improve")
	}

	medium := models.ReviewResult{
		Suggestions: []models.ReviewSuggestion{
			{TargetID: "P1-V1-C1", Priority: models.PriorityMedium},
		},
	}
	if !hasMediumOrHigherReviewSuggestion(medium) {
		t.Fatalf("medium review should require Agent SDK improve")
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func containsExactString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestBuildAgentSDKChaptersPromptInputUsesTargetQueries(t *testing.T) {
	input := ComposeChaptersInput{
		Part:           models.Part{ID: "P1"},
		Volume:         models.Volume{ID: "P1-V2", Title: "Target", Summary: strings.Repeat("summary", 100)},
		VolumeIndex:    2,
		TotalVolumes:   4,
		ChaptersPerVol: 10,
		PreviousVolume: &models.Volume{ID: "P1-V1", Title: "Previous"},
	}

	got := buildAgentSDKChaptersPromptInput(input)
	if got.TargetVolumeID != "P1-V2" || got.PreviousVolumeID != "P1-V1" {
		t.Fatalf("target payload = %#v", got)
	}
	if len(got.RequiredQueries) != 1 {
		t.Fatalf("required queries = %#v", got.RequiredQueries)
	}
	if got.RequiredQueries[0] != "novelgen tool query context --type outline-volume --id P1-V2 --view brief" {
		t.Fatalf("required query = %#v", got.RequiredQueries)
	}
}

func TestComposeAgentSDKParamsUseClaudeToolsAndGenerousTurns(t *testing.T) {
	params := composeAgentSDKParams("review", "outline-improve-volume-workflow", 28, []string{"novelgen tool query context --type outline-volume --id P1-V1 --view brief"})
	if !params.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	if params.MaxTurns != 28 {
		t.Fatalf("MaxTurns = %d, want 28", params.MaxTurns)
	}
	if params.Timeout != 900 {
		t.Fatalf("Timeout = %d, want 900", params.Timeout)
	}
	if len(params.SDKSkills) != 2 || params.SDKSkills[0] != "novel-tools-core" || params.SDKSkills[1] != "outline-improve-volume-workflow" {
		t.Fatalf("SDKSkills = %#v", params.SDKSkills)
	}
	if len(params.Tools) != 1 || params.Tools[0] != "Bash" {
		t.Fatalf("Tools = %#v", params.Tools)
	}
	if !containsExactString(params.ToolAllowlist, "novelgen tool query context --type outline-volume --id P1-V1 --view brief") ||
		!containsExactString(params.ToolAllowlist, "novelgen tool query logs --type prompts --view index") ||
		!containsExactString(params.ToolAllowlist, "novelgen tool query logs --id") ||
		containsExactString(params.ToolAllowlist, "novelgen tool query") {
		t.Fatalf("ToolAllowlist = %#v", params.ToolAllowlist)
	}
}

func TestComposeAgentSDKParamsAllowApplyOnlyWhenRequested(t *testing.T) {
	volume := models.Volume{ID: "P1-V1", Chapters: []models.Chapter{{ID: "P1-V1-C1"}}}
	promptInput := composeAgentSDKImproveVolumePromptInput{
		TargetVolumeID: "P1-V1",
		RequiredQueries: []string{
			"novelgen tool query context --type outline-volume --id P1-V1 --view index",
		},
	}
	dryRun := composeAgentSDKParams("review", "outline-improve-volume-workflow", 28, composeImproveToolAllowlist(volume, promptInput, false))
	if strings.Contains(strings.Join(dryRun.ToolAllowlist, " "), "--apply") {
		t.Fatalf("dry-run ToolAllowlist should not allow apply: %#v", dryRun.ToolAllowlist)
	}
	if !containsString(dryRun.ToolAllowlist, "outline-repair --id P1-V1-C1 --name") ||
		!containsString(dryRun.ToolAllowlist, "tool check all --target outline --scope volume --id \"P1-V1\"") ||
		!containsExactString(dryRun.ToolAllowlist, `novelgen tool patch outline --target volume --id "P1-V1"`) {
		t.Fatalf("dry-run ToolAllowlist missing target-local entries: %#v", dryRun.ToolAllowlist)
	}
	if containsExactString(dryRun.ToolAllowlist, `novelgen tool query context --type outline-volume --id P1-V1`) {
		t.Fatalf("no-task dry-run should not allow broad same-volume context beyond required index: %#v", dryRun.ToolAllowlist)
	}
	if containsExactString(dryRun.ToolAllowlist, `novelgen tool query outline --type chapter --id "P1-V1-C1"`) ||
		containsExactString(dryRun.ToolAllowlist, `novelgen tool query outline --type events --chapter-id "P1-V1-C1"`) {
		t.Fatalf("dry-run ToolAllowlist should not allow direct chapter/event detail without issue navigation: %#v", dryRun.ToolAllowlist)
	}
	if containsExactString(dryRun.ToolAllowlist, "novelgen tool query") || containsExactString(dryRun.ToolAllowlist, "novelgen tool check") {
		t.Fatalf("dry-run ToolAllowlist should not grant broad query/check: %#v", dryRun.ToolAllowlist)
	}

	apply := composeAgentSDKParams("review", "outline-improve-volume-workflow", 28, composeImproveToolAllowlist(volume, promptInput, true))
	if !containsExactString(apply.ToolAllowlist, `novelgen tool patch outline --target volume --id "P1-V1" --apply`) {
		t.Fatalf("apply ToolAllowlist = %#v", apply.ToolAllowlist)
	}
}

func TestComposeOutlineReviewAgentSDKUsesReadOnlyWorkflow(t *testing.T) {
	outline := models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID:       "P1-V1",
			Title:    "第一卷",
			Chapters: []models.Chapter{{ID: "P1-V1-C1"}, {ID: "P1-V1-C2"}},
		}},
	}}}
	runtime := &fakeComposeImproveRuntime{result: &agentruntime.Result{Content: `{
		"overall_score": 85,
		"summary": "整体不错",
		"strengths": ["结构清晰"],
		"suggestions": [
			{"category": "character", "target_id": "P1-V1-C2", "target_name": "第二章", "issue": "动机不足", "suggestion": "补充铺垫", "priority": "high"}
		]
	}`, LiveSummary: &agentruntime.LiveSummary{
		QueryCalls:          1,
		AllowedToolCommands: []string{"novelgen tool query outline --type all --view index"},
	}}}
	agent := &ComposeAgent{base: &BaseAgent{name: "ComposeAgent", runtime: runtime, config: &llm.Config{}, language: "zh"}}
	review, err := agent.ReviewOutlineWithAgentSDK(context.Background(), ComposeOutlineReviewInput{Outline: outline, UserPrompt: "检查角色动机"})
	if err != nil {
		t.Fatalf("ReviewOutlineWithAgentSDK() error = %v", err)
	}
	if review.OverallScore != 85 || len(review.Suggestions) != 1 || review.Suggestions[0].TargetID != "P1-V1-C2" {
		t.Fatalf("review = %#v", review)
	}

	invocation := runtime.invocation
	if len(invocation.SDKSkills) != 2 || invocation.SDKSkills[0] != "novel-tools-core" || invocation.SDKSkills[1] != "outline-review-workflow" {
		t.Fatalf("SDKSkills = %#v", invocation.SDKSkills)
	}
	if !invocation.RequireSDK {
		t.Fatalf("RequireSDK = false")
	}
	joined := strings.Join(invocation.ToolAllowlist, "\n")
	if strings.Contains(joined, " tool patch ") || strings.Contains(joined, " tool check ") || strings.Contains(joined, " tool patch-buffer ") {
		t.Fatalf("review allowlist must be read-only: %#v", invocation.ToolAllowlist)
	}
	if !containsExactString(invocation.ToolAllowlist, "novelgen tool query outline --type all --view index") {
		t.Fatalf("whole-outline review allowlist missing index query: %#v", invocation.ToolAllowlist)
	}
	if !containsExactString(invocation.ToolAllowlist, `novelgen tool query outline --type volume --id "P1-V1" --view brief`) {
		t.Fatalf("review allowlist missing volume brief: %#v", invocation.ToolAllowlist)
	}
	if !containsExactString(invocation.ToolAllowlist, `novelgen tool query outline --type chapter --id "P1-V1-C2" --view brief`) ||
		!containsExactString(invocation.ToolAllowlist, `novelgen tool query outline --type events --chapter-id "P1-V1-C1" --view brief`) {
		t.Fatalf("review allowlist missing chapter-level reads: %#v", invocation.ToolAllowlist)
	}
	if invocation.ToolEvidence.MinQueryCalls != 1 || !invocation.ToolEvidence.RequireNoDeniedTools {
		t.Fatalf("tool evidence = %#v", invocation.ToolEvidence)
	}
	if len(invocation.ToolEvidence.RequiredToolCommands) != 1 || invocation.ToolEvidence.RequiredToolCommands[0] != "novelgen tool query outline --type all --view index" {
		t.Fatalf("required tool commands = %#v", invocation.ToolEvidence.RequiredToolCommands)
	}
	if invocation.Options.MaxTurns != 20 || invocation.Options.Timeout != 600 {
		t.Fatalf("options = %#v", invocation.Options)
	}
}

func TestComposeOutlineReviewAgentSDKScopedVolume(t *testing.T) {
	runtime := &fakeComposeImproveRuntime{result: &agentruntime.Result{
		Content: `{"overall_score": 90, "summary": "ok"}`,
		LiveSummary: &agentruntime.LiveSummary{
			QueryCalls:          1,
			AllowedToolCommands: []string{"novelgen tool query context --type outline-volume --id P1-V2 --view index"},
		},
	}}
	agent := &ComposeAgent{base: &BaseAgent{name: "ComposeAgent", runtime: runtime, config: &llm.Config{}, language: "zh"}}
	outline := models.Outline{Parts: []models.Part{{ID: "P1", Volumes: []models.Volume{{ID: "P1-V1"}, {ID: "P1-V2"}}}}}
	if _, err := agent.ReviewOutlineWithAgentSDK(context.Background(), ComposeOutlineReviewInput{Outline: outline, VolumeID: "P1-V2"}); err != nil {
		t.Fatalf("ReviewOutlineWithAgentSDK() error = %v", err)
	}
	joined := strings.Join(runtime.invocation.ToolAllowlist, "\n")
	if strings.Contains(joined, "P1-V1") {
		t.Fatalf("scoped review allowlist must not include other volumes: %#v", runtime.invocation.ToolAllowlist)
	}
	if !containsExactString(runtime.invocation.ToolAllowlist, `novelgen tool query outline --type volume --id "P1-V2" --view brief`) {
		t.Fatalf("scoped review allowlist missing target volume: %#v", runtime.invocation.ToolAllowlist)
	}
	if len(runtime.invocation.ToolEvidence.RequiredToolCommands) != 1 ||
		runtime.invocation.ToolEvidence.RequiredToolCommands[0] != "novelgen tool query context --type outline-volume --id P1-V2 --view index" {
		t.Fatalf("scoped required tool commands = %#v", runtime.invocation.ToolEvidence.RequiredToolCommands)
	}
}

func TestComposeImproveAgentSDKRequiresContextAndCheckEvidence(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part:   models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V1", Chapters: []models.Chapter{{ID: "P1-V1-C1"}}},
	}
	promptInput := buildAgentSDKImproveVolumePromptInput(input, true, false)
	params := composeAgentSDKParams("review", "outline-improve-volume-workflow", 28, composeImproveToolAllowlist(input.Volume, promptInput, true))
	params.ToolEvidence = composeImproveToolEvidence(promptInput, true)
	if params.ToolEvidence.MinContextQueryCalls != 1 ||
		params.ToolEvidence.MinCheckCalls != 1 ||
		params.ToolEvidence.MaxQueryCalls != 2 ||
		params.ToolEvidence.MaxContextQueryCalls != 2 ||
		params.ToolEvidence.DisallowQueryBriefCalls ||
		!params.ToolEvidence.DisallowQueryFullCalls ||
		!params.ToolEvidence.RequireNoDeniedTools ||
		!params.ToolEvidence.RequirePatchApplyFollowupCheck {
		t.Fatalf("tool evidence = %#v", params.ToolEvidence)
	}

	focusedInput := input
	focusedInput.UserPrompt = "修复第一章逻辑"
	focusedPrompt := buildAgentSDKImproveVolumePromptInput(focusedInput, true, false)
	focusedEvidence := composeImproveToolEvidence(focusedPrompt, true)
	if focusedEvidence.MaxQueryCalls != 0 ||
		focusedEvidence.MaxContextQueryCalls != 0 ||
		focusedEvidence.DisallowQueryBriefCalls ||
		focusedEvidence.DisallowQueryFullCalls {
		t.Fatalf("focused tool evidence should not use clean-path query caps: %#v", focusedEvidence)
	}

	cleanProbeInput := input
	cleanProbeInput.UserPrompt = scopeComposeAgentSDKVolumeUserPrompt("如果 check 返回 0 issue，不要为了润色而改写", "P1-V1", "Target")
	cleanProbePrompt := buildAgentSDKImproveVolumePromptInput(cleanProbeInput, true, true)
	cleanProbeEvidence := composeImproveToolEvidence(cleanProbePrompt, true)
	if cleanProbeEvidence.MaxQueryCalls != 2 ||
		cleanProbeEvidence.MaxContextQueryCalls != 2 ||
		cleanProbeEvidence.DisallowQueryBriefCalls ||
		!cleanProbeEvidence.DisallowQueryFullCalls {
		t.Fatalf("clean-probe evidence should cap detail queries: %#v", cleanProbeEvidence)
	}
}

func TestComposeImproveToolAllowlistIncludesSuggestionNavigation(t *testing.T) {
	volume := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{{ID: "P1-V2-C1"}}}
	promptInput := composeAgentSDKImproveVolumePromptInput{
		TargetVolumeID: "P1-V2",
		RequiredQueries: []string{
			"novelgen tool query context --type outline-volume --id P1-V2 --view index",
		},
		ReviewResult: composeAgentSDKPromptReviewResult{Suggestions: []composeAgentSDKPromptReviewSuggestion{{
			Navigation: &composeAgentSDKSuggestionNavigation{
				DetailQueries: []string{
					`novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`,
				},
				FocusedCheckQuery:  `novelgen tool check all --target outline --scope chapter --id "P1-V2-C1" --category logic --min-priority low --max-issues 8`,
				RepairRouteQuery:   `novelgen tool query context --type outline-repair --id "P1-V2-C1" --name "logic" --view index`,
				RepairContextQuery: `novelgen tool query context --type outline-repair --id "P1-V2-C1" --name "logic" --view brief`,
			},
		}}},
	}

	got := composeImproveToolAllowlist(volume, promptInput, false)
	for _, want := range []string{
		`novelgen tool query context --type outline-volume --id P1-V2 --view index`,
		`novelgen tool query context --type outline-volume --id P1-V2`,
		`novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`,
		`novelgen tool check all --target outline --scope chapter --id "P1-V2-C1" --category logic --min-priority low --max-issues 8`,
		`novelgen tool query context --type outline-repair --id "P1-V2-C1" --name "logic" --view index`,
		`novelgen tool query context --type outline-repair --id "P1-V2-C1" --name "logic" --view brief`,
		`novelgen tool patch outline --target volume --id "P1-V2"`,
	} {
		if !containsExactString(got, want) {
			t.Fatalf("allowlist missing %q: %#v", want, got)
		}
	}
}

func TestComposeImproveToolAllowlistAllowsVolumeBriefForCleanAndFocusedTasks(t *testing.T) {
	volume := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{{ID: "P1-V2-C1"}}}
	base := composeAgentSDKImproveVolumePromptInput{
		TargetVolumeID: "P1-V2",
		RequiredQueries: []string{
			"novelgen tool query context --type outline-volume --id P1-V2 --view index",
		},
	}

	cleanProbe := composeImproveToolAllowlist(volume, base, true)
	if containsExactString(cleanProbe, `novelgen tool query context --type outline-volume --id P1-V2`) {
		t.Fatalf("clean probe should not allow broad same-volume brief/full query: %#v", cleanProbe)
	}
	if !containsExactString(cleanProbe, `novelgen tool query context --type outline-volume --id P1-V2 --view brief`) {
		t.Fatalf("clean probe should allow exact same-volume brief query: %#v", cleanProbe)
	}

	withPrompt := base
	withPrompt.UserPrompt = "加强第二卷的敌方压迫感"
	focused := composeImproveToolAllowlist(volume, withPrompt, true)
	if !containsExactString(focused, `novelgen tool query context --type outline-volume --id P1-V2`) {
		t.Fatalf("focused task should allow same-volume detail query: %#v", focused)
	}
	if !containsExactString(focused, `novelgen tool query outline --type volume --id "P1-V2"`) {
		t.Fatalf("focused task should allow same-volume outline volume query: %#v", focused)
	}
	if !containsExactString(focused, `novelgen tool query outline --type events --volume-id "P1-V2" --view brief`) {
		t.Fatalf("focused task should allow same-volume events query: %#v", focused)
	}
	if !containsExactString(focused, `novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`) {
		t.Fatalf("focused task should allow target-volume chapter brief query: %#v", focused)
	}
	if !containsExactString(focused, `novelgen tool query outline --type events --chapter-id "P1-V2-C1" --view brief`) {
		t.Fatalf("focused task should allow target-volume chapter events query: %#v", focused)
	}

	cleanPrompt := base
	cleanPrompt.UserPrompt = scopeComposeAgentSDKVolumeUserPrompt("如果 check 返回 0 issue，不要为了润色而改写", "P1-V2", "Target")
	cleanPrompt.ForceIssueRepair = true
	cleanScoped := composeImproveToolAllowlist(volume, cleanPrompt, true)
	if containsExactString(cleanScoped, `novelgen tool query context --type outline-volume --id P1-V2`) ||
		containsExactString(cleanScoped, `novelgen tool query outline --type volume --id "P1-V2"`) ||
		containsExactString(cleanScoped, `novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`) {
		t.Fatalf("clean-probe prompt should not allow exploratory detail queries: %#v", cleanScoped)
	}
	if !containsExactString(cleanScoped, `novelgen tool query context --type outline-volume --id P1-V2 --view brief`) {
		t.Fatalf("clean-probe prompt should allow exact same-volume brief query: %#v", cleanScoped)
	}
	if !containsExactString(cleanScoped, `novelgen tool check all --target outline --scope volume --id "P1-V2"`) {
		t.Fatalf("clean-probe prompt should still allow target volume check: %#v", cleanScoped)
	}
}

func TestComposeImproveToolAllowlistIncludesAdjacentChaptersForFocusedSuggestion(t *testing.T) {
	volume := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{
		{ID: "P1-V2-C1"},
		{ID: "P1-V2-C2"},
		{ID: "P1-V2-C3"},
		{ID: "P1-V2-C4"},
	}}
	promptInput := composeAgentSDKImproveVolumePromptInput{
		TargetVolumeID: "P1-V2",
		RequiredQueries: []string{
			"novelgen tool query context --type outline-volume --id P1-V2 --view index",
		},
		ReviewResult: composeAgentSDKPromptReviewResult{Suggestions: []composeAgentSDKPromptReviewSuggestion{{
			TargetID: "P1-V2-C2",
		}}},
	}

	got := composeImproveToolAllowlist(volume, promptInput, true)
	for _, want := range []string{
		`novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V2-C2" --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V2-C3" --view brief`,
	} {
		if !containsExactString(got, want) {
			t.Fatalf("adjacent allowlist missing %q: %#v", want, got)
		}
	}
	if containsExactString(got, `novelgen tool query outline --type chapter --id "P1-V2-C4" --view brief`) {
		t.Fatalf("adjacent allowlist should not include unrelated chapter detail: %#v", got)
	}
}

func TestComposeImproveToolAllowlistAllowsWholeVolumeDetailsForVolumeSuggestion(t *testing.T) {
	volume := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{
		{ID: "P1-V2-C1"},
		{ID: "P1-V2-C2"},
	}}
	promptInput := composeAgentSDKImproveVolumePromptInput{
		TargetVolumeID: "P1-V2",
		RequiredQueries: []string{
			"novelgen tool query context --type outline-volume --id P1-V2 --view index",
		},
		ReviewResult: composeAgentSDKPromptReviewResult{Suggestions: []composeAgentSDKPromptReviewSuggestion{{
			TargetID: "P1-V2",
			Category: "redundancy",
		}}},
	}

	got := composeImproveToolAllowlist(volume, promptInput, true)
	for _, want := range []string{
		`novelgen tool query outline --type events --volume-id "P1-V2" --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V2-C2" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C2" --view brief`,
	} {
		if !containsExactString(got, want) {
			t.Fatalf("volume-level allowlist missing %q: %#v", want, got)
		}
	}
}

func TestComposeImproveToolAllowlistNarrowsBoundaryPromptDetails(t *testing.T) {
	volume := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{
		{ID: "P1-V2-C1"},
		{ID: "P1-V2-C2"},
		{ID: "P1-V2-C3"},
	}}
	promptInput := composeAgentSDKImproveVolumePromptInput{
		TargetVolumeID: "P1-V2",
		UserPrompt:     "强化首章主动目标和卷尾悬念",
		RequiredQueries: []string{
			"novelgen tool query context --type outline-volume --id P1-V2 --view index",
		},
	}

	got := composeImproveToolAllowlist(volume, promptInput, true)
	for _, want := range []string{
		`novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V2-C3" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C3" --view brief`,
		`novelgen tool query outline --type refs --entity-type character --name`,
		`novelgen tool query outline --type refs --entity-type item --name`,
		`novelgen tool query outline --type refs --entity-type location --name`,
		`novelgen tool query story-setup --type search`,
		`novelgen tool query story-setup --type core-cast --name`,
		`novelgen tool query story-setup --type storyline --name`,
		`novelgen tool query story-setup --type premise --name`,
		`novelgen tool query story-setup --type resource --name`,
		`novelgen tool query story-setup --type timeline --name`,
	} {
		if !containsExactString(got, want) {
			t.Fatalf("boundary allowlist missing %q: %#v", want, got)
		}
	}
	for _, unwanted := range []string{
		`novelgen tool query outline --type chapter --id "P1-V2-C2" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C2" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C2" --fields result,details,target,target_type,actor,action --view brief`,
	} {
		if containsExactString(got, unwanted) {
			t.Fatalf("boundary allowlist should not include middle chapter detail %q: %#v", unwanted, got)
		}
	}
	if !containsExactString(got, `novelgen tool check all --target outline --scope chapter --id "P1-V2-C2"`) {
		t.Fatalf("chapter checks should remain available for explicit check routing: %#v", got)
	}
}

func TestComposeImproveToolAllowlistKeepsAllDetailsForWholeVolumePrompt(t *testing.T) {
	volume := models.Volume{ID: "P1-V2", Chapters: []models.Chapter{
		{ID: "P1-V2-C1"},
		{ID: "P1-V2-C2"},
	}}
	promptInput := composeAgentSDKImproveVolumePromptInput{
		TargetVolumeID: "P1-V2",
		UserPrompt:     "逐章强化全卷的敌方压迫感",
		RequiredQueries: []string{
			"novelgen tool query context --type outline-volume --id P1-V2 --view index",
		},
	}

	got := composeImproveToolAllowlist(volume, promptInput, true)
	for _, want := range []string{
		`novelgen tool query outline --type chapter --id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type chapter --id "P1-V2-C2" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C1" --view brief`,
		`novelgen tool query outline --type events --chapter-id "P1-V2-C2" --view brief`,
	} {
		if !containsExactString(got, want) {
			t.Fatalf("whole-volume prompt should keep all chapter detail %q: %#v", want, got)
		}
	}
}

func TestBuildAgentSDKImproveVolumePromptInputMentionsBoundaryScope(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part: models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V2", Chapters: []models.Chapter{
			{ID: "P1-V2-C1"},
			{ID: "P1-V2-C2"},
			{ID: "P1-V2-C3"},
		}},
		UserPrompt: "强化首章主动目标和卷尾悬念",
	}

	got := buildAgentSDKImproveVolumePromptInput(input, true, false)
	text := strings.Join(got.Instructions, "\n")
	if !strings.Contains(text, "boundary-scoped") ||
		!strings.Contains(text, "P1-V2-C1, P1-V2-C3") ||
		strings.Contains(text, "P1-V2-C2") {
		t.Fatalf("boundary instruction should name only first/last chapters: %s", text)
	}
}

func TestBuildAgentSDKImproveVolumePromptInputRequiresFocusedChecks(t *testing.T) {
	input := ComposeImproveVolumeInput{
		Part: models.Part{ID: "P1"},
		Volume: models.Volume{ID: "P1-V1", Chapters: []models.Chapter{
			{ID: "P1-V1-C1"},
			{ID: "P1-V1-C2"},
		}},
		UserPrompt: "只验证 P1-V1-C2 的 transition/logic focused check",
	}

	got := buildAgentSDKImproveVolumePromptInput(input, true, true)
	text := strings.Join(got.Instructions, "\n")
	for _, want := range []string{
		"Required focused checks before final JSON",
		`novelgen tool check all --target outline --scope chapter --id "P1-V1-C2" --category logic --min-priority low --max-issues 8`,
		`novelgen tool check all --target outline --scope chapter --id "P1-V1-C2" --category transition --min-priority low --max-issues 8`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("required focused check instruction missing %q:\n%s", want, text)
		}
	}
	evidence := composeImproveToolEvidence(got, true)
	if len(evidence.RequiredToolCommands) != 2 {
		t.Fatalf("required tool commands = %#v", evidence.RequiredToolCommands)
	}
	if !containsExactString(evidence.RequiredToolCommands, `novelgen tool check all --target outline --scope chapter --id "P1-V1-C2" --category logic --min-priority low --max-issues 8`) ||
		!containsExactString(evidence.RequiredToolCommands, `novelgen tool check all --target outline --scope chapter --id "P1-V1-C2" --category transition --min-priority low --max-issues 8`) {
		t.Fatalf("required tool commands missing focused checks: %#v", evidence.RequiredToolCommands)
	}
}

func TestComposeImproveBoundaryChapterIDsFindsExplicitAndOrdinalTargets(t *testing.T) {
	volume := models.Volume{ID: "P1-V1", Chapters: []models.Chapter{
		{ID: "P1-V1-C1"},
		{ID: "P1-V1-C2"},
		{ID: "P1-V1-C3"},
	}}

	byID := composeImproveBoundaryChapterIDs(volume, "只修复 P1-V1-C2 的 transition/logic focused check")
	if len(byID) != 1 || byID[0] != "P1-V1-C2" {
		t.Fatalf("explicit ID boundary = %#v", byID)
	}

	byArabicOrdinal := composeImproveBoundaryChapterIDs(volume, "只修复第2章的开场过渡")
	if len(byArabicOrdinal) != 1 || byArabicOrdinal[0] != "P1-V1-C2" {
		t.Fatalf("arabic ordinal boundary = %#v", byArabicOrdinal)
	}

	byChineseOrdinal := composeImproveBoundaryChapterIDs(volume, "只修复第二章的开场过渡")
	if len(byChineseOrdinal) != 1 || byChineseOrdinal[0] != "P1-V1-C2" {
		t.Fatalf("chinese ordinal boundary = %#v", byChineseOrdinal)
	}
}

func TestComposeImproveBoundaryChapterIDsWholeVolumeEnumerationHasNoBoundary(t *testing.T) {
	volume := models.Volume{ID: "P1-V1", Chapters: []models.Chapter{
		{ID: "P1-V1-C1"},
		{ID: "P1-V1-C2"},
		{ID: "P1-V1-C3"},
	}}
	prompt := "第1章无改动；第2章修复过渡；第3章修逻辑；第4章检查节奏；覆盖第1到第3章"
	if ids := composeImproveBoundaryChapterIDs(volume, prompt); ids != nil {
		t.Fatalf("whole-volume enumeration should have no boundary restriction: %#v", ids)
	}
	if checks := composeImproveRequiredFocusedChecks(volume, prompt); len(checks) != 0 {
		t.Fatalf("whole-volume enumeration should not force per-chapter checks: %#v", checks)
	}
}

func TestComposeAgentSDKPatchInstructionPrefersStdinPipeForChineseJSON(t *testing.T) {
	instruction := composeAgentSDKPatchInstruction(true)
	for _, want := range []string{
		`printf '%s' '<compact-json>' | novelgen tool patch outline --target volume --id <target_volume_id>`,
		"do not use --patch-json",
		"do not run Python/Node/PowerShell/help commands",
		"Use --patch-json only for small ASCII-only patches",
		"Apply at most one successful outline volume patch",
	} {
		if !strings.Contains(instruction, want) {
			t.Fatalf("patch instruction missing %q: %s", want, instruction)
		}
	}

	checkInstruction := composeAgentSDKCheckInstruction(true)
	if !strings.Contains(checkInstruction, "return final JSON") ||
		!strings.Contains(checkInstruction, "Do not start another dry-run/apply cycle") {
		t.Fatalf("check instruction should force one incremental patch cycle: %s", checkInstruction)
	}
}

func TestValidateAgentSDKApplyOutputRejectsMultipleApplyPatches(t *testing.T) {
	output := composeAgentSDKApplyOutput{
		ReviewResult:      composeAgentSDKReviewResult{OverallScore: 60, Summary: "patched"},
		AppliedPatches:    true,
		AppliedPatchCount: 2,
	}
	if err := validateAgentSDKApplyOutput(output); err == nil || !strings.Contains(err.Error(), "at most 1") {
		t.Fatalf("expected multiple apply patches to be rejected, got %v", err)
	}
}

func TestBuildComposeAgentSDKGlobalNavigationRoutesFactionTierToSetupPatch(t *testing.T) {
	nav := buildComposeAgentSDKSuggestionNavigation("", models.ReviewSuggestion{
		Category: "faction_tier",
		TargetID: "zerg",
		Issue:    "missing faction tier ladder",
		Priority: models.PriorityLow,
	})
	if nav == nil {
		t.Fatalf("navigation missing")
	}
	if nav.TargetKind != "setup.faction_tier" {
		t.Fatalf("target kind = %q", nav.TargetKind)
	}
	if nav.PatchQuery != "novelgen tool patch setup" || nav.PatchShape == nil {
		t.Fatalf("expected setup patch route, got %#v", nav)
	}
	premises, ok := nav.PatchShape["premises"].([]map[string]string)
	if !ok || len(premises) != 1 || premises[0]["category"] != "zerg" {
		t.Fatalf("patch shape should name the target faction category: %#v", nav.PatchShape)
	}
	if !containsExactString(nav.DetailQueries, `novelgen tool query story-setup --type search --name "zerg" --view index`) ||
		!containsExactString(nav.DetailQueries, `novelgen tool query story-setup --type search --name "zerg" --view brief`) {
		t.Fatalf("expected focused story-setup queries, got %#v", nav.DetailQueries)
	}
	if strings.Contains(nav.RepairContextQuery, "outline --view") {
		t.Fatalf("global setup repair should not route to full outline: %#v", nav)
	}
}

func TestComposeGlobalRepairToolAllowlistRestrictsToGlobalRepairSetupAndVolumePatch(t *testing.T) {
	dryRun := composeGlobalRepairToolAllowlist(false)
	for _, want := range []string{
		"novelgen tool query context --type outline-global-repair",
		"novelgen tool query story-setup --type search",
		"novelgen tool check all --target outline",
		"novelgen tool check all --target setup",
		"novelgen tool patch setup",
		"novelgen tool patch outline --target volume --id",
	} {
		if !containsExactString(dryRun, want) {
			t.Fatalf("dry-run allowlist missing %q: %#v", want, dryRun)
		}
	}
	if containsExactString(dryRun, "novelgen tool patch setup --apply") ||
		containsExactString(dryRun, "novelgen tool query outline") ||
		containsExactString(dryRun, "novelgen tool patch outline") ||
		containsExactString(dryRun, "novelgen tool patch outline --target chapter") {
		t.Fatalf("dry-run allowlist is too broad: %#v", dryRun)
	}

	apply := composeGlobalRepairToolAllowlist(true)
	if !containsExactString(apply, "novelgen tool patch setup --apply") ||
		!containsExactString(apply, "novelgen tool patch outline --target volume --id --apply") {
		t.Fatalf("apply allowlist missing setup apply: %#v", apply)
	}
}

func TestRepairGlobalIssuesWithAgentSDKUsesGlobalWorkflowAndEvidence(t *testing.T) {
	output := `{"review_result":{"overall_score":96,"summary":"patched setup faction tier","suggestions":[]},"applied_patches":true,"applied_patch_count":1,"final_check":"clean"}`
	runtime := &fakeComposeImproveRuntime{result: &agentruntime.Result{
		Content: output,
		Usage:   agentruntime.Usage{TotalTokens: 10},
		LiveSummary: &agentruntime.LiveSummary{
			ContextQueryCalls: 1,
			CheckCalls:        1,
			PatchApplies:      1,
			AllowedToolCommands: []string{
				"novelgen tool check all --target setup --category faction_tier --min-priority low --max-issues 12",
			},
		},
	}}
	agent := &ComposeAgent{base: &BaseAgent{
		name:     "ComposeAgent",
		runtime:  runtime,
		config:   &llm.Config{},
		language: "zh",
	}}
	review := models.ReviewResult{Suggestions: []models.ReviewSuggestion{{
		Category: "faction_tier",
		TargetID: "zerg",
		Issue:    "missing faction tier ladder",
		Priority: models.PriorityLow,
	}}}

	got, err := agent.RepairGlobalIssuesWithAgentSDK(context.Background(), review, true)
	if err != nil {
		t.Fatalf("global repair should pass with live evidence: %v", err)
	}
	if got.Summary != "patched setup faction tier" {
		t.Fatalf("summary = %q", got.Summary)
	}
	if !containsExactString(runtime.invocation.SDKSkills, "outline-global-repair-workflow") ||
		!containsExactString(runtime.invocation.ToolAllowlist, "novelgen tool query context --type outline-global-repair") ||
		!containsExactString(runtime.invocation.ToolAllowlist, "novelgen tool check all --target setup --category faction_tier --min-priority low --max-issues 12") ||
		!containsExactString(runtime.invocation.ToolAllowlist, "novelgen tool patch setup --apply") ||
		!containsExactString(runtime.invocation.ToolAllowlist, "novelgen tool patch outline --target volume --id --apply") {
		t.Fatalf("runtime invocation missing global repair constraints: %#v", runtime.invocation)
	}
	if !strings.Contains(runtime.invocation.UserPrompt, "printf '%s' '<compact-json>' | <patch_query>") ||
		!strings.Contains(runtime.invocation.UserPrompt, "If patch_task is present") ||
		!strings.Contains(runtime.invocation.UserPrompt, "use patch_task.dry_run_command and patch_task.apply_command exactly") ||
		!strings.Contains(runtime.invocation.UserPrompt, "run dry_run_command once") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Do not run repair_context_query, outline-volume, outline-repair") ||
		!strings.Contains(runtime.invocation.UserPrompt, "do not use --patch-json") ||
		!strings.Contains(runtime.invocation.UserPrompt, "do not run placeholder text such as `<json>`") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Do not invent combined category names") ||
		!strings.Contains(runtime.invocation.UserPrompt, "provided post_patch_check_query exactly") ||
		!strings.Contains(runtime.invocation.UserPrompt, "Required post-patch checks before final JSON") ||
		!strings.Contains(runtime.invocation.UserPrompt, "do not run Python/Node/PowerShell/help commands") {
		t.Fatalf("global repair prompt should prefer stdin-piped patch JSON:\n%s", runtime.invocation.UserPrompt)
	}
	runtime.result.LiveSummary.ContextQueryCalls = 3
	recovered, err := agent.RepairGlobalIssuesWithAgentSDK(context.Background(), review, true)
	if err != nil {
		t.Fatalf("expected applied global patch recovery after context cap diagnostic, got %v", err)
	}
	if !strings.Contains(recovered.Summary, "recovered saved project state") {
		t.Fatalf("recovery summary = %q", recovered.Summary)
	}
	runtime.result.LiveSummary.PatchApplies = 0
	if _, err := agent.RepairGlobalIssuesWithAgentSDK(context.Background(), review, true); err == nil || !strings.Contains(err.Error(), "context query calls=3") {
		t.Fatalf("expected context query cap error without applied patch, got %v", err)
	}
	runtime.result.LiveSummary.ContextQueryCalls = 1
	runtime.result.LiveSummary.PatchApplies = 1

	runtime.result.LiveSummary.CheckCalls = 0
	if _, err := agent.RepairGlobalIssuesWithAgentSDK(context.Background(), review, true); err == nil || !strings.Contains(err.Error(), "check calls=0") {
		t.Fatalf("expected missing check evidence error, got %v", err)
	}
	runtime.result.LiveSummary.CheckCalls = 1
	runtime.result.LiveSummary.ApplyWithoutFollowupCheck = 1
	if _, err := agent.RepairGlobalIssuesWithAgentSDK(context.Background(), review, true); err == nil || !strings.Contains(err.Error(), "without follow-up check") {
		t.Fatalf("expected missing follow-up check evidence error, got %v", err)
	}
	runtime.result.LiveSummary.ApplyWithoutFollowupCheck = 0
	runtime.result.LiveSummary.AllowedToolCommands = []string{
		`novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority medium --max-issues 12`,
	}
	if _, err := agent.RepairGlobalIssuesWithAgentSDK(context.Background(), review, true); err == nil || !strings.Contains(err.Error(), "required tool command not observed") {
		t.Fatalf("expected exact post-patch check evidence error, got %v", err)
	}
}

func TestValidateComposeImproveAgentSDKPatchApplyEvidenceRequiresPatchApplyEvidence(t *testing.T) {
	noApply := &agentruntime.Result{LiveSummary: &agentruntime.LiveSummary{
		ContextQueryCalls: 1,
		CheckCalls:        1,
	}}
	if err := validateComposeImproveAgentSDKPatchApplyEvidence(noApply, true, 1); err == nil || !strings.Contains(err.Error(), "no patch apply") {
		t.Fatalf("expected missing apply error, got %v", err)
	}

	fewerApplies := &agentruntime.Result{LiveSummary: &agentruntime.LiveSummary{
		ContextQueryCalls: 1,
		CheckCalls:        1,
		PatchApplies:      1,
	}}
	if err := validateComposeImproveAgentSDKPatchApplyEvidence(fewerApplies, true, 2); err == nil || !strings.Contains(err.Error(), "reported 2 applied") {
		t.Fatalf("expected apply count mismatch, got %v", err)
	}
}

func TestImproveVolumeWithAgentSDKApplyFalseDoesNotReloadDiskOutline(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "story", "compose"), 0o755); err != nil {
		t.Fatalf("mkdir compose: %v", err)
	}
	diskOutline := models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID:      "P1-V1",
			Title:   "Disk",
			Summary: "disk-only change",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "Disk chapter",
				Summary: "disk chapter",
				Events:  []models.Event{{Actor: "Lin", Action: models.ActionDiscover, Target: "Signal", TargetType: models.TargetTypeKnowledge}},
				Scenes:  []models.OutlineScene{{Order: 1, POV: "Lin", Goal: "Check", Beats: []string{"Check signal"}}},
			}},
		}},
	}}}
	if err := diskOutline.Save(filepath.Join(root, "story", "compose", "outline.json")); err != nil {
		t.Fatalf("save disk outline: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	output := `{"review_result":{"overall_score":100,"summary":"medium check clean","suggestions":[]},"applied_patches":false,"applied_patch_count":0,"final_check":"clean"}`
	runtime := &fakeComposeImproveRuntime{result: &agentruntime.Result{
		Content: output,
		Usage:   agentruntime.Usage{TotalTokens: 10},
		LiveSummary: &agentruntime.LiveSummary{
			ContextQueryCalls: 1,
			CheckCalls:        1,
		},
	}}
	agent := &ComposeAgent{base: &BaseAgent{name: "ComposeAgent", runtime: runtime, config: &llm.Config{}, language: "zh"}}
	input := ComposeImproveVolumeInput{
		Part: models.Part{ID: "P1", Title: "Part", Summary: "Part"},
		Volume: models.Volume{
			ID:      "P1-V1",
			Title:   "Input",
			Summary: "input summary",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "Input chapter",
				Summary: "input chapter",
				Events:  []models.Event{{Actor: "Lin", Action: models.ActionDiscover, Target: "Signal", TargetType: models.TargetTypeKnowledge}},
				Scenes:  []models.OutlineScene{{Order: 1, POV: "Lin", Goal: "Check", Beats: []string{"Check signal"}}},
			}},
		},
	}

	got, err := agent.ImproveVolumeWithAgentSDK(context.Background(), input, true, false)
	if err != nil {
		t.Fatalf("apply-mode clean check should pass: %v", err)
	}
	if got.Volume.Title != "Input" || got.Volume.Summary != "input summary" {
		t.Fatalf("agent apply=false should not reload disk-only outline changes: %+v", got.Volume)
	}
	if !containsExactString(runtime.invocation.SDKSkills, "outline-improve-volume-workflow") {
		t.Fatalf("runtime invocation missing workflow skill: %#v", runtime.invocation)
	}
}

func TestImproveVolumeWithAgentSDKRecoversAppliedPatchAfterMaxTurns(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "story", "compose"), 0o755); err != nil {
		t.Fatalf("mkdir compose: %v", err)
	}
	diskOutline := models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID:      "P1-V1",
			Title:   "Recovered",
			Summary: "agent-applied summary",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "Recovered chapter",
				Summary: "agent-applied chapter",
				Events:  []models.Event{{Actor: "林野", Action: models.ActionDiscover, Target: "信号", TargetType: models.TargetTypeKnowledge}},
				Scenes:  []models.OutlineScene{{Order: 1, POV: "林野", Goal: "确认信号", Beats: []string{"林野确认信号来自废墟深处。"}}},
			}},
		}},
	}}}
	if err := diskOutline.Save(filepath.Join(root, "story", "compose", "outline.json")); err != nil {
		t.Fatalf("save disk outline: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	runtime := &fakeComposeImproveRuntime{
		err: errors.New("Reached maximum number of turns (28)"),
		result: &agentruntime.Result{LiveSummary: &agentruntime.LiveSummary{
			ContextQueryCalls:         1,
			CheckCalls:                1,
			PatchApplies:              1,
			ApplyWithoutFollowupCheck: 0,
		}},
	}
	agent := &ComposeAgent{base: &BaseAgent{name: "ComposeAgent", runtime: runtime, config: &llm.Config{}, language: "zh"}}
	input := ComposeImproveVolumeInput{
		Part: models.Part{ID: "P1", Title: "第一部", Summary: "测试"},
		Volume: models.Volume{
			ID:      "P1-V1",
			Title:   "Input",
			Summary: "input summary",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "Input chapter",
				Summary: "input chapter",
				Events:  []models.Event{{Actor: "林野", Action: models.ActionDiscover, Target: "信号", TargetType: models.TargetTypeKnowledge}},
				Scenes:  []models.OutlineScene{{Order: 1, POV: "林野", Goal: "确认信号", Beats: []string{"林野确认信号来自废墟深处。"}}},
			}},
		},
	}

	got, err := agent.ImproveVolumeWithAgentSDK(context.Background(), input, true, false)
	if err != nil {
		t.Fatalf("expected applied patch recovery, got %v", err)
	}
	if !strings.Contains(got.Volume.Title, "Recovered") || got.Volume.Summary != "agent-applied summary" {
		t.Fatalf("did not recover disk-applied volume: %+v", got.Volume)
	}
	if got.ReviewResult.OverallScore != 80 || !strings.Contains(got.ReviewResult.Summary, "recovered saved outline") {
		t.Fatalf("recovery review result = %+v", got.ReviewResult)
	}
}

func TestValidateAgentSDKApplyOutputRejectsPatchCountWithoutAppliedPatches(t *testing.T) {
	err := validateAgentSDKApplyOutput(composeAgentSDKApplyOutput{
		ReviewResult:      composeAgentSDKReviewResult{OverallScore: 100, Summary: "clean"},
		AppliedPatches:    false,
		AppliedPatchCount: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "applied_patches=false") {
		t.Fatalf("expected inconsistent patch count error, got %v", err)
	}
}

func TestImproveVolumeWithAgentSDKRequiresLiveCheckEvidence(t *testing.T) {
	output := `{"review_result":{"overall_score":100,"summary":"检查干净","suggestions":[]},"volume_patch":{"id":"P1-V1","changed_chapters":[]}}`
	runtime := &fakeComposeImproveRuntime{result: &agentruntime.Result{
		Content: output,
		Usage:   agentruntime.Usage{TotalTokens: 10},
		LiveSummary: &agentruntime.LiveSummary{
			ContextQueryCalls: 1,
			CheckCalls:        1,
		},
	}}
	agent := &ComposeAgent{base: &BaseAgent{
		name:     "ComposeAgent",
		runtime:  runtime,
		config:   &llm.Config{},
		language: "zh",
	}}
	input := ComposeImproveVolumeInput{
		Part: models.Part{ID: "P1", Title: "第一部", Summary: "测试"},
		Volume: models.Volume{
			ID:      "P1-V1",
			Title:   "第一卷",
			Summary: "测试卷",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "第一章",
				Summary: "测试章",
				Events: []models.Event{{
					Actor:      "林野",
					Action:     models.ActionDiscover,
					Target:     "信号",
					TargetType: models.TargetTypeKnowledge,
				}},
				Scenes: []models.OutlineScene{{
					Order: 1,
					POV:   "林野",
					Goal:  "确认信号",
					Beats: []string{"林野确认信号来自废墟深处。"},
				}},
			}},
		},
	}

	if _, err := agent.ImproveVolumeWithAgentSDK(context.Background(), input, false, false); err != nil {
		t.Fatalf("expected live check evidence to pass: %v", err)
	}
	if runtime.invocation.Command == "" || runtime.invocation.OutputJSONSchema == nil {
		t.Fatalf("runtime invocation was not populated: %#v", runtime.invocation)
	}

	runtime.result.LiveSummary.CheckCalls = 0
	if _, err := agent.ImproveVolumeWithAgentSDK(context.Background(), input, false, false); err == nil || !strings.Contains(err.Error(), "check calls=0") {
		t.Fatalf("expected missing check evidence error, got %v", err)
	}
}

func TestAppealAndPayoffBriefsSkipEmptyAndKeepUpgradePath(t *testing.T) {
	appeal := formatAppealEngineBrief("Appeal", &models.AppealEngine{
		Appeal:      "wins through timing",
		UpgradePath: "predicts longer chains",
	})
	if !strings.Contains(appeal, "upgrade_path=predicts longer chains") {
		t.Fatalf("appeal brief missing upgrade path: %q", appeal)
	}

	emptyVolume := formatVolumeBrief(models.Volume{
		ID:             "P1-V1",
		Title:          "Empty",
		Summary:        "No payoff yet",
		PayoffContract: &models.VolumePayoffContract{},
	})
	if strings.Contains(emptyVolume, "Payoff:") {
		t.Fatalf("empty payoff contract should not be formatted:\n%s", emptyVolume)
	}

	emptyChapter := formatChapterBrief(models.Chapter{
		ID:            "P1-V1-C1",
		Title:         "Empty",
		Summary:       "No chapter payoff yet",
		ChapterPayoff: &models.ChapterPayoff{},
	})
	if strings.Contains(emptyChapter, "payoff:") {
		t.Fatalf("empty chapter payoff should not be formatted:\n%s", emptyChapter)
	}

	partialChapter := formatChapterBrief(models.Chapter{
		ID:      "P1-V1-C2",
		Title:   "Partial",
		Summary: "Only a payoff moment is known",
		ChapterPayoff: &models.ChapterPayoff{
			PayoffMoment: "The guard salutes an empty corner.",
		},
	})
	if strings.Contains(partialChapter, "->") || !strings.Contains(partialChapter, "moment=The guard salutes an empty corner.") {
		t.Fatalf("partial chapter payoff should format only populated fields:\n%s", partialChapter)
	}
}

func TestBuildSetupBriefCapsLargeSetupSections(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Large",
		Premise:     strings.Repeat("p", 800),
		Theme:       "growth",
	}
	for i := 0; i < 14; i++ {
		setup.CoreCast = append(setup.CoreCast, models.CoreCastSeed{
			Name:          "Cast",
			Role:          "ally",
			Importance:    5,
			StoryFunction: strings.Repeat("function", 40),
			EntryPhase:    "series",
		})
		setup.Storylines = append(setup.Storylines, models.Storyline{
			Name:        "Arc",
			Type:        "subplot",
			Importance:  5,
			Description: strings.Repeat("description", 40),
		})
		setup.WorldResources = append(setup.WorldResources, models.WorldResource{
			Name:        "Resource",
			Category:    "currency",
			Scarcity:    "rare",
			Description: strings.Repeat("resource", 40),
		})
	}
	for i := 0; i < 10; i++ {
		premise := models.Premise{
			Name:        "System",
			Category:    "progression",
			Description: strings.Repeat("system", 80),
		}
		for level := 1; level <= 10; level++ {
			premise.Progression = append(premise.Progression, models.ProgressionStage{
				Level:       level,
				Name:        "Tier",
				Description: strings.Repeat("stage", 50),
			})
		}
		setup.Premises = append(setup.Premises, premise)
	}

	brief := (&ComposeAgent{}).buildSetupBrief(setup)

	for _, want := range []string{
		"... 2 more cast seed(s)",
		"... 2 more storyline(s)",
		"... 2 more progression system(s)",
		"... 2 more progression stage(s)",
		"... 2 more resource(s)",
		"...",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("setup brief missing %q:\n%s", want, brief)
		}
	}
	if strings.Contains(brief, strings.Repeat("function", 40)) {
		t.Fatalf("setup brief should clip oversized cast functions:\n%s", brief)
	}
}
