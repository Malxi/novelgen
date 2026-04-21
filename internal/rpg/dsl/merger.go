package dsl

import (
	"fmt"
	"strings"
)

// MergePhase represents the phase of DSL generation
type MergePhase string

const (
	PhaseOutline MergePhase = "outline"
	PhaseCraft   MergePhase = "craft"
	PhaseSystems MergePhase = "systems"
	PhaseFinal   MergePhase = "final"
)

// PlaceholderMarker marks an element as waiting for detailed information
type PlaceholderMarker struct {
	IsPlaceholder bool   `json:"__placeholder__"`
	SourcePhase   string `json:"__source_phase__"`
}

// DSLMerger merges multiple DSL fragments into a complete DSL
type DSLMerger struct {
	fragments []*DSLFragment
	logger    *Logger
}

// DSLFragment represents a single DSL file with metadata
type DSLFragment struct {
	DSL      *DSL
	Phase    MergePhase
	FilePath string
	Priority int // Higher priority wins in conflicts
}

// MergeResult contains the result of merging
type MergeResult struct {
	DSL             *DSL
	Placeholders    []PlaceholderInfo
	Conflicts       []MergeConflict
	Warnings        []string
	PhasesMerged    []MergePhase
}

// PlaceholderInfo tracks unfilled placeholders
type PlaceholderInfo struct {
	Type       string // "character", "location", "item"
	ID         string
	Name       string
	SourceFile string
}

// MergeConflict records when two fragments define the same element differently
type MergeConflict struct {
	ElementType string
	ElementID   string
	Field       string
	Value1      interface{}
	Value2      interface{}
	Resolved    bool
	Resolution  string
}

// NewDSLMerger creates a new DSL merger
func NewDSLMerger(logger *Logger) *DSLMerger {
	if logger == nil {
		logger = NewConsoleLogger(WithMinLevel(LogLevelInfo))
	}
	return &DSLMerger{
		fragments: make([]*DSLFragment, 0),
		logger:    logger,
	}
}

// AddFragment adds a DSL fragment to the merge queue
func (dm *DSLMerger) AddFragment(dsl *DSL, phase MergePhase, filePath string) {
	// Determine priority based on phase
	priority := 0
	switch phase {
	case PhaseOutline:
		priority = 1
	case PhaseCraft:
		priority = 2
	case PhaseSystems:
		priority = 3
	}

	fragment := &DSLFragment{
		DSL:      dsl,
		Phase:    phase,
		FilePath: filePath,
		Priority: priority,
	}
	dm.fragments = append(dm.fragments, fragment)

	dm.logger.Info(LogCategorySystem, "Added DSL fragment",
		map[string]interface{}{
			"phase":     phase,
			"file":      filePath,
			"priority":  priority,
			"fragments": len(dm.fragments),
		})
}

// Merge combines all fragments into a single DSL
func (dm *DSLMerger) Merge() (*MergeResult, error) {
	if len(dm.fragments) == 0 {
		return nil, fmt.Errorf("no DSL fragments to merge")
	}

	result := &MergeResult{
		DSL: &DSL{
			Metadata:   &Metadata{},
			World:      &World{},
			Characters: &Characters{},
			Storyline:  &Storyline{},
			Systems:    &Systems{},
		},
		Placeholders: make([]PlaceholderInfo, 0),
		Conflicts:    make([]MergeConflict, 0),
		Warnings:     make([]string, 0),
		PhasesMerged: make([]MergePhase, 0),
	}

	dm.logger.Info(LogCategorySystem, "Starting DSL merge",
		map[string]interface{}{"fragment_count": len(dm.fragments)})

	// Sort fragments by priority (ascending, so higher priority comes later and overrides)
	dm.sortFragmentsByPriority()

	// Merge each fragment
	for _, fragment := range dm.fragments {
		dm.logger.Info(LogCategorySystem, "Merging fragment",
			map[string]interface{}{
				"phase": fragment.Phase,
				"file":  fragment.FilePath,
			})

		if err := dm.mergeFragment(result, fragment); err != nil {
			return nil, fmt.Errorf("failed to merge fragment %s: %w", fragment.FilePath, err)
		}

		result.PhasesMerged = append(result.PhasesMerged, fragment.Phase)
	}

	// Validate placeholders
	dm.validatePlaceholders(result)

	// Log summary
	dm.logger.Info(LogCategorySystem, "DSL merge completed",
		map[string]interface{}{
			"placeholders": len(result.Placeholders),
			"conflicts":    len(result.Conflicts),
			"warnings":     len(result.Warnings),
		})

	return result, nil
}

