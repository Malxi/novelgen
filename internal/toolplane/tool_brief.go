package toolplane

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/store"
	"novelgen/internal/toolcmd"
	"os"
	"sort"
	"strings"
)

func briefToolResults(section string, results interface{}) interface{} {
	switch NormalizeKey(section) {
	case "story-setup", "setup":
		return briefStorySetupResults(results, false)
	case "outline":
		return briefOutlineResults(results, false)
	case "chapter", "chapters":
		return briefChapterResults(results, false)
	case "craft":
		return briefCraftResults(results, false)
	case "context":
		return briefContextResults(results, false)
	case "logs", "log":
		return briefLogResults(results, false)
	default:
		return results
	}
}

func indexToolResults(section string, results interface{}) interface{} {
	switch NormalizeKey(section) {
	case "story-setup", "setup":
		return briefStorySetupResults(results, true)
	case "outline":
		return briefOutlineResults(results, true)
	case "chapter", "chapters":
		return briefChapterResults(results, true)
	case "craft":
		return briefCraftResults(results, true)
	case "context":
		return briefContextResults(results, true)
	case "logs", "log":
		return briefLogResults(results, true)
	default:
		return results
	}
}

func briefCraftResults(results interface{}, indexOnly bool) interface{} {
	switch typed := results.(type) {
	case []map[string]interface{}:
		out := make([]map[string]interface{}, 0, len(typed))
		for _, hit := range typed {
			if indexOnly {
				out = append(out, indexCraftHit(hit))
			} else {
				out = append(out, briefCraftHit(hit))
			}
		}
		return out
	case map[string]interface{}:
		if indexOnly {
			out := map[string]interface{}{}
			for section, value := range typed {
				out[section] = craftCollectionIndex(section, value)
			}
			return out
		}
		return results
	default:
		return results
	}
}

func briefCraftHit(hit map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, key := range []string{"type", "key"} {
		if value, ok := hit[key]; ok {
			out[key] = value
		}
	}
	if raw, ok := hit["object"].(map[string]interface{}); ok {
		out["object"] = compactCraftObject(raw)
	} else if object, ok := hit["object"]; ok {
		out["object"] = object
	}
	return out
}

func indexCraftHit(hit map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	key := fmt.Sprint(hit["key"])
	kind := normalizeCraftType(fmt.Sprint(hit["type"]))
	if key != "" {
		out["key"] = key
	}
	if kind != "" {
		out["type"] = kind
	}
	name := key
	if raw, ok := hit["object"].(map[string]interface{}); ok {
		if objectName := StringMapValue(raw, "name"); objectName != "" {
			name = objectName
			out["name"] = objectName
		}
		for _, field := range []string{"role_in_story", "rpg_role", "combat_role", "type", "category"} {
			if value, ok := raw[field]; ok {
				if text, ok := value.(string); ok {
					out[field] = clipToolString(text, 120)
				} else {
					out[field] = value
				}
			}
		}
	}
	if kind != "" && name != "" {
		out["detail_query"] = fmt.Sprintf("novelgen tool query craft --type %s --name %q --view brief", kind, name)
	}
	return out
}

func craftCollectionIndex(section string, value interface{}) interface{} {
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]interface{}{"count": 0}
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]interface{}{"count": 0}
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return map[string]interface{}{
		"count":        len(keys),
		"keys":         keys,
		"detail_query": fmt.Sprintf("novelgen tool query craft --type %s --name <name> --view brief", craftCollectionType(section)),
	}
}

