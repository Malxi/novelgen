package dsl

import (
	"fmt"
	"sort"
	"strings"

	"novelgen/internal/models"
)

// ModelAdapter builds DSL AST structs from models.* types used by CLI commands.
// All constructor parameters are optional — nil values produce a minimal DSL skeleton
// that the simulator can still run against (producing info-level issues about what's missing).
type ModelAdapter struct {
	setup         *models.StorySetup
	outline       *models.Outline
	characters    map[string]*models.Character
	locations     map[string]*models.Location
	items         map[string]*models.Item
	organizations map[string]*models.Organization
}

// NewModelAdapter creates a new ModelAdapter. All parameters are optional.
func NewModelAdapter(
	setup *models.StorySetup,
	outline *models.Outline,
	characters map[string]*models.Character,
	locations map[string]*models.Location,
	items map[string]*models.Item,
) *ModelAdapter {
	return &ModelAdapter{
		setup:      setup,
		outline:    outline,
		characters: characters,
		locations:  locations,
		items:      items,
	}
}

// NewModelAdapterWithOrganizations creates a ModelAdapter that also carries
// craft organization data. Existing callers can keep using NewModelAdapter.
func NewModelAdapterWithOrganizations(
	setup *models.StorySetup,
	outline *models.Outline,
	characters map[string]*models.Character,
	locations map[string]*models.Location,
	items map[string]*models.Item,
	organizations map[string]*models.Organization,
) *ModelAdapter {
	adapter := NewModelAdapter(setup, outline, characters, locations, items)
	adapter.organizations = organizations
	return adapter
}

// BuildDSL constructs a DSL AST populated according to phase.
func (a *ModelAdapter) BuildDSL(phase MergePhase) (*DSL, error) {
	dsl := &DSL{
		Metadata:   &Metadata{},
		World:      &World{},
		Characters: &Characters{},
		Storyline:  &Storyline{},
		Systems:    &Systems{},
	}

	dsl.Metadata.DSLVersion = "0.2.0"
	dsl.Metadata.Phase = string(phase)

	// Always build base metadata + player
	a.buildMetadata(dsl)
	a.buildDefaultPlayer(dsl)
	a.buildSetupWorld(dsl)
	a.buildDefaultSystems(dsl)
	a.buildStorylineContracts(dsl)

	switch phase {
	case PhaseSetup:
		a.buildSetupContractChapters(dsl)
		return dsl, nil
	case PhaseOutline:
		a.buildChaptersFromOutline(dsl)
		a.buildPlaceholderNPCs(dsl)
		a.buildPlaceholderLocations(dsl)
		return dsl, nil
	case PhaseCraft:
		a.buildChaptersFromOutline(dsl)
		a.buildCharacters(dsl)
		a.buildLocations(dsl)
		a.buildItems(dsl)
		a.buildOrganizations(dsl)
		return dsl, nil
	default:
		return dsl, nil
	}
}

// Simulate builds DSL for the given phase and runs the simulator.
func (a *ModelAdapter) Simulate(phase MergePhase) ([]SimulationIssue, error) {
	dslData, err := a.BuildDSL(phase)
	if err != nil {
		return nil, err
	}
	sim := NewSimulator(dslData)
	sim.SimulateAll()
	return sim.Issues, nil
}

func (a *ModelAdapter) buildMetadata(dsl *DSL) {
	if a.setup != nil {
		if strings.TrimSpace(a.setup.ProjectName) != "" {
			dsl.Metadata.Title = a.setup.ProjectName
		}
		dsl.Metadata.Genre = append([]string(nil), a.setup.Genres...)
		dsl.Metadata.Tone = a.setup.Tone
		dsl.Metadata.PowerSystem = a.inferPowerSystem()
	} else {
		dsl.Metadata.Title = "untitled"
		dsl.Metadata.PowerSystem = "default_progression_system"
	}
}

func (a *ModelAdapter) buildDefaultPlayer(dsl *DSL) {
	playerName := "主角"
	playerID := "protagonist"
	playerStats := Stats{STR: 10, AGI: 10, INT: 10, VIT: 10, HP: 100, MP: 50}

	if a.setup != nil && a.setup.ProjectName != "" {
		playerID = sanitizeID(a.setup.ProjectName + "_protagonist")
	}

	// If craft characters are available, find the protagonist among them
	if a.characters != nil {
		for _, id := range sortedCharacterKeys(a.characters) {
			ch := a.characters[id]
			if ch == nil {
				continue
			}
			if ch.RoleInStory == "protagonist" || ch.RoleInStory == "主角" {
				playerName = ch.Name
				playerID = id
				playerStats = statsFromCraft(ch.RPGStats, playerStats)
				break
			}
		}
	}

	dsl.Characters.Player = &Player{
		ID:          playerID,
		Name:        playerName,
		Stats:       playerStats,
		Description: "主角",
		RoleInStory: "protagonist",
	}
}

