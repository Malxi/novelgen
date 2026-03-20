# Nightly continuity improvement plan

## Goal
Improve chapter-to-chapter continuity (no teleporting scene/location, maintain ongoing conversations, carry over unresolved beats).

## Implemented tonight
1) WriteAgent now injects FULL previous draft chapters into context (instead of first 500 chars).
2) Draft generation now supports --context N to include FULL previous draft chapters.
3) Added SCENE-ANCHOR RULE to draft and write prompts.
4) Added scaffolding for canonical per-chapter recap extraction (chapter_recap skill) for higher-signal continuity.

## Next steps (not yet wired into CLI)
- Add `novel recap gen --chapter ...` to extract recaps from drafts/chapters into story/recaps/*.json.
- Modify draft/write to prefer recap for immediate previous chapter (plus optionally full text) to reduce noise.
- After generating each chapter, auto-extract recap and save.
- (Optional) Add state updates from recap to StateMatrix or a canon file.

## Build note
In restricted networks, set GOPROXY:
  GOPROXY=https://goproxy.cn,direct go test ./...
