# m4yk3ldev/notebooklm-mcp

- URL: https://github.com/m4yk3ldev/notebooklm-mcp (npm `@m4ykeldev/notebooklm-mcp`; note the npm scope spelling differs from the GitHub owner)
- Language / runtime: TypeScript, Node >= 18, pnpm, tsup, vitest (100% coverage claimed, commit ffbd06c), `ws` for CDP, commander for CLI (package.json)
- License: MIT (LICENSE "Copyright (c) 2026 m4yk3ldev"). Snippets <15 lines allowed.
- Stars / last commit: 20 stars (research/upstreams.md); HEAD 477203f 2026-07-11 "chore(release): v0.3.1"
- Versions reviewed: HEAD = 0.3.1 (package.json:3). 42 tags v0.1.0 (2026-02-18) .. v0.3.1 (2026-07-11); 150 commits.
- REA ran (217 KB md, unreadable node output); direct reading. src is about 3.2k lines excluding tests (client.ts 1069, browser-auth.ts 431, transport.ts 262, auth.ts 262).
- Standout: a live E2E audit (v0.3.1) that proved which wire payloads Google rejects. Also the only implementation with a pure CDP (no Playwright) cookie/token extraction and a single-flight refresh.

## Access method

Internal batchexecute RPC + streaming endpoint over fetch; no browser at runtime except for auth refresh: when an RPC returns auth error 16, the server relaunches Chrome (non-headless by default) via CDP to re-extract cookies/tokens. MCP over stdio (`notebooklm-mcp serve`, default command), 27 tools.

## RPC catalog

Transport (src/rpc/transport.ts):
- `POST https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute?rpcids={id}&bl={bl}&hl=en-US&_reqid={n}&rt=c[&f.sid={sid}]` (transport.ts:60-74). NOTE: no `source-path` is sent at all (parameter accepted but ignored: `_sourcePath`, transport.ts:98); `bl` = token captured from the page (`WIZ_global_data.cfb2h`, stored in auth.json) else env `NOTEBOOKLM_BL` else `DEFAULT_BL boq_labs-tailwind-frontend_20260419.02_p0` (constants.ts:203). `_reqid` is a simple counter 1,2,3... (transport.ts:21,61).
- Body order: `at=<csrf>&f.sid=<sid>&f.req=<urlenc>` (transport.ts:45-58): `f.sid` also in the body; `f.req=[[[id, JSON(params), null, "generic"]]]`. No trailing `&`.
- Headers (transport.ts:28-43): Content-Type form, Origin, Referer, Cookie, `X-Same-Domain: 1`, Linux Chrome 133 UA, `sec-ch-ua`, `sec-ch-ua-mobile`, `sec-ch-ua-platform: "Linux"`, `X-Goog-Encode-Response-If-Executable: base64`, `X-Google-SIDRT: 1`. The streaming endpoint additionally gets `X-Goog-BatchExecute-Path: <QUERY_PATH>` (transport.ts:158-161). These extra browser-mimic headers are not used by agmmnn or roomi; whether they matter is unknown.
- Cookie handling: reads `Set-Cookie` from every response via `getSetCookie()` and merges into the jar and persists (transport.ts:121-131, auth-state.ts:320-329). Naive parse (`split(";")[0].split("=")`) truncates values containing `=` [bug: `parts[1]` only].
- `af.httprm` envelope: when an item `["af.httprm", ?, "<sid>"]` appears in a response, the session id (`f.sid`) is updated from it and persisted (client.ts:62-72, auth-state.ts:306-310). I.e. Google rotates `f.sid` in responses.
- Error decoding (client.ts:77-93): for `wrb.fr` with matching id, `item[5]` non-empty array: includes 16 -> `AuthenticationError`; otherwise `Error("... failed with error code(s) [..]")` (no code table). Missing id returns null silently.
- Parse (src/rpc/wire.ts:6-51): standard "byte count line then JSON line" with warnings for unparseable chunks ("silent skips here used to hide upstream API shape drift entirely", wire.ts:30-31).
- Timeouts: default 30 s, extended 120 s (constants.ts:216-217); query timeout CLI `--query-timeout` default 120000. A `readBodyWithAbort` guards against hangs after headers arrive (transport.ts:226-261).