func (a *ModelAdapter) buildDefaultSystems(dsl *DSL) {
	dsl.Systems.Progression = &Progression{
		Type: "level",
		Formula: ProgressionFormula{
			ExpToNext:    "level * 100",
			ExpFromEnemy: "enemy_level * 10",
			ExpFromQuest: "quest_level * 50",
		},
		LevelUp: LevelUpRewards{
			StatPoints:  5,
			SkillPoints: 1,
			HPRestore:   "full",
			MPRestore:   "full",
		},
	}

	// Build custom attribute system from setup premises
	if a.setup != nil && len(a.setup.Premises) > 0 {
		var attrs []AttributeDef
		for _, premise := range a.setup.Premises {
			attrs = append(attrs, AttributeDef{
				ID:          sanitizeID(premise.Name),
				Name:        premise.Name,
				Description: premise.Description,
				Type:        "resource",
				BaseValue:   100,
				IsResource:  true,
			})
		}
		if len(attrs) > 0 {
			attrs = appendResourceAttributes(attrs, a.setup)
			dsl.Systems.AttributeSystem = &AttributeSystem{
				ID:         "custom_attrs",
				Name:       "Custom Attributes",
				Attributes: attrs,
			}
		}
	} else if a.setup != nil && len(a.setup.WorldResources) > 0 {
		dsl.Systems.AttributeSystem = &AttributeSystem{
			ID:         "custom_attrs",
			Name:       "Custom Attributes",
			Attributes: appendResourceAttributes(nil, a.setup),
		}

		// Build progression systems
	}

	if a.setup != nil {
		for _, premise := range a.setup.Premises {
			if len(premise.Progression) > 0 {
				var levels []ProgressionLevel
				for _, stage := range premise.Progression {
					levels = append(levels, ProgressionLevel{
						Level:        stage.Level,
						Name:         stage.Name,
						Requirements: stage.Requirements,
						Bonuses:      []string{stage.Description},
					})
				}
				dsl.Systems.ProgressionSystems = append(dsl.Systems.ProgressionSystems, ProgressionSystem{
					ID:          sanitizeID(premise.Name),
					Name:        premise.Name,
					Description: premise.Description,
					Levels:      levels,
				})
			}
		}
	}

	dsl.Systems.PowerFormula = &PowerFormula{
		ID:        "default",
		Name:      "Default Power Formula",
		Formula:   "str*2 + agi*1 + int*2 + vit*1 + hp/10",
		BasePower: 10,
		Factors: []Factor{
			{Attribute: "str", Name: "力量", Weight: 2},
			{Attribute: "agi", Name: "敏捷", Weight: 1},
			{Attribute: "int", Name: "智力", Weight: 2},
			{Attribute: "vit", Name: "耐力", Weight: 1},
		},
	}
}

func (a *ModelAdapter) buildSetupWorld(dsl *DSL) {
	if a.setup == nil || dsl.World == nil {
		return
	}
	dsl.World.Rules = buildRulesFromSetup(a.setup)
	dsl.World.Items = append(dsl.World.Items, buildWorldResourceItems(a.setup)...)
}

func (a *ModelAdapter) buildChaptersFromOutline(dsl *DSL) {
	if a.outline == nil {
		return
	}

	pos := 0
	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			if vol.ID != "" {
				dsl.Storyline.Arcs = append(dsl.Storyline.Arcs, Arc{
					ID:       vol.ID,
					Name:     vol.Title,
					Position: len(dsl.Storyline.Arcs) + 1,
				})
			}
			for _, ch := range vol.Chapters {
				pos++
				chapter := Chapter{
					ID:       ch.ID,
					Title:    ch.Title,
					Arc:      vol.ID,
					Position: pos,
				}

				// Build objectives from chapter events and beats
				var steps []Step
				stepOrder := 0

				for _, advance := range ch.StorylineAdvances {
					stepOrder++
					steps = append(steps, Step{
						Order:       stepOrder,
						Description: storylineAdvanceDescription(advance),
						Event: Event{
							Type:        "storyline",
							StateDeltas: buildStorylineAdvanceDeltas(ch.ID, advance),
						},
					})
				}

				for _, step := range buildMysterySteps(ch.ID, ch.Mysteries) {
					stepOrder++
					step.Order = stepOrder
					steps = append(steps, step)
				}

				for _, delta := range buildStateAnchorDeltas(ch) {
					stepOrder++
					steps = append(steps, Step{
						Order:       stepOrder,
						Description: delta.Note,
						Event: Event{
							Type:        "status",
							StateDeltas: []StateDelta{delta},
						},
					})
				}

				a.addOutlineEnemies(dsl, ch.Enemies)
				for _, evt := range ch.Events {
					stepOrder++
					step := Step{
						Order:       stepOrder,
						Description: describeModelEvent(evt),
					}
					step.Event = a.buildEventFromModel(evt, ch.Enemies)
					steps = append(steps, step)
				}

				for _, entry := range ch.ResourceLedger {
					stepOrder++
					steps = append(steps, Step{
						Order:       stepOrder,
						Description: fmt.Sprintf("%s resource change: %d %+d = %d. %s", entry.Item, entry.Start, entry.Delta, entry.End, entry.Reason),
						Event: Event{
							Type: "resource",
							StateDeltas: []StateDelta{{
								Target: entry.Item,
								Kind:   "resource",
								Field:  "quantity",
								From:   fmt.Sprintf("%d", entry.Start),
								To:     fmt.Sprintf("%d", entry.End),
								Delta:  entry.Delta,
								Note:   entry.Reason,
							}},
						},
					})
				}

				if ch.Timeline.TimeJump || strings.TrimSpace(ch.Timeline.Transition) != "" {
					stepOrder++
					steps = append(steps, Step{
						Order:       stepOrder,
						Description: strings.TrimSpace(ch.Timeline.Transition),
						Event: Event{
							Type: "transition",
							StateDeltas: []StateDelta{{
								Kind:  "transition",
								Field: "time_jump",
								To:    "true",
								Note:  strings.TrimSpace(ch.Timeline.PreviousGap + " " + ch.Timeline.Transition),
							}},
						},
					})
				}

				for _, beat := range ch.GetBeats() {
					stepOrder++
					step := Step{
						Order:       stepOrder,
						Description: beat,
					}
					steps = append(steps, step)
				}

				if len(steps) > 0 {
					chapter.Objectives = []Objective{{
						ID:    ch.ID + "-obj",
						Name:  ch.Title,
						Type:  "sequence",
						Steps: steps,
					}}
				}

				dsl.Storyline.Chapters = append(dsl.Storyline.Chapters, chapter)
			}
		}
	}
}

