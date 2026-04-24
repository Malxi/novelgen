# Novel Generation Tool: CLI System Documentation

# 1. Overview

The Novel Generation Tool is a command-line interface (CLI) system designed to facilitate step-by-step novel creation through a structured "decompress" workflow. It guides users from initial idea to complete chapter drafts by progressively expanding and injecting context at each stage, while ensuring consistency across all story elements.

## Core Workflow Phases

1. **Init**: Use AI to Convert a high-level idea into structured story setup (genre, premise, rules, theme, tone, POV style, etc.)

2. **Compose**: Use AI to Generate a story outline (parts → volumes → chapters) with detailed plot beats, conflict, and pacing

3. **Worldbuilding**: Use AI to Create detailed world elements (characters, weapons, items, map, factions, etc.) based on story outline

4. **Storyline**: Use AI to Inject worldbuilding elements into the outline to form concrete, playable chapter storylines

5. **Write**: Use AI to draft full chapters based on finalized storylines, with strict style and consistency constraints

6. **Check & Sync**: Use AI to Validate story consistency and auto-update downstream content when edits are made

# 2. Project Structure

The tool organizes each novel project into a directory with structured data files, ensuring context is preserved and passed between phases. Each project is identified by a `novel.json` file in the root directory.

## 2.1 code strcuture
```bash
novels/
├── core/               # Configuration files (story setup, outline, worldbuilding)
│   ├── skills/          # Skill definitions for AI agents
│   │   └── build-story/
│   │   │   └── SKILL.md # build story based on (ideas)
│   │   └── build-outline/
│   │   │   └── SKILL.md # build outline based on (story setup)
│   │   └── build-world/
│   │   │   └── SKILL.md # build world elements based on (outline)
│   │   └── write-storyline/
│   │   │   └── SKILL.md # write storyline draft based on (outline, world elements)
│   │   └── write-chapter/
│   │   │   └── SKILL.md # write chapter draft based on (storyline, status)
│   │   └── sync-story/
│   │   │   └── SKILL.md # sync chapter status to story status
│   ├── llm/          # AI connector (OpenAI, etc.)
│   ├── injector/     # context injector (outline, world elements, etc.)
│── cli/          # Command-line interface for user interaction

```


## 2.2 artifact strcuture
```bash
novels/
├── novel.json            # Project configuration (name, version, created_at)
├── config/               # Configuration files (story setup, outline, worldbuilding)
│   ├── init/
│   │   └── story_setup.md  # Story setup (genre, premise, rules, theme, POV)
│   ├── compose/
│   │   └── outline.md      # Story outline (parts → volumes → chapters, with beats)
│   ├── worldbuilding/
│   │   ├── characters/   # Character details
│   │   │   └── joker.json   # Character 0001 details
│   │   ├── weapons/      # Weapon details
│   │   │   └── dagger.json   # Weapon 0001 details
│   │   ├── items/        # Item details
│   │   │   └── key.json     # Item 0001 details
│   │   ├── locations/    # World map/locations
│   │   │   └── villeage.json  
│   │   ├── factions/     # Faction details
│   │   │   └── mafia.json
│   ├── storyline/
│   │   └── storylines.md   # Injected storylines (outline + world elements)
├── data/                 # Generated content
│   └── chapters/         # AI-generated chapter drafts (with versioned subdirectories)
│   │   └── 0001.md       # Injected storylines (outline + world elements)
│   │   └── 0002.md       # Injected storylines (outline + world elements)
│   └── snapshots/         # AI-generated chapter drafts (with versioned subdirectories)
│   │   ├── 0001/
│   │   │   └── status.json   # status of chapter 0001
├── logs/                 # Command execution logs
└── version/              # Auto-saved versions of key files for rollback
```

## Multiple Novel Projects

The tool supports creating multiple novel projects. Each project is self-contained in its own directory:

