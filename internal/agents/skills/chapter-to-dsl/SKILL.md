# Skill: Convert Chapters to RPG-DSL

Convert novelgen chapter recaps to RPG-DSL format for story simulation and validation.

## Description

This skill analyzes chapter recap data (plot beats, characters, locations) and generates a complete RPG-DSL file that can be used for:
- Story structure validation
- Plot logic checking
- Character consistency verification
- Event sequence simulation

The DSL captures the narrative structure in an executable format.

## Input Format

```json
{
  "book_name": "mine",
  "story_setup": { /* story setup object */ },
  "characters": { /* character definitions from craft */ },
  "locations": { /* location definitions from craft */ },
  "chapters": [
    {
      "chapter_id": "P1-V1-C1",
      "title": "Chapter Title",
      "location": "Primary location",
      "time": "Timeline info",
      "present": ["Character names"],
      "plot_beats": ["Beat 1", "Beat 2", ...]
    }
  ]
}
```

## Output Format

Generate a complete RPG-DSL file with this structure:

```dsl
metadata {
  title = "Book Title"
  genre = ["genre1", "genre2"]
  power_system = "system_name"
  dsl_version = "0.2.0"
  source = "novelgen_outline"
}

world {
  // Extract unique locations from chapters
  location "Location Name" {
    id = "location_id"
    name = "Location Name"
    type = "indoor|outdoor|dungeon|city"
    description = "Brief description"
    __placeholder__ = false
  }
}

characters {
  // Define player/protagonist
  player "Protagonist Name" {
    id = "char_protagonist"
    name = "Protagonist Name"
    description = "Brief description"
    __placeholder__ = false
    
    str = 10
    agi = 10
    int = 10
    vit = 10
    hp = 100
    mp = 50
    
    class = "class_name"
    skills = []
  }
  
  // Define NPCs from chapters
  npc "NPC Name" {
    id = "char_npc_id"
    name = "NPC Name"
    description = "Brief description"
    __placeholder__ = false
    
    role = "role_type"
    default_location = "location_id"
  }
}

storyline {
  // Convert each chapter to a DSL chapter
  chapter "Chapter Title" {
    id = "chapter_id"
    
    objective "Main Objective" {
      // Convert plot beats to steps
      step 1 {
        description = "Plot beat description"
        event {
          type = "status|combat|location|acquire|knowledge"
          // Event-specific fields
        }
      }
    }
  }
}
```

## Conversion Rules

### 1. Chapter Mapping

Each chapter JSON becomes one DSL `chapter` block:
- `chapter_id` -> `id`
- `title` -> chapter name
- Each `plot_beat` -> one `step`

### 2. Event Type Detection

Analyze plot beats to determine event type:

**combat**: Contains fighting, battle, attack, kill, defeat
```dsl
event {
  type = "combat"
  combat {
    setup {
      enemies = [
        { id = "enemy_id", count = 1, level = 1 }
      ]
    }
  }
}
```

**location**: Contains movement, arrival, enter, go to
```dsl
event {
  type = "location"
  move {
    to = "destination_location_id"
  }
}
```

**acquire**: Contains get, obtain, find, receive item
```dsl
event {
  type = "acquire"
}
```

**knowledge**: Contains learn, discover, reveal, know
```dsl
event {
  type = "knowledge"
}
```

**status**: Default for character development, dialogue, emotions
```dsl
event {
  type = "status"
}
```

### 3. Character Extraction

From `present` array and plot beats:
- First appearance -> define NPC in `characters` block
- Track which chapters each character appears in
- Infer role from context (ally, enemy, neutral)

### 4. Location Extraction

From `location` field and plot beats:
- Create unique location definitions
- Use `connected_locations` if movement between locations is described

### 5. ID Naming Conventions

Use descriptive IDs with prefixes:
- Characters: `char_` + lowercase_name (e.g., `char_lin_yan`)
- Locations: `loc_` + lowercase_name (e.g., `loc_heifeng_mine`)
- Enemies: `enemy_` + type (e.g., `enemy_guard`)
- Items: `item_` + name (e.g., `item_jade_pendant`)

## Example

### Input Chapter:
```json
{
  "chapter_id": "P1-V1-C1",
  "title": "寒矿醒转，首死触发复生",
  "location": "黑风灵石矿丙字矿区地下矿道",
  "time": "苏醒当日",
  "present": ["林砚", "周虎", "老陈"],
  "plot_beats": [
    "林砚穿越到修仙世界成为矿奴",
    "遭到矿监周虎鞭打",
    "老陈提醒近期矿内不太平",
    "林砚开始挖矿",
    "矿道坍塌，林砚被砸死",
    "林砚复活，发现死而复生能力"
  ]
}
```

### Output DSL:
```dsl
  chapter "寒矿醒转，首死触发复生" {
    id = "P1-V1-C1"
    
    objective "矿场生存" {
      step 1 {
        description = "林砚穿越到修仙世界成为黑风灵石矿的矿奴"
        event {
          type = "status"
        }
      }
      
      step 2 {
        description = "遭到矿监周虎鞭打，决定暂时隐忍"
        event {
          type = "combat"
          combat {
            setup {
              enemies = [
                { id = "enemy_zhou_hu", count = 1, level = 3 }
              ]
            }
          }
          on_complete {
            narration = "林砚决定隐忍，不与周虎正面冲突"
          }
        }
      }
      
      step 3 {
        description = "老陈提醒近期矿内不太平，有灰衣官爷抓人"
        event {
          type = "knowledge"
        }
      }
      
      step 4 {
        description = "矿道突发坍塌，林砚被落石砸中死亡"
        event {
          type = "combat"
          combat {
            setup {
              enemies = [
                { id = "environment_collapse", count = 1, level = 1 }
              ]
            }
          }
          on_complete {
            narration = "林砚在矿道内死亡"
          }
        }
      }
      
      step 5 {
        description = "林砚复活，发现拥有死而复生的特殊能力"
        event {
          type = "status"
          on_complete {
            narration = "确认死而复生能力，修为跌落"
            exp = 50
          }
        }
      }
    }
  }
```

## Validation Checklist

Before outputting, verify:
- [ ] All chapters are converted
- [ ] Each chapter has at least one objective
- [ ] Each objective has steps matching plot_beats
- [ ] Event types are appropriate for the content
- [ ] Character IDs match those defined in characters block
- [ ] Location IDs match those defined in world block
- [ ] DSL syntax is valid (proper braces, quotes, equals)

## Notes

- Use `__placeholder__ = false` for all elements with actual content
- Include `description` fields for all major elements
- Add `on_complete` with narration for significant events
- Set appropriate `exp` rewards for breakthrough/achievement moments
- Use Chinese for all text content (descriptions, names, narrations)
