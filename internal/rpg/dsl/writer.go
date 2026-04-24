package dsl

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// DSLWriter writes DSL structures to text format
type DSLWriter struct {
	writer io.Writer
	indent int
}

// NewDSLWriter creates a new DSL writer
func NewDSLWriter(writer io.Writer) *DSLWriter {
	return &DSLWriter{
		writer: writer,
		indent: 0,
	}
}

// WriteDSL writes a complete DSL
func (dw *DSLWriter) WriteDSL(dsl *DSL) error {
	if err := dw.writeMetadata(dsl.Metadata); err != nil {
		return err
	}

	if err := dw.writeCharacters(dsl.Characters); err != nil {
		return err
	}

	if err := dw.writeWorld(dsl.World); err != nil {
		return err
	}

	if err := dw.writeStoryline(dsl.Storyline); err != nil {
		return err
	}

	if err := dw.writeSystems(dsl.Systems); err != nil {
		return err
	}

	return nil
}

// writeMetadata writes the metadata block
func (dw *DSLWriter) writeMetadata(meta *Metadata) error {
	if meta == nil {
		return nil
	}

	dw.writeLine("")
	dw.writeBlock("metadata", "", func() error {
		if meta.Title != "" {
			dw.writeField("title", fmt.Sprintf("%q", meta.Title))
		}
		if meta.Subtitle != "" {
			dw.writeField("subtitle", fmt.Sprintf("%q", meta.Subtitle))
		}
		if len(meta.Genre) > 0 {
			dw.writeField("genre", dw.formatStringSlice(meta.Genre))
		}
		if meta.PowerSystem != "" {
			dw.writeField("power_system", fmt.Sprintf("%q", meta.PowerSystem))
		}
		if meta.Tone != "" {
			dw.writeField("tone", fmt.Sprintf("%q", meta.Tone))
		}
		if meta.DSLVersion != "" {
			dw.writeField("dsl_version", fmt.Sprintf("%q", meta.DSLVersion))
		}
		if meta.Phase != "" {
			dw.writeField("phase", fmt.Sprintf("%q", meta.Phase))
		}
		return nil
	})

	return nil
}

