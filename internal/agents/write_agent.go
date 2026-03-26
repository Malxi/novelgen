package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// WriteGenInput is the input for final chapter generation
type WriteGenInput struct {
	StorySetup   models.StorySetup `md:"story_setup"`
	Chapter      models.Chapter    `md:"chapter"`
	StateMatrix  string            `md:"state_matrix"`
	TargetWords  int               `md:"target_words"`
	Context      string            `md:"context,omitempty"`
	Recap        string            `md:"recap,omitempty"`
	NextChapters []NextChapterInfo `md:"next_chapters,omitempty"`
}

// WriteGenOutput is the output for final chapter generation
type WriteGenOutput struct {
	Content string `md:"content"`
}

// WriteImproveInput is the input for chapter improvement
type WriteImproveInput struct {
	StorySetup   models.StorySetup `md:"story_setup"`
	Chapter      models.Chapter    `md:"chapter"`
	StateMatrix  string            `md:"state_matrix"`
	TargetWords  int               `md:"target_words"`
	CurrentDraft string            `md:"current_draft"`
	Suggestions  string            `md:"suggestions"`
	Context      string            `md:"context,omitempty"`
	Recap        string            `md:"recap,omitempty"`
}

// WriteImproveOutput is the output for chapter improvement
type WriteImproveOutput struct {
	Content string `md:"content"`
}

// WriteReviewInput is the input for chapter review
type WriteReviewInput struct {
	StorySetup     models.StorySetup `md:"story_setup"`
	Chapter        models.Chapter    `md:"chapter"`
	ChapterContent string            `md:"chapter_content"`
	TargetWords    int               `md:"target_words"`
	Iteration      int               `md:"iteration"`
}

// WriteReviewOutput is the output for chapter review
type WriteReviewOutput struct {
	Result models.ReviewResult `md:"review_result"`
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

// WriteAgent generates final chapter content with continuity
// It wraps BaseAgent to provide type-safe methods
type WriteAgent struct {
	base    *BaseAgent
	setup   *models.StorySetup
	outline *models.Outline
}

// NewWriteAgent creates a new WriteAgent
func NewWriteAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline) *WriteAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "WriteAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &WriteAgent{
		base:    base,
		setup:   setup,
		outline: outline,
	}
}

