package cmd

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/rpg/dsl"
)

type qualityGateResult struct {
	Suggestions []models.ReviewSuggestion
	Blocking    bool
}

func runSetupQualityGate(setup *models.StorySetup) qualityGateResult {
	var result qualityGateResult
	models.NormalizeStorySetup(setup)
	result.add(validateStorySetupDirect(setup)...)

	adapter := dsl.NewModelAdapter(setup, nil, nil, nil, nil)
	if issues, err := adapter.Simulate(dsl.PhaseSetup); err == nil {
		result.add(dsl.NewSimulationBridge().ConvertIssuesToSuggestions(issues)...)
	}

	result.dedup()
	result.Blocking = hasBlockingSuggestions(result.Suggestions)
	return result
}

func runOutlineQualityGate(setup *models.StorySetup, outline *models.Outline) qualityGateResult {
	var result qualityGateResult
	models.NormalizeStorySetup(setup)
	models.NormalizeOutline(outline)
	result.add(validateOutlineDirect(setup, outline)...)
	result.add(runOutlineValidatorOnModel(outline)...)

	adapter := dsl.NewModelAdapter(setup, outline, nil, nil, nil)
	if issues, err := adapter.Simulate(dsl.PhaseOutline); err == nil {
		result.add(dsl.NewSimulationBridge().ConvertIssuesToSuggestions(issues)...)
	}

	result.dedup()
	result.Blocking = hasBlockingSuggestions(result.Suggestions)
	return result
}

func (r *qualityGateResult) add(suggestions ...models.ReviewSuggestion) {
	r.Suggestions = append(r.Suggestions, suggestions...)
}

func (r *qualityGateResult) dedup() {
	seen := make(map[string]bool)
	var deduped []models.ReviewSuggestion
	for _, s := range r.Suggestions {
		key := strings.Join([]string{
			strings.TrimSpace(s.Category),
			strings.TrimSpace(s.TargetID),
			strings.TrimSpace(s.Issue),
			strings.TrimSpace(s.Suggestion),
		}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, s)
	}
	r.Suggestions = deduped
}

func hasBlockingSuggestions(suggestions []models.ReviewSuggestion) bool {
	for _, s := range suggestions {
		switch strings.ToLower(strings.TrimSpace(s.Priority)) {
		case models.PriorityCritical, models.PriorityHigh:
			return true
		}
	}
	return false
}

func qualityGateReviewResult(summary string, gate qualityGateResult) *models.ReviewResult {
	return &models.ReviewResult{
		OverallScore: scoreFromGate(gate),
		Summary:      summary,
		Suggestions:  gate.Suggestions,
	}
}

func scoreFromGate(gate qualityGateResult) float64 {
	score := 100.0
	for _, s := range gate.Suggestions {
		switch strings.ToLower(strings.TrimSpace(s.Priority)) {
		case models.PriorityCritical:
			score -= 8
		case models.PriorityHigh:
			score -= 5
		case models.PriorityMedium:
			score -= 2
		default:
			score -= 0.5
		}
	}
	if score < 0 {
		return 0
	}
	return score
}

func logQualityGateResult(name string, gate qualityGateResult) {
	if len(gate.Suggestions) == 0 {
		fmt.Printf("\nQuality gate (%s): passed\n", name)
		return
	}

	critical, high, medium, low := countSuggestionPriorities(gate.Suggestions)
	fmt.Printf("\nQuality gate (%s): %d issue(s) [critical=%d high=%d medium=%d low=%d]\n",
		name, len(gate.Suggestions), critical, high, medium, low)
	printQualityGateSuggestions(gate.Suggestions, 8)
	if gate.Blocking {
		fmt.Println("Quality gate: blocking issues detected; repair/improve should address them before downstream stages.")
	}
}

func printQualityGateSuggestions(suggestions []models.ReviewSuggestion, limit int) {
	for i, suggestion := range suggestions {
		if i >= limit {
			fmt.Printf("  ... %d more issue(s)\n", len(suggestions)-limit)
			return
		}
		target := coalesceString(suggestion.TargetName, suggestion.TargetID, "global")
		fmt.Printf("  - [%s/%s] %s: %s\n",
			coalesceString(suggestion.Priority, "low"),
			coalesceString(suggestion.Category, "general"),
			target,
			suggestion.Issue)
		if strings.TrimSpace(suggestion.Suggestion) != "" {
			fmt.Printf("    fix: %s\n", suggestion.Suggestion)
		}
	}
}

func countSuggestionPriorities(suggestions []models.ReviewSuggestion) (critical, high, medium, low int) {
	for _, s := range suggestions {
		switch strings.ToLower(strings.TrimSpace(s.Priority)) {
		case models.PriorityCritical:
			critical++
		case models.PriorityHigh:
			high++
		case models.PriorityMedium:
			medium++
		default:
			low++
		}
	}
	return
}

