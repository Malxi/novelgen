package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/models"
)

// ReviewSuggestion represents a single improvement suggestion
// Deprecated: Use models.ReviewSuggestion instead
type ReviewSuggestion struct {
	Type       string `json:"type"`       // "part", "volume", "chapter"
	ID         string `json:"id"`         // e.g., "1", "1_1", "1_1_1"
	Title      string `json:"title"`      // Current title
	Issue      string `json:"issue"`      // Description of the problem
	Suggestion string `json:"suggestion"` // Specific improvement suggestion
	Priority   string `json:"priority"`   // "high", "medium", "low"
}

// ToModelsSuggestion converts prompts.ReviewSuggestion to models.ReviewSuggestion
func (r *ReviewSuggestion) ToModelsSuggestion() models.ReviewSuggestion {
	return models.ReviewSuggestion{
		Category:   r.Type,
		TargetID:   r.ID,
		TargetName: r.Title,
		Issue:      r.Issue,
		Suggestion: r.Suggestion,
		Priority:   r.Priority,
	}
}

// ReviewResult represents the complete review output
// Deprecated: Use models.ReviewResult instead. This is kept for backward compatibility.
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

// ToModelsResult converts prompts.ReviewResult to models.ReviewResult
func (r *ReviewResult) ToModelsResult() *models.ReviewResult {
	dimensions := []models.DimensionScore{
		{Name: models.DimLogic, Score: r.LogicScore, Max: 100},
		{Name: models.DimEngagement, Score: r.EngagementScore, Max: 100},
		{Name: models.DimPacing, Score: r.PacingScore, Max: 100},
		{Name: models.DimCoherence, Score: r.CoherenceScore, Max: 100},
	}

	suggestions := make([]models.ReviewSuggestion, len(r.Suggestions))
	for i, s := range r.Suggestions {
		suggestions[i] = s.ToModelsSuggestion()
	}

	return &models.ReviewResult{
		OverallScore: r.OverallScore,
		Dimensions:   dimensions,
		Summary:      r.Summary,
		Strengths:    r.Strengths,
		Weaknesses:   r.Weaknesses,
		Suggestions:  suggestions,
	}
}

// FromModelsResult creates a prompts.ReviewResult from models.ReviewResult
// This is useful when working with the new unified model but need backward compatibility
func FromModelsResult(r *models.ReviewResult) *ReviewResult {
	result := &ReviewResult{
		OverallScore: r.OverallScore,
		Summary:      r.Summary,
		Strengths:    r.Strengths,
		Weaknesses:   r.Weaknesses,
	}

	// Extract dimension scores
	for _, dim := range r.Dimensions {
		switch dim.Name {
		case models.DimLogic:
			result.LogicScore = dim.Score
		case models.DimEngagement:
			result.EngagementScore = dim.Score
		case models.DimPacing:
			result.PacingScore = dim.Score
		case models.DimCoherence:
			result.CoherenceScore = dim.Score
		}
	}

	// Convert suggestions
	result.Suggestions = make([]ReviewSuggestion, len(r.Suggestions))
	for i, s := range r.Suggestions {
		result.Suggestions[i] = ReviewSuggestion{
			Type:       s.Category,
			ID:         s.TargetID,
			Title:      s.TargetName,
			Issue:      s.Issue,
			Suggestion: s.Suggestion,
			Priority:   s.Priority,
		}
	}

	return result
}

// registerOutlineReviewPrompts registers all outline review prompts
func registerOutlineReviewPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillOutlineReview,
		Name:         "default",
		Description:  "Review outline for logical consistency, engagement, and pacing",
		SystemPrompt: outlineReviewSystemPrompt,
		OutputFormat: FormatJSON,
		OutputModel:  ReviewResult{}, // Auto-generate schema from struct
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
7. **CONTINUITY & COHERENCE** (CRITICAL):
   - Chapter-to-chapter causality: Does each chapter logically follow from the previous one?
   - Event tracking: Are events from previous chapters reflected in subsequent chapters?
   - Character state persistence: Do characters maintain consistent emotional states and relationships?
   - Plot thread management: Do storylines progress naturally or do they appear/disappear arbitrarily?
   - Setup and payoff: Are elements introduced early paid off later? Are payoffs properly set up?
   - Beat anchors: Does opening_beat match beats[0] and closing_beat match beats[last]?
   - Handoff: Does chapter N closing_beat directly lead into chapter N+1 opening_beat?
   - State change alignment: Does state_change map to a concrete Events entry?

Continuity Check Process:
- For each chapter N, ask: "What specific events from chapter N-1 directly influence this chapter?"
- Check if character reactions and decisions are consistent with their previous experiences
- Verify that items, relationships, and goals mentioned in earlier chapters are tracked
- Identify any jarring transitions where the story seems to "jump" without proper connective tissue

Scoring Guide (0-100):
- 90-100: Exceptional, publish-ready quality
- 80-89: Strong, minor improvements needed
- 70-79: Good, several areas need work
- 60-69: Fair, significant revision recommended
- Below 60: Poor, major restructuring needed

Provide SPECIFIC, ACTIONABLE suggestions. Each suggestion must identify:
- Exact location (part/volume/chapter ID in the EXACT format used in the outline)
- Specific issue
- Concrete improvement recommendation
- Priority level (high/medium/low)

ID Format Reference (use EXACTLY as shown in the outline):
- Part: "P1", "P2", etc.
- Volume: "P1-V1", "P1-V2", etc.
- Chapter: "P1-V1-C1", "P1-V1-C2", etc.

IMPORTANT: Always use the full chapter ID (e.g., "P1-V1-C19") when referring to chapters. Do NOT abbreviate as "C19" or use incorrect part numbers like "P2" when you mean "P1-V1-C2".

HIGH PRIORITY issues should include:
- Major plot holes or contradictions
- Broken continuity between chapters
- Missing or inconsistent opening_beat/closing_beat/state_change
- Beats that do not align with anchors (beats[0]/beats[last])
- state_change not represented in Events
- Character behavior that contradicts established traits
- Storylines that start but never continue

For continuity issues, the suggestion MUST reference the specific anchor(s) involved (e.g., previous closing_beat, next opening_beat, or the exact Events entry that should mirror state_change).

Respond ONLY with a valid JSON object.`

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
	setup, _ := data["setup"].(string)
	iteration, _ := data["iteration"].(int)

	var prompt strings.Builder
	prompt.WriteString("Please review the following story outline:\n\n")
	prompt.WriteString("=== STORY SETUP ===\n")
	prompt.WriteString(setup)
	prompt.WriteString("\n\n=== OUTLINE ===\n")
	prompt.WriteString(outline)
	prompt.WriteString("\n\n=== ITERATION INFO ===\n")
	prompt.WriteString(fmt.Sprintf("This is iteration %d.\n", iteration))
	prompt.WriteString("\nPlease provide a detailed review following the criteria in your instructions.")

	return prompt.String()
}
