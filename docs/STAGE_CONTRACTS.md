# Novelgen Stage Contracts

Last updated: 2026-05-07

This document defines the persistent data contracts between Novelgen workflow
stages. Treat these contracts as part of the public behavior of the repository:
when a stage input, output, invariant, file path, or consumer changes, update this
file in the same change.

## Contract Rules

- A stage output is project state. It must be parsed into typed Go data,
  normalized when needed, validated deterministically, and only then written to
  disk.
- A prompt may request a contract, but Go code owns the contract. Required
  invariants must be enforced by code, tests, or both.
- Every contract field needs clear producer and consumer ownership. If a field is
  optional, consumers must define the default behavior when it is missing.
- Project files must remain readable by older projects unless the change includes
  an explicit migration or compatibility fallback.
- Chapter IDs are stable identifiers. Do not renumber or rename chapter files
  after downstream files exist unless the change migrates all derived state.

## Persistent Layout

Project root:

- `novel.json`: `models.ProjectConfig`
- `story/setup/story_setup.json`: `models.StorySetup`
- `story/compose/outline.json`: `models.Outline`
- `story/craft/characters.json`: `map[string]models.Character`
- `story/craft/locations.json`: `map[string]models.Location`
- `story/craft/items.json`: `map[string]models.Item`
- `story/craft/organizations.json`: `map[string]models.Organization`
- `chapters/chapter-<chapter_id>.md`: final chapter text
- `drafts/<chapter_id>.md`: legacy draft text
- `story/recaps/<chapter_id>.json`: `models.ChapterRecap`
- `story/reviews/*.json`: review outputs
- `story/rpg/*.rpg`: RPG-DSL fragments

Global user config:

- `~/.novelgen/agent_config.json`: default agent runtime configuration for
  Claude/Python SDK execution. A runtime may set `provider` to reference a
  provider from `~/.novelgen/llm_config.json`; base URL, API key, and timeout
  are then resolved at runtime (explicit runtime fields win). When the
  effective runtime carries Anthropic-compatible credentials and no explicit
  `settings`, novelgen auto-generates a Claude flag-settings file under
  `~/.novelgen/agents/settings/<runtime>.json` containing
  `ANTHROPIC_BASE_URL`, `ANTHROPIC_API_KEY`, model defaults, and runtime
  `env` overrides. `ANTHROPIC_AUTH_TOKEN` is never generated because Claude
  Code prefers it (Bearer auth) over `ANTHROPIC_API_KEY` (x-api-key).
- `~/.novelgen/agents/`: user-level agent home for runtime skills, KB, tools,
  and runner-local cache. Project state is still written only by Go after typed
  parsing and deterministic validation.
- `~/.novelgen/llm_config.json`: OpenAI-compatible provider config shared by
  the legacy pipeline and agent runtime provider references. Optional
  `agent_base_url` pins the Anthropic-compatible base URL used by the agent
  runtime; otherwise a trailing `/v1` is stripped from `base_url`.

## Stage Matrix

| Stage | Main producer | Main output | Main consumers |
| --- | --- | --- | --- |
| `init.v1` | `novelgen init` | `novel.json`, directory layout | all project commands |
| `setup.v1` | `novelgen setup` | `story/setup/story_setup.json` | compose, craft, write, RPG |
| `compose.v1` | `novelgen compose` | `story/compose/outline.json` | craft, write continuity, RPG DSL |
| `craft.v1` | `novelgen craft` | `story/craft/*.json` | write continuity, RPG DSL |
| `write.v1` | `novelgen write` | `chapters/chapter-<chapter_id>.md` | recap, review, RPG DSL |
| `recap.v1` | `novelgen recap`, `write pipeline` | `story/recaps/<chapter_id>.json` | continuity, write |
| `rpg-dsl.v1` | `novelgen rpg-dsl`, `write pipeline` | `story/rpg/*.rpg` | parser, validator, simulator, write continuity |
| `draft.v1` | `novelgen draft` | `drafts/<chapter_id>.md` | legacy review/improve only |

## `init.v1`

Producer:

- `cmd/init.go`
- Command: `novelgen init [book_name]`

Inputs:

- CLI args and flags: book name, chapter count, genre, provider, model, language

Outputs:

- `novel.json`
- `story/setup/`
- `story/compose/`
- `story/craft/`
- `story/reviews/`
- `chapters/`
- `drafts/`
- `logs/`

Go types:

- `models.ProjectConfig`
- `models.StoryStructure`
- `models.ChapterConfig`
- `models.ProjectLLM`

Required invariants:

- `ProjectConfig.Validate()` passes.
- `Structure.TargetParts`, `TargetVolumes`, and `TargetChapters` are positive.
- `ChapterConfig.MinWordsPerChapter <= TargetWordsPerChapter <= MaxWordsPerChapter`.
- `Language`, `LLM.Provider`, and `LLM.Model` are non-empty.
- New persistent directories added by later stages must be created here or lazily
  created by the stage writer.

