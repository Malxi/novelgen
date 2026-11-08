# Review workflow

Use this after chapters exist.

1. Check the whole project:
   `novelgen tool check all --target outline --min-priority medium --max-issues 20`.
2. Check a chapter:
   `novelgen tool check all --target chapter --scope chapter --id <chapter_id>`.
3. Read the evidence, not just the issue label. Repair only the named target,
   through `novelgen tool patch` or `novelgen tool replace`.
4. Do not rewrite a clean chapter just to change style.

Review is read-only until the user asks for a repair.
