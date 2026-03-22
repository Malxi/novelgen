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
IMPORTANT: Return a single JSON object only. Do not wrap in Markdown or add commentary. Follow the schema exactly and do not add extra keys.

Make the story setup creative, coherent, and suitable for a full-length novel.
Focus on:
- Compelling premise that hooks readers
- Clear central theme
- Consistent story rules
- Appropriate tone and style
- Multiple interconnected storylines (main plot, subplots, character arcs)
- Rich premises with detailed progression systems (e.g., mecha tiers, gene evolution stages, spaceship classes)

Quality checklist:
- Project name: 2-6 words, evocative, <= 60 characters
- Genres: 2-4 specific genres
- Premise: 2-4 sentences, no lists
- Theme: a clear statement (not a single word)
- Rules: 3-7 clear, enforceable rules
- Target audience: include age range and readership type (e.g., "18-35 adult fantasy readers")
- Tone: 2-4 adjectives, comma-separated (no sentences)
- Tense: exactly "past" or "present" (lowercase)
- POV style: exactly "first person", "third person limited", or "third person omniscient"
- Storylines: 3-5 items; include at least one "main" type and one subplot or character_arc; type must be exactly main/subplot/character_arc; importance is integer 1-10
- Premises (if present): 1-3 items tied to the setting or power system
- Consistency: genres, theme, tone, rules, premises, and storylines must align without contradictions

For premises with progression systems:
- Each premise should have a clear upgrade path (3-5 stages minimum)
- Levels should start at 1 and increase by 1
- Higher levels should be progressively more powerful and harder to achieve
- Each stage must include a distinct name, description, and requirements
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