- Run `novelgen init` in any directory to create a new novel project
- If `novel.json` already exists in the current directory, an error will be raised
- All other commands (`compose`, `worldbuild`, `storyline`, `write`, `check`, `sync`) automatically detect the project by searching for `novel.json` in the current directory and parent directories

# 3. CLI Commands

## 3.1 `novelgen init`

**Purpose**: Initialize a new novel project and define core story setup, with added fields to ensure consistent tone and perspective.

### Inputs

- Interactive prompts or JSON file for:
        
    - Project name

    - Genre(s)

    - Core premise

    - Story rules/constraints

    - Target audience

    - Tone/style

    - Core theme (e.g., courage vs. power, redemption)

    - Narrative tense (past/present)

    - POV style (first-person, third-person limited/omniscient)

### Outputs

- `config/init/story_setup.md` with structured story setup data (including new fields)

### Options

- `--gen "<promopt>"` : Ue AI to genrate the story setup based on the promot


### Example

```bash
novelgen init --template fantasy
# Interactive prompts:
# Project name: The Dragon's Covenant
# Genre: Fantasy, Adventure
# Premise: A young blacksmith discovers a dragon egg and must protect it from an evil king
# Rules: Magic is rare, dragons are thought extinct, kingdoms wage constant war
# Theme: Courage vs power
# Tense: Past
# POV Style: Third-person limited
# Tone: Epic, Hopeful
```

## 3.2 `novelgen compose`

**Purpose**: Generate a story outline with a rigid 3-level structure (parts → volumes → chapters), including plot beats, conflict, and pacing to guide AI writing.

### Inputs

- Reads `data/init/story_setup.json`

- Interactive prompts or JSON file for:
        

    - Number of parts

    - Volume structure per part

    - Chapter structure per volume

    - Key plot points (beats) for each chapter

    - Conflict type for each chapter/volume

    - Pacing (slow/normal/fast) for each chapter

### Outputs

- `config/compose/outline.json` with hierarchical 3-level outline structure (parts → volumes → chapters)

### Options

- `--gen`: Automatically generate whole outline based on setup

- `--regen <id>`: Regenerate a specific part, volume, or chapter (e.g., "1_1_1")

### Example

```bash
novelgen compose
# Reads story_setup.json
# Interactive prompts:
# Number of parts: 3
# Part 1 volumes: 2 (Introduction, Rising Action)
# Part 1 Volume 1 chapters: 3
# Chapter 1 (Introduction): Title, Summary, Beats, Conflict, Pacing
# Generates outline.json with 3-level structure and detailed chapter beats
```

## 3.3 `novelgen worldbuild`

**Purpose**: Create detailed worldbuilding elements for the novel, including relationship mapping to ensure consistent character/faction dynamics.

### Inputs

- Reads `config/init/story_setup.json` and `config/compose/outline.json`
- Subcommond
- - gen: generate by AI
- - regen: regenerate specific element with traget name (e.g., "characters", "locations")

### Outputs

- `config/worldbuilding/` directory with individual JSON files for each element type

### Options

- `--element <type>`: Focus on specific element (e.g., "characters", "locations")
- `--name <name>:`: Specify the name of the element to generate or regenrate

### Example

```bash
novelgen worldbuild gen --element characters
# Reads setup and outline
# Generates Kael.json
```

## 3.4 `novelgen storyline`

**Purpose**: Inject worldbuilding elements into the outline to create concrete, playable storylines for each chapter, with clear goals, conflict, and POV.

### Inputs

- Reads `config/compose/outline.json` and`config/worldbuilding/` files

- Interactive prompts or JSON file for:

    - Character placements in each chapter

    - Location assignments for key scenes

    - Items/weapons used in each chapter

    - Factions involved in each chapter

    - Chapter goal (what the POV character aims to achieve)

    - Conflict (obstacle to the chapter goal)

    - Twist/foreshadowing (for narrative continuity)

    - Hook injections for each chapter/volume

### Outputs

- `config/storyline/storylines.json` with detailed storylines (outline + world elements + chapter-specific context)

### Options

- `--file <path>`: Load storyline injections from JSON file

