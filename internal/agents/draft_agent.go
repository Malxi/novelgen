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

// DraftGenInput is the input for draft generation
type DraftGenInput struct {
	StorySetup   models.StorySetup `json:"story_setup" md:"story_setup" desc:"Story setup including premise, genres, themes, rules"`
	Chapter      models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	StateMatrix  string            `json:"state_matrix" md:"state_matrix" desc:"Current story state including character statuses, relationships, goals"`
	TargetWords  int               `json:"target_words" md:"target_words" desc:"Target word count for the draft"`
	Context      string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap        string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter"`
	NextChapters []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing"`
	CustomPrompt string            `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional custom prompt for generation"`
}

// NextChapterInfo holds info about upcoming chapters for foreshadowing
type NextChapterInfo struct {
	ID      string `json:"id" desc:"Chapter ID"`
	Title   string `json:"title" desc:"Chapter title"`
	Summary string `json:"summary" desc:"Chapter summary"`
}

// DraftGenOutput is the output for draft generation
type DraftGenOutput struct {
	Content string `json:"content" md:"content" desc:"Generated draft content"`
}

// DraftImproveInput is the input for draft improvement
type DraftImproveInput struct {
	StorySetup   models.StorySetup `json:"story_setup" md:"story_setup" desc:"Story setup including premise, genres, themes, rules"`
	Chapter      models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	StateMatrix  string            `json:"state_matrix" md:"state_matrix" desc:"Current story state including character statuses, relationships, goals"`
	TargetWords  int               `json:"target_words" md:"target_words" desc:"Target word count for the draft"`
	CurrentDraft string            `json:"current_draft" md:"current_draft" desc:"The current draft content to be improved"`
	Suggestions  string            `json:"suggestions" md:"suggestions" desc:"Review suggestions for improvement"`
	Context      string            `json:"context,omitempty" md:"context,omitempty" desc:"Continuity context from previous chapters"`
	Recap        string            `json:"recap,omitempty" md:"recap,omitempty" desc:"Canonical recap from previous chapter"`
	NextChapters []NextChapterInfo `json:"next_chapters,omitempty" md:"next_chapters,omitempty" desc:"Information about upcoming chapters for foreshadowing"`
	CustomPrompt string            `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional custom prompt for improvement"`
}

// DraftImproveOutput is the output for draft improvement
type DraftImproveOutput struct {
	Content string `json:"content" md:"content" desc:"Improved draft content"`
}

// DraftReviewInput is the input for draft review
type DraftReviewInput struct {
	StorySetup   models.StorySetup `json:"story_setup" md:"story_setup" desc:"Story setup including premise, genres, themes, rules"`
	Chapter      models.Chapter    `json:"chapter" md:"chapter" desc:"Chapter information including title, summary, beats, characters"`
	DraftContent string            `json:"draft_content" md:"draft_content" desc:"The draft content to be reviewed"`
	TargetWords  int               `json:"target_words" md:"target_words" desc:"Target word count for the draft"`
	Iteration    int               `json:"iteration" md:"iteration" desc:"Current iteration number"`
}

// DraftReviewOutput is the output for draft review
type DraftReviewOutput struct {
	Result models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result including scores and improvement suggestions"`
}

// DraftReview contains review results for a single draft (for backward compatibility)
type DraftReview struct {
	ChapterID            string                `json:"chapter_id"`
	ChapterTitle         string                `json:"chapter_title"`
	OverallScore         int                   `json:"overall_score"`         // 1-10
	PlotCoherence        PlotCoherenceReview   `json:"plot_coherence"`        // 剧情连贯性
	PlotRationality      RationalityReview     `json:"plot_rationality"`      // 情节合理性
	CharacterConsistency CharacterReview       `json:"character_consistency"` // 角色一致性
	PacingReview         PacingReview          `json:"pacing_review"`         // 节奏评价
	SceneContinuity      SceneContinuityReview `json:"scene_continuity"`      // 场景/转场连续性
	CharacterPresence    PresenceReview        `json:"character_presence"`    // 角色出场一致性（启发式）
	RecapQuality         GateReview            `json:"recap_quality"`         // Recap 质量门禁（启发式）
	Suggestions          []string              `json:"suggestions"`           // 改进建议
	NeedsRevision        bool                  `json:"needs_revision"`        // 是否需要重写
}

