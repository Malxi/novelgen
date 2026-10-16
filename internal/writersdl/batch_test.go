package writersdl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"
	"novelgen/internal/store"
	"novelgen/internal/writersdl"
)

func inputFixture(t *testing.T, dir string, ids ...string) []writersdl.WriteRPGDSLChapterInput {
	t.Helper()
	inputs := make([]writersdl.WriteRPGDSLChapterInput, 0, len(ids))
	for _, id := range ids {
		path := filepath.Join(dir, "chapter-"+id+".md")
		if err := os.WriteFile(path, []byte("# "+id+"\n\nprose\n"), 0644); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, writersdl.WriteRPGDSLChapterInput{ChapterID: id, ChapterPath: path})
	}
	return inputs
}

func TestBuildWriteRPGDSLBatchManifestIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	inputs := inputFixture(t, dir, "P1-V1-C1", "P1-V1-C2")

	first, err := writersdl.BuildWriteRPGDSLBatchManifest(store.OSFS{}, "book", 0, 10, inputs, cacheDir)
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	second, err := writersdl.BuildWriteRPGDSLBatchManifest(store.OSFS{}, "book", 0, 10, inputs, cacheDir)
	if err != nil {
		t.Fatalf("build manifest again: %v", err)
	}
	if first.BatchHash == "" || first.BatchHash != second.BatchHash {
		t.Fatalf("batch hash should be stable, got %q and %q", first.BatchHash, second.BatchHash)
	}
	if !strings.HasPrefix(filepath.Base(first.DSLFile), "batch_001_P1-V1-C1_P1-V1-C2_") {
		t.Fatalf("DSL file name = %q", first.DSLFile)
	}
	if len(first.Chapters) != 2 || first.Chapters[0].ChapterSHA256 == "" {
		t.Fatalf("chapter states = %#v", first.Chapters)
	}
	if first.Version == 0 {
		t.Fatal("manifest version should be pinned")
	}

	if _, err := writersdl.BuildWriteRPGDSLBatchManifest(store.OSFS{}, "book", 0, 10, nil, cacheDir); err == nil {
		t.Fatal("empty batch should be rejected")
	}
}

func TestBatchCacheRoundTripAndInvalidation(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	inputs := inputFixture(t, dir, "P1-V1-C1")
	manifest, err := writersdl.BuildWriteRPGDSLBatchManifest(store.OSFS{}, "book", 0, 10, inputs, cacheDir)
	if err != nil {
		t.Fatal(err)
	}

	if _, reused, err := writersdl.LoadWriteRPGDSLBatchFromCache(store.OSFS{}, manifest); err != nil || reused {
		t.Fatalf("missing cache should not be reused: reused=%v err=%v", reused, err)
	}

	const cached = `storyline {
  chapter "Chapter 1" {
    id = "P1-V1-C1"
    objective "one" {
      step 1 {
        description = "a"
      }
    }
  }
}`
	if err := writersdl.SaveWriteRPGDSLBatchToCache(store.OSFS{}, manifest, cached); err != nil {
		t.Fatal(err)
	}
	content, reused, err := writersdl.LoadWriteRPGDSLBatchFromCache(store.OSFS{}, manifest)
	if err != nil || !reused {
		t.Fatalf("cache should be reused: reused=%v err=%v", reused, err)
	}
	if content != cached {
		t.Fatalf("cached content = %q, want %q", content, cached)
	}

	// A cached payload that no longer parses must be ignored, not returned.
	if err := writersdl.SaveWriteRPGDSLBatchToCache(store.OSFS{}, manifest, "this is not DSL"); err != nil {
		t.Fatal(err)
	}
	if _, reused, err := writersdl.LoadWriteRPGDSLBatchFromCache(store.OSFS{}, manifest); err != nil || reused {
		t.Fatalf("unparseable cache should be invalidated: reused=%v err=%v", reused, err)
	}
}

