# Agent Tools

`novelgen tool query` provides read-only JSON queries for Claude/agent use.
`novelgen tool check` provides scoped quality/simulation checks, and
`novelgen tool patch setup|outline|craft|recap|chapter` applies validated patches through Go.
Run commands from a novel project root, or any child directory under a project
containing `novel.json`.

The command is organized by four query sections:

```bash
novelgen tool query story-setup [flags]
novelgen tool query outline [flags]
novelgen tool query craft [flags]
novelgen tool query chapter [flags]
novelgen tool query context --type craft-character --name <character>
novelgen tool query context --type craft-item --name <item>
novelgen tool query context --type craft-location --name <location>
novelgen tool query context --type craft-organization --name <organization>
novelgen tool query context --type outline-volume --id <volume_id>
novelgen tool query context --type outline-repair --id <volume_or_chapter_id> [--name <issue_category>]
novelgen tool query context --type outline-global-repair [--name <issue_category>]
novelgen tool query context --type recap-repair --id <chapter_id>
novelgen tool query context --type chapter-write --id <chapter_id>
novelgen tool query context --type chapter-repair --id <chapter_id> [--name <issue_category>]
novelgen tool query logs --type prompts|responses|agent-live [--name <agent>] [--view index|brief]
novelgen tool query logs --id <relative_log_path> --content --view brief
novelgen tool check quality|simulation|all --target setup
novelgen tool check quality|simulation|all --target outline [--scope all|volume|chapter] [--id <id>]
novelgen tool check quality|simulation|all --target chapter --scope chapter --id <chapter_id>
novelgen tool check schema --target craft --scope character|item|location|organization [--id <name>]
novelgen tool check quality|all|schema --target recap --scope chapter --id <chapter_id>
novelgen tool patch setup --patch-json '<json>' [--apply]
novelgen tool patch outline --target chapter|volume --id <id> --patch-json '<json>' [--apply]
novelgen tool patch craft --target character|item|location|organization --id <name> --patch-json '<json>' [--apply]
novelgen tool patch recap --id <chapter_id> --patch-json '<json>' [--apply]
printf '%s' '{"content":"..."}' | novelgen tool patch chapter --id <chapter_id> [--apply]
novelgen tool patch-buffer clear --id <buffer_id>
novelgen tool patch-buffer append --id <buffer_id> --text '<chapter text chunk>'
printf '%s' '<chapter text chunk>' | novelgen tool patch-buffer append --id <buffer_id> --stdin
novelgen tool patch-buffer show --id <buffer_id> [--max-chars 300]
novelgen tool patch chapter --id <chapter_id> --patch-buffer <buffer_id> [--apply --refresh-derived]
novelgen tool refresh chapter-dsl --id <chapter_id> [--batch-size 10]
```

When using `--patch-json`, pass ASCII-only JSON. Escape non-ASCII text as
`\uXXXX` instead of putting raw Chinese directly in the argv string; Windows
agent shells can turn raw non-ASCII argv text into `????`, which the patch
validator correctly rejects as garbled. For long chapter prose, use
`tool patch-buffer`.

For Chinese JSON patches to setup, outline, craft, or recap, prefer piping
compact literal JSON on stdin instead of using `--patch-json`:

```bash
printf '%s' '{"changed_chapters":[{"id":"P1-V1-C1","summary":"中文摘要"}]}' | novelgen tool patch outline --target volume --id P1-V1
```

Do not run Python, Node, PowerShell, help commands, temp files, or shell
redirection to encode patch JSON inside Agent SDK workflows. If a patch dry-run
is valid and the workflow allows writes, repeat the same stdin-piped command
with `--apply`.

Agent SDK workflows should run the exact `novelgen tool ...` commands provided
by `required_queries`, `navigation`, or `workflow` fields. Do not add shell
wrappers, `2>&1`, redirection, `Get-Content`, `Select-String`, `grep`, or temp
files. If output is too large, narrow the query with `--view`, `--fields`,
`--limit`, `--id`, or `--name`. UTF-8 handling belongs to the runtime/logging
layer, not to ad hoc agent shell commands.

Query and check never mutate project state. Patch defaults to dry-run; it writes
only when `--apply` is present. Setup patches merge into the typed story setup,
normalize, run quality + simulation checks, and checkpoint `story_setup.json`.
Outline patches preserve IDs and run scoped quality + simulation checks. Craft
patches preserve raw unknown fields, validate against the typed craft schema,
normalize RPG/DSL metadata, and checkpoint the target craft file.
Recap patches preserve `chapter_id`, force the title from the outline when
available, run the recap minimal/consistency/title check, checkpoint the old
recap, and save only when `--apply` is present and no blocking issue remains.
Use `--view brief|index|full` to control output size. The default is `brief`.
Agents should use `index`/`brief` for navigation and only use `full` for a
precise small object.
Use `--fields a,b,c` to project only selected JSON fields from each result while
preserving navigation keys such as `id`, `title`, and `path`. Prefer this over
shell pipes like `Select-String` or `grep`.
For history-aware continuation, use `tool query logs --view index` first. The
logs query returns only relative log ids, agent names, sizes, mtimes, and
detail queries in index mode. It returns capped content only when `--content`
is paired with an exact `--id`; do not read `logs/` with shell commands.
For Agent SDK runs, prefer
`novelgen tool query logs --view index --limit 5` before
opening prompt/response logs. `agent-live` index/brief results include a
structured `summary` with `model`, `final_model`, `sdk_skills`, tool-call
counts, `query_calls`, `check_calls`, `patch_calls`, `patch_applies`,
`allowed_tool_commands`, and `denied_tool_commands`. The summary redacts
large stdin patch-buffer writes as `--stdin <stdin>` and Claude temporary tool
output reads as `<claude-temp-tool-output>`, so agents can inspect what the last
run did without pulling full logs into context.
For larger craft patches, prefer piping a literal compact JSON string to
`novelgen tool patch craft ...`; do not read patch JSON from files or execute
placeholder text such as `<json>`.
For long chapter patches, prefer `tool patch-buffer` over Python, Node, temp
files, `Get-Content`, or shell redirection. `patch-buffer` writes only a
novelgen-managed temporary buffer under `.novelgen/agent-patches`; story state
still changes only through `tool patch chapter --patch-buffer ... --apply`.
When applying a chapter patch in an Agent SDK workflow, prefer
`tool patch chapter ... --apply --refresh-derived`. That single command writes
the chapter, refreshes the target chapter DSL, and returns a post-refresh
focused check. Only run standalone `tool refresh chapter-dsl --id <chapter_id>`
when the workflow does not allow `--refresh-derived` or a tool-returned
`next_actions` entry explicitly asks for it. The refresh path invokes the AI
chapter-to-DSL conversion for the target chapter, writes a target-chapter batch
cache, and replaces only that chapter block inside `story/rpg/04_chapters.rpg`.
Other chapter blocks are preserved. This keeps agent repair loops incremental
and avoids a one-chapter patch being polluted by a large multi-chapter DSL
conversion context.

