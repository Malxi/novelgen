package toolplane_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	models "novelgen/internal/contract/story"
	"novelgen/internal/toolplane"
)

func TestToolViewBriefCompactsStorySetupLongFields(t *testing.T) {
	resp := toolplane.ToolResponse{
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

	toolplane.ApplyToolView(&resp, "brief")
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
	ctx := toolplane.ToolProjectContext{Setup: &models.StorySetup{
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

	resp := toolplane.QueryStorySetup(ctx, "search", "虫族")
	if !resp.OK || resp.Count != 5 {
		t.Fatalf("search response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.StorySetupSearchResult)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.StorySetupSearchResult", resp.Results)
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
	rootStore := toolRoot(t, root)
	if err := rootStore.SaveConfig(&models.ProjectConfig{Name: "Context Fixture", Language: "en"}); err != nil {
		t.Fatal(err)
	}
	if err := rootStore.SaveSetup(&models.StorySetup{
		ProjectName: "Context Fixture",
		Premise:     "A signal leads through an abandoned mine.",
	}); err != nil {
		t.Fatal(err)
	}
	targetID := "P1-V1-C2"
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "Opening",
		Location:        "Mine",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Lin finds the Star Core."},
		Promises:        []string{"P01", "P02", "P03", "P04", "P05", "P06", "P07", "P08", "P09", "P10", "P11", "P12"},
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
	ctx := toolplane.ToolProjectContext{
		Root: toolRoot(t, root),
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
	if err := rootStore.SaveOutline(ctx.Outline); err != nil {
		t.Fatal(err)
	}

	resp := toolplane.QueryContext(ctx, "write-chapter", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.ChapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.ChapterWriteContext", resp.Results)
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
	if !strings.Contains(strings.Join(resp.Warnings, "\n"), "--intent promise-audit") {
		t.Fatalf("bounded promise context should advertise the complete audit: %v", resp.Warnings)
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

	ctx := toolplane.ToolProjectContext{
		Root: toolRoot(t, root),
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

	resp := toolplane.QueryContext(ctx, "chapter-write", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.ChapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.ChapterWriteContext", resp.Results)
	}
	if result.PreviousRecap != nil || result.Stats["has_previous_recap"] != 0 {
		t.Fatalf("stale previous recap should be omitted, recap=%#v stats=%#v", result.PreviousRecap, result.Stats)
	}
	if !strings.Contains(strings.Join(resp.Warnings, "\n"), "previous recap ignored") ||
		!strings.Contains(strings.Join(resp.Warnings, "\n"), "older than chapter markdown") {
		t.Fatalf("missing stale previous recap warning: %v", resp.Warnings)
	}
}

func TestToolQueryContextChapterWriteCanSuppressRecapBriefs(t *testing.T) {
	root := t.TempDir()
	if err := saveTestRecap(root, models.ChapterRecap{
		ChapterID:       "P1-V1-C1",
		Title:           "Opening",
		Location:        "Mine",
		Present:         []string{"Lin"},
		PlotBeats:       []string{"Lin opens the door."},
		LastLine:        "The door opens.",
		NextOpeningHint: "Lin enters.",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "chapters"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "chapters", "chapter-P1-V1-C1.md"), []byte("# Opening\n\nLin opens the door."), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOVELGEN_NO_RECAP_CONTEXT", "1")
	ctx := toolplane.ToolProjectContext{
		Root: toolRoot(t, root),
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID: "P1-V1",
				Chapters: []models.Chapter{
					{ID: "P1-V1-C1", Title: "Opening"},
					{ID: "P1-V1-C2", Title: "Return"},
				},
			}},
		}}},
	}

	resp := toolplane.QueryContext(ctx, "chapter-write", "P1-V1-C2", "")
	if !resp.OK {
		t.Fatalf("chapter write context failed: %v", resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.ChapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T", resp.Results)
	}
	if result.PreviousRecap != nil || result.CurrentRecap != nil {
		t.Fatalf("recap briefs should be suppressed: %#v", result)
	}
	if result.Stats["recap_context_disabled"] != 1 || !strings.Contains(strings.Join(resp.Warnings, "\n"), "NOVELGEN_NO_RECAP_CONTEXT=1") {
		t.Fatalf("missing suppression evidence: stats=%v warnings=%v", result.Stats, resp.Warnings)
	}
}

func TestToolQueryContextChapterWriteIncludesCreativeHistorySnapshot(t *testing.T) {
	root := t.TempDir()
	targetID := "P1-V1-C1"
	writeToolHistoryLog(t, root, "responses", "CraftAgent_20260706_090000.md", "李侑用客服感系统提示拆解危机。\n第二行保留语气。\n第三行保留节奏。", toolplane.ToolProcessStartedAt.Add(-2*time.Hour))
	writeToolHistoryLog(t, root, "responses", "WriteAgent_20260706_091000.md", "主角把日志线索变成行动优势。", toolplane.ToolProcessStartedAt.Add(-90*time.Minute))
	writeToolHistoryLog(t, root, "prompts", "CraftAgent_20260706_085900.md", "Generate craft with system-log contrast.", toolplane.ToolProcessStartedAt.Add(-3*time.Hour))

	ctx := toolplane.ToolProjectContext{
		Root:  toolRoot(t, root),
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

	resp := toolplane.QueryContext(ctx, "chapter-write", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.ChapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.ChapterWriteContext", resp.Results)
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
	writeToolHistoryLog(t, root, "prompts", "WriteAgent_20990101_000000.md", "current prompt should not be reflected", toolplane.ToolProcessStartedAt.Add(5*time.Second))

	ctx := toolplane.ToolProjectContext{
		Root: toolRoot(t, root),
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

	resp := toolplane.QueryContext(ctx, "chapter-write", targetID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter write context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.ChapterWriteContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.ChapterWriteContext", resp.Results)
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

	ctx := toolplane.ToolProjectContext{
		Root:    toolRoot(t, root),
		Outline: outline,
		Setup:   &models.StorySetup{ProjectName: "Test", Premise: "Signal in the mine."},
	}
	resp := toolplane.QueryContext(ctx, "chapter-repair", "P1-V1-C1", "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter repair context response = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	result, ok := resp.Results.(toolplane.ChapterRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.ChapterRepairContext", resp.Results)
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
	volume := toolplane.VolumeBrief{
		ID:      "P1-V1",
		Title:   "Volume",
		Summary: "Summary",
		PayoffContract: &models.VolumePayoffContract{
			VolumeQuestion: "large payoff",
		},
	}
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.ChapterRepairContext{
			ChapterID:    "P1-V1-C1",
			ParentVolume: volume,
			Events: []toolplane.EventHitBrief{{
				ChapterID: "P1-V1-C1",
				Path:      &toolplane.OutlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C1"},
				Event:     toolplane.EventBrief{Action: "fight", Result: "wins"},
			}},
			NextActions: []toolplane.ToolNextAction{{Step: 1, Action: "use_current_repair_bundle"}},
		},
	}

	toolplane.ApplyToolView(&resp, "brief")
	result, ok := resp.Results.(toolplane.ChapterRepairContext)
	if !ok {
		t.Fatalf("brief context type = %T", resp.Results)
	}
	parent, ok := result.ParentVolume.(toolplane.VolumeBrief)
	if !ok || parent.PayoffContract != nil {
		t.Fatalf("brief context should strip heavy volume payoff: %#v", result.ParentVolume)
	}
	events, ok := result.Events.([]toolplane.EventHitBrief)
	if !ok || len(events) != 1 || events[0].Path != nil || events[0].Event.Result != "wins" {
		t.Fatalf("brief context should strip repeated event path but keep event facts: %#v", result.Events)
	}
}

func TestToolContextIndexViewKeepsExecutionRouteOnly(t *testing.T) {
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.ChapterRepairContext{
			ChapterID: "P1-V1-C1",
			Path:      toolplane.OutlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C1"},
			Check: &toolplane.ToolCheckResult{
				Kind:     "all",
				Target:   "chapter",
				Scope:    "chapter",
				ID:       "P1-V1-C1",
				Blocking: true,
				Summary:  toolplane.ToolCheckSummary{Total: 2, Critical: 1},
				Issues: []models.ReviewSuggestion{{
					Issue: strings.Repeat("large issue ", 80),
				}},
			},
			ExistingChapterExcerpt: map[string]string{"opening": strings.Repeat("large prose ", 80)},
			EntityIndex:            toolplane.OutlineVolumeEntities{Characters: []string{"Mira"}, Items: []string{"Signal Key"}, Locations: []string{"Camp"}},
			Events:                 []toolplane.EventHitBrief{{Event: toolplane.EventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation:             map[string]interface{}{"patch_query": `novelgen tool patch chapter --id 'P1-V1-C1'`},
			Workflow:               map[string]interface{}{"post_repair_check_query": `novelgen tool check all --target chapter --scope chapter --id 'P1-V1-C1'`},
			NextActions:            []toolplane.ToolNextAction{{Step: 1, Action: "use_current_repair_bundle"}},
			Stats:                  map[string]int{"check_issues": 2},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.ChapterRepairContext{
			ChapterID:     "P1-V1-C1",
			IssueCategory: "logic",
			Check: &toolplane.ToolCheckResult{
				Kind:    "all",
				Target:  "chapter",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolplane.ToolCheckSummary{Total: 0},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch chapter --id 'P1-V1-C1'`},
			NextActions: []toolplane.ToolNextAction{{
				Step:   1,
				Action: "use_current_repair_bundle",
			}},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.ChapterRepairContext{
			ChapterID:     "P1-V1-C1",
			IssueCategory: "simulation",
			Check: &toolplane.ToolCheckResult{
				Kind:    "simulation",
				Target:  "chapter",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolplane.ToolCheckSummary{Total: 1, High: 1},
				Issues: []models.ReviewSuggestion{{
					Category: models.CategoryLogic,
					TargetID: "P1-V1-C1",
					Issue:    "chapter RPG DSL is stale relative to the saved final chapter markdown",
					Priority: models.PriorityHigh,
				}},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch chapter --id 'P1-V1-C1'`},
			NextActions: []toolplane.ToolNextAction{{
				Step:   1,
				Action: "use_current_repair_bundle",
			}},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.OutlineRepairContext{
			Scope:         "chapter",
			ID:            "P1-V1-C1",
			IssueCategory: "logic",
			Check: &toolplane.ToolCheckResult{
				Kind:    "all",
				Target:  "outline",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolplane.ToolCheckSummary{Total: 0},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch outline --target volume --id 'P1-V1'`},
			NextActions: []toolplane.ToolNextAction{{
				Step:   1,
				Action: "use_current_repair_bundle",
			}},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.RecapRepairContext{
			ChapterID: "P1-V1-C1",
			Check: &toolplane.ToolCheckResult{
				Kind:    "quality",
				Target:  "recap",
				Scope:   "chapter",
				ID:      "P1-V1-C1",
				Summary: toolplane.ToolCheckSummary{Total: 0},
			},
			Navigation: map[string]interface{}{"patch_query": `novelgen tool patch recap --id 'P1-V1-C1'`},
			NextActions: []toolplane.ToolNextAction{{
				Step:   1,
				Action: "use_current_context",
			}},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.ChapterWriteContext{
			ChapterID: "P1-V1-C2",
			Path:      toolplane.OutlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C2"},
			StorySetup: toolplane.StorySetupBrief{
				Premise: strings.Repeat("large setup ", 80),
			},
			TargetChapter:          toolplane.ChapterBrief{ID: "P1-V1-C2", Title: "Signal Road", Summary: strings.Repeat("large chapter ", 80)},
			PreviousChapter:        toolplane.ChapterBrief{ID: "P1-V1-C1", Title: "Opening", Summary: strings.Repeat("large previous ", 80)},
			ExistingChapterExcerpt: map[string]string{"opening": strings.Repeat("large prose ", 80)},
			EntityIndex:            toolplane.OutlineVolumeEntities{Characters: []string{"Mira"}, Items: []string{"Signal Key"}, Locations: []string{"Camp"}},
			Events:                 []toolplane.EventHitBrief{{Event: toolplane.EventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation:             map[string]interface{}{"entity_names": map[string][]string{"characters": []string{"Mira"}}},
			Workflow:               map[string]interface{}{"goal": "Write one chapter"},
			NextActions:            []toolplane.ToolNextAction{{Step: 1, Action: "use_current_context"}, {Step: 2, Action: "return_final_json"}},
			Stats:                  map[string]int{"character_count": 1, "item_count": 1, "location_count": 1},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.OutlineRepairContext{
			Scope:         "chapter",
			ID:            "P1-V1-C2",
			Path:          toolplane.OutlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C2"},
			IssueCategory: "logic",
			Check: &toolplane.ToolCheckResult{
				Kind:     "all",
				Target:   "outline",
				Scope:    "chapter",
				ID:       "P1-V1-C2",
				Blocking: true,
				Summary:  toolplane.ToolCheckSummary{Total: 1, High: 1},
				Issues: []models.ReviewSuggestion{{
					Issue: strings.Repeat("large outline issue ", 80),
				}},
			},
			IssueContext: []toolplane.OutlineRepairIssue{{
				Category:            "logic",
				TargetID:            "P1-V1-C2",
				TargetName:          "Signal Road",
				Issue:               strings.Repeat("large issue detail ", 80),
				Suggestion:          strings.Repeat("large suggestion ", 80),
				Priority:            models.PriorityHigh,
				FocusedCheckQuery:   `novelgen tool check all --target outline --scope chapter --id 'P1-V1-C2'`,
				RepairContextQuery:  `novelgen tool query context --type outline-repair --id 'P1-V1-C2' --name 'logic' --view brief`,
				PatchQuery:          `novelgen tool patch outline --target volume --id 'P1-V1'`,
				PatchShape:          map[string]interface{}{"changed_chapters": []map[string]string{{"id": "P1-V1-C2"}}},
				PostPatchCheckQuery: `novelgen tool check all --target outline --scope volume --id 'P1-V1'`,
				Evidence:            map[string]interface{}{"summary": strings.Repeat("large evidence ", 80)},
			}},
			Current: toolplane.ChapterBrief{ID: "P1-V1-C2", Summary: strings.Repeat("large current ", 80)},
			Events:  []toolplane.EventHitBrief{{Event: toolplane.EventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation: map[string]interface{}{
				"patch_query": `novelgen tool patch outline --target volume --id 'P1-V1'`,
				"patch_shape": map[string]interface{}{"changed_chapters": []map[string]string{{"id": "P1-V1-C2"}}},
			},
			Workflow: map[string]interface{}{
				"patch_query":            `novelgen tool patch outline --target volume --id 'P1-V1'`,
				"patch_shape":            map[string]interface{}{"changed_chapters": []map[string]string{{"id": "P1-V1-C2"}}},
				"post_patch_check_query": `novelgen tool check all --target outline --scope volume --id 'P1-V1'`,
			},
			NextActions: []toolplane.ToolNextAction{{Step: 1, Action: "use_current_repair_bundle"}},
			Stats:       map[string]int{"returned_issues": 1},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.RecapRepairContext{
			ChapterID: "P1-V1-C2",
			Path:      toolplane.OutlinePath{PartID: "P1", VolumeID: "P1-V1", ChapterID: "P1-V1-C2"},
			Check: &toolplane.ToolCheckResult{
				Kind:     "quality",
				Target:   "recap",
				Scope:    "chapter",
				ID:       "P1-V1-C2",
				Blocking: true,
				Summary:  toolplane.ToolCheckSummary{Total: 1, High: 1},
				Issues: []models.ReviewSuggestion{{
					Issue: strings.Repeat("large recap issue ", 80),
				}},
			},
			Current:        map[string]string{"last_line": strings.Repeat("large recap ", 80)},
			Outline:        toolplane.ChapterBrief{ID: "P1-V1-C2", Summary: strings.Repeat("large outline ", 80)},
			ChapterExcerpt: map[string]string{"opening": strings.Repeat("large prose ", 80)},
			Navigation: map[string]interface{}{
				"patch_query":               `novelgen tool patch recap --id 'P1-V1-C2'`,
				"patch_shape":               map[string]interface{}{"location": "<scene anchor>", "last_line": "<final visible line>"},
				"focused_check_query":       `novelgen tool check quality --target recap --scope chapter --id 'P1-V1-C2'`,
				"external_regenerate_query": `novelgen recap gen --agent-sdk --chapter "P1-V1-C2" --source chapters`,
			},
			Workflow: map[string]interface{}{
				"patch_query":           `novelgen tool patch recap --id 'P1-V1-C2'`,
				"patch_shape":           map[string]interface{}{"location": "<scene anchor>", "last_line": "<final visible line>"},
				"post_save_check_query": `novelgen tool check quality --target recap --scope chapter --id 'P1-V1-C2'`,
			},
			NextActions: []toolplane.ToolNextAction{{Step: 1, Action: "use_current_context"}},
			Stats:       map[string]int{"returned_issues": 1},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
		Section: "context",
		Results: toolplane.CraftElementContext{
			Target:        "character",
			Name:          "Mira",
			CharacterName: "Mira",
			StorySetup: toolplane.StorySetupBrief{
				Premise: strings.Repeat("large setup ", 80),
			},
			ExistingCraft:    []map[string]string{{"notes": strings.Repeat("large craft ", 80)}},
			OutlineRefs:      []toolplane.OutlineHit{{ID: "P1-V1-C2", Object: toolplane.ChapterBrief{Summary: strings.Repeat("large outline ", 80)}}},
			RelevantChapters: []map[string]string{{"id": "P1-V1-C2", "summary": strings.Repeat("large chapter ", 80)}},
			Events:           []toolplane.EventHitBrief{{Event: toolplane.EventBrief{Details: strings.Repeat("large event ", 80)}}},
			Navigation: map[string]interface{}{
				"schema_check_query":     `novelgen tool check schema --target craft --scope character --id 'Mira'`,
				"patch_query":            `novelgen tool patch craft --target character --id 'Mira'`,
				"patch_shape":            map[string]interface{}{"<field_to_change>": "<new value>"},
				"post_patch_check_query": `novelgen tool check schema --target craft --scope character --id 'Mira'`,
			},
			NextActions: []toolplane.ToolNextAction{{Step: 1, Action: "use_current_context"}},
			Stats:       map[string]int{"existing_craft": 1, "outline_refs": 8, "events": 4},
		},
	}

	toolplane.ApplyToolView(&resp, "index")
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
	resp := toolplane.ToolResponse{
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

	toolplane.ApplyToolView(&resp, "index")
	data, err := json.Marshal(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"key":"Lin"`, `"name":"Lin"`, `"role_in_story":"protagonist`, `tool query craft --type character --name 'Lin' --view brief`} {
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
	check := &toolplane.ToolCheckResult{
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

	hints := toolplane.BuildChapterSimulationRepairHints("P1-V1-C1", check)
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
	check := &toolplane.ToolCheckResult{
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

	hints := toolplane.BuildChapterSimulationRepairHints("P1-V1-C1", check)
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

	nav := toolplane.ToolIssueNavigation("simulation", "chapter", "chapter", "P1-V1-C1", issue, 0)
	if nav["refresh_query"] == "" || nav["post_refresh_check_query"] == "" {
		t.Fatalf("simulation diagnostics navigation should include refresh queries: %#v", nav)
	}
	if !strings.Contains(fmt.Sprint(nav["refresh_query"]), "tool refresh chapter-dsl") {
		t.Fatalf("refresh_query should refresh chapter dsl: %#v", nav)
	}
}

func TestChapterRepairSimulationCategoryUsesRealIssueCategories(t *testing.T) {
	filter := toolplane.ChapterRepairIssueCategoryFilter("simulation")
	if !strings.Contains(filter, "simulation") || !strings.Contains(filter, "logic") || !strings.Contains(filter, "structure") {
		t.Fatalf("simulation repair filter should include real issue categories and simulation stale diagnostics, got %q", filter)
	}
	nav := toolplane.BuildChapterRepairNavigation("P1-V1-C1", "simulation", toolplane.OutlineVolumeEntities{})
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
	resp := toolplane.ToolResponse{
		Section: "outline",
		Results: []toolplane.OutlineHit{{Type: "volume", ID: "vol_1", Object: volume}},
	}

	toolplane.ApplyToolView(&resp, "full")
	hits, ok := resp.Results.([]toolplane.OutlineHit)
	if !ok {
		t.Fatalf("results type = %T, want []toolplane.OutlineHit", resp.Results)
	}
	if _, ok := hits[0].Object.(models.Volume); !ok {
		t.Fatalf("full view object type = %T, want models.Volume", hits[0].Object)
	}
}

func TestToolQueryCraftPreservesRawCharacterFields(t *testing.T) {
	ctx := toolplane.ToolProjectContext{
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

	resp := toolplane.QueryCraft(ctx, "character", "Lin")
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

	resp = toolplane.QueryCraft(ctx, "character", "student")
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

	data, err := json.Marshal(toolplane.BriefCraftContextResults(results))
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
	resp := toolplane.ToolResponse{
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

	toolplane.ApplyToolView(&resp, "brief")
	toolplane.ApplyToolFields(&resp, "name,personality,motivation,skills,abilities,voice,notes")
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

	got, err := toolplane.NormalizeCraftPatchObject("character", "Lin", raw)
	if err != nil {
		t.Fatalf("toolplane.NormalizeCraftPatchObject: %v", err)
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

	_, err := toolplane.NormalizeCraftPatchObject("character", "Lin", raw)
	if err == nil {
		t.Fatalf("expected unsupported rpg_stats key to fail")
	}
	if !strings.Contains(err.Error(), "unsupported craft character rpg_stats key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// The chapter excerpt an agent gets from a repair context must come from the same
// file the write pipeline and export read. This used to be a separate candidate
// list inside the tool plane, which silently skipped the legacy numeric chapter
// filename.
func TestChapterRepairContextReadsStoreChapterCandidates(t *testing.T) {
	chapterID := "chap_1_2_3"
	root := t.TempDir()
	chapterDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(chapterDir, "chapter-3.md")
	if err := os.WriteFile(path, []byte("chapter text"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := toolplane.ToolProjectContext{
		Root: toolRoot(t, root),
		Outline: &models.Outline{Parts: []models.Part{{
			ID: "P1",
			Volumes: []models.Volume{{
				ID:       "P1-V2",
				Chapters: []models.Chapter{{ID: chapterID, Title: "Numeric"}},
			}},
		}}},
	}

	resp := toolplane.QueryContext(ctx, "chapter-repair", chapterID, "")
	if !resp.OK || resp.Count != 1 {
		t.Fatalf("chapter repair context = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	bundle, ok := resp.Results.(toolplane.ChapterRepairContext)
	if !ok {
		t.Fatalf("results type = %T, want toolplane.ChapterRepairContext", resp.Results)
	}
	if gotPath := bundle.ExistingChapterExcerpt["content_path"]; gotPath != path {
		t.Fatalf("excerpt path = %q, want %q", gotPath, path)
	}
	if opening := bundle.ExistingChapterExcerpt["opening"]; opening != "chapter text" {
		t.Fatalf("excerpt opening = %q, want chapter text", opening)
	}
}

func containsJSONString(data []byte, needle string) bool {
	return strings.Contains(string(data), `"`+needle+`"`)
}

func TestQueryStorySetupRulesType(t *testing.T) {
	ctx := toolplane.ToolProjectContext{Setup: &models.StorySetup{
		Rules: []string{
			"李侑的日志只能看到系统宿主使用系统时的行为记录",
			"日志由系统自动生成，宿主不能写入或修改",
			"顾恒的模拟器一天只能走一次，多用吃不消",
		},
	}}
	resp := toolplane.QueryStorySetup(ctx, "rules", "")
	if !resp.OK || resp.Count != 3 {
		t.Fatalf("rules all = ok %v count %d warnings %v", resp.OK, resp.Count, resp.Warnings)
	}
	resp = toolplane.QueryStorySetup(ctx, "rule", "模拟器")
	hits, ok := resp.Results.([]string)
	if !ok || resp.Count != 1 || len(hits) != 1 {
		t.Fatalf("rules filtered = ok %v count %d results %T %#v", resp.OK, resp.Count, resp.Results, resp.Results)
	}
	if !strings.Contains(hits[0], "模拟器") {
		t.Fatalf("rules filtered hit = %q", hits[0])
	}
}
