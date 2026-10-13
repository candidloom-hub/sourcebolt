# jacob-bd/gemini-notebook-mcp-cli (formerly notebooklm-mcp-cli, notebooklm-mcp)

- URL: https://github.com/jacob-bd/gemini-notebook-mcp-cli
- Language / runtime: Python >=3.11, FastMCP (`fastmcp>=2.0.0,<5.0`), httpx, typer, websocket-client (CDP), keyring + cryptography (pyproject.toml:26-43)
- License: MIT, (c) 2025 Jacob Ben David (LICENSE:1-3). Short snippets (<15 lines) are allowed under this note's rules.
- Stars / last commit: 6302 stars per research/upstreams.md; HEAD `1df1e76` 2026-10-06 ("fix: harden v0.15.4 headless refresh, downloads, aliases, query slots")
- Versions reviewed: tags v0.1.0 (2026-01-01) .. v0.15.4 (2026-10-06), 163 tags, 948 commits. Read at HEAD v0.15.4.
- PyPI package name is still `notebooklm-mcp-cli` (pyproject.toml:6); console scripts `nlm` and `notebooklm-mcp` (pyproject.toml:57-62); python package dir `src/notebooklm_tools/`; FastMCP server name `gemini-notebook-mcp` (src/notebooklm_tools/mcp/server.py:44-45).

## Access method

