# moodRobotics/notebooklm-mcp-server

- URL: https://github.com/moodRobotics/notebooklm-mcp-server (npm `notebooklm-mcp-server`)
- Language / runtime: TypeScript, Node, axios for HTTP, Playwright (Chromium) only for login, esbuild bundle (build.js), `pdf-parse` for local PDFs (package.json, src/client.ts:1-15)
- License: MIT (LICENSE). Short snippets allowed.
- Stars / last commit: 41 stars (research/upstreams.md); HEAD d1b7ca8 2026-08-22 "chore: bump version to 4.0.2"
- Versions reviewed: HEAD = 4.0.2 (package.json:3). Only 2 git tags (v3.0.6, v3.0.7, both 2026-02-15); npm is ahead of tags.
- REA: ran on the clone (139 KB md); output is a graph whose node fields print as `[object Object]`, so only a file inventory was usable. Facts below are from direct reading. Source is small: src/client.ts 2017 lines, src/server.ts 1235, src/auth.ts 229, src/constants.ts 165, src/update.ts 139.

## "Gemini Notebook rename 2026-07-16" verification

- README claim (README.fr.md:52 and 4 other translations; English README same): "Google renamed the product on 16 July 2026 (see notebook.google). Same product, same notebooks, same API surface; this server works unchanged and the npm package keeps its name." The date appears only in the READMEs, not in code. WebFetch of https://notebook.google/ returned only the page title "Gemini Notebook | AI Research Tool & Thinking Partner" with no rename text or date, so the exact date stays [unverified against a Google source] (a name change itself is corroborated by that title and by roomi-fields' CHANGELOG, which dates the DOM breakage to 2026-07-30 and says accounts moved to `notebook.google.com`).
- In code, what did NOT change: `BASE_URL = "https://notebooklm.google.com"` (src/constants.ts:53), batchexecute path `/_/LabsTailwindUi/data/batchexecute` (54), streaming path `.../LabsTailwindOrchestrationService/GenerateFreeFormStreamed` (55), `bl` `boq_labs-tailwind-frontend_20260108.06_p0` (61, a January build label still working in August 2026), all RPC ids (6-51; all identical to the pre-rename set in `git show a713940^`). Every host mention in src stays `notebooklm.google.com` (auth.ts:24,34,54,121; update flow n/a). The package docs (docs/ARCHITECTURE.md:11) still say "at `notebooklm.google.com`".
- What DID change on the wire, and is the real consequence of the "Gemini 3.5 era / migrated backends": a new nested request-options block. The old flat tail `[2], [1,null*9,[1]]` is rejected by migrated accounts with gRPC status 3, 5 or 9; the new `templateBlock()` = `[2, null, null, [1, null, null, null, null, null, null, null, null, null, [1]]]` (src/constants.ts:63-71) is used for CREATE_NOTEBOOK (client.ts: `[title, null, null, templateBlock()]`), ADD_SOURCE url/text (`[[sourceData], notebookId, templateBlock()]`), GET_NOTEBOOK (`[nbId, null, templateBlock(), null, 0]`, comment "verified safe on both migrated and un-migrated cohorts"), SUGGEST_PROMPTS, and `artifactClientOptions()` = templateBlock + `[[1,4,8,2,3,6]]` (constants.ts:77-79, "observed on current web-UI CREATE_ARTIFACT calls"). Introduced in commit a713940 (2026-08-20). Contrast: roomi-fields v3.2.0 still sends the OLD flat tail for CREATE_NOTEBOOK/ADD_SOURCE (roomi src/rpc/notebooks-rpc.ts:52-58, sources-rpc.ts:53,101) and works on its test account, and ADD_SOURCE URL v2 (`ozz5Z`) exists there; so the backend has at least two cohorts. Our client must send the nested block (accepted by both per the comment, which is the author's claim, not independently tested) and treat statuses 3/5/9 as a cohort-mismatch signal.
- Also changed by the era: artifact status enum `1 pending, 2 processing, 3 ready, 4 failed, 5 suggested` (client.ts studioStatusToString; commit message "fix artifact status mapping (1=pending, 2=in_progress, 4=failed)"), and the rotating `__Secure-1PSIDTS` requirement plus Device Bound Session Credentials (DBSC) cookies that may not replay outside the issuing browser (auth.ts:179-220, commit eda535d "document the DBSC limitation").
- Verdict: no domain or endpoint change in this repo's code; payload cohort change and cookie-binding change are the real rename-era differences. Compare roomi-fields which found the logged-in host moved to `notebook.google.com` for some accounts; this repo would fail bootstrap for such accounts [unverified; no host probing here].

## Access method

Pure internal `batchexecute` RPC over axios, no browser at runtime. Playwright used only in `notebooklm-mcp-server auth` (src/auth.ts). No browser fallback. `postinstall` runs `npx playwright install chromium` (package.json) which downloads a browser on every install.

## RPC catalog

Endpoint: `POST https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute?rpcids={id}&source-path={path}&bl={bl}&hl=en&rt=c[&f.sid={FdrFJe}]` (client.ts buildUrl ~lines 334-352). No `_reqid` on batchexecute. Body `f.req=<urlenc([[[id, JSON(params), null, "generic"]]])>&at=<urlenc(SNlM0e)>&` (buildRequestBody). Headers set on the axios instance (client.ts ~lines 89-100): `Cookie`, Chrome 143 Windows `User-Agent`, `Content-Type: application/x-www-form-urlencoded;charset=UTF-8`, `Origin`, `Referer`, `X-Same-Domain: 1`. Notably NO `X-Goog-Csrf-Token` header (roomi sends it; evidently not required).
Bootstrap: `GET /` with browser-like headers (`Accept`, `Accept-Language`, `Sec-Fetch-*`), `maxRedirects: 5`; redirect to accounts.google.com = expired; regex `"SNlM0e"\s*:\s*"..."` (csrf) and `"FdrFJe"\s*:\s*"..."` (f.sid) (client.ts init ~lines 262-310). `bl` is a hard-coded constant, NOT scraped (constants.ts:61).
Response: strip `)]}'\n`, then loop lines: a numeric line = byte count, parse the next line as JSON (client.ts parseResponse ~lines 395-430, honors the count lines by skipping, not by byte arithmetic). Result extraction: first `wrb.fr` item whose `item[1] == rpcId`; auth-expired signature `["wrb.fr", id, null, null, null, [16], "generic"]` -> AuthenticationError (extractRpcResult ~lines 438-455); any other status is NOT mapped (returns the null `item[2]`), so refusals look like empty results. Missing envelope returns null silently.

| RPC id | Name here | Purpose | Request params | Response / notes | file:line (client.ts unless noted) |
|---|---|---|---|---|---|
| wXbhsf | LIST_NOTEBOOKS | list | `[null,1,null,[2]]` | `result[0]` rows: `[title, sources, id, emoji, null, meta]`; meta[0] 1=owned 2=shared, meta[1] shared flag, meta[5]=[sec,nanos] modified, meta[8]=[sec,nanos] created | ~486-545 |
| rLM1Ne | GET_NOTEBOOK | notebook + source ids | `[nbId, null, templateBlock(), null, 0]` path `/notebook/{id}` | `data[0][1]` sources `[[id], title, ...]` | ~548-585 |
| CCqFvf | CREATE_NOTEBOOK | create | `[title, null, null, templateBlock()]` | `result[2]` = id | ~588-598 |
| WWINqb | DELETE_NOTEBOOK | delete | `[[nbId],[2]]` | | ~603 |
| s0tc2d | RENAME_NOTEBOOK (MutateProject) | rename and chat settings | rename `[nbId, [[null,null,null,[null,title]]]]`; chat config `[nbId, [[null*7, [[goalCode(,customPrompt)],[lengthCode]]]]]` (slot 7) | goal 1 default/2 custom/3 learning_guide; length 1 default/4 longer/5 shorter (constants.ts:142-153) | ~610-618, ~700-722 |
| VfAZjd | SUMMARIZE | notebook guide + suggested questions | `[nbId,[2]]` | `result[0][0][0]` summary text; `result[0][1][0]` = `[[question,prompt]..]` | ~1739-1763 |
| otmP3b | SUGGEST_PROMPTS | suggested prompts | `[templateBlock(), nbId, [[srcId]..], mode, null, queryOrNull]` mode 4 chat/8 quiz/9 flashcards/1,2 audio | raw result returned | ~1767-1795 |
| izAoDd | ADD_SOURCE | url / youtube / text / drive | url `[[ [null,null,[url],null*6,1] ], nbId, templateBlock()]`; youtube url at slot 7; text `[null,[title,content],null,2,null*6,1]`; Drive `[[docId,mime,1,title],null*9,1]` with OLD flat tail `[2],[1,null*9,[1]]` (client.ts ~668-700, inconsistent with the migration) | `result[0][0][0]` = source id; timeout 120 s | ~620-715 |
| hizoJc | GET_SOURCE | declared | unused in client.ts [grep] | | constants.ts:17 |
| tGMBJ | DELETE_SOURCE | delete | `[[[srcId]],[2]]` (no notebook id, no source-path) | | ~718 |
| FLmJqe | SYNC_DRIVE_SOURCE | refresh Drive source | `[null,[srcId],[2]]` | `result[0]` -> id,title | ~724-735 |
| yR9Yof | CHECK_FRESHNESS | freshness | `[null,[srcId],[2]]` | raw | ~1726-1735 |
| b7Wfje | UPDATE_SOURCE | rename source | `[null,[srcId],[[[newTitle]]]]` | | ~1718-1724 |
| tr032e | GET_SOURCE_GUIDE | per-source summary/topics | `[[[[srcId]]]]` | raw | ~1710-1716 |
| Ljjv0c / QA9ei | START_FAST/DEEP_RESEARCH | research | fast `[[query, 1web|2drive], null, 1, nbId]`; deep `[null,[1],[query,type],5,nbId]` | `result[0]` task id, `result[1]` report id | ~760-790 |
| e3bVqc | POLL_RESEARCH | poll | `[null,null,nbId]` | task `[id, [?, [query,type], mode(5=deep), [sources, summary], status]]`; status 2 or 6 = done; deep source rows have `src[0]==null` | ~795-865 |
| LBwxtb | IMPORT_RESEARCH | import | `[null,[1],taskId,nbId,[row..]]`; web row `[null,null,[url,title],null*7,2]`, drive row `[[docId,mime,1,title],null*9,2]`; skips type 5 (deep report) | list of `[[id],title]`; timeout 120 s | ~870-935 |
| R7cb6c | STUDIO_GENERATE | create artifacts | `[[2], nbId, [null,null,TYPE,[[[src]]..],null,null,options]]` for audio(1) video(3) report(2) infographic(7) slides(8) data table(9); quiz family type 4 with variant at `[9][1][0]` (flashcards 1, quiz 2, interactive mind map 4) | `result[0][0]` artifact id, `result[0][4]` status | ~1090-1370 |
| gArtLc | STUDIO_STATUS | poll all | `[[2], nbId, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | rows: [0] id, [1] title, [2] type (1 audio 2 report 3 video 4 flashcards 7 infographic 8 slides 9 data table), [4] status, [7][0] content text, [8] media url (as plain string here; roomi parses `a[6][5]`, `a[8][3]` etc. so the two disagree) | ~1387-1430 |
| V5N4be | STUDIO_DELETE | delete | `[[2], nbId, [artifactId]]` (roomi sends `[[2], artifactId]`) | | ~1377-1385 |
| v9rmvd | GET_INTERACTIVE_HTML | quiz/flashcards HTML + data | see artifact_content_get | | ~1555-1570 |
| rc3d8d | RENAME_ARTIFACT | rename | tool `artifact_rename` | | ~1572-1584 |
| Krh3pd | EXPORT_ARTIFACT | to Google Docs/Sheets | `[null, artifactId, null, title, 1docs|2sheets]` | | ~1586-1600 |
| yyryJe | GENERATE_MIND_MAP | mind map JSON | `[[[[src]]..],null*4,["interactive_mindmap",[["[CONTEXT]",""]],""],null,[2,null,[1]]]` | `result[0][0]` JSON string, `[0][2][0]` generation id | ~980-1000 |
| CYK0Xb / cYAfTb / cFji9 / AH0mwd | CREATE_NOTE / UPDATE_NOTE / GET_NOTES / DELETE_NOTE | notes and note-backed mind maps | create `[nbId, json|content, [2,null,null,5,[[src]..]], null, title]`; get `[nbId]`; delete `[nbId, null, [id], [2]]`; CREATE_NOTE ignores the title so follow with UPDATE_NOTE (comment ~1490); GET_NOTES has soft-deleted rows `[id,null,2]` and two row layouts | | ~1002-1075, 1494-1553 |
| hPTbtc | GET_LAST_CONVERSATION_ID | latest conversation id | `[[], null, nbId, 1]` | `[[[conversationId]]]` | ~1668-1677 |
| khqZz | GET_CONVERSATION_TURNS | chat history | `[[], null, null, conversationId, limit]` | raw turns | ~1679-1690 |
| J7Gthc | DELETE_CHAT_HISTORY | clear chat | `[[], conversationId, null, 1]` | | ~1692-1705 |
| QDyure | SHARE_NOTEBOOK | public link / collaborators | public `[[[nbId,null,[access],[access,""]]],1,null,[2]]` (access 0 restricted, 1 anyone-with-link); user `[[[nbId,[[email,null,perm]],null,[msgFlag,message]]], notifyFlag, null,[2]]`, perm editor 2, viewer 3, remove 4 | | ~1617-1660 |
| JFMDGd | GET_SHARE_STATUS | share state | `[nbId,[2]]` | raw | ~1605-1613 |

Streaming query: constant `QUERY: "Y0vGub"` (constants.ts:24) is declared but the request goes to the `GenerateFreeFormStreamed` URL, not through batchexecute; the id is unused [grep].

## Streaming/chat

- `POST {BASE}{QUERY_PATH}?bl=&hl=en&_reqid=&rt=c[&f.sid=]` (client.ts query ~1831-1900). `_reqid` starts at random 100000-999999 and adds 100000 per query. Body `f.req=<urlenc([null, JSON(params)])>&at=<csrf>&`. Params `[ [[[srcId]]...], query, null, [2,null,[1]], conversationId ]`; source ids auto-fetched via GET_NOTEBOOK when omitted; `conversationId` defaults to a fresh `randomUUID()` and can be passed back for follow-ups (tool param `conversation_id`), but history is not re-sent by the client, so whether the server continues the thread by id alone is [unverified]. Timeout 120 s (axios). Auth failure (401/403 or AuthenticationError) -> reload cookies from disk + RotateCookies + re-init, up to 2 retries.
- Parse: strip `)]}'`; each chunk: `item[0]=='wrb.fr'`, inner JSON string `item[2]`, `inner[0][0]` is text (must be > 20 chars, else ignored), `inner[0][4]` last element `== 1` marks final answer; keep longest answer else longest "thinking" chunk; fallback text `'No answer received.'` (parseQueryResponse/extractFromChunk ~1905-2010). No citations are extracted (answer text only). Contrast with roomi which extracts citations.

