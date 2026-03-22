package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/prompts"
)

// CraftAgent generates detailed story elements (characters, locations, items)
type CraftAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
	setup      *models.StorySetup
	outline    *models.Outline
	language   string
	log        logger.LoggerInterface
	pm         *prompts.PromptManager
}

// NewCraftAgent creates a new craft agent
func NewCraftAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline, language string) *CraftAgent {
	return &CraftAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
		setup:      setup,
		outline:    outline,
		language:   language,
		log:        logger.GetLogger(),
		pm:         prompts.NewPromptManager(),
	}
}

// GenerateCharacters generates detailed character profiles
func (a *CraftAgent) GenerateCharacters(names []string, customPrompt string) (map[string]*models.Character, error) {
	a.log.Info("Generating characters: count=%d", len(names))

	// Build prompt data
	data := map[string]interface{}{
		"story_title":    a.setup.ProjectName,
		"story_genre":    strings.Join(a.setup.Genres, ", "),
		"story_style":    a.setup.Tone,
		"characters":     names,
		"custom_prompt":  customPrompt,
		"story_setup":    prompts.StructToPrompt(a.setup, ""),
		"outline_sample": a.getOutlineSample(),
		"language":       a.language,
		"language_name":  prompts.GetLanguageName(a.language),
	}

	// Build prompts using PromptManager
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillCharacterCreation, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call LLM with options from config
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate characters: %w", err)
	}

	// Parse response
	var characters map[string]*models.Character
	if err := json.Unmarshal([]byte(response.Content), &characters); err != nil {
		// Try to extract JSON from markdown code block
		jsonStr := extractJSONFromMarkdown(response.Content)
		if err := json.Unmarshal([]byte(jsonStr), &characters); err != nil {
			return nil, fmt.Errorf("failed to parse character response: %w", err)
		}
	}

	return characters, nil
}

// GenerateLocations generates detailed location descriptions
func (a *CraftAgent) GenerateLocations(names []string, customPrompt string) (map[string]*models.Location, error) {
	a.log.Info("Generating locations: count=%d", len(names))

	// Build prompt data
	data := map[string]interface{}{
		"story_title":    a.setup.ProjectName,
		"story_genre":    strings.Join(a.setup.Genres, ", "),
		"story_style":    a.setup.Tone,
		"locations":      names,
		"custom_prompt":  customPrompt,
		"story_setup":    prompts.StructToPrompt(a.setup, ""),
		"outline_sample": a.getOutlineSample(),
		"language":       a.language,
		"language_name":  prompts.GetLanguageName(a.language),
	}

	// Build prompts using PromptManager
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillLocationCreation, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call LLM with options from config
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)
	// Locations need more tokens due to detailed sensory descriptions
	if opts.MaxTokens < 12000 {
		opts.MaxTokens = 12000
	}

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate locations: %w", err)
	}

	// Parse response
	var locations map[string]*models.Location
	if err := json.Unmarshal([]byte(response.Content), &locations); err != nil {
		jsonStr := extractJSONFromMarkdown(response.Content)
		if err := json.Unmarshal([]byte(jsonStr), &locations); err != nil {
			return nil, fmt.Errorf("failed to parse location response: %w", err)
		}
	}

	return locations, nil
}

// GenerateItems generates detailed item descriptions
func (a *CraftAgent) GenerateItems(names []string, customPrompt string) (map[string]*models.Item, error) {
	a.log.Info("Generating items: count=%d", len(names))

	// Build prompt data
	data := map[string]interface{}{
		"story_title":    a.setup.ProjectName,
		"story_genre":    strings.Join(a.setup.Genres, ", "),
		"story_style":    a.setup.Tone,
		"items":          names,
		"custom_prompt":  customPrompt,
		"story_setup":    prompts.StructToPrompt(a.setup, ""),
		"outline_sample": a.getOutlineSample(),
		"language":       a.language,
		"language_name":  prompts.GetLanguageName(a.language),
	}

	// Build prompts using PromptManager
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillItemCreation, "default", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call LLM with options from config
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)
	if opts.MaxTokens < 8000 {
		opts.MaxTokens = 8000
	}

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate items: %w", err)
	}

	// Parse response
	var items map[string]*models.Item
	if err := json.Unmarshal([]byte(response.Content), &items); err != nil {
		jsonStr := extractJSONFromMarkdown(response.Content)
		if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
			return nil, fmt.Errorf("failed to parse item response: %w", err)
		}
	}

	return items, nil
}

func (a *CraftAgent) getOutlineSample() string {
	if a.outline == nil || len(a.outline.Parts) == 0 {
		return "No outline available"
	}

	var sample strings.Builder
	sample.WriteString("Story Outline:\n")

	// Add first part as sample
	part := a.outline.Parts[0]
	sample.WriteString(fmt.Sprintf("Part: %s\n", part.Title))

	if len(part.Volumes) > 0 {
		vol := part.Volumes[0]
		sample.WriteString(fmt.Sprintf("  Volume: %s\n", vol.Title))

		if len(vol.Chapters) > 0 {
			// Show first 3 chapters
			maxChapters := 3
			if len(vol.Chapters) < maxChapters {
				maxChapters = len(vol.Chapters)
			}
			for i := 0; i < maxChapters; i++ {
				ch := vol.Chapters[i]
				sample.WriteString(fmt.Sprintf("    Chapter %s: %s\n", ch.ID, ch.Title))
				if ch.Summary != "" {
					sample.WriteString(fmt.Sprintf("      Summary: %s\n", ch.Summary))
				}
			}
		}
	}

	return sample.String()
}
