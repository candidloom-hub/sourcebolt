# ers123/notebooklm-mcp

- URL: https://github.com/ers123/notebooklm-mcp
- Language / runtime: TypeScript, Node, MCP SDK + zod, vitest; `patchright` is an optional dependency used only for login (package.json)
- License: MIT (LICENSE "Copyright (c) 2026 ers123"). Snippets <15 lines allowed.
- Stars / last commit: 2 stars (research/upstreams.md); HEAD f550c6f 2026-02-09 "Add CLI entry point for Claude Code skill integration"
- Versions reviewed: HEAD, package.json 2.0.0; 0 tags; 25 commits 2026-02-06..2026-02-09 (v2.0.0 commit 170c6cf "Switch from browser automation to direct HTTP API", 2026-02-07).
- REA ran (399 KB md, unreadable node output); direct reading. src is 4779 lines (api/ 4 files, ~900 lines: the interesting part).
- Origin: reverse-engineering references taken from jacob-bd's project (commit 1e173d4 "Add acknowledgment for API reverse-engineering reference"); the commit history is a debug log of Feb 2026 wire fixes, which is the useful part.

## Access method

Pure HTTP RPC (batchexecute + GenerateFreeFormStreamed) with fetch; no browser at runtime. Login via patchright (`setup_auth` tool), cookies stored encrypted. Includes an MCP server (35 tools) and a CLI (`src/cli.ts`: `node dist/cli.js <tool> '<json>'`) using the same handlers.

## RPC catalog

Transport (src/api/rpc-client.ts, src/api/auth-headers.ts, src/config.ts):
- URL: `POST https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute?rpcids={id}&source-path={path}&bl=boq_labs-tailwind-frontend_20260108.06_p0&hl=en&rt=c[&f.sid={FdrFJe}]` (rpc-client.ts:98-110, constant at :11). Commit history shows the failures that led here: 400 until query params + session id added (c350c18), 405 until the path was `/_/LabsTailwindUi/data/batchexecute` and `X-Same-Domain: 1` added (6e86b04), source-path required for all notebook-specific calls (60af55b), "Fix all RPC params and constants" (9d09c1c).
- Headers (auth-headers.ts:203-226): Content-Type form, Cookie, Origin, Referer, `X-Same-Domain: 1`, Chrome 131 macOS UA, and `Authorization: SAPISIDHASH <ts>_<sha1("<ts> <SAPISID> <origin>")>` computed from `SAPISID` (fallback `__Secure-3PAPISID`, `__Secure-1PAPISID`) on every request (auth-headers.ts:320-336). Commit e11bb21 "Fix CSRF auth: add SAPISIDHASH header and shared AuthHeaders". Whether Google requires this for batchexecute is unclear: agmmnn's client omits it and works [conflict; verify live]. Note the SAPISIDHASH origin is `https://notebooklm.google.com`.
- Body: `f.req=<urlenc([[[id, JSON(params), null, "generic"]]])>&at=<csrf>&` (rpc-client.ts:115-119).
- CSRF: `GET https://notebooklm.google.com` with cookies; session id regex `FdrFJe":"([^"]+)"`; CSRF patterns tried in order: `SNlM0e":"..."`, escaped `SNlM0e\x22:\x22...\x22`, and `"SNlM0e"\s*:\s*"..."` (auth-headers.ts:178-182,267-283). Cached 5 minutes (`csrfTokenExpiry`, :279), invalidated on 401/403. `bl` constant, not scraped.
- Retries: MAX_RETRIES 2 on auth errors (re-fetch CSRF) and generic errors; RPC timeout 60 s, query timeout 120 s (config.ts). 401/403 -> AuthError after retries; other non-2xx -> ValidationError (no 429 mapping).
- Parser (src/api/response-parser.ts): strip `)]}'`; `parseChunkedResponse` treats a positive integer line as a byte count and then ACCUMULATES following lines until `JSON.parse` succeeds (handles multi-line JSON), else parses each line directly. History: byte-count slicing with UTF-8 handling was tried (a3abf46) and reverted to line accumulation (4e20855, 83fc07f) because slicing was unreliable. Auth error signature: envelope with `env[6]=='generic'` and `env[5]` including 16 -> AuthError (:106-114). Missing id returns `null` silently (:129), no drift error.

