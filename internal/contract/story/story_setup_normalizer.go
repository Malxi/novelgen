package models

import (
	"fmt"
	"sort"
	"strings"
)

// StorySetupNormalizationReport records deterministic cleanups applied to setup.
type StorySetupNormalizationReport struct {
	Changes []StorySetupNormalizationChange `json:"changes"`
}

// StorySetupNormalizationChange describes one deterministic setup cleanup.
type StorySetupNormalizationChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Value  string `json:"value"`
}

// Changed reports whether normalization modified the setup.
func (r StorySetupNormalizationReport) Changed() bool {
	return len(r.Changes) > 0
}

// NormalizeStorySetup applies deterministic, schema-preserving cleanup rules to
// keep setup state stable across generation, review, and save operations.
func NormalizeStorySetup(setup *StorySetup) StorySetupNormalizationReport {
	report := StorySetupNormalizationReport{Changes: []StorySetupNormalizationChange{}}
	if setup == nil {
		return report
	}

	normalizeSetupString(&setup.ProjectName, "project_name", &report)
	normalizeSetupString(&setup.Premise, "premise", &report)
	normalizeSetupString(&setup.Theme, "theme", &report)
	normalizeSetupString(&setup.TargetAudience, "target_audience", &report)
	normalizeSetupString(&setup.Tone, "tone", &report)
	normalizeSetupString(&setup.Tense, "tense", &report)
	normalizeSetupString(&setup.POVStyle, "pov_style", &report)

	setup.Genres = normalizeSetupStrings(setup.Genres, "genres", &report)
	setup.Rules = normalizeSetupStrings(setup.Rules, "rules", &report)
	setup.WorldTimeline = normalizeSetupTimeline(setup.WorldTimeline, &report)
	setup.WorldResources = normalizeSetupResources(setup.WorldResources, &report)

	if !setup.WritingStyle.IsZero() {
		setup.WritingStyle = setup.WritingStyle.CompactReference(len([]rune(strings.TrimSpace(setup.WritingStyle.ReferenceExcerpt))))
	}

	setup.LongFormPlan = normalizeLongFormPlan(setup.LongFormPlan, &report)
	setup.CoreCast = normalizeCoreCast(setup.CoreCast, &report)
	setup.Storylines = normalizeStorylines(setup.Storylines, &report)
	setup.Premises = normalizePremises(setup.Premises, &report)

	return report
}

