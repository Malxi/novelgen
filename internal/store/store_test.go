package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/store"
)

func writeProjectFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestLayoutPaths pins the whole on-disk contract in one place: this table is
// now the only description of where a project keeps its artifacts.
func TestLayoutPaths(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"NovelJSON", root.NovelJSON(), "book/novel.json"},
		{"LLMConfigJSON", root.LLMConfigJSON(), "book/llm_config.json"},
		{"SetupJSON", root.SetupJSON(), "book/story/setup/story_setup.json"},
		{"SetupMarkdown", root.SetupMarkdown(), "book/story/setup/story_setup.md"},
		{"OutlineJSON", root.OutlineJSON(), "book/story/compose/outline.json"},
		{"OutlineMarkdown", root.OutlineMarkdown(), "book/story/compose/outline.md"},
		{"ComposeFile", root.ComposeFile("outline_iter_3.json"), "book/story/compose/outline_iter_3.json"},
		{"CraftFile", root.CraftFile(store.CraftCharacters), "book/story/craft/characters.json"},
		{"CraftFileName", store.CraftOrganizations.FileName(), "organizations.json"},
		{"RecapFile", root.RecapFile("P1-V1-C1"), "book/story/recaps/P1-V1-C1.json"},
		{"VolumeReviewFile", root.VolumeReviewFile("P1-V1"), "book/story/reviews/P1-V1_review.json"},
		{"ReviewFile", root.ReviewFile("outline_review.json"), "book/story/reviews/outline_review.json"},
		{"RPGFile", root.RPGFile("01_outline.rpg"), "book/story/rpg/01_outline.rpg"},
		{"RPGChaptersDSL", root.RPGChaptersDSL(), "book/story/rpg/04_chapters.rpg"},
		{"RPGCacheDir", root.RPGCacheDir(), "book/story/rpg/cache"},
		{"RPGChapterBatchCacheDir", root.RPGChapterBatchCacheDir(), "book/story/rpg/cache/chapter_batches"},
		{"ChapterFile", root.ChapterFile("P1-V1-C1"), "book/chapters/chapter-P1-V1-C1.md"},
		{"DraftFile", root.DraftFile("P1-V1-C1"), "book/drafts/P1-V1-C1.md"},
		{"UsageLog", root.UsageLog(), "book/logs/usage.jsonl"},
		{"CloneManifest", root.CloneManifest(), "book/logs/clone_manifest.json"},
		{"LogFile", root.LogFile("agent-live"), "book/logs/agent-live"},
		{"AgentWorkDir", root.AgentWorkDir(), "book/.novelgen"},
		{"AgentPatchDir", root.AgentPatchDir(), "book/.novelgen/agent-patches"},
		{"CheckpointsDir", root.CheckpointsDir(store.ScopeCompose), "book/story/compose/checkpoints"},
		{"CheckpointsDirChapters", root.CheckpointsDir(store.ScopeChapters), "book/chapters/checkpoints"},
		{"BackupsDir", root.BackupsDir(store.ScopeCompose), "book/story/compose/backups"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := filepath.ToSlash(tc.got); got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestRPGPhaseFiles pins the phased DSL fragment names and their merge order.
func TestRPGPhaseFiles(t *testing.T) {
	t.Parallel()

	got := store.RPGPhaseFiles()
	want := []string{"00_setup.rpg", "01_outline.rpg", "02_craft.rpg", "03_systems.rpg", "04_chapters.rpg"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("RPGPhaseFiles = %v, want %v", got, want)
	}
	if store.RPGPhaseChaptersFile != "04_chapters.rpg" {
		t.Fatalf("RPGPhaseChaptersFile = %q, want 04_chapters.rpg", store.RPGPhaseChaptersFile)
	}
}

// TestAtAcceptsAnAlreadyResolvedDirectory pins the constructor the callers that
// hold a directory string use: it must not absolutize or require the directory
// to exist.
func TestAtAcceptsAnAlreadyResolvedDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "does", "not", "exist", "yet")
	root := store.At(dir)
	if filepath.ToSlash(root.Dir()) != filepath.ToSlash(filepath.Clean(dir)) {
		t.Fatalf("At(%q).Dir() = %q", dir, root.Dir())
	}
	if got := filepath.ToSlash(root.OutlineJSON()); got != filepath.ToSlash(filepath.Join(dir, "story", "compose", "outline.json")) {
		t.Fatalf("OutlineJSON = %q", got)
	}
}

