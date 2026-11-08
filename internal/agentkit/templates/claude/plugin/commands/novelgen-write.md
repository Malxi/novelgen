---
description: Write one Novelgen chapter
argument-hint: <chapter-id>
allowed-tools: Bash(novelgen:*)
---

Use the chapter id `$ARGUMENTS`. Follow `.novelgen/workflows/write.md`:

1. Query `novelgen tool query context --type chapter-write --id <id>`.
2. Draft the chapter with the outline's canonical character names.
3. Apply `{"content":"..."}` through `novelgen tool patch chapter --id <id> --apply`.
4. Check the chapter, then report the result. Do not refresh derived DSL unless asked.
