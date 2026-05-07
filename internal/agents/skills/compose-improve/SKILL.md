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

### 5. DSL Simulation Feedback
- Treat review suggestions mentioning DSL, simulation, storyline contracts, durable state change, payoff, resource budget, or progression bounds as structural feedback.
- If a setup storyline is not advanced, add sparse `storyline_advances` to the most natural key chapters rather than every chapter.
- If a chapter has activity but no durable state change, clarify what changed: goal, relationship, resource, injury, clue, pressure, consequence, or payoff.
- If a payoff is promised but not represented, either add a payoff/recovery chapter movement or soften the setup-facing promise through the outline's arc.
- If resource/progression changes lack support, repair `events`, `resource_ledger`, or `state_anchor` so the simulator can trace the change.

### 6. Engagement Improvements
- Add hooks and reveals
- Enhance emotional beats
- Improve chapter endings
- Repair weak `payoff_contract` fields on volumes when review feedback says the volume lacks reader promise, big win, visible reward, or escalation.
- Repair weak `chapter_payoff` fields when a chapter has events but no satisfying win pattern. Make the chain concrete: desire → pressure → clever move → payoff moment → reward → social proof → hook.
- If a chapter already has pressure but no爽点, do not simply increase danger. Add a more interesting exploit, opponent misread, or visible reward.

## Guidelines

1. Preserve the overall story structure unless major issues exist
2. Make targeted changes based on specific feedback
3. Ensure all chapters maintain required fields
4. Keep character and location references consistent
5. Maintain continuity with previous and following chapters
6. If `user_prompt` is provided, prioritize the user's specific requests alongside review feedback
7. Keep storyline tracking flexible: do not add `storyline_advances` to every chapter, and do not convert the outline into a rigid formula
8. Keep爽点 design concrete: wins should use this book's unique setup, not generic luck, generic strength, or unexplained author convenience

## Output Requirements

The improved outline must include:
- All required parts, volumes, and chapters
- Complete chapter information (summary, beats, events, etc.)
- Consistent IDs and references
- Proper pacing and tension progression
