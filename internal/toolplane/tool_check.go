// The check result model and the tool check entry point: the result JSON contract, issue budgets
// and next actions, issue filters, and the dispatcher into the per-target checks.

package toolplane

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/quality"
	"novelgen/internal/toolcmd"
)

type ToolCheckResult struct {
	Kind     string                    `json:"kind"`
	Target   string                    `json:"target"`
	Scope    string                    `json:"scope"`
	ID       string                    `json:"id,omitempty"`
	OK       bool                      `json:"ok"`
	Blocking bool                      `json:"blocking"`
	Score    float64                   `json:"score"`
	Summary  ToolCheckSummary          `json:"summary"`
	Issues   []models.ReviewSuggestion `json:"issues,omitempty"`
	Warnings []string                  `json:"warnings,omitempty"`
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
	Summary     ToolCheckSummary       `json:"summary"`
	Issues      []toolCheckIssue       `json:"issues,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	NextActions []ToolNextAction       `json:"next_actions,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type ToolCheckSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

func (r ToolCheckResult) MarshalJSON() ([]byte, error) {
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
		Warnings:    r.Warnings,
		NextActions: toolCheckNextActions(r),
		Meta:        toolCheckMetaForJSON(r),
	})
}

func toolCheckMetaForJSON(result ToolCheckResult) map[string]interface{} {
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

func toolCheckNextActions(result ToolCheckResult) []ToolNextAction {
	if len(result.Issues) == 0 {
		if toolCheckFilterNoMatches(result) {
			command := toolcmd.Check(toolcmd.CheckOptions{
				Kind:        result.Kind,
				Target:      result.Target,
				Scope:       result.Scope,
				ID:          result.ID,
				TargetWords: ToolCheckResultTargetWords(result),
			})
			return []ToolNextAction{{
				Step:    1,
				Action:  "fix_check_filter",
				Command: command,
				Purpose: fmt.Sprintf("No issues matched the current --category/--min-priority filter, but the unfiltered check reported %d issue(s). Widen the filter or rerun the unfiltered check before returning final JSON.", result.Summary.Total),
			}}
		}
		return []ToolNextAction{{
			Step:    1,
			Action:  "return_final_json",
			Purpose: "No returned issues match the current check filters; stop tool exploration and return the workflow JSON.",
		}}
	}
	selectedIssue, nav, selectedIndex, selectedPatchable := toolCheckPreferredIssueNavigation(result)
	firstAction := ToolNextAction{
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
		firstAction.When = "Selected category: " + NormalizeKey(selectedIssue.Category)
	}
	actions := []ToolNextAction{{
		Step:    firstAction.Step,
		Action:  firstAction.Action,
		Purpose: firstAction.Purpose,
		When:    firstAction.When,
	}}
	if refreshCommand := StringMapValue(nav, "refresh_query"); refreshCommand != "" {
		actions[0].Action = "refresh_derived_dsl_first"
		actions[0].Purpose = "Refresh derived chapter RPG DSL before querying repair context or patching prose; stale/missing DSL can be a cache problem."
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "refresh_derived_dsl",
			Command: refreshCommand,
			Purpose: "Regenerate the selected chapter's derived RPG DSL from saved prose.",
		})
		postRefreshCommand := StringMapValue(nav, "post_refresh_check_query")
		if postRefreshCommand == "" {
			postRefreshCommand = StringMapValue(nav, "focused_check_query")
		}
		if postRefreshCommand != "" {
			actions = append(actions, ToolNextAction{
				Step:    len(actions) + 1,
				Action:  "post_refresh_check",
				Command: postRefreshCommand,
				Purpose: "Rerun the focused simulation check after refresh; repair prose only if this check still returns issues.",
			})
		}
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "return_final_json",
			Purpose: "If the post-refresh check is clean, report that refresh resolved the issue; otherwise continue from the returned check.issues navigation.",
		})
		return actions
	}
	if command := StringMapValue(nav, "repair_route_query"); command != "" {
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "query_repair_route",
			Command: command,
			Purpose: "Fetch the smallest route map for the failing target.",
		})
	}
	if command := StringMapValue(nav, "repair_context_query"); command != "" {
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "query_repair_context_if_needed",
			Command: command,
			Purpose: "Fetch focused facts only if the route or issue cannot be repaired from current data.",
			When:    "The route/check summary says target facts, event facts, or excerpts are needed.",
		})
	}
	if command := StringMapValue(nav, "patch_query"); command != "" && selectedPatchable {
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "patch_dry_run",
			Command: command,
			Purpose: "Validate the minimal patch for this issue before any apply.",
			When:    "A concrete patch is ready.",
		})
	}
	if command := StringMapValue(nav, "focused_check_query"); command != "" {
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "focused_recheck",
			Command: command,
			Purpose: "Verify the repaired target before moving to the next issue.",
			When:    "After a patch dry-run/apply or when confirming whether the issue still exists.",
		})
	}
	if !selectedPatchable && StringMapValue(nav, "classification_rule") != "" {
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "return_final_json",
			Purpose: "This global route has no validated patch shape. Report it as remaining/unpatchable or recommend a smaller targeted workflow instead of fetching broader context.",
		})
	}
	return actions
}

