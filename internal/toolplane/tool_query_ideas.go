// The ideas query section: deterministic structured nonsense for setup and
// outline ideation. It never reads or writes project state, so it can run from
// any directory and remains a pure function of its flags and seed.

package toolplane

import (
	"fmt"
	"strings"

	"novelgen/internal/fiction/domain/creativity"
)

// QueryIdeas returns a list of provocations plus the exact seed needed to
// replay the draw. The caller is expected to rationalize the sentences into
// canon facts rather than treating them as finished premises.
func QueryIdeas(queryType, genre, level, mode string, seed int64, count, selectCount int, source, category string) (ToolResponse, error) {
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
	if queryType == "engine" {
		result, err := creativity.GenerateEngines(creativity.EngineRequest{
			Genre: genre,
			Seed:  seed,
			Count: count,
		})
		if err != nil {
			return ToolResponse{}, err
		}
		return ToolResponse{
			OK:       true,
			Section:  "ideas",
			Count:    len(result.Engines),
			Results:  result.Engines,
			Warnings: result.Warnings,
			Meta: map[string]interface{}{
				"seed":  result.Seed,
				"genre": result.Genre,
				"mode":  "engine",
				"usage": []string{
					"Treat one engine card as a closed causal packet; do not combine premise, cast, or resources from a different engine unless the user explicitly asks.",
					"Use worldpack for a ready-to-inspect bundle of premise, storylines, cast, and resources.",
					"Fill every required_question and reject the card if any reject_if condition is true.",
				},
				"replay_command": fmt.Sprintf(
					"novelgen tool query ideas --type engine --genre %s --seed %d --count %d",
					result.Genre, result.Seed, len(result.Engines),
				),
			},
		}, nil
	}
	if queryType == "reference" {
		cards, err := creativity.ReferenceCards(source)
		if err != nil {
			return ToolResponse{}, err
		}
		return ToolResponse{
			OK:      true,
			Section: "ideas",
			Count:   len(cards),
			Results: cards,
			Meta: map[string]interface{}{
				"mode": "reference",
				"usage": []string{
					"Reference cards are researched pattern cards, not permission to copy protected proper nouns, characters, or plot text.",
					"Pass the same --source to worldpack to apply one reference lens to every generated pack.",
					"The reference supplies structural mechanics and tone; the engine, names, resources, and storylines remain original.",
				},
			},
		}, nil
	}
	if queryType == "namecard" {
		referenceID := ""
		if strings.TrimSpace(source) != "" {
			refs, err := creativity.ReferenceCards(source)
			if err != nil {
				return ToolResponse{}, err
			}
			referenceID = refs[0].ID
		}
		result, err := creativity.GenerateNameCards(creativity.NameRequest{
			Genre:       genre,
			Seed:        seed,
			Count:       count,
			Source:      source,
			ReferenceID: referenceID,
		})
		if err != nil {
			return ToolResponse{}, err
		}
		replayCommand := fmt.Sprintf(
			"novelgen tool query ideas --type namecard --genre %s --seed %d --count %d",
			result.Genre, result.Seed, len(result.Cards),
		)
		if source != "" {
			replayCommand += fmt.Sprintf(" --source %q", source)
		}
		return ToolResponse{
			OK:       true,
			Section:  "ideas",
			Count:    len(result.Cards),
			Results:  result.Cards,
			Warnings: result.Warnings,
			Meta: map[string]interface{}{
				"seed":  result.Seed,
				"genre": result.Genre,
				"mode":  "namecard",
				"usage": []string{
					"Draw the archetype and contradiction before using the generated name.",
					"Use speech_style and body_language as active roleplay constraints, not decorative notes.",
					"Names are original; source records only inform surname range, tone, and naming logic.",
				},
				"name_sources":   creativity.NameResearchSources(),
				"replay_command": replayCommand,
			},
		}, nil
	}
	if queryType == "itemcard" {
		result, err := creativity.GenerateItemCards(creativity.ItemRequest{
			Genre:    genre,
			Seed:     seed,
			Count:    count,
			Source:   source,
			Category: category,
		})
		if err != nil {
			return ToolResponse{}, err
		}
		replayCommand := fmt.Sprintf(
			"novelgen tool query ideas --type itemcard --genre %s --seed %d --count %d",
			result.Genre, result.Seed, len(result.Items),
		)
		if source != "" {
			replayCommand += fmt.Sprintf(" --source %q", source)
		}
		if category != "" {
			replayCommand += fmt.Sprintf(" --category %q", category)
		}
		return ToolResponse{
			OK:       true,
			Section:  "ideas",
			Count:    len(result.Items),
			Results:  result.Items,
			Warnings: result.Warnings,
			Meta: map[string]interface{}{
				"seed":       result.Seed,
				"genre":      result.Genre,
				"mode":       "itemcard",
				"categories": creativity.ItemCategories(),
				"usage": []string{
					"Use category when the scene needs a specific kind of object: resource, commodity, equipment, consumable, prop, vehicle, or credential.",
					"An item must change a scene through function, cost, limit, counterplay, or evidence; reject decorative loot.",
					"Items drawn with worldpack share its pack_id and reference_id and are structurally linked to the engine artifact.",
				},
				"replay_command": replayCommand,
			},
		}, nil
	}
	if queryType == "worldpack" || queryType == "premise" || queryType == "storyline" || queryType == "cast" || queryType == "resource" {
		result, err := creativity.GenerateWorldPacks(creativity.EngineRequest{
			Genre:  genre,
			Seed:   seed,
			Count:  count,
			Source: source,
		})
		if err != nil {
			return ToolResponse{}, err
		}

		var results interface{}
		itemCount := 0
		switch queryType {
		case "worldpack":
			results = result.Packs
			itemCount = len(result.Packs)
		case "premise":
			items := make([]creativity.PremiseCard, 0, len(result.Packs))
			for _, pack := range result.Packs {
				items = append(items, pack.Premise)
			}
			results = items
			itemCount = len(items)
		case "storyline":
			items := make([]creativity.StorylineCard, 0, len(result.Packs)*3)
			for _, pack := range result.Packs {
				items = append(items, pack.Storylines...)
			}
			results = items
			itemCount = len(items)
		case "cast":
			items := make([]creativity.CastCard, 0, len(result.Packs)*4)
			for _, pack := range result.Packs {
				items = append(items, pack.Cast...)
			}
			results = items
			itemCount = len(items)
		case "resource":
			items := make([]creativity.ResourceCard, 0, len(result.Packs)*2)
			for _, pack := range result.Packs {
				items = append(items, pack.Resources...)
			}
			results = items
			itemCount = len(items)
		}
		replayCommand := fmt.Sprintf(
			"novelgen tool query ideas --type %s --genre %s --seed %d --count %d",
			queryType, result.Genre, result.Seed, len(result.Packs),
		)
		if source = strings.TrimSpace(source); source != "" {
			replayCommand += fmt.Sprintf(" --source %q", source)
		}

		return ToolResponse{
			OK:       true,
			Section:  "ideas",
			Count:    itemCount,
			Results:  results,
			Warnings: result.Warnings,
			Meta: map[string]interface{}{
				"seed":  result.Seed,
				"genre": result.Genre,
				"mode":  queryType,
				"packs": len(result.Packs),
				"compatibility": []string{
					"Cards with the same pack_id are derived from one engine card.",
					"Use the same genre, seed, and pack count across premise/storyline/cast/resource draws to retrieve matching decks.",
					"worldpack returns the complete validated bundle in one call.",
					"A --source reference is a structural style lens; generated names and institutions remain original.",
				},
				"usage": []string{
					"Get worldpack first to select coherent cards, or draw one deck and match pack_id across the other decks.",
					"Do not merge a protagonist, secret, storyline, and resource from different pack_id values without rebuilding their causal relationship.",
					"Treat cast names as presentation; the role, want, secret, and function are the contract.",
				},
				"replay_command": replayCommand,
			},
		}, nil
	}
	switch queryType {
	case "", "provocation":
		// Keep the requested level.
	case creativity.LevelAxiom, creativity.LevelEvent, creativity.LevelLaw, creativity.LevelName:
		level = queryType
	default:
		return ToolResponse{}, fmt.Errorf("unsupported ideas type %q; valid types: provocation, axiom, event, law, scenario, engine, worldpack, premise, storyline, cast, namecard, resource, itemcard, reference, name", queryType)
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
