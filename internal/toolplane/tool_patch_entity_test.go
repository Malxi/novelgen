package toolplane_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	models "novelgen/internal/contract/story"
	"novelgen/internal/contract/testfixtures"
	"novelgen/internal/fiction/domain/logic/continuity/recap"
	"novelgen/internal/platform/store"
	"novelgen/internal/toolplane"

	"github.com/spf13/cobra"
)

func TestApplyChapterPatchPreservesIDAndOverlaysFields(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := []byte(`{"id":"P1-V1-C1","summary":"new summary","characters":["Lin","Mira"]}`)

	changes, err := toolplane.ApplyChapterPatch(outline, "P1-V1-C1", patch)
	if err != nil {
		t.Fatal(err)
	}
	chapter := outline.GetChapterByID("P1-V1-C1")
	if chapter.ID != "P1-V1-C1" {
		t.Fatalf("chapter id changed to %q", chapter.ID)
	}
	if chapter.Summary != "new summary" || len(chapter.Characters) != 2 {
		t.Fatalf("patch not applied: %#v", chapter)
	}
	if !hasPatchChange(changes, "outline.chapter.P1-V1-C1.summary") {
		t.Fatalf("summary change not reported: %#v", changes)
	}
}

func TestApplyChapterPatchRejectsIDMismatch(t *testing.T) {
	outline := validToolPatchTestOutline()
	_, err := toolplane.ApplyChapterPatch(outline, "P1-V1-C1", []byte(`{"id":"P1-V1-C2","summary":"bad"}`))
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected id mismatch error, got %v", err)
	}
}

func TestApplyChapterPatchRejectsGarbledPatchText(t *testing.T) {
	err := toolplane.ValidateToolPatchBytes([]byte("{\"summary\":\"\ufffd\ufffd\ufffd????\"}"))
	if err == nil || !strings.Contains(err.Error(), "suspicious") {
		t.Fatalf("expected garbled text rejection, got %v", err)
	}
}

func TestValidateToolChapterPatchLengthRejectsHardOvershoot(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	okContent := "# Opening\n\n" + strings.Repeat("字", 3600)
	if err := toolplane.ValidateToolChapterPatchLength(chapter, okContent, 3000); err != nil {
		t.Fatalf("expected hard max boundary to pass, got %v", err)
	}
	tooLong := "# Opening\n\n" + strings.Repeat("字", 3601)
	err := toolplane.ValidateToolChapterPatchLength(chapter, tooLong, 3000)
	if err == nil || !strings.Contains(err.Error(), "too long") || !strings.Contains(err.Error(), "hard max 3600") {
		t.Fatalf("expected hard overshoot error, got %v", err)
	}
}

func TestRunToolChapterQualityGateFlagsTargetOvershoot(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}
	content := "# Opening\n\n" + strings.Repeat("字", 1501)

	gate := toolplane.RunToolChapterQualityGate(chapter, content, 1200)
	if len(gate.Suggestions) == 0 {
		t.Fatal("expected target overshoot suggestion")
	}
	found := false
	for _, suggestion := range gate.Suggestions {
		if suggestion.Category == "length" && strings.Contains(suggestion.Issue, "much longer than target") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected length overshoot suggestion, got %#v", gate.Suggestions)
	}
}

func TestValidateToolChapterPatchLengthRejectsOverShrink(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C2", Title: "出招"}
	previous := "# 出招\n\n" + strings.Repeat("旧", 1300)
	tooShort := "# 出招\n\n" + strings.Repeat("新", 700)

	err := toolplane.ValidateToolChapterPatchLength(chapter, tooShort, 1200, previous)
	if err == nil || !strings.Contains(err.Error(), "removes too much existing prose") || !strings.Contains(err.Error(), "minimum retained 1105") {
		t.Fatalf("expected over-shrink error, got %v", err)
	}

	ok := "# 出招\n\n" + strings.Repeat("新", 1110)
	if err := toolplane.ValidateToolChapterPatchLength(chapter, ok, 1200, previous); err != nil {
		t.Fatalf("expected minimal shrink to pass, got %v", err)
	}
}

func TestValidateToolChapterPatchLengthAllowsTrimmingOversizedPrevious(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C2", Title: "出招"}
	previous := "# 出招\n\n" + strings.Repeat("旧", 1900)
	trimmed := "# 出招\n\n" + strings.Repeat("新", 1200)

	if err := toolplane.ValidateToolChapterPatchLength(chapter, trimmed, 1200, previous); err != nil {
		t.Fatalf("expected oversized previous content to allow trimming, got %v", err)
	}
}

