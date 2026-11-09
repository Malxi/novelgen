# Agent Kit

The primary integration is now the native marketplace in
[`wish-machine`](WISH_MACHINE.md): four skills (`wish-book`, `wish-check`,
`wish-ideas`, `wish-gen`), four thin commands, and the `book_*` MCP tools.
The project-local adapter described below remains for Codex/DSH and as a
compatibility fallback.

Novelgen is the project system; the agent harness is replaceable. The agent kit
keeps the portable parts in the CLI and puts harness behavior in thin adapters.

## Layers

| Layer | Owner | Responsibility |
| --- | --- | --- |
| Project state | `novelgen` Go core | schema, mutation service, checkpoints, checks, refresh |
| Tool plane | `novelgen tool query/check/patch/replace/refresh` | deterministic project access |
| MCP | `novelgen mcp serve` | local stdio tools for Claude/Codex/DSH |
| Workflows | `.novelgen/workflows/*.md` | stage instructions shared by adapters |
| Hooks | `novelgen hook ...` | guard direct writes and print session context |
| Adapters | `novelgen agent install` | Claude/Codex/WorkBuddy/DSH files, MCP config, skills, hooks |

The CLI is the only write path. An adapter must not write `story/`, `chapters/`
or `novel.json` directly; the `guard-write` hook enforces that when the harness
supports hooks.

## Install

From a Novelgen project root:

```bash
# Preview files; no writes
novelgen agent install --target all

# Apply Claude + Codex
novelgen agent install --target all --apply

# Install Claude hooks too; requires trust in the local novelgen binary
novelgen agent install --target claude --apply --allow-hooks

# WorkBuddy (CodeBuddy): project MCP server + CODEBUDDY.md + project skill
novelgen agent install --target workbuddy --apply

# Codex reads the user config, not a project .codex/config.toml
novelgen agent install --target codex --apply --codex-home "$HOME/.codex"
```

`--target all` installs Claude and Codex; pass `workbuddy`/`dsh` explicitly.

The installer writes:

```text
.novelgen/workflows/                 # shared stage instructions
.novelgen/adapters/claude/           # local marketplace + plugin
.novelgen/adapters/codex/            # Codex kit
.codex/config.toml                   # MCP server (only if absent, unless --force)
.codex/skills/novelgen/SKILL.md
.mcp.json                            # WorkBuddy project MCP server (merged)
.codebuddy/skills/novelgen/SKILL.md  # WorkBuddy project skill
CODEBUDDY.md                         # managed WorkBuddy block (or existing guidance file)
AGENTS.md                            # managed novelgen block
```

Use `--force` to overwrite an existing adapter file. Existing `.codex/config.toml`
is preserved by default so a user's MCP configuration is not clobbered.

## Claude Code

```text
/plugin marketplace add .novelgen/adapters/claude
/plugin install novelgen@novelgen-local
```

Or run with `claude --plugin-dir .novelgen/adapters/claude/plugin`.

The plugin supplies:

- `/novelgen-status`, `/novelgen-next`, `/novelgen-write`
- `novelgen` skill
- `PreToolUse` write guard
- `SessionStart` status context
- `.mcp.json` with `novelgen mcp serve`

Hooks run the absolute `novelgen` path recorded at install time; `novelgen` must
exist at that path. Hooks are skipped unless `--allow-hooks` is passed.

## Codex

Codex uses an agent kit rather than Claude's plugin marketplace:

- `AGENTS.md` managed block
- `.codex/config.toml` as a project reference
- `.codex/skills/novelgen/SKILL.md`
- `.novelgen/workflows/*.md`

Codex CLI reads `$CODEX_HOME/config.toml` (usually `~/.codex/config.toml`), not
a project `.codex/config.toml`. Pass `--codex-home` to write the MCP table, or
run:

```bash
codex mcp add novelgen -- /absolute/path/to/novelgen mcp serve
```

Restart Codex in the project after install. If Codex does not support project
hooks in the installed version, the `guard-write` rule is still stated in
`AGENTS.md`; the Go mutation service remains the final validation layer.

## WorkBuddy (CodeBuddy)

WorkBuddy can use Novelgen in two ways.

### WorkBuddy-first: one global MCP server

This is the mode where WorkBuddy creates the book. In WorkBuddy:
**Settings → MCP Servers → Add MCP**:

- Name: `novelgen`
- Transport: `stdio`
- Command: `D:\Code\nolvegen\bin\novelgen.exe`
- Arguments (one per row):
  - `mcp`
  - `serve`
  - `--global`
  - `--workspace`
  - `<the folder you open in WorkBuddy, e.g. D:\Code\AI\books>`
- Working directory: optional; leave empty to use the session workspace.

Then ask WorkBuddy to call `novelgen_init`:

```text
name = "我的第一本书"
projectDir = "my-first-novel"   # relative to --workspace
genre = "都市"
parts = 1
volumes = 3
chapters = 5
language = "zh"
```

