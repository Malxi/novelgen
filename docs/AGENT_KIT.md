# Agent Kit

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
| Adapters | `novelgen agent install` | Claude/Codex/DSH files, MCP config, skills, hooks |

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

# Codex reads the user config, not a project .codex/config.toml
novelgen agent install --target codex --apply --codex-home "$HOME/.codex"
```

The installer writes:

```text
.novelgen/workflows/                 # shared stage instructions
.novelgen/adapters/claude/           # local marketplace + plugin
.novelgen/adapters/codex/            # Codex kit
.codex/config.toml                   # MCP server (only if absent, unless --force)
.codex/skills/novelgen/SKILL.md
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

## MCP tools

`novelgen mcp serve` (stdio, newline-delimited JSON-RPC) exposes:

- `novelgen_status`
- `novelgen_query`
- `novelgen_check`
- `novelgen_patch`
- `novelgen_replace` (dry run unless `apply: true`)
- `novelgen_refresh`

The server is local to one project (`--dir` or cwd) and opens no network port.
It shells out to the same `novelgen` binary for `query/check/patch/replace/refresh`
so adapters and the CLI cannot drift into different validation semantics.

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
novelgen hook guard-write < hook.json
novelgen mcp serve < requests.jsonl
```

The focused tests cover adapter installation (including rendered `.mcp.json` and
hook config), managed `AGENTS.md` updates, dry-run behavior, hook session status,
MCP initialize/list/status, `tool replace` dry-run/removal warnings, and the
`tool replace` recap id check.
