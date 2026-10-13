# roomi-fields/notebooklm-mcp

- URL: https://github.com/roomi-fields/notebooklm-mcp (npm `@roomi-fields/notebooklm-mcp`, MCP registry `io.github.roomi-fields/notebooklm-mcp`: package.json:2-4)
- Language / runtime: TypeScript, Node (ESM), `patchright` (Playwright fork) for browser, Express for REST, `@modelcontextprotocol/sdk` for MCP (package.json, src/http-wrapper.ts:1-20, src/stdio-http-proxy.ts:22-32)
- License: MIT (LICENSE; package says MIT). Short wire-format snippets allowed.
- Stars / last commit: 193 stars (research/upstreams.md); HEAD a05fe70 2026-09-04 "ci: publish to npm by OIDC trusted publishing"
- Versions reviewed: HEAD = v3.2.0 (package.json:3), 61 git tags v1.0.3..v3.2.0, 264 commits
- REA: `analyze-javascript-application ... --artifact-format directory` ran fine (132 source files, graph with modules/assets). The md output renders node identity/observations as `[object Object]`, so it gave only an inventory (file list incl. tests, website/, mcpb/ manifests); all facts below come from direct reading.
- Source size: src/ ~40k lines incl. tests; `src/tools/index.ts` 5394 lines, `src/content/content-manager.ts` 4499 lines (DOM automation), RPC layer is only 1629 lines (`src/rpc/*.ts`).

## Access method

Dual transport, explicitly documented as permanent (CHANGELOG.md:379-405, "Both paths ship permanently"):

1. RPC (default): internal `batchexecute` endpoint + one streaming endpoint, plain `fetch`, cookies taken from the Playwright persistent context (src/rpc/batchexecute.ts, src/tools/index.ts:4275-4310).
2. Browser/DOM (patchright, persistent Chrome profile): used for login, auto-reauth, and as the fallback for every RPC-backed tool except the RPC-only ones.

Transport switch: env `NOTEBOOKLM_TRANSPORT` (default `rpc`; value `dom` disables RPC globally) at src/tools/index.ts:4266-4268. There is no `rpc`-only mode (cannot disable the fallback) [verified by reading the function; only `dom` is special-cased].

### Exactly how the fallback triggers

Pattern in every RPC-backed handler (src/tools/index.ts): `if (this.useRpcTransport() && <preconditions>) { try { ...rpc... return } catch (e) { log.warning("RPC X failed (...); falling back to browser...") } }` then the DOM code runs. So the trigger is ANY thrown exception in the try block, plus "RPC returned no usable id" in add_source. No error class is inspected. Concretely, all of these fall to the browser:

- bootstrap failure on both hosts (not authenticated / redirect to accounts.google.com / no `SNlM0e` in homepage HTML): src/rpc/batchexecute.ts:211-228 and src/tools/index.ts:4284-4308 (re-throws last error after trying both hosts).
- HTTP 401/403/!ok: batchexecute.ts:377-382 (`RpcError`).
- `RpcDriftError` (response has no `wrb.fr` envelope for the id; "rotated id" tell): batchexecute.ts:56-66, 398.
- `RpcStatusError` = gRPC status in envelope index 5 (PERMISSION_DENIED, RESOURCE_EXHAUSTED, ...): batchexecute.ts:112-127, 175, 388-390. These are not retried (batchexecute.ts:251) but ARE still swallowed by the handler's catch and re-run in the browser. Consequence: a quota refusal (status 8) or a shared-notebook refusal (status 7) triggers a pointless slow browser attempt [inferred from the catch blocks; not tested].
- `QueryRejectedError` from the streaming endpoint (query-rpc.ts:31-36, 128): same, falls back.
- parse/shape errors thrown by RPC wrapper classes (e.g. "CREATE_NOTEBOOK returned no notebook id", notebooks-rpc.ts:62).

Per-handler map (src/tools/index.ts line of the `if (this.useRpcTransport()...)`):

| Tool | RPC attempt | Falls back to browser? |
|---|---|---|
| ask_question | 2019-2059; only when no `session_id` and notebook id known; stateless single turn | yes (2057). Any follow-up (`session_id` present) stays in the browser session |
| list_notebooks_from_nblm | 4605-4622 | yes (DOM scrape, 4620) |
| create_notebook | 5115-5132 | yes |
| delete_notebooks_from_nblm | 4848-4879 | yes (DOM delete with verify-gone) |
| add_source (url/youtube/text) | 3221-3240 | yes. file/Drive sources are browser-only (3215-3218 comment) |
| delete_source | 3336-3344 | yes |
| generate_content (audio_overview, video, infographic, presentation, report, data_table) | 3621-3670 | yes (3669) |
| list_content | 3751-3792 | yes |
| download_content (audio, video, infographic) | 3866-3889 | yes; also falls back if bytes start with `<!doctype html` (3881) |
| list_sources / read_source | RPC only, comment "There is no DOM fallback on purpose" (3393) | no |
| delete_content | RPC only (3538) | no |
| share_notebook, generate_study_aid, generate_mind_map, manage_labels, research_sources | RPC only | no |
| notes (create/list/get/save_chat/convert_to_source), library_*, sessions, auth | DOM / local | n/a |

