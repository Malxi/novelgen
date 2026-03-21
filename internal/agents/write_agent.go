package agents

import (
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/prompts"
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
		"story_genre":     strings.Join(a.setup.Genres, ", "),
		"story_style":     a.setup.Tone,
		"chapter_id":      chapter.ID,
		"chapter_title":   chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":      strings.Join(chapter.Characters, ", "),
		"location":        chapter.Location,
		"context":         a.formatContext(context),
		"recap":           context.Recap,
		"state_matrix":    prompts.FormatStateMatrix(state, chapter),
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
		"story_genre":     strings.Join(a.setup.Genres, ", "),
		"story_style":     a.setup.Tone,
		"chapter_id":      chapter.ID,
		"chapter_title":   chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":      strings.Join(chapter.Characters, ", "),
		"location":        chapter.Location,
		"context":         a.formatContext(context),
		"recap":           context.Recap,
		"state_matrix":    prompts.FormatStateMatrix(state, chapter),
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

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate improved chapter: %w", err)
	}

	return response.Content, nil
}

// formatContext formats the chapter context for the prompt
func (a *WriteAgent) formatContext(context *ChapterContext) string {
	// Convert ChapterContext to prompts.ContextChapter slices
	previous := make([]*prompts.ContextChapter, len(context.Previous))
	for i, p := range context.Previous {
		previous[i] = &prompts.ContextChapter{
			Chapter: p.Chapter,
			Content: p.Content,
		}
	}
	next := make([]*prompts.ContextChapter, len(context.Next))
	for i, n := range context.Next {
		next[i] = &prompts.ContextChapter{
			Chapter: n.Chapter,
			Content: n.Content,
		}
	}
	return prompts.FormatChapterContext(previous, next, 500)
}
