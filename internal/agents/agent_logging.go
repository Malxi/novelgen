package agents

import (
	"fmt"
	"novelgen/internal/agentruntime"
	"novelgen/internal/envutil"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/store"
	"novelgen/internal/usage"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func logAgentRuntimeLiveSummary(agentName string, result *agentruntime.Result) {
	if result == nil || result.LiveSummary == nil {
		return
	}
	summary := result.LiveSummary
	logger.Info("[%s] Agent live summary: model=%s final_model=%s query=%d index=%d brief=%d full=%d context=%d check=%d refresh=%d patch=%d tools=%d allowed=%d denied=%d messages=%d log=%s",
		agentName,
		summary.Model,
		summary.FinalModel,
		summary.QueryCalls,
		summary.QueryIndexCalls,
		summary.QueryBriefCalls,
		summary.QueryFullCalls,
		summary.ContextQueryCalls,
		summary.CheckCalls,
		summary.RefreshCalls,
		summary.PatchCalls,
		summary.ToolCalls,
		summary.ToolAllowed,
		summary.ToolDenied,
		summary.Messages,
		result.LiveLogPath,
	)
	if summary.PatchApplies > 0 {
		logger.Info("[%s] Agent patch apply summary: apply=%d missing_followup_check=%d", agentName, summary.PatchApplies, summary.ApplyWithoutFollowupCheck)
	}
	if len(summary.SDKSkills) > 0 || len(summary.LoadedSDKSkills) > 0 || len(summary.MissingSDKSkills) > 0 {
		logger.Info("[%s] Agent SDK skills: requested=%s loaded=%s missing=%s prompt_chars=%d",
			agentName,
			strings.Join(summary.SDKSkills, ","),
			strings.Join(summary.LoadedSDKSkills, ","),
			strings.Join(summary.MissingSDKSkills, ","),
			summary.SDKSkillPromptChars,
		)
	}
	if summary.SlowestToolDurationMS > 0 {
		logger.Info("[%s] Agent slowest tool: duration_ms=%d command=%s", agentName, summary.SlowestToolDurationMS, summary.SlowestToolCommand)
	}
	if len(summary.WorkflowDeniedToolCommands) > 0 {
		logger.Info("[%s] Agent workflow guard blocked tool commands: %s", agentName, strings.Join(summary.WorkflowDeniedToolCommands, " | "))
	}
	if len(summary.DeniedToolCommands) > 0 {
		if summary.DenialsResolved {
			logger.Info("[%s] Agent denied tool commands (corrected in-turn): %s", agentName, strings.Join(summary.DeniedToolCommands, " | "))
		} else {
			logger.Warn("[%s] Agent denied tool commands: %s", agentName, strings.Join(summary.DeniedToolCommands, " | "))
		}
	}
	if len(summary.HookErrors) > 0 {
		logger.Warn("[%s] Agent hook errors: %s", agentName, strings.Join(summary.HookErrors, " | "))
	}
	if len(summary.MissingSDKSkills) > 0 {
		logger.Warn("[%s] Agent missing SDK skills: %s", agentName, strings.Join(summary.MissingSDKSkills, ","))
	}
	if summary.ApplyWithoutFollowupCheck > 0 {
		logger.Warn("[%s] Agent applied patches without a later tool check; Go post-save validation will still run when available", agentName)
	}
}

// savePromptsToFile saves the prompts to a file for debugging
func (a *BaseAgent) savePromptsToFile(agentName, systemPrompt, userPrompt string) error {
	// Build content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Agent: %s\n", agentName))
	content.WriteString(fmt.Sprintf("# Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("---\n\n")
	content.WriteString("# SYSTEM PROMPT\n\n")
	content.WriteString(systemPrompt)
	content.WriteString("\n\n---\n\n")
	content.WriteString("# USER PROMPT\n\n")
	content.WriteString(userPrompt)

	path, err := writeAgentLogFile("prompts", agentName, content.String())
	if err != nil {
		return fmt.Errorf("failed to write prompts file: %w", err)
	}
	if path != "" {
		logger.Info("[%s] Prompt log: %s", a.name, path)
	}
	return nil
}

// saveResponseToFile saves the AI response to a file for debugging
func (a *BaseAgent) saveResponseToFile(agentName, response string) error {
	// Build content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Agent: %s\n", agentName))
	content.WriteString(fmt.Sprintf("# Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("---\n\n")
	content.WriteString("# AI RESPONSE\n\n")
	content.WriteString("```\n")
	content.WriteString(response)
	content.WriteString("\n```\n")

	path, err := writeAgentLogFile("responses", agentName, content.String())
	if err != nil {
		return fmt.Errorf("failed to write response file: %w", err)
	}
	if path != "" {
		logger.Info("[%s] Response log: %s (%d chars)", a.name, path, len(response))
	}
	return nil
}

// writeAgentLogFile stores one agent log file and returns its path. Names are
// allocated with O_EXCL so concurrent workers writing in the same second can
// never overwrite each other.
func writeAgentLogFile(kind, agentName, content string) (string, error) {
	if agentLogsDisabled() {
		return "", nil
	}
	logsDir := agentLogDir(kind)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create logs directory: %w", err)
	}

	now := time.Now()
	stamp := now.Format("20060102_150405")
	candidates := []string{
		filepath.Join(logsDir, fmt.Sprintf("%s_%s.md", agentName, stamp)),
		// A single second is not enough resolution when best-of-N, craft
		// concurrency, or write workers fire at the same moment.
		filepath.Join(logsDir, fmt.Sprintf("%s_%s_%09d.md", agentName, stamp, now.Nanosecond())),
	}
	for attempt := 2; attempt < 64; attempt++ {
		candidates = append(candidates, filepath.Join(logsDir,
			fmt.Sprintf("%s_%s_%09d_%d.md", agentName, stamp, now.Nanosecond(), attempt)))
	}

	for _, path := range candidates {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		if _, err := file.WriteString(content); err != nil {
			_ = file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
		return path, nil
	}
	return "", fmt.Errorf("could not allocate a unique log file name in %s", logsDir)
}

func agentLogsDisabled() bool {
	return envutil.Bool(AgentLogsDisabledEnv, false)
}

// recordUsage appends one line to logs/usage.jsonl so a long creative run can
// be costed afterwards (novelgen project usage). Failures here must never break
// the creative pipeline, so they are only logged at debug level.
func (a *BaseAgent) recordUsage(params InvokeParams, kind, model string, tokens llm.Usage, elapsed time.Duration, callErr error) {
	entry := usage.Entry{
		Time:             time.Now(),
		Agent:            a.name,
		Command:          params.Command,
		Kind:             kind,
		Model:            model,
		PromptTokens:     tokens.PromptTokens,
		CompletionTokens: tokens.CompletionTokens,
		TotalTokens:      tokens.TotalTokens,
		DurationMS:       elapsed.Milliseconds(),
		Success:          callErr == nil,
	}
	if callErr != nil {
		entry.Error = callErr.Error()
	}
	root, err := store.Open(agentLogRoot())
	if err != nil {
		logger.Debug("[%s] Failed to record usage: %v", a.name, err)
		return
	}
	if err := root.AppendUsage(entry); err != nil {
		logger.Debug("[%s] Failed to record usage: %v", a.name, err)
	}
}

func agentLogRoot() string {
	if dir := strings.TrimSpace(logger.Default().ProjectDir()); dir != "" {
		return dir
	}
	if root, err := store.Default(); err == nil {
		return root.Dir()
	}
	return "."
}

// agentLogDir returns the directory holding one kind of agent log, using the
// project layout from internal/store so log locations stay defined in one place.
func agentLogDir(kind string) string {
	return store.At(agentLogRoot()).LogFile(kind)
}