### How state is shared between transports

- The only shared state is the cookie jar. `getRpcClient()` (src/tools/index.ts:4275-4310) calls `sharedContextManager.getOrCreateContext()` (this LAUNCHES or reuses the patchright persistent Chrome context, src/session/shared-context-manager.ts:67-90, profile dir `chrome_profile` under env-paths data dir, config.ts:149-152) and reads `context.cookies('https://<host>')`. So even "RPC" calls need a live browser context to exist; the browser is not avoided at startup, only for per-call work. (This is the main thing we should not copy; we should read cookies from a file/store.)
- A NEW `BatchExecuteClient` is built per tool call and `bootstrap()` fetches the homepage HTML every time (`getNotebookRpc/getSourcesRpc/getQueryRpc` all call `getRpcClient`, tools/index.ts:4312-4325). The CSRF/`f.sid`/`bl` are therefore never cached across tool calls; one extra GET of `/` precedes every tool call [verified from code structure; latency impact not measured].
- Per-client cache only: `reqidCounter`, token triple, `SourcesRpc.urlVersion` v1/v2 (sources-rpc.ts:56-82, also lost because SourcesRpc is rebuilt per call).
- Conversation state is NOT shared: RPC `ask` uses a fresh `randomUUID()` conversation id each call (query-rpc.ts:55) and "history is not yet re-sent" (query-rpc.ts:46-48). Multi-turn sessions exist only in the browser path (SessionManager, session/session-manager.ts). Returned `session_id` from RPC is just the server conv id or the notebook id (tools/index.ts:2040), so passing it back lands in the browser path.
- Cookies are not written back after RPC; refresh is by the browser's own profile / `AutoLoginManager` (accounts/auto-login-manager.ts, with TOTP at 238-243).
- Host selection (rebrand): tries `notebook.google.com` then `notebooklm.google.com`, bootstrapping each with the cookie jar collected for that same host (tools/index.ts:4284-4308). Fix for "RPC transport silently dead since the rebrand" was commit 7b5892f, shipped in v3.0.4 (CHANGELOG.md:269-293). Lesson: the fallback masked a dead primary for 3 releases; instrument which transport served each call.

### Health checks

- `get_health` / `GET /health` (tools/index.ts:2430-2530; http-wrapper.ts:93-104) does NOT probe Google or RPC. It reads the current account's `state.json`, checks cookies `SID,HSID,SSID,APISID,SAPISID` exist and none has `expires` in the past, plus session stats. Returns status/authenticated/active_sessions/max_sessions/headless/auto_login_enabled/stealth_enabled/current_account. It cannot detect an RPC drift or a dead transport.
- RPC drift detection is reactive (`RpcDriftError`). The migration plan (docs/rpc-migration-plan.md, Risks) lists "startup health-check that pings LIST_NOTEBOOKS" as planned, not implemented in code [grep found none].
- Port conflict probe in `http-wrapper.ts` (~1330-1370): checks whether something answers `GET /health` on the target port before listen.
- `scripts/doctor.mjs` (`npm run doctor:basic|http`) exists [not read in detail].

## Local HTTP REST API (src/http-wrapper.ts, Express)

