package prompts

import (
	"fmt"
	"strings"

	"nolvegen/internal/models"
)

// registerCraftPrompts registers all craft-related prompts
func registerCraftPrompts(pm *PromptManager) {
	// Character creation prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillCharacterCreation,
		Name:         "default",
		Description:  "Generate detailed character profiles",
		SystemPrompt: buildCharacterSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  map[string]models.Character{},
	})

	// Location creation prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillLocationCreation,
		Name:         "default",
		Description:  "Generate detailed location descriptions",
		SystemPrompt: buildLocationSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  map[string]models.Location{},
	})

	// Item creation prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillItemCreation,
		Name:         "default",
		Description:  "Generate detailed item descriptions",
		SystemPrompt: buildItemSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  map[string]models.Item{},
	})
}

func buildCharacterSystemPrompt() string {
	return `You are a professional character designer for novels.
Your task is to create detailed character profiles based on the story context provided.

STRICT REQUIREMENTS:
1. Generate EXACTLY the requested characters
2. Each character MUST have a unique and memorable personality
3. Characters should fit the story's genre and style
4. Include specific details that can be referenced in writing

RELATIONSHIPS RULE (VERY IMPORTANT):
- The "relationships" field MUST ONLY include relationships that already exist at the START of the story (chapter 1 opening).
- Do NOT invent or pre-fill relationships that will be formed later in the plot.
- If you are not 100% sure a relationship exists at the start, OMIT it from "relationships".
- Keep relationships minimal: only the few strongest, canonical starting links (family, mentor/student, boss/subordinate, sworn enemies at start).

Character fields:
- name: Full name of the character
- aliases: Array of nicknames or alternative names (optional)
- age: String describing age (e.g., "25", "unknown", "appears 20 but is ancient")
- gender: male/female/other (optional)
- appearance: Detailed physical description
- personality: Array of personality traits
- background: Character history and backstory
- motivation: What drives this character
- goals: Array of character's goals
- fears: Array of fears (optional)
- skills: Array of abilities/skills (optional)
- relationships: Object mapping other character names to relationship descriptions (optional; START-OF-STORY ONLY)
- role_in_story: Character's role (protagonist/antagonist/supporting/mentor/etc)
- character_arc: How this character develops (optional)
- voice: Speaking style and mannerisms (optional)
- notes: Additional notes for writers (optional)`
}

func buildLocationSystemPrompt() string {
	return `You are a professional world builder for novels.
Your task is to create detailed location descriptions based on the story context provided.

STRICT REQUIREMENTS:
1. Generate EXACTLY the requested locations
2. Each location MUST have a distinct atmosphere
3. Locations should fit the story's genre and style
4. Include sensory details that can be used in writing

Location fields:
- name: Full location name
- type: Type of location (city/building/landmark/region/etc)
- description: Overall description
- appearance: Visual details and architecture
- atmosphere: Mood and feeling of the place
- sensory_details: Object with sights, sounds, smells, textures arrays (optional)
- significance: Why this location matters to the story
- history: Background history (optional)
- inhabitants: Array of types of people/creatures here (optional)
- connected_locations: Array of nearby place names (optional)
- events: Array of significant events that happened here (optional)
- secrets: Hidden aspects or secrets of this location as a string (optional)
- notes: Additional notes for writers (optional)`
}

func buildItemSystemPrompt() string {
	return `You are a professional item designer for novels.
Your task is to create detailed item descriptions based on the story context provided.

STRICT REQUIREMENTS:
1. Generate EXACTLY the requested items
2. Each item MUST have significance to the story
3. Items should fit the story's genre and style
4. Include details about appearance, function, and importance

Item fields:
- name: Full item name
- type: Type of item (weapon/artifact/tool/document/etc)
- description: Overall description
- appearance: Visual details
- function: What the item does or is used for
- origin: Where the item comes from (optional)
- history: Background history of the item (optional)
- powers: Array of special abilities (optional)
- limitations: Array of restrictions or weaknesses (optional)
- owner: Current or original owner (optional)
- significance: Why this item matters to the story
- related_items: Array of related item names (optional)
- secrets: Hidden aspects of this item (optional)
- notes: Additional notes for writers (optional)`
}

// buildCraftUserPrompt builds user prompt for craft skills
func buildCraftUserPrompt(skill Skill, data map[string]interface{}) string {
	switch skill {
	case SkillCharacterCreation:
		return buildCharacterUserPrompt(data)
	case SkillLocationCreation:
		return buildLocationUserPrompt(data)
	case SkillItemCreation:
		return buildItemUserPrompt(data)
	default:
		return "Please generate the requested content."
	}
}

