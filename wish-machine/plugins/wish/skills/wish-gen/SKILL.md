---
name: wish-gen
description: Use before generating a Novelgen setup, outline, craft element, chapter or review. Loads the right workflow, skill and reference through book_guide.
---

# Wish Gen

This skill is guidance only. It does not call a model; the harness model does
the writing.

## Tool

```text
book_guide({ kind: "workflow", name: "outline" })
book_guide({ kind: "skill", name: "write-chapter-workflow" })
book_guide({ kind: "reference", name: "book-write" })
```

`kind` is `workflow`, `skill` or `reference`. Omit `name` to list available
entries.

## Stage map

| Stage | Workflow | Skill |
| --- | --- | --- |
| setup | `setup` | `setup-generate-workflow` / `setup-improve-workflow` |
| outline skeleton | `outline` | `outline-compose-skeleton-workflow` |
| outline volume | `outline` | `outline-improve-volume-workflow` |
| craft | — | `craft-character-workflow` / `craft-element-workflow` |
| write | `write` | `write-chapter-workflow` / `write-improve-workflow` |
| review | `review` | `write-review-workflow` |
| refresh | `refresh` | — |

Before the first setup write, load:

```text
book_guide({ kind: "reference", name: "setup-schema" })
```

Before the first outline write, load:

```text
book_guide({ kind: "reference", name: "outline-schema" })
```

These schemas matter: `premises`, `world_resources`, `core_cast`, and
`storylines` are arrays of objects; `writing_style` is an object; `premise` is a
string. Getting these shapes wrong costs a failed write and a retry.

Read only the workflow/skill/reference needed for the current target. Then use
`wish-book` (`book_write`) to commit the result.

## Rules

- Generate with this harness's model and token.
- Do not run Novelgen's own `setup gen`, `compose pipeline` or `write pipeline`.
- Do not paste an entire skill catalog into context; load on demand.
- Use `book_guide({kind: "reference", name: "book-write"})` before the first
  write, and `check-profiles` before reporting checks.
