package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/usage"
	"novelgen/internal/platform/utils"
)

const (
	defaultGoalAgentMaxTurns = 40
	defaultGoalAgentTimeout  = 1800
	defaultGoalAgentAttempts = 2
)

// AgentGoalOptions describes one goal-level agent session. Unlike a stage
// method, it does not prescribe queries or a step order; it only supplies the
// objective, the skills the agent may discover, and the write policy.
type AgentGoalOptions struct {
	Goal          string
	Root          *store.Root
	AllowedSkills []string
	AllowWrite    bool
	MaxTurns      int
	Timeout       int
	Model         string
	RuntimeName   string
	Language      string
	Runtime       agentruntime.Runtime
	MaxAttempts   int
	ResumeSession string
}

// AgentGoalReport is the normalized result Go returns after validating the
// agent's proposed completion state.
type AgentGoalReport struct {
	Goal            string                    `json:"goal"`
	Status          string                    `json:"status"`
	Summary         string                    `json:"summary"`
	SkillsUsed      []string                  `json:"skills_used,omitempty"`
	ChangedTargets  []string                  `json:"changed_targets,omitempty"`
	Verification    []string                  `json:"verification,omitempty"`
	RemainingIssues []string                  `json:"remaining_issues,omitempty"`
	AllowWrite      bool                      `json:"allow_write"`
	Model           string                    `json:"model,omitempty"`
	SessionID       string                    `json:"session_id,omitempty"`
	Plan            []agentruntime.PlanItem   `json:"plan,omitempty"`
	PlanUpdated     bool                      `json:"plan_updated,omitempty"`
	Usage           agentruntime.Usage        `json:"usage,omitempty"`
	LiveLogPath     string                    `json:"live_log_path,omitempty"`
	ToolSummary     *agentruntime.LiveSummary `json:"tool_summary,omitempty"`
}

// agentGoalOutput is the wire contract the model returns. The fields are kept
// intentionally small: project state is already represented by tool results and
// patches, so the final response only needs to state outcome and evidence.
type agentGoalOutput struct {
	Status          string   `json:"status" desc:"completed when the goal is satisfied, blocked when it cannot proceed"`
	Summary         string   `json:"summary" desc:"Concise outcome summary in the project language"`
	SkillsUsed      []string `json:"skills_used,omitempty" desc:"Skill names loaded and used"`
	ChangedTargets  []string `json:"changed_targets,omitempty" desc:"Project objects changed through validated patch tools"`
	Verification    []string `json:"verification,omitempty" desc:"Checks and evidence that prove the outcome"`
	RemainingIssues []string `json:"remaining_issues,omitempty" desc:"Concrete blockers when status is blocked"`
}