func (a *ModelAdapter) buildEventFromModel(evt models.Event, enemies []models.OutlineEnemy) Event {
	dslEvent := Event{
		Type:        normalizeOutlineEventType(evt.Type, evt.Action, evt.TargetType),
		StateDeltas: buildEventStateDeltas(evt),
	}
	if dslEvent.Type == "" {
		dslEvent.Type = eventTypeFromAction(evt.Action)
	}

	switch evt.Action {
	case models.ActionCombat:
		dslEvent.Type = "combat"
		dslEvent.Combat = &CombatEvent{
			Setup: CombatSetup{
				Location: evt.Context,
				Enemies:  outlineEnemySpawns(enemies),
			},
		}
	case models.ActionMove:
		dslEvent.Type = "move"
	case models.ActionAcquire:
		dslEvent.Type = "acquire"
		dslEvent.Acquire = &AcquireEvent{
			Actor: evt.Actor,
			Item:  evt.Target,
		}
	case models.ActionMeet:
		dslEvent.Type = "dialogue"
	}
	if dslEvent.Type == "combat" && dslEvent.Combat == nil {
		dslEvent.Combat = &CombatEvent{
			Setup: CombatSetup{
				Location: evt.Context,
				Enemies:  outlineEnemySpawns(enemies),
			},
		}
	}

	return dslEvent
}

func (a *ModelAdapter) addOutlineEnemies(dsl *DSL, enemies []models.OutlineEnemy) {
	if dsl == nil || dsl.Characters == nil {
		return
	}
	seen := make(map[string]bool, len(dsl.Characters.Enemies))
	for _, enemy := range dsl.Characters.Enemies {
		seen[enemy.ID] = true
	}
	for i, enemy := range enemies {
		name := strings.TrimSpace(enemy.Name)
		if name == "" {
			continue
		}
		id := outlineEnemyID(enemy, i)
		if seen[id] {
			continue
		}
		seen[id] = true
		level := enemy.Level
		if level <= 0 {
			level = 1
		}
		dsl.Characters.Enemies = append(dsl.Characters.Enemies, Enemy{
			ID:          id,
			Name:        name,
			Type:        coalesceString(enemy.Faction, "enemy"),
			Description: strings.TrimSpace(enemy.Context),
			Level:       level,
			Template: EnemyTemplate{
				BaseLevel: level,
				HPFormula: fmt.Sprintf("%d", 40+level*20),
				StatsPerLevel: map[string]int{
					"str": 6 + level,
					"agi": 4 + level,
					"int": 3 + level,
					"vit": 5 + level,
				},
			},
		})
	}
}

func outlineEnemySpawns(enemies []models.OutlineEnemy) []EnemySpawn {
	spawns := make([]EnemySpawn, 0, len(enemies))
	for i, enemy := range enemies {
		if strings.TrimSpace(enemy.Name) == "" {
			continue
		}
		count := enemy.Count
		if count <= 0 {
			count = 1
		}
		spawns = append(spawns, EnemySpawn{
			ID:    outlineEnemyID(enemy, i),
			Count: count,
			Level: enemy.Level,
			Boss:  enemy.IsBoss,
		})
	}
	return spawns
}

func outlineEnemyID(enemy models.OutlineEnemy, index int) string {
	if strings.TrimSpace(enemy.BossID) != "" {
		return sanitizeID(enemy.BossID)
	}
	base := strings.TrimSpace(enemy.Faction + "_" + enemy.Tier + "_" + enemy.Name)
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("enemy_%02d", index+1)
	}
	return coalesceID(sanitizeID(base), fmt.Sprintf("enemy_%02d", index+1))
}

