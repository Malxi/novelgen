# Recap Extract Skill

## Purpose
Extract a canonical recap JSON from chapter text for continuity tracking.

## Input
- `chapter_id`: The unique identifier for the chapter
- `title`: The chapter title
- `chapter_text`: The full text content of the chapter
- `feedback`: Optional feedback for fixing missing fields (used in retry passes)

## Output
- `recap`: A ChapterRecap object containing:
  - `chapter_id`: Chapter identifier
  - `title`: Chapter title
  - `location`: Where the chapter takes place
  - `time`: When it takes place (e.g., "same night", "next morning", "immediately after")
  - `present`: List of characters present in the chapter
  - `plot_beats`: What actually happened (not what was planned)
  - `decisions`: Key decisions made by characters
  - `reveals`: Important revelations or discoveries
  - `unresolved`: Unresolved plot points or cliffhangers
  - `promises`: Promises made that need to be fulfilled later
  - `items`: Items acquired, lost, or with changed status
  - `status`: Character status changes (injuries, mood, power level, etc.)
  - `last_line`: The final line of the chapter (for continuity)
  - `cliffhanger`: Any cliffhanger at the end
  - `next_opening_hint`: Optional suggestion for how the next chapter should open

## Guidelines

1. **Extract facts, not interpretations** - Record what actually happened, not analysis
2. **Be specific** - Include concrete details that matter for continuity
3. **Focus on continuity-critical information**:
   - Character locations and presence
   - Item locations and status
   - Unresolved plot threads
   - Character physical/emotional states
   - Time and place anchors

4. **Last line matters** - The final line is crucial for seamless chapter transitions
5. **Track promises** - Any "I will..." or "We need to..." statements become promises
6. **Note unresolved threads** - Anything left hanging should be recorded

## Output Format

Return a valid JSON object matching the ChapterRecap structure. All fields should be filled when possible. Empty arrays `[]` or empty strings `""` are acceptable for optional fields with no relevant data.

## Validation Rules

The recap must pass minimal validation:
- `chapter_id` must not be empty
- `location` must not be empty
- `present` must have at least one character
- `plot_beats` must have at least one entry
- `last_line` must not be empty

If feedback is provided, address all issues mentioned in the feedback.
