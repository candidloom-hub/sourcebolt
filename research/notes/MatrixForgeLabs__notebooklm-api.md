# MatrixForgeLabs/notebooklm-api

- URL: https://github.com/MatrixForgeLabs/notebooklm-api
- Language / runtime: Rust 2024 edition, MSRV 1.85 (`Cargo.toml:4,12`); library `notebooklm_api` + CLI `notebooklm` (`src/bin/notebooklm.rs`, 1354 lines). Deps: `clap 4.5`, `reqwest 0.12` (rustls, http2), `serde_json`, `thiserror 2`, `tokio 1.44`, `regex`, `uuid`, `urlencoding`, `dirs` (`Cargo.toml:30-41`). Release profile `lto=fat`, `strip`.
- License: MIT (`Cargo.toml:5`, `LICENSE`).
- Stars / last commit: 1 star (per upstreams.md); HEAD `28d288d` 2026-02-26; 5 commits.
- Versions reviewed: tags v0.1.0-alpha (2026-02-18), v0.1.1-alpha (2026-02-26). 
- Size: `src/client.rs` 3465 lines (everything in one file), `types.rs` 709, `rpc.rs` 230, `auth.rs` 142, `error.rs` 55. Fixtures: 3 JSON files in `tests/fixtures/`.
- Description (README): "ports the Python `notebooklm-py` functionality to Rust". Not an MCP server.

## Access method

Internal batchexecute RPC (+ streaming chat endpoint). Cookies come from a Playwright/Chromium `storage_state.json`. Host `https://notebooklm.google.com` (old host) (`src/rpc.rs:4-6`).

## RPC catalog

IDs (`src/rpc.rs:42-77`), 30 methods, all consistent with the other repos: ListNotebooks wXbhsf, CreateNotebook CCqFvf, GetNotebook rLM1Ne, RenameNotebook s0tc2d, DeleteNotebook WWINqb, Summarize VfAZjd, AddSource izAoDd, DeleteSource tGMBJ, GetSource hizoJc, RefreshSource FLmJqe, UpdateSource b7Wfje, GetConversationHistory hPTbtc (named for the list-conversations RPC), CreateArtifact R7cb6c, ListArtifacts gArtLc, DeleteArtifact V5N4be, RenameArtifact rc3d8d, ExportArtifact Krh3pd, GenerateMindMap yyryJe, CreateNote CYK0Xb, UpdateNote cYAfTb, GetInteractiveHtml v9rmvd, GetNotesAndMindMaps cFji9, StartFastResearch Ljjv0c, StartDeepResearch QA9ei, PollResearch e3bVqc, ImportResearch LBwxtb, GetUserSettings ZwVcOc, SetUserSettings hT54vc, GetShareStatus JFMDGd, ShareNotebook QDyure. No new IDs.

Wire envelope (`rpc.rs:79-97`): `[[[id, "<params json>", null, "generic"]]]`, body `f.req=<urlencoded>&at=<token>&`. batchexecute URL params are only `rpcids`, `source-path`, `f.sid`, `rt=c` (`client.rs:2167-2178`) — **no `bl`, no `hl`, no `_reqid`**. Chat URL adds `bl` (default `boq_labs-tailwind-frontend_20251221.14_p0`, `client.rs:27`), `hl=en`, `_reqid` (starts at 100000 and advances by 100000 with an atomic counter) and `f.sid` (`client.rs:2180-2192`).

Decode (`rpc.rs:99-218`): strips `)]}'`, splits by lines ignoring declared lengths, picks the first frame whose id matches; `er` frames map ints 401/403/404/429 to messages; a null result is an error unless the caller passes `allow_null`. Does not read the in-band status at position 5.

Notebook list fixture (synthetic): `[[["thought\nMy Notebook", null, "nb_123", null, null, [null,false,null,null,null,[1700000000]]]]]` (`tests/fixtures/notebooks_list.json`) — i.e. entry = `[title, sources?, id, emoji?, ..., meta]`, and the title may carry a `thought\n` prefix that clients strip (from the Python lineage). This is the shape LocalKinAI/Dokkabei parse by index, while tmc's 2024 capture shows `[[id], title, ...]`; the layout has changed over time, so re-capture on day one.

## Streaming/chat

`query_call` posts `f.req=[null,"<params>"]&at=<csrf>&` to the GenerateFreeFormStreamed URL and returns the **entire body text**; parsing happens afterwards (no incremental stream). Includes a local `conversation_cache` of previous turns per conversation id so follow-ups can resend history (`client.rs:34,283-302`) — the same idea sahaand's WIP lists as missing. [Details of parsing not read.]

## Auth lifecycle

