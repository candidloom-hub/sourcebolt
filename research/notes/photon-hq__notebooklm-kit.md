# photon-hq/notebooklm-kit

- URL: https://github.com/photon-hq/notebooklm-kit (npm `notebooklm-kit`)
- Language / runtime: TypeScript SDK (no MCP server), Node `fetch`, Playwright only for auto-login (src/auth/auth.ts:6)
- License: MIT (package.json). Short snippets allowed.
- Stars / last commit: 42 stars (research/upstreams.md); HEAD aa44b49 2026-03-06 CODEOWNERS; last functional commit 3ced31e 2026-01-14 "bump version to 2.2.0"
- Versions reviewed: HEAD = 2.2.0 (package.json:3). Tags: v2.0.0 (2025-12-26), v2.1.1, v2.1.2, v0.0.1 (2026-01-10, odd), v2.2.0 (2026-01-12).
- REA ran (311 KB md); node output unreadable (`[object Object]`), used only as file inventory. Source 17.9k lines in src (artifacts.ts 6006, sources.ts 3349).
- Sibling note: Kisara-k/notebooklm-kit is a fork of this repo with 30 commits ahead (see Kisara-k__notebooklm-kit.md), 
  which contains the only live-verified fixes to this SDK's payloads and the move to `notebook.google.com`.

## Access method

Internal `batchexecute` RPC plus the streaming chat endpoint, over `fetch`. Service-style SDK: `client.notebooks / sources / artifacts / notes / generation` (src/client/notebooklm-client.ts, 591 lines). README "Debugging" section shows the debug log with RPC ids (README.md:518 `[DEBUG] RPC Call: wXbhsf with args: [null, 1, null, [2]]`, :521 `Auto-refresh: Token expires in 5 minutes`), enabled by `NOTEBOOKLM_DEBUG=true` (README.md:489).
Client-side quota manager enforcing plan limits (src/utils/quota.ts), file chunking, and an auto-refresh scheduler.

## RPC catalog

Endpoint (src/utils/batch-execute.ts:97-114): `POST https://{host}/_/LabsTailwindUi/data/batchexecute` with query `rpcids=<ids joined by ,>`, `_reqid` (random base 1000-9999 plus sequence*100000, :18-31), and from config `bl=boq_labs-tailwind-frontend_20250129.00_p0`, `f.sid=-7121977511756781186` (hard-coded constant), `hl=en`, `authuser=<0 default>` (src/rpc/rpc-client.ts:51-58), plus per-call `source-path=/notebook/{id}` or `/` (rpc-client.ts:87-95). Host `notebooklm.google.com` (rpc-client.ts:36).
Body: `f.req=JSON([[ [id, JSON(args), null, "generic"], ... ]])&at=<authToken>` (batch-execute.ts:117-123, 270-279); supports several RPCs per request (comma-joined rpcids), though callers use one.
Headers: content-type form, origin/referer, `x-same-domain: 1`, `accept-language`, `cache-control: no-cache` (rpc-client.ts:40-50); cookie header from config.
CSRF `at` is NOT scraped at runtime; it comes from config / env `NOTEBOOKLM_AUTH_TOKEN` (`SNlM0e` of the form `token:timestampMs`, expiry assumed timestamp + 1 h, src/auth/refresh.ts:176-196). `f.sid` and `bl` are constants (stale: one is 2025-01, another 2026-01 in streaming client), which still work, i.e. the server tolerates stale `bl`/`f.sid` [observed by authors, not verified here].
Retry (batch-execute.ts:69-80,135-248,386-411): default 1 retry, 1 s delay doubling to 5 s cap (env `NOTEBOOKLM_MAX_RETRIES`, `NOTEBOOKLM_RETRY_DELAY`, `NOTEBOOKLM_RETRY_MAX_DELAY`); retries on 429/500/502/503/504 and network error strings; 401 = non-retryable auth error. No fetch timeout.
Response decode (batch-execute.ts:284-384, utils/chunked-decoder.ts): strip `)]}'`, if it starts with a digit use the chunked parser (honors byte counts), else plain JSON; envelopes with `rpcData[0]=='wrb.fr'`, data at `[2]` else `[5]`; a bare numeric body is returned as an error code; error classification table in src/utils/errors.ts (ErrorType enum with RATE_LIMIT, RESOURCE_EXHAUSTED, ...).