| RPC id | Name | Request payload (as sent) | Response / note | file:line |
|---|---|---|---|---|
| wXbhsf | LIST_NOTEBOOKS | (client.ts:227) `[null,1,null,[2]]` presumed; max 100 | row: title, sources, id, ..., ownership at meta[0] (1 mine, 2 shared) | constants.ts:44,67-69 |
| rLM1Ne | GET_NOTEBOOK | with source-path (ignored) | | client.ts:243 |
| CCqFvf | CREATE_NOTEBOOK | title + old flat tail | | client.ts:252 |
| s0tc2d | RENAME_NOTEBOOK | `[nbId,[[null,null,null,[null,title]]]]` | | client.ts:259-265 |
| WWINqb | DELETE_NOTEBOOK | `[[nbId],[2]]`; bare `[nbId]` fails INVALID_ARGUMENT (code 3), verified live | | client.ts:267-276; commit 4b83e34 |
| VfAZjd | GET_SUMMARY | `[nbId,null,[2]]` (agmmnn/mood use `[nbId,[2]]`) | `data[0]` string | client.ts:278-288 |
| izAoDd | ADD_SOURCE | URL/YouTube: `[[sourceData], nbId, [2], [1,null*9,[1]]]` (old flat tail, works); text `[null,[title,text],null,2,null*6,1]`; Drive in addDriveSource | `data[0][0]` -> `[0][0]` id, `[1]` title; 120 s timeout | client.ts:290-402 |
| hizoJc | GET_SOURCE | `[[sourceId]]` plus selectors? (line shows `[[sourceId]]`, then content from `data[3]` via wire.extractTextFromBlocks path `block[2][2][0][0][2][0]`) | title `meta[1]`, type code `meta[3][1]` mapped by SOURCE_TYPES (1 google_docs, 2 slides/sheets, 3 pdf, 4 pasted_text, 5 web_page, 8 generated_text, 9 youtube, 11 uploaded_file, 13 image, 14 word_doc) | client.ts:404-423; constants.ts:83-94 |
| tr032e | GET_SOURCE_GUIDE | `[sourceId, nbId, [2]]` (differs from `[[[[srcId]]]]` in agmmnn/mood) | summary `data[0]`, keywords `data[1]` | client.ts:425-440 |
| FLmJqe | SYNC_DRIVE | `[sourceId, nbId]` per source | | client.ts:441-450 |
| tGMBJ | DELETE_SOURCE | `[[[sourceId]],[2]]`, notebook id only in source-path | | client.ts:452-463 |
| (streaming) | QUERY | see below. NOTE constants declare `QUERY: "ZAnZ8"` as an RPC id (constants.ts:53), unused for the HTTP request [unverified meaning; likely the rpc name of GenerateFreeFormStreamed] | | |
| hPTbtc | GET_CONVERSATIONS | declared only | | constants.ts:54 |
| ozz5Z | SUBSCRIPTION | declared as "SUBSCRIPTION" (roomi calls the same id ADD_SOURCE_URL_V2; photon "called after slide deck creation"); three conflicting names for one id: needs live capture | | constants.ts:55 |
| ZwVcOc | SETTINGS | `[null,1]`, used as the post-refresh warm-up call (client.ts:139-142) | agmmnn: GET_USER_SETTINGS | constants.ts:56 |
| Ljjv0c / QA9ei / e3bVqc / LBwxtb | research start fast/deep, poll, import | fast `[[query, sourceCode],null,1,nbId]`; deep `[null,[1],[query,sourceCode],5,nbId]`; poll `[null,null,nbId]` (web=1, drive=2; modes fast=1, deep=5; result types web 1, google_doc 2, slides 3, deep_report 5, sheets 8) | | client.ts:609-708; constants.ts:71-80 |
| R7cb6c | CREATE_STUDIO | `[[2], nbId, content]` | returns artifact id `data[0]` | client.ts:717-1030 |
| gArtLc | POLL_STUDIO | `[[2], nbId]` (no filter string; others add `'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"'`) | row `[id,title,type,sources,status,...]`; statuses 1 pending, 2 generating, 3 completed, 4 failed | client.ts:1030-1064 |