func (a *ModelAdapter) buildStorylineContracts(dsl *DSL) {
	if a.setup == nil || dsl.Storyline == nil {
		return
	}
	for i, storyline := range a.setup.Storylines {
		name := strings.TrimSpace(storyline.Name)
		if name == "" {
			name = fmt.Sprintf("storyline_%02d", i+1)
		}
		id := coalesceID(sanitizeID(name), fmt.Sprintf("storyline_%02d", i+1))
		dsl.Storyline.Arcs = append(dsl.Storyline.Arcs, Arc{
			ID:       id,
			Name:     name,
			Position: len(dsl.Storyline.Arcs) + 1,
			CompletionReward: Reward{
				Title:       storyline.Payoff,
				Description: storyline.OpenQuestion,
			},
		})
	}
}

func (a *ModelAdapter) buildSetupContractChapters(dsl *DSL) {
	if a.setup == nil || dsl.Storyline == nil || len(a.setup.Storylines) == 0 {
		return
	}
	for i, storyline := range a.setup.Storylines {
		name := strings.TrimSpace(storyline.Name)
		if name == "" {
			name = fmt.Sprintf("storyline_%02d", i+1)
		}
		id := coalesceID(sanitizeID(name), fmt.Sprintf("storyline_%02d", i+1))
		step := Step{
			Order:       1,
			Description: strings.TrimSpace(storyline.Description),
			Event: Event{
				Type: "storyline_contract",
				StateDeltas: []StateDelta{
					{Target: name, Kind: "storyline", Field: "contract", To: "promised", Note: storylineContractNote(storyline)},
					{Target: name, Kind: "plot_thread", Field: "open_question", To: "raised", Note: storyline.OpenQuestion},
				},
			},
		}
		dsl.Storyline.Chapters = append(dsl.Storyline.Chapters, Chapter{
			ID:       "setup_" + id,
			Title:    name,
			Arc:      id,
			Position: i + 1,
			Objectives: []Objective{{
				ID:    "setup_" + id + "_contract",
				Name:  "Storyline Contract: " + name,
				Type:  "sequence",
				Steps: []Step{step},
			}},
		})
	}
}

func describeModelEvent(evt models.Event) string {
	for _, candidate := range []string{evt.Details, evt.Result, evt.Context} {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}
	parts := []string{}
	for _, value := range []string{evt.Actor, evt.Action, evt.Target, evt.Change, evt.Subject} {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, strings.TrimSpace(value))
		}
	}
	if len(parts) == 0 {
		return "outline event"
	}
	return strings.Join(parts, " -> ")
}

func eventTypeFromAction(action string) string {
	switch action {
	case models.ActionCombat, models.ActionDefeat:
		return "combat"
	case models.ActionMove, models.ActionEnter, models.ActionLeave, models.ActionTeleport, models.ActionEscape:
		return "move"
	case models.ActionAcquire, models.ActionUse, models.ActionLose, models.ActionCraft:
		return "acquire"
	case models.ActionMeet, models.ActionBefriend, models.ActionBetray, models.ActionReconcile:
		return "relationship"
	case models.ActionLearn, models.ActionDiscover, models.ActionReveal:
		return "knowledge"
	case models.ActionAwaken, models.ActionUpgrade, models.ActionMaster, models.ActionTransform, models.ActionRecover, models.ActionAfflict:
		return "status"
	case models.ActionSet, models.ActionProgress, models.ActionAchieve, models.ActionAbandon:
		return "goal"
	default:
		return "story"
	}
}

func buildEventStateDeltas(evt models.Event) []StateDelta {
	var deltas []StateDelta
	target := strings.TrimSpace(evt.Target)
	if target == "" {
		target = strings.TrimSpace(evt.Subject)
	}
	kind := strings.TrimSpace(evt.TargetType)
	if kind == "" {
		kind = strings.TrimSpace(evt.Type)
	}
	if kind == "" {
		kind = eventTypeFromAction(evt.Action)
	}
	if target != "" || evt.Change != "" || evt.Result != "" {
		deltas = append(deltas, StateDelta{
			Target: target,
			Kind:   kind,
			Field:  evt.Action,
			To:     coalesceString(evt.Change, evt.Result),
			Note:   describeModelEvent(evt),
		})
	}
	if evt.Type == models.EventTypeStoryline || evt.TargetType == models.TargetTypeStoryline {
		deltas = append(deltas, StateDelta{
			Target: target,
			Kind:   "storyline",
			Field:  "event",
			To:     coalesceString(evt.Change, evt.Action),
			Note:   describeModelEvent(evt),
		})
	}
	return deltas
}

func storylineAdvanceDescription(advance models.StorylineAdvance) string {
	parts := []string{advance.StorylineName, advance.Stage, advance.Change, advance.Consequence, advance.Pressure}
	var out []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return strings.Join(out, " | ")
}

