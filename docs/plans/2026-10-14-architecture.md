# Architecture spec: Go NotebookLM MCP server

Date: 2026-10-14. Status: **approved by owner 2026-10-14**; defaults D1–D15 accepted; name sourcebolt.
Sources: [feature-matrix](../../research/feature-matrix.md), [rpc-catalog](../../research/rpc-catalog.md), [best-practices](../../research/best-practices.md), [performance](../../research/performance.md), [decisions-needed](../../research/decisions-needed.md).

## 1. Goal

One MCP server for Google NotebookLM that covers the union of upstream features (33 tools), answers faster than any upstream, and survives Google wire drift. One static Go binary for the RPC path. Released on GitHub, Docker, npm and PyPI.

## 2. Locked decisions

| Area | Decision |
|---|---|
| Language | Go 1.27, `CGO_ENABLED=0` |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` v1.8.0; stdio default; streamable HTTP opt-in (loopback, bearer token, Origin check) |
| Transports | 1. web RPC (`batchexecute` + `GenerateFreeFormStreamed` + Scotty upload). 2. browser fallback (chromedp). 3. Enterprise REST: interface slot only, ships after 1.0 (D2-B) |
| Fallback | Only on transport, drift and cookie-binding errors (D9-A). Never on 429, quota, auth (16), permission, not-found, invalid-argument |
| Host | Default `notebook.google.com`; per-profile learned host; also `notebooklm.google.com`, `notebook.cloud.google.com` |
| License | Apache-2.0 + `NOTICE` crediting adapted MIT/Apache sources. GPL/no-license upstreams: behavior only |
| JSON | stdlib `encoding/json/v2` + `jsontext`, lazy decode |
| Browser | chromedp, user-installed Chrome/Edge or `chrome-headless-shell` in Docker; dedicated profile dir |

## 3. Defaults adopted from research (accepted by owner)

D1-A no Android schema. D2-B Enterprise after 1.0. D3-B `cookies.txt` import in v1, Firefox in v1.x. D4-B no Chromium store decryption. D5-B MCP only (no REST). D6-A minimal CLI with `call`. D7-A refresh-command hook + headless re-mint on dedicated profile, no master token. D8-B stdlib TLS, uTLS optional flag. D9-A cookie-binding = transport error. D10-B in-page fetch, then DOM for chat + add-source only. D11-B 32 default tools, `minimal` 8, `batch` group off. D12-A no account rotation. D13-A fire-and-return + status tools. D14-A disk cache opt-in.

## 4. Package layout

Module: `github.com/candidloom-hub/sourcebolt` . Binary: `sourcebolt`.

```
cmd/sourcebolt/      CLI entry: serve, call, login, logout, status, doctor, capture
internal/wire/           batchexecute encode/decode, stream decode, gRPC status mapping, positional accessors
internal/rpcids/         RPC id registry + JSON override file
internal/session/        host, token bootstrap, cookie jar, import, encrypted store, rotation
internal/transport/      Transport interface, error classes, router (fallback policy)
internal/transport/rpc/  HTTP transport (tuned client, limiter, retries, Set-Cookie merge)
internal/transport/browser/  chromedp transport (in-page fetch + DOM for chat/add-source)
internal/nlm/            typed domain client: notebooks, sources, chat, research, studio, notes, labels, sharing, account
internal/cache/          TTL cache + singleflight + stale-while-revalidate
internal/resolve/        fuzzy id/title/prefix resolver
internal/jobs/           local job table for detached asks, waits, cancels
internal/mcpserver/      server, tool registration, profiles, error envelope, progress
internal/safety/         SSRF URL check, path allow-root check, log redaction
testdata/fixtures/       sanitized golden wire captures (request + response + metadata)
tools/capture/           HAR -> sanitized fixture converter
```

## 5. Core interfaces

```go
// internal/transport
type Call struct {
    RPC        string          // logical name, e.g. "notebooks.list"
    ID         string          // wire id resolved from rpcids
    Params     any             // positional payload, JSON-encodable
    SourcePath string          // "/" or "/notebook/{id}"
    Idempotent bool
}
type Result struct {
    Payload   jsontext.Value   // inner JSON of the wrb.fr frame (unquoted)
    Transport string           // "rpc" | "browser" | "enterprise"
}
type Transport interface {
    Name() string
    Do(ctx context.Context, c Call) (Result, error)
    Supports(rpc string) bool
}
```

Error classes (`internal/transport/errors.go`): `KindAuth`, `KindCookieBinding`, `KindDrift`, `KindTransport`, `KindRateLimited`, `KindQuota`, `KindPermission`, `KindNotFound`, `KindInvalidArgument`, `KindPending`, `KindUnsupported`, `KindInternal`. One type `*Error{Kind, RPC, GRPCCode, Message, Retriable bool, Transport string, Cause error}`. Router falls back only when `Kind ∈ {KindTransport, KindDrift, KindCookieBinding}`.

MCP error envelope (structured content of a failed tool result): `{status:"error", error, code, retriable, guidance, suggestions[], transport, retry_after_seconds}` (`retry_after_seconds` only when Google sent Retry-After).

## 6. Data flow (RPC path)

1. Tool handler resolves names to ids (`resolve`), checks cache for reads.
2. `nlm` builds the positional payload, calls `router.Do(ctx, Call)`.
3. RPC transport: limiter `Wait(ctx)` → session tokens (cached; refetched on drift/auth) → POST `/_/LabsTailwindUi/data/batchexecute?rpcids=..&source-path=..&f.sid=..&bl=..&hl=..&rt=c` → merge `Set-Cookie` → `wire.Decode`.
4. `wire.Decode` returns the frame for the id, or a typed error from position-5 status, `er` frame, missing id (drift) or no frames (signed out / anti-bot).
5. `nlm` decodes the payload lazily into typed structs; handler formats the result.

## 7. Auth ladder

On `KindAuth`: (1) re-bootstrap tokens from `GET /`; (2) `RotateCookies` (≤ 1 per 60 s per profile); (3) operator `NOTEBOOKLM_REFRESH_CMD`; (4) headless re-mint on the dedicated browser profile (opt-out); (5) return `KindAuth` with guidance `run: sourcebolt login`. Network and 5xx never enter the ladder. Credentials saved atomically with a revision counter.

Storage: cookie jar in an AES-256-GCM file under the user config dir; 32-byte key in the OS keyring (`zalando/go-keyring` v0.2.8). Keyring unavailable (headless Linux, Docker): key file with 0600 and a warning in `doctor`.

## 8. Performance targets (gate for 1.0; measured by Plan 5 benchmarks)

| # | Metric | Target |
|---|---|---|
| B1 | cold start to `initialize` | p50 < 30 ms, p95 < 60 ms |
| B2 | idle RSS after listing 100 notebooks, RPC mode | < 25 MB |
| B3 | tool-call overhead excluding Google (fake upstream) | p50 < 2 ms, p95 < 5 ms |
| B4 | decode 40 KB envelope | p50 < 1 ms |
| B5 | cache hit / 50 concurrent identical reads | < 1 µs / exactly 1 upstream call |
| B6 | connection reuse after first call | > 95% `GotConn.Reused` |
| B7 | binary size, stripped | < 20 MB |
| B8 | warm browser fallback overhead | < 3 s over RPC |
| B9 | streamed chat time-to-first-progress | ≤ Google first chunk + 50 ms |

## 9. Wire drift policy

- Every RPC payload builder and decoder has a golden fixture test. Fixtures start from upstream-documented shapes, marked `synthetic: true`. Plan 1 Task 13 replaces them with live captures (owner logs in; 40-item checklist in rpc-catalog §10).
- Decoders are lossless-checked: re-encoding a decoded fixture must equal the original; unknown slots are reported in tests, ignored at runtime.
- RPC ids overridable at runtime from `<config>/rpc-ids.json`. Missing id in a response = `KindDrift`.

## 10. Plans and model assignment

| Plan | Scope | Implementer model | Reviewer |
|---|---|---|---|
| [P1 core](2026-10-14-p1-core.md) | module, wire codec, session, RPC transport, cache, resolver, MCP skeleton (3 tools), CLI, capture + live smoke, CI | Sonnet per task | Opus whole-branch |
| [P2 sources, chat, research](2026-10-14-p2-sources-chat-research.md) | 11 tools | Sonnet | Opus |
| [P3 studio, notes, labels, sharing, account, delete](2026-10-14-p3-studio-and-rest.md) | 16 tools | Sonnet | Opus |
| [P4 browser fallback + login](2026-10-14-p4-browser-and-login.md) | chromedp transport, router, `auth_login`/`auth_logout` | Opus (DOM + concurrency) | Opus |
| [P5 release + perf gate](2026-10-14-p5-release.md) | benchmarks B1–B9, GoReleaser, Docker, npm, PyPI, docs | Sonnet; mechanical packaging on Haiku | Opus |

Order: P1 → (P2 ∥ P4 after P1 Task 11) → P3 → P5. Approval gates: plan approval; live capture needs owner login; making the repo public; every publish.
