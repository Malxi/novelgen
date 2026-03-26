# Compose Chapters Skill

## Purpose
Generate detailed chapters for a specific volume, maintaining continuity with previous volumes and building toward the volume's goals.

## Input
- `setup`: StorySetup with premise, characters, storylines, etc.
- `part`: Current Part information (title, summary)
- `volume`: Current Volume to generate chapters for (title, summary)
- `volume_index`: Index of this volume (1-based)
- `total_volumes`: Total number of volumes in the outline
- `chapters_per_volume`: Number of chapters to generate
- `previous_volume`: Previous volume (if any) for continuity
- `outline_context`: Context from previous volumes

## Output
- `chapters`: Array of Chapter objects with complete details

## Chapter Requirements

### Required Fields
1. **title**: Chapter title (string)
2. **summary**: One-sentence summary (Character + Location + Event) (string)
3. **characters**: List of characters appearing (array of strings)
4. **location**: Primary location (string)
5. **events**: State change events (array of event objects)
6. **beats**: 3-5 plot beats (array of strings, NOT comma-separated)
7. **opening_beat**: Must equal beats[0] (string)
8. **closing_beat**: Must equal beats[last] (string)
9. **conflict**: Core conflict description (string)
10. **pacing**: slow/normal/fast (string)

### CRITICAL: Beats Format
The `beats` field MUST be an array of strings:
```json
"beats": [
  "Then, Lin Yan wakes up in the collapsed mine with no memory",
  "Therefore, he searches for survivors and finds Old Miner Wang trapped",
  "Then, a second collapse kills them both",
  "Therefore, Lin Yan resurrects and realizes he has a unique ability"
]
```

NOT a comma-separated string like:
```json
"beats": "beat1, beat2, beat3"  // WRONG!
```

### Event Types
Each event must have these fields:
- `type`: Event type (string) - one of: relationship, goal, item, premise, storyline, gate, status, memory
- `subject`: Who/what is affected (string)
- `change`: What changed (string)

Example:
```json
{
  "type": "premise",
  "subject": "Lin Yan",
  "change": "Discovers death-resurrection ability"
}
```

## Continuity Requirements

### Volume-Level Continuity
- Chapter 1 must follow from previous volume's ending (if any)
- Each chapter must lead logically to the next
- Last chapter must fulfill the volume's summary

### Beat Continuity
- Chapter N's closing beat must connect to Chapter N+1's opening
- Use "Therefore," or "Then," to show causality
- No time jumps or scene breaks between chapters

## Output Format Example

```json
{
  "chapters": [
    {
      "title": "The Awakening",
      "summary": "Lin Yan discovers his ability in the collapsed mine",
      "characters": ["Lin Yan", "Old Miner Wang"],
      "location": "Abandoned Mine",
      "events": [
        {"type": "premise", "subject": "Lin Yan", "change": "Discovers death-resurrection ability"},
        {"type": "status", "subject": "Lin Yan", "change": "Becomes aware of the test field"}
      ],
      "beats": [
        "Then, Lin Yan wakes up in the collapsed mine with no memory",
        "Therefore, he searches for survivors and finds Old Miner Wang trapped",
        "Then, a second collapse kills them both",
        "Therefore, Lin Yan resurrects and realizes he has a unique ability"
      ],
      "opening_beat": "Then, Lin Yan wakes up in the collapsed mine with no memory",
      "closing_beat": "Therefore, Lin Yan resurrects and realizes he has a unique ability",
      "conflict": "Survival in the collapsed mine while discovering his ability",
      "pacing": "fast"
    }
  ]
}
```

## Guidelines

1. **Volume Focus**: All chapters must serve the volume's summary
2. **Progressive Tension**: Tension should build throughout the volume
3. **Character Consistency**: Characters must act consistently with their established traits
4. **Event Tracking**: Use events to track all state changes
5. **Beat Quality**: Each beat must advance the plot or reveal character
6. **First Chapter Hook**: Chapter 1 must immediately engage the reader
7. **Last Chapter Cliffhanger**: Final chapter should set up the next volume
