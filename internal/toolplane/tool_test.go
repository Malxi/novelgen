package toolplane_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	models "novelgen/internal/contract/story"
	"novelgen/internal/toolplane"

	"github.com/spf13/cobra"
)

func TestToolQueryOutlineRefsFindsCharacterItemAndLocation(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID:    "part_1",
		Title: "Part One",
		Volumes: []models.Volume{{
			ID:    "vol_1",
			Title: "Volume One",
			Chapters: []models.Chapter{{
				ID:          "chap_001",
				Title:       "Opening",
				Characters:  []string{"Lin"},
				Location:    "Mine",
				StateAnchor: models.StateAnchor{KeyItems: []string{"Star Core"}},
				Events: []models.Event{{
					Actor:      "Lin",
					Action:     models.ActionAcquire,
					Target:     "Star Core",
					TargetType: models.TargetTypeItem,
				}},
			}},
		}},
	}}}

	charResp := toolplane.QueryOutlineRefs(outline, "character", "Lin", true)
	if charResp.Count != 1 {
		t.Fatalf("character refs count = %d, want 1", charResp.Count)
	}
	itemResp := toolplane.QueryOutlineRefs(outline, "item", "Star Core", true)
	if itemResp.Count != 1 {
		t.Fatalf("item refs count = %d, want 1", itemResp.Count)
	}
	locResp := toolplane.QueryOutlineRefs(outline, "location", "Mine", true)
	if locResp.Count != 1 {
		t.Fatalf("location refs count = %d, want 1", locResp.Count)
	}
}

func TestFieldsRequestScenes(t *testing.T) {
	cases := map[string]bool{
		"":                     false,
		"summary":              false,
		"summary,opening_beat": false,
		"scenes":               true,
		"scenes.beats":         true,
		"scenes,summary":       true,
	}
	for fields, want := range cases {
		if got := toolplane.FieldsRequestScenes(fields); got != want {
			t.Fatalf("toolplane.FieldsRequestScenes(%q) = %v, want %v", fields, got, want)
		}
	}
}

func TestToolQueryChapterFieldsScenesBypassesBriefTrim(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "P1",
		Volumes: []models.Volume{{
			ID: "P1-V1",
			Chapters: []models.Chapter{{
				ID:      "P1-V1-C1",
				Summary: "chapter summary",
				Scenes: []models.OutlineScene{{
					Order: 1,
					POV:   "Lin",
					Goal:  "scene goal",
					Beats: []string{"scene beat one"},
				}},
			}},
		}},
	}}}

	// Plain brief view must strip scenes so ordinary queries stay compact.
	brief := toolplane.QueryOutlineNode(outline, "chapter", "P1-V1-C1")
	brief.Section = "outline"
	toolplane.ApplyToolView(&brief, "brief")
	briefData, err := json.Marshal(brief.Results)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(briefData), "scene beat one") {
		t.Fatalf("brief chapter query leaked scenes: %s", briefData)
	}

	// --fields scenes --view brief must return the beats from the full object.
	withFields := toolplane.QueryOutlineNode(outline, "chapter", "P1-V1-C1")
	withFields.Section = "outline"
	withFields.Query = map[string]string{"fields": "scenes"}
	toolplane.ApplyToolView(&withFields, "brief")
	toolplane.ApplyToolFields(&withFields, "scenes")
	fieldData, err := json.Marshal(withFields.Results)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fieldData), "scene beat one") {
		t.Fatalf("--fields scenes --view brief did not return beats: %s", fieldData)
	}
}

func TestWriteToolJSONUsesCommandOutput(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	if err := toolplane.WriteToolJSON(cmd, toolplane.ToolResponse{OK: true, Section: "context", Count: 1}); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 || !strings.Contains(out.String(), `"section": "context"`) {
		t.Fatalf("toolplane.WriteToolJSON did not write to command output: %q", out.String())
	}
}

func TestToolQueryLogsIndexDoesNotReturnContent(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "logs", "prompts", "ComposeAgent_20260706_090000.md")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("first line\nsecond line\nthird line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	resp := toolplane.QueryLogs(root, "prompts", "", "ComposeAgent", false)
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("toolplane.QueryLogs response = %#v", resp)
	}
	toolplane.ApplyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "detail_query") || !strings.Contains(text, "ComposeAgent") {
		t.Fatalf("index result missing navigation: %s", text)
	}
	if strings.Contains(text, "first line") || strings.Contains(text, "content") || strings.Contains(text, "preview") {
		t.Fatalf("index result leaked log content: %s", text)
	}
}

func TestToolQueryLogsPreviewSkipsMetadataHeader(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "logs", "responses", "WriteAgent_20260706_090000.md")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := "# Agent: WriteAgent\n# Time: 2026-07-06 09:00:00\n\n---\n\n# AI RESPONSE\n\n```json\n{\n\"content\":\"李侑把日志线索变成行动优势。\"\n}\n```"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	resp := toolplane.QueryLogs(root, "responses", "", "WriteAgent", false)
	hits, ok := resp.Results.([]toolplane.LogHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("results = %#v", resp.Results)
	}
	if strings.Contains(hits[0].Preview, "# Agent") || strings.Contains(hits[0].Preview, "# AI RESPONSE") {
		t.Fatalf("preview should skip metadata header: %q", hits[0].Preview)
	}
	if !strings.Contains(hits[0].Preview, "李侑") {
		t.Fatalf("preview missing semantic content: %q", hits[0].Preview)
	}
}

