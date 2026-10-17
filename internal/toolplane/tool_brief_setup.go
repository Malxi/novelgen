// Story-setup briefs: the setup index and slices, the long-form plan and progression stages.

package toolplane

import (
	"novelgen/internal/contract/story"
)

func briefStorySetupResults(results interface{}, indexOnly bool) interface{} {
	setup, ok := results.(*models.StorySetup)
	if !ok || setup == nil {
		return results
	}
	brief := StorySetupBrief{
		ProjectName:    setup.ProjectName,
		Genres:         setup.Genres,
		TargetAudience: clipToolString(setup.TargetAudience, 160),
		Tone:           clipToolString(setup.Tone, 160),
		POVStyle:       setup.POVStyle,
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
	if !indexOnly {
		brief.Premise = clipToolString(setup.Premise, 900)
		brief.Theme = clipToolString(setup.Theme, 500)
		brief.Rules = clipToolStrings(setup.Rules, 8, 220)
		brief.LongFormPlan = makeLongFormPlanBrief(setup.LongFormPlan, false)
	} else {
		brief.LongFormPlan = makeLongFormPlanBrief(setup.LongFormPlan, true)
	}
	for _, cast := range setup.CoreCast {
		item := coreCastBrief{ID: cast.ID, Name: cast.Name, Role: cast.Role, Importance: cast.Importance, EntryPhase: cast.EntryPhase, StorylineRefs: cast.StorylineRefs}
		if !indexOnly {
			item.StoryFunction = clipToolString(cast.StoryFunction, 220)
			item.RelationshipToLead = clipToolString(cast.RelationshipToLead, 160)
			item.RelationshipArc = clipToolString(cast.RelationshipArc, 180)
			item.Payoff = clipToolString(cast.Payoff, 180)
		}
		brief.CoreCast = append(brief.CoreCast, item)
	}
	for _, storyline := range setup.Storylines {
		item := storylineBrief{Name: storyline.Name, Type: storyline.Type, Importance: storyline.Importance, Scope: storyline.Scope}
		if !indexOnly {
			item.Description = clipToolString(storyline.Description, 260)
			item.SetupRole = clipToolString(storyline.SetupRole, 180)
			item.RepeatablePressure = clipToolString(storyline.RepeatablePressure, 160)
			item.PayoffCadence = clipToolString(storyline.PayoffCadence, 140)
			item.FailureMode = clipToolString(storyline.FailureMode, 160)
			item.OpenQuestion = clipToolString(storyline.OpenQuestion, 160)
			item.AppealEngine = storyline.AppealEngine
		}
		brief.Storylines = append(brief.Storylines, item)
	}
	for _, premise := range setup.Premises {
		item := premiseBrief{Name: premise.Name, Category: premise.Category}
		if !indexOnly {
			item.Description = clipToolString(premise.Description, 260)
			item.Progression = makeProgressionBrief(premise.Progression)
		}
		brief.Premises = append(brief.Premises, item)
	}
	for _, entry := range setup.WorldTimeline {
		item := worldTimelineBrief{Year: entry.Year, Event: clipToolString(entry.Event, 180)}
		if !indexOnly {
			item.Impact = clipToolString(entry.Impact, 180)
			item.RelatedMystery = entry.RelatedMystery
		}
		brief.WorldTimeline = append(brief.WorldTimeline, item)
	}
	for _, resource := range setup.WorldResources {
		item := worldResourceBrief{Name: resource.Name, Category: resource.Category, Scarcity: resource.Scarcity}
		if !indexOnly {
			item.Description = clipToolString(resource.Description, 180)
		}
		brief.WorldResources = append(brief.WorldResources, item)
	}
	return brief
}

func makeLongFormPlanBrief(plan *models.LongFormPlan, indexOnly bool) interface{} {
	if plan == nil || plan.IsZero() {
		return nil
	}
	brief := longFormPlanBrief{
		TargetChapters: plan.TargetChapters,
		TargetVolumes:  plan.TargetVolumes,
		MainLoop:       clipToolString(plan.MainLoop, 180),
	}
	if indexOnly {
		return brief
	}
	brief.EscalationLadder = clipToolStrings(plan.EscalationLadder, 8, 120)
	brief.ReaderPromises = clipToolStrings(plan.ReaderPromises, 8, 120)
	brief.PayoffCadence = clipToolString(plan.PayoffCadence, 180)
	brief.VolumePattern = clipToolStrings(plan.VolumePattern, 10, 100)
	brief.MidpointMutation = clipToolString(plan.MidpointMutation, 180)
	brief.EndgamePromise = clipToolString(plan.EndgamePromise, 180)
	return brief
}

func makeProgressionBrief(stages []models.ProgressionStage) []progressionStageBrief {
	if len(stages) == 0 {
		return nil
	}
	out := make([]progressionStageBrief, 0, len(stages))
	for _, stage := range stages {
		out = append(out, progressionStageBrief{
			Level: stage.Level,
			Name:  clipToolString(stage.Name, 80),
		})
	}
	return out
}
