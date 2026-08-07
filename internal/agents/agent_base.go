package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/utils"
)

// BaseAgent is the base struct for all agents.
// It loads prompts from skill files and provides common LLM interaction logic.
type BaseAgent struct {
	name          string
	client        llm.Client
	runtime       agentruntime.Runtime
	config        *llm.Config
	projectLLM    *models.ProjectLLM
	skillLoader   *SkillLoader
	language      string
	modelOverride string
}

// BaseAgentConfig holds configuration for creating a BaseAgent
type BaseAgentConfig struct {
	Name       string
	Client     llm.Client
	Runtime    agentruntime.Runtime
	Config     *llm.Config
	ProjectLLM *models.ProjectLLM
	Language   string
}

// NewBaseAgent creates a new BaseAgent
func NewBaseAgent(cfg BaseAgentConfig) *BaseAgent {
	// Get the directory of the current file to locate skills
	_, filename, _, _ := runtime.Caller(0)
	skillsDir := filepath.Join(filepath.Dir(filename), "skills")

	return &BaseAgent{
		name:        cfg.Name,
		client:      cfg.Client,
		runtime:     resolveRuntime(cfg.ProjectLLM, cfg.Runtime, cfg.Client),
		config:      cfg.Config,
		projectLLM:  cfg.ProjectLLM,
		skillLoader: NewSkillLoader(skillsDir),
		language:    cfg.Language,
	}
}

// SetLanguage sets the output language
func (a *BaseAgent) SetLanguage(language string) {
	a.language = language
}

// SetModelOverride overrides the project model for this agent's chat options.
// When the model exists in the active provider's llm_config, its max_tokens
// and temperature are applied as well; otherwise only the model name changes.
func (a *BaseAgent) SetModelOverride(model string) {
	a.modelOverride = strings.TrimSpace(model)
}

// InvokeParams holds the parameters for invoking an agent
type InvokeParams struct {
	Skills                 []string
	SDKSkills              []string
	Tools                  []string
	AllowedTools           []string
	PermissionMode         string
	RequireSDK             bool
	ToolAllowlist          []string
	ToolEvidence           ToolEvidenceRequirement
	MaxTurns               int
	Timeout                int
	CompactOutputSchema    bool
	DisableSDKOutputFormat bool
	Command                string
}

// ToolEvidenceRequirement asserts minimum tool activity observed in the Agent
// SDK live log. It lets Go reject successful-looking JSON when the agent skipped
// required tool steps.
type ToolEvidenceRequirement struct {
	MinQueryCalls                  int
	MinContextQueryCalls           int
	MinCheckCalls                  int
	MinPatchApplyCalls             int
	MaxQueryCalls                  int
	MaxContextQueryCalls           int
	DisallowQueryBriefCalls        bool
	DisallowQueryFullCalls         bool
	RequirePatchApplyFollowupCheck bool
	RequireNoDeniedTools           bool
	RequiredToolCommands           []string
}

// Execute sends the input to AI and returns the result
// This is the core method that all agents use
func (a *BaseAgent) Execute(ctx context.Context, params InvokeParams, input interface{}, output interface{}) error {
	_, err := a.ExecuteWithRuntimeResult(ctx, params, input, output)
	return err
}

