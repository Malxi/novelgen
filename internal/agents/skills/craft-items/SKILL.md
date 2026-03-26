# Craft Items Skill

## Purpose
Generate detailed item descriptions based on story setup and outline context.

## Input
- `story_setup`: The story setup containing premise, genres, themes, etc.
- `outline`: Complete story outline structure (parts, volumes, chapters)
- `relevant_chapters`: Array of chapter summaries where these items appear
- `items`: List of item names to generate
- `custom_prompt`: Optional additional instructions

## Output
- `items`: Map of item names to detailed item profiles

## Item Profile Structure

Each item must include:
- **name**: Full item name
- **type**: Type of item (weapon/artifact/tool/document/etc)
- **description**: Overall description
- **appearance**: Visual details
- **function**: What the item does or is used for
- **origin**: Where the item comes from (optional)
- **history**: Background history of the item (optional)
- **powers**: Array of special abilities (optional)
- **limitations**: Array of restrictions or weaknesses (optional)
- **owner**: Current or original owner (optional)
- **significance**: Why this item matters to the story
- **related_items**: Array of related item names (optional)
- **secrets**: Hidden aspects of this item (optional)
- **notes**: Additional notes for writers (optional)

## Guidelines

1. **Generate EXACTLY the requested items** - no more, no less
2. Each item MUST have significance to the story
3. Items should fit the story's genre and style
4. **Use relevant_chapters to understand item context** - these chapters show how the item is used in the story
5. **Align item function and significance with events in relevant chapters** - the item should serve a clear purpose
6. Include details about appearance, function, and importance
7. Keep all content in the specified language

## Output Format Example

```json
{
  "Item Name": {
    "name": "Item Name",
    "type": "legendary weapon",
    "description": "An ancient sword forged from star metal...",
    "appearance": "The blade shimmers with an ethereal blue light...",
    "function": "Can cut through any material and channel magical energy",
    "origin": "Forged by the Celestial Smiths in the Age of Gods",
    "history": "Wielded by heroes throughout history, lost for centuries...",
    "powers": ["indestructible blade", "magic channeling", "light emission"],
    "limitations": ["requires pure heart to wield", "drains user's energy"],
    "owner": "The Chosen One",
    "significance": "Key to defeating the Dark Lord",
    "related_items": ["Shield of Dawn", "Armor of Light"],
    "secrets": "Contains a fragment of a god's soul",
    "notes": "Glows brighter when near evil"
  }
}
```
