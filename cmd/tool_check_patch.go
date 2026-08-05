package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/logic/style"
	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"
	"novelgen/internal/utils"

	"github.com/spf13/cobra"
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

func runToolPatch(cmd *cobra.Command, args []string) error {
	section := normalizeKey(args[0])
	if section != "outline" && section != "craft" && section != "setup" && section != "recap" && section != "chapter" {
		return fmt.Errorf("unsupported patch section %q", section)
	}
	target := normalizeKey(toolPatchFlags.Target)
	if section == "outline" && target == "" {
		target = "chapter"
	}
	if section == "outline" && target != "chapter" && target != "volume" {
		return fmt.Errorf("unsupported outline patch target %q", target)
	}
	if section == "craft" {
		target = normalizeCraftPatchTarget(target)
		if target == "" {
			return fmt.Errorf("--target is required for craft patch (character/item/location/organization)")
		}
	}
	id := strings.TrimSpace(toolPatchFlags.ID)
	taskID := strings.TrimSpace(toolPatchFlags.Task)
	idRepaired := false
	if section != "setup" {
		repairedID := utils.RepairLikelyMojibakeText(id)
		idRepaired = repairedID != id
		id = repairedID
	}
	if id == "" && section != "setup" && taskID == "" {
		return fmt.Errorf("--id is required")
	}
	warnings := []string{}
	if idRepaired {
		warnings = append(warnings, fmt.Sprintf("repaired possible mojibake in --id from %q to %q", toolPatchFlags.ID, id))
	}
	apply := toolPatchFlags.Apply && !toolPatchFlags.DryRun

	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	var patchBytes []byte
	if taskID != "" {
		resolvedTarget, resolvedID, resolvedPatchBytes, taskWarnings, err := resolveToolPatchTask(root, section, target, id, taskID)
		if err != nil {
			return err
		}
		target = resolvedTarget
		id = resolvedID
		patchBytes = resolvedPatchBytes
		warnings = append(warnings, taskWarnings...)
	} else if strings.TrimSpace(toolPatchFlags.PatchBuffer) != "" {
		patchBytes, err = loadToolPatchBuffer(root, toolPatchFlags.PatchBuffer, section)
		if err != nil {
			return err
		}
	} else if strings.TrimSpace(toolPatchFlags.PatchJSON) != "" {
		patchBytes = []byte(toolPatchFlags.PatchJSON)
	} else {
		patchBytes, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("failed to read patch stdin: %w", err)
		}
	}
	patchBytes = utils.StripUTF8BOM(patchBytes)
	if len(strings.TrimSpace(string(patchBytes))) == 0 {
		return fmt.Errorf("patch JSON is required on stdin")
	}
	if err := validateToolPatchBytes(patchBytes); err != nil {
		return fmt.Errorf("patch rejected: %w", err)
	}

	switch section {
	case "outline":
		return runToolOutlinePatch(cmd, root, target, id, patchBytes, apply, warnings)
	case "craft":
		return runToolCraftPatch(cmd, root, target, id, patchBytes, apply, warnings)
	case "setup":
		return runToolSetupPatch(cmd, root, patchBytes, apply, warnings)
	case "recap":
		return runToolRecapPatch(cmd, root, id, patchBytes, apply, warnings)
	case "chapter":
		return runToolChapterPatch(cmd, root, id, patchBytes, apply, warnings)
	default:
		return fmt.Errorf("unsupported patch section %q", section)
	}
}

func resolveToolPatchTask(root, section, target, id, taskID string) (string, string, []byte, []string, error) {
	if section != "outline" {
		return "", "", nil, nil, fmt.Errorf("--task is currently supported only for outline patches")
	}
	const prefix = "outline-global-repair:"
	if !strings.HasPrefix(normalizeKey(taskID), prefix) {
		return "", "", nil, nil, fmt.Errorf("unsupported patch task %q", taskID)
	}
	name := strings.TrimPrefix(normalizeKey(taskID), prefix)
	if name == "" {
		return "", "", nil, nil, fmt.Errorf("patch task category is required")
	}
	ctx := toolProjectContext{Root: root}
	if err := ctx.loadSetup(); err != nil {
		return "", "", nil, nil, err
	}
	if err := ctx.loadOutline(); err != nil {
		return "", "", nil, nil, err
	}
	resp := queryOutlineGlobalRepairContext(ctx, name)
	if !resp.OK {
		return "", "", nil, nil, fmt.Errorf("failed to resolve patch task %q", taskID)
	}
	bundle, ok := resp.Results.(outlineGlobalRepairContext)
	if !ok || bundle.PatchTask == nil {
		return "", "", nil, nil, fmt.Errorf("patch task %q has no patchable target", taskID)
	}
	task := bundle.PatchTask
	resolvedTarget := extractNovelgenCommandFlag(task.PatchQuery, "--target")
	resolvedID := extractNovelgenCommandFlag(task.PatchQuery, "--id")
	if resolvedTarget == "" || resolvedID == "" {
		return "", "", nil, nil, fmt.Errorf("patch task %q has incomplete patch query %q", taskID, task.PatchQuery)
	}
	if target != "" && target != "chapter" && normalizeKey(target) != normalizeKey(resolvedTarget) {
		return "", "", nil, nil, fmt.Errorf("--target %q does not match patch task target %q", target, resolvedTarget)
	}
	if id != "" && id != resolvedID {
		return "", "", nil, nil, fmt.Errorf("--id %q does not match patch task id %q", id, resolvedID)
	}
	patchBytes, err := json.Marshal(task.PatchShape)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to encode patch task %q: %w", taskID, err)
	}
	return normalizeKey(resolvedTarget), resolvedID, patchBytes, []string{fmt.Sprintf("using patch task %s", taskID)}, nil
}

func extractNovelgenCommandFlag(command, flag string) string {
	fields := strings.Fields(command)
	for i := 0; i < len(fields); i++ {
		if fields[i] == flag && i+1 < len(fields) {
			return strings.Trim(fields[i+1], `"'`)
		}
		prefix := flag + "="
		if strings.HasPrefix(fields[i], prefix) {
			return strings.Trim(strings.TrimPrefix(fields[i], prefix), `"'`)
		}
	}
	return ""
}

func runToolPatchBuffer(cmd *cobra.Command, args []string) error {
	op := normalizeKey(args[0])
	if !allowedToolPatchBufferOps[op] {
		return fmt.Errorf("unsupported patch-buffer operation %q", op)
	}
	id := strings.TrimSpace(toolPatchBufferFlags.ID)
	if id == "" {
		return fmt.Errorf("--id is required")
	}
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	path, err := toolPatchBufferPath(root, id)
	if err != nil {
		return err
	}
	switch op {
	case "clear":
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clear patch buffer: %w", err)
		}
		return writeJSON(cmd, map[string]interface{}{"ok": true, "id": id, "path": path, "bytes": 0})
	case "append":
		text := toolPatchBufferFlags.Text
		if toolPatchBufferFlags.Stdin && text != "" {
			return fmt.Errorf("use either --text or --stdin, not both")
		}
		if text == "" || toolPatchBufferFlags.Stdin {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read patch buffer stdin: %w", err)
			}
			text = string(utils.StripUTF8BOM(data))
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("buffer text is required via --text or stdin")
		}
		if err := utils.ValidateNoSuspiciousPatchText(map[string]interface{}{"text": text}); err != nil {
			return fmt.Errorf("patch buffer rejected: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create patch buffer directory: %w", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open patch buffer: %w", err)
		}
		if _, err := f.WriteString(text); err != nil {
			_ = f.Close()
			return fmt.Errorf("failed to append patch buffer: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close patch buffer: %w", err)
		}
		info, _ := os.Stat(path)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		return writeJSON(cmd, map[string]interface{}{"ok": true, "id": id, "path": path, "bytes": size})
	case "show":
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read patch buffer: %w", err)
		}
		maxChars := toolPatchBufferFlags.MaxChars
		if maxChars <= 0 {
			maxChars = 400
		}
		if maxChars > 2000 {
			maxChars = 2000
		}
		runes := []rune(string(data))
		preview := string(runes)
		truncated := false
		if len(runes) > maxChars {
			preview = string(runes[:maxChars])
			truncated = true
		}
		return writeJSON(cmd, map[string]interface{}{
			"ok":        true,
			"id":        id,
			"path":      path,
			"bytes":     len(data),
			"chars":     len(runes),
			"preview":   preview,
			"truncated": truncated,
		})
	}
	return fmt.Errorf("unsupported patch-buffer operation %q", op)
}

