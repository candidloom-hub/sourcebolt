# LocalKinAI/notebooklm-go

- URL: https://github.com/LocalKinAI/notebooklm-go
- Language / runtime: Go 1.24 (`go.mod:3`); library + CLI binary `notebooklm-go`. Only dependency: `chromedp` (+ cdproto) (`go.mod:5-8`). Stdlib HTTP otherwise.
- License: Apache-2.0 (`LICENSE`, full text, 202 lines). **No `NOTICE` file exists** in the repo, so copying code needs only license + copyright notice retention (Apache s4); we should credit it in our NOTICE anyway.
- Stars / last commit: 1 star (per upstreams.md); HEAD `2d17dd3` 2026-05-16
- Versions reviewed: v0.1.0, v0.1.1, v0.2.0, v0.2.1, v0.2.2, all tagged 2026-05-16 (single day); 5 commits total (`git log`).
- Size: 17 files, 3984 lines. `rpc.go` 314, `client.go` 770, `client_extra.go` 214, `auth.go` 190, `login.go` 267, `cmd/notebooklm-go/main.go` 1010.

Verdict: tiny, readable, license-friendly, but **quality is below tmc/nlm**. Many "stable" request shapes disagree with tmc and with the October 2026 rework recorded by sahaand. Use as a second opinion on wire shapes, not as a source of truth.

## Access method

Internal batchexecute RPC only; Chrome (chromedp) only for login. Host `notebooklm.google.com` (old host; tmc found it rejects newly harvested credentials since 2026-07-27, so this client is likely stale) (`rpc.go:20-22`).

- batchexecute URL: `https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute?rpcids=<id>&source-path=<path>&hl=en&rt=c[&f.sid=<FdrFJe>]` (`rpc.go:125-136`). **No `bl` and no `_reqid`** on batchexecute calls. For chat, `bl=""` (empty), `hl`, `_reqid=UnixMilli%1e6`, `rt=c`, `f.sid` (`client.go:202-240`).
- Body: `f.req=<urlencoded [[[id, "<params json>", null, "generic"]]]>&at=<SNlM0e>&` (`rpc.go:93-122`).
- Headers: only `Content-Type: application/x-www-form-urlencoded;charset=UTF-8` and `Cookie` (`rpc.go:282-283`). No Origin/Referer/X-Same-Domain/User-Agent.
- HTTP client timeout 120 s; downloads use a separate client (600 s) with a real cookiejar because media lives on `googleusercontent.com` and a flat Cookie header is not honored cross-domain (`client.go:29-65`, `auth.go:65-119`).

## RPC catalog

Constants (`rpc.go:25-71`). Param shapes from `client.go`/`client_extra.go`. "stable/exp" is the README label. Items marked (!) disagree with tmc/nlm or Dokkabei97.

