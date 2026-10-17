// The outline-repair and outline-global-repair context bundles: scoped checks, repair issues, patch
// tasks and the workflow/next-action builders around them.

package toolplane

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/models"
	"novelgen/internal/quality"
	"novelgen/internal/toolcmd"
	"strings"
)

func queryOutlineRepairContext(ctx ToolProjectContext, id, category string) ToolResponse {
	id = strings.TrimSpace(id)
	category = NormalizeKey(category)
	if id == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type outline-repair"}}
	}
	if ToolLooksLikeVolumeID(id) {
		return queryOutlineVolumeRepairContext(ctx, id, category)
	}
	if _, ok := ToolVolumeIDFromChapterID(id); ok {
		return queryOutlineChapterRepairContext(ctx, id, category)
	}
	return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{fmt.Sprintf("cannot infer outline repair scope from id %q", id)}}
}

func QueryOutlineGlobalRepairContext(ctx ToolProjectContext, category string) ToolResponse {
	category = NormalizeKey(category)
	check, err := RunToolOutlineCheckAt(ctx.Root, "all", ctx.Setup, ctx.Outline, "all", "")
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", category, 12)
	mysteryThreads := CollectOutlineMysteryThreads(ctx.Outline)
	issueContext := buildOutlineGlobalRepairIssues(check, category, ctx.Outline)
	patchTask := buildOutlineGlobalPatchTask(issueContext)
	result := OutlineGlobalRepairContext{
		IssueCategory:  category,
		Check:          check,
		IssueContext:   issueContext,
		PatchTask:      patchTask,
		MysteryThreads: filterOutlineMysteryThreadsForCategory(mysteryThreads, category),
		StorySetup:     briefStorySetupResults(ctx.Setup, true),
		Outline:        makeOutlineGlobalRepairBrief(ctx.Outline),
		Navigation: map[string]interface{}{
			"focused_check_query":  toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "all", Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 12}),
			"setup_search_query":   "novelgen tool query story-setup --type search --name <keyword> --view brief",
			"volume_context_query": "novelgen tool query context --type outline-volume --id <volume_id> --view brief",
			"patch_task_rule":      "If patch_task is present, use patch_task.dry_run_command and patch_task.apply_command directly; do not query outline-volume, outline-repair, chapter, tool-results, or source files.",
			"mystery_repair_rule":  "For unresolved mysteries, patch only the suggested_resolution chapter named in mystery_threads or issue_context; preserve existing mystery entries.",
			"repair_rule":          "Use issue_context patch routes when present; do not patch broad global issues without a patch_shape.",
		},
		Workflow: map[string]interface{}{
			"goal":                   "Repair or classify global outline quality/simulation issues without loading the full setup, full outline, or all chapters.",
			"post_patch_check_query": toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "all", Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 12}),
			"stop_rule":              "If no issue_context item has patch_query and patch_shape, return review_result with the smallest recommended next workflow instead of inventing a broad patch.",
		},
		NextActions: buildOutlineGlobalRepairNextActions(check, issueContext),
		Stats:       outlineGlobalRepairStats(ctx.Outline, check),
	}
	if len(result.MysteryThreads) > 0 {
		result.Stats["mystery_thread_count"] = len(result.MysteryThreads)
		result.Stats["unresolved_mystery_count"] = countUnresolvedOutlineMysteryThreads(result.MysteryThreads)
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result}
}