func validateStorySetupDirect(setup *models.StorySetup) []models.ReviewSuggestion {
	if setup == nil {
		return []models.ReviewSuggestion{qualitySuggestion("setup", "global", "setup", "story setup is nil", "Generate or import story setup before continuing.", models.PriorityCritical)}
	}

	var suggestions []models.ReviewSuggestion
	requiredText := map[string]string{
		"project_name":    setup.ProjectName,
		"premise":         setup.Premise,
		"theme":           setup.Theme,
		"target_audience": setup.TargetAudience,
		"tone":            setup.Tone,
		"tense":           setup.Tense,
		"pov_style":       setup.POVStyle,
	}
	for field, value := range requiredText {
		if strings.TrimSpace(value) == "" {
			suggestions = append(suggestions, qualitySuggestion("setup", field, field, "required setup field is empty", "Fill this field so later agents have a stable story contract.", models.PriorityHigh))
		}
	}
	if len(setup.Genres) == 0 {
		suggestions = append(suggestions, qualitySuggestion("setup", "genres", "genres", "setup has no genres", "Add 2-4 concrete genres/subgenres.", models.PriorityHigh))
	}
	if len(setup.Rules) == 0 {
		suggestions = append(suggestions, qualitySuggestion("setup", "rules", "rules", "setup has no world/story rules", "Add concrete rules, costs, limits, or exceptions.", models.PriorityHigh))
	}
	if len(setup.Storylines) == 0 {
		suggestions = append(suggestions, qualitySuggestion("structure", "storylines", "storylines", "setup has no storylines", "Add at least one main storyline with pressure and payoff hints.", models.PriorityHigh))
	}
	if storySetupWantsCoreCast(setup) && len(setup.CoreCast) == 0 {
		suggestions = append(suggestions, qualitySuggestion("character", "core_cast", "core_cast", "setup has no core cast seeds", "Add a small core cast with protagonist, major leads, rivals, mentors, and payoff roles so craft can expand them consistently.", models.PriorityMedium))
	}
	if storySetupWantsLongFormPlan(setup) && setup.LongFormPlan == nil {
		suggestions = append(suggestions, qualitySuggestion("structure", "long_form_plan", "long_form_plan", "long-form setup has no long_form_plan", "Add a lightweight long_form_plan with target scale, repeatable main loop, escalation ladder, reader promises, payoff cadence, and volume pattern.", models.PriorityMedium))
	}
	if setup.LongFormPlan != nil {
		suggestions = append(suggestions, validateLongFormPlan(setup.LongFormPlan)...)
	}
	suggestions = append(suggestions, validateSetupPromptBudget(setup)...)
	if storySetupWantsExpandedSystems(setup) && len(setup.Premises) < 3 {
		suggestions = append(suggestions, qualitySuggestion("setup", "premises", "premises", "setup has too few progression systems for long-form genre fiction", "Keep one root premise, but add 3-6 derived premise systems with progression ladders, such as protagonist growth, enemy tiers, faction technology, resource economy, and long-term threat systems.", models.PriorityMedium))
	}

	for i, storyline := range setup.Storylines {
		target := fmt.Sprintf("storylines[%d]", i)
		if strings.TrimSpace(storyline.Name) == "" {
			suggestions = append(suggestions, qualitySuggestion("structure", target, target, "storyline has no stable name", "Give the storyline a stable name so outline.storyline_advances can reference it.", models.PriorityHigh))
		}
		if storyline.Importance < 1 || storyline.Importance > 10 {
			suggestions = append(suggestions, qualitySuggestion("structure", target, storyline.Name, "storyline importance is outside 1-10", "Set importance to a value from 1 to 10.", models.PriorityMedium))
		}
		if storyline.Importance >= 8 && storylineTexture(storyline) < 3 {
			suggestions = append(suggestions, qualitySuggestion("plot", target, storyline.Name, "important storyline is under-specified", "Add 2-4 pressure hints such as desire, opposition, stakes, open_question, turn, or payoff.", models.PriorityMedium))
		}
		if storyline.Importance >= 8 && !hasStorylineArcContract(storyline) {
			suggestions = append(suggestions, qualitySuggestion("plot", target, storyline.Name, "important storyline lacks an arc contract", "Add high-level scope, setup_role, payoff_style, and 2-4 pressure_points so outline generation knows how long the promise runs and what kind of pressure to apply.", models.PriorityMedium))
		}
		if storyline.Importance >= 8 && storySetupWantsLongFormPlan(setup) {
			suggestions = append(suggestions, validateLongFormStorylineEngine(storyline, target)...)
		}
		if storyline.Importance >= 8 {
			if missing := missingAppealEngineFields(storyline.AppealEngine); len(missing) > 0 {
				issue := "important storyline lacks a complete appeal_engine"
				if storyline.AppealEngine != nil && appealEngineCompleteness(storyline.AppealEngine) > 0 {
					issue = "important storyline has a thin appeal_engine"
				}
				suggestions = append(suggestions, qualitySuggestion("appeal", target, storyline.Name, issue, fmt.Sprintf("Fill setup.storylines[].appeal_engine fields for a repeatable power-fantasy promise: %s.", strings.Join(missing, ", ")), models.PriorityMedium))
			}
		}
	}

	for i, premise := range setup.Premises {
		target := fmt.Sprintf("premises[%d]", i)
		if strings.TrimSpace(premise.Name) == "" {
			suggestions = append(suggestions, qualitySuggestion("setup", target, target, "premise has no name", "Give the premise a stable name.", models.PriorityHigh))
		}
		if len(premise.Progression) == 0 {
			suggestions = append(suggestions, qualitySuggestion("logic", target, premise.Name, "premise has no progression ladder", "Add progression stages with levels, boundaries, and requirements.", models.PriorityMedium))
			continue
		}
		if storySetupWantsExpandedSystems(setup) {
			if missing := missingAppealEngineFields(premise.AppealEngine); len(missing) > 0 {
				issue := "core premise lacks appeal_engine"
				if premise.AppealEngine != nil && appealEngineCompleteness(premise.AppealEngine) > 0 {
					issue = "core premise has a thin appeal_engine"
				}
				suggestions = append(suggestions, qualitySuggestion("appeal", target, premise.Name, issue, fmt.Sprintf("Clarify how this setting system creates fun wins by filling: %s.", strings.Join(missing, ", ")), models.PriorityMedium))
			}
		}
		previousLevel := -1
		for j, stage := range premise.Progression {
			stageTarget := fmt.Sprintf("%s.progression[%d]", target, j)
			if stage.Level <= 0 {
				suggestions = append(suggestions, qualitySuggestion("logic", stageTarget, premise.Name, "progression stage level must be positive", "Use positive sequential levels.", models.PriorityMedium))
			}
			if previousLevel > 0 && stage.Level <= previousLevel {
				suggestions = append(suggestions, qualitySuggestion("logic", stageTarget, premise.Name, "progression levels are not increasing", "Order progression stages by increasing level.", models.PriorityMedium))
			}
			if strings.TrimSpace(stage.Name) == "" || strings.TrimSpace(stage.Description) == "" {
				suggestions = append(suggestions, qualitySuggestion("logic", stageTarget, premise.Name, "progression stage lacks name or description", "Fill both name and description so growth can be checked.", models.PriorityMedium))
			}
			previousLevel = stage.Level
		}
	}

	for i, seed := range setup.CoreCast {
		target := fmt.Sprintf("core_cast[%d]", i)
		if strings.TrimSpace(seed.Name) == "" {
			suggestions = append(suggestions, qualitySuggestion("character", target, target, "core cast seed has no name", "Give the seed a stable name or placeholder so craft can expand it.", models.PriorityHigh))
		}
		if strings.TrimSpace(seed.Role) == "" {
			suggestions = append(suggestions, qualitySuggestion("character", target, seed.Name, "core cast seed has no role", "Assign a stable story role such as protagonist, lead, rival, mentor, or villain.", models.PriorityHigh))
		}
		if seed.Importance < 1 || seed.Importance > 10 {
			suggestions = append(suggestions, qualitySuggestion("character", target, seed.Name, "core cast seed importance is outside 1-10", "Set importance to a value from 1 to 10.", models.PriorityMedium))
		}
		if seed.Importance >= 8 && strings.TrimSpace(seed.StoryFunction) == "" {
			suggestions = append(suggestions, qualitySuggestion("character", target, seed.Name, "important core cast seed lacks story function", "Describe the character's story function so craft knows why this person matters.", models.PriorityMedium))
		}
		if seed.Importance >= 8 && strings.TrimSpace(seed.EntryPhase) == "" {
			suggestions = append(suggestions, qualitySuggestion("structure", target, seed.Name, "important core cast seed lacks entry phase", "Set entry_phase so the long-form story can control when this character enters the book.", models.PriorityMedium))
		}
		if seed.Importance >= 8 && strings.TrimSpace(seed.Payoff) == "" {
			suggestions = append(suggestions, qualitySuggestion("appeal", target, seed.Name, "important core cast seed lacks payoff", "Describe what payoff this character promises so later craft and write stages can cash it in.", models.PriorityMedium))
		}
	}
	if storySetupWantsCoreCast(setup) && len(setup.CoreCast) > 0 {
		suggestions = append(suggestions, validateCoreCastCapacity(setup)...)
	}

	seenResources := make(map[string]bool)
	for i, resource := range setup.WorldResources {
		target := fmt.Sprintf("world_resources[%d]", i)
		name := strings.TrimSpace(resource.Name)
		if name == "" || strings.TrimSpace(resource.Category) == "" || strings.TrimSpace(resource.Scarcity) == "" {
			suggestions = append(suggestions, qualitySuggestion("setup", target, name, "world resource lacks name, category, or scarcity", "Fill resource name, category, and scarcity so outline ledgers can reference it.", models.PriorityMedium))
		}
		key := strings.ToLower(name)
		if key != "" && seenResources[key] {
			suggestions = append(suggestions, qualitySuggestion("consistency", target, name, "duplicate world resource name", "Keep resource names unique and stable.", models.PriorityMedium))
		}
		seenResources[key] = true
	}

	return suggestions
}

