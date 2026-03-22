package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/logger"
)

// registerCraftReviewPrompts registers all craft review and improvement prompts
func registerCraftReviewPrompts(pm *PromptManager) {
	// Character review prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillCharacterReview,
		Name:         "default",
		Description:  "Review characters for quality and consistency",
		SystemPrompt: buildCharacterReviewSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  CraftReviewResult{},
	})

	// Location review prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillLocationReview,
		Name:         "default",
		Description:  "Review locations for quality and consistency",
		SystemPrompt: buildLocationReviewSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  CraftReviewResult{},
	})

	// Item review prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillItemReview,
		Name:         "default",
		Description:  "Review items for quality and consistency",
		SystemPrompt: buildItemReviewSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  CraftReviewResult{},
	})

	// Character improvement prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillCharacterImprovement,
		Name:         "default",
		Description:  "Improve characters based on review suggestions",
		SystemPrompt: buildCharacterImprovementSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  map[string]interface{}{}, // Will be Character map
	})

	// Location improvement prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillLocationImprovement,
		Name:         "default",
		Description:  "Improve locations based on review suggestions",
		SystemPrompt: buildLocationImprovementSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  map[string]interface{}{}, // Will be Location map
	})

	// Item improvement prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillItemImprovement,
		Name:         "default",
		Description:  "Improve items based on review suggestions",
		SystemPrompt: buildItemImprovementSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  map[string]interface{}{}, // Will be Item map
	})
}

// CraftReviewResult is the expected output structure for craft reviews
type CraftReviewResult struct {
	OverallScore     float64                 `json:"overall_score"`
	ConsistencyScore float64                 `json:"consistency_score"`
	DepthScore       float64                 `json:"depth_score"`
	OriginalityScore float64                 `json:"originality_score"`
	Suggestions      []CraftReviewSuggestion `json:"suggestions"`
}

// CraftReviewSuggestion represents a single suggestion
type CraftReviewSuggestion struct {
	ElementName string `json:"element_name"`
	Issue       string `json:"issue"`
	Suggestion  string `json:"suggestion"`
	Priority    string `json:"priority"`
}

func buildCharacterReviewSystemPrompt() string {
	return `You are a professional character editor for novels. Your task is to review character profiles and identify areas for improvement.

REVIEW CRITERIA:
1. Consistency - Do characters have consistent personalities, backgrounds, and motivations?
2. Depth - Are characters multi-dimensional with clear motivations, flaws, and growth potential?
3. Originality - Are characters unique and memorable, avoiding clichés?
4. Story Fit - Do characters fit the story's genre, tone, and world?
5. Static-only compliance - Do profiles avoid relationships, goals, character arcs, and fears?

OUTPUT FORMAT:
Return a JSON object with the following structure:
{
  "overall_score": 75,
  "consistency_score": 80,
  "depth_score": 70,
  "originality_score": 75,
  "suggestions": [
    {
      "element_name": "Character Name",
      "issue": "Description of the issue",
      "suggestion": "How to fix it",
      "priority": "high" // high, medium, or low
    }
  ]
}

SCORING GUIDE:
- 90-100: Exceptional, publication-ready
- 80-89: Good, minor improvements needed
- 70-79: Acceptable, some issues to address
- 60-69: Needs significant work
- Below 60: Major revision required

PRIORITY GUIDE:
- high: Critical issues that affect story quality (inconsistencies, flat characters)
- medium: Improvements that would enhance the story
- low: Nice-to-have enhancements`
}