Consumers:

- Every project command.

Change checklist:

- Update `createProjectStructure` when adding a new persistent directory.
- Update README and `AGENTS.md` when changing default workflow or defaults.
- Add config validation tests for new required fields.

## `setup.v1`

Producer:

- `cmd/setup.go`
- `internal/agents/setup_agent.go`
- Skills: `setup-gen`, `setup-improve`, `setup-review`

Inputs:

- `novel.json`
- User idea, imported markdown, or existing `story_setup.json`

Outputs:

- `story/setup/story_setup.json`
- Optional human-editable setup markdown

Go type:

- `models.StorySetup`

Required invariants:

- `ProjectName`, `Genres`, `Premise`, `Theme`, `Rules`, `TargetAudience`,
  `Tone`, `Tense`, and `POVStyle` are present for generated setup.
- `Storylines[].Name` is stable enough for outline `storyline_advances` to
  reference.
- `Storylines[].Importance` is in the documented range.
- `CoreCast[]` is optional setup-level character seed data. It should stay
  lightweight: role, importance, story function, relationship arc, entry phase,
  payoff, and storyline references. Setup owns the promise; craft owns the full
  character card.
- Setup is a compact contract, not an encyclopedia. Long biographies, minor
  cast lists, exhaustive rulebooks, and chapter-scale lore should move to craft,
  notes, or later stage-specific context. Quality gates may flag oversized
  setup sections or fields that are too long for stable prompting.
- `LongFormPlan` is optional setup-level serial capacity data. It owns target
  scale, repeatable main loop, escalation ladder, reader promises, payoff
  cadence, reusable volume pattern, midpoint mutation, and endgame promise.
  It must not become a chapter-by-chapter outline.
- Long-form setups that imply a large serial, web novel, power fantasy, group
  cast, or multi-system genre story should use `CoreCast[]` as capacity seeds:
  at least one protagonist anchor, several important (`Importance >= 8`) roles,
  distinct story functions, relationship arcs, staggered `entry_phase` values,
  payoff promises, and `storyline_refs` that match `Storylines[].Name`.
- Long-form setups should include `LongFormPlan` when target scale is explicit
  or implied. Quality gates may suggest it when a setup says 300+, 500+, 1000
  chapters, serial/web novel, large series, or similar signals.
- Important `Storylines[]` (`Importance >= 8`) should carry enough high-level
  arc contract data for downstream planning: `scope`, `setup_role`,
  `payoff_style`, and pressure hints are preferred, but older projects may omit
  them and receive quality-gate suggestions rather than parse failures.
- Long-form important `Storylines[]` may also carry serial-engine hints:
  `repeatable_pressure`, `payoff_cadence`, `mutation`, and `failure_mode`.
  These stay at setup level and help compose avoid one-use arcs, endless
  delayed payoffs, and repetitive filler.
- Important `Storylines[]` and core `Premises[]` may include optional
  `appeal_engine` data. The setup stage produces it when it helps define the
  book's power-fantasy promise: `appeal`, `surface_limit`, `exploit`,
  `signature_win`, `upgrade_path`, `opponent_misread`, and `reward_type`.
  Missing `appeal_engine` is valid for older projects; compose and write fall
  back to premise/rule/storyline text and review stages may suggest adding it.
- `models.NormalizeStorySetup` and `StorySetup.Save` trim whitespace, remove
  blank list items, drop empty appeal-engine blocks, and order progression
  stages by level when possible before persistence.
- Long-form genre setups should keep one root `Premise` string while splitting
  derived world logic into multiple `Premises[]` systems when useful. These
  systems give RPG/DSL conversion simulatable growth, enemy, faction, resource,
  social, or external-threat tracks without creating conflicting root premises.
- `Premises[].Progression` is ordered by level when levels are present.
- `WorldResources[].Name` is stable enough for outline resource ledgers to
  reference.
- `WritingStyle` is optional. It is produced by `setup gen`, `setup improve`,
  or `setup import` only when a user requests a prose style or provides a
  reference passage. Older projects may omit it and consumers must fall back to
  `Tone`, `Tense`, and `POVStyle`.
- `WritingStyle.ReferenceExcerpt` is a style signal only. It is not story canon:
  downstream stages must not import its plot, characters, places, terminology,
  or world facts into generated project state.
- `setup improve` may pass a prompt-only `revision_context` between review and
  improve rounds. It is not persisted in `story_setup.json`; it only helps the
  agent remember what the current session already tried.
