# Compose Improve Skill

## Purpose
Improve a story outline based on review feedback or user guidance.

## Input
- `existing_outline`: The current story outline
- `review_result` (optional): ReviewResult with improvement suggestions
- `user_prompt` (optional): Additional user suggestions for improvement
- `setup` (optional): Story setup including premise, genres, themes, rules, storylines, and premises

## Output
- `outline`: The improved story outline

## Improvement Areas

### 1. Structure Fixes
- Reorganize chapters for better flow
- Adjust part/volume boundaries
- Balance content distribution

### 2. Pacing Adjustments
- Add or remove beats as needed
- Adjust chapter tension levels
- Ensure proper buildup to climaxes

### 3. Character Development
- Strengthen character arcs
- Clarify motivations
- Improve character interactions

### 4. Plot Enhancements
- Fix logical inconsistencies
- Add missing plot points
- Strengthen cause-effect chains
- When a storyline feels thin, optionally add or refine `storyline_advances` on chapters that create real pressure, reveal, choice, consequence, reversal, or payoff

### 5. Engagement Improvements
- Add hooks and reveals
- Enhance emotional beats
- Improve chapter endings

## Guidelines

1. Preserve the overall story structure unless major issues exist
2. Make targeted changes based on specific feedback
3. Ensure all chapters maintain required fields
4. Keep character and location references consistent
5. Maintain continuity with previous and following chapters
6. If `user_prompt` is provided, prioritize the user's specific requests alongside review feedback
7. Keep storyline tracking flexible: do not add `storyline_advances` to every chapter, and do not convert the outline into a rigid formula

## Output Requirements

The improved outline must include:
- All required parts, volumes, and chapters
- Complete chapter information (summary, beats, events, etc.)
- Consistent IDs and references
- Proper pacing and tension progression
