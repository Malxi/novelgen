package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/logic/style"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/quality"
	rpgdsl "novelgen/internal/rpg/dsl"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var toolCheckFlags struct {
	Target      string
	Scope       string
	ID          string
	MinPriority string
	Category    string
	MaxIssues   int
	TargetWords int
}

var toolPatchFlags struct {
	Target         string
	ID             string
	PatchJSON      string
	PatchBuffer    string
	Task           string
	Apply          bool
	DryRun         bool
	RefreshDerived bool
	TargetWords    int
}

var toolPatchBufferFlags struct {
	ID       string
	Text     string
	Stdin    bool
	MaxChars int
}

type toolCheckResult struct {
	Kind     string                    `json:"kind"`
	Target   string                    `json:"target"`
	Scope    string                    `json:"scope"`
	ID       string                    `json:"id,omitempty"`
	OK       bool                      `json:"ok"`
	Blocking bool                      `json:"blocking"`
	Score    float64                   `json:"score"`
	Summary  toolCheckSummary          `json:"summary"`
	Issues   []models.ReviewSuggestion `json:"issues,omitempty"`
	Meta     map[string]interface{}    `json:"meta,omitempty"`
}

type toolCheckIssue struct {
	models.ReviewSuggestion
	Navigation map[string]interface{} `json:"navigation,omitempty"`
}

type toolCheckResultJSON struct {
	Kind        string                 `json:"kind"`
	Target      string                 `json:"target"`
	Scope       string                 `json:"scope"`
	ID          string                 `json:"id,omitempty"`
	OK          bool                   `json:"ok"`
	Blocking    bool                   `json:"blocking"`
	Score       float64                `json:"score"`
	Summary     toolCheckSummary       `json:"summary"`
	Issues      []toolCheckIssue       `json:"issues,omitempty"`
	NextActions []toolNextAction       `json:"next_actions,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type toolCheckSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

type toolPatchResult struct {
	OK          bool                   `json:"ok"`
	Applied     bool                   `json:"applied"`
	DryRun      bool                   `json:"dry_run"`
	Target      string                 `json:"target"`
	ID          string                 `json:"id"`
	Changed     []toolPatchChange      `json:"changed,omitempty"`
	Check       *toolCheckResult       `json:"check,omitempty"`
	Files       map[string]string      `json:"files,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	NextActions []toolNextAction       `json:"next_actions,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type toolPatchChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

func (r toolCheckResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(toolCheckResultJSON{
		Kind:        r.Kind,
		Target:      r.Target,
		Scope:       r.Scope,
		ID:          r.ID,
		OK:          r.OK,
		Blocking:    r.Blocking,
		Score:       r.Score,
		Summary:     r.Summary,
		Issues:      toolCheckIssuesForJSON(r),
		NextActions: toolCheckNextActions(r),
		Meta:        toolCheckMetaForJSON(r),
	})
}

func toolCheckMetaForJSON(result toolCheckResult) map[string]interface{} {
	meta := map[string]interface{}{}
	for key, value := range result.Meta {
		meta[key] = value
	}
	meta["check_budget"] = map[string]interface{}{
		"strategy":            "patchable_issue_navigation_first",
		"current_issue_count": len(result.Issues),
		"summary_total":       result.Summary.Total,
		"max_repair_issues":   3,
		"prefer_commands_in":  []string{"next_actions", "issues[].navigation"},
		"avoid":               []string{"full_project_setup", "full_outline", "all_chapters", "source_code_search", "manual_rpg_file_reads"},
	}
	return meta
}

func toolCheckNextActions(result toolCheckResult) []toolNextAction {
	if len(result.Issues) == 0 {
		return []toolNextAction{{
			Step:    1,
			Action:  "return_final_json",
			Purpose: "No returned issues match the current check filters; stop tool exploration and return the workflow JSON.",
		}}
	}
	selectedIssue, nav, selectedIndex, selectedPatchable := toolCheckPreferredIssueNavigation(result)
	firstAction := toolNextAction{
		Step:    1,
		Action:  "inspect_first_returned_issue_route",
		Purpose: "Use the first returned issue navigation before querying broader context; patch only when navigation includes a validated patch_shape.",
	}
	if selectedPatchable {
		firstAction.Action = "repair_first_patchable_issue"
		firstAction.Purpose = fmt.Sprintf("Use issue #%d because it has a validated patch route; repair issues one at a time before querying broader context.", selectedIndex+1)
	} else if selectedIndex > 0 {
		firstAction.Action = "inspect_priority_issue_route"
		firstAction.Purpose = fmt.Sprintf("Use issue #%d because it is the highest-priority routed issue; inspect it without assuming a patch is available.", selectedIndex+1)
	}
	if strings.TrimSpace(selectedIssue.Category) != "" {
		firstAction.When = "Selected category: " + normalizeKey(selectedIssue.Category)
	}
	actions := []toolNextAction{{
		Step:    firstAction.Step,
		Action:  firstAction.Action,
		Purpose: firstAction.Purpose,
		When:    firstAction.When,
	}}
	if refreshCommand := stringMapValue(nav, "refresh_query"); refreshCommand != "" {
		actions[0].Action = "refresh_derived_dsl_first"
		actions[0].Purpose = "Refresh derived chapter RPG DSL before querying repair context or patching prose; stale/missing DSL can be a cache problem."
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "refresh_derived_dsl",
			Command: refreshCommand,
			Purpose: "Regenerate the selected chapter's derived RPG DSL from saved prose.",
		})
		postRefreshCommand := stringMapValue(nav, "post_refresh_check_query")
		if postRefreshCommand == "" {
			postRefreshCommand = stringMapValue(nav, "focused_check_query")
		}
		if postRefreshCommand != "" {
			actions = append(actions, toolNextAction{
				Step:    len(actions) + 1,
				Action:  "post_refresh_check",
				Command: postRefreshCommand,
				Purpose: "Rerun the focused simulation check after refresh; repair prose only if this check still returns issues.",
			})
		}
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "return_final_json",
			Purpose: "If the post-refresh check is clean, report that refresh resolved the issue; otherwise continue from the returned check.issues navigation.",
		})
		return actions
	}
	if command := stringMapValue(nav, "repair_route_query"); command != "" {
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "query_repair_route",
			Command: command,
			Purpose: "Fetch the smallest route map for the failing target.",
		})
	}
	if command := stringMapValue(nav, "repair_context_query"); command != "" {
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "query_repair_context_if_needed",
			Command: command,
			Purpose: "Fetch focused facts only if the route or issue cannot be repaired from current data.",
			When:    "The route/check summary says target facts, event facts, or excerpts are needed.",
		})
	}
	if command := stringMapValue(nav, "patch_query"); command != "" && selectedPatchable {
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "patch_dry_run",
			Command: command,
			Purpose: "Validate the minimal patch for this issue before any apply.",
			When:    "A concrete patch is ready.",
		})
	}
	if command := stringMapValue(nav, "focused_check_query"); command != "" {
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "focused_recheck",
			Command: command,
			Purpose: "Verify the repaired target before moving to the next issue.",
			When:    "After a patch dry-run/apply or when confirming whether the issue still exists.",
		})
	}
	if !selectedPatchable && stringMapValue(nav, "classification_rule") != "" {
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "return_final_json",
			Purpose: "This global route has no validated patch shape. Report it as remaining/unpatchable or recommend a smaller targeted workflow instead of fetching broader context.",
		})
	}
	return actions
}

func toolCheckPreferredIssueNavigation(result toolCheckResult) (models.ReviewSuggestion, map[string]interface{}, int, bool) {
	targetWords := toolCheckResultTargetWords(result)
	bestIssue := result.Issues[0]
	bestNav := toolIssueNavigation(result.Kind, result.Target, result.Scope, result.ID, bestIssue, targetWords)
	bestIndex := 0
	bestPatchPriority := -1
	foundPatch := false
	bestRoutablePriority := -1
	foundRoutable := false
	if toolCheckNavigationIsPatchable(bestNav) {
		bestPatchPriority = toolCheckIssuePriorityScore(bestIssue.Priority)
		foundPatch = true
	} else if toolCheckNavigationIsRoutable(bestNav) {
		bestRoutablePriority = toolCheckIssuePriorityScore(bestIssue.Priority)
		foundRoutable = true
	}

	routableIssue := bestIssue
	routableNav := bestNav
	routableIndex := 0
	for i := 1; i < len(result.Issues); i++ {
		issue := result.Issues[i]
		nav := toolIssueNavigation(result.Kind, result.Target, result.Scope, result.ID, issue, targetWords)
		priority := toolCheckIssuePriorityScore(issue.Priority)
		if toolCheckNavigationIsPatchable(nav) {
			if !foundPatch || priority > bestPatchPriority {
				bestIssue = issue
				bestNav = nav
				bestIndex = i
				bestPatchPriority = priority
				foundPatch = true
			}
			continue
		}
		if toolCheckNavigationIsRoutable(nav) && (!foundRoutable || priority > bestRoutablePriority) {
			routableIssue = issue
			routableNav = nav
			routableIndex = i
			bestRoutablePriority = priority
			foundRoutable = true
		}
	}
	if foundPatch {
		return bestIssue, bestNav, bestIndex, true
	}
	if foundRoutable {
		return routableIssue, routableNav, routableIndex, false
	}
	return bestIssue, bestNav, bestIndex, false
}

