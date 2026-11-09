# Stage contracts

The typed contracts for setup, outline, craft, chapters, recaps and reviews
live in `internal/contract/story`. The stage prompt contracts and invariants
are documented in `docs/STAGE_CONTRACTS.md`.

When generating an artifact:

1. Read the matching workflow with `book_guide(kind="workflow", name="...")`.
2. Read the matching skill with `book_guide(kind="skill", name="...")`.
3. Use the canonical field names from the reference; do not invent fields.
4. Write through `book write` (or MCP `book_write`), not direct file edits.