func TestToolQueryLogsContentRequiresExactIDAndIsCapped(t *testing.T) {
	root := t.TempDir()
	completedPath := filepath.Join(root, "logs", "agent-live", "WriteAgent_20260706_085900_000000000.jsonl")
	logPath := filepath.Join(root, "logs", "agent-live", "WriteAgent_20260706_090000_000000000.jsonl")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(completedPath, []byte(`{"event":"start"}`+"\n"+`{"event":"final","model":"deepseek-v4-flash"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte(strings.Repeat("x", 13000)), 0644); err != nil {
		t.Fatal(err)
	}

	withoutID := toolplane.QueryLogs(root, "agent-live", "", "WriteAgent", true)
	hits, ok := withoutID.Results.([]toolplane.LogHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("withoutID results = %#v", withoutID.Results)
	}
	if hits[0].ID != "agent-live/WriteAgent_20260706_085900_000000000.jsonl" {
		t.Fatalf("withoutID should skip incomplete current logs, got %#v", hits)
	}
	if hits[0].Content != "" || hits[0].Preview == "" {
		t.Fatalf("--content without exact --id should return preview only: %#v", hits[0])
	}

	withID := toolplane.QueryLogs(root, "agent-live", "agent-live/WriteAgent_20260706_090000_000000000.jsonl", "", true)
	hits, ok = withID.Results.([]toolplane.LogHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("withID results = %#v", withID.Results)
	}
	if len([]rune(hits[0].Content)) != 12000 || !hits[0].Truncated {
		t.Fatalf("content length/truncated = %d/%t", len([]rune(hits[0].Content)), hits[0].Truncated)
	}
}

func TestToolQueryLogsHonorsHistoryCutoffForIndexes(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs", "agent-live")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	promptDir := filepath.Join(root, "logs", "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(logDir, "WriteAgent_20260706_090000_000000000.jsonl")
	newPath := filepath.Join(logDir, "WriteAgent_20260707_090000_000000000.jsonl")
	oldPromptPath := filepath.Join(promptDir, "WriteAgent_20260706_080000.md")
	newPromptPath := filepath.Join(promptDir, "WriteAgent_20260707_080000.md")
	completed := `{"event":"start","model":"deepseek-v4-flash"}` + "\n" + `{"event":"final","model":"deepseek-v4-flash"}`
	if err := os.WriteFile(oldPath, []byte(completed), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(completed), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPromptPath, []byte("old prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPromptPath, []byte("new prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	for _, path := range []string{oldPath, oldPromptPath} {
		if err := os.Chtimes(path, cutoff.Add(-time.Hour), cutoff.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{newPath, newPromptPath} {
		if err := os.Chtimes(path, cutoff.Add(time.Hour), cutoff.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("NOVELGEN_LOG_HISTORY_CUTOFF", cutoff.Format(time.RFC3339Nano))

	indexResp := toolplane.QueryLogs(root, "", "", "WriteAgent", false)
	hits, ok := indexResp.Results.([]toolplane.LogHit)
	if !ok || len(hits) != 2 {
		t.Fatalf("index results = %#v", indexResp.Results)
	}
	ids := []string{hits[0].ID, hits[1].ID}
	if !containsString(ids, "agent-live/WriteAgent_20260706_090000_000000000.jsonl") ||
		!containsString(ids, "prompts/WriteAgent_20260706_080000.md") {
		t.Fatalf("index should only return logs before cutoff, got %#v", hits)
	}

	exactResp := toolplane.QueryLogs(root, "agent-live", "agent-live/WriteAgent_20260707_090000_000000000.jsonl", "", true)
	exactHits, ok := exactResp.Results.([]toolplane.LogHit)
	if !ok || len(exactHits) != 1 || exactHits[0].Content == "" {
		t.Fatalf("exact id should still read post-cutoff log, got %#v", exactResp.Results)
	}
}

func TestToolQueryLogsIndexSkipsCloneManifestByDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "logs", "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logs", "clone_manifest.json"), []byte(`{"source":"book"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logs", "prompts", "WriteAgent_20260706_090000.md"), []byte("prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	indexResp := toolplane.QueryLogs(root, "", "", "", false)
	hits, ok := indexResp.Results.([]toolplane.LogHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("default index should only return creative logs, got %#v", indexResp.Results)
	}
	if hits[0].ID != "prompts/WriteAgent_20260706_090000.md" {
		t.Fatalf("default index returned unexpected hit: %#v", hits)
	}
	exactResp := toolplane.QueryLogs(root, "", "clone_manifest.json", "", true)
	exactHits, ok := exactResp.Results.([]toolplane.LogHit)
	if !ok || len(exactHits) != 1 || !strings.Contains(exactHits[0].Content, "book") {
		t.Fatalf("exact clone manifest query should still work, got %#v", exactResp.Results)
	}
}

func TestToolQueryAgentLiveLogsRedactsPatchBufferStdinCommands(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "logs", "agent-live", "WriteAgent_20260706_090000_000000000.jsonl")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}
	rawCommand := `printf '%s' 'SECRET_CHAPTER_BODY 很长的正文' | novelgen tool patch-buffer append --id 'P1-V1-C1-draft' --stdin`
	line, err := json.Marshal(map[string]interface{}{
		"event":   "tool_hook",
		"hook":    "PreToolUse",
		"command": rawCommand,
		"allowed": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	content := string(line) + "\n" +
		`{"event":"tool_hook","command":"printf '%s' 'SECRET_CHAPTER_BODY truncated' | novelgen tool patch-buffer append --id 'P1-V1-C1-draft' --stdin` + "\n" +
		`{"event":"final","model":"deepseek-v4-flash"}`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	previewResp := toolplane.QueryLogs(root, "agent-live", "", "WriteAgent", true)
	previewHits, ok := previewResp.Results.([]toolplane.LogHit)
	if !ok || len(previewHits) != 1 {
		t.Fatalf("preview results = %#v", previewResp.Results)
	}
	preview := previewHits[0].Preview
	if strings.Contains(preview, "SECRET_CHAPTER_BODY") || strings.Contains(preview, "printf") {
		t.Fatalf("preview leaked stdin body: %s", preview)
	}
	if !strings.Contains(preview, `novelgen tool patch-buffer append --id`) ||
		!strings.Contains(preview, `P1-V1-C1-draft`) ||
		!strings.Contains(preview, `--stdin`) ||
		!(strings.Contains(preview, `<stdin>`) || strings.Contains(preview, `\u003cstdin\u003e`)) {
		t.Fatalf("preview missing redacted command: %s", preview)
	}

	contentResp := toolplane.QueryLogs(root, "agent-live", "agent-live/WriteAgent_20260706_090000_000000000.jsonl", "", true)
	contentHits, ok := contentResp.Results.([]toolplane.LogHit)
	if !ok || len(contentHits) != 1 {
		t.Fatalf("content results = %#v", contentResp.Results)
	}
	excerpt := contentHits[0].Content
	if strings.Contains(excerpt, "SECRET_CHAPTER_BODY") || strings.Contains(excerpt, "printf") {
		t.Fatalf("content leaked stdin body: %s", excerpt)
	}
	count := strings.Count(excerpt, "--stdin <stdin>") + strings.Count(excerpt, `--stdin \u003cstdin\u003e`)
	if count != 2 {
		t.Fatalf("redacted stdin count = %d, content = %s", count, excerpt)
	}
}

func TestToolQueryAgentLiveLogsIncludesStructuredSummary(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "logs", "agent-live", "WriteAgent_20260706_100000_000000000.jsonl")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}
	rawCommand := `printf '%s' 'SECRET_CHAPTER_BODY long chapter body' | novelgen tool patch-buffer append --id 'P1-V1-C1-draft' --stdin`
	records := []map[string]interface{}{
		{
			"event":             "start",
			"model":             "deepseek-v4-flash",
			"sdk_skills":        []string{"novel-tools-core", "write-improve-workflow"},
			"loaded_sdk_skills": []string{"novel-tools-core", "write-improve-workflow"},
		},
		{
			"event":   "tool_hook",
			"hook":    "PreToolUse",
			"command": rawCommand,
			"allowed": true,
		},
		{
			"event":   "tool_hook",
			"hook":    "PreToolUse",
			"command": `novelgen tool check all --target chapter --scope chapter --id 'P1-V1-C1' --max-issues 8`,
			"allowed": true,
		},
		{
			"event":   "tool_hook",
			"hook":    "PreToolUse",
			"command": `powershell -Command "Get-Content 'C:\Users\me\AppData\Local\Temp\claude\project\tasks\abc.output' -Tail 20 -Wait" 2>$null`,
			"allowed": false,
		},
		{
			"event": "message",
		},
		{
			"event": "final",
			"model": "deepseek-v4-flash",
		},
	}
	lines := make([]string, 0, len(records))
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(data))
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	resp := toolplane.QueryLogs(root, "agent-live", "", "WriteAgent", false)
	hits, ok := resp.Results.([]toolplane.LogHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("results = %#v", resp.Results)
	}
	summary := hits[0].Summary
	if summary["model"] != "deepseek-v4-flash" || summary["final_model"] != "deepseek-v4-flash" {
		t.Fatalf("summary model = %#v", summary)
	}
	if summary["tool_calls"] != 3 || summary["tool_allowed"] != 2 || summary["tool_denied"] != 1 ||
		summary["patch_calls"] != 1 || summary["check_calls"] != 1 || summary["messages"] != 1 {
		t.Fatalf("summary counts = %#v", summary)
	}
	commands, ok := summary["allowed_tool_commands"].([]string)
	if !ok || len(commands) != 2 {
		t.Fatalf("allowed tool commands = %#v", summary["allowed_tool_commands"])
	}
	joined := strings.Join(commands, "\n")
	if strings.Contains(joined, "SECRET_CHAPTER_BODY") || strings.Contains(joined, "printf") {
		t.Fatalf("summary commands leaked stdin body: %#v", commands)
	}
	if !strings.Contains(joined, "--stdin <stdin>") {
		t.Fatalf("summary commands missing stdin placeholder: %#v", commands)
	}
	denied, ok := summary["denied_tool_commands"].([]string)
	if !ok || len(denied) != 1 {
		t.Fatalf("denied tool commands = %#v", summary["denied_tool_commands"])
	}
	if denied[0] != "powershell Get-Content <claude-temp-tool-output>" {
		t.Fatalf("denied command was not summarized: %#v", denied)
	}
}