func TestNormalizeWriteRPGDSLChapterIDsAssignsByPosition(t *testing.T) {
	dir := t.TempDir()
	inputs := inputFixture(t, dir, "P1-V1-C1", "P1-V1-C2")
	manifest, err := writersdl.BuildWriteRPGDSLBatchManifest(store.OSFS{}, "book", 0, 10, inputs, filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}

	parsed := &rpgdsl.DSL{Storyline: &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{
		{ID: "wrong"}, {ID: "also-wrong"}, {ID: "extra"},
	}}}
	writersdl.NormalizeWriteRPGDSLChapterIDs(parsed, manifest)

	if parsed.Storyline.Chapters[0].ID != "P1-V1-C1" || parsed.Storyline.Chapters[1].ID != "P1-V1-C2" {
		t.Fatalf("chapter IDs = %#v", parsed.Storyline.Chapters)
	}
	if parsed.Storyline.Chapters[2].ID != "extra" {
		t.Fatalf("chapters beyond the manifest must be untouched: %#v", parsed.Storyline.Chapters)
	}
}

func TestSplitSortAndParseChapterInputs(t *testing.T) {
	inputs := []writersdl.WriteRPGDSLChapterInput{
		{ChapterID: "P1-V2-C1"},
		{ChapterID: "P1-V1-C2"},
		{ChapterID: "P1-V1-C1"},
	}
	writersdl.SortRPGDSLChapterInputs(inputs)
	got := []string{inputs[0].ChapterID, inputs[1].ChapterID, inputs[2].ChapterID}
	want := []string{"P1-V1-C1", "P1-V1-C2", "P1-V2-C1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sorted = %v, want %v", got, want)
	}

	batches := writersdl.SplitRPGDSLBatchInputs(inputs, 2)
	if len(batches) != 2 || len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("batches = %#v", batches)
	}

	p, v, c := writersdl.ParseRPGDSLChapterID("P1-V2-C3")
	if p != 1 || v != 2 || c != 3 {
		t.Fatalf("ParseRPGDSLChapterID = %d/%d/%d", p, v, c)
	}
	if p, v, c := writersdl.ParseRPGDSLChapterID("chapter-one"); p != 9999 || v != 9999 || c != 9999 {
		t.Fatalf("unparseable chapter ID should sort last, got %d/%d/%d", p, v, c)
	}
}

func TestMergeWriteRPGDSLKeepsFirstOccurrence(t *testing.T) {
	dir := t.TempDir()
	inputs := inputFixture(t, dir, "P1-V1-C1")
	manifest, err := writersdl.BuildWriteRPGDSLBatchManifest(store.OSFS{}, "book", 0, 1, inputs, filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	_ = manifest

	dst := &rpgdsl.DSL{
		Metadata:   &rpgdsl.Metadata{Title: "kept", Genre: []string{"fantasy"}},
		World:      &rpgdsl.World{Locations: []rpgdsl.Location{{ID: "arena", Name: "Arena"}}, Rules: []rpgdsl.Rule{{Name: "rule-1"}}},
		Characters: &rpgdsl.Characters{NPCs: []rpgdsl.NPC{{ID: "hero", Name: "Hero"}}},
		Storyline:  &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{{ID: "P1-V1-C1"}}},
		Systems:    nil,
	}
	src := &rpgdsl.DSL{
		Metadata:   &rpgdsl.Metadata{Title: "ignored", Genre: []string{"fantasy", "wuxia"}},
		World:      &rpgdsl.World{Locations: []rpgdsl.Location{{ID: "arena", Name: "Other"}, {ID: "market", Name: "Market"}}, Rules: []rpgdsl.Rule{{Name: "rule-2"}}},
		Characters: &rpgdsl.Characters{NPCs: []rpgdsl.NPC{{ID: "hero", Name: "Other"}, {ID: "mentor", Name: "Mentor"}}},
		Storyline:  &rpgdsl.Storyline{Chapters: []rpgdsl.Chapter{{ID: "P1-V1-C2"}}},
		Systems:    &rpgdsl.Systems{},
	}

	writersdl.MergeWriteRPGDSL(dst, src)

	if dst.Metadata.Title != "kept" {
		t.Fatalf("existing metadata must win: %#v", dst.Metadata)
	}
	if strings.Join(dst.Metadata.Genre, ",") != "fantasy,wuxia" {
		t.Fatalf("genre merge = %v", dst.Metadata.Genre)
	}
	if len(dst.World.Locations) != 2 || dst.World.Locations[0].Name != "Arena" {
		t.Fatalf("locations = %#v", dst.World.Locations)
	}
	if len(dst.World.Rules) != 2 {
		t.Fatalf("rules should append: %#v", dst.World.Rules)
	}
	if len(dst.Characters.NPCs) != 2 || dst.Characters.NPCs[0].Name != "Hero" {
		t.Fatalf("npcs = %#v", dst.Characters.NPCs)
	}
	if len(dst.Storyline.Chapters) != 2 {
		t.Fatalf("chapters = %#v", dst.Storyline.Chapters)
	}
	if dst.Systems == nil {
		t.Fatal("nil systems should be filled from src")
	}
}