func toolCheckNavigationIsPatchable(nav map[string]interface{}) bool {
	return stringMapValue(nav, "patch_query") != "" && nav["patch_shape"] != nil
}

func toolCheckNavigationIsRoutable(nav map[string]interface{}) bool {
	return stringMapValue(nav, "repair_route_query") != "" || stringMapValue(nav, "focused_check_query") != ""
}

func toolCheckIssuePriorityScore(priority string) int {
	switch normalizeKey(priority) {
	case models.PriorityCritical, "fatal", "blocker", "serious":
		return 4
	case models.PriorityHigh:
		return 3
	case models.PriorityMedium:
		return 2
	case models.PriorityLow:
		return 1
	default:
		return 0
	}
}

func toolCheckResultTargetWords(result toolCheckResult) int {
	if result.Meta == nil {
		return 0
	}
	value, ok := result.Meta["target_words"]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func toolCheckIssuesForJSON(result toolCheckResult) []toolCheckIssue {
	if len(result.Issues) == 0 {
		return nil
	}
	issues := make([]toolCheckIssue, 0, len(result.Issues))
	targetWords := toolCheckResultTargetWords(result)
	for _, issue := range result.Issues {
		issues = append(issues, toolCheckIssue{
			ReviewSuggestion: issue,
			Navigation:       toolIssueNavigation(result.Kind, result.Target, result.Scope, result.ID, issue, targetWords),
		})
	}
	return issues
}

func toolIssueNavigation(kind, target, scope, checkID string, issue models.ReviewSuggestion, targetWords int) map[string]interface{} {
	kind = normalizeKey(kind)
	target = normalizeKey(target)
	scope = normalizeToolScope(scope)
	checkID = strings.TrimSpace(checkID)
	targetID := strings.TrimSpace(issue.TargetID)
	targetName := strings.TrimSpace(issue.TargetName)
	category := normalizeKey(issue.Category)
	navigation := map[string]interface{}{}
	if targetID == "" || strings.EqualFold(targetID, "global") {
		return toolGlobalIssueNavigation(kind, target, scope, checkID, category)
	}

	switch target {
	case "outline":
		if volumeID, ok := toolVolumeIDFromChapterID(targetID); ok {
			navigation["target_kind"] = "chapter"
			navigation["volume_id"] = volumeID
			navigation["detail_queries"] = []string{
				fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", targetID),
				fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --fields result,details,target,target_type,actor,action --view brief", targetID),
			}
			navigation["focused_check_query"] = fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --category %s --min-priority low --max-issues 8", targetID, categoryOrAll(category))
			navigation["repair_route_query"] = fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view index", targetID, categoryOrAll(category))
			navigation["repair_context_query"] = fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", targetID, categoryOrAll(category))
			navigation["patch_query"] = fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID)
			navigation["patch_shape"] = toolOutlineChapterPatchShape(targetID, issue)
			return navigation
		}
		if toolLooksLikeVolumeID(targetID) {
			navigation["target_kind"] = "volume"
			navigation["detail_queries"] = []string{
				fmt.Sprintf("novelgen tool query context --type outline-volume --id %q --view brief", targetID),
			}
			if category == "redundancy" {
				navigation["detail_queries"] = append(navigation["detail_queries"].([]string),
					fmt.Sprintf("novelgen tool query outline --type events --volume-id %q --fields event_index,chapter_id,type,action,actor,target,target_type,details,result --view brief", targetID),
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
			navigation["focused_check_query"] = fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --category %s --min-priority low --max-issues 12", targetID, categoryOrAll(category))
			navigation["repair_route_query"] = fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view index", targetID, categoryOrAll(category))
			navigation["repair_context_query"] = fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", targetID, categoryOrAll(category))
			navigation["patch_query"] = fmt.Sprintf("novelgen tool patch outline --target volume --id %q", targetID)
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
			fmt.Sprintf("novelgen tool query context --type %s --name %q --view index", contextType, targetID),
			fmt.Sprintf("novelgen tool query context --type %s --name %q --view brief", contextType, targetID),
			fmt.Sprintf("novelgen tool query craft --type %s --name %q --view brief", scope, targetID),
		}
		navigation["repair_route_query"] = fmt.Sprintf("novelgen tool query context --type %s --name %q --view index", contextType, targetID)
		navigation["repair_context_query"] = fmt.Sprintf("novelgen tool query context --type %s --name %q --view brief", contextType, targetID)
		navigation["focused_check_query"] = fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", scope, targetID)
		navigation["patch_query"] = fmt.Sprintf("novelgen tool patch craft --target %s --id %q", scope, targetID)
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
			fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view index", searchName),
			fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view brief", searchName),
		}
		navigation["focused_check_query"] = fmt.Sprintf("novelgen tool check all --target setup --category %s --min-priority low --max-issues 12", categoryOrAll(category))
		navigation["repair_route_query"] = "novelgen tool query story-setup --view index"
		navigation["repair_context_query"] = "novelgen tool query story-setup --view brief"
		navigation["patch_query"] = "novelgen tool patch setup"
		navigation["patch_shape"] = map[string]interface{}{"<setup_field>": "<new value>"}
		navigation["post_patch_check_query"] = fmt.Sprintf("novelgen tool check all --target setup --category %s --min-priority low --max-issues 12", categoryOrAll(category))
	case "recap":
		navigation["target_kind"] = "recap.chapter"
		navigation["detail_queries"] = []string{
			fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view index", targetID),
			fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", targetID),
		}
		navigation["focused_check_query"] = fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", targetID)
		navigation["repair_route_query"] = fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view index", targetID)
		navigation["repair_context_query"] = fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", targetID)
		navigation["patch_query"] = fmt.Sprintf("novelgen tool patch recap --id %q", targetID)
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
			fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief", targetID),
			fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", targetID),
		}
		chapterCategory := chapterRepairIssueCategoryFilter(category)
		simulationQuery := fmt.Sprintf("novelgen tool check simulation --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 8", targetID, chapterCategory)
		checkTargetWords := toolTargetWordsFlagSuffix(targetWords)
		switch kind {
		case "simulation":
			navigation["focused_check_query"] = simulationQuery
		case "all":
			navigation["focused_check_query"] = fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 8%s", targetID, chapterCategory, checkTargetWords)
		default:
			navigation["focused_check_query"] = fmt.Sprintf("novelgen tool check quality --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 8%s", targetID, chapterCategory, checkTargetWords)
		}
		navigation["simulation_check_query"] = simulationQuery
		if (kind == "simulation" || kind == "all") && issueNeedsChapterDSLRefresh(issue) {
			navigation["refresh_query"] = fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", targetID)
			navigation["post_refresh_check_query"] = simulationQuery
		}
		navigation["repair_route_query"] = fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view index", targetID, categoryOrAll(category))
		navigation["repair_context_query"] = fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view brief", targetID, categoryOrAll(category))
		navigation["patch_query"] = fmt.Sprintf("novelgen tool patch chapter --id %q%s", targetID, checkTargetWords)
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
	category := normalizeKey(issue.Category)
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
	routeQuery := fmt.Sprintf("novelgen tool query story-setup --type %s --name %q --view index", queryType, queryName)
	contextQuery := fmt.Sprintf("novelgen tool query story-setup --type %s --name %q --view brief", queryType, queryName)
	return map[string]interface{}{
		"target_kind":            targetKind,
		"detail_queries":         []string{routeQuery, contextQuery, "novelgen tool query story-setup --view index"},
		"repair_route_query":     routeQuery,
		"repair_context_query":   contextQuery,
		"focused_check_query":    toolFocusedCheckCommand(kind, "setup", scope, checkID, category, 12),
		"patch_query":            "novelgen tool patch setup",
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
	case normalizeKey(targetID) == "core_cast":
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
	case normalizeKey(targetID) == "long_form_plan":
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
	return normalizeKey(targetID[:open]), true
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
	category = normalizeKey(category)
	if targetID == "" {
		return nil
	}
	switch category {
	case "faction_tier":
		searchIndex := fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view index", targetID)
		searchBrief := fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view brief", targetID)
		return map[string]interface{}{
			"target_kind":            "setup.faction_tier",
			"detail_queries":         []string{searchIndex, searchBrief, "novelgen tool query story-setup --view index"},
			"repair_route_query":     searchIndex,
			"repair_context_query":   searchBrief,
			"focused_check_query":    toolFocusedCheckCommand(kind, "outline", scope, checkID, category, 12),
			"patch_query":            "novelgen tool patch setup",
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
				fmt.Sprintf("novelgen tool query context --type outline-volume --id %q --view index", checkID),
				fmt.Sprintf("novelgen tool query context --type outline-volume --id %q --view brief", checkID),
			)
			navigation["repair_route_query"] = queries[0]
			navigation["repair_context_query"] = queries[1]
			navigation["patch_query"] = fmt.Sprintf("novelgen tool patch outline --target volume --id %q", checkID)
			navigation["patch_shape"] = map[string]interface{}{"changed_chapters": []map[string]string{{"id": "<chapter_id>"}}}
		} else if scope == "chapter" && checkID != "" {
			volumeID, _ := toolVolumeIDFromChapterID(checkID)
			queries = append(queries,
				fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view index", checkID, category),
				fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", checkID, category),
			)
			navigation["repair_route_query"] = queries[0]
			navigation["repair_context_query"] = queries[1]
			if volumeID != "" {
				navigation["patch_query"] = fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID)
			}
		} else {
			globalIndex := fmt.Sprintf("novelgen tool query context --type outline-global-repair --name %q --view index", category)
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
		navigation["patch_query"] = "novelgen tool patch setup"
		navigation["patch_shape"] = map[string]interface{}{"<setup_field>": "<new value>"}
	case "chapter":
		if checkID != "" {
			navigation["detail_queries"] = []string{
				fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view index", checkID, category),
				fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view brief", checkID, category),
			}
			navigation["repair_route_query"] = fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view index", checkID, category)
			navigation["repair_context_query"] = fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view brief", checkID, category)
			navigation["patch_query"] = fmt.Sprintf("novelgen tool patch chapter --id %q", checkID)
			navigation["patch_shape"] = map[string]interface{}{"content": "<complete revised chapter markdown or prose>"}
		}
	case "recap":
		if checkID != "" {
			navigation["detail_queries"] = []string{
				fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view index", checkID),
				fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", checkID),
			}
			navigation["repair_route_query"] = fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view index", checkID)
			navigation["repair_context_query"] = fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", checkID)
			navigation["patch_query"] = fmt.Sprintf("novelgen tool patch recap --id %q", checkID)
		}
	case "craft":
		if scope != "" && scope != "all" {
			query := fmt.Sprintf("novelgen tool query craft --type %s --view index", scope)
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
	parts := []string{"novelgen", "tool", "check", kind, "--target", target}
	if scope != "" && scope != "all" {
		parts = append(parts, "--scope", scope)
	}
	if id != "" {
		parts = append(parts, "--id", fmt.Sprintf("%q", id))
	}
	if category != "" {
		parts = append(parts, "--category", category)
	}
	parts = append(parts, "--min-priority", "low", "--max-issues", fmt.Sprintf("%d", maxIssues))
	return strings.Join(parts, " ")
}

func issueNeedsChapterDSLRefresh(issue models.ReviewSuggestion) bool {
	text := strings.ToLower(strings.Join([]string{
		issue.Category,
		issue.Issue,
		issue.Suggestion,
	}, " "))
	return strings.Contains(text, "chapter rpg dsl is stale") ||
		strings.Contains(text, "chapter rpg dsl simulation is unavailable") ||
		strings.Contains(text, "refresh chapter rpg dsl") ||
		strings.Contains(text, "simulation checks do not invoke llm conversion") ||
		strings.Contains(text, "simulation signal diagnostics") ||
		strings.Contains(text, "combat_result=false") ||
		strings.Contains(text, "missing_repair_signals=") ||
		strings.Contains(text, "on_complete narration/result")
}

func categoryOrAll(category string) string {
	if strings.TrimSpace(category) == "" {
		return "logic,plot,structure,character,pacing"
	}
	return category
}

func toolVolumeIDFromChapterID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	idx := strings.LastIndex(strings.ToUpper(id), "-C")
	if idx <= 0 {
		return "", false
	}
	return id[:idx], true
}

func toolLooksLikeVolumeID(id string) bool {
	id = strings.ToUpper(strings.TrimSpace(id))
	return strings.Contains(id, "-V") && !strings.Contains(id, "-C")
}

var toolCheckCmd = &cobra.Command{
	Use:   "check [quality|simulation|all|schema]",
	Short: "Run scoped checks as JSON for agents",
	Long: `Run scoped checks as JSON for agents.

quality checks deterministic structure/contract rules.
simulation checks the same target through the in-memory RPG simulation backend.
all runs both and merges issues.
setup target checks story/setup/story_setup.json.
outline target supports scoped all/volume/chapter checks.
chapter target checks saved final chapter markdown and can simulate existing RPG DSL without invoking LLM.
recap target checks story/recaps/<chapter_id>.json continuity anchors.
schema validates typed craft objects and saved recap JSON already in project state.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runToolCheck,
}

var toolPatchCmd = &cobra.Command{
	Use:   "patch [outline|craft|setup|recap|chapter]",
	Short: "Apply a validated JSON patch to project state",
	Long: `Apply a validated JSON patch to project state.

The tool reads the current project from the workspace. The agent submits only
the target and patch JSON on stdin. Project files are written only with --apply;
without --apply the command performs a dry run and returns the diff/check JSON.
Setup patches do not require --id; outline, craft, and recap patches patch a
specific target object. Chapter patches update saved final chapter markdown with
{"content":"..."} and run deterministic chapter checks.`,
	Args: cobra.ExactArgs(1),
	RunE: runToolPatch,
}

func runToolCheck(cmd *cobra.Command, args []string) error {
	kind := "all"
	if len(args) > 0 {
		kind = normalizeKey(args[0])
	}
	if kind == "" {
		kind = "all"
	}
	if kind != "quality" && kind != "simulation" && kind != "all" && kind != "schema" {
		return fmt.Errorf("unsupported check kind %q", kind)
	}
	target := normalizeKey(toolCheckFlags.Target)
	if target == "" {
		target = "outline"
	}
	if target != "outline" && target != "craft" && target != "setup" && target != "recap" && target != "chapter" {
		return fmt.Errorf("unsupported check target %q", target)
	}
	if (target == "outline" || target == "setup") && kind == "schema" {
		return fmt.Errorf("schema check is only supported for --target craft or --target recap")
	}
	if target == "craft" && kind != "schema" {
		return fmt.Errorf("craft checks currently support only kind schema")
	}
	if target == "recap" && kind == "simulation" {
		return fmt.Errorf("recap checks do not support kind simulation")
	}
	if target == "chapter" && kind != "quality" && kind != "simulation" && kind != "all" {
		return fmt.Errorf("chapter checks currently support only kind quality, simulation, or all")
	}

	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	if target == "craft" {
		result, err := runToolCraftSchemaCheck(root, toolCheckFlags.Scope, toolCheckFlags.ID)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root)
		if err := applyToolCheckIssueFilters(result, toolCheckFlags.MinPriority, toolCheckFlags.Category, toolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "recap" {
		result, err := runToolRecapCheck(root, kind, toolCheckFlags.Scope, toolCheckFlags.ID)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root)
		if err := applyToolCheckIssueFilters(result, toolCheckFlags.MinPriority, toolCheckFlags.Category, toolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "chapter" {
		result, err := runToolChapterCheckWithTargetWords(root, kind, toolCheckFlags.Scope, toolCheckFlags.ID, toolCheckFlags.TargetWords)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root)
		if err := applyToolCheckIssueFilters(result, toolCheckFlags.MinPriority, toolCheckFlags.Category, toolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "setup" {
		ctx := toolProjectContext{Root: root}
		if err := ctx.loadSetup(); err != nil {
			return err
		}
		result, err := runToolSetupCheck(kind, ctx.Setup)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root)
		if err := applyToolCheckIssueFilters(result, toolCheckFlags.MinPriority, toolCheckFlags.Category, toolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}

	ctx := toolProjectContext{Root: root}
	if err := ctx.loadSetup(); err != nil {
		return err
	}
	if err := ctx.loadOutline(); err != nil {
		return err
	}

	result, err := runToolOutlineCheck(kind, ctx.Setup, ctx.Outline, toolCheckFlags.Scope, toolCheckFlags.ID)
	if err != nil {
		return err
	}
	addToolCheckMeta(result, "project_root", root)
	if err := applyToolCheckIssueFilters(result, toolCheckFlags.MinPriority, toolCheckFlags.Category, toolCheckFlags.MaxIssues); err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func applyToolCheckIssueFilters(result *toolCheckResult, minPriority string, category string, maxIssues int) error {
	if result == nil {
		return nil
	}
	minPriority = normalizeKey(minPriority)
	categories := parseToolCheckCategories(category)
	if minPriority == "" && len(categories) == 0 && maxIssues <= 0 {
		return nil
	}
	minRank := 0
	if minPriority != "" {
		rank, ok := reviewPriorityRank(minPriority)
		if !ok {
			return fmt.Errorf("unsupported --min-priority %q (use low, medium, high, critical)", minPriority)
		}
		minRank = rank
	}
	original := len(result.Issues)
	filtered := make([]models.ReviewSuggestion, 0, len(result.Issues))
	for _, issue := range result.Issues {
		if len(categories) > 0 && !categories[normalizeKey(issue.Category)] {
			continue
		}
		if minRank > 0 {
			rank, ok := reviewPriorityRank(issue.Priority)
			if !ok || rank < minRank {
				continue
			}
		}
		filtered = append(filtered, issue)
	}
	if maxIssues > 0 && len(filtered) > maxIssues {
		filtered = filtered[:maxIssues]
	}
	result.Issues = filtered
	if result.Meta == nil {
		result.Meta = map[string]interface{}{}
	}
	result.Meta["issue_filter"] = map[string]interface{}{
		"min_priority":    minPriority,
		"category":        sortedToolCheckCategories(categories),
		"max_issues":      maxIssues,
		"original_issues": original,
		"returned_issues": len(filtered),
	}
	return nil
}

func parseToolCheckCategories(raw string) map[string]bool {
	categories := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		category := normalizeKey(item)
		if category == "" {
			continue
		}
		categories[category] = true
	}
	return categories
}

func sortedToolCheckCategories(categories map[string]bool) []string {
	if len(categories) == 0 {
		return nil
	}
	values := make([]string, 0, len(categories))
	for category := range categories {
		values = append(values, category)
	}
	sort.Strings(values)
	return values
}

func reviewPriorityRank(priority string) (int, bool) {
	switch normalizeKey(priority) {
	case "low":
		return 1, true
	case "medium":
		return 2, true
	case "high":
		return 3, true
	case "critical":
		return 4, true
	default:
		return 0, false
	}
}

type setupStringListPatchItem struct {
	Index int    `json:"index"`
	Value string `json:"value"`
}

type toolChapterPatch struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content"`
}

func runToolSetupCheck(kind string, setup *models.StorySetup) (*toolCheckResult, error) {
	var gate quality.QualityGateResult
	switch kind {
	case "quality":
		gate = quality.RunSetupQualityGate(setup)
	case "simulation":
		gate = quality.RunSetupSimulationGate(setup)
	case "all":
		gate = quality.RunSetupQualityGate(setup)
		sim := quality.RunSetupSimulationGate(setup)
		gate.Add(sim.Suggestions...)
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	default:
		return nil, fmt.Errorf("unsupported setup check kind %q", kind)
	}
	return makeToolCheckResult(kind, "setup", "all", "", gate), nil
}

func runToolOutlineCheck(kind string, setup *models.StorySetup, outline *models.Outline, scope, id string) (*toolCheckResult, error) {
	scope = normalizeToolScope(scope)
	id = strings.TrimSpace(id)
	if scope != "all" && id == "" {
		return nil, fmt.Errorf("--id is required for scope %q", scope)
	}
	scoped, err := scopedOutline(outline, scope, id)
	if err != nil {
		return nil, err
	}
	var gate quality.QualityGateResult
	switch kind {
	case "quality":
		if scope == "all" {
			gate = quality.RunOutlineQualityGate(setup, scoped)
		} else {
			gate = quality.RunScopedOutlineQualityGate(setup, outline)
		}
	case "simulation":
		gate = quality.RunOutlineSimulationGate(setup, outline)
	case "all":
		if scope == "all" {
			gate = quality.RunOutlineQualityGate(setup, scoped)
		} else {
			gate = quality.RunScopedOutlineQualityGate(setup, outline)
		}
		sim := quality.RunOutlineSimulationGate(setup, outline)
		gate.Add(sim.Suggestions...)
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}
	if scope != "all" {
		gate.Suggestions = filterToolScopedSuggestions(gate.Suggestions, scoped, scope, id)
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}
	return makeToolCheckResult(kind, "outline", scope, id, gate), nil
}

func runToolCraftSchemaCheck(root, scope, id string) (*toolCheckResult, error) {
	scope = normalizeCraftPatchTarget(scope)
	if scope == "" {
		return nil, fmt.Errorf("--scope is required for craft schema check (character/item/location/organization)")
	}
	filename, err := craftPatchFilename(scope)
	if err != nil {
		return nil, err
	}
	values, err := loadRawCraftMap(root, filename)
	if err != nil {
		return nil, err
	}
	gate := quality.QualityGateResult{}
	if values == nil {
		values = map[string]map[string]interface{}{}
	}
	knownCharacters := map[string]bool{}
	if scope == "item" {
		knownCharacters = loadCraftCharacterNames(root)
	}
	if strings.TrimSpace(id) != "" {
		key, ok := resolveCraftPatchKey(values, id)
		if !ok {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "schema",
				TargetID:   id,
				TargetName: id,
				Issue:      "craft element not found",
				Suggestion: fmt.Sprintf("Create or patch craft %s %q before checking it.", scope, id),
				Priority:   models.PriorityHigh,
			})
			gate.Blocking = true
			return makeToolCheckResult("schema", "craft", scope, id, gate), nil
		}
		validateCraftSchemaObject(&gate, scope, key, values[key])
		validateCraftConsistencyObject(&gate, scope, key, values[key], knownCharacters)
		validateCraftQualityObject(&gate, scope, key, values[key])
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
		return makeToolCheckResult("schema", "craft", scope, key, gate), nil
	}
	for key, value := range values {
		validateCraftSchemaObject(&gate, scope, key, value)
		validateCraftConsistencyObject(&gate, scope, key, value, knownCharacters)
		validateCraftQualityObject(&gate, scope, key, value)
	}
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	return makeToolCheckResult("schema", "craft", scope, "", gate), nil
}