// PlotCoherenceReview evaluates plot continuity
type PlotCoherenceReview struct {
	Score       int      `json:"score"`       // 1-10
	Issues      []string `json:"issues"`      // 连贯性问题
	Suggestions []string `json:"suggestions"` // 改进建议
}

// RationalityReview evaluates plot logic
type RationalityReview struct {
	Score       int      `json:"score"`       // 1-10
	LogicFlaws  []string `json:"logic_flaws"` // 逻辑漏洞
	Suggestions []string `json:"suggestions"` // 改进建议
}

// CharacterReview evaluates character consistency
type CharacterReview struct {
	Score           int      `json:"score"`           // 1-10
	Inconsistencies []string `json:"inconsistencies"` // 角色不一致之处
	Suggestions     []string `json:"suggestions"`     // 改进建议
}

// PacingReview evaluates story pacing
type PacingReview struct {
	Score       int      `json:"score"`       // 1-10
	Issues      []string `json:"issues"`      // 节奏问题
	Suggestions []string `json:"suggestions"` // 改进建议
}

// SceneContinuityReview evaluates scene-to-scene continuity (teleport transitions)
type SceneContinuityReview struct {
	Score       int      `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

// PresenceReview evaluates whether outlined entities actually appear in the draft.
type PresenceReview struct {
	Score       int      `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

// GateReview is a generic quality gate result (heuristics + deterministic validators).
type GateReview struct {
	Score       int      `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

// VolumeReview contains reviews for all drafts in a volume
type VolumeReview struct {
	VolumeID    string        `json:"volume_id"`
	VolumeTitle string        `json:"volume_title"`
	Reviews     []DraftReview `json:"reviews"`
	Summary     string        `json:"summary"`
}

// DraftAgent generates draft chapters based on story state
// It wraps BaseAgent to provide type-safe methods
type DraftAgent struct {
	base    *BaseAgent
	setup   *models.StorySetup
	outline *models.Outline
}

// NewDraftAgent creates a new DraftAgent
func NewDraftAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline) *DraftAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "DraftAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &DraftAgent{
		base:    base,
		setup:   setup,
		outline: outline,
	}
}

// SetLanguage sets the output language
func (a *DraftAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// GenerateDraft generates a draft chapter
func (a *DraftAgent) GenerateDraft(ctx context.Context, chapter *models.Chapter, state *models.StateMatrix, targetWords int) (string, error) {
	logger.Section("DRAFT AGENT - Draft Generation")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	input := DraftGenInput{
		StorySetup:  *a.setup,
		Chapter:     *chapter,
		StateMatrix: formatStateMatrix(state, chapter),
		TargetWords: targetWords,
	}

	var output DraftGenOutput
	params := InvokeParams{
		Skills:  []string{"draft-generate"},
		Command: "generate draft chapter",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	// Log context for debugging
	if err := a.logDraftContext(chapter.ID, "generate", input, output.Content); err != nil {
		logger.Warn("Failed to log draft context: %v", err)
	}

	logger.Info("✓ Generated draft: %d characters", len(output.Content))
	return output.Content, nil
}

// GenerateDraftWithContext generates a draft chapter with continuity context
func (a *DraftAgent) GenerateDraftWithContext(ctx context.Context, chapter *models.Chapter, state *models.StateMatrix, targetWords int, contextText string, recap string, nextChapters []*models.Chapter) (string, error) {
	logger.Section("DRAFT AGENT - Draft Generation with Context")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	// Convert next chapters to info
	var nextInfos []NextChapterInfo
	for _, nc := range nextChapters {
		nextInfos = append(nextInfos, NextChapterInfo{
			ID:      nc.ID,
			Title:   nc.Title,
			Summary: nc.Summary,
		})
	}

	input := DraftGenInput{
		StorySetup:   *a.setup,
		Chapter:      *chapter,
		StateMatrix:  formatStateMatrix(state, chapter),
		TargetWords:  targetWords,
		Context:      contextText,
		Recap:        recap,
		NextChapters: nextInfos,
	}

	var output DraftGenOutput
	params := InvokeParams{
		Skills:  []string{"draft-generate"},
		Command: "generate draft chapter with continuity context",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	// Log context for debugging
	if err := a.logDraftContext(chapter.ID, "context", input, output.Content); err != nil {
		logger.Warn("Failed to log draft context: %v", err)
	}

	logger.Info("✓ Generated draft with context: %d characters", len(output.Content))
	return output.Content, nil
}

// GenerateDraftWithSuggestions generates an improved draft based on review suggestions
func (a *DraftAgent) GenerateDraftWithSuggestions(ctx context.Context, chapter *models.Chapter, state *models.StateMatrix, targetWords int, suggestions string, contextText string, recap string, nextChapters []*models.Chapter) (string, error) {
	logger.Section("DRAFT AGENT - Draft Improvement")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Target words: %d", targetWords)
	logger.Info("Language: %s", a.base.language)

	// Convert next chapters to info
	var nextInfos []NextChapterInfo
	for _, nc := range nextChapters {
		nextInfos = append(nextInfos, NextChapterInfo{
			ID:      nc.ID,
			Title:   nc.Title,
			Summary: nc.Summary,
		})
	}

	input := DraftImproveInput{
		StorySetup:   *a.setup,
		Chapter:      *chapter,
		StateMatrix:  formatStateMatrix(state, chapter),
		TargetWords:  targetWords,
		Suggestions:  suggestions,
		Context:      contextText,
		Recap:        recap,
		NextChapters: nextInfos,
	}

	var output DraftImproveOutput
	params := InvokeParams{
		Skills:  []string{"draft-improve"},
		Command: "improve draft based on suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	// Log context for debugging
	if err := a.logDraftContext(chapter.ID, "improve", input, output.Content); err != nil {
		logger.Warn("Failed to log draft context: %v", err)
	}

	logger.Info("✓ Generated improved draft: %d characters", len(output.Content))
	return output.Content, nil
}

// ReviewDraft reviews a draft chapter and provides improvement suggestions
func (a *DraftAgent) ReviewDraft(ctx context.Context, chapter *models.Chapter, draftContent string, targetWords int, iteration int) (models.ReviewResult, error) {
	logger.Section("DRAFT AGENT - Draft Review")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Iteration: %d", iteration)
	logger.Info("Language: %s", a.base.language)

	input := DraftReviewInput{
		StorySetup:   *a.setup,
		Chapter:      *chapter,
		DraftContent: draftContent,
		TargetWords:  targetWords,
		Iteration:    iteration,
	}

	var output DraftReviewOutput
	params := InvokeParams{
		Skills:  []string{"draft-review"},
		Command: "review draft and provide improvement suggestions",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return models.ReviewResult{}, err
	}

	logger.Section("Draft Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	logger.Info("Suggestions: %d", len(output.Result.Suggestions))

	return output.Result, nil
}

// IterateDraft runs the review-improvement loop for a draft
func (a *DraftAgent) IterateDraft(ctx context.Context, chapter *models.Chapter, state *models.StateMatrix, targetWords int, initialDraft string, maxIterations int, qualityThreshold float64, contextText string, recap string, nextChapters []*models.Chapter) (string, *models.ReviewResult, error) {
	logger.Section("DRAFT AGENT - Draft Iteration Loop")
	logger.Info("Chapter: %s", chapter.ID)
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)

	currentDraft := initialDraft
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review
		review, err := a.ReviewDraft(ctx, chapter, currentDraft, targetWords, i)
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
		suggestions := formatSuggestions(review.Suggestions)
		improved, err := a.GenerateDraftWithSuggestions(ctx, chapter, state, targetWords, suggestions, contextText, recap, nextChapters)
		if err != nil {
			return "", nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}
		currentDraft = improved
	}

	return currentDraft, finalReview, nil
}

// logDraftContext logs the draft context to a markdown file for debugging
func (a *DraftAgent) logDraftContext(chapterID, variant string, input interface{}, output string) error {
	debugDir := filepath.Join("logs", "draft_contexts")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%s_%s.md", chapterID, variant, timestamp))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Draft Context: %s (%s)\n\n", chapterID, variant))
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

// formatStateMatrix formats the state matrix for the prompt
func formatStateMatrix(state *models.StateMatrix, chapter *models.Chapter) string {
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

// formatSuggestions formats review suggestions for the improvement prompt
func formatSuggestions(suggestions []models.ReviewSuggestion) string {
	var sb strings.Builder
	for i, s := range suggestions {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, s.Priority, s.Issue))
		sb.WriteString(fmt.Sprintf("   Suggestion: %s\n\n", s.Suggestion))
	}
	return sb.String()
}

// ReviewVolume reviews all drafts in a volume and returns a VolumeReview
// This method provides backward compatibility with the old ReviewAgent
func (a *DraftAgent) ReviewVolume(ctx context.Context, volume *models.Volume, drafts map[string]string) (*VolumeReview, error) {
	logger.Section("DRAFT AGENT - Volume Review")
	logger.Info("Volume: %s - %s", volume.ID, volume.Title)
	logger.Info("Drafts to review: %d", len(drafts))

	var reviews []DraftReview

	// Review each chapter in the volume
	for i := range volume.Chapters {
		chapter := &volume.Chapters[i]
		draft, exists := drafts[chapter.ID]
		if !exists || draft == "" {
			logger.Warn("No draft found for chapter: %s", chapter.ID)
			continue
		}

		logger.Info("Reviewing chapter: %s - %s", chapter.ID, chapter.Title)

		// Use ReviewDraft to review this chapter
		reviewResult, err := a.ReviewDraft(ctx, chapter, draft, 0, 1)
		if err != nil {
			logger.Error("Failed to review chapter %s: %v", chapter.ID, err)
			continue
		}

		// Convert ReviewResult to DraftReview
		draftReview := convertReviewResultToDraftReview(chapter, reviewResult)
		reviews = append(reviews, draftReview)
	}

	if len(reviews) == 0 {
		return nil, fmt.Errorf("no drafts found for volume %s", volume.ID)
	}

	// Generate summary
	summary := a.generateVolumeSummary(reviews)

	logger.Info("✓ Volume review complete: %d chapters reviewed", len(reviews))

	return &VolumeReview{
		VolumeID:    volume.ID,
		VolumeTitle: volume.Title,
		Reviews:     reviews,
		Summary:     summary,
	}, nil
}

// generateVolumeSummary generates a summary for volume review
func (a *DraftAgent) generateVolumeSummary(reviews []DraftReview) string {
	if len(reviews) == 0 {
		return ""
	}

	totalScore := 0
	needsRevision := 0

	for _, r := range reviews {
		totalScore += r.OverallScore
		if r.NeedsRevision {
			needsRevision++
		}
	}

	avgScore := float64(totalScore) / float64(len(reviews))

	if a.base.language == "zh" {
		return fmt.Sprintf("共审阅 %d 章，平均评分 %.1f/10，其中 %d 章需要修改", len(reviews), avgScore, needsRevision)
	}

	return fmt.Sprintf("Reviewed %d chapters, average score %.1f/10, %d chapters need revision", len(reviews), avgScore, needsRevision)
}

// convertReviewResultToDraftReview converts a ReviewResult to DraftReview format
func convertReviewResultToDraftReview(chapter *models.Chapter, result models.ReviewResult) DraftReview {
	review := DraftReview{
		ChapterID:    chapter.ID,
		ChapterTitle: chapter.Title,
		OverallScore: int(result.OverallScore / 10), // Convert 0-100 to 1-10
	}

	// Convert suggestions
	for _, s := range result.Suggestions {
		review.Suggestions = append(review.Suggestions, s.Issue+": "+s.Suggestion)
	}

	// Determine if revision is needed (score < 7 or has high priority issues)
	review.NeedsRevision = review.OverallScore < 7
	for _, s := range result.Suggestions {
		if s.Priority == "high" {
			review.NeedsRevision = true
			break
		}
	}

	// Map dimensions to specific review categories
	for _, d := range result.Dimensions {
		switch d.Name {
		case "continuity":
			review.SceneContinuity = SceneContinuityReview{
				Score:       int(d.Score / 10),
				Issues:      []string{},
				Suggestions: []string{},
			}
		case "plot_coherence":
			review.PlotCoherence = PlotCoherenceReview{
				Score:       int(d.Score / 10),
				Issues:      []string{},
				Suggestions: []string{},
			}
		case "character_consistency":
			review.CharacterConsistency = CharacterReview{
				Score:           int(d.Score / 10),
				Inconsistencies: []string{},
				Suggestions:     []string{},
			}
		case "writing_quality":
			review.PacingReview = PacingReview{
				Score:       int(d.Score / 10),
				Issues:      []string{},
				Suggestions: []string{},
			}
		}
	}

	return review
}
