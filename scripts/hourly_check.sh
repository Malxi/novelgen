#!/usr/bin/env bash
set -euo pipefail

# Prevent overlap
LOCKFILE="/tmp/novelgen_hourly_check.lock"
exec 9>"$LOCKFILE"
if ! flock -n 9; then
  exit 0
fi

TS="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
LOG="/root/.openclaw/workspace/projects/novelgen/logs/hourly_check.log"

{
  echo "[$TS] hourly_check start"
  cd /root/.openclaw/workspace/projects/novelgen

  # Format (safe, deterministic)
  if command -v gofmt >/dev/null 2>&1; then
    gofmt -w ./cmd ./internal ./main.go 2>/dev/null || true
  fi

  # Quick compile/test (network may be restricted; use goproxy.cn as fallback)
  export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
  go test ./... || true

  # Snapshot git status for review
  if command -v git >/dev/null 2>&1; then
    echo "--- git status --porcelain ---"
    git status --porcelain || true
  fi

  echo "[$TS] hourly_check end"
  echo
} >> "$LOG" 2>&1
