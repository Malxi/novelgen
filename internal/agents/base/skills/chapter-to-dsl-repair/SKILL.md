# Skill: Repair Invalid RPG-DSL

Repair an invalid RPG-DSL text into parser-compatible DSL.

## Goal

Given:
- parser error message
- invalid DSL content

Return **only** corrected DSL content inside JSON field `dsl_content`.

## Hard Rules

- Do not output markdown fences.
- Do not output explanations.
- Do not output comments.
- Keep existing story meaning; only fix syntax/structure.
- Use only ASCII DSL keywords and field keys.
- Text values can be Chinese, but keys must be English identifiers.

## Allowed Top-level Blocks

- `metadata { ... }`
- `world { ... }`
- `characters { ... }`
- `storyline { ... }`
- optional `systems { ... }`

## Allowed DSL Keywords (must be English)

- structural: `metadata`, `world`, `characters`, `storyline`, `systems`
- entities: `location`, `player`, `npc`, `chapter`, `objective`, `step`, `event`
- event nested: `combat`, `move`, `spawn`, `on_complete`, `state_delta`
- assignments use `=` only

## Frequent Fixes

- Replace non-English keys with English keys.
- Remove unknown nested objects not supported by parser.
- Ensure every string is quoted with `"` and escaped correctly.
- Ensure braces are balanced.
- Ensure blocks use `key "name" { ... }` where required:
  - `location "..." { ... }`
  - `player "..." { ... }`
  - `npc "..." { ... }`
  - `chapter "..." { ... }`
  - `objective "..." { ... }`
  - `step N { ... }`
- For `event` block:
  - allow `type = "status|combat|location|acquire|knowledge|relationship"`
  - optional `combat { enemies = [...] }`
  - optional `move { to = "loc_xxx" }`
  - optional `spawn { actor = "char_xxx" location = "loc_xxx" }`
  - optional `on_complete { narration = "..." exp = 10 }`
  - optional `state_delta { target = "char_xxx" kind = "cultivation|lifespan|injury|resource|death|revive|time|transition" field = "..." from = "..." to = "..." delta = -1 unit = "..." cost = "..." note = "..." }`

## Output

Return JSON only:

```json
{
  "dsl_content": "<valid DSL text>"
}
```
