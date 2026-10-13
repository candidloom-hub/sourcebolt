# agmmnn/notebooklm-sdk

- URL: https://github.com/agmmnn/notebooklm-sdk (npm `notebooklm-sdk`)
- Language / runtime: TypeScript SDK, ESM+CJS via tsup, Node `fetch`, Playwright/patchright only for `login` (src/auth/browser.ts), Vitest tests, Biome lint (package.json, tsup.config.ts, vitest.config.ts)
- License: MIT (LICENSE: "Copyright (c) 2026 agmmnn"). Snippets under 15 lines allowed.
- Stars / last commit: 28 stars (research/upstreams.md); HEAD e3be717 2026-03-18 "docs: update readme"
- Versions reviewed: HEAD = 0.3.4 (package.json). 14 tags v0.1.1 (2026-03-13) .. v0.3.4 (2026-03-16): the whole library was written in 4 days.
- REA ran (444 KB md, unreadable node output); direct reading used. Source is only 4608 lines in src/ (largest: api/artifacts.ts 931, api/sources.ts 493, api/chat.ts 413, types/enums.ts 396).
- It is a TypeScript port of Python `teng-lin/notebooklm-py` ("reaching parity with the Python SDK", CHANGELOG.md 0.3.0). Not an MCP server.
- Staleness: uses host `notebooklm.google.com` everywhere and whitelists cookie domains that exclude `notebook.google.com` (see Auth); not updated for the 2026-07/08 host move seen in roomi-fields and Kisara-k.

## SDK design (the reason this repo is in the batch)

- One thin `RPCCore` class owns transport: `call(methodId, params, {sourcePath, allowNull, timeoutMs})` (src/rpc/core.ts:36-117). Resource classes (`NotebooksAPI, SourcesAPI, ArtifactsAPI, ChatAPI, NotesAPI, ResearchAPI, SettingsAPI, SharingAPI`) take the `RPCCore` and are wired in `NotebookLMClient` (src/client.ts:30-55). Each API file maps 1:1 to a resource group and only builds param arrays and parses nested results. Clean layering: encoder (src/rpc/encoder.ts, 37 lines), decoder (src/rpc/decoder.ts, 199), core (148). Compare with the monolithic files in photon-hq and roomi-fields.
- Typed errors (src/types/errors.ts): `AuthError, RateLimitError (retryAfter), RPCError (methodId, rpcCode, foundIds, rawResponse), RPCTimeoutError, NetworkError, ServerError, ClientError, ChatError, SourceProcessingError, SourceTimeoutError`; HTTP status mapped in core.ts:81-112 (429 -> RateLimitError with `Retry-After`, 401/403 -> AuthError after one auto-refresh, >=500 -> ServerError, other 4xx -> ClientError).
- Auth refresh de-duplication: `refreshTokens()` memoizes an in-flight promise so concurrent 401s trigger one refresh (src/client.ts:72-81). Core retries a failed call exactly once after refresh (core.ts:91-95, `retried` flag).
- `allowNull` per call: some RPCs legitimately return null (delete, rename, etc.), others must not; missing id vs null is distinguished.
- Drift diagnostics: `decodeResponse` collects all RPC ids present in the response (`collectRPCIds`) and, when the wanted id is absent, errors with "Response has IDs: ... Method ID may have changed." (decoder.ts:184-196). Same idea as roomi's `RpcDriftError`.
- `pollUntilReady` helper with timeout, interval, `onTick`, `AbortSignal`, and a stricter rule that audio/video are ready only when a usable media URL exists (CHANGELOG 0.3.3/0.3.4; api/artifacts.ts:533-596).
- Exponential backoff for source readiness: 1 s initial, x1.5, cap 10 s, default 120 s timeout (api/sources.ts waitUntilReady ~lines 403-430).
- Public sub-exports `./errors` and `./auth` (package.json exports).
- Context7 config (`context7.json`, `DOCS.md`) so agents can query docs.

## Access method

Internal `batchexecute` + streaming endpoint, plain fetch, no browser at runtime. `npx notebooklm-sdk login` opens a patchright persistent context and saves a Playwright storage-state file; `whoami` validates (src/bin.ts:10-46).

## RPC catalog

