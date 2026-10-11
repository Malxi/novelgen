# tools/

Standalone developer/analysis binaries. These are **not** part of the shipped
`novelgen` CLI (`main.go` + `cmd/`); they are one-off entry points that were
previously scattered under `cmd/` and confused with the real command tree.

They are still part of the root Go module, so `go build ./...` and
`go test ./...` compile and run them, but `build.ps1` / `build.bat` / the
`Makefile` `build` target only produce `bin/novelgen.exe`.

| Tool | Purpose |
| --- | --- |
| `analyze-outline` | Offline outline analysis against RPG rules |
| `check_novel` | Legacy batch novel consistency checker |
| `check_novel_smart` | Benchmark-backed novel checker |
| `deep-analyze` | Deep multi-pass outline/chapter analysis |
| `extract-llm` | LLM-backed entity extraction |
| `extract-novel` | Novel-to-RPG data extraction |
| `extract-simulate` | Extraction + simulation driver |
| `novelgen-rpg` | Standalone RPG data converter |
| `rpg-check` | Standalone RPG outline checker |
| `rpg-write` | RPG-constrained write/report helper |
| `simulate` | Story simulation runner |
| `story2rpg` | Outline-to-RPG converter |
| `validate-outline` | Outline validator |
| `visualize` | RPG visualization report generator |

Prefer adding new user-facing capability as a Cobra command under `cmd/`; only
add a binary here when it is genuinely a separate developer tool.