// sortFragmentsByPriority sorts fragments by priority (ascending)
func (dm *DSLMerger) sortFragmentsByPriority() {
	// Simple bubble sort (sufficient for small number of fragments)
	n := len(dm.fragments)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if dm.fragments[j].Priority > dm.fragments[j+1].Priority {
				dm.fragments[j], dm.fragments[j+1] = dm.fragments[j+1], dm.fragments[j]
			}
		}
	}
}

// mergeFragment merges a single fragment into the result
func (dm *DSLMerger) mergeFragment(result *MergeResult, fragment *DSLFragment) error {
	switch fragment.Phase {
	case PhaseOutline:
		return dm.mergeOutline(result, fragment)
	case PhaseCraft:
		return dm.mergeCraft(result, fragment)
	case PhaseSystems:
		return dm.mergeSystems(result, fragment)
	default:
		return fmt.Errorf("unknown merge phase: %s", fragment.Phase)
	}
}

// mergeOutline merges outline phase DSL (basic framework)
func (dm *DSLMerger) mergeOutline(result *MergeResult, fragment *DSLFragment) error {
	dsl := fragment.DSL

	// Merge metadata (outline provides base info)
	if dsl.Metadata != nil {
		result.DSL.Metadata.Title = dm.coalesceString(result.DSL.Metadata.Title, dsl.Metadata.Title)
		result.DSL.Metadata.Subtitle = dm.coalesceString(result.DSL.Metadata.Subtitle, dsl.Metadata.Subtitle)
		result.DSL.Metadata.Genre = dm.coalesceSlice(result.DSL.Metadata.Genre, dsl.Metadata.Genre)
		result.DSL.Metadata.PowerSystem = dm.coalesceString(result.DSL.Metadata.PowerSystem, dsl.Metadata.PowerSystem)
		result.DSL.Metadata.Tone = dm.coalesceString(result.DSL.Metadata.Tone, dsl.Metadata.Tone)
		result.DSL.Metadata.DSLVersion = dm.coalesceString(result.DSL.Metadata.DSLVersion, dsl.Metadata.DSLVersion)
	}

	// Merge character placeholders
	if dsl.Characters != nil {
		// Player placeholder
		if dsl.Characters.Player != nil {
			result.DSL.Characters.Player = dm.createPlaceholderCharacter(dsl.Characters.Player, fragment.Phase)
		}

		// Enemy placeholders
		for _, enemy := range dsl.Characters.Enemies {
			placeholder := dm.createPlaceholderEnemy(enemy, fragment.Phase)
			result.DSL.Characters.Enemies = dm.mergeEnemyList(result.DSL.Characters.Enemies, placeholder)
		}

		// NPC placeholders
		for _, npc := range dsl.Characters.NPCs {
			placeholder := dm.createPlaceholderNPC(npc, fragment.Phase)
			result.DSL.Characters.NPCs = dm.mergeNPCList(result.DSL.Characters.NPCs, placeholder)
		}
	}

	// Merge location placeholders
	if dsl.World != nil {
		for _, loc := range dsl.World.Locations {
			placeholder := dm.createPlaceholderLocation(loc, fragment.Phase)
			result.DSL.World.Locations = dm.mergeLocationList(result.DSL.World.Locations, placeholder)
		}

		// Items (outline may have basic item references)
		for _, item := range dsl.World.Items {
			result.DSL.World.Items = dm.mergeItemList(result.DSL.World.Items, item)
		}
	}

	// Storyline (outline has the most complete structure)
	if dsl.Storyline != nil {
		result.DSL.Storyline = dsl.Storyline
	}

	return nil
}

