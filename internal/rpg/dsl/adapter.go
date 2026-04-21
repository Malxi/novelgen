package dsl

import (
	"fmt"
	"path/filepath"
	"strings"

	"novelgen/internal/rpg"
)

// NovelgenAdapter converts novelgen project data to DSL
type NovelgenAdapter struct {
	project  *rpg.NovelgenProject
	logger   *Logger
}

// NewNovelgenAdapter creates a new adapter
func NewNovelgenAdapter(project *rpg.NovelgenProject, logger *Logger) *NovelgenAdapter {
	if logger == nil {
		logger = NewConsoleLogger(WithMinLevel(LogLevelInfo))
	}
	return &NovelgenAdapter{
		project: project,
		logger:  logger,
	}
}

// ToDSL converts the novelgen project to DSL
func (na *NovelgenAdapter) ToDSL(phase MergePhase) (*DSL, error) {
	na.logger.Info(LogCategorySystem, "Converting novelgen project to DSL",
		map[string]interface{}{
			"book":  na.project.BookName,
			"phase": phase,
		})

	dsl := &DSL{
		Metadata:   &Metadata{},
		World:      &World{},
		Characters: &Characters{},
		Storyline:  &Storyline{},
		Systems:    &Systems{},
	}

	// Set metadata
	dsl.Metadata.Title = na.project.BookName
	dsl.Metadata.DSLVersion = "0.2.0"
	dsl.Metadata.Phase = string(phase)

	switch phase {
	case PhaseOutline:
		return na.toOutlineDSL(dsl)
	case PhaseCraft:
		return na.toCraftDSL(dsl)
	default:
		return nil, fmt.Errorf("unsupported phase: %s", phase)
	}
}

// toOutlineDSL creates outline phase DSL (basic framework)
func (na *NovelgenAdapter) toOutlineDSL(dsl *DSL) (*DSL, error) {
	na.logger.Info(LogCategorySystem, "Generating outline DSL")

	// Convert characters (placeholders only)
	for name, char := range na.project.Characters {
		npc := NPC{
			ID:                sanitizeID(name),
			Name:              name,
			Role:              char.RoleInStory,
			IsPlaceholder:     true,
			PlaceholderSource: "outline",
		}
		dsl.Characters.NPCs = append(dsl.Characters.NPCs, npc)
	}

	// Convert locations (placeholders only)
	for name, loc := range na.project.Locations {
		location := Location{
			ID:                sanitizeID(name),
			Name:              name,
			Type:              inferMapTypeFromName(name),
			IsPlaceholder:     true,
			PlaceholderSource: "outline",
		}
		
		// Add connections
		for _, connected := range loc.ConnectedLocs {
			location.Connections = append(location.Connections, Connection{
				To: sanitizeID(connected),
			})
		}
		
		dsl.World.Locations = append(dsl.World.Locations, location)
	}

	// Convert outline to storyline
	if err := na.convertOutlineToStoryline(dsl); err != nil {
		return nil, err
	}

	return dsl, nil
}

// toCraftDSL creates craft phase DSL (detailed information)
func (na *NovelgenAdapter) toCraftDSL(dsl *DSL) (*DSL, error) {
	na.logger.Info(LogCategorySystem, "Generating craft DSL")

	// Convert characters with full details
	for name, char := range na.project.Characters {
		player := na.convertNovelgenCharacterToPlayer(name, char)
		
		// Only set player if it's the protagonist
		if char.RoleInStory == "主角" || char.RoleInStory == "protagonist" {
			dsl.Characters.Player = player
			// Don't add protagonist as NPC
			continue
		}
		
		// Otherwise add as NPC
		npc := na.convertPlayerToNPC(player)
		npc.IsPlaceholder = false
		dsl.Characters.NPCs = append(dsl.Characters.NPCs, npc)
	}

	// Convert locations with full details
	for name, loc := range na.project.Locations {
		location := na.convertNovelgenLocationToLocation(name, loc)
		dsl.World.Locations = append(dsl.World.Locations, location)
	}

	// Convert items
	for name, item := range na.project.Items {
		rpgItem := na.convertNovelgenItemToItem(name, item)
		dsl.World.Items = append(dsl.World.Items, rpgItem)
	}

	return dsl, nil
}

// convertOutlineToStoryline converts the story outline to DSL storyline
func (na *NovelgenAdapter) convertOutlineToStoryline(dsl *DSL) error {
	outline := na.project.Outline

	// Convert parts/volumes/chapters
	partNum := 1
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				dslChapter := Chapter{
					ID:       chapter.ID,
					Title:    chapter.Title,
					Arc:      volume.ID,
					Position: partNum,
				}

				// Convert objectives from chapter content
				objective := Objective{
					ID:   fmt.Sprintf("obj_%s", chapter.ID),
					Name: chapter.Title,
					Type: "sequence",
				}

				// Convert events to steps
				stepNum := 1
				for _, event := range chapter.Events {
					step := na.convertEventToStep(event, stepNum)
					if step != nil {
						objective.Steps = append(objective.Steps, *step)
						stepNum++
					}
				}

				if len(objective.Steps) > 0 {
					dslChapter.Objectives = append(dslChapter.Objectives, objective)
				}

				dsl.Storyline.Chapters = append(dsl.Storyline.Chapters, dslChapter)
				partNum++
			}
		}
	}

	return nil
}