- `setup improve --agent-sdk` and `setup regen --agent-sdk` use the Claude
  Agent SDK workflow skill `setup-improve-workflow`. The agent receives compact
  review/check findings plus optional user guidance, queries project facts with
  `novelgen tool query`, validates candidate changes with
  `novelgen tool patch setup`, and returns a minimal top-level `setup_patch`.
  `setup regen --agent-sdk` is a focused regeneration/repair mode for an
  existing setup; it does not send the full setup JSON as prompt context and it
  does not allow the agent to replace project files directly. In the default
  mode Go remains the writer: it merges, validates suspicious text, normalizes,
  runs setup quality + simulation checks, checkpoints, exports markdown, and
  saves. With `--agent-apply`, the agent may write only through
  `novelgen tool patch setup ... --apply` after a successful dry-run, and Go
  reloads the saved setup.

Consumers:

- `compose` uses setup as the source of story intent and structure.
- `compose` sends a prompt-only `setup_brief` rather than the full
  `StorySetup` where possible. The persisted setup remains complete, but
  outline generation/review/improve/chapter generation consume the compact
  contract so large setup files do not crowd out the active outline task.
- `compose` consumes optional `appeal_engine` hints to create volume
  `payoff_contract` and chapter `chapter_payoff` entries.
- `compose` consumes optional `LongFormPlan` to align volume count, chapter
  scale, volume payoff contracts, escalation, and serial cadence.
- `compose` consumes optional storyline serial-engine hints to place recurring
  pressure, partial payoffs, mid-story mutations, and avoid known failure modes.
- `craft` uses setup to generate world elements.
- `craft` consumes optional `CoreCast[]` seeds to anchor character expansion
  and uses `story_setup.core_cast` names as a character extraction source even
  when the outline does not mention them yet.
- `write` uses setup for style, rules, tone, and continuity.
- `write` consumes optional `WritingStyle` for prose voice, rhythm, description
  density, dialogue texture, and style avoid-list guidance.
- RPG/DSL generators use setup for systems, resources, and world rules.
- `compose improve` may pass a prompt-only `revision_context` across review and
  improve rounds. It is not persisted in `outline.json`; it just keeps the
  current session from forgetting which fixes were already tried.

Change checklist:

- Update setup skills when adding/removing fields.
- Update compose/craft/write inputs if they should consume the field.
- Add defaults or compatibility handling for old `story_setup.json` files.

## `compose.v1`

Producer:

- `cmd/compose.go`
- `internal/agents/compose_agent.go`
- Skills: `compose-gen`, `compose-skeleton`, `compose-skeleton-review`,
  `compose-skeleton-improve`, `compose-chapters`, `compose-regen`,
  `compose-review`, `compose-improve`, `compose-improve-volume`

Inputs:

- `novel.json`
- `story/setup/story_setup.json`

Outputs:

- `story/compose/outline.json`
  - During hierarchical generation this file is the canonical incremental
    state: skeleton generation writes all parts/volumes with empty
    `Volume.Chapters`, and each completed volume updates the same file.
  - `compose skeleton-review` reviews the current parts/volumes skeleton
    without requiring chapters and writes `story/compose/skeleton_review.json`.
  - `compose skeleton-improve` runs skeleton review/improve loops, preserves
    part IDs, volume IDs, and existing chapter arrays, then writes the improved
    skeleton back to `outline.json` and `outline.md`. Its final review is saved
    as `story/compose/skeleton_improve_review.json`.
  - `compose pipeline` uses the same file as its only compose checkpoint:
    it creates or loads the skeleton, optionally generates one global volume
    range, improves each generated volume in isolation, then merges it back
    into `outline.json`.
  - `compose improve --agent-sdk` volume patches may update
    `Volume.Chapters[].Scenes[].Beats` through `changed_chapters[].scenes`
    (query with `--fields scenes`; the brief view intentionally strips scenes,
    and Go merges patch scenes into the original chapter by scene order, so a
    partial scenes list never truncates untouched scenes).
  - Older interrupted projects may still have
    `story/compose/outline_progress.json`; `compose gen` treats it as a legacy
    resume source and migrates it into `outline.json`.

Go types:

- `models.Outline`
- `models.Part`
- `models.Volume`
- `models.Chapter`
- `models.Event`

Required invariants:

- Part and volume counts match `ProjectConfig.Structure`.
- Complete outlines have each volume's chapter count matching
  `ProjectConfig.Structure.TargetChapters`.
- In-progress hierarchical outlines may have empty `Volume.Chapters` for
  not-yet-generated volumes. Downstream stages that require chapters must skip
  or reject those empty volumes; `compose improve` improves only generated
  volumes and merges them back without filling empty volumes. When
  `--volume`, `--from-volume`, or `--to-volume` is supplied, `compose improve`
  narrows that generated-volume view to the selected 1-based global volume
  index range before improving.
- `compose skeleton-review` and `compose skeleton-improve` must not treat empty
  `Volume.Chapters` as an error. Their contract is limited to part/volume
  titles, summaries, payoff contracts, and long-form escalation.