func TestToolQueryEventsFiltersByEntity(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{{
			ID: "vol_1",
			Chapters: []models.Chapter{{
				ID:    "chap_001",
				Title: "Opening",
				Events: []models.Event{{
					Actor:      "Lin",
					Action:     models.ActionAcquire,
					Target:     "Star Core",
					TargetType: models.TargetTypeItem,
				}, {
					Actor:      "Lin",
					Action:     models.ActionMove,
					Target:     "Mine",
					TargetType: models.TargetTypeLocation,
				}},
			}},
		}},
	}}}

	resp := toolplane.QueryEvents(outline, "", "", "item", "Star Core")
	if resp.Count != 1 {
		t.Fatalf("events count = %d, want 1", resp.Count)
	}
	hits, ok := resp.Results.([]toolplane.EventHit)
	if !ok {
		t.Fatalf("results type = %T, want []toolplane.EventHit", resp.Results)
	}
	if hits[0].Event.GetTarget() != "Star Core" {
		t.Fatalf("event target = %q, want Star Core", hits[0].Event.GetTarget())
	}
}

func TestToolQueryEventsFiltersByVolumeAndIncludesIndex(t *testing.T) {
	outline := &models.Outline{Parts: []models.Part{{
		ID: "part_1",
		Volumes: []models.Volume{{
			ID: "vol_1",
			Chapters: []models.Chapter{{
				ID: "chap_001",
				Events: []models.Event{{
					Actor:  "Lin",
					Action: models.ActionDiscover,
					Target: "Door",
				}, {
					Actor:  "Lin",
					Action: models.ActionMove,
					Target: "Mine",
				}},
			}},
		}, {
			ID: "vol_2",
			Chapters: []models.Chapter{{
				ID: "chap_002",
				Events: []models.Event{{
					Actor:  "Mira",
					Action: models.ActionAcquire,
					Target: "Key",
				}},
			}},
		}},
	}}}

	resp := toolplane.QueryEvents(outline, "", "vol_1", "", "")
	if resp.Count != 2 {
		t.Fatalf("events count = %d, want 2", resp.Count)
	}
	hits, ok := resp.Results.([]toolplane.EventHit)
	if !ok {
		t.Fatalf("results type = %T, want []toolplane.EventHit", resp.Results)
	}
	if hits[0].ChapterID != "chap_001" || hits[0].EventIndex != 0 ||
		hits[1].ChapterID != "chap_001" || hits[1].EventIndex != 1 {
		t.Fatalf("unexpected event hits: %#v", hits)
	}
}

func TestRepairToolLookupTextRepairsMojibakeName(t *testing.T) {
	got, repaired := toolplane.RepairToolLookupText("\u93cb\u6945\u5679")
	if !repaired {
		t.Fatalf("expected lookup text to be repaired")
	}
	if got != "\u6797\u91ce" {
		t.Fatalf("lookup text = %q, want 林野", got)
	}
}

