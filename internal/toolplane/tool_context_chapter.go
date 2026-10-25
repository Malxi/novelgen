// The chapter-write and chapter-repair context bundles, including write history, navigation,
// workflow, next-action and simulation-repair-hint builders.

package toolplane

import (
	"fmt"
	"novelgen/internal/contract/story"
	"novelgen/internal/contract/toolcmd"
	"novelgen/internal/fiction/domain/logic/continuity/recap"
	"novelgen/internal/platform/store"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func queryChapterWriteContext(ctx ToolProjectContext, chapterID string) ToolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type chapter-write"}}
	}

	chapters := flattenChapters(ctx.Outline)
	targetIndex := -1
	for i, chapter := range chapters {
		if chapter.Chapter != nil && idMatches(chapter.Chapter.ID, chapterID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: false, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	target := chapters[targetIndex]
	eventsResp := QueryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
	events := limitEventHitResults(eventsResp.Results, 8)
	entities := collectChapterEntities(*target.Chapter)

	var previousChapter *flatChapter
	var nextChapter *flatChapter
	if targetIndex > 0 {
		previousChapter = &chapters[targetIndex-1]
	}
	if targetIndex+1 < len(chapters) {
		nextChapter = &chapters[targetIndex+1]
	}

	var previousRecap *models.ChapterRecap
	var currentRecap *models.ChapterRecap
	warnings := []string{}
	if ctx.Root != nil {
		store := recap.NewStoreAt(ctx.Root)
		if previousChapter != nil && previousChapter.Chapter != nil {
			if loaded, warning := loadUsableToolContextRecap(ctx.Root, store, previousChapter.Chapter); loaded != nil {
				previousRecap = loaded
			} else if warning != "" {
				warnings = append(warnings, "previous recap ignored: "+warning)
			}
		}
		if loaded, warning := loadUsableToolContextRecap(ctx.Root, store, target.Chapter); loaded != nil {
			currentRecap = loaded
		} else if warning != "" {
			warnings = append(warnings, "current recap ignored: "+warning)
		}
	}

	excerpt, _ := loadChapterExcerptForTool(ctx.Root, target.Chapter.ID)
	creativeHistory := buildChapterWriteHistory(ctx.Root)
	narrativeMemory, memoryWarnings, memoryErr := queryNarrativeMemory(ctx.Root, memoryQueryOptions{
		ChapterID: target.Chapter.ID,
		Intent:    "chapter-write",
		MaxChars:  4000,
		Limit:     6,
	})
	warnings = append(warnings, memoryWarnings...)
	if memoryErr != nil {
		warnings = append(warnings, memoryUnavailableWarning(memoryErr))
	}
	result := ChapterWriteContext{
		ChapterID:              target.Chapter.ID,
		Path:                   target.Path,
		StorySetup:             briefStorySetupResults(ctx.Setup, true),
		TargetChapter:          makeRepairChapterBrief(*target.Chapter),
		PreviousRecap:          makeRecapBrief(previousRecap),
		CurrentRecap:           makeRecapBrief(currentRecap),
		ExistingChapterExcerpt: excerpt,
		CreativeHistory:        creativeHistory,
		NarrativeMemory:        narrativeMemory,
		EntityIndex:            entities,
		Events:                 briefOutlineResults(events, false),
		Navigation:             buildChapterWriteNavigation(target.Chapter.ID, previousChapter, nextChapter, entities),
		Workflow:               buildChapterWriteWorkflow(target.Chapter.ID, previousChapter),
		NextActions:            buildChapterWriteNextActions(target.Chapter.ID, previousChapter),
		Stats: map[string]int{
			"chapter_index":   targetIndex,
			"chapter_count":   len(chapters),
			"event_count":     eventsResp.Count,
			"returned_events": len(events),
			"character_count": len(entities.Characters),
			"item_count":      len(entities.Items),
			"location_count":  len(entities.Locations),
			"storyline_count": len(entities.Storylines),
		},
	}
	if parent := ctx.Outline.GetVolumeByID(target.Path.VolumeID); parent != nil {
		result.ParentVolume = makeContextVolumeBrief(*parent, false)
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		result.PreviousChapter = makeContextChapterBrief(*previousChapter.Chapter)
		result.Stats["has_previous_chapter"] = 1
		if previousRecap != nil {
			result.Stats["has_previous_recap"] = 1
		}
	}
	if nextChapter != nil && nextChapter.Chapter != nil {
		result.NextChapter = makeContextChapterBrief(*nextChapter.Chapter)
		result.Stats["has_next_chapter"] = 1
	}
	if currentRecap != nil {
		result.Stats["has_current_recap"] = 1
	}
	if excerpt != nil {
		result.Stats["excerpt_fields"] = len(excerpt)
	}
	if creativeHistory != nil {
		result.Stats["history_prompt_count"] = creativeHistory.Counts["prompts"]
		result.Stats["history_response_count"] = creativeHistory.Counts["responses"]
		result.Stats["history_recent_prompts"] = len(creativeHistory.RecentPrompts)
		result.Stats["history_recent_responses"] = len(creativeHistory.RecentResponses)
	}
	if narrativeMemory != nil {
		result.Stats["narrative_memory_scenes"] = len(narrativeMemory.Retrieved)
		result.Stats["narrative_memory_promises"] = len(narrativeMemory.Promises)
		result.Stats["narrative_memory_chars"] = narrativeMemory.Meta.ReturnedChars
	}

	if eventsResp.Count > len(events) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", eventsResp.Count, len(events)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func buildChapterWriteHistory(root *store.Root) *chapterWriteHistory {
	if root.Dir() == "" {
		return nil
	}
	logsRoot := root.LogsDir()
	if !root.IsDir(logsRoot) {
		return nil
	}
	counts := map[string]int{}
	responses := []chapterWriteHistoryEntry{}
	prompts := []chapterWriteHistoryEntry{}
	_ = root.Walk(logsRoot, func(path string, info os.FileInfo) error {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(logsRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		kind := logKindFromRel(rel)
		if kind != "prompts" && kind != "responses" {
			return nil
		}
		if !info.ModTime().Before(ToolProcessStartedAt.Add(-1 * time.Second)) {
			return nil
		}
		counts[kind]++
		preview, truncated := readLogPreviewForQuery(root.FS(), path, kind, 700)
		if strings.TrimSpace(preview) == "" {
			return nil
		}
		item := chapterWriteHistoryEntry{
			ID:         rel,
			Kind:       kind,
			Agent:      logAgentFromFilename(filepath.Base(path)),
			ModifiedAt: info.ModTime(),
			Preview:    preview,
			Truncated:  truncated,
		}
		if kind == "responses" {
			responses = append(responses, item)
		} else {
			prompts = append(prompts, item)
		}
		return nil
	})
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].ModifiedAt.After(responses[j].ModifiedAt)
	})
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].ModifiedAt.After(prompts[j].ModifiedAt)
	})
	responses = firstNHistoryEntries(responses, 3)
	prompts = firstNHistoryEntries(prompts, 2)
	if len(responses) == 0 && len(prompts) == 0 {
		return nil
	}
	return &chapterWriteHistory{
		Counts:          counts,
		RecentResponses: responses,
		RecentPrompts:   prompts,
		Guidance: []string{
			"这些最近日志预览只作为风格与历史连续性感知；事实仍以 story_setup、target_chapter、events、recaps 和 outline context 为准。",
			"不要在正文中引用或总结日志内容；write generation 阶段不要再运行额外 log query。",
		},
	}
}