- `--auto`: Automatically inject elements based on outline and worldbuilding

- `--regenerate <id>`: Regenerate storylines for a specific part, volume, or chapter

### Example

```bash
novelgen storyline
# Reads outline and worldbuilding files
# Interactive prompts:
# Part 1 Volume 1 Chapter 1 (Introduction): POV Character: Kael, Location: Eldermore Forest, Items: Dragon Egg, Goal: Hide the egg, Conflict: Fear of being caught, Foreshadowing: Egg glows when Kael touches it
# Part 1 Volume 1 Chapter 2: POV Character: Kael, Location: Blacksmith's Forge, Characters: Gareth, Conflict: Gareth questions Kael's secrecy
# Generates storylines.json
```

## 3.5 `novelgen events`

**Purpose**: Manage story events with a standardized format that enables state reconstruction at any chapter without AI involvement.

### Inputs

- Reads `config/storyline/storylines.json` and `data/chapters/` files

- Interactive prompts or JSON file for:

    - Event types (item_acquisition, status_change, relationship_change, etc.)

    - Event subjects and objects (who/what is involved)

    - Event actions (what happens)

    - Event details and location

    - Explicit state changes for affected entities

### Outputs

- `config/events/events.json` with:

    - Chronological list of structured events

    - State snapshots for each chapter

    - Reconstructible story state at any point

### Options

- `--file <path>`: Load events from JSON file

- `--auto`: Automatically extract events from written chapters

- `--regenerate <chapter_id>`: Regenerate events and state for specific chapter

- `--reconstruct <chapter_id>`: Reconstruct story state at specific chapter without AI

### Example

```bash
novelgen events
# Reads storylines and chapter drafts
# Interactive prompts:
# Event 1: Type: item_acquisition, Subject: char_1, Object: item_1, Action: acquires, Location: loc_2, Chapter: chap_1_1_1
# State Changes: char_1.inventory: +item_1, char_1.status: +has_dragon_egg
# Event 2: Type: status_change, Subject: item_1, Action: activates, Location: loc_1, Chapter: chap_1_1_2
# State Changes: item_1.status: +active, char_1.knowledge: +egg_is_alive
# Generates events.json with structured events and state snapshots

# State reconstruction example
novelgen events --reconstruct chap_1_1_2
# Reconstructs and displays story state at Chapter 1.1.2
# Output: Character Kael has inventory [item_1], status [has_dragon_egg], knowledge [egg_is_alive, egg_contains_dragon]
# Output: Item egg has status [active, hatching_soon], location: char_1
# Output: Relationship Kael-Gareth: strained (trust: 0.7)
```

## 3.6 `novelgen write`

**Purpose**: Use AI to draft full chapters based on finalized storylines, with strict style and consistency constraints to avoid OOC or worldbreaking content.

### Inputs

- Reads `config/storyline/storylines.json`, `config/events/events.json`, and all previous context files
- Uses events from chapters 1 to x-1 when writing chapter x

- Interactive prompts or configuration for:
        

    - Chapter length (target word count)

    - Writing style (matches `story_setup.json` tone)

    - AI model selection

    - Output format

    - POV enforcement (matches chapter storyline)

    - Tense enforcement (matches `story_setup.json`)

    - Consistency lock (prevents OOC/rule violations)

### Outputs

- `data/chapters/` directory with AI-generated chapter drafts (each chapter has a versioned subdirectory for rollback)

### Options

- `--model <name>`: Specify AI model (e.g., "gpt-4", "claude-3")

- `--length <words>`: Set target chapter length

- `--output <format>`: Specify output format (e.g., "txt", "md")

- `--regenerate <chapter_id>`: Regenerate a specific chapter draft

- `--version`: Save a new version of the chapter without overwriting the original

### Example

```bash
novelgen write --model gpt-4 --length 3000 --version
# Reads storylines.json and all context
# AI generates Chapter 1 (version 1): Kael discovers the dragon egg in the forest
# AI generates Chapter 2 (version 1): Kael hides the egg and is questioned by Gareth
# Saves chapters to data/chapters/chap1/v1.txt and chap2/v1.txt
```