// writeCharacters writes the characters block
func (dw *DSLWriter) writeCharacters(chars *Characters) error {
	if chars == nil {
		return nil
	}

	dw.writeLine("")
	dw.writeBlock("characters", "", func() error {
		// Write player
		if chars.Player != nil {
			if err := dw.writePlayer(chars.Player); err != nil {
				return err
			}
		}

		// Write enemies
		for _, enemy := range chars.Enemies {
			if err := dw.writeEnemy(enemy); err != nil {
				return err
			}
		}

		// Write NPCs
		for _, npc := range chars.NPCs {
			if err := dw.writeNPC(npc); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

// writePlayer writes a player character
func (dw *DSLWriter) writePlayer(player *Player) error {
	dw.writeBlock("player", fmt.Sprintf("%q", player.Name), func() error {
		dw.writeField("id", fmt.Sprintf("%q", player.ID))

		if player.IsPlaceholder {
			dw.writeField("__placeholder__", "true")
			dw.writeField("__source_phase__", fmt.Sprintf("%q", player.PlaceholderSource))
			return nil
		}

		if player.Class != "" {
			dw.writeField("class", fmt.Sprintf("%q", player.Class))
		}
		if player.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", player.Description))
		}
		if player.Age > 0 {
			dw.writeField("age", fmt.Sprintf("%d", player.Age))
		}
		if player.Gender != "" {
			dw.writeField("gender", fmt.Sprintf("%q", player.Gender))
		}
		if player.Race != "" {
			dw.writeField("race", fmt.Sprintf("%q", player.Race))
		}
		if player.Background != "" {
			dw.writeField("background", fmt.Sprintf("%q", player.Background))
		}
		if len(player.Personality) > 0 {
			dw.writeField("personality", dw.formatStringSlice(player.Personality))
		}
		if player.Motivation != "" {
			dw.writeField("motivation", fmt.Sprintf("%q", player.Motivation))
		}
		if len(player.Abilities) > 0 {
			dw.writeField("abilities", dw.formatStringSlice(player.Abilities))
		}
		if len(player.Affiliations) > 0 {
			dw.writeField("affiliations", dw.formatStringSlice(player.Affiliations))
		}
		if player.RoleInStory != "" {
			dw.writeField("role_in_story", fmt.Sprintf("%q", player.RoleInStory))
		}
		if player.Voice != "" {
			dw.writeField("voice", fmt.Sprintf("%q", player.Voice))
		}

		// Stats
		if err := dw.writeStats(&player.Stats); err != nil {
			return err
		}

		// Skills
		if len(player.Skills) > 0 {
			dw.writeField("skills", dw.formatStringSlice(player.Skills))
		}

		// Traits
		if len(player.Traits) > 0 {
			dw.writeBlock("traits", "", func() error {
				for name, trait := range player.Traits {
					dw.writeBlock("trait", fmt.Sprintf("%q", name), func() error {
						if trait.Unlocked {
							dw.writeField("unlocked", "true")
						}
						if trait.Trigger != "" {
							dw.writeField("trigger", fmt.Sprintf("%q", trait.Trigger))
						}
						return nil
					})
				}
				return nil
			})
		}

		return nil
	})

	return nil
}

// writeEnemy writes an enemy
func (dw *DSLWriter) writeEnemy(enemy Enemy) error {
	dw.writeBlock("enemy", fmt.Sprintf("%q", enemy.Name), func() error {
		dw.writeField("id", fmt.Sprintf("%q", enemy.ID))

		if enemy.IsPlaceholder {
			dw.writeField("__placeholder__", "true")
			dw.writeField("__source_phase__", fmt.Sprintf("%q", enemy.PlaceholderSource))
			return nil
		}

		if enemy.Type != "" {
			dw.writeField("type", fmt.Sprintf("%q", enemy.Type))
		}
		if enemy.Level > 0 {
			dw.writeField("level", fmt.Sprintf("%d", enemy.Level))
		}
		if enemy.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", enemy.Description))
		}
		if enemy.Appearance != "" {
			dw.writeField("appearance", fmt.Sprintf("%q", enemy.Appearance))
		}
		if len(enemy.Abilities) > 0 {
			dw.writeField("abilities", dw.formatStringSlice(enemy.Abilities))
		}

		return nil
	})

	return nil
}

// writeNPC writes an NPC
func (dw *DSLWriter) writeNPC(npc NPC) error {
	dw.writeBlock("npc", fmt.Sprintf("%q", npc.Name), func() error {
		dw.writeField("id", fmt.Sprintf("%q", npc.ID))

		if npc.IsPlaceholder {
			dw.writeField("__placeholder__", "true")
			dw.writeField("__source_phase__", fmt.Sprintf("%q", npc.PlaceholderSource))
			return nil
		}

		if npc.Role != "" {
			dw.writeField("role", fmt.Sprintf("%q", npc.Role))
		}
		if npc.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", npc.Description))
		}
		if npc.Age > 0 {
			dw.writeField("age", fmt.Sprintf("%d", npc.Age))
		}
		if npc.Gender != "" {
			dw.writeField("gender", fmt.Sprintf("%q", npc.Gender))
		}
		if npc.Appearance != "" {
			dw.writeField("appearance", fmt.Sprintf("%q", npc.Appearance))
		}
		if npc.Background != "" {
			dw.writeField("background", fmt.Sprintf("%q", npc.Background))
		}
		if len(npc.Personality) > 0 {
			dw.writeField("personality", dw.formatStringSlice(npc.Personality))
		}
		if npc.DefaultLocation != "" {
			dw.writeField("default_location", fmt.Sprintf("%q", npc.DefaultLocation))
		}

		return nil
	})

	return nil
}

// writeStats writes stats as flat fields (not a block)
func (dw *DSLWriter) writeStats(stats *Stats) error {
	if stats == nil {
		return nil
	}

	// Write stats as flat fields, not as a nested block
	if stats.STR > 0 {
		dw.writeField("str", fmt.Sprintf("%d", stats.STR))
	}
	if stats.AGI > 0 {
		dw.writeField("agi", fmt.Sprintf("%d", stats.AGI))
	}
	if stats.INT > 0 {
		dw.writeField("int", fmt.Sprintf("%d", stats.INT))
	}
	if stats.VIT > 0 {
		dw.writeField("vit", fmt.Sprintf("%d", stats.VIT))
	}
	if stats.HP > 0 {
		dw.writeField("hp", fmt.Sprintf("%d", stats.HP))
	}
	if stats.MP > 0 {
		dw.writeField("mp", fmt.Sprintf("%d", stats.MP))
	}

	return nil
}

// writeWorld writes the world block
func (dw *DSLWriter) writeWorld(world *World) error {
	if world == nil {
		return nil
	}

	dw.writeLine("")
	dw.writeBlock("world", "", func() error {
		// Write locations
		for _, loc := range world.Locations {
			if err := dw.writeLocation(loc); err != nil {
				return err
			}
		}

		// Write items
		for _, item := range world.Items {
			if err := dw.writeItem(item); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

// writeLocation writes a location
func (dw *DSLWriter) writeLocation(loc Location) error {
	dw.writeBlock("location", fmt.Sprintf("%q", loc.Name), func() error {
		dw.writeField("id", fmt.Sprintf("%q", loc.ID))

		if loc.IsPlaceholder {
			dw.writeField("__placeholder__", "true")
			dw.writeField("__source_phase__", fmt.Sprintf("%q", loc.PlaceholderSource))
			return nil
		}

		if loc.Type != "" {
			dw.writeField("type", fmt.Sprintf("%q", loc.Type))
		}
		if loc.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", loc.Description))
		}
		if loc.Appearance != "" {
			dw.writeField("appearance", fmt.Sprintf("%q", loc.Appearance))
		}
		if loc.Atmosphere != "" {
			dw.writeField("atmosphere", fmt.Sprintf("%q", loc.Atmosphere))
		}
		if loc.History != "" {
			dw.writeField("history", fmt.Sprintf("%q", loc.History))
		}
		if loc.Secrets != "" {
			dw.writeField("secrets", fmt.Sprintf("%q", loc.Secrets))
		}

		// Connections
		for _, conn := range loc.Connections {
			dw.writeBlock("connection", fmt.Sprintf("%q", conn.To), func() error {
				if conn.Direction != "" {
					dw.writeField("direction", fmt.Sprintf("%q", conn.Direction))
				}
				return nil
			})
		}

		return nil
	})

	return nil
}

// writeItem writes an item
func (dw *DSLWriter) writeItem(item Item) error {
	dw.writeBlock("item", fmt.Sprintf("%q", item.Name), func() error {
		dw.writeField("id", fmt.Sprintf("%q", item.ID))

		if item.Type != "" {
			dw.writeField("type", fmt.Sprintf("%q", item.Type))
		}
		if item.Rarity != "" {
			dw.writeField("rarity", fmt.Sprintf("%q", item.Rarity))
		}
		if item.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", item.Description))
		}

		return nil
	})

	return nil
}

// writeStoryline writes the storyline block
func (dw *DSLWriter) writeStoryline(story *Storyline) error {
	if story == nil {
		return nil
	}

	dw.writeLine("")
	dw.writeBlock("storyline", "", func() error {
		for _, chapter := range story.Chapters {
			if err := dw.writeChapter(chapter); err != nil {
				return err
			}
		}
		return nil
	})

	return nil
}

// writeChapter writes a chapter
func (dw *DSLWriter) writeChapter(chapter Chapter) error {
	dw.writeBlock("chapter", fmt.Sprintf("%q", chapter.Title), func() error {
		dw.writeField("id", fmt.Sprintf("%q", chapter.ID))
		if chapter.Arc != "" {
			dw.writeField("arc", fmt.Sprintf("%q", chapter.Arc))
		}

		for _, objective := range chapter.Objectives {
			if err := dw.writeObjective(objective); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

// writeObjective writes an objective
func (dw *DSLWriter) writeObjective(obj Objective) error {
	dw.writeBlock("objective", fmt.Sprintf("%q", obj.Name), func() error {
		if obj.ID != "" {
			dw.writeField("id", fmt.Sprintf("%q", obj.ID))
		}
		if obj.Type != "" {
			dw.writeField("type", fmt.Sprintf("%q", obj.Type))
		}

		for _, step := range obj.Steps {
			if err := dw.writeStep(step); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

// writeStep writes a step
func (dw *DSLWriter) writeStep(step Step) error {
	dw.writeBlock("step", fmt.Sprintf("%d", step.Order), func() error {
		if step.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", step.Description))
		}

		// Write event
		if step.Event.Type != "" {
			dw.writeBlock("event", "", func() error {
				dw.writeField("type", fmt.Sprintf("%q", step.Event.Type))

				if step.Event.Combat != nil {
					enemyIDs := make([]string, 0, len(step.Event.Combat.Setup.Enemies))
					for _, spawn := range step.Event.Combat.Setup.Enemies {
						enemyIDs = append(enemyIDs, spawn.ID)
					}
					dw.writeField("enemies", dw.formatStringSlice(enemyIDs))
				}

				return nil
			})
		}

		return nil
	})

	return nil
}

// writeSystems writes the systems block
func (dw *DSLWriter) writeSystems(systems *Systems) error {
	if systems == nil {
		return nil
	}

	dw.writeLine("")
	dw.writeBlock("systems", "", func() error {
		// Write custom attribute system
		if systems.AttributeSystem != nil {
			if err := dw.writeAttributeSystem(systems.AttributeSystem); err != nil {
				return err
			}
		}

		// Write power formula
		if systems.PowerFormula != nil {
			if err := dw.writePowerFormula(systems.PowerFormula); err != nil {
				return err
			}
		}

		// Write counters
		for _, counter := range systems.Counters {
			if err := dw.writeCounter(counter); err != nil {
				return err
			}
		}

		// Write progression systems
		for _, prog := range systems.ProgressionSystems {
			if err := dw.writeProgressionSystem(prog); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

func (dw *DSLWriter) writeAttributeSystem(sys *AttributeSystem) error {
	if sys == nil {
		return nil
	}

	return dw.writeBlock("attribute_system", fmt.Sprintf("%q", sys.Name), func() error {
		if sys.ID != "" {
			dw.writeField("id", fmt.Sprintf("%q", sys.ID))
		}
		if sys.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", sys.Description))
		}

		for _, attr := range sys.Attributes {
			a := attr
			if err := dw.writeBlock("attribute", fmt.Sprintf("%q", a.Name), func() error {
				if a.ID != "" {
					dw.writeField("id", fmt.Sprintf("%q", a.ID))
				}
				if a.Description != "" {
					dw.writeField("description", fmt.Sprintf("%q", a.Description))
				}
				if a.Type != "" {
					dw.writeField("type", fmt.Sprintf("%q", a.Type))
				}
				dw.writeField("base_value", fmt.Sprintf("%d", a.BaseValue))
				if a.MinValue != 0 {
					dw.writeField("min_value", fmt.Sprintf("%d", a.MinValue))
				}
				if a.MaxValue != 0 {
					dw.writeField("max_value", fmt.Sprintf("%d", a.MaxValue))
				}
				if a.IsResource {
					dw.writeField("is_resource", "true")
				} else {
					dw.writeField("is_resource", "false")
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (dw *DSLWriter) writePowerFormula(formula *PowerFormula) error {
	if formula == nil {
		return nil
	}

	return dw.writeBlock("power_formula", fmt.Sprintf("%q", formula.Name), func() error {
		if formula.ID != "" {
			dw.writeField("id", fmt.Sprintf("%q", formula.ID))
		}
		if formula.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", formula.Description))
		}
		dw.writeField("base_power", fmt.Sprintf("%d", formula.BasePower))
		if formula.Formula != "" {
			dw.writeField("formula", fmt.Sprintf("%q", formula.Formula))
		}

		for _, f := range formula.Factors {
			factor := f
			if err := dw.writeBlock("factor", fmt.Sprintf("%q", factor.Name), func() error {
				if factor.Attribute != "" {
					dw.writeField("attribute", fmt.Sprintf("%q", factor.Attribute))
				}
				dw.writeField("weight", fmt.Sprintf("%g", factor.Weight))
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeCounter writes a counter
func (dw *DSLWriter) writeCounter(counter CounterSystem) error {
	dw.writeBlock("counter", fmt.Sprintf("%q", counter.Name), func() error {
		if counter.Track != "" {
			dw.writeField("track", fmt.Sprintf("%q", counter.Track))
		}
		if counter.Description != "" {
			dw.writeField("description", fmt.Sprintf("%q", counter.Description))
		}

		for _, milestone := range counter.Milestones {
			dw.writeBlock("milestone", fmt.Sprintf("%d", milestone.Value), func() error {
				if milestone.Reward.Title != "" {
					dw.writeField("title", fmt.Sprintf("%q", milestone.Reward.Title))
				}
				if milestone.Reward.Exp > 0 {
					dw.writeField("exp", fmt.Sprintf("%d", milestone.Reward.Exp))
				}
				return nil
			})
		}

		return nil
	})

	return nil
}

// writeProgressionSystem writes a progression system
func (dw *DSLWriter) writeProgressionSystem(prog ProgressionSystem) error {
	dw.writeBlock("progression", fmt.Sprintf("%q", prog.Name), func() error {
		dw.writeField("id", fmt.Sprintf("%q", prog.ID))

		for _, level := range prog.Levels {
			dw.writeBlock("level", fmt.Sprintf("%d", level.Level), func() error {
				if level.Name != "" {
					dw.writeField("name", fmt.Sprintf("%q", level.Name))
				}
				if level.Requirements != "" {
					dw.writeField("requirements", fmt.Sprintf("%q", level.Requirements))
				}
				if len(level.Bonuses) > 0 {
					dw.writeField("bonuses", dw.formatStringSlice(level.Bonuses))
				}
				return nil
			})
		}

		return nil
	})

	return nil
}

// Helper methods

func (dw *DSLWriter) writeLine(line string) {
	indent := strings.Repeat("  ", dw.indent)
	fmt.Fprintln(dw.writer, indent+line)
}

func (dw *DSLWriter) writeField(name, value string) {
	dw.writeLine(fmt.Sprintf("%s = %s", name, value))
}

func (dw *DSLWriter) writeBlock(blockType, identifier string, content func() error) error {
	if identifier != "" {
		dw.writeLine(fmt.Sprintf("%s %s {", blockType, identifier))
	} else {
		dw.writeLine(fmt.Sprintf("%s {", blockType))
	}

	dw.indent++
	if err := content(); err != nil {
		return err
	}
	dw.indent--

	dw.writeLine("}")
	return nil
}

func (dw *DSLWriter) formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return "[]"
	}

	var parts []string
	for _, s := range slice {
		parts = append(parts, fmt.Sprintf("%q", s))
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
}

// WriteToFile writes DSL to a file
func (dsl *DSL) WriteToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := NewDSLWriter(file)
	return writer.WriteDSL(dsl)
}

// String returns DSL as string
func (dsl *DSL) String() string {
	var sb strings.Builder
	writer := NewDSLWriter(&sb)
	writer.WriteDSL(dsl)
	return sb.String()
}