- `AuthTokens { cookies: HashMap, csrf_token, session_id }` loaded from `~/.notebooklm/storage_state.json` (`auth.rs:49-54`), cookie domains kept: `.google.com`, `notebooklm.google.com`, `.googleusercontent.com`, any `.google.*` (`auth.rs:88-93`). `SID` is mandatory (`:79-83`). Cookies are a `HashMap` so later duplicates overwrite earlier ones with no domain precedence.
- Tokens: `GET https://notebooklm.google.com/`; redirect URL containing `accounts.google.com` = expired; regexes for `SNlM0e` and `FdrFJe`; both mandatory (`auth.rs:95-142`). Note the regex requires no whitespace around `:` (`"SNlM0e":"..."`), stricter than tmc's.
- Refresh policy: on 401/403 it re-reads `storage_state.json` **from disk** and re-fetches tokens once; second failure -> `StaleAuth` (`client.rs:129-203`). It never relaunches a browser; login is delegated to Playwright. If the file did not change, refresh is pointless (the same cookies are retried).
- No DPAPI / cookie-DB decryption.

## Reliability

- `RetryPolicy { max_retries: 3, base_delay_ms: 250, max_delay_ms: 5000, jitter_ms: 200 }` (`types.rs:478-494`): exponential `base * 2^attempt` capped, plus pseudo-jitter from `SystemTime` sub-second nanos, plus `Retry-After` seconds (integer only) (`client.rs:2219-2250`). Retries transport errors (timeout, connect, request) and HTTP 429 + all 5xx; same retry loop for RPC, query and download. Applies to non-idempotent calls too.
- Typed errors with `thiserror`: `Auth, RateLimit{retry_after}, Client{status}, Server{status}, Config, Network{operation}, Timeout{operation}, StaleAuth, Rpc{method_id, message, code}, RpcDecode, DecodeShape{path, context}` (`error.rs:1-55`). The `DecodeShape{path, context}` variant ("unexpected response shape at path X") is a good idea for positional-array parsing failures. 
- Parse helper `json_at(value, &[i, j, k])` safe path indexing used everywhere (`client.rs` tests, e.g. `json_at(&payload,&[2,7,1,0])`).
- HTTP client timeout 30 s fixed (`client.rs:58,123`); chat can legitimately take longer (tmc uses 5-15 min).

## Performance notes

One `reqwest::Client` with default headers (Content-Type, Cookie); cloned per call. Single 3.4k-line `client.rs`. No measurements.

## MCP tools

None.

## Tests and quality

- 20 `#[test]` attributes in `src/*.rs` (shape parsers, retry delay math, rpc decode), 3 fixtures via `include_str!`.
- CI on **ubuntu and windows**: `cargo fmt --check`, `cargo check --all-targets`, `cargo clippy -D warnings`, `cargo test` (`.github/workflows/ci.yml`). Release workflow builds binaries (`release.yml`). Docs deliberately removed in the last commits ("docs: remove docs and porting references", "ignore docs and local porting/security notes").
- No live/HAR-derived fixtures; fixtures are hand-written.

## Versions

v0.1.0-alpha 2026-02-18 (feat: typed generation, retries, docs, release automation; CI/CD matrices linux+windows). v0.1.1-alpha 2026-02-26 (docs cleanup). `CHANGELOG.md` lists typed artifact options, retry w/ backoff+jitter across RPC/query/download, mind-map persistence to notes, CLI `--output json|table|tsv` and `--quiet`.

## Adopt

1. **`DecodeShape{path, context}` error** and a `json_at` safe-path accessor for positional arrays (`error.rs`, `client.rs`). In Go: `func at(v any, idx ...int) (any, bool)` plus an error that carries the path.
2. **Typed artifact generation options** (typed audio format/length, report templates, etc.) as enums at the tool layer instead of raw ints (`CHANGELOG.md` 0.1.0-alpha).
3. **CLI output modes** `--output json|table|tsv` + `--quiet` as a convention for our CLI/doctor commands.
4. **Windows in the CI matrix from the start** (`ci.yml`).
5. Local conversation-history cache keyed by conversation id so follow-ups can resend history when the server loses context (`conversation_cache`).

## Avoid

1. One 3.4k-line source file; split by domain.
2. Reload-from-disk "refresh" that cannot recover an expired session.
3. Retrying all non-idempotent calls; fixed 30 s timeout on chat.
4. Missing `bl`/`hl`/`_reqid` on batchexecute and stale hard-coded `bl` for chat (`20251221`).
5. First-match frame selection that ignores the position-5 status.

## Evidence

Read: `Cargo.toml`, `README.md`, `CHANGELOG.md`, `src/rpc.rs` (full), `src/auth.rs` (full), `src/error.rs`, `src/client.rs` (lines 1-300, 2160-2250, 3390-3430, grep), `src/types.rs:478-494`, `.github/workflows/ci.yml`, fixtures list, `git log`, `git tag`. Nothing executed.
