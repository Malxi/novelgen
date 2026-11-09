# Setup schema

`book_write(section="setup", mode="replace", artifact=<object>)` expects one
`StorySetup` object. Do not wrap it in another object.

## Canonical shape

```json
{
  "project_name": "string",
  "genres": ["string"],
  "premise": "string (2-4 sentences)",
  "theme": "string",
  "rules": ["string"],
  "target_audience": "string",
  "tone": "string",
  "tense": "past",
  "pov_style": "third_person_limited",
  "writing_style": {
    "name": "string",
    "description": "string",
    "principles": ["string"],
    "avoid": ["string"],
    "reference_excerpt": "string"
  },
  "long_form_plan": {
    "target_chapters": 15,
    "target_volumes": 3,
    "main_loop": "string",
    "escalation_ladder": ["string"],
    "reader_promises": ["string"],
    "payoff_cadence": "string",
    "volume_pattern": ["string"],
    "midpoint_mutation": "string",
    "endgame_promise": "string"
  },
  "core_cast": [
    {
      "id": "cast_protagonist",
      "name": "string",
      "role": "protagonist",
      "importance": 10,
      "story_function": "string",
      "relationship_to_lead": "string",
      "relationship_arc": "string",
      "entry_phase": "opening",
      "payoff": "string",
      "storyline_refs": ["string"]
    }
  ],
  "storylines": [
    {
      "name": "string",
      "description": "string",
      "type": "main",
      "importance": 10,
      "scope": "book",
      "payoff_style": "staged_reveal",
      "setup_role": "string",
      "repeatable_pressure": "string",
      "payoff_cadence": "string",
      "mutation": "string",
      "failure_mode": "string",
      "desire": "string",
      "opposition": "string",
      "stakes": "string",
      "turn": "string",
      "payoff": "string",
      "open_question": "string",
      "pressure_points": ["string"],
      "appeal_engine": {
        "appeal": "string",
        "surface_limit": "string",
        "exploit": "string",
        "signature_win": "string",
        "upgrade_path": "string",
        "opponent_misread": "string",
        "reward_type": "string"
      }
    }
  ],
  "premises": [
    {
      "name": "string",
      "description": "string",
      "category": "string",
      "progression": [
        {
          "level": 1,
          "name": "string",
          "description": "string",
          "requirements": "string"
        }
      ],
      "appeal_engine": {
        "appeal": "string",
        "surface_limit": "string",
        "exploit": "string",
        "signature_win": "string"
      }
    }
  ],
  "world_timeline": [
    {
      "year": "string",
      "event": "string",
      "impact": "string",
      "related_mystery": "string"
    }
  ],
  "world_resources": [
    {
      "name": "string",
      "category": "能源|消耗品|材料|货币",
      "scarcity": "常见|稀有|独一无二",
      "description": "string"
    }
  ]
}
```

## Common mistakes

- `premise` is a **string**; `premises` is an **array of objects**. Never put
  strings in `premises`.
- `writing_style` is an **object**, not a string. Omit it or use the object
  shape above.
- `world_resources`, `core_cast`, `storylines`, and `world_timeline` are arrays
  of objects, not arrays of strings.
- `genres`, `rules`, `storyline_refs`, and `principles` are arrays of strings.
- `importance` is a number; `long_form_plan.target_chapters` /
  `target_volumes` are numbers.
- A replace replaces the whole setup. A patch should send only changed fields.
