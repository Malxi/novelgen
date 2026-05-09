package agents

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"novelgen/internal/models"
)

const (
	revisionHistoryLimit    = 6
	revisionSuggestionLimit = 8
	revisionStrengthLimit   = 4
)

// RevisionSession is a compact, structured memory for review/improve loops.
// It is prompt context only: the current typed artifact remains the source of truth.
type RevisionSession struct {
	SessionID string
	Stage     string
	Goal      string
	History   []RevisionTurn
}

type RevisionTurn struct {
	Round        int
	Phase        string
	Summary      string
	Score        float64
	Strengths    []string
	Suggestions  []RevisionSuggestionBrief
	UserGuidance string
}

type RevisionSuggestionBrief struct {
	Priority   string
	Category   string
	Target     string
	Issue      string
	Suggestion string
}

func NewRevisionSession(stage, goal string) *RevisionSession {
	return &RevisionSession{
		SessionID: fmt.Sprintf("%s-%d", strings.TrimSpace(stage), time.Now().UnixNano()),
		Stage:     strings.TrimSpace(stage),
		Goal:      strings.TrimSpace(goal),
	}
}

func (s *RevisionSession) AddUserGuidance(round int, guidance string) {
	if s == nil || strings.TrimSpace(guidance) == "" {
		return
	}
	s.History = append(s.History, RevisionTurn{
		Round:        round,
		Phase:        "user_guidance",
		UserGuidance: clipForPrompt(guidance, 600),
	})
	s.trimHistory()
}

func (s *RevisionSession) AddReview(round int, review models.ReviewResult) {
	if s == nil {
		return
	}
	s.History = append(s.History, RevisionTurn{
		Round:       round,
		Phase:       "review",
		Summary:     clipForPrompt(review.Summary, 700),
		Score:       review.OverallScore,
		Strengths:   compactStrengths(review.Strengths),
		Suggestions: compactSuggestions(review.Suggestions),
	})
	s.trimHistory()
}

func (s *RevisionSession) AddImprove(round int, note string) {
	if s == nil {
		return
	}
	s.History = append(s.History, RevisionTurn{
		Round:   round,
		Phase:   "improve",
		Summary: clipForPrompt(note, 500),
	})
	s.trimHistory()
}

func (s *RevisionSession) Prompt() string {
	if s == nil || (s.SessionID == "" && s.Stage == "" && s.Goal == "" && len(s.History) == 0) {
		return ""
	}

	var b strings.Builder
	b.WriteString("Revision Session\n")
	if s.SessionID != "" {
		b.WriteString(fmt.Sprintf("- session_id: %s\n", s.SessionID))
	}
	if s.Stage != "" {
		b.WriteString(fmt.Sprintf("- stage: %s\n", s.Stage))
	}
	if s.Goal != "" {
		b.WriteString(fmt.Sprintf("- goal: %s\n", clipForPrompt(s.Goal, 600)))
	}
	if len(s.History) == 0 {
		return strings.TrimSpace(b.String())
	}

	b.WriteString("- history:\n")
	for _, turn := range s.History {
		b.WriteString(fmt.Sprintf("  - round %d %s", turn.Round, turn.Phase))
		if turn.Score > 0 {
			b.WriteString(fmt.Sprintf(" score %.1f/100", turn.Score))
		}
		b.WriteString("\n")
		if turn.UserGuidance != "" {
			b.WriteString(fmt.Sprintf("    user_guidance: %s\n", turn.UserGuidance))
		}
		if turn.Summary != "" {
			b.WriteString(fmt.Sprintf("    summary: %s\n", turn.Summary))
		}
		if len(turn.Strengths) > 0 {
			b.WriteString(fmt.Sprintf("    preserve: %s\n", strings.Join(turn.Strengths, "; ")))
		}
		for _, suggestion := range turn.Suggestions {
			target := suggestion.Target
			if target == "" {
				target = "global"
			}
			b.WriteString(fmt.Sprintf("    suggestion[%s/%s/%s]: %s -> %s\n",
				nonEmpty(suggestion.Priority, "unknown"),
				nonEmpty(suggestion.Category, "general"),
				target,
				suggestion.Issue,
				suggestion.Suggestion))
		}
	}
	b.WriteString("Use this session as continuity for the revision: preserve accepted strengths, avoid cycling back to already-fixed issues, and move the current artifact forward.")

	return strings.TrimSpace(b.String())
}

func (s *RevisionSession) trimHistory() {
	if s == nil || len(s.History) <= revisionHistoryLimit {
		return
	}
	s.History = append([]RevisionTurn(nil), s.History[len(s.History)-revisionHistoryLimit:]...)
}

func compactStrengths(strengths []string) []string {
	var compact []string
	for _, strength := range strengths {
		strength = clipForPrompt(strength, 220)
		if strength == "" {
			continue
		}
		compact = append(compact, strength)
		if len(compact) >= revisionStrengthLimit {
			break
		}
	}
	return compact
}

func compactSuggestions(suggestions []models.ReviewSuggestion) []RevisionSuggestionBrief {
	if len(suggestions) == 0 {
		return nil
	}

	ordered := append([]models.ReviewSuggestion(nil), suggestions...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return priorityRank(ordered[i].Priority) < priorityRank(ordered[j].Priority)
	})

	compact := make([]RevisionSuggestionBrief, 0, min(len(ordered), revisionSuggestionLimit))
	for _, suggestion := range ordered {
		target := strings.TrimSpace(suggestion.TargetID)
		if target == "" {
			target = strings.TrimSpace(suggestion.TargetName)
		}
		brief := RevisionSuggestionBrief{
			Priority:   strings.TrimSpace(suggestion.Priority),
			Category:   strings.TrimSpace(suggestion.Category),
			Target:     clipForPrompt(target, 100),
			Issue:      clipForPrompt(suggestion.Issue, 260),
			Suggestion: clipForPrompt(suggestion.Suggestion, 320),
		}
		if brief.Issue == "" && brief.Suggestion == "" {
			continue
		}
		compact = append(compact, brief)
		if len(compact) >= revisionSuggestionLimit {
			break
		}
	}
	return compact
}

func priorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case models.PriorityCritical, "fatal", "blocker", "serious":
		return 0
	case models.PriorityHigh:
		return 1
	case models.PriorityMedium:
		return 2
	case models.PriorityLow:
		return 3
	default:
		return 4
	}
}