// mergeCraft merges craft phase DSL (detailed information)
func (dm *DSLMerger) mergeCraft(result *MergeResult, fragment *DSLFragment) error {
	dsl := fragment.DSL

	// Merge detailed characters (fill placeholders)
	if dsl.Characters != nil {
		// Player details
		if dsl.Characters.Player != nil {
			if result.DSL.Characters.Player != nil && result.DSL.Characters.Player.IsPlaceholder {
				// Fill the placeholder
				dm.fillCharacterPlaceholder(result.DSL.Characters.Player, dsl.Characters.Player)
			} else {
				// Add as new or update existing
				result.DSL.Characters.Player = dsl.Characters.Player
			}
		}

		// Enemy details
		for _, enemy := range dsl.Characters.Enemies {
			result.DSL.Characters.Enemies = dm.mergeEnemyWithDetails(
				result.DSL.Characters.Enemies,
				enemy,
				result,
			)
		}

		// NPC details
		for _, npc := range dsl.Characters.NPCs {
			result.DSL.Characters.NPCs = dm.mergeNPCWithDetails(
				result.DSL.Characters.NPCs,
				npc,
				result,
			)
		}
	}

	// Merge detailed locations
	if dsl.World != nil {
		for _, loc := range dsl.World.Locations {
			result.DSL.World.Locations = dm.mergeLocationWithDetails(
				result.DSL.World.Locations,
				loc,
				result,
			)
		}

		// Merge detailed items
		for _, item := range dsl.World.Items {
			result.DSL.World.Items = dm.mergeItemWithDetails(
				result.DSL.World.Items,
				item,
				result,
			)
		}
	}

	return nil
}

// mergeSystems merges systems phase DSL (game mechanics)
func (dm *DSLMerger) mergeSystems(result *MergeResult, fragment *DSLFragment) error {
	dsl := fragment.DSL

	if dsl.Systems != nil {
		// Merge progression systems (new format)
		for _, prog := range dsl.Systems.ProgressionSystems {
			result.DSL.Systems.ProgressionSystems = dm.mergeProgressionSystemList(
				result.DSL.Systems.ProgressionSystems,
				prog,
			)
		}

		// Merge counters
		for _, counter := range dsl.Systems.Counters {
			result.DSL.Systems.Counters = dm.mergeCounterList(
				result.DSL.Systems.Counters,
				counter,
			)
		}
	}

	return nil
}

// Helper methods for merging

func (dm *DSLMerger) createPlaceholderCharacter(char *Player, phase MergePhase) *Player {
	placeholder := &Player{
		Name:          char.Name,
		ID:            char.ID,
		IsPlaceholder: true,
		PlaceholderSource: string(phase),
	}
	return placeholder
}

func (dm *DSLMerger) createPlaceholderEnemy(enemy Enemy, phase MergePhase) Enemy {
	enemy.IsPlaceholder = true
	enemy.PlaceholderSource = string(phase)
	return enemy
}

func (dm *DSLMerger) createPlaceholderNPC(npc NPC, phase MergePhase) NPC {
	npc.IsPlaceholder = true
	npc.PlaceholderSource = string(phase)
	return npc
}

func (dm *DSLMerger) createPlaceholderLocation(loc Location, phase MergePhase) Location {
	loc.IsPlaceholder = true
	loc.PlaceholderSource = string(phase)
	return loc
}

func (dm *DSLMerger) fillCharacterPlaceholder(target, source *Player) {
	// Preserve ID and Name from outline
	id := target.ID
	name := target.Name

	// Copy all fields from craft
	*target = *source

	// Restore ID and Name (in case craft had different ones)
	target.ID = id
	target.Name = name

	// Mark as filled
	target.IsPlaceholder = false
	target.PlaceholderSource = ""
}

func (dm *DSLMerger) mergeEnemyList(enemies []Enemy, newEnemy Enemy) []Enemy {
	for i, e := range enemies {
		if e.ID == newEnemy.ID {
			// Update existing
			enemies[i] = newEnemy
			return enemies
		}
	}
	// Add new
	return append(enemies, newEnemy)
}

func (dm *DSLMerger) mergeNPCList(npcs []NPC, newNPC NPC) []NPC {
	for i, n := range npcs {
		if n.ID == newNPC.ID {
			npcs[i] = newNPC
			return npcs
		}
	}
	return append(npcs, newNPC)
}