## Story Setup

```bash
novelgen tool query story-setup
novelgen tool query story-setup --type storyline --name "Main arc"
novelgen tool query story-setup --view index
novelgen tool query story-setup --type premise --name "cultivation"
novelgen tool query story-setup --type core-cast --name "Lin"
novelgen tool query story-setup --type resource --name "spirit stone"
novelgen tool query story-setup --type timeline
novelgen tool query story-setup --type long-form-plan
novelgen tool query story-setup --type search --name "insect swarm"
```

Use `--type search --name <keyword>` when an agent needs compact setup context
for a character, item, faction, resource, or world concept. It searches across
core cast, storylines, premises, resources, and timeline entries, clips long
text, and returns follow-up detail queries.

## Outline

```bash
novelgen tool query outline
novelgen tool query outline --type part --id part_1
novelgen tool query outline --type volume --id vol_1
novelgen tool query outline --type volume --id vol_1 --view brief
novelgen tool query outline --type chapter --id chap_001
novelgen tool query outline --type chapter --id chap_001 --fields storyline_advances,chapter_payoff,conflict --view brief
novelgen tool query outline --type refs --entity-type character --name "Lin"
novelgen tool query outline --type refs --entity-type item --name "Star Core"
novelgen tool query outline --type refs --entity-type location --name "Mine"
novelgen tool query outline --type events --entity-type item --name "Star Core"
novelgen tool query outline --type events --chapter-id chap_001 --fields result,target,target_type --view brief
novelgen tool query outline --type events --volume-id vol_1 --fields event_index,chapter_id,action,target,type --view brief
```

## Craft

```bash
novelgen tool query craft
novelgen tool query craft --type character --name "Lin"
novelgen tool query craft --type item --name "Star Core"
novelgen tool query craft --type location --name "Mine"
novelgen tool query craft --type organization --name "Guild"
```

Craft lookup matches keys, names, aliases where available, and profile text.

## Chapter

```bash
novelgen tool query chapter
novelgen tool query chapter --id chap_001
novelgen tool query chapter --id chap_001 --content
novelgen tool query chapter --entity-type character --name "Lin"
novelgen tool query chapter --entity-type item --name "Star Core" --content
novelgen tool query chapter --entity-type location --name "Mine"
novelgen tool query chapter --type events --chapter-id chap_001
```

The `reasons` field explains why a chapter matched.

## Context

`context` returns compact workflow-specific bundles so an agent can start from
the right facts without manually chaining broad queries.

```bash
novelgen tool query context --type craft-character --name "Lin"
novelgen tool query context --type craft-item --name "Star Core"
novelgen tool query context --type craft-location --name "Mine"
novelgen tool query context --type craft-organization --name "Guild"
novelgen tool query context --type outline-volume --id "P1-V1"
novelgen tool query context --type outline-repair --id "P1-V1-C3" --name "logic"
novelgen tool query context --type outline-global-repair --name "structure"
novelgen tool query context --type recap-repair --id "P1-V1-C3"
novelgen tool query context --type chapter-write --id "P1-V1-C3"
novelgen tool query context --type chapter-repair --id "P1-V1-C3" --name "logic"
```

Most context bundles include `next_actions`: a small ordered plan for the
current task. Agents should follow `next_actions` first, use the current bundle
before querying more data, and execute optional `navigation.detail_queries` or
craft/setup queries only when the `when` condition applies.
Context query responses also include `meta.context_budget`. Treat it as the
query budget contract: `index` is route-only and normally upgrades to the exact
same context with `--view brief`; `brief` is focused facts and should only lead
to exact target queries named by `navigation` or `next_actions`; `full` should
not broaden further. The budget `avoid` list is mandatory guidance for agents:
do not query full setup, full outline, all chapters, source code, or derived RPG
refresh commands unless the current workflow explicitly tells you to.
Use `--view index` on context when you only need the route: it keeps identity,
check summary, navigation, workflow, next_actions, and stats while omitting
excerpts, events, and target object detail. Use `--view brief` when you are
ready to make or validate a patch.

`craft-character`, `craft-item`, `craft-location`, and `craft-organization`
include a clipped story setup brief, matching existing craft, outline refs or
matching events, relevant chapters where supported, stats, schema/patch
navigation, and follow-up detail queries. Agents should call this before
generating or improving craft, then use targeted chapter/event/craft queries
only if the bundle says more detail is needed.