func craftCollectionType(section string) string {
	switch NormalizeKey(section) {
	case "characters":
		return "character"
	case "items":
		return "item"
	case "locations":
		return "location"
	case "organizations", "orgs":
		return "organization"
	default:
		return NormalizeKey(section)
	}
}

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
		Command: fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief", ctx.ChapterID),
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
		Command: fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view brief", ctx.ChapterID, categoryOrAll(ctx.IssueCategory)),
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
			fmt.Sprintf("novelgen tool check all --target outline --scope %s --id %q --category %s --min-priority low --max-issues 8", ctx.Scope, ctx.ID, categoryOrAll(ctx.IssueCategory)),
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
		Command: fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", ctx.ID, categoryOrAll(ctx.IssueCategory)),
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
				Command: fmt.Sprintf("novelgen tool check all --target outline --scope all --category %s --min-priority low --max-issues 12", categoryOrAll(category)),
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
			Command: fmt.Sprintf("novelgen tool query context --type outline-global-repair --name %q --view brief", categoryOrAll(ctx.IssueCategory)),
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
			fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", ctx.ChapterID),
			"The scoped recap check returned zero issues; do not fetch brief repair context or patch recap.",
		)
	}
	out := []ToolNextAction{{
		Step:    1,
		Action:  "query_brief_repair_context",
		Purpose: "Fetch the focused recap repair bundle before patching; index view is only the route map and check summary.",
		Command: fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", ctx.ChapterID),
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
		Command: fmt.Sprintf("novelgen tool query context --type craft-%s --name %q --view brief", ctx.Target, ctx.Name),
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

func briefStorySetupResults(results interface{}, indexOnly bool) interface{} {
	setup, ok := results.(*models.StorySetup)
	if !ok || setup == nil {
		return results
	}
	brief := StorySetupBrief{
		ProjectName:    setup.ProjectName,
		Genres:         setup.Genres,
		TargetAudience: clipToolString(setup.TargetAudience, 160),
		Tone:           clipToolString(setup.Tone, 160),
		POVStyle:       setup.POVStyle,
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				"novelgen tool query story-setup --type search --name <keyword> --view brief",
				"novelgen tool query story-setup --type storyline --name <name>",
				"novelgen tool query story-setup --type core-cast --name <name>",
				"novelgen tool query story-setup --type premise --name <name>",
				"novelgen tool query story-setup --type resource --name <name>",
			},
		},
	}
	if !indexOnly {
		brief.Premise = clipToolString(setup.Premise, 900)
		brief.Theme = clipToolString(setup.Theme, 500)
		brief.Rules = clipToolStrings(setup.Rules, 8, 220)
		brief.LongFormPlan = makeLongFormPlanBrief(setup.LongFormPlan, false)
	} else {
		brief.LongFormPlan = makeLongFormPlanBrief(setup.LongFormPlan, true)
	}
	for _, cast := range setup.CoreCast {
		item := coreCastBrief{ID: cast.ID, Name: cast.Name, Role: cast.Role, Importance: cast.Importance, EntryPhase: cast.EntryPhase, StorylineRefs: cast.StorylineRefs}
		if !indexOnly {
			item.StoryFunction = clipToolString(cast.StoryFunction, 220)
			item.RelationshipToLead = clipToolString(cast.RelationshipToLead, 160)
			item.RelationshipArc = clipToolString(cast.RelationshipArc, 180)
			item.Payoff = clipToolString(cast.Payoff, 180)
		}
		brief.CoreCast = append(brief.CoreCast, item)
	}
	for _, storyline := range setup.Storylines {
		item := storylineBrief{Name: storyline.Name, Type: storyline.Type, Importance: storyline.Importance, Scope: storyline.Scope}
		if !indexOnly {
			item.Description = clipToolString(storyline.Description, 260)
			item.SetupRole = clipToolString(storyline.SetupRole, 180)
			item.RepeatablePressure = clipToolString(storyline.RepeatablePressure, 160)
			item.PayoffCadence = clipToolString(storyline.PayoffCadence, 140)
			item.FailureMode = clipToolString(storyline.FailureMode, 160)
			item.OpenQuestion = clipToolString(storyline.OpenQuestion, 160)
			item.AppealEngine = storyline.AppealEngine
		}
		brief.Storylines = append(brief.Storylines, item)
	}
	for _, premise := range setup.Premises {
		item := premiseBrief{Name: premise.Name, Category: premise.Category}
		if !indexOnly {
			item.Description = clipToolString(premise.Description, 260)
			item.Progression = makeProgressionBrief(premise.Progression)
		}
		brief.Premises = append(brief.Premises, item)
	}
	for _, entry := range setup.WorldTimeline {
		item := worldTimelineBrief{Year: entry.Year, Event: clipToolString(entry.Event, 180)}
		if !indexOnly {
			item.Impact = clipToolString(entry.Impact, 180)
			item.RelatedMystery = entry.RelatedMystery
		}
		brief.WorldTimeline = append(brief.WorldTimeline, item)
	}
	for _, resource := range setup.WorldResources {
		item := worldResourceBrief{Name: resource.Name, Category: resource.Category, Scarcity: resource.Scarcity}
		if !indexOnly {
			item.Description = clipToolString(resource.Description, 180)
		}
		brief.WorldResources = append(brief.WorldResources, item)
	}
	return brief
}

