# Skill: Compose RPG-DSL

Generate RPG-DSL (Domain-Specific Language) for story chapters that can be directly converted to RPG game world.

## Description

This skill generates RPG-DSL code for story chapters. The DSL is a structured, executable format that describes:
- Story metadata (title, genre, power system)
- Game world (locations, items)
- Characters (player, enemies, NPCs)
- Storyline (chapters, objectives, events)

The output can be directly parsed and converted to RPG game data for simulation.

## Usage

```
Input: Story outline and requirements
Output: RPG-DSL code
```

## Output Format

The output MUST be valid RPG-DSL code following this structure:

```dsl
metadata {
  title = "Story Title"
  genre = ["genre1", "genre2"]
  power_system = "power_system_name"
  dsl_version = "0.1.0"
}

world {
  location "Location Name" {
    id = "location_id"
    type = "indoor|outdoor|dungeon"
  }
}

characters {
  player "Player Name" {
    id = "player_id"
    class = "class_name"
    str = 10
    agi = 12
    hp = 100
  }
  
  enemy "Enemy Name" {
    id = "enemy_id"
    str = 8
    hp = 50
  }
}

storyline {
  chapter "Chapter Title" {
    id = "chapter_id"
    
    objective "Objective Name" {
      step 1 {
        description = "Step description"
        event {
          type = "spawn|move|combat|dialogue|acquire"
          # event-specific fields
        }
      }
    }
  }
}
```

## Event Types

### spawn
```dsl
event {
  type = "spawn"
  actor = "character_id"
  location = "location_id"
}
```

### move
```dsl
event {
  type = "move"
  actor = "character_id"
  to = "location_id"
}
```

### combat
```dsl
event {
  type = "combat"
  enemies = [
    { id = "enemy_id", count = 1, level = 1 }
  ]
}
```

### dialogue
```dsl
event {
  type = "dialogue"
  speaker = "character_name"
  text = "Dialogue text"
}
```

### acquire
```dsl
event {
  type = "acquire"
  actor = "character_id"
  item = "item_id"
  quantity = 1
}
```

## Rules

1. **IDs must be unique** - Use descriptive IDs like `char_player`, `enemy_wasp`, `loc_base`
2. **Every chapter needs at least one objective** - Objectives contain executable steps
3. **Every step needs an event** - Events are the atomic actions in the game
4. **References must be valid** - All location_ids, character_ids must be defined
5. **Player character is required** - Must define a player in characters block

## Examples

### Example 1: Simple Chapter

Input: "A chapter where the player wakes up in a cryo facility and fights a wasp"

Output:
```dsl
metadata {
  title = "冷舱梦醒"
  genre = ["科幻", "废土"]
  power_system = "基因进化"
  dsl_version = "0.1.0"
}

world {
  location "休眠舱" {
    id = "loc_cryo"
    type = "indoor"
  }
}

characters {
  player "陆沉" {
    id = "char_player"
    class = "engineer"
    str = 10
    agi = 12
    hp = 100
  }
  
  enemy "虫族工蜂" {
    id = "enemy_wasp"
    str = 8
    hp = 50
  }
}

storyline {
  chapter "冷舱梦醒" {
    id = "chap_001"
    
    objective "逃离休眠舱" {
      step 1 {
        description = "从休眠仓中醒来"
        event {
          type = "spawn"
          actor = "char_player"
          location = "loc_cryo"
        }
      }
      
      step 2 {
        description = "击败入侵的工蜂"
        event {
          type = "combat"
          enemies = [
            { id = "enemy_wasp", count = 1, level = 1 }
          ]
        }
      }
    }
  }
}
```

### Example 2: Multi-Step Quest

Input: "Player needs to find a key, unlock a door, and defeat a boss"

Output:
```dsl
metadata {
  title = "地下城探险"
  genre = ["奇幻", "冒险"]
  power_system = "魔法"
  dsl_version = "0.1.0"
}

world {
  location "入口大厅" {
    id = "loc_entrance"
    type = "indoor"
  }
  
  location "宝藏室" {
    id = "loc_treasure"
    type = "indoor"
  }
}

characters {
  player "勇者" {
    id = "char_hero"
    class = "warrior"
    str = 15
    agi = 10
    hp = 120
  }
  
  enemy "地牢守卫" {
    id = "enemy_guard"
    str = 10
    hp = 80
  }
  
  enemy "地牢领主" {
    id = "enemy_boss"
    str = 20
    hp = 200
  }
}

storyline {
  chapter "地牢深处" {
    id = "chap_dungeon"
    
    objective "探索地牢" {
      step 1 {
        description = "进入地牢"
        event {
          type = "spawn"
          actor = "char_hero"
          location = "loc_entrance"
        }
      }
      
      step 2 {
        description = "击败守卫"
        event {
          type = "combat"
          enemies = [
            { id = "enemy_guard", count = 2, level = 1 }
          ]
        }
      }
      
      step 3 {
        description = "前往宝藏室"
        event {
          type = "move"
          actor = "char_hero"
          to = "loc_treasure"
        }
      }
      
      step 4 {
        description = "击败地牢领主"
        event {
          type = "combat"
          enemies = [
            { id = "enemy_boss", count = 1, level = 5 }
          ]
        }
      }
    }
  }
}
```