func buildCharacterUserPrompt(data map[string]interface{}) string {
	var sb strings.Builder

	if title, ok := data["story_title"].(string); ok && title != "" {
		sb.WriteString(fmt.Sprintf("Story Title: %s\n", title))
	}
	if genre, ok := data["story_genre"].(string); ok && genre != "" {
		sb.WriteString(fmt.Sprintf("Genre: %s\n", genre))
	}
	if style, ok := data["story_style"].(string); ok && style != "" {
		sb.WriteString(fmt.Sprintf("Style: %s\n", style))
	}

	sb.WriteString("\nStory Setup:\n")
	if setup, ok := data["story_setup"].(string); ok {
		sb.WriteString(setup)
	}

	sb.WriteString("\n\nOutline Sample:\n")
	if outline, ok := data["outline_sample"].(string); ok {
		sb.WriteString(outline)
	}

	sb.WriteString("\n\nCharacters to generate: ")
	if chars, ok := data["characters"].([]string); ok {
		sb.WriteString(strings.Join(chars, ", "))
	}

	if custom, ok := data["custom_prompt"].(string); ok && custom != "" {
		sb.WriteString(fmt.Sprintf("\n\nAdditional instructions: %s", custom))
	}

	sb.WriteString("\n\nGenerate detailed character profiles for the characters listed above.")
	sb.WriteString(" Each character should fit the story's world and have a distinct personality.")
	sb.WriteString(" IMPORTANT: In 'relationships', include ONLY relationships that exist at the very start of the story. Do NOT add future relationships that happen later in the plot.")
	sb.WriteString(" Return the result as a JSON object with character names as keys.")

	return sb.String()
}

func buildLocationUserPrompt(data map[string]interface{}) string {
	var sb strings.Builder

	if title, ok := data["story_title"].(string); ok && title != "" {
		sb.WriteString(fmt.Sprintf("Story Title: %s\n", title))
	}
	if genre, ok := data["story_genre"].(string); ok && genre != "" {
		sb.WriteString(fmt.Sprintf("Genre: %s\n", genre))
	}
	if style, ok := data["story_style"].(string); ok && style != "" {
		sb.WriteString(fmt.Sprintf("Style: %s\n", style))
	}

	sb.WriteString("\nStory Setup:\n")
	if setup, ok := data["story_setup"].(string); ok {
		sb.WriteString(setup)
	}

	sb.WriteString("\n\nOutline Sample:\n")
	if outline, ok := data["outline_sample"].(string); ok {
		sb.WriteString(outline)
	}

	sb.WriteString("\n\nLocations to generate: ")
	if locs, ok := data["locations"].([]string); ok {
		sb.WriteString(strings.Join(locs, ", "))
	}

	if custom, ok := data["custom_prompt"].(string); ok && custom != "" {
		sb.WriteString(fmt.Sprintf("\n\nAdditional instructions: %s", custom))
	}

	sb.WriteString("\n\nGenerate detailed location descriptions for the locations listed above.")
	sb.WriteString(" Each location should have a distinct atmosphere and fit the story's world.")
	sb.WriteString(" Return the result as a JSON object with location names as keys.")

	return sb.String()
}

func buildItemUserPrompt(data map[string]interface{}) string {
	var sb strings.Builder

	if title, ok := data["story_title"].(string); ok && title != "" {
		sb.WriteString(fmt.Sprintf("Story Title: %s\n", title))
	}
	if genre, ok := data["story_genre"].(string); ok && genre != "" {
		sb.WriteString(fmt.Sprintf("Genre: %s\n", genre))
	}
	if style, ok := data["story_style"].(string); ok && style != "" {
		sb.WriteString(fmt.Sprintf("Style: %s\n", style))
	}

	sb.WriteString("\nStory Setup:\n")
	if setup, ok := data["story_setup"].(string); ok {
		sb.WriteString(setup)
	}

	sb.WriteString("\n\nOutline Sample:\n")
	if outline, ok := data["outline_sample"].(string); ok {
		sb.WriteString(outline)
	}

	sb.WriteString("\n\nItems to generate: ")
	if items, ok := data["items"].([]string); ok {
		sb.WriteString(strings.Join(items, ", "))
	}

	if custom, ok := data["custom_prompt"].(string); ok && custom != "" {
		sb.WriteString(fmt.Sprintf("\n\nAdditional instructions: %s", custom))
	}

	sb.WriteString("\n\nGenerate detailed item descriptions for the items listed above.")
	sb.WriteString(" Each item should have significance to the story and fit the world's setting.")
	sb.WriteString(" Return the result as a JSON object with item names as keys.")

	return sb.String()
}