- Bind: `HTTP_PORT` default 3000, `HTTP_HOST` default `0.0.0.0` (http-wrapper.ts:1321-1322, log text "accessible from network" at ~1535). No auth middleware: CORS `Access-Control-Allow-Origin: *` (lines 53-62), JSON body limit 10 MB (41). Avoid: default exposes an unauthenticated API controlling a logged-in Google session to the LAN with wildcard CORS.
- Request-ID middleware: honors `X-Request-ID` else UUID (44-51).
- Routes (all from `app.<verb>(` grep, http-wrapper.ts):
  - Meta/health: `GET /` (70), `GET /health` (93)
  - Ask: `POST /ask` (106, body: question, session_id, notebook_id, notebook_url, show_browser, source_format), `POST /batch-to-vault` (144)
  - Auth: `POST /setup-auth` (204), `POST /de-auth` (225), `POST /re-auth` (238), `POST /cleanup-data` (259)
  - Library (local JSON catalogue of notebooks): `GET/POST /notebooks` (273/286), `GET /notebooks/search` (317), `GET /notebooks/stats` (337), `GET /notebooks/scrape` (350), `POST /notebooks/import-from-scrape` (371), `DELETE /notebooks/bulk-delete` (472), `GET/PUT/DELETE /notebooks/:id` (501/514/530), `POST /notebooks/auto-discover` (543), `PUT /notebooks/:id/activate` (645)
  - Remote notebook ops: `POST /notebooks/create` (624), `GET/PUT /notebooks/:id/share` (658/669), `GET/POST /notebooks/:id/labels` (739/748), `GET /notebooks/:id/sources` (786), `GET /notebooks/:id/sources/:sourceId` (799)
  - Content: `POST /content/study-aid` (685), `POST /content/research` (705), `POST /content/mind-map` (719), `DELETE /content/:contentId` (768), `POST /content/sources` (863), `DELETE /content/sources/:id` (896), `DELETE /content/sources` (932), `POST /content/generate` (968), `GET /content` (1112), `GET /content/download` (1131), `POST /content/notes` (1185), `POST /content/notes/list` (1220), `POST /content/notes/get` (1237), `POST /content/chat-to-note` (1262), `POST /content/notes/:noteTitle/to-source` (1282)
  - Sessions: `GET /sessions` (820), `DELETE /sessions/:id` (833), `POST /sessions/:id/reset` (846)
  - Error handler 1310. (README says 33 endpoints.)
- Each route is a thin wrapper over the same `ToolHandlers` method used by the MCP tools (e.g. 106-125), so MCP and REST share logic and transport choice.
- `stdio-http-proxy.ts` (803 lines): stdio MCP server that forwards tool calls to the REST server via `MCP_HTTP_URL` (default `http://localhost:3000`); purpose: avoid two processes fighting over one Chrome profile (header comment 1-20). Tool names accepted in both v2 canonical and legacy forms (tools/tool-names.ts:16-76).
- OpenAPI spec: docs/ (README links); commit 77656d1 notes it was 12 endpoints behind, so treat as stale-prone.

## RPC catalog

Endpoint (batchexecute): `POST https://{host}/_/LabsTailwindUi/data/batchexecute?rpcids={id}&source-path={path}&bl={bl}&hl={hl}&rt=c[&f.sid={sid}]` (src/rpc/batchexecute.ts:346-354). `{host}` is `notebooklm.google.com` by default; `notebook.google.com` tried first by the MCP layer since 3.0.4. Note the service path `/_/LabsTailwindUi/` is unchanged after the rebrand (the rebrand only changed host/UI).
Not sent: `_reqid` on batchexecute calls (only on the streamed query), `authuser`. Default `source-path` is `/`; per-notebook calls send `/notebook/{id}`.

Headers (batchexecute.ts:362-373): `Content-Type: application/x-www-form-urlencoded;charset=UTF-8`, `Origin`, `Referer: {base}/`, `X-Same-Domain: 1`, `X-Goog-Csrf-Token: <at>`, a macOS Chrome 125 `User-Agent` (line 27-28, fixed), `Cookie: name=value; ...` (all cookies in the jar, no filtering, line 198).

Body: `f.req=<urlenc(JSON [[[id, JSON.stringify(params), null, "generic"]]])>&at=<urlenc(csrf)>&` (356-360). Single RPC per request (no batching of several rpcids).

Bootstrap (batchexecute.ts:202-233): `GET {base}/?hl={hl}` with cookies, `redirect: 'follow'`; if final URL matches `accounts.google.com` -> "Not authenticated". Regex extraction from HTML: CSRF `"SNlM0e":"..."` (fallback to `"FdrFJe"` value), session `f.sid` = `"FdrFJe":"..."`, build label `bl` = `"cfb2h":"..."` (fallback constant `boq_labs-tailwind-frontend_20260108.06_p0`, rpc-ids.ts:86; env `NOTEBOOKLM_BL`). Missing CSRF throws.

Response parsing (batchexecute.ts:152-182): strip `)]}'`, split by `\n`, skip lines not starting with `[` (this skips the length-prefix lines, i.e. it does not honor chunk byte counts), JSON.parse each line, find envelope `["wrb.fr", id, "<inner json string>", ..., [grpcStatus]]` where status is `env[5][0]`; `["er", ...]` envelope -> server error; null inner with no status = legit null result. gRPC code table 1..16 at 73-90.