`outline-volume` includes a clipped story setup index, the target volume, the
neighboring volumes, a compact entity index, volume events, stats, and
follow-up queries for chapter detail, events, setup search, scoped checks, and
validated outline patches. Agents should call this before improving a volume
instead of querying the full setup or full outline.

`outline-repair` is the smallest context bundle for an already identified
check issue. Pass a volume ID or chapter ID as `--id`; optionally pass an issue
category such as `logic`, `plot`, `structure`, `character`, or `pacing` through
`--name`. It returns the scoped check result, the current target object,
nearby chapter/volume context, compact events, and a legal `patch_shape`.
For chapter IDs it still points patching at the parent volume, so agents do not
need to guess the `changed_chapters` wrapper.

`outline-global-repair` is the focused entry after a full-outline check reports
global issues such as missing long-form alignment, unresolved mystery pressure,
or setup-backed faction tier problems. Use `--view index` first: it returns the
check summary, issue navigation, patchable routes, workflow, next actions, and
stats without loading full setup or full outline. Use `--view brief` only before
building a patch or final diagnosis; patch only issue_context entries that have
both `patch_query` and `patch_shape`.
For `mysteries`, the context includes a bounded `mystery_threads` ledger and at
most one patchable unresolved thread route. Treat it like a small code diff:
dry-run one `issue_context` patch, apply the same patch only if validation
passes, run the provided post-patch check, then return final JSON. Do not loop
over every unresolved mystery in one invocation, do not use placeholder text
such as `<json>`, and do not broaden to full outline context.

`recap-repair` is the focused context bundle for recap check issues. It returns
the saved recap, scoped recap check, compact outline chapter, chapter opening
and closing excerpts, and a workflow with the post-save check. It does not
include full chapter text and does not provide a patch query, because recap
files are still written by Go through recap extraction.

`chapter-write` is the focused entry bundle for writing or revising one
chapter. It returns the target chapter brief, parent volume brief, adjacent
chapter briefs, previous/current recap briefs when present, a clipped existing
manuscript excerpt, target chapter events, entity indexes, and concrete
follow-up queries for craft context and recap checks. Agents should use it
before writing a chapter instead of querying full setup, full outline, or all
chapter content. Use the returned `craft_context_queries` only for named
entities that matter to the scene. Recap briefs are included only when they
match the outline chapter identity/title and are not older than the saved
chapter markdown; stale or mismatched recaps are omitted with warnings so the
agent does not mix an old storyline into current prose.

`chapter-repair` is the focused entry bundle after `tool check --target chapter`
reports final prose or RPG simulation issues. It returns the scoped chapter
check, target outline facts, saved chapter opening/closing excerpt, entity
index, quality/simulation/all check commands, and the `write improve
--agent-sdk` repair command. Use it instead of manually combining full chapter
content, full outline, and RPG files.

## Check

```bash
novelgen tool check all --target setup
novelgen tool check all --target setup --category core_cast --min-priority low --max-issues 8
novelgen tool check quality --target outline --scope volume --id P1-V1
novelgen tool check simulation --target outline --scope chapter --id P1-V1-C1
novelgen tool check all --target outline --scope volume --id P1-V1
novelgen tool check all --target outline --scope volume --id P1-V1 --min-priority medium --max-issues 12
novelgen tool check all --target outline --scope volume --id P1-V1 --category logic,plot --min-priority low --max-issues 8
novelgen tool check quality --target chapter --scope chapter --id P1-V1-C1 --max-issues 8
novelgen tool check simulation --target chapter --scope chapter --id P1-V1-C1 --max-issues 8
novelgen tool check schema --target craft --scope character --id "Lin"
novelgen tool check quality --target recap --scope chapter --id P1-V1-C1
```

`quality` runs deterministic structure/direct rules. `simulation` runs the
RPG/DSL simulator. `all` merges both result sets and reports whether any issue
is blocking. `--target setup` checks the saved story setup contract before
outline/craft/write consume it. `quality --target chapter` checks saved final
chapter markdown for deterministic prose/format/length/style issues without
invoking an LLM or rebuilding RPG DSL. `simulation --target chapter` loads and
merges existing `story/rpg/*.rpg` files, then runs deterministic
`SimulateChapter`; it reports missing/invalid DSL instead of converting chapter
text through an LLM. `schema --target craft` validates saved craft
objects against the typed Go schema and text/metadata safety rules. After an agent-owned
`tool patch craft ... --apply`, run the matching schema check and repair any
blocking issue before returning. `--target recap` validates saved
`story/recaps/<chapter_id>.json` continuity anchors. Minimal recap failures are
blocking; `next_opening_hint` consistency issues are non-blocking warnings.
Use `--min-priority`, `--category`, and `--max-issues` to keep agent-facing
check output focused; the summary still reports the full issue counts.
Check responses include top-level `next_actions` and `meta.check_budget`. If no
issues are returned, follow `return_final_json` and stop querying. If issues are
returned, repair one issue at a time using the first returned issue's
`navigation.repair_route_query` before any broader context query; only call
`repair_context_query`, patch dry-run, or focused recheck when those commands
are listed by `next_actions` or `issues[].navigation`.
For chapter simulation issues caused by stale or missing derived RPG DSL,
top-level `next_actions` starts with `refresh_derived_dsl`. Run that refresh and
the returned `post_refresh_check` before querying repair context or patching
prose. Treat prose repair as necessary only if the post-refresh check still
returns issues.
Global issues without `target_id` still return conservative navigation, usually
an index-sized outline/setup route plus a focused recheck. Treat that as the
upper bound for context; do not scan all chapters to interpret a global warning.
Each returned issue may include `navigation` with `repair_route_query`,
`repair_context_query`, `detail_queries`, `focused_check_query`,
`simulation_check_query`, and
`patch_query`. Chapter simulation issues may also include `refresh_query` and
`post_refresh_check_query`; run those first when the issue is stale or missing
RPG DSL, and skip prose edits if the refreshed check clears the issue. For
outline and final-chapter issues, prefer `repair_route_query` first when present;
it returns an index-sized route with `next_actions`. Run
`repair_context_query` only when the route indicates that detailed target facts,
nearby context, event facts, or excerpts are needed. Use the other navigation
commands only when the route or repair bundle shows that more detail is needed.
Agent SDK compose workflows also copy focused navigation into
`review_result.suggestions[].navigation` when deterministic pre-check issues are
passed through the prompt, so the agent can start from the same narrow queries
without re-running a broad scan.