func buildStorylineAdvanceDeltas(chapterID string, advance models.StorylineAdvance) []StateDelta {
	stage := strings.ToLower(strings.TrimSpace(advance.Stage))
	if stage == "" {
		stage = "progress"
	}
	note := fmt.Sprintf("change=%s; consequence=%s; pressure=%s", advance.Change, advance.Consequence, advance.Pressure)
	deltas := []StateDelta{{
		Target: strings.TrimSpace(advance.StorylineName),
		Kind:   "storyline",
		Field:  "stage",
		To:     stage,
		Cost:   strings.TrimSpace(advance.Consequence),
		Unit:   strings.TrimSpace(advance.Pressure),
		Note:   note,
	}}
	switch stage {
	case "payoff", "resolve", "resolved", "completion", "completed":
		deltas = append(deltas, StateDelta{Target: advance.StorylineName, Kind: "plot_thread", Field: "storyline", To: "resolved", Note: "payoff in " + chapterID})
	case "hook", "pressure", "reveal", "reversal", "twist", "progress":
		deltas = append(deltas, StateDelta{Target: advance.StorylineName, Kind: "plot_thread", Field: "storyline", To: "raised", Note: "active pressure in " + chapterID})
	}
	return deltas
}

func buildMysterySteps(chapterID string, mysteries models.ChapterMysteries) []Step {
	var steps []Step
	for _, planted := range mysteries.Planted {
		id := strings.TrimSpace(planted.ID)
		if id == "" {
			continue
		}
		clue := strings.TrimSpace(planted.Clue)
		steps = append(steps, Step{
			Description: fmt.Sprintf("mystery %s planted: %s", id, clue),
			Event: Event{
				Type: "mystery",
				StateDeltas: []StateDelta{{
					Target: id,
					Kind:   "plot_thread",
					Field:  "mystery",
					To:     "raised",
					Unit:   strings.TrimSpace(planted.Horizon),
					Cost:   strings.TrimSpace(planted.Status),
					Note:   clue,
				}},
			},
		})
	}
	for _, resolved := range mysteries.Resolved {
		id := strings.TrimSpace(resolved.ID)
		if id == "" {
			continue
		}
		resolution := strings.TrimSpace(resolved.Resolution)
		steps = append(steps, Step{
			Description: fmt.Sprintf("mystery %s resolved: %s", id, resolution),
			Event: Event{
				Type: "mystery",
				StateDeltas: []StateDelta{{
					Target: id,
					Kind:   "plot_thread",
					Field:  "mystery",
					To:     "resolved",
					Note:   resolution,
				}},
			},
		})
	}
	return steps
}

func buildStateAnchorDeltas(ch models.Chapter) []StateDelta {
	var deltas []StateDelta
	if strings.TrimSpace(ch.StateAnchor.Cultivation) != "" {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "cultivation",
			Field:  "realm",
			To:     ch.StateAnchor.Cultivation,
			Note:   "chapter start cultivation: " + ch.StateAnchor.Cultivation,
		})
	}
	if ch.StateAnchor.SpiritStones > 0 {
		deltas = append(deltas, StateDelta{
			Target: "spirit_stones",
			Kind:   "resource",
			Field:  "quantity",
			To:     fmt.Sprintf("%d", ch.StateAnchor.SpiritStones),
			Note:   "chapter start spirit stones",
		})
	}
	if len(ch.StateAnchor.Injuries) > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "injury",
			Field:  "active",
			To:     "injured",
			Note:   "chapter start injuries: " + strings.Join(ch.StateAnchor.Injuries, ", "),
		})
	}
	if len(ch.StateAnchor.Allies) > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "ally",
			Field:  "active",
			To:     strings.Join(ch.StateAnchor.Allies, ", "),
			Note:   "chapter start allies: " + strings.Join(ch.StateAnchor.Allies, ", "),
		})
	}
	if len(ch.StateAnchor.KeyItems) > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "item",
			Field:  "key_items",
			To:     strings.Join(ch.StateAnchor.KeyItems, ", "),
			Note:   "chapter start key items: " + strings.Join(ch.StateAnchor.KeyItems, ", "),
		})
	}
	deltas = append(deltas, buildStructuredProgressionDeltas(
		ch.StateAnchor.Cultivation,
		ch.StateAnchor.KeyItems,
		ch.StateAnchor.Injuries,
	)...)
	return deltas
}

func storylineContractNote(storyline models.Storyline) string {
	parts := []string{
		"scope=" + storyline.Scope,
		"payoff_style=" + storyline.PayoffStyle,
		"setup_role=" + storyline.SetupRole,
		"desire=" + storyline.Desire,
		"opposition=" + storyline.Opposition,
		"stakes=" + storyline.Stakes,
		"turn=" + storyline.Turn,
		"payoff=" + storyline.Payoff,
		"open_question=" + storyline.OpenQuestion,
	}
	return strings.Join(parts, "; ")
}

func (a *ModelAdapter) buildPlaceholderNPCs(dsl *DSL) {
	if a.outline == nil {
		return
	}

	seen := make(map[string]bool)
	if dsl.Characters != nil && dsl.Characters.Player != nil {
		seen[strings.TrimSpace(dsl.Characters.Player.Name)] = true
		seen[strings.TrimSpace(dsl.Characters.Player.ID)] = true
	}
	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				for _, rawName := range ch.Characters {
					name := canonicalCharacterName(rawName)
					if seen[name] {
						continue
					}
					seen[name] = true
					if name == "" || isGenericProtagonistName(name) || characterReferenceMatchesPlayer(name, dsl.Characters.Player) {
						continue
					}

					npcID := sanitizeID(name)
					dsl.Characters.NPCs = append(dsl.Characters.NPCs, NPC{
						ID:            npcID,
						Name:          name,
						Description:   fmt.Sprintf("Placeholder for %s from outline", name),
						IsPlaceholder: true,
					})
				}
			}
		}
	}
}

