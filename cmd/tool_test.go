package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"novelgen/internal/models"

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

	charResp := queryOutlineRefs(outline, "character", "Lin", true)
	if charResp.Count != 1 {
		t.Fatalf("character refs count = %d, want 1", charResp.Count)
	}
	itemResp := queryOutlineRefs(outline, "item", "Star Core", true)
	if itemResp.Count != 1 {
		t.Fatalf("item refs count = %d, want 1", itemResp.Count)
	}
	locResp := queryOutlineRefs(outline, "location", "Mine", true)
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
		if got := fieldsRequestScenes(fields); got != want {
			t.Fatalf("fieldsRequestScenes(%q) = %v, want %v", fields, got, want)
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
	brief := queryOutlineNode(outline, "chapter", "P1-V1-C1")
	brief.Section = "outline"
	applyToolView(&brief, "brief")
	briefData, err := json.Marshal(brief.Results)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(briefData), "scene beat one") {
		t.Fatalf("brief chapter query leaked scenes: %s", briefData)
	}

	// --fields scenes --view brief must return the beats from the full object.
	withFields := queryOutlineNode(outline, "chapter", "P1-V1-C1")
	withFields.Section = "outline"
	withFields.Query = map[string]string{"fields": "scenes"}
	applyToolView(&withFields, "brief")
	applyToolFields(&withFields, "scenes")
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

	if err := writeToolJSON(cmd, toolResponse{OK: true, Section: "context", Count: 1}); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 || !strings.Contains(out.String(), `"section": "context"`) {
		t.Fatalf("writeToolJSON did not write to command output: %q", out.String())
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

	resp := queryLogs(root, "prompts", "", "ComposeAgent", false)
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("queryLogs response = %#v", resp)
	}
	applyToolView(&resp, "index")
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

	resp := queryLogs(root, "responses", "", "WriteAgent", false)
	hits, ok := resp.Results.([]logHit)
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

	withoutID := queryLogs(root, "agent-live", "", "WriteAgent", true)
	hits, ok := withoutID.Results.([]logHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("withoutID results = %#v", withoutID.Results)
	}
	if hits[0].ID != "agent-live/WriteAgent_20260706_085900_000000000.jsonl" {
		t.Fatalf("withoutID should skip incomplete current logs, got %#v", hits)
	}
	if hits[0].Content != "" || hits[0].Preview == "" {
		t.Fatalf("--content without exact --id should return preview only: %#v", hits[0])
	}

	withID := queryLogs(root, "agent-live", "agent-live/WriteAgent_20260706_090000_000000000.jsonl", "", true)
	hits, ok = withID.Results.([]logHit)
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

	indexResp := queryLogs(root, "", "", "WriteAgent", false)
	hits, ok := indexResp.Results.([]logHit)
	if !ok || len(hits) != 2 {
		t.Fatalf("index results = %#v", indexResp.Results)
	}
	ids := []string{hits[0].ID, hits[1].ID}
	if !containsString(ids, "agent-live/WriteAgent_20260706_090000_000000000.jsonl") ||
		!containsString(ids, "prompts/WriteAgent_20260706_080000.md") {
		t.Fatalf("index should only return logs before cutoff, got %#v", hits)
	}

	exactResp := queryLogs(root, "agent-live", "agent-live/WriteAgent_20260707_090000_000000000.jsonl", "", true)
	exactHits, ok := exactResp.Results.([]logHit)
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

	indexResp := queryLogs(root, "", "", "", false)
	hits, ok := indexResp.Results.([]logHit)
	if !ok || len(hits) != 1 {
		t.Fatalf("default index should only return creative logs, got %#v", indexResp.Results)
	}
	if hits[0].ID != "prompts/WriteAgent_20260706_090000.md" {
		t.Fatalf("default index returned unexpected hit: %#v", hits)
	}
	exactResp := queryLogs(root, "", "clone_manifest.json", "", true)
	exactHits, ok := exactResp.Results.([]logHit)
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
	rawCommand := `printf '%s' 'SECRET_CHAPTER_BODY 很长的正文' | novelgen tool patch-buffer append --id "P1-V1-C1-draft" --stdin`
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
		`{"event":"tool_hook","command":"printf '%s' 'SECRET_CHAPTER_BODY truncated' | novelgen tool patch-buffer append --id \"P1-V1-C1-draft\" --stdin` + "\n" +
		`{"event":"final","model":"deepseek-v4-flash"}`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	previewResp := queryLogs(root, "agent-live", "", "WriteAgent", true)
	previewHits, ok := previewResp.Results.([]logHit)
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

	contentResp := queryLogs(root, "agent-live", "agent-live/WriteAgent_20260706_090000_000000000.jsonl", "", true)
	contentHits, ok := contentResp.Results.([]logHit)
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
	rawCommand := `printf '%s' 'SECRET_CHAPTER_BODY long chapter body' | novelgen tool patch-buffer append --id "P1-V1-C1-draft" --stdin`
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
			"command": `novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1" --max-issues 8`,
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

	resp := queryLogs(root, "agent-live", "", "WriteAgent", false)
	hits, ok := resp.Results.([]logHit)
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

	resp := queryEvents(outline, "", "", "item", "Star Core")
	if resp.Count != 1 {
		t.Fatalf("events count = %d, want 1", resp.Count)
	}
	hits, ok := resp.Results.([]eventHit)
	if !ok {
		t.Fatalf("results type = %T, want []eventHit", resp.Results)
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

	resp := queryEvents(outline, "", "vol_1", "", "")
	if resp.Count != 2 {
		t.Fatalf("events count = %d, want 2", resp.Count)
	}
	hits, ok := resp.Results.([]eventHit)
	if !ok {
		t.Fatalf("results type = %T, want []eventHit", resp.Results)
	}
	if hits[0].ChapterID != "chap_001" || hits[0].EventIndex != 0 ||
		hits[1].ChapterID != "chap_001" || hits[1].EventIndex != 1 {
		t.Fatalf("unexpected event hits: %#v", hits)
	}
}

func TestRepairToolLookupTextRepairsMojibakeName(t *testing.T) {
	got, repaired := repairToolLookupText("\u93cb\u6945\u5679")
	if !repaired {
		t.Fatalf("expected lookup text to be repaired")
	}
	if got != "\u6797\u91ce" {
		t.Fatalf("lookup text = %q, want 林野", got)
	}
}