Studio content layouts it ships (live-tested OK per commit 4b83e34 audit, since not removed): audio options idx 6 `[null,[focus,length,null,[[id]..],lang,null,format]]`; report idx 7 `[null,[title,desc,null,[[id]..],lang,prompt,null,true]]`; infographic idx 14 `[[focus,lang,null,orientation,detail]]` (5 elements; the fork-verified layout has a 6th trailing `1`); flashcards type 4 idx 9 `[null,[1,null*5,[difficulty,count]]]`; quiz type 4 idx 9 `[null,[2,null*6,[questionCount,difficulty]]]` (note: count/difficulty order is reversed for flashcards vs quiz, same as the asymmetry comment in moodRobotics client.ts); video and slide deck functions exist (video idx 8; slides idx 16 per fork). This matches roomi/agmmnn/Kisara layouts, strengthening them as canonical.
Removed because Google answers INVALID_ARGUMENT (3) and the right shape is unknown (commit 4b83e34, 2026-07-11): `chat_configure` (RPC `hT54vc` "PREFERENCES" with the guessed payload), `source_list_drive` (yR9Yof soft-failed to null), `data_table_create`, `mind_map_create` (yyryJe), `studio_delete` (V5N4be). Counter-evidence: roomi (3.2.0) and moodRobotics claim working mind map, studio delete and chat settings (via `s0tc2d`, not `hT54vc`), so those removed payloads were simply wrong, e.g. the chat-config goes to s0tc2d slot 7 and V5N4be needs `[[2], artifactId]`-style shapes.

## Streaming/chat

- `POST {BASE}{QUERY_PATH}?bl=&hl=en&_reqid=<n>&rt=c[&f.sid=]` (transport.ts:76-89) with `X-Goog-BatchExecute-Path` header; body `f.req=[null, JSON(params)]&at=<csrf>&f.sid=<sid>` (client.ts query ~line 465+).
- Params (client.ts query, comment "Discovered format"): `[sourcesNested, query, history, [2,null,[1],[1]], conversationId|null, null, null, notebookId, 1]`. Same 9-element layout as agmmnn/notebooklm-sdk chat.ts:56-66, independently reached, so treat as canonical for 2026. Source nesting here is `[[[sid]]]` (3 levels) per source whereas agmmnn and roomi use `[[sid]]` (2 levels); both reportedly work [conflict; test].
- History: kept in-process per conversation id, last 10 turns (`slice(-10)`), appended as `[query,null,1]` then `[answer,null,2]` (chronological), whereas agmmnn builds `[answer,null,2],[query,null,1]` pairs. Order conflict [unverified which the server prefers; Go implementation should capture both].
- Response: wrb.fr items, `data[0][0]` is the answer text; longest wins; conversation id read from `inner[10]` (differs from agmmnn/roomi `first[2][0]`) [conflict]. No citations (`sources_used: []` always), no thinking/answer type filter, so a "thinking" chunk longer than the answer could win [bug].
- On auth error: reload from disk if fresher, else CDP refresh then warm-up `SETTINGS`, then retry once (client.ts:105-157, 465+).

## Auth lifecycle