func validateLongFormStorylineEngine(storyline models.Storyline, target string) []models.ReviewSuggestion {
	var suggestions []models.ReviewSuggestion
	if strings.TrimSpace(storyline.RepeatablePressure) == "" {
		suggestions = append(suggestions, qualitySuggestion("plot", target, storyline.Name, "important long-form storyline lacks repeatable pressure", "Describe how this storyline can repeatedly create pressure across many volumes without becoming a single-use premise.", models.PriorityMedium))
	}
	if strings.TrimSpace(storyline.PayoffCadence) == "" {
		suggestions = append(suggestions, qualitySuggestion("appeal", target, storyline.Name, "important long-form storyline lacks payoff cadence", "Define how often this storyline gives partial reveals, reversals, wins, or major payoffs.", models.PriorityMedium))
	}
	if strings.TrimSpace(storyline.Mutation) == "" {
		suggestions = append(suggestions, qualitySuggestion("structure", target, storyline.Name, "important long-form storyline lacks mutation", "Describe how this storyline changes after its initial pattern would become stale.", models.PriorityMedium))
	}
	if strings.TrimSpace(storyline.FailureMode) == "" {
		suggestions = append(suggestions, qualitySuggestion("structure", target, storyline.Name, "important long-form storyline lacks failure mode", "Name the most likely way this storyline could become repetitive or inconsistent so later improve/review can catch it.", models.PriorityLow))
	}
	return suggestions
}

func validateLongFormPlan(plan *models.LongFormPlan) []models.ReviewSuggestion {
	var suggestions []models.ReviewSuggestion
	if plan.TargetChapters < 0 {
		suggestions = append(suggestions, qualitySuggestion("structure", "long_form_plan.target_chapters", "long_form_plan", "long_form_plan target_chapters cannot be negative", "Use a positive target chapter count, or omit the field for short fiction.", models.PriorityMedium))
	}
	if plan.TargetVolumes < 0 {
		suggestions = append(suggestions, qualitySuggestion("structure", "long_form_plan.target_volumes", "long_form_plan", "long_form_plan target_volumes cannot be negative", "Use a positive target volume count, or omit the field if the project structure controls volume count.", models.PriorityMedium))
	}

	longScale := plan.TargetChapters >= 300 || plan.TargetVolumes >= 5
	if longScale && strings.TrimSpace(plan.MainLoop) == "" {
		suggestions = append(suggestions, qualitySuggestion("appeal", "long_form_plan.main_loop", "long_form_plan", "long-form plan lacks a repeatable main_loop", "Describe the recurring reader loop, such as pressure, clever exploit, visible win, reward, and bigger game.", models.PriorityMedium))
	}
	if longScale && len(plan.EscalationLadder) < 4 {
		suggestions = append(suggestions, qualitySuggestion("structure", "long_form_plan.escalation_ladder", "long_form_plan", "long-form plan has too few escalation ladder stages", "Add at least four high-level scope stages so outline generation can escalate without repeating the same arena.", models.PriorityMedium))
	}
	if longScale && len(plan.ReaderPromises) < 3 {
		suggestions = append(suggestions, qualitySuggestion("appeal", "long_form_plan.reader_promises", "long_form_plan", "long-form plan has too few reader promises", "Add 3-6 repeatable reader promises such as power growth, face-slapping reversals, faction rise, relationship progress, mystery reveals, or resource wins.", models.PriorityMedium))
	}
	if longScale && strings.TrimSpace(plan.PayoffCadence) == "" {
		suggestions = append(suggestions, qualitySuggestion("appeal", "long_form_plan.payoff_cadence", "long_form_plan", "long-form plan lacks payoff cadence", "Define how often small, medium, and major payoffs should land so long outlines do not become flat setup.", models.PriorityMedium))
	}
	if longScale && len(plan.VolumePattern) < 4 {
		suggestions = append(suggestions, qualitySuggestion("structure", "long_form_plan.volume_pattern", "long_form_plan", "long-form plan lacks a usable volume pattern", "Add reusable volume blueprint beats such as hook, pressure, misread, exploit, big win, visible reward, and next gate.", models.PriorityMedium))
	}
	if longScale && strings.TrimSpace(plan.MidpointMutation) == "" {
		suggestions = append(suggestions, qualitySuggestion("structure", "long_form_plan.midpoint_mutation", "long_form_plan", "long-form plan lacks midpoint mutation", "Describe how the serial changes after the initial loop would otherwise grow stale.", models.PriorityMedium))
	}
	return suggestions
}

