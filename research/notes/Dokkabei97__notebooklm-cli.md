# Dokkabei97/notebooklm-cli

- URL: https://github.com/Dokkabei97/notebooklm-cli
- Language / runtime: Go 1.26.1 (`go.mod:3`), cobra CLI `nlm`. Deps: cobra, lipgloss, go-rod/rod (browser login), `modernc.org/sqlite` (Chrome cookie DB), `golang.org/x/crypto` (PBKDF2) (`go.mod:5-9`). Pure Go, no cgo.
- License: MIT (`LICENSE:1`, "Copyright (c) 2025"). Short snippets allowed with attribution.
- Stars / last commit: 2 stars (per upstreams.md); HEAD `52369b6` 2026-03-15. 7 commits, all on 2026-03-15.
- Versions reviewed: tag v0.1.0 (2026-03-15).
- Size: 67 files; `internal/rpc/` is 625 lines total (`caller.go` 117, `decoder.go` 224, `decoder_test.go` 37, `encoder.go` 63, `errors.go` 49, `method.go` 135).

Small, clean, a direct port of teng-lin's conventions (`CLAUDE.md`: "Method ID는 Python 레퍼런스(teng-lin/notebooklm-py)에서 추출"). Useful as a minimal reference for the `rpc` package split and for the one thing no other Go repo has: **macOS Chrome cookie-DB decryption in pure Go**.

## Package layout (reusable)

```
main.go
cmd/            cobra commands: root auth notebook source chat artifact generate note research share completion use version
internal/rpc/   encoder.go decoder.go caller.go errors.go method.go      <- batchexecute
internal/auth/  browser.go (rod login) chrome.go (cookie DB) extract.go (SNlM0e/FdrFJe) storage.go (storage_state.json) tokens.go
internal/api/   notebooks sources chat artifacts notes research sharing client errors   <- business logic on rpc.Caller
internal/model/ domain structs + enums
internal/config paths.go (~/.notebooklm)
internal/output table/json/format
e2e/            CLI-level tests (need live auth)
skills/notebooklm/SKILL.md  Claude Code skill file
```
(`CLAUDE.md` "구조" section; `git ls-files`). The layering `rpc` (wire) -> `api` (typed operations) -> `cmd` (UX) is the same split we want (transport / client / MCP tools).

## Access method

Internal batchexecute RPC + chat streaming endpoint + Scotty resumable upload. Browser (rod) only for login. Host: `https://notebooklm.google.com` (old host; see tmc note for the 2026-07-27 move to `notebook.google.com`) (`internal/rpc/method.go:246-251`).

## RPC catalog

URL: `https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute?rpcids=<id>&source-path=<path>&f.sid=<FdrFJe>&rt=c` (`encoder.go:157-167`; params sorted by `url.Values.Encode`). **No `bl`, `hl`, `_reqid`** on batchexecute. Body `f.req=<urlencoded [[[id,"<params>",null,"generic"]]]>&at=<SNlM0e>&` (`encoder.go:126-155`; note comment says "Triple-nest [[inner]]", the code produces `[[ [id,...] ]]`, same as tmc). Headers: `Content-Type: application/x-www-form-urlencoded`, `Origin`, `Referer`, a fixed macOS Chrome 131 UA, `X-Same-Domain: 1`, `Cookie` (`caller.go:62-67`).

Method IDs (`method.go:186-244`) and verified-in-code param shapes (`internal/api/*.go`):

