# Decisions needed (owner: user)

Date: 2026-10-14. Surfaced by the synthesis in [feature-matrix.md](feature-matrix.md), [rpc-catalog.md](rpc-catalog.md) and [best-practices.md](best-practices.md). Already decided and not repeated here: Go + go-sdk v1.8.0 stdio, RPC first with browser fallback, all distribution channels, default host `notebook.google.com`. Still pending from CLAUDE.md: project name, and confirmation of Go and Apache-2.0.

Each item: options, recommendation, why (with the note that supports it).

## D1. Android gRPC schema (`schema.proto`) from teng-lin

TL recovered a 326-message protobuf schema from Google's Android app and runs a gRPC transport to `notebooklm-pa.googleapis.com` with a master-token bearer ([TL note](notes/teng-lin__notebooklm-py.md)). It would give typed models instead of positional arrays.

- A. Do not use it at all; build typed Go structs from our own web captures (TMC-style lossless round-trip tests).
- B. Use it only as a private reading aid for field meaning; nothing copied into the repo.
- C. Vendor or port it after a legal review.
- D. Also ship the Android gRPC transport.

**Recommendation: A.** It is a decompilation of Google's app (legal gray area, TL note), the Android path needs a full-account master token (ToS and lockout risk), and TMC shows the web wire can be modelled losslessly from captures ([TMC note](notes/tmc__nlm.md)). Revisit only if DBSC kills web cookie replay (D9).

## D2. Enterprise REST transport: v1 or later

Official, cookie-free, but covers only notebook CRUD, sources and an option-less audio overview; no chat, studio, notes or research ([ROB](notes/Robiton__notebooklm-mcp-cli.md), [KD](notes/K-dash__nblm-rs.md)). Pre-GA, documented fields rejected ("audio only `{}`").

- A. v1.0 with capability flags.
- B. v1.x after the consumer RPC path is stable, behind the same transport interface.
- C. Never.

**Recommendation: B.** Design the interface and `UNSUPPORTED_BY_TRANSPORT` errors in v1 so it slots in, but ship it after 1.0; it needs a licensed GCP project to test, and its value is limited to Workspace users. The license-free Podcast API (ROB) can ride along then.

## D3. Firefox (and Safari) cookie import

Firefox stores cookies unencrypted on every OS, so it is the one browser-store import that works on Windows despite Chrome App-Bound Encryption ([MIS](notes/missdeer__notebooklm-client.md), behavior only; [FAX](notes/Faxtom__notebooklm-mcp.md)). Costs: a pure-Go SQLite dependency (binary size), schema-version handling (expiry ms vs s), and reading the user's real browser store.

- A. Include `auth_login(method=firefox)` in v1 as an explicit user command.
- B. Support only Netscape `cookies.txt` export (user exports from any browser) in v1; Firefox later.
- C. Neither.

**Recommendation: B for v1, A in v1.x.** cookies.txt import has no dependency and covers every browser via an extension; CDP attach and the dedicated profile cover the rest. Never read stores implicitly from a tool call (STU anti-pattern).

## D4. Chromium cookie-store decryption (DPAPI / Keychain / libsecret)

- A. Implement per OS.
- B. Do not implement.

**Recommendation: B.** Windows ABE makes it unreliable, DOK only covers macOS, and it reads like credential theft (performance.md s7, [KAF](notes/KAFKA2306__nlm.md), [DOK](notes/Dokkabei97__notebooklm-cli.md)).

## D5. REST/HTTP API besides MCP

ROO ships a 33-route Express API (with unsafe defaults); TL ships a FastAPI server ([ROO](notes/roomi-fields__notebooklm-mcp.md), [TL](notes/teng-lin__notebooklm-py.md)).

- A. Ship a REST API in v1.
- B. Ship only MCP over stdio plus opt-in streamable HTTP MCP (loopback, bearer token, Origin check); expose scripting through the CLI.
- C. REST later if users ask.

**Recommendation: B, then C.** A second API surface doubles the security and compatibility surface; streamable HTTP MCP already serves remote and Docker use.

## D6. CLI and skills scope