// TestRelPathReportsProjectRelativeArtifacts pins the form agent tool responses
// report derived artifacts in.
func TestRelPathReportsProjectRelativeArtifacts(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	if got := root.RelPath(root.RPGChaptersDSL()); got != "story/rpg/04_chapters.rpg" {
		t.Fatalf("RelPath(RPGChaptersDSL) = %q", got)
	}
	if got := root.RelPath(root.RPGChapterBatchCacheDir()); got != "story/rpg/cache/chapter_batches" {
		t.Fatalf("RelPath(RPGChapterBatchCacheDir) = %q", got)
	}
}

func TestChapterCandidatesCoverLegacyNames(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	got := []string{}
	for _, path := range root.ChapterCandidates("chap_1_2_3") {
		got = append(got, filepath.ToSlash(path))
	}
	want := []string{
		"book/chapters/chapter-chap_1_2_3.md",
		"book/chapters/chapter-3.md",
		"book/chapters/chap_1_2_3.md",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ChapterCandidates = %v, want %v", got, want)
	}

	// A structured ID has no distinct numeric form, so the list collapses.
	full := root.ChapterCandidates("P1-V1-C1")
	if len(full) != 2 {
		t.Fatalf("structured id candidates = %v, want 2 entries", full)
	}
}

func TestLocateWalksUpAndReportsMissingProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProjectFile(t, dir, "novel.json", `{"name":"x"}`)
	deep := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}

	found, err := store.Locate(deep)
	if err != nil {
		t.Fatalf("Locate(%q): %v", deep, err)
	}
	if found.Dir() != filepath.Clean(dir) {
		t.Fatalf("Locate = %q, want %q", found.Dir(), dir)
	}
	if !store.IsProjectDir(dir) {
		t.Fatalf("IsProjectDir(%q) = false", dir)
	}

	empty := t.TempDir()
	if _, err := store.Locate(empty); !errors.Is(err, store.ErrNoProject) {
		t.Fatalf("Locate on empty dir = %v, want ErrNoProject", err)
	}
	if store.IsProjectDir(empty) {
		t.Fatalf("IsProjectDir(%q) = true", empty)
	}
}

func TestWritesAreAtomicAndLeaveNoTempFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	target := root.OutlineJSON()
	if err := root.WriteText(target, "first"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := root.WriteText(target, "second"); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "second" {
		t.Fatalf("content = %q, want %q", data, "second")
	}

	entries, err := os.ReadDir(root.ComposeDir())
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", entry.Name())
		}
	}
	if len(entries) != 1 || entries[0].Name() != "outline.json" {
		t.Fatalf("compose dir = %v, want only outline.json", entries)
	}
}

