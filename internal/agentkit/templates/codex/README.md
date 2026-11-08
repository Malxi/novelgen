# Novelgen adapter (Codex agent kit)

Codex does not use the Claude plugin marketplace. This adapter installs a
project-local agent kit:

- `AGENTS.md` managed block with the Novelgen rules
- `.codex/config.toml` with the `novelgen mcp serve` MCP server
- `.codex/skills/novelgen/SKILL.md`
- `.novelgen/workflows/*.md`

Usage:

```bash
novelgen agent install --target codex --apply
```

Then restart Codex in this project. The MCP server is local stdio only.
