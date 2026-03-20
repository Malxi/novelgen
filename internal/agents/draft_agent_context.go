package agents

import (
	"fmt"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// GenerateDraftWithContext generates a draft chapter with extra continuity context.
//
// recap is an optional, high-signal continuity anchor (typically extracted from
// the immediately previous chapter) that prompts can prioritize over the raw
// full-context text.
func (a *DraftAgent) GenerateDraftWithContext(chapter *models.Chapter, state *models.StateMatrix, targetWords int, contextText string, recap string) (string, error) {
	a.log.Info("Generating draft for chapter: %s (with context)", chapter.ID)

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
		"state_matrix":    a.formatStateMatrix(state, chapter),
		"target_words":    targetWords,
		"language":        a.language,
		"context":         contextText,
		"recap":           recap,
	}

	// Build prompts using PromptManager (reuse chapter_writing/default)
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
	if opts.MaxTokens < 4000 {
		opts.MaxTokens = 4000
	}

	response, err := a.client.ChatCompletion(messages, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate draft: %w", err)
	}

	return response.Content, nil
}