func toolCheckPreferredIssueNavigation(result ToolCheckResult) (models.ReviewSuggestion, map[string]interface{}, int, bool) {
	targetWords := ToolCheckResultTargetWords(result)
	bestIssue := result.Issues[0]
	bestNav := ToolIssueNavigation(result.Kind, result.Target, result.Scope, result.ID, bestIssue, targetWords)
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
		nav := ToolIssueNavigation(result.Kind, result.Target, result.Scope, result.ID, issue, targetWords)
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
	return StringMapValue(nav, "patch_query") != "" && nav["patch_shape"] != nil
}

func toolCheckNavigationIsRoutable(nav map[string]interface{}) bool {
	return StringMapValue(nav, "repair_route_query") != "" || StringMapValue(nav, "focused_check_query") != ""
}

func toolCheckIssuePriorityScore(priority string) int {
	switch NormalizeKey(priority) {
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

func ToolCheckResultTargetWords(result ToolCheckResult) int {
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

func toolCheckIssuesForJSON(result ToolCheckResult) []toolCheckIssue {
	if len(result.Issues) == 0 {
		return nil
	}
	issues := make([]toolCheckIssue, 0, len(result.Issues))
	targetWords := ToolCheckResultTargetWords(result)
	for _, issue := range result.Issues {
		issues = append(issues, toolCheckIssue{
			ReviewSuggestion: issue,
			Navigation:       ToolIssueNavigation(result.Kind, result.Target, result.Scope, result.ID, issue, targetWords),
		})
	}
	return issues
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

func ToolVolumeIDFromChapterID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	idx := strings.LastIndex(strings.ToUpper(id), "-C")
	if idx <= 0 {
		return "", false
	}
	return id[:idx], true
}

func ToolLooksLikeVolumeID(id string) bool {
	id = strings.ToUpper(strings.TrimSpace(id))
	return strings.Contains(id, "-V") && !strings.Contains(id, "-C")
}

func runToolCheck(cmd *cobra.Command, args []string) error {
	kind := "all"
	if len(args) > 0 {
		kind = NormalizeKey(args[0])
	}
	if kind == "" {
		kind = "all"
	}
	if kind != "quality" && kind != "simulation" && kind != "all" && kind != "schema" {
		return fmt.Errorf("unsupported check kind %q; valid kinds: quality, simulation, all, schema", kind)
	}
	target := NormalizeKey(ToolCheckFlags.Target)
	if target == "" {
		target = "outline"
	}
	if target != "outline" && target != "craft" && target != "setup" && target != "recap" && target != "chapter" {
		return fmt.Errorf("unsupported check target %q; valid targets: outline, setup, craft, chapter, recap", target)
	}
	if (target == "outline" || target == "setup") && kind == "schema" {
		return fmt.Errorf("kind schema is only supported for --target craft or --target recap, not --target %s; use quality/simulation/all instead", target)
	}
	if target == "craft" && kind != "schema" {
		return fmt.Errorf("craft checks currently support only kind schema, got %q", kind)
	}
	if target == "recap" && kind == "simulation" {
		return fmt.Errorf("recap checks do not support kind simulation; use quality, all, or schema")
	}
	if target == "chapter" && kind != "quality" && kind != "simulation" && kind != "all" {
		return fmt.Errorf("chapter checks currently support only kind quality, simulation, or all")
	}

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Root
	if target == "craft" {
		scope, err := resolveCraftCheckScope(root, ToolCheckFlags.Scope, ToolCheckFlags.ID)
		if err != nil {
			return err
		}
		result, err := runToolCraftSchemaCheckAt(root, scope, ToolCheckFlags.ID)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "recap" {
		result, err := runToolRecapCheckAt(root, kind, ToolCheckFlags.Scope, ToolCheckFlags.ID)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "chapter" {
		result, err := runToolChapterCheckAt(root, kind, ToolCheckFlags.Scope, ToolCheckFlags.ID, ToolCheckFlags.TargetWords)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "setup" {
		ctx := ToolProjectContext{Root: root}
		if err := ctx.loadSetup(); err != nil {
			return err
		}
		result, err := RunToolSetupCheck(kind, ctx.Setup)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}

	ctx := ToolProjectContext{Root: root}
	if err := ctx.loadSetup(); err != nil {
		return err
	}
	if err := ctx.loadOutline(); err != nil {
		return err
	}

	result, err := RunToolOutlineCheck(kind, ctx.Setup, ctx.Outline, ToolCheckFlags.Scope, ToolCheckFlags.ID)
	if err != nil {
		return err
	}
	addToolCheckMeta(result, "project_root", root.Dir())
	applyToolCheckFlagWarnings(result, target)
	if err := ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func ApplyToolCheckIssueFilters(result *ToolCheckResult, minPriority string, category string, maxIssues int) error {
	if result == nil {
		return nil
	}
	minPriority = NormalizeKey(minPriority)
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
		if len(categories) > 0 && !categories[NormalizeKey(issue.Category)] {
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
		"no_matches":      original > 0 && len(filtered) == 0,
	}
	return nil
}

func toolCheckFilterNoMatches(result ToolCheckResult) bool {
	if result.Meta == nil {
		return false
	}
	filter, ok := result.Meta["issue_filter"].(map[string]interface{})
	if !ok {
		return false
	}
	noMatches, _ := filter["no_matches"].(bool)
	return noMatches
}

func parseToolCheckCategories(raw string) map[string]bool {
	categories := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		category := NormalizeKey(item)
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
	switch NormalizeKey(priority) {
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

func MakeToolCheckResult(kind, target, scope, id string, gate quality.QualityGateResult) *ToolCheckResult {
	critical, high, medium, low := quality.CountSuggestionPriorities(gate.Suggestions)
	return &ToolCheckResult{
		Kind:     kind,
		Target:   target,
		Scope:    scope,
		ID:       id,
		OK:       !gate.Blocking,
		Blocking: gate.Blocking,
		Score:    quality.ScoreFromGate(gate),
		Summary: ToolCheckSummary{
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
	kind = NormalizeKey(kind)
	target = NormalizeKey(target)
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

func addToolCheckMeta(result *ToolCheckResult, key string, value interface{}) {
	if result == nil || strings.TrimSpace(key) == "" {
		return
	}
	if result.Meta == nil {
		result.Meta = map[string]interface{}{}
	}
	result.Meta[key] = value
}

func applyToolCheckFlagWarnings(result *ToolCheckResult, target string) {
	if result == nil {
		return
	}
	add := func(message string) {
		result.Warnings = append(result.Warnings, message)
	}
	target = NormalizeKey(target)
	switch target {
	case "setup":
		if scope := normalizeToolScope(ToolCheckFlags.Scope); scope != "all" {
			add(fmt.Sprintf("--scope %q is not used by setup checks and was ignored", ToolCheckFlags.Scope))
		}
		if strings.TrimSpace(ToolCheckFlags.ID) != "" {
			add("--id is not used by setup checks and was ignored")
		}
	case "outline":
		if normalizeToolScope(ToolCheckFlags.Scope) == "all" && strings.TrimSpace(ToolCheckFlags.ID) != "" {
			add("--id is ignored when outline scope is all")
		}
	}
	if target != "chapter" && ToolCheckFlags.TargetWords > 0 {
		add("--target-words is only used by chapter checks and was ignored")
	}
}