func makeLongFormPlanBrief(plan *models.LongFormPlan, indexOnly bool) interface{} {
	if plan == nil || plan.IsZero() {
		return nil
	}
	brief := longFormPlanBrief{
		TargetChapters: plan.TargetChapters,
		TargetVolumes:  plan.TargetVolumes,
		MainLoop:       clipToolString(plan.MainLoop, 180),
	}
	if indexOnly {
		return brief
	}
	brief.EscalationLadder = clipToolStrings(plan.EscalationLadder, 8, 120)
	brief.ReaderPromises = clipToolStrings(plan.ReaderPromises, 8, 120)
	brief.PayoffCadence = clipToolString(plan.PayoffCadence, 180)
	brief.VolumePattern = clipToolStrings(plan.VolumePattern, 10, 100)
	brief.MidpointMutation = clipToolString(plan.MidpointMutation, 180)
	brief.EndgamePromise = clipToolString(plan.EndgamePromise, 180)
	return brief
}

func makeProgressionBrief(stages []models.ProgressionStage) []progressionStageBrief {
	if len(stages) == 0 {
		return nil
	}
	out := make([]progressionStageBrief, 0, len(stages))
	for _, stage := range stages {
		out = append(out, progressionStageBrief{
			Level: stage.Level,
			Name:  clipToolString(stage.Name, 80),
		})
	}
	return out
}

func briefOutlineResults(results interface{}, indexOnly bool) interface{} {
	switch values := results.(type) {
	case *models.Outline:
		if values == nil {
			return results
		}
		out := OutlineBrief{Navigation: map[string]interface{}{
			"detail_queries": []string{
				"novelgen tool query outline --type volume --id <volume_id>",
				"novelgen tool query outline --type chapter --id <chapter_id>",
				"novelgen tool query outline --type events --chapter-id <chapter_id>",
			},
		}}
		for _, part := range values.Parts {
			out.Parts = append(out.Parts, makePartBrief(part, indexOnly))
		}
		return out
	case []OutlineHit:
		out := make([]OutlineHit, 0, len(values))
		for _, hit := range values {
			hit.Object = briefOutlineObject(hit.Object, indexOnly)
			out = append(out, hit)
		}
		return out
	case []EventHit:
		out := make([]EventHitBrief, 0, len(values))
		for _, hit := range values {
			out = append(out, makeEventHitBrief(hit, indexOnly))
		}
		return out
	default:
		return results
	}
}

func briefOutlineObject(value interface{}, indexOnly bool) interface{} {
	switch typed := value.(type) {
	case models.Part:
		return makePartBrief(typed, indexOnly)
	case models.Volume:
		return makeVolumeBrief(typed, indexOnly)
	case models.Chapter:
		return makeChapterBrief(typed, indexOnly)
	case *models.Part:
		if typed != nil {
			return makePartBrief(*typed, indexOnly)
		}
	case *models.Volume:
		if typed != nil {
			return makeVolumeBrief(*typed, indexOnly)
		}
	case *models.Chapter:
		if typed != nil {
			return makeChapterBrief(*typed, indexOnly)
		}
	}
	return value
}

func makePartBrief(part models.Part, indexOnly bool) PartBrief {
	brief := PartBrief{ID: part.ID, Title: part.Title, Summary: clipToolString(part.Summary, 500)}
	for _, volume := range part.Volumes {
		brief.Volumes = append(brief.Volumes, makeVolumeBrief(volume, indexOnly))
	}
	return brief
}

