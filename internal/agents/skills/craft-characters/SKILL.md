# Craft Characters Skill

## Purpose
Generate detailed character profiles based on story setup and outline context.

## Input
- `story_setup`: The story setup containing premise, genres, themes, etc.
- `outline`: Complete story outline structure (parts, volumes, chapters)
- `relevant_chapters`: Array of chapter summaries where these characters appear
- `characters`: List of character names to generate
- `custom_prompt`: Optional additional instructions

## Output
- `characters`: Map of character names to detailed character profiles

## Character Profile Structure

Each character must include:
- **name**: Full name of the character
- **aliases**: Array of nicknames or alternative names (optional)
- **age**: String describing age (e.g., "25", "unknown", "appears 20 but is ancient")
- **gender**: male/female/other (optional)
- **race**: Race or species (optional)
- **appearance**: Detailed physical description
- **personality**: Array of personality traits
- **background**: Character history and backstory (focus on past, not future)
- **motivation**: Core inner drive (static aspect of character)
- **skills**: Array of abilities/skills (optional)
- **abilities**: Special powers or talents (optional)
- **affiliations**: Organizations or groups the character belongs to (optional)
- **role_in_story**: Character's role (protagonist/antagonist/supporting/mentor/etc)
- **voice**: Speaking style and mannerisms (optional)
- **notes**: Additional notes for writers (optional)

## Guidelines

1. **Generate EXACTLY the requested characters** - no more, no less
2. Each character MUST have a unique and memorable personality
3. Characters should fit the story's genre and style
4. **Use relevant_chapters to understand character context** - these chapters show where and how the character appears in the story
5. **Align character motivation with their role in relevant chapters** - the character's actions should make sense given their motivation
6. Include specific details that can be referenced in writing
7. Focus on STATIC attributes only - do NOT include dynamic story elements
8. Keep all content in the specified language

## IMPORTANT - DO NOT INCLUDE

- **relationships**: Relationships are managed dynamically by the story system
- **goals**: Character goals change throughout the story
- **character_arc**: Character development happens during the story
- **fears**: Fears may be revealed or change during the story

## Output Format Example

```json
{
  "Character Name": {
    "name": "Character Name",
    "aliases": ["Nickname"],
    "age": "25",
    "gender": "male",
    "race": "Human",
    "appearance": "Tall with dark hair and piercing blue eyes...",
    "personality": ["brave", "stubborn", "compassionate"],
    "background": "Born in a small village, trained as a warrior...",
    "motivation": "To protect his family and prove his worth",
    "skills": ["sword fighting", "tracking"],
    "abilities": ["enhanced strength"],
    "affiliations": ["Royal Guard"],
    "role_in_story": "protagonist",
    "voice": "Speaks formally but with occasional dry humor",
    "notes": "Has a mysterious scar on his left arm"
  }
}
```
