# Write workflow

Use this for one chapter at a time. The chapter id is canonical, for example
`P1-V1-C3`.

1. Get the chapter context:
   `novelgen tool query context --type chapter-write --id <chapter_id>`.
2. Read the outline characters and use their canonical full names in the prose.
3. Produce one JSON object with only `{"content": "<markdown>"}`.
4. Apply it:
   `printf '%s' '{"content":"..."}' | novelgen tool patch chapter --id <chapter_id> --apply`.
5. Do not run derived refresh on every chapter. After a batch, run:
   `novelgen tool refresh chapter-dsl --id <chapter_id>`.
6. Re-check the chapter:
   `novelgen tool check all --target chapter --scope chapter --id <chapter_id>`.

Never write `chapters/chapter-*.md` directly. Never invent a different chapter
id or edit the outline to make a chapter pass the character check.