func loadToolPatchBuffer(root, id, section string) ([]byte, error) {
	path, err := toolPatchBufferPath(root, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read patch buffer %q: %w", id, err)
	}
	content := string(utils.StripUTF8BOM(data))
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("patch buffer %q is empty", id)
	}
	if normalizeKey(section) == "chapter" {
		payload, err := json.Marshal(map[string]string{"content": content})
		if err != nil {
			return nil, err
		}
		return payload, nil
	}
	return []byte(content), nil
}

func toolPatchBufferPath(root, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("patch buffer id is required")
	}
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			return "", fmt.Errorf("patch buffer id %q contains unsupported character %q", id, r)
		}
	}
	clean := b.String()
	if clean == "" || clean != id {
		return "", fmt.Errorf("invalid patch buffer id %q", id)
	}
	return filepath.Join(root, ".novelgen", "agent-patches", clean+".txt"), nil
}

func runToolOutlinePatch(cmd *cobra.Command, root, target, id string, patchBytes []byte, apply bool, warnings []string) error {
	ctx := toolProjectContext{Root: root}
	if err := ctx.loadSetup(); err != nil {
		return err
	}
	if err := ctx.loadOutline(); err != nil {
		return err
	}

	beforeOutline := cloneOutline(ctx.Outline)
	var changes []toolPatchChange
	var err error
	switch target {
	case "chapter":
		changes, err = applyChapterPatch(ctx.Outline, id, patchBytes)
	case "volume":
		changes, err = applyVolumePatch(ctx.Outline, id, patchBytes)
	}
	if err != nil {
		return err
	}

	scope := target
	check, err := runToolPatchValidationCheck(ctx.Setup, ctx.Outline, scope, id)
	if err != nil {
		return err
	}
	addToolCheckMeta(check, "project_root", root)
	globalCheck, err := runToolPatchValidationCheck(ctx.Setup, ctx.Outline, "all", "")
	if err != nil {
		return err
	}
	addToolCheckMeta(globalCheck, "project_root", root)
	if newBlocking := newBlockingPatchIssues(beforeOutline, ctx.Setup, globalCheck); len(newBlocking) > 0 {
		ctx.Outline = beforeOutline
		first := newBlocking[0]
		return fmt.Errorf("patch rejected: global outline check introduced %d new blocking issue(s); first [%s/%s] %s: %s", len(newBlocking), first.Priority, first.Category, first.TargetID, first.Issue)
	}
	if apply && check.Blocking {
		ctx.Outline = beforeOutline
		return fmt.Errorf("patch rejected: quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolPatchCheckpoint(root, beforeOutline)
		if err != nil {
			return err
		}
		outlinePath := filepath.Join(root, "story", "compose", "outline.json")
		if err := savePartialOutline(ctx.Outline, outlinePath); err != nil {
			return fmt.Errorf("failed to save patched outline: %w", err)
		}
		mdPath := filepath.Join(root, "story", "compose", "outline.md")
		if err := createOutlineMarkdown(ctx.Outline, mdPath); err != nil {
			return fmt.Errorf("failed to save outline markdown: %w", err)
		}
		files["outline"] = outlinePath
		files["markdown"] = mdPath
		files["checkpoint"] = checkpointPath
	} else {
		ctx.Outline = beforeOutline
	}

	result := toolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      target,
		ID:          id,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, outlinePatchPostCheckCommand(target, id)),
		Meta: map[string]interface{}{
			"project_root": root,
			"validation":   toolPatchValidationMeta(check),
		},
	}
	return writeJSON(cmd, result)
}

func newBlockingPatchIssues(before *models.Outline, setup *models.StorySetup, afterCheck *toolCheckResult) []models.ReviewSuggestion {
	if before == nil || setup == nil || afterCheck == nil {
		return nil
	}
	beforeCheck, err := runToolPatchValidationCheck(setup, before, "all", "")
	if err != nil {
		return nil
	}
	beforeBlocking := map[string]bool{}
	for _, issue := range beforeCheck.Issues {
		if toolPatchIssueIsBlocking(issue) {
			beforeBlocking[toolPatchIssueSignature(issue)] = true
		}
	}
	var out []models.ReviewSuggestion
	for _, issue := range afterCheck.Issues {
		if !toolPatchIssueIsBlocking(issue) {
			continue
		}
		if beforeBlocking[toolPatchIssueSignature(issue)] {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func toolPatchIssueIsBlocking(issue models.ReviewSuggestion) bool {
	rank, ok := reviewPriorityRank(issue.Priority)
	return ok && rank >= 3
}

func toolPatchIssueSignature(issue models.ReviewSuggestion) string {
	return strings.Join([]string{
		normalizeKey(issue.Category),
		strings.ToLower(strings.TrimSpace(issue.TargetID)),
		strings.ToLower(strings.TrimSpace(issue.TargetName)),
		strings.ToLower(strings.TrimSpace(issue.Issue)),
	}, "\x00")
}

func runToolCraftPatch(cmd *cobra.Command, root, target, id string, patchBytes []byte, apply bool, warnings []string) error {
	filename, err := craftPatchFilename(target)
	if err != nil {
		return err
	}
	values, err := loadRawCraftMap(root, filename)
	if err != nil {
		return err
	}
	if values == nil {
		values = map[string]map[string]interface{}{}
	}

	beforeValues := cloneRawCraftMap(values)
	key, action, changes, err := applyCraftElementPatch(values, target, id, patchBytes)
	if err != nil {
		return err
	}

	gate := qualityGateResult{}
	validateCraftSchemaObject(&gate, target, key, values[key])
	if normalizeCraftPatchTarget(target) == "item" {
		validateCraftConsistencyObject(&gate, target, key, values[key], loadCraftCharacterNames(root))
	}
	validateCraftQualityObject(&gate, target, key, values[key])
	gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
	check := makeToolCheckResult("schema", "craft", target, key, gate)
	addToolCheckMeta(check, "project_root", root)

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolCraftPatchCheckpoint(root, filename, beforeValues)
		if err != nil {
			return err
		}
		path := filepath.Join(root, "story", "craft", filename)
		if err := saveRawCraftMap(path, values); err != nil {
			return err
		}
		files["craft"] = path
		files["checkpoint"] = checkpointPath
	}

	result := toolPatchResult{
		OK:          true,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "craft." + target,
		ID:          key,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", target, key)),
		Meta: map[string]interface{}{
			"project_root": root,
			"action":       action,
		},
	}
	return writeJSON(cmd, result)
}

func runToolSetupPatch(cmd *cobra.Command, root string, patchBytes []byte, apply bool, warnings []string) error {
	setupPath := filepath.Join(root, "story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(setupPath)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}
	before := cloneStorySetup(setup)
	merged, setupPatchWarnings, err := applySetupPatchObject(*setup, patchBytes)
	if err != nil {
		return err
	}
	warnings = append(warnings, setupPatchWarnings...)
	if err := utils.ValidateNoSuspiciousPatchText(merged); err != nil {
		return fmt.Errorf("setup patch rejected: %w", err)
	}
	models.NormalizeStorySetup(&merged)

	check, err := runToolSetupCheck("all", &merged)
	if err != nil {
		return err
	}
	addToolCheckMeta(check, "project_root", root)
	if apply && check.Blocking {
		return fmt.Errorf("patch rejected: setup quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolSetupPatchCheckpoint(root, before)
		if err != nil {
			return err
		}
		if err := merged.Save(setupPath); err != nil {
			return fmt.Errorf("failed to save patched story setup: %w", err)
		}
		mdPath := filepath.Join(root, "story", "setup", "story_setup.md")
		if err := createStorySetupMarkdown(&merged, mdPath); err != nil {
			return fmt.Errorf("failed to save story setup markdown: %w", err)
		}
		files["setup"] = setupPath
		files["markdown"] = mdPath
		files["checkpoint"] = checkpointPath
	}

	result := toolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "setup",
		ID:          "story_setup",
		Changed:     diffStructFields("setup", before, merged),
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, "novelgen tool check all --target setup --min-priority medium --max-issues 12"),
		Meta: map[string]interface{}{
			"project_root": root,
		},
	}
	return writeJSON(cmd, result)
}

func applySetupPatchObject(setup models.StorySetup, patchBytes []byte) (models.StorySetup, []string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchBytes, &raw); err != nil {
		return setup, nil, fmt.Errorf("failed to parse setup patch JSON: %w", err)
	}
	scalarPatch := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		switch key {
		case "core_cast", "storylines", "premises", "world_timeline", "world_resources", "long_form_plan", "writing_style", "rules", "genres", "rules_patch", "genres_patch":
			continue
		default:
			scalarPatch[key] = value
		}
	}

	merged := setup
	if len(scalarPatch) > 0 {
		data, err := json.Marshal(scalarPatch)
		if err != nil {
			return setup, nil, err
		}
		mergedValue, mergeErr := mergeJSONPatchObject(setup, data)
		if mergeErr != nil {
			return setup, nil, mergeErr
		}
		merged = mergedValue
	}

	var upserted []string
	var err error
	if value, ok := raw["genres"]; ok {
		merged.Genres, err = upsertSetupStringList(merged.Genres, value, "genres")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "genres")
	}
	if value, ok := raw["rules"]; ok {
		merged.Rules, err = upsertSetupStringList(merged.Rules, value, "rules")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "rules")
	}
	if value, ok := raw["genres_patch"]; ok {
		merged.Genres, err = applySetupStringListPatch(merged.Genres, value, "genres_patch")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "genres_patch")
	}
	if value, ok := raw["rules_patch"]; ok {
		merged.Rules, err = applySetupStringListPatch(merged.Rules, value, "rules_patch")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "rules_patch")
	}
	if value, ok := raw["core_cast"]; ok {
		merged.CoreCast, err = upsertSetupPatchItems(merged.CoreCast, value, setupCoreCastPatchKey, "core_cast")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "core_cast")
	}
	if value, ok := raw["storylines"]; ok {
		merged.Storylines, err = upsertSetupPatchItems(merged.Storylines, value, func(item models.Storyline) string { return item.Name }, "storylines")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "storylines")
	}
	if value, ok := raw["premises"]; ok {
		merged.Premises, err = upsertSetupPatchItems(merged.Premises, value, func(item models.Premise) string { return item.Name }, "premises")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "premises")
	}
	if value, ok := raw["world_timeline"]; ok {
		merged.WorldTimeline, err = upsertSetupPatchItems(merged.WorldTimeline, value, setupTimelinePatchKey, "world_timeline")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "world_timeline")
	}
	if value, ok := raw["world_resources"]; ok {
		merged.WorldResources, err = upsertSetupPatchItems(merged.WorldResources, value, func(item models.WorldResource) string { return item.Name }, "world_resources")
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "world_resources")
	}
	if value, ok := raw["long_form_plan"]; ok {
		merged.LongFormPlan, err = mergeSetupLongFormPlan(merged.LongFormPlan, value)
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "long_form_plan")
	}
	if value, ok := raw["writing_style"]; ok {
		merged.WritingStyle, err = mergeSetupWritingStyle(merged.WritingStyle, value)
		if err != nil {
			return setup, nil, err
		}
		upserted = append(upserted, "writing_style")
	}

	var warnings []string
	if len(upserted) > 0 {
		warnings = append(warnings, "setup patch used incremental merge/upsert semantics for field(s): "+strings.Join(upserted, ", "))
	}
	return merged, warnings, nil
}

