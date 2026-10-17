# Compose Skeleton Improve Skill

## Purpose
Improve an outline skeleton based on review feedback or user guidance.
Only improve parts, volumes, summaries, and volume payoff contracts.

## Input
- `existing_outline`: Current outline skeleton.
- `review_result` (optional): ReviewResult with suggestions.
- `user_prompt` (optional): User guidance.
- `setup_brief`: Compact story setup contract.
- `structure`: Expected target parts and volumes.
- `revision_context` (optional): Previous review/improve trail.

## Output
Return the complete improved `Outline` JSON object.

## Improvement Rules

1. Preserve the exact number of parts and volumes.
2. Do not generate chapters.
3. Do not remove existing chapters if they appear in the input. Leave chapter
   arrays unchanged or omit them; the caller will preserve existing chapters.
4. Preserve the setup's protagonist limits, main loop, long-form escalation, and
   core cast logic.
5. Improve titles, summaries, volume-to-volume causality, and payoff contracts.
6. Make volume hooks readable to web-novel readers: concrete, dramatic, and
   easy to pitch.
7. Avoid abstract backend terms unless the setup explicitly wants them.

## Target Shape

Each volume should answer:
- What arena or gameplay makes this volume different?
- Why is the protagonist dragged in?
- What does the opponent misunderstand?
- What clever rule exploit creates the big win?
- What visible reward and reputation shift remain after the volume?
- What next bigger game opens because of this win?

## Output Requirements
- Return only the improved outline object.
- Include all original parts and volumes.
- Every volume should include `title`, `summary`, `payoff_contract`, and
  `chapters`.
- `chapters` should stay empty for skeleton-only volumes.