## 3.6 `novelgen check`

**Purpose**: Automatically validate the consistency of all story elements to prevent plot holes, OOC behavior, and world rule violations.

### Inputs

- Reads all data files (`story_setup.json`, `outline.json`, `worldbuilding/`, `storylines.json`, and chapter drafts)

### Checks Performed

- Character consistency (traits, motivations, relationships across chapters)

- Faction relationship consistency (alliances, rivalries)

- World rule compliance (no violations of magic, power system, or world logic)

- Timeline and location logic (characters/locations align with chapter context)

- Foreshadowing and hook resolution (unresolved伏笔 or unused hooks)

- POV and tense consistency (no unexpected POV/tense shifts)

### Outputs

- A detailed consistency report (text file) highlighting issues and suggested fixes

- Option to auto-fix minor consistency issues (e.g., tense errors)

### Options

- `--report <path>`: Save consistency report to a specific file

- `--auto-fix`: Auto-correct minor consistency issues (tense, POV, typos)

- `--focus <type>`: Focus on specific consistency checks (e.g., "characters", "rules")

### Example

```bash
novelgen check --auto-fix --report consistency_report.txt
# Reads all project data
# Identifies: Kael's trait "cowardly" in Chapter 3 conflicts with "brave" in characters.json
# Auto-fixes: Tense shift in Chapter 2 (present → past)
# Saves report to consistency_report.txt
```

## 3.7 `novelgen sync` 

**Purpose**: Automatically update downstream content when edits are made to upstream files (e.g., outline, worldbuilding), ensuring all elements stay in sync.

### Inputs

- Detects changes to `story_setup.json`, `outline.json`, `worldbuilding/` files, or `events.json`

### Sync Actions

- If outline is edited: Update `storylines.json` and `events.json` to reflect new chapter structure

- If worldbuilding is edited: Update `storylines.json` and `events.json` to include new characters/items/relationships

- If `story_setup.json` is edited: Update `outline.json`, `storylines.json`, `events.json`, and future chapter drafts to match new tone/POV/tense

- If `events.json` is edited: Update subsequent chapter drafts to reflect event changes

### Outputs

- Updated `storylines.json`, `events.json`, and other affected files

- Sync report highlighting changes made

### Options

- `--dry-run`: Show proposed sync changes without modifying files

- `--force`: Overwrite existing files with sync changes (bypasses confirmation)

### Example

```bash
novelgen sync
# Detects: New character (Lira) added to characters.json
# Updates: storylines.json to include Lira in Chapter 5 (per outline context)
# Generates: sync_report.txt with details of changes
```

# 4. Data Models

## 4.1 Story Setup (`story_setup.json`)

```json
{
  "project_name": "The Dragon's Covenant",
  "genres": ["Fantasy", "Adventure"],
  "premise": "A young blacksmith discovers a dragon egg and must protect it from an evil king",
  "theme": "Courage vs power",
  "rules": [
    "Magic is rare and only usable by select individuals",
    "Dragons are thought to be extinct",
    "Kingdoms wage constant war for resources"
  ],
  "target_audience": "Young Adult",
  "tone": "Epic, Hopeful",
  "tense": "past",
  "pov_style": "third-person limited"
}
```

## 4.2 Events (`events.json`)