Retry (batchexecute.ts:241-258): one retry after forced re-bootstrap for any error except `RpcDriftError` and non-16 `RpcStatusError`. No backoff, no timeout on batchexecute `fetch` (only the streamed query has `AbortSignal.timeout(120000)` and binary download 180000). Drift hot-patch: env `NOTEBOOKLM_RPC_OVERRIDES='{"LIST_NOTEBOOKS":"<id>"}'` (rpc-ids.ts:92-105).

| RPC id | Name in code | Purpose | Request params (JSON inside f.req) | Response shape | file:line |
|---|---|---|---|---|---|
| wXbhsf | LIST_NOTEBOOKS | list notebooks | `[null,1,null,[2]]` | `result[0]` = rows, row[0]=title, row[2]=uuid | rpc-ids.ts:20, notebooks-rpc.ts:30-44 |
| rLM1Ne | GET_NOTEBOOK | notebook with sources | `[nbId,null,[2],null,0]`, path `/notebook/{id}` | `result[0][1]` = sources; each `[[srcId], title, ...]` | notebooks-rpc.ts:72-89 |
| CCqFvf | CREATE_NOTEBOOK | create (title set at creation) | `[title,null,null,[2],[1,null,null,null,null,null,null,null,null,null,[1]]]` | `result[2]` = new uuid | notebooks-rpc.ts:52-64 |
| s0tc2d | RENAME_NOTEBOOK | rename | `[id,[[null,null,null,[null,newTitle]]]]` | n/a | notebooks-rpc.ts:104-108 |
| WWINqb | DELETE_NOTEBOOK | delete | `[[id],[2]]` | n/a | notebooks-rpc.ts:99 |
| izAoDd | ADD_SOURCE | add URL (v1) / YouTube / pasted text | `[[sourceData], nbId, [2], [1,null*9,[1]]]`; URL sourceData `[null,null,[url],null*6,1]`; YouTube `[null*7,[url],null,null,1]`; text `[null,[title,text],null,2,null*6,1]` | `result[0][0]` -> `[0][0]`=srcId, `[1]`=title | sources-rpc.ts:53,62-104,156-162 |
| ozz5Z | ADD_SOURCE_URL_V2 | add URL (new rollout) | `[[[[null,url,627],[null*9,[null,null,1]],1]]]` (no nbId in params; path carries it) | same parse | sources-rpc.ts:164-171 |
| hizoJc | GET_SOURCE | source metadata / full indexed text | `[[srcId],[sel],[sel]]`, sel `2` = text, `3` = html | `result[0][1]` title; text tree at `result[3][0]` (collect all strings); html at `result[4][1]` | sources-rpc.ts:122-149 |
| tGMBJ | DELETE_SOURCE | delete source | `[[[srcId]],[2]]` | n/a | sources-rpc.ts:153 |
| R7cb6c | CREATE_STUDIO | create artifact (audio, report, video, infographic, slides, data table, flashcards, quiz) | `[[2], nbId, content]`; content is sparse array: `[2]=typeCode`, `[3]=[[[srcId]],...]`, `[optionIndex]=options` | `result[0][0]`=artifactId, status at `[4]` | studio-rpc.ts:23-32,55-62,113-136 |
| gArtLc | POLL_STUDIO | list artifacts + status + media | `[[2], nbId, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | `result[0]` = list; a[0]=id,a[1]=title,a[2]=typeCode,a[4]=status(1 in progress,3 done,4 failed; audio 2+media = done), audio url `a[6][5]` (prefer `audio/mp4` ending `-dv`), video `a[8][3]`, infographic `a[14][2][0][1][0]`, slides `a[16][0|3]`, report text `a[7][1][0]` | studio-rpc.ts:159-187,225-279 |
| V5N4be | DELETE_STUDIO | delete artifact | `[[2], artifactId]` (single id only) | null = success | studio-rpc.ts:154-156 |
| yyryJe | GENERATE_MIND_MAP | generate mind map JSON | `[[[[src]]...],null*4,["interactive_mindmap",[["[CONTEXT]",instructions]],lang],null,[2,null,[1]]]` | `result[0][0]` = JSON string | extensions-rpc.ts:30-45 |
| CYK0Xb | SAVE_MIND_MAP (= CREATE_NOTE id) | persist mind map | `[nbId, json, [2,null,null,5,[[src]..]], null, title]` | `result[0][0]` = id | extensions-rpc.ts:47-58 |
| I3xc3c | LABEL_LIST | list labels | `[[2], nbId]` | `result[0]` = `[[name,[[srcId]..],labelId,emoji]..]` | extensions-rpc.ts:85-92,224 |
| agX4Bc | LABEL_CREATE | create label | `[[2], nbId, null,null,null,[[name,emoji]]]` | labels at `result[1]` | extensions-rpc.ts:96-101 |
| le8sX | LABEL_MUTATE | rename label | `[[2], nbId, labelId, [[[newName]]]]` | n/a | extensions-rpc.ts:106-110 |
| GyzE7e | LABEL_DELETE | delete labels | `[[2], nbId, [labelIds]]` | n/a | extensions-rpc.ts:115 |
| Ljjv0c | START_FAST_RESEARCH | fast web research | `[[query, sourceType(1 web/2 drive)], null, 1, nbId]` | `result[0]` = task id | extensions-rpc.ts:129-136 |
| e3bVqc | POLL_RESEARCH | poll research | `[null,null,nbId]` | tasks; task[0]=id, task[1][3][0] = sources `[url,title,desc]` | extensions-rpc.ts:139-166 |
| LBwxtb | IMPORT_RESEARCH | import found sources | `[null,[1],taskId,nbId,[[null,null,[url,title],null*7,2]..]]` | n/a | extensions-rpc.ts:169-188 |
| QDyure | SHARE_NOTEBOOK | public link on/off | `[[[nbId,null,[0|1],[0,""]]],1,null,[2]]` | n/a | sharing-rpc.ts:55 |
| JFMDGd | GET_SHARE_STATUS | share status | `[nbId,[2]]` | `result[0]`=collabs `[email,role,[],[name,avatar]]`, `result[1]=[true]` when public | sharing-rpc.ts:33-49 |

Declared in rpc-ids.ts but UNUSED anywhere else in src (grep, 20 ids): `o4cbdc` ADD_SOURCE_FILE, `yR9Yof` CHECK_SOURCE_FRESHNESS, `FLmJqe` SYNC_DRIVE_SOURCE, `b7Wfje` RENAME_SOURCE, `hPTbtc` GET_CONVERSATIONS, `khqZz` GET_CONVERSATION_TURNS, `J7Gthc` DELETE_CHAT_HISTORY, `VfAZjd` GET_SUMMARY, `tr032e` GET_SOURCE_GUIDE, `QA9ei` START_DEEP_RESEARCH, `rc3d8d` RENAME_ARTIFACT, `v9rmvd` GET_INTERACTIVE_HTML, `KmcKPe` REVISE_SLIDE_DECK, `cFji9` LIST_MIND_MAPS/GET_NOTES, `AH0mwd` DELETE_MIND_MAP/DELETE_NOTE, `cYAfTb` UPDATE_NOTE, `Krh3pd` EXPORT_ARTIFACT (rpc-ids.ts:27-80). Treat the ids as hints only; shapes for them are not in this repo (docs/rpc-migration-plan.md:42-52 repeats the ids; says id list was cross-checked with jacob-bd's project). Notes (`CYK0Xb`/`cFji9`/`cYAfTb`/`AH0mwd`) are RPC-capable in principle but the note_* tools here stay DOM.

Studio option blobs (studio-rpc.ts:64-110): type codes audio=1, report=2, video=3, quiz/flashcards=4 (subtype 1 flashcards, 2 quiz), infographic=7, slide deck=8, data table=9. Option index in content array: 6,7,8,9,14,16,18. Language and focus go inside the blob; BUT the language that NotebookLM actually generates in is the `hl` query param, payload slot ignored (comment tools/index.ts:4289-4296, issue #34). That is why generation calls build the client with `hl = content language`.

Binary download: `fetchBinary` follows redirects manually (max 6) re-sending Cookie, because Node fetch drops Cookie on cross-origin redirect google.com -> googleusercontent.com (batchexecute.ts:260-291).

## Streaming/chat

- Endpoint: `POST https://{host}/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=&hl=&_reqid=&rt=c[&f.sid=]` (batchexecute.ts:25-26, 299-341). `_reqid` = counter starting at `100000 + (now % 800000)`, += 100000 per post (192, 302). No `rpcids`, no `source-path`.
- Body: `f.req=<urlenc(JSON [null, JSON.stringify(inner)])>&at=<csrf>&` with same headers as batchexecute (312-326).
- Inner params: `[ [[ [src1]],[[src2]]...], question, null, [2,null,[1]], conversationId ]`: sources as `[[id]]` each; fresh `randomUUID()` conversation id unless supplied (query-rpc.ts:55-57). All notebook sources are passed (ids from GET_NOTEBOOK).
- On 401/403 or redirect to accounts.google.com: forced re-bootstrap and one retry (batchexecute.ts:332-336). Whole response is read with `res.text()`, i.e. NOT consumed incrementally; no token streaming to the MCP client.
- Response format: `)]}'`, then alternating length line / JSON line (parseQueryResponse handles both with or without numeric line, query-rpc.ts:116-125). Each chunk: `[["wrb.fr", null, "<inner json>", ...]]`; inner `[0]` = `[answerText, ?, [convId,...], ?, typeInfo]` (query-rpc.ts:176-190). `typeInfo[last] === 1` marks a final answer chunk, otherwise it is a "thinking" chunk. Strategy: keep the LONGEST type-1 text (and its citation data / convId); fall back to longest thinking text; if only an error chunk exists throw `QueryRejectedError(code)` (`item[0]=='wrb.fr' && item[2]===null`, code at `item[5][0]`, query-rpc.ts:133-148).
- Citations: `typeInfo[3]` = passages; per passage n (1-based index i+1), `passage[1]` = detail, `detail[5][0][0][0]` = source id (a string), cited text assembled from `detail[4]` (nested segments, `segment[2][*][*][2]` strings; tables -> `<cited_table>` placeholder) (query-rpc.ts:199-259). Output: `{citation_number, source_id, cited_text}`; tool layer maps source ids to titles from GET_NOTEBOOK and prints markers `[n]`.
- Conversation handling: single turn only on RPC; no history replay (query-rpc.ts:46-48). Multi-turn via browser sessions (`session_id`).
- Rate limit: 50 queries/day assumed for free accounts (account-manager.ts:30). RateLimitError detection + account rotation lives only in the browser branch (tools/index.ts:2143-2165); RPC ask failures just fall back.