func upsertSetupStringList(existing []string, patchRaw json.RawMessage, path string) ([]string, error) {
	if isJSONNull(patchRaw) {
		return nil, nil
	}
	var items []string
	if err := json.Unmarshal(patchRaw, &items); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of strings: %w", path, err)
	}
	return appendUniqueSetupStrings(existing, items), nil
}

type setupStringListPatchItem struct {
	Index int    `json:"index"`
	Value string `json:"value"`
}

func applySetupStringListPatch(existing []string, patchRaw json.RawMessage, path string) ([]string, error) {
	if isJSONNull(patchRaw) {
		return existing, nil
	}
	var items []setupStringListPatchItem
	if err := json.Unmarshal(patchRaw, &items); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of {index,value}: %w", path, err)
	}
	out := append([]string(nil), existing...)
	for _, item := range items {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, fmt.Errorf("setup patch %s index %d out of range 0..%d", path, item.Index, len(out)-1)
		}
		value := strings.TrimSpace(item.Value)
		if value == "" {
			return nil, fmt.Errorf("setup patch %s index %d value is empty", path, item.Index)
		}
		out[item.Index] = value
	}
	return out, nil
}

func mergeSetupLongFormPlan(existing *models.LongFormPlan, patchRaw json.RawMessage) (*models.LongFormPlan, error) {
	if isJSONNull(patchRaw) {
		return nil, nil
	}
	base := models.LongFormPlan{}
	if existing != nil {
		base = *existing
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchRaw, &raw); err != nil {
		return nil, fmt.Errorf("setup patch long_form_plan must be an object or null: %w", err)
	}
	patchCopy := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		switch key {
		case "escalation_ladder", "reader_promises", "volume_pattern":
			continue
		default:
			patchCopy[key] = value
		}
	}
	merged := base
	if len(patchCopy) > 0 {
		data, err := json.Marshal(patchCopy)
		if err != nil {
			return nil, err
		}
		mergedValue, err := mergeJSONPatchObject(base, data)
		if err != nil {
			return nil, fmt.Errorf("setup patch long_form_plan does not match schema: %w", err)
		}
		merged = mergedValue
	}
	var err error
	if value, ok := raw["escalation_ladder"]; ok {
		merged.EscalationLadder, err = upsertSetupStringList(base.EscalationLadder, value, "long_form_plan.escalation_ladder")
		if err != nil {
			return nil, err
		}
	}
	if value, ok := raw["reader_promises"]; ok {
		merged.ReaderPromises, err = upsertSetupStringList(base.ReaderPromises, value, "long_form_plan.reader_promises")
		if err != nil {
			return nil, err
		}
	}
	if value, ok := raw["volume_pattern"]; ok {
		merged.VolumePattern, err = upsertSetupStringList(base.VolumePattern, value, "long_form_plan.volume_pattern")
		if err != nil {
			return nil, err
		}
	}
	return &merged, nil
}

func mergeSetupWritingStyle(existing models.WritingStyle, patchRaw json.RawMessage) (models.WritingStyle, error) {
	if isJSONNull(patchRaw) {
		return models.WritingStyle{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchRaw, &raw); err != nil {
		return existing, fmt.Errorf("setup patch writing_style must be an object or null: %w", err)
	}
	patchCopy := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		switch key {
		case "principles", "avoid":
			continue
		default:
			patchCopy[key] = value
		}
	}
	merged := existing
	if len(patchCopy) > 0 {
		data, err := json.Marshal(patchCopy)
		if err != nil {
			return existing, err
		}
		mergedValue, err := mergeJSONPatchObject(existing, data)
		if err != nil {
			return existing, fmt.Errorf("setup patch writing_style does not match schema: %w", err)
		}
		merged = mergedValue
	}
	var err error
	if value, ok := raw["principles"]; ok {
		merged.Principles, err = upsertSetupStringList(existing.Principles, value, "writing_style.principles")
		if err != nil {
			return existing, err
		}
	}
	if value, ok := raw["avoid"]; ok {
		merged.Avoid, err = upsertSetupStringList(existing.Avoid, value, "writing_style.avoid")
		if err != nil {
			return existing, err
		}
	}
	return merged, nil
}

func appendUniqueSetupStrings(existing, patch []string) []string {
	out := append([]string(nil), existing...)
	seen := map[string]bool{}
	for _, item := range out {
		key := normalizeSetupPatchKey(item)
		if key != "" {
			seen[key] = true
		}
	}
	for _, item := range patch {
		item = strings.TrimSpace(item)
		key := normalizeSetupPatchKey(item)
		if key == "" || seen[key] {
			continue
		}
		out = append(out, item)
		seen[key] = true
	}
	return out
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.EqualFold(strings.TrimSpace(string(raw)), "null")
}

func upsertSetupPatchItems[T any](existing []T, patchRaw json.RawMessage, keyFn func(T) string, path string) ([]T, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(patchRaw, &raws); err != nil {
		return nil, fmt.Errorf("setup patch %s must be an array of objects: %w", path, err)
	}
	out := append([]T(nil), existing...)
	for _, raw := range raws {
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("setup patch %s item does not match schema: %w", path, err)
		}
		key := normalizeSetupPatchKey(keyFn(item))
		if key == "" {
			out = append(out, item)
			continue
		}
		found := false
		for i := range out {
			if normalizeSetupPatchKey(keyFn(out[i])) != key {
				continue
			}
			merged, err := mergeJSONPatchObject(out[i], raw)
			if err != nil {
				return nil, fmt.Errorf("setup patch %s item %q does not match schema: %w", path, keyFn(item), err)
			}
			out[i] = merged
			found = true
			break
		}
		if !found {
			out = append(out, item)
		}
	}
	return out, nil
}

func setupCoreCastPatchKey(item models.CoreCastSeed) string {
	if strings.TrimSpace(item.ID) != "" {
		return "id:" + item.ID
	}
	return "name:" + item.Name
}