func TestDerefMapsSkipNilValues(t *testing.T) {
	characters := writersdl.DerefCharacterMap(map[string]*models.Character{
		"hero":  {Name: "Hero"},
		"ghost": nil,
	})
	if len(characters) != 1 || characters["hero"].Name != "Hero" {
		t.Fatalf("characters = %#v", characters)
	}

	locations := writersdl.DerefLocationMap(map[string]*models.Location{
		"arena": {Name: "Arena"},
		"void":  nil,
	})
	if len(locations) != 1 || locations["arena"].Name != "Arena" {
		t.Fatalf("locations = %#v", locations)
	}
}

func TestIssueFiltersAndSuggestionFormatting(t *testing.T) {
	issues := []rpgdsl.SimulationIssue{
		{Type: rpgdsl.IssueLogic, Severity: rpgdsl.SeverityCritical, Chapter: "P1-V1-C1", Step: 2, Description: "broken", Suggestion: "fix it"},
		{Type: rpgdsl.IssuePacing, Severity: rpgdsl.SeverityWarning, Chapter: "P1-V1-C2", Description: "slow"},
		{Type: rpgdsl.IssuePacing, Severity: rpgdsl.SeverityInfo, Chapter: "P1-V1-C2", Description: "note"},
	}

	important := writersdl.FilterImportantIssues(issues)
	if len(important) != 2 {
		t.Fatalf("important issues = %#v", important)
	}

	formatted := writersdl.FormatIssuesAsSuggestions(issues)
	for _, want := range []string{"DSL-RPG 模拟检测发现的问题", "[严重]", "[警告]", "章节 P1-V1-C1 (步骤 2)"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted issues missing %q:\n%s", want, formatted)
		}
	}
	if got := writersdl.FormatIssuesAsSuggestions(nil); got != "" {
		t.Fatalf("no issues should format to empty, got %q", got)
	}

	chapterOnly := writersdl.GetChapterSpecificSuggestions(issues, "P1-V1-C1")
	if strings.Contains(chapterOnly, "P1-V1-C2") || !strings.Contains(chapterOnly, "P1-V1-C1") {
		t.Fatalf("chapter suggestions should be scoped:\n%s", chapterOnly)
	}
}

func TestInferPhaseFromFilename(t *testing.T) {
	cases := map[string]rpgdsl.MergePhase{
		"01_setup.rpg":    rpgdsl.PhaseSetup,
		"01_OUTLINE.rpg":  rpgdsl.PhaseOutline,
		"02_craft.rpg":    rpgdsl.PhaseCraft,
		"03_systems.rpg":  rpgdsl.PhaseSystems,
		"04_chapters.rpg": rpgdsl.PhaseFinal,
	}
	for filename, want := range cases {
		if got := writersdl.InferPhaseFromFilename(filename); got != want {
			t.Errorf("InferPhaseFromFilename(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestStorylineBlockHelpers(t *testing.T) {
	content := "metadata {\n  title = \"book\"\n}\nstoryline {\n  objective \"x\" {\n    step 1 {\n      description = \"a { b }\"\n    }\n  }\n}\n"
	start, open, close, ok := writersdl.FindWriteRPGDSLStorylineBlock(content)
	if !ok {
		t.Fatalf("storyline block not found in:\n%s", content)
	}
	if content[start:start+len("storyline")] != "storyline" {
		t.Fatalf("start index points at %q", content[start:start+12])
	}
	if content[open] != '{' || content[close] != '}' {
		t.Fatalf("brace indices = %d/%d", open, close)
	}
	if !strings.Contains(content[open:close], `description = "a { b }"`) {
		t.Fatalf("nested braces inside a string must not close the block:\n%s", content[open:close])
	}

	if _, _, _, ok := writersdl.FindWriteRPGDSLStorylineBlock("metadata {}"); ok {
		t.Fatal("missing storyline block should report ok=false")
	}

	indented := writersdl.IndentWriteRPGDSLLines("a\n\n  b", "  ")
	if indented != "  a\n\n    b" {
		t.Fatalf("indent = %q", indented)
	}
}
