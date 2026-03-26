# Craft Review Items Skill

## Purpose
Review item descriptions and provide improvement suggestions based on story significance, consistency, and quality.

## Input
- `story_setup`: The story setup for context
- `outline`: Story outline summary for context
- `items`: Map of item names to item profiles
- `iteration`: Current iteration number

## Output
- `result`: Review result containing scores and suggestions

## Review Result Structure

The review result must include:
- **overall_score**: Overall quality score (0-100)
- **dimensions**: Array of dimension scores:
  - `consistency`: Consistency with story setup and outline (0-25)
  - `significance`: Story significance and purpose (0-25)
  - `originality`: Uniqueness and creativity (0-25)
  - `usability`: Practical use for writers (0-25)
- **summary**: Brief summary of the review
- **strengths**: Array of strong points
- **weaknesses**: Array of weak points
- **suggestions`: Array of specific improvement suggestions with:
  - `target_id`: Item name or "all"
  - `issue`: Description of the problem
  - `suggestion`: How to fix it
  - `priority`: "high", "medium", or "low"

## Guidelines

1. Evaluate items against the story setup and outline
2. Check for consistency in world-building and mechanics
3. Assess significance - does each item serve a story purpose?
4. Look for uniqueness - avoid generic magic items
5. Consider usability - do writers have enough detail to work with?
6. Provide specific, actionable suggestions
7. Prioritize high-impact issues

## Output Format Example

```json
{
  "overall_score": 72.0,
  "dimensions": [
    {"name": "consistency", "score": 19.0, "max": 25.0},
    {"name": "significance", "score": 17.0, "max": 25.0},
    {"name": "originality", "score": 18.0, "max": 25.0},
    {"name": "usability", "score": 18.0, "max": 25.0}
  ],
  "summary": "Items have good variety but some lack clear story purpose...",
  "strengths": [
    "Creative powers and limitations",
    "Good visual descriptions"
  ],
  "weaknesses": [
    "Some items feel disconnected from the plot",
    "Origins need more detail"
  ],
  "suggestions": [
    {
      "target_id": "Mystic Amulet",
      "issue": "Unclear connection to main plot",
      "suggestion": "Link the amulet's power to the protagonist's goal",
      "priority": "high"
    }
  ]
}
```