func (a *ModelAdapter) buildPlaceholderLocations(dsl *DSL) {
	if a.outline == nil {
		return
	}

	seen := make(map[string]bool)
	for _, loc := range dsl.World.Locations {
		seen[loc.ID] = true
	}
	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				addModelOutlineLocationPlaceholder(dsl, seen, ch.Location)
				addModelOutlineLocationPlaceholder(dsl, seen, ch.StateAnchor.Location)
			}
		}
	}
}

func addModelOutlineLocationPlaceholder(dsl *DSL, seen map[string]bool, raw string) {
	locName, locDescription := splitOutlineLocation(raw)
	if locName == "" {
		return
	}
	locID := coalesceID(sanitizeID(locName), fmt.Sprintf("location_%02d", len(dsl.World.Locations)+1))
	if seen[locID] {
		return
	}
	seen[locID] = true

	description := strings.TrimSpace(locDescription)
	if description == "" {
		description = fmt.Sprintf("Placeholder for %s from outline", locName)
	}
	dsl.World.Locations = append(dsl.World.Locations, Location{
		ID:                locID,
		Name:              locName,
		Type:              inferMapTypeFromName(locName),
		Description:       description,
		Atmosphere:        locDescription,
		IsPlaceholder:     true,
		PlaceholderSource: "outline",
	})
}

func splitOutlineLocation(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	for _, sep := range []string{"：", ":", "\n", " - ", " -- "} {
		if idx := strings.Index(raw, sep); idx > 0 {
			name := strings.TrimSpace(raw[:idx])
			desc := strings.TrimSpace(raw[idx+len(sep):])
			return name, desc
		}
	}

	return raw, ""
}

func (a *ModelAdapter) buildCharacters(dsl *DSL) {
	if a.characters == nil {
		return
	}

	for _, id := range sortedCharacterKeys(a.characters) {
		ch := a.characters[id]
		if ch == nil {
			continue
		}
		if isCraftProtagonist(ch) {
			dsl.Characters.Player = &Player{
				ID:           id,
				Name:         ch.Name,
				Description:  ch.Background,
				Age:          parseAge(ch.Age),
				Gender:       ch.Gender,
				Race:         ch.Race,
				Background:   ch.Background,
				Personality:  ch.Personality,
				Motivation:   ch.Motivation,
				Abilities:    ch.Abilities,
				Affiliations: ch.Affiliations,
				RoleInStory:  ch.RoleInStory,
				Voice:        ch.Voice,
				Class:        coalesceString(ch.CombatRole, "adventurer"),
				Skills:       ch.Skills,
				Stats:        statsFromCraft(ch.RPGStats, Stats{STR: 10, AGI: 10, INT: 10, VIT: 10, HP: 100, MP: 50}),
				Traits:       traitsFromTags(ch.DSLTags),
			}
		} else if isCraftEnemy(ch) {
			level := ch.PowerLevel
			if level <= 0 && ch.RPGStats != nil {
				level = ch.RPGStats.Level
			}
			if level <= 0 {
				level = 1
			}
			stats := statsFromCraft(ch.RPGStats, Stats{STR: 6 + level, AGI: 4 + level, INT: 3 + level, VIT: 5 + level, HP: 40 + level*20, MP: 20 + level*10})
			dsl.Characters.Enemies = append(dsl.Characters.Enemies, Enemy{
				ID:          id,
				Name:        ch.Name,
				Type:        coalesceString(ch.RPGRole, coalesceString(ch.CombatRole, "enemy")),
				Description: ch.Background,
				Appearance:  ch.Appearance,
				Abilities:   append([]string(nil), ch.Abilities...),
				Level:       level,
				Template: EnemyTemplate{
					BaseLevel: level,
					HPFormula: fmt.Sprintf("%d", stats.HP),
					StatsPerLevel: map[string]int{
						"str": maxPairInt(1, stats.STR),
						"agi": maxPairInt(1, stats.AGI),
						"int": maxPairInt(1, stats.INT),
						"vit": maxPairInt(1, stats.VIT),
					},
				},
			})
		} else {
			dsl.Characters.NPCs = append(dsl.Characters.NPCs, NPC{
				ID:           id,
				Name:         ch.Name,
				Role:         coalesceString(ch.RPGRole, ch.RoleInStory),
				Description:  ch.Background,
				Age:          parseAge(ch.Age),
				Gender:       ch.Gender,
				Appearance:   ch.Appearance,
				Background:   ch.Background,
				Personality:  ch.Personality,
				Affiliations: ch.Affiliations,
			})
		}
	}
}

