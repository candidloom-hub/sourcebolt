# Performance research: NotebookLM MCP server in Go

Date: 2026-10-14. Scope: lowest latency per tool call, fastest cold start, small memory, robust under Google throttling.

Conventions:
- Every claim carries a URL. Claims with no primary source are marked **[unverified]**.
- "Measured here" means I ran it on this machine: Windows 11, Intel i7-12650H (16 threads), Go 1.27.0 windows/amd64, go-sdk v1.8.0, `CGO_ENABLED=0`. Throwaway code lived in the session scratchpad, not in this repo. Numbers are from one machine and one run set, so treat them as baselines, not guarantees.
- "Target" means a proposal, not a measured fact.
- Upstream NotebookLM facts (cookie rotation, retry policy, RPC IDs) come from reading `teng-lin/notebooklm-py` and `jacob-bd/notebooklm-mcp-cli` source and docs. They are reverse-engineered third-party claims, not Google documentation.

---

## 0. Headline findings

1. Our client-side overhead is tiny next to the Google round trip. Measured here: a stripped MCP server with 20 typed tools answers `initialize` in about 17 ms p50 and sits at about 12 MB peak working set. Parsing a 37 KB batchexecute chunk costs 0.3 to 1.0 ms depending on method. The levers that matter are connection reuse, avoiding redundant calls (cache plus singleflight), correct retry/backoff, and avoiding the browser path. Micro-optimising JSON or the linker will not move user-visible latency. See sections 2, 3, 8.
2. Server-side quotas dominate "robustness under throttling", and the browser fallback does not escape them. Same Google account, same daily quota. See section 4 and the "Contradictions" list.
3. Go 1.27 changes the JSON decision. `encoding/json` is now backed by the v2 implementation, and `encoding/json/v2` plus `encoding/json/jsontext` are importable without a GOEXPERIMENT. Stdlib is enough. See section 3.

---

## 1. MCP go-sdk

