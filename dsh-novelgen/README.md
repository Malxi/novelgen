# Novelgen Studio for DeepSeek Harness

A DSH bundle that turns a [Novelgen](../../README.md) project into a subagent-driven
creation workflow and a visual workbench beside the conversation composer.

It has two halves:

- **Host half** (`index.js`, `lib/`) exposes six `novelgen_*` agent tools, a
  same-origin HTTP bridge for the UI, and a per-session project selection.
- **Client half** (`client.js`) registers the full **Novel** view in
  `conversation.view`: creative flow, resources, relationship graph, and a
  selection tray that inserts a ready-to-use "select prompt" into the main
  conversation input.

## Requirements

- A DSH profile with `tools`, `webServer`, `agents`, and `workingDirectory`
  services (the Desktop/Web profile has them).
- A Novelgen project: a directory containing `novel.json`.
- The `novelgen` executable. The plugin auto-detects, in order:
  1. `novelgenPath` from its config,
  2. `NOVELGEN_BIN`,
  3. `bin/novelgen.exe` or `bin/novelgen` in the project or an ancestor,
  4. `bin/novelgen.exe` in the process working directory,
  5. `novelgen` / `novelgen.exe` on `PATH`.

## Install

From the repository that contains this directory:

```text
plugin_manager install_bundle target=<absolute path to dsh-novelgen>
```

Then reload the DSH Web page. The bundle inserts one host row (`novelgen`) and
loads the client module `@local/dsh-novelgen`.

## Configuration

The bundle patch ships these defaults; edit the `novelgen` row in the profile's
`cordis.patch.yml` (or through the plugin configuration page when available):

```yaml
- id: novelgen
  config:
    projectRoot: ''      # absolute project dir, or empty for auto-detection
    projectRoots: []     # dirs whose children are novel projects
    novelgenPath: ''     # absolute path to the novelgen executable
```

The UI also has a project picker. A picked project is remembered per DSH
session in `~/.dsh/novelgen/sessions.json` (best effort; read-only homes fall back
to auto-detection).

## Agent workflow

The main conversation owns planning, user requests, and long-range continuation.
Concrete creation is delegated to a DSH subagent.

1. `novelgen_status`
   - Returns project name, stage progress, resource counts, and the single best
     next step (`next.prepareArguments`).
2. `novelgen_prepare`
   - Builds a self-contained task packet under
     `<session cwd>/.dsh-novelgen/tasks/<id>/` (`task.md`, `context.json`).
   - Returns `delegate.prompt` plus the exact `novelgen_apply` arguments.
   - For a missing outline skeleton or an empty volume it returns
     `mode: "direct"` with a `novelgen compose pipeline` command instead: section
     IDs and structural validation belong to Novelgen's Go pipeline.
3. `subagent`
   - Pass `delegate.prompt` verbatim. The subagent reads the task packet, follows
     the named Novelgen skill (`novelgen tool skill read ...`), and returns only a
     JSON object. It is told not to write files or run patch commands.
4. `novelgen_apply`
   - Runs `novelgen tool patch <section>` as a dry-run first. If the check is
     non-blocking it repeats the identical patch with `--apply`
     (and `--refresh-derived` for chapters), then returns compact changes,
     checks, and the next step.
5. Repeat `status -> prepare -> subagent -> apply`.
   - For a long-running objective, record it with `create_goal` or
     `schedule_create` so the main conversation keeps driving the loop.

### Supported stages

| Stage | Prepare skill | Apply |
| --- | --- | --- |
| `setup` | `setup-generate-workflow` / `setup-improve-workflow` | `tool patch setup` |
| `craft` | `craft-character-workflow` / `craft-element-workflow` | `tool patch craft --target ... --id ...` |
| `outline` | `outline-improve-volume-workflow` | `tool patch outline --target volume/chapter --id ...` |
| `write` | `write-chapter-workflow` / `write-improve-workflow` | `tool patch chapter --id ... --refresh-derived` |
| `recap` | `recap-extract-workflow` (optional) | `tool patch recap --id ...` |

Selecting a part in the UI prepares its first unfinished volume, because
Novelgen patches volumes and chapters rather than parts. Selecting a part in
the agent tools does the same and reports the resulting volume id.

`recap` remains an auxiliary CLI surface (`novelgen recap gen`) and an optional
manual task, but it is **not** a primary stage in `novelgen_status.next`: the
recommended `write pipeline` auto-extracts recaps, so the studio no longer asks
for a separate recap pass. Resources under `story/recaps/` still preview and can
be selected manually.

