# Outline workflow

Use this when the outline is missing, a volume is empty, or the user asks to
regenerate structure.

1. Read setup first:
   `novelgen tool query story-setup --view index`.
2. Read the current structure:
   `novelgen tool query outline --view index`.
3. For a complete outline, produce a JSON object with `parts -> volumes -> chapters`.
   Each chapter needs `id`, `title`, `summary`, `characters`, `location`,
   `events`, `beats`, `opening_beat`, `closing_beat`, `conflict`, and `pacing`.
4. Validate and commit the complete artifact:
   `printf '%s' '<complete outline json>' | novelgen tool replace outline --apply`.
5. Check it:
   `novelgen tool check all --target outline --min-priority medium --max-issues 12`.

For long-form projects, prefer the existing `novelgen compose pipeline` for a
large skeleton; use `tool replace outline` when an agent owns the complete
outline or is regenerating a bounded project. Never add chapters by writing
`story/compose/outline.json` directly.