func setupTimelinePatchKey(item models.WorldTimelineEntry) string {
	if strings.TrimSpace(item.RelatedMystery) != "" {
		return "mystery:" + item.RelatedMystery
	}
	if strings.TrimSpace(item.Year) != "" || strings.TrimSpace(item.Event) != "" {
		return "time:" + item.Year + "|" + item.Event
	}
	return ""
}

func normalizeSetupPatchKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func runToolRecapPatch(cmd *cobra.Command, root, id string, patchBytes []byte, apply bool, warnings []string) error {
	store := recap.NewStore(root)
	before, err := store.Load(id)
	if err != nil {
		before = &models.ChapterRecap{ChapterID: id}
		warnings = append(warnings, fmt.Sprintf("recap file not found; patch will create %s", id))
	}
	var outlineChapter *models.Chapter
	if outline, err := models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json")); err == nil && outline != nil {
		outlineChapter = outline.GetChapterByID(id)
	}

	merged, err := mergeJSONPatchObject(*before, patchBytes)
	if err != nil {
		return err
	}
	patchIdentityWarnings := normalizeRecapPatchIdentity(&merged, id, outlineChapter, patchBytes)
	warnings = append(warnings, patchIdentityWarnings...)
	if err := utils.ValidateNoSuspiciousPatchText(merged); err != nil {
		return fmt.Errorf("recap patch rejected: %w", err)
	}

	check := runToolRecapCheckForValue("all", "chapter", id, &merged, outlineChapter)
	addToolCheckMeta(check, "project_root", root)
	if apply && check.Blocking {
		return fmt.Errorf("patch rejected: recap check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if apply {
		checkpointPath, err := saveToolRecapPatchCheckpoint(root, id, before)
		if err != nil {
			return err
		}
		if err := store.Save(&merged); err != nil {
			return fmt.Errorf("failed to save patched recap: %w", err)
		}
		files["recap"] = filepath.Join(root, "story", "recaps", id+".json")
		files["checkpoint"] = checkpointPath
	}

	result := toolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "recap.chapter",
		ID:          id,
		Changed:     diffStructFields("recap."+id, *before, merged),
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(apply, check, fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", id)),
		Meta: map[string]interface{}{
			"project_root": root,
		},
	}
	return writeJSON(cmd, result)
}

type toolChapterPatch struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content"`
}

func runToolChapterPatch(cmd *cobra.Command, root, id string, patchBytes []byte, apply bool, warnings []string) error {
	outline, err := models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json"))
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}
	chapter := outline.GetChapterByID(id)
	if chapter == nil {
		return fmt.Errorf("chapter %q not found in outline", id)
	}
	var patch toolChapterPatch
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return fmt.Errorf("failed to parse chapter patch: %w", err)
	}
	if strings.TrimSpace(patch.ID) != "" && strings.TrimSpace(patch.ID) != id {
		return fmt.Errorf("patch id %q does not match target chapter %q", patch.ID, id)
	}
	content := normalizeChapterPatchContent(chapter, patch.Content)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("chapter patch content is required")
	}
	if err := utils.ValidateNoSuspiciousPatchText(content); err != nil {
		return fmt.Errorf("chapter patch rejected: %w", err)
	}
	targetWords := toolPatchFlags.TargetWords
	if targetWords <= 0 {
		targetWords = toolChapterTargetWords()
	}
	beforePath, beforeContent := loadFinalChapterContentWithPath(root, id)
	if err := validateToolChapterPatchLength(chapter, content, targetWords, beforeContent); err != nil {
		return fmt.Errorf("chapter patch rejected: %w", err)
	}

	changes := []toolPatchChange{{
		Path:   "chapter." + id + ".content",
		Action: "update",
	}}
	if beforePath == "" {
		changes[0].Action = "create"
	}

	qualityCheck := makeToolCheckResult("quality", "chapter", "chapter", id, runToolChapterQualityGate(chapter, content, targetWords))
	addToolCheckMeta(qualityCheck, "project_root", root)
	addToolCheckMeta(qualityCheck, "mode", "pre_apply")
	addToolCheckMeta(qualityCheck, "target_words", targetWords)
	if apply && qualityCheck.Blocking {
		return fmt.Errorf("patch rejected: chapter quality check found blocking issues (critical=%d high=%d total=%d)", qualityCheck.Summary.Critical, qualityCheck.Summary.High, qualityCheck.Summary.Total)
	}

	files := map[string]string{}
	check := qualityCheck
	refreshedDerived := false
	if apply {
		checkpointPath, err := saveToolChapterPatchCheckpoint(root, id, beforePath, beforeContent)
		if err != nil {
			return err
		}
		path := toolFinalChapterPathForID(root, id)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to save patched chapter: %w", err)
		}
		files["chapter"] = path
		files["checkpoint"] = checkpointPath
		if toolPatchFlags.RefreshDerived {
			var refresh *toolRefreshResult
			func() {
				restoreLogger := suppressToolRefreshLogs()
				defer restoreLogger()
				refresh, err = refreshToolChapterDSL(cmd.Context(), root, id, 0)
			}()
			if err != nil {
				return fmt.Errorf("chapter patch applied but derived RPG DSL refresh failed: %w", err)
			}
			refreshedDerived = true
			if refresh.DSLPath != "" {
				files["chapter_dsl"] = refresh.DSLPath
			}
			if refresh.CacheDir != "" {
				files["chapter_dsl_cache"] = refresh.CacheDir
			}
			postCheck, err := runToolChapterCheckWithTargetWords(root, "all", "chapter", id, targetWords)
			if err != nil {
				return err
			}
			addToolCheckMeta(postCheck, "project_root", root)
			addToolCheckMeta(postCheck, "mode", "post_apply_refresh_all")
			addToolCheckMeta(postCheck, "derived_dsl_refreshed", true)
			check = postCheck
		} else {
			postCheck, err := runToolChapterCheckWithTargetWords(root, "quality", "chapter", id, targetWords)
			if err != nil {
				return err
			}
			addToolCheckMeta(postCheck, "project_root", root)
			addToolCheckMeta(postCheck, "mode", "post_apply_quality")
			addToolCheckMeta(postCheck, "simulation_requires_derived_dsl_refresh", true)
			check = postCheck
		}
	}

	result := toolPatchResult{
		OK:          !check.Blocking,
		Applied:     apply,
		DryRun:      !apply,
		Target:      "chapter",
		ID:          id,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildChapterPatchNextActions(id, apply, refreshedDerived, check),
		Meta: map[string]interface{}{
			"project_root":           root,
			"previous_content_path":  beforePath,
			"previous_content_bytes": len(beforeContent),
			"content_bytes":          len(content),
			"refresh_derived":        refreshedDerived,
		},
	}
	return writeJSON(cmd, result)
}

func buildChapterPatchNextActions(chapterID string, applied bool, refreshedDerived bool, check *toolCheckResult) []toolNextAction {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return nil
	}
	if !applied {
		actions := []toolNextAction{{
			Step:    1,
			Action:  "apply_validated_patch",
			Purpose: "If this dry-run patch is acceptable and the workflow allows writes, repeat the same patch command with --apply.",
			When:    "Only after check.blocking is false and apply is explicitly allowed.",
		}}
		if check != nil && check.Blocking {
			actions[0] = toolNextAction{
				Step:    1,
				Action:  "repair_patch_content",
				Purpose: "Use check.issues navigation to repair the proposed chapter content before trying apply.",
				When:    "The dry-run check is blocking.",
			}
		}
		return actions
	}
	if refreshedDerived {
		if check != nil && check.Blocking {
			return []toolNextAction{{
				Step:    1,
				Action:  "repair_remaining_issues",
				Purpose: "Patch applied and derived RPG DSL was refreshed, but the post-refresh all check is blocking; repair the returned check.issues one target at a time.",
				Command: fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID),
			}}
		}
		return []toolNextAction{{
			Step:    1,
			Action:  "return_final_json",
			Purpose: "Patch applied, derived RPG DSL refreshed, and post-refresh all check is clean for blocking issues.",
		}}
	}
	return []toolNextAction{
		{
			Step:    1,
			Action:  "refresh_derived_dsl",
			Purpose: "Chapter prose changed; regenerate derived RPG DSL before trusting simulation results.",
			Command: fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID),
		},
		{
			Step:    2,
			Action:  "post_refresh_check",
			Purpose: "Run deterministic quality and simulation checks after derived DSL refresh.",
			Command: fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID),
		},
	}
}

