// Routing an issue back to an actionable command: the navigation block attached to each check issue,
// including the patch shape and the focused post-patch check it advertises.

package toolplane

import (
	"fmt"
	"strconv"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/toolcmd"
)

func ToolIssueNavigation(kind, target, scope, checkID string, issue models.ReviewSuggestion, targetWords int) map[string]interface{} {
	kind = NormalizeKey(kind)
	target = NormalizeKey(target)
	scope = normalizeToolScope(scope)
	checkID = strings.TrimSpace(checkID)
	targetID := strings.TrimSpace(issue.TargetID)
	targetName := strings.TrimSpace(issue.TargetName)
	category := NormalizeKey(issue.Category)
	navigation := map[string]interface{}{}
	if targetID == "" || strings.EqualFold(targetID, "global") {
		return toolGlobalIssueNavigation(kind, target, scope, checkID, category)
	}

	switch target {
	case "outline":
		if volumeID, ok := ToolVolumeIDFromChapterID(targetID); ok {
			navigation["target_kind"] = "chapter"
			navigation["volume_id"] = volumeID
			navigation["detail_queries"] = []string{
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: targetID, View: "brief"}),
				toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", ChapterID: targetID, Fields: "result,details,target,target_type,actor,action", View: "brief"}),
			}
			navigation["focused_check_query"] = toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "chapter", ID: targetID, Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 8})
			navigation["repair_route_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: targetID, Name: categoryOrAll(category), View: "index"})
			navigation["repair_context_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: targetID, Name: categoryOrAll(category), View: "brief"})
			navigation["patch_query"] = toolcmd.PatchOutlineVolume(volumeID)
			navigation["patch_shape"] = toolOutlineChapterPatchShape(targetID, issue)
			return navigation
		}
		if ToolLooksLikeVolumeID(targetID) {
			navigation["target_kind"] = "volume"
			navigation["detail_queries"] = []string{
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: targetID, View: "brief"}),
			}
			if category == "redundancy" {
				navigation["detail_queries"] = append(navigation["detail_queries"].([]string),
					toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", VolumeID: targetID, Fields: "event_index,chapter_id,type,action,actor,target,target_type,details,result", View: "brief"}),
				)
				navigation["patch_shape"] = map[string]interface{}{
					"changed_events": []map[string]interface{}{{
						"chapter_id":  "<chapter id from events query>",
						"event_index": 0,
						"type":        "<more specific event type>",
						"action":      "<more specific action>",
					}},
				}
			}
			navigation["focused_check_query"] = toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: targetID, Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 12})
			navigation["repair_route_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: targetID, Name: categoryOrAll(category), View: "index"})
			navigation["repair_context_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: targetID, Name: categoryOrAll(category), View: "brief"})
			navigation["patch_query"] = toolcmd.PatchOutlineVolume(targetID)
			return navigation
		}
		if nav := toolOutlineNamedGlobalIssueNavigation(kind, scope, checkID, targetID, category); len(nav) > 0 {
			return nav
		}
		navigation["target_kind"] = "outline"
		navigation["detail_queries"] = []string{
			"novelgen tool query outline --view index",
		}
	case "craft":
		navigation["target_kind"] = "craft." + scope
		contextType := "craft-" + scope
		navigation["detail_queries"] = []string{
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: contextType, Name: targetID, View: "index"}),
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: contextType, Name: targetID, View: "brief"}),
			toolcmd.QueryCraft(toolcmd.QueryOptions{Type: scope, Name: targetID, View: "brief"}),
		}
		navigation["repair_route_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: contextType, Name: targetID, View: "index"})
		navigation["repair_context_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: contextType, Name: targetID, View: "brief"})
		navigation["focused_check_query"] = toolcmd.Check(toolcmd.CheckOptions{Kind: "schema", Target: "craft", Scope: scope, ID: targetID})
		navigation["patch_query"] = toolcmd.PatchCraft(scope, targetID)
	case "setup":
		if nav := toolSetupIssueNavigation(kind, scope, checkID, targetID, targetName, category); len(nav) > 0 {
			return nav
		}
		searchName := targetID
		if targetName != "" && !strings.EqualFold(targetName, targetID) {
			searchName = targetName
		}
		navigation["target_kind"] = "setup"
		navigation["detail_queries"] = []string{
			"novelgen tool query story-setup --view index",
			"novelgen tool query story-setup --view brief",
			toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: searchName, View: "index"}),
			toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: searchName, View: "brief"}),
		}
		navigation["focused_check_query"] = toolcmd.Check(toolcmd.CheckOptions{Target: "setup", Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 12})
		navigation["repair_route_query"] = "novelgen tool query story-setup --view index"
		navigation["repair_context_query"] = "novelgen tool query story-setup --view brief"
		navigation["patch_query"] = toolcmd.PatchSetup()
		navigation["patch_shape"] = map[string]interface{}{"<setup_field>": "<new value>"}
		navigation["post_patch_check_query"] = toolcmd.Check(toolcmd.CheckOptions{Target: "setup", Category: categoryOrAll(category), MinPriority: "low", MaxIssues: 12})
	case "recap":
		navigation["target_kind"] = "recap.chapter"
		navigation["detail_queries"] = []string{
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: targetID, View: "index"}),
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: targetID, View: "brief"}),
		}
		navigation["focused_check_query"] = toolcmd.Check(toolcmd.CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: targetID, MinPriority: "low", MaxIssues: 8})
		navigation["repair_route_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: targetID, View: "index"})
		navigation["repair_context_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: targetID, View: "brief"})
		navigation["patch_query"] = toolcmd.PatchRecap(targetID)
		navigation["patch_shape"] = map[string]interface{}{
			"location":          "<scene anchor>",
			"present":           []string{"<character>"},
			"last_line":         "<final visible line>",
			"next_opening_hint": "<continuation from last_line>",
		}
		navigation["regenerate_query"] = fmt.Sprintf("novelgen recap gen --chapter %q --source chapters", targetID)
	case "chapter":
		navigation["target_kind"] = "final_chapter"
		navigation["detail_queries"] = []string{
			toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-write", ID: targetID, View: "brief"}),
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: targetID, View: "brief"}),
		}
		chapterCategory := ChapterRepairIssueCategoryFilter(category)
		simulationQuery := toolcmd.ChapterCheck(targetID, toolcmd.ChapterCheckOptions{Kind: "simulation", Category: chapterCategory, MinPriority: "low", MaxIssues: 8})
		switch kind {
		case "simulation":
			navigation["focused_check_query"] = simulationQuery
		case "all":
			navigation["focused_check_query"] = toolcmd.ChapterCheck(targetID, toolcmd.ChapterCheckOptions{Category: chapterCategory, MinPriority: "low", MaxIssues: 8, TargetWords: targetWords})
		default:
			navigation["focused_check_query"] = toolcmd.ChapterCheck(targetID, toolcmd.ChapterCheckOptions{Kind: "quality", Category: chapterCategory, MinPriority: "low", MaxIssues: 8, TargetWords: targetWords})
		}
		navigation["simulation_check_query"] = simulationQuery
		if (kind == "simulation" || kind == "all") && issueNeedsChapterDSLRefresh(issue) {
			navigation["refresh_query"] = toolcmd.ChapterDSLRefresh(targetID)
			navigation["post_refresh_check_query"] = simulationQuery
		}
		navigation["repair_route_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-repair", ID: targetID, Name: categoryOrAll(category), View: "index"})
		navigation["repair_context_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-repair", ID: targetID, Name: categoryOrAll(category), View: "brief"})
		navigation["patch_query"] = toolcmd.Patch(toolcmd.PatchOptions{Section: "chapter", ID: targetID, TargetWords: targetWords})
		navigation["patch_shape"] = map[string]interface{}{
			"content": "<complete revised chapter markdown or prose>",
		}
		navigation["repair_command"] = fmt.Sprintf("novelgen write improve --agent-sdk --chapter %q --max-rounds 1%s", targetID, writeTargetWordsFlagSuffix(targetWords))
	}
	if len(navigation) == 0 {
		return nil
	}
	return navigation
}

