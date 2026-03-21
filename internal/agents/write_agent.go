package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// Log context to file for debugging
	if err := a.logWriteContext(chapter.ID, "final", systemPrompt, userPrompt); err != nil {
		a.log.Warn("Failed to log write context: %v", err)
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

// logWriteContext logs the write context to a markdown file for debugging
func (a *WriteAgent) logWriteContext(chapterID, variant, systemPrompt, userPrompt string) error {
	debugDir := filepath.Join("logs", "write_contexts")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%s_%s.md", chapterID, variant, timestamp))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Write Context: %s (%s)\n\n", chapterID, variant))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## System Prompt\n\n")
	sb.WriteString("```\n")
	sb.WriteString(systemPrompt)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## User Prompt\n\n")
	sb.WriteString("```\n")
	sb.WriteString(userPrompt)
	sb.WriteString("\n```\n")

	return os.WriteFile(filename, []byte(sb.String()), 0644)
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

	// Log context to file for debugging
	if err := a.logWriteContext(chapter.ID, "improve", systemPrompt, userPrompt); err != nil {
		a.log.Warn("Failed to log write context: %v", err)
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
	// Use 0 to indicate full content (no truncation)
	return prompts.FormatChapterContext(previous, next, 0)
}
