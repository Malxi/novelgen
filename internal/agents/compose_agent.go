package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/models"
)

// ComposeAgent handles AI generation for story outline
type ComposeAgent struct {
	client llm.Client
	config *llm.Config
}

// NewComposeAgent creates a new ComposeAgent
func NewComposeAgent(client llm.Client, config *llm.Config) *ComposeAgent {
	return &ComposeAgent{
		client: client,
		config: config,
	}
}

// GenerateOutline generates a story outline from a story setup
func (a *ComposeAgent) GenerateOutline(setup *models.StorySetup) (*models.Outline, error) {
	fmt.Println("🤖 Generating story outline with AI...")
	fmt.Println()

	// Build the system prompt
	systemPrompt := fmt.Sprintf(`You are a creative writing assistant specializing in novel outlining.
Your task is to generate a detailed story outline based on the story setup provided.

The outline must follow a strict 3-level structure: Parts → Volumes → Chapters.

Respond ONLY with a valid JSON object in the following format:
{
  "parts": [
    {
      "id": "part_1",
      "title": "Part Title",
      "summary": "Brief summary of this part",
      "volumes": [
        {
          "id": "vol_1_1",
          "title": "Volume Title",
          "summary": "Brief summary of this volume",
          "chapters": [
            {
              "id": "chap_1_1_1",
              "title": "Chapter Title",
              "summary": "Brief summary of this chapter",
              "beats": ["Plot beat 1", "Plot beat 2", "Plot beat 3"],
              "conflict": "Main conflict in this chapter",
              "pacing": "slow|normal|fast"
            }
          ]
        }
      ]
    }
  ]
}

Guidelines:
- Create 2-3 parts for a complete story
- Each part should have 1-3 volumes
- Each volume should have 2-5 chapters
- Ensure the outline follows a coherent narrative arc
- Include specific plot beats for each chapter
- Vary the pacing (slow/normal/fast) based on the story needs
- Make conflicts clear and compelling

Story Setup:
- Project Name: %s
- Genres: %s
- Premise: %s
- Theme: %s
- Rules: %s
- Tone: %s
- Tense: %s
- POV: %s`,
		setup.ProjectName,
		strings.Join(setup.Genres, ", "),
		setup.Premise,
		setup.Theme,
		strings.Join(setup.Rules, "; "),
		setup.Tone,
		setup.Tense,
		setup.POVStyle,
	)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: "Generate a complete story outline based on the story setup above.",
		},
	}

	options := a.config.GetChatOptions()

	fmt.Println("Sending request to AI (this may take a while)...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	fmt.Printf("Received response (%d tokens used)\n", resp.Usage.TotalTokens)
	fmt.Println()

	// Parse the JSON response
	var outline models.Outline
	if err := json.Unmarshal([]byte(resp.Content), &outline); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &outline); err != nil {
			return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, resp.Content)
		}
	}

	// Validate the outline
	if len(outline.Parts) == 0 {
		return nil, fmt.Errorf("AI did not generate any parts")
	}

	fmt.Printf("✓ Generated outline with %d part(s)\n", len(outline.Parts))
	for _, part := range outline.Parts {
		chapterCount := 0
		for _, vol := range part.Volumes {
			chapterCount += len(vol.Chapters)
		}
		fmt.Printf("  - %s: %d volume(s), %d chapter(s)\n", part.Title, len(part.Volumes), chapterCount)
	}
	fmt.Println()

	return &outline, nil
}