func TestToolQueryContextCraftCharacterBundlesCompactFacts(t *testing.T) {
	ctx := toolplane.ToolProjectContext{
		Setup: &models.StorySetup{
			ProjectName: "Fire Galaxy",
			Premise:     strings.Repeat("premise ", 40),
		},
		Outline: &models.Outline{Parts: []models.Part{{
			ID:    "P1",
			Title: "Part One",
			Volumes: []models.Volume{{
				ID:    "P1-V1",
				Title: "Volume One",
				Chapters: []models.Chapter{{
					ID:         "P1-V1-C1",
					Title:      "Wake",
					Summary:    "Lin wakes and fights.",
					Characters: []string{"Lin"},
					Scenes: []models.OutlineScene{{
						Order: 1,
						POV:   "Lin",
						Goal:  strings.Repeat("heavy scene ", 80),
					}},
					Events: []models.Event{{
						Actor:      "Lin",
						Action:     models.ActionMove,
						Target:     "Worker",
						TargetType: models.TargetTypeCharacter,
						Context:    strings.Repeat("event context ", 80),
					}},
				}},
			}},
		}}},
		RawCharacters: map[string]map[string]interface{}{
			"Lin": {"name": "Lin", "notes": "existing"},
		},
		Characters: map[string]*models.Character{},
	}

	resp := toolplane.QueryCraftCharacterContext(ctx, "Lin")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("context response = %#v", resp)
	}
	bundle, ok := resp.Results.(toolplane.CraftElementContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.CraftElementContext", resp.Results)
	}
	if bundle.Stats["existing_craft"] != 1 ||
		bundle.Stats["outline_refs"] != 1 ||
		bundle.Stats["relevant_chapters"] != 1 ||
		bundle.Stats["events"] != 1 {
		t.Fatalf("unexpected stats: %#v", bundle.Stats)
	}
	if len(bundle.NextActions) < 3 || bundle.NextActions[1].Action != "schema_check" {
		t.Fatalf("missing craft next actions: %+v", bundle.NextActions)
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"story_setup"`) ||
		!strings.Contains(text, `"existing_craft"`) ||
		!strings.Contains(text, `"outline_refs"`) ||
		!strings.Contains(text, `"relevant_chapters"`) ||
		!strings.Contains(text, `"events"`) {
		t.Fatalf("context bundle missing expected sections: %s", text)
	}
	if strings.Contains(text, `"scenes"`) || strings.Contains(text, "heavy scene") {
		t.Fatalf("context bundle should not include heavy scene payload: %s", text)
	}
}

func TestToolQueryContextCraftItemBundlesCompactFacts(t *testing.T) {
	ctx := toolplane.ToolProjectContext{
		Setup: &models.StorySetup{
			ProjectName: "Fire Galaxy",
			WorldResources: []models.WorldResource{{
				Name:        "Star Core",
				Category:    "energy",
				Scarcity:    "unique",
				Description: strings.Repeat("resource ", 60),
			}},
		},
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{{
					ID:      "P1-V1-C1",
					Title:   "Opening",
					Summary: "Lin finds the core.",
					StateAnchor: models.StateAnchor{
						KeyItems: []string{"Star Core"},
					},
					Scenes: []models.OutlineScene{{
						Order: 1,
						Goal:  strings.Repeat("heavy scene ", 80),
					}},
					Events: []models.Event{{
						Actor:      "Lin",
						Action:     models.ActionAcquire,
						Target:     "Star Core",
						TargetType: models.TargetTypeItem,
						Details:    strings.Repeat("heavy event ", 80),
					}},
				}},
			}},
		}}},
		RawItems: map[string]map[string]interface{}{
			"Star Core": {"name": "Star Core", "description": strings.Repeat("existing ", 60)},
		},
		Items: map[string]*models.Item{},
	}

	resp := toolplane.QueryContext(ctx, "craft-item", "", "Star Core")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("context response = %#v", resp)
	}
	bundle, ok := resp.Results.(toolplane.CraftElementContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.CraftElementContext", resp.Results)
	}
	if bundle.Target != "item" || bundle.Name != "Star Core" {
		t.Fatalf("unexpected bundle identity: %#v", bundle)
	}
	if bundle.Stats["existing_craft"] != 1 ||
		bundle.Stats["outline_refs"] != 1 ||
		bundle.Stats["relevant_chapters"] != 1 ||
		bundle.Stats["events"] != 1 {
		t.Fatalf("unexpected stats: %#v", bundle.Stats)
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"existing_craft"`, `"outline_refs"`, `"relevant_chapters"`, `"events"`,
		`tool check schema --target craft --scope item`, `tool patch craft --target item`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("context bundle missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, `"scenes"`) || strings.Contains(text, "heavy scene") || strings.Contains(text, "heavy event") {
		t.Fatalf("context bundle should not include heavy payload: %s", text)
	}
}

