package dsl

import (
	"fmt"
	"strconv"
	"strings"
)

// Validator validates DSL AST
type Validator struct {
	errors   []ValidationError
	warnings []ValidationWarning
}

type validationRefs struct {
	locations map[string]string
	items     map[string]string
	enemies   map[string]string
	npcs      map[string]string
	arcs      map[string]string
	chapters  map[string]string
	globalIDs map[string]string
	actors    map[string]string
}

// ValidationError represents a validation error
type ValidationError struct {
	Line    int
	Column  int
	Field   string
	Message string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Line    int
	Field   string
	Message string
}

// Error implements error interface
func (e ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s: %s", e.Line, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		errors:   make([]ValidationError, 0),
		warnings: make([]ValidationWarning, 0),
	}
}

// Validate performs full validation on DSL
func (v *Validator) Validate(dsl *DSL) error {
	v.errors = make([]ValidationError, 0)
	v.warnings = make([]ValidationWarning, 0)

	if dsl == nil {
		v.addError("dsl", "dsl document is required")
		return v.errors[0]
	}

	// Validate required blocks
	v.validateMetadata(dsl.Metadata)
	v.validateWorld(dsl.World)
	v.validateCharacters(dsl.Characters)
	v.validateStoryline(dsl.Storyline)
	v.validateSystems(dsl.Systems)

	// Cross-reference validation
	v.validateReferences(dsl, v.buildRefs(dsl))

	if len(v.errors) > 0 {
		return v.errors[0]
	}

	return nil
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() []ValidationError {
	return v.errors
}

// GetWarnings returns all validation warnings
func (v *Validator) GetWarnings() []ValidationWarning {
	return v.warnings
}

// HasErrors returns true if there are errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// HasWarnings returns true if there are warnings
func (v *Validator) HasWarnings() bool {
	return len(v.warnings) > 0
}

// AddError adds an error
func (v *Validator) addError(field, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// AddWarning adds a warning
func (v *Validator) addWarning(field, message string) {
	v.warnings = append(v.warnings, ValidationWarning{
		Field:   field,
		Message: message,
	})
}

// validateMetadata validates metadata block
func (v *Validator) validateMetadata(meta *Metadata) {
	if meta == nil {
		v.addError("metadata", "metadata block is required")
		return
	}

	if strings.TrimSpace(meta.Title) == "" {
		v.addError("metadata.title", "title is required")
	}

	if meta.DSLVersion == "" {
		v.addWarning("metadata.dsl_version", "dsl_version not specified, assuming 0.1.0")
		meta.DSLVersion = "0.1.0"
	}

	// Validate genre
	if len(meta.Genre) == 0 {
		v.addWarning("metadata.genre", "no genres specified")
	}
}

// validateWorld validates world block
func (v *Validator) validateWorld(world *World) {
	if world == nil {
		v.addWarning("world", "world block not defined")
		return
	}

	// Validate locations
	locationIDs := make(map[string]bool)
	for i, loc := range world.Locations {
		field := fmt.Sprintf("world.locations[%d]", i)

		if loc.ID == "" {
			v.addError(field+".id", "location id is required")
		} else if locationIDs[loc.ID] {
			v.addError(field+".id", fmt.Sprintf("duplicate location id: %s", loc.ID))
		} else {
			locationIDs[loc.ID] = true
		}

		if loc.Name == "" {
			v.addError(field+".name", "location name is required")
		}

		// Validate location type
		validTypes := []string{"indoor", "outdoor", "dungeon", "city", ""}
		if !contains(validTypes, loc.Type) {
			v.addWarning(field+".type", fmt.Sprintf("unknown location type: %s", loc.Type))
		}

		// Validate connections
		for j, conn := range loc.Connections {
			connField := fmt.Sprintf("%s.connections[%d]", field, j)
			if conn.To == "" {
				v.addError(connField+".to", "connection target is required")
			}
			if strings.TrimSpace(conn.Direction) == "" {
				v.addWarning(connField+".direction", "connection direction is not specified")
			}
		}
	}

	// Validate items
	itemIDs := make(map[string]bool)
	for i, item := range world.Items {
		field := fmt.Sprintf("world.items[%d]", i)

		if item.ID == "" {
			v.addError(field+".id", "item id is required")
		} else if itemIDs[item.ID] {
			v.addError(field+".id", fmt.Sprintf("duplicate item id: %s", item.ID))
		} else {
			itemIDs[item.ID] = true
		}

		if item.Name == "" {
			v.addError(field+".name", "item name is required")
		}

		// Validate rarity
		validRarities := []string{"common", "uncommon", "rare", "epic", "legendary", ""}
		if !contains(validRarities, item.Rarity) {
			v.addWarning(field+".rarity", fmt.Sprintf("unknown rarity: %s", item.Rarity))
		}
	}
}

// validateCharacters validates characters block
func (v *Validator) validateCharacters(chars *Characters) {
	if chars == nil {
		v.addError("characters", "characters block is required")
		return
	}

	// Validate player
	if chars.Player == nil {
		v.addError("characters.player", "player character is required")
	} else {
		v.validatePlayer(chars.Player)
	}

	// Validate enemies
	enemyIDs := make(map[string]bool)
	for i, enemy := range chars.Enemies {
		field := fmt.Sprintf("characters.enemies[%d]", i)

		if enemy.ID == "" {
			v.addError(field+".id", "enemy id is required")
		} else if enemyIDs[enemy.ID] {
			v.addError(field+".id", fmt.Sprintf("duplicate enemy id: %s", enemy.ID))
		} else {
			enemyIDs[enemy.ID] = true
		}

		if enemy.Name == "" {
			v.addError(field+".name", "enemy name is required")
		}

		// Validate template. Outline-derived enemies often start as narrative
		// placeholders; the simulator can use deterministic defaults until craft
		// supplies full combat templates.
		if enemy.Template.HPFormula == "" {
			chars.Enemies[i].Template.HPFormula = "100 + level * 20"
		}
	}

	// Validate NPCs
	npcIDs := make(map[string]bool)
	for i, npc := range chars.NPCs {
		field := fmt.Sprintf("characters.npcs[%d]", i)

		if npc.ID == "" {
			v.addError(field+".id", "npc id is required")
		} else if npcIDs[npc.ID] {
			v.addError(field+".id", fmt.Sprintf("duplicate npc id: %s", npc.ID))
		} else {
			npcIDs[npc.ID] = true
		}

		if npc.Name == "" {
			v.addError(field+".name", "npc name is required")
		}
	}
}

// validateSystems validates optional systems block
func (v *Validator) validateSystems(systems *Systems) {
	if systems == nil {
		return
	}

	if systems.AttributeSystem != nil {
		field := "systems.attribute_system"
		if strings.TrimSpace(systems.AttributeSystem.ID) == "" {
			v.addError(field+".id", "attribute system id is required")
		}
		if strings.TrimSpace(systems.AttributeSystem.Name) == "" {
			v.addError(field+".name", "attribute system name is required")
		}
		if len(systems.AttributeSystem.Attributes) == 0 {
			v.addWarning(field+".attributes", "attribute system has no attributes")
		}
		seenAttrIDs := make(map[string]bool)
		for i, attr := range systems.AttributeSystem.Attributes {
			attrField := fmt.Sprintf("%s.attributes[%d]", field, i)
			if strings.TrimSpace(attr.ID) == "" {
				v.addError(attrField+".id", "attribute id is required")
			} else if seenAttrIDs[attr.ID] {
				v.addError(attrField+".id", fmt.Sprintf("duplicate attribute id: %s", attr.ID))
			} else {
				seenAttrIDs[attr.ID] = true
			}
			if strings.TrimSpace(attr.Name) == "" {
				v.addError(attrField+".name", "attribute name is required")
			}
			validTypes := []string{"resource", "stat", "special", ""}
			if !contains(validTypes, attr.Type) {
				v.addWarning(attrField+".type", fmt.Sprintf("unknown attribute type: %s", attr.Type))
			}
			if attr.MaxValue > 0 && attr.MinValue > attr.MaxValue {
				v.addError(attrField+".min_value", "min_value must not exceed max_value")
			}
		}
	}

	if systems.PowerFormula != nil {
		field := "systems.power_formula"
		if strings.TrimSpace(systems.PowerFormula.ID) == "" {
			v.addError(field+".id", "power formula id is required")
		}
		if strings.TrimSpace(systems.PowerFormula.Name) == "" {
			v.addError(field+".name", "power formula name is required")
		}
		if len(systems.PowerFormula.Factors) == 0 {
			v.addWarning(field+".factors", "power formula has no factors")
		}
		seenFactors := make(map[string]bool)
		for i, factor := range systems.PowerFormula.Factors {
			factorField := fmt.Sprintf("%s.factors[%d]", field, i)
			if strings.TrimSpace(factor.Attribute) == "" {
				v.addError(factorField+".attribute", "power factor attribute is required")
			} else if seenFactors[factor.Attribute] {
				v.addWarning(factorField+".attribute", fmt.Sprintf("duplicate factor attribute: %s", factor.Attribute))
			} else {
				seenFactors[factor.Attribute] = true
			}
			if strings.TrimSpace(factor.Name) == "" {
				v.addWarning(factorField+".name", "power factor name is not specified")
			}
			if factor.Weight == 0 {
				v.addWarning(factorField+".weight", "power factor weight is zero")
			}
		}
	}

	if len(systems.ProgressionSystems) > 0 {
		seenProgression := make(map[string]bool)
		for i, prog := range systems.ProgressionSystems {
			field := fmt.Sprintf("systems.progression_systems[%d]", i)
			if strings.TrimSpace(prog.ID) == "" {
				v.addError(field+".id", "progression system id is required")
			} else if seenProgression[prog.ID] {
				v.addError(field+".id", fmt.Sprintf("duplicate progression system id: %s", prog.ID))
			} else {
				seenProgression[prog.ID] = true
			}
			if strings.TrimSpace(prog.Name) == "" {
				v.addError(field+".name", "progression system name is required")
			}
			if len(prog.Levels) == 0 {
				v.addWarning(field+".levels", "progression system has no levels")
			}
		}
	}

	if len(systems.Counters) > 0 {
		seenCounters := make(map[string]bool)
		for i, counter := range systems.Counters {
			field := fmt.Sprintf("systems.counters[%d]", i)
			if strings.TrimSpace(counter.Name) == "" {
				v.addError(field+".name", "counter name is required")
			} else if seenCounters[counter.Name] {
				v.addError(field+".name", fmt.Sprintf("duplicate counter name: %s", counter.Name))
			} else {
				seenCounters[counter.Name] = true
			}
			if len(counter.Milestones) == 0 {
				v.addWarning(field+".milestones", "counter has no milestones")
			}
		}
	}
}

// validatePlayer validates player character
func (v *Validator) validatePlayer(player *Player) {
	if player.ID == "" {
		v.addWarning("characters.player.id", "player id not specified, using default")
		player.ID = "char_player"
	}

	if player.Name == "" {
		v.addError("characters.player.name", "player name is required")
	}

	// Validate stats
	if player.Stats.HP <= 0 {
		v.addWarning("characters.player.stats.hp", "hp not specified or invalid, using default 100")
		player.Stats.HP = 100
	}

	if player.Stats.STR <= 0 {
		player.Stats.STR = 10
	}
	if player.Stats.AGI <= 0 {
		player.Stats.AGI = 10
	}
	if player.Stats.INT <= 0 {
		player.Stats.INT = 10
	}
	if player.Stats.VIT <= 0 {
		player.Stats.VIT = 10
	}
}

// validateStoryline validates storyline block
func (v *Validator) validateStoryline(story *Storyline) {
	if story == nil {
		v.addError("storyline", "storyline block is required")
		return
	}

	if len(story.Chapters) == 0 {
		v.addError("storyline.chapters", "at least one chapter is required")
		return
	}

	chapterIDs := make(map[string]bool)
	for i, chapter := range story.Chapters {
		field := fmt.Sprintf("storyline.chapters[%d]", i)

		if chapter.ID == "" {
			v.addError(field+".id", "chapter id is required")
		} else if chapterIDs[chapter.ID] {
			v.addError(field+".id", fmt.Sprintf("duplicate chapter id: %s", chapter.ID))
		} else {
			chapterIDs[chapter.ID] = true
		}

		if chapter.Title == "" {
			v.addError(field+".title", "chapter title is required")
		}
		if chapter.Position <= 0 {
			v.addWarning(field+".position", "chapter position is not specified")
		}

		// Validate objectives
		if len(chapter.Objectives) == 0 {
			v.addWarning(field+".objectives", "no objectives defined for chapter")
		}

		for j, obj := range chapter.Objectives {
			objField := fmt.Sprintf("%s.objectives[%d]", field, j)
			v.validateObjective(objField, &obj)
		}
	}

	arcIDs := make(map[string]bool)
	for i, arc := range story.Arcs {
		field := fmt.Sprintf("storyline.arcs[%d]", i)

		if strings.TrimSpace(arc.ID) == "" {
			v.addError(field+".id", "arc id is required")
		} else if arcIDs[arc.ID] {
			v.addError(field+".id", fmt.Sprintf("duplicate arc id: %s", arc.ID))
		} else {
			arcIDs[arc.ID] = true
		}
		if strings.TrimSpace(arc.Name) == "" {
			v.addError(field+".name", "arc name is required")
		}
		if arc.Position <= 0 {
			v.addWarning(field+".position", "arc position is not specified")
		}
		if len(arc.Chapters) == 0 {
			v.addWarning(field+".chapters", "arc has no chapter references")
		}
	}
}

// validateObjective validates an objective
func (v *Validator) validateObjective(field string, obj *Objective) {
	if obj.Name == "" {
		v.addError(field+".name", "objective name is required")
	}

	// Validate type
	validTypes := []string{"sequence", "parallel", "optional", ""}
	if !contains(validTypes, obj.Type) {
		v.addWarning(field+".type", fmt.Sprintf("unknown objective type: %s, defaulting to sequence", obj.Type))
		obj.Type = "sequence"
	}

	// Validate steps
	if len(obj.Steps) == 0 {
		v.addError(field+".steps", "at least one step is required")
		return
	}

	seenOrders := make(map[int]bool)
	previousOrder := 0
	for i, step := range obj.Steps {
		stepField := fmt.Sprintf("%s.steps[%d]", field, i)
		if step.Order > 0 {
			if seenOrders[step.Order] {
				v.addError(stepField+".order", fmt.Sprintf("duplicate step order: %d", step.Order))
			} else {
				seenOrders[step.Order] = true
			}
			if previousOrder > 0 && step.Order <= previousOrder {
				v.addWarning(stepField+".order", "steps are not sorted by increasing order")
			}
			previousOrder = step.Order
		}
		v.validateStep(stepField, &step)
	}

	if strings.EqualFold(obj.Type, "sequence") || strings.TrimSpace(obj.Type) == "" {
		for expected := 1; expected <= len(obj.Steps); expected++ {
			if !seenOrders[expected] {
				v.addWarning(field+".steps", fmt.Sprintf("sequence objective is missing step order %d", expected))
			}
		}
	}
}

// validateStep validates a step
func (v *Validator) validateStep(field string, step *Step) {
	if step.Order <= 0 {
		v.addError(field+".order", "step order must be positive")
	}
	if strings.TrimSpace(step.Description) == "" {
		v.addWarning(field+".description", "step description is not specified")
	}
	if step.Trigger != nil && strings.TrimSpace(step.Trigger.Location) == "" && strings.TrimSpace(step.Trigger.Type) == "location" {
		v.addWarning(field+".trigger.location", "location trigger is missing a location")
	}

	// Validate event
	v.validateEvent(field+".event", &step.Event)
}

// validateEvent validates an event
func (v *Validator) validateEvent(field string, event *Event) {
	if event.Type == "" {
		v.addError(field+".type", "event type is required")
		return
	}

	// Validate event type. Chapter-to-DSL may emit narrative event types that
	// are interpreted through state_delta rather than a dedicated sub-block.
	validTypes := []string{
		"spawn", "move", "combat", "dialogue", "acquire",
		"status", "knowledge", "relationship", "location", "transition",
		"story", "storyline", "mystery", "plot_thread", "resource", "goal",
	}
	if !contains(validTypes, event.Type) {
		v.addWarning(field+".type", fmt.Sprintf("unknown event type: %s", event.Type))
	}

	// Type-specific validation
	switch event.Type {
	case "spawn":
		if event.Spawn == nil {
			v.addWarning(field, "spawn event missing spawn data (treated as narrative-only)")
		} else {
			if strings.TrimSpace(event.Spawn.Actor) == "" {
				v.addWarning(field+".actor", "spawn event actor is not specified")
			}
			if strings.TrimSpace(event.Spawn.Location) == "" {
				v.addWarning(field+".location", "spawn event location is not specified")
			}
		}

	case "move":
		if event.Move == nil {
			v.addWarning(field, "move event missing move data (treated as narrative-only)")
		} else {
			if strings.TrimSpace(event.Move.Actor) == "" {
				v.addWarning(field+".actor", "move event actor is not specified")
			}
			if event.Move.To == "" {
				v.addError(field+".to", "move event requires destination")
			}
			if strings.TrimSpace(event.Move.From) != "" && strings.TrimSpace(event.Move.From) == strings.TrimSpace(event.Move.To) {
				v.addWarning(field+".from", "move event origin and destination are the same")
			}
		}

	case "combat":
		if event.Combat == nil {
			v.addWarning(field, "combat event without setup data")
		} else {
			if len(event.Combat.Setup.Enemies) == 0 {
				v.addError(field+".combat.enemies", "combat event requires at least one enemy")
			}
			for i, enemy := range event.Combat.Setup.Enemies {
				enemyField := fmt.Sprintf("%s.combat.enemies[%d]", field, i)
				if strings.TrimSpace(enemy.ID) == "" {
					v.addError(enemyField+".id", "combat enemy id is required")
				}
				if enemy.Count <= 0 {
					v.addError(enemyField+".count", "combat enemy count must be positive")
				}
				if enemy.Level < 0 {
					v.addError(enemyField+".level", "combat enemy level cannot be negative")
				}
			}
		}

	case "dialogue":
		if event.Dialogue == nil {
			v.addWarning(field, "dialogue event without dialogue data")
		} else {
			if strings.TrimSpace(event.Dialogue.Speaker) == "" {
				v.addWarning(field+".speaker", "dialogue speaker is not specified")
			}
			if strings.TrimSpace(event.Dialogue.Text) == "" {
				v.addError(field+".text", "dialogue text is required")
			}
		}

	case "acquire":
		if event.Acquire == nil {
			v.addWarning(field, "acquire event without acquisition data")
		} else {
			if strings.TrimSpace(event.Acquire.Item) == "" {
				v.addError(field+".item", "acquire event item is required")
			}
			if event.Acquire.Quantity <= 0 {
				v.addError(field+".quantity", "acquire event quantity must be positive")
			}
		}
	}
}

// validateReferences validates cross-references
func (v *Validator) validateReferences(dsl *DSL, refs *validationRefs) {
	if dsl == nil || refs == nil {
		return
	}

	if dsl.World != nil {
		for i, loc := range dsl.World.Locations {
			field := fmt.Sprintf("world.locations[%d]", i)
			for j, conn := range loc.Connections {
				connField := fmt.Sprintf("%s.connections[%d]", field, j)
				if strings.TrimSpace(conn.To) == "" {
					continue
				}
				if _, ok := refs.locations[conn.To]; !ok {
					v.addError(connField+".to", fmt.Sprintf("undefined location: %s", conn.To))
				}
			}
		}
		for i, item := range dsl.World.Items {
			field := fmt.Sprintf("world.items[%d]", i)
			if strings.TrimSpace(item.ID) == "" {
				continue
			}
			refs.items[item.ID] = field
		}
	}

	if dsl.Characters != nil {
		playerName := ""
		playerID := ""
		if dsl.Characters.Player != nil {
			playerName = strings.TrimSpace(dsl.Characters.Player.Name)
			playerID = strings.TrimSpace(dsl.Characters.Player.ID)
		}

		for i, enemy := range dsl.Characters.Enemies {
			field := fmt.Sprintf("characters.enemies[%d]", i)
			for _, locID := range enemy.SpawnLocations {
				if strings.TrimSpace(locID) == "" {
					continue
				}
				if _, ok := refs.locations[locID]; !ok {
					v.addError(field+".spawn_locations", fmt.Sprintf("undefined location: %s", locID))
				}
			}
			for j, drop := range enemy.Drops.Fixed {
				dropField := fmt.Sprintf("%s.drops.fixed[%d]", field, j)
				if strings.TrimSpace(drop.Item) != "" {
					if _, ok := refs.items[drop.Item]; !ok {
						v.addError(dropField+".item", fmt.Sprintf("undefined item: %s", drop.Item))
					}
				}
			}
			for j, drop := range enemy.Drops.Random {
				dropField := fmt.Sprintf("%s.drops.random[%d]", field, j)
				if strings.TrimSpace(drop.Item) != "" {
					if _, ok := refs.items[drop.Item]; !ok {
						v.addError(dropField+".item", fmt.Sprintf("undefined item: %s", drop.Item))
					}
				}
			}
		}

		for i, npc := range dsl.Characters.NPCs {
			field := fmt.Sprintf("characters.npcs[%d]", i)
			if strings.TrimSpace(npc.DefaultLocation) != "" {
				if _, ok := refs.locations[npc.DefaultLocation]; !ok {
					v.addError(field+".default_location", fmt.Sprintf("undefined location: %s", npc.DefaultLocation))
				}
			}
			if npc.Services.Trade != nil {
				for j, itemID := range npc.Services.Trade.AcceptsItems {
					if strings.TrimSpace(itemID) == "" {
						continue
					}
					if _, ok := refs.items[itemID]; !ok {
						v.addError(fmt.Sprintf("%s.services.trade.accepts_items[%d]", field, j), fmt.Sprintf("undefined item: %s", itemID))
					}
				}
			}
			if strings.TrimSpace(npc.ID) != "" {
				refs.actors[normalizeValidationKey(npc.ID)] = field + ".id"
			}
			if strings.TrimSpace(npc.Name) != "" {
				refs.actors[normalizeValidationKey(npc.Name)] = field + ".name"
			}
		}

		if strings.TrimSpace(playerID) != "" {
			refs.actors[normalizeValidationKey(playerID)] = "characters.player.id"
		}
		if strings.TrimSpace(playerName) != "" {
			refs.actors[normalizeValidationKey(playerName)] = "characters.player.name"
		}
	}

	if dsl.Storyline == nil {
		return
	}

	if dsl.Systems != nil {
		if dsl.Systems.Breakthrough != nil {
			for i, stage := range dsl.Systems.Breakthrough.Stages {
				itemID := strings.TrimSpace(stage.Requirements.Item)
				if itemID == "" {
					continue
				}
				if _, ok := refs.items[itemID]; !ok {
					v.addError(fmt.Sprintf("systems.breakthrough.stages[%d].requirements.item", i), fmt.Sprintf("undefined item: %s", itemID))
				}
			}
		}
		for i, trigger := range dsl.Systems.Triggers {
			for j, condition := range trigger.Conditions {
				if strings.TrimSpace(condition.Location) == "" {
					continue
				}
				if _, ok := refs.locations[condition.Location]; !ok {
					v.addError(fmt.Sprintf("systems.triggers[%d].conditions[%d].location", i, j), fmt.Sprintf("undefined location: %s", condition.Location))
				}
			}
		}
	}

	chapterPositionsByArc := make(map[string]map[int]string)
	for i, chapter := range dsl.Storyline.Chapters {
		field := fmt.Sprintf("storyline.chapters[%d]", i)
		chapterID := strings.TrimSpace(chapter.ID)
		arcID := strings.TrimSpace(chapter.Arc)
		v.validateItemRefs(field+".completion.items", refs, chapter.Completion.Items)
		if arcID != "" {
			if _, ok := refs.arcs[arcID]; !ok {
				v.addError(field+".arc", fmt.Sprintf("undefined arc: %s", arcID))
			}
			if chapterPositionsByArc[arcID] == nil {
				chapterPositionsByArc[arcID] = make(map[int]string)
			}
			if chapter.Position > 0 {
				if prevField, ok := chapterPositionsByArc[arcID][chapter.Position]; ok {
					v.addWarning(field+".position", fmt.Sprintf("duplicate chapter position %d also used by %s", chapter.Position, prevField))
				} else {
					chapterPositionsByArc[arcID][chapter.Position] = field
				}
			}
		}
		if chapterID != "" {
			refs.chapters[chapterID] = field
		}
	}

	for i, arc := range dsl.Storyline.Arcs {
		field := fmt.Sprintf("storyline.arcs[%d]", i)
		v.validateItemRefs(field+".completion_reward.items", refs, arc.CompletionReward.Items)
		for j, chapterID := range arc.Chapters {
			if strings.TrimSpace(chapterID) == "" {
				continue
			}
			if _, ok := refs.chapters[chapterID]; !ok {
				v.addError(fmt.Sprintf("%s.chapters[%d]", field, j), fmt.Sprintf("undefined chapter: %s", chapterID))
			}
		}
	}

	for i, chapter := range dsl.Storyline.Chapters {
		chapterField := fmt.Sprintf("storyline.chapters[%d]", i)
		for j, obj := range chapter.Objectives {
			objField := fmt.Sprintf("%s.objectives[%d]", chapterField, j)
			for k, step := range obj.Steps {
				stepField := fmt.Sprintf("%s.steps[%d]", objField, k)
				v.validateStepReferences(dsl, refs, chapter.ID, stepField, &step)
			}
		}
	}
}

func (v *Validator) validateStepReferences(dsl *DSL, refs *validationRefs, chapterID, field string, step *Step) {
	if step == nil {
		return
	}

	event := &step.Event
	if event.Spawn != nil {
		if strings.TrimSpace(event.Spawn.Location) != "" {
			if _, ok := refs.locations[event.Spawn.Location]; !ok {
				v.addError(field+".event.spawn.location", fmt.Sprintf("undefined location: %s", event.Spawn.Location))
			}
		}
		if strings.TrimSpace(event.Spawn.Actor) != "" {
			v.validateActorRef(field+".event.spawn.actor", refs, event.Spawn.Actor)
		}
	}

	if event.Move != nil {
		if strings.TrimSpace(event.Move.From) != "" {
			if _, ok := refs.locations[event.Move.From]; !ok {
				v.addError(field+".event.move.from", fmt.Sprintf("undefined location: %s", event.Move.From))
			}
		}
		if strings.TrimSpace(event.Move.To) != "" {
			if _, ok := refs.locations[event.Move.To]; !ok {
				v.addError(field+".event.move.to", fmt.Sprintf("undefined location: %s", event.Move.To))
			}
		}
		if strings.TrimSpace(event.Move.Actor) != "" {
			v.validateActorRef(field+".event.move.actor", refs, event.Move.Actor)
		}
	}

	if event.Combat != nil {
		if strings.TrimSpace(event.Combat.Setup.Location) != "" {
			if _, ok := refs.locations[event.Combat.Setup.Location]; !ok {
				v.addError(field+".event.combat.location", fmt.Sprintf("undefined location: %s", event.Combat.Setup.Location))
			}
		}
		for i, enemy := range event.Combat.Setup.Enemies {
			enemyField := fmt.Sprintf("%s.event.combat.enemies[%d]", field, i)
			if strings.TrimSpace(enemy.ID) == "" {
				continue
			}
			if _, ok := refs.enemies[enemy.ID]; !ok {
				v.addWarning(enemyField+".id", fmt.Sprintf("undefined enemy: %s (will be treated as generic)", enemy.ID))
			}
		}
		for _, result := range []*EventResult{event.Combat.OnVictory, event.Combat.OnDefeat} {
			if result == nil {
				continue
			}
			v.validateEventResultItems(field+".event.combat", refs, result)
		}
	}

	if event.Dialogue != nil && strings.TrimSpace(event.Dialogue.Speaker) != "" {
		v.validateActorRef(field+".event.dialogue.speaker", refs, event.Dialogue.Speaker)
	}
	if event.Acquire != nil && strings.TrimSpace(event.Acquire.Actor) != "" {
		v.validateActorRef(field+".event.acquire.actor", refs, event.Acquire.Actor)
	}

	if event.Trigger != nil && strings.TrimSpace(event.Trigger.Location) != "" {
		if _, ok := refs.locations[event.Trigger.Location]; !ok {
			v.addError(field+".event.trigger.location", fmt.Sprintf("undefined location: %s", event.Trigger.Location))
		}
	}

	if event.Require != nil {
		v.validateItemRefs(field+".event.require.items", refs, event.Require.Items)
	}
	if event.OnComplete != nil {
		v.validateEventResultItems(field+".event.on_complete", refs, event.OnComplete)
	}
	if event.OnFail != nil {
		v.validateEventResultItems(field+".event.on_fail", refs, event.OnFail)
	}

	for i, delta := range event.StateDeltas {
		v.validateStateDelta(fmt.Sprintf("%s.event.state_deltas[%d]", field, i), delta, refs)
	}

	if event.Acquire != nil {
		if strings.TrimSpace(event.Acquire.Item) != "" {
			if _, ok := refs.items[event.Acquire.Item]; !ok {
				v.addError(field+".event.acquire.item", fmt.Sprintf("undefined item: %s", event.Acquire.Item))
			}
		}
	}
}

func (v *Validator) validateEventResultItems(field string, refs *validationRefs, result *EventResult) {
	if result == nil {
		return
	}
	v.validateItemRefs(field+".items", refs, result.Items)
}

func (v *Validator) validateItemRefs(field string, refs *validationRefs, items []string) {
	for i, itemID := range items {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		if _, ok := refs.items[itemID]; !ok {
			v.addError(fmt.Sprintf("%s[%d]", field, i), fmt.Sprintf("undefined item: %s", itemID))
		}
	}
}

func (v *Validator) validateActorRef(field string, refs *validationRefs, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if _, ok := refs.actors[normalizeValidationKey(value)]; !ok {
		v.addWarning(field, fmt.Sprintf("unknown actor reference: %s", value))
	}
}

func (v *Validator) validateStateDelta(field string, delta StateDelta, refs *validationRefs) {
	if strings.TrimSpace(delta.Target) == "" && strings.TrimSpace(delta.Kind) == "" && strings.TrimSpace(delta.Field) == "" &&
		strings.TrimSpace(delta.From) == "" && strings.TrimSpace(delta.To) == "" && delta.Delta == 0 &&
		strings.TrimSpace(delta.Unit) == "" && strings.TrimSpace(delta.Cost) == "" && strings.TrimSpace(delta.Note) == "" {
		v.addWarning(field, "state delta is empty")
		return
	}

	kind := strings.ToLower(strings.TrimSpace(delta.Kind))
	validKinds := []string{
		"storyline", "plot_thread", "resource", "cultivation", "relationship",
		"goal", "status", "premise", "item", "injury", "death", "revive",
		"time", "transition", "gene", "mech", "ally", "equipment", "location",
		"knowledge", "story",
	}
	if kind != "" && !contains(validKinds, kind) {
		v.addWarning(field+".kind", fmt.Sprintf("unknown state delta kind: %s", delta.Kind))
	}

	if delta.Delta != 0 {
		from, fromOK := strconv.Atoi(strings.TrimSpace(delta.From))
		to, toOK := strconv.Atoi(strings.TrimSpace(delta.To))
		if fromOK == nil && toOK == nil && from+delta.Delta != to {
			v.addError(field+".delta", fmt.Sprintf("delta mismatch: %d + %d != %d", from, delta.Delta, to))
		}
	}

	if kind == "resource" || kind == "item" || kind == "goal" || kind == "storyline" || kind == "plot_thread" {
		if strings.TrimSpace(delta.Target) == "" && strings.TrimSpace(delta.Field) == "" {
			v.addWarning(field+".target", fmt.Sprintf("%s state delta has no target or field", kind))
		}
	}

}

func (v *Validator) buildRefs(dsl *DSL) *validationRefs {
	refs := &validationRefs{
		locations: make(map[string]string),
		items:     make(map[string]string),
		enemies:   make(map[string]string),
		npcs:      make(map[string]string),
		arcs:      make(map[string]string),
		chapters:  make(map[string]string),
		globalIDs: make(map[string]string),
		actors:    make(map[string]string),
	}

	if dsl == nil {
		return refs
	}

	register := func(kind, field, id string, dest map[string]string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if prevKind, ok := refs.globalIDs[id]; ok && prevKind != kind {
			v.addWarning(field+".id", fmt.Sprintf("id %q is shared by multiple entity types; references may be ambiguous", id))
		} else if !ok {
			refs.globalIDs[id] = kind
		}
		if dest != nil {
			if _, exists := dest[id]; !exists {
				dest[id] = field
			}
		}
	}
	addActor := func(field, value string) {
		key := normalizeValidationKey(value)
		if key == "" {
			return
		}
		if _, exists := refs.actors[key]; !exists {
			refs.actors[key] = field
		}
	}

	if dsl.World != nil {
		for i, loc := range dsl.World.Locations {
			register("location", fmt.Sprintf("world.locations[%d]", i), loc.ID, refs.locations)
		}
		for i, item := range dsl.World.Items {
			register("item", fmt.Sprintf("world.items[%d]", i), item.ID, refs.items)
		}
	}

	if dsl.Characters != nil {
		if dsl.Characters.Player != nil {
			register("player", "characters.player", dsl.Characters.Player.ID, nil)
			addActor("characters.player.id", dsl.Characters.Player.ID)
			addActor("characters.player.name", dsl.Characters.Player.Name)
		}
		for i, enemy := range dsl.Characters.Enemies {
			field := fmt.Sprintf("characters.enemies[%d]", i)
			register("enemy", field, enemy.ID, refs.enemies)
			addActor(field+".id", enemy.ID)
			addActor(field+".name", enemy.Name)
		}
		for i, npc := range dsl.Characters.NPCs {
			field := fmt.Sprintf("characters.npcs[%d]", i)
			register("npc", field, npc.ID, refs.npcs)
			addActor(field+".id", npc.ID)
			addActor(field+".name", npc.Name)
		}
	}

	if dsl.Storyline != nil {
		for i, arc := range dsl.Storyline.Arcs {
			register("arc", fmt.Sprintf("storyline.arcs[%d]", i), arc.ID, refs.arcs)
		}
		for i, chapter := range dsl.Storyline.Chapters {
			register("chapter", fmt.Sprintf("storyline.chapters[%d]", i), chapter.ID, refs.chapters)
		}
	}

	return refs
}

func normalizeValidationKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
