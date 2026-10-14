#!/usr/bin/env bash
# Compare two `go test -bench` result files with benchstat and fail when any
# metric got significantly worse (benchstat's p < 0.05) by more than 10%.
# Usage: bench/compare.sh old.txt new.txt [threshold-percent]
set -euo pipefail

old="$1" new="$2" threshold="${3:-10}"
BENCHSTAT="${BENCHSTAT:-go run golang.org/x/perf/cmd/benchstat@latest}"

out=$($BENCHSTAT "$old" "$new")
echo "$out"

# benchstat marks a significant change as "+12.34% (p=0.002 n=10)"; "~" means
# no significant difference. A positive delta is a regression for every unit we
# benchmark (sec/op, B/op, allocs/op, p50). The p95 tables are printed but not
# gated: tail latency on shared runners is too noisy for a 10% rule.
bad=$(echo "$out" | awk -v t="$threshold" '
  /^$/ { skip = 0 }
  /p95-/ { skip = 1 }
  !skip && match($0, /\+[0-9]+\.[0-9]+% \(p=/) {
    d = substr($0, RSTART + 1, RLENGTH - 1) + 0
    if (d > t) print $0
  }')
if [ -n "$bad" ]; then
  echo
  echo "FAIL: significant regression of more than ${threshold}%:" >&2
  echo "$bad" >&2
  exit 1
fi
echo "OK: no significant regression above ${threshold}%"
