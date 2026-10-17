// The story-setup query section and the clipped setup briefs other sections reuse.

package toolplane

import (
	"fmt"
	"novelgen/internal/models"
	"novelgen/internal/toolcmd"
	"strings"
)

func QueryStorySetup(ctx ToolProjectContext, queryType, name string) ToolResponse {
	if ctx.Setup == nil {
		return ToolResponse{OK: true, Count: 0, Results: nil}
	}
	switch normalizeSetupType(queryType) {
	case "index":
		return setupIndexResponse(ctx.Setup)
	case "", "all":
		return ToolResponse{OK: true, Count: 1, Results: ctx.Setup}
	case "search", "find", "refs":
		return queryStorySetupSearch(ctx.Setup, name)
	case "storyline":
		return queryStorylines(ctx, name)
	case "premise":
		return querySetupSlice("premise", name, ctx.Setup.Premises, func(v models.Premise) string { return v.Name })
	case "core-cast":
		return querySetupSlice("core-cast", name, ctx.Setup.CoreCast, func(v models.CoreCastSeed) string { return v.Name })
	case "resource":
		return querySetupSlice("resource", name, ctx.Setup.WorldResources, func(v models.WorldResource) string { return v.Name })
	case "timeline":
		return querySetupSlice("timeline", name, ctx.Setup.WorldTimeline, func(v models.WorldTimelineEntry) string { return v.Event })
	case "rules":
		// 创作铁律（日志边界、能力边界等）。review/setup 核对时按名字直查，
		// 免得只能靠 "--type search" 撞关键词或整本 --type all 拉全量。
		return querySetupSlice("rules", name, ctx.Setup.Rules, func(v string) string { return v })
	case "long-form-plan":
		count := 0
		if ctx.Setup.LongFormPlan != nil {
			count = 1
		}
		return ToolResponse{OK: true, Count: count, Results: ctx.Setup.LongFormPlan}
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported story-setup type %q", queryType)}}
	}
}

// setupIndexResponse 返回 setup 的轻量索引: 各主要字段的数量与名称列表,
// 供 agent 快速了解设定书全貌而不拉取完整数据(省 token)。
func setupIndexResponse(setup *models.StorySetup) ToolResponse {
	if setup == nil {
		return ToolResponse{OK: true, Count: 0, Results: nil}
	}
	coreCast := make([]coreCastBrief, 0, len(setup.CoreCast))
	for _, item := range setup.CoreCast {
		coreCast = append(coreCast, coreCastBrief{
			ID:         item.ID,
			Name:       item.Name,
			Role:       item.Role,
			Importance: item.Importance,
			EntryPhase: item.EntryPhase,
		})
	}
	storylines := make([]storylineBrief, 0, len(setup.Storylines))
	for _, item := range setup.Storylines {
		storylines = append(storylines, storylineBrief{
			Name:       item.Name,
			Type:       item.Type,
			Importance: item.Importance,
			Scope:      item.Scope,
		})
	}
	premises := make([]premiseBrief, 0, len(setup.Premises))
	for _, item := range setup.Premises {
		premises = append(premises, premiseBrief{Name: item.Name, Category: item.Category})
	}
	timeline := make([]worldTimelineBrief, 0, len(setup.WorldTimeline))
	for _, item := range setup.WorldTimeline {
		timeline = append(timeline, worldTimelineBrief{
			Year:  item.Year,
			Event: clipToolString(item.Event, 100),
		})
	}
	resources := make([]worldResourceBrief, 0, len(setup.WorldResources))
	for _, item := range setup.WorldResources {
		resources = append(resources, worldResourceBrief{
			Name:     item.Name,
			Category: item.Category,
			Scarcity: item.Scarcity,
		})
	}
	index := map[string]interface{}{
		"project_name":    setup.ProjectName,
		"genres":          setup.Genres,
		"premise":         clipToolString(setup.Premise, 240),
		"theme":           clipToolString(setup.Theme, 200),
		"rules_count":     len(setup.Rules),
		"target_audience": setup.TargetAudience,
		"tone":            setup.Tone,
		"pov_style":       setup.POVStyle,
		"core_cast":       coreCast,
		"storylines":      storylines,
		"premises":        premises,
		"world_timeline":  timeline,
		"world_resources": resources,
		"long_form_plan":  makeLongFormPlanBrief(setup.LongFormPlan, true),
		"core_cast_count": len(setup.CoreCast),
		"storyline_count": len(setup.Storylines),
		"premise_count":   len(setup.Premises),
		"timeline_count":  len(setup.WorldTimeline),
		"resource_count":  len(setup.WorldResources),
		"has_long_form":   setup.LongFormPlan != nil,
	}
	return ToolResponse{OK: true, Count: 1, Results: index}
}

