# icebear0828/notebooklm-client

- URL: https://github.com/icebear0828/notebooklm-client (npm `notebooklm-client`, CLI `notebooklm`, agent skill "notecraft"/SKILL.md)
- Language / runtime: TypeScript (ESM), Node >= 20 (package.json:58-59). Deps: `commander`, `puppeteer-core` **aliased to `rebrowser-puppeteer-core@^24`** (package.json:46; a stealth-patched Puppeteer fork [unverified claim about the fork's purpose]), `undici ^7`; optional `tlsclientwrapper` (package.json:55-57). Not an MCP server: CLI + library (`src/index.ts` exports `NotebookClient`).
- License: MIT, "Copyright (c) 2026 icebear0828" (LICENSE:1-3). Snippets allowed.
- Stars / last commit: 227 stars; HEAD `7319b6c` 2026-05-27 (merge PR #26 "add 5 missing RPC IDs"). Version 0.2.0. No git tags (`git tag` empty).
- Versions reviewed: HEAD 7319b6c; history from 2026-03-16 (`af45e5b Package as npm SDK`) to 2026-05-27. Clone: `research/upstreams/icebear0828__notebooklm-client`.
- Note: research/upstreams.md says "Puppeteer; browser automation plus RPC [unverified split]". Verified here: it is an **RPC client first**; the browser is used for login/session export and as one of four transports, not for UI clicking. The whole client is built on `batchexecute`; there is no DOM scraping of answers.
- REA: `rea analyze-javascript-application ... --artifact-format directory` (680 module nodes incl. tests; 1 `endpoint` node; 1 false `electron-main`). The module map was decisive: `src/transport*.ts` (6 files: interface, resolver, browser, http, curl, tlsclient), `rpc-ids.ts`/`rpc-config.ts` (id table + disk override), `boq-parser.ts`, `parser.ts` (16 exported response parsers), `api.ts` (all RPC call builders, 40+ exports), `session-store.ts` (imports `child_process` for curl-impersonate), `tls-config.ts` (imports `node:tls`), `browser.ts` (`injectAntiDetection`, `launchBrowser`), `utils/refresh-guard.ts`, `utils/humanize.ts`. REA flags that only `transport-browser.ts`, `browser.ts`, `download.ts`, `client.ts` import `puppeteer-core`; everything else is pure protocol code, which is the layering we want in Go.

## Access method and transport selection (directly relevant to our dual transport)

All features go through one abstraction (`src/transport.ts:11-29`): `Transport { execute({url, queryParams, body}) -> raw text; getSession(); refreshSession(); dispose() }` with a `NotebookRpcSession = {at, bl, fsid, cookies, cookieJar?, userAgent, language?}` (`src/types.ts:324-335`). Four implementations:

| Transport | File | How a request leaves | Needs |
|---|---|---|---|
| `browser` | `transport-browser.ts` | `page.evaluate(fetch(url?qp, {method POST, credentials:"include"}))` in a launched real Chrome (puppeteer), tokens from `window.WIZ_global_data` | Chrome + profile, login once |
| `curl-impersonate` (tier 1) | `transport-curl.ts` | spawns `curl-impersonate`/`curl_chrome131`/`curl_chrome116` with `--impersonate chrome136`, cookies in a Netscape temp file (to avoid ARG_MAX), 60 s timeout | binary on macOS/Linux; session.json |
| `tls-client` (tier 2) | `transport-tlsclient.ts` | optional npm `tlsclientwrapper` (wraps the Go bogdanfinn/tls-client shared lib), profile `chrome_131`, 60 s timeout | optional dep |
| `http` (tier 3) | `transport-http.ts` | `undici` with Chrome cipher list + ALPN h2/http1.1, fixed Chrome-131 headers incl. `Sec-Ch-Ua*`, `Sec-Fetch-*`, `Origin/Referer https://notebooklm.google.com` | always available |

**How it chooses (`--transport browser|auto|curl-impersonate|tls-client|http`, `src/cli.ts:31, 65-72`):**
- Default is `browser` (cli.ts:72; README:96 says `auto | browser (default: browser)`). `NotebookClient.connect()` takes `browser` -> `connectBrowser` (launch Chrome, extract tokens, **auto-save the session to disk**, `client.ts:109-132`) and anything else -> `connectHeadless` (load session from config object, env `NOTEBOOKLM_AUTH_JSON`, or `~/.notebooklm/session.json`; error "No session available. Run export-session ..." `client.ts:134-153`).
- `auto` means "best **headless** tier", resolved by `detectBestTier()`: curl-impersonate if the binary runs, else tls-client if the module has the expected API, else undici (`transport-resolver.ts:62-75`). `auto` **never** selects the browser and never falls back to it. There is no runtime fall-through between tiers on request failure either: the tier is chosen once at connect (`client.ts:172-188`), the label is logged (`TIER_LABELS`), and a failing tier surfaces its error.
- Browser <-> headless relationship: browser mode is the *session provider* (`export-session` logs in via browser and writes `session.json`; `import-session` accepts a file/JSON so a session can be moved to a server; README:109-110). Headless tiers replay those cookies + `at/bl/f.sid` with a Chrome-like TLS fingerprint.
- Failure policy per call (`client.ts:254-295, 298-358`): request -> if the error text matches `/HTTP\s+(401|400)\b/` (`isAuthError`, :355-358) call `transport.refreshSession()` and retry once. **400 is treated as auth expiry**, so a malformed request or a rotated RPC id also triggers one pointless refresh. Responses containing `UserDisplayableError` are thrown as such without retry (:279-283).
- Refresh per transport: browser = reload page + re-read WIZ tokens, de-duplicated with `withRefreshGuard` (a WeakMap of in-flight promises, `utils/refresh-guard.ts:5-28`; `transport-browser.ts:101-118`); headless = `onSessionExpired` -> `refreshTokens()` which does a GET of the dashboard through curl-impersonate with the stored cookies, regexes `"SNlM0e"`, `"cfb2h"`, `"FdrFJe"` and `<html lang>` out of the HTML, merges `Set-Cookie` rotations, and rewrites session.json (`session-store.ts:164-271`). **That refresh hard-requires the curl-impersonate binary** (`:172-175`) even when the transport tier is undici/tls-client; if refresh fails it re-reads the file on disk once, then errors "Session expired and auto-refresh failed ... Re-run export-session" (`client.ts:158-170`).
- Session validity helper: `hasValidSession(maxAge=2 h)` by file `exportedAt` (`session-store.ts:130-152`) - an age heuristic.
- Download fallback inside a transport: media URLs downloaded via curl/undici with a Netscape cookie jar, retries on CDN 404 after generation, login-page (HTML) response detection (commits 3b38f40, 0181066, a2f7e10); audio can also be downloaded *through the browser* with CDP `Browser.setDownloadBehavior` + synthetic anchor click (`download.ts` `downloadAudioBrowser`, ~:215-260). So a browser fallback exists only for downloads.
- Session expiry and TLS: the project's own comment admits "Node.js/undici cannot fully replicate JA3/JA4 fingerprints ... close enough for most Google endpoint checks" (`tls-config.ts:9-11`); curl-impersonate/tls-client exist because that was not enough [claim in README/comments; not independently verified]. In Go this concern is solved natively with uTLS (`refraction-networking/utls`) or `bogdanfinn/tls-client` [unverified suggestion].

Takeaways for our dual transport:
1. Put the browser behind the same `Transport` interface that the pure-HTTP clients use (execute raw request, get session, refresh, dispose); parsers/builders never see the transport (`api.ts` takes an `RpcCaller` function: `(rpcId, payload, sourcePath?) -> raw`, `download.ts` re-exports the type).
2. Make session = `{at, bl, fsid, cookies, userAgent, language}` the unit that moves between browser and HTTP. Only the browser can create it; HTTP can refresh it with a GET of the notebook home page.
3. The pattern "choose once at start" loses to arlinamid's per-operation `tryRpc` fallback. A hybrid is possible: HTTP first, on `RpcError` (not auth) retry through in-page fetch, then UI.

## RPC catalog

Endpoint and params (`src/rpc-ids.ts:78-84`, `client.ts:254-295`): `POST https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute?rpcids=<id>&source-path=<sp>&bl=<cfb2h>&hl=<lang>&_reqid=<n>&rt=c[&f.sid=<FdrFJe>]`; body (form-urlencoded) `f.req=[[[id, JSON(payload), null, "generic"]]]&at=<SNlM0e>`; `Content-Type: application/x-www-form-urlencoded;charset=UTF-8`; HTTP tiers add `Cookie`, `Origin`, `Referer`, `X-Same-Domain: 1`, Chrome UA and `Sec-*` client-hint headers (`transport-http.ts:109-129`); the browser tier sends only Content-Type (`transport-browser.ts:79-84`). `source-path` = `/notebook/<id>` if a notebook is active else `/` (`client.ts:260`). `_reqid` starts at 100000 and increases by `jitteredIncrement(100000, 0.2)` per call (`client.ts:102, 249-252`, `utils/humanize.ts`). `hl` = session language from `navigator.language` (browser) or `<html lang>` (refresh) (`transport-browser.ts:147`, `session-store.ts:247`). Token acquisition: browser tier waits (polling 2 s, timeout 180 s then reload + 60 s) for `window.WIZ_global_data.SNlM0e` and `cfb2h` containing `labs-tailwind`, then reads `at=SNlM0e`, `bl=cfb2h`, `fsid=FdrFJe` (`transport-browser.ts:40-70, 139-148`); cookies via CDP `Network.getAllCookies` filtered to `*.google.com`, `googleapis.com`, `googleusercontent.com` (:152-188); headless refresh via regex over the dashboard HTML (above). RPC id overrides: `~/.notebooklm/rpc-ids.json` maps static name -> id and is consulted on every call (`client.ts:244-247`, `rpc-config.ts:121-166`) so a rotated id can be patched without a release; `NB_URLS.BASE` still points to `notebooklm.google.com` (rpc-ids.ts:79) and `transport-http` sets `Origin: https://notebooklm.google.com` (:117): after the 2026 host move these presumably rely on the 301 for GET but a POST to the old host is not a verified path [unverified].

Response parsing (`boq-parser.ts:101-191`): strip `)]}'`, read length-prefixed chunks (one JSON per line, fallback multi-line accumulation up to the byte count), pick envelopes with `env[0] === 'wrb.fr'` and parse `env[2]` string as JSON. (arlinamid ignores the length prefix and parses every line starting with `[`; both work.) Helpers `get/getString/getArray` walk index paths and return null/'' instead of throwing (`parser.ts:11-28`).

| RPC id | Name in `NB_RPC` | Purpose | Request payload (`params`) | Response shape | file:line |
|---|---|---|---|---|---|
| CCqFvf | CREATE_NOTEBOOK | create notebook | `["", null, null, [2], [1,null,...,[1]]]`, sp `/` | `inner[2]` = uuid, `inner[11][0][0]` = default chat thread id | api.ts:52; parser.ts:41-58 |
| wXbhsf | LIST_NOTEBOOKS | list | `[null, 1, null, [2]]`, sp `/` | `inner[0]` entries `[title, sources[], uuid(...)]`; id filtered by `^[0-9a-f]{8}-` | api.ts:77; parser.ts:73-91 |
| rLM1Ne | GET_NOTEBOOK | notebook detail | `[id, null, [2], null, 1]` (arlinamid sends `...0` as last arg) | `[title, sources[]]`; source `[[id], title, meta[]]`, words = `meta[1]`, url = `meta[7]` | api.ts:86; parser.ts:93-126 |
| s0tc2d | RENAME_NOTEBOOK | rename | `[id, [[null,null,null,[null,title]]]]`, sp `/` | - | api.ts:103 |
| WWINqb | DELETE_NOTEBOOK | delete | `[[id],[2]]`, sp `/` | - | api.ts:94 |
| fejl7e | REMOVE_RECENTLY_VIEWED | remove from recents | (not used in api.ts) | - | rpc-ids.ts:12 |
| izAoDd | ADD_SOURCE | add url / text | url: `[[[null,null,[url],null x7,1]], id, [2], [1,null..,[1]]]`; text: `[[[null,[title,content],null,2,null x6,1]], id, [2], settings]` | `[0][0]=[[sourceId], title,...]` | api.ts:117, 136; parser.ts:128-136 |
| o4cbdc | ADD_SOURCE_FILE | register an upload | `[[[fileName]], id, [2], settings]` | source id; then **resumable upload** below | api.ts:171 |
| hizoJc | GET_SOURCE_CONTENT | source text | (not called in api.ts) | | rpc-ids.ts:17 |
| tr032e | GET_SOURCE_SUMMARY | source guide | `[[[[sourceId]]]]` | summary | api.ts:287; parser.ts:161 |
| tGMBJ | DELETE_SOURCE | delete source | `[[[sourceId]], [2]]` | - | api.ts:280 |
| FLmJqe | REFRESH_SOURCE | re-crawl a URL/Drive source | `[null, [sourceId], [2]]`, sp notebook | - | api.ts:310 |
| yR9Yof | CHECK_SOURCE_FRESHNESS | staleness check | (unused) | | rpc-ids.ts:21 |
| b7Wfje | UPDATE_SOURCE | rename source | `[null, [sourceId], [[[title]]]]` | - | api.ts:298 |
| Ljjv0c | CREATE_WEB_SEARCH | fast research | `[[query, 1], null, 1, notebookId]` | `[0]` task id | api.ts:555 |
| QA9ei | CREATE_DEEP_RESEARCH | deep research | `[null, [1], [query, 1], 5, notebookId]` | `[0]` task id, `[1]` report id | api.ts:574 |
| e3bVqc | POLL_RESEARCH | poll | `[null, null, notebookId]` | `status >= 2` means done; results + report | api.ts:596; parser.ts:483 |
| LBwxtb | IMPORT_RESEARCH | import found sources | `[null,[1],researchId,notebookId,sources]` | - | api.ts:631 |
| R7cb6c | GENERATE_ARTIFACT | create Studio artifact | `[[2,null,null,[1,null,...,[1]],[[2,1,3]]], notebookId, innerPayload]` (`DEFAULT_USER_CONFIG`, rpc-ids.ts:86); `innerPayload` built per type (`artifact-payloads.ts:109-269`: audio fmt 1 deep_dive/2 brief/3 critique/4 debate, length 1/2/3; video fmt 1 explainer/2 brief/3 cinematic, style codes 1 auto,3 classic,4 whiteboard,5 kawaii,6 anime,7 watercolor,8 retro_print; quiz qty/difficulty; infographic orientation/detail/style; slide deck; data table) | `[artifactId, title]` | api.ts:515; parser.ts:171 |
| gArtLc | GET_ARTIFACTS_FILTERED | list artifacts | `[DEFAULT_USER_CONFIG, notebookId, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | artifacts incl. media URLs (walked by `findMediaUrls`) | api.ts:527; parser.ts:180-282 |
| V5N4be | DELETE_ARTIFACT | delete artifact | `[DEFAULT_USER_CONFIG, artifactId]` | - | api.ts:539 |
| rc3d8d | RENAME_ARTIFACT | rename artifact | `[artifactId, title]` (arlinamid sends `[[id,title],[["title"]]]`; payload shapes differ) | - | api.ts:467 |
| v9rmvd | GET_INTERACTIVE_HTML | quiz / flashcards / mind-map HTML | `[artifactId]` | HTML string (single-element array wrapper handled, commit 6ef613b) | api.ts:474; download.ts:270 |
| Krh3pd | EXPORT_ARTIFACT | export to Docs/Sheets | (unused) | | rpc-ids.ts:36 |
| RGP97b | SHARE_ARTIFACT | share artifact | (unused) | | rpc-ids.ts:37 |
| KmcKPe | REVISE_SLIDE | revise slide | (unused) | | rpc-ids.ts:38 |
| ciyUvf | GET_SUGGESTED_REPORTS | suggestions | (unused here) | | rpc-ids.ts:39 |
| sqTeoe | GET_STUDIO_CONFIG | studio config / allowed options | `[DEFAULT_USER_CONFIG, notebookId]` | config | api.ts:643; parser.ts:405 |
| CYK0Xb | CREATE_NOTE | create note | `[notebookId, '', [1], null, 'New Note']` | `[0][0]` note id | api.ts:352 |
| cFji9 | GET_NOTES | list notes | `[notebookId]` | rows `[id, [..title, content]]` (rows with `[1]===null && [2]===2` skipped) | api.ts:322 |
| cYAfTb | UPDATE_NOTE | update note | `[notebookId, noteId, [[[content, title, [], 0]]]]` | - | api.ts:378 |
| AH0mwd | DELETE_NOTE | delete note | `[notebookId, null, [noteId]]` | - | api.ts:389 |
| yyryJe | GENERATE_MIND_MAP | mind map | (unused here) | | rpc-ids.ts:45 |
| hPTbtc | LIST_CHAT_THREADS | chat threads | `[[], null, notebookId, 20]` | `[0]` rows; first id = default thread | api.ts:69; parser.ts:60 |
| khqZz | GET_CONVERSATION_TURNS | thread turns | (unused in api.ts) | | rpc-ids.ts:51 |
| J7Gthc | DELETE_CHAT_THREAD | delete chat | `[[], threadId, null, 1]` | - | api.ts:678 |
| JFMDGd | GET_SHARE_STATUS | share status | `[notebookId, [2]]` | raw envelope | api.ts:399 |
| QDyure | SHARE_NOTEBOOK | public link / invite user | public: `[[[id,null,[access],[access,'']]],1,null,[2]]`; user: `[[[id,[[email,null,permCode]],null,[msgFlag,msg]]], notify, null, [2]]` | - | api.ts:413, 431 |
| ZwVcOc | GET_ACCOUNT_INFO | account info + quota | lang read: `[null,[1,null,...,[1]]]`; quota: `[DEFAULT_USER_CONFIG]` | quota: `limits=[plan, notebookLimit, sourceLimit, wordLimit]`, `flags[0]` = plus | api.ts:441, 651; parser.ts:446-481 |
| hT54vc | SET_USER_SETTINGS | output language | `[[[null,[[null,null,null,null,[lang]]]]]]` | - | api.ts:457 |
| VfAZjd | GET_NOTEBOOK_SUMMARY | notebook overview | (unused here) | | rpc-ids.ts:61 |
| otmP3b | GET_RECOMMENDED_TOPICS | suggested topics | (unused) | | rpc-ids.ts:62 |
| ozz5Z | GET_UI_CONFIG (name) | (arlinamid uses it as an add-source variant) | | | rpc-ids.ts:63 |
| Fxmvse | REPORT_PLAY_PROGRESS | audio progress ping | (unused) | | rpc-ids.ts:64 |
| (stream) | CHAT_STREAM URL | chat | see below | | rpc-ids.ts:82 |
| (upload) | UPLOAD URL `https://notebooklm.google.com/upload/_/` | file bytes | see below | | rpc-ids.ts:83 |

Union of ids not in arlinamid's catalog (new evidence): `fejl7e, o4cbdc, tGMBJ, FLmJqe, yR9Yof, V5N4be, Krh3pd, RGP97b, KmcKPe, sqTeoe, CYK0Xb, cFji9, yyryJe, cYAfTb, AH0mwd, otmP3b, Fxmvse`. (Commit 77ce64c 2026-05-26 "add 5 missing RPC IDs from upstream" added ids copied from another project, so some table rows are unused-but-listed.)

Differences to reconcile when we test live: `rLM1Ne` last arg `1` (icebear) vs `0` (arlinamid); `rc3d8d` payload shape; chat `params`.

### File upload ("Scotty" resumable protocol) - not present in any browser-based fork
`api.ts:158-277`: (1) RPC `o4cbdc` with `[[[fileName]], notebookId, [2], settings]` returns a `sourceId`; (2) `POST https://notebooklm.google.com/upload/_/?authuser=0` with headers `x-goog-upload-command: start`, `x-goog-upload-header-content-length: <size>`, `x-goog-upload-protocol: resumable`, `x-goog-authuser: 0`, Cookie, Origin/Referer, and JSON body `{"PROJECT_ID": notebookId, "SOURCE_NAME": fileName, "SOURCE_ID": sourceId}` -> response header `x-goog-upload-url`; (3) `POST <that url>` with `x-goog-upload-command: upload, finalize`, `x-goog-upload-offset: 0` and the raw file bytes, 5 min timeouts (`headersTimeout/bodyTimeout 300_000`). Whole file read into memory (`readFileSync`). For Go: stream the body and support >1 chunk.

## Streaming/chat

- Endpoint: `POST https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=&hl=&_reqid=&rt=c[&f.sid=]` (`rpc-ids.ts:82`, `client.ts:332-347`). Body `f.req=[null, JSON(inner)]&at=<token>`.
- `inner = [sourceIds.map(id => [[id]]), message, history|null, [2, null, [1], [1]], threadId|null, null, null, notebookId, turnCounter + 1]` (`client.ts:310-330`). (arlinamid sends a shorter form `[ids, question, history|null, [2,null,[1]], convId]` with a different history ordering - see below.)
- Thread/history state is **client-side** per notebook: `ChatState {threadId, history[], turnCounter}` (`client.ts:79-83, 104`); `ensureChatThread` takes the first id from `LIST_CHAT_THREADS` (`hPTbtc`) (:582-590, commit c7e373c "use notebook's default thread"), `bindChatThread` resets history when the thread changes ("the web UI's follow-up payload format requires history to start empty for a fresh thread", :569-577), each completed turn does `history.unshift([question,null,1])` then `unshift([reply,null,2])` so **history is newest-first** (:604-612). arlinamid builds history oldest-first (reverses khqZz output). One of them is wrong or the server accepts both [unverified]; test live.
- Per-notebook FIFO queue serialises chats (`chatQueues` Map of promise tails, `enqueueChat` :626-645) = same idea as arlinamid's PageLock but keyed by notebook, not by tab.
- Parsing (`parser.ts:284-403`): `extractAllInner` returns every envelope's inner JSON; `parseChatStream` simply keeps the **last non-empty `payload[0]` string** as answer and `meta = payload[2]` -> `meta[0]` thread id, `meta[1]` response id (:284-312). It does not skip "thinking" chunks (arlinamid checks `meta[last] === 1`); on a UI that streams thinking text last, this could return thinking text [unverified].
- Citations (`parseChatWithCitations` :349-403, commit a5d40fc): the last envelope with `payload[4][0]` where `[1]` is the inline-refs array; each ref `[ [chunkId], [?, charStart, charEnd] ]` (:372-396); `payload[4][3]` is the citation tree flattened into a chunkId -> `{sourceId, relevance, excerpt}` map (`buildChunkMap` :330-347, `flattenExcerptTree` :316-328). Output: `{index, sourceId, relevance, charStart, charEnd, excerpt, chunkId}` per citation. This char-offset form (positions in the answer text) is richer than arlinamid's passage list.
- The whole response is read as text (`execute` returns string) in all tiers; no incremental streaming to the caller.
- Timeouts: curl/tls 60 s per call; browser: protocolTimeout 120 s (browser.ts:459-460).

## Auth lifecycle

- Source of cookies: (a) `export-session`: launch real Chrome with a persistent profile `~/.api-reverser/gemini-profile` (shared with the author's other tool, `paths.ts:28-31`), user logs in once, cookies pulled over CDP `Network.getAllCookies`, written with `at/bl/fsid/userAgent/language` to `~/.notebooklm/session.json` as `{version:1, exportedAt, session}` (`session-store.ts:63-95`); (b) `import-session <file|json>`; (c) env `NOTEBOOKLM_AUTH_JSON` (`client.ts:137-143`).
- Required cookies: not enumerated; sends **all** `*.google.com`, `googleapis.com`, `googleusercontent.com` cookies, deduplicated on name=value (`transport-browser.ts:162-186`). Flat cookie string + per-cookie jar (domain/path/secure/httpOnly) for cross-domain media downloads (commit da6457c "infer cookieJar from flat cookies + fix Netscape domain flags", 3b38f40 "domain-scoped cookie jar").
- Storage: plain JSON with default permissions (`writeFile(filePath, JSON.stringify(...))`, `session-store.ts:93`; no chmod); cookies also written to temp files for curl and unlinked in `finally` (`transport-curl.ts:76-134`, `session-store.ts:180-222`); an earlier bug passed cookies on the command line (commit f8b8151 fixed ARG_MAX/ps exposure).
- Refresh: see "Access method". Expiry detection = HTTP 400/401 text match, `UserDisplayableError` marker, or `SNlM0e` not found on refresh ("cookies may be expired", `session-store.ts:249-251`), plus in-body login-page detection for downloads (`text/html`). `NOTEBOOKLM_HOME` / `--home` gives one directory per account (multi-account = separate homes + profiles; `paths.ts:1-21`, README:211-217; commit e0e3b5b "browser profile dir was eagerly evaluated").
- Proxy: `--proxy` / `HTTPS_PROXY` for http/socks5/socks5h on all tiers and Chrome `--proxy-server` (commit 5e77bf0).
- First-run detection: absence of `<profile>/Default` (transport-browser.ts:26) prints "please log in".

## Selectors

Not applicable for core features (RPC). The only DOM interaction is login and token extraction through `window.WIZ_global_data` (`transport-browser.ts:42-44, 142-148`), the check `cfb2h.includes('labs-tailwind')` as a "page is the right app" probe (:44, 64), and the audio download via anchor click. This is the least UI-coupled design of the six; the fragile part is the id/payload layout, mitigated by `rpc-ids.json` override and `diagnose` command (CLI) that writes a report.

## Stealth / anti-detection (browser tier only)

`src/browser.ts`:
- Launch: system Chrome/Edge/Brave/Chromium discovered from OS-specific path lists (:25-79); `headless:false` default (:455); `ignoreDefaultArgs:['--enable-automation']`; args `--disable-blink-features=AutomationControlled --no-first-run --no-default-browser-check --disable-extensions --remote-allow-origins=* --lang=en-US --disable-session-crashed-bubble --disable-features=InfiniteSessionRestore --hide-crash-restore-bubble` (:121-131). `--remote-allow-origins=*` is a security smell (any web page may connect to the CDP port) [judgement].
- Profile hygiene before launch: rewrite `Default/Preferences` `profile.exit_type=Normal` & `exited_cleanly=true`, same in `Local State`, delete stale `DevToolsActivePort` (:325-420) to suppress the "restore pages" bubble after a crash.
- `injectAntiDetection` via `evaluateOnNewDocument` (:133-321): `navigator.webdriver` -> undefined; fake `navigator.plugins` (5 PDF viewers with `PluginArray` prototype); `languages`, `platform` ('Win32'); `permissions.query` for notifications -> denied; fake `window.chrome` (`runtime`, `loadTimes`, `csi`, `app`); deterministic per-host **canvas noise** (8 pixels, seed = md5(hostname), mulberry32) on `toDataURL`/`toBlob`; WebGL vendor/renderer overridden for UNMASKED params (0x9245/0x9246) to "NVIDIA GeForce GTX 1080"; `hardwareConcurrency`/`deviceMemory` from the real host (memory capped at 8); `Error` constructor proxy rewriting stack strings `pptr://`, `devtools://`, `__puppeteer_evaluation_script__` to `chrome-extension://...`.
- Inconsistency risks: platform/WebGL hard-coded Windows+NVIDIA regardless of real OS/GPU (:109-112), UA taken from the real browser; mismatches between `Sec-Ch-Ua-Platform` ("Windows" fixed in transport-http.ts:123) and the real machine are themselves a fingerprint [judgement].
- Behavioural: `humanSleep` jitter (+-30 %) on polling (`utils/humanize.ts`, `workflows.ts:107-132, 135-173`), `_reqid` jitter (client.ts:249-252); no typing simulation because there is no typing.
- Headless tiers rely on TLS impersonation (curl-impersonate `chrome136`, tls-client `chrome_131`, undici cipher order) and fixed Chrome-131 client-hint headers.

## Browser / page lifecycle, pooling, timeouts, concurrency

- Browser transport = one browser, one page (`transport-browser.ts:16-18`), closes all other pages (`browser.ts:463-470`); no pool. Headless tiers need no browser at all.
- Timeouts: launch 60 s, protocolTimeout 120 s (browser.ts:459-460), dashboard goto 60 s `networkidle2`, token wait 180 s then reload 60 s, refresh reload 30 s (transport-browser.ts:32, 46, 66, 106-113), curl/tls-client 60 s, uploads 300 s, research poll 120 s fast / 1 200 s deep (`workflows.ts:74`), sources-ready poll 600 s with backoff `min(3000 + n*1500, 15000)` ms (`workflows.ts:99-130`), artifact poll `min(5000 + n*2500, 30000)` ms (:135-173).
- Concurrency: per-notebook FIFO for chat; `withRefreshGuard` collapses concurrent token refreshes to one; no global rate limiter or retry/backoff on 429/5xx (only on CDN 404 for downloads, `download.ts`).
- Singletons: none; each CLI invocation connects, works, disconnects.

## Notebook library / session management

No library concept. `NotebookClient` holds `activeNotebookId`, `chatStates`, `chatQueues` (client.ts:98-105). CLI flow functions in `workflows.ts` (runAudioOverview, runReport, runMindMap, runFlashcards, runQuiz, runInfographic, runSlideDeck, runDataTable, runVideo, runAnalyze, pollSourcesReady, pollArtifactReady, addSourceFromInput) create a notebook, add sources (url/text/file or research topic), generate, poll, download.

## MCP tools

None (not an MCP server). CLI commands (README:104-125): `export-session, import-session, list, detail, source add (--file/--url/--text), chat, audio, report, video, quiz, flashcards, infographic, slides, data-table, analyze, diagnose, skill install`; plus `delete` (commit e1005da) and per-type options (formats, lengths, styles, `--language`, `--research-mode fast|deep`, `--proxy`). Useful as the list of Studio features to expose: audio (deep_dive/brief/critique/debate x short/default/long), video (explainer/brief/cinematic x 7 styles), report (briefing_doc/study_guide/blog_post/custom, `REPORT_TEMPLATES` artifact-payloads.ts), quiz, flashcards (incl. `--language`), infographic (orientation/detail/style), slide deck (format/length, pdf+pptx download), data table (CSV extraction, commit a6fdcb7 fixes prompt-echo bug), mind map.

## Reliability

Single retry after token refresh; no backoff; errors are plain `Error`/`SessionError`/`UserDisplayableError` (`errors.ts`). Known fragility fixed in history: Google frontend change broke deep research on 2026-03-19 (commit 475934d), quota parser used the wrong RPC (e90960c "was GetOrCreateAccount, not GetQuota"), data-table CSV saved the prompt echo (a6fdcb7), tls-client API shape verification (62a337c), session refresh path needing curl-impersonate + in-body auth detection (e7a9b50). Evidence that RPC payload layouts change roughly monthly.

## Performance notes

Headless tiers: one HTTPS request per call, keep-alive 30 s (`transport-http.ts:151-152`), no browser memory; curl tier spawns a process per call (tens of ms overhead + TLS handshake each time, no connection reuse) [inferred]. Browser tier reuses one tab's connection. `_reqid` increments exist only to look natural.

## Tests and quality

17 vitest files (~220 `it` blocks counted by grep): unit tests for `boq-parser`, `parser`, `artifact-payloads`, `chat-payload`, `rpc-config`, `session-store`, `transport-*`, `tls-config`, `source-add`, `data-table`, `paths`; e2e files hit the live service (`vitest.e2e.config.ts`: e2e-artifacts, e2e-http, e2e-proxy, e2e-file-upload, e2e-research). Fixture-driven parser tests are the model to copy for our Go RPC layer.

## Versions

No tags; package version 0.2.0. Timeline from `git log`: 2026-03-16 npm SDK packaging, session import (`9fdbb14`); 03-17 quota/audio fixes, proxy; 03-20 deep research fix after 2026-03-19 frontend change; 03-22 notes/sharing/settings APIs, research import; 03-24 file upload (Scotty); 03-26/27 all artifact types, split of client.ts into modules, cookie jar fixes; 03-28 delete CLI; 04-09 CDN download robustness; 04-17 `source add`, `--language`; 04-21 `source add` CLI, session refresh fixes (04-26); 04-23 per-citation metadata; 05-12 default chat thread; 05-26/27 five more RPC ids.

## Adopt

1. The `Transport` interface (`transport.ts:11-29`) with `execute(raw)/getSession/refreshSession/dispose` and `NotebookRpcSession` as the portable credential.
2. Token-refresh by GET of the notebook home page + regexes for `SNlM0e`, `cfb2h`, `FdrFJe`, `<html lang>` and `Set-Cookie` merge (`session-store.ts:164-301`) - works without a browser once cookies exist. In Go use `net/http` with a cookie jar.
3. `rpc-ids.json` override file consulted per call (`client.ts:244-247`, `rpc-config.ts`) - emergency patch for rotated ids; extend to payload templates.
4. `withRefreshGuard` single-flight on refresh (`utils/refresh-guard.ts`) -> Go `singleflight.Group`.
5. Scotty resumable upload sequence (`api.ts:158-277`), with streaming in Go.
6. Chat citations with char offsets and chunk map (`parser.ts:316-403`).
7. Chat state per notebook + per-notebook FIFO queue (`client.ts:104-105, 626-645`).
8. All the Studio payload builders and enum code tables (`artifact-payloads.ts:109-269`), research flow (`api.ts:555-631`), notes/share APIs (`api.ts:322-436`).
9. Download robustness: cookie jar by domain, retries on CDN 404 after generation, reject HTML (login) bodies (`download.ts`, commits 3b38f40, 0181066, a2f7e10).
10. Profile hygiene before launching Chrome: reset `exit_type`/`exited_cleanly`, delete `DevToolsActivePort` (`browser.ts:325-420`).
11. Tests as fixtures of captured responses per parser (`tests/parser.test.ts`).
12. `diagnose` command that prints transport tiers, session age and a probe call (`cli.ts:625-729`) for supportability.

## Avoid

1. Choosing the transport once at connect with no per-request fall-through; and `auto` that excludes the browser (`client.ts:172-188`, `transport-resolver.ts:62-75`).
2. Treating HTTP 400 as an auth error (`client.ts:355-358`): masks protocol/payload errors and refreshes needlessly.
3. Headless refresh that depends on an external binary (`session-store.ts:172-175`).
4. Plain `session.json` with no permissions (`session-store.ts:93`); cookies in temp files.
5. Parsing "last non-empty chunk" as the answer (`parser.ts:284-312`) - ignores thinking chunks.
6. Hard-coded Windows/NVIDIA fingerprint and fixed Chrome-131 hints (`browser.ts:109-112`, `transport-http.ts:121-124`) - stale versions are themselves a signal.
7. `--remote-allow-origins=*` (browser.ts:126).
8. `NB_URLS.BASE` hard-coded to the old host (`rpc-ids.ts:79-83`) - make the host a config value derived from the page's origin.
9. Whole-file `readFileSync` upload (api.ts:183).
10. Dependence on third-party native helpers (curl-impersonate binary, tlsclientwrapper) - in Go use uTLS in-process.
11. Using `ozz5Z`-style unnamed ids without semantics; keep a typed registry with verified-on date.

## Evidence

Read: `src/transport.ts`, `transport-resolver.ts`, `transport-browser.ts`, `transport-http.ts`, `transport-curl.ts` (key parts), `transport-tlsclient.ts` (key parts), `rpc-ids.ts`, `rpc-config.ts`, `boq-parser.ts`, `parser.ts` (functions cited), `api.ts:1-700`, `client.ts:1-700`, `session-store.ts`, `browser.ts`, `paths.ts`, `tls-config.ts:1-40`, `utils/{humanize,refresh-guard}.ts`, `workflows.ts:70-175`, `download.ts:190-260`, `cli.ts:31-110`, README, package.json, `git log`, `git tag` (empty). REA JSON module map gave the file/import graph and the puppeteer-isolation fact.