func firstNHistoryEntries(entries []chapterWriteHistoryEntry, n int) []chapterWriteHistoryEntry {
	if n <= 0 || len(entries) == 0 {
		return nil
	}
	if len(entries) <= n {
		return entries
	}
	return entries[:n]
}

func buildChapterWriteNavigation(chapterID string, previousChapter, nextChapter *flatChapter, entities OutlineVolumeEntities) map[string]interface{} {
	adjacent := map[string]string{}
	if previousChapter != nil && previousChapter.Chapter != nil {
		adjacent["previous_chapter_id"] = previousChapter.Chapter.ID
	}
	if nextChapter != nil && nextChapter.Chapter != nil {
		adjacent["next_chapter_id"] = nextChapter.Chapter.ID
	}
	return map[string]interface{}{
		"target_chapter_id": chapterID,
		"adjacent":          adjacent,
		"entity_names": map[string][]string{
			"characters": firstNStrings(entities.Characters, 5),
			"items":      firstNStrings(entities.Items, 5),
			"locations":  firstNStrings(entities.Locations, 4),
		},
		"tool_policy": []string{
			"chapter-write context already contains the focused facts required for generation.",
			"Do not run extra tool queries, checks, refreshes, patch commands, shell fallbacks, or file reads in write generation.",
			"Return final JSON content to Go; Go handles saving, recap, checks, and derived DSL refresh.",
		},
	}
}

