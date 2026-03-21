package prompts

import (
	"fmt"

	"novelgen/internal/models"
)

// registerStorySetupPrompts registers all story setup prompts
func registerStorySetupPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillStorySetup,
		Name:         "default",
		Description:  "Generate story setup from user idea",
		SystemPrompt: storySetupSystemPrompt,
		OutputFormat: FormatJSON,
		OutputModel:  models.StorySetup{}, // Auto-generate schema from struct
	})
}

// buildStorySetupUserPrompt builds user prompt for story setup
func buildStorySetupUserPrompt(data map[string]interface{}) string {
	idea, _ := data["idea"].(string)
	return fmt.Sprintf("Create a story setup based on this idea: %s", idea)
}

// BuildStorySetupData builds data for story setup prompt
func BuildStorySetupData(idea, language string) map[string]interface{} {
	return map[string]interface{}{
		"idea":     idea,
		"language": language,
	}
}

// GetStorySetupSystemPrompt returns the system prompt with language requirement
func GetStorySetupSystemPrompt(language string) string {
	langName := GetLanguageName(language)
	return fmt.Sprintf(`You are a creative writing assistant specializing in novel planning.
Your task is to generate a structured story setup based on the user's idea.

IMPORTANT: All output MUST be in %s. This includes project name, genres, premise, descriptions, theme, rules, target audience, tone/style, storyline names and descriptions, premise names, descriptions, and progression stages.

Make the story setup creative, coherent, and suitable for a full-length novel.
Focus on:
- Compelling premise that hooks readers
- Clear central theme
- Consistent story rules
- Appropriate tone and style
- Multiple interconnected storylines (main plot, subplots, character arcs)
- Rich premises with detailed progression systems (e.g., mecha tiers, gene evolution stages, spaceship classes)

For premises with progression systems:
- Each premise should have a clear upgrade path (3-5 stages minimum)
- Higher levels should be progressively more powerful and harder to achieve
- Include specific requirements or conditions for advancement
- Make each stage distinct and meaningful to the story`, langName)
}

// GetLanguageName returns the full language name
func GetLanguageName(code string) string {
	switch code {
	case "zh":
		return "Chinese (中文)"
	case "en":
		return "English"
	case "ja":
		return "Japanese (日本語)"
	case "ko":
		return "Korean (한국어)"
	case "es":
		return "Spanish (Español)"
	case "fr":
		return "French (Français)"
	case "de":
		return "German (Deutsch)"
	default:
		return "Chinese (中文)"
	}
}

const storySetupSystemPrompt = `You are a creative writing assistant specializing in novel planning.
Your task is to generate a structured story setup based on the user's idea.

Make the story setup creative, coherent, and suitable for a full-length novel.
Focus on:
- Compelling premise that hooks readers
- Clear central theme
- Consistent story rules
- Appropriate tone and style
- Multiple interconnected storylines (main plot, subplots, character arcs)
- Rich premises with detailed progression systems (e.g., mecha tiers, gene evolution stages, spaceship classes)

For premises with progression systems:
- Each premise should have a clear upgrade path (3-5 stages minimum)
- Higher levels should be progressively more powerful and harder to achieve
- Include specific requirements or conditions for advancement
- Make each stage distinct and meaningful to the story`
