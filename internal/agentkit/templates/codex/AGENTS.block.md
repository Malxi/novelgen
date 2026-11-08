## Novelgen

This project is a Novelgen project. Read `.novelgen/workflows/*.md` before
working on setup, outline, chapters, review, or refresh.

Hard rules:

- Do not edit `story/`, `chapters/`, or `novel.json` directly.
- Use the `novelgen` CLI as the only write path:
  - complete artifact: `novelgen tool replace ... --apply`
  - incremental repair: `novelgen tool patch ...`
  - derived refresh: `novelgen tool refresh ...`
- Query first: `novelgen tool query ...`; check before claiming completion:
  `novelgen tool check all --target ...`.
- Use canonical character names from the outline in chapter prose.
- Use `novelgen tool query ideas --type namecard` for character names instead
  of inventing generic names.

The local MCP server is `novelgen mcp serve` (configured in
`.codex/config.toml` when available). It exposes project query/check/replace/
patch/refresh tools.
