package dsl

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

type storylineContract struct {
	ID             string
	Name           string
	Type           string
	Importance     int
	Scope          string
	PayoffStyle    string
	SetupRole      string
	Desire         string
	Opposition     string
	Stakes         string
	Turn           string
	Payoff         string
	OpenQuestion   string
	PressurePoints string
}

type storylineMovement struct {
	ChapterID string
	Index     int
	Target    string
	Stage     string
	Cost      string
	Note      string
}

// checkStoryContractQuality treats setup storylines as soft story contracts and
// outline chapters as execution traces. It only emits suggestions/warnings; it
// does not judge prose quality or force a fixed chapter formula.
func (s *Simulator) checkStoryContractQuality() {
	if s == nil || s.DSL == nil {
		return
	}

	contracts := s.storylineContracts()
	if len(contracts) == 0 {
		if s.DSL.Metadata != nil && s.DSL.Metadata.Phase == string(PhaseSetup) {
			s.addIssue(IssueMissingInfo, SeverityInfo, "", 0,
				"setup DSL has no storyline contracts",
				"Add setup.storylines with desire, opposition, stakes, open_question, or payoff so simulate can pressure-test story drive.")
		}
		return
	}

	s.checkSetupStorylineContracts(contracts)
	if s.DSL.Metadata != nil && s.DSL.Metadata.Phase == string(PhaseSetup) {
		return
	}
	if s.DSL.Storyline == nil || len(s.DSL.Storyline.Chapters) == 0 {
		return
	}

	movements := s.collectStorylineMovements()
	s.checkOutlineStorylineCoverage(contracts, movements)
	s.checkChapterDurableChanges()
}

func (s *Simulator) storylineContracts() []storylineContract {
	var contracts []storylineContract
	for _, rule := range s.setupStructuredRules("storyline.contract") {
		fields := parseSetupRuleEffect(rule.Effect)
		name := firstNonEmpty(fields["name"], fields["id"], rule.Name)
		contracts = append(contracts, storylineContract{
			ID:             fields["id"],
			Name:           name,
			Type:           fields["type"],
			Importance:     atoiSafe(fields["importance"]),
			Scope:          fields["scope"],
			PayoffStyle:    fields["payoff_style"],
			SetupRole:      fields["setup_role"],
			Desire:         fields["desire"],
			Opposition:     fields["opposition"],
			Stakes:         fields["stakes"],
			Turn:           fields["turn"],
			Payoff:         fields["payoff"],
			OpenQuestion:   fields["open_question"],
			PressurePoints: fields["pressure_points"],
		})
	}
	return contracts
}

func (s *Simulator) checkSetupStorylineContracts(contracts []storylineContract) {
	for _, contract := range contracts {
		texture := nonEmptyCount(
			contract.Desire,
			contract.Opposition,
			contract.Stakes,
			contract.Turn,
			contract.Payoff,
			contract.OpenQuestion,
			contract.PressurePoints,
			contract.Scope,
			contract.PayoffStyle,
			contract.SetupRole,
		)
		if texture >= 3 {
			continue
		}
		s.addIssueWithEvidence(IssueConflict, SeverityInfo, "", 0,
			fmt.Sprintf("storyline '%s' is under-specified as a story contract", contract.Name),
			"Give this storyline 2-4 soft pressure hints, such as desire, opposition, stakes, open_question, or payoff. Keep them suggestive rather than templated.",
			[]IssueEvidence{chapterEvidence("", "storyline_contract", fmt.Sprintf("name=%s texture=%d", contract.Name, texture))})
	}
}

