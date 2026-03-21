# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common commands

- Build CLI binary: `go build -o nolvegen.exe`
- Show CLI help: `novel --help` or `novel <command> --help`

Note: No `*_test.go` files were found in the repo.

## Architecture overview

- Entry point: `main.go` calls `cmd.RegisterAllCommands()` and `cmd.Execute()` to run the CLI.
- CLI commands live in `cmd/` and use Cobra. Each command registers itself via `RegisterCommand` (see `cmd/registry.go`), which builds the command tree under `cmd/root.go`.
- LLM integration is in `internal/llm/`:
  - `config.go` loads or creates `llm_config.json` (local) or `~/.nolvegen/llm_config.json` (global) and selects provider/model.
  - `client.go` implements an OpenAI-compatible chat client.
- AI workflow logic is organized under `internal/agents/`, each agent handling a pipeline stage (setup/compose/craft/draft/write/recap/translate) and registered via `internal/agents/registry.go`.
- Prompt templates and prompt builders are in `internal/prompts/`.
- Data structures for story setup, outlines, elements, recaps, and project config live in `internal/models/`.
- Continuity logic (character presence, transitions, recaps) lives in `internal/logic/continuity/`, with shared state helpers in `internal/logic/`.
- The project’s generated content layout (created by `novel init`) matches the structure documented in README (e.g., `novel.json`, `llm_config.json`, `story/setup`, `story/compose`, `story/craft`, `story/recaps`, `drafts/`, `chapters/`).
