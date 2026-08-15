#!/usr/bin/env bash
# Run the BEEF hot-path benchmarks and compare against the previous run.
#
# Usage:
#   ./bench.sh                 # full run (10 iterations per benchmark)
#   BENCHTIME=100x ./bench.sh  # custom -benchtime
#
# Results are stored in benchresults/<timestamp>.txt. If `benchstat` is
# installed (go install golang.org/x/perf/cmd/benchstat@latest), each run is
# statistically compared with the previous one — the core of the optimization
# iteration loop.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$DIR/../.." && pwd)"
BENCHTIME="${BENCHTIME:-10x}"

mkdir -p "$DIR/benchresults"
TS="$(date +%Y%m%d-%H%M%S)"
OUT="$DIR/benchresults/bench-$TS.txt"

PREV="$(ls -t "$DIR"/benchresults/bench-*.txt 2>/dev/null | head -n 1 || true)"

(cd "$REPO" && go test -bench . -benchmem -benchtime "$BENCHTIME" -count 6 ./examples/perf_torture/beefbench) | tee "$OUT"

if [[ -n "$PREV" ]]; then
  echo
  if command -v benchstat >/dev/null 2>&1; then
    echo "=== benchstat vs previous run ($(basename "$PREV")) ==="
    benchstat "$PREV" "$OUT"
  else
    echo "install benchstat for statistical comparison:"
    echo "  go install golang.org/x/perf/cmd/benchstat@latest"
    echo "then: benchstat $PREV $OUT"
  fi
fi
