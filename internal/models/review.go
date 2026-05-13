package models

import "strings"

// ReviewSuggestion represents a single improvement suggestion
// This is the universal suggestion structure used across all review types
type ReviewSuggestion struct {
	Category   string `json:"category"`    // Category of the issue (e.g., "logic", "appeal", "consistency")
	TargetID   string `json:"target_id"`   // ID of the target element (e.g., "P1", "P1-V1-C1")
	TargetName string `json:"target_name"` // Name/title of the target element
	Issue      string `json:"issue"`       // Description of the problem
	Suggestion string `json:"suggestion"`  // Specific improvement suggestion
	Priority   string `json:"priority"`    // "high", "medium", "low"
}

// DimensionScore represents a score for a specific review dimension
type DimensionScore struct {
	Name  string  `json:"name"`  // Dimension name (e.g., "Logic", "Appeal")
	Score float64 `json:"score"` // Score value
	Max   float64 `json:"max"`   // Maximum possible score
}

// ContinuityIssue represents a continuity or abrupt plot issue found during review
type ContinuityIssue struct {
	Type        string `json:"type"`        // "abrupt_plot", "timeline", "space", "character_state", "relationship"
	Location    string `json:"location"`    // Specific location in text (paragraph/sentence)
	Description string `json:"description"` // Description of the issue
	Reason      string `json:"reason"`      // Why it's an issue (lack of setup, logic jump, etc.)
	Suggestion  string `json:"suggestion"`  // How to fix it
	Severity    string `json:"severity"`    // "fatal", "serious", "minor"
}

// ReviewResult represents a universal review result structure
// It can be used for setup review, outline review, chapter review, etc.
type ReviewResult struct {
	OverallScore     float64            `json:"overall_score"`     // 0-100
	Dimensions       []DimensionScore   `json:"dimensions"`        // Detailed scores by dimension
	Summary          string             `json:"summary"`           // Overall assessment
	Strengths        []string           `json:"strengths"`         // What's working well
	Weaknesses       []string           `json:"weaknesses"`        // Areas needing improvement (optional)
	Suggestions      []ReviewSuggestion `json:"suggestions"`       // Specific improvement suggestions
	Iteration        int                `json:"iteration"`         // Iteration number (for tracking)
	ContinuityIssues []ContinuityIssue  `json:"continuity_issues"` // Abrupt plot and continuity issues
}

// GetDimensionScore gets the score for a specific dimension by name
func (r *ReviewResult) GetDimensionScore(name string) (float64, bool) {
	for _, d := range r.Dimensions {
		if d.Name == name {
			return d.Score, true
		}
	}
	return 0, false
}

// GetHighPrioritySuggestions returns only high priority suggestions
func (r *ReviewResult) GetHighPrioritySuggestions() []ReviewSuggestion {
	var highPriority []ReviewSuggestion
	for _, s := range r.Suggestions {
		if normalizeReviewSeverity(s.Priority) == PriorityHigh {
			highPriority = append(highPriority, s)
		}
	}
	return highPriority
}

// HasBlockingSuggestions reports whether review feedback should force an
// improve pass even when the numeric score already meets the threshold.
func (r *ReviewResult) HasBlockingSuggestions() bool {
	if r == nil {
		return false
	}
	for _, s := range r.Suggestions {
		switch normalizeReviewSeverity(s.Priority) {
		case PriorityCritical, PriorityHigh:
			return true
		}
	}
	for _, issue := range r.ContinuityIssues {
		switch normalizeReviewSeverity(issue.Severity) {
		case PriorityCritical, "fatal", "serious", "blocker":
			return true
		}
	}
	return false
}

// GetSuggestionsByCategory returns suggestions filtered by category
func (r *ReviewResult) GetSuggestionsByCategory(category string) []ReviewSuggestion {
	var filtered []ReviewSuggestion
	for _, s := range r.Suggestions {
		if s.Category == category {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// IsPassing checks if the review meets a quality threshold
func (r *ReviewResult) IsPassing(threshold float64) bool {
	return r.OverallScore >= threshold
}

// NormalizeScoreScale makes AI review scores comparable as 0-100 percentages.
// Some models return overall_score on a 0-10 scale while also returning
// dimensions such as 9/10. When dimension maxima are available, trust their
// ratio because it preserves non-uniform dimension scores.
func (r *ReviewResult) NormalizeScoreScale() {
	if r == nil {
		return
	}
	if len(r.Dimensions) > 0 {
		var totalScore, totalMax float64
		for _, d := range r.Dimensions {
			if d.Max > 0 {
				totalScore += d.Score
				totalMax += d.Max
			}
		}
		if totalMax > 0 && r.OverallScore >= 0 && r.OverallScore <= 10 {
			r.OverallScore = (totalScore / totalMax) * 100
			return
		}
	}
	if r.OverallScore > 0 && r.OverallScore <= 10 {
		r.OverallScore *= 10
	}
}

// CalculateOverallScore recalculates the overall score based on dimensions
// This is useful when dimensions are modified
func (r *ReviewResult) CalculateOverallScore() float64 {
	if len(r.Dimensions) == 0 {
		return r.OverallScore
	}

	var totalScore, totalMax float64
	for _, d := range r.Dimensions {
		totalScore += d.Score
		totalMax += d.Max
	}

	if totalMax == 0 {
		return 0
	}

	r.OverallScore = (totalScore / totalMax) * 100
	return r.OverallScore
}

// Common dimension names for different review types
const (
	// Setup review dimensions
	DimRootSetting  = "Root Setting"
	DimLogicClosure = "Logic Closure"
	DimAdaptation   = "Adaptation"
	DimAppeal       = "Appeal"
	DimScalability  = "Scalability"

	// Outline review dimensions
	DimLogic      = "Logic"
	DimEngagement = "Engagement"
	DimPacing     = "Pacing"
	DimCoherence  = "Coherence"

	// Craft review dimensions
	DimConsistency = "Consistency"
	DimDepth       = "Depth"
	DimOriginality = "Originality"

	// Priority levels
	PriorityCritical = "critical"
	PriorityHigh     = "high"
	PriorityMedium   = "medium"
	PriorityLow      = "low"

	// Suggestion categories
	CategoryLogic       = "logic"
	CategoryAppeal      = "appeal"
	CategoryConsistency = "consistency"
	CategoryStructure   = "structure"
	CategoryCharacter   = "character"
	CategoryPlot        = "plot"
)

func normalizeReviewSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "致命", "严重", "阻塞", "阻断", "最高":
		return PriorityCritical
	case "高", "高优先级":
		return PriorityHigh
	case "中", "中等", "中优先级":
		return PriorityMedium
	case "低", "低优先级":
		return PriorityLow
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}
