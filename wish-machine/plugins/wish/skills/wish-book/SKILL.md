---
name: wish-book
description: Use when creating, continuing, repairing or inspecting a Novelgen book through the wish-machine MCP tools. Covers book_status, book_read, book_write, book_refresh and book_init.
---

# Wish Book

The harness model writes; Novelgen owns the project state and validation.

## Tools

MCP server `novelgen` (the harness may prefix tools as
`mcp__novelgen__<name>`):

- `book_init` — create a book (global MCP only)
- `book_status` — progress and the single next step
- `book_read` — setup/outline/craft/chapter/context/logs
- `book_write` — `mode: patch | replace`, dry run unless `apply: true`
- `book_refresh` — derived chapter DSL or narrative memory

All tools accept `projectDir` when the server is global.

## New book

```text
book_init({
  name: "我的第一本书",
  projectDir: "my-first-novel",
  genre: "科幻",
  parts: 1,
  volumes: 3,
  chapters: 5,
  language: "zh"
})
```

`chapters` is chapters per volume. Use the returned `projectDir` on every
later call.

## Loop

1. `book_status`; follow `next.stage` / `next.targetId`.
2. `book_read` with `view: "index"` or `"brief"` and a `limit`; load only the
   context needed.
3. Generate the complete artifact with this harness's model. Use canonical
   names from the outline. Before the first setup or outline write, load
   `book_guide(kind="reference", name="setup-schema")` or `outline-schema`;
   the array/object shapes prevent a failed first write.
4. `book_write` with `apply: false`; inspect `changed`, `check`, `warnings`.
5. Repair blocking findings, then repeat the same call with `apply: true`.
6. `book_check` on the affected target.
7. `book_refresh` only when derived state is actually needed; `chapter-dsl`
   invokes Novelgen's LLM, so skip it unless the user explicitly asks.

## Write modes

```text
replace: complete setup / outline / chapter / recap
patch:   targeted outline volume/chapter, craft, paragraph repair
```

A whole-outline replace reports chapters it would remove and whether saved
prose or recaps exist. Report that to the user before applying.

## Never

- Never edit `story/`, `chapters/`, `drafts/`, or `novel.json` directly.
- Never run `novelgen setup gen`, `compose pipeline`, or `write pipeline` from
  this harness; they use Novelgen's LLM configuration.
- Never apply a write that still has blocking findings.