func queryOutlineVolumeRepairContext(ctx ToolProjectContext, volumeID, category string) ToolResponse {
	volumes := flattenVolumes(ctx.Outline)
	targetIndex := -1
	for i, volume := range volumes {
		if idMatches(volume.Volume.ID, volumeID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: false, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline volume not found: %q", volumeID)}}
	}
	target := volumes[targetIndex]
	check, err := RunToolOutlineCheckAt(ctx.Root, "all", ctx.Setup, ctx.Outline, "volume", target.Volume.ID)
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", category, 8)
	events := collectRepairVolumeEvents(target, check)
	returnedEvents := events
	if len(returnedEvents) > 8 {
		returnedEvents = returnedEvents[:8]
	}
	result := OutlineRepairContext{
		Scope:         "volume",
		ID:            target.Volume.ID,
		Path:          target.Path,
		IssueCategory: category,
		Check:         check,
		IssueContext:  buildOutlineRepairIssues(check, "volume", target.Volume.ID, target.Volume.ID, category, nil, target.Volume),
		Current:       makeRepairVolumeBrief(*target.Volume, check),
		Events:        briefOutlineResults(returnedEvents, true),
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: target.Volume.ID, View: "brief"}),
				toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: target.Volume.ID, Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 8}),
			},
			"patch_query":                 toolcmd.PatchOutlineVolume(target.Volume.ID),
			"patch_shape":                 map[string]interface{}{"summary": "<optional new summary>", "changed_chapters": []map[string]string{{"id": "<chapter_id>"}}},
			"chapter_patch_shape":         map[string]interface{}{"changed_chapters": []map[string]string{{"id": "<chapter_id>", "summary": "<new summary>"}}},
			"validated_patch_instruction": "Dry-run the volume patch first; apply only when the workflow explicitly allows it.",
		},
		Workflow: buildOutlineRepairWorkflow("volume", target.Volume.ID, target.Volume.ID, category, map[string]interface{}{"summary": "<optional new summary>", "changed_chapters": []map[string]string{{"id": "<chapter_id>"}}}),
		NextActions: buildOutlineRepairNextActions(
			"volume",
			target.Volume.ID,
			target.Volume.ID,
			category,
		),
		Stats: map[string]int{
			"volume_index":    targetIndex,
			"chapter_count":   len(target.Volume.Chapters),
			"event_count":     len(events),
			"returned_events": len(returnedEvents),
			"returned_issues": len(check.Issues),
		},
	}
	warnings := []string{}
	if len(events) > len(returnedEvents) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", len(events), len(returnedEvents)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func queryOutlineChapterRepairContext(ctx ToolProjectContext, chapterID, category string) ToolResponse {
	chapters := flattenChapters(ctx.Outline)
	targetIndex := -1
	for i, chapter := range chapters {
		if idMatches(chapter.Chapter.ID, chapterID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: false, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}
	target := chapters[targetIndex]
	volumeID := target.Path.VolumeID
	check, err := RunToolOutlineCheckAt(ctx.Root, "all", ctx.Setup, ctx.Outline, "chapter", target.Chapter.ID)
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", category, 8)
	eventResp := QueryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
	events := limitEventHitResults(eventResp.Results, 8)
	result := OutlineRepairContext{
		Scope:         "chapter",
		ID:            target.Chapter.ID,
		Path:          target.Path,
		IssueCategory: category,
		Check:         check,
		IssueContext:  buildOutlineRepairIssues(check, "chapter", target.Chapter.ID, volumeID, category, target.Chapter, nil),
		Current:       makeRepairChapterBrief(*target.Chapter),
		Events:        briefOutlineResults(events, false),
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: target.Chapter.ID, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: target.Chapter.ID, View: "brief"}),
				toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "chapter", ID: target.Chapter.ID, Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 8}),
			},
			"patch_query":                 toolcmd.PatchOutlineVolume(volumeID),
			"patch_shape":                 map[string]interface{}{"changed_chapters": []map[string]string{{"id": target.Chapter.ID, "summary": "<new summary>"}}},
			"validated_patch_instruction": "Patch chapters through the parent volume wrapper; dry-run first and apply only when the workflow explicitly allows it.",
		},
		Workflow: buildOutlineRepairWorkflow("chapter", target.Chapter.ID, volumeID, category, map[string]interface{}{"changed_chapters": []map[string]string{{"id": target.Chapter.ID, "summary": "<new summary>"}}}),
		NextActions: buildOutlineRepairNextActions(
			"chapter",
			target.Chapter.ID,
			volumeID,
			category,
		),
		Stats: map[string]int{
			"chapter_index":   targetIndex,
			"event_count":     eventResp.Count,
			"returned_events": len(events),
			"returned_issues": len(check.Issues),
		},
	}
	if parent := ctx.Outline.GetVolumeByID(volumeID); parent != nil {
		result.ParentVolume = makeContextVolumeBrief(*parent, false)
	}
	if targetIndex > 0 && chapters[targetIndex-1].Path.VolumeID == volumeID {
		result.PreviousChapter = makeContextChapterBrief(*chapters[targetIndex-1].Chapter)
	}
	if targetIndex+1 < len(chapters) && chapters[targetIndex+1].Path.VolumeID == volumeID {
		result.NextChapter = makeContextChapterBrief(*chapters[targetIndex+1].Chapter)
	}
	warnings := []string{}
	if eventResp.Count > len(events) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", eventResp.Count, len(events)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func buildOutlineRepairIssues(check *ToolCheckResult, scope, targetID, volumeID, category string, chapter *models.Chapter, volume *models.Volume) []OutlineRepairIssue {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	issues := make([]OutlineRepairIssue, 0, len(check.Issues))
	for _, issue := range check.Issues {
		issueTargetID := strings.TrimSpace(issue.TargetID)
		if issueTargetID == "" {
			issueTargetID = targetID
		}
		issueCategory := NormalizeKey(issue.Category)
		if issueCategory == "" {
			issueCategory = category
		}
		issueScope := scope
		patchShape := map[string]interface{}{"summary": "<optional new summary>"}
		if _, ok := ToolVolumeIDFromChapterID(issueTargetID); ok {
			issueScope = "chapter"
			patchShape = map[string]interface{}{"changed_chapters": []map[string]string{{"id": issueTargetID}}}
		}
		evidenceChapter := chapter
		if evidenceChapter == nil && volume != nil {
			evidenceChapter = findVolumeChapterByID(volume, issueTargetID)
		}
		item := OutlineRepairIssue{
			Category:            issue.Category,
			TargetID:            issueTargetID,
			TargetName:          issue.TargetName,
			Issue:               issue.Issue,
			Suggestion:          issue.Suggestion,
			Priority:            issue.Priority,
			FocusedCheckQuery:   buildOutlineFocusedCheckQuery(issueScope, issueTargetID, issueCategory, 8),
			RepairRouteQuery:    toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: issueTargetID, Name: categoryOrAll(issueCategory), View: "index"}),
			RepairContextQuery:  toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: issueTargetID, Name: categoryOrAll(issueCategory), View: "brief"}),
			PatchQuery:          toolcmd.PatchOutlineVolume(volumeID),
			PatchShape:          patchShape,
			PostPatchCheckQuery: buildOutlineFocusedCheckQuery("volume", volumeID, "", 12),
			Evidence:            buildOutlineRepairEvidence(evidenceChapter),
		}
		issues = append(issues, item)
	}
	return issues
}