func queryStorySetupSearch(setup *models.StorySetup, name string) ToolResponse {
	name = strings.TrimSpace(name)
	if name == "" {
		return ToolResponse{OK: false, Section: "story-setup", Count: 0, Warnings: []string{"--name is required for story-setup --type search"}}
	}

	const perSectionLimit = 3
	result := StorySetupSearchResult{
		Query: name,
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "core-cast", Name: name, View: "brief"}),
				toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "storyline", Name: name, View: "brief"}),
				toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "premise", Name: name, View: "brief"}),
				toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "resource", Name: name, View: "brief"}),
				toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "timeline", Name: name, View: "brief"}),
			},
			"hint": "Use the detail queries only when the compact search result is insufficient.",
		},
		Stats: map[string]int{},
	}

	coreCastTotal := 0
	for _, cast := range setup.CoreCast {
		if !textMatches(cast.Name, name) && !objectJSONContains(cast, name) {
			continue
		}
		coreCastTotal++
		if len(result.CoreCast) >= perSectionLimit {
			continue
		}
		result.CoreCast = append(result.CoreCast, coreCastBrief{
			ID:                 cast.ID,
			Name:               cast.Name,
			Role:               cast.Role,
			Importance:         cast.Importance,
			StoryFunction:      clipToolString(cast.StoryFunction, 100),
			RelationshipToLead: clipToolString(cast.RelationshipToLead, 80),
			RelationshipArc:    clipToolString(cast.RelationshipArc, 90),
			EntryPhase:         cast.EntryPhase,
			Payoff:             clipToolString(cast.Payoff, 90),
			StorylineRefs:      cast.StorylineRefs,
		})
	}

	storylineTotal := 0
	for _, storyline := range setup.Storylines {
		if !textMatches(storyline.Name, name) && !objectJSONContains(storyline, name) {
			continue
		}
		storylineTotal++
		if len(result.Storylines) >= perSectionLimit {
			continue
		}
		result.Storylines = append(result.Storylines, storylineBrief{
			Name:               storyline.Name,
			Type:               storyline.Type,
			Importance:         storyline.Importance,
			Scope:              storyline.Scope,
			Description:        clipToolString(storyline.Description, 110),
			SetupRole:          clipToolString(storyline.SetupRole, 80),
			RepeatablePressure: clipToolString(storyline.RepeatablePressure, 80),
			PayoffCadence:      clipToolString(storyline.PayoffCadence, 80),
			FailureMode:        clipToolString(storyline.FailureMode, 80),
			OpenQuestion:       clipToolString(storyline.OpenQuestion, 80),
		})
	}

	premiseTotal := 0
	for _, premise := range setup.Premises {
		if !textMatches(premise.Name, name) && !objectJSONContains(premise, name) {
			continue
		}
		premiseTotal++
		if len(result.Premises) >= perSectionLimit {
			continue
		}
		result.Premises = append(result.Premises, premiseBrief{
			Name:        premise.Name,
			Category:    premise.Category,
			Description: clipToolString(premise.Description, 120),
			Progression: makeProgressionBrief(premise.Progression),
		})
	}

	timelineTotal := 0
	for _, entry := range setup.WorldTimeline {
		if !textMatches(entry.Event, name) && !objectJSONContains(entry, name) {
			continue
		}
		timelineTotal++
		if len(result.WorldTimeline) >= perSectionLimit {
			continue
		}
		result.WorldTimeline = append(result.WorldTimeline, worldTimelineBrief{
			Year:           entry.Year,
			Event:          clipToolString(entry.Event, 100),
			Impact:         clipToolString(entry.Impact, 100),
			RelatedMystery: entry.RelatedMystery,
		})
	}

	resourceTotal := 0
	for _, resource := range setup.WorldResources {
		if !textMatches(resource.Name, name) && !objectJSONContains(resource, name) {
			continue
		}
		resourceTotal++
		if len(result.WorldResources) >= perSectionLimit {
			continue
		}
		result.WorldResources = append(result.WorldResources, worldResourceBrief{
			Name:        resource.Name,
			Category:    resource.Category,
			Scarcity:    resource.Scarcity,
			Description: clipToolString(resource.Description, 110),
		})
	}

	result.Stats["core_cast_total"] = coreCastTotal
	result.Stats["core_cast_returned"] = len(result.CoreCast)
	result.Stats["storylines_total"] = storylineTotal
	result.Stats["storylines_returned"] = len(result.Storylines)
	result.Stats["premises_total"] = premiseTotal
	result.Stats["premises_returned"] = len(result.Premises)
	result.Stats["world_timeline_total"] = timelineTotal
	result.Stats["world_timeline_returned"] = len(result.WorldTimeline)
	result.Stats["world_resources_total"] = resourceTotal
	result.Stats["world_resources_returned"] = len(result.WorldResources)

	total := coreCastTotal + storylineTotal + premiseTotal + timelineTotal + resourceTotal
	warnings := make([]string, 0)
	if coreCastTotal > len(result.CoreCast) {
		warnings = append(warnings, fmt.Sprintf("core_cast truncated from %d to %d", coreCastTotal, len(result.CoreCast)))
	}
	if storylineTotal > len(result.Storylines) {
		warnings = append(warnings, fmt.Sprintf("storylines truncated from %d to %d", storylineTotal, len(result.Storylines)))
	}
	if premiseTotal > len(result.Premises) {
		warnings = append(warnings, fmt.Sprintf("premises truncated from %d to %d", premiseTotal, len(result.Premises)))
	}
	if timelineTotal > len(result.WorldTimeline) {
		warnings = append(warnings, fmt.Sprintf("world_timeline truncated from %d to %d", timelineTotal, len(result.WorldTimeline)))
	}
	if resourceTotal > len(result.WorldResources) {
		warnings = append(warnings, fmt.Sprintf("world_resources truncated from %d to %d", resourceTotal, len(result.WorldResources)))
	}
	if total == 0 {
		warnings = append(warnings, fmt.Sprintf("no story-setup entries matched %q", name))
	}
	return ToolResponse{OK: true, Section: "story-setup", Count: total, Results: result, Warnings: warnings}
}

