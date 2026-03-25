package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/prompts"
)

// BaseAgent is the base struct for all agents.
// It loads prompts from skill files and provides common LLM interaction logic.
type BaseAgent struct {
	name        string
	client      llm.Client
	config      *llm.Config
	projectLLM  *models.ProjectLLM
	skillLoader *SkillLoader
	language    string
}

// BaseAgentConfig holds configuration for creating a BaseAgent
type BaseAgentConfig struct {
	Name       string
	Client     llm.Client
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

// InvokeParams holds the parameters for invoking an agent
type InvokeParams struct {
	Skills  []string
	Command string
}

// Execute sends the input to AI and returns the result
// This is the core method that all agents use
func (a *BaseAgent) Execute(ctx context.Context, params InvokeParams, input interface{}, output interface{}) error {
	// Use provided params or fall back to agent defaults
	// Load system prompt from skill files
	skillPrompt, err := a.loadSystemPromptWithSkills(params.Skills)
	if err != nil {
		return fmt.Errorf("failed to load system prompt: %w", err)
	}

	// Build user prompt from input
	userPrompt := fmt.Sprintf("Follow skill to %s based on the input %s\n",
		params.Command, prompts.StructToMarkdown(input, 0))

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

	options := a.config.GetChatOptions(a.projectLLM)

	logger.Info("[%s] Sending request to AI...", a.name)
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		logger.Error("[%s] AI request failed: %v", a.name, err)
		return fmt.Errorf("AI request failed: %w", err)
	}

	logger.Info("[%s] Received response (%d tokens used)", a.name, resp.Usage.TotalTokens)

	// Parse response into output
	if err := a.parseResponse(resp.Content, output); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}

// loadSystemPromptWithSkills loads the system prompt from specified skills
// All skills are wrapped with skill markers
func (a *BaseAgent) loadSystemPromptWithSkills(skills []string) (string, error) {
	vars := map[string]string{
		"language": prompts.GetLanguageName(a.language),
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
=== END REQUIREMENTS ===`, prompts.GetLanguageName(a.language), prompts.StructToJSONSchema(output, "  "))
}

// savePromptsToFile saves the prompts to a file for debugging
func (a *BaseAgent) savePromptsToFile(agentName, systemPrompt, userPrompt string) error {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join("logs", "prompts")
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

	logger.Debug("[%s] Saved prompts to: %s", a.name, filepath)
	return nil
}

// parseResponse parses the AI response into the output struct
func (a *BaseAgent) parseResponse(content string, output interface{}) error {
	// Try to parse as JSON directly
	if err := json.Unmarshal([]byte(content), output); err != nil {
		// Try to extract JSON from markdown code block
		jsonContent := extractJSONFromMarkdown(content)
		logger.Debug("[%s] Extracted JSON from markdown", a.name)
		if err := json.Unmarshal([]byte(jsonContent), output); err != nil {
			logger.Error("[%s] Failed to parse AI response as JSON: %v", a.name, err)
			logger.Debug("[%s] Raw response: %s", a.name, content)
			return fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, content)
		}
	}
	return nil
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(content string) string {
	// Look for ```json ... ``` or ``` ... ``` blocks
	startIdx := strings.Index(content, "```json")
	if startIdx == -1 {
		startIdx = strings.Index(content, "```")
	}
	if startIdx == -1 {
		return content
	}

	// Find the end of the opening marker
	codeStart := strings.Index(content[startIdx:], "\n")
	if codeStart == -1 {
		return content
	}
	codeStart += startIdx + 1

	// Find the closing marker
	endIdx := strings.Index(content[codeStart:], "```")
	if endIdx == -1 {
		return content[codeStart:]
	}

	return strings.TrimSpace(content[codeStart : codeStart+endIdx])
}