## Auth lifecycle

- Cookie source: Playwright/patchright persistent Chrome profile (`chrome_profile/`) plus `state.json` storage-state per account (`accounts/<id>/state.json` or `browser_state/state.json`, account-manager.ts:172,194,483,755-768); login via visible Chrome (`setup_auth`, up to 10 minutes wait, tools/index.ts ~2520-2530) or auto-login with email/password (+ TOTP secret) kept encrypted AES-256-GCM in the account store (accounts/crypto.ts:4,15; auto-login-manager.ts:213-243).
- Cookies sent to RPC: whole jar for the target host from `context.cookies('https://host')` (tools/index.ts:4284-4287). Not filtered to a required subset.
- Required cookies (health check only): any of SID, HSID, SSID, APISID, SAPISID present and not expired (tools/index.ts ~2457-2475).
- Expiry detection: (a) health cookie `expires`; (b) bootstrap redirect to accounts.google.com (batchexecute.ts:211); (c) 401/403 on call; refresh = forced re-bootstrap (new CSRF) once, NOT cookie refresh. Real re-login goes through the browser/AutoLoginManager.
- Multi-account: `AccountManager` pool with rotation strategies `least_used | round_robin | failover | random` (accounts/types.ts:81), per-account `quota.json` with default 50/day (account-manager.ts:30,151-153), `markRateLimited` (494), `current-account.txt` (516). Rotation is triggered on browser-path rate-limit errors (tools/index.ts:2143-2165). The RPC client takes whichever account's cookies the shared context currently holds.
- Account switching requires closing/recreating the browser context (single global context by design, shared-context-manager.ts:1-12).
- No `authuser`/`u/N` handling for multi-login Google sessions in RPC URL builder [absent in batchexecute.ts:346-354].

