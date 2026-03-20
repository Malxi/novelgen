package prompts

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// Skill represents an AI capability/skill
type Skill string

const (
	// Story skills
	SkillStorySetup    Skill = "story_setup"
	SkillOutlineGen    Skill = "outline_generation"
	SkillOutlineRegen  Skill = "outline_regeneration"
	SkillOutlineReview Skill = "outline_review"

	// World building skills
	SkillCharacterCreation     Skill = "character_creation"
	SkillLocationCreation      Skill = "location_creation"
	SkillItemCreation          Skill = "item_creation"
	SkillOrganizationCreation  Skill = "organization_creation"
	SkillRaceCreation          Skill = "race_creation"
	SkillAbilitySystemCreation Skill = "ability_system_creation"
	SkillWorldLoreCreation     Skill = "world_lore_creation"

	// Craft review and improvement skills
	SkillCharacterReview      Skill = "character_review"
	SkillLocationReview       Skill = "location_review"
	SkillItemReview           Skill = "item_review"
	SkillCharacterImprovement Skill = "character_improvement"
	SkillLocationImprovement  Skill = "location_improvement"
	SkillItemImprovement      Skill = "item_improvement"

	// Writing skills
	SkillChapterWriting Skill = "chapter_writing"

	// Recap skill
	SkillChapterRecap Skill = "chapter_recap"
)

// OutputFormat represents the expected output format
type OutputFormat string

const (
	FormatJSON OutputFormat = "json"
	FormatText OutputFormat = "text"
)

// PromptTemplate defines a reusable prompt structure
type PromptTemplate struct {
	Skill        Skill
	Name         string
	Description  string
	SystemPrompt string
	OutputFormat OutputFormat
	OutputSchema string      // JSON schema string (optional, will be auto-generated from OutputModel if empty)
	OutputModel  interface{} // Struct instance for auto-generating JSON schema (optional)
	Language     string
}

// PromptManager manages all prompt templates
type PromptManager struct {
	templates map[Skill]map[string]*PromptTemplate
}

// NewPromptManager creates a new prompt manager
func NewPromptManager() *PromptManager {
	pm := &PromptManager{
		templates: make(map[Skill]map[string]*PromptTemplate),
	}
	pm.registerDefaultPrompts()
	pm.registerAllPrompts() // Register additional prompts from plugin files
	return pm
}

// Register registers a prompt template
func (pm *PromptManager) Register(template *PromptTemplate) {
	if pm.templates[template.Skill] == nil {
		pm.templates[template.Skill] = make(map[string]*PromptTemplate)
	}
	pm.templates[template.Skill][template.Name] = template
}

// Get retrieves a prompt template
func (pm *PromptManager) Get(skill Skill, name string) (*PromptTemplate, bool) {
	skillTemplates, ok := pm.templates[skill]
	if !ok {
		return nil, false
	}
	template, ok := skillTemplates[name]
	return template, ok
}

// Build builds a complete prompt with data
func (pm *PromptManager) Build(skill Skill, name string, data map[string]interface{}) (string, string, error) {
	template, ok := pm.Get(skill, name)
	if !ok {
		return "", "", fmt.Errorf("prompt template not found: %s/%s", skill, name)
	}

	systemPrompt := pm.interpolate(template.SystemPrompt, data)
	outputRequirements := pm.buildOutputRequirements(template)

	fullSystemPrompt := systemPrompt + "\n\n" + outputRequirements

	userPrompt := pm.buildUserPrompt(template, data)

	return fullSystemPrompt, userPrompt, nil
}

// interpolate replaces placeholders in template using Go's text/template
func (pm *PromptManager) interpolate(tmpl string, data map[string]interface{}) string {
	// Try to use Go template first
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		// Fallback to simple string replacement for backward compatibility
		result := tmpl
		for key, value := range data {
			placeholder := fmt.Sprintf("{{%s}}", key)
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}
		return result
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		// Fallback to simple string replacement
		result := tmpl
		for key, value := range data {
			placeholder := fmt.Sprintf("{{%s}}", key)
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}
		return result
	}
	return buf.String()
}

// buildOutputRequirements builds output requirements section
func (pm *PromptManager) buildOutputRequirements(template *PromptTemplate) string {
	var parts []string

	parts = append(parts, "=== OUTPUT REQUIREMENTS ===")
	parts = append(parts, fmt.Sprintf("Format: %s", template.OutputFormat))

	// Get schema - use explicit OutputSchema or auto-generate from OutputModel
	schema := template.OutputSchema
	if schema == "" && template.OutputModel != nil {
		schema = StructToJSONSchema(template.OutputModel, "  ")
	}

	if schema != "" {
		parts = append(parts, fmt.Sprintf("Structure:\n%s", schema))
	}

	if template.Language != "" {
		parts = append(parts, fmt.Sprintf("Language: All content MUST be in %s", template.Language))
	}

	parts = append(parts, "=== END REQUIREMENTS ===")

	return strings.Join(parts, "\n")
}

// buildUserPrompt builds the user prompt based on skill type
func (pm *PromptManager) buildUserPrompt(template *PromptTemplate, data map[string]interface{}) string {
	switch template.Skill {
	case SkillStorySetup:
		return buildStorySetupUserPrompt(data)
	case SkillOutlineGen:
		return buildOutlineGenUserPrompt(data)
	case SkillOutlineRegen:
		return buildOutlineRegenUserPrompt(data)
	case SkillOutlineReview:
		return buildOutlineReviewUserPrompt(data)
	case SkillCharacterCreation, SkillLocationCreation, SkillItemCreation,
		SkillOrganizationCreation, SkillRaceCreation, SkillAbilitySystemCreation, SkillWorldLoreCreation:
		return buildCraftUserPrompt(template.Skill, data)
	case SkillCharacterReview, SkillLocationReview, SkillItemReview:
		if elementType, ok := data["element_type"].(string); ok {
			return buildCraftReviewUserPrompt(elementType, data)
		}
		return "Please review the provided elements."
	case SkillCharacterImprovement, SkillLocationImprovement, SkillItemImprovement:
		// Extract element type from skill name
		skillStr := string(template.Skill)
		if strings.Contains(skillStr, "character") {
			return buildCraftImprovementUserPrompt("characters", data)
		} else if strings.Contains(skillStr, "location") {
			return buildCraftImprovementUserPrompt("locations", data)
		} else if strings.Contains(skillStr, "item") {
			return buildCraftImprovementUserPrompt("items", data)
		}
		return "Please improve the provided elements."
	case SkillChapterWriting:
		if template.Name == "final" {
			return buildFinalChapterUserPrompt(data)
		}
		if template.Name == "improve" {
			return buildImproveChapterUserPrompt(data)
		}
		return buildChapterWritingUserPrompt(data)
	case SkillChapterRecap:
		return buildRecapUserPrompt(data)
	case SkillTranslation:
		return buildTranslateUserPrompt(data)
	default:
		return "Please generate the requested content."
	}
}

// registerDefaultPrompts registers all default prompts
func (pm *PromptManager) registerDefaultPrompts() {
	registerStorySetupPrompts(pm)
	registerOutlineGenPrompts(pm)
	registerOutlineRegenPrompts(pm)
	registerOutlineReviewPrompts(pm)
	registerCraftPrompts(pm)
	registerCraftReviewPrompts(pm)
	registerDraftPrompts(pm)
	registerWritePrompts(pm)
	registerRecapPrompts(pm)
}