func runToolRecapCheck(root, kind, scope, id string) (*toolCheckResult, error) {
	kind = normalizeKey(kind)
	if kind == "" {
		kind = "all"
	}
	switch kind {
	case "quality", "all", "schema":
	default:
		return nil, fmt.Errorf("unsupported recap check kind %q", kind)
	}
	scope = normalizeRecapCheckScope(scope)
	id = strings.TrimSpace(id)
	if scope != "chapter" {
		return nil, fmt.Errorf("recap checks currently support only --scope chapter")
	}
	if id == "" {
		return nil, fmt.Errorf("--id is required for recap chapter check")
	}

	store := recap.NewStore(root)
	recapData, err := store.Load(id)
	gate := quality.QualityGateResult{}
	if err != nil {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   id,
			TargetName: id,
			Issue:      fmt.Sprintf("recap file is not available: %v", err),
			Suggestion: fmt.Sprintf("Generate recap for chapter %s, then rerun tool check quality --target recap --scope chapter --id %q.", id, id),
			Priority:   models.PriorityHigh,
		})
		gate.Blocking = true
		return makeToolCheckResult(kind, "recap", scope, id, gate), nil
	}
	var outlineChapter *models.Chapter
	if outline, err := models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json")); err == nil && outline != nil {
		outlineChapter = outline.GetChapterByID(id)
	}
	return runToolRecapCheckForValue(kind, scope, id, recapData, outlineChapter), nil
}

