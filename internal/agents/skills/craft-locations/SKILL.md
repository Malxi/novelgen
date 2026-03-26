# Craft Locations Skill

## Purpose
Generate detailed location descriptions based on story setup and outline context.

## Input
- `story_setup`: The story setup containing premise, genres, themes, etc.
- `outline`: Complete story outline structure (parts, volumes, chapters)
- `relevant_chapters`: Array of chapter summaries where these locations appear
- `locations`: List of location names to generate
- `custom_prompt`: Optional additional instructions

## Output
- `locations`: Map of location names to detailed location profiles

## Location Profile Structure

Each location must include:
- **name**: Full location name
- **type**: Type of location (city/building/landmark/region/etc)
- **description**: Overall description
- **appearance**: Visual details and architecture
- **atmosphere**: Mood and feeling of the place
- **sensory_details**: Object with sights, sounds, smells, textures arrays (optional)
- **significance**: Why this location matters to the story
- **history**: Background history (optional)
- **inhabitants**: Array of types of people/creatures here (optional)
- **connected_locations**: Array of nearby place names (optional)
- **events**: Array of significant events that happened here (optional)
- **secrets**: Hidden aspects or secrets of this location as a string (optional)
- **notes**: Additional notes for writers (optional)

## Guidelines

1. **Generate EXACTLY the requested locations** - no more, no less
2. Each location MUST have a distinct atmosphere
3. Locations should fit the story's genre and style
4. **Use relevant_chapters to understand location context** - these chapters show what events happen at this location
5. **Align location significance with events in relevant chapters** - the location should support the story events
6. Include sensory details that can be used in writing
7. Keep all content in the specified language

## Output Format Example

```json
{
  "Location Name": {
    "name": "Location Name",
    "type": "ancient temple",
    "description": "A forgotten temple hidden deep in the mountains...",
    "appearance": "Crumbling stone walls covered in moss, with towering pillars...",
    "atmosphere": "Mysterious and foreboding, with an air of ancient power",
    "sensory_details": {
      "sights": ["flickering torches", "intricate carvings"],
      "sounds": ["dripping water", "howling wind"],
      "smells": ["incense", "damp stone"],
      "textures": ["rough stone", "smooth marble"]
    },
    "significance": "Contains the artifact needed to defeat the antagonist",
    "history": "Built 1000 years ago by an ancient civilization...",
    "inhabitants": ["ghosts", "treasure hunters"],
    "connected_locations": ["Mountain Pass", "Hidden Valley"],
    "events": ["The Great Ritual", "The Fall of the Temple"],
    "secrets": "A hidden chamber beneath the altar contains...",
    "notes": "Best used for the climax scene"
  }
}
```
