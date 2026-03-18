package prompts

import (
	"fmt"
	"strings"

	"nolvegen/internal/models"
)

// GetVolumeReviewSystemPrompt returns system prompt for volume review
func GetVolumeReviewSystemPrompt(language string) string {
	if language == "zh" {
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

// ChapterDraftContent holds chapter and its draft for volume review
type ChapterDraftContent struct {
	Chapter *models.Chapter
	Draft   string
}

// BuildVolumeReviewPrompt builds prompt for reviewing entire volume
func BuildVolumeReviewPrompt(setup *models.StorySetup, volume *models.Volume, chapterContents []ChapterDraftContent, language string) string {
	var sb strings.Builder

	if language == "zh" {
		sb.WriteString("# 小说卷审阅\n\n")

		sb.WriteString("## 故事设定\n")
		sb.WriteString(fmt.Sprintf("类型: %s\n", strings.Join(setup.Genres, ", ")))
		sb.WriteString(fmt.Sprintf("核心设定: %s\n\n", setup.Premise))

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
		sb.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(setup.Genres, ", ")))
		sb.WriteString(fmt.Sprintf("Premise: %s\n\n", setup.Premise))

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