func validateSetupPromptBudget(setup *models.StorySetup) []models.ReviewSuggestion {
	if setup == nil {
		return nil
	}

	var suggestions []models.ReviewSuggestion
	if len(setup.CoreCast) > 12 {
		suggestions = append(suggestions, qualitySuggestion("setup", "core_cast", "core_cast", "setup has too many core cast seeds", "Keep setup.core_cast to the central 6-12 seeds. Move minor characters and full biographies to craft.", models.PriorityMedium))
	}
	if len(setup.Storylines) > 12 {
		suggestions = append(suggestions, qualitySuggestion("setup", "storylines", "storylines", "setup has too many storylines", "Merge small threads into larger storyline contracts so compose can track them reliably.", models.PriorityMedium))
	}
	if len(setup.Premises) > 8 {
		suggestions = append(suggestions, qualitySuggestion("setup", "premises", "premises", "setup has too many premise systems", "Keep only major world/progression systems in setup. Put encyclopedic details into craft or later notes.", models.PriorityMedium))
	}
	if len(setup.Rules) > 15 {
		suggestions = append(suggestions, qualitySuggestion("setup", "rules", "rules", "setup has too many world/story rules", "Keep setup rules to the most generative constraints and exploitable surfaces.", models.PriorityMedium))
	}
	if len(setup.WorldResources) > 12 {
		suggestions = append(suggestions, qualitySuggestion("setup", "world_resources", "world_resources", "setup has too many world resources", "Track only resources that outline chapters must ledger; move flavor resources to craft.", models.PriorityMedium))
	}

	suggestions = append(suggestions, validateSetupTextBudget("setup", "premise", "premise", setup.Premise, 900)...)
	suggestions = append(suggestions, validateSetupTextBudget("setup", "theme", "theme", setup.Theme, 500)...)
	suggestions = append(suggestions, validateSetupTextBudget("setup", "tone", "tone", setup.Tone, 240)...)
	for i, rule := range setup.Rules {
		target := fmt.Sprintf("rules[%d]", i)
		suggestions = append(suggestions, validateSetupTextBudget("setup", target, target, rule, 300)...)
	}

	if setup.LongFormPlan != nil {
		plan := setup.LongFormPlan
		suggestions = append(suggestions, validateSetupTextBudget("setup", "long_form_plan.main_loop", "long_form_plan", plan.MainLoop, 320)...)
		suggestions = append(suggestions, validateSetupTextBudget("setup", "long_form_plan.payoff_cadence", "long_form_plan", plan.PayoffCadence, 320)...)
		suggestions = append(suggestions, validateSetupTextBudget("setup", "long_form_plan.midpoint_mutation", "long_form_plan", plan.MidpointMutation, 260)...)
		suggestions = append(suggestions, validateSetupTextBudget("setup", "long_form_plan.endgame_promise", "long_form_plan", plan.EndgamePromise, 260)...)
		suggestions = append(suggestions, validateSetupStringListBudget("setup", "long_form_plan.escalation_ladder", plan.EscalationLadder, 12, 180)...)
		suggestions = append(suggestions, validateSetupStringListBudget("setup", "long_form_plan.reader_promises", plan.ReaderPromises, 10, 160)...)
		suggestions = append(suggestions, validateSetupStringListBudget("setup", "long_form_plan.volume_pattern", plan.VolumePattern, 12, 160)...)
	}

	for i, seed := range setup.CoreCast {
		target := fmt.Sprintf("core_cast[%d]", i)
		suggestions = append(suggestions, validateSetupTextBudget("character", target+".story_function", seed.Name, seed.StoryFunction, 240)...)
		suggestions = append(suggestions, validateSetupTextBudget("character", target+".relationship_arc", seed.Name, seed.RelationshipArc, 240)...)
		suggestions = append(suggestions, validateSetupTextBudget("character", target+".payoff", seed.Name, seed.Payoff, 240)...)
		if len(seed.StorylineRefs) > 5 {
			suggestions = append(suggestions, qualitySuggestion("character", target+".storyline_refs", seed.Name, "setup list field is too large for stable prompting", "Link each core cast seed to only its strongest 1-5 storyline contracts.", models.PriorityLow))
		}
	}

	for i, storyline := range setup.Storylines {
		target := fmt.Sprintf("storylines[%d]", i)
		suggestions = append(suggestions, validateSetupTextBudget("plot", target+".description", storyline.Name, storyline.Description, 360)...)
		suggestions = append(suggestions, validateStorylineSerialHintBudget(target+".repeatable_pressure", storyline.Name, storyline.RepeatablePressure)...)
		suggestions = append(suggestions, validateStorylineSerialHintBudget(target+".payoff_cadence", storyline.Name, storyline.PayoffCadence)...)
		suggestions = append(suggestions, validateStorylineSerialHintBudget(target+".mutation", storyline.Name, storyline.Mutation)...)
		suggestions = append(suggestions, validateStorylineSerialHintBudget(target+".failure_mode", storyline.Name, storyline.FailureMode)...)
		suggestions = append(suggestions, validateSetupTextBudget("plot", target+".desire", storyline.Name, storyline.Desire, 240)...)
		suggestions = append(suggestions, validateSetupTextBudget("plot", target+".opposition", storyline.Name, storyline.Opposition, 240)...)
		suggestions = append(suggestions, validateSetupTextBudget("plot", target+".stakes", storyline.Name, storyline.Stakes, 240)...)
		suggestions = append(suggestions, validateSetupTextBudget("plot", target+".turn", storyline.Name, storyline.Turn, 240)...)
		suggestions = append(suggestions, validateSetupTextBudget("plot", target+".payoff", storyline.Name, storyline.Payoff, 240)...)
		suggestions = append(suggestions, validateSetupStringListBudget("plot", target+".pressure_points", storyline.PressurePoints, 6, 180)...)
		suggestions = append(suggestions, validateAppealEngineBudget("appeal", target+".appeal_engine", storyline.Name, storyline.AppealEngine)...)
	}

	for i, premise := range setup.Premises {
		target := fmt.Sprintf("premises[%d]", i)
		suggestions = append(suggestions, validateSetupTextBudget("setup", target+".description", premise.Name, premise.Description, 360)...)
		suggestions = append(suggestions, validateAppealEngineBudget("appeal", target+".appeal_engine", premise.Name, premise.AppealEngine)...)
		if len(premise.Progression) > 12 {
			suggestions = append(suggestions, qualitySuggestion("setup", target+".progression", premise.Name, "setup list field is too large for stable prompting", "Keep setup progression to major tiers. Put sub-level details into craft or a later system note.", models.PriorityLow))
		}
		for j, stage := range premise.Progression {
			stageTarget := fmt.Sprintf("%s.progression[%d]", target, j)
			suggestions = append(suggestions, validateSetupTextBudget("setup", stageTarget+".description", stage.Name, stage.Description, 240)...)
			suggestions = append(suggestions, validateSetupTextBudget("setup", stageTarget+".requirements", stage.Name, stage.Requirements, 220)...)
		}
	}

	for i, resource := range setup.WorldResources {
		target := fmt.Sprintf("world_resources[%d]", i)
		suggestions = append(suggestions, validateSetupTextBudget("setup", target+".description", resource.Name, resource.Description, 240)...)
	}

	return suggestions
}

func validateSetupTextBudget(category, targetID, targetName, value string, maxRunes int) []models.ReviewSuggestion {
	if runeLen(value) <= maxRunes {
		return nil
	}
	return []models.ReviewSuggestion{qualitySuggestion(category, targetID, targetName, "setup field is too long for stable prompting", fmt.Sprintf("Shorten this setup field to about %d characters. Setup should hold contracts and seeds; move full detail to craft or notes.", maxRunes), models.PriorityLow)}
}

