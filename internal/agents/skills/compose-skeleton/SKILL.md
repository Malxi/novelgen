# Compose Skeleton Skill

## Purpose
Generate the story outline skeleton - parts and volumes only, without chapters.
This creates the high-level structure that will be filled in with chapters later.

## Input
- `setup_brief`: Compact StorySetup contract with premise, long-form plan, core cast seeds, storylines, progression systems, and resources.
- `structure`: StoryStructure with target parts and volumes

## Output
- `parts`: Array of Part objects, each containing:
  - `title`: Part title
  - `summary`: Part summary
  - `volumes`: Array of Volume objects, each containing:
    - `title`: Volume title
    - `summary`: Volume summary
    - `payoff_contract` (optional but strongly recommended): reader promise and satisfying volume payoff

## Structure Requirements

### Part Level
- Each part represents a major story arc
- Parts should have clear progression (setup → conflict → resolution)
- Part summaries should describe the overall arc

### Volume Level
- Each volume contains multiple chapters (will be generated later)
- Volume summaries should describe the specific storyline
- Volumes within a part should build toward the part's conclusion
- Volume-to-volume continuity must be maintained
- If setup has `long_form_plan`, map its `escalation_ladder`, `reader_promises`, `payoff_cadence`, and `volume_pattern` onto the volume sequence. Each volume should feel like a specific instance of the serial loop, not a repeated placeholder.
- Each volume should have a `payoff_contract` that makes the爽点 promise explicit:
  - `volume_question`: the question that keeps readers turning pages
  - `power_promise`: the cool rule/ability/fantasy this volume will demonstrate
  - `main_opponent_misread`: what the main obstacle misunderstands about the protagonist or rules
  - `big_win`: the satisfying win image the volume builds toward
  - `visible_reward`: concrete reward after the win
  - `reputation_shift`: how others see the protagonist differently afterward
  - `next_bigger_game`: the larger game revealed after the win

## Output Format Example

```json
{
  "parts": [
    {
      "title": "Part 1: The Beginning",
      "summary": "Introduction to the world and main conflict",
      "volumes": [
        {
          "title": "Volume 1: Discovery",
          "summary": "Protagonist discovers their unique ability and faces first challenge",
          "payoff_contract": {
            "volume_question": "Can the protagonist survive long enough to understand the ability?",
            "power_promise": "The protagonist turns a visible weakness into a winning trick.",
            "main_opponent_misread": "Enemies think the ability is only defensive.",
            "big_win": "The protagonist uses the limitation to bait and defeat a stronger enemy.",
            "visible_reward": "A new resource and public proof of competence.",
            "reputation_shift": "From disposable novice to dangerous variable.",
            "next_bigger_game": "The enemy behind the first attack notices him."
          }
        },
        {
          "title": "Volume 2: Awakening",
          "summary": "Protagonist masters their ability and confronts the antagonist's minions"
        }
      ]
    }
  ]
}
```

## Guidelines

1. **Arc Progression**: Each part should represent a complete story arc
2. **Volume Continuity**: Volume N+1 should directly follow from Volume N
3. **Balanced Scope**: Each volume should have similar scope and importance
4. **Setup Integration**: Use the story setup (premises, characters, storylines) to inform structure
5. **Genre Awareness**: Consider genre conventions when structuring parts and volumes