- `compose pipeline --from-volume A --to-volume B` addresses volumes by the
  same 1-based global volume index. It must not generate or improve volumes
  outside the selected range, and it preserves empty future volumes unless
  `--force` is explicitly used on an already generated selected volume.
- After each pipeline volume, the setup/outline cross check may deterministically
  add missing `WorldResources[]` entries referenced by generated resource
  ledgers. These placeholders are setup state, not outline state, and should be
  refined by a later setup-improve pass.
- IDs are stable and unique. Preferred chapter format is `P<n>-V<n>-C<n>`.
- Each chapter has `ID`, `Title`, `Summary`, `Characters`, `Location`,
  `Conflict`, `Pacing`, and meaningful `Events`.
- Volumes may include optional `PayoffContract` (`payoff_contract`) describing
  the reader promise for that volume: `volume_question`, `power_promise`,
  `main_opponent_misread`, `big_win`, `visible_reward`, `reputation_shift`, and
  `next_bigger_game`. It is produced by compose skeleton/full generation and
  consumed by compose review/improve, volume review, and write context through
  the current volume/chapter data. Missing values are valid for old outlines and
  should degrade to ordinary summary-based planning.
- Chapters may include optional `ChapterPayoff` (`chapter_payoff`) describing
  the chapter-level satisfying win pattern: `desire`, `pressure`,
  `clever_move`, `payoff_moment`, `reward`, `social_proof`, and `hook`. It is
  produced by compose chapter/full generation and consumed by write generation,
  write review/improve, volume review, and markdown exports. Missing values are
  valid for old outlines; write falls back to scenes, beats, conflict, events,
  and storyline advances.
- `models.NormalizeOutline` and `Outline.Save` trim payoff fields and drop
  empty payoff containers before persistence.
- `compose improve --agent-sdk` and `compose pipeline --agent-sdk` keep Go as
  the orchestrator. In normal SDK mode the agent returns a compact
  `volume_patch`, and Go merges, normalizes, validates, and saves. With
  `--agent-apply`, the agent may write only through validated
  `novelgen tool patch outline --target volume ... --apply`; after the tool
  writes, Go reloads the target volume from `story/compose/outline.json` and
  continues validation/checkpoint/export. Apply-mode final agent JSON is only
  `review_result`, `applied_patches`, `applied_patch_count`, and `final_check`
  so the prompt does not include the full outline patch schema.
- Outline patch merge is recursive for nested JSON objects and replacement-based
  for arrays. An agent can patch only `chapter_payoff.desire` and
  `chapter_payoff.hook` without resending the full payoff object, while list
  fields such as `events`, `storyline_advances`, and `resource_ledger` must be
  supplied as complete replacement lists when changed.
- When a user prompt or focused repair task is present, the Agent SDK workflow
  may continue from a clean medium+ volume check into target-volume chapter and
  event detail queries before deciding whether to patch. Clean-stop routing is
  reserved for no-task probe runs where no focused detail queries are granted.
- `Events` are consumable through `Event.GetActor`, `GetAction`, `GetTarget`,
  and `GetTargetType`.
- Code that consumes events should use accessor methods rather than direct field
  reads unless it is deliberately handling old-format compatibility.
- `ResourceLedgerEntry.Start + Delta == End` when a ledger entry is present.
- `OutlineScene.Order` starts at 1 and is stable within a chapter.
- Older outlines may still contain chapter-level `beats`, `opening_beat`, and
  `closing_beat`. These are compatibility fields; `compose normalize` preserves
  them and deterministically migrates them into `scenes` for downstream stages.
- Mystery IDs and boss IDs are stable if referenced across chapters.
- `StorylineAdvance.StorylineName` should match or clearly refer to a setup
  storyline.

Consumers:

- `craft` extracts character, location, and item targets from the outline.
- `write` uses chapters, scenes, events, state anchors, enemies, resources, and
  storyline advances.
- `logic.ChapterContinuityBuilder` folds outline events into write continuity.
- RPG and RPG-DSL converters derive story structure and state deltas.

Change checklist:

- Update compose skills and output validation together.
- Update `internal/models/outline_normalizer.go` or add normalization when
  introducing fields that can drift.
- Update write continuity builders, write context builders, and RPG converters for any field
  that affects continuity or simulation.
- Add tests for structure validation, event compatibility, or normalization.

## `craft.v1`

Producer:

- `cmd/craft.go`
- `internal/agents/craft_agent.go`
- `internal/agents/craft_iteration_agent.go`
- Skills: `craft-characters`, `craft-locations`, `craft-items`, and matching
  review/improve skills

Inputs:

- `story/setup/story_setup.json`
- `story/compose/outline.json`
- Optional user prompt

Outputs:

- `story/craft/characters.json`
- `story/craft/locations.json`
- `story/craft/items.json`
- `story/craft/organizations.json`

Go types:

- `map[string]models.Character`
- `map[string]models.Location`
- `map[string]models.Item`
- `map[string]models.Organization`

Required invariants:

- Map keys should be canonical names and should match each element's `Name`
  field unless a compatibility reason is documented.
- Names referenced by outline chapters should be present or explicitly handled as
  unknown/minor elements.
- `Character.Notes` and `Item.Notes` are strings, not arrays.
- Alias fields are additive and must not replace canonical names.
- Location and item references should use names stable enough for write and RPG
  conversion.
- Ability systems and progression stages are owned by `setup.v1`
  (`StorySetup.Premises[]`) and must not be duplicated as craft project state.
  Craft may use them to ground characters, locations, and items.
- Craft elements may include optional RPG/DSL metadata. Producers are
  `craft gen` and `craft improve`; consumers are RPG adapters, DSL conversion,
  and simulation. Missing fields are valid for old projects and fall back to
  deterministic inference.
- Character RPG metadata includes `rpg_role`, `combat_role`, `power_level`,
  `rpg_stats`, `dsl_tags`, and `state_effects`. These are static defaults only;
  relationship progress, goals, injuries, resources, and chapter-by-chapter power
  changes remain owned by outline/recap/state stages.
- Location RPG metadata includes `rpg_map_type`, `danger_level`,
  `encounter_tags`, `resource_tags`, `dsl_tags`, and `state_effects`.
- Item RPG metadata includes `rpg_item_type`, `rarity`, `power_level`,
  `quantity_tracking`, `dsl_tags`, and `state_effects`.
- Item `owner`, when present, should be a known craft character name/key or a
  stable generic owner such as `主角`; `tool check schema --target craft --scope
  item` reports unknown owners as medium consistency feedback.
- Organization profiles describe durable factions, guilds, sects, companies,
  empires, clans, armies, agencies, alliances, or other power groups. They are
  produced by `craft gen` from explicit outline factions and faction-like setup
  storylines, then refined by `craft improve`. Missing
  `organizations.json` remains valid for older projects.
- Craft extraction scans chapter characters, scenes, enemies, `state_anchor`,
  typed event actors/targets via `Event` accessors, and `resource_ledger` so
  RPG-relevant elements are not missed.
- Outline events that reference premise/ability targets should resolve to
  `StorySetup.Premises[]` names or progression stage names. Missing references
  are reported by `craft gen` as setup gaps; craft does not create a separate
  `ability_systems.json` fallback.
- `craft gen/improve --agent-sdk` uses focused craft context queries and
  validated craft patch dry-runs. In ordinary SDK mode Go still validates and
  saves the returned typed craft data.
- `craft improve --name <exact-name>` narrows the selected craft maps before any
  LLM or Agent SDK call. If the name is absent from the selected `--type`, the
  command fails rather than asking the agent to infer a target.
- `craft gen/improve --agent-sdk --agent-apply` may write only through
  name-scoped `tool patch craft --target <kind> --id <name> --apply` commands
  granted for the current batch. Go reloads the saved craft files and verifies
  the requested names; commands for other names or craft target kinds are denied.

Consumers:

- `write` uses craft data for chapter context and prose grounding.
- `logic.ChapterContinuityBuilder` loads craft data into write continuity.
- RPG/DSL conversion uses craft data to enrich entities, locations, items, and
  organization world rules.
- RPG/DSL conversion prefers explicit craft RPG metadata when present, then
  falls back to the older name/type/skill heuristics.
- Organization data grounds faction names used by character affiliations,
  outline enemies, and storyline pressure. `write` includes a compact relevant
  organization summary in chapter context; RPG/DSL conversion emits organization
  profile and state-effect rules. Downstream consumers may ignore it for old
  projects when the file is absent.

Change checklist:

- Update craft generation/review/improve skills.
- Update any context builders that summarize craft data for writing.
- Update RPG adapters if the new field maps into simulation.
- Add fixture or roundtrip tests for changed element shapes.

## `write.v1`

Producer:

- `cmd/write.go`
- `internal/agents/write_agent.go`
- Skills: `write-generate`, `write-review`, `write-improve`, `volume-review`
- Agent SDK workflow skills: `write-chapter-workflow` for
  `write gen --agent-sdk`, and `write-improve-workflow` for
  `write improve --agent-sdk`

Inputs:

- `novel.json`
- `story/setup/story_setup.json`
- `story/compose/outline.json`
- `story/craft/*.json`
- `story/recaps/<previous_chapter_id>.json`
- `story/rpg/*.rpg` when RPG checks are enabled
- Neighboring chapter text when context is requested

Outputs:

