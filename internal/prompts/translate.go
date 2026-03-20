package prompts

import "fmt"

// init registers the translation prompts
// This is called automatically when the package is imported
func init() {
	RegisterPrompts(registerTranslatePrompts)
}

// SkillTranslation is the skill identifier for translation
const SkillTranslation Skill = "translation"

// registerTranslatePrompts registers all translation prompts
func registerTranslatePrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillTranslation,
		Name:         "default",
		Description:  "Translate text between languages while preserving style",
		SystemPrompt: translateSystemPrompt,
		OutputFormat: FormatText,
	})
}

// buildTranslateUserPrompt builds user prompt for translation
func buildTranslateUserPrompt(data map[string]interface{}) string {
	content, _ := data["content"].(string)
	sourceLang, _ := data["source_lang"].(string)
	targetLang, _ := data["target_lang"].(string)

	return fmt.Sprintf(`Please translate the following text from %s to %s.

IMPORTANT TRANSLATION GUIDELINES:
1. Preserve the narrative style, tone, and voice of the original
2. Maintain character names, place names, and proper nouns appropriately
3. Keep paragraph structure and formatting intact
4. Ensure natural flow in the target language
5. Preserve dialogue style and character voice
6. Maintain cultural context where appropriate

SOURCE TEXT (%s):
%s`, sourceLang, targetLang, sourceLang, content)
}

// BuildTranslationData builds data for translation prompt
func BuildTranslationData(content, sourceLang, targetLang string) map[string]interface{} {
	return map[string]interface{}{
		"content":     content,
		"source_lang": sourceLang,
		"target_lang": targetLang,
	}
}

const translateSystemPrompt = `You are a professional literary translator specializing in novel translation.

Your task is to translate text while preserving:
- Narrative flow and pacing
- Character voice and dialogue style
- Descriptive richness and atmosphere
- Cultural nuances and idioms (adapt appropriately)
- Tone and emotional impact

Translate naturally - don't translate word-for-word. Make the text feel like it was originally written in the target language.

Output only the translated text without explanations, notes, or formatting markers.`
