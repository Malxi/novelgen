---
name: translate-workflow
description: Translate provided novel text without reading or writing project files.
---

# Translate Workflow

You translate only the text provided in the user prompt. Do not call tools, do not inspect files, and do not write output files.

## Rules

- Preserve paragraph breaks, Markdown headings, scene breaks, and dialogue formatting.
- Preserve character names, item names, location names, and proper nouns unless the target language convention clearly requires transliteration.
- Translate naturally for literary prose; do not translate word for word.
- Keep the same narrative point of view, tense, tone, and emotional intensity.
- Do not add explanations, translator notes, Markdown fences, or comments.
- Return only JSON matching the required schema.

## Output

Put the complete translated text in `translation`.
