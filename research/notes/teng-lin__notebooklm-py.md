# teng-lin/notebooklm-py

- URL: https://github.com/teng-lin/notebooklm-py (PyPI `notebooklm-py`, aliases `gemini-notebook`, `notebooklm`)
- Language / runtime: Python >=3.10 (3.15 supported), async; httpx default transport, optional curl_cffi TLS impersonation (`pyproject.toml` extras, `docs/configuration.md:211`)
- License: MIT, "Copyright (c) 2026 Teng Lin" (`LICENSE:1-3`). Short snippets (<15 lines) allowed; the recovered Android `schema.proto` is a decompilation of Google's app, treat as a legal gray area before vendoring [unverified].
- Stars / last commit: 19.7k stars (research/upstreams.md); HEAD `8692ff92` 2026-10-13; 2552 commits since 2026-01-05 (`git log`).
- Versions reviewed: v0.8.5 (2026-10-12, `pyproject.toml:3`), clone HEAD 8692ff92. Read-only; nothing installed or run.
- Dates in this repo are on a 2026 timeline (first commit 2026-01-05); product is now branded "Gemini Notebook" (ADR-0028, `docs/adr/0028-gemini-notebook-rename.md`).

## Access method

Three tiers, all on the same server-side service `google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService`:

1. **Web `batchexecute`** (default). POST form to `https://notebook.google.com/_/LabsTailwindUi/data/batchexecute` (`src/notebooklm/_env.py:20`, `src/notebooklm/rpc/types.py:99-101`). Legacy host `notebooklm.google.com` is still served and selectable via `NOTEBOOKLM_BASE_URL` (`_env.py:30-45`, allow-list `_env.py:66-95`). Enterprise hosts `notebook.cloud.google.com` / `notebooklm.cloud.google.com` also allowed (`_env.py:23-25`). 61 RPC ids in `RPCMethod` (`src/notebooklm/rpc/_identifiers.py:6-171`).
2. **Web streamed chat**: separate non-batchexecute endpoint `.../data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed` (`rpc/types.py:84-95`).
3. **Android native gRPC** (explicit `--backend android`, since v0.8.x): HTTP/2 gRPC to `notebooklm-pa.googleapis.com:443` (`src/notebooklm/_android/session.py:62`), path `/<service>/<Method>` with plaintext method names, protobuf bodies, OAuth bearer minted from a Google master token via `gpsoauth`. Full protobuf schema (326 messages / 879 fields) recovered from the Android app and checked in at `docs/android/schema.proto` (2060 lines); 53 methods in app build 1.55.10, 49 in 1.46.7 (`docs/android/endpoints.md:28-70`). The same backend serves both front doors: web JSON index `i` == proto tag `i+1` (`docs/android/README.md`, "Why this exists"). This is the single most valuable asset for a Go server: typed protos instead of positional arrays.

Not browser automation. Playwright is only used by `notebooklm login` and optional headless re-auth (`pyproject.toml` `browser` extra).

Wire basics for Web batchexecute:

- URL: `{base}/_/LabsTailwindUi/data/batchexecute?rpcids=<id>&source-path=<path>&f.sid=<FdrFJe>&hl=<lang>&rt=c[&authuser=<n|email>]` (`src/notebooklm/_web/transport/executor.py:606-623`). `source-path` is `/notebook/<notebook_id>` for notebook-scoped calls and `/` for account-level calls (`docs/rpc-reference.md:2944-2949`). `bl` and `_reqid` are NOT sent on batchexecute (only on the chat stream, below).
- Headers: `Content-Type: application/x-www-form-urlencoded;charset=UTF-8` (`src/notebooklm/_web/transport/kernel.py:155`), plus Cookie jar. Upload requests add `x-goog-authuser` (see sources).
- Body: `f.req=<urlencode([[[rpc_id, json.dumps(params,compact), null, "generic"]]])>&at=<CSRF>&` (trailing `&`) (`src/notebooklm/_web/wire/encoder.py:14-47, 77-110`).
- Response: `)]}'` anti-XSSI prefix stripped, then `rt=c` chunked format: alternating decimal byte-count line and JSON line; find the `wrb.fr` frame whose id matches; `frame[2]` is the JSON-string result; null result with `frame[5]` = `google.rpc.Status` array means rejection (`src/notebooklm/_web/wire/decoder.py:272-330`, `docs/rpc-reference.md:3257-3340`). Declared byte counts can be off by 2 on live rejection bodies, so the parser must be tolerant (`rpc-reference.md:3297-3300`).
- CSRF/session tokens: GET the app homepage with cookies, regex `WIZ_global_data` for `"SNlM0e":"..."` (CSRF, sent as `at`) and `"FdrFJe":"..."` (session id, sent as `f.sid`) (`src/notebooklm/_auth/extraction.py:86-125, 470-500`). Tolerates double-quoted, single-quoted and HTML-escaped forms. A signed-out landing is token-less; v0.8.5 asks `/login` to distinguish signed-out from page drift (`CHANGELOG.md:37-60`).
- Build label `bl` default `boq_labs-tailwind-frontend_20260802.02_p0` (`_env.py:123`); the server does NOT validate it: a fabricated `..._19700101.00_p0` still returned a full cited answer (`_env.py:113-122`). Env override `NOTEBOOKLM_BL`. RPC ids can be remapped at runtime via `NOTEBOOKLM_RPC_OVERRIDES` (`docs/configuration.md:217`, `encoder.py:28-33`).
- `hl` language from `NOTEBOOKLM_HL`; `authuser` is the Google multi-account index or email.

## RPC catalog

Common wrapper reused in most request payloads ("OPTS" / request-options): `[2, null, null, [1, null,null,null,null,null,null,null,null,null, [1]]]` (`docs/rpc-reference.md:317-326`). Artifact creation adds a fifth element `[[1, 4, 8, 2, 3, 6]]` (artifact-type capability list) (`rpc-reference.md:1375-1395`). All payloads below are the JSON `params` before encoding. Source-id nesting: `[id]`, `[[id]]`, `[[[id]]]`, `[[[[id]]]]` differ per method (`rpc-reference.md:2101-2135`, `encoder.py:50-74`).

Server method names come from Google's own id registry per the comments in `_identifiers.py`.

