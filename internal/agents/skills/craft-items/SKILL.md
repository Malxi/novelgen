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
- **rpg_item_type**: Optional RPG item type: `weapon`, `armor`, `consumable`, `artifact`, `document`, `resource`, `key`, or `material`
- **rarity**: Optional RPG rarity: `common`, `uncommon`, `rare`, `epic`, `legendary`, or `unique`
- **power_level**: Optional non-negative integer power level for simulation
- **quantity_tracking**: Optional boolean; true only for countable resources/currencies/consumables
- **dsl_tags**: Optional stable tags for RPG-DSL conversion
- **state_effects**: Optional array of static state effects with `target`, `kind`, `field`, `from`, `to`, `delta`, `unit`, `cost`, `note`

## Guidelines

1. **Generate EXACTLY the requested items** - no more, no less
2. Each item MUST have significance to the story
3. Items should fit the story's genre and style
4. **Use relevant_chapters to understand item context** - these chapters show how the item is used in the story
5. **Align item function and significance with events in relevant chapters** - the item should serve a clear purpose
6. Use resource ledgers and item events to set RPG type, rarity, quantity tracking, and state effects
7. Keep state effects deterministic and numeric when possible; do not invent future ownership changes beyond the outline
8. Include details about appearance, function, and importance
9. Keep all content in the specified language

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
    "rpg_item_type": "weapon",
    "rarity": "legendary",
    "power_level": 8,
    "quantity_tracking": false,
    "dsl_tags": ["artifact", "anti_dark_lord"],
    "state_effects": [
      {"target": "protagonist", "kind": "equipment", "field": "weapon", "to": "Item Name", "note": "Equipped when acquired"}
    ],
    "notes": "Glows brighter when near evil"
  }
}
```