func makeVolumeBrief(volume models.Volume, indexOnly bool) VolumeBrief {
	brief := VolumeBrief{
		ID:      volume.ID,
		Title:   volume.Title,
		Summary: clipToolString(volume.Summary, 700),
		Navigation: map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id>",
				"novelgen tool query outline --type events --chapter-id <chapter_id>",
			},
		},
	}
	if !indexOnly && !volume.PayoffContract.IsZero() {
		brief.PayoffContract = volume.PayoffContract
	}
	for _, chapter := range volume.Chapters {
		brief.Chapters = append(brief.Chapters, makeChapterBrief(chapter, true))
	}
	return brief
}

func makeContextVolumeBrief(volume models.Volume, includeChapters bool) VolumeBrief {
	brief := VolumeBrief{
		ID:      volume.ID,
		Title:   volume.Title,
		Summary: clipToolString(volume.Summary, 360),
		Navigation: map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id> --view brief",
				"novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			},
		},
	}
	if !volume.PayoffContract.IsZero() {
		brief.PayoffContract = volume.PayoffContract
	}
	if !includeChapters {
		return brief
	}
	for _, chapter := range volume.Chapters {
		brief.Chapters = append(brief.Chapters, makeContextChapterBrief(chapter))
	}
	return brief
}

func makeContextVolumeRouteMap(volume models.Volume, includeChapters bool) map[string]interface{} {
	brief := map[string]interface{}{
		"id":      volume.ID,
		"title":   volume.Title,
		"summary": clipToolString(volume.Summary, 180),
		"navigation": map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id> --view brief",
				"novelgen tool query outline --type chapter --id <chapter_id> --fields location,timeline,state_anchor --view brief",
				"novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			},
		},
	}
	if !includeChapters {
		return brief
	}
	chapters := make([]map[string]interface{}, 0, len(volume.Chapters))
	for _, chapter := range volume.Chapters {
		chapters = append(chapters, map[string]interface{}{
			"id":            chapter.ID,
			"title":         chapter.Title,
			"characters":    clipToolStrings(chapter.Characters, 6, 40),
			"location":      clipToolString(chapter.Location, 80),
			"event_count":   len(chapter.Events),
			"advance_count": len(chapter.StorylineAdvances),
		})
	}
	brief["chapters"] = chapters
	return brief
}

