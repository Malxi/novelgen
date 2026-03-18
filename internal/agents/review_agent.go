package agents

import (
	"encoding/json"
	"fmt"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// ReviewAgent reviews drafts and provides improvement suggestions
type ReviewAgent struct {
	client   llm.Client
	config   *llm.Config
	llmCfg   *models.ProjectLLM
	setup    *models.StorySetup
	outline  *models.Outline
	language string
}

// DraftReview contains review results for a single draft
type DraftReview struct {
	ChapterID            string              `json:"chapter_id"`
	ChapterTitle         string              `json:"chapter_title"`
	OverallScore         int                 `json:"overall_score"`         // 1-10
	PlotCoherence        PlotCoherenceReview `json:"plot_coherence"`        // 剧情连贯性
	PlotRationality      RationalityReview   `json:"plot_rationality"`      // 情节合理性
	CharacterConsistency CharacterReview     `json:"character_consistency"` // 角色一致性
	PacingReview         PacingReview        `json:"pacing_review"`         // 节奏评价
	Suggestions          []string            `json:"suggestions"`           // 改进建议
	NeedsRevision        bool                `json:"needs_revision"`        // 是否需要重写
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

// VolumeReview contains reviews for all drafts in a volume
type VolumeReview struct {
	VolumeID    string        `json:"volume_id"`
	VolumeTitle string        `json:"volume_title"`
	Reviews     []DraftReview `json:"reviews"`
	Summary     string        `json:"summary"`
}

// NewReviewAgent creates a new review agent
func NewReviewAgent(client llm.Client, config *llm.Config, llmCfg *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline, language string) *ReviewAgent {
	return &ReviewAgent{
		client:   client,
		config:   config,
		llmCfg:   llmCfg,
		setup:    setup,
		outline:  outline,
		language: language,
	}
}

// SetLanguage sets the review language
func (a *ReviewAgent) SetLanguage(lang string) {
	a.language = lang
}

// ReviewVolume reviews all drafts in a volume as a whole
func (a *ReviewAgent) ReviewVolume(volume *models.Volume, drafts map[string]string) (*VolumeReview, error) {
	log := logger.GetLogger()
	log.Info("Reviewing volume: %s - %s", volume.ID, volume.Title)

	// Build all chapters content
	var chapterContents []prompts.ChapterDraftContent
	for i := range volume.Chapters {
		chapter := &volume.Chapters[i]
		draft, exists := drafts[chapter.ID]
		if !exists || draft == "" {
			log.Warn("No draft found for chapter: %s", chapter.ID)
			continue
		}
		chapterContents = append(chapterContents, prompts.ChapterDraftContent{
			Chapter: chapter,
			Draft:   draft,
		})
	}

	if len(chapterContents) == 0 {
		return nil, fmt.Errorf("no drafts found for volume %s", volume.ID)
	}

	// Review all chapters together
	reviews, err := a.reviewAllDrafts(volume, chapterContents)
	if err != nil {
		return nil, fmt.Errorf("failed to review volume: %w", err)
	}

	// Generate summary
	summary := a.generateSummary(reviews)

	return &VolumeReview{
		VolumeID:    volume.ID,
		VolumeTitle: volume.Title,
		Reviews:     reviews,
		Summary:     summary,
	}, nil
}

// reviewAllDrafts reviews all drafts in a volume together
func (a *ReviewAgent) reviewAllDrafts(volume *models.Volume, chapterContents []prompts.ChapterDraftContent) ([]DraftReview, error) {
	log := logger.GetLogger()
	log.Info("Sending %d chapters for batch review", len(chapterContents))

	prompt := prompts.BuildVolumeReviewPrompt(a.setup, volume, chapterContents, a.language)

	messages := []llm.Message{
		{Role: "system", Content: prompts.GetVolumeReviewSystemPrompt(a.language)},
		{Role: "user", Content: prompt},
	}

	options := a.config.GetChatOptions(a.llmCfg)
	options.MaxTokens = 8000 // Increase for multiple chapters

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("review request failed: %w", err)
	}

	return a.parseVolumeReviewResponse(chapterContents, resp.Content)
}

func (a *ReviewAgent) generateSummary(reviews []DraftReview) string {
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

	if a.language == "zh" {
		return fmt.Sprintf("共审阅 %d 章，平均评分 %.1f/10，其中 %d 章需要修改", len(reviews), avgScore, needsRevision)
	}

	return fmt.Sprintf("Reviewed %d chapters, average score %.1f/10, %d chapters need revision", len(reviews), avgScore, needsRevision)
}

// parseVolumeReviewResponse parses the response for volume review
func (a *ReviewAgent) parseVolumeReviewResponse(chapterContents []prompts.ChapterDraftContent, content string) ([]DraftReview, error) {
	// Extract JSON from response
	content = extractJSONFromMarkdown(content)

	// Try to parse as array first
	var reviews []DraftReview
	if err := json.Unmarshal([]byte(content), &reviews); err == nil {
		// Ensure chapter info is set for each review
		for i := range reviews {
			if reviews[i].ChapterID == "" && i < len(chapterContents) {
				reviews[i].ChapterID = chapterContents[i].Chapter.ID
				reviews[i].ChapterTitle = chapterContents[i].Chapter.Title
			}
		}
		return reviews, nil
	}

	// Try to parse as object with reviews field
	var result struct {
		Reviews       []DraftReview `json:"reviews"`
		VolumeSummary string        `json:"volume_summary"`
	}
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		// Ensure chapter info is set for each review
		for i := range result.Reviews {
			if result.Reviews[i].ChapterID == "" && i < len(chapterContents) {
				result.Reviews[i].ChapterID = chapterContents[i].Chapter.ID
				result.Reviews[i].ChapterTitle = chapterContents[i].Chapter.Title
			}
		}
		return result.Reviews, nil
	}

	return nil, fmt.Errorf("failed to parse volume review response")
}
