# Craft Characters Skill

## Purpose
Generate detailed character profiles based on story setup and outline context.

## Input
- `story_setup`: The story setup containing premise, genres, themes, etc.
- `outline`: Complete story outline structure (parts, volumes, chapters)
- `relevant_chapters`: Array of chapter summaries where these characters appear
- `characters`: List of character names to generate
- `custom_prompt`: Optional additional instructions

## Output
- `characters`: Map of character names to detailed character profiles

## Character Profile Structure

Each character must include:
- **name**: Full name of the character
- **aliases**: Array of nicknames or alternative names (optional)
- **age**: String describing age (e.g., "25", "unknown", "appears 20 but is ancient")
- **gender**: male/female/other (optional)
- **race**: Race or species (optional)
- **appearance**: Detailed physical description
- **personality**: Array of personality traits
- **background**: Character history and backstory (focus on past, not future)
- **motivation**: Core inner drive (static aspect of character)
- **skills**: Array of abilities/skills (optional)
- **abilities**: Special powers or talents (optional)
- **affiliations**: Organizations or groups the character belongs to (optional)
- **role_in_story**: Character's role (protagonist/antagonist/supporting/mentor/etc)
- **voice**: Speaking style and mannerisms (optional)
- **notes**: Additional notes for writers (optional)
- **rpg_role**: Optional RPG role: `player`, `npc`, `ally`, `enemy`, `boss`, `mentor`, `vendor`, or `quest_giver`
- **combat_role**: Optional combat function, such as `striker`, `tank`, `support`, `controller`, `scout`, or `noncombat`
- **power_level**: Optional non-negative integer starting power level for simulation
- **rpg_stats**: Optional object with numeric `str`, `agi`, `int`, `vit`, `hp`, `mp`, `level`
- **dsl_tags**: Optional stable tags for RPG-DSL conversion, such as factions, systems, archetypes, or encounter roles
- **state_effects**: Optional array of static state effects with `target`, `kind`, `field`, `from`, `to`, `delta`, `unit`, `cost`, `note`

## Guidelines

1. **Generate EXACTLY the requested characters** - no more, no less
2. Each character MUST have a unique and memorable personality
3. Characters should fit the story's genre and style
4. **Use relevant_chapters to understand character context** - these chapters show where and how the character appears in the story
5. **Align character motivation with their role in relevant chapters** - the character's actions should make sense given their motivation
6. Include specific details that can be referenced in writing
7. Focus on STATIC attributes only - do NOT include dynamic story elements
8. Use outline events, state anchors, enemies, and resource ledgers to set RPG/DSL metadata conservatively
9. Keep `rpg_stats` balanced; use small integers for early characters and only high values when the outline clearly supports them
10. Keep all content in the specified language

## IMPORTANT - DO NOT INCLUDE

- **relationships**: Relationships are managed dynamically by the story system
- **goals**: Character goals change throughout the story
- **character_arc**: Character development happens during the story
- **fears**: Fears may be revealed or change during the story
- Do not put future relationship/goal/arc progress into `state_effects`; only include static effects that are true when the character is introduced

## Output Format Example

```json
{
  "Character Name": {
    "name": "Character Name",
    "aliases": ["Nickname"],
    "age": "25",
    "gender": "male",
    "race": "Human",
    "appearance": "Tall with dark hair and piercing blue eyes...",
    "personality": ["brave", "stubborn", "compassionate"],
    "background": "Born in a small village, trained as a warrior...",
    "motivation": "To protect his family and prove his worth",
    "skills": ["sword fighting", "tracking"],
    "abilities": ["enhanced strength"],
    "affiliations": ["Royal Guard"],
    "role_in_story": "protagonist",
    "rpg_role": "player",
    "combat_role": "striker",
    "power_level": 3,
    "rpg_stats": {"str": 12, "agi": 11, "int": 14, "vit": 12, "hp": 120, "mp": 60, "level": 3},
    "dsl_tags": ["protagonist", "survival", "mecha_pilot"],
    "state_effects": [
      {"target": "protagonist", "kind": "status", "field": "identity", "to": "newly_awakened", "note": "Initial static identity for DSL simulation"}
    ],
    "voice": "Speaks formally but with occasional dry humor",
    "notes": "Has a mysterious scar on his left arm"
  }
}
```