func runToolRecapCheckForValue(kind, scope, id string, recapData *models.ChapterRecap, outlineChapter *models.Chapter) *toolCheckResult {
	gate := quality.QualityGateResult{}
	validateRecapForToolCheck(&gate, recapData, id, outlineChapter)
	gate.Dedup()
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	return makeToolCheckResult(kind, "recap", scope, id, gate)
}

func runToolChapterCheck(root, kind, scope, id string) (*toolCheckResult, error) {
	return runToolChapterCheckWithTargetWords(root, kind, scope, id, 0)
}

func runToolChapterCheckWithTargetWords(root, kind, scope, id string, targetWordsOverride int) (*toolCheckResult, error) {
	kind = normalizeKey(kind)
	if kind == "" {
		kind = "quality"
	}
	switch kind {
	case "quality", "simulation", "all":
	default:
		return nil, fmt.Errorf("unsupported chapter check kind %q", kind)
	}
	scope = normalizeToolScope(scope)
	if scope == "all" || scope == "" {
		scope = "chapter"
	}
	id = strings.TrimSpace(id)
	if scope != "chapter" {
		return nil, fmt.Errorf("chapter checks currently support only --scope chapter")
	}
	if id == "" {
		return nil, fmt.Errorf("--id is required for chapter check")
	}

	outline, err := models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json"))
	if err != nil {
		return nil, fmt.Errorf("load outline: %w", err)
	}
	chapter := outline.GetChapterByID(id)
	if chapter == nil {
		return nil, fmt.Errorf("chapter %q not found in outline", id)
	}

	content := loadFinalChapterContent(chapter)
	targetWords := targetWordsOverride
	if targetWords <= 0 {
		targetWords = toolChapterTargetWords()
	}
	var gate quality.QualityGateResult
	switch kind {
	case "quality":
		gate = runToolChapterQualityGate(chapter, content, targetWords)
	case "simulation":
		gate = runToolChapterSimulationGate(root, id)
	case "all":
		gate = runToolChapterQualityGate(chapter, content, targetWords)
		sim := runToolChapterSimulationGate(root, id)
		gate.Add(sim.Suggestions...)
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}
	result := makeToolCheckResult(kind, "chapter", scope, id, gate)
	if targetWords > 0 {
		addToolCheckMeta(result, "target_words", targetWords)
	}
	return result, nil
}

