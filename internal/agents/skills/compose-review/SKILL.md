# Compose Review Skill

## Purpose
Review a story outline and provide comprehensive feedback on its structure, pacing, character development, and overall quality.

## Input
- `existing_outline`: The current story outline with parts, volumes, and chapters
- `setup` (optional): Story setup including premise, genres, themes, rules, storylines, and premises
- `user_prompt` (optional): Additional user suggestions for review focus. **If provided, prioritize the user's specific concerns when generating suggestions**

## Output
- `result`: ReviewResult containing:
  - `overall_score`: Overall quality score (0-100)
  - `dimensions`: Detailed scores by dimension
  - `summary`: Overall assessment
  - `strengths`: What's working well
  - `weaknesses`: Areas needing improvement
  - `suggestions`: Specific improvement suggestions

## Review Dimensions

### 1. Structure (25 points)
- Part/volume/chapter hierarchy is clear and logical
- Each level has appropriate scope and content
- Transitions between parts/volumes are smooth

### 2. Pacing (20 points)
- Chapter lengths are appropriate
- Tension builds progressively
- No sections feel rushed or dragging

### 3. Character Arcs (20 points)
- Characters show clear development
- Motivations are consistent
- Character interactions drive plot

### 4. Plot Coherence (20 points)
- Events flow logically
- Cause and effect relationships are clear
- No major plot holes

### 5. Engagement (15 points)
- Story hooks reader interest
- Cliffhangers and reveals are effective
- Emotional beats land well

## Output Format

```json
{
  "overall_score": 75,
  "dimensions": [
    {"name": "structure", "score": 20, "max": 25},
    {"name": "pacing", "score": 15, "max": 20},
    {"name": "character_arcs", "score": 18, "max": 20},
    {"name": "plot_coherence", "score": 17, "max": 20},
    {"name": "engagement", "score": 12, "max": 15}
  ],
  "summary": "The outline has a solid foundation but needs work on pacing and engagement.",
  "strengths": [
    "Strong character motivations",
    "Clear three-act structure"
  ],
  "weaknesses": [
    "Middle section drags",
    "Some chapters lack clear purpose"
  ],
  "suggestions": [
    {
      "category": "pacing",
      "target_id": "P1-V1-C5",
      "target_name": "The Middle Chapter",
      "issue": "Chapter lacks tension and feels like filler",
      "suggestion": "Add a major revelation or conflict escalation",
      "priority": "high"
    }
  ]
}
```

## Guidelines

1. Be specific in suggestions - reference exact chapters/sections
2. Prioritize issues by impact on story quality
3. Balance criticism with recognition of strengths
4. Consider the target genre and audience
5. Ensure suggestions are actionable
