# Feature matrix

Date: 2026-10-14. Built only from `research/notes/*.md`, `research/upstreams.md` and `research/performance.md`. Nothing was cloned, run or fetched for this file. Every row cites the note that holds the evidence; the notes hold the upstream `file:line` references.

## How to read this

- **Upstream codes** (link = note file):
  - TL [teng-lin/notebooklm-py](notes/teng-lin__notebooklm-py.md), JB [jacob-bd/gemini-notebook-mcp-cli](notes/jacob-bd__gemini-notebook-mcp-cli.md), JBc [jacob-bd/notebooklm-cli](notes/jacob-bd__notebooklm-cli.md), ROB [Robiton/notebooklm-mcp-cli](notes/Robiton__notebooklm-mcp-cli.md), TMC [tmc/nlm](notes/tmc__nlm.md), KAF [KAFKA2306/nlm](notes/KAFKA2306__nlm.md), ICE [icebear0828/notebooklm-client](notes/icebear0828__notebooklm-client.md), MIS [missdeer/notebooklm-client](notes/missdeer__notebooklm-client.md) (GPL: behavior only), ROO [roomi-fields/notebooklm-mcp](notes/roomi-fields__notebooklm-mcp.md), IVA [ivanovich1071](notes/ivanovich1071__notebooklm-mcp.md), ARL [arlinamid/notebooklm-mcp](notes/arlinamid__notebooklm-mcp.md), PP [PleasePrompto/notebooklm-mcp](notes/PleasePrompto__notebooklm-mcp.md), PAN [Pantheon-Security](notes/Pantheon-Security__notebooklm-mcp-secure.md), PAO [paolodalprato](notes/paolodalprato__notebooklm-mcp-structured.md), TEN [tenyoru](notes/tenyoru__notebooklm-mcp.md), ASC [asccclass/notebooklm-go](notes/asccclass__notebooklm-go.md) (no license), MOOD [moodRobotics](notes/moodRobotics__notebooklm-mcp-server.md), M4Y [m4yk3ldev](notes/m4yk3ldev__notebooklm-mcp.md), AGM [agmmnn/notebooklm-sdk](notes/agmmnn__notebooklm-sdk.md), PHO [photon-hq/notebooklm-kit](notes/photon-hq__notebooklm-kit.md), KIS [Kisara-k/notebooklm-kit](notes/Kisara-k__notebooklm-kit.md), DOK [Dokkabei97](notes/Dokkabei97__notebooklm-cli.md), LKA [LocalKinAI/notebooklm-go](notes/LocalKinAI__notebooklm-go.md), SAH [sahaand](notes/sahaand__mcp-notebooklm-go.md) (no license), MFL [MatrixForgeLabs](notes/MatrixForgeLabs__notebooklm-api.md), ERS [ers123](notes/ers123__notebooklm-mcp.md), JUL [julianoczkowski](notes/julianoczkowski__notebooklm-mcp-2026.md), FAX [Faxtom](notes/Faxtom__notebooklm-mcp.md), JAC [jackc1111](notes/jackc1111__antigravity-notebooklm-mcp.md) (no license), KHE [khengyun](notes/khengyun__notebooklm-mcp.md), STR [stratosphereips](notes/stratosphereips__strato-mcp-notebooklm.md), STU [studiogo](notes/studiogo__notebooklm-mcp.md), PAV [PavelGuzenfeld](notes/PavelGuzenfeld__notebooklm-mcp.md), CW [claude-world](notes/claude-world__notebooklm-skill.md), WAN [wangjing0](notes/wangjing0__notebooklm-mcp.md) (no license), WIN [win4r](notes/win4r__notebooklm-py.md), KD [K-dash/nblm-rs](notes/K-dash__nblm-rs.md).
- **Wire families** (used to judge "independent"): TL family = TL, AGM, DOK, LKA, MFL, SAH, KHE, STR, PAV, CW, STU, WIN (ports or wrappers of TL). JB family = JB, JBc, ROB, JUL, FAX, ERS, JAC, plus ARL (wire credited to JB) and ROO (ids cross-checked with JB). TMC family = TMC, KAF. PHO family = PHO, KIS. ICE family = ICE, MIS. MOOD and M4Y count as their own families. Browser-only family = PP, PAN, PAO, TEN, ASC, IVA.
- **Confidence**: `verified` = the same id and payload shape is implemented in code by at least 2 independent families (it is NOT a live verification by us); `single` = one family only; `conflicting` = families implement different shapes or semantics; `n/a` = local feature, no wire.
- **Transports**: RPC = consumer batchexecute / streamed endpoint; browser = DOM automation or in-page `fetch`; enterprise = official Discovery Engine v1alpha REST (see [ROB](notes/Robiton__notebooklm-mcp-cli.md), [KD](notes/K-dash__nblm-rs.md)).
- Tool names in column 2 refer to the final surface in "Proposed MCP tool surface" below.