func validateStorylineSerialHintBudget(targetID, targetName, value string) []models.ReviewSuggestion {
	if runeLen(value) <= 240 {
		return nil
	}
	return []models.ReviewSuggestion{qualitySuggestion("plot", targetID, targetName, "storyline serial engine hint is too long", "Compress this hint into one reusable pressure/cadence/mutation rule so compose can apply it consistently.", models.PriorityLow)}
}

func validateSetupStringListBudget(category, targetID string, values []string, maxItems, maxRunes int) []models.ReviewSuggestion {
	var suggestions []models.ReviewSuggestion
	if len(values) > maxItems {
		suggestions = append(suggestions, qualitySuggestion(category, targetID, targetID, "setup list field is too large for stable prompting", fmt.Sprintf("Keep this list to the strongest %d item(s); move lower-level detail to craft or notes.", maxItems), models.PriorityLow))
	}
	for i, value := range values {
		itemTarget := fmt.Sprintf("%s[%d]", targetID, i)
		suggestions = append(suggestions, validateSetupTextBudget(category, itemTarget, itemTarget, value, maxRunes)...)
	}
	return suggestions
}

func validateAppealEngineBudget(category, targetID, targetName string, engine *models.AppealEngine) []models.ReviewSuggestion {
	if engine == nil {
		return nil
	}
	var suggestions []models.ReviewSuggestion
	fields := []struct {
		name  string
		value string
	}{
		{"appeal", engine.Appeal},
		{"surface_limit", engine.SurfaceLimit},
		{"exploit", engine.Exploit},
		{"signature_win", engine.SignatureWin},
		{"upgrade_path", engine.UpgradePath},
		{"opponent_misread", engine.OpponentMisread},
		{"reward_type", engine.RewardType},
	}
	for _, field := range fields {
		suggestions = append(suggestions, validateSetupTextBudget(category, targetID+"."+field.name, targetName, field.value, 180)...)
	}
	return suggestions
}

func runeLen(value string) int {
	return len([]rune(strings.TrimSpace(value)))
}

func validateCoreCastCapacity(setup *models.StorySetup) []models.ReviewSuggestion {
	var suggestions []models.ReviewSuggestion
	if !coreCastHasProtagonist(setup.CoreCast) {
		suggestions = append(suggestions, qualitySuggestion("character", "core_cast", "core_cast", "core cast has no protagonist", "Add one protagonist/lead seed so craft and outline share the same central character anchor.", models.PriorityHigh))
	}

	importantCount := importantCoreCastCount(setup.CoreCast)
	if importantCount < 5 {
		suggestions = append(suggestions, qualitySuggestion("character", "core_cast", "core_cast", "long-form setup has too few important core cast seeds", "For long-form fiction, seed at least five importance >= 8 roles such as protagonist, major lead, rival, mentor, antagonist, or key ally.", models.PriorityMedium))
	}

	entryCounts := coreCastEntryPhaseCounts(setup.CoreCast)
	if importantCount >= 3 && entryCounts["mid"]+entryCounts["late"]+entryCounts["series"] == 0 && entryCounts["opening"]+entryCounts["early"] > 0 {
		suggestions = append(suggestions, qualitySuggestion("structure", "core_cast", "core_cast", "core cast entry phases are front-loaded", "Reserve at least one important role for mid, late, or series entry so the long-form story can keep introducing fresh pressure.", models.PriorityMedium))
	}

	if importantCount >= 5 && coreCastRoleDiversity(setup.CoreCast) < 3 {
		suggestions = append(suggestions, qualitySuggestion("character", "core_cast", "core_cast", "core cast role diversity is too low", "Give the important cast at least three distinct story roles, such as protagonist, lead, rival, mentor, antagonist, ally, or faction operator.", models.PriorityMedium))
	}

	storylines := setupStorylineNameSet(setup)
	for i, seed := range setup.CoreCast {
		if seed.Importance < 8 {
			continue
		}
		target := fmt.Sprintf("core_cast[%d]", i)
		if strings.TrimSpace(seed.RelationshipArc) == "" {
			suggestions = append(suggestions, qualitySuggestion("character", target, seed.Name, "important core cast seed lacks relationship arc", "Add a high-level relationship movement such as mistrust to alliance, rivalry to respect, protector to dependent, or hidden enemy to revealed threat.", models.PriorityMedium))
		}
		if len(storylines) > 0 && len(seed.StorylineRefs) == 0 {
			suggestions = append(suggestions, qualitySuggestion("plot", target, seed.Name, "important core cast seed lacks storyline_refs", "Link important cast seeds to one or more setup.storylines names so craft can expand them for the right plot engine.", models.PriorityMedium))
		}
		for _, ref := range seed.StorylineRefs {
			key := strings.ToLower(strings.TrimSpace(ref))
			if key != "" && len(storylines) > 0 && !storylines[key] {
				suggestions = append(suggestions, qualitySuggestion("consistency", target, seed.Name, "core cast seed references unknown storyline", "Use an exact setup.storylines[].name value in storyline_refs, or add the missing storyline to setup.", models.PriorityMedium))
			}
		}
	}

	return suggestions
}

func storySetupWantsExpandedSystems(setup *models.StorySetup) bool {
	if setup == nil {
		return false
	}
	text := strings.ToLower(strings.Join(setup.Genres, " ") + " " + setup.Premise + " " + setup.Theme + " " + strings.Join(setup.Rules, " "))
	signals := []string{
		"fantasy", "sci-fi", "science fiction", "cultivation", "xianxia", "wuxia", "apocalypse", "post-apocalyptic", "mecha", "rpg",
		"科幻", "机甲", "废土", "末世", "升级", "进化", "修炼", "修仙", "玄幻", "异能", "虫族",
	}
	for _, signal := range signals {
		if strings.Contains(text, strings.ToLower(signal)) {
			return true
		}
	}
	return len(setup.Rules) >= 6 || len(setup.WorldResources) >= 4
}

func storySetupWantsCoreCast(setup *models.StorySetup) bool {
	if setup == nil {
		return false
	}
	text := strings.ToLower(strings.Join(setup.Genres, " ") + " " + setup.Premise + " " + setup.Theme)
	signals := []string{
		"web novel", "long-form", "serial", "series", "1000", "长篇", "连载", "群像", "恋爱", "后宫", "爽文",
		"sci-fi", "fantasy", "cultivation", "mecha", "apocalypse",
		"网文", "长篇", "爽", "主角", "女主", "男主",
	}
	for _, signal := range signals {
		if strings.Contains(text, strings.ToLower(signal)) {
			return true
		}
	}
	return len(setup.Storylines) >= 3 || len(setup.Premises) >= 3
}

