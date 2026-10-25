package toolplane

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/memory"
	"novelgen/internal/platform/store"
)

func TestNarrativeMemoryContextAndRefreshIntegration(t *testing.T) {
	root := narrativeMemoryFixture(t)
	originalPath := memoryConfigPathForRoot
	memoryConfigPathForRoot = func(*store.Root) string { return "" }
	defer func() { memoryConfigPathForRoot = originalPath }()

	ctx := ToolProjectContext{Root: root}
	if err := ctx.Load("context"); err != nil {
		t.Fatal(err)
	}
	resp := QueryContextWithOptions(ctx, "narrative-memory", "P1-V1-C2", "", QueryContextOptions{
		Intent: "chapter-write", MaxChars: 1200,
	})
	if !resp.OK || resp.Count == 0 {
		t.Fatalf("narrative memory query = %#v", resp)
	}
	result, ok := resp.Results.(memory.ContextResult)
	if !ok {
		t.Fatalf("results type = %T", resp.Results)
	}
	if len(result.Retrieved) == 0 || result.Retrieved[0].ChapterID != "P1-V1-C1" {
		t.Fatalf("unexpected memory result: %#v", result)
	}
	if result.Meta.Status != "ephemeral" {
		t.Fatalf("status = %q, want ephemeral", result.Meta.Status)
	}

	ApplyToolView(&resp, "index")
	indexView, ok := resp.Results.(map[string]interface{})
	if !ok || indexView["intent"] != "chapter-write" {
		t.Fatalf("index view = %#v", resp.Results)
	}

	originalFlags := ToolRefreshFlags
	defer func() { ToolRefreshFlags = originalFlags }()
	cmd := &cobra.Command{}
	cmd.SetContext(store.WithRoot(context.Background(), root))
	var out strings.Builder
	cmd.SetOut(&out)
	if err := RunToolRefresh(cmd, []string{"narrative-memory"}); err != nil {
		t.Fatal(err)
	}
	var refresh memory.RefreshResult
	if err := json.Unmarshal([]byte(out.String()), &refresh); err != nil {
		t.Fatalf("decode refresh: %v\n%s", err, out.String())
	}
	if refresh.Status != "rebuilt" || refresh.Stats.Scenes == 0 {
		t.Fatalf("unexpected refresh result: %#v", refresh)
	}
	if !root.Exists(root.NarrativeMemoryIndexFile()) {
		t.Fatalf("narrative memory index was not written")
	}
	if got := memory.Inspect(root, ""); got.Status != "fresh" {
		t.Fatalf("inspect after refresh = %#v", got)
	}
}

func TestNarrativeMemoryQueryCLIUsesIntentBudgetAndView(t *testing.T) {
	root := narrativeMemoryFixture(t)
	originalPath := memoryConfigPathForRoot
	originalFlags := ToolQueryFlags
	memoryConfigPathForRoot = func(*store.Root) string { return "" }
	defer func() {
		memoryConfigPathForRoot = originalPath
		ToolQueryFlags = originalFlags
	}()
	ToolQueryFlags = struct {
		Type           string
		ID             string
		Name           string
		EntityType     string
		ChapterID      string
		VolumeID       string
		View           string
		Fields         string
		IncludeContent bool
		Limit          int
		Genre          string
		Level          string
		Mode           string
		Seed           int64
		Count          int
		Select         int
		Source         string
		Category       string
		Intent         string
		MaxChars       int
	}{
		Type: "narrative-memory", ID: "P1-V1-C2", Name: "blue key",
		View: "brief", Limit: 2, Intent: "continuity", MaxChars: 900,
	}
	cmd := &cobra.Command{}
	cmd.SetContext(store.WithRoot(context.Background(), root))
	var out strings.Builder
	cmd.SetOut(&out)
	if err := runToolQuery(cmd, []string{"context"}); err != nil {
		t.Fatal(err)
	}
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &response); err != nil {
		t.Fatalf("decode query response: %v\n%s", err, out.String())
	}
	if response["ok"] != true || response["count"] == float64(0) {
		t.Fatalf("unexpected query response: %s", out.String())
	}
	query := response["query"].(map[string]interface{})
	if query["intent"] != "continuity" || query["max_chars"] != "900" {
		t.Fatalf("query metadata missing intent/budget: %#v", query)
	}
}

func TestChapterWriteContextIncludesNarrativeMemory(t *testing.T) {
	root := narrativeMemoryFixture(t)
	originalPath := memoryConfigPathForRoot
	memoryConfigPathForRoot = func(*store.Root) string { return "" }
	defer func() { memoryConfigPathForRoot = originalPath }()

	ctx := ToolProjectContext{Root: root}
	if err := ctx.Load("context"); err != nil {
		t.Fatal(err)
	}
	resp := QueryContext(ctx, "chapter-write", "P1-V1-C2", "")
	if !resp.OK {
		t.Fatalf("chapter-write context = %#v", resp)
	}
	ApplyToolFields(&resp, "path,navigation,narrative_memory,stats,warnings")
	writeContext, ok := resp.Results.(map[string]interface{})
	if !ok || writeContext["narrative_memory"] == nil {
		t.Fatalf("narrative memory missing from chapter-write context: %#v", resp.Results)
	}
}

func TestCanonicalChapterWriteMarksMemoryStaleWithoutRefreshFlag(t *testing.T) {
	root := narrativeMemoryFixture(t)
	originalPath := memoryConfigPathForRoot
	memoryConfigPathForRoot = func(*store.Root) string { return "" }
	defer func() { memoryConfigPathForRoot = originalPath }()
	if _, err := memory.Refresh(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMutationService(root).ReplaceChapter(
		context.Background(),
		"P1-V1-C1",
		"# Opening\n\nLin finds a different key.",
		MutationOptions{Validate: false, Source: "test"},
	); err != nil {
		t.Fatal(err)
	}
	if !root.Exists(root.NarrativeMemoryDirtyFile()) {
		t.Fatalf("canonical chapter write did not mark narrative memory stale")
	}
	if got := memory.Inspect(root, ""); got.Status != "stale" {
		t.Fatalf("inspect after chapter write = %#v, want stale", got)
	}
}

func narrativeMemoryFixture(t *testing.T) *store.Root {
	t.Helper()
	root := store.At(t.TempDir())
	if err := root.SaveConfig(&models.ProjectConfig{Name: "Memory Tool", Language: "en"}); err != nil {
		t.Fatal(err)
	}
	if err := root.SaveSetup(&models.StorySetup{
		ProjectName: "Memory Tool",
		Premise:     "A sealed door remembers.",
		Theme:       "Memory",
		Storylines:  []models.Storyline{{Name: "Door", Importance: 9}},
	}); err != nil {
		t.Fatal(err)
	}
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1", Title: "Part", Volumes: []models.Volume{{
			ID: "P1-V1", Title: "Volume", Chapters: []models.Chapter{
				{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin finds the blue key.", Characters: []string{"Lin"}, Location: "Mine"},
				{ID: "P1-V1-C2", Title: "Return", Summary: "Lin returns to the sealed door.", Characters: []string{"Lin"}, Location: "Mine"},
			},
		}},
	}}}
	if err := root.SaveOutline(outline); err != nil {
		t.Fatal(err)
	}
	if err := root.SaveChapterContent("P1-V1-C1", "# Opening\n\nLin finds the blue key.\n\nA voice whispers behind the sealed door.\n\nLin hides the key.\n\nLin marks the door."); err != nil {
		t.Fatal(err)
	}
	return root
}