| RPC id | Purpose (their name) | Request params | Response parse | file:line |
|---|---|---|---|---|
| wXbhsf | ListNotebooks | `[]` (empty array) (!) tmc sends `[null,1,null,[2]]` | `result[0]` = list of entries; they read `arr[0]` as id and `arr[1]` as title (string or `[[title]]`) (`client.go:84-110`). (!) tmc's captured fixture shows entries shaped `[["uuid"], "title", [meta...], ...]` (older capture, Nov 2024) while teng-lin-derived code uses `[title, sources, id, ...]`; which one is current is [unverified]. Re-capture before relying on it | `client.go:72-115` |
| CCqFvf | CreateNotebook | `[title, null, null, [2], [1]]` | id at index 2 | `client.go:117-152` |
| rLM1Ne | GetNotebook (metadata) | `[notebookID]` | raw | `client.go:739-760` |
| s0tc2d | RenameNotebook (exp) | in `client_extra.go` | | `rpc.go:30` |
| WWINqb | DeleteNotebook | `[notebookID]` | | `client.go:154-157` |
| izAoDd | AddSource | URL: `[[[null,null,null,null,null,null,null,null,[url],null,1]], nb, [2], [1,null*9,[1]]]` (!) URL at index 8; tmc/Dokkabei put `[url]` at index 2. YouTube: `[url]` at index 7 (matches tmc). Text: `[null,null,content,null*7,1]` (!) tmc/Dokkabei use `[null,[title,content],null,2]` | none parsed | `client.go:162-196` |
| tGMBJ / hizoJc / FLmJqe / b7Wfje | Delete/Get/Refresh/Update source (exp) | `client_extra.go` | | `rpc.go:35-38` |
| VfAZjd | Summarize | `[notebookID]` | string | `client.go:739-760` |
| tr032e | GetSourceGuide | `[notebookID]` (!) tmc: keyed by **source** id | | |
| R7cb6c | CreateArtifact (audio etc.) | audio: `[nb, null, 1, null, null, format]`; generic: `[nb, null, typeCode]` (!) tmc: `[ctx, project_id, options,...]` with nested option block; this flat form is probably the pre-2026 shape and likely rejected now [unverified] | artifact id = `result[0]` | `client.go:343-385` |
| gArtLc | ListArtifacts | `[[2], nb, "NOT artifact.status = \"ARTIFACT_STATUS_SUGGESTED\""]` (matches tmc `[ctx,project_id,filter]`) | artifact rows: id `[0]`, title `[1]`, type `[2]`, status `[4]`, created `[15][0]` | `client.go:410-480` |
| V5N4be / Krh3pd | DeleteArtifact / ExportArtifact (exp) | | | `rpc.go:46-48` |
| hPTbtc / khqZz | GetLastConvID / GetConvTurns (exp) | | | `rpc.go:51-52` |
| yyryJe / CYK0Xb / cFji9 | MindMap legacy / CreateNote / GetNotes (exp) | | | `rpc.go:55-57` |
| Ljjv0c / QA9ei | StartFastResearch / StartDeepResearch | `[notebookID, query]` (!) tmc: `[[query,1],null,1,project_id]` / `[null,[1],[query,1],5,project_id]` | | `client.go:716-737` |
| e3bVqc / LBwxtb | Poll / Import research (exp) | | | `rpc.go:62-63` |
| QDyure / JFMDGd | Share / GetShareStatus | `[notebookID]` | raw | `client.go:750-752` |
| ZwVcOc | GetUserSettings (used as CheckAuth) | | | `rpc.go:70` |

Artifact type codes in this repo: Audio 1, Report 2, Video 3, Quiz 4, MindMap 5, Infographic 7, SlideDeck 8, DataTable 9 (`rpc.go:74-83`). Audio format: DeepDive 1, Brief 2, Critique 3, Debate 4 (`:86-91`). These match teng-lin conventions as quoted by Dokkabei97/missdeer. (Their Artifact struct comment says "1=video, 3=audio, 7=infographic" for **response** type codes, which is the opposite of the request codes — noted in `client.go` doc comment; confirm against teng-lin.)

Response download URL extraction: audio/video/infographic URL located by shape search in the artifact row (`extractAudioURL/extractVideoURL/extractInfographicURL`, `client.go:550-672`).

## Streaming/chat

- Endpoint: `https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed` (`client.go:~202-204`).
- Body `f.req=[null,"<inner>"]&at=<token>&`; inner = `[nil(sources), question, nil(history), [2,null,[1],[1]], convID, null, null, notebookID, 1]` (`client.go:206-220`). **Sources are sent as nil** ("nil = all"), unlike tmc/Dokkabei/sahaand which send an explicit `[[[id]]...]` list. Whether nil works against the current backend is [unverified]; tmc's README says unpinned selection is server-chosen and unstable.
- conversation id is `%x` of `UnixNano` (not a UUID) (!).
- No `x-goog-ext-353267353-jspb` header (tmc says it is required and was "observed in HAR capture") (!).
- Parse: whole body read, then `extractChatText` collects first-array-element text from every `wrb.fr` frame and returns the **longest** string, on the theory that thinking text is shorter than the answer (`client.go:272-340`). Fragile; ignores citations, follow-ups, phase flags. No streaming to the caller.

## Auth lifecycle

- Storage: Playwright `storage_state.json` at `~/.notebooklm/storage_state.json` (or `$NOTEBOOKLM_AUTH_JSON` path) (`auth.go:172-181`). Cookies filtered to `google.com`, `*.google.com`, `googleusercontent.com`, `*.googleusercontent.com` (`auth.go:184-190`).
- Tokens: `GET https://notebooklm.google.com/` with the cookie header and `CheckRedirect = ErrUseLastResponse` (any 3xx = "authentication expired"), then regex `"SNlM0e"\s*:\s*"([^"]+)"` and `"FdrFJe"\s*:\s*"([^"]+)"` (`auth.go:123-169`). Good minimal pattern. `bl` (`cfb2h`) is not extracted.
- Login: (a) `Login()` launches a fresh chromedp Chrome, user signs in, copies `network.GetCookies` into storage_state (`login.go:18-130`). Google now blocks this ("Browser not secure"). (b) `LoginAttach(port)` (v0.2.1): user restarts Chrome with `--remote-debugging-port=9222`, tool GETs `http://localhost:9222/json/version` to find the websocket, attaches via chromedp remote allocator, reads cookies (`login.go:140-265`). Same idea as tmc's `-cdp-url`.
- No cookie-DB decryption, no DPAPI/ABE code.
- Refresh: none. 401/403 -> error telling user to re-login (`rpc.go:291-293`). No in-body UNAUTHENTICATED detection (`[16]` at frame position 5 is ignored; a null result becomes "no result found"). 
- Multi-account: none.
- Secrets: storage file written mode 0600 (`login.go:123,260`).