func buildLocationReviewSystemPrompt() string {
	return `You are a professional world-building editor for novels. Your task is to review location descriptions and identify areas for improvement.

REVIEW CRITERIA:
1. Consistency - Do locations fit together logically in the world?
2. Depth - Are locations richly detailed with sensory information?
3. Atmosphere - Do locations have distinct moods and feelings?
4. Story Significance - Do locations serve the story effectively?
5. Sensory Details - Are all five senses represented where appropriate?

OUTPUT FORMAT:
Return a JSON object with the following structure:
{
  "overall_score": 75,
  "consistency_score": 80,
  "depth_score": 70,
  "originality_score": 75,
  "suggestions": [
    {
      "element_name": "Location Name",
      "issue": "Description of the issue",
      "suggestion": "How to fix it",
      "priority": "high" // high, medium, or low
    }
  ]
}

SCORING GUIDE:
- 90-100: Exceptional, immersive and vivid
- 80-89: Good, minor improvements needed
- 70-79: Acceptable, some issues to address
- 60-69: Needs significant work
- Below 60: Major revision required

PRIORITY GUIDE:
- high: Critical issues (contradictions, lack of essential details)
- medium: Improvements that would enhance immersion
- low: Nice-to-have enhancements`
}

func buildItemReviewSystemPrompt() string {
	return `You are a professional item/magic system editor for novels. Your task is to review item descriptions and identify areas for improvement.

REVIEW CRITERIA:
1. Consistency - Do items follow the rules of the world?
2. Significance - Do items have clear importance to the story?
3. Originality - Are items unique and interesting?
4. Function Clarity - Are item functions and limitations clear?
5. Integration - Do items connect well to characters and plot?

OUTPUT FORMAT:
Return a JSON object with the following structure:
{
  "overall_score": 75,
  "consistency_score": 80,
  "depth_score": 70,
  "originality_score": 75,
  "suggestions": [
    {
      "element_name": "Item Name",
      "issue": "Description of the issue",
      "suggestion": "How to fix it",
      "priority": "high" // high, medium, or low
    }
  ]
}

SCORING GUIDE:
- 90-100: Exceptional, iconic items
- 80-89: Good, minor improvements needed
- 70-79: Acceptable, some issues to address
- 60-69: Needs significant work
- Below 60: Major revision required

PRIORITY GUIDE:
- high: Critical issues (rule violations, unclear functions)
- medium: Improvements that would enhance the story
- low: Nice-to-have enhancements`
}

func buildCharacterImprovementSystemPrompt() string {
	return `You are a professional character editor. Your task is to improve character profiles based on the provided suggestions.

IMPROVEMENT GUIDELINES:
1. Address all high-priority suggestions
2. Maintain character consistency while adding depth
3. Preserve static-only profiles (no relationships, goals, character arcs, or fears)
4. Add specific details that writers can use
5. Ensure characters fit the story's genre and tone

IMPORTANT:
- Return the COMPLETE set of characters with improvements applied
- Do not remove any characters
- Maintain the original JSON structure
- Enhance existing fields rather than replacing them entirely
- Add new fields only if they add meaningful depth`
}

func buildLocationImprovementSystemPrompt() string {
	return `You are a professional world-building editor. Your task is to improve location descriptions based on the provided suggestions.

IMPROVEMENT GUIDELINES:
1. Address all high-priority suggestions
2. Enhance sensory details (sights, sounds, smells, textures)
3. Strengthen atmosphere and mood
4. Clarify location significance to the story
5. Ensure locations fit together logically

IMPORTANT:
- Return the COMPLETE set of locations with improvements applied
- Do not remove any locations
- Maintain the original JSON structure
- Enhance existing fields rather than replacing them entirely
- Add sensory_details object if missing or incomplete`
}

func buildItemImprovementSystemPrompt() string {
	return `You are a professional item/magic system editor. Your task is to improve item descriptions based on the provided suggestions.

IMPROVEMENT GUIDELINES:
1. Address all high-priority suggestions
2. Clarify item functions and limitations
3. Enhance item significance to the story
4. Ensure consistency with world rules
5. Add interesting details about history and origin

IMPORTANT:
- Return the COMPLETE set of items with improvements applied
- Do not remove any items
- Maintain the original JSON structure
- Enhance existing fields rather than replacing them entirely
- Ensure powers and limitations are clearly defined`
}

