# Compose Improve Skill

## Purpose
Improve a story outline based on review feedback or user guidance.

## Input
- `existing_outline`: The current story outline
- `review_result` (optional): ReviewResult with improvement suggestions
- `user_prompt` (optional): Additional user suggestions for improvement
- `setup_brief` (optional): Compact story setup contract including premise, rules, long-form plan, core cast seeds, storylines, progression systems, and resources. Use it as the setup authority without expanding it into encyclopedic detail.
- `revision_context` (optional): Compact session memory from earlier outline review and improve rounds. Use it to keep continuity across iterations and avoid undoing already-fixed structure.

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
- If a setup storyline has `repeatable_pressure`, `payoff_cadence`, `mutation`, or `failure_mode`, repair the outline to respect those hints without making every chapter formulaic.
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

### Long-Form Plan Alignment
- If setup includes `long_form_plan`, use it as the repair target for scale, escalation, payoff cadence, and volume identity.
- Repair outlines whose parts/volumes repeat the same arena instead of following `escalation_ladder`.
- Repair volumes that ignore `volume_pattern` by adding clearer question, pressure, exploit, big win, visible reward, and next gate.
- If the outline is intentionally shorter than `target_chapters`, keep the current structure but still preserve the plan's main loop and escalation logic.

## Guidelines

1. Preserve the overall story structure unless major issues exist
2. Make targeted changes based on specific feedback
3. Ensure all chapters maintain required fields
4. Keep character and location references consistent
5. Maintain continuity with previous and following chapters
6. If `user_prompt` is provided, prioritize the user's specific requests alongside review feedback
7. Keep storyline tracking flexible: do not add `storyline_advances` to every chapter, and do not convert the outline into a rigid formula
8. Keep爽点 design concrete: wins should use this book's unique setup, not generic luck, generic strength, or unexplained author convenience

9. If `revision_context` is present, treat it as the current revision trail and continue from the previous round instead of starting over

## Output Requirements

The improved outline must include:
- All required parts, volumes, and chapters
- Complete chapter information (summary, beats, events, etc.)
- Consistent IDs and references
- Proper pacing and tension progression