func buildOutlineGlobalRepairIssues(check *ToolCheckResult, category string, outline *models.Outline) []OutlineRepairIssue {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	mysteryThreads := CollectOutlineMysteryThreads(outline)
	issues := make([]OutlineRepairIssue, 0, len(check.Issues))
	for _, issue := range check.Issues {
		issueCategory := NormalizeKey(issue.Category)
		if issueCategory == "" {
			issueCategory = category
		}
		nav := ToolIssueNavigation(check.Kind, check.Target, check.Scope, check.ID, issue, ToolCheckResultTargetWords(*check))
		item := OutlineRepairIssue{
			Category:            issue.Category,
			TargetID:            issue.TargetID,
			TargetName:          issue.TargetName,
			Issue:               issue.Issue,
			Suggestion:          issue.Suggestion,
			Priority:            issue.Priority,
			FocusedCheckQuery:   StringMapValue(nav, "focused_check_query"),
			RepairRouteQuery:    StringMapValue(nav, "repair_route_query"),
			RepairContextQuery:  StringMapValue(nav, "repair_context_query"),
			PatchQuery:          StringMapValue(nav, "patch_query"),
			PostPatchCheckQuery: StringMapValue(nav, "post_patch_check_query"),
		}
		if shape, ok := nav["patch_shape"]; ok {
			item.PatchShape = shape
		}
		if issueCategory == "mysteries" && item.PatchShape == nil {
			if thread := FirstPatchableOutlineMysteryThread(mysteryThreads); thread != nil {
				item.TargetID = thread.ID
				item.TargetName = thread.ID
				item.RepairContextQuery = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-global-repair", Name: categoryOrAll(issueCategory), View: "brief"})
				item.PatchQuery = thread.PatchQuery
				item.PatchShape = thread.PatchShape
				item.PostPatchCheckQuery = thread.PostPatchCheckQuery
				item.Evidence = map[string]interface{}{
					"mystery_id":                     thread.ID,
					"planted_chapter_id":             thread.PlantedChapterID,
					"planted_volume_id":              thread.PlantedVolumeID,
					"suggested_resolution_chapter":   thread.SuggestedResolutionChapterID,
					"suggested_resolution_volume_id": thread.SuggestedResolutionVolumeID,
					"repair_strategy":                thread.RepairStrategy,
				}
			}
		}
		issues = append(issues, item)
	}
	return issues
}