Endpoint: `POST https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute?rpcids={id}&source-path={path}&f.sid={FdrFJe}&hl=en&rt=c` (src/rpc/core.ts:15, src/rpc/encoder.ts:173-185). NOTE: no `bl` param at all, no `_reqid`, no `authuser`, and headers are only `Content-Type` and `Cookie` (core.ts:56-64): no Origin/Referer/X-Same-Domain/X-Goog-Csrf-Token. This minimal request evidently works (the repo's own e2e usage), which is the minimal viable set to compare with roomi (which sends more).
Body: `f.req=<urlenc([[[id, JSON(params), null, "generic"]]])>&at=<urlenc(SNlM0e)>&` (encoder.ts:155-168). 30 s default timeout via AbortController (core.ts:17,51-52).
Tokens: `GET https://notebooklm.google.com/` with Cookie, `redirect: "follow"`, error if final URL contains `accounts.google.com` or `signin`; regex `"SNlM0e"\s*:\s*"([^"]+)"` and `"FdrFJe"\s*:\s*"([^"]+)"` (src/auth.ts:125-184).
Decode: strip `)]}'\r?\n`; split lines; digit-only line = byte count so parse the next line; otherwise parse line as JSON; if more than 10% of lines fail to parse, throw (decoder.ts:18-69). Result: `["er", id, code]` -> RPCError with mapped message for 429/401/403/404/5xx; `["wrb.fr", id, data, ..., status?]`: if data is null and `item[5]` contains the substring `UserDisplayableError` anywhere -> `RateLimitError` ("API rate limit or quota exceeded") (decoder.ts:71-78,127-138). This is a different (and useful) quota signal than roomi's gRPC status 8 reading; the wire shape of `item[5]` carrying `UserDisplayableError` is [unverified; implemented from teng-lin's Python].

