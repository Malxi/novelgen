package models

import "strings"

// Provider names understood by novel.json and llm_config.json. They are the
// vocabulary the CLI, the LLM config and the agent runtimes branch on, so they
// live next to ProjectLLM instead of being repeated as string literals in each
// caller.
const (
	ProviderClaude    = "claude"
	ProviderClaudeSDK = "claude_sdk"
	ProviderAgent     = "agent"
	ProviderOpenAI    = "openai"
	ProviderOllama    = "ollama"
)

// IsAgentSDKProvider reports whether a project provider selects the Claude Agent
// SDK runtime, which the major creative stages require, instead of the plain
// OpenAI-compatible chat client. The alias spellings the CLI has historically
// accepted are all recognized here; the runtime mapping itself lives in
// internal/agentruntime.
func IsAgentSDKProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case ProviderClaude, ProviderClaudeSDK, ProviderAgent:
		return true
	default:
		return false
	}
}