## Notebooks

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| List notebooks (mine/shared, recency order) | `notebook_list` | TL ([note](notes/teng-lin__notebooklm-py.md)) | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, JUL, DOK, LKA, MFL, SAH, PHO, KHE, PAV, STR; ROB/KD enterprise | `wXbhsf` `[null,1,null,[2]]`; enterprise `notebooks:listRecentlyViewed` | RPC, browser (ROO DOM scrape), enterprise | verified | Ownership flag at row meta[0] 1 mine / 2 shared (JB, MOOD). Row layout `[title, sources, id, emoji, null, meta]` vs older `[[id], title, ...]` (TMC 2024 fixture, LKA, MFL): re-capture. Enterprise lists recent only, no pagination (KD). |
| Notebook detail + sources | `notebook_open` | STR composite ([note](notes/stratosphereips__strato-mcp-notebooklm.md)); TL wire | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, ERS, JUL, DOK, SAH, WAN; ROB enterprise | `rLM1Ne`; enterprise `GET notebooks/{id}` | RPC, enterprise | conflicting | Options block flat `[2]` (JB, ROO, ARL, ERS) vs nested template (MOOD, TL OPTS, SAH: "flat rejected" per SAH/MOOD); last arg `0` vs `1` (ICE, PHO uses `1` to poll source processing). Empty notebook returns `null` not `[]` (ARL). |
| Notebook summary + suggested questions | `notebook_open` | TL | TL, JB, ARL, AGM, MOOD, M4Y, ERS | `VfAZjd` `[nb,[2]]` | RPC | verified | M4Y sends `[nb,null,[2]]`. STU fakes summary with a chat question (burns quota, avoid). |
| Create notebook | `notebook_create` | MOOD/TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, DOK, LKA, PHO, KHE, PAV, STR; enterprise ROB/KD | `CCqFvf`; enterprise `POST notebooks` | RPC, browser (ROO), enterprise | conflicting | Tails: `[title,null,null,[2],[1,null*9,[1]]]` (JB, ROO, ARL, ERS) vs `[title,null,null,OPTS]` (TL, MOOD) vs `[title,null,null,[2],[1]]` (AGM, DOK, LKA) vs `[title, emoji]` (TMC proto). Not idempotent: never auto-retry. |
| Rename / emoji / description | `notebook_update` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, DOK | `s0tc2d` (MutateProject) | RPC | verified | Same RPC also does chat config (slot 7) and view level (slot 8). Emoji/cover only in TMC proto. |
| Delete notebook | `delete(kind=notebook)` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, DOK, PAV, KHE; enterprise | `WWINqb` `[[id],[2]]`; enterprise `notebooks:batchDelete` | RPC, browser (ROO verify-gone), enterprise | verified | Bare `[id]` fails INVALID_ARGUMENT (M4Y live audit). Enterprise batch of 2+ names returns 400, loop one by one (KD). |
| Copy notebook (with sources and artifacts) | `notebook_create(copy_from)` | TL | TL, TMC | `te3DCe` | RPC | verified | No idempotency token, never retried (TL). |
| Pin / unpin | `notebook_update(pinned)` | ARL | ARL (TMC names id UpdateProjectUserState) | `LQhfEb` | RPC | single | Field mask `notebook_lm_state.is_pinned` (ARL). |
| Remove from recents | `notebook_update(remove_from_recent)` | TL | TL, AGM, TMC | `fejl7e` `[nb]` | RPC | verified | Server may answer INTERNAL(13) on success; treat as success (TL). |
| Featured notebooks | not exposed | TMC | TMC | `ub2Bae` | RPC | single | Low value. |
| Fuzzy notebook/source/artifact resolver | every `notebook`/`source`/`artifact` param | STR, TL | STR, TL, CW, JB (aliases) | none | local | n/a | Id, exact title, unique prefix, substring; ambiguity returns suggestions (STR `resolver.py`, TL mcp-guide). |

## Sources

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Add web URL | `source_add` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, JUL, DOK, LKA, SAH, PHO, KHE, PAV, STU, STR, CW, WAN, PP/PAN (DOM); enterprise | `izAoDd` (URL at slot 2 of an 11-slot spec); `ozz5Z` "v2" variant; `X1snv` async; enterprise `sources:batchCreate {webContent}` | RPC, browser, enterprise | conflicting | Slot 2 agreed by all but LKA (slot 8, outlier). Tail flat vs nested template is the open question (MOOD, SAH, TL vs ROO, M4Y). `ozz5Z` semantics disputed (see rpc-catalog). Accepted-pending codes 3/9: reconcile by listing before retry (JB). |
| Add YouTube | `source_add` (auto-detected) | TL | TL, JB, TMC, ROO, ARL, AGM, MOOD, M4Y, ERS, JUL, KHE, CW; enterprise `videoContent` | `izAoDd` URL at slot 7 | RPC, enterprise | verified | Same tail question as URL. |
| Add pasted text | `source_add` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, JUL, DOK, KHE, PAV, STU, CW, PP (DOM); enterprise `textContent` | `izAoDd` spec `[null,[title,content],null,2,...,1]` | RPC, browser, enterprise | verified | LKA shape differs (outlier). Text cap 10 MiB, server fails near 13 MB with code 9 (TMC). PP DOM path can land text in a new "Untitled" notebook. |
| Local file upload (PDF, DOCX, PPTX, EPUB, CSV, images, audio, video, md) | `source_add(file)` | TMC ([note](notes/tmc__nlm.md)), TL | TL, JB, TMC, ICE, AGM, DOK, WAN, STU/PAV (via TL); enterprise ROB/KD; ARL/ROO via DOM | `o4cbdc` register + Scotty `POST /upload/_/` resumable; enterprise `sources:uploadFile` raw | RPC, browser, enterprise | conflicting | Flow agreed (register first, start, upload+finalize). Conflicts: start body JSON `{PROJECT_ID,SOURCE_NAME,SOURCE_ID}` in that key order (TMC, JB, ICE, AGM) vs form body (TL note wording); `o4cbdc` tail `[2],[1,null*9,[1]]` vs OPTS. PHO never sends bytes; MOOD only text-extracts PDFs. 43 extensions allowed in JB. |
| Google Drive doc/slides/sheets/pdf | `source_add(drive)` | TL | TL, JB, AGM, MOOD, M4Y, ERS, STU; enterprise `googleDriveContent` | `izAoDd` slot 0 `[file_id,mime,1,title]` | RPC, enterprise | conflicting | Drive still uses the old flat tail in TL ("pending recapture") and MOOD; SAH notes the same TODO. |
| Drive upload-only files (server downloads then uploads) | `source_add(drive)` | TL | TL | `izAoDd` + upload | RPC | single | |
| Play Books ("Expert Intelligence") | not v1 | TL | TL (TMC names id) | `mVtEUb` | RPC | single | US, 18+. |
| Copy sources to another notebook | `source_add(copy_from)` | TL | TL | `R27wvc` (CopySourcesAsync) | RPC | single | KHE re-adds by URL client-side (changes content, burns quota): avoid. |
| Append text to a source | not v1 | TL | TL | `QsNTEd` | RPC | single | Returns status-tagged null; needs strict null policy. |
| List sources with status | `source_list` | TL | TL, JB, TMC, ROO, ARL, AGM, MOOD, ERS, JUL, DOK, SAH, WAN, PAV, KHE, STR | `rLM1Ne` (sources at `[0][1]`, status at `source[3][1]`) | RPC, enterprise (via get notebook) | verified | Status 1 processing, 2 ready, 3 error, 5 preparing (TL, DOK). Type codes table in TL note. |
| Wait until source ready | `source_add(wait)`, `source_list(wait_seconds)` | TL | TL, AGM, ARL, JB, ICE, CW | polling `rLM1Ne` | RPC | verified | TL default 120 s, 1 s start; ARL caps 40 s because host clients time out near 60 s. Status 3 is hard failure only for known non-audio types (JB). |
| Read full text (plain / markdown) | `source_read(mode=fulltext)` | TL | TL, JB, ROO, ARL, AGM, JUL, ERS, M4Y | `hizoJc` `[[sid],[2],[2]]` (`[3],[3]` markdown/HTML) | RPC | verified | Paginate (TL 10k chars, ROO offset/limit). PHO deprecated it for "reliability issues". |
| Source guide (AI summary + keywords) | `source_read(mode=guide)` | TL | TL, JB, ICE, ARL, AGM, MOOD, ERS | `tr032e` `[[[[sid]]]]` | RPC | conflicting | M4Y `[sid, nb, [2]]`, LKA keys by notebook id (outliers). |
| Passage search over sources | `source_read(mode=search)` | TL | TL | `ASU5Oe` | RPC | single | Experimental. |
| Refresh / sync URL or Drive source | `source_update(refresh)` | TL | TL, JB, ICE, MOOD, AGM, M4Y, ERS, DOK | `FLmJqe` `[null,[sid],[2]]`; `dtT1F` batch | RPC | conflicting | Majority shape TL/JB/ICE/MOOD; AGM `[nb,sid,[2]]`, M4Y `[sid,nb]`, DOK `[sid]`. `dtT1F` works where `FLmJqe` is rejected (TL). |
| Freshness check | `source_list(check_freshness)` | TL | TL, JB, AGM, MOOD, ERS | `yR9Yof` `[null,[sid],[2]]` | RPC | verified | URL fresh = `[]`; Drive `[[null,bool,[id]]]` (TL). Skip by default to avoid N+1 (JBc). |
| Rename source | `source_update(rename)` | TL | TL, JB, ICE, ARL, MOOD | `b7Wfje` `[null,[sid],[[[title]]]]` | RPC | verified | JAC shape is wrong (outlier). |
| Delete source(s) | `delete(kind=source)` | TL | TL, JB, TMC, ICE, ROO, ARL (UI), AGM, MOOD, M4Y, ERS, DOK, PHO, WAN, KHE, PAV; enterprise | `tGMBJ`; enterprise `sources:batchDelete` (resource name uses singular `source`) | RPC, browser, enterprise | conflicting | `[[[sid]],[2]]` (ROO, ICE, MOOD, M4Y, ERS, PHO) vs `[[[sid]]]` (TL, DOK) vs `[nb,[sid],[2]]` (AGM). Notebook id goes in `source-path`. |