// RunAgentGoal runs one goal through the Agent SDK runtime. The model chooses
// skills and tools inside the session; Go validates the completed state and
// records usage.
func RunAgentGoal(ctx context.Context, opts AgentGoalOptions) (*AgentGoalReport, error) {
	goal := strings.TrimSpace(opts.Goal)
	if goal == "" {
		return nil, fmt.Errorf("agent goal is required")
	}
	if opts.Root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	if opts.MaxTurns <= 0 {
		opts.MaxTurns = defaultGoalAgentMaxTurns
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultGoalAgentTimeout
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultGoalAgentAttempts
	}

	allSkills, err := agentruntime.ListEmbeddedSkills()
	if err != nil {
		return nil, err
	}
	selectable := filterCoreSkill(allSkills)
	skills, err := agentruntime.SelectSkills(selectable, opts.AllowedSkills)
	if err != nil {
		return nil, err
	}
	allowedSkillNames := map[string]bool{"goal-agent-core": true}
	for _, skill := range skills {
		allowedSkillNames[skill.Name] = true
	}

	runtime := opts.Runtime
	if runtime == nil {
		runtimeConfig, err := agentruntime.LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("load agent runtime config: %w", err)
		}
		runtime, err = runtimeConfig.NewRuntime(opts.RuntimeName)
		if err != nil {
			return nil, fmt.Errorf("create agent runtime: %w", err)
		}
	}

	logger.Section("GOAL AGENT")
	logger.Info("Goal: %s", goal)
	logger.Info("Skills available: %d", len(skills))
	logger.Info("Write permission: %t", opts.AllowWrite)

	initialPlan := loadGoalPlan(opts.Root, opts.ResumeSession)
	var (
		lastReport  *AgentGoalReport
		lastErr     error
		aggregate   agentruntime.Usage
		toolEvents  []agentruntime.ToolEvent
		liveSummary *agentruntime.LiveSummary
		liveLogPath string
		lastModel   string
		sessionID   string
		planItems   []agentruntime.PlanItem
		planUpdated bool
		previous    string
	)
	planItems = append([]agentruntime.PlanItem(nil), initialPlan...)
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		allowWrite := opts.AllowWrite && attempt == 1
		output := agentGoalOutput{}
		invocation := agentruntime.Invocation{
			AgentName:           "GoalAgent",
			Command:             "complete the supplied user goal autonomously",
			SDKSkills:           []string{"goal-agent-core"},
			Language:            opts.Language,
			WorkspaceRoot:       opts.Root.Dir(),
			Tools:               []string{"Bash"},
			AllowedTools:        []string{"Bash"},
			PermissionMode:      "dontAsk",
			RequireSDK:          true,
			ToolAllowlist:       goalToolAllowlist(allowWrite),
			SystemPrompt:        buildGoalSystemPrompt(skills, allowWrite),
			UserPrompt:          buildGoalUserPrompt(goal, allowWrite, opts.Language),
			UserPromptDriven:    true,
			NativeTools:         &agentruntime.NativeToolPolicy{Enabled: true, AllowWrite: allowWrite},
			ResumeSessionID:     "",
			InitialPlan:         initialPlan,
			OutputSchemaText:    utils.StructToJSONSchema(agentGoalOutput{}, "  "),
			OutputJSONSchema:    utils.StructToJSONSchemaObject(agentGoalOutput{}),
			CompactOutputSchema: true,
			ToolEvidence:        agentruntime.ToolEvidence{RequireNoDeniedTools: true},
			Options: agentruntime.Options{
				Model:    strings.TrimSpace(opts.Model),
				MaxTurns: opts.MaxTurns,
				Timeout:  opts.Timeout,
			},
			Metadata: map[string]string{
				"mode":        "goal-agent",
				"allow_write": fmt.Sprintf("%t", allowWrite),
				"attempt":     fmt.Sprintf("%d", attempt),
			},
		}
		if attempt == 1 {
			invocation.ResumeSessionID = strings.TrimSpace(opts.ResumeSession)
		} else {
			invocation.InitialPlan = planItems
		}
		if attempt > 1 {
			invocation.UserPrompt += "\n\n" + previous
		}

		startedAt := time.Now()
		result, invokeErr := runtime.Invoke(ctx, invocation)
		elapsed := time.Since(startedAt)
		if invokeErr != nil {
			recordGoalUsage(opts.Root, result, elapsed, invokeErr)
			return nil, fmt.Errorf("goal agent failed: %w", invokeErr)
		}
		accumulateGoalUsage(&aggregate, result)
		if err := decodeGoalOutput(result.Content, &output); err != nil {
			recordGoalUsage(opts.Root, result, elapsed, err)
			lastErr = fmt.Errorf("parse goal agent output: %w", err)
			previous = buildGoalRetryPrompt(result.Content, lastErr)
			if attempt < opts.MaxAttempts {
				continue
			}
			return nil, lastErr
		}
		toolEvents = append(toolEvents, result.ToolEvents...)
		liveSummary = mergeLiveSummary(liveSummary, result.LiveSummary)
		if result.LiveLogPath != "" {
			liveLogPath = result.LiveLogPath
		}
		if result.Model != "" {
			lastModel = result.Model
		}
		if result.SessionID != "" {
			sessionID = result.SessionID
		}
		if result.PlanUpdated || len(result.Plan) > 0 {
			planItems = result.Plan
		}
		if result.PlanUpdated {
			planUpdated = true
		}
		summary := liveSummary
		if structured := agentruntime.SummarizeToolEvents(toolEvents); structured != nil {
			summary = structured
		}
		report := &AgentGoalReport{
			Goal:            goal,
			Status:          normalizeGoalStatus(output.Status),
			Summary:         strings.TrimSpace(output.Summary),
			SkillsUsed:      trimStringSlice(output.SkillsUsed),
			ChangedTargets:  trimStringSlice(output.ChangedTargets),
			Verification:    trimStringSlice(output.Verification),
			RemainingIssues: trimStringSlice(output.RemainingIssues),
			AllowWrite:      opts.AllowWrite,
			Model:           lastModel,
			SessionID:       sessionID,
			Plan:            planItems,
			PlanUpdated:     planUpdated,
			Usage:           aggregate,
			LiveLogPath:     liveLogPath,
			ToolSummary:     summary,
		}
		lastReport = report
		if err := validateGoalReport(report, allowedSkillNames); err != nil {
			recordGoalUsage(opts.Root, result, elapsed, err)
			lastErr = err
			persistGoalPlan(opts.Root, report)
			previous = buildGoalRetryPrompt(result.Content, err)
			if attempt < opts.MaxAttempts {
				continue
			}
			return report, err
		}
		recordGoalUsage(opts.Root, result, elapsed, nil)
		persistGoalPlan(opts.Root, report)
		return report, nil
	}
	if lastErr != nil {
		return lastReport, lastErr
	}
	return lastReport, fmt.Errorf("goal agent did not complete")
}

