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

// CompactStorySetup is a minimal version of StorySetup for chapter generation
// Only includes essential fields needed for writing
type CompactStorySetup struct {
	Genres   []string `json:"genres" md:"genres" desc:"Story genres (2-4 specific genres)"`
	Premise  string   `json:"premise" md:"premise" desc:"Core story premise (2-4 sentences)"`
	Theme    string   `json:"theme" md:"theme" desc:"Story theme (clear statement)"`
	Rules    []string `json:"rules" md:"rules" desc:"World rules (3-7 enforceable rules)"`
	Tone     string   `json:"tone" md:"tone" desc:"Writing tone (2-4 adjectives, comma-separated)"`
	Tense    string   `json:"tense" md:"tense" desc:"Narrative tense (past or present)"`
	POVStyle string   `json:"pov_style" md:"pov_style" desc:"POV style (first person, third person limited, or third person omniscient)"`
}

// ToCompact converts a full StorySetup to CompactStorySetup
func ToCompact(setup *models.StorySetup) CompactStorySetup {
	return CompactStorySetup{
		Genres:   setup.Genres,
		Premise:  setup.Premise,
		Theme:    setup.Theme,
		Rules:    setup.Rules,
		Tone:     setup.Tone,
		Tense:    setup.Tense,
		POVStyle: setup.POVStyle,
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
	CurrentDraft string            `json:"current_draft" md:"current_draft" desc:"The current draft content to be improved"`
	Suggestions  string            `json:"suggestions" md:"suggestions" desc:"Review suggestions for improvement"`
	Context      string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap        string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter"`
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
	Recap          string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter for continuity checking"`
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

// PlotBug represents a plot bug found in the volume
type PlotBug struct {
	BugType     string `json:"bug_type" md:"bug_type" desc:"Type of plot bug: plot_contradiction/setting_conflict/character_break"`
	Severity    string `json:"severity" md:"severity" desc:"Severity: critical/major/minor"`
	Location    string `json:"location" md:"location" desc:"Chapter ID where bug is found"`
	Description string `json:"description" md:"description" desc:"Bug description"`
	Expected    string `json:"expected" md:"expected" desc:"What it should be"`
	Actual      string `json:"actual" md:"actual" desc:"What was actually written"`
}

// LogicBug represents a logic bug found in the volume
type LogicBug struct {
	BugType     string `json:"bug_type" md:"bug_type" desc:"Type of logic bug: timeline/space/numeric/information"`
	Severity    string `json:"severity" md:"severity" desc:"Severity: critical/major/minor"`
	Location    string `json:"location" md:"location" desc:"Chapter ID where bug is found"`
	Description string `json:"description" md:"description" desc:"Bug description"`
	Explanation string `json:"explanation" md:"explanation" desc:"Explanation of the logic error"`
}

// VolumeReviewResult represents the result of volume-level review
type VolumeReviewResult struct {
	OverallScore            int                     `json:"overall_score" md:"overall_score" desc:"Overall volume quality score 0-100"`
	VolumeStructureAnalysis VolumeStructureAnalysis `json:"volume_structure_analysis" md:"volume_structure_analysis" desc:"Analysis of volume structure: opening, rising action, climax, resolution"`
	ChapterReviews          []VolumeChapterReview   `json:"chapter_reviews" md:"chapter_reviews" desc:"Review results for each chapter"`
	VolumeLevelIssues       []string                `json:"volume_level_issues" md:"volume_level_issues" desc:"Issues that affect the entire volume"`
	VolumeLevelSuggestions  []string                `json:"volume_level_suggestions" md:"volume_level_suggestions" desc:"Suggestions for improving the entire volume"`
	PlotBugs                []PlotBug               `json:"plot_bugs" md:"plot_bugs" desc:"List of plot bugs found in the volume"`
	LogicBugs               []LogicBug              `json:"logic_bugs" md:"logic_bugs" desc:"List of logic bugs found in the volume"`
	PlotFixSuggestions      []string                `json:"plot_fix_suggestions" md:"plot_fix_suggestions" desc:"Suggestions for fixing plot bugs"`
	LogicFixSuggestions     []string                `json:"logic_fix_suggestions" md:"logic_fix_suggestions" desc:"Suggestions for fixing logic bugs"`
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
		StorySetup:   ToCompact(a.setup),
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

	// Validate output content is not empty
	if strings.TrimSpace(output.Content) == "" {
		return "", fmt.Errorf("AI returned empty content for chapter %s", chapter.ID)
	}

	// Log context for debugging
	if err := a.logWriteContext(chapter.ID, "final", input, output.Content); err != nil {
		logger.Warn("Failed to log write context: %v", err)
	}

	logger.Info("✓ Generated final chapter: %d characters", len(output.Content))
	return output.Content, nil
}

// GenerateChapterWithSuggestions generates improved chapter content with review suggestions
func (a *WriteAgent) GenerateChapterWithSuggestions(ctx context.Context, chapter *models.Chapter, context *ChapterContext, state *models.StateMatrix, targetWords int, currentDraft string, suggestions string) (string, error) {
	logger.Section("WRITE AGENT - Chapter Improvement")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	input := WriteImproveInput{
		StorySetup:   ToCompact(a.setup),
		Chapter:      *chapter,
		StateMatrix:  formatStateMatrixForWrite(state, chapter),
		TargetWords:  targetWords,
		CurrentDraft: currentDraft,
		Suggestions:  suggestions,
		Context:      formatChapterContext(context),
		Recap:        context.Recap,
	}

	var output WriteImproveOutput
	params := InvokeParams{
		Skills:  []string{"write-improve"},
		Command: "improve chapter based on suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	// Validate output content is not empty
	if strings.TrimSpace(output.Content) == "" {
		return "", fmt.Errorf("AI returned empty content for chapter %s improvement", chapter.ID)
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
	if context != nil {
		recap = context.Recap
	}

	input := WriteReviewInput{
		StorySetup:     ToCompact(a.setup),
		StateMatrix:    formatStateMatrixForWrite(state, chapter),
		Chapter:        *chapter,
		ChapterContent: content,
		TargetWords:    targetWords,
		Iteration:      iteration,
		Recap:          recap,
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
		beats := chapter.Beats

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
		improved, err := a.GenerateChapterWithSuggestions(ctx, chapter, context, state, targetWords, currentContent, suggestions)
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
// Only includes characters, relationships, and storylines relevant to the current chapter
func formatStateMatrixForWrite(state *models.StateMatrix, chapter *models.Chapter) string {
	if state == nil {
		return "No state matrix available"
	}

	var sb strings.Builder
	sb.WriteString("CURRENT STORY STATE:\n")

	// Build set of relevant characters for this chapter
	relevantChars := make(map[string]bool)
	// Add characters explicitly listed in the chapter
	for _, charName := range chapter.Characters {
		relevantChars[charName] = true
	}
	// Add characters mentioned in chapter events
	for _, event := range chapter.Events {
		for _, charName := range event.Characters {
			relevantChars[charName] = true
		}
	}

	// Character states - only show relevant characters
	if len(state.Characters) > 0 {
		sb.WriteString("\nCharacters:\n")
		for name, char := range state.Characters {
			// Skip if not relevant to this chapter
			if !relevantChars[name] {
				continue
			}
			sb.WriteString(fmt.Sprintf("  %s:\n", name))
			if char.Personality != nil && len(char.Personality) > 0 {
				sb.WriteString(fmt.Sprintf("    - Personality: %s\n", strings.Join(char.Personality, ", ")))
			}
			if char.Background != "" {
				sb.WriteString(fmt.Sprintf("    - Background: %s\n", char.Background))
			}
		}
	}

	// Active relationships - only show relationships between relevant characters
	if len(state.Relationships) > 0 {
		sb.WriteString("\nActive Relationships:\n")
		for relKey, relStatus := range state.Relationships {
			// Check if this relationship involves relevant characters
			parts := strings.Split(relKey, "_")
			if len(parts) >= 2 {
				char1, char2 := parts[0], parts[1]
				if relevantChars[char1] || relevantChars[char2] {
					sb.WriteString(fmt.Sprintf("  %s: %s\n", relKey, relStatus))
				}
			}
		}
	}

	// Active storylines - only show storylines relevant to chapter events
	if len(state.Storylines) > 0 {
		sb.WriteString("\nActive Storylines:\n")
		for _, sl := range state.Storylines {
			if sl.Status == "active" || sl.Status == "in_progress" {
				// Check if this storyline is mentioned in chapter events
				isRelevant := false
				for _, event := range chapter.Events {
					if event.Type == "storyline" && event.Subject == sl.Name {
						isRelevant = true
						break
					}
				}
				if isRelevant {
					sb.WriteString(fmt.Sprintf("  %s: %s\n", sl.Name, sl.Progress))
				}
			}
		}
	}

	// Character Goals - only show for relevant characters
	if len(state.Goals) > 0 {
		hasGoals := false
		var goalsBuilder strings.Builder
		goalsBuilder.WriteString("\nCharacter Goals:\n")
		for charName, goals := range state.Goals {
			if relevantChars[charName] && len(goals) > 0 {
				hasGoals = true
				goalsBuilder.WriteString(fmt.Sprintf("  %s:\n", charName))
				for _, goal := range goals {
					goalsBuilder.WriteString(fmt.Sprintf("    - %s\n", goal))
				}
			}
		}
		if hasGoals {
			sb.WriteString(goalsBuilder.String())
		}
	}

	// Character Status - only show for relevant characters
	if len(state.Status) > 0 {
		hasStatus := false
		var statusBuilder strings.Builder
		statusBuilder.WriteString("\nCharacter Status:\n")
		for statusKey, status := range state.Status {
			// statusKey format: "character_name_status_type"
			parts := strings.Split(statusKey, "_")
			if len(parts) >= 1 {
				charName := parts[0]
				if relevantChars[charName] {
					hasStatus = true
					statusBuilder.WriteString(fmt.Sprintf("  %s: %s", charName, status.Type))
					if status.State != "" {
						statusBuilder.WriteString(fmt.Sprintf(" (%s)", status.State))
					}
					if status.Severity != "" {
						statusBuilder.WriteString(fmt.Sprintf(" [%s]", status.Severity))
					}
					if status.Details != "" {
						statusBuilder.WriteString(fmt.Sprintf(" - %s", status.Details))
					}
					statusBuilder.WriteString("\n")
				}
			}
		}
		if hasStatus {
			sb.WriteString(statusBuilder.String())
		}
	}

	// Items - only show items owned by relevant characters
	if len(state.Items) > 0 {
		hasItems := false
		var itemsBuilder strings.Builder
		itemsBuilder.WriteString("\nItems:\n")
		for itemName, item := range state.Items {
			// Only show if item is owned by a relevant character
			// Items without owner should not be shown (they are from craft definition, not acquired yet)
			if item.Owner != "" && relevantChars[item.Owner] {
				hasItems = true
				itemsBuilder.WriteString(fmt.Sprintf("  %s", itemName))
				itemsBuilder.WriteString(fmt.Sprintf(" (held by: %s)", item.Owner))
				if item.Description != "" {
					itemsBuilder.WriteString(fmt.Sprintf(" - %s", item.Description))
				}
				itemsBuilder.WriteString("\n")
			}
		}
		if hasItems {
			sb.WriteString(itemsBuilder.String())
		}
	}

	// Character Progression/Premises - show character levels/ranks
	if len(state.Premises) > 0 {
		hasPremises := false
		var premisesBuilder strings.Builder
		premisesBuilder.WriteString("\nCharacter Progression:\n")

		// Group premises by character
		charPremises := make(map[string]map[string]string) // charName -> premiseName -> value
		for key, value := range state.Premises {
			// key format: "characterName_premiseName"
			parts := strings.SplitN(key, "_", 2)
			if len(parts) == 2 {
				charName, premiseName := parts[0], parts[1]
				if relevantChars[charName] {
					if charPremises[charName] == nil {
						charPremises[charName] = make(map[string]string)
					}
					charPremises[charName][premiseName] = value
				}
			}
		}

		// Output grouped by character
		for charName, premises := range charPremises {
			hasPremises = true
			premisesBuilder.WriteString(fmt.Sprintf("  %s:\n", charName))
			for premiseName, value := range premises {
				premisesBuilder.WriteString(fmt.Sprintf("    - %s: %s\n", premiseName, value))
			}
		}

		if hasPremises {
			sb.WriteString(premisesBuilder.String())
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