| RPC id | Purpose | Request params | Response | file:line |
|---|---|---|---|---|
| wXbhsf | ListNotebooks | `[null, 1, null, [2]]` (matches tmc) | parsed by `parseNotebooks`; null -> empty | `api/notebooks.go:11-20` |
| CCqFvf | CreateNotebook | `[title, null, null, [2], [1]]` | `parseNotebook`; null -> success, no data | `notebooks.go:23-33` |
| rLM1Ne | GetNotebook (also source list) | `[notebookID, null, [2], null, 0]` (older flat `[2]` block; sahaand reports this is rejected since Oct 2026) | | `api/sources.go:19` |
| WWINqb | DeleteNotebook | `[[notebookID], [2]]` | void | `notebooks.go:36-41` |
| s0tc2d | RenameNotebook | `[notebookID, [[null,null,null,[null,newTitle]]]]` | | `notebooks.go:45-49` |
| izAoDd | AddSource url / text | url: `[[[null,null,[url],null,null,null,null,null]], nb, [2], null, null]`; text: `[[[null,[title,content],null*6]], nb, [2], null, null]` (`api/sources.go:31-59`). URL at index 2 and text as `[title, content]` at index 1, same shape as tmc's older tail (no nested `[2,null,[1],[1,...]]` block). Tail is `[2], null, null` here vs `[2],[1,null*9,[1]]` for file registration. |  | `sources.go:32-62` |
| o4cbdc | AddSourceFile (register) | `[[[filename]], nb, [2], [1,null*9,[1]]]` -> returns SOURCE_ID (first string found by recursive `extractNestedString`) | | `sources.go:76-90` |
| tGMBJ | DeleteSource | `[[[sourceID]]]` | void | `sources.go:195-196` |
| hizoJc | GetSource (fulltext) | | | `method.go:198` |
| FLmJqe | RefreshSource | `[sourceID]` | | `sources.go:205` |
| yR9Yof | CheckSourceFreshness | | | `method.go:200` |
| b7Wfje | UpdateSource (rename) | | | `method.go:201` |
| qXyaNe | DiscoverSources | tmc found the real id is `Es3dTe`; `qXyaNe` is "not present in any JS bundle or HAR" (`tmc internal/notebooklm/rpc/rpc.go:66-75`) (!) | | `method.go:202` |
| VfAZjd / tr032e / ciyUvf | Summarize / GetSourceGuide / GetSuggestedReports | | | `method.go:205-207` |
| R7cb6c / gArtLc / V5N4be / rc3d8d / Krh3pd / RGP97b / v9rmvd / KmcKPe | create/list/delete/rename/export artifact, share artifact, interactive HTML, revise slide | | | `method.go:210-217` |
| Ljjv0c / QA9ei / e3bVqc / LBwxtb | fast research / deep research / poll / import | | | `method.go:220-223` |
| yyryJe / CYK0Xb / cFji9 / cYAfTb / AH0mwd | mind map / create / list(notes+mindmaps) / update / delete note | | | `method.go:226-230` |
| hPTbtc / khqZz | last conversation id / conversation turns | `hPTbtc: nil`; `khqZz: [convID, 20]` | | `api/chat.go:97-112` |
| QDyure / JFMDGd | share notebook / share status | | | `method.go:237-238` |
| fejl7e / ZwVcOc / hT54vc | remove recently viewed / get user settings / set | | | `method.go:241-243` |

Artifact type codes (request): Audio 1, Report 2, Video 3, Quiz 4, MindMap 5, Infographic 7, SlideDeck 8, DataTable 9 (`method.go:257-265`). Artifact status: 1 processing, 2 pending, 3 completed, 4 failed (`:271-275`). Source status: 1 processing, 2 ready, 3 error, 5 preparing (`:296-300`).

Response frames handled: `wrb.fr` (result at [2], string-encoded JSON is parsed once; null = valid void/empty) and `er` (error entry; becomes `Error{ErrServer}` with raw payload) (`decoder.go:84-161`). Not handled: `["e",4,null,null,141]` trailer and `["di",...]` metadata are skipped; **status at frame position 5 is ignored**, so `["wrb.fr","cFji9",null,null,null,[3],"generic"]` (an INVALID_ARGUMENT failure) decodes as a *successful null result* (see the unit test that pins exactly this frame as "null result decoded as success", `decoder_test.go:7-21`). tmc and sahaand both fixed this later (tmc `chunked.go:316-328`; sahaand `rpc/decoder.go:257-300`).

## Streaming/chat