func toolTargetWordsFlagSuffix(targetWords int) string {
	if targetWords <= 0 {
		return ""
	}
	return fmt.Sprintf(" --target-words %d", targetWords)
}

func writeTargetWordsFlagSuffix(targetWords int) string {
	if targetWords <= 0 {
		return ""
	}
	return fmt.Sprintf(" --words %d", targetWords)
}

func toolOutlineChapterPatchShape(targetID string, issue models.ReviewSuggestion) map[string]interface{} {
	chapterPatch := map[string]string{"id": targetID}
	if toolOutlineChapterIssueNeedsOpeningBeat(issue) {
		chapterPatch["opening_beat"] = "<rewrite the chapter opening beat; for continuity include 继续/随后/紧接着, and for location moves include 来到/前往/到达>"
	}
	return map[string]interface{}{
		"changed_chapters": []map[string]string{chapterPatch},
	}
}

func toolOutlineChapterIssueNeedsOpeningBeat(issue models.ReviewSuggestion) bool {
	category := NormalizeKey(issue.Category)
	text := strings.Join([]string{issue.Issue, issue.Suggestion}, " ")
	return category == "transition" ||
		strings.Contains(text, "章节开场") ||
		strings.Contains(text, "开场节拍") ||
		strings.Contains(text, "缺少与上一章的过渡") ||
		strings.Contains(text, "缺少过渡描述") ||
		strings.Contains(text, "地点从")
}

