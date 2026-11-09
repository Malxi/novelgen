---
name: wish-check
description: Use when checking a Novelgen book with quality, schema or RPG rules. Explains findings, severities and blocking status.
---

# Wish Check

Checks are deterministic; they do not call a model.

## Tool

```text
book_check({
  projectDir: "...",       // when using the global MCP server
  profile: "quality",      // quality | schema | rpg | all
  target: "outline",       // setup | outline | craft | chapter | recap
  scope: "chapter",
  id: "P1-V1-C1",
  minPriority: "high",
  maxIssues: 12
})
```

`profile: "rpg"` uses the in-memory RPG simulator. `schema` validates typed
craft/recap JSON. `all` combines quality and RPG.

The result exposes `findings`; `issues` is kept as a compatibility alias. Each
finding has `rule_id`, `severity` and, for patchable findings, `navigation`.
A finding is blocking when its `severity` is `high`/`critical`, or when
`check.blocking` is true. `check.summary` always contains the full counts even
when the returned list is capped by `maxIssues`.

## Reporting

Report:

- blocking findings first, with `target_id` and `message`
- then medium/low findings only if the user asked for a full review
- the exact check command or tool arguments used

Do not edit project files to make a check pass. Hand a repair plan to the
`wish-book` skill or `book_write` dry run.

`profile: "custom"` evaluates `.novelgen/rules/*.json` for setup, outline, or
chapter. A rule file can be an object or an array:

```json
{"id":"no-deus-ex","target":"chapter","severity":"high","kind":"regex","pattern":"突然.*醒来","message":"possible deus ex machina"}
```