Ids declared in src/rpc/rpc-methods.ts (all, with usage count from grep in services):
| RPC id | Name | Purpose / request args as built in code | file:line |
|---|---|---|---|
| wXbhsf | LIST_MY_NOTEBOOKS | list; `[null,1,null,[2]]` | rpc-methods.ts:11; services/notebooks.ts:14-18 |
| rLM1Ne | GET_PROJECT / POLL_SOURCE_PROCESSING | notebook details `[nbId,null,[2],null,0]`; source-processing status `[nbId,null,[2],null,1]` (same id, last param 1) | notebooks.ts:31-35; sources.ts:2989 |
| CCqFvf | CREATE_PROJECT | `[title,null,null,[2],[1,null*8,[1]]]` (note: only 8 nulls, one fewer than roomi/mood; probable bug or old shape) | notebooks.ts:59-62 |
| WWINqb | DELETE_PROJECTS | `[[id],[2]]` one at a time | notebooks.ts:153,172,192 |
| s0tc2d | UPDATE_PROJECT / SET_CHAT_CONFIG | `[nbId, updates]`; chat config `[nbId, [[null*7, configArray]]]`; emoji/metadata | notebooks.ts:105; generation.ts:115-123 |
| izAoDd | ADD_SOURCES | add (url, text, youtube, drive, ...); 10 call sites | sources.ts:1188 etc. |
| o4cbdc | UPLOAD_FILE_BY_FILENAME | file add, `[[ [fileName,13] ], nbId, [2], [1,null*9,[1]]]`; the file bytes are base64-encoded into a variable but never put in this request (sources.ts:1054-1070); no resumable upload step exists, so file upload only registers a name [as read; not tested] | sources.ts:1059 |
| tGMBJ | DELETE_SOURCES | `[[[srcId]],[2]]` with source-path | sources.ts:2870-2880 |
| b7Wfje | MUTATE_SOURCE | rename source | sources.ts:2930 |
| FLmJqe | REFRESH_SOURCE | re-fetch | rpc-methods.ts:46 |
| hizoJc | LOAD_SOURCE | `[[srcId],[2],[2]]` (deprecated here, "API reliability issues", sources.ts:3040-3050) | sources.ts:3046 |
| yR9Yof | CHECK_SOURCE_FRESHNESS | | sources.ts:3110 |
| tr032e | LOAD_SOURCE_CONTENT / source guide | `[[[srcId]]]` | sources.ts:3080 |
| Ljjv0c | SEARCH_WEB_SOURCES | research start: `[[query, sourceType], null, mode, nbId]` (comment: "from curl"); Drive supports fast only | sources.ts:108-125 |
| e3bVqc | GET_SEARCH_RESULTS | poll | sources.ts:2407 |
| LBwxtb | ADD_DISCOVERED_SOURCES | import | sources.ts:2633 |
| QA9ei | ADD_DEEP_RESEARCH_REPORT | add deep report as source | sources.ts:3180 |
| CYK0Xb / cYAfTb / AH0mwd / cFji9 | notes create / mutate / delete / get | create `[nbId, content, noteType, null, title]`; list `[nbId]` | notes.ts:60-75, 28-40 |
| R7cb6c | CREATE_VIDEO_OVERVIEW and CREATE_REPORT | studio create (video, report, slides, infographic, flashcards, quiz per artifacts.ts; shapes see Kisara note) | artifacts.ts:1943, 5133 |
| xpWGLf | CREATE_ARTIFACT | older "create artifact" path used for non-audio/video types | artifacts.ts:2311 |
| AHyHrd / VUsiyb / Fxmvse | CREATE/GET_AUDIO_OVERVIEW, GET_AUDIO_DOWNLOAD | legacy audio: `[nbId, audioType, [instructions]]`; get tries request types 0..5 | artifacts.ts:2359-2380, 3297 |
| gArtLc | LIST_ARTIFACTS | poll/list | artifacts.ts:1508 |
| BnLyuf / WxBZtb / V5N4be | GET / DELETE ARTIFACT / DELETE_AUDIO_OVERVIEW | audio and video delete `[[2], artifactId]` via V5N4be; others `[artifactId]` via WxBZtb (artifacts.ts:526-545) | artifacts.ts |
| rc3d8d / v9rmvd / Krh3pd | RENAME_ARTIFACT / GET_QUIZ_DATA / EXPORT_REPORT | | artifacts.ts:482, 3502, 5185 |
| yyryJe | ACT_ON_SOURCES (mind map) | `[[[[src]]..],null*4,["interactive_mindmap",[[instructions||"[CONTEXT]",""]],"",null,[2,null,[1]]]]` | artifacts.ts:2240-2262 |
| VfAZjd | GENERATE_NOTEBOOK_GUIDE | | rpc-methods.ts:129 |
| QDyure / JFMDGd | SHARE_PROJECT / GET_SHARING_DETAILS | share: users `[nbId, [[email,null,role]..]]` roles 2 editor, 3 viewer, 4 remove; link `[nbId,null,[1]]`; client first calls JFMDGd `[nbId,[2]]` "to initialize sharing state" (notebooks.ts:255-283) | notebooks.ts |
| (declared, unused by services) hT54vc, qXyaNe, lCjAd, BeTrYd, exXvGf, pGC7gf, GHsKob, ZwVcOc, rJKx8e, RGP97b, ARGkVc, EYqtU, YJBpHc, R6smae, LJyzeb, OTl0K, itA0pc, ozz5Z | list-projects legacy, discover-sources, outline/section/draft generation, report suggestions, get-or-create account, report content, share audio, "guidebook" family, ozz5Z ("called after slide deck creation, purpose unknown") | rpc-methods.ts:14,58,132-150,157,172-203,236,246 |