func normalizeSetupString(value *string, path string, report *StorySetupNormalizationReport) {
	if value == nil {
		return
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == *value {
		return
	}
	*value = trimmed
	report.Changes = append(report.Changes, StorySetupNormalizationChange{
		Path:   path,
		Action: "trim_string",
		Value:  trimmed,
	})
}

func normalizeSetupStrings(values []string, path string, report *StorySetupNormalizationReport) []string {
	cleaned := compactUniqueStrings(values)
	if !stringSlicesEqual(values, cleaned) {
		report.Changes = append(report.Changes, StorySetupNormalizationChange{
			Path:   path,
			Action: "compact_string_list",
			Value:  fmt.Sprintf("%d -> %d item(s)", len(values), len(cleaned)),
		})
	}
	return cleaned
}

func normalizeSetupTimeline(entries []WorldTimelineEntry, report *StorySetupNormalizationReport) []WorldTimelineEntry {
	cleaned := make([]WorldTimelineEntry, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		before := entry
		entry.Year = strings.TrimSpace(entry.Year)
		entry.Event = strings.TrimSpace(entry.Event)
		entry.Impact = strings.TrimSpace(entry.Impact)
		entry.RelatedMystery = strings.TrimSpace(entry.RelatedMystery)
		if entry.Year == "" && entry.Event == "" && entry.Impact == "" && entry.RelatedMystery == "" {
			continue
		}
		if entry != before {
			report.Changes = append(report.Changes, StorySetupNormalizationChange{
				Path:   fmt.Sprintf("world_timeline[%d]", i),
				Action: "trim_timeline_entry",
				Value:  compactSummary(entry.Year, entry.Event),
			})
		}
		cleaned = append(cleaned, entry)
	}
	return cleaned
}

func normalizeSetupResources(entries []WorldResource, report *StorySetupNormalizationReport) []WorldResource {
	cleaned := make([]WorldResource, 0, len(entries))
	for i := range entries {
		resource := entries[i]
		before := resource
		resource.Name = strings.TrimSpace(resource.Name)
		resource.Category = strings.TrimSpace(resource.Category)
		resource.Scarcity = strings.TrimSpace(resource.Scarcity)
		resource.Description = strings.TrimSpace(resource.Description)
		if resource.Name == "" && resource.Category == "" && resource.Scarcity == "" && resource.Description == "" {
			continue
		}
		if resource != before {
			report.Changes = append(report.Changes, StorySetupNormalizationChange{
				Path:   fmt.Sprintf("world_resources[%d]", i),
				Action: "trim_resource_entry",
				Value:  resource.Name,
			})
		}
		cleaned = append(cleaned, resource)
	}
	return cleaned
}

func normalizeLongFormPlan(plan *LongFormPlan, report *StorySetupNormalizationReport) *LongFormPlan {
	if plan == nil {
		return nil
	}
	before := *plan
	beforeEscalation := append([]string(nil), plan.EscalationLadder...)
	beforePromises := append([]string(nil), plan.ReaderPromises...)
	beforePattern := append([]string(nil), plan.VolumePattern...)

	plan.MainLoop = strings.TrimSpace(plan.MainLoop)
	plan.EscalationLadder = normalizeSetupStrings(plan.EscalationLadder, "long_form_plan.escalation_ladder", report)
	plan.ReaderPromises = normalizeSetupStrings(plan.ReaderPromises, "long_form_plan.reader_promises", report)
	plan.PayoffCadence = strings.TrimSpace(plan.PayoffCadence)
	plan.VolumePattern = normalizeSetupStrings(plan.VolumePattern, "long_form_plan.volume_pattern", report)
	plan.MidpointMutation = strings.TrimSpace(plan.MidpointMutation)
	plan.EndgamePromise = strings.TrimSpace(plan.EndgamePromise)

	if plan.IsZero() {
		report.Changes = append(report.Changes, StorySetupNormalizationChange{
			Path:   "long_form_plan",
			Action: "drop_empty_long_form_plan",
			Value:  "",
		})
		return nil
	}

	if plan.TargetChapters != before.TargetChapters ||
		plan.TargetVolumes != before.TargetVolumes ||
		plan.MainLoop != before.MainLoop ||
		plan.PayoffCadence != before.PayoffCadence ||
		plan.MidpointMutation != before.MidpointMutation ||
		plan.EndgamePromise != before.EndgamePromise ||
		!stringSlicesEqual(beforeEscalation, plan.EscalationLadder) ||
		!stringSlicesEqual(beforePromises, plan.ReaderPromises) ||
		!stringSlicesEqual(beforePattern, plan.VolumePattern) {
		report.Changes = append(report.Changes, StorySetupNormalizationChange{
			Path:   "long_form_plan",
			Action: "trim_long_form_plan",
			Value:  compactSummary(plan.MainLoop, plan.PayoffCadence),
		})
	}
	return plan
}

func normalizeCoreCast(entries []CoreCastSeed, report *StorySetupNormalizationReport) []CoreCastSeed {
	cleaned := make([]CoreCastSeed, 0, len(entries))
	for i := range entries {
		seed := entries[i]
		beforeRefs := append([]string(nil), seed.StorylineRefs...)
		seed.ID = strings.TrimSpace(seed.ID)
		seed.Name = strings.TrimSpace(seed.Name)
		seed.Role = strings.TrimSpace(seed.Role)
		seed.StoryFunction = strings.TrimSpace(seed.StoryFunction)
		seed.RelationshipToLead = strings.TrimSpace(seed.RelationshipToLead)
		seed.RelationshipArc = strings.TrimSpace(seed.RelationshipArc)
		seed.EntryPhase = strings.TrimSpace(seed.EntryPhase)
		seed.Payoff = strings.TrimSpace(seed.Payoff)
		seed.StorylineRefs = normalizeSetupStrings(seed.StorylineRefs, fmt.Sprintf("core_cast[%d].storyline_refs", i), report)

		changed := seed.ID != entries[i].ID ||
			seed.Name != entries[i].Name ||
			seed.Role != entries[i].Role ||
			seed.StoryFunction != entries[i].StoryFunction ||
			seed.RelationshipToLead != entries[i].RelationshipToLead ||
			seed.RelationshipArc != entries[i].RelationshipArc ||
			seed.EntryPhase != entries[i].EntryPhase ||
			seed.Payoff != entries[i].Payoff ||
			!stringSlicesEqual(beforeRefs, seed.StorylineRefs)

		if seed.ID == "" && seed.Name == "" && seed.Role == "" && seed.Importance == 0 &&
			seed.StoryFunction == "" && seed.RelationshipToLead == "" && seed.RelationshipArc == "" &&
			seed.EntryPhase == "" && seed.Payoff == "" && len(seed.StorylineRefs) == 0 {
			continue
		}
		if changed {
			report.Changes = append(report.Changes, StorySetupNormalizationChange{
				Path:   fmt.Sprintf("core_cast[%d]", i),
				Action: "trim_core_cast_seed",
				Value:  seed.Name,
			})
		}
		cleaned = append(cleaned, seed)
	}
	return cleaned
}

func normalizeStorylines(entries []Storyline, report *StorySetupNormalizationReport) []Storyline {
	cleaned := make([]Storyline, 0, len(entries))
	for i := range entries {
		item := entries[i]
		changed := false
		beforePressure := append([]string(nil), item.PressurePoints...)
		beforeAppeal := item.AppealEngine
		item.Name = strings.TrimSpace(item.Name)
		item.Description = strings.TrimSpace(item.Description)
		item.Type = strings.TrimSpace(item.Type)
		item.Scope = strings.TrimSpace(item.Scope)
		item.PayoffStyle = strings.TrimSpace(item.PayoffStyle)
		item.SetupRole = strings.TrimSpace(item.SetupRole)
		item.RepeatablePressure = strings.TrimSpace(item.RepeatablePressure)
		item.PayoffCadence = strings.TrimSpace(item.PayoffCadence)
		item.Mutation = strings.TrimSpace(item.Mutation)
		item.FailureMode = strings.TrimSpace(item.FailureMode)
		item.Desire = strings.TrimSpace(item.Desire)
		item.Opposition = strings.TrimSpace(item.Opposition)
		item.Stakes = strings.TrimSpace(item.Stakes)
		item.Turn = strings.TrimSpace(item.Turn)
		item.Payoff = strings.TrimSpace(item.Payoff)
		item.OpenQuestion = strings.TrimSpace(item.OpenQuestion)
		item.PressurePoints = normalizeSetupStrings(item.PressurePoints, fmt.Sprintf("storylines[%d].pressure_points", i), report)
		item.AppealEngine = normalizeAppealEngine(item.AppealEngine, fmt.Sprintf("storylines[%d].appeal_engine", i), report)
		if item.Name != entries[i].Name || item.Description != entries[i].Description || item.Type != entries[i].Type ||
			item.Scope != entries[i].Scope || item.PayoffStyle != entries[i].PayoffStyle || item.SetupRole != entries[i].SetupRole ||
			item.RepeatablePressure != entries[i].RepeatablePressure || item.PayoffCadence != entries[i].PayoffCadence ||
			item.Mutation != entries[i].Mutation || item.FailureMode != entries[i].FailureMode ||
			item.Desire != entries[i].Desire || item.Opposition != entries[i].Opposition || item.Stakes != entries[i].Stakes ||
			item.Turn != entries[i].Turn || item.Payoff != entries[i].Payoff || item.OpenQuestion != entries[i].OpenQuestion {
			changed = true
		}
		if !stringSlicesEqual(beforePressure, item.PressurePoints) {
			changed = true
		}
		if !appealEnginesEqual(beforeAppeal, item.AppealEngine) {
			changed = true
		}
		if item.Name == "" && item.Description == "" && item.Type == "" && item.Importance == 0 &&
			item.Scope == "" && item.PayoffStyle == "" && item.SetupRole == "" &&
			item.RepeatablePressure == "" && item.PayoffCadence == "" && item.Mutation == "" && item.FailureMode == "" &&
			item.Desire == "" && item.Opposition == "" && item.Stakes == "" && item.Turn == "" &&
			item.Payoff == "" && item.OpenQuestion == "" && len(item.PressurePoints) == 0 && item.AppealEngine == nil {
			continue
		}
		if changed {
			report.Changes = append(report.Changes, StorySetupNormalizationChange{
				Path:   fmt.Sprintf("storylines[%d]", i),
				Action: "trim_storyline",
				Value:  item.Name,
			})
		}
		cleaned = append(cleaned, item)
	}
	return cleaned
}

func normalizePremises(entries []Premise, report *StorySetupNormalizationReport) []Premise {
	cleaned := make([]Premise, 0, len(entries))
	for i := range entries {
		item := entries[i]
		changed := false
		beforeProgression := append([]ProgressionStage(nil), item.Progression...)
		beforeAppeal := item.AppealEngine
		item.Name = strings.TrimSpace(item.Name)
		item.Description = strings.TrimSpace(item.Description)
		item.Category = strings.TrimSpace(item.Category)
		item.AppealEngine = normalizeAppealEngine(item.AppealEngine, fmt.Sprintf("premises[%d].appeal_engine", i), report)
		item.Progression = normalizeProgression(item.Progression, fmt.Sprintf("premises[%d].progression", i), report)
		if item.Name != entries[i].Name || item.Description != entries[i].Description || item.Category != entries[i].Category {
			changed = true
		}
		if !progressionEqual(beforeProgression, item.Progression) {
			changed = true
		}
		if !appealEnginesEqual(beforeAppeal, item.AppealEngine) {
			changed = true
		}
		if item.Name == "" && item.Description == "" && item.Category == "" &&
			len(item.Progression) == 0 && item.AppealEngine == nil {
			continue
		}
		if changed {
			report.Changes = append(report.Changes, StorySetupNormalizationChange{
				Path:   fmt.Sprintf("premises[%d]", i),
				Action: "trim_premise",
				Value:  item.Name,
			})
		}
		cleaned = append(cleaned, item)
	}
	return cleaned
}

func normalizeProgression(entries []ProgressionStage, path string, report *StorySetupNormalizationReport) []ProgressionStage {
	cleaned := make([]ProgressionStage, 0, len(entries))
	for i := range entries {
		stage := entries[i]
		before := stage
		stage.Name = strings.TrimSpace(stage.Name)
		stage.Description = strings.TrimSpace(stage.Description)
		stage.Requirements = strings.TrimSpace(stage.Requirements)
		if stage.Level == 0 && stage.Name == "" && stage.Description == "" && stage.Requirements == "" {
			continue
		}
		if stage != before {
			report.Changes = append(report.Changes, StorySetupNormalizationChange{
				Path:   fmt.Sprintf("%s[%d]", path, i),
				Action: "trim_progression_stage",
				Value:  stage.Name,
			})
		}
		cleaned = append(cleaned, stage)
	}

	if len(cleaned) < 2 {
		return cleaned
	}

	sorted := append([]ProgressionStage(nil), cleaned...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Level == sorted[j].Level {
			return i < j
		}
		if sorted[i].Level <= 0 {
			return false
		}
		if sorted[j].Level <= 0 {
			return true
		}
		return sorted[i].Level < sorted[j].Level
	})
	if !progressionEqual(cleaned, sorted) {
		report.Changes = append(report.Changes, StorySetupNormalizationChange{
			Path:   path,
			Action: "sort_progression_by_level",
			Value:  fmt.Sprintf("%d stage(s)", len(sorted)),
		})
	}
	return sorted
}