## Patch

```bash
printf '%s' '{"theme":"Updated theme"}' | novelgen tool patch setup
printf '%s' '{"summary":"Updated summary"}' | novelgen tool patch outline --target chapter --id P1-V1-C1
printf '%s' '{"changed_chapters":[{"id":"P1-V1-C1","summary":"Updated summary"}]}' | novelgen tool patch outline --target volume --id P1-V1
printf '%s' '{"changed_events":[{"chapter_id":"P1-V1-C1","event_index":0,"type":"plan","action":"plan"}]}' | novelgen tool patch outline --target volume --id P1-V1
printf '%s' '{"notes":"Updated notes"}' | novelgen tool patch craft --target character --id "Lin"
printf '%s' '{"power_level":3,"rarity":"rare"}' | novelgen tool patch craft --target item --id "Star Core"
printf '%s' '{"location":"Mine","present":["Lin"],"last_line":"Lin opens the sealed door.","next_opening_hint":"Lin opens the sealed door and sees blue light."}' | novelgen tool patch recap --id P1-V1-C1
printf '%s' '{"content":"# Opening\n\nLin repairs the scene."}' | novelgen tool patch chapter --id P1-V1-C1
printf '%s' '{"summary":"Updated summary"}' | novelgen tool patch outline --target chapter --id P1-V1-C1 --apply
```

On Windows PowerShell, prefer piping compact JSON:

```powershell
"{""theme"":""Updated-theme""}" | novelgen tool patch setup
"{""summary"":""Updated-summary""}" | novelgen tool patch outline --target chapter --id P1-V1-C1
"{""notes"":""Updated-notes""}" | novelgen tool patch craft --target character --id Lin
"{""last_line"":""Updated-last-line"",""next_opening_hint"":""Updated-last-line continues.""}" | novelgen tool patch recap --id P1-V1-C1
"{""content"":""# Opening\n\nLin repairs the scene.""}" | novelgen tool patch chapter --id P1-V1-C1
```

Patch dry-run and apply both run scoped `all` validation, so the returned
`check` includes deterministic quality rules and RPG/DSL simulation for setup
or outline patches.
Read `check.meta.coverage` before deciding what to do next. For outline
checks and outline patches, `coverage.simulation_backend` is
`in_memory_model_adapter`, `coverage.invokes_llm=false`,
`coverage.uses_derived_rpg_files=false`, and
`coverage.refresh_required_before_simulation=false`; do not refresh or rebuild
`story/rpg/01_outline.rpg` for an outline patch. Chapter prose patches are
different: they return `simulation_requires_derived_dsl_refresh=true` and
`next_actions` for `tool refresh chapter-dsl`, because chapter simulation reads
derived `story/rpg/04_chapters.rpg`.
`--apply` fails without writing files when validation has blocking issues.
Patch results may include `next_actions`. For dry-runs, follow
`apply_validated_patch` only when the workflow allows writes and
`check.blocking=false`; follow `repair_patch_content` when the dry-run check is
blocking. For applied patches, prefer the returned `post_apply_check` command
instead of inventing a broader check.
In Agent SDK workflows, `--apply` is rejected unless the same target and same
patch input have already had a successful dry-run in the current agent run; a
dry-run that returns `repair_patch_content` must be repaired and dry-run again
before apply.

JSON patch tools use typed merge semantics, not blind file replacement. Object
fields are merged recursively, so an outline patch may provide only
`{"chapter_payoff":{"desire":"...","hook":"..."}}` and Go preserves the
existing `pressure`, `clever_move`, `reward`, and other nested fields. Arrays
still replace unless a tool documents special upsert behavior, so patch
`events`, `storyline_advances`, `resource_ledger`, and similar lists only when
you are intentionally replacing the complete list.

For recap patches, do not include `chapter_id` or rely on a patched `title`.
Go fixes `chapter_id` from `--id` and fixes `title` from the outline chapter
when available. Patch only content fields such as `location`, `present`,
`plot_beats`, `last_line`, and `next_opening_hint`. Blocking minimal-gate
failures reject `--apply`; fuzzy consistency issues are returned in `check`.

Chapter patches update saved final chapter markdown. Use only
`{"content":"complete revised chapter markdown or prose"}`. For full chapters,
prefer piping a literal compact JSON string, for example
`printf '%s' '{"content":"# Opening\n\n..."}' | novelgen tool patch chapter --id P1-V1-C1`;
on Windows PowerShell, set UTF-8 before piping Chinese stdin:
`$OutputEncoding = [System.Text.UTF8Encoding]::new(); [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); [Console]::InputEncoding = [System.Text.UTF8Encoding]::new(); '{"content":"# Opening\n\n..."}' | novelgen tool patch chapter --id "P1-V1-C1"`.
Do not use Python, temp files, `Get-Content`, or redirection to construct the
payload. Go normalizes the chapter heading from the outline title when it is
missing, runs deterministic final-chapter quality checks before writing, writes
a checkpoint under `chapters/checkpoints/`, saves `chapters/chapter-<id>.md`,
then returns a post-apply chapter quality check result. It does not immediately
trust chapter simulation, because `story/rpg/04_chapters.rpg` must be refreshed
from the changed prose first.
The patch result also includes `next_actions`: after dry-run, apply only when
the check is non-blocking and writes are explicitly allowed; after `--apply`,
run the returned `refresh_derived_dsl` command and then `post_refresh_check`.
Chapter simulation checks should be trusted only after the derived chapter RPG
DSL has been refreshed from the changed prose.

