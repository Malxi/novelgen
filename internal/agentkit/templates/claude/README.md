# Novelgen adapter (Claude Code)

This directory is a local Claude Code marketplace with one plugin.

```bash
# In Claude Code, from the project root:
/plugin marketplace add .novelgen/adapters/claude
/plugin install novelgen@novelgen-local
```

Or load it directly for a session:

```bash
claude --plugin-dir .novelgen/adapters/claude/plugin
```

The plugin provides:

- `/novelgen-status`
- `/novelgen-next`
- `/novelgen-write <chapter-id>`
- the `novelgen` skill
- `PreToolUse` write guard and `SessionStart` status hook
- an MCP server entry for `novelgen mcp serve`

Requirements:

- `novelgen` must be on `PATH`.
- Hooks execute `novelgen hook ...`; install them only if you trust the binary.
- The MCP server is local stdio only; it does not open a network port.
