package setupcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/creativity"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
)

func writeScenarioPlan(root *store.Root, seed int64, plan creativity.ScenarioPlan) (string, error) {
	if root == nil {
		return "", fmt.Errorf("project root is required")
	}
	path := root.SetupExploreFile(seed, "scenario_plan.json")
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal scenario plan: %w", err)
	}
	if err := root.Write(path, data, store.FileMode); err != nil {
		return "", fmt.Errorf("write scenario plan: %w", err)
	}
	return path, nil
}

func predictScenarioPlan(ctx context.Context, agent *agents.SetupAgent, idea string, plan creativity.ScenarioPlan, limit int, useSDK bool) (creativity.ScenarioPlan, error) {
	if agent == nil {
		return plan, fmt.Errorf("setup agent is required")
	}
	if limit < len(plan.Selected) {
		limit = len(plan.Selected)
	}
	if limit <= 0 {
		limit = 5
	}
	items := append([]creativity.ScenarioPlanItem(nil), plan.Selected...)
	items = append(items, plan.Rejected...)
	if limit > len(items) {
		limit = len(items)
	}
	for i := 0; i < limit; i++ {
		input := agents.ScenarioPredictionInput{
			Idea:     idea,
			Scenario: items[i].Card,
		}
		var prediction agents.ScenarioPredictionOutput
		var err error
		if useSDK {
			prediction, err = agent.PredictScenarioWithAgentSDK(ctx, input)
		} else {
			prediction, err = agent.PredictScenario(ctx, input)
		}
		if err != nil {
			return plan, fmt.Errorf("predict scenario %s: %w", items[i].Card.ID, err)
		}
		predictability := clampScenarioInt(prediction.Predictability)
		novelty := clampScenarioInt(prediction.NormalizedNovelty())
		items[i].Predictability = predictability
		items[i].Novelty = novelty
		reason := strings.TrimSpace(prediction.Summary)
		if reason == "" {
			reason = strings.Join(prediction.ObviousDevelopments, "; ")
		}
		items[i].SelectionReason = reason
	}

	selectedCount := len(plan.Selected)
	selected := creativity.SelectScenarioItems(items, selectedCount)
	selectedIDs := map[string]bool{}
	for _, item := range selected {
		selectedIDs[item.Card.ID] = true
	}
	rejected := make([]creativity.ScenarioPlanItem, 0, len(items)-len(selected))
	for _, item := range items {
		if !selectedIDs[item.Card.ID] {
			rejected = append(rejected, item)
		}
	}
	return creativity.ScenarioPlan{
		Seed:     plan.Seed,
		Genre:    plan.Genre,
		Selected: selected,
		Rejected: rejected,
	}, nil
}

func clampScenarioInt(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// scenarioCoverageMissing checks the six structural elements against the
// generated setup. The scenario card is a contract; a noun mentioned only in a
// title does not satisfy it.
func scenarioCoverageMissing(setup *models.StorySetup, scenario *creativity.ScenarioCard) []string {
	if setup == nil || scenario == nil {
		return nil
	}
	text := setup.Premise + "\n" + setup.Theme + "\n" + strings.Join(setup.Rules, "\n")
	structural := setup.Premise + "\n" + strings.Join(setup.Rules, "\n")
	for _, storyline := range setup.Storylines {
		text += "\n" + storyline.Name + "\n" + storyline.Description + "\n" + storyline.Turn + "\n" + storyline.Payoff
		structural += "\n" + storyline.Name + "\n" + storyline.Description
	}
	for _, premise := range setup.Premises {
		text += "\n" + premise.Name + "\n" + premise.Description
		structural += "\n" + premise.Name + "\n" + premise.Description
		for _, stage := range premise.Progression {
			text += "\n" + stage.Name + "\n" + stage.Description
		}
	}
	for _, seed := range setup.CoreCast {
		text += "\n" + seed.Name + "\n" + seed.StoryFunction + "\n" + seed.Payoff
		structural += "\n" + seed.Name + "\n" + seed.StoryFunction
	}
	for _, resource := range setup.WorldResources {
		text += "\n" + resource.Name + "\n" + resource.Description
		structural += "\n" + resource.Name + "\n" + resource.Description
	}
	for _, event := range setup.WorldTimeline {
		text += "\n" + event.Event + "\n" + event.Impact
	}

	checks := []struct {
		element    string
		terms      []string
		structural bool
	}{
		{"星战", []string{"星战", "舰队", "舰队联合体", "战争"}, false},
		{"战舰", []string{"战舰", "舰船", "母舰", "护航舰", "破冰船"}, false},
		{"机甲", []string{"机甲", "机甲驾驶员", "机甲部队"}, true},
		{"天灾", []string{"天灾", scenario.Hazard}, false},
		{"虫洞", []string{"虫洞"}, false},
		{"虫族", []string{"虫族", "虫群", "虫巢"}, false},
	}
	missing := make([]string, 0)
	for _, check := range checks {
		found := false
		source := text
		if check.structural {
			source = structural
		}
		for _, term := range check.terms {
			if term != "" && strings.Contains(source, term) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, check.element)
		}
	}
	return missing
}

func scenarioCoverageReview(missing []string, scenario *creativity.ScenarioCard) models.ReviewResult {
	suggestions := make([]models.ReviewSuggestion, 0, len(missing))
	for _, element := range missing {
		role := ""
		if scenario != nil {
			role = scenario.ElementRoles[element]
		}
		suggestions = append(suggestions, models.ReviewSuggestion{
			Category:   "structure",
			TargetID:   "scenario",
			TargetName: element,
			Issue:      "scenario element is missing from setup",
			Suggestion: fmt.Sprintf("Add %s as a structural rule, premise, storyline, or core cast function. Required role: %s. Do not replace it with a different unit or a decorative noun.", element, role),
			Priority:   models.PriorityHigh,
		})
	}
	return models.ReviewResult{
		Summary:     "Scenario card coverage is incomplete: " + strings.Join(missing, ", "),
		Suggestions: suggestions,
	}
}

func repairSetupWithScenarioCoverage(ctx context.Context, setup *models.StorySetup, scenario *creativity.ScenarioCard, projectConfig *models.ProjectConfig, cfg *llm.Config, client llm.Client, stage string) (*models.StorySetup, error) {
	if scenario == nil {
		return setup, nil
	}
	missing := scenarioCoverageMissing(setup, scenario)
	if len(missing) == 0 {
		return setup, nil
	}
	logger.Warn("Scenario card coverage missing after %s: %s; running one structural repair pass", stage, strings.Join(missing, ", "))
	agent := agents.NewSetupAgent(client, cfg, &projectConfig.LLM)
	agent.SetLanguage(projectConfig.Language)
	review := scenarioCoverageReview(missing, scenario)
	session := agents.NewRevisionSession("setup", fmt.Sprintf("Repair %s scenario-card coverage before saving project state.", stage))
	session.AddReview(1, review)
	output, err := agent.Improve(ctx, agents.SetupImproveInput{
		ExistingSetup:   *setup,
		ReviewResult:    review,
		RevisionContext: session.Prompt(),
	})
	if err != nil {
		return setup, fmt.Errorf("scenario coverage repair failed: %w", err)
	}
	repaired := &output.Setup
	remaining := scenarioCoverageMissing(repaired, scenario)
	if len(remaining) > 0 {
		return repaired, fmt.Errorf("scenario coverage still missing after repair: %s", strings.Join(remaining, ", "))
	}
	return repaired, nil
}