func buildChapterWriteWorkflow(chapterID string, previousChapter *flatChapter) map[string]interface{} {
	stopRules := []string{
		"Use this focused bundle as the complete generation context.",
		"Do not query full setup, full outline, logs, source files, or chapter collections for write generation.",
		"Do not run patch/check/refresh commands in write generation; chapter prose JSON is returned to Go for validation and saving.",
		"Go handles recap generation, quality checks, simulation checks, and derived DSL refresh after saving.",
	}
	workflow := map[string]interface{}{
		"goal":              "Write one chapter using compact outline, adjacent continuity, recaps, events, entity names, and creative_history.",
		"target_chapter_id": chapterID,
		"stop_rules":        stopRules,
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		workflow["previous_chapter_id"] = previousChapter.Chapter.ID
	}
	return workflow
}

func buildChapterWriteNextActions(chapterID string, previousChapter *flatChapter) []ToolNextAction {
	actions := []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_context",
			Purpose: "Read target_chapter, parent_volume, adjacent chapters, recap briefs, entity_index, events, and existing_chapter_excerpt before any extra query.",
		},
		{
			Step:    2,
			Action:  "return_final_json",
			Purpose: "Write the chapter content from the bundled facts and return the schema JSON to Go.",
			When:    "After reading this context; no extra tools are needed for write generation.",
		},
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "preserve_previous_continuity",
			Purpose: "Use previous_chapter and previous_recap facts already included in this context; do not run recap checks.",
			When:    "The target opening depends on previous chapter continuity.",
		})
	}
	return actions
}

