package agents

import (
	"fmt"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// WriteAgent generates final chapter content with continuity
type WriteAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
	setup      *models.StorySetup
	outline    *models.Outline
	language   string
	log        logger.LoggerInterface
	pm         *prompts.PromptManager
}

// ChapterContext holds surrounding chapter information for continuity
type ChapterContext struct {
	Previous []*ContextChapter
	Current  *models.Chapter
	Next     []*ContextChapter
	Recap    string
}

// ContextChapter represents a chapter with its content
type ContextChapter struct {
	Chapter *models.Chapter
	Content string
}

// NewWriteAgent creates a new write agent
func NewWriteAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline, language string) *WriteAgent {
	return &WriteAgent{
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

// GenerateChapter generates final chapter content with continuity
func (a *WriteAgent) GenerateChapter(chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, targetWords int) (string, error) {
	a.log.Info("Generating final content for chapter: %s", chapter.ID)

	// Build prompt data
	data := map[string]interface{}{
		"story_title":     a.setup.ProjectName,
		"story_genre":     strings.Join(a.setup.Genres, ", "),
		"story_style":     a.setup.Tone,
		"chapter_id":      chapter.ID,
		"chapter_title":   chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":      strings.Join(chapter.Characters, ", "),
		"location":        chapter.Location,
		"context":         a.formatContext(context),
		"recap":           context.Recap,
		"state_matrix":    a.formatStateMatrix(state, chapter),
		"target_words":    targetWords,
		"language":        a.language,
	}

	// Build prompts using PromptManager
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillChapterWriting, "final", data)
	if err != nil {
		return "", fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call LLM
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)
	// Final chapter generation needs more tokens
	if opts.MaxTokens < 6000 {
		opts.MaxTokens = 6000
	}

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate chapter: %w", err)
	}

	return response.Content, nil
}

// GenerateChapterWithSuggestions generates improved chapter content with review suggestions
func (a *WriteAgent) GenerateChapterWithSuggestions(chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, targetWords int, suggestions string) (string, error) {
	a.log.Info("Generating improved content for chapter: %s", chapter.ID)

	// Build prompt data
	data := map[string]interface{}{
		"story_title":     a.setup.ProjectName,
		"story_genre":     strings.Join(a.setup.Genres, ", "),
		"story_style":     a.setup.Tone,
		"chapter_id":      chapter.ID,
		"chapter_title":   chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":      strings.Join(chapter.Characters, ", "),
		"location":        chapter.Location,
		"context":         a.formatContext(context),
		"recap":           context.Recap,
		"state_matrix":    a.formatStateMatrix(state, chapter),
		"target_words":    targetWords,
		"suggestions":     suggestions,
		"language":        a.language,
	}

	// Build prompts using PromptManager with improve template
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillChapterWriting, "improve", data)
	if err != nil {
		return "", fmt.Errorf("failed to build prompt: %w", err)
	}

	// Call LLM
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)
	// Final chapter generation needs more tokens
	if opts.MaxTokens < 6000 {
		opts.MaxTokens = 6000
	}

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate improved chapter: %w", err)
	}

	return response.Content, nil
}

// formatContext formats the chapter context for the prompt
func (a *WriteAgent) formatContext(context *ChapterContext) string {
	var sb strings.Builder

	sb.WriteString("=== CHAPTER CONTEXT ===\n\n")

	// Previous chapters
	if len(context.Previous) > 0 {
		sb.WriteString("PREVIOUS CHAPTERS:\n")
		for _, prev := range context.Previous {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", prev.Chapter.ID, prev.Chapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", prev.Chapter.Summary))
			// Include a snippet of the draft content (first 500 chars)
			content := prev.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("Content:\n%s\n", content))
		}
		sb.WriteString("\n")
	}

	// Next chapters (for foreshadowing)
	if len(context.Next) > 0 {
		sb.WriteString("UPCOMING CHAPTERS (for foreshadowing):\n")
		for _, next := range context.Next {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", next.Chapter.ID, next.Chapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", next.Chapter.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== END CONTEXT ===\n")

	return sb.String()
}

// formatStateMatrix formats the state matrix for the prompt
func (a *WriteAgent) formatStateMatrix(state *models.StateMatrix, chapter *models.Chapter) string {
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

	sb.WriteString("=== END STORY STATE ===\n")

	return sb.String()
}