Setup patches merge a JSON object into `story/setup/story_setup.json`; they do
not require `--id`. Applied patches write a checkpoint under
`story/setup/checkpoints/` and regenerate `story_setup.md`. Use setup patches
for high-level contract fields such as `theme`, `rules`, `storylines`,
`premises`, `core_cast`, `world_resources`, and `long_form_plan`; do not put
chapter-scale outline or detailed craft cards into setup.
For object arrays, setup patch uses keyed upsert semantics rather than whole
array replacement: `storylines`, `premises`, and `world_resources` match by
`name`, `core_cast` matches by `id` then `name`, and `world_timeline` matches
by `related_mystery` or `year|event`. Agents should submit only the few array
items they need to add or update.
String arrays such as `rules`, `genres`, and the list fields inside
`long_form_plan`/`writing_style` append and deduplicate. `long_form_plan` and
`writing_style` are field-merged, so a patch may safely provide only
`main_loop`, `payoff_cadence`, `principles`, or another specific field.
To replace one existing `rules[n]` or `genres[n]` entry, use the patch-only
virtual fields `rules_patch` or `genres_patch`, for example
`{"rules_patch":[{"index":1,"value":"Shortened rule."}]}`. These virtual
fields are consumed by the tool layer and are not written into
`story_setup.json`.

Volume patches must use `changed_chapters` for chapter fields or
`changed_events` for individual event fields; replacing the whole `chapters`
array is rejected. Every changed chapter keeps its existing ID. `changed_events`
uses `chapter_id` plus the 0-based `event_index` returned by
`tool query outline --type events`, and field-merges only the supplied event
fields. Applied patches write a checkpoint under `story/compose/checkpoints/`
before saving `outline.json` and `outline.md`.

Craft patches patch a single element by key/name. Supported targets are
`character`, `item`, `location`, and `organization`. The tool validates the
patched object against the corresponding Go model, rejects suspicious mojibake
text, normalizes craft metadata such as `power_level`, `danger_level`,
`dsl_tags`, and `state_effects`, and preserves unknown raw JSON fields where
possible. Applied craft patches write a checkpoint under `story/craft/checkpoints/`.
For character `rpg_stats`, use only `str`, `agi`, `int`, `vit`, `hp`, `mp`,
and `level`. Common aliases such as `strength`, `agility`, `intelligence`, and
`endurance` are normalized, but unsupported free-form stats such as
`perception` or `willpower` are rejected during dry-run.
After applying a craft patch, verify the saved object with:

```bash
novelgen tool check schema --target craft --scope character --id "Lin"
```

The craft schema check also reports non-blocking item consistency issues when
`owner` is neither a known craft character nor a stable generic owner such as
`主角`.

## Agent SDK Tool Evidence Guard

Agent SDK workflows that require tool activity (queries, checks, patches)
enforce the requirement **in-flight**: the runner registers a Stop hook that
blocks the agent from ending its turn until the required `novelgen tool ...`
commands have actually been observed. When the agent tries to stop without the
required evidence (for example, without running `novelgen tool check`), the
runner returns a Stop-hook `decision: "block"` with the missing-command
instruction, so the agent continues the same turn and completes the work
instead of being rejected after the fact.

Go still revalidates the live log after the run (`tool evidence` counters such
as `check_calls`), so the in-flight guard is an early correction loop rather
than a replacement for the deterministic post-run validation. If the agent
still exits without the required evidence (for example after hitting the turn
limit), the run fails with the same evidence error and checkpoint/resume
handling applies as before.

`compose improve --agent-sdk` additionally retries transient runner/network
failures (up to 3 attempts total): because each volume is checkpointed to
`story/compose/outline_improve_progress.json`, a retry resumes from the last
completed volume instead of restarting the whole range.

## Agent SDK Outline Apply

`compose improve --agent-sdk` defaults to the conservative flow: the agent uses
query/check/patch dry-runs, returns `volume_patch`, and Go merges/saves. This
keeps Go as the final writer for ordinary compose workflows.

The per-volume workflow may also read the immediately adjacent volumes'
`payoff_contract/summary` (read-only) to verify cross-volume continuity facts;
adjacent volumes are never patchable in this workflow.

Use `--agent-apply` when you want the Agent SDK workflow to write through the
validated patch tool, closer to an edit/check loop:

```bash
novelgen compose improve --agent-sdk --agent-apply --volume 1 --max-rounds 1
novelgen compose pipeline --agent-sdk --agent-apply --from-volume 1 --to-volume 1
```

`compose improve --agent-sdk --repair-budget <n>` controls how many targetable
quality/simulation issues each post-check repair pass processes (default 20).
After the run completes, a Markdown improvement report is written to
`logs/compose_improve_report_<timestamp>.md` with per-volume scores, change
summaries, remaining issues, and the post-gate remaining issue count.

