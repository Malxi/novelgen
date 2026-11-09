## Novelgen (MCP tools)

This project is a Novelgen project. WorkBuddy owns the model/token; Novelgen
owns project state. Use the `novelgen` MCP server and do not edit `story/`,
`chapters/`, `drafts/`, or `novel.json` directly.

Read the full workflow at `.codebuddy/skills/novelgen/SKILL.md` (also installed
at `~/.codebuddy/skills/novelgen/SKILL.md`) before the first action.

- MCP tools (WorkBuddy exposes them as `mcp__novelgen__<name>`):
  - `novelgen_status` — progress and the next step; call it first.
  - `novelgen_query` — read setup, outline, craft, chapter, context, ideas, logs.
  - `novelgen_check` — deterministic quality/simulation checks.
  - `novelgen_replace` — replace a complete setup/outline/chapter/recap artifact.
  - `novelgen_patch` — incremental repair of setup/outline/craft/chapter/recap.
  - `novelgen_refresh` — rebuild derived chapter DSL or narrative memory.
- `novelgen_replace` and `novelgen_patch` are dry runs by default. Review the
  dry-run result, fix blocking issues, then repeat the same call with
  `apply: true`.
- A complete setup uses `novelgen_replace`; a complete outline uses
  `novelgen_replace`; a targeted volume/chapter outline change uses
  `novelgen_patch`; a whole chapter uses `novelgen_replace`; an incremental
  paragraph edit uses `novelgen_patch`.
- After writing, run `novelgen_check` with `minPriority: "high"` and
  `maxIssues: 12`. Derived refresh is optional: `novelgen_refresh` with
  `chapter-dsl` calls Novelgen's LLM converter, so do not run it in a
  WorkBuddy-token setup unless the user explicitly asks.
- Keep context small: prefer `novelgen_query` with `view: "index"` or
  `view: "brief"` and a `limit`.
- Do not run `novelgen setup gen`, `novelgen compose pipeline`,
  `novelgen write pipeline`, or other stage commands: they use Novelgen's own
  LLM configuration. This project uses WorkBuddy's model instead.

Read `.novelgen/workflows/*.md` for the detailed stage instructions and use
canonical character names from the outline in chapter prose.
