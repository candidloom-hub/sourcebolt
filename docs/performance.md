# Performance

This page explains how sourcebolt's speed is measured, what the current numbers are, and how to reproduce them. The raw data and the full method are in [bench/RESULTS.md](../bench/RESULTS.md).

## The short version

- sourcebolt starts in about 10 ms and adds a few milliseconds of its own work per call.
- A real NotebookLM call takes most of a second or more. That time is Google's, not sourcebolt's.
- So for you, the biggest wins are fast start-up (apps can start the server on demand) and low memory, not shaving milliseconds off each call.

## How we measure

- **Synthetic data, no Google traffic.** Benchmarks replay a real, sanitized `notebook_list` response from NotebookLM, cloned with fresh ids, against a local fake server. No real account is used. This makes runs repeatable and keeps Google's own delay out of the numbers.
- **The real binary where possible.** Start-up time and idle memory are measured on the stripped release-style binary (`CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`).
- **Medians and 95th percentiles.** "p50" is the median: half the runs were faster. "p95" means 95 out of 100 runs were at least this fast.
- **Repeated, compared statistically.** Go benchmarks run 10 times and are summarized with `benchstat`. When comparing two versions, runs are interleaved (old, new, old, new) because back-to-back runs drift on a laptop.
- **One live check.** The only number that includes Google is a round trip of `notebook_list` against a real account, labelled as such.

## Current numbers

Machine: Windows 11, Intel Core i7-12650H (16 threads), 16 GB RAM, Go 1.27.0, a laptop with other programs running. Measured 2026-10-14. Variance on this machine is high (20 to 50 percent on millisecond benchmarks), so read these as orders of magnitude.

| What | Target | Measured | Result |
|---|---|---|---|
| Start-up, process start to MCP `initialize` answered | p50 < 30 ms, p95 < 60 ms | p50 10.3 ms, p95 12.6 ms (29 runs) | Meets |
| Memory while idle (real binary) | | 12.4 MB | |
| Peak memory after listing 100 notebooks | < 25 MB | 19.9 MB (in-process, includes a test client and fake server, so it overstates sourcebolt) | Meets |
| Decode a 40 KB notebook-list response | p50 < 1 ms, < 100 allocations | 475 µs median, 93 allocations | Meets |
| sourcebolt's own time per call, 100 notebooks, cached | p50 < 2 ms, p95 < 5 ms | p50 1.6 ms, p95 5.4 ms | Borderline |
| sourcebolt's own time per call, 100 notebooks, not cached | p50 < 2 ms, p95 < 5 ms | p50 9.6 ms, p95 18 ms | Misses |
| Connection reuse after the first request | > 95% | 100% (one TLS handshake, HTTP/2) | Meets |
| Program size, stripped | < 20 MB | 10.06 MB | Meets |
| Live `notebook_list` round trip, including Google | | p50 848 ms, p95 1.04 s (10 calls, one account) | |

Later work (2026-10-20) on the 2-CPU CI runner: removing a redundant output re-parse and validation step cut the uncached per-call time from 2.88 ms to 2.26 ms median and the cached time from 1.67 ms to 1.00 ms, with 72% fewer allocations (interleaved, 10 runs each). The uncached path is still above the 2 ms target; see [bench/RESULTS.md](../bench/RESULTS.md) for the analysis.

## Not measured yet

- The browser fallback (time to answer through the headless browser).
- Streamed chat: time from asking to the first progress update.

These are listed so that nobody quotes a number that does not exist.

## Reproduce the numbers

You need Go 1.27 and a clone of the repository. Benchmarks use only local fake servers.

**1. Build the stripped binary.**

bash:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/sourcebolt ./cmd/sourcebolt
```

PowerShell:

```powershell
$env:CGO_ENABLED = "0"; go build -trimpath -ldflags="-s -w" -o bin\sourcebolt.exe .\cmd\sourcebolt
```

**2. Run each measurement.** On Windows, use `bin/sourcebolt.exe` as the binary path.

| What | Command |
|---|---|
| Start-up | `go run ./bench/coldstart -bin bin/sourcebolt` |
| Memory | `go run ./bench/rss -bin bin/sourcebolt` |
| Per-call time and decode | `go test ./internal/mcpserver ./internal/wire -run=XXX -bench "ToolCallNotebookList\|Decode40KB" -count=10` |
| Connection reuse | `go test ./internal/transport/rpc -run TestConnectionReuse -v` |
| Program size | `bash bench/binsize.sh` |

Summarize benchmark output with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat). To compare two versions fairly, use `bench/interleave.sh`.

Tips for steady numbers: close other programs, plug in a laptop, and run each benchmark several times. On a busy laptop, differences under about 20 percent are noise.

## Regression gate

Pull requests run the benchmarks in CI. A statistically significant slowdown of more than 10 percent (in time, allocations or memory per call) fails the check. One exception: the timing of the per-call benchmark is not compared, because its time moves with garbage collection of the tool schemas rather than with the call path; its allocations are still gated. Details: [bench/RESULTS.md](../bench/RESULTS.md#regression-gate-and-ci).
