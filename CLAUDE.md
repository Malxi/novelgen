# CLAUDE.md

This repository's authoritative guidance for AI coding assistants lives in
[`AGENTS.md`](AGENTS.md) and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).
Read those first; this file is intentionally a short pointer so there is a
single source of truth instead of two diverging copies.

Quick facts:

- Build the CLI: `./build.ps1` (PowerShell) or `./build.bat` (CMD); output is `bin/novelgen.exe`.
- Tests (root module): `go test ./...`
- Tests (webui module — separate Go module, see `go.work`): `cd webui && go test ./...`
- All module tests at once: `make test-all`
- `cmd/` is the shipped CLI; `tools/` holds standalone developer binaries.