| RPC id | Name | Request params | Response / note | file:line |
|---|---|---|---|---|
| wXbhsf | LIST_NOTEBOOKS | `[null,1,null,[2]]` | `result[0]` rows | api/notebooks.ts:15 |
| CCqFvf | CREATE_NOTEBOOK | `[title,null,null,[2],[1]]` (short tail, unlike others) | parseNotebook(result) | notebooks.ts:23 |
| rLM1Ne | GET_NOTEBOOK | `[nbId,null,[2],null,0]`, path `/notebook/{id}`; source ids = `data[0][1][i][0][0]` | | notebooks.ts:29; rpc/core.ts:119-147 |
| s0tc2d | RENAME_NOTEBOOK (MutateProject) | rename `[nbId,[[null,null,null,[null,title]]]]`; chat config `[nbId,[[null*7,[goalArray,[length]]]]]`; view level `[nbId,[[null*8,[[level]]]]]` (slot 8) | | notebooks.ts:44; chat.ts configure; sharing.ts:41 |
| WWINqb | DELETE_NOTEBOOK | `[[nbId],[2]]` | | notebooks.ts:38 |
| VfAZjd | SUMMARIZE | `[nbId,[2]]` | summary + suggested topics | notebooks.ts:53,80 |
| fejl7e | REMOVE_RECENTLY_VIEWED | `[nbId]` | removes from recents (new id) | notebooks.ts:69 |
| RGP97b | SHARE_ARTIFACT | `[shareOptions, nbId(, artifactId)]` | share audio/artifact (id also in photon rpc-methods) | notebooks.ts:118 |
| izAoDd | ADD_SOURCE | URL: `[[[null,null,[url],null*5]], nbId, [2], null, null]` (short form); YouTube: `[[ [null*7,[url],null,null,1] ], nbId, [2], [1,null*9,[1]]]`; Drive: `[[ [[fileId,mime,1,title],null*9,1] ], nbId,[2],[1,null*9,[1]]]`; text via same id | `extractSourceId(result)`; this SDK does not use `ozz5Z` | sources.ts:45-75,114-150 |
| o4cbdc | ADD_SOURCE_FILE | step 1 of upload: `[[[fileName]], nbId, [2], [1,null*9,[1]]]` returns SOURCE_ID | sources.ts:159-190 |
| (HTTP) | Resumable upload | see below | | sources.ts:213-271 |
| hizoJc | GET_SOURCE | `[[srcId],[2],[2]]`; title `result[0][1]`, url `result[0][2][7][0]`, text from `result[3][0]` tree | getFulltext | sources.ts:305-340 |
| tr032e | GET_SOURCE_GUIDE | `[[[[srcId]]]]`; summary `result[0][0][1][0]`, keywords `[0][0][2][0]` | | sources.ts:275-300 |
| yR9Yof | CHECK_SOURCE_FRESHNESS | `[null,[srcId],[2]]`; fresh if `result[0][1]===true` | | sources.ts:342-356 |
| tGMBJ | DELETE_SOURCE | `[nbId,[srcId],[2]]` (differs from `[[[srcId]],[2]]` in roomi/mood) | | sources.ts:358 |
| FLmJqe | REFRESH_SOURCE | `[nbId, srcId, [2]]` (differs from mood `[null,[srcId],[2]]`) | | sources.ts:367 |
| b7Wfje | UPDATE_SOURCE | rename (params not re-read) | | sources.ts:376 |
| R7cb6c | CREATE_ARTIFACT | `[[2], nbId, content]`; content sparse array with `[2]=typeCode` (1 audio, 2 report, 3 video, 4 quiz/flashcards, 5 mind map list type, 7 infographic, 8 slides, 9 data table; enums.ts:59-68), `[3]=[[[src]]..]`, options at idx 6 (audio `[null,[instr,length,null,[[src]..],lang,null,format]]`), 7 (report `[null,[title,desc,null,[[src]..],lang,prompt,null,true]]`), 8 (video `[null,null,[[[src]..],lang,instr,null,format,style]]`), 14 (infographic `[[instr,lang,null,orientation,detail,style]]`), 16 (slides `[[instr,lang,format,length]]`), 9 / 18 for quiz-flashcards / data table (not re-read) | `result[0][0]` id, `[0][4]` status | artifacts.ts:207-486,826-857 |
| gArtLc | LIST_ARTIFACTS | `[[2], nbId, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | rows; `item[4]` status (1 processing, 2 pending, 3 completed, 4 failed) | artifacts.ts:138-146; enums.ts:76-81 |
| V5N4be | DELETE_ARTIFACT | `[[2], nbId, artifactId]` (third variant; roomi `[[2],artifactId]`, mood `[[2],nbId,[artifactId]]`) | | artifacts.ts:185-190 |
| rc3d8d | RENAME_ARTIFACT | `[[2], nbId, artifactId, newTitle]` | | artifacts.ts:194-196 |
| Krh3pd | EXPORT_ARTIFACT | `[null, artifactId, null, title, ExportType]` (docs / sheets) | returns doc URL | artifacts.ts:761-785 |
| v9rmvd | GET_INTERACTIVE_HTML | `[artifactId]` (quiz/flashcard data) | | artifacts.ts:636-650 |
| KmcKPe | REVISE_SLIDE | `[[2], artifactId, [[[slideIndex, prompt]]]]` | per-slide revision (roomi has the id only) | artifacts.ts:730-745 |
| ciyUvf | GET_SUGGESTED_REPORTS | `[[2], nbId]` | report suggestions (new id) | artifacts.ts:706-728 |
| yyryJe | GENERATE_MIND_MAP | `[[[[src]]..],null*4,["interactive_mindmap",[["[CONTEXT]",""]],""],null,[2,null,[1]]]`; result[0][0] is JSON string; does not persist; then CREATE_NOTE | | artifacts.ts:489-531 |
| CYK0Xb / cFji9 / cYAfTb / AH0mwd | notes | update `[nbId, noteId, [[[content, title||"New Note", [], 0]]]]`; delete `[nbId, null, [noteId]]`; list `[nbId]` | | notes.ts:27-71 |
| Ljjv0c / QA9ei / e3bVqc / LBwxtb | research | fast `[[query,type],null,1,nbId]`; deep `[null,[1],[query,type],5,nbId]`; poll `[null,null,nbId]`; import `[null,[1],taskId,nbId,sourceArray]` | | research.ts:61-65,87,224 |
| hPTbtc / khqZz | chat history | last conversation id `[[],null,nbId,1]`; turns `[[],null,null,convId,limit]` (turn row: `[?,?,2 or 1,query|..., [[answer]]]`; walk reversed, type 1 = user, 2 = model) | | chat.ts getLastConversationId/getConversationTurns |
| QDyure / JFMDGd | share | status `[nbId,[2]]`; public `[[[nbId,null,[access],[access,""]]],1,null,[2]]`; user `[[[nbId,[[email,null,permission]],null,[msgFlag,msg]]], notifyFlag, null,[2]]`; remove permission `_REMOVE` with `[0,""]`, notify 0 | | sharing.ts:17-97 |
| ZwVcOc / hT54vc | GET/SET_USER_SETTINGS | get `[null,[1,null*9,[1]]]`; set output language `[[[null,[[null,null,null,null,[language]]]]]]` with path `/` | account-level default output language (a cleaner way to set content language than per-request `hl`) | settings.ts:9,25 |

Resumable file upload (src/api/sources.ts:213-271), the only complete implementation of file add in this batch:
1. RPC `o4cbdc` registers `[[fileName]]` and returns the new SOURCE_ID.
2. `POST https://notebooklm.google.com/upload/_/?authuser=0` with headers `x-goog-authuser: 0`, `x-goog-upload-command: start`, `x-goog-upload-header-content-length: <bytes>`, `x-goog-upload-protocol: resumable`, `Origin`, `Referer`, `Cookie`, body JSON `{PROJECT_ID, SOURCE_NAME, SOURCE_ID}`; response header `x-goog-upload-url` is the session URL.
3. `POST <session url>` with `X-Goog-Upload-Command: upload, finalize`, `X-Goog-Upload-Offset: 0`, body = file bytes (single shot).
No chunked resume is implemented; MIME type is ignored (`_mimeType`). This fills the gap in photon-hq (file add never sent bytes) and moodRobotics (text-only).

