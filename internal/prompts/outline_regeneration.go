package prompts

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
)

// registerOutlineRegenPrompts registers all outline regeneration prompts
func registerOutlineRegenPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineRegen,
		Name:         "part",
		Description:  "Regenerate a part",
		SystemPrompt: outlineRegenSystemPrompt,
		OutputFormat: FormatJSON,
		OutputModel:  models.Part{}, // Auto-generate schema from struct
	})

	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineRegen,
		Name:         "volume",
		Description:  "Regenerate a volume",
		SystemPrompt: outlineRegenSystemPrompt,
		OutputFormat: FormatJSON,
		OutputModel:  models.Volume{}, // Auto-generate schema from struct
	})

	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineRegen,
		Name:         "chapter",
		Description:  "Regenerate a chapter",
		SystemPrompt: outlineRegenSystemPrompt,
		OutputFormat: FormatJSON,
		OutputModel:  models.Chapter{}, // Auto-generate schema from struct
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

REGENERATION REQUIREMENTS - CRITICAL:
1. CONTINUITY IS PARAMOUNT: The regenerated chapter MUST directly follow from the events of previous chapters. Reference specific events, character states, and plot developments from the "Previous Chapters" section.
2. SET UP THE NEXT CHAPTER: This chapter MUST naturally lead into the "Next Chapter" described in the context. Create a bridge that makes the next chapter's events feel inevitable.
3. PRESERVE EVENTS: If the current chapter has events listed, ensure the regenerated version includes those same events (or appropriate replacements that serve the same narrative purpose).
4. CHARACTER CONSISTENCY: Characters must behave consistently with their established emotional states and relationships from previous chapters.
5. CAUSAL LOGIC: Every plot beat must be a logical consequence of what came before. Ask "Why is this happening NOW?" and "What caused this?"

Please regenerate this {{element_type}}, ensuring:
1. Strict continuity with previous chapters - reference specific events and their consequences
2. Proper setup for the next chapter - create narrative momentum
3. Alignment with overall story tone
4. Consideration of user suggestions
5. ALL content in the specified language`
