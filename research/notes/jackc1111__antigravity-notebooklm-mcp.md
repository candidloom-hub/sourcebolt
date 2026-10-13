# jackc1111/antigravity-notebooklm-mcp

NO LICENSE FILE (package.json declares `"license": "ISC"` at package.json:12 but the repo has no LICENSE file; GitHub shows none; treat as all rights reserved). This note describes behavior only. No code is reproduced.

- URL: https://github.com/jackc1111/antigravity-notebooklm-mcp (npm `antigravity-notebooklm-mcp` 1.0.0)
- Language / runtime: TypeScript, Node, axios, MCP SDK
- Stars / last commit: 39 stars (research/upstreams.md); HEAD 2c122a7 2026-01-27 "docs: Update README for Antigravity release"
- Versions reviewed: HEAD; 0 tags; 6 commits, all on 2026-01-27 (a515352 initial .. 2c122a7), package version 1.0.0.
- REA ran (101 KB md, unreadable node output); direct reading. src is 1705 lines (api-client.ts 536, index.ts 373, browser-auth.ts 358).
- Overall: an early, thin, partly wrong clone of the jacob-bd protocol (written in a day); low reuse value. Mostly useful as a data point for what NOT to trust and for tool-design ideas (consolidated tools).

## Access method

Internal batchexecute RPC via axios; authentication by launching Chrome with remote debugging and reading cookies over CDP; a manual cookie/token path as fallback. No browser at runtime otherwise. MCP over stdio.

## RPC catalog (behavior; payloads described, not copied)

Transport (src/api-client.ts:15-135): base `https://notebooklm.google.com`; batchexecute path `/_/LabsTailwindUi/data/batchexecute` with query `rpcids`, `source-path`, `bl`, `hl=en`, `_reqid` (counter += 100000), `rt=c`, optional `f.sid`. `bl` is scraped from the homepage via the key `cfb2h`, with a second regex on a literal `"bl"` key as fallback, default `boq_labs-tailwind-frontend_20260121.08_p0` (api-client.ts:55-66,102-108). CSRF `SNlM0e` regex from the homepage (`refreshAtToken`, api-client.ts:40-72). Browser-like headers on the axios instance: macOS-less Linux Chrome 131 UA, `X-Goog-AuthUser: 0`, Sec-Ch-Ua and Sec-Fetch-* headers, Origin, Referer (api-client.ts:19-37). Body `f.req` triple-nested envelope + `at` + trailing `&`; query body uses the `[null, paramsJson]` envelope (api-client.ts:74-97). Response parsing returns only the FIRST `wrb.fr` envelope found, regardless of requested id (api-client.ts:117-140): wrong when multiple envelopes exist. No gRPC status decode, no retry other than an `at` refresh helper.

Ids declared (src/constants.ts:1-30): wXbhsf, rLM1Ne, CCqFvf, s0tc2d, WWINqb, izAoDd, hizoJc, yR9Yof, FLmJqe, tGMBJ, b7Wfje (rename source), hPTbtc, hT54vc (PREFERENCES), ozz5Z (SUBSCRIPTION), ZwVcOc (SETTINGS), VfAZjd, tr032e, Ljjv0c, QA9ei, e3bVqc, LBwxtb, R7cb6c, gArtLc, V5N4be, yyryJe, CYK0Xb, cFji9, AH0mwd. Same set as jacob-bd lineage; no new ids. Note `PREFERENCES hT54vc`, `SUBSCRIPTION ozz5Z` and `SETTINGS ZwVcOc` labels are shared with m4yk3ldev (same upstream source), whereas agmmnn calls hT54vc SET_USER_SETTINGS and roomi calls ozz5Z ADD_SOURCE_URL_V2: names unreliable.

Verified-looking behaviors (agree with other upstreams): list-notebooks uses a first param `null` and second param 1/2 here (`[null,2]`, api-client.ts:148) versus `[null,1,null,[2]]` elsewhere: probably wrong or at least a different mode; create-notebook, rename, delete and add-source (url/text/drive) use the same shapes as roomi/mood/agmmnn (api-client.ts:206-262, 381-420); research fast/deep param shapes match (api-client.ts:266-283), including that deep research returns its task id at index 1 and report id at index 0, the inverse of fast (api-client.ts:279-280) [unique detail, worth live check]; sync drive and freshness; chat configuration reuses the rename RPC s0tc2d with the chat-settings block at mutation slot 7 (api-client.ts:232-252, agrees with moodRobotics/agmmnn).

