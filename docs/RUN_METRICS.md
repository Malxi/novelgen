# Agent Run Metrics

Novelgen records a deterministic event ledger for each project command under
`logs/runs/<run_id>.jsonl`. This is execution observability, not an AI-authored
quality score.

## What is recorded

- CLI command, start/end time and status
- Agent and model calls with token usage, duration and prompt/response sizes
- Requested and loaded skills, including missing SDK skills
- Tool commands, permission decisions, denial reasons and tool durations
- Deterministic stuck signals such as repeated commands, denied tools, missing
  skills and patch applies without a follow-up check

Structured request payloads are redacted before entering the ledger using an
allowlist: structural flags (`--id`, `--chapter`, `--scope`, `--limit`, …) keep
their values, known boolean flags are preserved, and every other flag value or
free-form token becomes `<redacted>`. A new content flag therefore cannot leak
manuscript text into `logs/runs/` without an explicit change to that allowlist.
Full prompts and responses remain in the existing `logs/ai_calls/` stream.

## Retention and limits

One ledger file is written per command invocation. To keep the directory
bounded, `Start` prunes the oldest ledgers once the count exceeds
`NOVELGEN_RUN_METRICS_KEEP` (default `500`; `0` disables pruning). The current
run is never pruned.

`project runs --limit N` bounds IO, not just the returned slice: when no
`--since`/`--stuck` filter is active, only the newest `N` files are parsed.
Ledger files are named with a fixed-width timestamp prefix, so file-name order
is time order.

Run ids are validated (`A-Z a-z 0-9 . _ -`, at most 128 characters) before a
file is opened; a malformed id is an error rather than a silent rewrite to a
different ledger. Each event carries `version`; a newer schema version is
surfaced as a `metric_parse_error` event instead of being misread.

## CLI

```bash
novelgen project runs
novelgen project runs --stuck
novelgen project runs --since 2026-10-01 --limit 50
novelgen project run <run_id>
novelgen project runs --json
novelgen project run <run_id> --json
```

The Web UI exposes the same data under **Agent 运行** and through:

- `GET /api/runs?project=<path>&limit=50&stuck=true`
- `GET /api/runs/<run_id>?project=<path>`

## Stuck signals

Stuck status is computed from observable execution facts. The initial rules are:

- a denied tool call
- a patch applied without a later check
- a requested SDK skill that was not loaded
- the same normalized tool command observed three or more times
- timeout, max-turn, empty-response or generic agent-call failure

The ledger stores the evidence for each signal, so a future dashboard can add
weights or run comparisons without changing the raw event stream.

## Disablement and child processes

Run metrics are enabled by default and contain command shapes, not prompt or
response bodies. Set `NOVELGEN_DISABLE_RUN_METRICS=1` to disable the ledger for
an environment.

The Agent SDK child processes receive the parent `NOVELGEN_RUN_ID`; tool
subcommands therefore do not create separate top-level runs. The parent records
the tool command and duration instead.
