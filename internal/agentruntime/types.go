package agentruntime

import "context"

// Runtime invokes an external agent backend and returns the model response text.
type Runtime interface {
	Invoke(ctx context.Context, invocation Invocation) (*Result, error)
}

// Invocation is the language-neutral contract between Go agents and SDK runners.
type Invocation struct {
	AgentName              string                 `json:"agent_name"`
	Command                string                 `json:"command"`
	Skills                 []string               `json:"skills,omitempty"`
	SDKSkills              []string               `json:"sdk_skills,omitempty"`
	Language               string                 `json:"language,omitempty"`
	WorkspaceRoot          string                 `json:"workspace_root,omitempty"`
	Settings               string                 `json:"settings,omitempty"`
	SettingSources         []string               `json:"setting_sources,omitempty"`
	AddDirs                []string               `json:"add_dirs,omitempty"`
	Tools                  []string               `json:"tools,omitempty"`
	AllowedTools           []string               `json:"allowed_tools,omitempty"`
	PermissionMode         string                 `json:"permission_mode,omitempty"`
	RequireSDK             bool                   `json:"require_sdk,omitempty"`
	ToolAllowlist          []string               `json:"tool_allowlist,omitempty"`
	LiveLogPath            string                 `json:"live_log_path,omitempty"`
	SystemPrompt           string                 `json:"system_prompt"`
	UserPrompt             string                 `json:"user_prompt"`
	UserPromptDriven       bool                   `json:"user_prompt_driven,omitempty"`
	OutputSchemaText       string                 `json:"output_schema_text,omitempty"`
	OutputJSONSchema       map[string]interface{} `json:"output_json_schema,omitempty"`
	CompactOutputSchema    bool                   `json:"compact_output_schema,omitempty"`
	DisableSDKOutputFormat bool                   `json:"disable_sdk_output_format,omitempty"`
	ToolEvidence           ToolEvidence           `json:"tool_evidence,omitempty"`
	Options                Options                `json:"options,omitempty"`
	Metadata               map[string]string      `json:"metadata,omitempty"`
}

// ToolEvidence is the in-flight evidence contract passed to the SDK runner.
// The runner blocks the agent from stopping until the listed tool activity has
// been observed, so required queries/checks/patches are corrected inside the
// same agent turn instead of being rejected after the fact. Go revalidates the
// live log after the run as the final safety net.
type ToolEvidence struct {
	MinQueryCalls        int      `json:"min_query_calls,omitempty"`
	MinContextQueryCalls int      `json:"min_context_query_calls,omitempty"`
	MinCheckCalls        int      `json:"min_check_calls,omitempty"`
	MinPatchApplyCalls   int      `json:"min_patch_apply_calls,omitempty"`
	RequiredToolCommands []string `json:"required_tool_commands,omitempty"`
	RequireNoDeniedTools bool     `json:"require_no_denied_tools,omitempty"`
}

// Options contains common generation options used by process runners.
type Options struct {
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	MaxTurns    int     `json:"max_turns,omitempty"`
	Timeout     int     `json:"timeout,omitempty"`
}

// Result is the normalized response returned by a runtime.
type Result struct {
	Content     string       `json:"content"`
	Model       string       `json:"model,omitempty"`
	Usage       Usage        `json:"usage,omitempty"`
	LiveLogPath string       `json:"live_log_path,omitempty"`
	LiveSummary *LiveSummary `json:"live_summary,omitempty"`
}

// Usage is provider-independent token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// LiveSummary is a compact view of the agent live JSONL log. Tool-specific
// command counters count allowed tool calls only; denied attempts are tracked
// separately through ToolDenied and DeniedToolCommands.
type LiveSummary struct {
	Events                     int      `json:"events,omitempty"`
	Messages                   int      `json:"messages,omitempty"`
	Model                      string   `json:"model,omitempty"`
	FinalModel                 string   `json:"final_model,omitempty"`
	ToolCalls                  int      `json:"tool_calls,omitempty"`
	ToolAllowed                int      `json:"tool_allowed,omitempty"`
	ToolDenied                 int      `json:"tool_denied,omitempty"`
	AllowedToolCommands        []string `json:"allowed_tool_commands,omitempty"`
	DeniedToolCommands         []string `json:"denied_tool_commands,omitempty"`
	WorkflowDeniedToolCommands []string `json:"workflow_denied_tool_commands,omitempty"`
	DenialsResolved            bool     `json:"denials_resolved,omitempty"`
	HookErrors                 []string `json:"hook_errors,omitempty"`
	SDKSkills                  []string `json:"sdk_skills,omitempty"`
	LoadedSDKSkills            []string `json:"loaded_sdk_skills,omitempty"`
	MissingSDKSkills           []string `json:"missing_sdk_skills,omitempty"`
	SDKSkillPromptChars        int      `json:"sdk_skill_prompt_chars,omitempty"`
	QueryCalls                 int      `json:"query_calls,omitempty"`
	QueryIndexCalls            int      `json:"query_index_calls,omitempty"`
	QueryBriefCalls            int      `json:"query_brief_calls,omitempty"`
	QueryFullCalls             int      `json:"query_full_calls,omitempty"`
	ContextQueryCalls          int      `json:"context_query_calls,omitempty"`
	CheckCalls                 int      `json:"check_calls,omitempty"`
	RefreshCalls               int      `json:"refresh_calls,omitempty"`
	PatchCalls                 int      `json:"patch_calls,omitempty"`
	PatchApplies               int      `json:"patch_applies,omitempty"`
	ToolDurationMS             int      `json:"tool_duration_ms,omitempty"`
	SlowestToolDurationMS      int      `json:"slowest_tool_duration_ms,omitempty"`
	SlowestToolCommand         string   `json:"slowest_tool_command,omitempty"`
	ApplyWithoutFollowupCheck  int      `json:"apply_without_followup_check,omitempty"`
	FinalRecords               int      `json:"final_records,omitempty"`
}