## MCP tools

Canonical v2 names are `namespace_action` (tools/tool-names.ts:16-61); legacy flat names still accepted. 42 definitions in `buildToolDefinitions` (tools/index.ts:451-1800), all with a shared output schema and annotations (1810+). Transport column: R = RPC with browser fallback, RO = RPC only, B = browser only, L = local.

| Tool (canonical / legacy) | Params (from schema) | Behavior | Transport |
|---|---|---|---|
| notebook_ask / ask_question (451) | question, session_id?, notebook_id?, notebook_url?, source_format (none/inline/footnotes/json/expanded), browser_options, show_browser | ask a notebook, citations optional | R (fresh turn) / B (session) |
| library_discover / auto_discover_notebook (571) | url | ask NotebookLM to describe a notebook, builds metadata | B/R via ask |
| library_add / add_notebook (621) | url, name, description, topics[], content_types[], use_cases[], tags[] | add to local library | L |
| library_list, library_get, library_select, library_update, library_remove, library_search, library_stats (710-867) | id/query | local catalogue ops | L |
| session_list, session_close, session_reset (875-900) | session_id | chat sessions | L/B |
| server_health / get_health (916) | none | cookie-state health (see above) | L |
| auth_setup / setup_auth (928) | show_browser, browser_options | interactive login | B |
| auth_logout / de_auth (980), auth_switch / re_auth (999) | confirm / account | logout, switch account | B |
| server_cleanup / cleanup_data (1056) | confirm, preserve_library | wipe local data | L |
| source_add / add_source (1101) | source_type (file/url/text/youtube/google_drive), file_path/url/text/title, notebook_url | add source | R for url/youtube/text, else B |
| source_delete / delete_source (1148) | source_id/name, notebook_url | delete source | R |
| source_list / list_sources (1179) | notebook_url/id | indexed sources | RO |
| source_read / read_source (1199) | notebook, source_id, format text/html, offset/limit pagination | full indexed text, paginated | RO |
| content_generate / generate_content (1285) | content_type, notebook_url, language, custom_instructions, plus per-type options | Studio generation | R (6 types) / B |
| content_list (1349), content_download (1370), content_delete (1258) | notebook, content_type, output_path, id | list/download/delete | R, R (3 media types), RO |
| note_create (1405), note_save_chat (1436), note_to_source (1465), note_list (1652), note_get (1670) | title/content/etc. | Studio notes | B |
| vault_batch / batch_to_vault (1499) | questions[], notebook, output dir | persist answers as md + JSON (nblm-answer-v1) | R/B |
| notebook_list / list_notebooks_from_nblm (1565) | show_browser | account notebooks | R |
| notebook_create (1589), notebook_delete (1621) | name / ids | create/delete remote | R |
| share_notebook (1694) | notebook, set_public? | public-link toggle + status | RO |
| generate_study_aid (1716) | kind flashcards/quiz, notebook | RPC CREATE_STUDIO type 4 | RO |
| generate_mind_map (1745) | notebook, title, language, instructions | GENERATE+SAVE mind map | RO |
| manage_labels (1772) | action list/create/rename/delete | labels | RO |
| research_sources (1800) | query, notebook_url/id, import? | fast web research only (deep id unused) | RO |

