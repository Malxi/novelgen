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
func (a *WriteAgent) GenerateChapter(chapter *models.Chapter, context *ChapterContext, targetWords int) (string, error) {
	a.log.Info("Generating final content for chapter: %s", chapter.ID)

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
		"context":        a.formatContext(context),
		"target_words":   targetWords,
		"language":       a.language,
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
