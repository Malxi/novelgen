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
	skills      []string
	client      llm.Client
	config      *llm.Config
	projectLLM  *models.ProjectLLM
	skillLoader *SkillLoader
	language    string
}

// BaseAgentConfig holds configuration for creating a BaseAgent
type BaseAgentConfig struct {
	Name       string
	Skills     []string
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
		skills:      cfg.Skills,
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

// Execute sends the input to AI and returns the result
// This is the core method that all agents use
func (a *BaseAgent) Execute(ctx context.Context, input interface{}, output interface{}) error {
	// Load system prompt from skill file
	systemPrompt, err := a.loadSystemPrompt()
	if err != nil {
		return fmt.Errorf("failed to load system prompt: %w", err)
	}

	// Build user prompt from input
	userPrompt := prompts.StructToMarkdown(input, 0)

	// Add output requirements
	outputRequirements := a.buildOutputRequirements(output)
	fullSystemPrompt := systemPrompt + outputRequirements

	// Log prompts - use first skill name for logging
	skillName := "unknown"
	if len(a.skills) > 0 {
		skillName = a.skills[0]
	}
	logger.Prompt(skillName, "default", fullSystemPrompt, userPrompt)

	// Save prompts to file for debugging
	if err := a.savePromptsToFile(skillName, fullSystemPrompt, userPrompt); err != nil {
		logger.Debug("[%s] Failed to save prompts to file: %v", a.name, err)
	}

	messages := []llm.Message{
		{Role: "system", Content: fullSystemPrompt},
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

// loadSystemPrompt loads the system prompt from skill files
// Multiple skills are concatenated with separators
func (a *BaseAgent) loadSystemPrompt() (string, error) {
	vars := map[string]string{
		"language": prompts.GetLanguageName(a.language),
	}

	var prompts []string
	for _, skillName := range a.skills {
		prompt, err := a.skillLoader.LoadWithVars(skillName, vars)
		if err != nil {
			return "", fmt.Errorf("failed to load skill %s: %w", skillName, err)
		}
		prompts = append(prompts, prompt)
	}

	// Join multiple skills with separators
	if len(prompts) == 1 {
		return prompts[0], nil
	}

	var result strings.Builder
	for i, prompt := range prompts {
		if i > 0 {
			result.WriteString("\n\n=== ADDITIONAL INSTRUCTIONS ===\n\n")
		}
		result.WriteString(prompt)
	}
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
func (a *BaseAgent) savePromptsToFile(skillName, systemPrompt, userPrompt string) error {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join("logs", "prompts")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.md", a.name, skillName, timestamp)
	filepath := filepath.Join(logsDir, filename)

	// Build content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Agent: %s\n", a.name))
	content.WriteString(fmt.Sprintf("# Skill: %s\n", skillName))
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