```json
{
  "events": [
    {
      "id": "event_1",
      "chapter_id": "chap_1_1_1",
      "event_type": "item_acquisition",
      "subject_id": "char_1",
      "object_id": "item_1",
      "action": "acquires",
      "details": "Kael finds a mysterious glowing egg in the forest",
      "location_id": "loc_2",
      "timeline_order": 1,
      "state_changes": {
        "char_1": {
          "inventory": ["item_1"],
          "status": ["has_dragon_egg"]
        }
      }
    },
    {
      "id": "event_2",
      "chapter_id": "chap_1_1_2",
      "event_type": "status_change",
      "subject_id": "item_1",
      "action": "activates",
      "details": "The egg starts glowing brighter and making sounds",
      "location_id": "loc_1",
      "timeline_order": 2,
      "state_changes": {
        "item_1": {
          "status": ["active", "hatching_soon"]
        },
        "char_1": {
          "knowledge": ["egg_is_alive", "egg_contains_dragon"]
        }
      }
    },
    {
      "id": "event_3",
      "chapter_id": "chap_1_1_2",
      "event_type": "relationship_change",
      "subject_id": "char_1",
      "object_id": "char_2",
      "action": "strains",
      "details": "Gareth questions Kael about his secrecy",
      "location_id": "loc_1",
      "timeline_order": 3,
      "state_changes": {
        "char_1_char_2": {
          "relationship_status": "strained",
          "trust_level": 0.7
        }
      }
    }
  ],
  "state_snapshots": {
    "chap_1_1_1": {
      "characters": {
        "char_1": {
          "inventory": ["item_1"],
          "status": ["has_dragon_egg"],
          "knowledge": []
        },
        "char_2": {
          "inventory": [],
          "status": [],
          "knowledge": []
        }
      },
      "items": {
        "item_1": {
          "status": ["in_inventory"],
          "location": "char_1"
        }
      },
      "relationships": {
        "char_1_char_2": {
          "relationship_status": "neutral",
          "trust_level": 0.9
        }
      }
    },
    "chap_1_1_2": {
      "characters": {
        "char_1": {
          "inventory": ["item_1"],
          "status": ["has_dragon_egg"],
          "knowledge": ["egg_is_alive", "egg_contains_dragon"]
        },
        "char_2": {
          "inventory": [],
          "status": [],
          "knowledge": []
        }
      },
      "items": {
        "item_1": {
          "status": ["active", "hatching_soon"],
          "location": "char_1"
        }
      },
      "relationships": {
        "char_1_char_2": {
          "relationship_status": "strained",
          "trust_level": 0.7
        }
      }
    }
  }
}
```

## 4.3 Outline (`outline.json`)

```json
{
  "parts": [
    {
      "id": "part_1",
      "title": "Discovery",
      "summary": "Kael discovers a dragon egg and begins his journey to protect it",
      "volumes": [
        {
          "id": "vol_1_1",
          "title": "Introduction",
          "summary": "Introduce Kael's life as a blacksmith's apprentice and the world of Eldermore",
          "chapters": [
            {
              "id": "chap_1_1_1",
              "title": "The Glow in the Woods",
              "summary": "Kael finds a glowing egg while gathering firewood in the forest outside Eldermore",
              "beats": [
                "Kael leaves the village to gather firewood",
                "He stumbles upon a strange glowing object in the underbrush",
                "He realizes it's an egg and decides to hide it from others",
                "He returns to the village, keeping the egg a secret"
              ],
              "conflict": "Kael's fear of being caught with the egg and uncertainty about its origin",
              "pacing": "normal"
            },
            {
              "id": "chap_1_1_2",
              "title": "Secrets in the Forge",
              "summary": "Kael hides the egg in the forge and is questioned by his mentor, Gareth",
              "beats": [
                "Kael hides the egg in a hidden compartment of the forge",
                "Gareth notices Kael's unusual behavior",
                "Gareth questions Kael, who struggles to keep the secret",
                "Kael promises to tell Gareth the truth if things get dangerous"
              ],
              "conflict": "Kael's internal conflict between keeping the egg safe and being honest with his mentor",
              "pacing": "normal"
            }
          ]
        },
        {
          "id": "vol_1_2",
          "title": "Rising Action",
          "summary": "King Vorath's soldiers arrive in Eldermore, searching for the egg",
          "chapters": [
            {
              "id": "chap_1_2_1",
              "title": "Soldiers at the Gate",
              "summary": "King Vorath's soldiers enter Eldermore, demanding to search the village",
              "beats": [
                "Soldiers led by Captain Rook arrive at Eldermore's gate",
                "They announce they are searching for a 'mystical artifact'",
                "The villagers are fearful and comply with the search",
                "Kael hides the egg deeper in the forge, worried it will be found"
              ],
              "conflict": "External conflict between the villagers and the soldiers; Kael's fear of the egg being discovered",
              "pacing": "fast"
            }
          ]
        }
      ]
    }
  ]
}
```

