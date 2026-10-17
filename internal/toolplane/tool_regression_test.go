package toolplane

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"novelgen/internal/models"
	"novelgen/internal/store"
	"novelgen/internal/testfixtures"
)

func regressionOutline() *models.Outline {
	return &models.Outline{Parts: []models.Part{{
		ID:    "P1",
		Title: "Part One",
		Volumes: []models.Volume{{
			ID:    "P1-V1",
			Title: "Volume One",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Title:   "Opening",
				Summary: "Lin enters the hall.",
				Events: []models.Event{{
					Actor:      "Lin",
					Action:     "discover",
					Target:     "Door",
					TargetType: "location",
				}},
			}},
		}},
	}}}
}

func regressionRoot(t *testing.T) *store.Root {
	t.Helper()
	root := store.At(t.TempDir())
	if err := root.SaveConfig(&models.ProjectConfig{Name: "Regression"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if err := root.SaveSetup(testfixtures.ValidLongFormSetup()); err != nil {
		t.Fatalf("SaveSetup: %v", err)
	}
	if err := root.SaveOutline(regressionOutline()); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	return root
}

func regressionCommand(root *store.Root) (*cobra.Command, *strings.Builder) {
	cmd := &cobra.Command{}
	cmd.SetContext(store.WithRoot(context.Background(), root))
	var out strings.Builder
	cmd.SetOut(&out)
	return cmd, &out
}

func TestChapterQueryKeepsRequestedContentInBriefView(t *testing.T) {
	root := regressionRoot(t)
	const content = "# Opening\n\nSaved prose for the repair loop.\n"
	if err := root.SaveChapterContent("P1-V1-C1", content); err != nil {
		t.Fatalf("SaveChapterContent: %v", err)
	}

	resp := queryChapter(ToolProjectContext{Root: root, Outline: regressionOutline()}, "P1-V1-C1", true)
	ApplyToolView(&resp, "brief")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Saved prose for the repair loop.") {
		t.Fatalf("brief chapter result dropped --content: %s", data)
	}
}

func TestSetupPatchMatchesCoreCastByNameAndTimelineByYear(t *testing.T) {
	setup := models.StorySetup{
		CoreCast: []models.CoreCastSeed{{
			ID:   "cast_1",
			Name: "Lin",
			Role: "protagonist",
		}},
		WorldTimeline: []models.WorldTimelineEntry{{
			Year:   "100",
			Event:  "War",
			Impact: "old",
		}},
	}

	merged, _, err := ApplySetupPatchObject(setup, []byte(`{"core_cast":[{"name":"Lin","role":"mentor"}]}`))
	if err != nil {
		t.Fatalf("core cast patch: %v", err)
	}
	if len(merged.CoreCast) != 1 || merged.CoreCast[0].Role != "mentor" || merged.CoreCast[0].ID != "cast_1" {
		t.Fatalf("core cast should match by name without duplicating: %#v", merged.CoreCast)
	}

	merged, _, err = ApplySetupPatchObject(merged, []byte(`{"world_timeline":[{"year":"100","impact":"new"}]}`))
	if err != nil {
		t.Fatalf("timeline patch: %v", err)
	}
	if len(merged.WorldTimeline) != 1 || merged.WorldTimeline[0].Impact != "new" || merged.WorldTimeline[0].Event != "War" {
		t.Fatalf("timeline should update the year match without duplicating: %#v", merged.WorldTimeline)
	}
}

func TestSetupPatchRejectsUnknownFields(t *testing.T) {
	_, _, err := ApplySetupPatchObject(models.StorySetup{}, []byte(`{"premizzes":"typo"}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown-field rejection, got %v", err)
	}
	_, _, err = ApplySetupPatchObject(models.StorySetup{}, []byte(`{"storylines":[{"description":"missing name"}]}`))
	if err == nil || !strings.Contains(err.Error(), "stable key") {
		t.Fatalf("expected missing-key rejection, got %v", err)
	}
}

func TestEmptyPatchIsRejected(t *testing.T) {
	if err := ValidateToolPatchBytes([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "at least one field") {
		t.Fatalf("expected empty-patch rejection, got %v", err)
	}
}

func TestOutlineAndVolumePatchesRejectUnknownFields(t *testing.T) {
	outline := regressionOutline()
	if _, err := ApplyChapterPatch(outline, "P1-V1-C1", []byte(`{"sumary":"typo"}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected chapter unknown-field rejection, got %v", err)
	}
	if _, err := ApplyVolumePatch(outline, "P1-V1", []byte(`{"sumary":"typo"}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected volume unknown-field rejection, got %v", err)
	}
}

func TestRecapPatchRejectsUnknownFieldsAndDoesNotOverwriteUnreadableFile(t *testing.T) {
	root := regressionRoot(t)
	cmd, _ := regressionCommand(root)
	const corrupt = "{not-json"
	if err := root.Write(root.RecapFile("P1-V1-C1"), []byte(corrupt), store.FileMode); err != nil {
		t.Fatalf("write corrupt recap: %v", err)
	}
	if err := RunToolRecapPatch(cmd, root, "P1-V1-C1", []byte(`{"locaton":"typo"}`), false, nil); err == nil {
		t.Fatal("expected recap unknown-field rejection")
	}
	got, err := root.Read(root.RecapFile("P1-V1-C1"))
	if err != nil {
		t.Fatalf("read recap: %v", err)
	}
	if string(got) != corrupt {
		t.Fatalf("unreadable recap was overwritten: %q", got)
	}

	if err := RunToolRecapPatch(cmd, root, "P1-V1-C1", []byte(`{"location":"Hall"}`), false, nil); err == nil || !strings.Contains(err.Error(), "failed to read recap") {
		t.Fatalf("expected unreadable-recap error, got %v", err)
	}
}

func TestChapterQueryEventsTreatsIDAsChapterID(t *testing.T) {
	ctx := ToolProjectContext{Outline: regressionOutline()}
	resp := queryChapterSection(ctx, "events", "P1-V1-C1", "", "", "", "", false)
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("events response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	if _, ok := resp.Results.([]EventHit); !ok {
		t.Fatalf("events results type = %T, want []EventHit", resp.Results)
	}
}

func TestCheckFilterNoMatchSuggestsWideningInsteadOfFinalJSON(t *testing.T) {
	result := ToolCheckResult{
		Kind:    "quality",
		Target:  "outline",
		Scope:   "all",
		Summary: ToolCheckSummary{Total: 2},
		Meta: map[string]interface{}{
			"issue_filter": map[string]interface{}{"no_matches": true},
		},
	}
	actions := toolCheckNextActions(result)
	if len(actions) == 0 || actions[0].Action != "fix_check_filter" {
		t.Fatalf("filtered-empty result should route to filter repair, got %#v", actions)
	}
}

func TestApplyToolLimitWarnsForObjectsAndUpdatesCountForLists(t *testing.T) {
	objectResp := ToolResponse{OK: true, Count: 1, Results: map[string]interface{}{"parts": []int{1}}}
	applyToolLimit(&objectResp, 5)
	if len(objectResp.Warnings) == 0 || !strings.Contains(objectResp.Warnings[0], "object-shaped") {
		t.Fatalf("object-shaped limit should warn, got %#v", objectResp.Warnings)
	}

	listResp := ToolResponse{OK: true, Count: 2, Results: []map[string]interface{}{{"id": "1"}, {"id": "2"}}}
	applyToolLimit(&listResp, 1)
	if listResp.Count != 1 || listResp.Meta["total_count"] != 2 {
		t.Fatalf("list limit = count %d meta %#v", listResp.Count, listResp.Meta)
	}
}

func TestApplyToolFieldsKeepsResultsWhenNoFieldMatches(t *testing.T) {
	resp := ToolResponse{OK: true, Results: map[string]interface{}{"theme": "keep me"}}
	ApplyToolFields(&resp, "theem")
	results, ok := resp.Results.(map[string]interface{})
	if !ok || results["theme"] != "keep me" {
		t.Fatalf("no-match --fields should preserve results, got %#v", resp.Results)
	}
}

func TestValidateToolViewRejectsTypo(t *testing.T) {
	if err := ValidateToolView("fulll"); err == nil || !strings.Contains(err.Error(), "index") {
		t.Fatalf("expected view typo rejection with valid values, got %v", err)
	}
}

func TestQueryResponseEchoesCanonicalType(t *testing.T) {
	root := regressionRoot(t)
	original := ToolQueryFlags
	defer func() { ToolQueryFlags = original }()
	ToolQueryFlags.Type = "chapters"
	ToolQueryFlags.View = "brief"
	ToolQueryFlags.ID = ""
	ToolQueryFlags.Name = ""
	ToolQueryFlags.ChapterID = ""
	ToolQueryFlags.VolumeID = ""
	ToolQueryFlags.EntityType = ""
	ToolQueryFlags.Fields = ""
	ToolQueryFlags.IncludeContent = false
	ToolQueryFlags.Limit = 0
	cmd, out := regressionCommand(root)
	if err := runToolQuery(cmd, []string{"outline"}); err != nil {
		t.Fatalf("runToolQuery: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	query, _ := decoded["query"].(map[string]interface{})
	if query["type"] != "chapter" {
		t.Fatalf("query.type = %#v, want canonical chapter", query["type"])
	}
}

func TestQueryWarnsWhenNameFilterIsIgnored(t *testing.T) {
	root := regressionRoot(t)
	original := ToolQueryFlags
	defer func() { ToolQueryFlags = original }()
	ToolQueryFlags.Type = ""
	ToolQueryFlags.View = "brief"
	ToolQueryFlags.ID = ""
	ToolQueryFlags.Name = "Lin"
	ToolQueryFlags.ChapterID = ""
	ToolQueryFlags.VolumeID = ""
	ToolQueryFlags.EntityType = ""
	ToolQueryFlags.Fields = ""
	ToolQueryFlags.IncludeContent = false
	ToolQueryFlags.Limit = 0
	cmd, out := regressionCommand(root)
	if err := runToolQuery(cmd, []string{"story-setup"}); err != nil {
		t.Fatalf("runToolQuery: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	warnings, _ := decoded["warnings"].([]interface{})
	joined := ""
	for _, warning := range warnings {
		joined += " " + warning.(string)
	}
	if !strings.Contains(joined, "--name is ignored") {
		t.Fatalf("expected ignored --name warning, got %v", warnings)
	}
}

func TestOutlineCheckReportsProjectRootPath(t *testing.T) {
	root := regressionRoot(t)
	cmd, out := regressionCommand(root)
	original := ToolCheckFlags
	defer func() { ToolCheckFlags = original }()
	ToolCheckFlags.Target = "outline"
	ToolCheckFlags.Scope = "all"
	ToolCheckFlags.ID = ""
	ToolCheckFlags.MinPriority = ""
	ToolCheckFlags.Category = ""
	ToolCheckFlags.MaxIssues = 0
	ToolCheckFlags.TargetWords = 0

	if err := runToolCheck(cmd, []string{"quality"}); err != nil {
		t.Fatalf("runToolCheck: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("decode check JSON: %v\n%s", err, out.String())
	}
	meta, _ := decoded["meta"].(map[string]interface{})
	if got, _ := meta["project_root"].(string); got != root.Dir() {
		t.Fatalf("project_root = %#v, want %q", meta["project_root"], root.Dir())
	}
}

func TestCraftCheckInfersScopeAndCanCheckAllKinds(t *testing.T) {
	root := regressionRoot(t)
	if err := root.WriteJSON(root.CraftFile(store.CraftCharacters), map[string]map[string]interface{}{
		"Lin": {"name": "Lin", "role_in_story": "protagonist"},
	}); err != nil {
		t.Fatalf("write characters: %v", err)
	}
	if err := root.WriteJSON(root.CraftFile(store.CraftItems), map[string]map[string]interface{}{
		"Sword": {"name": "Sword"},
	}); err != nil {
		t.Fatalf("write items: %v", err)
	}
	original := ToolCheckFlags
	defer func() { ToolCheckFlags = original }()
	ToolCheckFlags.Target = "craft"
	ToolCheckFlags.MinPriority = ""
	ToolCheckFlags.Category = ""
	ToolCheckFlags.MaxIssues = 0
	ToolCheckFlags.TargetWords = 0

	ToolCheckFlags.Scope = ""
	ToolCheckFlags.ID = "Lin"
	cmd, out := regressionCommand(root)
	if err := runToolCheck(cmd, []string{"schema"}); err != nil {
		t.Fatalf("craft check by id: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["scope"] != "character" || decoded["id"] != "Lin" {
		t.Fatalf("inferred craft scope = %#v", decoded)
	}

	ToolCheckFlags.Scope = ""
	ToolCheckFlags.ID = ""
	cmd, out = regressionCommand(root)
	if err := runToolCheck(cmd, []string{"schema"}); err != nil {
		t.Fatalf("craft check all: %v", err)
	}
	decoded = nil
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["scope"] != "all" {
		t.Fatalf("all craft check scope = %#v", decoded)
	}
}

func TestRecapCheckDefaultsToChapterScope(t *testing.T) {
	root := regressionRoot(t)
	if err := root.SaveRecap(&models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "Opening",
		Location:        "Hall",
		Present:         []string{"Lin"},
		LastLine:        "The door opens.",
		NextOpeningHint: "Then the light returns.",
	}); err != nil {
		t.Fatalf("SaveRecap: %v", err)
	}
	original := ToolCheckFlags
	defer func() { ToolCheckFlags = original }()
	ToolCheckFlags.Target = "recap"
	ToolCheckFlags.Scope = ""
	ToolCheckFlags.ID = "P1-V1-C1"
	ToolCheckFlags.MinPriority = ""
	ToolCheckFlags.Category = ""
	ToolCheckFlags.MaxIssues = 0
	ToolCheckFlags.TargetWords = 0
	cmd, out := regressionCommand(root)
	if err := runToolCheck(cmd, []string{"quality"}); err != nil {
		t.Fatalf("recap check: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["scope"] != "chapter" {
		t.Fatalf("recap default scope = %#v", decoded["scope"])
	}
}

func TestSetupCheckWarnsAboutIgnoredScopingFlags(t *testing.T) {
	root := regressionRoot(t)
	original := ToolCheckFlags
	defer func() { ToolCheckFlags = original }()
	ToolCheckFlags.Target = "setup"
	ToolCheckFlags.Scope = "volume"
	ToolCheckFlags.ID = "not-used"
	ToolCheckFlags.TargetWords = 800
	ToolCheckFlags.MinPriority = ""
	ToolCheckFlags.Category = ""
	ToolCheckFlags.MaxIssues = 0
	cmd, out := regressionCommand(root)
	if err := runToolCheck(cmd, []string{"quality"}); err != nil {
		t.Fatalf("setup check: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	warnings, _ := decoded["warnings"].([]interface{})
	joined := ""
	for _, warning := range warnings {
		joined += " " + warning.(string)
	}
	for _, want := range []string{"--scope", "--id", "--target-words"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("setup check warnings missing %q: %v", want, warnings)
		}
	}
}

func TestPatchRejectsConflictingSources(t *testing.T) {
	original := ToolPatchFlags
	defer func() { ToolPatchFlags = original }()
	ToolPatchFlags.Task = "outline-global-repair:structure"
	ToolPatchFlags.PatchJSON = `{"summary":"x"}`
	ToolPatchFlags.PatchBuffer = ""
	ToolPatchFlags.Target = ""
	ToolPatchFlags.ID = "P1-V1"
	cmd := &cobra.Command{}
	err := RunToolPatch(cmd, []string{"outline"})
	if err == nil || !strings.Contains(err.Error(), "only one patch source") {
		t.Fatalf("expected conflicting-source error, got %v", err)
	}
}

func TestChapterPatchMetaProjectRootIsPath(t *testing.T) {
	root := regressionRoot(t)
	cmd, out := regressionCommand(root)
	original := ToolPatchFlags
	defer func() { ToolPatchFlags = original }()
	ToolPatchFlags.TargetWords = 0
	ToolPatchFlags.Target = ""
	ToolPatchFlags.PatchJSON = ""
	ToolPatchFlags.PatchBuffer = ""
	ToolPatchFlags.Task = ""
	ToolPatchFlags.Apply = false
	ToolPatchFlags.DryRun = false
	ToolPatchFlags.RefreshDerived = false
	if err := runToolChapterPatch(cmd, root, "P1-V1-C1", []byte(`{"content":"# Opening\n\nShort."}`), false, nil); err != nil {
		t.Fatalf("chapter dry-run: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	meta, _ := decoded["meta"].(map[string]interface{})
	if got, _ := meta["project_root"].(string); got != root.Dir() {
		t.Fatalf("chapter patch project_root = %#v, want %q", meta["project_root"], root.Dir())
	}
}
