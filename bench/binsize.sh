#!/usr/bin/env bash
# B7: build the stripped static binary and fail if it is 20 MB or larger.
# Usage: bench/binsize.sh [output-path]   (run from the repo root)
set -euo pipefail

out="${1:-bin/sourcebolt}"
[ "$(go env GOOS)" = windows ] && [[ "$out" != *.exe ]] && out="$out.exe"
limit=$((20 * 1000 * 1000))

CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/sourcebolt
size=$(wc -c <"$out" | tr -d ' ')
echo "B7 binary size: $size bytes ($(awk -v s="$size" 'BEGIN{printf "%.1f", s/1e6}') MB), limit $limit"
if [ "$size" -ge "$limit" ]; then
  echo "FAIL: binary is 20 MB or larger" >&2
  exit 1
fi
