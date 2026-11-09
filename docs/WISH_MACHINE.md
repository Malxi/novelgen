# 许愿机 · wish-machine

`wish-machine` is the native agent marketplace for Novelgen. It packages four
focused skills, four thin commands, and the `novelgen` MCP server.

## Layout

```text
wish-machine/
  .claude-plugin/marketplace.json
  .codebuddy-plugin/marketplace.json
  plugins/wish/
    .claude-plugin/plugin.json
    .codebuddy-plugin/plugin.json
    .mcp.json
    commands/{book,check,ideas,gen}.md
    skills/{wish-book,wish-check,wish-ideas,wish-gen}/SKILL.md
    references/
    hooks/hooks.json
```

The command files are routing entries only. The workflow lives in `SKILL.md`;
schemas and examples in `references/` are read through `book_guide`. Key
references: `setup-schema`, `outline-schema`, `book-write`, `check-profiles`.

## Commands and skills

```text
/wish:book    → wish-book    book_init/status/read/write/refresh
/wish:check   → wish-check   book_check findings
/wish:ideas   → wish-ideas   book_ideas draws
/wish:gen     → wish-gen     book_guide workflow/skill/reference
```

## MCP tools

```text
book_init
book_status
book_read
book_write      mode=patch|replace, dry run unless apply=true
book_refresh
book_check      profile=quality|schema|rpg|custom|all
book_ideas
book_guide      kind=workflow|skill|reference
```

Legacy `novelgen_*` tool names remain advertised and accepted as aliases for
the migration window; new skills and docs use `book_*`.

## Install

Claude Code:

```text
/plugin marketplace add Malxi/novelgen
/plugin install wish@wish-machine
```

WorkBuddy:

```text
Plugin Management → Add Marketplace
Source: Malxi/novelgen
Install: wish
```

The root `.codebuddy-plugin/marketplace.json` and
`.claude-plugin/marketplace.json` point at `./wish-machine/plugins/wish`, so
the GitHub repository itself is the marketplace. For a local checkout, use
`D:\Code\nolvegen` as the marketplace source.

Install `novelgen` on `PATH` first; the plugin's `.mcp.json` and hooks invoke
`novelgen mcp serve --global` and `novelgen hook ...`. Restart WorkBuddy after
installing the plugin so skills, commands, MCP and hooks are loaded.

Codex has no marketplace. Copy `plugins/wish/skills/*` to `~/.codex/skills/` and
register the MCP server in `~/.codex/config.toml`.

DSH uses the separate `dsh-novelgen` Cordis bundle.

## Custom rules

`check --profile custom` evaluates `.novelgen/rules/*.json` for `setup`,
`outline`, or `chapter`:

```json
{"id":"no-deus-ex","target":"chapter","severity":"high","kind":"regex","pattern":"突然.*醒来","message":"possible deus ex machina"}
```

`kind` is `regex` or `contains`; rules may be a single object or an array.

## Official WorkBuddy platform

For the public WorkBuddy market, use the CLI + Skill Connector package in
[`open-platform/novelgen-connector`](../open-platform/novelgen-connector) and
follow [OPEN_PLATFORM.md](OPEN_PLATFORM.md). The marketplace in this repository
is for local/team GitHub installation, not the official public listing.

## CLI mapping

```text
novelgen book init|status|read|write|refresh
novelgen check [--profile ...]
novelgen ideas draw|types
novelgen gen workflow|skill|reference list|read
```

The older `novelgen tool query|check|patch|replace|refresh` commands remain as
compatibility aliases.
