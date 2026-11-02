package base

import (
	"context"
	"fmt"
	"strings"
	"time"

	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/runmetrics"
)

func (a *BaseAgent) currentRun(ctx context.Context) *runmetrics.Run {
	return runmetrics.FromContext(ctx)
}

// recordRunMetrics projects one model call and its runtime tool events into the
// run-metrics ledger. The helper is intentionally best-effort: observability
// must never turn a creative workflow into a failed workflow.
func (a *BaseAgent) recordRunMetrics(
	ctx context.Context,
	params InvokeParams,
	systemPrompt, userPrompt, model, callKind string,
	response string,
	usage llm.Usage,
	runtimeResult *agentruntime.Result,
	startedAt time.Time,
	callErr error,
) {
	run := runmetrics.FromContext(ctx)
	if run == nil {
		return
	}

	status := "ok"
	if callErr != nil {
		status = "error"
	}
	spanID := run.NewSpanID(a.name)
	requestedSkills := append([]string(nil), params.Skills...)
	requestedSkills = append(requestedSkills, params.SDKSkills...)
	loadedSkills := []string(nil)
	missingSkills := []string(nil)
	if runtimeResult != nil && runtimeResult.LiveSummary != nil {
		loadedSkills = append(loadedSkills, runtimeResult.LiveSummary.LoadedSDKSkills...)
		missingSkills = append(missingSkills, runtimeResult.LiveSummary.MissingSDKSkills...)
	}

	if len(requestedSkills) > 0 || len(loadedSkills) > 0 || len(missingSkills) > 0 {
		_ = run.Emit(runmetrics.Event{
			SpanID:        spanID,
			Event:         runmetrics.EventSkillLoad,
			Agent:         a.name,
			Skills:        requestedSkills,
			LoadedSkills:  loadedSkills,
			MissingSkills: missingSkills,
		})
	}

	if runtimeResult != nil {
		if len(runtimeResult.ToolEvents) > 0 {
			for _, tool := range runtimeResult.ToolEvents {
				toolStatus := "ok"
				var allowedPtr *bool
				if tool.Source == "post" {
					toolStatus = "completed"
				} else {
					allowed := tool.Allowed
					allowedPtr = &allowed
				}
				if allowedPtr != nil && !*allowedPtr {
					toolStatus = "denied"
				}
				_ = run.Emit(runmetrics.Event{
					SpanID:     spanID,
					Event:      runmetrics.EventTool,
					Status:     toolStatus,
					Agent:      a.name,
					Kind:       tool.Source,
					Command:    tool.Command,
					ToolName:   tool.ToolName,
					Allowed:    allowedPtr,
					Reason:     tool.Reason,
					DurationMS: int64(tool.DurationMS),
					Metadata: map[string]string{
						"hook":        tool.Hook,
						"patch_key":   tool.PatchKey,
						"patch_apply": fmt.Sprintf("%t", tool.PatchApply),
					},
				})
			}
		} else if runtimeResult.LiveSummary != nil {
			emitLiveSummaryTools(run, spanID, a.name, runtimeResult.LiveSummary)
		}
	}

	event := runmetrics.Event{
		SpanID:            spanID,
		Event:             runmetrics.EventAgentCall,
		Status:            status,
		Agent:             a.name,
		Model:             model,
		Kind:              callKind,
		Command:           params.Command,
		Skills:            requestedSkills,
		LoadedSkills:      loadedSkills,
		Error:             errorText(callErr),
		DurationMS:        time.Since(startedAt).Milliseconds(),
		PromptTokens:      usage.PromptTokens,
		CompletionTokens:  usage.CompletionTokens,
		TotalTokens:       usage.TotalTokens,
		SystemPromptChars: len([]rune(systemPrompt)),
		PromptChars:       len([]rune(userPrompt)),
		ResponseChars:     len([]rune(response)),
		LiveLogPath:       runtimeLiveLog(runtimeResult),
	}
	if a.noRecapContext {
		event.Metadata = map[string]string{"no_recap_context": "true"}
	}
	_ = run.Emit(event)

	if runtimeResult == nil {
		if callErr != nil {
			emitStuck(run, spanID, a.name, classifyCallError(callErr))
		}
		return
	}
	summary := runtimeResult.LiveSummary
	if summary == nil {
		summary = agentruntime.SummarizeToolEvents(runtimeResult.ToolEvents)
	}
	if summary == nil {
		return
	}
	if callErr != nil {
		emitStuck(run, spanID, a.name, classifyCallError(callErr))
	}
	if summary.ToolDenied > 0 {
		emitStuck(run, spanID, a.name, fmt.Sprintf("tool_denied (%d)", summary.ToolDenied))
	}
	if summary.ApplyWithoutFollowupCheck > 0 {
		emitStuck(run, spanID, a.name, fmt.Sprintf("patch_apply_without_followup_check (%d)", summary.ApplyWithoutFollowupCheck))
	}
	if len(summary.MissingSDKSkills) > 0 {
		emitStuck(run, spanID, a.name, "missing_skills: "+strings.Join(summary.MissingSDKSkills, ","))
	}
	for command, count := range repeatedAllowedCommands(summary.AllowedToolCommands) {
		if count >= 3 {
			emitStuck(run, spanID, a.name, fmt.Sprintf("repeated_command:%s (%d)", command, count))
		}
	}
}

func emitLiveSummaryTools(run *runmetrics.Run, spanID, agent string, summary *agentruntime.LiveSummary) {
	if summary == nil {
		return
	}
	allowed := true
	for _, command := range summary.AllowedToolCommands {
		_ = run.Emit(runmetrics.Event{
			SpanID:  spanID,
			Event:   runmetrics.EventTool,
			Status:  "ok",
			Agent:   agent,
			Kind:    "live-summary",
			Command: command,
			Allowed: &allowed,
		})
	}
	denied := false
	for _, command := range summary.DeniedToolCommands {
		_ = run.Emit(runmetrics.Event{
			SpanID:  spanID,
			Event:   runmetrics.EventTool,
			Status:  "denied",
			Agent:   agent,
			Kind:    "live-summary",
			Command: command,
			Allowed: &denied,
		})
	}
	for _, command := range summary.WorkflowDeniedToolCommands {
		_ = run.Emit(runmetrics.Event{
			SpanID:  spanID,
			Event:   runmetrics.EventTool,
			Status:  "denied",
			Agent:   agent,
			Kind:    "workflow-guard",
			Command: command,
			Allowed: &denied,
		})
	}
}

func emitStuck(run *runmetrics.Run, spanID, agent, reason string) {
	_ = run.Emit(runmetrics.Event{
		SpanID: spanID,
		Event:  runmetrics.EventStuck,
		Agent:  agent,
		Reason: reason,
	})
}

func repeatedAllowedCommands(commands []string) map[string]int {
	counts := make(map[string]int)
	for _, command := range commands {
		normalized := runmetrics.NormalizeCommand(command)
		if normalized != "" {
			counts[normalized]++
		}
	}
	return counts
}

func classifyCallError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "timeout"), strings.Contains(text, "deadline"):
		return "timeout"
	case strings.Contains(text, "max turns"), strings.Contains(text, "max_turns"):
		return "max_turns"
	case strings.Contains(text, "empty content"):
		return "empty_response"
	case strings.Contains(text, "denied"):
		return "tool_denied"
	default:
		return "agent_call_error"
	}
}

func runtimeLiveLog(result *agentruntime.Result) string {
	if result == nil {
		return ""
	}
	return result.LiveLogPath
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
