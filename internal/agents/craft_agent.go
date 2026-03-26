package agents

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// CraftGenCharactersInput is the input for character generation
type CraftGenCharactersInput struct {
	StorySetup       models.StorySetup `md:"story_setup"`
	Outline          models.Outline    `md:"outline"`
	RelevantChapters []string          `md:"relevant_chapters"`
	Characters       []string          `md:"characters"`
	CustomPrompt     string            `md:"custom_prompt,omitempty"`
}

// CraftGenCharactersOutput is the output for character generation
type CraftGenCharactersOutput struct {
	Characters map[string]models.Character `md:"characters"`
}

// CraftGenLocationsInput is the input for location generation
type CraftGenLocationsInput struct {
	StorySetup       models.StorySetup `md:"story_setup"`
	Outline          models.Outline    `md:"outline"`
	RelevantChapters []string          `md:"relevant_chapters"`
	Locations        []string          `md:"locations"`
	CustomPrompt     string            `md:"custom_prompt,omitempty"`
}

// CraftGenLocationsOutput is the output for location generation
type CraftGenLocationsOutput struct {
	Locations map[string]models.Location `md:"locations"`
}

// CraftGenItemsInput is the input for item generation
type CraftGenItemsInput struct {
	StorySetup       models.StorySetup `md:"story_setup"`
	Outline          models.Outline    `md:"outline"`
	RelevantChapters []string          `md:"relevant_chapters"`
	Items            []string          `md:"items"`
	CustomPrompt     string            `md:"custom_prompt,omitempty"`
}

// CraftGenItemsOutput is the output for item generation
type CraftGenItemsOutput struct {
	Items map[string]models.Item `md:"items"`
}

// CraftAgent generates detailed story elements (characters, locations, items)
// It wraps BaseAgent to provide type-safe methods
type CraftAgent struct {
	base    *BaseAgent
	setup   *models.StorySetup
	outline *models.Outline
}

// NewCraftAgent creates a new CraftAgent
func NewCraftAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline) *CraftAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "CraftAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &CraftAgent{
		base:    base,
		setup:   setup,
		outline: outline,
	}
}

// SetLanguage sets the output language
func (a *CraftAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// GenerateCharacters generates detailed character profiles
func (a *CraftAgent) GenerateCharacters(ctx context.Context, names []string, customPrompt string) (map[string]models.Character, error) {
	logger.Section("CRAFT AGENT - Character Generation")
	logger.Info("Characters: %v", names)
	logger.Info("Language: %s", a.base.language)

	// Find chapters where these characters appear
	relevantChapters := a.findChaptersWithCharacters(names)
	logger.Info("Found %d relevant chapters for these characters", len(relevantChapters))

	input := CraftGenCharactersInput{
		StorySetup:       *a.setup,
		Outline:          *a.outline,
		RelevantChapters: relevantChapters,
		Characters:       names,
		CustomPrompt:     customPrompt,
	}

	var output CraftGenCharactersOutput
	params := InvokeParams{
		Skills:  []string{"craft-characters"},
		Command: "generate detailed character profiles",
	}

	if err := a.base.Execute(ctx, params, input, &output.Characters); err != nil {
		return nil, err
	}

	// Validate and normalize
	for name, char := range output.Characters {
		if char.Name == "" {
			char.Name = name
		}
		output.Characters[name] = char
	}

	logger.Info("✓ Generated %d characters", len(output.Characters))
	return output.Characters, nil
}

// GenerateLocations generates detailed location descriptions
func (a *CraftAgent) GenerateLocations(ctx context.Context, names []string, customPrompt string) (map[string]models.Location, error) {
	logger.Section("CRAFT AGENT - Location Generation")
	logger.Info("Locations: %v", names)
	logger.Info("Language: %s", a.base.language)

	// Find chapters where these locations appear
	relevantChapters := a.findChaptersWithLocations(names)
	logger.Info("Found %d relevant chapters for these locations", len(relevantChapters))

	input := CraftGenLocationsInput{
		StorySetup:       *a.setup,
		Outline:          *a.outline,
		RelevantChapters: relevantChapters,
		Locations:        names,
		CustomPrompt:     customPrompt,
	}

	var output CraftGenLocationsOutput
	params := InvokeParams{
		Skills:  []string{"craft-locations"},
		Command: "generate detailed location descriptions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Locations); err != nil {
		return nil, err
	}

	// Validate and normalize
	for name, loc := range output.Locations {
		if loc.Name == "" {
			loc.Name = name
		}
		output.Locations[name] = loc
	}

	logger.Info("✓ Generated %d locations", len(output.Locations))
	return output.Locations, nil
}

// GenerateItems generates detailed item descriptions
func (a *CraftAgent) GenerateItems(ctx context.Context, names []string, customPrompt string) (map[string]models.Item, error) {
	logger.Section("CRAFT AGENT - Item Generation")
	logger.Info("Items: %v", names)
	logger.Info("Language: %s", a.base.language)

	// Find chapters where these items appear
	relevantChapters := a.findChaptersWithItems(names)
	logger.Info("Found %d relevant chapters for these items", len(relevantChapters))

	input := CraftGenItemsInput{
		StorySetup:       *a.setup,
		Outline:          *a.outline,
		RelevantChapters: relevantChapters,
		Items:            names,
		CustomPrompt:     customPrompt,
	}

	var output CraftGenItemsOutput
	params := InvokeParams{
		Skills:  []string{"craft-items"},
		Command: "generate detailed item descriptions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Items); err != nil {
		return nil, err
	}

	// Validate and normalize
	for name, item := range output.Items {
		if item.Name == "" {
			item.Name = name
		}
		output.Items[name] = item
	}

	logger.Info("✓ Generated %d items", len(output.Items))
	return output.Items, nil
}

// findChaptersWithCharacters finds chapters that mention the given characters
func (a *CraftAgent) findChaptersWithCharacters(characterNames []string) []string {
	if a.outline == nil {
		return nil
	}

	var relevantChapters []string
	nameSet := make(map[string]bool)
	for _, name := range characterNames {
		nameSet[name] = true
	}

	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				// Check if any character appears in this chapter's character list
				for _, char := range ch.Characters {
					if nameSet[char] {
						chapterInfo := fmt.Sprintf("Chapter %s: %s\nSummary: %s", ch.ID, ch.Title, ch.Summary)
						relevantChapters = append(relevantChapters, chapterInfo)
						break
					}
				}

				// Also check in events
				for _, event := range ch.Events {
					for _, char := range event.Characters {
						if nameSet[char] {
							chapterInfo := fmt.Sprintf("Chapter %s: %s\nSummary: %s", ch.ID, ch.Title, ch.Summary)
							relevantChapters = append(relevantChapters, chapterInfo)
							break
						}
					}
				}
			}
		}
	}

	return relevantChapters
}