## Streaming/chat

- `POST https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=<DEFAULT_BL boq_labs-tailwind-frontend_20260301.03_p0 or env NOTEBOOKLM_BL>&hl=en&_reqid=<n>&rt=c&f.sid=<FdrFJe>` (src/api/chat.ts:5-8,69-76). Only Cookie + Content-Type headers (chat.ts _postChatRequest). `_reqid` starts random 100000-999999, +100000 per call.
- Params (chat.ts:56-66): `[sourcesArray ([[id]] each), query, history|null, [2,null,[1],[1]], conversationId, null, null, notebookId, 1]`, body `f.req=[null, JSON(params)]&at=<csrf>&`. Differences from the others: mode block is `[2,null,[1],[1]]` (extra `[1]`), `notebookId` appears at index 7 AND the trailing `1` at index 8, conversation id at index 4. This is the newest shape (March 2026) and the one to prefer; roomi/mood send the 5-element `[src,q,null,[2,null,[1]],convId]` form and photon puts notebookId at index 4.
- Multi-turn: client keeps a per-conversation cache of `{query, answer}` (chat.ts conversationCache) and replays it as `history` on follow-ups: array built in order `[answer,null,2]`, `[query,null,1]` for each earlier turn (`_buildHistory`). New conversation = fresh `randomUUID()` and `history: null`; server may return its own conversation id at `first[2][0]`, which replaces the local one. Server-side history can be read back with hPTbtc + khqZz. This is the missing multi-turn mechanism in roomi-fields ("history is not yet re-sent").
- Response parse (chat.ts parseStreamingResponse): same longest-marked-answer rule (type flag `typeInfo[last]==1`), conversation id from `first[2][0]`; references = every UUID found (depth<=8) in `typeInfo[3]` -> `{sourceId, title:null, url:null}`: no cited text, no citation numbers (weaker than roomi's query-rpc.ts:199-259).
- Not incremental: `response.text()`.
- Chat settings: `chat.configure(nb, goal 1 default/2 custom(<=10000 chars)/3 learning guide, length 1 default/4 longer/5 shorter, customPrompt)` via s0tc2d; `setMode` presets default [1,1], learning_guide [3,4], concise [1,5], detailed [1,4] (enums.ts:321-344).

## Auth lifecycle

- Session file: Playwright storage state at `~/.notebooklm/session.json` (src/auth.ts:7; written by `bin.ts login`). Discovery order for `NotebookLMClient.connect()`: explicit options (`cookies` string, `cookiesFile`, `cookiesObject`) -> env `NOTEBOOKLM_COOKIES_FILE` -> `~/.notebooklm/session.json` -> `./storage_state.json` -> env `NOTEBOOKLM_COOKIES` string (auth.ts:199-238). The plain-string path cannot build a media cookie header.
- Cookie selection `extractCookiesFromStorageState` (auth.ts:76-99): allowed domains exactly `.google.com`, `notebooklm.google.com`, `.googleusercontent.com`, or any `.google.*` regional domain; `.google.com` wins on name collisions; requires `SID`. Defects: `notebook.google.com` (new host) is NOT allowed, and `accounts.google.com` host cookies (`LSID`, `__Secure-1PSIDTS` scope) are dropped; both matter after the rebrand and for RotateCookies-based refresh (see moodRobotics note).
- Media cookie header: separate `googleCookieHeader` with only `.google.com` cookies for media downloads; redirects followed manually (max 10) with a trusted-domain allowlist `isTrustedDomain` and HTML content-type rejection (artifacts.ts:795-825).
- Refresh: re-fetch homepage for CSRF/session (auth.ts:153-158). No cookie refresh (no RotateCookies, no signaler). Cookie expiry => user must re-run login.
- Multi-account: none (single file); `authuser=0` hard-coded in the upload call.

## MCP tools

None. SDK surface: `client.notebooks` (list, create, get, delete, rename, summary, removeFromRecent, share artifact), `sources` (list, get, addUrl, addText, addDrive, addFile, addFileBuffer, getGuide, getFulltext, checkFreshness, delete, refresh, rename, waitUntilReady, waitForSources), `artifacts` (list + typed lists, get, create{Audio, Video, Quiz, Flashcards, Infographic, SlideDeck, DataTable, Report, MindMap}, pollStatus, pollUntilReady, download{Audio,Video,SlideDeck,Infographic}, getReportMarkdown, getInteractiveHtml, suggestReports, reviseSlide, exportReport, exportDataTable, delete, rename), `chat` (ask, getHistory, getConversationTurns, getLastConversationId, configure, setMode, cache helpers), `notes`, `research` (start fast/deep, poll, import), `settings` (get/set output language), `sharing` (status, setPublic, setViewLevel, add/update/remove user).

## Versions

v0.1.1 .. v0.1.8 (2026-03-13..15), v0.2.0, v0.3.0 (2026-03-15, "7 new API methods reaching parity with the Python SDK"), v0.3.1 Drive source + chat.configure, v0.3.2 export report/data table, v0.3.3 (2026-03-16) auto-refresh on 401/403 + `pollStatus`, v0.3.4 `pollUntilReady`. No activity after 2026-03-18.

## Reliability / tests

Vitest in `tests/` [not read in detail]; typed errors; single retry after auth refresh; no retry on 5xx or network errors; no backoff in core (only in readiness polling).

## Adopt

1. Layering: tiny transport core (encoder/decoder/core) + one API class per resource group + typed error taxonomy (src/rpc/*, src/types/errors.ts). Mirror as Go packages: `rpc` (encode, decode, call), `notebooklm` services, `errors`.
2. Chat request shape and multi-turn history replay (chat.ts:56-66, `_buildHistory`) plus getHistory via hPTbtc/khqZz. Use this to give our MCP server real conversations, with the history cached in the process and reconciled with the server-assigned conversation id.
3. Complete resumable upload sequence (sources.ts:213-271).
4. `collectRPCIds` drift diagnostic and decoder error-rate guard (decoder.ts:83-96,58-66).
5. Single in-flight token refresh promise (client.ts:72-81), `retried` flag to avoid refresh loops (core.ts:40-41).
6. User settings RPC for output language (settings.ts:25) as an account-level default; compare with roomi's finding that `hl` drives the generation language.
7. Backoff polling helpers with AbortSignal (artifacts.ts:533-596, sources.ts waitUntilReady).
8. Media fetch with manual redirects, trusted-domain allowlist, HTML sniff (artifacts.ts:795-825).
9. New ids worth capturing against live traffic: fejl7e, ciyUvf, KmcKPe, RGP97b, ZwVcOc/hT54vc, v9rmvd `[artifactId]`.
10. Minimal-request experiment: this client omits `bl`, `_reqid`, Origin/Referer/X-Same-Domain on batchexecute and still works per its authors; use as the baseline for the minimum required header set in our Go client (verify live).

## Avoid

1. Cookie-domain whitelist that excludes `notebook.google.com` and accounts-host cookies (auth.ts:101-113).
2. Hard-coded `notebooklm.google.com` in transport, auth, upload (core.ts:15, auth.ts:125, sources.ts:10,221-222, chat.ts:7).
3. Debug `console.log` of the raw response item when the result is null (decoder.ts:147-152) - leaks payloads and is noisy for legitimate null results.
4. References with only source ids: no passage text (chat.ts parse).
5. `source` delete/refresh payloads that differ from other implementations (sources.ts:358-370) - at least one variant is wrong for current backends [unverified which].
6. Single-shot upload with no resume or size cap for large files.
7. Substring match "UserDisplayableError" over the whole `item[5]` as the quota signal without status-code decoding.
8. `QuizQuantity.MORE = 2` aliasing (enums.ts) shows the API collapses values; do not expose enum values the wire does not honor.

## Evidence

Read in full: src/rpc/core.ts, encoder.ts, decoder.ts, client.ts, auth.ts, api/chat.ts (parse + ask), api/sharing.ts, api/settings.ts, api/notes.ts, api/notebooks.ts (grep), api/research.ts (grep), enums.ts (RPCMethod, codes, chat modes). Read in part: api/sources.ts (40-275, grep), api/artifacts.ts (207-532, 596-610, 795-870), bin.ts, CHANGELOG.md. `git tag`, `git log`.