func loadGoalPlan(root *store.Root, sessionID string) []agentruntime.PlanItem {
	sessionID = strings.TrimSpace(sessionID)
	if root == nil || sessionID == "" {
		return nil
	}
	var plan []agentruntime.PlanItem
	if err := root.ReadJSON(root.AgentGoalPlanFile(sessionID), &plan); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("Failed to load goal plan for session %s: %v", sessionID, err)
		}
		return nil
	}
	return plan
}

func persistGoalPlan(root *store.Root, report *AgentGoalReport) {
	if root == nil || report == nil || !report.PlanUpdated {
		return
	}
	sessionID := strings.TrimSpace(report.SessionID)
	if sessionID == "" {
		return
	}
	if err := root.WriteJSON(root.AgentGoalPlanFile(sessionID), report.Plan); err != nil {
		logger.Warn("Failed to persist goal plan for session %s: %v", sessionID, err)
	}
}

func filterCoreSkill(skills []agentruntime.SkillInfo) []agentruntime.SkillInfo {
	out := make([]agentruntime.SkillInfo, 0, len(skills))
	for _, skill := range skills {
		if skill.Name == "goal-agent-core" {
			continue
		}
		out = append(out, skill)
	}
	return out
}

// goalToolAllowlist grants tool families, not one exact command. The runner
// keeps its shell-safety checks and mutation tools still validate every patch.
func goalToolAllowlist(allowWrite bool) []string {
	allowed := []string{
		"novelgen tool skill",
		"novelgen tool query",
		"novelgen tool check",
	}
	if !allowWrite {
		return allowed
	}
	return append(allowed,
		"novelgen tool patch outline",
		"novelgen tool patch craft",
		"novelgen tool patch setup",
		"novelgen tool patch recap",
		"novelgen tool patch chapter",
		"novelgen tool patch-buffer",
		"novelgen tool refresh chapter-dsl",
		"novelgen tool patch outline --apply",
		"novelgen tool patch craft --apply",
		"novelgen tool patch setup --apply",
		"novelgen tool patch recap --apply",
		"novelgen tool patch chapter --apply",
		"novelgen tool patch chapter --apply --refresh-derived",
	)
}

