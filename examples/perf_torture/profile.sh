#!/usr/bin/env bash
# Capture a CPU profile from the running perf_torture example (or any toolbox
# process exposing net/http/pprof) and compare it with the previous capture.
#
# Usage:
#   ./profile.sh [seconds]          # default 30
#   PPROF_HOST=localhost:6060 ./profile.sh 30
#
# Each capture is stored in profiles/cpu-<timestamp>.pb.gz. When a previous
# capture exists, a -diff_base report is printed so each optimization
# iteration shows exactly what got faster or slower.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
SECS="${1:-30}"
HOST="${PPROF_HOST:-localhost:6060}"

mkdir -p "$DIR/profiles"
TS="$(date +%Y%m%d-%H%M%S)"
OUT="$DIR/profiles/cpu-$TS.pb.gz"

PREV="$(ls -t "$DIR"/profiles/cpu-*.pb.gz 2>/dev/null | head -n 1 || true)"

echo "capturing ${SECS}s CPU profile from http://$HOST ..."
curl -sf -o "$OUT" "http://$HOST/debug/pprof/profile?seconds=$SECS"
echo "saved: $OUT"
echo

echo "=== top 25 (cumulative) ==="
go tool pprof -top -cum -nodecount=25 "$OUT"

if [[ -n "$PREV" ]]; then
  echo
  echo "=== diff vs previous capture ($(basename "$PREV")) ==="
  go tool pprof -top -nodecount=25 -diff_base="$PREV" "$OUT"
fi

echo
echo "flame graph:  go tool pprof -http=:8080 $OUT"