// findChaptersWithLocations finds chapters that mention the given locations
func (a *CraftAgent) findChaptersWithLocations(locationNames []string) []string {
	if a.outline == nil {
		return nil
	}

	var relevantChapters []string
	nameSet := make(map[string]bool)
	for _, name := range locationNames {
		nameSet[name] = true
	}

	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				// Check if location matches
				if nameSet[ch.Location] {
					chapterInfo := fmt.Sprintf("Chapter %s: %s\nSummary: %s", ch.ID, ch.Title, ch.Summary)
					relevantChapters = append(relevantChapters, chapterInfo)
					continue
				}

				// Also check in summary and title
				content := ch.Title + " " + ch.Summary
				for name := range nameSet {
					if strings.Contains(content, name) {
						chapterInfo := fmt.Sprintf("Chapter %s: %s\nSummary: %s", ch.ID, ch.Title, ch.Summary)
						relevantChapters = append(relevantChapters, chapterInfo)
						break
					}
				}
			}
		}
	}

	return relevantChapters
}

// findChaptersWithItems finds chapters that mention the given items
func (a *CraftAgent) findChaptersWithItems(itemNames []string) []string {
	if a.outline == nil {
		return nil
	}

	var relevantChapters []string
	nameSet := make(map[string]bool)
	for _, name := range itemNames {
		nameSet[name] = true
	}

	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				// Check in events for item-related events
				found := false
				for _, event := range ch.Events {
					if nameSet[event.Subject] {
						chapterInfo := fmt.Sprintf("Chapter %s: %s\nSummary: %s", ch.ID, ch.Title, ch.Summary)
						relevantChapters = append(relevantChapters, chapterInfo)
						found = true
						break
					}
				}
				if found {
					continue
				}

				// Also check in summary and title
				content := ch.Title + " " + ch.Summary
				for name := range nameSet {
					if strings.Contains(content, name) {
						chapterInfo := fmt.Sprintf("Chapter %s: %s\nSummary: %s", ch.ID, ch.Title, ch.Summary)
						relevantChapters = append(relevantChapters, chapterInfo)
						break
					}
				}
			}
		}
	}

	return relevantChapters
}