- `chapters/chapter-<chapter_id>.md`
- Review files under `story/reviews/`
  - `story/reviews/<volume_id>_review.json`: compatibility volume review
    summary used by existing improve flows.
  - `story/reviews/<chapter_id>_write_review.json`: full
    `models.ReviewResult` for the chapter, preserving dimensions and
    structured suggestions.
- Recaps through the recap stage when running `write pipeline`
- Optional `story/rpg/04_chapters.rpg`

Go types:

- `agents.ChapterContext`
- `models.ChapterContinuity`
- `models.RPGState`
- `models.ReviewResult`
- `models.ChapterRecap` through recap extraction

Required invariants:

- Chapter filename is based on `models.Chapter.ID`.
- Generation should use the current chapter, previous recap, nearby context,
  relevant craft context such as organizations, pre-chapter continuity, and
  configured word target. The continuity sent to write generation/review/improve
  represents facts before the target chapter begins; the target chapter's own
  events remain planned beats/deltas rather than already-true state.
- Optional setup `WritingStyle` is passed to write generation, review, improve,
  and volume review as part of the compact setup. Long `ReferenceExcerpt` values
  are truncated before prompt use. The excerpt may guide prose-level style only;
  it must not become continuity, plot, character, location, item, or world-rule
  state.
- Review/improve should preserve chapter identity and should not silently write
  results for a different chapter ID.
- `write pipeline --min-score` is the single CLI threshold for pipeline
  `NeedsRevision` decisions and post-improvement early stopping. It is expressed
  as 0-100 on the CLI, matching `models.ReviewResult.OverallScore`. Compatibility
  volume review files may still store old 1-10 scores; write improve normalizes
  them to percent before comparing. New write review compatibility files store
  percent scores.
- Batch write commands must surface chapter-level generation, review, improve,
  save, and final recap failures as command errors instead of reporting overall
  success after skipped failed chapters.
- When write humanization is enabled, deterministic AI-flavor checks may add
  style issues and suggestions before improve decisions. These checks only
  produce review feedback; all prose changes still go through the typed
  write-improve JSON contract.
- Write generation and improvement agent outputs are JSON objects with a
  `content` field; `content` is validated as prose before writing. Empty prose,
  JSON-as-prose, fenced code blocks, and severe length shortfalls are rejected.
- `write gen --agent-sdk` changes only the generation runtime. The SDK agent
  may query focused project context through `novelgen tool query context --type
  chapter-write` and run scoped checks, but it is not granted patch tools and
  does not write chapter files. Go remains the only writer for final markdown,
  using the same `content` validation and save path as ordinary `write gen`.
  Focused chapter context omits recap briefs whose identity/title does not
  match the outline or whose recap JSON is older than the saved chapter
  markdown, and returns warnings instead of feeding stale continuity anchors to
  the agent.
- `write review --agent-sdk` changes only the chapter review runtime. Go still
  selects chapters, loads saved final markdown, persists per-chapter review
  JSON, updates volume review compatibility files, and applies deterministic
  humanization/continuity checks. The SDK agent may query focused
  `chapter-write`/`chapter-repair` context, run scoped chapter/outline checks,
  and refresh stale chapter DSL when a simulation issue explicitly requires it.
  It is not granted patch tools and cannot write project state. Its Agent SDK
  output schema is compact and omits full `ReviewResult` fields such as
  dimensions, continuity issues, and iteration; Go converts score, summary,
  strengths, weaknesses, and suggestions into the persisted review contract.
- `write improve --agent-sdk` changes only the suggestion-based rewrite runtime.
  Review loading, chapter selection, deterministic transition/character fixers,
  and final markdown saves remain in Go. The SDK agent receives the current
  draft plus review suggestions, may query focused `chapter-write` or
  `chapter-repair` context, run scoped checks, and use `tool patch chapter`
  dry-run validation. With `--agent-apply`, the SDK agent may repeat a
  successful chapter patch dry-run with `--apply`; Go then reloads the saved
  chapter and continues deterministic fixers/post-save checks. Long chapter
  edits are limited to the scoped patch buffer `<chapter_id>-draft`; once a
  dry-run validates, the runner rejects further buffer mutation for that
  chapter and requires apply or final JSON. Chapter applies should use
  `--apply --refresh-derived` so the same validated patch write refreshes the
  target chapter DSL and returns a post-refresh focused check. Standalone
  `tool refresh chapter-dsl --id <chapter_id>` is reserved for workflows that
  cannot use `--refresh-derived` or for explicit tool-returned `next_actions`.
  The refresh path performs a target-chapter conversion and replaces only that
  chapter block in `story/rpg/04_chapters.rpg`; it preserves other chapter
  blocks so one repair loop does not rebuild or contaminate the whole chapter
  DSL. Direct file writes remain forbidden. Go rejects non-prose, severe
  shortfall, and severe Agent SDK overshoot before saving or accepting returned
  content. For focused repairs on existing chapters, Go uses the current
  chapter's narrative-unit count as the Agent SDK repair target unless review or
  check feedback explicitly asks for length expansion; this keeps one-issue
  repairs from drifting into whole-chapter rewrites. Chapter simulation treats
  structured knowledge/insight/clue/information/strategy deltas as protagonist
  growth (主角成长), so system-log and information-as-power (信息差) chapters do not need artificial
  breakthroughs, items, or allies when the actual payoff is an actionable
  information advantage.
