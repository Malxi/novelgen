package toolplane

import "novelgen/internal/fiction/domain/creativity"

// briefIdeaResults keeps the default view rich enough for an agent to solve the
// provocation, while index view stays route-only and cheap.
func briefIdeaResults(results interface{}, index bool) interface{} {
	if !index {
		return results
	}
	switch items := results.(type) {
	case []creativity.ScenarioPlanItem:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":               item.Card.ID,
				"premise_kernel":   item.Card.PremiseKernel,
				"family":           item.Family,
				"predictability":   item.Predictability,
				"novelty":          item.Novelty,
				"selection_reason": item.SelectionReason,
			})
		}
		return out
	case []creativity.ScenarioCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":             item.ID,
				"premise_kernel": item.PremiseKernel,
				"arena":          item.Arena,
				"scarcity":       item.Scarcity,
				"hazard":         item.Hazard,
				"objective":      item.Objective,
				"enforcer":       item.Enforcer,
				"mobility":       item.Mobility,
				"threat":         item.Threat,
				"cost":           item.Cost,
				"twist":          item.Twist,
			})
		}
		return out
	case []creativity.Provocation:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":            item.ID,
				"sentence":      item.Sentence,
				"required_role": item.RequiredRole,
				"template_id":   item.TemplateID,
			})
		}
		return out
	default:
		return results
	}
}