## Research / discover

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Discover sources (synchronous, ~8 s) | `research_start(mode=discover)` | TL | TL, TMC (id), SAH (id) | `Es3dTe` | RPC | conflicting | TL `[[query,1],null,mode,nb]` vs TMC `[%project_id%,%query%]` (no HAR evidence). `qXyaNe` (DOK, PHO) is wrong per TMC. |
| Fast research (web or Drive) | `research_start(mode=fast)` | TL | TL, JB, TMC (HAR-verified 2026-04-17), ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, WAN, PAV, STR, CW, PHO | `Ljjv0c` `[[query,1 web/2 drive],null,1,nb]` | RPC | verified | LKA sends `[nb,query]` (outlier). |
| Deep research (web only) | `research_start(mode=deep)` | TL | TL, JB, TMC, ICE, ARL, AGM, MOOD, M4Y, ERS, PAV | `QA9ei` `[null,[1],[query,1],5,nb]` | RPC | verified | Response `[a,b]`: TL "session id (unpollable), report id"; TMC "research id, conversation id"; JAC says inverse of fast. Pin by capture. Deep Research 10/month on Standard (TL quota doc). |
| Poll research | `research_status` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, WAN, PAV | `e3bVqc` `[null,null,nb]` | RPC | conflicting | Done = status 2 (TL: 1 running, 2 done, 3 no results, 4 cancelled) vs "2 or 6 done" (JB, ARL, MOOD). TMC says the id is polymorphic by args (also DeleteChatHistory). |
| Import research results | `research_import` | TL wire, ARL vetting ([note](notes/arlinamid__notebooklm-mcp.md)) | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, WAN, CW | `LBwxtb` `[null,[1],task,nb,[entries]]` | RPC | verified | Not atomic; rerun after partial commit gives FAILED_PRECONDITION(9) (TL); reconcile by diffing sources (JB). ERS sends empty entries (bug). |
| Cancel research | `job_cancel` | TL | TL, TMC, SAH | `Zbrupe` | RPC | verified | Always returns `[]`, no validation (TL). |

## Chat

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Ask with citations | `chat_ask` | TL (answer/citation model), TMC (streaming parser) | RPC: TL, JB, TMC, ICE, ROO, ARL, AGM, SAH, DOK, ERS, JUL, PHO (no citations: MOOD, M4Y, LKA, JAC); DOM: PP, PAN, PAO, TEN, ASC, WAN, IVA | `GenerateFreeFormStreamed` endpoint (JS id `laWbsf` per TMC) | RPC, browser | conflicting | 5-element vs 9-element params, source nesting depth, `x-goog-ext-353267353-jspb` header (see rpc-catalog "Chat"). Citations at answer type block `[4][3]`, source id at `passage[1][5][0][0][0]`, offsets in UTF-16 (JB, ROO, ARL, SAH). |
| Scope ask to chosen sources | `chat_ask(sources)` | ARL | ARL, TL, JB, TMC, SAH | same | RPC | verified | Always send explicit ids: unpinned questions get a server-chosen subset (TMC README). |
| Multi-turn follow-ups | `chat_ask(conversation_id)` | ARL, TL | TL, ARL, AGM, JB, ICE, M4Y, MFL, SAH; DOM tab sessions in PP family | stream + `hPTbtc` + `khqZz` | RPC, browser | conflicting | History order newest-first (ICE) vs oldest-first (JB, ARL, AGM, M4Y); real conversation id via `hPTbtc` after the first ask (TL, SAH) vs `first[2][0]` in the chunk (JB, AGM, ROO) vs `inner[10]` (M4Y). ROO RPC path is single-turn only. |
| Detached ask (start, poll) | `chat_ask(wait=false)` + `chat_status` | TL | TL (`chat_start/status/cancel`, FIFO, concurrency 3), JB (`notebook_query_start/status`, 8 in flight) | `oXwmh` session status (TL) | RPC | single | Needed because hosts time out near 60 s while chat may take 120-180 s (TL, JB, CW). |
| Cancel an answer in progress | `job_cancel` | TL | TL, TMC | `XgrPMd` `[null,conv]` | RPC | verified | Stream stays open client-side (TL). |
| Read / export conversation history | `chat_history` | ARL | ARL, TL, JB, MOOD, AGM, PAV; PAN via DOM | `hPTbtc`, `khqZz` | RPC, browser | verified | Turns newest first; `khqZz` cursor paging (ARL). |
| Delete conversation history | `delete(kind=chat_history)` | TL | TL, JB, ICE, ARL, MOOD | `J7Gthc` | RPC | conflicting | `[[],conv,null,1]` (TL, ICE, MOOD; ARL with header) vs `[nb,conv]` (JB). |
| Suggested prompts | `suggest(surface=chat)` | TL | TL, TMC, MOOD | `otmP3b` | RPC | verified | Mode 1..10, 4 = chat (TL); MOOD 8 quiz, 9 flashcards. |
| Grounded next-step questions | `suggest(surface=followups)` | TL | TL | `OcvKNc` | RPC | single | Also embedded in the answer row (TL). |
| Chat goal / persona / response length | `chat_configure` | TL | TL, JB, ARL, AGM, MOOD, STR, PAV; ERS (flat, wrong) | `s0tc2d` slot 7 `[[goal(,prompt)],[len]]` | RPC | verified | Goal 1 default, 2 custom (prompt up to 10000 chars), 3 learning guide; length 1 default, 4 longer, 5 shorter. M4Y tried `hT54vc` and got INVALID_ARGUMENT. |
| Chat-only view level for shared viewers | `share_update(view_level)` | TL | TL, AGM | `s0tc2d` slot 8 | RPC | single | AGM is a TL port (one family). |