// ExecuteWithRuntimeResult sends the input to AI and returns the raw runtime
// result when an Agent SDK runtime was used. Ordinary chat-client executions
// return nil for the runtime result.
func (a *BaseAgent) ExecuteWithRuntimeResult(ctx context.Context, params InvokeParams, input interface{}, output interface{}) (*agentruntime.Result, error) {
	// Use provided params or fall back to agent defaults
	// Load system prompt from skill files
	skillPrompt, err := a.loadSystemPromptWithSkills(params.Skills)
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %w", err)
	}

	// Build user prompt from input
	userPrompt := fmt.Sprintf("Follow skill to %s for the input:\n %s",
		params.Command, utils.StructToMarkdown(input, 0))

	// Add output requirements
	outputRequirements := a.buildOutputRequirements(output)
	systemPrompt := skillPrompt + outputRequirements

	// Log prompts
	logger.Prompt(a.name, "default", systemPrompt, userPrompt)

	// Save prompts to file for debugging
	if err := a.savePromptsToFile(a.name, systemPrompt, userPrompt); err != nil {
		logger.Debug("[%s] Failed to save prompts to file: %v", a.name, err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	options := a.chatOptions()

	logger.Info("[%s] Sending request to AI...", a.name)
	resp, runtimeResult, err := a.invokeAIWithRuntimeResult(ctx, params, messages, options, outputRequirements, output)
	if err != nil {
		logAgentRuntimeLiveSummary(a.name, runtimeResult)
		logger.Error("[%s] AI request failed: %v", a.name, err)
		return runtimeResult, fmt.Errorf("AI request failed: %w", err)
	}
	if err := validateAgentRuntimeToolEvidence(a.name, params.ToolEvidence, runtimeResult); err != nil {
		return runtimeResult, err
	}

	logger.Info("[%s] Received response (%d tokens used)", a.name, resp.Usage.TotalTokens)

	// Save response to file for debugging
	if err := a.saveResponseToFile(a.name, resp.Content); err != nil {
		logger.Debug("[%s] Failed to save response to file: %v", a.name, err)
	}

	// Parse response into output
	if err := a.parseResponse(resp.Content, output); err != nil {
		logger.Warn("[%s] Initial JSON parse failed; attempting JSON repair: %v", a.name, err)
		if repairErr := a.repairJSONResponse(ctx, resp.Content, output, err); repairErr != nil {
			return runtimeResult, fmt.Errorf("failed to parse response: %w; JSON repair also failed: %v", err, repairErr)
		}
		logger.Info("[%s] JSON repair succeeded", a.name)
	}

	return runtimeResult, nil
}

func (a *BaseAgent) repairJSONResponse(ctx context.Context, malformed string, output interface{}, parseErr error) error {
	if strings.TrimSpace(malformed) == "" {
		return fmt.Errorf("cannot repair empty AI response")
	}

	repairOptions := *a.chatOptions()
	repairOptions.Temperature = 0.1

	schema := a.buildOutputRequirements(output)
	systemPrompt := `You are a JSON repair tool.
Return ONLY valid JSON.
Preserve the original semantic content and field values as much as possible.
Do not add markdown fences, explanations, comments, or new creative content.
The repaired JSON must match the requested structure.`

	userPrompt := fmt.Sprintf("The previous response failed JSON parsing.\n\nParse error:\n%s\n\n%s\n\nMalformed response:\n%s",
		parseErr.Error(), schema, malformed)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	logger.Info("[%s] Sending malformed JSON to repair pass...", a.name)
	resp, err := a.invokeAI(ctx, InvokeParams{Command: "repair malformed JSON"}, messages, &repairOptions, schema, output)
	if err != nil {
		return fmt.Errorf("JSON repair request failed: %w", err)
	}
	logger.Info("[%s] JSON repair response received (%d tokens used)", a.name, resp.Usage.TotalTokens)
	if err := a.saveResponseToFile(a.name+"_JSONRepair", resp.Content); err != nil {
		logger.Debug("[%s] Failed to save JSON repair response: %v", a.name, err)
	}

	if err := a.parseResponse(resp.Content, output); err != nil {
		return fmt.Errorf("repaired response still invalid: %w", err)
	}
	return nil
}

func (a *BaseAgent) invokeAI(ctx context.Context, params InvokeParams, messages []llm.Message, options *llm.ChatOptions, outputSchemaText string, output interface{}) (*llm.ChatResponse, error) {
	resp, _, err := a.invokeAIWithRuntimeResult(ctx, params, messages, options, outputSchemaText, output)
	return resp, err
}

func (a *BaseAgent) invokeAIWithRuntimeResult(ctx context.Context, params InvokeParams, messages []llm.Message, options *llm.ChatOptions, outputSchemaText string, output interface{}) (*llm.ChatResponse, *agentruntime.Result, error) {
	if options == nil {
		options = &llm.ChatOptions{}
	}
	runtime := a.runtime
	if runtime == nil && params.RequireSDK {
		requiredRuntime, err := loadRequiredAgentRuntime()
		if err != nil {
			return nil, nil, err
		}
		runtime = requiredRuntime
	}
	if runtime != nil {
		invocation := agentruntime.Invocation{
			AgentName:              a.name,
			Command:                params.Command,
			Skills:                 append([]string(nil), params.Skills...),
			SDKSkills:              append([]string(nil), params.SDKSkills...),
			Language:               utils.GetLanguageName(a.language),
			WorkspaceRoot:          agentWorkspaceRoot(),
			Tools:                  append([]string(nil), params.Tools...),
			AllowedTools:           append([]string(nil), params.AllowedTools...),
			PermissionMode:         params.PermissionMode,
			RequireSDK:             params.RequireSDK,
			ToolAllowlist:          append([]string(nil), params.ToolAllowlist...),
			SystemPrompt:           messageContent(messages, "system"),
			UserPrompt:             messageContent(messages, "user"),
			OutputSchemaText:       outputSchemaText,
			OutputJSONSchema:       utils.StructToJSONSchemaObject(output),
			CompactOutputSchema:    params.CompactOutputSchema,
			DisableSDKOutputFormat: params.DisableSDKOutputFormat,
			ToolEvidence:           agentEvidenceConfig(params.ToolEvidence),
			Options: agentruntime.Options{
				Model:       agentRuntimeModel(params, options.Model),
				Temperature: options.Temperature,
				MaxTokens:   options.MaxTokens,
				MaxTurns:    params.MaxTurns,
				Timeout:     params.Timeout,
			},
		}
		result, err := runtime.Invoke(ctx, invocation)
		if err != nil {
			return nil, result, err
		}
		logAgentRuntimeLiveSummary(a.name, result)
		return &llm.ChatResponse{
			Content: result.Content,
			Model:   result.Model,
			Usage: llm.Usage{
				PromptTokens:     result.Usage.PromptTokens,
				CompletionTokens: result.Usage.CompletionTokens,
				TotalTokens:      result.Usage.TotalTokens,
			},
		}, result, nil
	}
	if params.RequireSDK {
		return nil, nil, fmt.Errorf("Claude Agent SDK runtime is required for this workflow")
	}
	if a.client == nil {
		return nil, nil, fmt.Errorf("no LLM client or agent runtime configured")
	}
	resp, err := a.client.ChatCompletion(ctx, messages, options)
	return resp, nil, err
}

// agentEvidenceConfig maps a Go-side evidence requirement into the runner
// contract so the runner can enforce it in-flight (before the agent stops).
func agentEvidenceConfig(req ToolEvidenceRequirement) agentruntime.ToolEvidence {
	return agentruntime.ToolEvidence{
		MinQueryCalls:        req.MinQueryCalls,
		MinContextQueryCalls: req.MinContextQueryCalls,
		MinCheckCalls:        req.MinCheckCalls,
		MinPatchApplyCalls:   req.MinPatchApplyCalls,
		RequiredToolCommands: append([]string(nil), req.RequiredToolCommands...),
		RequireNoDeniedTools: req.RequireNoDeniedTools,
	}
}

func validateAgentRuntimeToolEvidence(agentName string, requirement ToolEvidenceRequirement, result *agentruntime.Result) error {
	if requirement.MinQueryCalls <= 0 &&
		requirement.MinContextQueryCalls <= 0 &&
		requirement.MinCheckCalls <= 0 &&
		requirement.MinPatchApplyCalls <= 0 &&
		requirement.MaxQueryCalls <= 0 &&
		requirement.MaxContextQueryCalls <= 0 &&
		!requirement.DisallowQueryBriefCalls &&
		!requirement.DisallowQueryFullCalls &&
		!requirement.RequirePatchApplyFollowupCheck &&
		!requirement.RequireNoDeniedTools &&
		len(requirement.RequiredToolCommands) == 0 {
		return nil
	}
	if result == nil || result.LiveSummary == nil {
		return fmt.Errorf("%s Agent SDK live summary is required to verify tool execution", agentName)
	}
	summary := result.LiveSummary
	if summary.QueryCalls < requirement.MinQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: query calls=%d, want at least %d", agentName, summary.QueryCalls, requirement.MinQueryCalls)
	}
	if summary.ContextQueryCalls < requirement.MinContextQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: context query calls=%d, want at least %d", agentName, summary.ContextQueryCalls, requirement.MinContextQueryCalls)
	}
	if summary.CheckCalls < requirement.MinCheckCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: check calls=%d, want at least %d", agentName, summary.CheckCalls, requirement.MinCheckCalls)
	}
	if summary.PatchApplies < requirement.MinPatchApplyCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: patch apply calls=%d, want at least %d", agentName, summary.PatchApplies, requirement.MinPatchApplyCalls)
	}
	if requirement.MaxQueryCalls > 0 && summary.QueryCalls > requirement.MaxQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: query calls=%d, want at most %d", agentName, summary.QueryCalls, requirement.MaxQueryCalls)
	}
	if requirement.MaxContextQueryCalls > 0 && summary.ContextQueryCalls > requirement.MaxContextQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: context query calls=%d, want at most %d", agentName, summary.ContextQueryCalls, requirement.MaxContextQueryCalls)
	}
	if requirement.DisallowQueryBriefCalls && summary.QueryBriefCalls > 0 {
		return fmt.Errorf("%s Agent SDK tool evidence failed: brief query calls=%d, want 0", agentName, summary.QueryBriefCalls)
	}
	if requirement.DisallowQueryFullCalls && summary.QueryFullCalls > 0 {
		return fmt.Errorf("%s Agent SDK tool evidence failed: full query calls=%d, want 0", agentName, summary.QueryFullCalls)
	}
	if requirement.RequirePatchApplyFollowupCheck && summary.ApplyWithoutFollowupCheck > 0 {
		return fmt.Errorf("%s Agent SDK tool evidence failed: patch apply calls without follow-up check=%d", agentName, summary.ApplyWithoutFollowupCheck)
	}
	if requirement.RequireNoDeniedTools && summary.ToolDenied > 0 {
		blockingDenied := blockingDeniedToolCommands(summary, requirement)
		if len(blockingDenied) > 0 && !summary.DenialsResolved {
			return fmt.Errorf("%s Agent SDK tool evidence failed: denied tool calls=%d commands=%s", agentName, len(blockingDenied), strings.Join(blockingDenied, " | "))
		}
		if len(summary.DeniedToolCommands) == 0 && len(summary.WorkflowDeniedToolCommands) == 0 {
			return fmt.Errorf("%s Agent SDK tool evidence failed: denied tool calls=%d", agentName, summary.ToolDenied)
		}
	}
	for _, required := range requirement.RequiredToolCommands {
		if !liveSummaryHasToolCommand(summary, required) {
			return fmt.Errorf("%s Agent SDK tool evidence failed: required tool command not observed: %s", agentName, required)
		}
	}
	return nil
}