## 4.3 Worldbuilding Elements

### Characters (`characters.json`)

```json
{
  "characters": [
    {
      "id": "char_1",
      "name": "Kael",
      "role": "Protagonist",
      "backstory": "Orphaned at a young age, raised by a blacksmith in the village of Eldermore. He has no memory of his parents and feels a strong connection to the village and his mentor, Gareth.",
      "motivation": "Protect the dragon egg and prevent it from falling into the king's hands; prove his worth beyond being an apprentice.",
      "traits": ["Brave", "Compassionate", "Skilled with tools", "Secretive when necessary", "Loyal to those he cares about"],
      "relationships": [
        {
          "target_id": "char_2",
          "type": "mentor",
          "dynamic": "Kael looks up to Gareth as a father figure; Gareth trusts Kael but worries about his impulsiveness."
        },
        {
          "target_id": "char_3",
          "type": "enemy",
          "dynamic": "Kael fears King Vorath and disagrees with his cruel methods; Vorath is unaware of Kael but would kill him to get the egg."
        }
      ]
    },
    {
      "id": "char_2",
      "name": "Gareth",
      "role": "Mentor",
      "backstory": "A retired soldier who became a blacksmith after the last war. He took Kael in when he was orphaned and taught him the trade.",
      "motivation": "Protect Eldermore and Kael; avoid conflict with the king's forces but stand up for what's right.",
      "traits": ["Wise", "Calm", "Protective", "Experienced in battle"],
      "relationships": [
        {
          "target_id": "char_1",
          "type": "mentor",
          "dynamic": "Gareth sees Kael as a son and wants him to be safe; he is patient but firm with Kael."
        }
      ]
    },
    {
      "id": "char_3",
      "name": "King Vorath",
      "role": "Antagonist",
      "backstory": "A power-hungry ruler who took the throne through force. He believes dragons hold the key to eternal power and has spent years searching for one.",
      "motivation": "Capture the dragon egg to hatch it and use the dragon's power to conquer all kingdoms.",
      "traits": ["Cruel", "Paranoid", "Greedy", "Ruthless"],
      "relationships": [
        {
          "target_id": "char_4",
          "type": "servant",
          "dynamic": "Vorath trusts Captain Rook to carry out his orders; Rook fears Vorath and will do anything to avoid punishment."
        }
      ]
    },
    {
      "id": "char_4",
      "name": "Captain Rook",
      "role": "Secondary Antagonist",
      "backstory": "A loyal soldier who rose through the ranks by being ruthless and obedient. He is tasked with finding the dragon egg for Vorath.",
      "motivation": "Impress King Vorath to gain more power and wealth.",
      "traits": ["Obedient", "Cruel", "Persistent"],
      "relationships": [
        {
          "target_id": "char_3",
          "type": "servant",
          "dynamic": "Rook is loyal to Vorath but secretly fears his wrath; he will stop at nothing to complete his mission."
        }
      ]
    }
  ]
}
```

### Locations (`locations.json`)