func (s *Simulator) collectStorylineMovements() []storylineMovement {
	if s.DSL == nil || s.DSL.Storyline == nil {
		return nil
	}
	var movements []storylineMovement
	for idx, chapter := range s.DSL.Storyline.Chapters {
		for _, obj := range chapter.Objectives {
			for _, step := range obj.Steps {
				for _, delta := range step.Event.StateDeltas {
					if strings.ToLower(strings.TrimSpace(delta.Kind)) != "storyline" {
						continue
					}
					stage := strings.ToLower(strings.TrimSpace(firstNonEmpty(delta.To, delta.Field)))
					movements = append(movements, storylineMovement{
						ChapterID: chapter.ID,
						Index:     idx,
						Target:    delta.Target,
						Stage:     stage,
						Cost:      delta.Cost,
						Note:      delta.Note,
					})

					if isStorylineProgressStage(stage) && strings.TrimSpace(delta.Cost) == "" && strings.TrimSpace(delta.Unit) == "" {
						s.addIssueWithEvidence(IssueConflict, SeverityInfo, chapter.ID, step.Order,
							fmt.Sprintf("storyline movement for '%s' has no visible cost, pressure, or consequence", delta.Target),
							"Add a soft consequence or pressure note when the chapter changes an arc, so outline improve knows what got harder afterward.",
							[]IssueEvidence{stateDeltaEvidence(chapter.ID, delta)})
					}
				}
			}
		}
	}
	return movements
}

func (s *Simulator) checkOutlineStorylineCoverage(contracts []storylineContract, movements []storylineMovement) {
	byTarget := map[string][]storylineMovement{}
	for _, movement := range movements {
		key := normalizeStorylineKey(movement.Target)
		if key == "" {
			continue
		}
		byTarget[key] = append(byTarget[key], movement)
	}

	chapterCount := len(s.DSL.Storyline.Chapters)
	maxGap := int(math.Max(4, math.Ceil(float64(chapterCount)/3)))
	for _, contract := range contracts {
		keys := []string{normalizeStorylineKey(contract.Name), normalizeStorylineKey(contract.ID)}
		var matched []storylineMovement
		for _, key := range keys {
			if key != "" && len(byTarget[key]) > len(matched) {
				matched = byTarget[key]
			}
		}
		if len(matched) == 0 {
			priority := SeverityInfo
			if contract.Importance >= 8 || strings.Contains(strings.ToLower(contract.Type), "main") {
				priority = SeverityWarning
			}
			s.addIssueWithEvidence(IssuePlotHole, priority, "", 0,
				fmt.Sprintf("setup storyline '%s' is not advanced by the outline DSL", contract.Name),
				"Add sparse storyline_advances to the key chapters that create pressure, reveal, reversal, or payoff for this setup storyline.",
				[]IssueEvidence{chapterEvidence("", "storyline_contract", contract.Name)})
			continue
		}

		if contract.Payoff != "" && !hasStage(matched, "payoff", "resolved", "completed", "completion") && requiresImmediatePayoff(contract) {
			s.addIssue(IssuePlotHole, SeverityInfo, "", 0,
				fmt.Sprintf("storyline '%s' promises a payoff but outline DSL has no payoff/resolved movement", contract.Name),
				"Either plan a payoff chapter with storyline_advances.stage=payoff, or mark the setup storyline as staged/long-scope if it should only be seeded here.")
		}
		if contract.Payoff != "" && !hasStage(matched, "hook", "pressure", "reveal", "reversal", "twist", "progress", "payoff", "resolved", "completed", "completion") {
			s.addIssue(IssuePlotHole, SeverityInfo, "", 0,
				fmt.Sprintf("storyline '%s' promises a payoff but outline DSL has no staged movement", contract.Name),
				"Add a hook, pressure, reveal, or progress movement now; save the final payoff for the proper scale.")
		}
		if contract.Turn != "" && hasStage(matched, "reversal", "twist") && !hasEarlierStageBefore(matched, []string{"reversal", "twist"}, "pressure", "reveal", "hook") {
			s.addIssue(IssuePlotHole, SeverityInfo, "", 0,
				fmt.Sprintf("storyline '%s' has a reversal without earlier pressure/reveal setup", contract.Name),
				"Plant a pressure or reveal movement before the reversal so the turn feels earned without becoming predictable.")
		}
		if maxGap > 0 {
			prev := matched[0].Index
			for _, movement := range matched[1:] {
				if movement.Index-prev > maxGap {
					s.addIssue(IssuePacing, SeverityInfo, movement.ChapterID, 0,
						fmt.Sprintf("storyline '%s' goes %d chapters without visible movement", contract.Name, movement.Index-prev),
						"Add a small pressure, clue, consequence, or choice in the gap, or intentionally mark the arc as dormant in outline notes.")
				}
				prev = movement.Index
			}
		}
	}
}

