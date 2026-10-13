# arlinamid/notebooklm-mcp

- URL: https://github.com/arlinamid/notebooklm-mcp (fork/continuation of PleasePrompto/notebooklm-mcp; 55 commits after the archived parent HEAD `0017998`, 0 behind: `git log 0017998..HEAD | wc -l` = 55)
- Language / runtime: TypeScript (ESM), Node >= 18, MCP SDK (stdio + Streamable HTTP + experimental MCP "tasks"), Patchright driving real Chrome.
- License: **MIT** with two copyright lines, "Copyright (c) 2025 Please Prompto!" and "Copyright (c) 2026 János Rózsavölgyi (arlinamid)" (LICENSE:1-4). research/upstreams.md lists it as NOASSERTION (GitHub's detector failed on the dual copyright); the file is plain MIT, so short snippets are allowed. Wire formats inside are credited to jacob-bd/gemini-notebook-mcp-cli (MIT, Jacob Ben David) in `src/notebooklm/rpc.ts:15-17` and `chat-rpc.ts:21-22`.
- Stars / last commit: 4 stars; HEAD `69fee3e` 2026-10-07 (merge of release 3.4.0). Version in package.json: 3.4.0.
- Versions reviewed: tags v3.0.0 (2026-10-04), v3.1.0, v3.1.1, v3.2.0, v3.3.0, v3.3.1, v3.4.0 (2026-10-07); HEAD 69fee3e. Clone: `research/upstreams/arlinamid__notebooklm-mcp`.
- REA: `rea analyze-javascript-application ... --artifact-format directory` run (571 module nodes, 74 assets, 1 `endpoint` node, 2 `storage` nodes). The REA module map (exports/imports per file) located the RPC layer: files `notebooklm/{rpc,rpc-ops,source-ops,studio-rpc,chat-rpc,chat-history,studio-ops,studio-download,account-notebooks}.ts` import only `patchright` (+`crypto`), i.e. all RPC is executed through the Playwright `Page`, never from Node. `coordination/coordinator.ts` is the only file importing the MCP *client* SDK (`client/index.js`, `client/streamableHttp.js`) = the multi-process leader/follower proxy. `utils/page-lock.ts`, `utils/request-context.ts`, `utils/file-roots.ts`, `tasks/task-runner.ts` have no external imports beyond the MCP SDK / node builtins.

## Access method

**Hybrid, and the closest existing design to our target:** the internal `batchexecute` RPC (and the streamed chat endpoint) is called with `fetch()` *from inside the signed-in notebook tab* via `page.evaluate`, with the UI path kept as automatic fallback. Evidence: `src/notebooklm/rpc.ts:1-19` ("Calls run inside the signed-in notebook tab (fetch from the page), so the browser sends its own, device-bound cookies; nothing is copied out of the profile ... faster and does not break when the layout changes"); fallback policy `tryRpc`/`requireRpc` in `src/session/browser-session.ts:1365-1405`; kill switch `NOTEBOOKLM_USE_RPC=false` (`rpc.ts:35-37`).

Which operations have which path:
- RPC primary, UI fallback: `ask_question` (`browser-session.ts:596-640`, falls to `askUnlocked` DOM path), `add_source` text/url/youtube (rpc-ops.ts:416-565, UI `sources.ts`), `generate_studio_artifact` for simple option sets (`studio-rpc.ts:108-259`, returns null -> dialog path for Generate-later, interactive reports, unknown language, unsupported options, :115-143), `list_sources`, `import_account_notebooks`, `configure_chat`, `get_usage`, `configure_output_language`.
- RPC only ("this operation has no UI fallback", `browser-session.ts:1380-1384`): create/rename/delete/pin notebook, collections, share, research, get_source, rename_source/artifact, suggest_reports, chat history read, download_studio_artifact.
- UI only: delete_source and delete_studio_artifact (`deletion.ts`, menus + confirm dialogs), audio generate/status/download via DOM (`audio.ts`), save_answer_as_note, convert_note_to_source (`notes.ts`), citation tooltip scraping, file upload via hidden `input[type=file]` [unverified: add_source type=file path not read].

Browser still required because auth is the browser profile: the token needed for every call, `SNlM0e`, is read from the page source (`rpc.ts:59-65`); the server never stores or replays cookies.

## RPC catalog

Endpoint (all rows): `POST {origin}/_/LabsTailwindUi/data/batchexecute?rpcids=<id>&source-path=<path>&bl=<cfb2h>&f.sid=<FdrFJe>&hl=en&rt=c` (`rpc.ts:67-76`), origin = the page's own (`fetch` with a relative URL, host is `notebook.google.com` after the 2026 rename; config.ts:24-31). Headers: `Content-Type: application/x-www-form-urlencoded;charset=UTF-8`, `X-Same-Domain: 1`, `credentials:"include"` (`rpc.ts:75-83`). Body: `f.req=` + urlencode(JSON `[[[rpcId, JSON.stringify(params), null, "generic"]]]`) + `&at=` + urlencode(SNlM0e) + `&` (`rpc.ts:66, 82`). **No `_reqid` is sent on batchexecute** (only on the chat stream, `chat-rpc.ts:102`). Token acquisition: regex over `document.documentElement.innerHTML` for `"SNlM0e":"..."` (at), `"cfb2h":"..."` (bl), `"FdrFJe":"..."` (f.sid), unescaping `=` to `=`; done instead of `window.WIZ_global_data` because "the stealth browser evaluates in an isolated world" (`rpc.ts:59-65`). No refresh logic: if `SNlM0e` missing -> RpcError "no CSRF token on the page (not signed in?)" (:65); HTTP 401/403 -> "needs a fresh login (setup_auth)" (:90-95); response code 16 inside the envelope -> "authentication expired" (:118-120). Response parsing: strip `)]}'`, split lines, `JSON.parse` each line starting with `[`, find `["wrb.fr", rpcId, "<json>", ..., error]` where `item[5][0]` is a numeric error code; `item[2]` is the payload JSON string (:101-130). `source-path` is `/` for account-level calls and `/notebook/<uuid>` for notebook-level ones.

Common fixed params: `USAGE_HEADER` / `HEADER` = `[2, null, [1], [1, null, null, null, null, null, null, null, null, null, [1, 3]]]` (`rpc-ops.ts:43-48`, `chat-rpc.ts:34-39`); `[2]` = platform web; `ADD_SETTINGS` = `[1,null,...,[1]]` (rpc-ops.ts:418).

| RPC id | Purpose | Request payload (`params`) | Response shape | file:line |
|---|---|---|---|---|
| wXbhsf | list notebooks | `[null, 1, null, [2]]`, path `/` | `[0]` = list of `[title, sources[], uuid, emoji, ?, [scopeFlag(1=mine), public?, ..., createdTs at [8]]]` | rpc-ops.ts:41, 151-177; rpc.ts:24 |
| rLM1Ne | get notebook (title, sources, settings) | `[notebookId, null, [2], null, 0]` | `[0]=[title, sources[], uuid, ..., settings at [7]]`; each source `[[id], title, meta[], [?, status]]`; `null` instead of `[]` for a notebook with no sources | rpc-ops.ts:39, 101-107, 435-460; chat-rpc.ts:58-64 |
| CCqFvf | create notebook | `[title, null, null, [2], [1,null,...,[1]]]`, path `/` | `[2]` = new uuid | rpc-ops.ts:183, 189-201 |
| s0tc2d | update notebook (rename / chat goal+length) | rename: `[id, [[null,null,null,[null,title]]]]`; chat: `[id, [[null x7, [[goalCode(,prompt)], [lenCode]]]]]` (goal 1 default/2 custom+prompt/3 learning_guide; length 1/4 longer/5 shorter) | rename: `[0]` = saved title; chat: settings at `[7]` | rpc-ops.ts:40, 54-57, 132-141, 209-220 |
| WWINqb | delete notebook | `[[id], [2]]`, path `/` | none (verified by re-listing) | rpc-ops.ts:184, 245-250 |
| LQhfEb | pin / unpin notebook | `[USAGE_HEADER, id, [null,[1|0]], [["notebook_lm_state.is_pinned"]]]` | `[0][1][0]` = bool | rpc-ops.ts:185, 223-239 |
| EylDcb | usage / limits | `[USAGE_HEADER]` | `[1]` = windows `[.., type at [4], resetTs at [5], fraction at [6]]` (type 1 "current AI usage", 2 "weekly limit") | rpc-ops.ts:38, 65-80 |
| I3xc3c / agX4Bc / le8sX / GyzE7e | list / create / edit / delete "collections" | list `[USAGE_HEADER,null,3]`; create `[USAGE_HEADER,null,null,null,null,[[name],null,notebookIds],3]`; edit `[USAGE_HEADER,null,collId,payload,3]` with payload `[[],[[name]]]`, `[[null,null,null,[[nbId]]],[]]` (add), `[[null,null,null,null,[[nbId]]],[]]` (remove); delete `[USAGE_HEADER,null,[collId],3]` | collection rows `[name, notebookIds[], id, emoji]` | rpc-ops.ts:256-355 |
| JFMDGd / QDyure | share status / set public link | status `[id,[2]]`; set `[[[id,null,[1|0],[0,""]]],1,null,[2]]` | people `[email, role(1 owner/2 editor/3 viewer), ?, [name]]`, public flag at `[1][0]` | rpc-ops.ts:360-411 |
| izAoDd | add source(s) | `[entries, notebookId, [2], ADD_SETTINGS]`; text entry `[null,[title,content],null,2,null x6,1]`; url `[null,null,[url],null x7,1]`; youtube `[null x7,[url],null,null,1]` | `[0]` = list of `[[sourceId], ...]` | rpc-ops.ts:416, 470-540 |
| ozz5Z | add-source fallback for URLs when izAoDd errors with code 3 or 9 | `[urls.map(u => [[null,u,627],[null x9,[null,null,1]],1])]` | none; ids found by re-listing | rpc-ops.ts:417, 519-538 (icebear0828 labels `ozz5Z` GET_UI_CONFIG, rpc-ids.ts:63; semantics disagree, treat as [unverified]) |
| ZwVcOc / hT54vc | get / set account settings (output language) | get `[USAGE_HEADER]`; set `[[[null,[[null,null,null,null,[code|""]]]]], USAGE_HEADER]` | language code at `[0][2][4][0]` | rpc-ops.ts:586-620 |
| hizoJc | get source text | `[[sourceId],[2],[2]]`, path `/` | `[3]` nested blocks of text | source-ops.ts:34, 176-210 |
| tr032e | source guide (summary + keywords) | `[[[[sourceId]]]]` | `[0][0]=[?, [summary], [keywords]]` | source-ops.ts:35, 154-169 |
| b7Wfje | rename source | `[null, [sourceId], [[[title]]]]` | `[0][1]` = saved title | source-ops.ts:40, 46-61 |
| Ljjv0c | start FAST research | `[[query, corpus(1 web/2 drive)], null, 1, notebookId]` | `[0]` = task id | source-ops.ts:36, 325-350 |
| QA9ei | start DEEP research | `[null, [1], [query, corpus], 5, notebookId]` | `[0]` = task id | source-ops.ts:37, 333-340 |
| e3bVqc | poll research | `[null, null, notebookId]` | rows `[taskId, [?, [query,corpus], mode(5=deep), [candidates[], summary], status(2/6 done,3 failed)]]`; candidate `[url,title,desc,typeCode,.. ,6=[?,cited?,passage],7=imported,8=citation#]`; type 5 = the deep report | source-ops.ts:38, 280-322 |
| LBwxtb | import research results | `[null,[1],taskId,notebookId,entries]`; drive entry `[[driveId,mime,1,title],null x9,2]`; web `[null,null,[url,title],null x7,2]` | `[0]` = rows `[[id],title]` | source-ops.ts:39, 356-397 |
| R7cb6c | create Studio artifact | `[[2], notebookId, content]`; content per type: audio `[null,null,1,[[id]..],null,null,[null,[prompt,length,null,[id]..,lang,null,format]]]`; video type 3; infographic 7; slide_deck 8; report 2 (see file). Type codes `{audio:1,report:2,video:3,infographic:7,slide_deck:8}` | `[0]=[artifactId, ?, typeCode,..]` (code checked against requested type) | studio-rpc.ts:37-40, 160-259 |
| gArtLc | list Studio artifacts | `[[2], notebookId, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | artifacts `[id, title, type, ..., status at [4] (3 = ready), ...]`; type codes 1 audio/2 report/3 video/4 interactive(quiz,flashcards,mind map via `[9][1][0]`)/7 infographic/8 slide deck/9,10 data table/11 report | rpc.ts:25-26; studio-download.ts:22-79 |
| v9rmvd | get artifact (interactive HTML of quiz / flashcards / mind map) | `[artifactId]` | `[0][9][0]` = HTML string | rpc.ts:27-28; studio-download.ts:159-166 |
| rc3d8d | rename Studio item | `[[artifactId, title],[["title"]]]` | `[1]` = saved title | studio-ops.ts:21, 94-111 |
| ciyUvf | suggest reports | `[SUGGEST_HEADER, notebookId, [[sid]..]]` | rows `[title, description, ?, ?, prompt, audience(1 general/2 expert)]` | studio-rpc.ts:264, 295-316 |
| VfAZjd | notebook overview (summary + suggested questions) | `[notebookId,[2]]` | summary, topics `[question, prompt]` | studio-rpc.ts:265, 319-338 |
| hPTbtc | list chat conversations | `[[], null, notebookId, 20]` | `[0]` = rows `[convId, createdTs, updatedTs]` | chat-history.ts:101-112; chat-rpc.ts:69 |
| khqZz | list conversation turns (paged) | `[HEADER, null, null, convId, 20(, cursor)]` | `[0]` turns (newest first; `t[2]`=1 question text at `t[3]`, 2 answer text at `t[4][0][0]`), `[1]` = next cursor | chat-history.ts:118-152; chat-rpc.ts:73-82 |
| J7Gthc | delete chat history | `[HEADER, convId, null, 1]` | none (verified by reading back turn count) | chat-history.ts:478, 486-505 |
| (stream) GenerateFreeFormStreamed | ask a question, see "Streaming/chat" | see below | chunks | chat-rpc.ts:31-33, 85-115 |

Not RPC: Studio file downloads use `context.request.get(url)` (Playwright APIRequestContext with the profile's cookies, 600 s timeout, up to 10 redirects), reject `text/html` responses (login page) and add " (n)" suffix to avoid overwriting (`browser-session.ts:1417-1460`).

## Streaming/chat

- Endpoint: `POST /_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=&hl=en&_reqid=<100000+rand(900000)>&rt=c&f.sid=` (`chat-rpc.ts:31-33, 99-105`).
- Body: `f.req=` urlencode(`[null, JSON.stringify(params)]`) `&at=<SNlM0e>&`, with `params = [ids.map(id => [[id]]), question, history|null, [2, null, [1]], convId]` (:85-91, :106-114). `ids` = all source ids of the notebook (from `rLM1Ne`) or only the chosen ones (`sources` arg) - scoping is done by sending only those ids, "the notebook's checkbox selection is never touched" (:11-13).
- Conversation continuity: first `hPTbtc` to get the notebook's server-side conversation id (random UUID if none, :69-70); then `khqZz` last 20 turns converted to `[text, null, 1|2]` pairs sent as `history` (:71-83). Follow-ups therefore work with no browser tab state and appear in NotebookLM's own chat (:9-11).
- Response: the whole body is read with `response.text()` (not incremental); the page-side loop parses every `wrb.fr` chunk, keeps the longest chunk where `first[4][last] === 1` (2 = "thinking" text, skipped) (:140-176). Chunks are cumulative so the longest is the final one. Each chunk `inner[0] = [text, null, [convId, ...], null, [segments, null, null, passages, 1]]` (:16-19).
- Citations: passage i is citation `[i+1]`; its source id is at `passage[1][5][0][0][0]`, cited text segments at `passage[1][4]` (walk: `seg[2]` groups -> `inner[2]` strings) (:119-138, :168-173). `[1-3]` / `[1, 4]` ranges in the answer are expanded to `[1][2][3]` / `[1][4]` before formatting (`expandCitationRanges` :213-226). Only the answer and passages leave the page, "the stream is often megabytes of cumulative chunks".
- Failure replies: `^I'm having trouble responding right now\.?$` (<= 200 chars) -> error (`chat.ts detectFailureReply`, browser-session.ts:~620). Error code in envelope `item[5][0]` -> RpcError "query rejected with code N" (chat-rpc.ts:190-195).
- UI path (fallback) differs from the parent: counts answer bubbles `.chat-message-pair, .to-user-container` (`Selectors.chat.turn`) and treats `count > priorCount` as a new answer so an answer identical to an old one no longer hangs; hides `thinking-chain-view, .thinking-chain` and rewrites citation chips to `[N]` inside `evaluateAll` before `innerText`; placeholder check only for text < 120 chars; waits `waitForChatHistory` until the bubble count is unchanged 1.5 s (max 12 s) before snapshotting (diff of chat.ts against 0017998).

## Auth lifecycle

Inherits PleasePrompto's (persistent Chrome profile + `state.json` + `session.json` + 24 h file-age rule, `auth-manager.ts` unchanged except host checks). Changes:
- Host awareness: Google renamed NotebookLM to "Gemini Notebook" and moved to `notebook.google.com` (2026-09 per config.ts:21-23 comment; paolodalprato dates it July 2026 and Pantheon "2026"; dates disagree, host change itself is confirmed by three repos). `isNotebookLMUrl()` accepts both hosts (`config.ts:34-41`) and replaces every `startsWith("https://notebooklm.google.com/")` in auth (diff of auth-manager.ts); `normalizeNotebookUrl()` rewrites legacy links at session creation and comparison (`config.ts:50-61`, `session-manager.ts:75, 220`). The login entry URL is rebuilt from `NOTEBOOKLM_BASE_URL` (:65-69).
- Promo dialogs: since the rebrand a `mat-dialog-container[role=dialog]` hosting e.g. `<accessibility-promo-dialog>` blocks clicks and is mistaken for the add-source dialog; `dismissPromoDialogs()` closes only dialogs whose Angular host tag matches `/promo|announcement|onboarding|whats-?new|welcome|tutorial|tour/i`, marks unclosable ones `data-mcp-promo-dialog` and the dialog selectors exclude that mark (`dialogs.ts:1-40`, `selectors.ts` overlayPane `:not([data-mcp-promo-dialog])`).
- Cross-process: one browser per Google account. First instance becomes *leader* (writes `leader.json` = pid, port, random token via exclusive create, serves token-protected Streamable HTTP on 127.0.0.1); others are *followers* that forward tool calls, plus elicitation/sampling/roots/progress, to the leader; dead-pid lock is stolen and the call retried once (`coordination/coordinator.ts:1-60, 80-120`). `NOTEBOOKLM_SINGLE_BROWSER=false` disables. Reason given: two Chromes on one Google session may get the session revoked (:7-17).
- `getContextOrCreate` serialised with a promise chain (`shared-context-manager.ts` diff: `contextReady`), fixing a double-launch race.
- Chrome sandbox: `sandboxArgs()` adds `--no-sandbox` only on Linux under WSL or uid 0, override `NOTEBOOKLM_NO_SANDBOX` (`browser/chromium-fallback.ts` diff).
- Multi-account: still `--account`/`NOTEBOOKLM_ACCOUNT` one per process (index.ts env list).

## Selectors

Same registry-file design as the parent; the diff shows the churn Google introduced during the rebrand, which is the best evidence for how fast selectors rot:
- Audio icon renamed: `audio_magic_eraser` -> `audio_spark` (both listed now), tiles gained `artifact-library-item:has(button[aria-description="Audio Overview"]...)`, in-progress tile detected by `mat-icon.artifact-icon:text-is("progress_activity")` + `data-studio-accent="blue"`.
- Add-source dialog host changed to `mat-dialog-container[role="dialog"]`; type buttons now `button:is(.source-action-button, .drop-zone-icon-button)`; link icon `link_2` added; confirm `button.mdc-button--unelevated.mat-primary`.
- Studio dialog: `mat-dialog-container:has(configurable-form-dialog)`; Generate now / Generate later buttons (`mat-dialog-actions .action-button-wrapper button`, text fallbacks).
- Query box: aria-label `Query box`, `query-box .submit-button button`.
- Citation tooltip: `xap-inline-dialog-container .citation-tooltip(-header|-text)`; source viewer `section.source-panel source-viewer`, close via `collapse_content` icon.
- New areas: deletion (`[role="menuitem"]:has(mat-icon:text-is("delete"))`, dialog hosts `delete-source`, `delete-dialog`), notes (`keep_pin` save, `note-editor`, `artifact-library-note`, `convert_to_text`), settings/usage dialog (`usage-limits-dialog`, `expected-usage [role="progressbar"]`), source rows by id `source-item-more-button-<id>`.
Strategy to note: DOM is now the *fallback*; for 25+ operations the RPC path makes selectors irrelevant, and the RPC id layer is a single file (`rpc.ts` + `*-ops.ts`) with explicit `at(v, ...path)` accessors that return `undefined` rather than throw on shape drift, and "did-it-take" verification by re-reading after every mutation (rename, pin, delete, share, language, chat config).

## Stealth / anti-detection

Unchanged from parent (humanType, randomDelay, persistent profile, `--disable-blink-features=AutomationControlled`, Patchright). The RPC path sends one real browser request per call, so it is invisible to fingerprinting beyond cookie+token use. No extra anti-bot code added (stealth-utils.ts +2 lines).

## Browser / page lifecycle, pooling, timeouts, concurrency

- Same shared persistent context + Map of tabs, `maxSessions` 10, idle 900 s (`session-manager.ts:30-46`, `config.ts:148`). **Idle cleanup skips busy tabs** (`!session.busy`, :248) - parent bug fixed.
- **Per-tab FIFO queue**: `PageLock` (`utils/page-lock.ts:14-75`) promise-chain mutex with labels, `queued` count, cancellation-aware (`abortable(previous)`), sends "Queued behind <op> on this session (N waiting)" progress. All tab operations go through `exclusive()`/`onPage()` (`browser-session.ts:234-285`). So two calls on one tab serialize; calls on different tabs run in parallel; RPC calls still go through the tab's page so they also queue behind UI work on that tab.
- Cancellation/progress/elicitation context: `AsyncLocalStorage<RequestContext>` with `signal`, `progress`, `choose`, `sample` (`utils/request-context.ts`); MCP `notifications/cancelled` aborts waiting.
- Timeouts: `add_source` waits at most 40 s with 2 s polls and progress, reporting still-processing sources as pending (`rpc-ops.ts:476, 543-563`; CHANGELOG 3.3.1: 90 s hit clients' 60 s request timeout); answers 600 s; audio default 600 s; forward timeout between instances 60 min with `resetTimeoutOnProgress` (`coordinator.ts:~48`); Studio download 600 s.
- MCP "tasks" (spec 2025-11-25, experimental): `ask_question, add_source, research_sources, import_research_sources, generate_audio, generate_studio_artifact` can run as background tasks with `tasks/get|result|cancel` (`tasks/task-runner.ts:1-50`).

## Notebook library / session management

Library as parent (`library.json`) plus: `import_account_notebooks` pulls notebooks from `wXbhsf` with scope mine/shared/all, dry-run, query filter; `create_notebook` registers and selects the new notebook; `delete_notebook` drops entry. Destructive tools use MCP **elicitation** for user approval, falling back to a `confirm:true` parameter when the client lacks elicitation (`handlers.ts:73-79, 1172-1195`). File paths for uploads/downloads must lie inside the client's `roots/list` or `NOTEBOOKLM_FILE_ROOTS`, symlinks resolved (`utils/file-roots.ts:1-30`). Research results are vetted before import: queries < 4 words rejected, duplicate runs answered from history, candidates need `reliability` high/medium plus reason, blocked domains env, < 500 indexed words flagged (`notebooklm/research-policy.ts`, CHANGELOG 3.3.0).

## MCP tools (38 + 2 local prompt tools; docs/tools.md)

Common targeting params on every notebook tool: `session_id, notebook_id, notebook_url, show_browser`.

| Tool | Params | Behavior |
|---|---|---|
| ask_question | question; source_format none/inline/footnotes/json; sources[]; browser_options | RPC stream then DOM fallback; returns answer, citations, session info |
| add_source | type url/youtube/text/file; content; file_paths[]; title | RPC add + poll status up to 40 s; UI fallback |
| research_sources | query; mode fast/deep; corpus web/drive; task_id; wait_seconds; include_report; all_candidates | Start/read research; never imports |
| import_research_sources | selections[{index,reliability,reason}]; task_id | Imports vetted candidates |
| get_source | source; include_text; offset; max_chars | metadata + guide + paged text |
| list_sources / delete_source / rename_source | (source; confirm; title) | list via RPC; delete via UI with approval |
| generate_audio | custom_prompt; format deep_dive/brief/critique/debate; length; sources[]; generate_later; wait_for_completion; timeout_ms | async |
| get_audio_status / download_audio | destination_dir | DOM status / menu download |
| generate_studio_artifact | type video/slide_deck/mind_map/report/flashcards/quiz/infographic/data_table/audio; prompt; format; length; count; difficulty; include_images; orientation; detail; style; language; sources[]; title; template; generate_later; ask_options | RPC when expressible, dialog otherwise |
| list_studio_artifacts / get_studio_artifact / rename_studio_artifact / delete_studio_artifact / download_studio_artifact | ids, destination_dir, format | gArtLc, rc3d8d, v9rmvd, URL download (audio .m4a, video .mp4, infographic .png, slides .pdf/.pptx, report .md, data_table .csv/.xlsx, quiz/flashcards .md/.json/.html, mind_map .json) |
| suggest_reports | sources[] | ciyUvf |
| save_answer_as_note / convert_note_to_source | question; note_title; all | UI |
| configure_chat | goal; custom_prompt; response_length | s0tc2d |
| configure_output_language / get_usage | language | ZwVcOc/hT54vc, EylDcb |
| get_chat_history / delete_chat_history | format; destination_dir; include_citations; max_turns / backup_dir; confirm | khqZz paging / J7Gthc |
| add_notebook, import_account_notebooks, create_notebook, rename_notebook, delete_notebook, pin_notebook, list_collections, manage_collection, share_notebook, list_notebooks, get_notebook, select_notebook, update_notebook, remove_notebook, search_notebooks, get_library_stats | library + account notebook ops | see RPC table |
| list_sessions, close_session, reset_session | session_id | tabs |
| get_health, setup_auth, re_auth, cleanup_data | | system |
Also MCP prompts and resources, `list_prompt_templates`/`get_prompt_template` (local), server icons, output schemas (`tools/output-schemas.ts`).

## Reliability

- Fallback ladder: RPC -> (on `RpcError` that is not auth code 16) log + UI path; auth errors propagate (`browser-session.ts:1391-1405`). Errors from RPC-only features say "Google may have changed it; this operation has no UI fallback" (:1380).
- Every mutating RPC is verified by a follow-up read (`rpc-ops.ts:142-146, 215-218, 235-237, 247-249, 338-340, 407-409, 613-617`), turning silent no-ops into errors.
- Shape guards: an unexpected response shape throws RpcError instead of reading as "empty" (`rpc-ops.ts:157-159`), but null-for-empty-list must be handled (3.3.1 bug: empty notebook returns `null`, `notebookSourceList` :451-458).
- Rate limit: `usage limit reached` texts in DOM path; `get_usage` exposes window percentages and reset time (EylDcb).
- Logging to `<dataDir>/logs/server.log` with pid (3.3.1).

## Performance notes

RPC path: one in-page `fetch` per call (tens to hundreds of ms plus Google latency) versus seconds of simulated typing and polling in the UI path. Chat via stream costs 3 round trips (rLM1Ne, hPTbtc, khqZz) + stream; we can cache source ids and conversation id. Megabyte streams are parsed in-page so only the answer crosses the CDP boundary.

## Tests and quality

`scripts/` contains offline tests (e.g. `scripts/chat-history-test.mjs` against a fake NotebookLM, per CHANGELOG 3.4.0), a CI workflow with a protocol smoke test (commit 3882e8d), `npm audit` gate on production deps (1b1ac0f), trusted publishing (186b7f3). No unit test framework in package.json [unverified]. Third-party notices file credits jacob-bd.

## Versions

| Tag | Date | Notable |
|---|---|---|
| v3.0.0 | 2026-10-04 | Gemini Notebook compatibility, Studio, notes, deletion, MCP protocol features (972af3e); fixes for open upstream bugs |
| v3.1.0 | 2026-10-04 | `import_account_notebooks` |
| v3.1.1 | 2026-10-05 | close sidebar source view after citations; failure replies reported |
| v3.2.0 | 2026-10-05 | per-session FIFO queue, `download_studio_artifact` via RPC, usage/chat config over RPC, one browser per account, add_source over RPC, Studio over RPC + account language, ask over streamed endpoint, `NOTEBOOKLM_USE_RPC` kill switch |
| v3.3.0 | 2026-10-06 | research_sources / import_research_sources / get_source, reports from selected sources, `suggest_reports`, agent skill + plugin manifests |
| v3.3.1 | 2026-10-06 | notebook management tools, collections, share, rename source/artifact, empty-notebook add_source fix, 40 s add_source wait |
| v3.4.0 | 2026-10-07 | `get_chat_history`, `delete_chat_history`, model label "Gemini 3.5 family" |

## Adopt

1. **The dual-transport contract itself**: RPC first inside the authenticated browser, UI fallback only on RPC *shape/protocol* errors, never on auth errors, with a per-feature "no fallback" class (`browser-session.ts:1365-1405`). Map directly onto our `Transport` interface.
2. The `callRpc` envelope and token scrape: SNlM0e / cfb2h / FdrFJe by regex over page HTML; envelope error code 16 = auth expired (`rpc.ts:51-130`). Our pure-HTTP transport needs the same three tokens from the notebook page HTML.
3. Read-back verification after every mutation (`rpc-ops.ts` list above).
4. `at(v, ...path)` tolerant accessors and `null`-list handling (`rpc-ops.ts:59-60, 451-458`).
5. Per-tab `PageLock` FIFO with queue reporting and cancel awareness (`utils/page-lock.ts`); idle sweeper must skip busy sessions (`session-manager.ts:248`).
6. Answer parsing details: choose chunk with `meta[last]===1`, passages at `[1][5][0][0][0]`, range expansion (`chat-rpc.ts:140-226`); server-side conversation continuation via hPTbtc + khqZz history.
7. Chat history export with cursor paging and loop-guard (`chat-history.ts:118-152`).
8. Promo-dialog sweeper limited by Angular host-tag regex (`dialogs.ts`), and excluding marked dialogs from selectors.
9. Leader/follower single-browser-per-account coordination with token-protected loopback endpoint and dead-pid lock stealing (`coordinator.ts`); relevant if several MCP clients start our binary on one profile.
10. Elicitation for destructive actions, file-roots enforcement, `roots/list` (utils/file-roots.ts, handlers.ts:73-79).
11. Research workflow with vetting policy (`research-policy.ts`) and `suggest_reports`/overview RPCs (ciyUvf, VfAZjd).
12. Artifact status/type tables (`studio-download.ts:22-79`, `source-ops.ts:75-98`).

## Avoid

1. Calling RPC through `page.evaluate` for everything means the browser is a hard dependency for all features (token only from page HTML; no headless token refresh; `rpc.ts:59-65`). Our primary pure-HTTP transport should hold cookies/tokens itself and use the browser only to obtain them or as fallback.
2. No `_reqid` increment and no retry/backoff on 429/5xx in `callRpc` (`rpc.ts:51-98`): a single try.
3. Reading the stream fully before parsing (`response.text()`, chat-rpc.ts:116): no incremental progress, memory for big streams. A Go client can stream-decode line by line.
4. `ozz5Z` fallback is a blind guess for URL adds (errors swallowed, `.catch(() => undefined)`, rpc-ops.ts:534).
5. Source/Studio deletion via UI only, although `tGMBJ` (delete source) and `V5N4be` (delete artifact) are known (icebear0828 rpc-ids.ts:19, 33).
6. Still relies on a 24 h file-age heuristic and plain `state.json` inherited from the parent.
7. UI selector lists with 8-locale text variants are large and only EN/DE verified; keep RPC as the main path rather than growing them.
8. Hard-coded `hl=en` in all calls (`rpc.ts:72`, `chat-rpc.ts:101`) - responses with localized strings (usage labels, errors) assume English.

## Evidence

Read: `src/notebooklm/{rpc,rpc-ops,source-ops,studio-rpc,studio-ops,studio-download,chat-rpc,chat-history,chat,dialogs}.ts` (RPC bodies and parsers), `src/session/browser-session.ts` (lock, tryRpc, ask flow, download), `src/utils/{page-lock,request-context,file-roots}.ts`, `src/coordination/coordinator.ts`, `src/config.ts:20-70`, `src/tasks/task-runner.ts:1-50`, `docs/tools.md` (param tables), CHANGELOG.md 3.0.0-3.4.0, `git diff 0017998 HEAD` for chat.ts, selectors.ts, auth-manager.ts, shared-context-manager.ts, chromium-fallback.ts. REA JSON module map for the import graph. No code was run.