// buildCraftReviewUserPrompt builds user prompt for craft review
func buildCraftReviewUserPrompt(elementType string, data map[string]interface{}) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Review\n\n", strings.Title(elementType)))

	if setup, ok := data["story_setup"].(string); ok {
		sb.WriteString("## Story Setup\n")
		sb.WriteString(setup)
		sb.WriteString("\n\n")
	}

	if outline, ok := data["outline"].(string); ok {
		sb.WriteString("## Story Outline\n")
		sb.WriteString(outline)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("## %s to Review\n", strings.Title(elementType)))
	if elements, ok := data["elements"].(string); ok {
		sb.WriteString(elements)
	}

	sb.WriteString("\n\nPlease review these ")
	sb.WriteString(elementType)
	sb.WriteString(" and provide improvement suggestions following the output format specified in your instructions.")

	return sb.String()
}

// buildCraftImprovementUserPrompt builds user prompt for craft improvement
func buildCraftImprovementUserPrompt(elementType string, data map[string]interface{}) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Improvement\n\n", strings.Title(elementType)))

	if setup, ok := data["story_setup"].(string); ok {
		sb.WriteString("## Story Setup\n")
		sb.WriteString(setup)
		sb.WriteString("\n\n")
	}

	if outline, ok := data["outline"].(string); ok {
		sb.WriteString("## Story Outline\n")
		sb.WriteString(outline)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("## Current %s\n", strings.Title(elementType)))
	if elements, ok := data["elements"].(string); ok {
		sb.WriteString(elements)
	}

	sb.WriteString("\n\n## Suggestions to Address\n")
	if suggestions, ok := data["suggestions"]; ok {
		switch typed := suggestions.(type) {
		case []CraftReviewSuggestion:
			for i, suggestion := range typed {
				sb.WriteString(fmt.Sprintf("\n%d. %s (Priority: %s)\n", i+1, suggestion.ElementName, suggestion.Priority))
				sb.WriteString(fmt.Sprintf("   Issue: %s\n", suggestion.Issue))
				sb.WriteString(fmt.Sprintf("   Suggestion: %s\n", suggestion.Suggestion))
			}
		case []interface{}:
			for i, s := range typed {
				if suggestion, ok := s.(map[string]interface{}); ok {
					sb.WriteString(fmt.Sprintf("\n%d. %s (Priority: %s)\n", i+1, suggestion["element_name"], suggestion["priority"]))
					sb.WriteString(fmt.Sprintf("   Issue: %s\n", suggestion["issue"]))
					sb.WriteString(fmt.Sprintf("   Suggestion: %s\n", suggestion["suggestion"]))
				}
			}
		default:
			payload, err := json.Marshal(typed)
			if err != nil {
				logger.Debug("Failed to marshal suggestions for prompt: %v", err)
				break
			}
			var fallback []map[string]interface{}
			if err := json.Unmarshal(payload, &fallback); err != nil {
				logger.Debug("Failed to unmarshal suggestions for prompt: %v", err)
				break
			}
			for i, suggestion := range fallback {
				sb.WriteString(fmt.Sprintf("\n%d. %s (Priority: %s)\n", i+1, suggestion["element_name"], suggestion["priority"]))
				sb.WriteString(fmt.Sprintf("   Issue: %s\n", suggestion["issue"]))
				sb.WriteString(fmt.Sprintf("   Suggestion: %s\n", suggestion["suggestion"]))
			}
		}
	}

	if customPrompt, ok := data["custom_prompt"].(string); ok && customPrompt != "" {
		sb.WriteString(fmt.Sprintf("\n## Additional Instructions\n%s\n", customPrompt))
	}

	sb.WriteString(fmt.Sprintf("\n\nPlease improve these %s based on the suggestions provided. Return the complete improved set in the same JSON format.", elementType))

	return sb.String()
}