func liveSummaryHasToolCommand(summary *agentruntime.LiveSummary, required string) bool {
	if summary == nil {
		return false
	}
	required = normalizeToolEvidenceCommand(required)
	if required == "" {
		return true
	}
	for _, command := range summary.AllowedToolCommands {
		observed := normalizeToolEvidenceCommand(command)
		if observed == required || strings.Contains(observed, required) || strings.Contains(required, observed) {
			return true
		}
	}
	return false
}

func normalizeToolEvidenceCommand(command string) string {
	command = strings.ToLower(strings.TrimSpace(command))
	command = strings.ReplaceAll(command, `"`, "")
	command = strings.Join(strings.Fields(command), " ")
	return command
}

func blockingDeniedToolCommands(summary *agentruntime.LiveSummary, requirement ToolEvidenceRequirement) []string {
	if summary == nil {
		return nil
	}
	commands := append([]string(nil), summary.DeniedToolCommands...)
	if len(commands) == 0 {
		return nil
	}
	toleratePatchApplyRetry := summary.PatchApplies >= maxInt(1, requirement.MinPatchApplyCalls)
	if requirement.RequirePatchApplyFollowupCheck && summary.ApplyWithoutFollowupCheck > 0 {
		toleratePatchApplyRetry = false
	}
	blocking := make([]string, 0, len(commands))
	for _, command := range commands {
		if summaryHasWorkflowDeniedCommand(summary, command) {
			continue
		}
		if toleratePatchApplyRetry && (isDeniedPatchCommand(command) || isDeniedPatchBufferCommand(command) || isDeniedClaudeToolResultRead(command) || isDeniedPostApplyInspectionCommand(command) || isDeniedPostApplyContextExpansionCommand(command) || isDeniedPostApplyRefreshCommand(command)) {
			continue
		}
		blocking = append(blocking, command)
	}
	return blocking
}

