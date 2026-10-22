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

Tests exist in `cmd/`, `internal/contract/story/`, `internal/fiction/domain/logic/`, and `internal/fiction/rpg/`. Run the focused package tests for narrow changes, and `go test ./...` before broad or contract-level changes. The `webui/` directory is a **separate Go module** (see `go.work`); the root `go test ./...` does not cover it.

## Architecture overview

- Two Go modules in one workspace (`go.work`): the root `novelgen` module (CLI + `internal/` + `tools/`) and the nested `webui` module (local web server + React frontend).
- webui integration boundary (enforced by `webui/boundary_test.go`): webui imports **nothing** from the parent module. Project reads and editor writes go through `novelgen storage projects/stat/read/list/write` via `exec`; canonical editor saves (setup/outline/craft/chapter/recap) use `novelgen storage replace`, which dispatches to `fiction/mutation.MutationService`; derived files and long tasks also go through the `novelgen` binary. webui still spells its project-relative paths itself, so keep them aligned with `internal/platform/store`'s layout by hand (`webui/README.md`).
- Entry point: `main.go` calls `cmd.RegisterAllCommands()` and `cmd.ExecuteContext(ctx)` to run the CLI (with SIGINT/SIGTERM cancellation).
- CLI commands live in `cmd/` and use Cobra. Each command registers itself via `RegisterCommand` (see `cmd/registry.go`), which builds the command tree under `cmd/root.go`.
- Command **groups can live in `cmd/<group>/` subpackages** (currently `config`, `translate`, `initcmd`, `export`, `recap`, `analyzecmd`, `simulatedsl`, `rpgcmd`, `craftcmd`, `draftcmd`): the subpackage exports `NewCommand()` (or `NewXCommand()`), and a `cmd/<group>_register.go` file wires it back via `RegisterCommand`. Prefer this when a command group only depends on `internal/*`.
- **`internal/platform/store` owns the project's on-disk layout.** `Root` (`Open`/`Locate`/`Default`) carries both the project directory and an `FS`, and every path (`OutlineJSON`, `CraftFile`, `ChapterFile`, `CheckpointDir`, …), atomic write, checkpoint/backup and typed artifact read/write lives there. Do not spell out `story/...`, `chapters/...` or `logs/...` anywhere else; add a `Root` method instead. `store.MemFS` backs in-memory projects for tests, and `cmd/root.go` resolves the root once per invocation (`--dir`, `NOVELGEN_PROJECT_DIR`, else walk up for `novel.json`) and installs it in the command context plus `store.SetDefault`.
- Shared project/content access lives in `internal/fiction/domain/project`, which is a thin facade over `internal/platform/store`: `Book` composes an explicit root and exposes domain loaders plus chapter/draft semantics; low-level path/file access must be requested explicitly through `Book.Root()`. `ForCommand`/`CommandDir` read the root from a command, and the package-level helpers (`FindRoot`, `LoadOutline`, `LoadFinalChapterContent`, `SaveDraft`, `SelectChapters`, `AllChapters`, …) keep working through `store.Default()`. Chapter markdown normalization (`NormalizeChapterContent`, `HasChapterTitle`, `StripChapterTitle`, `NarrativeUnitCount`, `ChapterContentChanged`, `ChapterNarrativeUnits`) also lives here.
- `internal/app.Session` is the command-level composition root: it resolves one `*store.Root`, loads `novel.json`, lazily loads setup/outline, and constructs the shared LLM client and agents from that root. Its state is private and must be built through `Open`, `OpenAt`, or `NewAt`; new command code should use it instead of repeating `LoadConfig`/`LoadSetup`/`LoadOutline`/`LoadOrCreateConfig`/`CreateClient` or constructing `Session` fields manually.
- Per-volume review JSON and the improvement-suggestion block live in `internal/fiction/domain/review` (`SaveVolumeReview`, `LoadVolumeReview`, `BuildImprovementSuggestions`).
- Chapter continuity (context loading, offline recaps, transition/character-presence auto-fixes) lives in `internal/fiction/domain/continuity`. New subpackages must use these packages instead of re-implementing project discovery, file layout, or normalization.
- Standalone developer/analysis binaries live in `tools/` (previously under `cmd/`); they are built by `go build ./...` but are not part of the shipped CLI.
- LLM integration is in `internal/platform/llm/`:
  - `config.go` loads or creates `llm_config.json` (local) or `~/.novelgen/llm_config.json` (global) and selects provider/model.
  - `client.go` implements an OpenAI-compatible chat client plus a `RuntimeClient` wrapper and retry logic.
