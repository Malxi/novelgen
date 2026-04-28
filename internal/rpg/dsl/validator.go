package dsl

import (
	"fmt"
	"strings"
)

// Validator validates DSL AST
type Validator struct {
	errors   []ValidationError
	warnings []ValidationWarning
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

	// Validate required blocks
	v.validateMetadata(dsl.Metadata)
	v.validateWorld(dsl.World)
	v.validateCharacters(dsl.Characters)
	v.validateStoryline(dsl.Storyline)

	// Cross-reference validation
	v.validateReferences(dsl)

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

		// Validate template
		if enemy.Template.HPFormula == "" {
			v.addWarning(field+".template.hp_formula", "hp_formula not specified, using default")
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

		// Validate objectives
		if len(chapter.Objectives) == 0 {
			v.addWarning(field+".objectives", "no objectives defined for chapter")
		}

		for j, obj := range chapter.Objectives {
			objField := fmt.Sprintf("%s.objectives[%d]", field, j)
			v.validateObjective(objField, &obj)
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

	for i, step := range obj.Steps {
		stepField := fmt.Sprintf("%s.steps[%d]", field, i)
		v.validateStep(stepField, &step)
	}
}

// validateStep validates a step
func (v *Validator) validateStep(field string, step *Step) {
	if step.Order <= 0 {
		v.addError(field+".order", "step order must be positive")
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
	}
	if !contains(validTypes, event.Type) {
		v.addWarning(field+".type", fmt.Sprintf("unknown event type: %s", event.Type))
	}

	// Type-specific validation
	switch event.Type {
	case "spawn":
		if event.Spawn == nil {
			v.addWarning(field, "spawn event missing spawn data (treated as narrative-only)")
		}

	case "move":
		if event.Move != nil {
			if event.Move.To == "" {
				v.addError(field+".to", "move event requires destination")
			}
		}

	case "combat":
		if event.Combat == nil {
			v.addWarning(field, "combat event without setup data")
		}
	}
}

// validateReferences validates cross-references
func (v *Validator) validateReferences(dsl *DSL) {
	if dsl.World == nil || dsl.Characters == nil || dsl.Storyline == nil {
		return
	}

	// Build ID maps
	locationIDs := make(map[string]bool)
	for _, loc := range dsl.World.Locations {
		locationIDs[loc.ID] = true
	}

	enemyIDs := make(map[string]bool)
	for _, enemy := range dsl.Characters.Enemies {
		enemyIDs[enemy.ID] = true
	}

	npcIDs := make(map[string]bool)
	for _, npc := range dsl.Characters.NPCs {
		npcIDs[npc.ID] = true
	}

	itemIDs := make(map[string]bool)
	for _, item := range dsl.World.Items {
		itemIDs[item.ID] = true
	}

	// Validate references in events
	for _, chapter := range dsl.Storyline.Chapters {
		for _, obj := range chapter.Objectives {
			for _, step := range obj.Steps {
				event := step.Event

				// Validate spawn locations
				if event.Spawn != nil && event.Spawn.Location != "" {
					if !locationIDs[event.Spawn.Location] {
						v.addError(fmt.Sprintf("chapter.%s.step.%d", chapter.ID, step.Order),
							fmt.Sprintf("undefined location: %s", event.Spawn.Location))
					}
				}

				// Validate move destinations
				if event.Move != nil && event.Move.To != "" {
					if !locationIDs[event.Move.To] {
						v.addError(fmt.Sprintf("chapter.%s.step.%d", chapter.ID, step.Order),
							fmt.Sprintf("undefined location: %s", event.Move.To))
					}
				}

				// Validate combat enemies
				if event.Combat != nil {
					for _, enemy := range event.Combat.Setup.Enemies {
						if !enemyIDs[enemy.ID] {
							v.addWarning(fmt.Sprintf("chapter.%s.step.%d", chapter.ID, step.Order),
								fmt.Sprintf("undefined enemy: %s (will be treated as generic)", enemy.ID))
						}
					}
				}
			}
		}
	}
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
