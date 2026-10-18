// The ideas query section: deterministic structured nonsense for setup and
// outline ideation. It never reads or writes project state, so it can run from
// any directory and remains a pure function of its flags and seed.

package toolplane

import (
	"fmt"

	"novelgen/internal/fiction/domain/creativity"
)

// QueryIdeas returns a list of provocations plus the exact seed needed to
// replay the draw. The caller is expected to rationalize the sentences into
// canon facts rather than treating them as finished premises.
func QueryIdeas(queryType, genre, level, mode string, seed int64, count, selectCount int) (ToolResponse, error) {
	queryType = NormalizeKey(queryType)
	if queryType == "scenario" {
		if selectCount > 0 {
			considered := count
			if considered < selectCount*3 {
				considered = selectCount * 3
			}
			if considered <= 0 {
				considered = 10
			}
			result, err := creativity.GenerateScenarios(creativity.ScenarioRequest{
				Genre: genre,
				Seed:  seed,
				Count: considered,
			})
			if err != nil {
				return ToolResponse{}, err
			}
			plan := creativity.PlanScenarios(result, selectCount)
			meta := map[string]interface{}{
				"seed":       result.Seed,
				"genre":      result.Genre,
				"mode":       "scenario-plan",
				"considered": len(result.Cards),
				"predictor":  "deterministic familiarity heuristic",
				"usage": []string{
					"Each selected item contains a card plus family/predictability/novelty.",
					"Generate a full setup only for a selected card via setup gen --scenario-index.",
					"Do not compare full setups before this card-level selection.",
				},
				"replay_command": fmt.Sprintf(
					"novelgen tool query ideas --type scenario --genre %s --seed %d --count %d --select %d",
					result.Genre, result.Seed, considered, selectCount,
				),
			}
			return ToolResponse{
				OK:      true,
				Section: "ideas",
				Count:   len(plan.Selected),
				Results: plan.Selected,
				Meta:    meta,
			}, nil
		}
		result, err := creativity.GenerateScenarios(creativity.ScenarioRequest{
			Genre: genre,
			Seed:  seed,
			Count: count,
		})
		if err != nil {
			return ToolResponse{}, err
		}
		meta := map[string]interface{}{
			"seed":  result.Seed,
			"genre": result.Genre,
			"mode":  "scenario",
			"usage": []string{
				"Treat every field as a fixed causal role, not as a name to decorate.",
				"Expand the card into a setup; do not change scarcity, hazard, objective, enforcer, mobility, threat, cost, or twist.",
				"Every one of the six elements must remain structural.",
				"Reject the card if it needs a second magic rule or if any element becomes a noun-only decoration.",
			},
			"replay_command": fmt.Sprintf(
				"novelgen tool query ideas --type scenario --genre %s --seed %d --count %d",
				result.Genre, result.Seed, len(result.Cards),
			),
		}
		return ToolResponse{
			OK:      true,
			Section: "ideas",
			Count:   len(result.Cards),
			Results: result.Cards,
			Meta:    meta,
		}, nil
	}
	switch queryType {
	case "", "provocation":
		// Keep the requested level.
	case creativity.LevelAxiom, creativity.LevelEvent, creativity.LevelLaw, creativity.LevelName:
		level = queryType
	default:
		return ToolResponse{}, fmt.Errorf("unsupported ideas type %q; valid types: provocation, axiom, event, law, name", queryType)
	}

	result, err := creativity.Generate(creativity.Request{
		Level: level,
		Genre: genre,
		Mode:  mode,
		Seed:  seed,
		Count: count,
	})
	if err != nil {
		return ToolResponse{}, err
	}

	meta := map[string]interface{}{
		"seed":  result.Seed,
		"level": result.Level,
		"genre": result.Genre,
		"mode":  result.Mode,
		"usage": []string{
			"Treat each sentence as a constraint, not as a finished premise.",
			"Use exactly one unexplained axiom (axiom_budget=1); derive every other rule from it and ordinary incentives.",
			"Rationalize it into 3-5 canon facts with beneficiaries, costs, enforcement, and observable consequences.",
			"Follow derivation_steps in order; do not add a second magic rule to explain the first.",
			"Reject the candidate if any item in reject_if is true.",
			"Reject dream, hallucination, legend, metaphor, and coincidence explanations.",
			"Keep at most one impossible axiom per world; derive the rest through ordinary logic.",
			"Return 2-3 clues that can be planted before the reveal.",
		},
		"replay_command": fmt.Sprintf(
			"novelgen tool query ideas --type %s --genre %s --mode %s --seed %d --count %d",
			result.Level, result.Genre, result.Mode, result.Seed, len(result.Provocations),
		),
	}

	return ToolResponse{
		OK:       true,
		Section:  "ideas",
		Count:    len(result.Provocations),
		Results:  result.Provocations,
		Warnings: result.Warnings,
		Meta:     meta,
	}, nil
}
