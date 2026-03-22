package prompts

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
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
5. Focus on STATIC attributes only - do NOT include dynamic story elements
6. Keep all content in the specified language

IMPORTANT - DO NOT INCLUDE:
- relationships: Relationships are managed dynamically by the story system
- goals: Character goals change throughout the story
- character_arc: Character development happens during the story
- fears: Fears may be revealed or change during the story

Character fields (STATIC attributes only):
- name: Full name of the character
- aliases: Array of nicknames or alternative names (optional)
- age: String describing age (e.g., "25", "unknown", "appears 20 but is ancient")
- gender: male/female/other (optional)
- race: Race or species (optional)
- appearance: Detailed physical description
- personality: Array of personality traits
- background: Character history and backstory (focus on past, not future)
- motivation: Core inner drive (static aspect of character)
- skills: Array of abilities/skills (optional)
- abilities: Special powers or talents (optional)
- affiliations: Organizations or groups the character belongs to (optional)
- role_in_story: Character's role (protagonist/antagonist/supporting/mentor/etc)
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
5. Keep all content in the specified language

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
5. Keep all content in the specified language

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
	if language, ok := data["language"].(string); ok && language != "" {
		sb.WriteString(fmt.Sprintf("Language: %s\n", GetLanguageName(language)))
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
	sb.WriteString(" Tie each character's motivation and background to the story setup and outline details where possible.")
	sb.WriteString(" IMPORTANT: Only include STATIC character attributes. Do NOT include relationships, goals, character_arc, or fears - these are managed dynamically by the story system.")
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
	if language, ok := data["language"].(string); ok && language != "" {
		sb.WriteString(fmt.Sprintf("Language: %s\n", GetLanguageName(language)))
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
	sb.WriteString(" Emphasize why each location matters to the story setup or outline events.")
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
	if language, ok := data["language"].(string); ok && language != "" {
		sb.WriteString(fmt.Sprintf("Language: %s\n", GetLanguageName(language)))
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
	sb.WriteString(" Link each item's significance to specific story setup or outline context where possible.")
	sb.WriteString(" Return the result as a JSON object with item names as keys.")

	return sb.String()
}