## Notes

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| List notes and note-backed mind maps | `note_list` | TL | TL, JB, ICE, AGM, MOOD, ERS | `cFji9` `[nb]` | RPC | verified | Soft-deleted rows `[id,null,2]` must be filtered (TL, JB, ICE). |
| Create / update note | `note_save` | TL | TL, JB, ICE, AGM, MOOD; ROO/ARL via DOM | `CYK0Xb` then `cYAfTb` | RPC, browser | verified | Create ignores title/content; follow with update (TL, MOOD). |
| Save chat answer as note with `[N]` citation anchors | `note_save(from_turn)` | TL | TL; ARL via UI | `CYK0Xb` 7-slot form | RPC, browser | single | |
| Delete note | `delete(kind=note)` | JBc ([note](notes/jacob-bd__notebooklm-cli.md)) | TL, JB, ICE, AGM, MOOD | `AH0mwd` | RPC | conflicting | `[nb,null,[ids]]` (TL, JB, ICE, AGM) vs `[nb,null,[id],[2]]` (MOOD, JBc). Mind maps need a second `cFji9` call with the creation timestamp or ghosts remain (JBc). Delete is soft. |
| Convert note to source | `source_add(from_note)` | ARL (UI) | ARL, ROO (DOM) | none native; compose `cFji9` + `izAoDd` text | RPC (composed), browser | single | Proposal: read note content then add as text. |

## Studio artifacts