- Endpoint `https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=boq_labs-tailwind-frontend_20260301.03_p0&hl=en&_reqid=100000&rt=c&f.sid=<FdrFJe>` (`encoder.go:170-180`). `bl` is a **hard-coded** build label (from March 2026), `_reqid` hard-coded 100000 (`api/chat.go:64`).
- Body `f.req=[null,"<inner>"]&at=<token>&` where inner = `[[[[id]]...], question, null, [2,null,[1],[1]], conversationUUID, null, null, notebookID, 1]` (`api/chat.go:28-52`). Sources default to all notebook sources via `ListSources` (`:19-27`).
- Headers: Origin, Referer, UA, Cookie; **no** `x-goog-ext-353267353-jspb`, no `X-Same-Domain` (`chat.go:71-75`) — tmc says that ext header is required; Dokkabei's chat may have worked on a March 2026 backend [unverified now].
- Response parse: documents the inner JSON as `[ [answer_text, null, [conv_id, turn_id, ts], null, [[[formatting]], ..., TYPE]], citations, ..., follow_ups ]` with `TYPE: 1 = answer, 2 = thinking` (`chat.go:120-133`); keeps the last answer-type text (`parseStreamingResponse`, `chat.go:134+`). Reads the whole body (no incremental streaming).
- Multi-turn: relies on server-side conversation; history param is `nil`.

## Auth lifecycle

Three ways in (`cmd/auth.go:20-35,102`):

1. `nlm auth login` -> rod opens a visible Chrome at `https://accounts.google.com/ServiceLogin?continue=https://notebooklm.google.com`, waits up to 5 min for URL to start with the NotebookLM host, sleeps 2 s, then `proto.NetworkGetAllCookies` (`internal/auth/browser.go:15-72`). (Google increasingly blocks rod/chromedp-launched sign-in; see LocalKinAI note.)
2. `nlm auth login --reuse` -> `ExtractChromeCookies()`: reads Chrome's cookie SQLite directly, no browser launch. **macOS only**: password via `security find-generic-password -s "Chrome Safe Storage" -w`, key = PBKDF2-HMAC-SHA1(password, "saltysalt", 1003 iterations, 16 bytes), cookie path hard-coded `~/Library/Application Support/Google/Chrome/Default/Cookies` (`internal/auth/chrome.go:25-47,193-200`). Decrypt: strip `v10` prefix, AES-128-CBC with IV = 16 spaces, PKCS#7 unpad, **then drop the first 32 bytes** (Chrome >= v24 DB schema prepends a SHA-256 of the host key) (`chrome.go:202-258`). DB is copied (with `-wal`/`-shm`) to a temp file first to avoid locks (`:91-107,260-300`). SQL filters to `host_key IN ('.google.com', '.notebooklm.google.com', 'notebooklm.google.com')` (`:120-127`); deliberately excludes regional cookies like `.google.co.kr`. Chrome epoch fix: `expires_utc - 11644473600000000` microseconds (`:168-172`). Requires `SID` present (`:53-63`). `findChromeProfiles` (Default + "Profile N") exists but is unused (`:69-89`).
   - **Not implemented**: Windows (DPAPI + AES-256-GCM `v10`/`v11`, and Chrome 127+ App-Bound Encryption `v20`), Linux (peanuts/ `v11` via libsecret or kwallet), Firefox, Edge/Brave, multi-profile choice. No `v20` handling at all: values that do not start with `v10` are returned as raw bytes and then dropped by `isPrintableASCII` (`:147-152,207-212`).
3. `NOTEBOOKLM_AUTH_JSON` env or `~/.notebooklm/storage_state.json` Playwright file (`internal/auth/storage.go:29-60`, `config/paths.go`). Saved with mode 0600 in a 0700 dir (`storage.go:114`, `paths.go:13-15`).

Token fetch: `GET https://notebooklm.google.com/` with Cookie header (re-applied on every redirect hop), regexes for `SNlM0e` and `FdrFJe`, redirect to `accounts.google.com` = expired, merges `Set-Cookie` from the response into the cookie set (`internal/auth/extract.go:19-99`). No `cfb2h`/bl extraction. Cookie header builder dedups by name and drops empty values (`extract.go:101-112`, `rpc/caller.go:20-40`).

Expiry handling: HTTP 401/403 -> `rpc.Error{ErrAuth/ErrForbidden}`; `IsAuthError()` helper (`rpc/errors.go:345-350`). No automatic refresh or retry. Multi-account: none (`authuser=0` hard-coded in upload URL and headers, `sources.go:110,129,162`).

## Reliability

