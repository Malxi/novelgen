---
name: wish-ideas
description: Use when drawing deterministic name cards, scenarios, axioms, world packs, storylines or other reference ideas for Novelgen generation.
---

# Wish Ideas

Ideas are a deterministic reference library, not project state. They do not
require an open project.

## Tool

```text
book_ideas({
  type: "namecard",     // provocation|axiom|event|law|scenario|engine|worldpack|
                        // premise|storyline|cast|namecard|resource|itemcard|reference|name
  genre: "科幻",
  count: 12,
  seed: 42,
  mode: "wild",         // safe|balanced|wild|grounded
  source: "starwars",   // optional style/source hint
  view: "brief"
})
```

`count` is capped by the CLI (maximum 20). Keep the seed so the draw is
reproducible; `seed: 0` asks the tool to choose and report a fresh seed.

## Use

- Use name cards for people; do not invent generic names without checking the
  draw.
- Use world packs/scenarios for premise pressure and setting constraints.
- Pass chosen references into setup/outline/workflow generation; do not write
  them into `novel.json` directly.
- Cite the seed and type when reporting what was drawn.
