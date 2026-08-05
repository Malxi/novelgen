package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"
)

func TestFindProjectChapterInputsSkipsStaleRecap(t *testing.T) {
	root := t.TempDir()
	chapterDir := filepath.Join(root, "chapters")
	recapDir := filepath.Join(root, "story", "recaps")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(recapDir, 0755); err != nil {
		t.Fatal(err)
	}

	chapterPath := filepath.Join(chapterDir, "chapter-P1-V1-C1.md")
	recapPath := filepath.Join(recapDir, "P1-V1-C1.json")
	if err := os.WriteFile(chapterPath, []byte("# Chapter\n\nNew prose."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recapPath, []byte(`{"chapter_id":"P1-V1-C1","title":"Old"}`), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(recapPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(chapterPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	inputs, err := findProjectChapterInputs(root, oneChapterOutline("P1-V1-C1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("inputs len = %d, want 1", len(inputs))
	}
	if inputs[0].RecapPath != "" {
		t.Fatalf("RecapPath = %q, want skipped stale recap", inputs[0].RecapPath)
	}
}

func TestFindProjectChapterInputsKeepsFreshRecap(t *testing.T) {
	root := t.TempDir()
	chapterDir := filepath.Join(root, "chapters")
	recapDir := filepath.Join(root, "story", "recaps")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(recapDir, 0755); err != nil {
		t.Fatal(err)
	}

	chapterPath := filepath.Join(chapterDir, "chapter-P1-V1-C1.md")
	recapPath := filepath.Join(recapDir, "P1-V1-C1.json")
	if err := os.WriteFile(chapterPath, []byte("# Chapter\n\nNew prose."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recapPath, []byte(`{"chapter_id":"P1-V1-C1","title":"Fresh"}`), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(chapterPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recapPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	inputs, err := findProjectChapterInputs(root, oneChapterOutline("P1-V1-C1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 {
		t.Fatalf("inputs len = %d, want 1", len(inputs))
	}
	if inputs[0].RecapPath != recapPath {
		t.Fatalf("RecapPath = %q, want %q", inputs[0].RecapPath, recapPath)
	}
}

func TestEnrichWriteRPGDSLCombatSignalsAddsStructuredMechAndGene(t *testing.T) {
	parsed := &rpgdsl.DSL{Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{{
		ID: "P1-V1-C1",
		Objectives: []rpgdsl.Objective{{
			Steps: []rpgdsl.Step{{
				Order: 1,
				Event: rpgdsl.Event{StateDeltas: []rpgdsl.StateDelta{
					{Target: "char_lvye", Kind: "breakthrough", Field: "gene_level", From: "0", To: "1"},
					{Target: "char_lvye", Kind: "resource", Field: "stability", Delta: 1, Note: "from 61% to 62%"},
					{Target: "char_lvye", Kind: "item", Field: "equipment", To: "acquired", Note: "basic exoskeleton with energy core"},
				}},
			}},
		}},
	}}}}

	enrichWriteRPGDSLCombatSignals(parsed)

	deltas := parsed.Storyline.Chapters[0].Objectives[0].Steps[0].Event.StateDeltas
	assertDelta := func(kind, field, to string) {
		t.Helper()
		for _, delta := range deltas {
			if delta.Kind == kind && delta.Field == field && delta.To == to {
				return
			}
		}
		t.Fatalf("missing delta kind=%q field=%q to=%q in %#v", kind, field, to, deltas)
	}
	assertDelta("gene", "level", "1")
	assertDelta("gene", "stability", "62")
	assertDelta("mech", "form", "basic exoskeleton with energy core")
	assertDelta("mech", "energy", "80")
}

func TestEnrichWriteRPGDSLCombatSignalsReadsChineseStepText(t *testing.T) {
	parsed := &rpgdsl.DSL{
		Characters: &rpgdsl.Characters{Player: &rpgdsl.Player{ID: "char_linye"}},
		Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{{
			Objectives: []rpgdsl.Objective{{
				Steps: []rpgdsl.Step{
					{
						Description: "\u4f7f\u75283D\u6253\u5370\u673a\u5236\u4f5c\u5916\u9aa8\u9abc",
						Event: rpgdsl.Event{StateDeltas: []rpgdsl.StateDelta{{
							Target: "resource_01",
							Kind:   "resource",
							Field:  "item",
							Delta:  -1,
							Note:   "\u542f\u52a8\u5916\u9aa8\u9abc",
						}}},
					},
					{
						Description: "\u57fa\u56e0\u9002\u914d\u5b8c\u6210\uff0c\u7a33\u5b9a\u602762%",
						Event:       rpgdsl.Event{},
					},
				},
			}},
		}}},
	}

	enrichWriteRPGDSLCombatSignals(parsed)

	step1 := parsed.Storyline.Chapters[0].Objectives[0].Steps[0]
	if !hasTestDelta(step1.Event.StateDeltas, "char_linye", "mech", "form") ||
		!hasTestDelta(step1.Event.StateDeltas, "char_linye", "mech", "energy") {
		t.Fatalf("expected resource/mech text to add protagonist mech deltas, got %#v", step1.Event.StateDeltas)
	}
	step2 := parsed.Storyline.Chapters[0].Objectives[0].Steps[1]
	if !hasTestDelta(step2.Event.StateDeltas, "char_linye", "gene", "level") ||
		!hasTestDelta(step2.Event.StateDeltas, "char_linye", "gene", "stability") {
		t.Fatalf("expected gene text to add protagonist gene deltas, got %#v", step2.Event.StateDeltas)
	}
}

func TestEnrichWriteRPGDSLCombatSignalsPreservesExplicitStructuredDeltas(t *testing.T) {
	parsed := &rpgdsl.DSL{Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{{
		Objectives: []rpgdsl.Objective{{
			Steps: []rpgdsl.Step{{
				Event: rpgdsl.Event{StateDeltas: []rpgdsl.StateDelta{
					{Target: "char_lvye", Kind: "item", Field: "equipment", To: "armor"},
					{Target: "char_lvye", Kind: "mech", Field: "form", To: "explicit mech"},
					{Target: "char_lvye", Kind: "mech", Field: "energy", To: "45"},
				}},
			}},
		}},
	}}}}

	enrichWriteRPGDSLCombatSignals(parsed)

	deltas := parsed.Storyline.Chapters[0].Objectives[0].Steps[0].Event.StateDeltas
	formCount := 0
	energyCount := 0
	for _, delta := range deltas {
		if delta.Kind == "mech" && delta.Field == "form" {
			formCount++
			if delta.To != "explicit mech" {
				t.Fatalf("explicit mech form overwritten: %#v", delta)
			}
		}
		if delta.Kind == "mech" && delta.Field == "energy" {
			energyCount++
			if delta.To != "45" {
				t.Fatalf("explicit mech energy overwritten: %#v", delta)
			}
		}
	}
	if formCount != 1 || energyCount != 1 {
		t.Fatalf("expected one explicit form and energy delta, got form=%d energy=%d deltas=%#v", formCount, energyCount, deltas)
	}
}

func TestFilterWriteRPGDSLChapterInputsTargetsOnlyRequestedChapters(t *testing.T) {
	inputs := []writeRPGDSLChapterInput{
		{ChapterID: "P1-V1-C1"},
		{ChapterID: "P1-V1-C2"},
		{ChapterID: "P1-V1-C3"},
	}
	filtered := filterWriteRPGDSLChapterInputs(inputs, writeRPGDSLTargetChapterSet([]string{"P1-V1-C2"}))
	if len(filtered) != 1 || filtered[0].ChapterID != "P1-V1-C2" {
		t.Fatalf("filtered = %#v, want only P1-V1-C2", filtered)
	}
}

func TestRemoveWriteRPGDSLChaptersKeepsUntargetedChapters(t *testing.T) {
	dsl := &rpgdsl.DSL{Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{
		{ID: "P1-V1-C1", Title: "one"},
		{ID: "P1-V1-C2", Title: "two-old"},
		{ID: "P1-V1-C3", Title: "three"},
	}}}

	removeWriteRPGDSLChapters(dsl, writeRPGDSLTargetChapterSet([]string{"P1-V1-C2"}))
	mergeWriteRPGDSL(dsl, &rpgdsl.DSL{Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{
		{ID: "P1-V1-C2", Title: "two-new"},
	}}})

	if len(dsl.Storyline.Chapters) != 3 {
		t.Fatalf("chapter count = %d, want 3", len(dsl.Storyline.Chapters))
	}
	titles := map[string]string{}
	for _, chapter := range dsl.Storyline.Chapters {
		titles[chapter.ID] = chapter.Title
	}
	if titles["P1-V1-C1"] != "one" || titles["P1-V1-C2"] != "two-new" || titles["P1-V1-C3"] != "three" {
		t.Fatalf("unexpected chapters after replacement: %#v", titles)
	}
}

func TestEnsureWriteRPGDSLManifestChaptersWrapsBareObjectives(t *testing.T) {
	content := `metadata {
  title = "第13章"
}

storyline {
  objective "本章事件" {
    step 1 {
      description = "李侑取得玄冰玉髓"
      event {
        type = "acquire"
      }
    }
  }
}`
	parsed, err := rpgdsl.NewParser(content).Parse()
	if err != nil {
		t.Fatal(err)
	}
	manifest := &writeRPGDSLBatchManifest{Chapters: []writeRPGDSLChapterState{{ChapterID: "P1-V2-C3"}}}

	wrapped, reparsed, err := ensureWriteRPGDSLManifestChapters(content, parsed, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wrapped, `chapter "第13章"`) || !strings.Contains(wrapped, `id = "P1-V2-C3"`) {
		t.Fatalf("wrapped content missing chapter/id:\n%s", wrapped)
	}
	if reparsed.Storyline == nil || len(reparsed.Storyline.Chapters) != 1 {
		t.Fatalf("chapters = %#v", reparsed.Storyline)
	}
	chapter := reparsed.Storyline.Chapters[0]
	if chapter.ID != "P1-V2-C3" || len(chapter.Objectives) != 1 || len(chapter.Objectives[0].Steps) != 1 {
		t.Fatalf("chapter = %#v", chapter)
	}
}

func hasTestDelta(deltas []rpgdsl.StateDelta, target, kind, field string) bool {
	for _, delta := range deltas {
		if delta.Target == target && delta.Kind == kind && delta.Field == field {
			return true
		}
	}
	return false
}

func oneChapterOutline(chapterID string) *models.Outline {
	return &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID: chapterID,
			}},
		}},
	}}}
}