func (a *ModelAdapter) buildLocations(dsl *DSL) {
	if a.locations == nil {
		return
	}

	// Clear placeholders first
	dsl.World.Locations = nil
	locationRefIDs := a.locationReferenceIDs()

	for _, id := range sortedLocationKeys(a.locations) {
		loc := a.locations[id]
		if loc == nil {
			continue
		}
		dslLoc := Location{
			ID:          id,
			Name:        loc.Name,
			Type:        coalesceString(loc.RPGMapType, loc.Type),
			Description: loc.Description,
			Appearance:  loc.Appearance,
			Atmosphere:  loc.Atmosphere,
			History:     loc.History,
			Inhabitants: loc.Inhabitants,
			Events:      loc.Events,
			Secrets:     loc.Secrets,
			Properties:  craftLocationProperties(loc),
		}

		for _, connName := range loc.ConnectedLocations {
			dslLoc.Connections = append(dslLoc.Connections, Connection{
				To: resolveLocationReferenceID(connName, locationRefIDs),
			})
		}

		if loc.SensoryDetails != nil {
			dslLoc.SensoryDetails = map[string][]string{
				"visual": loc.SensoryDetails.Sights,
				"audio":  loc.SensoryDetails.Sounds,
				"smell":  loc.SensoryDetails.Smells,
			}
		}

		dsl.World.Locations = append(dsl.World.Locations, dslLoc)
	}
}

func (a *ModelAdapter) locationReferenceIDs() map[string]string {
	refs := make(map[string]string)
	if a.locations == nil {
		return refs
	}
	for _, id := range sortedLocationKeys(a.locations) {
		loc := a.locations[id]
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		refs[normalizeValidationKey(id)] = id
		if loc != nil && strings.TrimSpace(loc.Name) != "" {
			refs[normalizeValidationKey(loc.Name)] = id
		}
	}
	return refs
}

func (a *ModelAdapter) buildItems(dsl *DSL) {
	if a.items == nil {
		return
	}

	for _, id := range sortedItemKeys(a.items) {
		item := a.items[id]
		if item == nil {
			continue
		}
		dsl.World.Items = append(dsl.World.Items, Item{
			ID:          id,
			Name:        item.Name,
			Type:        coalesceString(item.RPGItemType, item.Type),
			Rarity:      item.Rarity,
			Description: item.Description,
			Effects:     craftItemEffects(item),
		})
	}
}