func summaryHasWorkflowDeniedCommand(summary *agentruntime.LiveSummary, command string) bool {
	if summary == nil {
		return false
	}
	normalized := normalizeToolEvidenceCommand(command)
	for _, workflowCommand := range summary.WorkflowDeniedToolCommands {
		observed := normalizeToolEvidenceCommand(workflowCommand)
		if observed == normalized {
			return true
		}
	}
	return false
}

func isDeniedPatchCommand(command string) bool {
	normalized := strings.ToLower(strings.TrimSpace(command))
	return strings.Contains(normalized, " tool patch ")
}

func isDeniedPatchBufferCommand(command string) bool {
	normalized := strings.ToLower(strings.TrimSpace(command))
	return strings.Contains(normalized, " tool patch-buffer ")
}

func isDeniedClaudeToolResultRead(command string) bool {
	normalized := strings.ToLower(strings.TrimSpace(command))
	if (strings.HasPrefix(normalized, "type ") || strings.HasPrefix(normalized, "get-content ")) &&
		strings.Contains(normalized, "tool-results") {
		return true
	}
	if strings.Contains(normalized, "get-content") &&
		strings.Contains(normalized, `\temp\claude\`) &&
		strings.Contains(normalized, `\tasks\`) &&
		strings.Contains(normalized, ".output") {
		return true
	}
	return false
}

func isDeniedPostApplyInspectionCommand(command string) bool {
	normalized := normalizeToolEvidenceCommand(command)
	if strings.Contains(normalized, "novelgen tool query chapter") &&
		strings.Contains(normalized, "--content") {
		return true
	}
	return false
}

func isDeniedPostApplyContextExpansionCommand(command string) bool {
	normalized := normalizeToolEvidenceCommand(command)
	return strings.Contains(normalized, "novelgen tool query context --type chapter-repair") &&
		strings.Contains(normalized, "--view brief")
}

func isDeniedPostApplyRefreshCommand(command string) bool {
	normalized := normalizeToolEvidenceCommand(command)
	return strings.Contains(normalized, "novelgen tool refresh chapter-dsl")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func loadRequiredAgentRuntime() (agentruntime.Runtime, error) {
	cfg, err := agentruntime.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("Claude Agent SDK runtime is required for this workflow: %w", err)
	}
	runtime, err := cfg.NewRuntime("")
	if err != nil {
		return nil, fmt.Errorf("Claude Agent SDK runtime is required for this workflow: %w", err)
	}
	return runtime, nil
}

func agentRuntimeModel(params InvokeParams, model string) string {
	return strings.TrimSpace(model)
}

func messageContent(messages []llm.Message, role string) string {
	for _, message := range messages {
		if message.Role == role {
			return message.Content
		}
	}
	return ""
}

func agentWorkspaceRoot() string {
	if dir := strings.TrimSpace(logger.Default().ProjectDir()); dir != "" {
		return dir
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

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

func resolveRuntime(projectLLM *models.ProjectLLM, provided agentruntime.Runtime, client llm.Client) agentruntime.Runtime {
	if provided != nil {
		return provided
	}
	if carrier, ok := client.(interface{ Runtime() agentruntime.Runtime }); ok {
		if runtime := carrier.Runtime(); runtime != nil {
			return runtime
		}
	}
	provider := ""
	if projectLLM != nil {
		provider = strings.TrimSpace(strings.ToLower(projectLLM.Provider))
	}
	if provider != "claude" && provider != "claude_sdk" && provider != "agent" {
		return nil
	}
	cfg, err := agentruntime.LoadConfig()
	if err != nil {
		return nil
	}
	runtime, err := cfg.NewRuntime(provider)
	if err == nil {
		return runtime
	}
	runtime, err = cfg.NewRuntime(cfg.DefaultRuntime)
	if err != nil {
		return nil
	}
	_ = client
	return runtime
}

func (a *BaseAgent) chatOptions() *llm.ChatOptions {
	if a.config == nil {
		return &llm.ChatOptions{}
	}
	if a.projectLLM == nil {
		return &llm.ChatOptions{}
	}
	options := a.config.GetChatOptions(a.projectLLM)
	if options == nil {
		return &llm.ChatOptions{}
	}
	if strings.TrimSpace(a.modelOverride) != "" {
		options.Model = strings.TrimSpace(a.modelOverride)
		if provider := a.config.GetActiveProvider(a.projectLLM); provider != nil {
			if model, ok := provider.Models[options.Model]; ok && model != nil {
				options.MaxTokens = model.MaxTokens
				options.Temperature = float64(model.Temp)
			}
		}
	}
	return options
}

// loadSystemPromptWithSkills loads the system prompt from specified skills
// All skills are wrapped with skill markers
func (a *BaseAgent) loadSystemPromptWithSkills(skills []string) (string, error) {
	vars := map[string]string{
		"language": utils.GetLanguageName(a.language),
	}

	var result strings.Builder
	result.WriteString("=== SKILLS ===\n\n")

	for _, skillName := range skills {
		prompt, err := a.skillLoader.LoadWithVars(skillName, vars)
		if err != nil {
			return "", fmt.Errorf("failed to load skill %s: %w", skillName, err)
		}

		result.WriteString(fmt.Sprintf("=== SKILL: %s ===\n\n", skillName))
		result.WriteString(prompt)
		result.WriteString("\n\n")
	}

	result.WriteString("=== END of SKILLS ===")
	return result.String(), nil
}

// buildOutputRequirements builds the output requirements section
func (a *BaseAgent) buildOutputRequirements(output interface{}) string {
	return fmt.Sprintf(`

=== OUTPUT REQUIREMENTS ===
Format: json
Language: All content MUST be in %s
Structure:
%s
=== END REQUIREMENTS ===`, utils.GetLanguageName(a.language), utils.StructToJSONSchema(output, "  "))
}

// savePromptsToFile saves the prompts to a file for debugging
func (a *BaseAgent) savePromptsToFile(agentName, systemPrompt, userPrompt string) error {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join(agentLogRoot(), "logs", "prompts")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.md", agentName, timestamp)
	filepath := filepath.Join(logsDir, filename)

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

	// Write to file
	if err := os.WriteFile(filepath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write prompts file: %w", err)
	}

	logger.Info("[%s] Prompt log: %s", a.name, filepath)
	return nil
}

// saveResponseToFile saves the AI response to a file for debugging
func (a *BaseAgent) saveResponseToFile(agentName, response string) error {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join(agentLogRoot(), "logs", "responses")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.md", agentName, timestamp)
	filepath := filepath.Join(logsDir, filename)

	// Build content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Agent: %s\n", agentName))
	content.WriteString(fmt.Sprintf("# Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString("---\n\n")
	content.WriteString("# AI RESPONSE\n\n")
	content.WriteString("```\n")
	content.WriteString(response)
	content.WriteString("\n```\n")

	// Write to file
	if err := os.WriteFile(filepath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write response file: %w", err)
	}

	logger.Info("[%s] Response log: %s (%d chars)", a.name, filepath, len(response))
	return nil
}

func agentLogRoot() string {
	if dir := strings.TrimSpace(logger.Default().ProjectDir()); dir != "" {
		return dir
	}
	return "."
}

// parseResponse parses the AI response into the output struct
func (a *BaseAgent) parseResponse(content string, output interface{}) error {
	// Check for empty content
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("AI response is empty")
	}

	// Check for common error patterns in AI response
	contentPreview := clipResponsePreview(content, 200)
	logger.Debug("[%s] Parsing response (length: %d), preview: %s", a.name, len(content), contentPreview)

	// Try to parse as JSON directly
	if err := json.Unmarshal([]byte(content), output); err != nil {
		if repaired, repairErr := tryParseDeterministicallyRepairedJSON(content, output); repairErr == nil {
			logger.Warn("[%s] JSON parse failed, recovered via deterministic JSON type repair: %s", a.name, repaired)
			return nil
		}

		// Try to extract JSON from markdown code block
		jsonContent := extractJSONFromMarkdown(content)
		logger.Debug("[%s] Extracted JSON from markdown (length: %d)", a.name, len(jsonContent))

		if strings.TrimSpace(jsonContent) == "" {
			// Log the problematic response for debugging
			logger.Error("[%s] Extracted JSON content is empty. Raw response preview: %s", a.name, contentPreview)
			return fmt.Errorf("extracted JSON content is empty - AI may have returned non-JSON content\nOriginal response length: %d", len(content))
		}

		if err := json.Unmarshal([]byte(jsonContent), output); err != nil {
			if a.trySetDSLContentFromMalformed(content, output) || a.trySetDSLContentFromMalformed(jsonContent, output) {
				logger.Warn("[%s] JSON parse failed, recovered DSL content via malformed-JSON fallback", a.name)
				return nil
			}
			if repaired, repairErr := tryParseDeterministicallyRepairedJSON(jsonContent, output); repairErr == nil {
				logger.Warn("[%s] JSON parse failed, recovered via deterministic JSON type repair: %s", a.name, repaired)
				return nil
			}

			logger.Error("[%s] Failed to parse AI response as JSON: %v", a.name, err)
			// Log more context about the error
			errorPreview := clipResponsePreview(jsonContent, 500)
			logger.Error("[%s] JSON content that failed to parse: %s", a.name, errorPreview)
			return fmt.Errorf("failed to parse AI response as JSON: %w\nResponse length: %d, JSON extraction length: %d", err, len(content), len(jsonContent))
		}
	}
	return nil
}

