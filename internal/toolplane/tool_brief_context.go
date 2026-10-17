// Context-bundle briefs: the route-only index of each workflow context bundle and the next-actions
// block that keeps an agent on the shortest path.

package toolplane

import (
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/contract/toolcmd"
)

func briefContextResults(results interface{}, indexOnly bool) interface{} {
	if indexOnly {
		return indexContextResults(results)
	}
	switch typed := results.(type) {
	case ChapterRepairContext:
		typed.ParentVolume = stripContextVolumePayoff(typed.ParentVolume)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case ChapterWriteContext:
		typed.ParentVolume = stripContextVolumePayoff(typed.ParentVolume)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case OutlineRepairContext:
		typed.ParentVolume = stripContextVolumePayoff(typed.ParentVolume)
		typed.Current = stripContextVolumePayoff(typed.Current)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case OutlineGlobalRepairContext:
		typed.StorySetup = nil
		typed.Outline = nil
		if len(typed.MysteryThreads) > 12 {
			typed.MysteryThreads = typed.MysteryThreads[:12]
		}
		return typed
	case OutlineVolumeContext:
		typed.TargetVolume = stripContextVolumePayoff(typed.TargetVolume)
		typed.PreviousVolume = stripContextVolumePayoff(typed.PreviousVolume)
		typed.NextVolume = stripContextVolumePayoff(typed.NextVolume)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case CraftElementContext:
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	default:
		return results
	}
}

func indexContextResults(results interface{}) interface{} {
	switch typed := results.(type) {
	case ChapterRepairContext:
		return map[string]interface{}{
			"chapter_id":     typed.ChapterID,
			"path":           typed.Path,
			"issue_category": typed.IssueCategory,
			"check":          toolCheckIndex(typed.Check),
			"navigation":     typed.Navigation,
			"workflow":       toolWorkflowIndex(typed.Workflow),
			"next_actions":   chapterRepairIndexNextActions(typed),
			"stats":          typed.Stats,
			"entity_index":   typed.EntityIndex,
		}
	case ChapterWriteContext:
		return map[string]interface{}{
			"chapter_id":   typed.ChapterID,
			"path":         typed.Path,
			"navigation":   typed.Navigation,
			"workflow":     toolWorkflowIndex(typed.Workflow),
			"next_actions": chapterWriteIndexNextActions(typed),
			"stats":        typed.Stats,
			"entity_index": typed.EntityIndex,
		}
	case RecapRepairContext:
		return map[string]interface{}{
			"chapter_id":   typed.ChapterID,
			"path":         typed.Path,
			"check":        toolCheckIndex(typed.Check),
			"navigation":   typed.Navigation,
			"workflow":     toolWorkflowIndex(typed.Workflow),
			"next_actions": recapRepairIndexNextActions(typed),
			"stats":        typed.Stats,
		}
	case OutlineRepairContext:
		return map[string]interface{}{
			"scope":          typed.Scope,
			"id":             typed.ID,
			"path":           typed.Path,
			"issue_category": typed.IssueCategory,
			"check":          toolCheckIndex(typed.Check),
			"issue_context":  outlineRepairIssueIndex(typed.IssueContext),
			"navigation":     typed.Navigation,
			"workflow":       toolWorkflowIndex(typed.Workflow),
			"next_actions":   outlineRepairIndexNextActions(typed),
			"stats":          typed.Stats,
		}
	case OutlineGlobalRepairContext:
		return map[string]interface{}{
			"issue_category": typed.IssueCategory,
			"check":          toolCheckIndex(typed.Check),
			"issue_context":  outlineGlobalRepairIssueIndex(typed.IssueContext),
			"patch_task":     typed.PatchTask,
			"navigation":     typed.Navigation,
			"workflow":       toolWorkflowIndex(typed.Workflow),
			"next_actions":   outlineGlobalRepairIndexNextActions(typed),
			"stats":          typed.Stats,
		}
	case OutlineVolumeContext:
		return map[string]interface{}{
			"volume_id":    typed.VolumeID,
			"path":         typed.Path,
			"navigation":   typed.Navigation,
			"next_actions": typed.NextActions,
			"stats":        typed.Stats,
			"entity_index": indexOutlineVolumeEntities(typed.EntityIndex),
		}
	case CraftElementContext:
		return map[string]interface{}{
			"target":       typed.Target,
			"name":         typed.Name,
			"navigation":   typed.Navigation,
			"next_actions": craftElementIndexNextActions(typed),
			"stats":        typed.Stats,
		}
	default:
		return results
	}
}

