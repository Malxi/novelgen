# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Common commands

- Build CLI binary: `./build.ps1` (PowerShell) or `./build.bat` (CMD)
- Manual build: `go build -o bin/novelgen.exe`
- Run tests (root module): `go test ./...`
- Run tests (webui module, separate module): `cd webui && go test ./...`
- Run all module tests at once: `make test-all`
- Run CLI: `bin/novelgen.exe --help` or `bin/novelgen.exe <command> --help`

## Binary output

All build scripts output the executable to `bin/novelgen.exe`. The `bin/` directory is created automatically if it doesn't exist.

Tests exist in `cmd/`, `internal/models/`, `internal/logic/`, and `internal/rpg/`. Run the focused package tests for narrow changes, and `go test ./...` before broad or contract-level changes. The `webui/` directory is a **separate Go module** (see `go.work`); the root `go test ./...` does not cover it.

## Architecture overview

- Two Go modules in one workspace (`go.work`): the root `novelgen` module (CLI + `internal/` + `tools/`) and the nested `webui` module (local web server + React frontend).
- webui integration boundary (enforced by `webui/boundary_test.go`): webui imports **nothing** from the parent module — read-only endpoints read project files themselves and pass the raw JSON through, and all mutations, derived files and long tasks go through the `novelgen` binary via `exec`. Consequently webui re-derives a handful of project paths (its own convention, documented in `webui/README.md`); keep those aligned with `internal/store`'s layout by hand, since the boundary test forbids sharing the code.
- Entry point: `main.go` calls `cmd.RegisterAllCommands()` and `cmd.ExecuteContext(ctx)` to run the CLI (with SIGINT/SIGTERM cancellation).
- CLI commands live in `cmd/` and use Cobra. Each command registers itself via `RegisterCommand` (see `cmd/registry.go`), which builds the command tree under `cmd/root.go`.
- Command **groups can live in `cmd/<group>/` subpackages** (currently `config`, `translate`, `initcmd`, `export`, `recap`, `analyzecmd`, `simulatedsl`, `rpgcmd`, `craftcmd`, `draftcmd`): the subpackage exports `NewCommand()` (or `NewXCommand()`), and a `cmd/<group>_register.go` file wires it back via `RegisterCommand`. Prefer this when a command group only depends on `internal/*`.
- **`internal/store` owns the project's on-disk layout.** `Root` (`Open`/`Locate`/`Default`) carries both the project directory and an `FS`, and every path (`OutlineJSON`, `CraftFile`, `ChapterFile`, `CheckpointDir`, …), atomic write, checkpoint/backup and typed artifact read/write lives there. Do not spell out `story/...`, `chapters/...` or `logs/...` anywhere else; add a `Root` method instead. `store.MemFS` backs in-memory projects for tests, and `cmd/root.go` resolves the root once per invocation (`--dir`, `NOVELGEN_PROJECT_DIR`, else walk up for `novel.json`) and installs it in the command context plus `store.SetDefault`.
- Shared project/content access lives in `internal/project`, which is a thin facade over `internal/store`: `Book` is the explicit-root view (chapter/draft IO, `LoadConfig`/`LoadSetup`/`LoadOutline`/`LoadElements`), `ForCommand`/`CommandDir` read the root from a command, and the package-level helpers (`FindRoot`, `LoadOutline`, `LoadFinalChapterContent`, `SaveDraft`, `SelectChapters`, `AllChapters`, …) keep working through `store.Default()`. Chapter markdown normalization (`NormalizeChapterContent`, `HasChapterTitle`, `StripChapterTitle`, `NarrativeUnitCount`, `ChapterContentChanged`, `ChapterNarrativeUnits`) also lives here.
- Per-volume review JSON and the improvement-suggestion block live in `internal/reviewstore` (`SaveVolumeReview`, `LoadVolumeReview`, `BuildImprovementSuggestions`).
- Chapter continuity (context loading, offline recaps, transition/character-presence auto-fixes) lives in `internal/continuity`. New subpackages must use these packages instead of re-implementing project discovery, file layout, or normalization.
- Standalone developer/analysis binaries live in `tools/` (previously under `cmd/`); they are built by `go build ./...` but are not part of the shipped CLI.
- LLM integration is in `internal/llm/`:
  - `config.go` loads or creates `llm_config.json` (local) or `~/.novelgen/llm_config.json` (global) and selects provider/model.
  - `client.go` implements an OpenAI-compatible chat client plus a `RuntimeClient` wrapper and retry logic.