## Reliability

- No retry, no backoff, no jitter. 429 -> "rate limited by Google — wait and retry" string error (`rpc.go:294-296`).
- Errors are plain `fmt.Errorf` strings; `["er", id, data]` frames become `RPC error for <id>: <raw>` (`rpc.go:247-249`). No typed errors, no gRPC status handling.
- Chunk parsing ignores the declared byte counts: line-based, takes the line after a numeric line (`rpc.go:164-196`). Breaks if a JSON frame contains a raw newline inside a string (tmc documents that NotebookLM titles and notes can).
- `decodeResponse` returns the first frame matching the rpc id; multi-RPC batches unsupported.
- `LOCALKIN_NB_DEBUG=1` dumps body if < 2000 bytes (`rpc.go:306-311`) — would leak tokens/PII in logs.

## Performance notes

One shared `http.Client`; no pooling tuning; no concurrency control. Nothing measured. README claims "saved ~500 MB RAM per pipeline run" by dropping Playwright (marketing, [unverified]).

## MCP tools

None. Not an MCP server (it is the audio backbone of the author's own agent product; MCP not mentioned in README).

## Tests and quality

- `rpc_test.go` (135 lines): envelope encode, request body, URL build, anti-XSSI strip, chunk parse, extract success/error, Google-domain filter (`rpc_test.go:9-117`). Nothing for client behavior, no fixtures, no recorded traffic.
- CI: ubuntu+macos, Go 1.22/1.24, `go vet`, `go build`, `go test -count=1 -race`, gofmt (`.github/workflows/test.yml`). No Windows.
- README labels "stable" vs "experimental" per method; experimental ones are "best-guess param shapes ... not verified against a live session" (`README.md`, "EXPERIMENTAL methods").

## Versions

v0.1.0 (initial), v0.1.1 full RPC surface incl. experimental wrappers, v0.2.0 rename binary to `notebooklm-go`, v0.2.1 `login --attach`, v0.2.2 `version` subcommand (`CHANGELOG.md`, `git log`). All on 2026-05-16.

## Adopt

1. **`login --attach` over CDP to the user's real, already running Chrome** (`login.go:140-265`; README "Recommended: --attach"). Google cannot distinguish it from normal use and it avoids all cookie-store decryption (DPAPI/ABE). Good fallback for Windows. Same as tmc `-cdp-url`; combine into one `auth attach` command.
2. **Minimal token fetch** with no redirect following to detect expired sessions (`auth.go:123-169`).
3. **Cookie jar for artifact downloads** (domain-aware, Playwright domain semantics incl. leading-dot, `auth.go:65-119`). Needed for googleusercontent media URLs.
4. **Playwright storage_state.json as an interchange format** (`auth.go:16-30`): also used by teng-lin, Dokkabei, sahaand, MatrixForge, so users can reuse existing logins. Import-only.
5. Apache-2.0 license: code may be adapted with attribution; candidate files are `auth.go`, `login.go`.

## Avoid

1. Treating "stable" labels as evidence: several request shapes disagree with tmc/Dokkabei (URL at index 8, text source shape, `[]` for list, flat artifact params, nil source list in chat). Re-verify every shape.
2. "Longest text wins" chat parsing (`client.go:272-340`).
3. Declared-length-ignoring, newline-based chunk parser (`rpc.go:164-196`).
4. No retry/typed errors; no `bl`/`_reqid`; no headers beyond Cookie.
5. Old host `notebooklm.google.com` hard-coded as a const (`rpc.go:20-22`).
6. Debug dump of raw response bodies by env var (`rpc.go:306-311`).

## Evidence

Read in full: `rpc.go`, `auth.go`, `client.go` (lines 1-420 and grep-scanned remainder), `login.go` (grep), `rpc_test.go` (function list), `CHANGELOG.md`, `README.md`, `.github/workflows/test.yml`, `go.mod`. `git log`, `git tag`. Line numbers were checked against `grep -n` on 2026-10-14.
