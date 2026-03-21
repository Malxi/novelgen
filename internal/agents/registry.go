package agents

import (
	"novelgen/internal/llm"
	"novelgen/internal/models"
)

// Agent is the common interface for all AI agents
type Agent interface {
	SetLanguage(language string)
}

// AgentFactory creates a new agent instance
type AgentFactory func(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) Agent

// agentRegistry holds all registered agent factories
var agentRegistry = make(map[string]AgentFactory)

// RegisterAgent registers an agent factory with a name
// Call this in your agent file's init() function
func RegisterAgent(name string, factory AgentFactory) {
	agentRegistry[name] = factory
}

// GetAgent creates an agent instance by name
func GetAgent(name string, client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) Agent {
	if factory, ok := agentRegistry[name]; ok {
		return factory(client, config, projectLLM)
	}
	return nil
}

// HasAgent checks if an agent is registered
func HasAgent(name string) bool {
	_, ok := agentRegistry[name]
	return ok
}