func clipResponsePreview(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func tryParseDeterministicallyRepairedJSON(content string, output interface{}) (string, error) {
	var data interface{}
	quoteChanges := 0
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		repairedQuotes, changes := escapeLikelyUnescapedStringQuotes(content)
		if changes == 0 {
			return "", err
		}
		if quoteErr := json.Unmarshal([]byte(repairedQuotes), &data); quoteErr != nil {
			return "", err
		}
		quoteChanges = changes
	}
	numericChanges := repairJSONNumericStrings(data)
	if quoteChanges == 0 && numericChanges == 0 {
		return "", fmt.Errorf("no deterministic JSON type repairs available")
	}
	repaired, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(repaired, output); err != nil {
		return "", err
	}
	parts := []string{}
	if quoteChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d unescaped quote(s)", quoteChanges))
	}
	if numericChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d numeric string field(s)", numericChanges))
	}
	return strings.Join(parts, ", "), nil
}

func escapeLikelyUnescapedStringQuotes(content string) (string, int) {
	var b strings.Builder
	b.Grow(len(content))
	inString := false
	escaped := false
	changes := 0
	for i, r := range content {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && inString {
			b.WriteRune(r)
			escaped = true
			continue
		}
		if r != '"' {
			b.WriteRune(r)
			continue
		}
		if !inString {
			inString = true
			b.WriteRune(r)
			continue
		}
		if isLikelyJSONStringTerminator(content[i+1:]) {
			inString = false
			b.WriteRune(r)
			continue
		}
		b.WriteString(`\"`)
		changes++
	}
	return b.String(), changes
}

