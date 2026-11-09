# Setup workflow

Use this when `story/setup/story_setup.json` is missing or the user asks to
regenerate the story setup.

1. Inspect the project: `novelgen tool query story-setup --view index`.
2. Generate names with the naming tool, not from memory:
   `novelgen tool query ideas --type namecard --genre 都市 --count 10 --seed 42`.
3. Build a complete setup object. Required anchors:
   - `project_name`, `genres`, `premise`, `theme`, `rules`
   - `core_cast` with at least one entry whose `role` is exactly `"protagonist"`
   - `storylines` with `name`, `description`, and pressure/payoff fields
   - `long_form_plan` for a long-form project
4. Write it through the validated mutation service:
   `printf '%s' '<complete setup json>' | novelgen tool replace setup --apply`
5. Check and repair until non-blocking:
   `novelgen tool check all --target setup --min-priority high --max-issues 12`.

Do not reuse an unrelated project's core concept (for example a system-log
premise) just because it is convenient. If the user gave only a broad theme,
design the premise from that theme and state the choice.