func (dm *DSLMerger) mergeLocationList(locs []Location, newLoc Location) []Location {
	for i, l := range locs {
		if l.ID == newLoc.ID {
			locs[i] = newLoc
			return locs
		}
	}
	return append(locs, newLoc)
}

func (dm *DSLMerger) mergeItemList(items []Item, newItem Item) []Item {
	for i, item := range items {
		if item.ID == newItem.ID {
			items[i] = newItem
			return items
		}
	}
	return append(items, newItem)
}

func (dm *DSLMerger) mergeEnemyWithDetails(enemies []Enemy, newEnemy Enemy, result *MergeResult) []Enemy {
	for i, e := range enemies {
		if e.ID == newEnemy.ID {
			if e.IsPlaceholder {
				// Fill placeholder
				enemies[i] = newEnemy
				enemies[i].IsPlaceholder = false
			} else {
				// Record conflict
				result.Conflicts = append(result.Conflicts, MergeConflict{
					ElementType: "enemy",
					ElementID:   e.ID,
					Field:       "full_definition",
					Value1:      "existing",
					Value2:      "new",
					Resolved:    true,
					Resolution:  "new_overrides",
				})
				enemies[i] = newEnemy
			}
			return enemies
		}
	}
	return append(enemies, newEnemy)
}

func (dm *DSLMerger) mergeNPCWithDetails(npcs []NPC, newNPC NPC, result *MergeResult) []NPC {
	for i, n := range npcs {
		if n.ID == newNPC.ID {
			if n.IsPlaceholder {
				npcs[i] = newNPC
				npcs[i].IsPlaceholder = false
			} else {
				result.Conflicts = append(result.Conflicts, MergeConflict{
					ElementType: "npc",
					ElementID:   n.ID,
					Field:       "full_definition",
					Value1:      "existing",
					Value2:      "new",
					Resolved:    true,
					Resolution:  "new_overrides",
				})
				npcs[i] = newNPC
			}
			return npcs
		}
	}
	return append(npcs, newNPC)
}

func (dm *DSLMerger) mergeLocationWithDetails(locs []Location, newLoc Location, result *MergeResult) []Location {
	for i, l := range locs {
		if l.ID == newLoc.ID {
			if l.IsPlaceholder {
				locs[i] = newLoc
				locs[i].IsPlaceholder = false
			} else {
				result.Conflicts = append(result.Conflicts, MergeConflict{
					ElementType: "location",
					ElementID:   l.ID,
					Field:       "full_definition",
					Value1:      "existing",
					Value2:      "new",
					Resolved:    true,
					Resolution:  "new_overrides",
				})
				locs[i] = newLoc
			}
			return locs
		}
	}
	return append(locs, newLoc)
}

func (dm *DSLMerger) mergeItemWithDetails(items []Item, newItem Item, result *MergeResult) []Item {
	for i, item := range items {
		if item.ID == newItem.ID {
			result.Conflicts = append(result.Conflicts, MergeConflict{
				ElementType: "item",
				ElementID:   item.ID,
				Field:       "full_definition",
				Value1:      "existing",
				Value2:      "new",
				Resolved:    true,
				Resolution:  "new_overrides",
			})
			items[i] = newItem
			return items
		}
	}
	return append(items, newItem)
}

func (dm *DSLMerger) mergeProgressionSystemList(progs []ProgressionSystem, newProg ProgressionSystem) []ProgressionSystem {
	for i, p := range progs {
		if p.ID == newProg.ID {
			progs[i] = newProg
			return progs
		}
	}
	return append(progs, newProg)
}

func (dm *DSLMerger) mergeCounterList(counters []CounterSystem, newCounter CounterSystem) []CounterSystem {
	for i, c := range counters {
		if c.Name == newCounter.Name {
			counters[i] = newCounter
			return counters
		}
	}
	return append(counters, newCounter)
}

// Validation

