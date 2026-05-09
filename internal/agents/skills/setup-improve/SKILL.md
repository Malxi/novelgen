# Setup Improve Skill

## Purpose
Improve a story setup based on review feedback or user guidance.

## Input
- `existing_setup`: The current story setup
- `review_result` (optional): ReviewResult with improvement suggestions including:
  - `summary`: Overall evaluation summary
  - `suggestions`: List of specific improvement suggestions with category, issue, suggestion, and priority
  - `strengths`: List of what works well (should be preserved)
- `revision_context` (optional): Compact session memory from earlier generation, review, and improve rounds. Use it to keep continuity across rounds, preserve already-accepted strengths, and avoid reworking fixed issues.

## Output
- `setup`: The improved story setup

## Setup Compactness

Keep setup as a compact creation contract. If review feedback asks for more
depth, add the smallest seed, rule, progression boundary, or payoff promise
that solves the issue. Do not expand setup into full biographies, exhaustive
lore, minor cast catalogs, chapter plans, or item encyclopedias. Oversized
sections should be compressed into reusable functions and moved conceptually to
craft or notes.

## Improvement Strategy

### 1. Analyze Review Feedback
- Read all suggestions carefully, ordered by priority (high → medium → low)
- Understand the root cause of each issue
- Identify patterns across multiple suggestions

### 2. Prioritize Changes
**Must Fix (High Priority):**
- Logic inconsistencies in rules/premises
- Critical gaps in world-building
- Major character motivation issues
- Core conflict problems

**Should Fix (Medium Priority):**
- Pacing and tension issues
- Underdeveloped storylines
- Weak hooks or reveals
- Character depth issues

**Nice to Fix (Low Priority):**
- Minor clarity improvements
- Additional detail suggestions
- Style and tone refinements

### 3. Apply Improvements Systematically
For each suggestion:
1. **Locate** the relevant section in existing_setup
2. **Understand** the issue and desired outcome
3. **Modify** the content to address the issue
4. **Verify** the change doesn't break other aspects

### 4. Preserve Strengths
- Keep elements marked as strengths in the review
- Don't over-correct and lose what works well
- Maintain the core vision and unique aspects

### 5. Use Revision Context
- If `revision_context` is present, treat it as the running memory of the current session
- Follow the order of history: generation -> review -> improve -> review
- Do not re-open issues that were already addressed unless the current review shows they still fail
- Keep the revision focused on the current round's highest-value fixes

## Specific Improvement Areas

### World Building & Rules
- Fix logical inconsistencies in power systems
- Clarify rule boundaries and limitations
- Add missing constraints or consequences
- Strengthen cause-effect relationships
- Preserve one clear root premise, but expand long-form genre setups into 3-6 derived `premises` systems when they are thin. Do not create unrelated cores; split the same world logic into simulatable tracks such as protagonist growth, enemy tiers, faction technology, resource economy, social/faction hierarchy, and final external threat.
- Every added or revised `premises[]` system should have a progression ladder with named stages, requirements/costs, ceilings, and a clear story use. Avoid one vague omnipotent upgrade system.
- For power-fantasy/web-novel setups, prefer "surface limits the protagonist can exploit" over grim mandatory costs. A good rule should create clever wins, enemy misreads, and visible rewards.

### Long-Form Capacity
- Add or refine `long_form_plan` when the setup targets or implies serial/web-novel scale, 300+ chapters, 1000 chapters, group cast, or multi-volume progression.
- Keep it as a high-level capacity contract: target_chapters, target_volumes, main_loop, escalation_ladder, reader_promises, payoff_cadence, volume_pattern, midpoint_mutation, and endgame_promise.
- Use it to make the story easier for compose to outline: define repeatable wins, escalation stages, and when bigger payoffs should land.
- Do not add concrete chapter lists here. Compose owns the actual parts, volumes, and chapters.