func querySetupSlice[T any](kind, name string, values []T, nameOf func(T) string) ToolResponse {
	hits := make([]T, 0)
	for _, value := range values {
		if name == "" || textMatches(nameOf(value), name) || objectJSONContains(value, name) {
			hits = append(hits, value)
		}
	}
	warnings := []string{}
	if strings.TrimSpace(name) != "" && len(hits) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s %q not found in story setup", kind, name))
	}
	return ToolResponse{OK: true, Section: "craft", Count: len(hits), Results: hits, Warnings: warnings}
}

func briefStorySetupForElementContext(setup *models.StorySetup, target, name string) interface{} {
	if setup == nil {
		return nil
	}
	brief := StorySetupBrief{
		ProjectName:    setup.ProjectName,
		Genres:         setup.Genres,
		Tone:           clipToolString(setup.Tone, 120),
		POVStyle:       setup.POVStyle,
		TargetAudience: clipToolString(setup.TargetAudience, 120),
		LongFormPlan:   makeLongFormPlanBrief(setup.LongFormPlan, true),
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				"novelgen tool query story-setup --type search --name <keyword> --view brief",
				"novelgen tool query story-setup --type storyline --name <name>",
				"novelgen tool query story-setup --type core-cast --name <name>",
				"novelgen tool query story-setup --type premise --name <name>",
				"novelgen tool query story-setup --type resource --name <name>",
			},
		},
	}
	if target == "character" {
		brief.CoreCast = append(brief.CoreCast, matchingCoreCastBriefs(setup.CoreCast, name, 4)...)
	}
	brief.Storylines = append(brief.Storylines, relevantStorylineBriefs(setup.Storylines, brief.CoreCast, 5)...)
	brief.Premises = append(brief.Premises, matchingPremiseBriefs(setup.Premises, name, 5)...)
	brief.WorldResources = append(brief.WorldResources, matchingWorldResourceBriefs(setup.WorldResources, name, 5)...)
	if len(brief.Storylines) == 0 && target != "character" {
		brief.Storylines = append(brief.Storylines, matchingStorylineBriefs(setup.Storylines, name, 3)...)
	}
	return brief
}