func buildOutlineGlobalPatchTask(issues []OutlineRepairIssue) *outlinePatchTask {
	issue := firstPatchableOutlineRepairIssue(issues)
	if issue == nil {
		return nil
	}
	patchQuery := strings.TrimSpace(issue.PatchQuery)
	postCheck := strings.TrimSpace(issue.PostPatchCheckQuery)
	if patchQuery == "" || issue.PatchShape == nil || postCheck == "" {
		return nil
	}
	taskID := outlineGlobalPatchTaskID(issue.Category)
	stdinPatchJSON, dryRunCommand, applyCommand := outlinePatchTaskCommands(patchQuery, issue.PatchShape, taskID)
	return &outlinePatchTask{
		Category:            issue.Category,
		TargetID:            issue.TargetID,
		TargetName:          issue.TargetName,
		Priority:            issue.Priority,
		Issue:               issue.Issue,
		TaskID:              taskID,
		PatchQuery:          patchQuery,
		PatchShape:          issue.PatchShape,
		StdinPatchJSON:      stdinPatchJSON,
		DryRunCommand:       dryRunCommand,
		ApplyCommand:        applyCommand,
		PostPatchCheckQuery: postCheck,
		RepairContextQuery:  issue.RepairContextQuery,
		StdinRequired:       taskID == "",
		MaxPatchAttempts:    1,
		MaxApplyAttempts:    1,
		DryRunInstruction:   "Run dry_run_command exactly once. Do not use --patch-json, placeholder text, temp files, or encoding helper commands.",
		ApplyInstruction:    "Only if dry-run succeeds, run apply_command exactly once, then run post_patch_check_query exactly.",
		StopAfterCheck:      true,
		ForbiddenQueries: []string{
			"novelgen tool query context --type outline-volume",
			"novelgen tool query context --type outline-repair",
			"novelgen tool query context --type chapter-repair",
			"novelgen tool query outline",
			"novelgen tool query chapter",
		},
		ForbiddenCommands: []string{
			"type <claude tool-results>",
			"Get-Content <claude tool-results>",
			"findstr",
			"echo test",
			"python/node/powershell encoding helpers",
			"--patch-json",
			"<json>",
			"<compact-json>",
		},
		Evidence: issue.Evidence,
	}
}

func outlineGlobalPatchTaskID(category string) string {
	category = NormalizeKey(category)
	if category == "" {
		return ""
	}
	return "outline-global-repair:" + category
}