Even in apply mode, the agent still cannot write files directly. It may only
run `novelgen tool patch outline --target volume ... --apply` after a successful
dry-run of the same patch. The patch tool preserves IDs, runs scoped quality +
simulation validation, checkpoints, and rejects blocking changes before writing.
Go remains the state owner: it defines the target scope, validates patch shape,
merges overlays, preserves IDs, reruns checks, writes checkpoints/markdown/JSON,
and reloads applied state. In apply mode the final agent JSON is intentionally
small: `review_result`, `applied_patches`, `applied_patch_count`, and
`final_check`; it does not echo `volume_patch` back to Go. The agent only
supplies a focused edit intent and may exercise that intent through the
validated patch tool when explicitly allowed.

Compose Agent SDK workflows use the compact `novel-tools-core` skill plus the
stage workflow skill instead of the full `novel-tools` manual. Keep stage
workflow skills short and push detailed context into `tool query context ...`
rather than startup prompt text.

## Agent SDK Setup Improve/Regen

`setup improve --agent-sdk` and `setup regen --agent-sdk` let the Agent SDK
workflow inspect setup through `tool query`, run setup checks, and propose a
minimal `setup_patch`:

```bash
novelgen setup improve --agent-sdk --max-rounds 1
novelgen setup improve --agent-sdk --prompt "make the protagonist promise sharper"
novelgen setup regen --agent-sdk --prompt "regenerate the setup around a sharper main promise"
```

In the default mode, the agent cannot write files. It must dry-run
`novelgen tool patch setup` with either ASCII-only `--patch-json` or a
stdin-piped compact JSON patch, return the patch, and Go handles merge,
suspicious-text validation, setup quality + simulation checks, checkpointing,
markdown export, and saving. For Chinese setup patches, prefer:

```bash
printf '%s' '<compact-json>' | novelgen tool patch setup
```

Use `--agent-apply` to let the agent perform the validated write loop itself:

```bash
novelgen setup improve --agent-sdk --agent-apply --max-rounds 1
novelgen setup regen --agent-sdk --agent-apply --prompt "repair setup check issues"
```

Apply mode still only grants `novelgen tool patch setup ... --apply` after a
successful dry-run of the same patch. Direct shell writes, source exploration,
and ad hoc Python/Node/PowerShell JSON encoding remain denied.

## Agent SDK Craft Generation

`craft gen --agent-sdk` supports character, item, location, and organization
craft generation. `craft improve --agent-sdk` supports the same craft targets
for existing craft. By default the agent queries facts and dry-runs
`tool patch craft`; Go validates the returned JSON and saves the final craft
data. Craft SDK workflows use `novel-tools-core` plus the craft workflow skill:

```bash
novelgen craft gen --agent-sdk --chapter P1-V1-C1
novelgen craft improve --agent-sdk --type characters --batch 1
novelgen craft improve --agent-sdk --agent-apply --type characters --name "李侑"
```

Use `craft improve --name <exact-name>` for targeted creative work on one saved
object, such as a protagonist craft card in a cloned sandbox. Go filters the
saved craft maps before invoking the SDK workflow; if the name is absent in the
selected `--type`, the command fails instead of letting the agent guess.

To test an agent-owned write loop, add `--agent-apply`. In this mode the agent
must still query facts and run patch dry-runs first, but it may write only
through the name-scoped patch command granted for the current batch, such as
`printf '%s' '<compact-json>' | novelgen tool patch craft --target character --id "Lin" --apply`,
`printf '%s' '<compact-json>' | novelgen tool patch craft --target item --id "Core" --apply`,
`printf '%s' '<compact-json>' | novelgen tool patch craft --target location --id "Mine" --apply`, or
`printf '%s' '<compact-json>' | novelgen tool patch craft --target organization --id "Guild" --apply`.
Commands for other names or craft target types remain denied. Go then reloads
the craft file and verifies the requested objects were written:

```bash
novelgen craft gen --agent-sdk --agent-apply --chapter P1-V1-C1
novelgen craft improve --agent-sdk --agent-apply --type items --batch 1
novelgen craft improve --agent-sdk --agent-apply --type characters --name "李侑"
```

This does not give the agent general file access. The Claude SDK runner only
allows `--apply` when the per-call allowlist explicitly grants the name-scoped
patch command, and direct shell writes remain denied.
For `craft improve --agent-sdk`, Go still decides which saved objects are in
scope, verifies that requested objects come back or were applied, and handles
ordinary saves/reloads; the agent only performs focused query/check/patch work.

## Agent SDK Chapter Generation And Improvement

`write gen --agent-sdk` routes final chapter prose generation through the Agent
SDK. The agent receives the normal typed write input, may use
`novelgen tool query ...` and `novelgen tool check ...`, and should start from:

```bash
novelgen tool query context --type chapter-write --id P1-V1-C1 --view brief --fields path,navigation,stats,warnings
```

The typed write input already contains the chapter facts, recap, adjacent
context, and next-chapter hook; the small tool query is only a project-state
sanity check. The workflow does not grant patch tools. The agent returns typed
`{"content": "..."}` JSON only; Go still validates that content is prose,
rejects empty/JSON-as-prose/fenced output, severe length shortfalls, and severe
Agent SDK length overshoots, saves `chapters/chapter-<chapter_id>.md`, and
optionally extracts recap afterward.

`write review --agent-sdk` routes final chapter review through the Agent SDK
`write-review-workflow`. Go still selects chapters, loads final markdown, saves
`story/reviews/<chapter_id>.json`, updates volume review compatibility files,
and applies deterministic humanization/continuity checks. The review agent may
query focused `chapter-write` / `chapter-repair` context, run scoped chapter or
outline checks, and refresh stale chapter DSL when a simulation issue explicitly
points to `refresh_query`; it is not granted patch tools and cannot write files.
The SDK review output schema is compact: score, summary, strengths, weaknesses,
and suggestions only. Go expands that into the normal `ReviewResult` persistence
format.

