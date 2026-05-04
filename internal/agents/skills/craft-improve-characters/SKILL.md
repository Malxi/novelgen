# Craft Improve Characters Skill

## Purpose
Improve character profiles based on review feedback while maintaining consistency with the story.

## Input
- `story_setup`: The story setup for context
- `outline`: Story outline summary for context
- `characters`: Current character profiles
- `review_result`: Review result with suggestions
- `custom_prompt`: Optional additional instructions

## Output
- `characters`: Improved character profiles

## Improvement Guidelines

1. **Address High Priority Issues First**
   - Focus on suggestions marked as "high" priority
   - Fix consistency problems with story setup
   - Add missing depth and detail

2. **Maintain Character Identity**
   - Keep the core personality intact
   - Preserve established background elements
   - Enhance rather than replace

3. **Enhance Static Attributes**
   - Improve physical descriptions
   - Deepen background and motivation
   - Add specific details and quirks
   - Strengthen voice and mannerisms

4. **Ensure Story Alignment**
   - Verify motivation aligns with story premise
   - Check that skills fit the world
   - Confirm role serves the narrative
   - Preserve and improve RPG/DSL metadata (`rpg_role`, `combat_role`, `power_level`, `rpg_stats`, `dsl_tags`, `state_effects`)
   - Keep simulation stats conservative and consistent with outline events

5. **Preserve Valid Elements**
   - Don't remove good content
   - Build on existing strengths
   - Keep what works

## Output Format

Return the complete set of characters as a JSON object with character names as keys. All characters should be included, even those not directly mentioned in the review suggestions.

```json
{
  "Character Name": {
    "name": "Character Name",
    "aliases": ["Nickname"],
    "age": "25",
    "gender": "male",
    "race": "Human",
    "appearance": "Enhanced detailed description...",
    "personality": ["brave", "stubborn", "compassionate"],
    "background": "Enhanced backstory with specific details...",
    "motivation": "Clear motivation tied to story...",
    "skills": ["sword fighting", "tracking"],
    "abilities": ["enhanced strength"],
    "affiliations": ["Royal Guard"],
    "role_in_story": "protagonist",
    "rpg_role": "player",
    "combat_role": "striker",
    "power_level": 3,
    "rpg_stats": {"str": 12, "agi": 11, "int": 14, "vit": 12, "hp": 120, "mp": 60, "level": 3},
    "dsl_tags": ["protagonist"],
    "state_effects": [],
    "voice": "Distinct speaking style...",
    "notes": "Additional writer notes..."
  }
}
```