func (a *ModelAdapter) buildOrganizations(dsl *DSL) {
	if a.organizations == nil || dsl.World == nil {
		return
	}

	ids := make([]string, 0, len(a.organizations))
	for id := range a.organizations {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for i, id := range ids {
		org := a.organizations[id]
		if org == nil {
			continue
		}
		ruleID := coalesceID(sanitizeID(id), fmt.Sprintf("organization_%02d", i+1))
		dsl.World.Rules = append(dsl.World.Rules, Rule{
			Name:    "organization_" + ruleID,
			Trigger: "organization.profile",
			Effect:  craftOrganizationRuleEffect(ruleID, org),
		})
		for j, effect := range org.StateEffects {
			dsl.World.Rules = append(dsl.World.Rules, Rule{
				Name:    fmt.Sprintf("organization_%s_state_%02d", ruleID, j+1),
				Trigger: "organization.state_effect",
				Effect:  craftStateEffectRuleEffect(effect),
			})
		}
	}
}

func isCraftProtagonist(ch *models.Character) bool {
	if ch == nil {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(ch.RoleInStory + " " + ch.RPGRole))
	return strings.Contains(role, "protagonist") || strings.Contains(role, "player") || strings.Contains(role, "主角")
}

func isCraftEnemy(ch *models.Character) bool {
	if ch == nil {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(ch.RoleInStory + " " + ch.RPGRole))
	return strings.Contains(role, "enemy") || strings.Contains(role, "boss") || strings.Contains(role, "antagonist") || strings.Contains(role, "反派")
}

func statsFromCraft(stats *models.CraftRPGStats, fallback Stats) Stats {
	if stats == nil {
		return fallback
	}
	out := fallback
	if stats.STR > 0 {
		out.STR = stats.STR
	}
	if stats.AGI > 0 {
		out.AGI = stats.AGI
	}
	if stats.INT > 0 {
		out.INT = stats.INT
	}
	if stats.VIT > 0 {
		out.VIT = stats.VIT
	}
	if stats.HP > 0 {
		out.HP = stats.HP
	}
	if stats.MP > 0 {
		out.MP = stats.MP
	}
	return out
}

func traitsFromTags(tags []string) map[string]Trait {
	if len(tags) == 0 {
		return nil
	}
	traits := make(map[string]Trait, len(tags))
	for _, tag := range tags {
		id := sanitizeID(strings.TrimSpace(tag))
		if id == "" {
			continue
		}
		traits[id] = Trait{Unlocked: true, Trigger: "craft_tag"}
	}
	return traits
}

func craftLocationProperties(loc *models.Location) map[string]interface{} {
	props := make(map[string]interface{})
	if loc.DangerLevel > 0 {
		props["danger_level"] = loc.DangerLevel
	}
	if len(loc.EncounterTags) > 0 {
		props["encounter_tags"] = append([]string(nil), loc.EncounterTags...)
	}
	if len(loc.ResourceTags) > 0 {
		props["resource_tags"] = append([]string(nil), loc.ResourceTags...)
	}
	if len(loc.DSLTags) > 0 {
		props["dsl_tags"] = append([]string(nil), loc.DSLTags...)
	}
	if len(loc.StateEffects) > 0 {
		props["state_effects"] = craftStateEffects(loc.StateEffects)
	}
	if len(props) == 0 {
		return nil
	}
	return props
}

func craftItemEffects(item *models.Item) map[string]interface{} {
	effects := make(map[string]interface{})
	for _, power := range item.Powers {
		if strings.TrimSpace(power) != "" {
			effects[sanitizeID(power)] = power
		}
	}
	if item.PowerLevel > 0 {
		effects["power_level"] = item.PowerLevel
	}
	if item.QuantityTracking {
		effects["quantity_tracking"] = true
	}
	if len(item.DSLTags) > 0 {
		effects["dsl_tags"] = append([]string(nil), item.DSLTags...)
	}
	if len(item.StateEffects) > 0 {
		effects["state_effects"] = craftStateEffects(item.StateEffects)
	}
	if len(effects) == 0 {
		return nil
	}
	return effects
}

func craftOrganizationRuleEffect(id string, org *models.Organization) string {
	if org == nil {
		return "id=" + id
	}
	parts := []string{
		"id=" + sanitizeRuleValue(id),
		"name=" + sanitizeRuleValue(org.Name),
		"type=" + sanitizeRuleValue(org.Type),
		"description=" + sanitizeRuleValue(org.Description),
		"headquarters=" + sanitizeRuleValue(org.Headquarters),
		"leadership=" + sanitizeRuleValue(org.Leadership),
		"goals=" + sanitizeRuleValue(strings.Join(org.Goals, " | ")),
		"ideology=" + sanitizeRuleValue(org.Ideology),
		"resources=" + sanitizeRuleValue(strings.Join(org.Resources, " | ")),
		"allies=" + sanitizeRuleValue(strings.Join(org.Allies, " | ")),
		"enemies=" + sanitizeRuleValue(strings.Join(org.Enemies, " | ")),
		"reputation=" + sanitizeRuleValue(org.Reputation),
		"structure=" + sanitizeRuleValue(org.Structure),
		"significance=" + sanitizeRuleValue(org.Significance),
		"dsl_tags=" + sanitizeRuleValue(strings.Join(org.DSLTags, " | ")),
	}
	return compactRuleParts(parts)
}

func craftStateEffectRuleEffect(effect models.CraftStateEffect) string {
	parts := []string{
		"target=" + sanitizeRuleValue(effect.Target),
		"kind=" + sanitizeRuleValue(effect.Kind),
		"field=" + sanitizeRuleValue(effect.Field),
		"from=" + sanitizeRuleValue(effect.From),
		"to=" + sanitizeRuleValue(effect.To),
		"unit=" + sanitizeRuleValue(effect.Unit),
		"cost=" + sanitizeRuleValue(effect.Cost),
		"note=" + sanitizeRuleValue(effect.Note),
	}
	if effect.Delta != 0 {
		parts = append(parts, fmt.Sprintf("delta=%d", effect.Delta))
	}
	return compactRuleParts(parts)
}

func compactRuleParts(parts []string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, key+"="+value)
	}
	return strings.Join(out, "; ")
}

func craftStateEffects(effects []models.CraftStateEffect) []StateDelta {
	deltas := make([]StateDelta, 0, len(effects))
	for _, effect := range effects {
		deltas = append(deltas, StateDelta{
			Target: effect.Target,
			Kind:   effect.Kind,
			Field:  effect.Field,
			From:   effect.From,
			To:     effect.To,
			Delta:  effect.Delta,
			Unit:   effect.Unit,
			Cost:   effect.Cost,
			Note:   effect.Note,
		})
	}
	return deltas
}

func resolveLocationReferenceID(raw string, refs map[string]string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if id, ok := refs[normalizeValidationKey(raw)]; ok {
		return id
	}
	return sanitizeID(raw)
}

func maxPairInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sortedCharacterKeys(values map[string]*models.Character) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedLocationKeys(values map[string]*models.Location) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedItemKeys(values map[string]*models.Item) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (a *ModelAdapter) inferPowerSystem() string {
	if a.setup == nil {
		return "default_progression_system"
	}

	keywords := strings.ToLower(a.setup.Premise + " " + a.setup.Theme)
	for _, genre := range a.setup.Genres {
		keywords += " " + strings.ToLower(genre)
	}

	switch {
	case strings.Contains(keywords, "修仙") || strings.Contains(keywords, "修真") || strings.Contains(keywords, "cultivation"):
		return "cultivation_system"
	case strings.Contains(keywords, "魔法") || strings.Contains(keywords, "magic") || strings.Contains(keywords, "fantasy"):
		return "magic_system"
	case strings.Contains(keywords, "科幻") || strings.Contains(keywords, "sci-fi") || strings.Contains(keywords, "机甲"):
		return "technology_system"
	case strings.Contains(keywords, "武侠") || strings.Contains(keywords, "martial"):
		return "martial_arts_system"
	default:
		return "default_progression_system"
	}
}