Write Agent SDK workflows use the compact `novel-tools-core` skill plus the
stage workflow skill. The full `novel-tools` manual is kept for broader/manual
reference, but write generation/review/improvement should rely on typed input
and focused context tools instead of startup prompt text.

`write pipeline --agent-sdk` applies the same Agent SDK contracts to generation,
review, and improvement. Go still owns chapter selection, review persistence,
deterministic fixers, checkpoint-like saves, recap extraction validation, RPG DSL
refresh, and final markdown writes unless `--agent-apply` is enabled for the
validated improvement patch loop.

`write improve --agent-sdk` keeps the existing review, chapter selection,
deterministic fixer, and save flow, but routes each suggestion-based prose
rewrite through the Agent SDK `write-improve-workflow`. The agent gets the
current draft and review suggestions, should use `chapter-repair` for
check-driven repairs or `chapter-write` for ordinary rewrites, may run scoped
checks, and may use `tool patch chapter` as a dry-run validator for the
complete `content` it is about to return. Focused repairs are length-preserving
by default: unless the issue explicitly says the chapter is too short, Go gives
the Agent SDK workflow the current chapter length as the repair target so a
one-paragraph fix does not become a full rewrite.

For system-log or information-advantage (信息差) stories, deterministic chapter checks
treat actionable knowledge as protagonist growth (主角成长). A saved chapter can satisfy growth by
showing the protagonist reading a log, acquiring a clue, forming an executable
judgment, or turning information asymmetry into an action plan; it does not need
to add a cultivation breakthrough, item, or ally just to pass the growth gate.

Add `--agent-apply` when you want the agent to own the validated write loop.
In this mode the workflow may repeat a successful chapter dry-run with
`--apply --refresh-derived`, then inspect the returned post-refresh check or run
the focused check command for the patched target, for example
`novelgen tool check all --target chapter --scope chapter --id P1-V1-C1`.
Long chapter edits use the scoped buffer id `<chapter_id>-draft`; after a
successful dry-run, the runner rejects further `patch-buffer clear/append` for
that target so the agent must apply the validated patch or return final JSON.
Do not run standalone `tool refresh chapter-dsl` after an apply that already
used `--refresh-derived`; reserve standalone refresh for tool-returned
`next_actions` or workflows that cannot use `--refresh-derived`. The runner
hook also reminds the agent of the exact post-patch check command. Go reloads
the saved chapter, keeps the deterministic fixer/review flow, and still runs
post-save checks.

```bash
novelgen write gen --agent-sdk --chapter P1-V1-C1
novelgen write gen --agent-sdk --chapter P1-V1-C1 --recap-agent-sdk
novelgen write review --agent-sdk --chapter P1-V1-C1
novelgen write improve --agent-sdk --chapter P1-V1-C1 --max-rounds 1
novelgen write improve --agent-sdk --agent-apply --chapter P1-V1-C1 --max-rounds 1
novelgen write pipeline --agent-sdk --chapter P1-V1-C1 --max-rounds 1
```

## Agent SDK Polish

`polish --agent-sdk` keeps polish as a Go-owned volume orchestration command:
Go still selects volumes/chapters, performs the volume-level review, gathers
RPG DSL issues, saves chapters, refreshes recaps, and runs post-save checks.
The per-chapter rewrite step is routed through the same Agent SDK
`write-improve-workflow` used by `write improve --agent-sdk`, so the agent gets
focused chapter query/check/patch tools instead of the full project context.

Add `--agent-apply` to let the agent repeat a validated `tool patch chapter`
dry-run with `--apply`. Go then reloads the saved chapter and continues the
normal polish validation and recap flow. `polish --agent-sdk` also routes recap
refresh through the recap Agent SDK by default; `--recap-agent-sdk` is available
for explicit recap-only control when not using chapter Agent SDK.

```bash
novelgen polish --volume 1 --agent-sdk
novelgen polish --volume 1 --agent-sdk --agent-apply
novelgen polish --volume 1 --recap-agent-sdk
```

## Agent SDK Recap Extraction

`recap gen --agent-sdk` runs recap extraction through the Agent SDK. In ordinary
SDK mode, Go provides the exact chapter text, validates the returned typed
`ChapterRecap`, and saves it:

```bash
novelgen recap gen --agent-sdk --chapter P1-V1-C1
novelgen recap gen --agent-sdk --all
novelgen write gen --recap-agent-sdk --chapter P1-V1-C1
novelgen write pipeline --recap-agent-sdk --chapter P1-V1-C1
```

With `--agent-apply`, recap extraction becomes a validated edit loop. The agent
must use the current chapter-scoped `recap-repair` context, run the focused recap
quality check, dry-run `printf '%s' '<compact-json>' | novelgen tool patch recap --id <chapter_id>`,
then repeat the same stdin-piped patch with `--apply` before the command treats
the recap as saved:

```bash
novelgen recap gen --agent-sdk --agent-apply --chapter P1-V1-C1
```

In apply mode, if the agent only returns JSON and does not change the saved recap
through the patch tool, Go leaves the saved recap unchanged. When a patch is
applied, Go reloads the saved patch result and does not rewrite it through the
ordinary save path. The patch tool validates the typed recap, checkpoints the
previous file, and rejects blocking minimal-gate failures.

If the final recap still fails the minimal gate, Go rejects it instead of saving
it. Consistency warnings may still be saved because they are fuzzy continuity
heuristics rather than hard schema requirements.
For write commands, `--recap-agent-sdk` affects only the automatic recap
extraction step after final chapter content exists; chapter generation, review,
improvement, markdown writes, and RPG DSL export keep their existing code paths.
After saving or regenerating a recap, use:

