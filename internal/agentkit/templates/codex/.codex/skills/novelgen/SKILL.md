---
name: novelgen
description: Use when creating, continuing, checking, or repairing a Novelgen novel project. Covers setup, outline, chapter writing, review, and derived refreshes through the novelgen CLI.
---

# Novelgen

Read the relevant workflow before acting:

- `.novelgen/workflows/setup.md`
- `.novelgen/workflows/outline.md`
- `.novelgen/workflows/write.md`
- `.novelgen/workflows/review.md`
- `.novelgen/workflows/refresh.md`

The only write path is the `novelgen` CLI. Never edit `story/`, `chapters/`, or
`novel.json` directly. Complete artifacts use `novelgen tool replace ... --apply`;
incremental repairs use `novelgen tool patch`; derived files use
`novelgen tool refresh`.

Use canonical outline character names in chapter prose. Check before claiming
completion.