func storySetupWantsLongFormPlan(setup *models.StorySetup) bool {
	if setup == nil {
		return false
	}
	text := strings.ToLower(strings.Join(setup.Genres, " ") + " " + setup.Premise + " " + setup.Theme + " " + strings.Join(setup.Rules, " "))
	signals := []string{
		"web novel", "long-form", "serial", "series", "1000", "300 chapters", "500 chapters",
		"网文", "长篇", "连载", "系列", "千章", "百章", "爽文",
	}
	for _, signal := range signals {
		if strings.Contains(text, strings.ToLower(signal)) {
			return true
		}
	}
	return len(setup.Storylines) >= 4 || len(setup.Premises) >= 4
}

func setupStorylineNameSet(setup *models.StorySetup) map[string]bool {
	result := make(map[string]bool)
	if setup == nil {
		return result
	}
	for _, storyline := range setup.Storylines {
		name := strings.ToLower(strings.TrimSpace(storyline.Name))
		if name != "" {
			result[name] = true
		}
	}
	return result
}

func coreCastHasProtagonist(seeds []models.CoreCastSeed) bool {
	for _, seed := range seeds {
		role := normalizedRole(seed.Role)
		if role == "protagonist" {
			return true
		}
	}
	return false
}

func importantCoreCastCount(seeds []models.CoreCastSeed) int {
	count := 0
	for _, seed := range seeds {
		if seed.Importance >= 8 {
			count++
		}
	}
	return count
}

func coreCastEntryPhaseCounts(seeds []models.CoreCastSeed) map[string]int {
	counts := make(map[string]int)
	for _, seed := range seeds {
		if seed.Importance < 8 {
			continue
		}
		phase := normalizedEntryPhase(seed.EntryPhase)
		if phase != "" {
			counts[phase]++
		}
	}
	return counts
}

func coreCastRoleDiversity(seeds []models.CoreCastSeed) int {
	roles := make(map[string]bool)
	for _, seed := range seeds {
		if seed.Importance < 8 {
			continue
		}
		role := normalizedRole(seed.Role)
		if role != "" {
			roles[role] = true
		}
	}
	return len(roles)
}

func normalizedRole(role string) string {
	value := strings.ToLower(strings.TrimSpace(role))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "protagonist", "main_character", "lead", "mc", "hero", "heroine", "male_lead", "male_1", "男主", "主角":
		return "protagonist"
	case "female_lead", "female_1", "love_interest", "romance_lead", "女主", "女1", "女一":
		return "lead"
	case "female_2", "female_3", "male_2", "male_3", "support", "supporting", "supporting_lead", "ally", "teammate", "女2", "女3", "男2", "男3":
		return "support"
	case "rival", "competitor", "foil", "对手", "宿敌":
		return "rival"
	case "villain", "antagonist", "enemy", "反派", "敌人":
		return "antagonist"
	case "mentor", "teacher", "master", "师父", "导师":
		return "mentor"
	default:
		return value
	}
}

func normalizedEntryPhase(phase string) string {
	value := strings.ToLower(strings.TrimSpace(phase))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "opening", "start", "act_1", "chapter_1", "开篇", "开局":
		return "opening"
	case "early", "first_volume", "volume_1", "前期":
		return "early"
	case "mid", "middle", "midgame", "中期":
		return "mid"
	case "late", "endgame", "后期":
		return "late"
	case "series", "sequel", "long_term", "系列", "长期":
		return "series"
	default:
		return value
	}
}

func hasStorylineArcContract(storyline models.Storyline) bool {
	highLevel := 0
	for _, value := range []string{storyline.Scope, storyline.SetupRole, storyline.PayoffStyle} {
		if strings.TrimSpace(value) != "" {
			highLevel++
		}
	}
	return highLevel >= 2 && len(storyline.PressurePoints) >= 2
}

func validateOutlineDirect(setup *models.StorySetup, outline *models.Outline) []models.ReviewSuggestion {
	if outline == nil {
		return []models.ReviewSuggestion{qualitySuggestion("outline", "global", "outline", "outline is nil", "Generate an outline before continuing.", models.PriorityCritical)}
	}
	if len(outline.Parts) == 0 {
		return []models.ReviewSuggestion{qualitySuggestion("outline", "parts", "parts", "outline has no parts", "Generate at least one part.", models.PriorityCritical)}
	}

	var suggestions []models.ReviewSuggestion
	suggestions = append(suggestions, validateOutlineLongFormAlignment(setup, outline)...)
	setupResources := setupResourceNames(setup)
	totalStorylineAdvances := 0

	for partIdx := range outline.Parts {
		part := &outline.Parts[partIdx]
		partTarget := coalesceString(part.ID, fmt.Sprintf("part[%d]", partIdx))
		if strings.TrimSpace(part.Title) == "" || strings.TrimSpace(part.Summary) == "" {
			suggestions = append(suggestions, qualitySuggestion("structure", partTarget, part.Title, "part lacks title or summary", "Give each part a clear title and arc summary.", models.PriorityHigh))
		}
		if len(part.Volumes) == 0 {
			suggestions = append(suggestions, qualitySuggestion("structure", partTarget, part.Title, "part has no volumes", "Generate volumes for this part.", models.PriorityHigh))
		}
		for volIdx := range part.Volumes {
			volume := &part.Volumes[volIdx]
			volumeTarget := coalesceString(volume.ID, fmt.Sprintf("%s.volume[%d]", partTarget, volIdx))
			if strings.TrimSpace(volume.Title) == "" || strings.TrimSpace(volume.Summary) == "" {
				suggestions = append(suggestions, qualitySuggestion("structure", volumeTarget, volume.Title, "volume lacks title or summary", "Give each volume a clear title and plot function.", models.PriorityHigh))
			}
			if len(volume.Chapters) == 0 {
				suggestions = append(suggestions, qualitySuggestion("structure", volumeTarget, volume.Title, "volume has no chapters", "Generate chapters for this volume.", models.PriorityHigh))
			}
			if len(volume.Chapters) > 0 {
				if missing := missingVolumePayoffFields(volume.PayoffContract); len(missing) > 0 {
					issue := "volume lacks payoff_contract"
					if volume.PayoffContract != nil && volumePayoffCompleteness(volume.PayoffContract) > 0 {
						issue = "volume has a thin payoff_contract"
					}
					suggestions = append(suggestions, qualitySuggestion("appeal", volumeTarget, volume.Title, issue, fmt.Sprintf("Fill payoff_contract so this volume has a clear reader promise and big win: %s.", strings.Join(missing, ", ")), models.PriorityMedium))
				}
			}
			for chapIdx := range volume.Chapters {
				chapter := &volume.Chapters[chapIdx]
				totalStorylineAdvances += len(chapter.StorylineAdvances)
				suggestions = append(suggestions, validateChapterDirect(chapter, setupResources)...)
			}
		}
	}

	if setup != nil && len(setup.Storylines) > 0 && totalStorylineAdvances == 0 {
		suggestions = append(suggestions, qualitySuggestion("plot", "storyline_advances", "storyline_advances", "outline never advances setup storylines", "Add sparse storyline_advances to key chapters that create pressure, reveal, reversal, or payoff.", models.PriorityMedium))
	}

	return suggestions
}

