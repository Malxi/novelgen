package dsl

import (
	"fmt"

	"novelgen/internal/rpg"
)

// Converter converts DSL AST to RPG World
type Converter struct {
	world   *rpg.GameWorld
	dsl     *DSL
	charMap map[string]string // DSL char ID -> RPG char ID
	locMap  map[string]string // DSL loc ID -> RPG map ID
	itemMap map[string]string // DSL item ID -> RPG item ID
}

// NewConverter creates a new converter
func NewConverter() *Converter {
	return &Converter{
		world:   rpg.NewGameWorld(),
		charMap: make(map[string]string),
		locMap:  make(map[string]string),
		itemMap: make(map[string]string),
	}
}

// Convert converts DSL to RPG World
func (c *Converter) Convert(dsl *DSL) (*rpg.GameWorld, error) {
	c.dsl = dsl

	// 1. Convert items
	if err := c.convertItems(); err != nil {
		return nil, fmt.Errorf("failed to convert items: %w", err)
	}

	// 2. Convert maps/locations
	if err := c.convertLocations(); err != nil {
		return nil, fmt.Errorf("failed to convert locations: %w", err)
	}

	// 3. Convert characters (templates)
	if err := c.convertCharacters(); err != nil {
		return nil, fmt.Errorf("failed to convert characters: %w", err)
	}

	// 4. Convert quests from storyline
	if err := c.convertStoryline(); err != nil {
		return nil, fmt.Errorf("failed to convert storyline: %w", err)
	}

	// 5. Set player
	if err := c.setupPlayer(); err != nil {
		return nil, fmt.Errorf("failed to setup player: %w", err)
	}

	return c.world, nil
}

// convertItems converts DSL items to RPG items
func (c *Converter) convertItems() error {
	if c.dsl.World == nil {
		return nil
	}

	for _, dslItem := range c.dsl.World.Items {
		item := &rpg.Item{
			ID:          dslItem.ID,
			Name:        dslItem.Name,
			Description: "", // DSL Item doesn't have Description field in MVP
			Type:        c.mapItemType(dslItem.Type),
			Rarity:      c.mapRarity(dslItem.Rarity),
			Weight:      1.0,
			MaxStack:    99,
			Value:       10,
			IsUsable:    false,
			IsDroppable: true,
			IsSellable:  true,
		}

		c.world.Items.AddItem(item)
		c.itemMap[dslItem.ID] = item.ID
	}

	return nil
}

// convertLocations converts DSL locations to RPG maps
func (c *Converter) convertLocations() error {
	if c.dsl.World == nil {
		return nil
	}

	for _, dslLoc := range c.dsl.World.Locations {
		gameMap := &rpg.Map{
			ID:          dslLoc.ID,
			Name:        dslLoc.Name,
			Description: dslLoc.Description,
			Type:        c.mapLocationType(dslLoc.Type),
			Width:       20,
			Height:      20,
			TileSize:    32,
			Entities:    make([]rpg.MapEntity, 0),
			Teleports:   make([]rpg.TeleportPoint, 0),
			Connections: make([]rpg.MapConnection, 0),
			LightLevel:  100,
		}

		// Add connections
		for _, conn := range dslLoc.Connections {
			gameMap.Connections = append(gameMap.Connections, rpg.MapConnection{
				Direction: conn.Direction,
				MapID:     conn.To,
			})
		}

		c.world.Maps.AddMap(gameMap)
		c.locMap[dslLoc.ID] = gameMap.ID
	}

	return nil
}

// convertCharacters converts DSL characters to RPG character templates
func (c *Converter) convertCharacters() error {
	if c.dsl.Characters == nil {
		return nil
	}

	// Convert enemies
	for _, dslEnemy := range c.dsl.Characters.Enemies {
		template := &rpg.CharacterTemplate{
			ID:   dslEnemy.ID,
			Name: dslEnemy.Name,
			Type: rpg.CharacterTypeEnemy,
			BaseStats: rpg.BaseStats{
				HP:     50,
				MP:     10,
				Attack: dslEnemy.Template.StatsPerLevel["str"],
				Speed:  dslEnemy.Template.StatsPerLevel["agi"],
			},
			GrowthStats: rpg.GrowthStats{
				HP:     10,
				MP:     2,
				Attack: float64(dslEnemy.Template.StatsPerLevel["str"]),
				Speed:  float64(dslEnemy.Template.StatsPerLevel["agi"]),
			},
			DropItems: make([]rpg.DropItem, 0),
		}

		// Parse drops
		for _, drop := range dslEnemy.Drops.Fixed {
			template.DropItems = append(template.DropItems, rpg.DropItem{
				ItemID:   drop.Item,
				Chance:   1.0,
				MinCount: drop.Min,
				MaxCount: drop.Max,
			})
		}
		for _, drop := range dslEnemy.Drops.Random {
			template.DropItems = append(template.DropItems, rpg.DropItem{
				ItemID:   drop.Item,
				Chance:   drop.Chance,
				MinCount: drop.Min,
				MaxCount: drop.Max,
			})
		}

		c.world.Characters.AddTemplate(template)
		c.charMap[dslEnemy.ID] = template.ID
	}

	// Convert NPCs
	for _, dslNPC := range c.dsl.Characters.NPCs {
		template := &rpg.CharacterTemplate{
			ID:   dslNPC.ID,
			Name: dslNPC.Name,
			Type: rpg.CharacterTypeNPC,
			BaseStats: rpg.BaseStats{
				HP:     50,
				MP:     20,
				Attack: 5,
				Speed:  5,
			},
			DialogueID: dslNPC.ID + "_dialogue",
		}

		c.world.Characters.AddTemplate(template)
		c.charMap[dslNPC.ID] = template.ID
	}

	return nil
}