- AI workflow logic is organized under `internal/agents/`, each agent handling a pipeline stage (setup/compose/craft/draft/write/recap/translate). Agents are constructed directly (no registry).
- `internal/agents/BaseAgent` (split across `agent_base.go`, `agent_json.go`, `agent_evidence.go`, `agent_logging.go`) has **two execution backends**:
  1. the OpenAI-compatible chat client (`internal/llm`), and
  2. the Claude **Agent SDK process runtime** (`internal/agentruntime` → `runners/claude_runner.py`). Major stages (setup/compose/craft/write/recap/translate) require the SDK backend (`RequireSDK: true`).
- There are **two disjoint skill trees**, both embedded with `go:embed`:
  - `internal/agents/skills/<skill>/SKILL.md` — stage prompts for the chat/typed-JSON path (`BaseAgent.Execute` loads these).
  - `internal/agentruntime/skills/<skill>/SKILL.md` — SDK workflow prompts, materialized to `~/.novelgen/agents/skills`.
  - (`.agents/skills/` and `.trae/skills/` are external dev-editor skills, unrelated to the runtime.)
- SDK agents may not read or write project files directly. Project facts are read/patched only through the CLI's `tool` subcommand (package `internal/toolplane`, wired from `cmd/tool_register.go`: `tool.go` is the command surface, `tool_query_*.go` the query sections, `tool_context*.go` the workflow context bundles, `tool_check_*.go`/`tool_patch_*.go`/`tool_brief_*.go` one file per target (setup/outline/craft/chapter/recap), and `tool_patch_*.go` the only writes): `tool query|check|patch|patch-buffer|refresh`. Patches are dry-run by default; Go merges, normalizes and validates before `--apply` writes.
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
- For a self-contained command group, add it under `cmd/<group>/` exposing `NewCommand()`, and register it from `cmd/<group>_register.go`.
- Register it with `RegisterCommand` in `init()`.
- Provide useful `Use`, `Short`, `Long`, flags, and examples for user-facing commands.
- Load project state through `internal/project` (`project.ForCommand(cmd)` for a command, `project.Default()` for helpers without a command handle, or plain `*store.Root` when you only need paths) — never build `story/...` paths or call `os.Getwd` yourself. The historical `cmd` wrappers such as `findProjectRoot`/`loadProjectConfig` are gone.
- Keep the CLI surface stable: `cmd/command_surface_test.go` pins top-level commands, subcommands, and key flags.
- If the command is agent-facing (a new `tool query|check|patch` surface), update the tool allowlists/evidence rules in the SDK workflows and `docs/AGENT_TOOLS.md`, not just the command.
- Update README/docs when the command changes the public workflow.

### New or changed agent stage

- Add or update the agent in `internal/agents/`.
- Add or update its skill under `internal/agents/skills/` (chat path) **and/or** `internal/agentruntime/skills/` (SDK path). These are separate trees; updating one does not update the other.
- Define input/output structs close to the agent or in `internal/models` when shared.
- Route all LLM calls through `BaseAgent.Execute` unless there is a clear reason not to. Note the two backends (chat client vs Agent SDK runtime) and whether the stage sets `RequireSDK`.
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
- For changes touching `webui/`, also run `cd webui && go test ./...` (or `make test-all`).
- For CLI changes, also run `bin/novelgen.exe --help` or the changed command's `--help`.