func requiresImmediatePayoff(contract storylineContract) bool {
	scope := strings.ToLower(strings.TrimSpace(contract.Scope))
	style := strings.ToLower(strings.TrimSpace(contract.PayoffStyle))
	if style == "immediate" || style == "direct" || style == "same_volume" || style == "current_volume" {
		return true
	}
	if style == "staged_reveal" || style == "slow_burn" || style == "final_turn" || style == "series_payoff" {
		return false
	}
	switch scope {
	case "current", "current_volume", "this_volume", "volume", "opening":
		return true
	case "book", "series", "multi_volume", "long_arc", "long":
		return false
	default:
		return false
	}
}

func (s *Simulator) checkChapterDurableChanges() {
	for _, chapter := range s.DSL.Storyline.Chapters {
		eventTypes := map[string]bool{}
		hasDurableChange := false
		for _, obj := range chapter.Objectives {
			for _, step := range obj.Steps {
				eventType := strings.ToLower(strings.TrimSpace(step.Event.Type))
				if eventType != "" {
					eventTypes[eventType] = true
				}
				for _, delta := range step.Event.StateDeltas {
					kind := strings.ToLower(strings.TrimSpace(delta.Kind))
					switch kind {
					case "storyline", "plot_thread", "resource", "cultivation", "relationship", "goal", "status", "premise", "item", "injury":
						hasDurableChange = true
					}
				}
			}
		}
		if hasDurableChange {
			continue
		}
		if eventTypes["combat"] || eventTypes["move"] || len(eventTypes) >= 2 {
			s.addIssue(IssuePlotHole, SeverityInfo, chapter.ID, 0,
				"chapter has activity but no durable story state change in DSL",
				"Make the outline record what changed after the activity: a goal shift, resource cost, relationship change, clue, injury, story pressure, or payoff.")
		}
	}
}

func normalizeStorylineKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return value
	}
	return b.String()
}

func isStorylineProgressStage(stage string) bool {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "hook", "pressure", "reveal", "reversal", "twist", "progress", "payoff", "resolved", "completed", "completion":
		return true
	default:
		return stage != ""
	}
}

func hasStage(movements []storylineMovement, stages ...string) bool {
	want := map[string]bool{}
	for _, stage := range stages {
		want[strings.ToLower(stage)] = true
	}
	for _, movement := range movements {
		if want[strings.ToLower(movement.Stage)] {
			return true
		}
	}
	return false
}

func hasEarlierStageBefore(movements []storylineMovement, beforeStages []string, earlierStages ...string) bool {
	wantEarlier := map[string]bool{}
	for _, stage := range earlierStages {
		wantEarlier[strings.ToLower(stage)] = true
	}
	wantBefore := map[string]bool{}
	for _, stage := range beforeStages {
		wantBefore[strings.ToLower(stage)] = true
	}
	seenEarlier := false
	for _, movement := range movements {
		stage := strings.ToLower(movement.Stage)
		if wantEarlier[stage] {
			seenEarlier = true
		}
		if wantBefore[stage] {
			return seenEarlier
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonEmptyCount(values ...string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func atoiSafe(value string) int {
	n := 0
	fmt.Sscanf(strings.TrimSpace(value), "%d", &n)
	return n
}