## Auth lifecycle

- Source: `notebooklm-mcp-server auth` launches visible Chromium (`--disable-blink-features=AutomationControlled`), user logs in, waits up to 300 s for any of: URL on notebooklm.google.com, main-app selectors, account avatar, or SID cookie (auth.ts:13-66). Hard-wired to `https://notebooklm.google.com`.
- Cookie filtering: `context.cookies('https://notebooklm.google.com')` deduped by name, plus `__Secure-1PSIDTS`, `__Secure-3PSIDTS`, `LSID` from the full jar (those live on accounts.google.com) (auth.ts:116-138). Commit e4a32c6 "filter cookies by domain to prevent CookieMismatch" (2026-02-08): sending all domains' cookies with duplicate names breaks auth.
- Required set warned on: SID, HSID, SSID, APISID, SAPISID (auth.ts:145); also needs `__Secure-1PSIDTS`.
- Storage: `~/.notebooklm-mcp/auth.json` = `{cookies: "<cookie header string>", updatedAt}` plaintext (auth.ts:10,166-176). Env `NOTEBOOKLM_COOKIES` pins cookies and disables disk persistence (docs/ARCHITECTURE.md Auth section).
- Runtime refresh (the notable part): `rotateCookies()` (client.ts ~150-235) POSTs to `https://accounts.google.com/RotateCookies` with body `[0,"-0000000000000000000"]` (fallback `[000,"-0000000000000000000"]`), headers `Content-Type: application/json`, `Origin: https://accounts.google.com`, the jar minus `OSID`/`__Secure-OSID` (host-scoped, rejected cross-host), `maxRedirects: 0`, 15 s timeout; applies `Set-Cookie` values to the jar and saves via `cookieSaver`. Precondition: jar has SID plus (OSID or APISID+SAPISID). 401/403 from RotateCookies = session dead -> tell user to re-auth. Called when `__Secure-1PSIDTS` is missing at init, on redirect to accounts.google.com at init, and on RPC 401/403 (`callRpc` retry <2: reload from disk, rotate, re-init) (client.ts ~255-275, 340-380).
- `validateSavedSession()` after login does a real LIST_NOTEBOOKS and reports `ok | rejected (DBSC) | no_session | error` (auth.ts:192-220), added in commit a7a4e28.
- `refresh_auth` MCP tool reloads cookies from disk (server.ts:747, 1201).
- Single account only; no rotation.

