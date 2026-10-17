# Craft Review Locations Skill

## Purpose
Review location descriptions and provide improvement suggestions based on story consistency, atmosphere, and quality.

## Input
- `story_setup`: The story setup for context
- `outline`: Story outline summary for context
- `locations`: Map of location names to location profiles
- `iteration`: Current iteration number

## Output
- `result`: Review result containing scores and suggestions

## Review Result Structure

The review result must include:
- **overall_score**: Overall quality score (0-100)
- **dimensions**: Array of dimension scores:
  - `consistency`: Consistency with story setup and outline (0-25)
  - `atmosphere`: Atmospheric detail and mood (0-25)
  - `originality`: Uniqueness and creativity (0-25)
  - `usability`: Practical use for writers (0-25)
- **summary**: Brief summary of the review
- **strengths**: Array of strong points
- **weaknesses**: Array of weak points
- **suggestions`: Array of specific improvement suggestions with:
  - `target_id`: Location name or "all"
  - `issue`: Description of the problem
  - `suggestion`: How to fix it
  - `priority`: "high", "medium", or "low"

## Guidelines

1. Evaluate locations against the story setup and outline
2. Check for consistency in world-building
3. Assess atmosphere - is the mood clear and evocative?
4. Look for uniqueness - avoid generic settings
5. Consider usability - do writers have enough sensory detail?
6. Check RPG/DSL metadata: `rpg_map_type`, `danger_level`, encounter/resource tags, and state effects should be conservative and simulatable
7. Provide specific, actionable suggestions
8. Prioritize high-impact issues

## Output Format Example

```json
{
  "overall_score": 78.0,
  "dimensions": [
    {"name": "consistency", "score": 20.0, "max": 25.0},
    {"name": "atmosphere", "score": 19.0, "max": 25.0},
    {"name": "originality", "score": 20.0, "max": 25.0},
    {"name": "usability", "score": 19.0, "max": 25.0}
  ],
  "summary": "Locations are atmospheric and well-integrated...",
  "strengths": [
    "Strong sensory details",
    "Good connection to story events"
  ],
  "weaknesses": [
    "Some locations lack historical context",
    "Atmosphere could be more distinct"
  ],
  "suggestions": [
    {
      "target_id": "Ancient Temple",
      "issue": "Missing historical significance",
      "suggestion": "Add connection to the story's ancient civilization",
      "priority": "medium"
    }
  ]
}
```