`chapters` is chapters per volume. The resulting project is
`1 part × 3 volumes × 5 chapters = 15 chapters`.

`novelgen_init` creates the directory and `novel.json`, writes the WorkBuddy
guidance (`CODEBUDDY.md`, `.codebuddy/skills`, `.novelgen/workflows`), and
returns `projectDir`. It deliberately does not add a project `.mcp.json`: the
user-level global `novelgen` server remains authoritative. Pass that
`projectDir` to every other Novelgen tool (status, query, check, patch, replace,
refresh). `novelgen_init` only creates projects inside `--workspace`; an existing
project may be passed as an absolute `projectDir` even when it lives outside the
workspace (useful after changing the WorkBuddy workspace).

### Project-local adapter

For an existing project, the installer writes project configuration:

- `.mcp.json` in the project root: the local `novelgen mcp serve` stdio server
- `CODEBUDDY.md` (or `.codebuddy/CODEBUDDY.md`, or an existing `AGENTS.md`):
  managed Novelgen rules
- `.codebuddy/skills/novelgen/SKILL.md`: WorkBuddy project skill
- `.novelgen/workflows/*.md`: shared stage instructions

```bash
cd /path/to/novel-project
novelgen agent install --target workbuddy --apply --user-skill
```

`--user-skill` also writes the skill to `~/.workbuddy/skills/novelgen/SKILL.md`
and `~/.codebuddy/skills/novelgen/SKILL.md`, so WorkBuddy can load it at session
start even for a project that `novelgen_init` creates mid-session.

The generated MCP entry uses WorkBuddy's `${CODEBUDDY_PROJECT_DIR}` so the same
file works whether WorkBuddy opens the project root or a subdirectory, and the
existing `.mcp.json` servers are preserved. Open the project in WorkBuddy,
switch to the agent/craft mode, approve the `novelgen` MCP server when prompted,
and start with a request such as "use novelgen_status and continue this novel".
WorkBuddy exposes the tools as `mcp__novelgen__novelgen_status`,
`mcp__novelgen__novelgen_query`, and so on.

**WorkBuddy's token:** WorkBuddy's model generates the setup, outline and prose;
the Novelgen MCP server only reads context, runs deterministic checks, and
writes validated artifacts. Do not run `novelgen setup gen`,
`novelgen compose pipeline`, or `novelgen write pipeline` in this setup — those
call Novelgen's own LLM configuration and spend its token budget instead. Use
the dry-run/apply loop: call `novelgen_replace` or `novelgen_patch` with
`apply: false`, repair blocking issues, then repeat with `apply: true`.
`novelgen_refresh` with `chapter-dsl` is the one MCP tool that calls Novelgen's
LLM converter; skip it in a WorkBuddy-token setup unless the user explicitly
asks and has a Novelgen token configured.

## MCP tools

`novelgen mcp serve` (stdio, newline-delimited JSON-RPC) exposes:

- `novelgen_init` (global mode only)
- `novelgen_status`
- `novelgen_query`
- `novelgen_check`
- `novelgen_patch`
- `novelgen_replace` (dry run unless `apply: true`)
- `novelgen_refresh` (chapter-dsl uses Novelgen's LLM)

Without `--global` the server is local to one project (`--dir` or cwd). With
`--global --workspace <dir>` it serves a workspace: `novelgen_init` creates a
project under that workspace and every other tool accepts `projectDir`.
It opens no network port and shells out to the same `novelgen` binary for
`init/query/check/patch/replace/refresh`, so adapters and the CLI cannot drift
into different validation semantics.

## Hooks

```bash
novelgen hook guard-write    # reads Claude PreToolUse JSON on stdin
novelgen hook session-start  # reads a new session and prints context
```

`guard-write` denies `Write/Edit/MultiEdit` under `story/`, `chapters/`, or
`novel.json`. It prints a `PermissionDecision` JSON and allows other paths.
Escape hatch for a deliberate migration:

```bash
NOVELGEN_ALLOW_DIRECT_WRITE=1
```

## DSH

The DSH studio remains a Cordis bundle (`dsh-novelgen`) because it owns
`conversation.view`, previews, the resource graph, and the selection tray. The
shared workflows and CLI/MCP tools are still available to it; the DSH UI is not
portable to Claude/Codex.

## Verification

```bash
go test ./...
go build -o bin/novelgen.exe .
novelgen agent install --target all --apply --allow-hooks
novelgen agent install --target workbuddy --apply
novelgen hook guard-write < hook.json
novelgen mcp serve < requests.jsonl
novelgen mcp serve --global --workspace D:\Code\nolvegen\books < requests.jsonl
```

The focused tests cover adapter installation (including rendered `.mcp.json`,
hook config and WorkBuddy MCP merge), managed `AGENTS.md` updates, dry-run
behavior, hook session status, MCP initialize/list/status, `tool replace`
dry-run/removal warnings, and the `tool replace` recap id check.