### Version and API shape
- Latest release: **v1.8.0, published 2026-09-14** (hardening of nesting/frame limits, a server option to narrow advertised protocol versions, `SetCacheable` hook, two `MCPGODEBUG` compat flags scheduled for removal in v1.9.0). Source: https://api.github.com/repos/modelcontextprotocol/go-sdk/releases/latest
- Module `go.mod` on main requires Go 1.25.0 and pulls `google/jsonschema-go`, `segmentio/encoding`, `golang.org/x/oauth2`, `golang.org/x/time` etc. All pure Go. Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/go.mod
- Measured here: `go list -deps` of a minimal server is 237 packages and builds with `CGO_ENABLED=0`.
- Shape (from the SDK README, via Context7 `/modelcontextprotocol/go-sdk`, https://github.com/modelcontextprotocol/go-sdk/blob/main/README.md):

```go
server := mcp.NewServer(&mcp.Implementation{Name: "x", Version: "v1"}, nil)
mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "..."},
    func(ctx context.Context, req *mcp.CallToolRequest, in Input) (*mcp.CallToolResult, Output, error) { ... })
server.Run(ctx, &mcp.StdioTransport{})
```
- `AddTool` infers `InputSchema` from the `In` struct, and `OutputSchema` from `Out` unless it is `any`. It validates arguments against the input schema and validates output against the output schema. `Out` is marshalled into both `StructuredContent` and text `Content`. An ordinary returned error becomes `IsError: true`. Source: https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/server.md (Context7 snippet from `internal/docs/server.src.md`).

### Transports
**Recommendation: ship stdio as the default and only required transport. Add streamable HTTP as an opt-in flag for Docker/remote use.**
- Why: the MCP spec says clients SHOULD support stdio whenever possible, and stdio is one subprocess with no HTTP stack, no Origin checks, no session IDs. Source: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports
- Stdio rules that bite: newline-delimited JSON with no embedded newlines; server MUST NOT write anything but MCP messages to stdout; logs go to stderr. Same source. Any stray `fmt.Println` or a browser library logging to stdout corrupts the stream.
- Streamable HTTP: servers MUST validate `Origin` and SHOULD bind to 127.0.0.1 locally (DNS rebinding). Same source.
- go-sdk streamable HTTP: `StreamableHTTPHandler` is an `http.Handler`; `StreamableHTTPOptions{Stateless, JSONResponse, EventStore}`. `JSONResponse: true` returns plain `application/json` instead of an SSE stream; `Stateless` handles each request in a temporary session; resumability is off unless an `EventStore` is set. Source: https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/docs/protocol.md
- Protocol drift risk: the SDK docs say protocol version `2026-07-28` requires stateless mode and removes `ping` and SSE resumability, and v1.8.0 makes stateful handlers return a JSON-RPC error for `2026-07-28` requests so clients renegotiate. Same protocol.md and release notes above. We need a test that a client negotiating `2026-07-28` against our stdio server behaves. **[unverified how stdio behaves under that version; test it.]**
- Do not enable `KeepAlive` pings for the `2026-07-28` case (docs say ping is removed there). Same protocol.md.

### Concurrency of tool handlers
**Recommendation: rely on the SDK's concurrent handler model, but gate Google calls ourselves (section 4).**
- Why: in the SDK source, `tools/call` (any call except `initialize`) invokes `jsonrpc2.Async(ctx)`, which releases the serial dispatcher so calls run concurrently; notifications are handled synchronously. Source: https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/server.go (comment citing go-sdk issue 26 near `jsonrpc2.Async`) and https://github.com/modelcontextprotocol/go-sdk/blob/main/internal/jsonrpc2/conn.go (`handleAsync`, `releaser`).
- Consequence: an LLM client that fires 5 parallel tool calls produces 5 goroutines hitting Google at once. Without a limiter that looks like a burst to Google.
- Measure: a test with N concurrent `CallTool` over `mcp.NewInMemoryTransports()` and a fake upstream with a counter of in-flight requests; assert the in-flight count never exceeds the configured limit.

### Progress, streaming partial results, cancellation
- Progress: client puts a `progressToken` in request `_meta`; handler calls `req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{...})`. It is sent only if the peer supplied a token. Source: https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/protocol.md (Context7 snippet "Report progress").
- Spec: `progress` MUST increase on each notification; both sides SHOULD rate-limit them. Source: https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/progress
- **Recommendation:** use progress for long operations only (audio/video generation polling, deep research, multi-source upload). Throttle to at most one per second **[proposal]**. There is no MCP mechanism for streaming partial tool *results* inside one `tools/call` beyond progress messages. For chat answers, return the final text and use progress `message` for status. **[unverified whether the 2025-11-25 "tasks" utility is implemented in go-sdk v1.8.0; spec shows `execution.taskSupport` on tools: https://modelcontextprotocol.io/specification/2025-11-25/server/tools. Check before designing long-running artifact generation around it.]**
- Cancellation: cancelling the context passed to a session method sends `notifications/cancelled`; server handler sees `ctx.Done()`. Source: protocol.md above. Spec: receivers SHOULD stop and not respond; cancellation can race with completion and must be tolerated. Source: https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/cancellation
- **Rule:** every outbound Google request must use the handler's `ctx` so cancel aborts the HTTP call and releases the limiter slot. A non-idempotent call already sent (chat ask, create notebook, add source) should not be auto-retried after cancel; see section 4.

### Structured output
- Spec: servers returning structured content SHOULD also return serialized JSON in a text block; if `outputSchema` is given, results MUST conform. Source: https://modelcontextprotocol.io/specification/2025-11-25/server/tools
- **Recommendation:** typed `Out` structs for list/get tools; keep schemas small (flat structs, `omitempty`). Large source full-text should be returned as text or a resource link, not duplicated into structured plus text content (SDK writes both by default; doubling a 200 KB source body doubles the bytes over stdio). Source for the duplication: docs/server.md snippet above. **[unverified: whether `Content` can be set to a short summary while keeping `StructuredContent` full; the docs say the SDK fills `Content` only if unset, which suggests yes.]**
- Spec distinguishes protocol errors from tool execution errors (`isError: true`); throttling and auth-expired should be tool execution errors with an actionable message so the model can back off. Same tools page.

### Schema generation cost
- The SDK ships `SchemaCache`, a `sync.Map`-based cache keyed by Go type to avoid reflection-based schema generation; its own docs say it matters for stateless per-request server construction. Source: https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/schema_cache.go and `ServerOptions.SchemaCache` in mcp/server.go.
- **Recommendation:** for the stdio server, register tools once at startup; no cache needed. If we run streamable HTTP in `Stateless` mode and rebuild the server per request, set one shared `SchemaCache`.
- Measured here: 20 typed tools registered, `initialize` p50 17.7 ms, `initialize`+`tools/list` p50 18.7 ms, including process spawn. So total schema generation for 20 tools is at most single-digit milliseconds (upper bound, not isolated).
- Measure in-project: `BenchmarkAddTool` (build the real tool set N times) and `GODEBUG=inittrace=1 ./notebooklm-mcp` for package-init cost (see section 9).

---

## 2. HTTP client tuning for batchexecute

**Recommendation: one process-wide `*http.Client` with a tuned `*http.Transport`, never created per call.**

| Setting | Recommendation | Why / source |
|---|---|---|
| Reuse | One `http.Transport`, shared | Docs: "Transports should be reused instead of created as needed"; safe for concurrent use. https://pkg.go.dev/net/http#Transport (verified locally with `go doc net/http.Transport`) |
| HTTP/2 | `ForceAttemptHTTP2: true` when you set a custom `TLSClientConfig` or dialer | Docs: any custom Dial/DialContext/TLSClientConfig conservatively disables HTTP/2 unless `ForceAttemptHTTP2` is true. Same page. Without it you silently fall back to HTTP/1.1 and lose multiplexing. |
| Idle conns | `MaxIdleConnsPerHost` = parallelism cap (e.g. 8), `MaxIdleConns` 32, `IdleConnTimeout` 90s | Default per-host idle limit is `DefaultMaxIdleConnsPerHost = 2`. Same page. With HTTP/2 you mostly use one connection, but HTTP/1.1 fallback and other Google hosts (accounts.google.com, upload endpoints) need slots. Values are **[proposal]**. |
| Keep-alive | Leave on. Always fully read and close the body | Go 1.27 now drains unread HTTP/1 `Response.Body` on `Close` up to a conservative limit to help reuse, but do not rely on it for large bodies. Source: https://go.dev/doc/go1.27 (net/http section) |
| TLS session resumption | Set `TLSClientConfig.ClientSessionCache = tls.NewLRUClientSessionCache(32)` | `crypto/tls` docs: on clients, session ticket support is disabled if `ClientSessionCache` is nil (`go doc crypto/tls.Config`, https://pkg.go.dev/crypto/tls#Config). Cheap, helps after idle-timeout reconnects. Verify with `httptrace` + `ConnectionState().DidResume`. |
| Compression | Do not set `Accept-Encoding` yourself; leave `DisableCompression` false | Docs: Transport adds `Accept-Encoding: gzip` itself and transparently decodes. If you set the header yourself it will not auto-decode. https://pkg.go.dev/net/http#Transport. Go 1.27 made *compression* faster (compress/flate) but that is not our response path. https://go.dev/doc/go1.27 |
| Timeouts | Per-call `context.WithTimeout` from the handler ctx; set `Transport.TLSHandshakeTimeout`, `ResponseHeaderTimeout` (e.g. 30s for normal RPCs); no `Client.Timeout` on streaming chat | `ResponseHeaderTimeout` excludes body read time (Transport docs). Values **[proposal]**. |
| Redirects | `CheckRedirect` returns `http.ErrUseLastResponse`; treat a 302 to `accounts.google.com` as "session expired" | Upstream reports expired cookies show up as redirects to the Google login page: https://github.com/jacob-bd/notebooklm-mcp-cli/blob/main/docs/KNOWN_ISSUES.md. Design guidance, not a Google doc. |
| Cookie handling | Use a `net/http/cookiejar` pre-seeded from the stored session | Google sets rotated cookies via Set-Cookie; the jar captures them (persist them back, section 7). |
| DNS caching | Do not add one first. Measure with `httptrace.ClientTrace.DNSDone` | Go stdlib has no DNS cache; maintainers say the OS should provide it and asked for numbers first: https://golang.org/issue/24796. Windows and macOS have OS caches (same issue thread). Only matters in minimal containers. |
| Pre-warming | At startup, in a goroutine, fetch the NotebookLM app page with the shared client. Not a separate dummy request | The page fetch is needed anyway: upstream extracts the CSRF token and the frontend build label `bl` from the page at login/refresh: https://github.com/jacob-bd/notebooklm-mcp-cli/blob/main/docs/KNOWN_ISSUES.md and https://github.com/jacob-bd/notebooklm-mcp-cli/blob/main/docs/AUTHENTICATION.md. One request does both: opens TCP+TLS+H2 and primes tokens. Do it lazily on first tool call if no stored session exists (do not touch Google before the user needs it). |

Notes:
- HTTP/3 is not in `net/http` as of the Go 1.27 release notes I read (no mention). **[unverified negative; confirm with `go doc net/http.Protocols`.]** Not worth chasing.
- `Transport.Protocols` and `Transport.HTTP2` (`HTTP2Config`) exist for finer control. Go 1.26 added `HTTP2Config.StrictMaxConcurrentRequests` to open a new connection when the stream limit is hit. Source: https://go.dev/doc/go1.26. Useful if we ever fan out more than the server's `MAX_CONCURRENT_STREAMS`.

**Measure:** `httptrace.ClientTrace` per request logging `GotConn.Reused`, `DNSDone`, `ConnectDone`, `TLSHandshakeDone`, `GotFirstResponseByte`. Metrics: fraction of calls with `Reused=true` (target above 95% after the first call **[proposal]**), TTFB, total. Source for the tracing hook: https://go.dev/doc/diagnostics ("httptrace.ClientTrace").

---

## 3. Response parsing

### Envelope facts (upstream, reverse-engineered)
- Responses start with `)]}'` (XSSI guard), then chunks of `<length>\n<json>\n`. Source: https://github.com/teng-lin/notebooklm-py/blob/main/src/notebooklm/_web/wire/decoder.py
- The declared length is unreliable. The upstream decoder tolerates mismatches and counts them, noting that live responses use a different unit than UTF-8 byte length, "likely UTF-16 code units". Same file. **Recommendation: do not use the length prefix to slice the body.** Split on newlines, skip numeric-only lines, parse each remaining line as one JSON value. If a payload could contain raw newlines this fails; JSON strings cannot contain raw newlines, so each chunk is one line. **[inference from JSON grammar; confirm against recorded fixtures.]**
- Outer row shape: `["wrb.fr", "<rpcId>", "<inner JSON as a string>", null, null, null, "generic"]`. The third element is JSON-in-JSON. Example RPC IDs (`wXbhsf` list notebooks, `rLM1Ne` get notebook, etc.): https://github.com/teng-lin/notebooklm-py/blob/main/docs/rpc-reference.md
- Stream chat uses a separate streaming parser upstream (`_web/rows/chat_stream.py`). Parse incrementally for chat.

### Library choice
Measured here (benchmark: a synthetic envelope with the inner payload as a JSON string; "n" notebooks each with nested arrays, Unicode and emoji; `b.Loop`, `-benchtime=2s -count=3`, `CGO_ENABLED=0`; times are the median-ish of 3 runs, rounded):

| Method | n=10 (1.9 KB outer) | n=200 (37 KB outer) | allocs (n=200) |
|---|---|---|---|
| `encoding/json` into `[]any`, then inner string into `any` (Go 1.27, v2-backed) | ~52 us | ~975 us | 13,801 |
| `encoding/json/v2` into `[]any` twice | ~28-35 us | ~700 us | 9,385 |
| `encoding/json/v2` with `jsontext.Value` lazy (outer split, inner top-level split only) | ~15-16 us | ~310 us | 33 |
| `goccy/go-json` into `[]any` twice | ~19 us | ~365-395 us | 2,911 |
| `bytedance/sonic` into `[]any` twice | ~31-34 us | ~640-700 us | 5,833 |

Reading it honestly:
- Everything is under 1 ms for a 37 KB response. A real Google round trip is tens to hundreds of milliseconds **[unverified; measure against live NotebookLM]**. The JSON library will not change perceived latency.
- The big win is *laziness*, not the library: parsing into `jsontext.Value` and descending only into the rows you need cut allocations from 9,385 to 33 and time by more than half versus `[]any`.
- Go 1.27's `encoding/json` is itself backed by v2 and claimed "significantly faster" unmarshal than before; marshal at parity. Source: https://go.dev/doc/go1.27 ("New encoding/json/v2 and encoding/json/jsontext packages"). Release notes list `GOEXPERIMENT=nojsonv2` as an opt-out expected to be removed later. Measured here: with `nojsonv2`, `encoding/json/jsontext` fails to build ("build constraints exclude all Go files"), so do not set it.
- v2 stricter defaults to know: rejects invalid UTF-8 and duplicate object names, nil slices marshal as `[]`, case-sensitive field matching, redefined `omitempty`. Sources: https://go.dev/blog/jsonv2-exp and Go 1.27 notes. Handle by decoding lenient inputs via `jsontext` options if Google emits odd UTF-8. **[unverified whether Google responses ever contain lone surrogates or invalid UTF-8; test on recorded fixtures.]**
- The package docs for `encoding/json/v2` in Go 1.27 carry no "experimental" marker (checked with `go doc encoding/json/v2` and `encoding/json/jsontext` locally). I treat both as stable in 1.27. **[Confirm the compatibility promise on go.dev before pinning API usage.]**

Alternatives, portability:
- sonic: amd64/arm64 only with a JIT; on unsupported environments it falls back to `encoding/json`; Go 1.24.0 unsupported without `-checklinkname=0`; its README lists Go 1.18 to 1.27. First-call JIT warm-up for big schemas ("Pretouch"). Source: https://raw.githubusercontent.com/bytedance/sonic/main/README.md. It is not the fastest on my payload. Reject: JIT, linkname fragility, no gain.
- goccy/go-json: pure Go, README claims drop-in and Go 1.27 compatibility; "personal project in spare time". Latest tag v0.11.2 (2026-09-30). Source: https://raw.githubusercontent.com/goccy/go-json/master/README.md and `gh api` release data. Faster than `[]any` v2 but not than lazy v2. Reject: extra dependency for no meaningful win.

**Recommendation:**
1. Stdlib only: `encoding/json/v2` + `jsontext`.
2. Write a small `batchexec` package: strip `)]}'`, split lines, for each row find `wrb.fr` with the wanted rpcId, hold the third element as `jsontext.Value`, unquote to the inner JSON, then decode positional arrays on demand via small accessor helpers (`path(0,2,1)`), not giant structs. Positional arrays are fragile by nature; accessors with explicit "missing field" errors localise drift.
3. Fuzz the splitter (`go test -fuzz`) and keep recorded sanitized fixtures.

**Measure:** `BenchmarkParseEnvelope` over sanitized real responses (n small, medium, 500 KB); `benchstat` (section 9). Target: parse p50 under 1 ms for a 40 KB response **[proposal]**, allocs/op tracked for regressions.

---

## 4. Concurrency and throttling

### What the server enforces (this bounds everything)
- Google's help page lists per-plan notebook and source limits and says "daily quotas refresh every 24 hours" and monthly every 30 days; the page I fetched gave no per-day chat figure. Source: https://support.google.com/notebooklm/answer/16213268.
- Upstream says the free tier has roughly 50 queries/day, "approximate, not officially documented": https://github.com/jacob-bd/notebooklm-mcp-cli/blob/main/docs/KNOWN_ISSUES.md. **[unverified by Google.]**
- So throttling is mostly quota, not request rate. No backoff cures a day-quota. Surface a clear error and stop retrying.

### Recommendations
1. **Singleflight for idempotent reads.** `golang.org/x/sync/singleflight`: `Do` collapses concurrent callers on one key; `DoChan` returns a channel; `Forget` drops a key. Source: https://pkg.go.dev/golang.org/x/sync/singleflight. The page says nothing about context. Design rule: run the shared call with a context detached from any single caller (e.g. `context.WithoutCancel` plus its own timeout) and let each waiter `select` on its own `ctx.Done()` using `DoChan`. Otherwise one cancelled caller kills the result for the others. **[design reasoning, not from docs.]** Key = rpcId + normalized params. Never coalesce mutations.
2. **Concurrency cap via `errgroup.SetLimit`** for fan-out (e.g. fetch N sources' details): `errgroup.WithContext`, `SetLimit(n)`, first error cancels the group context. Source: https://pkg.go.dev/golang.org/x/sync/errgroup. Use a small N (3 to 4) **[proposal]**.
3. **Token bucket with `golang.org/x/time/rate`**: `rate.NewLimiter(r, b)`, call `Wait(ctx)`; it errors if the wait would exceed the context deadline, which gives fast failure rather than a hung tool call. Source: https://pkg.go.dev/golang.org/x/time/rate. Two limiters: global requests/sec to the batchexecute host, and a stricter one for "expensive" RPCs (chat, artifact generation). Starting values **[proposal]**: 3 req/s burst 5 global; 1 per 2 s for chat. Tune against observed 429s. Both libs are already indirect deps of go-sdk (x/time v0.15.0, x/sync v0.20.0 in its go.mod), so zero added weight.
4. **Retry with exponential backoff plus jitter, honour `Retry-After`, only for retry-safe failures.** Upstream's chosen parameters: base 1.0 s, cap 30 s, jitter ratio 0.2, `Retry-After` honoured first, 429 and 5xx handled separately, retry sleeps clamped by the call's overall deadline: https://github.com/teng-lin/notebooklm-py/blob/main/src/notebooklm/_web/transport/middleware/retry.py. Copy that policy shape (behaviour, not code).
   - **Idempotency rule from the same file:** replaying a request that may have been transmitted can duplicate side effects (their streamed chat disables replay because "replaying a sent POST can record a duplicate answer"); retry only when the failure was provably before transmission (connect error, connect timeout, pool timeout). Mirror that: reads retry freely; chat/create/add-source retry only on pre-send failures.
   - Implement ~30 lines yourself; do not add a dependency. If you want one: `cenkalti/backoff` **[latest version unverified; GitHub API returned no "latest release"]**.
5. **Circuit breaker** (`sony/gobreaker/v2`, generics: `NewCircuitBreaker[T]`, settings `MaxRequests`, `Interval`, `Timeout` default 60 s, `ReadyToTrip` default more than 5 consecutive failures). Source: https://raw.githubusercontent.com/sony/gobreaker/master/README.md. **[Latest tag unverified.]** Use it on the RPC transport to decide when to try the browser fallback.
   - **What should trip it:** protocol drift signals (unparseable envelope, missing expected `wrb.fr` row, repeated 400 with a stale `bl`, response shape mismatch) and auth failures after one refresh attempt.
   - **What must not trip it:** 429 and quota errors. The browser fallback uses the same account and quota, so falling back on a throttle only adds ~seconds of latency and a second throttled request. Handle 429 with backoff and a user-visible message.
   - Half-open probe: a cheap read RPC (list notebooks) before routing real traffic back.
6. **Refresh `bl` and CSRF once on a drift/401-like error** before tripping (upstream auto-extracts both: KNOWN_ISSUES.md and AUTHENTICATION.md above).

**Measure:** fake upstream (`httptest`) that injects 429 with `Retry-After`, 5xx, slow responses, malformed envelopes; assert (a) retry count and total sleep bounded by deadline, (b) no retry of non-idempotent after send, (c) breaker opens on malformed envelopes within N calls and not on 429, (d) in-flight never exceeds cap. Go 1.27 adds `testing/synctest` helpers (`Sleep`) and `httptest.NewTestServer` with an in-memory network, which make backoff tests instant and deterministic: https://go.dev/doc/go1.27.

---

## 5. Caching

**Recommendation: tiny in-process TTL cache plus singleflight. On-disk cache only for expensive, rarely-changing, large content.**
- Data volume is small: free tier max 100 notebooks and 50 sources per notebook; top tiers 500 notebooks and up to 600 sources. Source: https://support.google.com/notebooklm/answer/16213268. A `map` + `sync.RWMutex` + expiry timestamp is enough. Libraries if you want eviction: `maypok86/otter` v2.3.0 (2025-12-22) or `dgraph-io/ristretto` v2.4.2 (2026-07-07) or `hashicorp/golang-lru` v2.0.7 (2023-09-29, stable but old). Versions from `gh api repos/<r>/releases/latest`. I would not add any of them for this data size **[judgement]**.
- TTLs **[proposals]**: notebook list 30 s; notebook detail and source list 60 s; source full-text (immutable per source version) 10 min in memory; never cache chat answers or generation status.
- **Invalidation:** write-through. Any mutating tool (create, rename, delete, add source, delete source, share) invalidates the affected keys (list + that notebook) in the same handler, before returning. External changes (web UI) are covered only by TTL; that is the accepted staleness. Upstream exposes `CHECK_SOURCE_FRESHNESS` (`yR9Yof`) for URL/Drive sources: https://github.com/teng-lin/notebooklm-py/blob/main/docs/rpc-reference.md. Expose a `refresh: true` argument that bypasses the cache.
- **Stale-while-revalidate** for lists: return cached value immediately, refresh in the background through singleflight when older than half the TTL **[proposal]**. This is the biggest perceived-latency win on repeated "list my notebooks" calls.
- **On-disk cache:** `os.UserCacheDir()` with 0600 perms; store only source full-text and artifact metadata keyed by (notebookId, sourceId, version). It contains private user data, so document it, make it opt-in or provide a `clear_cache` tool, and never put credentials there. **[Persistence format unverified; JSON files are enough at this scale.]**
- MCP-side hint: v1.8.0 added `SetCacheable` for list/read results so clients can cache with a TTL: https://api.github.com/repos/modelcontextprotocol/go-sdk/releases/latest and docs/server.md above. Set small TTLs on `tools/list` only if tool set is static.

**Measure:** hit ratio, `BenchmarkCacheHit` (target under 1 us per hit **[proposal]**), and an integration test proving invalidation after each mutating tool.

---

## 6. Browser fallback

### chromedp vs go-rod
| | chromedp | go-rod |
|---|---|---|
| Latest release | v0.21.0, 2026-10-11 (`gh api`) | v0.116.2, 2024-07-12 (`gh api`) |
| Activity | repo pushed 2026-10-11; `go.mod` requires Go 1.25: https://github.com/chromedp/chromedp | Latest non-"update sponsors" commits: 2026-02-14 "doc", 2025-09-18 logo, then 2024-12-07 code changes (`gh api repos/go-rod/rod/commits`) |
| Browser provisioning | Finds an installed browser; does **not** download. Source (allocate.go): https://github.com/chromedp/chromedp/blob/master/allocate.go searches `PATH` and, on Windows, Chrome Program Files paths, Chromium, and Edge (`msedge.exe`) | `launcher.NewBrowser().Get()` auto-downloads a pinned Chromium snapshot from `storage.googleapis.com/chromium-browser-snapshots` (races mirrors); default revision 1321438; default dir `$HOME/.cache/rod/browser`: https://github.com/go-rod/rod/blob/main/lib/launcher/browser.go and revision.go |
| Windows | Default transport is pipes; works on Windows without extra setup per README: https://raw.githubusercontent.com/chromedp/chromedp/master/README.md | Not mentioned in README |
| API | Action-list style (`chromedp.Run(ctx, actions...)`), typed CDP via `cdproto` | Chained high-level objects, auto-wait, thread-safe per README: https://raw.githubusercontent.com/go-rod/rod/main/README.md |
| Deps | `cdproto` plus go-json-experiment | stdlib-heavy |

**Recommendation: chromedp, using the user's installed Chrome/Edge (or `chrome-headless-shell` in Docker).**
- Why: actively released and tracks current Go; no multi-hundred-MB Chromium download inside our process (keeps the npm/PyPI wrappers small and our 8 MB binary honest); Edge on Windows is a built-in fallback so Windows users nearly always have a usable browser. Rod's pinned revision 1321438 is old, and the release cadence stopped in 2024. **[Rod's revision date not verified; the number is from its source file.]**
- Docker: the `chromedp/headless-shell` image contains a smaller headless Chrome that chromedp finds by default. Source: chromedp README above. Caveat: this means the container image is **not** `FROM scratch`; only the RPC-only image can be.
- Headless shell vs new headless: Chrome says `chrome-headless-shell` is "in some ways more performant" and has fewer dependencies, but new headless is the real browser and more authentic. No size or speed numbers given. Source: https://developer.chrome.com/blog/chrome-headless-shell. Download via Chrome for Testing / `@puppeteer/browsers install chrome-headless-shell@stable` (same page). **[Whether Google sign-in works in headless-shell is unverified; assume login happens in a real, visible Chrome once, and the fallback reuses the profile.]**

### Lazy launch, warm browser, pool
- **Never launch at startup.** The browser is the fallback; start it on first fallback (breaker open or RPC unsupported for that feature) and keep it alive for an idle window (e.g. 5 min **[proposal]**), then kill it.
- One browser, many tabs: call `chromedp.Run` once on a parent context to start the browser, then `chromedp.NewContext(parent)` per goroutine for separate tabs; don't share one context between goroutines. Source: https://pkg.go.dev/github.com/chromedp/chromedp (NewContext docs, fetched). Keep 1 warm tab; cap tabs at 2 to 3 **[proposal]**.
- Keep one logged-in page on the NotebookLM home, to avoid cold SPA load per call. **[Gain unmeasured.]**

### Speeding page loads
- `network.SetBlockedURLs` (CDP) blocks URLs from loading: https://pkg.go.dev/github.com/chromedp/cdproto/network#SetBlockedURLs. Use patterns for images/fonts/media. Request interception via the Fetch domain also works but adds a round trip per request **[unverified cost]**. Rod equivalent: request hijacking **[Rod doc page not retrieved; unverified]**.
- Do not block scripts/XHR/CSS the SPA needs. Test that the NotebookLM UI still functions with images and fonts blocked. **[Unknown; must be tested.]**
- chromedp's default flags are tuned for scraping: `disable-dev-shm-usage`, `enable-automation`, `password-store=basic`, `use-mock-keychain`, etc. (allocate.go above). `enable-automation` is a likely trigger for Google's "this browser may not be secure" sign-in block. **[unverified; I found no primary source. Mitigate by doing interactive login in a normal, non-automation-launched Chrome and only attach with automation afterwards.]**

### Persistent user-data-dir
- Chrome 136+ ignores `--remote-debugging-port`/`--remote-debugging-pipe` on the default profile; they only work with a non-default `--user-data-dir`. Source: https://developer.chrome.com/blog/remote-debugging-port. So: a dedicated profile dir under our config dir (upstream does the same, `nlm login` dedicated profile: AUTHENTICATION.md above). Never point at the user's real Chrome profile.
- Only one Chrome process may own a user-data-dir; guard with a lock file and a single launcher. Profile dir holds Google cookies: 0700 perms, document it.
- Upstream also notes login waits for signed-in cookies and the CSRF token, not just a 200 or an anonymous `NID` cookie (AUTHENTICATION.md above).

**Measure:** cold fallback (no browser) and warm fallback (browser + tab alive) wall time for 3 representative operations; RSS of the Chrome process tree (Windows: sum of child processes). Target **[proposal]**: warm-fallback overhead over RPC under 3 s per call; idle-reaped to 0 MB after 5 min.

---

## 7. Cookie and auth handling

Upstream facts:
- Google session cookies from a signed-in browser plus a CSRF token (`at`/`SNlM0e`-style) and build label `bl` taken from the app page. Cookies "typically expire after 2-4 weeks" per upstream's observations. Sources: https://github.com/jacob-bd/notebooklm-mcp-cli/blob/main/docs/KNOWN_ISSUES.md and docs/AUTHENTICATION.md.
- Short-lived freshness cookies: `__Secure-1PSIDTS`/`__Secure-3PSIDTS` rotate; pure RPC traffic to the app host never triggers rotation, so long-lived stored sessions go stale; upstream POSTs to `accounts.google.com/RotateCookies`, whose response declares a 600 s next-rotation interval, and rate-limits its own poke to once per 60 s. Source: https://github.com/teng-lin/notebooklm-py/blob/main/src/notebooklm/_auth/keepalive.py. **This is reverse-engineered behaviour; it may change.**
- Device-bound sessions: upstream mentions DBSC-bound vs unbound profiles. DBSC binds a short-lived cookie to a hardware-held key, designed so exfiltrated cookies are useless on another device; origin trial from Chrome 135. Source: https://developer.chrome.com/blog/dbsc-origin-trial. **Risk: if Google enforces DBSC for NotebookLM, cookie replay from our Go client stops working and the browser path becomes the only path.** Upstream says its keepalive POST worked for both bound and unbound profiles at the time of writing.

**Recommendations**
1. **Acquire:** login flow launches a visible, normally-started Chrome/Edge with a dedicated `--user-data-dir`, user signs in, then we read cookies over CDP (`network.GetCookies`/storage) and fetch the app page for CSRF and `bl`. Same approach as upstream. Don't decrypt Chrome's on-disk cookie DB (app-bound encryption, fragile, reads like credential theft).
2. **Store:** `zalando/go-keyring` v0.2.8 (2026-03-23): macOS via `/usr/bin/security`, Linux via Secret Service/D-Bus, Windows via Credential Manager. API `Set/Get/Delete/DeleteAll`, `MockInit()` for tests. Source: https://pkg.go.dev/github.com/zalando/go-keyring and its README. **Size limits matter: Windows password max 2,560 bytes, macOS about 3,000 bytes combined, Linux fine below about 100 KiB** (same page). A full Google cookie jar can easily exceed 2,560 bytes **[unverified size; measure your real jar]**. Pattern: generate a 32-byte key, store *that* in the keyring, keep the cookie jar as an AES-GCM-encrypted file (0600) in `os.UserConfigDir()`. **[design proposal.]**
3. **Headless Linux / Docker:** no Secret Service. Fall back to an encrypted-at-rest file with a key from env (`NOTEBOOKLM_SESSION_KEY`) or a mounted secret; or accept cookies via env var, which upstream also supports (`NOTEBOOKLM_COOKIES`, KNOWN_ISSUES.md). Never bake cookies into images; CLAUDE.md already forbids committing them.
4. **Detect expiry:** (a) redirect (3xx) to `accounts.google.com` or a login HTML body where `)]}'` is expected; (b) missing CSRF token on the app page; (c) an RPC returning an auth-class error. On (a)/(b): refresh `bl`/CSRF once, then attempt `RotateCookies` once, retry once; if still failing return a tool error "session expired, run login". Do not loop.
5. **Refresh proactively but sparingly:** rotate at most every 10 minutes while the server is active and persist the updated jar atomically (write temp file then rename) when Set-Cookie changes it, with a file lock for multiple server instances. Upstream uses a storage lock for the same reason (keepalive.py above).
6. **Never log** cookie or token values; redact in `httptrace` output.

**Measure:** a login-expiry simulation test (fake server returning 302 to accounts.google.com) asserting exactly one refresh attempt and a typed `ErrSessionExpired`; keyring round-trip test using `go-keyring`'s `MockInit()`; measure real cookie jar size against the Windows 2,560-byte cap.

---

## 8. Binary, build, startup

| Item | Recommendation | Evidence |
|---|---|---|
| Static build | `CGO_ENABLED=0` | Measured here: go-sdk server builds with `CGO_ENABLED=0` (237 packages). go-keyring is pure Go (README says it aims at statically linked binaries: https://raw.githubusercontent.com/zalando/go-keyring/master/README.md). chromedp is pure Go. |
| Strip/trim | `-trimpath -ldflags="-s -w"` | Measured here (20-tool go-sdk server, Windows amd64): 12,038,656 B plain vs 8,288,256 B with `-trimpath -ldflags="-s -w"`, about 31% smaller. **Startup unchanged**: initialize p50 16.5 to 17.7 ms either way (17.3 ms plain). So these flags are for download size, not speed. GoReleaser's default ldflags already include `-s -w`; it recommends adding `-trimpath` via `flags`: https://goreleaser.com/customization/builds/go/ |
| PGO | Do not start with PGO. Revisit after real CPU profiles exist | `default.pgo` in the main package is picked up automatically (`-pgo=auto` since Go 1.21); Go docs claim 2-14% (as of Go 1.22) on representative CPU-bound programs, with higher build time and slightly larger binaries; collect profiles from production, not microbenchmarks: https://go.dev/doc/pgo. Original blog: typically 2-7% CPU: https://go.dev/blog/pgo. Our process is network-bound; expected gain is small **[unverified; the profile will tell]**. If adopted: commit `default.pgo` for the main package, refresh per release, build all release targets with it, and A/B with benchstat. |
| GOAMD64 | Keep default v1 | Levels: v1 baseline, v2 adds SSE4.2/POPCNT, v3 adds AVX2/BMI/FMA etc.; a v3 binary will not start on CPUs lacking them. Source: https://go.dev/wiki/MinimumRequirements. Our hot path is TLS and I/O; no AVX2 gain worth breaking old machines. Go TLS/crypto already select CPU features at runtime **[unverified; standard library behaviour, not checked here]**. |
| UPX | Do not use | UPX claims 50-70% size reduction and "no runtime or memory penalty for most formats": https://upx.github.io/. It says nothing about Go. My expectation is slower cold start (full decompress before run), higher RSS (pages not file-backed/shared), and antivirus false positives on Windows **[unverified for Go; measure before ever adopting]**. A 8 MB binary does not need it. |
| Go 1.27 runtime | Build with 1.27 | Release notes: size-specialised small allocations, up to 30% cheaper for small (under 80 byte) allocs, about 1% real-program gain, about 60 KB larger binary: https://go.dev/doc/go1.27. Go 1.26: Green Tea GC default, 10-40% lower GC overhead on GC-heavy programs; faster cgo; `io.ReadAll` about 2x faster; `B.Loop` no longer blocks inlining: https://go.dev/doc/go1.26. Our heap is small, so GC gains are minor. |
| Memory | Set a soft limit for the Docker/HTTP mode | `debug.SetMemoryLimit` / `GOMEMLIMIT` **[unverified in this research; standard runtime knob]**. Measured here: hello-world server peak working set 11.8 MB. |
| Startup | Lazy init everything that is not needed for `initialize` | Register tools at startup (cheap), but do not read the keyring, parse cookies, open network, start browser, or load the cache until first use. Measure with `GODEBUG=inittrace=1` (https://go.dev/doc/diagnostics): measured here, the largest init in the hello server was `net` at about 1.0 ms. |

Measured cold start (Windows 11, spawn via Go harness, 30 runs after 1 warmup, includes process creation and pipe round trip):
- `initialize` p50 16.5 to 17.7 ms, p95 17.5 to 19.1 ms.
- `initialize` + `tools/list` (20 tools) p50 17.5 to 18.7 ms.
- Peak working set p50 11.8 MB.
Caveat: Windows Defender or SmartScreen on first run of a freshly downloaded binary can add large one-time delay **[unverified; observed behaviour class, not measured]**. Sign Windows binaries if possible.

---

## 9. Profiling and benchmark toolchain

**Tools**
- `testing.B` with **`b.Loop()`** (resets timer on first call, stops at end, and since Go 1.26 no longer prevents inlining in the loop body): `go doc testing.B.Loop`, https://go.dev/doc/go1.26.
- **benchstat**: run each benchmark at least 10 times, ideally 20; compare files; reads medians with 95% CI, a `~` means no significant difference; do not rerun until you see a change: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat.
- **pprof**: `go test -cpuprofile cpu.out -memprofile mem.out -bench .`; heap, goroutine, block, mutex profiles; `net/http/pprof` for a running server: https://go.dev/doc/diagnostics. Go 1.26 made the pprof web UI default to flame graph; Go 1.27 added a `goroutineleak` profile: go.dev/doc/go1.26, go.dev/doc/go1.27.
- **runtime/trace** for scheduling/latency (not hotspots): https://go.dev/doc/diagnostics.
- **httptrace** for connection reuse/DNS/TLS: same page.
- **inittrace**: `GODEBUG=inittrace=1` for package-init cost: same page.
- **testing/synctest** for deterministic time (backoff, TTL): https://go.dev/doc/go1.27.

**Proposed benchmark suite** (all targets are **proposals**; baseline column is measured here where available)

| # | Benchmark | How | Baseline (measured here) | Proposed target |
|---|---|---|---|---|
| B1 | Cold start to `initialize` response | Go harness spawns binary, writes initialize, times first response line; 30 runs, drop first | 17 ms p50, 19 ms p95 (hello, 20 tools) | p50 under 30 ms, p95 under 60 ms on a dev laptop |
| B2 | Idle RSS (RPC mode, no browser) | Windows `GetProcessMemoryInfo` PeakWorkingSetSize, Linux `/proc/<pid>/status` VmRSS | 11.8 MB (hello) | under 25 MB after listing 100 notebooks |
| B3 | Tool-call overhead excluding Google | In-memory MCP transport + `httptest` fake upstream returning recorded envelopes; time `CallTool` | not measured | p50 under 2 ms, p95 under 5 ms for list-notebooks; allocs/op tracked |
| B4 | Envelope parse | `BenchmarkParseEnvelope` on 2 KB, 40 KB, 500 KB fixtures | 15 us (2 KB), 310 us (37 KB) lazy v2 | 40 KB under 1 ms, allocs under 100 |
| B5 | Cache | hit, miss+singleflight under 50 concurrent callers | not measured | hit under 1 us; 50 concurrent identical reads produce 1 upstream call |
| B6 | Connection reuse | `httptrace` over 100 sequential calls to fake HTTP/2 server | not measured | `Reused` on 99 of 100; one TLS handshake |
| B7 | Throttle handling | fake upstream with 429+Retry-After; measure total delay | not measured | retries honour Retry-After; total wait under call deadline |
| B8 | Browser fallback | cold and warm, 3 operations | not measured | warm overhead under 3 s over RPC; Chrome tree reaped after idle |
| B9 | Binary | size of release artifacts per platform | 8.3 MB hello (stripped, trimpath) | under 15 MB stripped for the full server |
| B10 | Live smoke (manual, not CI) | real NotebookLM, list notebooks end to end | not measured | record p50/p95 Google RTT separately from our overhead |

CI: run B1, B3, B4, B5 on every PR with `-count=10`, compare against the main-branch baseline with benchstat, and fail only on regressions that benchstat marks significant and above a threshold (for example 20%) **[proposal]**. CI runner noise is high; keep the threshold generous.

---

## 10. Release engineering

### GoReleaser (v2.18.3, 2026-10-09 via `gh api`)
- Builds: defaults to darwin/linux/windows and 386/amd64/arm64; `env: CGO_ENABLED=0`; `goamd64` default v1; `mod_timestamp` for reproducibility; default ldflags `-s -w -X main.version=...`; add `-trimpath` in `flags`. Source: https://goreleaser.com/customization/builds/go/
- SBOM: default uses Syft, one SBOM per archive; `sboms:` block, e.g. `cyclonedx-json`. Docker images from GoReleaser are not cataloged by this step. Source: https://goreleaser.com/customization/sbom/
- Signing: sign the checksum file with cosign using `sign-blob --bundle=${signature}`; verify with `cosign verify-blob` using `--certificate-identity` and `--certificate-oidc-issuer https://token.actions.githubusercontent.com`. Source: https://goreleaser.com/customization/sign/
- Docker: `dockers_v2` (since v2.12) uses buildx for multi-arch (`linux/amd64`, `linux/arm64`), copies prebuilt binaries into e.g. `FROM scratch`. Pro-only: templated Dockerfile/extra files, `--split`/`--prepare`. Source: https://goreleaser.com/customization/dockers_v2/. For the browser-fallback image use a base that contains Chrome (see section 6).
- **npm publishing in GoReleaser is Pro-only** ("exclusively available with GoReleaser Pro", since v2.8) and works by a `postinstall` script that downloads the archive, so it breaks under `--ignore-scripts`. Source: https://goreleaser.com/customization/npm/. Plan around OSS only.

### npm: per-platform `optionalDependencies`
- esbuild: root package lists one `@esbuild/<os>-<cpu>` package per platform under `optionalDependencies`, plus `postinstall: node install.js` and `bin: bin/esbuild`. Source: https://raw.githubusercontent.com/evanw/esbuild/main/npm/esbuild/package.json. Docs: npm installs only the matching optional package; with `--ignore-scripts` the JS shim stays (slower start) but works; with `--no-optional` the script tries a registry download; with both, no binary. Source: https://esbuild.github.io/getting-started/#install-the-native-executable
- Biome: same pattern with `@biomejs/cli-<os>-<cpu>[-musl]` optionalDependencies and `bin: bin/biome`, no scripts field in the root. Source: https://raw.githubusercontent.com/biomejs/biome/main/packages/@biomejs/biome/package.json.
- **Recommendation:** follow Biome's variant: no postinstall; each platform package sets `os` and `cpu` fields (the mechanism behind optional selection; the esbuild docs page does not spell it out, so **[confirm against npm docs]**), a tiny JS launcher resolves `require.resolve('@scope/pkg-<platform>/bin/notebooklm-mcp')` and `spawn`s it with `stdio: 'inherit'`. Cold start overhead of Node launcher is real (tens of ms) **[unverified, measure]**; document `npx` as the convenience path and the direct binary as the fast path. Add musl packages for Alpine like Biome.
- The `gowheels` tool can also emit npm packages in the esbuild pattern: https://pkg.go.dev/github.com/StevenACoffman/gowheels (v0.2.0, April 2026, MIT, young; vet before depending).

### PyPI: platform wheels carrying the Go binary
- `simonw/go-to-wheel`: one wheel per target, bundles the binary and a small Python wrapper that `execvp`s it (Unix) or uses `subprocess` (Windows). Source: https://simonwillison.net/2026/Feb/4/distributing-go-binaries/. The author calls it "a tiny bit abusive" and had tested only one platform's wheel. AI-assisted code: review it.
- `gowheels` tag table (static `CGO_ENABLED=0` binary yields both manylinux and musllinux wheels): `manylinux_2_17_x86_64.manylinux2014_x86_64`, `musllinux_1_2_x86_64`, aarch64 equivalents, `macosx_10_9_x86_64`, `macosx_11_0_arm64`, `win_amd64`, `win_arm64`. Source: https://pkg.go.dev/github.com/StevenACoffman/gowheels. Cross-check the tag grammar against https://packaging.python.org/en/latest/specifications/platform-compatibility-tags/ (macOS `macosx_11_0_arm64` form confirmed there; manylinux/musllinux schemas confirmed).
- Another option, `bin2whl` (puts binaries directly in the venv `bin/`, no Python wrapper), found via search, not read: https://pypi.org/project/bin2whl/ **[unverified]**.
- **Recommendation:** generate wheels in the same GitHub Actions job from the GoReleaser archives, tag them `py3-none-<platform>`, test by `pip install` into a clean venv per OS runner, and publish with PyPI trusted publishing **[trusted publishing not researched here]**. The wrapper adds a Python interpreter start (about tens of ms **[unverified]**); using an entry-point script that `exec`s the binary on Unix avoids a lingering Python parent.

---

## Contradictions and risks vs the planned architecture

1. **The browser fallback does not protect against throttling.** Same account, same server-side daily quota (https://support.google.com/notebooklm/answer/16213268). It protects against protocol drift and some auth failures. Do not trip the RPC-to-browser breaker on 429.
2. **"NotebookLM has no stable public consumer API" (CLAUDE.md) is true for consumers, but an official API exists for enterprise.** NotebookLM Enterprise: Discovery Engine REST at `https://{us|eu|global}-discoveryengine.googleapis.com/v1alpha/...`, Bearer token from `gcloud auth print-access-token`, notebooks create/get/listRecentlyViewed/batchDelete/share, in Preview, sources and audio on separate pages. Source: https://docs.cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/api-notebooks. Consider a third transport behind the same interface for Workspace/Enterprise users; it needs no cookies. Upstream `notebooklm-py` also ships an Android gRPC client (`_android/`, proto files) as an alternative transport: https://github.com/teng-lin/notebooklm-py/blob/main/docs/rpc-reference.md **[unverified, not studied]**.
3. **Cookie replay may stop working (DBSC)**, making the browser path (or the official API) the only viable path. https://developer.chrome.com/blog/dbsc-origin-trial.
4. **Chrome 136 blocks debugging flags on the default profile**, so the browser fallback can never reuse the user's everyday Chrome login; it needs its own signed-in profile. https://developer.chrome.com/blog/remote-debugging-port.
5. **"Single static binary" is true for the RPC path only.** The browser fallback needs Chrome/Edge installed (or a bigger Docker image). The npm/PyPI wrappers do not ship a browser. Rod would download one; chromedp will not.
6. **GoReleaser OSS cannot publish npm**; its Pro npm feature also relies on postinstall downloads.
7. **go-keyring Windows limit (2,560 bytes) probably cannot hold a full cookie jar.** Store an encryption key in the keyring and the jar in an encrypted file.
8. **Declared chunk lengths in batchexecute responses are unreliable**; any parser that trusts them will intermittently fail.
9. **Protocol version `2026-07-28` is stateless and removes ping/SSE resumability**; go-sdk v1.8.0 changed error behaviour for it. Test negotiation on stdio before release.
10. **Performance work that will not matter here:** PGO, GOAMD64 above v1, UPX, third-party JSON libraries. Spend the effort on caching, coalescing, retry correctness and keeping the browser cold.

## Reproducing my measurements
Throwaway code (not in this repo) lived in the session scratchpad: a 20-tool go-sdk v1.8.0 server, a Go harness that spawns it and times `initialize` and `tools/list` and reads `PeakWorkingSetSize` through `psapi.dll`, and a `b.Loop` JSON benchmark importing `encoding/json`, `encoding/json/v2`, `goccy/go-json`, `bytedance/sonic`. Recreate them under `bench/` in the real repo when the toolchain is chosen, and add them to the build commands in CLAUDE.md.