func TestWriteReplacesExistingFileThroughMemFS(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	if err := root.WriteText(root.OutlineJSON(), "one"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := root.WriteText(root.OutlineJSON(), "two"); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	fs := root.FS().(*store.MemFS)
	names := fs.Names()
	if len(names) != 1 || names[0] != "/book/story/compose/outline.json" {
		t.Fatalf("memfs names = %v, want only the outline", names)
	}
	if data, ok := fs.File(root.OutlineJSON()); !ok || string(data) != "two" {
		t.Fatalf("memfs file = %q, %v", data, ok)
	}
}

func TestTypedArtifactsRoundTrip(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()

	config := &models.ProjectConfig{Name: "Demo", Language: "zh"}
	if err := root.SaveConfig(config); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	loadedConfig, err := root.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loadedConfig.Name != "Demo" || !root.HasConfig() {
		t.Fatalf("loaded config = %+v", loadedConfig)
	}

	if err := root.SaveSetup(&models.StorySetup{ProjectName: "Demo", Premise: "  spaced  "}); err != nil {
		t.Fatalf("SaveSetup: %v", err)
	}
	loadedSetup, err := root.LoadSetup()
	if err != nil {
		t.Fatalf("LoadSetup: %v", err)
	}
	if loadedSetup.Premise != "spaced" {
		t.Fatalf("setup premise = %q, want normalized %q", loadedSetup.Premise, "spaced")
	}

	outline := &models.Outline{
		Parts: []models.Part{{
			Volumes: []models.Volume{{
				Chapters: []models.Chapter{{Title: "One Chapter", Characters: []string{"  Lin  ", "Lin", ""}}},
			}},
		}},
	}
	if err := root.SaveOutline(outline); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	loadedOutline, err := root.LoadOutline()
	if err != nil {
		t.Fatalf("LoadOutline: %v", err)
	}
	chapter := loadedOutline.Parts[0].Volumes[0].Chapters[0]
	if chapter.ID == "" {
		t.Fatal("outline chapter lost its normalized identity")
	}
	if len(chapter.Characters) != 1 || chapter.Characters[0] != "Lin" {
		t.Fatalf("chapter characters = %v, want trimmed and de-duplicated", chapter.Characters)
	}

	recap := &models.ChapterRecap{ChapterID: "P1-V1-C1", Title: "Recap"}
	if err := root.SaveRecap(recap); err != nil {
		t.Fatalf("SaveRecap: %v", err)
	}
	loadedRecap, err := root.LoadRecap("P1-V1-C1")
	if err != nil {
		t.Fatalf("LoadRecap: %v", err)
	}
	if loadedRecap.Title != "Recap" {
		t.Fatalf("recap = %+v", loadedRecap)
	}
}

func TestLoadElementsToleratesMissingFiles(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	characters, locations, items, organizations, err := root.LoadElements()
	if err != nil {
		t.Fatalf("LoadElements on empty project: %v", err)
	}
	if len(characters)+len(locations)+len(items)+len(organizations) != 0 {
		t.Fatal("empty project must yield empty element maps")
	}

	payload, err := json.Marshal(map[string]any{
		"Hero": map[string]any{
			"name":          "Hero",
			"role_in_story": "lead",
			"power_level":   -5,
			"dsl_tags":      []string{"scout", "scout", ""},
		},
	})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := root.Write(root.CraftFile(store.CraftCharacters), payload, store.FileMode); err != nil {
		t.Fatalf("write craft: %v", err)
	}
	characters, _, _, _, err = root.LoadElements()
	if err != nil {
		t.Fatalf("LoadElements: %v", err)
	}
	hero := characters["Hero"]
	if hero == nil {
		t.Fatal("character Hero missing after load")
	}
	if hero.PowerLevel != 0 || len(hero.DSLTags) != 1 || hero.DSLTags[0] != "scout" {
		t.Fatalf("character = %+v, want clamped power level and compacted tags", hero)
	}
}

func TestChapterAndDraftContentFallbacks(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	if content, path := root.LoadChapterContent("P1-V1-C1"); content != "" || path != "" {
		t.Fatalf("missing chapter = %q at %q, want empty", content, path)
	}

	if err := root.SaveChapterContent("P1-V1-C1", "# Canonical"); err != nil {
		t.Fatalf("SaveChapterContent: %v", err)
	}
	content, path := root.LoadChapterContent("P1-V1-C1")
	if content != "# Canonical" || filepath.Base(path) != "chapter-P1-V1-C1.md" {
		t.Fatalf("chapter = %q at %q", content, path)
	}

	// Legacy numeric name wins over the bare-id name.
	root.FS().(*store.MemFS).SetFile(root.ChapterFile("3"), []byte("# Numeric"))
	root.FS().(*store.MemFS).SetFile(filepath.Join(root.ChaptersDir(), "chap_1_2_3.md"), []byte("# Bare"))
	if content, path := root.LoadChapterContent("chap_1_2_3"); content != "# Numeric" {
		t.Fatalf("legacy chapter = %q at %q, want numeric fallback", content, path)
	}

	if err := root.SaveDraftContent("P1-V1-C1", "# Draft"); err != nil {
		t.Fatalf("SaveDraftContent: %v", err)
	}
	if got := root.LoadDraftContent("P1-V1-C1"); got != "# Draft" {
		t.Fatalf("draft = %q", got)
	}
	if drafts := root.LoadAllDrafts(); len(drafts) != 1 || drafts["P1-V1-C1"] != "# Draft" {
		t.Fatalf("drafts = %v", drafts)
	}
}

func TestSnapshots(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	checkpoint, err := root.SaveCheckpoint(store.ScopeCompose, "outline", ".json", []byte("{}"))
	if err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	if dir := filepath.ToSlash(filepath.Dir(checkpoint)); dir != "book/story/compose/checkpoints" {
		t.Fatalf("checkpoint dir = %q", dir)
	}
	if base := filepath.Base(checkpoint); !strings.HasPrefix(base, "outline_") || !strings.HasSuffix(base, ".json") {
		t.Fatalf("checkpoint name = %q", base)
	}

	if path, err := root.BackupFile(store.ScopeCompose, root.OutlineJSON(), "outline"); err != nil || path != "" {
		t.Fatalf("BackupFile on missing source = %q, %v; want empty path without error", path, err)
	}
	if err := root.WriteText(root.OutlineJSON(), "outline body"); err != nil {
		t.Fatalf("seed outline: %v", err)
	}
	backup, err := root.BackupFile(store.ScopeCompose, root.OutlineJSON(), "outline")
	if err != nil {
		t.Fatalf("BackupFile: %v", err)
	}
	if got := filepath.ToSlash(backup); !strings.HasPrefix(got, "book/story/compose/backups/outline_") || !strings.HasSuffix(got, ".json") {
		t.Fatalf("backup = %q", got)
	}
	if data := root.FS().(*store.MemFS); data != nil {
		if content, ok := data.File(backup); !ok || string(content) != "outline body" {
			t.Fatalf("backup content = %q, %v", content, ok)
		}
	}
}

func TestRootContextAndDefault(t *testing.T) {
	// Not parallel: it installs the process-wide default root.
	explicit := store.MemRoot()
	ctx := store.WithRoot(context.Background(), explicit)
	got, ok := store.FromContext(ctx)
	if !ok || got != explicit {
		t.Fatalf("FromContext = %v, %v", got, ok)
	}
	if _, ok := store.FromContext(context.Background()); ok {
		t.Fatal("bare context must not carry a root")
	}

	store.SetDefault(explicit)
	defer store.ResetDefault()
	if resolved, err := store.Default(); err != nil || resolved != explicit {
		t.Fatalf("Default = %v, %v", resolved, err)
	}
}

func TestReadJSONToleratesBOM(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	path := root.SetupJSON()
	if err := root.Write(path, []byte("\xef\xbb\xbf{\"project_name\":\"Demo\"}"), store.FileMode); err != nil {
		t.Fatalf("write: %v", err)
	}
	setup, err := root.LoadSetup()
	if err != nil {
		t.Fatalf("LoadSetup with BOM: %v", err)
	}
	if setup.ProjectName != "Demo" {
		t.Fatalf("setup = %+v", setup)
	}
}

func TestWriteJSONMergingKeepsExistingKeys(t *testing.T) {
	t.Parallel()

	root := store.MemRoot()
	path := root.CraftFile(store.CraftCharacters)
	if err := root.Write(path, []byte(`{"Hero":{"name":"Hero","role":"lead"},"Villain":{"name":"Villain"}}`), store.FileMode); err != nil {
		t.Fatalf("seed: %v", err)
	}
	patch := map[string]map[string]any{"Hero": {"role": "support"}}
	if err := root.WriteJSONMerging(path, patch); err != nil {
		t.Fatalf("WriteJSONMerging: %v", err)
	}
	merged, err := root.LoadCraftRaw(store.CraftCharacters)
	if err != nil {
		t.Fatalf("LoadCraftRaw: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("merged = %v, want untouched sibling preserved", merged)
	}
	if merged["Hero"]["role"] != "support" {
		t.Fatalf("hero = %v", merged["Hero"])
	}
}
