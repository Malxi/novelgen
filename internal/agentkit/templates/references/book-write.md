# Book write reference

`novelgen book write <setup|outline|craft|chapter|recap>` reads the complete
artifact from stdin. The command defaults to a dry run; `--apply` writes.

| Section | mode | stdin |
| --- | --- | --- |
| setup | replace | complete StorySetup JSON |
| outline | replace | complete Outline JSON (`parts`) |
| outline | patch | volume/chapter patch JSON |
| craft | patch or replace | element object or complete craft map |
| chapter | replace | complete chapter markdown |
| chapter | patch | chapter patch JSON (`content` or paragraph edits) |
| recap | replace | complete ChapterRecap JSON |

Useful flags:

```text
--mode patch|replace
--target character|item|location|organization   # craft
--id P1-V1-C1                                  # chapter/recap/outline
--apply
--dry-run
--refresh-derived                              # chapter patch
--max-issues 12
```

A whole-outline replace returns `warnings` naming chapters it would remove
(and whether saved prose or recaps exist). Review them before `--apply`.

Before the first setup or outline write, load the matching reference:

```text
book_guide(kind="reference", name="setup-schema")
book_guide(kind="reference", name="outline-schema")
```

The schemas distinguish `premise` (string) from `premises` (array of objects)
and `writing_style` (object), the shapes that most often fail a first write.