func TestToolQueryContextCraftCharacterBundlesCompactFacts(t *testing.T) {
	ctx := toolProjectContext{
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

	resp := queryCraftCharacterContext(ctx, "Lin")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("context response = %#v", resp)
	}
	bundle, ok := resp.Results.(craftElementContext)
	if !ok {
		t.Fatalf("results type = %T, want craftElementContext", resp.Results)
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
	ctx := toolProjectContext{
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

	resp := queryContext(ctx, "craft-item", "", "Star Core")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("context response = %#v", resp)
	}
	bundle, ok := resp.Results.(craftElementContext)
	if !ok {
		t.Fatalf("results type = %T, want craftElementContext", resp.Results)
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
	ctx := toolProjectContext{
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

	resp := queryContext(ctx, "outline-volume", "P1-V1", "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("context response = %#v", resp)
	}
	bundle, ok := resp.Results.(outlineVolumeContext)
	if !ok {
		t.Fatalf("results type = %T, want outlineVolumeContext", resp.Results)
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
	resp := toolResponse{
		Section: "context",
		Results: outlineVolumeContext{
			VolumeID: "P1-V1",
			Path:     outlinePath{PartID: "P1", VolumeID: "P1-V1"},
			StorySetup: storySetupBrief{
				Premise: strings.Repeat("large setup ", 80),
			},
			TargetVolume:   volumeBrief{ID: "P1-V1", Summary: strings.Repeat("large target ", 80)},
			PreviousVolume: volumeBrief{ID: "P1-V0", Summary: strings.Repeat("large previous ", 80)},
			NextVolume:     volumeBrief{ID: "P1-V2", Summary: strings.Repeat("large next ", 80)},
			EntityIndex: outlineVolumeEntities{
				Characters: append([]string{"Mira"}, numberedToolStrings("Character", 20)...),
				Items:      append([]string{"Signal Key"}, numberedToolStrings("Item", 20)...),
				Locations:  append([]string{"Camp"}, numberedToolStrings("Location", 20)...),
				Storylines: numberedToolStrings("Storyline", 12),
			},
			Events: []eventHitBrief{{Event: eventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation: map[string]interface{}{
				"patch_query":            `novelgen tool patch outline --target volume --id "P1-V1"`,
				"patch_shape":            map[string]interface{}{"changed_chapters": []map[string]string{{"id": "<chapter_id>"}}},
				"post_patch_check_query": `novelgen tool check all --target outline --scope volume --id "P1-V1"`,
			},
			NextActions: []toolNextAction{{Step: 1, Action: "query_brief_context", Command: `novelgen tool query context --type outline-volume --id "P1-V1" --view brief`}},
			Stats:       map[string]int{"chapter_count": 8, "event_count": 20, "character_count": 21, "item_count": 21, "location_count": 21, "storyline_count": 12},
		},
	}

	applyToolView(&resp, "index")
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
	resp := toolResponse{
		Section: "context",
		Results: outlineGlobalRepairContext{
			IssueCategory: "structure",
			Check: &toolCheckResult{
				Kind:    "all",
				Target:  "outline",
				Scope:   "all",
				Summary: toolCheckSummary{Total: 2, Medium: 1, Low: 1},
			},
			IssueContext: []outlineRepairIssue{{
				Category:          "structure",
				TargetID:          "global",
				Issue:             "global structure issue",
				FocusedCheckQuery: `novelgen tool check all --target outline --scope all --category structure --min-priority low --max-issues 12`,
			}},
			StorySetup: storySetupBrief{Premise: strings.Repeat("large setup ", 80)},
			Outline: outlineBrief{Parts: []partBrief{{
				ID:      "P1",
				Title:   "Part One",
				Summary: strings.Repeat("large outline ", 80),
			}}},
			Navigation:  map[string]interface{}{"focused_check_query": `novelgen tool check all --target outline --scope all --category structure --min-priority low --max-issues 12`},
			Workflow:    map[string]interface{}{"stop_rule": "return final json"},
			NextActions: []toolNextAction{{Step: 1, Action: "inspect_first_global_issue"}},
			Stats:       map[string]int{"part_count": 1, "volume_count": 2, "chapter_count": 20, "returned_issues": 2},
		},
	}

	applyToolView(&resp, "index")
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
	ctx := toolProjectContext{
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

	resp := queryOutlineGlobalRepairContext(ctx, "mysteries")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("global repair response = %#v", resp)
	}
	bundle, ok := resp.Results.(outlineGlobalRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want outlineGlobalRepairContext", resp.Results)
	}
	if len(bundle.MysteryThreads) != 1 || bundle.MysteryThreads[0].ID != "myst_signal" {
		t.Fatalf("mystery threads = %#v", bundle.MysteryThreads)
	}
	if len(bundle.IssueContext) == 0 ||
		bundle.IssueContext[0].PatchQuery != `novelgen tool patch outline --target volume --id "P1-V1"` ||
		bundle.IssueContext[0].PatchShape == nil ||
		bundle.IssueContext[0].PostPatchCheckQuery == "" {
		t.Fatalf("mystery issue should be patchable: %#v", bundle.IssueContext)
	}
	if bundle.PatchTask == nil ||
		bundle.PatchTask.PatchQuery != `novelgen tool patch outline --target volume --id "P1-V1"` ||
		bundle.PatchTask.TaskID != `outline-global-repair:mysteries` ||
		bundle.PatchTask.PatchShape == nil ||
		!strings.Contains(bundle.PatchTask.DryRunCommand, `--task 'outline-global-repair:mysteries'`) ||
		!strings.Contains(bundle.PatchTask.DryRunCommand, `novelgen tool patch outline --target volume --id "P1-V1"`) ||
		!strings.Contains(bundle.PatchTask.ApplyCommand, `--apply`) ||
		bundle.PatchTask.PostPatchCheckQuery == "" ||
		bundle.PatchTask.StdinRequired ||
		!containsString(bundle.PatchTask.ForbiddenQueries, "novelgen tool query context --type outline-volume") {
		t.Fatalf("mystery issue should expose a focused patch_task: %#v", bundle.PatchTask)
	}

	indexResp := resp
	applyToolView(&indexResp, "index")
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
	applyToolView(&briefResp, "brief")
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
	resp := toolResponse{
		OK:      true,
		Section: "context",
		Results: outlineGlobalRepairContext{
			IssueContext: []outlineRepairIssue{{
				Category:          "mysteries",
				Issue:             "many open mysteries",
				Priority:          models.PriorityLow,
				FocusedCheckQuery: `novelgen tool check all --target outline --category mysteries --min-priority low --max-issues 12`,
			}},
			NextActions: []toolNextAction{{
				Step:    1,
				Action:  "classify_unpatchable_global_issue",
				Purpose: "No patch route.",
			}},
		},
	}

	applyToolView(&resp, "index")
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
	resp := toolResponse{
		Section: "context",
		Results: outlineGlobalRepairContext{
			IssueCategory: "faction_tier",
			Check:         &toolCheckResult{Kind: "all", Target: "outline", Scope: "all"},
			IssueContext: []outlineRepairIssue{{
				Category:           "faction_tier",
				TargetID:           "zerg",
				RepairContextQuery: "novelgen tool query story-setup --type search --name \"zerg\" --view brief",
				PatchQuery:         "novelgen tool patch setup",
				PatchShape:         map[string]interface{}{"premises": []map[string]string{{"category": "zerg"}}},
			}},
			NextActions: []toolNextAction{{Step: 1, Action: "repair_first_patchable_issue"}},
		},
	}

	applyToolView(&resp, "index")
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
	resp := toolResponse{
		Section: "outline",
		Results: []outlineHit{{
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

	applyToolView(&resp, "brief")
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
	resp := toolResponse{
		Section: "outline",
		Results: []eventHit{{
			ChapterID:    "chap_001",
			ChapterTitle: "Opening",
			Path:         outlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
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

	applyToolView(&resp, "brief")
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

	resp.Results = []eventHit{{
		ChapterID:    "chap_001",
		ChapterTitle: "Opening",
		Path:         outlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
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
	applyToolView(&resp, "index")
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
	resp := toolResponse{
		Section: "outline",
		Results: []outlineHit{{
			Type:  "chapter",
			ID:    "chap_001",
			Title: "Opening",
			Path:  outlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
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

	applyToolView(&resp, "brief")
	applyToolFields(&resp, "storyline_advances,chapter_payoff,conflict")
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
	resp := toolResponse{
		Section: "outline",
		Results: []eventHit{{
			ChapterID:    "chap_001",
			ChapterTitle: "Opening",
			Path:         outlinePath{PartID: "part_1", VolumeID: "vol_1", ChapterID: "chap_001"},
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

	applyToolView(&resp, "brief")
	applyToolFields(&resp, "result,target_type")
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
	ctx := toolProjectContext{Outline: validToolPatchTestOutline()}

	resp := queryContext(ctx, "outline-repair", "P1-V1-C1", "logic")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(outlineRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want outlineRepairContext", resp.Results)
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
	if got := fmt.Sprint(result.Navigation["patch_query"]); got != `novelgen tool patch outline --target volume --id "P1-V1"` {
		t.Fatalf("patch_query = %q", got)
	}
	if got := fmt.Sprint(result.Workflow["post_patch_check_query"]); got != `novelgen tool check all --target outline --scope volume --id "P1-V1" --min-priority low --max-issues 12` {
		t.Fatalf("post_patch_check_query = %q", got)
	}
	current, ok := result.Current.(chapterBrief)
	if !ok {
		t.Fatalf("current type = %T, want chapterBrief", result.Current)
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
	ctx := toolProjectContext{Outline: validToolPatchTestOutline()}

	resp := queryContext(ctx, "repair", "P1-V1", "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(outlineRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want outlineRepairContext", resp.Results)
	}
	if result.Scope != "volume" || result.ID != "P1-V1" {
		t.Fatalf("unexpected repair identity: %+v", result)
	}
	if got := fmt.Sprint(result.Navigation["patch_query"]); got != `novelgen tool patch outline --target volume --id "P1-V1"` {
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
	ctx := toolProjectContext{
		Root: root,
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

	resp := queryContext(ctx, "recap-repair", chapterID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("recap repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(recapRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want recapRepairContext", resp.Results)
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

func TestToolViewBriefCompactsStorySetupLongFields(t *testing.T) {
	resp := toolResponse{
		Section: "story-setup",
		Results: &models.StorySetup{
			ProjectName: "Fire Galaxy",
			Premise:     strings.Repeat("premise ", 200),
			LongFormPlan: &models.LongFormPlan{
				TargetChapters:   600,
				TargetVolumes:    6,
				MainLoop:         strings.Repeat("loop ", 80),
				EscalationLadder: []string{strings.Repeat("ladder ", 80)},
				ReaderPromises:   []string{strings.Repeat("promise ", 80)},
				PayoffCadence:    strings.Repeat("payoff ", 80),
			},
			Premises: []models.Premise{{
				Name:        "Core System",
				Description: strings.Repeat("description ", 80),
				Progression: []models.ProgressionStage{{
					Level:        1,
					Name:         "Stage One",
					Description:  strings.Repeat("stage description ", 80),
					Requirements: strings.Repeat("requirements ", 80),
				}},
			}},
		},
	}

	applyToolView(&resp, "brief")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, `"requirements"`) || strings.Contains(text, `"stage description"`) {
		t.Fatalf("brief setup view should omit heavy progression detail: %s", text)
	}
	if !strings.Contains(text, `"target_chapters"`) || !strings.Contains(text, `"progression"`) {
		t.Fatalf("brief setup view missing compact planning data: %s", text)
	}
	if len([]rune(text)) > 2500 {
		t.Fatalf("brief setup view too large: %d runes", len([]rune(text)))
	}
}

func TestQueryStorySetupSearchReturnsCompactCrossSetupContext(t *testing.T) {
	ctx := toolProjectContext{Setup: &models.StorySetup{
		ProjectName: "Fire Galaxy",
		CoreCast: []models.CoreCastSeed{{
			ID:            "cast_worker",
			Name:          "虫族工虫",
			Role:          "support",
			Importance:    4,
			StoryFunction: strings.Repeat("虫族生态线索 ", 80),
			EntryPhase:    "opening",
		}},
		Storylines: []models.Storyline{{
			Name:        "虫族边境危机",
			Type:        "main",
			Importance:  8,
			Description: strings.Repeat("虫族压力升级 ", 80),
		}},
		Premises: []models.Premise{{
			Name:        "虫族母巢网络",
			Category:    "world-system",
			Description: strings.Repeat("虫族通过母巢同步信息 ", 80),
			Progression: []models.ProgressionStage{{
				Level:        1,
				Name:         "工虫低语",
				Description:  strings.Repeat("heavy ", 80),
				Requirements: strings.Repeat("requirements ", 80),
			}},
		}},
		WorldResources: []models.WorldResource{{
			Name:        "虫族晶核",
			Category:    "resource",
			Scarcity:    "rare",
			Description: strings.Repeat("虫族晶核会污染舰载系统 ", 80),
		}},
		WorldTimeline: []models.WorldTimelineEntry{{
			Year:   "1024",
			Event:  "虫族第一次越过星门",
			Impact: strings.Repeat("边境战线收缩 ", 80),
		}},
	}}

	resp := queryStorySetup(ctx, "search", "虫族")
	if !resp.OK || resp.Count != 5 {
		t.Fatalf("search response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(storySetupSearchResult)
	if !ok {
		t.Fatalf("results type = %T, want storySetupSearchResult", resp.Results)
	}
	if len(result.CoreCast) != 1 || len(result.Storylines) != 1 || len(result.Premises) != 1 || len(result.WorldResources) != 1 || len(result.WorldTimeline) != 1 {
		t.Fatalf("unexpected section hits: %+v", result)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "requirements") || len([]rune(text)) > 2400 {
		t.Fatalf("search result should be compact and omit heavy progression details: %s", text)
	}
	if !strings.Contains(text, "detail_queries") || !strings.Contains(text, "story-setup --type premise") {
		t.Fatalf("search result missing navigation: %s", text)
	}
}

func TestToolQueryContextChapterWriteBundlesFocusedContinuityAndNavigation(t *testing.T) {
	root := t.TempDir()
	targetID := "P1-V1-C2"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "Opening",
		Location:        "Mine",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Lin finds the Star Core."},
		LastLine:        "The signal points toward Camp.",
		NextOpeningHint: "Lin follows the signal.",
	}); err != nil {
		t.Fatal(err)
	}
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID: targetID,
		Title:     "Stale Different Story",
		Location:  "Wrong place",
		Present:   []string{"Wrong"},
		LastLine:  "Wrong last line.",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "chapters"), 0755); err != nil {
		t.Fatal(err)
	}
	longMiddle := strings.Repeat("middle chapter prose ", 300)
	content := "Lin follows the signal.\n" + longMiddle + "\nMira sees the Camp lights."
	if err := os.WriteFile(filepath.Join(root, "chapters", "chapter-"+targetID+".md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := toolProjectContext{
		Root: root,
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{{
					ID:         "P1-V1-C1",
					Title:      "Opening",
					Summary:    "Lin enters the mine.",
					Characters: []string{"Lin"},
					Location:   "Mine",
				}, {
					ID:          targetID,
					Title:       "Signal Road",
					Summary:     strings.Repeat("Lin and Mira move toward Camp. ", 30),
					Characters:  []string{"Lin", "Mira"},
					Location:    "Road",
					OpeningBeat: "Lin follows the old signal.",
					ClosingBeat: "Mira sees the Camp lights.",
					StateAnchor: models.StateAnchor{
						Location: "Road",
						KeyItems: []string{"Star Core"},
					},
					StorylineAdvances: []models.StorylineAdvance{{
						StorylineName: "Main arc",
						Stage:         "escape",
						Change:        "The signal becomes actionable.",
					}},
					Events: []models.Event{{
						Actor:      "Lin",
						Action:     models.ActionUse,
						Target:     "Star Core",
						TargetType: models.TargetTypeItem,
						Result:     "The route wakes.",
					}, {
						Actor:      "Mira",
						Action:     models.ActionMove,
						Target:     "Camp",
						TargetType: models.TargetTypeLocation,
					}},
				}, {
					ID:         "P1-V1-C3",
					Title:      "Camp",
					Summary:    "The Camp answers.",
					Characters: []string{"Mira"},
					Location:   "Camp",
				}},
			}},
		}}},
	}

	resp := queryContext(ctx, "write-chapter", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(chapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want chapterWriteContext", resp.Results)
	}
	if result.ChapterID != targetID || result.TargetChapter == nil || result.ParentVolume == nil {
		t.Fatalf("missing target context: %+v", result)
	}
	if len(result.NextActions) < 2 || result.NextActions[1].Action != "return_final_json" {
		t.Fatalf("missing chapter write next actions: %+v", result.NextActions)
	}
	if result.PreviousChapter == nil || result.NextChapter == nil || result.PreviousRecap == nil {
		t.Fatalf("missing continuity context: %+v", result)
	}
	if result.CurrentRecap != nil || !strings.Contains(strings.Join(resp.Warnings, "\n"), "current recap ignored") {
		t.Fatalf("stale current recap should be ignored with warning, current=%+v warnings=%v", result.CurrentRecap, resp.Warnings)
	}
	if result.ExistingChapterExcerpt == nil || !strings.Contains(result.ExistingChapterExcerpt["last_line"], "Camp lights") {
		t.Fatalf("missing chapter excerpt: %#v", result.ExistingChapterExcerpt)
	}
	if len(result.EntityIndex.Characters) == 0 || len(result.EntityIndex.Items) == 0 || len(result.EntityIndex.Locations) == 0 {
		t.Fatalf("missing entity index: %+v", result.EntityIndex)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`return_final_json`,
		`chapter-write context already contains`,
		`Go handles saving`,
		`Do not query full setup`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("chapter write context missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{
		`tool query context --type craft-character`,
		`tool query context --type recap-repair`,
		`tool refresh chapter-dsl`,
		`tool check all --target chapter`,
		`tool patch chapter`,
		`tool query logs`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("chapter write context should not advertise %q: %s", forbidden, text)
		}
	}
	if strings.Count(text, "middle chapter prose") > 80 {
		t.Fatalf("chapter write context should omit full chapter text: %s", text)
	}
}

func TestToolQueryContextChapterWriteSkipsStalePreviousRecap(t *testing.T) {
	root := t.TempDir()
	targetID := "P1-V1-C2"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "Opening",
		Location:        "Old timeline",
		Present:         []string{"Wrong"},
		PlotBeats:       []string{"Old stale recap."},
		LastLine:        "Old stale line.",
		NextOpeningHint: "Old stale hint.",
	}); err != nil {
		t.Fatal(err)
	}
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	previousPath := filepath.Join(chapterDir, "chapter-P1-V1-C1.md")
	targetPath := filepath.Join(chapterDir, "chapter-"+targetID+".md")
	if err := os.WriteFile(previousPath, []byte("# Opening\n\nCurrent previous chapter."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("# Signal Road\n\nCurrent target chapter."), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(filepath.Join(root, "story", "recaps", "P1-V1-C1.json"), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(previousPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	ctx := toolProjectContext{
		Root: root,
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{{
					ID:    "P1-V1-C1",
					Title: "Opening",
				}, {
					ID:    targetID,
					Title: "Signal Road",
				}},
			}},
		}}},
	}

	resp := queryContext(ctx, "chapter-write", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(chapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want chapterWriteContext", resp.Results)
	}
	if result.PreviousRecap != nil || result.Stats["has_previous_recap"] != 0 {
		t.Fatalf("stale previous recap should be omitted, recap=%#v stats=%#v", result.PreviousRecap, result.Stats)
	}
	if !strings.Contains(strings.Join(resp.Warnings, "\n"), "previous recap ignored") ||
		!strings.Contains(strings.Join(resp.Warnings, "\n"), "older than chapter markdown") {
		t.Fatalf("missing stale previous recap warning: %v", resp.Warnings)
	}
}

func TestToolQueryContextChapterWriteIncludesCreativeHistorySnapshot(t *testing.T) {
	root := t.TempDir()
	targetID := "P1-V1-C1"
	writeToolHistoryLog(t, root, "responses", "CraftAgent_20260706_090000.md", "李侑用客服感系统提示拆解危机。\n第二行保留语气。\n第三行保留节奏。", toolProcessStartedAt.Add(-2*time.Hour))
	writeToolHistoryLog(t, root, "responses", "WriteAgent_20260706_091000.md", "主角把日志线索变成行动优势。", toolProcessStartedAt.Add(-90*time.Minute))
	writeToolHistoryLog(t, root, "prompts", "CraftAgent_20260706_085900.md", "Generate craft with system-log contrast.", toolProcessStartedAt.Add(-3*time.Hour))

	ctx := toolProjectContext{
		Root:  root,
		Setup: &models.StorySetup{ProjectName: "System Log", Premise: "A protagonist reads system logs."},
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{{
					ID:      targetID,
					Title:   "First Log",
					Summary: "Li sees the first impossible log.",
				}},
			}},
		}}},
	}

	resp := queryContext(ctx, "chapter-write", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(chapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want chapterWriteContext", resp.Results)
	}
	if result.CreativeHistory == nil {
		t.Fatalf("creative history missing")
	}
	if result.CreativeHistory.Counts["responses"] != 2 || result.CreativeHistory.Counts["prompts"] != 1 {
		t.Fatalf("history counts = %#v", result.CreativeHistory.Counts)
	}
	if len(result.CreativeHistory.RecentResponses) != 2 || !strings.Contains(result.CreativeHistory.RecentResponses[0].Preview, "行动优势") {
		t.Fatalf("recent responses = %#v", result.CreativeHistory.RecentResponses)
	}
	if result.Stats["history_response_count"] != 2 || result.Stats["history_recent_prompts"] != 1 {
		t.Fatalf("history stats = %#v", result.Stats)
	}
	data, err := json.Marshal(result.CreativeHistory)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "tool query logs") || strings.Contains(text, "detail_query") {
		t.Fatalf("creative history should not advertise extra log queries: %s", text)
	}
	if !strings.Contains(text, "风格") || !strings.Contains(text, "不要") {
		t.Fatalf("creative history guidance missing: %s", text)
	}
}

func TestToolQueryContextChapterWriteSkipsCurrentProcessPromptLog(t *testing.T) {
	root := t.TempDir()
	targetID := "P1-V1-C1"
	writeToolHistoryLog(t, root, "prompts", "WriteAgent_20990101_000000.md", "current prompt should not be reflected", toolProcessStartedAt.Add(5*time.Second))

	ctx := toolProjectContext{
		Root: root,
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{{
					ID:      targetID,
					Title:   "First Log",
					Summary: "Li sees the first impossible log.",
				}},
			}},
		}}},
	}

	resp := queryContext(ctx, "chapter-write", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(chapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want chapterWriteContext", resp.Results)
	}
	if result.CreativeHistory != nil {
		t.Fatalf("current process prompt should be skipped, history=%#v", result.CreativeHistory)
	}
}

func writeToolHistoryLog(t *testing.T, root, kind, name, content string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(root, "logs", kind, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set log mtime: %v", err)
	}
}

func TestToolQueryContextChapterRepairBundlesChecksAndNavigation(t *testing.T) {
	root := t.TempDir()
	outline := validToolPatchTestOutline()
	if err := writeToolTestOutline(root, outline); err != nil {
		t.Fatal(err)
	}
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "# Opening\n\n" + strings.Repeat("Lin checks the mine and follows the signal. ", 80)
	if err := os.WriteFile(filepath.Join(chapterDir, "chapter-P1-V1-C1.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := toolProjectContext{
		Root:    root,
		Outline: outline,
		Setup:   &models.StorySetup{ProjectName: "Test", Premise: "Signal in the mine."},
	}
	resp := queryContext(ctx, "chapter-repair", "P1-V1-C1", "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(chapterRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want chapterRepairContext", resp.Results)
	}
	if result.Check == nil || result.Check.Target != "chapter" || result.Check.Kind != "all" {
		t.Fatalf("missing chapter check: %#v", result.Check)
	}
	if result.ExistingChapterExcerpt == nil || result.ExistingChapterExcerpt["opening"] == "" {
		t.Fatalf("missing chapter excerpt: %#v", result.ExistingChapterExcerpt)
	}
	if len(result.NextActions) < 5 || result.NextActions[3].Action != "refresh_derived_dsl" {
		t.Fatalf("missing chapter repair next actions: %+v", result.NextActions)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`tool check all --target chapter`,
		`tool check simulation --target chapter`,
		`tool query context --type chapter-write`,
		`write improve --agent-sdk`,
		`tool refresh chapter-dsl`,
		`do not invoke LLM conversion`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("chapter repair context missing %q: %s", want, text)
		}
	}
}

func TestToolContextBriefViewCompactsRepeatedEventPathsAndPayoff(t *testing.T) {
	volume := volumeBrief{
		ID:      "P1-V1",
		Title:   "Volume",
		Summary: "Summary",
		PayoffContract: &models.VolumePayoffContract{
			VolumeQuestion: "large payoff",
		},
	}
	resp := toolResponse{
		Section: "context",
		Results: chapterRepairContext{
			ChapterID:    "P1-V1-C1",
			ParentVolume: volume,
			Events: []eventHitBrief{{
				ChapterID: "P1-V1-C1",
				Path:      &outlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C1"},
				Event:     eventBrief{Action: "fight", Result: "wins"},
			}},
			NextActions: []toolNextAction{{Step: 1, Action: "use_current_repair_bundle"}},
		},
	}

	applyToolView(&resp, "brief")
	result, ok := resp.Results.(chapterRepairContext)
	if !ok {
		t.Fatalf("brief context type = %T", resp.Results)
	}
	parent, ok := result.ParentVolume.(volumeBrief)
	if !ok || parent.PayoffContract != nil {
		t.Fatalf("brief context should strip heavy volume payoff: %#v", result.ParentVolume)
	}
	events, ok := result.Events.([]eventHitBrief)
	if !ok || len(events) != 1 || events[0].Path != nil || events[0].Event.Result != "wins" {
		t.Fatalf("brief context should strip repeated event path but keep event facts: %#v", result.Events)
	}
}

func TestToolContextIndexViewKeepsExecutionRouteOnly(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: chapterRepairContext{
			ChapterID: "P1-V1-C1",
			Path:      outlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C1"},
			Check: &toolCheckResult{
				Kind:     "all",
				Target:   "chapter",
				Scope:    "chapter",
				ID:       "P1-V1-C1",
				Blocking: true,
				Summary:  toolCheckSummary{Total: 2, Critical: 1},
				Issues: []models.ReviewSuggestion{{
					Issue: strings.Repeat("large issue ", 80),
				}},
			},
			ExistingChapterExcerpt: map[string]string{"opening": strings.Repeat("large prose ", 80)},
			EntityIndex:            outlineVolumeEntities{Characters: []string{"Mira"}, Items: []string{"Signal Key"}, Locations: []string{"Camp"}},
			Events:                 []eventHitBrief{{Event: eventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation:             map[string]interface{}{"patch_query": `novelgen tool patch chapter --id "P1-V1-C1"`},
			Workflow:               map[string]interface{}{"post_repair_check_query": `novelgen tool check all --target chapter --scope chapter --id "P1-V1-C1"`},
			NextActions:            []toolNextAction{{Step: 1, Action: "use_current_repair_bundle"}},
			Stats:                  map[string]int{"check_issues": 2},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`next_actions`, `navigation`, `summary`, `post_repair_check_query`, `entity_index`, `Mira`, `Signal Key`, `query_brief_repair_context`, `--view brief`} {
		if !strings.Contains(text, want) {
			t.Fatalf("context index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`large prose`, `large event`, `large issue`, `existing_chapter_excerpt`, `events`, `use_current_repair_bundle`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("context index should omit heavy field %q: %s", forbidden, text)
		}
	}
	budget, ok := resp.Meta["context_budget"].(map[string]interface{})
	if !ok {
		t.Fatalf("context_budget missing from meta: %#v", resp.Meta)
	}
	if budget["context_level"] != "route_only" || budget["next_view"] != "brief" {
		t.Fatalf("context_budget = %#v", budget)
	}
	avoid, ok := budget["avoid"].([]string)
	if !ok || !containsAllToolTestStrings(avoid, "full_outline", "all_chapters", "source_code_search") {
		t.Fatalf("context_budget avoid = %#v", budget["avoid"])
	}
}

func TestToolContextIndexCleanChapterRepairStopsWithoutBrief(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: chapterRepairContext{
			ChapterID:     "P1-V1-C1",
			IssueCategory: "logic",
			Check: &toolCheckResult{
				Kind:    "all",
				Target:  "chapter",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolCheckSummary{Total: 0},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch chapter --id "P1-V1-C1"`},
			NextActions: []toolNextAction{{
				Step:   1,
				Action: "use_current_repair_bundle",
			}},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`focused_check_clean`, `return_final_json`, `tool check all --target chapter`} {
		if !strings.Contains(text, want) {
			t.Fatalf("clean chapter repair index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`query_brief_repair_context`, `--view brief`, `patch_dry_run`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("clean chapter repair index should stop before %q: %s", forbidden, text)
		}
	}
	budget := resp.Meta["context_budget"].(map[string]interface{})
	if budget["strategy"] != "route_only_clean_focused_check" ||
		budget["next_view"] != "none" ||
		budget["max_extra_queries"] != 0 {
		t.Fatalf("clean chapter repair budget = %#v", budget)
	}
}

func TestToolContextIndexStaleChapterRepairRefreshesBeforeBrief(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: chapterRepairContext{
			ChapterID:     "P1-V1-C1",
			IssueCategory: "simulation",
			Check: &toolCheckResult{
				Kind:    "simulation",
				Target:  "chapter",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolCheckSummary{Total: 1, High: 1},
				Issues: []models.ReviewSuggestion{{
					Category: models.CategoryLogic,
					TargetID: "P1-V1-C1",
					Issue:    "chapter RPG DSL is stale relative to the saved final chapter markdown",
					Priority: models.PriorityHigh,
				}},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch chapter --id "P1-V1-C1"`},
			NextActions: []toolNextAction{{
				Step:   1,
				Action: "use_current_repair_bundle",
			}},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`refresh_derived_dsl_first`, `tool refresh chapter-dsl --id`, `post_refresh_check`, `return_final_json`} {
		if !strings.Contains(text, want) {
			t.Fatalf("stale chapter repair index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`query_brief_repair_context`, `--view brief`, `patch_dry_run`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("stale chapter repair index should stop before %q: %s", forbidden, text)
		}
	}
	budget := resp.Meta["context_budget"].(map[string]interface{})
	if budget["strategy"] != "route_only_refresh_derived_first" ||
		budget["next_view"] != "none" ||
		budget["max_extra_queries"] != 0 {
		t.Fatalf("stale chapter repair budget = %#v", budget)
	}
}

func TestToolContextIndexCleanOutlineRepairStopsWithoutBrief(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: outlineRepairContext{
			Scope:         "chapter",
			ID:            "P1-V1-C1",
			IssueCategory: "logic",
			Check: &toolCheckResult{
				Kind:    "all",
				Target:  "outline",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolCheckSummary{Total: 0},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch outline --target volume --id "P1-V1"`},
			NextActions: []toolNextAction{{
				Step:   1,
				Action: "use_current_repair_bundle",
			}},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`focused_check_clean`, `return_final_json`, `tool check all --target outline`} {
		if !strings.Contains(text, want) {
			t.Fatalf("clean outline repair index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`query_brief_repair_context`, `--view brief`, `patch_dry_run`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("clean outline repair index should stop before %q: %s", forbidden, text)
		}
	}
	budget := resp.Meta["context_budget"].(map[string]interface{})
	if budget["strategy"] != "route_only_clean_focused_check" ||
		budget["next_view"] != "none" ||
		budget["max_extra_queries"] != 0 {
		t.Fatalf("clean outline repair budget = %#v", budget)
	}
}

func TestToolContextIndexCleanRecapRepairStopsWithoutBrief(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: recapRepairContext{
			ChapterID: "P1-V1-C1",
			Check: &toolCheckResult{
				Kind:    "quality",
				Target:  "recap",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolCheckSummary{Total: 0},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch recap --id "P1-V1-C1"`},
			NextActions: []toolNextAction{{
				Step:   1,
				Action: "use_current_context",
			}},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`focused_check_clean`, `return_final_json`, `tool check quality --target recap`} {
		if !strings.Contains(text, want) {
			t.Fatalf("clean recap repair index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`query_brief_repair_context`, `--view brief`, `patch_dry_run`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("clean recap repair index should stop before %q: %s", forbidden, text)
		}
	}
	budget := resp.Meta["context_budget"].(map[string]interface{})
	if budget["strategy"] != "route_only_clean_focused_check" ||
		budget["next_view"] != "none" ||
		budget["max_extra_queries"] != 0 {
		t.Fatalf("clean recap repair budget = %#v", budget)
	}
}

func containsAllToolTestStrings(values []string, wants ...string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, want := range wants {
		if !seen[want] {
			return false
		}
	}
	return true
}

func TestToolContextChapterWriteIndexViewKeepsEntityRouteOnly(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: chapterWriteContext{
			ChapterID: "P1-V1-C2",
			Path:      outlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C2"},
			StorySetup: storySetupBrief{
				Premise: strings.Repeat("large setup ", 80),
			},
			TargetChapter:          chapterBrief{ID: "P1-V1-C2", Title: "Signal Road", Summary: strings.Repeat("large chapter ", 80)},
			PreviousChapter:        chapterBrief{ID: "P1-V1-C1", Title: "Opening", Summary: strings.Repeat("large previous ", 80)},
			ExistingChapterExcerpt: map[string]string{"opening": strings.Repeat("large prose ", 80)},
			EntityIndex:            outlineVolumeEntities{Characters: []string{"Mira"}, Items: []string{"Signal Key"}, Locations: []string{"Camp"}},
			Events:                 []eventHitBrief{{Event: eventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation:             map[string]interface{}{"entity_names": map[string][]string{"characters": []string{"Mira"}}},
			Workflow:               map[string]interface{}{"goal": "Write one chapter"},
			NextActions:            []toolNextAction{{Step: 1, Action: "use_current_context"}, {Step: 2, Action: "return_final_json"}},
			Stats:                  map[string]int{"character_count": 1, "item_count": 1, "location_count": 1},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`entity_index`, `Mira`, `Signal Key`, `entity_names`, `goal`, `next_actions`, `query_brief_context`, `return_final_json`, `--view brief`} {
		if !strings.Contains(text, want) {
			t.Fatalf("chapter write context index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`large setup`, `large chapter`, `large previous`, `large prose`, `large event`, `existing_chapter_excerpt`, `target_chapter`, `events`, `story_setup`, `use_current_context`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("chapter write context index should omit heavy field %q: %s", forbidden, text)
		}
	}
}

func TestToolContextOutlineRepairIndexViewKeepsPatchRouteOnly(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: outlineRepairContext{
			Scope:         "chapter",
			ID:            "P1-V1-C2",
			Path:          outlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C2"},
			IssueCategory: "logic",
			Check: &toolCheckResult{
				Kind:     "all",
				Target:   "outline",
				Scope:    "chapter",
				ID:       "P1-V1-C2",
				Blocking: true,
				Summary:  toolCheckSummary{Total: 1, High: 1},
				Issues: []models.ReviewSuggestion{{
					Issue: strings.Repeat("large outline issue ", 80),
				}},
			},
			IssueContext: []outlineRepairIssue{{
				Category:            "logic",
				TargetID:            "P1-V1-C2",
				TargetName:          "Signal Road",
				Issue:               strings.Repeat("large issue detail ", 80),
				Suggestion:          strings.Repeat("large suggestion ", 80),
				Priority:            models.PriorityHigh,
				FocusedCheckQuery:   `novelgen tool check all --target outline --scope chapter --id "P1-V1-C2"`,
				RepairContextQuery:  `novelgen tool query context --type outline-repair --id "P1-V1-C2" --name "logic" --view brief`,
				PatchQuery:          `novelgen tool patch outline --target volume --id "P1-V1"`,
				PatchShape:          map[string]interface{}{"changed_chapters": []map[string]string{{"id": "P1-V1-C2"}}},
				PostPatchCheckQuery: `novelgen tool check all --target outline --scope volume --id "P1-V1"`,
				Evidence:            map[string]interface{}{"summary": strings.Repeat("large evidence ", 80)},
			}},
			Current: chapterBrief{ID: "P1-V1-C2", Summary: strings.Repeat("large current ", 80)},
			Events:  []eventHitBrief{{Event: eventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation: map[string]interface{}{
				"patch_query": `novelgen tool patch outline --target volume --id "P1-V1"`,
				"patch_shape": map[string]interface{}{"changed_chapters": []map[string]string{{"id": "P1-V1-C2"}}},
			},
			Workflow: map[string]interface{}{
				"patch_query":            `novelgen tool patch outline --target volume --id "P1-V1"`,
				"patch_shape":            map[string]interface{}{"changed_chapters": []map[string]string{{"id": "P1-V1-C2"}}},
				"post_patch_check_query": `novelgen tool check all --target outline --scope volume --id "P1-V1"`,
			},
			NextActions: []toolNextAction{{Step: 1, Action: "use_current_repair_bundle"}},
			Stats:       map[string]int{"returned_issues": 1},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`issue_context`, `patch_shape`, `changed_chapters`, `patch_query`, `post_patch_check_query`, `query_brief_repair_context`, `--view brief`} {
		if !strings.Contains(text, want) {
			t.Fatalf("outline repair index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`large outline issue`, `large issue detail`, `large suggestion`, `large evidence`, `large current`, `large event`, `current`, `events`, `use_current_repair_bundle`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("outline repair index should omit heavy field %q: %s", forbidden, text)
		}
	}
}

func TestToolContextRecapRepairIndexViewKeepsPatchRouteOnly(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: recapRepairContext{
			ChapterID: "P1-V1-C2",
			Path:      outlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C2"},
			Check: &toolCheckResult{
				Kind:     "quality",
				Target:   "recap",
				Scope:    "chapter",
				ID:       "P1-V1-C2",
				Blocking: true,
				Summary:  toolCheckSummary{Total: 1, High: 1},
				Issues: []models.ReviewSuggestion{{
					Issue: strings.Repeat("large recap issue ", 80),
				}},
			},
			Current:        map[string]string{"last_line": strings.Repeat("large recap ", 80)},
			Outline:        chapterBrief{ID: "P1-V1-C2", Summary: strings.Repeat("large outline ", 80)},
			ChapterExcerpt: map[string]string{"opening": strings.Repeat("large prose ", 80)},
			Navigation: map[string]interface{}{
				"patch_query":               `novelgen tool patch recap --id "P1-V1-C2"`,
				"patch_shape":               map[string]interface{}{"location": "<scene anchor>", "last_line": "<final visible line>"},
				"focused_check_query":       `novelgen tool check quality --target recap --scope chapter --id "P1-V1-C2"`,
				"external_regenerate_query": `novelgen recap gen --agent-sdk --chapter "P1-V1-C2" --source chapters`,
			},
			Workflow: map[string]interface{}{
				"patch_query":           `novelgen tool patch recap --id "P1-V1-C2"`,
				"patch_shape":           map[string]interface{}{"location": "<scene anchor>", "last_line": "<final visible line>"},
				"post_save_check_query": `novelgen tool check quality --target recap --scope chapter --id "P1-V1-C2"`,
			},
			NextActions: []toolNextAction{{Step: 1, Action: "use_current_context"}},
			Stats:       map[string]int{"returned_issues": 1},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`patch_query`, `patch_shape`, `post_save_check_query`, `query_brief_repair_context`, `--view brief`, `external_regenerate_query`} {
		if !strings.Contains(text, want) {
			t.Fatalf("recap repair index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`large recap issue`, `large recap`, `large outline`, `large prose`, `current`, `chapter_excerpt`, `outline`, `use_current_context`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("recap repair index should omit heavy field %q: %s", forbidden, text)
		}
	}
}

func TestToolContextCraftElementIndexViewKeepsPatchRouteOnly(t *testing.T) {
	resp := toolResponse{
		Section: "context",
		Results: craftElementContext{
			Target:        "character",
			Name:          "Mira",
			CharacterName: "Mira",
			StorySetup: storySetupBrief{
				Premise: strings.Repeat("large setup ", 80),
			},
			ExistingCraft:    []map[string]string{{"notes": strings.Repeat("large craft ", 80)}},
			OutlineRefs:      []outlineHit{{ID: "P1-V1-C2", Object: chapterBrief{Summary: strings.Repeat("large outline ", 80)}}},
			RelevantChapters: []map[string]string{{"id": "P1-V1-C2", "summary": strings.Repeat("large chapter ", 80)}},
			Events:           []eventHitBrief{{Event: eventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation: map[string]interface{}{
				"schema_check_query":     `novelgen tool check schema --target craft --scope character --id "Mira"`,
				"patch_query":            `novelgen tool patch craft --target character --id "Mira"`,
				"patch_shape":            map[string]interface{}{"<field_to_change>": "<new value>"},
				"post_patch_check_query": `novelgen tool check schema --target craft --scope character --id "Mira"`,
			},
			NextActions: []toolNextAction{{Step: 1, Action: "use_current_context"}},
			Stats:       map[string]int{"existing_craft": 1, "outline_refs": 8, "events": 4},
		},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`query_brief_context`, `craft-character`, `--view brief`, `schema_check_query`, `patch_query`, `patch_shape`, `post_patch_check_query`, `post_patch_check`} {
		if !strings.Contains(text, want) {
			t.Fatalf("craft context index missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{`large setup`, `large craft`, `large outline`, `large chapter`, `large event`, `story_setup`, `relevant_chapters`, `use_current_context`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("craft context index should omit heavy field %q: %s", forbidden, text)
		}
	}
}

func TestToolCraftIndexViewOmitsFullObjects(t *testing.T) {
	resp := toolResponse{
		Section: "craft",
		Results: []map[string]interface{}{{
			"type": "character",
			"key":  "Lin",
			"object": map[string]interface{}{
				"name":          "Lin",
				"role_in_story": "protagonist " + strings.Repeat("long role ", 30),
				"background":    strings.Repeat("large background ", 50),
				"motivation":    strings.Repeat("large motivation ", 50),
			},
		}},
	}

	applyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"key":"Lin"`, `"name":"Lin"`, `"role_in_story":"protagonist`, `tool query craft --type character --name \"Lin\" --view brief`} {
		if !strings.Contains(text, want) {
			t.Fatalf("craft index missing %q: %s", want, text)
		}
	}
	if strings.Count(text, "long role") > 15 {
		t.Fatalf("craft index should clip long role_in_story: %s", text)
	}
	for _, forbidden := range []string{"large background", "large motivation", `"object"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("craft index should omit %q: %s", forbidden, text)
		}
	}
}

func TestBuildChapterSimulationRepairHintsForCombatBalance(t *testing.T) {
	check := &toolCheckResult{
		Kind:   "simulation",
		Target: "chapter",
		Scope:  "chapter",
		ID:     "P1-V1-C1",
		Issues: []models.ReviewSuggestion{{
			Category:   "simulation",
			TargetID:   "P1-V1-C1",
			TargetName: "Opening",
			Issue:      "战斗难度过高！主角基础战力(40)+成长/机甲修正(0)+无盟友支援(0)+战术修正(0)仍低于敌人有效战力(195)",
			Suggestion: "降低敌人等级/数量，或给主角增加技能、道具、盟友支援",
			Priority:   models.PriorityCritical,
		}},
	}

	hints := buildChapterSimulationRepairHints("P1-V1-C1", check)
	if len(hints) != 1 {
		t.Fatalf("hints len = %d, want 1: %#v", len(hints), hints)
	}
	data, err := json.Marshal(hints[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`combat_balance`,
		`机甲`,
		`伏击`,
		`ally`,
		`mech`,
		`power_change`,
		`tool refresh chapter-dsl`,
		`tool check simulation --target chapter`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("combat repair hint missing %q: %s", want, text)
		}
	}
}

func TestBuildChapterSimulationRepairHintsForMachineDiagnostics(t *testing.T) {
	check := &toolCheckResult{
		Kind:   "simulation",
		Target: "chapter",
		Scope:  "chapter",
		ID:     "P1-V1-C1",
		Issues: []models.ReviewSuggestion{{
			Category:   models.CategoryLogic,
			TargetID:   "P1-V1-C1",
			TargetName: "Opening",
			Issue:      "simulation signal diagnostics: combat_steps=1; enemies=enemy_raider; combat_result=false; power_change=false; breakthrough=false; gene=false; mech=false; equipment_or_item=false; ally=false; tactical_text=false; missing_repair_signals=combat_result_on_complete, power_change, mech, ally",
			Suggestion: "Make the next prose patch produce explicit DSL-readable repair signals. For combat balance, add supported mech/gene/equipment/item/ally/power_change/breakthrough signals or reduce enemy count/level. For missing combat result, add an on_complete narration/result and durable state_delta consequence.",
			Priority:   models.PriorityLow,
		}},
	}

	hints := buildChapterSimulationRepairHints("P1-V1-C1", check)
	if len(hints) != 2 {
		t.Fatalf("hints len = %d, want 2: %#v", len(hints), hints)
	}
	types := map[string]bool{}
	data, err := json.Marshal(hints)
	if err != nil {
		t.Fatal(err)
	}
	for _, hint := range hints {
		types[hint.IssueType] = true
	}
	for _, want := range []string{"combat_balance", "combat_result"} {
		if !types[want] {
			t.Fatalf("missing hint type %q: %s", want, data)
		}
	}
	text := string(data)
	for _, want := range []string{
		`refresh chapter-dsl`,
		`tool check simulation --target chapter`,
		`power_change`,
		`on_complete`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("machine diagnostic repair hint missing %q: %s", want, text)
		}
	}
}

func TestToolIssueNavigationForSimulationDiagnosticsIncludesRefresh(t *testing.T) {
	issue := models.ReviewSuggestion{
		Category: models.CategoryLogic,
		TargetID: "P1-V1-C1",
		Issue:    "simulation signal diagnostics: combat_steps=1; combat_result=false; missing_repair_signals=combat_result_on_complete, power_change",
	}

	nav := toolIssueNavigation("simulation", "chapter", "chapter", "P1-V1-C1", issue, 0)
	if nav["refresh_query"] == "" || nav["post_refresh_check_query"] == "" {
		t.Fatalf("simulation diagnostics navigation should include refresh queries: %#v", nav)
	}
	if !strings.Contains(fmt.Sprint(nav["refresh_query"]), "tool refresh chapter-dsl") {
		t.Fatalf("refresh_query should refresh chapter dsl: %#v", nav)
	}
}

func TestChapterRepairSimulationCategoryUsesRealIssueCategories(t *testing.T) {
	filter := chapterRepairIssueCategoryFilter("simulation")
	if !strings.Contains(filter, "simulation") || !strings.Contains(filter, "logic") || !strings.Contains(filter, "structure") {
		t.Fatalf("simulation repair filter should include real issue categories and simulation stale diagnostics, got %q", filter)
	}
	nav := buildChapterRepairNavigation("P1-V1-C1", "simulation", outlineVolumeEntities{})
	data, err := json.Marshal(nav)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "--category logic,plot,structure,character,pacing,simulation") {
		t.Fatalf("simulation navigation should include quality categories plus simulation stale diagnostics: %s", text)
	}
}

func TestToolViewFullPreservesOutlineObject(t *testing.T) {
	volume := models.Volume{ID: "vol_1", Title: "Volume One"}
	resp := toolResponse{
		Section: "outline",
		Results: []outlineHit{{Type: "volume", ID: "vol_1", Object: volume}},
	}

	applyToolView(&resp, "full")
	hits, ok := resp.Results.([]outlineHit)
	if !ok {
		t.Fatalf("results type = %T, want []outlineHit", resp.Results)
	}
	if _, ok := hits[0].Object.(models.Volume); !ok {
		t.Fatalf("full view object type = %T, want models.Volume", hits[0].Object)
	}
}

func TestToolQueryCraftPreservesRawCharacterFields(t *testing.T) {
	ctx := toolProjectContext{
		Characters: map[string]*models.Character{
			"Lin":  {Name: "Lin", RoleInStory: "protagonist"},
			"Mira": {Name: "Mira", RoleInStory: "supporting"},
		},
		RawCharacters: map[string]map[string]interface{}{
			"Lin": {
				"name":          "Lin",
				"role_in_story": "protagonist",
				"character_arc": "learns the cost of power",
				"fears":         []interface{}{"betrayal"},
				"goals":         []interface{}{"survive"},
				"relationships": map[string]interface{}{"Mira": "mentor"},
			},
			"Mira": {
				"name":          "Mira",
				"role_in_story": "supporting",
				"relationships": map[string]interface{}{"Lin": "student"},
			},
		},
	}

	resp := queryCraft(ctx, "character", "Lin")
	if resp.Count != 1 {
		t.Fatalf("craft count = %d, want 1", resp.Count)
	}
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"character_arc", "fears", "goals", "relationships"} {
		if !json.Valid(data) || !containsJSONString(data, field) {
			t.Fatalf("raw field %q missing from response: %s", field, data)
		}
	}

	resp = queryCraft(ctx, "character", "student")
	if resp.Count != 0 {
		t.Fatalf("relationship-only name match count = %d, want 0", resp.Count)
	}
}

func writeToolTestOutline(root string, outline *models.Outline) error {
	dir := filepath.Join(root, "story", "compose")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "outline.json"), data, 0644)
}

func TestBriefCraftContextKeepsCompactElementFields(t *testing.T) {
	results := []map[string]interface{}{{
		"key": "Star Core",
		"object": map[string]interface{}{
			"name":         "Star Core",
			"type":         "artifact",
			"description":  strings.Repeat("ancient ignition memory ", 40),
			"appearance":   "a cracked blue-white core",
			"function":     "stores navigation fire",
			"significance": "proves the protagonist can cross the dead belt",
			"scenes":       []interface{}{strings.Repeat("heavy scene ", 100)},
		},
	}}

	data, err := json.Marshal(briefCraftContextResults(results))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, field := range []string{"description", "appearance", "function", "significance", "available_fields"} {
		if !strings.Contains(text, field) {
			t.Fatalf("brief craft item missing %q: %s", field, text)
		}
	}
	if strings.Contains(text, "heavy scene") {
		t.Fatalf("brief craft item should omit heavy scenes: %s", text)
	}
}

func TestToolFieldsProjectsCharacterCraftFieldsFromBrief(t *testing.T) {
	resp := toolResponse{
		Section: "craft",
		Results: []map[string]interface{}{{
			"type": "character",
			"key":  "Lin",
			"object": map[string]interface{}{
				"name":          "Lin",
				"role_in_story": "protagonist",
				"personality": []interface{}{
					"patient under pressure",
					"keeps secrets reflexively",
				},
				"motivation": "survive long enough to choose his own ending",
				"skills":     []interface{}{"misdirection", "pattern reading"},
				"abilities":  []interface{}{"log sight"},
				"voice":      "quiet, exact, and dryly funny when cornered",
				"notes":      strings.Repeat("keeps a private ledger of risks ", 30),
			},
		}},
	}

	applyToolView(&resp, "brief")
	applyToolFields(&resp, "name,personality,motivation,skills,abilities,voice,notes")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, field := range []string{"name", "personality", "motivation", "skills", "abilities", "voice", "notes"} {
		if !strings.Contains(text, field) {
			t.Fatalf("projected craft field %q missing: %s", field, text)
		}
	}
	if strings.Contains(text, "private ledger of risks private ledger of risks private ledger of risks private ledger of risks private ledger of risks private ledger of risks private ledger of risks private ledger of risks private ledger of risks private ledger of risks") {
		t.Fatalf("brief projected notes should stay clipped: %s", text)
	}
}

func TestNormalizeCraftPatchObjectMapsRPGStatAliases(t *testing.T) {
	raw := map[string]interface{}{
		"name":          "Lin",
		"appearance":    "lean pilot",
		"personality":   []interface{}{"focused"},
		"background":    "old world survivor",
		"motivation":    "survive",
		"role_in_story": "lead",
		"rpg_stats": map[string]interface{}{
			"strength":     float64(6),
			"敏捷":           float64(5),
			"intelligence": float64(7),
			"endurance":    float64(4),
		},
	}

	got, err := normalizeCraftPatchObject("character", "Lin", raw)
	if err != nil {
		t.Fatalf("normalizeCraftPatchObject: %v", err)
	}
	stats, ok := got["rpg_stats"].(map[string]interface{})
	if !ok {
		t.Fatalf("rpg_stats type = %T, want map", got["rpg_stats"])
	}
	for key, want := range map[string]float64{"str": 6, "agi": 5, "int": 7, "vit": 4} {
		if got := stats[key]; got != want {
			t.Fatalf("rpg_stats[%s] = %#v, want %v; stats=%#v", key, got, want, stats)
		}
	}
}

func TestNormalizeCraftPatchObjectRejectsUnsupportedRPGStats(t *testing.T) {
	raw := map[string]interface{}{
		"name":          "Lin",
		"appearance":    "lean pilot",
		"personality":   []interface{}{"focused"},
		"background":    "old world survivor",
		"motivation":    "survive",
		"role_in_story": "lead",
		"rpg_stats": map[string]interface{}{
			"perception": float64(6),
		},
	}

	_, err := normalizeCraftPatchObject("character", "Lin", raw)
	if err == nil {
		t.Fatalf("expected unsupported rpg_stats key to fail")
	}
	if !strings.Contains(err.Error(), "unsupported craft character rpg_stats key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadToolChapterContentUsesFinalChapterPath(t *testing.T) {
	root := t.TempDir()
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(chapterDir, "chapter-chap_001.md")
	if err := os.WriteFile(path, []byte("chapter text"), 0644); err != nil {
		t.Fatal(err)
	}

	gotPath, content := loadToolChapterContent(root, "chap_001")
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if content != "chapter text" {
		t.Fatalf("content = %q, want chapter text", content)
	}
}

func containsJSONString(data []byte, needle string) bool {
	return strings.Contains(string(data), `"`+needle+`"`)
}