## MCP tools (47)

Declared in src/server.ts:61-747. Destructive tools require `confirm: true` (docs/ARCHITECTURE.md). Grouped:
| Tool(s) | Params | Behavior |
|---|---|---|
| notebook_list, notebook_create(title), notebook_delete(id, confirm), notebook_rename(id,title), notebook_get(id), notebook_summarize(id), prompts_suggest(id, source_ids?, mode?, query?) | | notebook CRUD and guide |
| notebook_share(id, public/ collaborator email+role+notify+message), notebook_share_status(id) | | public link, collaborators |
| notebook_add_url, notebook_add_text, notebook_add_drive, notebook_add_local_file(path: .pdf/.txt/.md only, text-extracted then ingested as pasted text) | | sources; no binary upload (client.ts:1800-1822) |
| source_delete, source_sync, source_get_guide, source_rename, source_check_freshness | source_id | source ops |
| notebook_query(notebook_id, query, source_ids?, conversation_id?), chat_history_get(id, limit), chat_history_delete(id, confirm), chat_configure(id, goal, custom_prompt<=10000, response_length) | | chat |
| research_start(notebook_id, query, source web/drive, mode fast/deep), research_poll, research_import | | research |
| audio_overview_create, video_overview_create, report_create, flashcards_create, quiz_create, infographic_create, slide_deck_create, data_table_create | language, focus, enums (audio format deep_dive/brief/critique/debate = 1-4, length short/default/long, video style 1-9, infographic orientation 1-3, quiz quantity/difficulty) | Studio create; return artifact id, no wait loop |
| studio_poll, studio_delete, artifact_content_get, artifact_rename, artifact_export(docs/sheets) | | Studio management |
| note_create/list/update/delete, mind_map_generate/save/list/delete | | notes and mind maps (delete = 3-step: list for timestamp, DELETE_NOTE, sync) |
| refresh_auth | | reload cookies |