func toolSetupIssueNavigation(kind, scope, checkID, targetID, targetName, category string) map[string]interface{} {
	targetID = strings.TrimSpace(targetID)
	targetName = strings.TrimSpace(targetName)
	category = categoryOrAll(category)
	targetKind, queryType, queryName, patchShape := toolSetupIssueRouteParts(targetID, targetName, category)
	if targetKind == "" || queryName == "" {
		return nil
	}
	routeQuery := toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: queryType, Name: queryName, View: "index"})
	contextQuery := toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: queryType, Name: queryName, View: "brief"})
	return map[string]interface{}{
		"target_kind":            targetKind,
		"detail_queries":         []string{routeQuery, contextQuery, "novelgen tool query story-setup --view index"},
		"repair_route_query":     routeQuery,
		"repair_context_query":   contextQuery,
		"focused_check_query":    toolFocusedCheckCommand(kind, "setup", scope, checkID, category, 12),
		"patch_query":            toolcmd.PatchSetup(),
		"patch_shape":            patchShape,
		"post_patch_check_query": toolFocusedCheckCommand(kind, "setup", scope, checkID, category, 12),
	}
}

func toolSetupIssueRouteParts(targetID, targetName, category string) (string, string, string, map[string]interface{}) {
	base, indexed := toolSetupIndexedTargetBase(targetID)
	index, hasIndex := toolSetupIndexedTargetIndex(targetID)
	name := strings.TrimSpace(targetName)
	if name == "" || strings.EqualFold(name, "global") {
		name = strings.TrimSpace(targetID)
	}
	switch {
	case indexed && base == "rules" && hasIndex:
		return "setup.rule", "search", targetID, map[string]interface{}{
			"rules_patch": []map[string]interface{}{{
				"index": index,
				"value": "<shortened rule text>",
			}},
		}
	case indexed && base == "genres" && hasIndex:
		return "setup.genre", "search", targetID, map[string]interface{}{
			"genres_patch": []map[string]interface{}{{
				"index": index,
				"value": "<genre>",
			}},
		}
	case indexed && base == "storylines" && name != "":
		return "setup.storyline", "storyline", name, map[string]interface{}{
			"storylines": []map[string]interface{}{toolSetupStorylinePatchShape(name, category)},
		}
	case indexed && base == "premises" && name != "":
		return "setup.premise", "premise", name, map[string]interface{}{
			"premises": []map[string]interface{}{toolSetupPremisePatchShape(name, category)},
		}
	case indexed && base == "core_cast" && name != "":
		return "setup.core_cast", "core-cast", name, map[string]interface{}{
			"core_cast": []map[string]interface{}{{
				"name":           name,
				"role":           "<role>",
				"importance":     5,
				"story_function": "<why this character exists in the story engine>",
				"entry_phase":    "<opening|early|mid|late|series>",
				"payoff":         "<promised payoff>",
			}},
		}
	case indexed && base == "world_resources" && name != "":
		return "setup.world_resource", "resource", name, map[string]interface{}{
			"world_resources": []map[string]interface{}{{
				"name":        name,
				"category":    "<resource category>",
				"scarcity":    "<common|uncommon|rare|unique>",
				"description": "<compact function and source>",
			}},
		}
	case indexed && base == "world_timeline" && name != "":
		return "setup.world_timeline", "timeline", name, map[string]interface{}{
			"world_timeline": []map[string]interface{}{{
				"event":  name,
				"year":   "<time marker>",
				"impact": "<current-world impact>",
			}},
		}
	case NormalizeKey(targetID) == "core_cast":
		return "setup.core_cast", "search", "core_cast", map[string]interface{}{
			"core_cast": []map[string]interface{}{{
				"name":           "<protagonist or major character>",
				"role":           "<protagonist|lead|rival|mentor|antagonist>",
				"importance":     8,
				"story_function": "<why this character exists in the story engine>",
				"entry_phase":    "<opening|early|mid|late|series>",
				"payoff":         "<promised payoff>",
			}},
		}
	case NormalizeKey(targetID) == "long_form_plan":
		return "setup.long_form_plan", "long-form-plan", "long_form_plan", map[string]interface{}{
			"long_form_plan": map[string]interface{}{
				"target_chapters":   300,
				"target_volumes":    6,
				"main_loop":         "<repeatable challenge/exploit/win/reward loop>",
				"escalation_ladder": []string{"<local scope>", "<larger scope>"},
				"reader_promises":   []string{"<repeatable appeal>"},
				"payoff_cadence":    "<small/medium/major payoff rhythm>",
				"volume_pattern":    []string{"<hook>", "<pressure>", "<win>", "<reward>", "<next gate>"},
			},
		}
	default:
		return "", "", "", nil
	}
}