- Storage: `~/.notebooklm-mcp/auth.json` with mode 0600, dir 0700 (auth.ts:9-51): `{cookies{name:value}, csrf_token, session_id, bl?, extracted_at}` plaintext JSON. Chrome profile at `~/.notebooklm-mcp/chrome-profile` (auth.ts:11, browser-auth.ts:197).
- Environment variables (auth.ts:75-96): `NOTEBOOKLM_COOKIES` (cookie header string; must contain all REQUIRED_COOKIES SID, HSID, SSID, APISID, SAPISID, constants.ts:205-211), `NOTEBOOKLM_CSRF_TOKEN`, `NOTEBOOKLM_SESSION_ID`, `NOTEBOOKLM_BL`; plus `NOTEBOOKLM_DEBUG`. Env wins over cache (`loadTokens`, auth.ts:98-109). If CSRF/SID are empty they are scraped from the landing HTML on first use (client.ts:159-179; patterns `"SNlM0e":"..."` then `at=...` then `"FdrFJe"` as last-resort csrf [bug: the FdrFJe fallback would return the session id as CSRF], session id `"FdrFJe":"..."` then `f.sid=(\d+)`, auth.ts:23-43).
- Auth CLI (cli.ts:38-40): `auth` (default smart CDP flow), `--manual` (paste Cookie header copied from DevTools Network batchexecute request, auth.ts:190-238), `--file <path>` (cookie file import with `#` comments, auth.ts:126-160), `--show-tokens`.
- CDP flow (src/browser-auth.ts): `findChrome()` per OS (browser-auth.ts:20-60); launch real Chrome with `--remote-debugging-port=0 --remote-debugging-address=127.0.0.1 --user-data-dir=<profile> --no-first-run --no-default-browser-check --disable-blink-features=AutomationControlled --disable-features=IsolateOrigins,site-per-process --disable-infobars --disable-dev-shm-usage --user-agent=<Chrome 137 Linux>` with start URL `https://notebooklm.google.com` and optional `--headless=new` (browser-auth.ts:204-226); the ephemeral port is discovered by reading Chrome's stderr for `DevTools listening on ws://127.0.0.1:<port>/` (browser-auth.ts:96-125); stale Chrome holding the profile is killed and `SingletonLock`/`DevToolsActivePort` removed first (`releaseProfile`, :142-180, commit 876a370); then `GET http://127.0.0.1:<port>/json/list` picks the page tab whose URL starts with BASE_URL, connects its `webSocketDebuggerUrl`, sends `Runtime.enable`, polls `Storage.getCookies` every 2 s (after 5 s), requires REQUIRED_COOKIES, then `Runtime.evaluate` of `JSON.stringify({href, csrf: WIZ_global_data.SNlM0e, sid: WIZ_global_data.FdrFJe, bl: WIZ_global_data.cfb2h})`; accepts only when `href` starts with `https://notebooklm.google.com` (login page = keep polling) (browser-auth.ts ~284-370). Cookies = ALL cookies of the browser (`Storage.getCookies` returns every domain, collapsed by name, last wins: duplicate names across domains can clobber; contrast moodRobotics' per-host filtering, commit e4a32c6).
- Refresh: single-flight `AuthState.refreshOnce()` (auth-state.ts:354-380): first caller runs `refreshCookiesHeadless()` (despite the name it launches NON-headless Chrome "to avoid Google's detection", browser-auth.ts ~380) with 30 s timeout; on failure falls back to interactive `runBrowserAuthFlow()` (120 s); concurrent callers await the same promise. Before that `reloadIfNewer()` compares `extracted_at` of the disk file so a separate `auth` run in another terminal is picked up.
- Security note in code: CDP bound to loopback only; comment says otherwise any LAN host could drive the logged-in session (browser-auth.ts:11-14). Google's DBSC risk not discussed.
- Host: hard-coded `notebooklm.google.com`, including the `href.startsWith` check, so for accounts served at `notebook.google.com` (see roomi/Kisara notes) the CDP flow would loop forever on "still on login page" [inferred; not tested].
- Single account.

## MCP tools (27; src/tools/*.ts, v0.3.1)

auth: refresh_auth, save_auth_tokens(cookies). notebook: notebook_list, notebook_create, notebook_get, notebook_describe, notebook_rename, notebook_delete. query: notebook_query(notebook_id, query, source_ids?, conversation_id?). research: research_start, research_status, research_import. source: source_describe, source_get_content, notebook_add_url, notebook_add_text, notebook_add_drive, source_sync_drive, source_delete. studio: audio_overview_create, video_overview_create, infographic_create, slide_deck_create, report_create, flashcards_create, quiz_create, studio_status. (README advertises 32 before the 4b83e34 removals; package.json description still says "32 tools" = stale.)

## Versions

