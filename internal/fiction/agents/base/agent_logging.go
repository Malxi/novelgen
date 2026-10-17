package base

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/envutil"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/usage"
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

	path, err := a.saveLogStream(store.LogPromptsName, agentName, content.String())
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

	path, err := a.saveLogStream(store.LogResponsesName, agentName, content.String())
	if err != nil {
		return fmt.Errorf("failed to write response file: %w", err)
	}
	if path != "" {
		logger.Info("[%s] Response log: %s (%d chars)", a.name, path, len(response))
	}
	return nil
}

// saveLogStream stores one agent log file and returns its path. Name
// allocation (O_EXCL plus a numeric suffix on collision) lives in internal/platform/store,
// so concurrent workers writing in the same second cannot overwrite each other.
func (a *BaseAgent) saveLogStream(kind, agentName, content string) (string, error) {
	if agentLogsDisabled() {
		return "", nil
	}
	root, err := a.logRoot()
	if err != nil {
		return "", fmt.Errorf("resolve agent log root: %w", err)
	}
	path, err := root.SaveLogStream(kind, agentName, ".md", []byte(content))
	if err != nil {
		return "", fmt.Errorf("failed to write %s log: %w", kind, err)
	}
	return path, nil
}

func agentLogsDisabled() bool {
	return envutil.Bool(AgentLogsDisabledEnv, false)
}

func (a *BaseAgent) logRoot() (*store.Root, error) {
	if a != nil && a.project != nil && a.project.Dir() != "" {
		return a.project, nil
	}
	return store.Open(agentLogRoot())
}

// LogRoot returns the project root used for agent log files. Stage packages use
// it when they need to append stage-specific review artifacts beside the
// prompt/response logs.
func (a *BaseAgent) LogRoot() (*store.Root, error) {
	return a.logRoot()
}

// aiCallRecord is one AI call as the web UI's AI-call viewer reads it from
// logs/ai_calls/<id>.json. webui cannot import this package, so these JSON tags
// are a cross-module contract: internal/architecture has a test that fails when
// this struct and the reader in webui/main.go drift apart.
type aiCallRecord struct {
	// ID stays empty on purpose. The reader derives a call's identity from its
	// file name, which is also what keeps it unique when two calls land in the
	// same second.
	ID           string      `json:"id"`
	Agent        string      `json:"agent"`
	Command      string      `json:"command"`
	Skills       []string    `json:"skills"`
	Model        string      `json:"model"`
	StartedAt    time.Time   `json:"started_at"`
	CompletedAt  time.Time   `json:"completed_at"`
	SystemPrompt string      `json:"system_prompt"`
	UserPrompt   string      `json:"user_prompt"`
	Response     string      `json:"response"`
	Usage        aiCallUsage `json:"usage"`
}

// aiCallUsage is the nested usage object of the same record.
type aiCallUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// recordAICall writes one structured record for a finished AI call: the model,
// both prompts, the response and the token usage the markdown prompt/response
// logs next to it do not carry. Failed calls are recorded too (with an empty
// response), because "did the call fail, and with what prompt" is exactly what
// those logs get opened to answer. It never fails a run.
func (a *BaseAgent) recordAICall(params InvokeParams, model, systemPrompt, userPrompt, response string, tokens llm.Usage, startedAt time.Time) {
	if agentLogsDisabled() {
		return
	}
	root, err := a.logRoot()
	if err != nil {
		logger.Debug("[%s] Failed to record AI call: %v", a.name, err)
		return
	}
	record := aiCallRecord{
		Agent:        a.name,
		Command:      params.Command,
		Skills:       params.Skills,
		Model:        model,
		StartedAt:    startedAt,
		CompletedAt:  time.Now(),
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Response:     response,
		Usage: aiCallUsage{
			PromptTokens:     tokens.PromptTokens,
			CompletionTokens: tokens.CompletionTokens,
			TotalTokens:      tokens.TotalTokens,
		},
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		logger.Debug("[%s] Failed to encode AI call record: %v", a.name, err)
		return
	}
	if _, err := root.SaveLogStream(store.LogAICallsName, a.name, ".json", append(data, '\n')); err != nil {
		logger.Debug("[%s] Failed to write AI call record: %v", a.name, err)
	}
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
	root, err := a.logRoot()
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