```json
{
  "locations": [
    {
      "id": "loc_1",
      "name": "Eldermore Village",
      "terrain": "Forest edge, rolling hills",
      "landmarks": ["Blacksmith's Forge", "Village Square", "Wooden Gate", "Well"],
      "culture": "Simple farming community, distrustful of outsiders, values hard work and loyalty. The village is small, with around 50 residents, and relies on farming and blacksmithing for survival.",
      "relationships": [
        {
          "target_id": "char_1",
          "type": "residence",
          "dynamic": "Kael has lived in Eldermore his entire life; he feels a strong connection to the village and its people."
        },
        {
          "target_id": "char_2",
          "type": "residence/place of work",
          "dynamic": "Gareth runs the blacksmith's forge in Eldermore; he is a respected member of the community."
        }
      ]
    },
    {
      "id": "loc_2",
      "name": "Eldermore Forest",
      "terrain": "Dense woods, tall trees, small streams",
      "landmarks": ["Glowing Egg Site", "Ancient Oak Tree", "Hidden Cave"],
      "culture": "No permanent residents; considered a place of mystery by the villagers. Some believe it is haunted by spirits, while others use it for gathering firewood and hunting.",
      "relationships": [
        {
          "target_id": "char_1",
          "type": "discovery site",
          "dynamic": "Kael found the dragon egg in this forest; it is the starting point of his journey."
        }
      ]
    },
    {
      "id": "loc_3",
      "name": "Vorath's Castle",
      "terrain": "Mountain top, rocky cliffs",
      "landmarks": ["Throne Room", "Dungeon", "Dragon Research Chamber"],
      "culture": "Cold and imposing; home to King Vorath and his soldiers. The castle is a symbol of fear and oppression for the surrounding kingdoms.",
      "relationships": [
        {
          "target_id": "char_3",
          "type": "residence/seat of power",
          "dynamic": "Vorath rules his kingdom from this castle; it is where he plans his conquests and searches for the dragon egg."
        },
        {
          "target_id": "char_4",
          "type": "place of work",
          "dynamic": "Rook is based at the castle when not on missions; he reports directly to Vorath."
        }
      ]
    }
  ]
}
```

## 4.4 Storylines (`storylines.json`)

```json
{
  "parts": [
    {
      "id": "part_1",
      "title": "Discovery",
      "volumes": [
        {
          "id": "vol_1_1",
          "title": "Introduction",
          "chapters": [
            {
              "id": "chap_1_1_1",
              "title": "The Glow in the Woods",
              "summary": "Kael finds a glowing egg while gathering firewood in the forest outside Eldermore",
              "beats": [
                "Kael leaves the village to gather firewood",
                "He stumbles upon a strange glowing object in the underbrush",
                "He realizes it's an egg and decides to hide it from others",
                "He returns to the village, keeping the egg a secret"
              ],
              "conflict": "Kael's fear of being caught with the egg and uncertainty about its origin",
              "pacing": "normal",
              "injections": {
                "pov_character": "char_1",
                "characters": ["char_1"],
                "locations": ["loc_2"],
                "items": ["item_1"],
                "factions": [],
                "goal": "Gather firewood and return to the village without being noticed with the egg",
                "conflict_detail": "Kael is unsure what the egg is, but its glow makes him nervous that others will want it. He worries about being accused of witchcraft or theft if he is caught with it.",
                "foreshadowing": "The egg glows brighter when Kael touches it, hinting at a connection between him and the egg.",
                "hook": "As Kael hides the egg in his bag, he hears a distant shout from the direction of the village – could someone already be looking for it?"
              }
            },
            {
              "id": "chap_1_1_2",
              "title": "Secrets in the Forge",
              "summary": "Kael hides the egg in the forge and is questioned by his mentor, Gareth",
              "beats": [
                "Kael hides the egg in a hidden compartment of the forge",
                "Gareth notices Kael's unusual behavior",
                "Gareth questions Kael, who struggles to keep the secret",
                "Kael promises to tell Gareth the truth if things get dangerous"
              ],
              "conflict": "Kael's internal conflict between keeping the egg safe and being honest with his mentor",
              "pacing": "normal",
              "injections": {
                "pov_character": "char_1",
                "characters": ["char_1", "char_2"],
                "locations": ["loc_1"],
                "items": ["item_1"],
                "factions": [],
                "goal": "Hide the egg safely and avoid arousing Gareth's suspicion",
                "conflict_detail": "Kael wants to trust Gareth, but he fears that Gareth will make him give up the egg or report it to the village elders. He also worries that Gareth will be in danger if he knows about the egg.",
                "foreshadowing": "Gareth mentions that he has heard rumors of King Vorath searching for a 'mystical artifact' – hinting at the danger to come.",
                "hook": "Just as Kael and Gareth finish their conversation, a loud knock is heard at the forge door – who could it be?"
              }
            }
          ]
        },
        {
          "id": "vol_1_2",
          "title": "Rising Action",
          "chapters": [
            {
              "id": "chap_1_2_1",
              "title": "Soldiers at the Gate",
              "summary": "King Vorath's soldiers enter Eldermore, demanding to search the village",
              "beats": [
                "Soldiers led by Captain Rook arrive at Eldermore's gate",
                "They announce they are searching for a 'mystical artifact'",
                "The villagers are fearful and comply with the search",
                "Kael hides the egg deeper in the forge, worried it will be found"
              ],
              "conflict": "External conflict between the villagers and the soldiers; Kael's fear of the egg being discovered",
              "pacing": "fast",
              "injections": {
                "pov_character": "char_1",
                "characters": ["char_1", "char_2", "char_4"],
                "locations": ["loc_1"],
                "items": ["item_1", "weapon_1"],
                "factions": ["faction_1", "faction_2"],
                "goal": "Keep the egg hidden from the soldiers and protect the village from harm",
                "conflict_detail": "Captain Rook and his soldiers are ruthless; they threaten to burn the village if the artifact is not found. Kael must hide the egg while also helping Gareth calm the villagers.",
                "foreshadowing": "Captain Rook glances at the forge and asks if anyone has been in the woods recently – he is already closing in on the egg.",
                "hook": "As the soldiers begin searching the forge, Kael realizes he forgot to close the hidden compartment – will the egg be found?"
              }
            }
          ]
        }
      ]
    }
  ]
}
```