0.1.0 2026-02-18; 0.1.2..0.1.30 daily Feb 2026 (auth hardening, anti-detection flags); 0.1.31 2026-04-15; 0.2.0-0.2.3 2026-04-21; 0.2.4-0.2.6 2026-05-17 (pre-release supply-chain audit, pnpm migration, rpc/* split, 100% coverage, 15-client integration guide); 0.2.7-0.2.9 2026-06-19 (CVE patches in ws/hono/vite, Chrome anti-detection flags, npx symlink fix); 0.3.0 2026-07-11 stale Chrome DevTools port fix; 0.3.1 2026-07-11 delete fix + 5 tools removed.

## Reliability / quality

Single-flight refresh, body-read abort, defensive Set-Cookie merge, warm-up call after refresh, 1 s settle delay; no 429/5xx retry or backoff; supply-chain CI (pnpm audit, overrides pinned to exact versions, docs/plans/2026-05-16-pre-release-supply-chain-audit.md); thorough unit tests (`__tests__/rpc-transport.test.ts`, `client-rpc.test.ts` 1129 lines, wire payload assertions e.g. `rpc-wire.test.ts`); live E2E done manually (no fixtures committed [unverified]).

## Adopt

1. Single-flight auth refresh promise shared by concurrent callers (auth-state.ts:354-380) and `reloadIfNewer()` from disk before launching a browser. In Go: `singleflight.Group` + file mtime check.
2. CDP-only auth (no Playwright): spawn system Chrome with `--remote-debugging-port=0` bound to 127.0.0.1, discover the port from stderr, use `/json/list` + WebSocket, `Storage.getCookies` + `Runtime.evaluate` of `WIZ_global_data.{SNlM0e,FdrFJe,cfb2h}` (browser-auth.ts). Zero node/playwright dependency; maps well to a Go CDP client (chromedp or raw websocket).
3. Capture and persist `bl` (`cfb2h`) at auth time and prefer it over the hard-coded default (auth.ts:62-65 comment explains the regression when dropped).
4. Learn `f.sid` rotation from `af.httprm` envelopes and merge `Set-Cookie` from every response (client.ts:62-72, transport.ts:121-131) - low-cost session keep-alive.
5. Env var contract for headless deployments: `NOTEBOOKLM_COOKIES`, `NOTEBOOKLM_CSRF_TOKEN`, `NOTEBOOKLM_SESSION_ID`, `NOTEBOOKLM_BL`; plus cookie file import and manual paste modes.
6. Process hygiene for the profile: kill stale Chrome and remove `SingletonLock`/`DevToolsActivePort` before launch (browser-auth.ts:142-180).
7. Warm-up RPC (ZwVcOc settings) after refresh before retrying the failed call.
8. Wire-payload assertion tests and the practice of deleting tools whose payloads are not live-verified rather than shipping guesses (commit 4b83e34); mirror as a "verified" flag per RPC in our matrix.
9. Auth file 0600 / dir 0700 (auth.ts:46-50); supply-chain audit script before release.
10. `readBodyWithAbort` pattern for hanging bodies (transport.ts:226-261): in Go use context deadlines on the body read.

## Avoid

1. Plaintext cookies in auth.json (0600 only); prefer keychain/encrypted store.
2. Collecting all cookies from the browser regardless of domain (browser-auth.ts `Storage.getCookies`); duplicate names collide.
3. Hard-coded host and the `href.startsWith("https://notebooklm.google.com")` login check.
4. `FdrFJe` as a CSRF fallback pattern and `at=([^&"]+)` pattern (auth.ts:24-28): can return wrong tokens.
5. Naive Set-Cookie parsing that truncates values with `=` (auth-state.ts:322-327).
6. Mimic headers (sec-ch-ua Linux, `X-Google-SIDRT`) without evidence; fixed Linux UA while running Chrome on other OSes.
7. Dropping features instead of recovering the payload: five features removed (data table, mind map, studio delete, chat configure, freshness) although other upstreams document working shapes.
8. Longest-text answer pick without the type flag (could return thinking text), no citations.
9. Non-headless "headless refresh" popping a visible Chrome window mid-session; no way to opt out.
10. Package metadata drift (tool count 32 vs 27), npm scope differing from repo owner.

## Evidence

Read in full: src/constants.ts, src/rpc/{wire,transport,auth-state}.ts, src/auth.ts, src/browser-auth.ts (1-60, 60-125 partly, 185-431), client.ts (58-180, 255-330, 404-465, 465-608, 609-708 grep, 717-1066 selected), cli.ts grep, tools/*.ts names; `git show 4b83e34` (message), CHANGELOG.md, tag list.