A complete outline can be committed through `novelgen tool replace outline --apply`
(the `novelgen_apply` call without `target`/`id`); a targeted volume/chapter
improvement uses `tool patch outline --target ... --id ...`. `novelgen_prepare`
still uses the direct compose pipeline when the skeleton is missing, because
`tool patch outline` intentionally cannot add chapters.

### Tools

| Tool | Purpose |
| --- | --- |
| `novelgen_status` | Progress, counts, stages, next step. |
| `novelgen_resources` | List/search resources; full detail or preview for exact ids. |
| `novelgen_prepare` | Delegation packet, task file, and apply arguments. |
| `novelgen_apply` | Apply a subagent JSON result; setup/full outline use replace, targeted outline uses patch. |
| `novelgen_refresh` | Rebuild derived chapter DSL or narrative memory after writes. |
| `novelgen_select` | Turn resource ids into the same select prompt the UI inserts. |

Chapter apply defaults to no derived-DSL refresh. Use `novelgen_refresh` after
the batch; this keeps a slow/failing derived rebuild from making a saved chapter
look like a failed apply.

All tools accept `projectDir` to override the session project.

## UI

The studio is the full **Novel** view registered in `conversation.view`, beside
the built-in **Chat / Trajectory** tabs. The old composer-dock bar has been
removed: the View tab is the single entry point. On narrow windows the tab can
fold into the View row's More menu; widen the window or use More to reach it.

The full view keeps the Flow, Resources, Graph, and selection tray at page
height:

- **Flow**: setup, outline, craft, and manuscript progress plus the next step.
  Clicking a stage selects a representative resource and shows its structured preview in the right column. "Run next step" submits the delegation
  prompt directly through the composer; "Insert prompt only" leaves it editable.
  A batch control submits "advance N steps" with `create_goal` instructions.
- **Resources**: search and filter by kind, volume, and status (unfinished /
  written / unwritten), with client-side paging. Select with the checkbox, click
  for a structured preview, and use "Run this resource" or "Insert prompt only".
- **Recent changes**: the Flow column lists recently modified setup, outline,
  craft, written chapters, and recaps by file mtime; click one to preview it.
- **Auto refresh**: while the Novel view is active it refreshes every 15 seconds
  and on window focus; the header shows the last refresh time. After a run is
  submitted it refreshes again shortly after.
- **Preview**: setup renders genres, premise, theme, tone, core cast, storylines,
  and rules; outlines render volume/chapter lists with written state; chapters
  render their outline events or saved prose; craft renders background,
  motivation, personality, skills, abilities, and voice; recaps render the last
  line and next opening. Every preview has an expandable **Raw data** section.
- **Graph**: the selected resource at the center, its neighbors around it, and
  edges for contains, features, set-in, uses, advances, next, and recap.
- **Selection tray**: intent, optional note, and "Insert into input" or "Run
  selection". The tray calls the host bridge to build the canonical select prompt
  and either inserts it or submits it through the composer's `InputActions`. If
  insertion or submission is refused, the prompt stays visible for copying.

The run actions use the composer's `InputActions.insertText` and `InputActions.submit`;
when the composer refuses insertion or submission, the generated prompt is kept
visible for copying instead of being lost.

The UI reads `/plugins/novelgen/state|preview|projects|selection|project` on the
same origin. It uses the host theme tokens and locale service; no iframe, no
duplicate React, no DOM outside the slot.

## Project projection

The host projection reads only documented Novelgen layout:

- `novel.json`
- `story/setup/story_setup.json`
- `story/compose/outline.json`
- `story/craft/{characters,locations,items,organizations}.json`
- `chapters/chapter-<id>.md` (and the bare `<id>.md` fallback)
- `story/recaps/<id>.json`

It never writes those files. Every mutation goes through the `novelgen` CLI.
The projection is cached by file mtime/size and invalidated by chapter-file
changes.

## Notes and limits

- The client bundle is plain browser JavaScript loaded by the DSH client module
  loader; it is not an iframe or a second application.
- The UI routes are loopback-only and, when the profile has `ctx.connection`,
  validated with `ctx.connection.requestRejection` (the same browser
  authentication as `/api`). A direct `curl` without the page cookie receives
  401. Do not expose the DSH web listener on an untrusted interface.
- A task packet can fall back to the temp directory or to an inline task body if
  the session workspace is not writable. Inline packets are complete but larger.
- `novelgen compose pipeline` is a long-running Novelgen command; run it as a
  background job and re-check `novelgen_status` afterward.
- Visual rendering was checked with server-rendered component tests for the
  Flow, Resources, and Graph tabs plus live slot registration; final pixel-level
  appearance depends on the installed DSH theme and viewport.
