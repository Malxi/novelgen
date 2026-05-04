# Craft Improve Locations Skill

## Purpose
Improve location descriptions based on review feedback while maintaining consistency with the story world.

## Input
- `story_setup`: The story setup for context
- `outline`: Story outline summary for context
- `locations`: Current location profiles
- `review_result`: Review result with suggestions
- `custom_prompt`: Optional additional instructions

## Output
- `locations`: Improved location profiles

## Improvement Guidelines

1. **Address High Priority Issues First**
   - Focus on suggestions marked as "high" priority
   - Fix consistency problems with story setup
   - Enhance atmosphere and sensory details

2. **Maintain Location Identity**
   - Keep the core concept intact
   - Preserve established history
   - Enhance rather than replace

3. **Enhance Atmosphere**
   - Add vivid sensory details
   - Strengthen mood and feeling
   - Include specific visual elements
   - Expand environmental descriptions

4. **Ensure Story Alignment**
   - Verify significance to plot
   - Check historical consistency
   - Confirm connections to events
   - Preserve and improve RPG/DSL metadata (`rpg_map_type`, `danger_level`, tags, `state_effects`)
   - Keep danger and state effects conservative and supported by the outline

5. **Preserve Valid Elements**
   - Don't remove good content
   - Build on existing strengths
   - Keep what works

## Output Format

Return the complete set of locations as a JSON object with location names as keys. All locations should be included, even those not directly mentioned in the review suggestions.

```json
{
  "Location Name": {
    "name": "Location Name",
    "type": "ancient temple",
    "description": "Enhanced vivid description...",
    "appearance": "Detailed visual elements...",
    "atmosphere": "Strong evocative mood...",
    "sensory_details": {
      "sights": ["specific visual 1", "specific visual 2"],
      "sounds": ["ambient sound 1", "ambient sound 2"],
      "smells": ["scent 1", "scent 2"],
      "textures": ["texture 1", "texture 2"]
    },
    "significance": "Clear story purpose...",
    "history": "Enhanced background...",
    "inhabitants": ["type 1", "type 2"],
    "connected_locations": ["Place 1", "Place 2"],
    "events": ["Event 1", "Event 2"],
    "secrets": "Hidden aspects...",
    "rpg_map_type": "dungeon",
    "danger_level": 5,
    "encounter_tags": ["tag 1"],
    "resource_tags": ["resource 1"],
    "dsl_tags": ["stable_tag"],
    "state_effects": [],
    "notes": "Writer notes..."
  }
}
```