func isLikelyJSONStringTerminator(tail string) bool {
	for _, r := range tail {
		if unicode.IsSpace(r) {
			continue
		}
		switch r {
		case ':', ',', '}', ']':
			return true
		default:
			return false
		}
	}
	return true
}

func repairJSONNumericStrings(value interface{}) int {
	switch typed := value.(type) {
	case map[string]interface{}:
		changes := 0
		for key, child := range typed {
			if raw, ok := child.(string); ok && shouldRepairIntegerJSONField(key) {
				if n, ok := parseStrictJSONInt(raw); ok {
					typed[key] = float64(n)
					changes++
					continue
				}
			}
			if raw, ok := child.(float64); ok && shouldRepairIntegerJSONField(key) && raw != math.Trunc(raw) {
				typed[key] = float64(int64(math.Round(raw)))
				changes++
				continue
			}
			changes += repairJSONNumericStrings(child)
		}
		changes += repairResourceLedgerArithmetic(typed)
		return changes
	case []interface{}:
		changes := 0
		for _, child := range typed {
			changes += repairJSONNumericStrings(child)
		}
		return changes
	default:
		return 0
	}
}

func shouldRepairIntegerJSONField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "count", "level", "start", "delta", "end":
		return true
	default:
		return false
	}
}

func repairResourceLedgerArithmetic(value map[string]interface{}) int {
	if _, hasItem := value["item"]; !hasItem {
		return 0
	}
	start, okStart := jsonNumberToInt(value["start"])
	delta, okDelta := jsonNumberToInt(value["delta"])
	end, okEnd := jsonNumberToInt(value["end"])
	if !okStart || !okDelta || !okEnd {
		return 0
	}
	expected := start + delta
	if end == expected {
		return 0
	}
	value["end"] = float64(expected)
	return 1
}