## Common Patterns

### Pattern 1: Combat with Victory Reward
```dsl
event {
  type = "combat"
  enemies = [{ id = "enemy_id", count = 1 }]
  on_complete {
    narration = "敌人倒下了！"
    exp = 100
  }
}
```

### Pattern 2: Dialogue Choice
```dsl
event {
  type = "dialogue"
  speaker = "NPC"
  text = "你要去哪里？"
}

event {
  type = "move"
  actor = "player"
  to = "destination"
}
```

### Pattern 3: Item Acquisition
```dsl
event {
  type = "acquire"
  actor = "player"
  item = "item_key"
  quantity = 1
}
```

## Validation Checklist

Before outputting, verify:
- [ ] metadata block exists with title
- [ ] characters block exists with player
- [ ] storyline block exists with at least one chapter
- [ ] Each chapter has at least one objective
- [ ] Each objective has at least one step
- [ ] Each step has an event with valid type
- [ ] All IDs are unique
- [ ] All references (location_id, character_id) are defined

## Advanced Features (Phase 2)

### Hook System

Hooks allow tracking game events and awarding achievements:

```dsl
systems {
  hooks {
    hook "on_kill" {
      id = "hook_kill_tracker"
      
      counter "goblins_killed" {
        filter = "enemy_id == 'enemy_goblin'"
        max = 100
        
        on_milestone 10 {
          reward { title = "哥布林杀手" }
        }
        on_milestone 50 {
          reward { 
            title = "哥布林克星"
            exp = 500
          }
        }
      }
    }
    
    hook "on_damage_taken" {
      id = "hook_near_death"
      condition = "damage >= max_hp * 0.5"
      
      counter "near_death_experiences" {
        max = 10
        
        on_milestone 3 {
          reward { 
            title = "幸存者"
            unlock_trait = "regeneration"
          }
        }
      }
    }
  }
}
```

**Supported Hooks:**
- `on_kill` - When player kills an enemy
- `on_damage_taken` - When player takes damage
- `on_skill_use` - When player uses a skill
- `on_level_up` - When player levels up

### Trigger System

Triggers activate when conditions are met:

```dsl
systems {
  triggers {
    trigger "觉醒契机" {
      id = "trigger_awakening"
      
      condition {
        and = [
          { stat = "hp", op = "<", value = 20 },
          { counter = "near_death_experiences", op = ">=", value = 3 }
        ]
      }
      
      on_trigger {
        narration = "生死之间，你感受到体内某种力量在苏醒..."
        unlock_trait = "regeneration"
        exp = 200
      }
      
      once = true  # Only trigger once
    }
  }
}
```

### Built-in Functions

**Math Functions:**
- `random(min, max)` - Random integer
- `random_float(min, max)` - Random float
- `min(a, b, ...)` - Minimum value
- `max(a, b, ...)` - Maximum value
- `clamp(val, min, max)` - Clamp value to range
- `round(val)` - Round to nearest integer

**Logic Functions:**
- `and(a, b, ...)` - Logical AND
- `or(a, b, ...)` - Logical OR
- `not(x)` - Logical NOT

**Query Functions:**
- `has_flag(name)` - Check if flag is set
- `has_item(id)` - Check if character has item
- `kill_count(enemy_id)` - Get kill count for enemy type
- `skill_use_count(skill_id)` - Get skill use count
- `get_stat(character_id, stat)` - Get character stat
- `player_level()` - Get player level

**Example Usage:**
```dsl
enemy "精英怪" {
  id = "enemy_elite"
  # Dynamic difficulty based on player level
  level = "player_level() + random(0, 2)"
}

trigger "隐藏任务" {
  condition = "and(kill_count('enemy_goblin') >= 10, has_flag('met_npc'))"
  on_trigger {
    narration = "你已经证明了自己的实力..."
  }
}
```

## Notes

- Keep the DSL simple and clear
- Use descriptive IDs (e.g., `enemy_wasp` not `e1`)
- Balance enemy stats with player stats
- Each step should represent a meaningful game action
- Comments can be added with # symbol
- Use hooks to track player progress and award achievements
- Use triggers for conditional story events