func TestValidateToolChapterPatchLengthRejectsParagraphCollapse(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C4", Title: "反杀"}
	previous := "# 反杀\n\n" + strings.Join([]string{
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
		strings.Repeat("旧", 120),
	}, "\n\n")
	collapsed := "# 反杀\n\n" + strings.Repeat("新", 1000)

	err := toolplane.ValidateToolChapterPatchLength(chapter, collapsed, 1200, previous)
	if err == nil || !strings.Contains(err.Error(), "collapses chapter") {
		t.Fatalf("expected paragraph collapse error, got %v", err)
	}

	readable := "# 反杀\n\n" + strings.Join([]string{
		strings.Repeat("新", 230),
		strings.Repeat("新", 230),
		strings.Repeat("新", 230),
		strings.Repeat("新", 230),
	}, "\n\n")
	if err := toolplane.ValidateToolChapterPatchLength(chapter, readable, 1200, previous); err != nil {
		t.Fatalf("expected readable paragraph patch to pass, got %v", err)
	}
}

func TestRunToolChapterQualityGateFlagsCollapsedParagraphs(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C4", Title: "反杀"}
	content := "# 反杀\n\n" + strings.Repeat("这一段连续推进剧情没有空行也没有段落拆分。", 80)

	gate := toolplane.RunToolChapterQualityGate(chapter, content, 1200)

	if !qualityGateHasIssue(gate, "structure", models.PriorityMedium) {
		t.Fatalf("expected collapsed paragraph issue, got %+v", gate.Suggestions)
	}
}

func TestApplyVolumePatchRejectsWholeChapterReplacement(t *testing.T) {
	outline := validToolPatchTestOutline()
	_, err := toolplane.ApplyVolumePatch(outline, "P1-V1", []byte(`{"chapters":[]}`))
	if err == nil || !strings.Contains(err.Error(), "changed_chapters") {
		t.Fatalf("expected changed_chapters error, got %v", err)
	}
}