func normalizeAppealEngine(engine *AppealEngine, path string, report *StorySetupNormalizationReport) *AppealEngine {
	if engine == nil {
		return nil
	}
	before := *engine
	engine.Appeal = strings.TrimSpace(engine.Appeal)
	engine.SurfaceLimit = strings.TrimSpace(engine.SurfaceLimit)
	engine.Exploit = strings.TrimSpace(engine.Exploit)
	engine.SignatureWin = strings.TrimSpace(engine.SignatureWin)
	engine.UpgradePath = strings.TrimSpace(engine.UpgradePath)
	engine.OpponentMisread = strings.TrimSpace(engine.OpponentMisread)
	engine.RewardType = strings.TrimSpace(engine.RewardType)

	if engine.Appeal == "" && engine.SurfaceLimit == "" && engine.Exploit == "" && engine.SignatureWin == "" &&
		engine.UpgradePath == "" && engine.OpponentMisread == "" && engine.RewardType == "" {
		report.Changes = append(report.Changes, StorySetupNormalizationChange{
			Path:   path,
			Action: "drop_empty_appeal_engine",
			Value:  "",
		})
		return nil
	}
	if *engine != before {
		report.Changes = append(report.Changes, StorySetupNormalizationChange{
			Path:   path,
			Action: "trim_appeal_engine",
			Value:  engine.Appeal,
		})
	}
	return engine
}

func compactUniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != b[i] {
			return false
		}
	}
	return true
}

func progressionEqual(a, b []ProgressionStage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func appealEnginesEqual(a, b *AppealEngine) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Appeal == b.Appeal &&
		a.SurfaceLimit == b.SurfaceLimit &&
		a.Exploit == b.Exploit &&
		a.SignatureWin == b.SignatureWin &&
		a.UpgradePath == b.UpgradePath &&
		a.OpponentMisread == b.OpponentMisread &&
		a.RewardType == b.RewardType
}

func compactSummary(values ...string) string {
	var parts []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " | ")
}