func runToolChapterQualityGate(chapter *models.Chapter, content string, targetWords int) (gate quality.QualityGateResult) {
	defer func() {
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}()
	chapterID := ""
	chapterTitle := ""
	if chapter != nil {
		chapterID = strings.TrimSpace(chapter.ID)
		chapterTitle = strings.TrimSpace(chapter.Title)
	}
	targetName := quality.CoalesceString(chapterTitle, chapterID)
	clean := strings.TrimSpace(content)
	if clean == "" {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter markdown is missing or empty", "Run write gen or write improve for this chapter, then rerun tool check quality --target chapter.", models.PriorityHigh))
		return gate
	}

	if chapterTitle != "" && !strings.HasPrefix(clean, "# "+chapterTitle) {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter markdown does not start with the outline title", "Keep the saved chapter headed by the outline title so downstream export and recap tools read the correct chapter.", models.PriorityMedium))
	}

	body := strings.TrimSpace(stripToolChapterMarkdownTitle(clean, chapterTitle))
	if body == "" {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter body is empty after the title", "Regenerate the chapter body before running recap or RPG DSL extraction.", models.PriorityHigh))
		return gate
	}
	if chapterTitle != "" && repeatedChapterTitlePrefix(body, chapterTitle) {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter body repeats the outline title immediately after the heading", "Remove the duplicate title from the first body paragraph; keep one markdown heading and start the body with scene prose.", models.PriorityMedium))
	}
	if strings.HasPrefix(body, "{") && strings.Contains(body, `"content"`) {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter body looks like raw JSON output", "Regenerate or improve the chapter so the saved markdown contains prose, not the agent JSON envelope.", models.PriorityHigh))
	}
	if strings.Contains(body, "```") {
		gate.Add(quality.QualitySuggestion("chapter", chapterID, targetName, "final chapter body contains markdown code fences", "Remove code fences and save only narrative prose in the final chapter markdown.", models.PriorityHigh))
	}
	if strings.Contains(strings.ToLower(body), "<br") {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter body contains HTML line break tags", "Replace HTML <br> tags with normal Markdown paragraph breaks or prose punctuation.", models.PriorityMedium))
	}
	if regexp.MustCompile(`\*\*[^*\n][^*\n]*\*\*`).MatchString(body) {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, "final chapter body contains inline bold markdown", "Remove inline bold formatting from final prose; keep only the chapter heading as Markdown structure.", models.PriorityMedium))
	}
	for _, artifact := range chapterTypoArtifacts(body) {
		gate.Add(quality.QualitySuggestion("prose", chapterID, targetName, fmt.Sprintf("final chapter body contains suspicious typo artifact: %s", artifact), "Fix the suspicious typo while preserving the surrounding prose and chapter events.", models.PriorityMedium))
	}

	count := toolNarrativeUnitCount(body)
	paragraphs := toolChapterParagraphCount(body)
	if count >= 600 && paragraphs < 3 {
		gate.Add(quality.QualitySuggestion("structure", chapterID, targetName, fmt.Sprintf("final chapter has too few prose paragraphs: got %d paragraphs for %d narrative units", paragraphs, count), "Split the chapter into mobile-readable prose and dialogue paragraphs while preserving the same events.", models.PriorityMedium))
	}
	if targetWords > 0 {
		min := targetWords / 2
		if min < 120 {
			min = 120
		}
		if count < min {
			gate.Add(quality.QualitySuggestion("length", chapterID, targetName, fmt.Sprintf("final chapter is too short: got %d narrative units, target %d", count, targetWords), "Regenerate or improve the chapter with enough scene-level prose to satisfy the target length.", models.PriorityHigh))
		}
		hardMax := toolChapterLengthHardMax(targetWords)
		if count > hardMax {
			gate.Add(quality.QualitySuggestion("length", chapterID, targetName, fmt.Sprintf("final chapter is much longer than target: got %d narrative units, target %d", count, targetWords), "Trim the chapter or split non-target material so this saved file contains only the selected chapter.", models.PriorityMedium))
		}
	}

	if chapter != nil && len(chapter.Characters) > 0 {
		missing := []string{}
		for _, name := range firstNStrings(chapter.Characters, 5) {
			name = strings.TrimSpace(name)
			if name != "" && !chapterTextMentionsCharacter(body, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			gate.Add(quality.QualitySuggestion("character", chapterID, targetName, fmt.Sprintf("final chapter text does not mention outline character(s): %s", strings.Join(missing, ", ")), "Revise the saved chapter so the named outline characters are present under their canonical names, or update the outline only through the validated outline patch workflow if the cast changed.", models.PriorityHigh))
		}
	}

	flavor := style.CheckAIFlavor(body, 75)
	if flavor.HasIssue {
		gate.Add(quality.QualitySuggestion("style", chapterID, targetName, fmt.Sprintf("deterministic AI-flavor check score is %d", flavor.Score), strings.Join(flavor.Suggestions, " "), models.PriorityMedium))
	}
	return gate
}

func chapterTypoArtifacts(body string) []string {
	checks := []string{
		"十好几个",
	}
	found := []string{}
	for _, check := range checks {
		if strings.Contains(body, check) {
			found = append(found, check)
		}
	}
	return found
}

func chapterTextMentionsCharacter(body, name string) bool {
	body = strings.TrimSpace(body)
	for _, candidate := range chapterCharacterMentionCandidates(name) {
		if candidate != "" && strings.Contains(body, candidate) {
			return true
		}
	}
	return false
}

func chapterCharacterMentionCandidates(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	candidates := []string{name}
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("()（）[]【】,，:：/、|", r)
	})
	if len(fields) > 0 {
		candidates = append(candidates, fields[len(fields)-1])
	}
	runes := []rune(name)
	if containsCJKRune(name) && len(runes) >= 4 {
		candidates = append(candidates, string(runes[len(runes)-2:]))
	}
	if containsCJKRune(name) && len(runes) >= 5 {
		candidates = append(candidates, string(runes[len(runes)-3:]))
	}
	return dedupeNonEmptyStrings(candidates)
}