func toolSetupIndexedTargetBase(targetID string) (string, bool) {
	targetID = strings.TrimSpace(targetID)
	open := strings.Index(targetID, "[")
	if open <= 0 || !strings.HasSuffix(targetID, "]") {
		return "", false
	}
	return NormalizeKey(targetID[:open]), true
}

func toolSetupIndexedTargetIndex(targetID string) (int, bool) {
	targetID = strings.TrimSpace(targetID)
	open := strings.Index(targetID, "[")
	close := strings.LastIndex(targetID, "]")
	if open < 0 || close <= open+1 {
		return 0, false
	}
	index, err := strconv.Atoi(targetID[open+1 : close])
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

func toolSetupStorylinePatchShape(name, category string) map[string]interface{} {
	shape := map[string]interface{}{"name": name}
	switch category {
	case "plot":
		shape["repeatable_pressure"] = "<how this storyline repeatedly creates pressure>"
	case "appeal":
		shape["payoff_cadence"] = "<how often partial or major payoffs land>"
		shape["appeal_engine"] = toolSetupAppealEnginePatchShape()
	case "structure":
		shape["mutation"] = "<how this storyline changes after the initial pattern gets stale>"
		shape["failure_mode"] = "<most likely repetitive or inconsistent failure mode>"
	default:
		shape["description"] = "<compact storyline contract>"
		shape["repeatable_pressure"] = "<repeatable pressure>"
		shape["payoff_cadence"] = "<payoff rhythm>"
	}
	return shape
}

func toolSetupPremisePatchShape(name, category string) map[string]interface{} {
	shape := map[string]interface{}{"name": name}
	switch category {
	case "appeal":
		shape["appeal_engine"] = toolSetupAppealEnginePatchShape()
	case "logic":
		shape["progression"] = []map[string]interface{}{{
			"level":        1,
			"name":         "<stage name>",
			"description":  "<stage boundary and story use>",
			"requirements": "<cost or unlock condition>",
		}}
	default:
		shape["category"] = "<premise category>"
		shape["description"] = "<compact system contract>"
		shape["progression"] = []map[string]interface{}{{
			"level":       1,
			"name":        "<stage name>",
			"description": "<stage boundary and story use>",
		}}
	}
	return shape
}

func toolSetupAppealEnginePatchShape() map[string]string {
	return map[string]string{
		"appeal":           "<reader-facing fun>",
		"surface_limit":    "<visible limit or rule boundary>",
		"exploit":          "<how protagonist uses the rule>",
		"signature_win":    "<concrete win image>",
		"upgrade_path":     "<how it scales without breaking>",
		"opponent_misread": "<what enemies misunderstand>",
		"reward_type":      "<resource|status|secret|ally|territory|freedom>",
	}
}

func toolOutlineNamedGlobalIssueNavigation(kind, scope, checkID, targetID, category string) map[string]interface{} {
	targetID = strings.TrimSpace(targetID)
	category = NormalizeKey(category)
	if targetID == "" {
		return nil
	}
	switch category {
	case "faction_tier":
		searchIndex := toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: targetID, View: "index"})
		searchBrief := toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: targetID, View: "brief"})
		return map[string]interface{}{
			"target_kind":            "setup.faction_tier",
			"detail_queries":         []string{searchIndex, searchBrief, "novelgen tool query story-setup --view index"},
			"repair_route_query":     searchIndex,
			"repair_context_query":   searchBrief,
			"focused_check_query":    toolFocusedCheckCommand(kind, "outline", scope, checkID, category, 12),
			"patch_query":            toolcmd.PatchSetup(),
			"post_patch_check_query": toolFocusedCheckCommand(kind, "outline", scope, checkID, category, 12),
			"patch_shape": map[string]interface{}{
				"premises": []map[string]string{{
					"name":        targetID + " faction tier ladder",
					"category":    targetID,
					"description": "<define stable tier ladder for " + targetID + ">",
				}},
			},
		}
	default:
		return nil
	}
}

