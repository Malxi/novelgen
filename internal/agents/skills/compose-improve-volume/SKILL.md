# Compose Improve Volume Skill

## Purpose
Improve the chapters in a specific volume based on review feedback, while maintaining continuity with the rest of the outline.

## Input
- `outline`: The complete story outline (for context)
- `part`: The current Part information
- `volume`: The Volume to improve (with current chapters)
- `review_result`: Review feedback specific to this volume

## Output
- `volume`: The improved Volume with updated chapters

## Improvement Focus

### 1. Address Review Suggestions
- Fix specific issues mentioned in review feedback
- Improve pacing where noted
- Strengthen character arcs as suggested
- Fix plot coherence issues

### 2. Maintain Continuity
- First chapter must connect to previous volume's ending
- Last chapter must set up for next volume (if any)
- Chapter-to-chapter beats must flow logically

### 3. Preserve Volume Identity
- Keep the volume's title and summary intact
- Ensure chapters serve the volume's overall purpose
- Maintain the volume's thematic focus

## Chapter Requirements

### Required Fields (All must be present)
1. **title**: Chapter title (string)
2. **summary**: One-sentence summary (string)
3. **characters**: List of characters (array of strings)
4. **location**: Primary location (string)
5. **events**: State change events (array of objects)
6. **beats**: 3-5 plot beats (array of strings)
7. **opening_beat**: Must equal beats[0] (string)
8. **closing_beat**: Must equal beats[last] (string)
9. **conflict**: Core conflict (string)
10. **pacing**: slow/normal/fast (string)

### Event Object Format
```json
{
  "type": "premise|relationship|goal|item|storyline|gate|status|memory",
  "subject": "who/what is affected",
  "change": "what changed"
}
```

## Output Format Example

```json
{
  "title": "Volume 1: The Awakening",
  "summary": "Protagonist discovers their power and faces first challenges",
  "chapters": [
    {
      "title": "Chapter 1: Discovery",
      "summary": "Lin Yan finds the ancient artifact in the ruins",
      "characters": ["Lin Yan"],
      "location": "Ancient Ruins",
      "events": [
        {"type": "item", "subject": "Lin Yan", "change": "Finds ancient artifact"}
      ],
      "beats": [
        "Then, Lin Yan enters the ruins searching for answers",
        "Therefore, he discovers a hidden chamber",
        "Then, the artifact glows as he approaches",
        "Therefore, Lin Yan realizes he has a special connection to it"
      ],
      "opening_beat": "Then, Lin Yan enters the ruins searching for answers",
      "closing_beat": "Therefore, Lin Yan realizes he has a special connection to it",
      "conflict": "Man vs unknown power",
      "pacing": "fast"
    }
  ]
}
```

## Guidelines

1. **Targeted Changes**: Only change what the review suggests
2. **Preserve Structure**: Keep the same number of chapters unless specifically asked to change
3. **Beat Continuity**: Ensure each chapter's closing beat leads to the next chapter's opening
4. **Volume Arc**: All chapters should build toward the volume's summary
5. **Context Awareness**: Consider the full outline context when making changes