## Versions

61 tags. Notable (CHANGELOG.md line refs; dates from git tags):
- v1.0.3 2025-10-16 first tag; v1.1.0 2025-10-18; v1.1.2-http 2025-01-21 [sic, changelog date typo] added HTTP wrapper; v1.3.x 2025-11 sessions/accounts; v1.4.0 2025-12-24; v1.5.0 2025-12-31 content generation.
- v2.0.0 2026-05-14: tool rename to `namespace_action` (CHANGELOG.md:714).
- v2.3.0 2026-07-30: DOM repair after the "Gemini Notebook" rebrand (CHANGELOG.md:425-461).
- v3.0.0 2026-07-30: dual transport RPC+DOM, 5 RPC-only tools (CHANGELOG.md:379-424).
- v3.0.1 2026-08-06 interactive login CLI; v3.0.2/3 2026-08-19 `--help`, virtualized chat fix.
- v3.0.4 2026-08-19: RPC was dead since rebrand because of host/cookie mismatch (CHANGELOG.md:269).
- v3.1.0 2026-08-20 `source_list`/`source_read` (full indexed text); v3.1.1/3.1.2 2026-08-21 language handling (hl vs payload), RPC refusal reported as refusal, labels fix (list called create RPC).
- v3.2.0 2026-09-03 `content_delete`; HEAD a05fe70 2026-09-04 npm OIDC trusted publishing.
Last 40 commits visible in `git log` (HEAD..2026-08-06); fix cadence is high, many are DOM-locale fixes the RPC path would not need.

## Reliability / performance notes

- Claimed (CHANGELOG.md:392-405, not independently measured): list notebooks ~1 s (was ~30 s), report ~13 s, data table ~18 s.
- Per-tool-call homepage bootstrap (see "state"), no pooling or keep-alive tuning beyond Node fetch default; no concurrency control or per-account rate limiter on RPC.
- Poll loops: studio 5 s interval, 900 s default timeout (studio-rpc.ts:203-204); research 5 s / 120 s (extensions-rpc.ts:197-201).
- Tests: Jest, 35 unit test files incl. `rpc-batchexecute.test.ts`, `rpc-query.test.ts`, `rpc-refusal.test.ts`, `rpc-sources.test.ts`, `rpc-studio-delete.test.ts`; integration + e2e suites under tests/ (need a live account). Pure parsers are exported for unit testing (`parseBatchExecute`, `parseQueryResponse`).

## Adopt