func toolGlobalIssueNavigation(kind, target, scope, checkID, category string) map[string]interface{} {
	category = categoryOrAll(category)
	navigation := map[string]interface{}{
		"target_kind":         target + ".global",
		"focused_check_query": toolFocusedCheckCommand(kind, target, scope, checkID, category, 12),
	}
	switch target {
	case "outline":
		queries := []string{}
		if scope == "volume" && checkID != "" {
			queries = append(queries,
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: checkID, View: "index"}),
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: checkID, View: "brief"}),
			)
			navigation["repair_route_query"] = queries[0]
			navigation["repair_context_query"] = queries[1]
			navigation["patch_query"] = toolcmd.PatchOutlineVolume(checkID)
			navigation["patch_shape"] = map[string]interface{}{"changed_chapters": []map[string]string{{"id": "<chapter_id>"}}}
		} else if scope == "chapter" && checkID != "" {
			volumeID, _ := ToolVolumeIDFromChapterID(checkID)
			queries = append(queries,
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: checkID, Name: category, View: "index"}),
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-repair", ID: checkID, Name: category, View: "brief"}),
			)
			navigation["repair_route_query"] = queries[0]
			navigation["repair_context_query"] = queries[1]
			if volumeID != "" {
				navigation["patch_query"] = toolcmd.PatchOutlineVolume(volumeID)
			}
		} else {
			globalIndex := toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-global-repair", Name: category, View: "index"})
			queries = append(queries, globalIndex)
			navigation["repair_route_query"] = globalIndex
			navigation["classification_rule"] = "Use the outline-global-repair index route. If it has no patch_query plus patch_shape, do not fetch brief context; return the issue as unpatchable or route to a smaller workflow."
		}
		navigation["detail_queries"] = queries
	case "setup":
		navigation["detail_queries"] = []string{
			"novelgen tool query story-setup --view index",
			"novelgen tool query story-setup --view brief",
		}
		navigation["repair_route_query"] = "novelgen tool query story-setup --view index"
		navigation["repair_context_query"] = "novelgen tool query story-setup --view brief"
		navigation["patch_query"] = toolcmd.PatchSetup()
		navigation["patch_shape"] = map[string]interface{}{"<setup_field>": "<new value>"}
	case "chapter":
		if checkID != "" {
			navigation["detail_queries"] = []string{
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-repair", ID: checkID, Name: category, View: "index"}),
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-repair", ID: checkID, Name: category, View: "brief"}),
			}
			navigation["repair_route_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-repair", ID: checkID, Name: category, View: "index"})
			navigation["repair_context_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "chapter-repair", ID: checkID, Name: category, View: "brief"})
			navigation["patch_query"] = toolcmd.PatchChapter(checkID)
			navigation["patch_shape"] = map[string]interface{}{"content": "<complete revised chapter markdown or prose>"}
		}
	case "recap":
		if checkID != "" {
			navigation["detail_queries"] = []string{
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: checkID, View: "index"}),
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: checkID, View: "brief"}),
			}
			navigation["repair_route_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: checkID, View: "index"})
			navigation["repair_context_query"] = toolcmd.QueryContext(toolcmd.QueryOptions{Type: "recap-repair", ID: checkID, View: "brief"})
			navigation["patch_query"] = toolcmd.PatchRecap(checkID)
		}
	case "craft":
		if scope != "" && scope != "all" {
			query := toolcmd.QueryCraft(toolcmd.QueryOptions{Type: scope, View: "index"})
			navigation["detail_queries"] = []string{query}
			navigation["repair_route_query"] = query
		}
	}
	return navigation
}

func toolFocusedCheckCommand(kind, target, scope, id, category string, maxIssues int) string {
	if maxIssues <= 0 {
		maxIssues = 12
	}
	if scope == "all" {
		scope = ""
	}
	return toolcmd.Check(toolcmd.CheckOptions{Kind: kind, Target: target, Scope: scope, ID: id, Category: category, MinPriority: "low", MaxIssues: maxIssues})
}
