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

// GetStorySetupSchema returns the JSON schema for story setup
func GetStorySetupSchema() string {
	return storySetupSchema
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

const storySetupSchema = `{
  "project_name": "小说标题（中文）",
  "genres": ["类型1", "类型2"],
  "premise": "故事核心设定的中文描述",
  "theme": "核心主题（如：勇气与力量、救赎、成长等）",
  "rules": ["故事规则1", "故事规则2"],
  "target_audience": "目标读者（如：青少年、成人、全年龄）",
  "tone": "风格基调（如：史诗、希望、黑暗、热血、悬疑）",
  "tense": "past 或 present",
  "pov_style": "first_person, third_person_limited, 或 third_person_omniscient",
  "storylines": [
    {
      "name": "故事线名称（如：主角的成长之路）",
      "description": "故事线的详细中文描述",
      "type": "main, subplot, 或 character_arc",
      "importance": 10
    }
  ],
  "premises": [
    {
      "name": "设定名称（如：战斗机甲）",
      "description": "该设定的中文描述",
      "category": "类别如：机甲, 基因进化, 飞船, 魔法, 武道, 异能等",
      "progression": [
        {
          "level": 1,
          "name": "阶段名称（如：标准型机甲）",
          "description": "该阶段的能力和特点的中文描述",
          "requirements": "晋升到该阶段的要求"
        },
        {
          "level": 2,
          "name": "阶段名称（如：精锐型机甲）",
          "description": "该阶段的能力和特点的中文描述",
          "requirements": "晋升到该阶段的要求"
        },
        {
          "level": 3,
          "name": "阶段名称（如：传奇型机甲）",
          "description": "该阶段的能力和特点的中文描述",
          "requirements": "晋升到该阶段的要求"
        }
      ]
    }
  ]
}`
