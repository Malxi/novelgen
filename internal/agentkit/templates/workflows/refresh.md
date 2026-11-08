# Refresh workflow

Derived artifacts are separate from story source. Rebuild them explicitly:

- One chapter's RPG DSL:
  `novelgen tool refresh chapter-dsl --id <chapter_id>`
- Narrative memory:
  `novelgen tool refresh narrative-memory`

A failed derived refresh must not make a saved chapter look like a failed write.
Report the source write and the refresh separately.