func queryChapterRepairContext(ctx ToolProjectContext, chapterID, issueCategory string) ToolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type chapter-repair"}}
	}

	chapters := flattenChapters(ctx.Outline)
	targetIndex := -1
	for i, chapter := range chapters {
		if chapter.Chapter != nil && idMatches(chapter.Chapter.ID, chapterID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: false, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	target := chapters[targetIndex]
	eventsResp := QueryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
	events := limitEventHitResults(eventsResp.Results, 8)
	entities := collectChapterEntities(*target.Chapter)

	var previousChapter *flatChapter
	var nextChapter *flatChapter
	if targetIndex > 0 {
		previousChapter = &chapters[targetIndex-1]
	}
	if targetIndex+1 < len(chapters) {
		nextChapter = &chapters[targetIndex+1]
	}

	warnings := []string{}
	var check *ToolCheckResult
	if ctx.Dir() == "" {
		warnings = append(warnings, "project root missing; chapter quality/simulation check unavailable")
	} else {
		var err error
		check, err = RunToolChapterCheckWithTargetWordsAt(ctx.Root, "all", "chapter", target.Chapter.ID, 0)
		if err != nil {
			warnings = append(warnings, "chapter check unavailable: "+err.Error())
		} else if err := ApplyToolCheckIssueFilters(check, "low", ChapterRepairIssueCategoryFilter(issueCategory), 12); err != nil {
			warnings = append(warnings, "chapter check filter ignored: "+err.Error())
		}
	}

	excerpt, excerptWarnings := loadChapterExcerptForTool(ctx.Root, target.Chapter.ID)
	warnings = append(warnings, excerptWarnings...)
	narrativeMemory, memoryWarnings, memoryErr := queryNarrativeMemory(ctx.Root, memoryQueryOptions{
		ChapterID: target.Chapter.ID,
		Intent:    "continuity",
		MaxChars:  2500,
		Limit:     4,
	})
	warnings = append(warnings, memoryWarnings...)
	if memoryErr != nil {
		warnings = append(warnings, memoryUnavailableWarning(memoryErr))
	}

	result := ChapterRepairContext{
		ChapterID:              target.Chapter.ID,
		Path:                   target.Path,
		IssueCategory:          categoryOrAll(issueCategory),
		Check:                  check,
		SimulationRepairHints:  BuildChapterSimulationRepairHints(target.Chapter.ID, check),
		TargetChapter:          makeRepairChapterBrief(*target.Chapter),
		ExistingChapterExcerpt: excerpt,
		NarrativeMemory:        narrativeMemory,
		EntityIndex:            entities,
		Events:                 briefOutlineResults(events, false),
		Navigation:             BuildChapterRepairNavigation(target.Chapter.ID, issueCategory, entities),
		Workflow:               buildChapterRepairWorkflow(target.Chapter.ID, issueCategory),
		NextActions:            buildChapterRepairNextActions(target.Chapter.ID, issueCategory),
		Stats: map[string]int{
			"chapter_index":   targetIndex,
			"chapter_count":   len(chapters),
			"event_count":     eventsResp.Count,
			"returned_events": len(events),
			"character_count": len(entities.Characters),
			"item_count":      len(entities.Items),
			"location_count":  len(entities.Locations),
			"storyline_count": len(entities.Storylines),
		},
	}
	if parent := ctx.Outline.GetVolumeByID(target.Path.VolumeID); parent != nil {
		result.ParentVolume = makeContextVolumeBrief(*parent, false)
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		result.PreviousChapter = makeContextChapterBrief(*previousChapter.Chapter)
		result.Stats["has_previous_chapter"] = 1
	}
	if nextChapter != nil && nextChapter.Chapter != nil {
		result.NextChapter = makeContextChapterBrief(*nextChapter.Chapter)
		result.Stats["has_next_chapter"] = 1
	}
	if excerpt != nil {
		result.Stats["excerpt_fields"] = len(excerpt)
	}
	if check != nil {
		result.Stats["check_issues"] = check.Summary.Total
		result.Stats["returned_check_issues"] = len(check.Issues)
	}
	if narrativeMemory != nil {
		result.Stats["narrative_memory_scenes"] = len(narrativeMemory.Retrieved)
		result.Stats["narrative_memory_promises"] = len(narrativeMemory.Promises)
	}
	if eventsResp.Count > len(events) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", eventsResp.Count, len(events)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func BuildChapterRepairNavigation(chapterID, issueCategory string, entities OutlineVolumeEntities) map[string]interface{} {
	category := ChapterRepairIssueCategoryFilter(issueCategory)
	craftQueries := []string{}
	for _, name := range firstNStrings(entities.Characters, 5) {
		craftQueries = append(craftQueries, toolcmd.QueryContext(toolcmd.QueryOptions{Type: "craft-character", Name: name, View: "brief"}))
	}
	for _, name := range firstNStrings(entities.Items, 5) {
		craftQueries = append(craftQueries, toolcmd.QueryContext(toolcmd.QueryOptions{Type: "craft-item", Name: name, View: "brief"}))
	}
	for _, name := range firstNStrings(entities.Locations, 4) {
		craftQueries = append(craftQueries, toolcmd.QueryContext(toolcmd.QueryOptions{Type: "craft-location", Name: name, View: "brief"}))
	}
	return map[string]interface{}{
		"detail_queries": []string{
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-write", ID: chapterID, View: "brief"}),
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: chapterID, View: "brief"}),
		},
		"craft_context_queries":     craftQueries,
		"quality_check_query":       toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "quality", Category: category, MinPriority: "low", MaxIssues: 12}),
		"simulation_check_query":    toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "simulation", Category: category, MinPriority: "low", MaxIssues: 12}),
		"all_check_query":           toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Category: category, MinPriority: "low", MaxIssues: 12}),
		"refresh_chapter_dsl_query": toolcmd.ChapterDSLRefresh(chapterID),
		"repair_command":            fmt.Sprintf("novelgen write improve --agent-sdk --chapter %q --max-rounds 1", chapterID),
	}
}

func ChapterRepairIssueCategoryFilter(issueCategory string) string {
	category := strings.TrimSpace(issueCategory)
	switch NormalizeKey(category) {
	case "", "all", "simulation":
		return categoryOrAll("") + ",simulation"
	default:
		return category
	}
}

func BuildChapterSimulationRepairHints(chapterID string, check *ToolCheckResult) []simulationRepairHint {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	hints := make([]simulationRepairHint, 0, 2)
	added := map[string]bool{}
	for _, issue := range check.Issues {
		text := simulationIssueText(issue)
		if text == "" {
			continue
		}
		if isCombatBalanceSimulationIssue(text) {
			if !added["combat_balance"] {
				hints = append(hints, combatBalanceSimulationRepairHint(chapterID, issue))
				added["combat_balance"] = true
			}
		}
		if isCombatResultSimulationIssue(text) {
			if !added["combat_result"] {
				hints = append(hints, combatResultSimulationRepairHint(chapterID, issue))
				added["combat_result"] = true
			}
		}
	}
	return hints
}