- `write pipeline --agent-sdk` applies the same Agent SDK contracts to the
  pipeline generation, review, and improvement phases. The pipeline remains the
  orchestrator: review persistence, deterministic fixers, post-save checks,
  recap validation, RPG DSL refresh, and final file saves stay in Go. The review
  phase uses the read-only `write-review-workflow` contract and is not granted
  patch tools. With `--agent-apply`, only the improvement phase may use
  validated `tool patch chapter --apply`; generation still returns typed JSON
  content for Go to save. Pipeline `--agent-sdk` also routes final recap
  extraction through the Agent SDK recap workflow, while Go still owns recap
  validation, retry, and save.
- `polish --agent-sdk` is an orchestration wrapper over the same write-improve
  Agent SDK contract. Go still performs volume selection, holistic volume
  review, RPG issue collection, chapter save/reload, post-save checks, and
  recap refresh. With `--agent-apply`, only validated `tool patch chapter
  --apply` may write prose before Go reloads and verifies the result.
- If RPG DSL emission is enabled, generated or repaired DSL must parse and
  validate before it is treated as usable state. `write pipeline` refreshes
  chapter RPG DSL before improvement to gather simulation feedback, then once
  more after final recap extraction so simulation feedback is based on actual
  final chapter files without refreshing after every improvement pass.
- Continuity repairs must be applied as deterministic patches or validated AI
  outputs, not as unchecked text rewrites.
- Deterministic transition and character-presence checks are merged into write
  review compatibility files before improve decisions are made.
- Recaps generated by write commands must pass the same minimal recap gate used
  by `recap.v1`: retry once with explicit feedback when required fields are
  missing, warn on consistency issues, and save only a minimally valid recap.
- `write gen --recap-agent-sdk` and `write pipeline --recap-agent-sdk` route
  automatic recap extraction through the Agent SDK recap workflow. For pipeline,
  `--agent-sdk` also enables this recap runtime. This changes only the
  invocation runtime; Go still owns validation, retry, saving, and any
  subsequent recap check.

Consumers:

- `recap` extracts continuity anchors from final chapter text.
- `rpg-dsl` extracts chapter-level state deltas and simulation data.
- `export` reads final chapters.
- Subsequent `write` calls read previous recaps and neighboring chapters.

Change checklist:

- Update write skills when context shape changes.
- Update continuity helpers when adding new context sources.
- Update recap expectations when final chapter output format changes.
- Add focused tests for deterministic patches or RPG validation paths.

## `recap.v1`

Producer:

- `cmd/recap_cmd.go`
- `internal/agents/recap_agent.go`
- `cmd/write.go` through `write pipeline`
- Skill: `recap-extract`
- Agent SDK workflow skill: `recap-extract-workflow` when `recap gen --agent-sdk`

Inputs:

- `chapters/chapter-<chapter_id>.md`
- Chapter ID and title from outline
- Optional feedback when retrying extraction
- In Agent SDK mode, the full chapter text is still supplied by Go.
- In `--agent-apply` mode, the agent may query only chapter-scoped recap repair
  context/checks and may write only through validated `tool patch recap --id
  <chapter_id> --apply`.

Outputs:

- `story/recaps/<chapter_id>.json`

Go type:

- `models.ChapterRecap`

Required invariants:

- `ChapterID` and `Title` match the outline chapter.
- Recap agent output does not own identity fields: Go overwrites
  `chapter_id` and `title` from the selected outline chapter before validation
  or saving.
- `Location`, `Present`, `LastLine`, and `NextOpeningHint` are present.
- `PlotBeats`, `Decisions`, `Reveals`, `Unresolved`, `Promises`, `Items`, and
  `Status` summarize what actually happened, not what the outline planned.
- `NextOpeningHint` is short and connects to `LastLine`.
- `recap.ValidateMinimal` passes before a recap is considered available.
- `recap gen --agent-sdk` uses the same typed output, retry, validation, and
  save path as ordinary recap extraction; it only changes the invocation
  runtime.
- `recap gen --agent-sdk --agent-apply` does not save returned JSON through the
  ordinary path. The saved recap must change via `tool patch recap`; Go reloads
  that saved patch result and leaves the existing recap unchanged if no patch was
  applied.
- Recap extraction returns an error after retry if `recap.ValidateMinimal` still
  fails. Consistency failures remain warnings because they are heuristic
  continuity signals.
