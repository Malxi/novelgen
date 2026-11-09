# Outline schema

`book_write(section="outline", mode="replace", artifact=<object>)` expects one
`Outline` object whose only top-level field is `parts`.

## Minimal complete shape

```json
{
  "parts": [
    {
      "id": "P1",
      "title": "string",
      "summary": "string",
      "volumes": [
        {
          "id": "P1-V1",
          "title": "string",
          "summary": "string",
          "payoff_contract": {
            "volume_question": "string",
            "power_promise": "string",
            "main_opponent_misread": "string",
            "big_win": "string",
            "visible_reward": "string",
            "reputation_shift": "string",
            "next_bigger_game": "string"
          },
          "chapters": [
            {
              "id": "P1-V1-C1",
              "title": "string",
              "summary": "string",
              "characters": ["string"],
              "location": "string",
              "events": [
                {
                  "type": "item",
                  "characters": ["string"],
                  "subject": "string",
                  "change": "get",
                  "details": "string",
                  "actor": "string",
                  "action": "acquire",
                  "target": "string",
                  "target_type": "item",
                  "context": "string",
                  "result": "string"
                }
              ],
              "conflict": "string",
              "pacing": "string",
              "timeline": {
                "anchor": "string",
                "start_time": "string",
                "end_time": "string",
                "duration": "string",
                "time_jump": false,
                "previous_gap": "string",
                "transition": "string"
              },
              "state_anchor": {
                "cultivation": "string",
                "spirit_stones": 0,
                "allies": ["string"],
                "injuries": ["string"],
                "location": "string",
                "key_items": ["string"],
                "notes": "string"
              },
              "enemies": [],
              "resource_ledger": [],
              "scenes": [],
              "mysteries": {
                "planted": [],
                "resolved": []
              },
              "storyline_advances": [
                {
                  "storyline_name": "string",
                  "stage": "pressure",
                  "change": "string",
                  "consequence": "string",
                  "pressure": "string"
                }
              ],
              "chapter_payoff": {
                "desire": "string",
                "pressure": "string",
                "clever_move": "string",
                "payoff_moment": "string",
                "reward": "string",
                "social_proof": "string",
                "hook": "string"
              }
            }
          ]
        }
      ]
    }
  ]
}
```

## Required level

- `parts` is an array; each part has `id`, `title`, `summary`, `volumes`.
- Each volume has `id`, `title`, `summary`, `chapters`.
- Each chapter has `id`, `title`, `summary`, `characters`, `location`,
  `events`, `conflict`, `pacing`.
- IDs are stable: `P1`, `P1-V1`, `P1-V1-C1`.
- `chapters` is an array of objects, never an array of strings.

## Nested object rules

- `events` is an array of objects. `characters` and `subject` are names, not
  nested character objects. Prefer the `actor` / `action` / `target` /
  `target_type` fields for new events; `type` / `subject` / `change` remain for
  compatibility.
- `timeline`, `state_anchor`, `mysteries`, and `chapter_payoff` are objects.
  `mysteries.planted` and `mysteries.resolved` are arrays of objects.
- `scenes` is an array of objects: `order`, `pov`, `goal`, `location`,
  `characters`, `words`, `tone`, `beats`.
- `enemies` is an array of objects: `name`, `faction`, `tier`, `count`,
  `level`, `is_boss`, `boss_id`, `status`, `context`.
- `resource_ledger` is an array of objects: `item`, `start`, `delta`, `end`,
  `reason`.

## Patching a volume

For a local repair, use `book_write` with
`mode="patch", target="volume", id="P1-V1"` and a patch object. Do not send a
whole `Outline` object in patch mode. A patch that would change the volume's
chapter set is still validated against the whole outline before writing.