Internal `batchexecute` RPC over httpx with Google cookies. Browser (CDP) is used for login, headless token refresh, and an opt-in "in-page fetch" transport:
- batchexecute: `POST {base}/_/LabsTailwindUi/data/batchexecute` (src/notebooklm_tools/core/base.py:215-218, 233-235).
- Streaming chat: `POST {base}/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed` (base.py:225-228, 366-367 `QUERY_ENDPOINT`).
- File upload: resumable protocol at `{base}/upload/_/?authuser=0` (core/sources.py:866-927).
- Experimental `NOTEBOOKLM_RPC_TRANSPORT=cdp` sends the identical form POST through `fetch()` inside the saved Chrome profile page, so Chrome supplies browser-bound cookies (core/cdp_transport.py:1-8, 53-57; base.py:910-934, 948-969). This is the only browser fallback for data calls; everything else is httpx.
- Enterprise mode: same RPC machinery against a different path prefix and RPC ids (see "Enterprise" below). It is NOT the Discovery Engine REST API (that is Robiton's fork, see its note).

### The "Gemini Notebook" rebrand: what actually changed in code (answer to the recon question)

Evidence is the git history of this repo plus HEAD code. Nothing here is from Google directly.
- Only the HOST changed, not the RPC surface. Commit `5c772d1` (2026-07-25, v0.9.3): "Google is rolling out a rebrand ... that redirects some signed-in accounts to notebook.google.com instead of notebooklm.google.com". It broke login detection, API calls (client always used the old host) and the `NOTEBOOKLM_BASE_URL` allowlist. Fix: add `notebook.google.com` to `_ALLOWED_BASE_HOSTS` (utils/config.py:68-74), persist per-profile `base_host` in `metadata.json` (core/auth.py `AuthTokens.base_host`, `Profile.base_host`), route every client via `get_base_url(profile_host)` with precedence env `NOTEBOOKLM_BASE_URL` > profile `base_host` > default `https://notebooklm.google.com` (utils/config.py:77-106). Test files added: tests/test_base_url_resolution.py, tests/test_base_host_capture.py.
- Default host is STILL `notebooklm.google.com` at v0.15.4 (utils/config.py:105; base.py:233). Rebrand is a per-account redirect rollout, not a hard cutover. A Go client must support both hosts and learn which one the account lands on at login.
- Enterprise/Workspace variant host `notebook.cloud.google.com` added in v0.9.4 (commit `ed21a32`, 2026-07-25; utils/config.py:47-52 in that commit, now :71). Login host check at utils/cdp.py:1380-1398 (`_is_notebooklm_url` accepts four hosts).
- Path, RPC ids and cookie model were NOT changed by the rebrand: the consumer URL path is still `/_/LabsTailwindUi/data/batchexecute` and the query service still `google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService` (base.py:218, 366-367). The `bl` fallback is still `boq_labs-tailwind-frontend_20260108.06_p0` (base.py:236); live `bl` is scraped from the page (`"cfb2h"`, base.py:1308-1313 region / utils/cdp.py:1418-1426). No RPC id was renamed in the rebrand commits (5c772d1, ed21a32 diffs touch only host lists, base_host plumbing and a Chrome-handoff error message).
- Doc-only rebrand: commit `a66a33d` (2026-07-25) "docs: rebrand Google NotebookLM to Gemini Notebook" changes 28 files of docs/skills text; commit `74d3575` (2026-07-25) "update repository URLs to gemini-notebook-mcp-cli" = the GitHub repo rename. The Python module (`notebooklm_tools`), the PyPI name (`notebooklm-mcp-cli`) and the `NOTEBOOKLM_*` env vars were NOT renamed. Only the MCP server `name=` string became `gemini-notebook-mcp` (mcp/server.py:44-45) and the default download dir is `~/Downloads/gemini-notebook` (mcp/tools/downloads.py docstring).
- Date vs claim: first rebrand-related commit is 2026-07-25, nine days after the 2026-07-16 date in research/upstreams.md (taken from moodRobotics README). The commit message calls it a "rollout", so 07-16 as the public date is [unverified here]; this repo only proves it was live for users by 07-25 (issue #269).
- New products after rebrand: Enterprise support merged 2026-08-24 (PR #309, commit `a016eac` by a contributor) and released in v0.10.0 (2026-08-27); Interactive Reports (artifact type 11) in v0.12.0 (2026-09-24); plan usage windows RPC `EylDcb` in v0.11.x (commit `1a30420`).

### Enterprise (Cloud/Vertex) mode in this repo
- Hosts: `notebook.cloud.google.com`, `notebooklm.cloud.google.com`, `vertexaisearch.cloud.google.com` (utils/config.py:68-74). Requires env `NOTEBOOKLM_PROJECT_ID`; `NOTEBOOKLM_LOCATION` default `global` (utils/config.py:109-120; base.py:497-503 constructor check).
- Prefix: vertexaisearch -> `/notebooklm/{loc}`, others -> `/{loc}` (base.py:207-213). batchexecute at `{base}{prefix}/_/CloudNotebookLmUi/data/batchexecute` (base.py:215-217). Streaming at `.../CloudNotebookLmUi/data/google.cloud.notebooklm.v1main.NotebookService/GenerateFreeFormStreamed` (base.py:225-227). Upload `{base}{prefix}/upload/_/` (base.py:220-223). `bl` fallback `boq_cloud-ml-notebooklm-ui_20260816.08_p0` (base.py:237). Auth is still browser cookies (same CSRF scrape from `{base}{prefix}/`, base.py:1308-1311 region), NOT Google Cloud credentials.
- List notebooks RPC `rG2vCb` params `["projects/{p}/locations/{l}", null, null, 1]` (core/notebooks.py:36-55, docs/API_REFERENCE.md:260-264). Query params `[[[[sid]]...], question, {"70000": "projects/{p}/locations/{l}/notebooks/{nb}"}]` (git show a016eac core/conversation.py hunk; docs/API_REFERENCE.md:266-275). docs say live validation was not possible (docs/API_REFERENCE.md:277-279).

## RPC catalog

Common envelope (all rows): `POST` form body `f.req=<urlencoded [[[rpcid, json.dumps(params), null, "generic"]]]>&at=<csrf>&` (base.py:685-701); URL query `rpcids`, `source-path`, `bl`, `hl`, `rt=c`, optional `f.sid` (base.py:703-718). No `_reqid` on batchexecute (only on the streamed query, conversation.py:508-520). Headers: `Content-Type: application/x-www-form-urlencoded;charset=UTF-8`, `Origin`, `Referer: {base}/`, `X-Same-Domain: 1`, `X-Goog-Csrf-Token: <csrf>`, UA string (base.py:633-661). Response: strip `)]}'` prefix, then alternating `<byte count>` / JSON lines, find `["wrb.fr", rpcid, "<json string>", ..., [errcode, ...]]` (base.py:720-765, 767-840). Error code in item[5][0]: 16 = auth expired; 8 = RESOURCE_EXHAUSTED (rate limit); 3 is overloaded ("accepted-pending" vs reject); 9 also accepted-pending for source adds. Missing expected rpcid with other ids present raises `RPCDriftError` (base.py:841-848, core/errors.py:156). Runtime hot patch: env `NOTEBOOKLM_RPC_OVERRIDES={"RPC_X":"newid"}` (base.py:97-116, 410-424).

`source-path` column: `/` default; `/notebook/{id}` when noted. `params` shown as the Python list literal (null = None).

| RPC id | Purpose | Request params | Response shape | file:line |
|---|---|---|---|---|
| wXbhsf | list notebooks | `[null,1,null,[2]]` | `[[ [title, sources[[ [sid], title, meta...]], nbid, emoji, null, [ownership(1 mine/2 shared), shared?, ..., modified@[5], created@[8]] ], ...]]` | base.py:244; core/notebooks.py:53-120 |
| rG2vCb | list notebooks (Enterprise) | `["projects/P/locations/L",null,null,1]` | same row layout as above | base.py:245; notebooks.py:41-51 |
| rLM1Ne | get notebook (+ sources) | `[nbid,null,[2],null,0]`, path `/notebook/{id}` | `[[title, sources, nbid, ...]]`; sources `[[sid], title, meta, [null,status]]` | notebooks.py:146-150; conversation.py:583-611 |
| CCqFvf | create notebook | `[title,null,null,[2],[1,null x9,[1]]]` | `result[2]` = new nbid | notebooks.py:185-194 |
| s0tc2d | rename notebook / also chat config | rename: `[nbid,[[null,null,null,[null,title]]]]`; chat config: `[nbid,[[null x7, [[goal],[length]]]]]` goal 1 default/2 custom+prompt/3 learning guide; length 1/4 longer/5 shorter | notebook row; chat settings at `result[7]` | notebooks.py:207-209, 222-246; constants.py:86-108 |
| WWINqb | delete notebook | `[[nbid],[2]]` | null on success | notebooks.py:270-272 |
| VfAZjd | notebook summary + suggested topics | `[nbid,[2]]`, path nb | summary `r[0][0]`, topics `r[1][0]` = `[question,prompt]` | notebooks.py:155-170 |
| izAoDd | add URL / YouTube / text / Drive source (legacy, one RPC for all) | URL: `[[[null,null,[url],null x6,1]],nbid,[2],[1,null x9,[1]]]`; YouTube puts `[url]` at index 7; text: source_data `[null,[title,text],null,2,null x7?,1]`; Drive: `[[docid,mime,1,title],null x9,1]` | `[[ [[sid], title, ...] ]]` | sources.py:485-506, 612-632, 695-720, 770-800 |
| ozz5Z | add URL source (new rollout, issue #121) and ALSO entitlement tier (dual-mapped) | add: `[[ [[null,url,627],[null x9,[null,null,1]],1] ]]` (notebook id only in source-path); entitlement: `[[[[null,"1",627],[null x9,[null,null,2]],1]]]` | add: same as izAoDd; entitlement: tier string `NOTEBOOKLM_TIER_*` found by tree search | sources.py:508-529, 634-649; usage.py:76-85, `_find_tier` |
| o4cbdc | register local file (step 1 of upload) | `[[filename]],nbid,[2],[1,null x9,[1]]` timeout 60 | nested list, first string = SOURCE_ID | sources.py:825-864 |
| (HTTP) upload start | resumable upload start | `POST {base}/upload/_/?authuser=0`, headers `x-goog-upload-command: start`, `x-goog-upload-protocol: resumable`, `x-goog-upload-header-content-length`, body JSON `{"PROJECT_ID":nbid,"SOURCE_NAME":fn,"SOURCE_ID":sid}`; reply header `x-goog-upload-url` | - | sources.py:866-927 |
| (HTTP) upload finalize | stream bytes | `POST <x-goog-upload-url>` headers `x-goog-upload-command: upload, finalize`, `x-goog-upload-offset: 0`, 64 KiB chunks | - | sources.py:929-967 |
| hizoJc | get source (raw fulltext) | `[[sid],[2],[2]]`, path `/` | `r[0]=[[sid],title,meta]`, content blocks at `r[3][0]` (recursive string collect); meta[4] = type code, URL at meta[7][0] (YouTube meta[5][0]) | sources.py:1068-1137, 52-65 |
| tr032e | source guide (summary + keywords) | `[[[[sid]]]]` | summary `r[0][0][1][0]`, keywords `r[0][0][2][0]` | sources.py:1046-1066 |
| tGMBJ | delete source(s) | `[[[sid],...],[2]]` | null | sources.py:299-321 |
| b7Wfje | rename source | `[null,[sid],[[[title]]]]`, path nb | - | sources.py:273-277 |
| yR9Yof | Drive freshness check | `[null,[sid],[2]]` | bool-like | sources.py:216-228 |
| FLmJqe | Drive sync | `[null,[sid],[2]]` | source row; sync time meta[3][1][0] | sources.py:231-258 |
| hPTbtc | get server conversation id | `[[],null,nbid,20]`, path nb | `[[[conv_id]]]` | conversation.py:203-243 |
| khqZz | conversation turns | `[[2,null,[1],[1,null x9,[1,3]]],null,null,convid,limit]` | `[[turn...],cont]`, turns newest first: answer `[id,[s,ns],2,null,[[text,..]]]`, query `[id,[s,ns],1,text]` | conversation.py:245-329 |
| J7Gthc | delete chat history | `[nbid,convid]` | - | conversation.py:331-353 |
| Ljjv0c | start FAST research | `[[query,srcType(1 web/2 drive)],null,1,nbid]` | `[task_id, ...]` | research.py:66-73 |
| QA9ei | start DEEP research (web only) | `[null,[1],[query,1],5,nbid]` | `[task_id,...]` | research.py:69-73 |
| e3bVqc | poll research | `[null,null,nbid]` | list of `[task_id,[?,[query,srcType],mode(5=deep),[sources,summary],status(1 prog/2,6 done)]]`; deep report in source[6][0] | research.py:107-190 |
| LBwxtb | import research results | `[null,[1],task_id,nbid,[source_data...]]` web item `[null,null,[url,title],null x7,2]`; on code 3/9 reconcile by diffing notebook sources | list of `[[sid],title]` | research.py:340-425 |
| R7cb6c | create studio artifact (audio, video, report, quiz, flashcards, infographic, slide deck, data table, interactive report) | envelope `[[2],nbid,[null,null,TYPE,[[[sid]]...],null...,OPTIONS@k]]`; TYPE 1 audio(options@6 `[null,[focus,len,null,[[sid]],lang,null,format]]`), 2 report(@7), 3 video(@8), 4 flashcards/quiz(@9), 7 infographic(@14), 8 slides(@16 `[[focus,lang,format,length]]`), 9 data table(@18 `[null,[desc,lang]]`), 11 interactive report (options@34) | `[[artifact_id, title, type, ..., status]]` | studio.py:229-285, 287-375, 767-845, 846-913, 915-1031, 1033-1099, 1265-1335, 1337-1414, 1416-1487 |
| gArtLc | poll all studio artifacts | `[[2],nbid,'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | `[[artifact...]]`; art[0]=id,[1]=title,[2]=type,[4]=status(1 prog,2 queued,3 done,4 failed,5 suggested); audio meta[6][5] media list; video [8][3]; infographic [14][2][0][1][0]; slide [16][0]/[3]; report [7][1][0]; data-table-xlsx [24] | studio.py:377-632, 178-226 |
| v9rmvd | get one artifact / interactive HTML (quiz/flashcards) | HTML: `[artifact_id]`; full: `[artifact_id,[2,null,null,[1,null x9,[1]],[[1,4,8,10,14,2,3,6,7]]]]` | HTML string at `r[0][9][0]` | core/download.py:1290-1346; studio.py:1100-1122 |
| V5N4be | delete studio artifact | `[[2],artifact_id]` | - | studio.py:638-665 |
| rc3d8d | rename artifact / set artifact fields | `[[artifact_id,new_title],[["title"]]]` | - | studio.py:703-721, 1212 |
| Rytqqe | kick off generation of a suggested artifact (interactive report elements) | see studio.py | - | studio.py:1157-1263 |
| KmcKPe | revise slide deck | `[[2],artifact_id,[[ [slideIdx0, instruction], ...]]]` | new artifact row | studio.py:723-765 |
| yyryJe | generate mind map JSON | `[[[sid]]...,null,null,null,null,["interactive_mindmap",[["[CONTEXT]",""]],""],null,[2,null,[1]]]` | `[[json_string,null,[gen_ids]]]` | studio.py:1489-1561 |
| CYK0Xb | save mind map / create note (same rpc) | mind map: `[nbid,json,[2,null,null,5,[[sid]...]],null,title]`; note: `[nbid,"",[1],null,title]` | `[[id,json,meta,null,title]]` | studio.py:1563-1617; notes.py:42-43 |
| cFji9 | list mind maps / list notes | `[nbid]` | `[[ [id, details|null] ]]`, tombstone `[uuid,null,2]` | studio.py:1619-1660; notes.py:83-84 |
| cYAfTb | update note | `[nbid,note_id,[[[content,title,[],0]]]]` | updated note | notes.py:186-192 |
| AH0mwd | delete note / delete mind map | note: `[nbid,null,[note_id]]` (soft delete) | null | notes.py:213-214; studio.py:667-701 |
| agX4Bc | labels: auto/list/create; and "collections" create/list | labels: auto `[[2],nbid,null,null,[]]`, create `[[2],nbid,null,null,null,[[name,emoji]]]`; collections: `[hdr,null,null,null,null,[[name],null,[nbids]],3]` with hdr `[2,null,[1],[1,null x9,[1,3]]]` | collections list is `r[2]` = `[name,[nbids],id,emoji]` | labels.py:45-66; collections.py:34-86 |
| le8sX | label rename/emoji/move source; collection edit/emoji | label rename `[[2],nbid,label_id,[[[name]]]]`; emoji `[[2],nbid,label_id,[[[null,emoji]]]]`; move `[[2],nbid,label_id,[[null,[[sid]]]]]`; collection edit `[hdr,null,cid,[[null,null,null,[[nbids]]],[[name]]],3]` | `[]` ok | labels.py:71-84; collections.py:126-133 |
| GyzE7e | delete label(s) / delete collection | label `[[2],nbid,[label_ids]]`; collection `[hdr,null,[cid],3]` | `[]` | labels.py:89-90; collections.py:140 |
| QDyure | sharing: set public / invite | public `[[[nbid,null,[0|1],[0,""]]],1,null,[2]]`; invite `[[[nbid,[[email,null,role(2 editor/3 viewer)]],null,[notifyFlag(0 notify),msg]]],1,null,[2]]` | - | sharing.py:109-111, 145-152, 194-196 |
| JFMDGd | share status | `[nbid,[2]]` | collaborators, public flag | sharing.py:32-33 |
| Krh3pd | export artifact to Google Docs/Sheets | `[null,artifact_id,content|null,title,1(docs)/2(sheets)]` | doc URL | exports.py:53-58 |
| EylDcb | plan usage windows | `[[2,null,[1],[1,null x9,[1,3]]]]` | `r[1]` = entries `[..,[4]=window type,[5]=[reset epoch],[6]=pct used,[7]=pct remaining]`, unordered | usage.py:42-69 |
| hT54vc, ZwVcOc | preferences/settings (declared, not called in code I read) | - | - | base.py:265-266 [unverified usage] |

Authenticated page fetch (CSRF/session/bl): `GET {base}/` (Enterprise `{base}/{prefix}/`) with browser-like headers (Sec-Fetch-*, generic Linux Chrome UA, deliberately no sec-ch-ua, base.py:426-441), redirects followed. If final URL contains `accounts.google.com` => auth expired (base.py:1290-1294). Regex: CSRF `"SNlM0e":"..."` (utils/cdp.py:1429-1433; core/auth.extract_csrf_from_page_source), session `"FdrFJe":"(\d+)"` (utils/cdp.py:1435-1445; base.py:1327), build label `"cfb2h":"..."` (base.py:1329-1330; cdp.py:1418-1426), email `"oPEP7c":"..@.."` (cdp.py:1448-1463). Debug dump of the page on CSRF failure at `get_storage_dir()/debug_page.html` created 0600 (base.py:1309-1325).

Cookie jar quirk: cookies duplicated onto `.googleusercontent.com` for artifact downloads that redirect (base.py:559-590). On artifact downloads the OSID / `__Secure-OSID` cookies are DELETED before the request or Google redirects to ServiceLogin (core/download.py:195-205); headers must mimic cross-site navigation (`Sec-Fetch-Site: cross-site`, Referer = host) or the audio CDN returns 403 (download.py:177-193).

## Streaming/chat

- Endpoint: `POST {base}{QUERY_ENDPOINT}?bl=<bl>&hl=<hl>&_reqid=<n>&rt=c[&f.sid=<sid>]` (conversation.py:508-545, base.py:366-367). `_reqid` starts random 100000-999999 and `+= 100000` per call under a lock (base.py:489-491; conversation.py:503-506). Body: `f.req=<urlencode([null, json(params)])>&at=<csrf>&`. Content-Type header is mandatory ("stricter than batchexecute", conversation.py:521-523).
- Params (consumer): `[ [[[sid]]...], query, history|null, [2,null,[1]], conversation_id ]`. History for follow-ups is `[[answer,null,2],[query,null,1],...]` oldest first (conversation.py:71-105, 448-480 area). Sources default to all sources fetched via rLM1Ne (conversation.py:403-419).
- Conversation id: reuse the notebook's persistent server conversation from `hPTbtc` (so chats show in the web UI) unless `new_conversation`; otherwise random uuid4 (conversation.py:424-442). Server may return a new id inside the answer chunk: `first_elem[2][0]`; local cache is migrated to it (conversation.py:556-580).
- Response is read in full (`response.text`, not iterated as a stream) then parsed line by line (conversation.py:549-555, 614-693). Chunk layout: `[["wrb.fr",null,"<json>"...]]`; inner `[[text, null, [conv_id, hash, ts], null, [segments, null, null, passages, typeCode]]]`; typeCode 1 = answer, 2 = thinking; the LONGEST type-1 text wins, thinking only as fallback (conversation.py:614-693, 744-820). That is not true incremental streaming: no token deltas are surfaced. A Go server could surface progress by parsing chunks as they arrive.
- Citations: `type_info[3]` passages; citation N (1-indexed) maps to `passage[1][5][0][0][0]` source id; cited text from `passage[1][4]` nested `[start,end,text]` triples, tables extracted separately as `cited_table` (conversation.py:979-1053, 822-978).
- Errors: chunk with `item[2]==null` and `item[5]=[code,...]` collected; if no answer text, raise `QueryRejectedError(code,type,raw)` (conversation.py:695-742).
- Budget: default 120 s wall clock `DEFAULT_QUERY_TIMEOUT` (conversation.py:23), env `NOTEBOOKLM_QUERY_TIMEOUT`; async variant `notebook_query_start/status` with max 8 in-flight (`NOTEBOOKLM_ASYNC_QUERY_MAX_INFLIGHT`, services/chat.py:205-219).
- Local cache is an LRU OrderedDict: 50 turns/conversation, 500 conversations, 100000 chars/turn (base.py:469-486).

## Auth lifecycle

- Cookie source: (a) `nlm login` launches a dedicated Chromium-family browser profile with `--remote-debugging-port`, `--user-data-dir=<per nlm profile>`, `--remote-allow-origins=http://127.0.0.1:{port}`, `--no-first-run`, `--disable-extensions` (utils/cdp.py:960-1010), connects over WebSocket and calls `Network.getAllCookies`, `Runtime.evaluate` for page HTML (cdp.py:1332-1352), waits up to 300 s for a real sign-in (cdp.py:1534-1690, `is_logged_in` at 1401-1415). Port scan range 9222-9231 (cdp.py:97-98). Browser candidates: Chrome, Arc, Dia, Comet, Brave, Edge/Edge Beta, Chromium, Vivaldi, Opera, Opera GX (cdp.py:366-433). Snap-aware profile dir (cdp.py:567-633). (b) `nlm login --manual` / MCP `save_auth_tokens` with pasted Cookie header (mcp/tools/auth.py:136). (c) env `NOTEBOOKLM_COOKIES` overrides everything (base.py:487-492; error text base.py:1193-1199). (d) Existing CDP endpoint (`extract_cookies_via_existing_cdp`, cdp.py:1692).
- Required cookies: `SID, HSID, SSID, APISID, SAPISID` (core/auth.py:390-403). No SAPISIDHASH needed; auth is cookie + `at` CSRF only.
- Storage: per-profile dir with `cookies.json`/`metadata.json` (0600) or "protected" mode `credentials.enc` AES-256-GCM with a key in the OS keystore via `keyring` (core/credential_store.py:1-33, v0.14.0 2026-09-30). Revision-based compare-and-save across processes (StaleRevisionError, base.py:1338-1370). Multi-account: named profiles, `profile` MCP tool switches the active one for the server lifetime (mcp/tools/profiles.py).
- Expiry detection: HTTP 400/401/403 or RPC error code 16 -> recovery ladder (base.py:1141-1190): L1 `_refresh_auth_tokens` (POST `https://accounts.google.com/RotateCookies` body `[000,"-0000000000000000000"]` JSON, rate limited 60 s, then GET homepage and re-scrape CSRF; core/cookie_rotation.py:26-31, 80-137); L2 reload cookies from disk if they differ; L3 headless Chrome on the saved profile (`run_headless_auth`, port 9223, utils/cdp.py:1925+; env `NOTEBOOKLM_DISABLE_HEADLESS_REFRESH=1` disables). Finding from issue #316: RotateCookies from plain httpx refreshes `*SIDCC` only, NOT `*PSIDTS`; aged-out sessions need a real browser (core/cookie_rotation.py:1-11; CHANGELOG.md:451). Non-live "good enough" heuristic is 168 h since last validation (core/auth.py:1099-1110).
- Transient vs auth: network/5xx during recovery is mapped to `TransientBackendError` instead of "auth expired" (base.py:1150-1176, `_is_unreachable_failure` base.py:147-177).

## MCP tools (53; tool_groups.py:34-125 lists every name)

FastMCP, stdio default; `--transport http|sse`, `--host` (loopback only unless `NOTEBOOKLM_ALLOW_EXTERNAL_BIND`), `--port 8000`, `--path /mcp`, stateless HTTP default true, `/healthz` and health route (mcp/server.py:80-92, 136-300). Hide groups via `NOTEBOOKLM_DISABLED_GROUPS`, `NOTEBOOKLM_DISABLED_TOOLS`, `NOTEBOOKLM_ENABLED_TOOLS` (tool_groups.py:1-20, 128-144). Destructive tools require `confirm=true`.

| Tool | Params | Behavior |
|---|---|---|
| notebook_list | max_results=100 | list (wXbhsf) |
| notebook_get | notebook_id | details + sources (rLM1Ne) |
| notebook_describe | notebook_id | AI summary + suggested topics (VfAZjd) |
| notebook_create / rename / delete | title / notebook_id,new_title / notebook_id,confirm | CCqFvf / s0tc2d / WWINqb |
| source_add | notebook_id, source_type(url\|text\|drive\|file), url, urls[], text, title, file_path, document_id, doc_type(doc\|slides\|sheets\|pdf), wait, wait_timeout=120 | unified add; 43 allowed file extensions (core/constants.py:154-232) |
| source_list_drive | notebook_id, skip_freshness | types + Drive staleness (yR9Yof) |
| source_sync_drive | source_ids, confirm | FLmJqe |
| source_rename / source_delete | notebook_id,source_id,new_title / source_id(s),confirm | b7Wfje / tGMBJ |
| source_describe | source_id | summary + keywords (tr032e) |
| source_get_content | source_id, wait, wait_timeout, poll_interval=3 | raw text (hizoJc) |
| notebook_query | notebook_id, query, source_ids, conversation_id, timeout, new_conversation | streamed chat (async tool) |
| notebook_query_start / notebook_query_status | same / query_id | background query + poll |
| chat_configure | notebook_id, goal(default\|learning_guide\|custom), custom_prompt(<=10000), response_length | s0tc2d |
| chat_list / chat_get / chat_export / chat_save_to_note | notebook_id, conversation_id, limit / format md\|json / turn,title | server-side history (khqZz) |
| cross_notebook_query | query, notebook_names, tags, all | fan-out + aggregate with per-notebook citations |
| research_start | query, source(web\|drive), mode(fast\|deep), notebook_id, title | Ljjv0c / QA9ei |
| research_status | notebook_id, poll_interval=30, max_wait=900, compact, task_id, query, auto_import | e3bVqc blocking poll |
| research_import | notebook_id, task_id, source_indices, timeout=300, cited_only | LBwxtb |
| studio_create | notebook_id, artifact_type(audio\|video\|infographic\|slide_deck\|report\|flashcards\|quiz\|data_table\|mind_map), source_ids, confirm, + type options (audio_format deep_dive\|brief\|critique\|debate, audio_length short\|default\|long, video_format explainer\|brief\|cinematic\|short, visual_style 9 values, orientation, detail_level, infographic_style 11 values, slide_format, slide_length, report_format Briefing Doc\|Study Guide\|Blog Post\|Create Your Own\|Interactive, report_template, custom_prompt, question_count, difficulty, language, focus_prompt, title, description) | R7cb6c |
| studio_status | notebook_id, action(status\|rename\|list_types), artifact_id, new_title, include_details, limit=20, offset | gArtLc, rc3d8d |
| studio_delete / studio_revise | notebook_id,artifact_id,confirm / slide_instructions[{slide,instruction}] | V5N4be / KmcKPe |
| report | notebook_id, artifact_id, action(get\|elements\|generate), wait_for, timeout=600, include_content, plan[], language, confirm | interactive report elements (v9rmvd, Rytqqe) |
| download_artifact | notebook_id, artifact_type (audio\|video\|report\|mind_map\|slide_deck\|infographic\|data_table\|data_table_xlsx\|file\|quiz\|flashcards), output_path, artifact_id, output_format json\|markdown\|html, slide_deck_format pdf\|pptx, wait, wait_timeout=180, poll_interval=5 | downloads confined to a directory (default `~/Downloads/gemini-notebook`, env `NOTEBOOKLM_DOWNLOAD_DIR`) |
| download_all_artifacts | notebook_id, output_dir, artifact_types, output_format, slide_deck_format, all_notebooks, skip_existing | bulk, incremental |
| export_artifact | notebook_id, artifact_id, export_type(docs\|sheets), title | Krh3pd |
| note | notebook_id, action(create\|list\|update\|delete), note_id, content, title, confirm | CYK0Xb/cFji9/cYAfTb/AH0mwd |
| label | notebook_id, action(auto\|list\|reorganize\|create\|rename\|set_emoji\|move_source\|delete), label_id(s), name, emoji, source_id, unlabeled_only, confirm | agX4Bc/le8sX/GyzE7e |
| collection_list/create/edit/set_emoji/delete | name, notebook_ids, collection_id, emoji, confirm | agX4Bc/le8sX/GyzE7e (list is done by creating then deleting a temp collection, collections.py:25-60) |
| notebook_share_status / _public / _invite / _batch | notebook_id; is_public; email, role viewer\|editor; recipients[], confirm | JFMDGd / QDyure |
| tag | action(add\|remove\|list\|select), notebook_id, tags, notebook_title, query | local JSON tag store for smart select (client-side only) |
| alias | action(list\|get\|set\|delete), name, value, alias_type | local name -> id aliases |
| batch | action(query\|add_source\|create\|delete\|studio), query, source_url, titles, artifact_type, notebook_names, tags, all, confirm | multi-notebook fan-out (client-side) |
| pipeline | action(run\|list\|create), notebook_id, pipeline_name, input_url, description, steps | client-side workflows |
| profile | action(list\|status\|switch), name, make_default | multi-account |
| refresh_auth / save_auth_tokens | - / cookies, csrf_token, session_id, request_body, request_url | auth recovery |
| server_info | - | version, auth_status, update check against PyPI, capabilities |
| usage_get | profile | EylDcb usage windows (rolling + weekly) |

CLI `nlm` (cli/main.py:1394-1456): noun-first and verb-first command trees: login (+profile), auth (+storage, refresh), notebook, source, chat/chats, studio and per-type apps (audio, report, quiz, flashcards, mindmap, slides, infographic, video, data-table), research, download, share, export, label, note, alias, config, skill, setup, doctor (incl. auth-replay), batch, cross, pipeline, tag, usage, plus verbs create/list/get/delete/add/rename/status/describe/query/sync/content/stale/configure/set/show/install/uninstall/update.

## Reliability

- Retries: HTTP 429/500/502/503/504 with exponential backoff `min(1*2^n, 16)` up to 3 retries; connect errors only (read/write timeouts are NOT retried to avoid duplicate mutations) (core/retry.py:18-34, 36-41; base.py:1070-1140). Rate-limit ceiling env `NOTEBOOKLM_RATE_LIMIT_MAX_RETRIES` (base.py:70-94). RPC code 8 retried with same backoff. `retry_server_errors=False` for non-idempotent mutations (base.py:970-990).
- "Accepted-pending" code 3/9 on add-source/import: reconcile by listing notebook sources and matching URL/Drive id before retrying, to avoid double-add (sources.py:431-470, 548-612; research.py:380-405).
- Timeouts: default 30 s, source add 120 s, file register 60 s, upload 300 s, query 120 s (base.py:143-144; sources.py:891-960).
- Caching: server version check cached; auth health 30 s TTL; conversation LRU (above).
- Threading: sync httpx client behind `threading.Lock` shared by FastMCP's worker threads; lock never held during I/O (base.py:513-519).

## Performance notes

- Python sync httpx; one shared `httpx.Client` (connection reuse), but a brand new `httpx.Client` per streamed query and per upload step (conversation.py:538-545; sources.py:915, 962). `notebook_query` calls rLM1Ne (get notebook) + hPTbtc (conversation id) before every first query, i.e. 2 extra round trips (conversation.py:403-442): cacheable in Go.
- Page scrape (`GET /`) costs one RTT on client creation unless tokens are cached from disk (base.py:535-542, 1244-1360).

## Tests and quality

tests/ present (about 250 files not enumerated here); CI matrix includes Windows since v0.15.4 (.github/workflows/lint-test.yml). `release-gate.yml` and `version-check.yml` exist. Tool-group coverage test ensures every tool is in a group (CHANGELOG.md:695).

## Versions

163 tags; selected: v0.1.0 2026-01-01; v0.3.0 2026-02-13 (service layer refactor); v0.4.0 2026-03-05; v0.6.0 2026-04-27 (source labels); v0.8.0 2026-06-30; v0.9.0 2026-07-20; v0.9.3 2026-07-25 (notebook.google.com host, rebrand fix); v0.9.4 2026-07-25 (notebook.cloud.google.com); v0.10.0 2026-08-27 (Enterprise); v0.11.0 2026-09-07 (non-interactive refresh `nlm auth refresh`, fix for 8-hour session death #316); v0.12.0 2026-09-24 (Interactive Reports); v0.13.0 2026-09-27 (setup wizard); v0.14.0 2026-09-30 (protected credential storage); v0.15.0 2026-10-01; v0.15.4 2026-10-06 (Windows CI, ephemeral CDP port, atomic aliases/tags, bounded query concurrency). Git log shows 948 commits; most recent 60 are community PRs (insane66613 contributed twelve in 0.15.4, CHANGELOG.md:10-43).

## Adopt (with evidence)

1. batchexecute envelope + parsing incl. `RPCDriftError` and env hot-patch of RPC ids (base.py:97-116, 767-848). Implement as a config file/env map in Go.
2. Four-layer auth recovery: refresh tokens -> reload disk -> headless browser; and treat network/5xx separately from auth expiry (base.py:1141-1190, 147-177).
3. Revision-based compare-and-save credential store, optional OS-keystore encrypted mode (core/credential_store.py).
4. Accepted-pending reconciliation for add-source and research import (sources.py:431-470; research.py:380-405).
5. Host abstraction: `base_host` learned at login and persisted per profile; allowlist of hosts (utils/config.py:68-106).
6. Download hygiene: drop OSID cookies, cross-site Sec-Fetch headers, prefer `-dv` audio variant, confine writes to an allowed dir (download.py:177-205, 392-425; MCP downloads dir).
7. Tool gating by group for context savings (tool_groups.py) and `confirm` flags on destructive tools.
8. Source status polling rules: status 3 is a hard failure only for known non-audio types (sources.py:152-215).
9. Rate-limit-aware retry rules: never retry read timeouts on mutations (retry.py:36-41).
10. In-page CDP fetch transport as the browser fallback behind the same interface (cdp_transport.py:1-8).

## Avoid (with evidence)

1. Do not copy the 53-tool surface literally: many tools are client-side orchestration (batch, pipeline, tag, alias, cross_notebook_query) that bloat the tool list. Offer as optional groups.
2. `collection_list` creates and deletes a temporary collection to force Google to return the list (collections.py:25-60); race and mutation risk. Prefer reading collection data from another read RPC if one exists [unverified].
3. Response is fully buffered then "longest type-1 chunk wins" (conversation.py:549-693): hides partial progress and risks truncated answers on long streams; parse incrementally.
4. Two extra RPCs before every first query (conversation.py:403-442) and per-request fresh httpx clients (conversation.py:538; sources.py:915) cost latency.
5. A single global sync client under a lock (base.py:513-519); in Go use per-profile client with per-request contexts.
6. Dual-mapped RPC `ozz5Z` (add source + entitlement) and `izAoDd` (url/text/drive) have overloaded semantics and error code 3 is ambiguous (sources.py:421-432): wrap them in explicit typed calls.
7. Magic constants copied from captures (627, "1", 1469, element type lists constants.py:285-291) with unknown meaning; keep them in a data table with provenance.
8. Enterprise path is based on a contributor capture and flagged as not live-validated (docs/API_REFERENCE.md:277-279): do not rely on it.

## Evidence

Read in full or in the relevant ranges: core/base.py (1-1501), core/constants.py (1-551), core/notebooks.py, core/sources.py (403-1147), core/conversation.py (60-1053), core/research.py (55-425), core/studio.py (178-1660 selected), core/download.py (170-215, 392-425, 1290-1346), core/{notes,sharing,exports,usage,collections,labels}.py, core/cookie_rotation.py, core/cdp_transport.py (1-60), core/retry.py, utils/cdp.py (selected), utils/config.py (60-145), mcp/tool_groups.py, mcp/server.py, mcp/tools/*.py signatures, CHANGELOG.md entries 0.9.3, 0.10.0, 0.11.0, 0.12.0, 0.14.0, 0.15.0, 0.15.4; git show 5c772d1, ed21a32, a016eac, a66a33d, 74d3575. No code from the clone was executed (only Python `ast` parsing of tool signatures via `python -I -X utf8`, and git/grep).
