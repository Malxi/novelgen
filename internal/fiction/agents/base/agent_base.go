package base

import (
	"context"
	"fmt"
	"novelgen/internal/contract/story"
	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/envutil"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BaseAgent is the base struct for all agents.
// It loads prompts from skill files and provides common LLM interaction logic.
type BaseAgent struct {
	name           string
	client         llm.Client
	runtime        agentruntime.Runtime
	config         *llm.Config
	projectLLM     *models.ProjectLLM
	skillLoader    *SkillLoader
	language       string
	modelOverride  string
	noRecapContext bool
	project        *store.Root
}

// BaseAgentConfig holds configuration for creating a BaseAgent
type BaseAgentConfig struct {
	Name        string
	Client      llm.Client
	Runtime     agentruntime.Runtime
	Config      *llm.Config
	ProjectLLM  *models.ProjectLLM
	Language    string
	ProjectRoot *store.Root
}

// NewBaseAgent creates a new BaseAgent
func NewBaseAgent(cfg BaseAgentConfig) *BaseAgent {
	return &BaseAgent{
		name:        cfg.Name,
		client:      cfg.Client,
		runtime:     resolveRuntime(cfg.ProjectLLM, cfg.Runtime, cfg.Client),
		config:      cfg.Config,
		projectLLM:  cfg.ProjectLLM,
		skillLoader: NewSkillLoader(resolveSkillsDir()),
		language:    cfg.Language,
		project:     cfg.ProjectRoot,
	}
}

// SetProjectRoot binds this agent to the project it was constructed for. SDK
// workspace selection and agent log files must use that root instead of the
// process's ambient project.
func (a *BaseAgent) SetProjectRoot(root *store.Root) {
	if a == nil {
		return
	}
	a.project = root
}

// SkillsDirEnv overrides where agent skills are read from. Useful when a
// deployment wants to ship edited prompts next to the binary.
const SkillsDirEnv = "NOVELGEN_SKILLS_DIR"

// resolveSkillsDir finds the best on-disk skills directory. It prefers an
// explicit override, then a "skills" folder next to the executable, and finally
// the source tree recorded at build time. When none of these exist the loader
// falls back to the skills embedded in the binary.
func resolveSkillsDir() string {
	if dir := envutil.String(SkillsDirEnv); dir != "" {
		return dir
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "skills")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	if _, filename, _, ok := runtime.Caller(0); ok {
		return filepath.Join(filepath.Dir(filename), "skills")
	}
	return ""
}

// SetLanguage sets the output language
func (a *BaseAgent) SetLanguage(language string) {
	a.language = language
}

// Language returns the output language configured for this agent.
func (a *BaseAgent) Language() string {
	if a == nil {
		return ""
	}
	return a.language
}

// Config returns the LLM configuration used by this agent. Stage packages need
// it when they build SDK prompt inputs (for example, to distinguish chat from
// SDK execution).
func (a *BaseAgent) Config() *llm.Config {
	if a == nil {
		return nil
	}
	return a.config
}

// SetModelOverride overrides the project model for this agent's chat options.
// When the model exists in the active provider's llm_config, its max_tokens
// and temperature are applied as well; otherwise only the model name changes.
func (a *BaseAgent) SetModelOverride(model string) {
	a.modelOverride = strings.TrimSpace(model)
}

// SetNoRecapContext propagates the recap-retirement experiment into Agent SDK
// tool child processes. It only suppresses recap prompt blocks; canonical recap
// data remains available to deterministic checks and state accounting.
func (a *BaseAgent) SetNoRecapContext(disabled bool) {
	if a != nil {
		a.noRecapContext = disabled
	}
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
	// UserPromptDriven marks SDK invocations where the user supplied an
	// explicit task/prompt. The runner treats clean scoped checks as
	// evidence rather than a stop signal in this mode, so the agent still
	// proceeds to the patch cycle when the user request describes changes.
	UserPromptDriven bool
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
	callStartedAt := time.Now()
	resp, runtimeResult, err := a.invokeAIWithRuntimeResult(ctx, params, messages, options, outputRequirements, output)
	callKind := "chat"
	if runtimeResult != nil {
		callKind = "agent-sdk"
	}
	if err != nil {
		logAgentRuntimeLiveSummary(a.name, runtimeResult)
		logger.Error("[%s] AI request failed: %v", a.name, err)
		a.recordUsage(params, callKind, "", llm.Usage{}, time.Since(callStartedAt), err)
		a.recordAICall(params, "", systemPrompt, userPrompt, "", llm.Usage{}, callStartedAt)
		return runtimeResult, fmt.Errorf("AI request failed: %w", err)
	}
	a.recordUsage(params, callKind, resp.Model, resp.Usage, time.Since(callStartedAt), nil)
	a.recordAICall(params, resp.Model, systemPrompt, userPrompt, resp.Content, resp.Usage, callStartedAt)
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

func (a *BaseAgent) invokeAI(ctx context.Context, params InvokeParams, messages []llm.Message, options *llm.ChatOptions, outputSchemaText string, output interface{}) (*llm.ChatResponse, error) {
	resp, _, err := a.invokeAIWithRuntimeResult(ctx, params, messages, options, outputSchemaText, output)
	return resp, err
}

func (a *BaseAgent) invokeAIWithRuntimeResult(ctx context.Context, params InvokeParams, messages []llm.Message, options *llm.ChatOptions, outputSchemaText string, output interface{}) (*llm.ChatResponse, *agentruntime.Result, error) {
	if options == nil {
		options = &llm.ChatOptions{}
	}
	startedAt := time.Now()
	systemPrompt := messageContent(messages, "system")
	userPrompt := messageContent(messages, "user")
	runtime := a.runtime
	if runtime == nil && params.RequireSDK {
		requiredRuntime, err := loadRequiredAgentRuntime()
		if err != nil {
			a.recordRunMetrics(ctx, params, systemPrompt, userPrompt, "", "agent-sdk", "", llm.Usage{}, nil, startedAt, err)
			return nil, nil, err
		}
		runtime = requiredRuntime
	}
	if runtime != nil {
		metadata := map[string]string{}
		if run := a.currentRun(ctx); run != nil {
			metadata["run_id"] = run.ID()
		}
		if a.noRecapContext {
			metadata["no_recap_context"] = "1"
		}
		invocation := agentruntime.Invocation{
			AgentName:              a.name,
			Command:                params.Command,
			Skills:                 append([]string(nil), params.Skills...),
			SDKSkills:              append([]string(nil), params.SDKSkills...),
			Language:               utils.GetLanguageName(a.language),
			WorkspaceRoot:          a.workspaceRoot(),
			Tools:                  append([]string(nil), params.Tools...),
			AllowedTools:           append([]string(nil), params.AllowedTools...),
			PermissionMode:         params.PermissionMode,
			RequireSDK:             params.RequireSDK,
			ToolAllowlist:          append([]string(nil), params.ToolAllowlist...),
			SystemPrompt:           messageContent(messages, "system"),
			UserPrompt:             messageContent(messages, "user"),
			UserPromptDriven:       params.UserPromptDriven,
			OutputSchemaText:       outputSchemaText,
			OutputJSONSchema:       utils.StructToJSONSchemaObject(output),
			CompactOutputSchema:    params.CompactOutputSchema,
			DisableSDKOutputFormat: params.DisableSDKOutputFormat,
			ToolEvidence:           agentEvidenceConfig(params.ToolEvidence),
			Metadata:               metadata,
			Options: agentruntime.Options{
				Model:       agentRuntimeModel(params, options.Model),
				Temperature: options.Temperature,
				MaxTokens:   options.MaxTokens,
				MaxTurns:    params.MaxTurns,
				Timeout:     params.Timeout,
			},
		}
		result, err := runtime.Invoke(ctx, invocation)
		usage := llm.Usage{}
		model := ""
		response := ""
		if result != nil {
			model = result.Model
			response = result.Content
			usage = llm.Usage{
				PromptTokens:     result.Usage.PromptTokens,
				CompletionTokens: result.Usage.CompletionTokens,
				TotalTokens:      result.Usage.TotalTokens,
			}
		}
		a.recordRunMetrics(ctx, params, systemPrompt, userPrompt, model, "agent-sdk", response, usage, result, startedAt, err)
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
		err := fmt.Errorf("the Claude Agent SDK runtime is required for this workflow")
		a.recordRunMetrics(ctx, params, systemPrompt, userPrompt, "", "agent-sdk", "", llm.Usage{}, nil, startedAt, err)
		return nil, nil, err
	}
	if a.client == nil {
		a.recordRunMetrics(ctx, params, systemPrompt, userPrompt, "", "chat", "", llm.Usage{}, nil, startedAt, fmt.Errorf("no LLM client or agent runtime configured"))
		return nil, nil, fmt.Errorf("no LLM client or agent runtime configured")
	}
	resp, err := a.client.ChatCompletion(ctx, messages, options)
	if resp != nil {
		a.recordRunMetrics(ctx, params, systemPrompt, userPrompt, resp.Model, "chat", resp.Content, resp.Usage, nil, startedAt, err)
	} else {
		a.recordRunMetrics(ctx, params, systemPrompt, userPrompt, "", "chat", "", llm.Usage{}, nil, startedAt, err)
	}
	return resp, nil, err
}

func loadRequiredAgentRuntime() (agentruntime.Runtime, error) {
	cfg, err := agentruntime.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("the Claude Agent SDK runtime is required for this workflow: %w", err)
	}
	runtime, err := cfg.NewRuntime("")
	if err != nil {
		return nil, fmt.Errorf("the Claude Agent SDK runtime is required for this workflow: %w", err)
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

// agentWorkspaceRoot returns the directory the Agent SDK runs against: the
// logger's project directory when the write pipeline pinned one, otherwise the
// resolved project root, otherwise the working directory. Reading the store
// first is what keeps an SDK run pointed at the project even when the process
// was not started inside it (for example with `novelgen --dir`).
func agentWorkspaceRoot() string {
	if dir := strings.TrimSpace(logger.Default().ProjectDir()); dir != "" {
		return dir
	}
	if root, err := store.Default(); err == nil {
		return root.Dir()
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

func (a *BaseAgent) workspaceRoot() string {
	if a != nil && a.project != nil && a.project.Dir() != "" {
		return a.project.Dir()
	}
	return agentWorkspaceRoot()
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
		provider = projectLLM.Provider
	}
	if !models.IsAgentSDKProvider(provider) {
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

// ActiveModelLimits reports the configured context window and generation token
// budget for the model the agent will invoke. Callers use it to size batched
// structured outputs before sending a prompt that can exceed the model budget.
func (a *BaseAgent) ActiveModelLimits() (contextWindow, maxOutputTokens int) {
	if a == nil || a.config == nil || a.projectLLM == nil {
		return 0, 0
	}
	options := a.chatOptions()
	if options != nil {
		maxOutputTokens = options.MaxTokens
	}
	provider := a.config.GetActiveProvider(a.projectLLM)
	if provider == nil || options == nil {
		return 0, maxOutputTokens
	}
	if model := provider.Models[options.Model]; model != nil {
		contextWindow = model.Context
	}
	return contextWindow, maxOutputTokens
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

// AgentLogsDisabledEnv turns off prompt/response log files when set to a
// truthy value. Large runs can produce tens of thousands of files.
const AgentLogsDisabledEnv = "NOVELGEN_DISABLE_AGENT_LOGS"
