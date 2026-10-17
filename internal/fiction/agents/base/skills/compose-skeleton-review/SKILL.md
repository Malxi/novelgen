# Compose Skeleton Review Skill

## Purpose
Review an outline skeleton before chapter generation. The skeleton contains only
parts, volumes, summaries, and volume payoff contracts. Do not penalize it for
missing chapters.

## Input
- `existing_outline`: Current outline skeleton with parts and volumes.
- `setup_brief`: Compact story setup contract.
- `structure`: Expected target parts and target volumes.
- `user_prompt` (optional): Review focus from the user.

## Output
Return a `ReviewResult` JSON object.

## Review Focus

### 1. Skeleton Structure
- The number of parts and volumes should match `structure`.
- Part and volume titles should be clear, commercial, and easy to remember.
- Each volume should have a distinct stage, arena, or gameplay hook.

### 2. Long-Form Escalation
- Volumes should follow the setup's long-form escalation ladder.
- Avoid repeated placeholder patterns such as “new enemy appears, protagonist wins.”
- Every 3-4 volumes should feel like a larger phase turn.

### 3. Cause And Effect
- Each volume should naturally cause or reveal the next bigger game.
- The protagonist's reputation, resources, and pressure should visibly change.
- Avoid sudden location changes or new stages without a bridge.

### 4. Payoff Contracts
Review every volume's `payoff_contract`:
- `volume_question` should be a strong reader hook.
- `power_promise` should use the story's unique rule, not generic strength.
- `main_opponent_misread` should enable a satisfying reversal.
- `big_win` should be a concrete image.
- `visible_reward` should change the protagonist's situation.
- `reputation_shift` should show how others now see the protagonist.
- `next_bigger_game` should pull readers into the next volume.

### 5. Setup Alignment
- Check that the skeleton uses core cast and premise lines from setup.
- Check that the protagonist's limits are preserved.
- Check that the skeleton does not introduce forbidden backend metaphors or
  flatten the setup into abstract system architecture.

## Important
- Do not ask for chapter-level beats, scenes, events, or chapter count fixes.
- Suggestions must target part IDs or volume IDs, e.g. `P1`, `P1-V8`.
- Prefer concrete wording improvements over abstract criticism.