All RPC generation goes through `R7cb6c` (CreateArtifact) with a type code and a type-specific config slot; status via `gArtLc`. Generation is fire-and-return; poll with backoff (TL: 2 s to 10 s, 300 s default; audio/video several minutes).

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Audio overview: formats deep_dive/brief/critique/debate, lengths short/default/long, language | `studio_generate(kind=audio)` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, PHO, KHE, STU, PAV, CW; DOM PP, PAN; enterprise ROB/KD | `R7cb6c` type 1, config slot 6 `[null,[instr,len,null,src2,lang,null,format]]`; legacy `AHyHrd` (TMC, PHO); enterprise `POST audioOverviews` | RPC, browser, enterprise | verified | ROO found generation language follows the `hl` query param, payload lang ignored (issue #34); others rely on payload: send both. Enterprise accepts only `{}` (KD, ROB), one overview per notebook. |
| Video overview: explainer/brief/cinematic/short, visual styles | `studio_generate(kind=video)` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, KIS, KHE, STU | type 3, slot 8 `[null,null,[src2,lang,instr,null,format,style,style_prompt?]]` | RPC, browser | conflicting | Style enum disagrees: TL 1 auto 2 classic 3 whiteboard 4 heritage ... 9 kawaii; ICE 1 auto 3 classic 4 whiteboard 5 kawaii ...; KIS 1 auto 2 custom 4 heritage 9 kawaii. Cinematic needs AI Ultra (TL). ERS field order differs (outlier). |
| Mind map (note-backed JSON or interactive) | `studio_generate(kind=mind_map)` | TL | TL, JB, ROO, AGM, MOOD, ERS, KHE, STU (M4Y removed it) | `yyryJe` then `CYK0Xb` save; or `R7cb6c` type 4 variant 4 + `v9rmvd` tree at `[0][9][3]` | RPC | verified | `yyryJe` shape agreed by TL, JB, ROO, AGM, MOOD, PHO. |
| Report: briefing doc, study guide, blog post, custom, AI-suggested formats | `studio_generate(kind=report)` | TL | TL, JB, ROO, ARL, AGM, MOOD, M4Y, PAV, STU | type 2, slot 7 `[null,[title,desc,null,src2,lang,prompt,null,true]]`; suggestions `ciyUvf` | RPC | verified | Study guide is a report format (TL, JB, PAV). Report markdown at row `[7]` (TL) vs `[7][1][0]` (JB, ROO). |
| Quiz (quantity, difficulty) | `studio_generate(kind=quiz)` | TL | TL, JB, ROO, AGM, MOOD, M4Y, ICE | type 4, slot 9, variant 2 | RPC | conflicting | TL `[null,[2,null,instr,null,null,null,null,[qty,diff]]]`; M4Y `[count, difficulty]` vs flashcards `[difficulty, count]` asymmetry; ERS sends no options. `QuizQuantity.MORE` aliases standard (AGM). |
| Flashcards | `studio_generate(kind=flashcards)` | TL | TL, JB, ROO, AGM, MOOD, M4Y, ICE, KIS | type 4, slot 9, variant 1 (pair at index 6) | RPC | conflicting | Quiz and flashcards both return type 4; read the variant code (KIS). |
| Slide deck + per-slide revise | `studio_generate(kind=slide_deck)`, `studio_update(revise_slides)` | KIS ([note](notes/Kisara-k__notebooklm-kit.md)) live capture; TL | TL, JB, ROO, ARL, AGM, KIS, MOOD, M4Y, STU | type 8, slot 16 `[[instr,lang,format,length]]`; revise `KmcKPe` `[[2],artifact,[[[idx0,prompt]]]]` | RPC | conflicting | Slot 16 agreed (KIS live, ROO, TL, JB, AGM); PHO 13 and ERS 14 are wrong. Enum values disagree: TL format 1 detailed / 2 presenter, length 1 default / 2 short; KIS format 2 presenter / 3 detailed, length 2 short / 3 default. PDF and PPTX URLs in row `[16]` (TL `[16][3]/[16][4]`, JB/ROO `[16][0]/[3]`). |
| Infographic (orientation, detail, style) | `studio_generate(kind=infographic)` | KIS live capture; TL | TL, JB, ROO, ARL, AGM, KIS, MOOD, M4Y, STU | type 7, slot 14 `[[instr,lang,null,orientation,detail,X]]` | RPC | conflicting | X = style preset (TL) vs constant `1` "purpose unknown" (KIS, ROO); M4Y sends 5 elements. JB lists 11 style values. |
| Data table (CSV / XLSX) | `studio_generate(kind=data_table)` | TL | TL, JB, ROO, MOOD, STU (M4Y removed it) | type 9, slot 18 `[null,[instr,lang]]` | RPC | verified | ICE fixed a prompt-echo bug in CSV extraction. XLSX at row `[24]` (JB). |
| Interactive report (type 11) | `studio_generate(kind=interactive_report)` (experimental) | JB | JB | type 11, slot 34; `v9rmvd`, `Rytqqe` | RPC | single | Added JB v0.12.0. |
| List / poll artifacts | `studio_list` | TL | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, LKA, PHO | `gArtLc` `[ctx, nb, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | RPC, browser | conflicting | Request agreed except first element (`[2]` vs OPTS vs DefaultUserConfig) and M4Y omits filter. Status 3 ready, 4 failed agreed; 1/2 naming differs. Media positions disagree (video `[8][4]` TL vs `[8][3]` JB/ROO; MOOD reads a plain string at `[8]`). Audio/video ready only when a media URL exists (AGM). Failed artifacts carry no reason (TL). |
| Artifact content (quiz/flashcard HTML, mind-map tree, report markdown, data table) | `studio_list(artifact, include_content)` | TL | TL, JB, ARL, AGM, MOOD, ICE | `v9rmvd` `[artifact]`, row slots from `gArtLc` | RPC | verified | HTML at `[0][9][0]` (TL, JB, ARL). |
| Rename artifact | `studio_update(rename)` | TL | TL, JB, ARL, ICE, AGM, MOOD | `rc3d8d` `[[id,title],[["title"]]]` | RPC | conflicting | ICE `[id,title]`, AGM `[[2],nb,id,title]`. |
| Retry failed artifact | `studio_update(retry)` | TL | TL, TMC; JB uses the id to generate suggested interactive-report elements | `Rytqqe` | RPC | conflicting | Semantics differ (retry in place vs generate suggested). |
| Delete artifact | `delete(kind=artifact)` | ROO | ROO, JB, ICE, AGM, MOOD, ERS, PHO; ARL via UI; enterprise audio only | `V5N4be`; enterprise `DELETE audioOverviews/default` | RPC, browser, enterprise | conflicting | `[[2],id]` (ROO, JB, ERS, PHO) vs `[[2],nb,id]` (AGM) vs `[[2],nb,[id]]` (MOOD) vs `[DefaultUserConfig,id]` (ICE). M4Y removed it after INVALID_ARGUMENT. |
| Studio customization choices | `suggest(surface=studio_options)` | TL | TL, TMC, ICE | `sqTeoe` | RPC | conflicting | Payload `[OPTS]` (TL) vs `[ctx,null,mode]` (TMC) vs `[cfg,nb]` (ICE). |
| Copy artifacts to another notebook | not v1 | TL | TL | `mKDdke` | RPC | single | |

## Downloads and export

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Download audio (.m4a/.mp4), video (.mp4), infographic (.png), slides (.pdf/.pptx), report (.md), data table (.csv/.xlsx), quiz/flashcards (.json/.md/.html), mind map (.json) | `studio_export(target=file)` | JB ([note](notes/jacob-bd__gemini-notebook-mcp-cli.md)) | JB, TL, ARL, ROO, ICE, AGM, STU, CW, PAV (audio); DOM PP, PAN | media URLs from `gArtLc` rows; `v9rmvd` | RPC + plain HTTPS to googleusercontent, browser (CDP download) | verified | Drop OSID/`__Secure-OSID` before media fetch, mimic cross-site `Sec-Fetch-*` or CDN 403 (JB); follow redirects manually keeping Cookie (ROO, AGM); reject HTML bodies (ROO, ARL, AGM, ICE); retry CDN 404 shortly after generation (ICE). Prefer `audio/mp4` `-dv` variant (JB, ROO). Confine writes to an allowed root (JB, ARL, CW). |
| Export report / data table to Google Docs / Sheets | `studio_export(target=docs or sheets)` | TL | TL, JB, AGM, MOOD | `Krh3pd` `[null,artifact,content,title,1 docs/2 sheets]` | RPC | verified | Returns doc URL. |
| Download everything in a notebook | `studio_export(all=true)` | JB | JB (`download_all_artifacts`), TL (`--all`) | none | local orchestration | n/a | Incremental, `skip_existing`. |

## Sharing

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Share status (collaborators, public flag) | `share_status` | TL | TL, JB, ROO, ARL, AGM, MOOD, ICE, PAV, KHE | `JFMDGd` `[nb,[2]]` | RPC | verified | PHO calls it before `QDyure` "to initialize sharing state" (lead to verify). |
| Public link on/off | `share_update(public)` | TL | TL, JB, ROO, ARL, AGM, MOOD, ICE, PAV, KHE | `QDyure` `[[[nb,null,[v],[v,""]]],1,null,[2]]` | RPC | verified | Widening access needs confirmation (TL `needs_confirmation`, ARL elicitation). |
| Invite / change / remove collaborators (notify, message) | `share_update(users)` | TL | TL, JB, AGM, MOOD, ICE, KHE; enterprise `notebooks/{id}:share` (ROB; KD says removed from docs 2025-11-07) | `QDyure` perm 2 editor / 3 viewer / 4 remove | RPC, enterprise (uncertain) | verified | PHO user-share shape differs (outlier). |
| Artifact share / deep link (legacy audio share) | `share_update(artifact)` | TL | TL, AGM, TMC | `RGP97b` `[[1/0],nb(,artifact)]` | RPC | verified | |

## Labels and collections

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Source labels: list, AI auto-group, create, rename, emoji, add/remove sources, delete | `label_list`, `label_update`, `delete(kind=label)` | TL | TL, JB, ROO, TMC | `I3xc3c` list, `agX4Bc` create/auto, `le8sX` mutate, `GyzE7e` delete | RPC | verified | Autolabel may return DEADLINE_EXCEEDED(4) after ~60 s as normal (TMC). `le8sX` honors only the first id per call (TL). ROO 3.1.2 fixed "list called create RPC". |
| Notebook collections (groups of notebooks) | `label_list(scope=collections)`, `label_update(scope=collections)` | ARL, TL | TL, JB, ARL | same ids with trailing scope `3` | RPC | conflicting | JB lists collections by creating and deleting a temporary collection (race, avoid); TL and ARL list with `I3xc3c [OPTS,null,3]`. |
| Local aliases and tags for notebooks | `notebook_update(alias, tags)` | JB | JB (`alias`, `tag`), PP/ASC/WAN (library JSON) | none | local | n/a | Feeds the resolver and `notebook_list(tag)`. |

## Account, quota, settings

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Tier, notebook/source/word limits | `server_info(include_usage)` | TL | TL, ICE, ARL, M4Y | `ZwVcOc` (`source-path=/`) | RPC | conflicting | Many payload variants (`[OPTS]`, `[null,[1,...,[1]]]`, `[null,1]`, `[header]`); limits at `[0][1]`, language `[0][2][4][0]` (TL). ICE: quota is GetOrCreateAccount, not GetQuota. |
| Live 5-hour and weekly usage windows | `server_info(include_usage)` | TL | TL, JB, ARL | `EylDcb` | RPC | conflicting | `[null]` (TL) vs `[[2,null,[1],[1,null*9,[1,3]]]]` (JB) vs `[header]` (ARL); windows unordered (JB). |
| Output language (account default) | `account_settings` | AGM, TL | TL, ICE, AGM, ARL | `ZwVcOc` get, `hT54vc` set | RPC | conflicting | Nesting depth of the set payload differs (TL vs ICE/AGM vs ARL). |
| Client-side quota guard (50 chats/day free etc.) | internal, reported in `server_info` | PAN | PAN, PHO, ROO, ASC | none | local | n/a | Figures are upstream observations, not Google docs; prefer live `EylDcb` (TL). |

## Auth and multi-account

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Multi-account profiles and `authuser` routing | `profile` param on every tool; `auth_login(profile)` | TL, JB, TMC | TL (profiles, `NOTEBOOKLM_MCP_PROFILES`), JB (profiles, `profile` tool), TMC (identities, `--authuser`), PHO, ROO (rotation pool), PP (`--account`), KIS | `authuser` URL param + `x-goog-authuser` header (omit for account 0, TMC) | RPC | verified | Concurrent users of one jar sign each other out (TL): one profile per worker. NOT_FOUND / PERMISSION_DENIED should hint "wrong account index" (SAH). |
| Account rotation on quota exhaustion | not planned | ROO | ROO | none | local | single | ToS-adjacent; see decisions-needed. |
| Login in a dedicated browser profile, harvest cookies + tokens over CDP | `auth_login(method=browser)` | TMC ([note](notes/tmc__nlm.md)) | TMC, JB, M4Y, JUL, FAX, ICE, ARL/PP (persistent profile), LKA, DOK (rod), MOOD | none | browser | verified | Chrome 136+ ignores debug flags on the default profile (performance.md s6). Wait for signed-in cookies + CSRF, not URL alone (ERS, PAO). |
| Attach to an already running Chrome over CDP | `auth_login(method=attach)` | LKA, TMC | LKA, TMC (`-cdp-url`), FAX, KAF, JB | none | browser | verified | Avoids cookie-DB decryption; KAF found profile copy fails with current Chrome encryption. |
| Paste / import cookies (cURL, Cookie header, JSON, Playwright storage_state, Netscape cookies.txt) | `auth_login(method=import)` | PAV, WIN | PAV, WIN, JB (`--manual`), M4Y, TL (`import-cookies`), MIS (Netscape), SAH, LKA, DOK | none | local | verified | Only path for HttpOnly cookies without a browser (PAV). Require SID + `__Secure-1PSIDTS` (TL). |
| Firefox / Safari cookie import | `auth_login(method=firefox)` (pending decision) | MIS (behavior only, GPL) | MIS, FAX (browser_cookie3), TL (rookie) | none | local | single | Firefox store is unencrypted on all OSes; schema 16+ stores expiry in ms (MIS). |
| Chromium cookie-DB decryption | not planned | DOK (macOS) | DOK, FAX, STU, TL (rookie) | none | local | n/a | Windows App-Bound Encryption blocks it (MIS README, KAF). |
| Cookies via env for headless / CI / Docker | config `NOTEBOOKLM_COOKIES` etc. | M4Y | JB, MOOD, M4Y, TL (`NOTEBOOKLM_AUTH_JSON`), TMC, ICE | none | local | verified | Env pins cookies and disables persistence (MOOD). |
| Cookie keepalive via RotateCookies | internal | TL | TL, JB, MOOD | `POST accounts.google.com/RotateCookies` | HTTP | conflicting | TL: returns fresh `__Secure-1PSIDTS`; JB issue #316: from plain httpx only `*SIDCC` refresh, aged sessions need a browser. |
| Signaler `refreshCreds` keepalive | internal (off by default) | TMC | TMC, PHO, KAF | `signaler-pa.clients6.google.com/punctual/v1/refreshCreds` | HTTP | conflicting | Body token differs (TMC `["tZf5V3ry"]`, PHO `["8hcNI0LV"]`); does not mint cookies; effect unverified. |
| Headless browser re-mint on the saved profile | auth recovery rung | JB | JB, TL (L3), M4Y (visible window), FAX | none | browser | verified | |
| Operator refresh hook command | config `NOTEBOOKLM_REFRESH_CMD` | TL | TL, WIN | none | local | verified | No shell, timeout, one attempt per window (TL ADR-0030). |
| Master-token (gpsoauth) headless login | not planned | TL | TL | Android OAuth | none | single | Full-account credential; ToS risk (TL note). |
| Enterprise auth (ADC / gcloud / env token) | enterprise transport | ROB, KD | ROB, KD | OAuth bearer | enterprise | verified | Use `golang.org/x/oauth2/google` instead of shelling to gcloud (ROB, KD Avoid). |

## Library, batch, surfaces

| Feature | MCP tool | Best upstream | Supported by | RPC id(s) | Transports | Confidence | Notes |
|---|---|---|---|---|---|---|---|
| Notebook metadata cache (TTL + stale-while-revalidate, write-through invalidation) | internal, `refresh` param | STR resolver; performance.md s5 | STR (60 s TTL), JB (conversation LRU, auth 30 s), MOOD (cached client) | none | local | n/a | ROO's per-call homepage GET is the anti-pattern. |
| Library injected into tool description / server instructions | server instructions | PP, ASC | PP, ASC, STR (dynamic instructions) | none | local | n/a | With RPC we can inject real titles instead of a manual library. Keep it short (PAO 120-line description is the anti-pattern). |
| Cross-notebook ask (fan-out, aggregated citations) | `chat_ask_many` (optional group) | JB | JB (`cross_notebook_query`, `batch`) | stream | RPC | single | Off by default. |
| Batch add many sources in one RPC | `source_add(items[])` | TL | TL (`izAoDd` multi-spec, `X1snv` async ~0.65 s), CW (bounded concurrency 4) | `izAoDd`, `X1snv` | RPC | single | Per-item failures are silently omitted unless all fail (TL): reconcile by listing. Report requested/succeeded/failed (CW). |
| Bulk create notebooks / pipelines / content workflows | not planned (skills) | JB, CW | JB (`batch`, `pipeline`), CW (trend pipelines), PAN (DOM) | none | local | n/a | Keep out of the server (CW Avoid). |
| Local REST/HTTP API | not v1 (decision) | ROO | ROO (Express, 33 routes, 0.0.0.0 + CORS `*`), TL (FastAPI server) | none | n/a | n/a | See decisions-needed. |
| Streamable HTTP MCP transport | `--http` flag, loopback + token | CW, PAO | PP, JB, TL, ARL, PAO, CW, PAV (0.0.0.0, avoid), KHE | none | n/a | verified | Validate Origin, bind 127.0.0.1 (performance.md s1). |
| CLI | `notebooklm-mcp login/doctor/status/call` | TL, TMC | TL, JB, TMC, ICE, DOK, MIS, LKA, ERS, KD, M4Y | none | n/a | n/a | Scope is a decision. |
| Agent skill file | ship `SKILL.md` | TL, JB | TL, JB, DOK, ICE, CW, WIN, ARL | none | n/a | n/a | |
| Doctor / diagnose | CLI `doctor` + `server_info` | KD | KD, TL, ICE, JB, JUL | none | n/a | n/a | Named checks with exit codes (KD). |
| Tool groups / profiles to cut `tools/list` tokens | config `tools.profile`, `tools.disabled` | JB, PP | JB (groups), PP, ASC, WAN (profiles), PAN | none | n/a | n/a | |
| One backend shared by several MCP clients on one profile | optional | PAO, ARL | PAO (singleton backend + handshake replay), ARL (leader/follower), ROO (stdio-to-HTTP proxy) | none | n/a | n/a | Two Chromes on one Google session may revoke it (ARL). Needed only for the browser path. |
| Confirmation for destructive actions | `confirm` param + MCP elicitation | ARL | ARL, TL, JB, MOOD, ERS, CW | none | n/a | n/a | |
| Progress notifications for long jobs | MCP progress | TMC, PAO | TMC (`watch_deep_research`), PAO (15 s heartbeat), ARL (queue position), PP | none | n/a | n/a | Only when the client sends `progressToken` (ASC uses request id: spec deviation). |
| MCP "tasks" utility | not v1 | ARL | ARL | none | n/a | single | go-sdk v1.8.0 support unverified (performance.md s1). |
| Live voice (WebRTC), Guidebooks, featured notebooks | not planned | TMC | TMC, PHO (ids only) | `Of0kDd`, `eyWvXc`, Guidebook ids | RPC | single | |

## Proposed MCP tool surface

Principles (from STR, TL ADR-0025, JB tool groups, CW, ARL): task-shaped tools, typed enums in the JSON schema (no JSON-string `options`, STR Avoid #1), every notebook/source/artifact parameter accepts id, title, or unique prefix, fire-and-return for slow jobs, one destructive tool so hosts can gate it, structured error envelope `{status, error, code, retriable, guidance, suggestions, transport}`.

Common optional params on every Google-touching tool: `profile` (account profile name), `refresh` (bypass cache where reads are cached). All handlers pass the MCP request `ctx` into every outbound call.

33 tools; 32 in the default profile (`chat_ask_many` is in the opt-in `batch` group). A `minimal` profile of 8 tools is marked M.

| # | Tool | Params | Behavior | readOnly | destructive | idempotent |
|---|---|---|---|---|---|---|
| 1 | `server_info` M | `include_usage?`, `include_profiles?` | Version, active profile, host in use, auth health (no network unless asked), last transport that served a call, per-transport capability list, tier/limits (`ZwVcOc`) and live usage windows (`EylDcb`) when `include_usage`. | yes | no | yes |
| 2 | `auth_login` | `method` (browser, attach, import, firefox), `profile?`, `cdp_url?`, `cookies?` (pasted cURL / header / JSON), `file?`, `timeout_s?` | Creates or refreshes a profile's session. `browser` opens a dedicated profile and waits for real sign-in; `attach` reads cookies from a running Chrome over CDP; `import` parses pasted or file cookies; validates with a real `wXbhsf` call before saving (MOOD). | no | no | no |
| 3 | `auth_logout` | `profile`, `confirm` | Deletes stored cookies/tokens for a profile (and optionally its browser profile dir). | no | yes | yes |
| 4 | `notebook_list` M | `query?`, `scope?` (mine, shared, all), `tag?`, `limit?`, `offset?` | Lists notebooks (`wXbhsf`) merged with local alias/tags; fuzzy filter by title. Cached 30 s with stale-while-revalidate. | yes | no | yes |
| 5 | `notebook_open` M | `notebook`, `include?` (summary, sources, settings, sharing) | Composite read: `rLM1Ne` + `VfAZjd` in parallel, source digests, chat settings, suggested questions; degrades per sub-call (STR `open_notebook`). | yes | no | yes |
| 6 | `notebook_create` | `title`, `emoji?`, `copy_from?`, `sources?` (same item schema as `source_add`) | `CCqFvf` or `te3DCe`, then optional source adds; returns the new id and per-source outcomes. Sent once, never auto-retried. | no | no | no |
| 7 | `notebook_update` | `notebook`, `title?`, `emoji?`, `pinned?`, `remove_from_recent?`, `alias?`, `tags?` | `s0tc2d` / `LQhfEb` / `fejl7e` plus local alias/tags; read-back verification after each mutation (ARL). | no | no | yes |
| 8 | `source_add` M | `notebook`, `items[]` (each one of `url`, `text{title,content}`, `file{path}`, `drive{document_id,mime}`, `copy_from{notebook,source}`, `from_note`), `wait?`, `timeout_s?` | Batches URL/text into one `izAoDd` call, files via `o4cbdc` + resumable upload, YouTube auto-detected; on codes 3/9 reconciles by listing before any retry; returns requested/succeeded/failed per item. SSRF and path checks first. | no | no | no |
| 9 | `source_list` | `notebook`, `status?`, `label?`, `check_freshness?`, `wait_seconds?` | Sources with type, status, label; optional `yR9Yof` freshness; optional wait until all ready (progress notifications). | yes | no | yes |
| 10 | `source_read` M | `notebook`, `source`, `mode` (guide, fulltext, markdown, search), `query?`, `offset?`, `max_chars?` | `tr032e` guide, `hizoJc` text (paged), or `ASU5Oe` passage search (experimental). | yes | no | yes |
| 11 | `source_update` | `notebook`, `source`, `title?`, `refresh?` | `b7Wfje` rename, `FLmJqe` refresh (fallback `dtT1F`). | no | no | yes |
| 12 | `research_start` | `notebook`, `query`, `mode` (discover, fast, deep), `corpus?` (web, drive) | `Es3dTe` (sync, returns candidates) or `Ljjv0c` / `QA9ei`; returns a job handle. Rejects vague queries (<4 words, ARL policy) unless `force`. | no | no | no |
| 13 | `research_status` | `notebook`, `job?`, `wait_seconds?`, `include_report?` | `e3bVqc` poll with backoff and progress; returns candidates with reliability hints and the deep report. | yes | no | yes |
| 14 | `research_import` | `notebook`, `job`, `select?` (indices) or `cited_only?`, `max_sources?` | `LBwxtb`; pre-filters URLs already in the notebook so a retry is safe; reports partial commits. | no | no | no |
| 15 | `chat_ask` M | `notebook`, `question`, `sources?`, `conversation_id?`, `new_conversation?`, `references?` (none, lite, full), `wait?` (default true) | Streamed ask with incremental parsing; progress notifications carry partial text; returns answer, citations (source title, cited text, UTF-16 range), conversation id (resolved via `hPTbtc`), follow-ups. `wait=false` returns a job handle. Never retried after send. | no | no | no |
| 16 | `chat_status` | `job` | Status and result of a detached ask (`oXwmh` plus local job table). | yes | no | yes |
| 17 | `chat_history` | `notebook`, `conversation_id?`, `limit?`, `cursor?`, `format?` (json, markdown) | `hPTbtc` + `khqZz` with cursor paging and citations. | yes | no | yes |
| 18 | `chat_configure` | `notebook`, `mode?` (default, learning_guide, concise, detailed, custom), `custom_prompt?`, `response_length?` (default, longer, shorter) | `s0tc2d` slot 7, merges partial settings (TL). | no | no | yes |
| 19 | `suggest` | `notebook`, `surface` (chat, followups, report, studio_options), `sources?`, `query?` | `otmP3b`, `OcvKNc`, `ciyUvf`, `sqTeoe`. | yes | no | yes |
| 20 | `note_list` | `notebook`, `include_mind_maps?` | `cFji9`, tombstones filtered. | yes | no | yes |
| 21 | `note_save` | `notebook`, `note?`, `title?`, `content?`, `from_turn?` (conversation id + turn) | Create (`CYK0Xb` then `cYAfTb`) or update; `from_turn` keeps `[N]` citation anchors. | no | no | no |
| 22 | `studio_generate` M | `notebook`, `kind` (audio, video, mind_map, report, quiz, flashcards, slide_deck, infographic, data_table, interactive_report), `sources?`, `instructions?`, `language?`, typed per-kind object (`audio{format,length}`, `video{format,style,style_prompt}`, `report{format,template}`, `quiz/flashcards{quantity,difficulty}`, `slide_deck{format,length}`, `infographic{orientation,detail,style}`, `mind_map{interactive}`), `wait?` | `R7cb6c` (or `yyryJe`+save for note-backed maps); fire-and-return with artifact id and `estimated_wait`; optional pre-flight quota check (`EylDcb`). Sets `hl` to the content language. | no | no | no |
| 23 | `studio_list` M | `notebook`, `artifact?`, `kind?`, `status?`, `include_content?`, `wait_seconds?` | `gArtLc` list or one artifact with content (`v9rmvd`); `wait_seconds` polls 2 s to 10 s with progress. | yes | no | yes |
| 24 | `studio_update` | `notebook`, `artifact`, `action` (rename, retry, revise_slides), `title?`, `slides?[{index,instruction}]` | `rc3d8d`, `Rytqqe`, `KmcKPe`. | no | no | no |
| 25 | `studio_export` | `notebook`, `artifact?` or `all?`, `target` (file, google_docs, google_sheets), `format?`, `output_dir?`, `overwrite?` | Downloads into an allowed root (no symlinks, no overwrite unless asked, CW) or `Krh3pd` export; returns paths or doc URL. | no | no | no |
| 26 | `share_status` | `notebook` | `JFMDGd`. | yes | no | yes |
| 27 | `share_update` | `notebook`, `public?`, `view_level?` (full, chat_only), `users?[{email, role: viewer/editor/remove}]`, `notify?`, `message?`, `artifact?`, `confirm?` | `QDyure` / `s0tc2d` slot 8 / `RGP97b`. Widening access or removing people requires `confirm` or elicitation. | no | yes | yes |
| 28 | `label_list` | `notebook?`, `scope` (source_labels, collections) | `I3xc3c`. | yes | no | yes |
| 29 | `label_update` | `notebook?`, `scope`, `action` (auto, create, rename, set_emoji, add_sources, remove_sources, add_notebooks, remove_notebooks), `label?`, `name?`, `emoji?`, `items?` | `agX4Bc` / `le8sX`; long autolabel treated as pending, not failure. | no | no | no |
| 30 | `account_settings` | `output_language?` | Reads, or sets when given, the account output language (`ZwVcOc` / `hT54vc`). | no | no | yes |
| 31 | `job_cancel` | `job` | Cancels a detached ask (`XgrPMd`), research (`Zbrupe`) or local wait. | no | yes | yes |
| 32 | `delete` | `kind` (notebook, source, note, artifact, chat_history, label, collection), `notebook?`, `ids[]`, `confirm` | The only general destructive tool: `WWINqb`, `tGMBJ`, `AH0mwd` (+ mind-map ghost cleanup), `V5N4be`, `J7Gthc`, `GyzE7e`. Returns a preview unless `confirm=true` (or elicitation accepted); invalidates caches. | no | yes | yes |
| 33 | `chat_ask_many` (batch group, off by default) | `question`, `notebooks?[]`, `tag?`, `max_parallel?` | Fan-out ask with a concurrency cap, per-notebook citations, partial results. | no | no | no |

Annotation notes:
- `openWorldHint=true` on `source_add`, `research_start` (they make Google fetch the open web) and `share_update` (affects other people); `false` elsewhere. Proposal, not taken from an upstream (TMC sets all false).
- `readOnly` tools are the only ones eligible for singleflight and caching; none of them writes files.
- Tools absent from the enterprise capability list (chat, studio except audio, notes, research, labels) return a typed `UNSUPPORTED_BY_TRANSPORT` error instead of being hidden per call (ROB Avoid #1); `server_info` lists them.
- MCP resources (optional): `notebooklm://notebooks` (cached list) and resource links for downloaded files instead of inlining large bodies (performance.md s1 "Structured output").