- AI workflow logic is organized under `internal/fiction/agents/`, one subpackage per stage (`setup`, `compose`, `craft`, `draft`, `write`, `recap`, `translate`, `chapter`, `pipeline`) plus shared `base`, `prompt`, `review`, and `shared` packages. The root `internal/fiction/agents` package is a compatibility facade; new code should import the stage package it actually needs.
- `internal/fiction/agents/base.BaseAgent` (split across `agent_base.go`, `agent_json.go`, `agent_evidence.go`, `agent_logging.go`) has **two execution backends**:
  1. the OpenAI-compatible chat client (`internal/platform/llm`), and
  2. the Claude **Agent SDK process runtime** (`internal/platform/agentruntime` → `runners/claude_runner.py`). Major stages (setup/compose/craft/write/recap/translate) require the SDK backend (`RequireSDK: true`).
- There are **two disjoint skill trees**, both embedded with `go:embed`:
  - `internal/fiction/agents/base/skills/<skill>/SKILL.md` — stage prompts for the chat/typed-JSON path (`BaseAgent.Execute` loads these).
  - `internal/platform/agentruntime/skills/<skill>/SKILL.md` — SDK workflow prompts, materialized to `~/.novelgen/agents/skills`.
  - (`.agents/skills/` and `.trae/skills/` are external dev-editor skills, unrelated to the runtime.)
- SDK agents may not read or write project files directly. Project facts are read/patched only through the CLI's `tool` subcommand (package `internal/toolplane`, wired from `cmd/tool_register.go`): `tool.go` is the command surface, `tool_query_*.go` the query sections, `tool_context*.go` the workflow context bundles, and `tool_brief_*.go`/`tool_view.go` the budgeted projections. The deterministic check engine lives in `internal/fiction/check`, the canonical write service in `internal/fiction/mutation`, and derived-DSL refresh in `internal/fiction/refresh`; `toolplane` only adapts them to flags, JSON and navigation. Patches are dry-run by default; Go merges, normalizes and validates before `--apply` writes. Chat/typed-JSON stages call `mutation.MutationService` directly instead of calling `book.Save*` on Agent output.
- `internal/` is grouped into modules with a one-way dependency graph: `contract` (pure story contracts), `platform` (store/LLM/runtime/logging/utilities), `fiction` (story + RPG domain and pipelines), `toolplane` (agent tool adapter), `app` (composition root). Within `fiction`, the order is `rpg < domain < check < mutation < agents < derive < refresh < pipeline`. `internal/architecture/modules_test.go` enforces this graph.
- Data structures for story setup, outlines, elements, recaps, and project config live in `internal/contract/story/`.
- Continuity logic (character presence, transitions, recaps) lives in `internal/fiction/domain/logic/continuity/`, with shared state helpers in `internal/fiction/domain/logic/`.
- RPG and RPG-DSL logic lives in `internal/fiction/rpg/`, especially `internal/fiction/rpg/dsl/` for parser, validator, converter, simulator, hooks, and merger behavior. `internal/fiction/rpg/project` is the only project-loading adapter; the engine and DSL packages stay free of project layout/loading concerns.
- The project's generated content layout (created by `novelgen init`) matches the structure documented in README (e.g., `novel.json`, `llm_config.json`, `story/setup`, `story/compose`, `story/craft`, `story/recaps`, `drafts/`, `chapters/`).

