---
name: novelgen
description: Use when creating, continuing, checking, or repairing a Novelgen novel project. Covers setup, outline, chapter writing, review, and derived refreshes through the novelgen CLI.
---

# Novelgen

Novelgen owns the project state. Read the relevant workflow before acting:

- `.novelgen/workflows/setup.md`
- `.novelgen/workflows/outline.md`
- `.novelgen/workflows/write.md`
- `.novelgen/workflows/review.md`
- `.novelgen/workflows/refresh.md`

Hard rules:

1. The only write path is the `novelgen` CLI (`tool replace`, `tool patch`,
   `tool refresh`, or a stage command). Never edit `story/`, `chapters/`, or
   `novel.json` directly.
2. Complete artifacts use `novelgen tool replace ... --apply`. Incremental repairs use
   `novelgen tool patch`.
3. A complete setup is required before an outline; an outline is required
   before chapters.
4. Use canonical character names from the outline in chapter prose.
5. Derived RPG DSL is refreshed explicitly, not as part of every chapter apply.
6. Before claiming completion, run the relevant
   `novelgen tool check all --target ...` and report blocking issues.

If the user gives only a broad theme, do not reuse another project's premise.
Generate names with `novelgen tool query ideas --type namecard` and choose a
concept that follows from the requested theme.
