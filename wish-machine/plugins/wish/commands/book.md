---
description: Manage a Novelgen book: status, read, write and refresh
---

Use the `wish-book` skill before acting.

Start with `book_status`. Read context with `book_read`, then write only through
`book_write`. A write is a dry run unless `apply: true`; report findings and
warnings before applying. Never edit `story/`, `chapters/` or `novel.json`
directly.
