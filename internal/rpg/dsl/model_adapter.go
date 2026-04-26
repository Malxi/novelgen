package dsl

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
)

// ModelAdapter builds DSL AST structs from models.* types used by CLI commands.
// All constructor parameters are optional — nil values produce a minimal DSL skeleton
// that the simulator can still run against (producing info-level issues about what's missing).
type ModelAdapter struct {
	setup       *models.StorySetup
	outline     *models.Outline
	characters  map[string]*models.Character
	locations   map[string]*models.Location
	items       map[string]*models.Item
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
	a.buildDefaultSystems(dsl)

	switch phase {
	case PhaseSetup:
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

	if a.setup != nil && a.setup.ProjectName != "" {
		playerID = sanitizeID(a.setup.ProjectName + "_protagonist")
	}

	// If craft characters are available, find the protagonist among them
	if a.characters != nil {
		for id, ch := range a.characters {
			if ch.RoleInStory == "protagonist" || ch.RoleInStory == "主角" {
				playerName = ch.Name
				playerID = id
				break
			}
		}
	}

	dsl.Characters.Player = &Player{
		ID:   playerID,
		Name: playerName,
		Stats: Stats{
			STR: 10,
			AGI: 10,
			INT: 10,
			VIT: 10,
			HP:  100,
			MP:  50,
		},
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
			dsl.Systems.AttributeSystem = &AttributeSystem{
				ID:         "custom_attrs",
				Name:       "Custom Attributes",
				Attributes: attrs,
			}
		}

		// Build progression systems
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

func (a *ModelAdapter) buildChaptersFromOutline(dsl *DSL) {
	if a.outline == nil {
		return
	}

	pos := 0
	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				pos++
				chapter := Chapter{
					ID:       ch.ID,
					Title:    ch.Title,
					Position: pos,
				}

				// Build objectives from chapter events and beats
				var steps []Step
				stepOrder := 0

				for _, evt := range ch.Events {
					stepOrder++
					step := Step{
						Order:       stepOrder,
						Description: evt.Details,
					}
					step.Event = a.buildEventFromModel(evt)
					steps = append(steps, step)
				}

				for _, beat := range ch.Beats {
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

func (a *ModelAdapter) buildEventFromModel(evt models.Event) Event {
	dslEvent := Event{
		Type: evt.Type,
	}

	switch evt.Action {
	case models.ActionCombat:
		dslEvent.Type = "combat"
		dslEvent.Combat = &CombatEvent{
			Setup: CombatSetup{
				Location: evt.Context,
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

	return dslEvent
}

func (a *ModelAdapter) buildPlaceholderNPCs(dsl *DSL) {
	if a.outline == nil {
		return
	}

	seen := make(map[string]bool)
	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				for _, name := range ch.Characters {
					if seen[name] {
						continue
					}
					seen[name] = true

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
	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				if ch.Location == "" || seen[ch.Location] {
					continue
				}
				seen[ch.Location] = true

				locID := sanitizeID(ch.Location)
				dsl.World.Locations = append(dsl.World.Locations, Location{
					ID:            locID,
					Name:          ch.Location,
					Type:          "indoor",
					Description:   fmt.Sprintf("Placeholder for %s from outline", ch.Location),
					IsPlaceholder: true,
				})
			}
		}
	}
}

func (a *ModelAdapter) buildCharacters(dsl *DSL) {
	if a.characters == nil {
		return
	}

	first := true
	for id, ch := range a.characters {
		if first && (ch.RoleInStory == "protagonist" || ch.RoleInStory == "主角") {
			// Replace default player with crafted character
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
				Stats: Stats{
					STR: 10,
					AGI: 10,
					INT: 10,
					VIT: 10,
					HP:  100,
					MP:  50,
				},
			}
			first = false
		} else {
			dsl.Characters.NPCs = append(dsl.Characters.NPCs, NPC{
				ID:           id,
				Name:         ch.Name,
				Role:         ch.RoleInStory,
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

	for id, loc := range a.locations {
		dslLoc := Location{
			ID:          id,
			Name:        loc.Name,
			Type:        loc.Type,
			Description: loc.Description,
			Appearance:  loc.Appearance,
			Atmosphere:  loc.Atmosphere,
			History:     loc.History,
			Inhabitants: loc.Inhabitants,
			Events:      loc.Events,
			Secrets:     loc.Secrets,
		}

		for _, connName := range loc.ConnectedLocations {
			dslLoc.Connections = append(dslLoc.Connections, Connection{
				To: sanitizeID(connName),
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

func (a *ModelAdapter) buildItems(dsl *DSL) {
	if a.items == nil {
		return
	}

	for id, item := range a.items {
		dsl.World.Items = append(dsl.World.Items, Item{
			ID:          id,
			Name:        item.Name,
			Type:        item.Type,
			Description: item.Description,
		})
	}
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