Clearly wrong or unverified (do not trust): get-notebook sends only the id (no `[2]`/`0` tail); import-research puts the task id before the `[1]` marker (api-client.ts:336); delete-source sends `[notebookId,[sourceId]]` (api-client.ts:435) while verified shape is `[[[sourceId]],[2]]`; rename-source uses b7Wfje with `[notebookId,[[sourceId,title]],[2]]` (api-client.ts:426) differing from mood/agmmnn; list-studio sends `[notebookId, types]` (api-client.ts:441-445) and delete-studio `[notebookId,[artifactId]]` (485): both not the `[[2], nbId, ...]` forms; studio type codes REPORT 3, VIDEO 5, INFOGRAPHIC 6, SLIDE_DECK 7, FLASHCARDS 9 in constants.ts:58-65 contradict the verified 2, 3, 7, 8, 4; `generateMindMap` sends `[notebookId, sourceIds, null]` and `createStudioArtifact` sends `[notebookId, type, config]` (api-client.ts:521-533), which are invented shapes. Chat query sends an EMPTY sources array `[]` ("or [] for all", api-client.ts:512) and a made-up conversation id string like `session-xxxxx` (not a UUID); returns the raw streamed response without parsing (api-client.ts:494-518). So chat, studio creation, mind map are effectively non-functional/untested here [unverified; no tests, no verification of live behavior; src/verify-all.ts and verify-research.ts are ad-hoc scripts].

## Streaming/chat

Endpoint and params as in other upstreams (`GenerateFreeFormStreamed`, `bl`, `hl`, `_reqid`, `rt=c`, `f.sid`; body `[null, json]` envelope). The client returns the raw response text; no answer extraction, no citations, no conversation handling.

## Auth lifecycle

- Storage: `~/.notebooklm-mcp/auth.json` (src/browser-auth.ts:321-322, index.ts:38, auth-cli.ts:11) - same directory name as moodRobotics and m4yk3ldev, so installing two of them collides; no permission hardening seen [unverified].
- Browser auth (src/browser-auth.ts): find Chrome, spawn with `--remote-debugging-port=9222` (fixed port, REMOTE_DEBUGGING_PORT at :23) and `--user-data-dir=~/.notebooklm-mcp/chrome-profile` (:261-278), connect over websocket and issue `Network.getAllCookies` (:153), save cookies + CSRF. Fixed port 9222 conflicts with a developer's own Chrome debugging session and exposes CDP to whatever listens on that port; address binding not verified [unverified].
- `chrome-extract.ts`: tries to read Chrome's SQLite cookie DB, immediately concedes cookies are encrypted (DPAPI/keychain) and prints manual instructions: dead code; shows why DB extraction is not viable without OS-specific decryption (which is itself a technique other tools use: see jacob-bd / notebooklm-connector in other notes).
- Manual: `authenticate` tool accepts a pasted cookie header and `at` token (README).
- Refresh: only `refreshAtToken()` (re-scrape CSRF/bl); no cookie rotation.
- Single account.

## MCP tools (7, consolidated style; src/index.ts:66-149)

`manage_notebook` (create/list/get/rename/delete/chat-config actions), `manage_source` (add text/url/drive, sync, freshness, rename, delete), `perform_deep_research` (start, poll, import), `generate_artifact` (audio, video, mind map, study guide, quiz), `query_notebook`, `authenticate`, `manage_studio` (list/delete artifacts). Few action-multiplexed tools instead of one tool per RPC (cf. strato-mcp-notebooklm's 13 workflow tools).

## Versions

No tags; 6 commits on one day: initial server, roadmap "based on expert analysis" (ROADMAP.md), browser auth via CDP, chat config + drive sync + freshness, studio list/delete, README.

## Adopt (ideas only, no code)

1. Action-multiplexed tool surface (7 tools) as a way to keep the MCP tool list short for agents; consider for our server in addition to or instead of 1:1 tools.
2. Scrape `bl` from `cfb2h` with a fallback regex on a generic `"bl"` key.
3. Document that SQLite cookie-DB extraction is a dead end without per-OS decryption.

## Avoid

1. Returning the first `wrb.fr` envelope regardless of RPC id (api-client.ts:117-140).
2. Invented payloads for studio/mind map/query (see above); copying ids without payload verification.
3. Fixed CDP port 9222 and shared `~/.notebooklm-mcp` directory name.
4. Raw unparsed chat output; empty source list for chat.
5. Shipping without tests or license; manifest says ISC with no LICENSE file.

## Evidence

Read: src/constants.ts (all), src/api-client.ts (1-140 full; remaining by grep and targeted sections 232-262, 266-340, 381-540), src/chrome-extract.ts (all), src/browser-auth.ts (grep of port, CDP method, paths), src/index.ts (tool name grep), README.md (1-25), package.json license field. `git log` (6 commits).