func buildPatchNextActions(applied bool, check *toolCheckResult, postApplyCheckCommand string) []toolNextAction {
	postApplyCheckCommand = strings.TrimSpace(postApplyCheckCommand)
	if !applied {
		action := toolNextAction{
			Step:    1,
			Action:  "apply_validated_patch",
			Purpose: "If this dry-run patch is acceptable and the workflow allows writes, repeat the same patch command with --apply.",
			When:    "Only after check.blocking is false and apply is explicitly allowed.",
		}
		if check != nil && check.Blocking {
			action = toolNextAction{
				Step:    1,
				Action:  "repair_patch_content",
				Purpose: "Use check.issues navigation to repair the proposed patch before trying apply.",
				When:    "The dry-run check is blocking.",
			}
		}
		return []toolNextAction{action}
	}
	if postApplyCheckCommand == "" {
		return []toolNextAction{{
			Step:    1,
			Action:  "inspect_patch_check",
			Purpose: "Use the check object returned by this patch result before deciding whether another repair is needed.",
		}}
	}
	return []toolNextAction{{
		Step:    1,
		Action:  "post_apply_check",
		Purpose: "Verify the saved project state after the validated patch write.",
		Command: postApplyCheckCommand,
	}}
}

func outlinePatchPostCheckCommand(target, id string) string {
	target = normalizeKey(target)
	id = strings.TrimSpace(id)
	switch target {
	case "volume":
		if id != "" {
			return fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --min-priority medium --max-issues 12", id)
		}
	case "chapter":
		if id != "" {
			return fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --min-priority low --max-issues 8", id)
		}
	}
	return ""
}

func normalizeChapterPatchContent(chapter *models.Chapter, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if chapter == nil || strings.TrimSpace(chapter.Title) == "" {
		return content
	}
	title := strings.TrimSpace(chapter.Title)
	titlePrefix := "# " + title
	if hasToolChapterMarkdownTitle(content, title) {
		body := stripToolChapterMarkdownTitle(content, title)
		for hasToolChapterMarkdownTitle(body, title) {
			body = stripToolChapterMarkdownTitle(body, title)
		}
		if cleaned := stripRepeatedChapterTitlePrefix(body, title); cleaned != body {
			return titlePrefix + "\n\n" + cleaned
		}
		if body == "" {
			return titlePrefix
		}
		return titlePrefix + "\n\n" + body
	}
	return titlePrefix + "\n\n" + stripRepeatedChapterTitlePrefix(content, title)
}

func validateToolChapterPatchLength(chapter *models.Chapter, content string, targetWords int, previousContent ...string) error {
	if targetWords <= 0 {
		return nil
	}
	chapterID := ""
	chapterTitle := ""
	if chapter != nil {
		chapterID = chapter.ID
		chapterTitle = chapter.Title
	}
	body := strings.TrimSpace(stripToolChapterMarkdownTitle(content, chapterTitle))
	count := toolNarrativeUnitCount(body)
	hardMax := toolChapterLengthHardMax(targetWords)
	if count > hardMax {
		return fmt.Errorf("content is too long for chapter %s: got %d narrative units, target %d, hard max %d; trim the patch and keep only the selected chapter", chapterID, count, targetWords, hardMax)
	}
	if len(previousContent) > 0 {
		previousBody := strings.TrimSpace(stripToolChapterMarkdownTitle(previousContent[0], chapterTitle))
		previousCount := toolNarrativeUnitCount(previousBody)
		minPreviousForShrinkGuard := targetWords / 2
		if minPreviousForShrinkGuard < 500 {
			minPreviousForShrinkGuard = 500
		}
		if previousCount >= minPreviousForShrinkGuard && previousCount <= hardMax {
			minRetained := int(float64(previousCount) * 0.85)
			if count < minRetained {
				return fmt.Errorf("content removes too much existing prose for chapter %s: got %d narrative units, previous %d, minimum retained %d; keep this as a minimal patch unless the check issue is explicitly about excessive length", chapterID, count, previousCount, minRetained)
			}
		}
		previousParagraphs := toolChapterParagraphCount(previousBody)
		paragraphs := toolChapterParagraphCount(body)
		if previousParagraphs >= 4 {
			minParagraphs := previousParagraphs / 2
			if minParagraphs < 3 {
				minParagraphs = 3
			}
			if minParagraphs > 6 {
				minParagraphs = 6
			}
			if paragraphs < minParagraphs {
				return fmt.Errorf("content collapses chapter %s paragraph structure: got %d prose paragraphs, previous %d, minimum %d; preserve mobile-readable paragraph breaks when patching", chapterID, paragraphs, previousParagraphs, minParagraphs)
			}
		}
	}
	return nil
}

func toolChapterLengthHardMax(targetWords int) int {
	if targetWords <= 0 {
		return 0
	}
	hardMax := int(float64(targetWords) * 1.2)
	if targetWords+250 > hardMax {
		hardMax = targetWords + 250
	}
	return hardMax
}

func loadFinalChapterContentWithPath(root, chapterID string) (string, string) {
	candidates := []string{
		toolFinalChapterPathForID(root, chapterID),
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", extractChapterNumber(chapterID))),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return path, string(data)
		}
	}
	return "", ""
}

func validateToolPatchBytes(patchBytes []byte) error {
	return utils.ValidatePatchJSONText(patchBytes)
}

func runToolPatchValidationCheck(setup *models.StorySetup, outline *models.Outline, scope, id string) (*toolCheckResult, error) {
	return runToolOutlineCheck("all", setup, outline, scope, id)
}

func toolPatchValidationMeta(check *toolCheckResult) map[string]interface{} {
	if check == nil {
		return nil
	}
	meta := map[string]interface{}{
		"kind":     check.Kind,
		"target":   check.Target,
		"scope":    check.Scope,
		"id":       check.ID,
		"blocking": check.Blocking,
		"score":    check.Score,
	}
	if coverage, ok := check.Meta["coverage"]; ok {
		meta["coverage"] = coverage
	}
	return meta
}