func makeRepairVolumeBrief(volume models.Volume, check *ToolCheckResult) VolumeBrief {
	brief := makeContextVolumeBrief(volume, false)
	chapterIDs := map[string]bool{}
	if check != nil {
		for _, issue := range check.Issues {
			if _, ok := ToolVolumeIDFromChapterID(issue.TargetID); ok {
				chapterIDs[issue.TargetID] = true
			}
		}
	}
	for _, chapter := range volume.Chapters {
		if len(chapterIDs) == 0 || chapterIDs[chapter.ID] {
			brief.Chapters = append(brief.Chapters, makeContextChapterBrief(chapter))
		}
		if len(chapterIDs) == 0 && len(brief.Chapters) >= 8 {
			break
		}
	}
	return brief
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

func compactStorylineAdvances(advances []models.StorylineAdvance, maxItems int) []models.StorylineAdvance {
	if maxItems <= 0 || len(advances) == 0 {
		return nil
	}
	if len(advances) > maxItems {
		advances = advances[:maxItems]
	}
	result := make([]models.StorylineAdvance, 0, len(advances))
	for _, advance := range advances {
		result = append(result, models.StorylineAdvance{
			StorylineName: clipToolString(advance.StorylineName, 80),
			Stage:         clipToolString(advance.Stage, 50),
			Change:        clipToolString(advance.Change, 180),
			Consequence:   clipToolString(advance.Consequence, 180),
			Pressure:      clipToolString(advance.Pressure, 180),
		})
	}
	return result
}

func makeRecapBrief(value *models.ChapterRecap) interface{} {
	if value == nil {
		return nil
	}
	return models.ChapterRecap{
		ChapterID:       value.ChapterID,
		Title:           clipToolString(value.Title, 80),
		Location:        clipToolString(value.Location, 120),
		Time:            clipToolString(value.Time, 120),
		Present:         clipToolStrings(value.Present, 12, 50),
		PlotBeats:       clipToolStrings(value.PlotBeats, 5, 160),
		Decisions:       clipToolStrings(value.Decisions, 4, 140),
		Reveals:         clipToolStrings(value.Reveals, 4, 140),
		Unresolved:      clipToolStrings(value.Unresolved, 4, 140),
		Promises:        clipToolStrings(value.Promises, 4, 140),
		Items:           clipToolStrings(value.Items, 5, 140),
		Status:          clipToolStrings(value.Status, 5, 140),
		LastLine:        clipToolString(value.LastLine, 220),
		Cliffhanger:     clipToolString(value.Cliffhanger, 180),
		NextOpeningHint: clipToolString(value.NextOpeningHint, 220),
	}
}

func recapUsableForChapter(value *models.ChapterRecap, chapter *models.Chapter) (bool, string) {
	if value == nil || chapter == nil {
		return false, ""
	}
	recapID := strings.TrimSpace(value.ChapterID)
	chapterID := strings.TrimSpace(chapter.ID)
	if recapID != "" && chapterID != "" && !idMatches(recapID, chapterID) {
		return false, fmt.Sprintf("recap chapter_id %q does not match outline chapter %q", recapID, chapterID)
	}
	recapTitle := strings.TrimSpace(value.Title)
	chapterTitle := strings.TrimSpace(chapter.Title)
	if recapTitle != "" && chapterTitle != "" && !strings.EqualFold(recapTitle, chapterTitle) {
		return false, fmt.Sprintf("recap title %q does not match outline title %q for %s", recapTitle, chapterTitle, chapterID)
	}
	return true, ""
}

func loadUsableToolContextRecap(root *store.Root, store *recap.Store, chapter *models.Chapter) (*models.ChapterRecap, string) {
	if store == nil || chapter == nil {
		return nil, ""
	}
	loaded, err := store.Load(chapter.ID)
	if err != nil {
		return nil, ""
	}
	if ok, warning := recapUsableForChapter(loaded, chapter); !ok {
		return nil, warning
	}
	if ok, warning := recapFreshForChapterMarkdown(root, chapter.ID); !ok {
		return nil, warning
	}
	return loaded, ""
}

func recapFreshForChapterMarkdown(root *store.Root, chapterID string) (bool, string) {
	chapterID = strings.TrimSpace(chapterID)
	if root.Dir() == "" || chapterID == "" {
		return true, ""
	}
	recapPath := root.RecapFile(chapterID)
	chapterPath := firstExistingPath(root.ChapterCandidates(chapterID))
	if chapterPath == "" {
		return true, ""
	}
	recapInfo, recapErr := os.Stat(recapPath)
	chapterInfo, chapterErr := os.Stat(chapterPath)
	if recapErr != nil || chapterErr != nil {
		return true, ""
	}
	if recapInfo.ModTime().Before(chapterInfo.ModTime()) {
		return false, fmt.Sprintf("recap %s is older than chapter markdown %s", recapPath, chapterPath)
	}
	return true, ""
}

func makeChapterBrief(chapter models.Chapter, indexOnly bool) ChapterBrief {
	brief := ChapterBrief{
		ID:           chapter.ID,
		Title:        chapter.Title,
		Summary:      clipToolString(chapter.Summary, 500),
		Characters:   chapter.Characters,
		Location:     chapter.Location,
		StateChange:  clipToolString(chapter.StateChange, 180),
		EventCount:   len(chapter.Events),
		AdvanceCount: len(chapter.StorylineAdvances),
	}
	if indexOnly {
		return brief
	}
	brief.OpeningBeat = clipToolString(chapter.OpeningBeat, 180)
	brief.ClosingBeat = clipToolString(chapter.ClosingBeat, 180)
	brief.Conflict = clipToolString(chapter.Conflict, 220)
	brief.Pacing = clipToolString(chapter.Pacing, 180)
	brief.Timeline = chapter.Timeline
	brief.StateAnchor = chapter.StateAnchor
	brief.StorylineAdvances = chapter.StorylineAdvances
	brief.ChapterPayoff = chapter.ChapterPayoff
	for _, event := range chapter.Events {
		brief.Events = append(brief.Events, makeEventBrief(event))
	}
	return brief
}

func makeEventBrief(event models.Event) EventBrief {
	return EventBrief{
		Type:       event.Type,
		Characters: event.Characters,
		Subject:    clipToolString(event.Subject, 120),
		Change:     clipToolString(event.Change, 140),
		Details:    clipToolString(event.Details, 180),
		Actor:      event.GetActor(),
		Action:     event.GetAction(),
		Target:     event.GetTarget(),
		TargetType: event.GetTargetType(),
		Context:    clipToolString(event.Context, 140),
		Result:     clipToolString(event.Result, 160),
	}
}

func makeEventIndexBrief(event models.Event) EventBrief {
	return EventBrief{
		Type:       event.Type,
		Subject:    clipToolString(event.Subject, 80),
		Change:     clipToolString(event.Change, 80),
		Actor:      event.GetActor(),
		Action:     event.GetAction(),
		Target:     event.GetTarget(),
		TargetType: event.GetTargetType(),
	}
}

func makeEventHitBrief(hit EventHit, indexOnly bool) EventHitBrief {
	brief := EventHitBrief{
		ChapterID:    hit.ChapterID,
		ChapterTitle: hit.ChapterTitle,
		EventIndex:   hit.EventIndex,
		Reasons:      hit.Reasons,
	}
	if indexOnly {
		brief.Event = makeEventIndexBrief(hit.Event)
		return brief
	}
	brief.Path = &hit.Path
	brief.Event = makeEventBrief(hit.Event)
	return brief
}

func briefChapterResults(results interface{}, indexOnly bool) interface{} {
	values, ok := results.([]chapterHit)
	if !ok {
		return results
	}
	out := make([]map[string]interface{}, 0, len(values))
	for _, hit := range values {
		item := map[string]interface{}{
			"id":      hit.ID,
			"title":   hit.Title,
			"summary": hit.Summary,
			"path":    hit.Path,
		}
		if len(hit.Reasons) > 0 {
			item["reasons"] = hit.Reasons
		}
		if hit.Outline != nil {
			brief := makeChapterBrief(*hit.Outline, indexOnly)
			item["outline"] = brief
		}
		if hit.ContentPath != "" && !indexOnly {
			item["content_path"] = hit.ContentPath
		}
		out = append(out, item)
	}
	return out
}

func clipToolStrings(values []string, maxItems, maxRunes int) []string {
	if len(values) == 0 {
		return nil
	}
	if maxItems > 0 && len(values) > maxItems {
		values = values[:maxItems]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if clipped := clipToolString(value, maxRunes); clipped != "" {
			out = append(out, clipped)
		}
	}
	return out
}

func firstNStrings(values []string, maxItems int) []string {
	if maxItems > 0 && len(values) > maxItems {
		return values[:maxItems]
	}
	return values
}

func loadChapterExcerptForTool(root *store.Root, chapterID string) (map[string]string, []string) {
	chapterID = strings.TrimSpace(chapterID)
	if root.Dir() == "" || chapterID == "" {
		return nil, []string{"project root or chapter id missing; chapter excerpt unavailable"}
	}
	for _, path := range root.ChapterCandidates(chapterID) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		return map[string]string{
			"content_path": path,
			"opening":      clipToolString(content, 320),
			"closing":      clipToolTailString(content, 420),
			"last_line":    clipToolString(lastNonEmptyLine(content), 220),
		}, nil
	}
	return nil, []string{fmt.Sprintf("chapter manuscript not found for %s", chapterID)}
}

func clipToolString(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func clipToolTailString(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return "..." + string(runes[len(runes)-maxRunes:])
}

func lastNonEmptyLine(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func applyToolLimit(resp *ToolResponse, limit int) {
	if resp == nil || limit <= 0 || resp.Results == nil {
		return
	}
	data, err := json.Marshal(resp.Results)
	if err != nil {
		return
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil || len(values) <= limit {
		return
	}
	resp.Results = values[:limit]
	resp.Warnings = append(resp.Warnings, fmt.Sprintf("results truncated to --limit=%d", limit))
}