func validateOutlineLongFormAlignment(setup *models.StorySetup, outline *models.Outline) []models.ReviewSuggestion {
	if setup == nil || setup.LongFormPlan == nil || outline == nil {
		return nil
	}
	plan := setup.LongFormPlan
	var suggestions []models.ReviewSuggestion
	totalChapters := countOutlineChapters(outline)
	totalVolumes := countOutlineVolumes(outline)
	if plan.TargetChapters > 0 && totalChapters > 0 {
		minExpected := int(float64(plan.TargetChapters) * 0.8)
		if totalChapters < minExpected {
			suggestions = append(suggestions, qualitySuggestion("structure", "outline", "outline", "outline chapter count is far below long_form_plan target", fmt.Sprintf("Align novel.json structure or long_form_plan.target_chapters; outline has %d chapter(s), target is %d.", totalChapters, plan.TargetChapters), models.PriorityMedium))
		}
	}
	if plan.TargetVolumes > 0 && totalVolumes > 0 {
		minExpected := int(float64(plan.TargetVolumes) * 0.8)
		if totalVolumes < minExpected {
			suggestions = append(suggestions, qualitySuggestion("structure", "outline", "outline", "outline volume count is far below long_form_plan target", fmt.Sprintf("Align novel.json structure or long_form_plan.target_volumes; outline has %d volume(s), target is %d.", totalVolumes, plan.TargetVolumes), models.PriorityMedium))
		}
	}
	if len(plan.VolumePattern) >= 4 {
		missingPayoff := 0
		for _, part := range outline.Parts {
			for _, volume := range part.Volumes {
				if volume.PayoffContract.IsZero() {
					missingPayoff++
				}
			}
		}
		if totalVolumes > 0 && missingPayoff*2 > totalVolumes {
			suggestions = append(suggestions, qualitySuggestion("appeal", "payoff_contract", "payoff_contract", "long-form outline has too many volumes without payoff_contract", "Use long_form_plan.volume_pattern to give most volumes a clear question, power promise, big win, visible reward, and next bigger game.", models.PriorityMedium))
		}
	}
	return suggestions
}

func validateChapterDirect(chapter *models.Chapter, setupResources map[string]bool) []models.ReviewSuggestion {
	var suggestions []models.ReviewSuggestion
	if chapter == nil {
		return []models.ReviewSuggestion{qualitySuggestion("outline", "chapter", "chapter", "chapter is nil", "Regenerate this chapter.", models.PriorityCritical)}
	}
	target := coalesceString(chapter.ID, chapter.Title)
	if target == "" {
		target = "chapter"
	}

	required := map[string]string{
		"id":       chapter.ID,
		"title":    chapter.Title,
		"summary":  chapter.Summary,
		"location": chapter.Location,
		"conflict": chapter.Conflict,
		"pacing":   chapter.Pacing,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			suggestions = append(suggestions, qualitySuggestion("structure", target, chapter.Title, fmt.Sprintf("chapter missing %s", field), "Fill the required chapter field before downstream writing.", models.PriorityHigh))
		}
	}
	if len(chapter.Characters) == 0 {
		suggestions = append(suggestions, qualitySuggestion("character", target, chapter.Title, "chapter has no characters", "List the characters present in this chapter.", models.PriorityHigh))
	}
	if len(chapter.Events) == 0 {
		suggestions = append(suggestions, qualitySuggestion("plot", target, chapter.Title, "chapter has no events", "Add 3-5 concrete state-changing events.", models.PriorityHigh))
	} else if len(chapter.Events) < 3 || len(chapter.Events) > 5 {
		suggestions = append(suggestions, qualitySuggestion("plot", target, chapter.Title, "chapter event count should be 3-5", "Split or consolidate events so each one records a meaningful state change.", models.PriorityMedium))
	}
	if len(chapter.Events) > 0 {
		if missing := missingChapterPayoffFields(chapter.ChapterPayoff); len(missing) > 0 {
			issue := "chapter lacks chapter_payoff"
			if chapter.ChapterPayoff != nil && chapterPayoffCompleteness(chapter.ChapterPayoff) > 0 {
				issue = "chapter has a thin chapter_payoff"
			}
			suggestions = append(suggestions, qualitySuggestion("appeal", target, chapter.Title, issue, fmt.Sprintf("Fill chapter_payoff so write generation can dramatize the win pattern: %s.", strings.Join(missing, ", ")), models.PriorityMedium))
		}
	}

	hasCombat := false
	for idx := range chapter.Events {
		evt := &chapter.Events[idx]
		if strings.TrimSpace(evt.GetActor()) == "" || strings.TrimSpace(evt.GetAction()) == "" || strings.TrimSpace(evt.GetTarget()) == "" {
			suggestions = append(suggestions, qualitySuggestion("plot", target, chapter.Title, "event lacks actor, action, or target", "Use actor/action/target fields, or compatible old event fields.", models.PriorityMedium))
		}
		action := strings.ToLower(strings.TrimSpace(evt.GetAction()))
		eventType := strings.ToLower(strings.TrimSpace(evt.Type))
		if action == models.ActionCombat || action == models.ActionDefeat || eventType == "combat" {
			hasCombat = true
		}
	}
	if hasCombat && len(chapter.Enemies) == 0 {
		suggestions = append(suggestions, qualitySuggestion("plot", target, chapter.Title, "combat chapter has no enemies", "Declare enemies with faction, tier, count, and boss status when relevant.", models.PriorityHigh))
	}

	if len(chapter.Scenes) == 0 {
		suggestions = append(suggestions, qualitySuggestion("structure", target, chapter.Title, "chapter has no scenes", "Add 2-3 scenes with scene-level beats.", models.PriorityHigh))
	} else if len(chapter.Scenes) < 2 || len(chapter.Scenes) > 3 {
		suggestions = append(suggestions, qualitySuggestion("structure", target, chapter.Title, "chapter should have 2-3 scenes", "Split the chapter into 2-3 focused scenes.", models.PriorityMedium))
	}
	for i, scene := range chapter.Scenes {
		if scene.Order != i+1 {
			suggestions = append(suggestions, qualitySuggestion("structure", target, chapter.Title, "scene order is not sequential from 1", "Number scenes from 1 without gaps.", models.PriorityMedium))
		}
		if strings.TrimSpace(scene.POV) == "" || strings.TrimSpace(scene.Goal) == "" || len(scene.Beats) == 0 {
			suggestions = append(suggestions, qualitySuggestion("structure", target, chapter.Title, "scene lacks pov, goal, or beats", "Each scene needs POV, goal, and 1-2 beats.", models.PriorityHigh))
		}
	}

	if strings.TrimSpace(chapter.Timeline.Anchor) == "" {
		suggestions = append(suggestions, qualitySuggestion("continuity", target, chapter.Title, "chapter timeline lacks anchor", "Add a relative time anchor such as Day 1 morning or three months later.", models.PriorityMedium))
	}
	if chapter.Timeline.TimeJump && strings.TrimSpace(chapter.Timeline.Transition) == "" {
		suggestions = append(suggestions, qualitySuggestion("continuity", target, chapter.Title, "time jump lacks transition", "Explain what happened during the gap.", models.PriorityHigh))
	}
	if strings.TrimSpace(chapter.StateAnchor.Location) == "" {
		suggestions = append(suggestions, qualitySuggestion("continuity", target, chapter.Title, "state_anchor lacks start location", "Set state_anchor.location to the protagonist's location at chapter start.", models.PriorityMedium))
	}

	for _, entry := range chapter.ResourceLedger {
		if entry.Start+entry.Delta != entry.End {
			suggestions = append(suggestions, qualitySuggestion("logic", target, chapter.Title, fmt.Sprintf("resource ledger arithmetic is invalid for %s", entry.Item), "Ensure each resource ledger entry satisfies start + delta == end.", models.PriorityHigh))
		}
		itemKey := strings.ToLower(strings.TrimSpace(entry.Item))
		if len(setupResources) > 0 && itemKey != "" && !setupResources[itemKey] {
			suggestions = append(suggestions, qualitySuggestion("consistency", target, chapter.Title, fmt.Sprintf("resource %q is not declared in setup.world_resources", entry.Item), "Use a stable setup.world_resources name or add the resource to setup.", models.PriorityMedium))
		}
	}

	return suggestions
}