// GenerateOutlineWithStructure generates a story outline with a predefined structure
func (a *ComposeAgent) GenerateOutlineWithStructure(setup *models.StorySetup, structure models.StoryStructure, language string) (*models.Outline, error) {
	fmt.Println("🤖 Generating story outline with AI...")
	fmt.Println()

	totalChapters := structure.TotalChapters()

	// Build the system prompt with strict structure requirements
	promptData := buildPromptData(structure, totalChapters, setup, language)
	systemPrompt := buildSystemPrompt(promptData)

	userContent := buildUserPrompt(structure, language)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userContent,
		},
	}

	options := a.config.GetChatOptions()

	fmt.Println("Sending request to AI (this may take a while)...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	fmt.Printf("Received response (%d tokens used)\n", resp.Usage.TotalTokens)
	fmt.Println()

	// Parse the JSON response
	var outline models.Outline
	if err := json.Unmarshal([]byte(resp.Content), &outline); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &outline); err != nil {
			return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, resp.Content)
		}
	}

	// Validate the outline structure
	if len(outline.Parts) != structure.TargetParts {
		return nil, fmt.Errorf("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
	}

	for i, part := range outline.Parts {
		if len(part.Volumes) != structure.TargetVolumes {
			return nil, fmt.Errorf("part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), structure.TargetVolumes)
		}
		for j, volume := range part.Volumes {
			if len(volume.Chapters) != structure.TargetChapters {
				return nil, fmt.Errorf("volume %d.%d has %d chapters, but %d were requested", i+1, j+1, len(volume.Chapters), structure.TargetChapters)
			}
		}
	}

	fmt.Printf("✓ Generated outline with %d part(s), %d volume(s) per part, %d chapter(s) per volume\n",
		len(outline.Parts), structure.TargetVolumes, structure.TargetChapters)
	fmt.Printf("  Total: %d chapters\n", totalChapters)
	fmt.Println()

	return &outline, nil
}

// PromptData holds data for building prompts
type PromptData struct {
	Structure     models.StoryStructure
	TotalChapters int
	Setup         *models.StorySetup
	Language      string
	LanguageName  string
}

// RegeneratePart regenerates a single part with user suggestions
func (a *ComposeAgent) RegeneratePart(part *models.Part, outline *models.Outline, setup *models.StorySetup, language, userPrompt string) error {
	fmt.Printf("🤖 Regenerating part with AI...\n")

	// Build context from surrounding parts
	context := a.buildPartContext(part, outline)

	// Build regeneration prompt
	regenPrompt := a.buildRegenPrompt("part", part.Title, context, setup, language, userPrompt)

	// Call AI
	messages := []llm.Message{
		{Role: "system", Content: regenPrompt},
		{Role: "user", Content: a.getRegenUserPrompt("part", language, userPrompt)},
	}

	options := a.config.GetChatOptions()
	// Use smaller max tokens for part regeneration
	options.MaxTokens = 10000

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	// Parse response
	var newPart models.Part
	if err := json.Unmarshal([]byte(resp.Content), &newPart); err != nil {
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &newPart); err != nil {
			return fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	// Update part
	part.Title = newPart.Title
	part.Summary = newPart.Summary

	fmt.Printf("✓ Part regenerated: %s\n", part.Title)
	return nil
}

// RegenerateVolume regenerates a single volume with user suggestions
func (a *ComposeAgent) RegenerateVolume(volume *models.Volume, outline *models.Outline, setup *models.StorySetup, language, userPrompt string) error {
	fmt.Printf("🤖 Regenerating volume with AI...\n")

	// Build context
	context := a.buildVolumeContext(volume, outline)

	// Build regeneration prompt
	regenPrompt := a.buildRegenPrompt("volume", volume.Title, context, setup, language, userPrompt)

	// Call AI
	messages := []llm.Message{
		{Role: "system", Content: regenPrompt},
		{Role: "user", Content: a.getRegenUserPrompt("volume", language, userPrompt)},
	}

	options := a.config.GetChatOptions()
	// Use smaller max tokens for volume regeneration
	options.MaxTokens = 10000

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	// Parse response
	var newVolume models.Volume
	if err := json.Unmarshal([]byte(resp.Content), &newVolume); err != nil {
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &newVolume); err != nil {
			return fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	// Update volume (keep chapters)
	volume.Title = newVolume.Title
	volume.Summary = newVolume.Summary

	fmt.Printf("✓ Volume regenerated: %s\n", volume.Title)
	return nil
}

// RegenerateChapter regenerates a single chapter with user suggestions
func (a *ComposeAgent) RegenerateChapter(chapter *models.Chapter, outline *models.Outline, setup *models.StorySetup, language, userPrompt string) error {
	fmt.Printf("🤖 Regenerating chapter with AI...\n")

	// Build context
	context := a.buildChapterContext(chapter, outline)

	// Build regeneration prompt
	regenPrompt := a.buildRegenPrompt("chapter", chapter.Title, context, setup, language, userPrompt)

	// Call AI
	messages := []llm.Message{
		{Role: "system", Content: regenPrompt},
		{Role: "user", Content: a.getRegenUserPrompt("chapter", language, userPrompt)},
	}

	options := a.config.GetChatOptions()
	// Use smaller max tokens for chapter regeneration
	options.MaxTokens = 5000

	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	// Parse response
	var newChapter models.Chapter
	if err := json.Unmarshal([]byte(resp.Content), &newChapter); err != nil {
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &newChapter); err != nil {
			return fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	// Update chapter
	chapter.Title = newChapter.Title
	chapter.Summary = newChapter.Summary
	chapter.Beats = newChapter.Beats
	chapter.Conflict = newChapter.Conflict
	chapter.Pacing = newChapter.Pacing

	fmt.Printf("✓ Chapter regenerated: %s\n", chapter.Title)
	return nil
}

// buildPartContext builds context for part regeneration
func (a *ComposeAgent) buildPartContext(part *models.Part, outline *models.Outline) string {
	var context strings.Builder

	// Find part index
	partIdx := -1
	for i, p := range outline.Parts {
		if p.ID == part.ID {
			partIdx = i
			break
		}
	}

	if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		context.WriteString(fmt.Sprintf("Previous Part (%s): %s\nSummary: %s\n\n",
			prevPart.ID, prevPart.Title, prevPart.Summary))
	}

	if partIdx < len(outline.Parts)-1 {
		nextPart := outline.Parts[partIdx+1]
		context.WriteString(fmt.Sprintf("Next Part (%s): %s\nSummary: %s\n\n",
			nextPart.ID, nextPart.Title, nextPart.Summary))
	}

	return context.String()
}

// buildVolumeContext builds context for volume regeneration
func (a *ComposeAgent) buildVolumeContext(volume *models.Volume, outline *models.Outline) string {
	var context strings.Builder

	// Find volume in outline
	for _, part := range outline.Parts {
		for i, vol := range part.Volumes {
			if vol.ID == volume.ID {
				// Add part context
				context.WriteString(fmt.Sprintf("Part: %s\nSummary: %s\n\n", part.Title, part.Summary))

				// Add sibling volumes
				if i > 0 {
					prevVol := part.Volumes[i-1]
					context.WriteString(fmt.Sprintf("Previous Volume (%s): %s\nSummary: %s\n\n",
						prevVol.ID, prevVol.Title, prevVol.Summary))
				}
				if i < len(part.Volumes)-1 {
					nextVol := part.Volumes[i+1]
					context.WriteString(fmt.Sprintf("Next Volume (%s): %s\nSummary: %s\n\n",
						nextVol.ID, nextVol.Title, nextVol.Summary))
				}
				return context.String()
			}
		}
	}

	return context.String()
}

// buildChapterContext builds context for chapter regeneration
func (a *ComposeAgent) buildChapterContext(chapter *models.Chapter, outline *models.Outline) string {
	var context strings.Builder

	// Find chapter in outline
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i, chap := range vol.Chapters {
				if chap.ID == chapter.ID {
					// Add volume context
					context.WriteString(fmt.Sprintf("Part: %s\nVolume: %s\nVolume Summary: %s\n\n",
						part.Title, vol.Title, vol.Summary))

					// Add sibling chapters
					if i > 0 {
						prevChap := vol.Chapters[i-1]
						context.WriteString(fmt.Sprintf("Previous Chapter (%s): %s\nSummary: %s\n\n",
							prevChap.ID, prevChap.Title, prevChap.Summary))
					}
					if i < len(vol.Chapters)-1 {
						nextChap := vol.Chapters[i+1]
						context.WriteString(fmt.Sprintf("Next Chapter (%s): %s\nSummary: %s\n\n",
							nextChap.ID, nextChap.Title, nextChap.Summary))
					}
					return context.String()
				}
			}
		}
	}

	return context.String()
}

// buildRegenPrompt builds the regeneration prompt
func (a *ComposeAgent) buildRegenPrompt(elementType, currentTitle, context string, setup *models.StorySetup, language, userPrompt string) string {
	langNames := map[string]string{
		"zh": "中文",
		"en": "English",
	}
	langName := langNames[language]
	if langName == "" {
		langName = language
	}

	suggestions := userPrompt
	if suggestions == "" {
		suggestions = "Improve and enhance while maintaining consistency with the overall story"
	}

	if language == "zh" {
		return fmt.Sprintf(`你是一位专业的小说大纲创作助手。
你的任务是重新生成一个%s，基于用户的建议和故事上下文。

当前%s标题：%s

故事上下文：
%s

故事设定：
- 项目名称：%s
- 类型：%s
- 主题：%s
- 基调：%s

用户建议：%s

请重新生成这个%s，确保：
1. 与前后内容保持连贯
2. 符合整体故事基调
3. 考虑用户的建议
4. 所有内容使用%s

请只返回有效的 JSON 对象。`,
			elementType, elementType, currentTitle, context,
			setup.ProjectName, strings.Join(setup.Genres, ", "),
			setup.Theme, setup.Tone,
			suggestions, elementType, langName)
	}

	return fmt.Sprintf(`You are a professional novel outlining assistant.
Your task is to regenerate a %s based on user suggestions and story context.

Current %s title: %s

Story Context:
%s

Story Setup:
- Project Name: %s
- Genres: %s
- Theme: %s
- Tone: %s

User Suggestions: %s

Please regenerate this %s, ensuring:
1. Consistency with surrounding content
2. Alignment with overall story tone
3. Consideration of user suggestions
4. ALL content in %s

Respond ONLY with a valid JSON object.`,
		elementType, elementType, currentTitle, context,
		setup.ProjectName, strings.Join(setup.Genres, ", "),
		setup.Theme, setup.Tone,
		suggestions, elementType, langName)
}

// getRegenUserPrompt gets the user prompt for regeneration
func (a *ComposeAgent) getRegenUserPrompt(elementType, language, userPrompt string) string {
	if language == "zh" {
		if userPrompt != "" {
			return fmt.Sprintf("请根据以下建议重新生成这个%s：%s", elementType, userPrompt)
		}
		return fmt.Sprintf("请重新生成这个%s，保持与整体故事的一致性", elementType)
	}

	if userPrompt != "" {
		return fmt.Sprintf("Please regenerate this %s based on the following suggestions: %s", elementType, userPrompt)
	}
	return fmt.Sprintf("Please regenerate this %s while maintaining consistency with the overall story", elementType)
}

// buildPromptData creates prompt data with language info
func buildPromptData(structure models.StoryStructure, totalChapters int, setup *models.StorySetup, language string) PromptData {
	languageNames := map[string]string{
		"zh": "中文",
		"en": "English",
		"ja": "日本語",
		"es": "Español",
		"fr": "Français",
		"de": "Deutsch",
	}

	langName, ok := languageNames[language]
	if !ok {
		langName = language
	}

	return PromptData{
		Structure:     structure,
		TotalChapters: totalChapters,
		Setup:         setup,
		Language:      language,
		LanguageName:  langName,
	}
}

// buildSystemPrompt builds the system prompt based on language
func buildSystemPrompt(data PromptData) string {
	prompts := map[string]string{
		"zh": `你是一位专业的小说大纲创作助手。
你的任务是根据提供的故事设定，生成详细的小说大纲。

严格结构要求：
- 必须生成恰好 %d 个部
- 每个部必须包含恰好 %d 卷
- 每卷必须包含恰好 %d 章
- 总章节数：%d

大纲必须遵循严格的 3 级结构：部 → 卷 → 章

请只返回有效的 JSON 对象，格式如下：
{
  "parts": [
    {
      "id": "part_1",
      "title": "第一部标题",
      "summary": "本部的简要概述",
      "volumes": [
        {
          "id": "vol_1_1",
          "title": "第一卷标题",
          "summary": "本卷的简要概述",
          "chapters": [
            {
              "id": "chap_1_1_1",
              "title": "第一章标题",
              "summary": "本章的简要概述",
              "beats": ["情节点1", "情节点2", "情节点3"],
              "conflict": "本章的主要冲突",
              "pacing": "slow|normal|fast"
            }
          ]
        }
      ]
    }
  ]
}

重要提示：
- 所有内容（标题、概述、情节点、冲突描述）必须使用中文
- 遵循上述严格的结构要求
- 确保大纲在所有部之间有连贯的叙事弧线
- 每章包含具体的情节点（每章3-5个情节点）
- 根据故事需要调整节奏（slow/normal/fast）
- 冲突要清晰且引人入胜
- 每个部应有明确的叙事目的
- 每卷应在部内推进故事
- 每章应有清晰的发展

故事设定：
- 项目名称：%s
- 类型：%s
- 前提：%s
- 主题：%s
- 规则：%s
- 基调：%s
- 时态：%s
- 视角：%s

结构：%d 部 × %d 卷 × %d 章 = %d 总章节数`,
		"en": `You are a professional novel outlining assistant.
Your task is to generate a detailed story outline based on the provided story setup.

STRICT STRUCTURE REQUIREMENTS:
- You MUST generate exactly %d parts
- Each part MUST have exactly %d volumes
- Each volume MUST have exactly %d chapters
- Total chapters: %d

The outline must follow a strict 3-level structure: Parts → Volumes → Chapters.

Respond ONLY with a valid JSON object in the following format:
{
  "parts": [
    {
      "id": "part_1",
      "title": "Part One Title",
      "summary": "Brief summary of this part",
      "volumes": [
        {
          "id": "vol_1_1",
          "title": "Volume One Title",
          "summary": "Brief summary of this volume",
          "chapters": [
            {
              "id": "chap_1_1_1",
              "title": "Chapter One Title",
              "summary": "Brief summary of this chapter",
              "beats": ["Plot beat 1", "Plot beat 2", "Plot beat 3"],
              "conflict": "Main conflict in this chapter",
              "pacing": "slow|normal|fast"
            }
          ]
        }
      ]
    }
  ]
}

IMPORTANT:
- ALL content (titles, summaries, plot beats, conflicts) MUST be in English
- Follow the EXACT structure specified above
- Ensure the outline follows a coherent narrative arc across all parts
- Include specific plot beats for each chapter (3-5 beats per chapter)
- Vary the pacing (slow/normal/fast) based on the story needs
- Make conflicts clear and compelling
- Each part should have a clear narrative purpose
- Each volume should advance the story within its part
- Each chapter should have clear progression

Story Setup:
- Project Name: %s
- Genres: %s
- Premise: %s
- Theme: %s
- Rules: %s
- Tone: %s
- Tense: %s
- POV: %s

Structure: %d parts × %d volumes × %d chapters = %d total chapters`,
	}

	promptTemplate, ok := prompts[data.Language]
	if !ok {
		promptTemplate = prompts["en"]
	}

	return fmt.Sprintf(promptTemplate,
		data.Structure.TargetParts,
		data.Structure.TargetVolumes,
		data.Structure.TargetChapters,
		data.TotalChapters,
		data.Setup.ProjectName,
		strings.Join(data.Setup.Genres, ", "),
		data.Setup.Premise,
		data.Setup.Theme,
		strings.Join(data.Setup.Rules, "; "),
		data.Setup.Tone,
		data.Setup.Tense,
		data.Setup.POVStyle,
		data.Structure.TargetParts,
		data.Structure.TargetVolumes,
		data.Structure.TargetChapters,
		data.TotalChapters,
	)
}

// buildUserPrompt builds the user prompt based on language
func buildUserPrompt(structure models.StoryStructure, language string) string {
	prompts := map[string]string{
		"zh": "请生成一个完整的小说大纲，包含恰好 %d 个部、每部 %d 卷、每卷 %d 章。所有内容必须使用中文。",
		"en": "Generate a complete story outline with exactly %d parts, %d volumes per part, and %d chapters per volume. ALL content must be in English.",
	}

	promptTemplate, ok := prompts[language]
	if !ok {
		promptTemplate = prompts["en"]
	}

	return fmt.Sprintf(promptTemplate,
		structure.TargetParts,
		structure.TargetVolumes,
		structure.TargetChapters,
	)
}
