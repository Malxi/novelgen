# Check profiles and findings

`novelgen check` runs deterministic checks and returns JSON. `findings` is the
canonical list; `issues` is kept as a compatibility alias. Each finding has
`rule_id`, `severity` and, for patchable findings, `navigation`.

Profiles:

```text
quality      structural/contract rules
schema       typed craft or recap schema validation
rpg          in-memory RPG simulation; same engine as rpg-dsl simulate
all          quality + rpg
custom       rules from .novelgen/rules/*.json
```

Common flags:

```text
--target setup|outline|craft|chapter|recap
--scope all|volume|chapter|character|item|location|organization
--id <target-id>
--min-priority low|medium|high|critical
--max-issues 12
```

A blocking finding has `priority` `high` or `critical`, or `blocking: true` in
the summary. Repair blocking findings before applying a write.
