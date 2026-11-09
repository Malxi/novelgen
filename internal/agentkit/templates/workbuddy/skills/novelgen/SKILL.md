---
name: novelgen
description: Use when creating, continuing, checking, or repairing a Novelgen novel project through the Novelgen MCP tools. WorkBuddy supplies the prose and token; Novelgen validates and writes project state.
---

# Novelgen

WorkBuddy is the model. Novelgen is the project state and write path. Never edit
`story/`, `chapters/`, `drafts/`, or `novel.json` directly; call the MCP tools.

WorkBuddy exposes this project's server as `mcp__novelgen__*`:

- `mcp__novelgen__novelgen_init` — create a new project (global MCP mode).
- `mcp__novelgen__novelgen_status` — stage, counts, and the single next step.
- `mcp__novelgen__novelgen_query` — read setup/outline/craft/chapter/context.
- `mcp__novelgen__novelgen_check` — scoped deterministic checks.
- `mcp__novelgen__novelgen_replace` — complete setup/outline/chapter/recap.
- `mcp__novelgen__novelgen_patch` — incremental repairs.
- `mcp__novelgen__novelgen_refresh` — derived chapter DSL / narrative memory.
  `chapter-dsl` uses Novelgen's LLM converter, so skip it in a WorkBuddy-token
  setup unless the user explicitly asks and has a Novelgen token configured.

For a new book, call `novelgen_init` with the requested structure, not the CLI:

```text
name="我的第一本书", projectDir="my-first-novel",
parts=1, volumes=3, chapters=5, genre="科幻", language="zh"
```

Then pass the returned `projectDir` to every later tool.

## Mandatory loop

1. Call `novelgen_status` and follow `next.stage` / `next.targetId`.
2. Read the relevant workflow under `.novelgen/workflows/` (`setup.md`,
   `outline.md`, `write.md`, `review.md`, `refresh.md`). Craft elements use the
   craft fields described in the query result and the same patch/replace rules.
3. Query only the context you need with `novelgen_query` (for example
   `section: "context"`, `type: "chapter-write"`, `id: "<chapter-id>"`). Use
   `view: "index"` or `view: "brief"` and `limit` to keep WorkBuddy's context
   small. For name candidates use:

   ```text
   section="ideas", type="namecard", genre="科幻", count=12, seed=42
   ```

   Do not substitute `name="科幻"`; that is not a genre filter.
4. Generate the complete artifact with WorkBuddy's model. Use canonical names
   from the outline; never reuse another project's premise.
5. Write through the tool:
   - complete setup/outline/chapter/recap → `novelgen_replace` with
     `apply: false`, inspect `changed`, `check.blocking`, and `warnings`, then
     repeat with `apply: true`; set `maxIssues: 12` to keep the result small;
   - targeted outline volume/chapter, craft, or paragraph repair →
     `novelgen_patch` the same way;
   - when a whole outline has a few blocking chapters, prefer
     `novelgen_patch section="outline" target="volume" id="P1-V1"` over editing
     the outline JSON outside Novelgen;
   - `novelgen_replace` on a whole outline lists chapters it would remove
     (including saved prose/recap); report that to the user before applying.
6. Run `novelgen_check` on the affected target with
   `minPriority: "high", maxIssues: 12` first; only widen the budget when a
   blocking issue needs more evidence. Repair blocking issues before claiming
   completion. A missing/stale derived DSL is a warning, not a reason to run
   Novelgen's LLM refresh.
7. Derived refresh is optional. `novelgen_refresh` with `chapter-dsl` calls
   Novelgen's LLM; in this WorkBuddy-token setup skip it unless the user
   explicitly asks and has a Novelgen token configured.
8. Continue `status -> query -> generate -> dry-run -> apply -> check` until
   the user's goal is complete or a decision is needed.

## Do not

- Do not shell out to `novelgen setup gen`, `novelgen compose pipeline`,
  `novelgen write pipeline`, or other commands that call an LLM; that would
  spend Novelgen's token budget instead of WorkBuddy's.
- Do not write project files with the editor or shell. Do not use Bash to run
  `novelgen tool replace`/`patch` or Python to edit `outline.json`; call the MCP
  tools directly.
- Do not refresh derived DSL after every paragraph patch; batch it.
- Do not rewrite a clean chapter just to change style.

If the tools are missing, tell the user to run
`novelgen agent install --target workbuddy --apply --user-skill` in the project
root and reopen the project in WorkBuddy.