func buildGoalSystemPrompt(skills []agentruntime.SkillInfo, allowWrite bool) string {
	writeRule := "Write permission is disabled. You may query, check, and load skills only. Do not request patch or refresh tools."
	if allowWrite {
		writeRule = "Write permission is enabled. Use patch tools only through the validated dry-run/apply contract, then run a follow-up check."
	}
	return fmt.Sprintf(`You are Novelgen GoalAgent. Work toward the supplied goal without waiting for a stage-by-stage script.

Available skills (load full instructions on demand):
%s

Tool policy:
- Prefer the native typed tools: `+"`plan_get`"+`, `+"`plan_update`"+`, `+"`skill_list`"+`, `+"`skill_read`"+`, `+"`project_query`"+`, `+"`project_check`"+`, `+"`project_patch`"+`, `+"`project_refresh`"+`.
- Use `+"`plan_update`"+` for multi-step goals so the visible todo list stays current.
- If typed tools are unavailable, fall back to `+"`novelgen tool ...`"+` commands.
- Project facts come from `+"`project_query`"+` and `+"`project_check`"+`.
- %s
- Never edit project files directly. Never use general shell, source search, tests, help probes, temp files, or redirection.
- Do not stop at the first clean check when the user goal still requires a concrete change.

Return only the final JSON object required by the output schema.`, agentruntime.FormatSkillCatalog(skills), writeRule)
}

func buildGoalUserPrompt(goal string, allowWrite bool, language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		language = "the project's configured language"
	}
	return fmt.Sprintf(`Goal:
%s

Write permission: %t
Response language: %s

Choose skills and tools yourself. Establish facts before changing state. If the goal is complete, return status=completed with the concrete verification evidence. If you cannot finish, return status=blocked with remaining_issues.`, goal, allowWrite, language)
}

func buildGoalRetryPrompt(previousContent string, previousErr error) string {
	reason := "the previous response did not pass completion validation"
	if previousErr != nil {
		reason = previousErr.Error()
	}
	return fmt.Sprintf(`The previous goal attempt did not pass completion validation:
%s

Previous response:
%s

This retry is read-only. Inspect the current project state, run any missing verification checks, and return a corrected final JSON report. Do not repeat writes; do not claim a change that is not already present in project state.`, reason, strings.TrimSpace(previousContent))
}

func decodeGoalOutput(content string, output *agentGoalOutput) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return errors.New("empty response")
	}
	if err := json.Unmarshal([]byte(content), output); err == nil {
		return nil
	}
	start := strings.Index(content, "{")
	if start < 0 {
		return fmt.Errorf("response does not contain a JSON object")
	}
	decoder := json.NewDecoder(strings.NewReader(content[start:]))
	if err := decoder.Decode(output); err != nil {
		return err
	}
	return nil
}

func normalizeGoalStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "complete":
		return "completed"
	case "failed", "unable":
		return "blocked"
	default:
		return status
	}
}

