# Compose Improve Volume Skill

## Purpose
Improve the chapters in a specific volume based on review feedback, while maintaining continuity with the rest of the outline.

## Input
- `setup_brief`: Compact story contract with premise, core rules, progression limits, resources, and storylines
- `outline_context`: Compact continuity context around the target volume. It may include part/volume summaries, previous/next volume summaries, and a chapter map.
- `target_volume`: The Volume to improve (with current chapters)
- `review_result`: Review feedback specific to this volume
- `user_prompt` (optional): Additional user suggestions for improvement

## Output
- `volume`: The improved Volume with updated chapters

**⚠️ CRITICAL: Return ONLY the Volume object, NOT the Part object**
- DO NOT return a `parts` array
- DO NOT return a `volumes` array  
- Return ONLY the single Volume with its `chapters` array
- The output must be a Volume, not a Part or Outline

## Improvement Focus

### 1. Address Review Suggestions
- Fix specific issues mentioned in review feedback
- Improve pacing where noted
- Strengthen character arcs as suggested
- Fix plot coherence issues
- Prefer small, targeted repairs over rewriting the volume from scratch

### 2. Maintain Continuity
- First chapter must connect to previous volume's ending
- Last chapter must set up for next volume (if any)
- Chapter-to-chapter beats must flow logically
- Use `outline_context` for continuity. Do not invent unrelated new arcs just because the complete outline is not present.

### 3. Preserve Volume Identity
- Keep the volume's title and summary intact
- Ensure chapters serve the volume's overall purpose
- Maintain the volume's thematic focus
- Keep the same chapter count and chapter order unless the review explicitly asks for a structural change

### 4. Strengthen Payoff Design
- Preserve or add `payoff_contract` for the target volume. It should define the volume question, power promise, opponent misread, big win, visible reward, reputation shift, and next bigger game.
- Preserve or add `chapter_payoff` for chapters that lack a satisfying win pattern. Make it concrete enough for write agents to dramatize.
- Do not solve weak chapters by adding only harsher pressure. Add how the protagonist uses the book's unique setting to win beautifully.
- If `setup_brief` includes Long Form Plan, align this volume with its main loop, payoff cadence, reader promises, and volume pattern. Keep the volume specific, but do not drift away from the serial engine.

## Chapter Requirements

### Required Fields (All must be present)
1. **title**: Chapter title (string)
2. **summary**: One-sentence summary (string)
3. **characters**: List of characters (array of strings)
4. **location**: Primary location (string)
5. **events**: State change events (array of objects)
6. **scenes**: Scene breakdown (array, 2-3 entries). Each scene contains its own `beats` array.
7. **conflict**: Core conflict (string)
8. **pacing**: slow/normal/fast (string)
9. **state_change**: One concise sentence describing the concrete state change caused by this chapter

### Optional but Strongly Recommended Fields
- `state_anchor`: protagonist state at chapter start when relevant (cultivation/ability, allies, injuries, location, key_items)
- `resource_ledger`: only when a scarce resource is gained, spent, lost, or consumed
- `storyline_advances`: only when the chapter meaningfully advances a setup storyline through pressure, reveal, reversal, payoff, choice, or consequence
- `chapter_payoff`: the chapter's爽点 contract with `desire`, `pressure`, `clever_move`, `payoff_moment`, `reward`, `social_proof`, and `hook`

### Scene Requirements

Beats live inside scenes. Do NOT output chapter-level `beats`, `opening_beat`, or `closing_beat`; those are derived from `scenes[].beats`.

Each scene should include:
- `order`: Scene order starting from 1
- `pov`: POV character
- `goal`: What this scene advances
- `location`: Scene location
- `characters`: Characters present
- `beats`: 1-2 concrete plot beats
- `words` (optional): Suggested word count
- `tone` (optional): Emotional tone

**Schema guardrail:** `beats` must always be a JSON array of strings, even when there is only one beat.

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
      "scenes": [
        {
          "order": 1,
          "pov": "Lin Yan",
          "goal": "Search the ruins for answers",
          "location": "Ancient Ruins",
          "characters": ["Lin Yan"],
          "beats": [
            "Lin Yan enters the ruins searching for answers",
            "He discovers a hidden chamber"
          ],
          "words": 1200,
          "tone": "tense"
        },
        {
          "order": 2,
          "pov": "Lin Yan",
          "goal": "Understand the artifact's connection to him",
          "location": "Hidden Chamber",
          "characters": ["Lin Yan"],
          "beats": [
            "The artifact glows as he approaches",
            "Lin Yan realizes he has a special connection to it"
          ],
          "words": 1300,
          "tone": "mysterious"
        }
      ],
      "conflict": "Man vs unknown power",
      "pacing": "fast"
    }
  ]
}
```

## Guidelines

1. **Targeted Changes**: Only change what the review suggests
2. **Preserve Structure**: Keep the same number of chapters unless specifically asked to change
3. **Beat Continuity**: Ensure the last scene beat of each chapter leads to the first scene beat of the next chapter
4. **Volume Arc**: All chapters should build toward the volume's summary
5. **Context Awareness**: Use the compact setup brief and outline context as boundaries for changes
6. **User Suggestions**: If `user_prompt` is provided, prioritize the user's specific requests alongside review feedback
7. **Output Discipline**: Return pure JSON only. Do not wrap the response in markdown fences and do not add explanations.
8. **Payoff Discipline**: Keep every chapter's win pattern visible and grounded in setup rules, not generic coincidence.
