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

// GenerateDraft generates a draft chapter
func (a *DraftAgent) GenerateDraft(chapter *models.Chapter, state *models.StateMatrix, targetWords int) (string, error) {
	a.log.Info("Generating draft for chapter: %s", chapter.ID)

	// Build prompt data
	data := map[string]interface{}{
		"story_genre":     strings.Join(a.setup.Genres, ", "),
		"story_style":     a.setup.Tone,
		"story_title":     a.setup.ProjectName,
		"chapter_id":      chapter.ID,
		"chapter_title":   chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":      strings.Join(chapter.Characters, ", "),
		"location":        chapter.Location,
		"state_matrix":    prompts.FormatStateMatrix(state, chapter),
		"target_words":    targetWords,
		"language":        a.language,
		"language_name":   prompts.GetLanguageName(a.language),
		"tense":           a.setup.Tense,
		"pov_style":       a.setup.POVStyle,
	}

	// Build prompts using PromptManager
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillChapterWriting, "default", data)
	if err != nil {
		return "", fmt.Errorf("failed to build prompt: %w", err)
	}

	// Log context to file for debugging
	if err := a.logDraftContext(chapter.ID, systemPrompt, userPrompt); err != nil {
		a.log.Warn("Failed to log draft context: %v", err)
	}

	// Call LLM
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate draft: %w", err)
	}

	return response.Content, nil
}

// logDraftContext logs the draft context to a markdown file for debugging
func (a *DraftAgent) logDraftContext(chapterID, systemPrompt, userPrompt string) error {
	debugDir := filepath.Join("logs", "draft_contexts")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%s.md", chapterID, timestamp))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Draft Context: %s\n\n", chapterID))
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

// GenerateDraftWithSuggestions generates a draft chapter with improvement suggestions
func (a *DraftAgent) GenerateDraftWithSuggestions(chapter *models.Chapter, state *models.StateMatrix, targetWords int, suggestions string, contextText string, recap string, nextChapters []*models.Chapter) (string, error) {
	a.log.Info("Generating improved draft for chapter: %s with suggestions", chapter.ID)

	// Build next chapters context for foreshadowing
	var nextContext string
	if len(nextChapters) > 0 {
		var sb strings.Builder
		sb.WriteString("UPCOMING CHAPTERS (for foreshadowing):\n")
		for _, next := range nextChapters {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", next.ID, next.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", next.Summary))
		}
		nextContext = sb.String()
	}

	// Build prompt data
	data := map[string]interface{}{
		"story_genre":     strings.Join(a.setup.Genres, ", "),
		"story_style":     a.setup.Tone,
		"story_title":     a.setup.ProjectName,
		"chapter_id":      chapter.ID,
		"chapter_title":   chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":      strings.Join(chapter.Characters, ", "),
		"location":        chapter.Location,
		"state_matrix":    prompts.FormatStateMatrix(state, chapter),
		"target_words":    targetWords,
		"language":        a.language,
		"language_name":   prompts.GetLanguageName(a.language),
		"tense":           a.setup.Tense,
		"pov_style":       a.setup.POVStyle,
		"suggestions":     suggestions,
		"context":         contextText,
		"recap":           recap,
		"next_chapters":   nextContext,
	}

	// Build prompts using PromptManager
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillChapterWriting, "iterate", data)
	if err != nil {
		// Fallback to default if iterate template doesn't exist
		systemPrompt, userPrompt, err = a.pm.Build(prompts.SkillChapterWriting, "default", data)
		if err != nil {
			return "", fmt.Errorf("failed to build prompt: %w", err)
		}
		// Add suggestions to user prompt
		userPrompt += "\n\n## 改进建议\n" + suggestions + "\n\n请根据以上建议改进本章内容。"
	}

	// Log context to file for debugging
	if err := a.logDraftContext(chapter.ID+"_improved", systemPrompt, userPrompt); err != nil {
		a.log.Warn("Failed to log draft context: %v", err)
	}

	// Call LLM
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate improved draft: %w", err)
	}

	return response.Content, nil
}

// GenerateDraftWithContext generates a draft chapter with extra continuity context.
//
// recap is an optional, high-signal continuity anchor (typically extracted from
// the immediately previous chapter) that prompts can prioritize over the raw
// full-context text.
//
// nextChapters contains upcoming chapters for foreshadowing purposes.
func (a *DraftAgent) GenerateDraftWithContext(chapter *models.Chapter, state *models.StateMatrix, targetWords int, contextText string, recap string, nextChapters []*models.Chapter) (string, error) {
	a.log.Info("Generating draft for chapter: %s (with context)", chapter.ID)

	// Build next chapters context for foreshadowing
	var nextContext string
	if len(nextChapters) > 0 {
		var sb strings.Builder
		sb.WriteString("UPCOMING CHAPTERS (for foreshadowing):\n")
		for _, next := range nextChapters {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", next.ID, next.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", next.Summary))
		}
		nextContext = sb.String()
	}

	// Build prompt data
	data := map[string]interface{}{
		"story_genre":     strings.Join(a.setup.Genres, ", "),
		"story_style":     a.setup.Tone,
		"story_title":     a.setup.ProjectName,
		"chapter_id":      chapter.ID,
		"chapter_title":   chapter.Title,
		"chapter_summary": chapter.Summary,
		"characters":      strings.Join(chapter.Characters, ", "),
		"location":        chapter.Location,
		"state_matrix":    prompts.FormatStateMatrix(state, chapter),
		"target_words":    targetWords,
		"language":        a.language,
		"language_name":   prompts.GetLanguageName(a.language),
		"tense":           a.setup.Tense,
		"pov_style":       a.setup.POVStyle,
		"context":         contextText,
		"recap":           recap,
		"next_chapters":   nextContext,
	}

	// Build prompts using PromptManager (reuse chapter_writing/default)
	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillChapterWriting, "default", data)
	if err != nil {
		return "", fmt.Errorf("failed to build prompt: %w", err)
	}

	// Log context to file for debugging
	if err := a.logDraftContext(chapter.ID+"_context", systemPrompt, userPrompt); err != nil {
		a.log.Warn("Failed to log draft context: %v", err)
	}

	// Call LLM
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	opts := a.config.GetChatOptions(a.projectLLM)

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate draft: %w", err)
	}

	return response.Content, nil
}