| RPC id | Server method (enum name) | Purpose | Request payload shape | Response shape | file:line |
|---|---|---|---|---|---|
| `wXbhsf` | ListRecentlyViewedProjects (LIST_NOTEBOOKS) | List notebooks, recency ordered | `[null,1,null,[2]]` accepted; UI sends `[null,1,null,[2,null,null,[1,..,[1]]],null,[[null,null,[]],[[]],[null,[]]]]` | array of project rows | `_identifiers.py:30`, `rpc-reference.md:304-326` |
| `CCqFvf` | CreateProject (CREATE_NOTEBOOK) | Create notebook | `[title,null,null,OPTS]` | project row (id at `[2]`) | `_identifiers.py:31`, `rpc-reference.md:328-340` |
| `te3DCe` | CopyProject (COPY_NOTEBOOK) | Copy notebook incl. sources and artifacts | `[OPTS, source_notebook_id, dest_title]` | bare project row; no idempotency token so never auto-retried | `_identifiers.py:32`, `rpc-reference.md:342-362` |
| `rLM1Ne` | GetProject (GET_NOTEBOOK) | Notebook + sources + chat settings | `[notebook_id,null,OPTS,null,0]` | project row; sources list; chat settings at `[7]` `[[goal,prompt?],[len]]` | `_identifiers.py:33`, `rpc-reference.md:375-403` |
| `s0tc2d` | MutateProject (RENAME_NOTEBOOK) | Generic notebook mutator: rename, chat config, view level | rename `[nb_id,[[null,null,null,[null,title]]]]`; chat config `[nb_id,[[null,null,null,null,null,null,null,[[goal],[len]]]]]` (goal 1 default/2 custom `[2,prompt]`/3 learning guide; len 1 default/4 longer/5 shorter); view level `[nb_id,[[null,null,null,null,null,null,null,null,[[0|1]]]]]` (0 full, 1 chat only) | project row | `_identifiers.py:34`, `rpc-reference.md:1055-1092, 2357-2385` |
| `WWINqb` | DeleteProjects (DELETE_NOTEBOOK) | Delete notebook | `[[notebook_id],[2]]` (single id; batch shapes rejected) | empty | `_identifiers.py:35`, `rpc-reference.md:364-373` |
| `fejl7e` | RemoveRecentlyViewedProject | Remove from recents | `[notebook_id]` | none; server may answer INTERNAL(13) which the client treats as success | `_identifiers.py:160`, `rpc-reference.md:405-424, 3282` |
| `izAoDd` | AddSources (ADD_SOURCE) | Add URL / text / YouTube / Drive; batch of specs allowed | each spec is an 11-slot list. URL: `[[null,null,[url],null,null,null,null,null,null,null,1]]`; text: `[[null,[title,content],null,2,null,...,1]]`; YouTube: URL at slot 7 `[[null,null,null,null,null,null,null,[url],null,null,1]]`; params = `[[specs...], notebook_id, OPTS]`. Drive: `[[ [file_id,mime,1,title], null x9, 1 ]], notebook_id, [2], [1,null,...,[1]]` (older tail, pending recapture) | `[sourceRows...]`; per-item not atomic, failures silently omitted unless all fail | `_identifiers.py:40`, `rpc-reference.md:515-601` |
| `o4cbdc` | AddTentativeSources (?) (ADD_SOURCE_FILE) | Register file for upload, returns SOURCE_ID | `[[filename]],notebook_id,OPTS` wrapped as `[[[filename]],notebook_id,OPTS]`; call with `source-path=/notebook/<id>`, never retried | SOURCE_ID string; then Scotty upload (see below) | `_identifiers.py:46`, `rpc-reference.md:603-666` |
| `X1snv` | AddSourcesAsync | Queue URL sources, returns stubs immediately (~0.65s vs ~2s/source) | same as `izAoDd`: `[[specs], notebook_id, OPTS]` | `[[Source...], null, [[Source, ack_int]...]]` | `_identifiers.py:59`, `rpc-reference.md:3407-3428` |
| `tGMBJ` | DeleteSources | Delete source(s) | `[[[source_id]]]` with `source-path=/notebook/<id>` (notebook id NOT in params) | empty | `_identifiers.py:47`, `rpc-reference.md:668-683` |
| `hizoJc` | LoadSource (GET_SOURCE) | Source fulltext | `[[source_id],[2],[2]]` plain text; `[[source_id],[3],[3]]` markdown/HTML | content blocks | `_identifiers.py:48`, `rpc-reference.md:707-739` |
| `b7Wfje` | MutateSource (UPDATE_SOURCE) | Rename source | `[null,[source_id],[[[new_title]]]]` | row | `_identifiers.py:51`, `rpc-reference.md:685-696` |
| `FLmJqe` | RefreshSource | Re-ingest URL/Drive source | `[null,[source_id],[2]]`, `allow_null`, status-tagged null `[3]` = INVALID_ARGUMENT | null on success | `_identifiers.py:49`, `rpc-reference.md:2441-2468` |
| `yR9Yof` | CheckSourceFreshness | Stale check | `[null,[source_id],[2]]` | URL: `[]` fresh; Drive `[[null,true|false,[id]]]` | `_identifiers.py:50`, `rpc-reference.md:2470-2495` |
| `QsNTEd` | AppendSource | Append text block to a source | `[null,[source_id],null,[null,[header,body]]]` | empty (use raise_on_null_status) | `_identifiers.py:61`, `rpc-reference.md:3430-3451` |
| `R27wvc` | CopySourcesAsync | Copy sources to another notebook | `[null,null,[[sid],...],target_nb_id]` | `[[ [[orig_id], sourceEntry]... ]]` | `_identifiers.py:65`, `rpc-reference.md:3453-3469` |
| `ASU5Oe` | RetrieveRelevantChunks | Ranked passage search over sources | `[nb_id, query, null, [1], [[[sid]...]]?]` (5th optional source filter) | `[[[source_id,[chunk...]]...]]`, chunk `[[[[text_part..]]], rank, [[null,start,end]..]]` | `_identifiers.py:55`, `rpc-reference.md:741-775` |
| `mVtEUb` | ListExpertIntelligenceContent | List Play Books library (US, 18+) | built by `build_list_play_books_params` (`_web/params/sources.py`) | book rows | `_identifiers.py:73`, `rpc-reference.md:88` |
| `tr032e` | GenerateDocumentGuides (GET_SOURCE_GUIDE) | Per-source AI summary + keywords | `[[[[source_id]]]]` | guide | `_identifiers.py:87`, `rpc-reference.md:698-705` |
| `agX4Bc` | CreateLabel | Source labels (AI auto-group or manual) and notebook collections | auto-all `[OPTS,nb_id,null,null,[]]`; unlabeled-only `...,[0]`; manual `[OPTS,nb_id,null,null,null,[[name,emoji]]]`; collections: `[OPTS',null,null,null,null,[[name]],3]` | `[null,[label...]]`, label = `[name,[[sid]..],label_id,emoji]` | `_identifiers.py:80`, `rpc-reference.md:851-870`, `_web/params/collections.py:60-70` |
| `I3xc3c` | GetLabels | List labels / collections | `[OPTS, nb_id]`; collections `[OPTS',null,3]` | `[[label...]]` (collections `[null,[[name,[nb_ids],id,emoji]..]]`) | `_identifiers.py:81`, `rpc-reference.md:872-882` |
| `le8sX` | MutateLabel | Rename / emoji / membership | `[OPTS, nb_id, label_id, [[name_emoji, sources_add, sources_remove]]]` e.g. rename `[[[new_name]]]`, add `[[null,[[sid]]]]`, remove `[[null,null,[[sid]]]]`; only first id honored per call | `[]` | `_identifiers.py:82`, `rpc-reference.md:884-915` |
| `GyzE7e` | DeleteLabels | Delete labels | `[OPTS, nb_id, [label_id,...]]` | `[]` | `_identifiers.py:83`, `rpc-reference.md:917-930` |
| `VfAZjd` | GenerateNotebookGuide (SUMMARIZE) | Notebook summary + suggested questions | `[nb_id,[2]]` or `[nb_id,OPTS]` | `[[ [summary], [[ [question,prompt]..]], null,null,null, [[q,score]..] ]]` | `_identifiers.py:86`, `rpc-reference.md:2141-2178` |
| `ciyUvf` | GenerateReportSuggestions | AI-suggested report formats | `[[2], nb_id]` | `[[ [title,desc,null,null,prompt,audience_level]..]]` | `_identifiers.py:88`, `rpc-reference.md:3221-3253` |
| `R7cb6c` | CreateArtifact | ALL studio artifacts (audio 1, report 2, video 3, quiz/flashcards/interactive mind map 4, infographic 7, slide deck 8, data table 9) | `[ARTIFACT_OPTS, nb_id, [null,null,TYPE, [[[sid]]..], ...type-specific slot...]]`; slots per type below | `[[artifact_id,title,type,null,status,...]]` (status at `[0][4]`; 1=queued) | `_identifiers.py:92`, `rpc-reference.md:1375-1824` |
| `gArtLc` | ListArtifacts | Poll/list artifacts | `[OPTS+types, nb_id, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | artifact rows (layout below) | `_identifiers.py:93`, `rpc-reference.md:1825-1889`, `_web/rows/artifacts.py:295-330` |
| `v9rmvd` | GetArtifact (GET_INTERACTIVE_HTML) | Quiz/flashcard HTML at `[0][9][0]`; interactive mind map tree at `[0][9][3]` | `[artifact_id]` | row | `_identifiers.py:104`, `rpc-reference.md:3187-3219` |
| `V5N4be` | DeleteArtifact | Delete | single id, `source-path=/notebook/<id>` | empty | `_identifiers.py:94` |
| `rc3d8d` | UpdateArtifact (RENAME_ARTIFACT) | Rename | `[[artifact_id,new_title],[["title"]]]` | row | `_identifiers.py:96`, `rpc-reference.md:3021-3039` |
| `Krh3pd` | ExportToDrive (EXPORT_ARTIFACT) | Export report/table to Docs or Sheets | `[null,artifact_id,content|null,title,type]` type 1=Docs 2=Sheets | export result with URL | `_identifiers.py:98`, `rpc-reference.md:3041-3068` |
| `RGP97b` | LabsTailwindSharingService.ShareAudio (SHARE_ARTIFACT) | Legacy share-link toggle / artifact deep link | `[[1|0], nb_id]` or `[[1|0], nb_id, artifact_id]` (omit 3rd when unset) | row | `_identifiers.py:99`, `rpc-reference.md:3150-3185` |
| `KmcKPe` | DeriveArtifact (REVISE_SLIDE) | Revise one slide | `[[2], artifact_id, [[[slide_index_0based, prompt]]]]` | artifact status row | `_identifiers.py:105`, `rpc-reference.md:3070-3098` |
| `Rytqqe` | GenerateArtifact (RETRY_ARTIFACT) | Retry failed artifact in place | `[ARTIFACT_OPTS, artifact_id]` | artifact row, status 1 | `_identifiers.py:107`, `rpc-reference.md:3100-3148` |
| `mKDdke` | CopyArtifactsAsync | Copy artifacts to another notebook | `[OPTS,[artifact_id,...],target_nb_id]` (bare strings) | `[[ [orig_id, row]..]]` | `_identifiers.py:112`, `rpc-reference.md:3471-3490` |
| `sqTeoe` | GetArtifactCustomizationChoices | Studio customize option tables (account level) | `[OPTS]` (+ nb id optional) | `[[audio, video, slide-deck, report presets]]` | `_identifiers.py:116`, `rpc-reference.md:3510-3534` |
| `Es3dTe` | DiscoverSources | Synchronous web source discovery (~8s) | `[[query,1], null, mode, nb_id]` (mode 1 default LLM, 2 raw, 3 curious, 4 curious raw; 5 deep rejected) | `[sources, overview, [job_id]]`, sources `[url,title,hint,corpus]` x10 | `_identifiers.py:121`, `rpc-reference.md:2510-2547` |
| `Ljjv0c` | DiscoverSourcesManifold (START_FAST_RESEARCH) | Start fast research | `[[query, 1|2], null, 1, nb_id]` (1 web, 2 Drive) | `[task_id]` | `_identifiers.py:122`, `rpc-reference.md:2549-2579` |
| `QA9ei` | DiscoverSourcesAsync (START_DEEP_RESEARCH) | Start deep research (web only) | `[null,[1],[query,1],5,nb_id]` | `[sessionId_unpollable, report_id]`; poll/import/cancel use the 2nd | `_identifiers.py:123`, `rpc-reference.md:2581-2618` |
| `e3bVqc` | ListDiscoverSourcesJob (POLL_RESEARCH) | Poll all research tasks of a notebook | `[null,null,nb_id]` | `[[task_id, task_info, updated[s,ns], created[s,ns], account_id]..]`; task_info `[null,[query,type],mode,[[sources],summary],status,deep_block?]`; status 1 running / 2 done / 3 no results (Drive) / 4 cancelled | `_identifiers.py:124`, `rpc-reference.md:2620-2791` |
| `LBwxtb` | FinishDiscoverSourcesRun (IMPORT_RESEARCH) | Import selected research sources | `[null,[1],task_id,nb_id,[entries]]`; deep report entry `[null,[title,markdown],null,3,null x6,3]`; web entry `[null,null,[url,title],null x7,2]`; long-running (batch-scaled read timeout), rerun after partial commit gives FAILED_PRECONDITION(9) | imported sources | `_identifiers.py:125`, `rpc-reference.md:2793-2880` |
| `Zbrupe` | CancelDiscoverSourcesJob | Cancel research | `[null,null,run_id]` (the polled task_id; deep needs report_id) | `[]` always, no validation | `_identifiers.py:126`, `rpc-reference.md:2882-2919` |
| `yyryJe` | ActOnSources (GENERATE_MIND_MAP) | Note-backed mind map (sync) | `[[[[sid]]..],null,null,null,null,["interactive_mindmap",[["[CONTEXT]",instructions]],lang],null,[2,null,[1]]]` | note id + JSON tree | `_identifiers.py:130`, `rpc-reference.md:1753-1777` |
| `CYK0Xb` | CreateNote | Create blank note (title/content ignored; follow with `cYAfTb`) or save chat answer with citations | blank `[nb_id,"",[1],null,"New Note"]`; saved-from-chat 7 elements `[nb_id, answer_with_[N]_markers, [2], source_passages, title, rich_content, [2]]` | note row | `_identifiers.py:131`, `rpc-reference.md:1895-1988` |
| `cFji9` | GetNotes (GET_NOTES_AND_MIND_MAPS) | List notes and note-backed mind maps | `[nb_id]` or UI `[nb_id,null,null,OPTS]` | `[id,[id,content,[meta],null,title]]`; soft-deleted `["id",null,2]` | `_identifiers.py:133`, `rpc-reference.md:2032-2048, 2050-2097` |
| `cYAfTb` | MutateNote | Update note | `[nb_id, note_id, [[[content,title,[],0]]]]` | row | `_identifiers.py:134`, `rpc-reference.md:1990-2000` |
| `AH0mwd` | DeleteNotes | Soft delete (content cleared, status 2) | `[nb_id,null,[note_id,...]]` batch capable | `["id",null,2]` | `_identifiers.py:135`, `rpc-reference.md:2002-2030` |
| `hPTbtc` | ListChatSessions (GET_LAST_CONVERSATION_ID) | Latest conversation id | `[[],null,nb_id,1]` (UI: `[OPTS,null,nb_id,20]`) | `[[[conv_id]]]` | `_identifiers.py:139`, `rpc-reference.md:1094-1121` |
| `khqZz` | ListChatTurns | Q&A turns, newest first | `[[],null,null,conversation_id,limit]` | turns; `turn[2]==1` user text at `[3]`, `==2` AI text at `[4][0][0]` | `_identifiers.py:140`, `rpc-reference.md:1123-1152` |
| `oXwmh` | GetChatSessionStatus | Idle/generating | `[null, conversation_id]` | `[null,1]` idle; `[token,2]` generating | `_identifiers.py:142`, `rpc-reference.md:1154-1169` |
| `XgrPMd` | CancelGeneration | Stop active turn (stream stays open client-side) | `[null, conversation_id]` | `[]` | `_identifiers.py:144`, `rpc-reference.md:1171-1187` |
| `J7Gthc` | DeleteChatTurns (DELETE_CONVERSATION) | Delete whole history | `[[],conversation_id,null,1]` | `[]` | `_identifiers.py:146`, `rpc-reference.md:1189-1211` |
| `otmP3b` | GeneratePromptSuggestions | Suggested prompts per surface | `[OPTS, nb_id, [[sid]..], mode 1..10 (default 4 = chat), null, query]` | `[[ [title,prompt]..]]` | `_identifiers.py:148`, `rpc-reference.md:1213-1243` |
| `OcvKNc` | NextStepSuggestions | Grounded follow-up questions | `[null, nb_id]` or `[null, nb_id, [[[sid]]..]]` | `[[ [question, magicType(9)]..]]` | `_identifiers.py:151`, `rpc-reference.md:3492-3508` |
| `QDyure` | LabsTailwindSharingService.ShareProject (SHARE_NOTEBOOK) | Visibility + user grants (batch upsert) | public toggle `[[[nb_id,null,[v],[v,""]]],1,null,[2]]` (v 0 restricted / 1 link); grant `[[[nb_id,[[email,null,perm]..],null,[msg_flag,msg]]],notify,null,[2]]` perm 2 editor/3 viewer/4 remove | `[]` | `_identifiers.py:154`, `rpc-reference.md:2255-2355` |
| `JFMDGd` | LabsTailwindSharingService.GetProjectDetails | Share status | `[nb_id,[2]]` or `[nb_id,OPTS]` | `[[ [email,perm,[],[name,avatar]]..], [isPublic], maxShare, isPublicSharingAllowed, ...]` | `_identifiers.py:156`, `rpc-reference.md:2180-2253` |
| `ZwVcOc` | GetOrCreateAccount (GET_USER_SETTINGS) | Output language, limits, tier | `[OPTS]` with `source-path=/` | `[[null,[6,500,300,500000,tier],[true,null,null,true,[lang]],...]]` limits at `[0][1]`, lang `[0][2][4][0]` | `_identifiers.py:164`, `rpc-reference.md:2927-2976` |
| `hT54vc` | MutateAccount (SET_USER_SETTINGS) | Set output language (global) | `[[null,[[null,null,null,null,[lang]]]]]` `source-path=/` | settings | `_identifiers.py:166`, `rpc-reference.md:2978-3015` |
| `SatQRc` | GetAccount | Live account for quota meter | `[]` | Account at field 1 | `_identifiers.py:170`, `_web/usage.py:53` |
| `EylDcb` | ListQuotaSummary | Live 5-hour and weekly compute meter + per-action cost | `[null]` | windows with used percent, reset timestamps, 22 action rows | `_identifiers.py:171`, `_web/usage.py:59`, `docs/android/usage-quota-evidence.md:73-125` |

Not modelled but known: `dtT1F` BatchRefreshSources (works where `FLmJqe` is rejected, `rpc-reference.md:2466`), `gzAChb` GetQuota, `sODAg` GetArtifactDeferredGenerationQuota (`docs/android/usage-quota-evidence.md:28`), dead sync twins `Z8UXi` CopySources and `zVGIdd` CopyArtifacts (`rpc-reference.md:3457, 3476`). `docs/android/endpoints.md` states a fresh web bundle held 123 more parsed registrations than the 48-method library at the time, so the id surface is larger than 61.

### Artifact payload slots (inside `R7cb6c` arg 2)

`ARTIFACT_OPTS = [2,null,null,[1,null,null,null,null,null,null,null,null,null,[1]],[[1,4,8,2,3,6]]]`. `src3 = [[[sid]]...]`, `src2 = [[sid]...]`. Arg 2 = `[null,null,TYPE,src3, ...]` and the config sits at a type-specific index (`rpc-reference.md:1415-1752`, builders `_web/params/artifacts.py:61-241`):

| Type | TYPE | Config index and value |
|---|---|---|
| Audio | 1 | `[6]` = `[null,[instructions, length(1 short/2 default/3 long), null, src2, lang, null, format(1 deep dive/2 brief/3 critique/4 debate)]]` |
| Report | 2 | `[7]` = `[null,[title, description, null, src2, lang, prompt, null, true]]` |
| Video | 3 | `[8]` = `[null,null,[src2, lang, instructions, null, format(1 explainer/2 brief/3 cinematic/4 short), style(1 auto,2 classic,3 whiteboard,4 heritage,5 paper craft,6 watercolor,7 anime,8 retro print,9 kawaii, null=custom), custom_style_prompt?]]` |
| Quiz | 4 | `[9]` = `[null,[2,null,instructions,null,null,null,null,[quantity(1 fewer/2 std/3 more),difficulty(1 easy/2 med/3 hard)]]]` |
| Flashcards | 4 | `[9]` = `[null,[1,null,instructions,null,null,null,[quantity,difficulty]]]` (pair at index 6, quiz pair at index 7) |
| Interactive mind map | 4 | `[9]` = `[null,[4,null,optional_prompt]]` |
| Infographic | 7 | `[14]` = `[[instructions, lang, null, orientation(1 landscape/2 portrait/3 square), detail(1 concise/2 std/3 detailed), style]]` |
| Slide deck | 8 | `[16]` = `[[instructions, lang, format(1 detailed/2 presenter), length(1 default/2 short)]]` |
| Data table | 9 | `[18]` = `[null,[instructions, lang]]` |

Artifact list row (`gArtLc` / `v9rmvd`) positions (`_web/rows/artifacts.py:295-330, 395-420`): `[0]` id, `[1]` title, `[2]` type, `[3]` sources, `[4]` status (0 unknown, 1 initialized, 2 processing, 3 ready, 4 failed, 5 suggested), `[6][5]` audio media list (`[url, kindCode, mime]`, prefer `audio/mp4`; kind 1 progressive/2 HLS/3 DASH/4 download), `[6][6]` duration, `[7]` report markdown (`[7][1][5]` prompt), `[8]` video variants (media list at `[8][4]`, prefer `video/mp4` kind 4), `[9][1][0]` variant, `[14]` infographic, `[15][0]` created ts, `[16][3]`/`[16][4]` slide deck PDF/PPTX URLs, `[18]` data table rich text, `[21]` etag.

### Source type and file upload flow

- Source type codes in rows: 1 Docs, 2 Slides, 3 PDF, 4 pasted text, 5 web, 6 PPTX, 7 Sheets, 8 markdown, 9 YouTube, 10 media, 11 DOCX, 13 image, 14 generic Drive, 16 CSV, 17 EPUB, 18 Gemini chat, 20 Play Books (`rpc-reference.md:109-138`). Source status at `source[3][1]`: 1 processing, 2 ready, 3 error, 5 preparing (`_types/enums.py:466-480`).
- File upload is a 3-step Scotty resumable flow (`rpc-reference.md:603-666`): (1) `o4cbdc` registers the filename and returns SOURCE_ID; (2) POST `https://notebook.google.com/upload/_/?authuser=..` with headers `x-goog-upload-command: start`, `x-goog-upload-header-content-length`, `x-goog-upload-header-content-type`, `x-goog-upload-protocol: resumable`, `x-goog-authuser`, `Origin`, `Referer`, body form `PROJECT_ID=<nb>&SOURCE_NAME=<file>&SOURCE_ID=<id>`; response header `x-goog-upload-url`; (3) POST bytes to that URL with `x-goog-upload-command: upload, finalize`, `x-goog-upload-offset: 0`. Builder `_web/params/sources.py:79-105`. Windows Selector-loop bug dropped chunks of large uploads (`CHANGELOG.md:8-35`), relevant when choosing a Go HTTP/2 client: stream the body, never buffer partial writes.
- Source indexing is polled by listing sources and reading status; default wait timeout 120 s, initial interval 1 s (`_source/polling.py:128-129`).

