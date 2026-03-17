package prompts

import "fmt"

// registerStorySetupPrompts registers all story setup prompts
func registerStorySetupPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillStorySetup,
		Name:         "default",
		Description:  "Generate story setup from user idea",
		SystemPrompt: storySetupSystemPrompt,
		OutputFormat: FormatJSON,
		OutputSchema: storySetupSchema,
	})
}

// buildStorySetupUserPrompt builds user prompt for story setup
func buildStorySetupUserPrompt(data map[string]interface{}) string {
	idea, _ := data["idea"].(string)
	return fmt.Sprintf("Create a story setup based on this idea: %s", idea)
}

// BuildStorySetupData builds data for story setup prompt
func BuildStorySetupData(idea string) map[string]interface{} {
	return map[string]interface{}{
		"idea": idea,
	}
}

const storySetupSystemPrompt = `You are a creative writing assistant specializing in novel planning.
Your task is to generate a structured story setup based on the user's idea.

Make the story setup creative, coherent, and suitable for a full-length novel.
Focus on:
- Compelling premise that hooks readers
- Clear central theme
- Consistent story rules
- Appropriate tone and style`

const storySetupSchema = `{
  "project_name": "Title of the novel",
  "genres": ["Genre1", "Genre2"],
  "premise": "A compelling description of what the story is about",
  "theme": "The central theme (e.g., 'courage vs power', 'redemption')",
  "rules": ["Story rule 1", "Story rule 2"],
  "target_audience": "Target audience (e.g., Young Adult, Adult)",
  "tone": "Tone/style (e.g., Epic, Hopeful, Dark, Gritty)",
  "tense": "past or present",
  "pov_style": "first-person, third-person limited, or third-person omniscient"
}`