// convertEventToStep converts an outline event to a storyline step
func (na *NovelgenAdapter) convertEventToStep(event rpg.StoryEvent, order int) *Step {
	step := &Step{
		Order:       order,
		Description: event.Result,
	}

	// Determine event type and create appropriate event
	switch event.Type {
	case "combat", "battle":
		step.Event = Event{
			Type: "combat",
			Combat: &CombatEvent{
				Setup: CombatSetup{
					Location: event.Context,
				},
			},
		}

	case "dialogue", "talk":
		step.Event = Event{
			Type: "dialogue",
			Dialogue: &DialogueEvent{
				Speaker: event.Actor,
				Text:    event.Result,
			},
		}

	case "acquire", "item":
		step.Event = Event{
			Type: "acquire",
			Acquire: &AcquireEvent{
				Actor:  event.Actor,
				Item:   event.Target,
				Source: event.Context,
			},
		}

	case "move", "reach":
		step.Event = Event{
			Type: "move",
			Move: &MoveEvent{
				Actor: event.Actor,
				To:    event.Target,
			},
		}

	case "status":
		step.Event = Event{
			Type: "status",
		}

	default:
		// Generic event
		step.Event = Event{
			Type: event.Type,
		}
	}

	return step
}

// convertNovelgenCharacterToPlayer converts novelgen character to DSL player
func (na *NovelgenAdapter) convertNovelgenCharacterToPlayer(name string, char rpg.NovelgenCharacter) *Player {
	player := &Player{
		ID:                sanitizeID(name),
		Name:              name,
		Description:       char.Background,
		Age:               parseAge(char.Age),
		Gender:            char.Gender,
		Race:              char.Race,
		Background:        char.Background,
		Personality:       char.Personality,
		Motivation:        char.Motivation,
		Abilities:         char.Abilities,
		Affiliations:      char.Affiliations,
		RoleInStory:       char.RoleInStory,
		Voice:             char.Voice,
		IsPlaceholder:     false,
		PlaceholderSource: "",
		Class:             inferClassFromCharacter(char),
		Skills:            char.Skills,
		Stats:             inferStatsFromCharacter(char),
		Traits:            make(map[string]Trait),
	}

	// Convert skills to traits
	for _, skill := range char.Skills {
		player.Traits[sanitizeID(skill)] = Trait{
			Unlocked: true,
			Trigger:  "passive",
		}
	}

	return player
}

// convertPlayerToNPC converts a player struct to NPC
func (na *NovelgenAdapter) convertPlayerToNPC(player *Player) NPC {
	return NPC{
		ID:              player.ID,
		Name:            player.Name,
		Role:            player.RoleInStory,
		Description:     player.Description,
		Age:             player.Age,
		Gender:          player.Gender,
		Appearance:      "", // Would need separate field in novelgen
		Background:      player.Background,
		Personality:     player.Personality,
		Affiliations:    player.Affiliations,
		IsPlaceholder:   player.IsPlaceholder,
	}
}

// convertNovelgenLocationToLocation converts novelgen location to DSL location
func (na *NovelgenAdapter) convertNovelgenLocationToLocation(name string, loc rpg.NovelgenLocation) Location {
	location := Location{
		ID:             sanitizeID(name),
		Name:           name,
		Type:           inferMapTypeFromLocation(loc),
		Description:    loc.Description,
		Appearance:     loc.Appearance,
		Atmosphere:     loc.Atmosphere,
		History:        loc.History,
		Secrets:        loc.Secrets,
		SensoryDetails: loc.SensoryDetails,
		Inhabitants:    loc.Inhabitants,
		Events:         loc.Events,
		IsPlaceholder:  false,
	}

	// Add connections
	for _, connected := range loc.ConnectedLocs {
		location.Connections = append(location.Connections, Connection{
			To: sanitizeID(connected),
		})
	}

	return location
}

// convertNovelgenItemToItem converts novelgen item to DSL item
func (na *NovelgenAdapter) convertNovelgenItemToItem(name string, item rpg.NovelgenItem) Item {
	return Item{
		ID:          sanitizeID(name),
		Name:        name,
		Description: item.Description,
		Type:        inferItemTypeFromNovelgen(item),
		Rarity:      inferRarityFromSignificance(item.Significance),
		Effects:     convertPowersToEffects(item.Powers),
	}
}