func jsonNumberToInt(value interface{}) (int64, bool) {
	n, ok := value.(float64)
	if !ok || n != math.Trunc(n) {
		return 0, false
	}
	return int64(n), true
}

func parseStrictJSONInt(raw string) (int64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (a *BaseAgent) trySetDSLContentFromMalformed(content string, output interface{}) bool {
	dslContent, ok := extractDSLContentFromMalformed(content)
	if !ok {
		return false
	}

	v := reflect.ValueOf(output)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return false
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return false
	}

	field := elem.FieldByName("DSLContent")
	if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.String {
		return false
	}
	field.SetString(dslContent)
	return true
}

func extractDSLContentFromMalformed(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", false
	}

	// Direct DSL fallback
	if isLikelyDSL(trimmed) {
		return trimmed, true
	}

	keyIdx := strings.Index(trimmed, "\"dsl_content\"")
	if keyIdx == -1 {
		return "", false
	}

	body := trimmed[keyIdx:]
	start := findFirstDSLBlockStart(body)
	if start == -1 {
		return "", false
	}

	candidate := strings.TrimSpace(body[start:])
	candidate = strings.TrimSuffix(candidate, "```")
	candidate = strings.TrimSpace(candidate)

	// Remove common loose-JSON tails
	for _, suffix := range []string{"\"}", "\"\n}", "\"\r\n}", "\",\n}", "\",\r\n}", "\""} {
		if strings.HasSuffix(candidate, suffix) {
			candidate = strings.TrimSpace(strings.TrimSuffix(candidate, suffix))
		}
	}

	if !isLikelyDSL(candidate) {
		return "", false
	}
	return candidate, true
}