func containsCJKRune(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func stripToolChapterMarkdownTitle(content, title string) string {
	return project.StripChapterTitle(content, title)
}

func hasToolChapterMarkdownTitle(content, title string) bool {
	return project.HasChapterTitle(content, title)
}

func repeatedChapterTitlePrefix(content, title string) bool {
	return project.HasRepeatedChapterTitlePrefix(content, title)
}

func stripRepeatedChapterTitlePrefix(content, title string) string {
	return project.StripRepeatedChapterTitlePrefix(content, title)
}

func matchingChapterTitlePrefixLen(content, title string) int {
	return project.MatchingChapterTitlePrefixLen(content, title)
}

func normalizedChapterTitleText(value string) string {
	return project.NormalizeChapterTitle(value)
}

func toolChapterTargetWords() int {
	cfg, err := loadProjectConfig()
	if err == nil && cfg.ChapterConfig.TargetWordsPerChapter > 0 {
		return cfg.ChapterConfig.TargetWordsPerChapter
	}
	return 2000
}

func toolNarrativeUnitCount(content string) int {
	return project.NarrativeUnitCount(content)
}

func toolChapterParagraphCount(content string) int {
	count := 0
	for _, part := range regexp.MustCompile(`\r?\n\s*\r?\n`).Split(strings.TrimSpace(content), -1) {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasPrefix(part, "#") || strings.HasPrefix(part, "<!--") {
			continue
		}
		count++
	}
	return count
}

func runToolChapterSimulationGate(root, chapterID string) quality.QualityGateResult {
	var gate quality.QualityGateResult
	if stale, info := toolChapterRPGDSLStale(root, chapterID); stale {
		gate.Add(quality.QualitySuggestion(
			"simulation",
			chapterID,
			chapterID,
			"chapter RPG DSL is stale relative to the saved final chapter markdown",
			fmt.Sprintf("Refresh chapter RPG DSL before trusting simulation results: novelgen tool refresh chapter-dsl --id %q. Chapter markdown %s is newer than %s. Simulation checks do not invoke LLM conversion by themselves.", chapterID, info.ChapterPath, info.DSLPath),
			models.PriorityHigh,
		))
		gate.Blocking = true
		return gate
	}
	dslData, err := loadToolMergedRPGDSL(root)
	if err != nil {
		gate.Add(quality.QualitySuggestion(
			"simulation",
			chapterID,
			chapterID,
			fmt.Sprintf("chapter RPG DSL simulation is unavailable: %v", err),
			"Refresh chapter RPG DSL first with `novelgen tool refresh chapter-dsl --id "+chapterID+"`, then rerun this simulation check. Simulation checks do not invoke LLM conversion by themselves.",
			models.PriorityHigh,
		))
		gate.Blocking = true
		return gate
	}
	simulator := rpgdsl.NewSimulator(dslData)
	issues := simulator.SimulateChapter(chapterID)
	gate.Add(rpgdsl.NewSimulationBridge().ConvertIssuesToSuggestions(issues)...)
	if diagnostic := buildChapterSimulationSignalDiagnostic(chapterID, dslData, issues); diagnostic != nil {
		gate.Add(*diagnostic)
	}
	gate.Dedup()
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	return gate
}

type chapterSimulationSignalDiagnostics struct {
	CombatSteps          int
	EnemyRefs            []string
	HasCombatResult      bool
	HasPowerChange       bool
	HasBreakthrough      bool
	HasGene              bool
	HasMech              bool
	HasEquipmentOrItem   bool
	HasAlly              bool
	HasTacticalText      bool
	MissingRepairSignals []string
}

func buildChapterSimulationSignalDiagnostic(chapterID string, dslData *rpgdsl.DSL, issues []rpgdsl.SimulationIssue) *models.ReviewSuggestion {
	if !chapterSimulationNeedsSignalDiagnostic(chapterID, issues) {
		return nil
	}
	diagnostic, ok := collectChapterSimulationSignalDiagnostics(dslData, chapterID)
	if !ok || diagnostic.CombatSteps == 0 {
		return nil
	}
	issue := fmt.Sprintf("simulation signal diagnostics: combat_steps=%d; enemies=%s; combat_result=%t; power_change=%t; breakthrough=%t; gene=%t; mech=%t; equipment_or_item=%t; ally=%t; tactical_text=%t",
		diagnostic.CombatSteps,
		strings.Join(firstNStrings(diagnostic.EnemyRefs, 8), ", "),
		diagnostic.HasCombatResult,
		diagnostic.HasPowerChange,
		diagnostic.HasBreakthrough,
		diagnostic.HasGene,
		diagnostic.HasMech,
		diagnostic.HasEquipmentOrItem,
		diagnostic.HasAlly,
		diagnostic.HasTacticalText,
	)
	if len(diagnostic.MissingRepairSignals) > 0 {
		issue += "; missing_repair_signals=" + strings.Join(diagnostic.MissingRepairSignals, ", ")
	}
	suggestion := "Make the next prose patch produce explicit DSL-readable repair signals. For combat balance, add supported mech/gene/equipment/item/ally/power_change/breakthrough signals or reduce enemy count/level. For missing combat result, add an on_complete narration/result and durable state_delta consequence. Then run `novelgen tool refresh chapter-dsl --id \"" + chapterID + "\"` and rerun chapter simulation check."
	return &models.ReviewSuggestion{
		Category:   models.CategoryLogic,
		TargetID:   chapterID,
		TargetName: chapterID,
		Issue:      issue,
		Suggestion: suggestion,
		Priority:   models.PriorityLow,
	}
}

func chapterSimulationNeedsSignalDiagnostic(chapterID string, issues []rpgdsl.SimulationIssue) bool {
	for _, issue := range issues {
		if strings.TrimSpace(issue.Chapter) != "" && issue.Chapter != chapterID {
			continue
		}
		if issue.Type == rpgdsl.IssueBalance || issue.Type == rpgdsl.IssueDescription {
			return true
		}
		text := issue.Description + " " + issue.Suggestion
		if strings.Contains(text, "战斗难度") || strings.Contains(text, "叙事性结果") || strings.Contains(strings.ToLower(text), "combat") {
			return true
		}
	}
	return false
}

func collectChapterSimulationSignalDiagnostics(dslData *rpgdsl.DSL, chapterID string) (chapterSimulationSignalDiagnostics, bool) {
	var out chapterSimulationSignalDiagnostics
	chapter := findDSLChapterByID(dslData, chapterID)
	if chapter == nil {
		return out, false
	}
	enemyRefs := map[string]bool{}
	for _, objective := range chapter.Objectives {
		for _, step := range objective.Steps {
			event := step.Event
			if strings.EqualFold(event.Type, "combat") || event.Combat != nil {
				out.CombatSteps++
				if event.Combat != nil {
					for _, enemy := range event.Combat.Setup.Enemies {
						ref := strings.TrimSpace(enemy.ID)
						if ref == "" {
							ref = "enemy_unknown"
						}
						ref = fmt.Sprintf("%s(count=%d,level=%d,elite=%t,boss=%t)", ref, enemy.Count, enemy.Level, enemy.Elite, enemy.Boss)
						if !enemyRefs[ref] {
							enemyRefs[ref] = true
							out.EnemyRefs = append(out.EnemyRefs, ref)
						}
					}
				}
			}
			if eventResultHasNarration(event.OnComplete) ||
				(event.Combat != nil && (eventResultHasNarration(event.Combat.OnVictory) || eventResultHasNarration(event.Combat.OnDefeat))) {
				out.HasCombatResult = true
			}
			if chapterStepHasTacticalText(step) {
				out.HasTacticalText = true
			}
			for _, delta := range event.StateDeltas {
				switch strings.ToLower(strings.TrimSpace(delta.Kind)) {
				case "power_change":
					out.HasPowerChange = true
				case "breakthrough", "evolution", "cultivation":
					out.HasBreakthrough = true
				case "gene":
					out.HasGene = true
				case "mech":
					out.HasMech = true
				case "item", "equipment", "resource":
					if strings.Contains(strings.ToLower(delta.Field+" "+delta.Target+" "+delta.To), "item") ||
						strings.TrimSpace(delta.Target) != "" ||
						strings.TrimSpace(delta.To) != "" {
						out.HasEquipmentOrItem = true
					}
				case "ally":
					out.HasAlly = true
				}
			}
		}
	}
	sort.Strings(out.EnemyRefs)
	out.MissingRepairSignals = missingChapterSimulationRepairSignals(out)
	return out, true
}

func findDSLChapterByID(dslData *rpgdsl.DSL, chapterID string) *rpgdsl.Chapter {
	if dslData == nil || dslData.Storyline == nil {
		return nil
	}
	for i := range dslData.Storyline.Chapters {
		if dslData.Storyline.Chapters[i].ID == chapterID {
			return &dslData.Storyline.Chapters[i]
		}
	}
	return nil
}

func eventResultHasNarration(result *rpgdsl.EventResult) bool {
	return result != nil && (strings.TrimSpace(result.Narration) != "" || strings.TrimSpace(result.Result) != "")
}

func chapterStepHasTacticalText(step rpgdsl.Step) bool {
	text := strings.ToLower(step.Description)
	event := step.Event
	if event.Combat != nil {
		text += " " + strings.ToLower(event.Combat.Setup.Location)
		for key, value := range event.Combat.Setup.Environment {
			text += " " + strings.ToLower(key) + " " + strings.ToLower(fmt.Sprint(value))
		}
		for _, phase := range event.Combat.Phases {
			text += " " + strings.ToLower(phase.Name+" "+phase.Trigger+" "+phase.Duration+" "+phase.Narration)
			for key, value := range phase.Modifiers {
				text += " " + strings.ToLower(key) + " " + strings.ToLower(fmt.Sprint(value))
			}
		}
	}
	for _, delta := range event.StateDeltas {
		text += " " + strings.ToLower(delta.Target+" "+delta.Kind+" "+delta.Field+" "+delta.From+" "+delta.To+" "+delta.Cost+" "+delta.Note)
	}
	for _, term := range []string{
		"机甲", "火种", "装甲", "伏击", "偷袭", "陷阱", "地形", "高地", "狭道", "三方", "混战", "互相消耗", "两败俱伤", "第三方",
		"mech", "armor", "ambush", "trap", "terrain", "high ground", "narrow", "chokepoint", "third party", "three-way", "attrition",
	} {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func missingChapterSimulationRepairSignals(d chapterSimulationSignalDiagnostics) []string {
	missing := make([]string, 0, 6)
	if !d.HasCombatResult {
		missing = append(missing, "combat_result_on_complete")
	}
	if !d.HasPowerChange {
		missing = append(missing, "power_change")
	}
	if !d.HasMech {
		missing = append(missing, "mech")
	}
	if !d.HasGene {
		missing = append(missing, "gene")
	}
	if !d.HasEquipmentOrItem {
		missing = append(missing, "equipment_or_item")
	}
	if !d.HasAlly {
		missing = append(missing, "ally")
	}
	if !d.HasTacticalText {
		missing = append(missing, "tactical_text")
	}
	return missing
}

type toolChapterRPGDSLStaleInfo struct {
	ChapterPath string
	DSLPath     string
}

func toolChapterRPGDSLStale(root, chapterID string) (bool, toolChapterRPGDSLStaleInfo) {
	info := toolChapterRPGDSLStaleInfo{
		ChapterPath: toolFinalChapterPathForID(root, chapterID),
		DSLPath:     filepath.Join(root, "story", "rpg", "04_chapters.rpg"),
	}
	chapterStat, err := os.Stat(info.ChapterPath)
	if err != nil {
		fallback := filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", extractChapterNumber(chapterID)))
		chapterStat, err = os.Stat(fallback)
		if err != nil {
			return false, info
		}
		info.ChapterPath = fallback
	}
	dslStat, err := os.Stat(info.DSLPath)
	if err != nil {
		return false, info
	}
	return chapterStat.ModTime().After(dslStat.ModTime()), info
}

func toolFinalChapterPathForID(root, chapterID string) string {
	return filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", chapterID))
}

func loadToolMergedRPGDSL(root string) (*rpgdsl.DSL, error) {
	fragments, err := loadToolRPGDSLFragments(root)
	if err != nil {
		return nil, err
	}
	if len(fragments) == 0 {
		return nil, fmt.Errorf("no RPG DSL files found under story/rpg")
	}
	if len(fragments) == 1 {
		return fragments[0].DSL, nil
	}
	merger := rpgdsl.NewDSLMerger(rpgdsl.NewConsoleLogger(rpgdsl.WithMinLevel(rpgdsl.LogLevelFatal)))
	for _, fragment := range fragments {
		merger.AddFragment(fragment.DSL, fragment.Phase, fragment.FilePath)
	}
	result, err := merger.Merge()
	if err != nil {
		return nil, err
	}
	return result.DSL, nil
}

func loadToolRPGDSLFragments(root string) ([]*rpgdsl.DSLFragment, error) {
	rpgDir := filepath.Join(root, "story", "rpg")
	patterns := []string{
		"00_setup.rpg",
		"01_outline.rpg",
		"02_craft.rpg",
		"03_systems.rpg",
		"04_chapters.rpg",
		"*.rpg",
	}
	var fragments []*rpgdsl.DSLFragment
	seen := map[string]bool{}
	var parseErrors []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(rpgDir, pattern))
		if err != nil {
			continue
		}
		for _, file := range matches {
			if seen[file] {
				continue
			}
			seen[file] = true
			content, err := os.ReadFile(file)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("read %s failed: %v", file, err))
				continue
			}
			parsed, err := rpgdsl.NewParser(string(content)).Parse()
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("parse %s failed: %v", file, err))
				continue
			}
			fragments = append(fragments, &rpgdsl.DSLFragment{
				DSL:      parsed,
				Phase:    inferPhaseFromFilename(filepath.Base(file)),
				FilePath: file,
			})
		}
	}
	if len(fragments) == 0 && len(parseErrors) > 0 {
		return nil, fmt.Errorf(strings.Join(parseErrors, "; "))
	}
	return fragments, nil
}