### Characters
- Use `core_cast` for setup-level character seeds, not complete character cards
- Add or refine core cast seeds when the setup needs stronger long-form character engines
- Each important seed should define role, importance, story_function, relationship_arc, entry_phase, payoff, and storyline_refs when useful
- Do not overfill biography, appearance, exact skills, stats, or detailed backstory here; craft owns full character cards

### Plot & Storylines
- Strengthen main storyline progression
- Enhance subplot connections
- Add or refine plot hooks
- Improve pacing and tension
- Optionally enrich storylines with high-level contract hints such as `scope`, `payoff_style`, `setup_role`, `desire`, `opposition`, `stakes`, `turn`, `payoff`, `open_question`, or `pressure_points` when those hints create real pressure or payoff
- For important storylines (`importance >= 8`), prefer a concrete arc contract: `scope`, `setup_role`, `payoff_style`, 2-4 `pressure_points`, and the most relevant desire/opposition/stakes/open_question/payoff fields
- For important long-form storylines, add or refine serial-engine hints when they are thin: `repeatable_pressure`, `payoff_cadence`, `mutation`, and `failure_mode`
- Keep `scope` high-level (`opening`, `volume`, `book`, `series`) and avoid chapter-specific planning in setup
- Use `payoff_style` to distinguish immediate payoff from staged reveals or slow-burn promises
- Add or refine optional `appeal_engine` on important storylines when the arc lacks a repeatable爽点: define `appeal`, `surface_limit`, `exploit`, `signature_win`, `upgrade_path`, `opponent_misread`, and `reward_type`.
- Add optional `appeal_engine` on core `premises[]` when a setting system is logically clear but not yet fun to write. The key question is: how can the protagonist use this unique setting to win in a way readers can picture?

### DSL Simulation Feedback
- Treat review suggestions from `category: logic`, `plot`, `conflict`, `structure`, or `consistency` as possible DSL simulation feedback.
- If the feedback says a storyline contract is under-specified, improve the relevant `storylines` entry with 2-4 useful hints: desire, opposition, stakes, open_question, pressure_points, or payoff.
- If the feedback says a long-running storyline promises a payoff too early, prefer adding `scope`/`payoff_style`/`setup_role` over turning setup into a detailed outline.
- If the feedback says setup rules/resources/progression are missing, add or clarify `rules`, `premises.progression`, or `world_resources` so later outline simulation can track costs and state changes.
- Keep these as creative contracts, not rigid chapter formulas. The setup should give later agents pressure and boundaries while leaving room for invention.

### Themes & Tone
- Clarify thematic elements
- Ensure tone consistency
- Strengthen emotional resonance
- Enhance genre alignment
- Preserve or refine `writing_style` when user guidance or review feedback asks for a specific prose voice
- If adding a reference excerpt, keep it short and mark it as style-only: it must not become story canon, plot, characters, places, or terminology

### Setting & Atmosphere
- Add sensory details
- Clarify setting rules
- Enhance atmosphere descriptions
- Fix setting inconsistencies

## Guidelines

1. **Address ALL high priority suggestions** - These are critical issues
2. **Address most medium priority suggestions** - These significantly improve quality
3. **Address low priority suggestions if time/space permits**
4. **Maintain consistency** - Changes in one area shouldn't break another
5. **Preserve the core concept** - Don't change the fundamental premise
6. **Keep valid content** - Don't remove elements that work well
7. **Ensure completeness** - All required fields must be populated
8. **Keep storyline texture flexible** - Do not fill every optional storyline field mechanically; choose the few hints that make the arc more alive
9. **Make wins designable** - Important rules and arcs should imply a satisfying win pattern, not just background logic

## Output Requirements

The improved setup must include:
- All required fields populated
- Review suggestions addressed
- Strengths preserved
- Consistent internal logic
- Clear and specific content (avoid vagueness)
- Proper structure and formatting

## Response Format

Provide the complete improved story setup with all changes integrated naturally.