- A. Minimal CLI: `login`, `logout`, `status`, `doctor`, `serve`, `call <tool> <json>` (every MCP tool reachable as JSON), plus one `SKILL.md`.
- B. Full noun-verb CLI like TL/JB (`nlm notebook list`, ...).
- C. No CLI beyond `serve` and `login`.

**Recommendation: A.** `call` gives scripting parity without a second command tree to maintain; `doctor` follows KD/TL. Pipelines and content workflows stay out (CW, JB `pipeline`).

## D7. Headless or non-interactive auth rungs

- A. Allow operator `NOTEBOOKLM_REFRESH_CMD` hook and headless re-mint on the dedicated profile; no master token.
- B. Also allow TL's master-token (gpsoauth) mint.
- C. Interactive only.

**Recommendation: A.** Master tokens are full-account credentials (TL note). Headless re-mint should be opt-out and must never pop a visible window mid-session without consent (M4Y anti-pattern).

## D8. TLS fingerprint impersonation (uTLS)

MIS claims plain Go TLS gets `CookieMismatch` on the dashboard; TMC, LKA, DOK fetch it with stdlib and work. Unverified either way.

- A. stdlib only.
- B. stdlib default, uTLS optional behind a flag, auto-suggested when `CookieMismatch` is detected.
- C. uTLS always.

**Recommendation: B**, pending live-capture item 4 in rpc-catalog.md.

## D9. How to classify DBSC / cookie-binding rejections for fallback

You set "never fall back on auth". A rejection caused by device-bound cookies or TLS binding is solved by in-page fetch inside the signed-in browser, not by re-login (ARL, JB `cdp_transport`).

- A. Treat `CookieMismatch` and DBSC-style rejections as transport errors: allowed to fall back to in-page fetch; plain UNAUTHENTICATED (16) stays on the auth ladder.
- B. Treat all of them as auth: no fallback.

**Recommendation: A**, with the error class reported in the tool result.

## D10. Browser fallback depth

- A. In-page `fetch` of the same RPCs only (cheap, robust to UI changes, does not fix payload drift).
- B. In-page fetch first, then DOM automation for chat and add-source only.
- C. Full DOM parity for every tool.

**Recommendation: B.** DOM maintenance is the costliest part of every browser upstream (PP, ARL selector churn); limit it to the two features most users need when RPC payloads drift.

## D11. Default tool profile and batch group

- A. Ship all 33 tools by default.
- B. Default 32 (batch group off); `minimal` profile of 8 tools documented for small-context hosts.

**Recommendation: B** (JB groups, PP profiles; TL ADR-0025 notes 38 tools is too many).

## D12. Account rotation on quota

ROO rotates across several Google accounts when one hits its daily limit.

- A. Do not implement.
- B. Implement as opt-in.

**Recommendation: A.** It is ToS-adjacent and the browser fallback does not escape quotas either (performance.md "Contradictions" #1). Multi-account stays explicit via `profile`.

## D13. Long-running work model

- A. Fire-and-return ids plus status tools with capped `wait` and progress notifications.
- B. MCP "tasks" utility (spec 2025-11-25) where the client supports it.

**Recommendation: A now, B when go-sdk support is confirmed** (performance.md s1 marks it unverified; ARL uses it experimentally in TypeScript).

## D14. On-disk cache of source text and artifact metadata

- A. Off by default, opt-in, `clear_cache` documented.
- B. On by default.

**Recommendation: A.** It holds private notebook content (performance.md s5).

## D15. License reuse policy confirmation

Code-reuse candidates by license, from the notes: MIT (TL, JB, TMC, ICE, ROO, ARL, PP, PAN, AGM, MOOD, M4Y, DOK, KD, MFL and others) and Apache-2.0 (LKA, STR) can be adapted with attribution; GPL-3.0 (MIS) and no-license repos (SAH, ASC, WAN, JAC, alfredang) are behavior-only.

- Confirm Apache-2.0 for our project and a `NOTICE` that credits MIT/Apache sources we adapt (TMC batchexecute decoder ideas, LKA auth/login, TL wire tables).

**Recommendation: confirm**, as proposed in CLAUDE.md.