func countOutlineChapters(outline *models.Outline) int {
	if outline == nil {
		return 0
	}
	total := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			total += len(volume.Chapters)
		}
	}
	return total
}

func countOutlineVolumes(outline *models.Outline) int {
	if outline == nil {
		return 0
	}
	total := 0
	for _, part := range outline.Parts {
		total += len(part.Volumes)
	}
	return total
}

func storylineTexture(storyline models.Storyline) int {
	texture := 0
	for _, value := range []string{
		storyline.Desire,
		storyline.Opposition,
		storyline.Stakes,
		storyline.Turn,
		storyline.Payoff,
		storyline.OpenQuestion,
		storyline.Scope,
		storyline.PayoffStyle,
		storyline.SetupRole,
		storyline.RepeatablePressure,
		storyline.PayoffCadence,
		storyline.Mutation,
		storyline.FailureMode,
	} {
		if strings.TrimSpace(value) != "" {
			texture++
		}
	}
	if len(storyline.PressurePoints) > 0 {
		texture++
	}
	return texture
}

func setupResourceNames(setup *models.StorySetup) map[string]bool {
	result := make(map[string]bool)
	if setup == nil {
		return result
	}
	for _, resource := range setup.WorldResources {
		name := strings.ToLower(strings.TrimSpace(resource.Name))
		if name != "" {
			result[name] = true
		}
	}
	return result
}

func qualitySuggestion(category, targetID, targetName, issue, suggestion, priority string) models.ReviewSuggestion {
	return models.ReviewSuggestion{
		Category:   category,
		TargetID:   targetID,
		TargetName: targetName,
		Issue:      issue,
		Suggestion: suggestion,
		Priority:   priority,
	}
}

func coalesceString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func appealEngineCompleteness(engine *models.AppealEngine) int {
	if engine == nil {
		return 0
	}
	count := 0
	for _, value := range []string{
		engine.Appeal,
		engine.SurfaceLimit,
		engine.Exploit,
		engine.SignatureWin,
		engine.UpgradePath,
		engine.OpponentMisread,
		engine.RewardType,
	} {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func missingAppealEngineFields(engine *models.AppealEngine) []string {
	fields := []struct {
		name  string
		value string
	}{
		{"appeal", engineValue(engine, func(e *models.AppealEngine) string { return e.Appeal })},
		{"surface_limit", engineValue(engine, func(e *models.AppealEngine) string { return e.SurfaceLimit })},
		{"exploit", engineValue(engine, func(e *models.AppealEngine) string { return e.Exploit })},
		{"signature_win", engineValue(engine, func(e *models.AppealEngine) string { return e.SignatureWin })},
		{"upgrade_path", engineValue(engine, func(e *models.AppealEngine) string { return e.UpgradePath })},
		{"opponent_misread", engineValue(engine, func(e *models.AppealEngine) string { return e.OpponentMisread })},
		{"reward_type", engineValue(engine, func(e *models.AppealEngine) string { return e.RewardType })},
	}
	var missing []string
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	return missing
}

func engineValue(engine *models.AppealEngine, pick func(*models.AppealEngine) string) string {
	if engine == nil {
		return ""
	}
	return pick(engine)
}

func volumePayoffCompleteness(contract *models.VolumePayoffContract) int {
	if contract == nil {
		return 0
	}
	count := 0
	for _, value := range []string{
		contract.VolumeQuestion,
		contract.PowerPromise,
		contract.MainOpponentMisread,
		contract.BigWin,
		contract.VisibleReward,
		contract.ReputationShift,
		contract.NextBiggerGame,
	} {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func missingVolumePayoffFields(contract *models.VolumePayoffContract) []string {
	if contract == nil {
		return []string{"volume_question", "power_promise", "main_opponent_misread", "big_win", "visible_reward", "reputation_shift", "next_bigger_game"}
	}
	var missing []string
	for _, field := range []struct {
		name  string
		value string
	}{
		{"volume_question", contract.VolumeQuestion},
		{"power_promise", contract.PowerPromise},
		{"main_opponent_misread", contract.MainOpponentMisread},
		{"big_win", contract.BigWin},
		{"visible_reward", contract.VisibleReward},
		{"reputation_shift", contract.ReputationShift},
		{"next_bigger_game", contract.NextBiggerGame},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	return missing
}

func chapterPayoffCompleteness(payoff *models.ChapterPayoff) int {
	if payoff == nil {
		return 0
	}
	count := 0
	for _, value := range []string{
		payoff.Desire,
		payoff.Pressure,
		payoff.CleverMove,
		payoff.PayoffMoment,
		payoff.Reward,
		payoff.SocialProof,
		payoff.Hook,
	} {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func missingChapterPayoffFields(payoff *models.ChapterPayoff) []string {
	if payoff == nil {
		return []string{"desire", "pressure", "clever_move", "payoff_moment", "reward", "social_proof", "hook"}
	}
	var missing []string
	for _, field := range []struct {
		name  string
		value string
	}{
		{"desire", payoff.Desire},
		{"pressure", payoff.Pressure},
		{"clever_move", payoff.CleverMove},
		{"payoff_moment", payoff.PayoffMoment},
		{"reward", payoff.Reward},
		{"social_proof", payoff.SocialProof},
		{"hook", payoff.Hook},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	return missing
}
