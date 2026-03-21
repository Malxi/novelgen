package prompts

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
)

// GetVolumeReviewSystemPrompt returns system prompt for volume review
func GetVolumeReviewSystemPrompt(language string) string {
	if language == "zh" {
		return `你是一位专业的小说编辑和评论家。你的任务是审阅整个卷（volume）的所有章节草稿，并提供详细的改进建议。

请从以下几个维度对每章进行评价：
1. 剧情连贯性 - 与前后章节的情节是否连贯
2. 场景/转场连续性 - 本章开头是否承接上一章结尾，有无“瞬移换地点/换话题”的断裂
3. 角色出场一致性 - 章节大纲标注的角色是否真的在正文出现；开头是否突然冒出未在大纲 characters 列表中的关键角色
4. 情节合理性 - 剧情逻辑是否合理，有无漏洞
5. 角色一致性 - 角色行为是否符合其设定
6. 节奏把控 - 故事节奏是否恰当

同时请从整个卷的层面评价：
- 卷的整体结构是否合理
- 章节之间的衔接是否流畅
- 是否有跨章节的情节漏洞

请以JSON格式输出评价结果，包含所有章节的评价。`
	}

	return `You are a professional novel editor and critic. Your task is to review all chapters in an entire volume and provide detailed improvement suggestions.

Please evaluate each chapter from the following dimensions:
1. Plot Coherence - Is the plot coherent with previous and next chapters?
2. Scene Continuity - Does the opening directly continue from the previous chapter ending, or does it "teleport" to a new place/topic without justification?
3. Character Presence - Do the outlined characters actually appear in the draft? Does the opening suddenly feature a major character not listed in the outline characters?
4. Plot Rationality - Is the plot logic reasonable? Any flaws?
5. Character Consistency - Do characters act according to their established traits?
6. Pacing - Is the story pacing appropriate?

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
      "scene_continuity": {
        "score": 6,
        "issues": ["上一章结尾在A场景对话，但本章开头直接瞬移到B场景"],
        "suggestions": ["补一个转场桥段并用上一章最后一句/最后一幕承接开场"]
      },
      "character_presence": {
        "score": 8,
        "issues": ["大纲角色A未在正文出现"],
        "suggestions": ["补写角色A出场/台词；若不出场则修正大纲 characters 列表"]
      },
      "recap_quality": {
        "score": 7,
        "issues": ["recap 缺少 next_opening_hint 或 last_line（连续性锚点不完整）"],
        "suggestions": ["确保 recap 包含 location/present/last_line/next_opening_hint；必要时重新抽取 recap"]
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
      "scene_continuity": {
        "score": 6,
        "issues": ["Previous chapter ends in scene A, but this chapter opens by teleporting to scene B"],
        "suggestions": ["Add a transition bridge and directly continue from the previous last beat/last line"]
      },
      "character_presence": {
        "score": 8,
        "issues": ["Outlined character A never appears in the draft"],
        "suggestions": ["Add character A's presence/dialogue; if they should not appear, fix the outline characters list"]
      },
      "recap_quality": {
        "score": 7,
        "issues": ["recap missing next_opening_hint or last_line (continuity anchor incomplete)"],
        "suggestions": ["Ensure recap includes location/present/last_line/next_opening_hint; re-extract recap if needed"]
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
