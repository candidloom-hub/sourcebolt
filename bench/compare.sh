#!/usr/bin/env bash
# Compare two `go test -bench` result files with benchstat and fail when a
# metric got significantly worse (benchstat's p < 0.05) beyond its threshold.
# Allocation metrics (B/op, allocs/op) are deterministic and use the strict
# threshold; timing metrics (sec/op, p50) use the looser one because shared
# runners are noisy. p95 tables are printed but not gated.
# TIME_EXCLUDE (regexp) names benchmarks whose timing is not compared with the
# base: their cost legitimately scales with the size of the server (e.g. the
# tool-call benchmark pays GC for every registered tool schema). Their
# allocations are still gated.
# Usage: bench/compare.sh old.txt new.txt [alloc-threshold-%] [time-threshold-%]
set -euo pipefail

old="$1" new="$2" talloc="${3:-10}" ttime="${4:-25}"
BENCHSTAT="${BENCHSTAT:-go run golang.org/x/perf/cmd/benchstat@latest}"

out=$($BENCHSTAT "$old" "$new")
echo "$out"

bad=$(echo "$out" | awk -v ta="$talloc" -v tt="$ttime" -v tx="${TIME_EXCLUDE:-^$}" '
  /^$/ { unit = "" }
  /p95-/ { unit = "skip" }
  /B\/op|allocs\/op/ && unit != "skip" { unit = "alloc" }
  /sec\/op|p50-/ && unit != "skip" { unit = "time" }
  unit != "skip" && unit != "" && match($0, /\+[0-9]+\.[0-9]+% \(p=/) {
    d = substr($0, RSTART + 1, RLENGTH - 1) + 0
    t = (unit == "alloc") ? ta : tt
    if (unit == "time" && $1 ~ tx) next
    if (d > t) print unit ": " $0
  }')
if [ -n "$bad" ]; then
  echo
  echo "FAIL: significant regression (alloc > ${talloc}%, time > ${ttime}%):" >&2
  echo "$bad" >&2
  exit 1
fi
echo "OK: no significant regression (alloc <= ${talloc}%, time <= ${ttime}%)"