func normalizeRecapCheckScope(scope string) string {
	switch normalizeKey(scope) {
	case "", "chapter", "chap":
		return "chapter"
	default:
		return normalizeKey(scope)
	}
}

func validateRecapForToolCheck(gate *quality.QualityGateResult, recapData *models.ChapterRecap, expectedID string, outlineChapter *models.Chapter) {
	if gate == nil {
		return
	}
	if recapData == nil {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   expectedID,
			TargetName: expectedID,
			Issue:      "recap is empty",
			Suggestion: "Regenerate the recap from the final chapter text.",
			Priority:   models.PriorityHigh,
		})
		return
	}
	if strings.TrimSpace(recapData.ChapterID) != "" && strings.TrimSpace(expectedID) != "" && recapData.ChapterID != expectedID {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   expectedID,
			TargetName: expectedID,
			Issue:      fmt.Sprintf("recap chapter_id %q does not match requested chapter %q", recapData.ChapterID, expectedID),
			Suggestion: "Regenerate or patch the recap so chapter_id matches the checked chapter.",
			Priority:   models.PriorityHigh,
		})
	}
	if outlineChapter != nil && strings.TrimSpace(outlineChapter.Title) != "" && strings.TrimSpace(recapData.Title) != "" && strings.TrimSpace(recapData.Title) != strings.TrimSpace(outlineChapter.Title) {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   expectedID,
			TargetName: recapData.Title,
			Issue:      fmt.Sprintf("recap title %q does not match outline title %q", recapData.Title, outlineChapter.Title),
			Suggestion: "Regenerate the recap from the current final chapter and outline title so recap.title matches the chapter being checked.",
			Priority:   models.PriorityMedium,
		})
	}
	if ok, reasons := recap.ValidateMinimal(recapData); !ok {
		for _, reason := range reasons {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "recap",
				TargetID:   expectedID,
				TargetName: recapData.Title,
				Issue:      reason,
				Suggestion: "Regenerate the recap from final chapter text and preserve location, present characters, last_line, and next_opening_hint.",
				Priority:   models.PriorityHigh,
			})
		}
	}
	if ok, reasons := recap.ValidateConsistency(recapData); !ok {
		for _, reason := range reasons {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "recap",
				TargetID:   expectedID,
				TargetName: recapData.Title,
				Issue:      reason,
				Suggestion: "Regenerate recap with a concise next_opening_hint that visibly continues from last_line.",
				Priority:   models.PriorityMedium,
			})
		}
	}
}

func normalizeRecapPatchIdentity(value *models.ChapterRecap, id string, outlineChapter *models.Chapter, patchBytes []byte) []string {
	if value == nil {
		return nil
	}
	var patch map[string]interface{}
	_ = json.Unmarshal(patchBytes, &patch)
	warnings := []string{}
	if raw, ok := patch["chapter_id"]; ok && strings.TrimSpace(fmt.Sprint(raw)) != "" && strings.TrimSpace(fmt.Sprint(raw)) != id {
		warnings = append(warnings, fmt.Sprintf("ignored patch chapter_id %q; recap identity is fixed to %q", fmt.Sprint(raw), id))
	}
	value.ChapterID = id
	if outlineChapter != nil && strings.TrimSpace(outlineChapter.Title) != "" {
		if raw, ok := patch["title"]; ok && strings.TrimSpace(fmt.Sprint(raw)) != "" && strings.TrimSpace(fmt.Sprint(raw)) != strings.TrimSpace(outlineChapter.Title) {
			warnings = append(warnings, fmt.Sprintf("ignored patch title %q; recap title is fixed to outline title %q", fmt.Sprint(raw), outlineChapter.Title))
		}
		value.Title = outlineChapter.Title
	}
	return warnings
}

func validateCraftSchemaObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}) {
	if gate == nil {
		return
	}
	if _, err := normalizeCraftPatchObject(scope, key, copyStringInterfaceMap(value)); err != nil {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "schema",
			TargetID:   key,
			TargetName: key,
			Issue:      fmt.Sprintf("craft %s does not match schema: %v", scope, err),
			Suggestion: fmt.Sprintf("Patch craft %s %q with fields that match the typed craft schema, then rerun tool check schema.", scope, key),
			Priority:   models.PriorityHigh,
		})
	}
}

func loadCraftCharacterNames(root string) map[string]bool {
	names := map[string]bool{}
	values, err := loadRawCraftMap(root, "characters.json")
	if err != nil {
		return names
	}
	for key, value := range values {
		addCraftKnownName(names, key)
		addCraftKnownName(names, stringMapValue(value, "name"))
	}
	return names
}

func validateCraftConsistencyObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}, knownCharacters map[string]bool) {
	if gate == nil || normalizeCraftPatchTarget(scope) != "item" || len(knownCharacters) == 0 {
		return
	}
	owner := strings.TrimSpace(stringMapValue(value, "owner"))
	if owner == "" || isGenericCraftOwner(owner) || knownCharacters[normalizeCraftLookupName(owner)] {
		return
	}
	gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
		Category:   "consistency",
		TargetID:   key,
		TargetName: key,
		Issue:      fmt.Sprintf("item owner %q is not present in craft characters", owner),
		Suggestion: "Patch the item owner to an existing character name, a stable generic owner such as 主角, or leave owner empty when ownership is not established.",
		Priority:   models.PriorityMedium,
	})
}

func validateCraftQualityObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}) {
	if gate == nil || normalizeCraftPatchTarget(scope) != "character" || value == nil {
		return
	}
	isLead := isLeadCraftCharacter(value)
	personalityCount := craftStringListCount(value["personality"])
	skillsCount := craftStringListCount(value["skills"])
	abilitiesCount := craftStringListCount(value["abilities"])
	motivation := stringMapValue(value, "motivation")
	voice := stringMapValue(value, "voice")
	notes := stringMapValue(value, "notes")
	notesLen := len([]rune(notes))

	if isLead {
		if personalityCount < 3 {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft has too few structured personality traits",
				Suggestion: "Patch personality with 3-6 concrete traits that affect decisions, dialogue, and failure modes.",
				Priority:   models.PriorityMedium,
			})
		}
		if motivation == "" {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft is missing a structured motivation",
				Suggestion: "Patch motivation with the character's durable desire, fear, and story-facing pressure in one concise paragraph.",
				Priority:   models.PriorityMedium,
			})
		}
		if skillsCount+abilitiesCount == 0 {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft has no structured skills or abilities",
				Suggestion: "Patch skills for mundane/tactical competencies and abilities for special powers, including limits or blind spots.",
				Priority:   models.PriorityMedium,
			})
		}
		if voice == "" {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft is missing voice guidance",
				Suggestion: "Patch voice with speech rhythm, humor/seriousness balance, and recurring verbal habits useful for drafting.",
				Priority:   models.PriorityLow,
			})
		}
	}
	if notesLen > 1200 && isLead {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "craft_quality",
			TargetID:   key,
			TargetName: key,
			Issue:      "protagonist craft concentrates too much writable structure in notes",
			Suggestion: "Move reusable facts from notes into personality, motivation, skills, abilities, and voice; keep notes for concise writer-only constraints.",
			Priority:   models.PriorityLow,
		})
	} else if notesLen > 1800 {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "craft_quality",
			TargetID:   key,
			TargetName: key,
			Issue:      "craft notes are too long for stable focused prompting",
			Suggestion: "Shorten notes and move repeated structured facts into typed craft fields.",
			Priority:   models.PriorityLow,
		})
	}
}

func isLeadCraftCharacter(value map[string]interface{}) bool {
	role := strings.ToLower(stringMapValue(value, "role_in_story"))
	rpgRole := strings.ToLower(stringMapValue(value, "rpg_role"))
	if strings.Contains(role, "protagonist") || strings.Contains(role, "lead") || strings.Contains(role, "主角") || rpgRole == "player" {
		return true
	}
	for _, tag := range craftStringListValues(value["dsl_tags"]) {
		tag = strings.ToLower(tag)
		if tag == "protagonist" || tag == "player" || tag == "lead" {
			return true
		}
	}
	return false
}

func craftStringListCount(raw interface{}) int {
	return len(craftStringListValues(raw))
}

func craftStringListValues(raw interface{}) []string {
	switch values := raw.(type) {
	case []string:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				out = append(out, strings.TrimSpace(value))
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func addCraftKnownName(names map[string]bool, name string) {
	name = normalizeCraftLookupName(name)
	if name != "" {
		names[name] = true
	}
}

func stringMapValue(value map[string]interface{}, key string) string {
	if value == nil {
		return ""
	}
	raw, ok := value[key]
	if !ok || raw == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func normalizeCraftLookupName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isGenericCraftOwner(owner string) bool {
	switch normalizeCraftLookupName(owner) {
	case "主角", "protagonist", "unknown", "未知", "无人", "none", "n/a":
		return true
	default:
		return false
	}
}

func filterToolScopedSuggestions(suggestions []models.ReviewSuggestion, outline *models.Outline, scope, id string) []models.ReviewSuggestion {
	if len(suggestions) == 0 {
		return nil
	}
	allowed := toolScopedTargetIDs(outline, scope, id)
	var filtered []models.ReviewSuggestion
	for _, suggestion := range suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if targetID == "" {
			continue
		}
		if allowed[targetID] || allowed[strings.ToLower(targetID)] {
			filtered = append(filtered, suggestion)
			continue
		}
		if scope == "volume" && strings.HasPrefix(targetID, id+"-C") {
			filtered = append(filtered, suggestion)
		}
	}
	return filtered
}

func toolScopedTargetIDs(outline *models.Outline, scope, id string) map[string]bool {
	allowed := map[string]bool{
		id:                  true,
		strings.ToLower(id): true,
	}
	if outline == nil {
		return allowed
	}
	switch scope {
	case "volume":
		volume := outline.GetVolumeByID(id)
		if volume == nil && len(outline.Parts) == 1 && len(outline.Parts[0].Volumes) == 1 {
			volume = &outline.Parts[0].Volumes[0]
		}
		if volume != nil {
			for _, chapter := range volume.Chapters {
				if strings.TrimSpace(chapter.ID) != "" {
					allowed[chapter.ID] = true
					allowed[strings.ToLower(chapter.ID)] = true
				}
			}
		}
		allowed["payoff_contract"] = true
	case "chapter":
		allowed["chapter"] = true
	}
	return allowed
}

func makeToolCheckResult(kind, target, scope, id string, gate quality.QualityGateResult) *toolCheckResult {
	critical, high, medium, low := quality.CountSuggestionPriorities(gate.Suggestions)
	return &toolCheckResult{
		Kind:     kind,
		Target:   target,
		Scope:    scope,
		ID:       id,
		OK:       !gate.Blocking,
		Blocking: gate.Blocking,
		Score:    quality.ScoreFromGate(gate),
		Summary: toolCheckSummary{
			Total:    len(gate.Suggestions),
			Critical: critical,
			High:     high,
			Medium:   medium,
			Low:      low,
		},
		Issues: gate.Suggestions,
		Meta:   toolCheckCoverageMeta(kind, target, scope),
	}
}

func toolCheckCoverageMeta(kind, target, scope string) map[string]interface{} {
	kind = normalizeKey(kind)
	target = normalizeKey(target)
	scope = normalizeToolScope(scope)
	meta := map[string]interface{}{
		"coverage": map[string]interface{}{
			"kind":   kind,
			"target": target,
			"scope":  scope,
		},
	}
	coverage := meta["coverage"].(map[string]interface{})
	switch kind {
	case "all":
		coverage["engines"] = []string{"quality", "simulation"}
	case "quality", "schema":
		coverage["engines"] = []string{kind}
	case "simulation":
		coverage["engines"] = []string{"simulation"}
	default:
		coverage["engines"] = []string{kind}
	}
	if target == "outline" && (kind == "all" || kind == "simulation") {
		coverage["simulation_backend"] = "in_memory_model_adapter"
		coverage["simulation_phase"] = "outline"
		coverage["invokes_llm"] = false
		coverage["uses_derived_rpg_files"] = false
		coverage["refresh_required_before_simulation"] = false
	}
	if target == "setup" && (kind == "all" || kind == "simulation") {
		coverage["simulation_backend"] = "in_memory_model_adapter"
		coverage["simulation_phase"] = "setup"
		coverage["invokes_llm"] = false
		coverage["uses_derived_rpg_files"] = false
		coverage["refresh_required_before_simulation"] = false
	}
	if target == "chapter" && (kind == "all" || kind == "simulation") {
		coverage["simulation_backend"] = "chapter_rpg_dsl"
		coverage["simulation_phase"] = "chapter"
		coverage["invokes_llm"] = false
		coverage["uses_derived_rpg_files"] = true
		coverage["refresh_required_before_simulation"] = true
	}
	return meta
}

func addToolCheckMeta(result *toolCheckResult, key string, value interface{}) {
	if result == nil || strings.TrimSpace(key) == "" {
		return
	}
	if result.Meta == nil {
		result.Meta = map[string]interface{}{}
	}
	result.Meta[key] = value
}

func normalizeToolScope(scope string) string {
	switch normalizeKey(scope) {
	case "", "all", "outline":
		return "all"
	case "vol", "volume":
		return "volume"
	case "chap", "chapter":
		return "chapter"
	default:
		return normalizeKey(scope)
	}
}

func scopedOutline(outline *models.Outline, scope, id string) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	if scope == "all" {
		return cloneOutline(outline), nil
	}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if scope == "volume" && volume.ID == id {
				partCopy := part
				volumeCopy := volume
				partCopy.Volumes = []models.Volume{volumeCopy}
				return &models.Outline{Parts: []models.Part{partCopy}}, nil
			}
			if scope == "chapter" {
				for _, chapter := range volume.Chapters {
					if chapter.ID == id {
						partCopy := part
						volumeCopy := volume
						partCopy.Volumes = []models.Volume{volumeCopy}
						return &models.Outline{Parts: []models.Part{partCopy}}, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("%s %q not found in outline", scope, id)
}

type outlineVolumePatch struct {
	ID              string                       `json:"id,omitempty"`
	Title           string                       `json:"title,omitempty"`
	Summary         string                       `json:"summary,omitempty"`
	PayoffContract  *models.VolumePayoffContract `json:"payoff_contract,omitempty"`
	ChangedChapters []json.RawMessage            `json:"changed_chapters,omitempty"`
	ChangedEvents   []json.RawMessage            `json:"changed_events,omitempty"`
}