func matchingStorylineBriefs(values []models.Storyline, name string, limit int) []storylineBrief {
	out := make([]storylineBrief, 0)
	for _, storyline := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !objectJSONContains(storyline, name) {
			continue
		}
		out = append(out, storylineBrief{
			Name:        storyline.Name,
			Type:        storyline.Type,
			Importance:  storyline.Importance,
			Scope:       storyline.Scope,
			Description: clipToolString(storyline.Description, 160),
		})
	}
	return out
}

func matchingCoreCastBriefs(values []models.CoreCastSeed, name string, limit int) []coreCastBrief {
	out := make([]coreCastBrief, 0)
	for _, cast := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !textMatches(cast.Name, name) && !objectJSONContains(cast, name) {
			continue
		}
		out = append(out, coreCastBrief{
			ID:                 cast.ID,
			Name:               cast.Name,
			Role:               cast.Role,
			Importance:         cast.Importance,
			StoryFunction:      clipToolString(cast.StoryFunction, 180),
			RelationshipToLead: clipToolString(cast.RelationshipToLead, 120),
			RelationshipArc:    clipToolString(cast.RelationshipArc, 140),
			EntryPhase:         cast.EntryPhase,
			Payoff:             clipToolString(cast.Payoff, 140),
			StorylineRefs:      cast.StorylineRefs,
		})
	}
	return out
}

func relevantStorylineBriefs(values []models.Storyline, cast []coreCastBrief, limit int) []storylineBrief {
	refs := map[string]bool{}
	for _, item := range cast {
		for _, ref := range item.StorylineRefs {
			refs[normalizeSearch(ref)] = true
		}
	}
	out := make([]storylineBrief, 0)
	if len(refs) > 0 {
		for _, storyline := range values {
			if len(out) >= limit {
				break
			}
			if !refs[normalizeSearch(storyline.Name)] {
				continue
			}
			out = append(out, storylineBrief{
				Name:               storyline.Name,
				Type:               storyline.Type,
				Importance:         storyline.Importance,
				Scope:              storyline.Scope,
				Description:        clipToolString(storyline.Description, 160),
				SetupRole:          clipToolString(storyline.SetupRole, 120),
				RepeatablePressure: clipToolString(storyline.RepeatablePressure, 100),
				PayoffCadence:      clipToolString(storyline.PayoffCadence, 100),
				FailureMode:        clipToolString(storyline.FailureMode, 100),
				OpenQuestion:       clipToolString(storyline.OpenQuestion, 100),
			})
		}
	}
	if len(out) > 0 || len(refs) > 0 {
		return out
	}
	for _, storyline := range values {
		if len(out) >= limit {
			break
		}
		out = append(out, storylineBrief{Name: storyline.Name, Type: storyline.Type, Importance: storyline.Importance, Scope: storyline.Scope})
	}
	return out
}

func matchingPremiseBriefs(values []models.Premise, name string, limit int) []premiseBrief {
	out := make([]premiseBrief, 0)
	for _, premise := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !objectJSONContains(premise, name) {
			continue
		}
		out = append(out, premiseBrief{
			Name:     premise.Name,
			Category: premise.Category,
		})
	}
	return out
}

func matchingWorldResourceBriefs(values []models.WorldResource, name string, limit int) []worldResourceBrief {
	out := make([]worldResourceBrief, 0)
	for _, resource := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !objectJSONContains(resource, name) {
			continue
		}
		out = append(out, worldResourceBrief{
			Name:     resource.Name,
			Category: resource.Category,
			Scarcity: resource.Scarcity,
		})
	}
	return out
}
