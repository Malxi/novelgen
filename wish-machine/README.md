# 许愿机 · wish-machine

A local agent marketplace for Novelgen.

One plugin (`wish`) provides four skills, four commands and the local MCP
server:

| Entry | Purpose |
| --- | --- |
| `wish-book` / `/wish:book` | manage a book: status, read, write, refresh, init |
| `wish-check` / `/wish:check` | deterministic checks and findings |
| `wish-ideas` / `/wish:ideas` | deterministic content reference draws |
| `wish-gen` / `/wish:gen` | workflows, skills and references for generation |

The MCP server exposes `book_init`, `book_status`, `book_read`, `book_write`,
`book_refresh`, `book_check`, `book_ideas` and `book_guide`.

## Requirements

- `novelgen` must be on `PATH` (the plugin runs `novelgen mcp serve --global`
  and `novelgen hook ...`). Install the CLI or add its directory to `PATH`.
- A harness with plugin marketplace support (Claude Code or WorkBuddy).

## Claude Code

```text
/plugin marketplace add Malxi/novelgen
/plugin install wish@wish-machine
```

## WorkBuddy

Plugin Management → Add Marketplace → Marketplace Source:

```text
Malxi/novelgen
```

or use the local checkout path:

```text
D:\Code\nolvegen
```

Then install `wish`.

WorkBuddy loads both `.codebuddy-plugin/marketplace.json` and the plugin's
`.codebuddy-plugin/plugin.json`, `skills/`, `commands/`, `.mcp.json` and
`hooks/hooks.json`.

## Codex

Codex does not have a plugin marketplace. Copy the four `skills/*/SKILL.md`
directories to `~/.codex/skills/` and keep the MCP server in
`~/.codex/config.toml`:

```toml
[mcp_servers.novelgen]
command = "novelgen"
args = ["mcp", "serve", "--global"]
```

## DSH

DSH uses the `dsh-novelgen` Cordis bundle instead of this marketplace.