func runToolSetupCheck(kind string, setup *models.StorySetup) (*toolCheckResult, error) {
	var gate qualityGateResult
	switch kind {
	case "quality":
		gate = runSetupQualityGate(setup)
	case "simulation":
		gate = runSetupSimulationGate(setup)
	case "all":
		gate = runSetupQualityGate(setup)
		sim := runSetupSimulationGate(setup)
		gate.add(sim.Suggestions...)
		gate.dedup()
		gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
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
	var gate qualityGateResult
	switch kind {
	case "quality":
		if scope == "all" {
			gate = runOutlineQualityGate(setup, scoped)
		} else {
			gate = runScopedOutlineQualityGate(setup, outline)
		}
	case "simulation":
		gate = runOutlineSimulationGate(setup, outline)
	case "all":
		if scope == "all" {
			gate = runOutlineQualityGate(setup, scoped)
		} else {
			gate = runScopedOutlineQualityGate(setup, outline)
		}
		sim := runOutlineSimulationGate(setup, outline)
		gate.add(sim.Suggestions...)
		gate.dedup()
		gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
	}
	if scope != "all" {
		gate.Suggestions = filterToolScopedSuggestions(gate.Suggestions, scoped, scope, id)
		gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
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
	gate := qualityGateResult{}
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
		gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
		return makeToolCheckResult("schema", "craft", scope, key, gate), nil
	}
	for key, value := range values {
		validateCraftSchemaObject(&gate, scope, key, value)
		validateCraftConsistencyObject(&gate, scope, key, value, knownCharacters)
		validateCraftQualityObject(&gate, scope, key, value)
	}
	gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
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
	gate := qualityGateResult{}
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
	gate := qualityGateResult{}
	validateRecapForToolCheck(&gate, recapData, id, outlineChapter)
	gate.dedup()
	gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
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
	var gate qualityGateResult
	switch kind {
	case "quality":
		gate = runToolChapterQualityGate(chapter, content, targetWords)
	case "simulation":
		gate = runToolChapterSimulationGate(root, id)
	case "all":
		gate = runToolChapterQualityGate(chapter, content, targetWords)
		sim := runToolChapterSimulationGate(root, id)
		gate.add(sim.Suggestions...)
		gate.dedup()
		gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
	}
	result := makeToolCheckResult(kind, "chapter", scope, id, gate)
	if targetWords > 0 {
		addToolCheckMeta(result, "target_words", targetWords)
	}
	return result, nil
}

func runToolChapterQualityGate(chapter *models.Chapter, content string, targetWords int) (gate qualityGateResult) {
	defer func() {
		gate.dedup()
		gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
	}()
	chapterID := ""
	chapterTitle := ""
	if chapter != nil {
		chapterID = strings.TrimSpace(chapter.ID)
		chapterTitle = strings.TrimSpace(chapter.Title)
	}
	targetName := coalesceString(chapterTitle, chapterID)
	clean := strings.TrimSpace(content)
	if clean == "" {
		gate.add(qualitySuggestion("chapter", chapterID, targetName, "final chapter markdown is missing or empty", "Run write gen or write improve for this chapter, then rerun tool check quality --target chapter.", models.PriorityHigh))
		return gate
	}

	if chapterTitle != "" && !strings.HasPrefix(clean, "# "+chapterTitle) {
		gate.add(qualitySuggestion("structure", chapterID, targetName, "final chapter markdown does not start with the outline title", "Keep the saved chapter headed by the outline title so downstream export and recap tools read the correct chapter.", models.PriorityMedium))
	}

	body := strings.TrimSpace(stripToolChapterMarkdownTitle(clean, chapterTitle))
	if body == "" {
		gate.add(qualitySuggestion("chapter", chapterID, targetName, "final chapter body is empty after the title", "Regenerate the chapter body before running recap or RPG DSL extraction.", models.PriorityHigh))
		return gate
	}
	if chapterTitle != "" && repeatedChapterTitlePrefix(body, chapterTitle) {
		gate.add(qualitySuggestion("structure", chapterID, targetName, "final chapter body repeats the outline title immediately after the heading", "Remove the duplicate title from the first body paragraph; keep one markdown heading and start the body with scene prose.", models.PriorityMedium))
	}
	if strings.HasPrefix(body, "{") && strings.Contains(body, `"content"`) {
		gate.add(qualitySuggestion("chapter", chapterID, targetName, "final chapter body looks like raw JSON output", "Regenerate or improve the chapter so the saved markdown contains prose, not the agent JSON envelope.", models.PriorityHigh))
	}
	if strings.Contains(body, "```") {
		gate.add(qualitySuggestion("chapter", chapterID, targetName, "final chapter body contains markdown code fences", "Remove code fences and save only narrative prose in the final chapter markdown.", models.PriorityHigh))
	}
	if strings.Contains(strings.ToLower(body), "<br") {
		gate.add(qualitySuggestion("structure", chapterID, targetName, "final chapter body contains HTML line break tags", "Replace HTML <br> tags with normal Markdown paragraph breaks or prose punctuation.", models.PriorityMedium))
	}
	if regexp.MustCompile(`\*\*[^*\n][^*\n]*\*\*`).MatchString(body) {
		gate.add(qualitySuggestion("structure", chapterID, targetName, "final chapter body contains inline bold markdown", "Remove inline bold formatting from final prose; keep only the chapter heading as Markdown structure.", models.PriorityMedium))
	}
	for _, artifact := range chapterTypoArtifacts(body) {
		gate.add(qualitySuggestion("prose", chapterID, targetName, fmt.Sprintf("final chapter body contains suspicious typo artifact: %s", artifact), "Fix the suspicious typo while preserving the surrounding prose and chapter events.", models.PriorityMedium))
	}

	count := toolNarrativeUnitCount(body)
	paragraphs := toolChapterParagraphCount(body)
	if count >= 600 && paragraphs < 3 {
		gate.add(qualitySuggestion("structure", chapterID, targetName, fmt.Sprintf("final chapter has too few prose paragraphs: got %d paragraphs for %d narrative units", paragraphs, count), "Split the chapter into mobile-readable prose and dialogue paragraphs while preserving the same events.", models.PriorityMedium))
	}
	if targetWords > 0 {
		min := targetWords / 2
		if min < 120 {
			min = 120
		}
		if count < min {
			gate.add(qualitySuggestion("length", chapterID, targetName, fmt.Sprintf("final chapter is too short: got %d narrative units, target %d", count, targetWords), "Regenerate or improve the chapter with enough scene-level prose to satisfy the target length.", models.PriorityHigh))
		}
		hardMax := toolChapterLengthHardMax(targetWords)
		if count > hardMax {
			gate.add(qualitySuggestion("length", chapterID, targetName, fmt.Sprintf("final chapter is much longer than target: got %d narrative units, target %d", count, targetWords), "Trim the chapter or split non-target material so this saved file contains only the selected chapter.", models.PriorityMedium))
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
			gate.add(qualitySuggestion("character", chapterID, targetName, fmt.Sprintf("final chapter text does not mention outline character(s): %s", strings.Join(missing, ", ")), "Revise the saved chapter so the named outline characters are present under their canonical names, or update the outline only through the validated outline patch workflow if the cast changed.", models.PriorityHigh))
		}
	}

	flavor := style.CheckAIFlavor(body, 75)
	if flavor.HasIssue {
		gate.add(qualitySuggestion("style", chapterID, targetName, fmt.Sprintf("deterministic AI-flavor check score is %d", flavor.Score), strings.Join(flavor.Suggestions, " "), models.PriorityMedium))
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
	clean := strings.TrimSpace(content)
	if title == "" {
		return clean
	}
	if !hasToolChapterMarkdownTitle(clean, title) {
		return clean
	}
	_, rest, _ := strings.Cut(clean, "\n")
	return strings.TrimSpace(rest)
}

func hasToolChapterMarkdownTitle(content, title string) bool {
	clean := strings.TrimSpace(content)
	if title == "" || !strings.HasPrefix(clean, "#") {
		return false
	}
	line, _, _ := strings.Cut(clean, "\n")
	lineTitle := strings.TrimSpace(strings.TrimLeft(line, "# \t"))
	return normalizedChapterTitleText(lineTitle) == normalizedChapterTitleText(title)
}

func repeatedChapterTitlePrefix(content, title string) bool {
	content = strings.TrimSpace(content)
	title = strings.TrimSpace(title)
	return matchingChapterTitlePrefixLen(content, title) > 0
}

func stripRepeatedChapterTitlePrefix(content, title string) string {
	prefixLen := matchingChapterTitlePrefixLen(strings.TrimSpace(content), title)
	if prefixLen <= 0 {
		return strings.TrimSpace(content)
	}
	rest := strings.TrimSpace(strings.TrimSpace(content)[prefixLen:])
	rest = strings.TrimLeft(rest, " \t\r\n:：-—,，.。")
	return strings.TrimSpace(rest)
}

func matchingChapterTitlePrefixLen(content, title string) int {
	content = strings.TrimSpace(content)
	title = strings.TrimSpace(title)
	if content == "" || title == "" {
		return 0
	}
	titleRunes := []rune(title)
	contentRunes := []rune(content)
	if len(contentRunes) < len(titleRunes) {
		return 0
	}
	candidate := string(contentRunes[:len(titleRunes)])
	if normalizedChapterTitleText(candidate) != normalizedChapterTitleText(title) {
		return 0
	}
	return len(candidate)
}

func normalizedChapterTitleText(value string) string {
	replacer := strings.NewReplacer(
		"“", "\"",
		"”", "\"",
		"‘", "'",
		"’", "'",
		"：", ":",
	)
	value = replacer.Replace(strings.TrimSpace(value))
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
}

func toolChapterTargetWords() int {
	cfg, err := loadProjectConfig()
	if err == nil && cfg.ChapterConfig.TargetWordsPerChapter > 0 {
		return cfg.ChapterConfig.TargetWordsPerChapter
	}
	return 2000
}

func toolNarrativeUnitCount(content string) int {
	hasCJK := false
	cjkCount := 0
	for _, r := range content {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			hasCJK = true
			cjkCount++
		}
	}
	if hasCJK {
		return cjkCount
	}
	return len(strings.Fields(content))
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

func runToolChapterSimulationGate(root, chapterID string) qualityGateResult {
	var gate qualityGateResult
	if stale, info := toolChapterRPGDSLStale(root, chapterID); stale {
		gate.add(qualitySuggestion(
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
		gate.add(qualitySuggestion(
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
	gate.add(rpgdsl.NewSimulationBridge().ConvertIssuesToSuggestions(issues)...)
	if diagnostic := buildChapterSimulationSignalDiagnostic(chapterID, dslData, issues); diagnostic != nil {
		gate.add(*diagnostic)
	}
	gate.dedup()
	gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
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

func validateRecapForToolCheck(gate *qualityGateResult, recapData *models.ChapterRecap, expectedID string, outlineChapter *models.Chapter) {
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

func validateCraftSchemaObject(gate *qualityGateResult, scope, key string, value map[string]interface{}) {
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

func validateCraftConsistencyObject(gate *qualityGateResult, scope, key string, value map[string]interface{}, knownCharacters map[string]bool) {
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

func validateCraftQualityObject(gate *qualityGateResult, scope, key string, value map[string]interface{}) {
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

func makeToolCheckResult(kind, target, scope, id string, gate qualityGateResult) *toolCheckResult {
	critical, high, medium, low := countSuggestionPriorities(gate.Suggestions)
	return &toolCheckResult{
		Kind:     kind,
		Target:   target,
		Scope:    scope,
		ID:       id,
		OK:       !gate.Blocking,
		Blocking: gate.Blocking,
		Score:    scoreFromGate(gate),
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

func applyChapterPatch(outline *models.Outline, chapterID string, patchBytes []byte) ([]toolPatchChange, error) {
	chapter := outline.GetChapterByID(chapterID)
	if chapter == nil {
		return nil, fmt.Errorf("chapter %q not found", chapterID)
	}
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(patchBytes, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if strings.TrimSpace(probe.ID) != "" && probe.ID != chapter.ID {
		return nil, fmt.Errorf("patch id %q does not match target chapter %q", probe.ID, chapter.ID)
	}
	before := *chapter
	merged, err := mergeJSONPatchObject(before, patchBytes)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(merged.ID) != "" && merged.ID != before.ID {
		return nil, fmt.Errorf("patch id %q does not match target chapter %q", merged.ID, before.ID)
	}
	merged.ID = before.ID
	*chapter = merged
	return diffStructFields("outline.chapter."+chapterID, before, merged), nil
}

type outlineVolumePatch struct {
	ID              string                       `json:"id,omitempty"`
	Title           string                       `json:"title,omitempty"`
	Summary         string                       `json:"summary,omitempty"`
	PayoffContract  *models.VolumePayoffContract `json:"payoff_contract,omitempty"`
	ChangedChapters []json.RawMessage            `json:"changed_chapters,omitempty"`
	ChangedEvents   []json.RawMessage            `json:"changed_events,omitempty"`
}

func applyVolumePatch(outline *models.Outline, volumeID string, patchBytes []byte) ([]toolPatchChange, error) {
	volume := outline.GetVolumeByID(volumeID)
	if volume == nil {
		return nil, fmt.Errorf("volume %q not found", volumeID)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if _, ok := raw["chapters"]; ok {
		return nil, fmt.Errorf("volume patch must use changed_chapters; replacing chapters is not allowed")
	}
	var patch outlineVolumePatch
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return nil, fmt.Errorf("failed to parse volume patch: %w", err)
	}
	if strings.TrimSpace(patch.ID) != "" && patch.ID != volume.ID {
		return nil, fmt.Errorf("patch id %q does not match target volume %q", patch.ID, volume.ID)
	}

	before := *volume
	if _, ok := raw["title"]; ok {
		volume.Title = patch.Title
	}
	if _, ok := raw["summary"]; ok {
		volume.Summary = patch.Summary
	}
	if _, ok := raw["payoff_contract"]; ok {
		volume.PayoffContract = models.MergeVolumePayoffContract(volume.PayoffContract, patch.PayoffContract)
	}
	changes := diffStructFields("outline.volume."+volumeID, before, *volume)

	for _, rawChapter := range patch.ChangedChapters {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rawChapter, &probe); err != nil {
			return nil, fmt.Errorf("failed to parse changed chapter patch: %w", err)
		}
		if strings.TrimSpace(probe.ID) == "" {
			return nil, fmt.Errorf("changed_chapters item missing id")
		}
		found := false
		for i := range volume.Chapters {
			if volume.Chapters[i].ID != probe.ID {
				continue
			}
			found = true
			beforeChapter := volume.Chapters[i]
			merged, err := mergeJSONPatchObject(beforeChapter, rawChapter)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(merged.ID) != "" && merged.ID != beforeChapter.ID {
				return nil, fmt.Errorf("patch chapter id %q does not match target chapter %q", merged.ID, beforeChapter.ID)
			}
			merged.ID = beforeChapter.ID
			volume.Chapters[i] = merged
			changes = append(changes, diffStructFields("outline.chapter."+probe.ID, beforeChapter, merged)...)
			break
		}
		if !found {
			return nil, fmt.Errorf("changed chapter %q is not in target volume %q", probe.ID, volumeID)
		}
	}
	for _, rawEvent := range patch.ChangedEvents {
		eventChanges, err := applyVolumeEventPatch(volume, volumeID, rawEvent)
		if err != nil {
			return nil, err
		}
		changes = append(changes, eventChanges...)
	}
	return changes, nil
}

func applyVolumeEventPatch(volume *models.Volume, volumeID string, rawEvent json.RawMessage) ([]toolPatchChange, error) {
	var probe struct {
		ChapterID  string `json:"chapter_id"`
		EventIndex *int   `json:"event_index"`
		Index      *int   `json:"index"`
	}
	if err := json.Unmarshal(rawEvent, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse changed event patch: %w", err)
	}
	chapterID := strings.TrimSpace(probe.ChapterID)
	if chapterID == "" {
		return nil, fmt.Errorf("changed_events item missing chapter_id")
	}
	eventIndex := -1
	if probe.EventIndex != nil {
		eventIndex = *probe.EventIndex
	} else if probe.Index != nil {
		eventIndex = *probe.Index
	} else {
		return nil, fmt.Errorf("changed_events item missing event_index")
	}
	if eventIndex < 0 {
		return nil, fmt.Errorf("changed_events item has negative event_index %d", eventIndex)
	}
	var patchFields map[string]json.RawMessage
	if err := json.Unmarshal(rawEvent, &patchFields); err != nil {
		return nil, fmt.Errorf("failed to parse changed event patch fields: %w", err)
	}
	delete(patchFields, "chapter_id")
	delete(patchFields, "event_index")
	delete(patchFields, "index")
	if len(patchFields) == 0 {
		return nil, fmt.Errorf("changed_events item for %s[%d] has no event fields to patch", chapterID, eventIndex)
	}
	patchBytes, err := json.Marshal(patchFields)
	if err != nil {
		return nil, fmt.Errorf("failed to encode changed event patch: %w", err)
	}
	for chapterIdx := range volume.Chapters {
		if volume.Chapters[chapterIdx].ID != chapterID {
			continue
		}
		if eventIndex >= len(volume.Chapters[chapterIdx].Events) {
			return nil, fmt.Errorf("changed_events index %d out of range for chapter %q", eventIndex, chapterID)
		}
		before := volume.Chapters[chapterIdx].Events[eventIndex]
		merged, err := mergeJSONPatchObject(before, patchBytes)
		if err != nil {
			return nil, err
		}
		volume.Chapters[chapterIdx].Events[eventIndex] = merged
		return diffStructFields(fmt.Sprintf("outline.event.%s.%d", chapterID, eventIndex), before, merged), nil
	}
	return nil, fmt.Errorf("changed event chapter %q is not in target volume %q", chapterID, volumeID)
}

func applyCraftElementPatch(values map[string]map[string]interface{}, kind, id string, patchBytes []byte) (string, string, []toolPatchChange, error) {
	var patch map[string]interface{}
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if len(patch) == 0 {
		return "", "", nil, fmt.Errorf("craft patch must be a non-empty JSON object")
	}
	key, exists := resolveCraftPatchKey(values, id)
	if key == "" {
		key = id
	}
	before := copyStringInterfaceMap(values[key])
	base := copyStringInterfaceMap(before)
	for field, value := range patch {
		if field == "id" {
			continue
		}
		base[field] = value
	}
	if _, ok := base["name"]; !ok {
		base["name"] = key
	}
	normalized, err := normalizeCraftPatchObject(kind, key, base)
	if err != nil {
		return "", "", nil, err
	}
	values[key] = normalized
	action := "update"
	if !exists {
		action = "create"
	}
	return key, action, diffJSONMaps("craft."+kind+"."+key, before, normalized), nil
}

func normalizeCraftPatchTarget(target string) string {
	switch normalizeKey(target) {
	case "character", "characters", "char", "chars":
		return "character"
	case "item", "items":
		return "item"
	case "location", "locations", "loc", "locs":
		return "location"
	case "organization", "organizations", "org", "orgs":
		return "organization"
	default:
		return normalizeKey(target)
	}
}

func craftPatchFilename(kind string) (string, error) {
	switch normalizeCraftPatchTarget(kind) {
	case "character":
		return "characters.json", nil
	case "item":
		return "items.json", nil
	case "location":
		return "locations.json", nil
	case "organization":
		return "organizations.json", nil
	default:
		return "", fmt.Errorf("unsupported craft patch target %q", kind)
	}
}

func resolveCraftPatchKey(values map[string]map[string]interface{}, id string) (string, bool) {
	if _, ok := values[id]; ok {
		return id, true
	}
	lowerID := strings.ToLower(strings.TrimSpace(id))
	for key, value := range values {
		if strings.ToLower(key) == lowerID {
			return key, true
		}
		if rawName, ok := value["name"].(string); ok && strings.ToLower(strings.TrimSpace(rawName)) == lowerID {
			return key, true
		}
	}
	return "", false
}

func normalizeCraftPatchObject(kind, key string, raw map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	switch normalizeCraftPatchTarget(kind) {
	case "character":
		if err := normalizeCraftRPGStatsPatch(raw); err != nil {
			return nil, err
		}
		data, err = json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		var value models.Character
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft character patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		out, err := mergeNormalizedCraftObject(raw, value)
		if _, ok := raw["power_level"]; ok {
			out["power_level"] = value.PowerLevel
		}
		return out, err
	case "item":
		var value models.Item
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft item patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		out, err := mergeNormalizedCraftObject(raw, value)
		if _, ok := raw["power_level"]; ok {
			out["power_level"] = value.PowerLevel
		}
		return out, err
	case "location":
		var value models.Location
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft location patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		out, err := mergeNormalizedCraftObject(raw, value)
		if _, ok := raw["danger_level"]; ok {
			out["danger_level"] = value.DangerLevel
		}
		return out, err
	case "organization":
		var value models.Organization
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("craft organization patch does not match schema: %w", err)
		}
		value.NormalizeForCraft(key)
		return mergeNormalizedCraftObject(raw, value)
	default:
		return nil, fmt.Errorf("unsupported craft patch target %q", kind)
	}
}

func normalizeCraftRPGStatsPatch(raw map[string]interface{}) error {
	stats, ok := raw["rpg_stats"]
	if !ok || stats == nil {
		return nil
	}
	statsMap, ok := stats.(map[string]interface{})
	if !ok {
		return fmt.Errorf("craft character rpg_stats must be an object with supported keys: str, agi, int, vit, hp, mp, level")
	}
	normalized := map[string]interface{}{}
	for key, value := range statsMap {
		canonical, ok := normalizeCraftRPGStatKey(key)
		if !ok {
			return fmt.Errorf("unsupported craft character rpg_stats key %q; use supported keys: str, agi, int, vit, hp, mp, level", key)
		}
		if _, exists := normalized[canonical]; exists {
			return fmt.Errorf("duplicate craft character rpg_stats key %q after normalization", canonical)
		}
		number, err := craftRPGStatNumber(value)
		if err != nil {
			return fmt.Errorf("craft character rpg_stats.%s: %w", key, err)
		}
		normalized[canonical] = number
	}
	raw["rpg_stats"] = normalized
	return nil
}

func normalizeCraftRPGStatKey(key string) (string, bool) {
	switch normalizeKey(key) {
	case "str", "strength", "力量":
		return "str", true
	case "agi", "agility", "dex", "dexterity", "敏捷":
		return "agi", true
	case "int", "intelligence", "智力":
		return "int", true
	case "vit", "vitality", "endurance", "stamina", "体质", "耐力":
		return "vit", true
	case "hp", "health", "生命", "生命值":
		return "hp", true
	case "mp", "mana", "energy", "mind", "精神", "能量", "法力":
		return "mp", true
	case "level", "lvl", "等级":
		return "level", true
	default:
		return "", false
	}
}

func craftRPGStatNumber(value interface{}) (int, error) {
	switch typed := value.(type) {
	case float64:
		number := int(typed)
		if typed != float64(number) {
			return 0, fmt.Errorf("must be an integer")
		}
		return number, nil
	case int:
		return typed, nil
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(number), nil
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

func mergeNormalizedCraftObject(raw map[string]interface{}, normalized interface{}) (map[string]interface{}, error) {
	out := copyStringInterfaceMap(raw)
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	var normalizedMap map[string]interface{}
	if err := json.Unmarshal(data, &normalizedMap); err != nil {
		return nil, err
	}
	for key, value := range normalizedMap {
		out[key] = value
	}
	if err := utils.ValidateNoSuspiciousPatchText(out); err != nil {
		return nil, err
	}
	return out, nil
}

func diffJSONMaps(prefix string, before, after map[string]interface{}) []toolPatchChange {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	changes := make([]toolPatchChange, 0)
	for key := range keys {
		if reflect.DeepEqual(before[key], after[key]) {
			continue
		}
		action := "replace"
		if _, ok := before[key]; !ok {
			action = "add"
		} else if _, ok := after[key]; !ok {
			action = "remove"
		}
		changes = append(changes, toolPatchChange{Path: prefix + "." + key, Action: action})
	}
	return changes
}

func copyStringInterfaceMap(value map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for key, item := range value {
		out[key] = item
	}
	return out
}

func cloneRawCraftMap(values map[string]map[string]interface{}) map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = copyStringInterfaceMap(value)
	}
	return out
}

func mergeJSONPatchObject[T any](original T, patchBytes []byte) (T, error) {
	var base map[string]interface{}
	data, err := json.Marshal(original)
	if err != nil {
		return original, err
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return original, err
	}
	var patch map[string]interface{}
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return original, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	for key, value := range patch {
		if key == "id" {
			continue
		}
		mergeJSONPatchValue(base, key, value)
	}
	mergedBytes, err := json.Marshal(base)
	if err != nil {
		return original, err
	}
	var merged T
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return original, fmt.Errorf("patch does not match target schema: %w", err)
	}
	return merged, nil
}

func mergeJSONPatchValue(base map[string]interface{}, key string, value interface{}) {
	if value == nil {
		base[key] = nil
		return
	}
	patchMap, ok := value.(map[string]interface{})
	if !ok {
		base[key] = value
		return
	}
	baseMap, ok := base[key].(map[string]interface{})
	if !ok || baseMap == nil {
		base[key] = value
		return
	}
	for childKey, childValue := range patchMap {
		if childKey == "id" {
			continue
		}
		mergeJSONPatchValue(baseMap, childKey, childValue)
	}
	base[key] = baseMap
}

func diffStructFields(prefix string, before, after interface{}) []toolPatchChange {
	beforeMap := structJSONMap(before)
	afterMap := structJSONMap(after)
	keys := map[string]bool{}
	for key := range beforeMap {
		keys[key] = true
	}
	for key := range afterMap {
		keys[key] = true
	}
	changes := make([]toolPatchChange, 0)
	for key := range keys {
		if reflect.DeepEqual(beforeMap[key], afterMap[key]) {
			continue
		}
		action := "replace"
		if _, ok := beforeMap[key]; !ok {
			action = "add"
		} else if _, ok := afterMap[key]; !ok {
			action = "remove"
		}
		changes = append(changes, toolPatchChange{Path: prefix + "." + key, Action: action})
	}
	return changes
}

func structJSONMap(value interface{}) map[string]interface{} {
	data, _ := json.Marshal(value)
	var out map[string]interface{}
	_ = json.Unmarshal(data, &out)
	return out
}

func cloneOutline(outline *models.Outline) *models.Outline {
	if outline == nil {
		return nil
	}
	data, _ := json.Marshal(outline)
	var cloned models.Outline
	_ = json.Unmarshal(data, &cloned)
	return &cloned
}

func cloneStorySetup(setup *models.StorySetup) models.StorySetup {
	if setup == nil {
		return models.StorySetup{}
	}
	data, _ := json.Marshal(setup)
	var cloned models.StorySetup
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

func saveToolPatchCheckpoint(root string, outline *models.Outline) (string, error) {
	dir := filepath.Join(root, "story", "compose", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "outline_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveToolSetupPatchCheckpoint(root string, setup models.StorySetup) (string, error) {
	dir := filepath.Join(root, "story", "setup", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "story_setup_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(setup, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveToolCraftPatchCheckpoint(root, filename string, values map[string]map[string]interface{}) (string, error) {
	dir := filepath.Join(root, "story", "craft", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	path := filepath.Join(dir, base+"_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveToolRecapPatchCheckpoint(root, chapterID string, value *models.ChapterRecap) (string, error) {
	dir := filepath.Join(root, "story", "recaps", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, chapterID+"_"+time.Now().Format("20060102_150405")+".json")
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveToolChapterPatchCheckpoint(root, chapterID, beforePath, beforeContent string) (string, error) {
	dir := filepath.Join(root, "chapters", "checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, chapterID+"_"+time.Now().Format("20060102_150405")+".md")
	var sb strings.Builder
	sb.WriteString("<!-- novelgen tool patch chapter checkpoint")
	if strings.TrimSpace(beforePath) != "" {
		sb.WriteString(" source=\"")
		sb.WriteString(beforePath)
		sb.WriteString("\"")
	}
	sb.WriteString(" -->\n\n")
	sb.WriteString(beforeContent)
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func saveRawCraftMap(path string, values map[string]map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func writeJSON(cmd *cobra.Command, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
