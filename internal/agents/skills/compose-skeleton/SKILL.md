# Compose Skeleton Skill

## Purpose
Generate the story outline skeleton - parts and volumes only, without chapters.
This creates the high-level structure that will be filled in with chapters later.

## Input
- `setup`: StorySetup with premise, characters, storylines, etc.
- `structure`: StoryStructure with target parts and volumes

## Output
- `parts`: Array of Part objects, each containing:
  - `title`: Part title
  - `summary`: Part summary
  - `volumes`: Array of Volume objects, each containing:
    - `title`: Volume title
    - `summary`: Volume summary

## Structure Requirements

### Part Level
- Each part represents a major story arc
- Parts should have clear progression (setup → conflict → resolution)
- Part summaries should describe the overall arc

### Volume Level
- Each volume contains multiple chapters (will be generated later)
- Volume summaries should describe the specific storyline
- Volumes within a part should build toward the part's conclusion
- Volume-to-volume continuity must be maintained

## Output Format Example

```json
{
  "parts": [
    {
      "title": "Part 1: The Beginning",
      "summary": "Introduction to the world and main conflict",
      "volumes": [
        {
          "title": "Volume 1: Discovery",
          "summary": "Protagonist discovers their unique ability and faces first challenge"
        },
        {
          "title": "Volume 2: Awakening",
          "summary": "Protagonist masters their ability and confronts the antagonist's minions"
        }
      ]
    }
  ]
}
```

## Guidelines

1. **Arc Progression**: Each part should represent a complete story arc
2. **Volume Continuity**: Volume N+1 should directly follow from Volume N
3. **Balanced Scope**: Each volume should have similar scope and importance
4. **Setup Integration**: Use the story setup (premises, characters, storylines) to inform structure
5. **Genre Awareness**: Consider genre conventions when structuring parts and volumes