func (dm *DSLMerger) validatePlaceholders(result *MergeResult) {
	// Check character placeholders
	if result.DSL.Characters != nil {
		if result.DSL.Characters.Player != nil && result.DSL.Characters.Player.IsPlaceholder {
			result.Placeholders = append(result.Placeholders, PlaceholderInfo{
				Type:       "character",
				ID:         result.DSL.Characters.Player.ID,
				Name:       result.DSL.Characters.Player.Name,
				SourceFile: "outline",
			})
		}

		for _, enemy := range result.DSL.Characters.Enemies {
			if enemy.IsPlaceholder {
				result.Placeholders = append(result.Placeholders, PlaceholderInfo{
					Type:       "enemy",
					ID:         enemy.ID,
					Name:       enemy.Name,
					SourceFile: "outline",
				})
			}
		}

		for _, npc := range result.DSL.Characters.NPCs {
			if npc.IsPlaceholder {
				result.Placeholders = append(result.Placeholders, PlaceholderInfo{
					Type:       "npc",
					ID:         npc.ID,
					Name:       npc.Name,
					SourceFile: "outline",
				})
			}
		}
	}

	// Check location placeholders
	if result.DSL.World != nil {
		for _, loc := range result.DSL.World.Locations {
			if loc.IsPlaceholder {
				result.Placeholders = append(result.Placeholders, PlaceholderInfo{
					Type:       "location",
					ID:         loc.ID,
					Name:       loc.Name,
					SourceFile: "outline",
				})
			}
		}
	}
}

// Utility methods

func (dm *DSLMerger) coalesceString(current, new string) string {
	if new != "" {
		return new
	}
	return current
}

func (dm *DSLMerger) coalesceSlice(current, new []string) []string {
	if len(new) > 0 {
		return new
	}
	return current
}

// HasUnfilledPlaceholders checks if there are any unfilled placeholders
func (dm *DSLMerger) HasUnfilledPlaceholders(result *MergeResult) bool {
	return len(result.Placeholders) > 0
}

// GetUnfilledPlaceholdersSummary returns a summary of unfilled placeholders
func (dm *DSLMerger) GetUnfilledPlaceholdersSummary(result *MergeResult) string {
	if len(result.Placeholders) == 0 {
		return "All placeholders filled!"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d unfilled placeholders:\n\n", len(result.Placeholders)))

	grouped := make(map[string][]PlaceholderInfo)
	for _, p := range result.Placeholders {
		grouped[p.Type] = append(grouped[p.Type], p)
	}

	for typ, placeholders := range grouped {
		sb.WriteString(fmt.Sprintf("[%s] (%d):\n", typ, len(placeholders)))
		for _, p := range placeholders {
			sb.WriteString(fmt.Sprintf("  - %s (id: %s)\n", p.Name, p.ID))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GeneratePromptForPlaceholders generates AI prompts for unfilled placeholders
func (dm *DSLMerger) GeneratePromptForPlaceholders(result *MergeResult) string {
	if len(result.Placeholders) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("请为以下 RPG 元素生成详细信息（用于 DSL 配置）：\n\n")

	for _, p := range result.Placeholders {
		sb.WriteString(fmt.Sprintf("## %s: %s\n\n", p.Type, p.Name))

		switch p.Type {
		case "character", "enemy", "npc":
			sb.WriteString("请提供以下信息：\n")
			sb.WriteString("- 背景故事 (background)\n")
			sb.WriteString("- 属性值 (stats: hp, mp, str, agi, int, vit)\n")
			sb.WriteString("- 技能列表 (skills)\n")
			sb.WriteString("- 外貌描述 (appearance)\n")
			sb.WriteString("- 性格特点 (personality)\n\n")

		case "location":
			sb.WriteString("请提供以下信息：\n")
			sb.WriteString("- 详细描述 (description)\n")
			sb.WriteString("- 氛围特点 (atmosphere)\n")
			sb.WriteString("- 历史背景 (history)\n")
			sb.WriteString("- 感官细节 (sensory: visual, audio, smell)\n")
			sb.WriteString("- 地图类型 (type: city, dungeon, field, etc.)\n\n")
		}
	}

	sb.WriteString("请使用以下 DSL 格式输出：\n\n")
	sb.WriteString("```dsl\n")
	sb.WriteString("# 添加到 02_craft.rpg\n")
	sb.WriteString("characters {\n")
	sb.WriteString("  player \"角色名\" {\n")
	sb.WriteString("    id = \"...\"\n")
	sb.WriteString("    // ... 详细信息\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	return sb.String()
}