## Versions

- Git tags: v3.0.6, v3.0.7 (2026-02-15). npm is at 4.0.2.
- 2026-02-01..05: initial MCP + auth; 2026-02-08: rapid 1.0.5 -> 3.0.0 bump in one day, auto-update feature, cookie domain filtering, getNotebook RPC fix, esbuild bundle (Node 24 CJS/ESM interop); 2026-02-15 multi-language docs 3.0.6/3.0.7.
- 2026-08-20: 3.0.8 `__Secure-1PSIDTS` extraction/recovery (issue #1, eb2d609); a713940 18 new tools (47 total) + nested template block; 4.0.0 (622a6e0) "Gemini Notebook feature wave"; 4.0.1 session verification after login + DBSC docs; 2026-08-22 4.0.2 drops `uuid` for `crypto.randomUUID`.

## Reliability / performance

- Reuse: one long-lived `NotebookLMClient` with cached csrf/sid (`initialized` flag), re-init only on auth failure. Good, unlike roomi's per-call bootstrap.
- Timeouts: default 30 s per RPC, 120 s for add-source/import/query. No generic retry for 5xx or refusals. No backoff.
- `checkForUpdates()` on startup (src/index.ts:18-36, src/update.ts): fetches npm registry latest and, if newer, runs `npm install -g notebooklm-mcp-server@latest` then relaunches itself (Windows: detached .cmd script; Unix: execSync). Supply-chain and surprise-behavior risk; also messages are in Spanish.
- No tests in repo (examples/ scripts only; `test:*` scripts run live examples).

## Adopt

1. RotateCookies refresh flow (client.ts ~150-235): body, headers, cookie subset (drop OSID), `Set-Cookie` merge + persist; precondition check; treat 401/403 as dead session. This is a runtime cookie keepalive without a browser, directly relevant to our RPC-first design.
2. Cookie extraction rules: per-host cookie URL filter, dedupe by name, add `__Secure-1PSIDTS/3PSIDTS/LSID` from accounts.google.com (auth.ts:116-138).
3. Post-login API validation and DBSC detection (auth.ts:192-220): verify cookies with a real RPC before claiming success.
4. `templateBlock()` nested request-options and treat gRPC 3/5/9 as cohort mismatch signal (constants.ts:63-79).
5. Single cached client with `initialized` flag; re-init on auth failure only.
6. Auth-expired signature `[...,[16],"generic"]` check (client.ts extractRpcResult).
7. Broad RPC coverage useful for us (ids and shapes above): chat history (hPTbtc, khqZz, J7Gthc), share with users (QDyure), export (Krh3pd), notebook summary (VfAZjd), suggest prompts (otmP3b), source guide (tr032e), chat configure via s0tc2d slot 7, source rename/freshness/sync.
8. Destructive tools gated by `confirm:true`.
9. `NOTEBOOKLM_COOKIES` env override for headless/CI.

## Avoid

1. Self-update that runs `npm install -g ...@latest` and relaunches (src/update.ts) and `postinstall` that downloads Chromium (package.json).
2. Plaintext cookie file with default perms (`auth.json`, auth.ts:166-176); no encryption, no 0600 chmod.
3. Silent null for non-16 refusals and missing envelopes (client.ts extractRpcResult): errors become "empty result"; needs status decoding (see roomi batchexecute.ts:73-127).
4. Hard-coded `bl` and host (constants.ts:53,61); no host probing for `notebook.google.com`.
5. Local file support is text-only via pdf-parse then pasted text (client.ts:1800-1822); loses structure, no real upload RPC.
6. Inconsistent payloads: Drive ADD_SOURCE still uses the old flat tail (client.ts ~690); report/slide/data-table option blobs placed at content index 6 whereas roomi puts them at 7/16/18 and builds different shapes (compare client.ts:1150-1180 with roomi studio-rpc.ts:23-32). One of the two is wrong or cohort-specific [unverified which]; do not copy Studio payloads from either without live capture.
7. Query parse ignores texts of <= 20 chars (extractFromChunk), would drop short valid answers.
8. No citations returned from `notebook_query`.

## Evidence

Read in full: src/constants.ts, src/auth.ts, src/update.ts; src/client.ts lines 1-1200 and 1262-2017 (Studio blobs for audio/video/report/infographic/slides/data-table at 1090-1375 read; quiz family 1194-1260 only the header comment); src/server.ts tool list (names via grep, schemas at 290-365, 740-775); docs/ARCHITECTURE.md; README.fr.md:52; `git log`, `git show a713940` (message, constants diff); WebFetch https://notebook.google/ (title only). Line numbers in client.ts marked `~` are approximate (taken from grep of method starts).