| RPC id | Name in code | Params as built | file |
|---|---|---|---|
| wXbhsf | NOTEBOOK_LIST | `[null,1,null,[2]]` | tools/notebook/notebook-list.ts:17 |
| CCqFvf | NOTEBOOK_CREATE | `[title,null,null,[2],[1,null*9,[1]]]` (old flat tail) | notebook-create.ts:12 |
| rLM1Ne | NOTEBOOK_GET | `[nbId,null,[2],null,0]` with source-path; source ids at `data[0][1][i][0][0]` | notebook-get.ts:17 |
| s0tc2d | NOTEBOOK_UPDATE | rename; chat configure `[nbId,[[null*7,[goalCode,lengthCode,customPrompt]]]]` (flat triple, unlike agmmnn/mood nested `[[goal(,prompt)],[length]]`) | chat-configure.ts:22 |
| WWINqb | NOTEBOOK_DELETE | `[[nbId],[2]]` | notebook-delete.ts:18 |
| VfAZjd | NOTEBOOK_DESCRIBE | `[nbId,[2]]` | notebook-describe.ts:17 |
| izAoDd | SOURCE_ADD | URL `[sourceEntry, nbId, [2]]` where sourceEntry `[null,null,[url],null*6,1]` (no outer `[[ ]]` wrapper and no CREATE_TAIL, differs from all others) ; YouTube url at slot 7; text/drive in sibling files | source-add-url.ts:10-25 |
| hizoJc | SOURCE_GET_CONTENT | `[[srcId],[2],[2]]`; longest strings extracted | source-get-content.ts:11 |
| yR9Yof | SOURCE_LIST_DRIVE | freshness check per source | source-list-drive.ts:54 |
| FLmJqe | SOURCE_SYNC_DRIVE | | source-sync-drive.ts:11 |
| tGMBJ | SOURCE_DELETE | `[[[srcId]],[2]]` (no source-path) | source-delete.ts:16 |
| tr032e | SOURCE_DESCRIBE | | source-describe.ts:11 |
| Ljjv0c / QA9ei / e3bVqc / LBwxtb | research fast / deep / status / import | status `[null,null,nbId]`; import `[null,[1],taskId,nbId,[]]` (empty source array: import is incomplete) | tools/research/* |
| R7cb6c | STUDIO_CREATE | `[[2], nbId, content]`: audio `[null,null,1,src,null,null,[null,[focus,length,null,src(triple),lang,null,format]]]` (sources inside config are triple-nested, others use double); video type 3 options at idx 8 `[null,null,src,lang,focus,null,format,style]` (different order from agmmnn `[double, lang, instr, null, format, style]`); quiz type 4 `[null,null,4,src]` and data table type 9 `[null,null,9,src]` with NO options; slides type 8 options idx 14 (`[[focus,lang,format,length]]`, vs live-verified idx 16) | tools/studio/*.ts |
| gArtLc / V5N4be | STUDIO_STATUS / DELETE | delete `[[2], artifactId]` | studio-status.ts:94; studio-delete.ts:18 |
| yyryJe / CYK0Xb / cFji9 / AH0mwd | mind map generate / save / list / delete | generate (source-path `/`): `[src(triple),null*4,["interactive_mindmap",[["[CONTEXT]",""]],""],null,[2,null,[1]]]`; save `[nbId,json,[2,null,null,5,[[src]..]],null,title]`; list `[nbId,[2]]`; delete `[nbId,mindMapId,[2]]` | mind-map-*.ts |

Takeaway: this repo's studio payloads are partly wrong/incomplete (see Avoid); do not use as wire reference except the transport history. The 4-way variance among implementations for source add, source delete, artifact delete, chat config, and studio option positions is itself a finding: Google tolerates several shapes, and we must pin each with a golden fixture from a live capture.

## Streaming/chat

- `POST https://notebooklm.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=...&hl=en&_reqid=<n>&rt=c[&f.sid=...]` (src/api/query-client.ts:47-56, config.ts QUERY_STREAM_URL). `_reqid` starts at 100000 and adds 100000 per call. Commit cb0de03 "Fix notebook_query: correct endpoint URL, f.req format, and source-based query".
- Body `f.req=[null, JSON(params)]&at=<csrf>&`; params `[[[sid]]..., question, history|null, [2,null,[1]], null]` (conversation id always null, query-client.ts:143-159).
- Follow-ups: per-notebook `conversationCache` holds a "conversation context" taken from the response (the first response item with `length > 3 && Array.isArray(item[0])`, query-client.ts:295-298) and is passed back as the `history` param on `isFollowUp=true`. Unlike agmmnn (which rebuilds `[answer,null,2],[query,null,1]` pairs), this stores whatever the server echoed [unverified that this works; heuristic].
- Parse: collect chunks with type = last element of `first[4]` (1 answer, 2 thinking), choose the longest type-1 text, else longest overall (query-client.ts:211-221); sources = every `[string, string]` pair found at depth 2 (sub[1] string), which is a heuristic, not the structured citation path (query-client.ts:283-293). Text is passed through `sanitizeResponse` (strip zero-width chars, flag prompt-injection phrases) before return.
- Not incremental (`response.text()`).

## Auth lifecycle

- Login (`setup_auth`): patchright browser to `https://notebooklm.google.com`, polls `context.cookies()` every 2 s for at least 2 of SID, HSID, SSID, APISID, SAPISID, `__Secure-1PSID`, `__Secure-3PSID`, `__Secure-1PAPISID`, `__Secure-3PAPISID`, 5 min timeout, then waits 3 s more and captures all (src/auth/auth-manager.ts:22-80). Replaced URL-based login detection with cookie polling in commit 529e3f8; commits d73c95d/b4ecd09/1db7b8e fought `context.cookies()` URL args and domain filtering.
- Store: cookies filtered to domains `.google.com`, `google.com`, `notebooklm.google.com` (config.ts ALLOWED_COOKIE_DOMAINS; suffix match so any `*.google.com` host incl. `notebook.google.com` and `accounts.google.com` passes, cookie-store.ts isDomainAllowed), AES-256-GCM encrypted (`crypto.ts`, 16-byte IV), file `~/.notebooklm-mcp/cookies.enc` written via a secure-file helper (permissions), key in the macOS Keychain through `security` CLI (`keychain.ts` uses `execFile('security', ...)`: macOS only [other OSes would fail; no fallback seen]).
- Expiry/refresh: `StateValidator` treats persistent cookie expiry vs now, `needsRefresh` when less than 2 h remain (state-validator.ts:5-8,22-30); no real refresh (no RotateCookies, no signaler); on 401/403 only the CSRF is refetched. `check_auth` tool reports state.
- Domain allowlist: requests only to `notebooklm.google.com` (url-validator.ts; `ALLOWED_DOMAINS`), so the new `notebook.google.com` host would be rejected by its own validator.
- Single account.

## MCP tools (35, per commit 42a34c9; handlers in src/tools/*)

auth: setup_auth, check_auth, clear_auth. notebook: notebook_list, notebook_create, notebook_get, notebook_describe, notebook_rename, notebook_delete, chat_configure. source: source_add_url, source_add_text, source_add_drive, source_describe, source_get_content, source_list_drive, source_sync_drive, source_delete. query: notebook_query (notebook id, question, follow-up flag). research: research_start, research_status, research_import. studio: audio_create, video_create, report_create, flashcards_create, quiz_create, infographic_create, slide_deck_create, data_table_create, studio_status, studio_delete, mind_map_create, mind_map_list, mind_map_delete. Destructive tools take `confirm: true` (e.g. source-delete.ts:10-14). Schemas validated with zod (src/tools/schemas.ts); errors mapped by class to text with `isError: true` (tools/index.ts). Fetch-based `max_sessions 5`, `SESSION_IDLE_TIMEOUT 15 min` constants exist from the earlier browser design (config.ts), vestigial.

## Versions

No tags. 2026-02-06 initial secure browser-based server; 2026-02-07 v2.0.0 HTTP switch + auth fixes; 2026-02-08 wire fixes; 2026-02-09 mind maps (35 tools) + CLI.

## Reliability / performance

Cached CSRF 5 min; 2 retries; timeouts 60 s/120 s; heavy `logger.info` of every response preview (rpc-client.ts:69-70 logs the first 200 bytes of every response: leaks notebook titles/content into logs). Tests: vitest in tests/ [not read].

## Adopt

1. Encrypted cookie store pattern: AES-256-GCM + key in OS keychain + 0600 file + domain re-validation on load (cookie-store.ts). In Go: use `zalando/go-keyring` or equivalent with an encrypted-file fallback (this repo has no fallback).
2. Response sanitizer for MCP output: strip zero-width characters and flag prompt-injection phrases in model answers before returning to the agent (security/response-sanitizer.ts:1-60). Cheap defense since NotebookLM answers contain user-source text.
3. Multi-line-safe chunk parsing by accumulating lines until valid JSON (response-parser.ts:26-81) as a robust fallback to byte-count framing, plus the recorded lesson that byte-count slicing broke.
4. Multiple CSRF regex patterns including the escaped variant (auth-headers.ts:178-182).
5. Cookie polling login completion instead of URL matching (auth-manager.ts:45-70).
6. CLI that shares handlers with the MCP server (cli.ts) for skills/scripts.
7. `confirm: true` gate and zod-validated inputs on destructive tools.
8. URL/domain allowlist for outbound requests (but include `notebook.google.com`).

## Avoid

1. SAPISIDHASH on every call without evidence it is needed; recomputed with the stale origin.
2. macOS-only keychain via `security` CLI with no fallback (keychain.ts).
3. Logging response previews (rpc-client.ts:69-70).
4. Wrong or incomplete payloads: slides config at index 14, quiz/data-table without options, import with empty source array (research-import.ts:12), source add missing the `[[ ]]` wrapper and options tail (source-add-url.ts), audio config sources triple-nested.
5. No drift detection, no gRPC status decoding (missing id -> null), no 429 mapping.
6. Heuristic source extraction in query (`[string,string]` pairs) and unverified conversation-context cache.
7. Hard-coded host and domain allowlist excluding the new host.
8. Vestigial browser-era constants (BLOCKED_BROWSER_ARGS blocks `--no-sandbox` which breaks Docker).

## Evidence

Read in full: src/api/rpc-client.ts, auth-headers.ts, response-parser.ts, constants.ts, query-client.ts, config.ts (first 50), auth/cookie-store.ts, auth/auth-manager.ts, auth/state-validator.ts, tools/index.ts, security/response-sanitizer.ts. Read in part: studio-* and source-* tool files (param blocks), chat-configure.ts, mind-map-create.ts, security/keychain.ts/crypto.ts (grep), cli.ts header. `git log` (25 commits). 
