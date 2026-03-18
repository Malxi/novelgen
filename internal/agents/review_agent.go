package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
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
	var chapterContents []chapterDraftContent
	for i := range volume.Chapters {
		chapter := &volume.Chapters[i]
		draft, exists := drafts[chapter.ID]
		if !exists || draft == "" {
			log.Warn("No draft found for chapter: %s", chapter.ID)
			continue
		}
		chapterContents = append(chapterContents, chapterDraftContent{
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

// chapterDraftContent holds chapter and its draft
type chapterDraftContent struct {
	Chapter *models.Chapter
	Draft   string
}

// reviewAllDrafts reviews all drafts in a volume together
func (a *ReviewAgent) reviewAllDrafts(volume *models.Volume, chapterContents []chapterDraftContent) ([]DraftReview, error) {
	log := logger.GetLogger()
	log.Info("Sending %d chapters for batch review", len(chapterContents))

	prompt := a.buildVolumeReviewPrompt(volume, chapterContents)

	messages := []llm.Message{
		{Role: "system", Content: a.getVolumeSystemPrompt()},
		{Role: "user", Content: prompt},
	}

	options := a.config.GetChatOptions(a.llmCfg)

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("review request failed: %w", err)
	}

	return a.parseVolumeReviewResponse(chapterContents, resp.Content)
}

// getVolumeSystemPrompt returns system prompt for volume review
func (a *ReviewAgent) getVolumeSystemPrompt() string {
	if a.language == "zh" {
		return `你是一位专业的小说编辑和评论家。你的任务是审阅整个卷（volume）的所有章节草稿，并提供详细的改进建议。

请从以下几个维度对每章进行评价：
1. 剧情连贯性 - 与前后章节的情节是否连贯
2. 情节合理性 - 剧情逻辑是否合理，有无漏洞
3. 角色一致性 - 角色行为是否符合其设定
4. 节奏把控 - 故事节奏是否恰当

同时请从整个卷的层面评价：
- 卷的整体结构是否合理
- 章节之间的衔接是否流畅
- 是否有跨章节的情节漏洞

请以JSON格式输出评价结果，包含所有章节的评价。`
	}

	return `You are a professional novel editor and critic. Your task is to review all chapters in an entire volume and provide detailed improvement suggestions.

Please evaluate each chapter from the following dimensions:
1. Plot Coherence - Is the plot coherent with previous and next chapters?
2. Plot Rationality - Is the plot logic reasonable? Any flaws?
3. Character Consistency - Do characters act according to their established traits?
4. Pacing - Is the story pacing appropriate?

Also evaluate at the volume level:
- Is the overall volume structure reasonable?
- Are transitions between chapters smooth?
- Are there any cross-chapter plot holes?

Please output the review results in JSON format, including reviews for all chapters.`
}

// ReviewDraft reviews a single draft
func (a *ReviewAgent) ReviewDraft(chapter *models.Chapter, draft string) (*DraftReview, error) {
	log := logger.GetLogger()
	log.Info("Reviewing draft: %s - %s", chapter.ID, chapter.Title)

	// Get previous and next chapters for context
	prevChapter, nextChapter := a.getAdjacentChapters(chapter)

	prompt := a.buildReviewPrompt(chapter, draft, prevChapter, nextChapter)

	messages := []llm.Message{
		{Role: "system", Content: a.getSystemPrompt()},
		{Role: "user", Content: prompt},
	}

	options := a.config.GetChatOptions(a.llmCfg)

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("review request failed: %w", err)
	}

	return a.parseReviewResponse(chapter, resp.Content)
}

func (a *ReviewAgent) getSystemPrompt() string {
	if a.language == "zh" {
		return `你是一位专业的小说编辑和评论家。你的任务是审阅小说草稿并提供详细的改进建议。

请从以下几个维度进行评价：
1. 剧情连贯性 - 与前后章节的情节是否连贯
2. 情节合理性 - 剧情逻辑是否合理，有无漏洞
3. 角色一致性 - 角色行为是否符合其设定
4. 节奏把控 - 故事节奏是否恰当

请以JSON格式输出评价结果。`
	}

	return `You are a professional novel editor and critic. Your task is to review draft chapters and provide detailed improvement suggestions.

Please evaluate from the following dimensions:
1. Plot Coherence - Is the plot coherent with previous and next chapters?
2. Plot Rationality - Is the plot logic reasonable? Any flaws?
3. Character Consistency - Do characters act according to their established traits?
4. Pacing - Is the story pacing appropriate?

Please output the review result in JSON format.`
}

func (a *ReviewAgent) buildReviewPrompt(chapter *models.Chapter, draft string, prevChapter, nextChapter *models.Chapter) string {
	var sb strings.Builder

	if a.language == "zh" {
		sb.WriteString("# 小说草稿审阅\n\n")

		sb.WriteString("## 故事设定\n")
		sb.WriteString(fmt.Sprintf("类型: %s\n", strings.Join(a.setup.Genres, ", ")))
		sb.WriteString(fmt.Sprintf("核心设定: %s\n\n", a.setup.Premise))

		sb.WriteString("## 当前章节大纲\n")
		sb.WriteString(fmt.Sprintf("章节ID: %s\n", chapter.ID))
		sb.WriteString(fmt.Sprintf("标题: %s\n", chapter.Title))
		sb.WriteString(fmt.Sprintf("摘要: %s\n", chapter.Summary))
		sb.WriteString(fmt.Sprintf("角色: %s\n", strings.Join(chapter.Characters, ", ")))
		sb.WriteString(fmt.Sprintf("地点: %s\n\n", chapter.Location))

		if prevChapter != nil {
			sb.WriteString("## 前一章节\n")
			sb.WriteString(fmt.Sprintf("标题: %s\n", prevChapter.Title))
			sb.WriteString(fmt.Sprintf("摘要: %s\n\n", prevChapter.Summary))
		}

		if nextChapter != nil {
			sb.WriteString("## 后一章节\n")
			sb.WriteString(fmt.Sprintf("标题: %s\n", nextChapter.Title))
			sb.WriteString(fmt.Sprintf("摘要: %s\n\n", nextChapter.Summary))
		}

		sb.WriteString("## 待审阅草稿\n")
		sb.WriteString(draft)
		sb.WriteString("\n\n")

		sb.WriteString("## 输出要求\n")
		sb.WriteString("请提供以下JSON格式的评价：\n")
		sb.WriteString(`{
  "chapter_id": "章节ID",
  "chapter_title": "章节标题",
  "overall_score": 7,
  "plot_coherence": {
    "score": 7,
    "issues": ["问题1", "问题2"],
    "suggestions": ["建议1", "建议2"]
  },
  "plot_rationality": {
    "score": 8,
    "logic_flaws": ["漏洞1"],
    "suggestions": ["建议1"]
  },
  "character_consistency": {
    "score": 9,
    "inconsistencies": [],
    "suggestions": []
  },
  "pacing_review": {
    "score": 6,
    "issues": ["节奏拖沓"],
    "suggestions": ["加快节奏"]
  },
  "suggestions": ["总体建议1", "总体建议2"],
  "needs_revision": true
}`)
	} else {
		sb.WriteString("# Novel Draft Review\n\n")

		sb.WriteString("## Story Setup\n")
		sb.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(a.setup.Genres, ", ")))
		sb.WriteString(fmt.Sprintf("Premise: %s\n\n", a.setup.Premise))

		sb.WriteString("## Current Chapter Outline\n")
		sb.WriteString(fmt.Sprintf("Chapter ID: %s\n", chapter.ID))
		sb.WriteString(fmt.Sprintf("Title: %s\n", chapter.Title))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", chapter.Summary))
		sb.WriteString(fmt.Sprintf("Characters: %s\n", strings.Join(chapter.Characters, ", ")))
		sb.WriteString(fmt.Sprintf("Location: %s\n\n", chapter.Location))

		if prevChapter != nil {
			sb.WriteString("## Previous Chapter\n")
			sb.WriteString(fmt.Sprintf("Title: %s\n", prevChapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n\n", prevChapter.Summary))
		}

		if nextChapter != nil {
			sb.WriteString("## Next Chapter\n")
			sb.WriteString(fmt.Sprintf("Title: %s\n", nextChapter.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n\n", nextChapter.Summary))
		}

		sb.WriteString("## Draft to Review\n")
		sb.WriteString(draft)
		sb.WriteString("\n\n")

		sb.WriteString("## Output Requirements\n")
		sb.WriteString("Please provide review in the following JSON format:\n")
		sb.WriteString(`{
  "chapter_id": "chapter_id",
  "chapter_title": "Chapter Title",
  "overall_score": 7,
  "plot_coherence": {
    "score": 7,
    "issues": ["issue1", "issue2"],
    "suggestions": ["suggestion1", "suggestion2"]
  },
  "plot_rationality": {
    "score": 8,
    "logic_flaws": ["flaw1"],
    "suggestions": ["suggestion1"]
  },
  "character_consistency": {
    "score": 9,
    "inconsistencies": [],
    "suggestions": []
  },
  "pacing_review": {
    "score": 6,
    "issues": ["pacing too slow"],
    "suggestions": ["speed up pacing"]
  },
  "suggestions": ["overall suggestion1", "overall suggestion2"],
  "needs_revision": true
}`)
	}

	return sb.String()
}

func (a *ReviewAgent) parseReviewResponse(chapter *models.Chapter, content string) (*DraftReview, error) {
	// Extract JSON from response
	content = extractJSON(content)

	var review DraftReview
	if err := json.Unmarshal([]byte(content), &review); err != nil {
		return nil, fmt.Errorf("failed to parse review response: %w", err)
	}

	// Ensure chapter info is set
	review.ChapterID = chapter.ID
	review.ChapterTitle = chapter.Title

	return &review, nil
}

// extractJSON extracts JSON from markdown code blocks if present
func extractJSON(content string) string {
	return extractJSONFromMarkdown(content)
}

func (a *ReviewAgent) getAdjacentChapters(chapter *models.Chapter) (*models.Chapter, *models.Chapter) {
	var prevChapter, nextChapter *models.Chapter
	found := false

	for _, part := range a.outline.Parts {
		for _, volume := range part.Volumes {
			for i := range volume.Chapters {
				if volume.Chapters[i].ID == chapter.ID {
					found = true
					if i > 0 {
						prevChapter = &volume.Chapters[i-1]
					}
					if i < len(volume.Chapters)-1 {
						nextChapter = &volume.Chapters[i+1]
					}
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}

	return prevChapter, nextChapter
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

// buildVolumeReviewPrompt builds prompt for reviewing entire volume
func (a *ReviewAgent) buildVolumeReviewPrompt(volume *models.Volume, chapterContents []chapterDraftContent) string {
	var sb strings.Builder

	if a.language == "zh" {
		sb.WriteString("# 小说卷审阅\n\n")

		sb.WriteString("## 故事设定\n")
		sb.WriteString(fmt.Sprintf("类型: %s\n", strings.Join(a.setup.Genres, ", ")))
		sb.WriteString(fmt.Sprintf("核心设定: %s\n\n", a.setup.Premise))

		sb.WriteString(fmt.Sprintf("## 卷信息: %s\n", volume.Title))
		sb.WriteString(fmt.Sprintf("摘要: %s\n\n", volume.Summary))

		sb.WriteString("## 章节大纲\n")
		for _, cc := range chapterContents {
			ch := cc.Chapter
			sb.WriteString(fmt.Sprintf("\n### %s: %s\n", ch.ID, ch.Title))
			sb.WriteString(fmt.Sprintf("摘要: %s\n", ch.Summary))
			sb.WriteString(fmt.Sprintf("角色: %s\n", strings.Join(ch.Characters, ", ")))
			sb.WriteString(fmt.Sprintf("地点: %s\n", ch.Location))
		}

		sb.WriteString("\n## 待审阅草稿\n")
		for _, cc := range chapterContents {
			sb.WriteString(fmt.Sprintf("\n---\n### %s: %s\n\n", cc.Chapter.ID, cc.Chapter.Title))
			sb.WriteString(cc.Draft)
			sb.WriteString("\n")
		}

		sb.WriteString("\n## 输出要求\n")
		sb.WriteString("请提供以下JSON格式的评价，包含所有章节的评价数组：\n")
		sb.WriteString(`{
  "reviews": [
    {
      "chapter_id": "C1",
      "chapter_title": "章节标题",
      "overall_score": 7,
      "plot_coherence": {
        "score": 7,
        "issues": ["问题1", "问题2"],
        "suggestions": ["建议1", "建议2"]
      },
      "plot_rationality": {
        "score": 8,
        "logic_flaws": ["漏洞1"],
        "suggestions": ["建议1"]
      },
      "character_consistency": {
        "score": 9,
        "inconsistencies": [],
        "suggestions": []
      },
      "pacing_review": {
        "score": 6,
        "issues": ["节奏拖沓"],
        "suggestions": ["加快节奏"]
      },
      "suggestions": ["总体建议1", "总体建议2"],
      "needs_revision": true
    }
  ],
  "volume_summary": "对整个卷的总体评价"
}`)
	} else {
		sb.WriteString("# Novel Volume Review\n\n")

		sb.WriteString("## Story Setup\n")
		sb.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(a.setup.Genres, ", ")))
		sb.WriteString(fmt.Sprintf("Premise: %s\n\n", a.setup.Premise))

		sb.WriteString(fmt.Sprintf("## Volume Info: %s\n", volume.Title))
		sb.WriteString(fmt.Sprintf("Summary: %s\n\n", volume.Summary))

		sb.WriteString("## Chapter Outlines\n")
		for _, cc := range chapterContents {
			ch := cc.Chapter
			sb.WriteString(fmt.Sprintf("\n### %s: %s\n", ch.ID, ch.Title))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", ch.Summary))
			sb.WriteString(fmt.Sprintf("Characters: %s\n", strings.Join(ch.Characters, ", ")))
			sb.WriteString(fmt.Sprintf("Location: %s\n", ch.Location))
		}

		sb.WriteString("\n## Drafts to Review\n")
		for _, cc := range chapterContents {
			sb.WriteString(fmt.Sprintf("\n---\n### %s: %s\n\n", cc.Chapter.ID, cc.Chapter.Title))
			sb.WriteString(cc.Draft)
			sb.WriteString("\n")
		}

		sb.WriteString("\n## Output Requirements\n")
		sb.WriteString("Please provide reviews in the following JSON format with an array of all chapter reviews:\n")
		sb.WriteString(`{
  "reviews": [
    {
      "chapter_id": "C1",
      "chapter_title": "Chapter Title",
      "overall_score": 7,
      "plot_coherence": {
        "score": 7,
        "issues": ["issue1", "issue2"],
        "suggestions": ["suggestion1", "suggestion2"]
      },
      "plot_rationality": {
        "score": 8,
        "logic_flaws": ["flaw1"],
        "suggestions": ["suggestion1"]
      },
      "character_consistency": {
        "score": 9,
        "inconsistencies": [],
        "suggestions": []
      },
      "pacing_review": {
        "score": 6,
        "issues": ["pacing too slow"],
        "suggestions": ["speed up pacing"]
      },
      "suggestions": ["overall suggestion1", "overall suggestion2"],
      "needs_revision": true
    }
  ],
  "volume_summary": "Overall evaluation of the entire volume"
}`)
	}

	return sb.String()
}

// parseVolumeReviewResponse parses the response for volume review
func (a *ReviewAgent) parseVolumeReviewResponse(chapterContents []chapterDraftContent, content string) ([]DraftReview, error) {
	// Extract JSON from response
	content = extractJSON(content)

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