// SetLanguage sets the output language
func (a *WriteAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// GenerateChapter generates final chapter content with continuity
func (a *WriteAgent) GenerateChapter(ctx context.Context, chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, targetWords int) (string, error) {
	logger.Section("WRITE AGENT - Final Chapter Generation")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	// Convert next chapters to info
	var nextInfos []NextChapterInfo
	for _, nc := range context.Next {
		nextInfos = append(nextInfos, NextChapterInfo{
			ID:      nc.Chapter.ID,
			Title:   nc.Chapter.Title,
			Summary: nc.Chapter.Summary,
		})
	}

	input := WriteGenInput{
		StorySetup:   *a.setup,
		Chapter:      *chapter,
		StateMatrix:  formatStateMatrixForWrite(state, chapter),
		TargetWords:  targetWords,
		Context:      formatChapterContext(context),
		Recap:        context.Recap,
		NextChapters: nextInfos,
	}

	var output WriteGenOutput
	params := InvokeParams{
		Skills:  []string{"write-generate"},
		Command: "generate final chapter content",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	// Log context for debugging
	if err := a.logWriteContext(chapter.ID, "final", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("✓ Generated final chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// GenerateChapterWithSuggestions generates improved chapter content with review suggestions
func (a *WriteAgent) GenerateChapterWithSuggestions(ctx context.Context, chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, targetWords int, suggestions string) (string, error) {
	logger.Section("WRITE AGENT - Chapter Improvement")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	input := WriteImproveInput{
		StorySetup:  *a.setup,
		Chapter:     *chapter,
		StateMatrix: formatStateMatrixForWrite(state, chapter),
		TargetWords: targetWords,
		Suggestions: suggestions,
		Context:     formatChapterContext(context),
		Recap:       context.Recap,
	}

	var output WriteImproveOutput
	params := InvokeParams{
		Skills:  []string{"write-improve"},
		Command: "improve chapter based on suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	// Log context for debugging
	if err := a.logWriteContext(chapter.ID, "improve", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("✓ Generated improved chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// ReviewChapter reviews a chapter and provides improvement suggestions
func (a *WriteAgent) ReviewChapter(ctx context.Context, chapter *models.Chapter, content string, targetWords int, iteration int) (models.ReviewResult, error) {
	logger.Section("WRITE AGENT - Chapter Review")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Iteration: %d", iteration)
	logger.Info("Language: %s", a.base.language)

	input := WriteReviewInput{
		StorySetup:     *a.setup,
		Chapter:        *chapter,
		ChapterContent: content,
		TargetWords:    targetWords,
		Iteration:      iteration,
	}

	var output WriteReviewOutput
	params := InvokeParams{
		Skills:  []string{"write-review"},
		Command: "review chapter and provide improvement suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return models.ReviewResult{}, err
	}

	logger.Section("Chapter Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	logger.Info("Suggestions: %d", len(output.Result.Suggestions))

	return output.Result, nil
}

// IterateChapter runs the review-improvement loop for a chapter
func (a *WriteAgent) IterateChapter(ctx context.Context, chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, targetWords int, initialContent string, maxIterations int, qualityThreshold float64) (string, *models.ReviewResult, error) {
	logger.Section("WRITE AGENT - Chapter Iteration Loop")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)

	currentContent := initialContent
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review
		review, err := a.ReviewChapter(ctx, chapter, currentContent, targetWords, i)
		if err != nil {
			return "", nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}
		finalReview = &review

		// Check if quality meets threshold
		if review.OverallScore >= qualityThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", review.OverallScore, qualityThreshold)
			break
		}

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}

		// Check if there are high priority suggestions
		hasHighPriority := false
		for _, s := range review.Suggestions {
			if s.Priority == "high" {
				hasHighPriority = true
				break
			}
		}
		if !hasHighPriority {
			logger.Info("No high priority issues, stopping iteration")
			break
		}

		// Improve
		suggestions := formatWriteSuggestions(review.Suggestions)
		improved, err := a.GenerateChapterWithSuggestions(ctx, chapter, context, state, targetWords, suggestions)
		if err != nil {
			return "", nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}
		currentContent = improved
	}

	return currentContent, finalReview, nil
}

// logWriteContext logs the write context to a markdown file for debugging
func (a *WriteAgent) logWriteContext(chapterID, variant string, input interface{}, output string) error {
	debugDir := filepath.Join("logs", "write_contexts")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%s_%s.md", chapterID, variant, timestamp))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Write Context: %s (%s)\n\n", chapterID, variant))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## Input\n\n")
	sb.WriteString("```json\n")
	// Simple representation
	sb.WriteString(fmt.Sprintf("%+v", input))
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Output\n\n")
	sb.WriteString("```\n")
	sb.WriteString(output)
	sb.WriteString("\n```\n")

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

// formatChapterContext formats the chapter context for the prompt
func formatChapterContext(context *ChapterContext) string {
	var sb strings.Builder

	if len(context.Previous) > 0 {
		sb.WriteString("PREVIOUS CHAPTERS:\n")
		for _, prev := range context.Previous {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", prev.Chapter.ID, prev.Chapter.Title))
			if len(prev.Content) > 500 {
				sb.WriteString(prev.Content[:500])
				sb.WriteString("...")
			} else {
				sb.WriteString(prev.Content)
			}
			sb.WriteString("\n")
		}
	}

	if len(context.Next) > 0 {
		sb.WriteString("\nUPCOMING CHAPTERS (for foreshadowing):\n")
		for _, next := range context.Next {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", next.Chapter.ID, next.Chapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", next.Chapter.Summary))
		}
	}

	return sb.String()
}

// formatStateMatrixForWrite formats the state matrix for the prompt
func formatStateMatrixForWrite(state *models.StateMatrix, chapter *models.Chapter) string {
	if state == nil {
		return "No state matrix available"
	}

	var sb strings.Builder
	sb.WriteString("CURRENT STORY STATE:\n")

	// Character states
	if len(state.Characters) > 0 {
		sb.WriteString("\nCharacters:\n")
		for name, char := range state.Characters {
			sb.WriteString(fmt.Sprintf("  %s:\n", name))
			if char.Motivation != "" {
				sb.WriteString(fmt.Sprintf("    - Motivation: %s\n", char.Motivation))
			}
			if char.Personality != nil && len(char.Personality) > 0 {
				sb.WriteString(fmt.Sprintf("    - Personality: %s\n", strings.Join(char.Personality, ", ")))
			}
			if char.Background != "" {
				sb.WriteString(fmt.Sprintf("    - Background: %s\n", char.Background))
			}
		}
	}

	// Active relationships
	if len(state.Relationships) > 0 {
		sb.WriteString("\nActive Relationships:\n")
		for relKey, relStatus := range state.Relationships {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", relKey, relStatus))
		}
	}

	// Active storylines
	if len(state.Storylines) > 0 {
		sb.WriteString("\nActive Storylines:\n")
		for _, sl := range state.Storylines {
			if sl.Status == "active" || sl.Status == "in_progress" {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", sl.Name, sl.Progress))
			}
		}
	}

	return sb.String()
}

// formatWriteSuggestions formats review suggestions for the improvement prompt
func formatWriteSuggestions(suggestions []models.ReviewSuggestion) string {
	var sb strings.Builder
	for i, s := range suggestions {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, s.Priority, s.Issue))
		sb.WriteString(fmt.Sprintf("   Suggestion: %s\n\n", s.Suggestion))
	}
	return sb.String()
}
