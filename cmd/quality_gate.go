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