func chapterWriteIndexNextActions(ctx ChapterWriteContext) []ToolNextAction {
	out := []ToolNextAction{{
		Step:    1,
		Action:  "query_brief_context",
		Purpose: "Fetch the focused write bundle before drafting; index view is only the route map.",
		Command: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-write", ID: ctx.ChapterID, View: "brief"}),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_context" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func chapterRepairIndexNextActions(ctx ChapterRepairContext) []ToolNextAction {
	if toolCheckResultIsClean(ctx.Check) {
		return cleanFocusedCheckNextActions(
			toolcmd.ChapterCheck(ctx.ChapterID, toolcmd.ChapterCheckOptions{Category: categoryOrAll(ctx.IssueCategory), MinPriority: "low", MaxIssues: 12}),
			"The scoped final-chapter check returned zero issues; do not fetch brief repair context or patch prose.",
		)
	}
	if actions := refreshFirstIndexNextActions(ctx.Check); len(actions) > 0 {
		return actions
	}
	out := []ToolNextAction{{
		Step:    1,
		Action:  "query_brief_repair_context",
		Purpose: "Fetch the focused repair bundle before patching; index view is only the route map and check summary.",
		Command: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-repair", ID: ctx.ChapterID, Name: categoryOrAll(ctx.IssueCategory), View: "brief"}),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_repair_bundle" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func outlineRepairIndexNextActions(ctx OutlineRepairContext) []ToolNextAction {
	if toolCheckResultIsClean(ctx.Check) {
		return cleanFocusedCheckNextActions(
			toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: ctx.Scope, ID: ctx.ID, Category: categoryOrAll(ctx.IssueCategory), MinPriority: "low", MaxIssues: 8}),
			"The scoped outline check returned zero issues; do not fetch brief repair context or patch outline.",
		)
	}
	if actions := refreshFirstIndexNextActions(ctx.Check); len(actions) > 0 {
		return actions
	}
	out := []ToolNextAction{{
		Step:    1,
		Action:  "query_brief_repair_context",
		Purpose: "Fetch the focused outline repair bundle before patching; index view is only the route map and check summary.",
		Command: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: ctx.ID, Name: categoryOrAll(ctx.IssueCategory), View: "brief"}),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_repair_bundle" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func cleanFocusedCheckNextActions(checkCommand, purpose string) []ToolNextAction {
	return []ToolNextAction{{
		Step:    1,
		Action:  "focused_check_clean",
		Purpose: purpose,
		Command: checkCommand,
	}, {
		Step:    2,
		Action:  "return_final_json",
		Purpose: "Report that the focused check is clean and stop tool exploration.",
	}}
}

func refreshFirstIndexNextActions(check *ToolCheckResult) []ToolNextAction {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	actions := toolCheckNextActions(*check)
	if len(actions) == 0 || actions[0].Action != "refresh_derived_dsl_first" {
		return nil
	}
	out := make([]ToolNextAction, len(actions))
	copy(out, actions)
	for i := range out {
		out[i].Step = i + 1
	}
	return out
}

func outlineGlobalRepairIndexNextActions(ctx OutlineGlobalRepairContext) []ToolNextAction {
	if !outlineRepairIssuesHavePatchRoute(ctx.IssueContext) {
		out := []ToolNextAction{{
			Step:    1,
			Action:  "classify_unpatchable_global_issue",
			Purpose: "No returned global issue has both patch_query and patch_shape; do not fetch brief context or invent a broad patch.",
		}}
		if category := strings.TrimSpace(ctx.IssueCategory); category != "" {
			out = append(out, ToolNextAction{
				Step:    len(out) + 1,
				Action:  "focused_recheck",
				Purpose: "Confirm whether the global diagnostic still reproduces before routing to a smaller workflow.",
				Command: toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "all", Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 12}),
				When:    "Only when you need to confirm the diagnostic; otherwise return final JSON with remaining issue notes.",
			})
		}
		out = append(out, ToolNextAction{
			Step:    len(out) + 1,
			Action:  "return_final_json",
			Purpose: "Report the issue as remaining/unpatchable from this route, or recommend a smaller targeted workflow.",
		})
		return out
	}
	if ctx.PatchTask == nil {
		out := []ToolNextAction{{
			Step:    1,
			Action:  "query_brief_global_repair_context",
			Purpose: "Fetch the focused global repair bundle before patching because this index response has no complete patch_task.",
			Command: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-global-repair", Name: categoryOrAll(ctx.IssueCategory), View: "brief"}),
		}}
		for _, action := range ctx.NextActions {
			if action.Action == "use_patch_task" || action.Action == "use_current_global_repair_bundle" {
				continue
			}
			action.Step = len(out) + 1
			out = append(out, action)
		}
		return out
	}
	out := []ToolNextAction{{
		Step:    1,
		Action:  "use_patch_task",
		Purpose: "Use patch_task.dry_run_command and patch_task.apply_command from this index response directly. Do not query brief context unless patch_task is absent.",
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_patch_task" {
			continue
		}
		if ctx.PatchTask != nil {
			switch action.Action {
			case "patch_dry_run":
				action.Command = ctx.PatchTask.DryRunCommand
			case "patch_apply":
				action.Command = ctx.PatchTask.ApplyCommand
			}
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func outlineRepairIssuesHavePatchRoute(issues []OutlineRepairIssue) bool {
	for _, issue := range issues {
		if strings.TrimSpace(issue.PatchQuery) == "" || issue.PatchShape == nil {
			continue
		}
		return true
	}
	return false
}

func recapRepairIndexNextActions(ctx RecapRepairContext) []ToolNextAction {
	if toolCheckResultIsClean(ctx.Check) {
		return cleanFocusedCheckNextActions(
			toolcmd.Check(toolcmd.CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: ctx.ChapterID, MinPriority: "low", MaxIssues: 8}),
			"The scoped recap check returned zero issues; do not fetch brief repair context or patch recap.",
		)
	}
	out := []ToolNextAction{{
		Step:    1,
		Action:  "query_brief_repair_context",
		Purpose: "Fetch the focused recap repair bundle before patching; index view is only the route map and check summary.",
		Command: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: ctx.ChapterID, View: "brief"}),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_context" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func craftElementIndexNextActions(ctx CraftElementContext) []ToolNextAction {
	out := []ToolNextAction{{
		Step:    1,
		Action:  "query_brief_context",
		Purpose: "Fetch the focused craft bundle before patching; index view is only the route map and counts.",
		Command: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "craft-" + ctx.Target, Name: ctx.Name, View: "brief"}),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_context" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func indexOutlineVolumeEntities(entities OutlineVolumeEntities) OutlineVolumeEntities {
	return OutlineVolumeEntities{
		Characters: firstNStrings(entities.Characters, 12),
		Items:      firstNStrings(entities.Items, 12),
		Locations:  firstNStrings(entities.Locations, 10),
		Storylines: firstNStrings(entities.Storylines, 8),
	}
}

func outlineRepairIssueIndex(issues []OutlineRepairIssue) []map[string]interface{} {
	if len(issues) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		out = append(out, map[string]interface{}{
			"category":               issue.Category,
			"target_id":              issue.TargetID,
			"target_name":            issue.TargetName,
			"priority":               issue.Priority,
			"focused_check_query":    issue.FocusedCheckQuery,
			"repair_context_query":   issue.RepairContextQuery,
			"patch_query":            issue.PatchQuery,
			"patch_shape":            issue.PatchShape,
			"post_patch_check_query": issue.PostPatchCheckQuery,
		})
	}
	return out
}

func outlineGlobalRepairIssueIndex(issues []OutlineRepairIssue) []map[string]interface{} {
	if len(issues) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		item := map[string]interface{}{
			"category":            issue.Category,
			"target_id":           issue.TargetID,
			"target_name":         issue.TargetName,
			"priority":            issue.Priority,
			"focused_check_query": issue.FocusedCheckQuery,
			"patch_query":         issue.PatchQuery,
			"patch_shape":         issue.PatchShape,
		}
		if strings.TrimSpace(issue.PatchQuery) != "" && issue.PatchShape != nil {
			item["repair_context_query"] = issue.RepairContextQuery
			item["post_patch_check_query"] = issue.PostPatchCheckQuery
		}
		out = append(out, item)
	}
	return out
}

func toolCheckIndex(check *ToolCheckResult) interface{} {
	if check == nil {
		return nil
	}
	return map[string]interface{}{
		"kind":     check.Kind,
		"target":   check.Target,
		"scope":    check.Scope,
		"id":       check.ID,
		"ok":       check.OK,
		"blocking": check.Blocking,
		"score":    check.Score,
		"summary":  check.Summary,
	}
}

func toolWorkflowIndex(workflow map[string]interface{}) map[string]interface{} {
	if len(workflow) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	for key, value := range workflow {
		normalized := NormalizeKey(key)
		if normalized == "steps" || normalized == "stop_rules" {
			continue
		}
		if strings.HasSuffix(normalized, "_query") ||
			strings.Contains(normalized, "query") ||
			normalized == "goal" ||
			normalized == "scope" ||
			normalized == "target_id" ||
			normalized == "target_chapter_id" ||
			normalized == "issue_category" ||
			normalized == "patch_shape" ||
			normalized == "patch_query" ||
			normalized == "parent_volume_id" ||
			normalized == "agent_rule" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stripContextVolumePayoff(value interface{}) interface{} {
	switch typed := value.(type) {
	case VolumeBrief:
		typed.PayoffContract = nil
		return typed
	default:
		return value
	}
}

func stripContextEventPaths(value interface{}) interface{} {
	events, ok := value.([]EventHitBrief)
	if !ok {
		return value
	}
	out := make([]EventHitBrief, 0, len(events))
	for _, event := range events {
		event.Path = nil
		out = append(out, event)
	}
	return out
}

func makeContextChapterBrief(chapter models.Chapter) ChapterBrief {
	return ChapterBrief{
		ID:           chapter.ID,
		Title:        chapter.Title,
		Summary:      clipToolString(chapter.Summary, 220),
		Characters:   clipToolStrings(chapter.Characters, 12, 50),
		Location:     clipToolString(chapter.Location, 90),
		StateChange:  clipToolString(chapter.StateChange, 120),
		EventCount:   len(chapter.Events),
		AdvanceCount: len(chapter.StorylineAdvances),
	}
}

func makeRepairChapterBrief(chapter models.Chapter) ChapterBrief {
	brief := ChapterBrief{
		ID:                chapter.ID,
		Title:             chapter.Title,
		Summary:           clipToolString(chapter.Summary, 360),
		Characters:        clipToolStrings(chapter.Characters, 12, 50),
		Location:          clipToolString(chapter.Location, 90),
		OpeningBeat:       clipToolString(chapter.OpeningBeat, 160),
		ClosingBeat:       clipToolString(chapter.ClosingBeat, 160),
		StateChange:       clipToolString(chapter.StateChange, 180),
		Conflict:          clipToolString(chapter.Conflict, 200),
		Pacing:            clipToolString(chapter.Pacing, 120),
		Timeline:          chapter.Timeline,
		StateAnchor:       chapter.StateAnchor,
		StorylineAdvances: compactStorylineAdvances(chapter.StorylineAdvances, 4),
		ChapterPayoff:     chapter.ChapterPayoff,
		EventCount:        len(chapter.Events),
		AdvanceCount:      len(chapter.StorylineAdvances),
	}
	return brief
}