# 5. Workflow Example

1. **Initialize Project**:
        `novelgen init --template fantasy
# Creates story_setup.json with theme, tense, and POV style`

2. **Generate 3-Level Outline**:
        `novelgen compose
# Creates outline.json with parts → volumes → chapters, including beats and conflict`

3. **Build World & Relationships**:
        `novelgen worldbuild
# Creates characters.json, locations.json, relationships.json, etc.`

4. **Inject Storylines**:
        `novelgen storyline
# Creates storylines.json with POV, goals, conflict, and foreshadowing`

5. **Manage Story Events**:
        `novelgen events --auto
# Extracts events from storylines and generates events.json`

6. **Validate Consistency**:
        `novelgen check --auto-fix
# Fixes minor consistency issues and generates a report`

7. **Draft Chapters**:
        `novelgen write --model gpt-4 --length 3000 --version
# Generates chapter drafts with versioning, using events from previous chapters`

8. **Iterate & Sync**:
        `# Edit characters.json (add new character Lira)
novelgen sync
# Updates storylines.json and events.json to include Lira
novelgen write --regenerate chap_1_2_1
# Regenerates Chapter 1.2.1 to include Lira and related events`

# 6. Key Features

- **Step-by-Step Context Injection**: Each phase builds on previous data, ensuring consistency across all story elements.

- **Flexible Input Methods**: Interactive prompts or JSON file inputs for each phase, catering to both beginners and advanced users.

- **Modular Design**: Focus on specific elements (e.g., only worldbuild characters, only regenerate a single chapter).

- **AI-Powered Drafting**: Leverage large language models for chapter generation, with strict constraints to avoid OOC or worldbreaking content.

- **Project Management**: Organized directory structure, version control, and rollback options for easy navigation and editing.

- **Consistency Checking**: Auto-validate story elements to prevent plot holes, OOC behavior, and rule violations.

- **Event Tracking System**: Maintain a chronological record of story events (item acquisitions, relationships, status changes) that influence subsequent chapters.

- **Auto-Sync**: Update downstream content automatically when edits are made to upstream files, saving time and reducing errors.