package prompts

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ReviewSuggestion represents a single improvement suggestion
type ReviewSuggestion struct {
	Type       string `json:"type"`       // "part", "volume", "chapter"
	ID         string `json:"id"`         // e.g., "1", "1_1", "1_1_1"
	Title      string `json:"title"`      // Current title
	Issue      string `json:"issue"`      // Description of the problem
	Suggestion string `json:"suggestion"` // Specific improvement suggestion
	Priority   string `json:"priority"`   // "high", "medium", "low"
}

// ReviewResult represents the complete review output
type ReviewResult struct {
	OverallScore    float64            `json:"overall_score"`    // 0-100
	LogicScore      float64            `json:"logic_score"`      // Plot logic 0-100
	EngagementScore float64            `json:"engagement_score"` // Reader engagement 0-100
	PacingScore     float64            `json:"pacing_score"`     // Story pacing 0-100
	CoherenceScore  float64            `json:"coherence_score"`  // Narrative coherence 0-100
	Summary         string             `json:"summary"`          // Overall assessment
	Strengths       []string           `json:"strengths"`        // What's working well
	Weaknesses      []string           `json:"weaknesses"`       // Areas needing improvement
	Suggestions     []ReviewSuggestion `json:"suggestions"`      // Specific suggestions
}

// registerOutlineReviewPrompts registers all outline review prompts
func registerOutlineReviewPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineReview,
		Name:         "default",
		Description:  "Review outline for logical consistency, engagement, and pacing",
		SystemPrompt: outlineReviewSystemPrompt,
		OutputFormat: FormatJSON,
		OutputSchema: outlineReviewSchema,
	})
}

// BuildOutlineReviewData builds data for outline review prompt
func BuildOutlineReviewData(outlineJSON string, storySetup string, iteration int) map[string]interface{} {
	return map[string]interface{}{
		"outline":   outlineJSON,
		"setup":     storySetup,
		"iteration": iteration,
	}
}

// SkillOutlineReview is defined in base.go

const outlineReviewSystemPrompt = `You are an expert story editor and literary critic specializing in novel structure and narrative design.

Your task is to critically review a story outline and provide detailed feedback for improvement.

Story Setup:
{{setup}}

Review Criteria:
1. **Logic & Consistency**: Check for plot holes, contradictions, cause-and-effect problems
2. **Engagement**: Assess hook strength, tension building, reader interest maintenance
3. **Pacing**: Evaluate story rhythm, balance between action and reflection, climax placement
4. **Character Arc**: Verify character development progression through the outline
5. **Theme Integration**: Check if themes are consistently developed
6. **Structural Balance**: Ensure parts/volumes/chapters are well-proportioned

Scoring Guide (0-100):
- 90-100: Exceptional, publish-ready quality
- 80-89: Strong, minor improvements needed
- 70-79: Good, several areas need work
- 60-69: Fair, significant revision recommended
- Below 60: Poor, major restructuring needed

Provide SPECIFIC, ACTIONABLE suggestions. Each suggestion must identify:
- Exact location (part/volume/chapter ID)
- Specific issue
- Concrete improvement recommendation
- Priority level (high/medium/low)

Respond ONLY with a valid JSON object.`

const outlineReviewSchema = `{
  "overall_score": <number 0-100>,
  "logic_score": <number 0-100>,
  "engagement_score": <number 0-100>,
  "pacing_score": <number 0-100>,
  "coherence_score": <number 0-100>,
  "summary": "<overall assessment in 2-3 sentences>",
  "strengths": ["<strength 1>", "<strength 2>", ...],
  "weaknesses": ["<weakness 1>", "<weakness 2>", ...],
  "suggestions": [
    {
      "type": "<part|volume|chapter>",
      "id": "<e.g., 1, 1_1, 1_1_1>",
      "title": "<current title>",
      "issue": "<specific problem description>",
      "suggestion": "<concrete improvement recommendation>",
      "priority": "<high|medium|low>"
    }
  ]
}`

// ParseReviewResult parses the AI response into ReviewResult
func ParseReviewResult(content string) (*ReviewResult, error) {
	var result ReviewResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Try to extract JSON from markdown
		content = extractJSONFromMarkdownForReview(content)
		if err := json.Unmarshal([]byte(content), &result); err != nil {
			return nil, fmt.Errorf("failed to parse review result: %w", err)
		}
	}
	return &result, nil
}

// extractJSONFromMarkdownForReview extracts JSON from markdown code blocks if present
func extractJSONFromMarkdownForReview(content string) string {
	// Look for JSON in code blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}
	if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + 3
		end := strings.Index(content[start:], "```")
		if end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}
	return content
}

// buildOutlineReviewUserPrompt builds the user prompt for outline review
func buildOutlineReviewUserPrompt(data map[string]interface{}) string {
	outline, _ := data["outline"].(string)
	iteration, _ := data["iteration"].(int)

	var sb strings.Builder
	sb.WriteString("Please review the following story outline:\n\n")

	if iteration > 0 {
		sb.WriteString(fmt.Sprintf("This is iteration %d of the improvement process.\n\n", iteration))
	}

	sb.WriteString("=== OUTLINE ===\n")
	sb.WriteString(outline)
	sb.WriteString("\n\n")

	sb.WriteString("Provide a comprehensive review following the specified format. " +
		"Focus on identifying specific issues with part/volume/chapter IDs and actionable improvements.")

	return sb.String()
}