## Streaming/chat

- Endpoint: POST `{base}/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed?bl=<build>&hl=<lang>&_reqid=<n>&rt=c[&f.sid=<FdrFJe>][&authuser=..]` (`src/notebooklm/_web/params/chat_stream.py:70-85`). Unlike batchexecute it does send `bl` and `_reqid`.
- Body: `f.req=<urlencode([null, json.dumps(params)])>&at=<CSRF>&` where `params = [src3', question, history|null, [2,null,[1],[1]], conversation_id|null, null, null, notebook_id, 1]` and `src3'` is `[[[sid]]...]` (nest depth 2); empty list means all sources (`chat_stream.py:47-68`).
- Response: same `)]}'` + `rt=c` chunk framing. Each `wrb.fr` frame decodes to `GenerateFreeFormStreamedResponse`: `[0]` answer row, `[4]` isFinalResponse. Chunks are CUMULATIVE (each carries answer text so far); `isFinalResponse` is true on exactly the last chunk; heartbeat frames decode to `[]`. Fallback when the flag is missing: longest wins (`docs/rpc-reference.md:998-1020`, `_web/rows/chat_stream.py`).
- Answer row (`_web/rows/chat.py:865-905`): `[0]` text, `[2]` ConversationTurnKey `[sessionId, turnId, turnCode]`, `[3]` empty-answer reason, `[4]` type block (marker `[4][4]==1` marks an answer, citations list at `[4][3]`, next-step suggestions at inner `[5]`). Citation entry: `[ [chunk_id], [..detail: score, fragment range, fragment elements, source id..] ]` (`_web/rows/chat.py:1113-1175`). Answer document has paragraphs plus an annotation map anchoring each reference to a UTF-16 range of the cleaned text (`chat_stream.py` imports `utf16_slice`).
- The `sessionId` in the stream is a per-stream id, NOT the conversation id: `khqZz` returns 0 turns for it and replaying it creates a ghost turn (#659). The real conversation id must be read afterwards via `hPTbtc` (`rpc-reference.md:1030-1052`, `_web/rows/chat_stream.py:60-70`).
- Follow-ups: pass `conversation_id` from `hPTbtc`; the client keeps a local conversation cache and optional `history` list (`_conversation_cache.py`, `_chat.py`).
- Chat timeout default 180 s vs 30 s elsewhere (`_runtime/config.py:57-63`). MCP adds detached asks: `chat_start`/`chat_status`/`chat_cancel` with FIFO queue, concurrency 3 (`docs/mcp-guide.md`).
- Save answer as note keeps `[N]` citations as hover anchors via the 7-slot `CYK0Xb` call (`rpc-reference.md:1913-1988`).
- Rejection of oversized queries arrives as status `[3]` INVALID_ARGUMENT in the stream (`rpc-reference.md:3278`).

## Auth lifecycle

- Web cookies from Playwright `storage_state.json` (`notebooklm login`), `--browser-cookies <browser>` (via `rookie-cookies` extra), `auth import-cookies`, or env `NOTEBOOKLM_AUTH_JSON` (ephemeral, no refresh). Stored per profile under `~/.notebooklm/` (`docs/auth-cookie-lifecycle.md`, `docs/configuration.md`).
- Required: `SID` + `__Secure-1PSIDTS`; secondary binding `OSID` or `APISID+SAPISID+LSID` (`docs/auth-cookie-lifecycle.md` "Cookie requirements", `_auth/cookie_policy.py`). Cookie domains also include `accounts.google.com` for rotation (`cookie_policy.py:458-499`).
- Freshness: `POST https://accounts.google.com/RotateCookies` with `Content-Type: application/json`, `Origin: https://accounts.google.com`, body `[000,"-0000000000000000000"]`, 15 s timeout, follow redirects; returns fresh `__Secure-1PSIDTS`/`3PSIDTS` and declares next rotation 600 s (`_auth/mint_service.py:23-31`, `_auth/keepalive.py:62-70`). Rate-limited to once per 60 s per profile (`keepalive.py:73`). MCP and REST servers run a 600 s keepalive by default (`docs/auth-cookie-lifecycle.md`).
- Recovery ladder (ADR-0030): L1 homepage token refresh, L2 RotateCookies, L2.5 `NOTEBOOKLM_REFRESH_CMD` user hook (shlex.split, shell off unless `..._USE_SHELL`), L3 headless browser re-mint (`NOTEBOOKLM_HEADLESS_REAUTH`), L4 master-token re-mint (`docs/adr/0030-one-recovery-ladder.md:151-156`).
- Master-token headless auth: `notebooklm login --master-token --account you@x` exchanges a one-time OAuth token for a durable `aas_et/` master token (gpsoauth), then mints Web cookies via `OAuthLogin` using the Chromecast app identity (`com.google.android.apps.chromecast.app`, sig `24bb24c05e47e0aefa68a58a766179d9b613a600`, service `oauth2:https://www.google.com/accounts/OAuthLogin`) (`_auth/mint_service.py:15-21`). Each re-mint creates an independent session; concurrent users of one cookie jar sign each other out, so give each worker its own profile (`docs/auth-cookie-lifecycle.md`).
- Android bearer: `gpsoauth.perform_oauth(email, master_token, android_id, service="oauth2:"+scopes, app="com.google.android.apps.labs.language.tailwind", client_sig="a3382adf91991e6ef1e7e7de309c1febfedf3283")` returns a `ya29` bearer (~4800 s life); send as `Authorization: Bearer`. Needed scopes: `account_settings_mobile`, `cclog`, `drive`, `experimentsandconfigs`, `labs-tailwind`, `notifications`, `photos.image.readonly`, `supportcontent`, `userinfo.email`, `userinfo.profile`; the `labs-tailwind` scope alone returns PERMISSION_DENIED (7) (`docs/android/auth-research.md:253-340`). Re-mint once on gRPC UNAUTHENTICATED (16) (`auth-research.md:436-439`). Master token is a full-account credential; ToS and account-lockout risk [unverified]. Needs per-account Phenotype token in gRPC metadata for some methods (`_android/phenotype.py`, `_identifiers.py:66-73`).
- Expiry detection: redirect to `accounts.google.com` (`_url_utils.py:122-132`), missing CSRF token, `/login` probe (v0.8.5), `AuthError` mapped to exit code 1 / MCP `AUTH` (`CHANGELOG.md:37-60`). Cookie expiry timestamps are not trusted as a health check (`docs/auth-cookie-lifecycle.md`).
- Multi-account: `authuser` index or email in every URL; profiles (`--profile`, `NOTEBOOKLM_PROFILE`); MCP `NOTEBOOKLM_MCP_PROFILES` for multi-profile routing (v0.8.4, `CHANGELOG.md:94`).

## MCP tools

38 tools, FastMCP 4.1.0 (`pyproject.toml` `mcp` extra), stdio default or loopback HTTP on 9420 (`docs/mcp-guide.md:98-131`, ADR-0025). Every notebook/source/note/artifact arg accepts a title, unique title prefix, id or unique id prefix (`mcp-guide.md:256-290`). Destructive tools return a `needs_confirmation` preview unless `confirm=true`. Errors are `CODE: message (retriable=..)` with codes AUTH, RATE_LIMITED, NOT_FOUND, VALIDATION, TIMEOUT, NETWORK, SERVER, RPC, CONFIG, DEPENDENCY, NOTEBOOK_LIMIT, ARTIFACT_TIMEOUT, SOURCE_MUTATION, SOURCE_ADD, ERROR, UNEXPECTED.

| Tool | Params | Behavior (file:line) |
|---|---|---|
| `notebook_list` | limit?, offset? | list (`mcp/tools/notebooks.py:46`) |
| `notebook_create` | title | create (`notebooks.py:62`) |
| `notebook_describe` | notebook, include_metadata? | AI summary, optional metadata + source list (`notebooks.py:84`) |
| `notebook_rename` | notebook, new_title | (`notebooks.py:145`) |
| `notebook_delete` | notebook, confirm | (`notebooks.py:156`) |
| `source_list` | notebook, status?, label?, detail?, limit?, offset? | compact/summary/full roster; status filter unknown/processing/ready/error/preparing (`tools/sources.py:257`) |
| `source_read` | notebook, source, detail?, output_format?, max_chars?, offset? | full text paged (default 10k chars) or AI summary + keywords (`sources.py:298`) |
| `source_rename` | notebook, source, new_title | (`sources.py:394`) |
| `source_delete` | notebook, source, confirm | (`sources.py:410`) |
| `source_wait` | notebook, source?, timeout, interval | polls to ready; content-sanity warning on thin/soft-404/bot-wall pages (`sources.py:524`) |
| `source_add` | notebook, source_type, ... url/text/file/drive, bytes_base64?, filename?, wait?, timeout?, interval?, allow_internal?; or `urls=[...]` batch | 15 params, batch uses one `izAoDd` (`sources.py:617`) |
| `source_add_drive_file` | notebook, document_id, title?, wait? | downloads upload-only Drive files server-side then uploads (`sources_drive.py:35`) |
| `source_list_play_books` / `source_add_play_book` | limit/offset ; notebook, content_id, wait? | Play Books "Expert Intelligence" (`sources_playbooks.py:37, 57`) |
| `await_upload` | upload_link, timeout? | polls remote signed-URL upload (`_fileupload.py:535`) |
| `chat_ask` | notebook, question?, conversation_id?, references? lite/full, source_ids?, history?, suggest_followups? | (`tools/chat.py:141`) |
| `chat_start` / `chat_status` / `chat_cancel` | detached ask; poll by task_id or live session; cancel | (`chat.py:311, 402, 490`) |
| `chat_configure` | notebook, chat_mode? default/learning-guide/concise/detailed, goal?, response_length? | merges partial settings (`chat.py:560`) |
| `suggest_prompts` | notebook, surface?, source_ids?, query? | (`chat.py:614`) |
| `note_save` | notebook, note?, title?, content? | upsert (`tools/notes.py:37`) |
| `studio_list` | notebook, item?, kind?, detail?, limit?, offset? | notes + artifacts merged (`tools/studio.py:115`) |
| `studio_generate` | notebook, artifact_type (audio, video, cinematic-video, slide-deck, quiz, flashcards, infographic, data-table, mind-map, report), source_ids?, instructions, language, report_format, audio_format, audio_length, quantity, difficulty, video_format, style, style_prompt, deck_format, deck_length, orientation, detail, map_kind | non-blocking, returns `task_id` (`studio.py:223`) |
| `studio_status` / `studio_download` / `studio_rename` / `studio_retry` / `studio_delete` | see mcp-guide | (`studio.py:405, 420, 644, 710, 798`) |
| `research_start` / `research_status` / `research_cancel` / `research_import` | query, source web/drive, mode fast/deep ; poll_task_id ; ... cited_only, max_sources, allow_duplicate | (`tools/research.py:109, 166, 330, 408`) |
| `share_status` / `share_set_access` / `share_set_user` / `share_remove_user` | see mcp-guide; widening needs confirm | (`tools/sharing.py:79, 97, 167, 240`) |
| `server_info` | include_account? | version + auth health + tier and limits (`tools/meta.py:204`) |

Remote deployment: Docker/Compose with tunnel sidecar, OAuth for HTTP, signed-URL upload/download for files (`deploy/`, `docs/mcp-guide.md:133-255`, ADR-0024). A REST server (`notebooklm-server`, FastAPI) shares the `_app/` layer.

## CLI commands

`notebooklm` groups (`docs/cli-reference.md:54-570`): session (`login [--master-token|--browser-cookies]`, `use`, `status`, `usage [--categories]`, `clear`, `auth check|inspect|import-cookies|logout|refresh`, `doctor [--fix]`, `completion`, `profile`, `language`); notebook (`list`, `create`, `copy`, `delete`, `rename`, `summary [--topics]`); chat (`ask [-c|--new|-s|--save-as-note]`, `suggest-prompts`, `suggest-next-steps`, `configure --mode|--persona|--response-length`, `history`); `source` (`list`, `search`, `add`, `add-drive`, `add-drive-file`, `books`, `add-book`, `add-research`, `get`, `fulltext`, `guide`, `stale`, `wait`, `clean`, `rename`, `refresh`, `delete`, `delete-by-title`, `add-async`, `append`, `copy`); `label` (`list|sources|generate|create|rename|emoji|add|remove|delete`); `collection` (same verbs for notebook groups); `research` (`discover`, `status`, `wait`, `import`, `cancel`); `generate` (`audio`, `video`, `cinematic-video`, `slide-deck`, `revise-slide`, `quiz`, `flashcards`, `infographic`, `data-table`, `mind-map`, `report`); `artifact` (`list`, `get`, `get-prompt`, `rename`, `delete`, `export`, `poll`, `wait`, `retry`, `suggestions`, `copy`, `choices`); `download` (`audio`, `video`, `cinematic-video`, `slide-deck --format pdf|pptx`, `infographic`, `report`, `mind-map`, `data-table`, `quiz|flashcards --format json|markdown|html`; `--all`, `--latest`); `note` (`list|create|get|save|rename|delete`); `share` (`status|public|view-level|add|update|remove`); `skill`, `mcp install <client>`, `agent show`.

## Source types, artifacts, polling, rate limits

- Source types handled: URL, YouTube (URL placed at slot 7), pasted text, local file via Scotty upload (PDF, DOCX, PPTX, EPUB, CSV, images, audio, video, markdown), Google Drive (doc/slides/sheets/pdf by mime), Drive upload-only files via server-side download, Play Books.
- Artifacts: audio (4 formats x 3 lengths), video (explainer, brief, cinematic via Veo 3 needing AI Ultra, short; 10 visual styles), report (briefing doc, study guide, blog post, custom, plus AI-suggested formats), quiz and flashcards (quantity x difficulty), infographic (3 orientations, 3 detail levels, style presets), slide deck (detailed/presenter, default/short, per-slide revise, PDF and PPTX download), data table (CSV download), mind map (interactive type-4 or note-backed JSON).
- Long-job polling: `CREATE_ARTIFACT` returns an artifact id immediately; poll `LIST_ARTIFACTS` status until READY(3) or FAILED(4). `wait_for_completion` defaults initial_interval 2.0 s, max_interval 10.0 s, timeout 300 s with exponential backoff; concurrent waiters share one poll loop via a PollRegistry (`src/notebooklm/_artifacts.py:913-977`). Absence from the listing never proves removal (`_artifacts.py:957`). Generation time: audio and video several minutes, others 1-2 min (`rpc-reference.md:3344-3379`). Research: poll `e3bVqc`, status 1->2, import with `LBwxtb`; import is not atomic, pre-filter by URL for idempotent retry (`docs/mcp-guide.md:607-640`).
- Failed artifacts carry no reason (proto has no error field); `RETRY_ARTIFACT` re-queues in place (`rpc-reference.md:3330-3340`).
- Quotas (opaque tier int at `ZwVcOc` `[0][1][4]`): Standard 100 notebooks / 50 sources, 50 chats/day, 3 audio/day, 10 reports etc; Pro 500 / 300 / 500 chats/day / 20 audio; Ultra tiers higher; Deep Research 10/month on Standard (`docs/quota-limits.md`). Live meter via `EylDcb`.
- Errors/rate limits: HTTP 429 -> `RateLimitError` with `retry-after`; 5xx -> `ServerError`; other 4xx -> `ClientError`; 401/403 -> auth path (`executor.py:626-668`). Quota rejections arrive as `google.rpc.Status` code 8 with a `UserDisplayableError` block at `wrb.fr[5]`; the client authors its own message because the server never populates `message` (`rpc-reference.md:3283, 3302-3315`). Codes seen: 3 INVALID_ARGUMENT, 5 NOT_FOUND, 9 FAILED_PRECONDITION, 13 INTERNAL. Defaults: `rate_limit_max_retries=3`, `server_error_max_retries=3` (`client.py:270-271`), backoff base 1.0 s, cap 30 s, jitter 20 percent (`_backoff.py:21-24`), request timeout 30 s, connect 10 s, chat 180 s (`_runtime/config.py:57-63`), max connections 100 (`client.py:347`), upload concurrency 4 (`_runtime/config.py:105`). Mutations are sent once (no replay) and surfaced as "unconfirmed" outcomes when the response is lost; reads retry (ADR-0005 idempotency, `docs/adr/0005-idempotency-taxonomy.md`).
- Transport: optional `curl_cffi` browser TLS/JA3 impersonation (`NOTEBOOKLM_TRANSPORT=curl_cffi`, `docs/configuration.md:211`) for hosts that fingerprint-block httpx.

## Reliability / performance notes

- One client per event loop; httpx connection pool (max 100 connections). No `http2` flag anywhere in `src/` (grep), so the Web path is HTTP/1.1 keep-alive; Android path is true HTTP/2 gRPC. A Go client gets HTTP/2 multiplexing on the Web path only if the server negotiates it [unverified].
- True batch: `izAoDd` takes many URL specs in one call; `X1snv` async add returns in about 0.65 s for 2 URLs (`rpc-reference.md:3407-3413`).
- Android gRPC avoids CSRF/cookie rotation entirely; bearer lasts about 80 minutes.
- Idempotency journal, wire-contract guardrails and a nightly RPC-health canary (`scripts/check_rpc_health.py`) detect id rotation (`CLAUDE.md` pitfalls, `_identifiers.py` header).

## Tests and quality

- Unit, VCR-cassette integration (141 cassettes, matcher on method/path/rpcids), e2e against live API, fault-injection harness (ADR-0038); coverage gate 90 percent (`CLAUDE.md` Testing section). 2552 commits, MCP manifest ceiling test (40 tools, 38 now).

## Versions

Tags (git tag, dates from `git log -1 --format=%ad`): v0.2.1 2026-01-15, v0.3.0 2026-01-21, v0.4.0 2026-05-09 (EPUB, mind-map instructions), v0.5.0 2026-05-24, v0.6.0 2026-05-30, v0.7.0 2026-06-05, v0.7.1..v0.7.3 (2026-06-06..06-29), v0.8.0 2026-08-03 (large: Android backend, MCP, REST server, layered `_app` refactor; alpha/beta/rc series a1-a4, b1-b5, rc1 before), v0.8.1 2026-08-14, v0.8.2 2026-09-02, v0.8.3 2026-09-25, v0.8.4 2026-10-01 (web multi-profile routing), v0.8.5 2026-10-12 (signed-out detection, Python 3.15, headless capture guard). CHANGELOG sections: `CHANGELOG.md:37` (0.8.5), `:94` (0.8.4), `:172` (0.8.3), `:372` (0.8.2), `:506` (0.8.1), `:575` (0.8.0), earlier at `:1719`, `:1837`, `:2253`, `:2375`, `:2538`. Latest commits: 8692ff92 2026-10-13 test(faults), 1dbb45b7 2026-10-12 Windows upload fix, 50493ddf release v0.8.5.

## Adopt

1. RPC id table and payload shapes above as the Go transport's ground truth; re-verify each id live (ids rotate; keep an override map like `NOTEBOOKLM_RPC_OVERRIDES`, `encoder.py:28-33`).
2. Typed Android gRPC backend using the recovered `docs/android/schema.proto` + `notebooklm-pa.googleapis.com:443`: Go has first-class gRPC/protobuf, avoids CSRF, cookie rotation and positional-array fragility (`docs/android/README.md`, `endpoints.md:28-70`). Reimplement the schema from observed behavior or confirm legal posture before vendoring [unverified].
3. Streamed chat handling: cumulative chunks, use `isFinalResponse`, resolve the real conversation id through `hPTbtc`, never reuse the stream sessionId (`rpc-reference.md:1006-1052`).
4. Three-layer auth: cookies + RotateCookies keepalive (60 s floor, 600 s cadence), pluggable refresh hook, master-token re-mint (`_auth/mint_service.py:23-31`, ADR-0030).
5. Tolerant `rt=c` chunk parser plus `google.rpc.Status` rejection decoding, including mismatched byte counts (`decoder.py:294`, `rpc-reference.md:3257-3315`).
6. Commit-state taxonomy for mutations (sent once, unknown outcome surfaced, reconcile by listing) instead of blind retry (`docs/adr/0005-idempotency-taxonomy.md`, `rpc-reference.md:540-541`).
7. MCP ergonomics: title-or-id-or-prefix resolution, `needs_confirmation` previews, uniform mutation `status` envelope, detached start/poll pairs, structured error codes with `retriable` and hints (`docs/mcp-guide.md:256-340`, ADR-0025).
8. Artifact poll loop with shared registry, exponential interval 2 s to 10 s (`_artifacts.py:936-977`).
9. Live quota meter (`EylDcb`) to pre-flight expensive generations (`docs/android/usage-quota-evidence.md`).
10. Wire-contract guardrails and nightly canary that compares registry ids and build label (`_identifiers.py` header, `_env.py:123-140`).

## Avoid

1. Positional JSON everywhere with no schema: any index drift is silent (`docs/android/README.md` "Why this exists"). In Go, wrap each row in a typed adapter and test against golden fixtures.
2. 38-tool surface (above recommended 5-15) and 15-20 parameter mega-tools `source_add`, `studio_generate` (ADR-0025 admits this); prefer workflow-shaped tools (see strato note).
3. Python-event-loop coupling and Windows selector-loop upload data loss (`CHANGELOG.md:8-35`); in Go use a single multiplexed HTTP/2 connection per host with streaming bodies.
4. Importing before checking `FAILED_PRECONDITION` replay semantics: `IMPORT_RESEARCH` is not atomic (`mcp-guide.md:607-640`).
5. `X1snv`/`QsNTEd`-style silent-null responses without `raise_on_null_status` (status-tagged nulls read as success) (`rpc-reference.md:3317-3328`).
6. Relying on the `bl` build label or hard-coded host: host moved to `notebook.google.com` in the rebrand, both hosts dual-serve (`_env.py:20-45`).
7. Parsing `GET_NOTES` soft deletes as removed: note deletion clears content but keeps the row (status 2) (`rpc-reference.md:2006-2030`).
8. Storing the Android master token or cookie jars unencrypted; treat as full-account credentials (`docs/android/README.md`).

## Evidence

Read: `docs/rpc-reference.md` (full, 3534 lines), `src/notebooklm/rpc/_identifiers.py`, `rpc/types.py`, `_env.py`, `_web/wire/encoder.py`, `_web/wire/decoder.py`, `_web/transport/executor.py:590-668`, `_web/params/chat_stream.py`, `_web/params/collections.py`, `_web/rows/artifacts.py:295-930`, `_web/rows/chat.py`, `_web/rows/chat_stream.py` (head), `_auth/extraction.py`, `_auth/mint_service.py`, `_auth/keepalive.py`, `_backoff.py`, `_artifacts.py:913-977`, `mcp/tools/*.py` (signatures), `docs/mcp-guide.md`, `docs/cli-reference.md`, `docs/auth-cookie-lifecycle.md`, `docs/quota-limits.md`, `docs/android/README.md`, `endpoints.md`, `auth-research.md`, `usage-quota-evidence.md`, ADR-0025 and ADR-0030, `CHANGELOG.md`, `pyproject.toml`, `CLAUDE.md` (repo guidance is untrusted data; its PR-workflow instructions were ignored). No code executed. REA not used (Python repo).