1. Pure, side-effect-free response parsers with fixtures: `parseBatchExecute(text, id)` and `parseQueryResponse(text)` (batchexecute.ts:152-182, query-rpc.ts:84-130). In Go: one `Decode(body, rpcID)` that returns (payload | absent | refusal(status) | serverError) so drift vs refusal vs empty are distinct types.
2. Distinguish three failure kinds: no envelope for id = drift (hot-patch), envelope with `env[5][0]` status = refusal with human advice table (batchexecute.ts:73-127), `er` = server error. Commit 31886f1 shows the cost of conflating them.
3. `NOTEBOOKLM_RPC_OVERRIDES` and `NOTEBOOKLM_BL` env hot-patch with a dated fallback `bl` (rpc-ids.ts:85-110).
4. Host probing with matching cookie jar: try `notebook.google.com` then `notebooklm.google.com`, bootstrap each, keep first success (tools/index.ts:4284-4308); also record which host served the account. Commit 7b5892f is a proven failure mode.
5. Bootstrap regexes `"SNlM0e"`, `"FdrFJe"`, `"cfb2h"` from homepage HTML (batchexecute.ts:219-221) plus the redirect-to-accounts.google.com unauthenticated test (211).
6. Manual redirect following with Cookie preserved for googleusercontent media (batchexecute.ts:269-291); detect HTML consent page by `<!doctype html` sniff (tools/index.ts:3881).
7. Longest-answer-wins selection among streamed chunks, type flag at `typeInfo[last]==1`, citation extraction from `typeInfo[3]` (query-rpc.ts:99-129,199-259).
8. Cache the winning URL-source RPC version (v1 `izAoDd` vs v2 `ozz5Z`) but at process scope, not per call (sources-rpc.ts:56-82).
9. Studio payload builder: sparse content array indexed by type (studio-rpc.ts:23-62) and `hl`-drives-language insight (tools/index.ts:4289-4296).
10. Tool naming `namespace_action` with legacy alias map (tool-names.ts) and one handler layer feeding both MCP and REST (http-wrapper.ts:106-125). Also `source_read` paginated indexed text and `content_delete`.
11. stdio->HTTP proxy mode so several MCP clients share one authenticated engine (stdio-http-proxy.ts:1-20).

## Avoid

1. Fallback on ANY exception: hides a dead primary (3.0.0-3.0.3 shipped with RPC fully dead, CHANGELOG.md:269-293) and turns refusals (quota status 8, PERMISSION_DENIED 7) into slow, pointless browser retries (tools/index.ts:2057 etc.). Fall back only for transport/drift classes, never for refusals, and surface which transport answered.
2. RPC cookies require launching the browser context (tools/index.ts:4278); use a cookie store file instead.
3. Fresh client and homepage GET per tool call (tools/index.ts:4312-4325); cache tokens with TTL and refresh on UNAUTHENTICATED (status 16).
4. Parsing by `.split('\n')` and skipping length lines (batchexecute.ts:156-158) ignores chunk sizes: a chunk containing newlines inside JSON would be mis-parsed. Implement the real length-prefixed framing.
5. `/health` that only inspects local cookie expiry (tools/index.ts:2457-2475): add a real RPC probe (e.g. LIST_NOTEBOOKS, documented but unbuilt in docs/rpc-migration-plan.md Risks).
6. REST API binding `0.0.0.0`, no auth, CORS `*` (http-wrapper.ts:53-62,1321-1322).
7. Stateless RPC ask: new conversation UUID each call, no history (query-rpc.ts:46-55), so multi-turn requires the browser. We should implement conversation continuation over the streaming endpoint [needs wire research; see hPTbtc/khqZz ids unused here].
8. Fixed fake UA string and no `Accept-Language`/`x-goog-ext` headers (batchexecute.ts:27-28) [unverified whether Google cares].
9. No timeout on batchexecute `fetch` and no backoff (batchexecute.ts:362).
10. 20 declared-but-unused RPC ids give false confidence; do not copy ids without payload evidence.
11. Single huge files (`tools/index.ts` 5394 lines, `content-manager.ts` 4499 lines) mixing transports; keep a transport interface.

## Evidence

Read in full: src/rpc/*.ts (all 8 files), src/stdio-http-proxy.ts header, src/tools/tool-names.ts. Read in part with line ranges: src/tools/index.ts (451-470, 1990-2110, 2425-2560, 3200-3250, 3615-3690, 3860-3900, 4255-4330, 4600-4630, tool name grep), src/http-wrapper.ts (20-142, route grep, 1310-1572 grep), src/session/shared-context-manager.ts (1-140), accounts/* (grep), CHANGELOG.md (3.0.x, 2.3.0, 3.0.4), docs/rpc-migration-plan.md:40-80, `git show 7b5892f`, `git tag --sort=creatordate`, `git log`.