func simulationIssueText(issue models.ReviewSuggestion) string {
	return strings.ToLower(strings.TrimSpace(strings.Join([]string{
		issue.Category,
		issue.Issue,
		issue.Suggestion,
	}, " ")))
}

func isCombatBalanceSimulationIssue(text string) bool {
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"战斗难度",
		"敌人有效战力",
		"主角基础战力",
		"combat balance",
		"enemy effective power",
		"protagonist power",
		"missing_repair_signals=power_change",
		"missing_repair_signals=breakthrough",
		"missing_repair_signals=gene",
		"missing_repair_signals=mech",
		"missing_repair_signals=equipment_or_item",
		"missing_repair_signals=ally",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	if strings.Contains(text, "missing_repair_signals=") {
		for _, marker := range []string{"power_change", "breakthrough", "gene", "mech", "equipment_or_item", "ally"} {
			if strings.Contains(text, marker) {
				return true
			}
		}
	}
	return false
}

func isCombatResultSimulationIssue(text string) bool {
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"叙事性结果",
		"战斗结果",
		"narrative result",
		"combat_result=false",
		"combat_result_on_complete",
		"missing combat result",
		"on_complete narration/result",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func combatBalanceSimulationRepairHint(chapterID string, issue models.ReviewSuggestion) simulationRepairHint {
	return simulationRepairHint{
		IssueType:     "combat_balance",
		AppliesWhen:   "RPG simulation says protagonist power plus structured/tactical/ally bonuses is still below enemy effective power.",
		SimulatorRule: "Enemy power is driven by combat.enemies id/count/level/elite/boss and enemy damage-state text. Protagonist side uses base power plus state_delta growth/gene/mech/equipment/item/ally, then tactical text signals such as 机甲/火种/装甲, 伏击/陷阱/地形/高地/狭道, 三方/混战/互相消耗.",
		ProseLevers: []string{
			"Reduce the same-scene enemy count, level, elite/boss framing, or simultaneous engagement scale if the outline allows it.",
			"Make the fight non-frontal: split enemies, ambush, trap, terrain bottleneck, high ground, narrow passage, third-party consumption, or enemy already damaged.",
			"Explicitly show supported power sources already allowed by setup/outline: mech, fire-core, armor, equipment, item, gene breakthrough, power increase, or ally assistance.",
			"If the protagonist gains strength in this chapter, write a concrete breakthrough/power-change moment before or during the decisive exchange.",
		},
		DSLSignals: []string{
			`combat { enemies = [{id = "enemy_x", count = 1, level = 1}] } must reflect the actual current enemy scale.`,
			`state_delta { target = "char_protagonist" kind = "power_change" delta = 50 note = "..." } for explicit combat-power gains.`,
			`state_delta { target = "char_protagonist" kind = "gene" field = "level|stability" to = "..." note = "..." } for gene progression.`,
			`state_delta { target = "char_protagonist" kind = "mech" field = "form|energy|module|damage" to = "..." note = "..." } for mech state.`,
			`state_delta { target = "item_id" kind = "item|equipment" field = "key_item" to = "..." note = "..." } for usable equipment/items.`,
			`state_delta { target = "char_protagonist" kind = "ally" to = "Ally Name" note = "..." } for concrete ally support.`,
			`on_complete { narration = "..." } should state the combat consequence, not just that the fight happened.`,
		},
		ValidationPlan: []string{
			toolcmd.ChapterDSLRefresh(chapterID),
			toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "simulation", MinPriority: "low", MaxIssues: 12}),
			toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
		},
		Avoid: []string{
			"Do not only add adjectives, longer action choreography, or courage lines; the simulator needs count/level/support/tactic/state-result signals.",
			"Do not invent a new ally, weapon, or breakthrough that contradicts setup, outline, craft, or adjacent chapters.",
			"Do not lower stakes by silently deleting the fight; preserve the chapter purpose and visible consequence.",
		},
		FocusedQueries: []string{
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-write", ID: chapterID, View: "brief"}),
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: chapterID, Fields: "action,actor,target,target_type,details,result", View: "brief"}),
		},
		RelatedIssue:      issue.Issue,
		RelatedSuggestion: issue.Suggestion,
	}
}

