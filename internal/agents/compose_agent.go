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
	model  string
}

// NewComposeAgent creates a new ComposeAgent
func NewComposeAgent(client llm.Client, model string) *ComposeAgent {
	return &ComposeAgent{
		client: client,
		model:  model,
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

	options := &llm.ChatOptions{
		Temperature: 0.8,
		MaxTokens:   50000,
		Model:       a.model,
	}

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

	options := &llm.ChatOptions{
		Temperature: 0.8,
		MaxTokens:   50000,
		Model:       a.model,
	}

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
