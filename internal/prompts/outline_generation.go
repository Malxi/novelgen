package prompts

import (
	"fmt"
	"nolvegen/internal/models"
)

// registerOutlineGenPrompts registers all outline generation prompts
func registerOutlineGenPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineGen,
		Name:         "with_structure",
		Description:  "Generate outline with predefined structure",
		SystemPrompt: outlineGenSystemPrompt,
		OutputFormat: FormatJSON,
		OutputModel:  models.Outline{}, // Auto-generate schema from struct
	})
}

// buildOutlineGenUserPrompt builds user prompt for outline generation
func buildOutlineGenUserPrompt(data map[string]interface{}) string {
	parts, _ := data["parts"].(int)
	volumes, _ := data["volumes"].(int)
	chapters, _ := data["chapters"].(int)
	language, _ := data["language"].(string)

	langName := "the specified language"
	if language == "zh" {
		langName = "Chinese"
	} else if language == "en" {
		langName = "English"
	}

	return fmt.Sprintf("Generate a complete story outline with exactly %d parts, %d volumes per part, and %d chapters per volume. ALL content must be in %s.",
		parts, volumes, chapters, langName)
}

// BuildOutlineGenData builds data for outline generation prompt
func BuildOutlineGenData(structure models.StoryStructure, setup *models.StorySetup, language string) map[string]interface{} {
	// Use StructToPrompt to convert StorySetup to formatted string
	setupPrompt := StructToPrompt(setup, "")

	return map[string]interface{}{
		"parts":          structure.TargetParts,
		"volumes":        structure.TargetVolumes,
		"chapters":       structure.TargetChapters,
		"total_chapters": structure.TotalChapters(),
		"setup":          setupPrompt,
		"language":       language,
	}
}

const outlineGenSystemPrompt = `You are a professional novel outlining assistant.
Your task is to generate a detailed story outline based on the story setup provided.

STRICT STRUCTURE REQUIREMENTS:
- You MUST generate exactly {{parts}} parts
- Each part MUST have exactly {{volumes}} volumes
- Each volume MUST have exactly {{chapters}} chapters
- Total chapters: {{total_chapters}}

The outline must follow a strict 3-level structure: Parts → Volumes → Chapters.

Story Setup Information:
{{setup}}

Guidelines:
- Follow the EXACT structure specified above
- Ensure the outline follows a coherent narrative arc across all parts
- Include specific plot beats for each chapter (3-5 beats per chapter)
- Vary the pacing (slow/normal/fast) based on the story needs
- Make conflicts clear and compelling
- Each part should have a clear narrative purpose
- Each volume should advance the story within its part
- Each chapter should have clear progression
- INCORPORATE the storylines into the outline naturally
- USE the premises and progression systems in the plot (e.g., characters should advance through the progression stages at appropriate points in the story)`

const outlineSchema = `{
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
}`