// convertStoryline converts DSL storyline to RPG quests
func (c *Converter) convertStoryline() error {
	if c.dsl.Storyline == nil {
		return nil
	}

	for _, dslChapter := range c.dsl.Storyline.Chapters {
		quest := &rpg.Quest{
			ID:          dslChapter.ID,
			Name:        dslChapter.Title,
			Description: "", // Could be extracted from first objective
			Type:        rpg.QuestTypeMain,
			Objectives:  make([]rpg.QuestObjective, 0),
		}

		// Convert objectives
		for _, dslObj := range dslChapter.Objectives {
			for _, dslStep := range dslObj.Steps {
				objective := c.convertStepToObjective(&dslStep, dslChapter.ID)
				quest.Objectives = append(quest.Objectives, objective)
			}
		}

		c.world.Quests.AddQuest(quest)
	}

	return nil
}

// convertStepToObjective converts a DSL step to RPG quest objective
func (c *Converter) convertStepToObjective(step *Step, chapterID string) rpg.QuestObjective {
	obj := rpg.QuestObjective{
		ID:          fmt.Sprintf("%s_step_%d", chapterID, step.Order),
		Description: step.Description,
		LocationID:  "",
		TargetCount: 1,
	}

	switch step.Event.Type {
	case "spawn":
		obj.Type = rpg.ObjectiveTalk
		if step.Event.Spawn != nil {
			obj.TargetID = step.Event.Spawn.Location
		}

	case "move":
		obj.Type = rpg.ObjectiveReach
		if step.Event.Move != nil {
			obj.TargetID = step.Event.Move.To
		}

	case "combat":
		obj.Type = rpg.ObjectiveDefeat
		if step.Event.Combat != nil && len(step.Event.Combat.Setup.Enemies) > 0 {
			obj.TargetID = step.Event.Combat.Setup.Enemies[0].ID
			obj.TargetCount = step.Event.Combat.Setup.Enemies[0].Count
		}

	case "dialogue":
		obj.Type = rpg.ObjectiveTalk

	case "acquire":
		obj.Type = rpg.ObjectiveCollect
		if step.Event.Acquire != nil {
			obj.TargetID = step.Event.Acquire.Item
			obj.TargetCount = step.Event.Acquire.Quantity
		}

	default:
		obj.Type = rpg.ObjectiveTalk
	}

	return obj
}

// setupPlayer creates and sets up the player character
func (c *Converter) setupPlayer() error {
	if c.dsl.Characters == nil || c.dsl.Characters.Player == nil {
		return nil
	}

	dslPlayer := c.dsl.Characters.Player

	// Create character template for player
	template := &rpg.CharacterTemplate{
		ID:   dslPlayer.ID,
		Name: dslPlayer.Name,
		Type: rpg.CharacterTypePlayer,
		BaseStats: rpg.BaseStats{
			HP:     dslPlayer.Stats.HP,
			MP:     dslPlayer.Stats.MP,
			Attack: dslPlayer.Stats.STR * 2, // Convert STR to Attack
			Speed:  dslPlayer.Stats.AGI,     // AGI becomes Speed
		},
		GrowthStats: rpg.GrowthStats{
			HP:     10,
			MP:     5,
			Attack: 2,
			Speed:  2,
		},
	}

	c.world.Characters.AddTemplate(template)

	// Create player instance
	player := c.world.Characters.CreateCharacter(dslPlayer.ID, dslPlayer.Name)
	if player == nil {
		return fmt.Errorf("failed to create player character")
	}

	// Set player stats
	player.BaseStats.HP = dslPlayer.Stats.HP
	player.BaseStats.MP = dslPlayer.Stats.MP
	player.BaseStats.Attack = dslPlayer.Stats.STR * 2
	player.BaseStats.Speed = dslPlayer.Stats.AGI
	player.CurrentStats = player.BaseStats

	// Add skills
	for _, skillID := range dslPlayer.Skills {
		player.Skills = append(player.Skills, skillID)
	}

	// Set as world player
	c.world.SetPlayer(player)
	c.charMap[dslPlayer.ID] = player.ID

	return nil
}

// Helper functions for type mapping

func (c *Converter) mapItemType(dslType string) rpg.ItemType {
	switch dslType {
	case "material":
		return rpg.ItemTypeMaterial
	case "consumable":
		return rpg.ItemTypeConsumable
	case "quest":
		return rpg.ItemTypeQuest
	default:
		return rpg.ItemTypeMaterial
	}
}

func (c *Converter) mapRarity(dslRarity string) rpg.Rarity {
	switch dslRarity {
	case "common":
		return rpg.RarityCommon
	case "uncommon":
		return rpg.RarityUncommon
	case "rare":
		return rpg.RarityRare
	case "epic":
		return rpg.RarityEpic
	case "legendary":
		return rpg.RarityLegendary
	default:
		return rpg.RarityCommon
	}
}

func (c *Converter) mapLocationType(dslType string) rpg.MapType {
	switch dslType {
	case "indoor":
		return rpg.MapTypeCave
	case "outdoor":
		return rpg.MapTypeField
	case "dungeon":
		return rpg.MapTypeDungeon
	case "city":
		return rpg.MapTypeTown
	default:
		return rpg.MapTypeField
	}
}