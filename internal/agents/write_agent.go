package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
)

const compactStyleReferenceRuneLimit = 2000

// CompactStorySetup is a minimal version of StorySetup for chapter generation
// Only includes essential fields needed for writing
type CompactStorySetup struct {
	Genres       []string            `json:"genres" md:"genres" desc:"Story genres (2-4 specific genres)"`
	Premise      string              `json:"premise" md:"premise" desc:"Core story premise (2-4 sentences)"`
	Theme        string              `json:"theme" md:"theme" desc:"Story theme (clear statement)"`
	Rules        []string            `json:"rules" md:"rules" desc:"World rules (3-7 enforceable rules)"`
	Tone         string              `json:"tone" md:"tone" desc:"Writing tone (2-4 adjectives, comma-separated)"`
	Tense        string              `json:"tense" md:"tense" desc:"Narrative tense (past or present)"`
	POVStyle     string              `json:"pov_style" md:"pov_style" desc:"POV style (first person, third person limited, or third person omniscient)"`
	WritingStyle models.WritingStyle `json:"writing_style,omitempty" md:"writing_style,omitempty" desc:"Optional prose style instructions and reference excerpt; use as style signal only, not story facts"`
}

// ToCompact converts a full StorySetup to CompactStorySetup
func ToCompact(setup *models.StorySetup) CompactStorySetup {
	if setup == nil {
		return CompactStorySetup{}
	}
	return CompactStorySetup{
		Genres:       setup.Genres,
		Premise:      setup.Premise,
		Theme:        setup.Theme,
		Rules:        setup.Rules,
		Tone:         setup.Tone,
		Tense:        setup.Tense,
		POVStyle:     setup.POVStyle,
		WritingStyle: setup.WritingStyle.CompactReference(compactStyleReferenceRuneLimit),
	}
}

// WriteGenInput is the input for final chapter generation
type WriteGenInput struct {
	StorySetup   CompactStorySetup `json:"story_setup" md:"story_setup" desc:"Core story setup including premise, genres, themes, rules"`
	Chapter      models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	StateMatrix  string            `json:"state_matrix" md:"state_matrix" desc:"Current story state including character statuses, relationships, goals"`
	TargetWords  int               `json:"target_words" md:"target_words" desc:"Target word count for the chapter"`
	Context      string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap        string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter"`
	NextChapters []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing"`
}

// WriteGenOutput is the output for final chapter generation
type WriteGenOutput struct {
	Content string `json:"content" md:"content" desc:"The final polished chapter content"`
}

// WriteImproveInput is the input for chapter improvement
type WriteImproveInput struct {
	StorySetup   CompactStorySetup `json:"story_setup" md:"story_setup" desc:"Core story setup including premise, genres, themes, rules"`
	Chapter      models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	StateMatrix  string            `json:"state_matrix" md:"state_matrix" desc:"Current story state including character statuses, relationships, goals"`
	TargetWords  int               `json:"target_words" md:"target_words" desc:"Target word count for the chapter"`
	Iteration    int               `json:"iteration" md:"iteration" desc:"Current improvement iteration number"`
	CurrentDraft string            `json:"current_draft" md:"current_draft" desc:"The current draft content to be improved"`
	Suggestions  string            `json:"suggestions" md:"suggestions" desc:"Review suggestions for improvement"`
	Context      string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap        string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter"`
	NextChapters []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing"`
}

// WriteImproveOutput is the output for chapter improvement
type WriteImproveOutput struct {
	Content string `json:"content" md:"content" desc:"The improved chapter content"`
}

// WriteReviewInput is the input for chapter review
type WriteReviewInput struct {
	StorySetup     CompactStorySetup `json:"story_setup" md:"story_setup" desc:"Core story setup including premise, genres, themes, rules"`
	StateMatrix    string            `json:"state_matrix" md:"state_matrix" desc:"Current story state including character statuses, relationships, goals"`
	Chapter        models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	ChapterContent string            `json:"chapter_content" md:"chapter_content" desc:"The chapter content to be reviewed"`
	TargetWords    int               `json:"target_words" md:"target_words" desc:"Target word count for the chapter"`
	Iteration      int               `json:"iteration" md:"iteration" desc:"Current iteration number"`
	Context        string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap          string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter for continuity checking"`
	NextChapters   []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing and hook checks"`
}

