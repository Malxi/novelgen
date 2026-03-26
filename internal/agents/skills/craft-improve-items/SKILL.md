# Craft Improve Items Skill

## Purpose
Improve item descriptions based on review feedback while maintaining consistency with the story world.

## Input
- `story_setup`: The story setup for context
- `outline`: Story outline summary for context
- `items`: Current item profiles
- `review_result`: Review result with suggestions
- `custom_prompt`: Optional additional instructions

## Output
- `items`: Improved item profiles

## Improvement Guidelines

1. **Address High Priority Issues First**
   - Focus on suggestions marked as "high" priority
   - Fix consistency problems with story setup
   - Strengthen story significance

2. **Maintain Item Identity**
   - Keep the core concept intact
   - Preserve established powers and limitations
   - Enhance rather than replace

3. **Enhance Significance**
   - Clarify connection to plot
   - Strengthen story purpose
   - Add meaningful history
   - Expand origin details

4. **Ensure Story Alignment**
   - Verify function fits the world
   - Check power level consistency
   - Confirm role in narrative

5. **Preserve Valid Elements**
   - Don't remove good content
   - Build on existing strengths
   - Keep what works

## Output Format

Return the complete set of items as a JSON object with item names as keys. All items should be included, even those not directly mentioned in the review suggestions.

```json
{
  "Item Name": {
    "name": "Item Name",
    "type": "legendary weapon",
    "description": "Enhanced detailed description...",
    "appearance": "Vivid visual details...",
    "function": "Clear purpose and use...",
    "origin": "Detailed background...",
    "history": "Rich historical context...",
    "powers": ["power 1", "power 2"],
    "limitations": ["limitation 1", "limitation 2"],
    "owner": "Current or original owner",
    "significance": "Strong story purpose...",
    "related_items": ["Item 1", "Item 2"],
    "secrets": "Hidden aspects...",
    "notes": "Writer notes..."
  }
}
```
