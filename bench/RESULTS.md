# Benchmark results

Targets: `docs/plans/2026-10-14-architecture.md` section 8. Method:
`research/performance.md` sections 8 and 9. All data is synthetic: rows come
from the sanitized live `wXbhsf` capture, cloned with fresh ids
(`bench/benchdata`). No Google traffic, no real profile.

Machine (2026-10-14): Windows 11 Home 10.0.26300, Intel Core i7-12650H
(16 threads), 16 GB RAM, Go 1.27.0 windows/amd64, laptop on a desktop session
with other programs running. Variance on this machine is high (coefficient of
variation 20 to 50 percent on millisecond benchmarks, and the same code ran
2x apart between sessions), so read these as order of magnitude. CI numbers
on ubuntu-latest replace them (see "CI" below).

| # | Metric | Target | Measured | Result |
|---|---|---|---|---|
| B1 | cold start to `initialize` | p50 < 30 ms, p95 < 60 ms | p50 10.3 ms, p95 12.6 ms (29 runs, first dropped) | meets |
| B2 | peak RSS after listing 100 notebooks | < 25 MB | 19.9 MB in-process incl. MCP client and fake server; idle binary 12.4 MB | meets (conservative) |
| B3 | tool call, excluding Google, 100 notebooks | p50 < 2 ms, p95 < 5 ms | cached: p50 1.6 ms, p95 5.4 ms. refresh (full path): p50 9.6 ms, p95 18 ms, 45.9k allocs/op | refresh path FAILS; cached is borderline |
| B4 | decode 40 KB envelope | p50 < 1 ms (allocs < 100) | 475 us median (78 notebook rows, 40,088 B), 93 allocs/op, 363 KiB/op | meets |
| B5 | cache hit / 50 concurrent reads | < 1 us / 1 upstream call | not re-run here (P1 cache tests) | n/a |
| B6 | connection reuse after first request | > 95% | 100% (50 of 50 after the first), 1 TLS handshake, all HTTP/2 | meets |
| B7 | stripped binary size | < 20 MB | 10.06 MB (`-trimpath -ldflags="-s -w"`, `CGO_ENABLED=0`) | meets |
| B8 | warm browser fallback | < 3 s over RPC | not measured (needs `-tags browser`, P4) | n/a |
| B9 | streamed chat time-to-first-progress | <= first chunk + 50 ms | not measured (chat streaming not in this branch) | n/a |

## Finding: B3 refresh path is dominated by wire accessors

CPU profile of `BenchmarkToolCallNotebookList/refresh`: about 40% of the time
is in `nlm.parseNotebookRow`, which calls `wire.At`, `wire.Int`, `wire.Str`
and `wire.Elems` repeatedly. Each accessor re-unmarshals the subtree with
`json.Unmarshal` (`wire.elems`), so a row costs O(size x accesses). Parsing
each row once into `[]jsontext.Value` and indexing that, or a streaming
`jsontext.Decoder` walk, would remove most of the 45.9k allocations. The
remainder is MCP framing and output-schema validation (the cached path, 5.8k
allocs, 1.6 ms p50). This is outside the files this task may touch; it needs
a follow-up in `internal/wire` and `internal/nlm`.

## How each number was produced

| # | Command |
|---|---|
| B1 | `go run ./bench/coldstart -bin bin/sourcebolt.exe` |
| B2 | `go run ./bench/rss -bin bin/sourcebolt.exe` |
| B3, B4 | `go test ./internal/mcpserver ./internal/wire -run=XXX -bench 'ToolCallNotebookList\|Decode40KB' -count=10`, summarised with benchstat |
| B6 | `go test ./internal/transport/rpc -run TestConnectionReuse -v` |
| B7 | `bash bench/binsize.sh` |

B2 caveat: the real binary speaks HTTPS and trusts only system roots, so it
cannot talk to a fake upstream without changing machine trust settings.
`bench/rss` therefore runs the production stack (rpc transport, nlm client,
cache, MCP server) in its own process against a local TLS fake, and also
measures the spawned binary idle. The in-process figure includes the test
client and fake server, so it over-states the server. Peak working set on
Windows (psapi `GetProcessMemoryInfo`), `VmHWM` on Linux, skipped on macOS.

## Regression gate and CI

`bench/baseline.txt` holds `-count=10` results for B3 and B4 recorded on the
Windows machine above. It is a local reference only: comparing an ubuntu run
to it would be meaningless. `.github/workflows/bench.yml` therefore uses it
only if it was recorded on Linux. Otherwise, on pull requests, it benchmarks
the base branch on the same runner and runs `bench/compare.sh old new`, which
calls benchstat and fails on a significant (p < 0.05) regression above 10% in
sec/op, p50, B/op or allocs/op (p95 is printed, not gated). The workflow
uploads `new.txt` as an artifact: on the first CI run, commit it as
`bench/baseline.txt` to switch to a stored Linux baseline. The workflow has
not been run (no CI access from this machine); `bench/compare.sh` was run
locally against this baseline and correctly flagged noise-level differences
on this laptop, which shows the 10% gate will need a quiet runner or a
looser threshold if it flakes.

## 2026-10-14: tool count and GC (PR #8)

Going from 3 to 30 registered tools made `BenchmarkToolCallNotebookList` about 10-14% slower on 2 CPUs (interleaved, identical result bytes and allocations). With `GOGC=off` the gap disappears (main 0.99 ms vs PR 0.99 ms), so the cost is garbage collection scanning the larger live heap held by tool schemas, not extra work in the call path. CI therefore no longer compares this benchmark's timing with the base branch (`TIME_EXCLUDE`); its allocations are still gated.

Open item: on the 2-CPU CI runner the uncached (refresh) path measured about 2.9 ms p50, above the 2 ms B3 target. PR #12 (2026-10-20) dropped go-sdk's per-call output re-parse and validation: refresh p50 2.88 → 2.26 ms, cached 1.67 → 1.00 ms, allocations −72% (interleaved, n=10). The rest is MCP JSON-RPC encoding and decoding on both sides of the in-memory transport; the benchmark counts the client's share. A smaller default page is the remaining lever.
