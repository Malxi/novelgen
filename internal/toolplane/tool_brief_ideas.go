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
	case []creativity.WorldPack:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":              item.ID,
				"family":          item.Family,
				"premise_title":   item.Premise.Title,
				"reference":       item.Reference.Name,
				"reference_id":    item.Reference.ID,
				"artifact":        item.Engine.Artifact,
				"issuer":          item.Engine.Issuer,
				"storyline_count": len(item.Storylines),
				"cast_count":      len(item.Cast),
				"resource_count":  len(item.Resources),
				"item_count":      len(item.Items),
				"compatible":      item.Compatibility.Compatible,
			})
		}
		return out
	case []creativity.Engine:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":              item.ID,
				"family":          item.Family,
				"artifact":        item.Artifact,
				"issuer":          item.Issuer,
				"hidden_function": item.HiddenFunction,
				"protagonist":     item.ProtagonistRole,
			})
		}
		return out
	case []creativity.PremiseCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":           item.ID,
				"pack_id":      item.PackID,
				"reference_id": item.ReferenceID,
				"family":       item.Family,
				"title":        item.Title,
				"scarcity":     item.Scarcity,
			})
		}
		return out
	case []creativity.StorylineCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":                item.ID,
				"pack_id":           item.PackID,
				"reference_id":      item.ReferenceID,
				"archetype_id":      item.ArchetypeID,
				"arc_slot":          item.ArcSlot,
				"kind":              item.Kind,
				"title":             item.Title,
				"turn":              item.Turn,
				"linked_characters": item.LinkedCharacters,
				"linked_items":      item.LinkedItems,
			})
		}
		return out
	case []creativity.CastCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":           item.ID,
				"pack_id":      item.PackID,
				"reference_id": item.ReferenceID,
				"namecard_id":  item.RunNamecardID,
				"name":         item.Name,
				"role":         item.Role,
				"archetype":    item.Archetype,
				"personality":  item.Personality,
				"relationship": item.Relationship,
			})
		}
		return out
	case []creativity.NameCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":           item.ID,
				"name":         item.Name,
				"archetype":    item.Archetype,
				"personality":  item.Personality,
				"naming_style": item.NamingStyle,
				"speech_style": item.SpeechStyle,
				"pack_id":      item.PackID,
				"reference_id": item.ReferenceID,
			})
		}
		return out
	case []creativity.ItemCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":                   item.ID,
				"pack_id":              item.PackID,
				"reference_id":         item.ReferenceID,
				"name":                 item.Name,
				"base_object":          item.BaseObject,
				"category":             item.Category,
				"function":             item.Function,
				"limit":                item.Limit,
				"engine_link":          item.EngineLink,
				"reference_constraint": item.ReferenceConstraint,
			})
		}
		return out
	case []creativity.ResourceCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":           item.ID,
				"pack_id":      item.PackID,
				"reference_id": item.ReferenceID,
				"name":         item.Name,
				"category":     item.Category,
				"issuer":       item.Issuer,
			})
		}
		return out
	case []creativity.ReferenceCard:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]interface{}{
				"id":            item.ID,
				"name":          item.Name,
				"original_work": item.OriginalWork,
				"medium":        item.Medium,
				"source_url":    item.SourceURL,
				"core_pattern":  item.CorePattern,
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