func TestApplyVolumePatchChangedChapters(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := map[string]interface{}{
		"id":    "P1-V1",
		"title": "New Volume",
		"changed_chapters": []map[string]interface{}{{
			"id":       "P1-V1-C1",
			"conflict": "new conflict",
		}},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := toolplane.ApplyVolumePatch(outline, "P1-V1", data)
	if err != nil {
		t.Fatal(err)
	}
	if got := outline.GetVolumeByID("P1-V1").Title; got != "New Volume" {
		t.Fatalf("volume title = %q", got)
	}
	if got := outline.GetChapterByID("P1-V1-C1").Conflict; got != "new conflict" {
		t.Fatalf("chapter conflict = %q", got)
	}
	if !hasPatchChange(changes, "outline.volume.P1-V1.title") ||
		!hasPatchChange(changes, "outline.chapter.P1-V1-C1.conflict") {
		t.Fatalf("expected volume and chapter changes, got %#v", changes)
	}
}

func TestApplyVolumePatchMergesPayoffContractFields(t *testing.T) {
	outline := validToolPatchTestOutline()
	volume := outline.GetVolumeByID("P1-V1")
	volume.PayoffContract = &models.VolumePayoffContract{
		VolumeQuestion:      "Can Lin survive the first signal?",
		PowerPromise:        "Signal reading creates tactical advantage.",
		MainOpponentMisread: "The rival thinks Lin is blind.",
		VisibleReward:       "Lin gains a map.",
		ReputationShift:     "Lin becomes credible.",
		NextBiggerGame:      "The war expands.",
	}

	changes, err := toolplane.ApplyVolumePatch(outline, "P1-V1", []byte(`{"payoff_contract":{"big_win":"Lin wins the first public duel."}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := outline.GetVolumeByID("P1-V1").PayoffContract
	if got == nil {
		t.Fatal("payoff_contract was removed")
	}
	if got.BigWin != "Lin wins the first public duel." ||
		got.VolumeQuestion != "Can Lin survive the first signal?" ||
		got.PowerPromise != "Signal reading creates tactical advantage." ||
		got.MainOpponentMisread != "The rival thinks Lin is blind." ||
		got.VisibleReward != "Lin gains a map." ||
		got.ReputationShift != "Lin becomes credible." ||
		got.NextBiggerGame != "The war expands." {
		t.Fatalf("payoff_contract should be field-merged: %#v", got)
	}
	if !hasPatchChange(changes, "outline.volume.P1-V1.payoff_contract") {
		t.Fatalf("payoff_contract change not reported: %#v", changes)
	}
}

func TestApplyVolumePatchChangedEvents(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := []byte(`{"changed_events":[{"chapter_id":"P1-V1-C1","event_index":1,"type":"plan","action":"plan","details":"Lin chooses the safer route."}]}`)
	changes, err := toolplane.ApplyVolumePatch(outline, "P1-V1", patch)
	if err != nil {
		t.Fatal(err)
	}
	events := outline.GetChapterByID("P1-V1-C1").Events
	if len(events) != 3 {
		t.Fatalf("event array length changed: %#v", events)
	}
	if events[1].Type != "plan" || events[1].Action != "plan" || events[1].Details != "Lin chooses the safer route." {
		t.Fatalf("target event not patched: %#v", events[1])
	}
	if events[1].Target != "Camp" || events[0].Target != "Star Core" || events[2].Target != "Signal" {
		t.Fatalf("event patch should preserve target event fields and untouched events: %#v", events)
	}
	if !hasPatchChange(changes, "outline.event.P1-V1-C1.1.type") ||
		!hasPatchChange(changes, "outline.event.P1-V1-C1.1.action") {
		t.Fatalf("event changes not reported: %#v", changes)
	}
}

func TestApplyVolumePatchDeepMergesNestedChapterObjects(t *testing.T) {
	outline := validToolPatchTestOutline()
	chapter := outline.GetChapterByID("P1-V1-C1")
	chapter.ChapterPayoff = &models.ChapterPayoff{
		Desire:       "old desire",
		Pressure:     "old pressure",
		CleverMove:   "old clever move",
		PayoffMoment: "old payoff",
		Reward:       "old reward",
		SocialProof:  "old social proof",
		Hook:         "old hook",
	}

	patch := []byte(`{"changed_chapters":[{"id":"P1-V1-C1","chapter_payoff":{"desire":"new desire","hook":"new hook"}}]}`)
	changes, err := toolplane.ApplyVolumePatch(outline, "P1-V1", patch)
	if err != nil {
		t.Fatal(err)
	}
	got := outline.GetChapterByID("P1-V1-C1").ChapterPayoff
	if got == nil {
		t.Fatal("chapter payoff was removed")
	}
	if got.Desire != "new desire" || got.Hook != "new hook" {
		t.Fatalf("patch fields not applied: %#v", got)
	}
	if got.Pressure != "old pressure" ||
		got.CleverMove != "old clever move" ||
		got.PayoffMoment != "old payoff" ||
		got.Reward != "old reward" ||
		got.SocialProof != "old social proof" {
		t.Fatalf("nested payoff fields should be preserved by deep merge: %#v", got)
	}
	if !hasPatchChange(changes, "outline.chapter.P1-V1-C1.chapter_payoff") {
		t.Fatalf("chapter payoff change not reported: %#v", changes)
	}
}

func TestApplyChapterPatchStillReplacesArrays(t *testing.T) {
	outline := validToolPatchTestOutline()
	patch := []byte(`{"events":[{"actor":"Lin","action":"discover","target":"Door","target_type":"knowledge"}]}`)

	if _, err := toolplane.ApplyChapterPatch(outline, "P1-V1-C1", patch); err != nil {
		t.Fatal(err)
	}
	chapter := outline.GetChapterByID("P1-V1-C1")
	if len(chapter.Events) != 1 || chapter.Events[0].Target != "Door" {
		t.Fatalf("arrays should be replaced, not appended/deep-merged: %#v", chapter.Events)
	}
}

func TestApplyCraftElementPatchPreservesRawFieldsAndNormalizes(t *testing.T) {
	values := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
			"character_arc": "learns the cost of power",
			"power_level":   3,
		},
	}
	patch := []byte(`{"notes":"updated","power_level":-5,"dsl_tags":["lead","lead"]}`)

	key, action, changes, err := toolplane.ApplyCraftElementPatch(values, "character", "Lin", patch)
	if err != nil {
		t.Fatal(err)
	}
	if key != "Lin" || action != "update" {
		t.Fatalf("key/action = %q/%q, want Lin/update", key, action)
	}
	got := values["Lin"]
	if got["character_arc"] != "learns the cost of power" {
		t.Fatalf("raw unknown field was not preserved: %#v", got)
	}
	if got["power_level"] != 0 {
		t.Fatalf("power_level = %#v, want normalized 0", got["power_level"])
	}
	tags, ok := got["dsl_tags"].([]interface{})
	if !ok || len(tags) != 1 || tags[0] != "lead" {
		t.Fatalf("dsl_tags not compacted: %#v", got["dsl_tags"])
	}
	if !hasPatchChange(changes, "craft.character.Lin.notes") ||
		!hasPatchChange(changes, "craft.character.Lin.power_level") {
		t.Fatalf("expected notes and power changes, got %#v", changes)
	}
}

func TestApplyCraftElementPatchCreatesElementWithName(t *testing.T) {
	values := map[string]map[string]interface{}{}
	key, action, _, err := toolplane.ApplyCraftElementPatch(values, "item", "Star Core", []byte(`{"type":"artifact","description":"Ancient core","function":"stores power","significance":"main reward"}`))
	if err != nil {
		t.Fatal(err)
	}
	if key != "Star Core" || action != "create" {
		t.Fatalf("key/action = %q/%q, want Star Core/create", key, action)
	}
	if values["Star Core"]["name"] != "Star Core" {
		t.Fatalf("created item missing normalized name: %#v", values["Star Core"])
	}
}

func TestRunToolCraftPatchApplyWritesFileAndCheckpoint(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
		},
	}
	if err := writeCraftFixture(t, root, store.CraftCharacters, initial); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := toolplane.RunToolCraftPatch(cmd, toolRoot(t, root), "character", "Lin", []byte(`{"notes":"updated by tool"}`), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeToolPatchResult(t, out.String())
	if len(result.NextActions) != 1 ||
		result.NextActions[0].Action != "post_apply_check" ||
		!strings.Contains(result.NextActions[0].Command, `tool check schema --target craft --scope character --id 'Lin'`) {
		t.Fatalf("craft apply should return schema check next action: %#v", result.NextActions)
	}

	data, err := os.ReadFile(filepath.Join(root, "story", "craft", "characters.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["Lin"]["notes"] != "updated by tool" {
		t.Fatalf("patch was not written: %#v", saved["Lin"])
	}
	checkpoints, err := filepath.Glob(filepath.Join(root, "story", "craft", "checkpoints", "characters_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint count = %d, want 1", len(checkpoints))
	}
}

func TestRunToolCraftSchemaCheckFlagsProtagonistFieldDistribution(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
			"notes":         strings.Repeat("tactical note ", 120),
		},
	}
	if err := writeCraftFixture(t, root, store.CraftCharacters, initial); err != nil {
		t.Fatal(err)
	}

	result, err := toolplane.RunToolCraftSchemaCheck(root, "character", "Lin")
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocking {
		t.Fatalf("craft quality suggestions should not block old data: %#v", result)
	}
	if result.Summary.Medium < 3 || result.Summary.Low < 1 {
		t.Fatalf("expected protagonist field distribution suggestions, got summary=%#v issues=%#v", result.Summary, result.Issues)
	}
	if !toolCheckHasIssue(result, "personality") ||
		!toolCheckHasIssue(result, "motivation") ||
		!toolCheckHasIssue(result, "skills or abilities") ||
		!toolCheckHasIssue(result, "notes") {
		t.Fatalf("missing expected craft quality issues: %#v", result.Issues)
	}
}

func TestRunToolCraftPatchDryRunReturnsCraftQualityCheck(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
		},
	}
	if err := writeCraftFixture(t, root, store.CraftCharacters, initial); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := toolplane.RunToolCraftPatch(cmd, toolRoot(t, root), "character", "Lin", []byte(`{"notes":"updated only"}`), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeToolPatchResult(t, out.String())
	if result.Check == nil || result.Check.Summary.Total == 0 {
		t.Fatalf("craft patch dry-run should return quality check issues: %#v", result.Check)
	}
	if result.Check.Blocking {
		t.Fatalf("craft quality suggestions should not block dry-run: %#v", result.Check)
	}
	if !toolCheckHasIssue(result.Check, "structured skills or abilities") {
		t.Fatalf("missing structured field issue: %#v", result.Check.Issues)
	}
}

func TestRunToolRecapCheckValidatesSavedRecap(t *testing.T) {
	root := t.TempDir()
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "醒来",
		Location:        "残骸",
		Time:            "同夜",
		Present:         []string{"林砚"},
		PlotBeats:       []string{"林砚醒来。"},
		LastLine:        "他推开舱门。",
		NextOpeningHint: "他推开舱门后，先看见蓝色火光。",
	}); err != nil {
		t.Fatal(err)
	}

	result, err := toolplane.RunToolRecapCheck(root, "quality", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Blocking || result.Summary.Total != 0 {
		t.Fatalf("recap check should pass: %#v", result)
	}
}

func TestRunToolRecapCheckReportsMissingRecap(t *testing.T) {
	result, err := toolplane.RunToolRecapCheck(t.TempDir(), "quality", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !result.Blocking || result.Summary.High != 1 {
		t.Fatalf("missing recap should block: %#v", result)
	}
	if !strings.Contains(result.Issues[0].Issue, "not available") {
		t.Fatalf("unexpected issue: %#v", result.Issues)
	}
}

func TestRunToolRecapCheckReportsMinimalAndConsistencyIssues(t *testing.T) {
	root := t.TempDir()
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "醒来",
		Location:        "",
		Present:         nil,
		LastLine:        "他推开舱门。",
		NextOpeningHint: "远处传来警报。",
	}); err != nil {
		t.Fatal(err)
	}

	result, err := toolplane.RunToolRecapCheck(root, "all", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !result.Blocking || result.Summary.High < 2 || result.Summary.Medium == 0 {
		t.Fatalf("recap check should report high minimal and medium consistency issues: %#v", result)
	}
}

func TestRunToolRecapCheckReportsOutlineTitleMismatch(t *testing.T) {
	root := t.TempDir()
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "旧标题",
		Location:        "残骸",
		Present:         []string{"林砚"},
		PlotBeats:       []string{"林砚醒来。"},
		LastLine:        "他推开舱门。",
		NextOpeningHint: "他推开舱门后，先看见蓝色火光。",
	}); err != nil {
		t.Fatal(err)
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:    "P1-V1-C1",
				Title: "新标题",
			}},
		}},
	}}}
	if err := os.MkdirAll(filepath.Join(root, "story", "compose"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := toolRoot(t, root).SaveOutline(outline); err != nil {
		t.Fatal(err)
	}

	result, err := toolplane.RunToolRecapCheck(root, "quality", "chapter", "P1-V1-C1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocking || result.Summary.Medium != 1 {
		t.Fatalf("title mismatch should be non-blocking medium issue: %#v", result)
	}
	if !strings.Contains(result.Issues[0].Issue, "does not match outline title") {
		t.Fatalf("unexpected issue: %#v", result.Issues)
	}
}

func TestRunToolRecapPatchApplyWritesValidatedFileAndCheckpoint(t *testing.T) {
	root := t.TempDir()
	chapterID := "P1-V1-C1"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       chapterID,
		Title:           "Old title",
		Location:        "Old place",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Old beat"},
		LastLine:        "Old last line.",
		NextOpeningHint: "Old last line opens the next scene.",
	}); err != nil {
		t.Fatal(err)
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:    chapterID,
				Title: "Outline title",
			}},
		}},
	}}}
	if err := os.MkdirAll(filepath.Join(root, "story", "compose"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := toolRoot(t, root).SaveOutline(outline); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := toolplane.RunToolRecapPatch(cmd, toolRoot(t, root), chapterID, []byte(`{
		"chapter_id":"WRONG",
		"title":"Wrong title",
		"location":"New place",
		"present":["Lin","Mira"],
		"plot_beats":["New beat"],
		"last_line":"Lin opens the sealed door.",
		"next_opening_hint":"Lin opens the sealed door and Mira sees blue light."
	}`), true, nil)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := recap.NewStore(root).Load(chapterID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ChapterID != chapterID || saved.Title != "Outline title" {
		t.Fatalf("recap identity not fixed by Go: %#v", saved)
	}
	if saved.Location != "New place" || len(saved.Present) != 2 || saved.PlotBeats[0] != "New beat" {
		t.Fatalf("recap patch not applied: %#v", saved)
	}
	checkpoints, err := filepath.Glob(filepath.Join(root, "story", "recaps", "checkpoints", chapterID+"_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint count = %d, want 1", len(checkpoints))
	}
	output := out.String()
	if !strings.Contains(output, `"target": "recap.chapter"`) ||
		!strings.Contains(output, "ignored patch chapter_id") ||
		!strings.Contains(output, "ignored patch title") {
		t.Fatalf("unexpected recap patch output: %s", output)
	}
	result := decodeToolPatchResult(t, output)
	if len(result.NextActions) != 1 ||
		result.NextActions[0].Action != "post_apply_check" ||
		!strings.Contains(result.NextActions[0].Command, `tool check quality --target recap --scope chapter --id 'P1-V1-C1'`) {
		t.Fatalf("recap apply should return recap check next action: %#v", result.NextActions)
	}
}

func TestRunToolRecapPatchRejectsBlockingApply(t *testing.T) {
	root := t.TempDir()
	chapterID := "P1-V1-C1"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       chapterID,
		Title:           "Opening",
		Location:        "Mine",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Lin wakes."},
		LastLine:        "Lin opens the sealed door.",
		NextOpeningHint: "Lin opens the sealed door and sees blue light.",
	}); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	err := toolplane.RunToolRecapPatch(cmd, toolRoot(t, root), chapterID, []byte(`{"location":"","present":[]}`), true, nil)
	if err == nil || !strings.Contains(err.Error(), "patch rejected") {
		t.Fatalf("expected blocking recap patch rejection, got %v", err)
	}
	saved, err := recap.NewStore(root).Load(chapterID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Location != "Mine" || len(saved.Present) != 1 {
		t.Fatalf("blocking apply should not write recap: %#v", saved)
	}
}

func TestRunToolSetupCheckAllMergesQualityAndSimulation(t *testing.T) {
	setup := &models.StorySetup{
		ProjectName: "Thin",
	}
	result, err := toolplane.RunToolSetupCheck("all", setup)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "all" || result.Target != "setup" || result.Scope != "all" {
		t.Fatalf("unexpected setup check identity: %#v", result)
	}
	if result.OK || !result.Blocking {
		t.Fatalf("thin setup should have blocking issues: %#v", result.Summary)
	}
	if result.Summary.Total == 0 {
		t.Fatalf("expected setup check issues")
	}
}

func TestRunToolSetupPatchApplyWritesFilesAndCheckpoint(t *testing.T) {
	root := t.TempDir()
	setup := testfixtures.ValidLongFormSetup()
	if err := toolRoot(t, root).SaveSetup(setup); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetOut(&out)
	err := toolplane.RunToolSetupPatch(cmd, toolRoot(t, root), []byte(`{"theme":"updated by setup patch"}`), true, nil)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := toolRoot(t, root).LoadSetup()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Theme != "updated by setup patch" {
		t.Fatalf("setup patch was not written: %q", saved.Theme)
	}
	if _, err := os.Stat(filepath.Join(root, "story", "setup", "story_setup.md")); err != nil {
		t.Fatalf("setup markdown was not written: %v", err)
	}
	checkpoints, err := filepath.Glob(filepath.Join(root, "story", "setup", "checkpoints", "story_setup_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("checkpoint count = %d, want 1", len(checkpoints))
	}
	if !strings.Contains(out.String(), `"applied": true`) || !strings.Contains(out.String(), `"target": "setup"`) {
		t.Fatalf("unexpected setup patch output: %s", out.String())
	}
	result := decodeToolPatchResult(t, out.String())
	if len(result.NextActions) != 1 ||
		result.NextActions[0].Action != "post_apply_check" ||
		!strings.Contains(result.NextActions[0].Command, "tool check all --target setup") {
		t.Fatalf("setup apply should return setup check next action: %#v", result.NextActions)
	}
}

func TestApplySetupPatchObjectUpsertsPremises(t *testing.T) {
	setup := *testfixtures.ValidLongFormSetup()
	setup.Premises = []models.Premise{{
		Name:        "Ranking System",
		Category:    "progression",
		Description: "public rank ladder",
		Progression: []models.ProgressionStage{{
			Level:       1,
			Name:        "Outer Rank",
			Description: "entry rank",
		}},
	}}

	merged, warnings, err := toolplane.ApplySetupPatchObject(setup, []byte(`{
		"premises": [{
			"name": "Zerg Tier Ladder",
			"category": "faction",
			"description": "Zerg ranks progress from drone to soldier to captain."
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "upsert") {
		t.Fatalf("expected upsert warning, got %#v", warnings)
	}
	if len(merged.Premises) != 2 {
		t.Fatalf("premises len = %d, want 2: %#v", len(merged.Premises), merged.Premises)
	}
	if merged.Premises[0].Name != "Ranking System" || merged.Premises[0].Progression[0].Name != "Outer Rank" {
		t.Fatalf("existing premise was not preserved: %#v", merged.Premises[0])
	}
	if merged.Premises[1].Name != "Zerg Tier Ladder" || merged.Premises[1].Category != "faction" {
		t.Fatalf("new premise was not appended: %#v", merged.Premises[1])
	}

	merged, _, err = toolplane.ApplySetupPatchObject(merged, []byte(`{
		"premises": [{
			"name": "Ranking System",
			"description": "updated public ladder"
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Premises) != 2 {
		t.Fatalf("update should not append duplicate premise: %#v", merged.Premises)
	}
	if merged.Premises[0].Description != "updated public ladder" ||
		merged.Premises[0].Category != "progression" ||
		merged.Premises[0].Progression[0].Name != "Outer Rank" {
		t.Fatalf("existing premise should be field-merged: %#v", merged.Premises[0])
	}
}

func TestApplySetupPatchObjectMergesNestedPlanAndStringLists(t *testing.T) {
	setup := *testfixtures.ValidLongFormSetup()
	setup.Rules = []string{"Existing rule"}
	setup.LongFormPlan = &models.LongFormPlan{
		TargetChapters:   1000,
		TargetVolumes:    10,
		MainLoop:         "old loop",
		EscalationLadder: []string{"frontier", "city"},
		ReaderPromises:   []string{"visible wins"},
		PayoffCadence:    "old cadence",
		VolumePattern:    []string{"hook", "win"},
		EndgamePromise:   "old endgame",
	}

	merged, warnings, err := toolplane.ApplySetupPatchObject(setup, []byte(`{
		"rules": ["Existing rule", "New faction rule"],
		"long_form_plan": {
			"main_loop": "new loop",
			"escalation_ladder": ["city", "region"],
			"reader_promises": ["visible wins", "territory growth"]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "rules") || !strings.Contains(warnings[0], "long_form_plan") {
		t.Fatalf("expected incremental merge warning, got %#v", warnings)
	}
	if len(merged.Rules) != 2 || merged.Rules[0] != "Existing rule" || merged.Rules[1] != "New faction rule" {
		t.Fatalf("rules should append/dedupe, got %#v", merged.Rules)
	}
	if merged.LongFormPlan == nil {
		t.Fatal("long_form_plan should remain present")
	}
	if merged.LongFormPlan.MainLoop != "new loop" ||
		merged.LongFormPlan.TargetChapters != 1000 ||
		merged.LongFormPlan.TargetVolumes != 10 ||
		merged.LongFormPlan.PayoffCadence != "old cadence" ||
		merged.LongFormPlan.EndgamePromise != "old endgame" {
		t.Fatalf("long_form_plan scalar fields were not merged safely: %#v", merged.LongFormPlan)
	}
	if strings.Join(merged.LongFormPlan.EscalationLadder, "|") != "frontier|city|region" {
		t.Fatalf("escalation ladder should append/dedupe: %#v", merged.LongFormPlan.EscalationLadder)
	}
	if strings.Join(merged.LongFormPlan.ReaderPromises, "|") != "visible wins|territory growth" {
		t.Fatalf("reader promises should append/dedupe: %#v", merged.LongFormPlan.ReaderPromises)
	}
	if strings.Join(merged.LongFormPlan.VolumePattern, "|") != "hook|win" {
		t.Fatalf("untouched nested arrays should be preserved: %#v", merged.LongFormPlan.VolumePattern)
	}
}

func TestApplySetupPatchObjectPatchesStringListByIndex(t *testing.T) {
	setup := *testfixtures.ValidLongFormSetup()
	setup.Genres = []string{"sci-fi", "progression"}
	setup.Rules = []string{"short rule", "this rule is far too verbose and needs to be compacted"}

	merged, warnings, err := toolplane.ApplySetupPatchObject(setup, []byte(`{
		"genres_patch": [{"index": 1, "value": "mecha progression"}],
		"rules_patch": [{"index": 1, "value": "Compact rule."}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "genres_patch") || !strings.Contains(warnings[0], "rules_patch") {
		t.Fatalf("expected patch warning, got %#v", warnings)
	}
	if strings.Join(merged.Genres, "|") != "sci-fi|mecha progression" {
		t.Fatalf("genres patch did not replace by index: %#v", merged.Genres)
	}
	if strings.Join(merged.Rules, "|") != "short rule|Compact rule." {
		t.Fatalf("rules patch did not replace by index: %#v", merged.Rules)
	}
}

func TestApplySetupPatchObjectRejectsStringListPatchOutOfRange(t *testing.T) {
	setup := *testfixtures.ValidLongFormSetup()
	setup.Rules = []string{"short rule"}

	_, _, err := toolplane.ApplySetupPatchObject(setup, []byte(`{"rules_patch":[{"index":2,"value":"nope"}]}`))
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected out-of-range error, got %v", err)
	}
}

func TestRunToolPatchSetupDoesNotRequireID(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"title":"Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	setup := testfixtures.ValidLongFormSetup()
	if err := toolRoot(t, root).SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	storeRoot := toolRoot(t, root)
	t.Cleanup(func() {
		toolplane.ToolPatchFlags = struct {
			Target         string
			ID             string
			PatchJSON      string
			PatchBuffer    string
			Task           string
			Apply          bool
			DryRun         bool
			RefreshDerived bool
			TargetWords    int
		}{}
	})

	toolplane.ToolPatchFlags.PatchJSON = `{"tone":"dry-run setup tone"}`
	cmd := bindProject(&cobra.Command{}, storeRoot)
	var out strings.Builder
	cmd.SetOut(&out)
	if err := toolplane.RunToolPatch(cmd, []string{"setup"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) || !strings.Contains(out.String(), "setup.tone") {
		t.Fatalf("unexpected setup dry-run output: %s", out.String())
	}
	result := decodeToolPatchResult(t, out.String())
	if len(result.NextActions) == 0 || result.NextActions[0].Action != "apply_validated_patch" {
		t.Fatalf("setup dry-run should return apply next action: %#v", result.NextActions)
	}
	reloaded, err := toolRoot(t, root).LoadSetup()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Tone == "dry-run setup tone" {
		t.Fatalf("dry-run setup patch should not write file")
	}
}

func TestRunToolPatchStripsBOMFromStdinPatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "novel.json"), []byte(`{"title":"Test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"role_in_story": "protagonist",
		},
	}
	if err := writeCraftFixture(t, root, store.CraftCharacters, initial); err != nil {
		t.Fatal(err)
	}
	storeRoot := toolRoot(t, root)
	t.Cleanup(func() {
		toolplane.ToolPatchFlags = struct {
			Target         string
			ID             string
			PatchJSON      string
			PatchBuffer    string
			Task           string
			Apply          bool
			DryRun         bool
			RefreshDerived bool
			TargetWords    int
		}{}
	})

	toolplane.ToolPatchFlags.Target = "character"
	toolplane.ToolPatchFlags.ID = "Lin"
	cmd := bindProject(&cobra.Command{}, storeRoot)
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader(string([]byte{0xEF, 0xBB, 0xBF}) + `{"notes":"stdin patch"}`))

	if err := toolplane.RunToolPatch(cmd, []string{"craft"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) || !strings.Contains(out.String(), "craft.character.Lin.notes") {
		t.Fatalf("unexpected dry-run output: %s", out.String())
	}
}

func TestRunToolCraftSchemaCheckValidatesSavedCharacter(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"appearance":    "lean pilot",
			"personality":   []interface{}{"focused", "risk-aware", "dry humor"},
			"background":    "old world survivor",
			"motivation":    "survive",
			"skills":        []interface{}{"route planning"},
			"abilities":     []interface{}{"system log reading with blind spots"},
			"role_in_story": "lead",
			"voice":         "short, observant, lightly sarcastic",
			"rpg_stats": map[string]interface{}{
				"str": float64(6),
				"agi": float64(5),
			},
		},
	}
	if err := writeCraftFixture(t, root, store.CraftCharacters, initial); err != nil {
		t.Fatal(err)
	}

	result, err := toolplane.RunToolCraftSchemaCheck(root, "character", "Lin")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Blocking || result.Summary.Total != 0 {
		t.Fatalf("schema check should pass: %#v", result)
	}
}

func TestRunToolCraftSchemaCheckReportsInvalidSavedCharacter(t *testing.T) {
	root := t.TempDir()
	initial := map[string]map[string]interface{}{
		"Lin": {
			"name":          "Lin",
			"appearance":    "lean pilot",
			"personality":   []interface{}{"focused"},
			"background":    "old world survivor",
			"motivation":    "survive",
			"role_in_story": "lead",
			"rpg_stats": map[string]interface{}{
				"perception": float64(6),
			},
		},
	}
	if err := writeCraftFixture(t, root, store.CraftCharacters, initial); err != nil {
		t.Fatal(err)
	}

	result, err := toolplane.RunToolCraftSchemaCheck(root, "character", "Lin")
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !result.Blocking || result.Summary.High != 1 {
		t.Fatalf("schema check should fail: %#v", result)
	}
	if !strings.Contains(result.Issues[0].Issue, "perception") {
		t.Fatalf("schema issue should mention invalid key: %#v", result.Issues)
	}
}

func TestApplyCraftElementPatchRejectsSchemaMismatch(t *testing.T) {
	values := map[string]map[string]interface{}{
		"Lin": {"name": "Lin", "role_in_story": "protagonist"},
	}
	_, _, _, err := toolplane.ApplyCraftElementPatch(values, "character", "Lin", []byte(`{"personality":"should be array"}`))
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("expected schema mismatch, got %v", err)
	}
}

func validToolPatchTestOutline() *models.Outline {
	return &models.Outline{Parts: []models.Part{{
		ID:      "P1",
		Title:   "Part One",
		Summary: "Part arc",
		Volumes: []models.Volume{{
			ID:      "P1-V1",
			Title:   "Volume One",
			Summary: "Volume arc",
			Chapters: []models.Chapter{{
				ID:         "P1-V1-C1",
				Title:      "Opening",
				Summary:    "Lin enters the mine and finds a star core.",
				Characters: []string{"Lin"},
				Location:   "Mine",
				Events: []models.Event{{
					Actor:      "Lin",
					Action:     models.ActionAcquire,
					Target:     "Star Core",
					TargetType: models.TargetTypeItem,
				}, {
					Actor:      "Lin",
					Action:     models.ActionMove,
					Target:     "Camp",
					TargetType: models.TargetTypeLocation,
				}, {
					Actor:      "Lin",
					Action:     models.ActionDiscover,
					Target:     "Signal",
					TargetType: models.TargetTypeKnowledge,
				}},
				StateChange: "Lin gains a star core and a new lead.",
				Conflict:    "The mine is unstable.",
				Pacing:      "normal",
				Timeline:    models.ChapterTimeline{Anchor: "Day 1"},
				StateAnchor: models.StateAnchor{Location: "Mine"},
				Scenes: []models.OutlineScene{{
					Order: 1, POV: "Lin", Goal: "Find supplies", Beats: []string{"Search the mine"},
				}, {
					Order: 2, POV: "Lin", Goal: "Escape", Beats: []string{"Follow the signal"},
				}},
			}},
		}},
	}}}
}

func hasPatchChange(changes []toolplane.ToolPatchChange, path string) bool {
	for _, change := range changes {
		if change.Path == path {
			return true
		}
	}
	return false
}

func saveTestRecap(root string, value models.ChapterRecap) error {
	dir := filepath.Join(root, "story", "recaps")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, value.ChapterID+".json"), data, 0644)
}
