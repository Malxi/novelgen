# Skill: Chapters To RPG-DSL

Convert chapter recap JSON into parser-compatible RPG-DSL.

## Output Contract

- Return JSON only:
  - `{"dsl_content":"..."}`
- `dsl_content` must be plain DSL text only.
- No markdown fences.
- No comments (`#` and `//` forbidden).
- Do not include explanations.

## Critical Syntax Rules

- Use ASCII keys and keywords only.
- Use only `=` for assignments.
- Strings must be wrapped in `"` and escaped.
- Keep braces balanced.
- Never output Chinese keys such as `时间`, `修为`, `地点`, `敌人`.
- Never output unknown top-level blocks.

## Allowed Top-Level Blocks

- `metadata { ... }`
- `world { ... }`
- `characters { ... }`
- `storyline { ... }`

`systems` is optional and should usually be omitted in chapter conversion.

## Allowed Named Blocks

- `location "Name" { ... }`
- `player "Name" { ... }`
- `npc "Name" { ... }`
- `chapter "Title" { ... }`
- `objective "Goal" { ... }`
- `step N { ... }`
- `event { ... }`

## Allowed Event Fields

- `type = "status|combat|location|acquire|knowledge|relationship"`
- Optional:
  - `combat { enemies = [{id = "enemy_x", count = 1, level = 1}] }`
  - `move { to = "loc_x" }`
  - `spawn { actor = "char_x" location = "loc_x" }`
  - `on_complete { narration = "..." exp = 10 }`
  - `state_delta { target = "char_x" kind = "cultivation|lifespan|injury|resource|death|revive|time|transition" field = "..." from = "..." to = "..." delta = -1 unit = "..." cost = "..." note = "..." }`

Do not use `setup { ... }` inside `combat`.
Use `combat { enemies = [...] }` directly.
Use one or more `state_delta` blocks when the step changes important story state.

## Required Minimal Fields

### metadata
- `title`
- `dsl_version = "0.2.0"`
- `source = "novelgen_outline"`

### world/location
- `id`
- `name`
- `description`
- `__placeholder__ = false`

### characters/player
- `id`
- `name`
- `description`
- `__placeholder__ = false`
- `str`, `agi`, `int`, `vit`, `hp`, `mp`
- `class`

### characters/npc
- `id`
- `name`
- `description`
- `__placeholder__ = false`
- `role`
- `default_location`

### storyline/chapter
- `id`
- at least one `objective`
- each objective has `step 1..N`
- each step has `description` + `event`

## Mapping Rules

- Convert each input chapter to one `chapter` block.
- Convert each `plot_beat` to one `step` in order.
- Use provided chapter ID exactly.
- Keep names/descriptions in {{language}}.
- IDs must be ASCII-safe snake-case style:
  - characters: `char_*`
  - locations: `loc_*`
  - enemies: `enemy_*`

## State Delta Rules

Add `state_delta` inside the relevant `event { ... }` whenever the chapter states or implies:

- protagonist dies or revives:
  - `kind = "death"` or `kind = "revive"`
  - `target` should be the character id, usually the player id.
- cultivation changes:
  - `kind = "cultivation"`
  - `field = "level"`
  - `from = "qi_2"` / `to = "qi_1"` or other stable ASCII values.
  - `delta` should be numeric if clear, for example `delta = -1`.
- lifespan changes:
  - `kind = "lifespan"`
  - `field = "years"`
  - `delta = -10` for losing ten years.
  - If text says no lifespan loss, use `delta = 0` and `cost = "none"`.
- injury changes:
  - `kind = "injury"`
  - `field = "status"`
  - `to = "injured|severe|recovered|acting_normally|acting_injured"`.
- resource changes:
  - `kind = "resource"`
  - `field = "item"`
  - `target = "item_or_resource_id"`
  - `delta` should be positive for gain and negative for consume if clear.
- time jumps:
  - `kind = "time"`
  - `field = "elapsed_days"`
  - `delta = N`.
- explicit transition explanation:
  - `kind = "transition"`
  - `field = "explained"`
  - `to = "true"`.

Prefer structured deltas over only natural-language narration. If the value is unclear, still add a delta with `note = "unclear: ..."` rather than inventing a number.

## Self-Check Before Output

- Is output valid JSON with `dsl_content`?
- Is `dsl_content` valid DSL without comments?
- Are all keys ASCII English?
- Are all `objective` lines in form: `objective "..." {`?
- Are `step` lines in form: `step N {` (without `=`)?
- Are combat enemies in form: `enemies = [{id = "...", count = 1, level = 1}]`?
- Are important deaths, revivals, cultivation changes, lifespan changes, injuries, resources, and time jumps represented with `state_delta`?