func combatResultSimulationRepairHint(chapterID string, issue models.ReviewSuggestion) simulationRepairHint {
	return simulationRepairHint{
		IssueType:     "combat_result",
		AppliesWhen:   "RPG simulation sees a combat event without a concrete on_complete narration/result.",
		SimulatorRule: "A combat step needs an on_complete narration or result so simulation can see what changed after the fight.",
		ProseLevers: []string{
			"End the exchange with a visible consequence: injury, retreat, loot, clue, damaged gear, power insight, relationship shift, or tactical position change.",
			"Tie the result to the next paragraph or scene so it is not a detached summary line.",
		},
		DSLSignals: []string{
			`on_complete { narration = "The fight ends with a concrete consequence." }`,
			`state_delta { target = "..." kind = "injury|resource|power_change|plot_thread|transition" ... } when the consequence changes durable state.`,
		},
		ValidationPlan: []string{
			toolcmd.ChapterDSLRefresh(chapterID),
			toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "simulation", MinPriority: "low", MaxIssues: 12}),
		},
		Avoid: []string{
			"Do not add a vague victory sentence that changes no state.",
			"Do not create rewards or injuries that adjacent chapter continuity cannot support.",
		},
		FocusedQueries: []string{
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-write", ID: chapterID, View: "brief"}),
		},
		RelatedIssue:      issue.Issue,
		RelatedSuggestion: issue.Suggestion,
	}
}

func buildChapterRepairWorkflow(chapterID, issueCategory string) map[string]interface{} {
	return map[string]interface{}{
		"goal":              "Repair one saved final chapter after tool check reported deterministic prose or RPG simulation issues.",
		"target_chapter_id": chapterID,
		"issue_category":    categoryOrAll(issueCategory),
		"steps": []string{
			"Start from this compact repair bundle instead of querying full setup, full outline, or all chapter files.",
			"Use check.issues and existing_chapter_excerpt to identify the smallest prose change that can fix the reported issue.",
			"If simulation_repair_hints is present, choose one or more prose levers that are supported by the outline and make their DSL signals extractable.",
			"Use craft_context_queries only for entities directly involved in the issue.",
			"Return improved content to the write workflow; do not write chapter markdown directly from an Agent SDK tool.",
			"After Go saves the chapter, refresh derived chapter RPG DSL when allowed, then rerun the all_check_query and fix remaining blocking issues. Simulation checks do not invoke LLM conversion; use post_repair_refresh_query first when chapter prose changed.",
		},
		"post_repair_refresh_query": toolcmd.ChapterDSLRefresh(chapterID),
		"post_repair_check_query":   toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
	}
}

func buildChapterRepairNextActions(chapterID, issueCategory string) []ToolNextAction {
	category := ChapterRepairIssueCategoryFilter(issueCategory)
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_repair_bundle",
			Purpose: "Start from the focused repair facts already returned in this response before querying more context.",
		},
		{
			Step:    2,
			Action:  "query_craft_only_if_needed",
			Purpose: "Use navigation.craft_context_queries only for entities directly involved in the failing issue.",
			When:    "The repair depends on a character/item/location fact not already in this bundle.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: toolcmd.PatchChapter(chapterID),
			Purpose: "Validate complete revised prose before returning it or applying it.",
		},
		{
			Step:    4,
			Action:  "refresh_derived_dsl",
			Command: toolcmd.ChapterDSLRefresh(chapterID),
			Purpose: "Regenerate derived RPG DSL after prose changes; simulation checks do not invoke LLM conversion.",
			When:    "After chapter prose has changed and refresh is allowed by the workflow.",
		},
		{
			Step:    5,
			Action:  "post_repair_check",
			Command: toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Category: category, MinPriority: "low", MaxIssues: 12}),
			Purpose: "Verify the targeted repair and catch remaining deterministic prose or simulation issues.",
		},
	}
}

func briefChapterNavigationResults(values []chapterHit) interface{} {
	out := make([]map[string]interface{}, 0, len(values))
	for _, hit := range values {
		item := map[string]interface{}{
			"id":      hit.ID,
			"title":   hit.Title,
			"summary": clipToolString(hit.Summary, 180),
			"path":    hit.Path,
		}
		if len(hit.Reasons) > 0 {
			item["reasons"] = hit.Reasons
		}
		out = append(out, item)
	}
	return out
}
