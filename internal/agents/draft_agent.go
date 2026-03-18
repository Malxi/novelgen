package agents

import (
	"fmt"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// DraftAgent generates draft chapters based on story state
type DraftAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
	setup      *models.StorySetup
	outline    *models.Outline
	language   string
	log        logger.LoggerInterface
	pm         *prompts.PromptManager
}

// NewDraftAgent creates a new draft agent
func NewDraftAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline, language string) *DraftAgent {
	return &DraftAgent{
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

// StateMatrix represents the current state of the story
type StateMatrix struct {
	Characters    map[string]*models.Character
	Locations     map[string]*models.Location
	Items         map[string]*models.Item
	Relationships map[string]string
	Storylines    map[string]string
	Premises      map[string]string
}

// GenerateDraft generates a draft chapter
func (a *DraftAgent) GenerateDraft(chapter *models.Chapter, state *StateMatrix, targetWords int) (string, error) {
	a.log.Info("Generating draft for chapter: %s", chapter.ID)

	// Build prompt data
	data := map[string]interface{}{
		"story_title":    a.setup.ProjectName,
		"story_genre":    strings.Join(a.setup.Genres, ", "),
		"story_style":    a.setup.Tone,
		"chapter_id":     chapter.ID,
		"chapter_title":  chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":     strings.Join(chapter.Characters, ", "),
		"location":       chapter.Location,
		"state_matrix":   a.formatStateMatrix(state, chapter),
		"target_words":   targetWords,
		"language":       a.language,
	}

	// Build prompts using PromptManager
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillChapterWriting, "default", data)
	if err != nil {
		return "", fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call LLM
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)
	// Draft generation needs more tokens
	if opts.MaxTokens < 4000 {
		opts.MaxTokens = 4000
	}

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate draft: %w", err)
	}

	return response.Content, nil
}

// formatStateMatrix formats the state matrix for the prompt
func (a *DraftAgent) formatStateMatrix(state *StateMatrix, chapter *models.Chapter) string {
	var sb strings.Builder

	sb.WriteString("=== CURRENT STORY STATE ===\n\n")

	// Characters present in this chapter
	sb.WriteString("Characters in this chapter:\n")
	for _, charName := range chapter.Characters {
		if char, exists := state.Characters[charName]; exists {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", char.Name, char.RoleInStory))
			if char.Age != "" {
				sb.WriteString(fmt.Sprintf("  Age: %s\n", char.Age))
			}
			if len(char.Personality) > 0 {
				sb.WriteString(fmt.Sprintf("  Personality: %s\n", strings.Join(char.Personality, ", ")))
			}
			if char.Motivation != "" {
				sb.WriteString(fmt.Sprintf("  Motivation: %s\n", char.Motivation))
			}
			if len(char.Goals) > 0 {
				sb.WriteString(fmt.Sprintf("  Current Goals: %s\n", strings.Join(char.Goals, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	// Location
	if chapter.Location != "" {
		sb.WriteString(fmt.Sprintf("Location: %s\n", chapter.Location))
		if loc, exists := state.Locations[chapter.Location]; exists {
			sb.WriteString(fmt.Sprintf("  Description: %s\n", loc.Description))
			sb.WriteString(fmt.Sprintf("  Atmosphere: %s\n", loc.Atmosphere))
		}
		sb.WriteString("\n")
	}

	// Active storylines
	if len(state.Storylines) > 0 {
		sb.WriteString("Active Storylines:\n")
		for storyline, status := range state.Storylines {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", storyline, status))
		}
		sb.WriteString("\n")
	}

	// Character relationships
	if len(state.Relationships) > 0 {
		sb.WriteString("Key Relationships:\n")
		for pair, relation := range state.Relationships {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", pair, relation))
		}
		sb.WriteString("\n")
	}

	// Character premise states
	if len(state.Premises) > 0 {
		sb.WriteString("Character Progression:\n")
		for key, progress := range state.Premises {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", key, progress))
		}
		sb.WriteString("\n")
	}

	// Items relevant to characters in this chapter
	relevantItems := []string{}
	for itemName, item := range state.Items {
		if item.Owner != "" {
			for _, charName := range chapter.Characters {
				if item.Owner == charName {
					relevantItems = append(relevantItems, fmt.Sprintf("%s (owned by %s)", itemName, charName))
					break
				}
			}
		}
	}
	if len(relevantItems) > 0 {
		sb.WriteString("Relevant Items:\n")
		for _, item := range relevantItems {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
		sb.WriteString("\n")
	}

	// Chapter events to cover
	if len(chapter.Events) > 0 {
		sb.WriteString("Events to cover in this chapter:\n")
		for _, event := range chapter.Events {
			sb.WriteString(fmt.Sprintf("- [%s] ", event.Type))
			if len(event.Characters) > 0 {
				sb.WriteString(fmt.Sprintf("Characters: %s, ", strings.Join(event.Characters, ", ")))
			}
			if event.Subject != "" {
				sb.WriteString(fmt.Sprintf("Subject: %s, ", event.Subject))
			}
			sb.WriteString(fmt.Sprintf("Change: %s\n", event.Change))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== END STATE ===\n")

	return sb.String()
}