The names `Guidebooks` / `NotebookLMSharing` / `LabsTailwindOrchestrationService` in the comments (rpc-methods.ts:167-206) are service names from Google's internal proto, useful hints for the id families; none of the guidebook ids is exercised here, so treat as [unverified].

## Streaming/chat

- Endpoint: `POST https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=boq_labs-tailwind-frontend_20260101.17_p0&f.sid=<const>&hl=en&authuser=0&pageId=none&_reqid=<counter>&rt=c&source-path=/notebook/{id}` (src/utils/streaming-client.ts:590-612). Differences from roomi/moodRobotics: includes `authuser`, `pageId=none`, and `source-path`. Counter starts random near 3114440 and increments.
- Extra headers (streaming-client.ts:618-633): `x-goog-ext-353267353-jspb: [null,null,null,282611]`, `sec-fetch-*`, UA Chrome 143. This header is not sent by the other two implementations.
- Body: `f.req=JSON([null, JSON([contextItems, prompt, null, [2,null,[1]], notebookId])])&at=...` where contextItems = `[[sourceId]]` per source (if none, `[[conversationId]]`). NOTE the last element is the NOTEBOOK ID here, but is a conversation UUID in roomi/mood (query-rpc.ts:55-57 in roomi). Conflict; both apparently work; photon comment says "notebookId as last parameter in request body" is a critical fix (streaming-client.ts:16-18) [unverified which is canonical].
- Streaming is truly incremental: `response.body.getReader()` loop with a buffer and aggressive drain after end; yields `StreamChunk` (chunkNumber, byteCount, text, thinking headers (bold), response, metadata `[convId, msgId, ts]`, messageIds, citations numbers, isError/errorCode) (streaming-client.ts:136-214, types at :27-60). Chunks arrive as `<byte_count>\n[["wrb.fr",null,"<escaped json>"]]` (header comment :20-23). "Thinking" vs "response" separation by bold formatting [per header comment]. History: `conversationHistory` is "always null" (streaming-client.ts:132).

## Auth lifecycle

- Credentials = `{authToken (SNlM0e), cookies (header string)}` (src/auth/auth.ts:17). Resolution order in `getCredentials` (auth.ts:358-400): explicit config -> env `NOTEBOOKLM_AUTH_TOKEN` + `NOTEBOOKLM_COOKIES` -> saved `credentials.json` in the CURRENT WORKING DIRECTORY (auth.ts:36-48, plaintext) unless `FORCE_REAUTH=true` -> auto-login with `GOOGLE_EMAIL`/`GOOGLE_PASSWORD` via Playwright (headless default true, auth.ts:94-135, 312-356). Auto-login with a password in env is the Avoid.
- Extraction: opens notebooklm.google.com, reads `window.WIZ_global_data.SNlM0e` (auth.ts:158-190), cookies = `document.cookie` merged with `context.cookies('https://notebooklm.google.com')` filtered by domain; if cookie string < 100 chars, prompts the user to paste cookies (auth.ts:259-305).
- Token refresh: `RefreshClient.refreshCredentials()` POSTs `https://signaler-pa.clients6.google.com/punctual/v1/refreshCreds?key=<API key>[&gsessionid=]` with `Authorization: SAPISIDHASH <ts>_<sha1(ts + " " + SAPISID + " " + origin)>`, `Content-Type: application/json+protobuf`, body `["8hcNI0LV"]`, origin = `https://notebooklm.google.com` (src/auth/refresh.ts:14-22, 77-135, 160-170). API key is the embedded public web key `AIzaSyC_pzr...` overridable by `NOTEBOOKLM_SIGNALER_API_KEY`. It does not return new cookies (comment :119-128); it appears to extend the session server-side [inferred from comment]. `AutoRefreshManager` schedules refresh: time-based default 10 min plus expiry-based check (default every 1 min, refresh ahead) (refresh.ts:235-290, 342-430, 436-479).
- Different refresh mechanism from moodRobotics (`accounts.google.com/RotateCookies`): worth supporting both [both unverified against live Google by me].
- Multi-account: `authuser` index param (default '0', rpc-client.ts:56); no pooling.