func findFirstDSLBlockStart(content string) int {
	keywords := []string{"metadata {", "world {", "characters {", "storyline {"}
	best := -1
	for _, kw := range keywords {
		if idx := strings.Index(content, kw); idx != -1 && (best == -1 || idx < best) {
			best = idx
		}
	}
	return best
}

func isLikelyDSL(content string) bool {
	required := []string{"{", "}"}
	for _, token := range required {
		if !strings.Contains(content, token) {
			return false
		}
	}
	// At least one top-level DSL block keyword
	return strings.Contains(content, "metadata {") ||
		strings.Contains(content, "world {") ||
		strings.Contains(content, "characters {") ||
		strings.Contains(content, "storyline {")
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(content string) string {
	// Look for ```json ... ``` or ``` ... ``` blocks
	startIdx := strings.Index(content, "```json")
	if startIdx == -1 {
		startIdx = strings.Index(content, "```")
	}
	if startIdx != -1 {
		// Find the end of the opening marker
		codeStart := strings.Index(content[startIdx:], "\n")
		if codeStart != -1 {
			codeStart += startIdx + 1

			// Find the closing marker
			endIdx := strings.Index(content[codeStart:], "```")
			if endIdx != -1 {
				return strings.TrimSpace(content[codeStart : codeStart+endIdx])
			}
			return strings.TrimSpace(content[codeStart:])
		}
	}

	// If no code block found, try to find JSON boundaries
	// Look for the first '{' and the last '}'
	startIdx = strings.Index(content, "{")
	if startIdx == -1 {
		return content
	}

	// Find the matching closing brace
	braceCount := 0
	endIdx := -1
	for i := startIdx; i < len(content); i++ {
		switch content[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				endIdx = i
				break
			}
		}
		if braceCount == 0 && endIdx != -1 {
			break
		}
	}

	if endIdx == -1 {
		return content[startIdx:]
	}

	return strings.TrimSpace(content[startIdx : endIdx+1])
}
