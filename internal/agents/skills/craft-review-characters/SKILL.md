# Craft Review Characters Skill

## Purpose
Review character profiles and provide improvement suggestions based on story consistency, depth, and quality.

## Input
- `story_setup`: The story setup for context
- `outline`: Story outline summary for context
- `characters`: Map of character names to character profiles
- `iteration`: Current iteration number

## Output
- `result`: Review result containing scores and suggestions

## Review Result Structure

The review result must include:
- **overall_score**: Overall quality score (0-100)
- **dimensions**: Array of dimension scores:
  - `consistency`: Consistency with story setup and outline (0-25)
  - `depth`: Character depth and complexity (0-25)
  - `originality`: Uniqueness and creativity (0-25)
  - `usability`: Practical use for writers (0-25)
- **summary**: Brief summary of the review
- **strengths**: Array of strong points
- **weaknesses**: Array of weak points
- **suggestions`: Array of specific improvement suggestions with:
  - `target_id`: Character name or "all"
  - `issue`: Description of the problem
  - `suggestion`: How to fix it
  - `priority`: "high", "medium", or "low"

## Guidelines

1. Evaluate characters against the story setup and outline
2. Check for consistency in personality, background, and motivation
3. Assess depth - are characters multi-dimensional?
4. Look for uniqueness - avoid clichés and stereotypes
5. Consider usability - do writers have enough detail to work with?
6. Check RPG/DSL metadata: `rpg_role`, `combat_role`, `power_level`, `rpg_stats`, `dsl_tags`, and `state_effects` should be consistent with outline events and not overpowered
7. Provide specific, actionable suggestions
8. Prioritize high-impact issues

## Output Format Example

```json
{
  "overall_score": 75.0,
  "dimensions": [
    {"name": "consistency", "score": 20.0, "max": 25.0},
    {"name": "depth", "score": 18.0, "max": 25.0},
    {"name": "originality", "score": 19.0, "max": 25.0},
    {"name": "usability", "score": 18.0, "max": 25.0}
  ],
  "summary": "Characters are well-developed with good consistency...",
  "strengths": [
    "Strong motivation alignment with story premise",
    "Distinct personalities"
  ],
  "weaknesses": [
    "Some backgrounds lack detail",
    "Physical descriptions could be more vivid"
  ],
  "suggestions": [
    {
      "target_id": "John Doe",
      "issue": "Background is too generic",
      "suggestion": "Add specific childhood event that shaped his motivation",
      "priority": "high"
    }
  ]
}
```