## MCP tools

None: it is an SDK. (Service surface: notebooks.list/get/create/update/delete/share/setEmoji, sources.add* (url, text, youtube, drive, file with auto-chunking), sources.status/delete/rename/search/importResearch/checkFreshness, artifacts.create/get/list/delete/rename/download/export per type, notes CRUD, generation.chat/streamChat/setChatConfig/guide; see README.)

## Versions

2.0.0 2025-12-26 -> 2.1.1/2.1.2 2026-01-10 -> 2.2.0 2026-01-12 "bigger files are auto-handled through dividing" (1a91794). Then only CODEOWNERS (2026-03-06). Effectively unmaintained since mid-January 2026; the Kisara-k fork carried it to the 2026-09 host change.

## Reliability / quota

- `QuotaManager` encodes plan limits: standard 100 notebooks, 50 sources/notebook, 500k words/source, 200 MB, 50 chats/day, 3 audio/day; plus 200/100/6; pro 500/300/20; ultra 500/600/5000 chats and 200 audio (src/utils/quota.ts:52-120, reference https://support.google.com/notebooklm/answer/16213268). Counted client-side only, not persisted [unverified].
- Auto-chunking of oversized text/binary into multiple sources by word/size caps (sources.ts:1030-1160, parallel via `Promise.all`).
- Tests: none in repo (examples/ only).

## Adopt

1. Streaming reader with chunk metadata (conversation id, message ids, thinking vs response): the only implementation here that really streams token chunks to the caller (streaming-client.ts:136-214). Map to MCP progress notifications.
2. SAPISIDHASH + signaler `refreshCreds` keepalive (refresh.ts:77-170) as an additional no-browser session keep-alive, and the expiry-ahead scheduler.
3. Exponential backoff on 429/5xx with env overrides (batch-execute.ts:69-80,135-148,409-411).
4. `authuser` and `pageId=none` params, `x-goog-ext-353267353-jspb` header: capture and compare with live traffic before deciding which are required.
5. Plan-limit table (quota.ts) as default guard rails, but fetch/verify rather than hard-code.
6. The `JFMDGd`-before-`QDyure` sharing handshake (notebooks.ts:255-283) as a lead; confirm on the wire.
7. Per-id rpc-methods catalog including Guidebook/Sharing families as hints for future features.

## Avoid

1. Hard-coded `f.sid` and `bl` constants (rpc-client.ts:53-54) and `at` never refreshed from the page; the SDK depends on user-supplied token with a 1 h assumed lifetime.
2. `credentials.json` plaintext in CWD and `GOOGLE_PASSWORD` env auto-login (auth.ts:36-48, 389-390).
3. Payload bugs: CREATE_PROJECT tail with 8 nulls (notebooks.ts:61), file upload that never sends bytes (sources.ts:1054-1070), the `o4cbdc` id used as "upload file by filename"; slides/infographic options at wrong indices fixed in Kisara fork.
4. Silent filter that drops a notebook titled "OpenStax's Biology" from list results (notebooks.ts:20-21) - author's test data leaked into library code.
5. Trial loops over request types 0..5 for audio fetch (artifacts.ts:2374-2390) and try-both-delete fallbacks: guessing instead of verified protocol.
6. 6006-line artifacts.ts and 3349-line sources.ts: no layering.
7. `console.warn` deprecations in library code and "Debug logging disabled" stubs.

## Evidence

Read in full: src/rpc/rpc-methods.ts, src/rpc/rpc-client.ts, src/utils/batch-execute.ts, src/auth/refresh.ts (1-230). Read in part: src/utils/streaming-client.ts (1-215, 590-703), src/auth/auth.ts (1-60, grep), src/utils/quota.ts (1-125), src/services/notebooks.ts (grep + 10-70, 255-290), notes.ts, sources.ts (1030-1110, 2870-2880, 2980-3050, 108-125), artifacts.ts (grep + 520-550, 2240-2262, 2295-2380), generation.ts (115-130), README.md:480-535. `git tag`, `git log`.