## AI controllability rules

- Treat every workflow stage as a data contract. Before changing setup, compose, craft, write, recap, or RPG-DSL behavior, read `docs/STAGE_CONTRACTS.md` and update it when the contract changes.
- AI output must not become project state directly. Parse AI output into a typed Go struct, normalize it when needed, validate deterministic invariants, then save it.
- When changing a model used as AI input or output, update all related skill prompts, validators/normalizers, downstream consumers, docs, and at least one focused test or fixture.
- Keep producer and consumer ownership explicit. If a field is added, document which command/agent writes it, which command/agent reads it, whether it is required, and how old projects behave when it is missing.
- JSON that crosses the webui module boundary has no shared Go type (webui cannot import `internal/`). Change both sides in one go: the writer struct and the webui reader struct; `internal/architecture/schema_test.go` compares the JSON tags and fails when they drift (today: `logs/ai_calls` records).
- Prefer deterministic logic for checks, normalization, IDs, ordering, state math, and DSL validation. Prompts may request good output, but code must enforce required invariants.
- Do not read `models.Event` fields directly in state, write, continuity, or RPG logic unless there is a specific compatibility reason. Prefer `GetActor`, `GetAction`, `GetTarget`, and `GetTargetType` so old and new outline formats stay compatible.
- Project state is read and written through `internal/platform/store` only: get paths from a `*store.Root` (or `project.Book.Root()`) and do IO with `Root.Read`/`Write*`/`Append`/`Save*`/`FS()`. Never feed a store path to `os.ReadFile`/`os.WriteFile`/`os.Stat`/…; `os.*` is for host paths that are not project layout (agent home, the user-global config, a caller-chosen output path, whole-directory copies). A missing `*store.Root` fails closed with `store.ErrNilRoot` and must never fall back to the process working directory. `internal/architecture` enforces this plus the directory/file-name literal rule, the web UI layout parity and an allowlist of the remaining ambient-root users, so a new violation fails the build.
- Distinguish "missing" from "unreadable": only `fs.ErrNotExist` means an artifact is absent. Readers return an error for anything else, and writers must not treat a failed read as an empty file (that silently drops the rest of the artifact — see `Root.WriteJSONMerging`).
- New pipeline code must not resolve the ambient project root (`store.Default`/`project.Default`); take an explicit `*store.Root`.

## Change checklists

### New CLI command

- Add the command in `cmd/<name>.go`.
- For a self-contained command group, add it under `cmd/<group>/` exposing `NewCommand()`, and register it from `cmd/<group>_register.go`.
- Register it with `RegisterCommand` in `init()`.
- Provide useful `Use`, `Short`, `Long`, flags, and examples for user-facing commands.
- Load project state through `internal/fiction/domain/project` (`project.ForCommand(cmd)` for a command, `project.Default()` for helpers without a command handle, or plain `*store.Root` when you only need paths) — never build `story/...` paths or call `os.Getwd` yourself. The historical `cmd` wrappers such as `findProjectRoot`/`loadProjectConfig` are gone.
- Keep the CLI surface stable: `cmd/command_surface_test.go` pins top-level commands, subcommands, and key flags.
- If the command is agent-facing (a new `tool query|check|patch` surface), update the tool allowlists/evidence rules in the SDK workflows and `docs/AGENT_TOOLS.md`, not just the command.
- Update README/docs when the command changes the public workflow.

### New or changed agent stage

- Add or update the agent in its stage package under `internal/fiction/agents/<stage>/`.
- Add or update its skill under `internal/fiction/agents/base/skills/` (chat path) **and/or** `internal/platform/agentruntime/skills/` (SDK path). These are separate trees; updating one does not update the other.
- Define input/output structs close to the agent or in `internal/contract/story` when shared.
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