// WriteReviewOutput is the output for chapter review
type WriteReviewOutput struct {
	Result models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result including scores and improvement suggestions"`
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

// VolumeReviewChapterInput represents a chapter in volume review
type VolumeReviewChapterInput struct {
	ChapterID    string   `json:"chapter_id" md:"chapter_id"`
	ChapterTitle string   `json:"chapter_title" md:"chapter_title"`
	Summary      string   `json:"summary" md:"summary"`
	Content      string   `json:"content" md:"content"`
	Beats        []string `json:"beats" md:"beats"`
}

// VolumeReviewInput is the input for volume-level review
type VolumeReviewInput struct {
	StorySetup            CompactStorySetup          `json:"story_setup" md:"story_setup"`
	VolumeID              string                     `json:"volume_id" md:"volume_id"`
	VolumeTitle           string                     `json:"volume_title" md:"volume_title"`
	VolumeSummary         string                     `json:"volume_summary" md:"volume_summary"`
	Chapters              []VolumeReviewChapterInput `json:"chapters" md:"chapters"`
	TargetWordsPerChapter int                        `json:"target_words_per_chapter" md:"target_words_per_chapter"`
}

// VolumeChapterReview represents review result for a single chapter in volume review
type VolumeChapterReview struct {
	ChapterID          string   `json:"chapter_id" md:"chapter_id" desc:"Chapter ID"`
	ChapterScore       float64  `json:"chapter_score" md:"chapter_score" desc:"Chapter score 0-10, can be decimal like 8.5"`
	ChapterRole        string   `json:"chapter_role" md:"chapter_role" desc:"Chapter role in volume: setup/development/turning_point/climax/resolution"`
	ContinuityWithPrev string   `json:"continuity_with_previous" md:"continuity_with_previous" desc:"Continuity evaluation with previous chapter"`
	ContinuityWithNext string   `json:"continuity_with_next" md:"continuity_with_next" desc:"Continuity evaluation with next chapter"`
	Issues             []string `json:"issues" md:"issues" desc:"List of issues found in this chapter"`
	Suggestions        []string `json:"suggestions" md:"suggestions" desc:"List of improvement suggestions for this chapter"`
}

// VolumeStructureAnalysis represents the volume structure analysis
type VolumeStructureAnalysis struct {
	OpeningHook  string `json:"opening_hook" md:"opening_hook" desc:"Evaluation of the opening hook"`
	RisingAction string `json:"rising_action" md:"rising_action" desc:"Evaluation of rising action distribution"`
	Climax       string `json:"climax" md:"climax" desc:"Evaluation of the climax"`
	Resolution   string `json:"resolution" md:"resolution" desc:"Evaluation of the resolution"`
}

// VolumeReviewResult represents the result of volume-level review
type VolumeReviewResult struct {
	OverallScore            int                     `json:"overall_score" md:"overall_score" desc:"Overall volume quality score 0-100"`
	VolumeStructureAnalysis VolumeStructureAnalysis `json:"volume_structure_analysis" md:"volume_structure_analysis" desc:"Analysis of volume structure: opening, rising action, climax, resolution"`
	ChapterReviews          []VolumeChapterReview   `json:"chapter_reviews" md:"chapter_reviews" desc:"Review results for each chapter"`
	VolumeLevelIssues       []string                `json:"volume_level_issues" md:"volume_level_issues" desc:"Issues that affect the entire volume"`
	VolumeLevelSuggestions  []string                `json:"volume_level_suggestions" md:"volume_level_suggestions" desc:"Suggestions for improving the entire volume"`
}

// VolumeReviewOutput is the output for volume review
type VolumeReviewOutput struct {
	Result VolumeReviewResult `json:"volume_review_result" md:"volume_review_result"`
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

	nextInfos := buildNextChapterInfos(context)
	recap := ""
	if context != nil {
		recap = context.Recap
	}

	input := WriteGenInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		StateMatrix:  formatStateMatrixForWrite(state, chapter),
		TargetWords:  targetWords,
		Context:      formatChapterContext(context),
		Recap:        recap,
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

	if err := validateWriteContent(chapter, output.Content, targetWords); err != nil {
		return "", err
	}
	if warn := validateWriteTargetLength(output.Content, targetWords); warn != "" {
		logger.Warn("Chapter %s length warning: %s", chapter.ID, warn)
	}

	// Log context for debugging
	if err := a.logWriteContext(chapter.ID, "final", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("✓ Generated final chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// GenerateChapterWithSuggestions generates improved chapter content with review suggestions
func (a *WriteAgent) GenerateChapterWithSuggestions(ctx context.Context, chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, targetWords int, currentDraft string, suggestions string, iteration ...int) (string, error) {
	logger.Section("WRITE AGENT - Chapter Improvement")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	iter := 1
	if len(iteration) > 0 && iteration[0] > 0 {
		iter = iteration[0]
	}

	input := WriteImproveInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		StateMatrix:  formatStateMatrixForWrite(state, chapter),
		TargetWords:  targetWords,
		Iteration:    iter,
		CurrentDraft: currentDraft,
		Suggestions:  suggestions,
		Context:      formatChapterContext(context),
		Recap:        recapForContext(context),
		NextChapters: buildNextChapterInfos(context),
	}

	var output WriteImproveOutput
	params := InvokeParams{
		Skills:  []string{"write-improve"},
		Command: "improve chapter based on suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	if err := validateWriteContent(chapter, output.Content, targetWords); err != nil {
		return "", err
	}
	if warn := validateWriteTargetLength(output.Content, targetWords); warn != "" {
		logger.Warn("Chapter %s improvement length warning: %s", chapter.ID, warn)
	}

	// Log context for debugging
	if err := a.logWriteContext(chapter.ID, "improve", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("✓ Generated improved chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// ReviewChapter reviews a chapter and provides improvement suggestions
func (a *WriteAgent) ReviewChapter(ctx context.Context, chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, content string, targetWords int, iteration int) (models.ReviewResult, error) {
	logger.Section("WRITE AGENT - Chapter Review")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Iteration: %d", iteration)
	logger.Info("Language: %s", a.base.language)

	recap := ""
	contextText := ""
	nextInfos := []NextChapterInfo(nil)
	if context != nil {
		recap = context.Recap
		contextText = formatChapterContext(context)
		nextInfos = buildNextChapterInfos(context)
	}

	input := WriteReviewInput{
		StorySetup:     ToCompact(a.setup),
		StateMatrix:    formatStateMatrixForWrite(state, chapter),
		Chapter:        *chapter,
		ChapterContent: content,
		TargetWords:    targetWords,
		Iteration:      iteration,
		Context:        contextText,
		Recap:          recap,
		NextChapters:   nextInfos,
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

// ReviewVolume performs a holistic review of all chapters in a volume
func (a *WriteAgent) ReviewVolume(ctx context.Context, volume *models.Volume, chapters []*models.Chapter, chapterContents map[string]string, targetWords int) (VolumeReviewResult, error) {
	logger.Section("WRITE AGENT - Volume Review")
	logger.Info("Volume: %s - %s", volume.ID, volume.Title)
	logger.Info("Chapters: %d", len(chapters))
	logger.Info("Language: %s", a.base.language)

	// Build chapter inputs
	var chapterInputs []VolumeReviewChapterInput
	for _, chapter := range chapters {
		content := chapterContents[chapter.ID]
		if content == "" {
			logger.Warn("No content for chapter %s, skipping in volume review", chapter.ID)
			continue
		}

		// Beats are already strings
		beats := chapter.GetBeats()

		chapterInputs = append(chapterInputs, VolumeReviewChapterInput{
			ChapterID:    chapter.ID,
			ChapterTitle: chapter.Title,
			Summary:      chapter.Summary,
			Content:      content,
			Beats:        beats,
		})
	}

	if len(chapterInputs) == 0 {
		return VolumeReviewResult{}, fmt.Errorf("no chapters with content to review")
	}

	input := VolumeReviewInput{
		StorySetup:            ToCompact(a.setup),
		VolumeID:              volume.ID,
		VolumeTitle:           volume.Title,
		VolumeSummary:         volume.Summary,
		Chapters:              chapterInputs,
		TargetWordsPerChapter: targetWords,
	}

	var output VolumeReviewOutput
	params := InvokeParams{
		Skills:  []string{"volume-review"},
		Command: "review entire volume holistically and provide chapter-level suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return VolumeReviewResult{}, err
	}

	logger.Section("Volume Review Result")
	logger.Info("Overall Volume Score: %d/100", output.Result.OverallScore)
	logger.Info("Chapters Reviewed: %d", len(output.Result.ChapterReviews))
	logger.Info("Volume Level Issues: %d", len(output.Result.VolumeLevelIssues))
	logger.Info("Volume Level Suggestions: %d", len(output.Result.VolumeLevelSuggestions))

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
		review, err := a.ReviewChapter(ctx, chapter, context, state, currentContent, targetWords, i)
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
		improved, err := a.GenerateChapterWithSuggestions(ctx, chapter, context, state, targetWords, currentContent, suggestions, i)
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
	if b, err := json.MarshalIndent(input, "", "  "); err == nil {
		sb.Write(b)
	} else {
		sb.WriteString(fmt.Sprintf("%+v", input))
	}
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
	if context == nil {
		return ""
	}

	if len(context.Previous) > 0 {
		sb.WriteString("PREVIOUS CHAPTERS:\n")
		for _, prev := range context.Previous {
			sb.WriteString(fmt.Sprintf("\n--- %s: %s ---\n", prev.Chapter.ID, prev.Chapter.Title))
			sb.WriteString(prev.Content)
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

func buildNextChapterInfos(context *ChapterContext) []NextChapterInfo {
	if context == nil || len(context.Next) == 0 {
		return nil
	}
	nextInfos := make([]NextChapterInfo, 0, len(context.Next))
	for _, nc := range context.Next {
		if nc == nil || nc.Chapter == nil {
			continue
		}
		nextInfos = append(nextInfos, NextChapterInfo{
			ID:      nc.Chapter.ID,
			Title:   nc.Chapter.Title,
			Summary: nc.Chapter.Summary,
		})
	}
	return nextInfos
}

func recapForContext(context *ChapterContext) string {
	if context == nil {
		return ""
	}
	return context.Recap
}

func validateWriteContent(chapter *models.Chapter, content string, targetWords int) error {
	chapterID := ""
	if chapter != nil {
		chapterID = chapter.ID
	}
	clean := strings.TrimSpace(content)
	if clean == "" {
		return fmt.Errorf("AI returned empty content for chapter %s", chapterID)
	}
	if strings.HasPrefix(clean, "{") && strings.Contains(clean, `"content"`) {
		return fmt.Errorf("AI returned JSON text as chapter prose for chapter %s", chapterID)
	}
	if strings.Contains(clean, "```") {
		return fmt.Errorf("AI returned markdown code fence in chapter prose for chapter %s", chapterID)
	}
	if targetWords > 0 && narrativeUnitCount(clean) < minAcceptableNarrativeUnits(targetWords) {
		return fmt.Errorf("AI returned too little content for chapter %s: got %d narrative units, target %d", chapterID, narrativeUnitCount(clean), targetWords)
	}
	return nil
}

func validateWriteTargetLength(content string, targetWords int) string {
	if targetWords <= 0 {
		return ""
	}
	count := narrativeUnitCount(content)
	low := int(float64(targetWords) * 0.95)
	high := int(float64(targetWords) * 1.05)
	if count < low || count > high {
		return fmt.Sprintf("got %d narrative units, target %d, preferred range %d-%d", count, targetWords, low, high)
	}
	return ""
}

func minAcceptableNarrativeUnits(targetWords int) int {
	min := targetWords / 2
	if min < 120 {
		min = 120
	}
	return min
}

func narrativeUnitCount(content string) int {
	hasCJK := false
	cjkCount := 0
	for _, r := range content {
		if isCJK(r) {
			hasCJK = true
			cjkCount++
		}
	}
	if hasCJK {
		return cjkCount
	}
	return len(strings.Fields(content))
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// formatStateMatrixForWrite formats the state matrix for the prompt
// Delegates to logic.FormatStateMatrix for consistency
func formatStateMatrixForWrite(state *models.StateMatrix, chapter *models.Chapter) string {
	return logic.FormatStateMatrix(state, chapter)
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
