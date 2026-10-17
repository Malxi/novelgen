# Craft Improve Organizations Skill

## Purpose
Improve organization/faction profiles using review feedback while preserving canonical names and story contracts.

## Input
- `story_setup`: Story setup
- `outline`: Outline summary
- `organizations`: Current organization profiles
- `review_result`: Review findings and suggestions
- `custom_prompt`: Optional additional instructions

## Output
- `organizations`: Improved organization profiles keyed by the same names

## Improvement Rules

1. Preserve every input map key; do not rename, drop, or add organizations
2. Strengthen goals, resources, leadership, reputation, and story significance
3. Align with setup premises and world resources without redefining ability systems
4. Keep future dynamic changes out of static profiles
5. Clarify faction relationships only when supported by setup, outline, or review feedback
6. Keep `state_effects` conservative and static
7. Keep all content in the specified language