func TestToolQueryContextOutlineVolumeBundlesCompactFacts(t *testing.T) {
	ctx := toolplane.ToolProjectContext{
		Setup: &models.StorySetup{
			ProjectName: "Fire Galaxy",
			Premise:     strings.Repeat("premise ", 80),
			Storylines: []models.Storyline{{
				Name:        "Main War",
				Type:        "main",
				Description: strings.Repeat("storyline ", 80),
			}},
		},
		Outline: &models.Outline{Parts: []models.Part{{
			ID:    "P1",
			Title: "Part One",
			Volumes: []models.Volume{
				{ID: "P1-V0", Title: "Previous", Summary: "before"},
				{
					ID:      "P1-V1",
					Title:   "Target",
					Summary: strings.Repeat("target summary ", 80),
					Chapters: []models.Chapter{{
						ID:          "P1-V1-C1",
						Title:       "Opening",
						Summary:     strings.Repeat("chapter summary ", 80),
						Characters:  []string{"Lin", "Mira"},
						Location:    "Mine",
						StateAnchor: models.StateAnchor{KeyItems: []string{"Star Core"}},
						StorylineAdvances: []models.StorylineAdvance{{
							StorylineName: "Main War",
						}},
						Scenes: []models.OutlineScene{{
							Order: 1,
							Goal:  strings.Repeat("heavy scene ", 80),
						}},
						Events: []models.Event{{
							Actor:      "Lin",
							Action:     models.ActionAcquire,
							Target:     "Star Core",
							TargetType: models.TargetTypeItem,
							Details:    strings.Repeat("heavy event ", 80),
						}},
					}},
				},
				{ID: "P1-V2", Title: "Next", Summary: "after"},
			},
		}}},
	}

	resp := toolplane.QueryContext(ctx, "outline-volume", "P1-V1", "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("context response = %#v", resp)
	}
	bundle, ok := resp.Results.(toolplane.OutlineVolumeContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.OutlineVolumeContext", resp.Results)
	}
	if bundle.VolumeID != "P1-V1" || bundle.Path.VolumeID != "P1-V1" {
		t.Fatalf("unexpected volume identity: %#v", bundle)
	}
	if bundle.Stats["chapter_count"] != 1 || bundle.Stats["event_count"] != 1 ||
		bundle.Stats["character_count"] != 2 || bundle.Stats["item_count"] != 1 ||
		bundle.Stats["storyline_count"] != 1 {
		t.Fatalf("unexpected stats: %#v", bundle.Stats)
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"target_volume"`, `"previous_volume"`, `"next_volume"`,
		`"entity_index"`, "tool check all", "tool patch outline", "story-setup --type search",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("context bundle missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, `"story_setup"`) || strings.Contains(text, `"events"`) ||
		strings.Contains(text, `"scenes"`) || strings.Contains(text, "heavy scene") || strings.Contains(text, "heavy event") {
		t.Fatalf("outline-volume context should omit heavy scene/event payload: %s", text)
	}
	if strings.Contains(text, "tool query outline --type volume") {
		t.Fatalf("outline-volume context should route volume detail through context queries: %s", text)
	}
}

func TestToolContextOutlineVolumeIndexViewKeepsRouteOnly(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.OutlineVolumeContext{
			VolumeID: "P1-V1",
			Path:     toolplane.OutlinePath{PartID: "P1", VolumeID: "P1-V1"},
			StorySetup: toolplane.StorySetupBrief{
				Premise: strings.Repeat("large setup ", 80),
			},
			TargetVolume:   toolplane.VolumeBrief{ID: "P1-V1", Summary: strings.Repeat("large target ", 80)},
			PreviousVolume: toolplane.VolumeBrief{ID: "P1-V0", Summary: strings.Repeat("large previous ", 80)},
			NextVolume:     toolplane.VolumeBrief{ID: "P1-V2", Summary: strings.Repeat("large next ", 80)},
			EntityIndex: toolplane.OutlineVolumeEntities{
				Characters: append([]string{"Mira"}, numberedToolStrings("Character", 20)...),
				Items:      append([]string{"Signal Key"}, numberedToolStrings("Item", 20)...),
				Locations:  append([]string{"Camp"}, numberedToolStrings("Location", 20)...),
				Storylines: numberedToolStrings("Storyline", 12),
			},
			Events: []toolplane.EventHitBrief{{Event: toolplane.EventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation: map[string]interface{}{
				"patch_query":            `novelgen tool patch outline --target volume --id 'P1-V1'`,
				"patch_shape":            map[string]interface{}{"changed_chapters": []map[string]string{{"id": "<chapter_id>"}}},
				"post_patch_check_query": `novelgen tool check all --target outline --scope volume --id 'P1-V1'`,
			},
			NextActions: []toolplane.ToolNextAction{{Step: 1, Action: "query_brief_context", Command: `novelgen tool query context --type outline-volume --id 'P1-V1' --view brief`}},
			Stats:       map[string]int{"chapter_count": 8, "event_count": 20, "character_count": 21, "item_count": 21, "location_count": 21, "storyline_count": 12},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`entity_index`, `Mira`, `patch_query`, `patch_shape`, `post_patch_check_query`, `next_actions`, `query_brief_context`, `--view brief`} {
		if !strings.Contains(text, want) {
			t.Fatalf("outline volume index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`large setup`, `large target`, `large previous`, `large next`, `large event`, `story_setup`, `target_volume`, `previous_volume`, `next_volume`, `events`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("outline volume index should omit heavy field %q: %s", forbidden, text)
		}
	}
	if strings.Contains(text, "Character 20") || strings.Contains(text, "Item 20") || strings.Contains(text, "Location 20") || strings.Contains(text, "Storyline 12") {
		t.Fatalf("outline volume index should truncate large entity lists: %s", text)
	}
	if !strings.Contains(text, `"character_count":21`) || !strings.Contains(text, `"storyline_count":12`) {
		t.Fatalf("outline volume index should preserve total entity counts in stats: %s", text)
	}
}

func TestToolContextOutlineGlobalRepairIndexViewKeepsRouteOnly(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.OutlineGlobalRepairContext{
			IssueCategory: "structure",
			Check: &toolplane.ToolCheckResult{
				Kind:    "all",
				Target:  "outline",
				Scope:   "all",
				Summary: toolplane.ToolCheckSummary{Total: 2, Medium: 1, Low: 1},
			},
			IssueContext: []toolplane.OutlineRepairIssue{{
				Category:          "structure",
				TargetID:          "global",
				Issue:             "global structure issue",
				FocusedCheckQuery: `novelgen tool check all --target outline --scope all --category structure --min-priority low --max-issues 12`,
			}},
			StorySetup: toolplane.StorySetupBrief{Premise: strings.Repeat("large setup ", 80)},
			Outline: toolplane.OutlineBrief{Parts: []toolplane.PartBrief{{
				ID:      "P1",
				Title:   "Part One",
				Summary: strings.Repeat("large outline ", 80),
			}}},
			Navigation:  map[string]interface{}{"focused_check_query": `novelgen tool check all --target outline --scope all --category structure --min-priority low --max-issues 12`},
			Workflow:    map[string]interface{}{"stop_rule": "return final json"},
			NextActions: []toolplane.ToolNextAction{{Step: 1, Action: "inspect_first_global_issue"}},
			Stats:       map[string]int{"part_count": 1, "volume_count": 2, "chapter_count": 20, "returned_issues": 2},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`issue_context`, `next_actions`, `classify_unpatchable_global_issue`, `focused_check_query`, `"part_count":1`} {
		if !strings.Contains(text, want) {
			t.Fatalf("outline global repair index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`story_setup`, `"outline":`, `large setup`, `large outline`, `query_brief_global_repair_context`, `repair_context_query`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("outline global repair index should omit heavy field %q: %s", forbidden, text)
		}
	}
	metaData, err := json.Marshal(resp.Meta)
	if err != nil {
		t.Fatal(err)
	}
	metaText := string(metaData)
	for _, want := range []string{`route_only_classify_unpatchable`, `"next_view":"none"`, `"max_extra_queries":1`} {
		if !strings.Contains(metaText, want) {
			t.Fatalf("outline global repair index meta missing %q: %s", want, metaText)
		}
	}
}

func TestToolContextOutlineGlobalRepairMysteriesProvidesPatchableThread(t *testing.T) {
	ctx := toolplane.ToolProjectContext{
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID:    "P1-V1",
				Title: "Volume One",
				Chapters: []models.Chapter{{
					ID:      "P1-V1-C1",
					Title:   "Plant",
					Summary: "Plant a mystery.",
					Mysteries: models.ChapterMysteries{Planted: []models.MysteryPlanted{{
						ID:   "myst_signal",
						Clue: "A blue signal wakes the protagonist.",
					}}},
				}, {
					ID:      "P1-V1-C2",
					Title:   "Follow",
					Summary: "A later chapter can resolve the signal.",
				}},
			}},
		}}},
	}

	resp := toolplane.QueryOutlineGlobalRepairContext(ctx, "mysteries")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("global repair response = %#v", resp)
	}
	bundle, ok := resp.Results.(toolplane.OutlineGlobalRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.OutlineGlobalRepairContext", resp.Results)
	}
	if len(bundle.MysteryThreads) != 1 || bundle.MysteryThreads[0].ID != "myst_signal" {
		t.Fatalf("mystery threads = %#v", bundle.MysteryThreads)
	}
	if len(bundle.IssueContext) == 0 ||
		bundle.IssueContext[0].PatchQuery != `novelgen tool patch outline --target volume --id 'P1-V1'` ||
		bundle.IssueContext[0].PatchShape == nil ||
		bundle.IssueContext[0].PostPatchCheckQuery == "" {
		t.Fatalf("mystery issue should be patchable: %#v", bundle.IssueContext)
	}
	if bundle.PatchTask == nil ||
		bundle.PatchTask.PatchQuery != `novelgen tool patch outline --target volume --id 'P1-V1'` ||
		bundle.PatchTask.TaskID != `outline-global-repair:mysteries` ||
		bundle.PatchTask.PatchShape == nil ||
		!strings.Contains(bundle.PatchTask.DryRunCommand, `--task 'outline-global-repair:mysteries'`) ||
		!strings.Contains(bundle.PatchTask.DryRunCommand, `novelgen tool patch outline --target volume --id 'P1-V1'`) ||
		!strings.Contains(bundle.PatchTask.ApplyCommand, `--apply`) ||
		bundle.PatchTask.PostPatchCheckQuery == "" ||
		bundle.PatchTask.StdinRequired ||
		!containsString(bundle.PatchTask.ForbiddenQueries, "novelgen tool query context --type outline-volume") {
		t.Fatalf("mystery issue should expose a focused patch_task: %#v", bundle.PatchTask)
	}

	indexResp := resp
	toolplane.ApplyToolView(&indexResp, "index")
	indexData, err := json.Marshal(indexResp.Results)
	if err != nil {
		t.Fatal(err)
	}
	indexText := string(indexData)
	for _, want := range []string{`patch_task`, `task_id`, `patch_query`, `patch_shape`, `dry_run_command`, `apply_command`, `--task`, `use_patch_task`, `"mystery_thread_count":1`} {
		if !strings.Contains(indexText, want) {
			t.Fatalf("mystery global repair index missing %q: %s", want, indexText)
		}
	}
	metaText := fmt.Sprint(indexResp.Meta["context_budget"])
	for _, want := range []string{`patch_task_only`, `next_view:none`, `max_extra_queries:0`} {
		if !strings.Contains(metaText, want) {
			t.Fatalf("mystery global repair context budget missing %q: %#v", want, indexResp.Meta["context_budget"])
		}
	}
	if strings.Contains(indexText, "query_brief_global_repair_context") {
		t.Fatalf("patchable mystery route should use patch_task without a brief-query detour: %s", indexText)
	}
	if strings.Contains(indexText, "classify_unpatchable_global_issue") {
		t.Fatalf("patchable mystery route should not be classified unpatchable: %s", indexText)
	}

	briefResp := resp
	toolplane.ApplyToolView(&briefResp, "brief")
	briefData, err := json.Marshal(briefResp.Results)
	if err != nil {
		t.Fatal(err)
	}
	briefText := string(briefData)
	for _, want := range []string{`patch_task`, `mystery_threads`, `suggested_resolution_chapter_id`, `P1-V1-C2`} {
		if !strings.Contains(briefText, want) {
			t.Fatalf("mystery global repair brief missing %q: %s", want, briefText)
		}
	}
	if strings.Contains(briefText, `"story_setup"`) || strings.Contains(briefText, `"outline":`) {
		t.Fatalf("mystery global repair brief should stay focused: %s", briefText)
	}
}

func TestContextBudgetTreatsUnnamedUnpatchableGlobalRepairAsRouteOnly(t *testing.T) {
	resp := toolplane.ToolResponse{
		OK:      true,
		Section: "context",
		Results: toolplane.OutlineGlobalRepairContext{
			IssueContext: []toolplane.OutlineRepairIssue{{
				Category:          "mysteries",
				Issue:             "many open mysteries",
				Priority:          models.PriorityLow,
				FocusedCheckQuery: `novelgen tool check all --target outline --category mysteries --min-priority low --max-issues 12`,
			}},
			NextActions: []toolplane.ToolNextAction{{
				Step:    1,
				Action:  "classify_unpatchable_global_issue",
				Purpose: "No patch route.",
			}},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
	budget, ok := resp.Meta["context_budget"].(map[string]interface{})
	if !ok {
		t.Fatalf("context_budget missing from meta: %#v", resp.Meta)
	}
	if budget["strategy"] != "route_only_classify_unpatchable" ||
		budget["next_view"] != "none" ||
		budget["max_extra_queries"] != 1 {
		t.Fatalf("unnamed unpatchable global budget should stay route-only: %#v", budget)
	}
}

func TestToolViewIndexKeepsBriefActionForPatchableOutlineGlobalRepair(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.OutlineGlobalRepairContext{
			IssueCategory: "faction_tier",
			Check:         &toolplane.ToolCheckResult{Kind: "all", Target: "outline", Scope: "all"},
			IssueContext: []toolplane.OutlineRepairIssue{{
				Category:           "faction_tier",
				TargetID:           "zerg",
				RepairContextQuery: "novelgen tool query story-setup --type search --name 'zerg' --view brief",
				PatchQuery:         "novelgen tool patch setup",
				PatchShape:         map[string]interface{}{"premises": []map[string]string{{"category": "zerg"}}},
			}},
			NextActions: []toolplane.ToolNextAction{{Step: 1, Action: "repair_first_patchable_issue"}},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `query_brief_global_repair_context`) ||
		!strings.Contains(text, `repair_context_query`) ||
		!strings.Contains(text, `repair_first_patchable_issue`) ||
		strings.Contains(text, `classify_unpatchable_global_issue`) {
		t.Fatalf("patchable global repair should keep brief action: %s", text)
	}
}

func numberedToolStrings(prefix string, count int) []string {
	out := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		out = append(out, fmt.Sprintf("%s %02d", prefix, i))
	}
	return out
}

func TestToolViewBriefCompactsOutlineVolume(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "outline",
		Results: []toolplane.OutlineHit{{
			Type:  "volume",
			ID:    "vol_1",
			Title: "Volume One",
			Object: models.Volume{
				ID:      "vol_1",
				Title:   "Volume One",
				Summary: strings.Repeat("summary ", 120),
				Chapters: []models.Chapter{{
					ID:         "chap_001",
					Title:      "Opening",
					Summary:    strings.Repeat("chapter ", 120),
					Characters: []string{"Lin"},
					Location:   "Mine",
					Scenes: []models.OutlineScene{{
						Order: 1, POV: "Lin", Goal: strings.Repeat("scene ", 100),
					}},
					Events: []models.Event{{
						Actor:      "Lin",
						Action:     models.ActionAcquire,
						Target:     "Star Core",
						TargetType: models.TargetTypeItem,
						Details:    strings.Repeat("details ", 100),
					}},
				}},
			},
		}},
	}

	toolplane.ApplyToolView(&resp, "brief")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, `"scenes"`) || strings.Contains(text, `"events"`) {
		t.Fatalf("brief volume view should omit heavy scenes/events: %s", text)
	}
	if !strings.Contains(text, `"navigation"`) || !strings.Contains(text, `"event_count"`) {
		t.Fatalf("brief view missing navigation/event count: %s", text)
	}
	if len([]rune(text)) > 5000 {
		t.Fatalf("brief view too large: %d runes", len([]rune(text)))
	}
}

func TestToolViewCompactsOutlineEvents(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "outline",
		Results: []toolplane.EventHit{{
			ChapterID:    "chap_001",
			ChapterTitle: "Opening",
			Path:         toolplane.OutlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
			Event: models.Event{
				Type:       models.EventTypeItem,
				Subject:    "Star Core",
				Change:     "get",
				Details:    strings.Repeat("details ", 120),
				Actor:      "Lin",
				Action:     models.ActionAcquire,
				Target:     "Star Core",
				TargetType: models.TargetTypeItem,
				Context:    strings.Repeat("context ", 80),
				Result:     strings.Repeat("result ", 80),
			},
			Reasons: []string{"event.text"},
		}},
	}

	toolplane.ApplyToolView(&resp, "brief")
	briefData, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	briefText := string(briefData)
	if !strings.Contains(briefText, `"path"`) || !strings.Contains(briefText, `"details"`) {
		t.Fatalf("brief event view missing useful context: %s", briefText)
	}
	if strings.Count(briefText, "details ") > 40 || strings.Count(briefText, "context ") > 40 {
		t.Fatalf("brief event view did not clip long fields: %s", briefText)
	}

	resp.Results = []toolplane.EventHit{{
		ChapterID:    "chap_001",
		ChapterTitle: "Opening",
		Path:         toolplane.OutlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
		Event: models.Event{
			Type:       models.EventTypeItem,
			Subject:    "Star Core",
			Change:     "get",
			Details:    strings.Repeat("details ", 120),
			Actor:      "Lin",
			Action:     models.ActionAcquire,
			Target:     "Star Core",
			TargetType: models.TargetTypeItem,
			Context:    strings.Repeat("context ", 80),
			Result:     strings.Repeat("result ", 80),
		},
	}}
	toolplane.ApplyToolView(&resp, "index")
	indexData, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	indexText := string(indexData)
	if strings.Contains(indexText, `"path"`) || strings.Contains(indexText, `"details"`) || strings.Contains(indexText, `"context"`) {
		t.Fatalf("index event view should omit path/details/context: %s", indexText)
	}
	if !strings.Contains(indexText, `"actor"`) || !strings.Contains(indexText, `"target"`) {
		t.Fatalf("index event view missing navigation fields: %s", indexText)
	}
	if len(indexText) >= len(briefText) {
		t.Fatalf("index event view should be smaller than brief: index=%d brief=%d", len(indexText), len(briefText))
	}
}

func TestToolFieldsProjectsNestedOutlineChapterFields(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "outline",
		Results: []toolplane.OutlineHit{{
			Type:  "chapter",
			ID:    "chap_001",
			Title: "Opening",
			Path:  toolplane.OutlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
			Object: models.Chapter{
				ID:       "chap_001",
				Title:    "Opening",
				Summary:  "heavy summary",
				Conflict: "Lin must escape.",
				StorylineAdvances: []models.StorylineAdvance{{
					StorylineName: "Main",
					Change:        "Lin commits.",
					Consequence:   "The camp notices him.",
				}},
				ChapterPayoff: &models.ChapterPayoff{
					Desire:       "escape",
					PayoffMoment: "Lin survives",
				},
				Events: []models.Event{{Details: strings.Repeat("heavy ", 80)}},
			},
		}},
	}

	toolplane.ApplyToolView(&resp, "brief")
	toolplane.ApplyToolFields(&resp, "storyline_advances,chapter_payoff,conflict")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"storyline_advances"`) || !strings.Contains(text, `"chapter_payoff"`) || !strings.Contains(text, `"conflict"`) {
		t.Fatalf("projected fields missing: %s", text)
	}
	if !strings.Contains(text, `"id":"chap_001"`) || !strings.Contains(text, `"path"`) {
		t.Fatalf("projection should preserve navigation keys: %s", text)
	}
	if strings.Contains(text, `"events"`) || strings.Contains(text, "heavy heavy") {
		t.Fatalf("projection should omit unrequested heavy fields: %s", text)
	}
}