- `novelgen tool check quality --target recap --scope chapter --id <chapter_id>`
  exposes the saved recap gate to agents. It is deterministic and does not run
  RPG/DSL simulation.

Consumers:

- Continuity helpers read previous recaps.
- `write` uses recaps as high-signal context for the next chapter.

Change checklist:

- Update `recap-extract` skill and `models.ChapterRecap` together.
- Update `logic/continuity/recap` validation when required fields change.
- Update write context loading if recap semantics change.

## `rpg-dsl.v1`

Producer:

- `cmd/rpg_dsl.go`
- `cmd/write.go` when `--emit-rpg-dsl` is enabled
- `internal/agents/chapter_to_dsl_agent.go`
- RPG adapters and converters under `internal/rpg/`

Inputs:

- `story/setup/story_setup.json`
- `story/compose/outline.json`
- `story/craft/*.json`
- `chapters/*.md`
- `story/recaps/*.json`

Outputs:

- `story/rpg/01_outline.rpg`
- `story/rpg/01_outline_vNN.rpg` when `rpg-dsl convert --phase outline
  --volume N` converts a single generated global outline volume
- `story/rpg/02_craft.rpg`
- `story/rpg/03_systems.rpg`
- `story/rpg/04_chapters.rpg`
- Merged or exported RPG data where commands request it

Core packages:

- `internal/rpg/dsl/ast.go`
- `internal/rpg/dsl/parser*.go`
- `internal/rpg/dsl/validator.go`
- `internal/rpg/dsl/converter.go`
- `internal/rpg/dsl/simulator.go`
- `internal/rpg/dsl/merger.go`

Required invariants:

- DSL must parse before validation.
- DSL must validate before conversion/simulation.
- New constructs require AST, parser, validator, converter, docs, and tests.
- Runtime constructs require simulator/evaluator/hook tests.
- Phased DSL fragments must merge without losing stable IDs.
- Validator must catch structural cross-reference problems before conversion,
  including missing location/item/arc/chapter references, invalid event setup,
  and inconsistent `state_delta` arithmetic.
- Outline-only conversion must not require craft files. A single-volume outline
  conversion must reject empty future volumes instead of emitting placeholder
  chapters.
- When `01_outline.rpg` and `01_outline_vNN.rpg` coexist, outline fragments
  merge by stable arc/chapter IDs: the volume fragment may update matching
  chapters, but it must not drop chapters from the full outline fragment.
- AI-generated DSL repair must be re-parsed and re-validated before saving.
- Outline `state_anchor` conversion emits both human-readable state deltas
  (`cultivation`, `item`, `injury`, etc.) and deterministic numeric deltas for
  combat-relevant progression when the text contains them:
  `gene.level`, `gene.stability`, `mech.form`, `mech.level`, `mech.energy`,
  `mech.module`, `mech.module_blueprint`, and `mech.damage`.
- The RPG simulator folds numeric `gene` and `mech` deltas into
  `ProtagonistState` before combat balance checks; text heuristics are only a
  fallback or tactical modifier, not the only source of mecha/gene power.
- Craft phase conversion consumes optional RPG metadata from
  `story/craft/*.json`: character stats/roles become player or enemy defaults,
  location map/danger/tags become location properties, and item type/rarity/tags
  become DSL item fields/effects. Organization profiles become world rules with
  `organization.profile` and `organization.state_effect` triggers. Old craft
  files without these fields remain valid and use deterministic fallback
  inference.

Consumers:

- RPG simulator and benchmark tools.
- `ChapterContinuityBuilder` can fold generated RPG state deltas into write
  continuity through `models.RPGState`.
- `write pipeline` can use generated DSL and simulation feedback to guide
  review/improve decisions.

Change checklist:

- Update `docs/RPG_DSL_SPEC.md` and related RPG docs for syntax/semantics.
- Add parser and validator tests for every new construct.
- Add simulator/evaluator tests when runtime behavior changes.
- Verify representative commands still parse/validate merged DSL.

## `draft.v1` Legacy

Producer:

- `cmd/draft.go`
- `internal/agents/draft_agent.go`
- Skills: `draft-generate`, `draft-review`, `draft-improve`

Outputs:

- `drafts/<chapter_id>.md`

Contract status:

- Draft is a legacy optional workflow. New features should prefer direct
  `write pipeline` unless they intentionally support old projects.
- Do not introduce new persistent dependencies on `drafts/` without documenting
  why final chapters cannot be used.

## Contract Change Review

Use this checklist before committing a stage-level change:

- Which stage owns the new or changed field?
- Which commands or agents produce it?
- Which commands or agents consume it?
- Is it required or optional?
- What happens when old project files do not contain it?
- Is there deterministic validation or normalization?
- Which skill files need prompt/schema updates?
- Which tests prove the contract still holds?