func outlinePatchTaskCommands(patchQuery string, patchShape interface{}, taskID string) (string, string, string) {
	if strings.TrimSpace(taskID) != "" {
		taskCommand := patchQuery + " --task " + shellSingleQuote(taskID)
		return "", taskCommand, taskCommand + " --apply"
	}
	payload, err := json.Marshal(patchShape)
	if err != nil || len(payload) == 0 {
		return "", patchQuery, patchQuery + " --apply"
	}
	stdinPatchJSON := string(payload)
	dryRunCommand := fmt.Sprintf("printf '%%s' %s | %s", shellSingleQuote(stdinPatchJSON), patchQuery)
	applyCommand := dryRunCommand + " --apply"
	return stdinPatchJSON, dryRunCommand, applyCommand
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func buildOutlineGlobalRepairNextActions(check *ToolCheckResult, issues []OutlineRepairIssue) []ToolNextAction {
	if patchable := firstPatchableOutlineRepairIssue(issues); patchable != nil {
		_, dryRunCommand, applyCommand := outlinePatchTaskCommands(patchable.PatchQuery, patchable.PatchShape, outlineGlobalPatchTaskID(patchable.Category))
		return []ToolNextAction{{
			Step:    1,
			Action:  "use_patch_task",
			Purpose: "Use patch_task.dry_run_command and patch_task.apply_command from this response directly; do not query outline-volume, outline-repair, chapters, tool-results, full setup, full outline, or all chapters.",
		}, {
			Step:    2,
			Action:  "patch_dry_run",
			Command: dryRunCommand,
			Purpose: "Validate patch_task once without writing, using the exact dry_run_command.",
		}, {
			Step:    3,
			Action:  "patch_apply",
			Command: applyCommand,
			Purpose: "Apply only after the same patch task succeeds in dry-run.",
		}, {
			Step:    4,
			Action:  "post_patch_check",
			Command: patchable.PostPatchCheckQuery,
			Purpose: "Confirm the global issue improved and no blocking outline regression was introduced.",
		}}
	}
	if check == nil || len(check.Issues) == 0 {
		return []ToolNextAction{{
			Step:    1,
			Action:  "return_final_json",
			Purpose: "No global outline issues match the current filters.",
		}}
	}
	actions := toolCheckNextActions(*check)
	if len(actions) == 0 {
		return actions
	}
	for i := range actions {
		actions[i].Step = i + 1
	}
	if actions[0].Action == "repair_first_returned_issue" || actions[0].Action == "inspect_first_returned_issue_route" {
		actions[0].Action = "inspect_first_global_issue"
		actions[0].Purpose = "Use issue_context for the first global issue; patch only when it has a concrete patch_query and patch_shape."
	}
	return actions
}

func firstPatchableOutlineRepairIssue(issues []OutlineRepairIssue) *OutlineRepairIssue {
	for i := range issues {
		if strings.TrimSpace(issues[i].PatchQuery) == "" || issues[i].PatchShape == nil {
			continue
		}
		return &issues[i]
	}
	return nil
}

func makeOutlineGlobalRepairBrief(outline *models.Outline) interface{} {
	if outline == nil {
		return nil
	}
	brief := OutlineBrief{Navigation: map[string]interface{}{
		"volume_context_query":  "novelgen tool query context --type outline-volume --id <volume_id> --view brief",
		"chapter_context_query": "novelgen tool query context --type outline-repair --id <chapter_id> --name <category> --view brief",
	}}
	for _, part := range outline.Parts {
		partItem := PartBrief{
			ID:      part.ID,
			Title:   part.Title,
			Summary: clipToolString(part.Summary, 360),
		}
		for _, volume := range part.Volumes {
			partItem.Volumes = append(partItem.Volumes, VolumeBrief{
				ID:      volume.ID,
				Title:   volume.Title,
				Summary: clipToolString(volume.Summary, 260),
			})
		}
		brief.Parts = append(brief.Parts, partItem)
	}
	return brief
}

func outlineGlobalRepairStats(outline *models.Outline, check *ToolCheckResult) map[string]int {
	stats := map[string]int{}
	if outline != nil {
		stats["part_count"] = len(outline.Parts)
		stats["volume_count"] = quality.CountOutlineVolumes(outline)
		stats["chapter_count"] = quality.CountOutlineChapters(outline)
	}
	if check != nil {
		stats["returned_issues"] = len(check.Issues)
		stats["summary_total"] = check.Summary.Total
		stats["summary_medium"] = check.Summary.Medium
		stats["summary_low"] = check.Summary.Low
	}
	return stats
}

func collectRepairVolumeEvents(target flatVolume, check *ToolCheckResult) []EventHit {
	if check == nil || len(check.Issues) == 0 {
		return collectVolumeEvents(target)
	}
	chapterIDs := map[string]bool{}
	for _, issue := range check.Issues {
		if _, ok := ToolVolumeIDFromChapterID(issue.TargetID); ok {
			chapterIDs[issue.TargetID] = true
		}
	}
	if len(chapterIDs) == 0 {
		return collectVolumeEvents(target)
	}
	events := []EventHit{}
	for _, chapter := range target.Volume.Chapters {
		if !chapterIDs[chapter.ID] {
			continue
		}
		for _, event := range chapter.Events {
			events = append(events, EventHit{
				ChapterID:    chapter.ID,
				ChapterTitle: chapter.Title,
				Path: OutlinePath{
					PartID:       target.Path.PartID,
					PartTitle:    target.Path.PartTitle,
					VolumeID:     target.Path.VolumeID,
					VolumeTitle:  target.Path.VolumeTitle,
					ChapterID:    chapter.ID,
					ChapterTitle: chapter.Title,
				},
				Event: event,
			})
		}
	}
	return events
}

func findVolumeChapterByID(volume *models.Volume, chapterID string) *models.Chapter {
	if volume == nil {
		return nil
	}
	for i := range volume.Chapters {
		if idMatches(volume.Chapters[i].ID, chapterID) {
			return &volume.Chapters[i]
		}
	}
	return nil
}

func buildOutlineRepairEvidence(chapter *models.Chapter) map[string]interface{} {
	if chapter == nil {
		return nil
	}
	evidence := map[string]interface{}{}
	if strings.TrimSpace(chapter.StateChange) != "" {
		evidence["state_change"] = clipToolString(chapter.StateChange, 180)
	}
	if strings.TrimSpace(chapter.Conflict) != "" {
		evidence["conflict"] = clipToolString(chapter.Conflict, 180)
	}
	if chapter.ChapterPayoff != nil && !chapter.ChapterPayoff.IsZero() {
		evidence["chapter_payoff"] = chapter.ChapterPayoff
	}
	if advances := compactStorylineAdvances(chapter.StorylineAdvances, 4); len(advances) > 0 {
		evidence["storyline_advances"] = advances
	}
	if len(evidence) == 0 {
		return nil
	}
	return evidence
}

func buildOutlineRepairWorkflow(scope, targetID, volumeID, category string, patchShape interface{}) map[string]interface{} {
	workflow := map[string]interface{}{
		"scope":                  scope,
		"target_id":              targetID,
		"focused_check_query":    buildOutlineFocusedCheckQuery(scope, targetID, category, 8),
		"patch_query":            toolcmd.PatchOutlineVolume(volumeID),
		"patch_shape":            patchShape,
		"post_patch_check_query": buildOutlineFocusedCheckQuery("volume", volumeID, "", 12),
		"stop_rules": []string{
			"Patch only the returned target volume and only the listed changed chapters.",
			"If issue_context is empty after filtering, treat the focused issue as not currently reproduced and return an empty patch.",
			"After a non-empty patch, run dry-run first; apply only when the agent workflow input allows apply_patches=true.",
			"After apply, run post_patch_check_query and repair any directly caused medium+ issue before returning.",
		},
	}
	if scope == "chapter" {
		workflow["parent_volume_id"] = volumeID
	}
	return workflow
}

func buildOutlineRepairNextActions(scope, targetID, volumeID, category string) []ToolNextAction {
	scope = normalizeToolScope(scope)
	if scope == "" {
		scope = "volume"
	}
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_repair_bundle",
			Purpose: "Read check.issues, issue_context, current target, events, and adjacent context from this response.",
		},
		{
			Step:    2,
			Action:  "focused_check",
			Command: buildOutlineFocusedCheckQuery(scope, targetID, category, 8),
			Purpose: "Reproduce the target issue after narrowing category or priority.",
			When:    "The current check output is insufficient or was filtered.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: toolcmd.PatchOutlineVolume(volumeID),
			Purpose: "Validate a minimal volume patch; chapter repairs must use changed_chapters under the parent volume.",
		},
		{
			Step:    4,
			Action:  "post_patch_check",
			Command: buildOutlineFocusedCheckQuery("volume", volumeID, "", 12),
			Purpose: "Run after apply to catch medium+ regressions in the whole patched volume.",
		},
	}
}

func buildOutlineFocusedCheckQuery(scope, id, category string, maxIssues int) string {
	scope = normalizeToolScope(scope)
	if scope == "" {
		scope = "volume"
	}
	return toolcmd.Check(toolcmd.CheckOptions{
		Target:      "outline",
		Scope:       scope,
		ID:          id,
		Category:    category,
		MinPriority: "low",
		MaxIssues:   maxIssues,
	})
}