func TestToolFieldsProjectsEventFields(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "outline",
		Results: []toolplane.EventHit{{
			ChapterID:    "chap_001",
			ChapterTitle: "Opening",
			Path:         toolplane.OutlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
			Event: models.Event{
				Type:       models.EventTypeItem,
				Subject:    "Star Core",
				Change:     "get",
				Details:    "Lin takes the core.",
				Result:     "The core wakes.",
				Target:     "Star Core",
				TargetType: models.TargetTypeItem,
			},
		}},
	}

	toolplane.ApplyToolView(&resp, "brief")
	toolplane.ApplyToolFields(&resp, "result,target_type")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"result"`) || !strings.Contains(text, `"target_type"`) {
		t.Fatalf("projected event fields missing: %s", text)
	}
	if strings.Contains(text, `"details"`) || strings.Contains(text, `"subject"`) {
		t.Fatalf("projection should omit unrequested event fields: %s", text)
	}
}

func TestToolQueryContextOutlineRepairChapterBundlesPatchNavigation(t *testing.T) {
	ctx := toolplane.ToolProjectContext{Outline: validToolPatchTestOutline()}

	resp := toolplane.QueryContext(ctx, "outline-repair", "P1-V1-C1", "logic")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.OutlineRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.OutlineRepairContext", resp.Results)
	}
	if result.Scope != "chapter" || result.ID != "P1-V1-C1" || result.IssueCategory != "logic" {
		t.Fatalf("unexpected repair identity: %+v", result)
	}
	if result.Check == nil || result.Check.Scope != "chapter" || result.Check.ID != "P1-V1-C1" {
		t.Fatalf("missing scoped check: %+v", result.Check)
	}
	if result.ParentVolume == nil || result.Current == nil || result.Events == nil {
		t.Fatalf("missing focused repair context: %+v", result)
	}
	if result.Workflow == nil {
		t.Fatalf("missing repair workflow: %+v", result)
	}
	if len(result.NextActions) < 4 || result.NextActions[2].Action != "patch_dry_run" {
		t.Fatalf("missing outline repair next actions: %+v", result.NextActions)
	}
	if got := fmt.Sprint(result.Navigation["patch_query"]); got != `novelgen tool patch outline --target volume --id 'P1-V1'` {
		t.Fatalf("patch_query = %q", got)
	}
	if got := fmt.Sprint(result.Workflow["post_patch_check_query"]); got != `novelgen tool check all --target outline --scope volume --id 'P1-V1' --min-priority low --max-issues 12` {
		t.Fatalf("post_patch_check_query = %q", got)
	}
	current, ok := result.Current.(toolplane.ChapterBrief)
	if !ok {
		t.Fatalf("current type = %T, want toolplane.ChapterBrief", result.Current)
	}
	if len(current.Events) != 0 {
		t.Fatalf("repair current should not duplicate event details: %+v", current.Events)
	}
	if len(result.Check.Issues) > 0 && len(result.IssueContext) == 0 {
		t.Fatalf("missing issue_context for returned issues: %+v", result.Check.Issues)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`changed_chapters`,
		`post_patch_check_query`,
		`stop_rules`,
		`tool check all --target outline --scope chapter`,
		`tool query outline --type events`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("repair context missing %q: %s", want, text)
		}
	}
}

func TestToolQueryContextOutlineRepairVolumeBundlesPatchNavigation(t *testing.T) {
	ctx := toolplane.ToolProjectContext{Outline: validToolPatchTestOutline()}

	resp := toolplane.QueryContext(ctx, "repair", "P1-V1", "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.OutlineRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.OutlineRepairContext", resp.Results)
	}
	if result.Scope != "volume" || result.ID != "P1-V1" {
		t.Fatalf("unexpected repair identity: %+v", result)
	}
	if got := fmt.Sprint(result.Navigation["patch_query"]); got != `novelgen tool patch outline --target volume --id 'P1-V1'` {
		t.Fatalf("patch_query = %q", got)
	}
	if result.Workflow == nil {
		t.Fatalf("missing repair workflow: %+v", result)
	}
	if len(result.NextActions) < 4 || result.NextActions[3].Action != "post_patch_check" {
		t.Fatalf("missing outline volume next actions: %+v", result.NextActions)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`tool query context --type outline-volume`,
		`chapter_patch_shape`,
		`post_patch_check_query`,
		`stop_rules`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("volume repair context missing %q: %s", want, text)
		}
	}
}

func TestToolQueryContextRecapRepairBundlesFocusedExcerptAndCheck(t *testing.T) {
	root := t.TempDir()
	chapterID := "P1-V1-C1"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       chapterID,
		Title:           "醒来",
		Location:        "残骸",
		Present:         []string{"林砚"},
		PlotBeats:       []string{"林砚醒来。"},
		LastLine:        "他推开舱门。",
		NextOpeningHint: "远处传来警报。",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "chapters"), 0755); err != nil {
		t.Fatal(err)
	}
	longMiddle := strings.Repeat("中段内容 ", 300)
	content := "第一行打开休眠仓。\n" + longMiddle + "\n他推开舱门。"
	if err := os.WriteFile(filepath.Join(root, "chapters", "chapter-"+chapterID+".md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := toolplane.ToolProjectContext{
		Root: toolRoot(t, root),
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{{
					ID:      chapterID,
					Title:   "醒来",
					Summary: strings.Repeat("chapter summary ", 80),
				}},
			}},
		}}},
	}

	resp := toolplane.QueryContext(ctx, "recap-repair", chapterID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("recap repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.RecapRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.RecapRepairContext", resp.Results)
	}
	if result.Check == nil || result.Check.Target != "recap" || result.Check.Scope != "chapter" {
		t.Fatalf("missing recap check: %+v", result.Check)
	}
	if result.Current == nil || result.Outline == nil || result.ChapterExcerpt == nil || result.Workflow == nil {
		t.Fatalf("missing focused recap context: %+v", result)
	}
	if len(result.NextActions) == 0 || result.NextActions[0].Action != "use_current_context" {
		t.Fatalf("missing recap next actions: %+v", result.NextActions)
	}
	if !strings.Contains(result.ChapterExcerpt["last_line"], "他推开舱门") {
		t.Fatalf("excerpt missing last line: %#v", result.ChapterExcerpt)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`tool check quality --target recap`,
		`tool patch recap`,
		`external_regenerate_query`,
		`post_save_check_query`,
		`Agent SDK workflow`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("recap repair context missing %q: %s", want, text)
		}
	}
	if strings.Count(text, "中段内容") > 220 {
		t.Fatalf("recap repair context should omit full chapter text: %s", text)
	}
}