```bash
novelgen tool check quality --target recap --scope chapter --id P1-V1-C1
```

For small recap repairs identified by that check, use the same patch loop:
dry-run first, apply the exact same patch only when validation passes, then run
the focused recap check again.

## Agent SDK Command Coverage

Use Agent SDK for commands where an agent needs project facts, can benefit from
focused query/check tools, and returns or applies a typed story-state patch.
Pure transforms such as translation may also use Agent SDK, but should expose no
project tools and should keep file writes in Go.

## Project Sandboxes

Before long Agent SDK creative runs, clone the source book into a sandbox:

```bash
novelgen project clone ../system-log-agent-run --source books/system-log --with-logs --name "System Log Agent Run"
```

Use `--with-logs` when the agent run should preserve prior prompt/response/live
history for debugging or creative continuation. Omit it for a lighter sandbox.
The clone command refuses to overwrite existing directories and refuses targets
inside the source project, so iterative agent runs do not accidentally mutate the
original book or recursively copy themselves. It always skips the source
project's `.novelgen/` temporary workspace, including stale patch buffers, so a
new sandbox starts with clean agent patch state even when logs are copied.

If the cloned project name needs cleanup after copying, run:

```bash
cd ../system-log-agent-run
novelgen project rename "System Log Agent Run"
```

`project rename` updates only `novel.json.name`; it does not modify setup,
outline, craft, chapters, recaps, or logs.

Before starting the Agent SDK run, verify the sandbox can support creative work:

```bash
novelgen project doctor --json
```

`novelgen project doctor --json` is read-only. It checks `novel.json`, story
setup, outline shape, queryable logs with per-kind counts (`logs_prompts`,
`logs_responses`, `logs_agent_live`), clone metadata when present, Agent SDK
runtime config, Python availability, `claude_agent_sdk` importability, and
embedded workflow skills. It also reports the model that Agent SDK calls will
request per call; the runtime model is only a fallback when a project does not
set one. A missing outline or empty log history is a warning; unreadable project
config/setup, missing project model, or an unavailable Agent SDK runtime is a
blocking error.

To continue writing from copied prompt/response/agent-live history, opt in with
`--agent-history` on Agent SDK write commands. This makes the agent inspect the
bounded log index before writing, while Go still validates and saves output:

```bash
novelgen write gen --agent-sdk --agent-history --chapter P1-V1-C2
novelgen write pipeline --agent-sdk --agent-history --agent-apply --chapter P1-V1-C2
```

`--agent-history` requires `--agent-sdk`. Without it, write agents keep the
smaller default context and do not query logs.

Current Agent SDK command surface:

| Command | Agent SDK role | Writer |
| --- | --- | --- |
| `compose gen --agent-sdk` | Generate skeleton/volume JSON with project queries | Go saves outline |
| `compose improve/pipeline --agent-sdk` | Review/repair selected outline volumes with query/check/patch tools | Go saves, or patch tool with `--agent-apply` |
| `setup improve/regen --agent-sdk` | Review or regenerate setup through minimal setup patches | Go saves, or patch tool with `--agent-apply` |
| `craft gen/improve --agent-sdk` | Generate/repair scoped craft objects from focused context | Go saves, or patch tool with `--agent-apply` |
| `write gen/review/improve/pipeline --agent-sdk` | Generate/review/repair final chapter prose with focused chapter tools | Go saves, or chapter patch tool with `--agent-apply` for improve/pipeline |
| `polish --agent-sdk` | Orchestrate per-chapter focused repair through write workflow | Go saves, or chapter patch tool with `--agent-apply` |
| `recap gen --agent-sdk` | Extract/repair typed recap JSON from chapter text | Go saves, or recap patch tool with `--agent-apply` |
| `translate --agent-sdk` | Translate provided file content through SDK JSON output with no project tools | Go writes output |

Do not add Agent SDK mode just because a command calls an LLM. Keep these
commands Go-owned unless their contract changes:

| Command area | Reason |
| --- | --- |
| `draft` | Legacy optional workflow; new work should use `write pipeline --agent-sdk` |
| `compose regen`, `skeleton-review`, `skeleton-improve` | Narrow legacy/specialized compose paths; route future repair through volume improve/pipeline |
| `rpg`, `rpg-dsl`, `simulate-dsl`, `tool refresh chapter-dsl` | Deterministic or conversion/export infrastructure; Go owns files and validation |
| `export`, `init`, `config`, `tool query/check/patch` | CLI infrastructure or deterministic project I/O |

When adding a new Agent SDK mode, first add or reuse a focused `tool query
context ...` route, a scoped `tool check ...`, and a patch contract if the
agent may write. If the command cannot define those three pieces, keep the
agent read-only and let Go remain the writer.

## Writer Ownership

Agent patch tools do not remove the need for the Go backend. Treat them as a
validated editing surface, not as general file ownership.

Use agent patch apply only when all of these are true:

- The state object already has a focused patch contract, such as setup, outline
  volume, or one craft element.
- The tool can preserve IDs, normalize typed models, checkpoint, and run scoped
  quality/schema/simulation checks before writing.
- The patch contract can express small typed edits through merge semantics
  without requiring the agent to rewrite unrelated fields.
- Go can reload the saved state and verify the requested target changed.

Keep Go as the only writer when the stage is a pure extraction or compilation
step, such as recap extraction, DSL conversion, markdown export, or cache
generation. In those stages the agent should return typed JSON or DSL text and
Go should parse, validate, and save.

## Agent Skill Guide

The Claude/agent-facing Chinese usage guide is stored at:

```text
internal/agentruntime/skills/novel-tools/SKILL.md
```

Configure that guide as an SDK skill when you want the Claude Agent SDK runtime
to know when and how to call the query tools.

