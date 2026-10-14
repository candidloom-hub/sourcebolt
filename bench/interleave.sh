#!/usr/bin/env bash
# Benchmark the base and head trees interleaved, so runner drift (thermal,
# noisy neighbours, cache warm-up) affects both sides equally. Running all
# base rounds first and all head rounds second made the head look 15-30%
# slower on shared runners with identical code.
# Usage: bench/interleave.sh <base-dir> <head-dir> <rounds> <pkgs...>
# Env: BENCH_RE (benchmark regexp). Writes old.txt and new.txt in the cwd.
set -euo pipefail
base="$1" head="$2" rounds="$3"; shift 3
bins="$(mktemp -d)"
for pkg in "$@"; do
  name="$(echo "$pkg" | tr '/.' '__')"
  (cd "$base" && go test -c -o "$bins/base_$name.test" "$pkg")
  (cd "$head" && go test -c -o "$bins/head_$name.test" "$pkg")
done
: > old.txt; : > new.txt
run() { # side dir pkg
  local name; name="$(echo "$3" | tr '/.' '__')"
  (cd "$2/$3" && "$bins/$1_$name.test" -test.run '^$' -test.bench "$BENCH_RE" -test.count 1 -test.benchmem) \
    | grep -v -e '^---' -e 'bench_test.go' -e '^PASS' -e '^ok'
}
for i in $(seq 1 "$rounds"); do
  for pkg in "$@"; do
    if (( i % 2 )); then
      run base "$base" "$pkg" >> old.txt; run head "$head" "$pkg" >> new.txt
    else
      run head "$head" "$pkg" >> new.txt; run base "$base" "$pkg" >> old.txt
    fi
  done
done
