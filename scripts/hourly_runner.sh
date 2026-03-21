#!/usr/bin/env bash
set -euo pipefail

# A lightweight hourly scheduler (no cron/systemd required).
# It skips runs if the lock is held (i.e., a run is already in progress).

LOCKFILE="/tmp/novelgen_hourly_job.lock"
LOG="/root/.openclaw/workspace/projects/novelgen/logs/hourly_runner.log"
JOB="/root/.openclaw/workspace/projects/novelgen/scripts/hourly_check.sh"

while true; do
  TS="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  {
    echo "[$TS] tick"
    # Non-overlap: if locked, skip this tick
    exec 9>"$LOCKFILE"
    if flock -n 9; then
      echo "[$TS] running hourly_check"
      /bin/bash "$JOB" || true
      echo "[$TS] done"
    else
      echo "[$TS] skipped (busy)"
    fi
  } >> "$LOG" 2>&1

  # sleep ~1 hour
  sleep 3600
done
