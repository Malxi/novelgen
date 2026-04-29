# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Common commands

- Build CLI binary: `./build.ps1` (PowerShell) or `./build.bat` (CMD)
- Manual build: `go build -o bin/novelgen.exe`
- Run tests: `go test ./...`
- Run CLI: `bin/novelgen.exe --help` or `bin/novelgen.exe <command> --help`

## Binary output

All build scripts output the executable to `bin/novelgen.exe`. The `bin/` directory is created automatically if it doesn't exist.

Tests exist in `cmd/`, `internal/models/`, `internal/logic/`, and `internal/rpg/`. Run the focused package tests for narrow changes, and `go test ./...` before broad or contract-level changes.

## Architecture overview

- Entry point: `main.go` calls `cmd.RegisterAllCommands()` and `cmd.Execute()` to run the CLI.
- CLI commands live in `cmd/` and use Cobra. Each command registers itself via `RegisterCommand` (see `cmd/registry.go`), which builds the command tree under `cmd/root.go`.
- LLM integration is in `internal/llm/`:
  - `config.go` loads or creates `llm_config.json` (local) or `~/.novelgen/llm_config.json` (global) and selects provider/model.
  - `client.go` implements an OpenAI-compatible chat client.
- AI workflow logic is organized under `internal/agents/`, each agent handling a pipeline stage (setup/compose/craft/draft/write/recap/translate) and registered via `internal/agents/registry.go`.
- Agent prompts are skill files under `internal/agents/skills/<skill-name>/SKILL.md`. `BaseAgent.Execute` loads those skills, converts typed inputs to markdown, asks for typed JSON output, and parses back into Go structs.
- Data structures for story setup, outlines, elements, recaps, and project config live in `internal/models/`.
- Continuity logic (character presence, transitions, recaps) lives in `internal/logic/continuity/`, with shared state helpers in `internal/logic/`.
- RPG and RPG-DSL logic lives in `internal/rpg/`, especially `internal/rpg/dsl/` for parser, validator, converter, simulator, hooks, and merger behavior.
- The project's generated content layout (created by `novelgen init`) matches the structure documented in README (e.g., `novel.json`, `llm_config.json`, `story/setup`, `story/compose`, `story/craft`, `story/recaps`, `drafts/`, `chapters/`).

## AI controllability rules

- Treat every workflow stage as a data contract. Before changing setup, compose, craft, write, recap, or RPG-DSL behavior, read `docs/STAGE_CONTRACTS.md` and update it when the contract changes.
- AI output must not become project state directly. Parse AI output into a typed Go struct, normalize it when needed, validate deterministic invariants, then save it.
- When changing a model used as AI input or output, update all related skill prompts, validators/normalizers, downstream consumers, docs, and at least one focused test or fixture.
- Keep producer and consumer ownership explicit. If a field is added, document which command/agent writes it, which command/agent reads it, whether it is required, and how old projects behave when it is missing.
- Prefer deterministic logic for checks, normalization, IDs, ordering, state math, and DSL validation. Prompts may request good output, but code must enforce required invariants.
- Do not read `models.Event` fields directly in state, write, continuity, or RPG logic unless there is a specific compatibility reason. Prefer `GetActor`, `GetAction`, `GetTarget`, and `GetTargetType` so old and new outline formats stay compatible.

## Change checklists

### New CLI command

- Add the command in `cmd/<name>.go`.
- Register it with `RegisterCommand` in `init()`.
- Provide useful `Use`, `Short`, `Long`, flags, and examples for user-facing commands.
- Load project state through the existing helpers (`findProjectRoot`, `loadProjectConfig`, `loadStorySetup`, `loadOutline`) when the command operates on a novel project.
- Update README/docs when the command changes the public workflow.

### New or changed agent stage

- Add or update the agent in `internal/agents/`.
- Add or update its skill under `internal/agents/skills/`.
- Define input/output structs close to the agent or in `internal/models` when shared.
- Route all LLM calls through `BaseAgent.Execute` unless there is a clear reason not to.
- Validate outputs before writing files.
- Update `docs/STAGE_CONTRACTS.md` if the stage input, output, or invariants changed.

### Model or contract change

- Update the Go struct tags (`json`, `md`, `desc`) so prompt schema generation stays accurate.
- Update every skill that emits or consumes the changed fields.
- Update deterministic validators, normalizers, and state/continuity/RPG consumers.
- Keep backward compatibility for existing project files, or add explicit migration/default behavior.
- Add a focused test for parsing, normalization, validation, or downstream consumption.

### RPG-DSL change

- A new DSL construct is incomplete until AST, parser, validator, converter, docs, and tests are updated.
- If runtime behavior changes, update simulator/evaluator/hook tests as well.
- Keep phased DSL files (`01_outline.rpg`, `02_craft.rpg`, `03_systems.rpg`, `04_chapters.rpg`) merge-compatible.

## Verification expectations

- For ordinary Go changes, run `gofmt` on touched Go files and at least the focused package tests.
- For model, contract, RPG, or cross-stage changes, run `go test ./...` and `go build -o bin/novelgen.exe`.
- For CLI changes, also run `bin/novelgen.exe --help` or the changed command's `--help`.