func validateGoalReport(report *AgentGoalReport, allowedSkills map[string]bool) error {
	if report == nil {
		return fmt.Errorf("goal report is required")
	}
	switch report.Status {
	case "completed":
		if report.Summary == "" {
			return fmt.Errorf("goal agent reported completion without a summary")
		}
		if len(report.RemainingIssues) > 0 {
			return fmt.Errorf("goal agent reported completion with remaining issues: %s", strings.Join(report.RemainingIssues, " | "))
		}
	case "blocked":
		if len(report.RemainingIssues) == 0 {
			return fmt.Errorf("goal agent reported blocked without remaining_issues")
		}
	default:
		return fmt.Errorf("goal agent returned unsupported status %q", report.Status)
	}
	for _, skill := range report.SkillsUsed {
		if !allowedSkills[skill] {
			return fmt.Errorf("goal agent reported unknown or disallowed skill %q", skill)
		}
	}
	if !report.AllowWrite && len(report.ChangedTargets) > 0 {
		return fmt.Errorf("goal agent reported changed targets while write permission was disabled")
	}
	if report.Status == "blocked" {
		return nil
	}
	if len(report.ChangedTargets) > 0 && (report.ToolSummary == nil || report.ToolSummary.PatchApplies == 0) {
		return fmt.Errorf("goal agent reported changed targets without a validated patch apply")
	}
	if report.ToolSummary == nil || report.ToolSummary.ToolAllowed == 0 {
		return fmt.Errorf("goal agent completed without using any project tool")
	}
	if report.ToolSummary.PatchApplies > 0 && report.ToolSummary.CheckCalls == 0 {
		return fmt.Errorf("goal agent applied a patch without running a follow-up check")
	}
	if report.ToolSummary.ApplyWithoutFollowupCheck > 0 {
		return fmt.Errorf("goal agent applied a patch without a matching follow-up check")
	}
	return nil
}

func trimStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
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

func recordGoalUsage(root *store.Root, result *agentruntime.Result, elapsed time.Duration, runErr error) {
	if root == nil {
		return
	}
	entry := usage.Entry{
		Time:       time.Now(),
		Agent:      "GoalAgent",
		Command:    "agent run",
		Kind:       "agent-sdk",
		DurationMS: elapsed.Milliseconds(),
		Success:    runErr == nil,
	}
	if result != nil {
		entry.Model = result.Model
		entry.PromptTokens = result.Usage.PromptTokens
		entry.CompletionTokens = result.Usage.CompletionTokens
		entry.TotalTokens = result.Usage.TotalTokens
	}
	if runErr != nil {
		entry.Error = runErr.Error()
	}
	if err := root.AppendUsage(entry); err != nil {
		logger.Warn("Failed to record goal agent usage: %v", err)
	}
}

func accumulateGoalUsage(total *agentruntime.Usage, result *agentruntime.Result) {
	if total == nil || result == nil {
		return
	}
	total.PromptTokens += result.Usage.PromptTokens
	total.CompletionTokens += result.Usage.CompletionTokens
	total.TotalTokens += result.Usage.TotalTokens
}

func mergeLiveSummary(left, right *agentruntime.LiveSummary) *agentruntime.LiveSummary {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	merged := *left
	merged.ToolCalls += right.ToolCalls
	merged.ToolAllowed += right.ToolAllowed
	merged.ToolDenied += right.ToolDenied
	merged.QueryCalls += right.QueryCalls
	merged.QueryIndexCalls += right.QueryIndexCalls
	merged.QueryBriefCalls += right.QueryBriefCalls
	merged.QueryFullCalls += right.QueryFullCalls
	merged.ContextQueryCalls += right.ContextQueryCalls
	merged.CheckCalls += right.CheckCalls
	merged.RefreshCalls += right.RefreshCalls
	merged.PatchCalls += right.PatchCalls
	merged.PatchApplies += right.PatchApplies
	merged.ToolDurationMS += right.ToolDurationMS
	merged.ApplyWithoutFollowupCheck += right.ApplyWithoutFollowupCheck
	merged.AllowedToolCommands = appendUniqueStrings(merged.AllowedToolCommands, right.AllowedToolCommands...)
	merged.DeniedToolCommands = appendUniqueStrings(merged.DeniedToolCommands, right.DeniedToolCommands...)
	merged.WorkflowDeniedToolCommands = appendUniqueStrings(merged.WorkflowDeniedToolCommands, right.WorkflowDeniedToolCommands...)
	merged.DenialsResolved = right.DenialsResolved
	return &merged
}

func appendUniqueStrings(existing []string, values ...string) []string {
	seen := make(map[string]bool, len(existing))
	for _, value := range existing {
		seen[value] = true
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		existing = append(existing, value)
	}
	return existing
}
