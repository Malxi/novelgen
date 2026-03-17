package prompts

import (
	"fmt"
	"strings"

	"nolvegen/internal/models"
)

// registerOutlineRegenPrompts registers all outline regeneration prompts
func registerOutlineRegenPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineRegen,
		Name:         "part",
		Description:  "Regenerate a part",
		SystemPrompt: outlineRegenSystemPrompt,
		OutputFormat: FormatJSON,
		OutputSchema: partSchema,
	})

	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineRegen,
		Name:         "volume",
		Description:  "Regenerate a volume",
		SystemPrompt: outlineRegenSystemPrompt,
		OutputFormat: FormatJSON,
		OutputSchema: volumeSchema,
	})

	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineRegen,
		Name:         "chapter",
		Description:  "Regenerate a chapter",
		SystemPrompt: outlineRegenSystemPrompt,
		OutputFormat: FormatJSON,
		OutputSchema: chapterSchema,
	})
}

// buildOutlineRegenUserPrompt builds user prompt for outline regeneration
func buildOutlineRegenUserPrompt(data map[string]interface{}) string {
	elementType, _ := data["element_type"].(string)
	suggestions, _ := data["suggestions"].(string)
	language, _ := data["language"].(string)

	if suggestions == "" {
		if language == "zh" {
			return fmt.Sprintf("请重新生成这个%s，保持与整体故事的一致性", elementType)
		}
		return fmt.Sprintf("Please regenerate this %s while maintaining consistency with the overall story", elementType)
	}

	if language == "zh" {
		return fmt.Sprintf("请根据以下建议重新生成这个%s：%s", elementType, suggestions)
	}
	return fmt.Sprintf("Please regenerate this %s based on the following suggestions: %s", elementType, suggestions)
}

// BuildOutlineRegenData builds data for outline regeneration prompt
func BuildOutlineRegenData(elementType, currentTitle, context string, setup *models.StorySetup, language, suggestions string) map[string]interface{} {
	return map[string]interface{}{
		"element_type":  elementType,
		"current_title": currentTitle,
		"context":       context,
		"project_name":  setup.ProjectName,
		"genres":        strings.Join(setup.Genres, ", "),
		"theme":         setup.Theme,
		"tone":          setup.Tone,
		"language":      language,
		"suggestions":   suggestions,
	}
}

const outlineRegenSystemPrompt = `You are a professional novel outlining assistant.
Your task is to regenerate a {{element_type}} based on user suggestions and story context.

Current {{element_type}} title: {{current_title}}

Story Context:
{{context}}

Story Setup:
- Project Name: {{project_name}}
- Genres: {{genres}}
- Theme: {{theme}}
- Tone: {{tone}}

User Suggestions: {{suggestions}}

Please regenerate this {{element_type}}, ensuring:
1. Consistency with surrounding content
2. Alignment with overall story tone
3. Consideration of user suggestions
4. ALL content in the specified language`

const partSchema = `{
  "id": "part_1",
  "title": "Part Title",
  "summary": "Brief summary of this part"
}`

const volumeSchema = `{
  "id": "vol_1_1",
  "title": "Volume Title",
  "summary": "Brief summary of this volume"
}`

const chapterSchema = `{
  "id": "chap_1_1_1",
  "title": "Chapter Title",
  "summary": "Brief summary of this chapter",
  "beats": ["Plot beat 1", "Plot beat 2", "Plot beat 3"],
  "conflict": "Main conflict in this chapter",
  "pacing": "slow|normal|fast"
}`
