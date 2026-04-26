# Skill: Chapters To RPG-DSL

Convert full chapter markdown text plus optional recap JSON into parser-compatible RPG-DSL.

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
- FOR COMBAT EVENTS (required):
  - `combat { enemies = [{id = "enemy_x", count = 1, level = 1}] }` — **REQUIRED** for every `type = "combat"`. Never use `enemies = []`.
- Optional event sub-blocks:
  - `move { to = "loc_x" }`
  - `spawn { actor = "char_x" location = "loc_x" }`
  - `on_complete { narration = "..." exp = 10 }`
  - `state_delta { target = "char_x" kind = "cultivation|lifespan|injury|resource|death|revive|time|transition|power_change|breakthrough|plot_thread" field = "..." from = "..." to = "..." delta = -1 unit = "..." cost = "..." note = "..." }`

Do not use `setup { ... }` inside `combat`.
Use `combat { enemies = [...] }` directly.
Use one or more `state_delta` blocks when the step changes important story state.

## Combat Event Rules (CRITICAL)

Every `type = "combat"` event MUST include `combat { enemies = [...] }` with at least one enemy extracted from the step description.

- Scan the description for enemy types and counts:
  - `"一只工蜂"` → `enemies = [{id = "enemy_worker_bee", count = 1}]`
  - `"三只工蜂虫"` → `enemies = [{id = "enemy_worker_bee", count = 3}]`
  - `"虫族哨兵"` → `enemies = [{id = "enemy_sentinel", count = 1}]`
  - `"兵蜂"` → `enemies = [{id = "enemy_soldier_bee", count = 1}]`
  - `"10台T-7型智网战斗机器人"` → `enemies = [{id = "enemy_t7_robot", count = 10}]`
- Use `count = 1` if the description does not specify a number.
- Enemy IDs must be ASCII snake_case.
- If the enemy type is genuinely unknown, use `id = "enemy_unknown"`.
- NEVER output `enemies = []` — this is a hard error.

Chapter text is the authoritative source for enemy details. Example from chapter content:

```
陆沉侧身猛地往旁边一滚，工蜂的前肢擦着他的肩膀狠狠扎进身后的合金墙面，三厘米厚的高强度合金居然像豆腐一样被扎出两个深达十厘米的洞。
```

→ `enemies = [{id = "enemy_worker_bee", count = 1}]` (the scene describes one worker bee against Lu Chen)

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

- Treat `content` as the authoritative source. Use `recap`/`plot_beats` only as navigation aids.
- Convert each input chapter to one `chapter` block.
- Convert important events from the full chapter text into ordered `step` blocks. `plot_beat` can suggest coarse order, but do not ignore details only present in `content`.
- Use provided chapter ID exactly.
- Keep names/descriptions in {{language}}.
- IDs must be ASCII-safe snake-case style:
  - characters: `char_*`
  - locations: `loc_*`
  - enemies: `enemy_*`

## State Delta Rules (CRITICAL — all are REQUIRED)

`state_delta` is the simulation's only source of truth. For every state-changing event in the chapter text, emit a `state_delta` inside the relevant `event { ... }`. Missing deltas = undetected bugs.

### injury (ALL characters, not just protagonist)

Whenever ANY named character is hurt, injured, or killed in chapter text:

- `kind = "injury"`
- `target = "character_id"` — use the character's DSL id. Create an NPC entry if the character doesn't exist yet.
- `field = "status"`
- `to = "injured"` or `to = "severe"` or `to = "dead"`.
- If a previously-injured character appears functioning normally without explanation, add: `kind = "injury"`, `to = "acting_normally"` with `note = "no_recovery_explained"`.

Example from chapter text:
```
老陈被落石砸中小腿，疼得蹲在地上哀嚎。
```
→ `state_delta { target = "char_laochen" kind = "injury" field = "status" to = "injured" note = "落石砸中小腿" }`

```
老陈扛着锄头走在队伍最前面。
```
→ If old injury not resolved: `state_delta { target = "char_laochen" kind = "injury" field = "status" to = "acting_normally" note = "no_recovery_explained" }`

### resource (numeric delta REQUIRED)

Whenever items, currency, or materials are gained, consumed, given away, or lost:

- `kind = "resource"`
- `target = "item_id"` (e.g. `"spirit_stone"`, `"gold_coin"`)
- `delta = N` — positive for gain, NEGATIVE for consume/lose/give. **This is mandatory.**
- `note = "从37块挖了3块变成40块"` — fragment of chapter text showing the arithmetic.

Example:
```
他从怀里摸出两块灵石递给老板。
```
→ `state_delta { target = "spirit_stone" kind = "resource" field = "item" delta = -2 note = "递给老板2块灵石" }`

### time (timeline markers for cross-chapter consistency)

Whenever the chapter text mentions a specific time offset, deadline, or date:

- `kind = "time"`
- `field = "timeline_marker"` 
- `to = "缉厄司_三个月后才到"` or `to = "矿难第3天"`
- `note = "exact quote"` — copy the exact sentence from chapter text.

This allows the simulator to detect contradictions like "三个月后" vs "三天前就来了".

### age (character age progression)

When a character's age is explicitly stated:

- `kind = "age_progression"`
- `target = "character_id"`
- `field = "age"`  
- `to = "11"` or `delta = 1` if age increased.
- `note = "第1章说小矿奴11岁"`

### plot_thread (CRITICAL for mystery tracking)
- explicit transition explanation:
  - `kind = "transition"`
  - `field = "explained"`
  - `to = "true"`.
- power / combat-power changes (for progression tracking across chapters):
  - `kind = "power_change"`
  - `target = "character_id"`
  - `delta = 50` (positive = increase, negative = decrease).
  - Use when the chapter shows clear strength gain or loss.
- breakthrough / evolution events (unlocks new tier, realm, or species ability):
  - `kind = "breakthrough"`
  - `target = "character_id"`
  - `field = "realm"` or `field = "stage"`
  - `from = "normal"` / `to = "stage_1"`.
  - `note = "基因进化突破，解锁皮下外骨骼"`
- unresolved plot threads, mysteries, or foreshadowing raised / resolved:
  - `kind = "plot_thread"`
  - `target = "mystery_or_thread_id"` (ASCII snake identifier).
  - `field = "status"`
  - `to = "raised"` when a mystery is introduced, `to = "resolved"` when it is answered.
  - `note = "休眠仓显示3011年，手环显示1023年，矛盾未解释"`
  - The simulator tracks raised vs resolved counts; unresolved threads produce a summary issue.

Use chapter text to capture details recap often drops:

- start/end continuity, especially injuries or status that should carry into the next chapter.
- exact resource gains/consumption and quantities when stated.
- rules explained in dialogue or narration.
- NPC identity features, aliases, age, and role clues.
- time skips and whether the prose provides transition explanation.

Prefer structured deltas over only natural-language narration. If the value is unclear, still add a delta with `note = "unclear: ..."` rather than inventing a number.

## Self-Check Before Output

- Is output valid JSON with `dsl_content`?
- Is `dsl_content` valid DSL without comments?
- Are all keys ASCII English?
- Are all `objective` lines in form: `objective "..." {`?
- Are `step` lines in form: `step N {` (without `=`)?
- Are combat enemies in form: `enemies = [{id = "...", count = 1, level = 1}]`?
- Are important deaths, revivals, cultivation changes, lifespan changes, injuries, resources, time jumps, power changes, breakthroughs, and plot threads represented with `state_delta`?