// ExportToDSLFiles exports the project to DSL files
func (na *NovelgenAdapter) ExportToDSLFiles(outputDir string) error {
	na.logger.Info(LogCategorySystem, "Exporting to DSL files",
		map[string]interface{}{"output_dir": outputDir})

	// Generate outline DSL
	outlineDSL, err := na.ToDSL(PhaseOutline)
	if err != nil {
		return fmt.Errorf("failed to generate outline DSL: %w", err)
	}

	outlinePath := filepath.Join(outputDir, "01_outline.rpg")
	if err := na.writeDSLToFile(outlineDSL, outlinePath); err != nil {
		return fmt.Errorf("failed to write outline DSL: %w", err)
	}

	// Generate craft DSL
	craftDSL, err := na.ToDSL(PhaseCraft)
	if err != nil {
		return fmt.Errorf("failed to generate craft DSL: %w", err)
	}

	craftPath := filepath.Join(outputDir, "02_craft.rpg")
	if err := na.writeDSLToFile(craftDSL, craftPath); err != nil {
		return fmt.Errorf("failed to write craft DSL: %w", err)
	}

	na.logger.Info(LogCategorySystem, "DSL export completed",
		map[string]interface{}{
			"outline_file": outlinePath,
			"craft_file":   craftPath,
		})

	return nil
}

// writeDSLToFile writes DSL to a file
func (na *NovelgenAdapter) writeDSLToFile(dsl *DSL, path string) error {
	na.logger.Info(LogCategorySystem, "Writing DSL file",
		map[string]interface{}{"path": path})

	return dsl.WriteToFile(path)
}

// Helper functions

func sanitizeID(name string) string {
	// Convert name to valid ID
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, "'", "")
	id = strings.ReplaceAll(id, `"`, "")
	return id
}

func parseAge(age string) int {
	// Parse age string to int
	// Simplified version
	var result int
	fmt.Sscanf(age, "%d", &result)
	return result
}

func inferClassFromCharacter(char rpg.NovelgenCharacter) string {
	// Infer RPG class from character info
	for _, skill := range char.Skills {
		skillLower := strings.ToLower(skill)
		if strings.Contains(skillLower, "剑") || strings.Contains(skillLower, "格斗") {
			return "warrior"
		}
		if strings.Contains(skillLower, "法") || strings.Contains(skillLower, "术") {
			return "mage"
		}
		if strings.Contains(skillLower, "弓") || strings.Contains(skillLower, "射") {
			return "archer"
		}
	}
	return "adventurer"
}

func inferStatsFromCharacter(char rpg.NovelgenCharacter) Stats {
	// Infer stats from character skills and abilities
	stats := Stats{
		HP:  100,
		MP:  50,
		STR: 10,
		AGI: 10,
		INT: 10,
		VIT: 10,
	}

	// Adjust based on skills
	for _, skill := range char.Skills {
		skillLower := strings.ToLower(skill)
		if strings.Contains(skillLower, "力") || strings.Contains(skillLower, "剑") {
			stats.STR += 2
		}
		if strings.Contains(skillLower, "速") || strings.Contains(skillLower, "轻") {
			stats.AGI += 2
		}
		if strings.Contains(skillLower, "智") || strings.Contains(skillLower, "法") {
			stats.INT += 2
		}
	}

	// Adjust based on role
	if char.RoleInStory == "主角" {
		stats.HP += 20
		stats.MP += 10
	}

	return stats
}

func inferMapTypeFromName(name string) string {
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "城") || strings.Contains(nameLower, "镇") || strings.Contains(nameLower, "据点") {
		return "city"
	}
	if strings.Contains(nameLower, "洞") || strings.Contains(nameLower, "穴") || strings.Contains(nameLower, "地下") {
		return "dungeon"
	}
	if strings.Contains(nameLower, "林") || strings.Contains(nameLower, "森") {
		return "forest"
	}
	return "field"
}

func inferMapTypeFromLocation(loc rpg.NovelgenLocation) string {
	nameLower := strings.ToLower(loc.Name)
	if strings.Contains(nameLower, "城") || strings.Contains(nameLower, "镇") || strings.Contains(nameLower, "据点") {
		return "city"
	}
	if strings.Contains(nameLower, "洞") || strings.Contains(nameLower, "穴") || strings.Contains(nameLower, "地下") {
		return "dungeon"
	}
	if strings.Contains(nameLower, "林") || strings.Contains(nameLower, "森") {
		return "forest"
	}
	return "field"
}

func inferItemTypeFromNovelgen(item rpg.NovelgenItem) string {
	typeLower := strings.ToLower(item.Type)
	if strings.Contains(typeLower, "消耗") || strings.Contains(typeLower, "药") {
		return "consumable"
	}
	if strings.Contains(typeLower, "材料") {
		return "material"
	}
	if strings.Contains(typeLower, "装备") || strings.Contains(typeLower, "武器") {
		return "equipment"
	}
	return "misc"
}

func inferRarityFromSignificance(significance string) string {
	sigLower := strings.ToLower(significance)
	if strings.Contains(sigLower, "核心") || strings.Contains(sigLower, "关键") {
		return "legendary"
	}
	if strings.Contains(sigLower, "重要") {
		return "epic"
	}
	return "common"
}

func convertPowersToEffects(powers []string) map[string]interface{} {
	effects := make(map[string]interface{})
	for _, power := range powers {
		effects[sanitizeID(power)] = power
	}
	return effects
}