- No retry, no backoff. `checkHTTPStatus` maps 401/403 -> auth, 404 -> not found, 429 -> rate limit, >=500 -> server (`caller.go:102-117`). Error type `rpc.Error{Code, Message, Method}` with `IsAuthError/IsNotFound/IsRateLimit` (`errors.go:332-364`). Type assertion `err.(*Error)` (not `errors.As`), so wrapped errors are missed.
- Default `http.Client{}` has **no timeout** (`caller.go:34`).
- `wrapErr(op, step, err)` adds context (`internal/api/errors.go`).
- Polling for readiness is `time.Sleep` loops (`api/sources.go:239`, `artifacts.go:82`) without context cancellation.

## Performance notes

Single shared client, new request per call. Whole-body `io.ReadAll`. Nothing measured.

## MCP tools

None. It ships a `skills/notebooklm/SKILL.md` for Claude Code instead (commit `52369b6`).

## Tests and quality

- `internal/rpc/decoder_test.go`: two tests (null result, string result) (37 lines).
- `e2e/*.go`: CLI tests that need a live session (`TestAuthStatusWhenAuthenticated`, `TestChatAsk`, ...).
- CI: `test.yml`, `release.yml` (goreleaser), `codeql.yml`, `nightly.yml` (E2E), **`rpc-health.yml`**: a daily cron that runs a read-only notebook against live endpoints using a secret storage-state file, opens an issue on failure (`.github/workflows/rpc-health.yml:1-50`). Note it is gated on `github.repository == 'jmk/notebooklm-cli'` so it never runs in this fork (`:19`).

## Versions

Only v0.1.0 (2026-03-15). Seven commits, all the same day (initial commit imported from a private repo with "English localization and GoReleaser").

## Adopt

1. **Package split rpc / api / model / auth / config** and tiny `rpc.Caller{HTTPClient, cookieHeader, CSRFToken, SessionID}` with `Call(rpcID, params, notebookPath) -> (*CallResult{Raw, Parsed}, error)` where `(nil, nil)` means a legitimate null result (`caller.go:42-100`). We need the same API but with the position-5 status check so `(nil,nil)` is only returned when status is absent/0.
2. **Daily RPC health workflow** (`rpc-health.yml`) as a canary for Google changing wire shapes: run a handful of read-only calls against a throwaway account and open an issue. Cheap insurance for a reverse-engineered project.
3. **Chrome cookie decryption recipe on macOS incl. the 32-byte prefix strip, WAL/SHM copy, Chrome epoch conversion** (`chrome.go:91-258`). Port as a macOS implementation behind a `CookieSource` interface. (We still need Windows and Linux.)
4. `SafeString/SafeFloat/SafeArray(data, idx...)` nested-array accessors that never panic (`decoder.go:175-224`). Useful until typed structs exist.
5. Domain filter for cookies: only `.google.com`, `.notebooklm.google.com`, `notebooklm.google.com`; skip regional domains to avoid cross-account cookie mixing (`chrome.go:115-127`). (Update for `notebook.google.com`.)
6. Dedup cookies by name and merge response `Set-Cookie` after token fetch (`extract.go:84-112`).

## Avoid

1. Treating a null result as success when frame position 5 carries a status (`decoder_test.go:7-21` pins the bug).
2. Hard-coded `bl` (`encoder.go:172`) and `_reqid` (`api/chat.go:64`).
3. `http.Client{}` without timeout; `time.Sleep` poll loops without ctx.
4. `err.(*Error)` type assertions; use `errors.As`.
5. `--reuse` cookie reading that silently drops non-`v10` values; fail with a precise "v20/App-Bound" message instead.
6. Old flat `[2]` template block in `rLM1Ne`/`izAoDd` (`sources.go:19,31-59`): sahaand says Google now rejects it with INVALID_ARGUMENT (see sahaand note).

## Evidence

Read in full: `internal/rpc/{caller,decoder,decoder_test,encoder,errors,method}.go`; `internal/auth/{chrome,browser,extract,tokens}.go`, `storage.go` (grep); `internal/api/{client,notebooks,sources}.go` (key sections), `chat.go:1-140`; `cmd/auth.go` (grep); `internal/config/paths.go`; `CLAUDE.md`; `.github/workflows/rpc-health.yml`; `go.mod`. `git log`, `git tag`. Nothing executed.
