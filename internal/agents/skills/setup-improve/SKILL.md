# Setup Improve Skill

## Purpose
Improve a story setup based on review feedback or user guidance.

## Input
- `existing_setup`: The current story setup
- `review_result` (optional): ReviewResult with improvement suggestions including:
  - `summary`: Overall evaluation summary
  - `suggestions`: List of specific improvement suggestions with category, issue, suggestion, and priority
  - `strengths`: List of what works well (should be preserved)

## Output
- `setup`: The improved story setup

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

## Specific Improvement Areas

### World Building & Rules
- Fix logical inconsistencies in power systems
- Clarify rule boundaries and limitations
- Add missing constraints or consequences
- Strengthen cause-effect relationships

### Characters
- Enhance motivation clarity
- Deepen character backgrounds
- Strengthen character arcs
- Improve character relationships

### Plot & Storylines
- Strengthen main storyline progression
- Enhance subplot connections
- Add or refine plot hooks
- Improve pacing and tension

### Themes & Tone
- Clarify thematic elements
- Ensure tone consistency
- Strengthen emotional resonance
- Enhance genre alignment

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
