# RPC catalog

Date: 2026-10-14. Union of every RPC id and endpoint found in `research/notes/*.md`. All facts are reverse-engineered third-party claims recorded in those notes, not Google documentation and not verified live by us. Upstream codes and wire families are defined in [feature-matrix.md](feature-matrix.md#how-to-read-this).

## Status legend and counts

- **consensus**: at least 2 independent wire families implement the same id with the same request shape (response details may still need fixtures).
- **conflicting**: families disagree on request shape, response positions or semantics. Must be pinned by live capture before we ship it (see checklist at the end).
- **stale**: legacy, dead, or shown wrong by another upstream.
- **single-source**: one family only, or only declared (named) without a working call.

Counts over the 108 distinct batchexecute ids below: **31 consensus, 23 conflicting, 11 stale, 43 single-source/declared-only**. Not counted as ids: the streamed chat endpoint (its params are conflicting), the Scotty upload endpoint, RotateCookies, signaler `refreshCreds`, Enterprise REST and the Android gRPC service.

`OPTS` below = TL's request-options block `[2,null,null,[1,null*9,[1]]]` (MOOD calls it `templateBlock()`). `HDR` = TMC/ARL block `[2,null,[1],[1,null*9,[1,3]]]`. `FLAT` = `[2],[1,null*9,[1]]` as two trailing elements. `src2` = `[[sid],...]`, `src3` = `[[[sid]],...]`.

## 1. Endpoint, query params, headers, tokens

**batchexecute endpoint.** `POST https://{host}/_/LabsTailwindUi/data/batchexecute`. Path unchanged by the Gemini Notebook rebrand (JB note "rebrand" section; KIS `bc32922`; MOOD). Enterprise web UI uses `{host}{prefix}/_/CloudNotebookLmUi/data/batchexecute` (JB `base.py:215-217`, not live-validated).

**Query params** (all families agree on `rpcids`, `rt=c`):

| Param | Value | Who sends it | Notes |
|---|---|---|---|
| `rpcids` | the RPC id (comma-joined for multi-RPC) | all | TMC sends only the first id even for multi-RPC envelopes (Avoid). PHO supports comma-joined ids. |
| `source-path` | `/notebook/{id}` for notebook-scoped calls, else `/` | TL, JB, TMC, ROO, ARL, ICE, AGM, ERS, PHO | ERS: notebook calls failed until it was added (`60af55b`). M4Y omits it entirely and works for its calls: tolerant, but send it. |
| `f.sid` | page `FdrFJe` | all except where missing token | Rotates: M4Y updates it from `["af.httprm",?,"<sid>"]` frames. PHO and TMC hard-code stale fallbacks (Avoid). |
| `bl` | page `cfb2h` build label | JB, TMC, ICE, M4Y, MOOD, ROO, JAC, PHO | Not sent by TL, AGM, DOK, LKA, MFL, SAH, WAN. TL: "server does NOT validate it", a fabricated `_19700101.00_p0` still answered (`_env.py:113-122`). JBc: weeks-old `bl` made mutations fail with 400 (TECHNICAL_DEEP_DIVE s3). Send the live value; never hard-code. |
| `hl` | UI language | most | ROO: drives generation language for Studio (issue #34). |
| `_reqid` | counter | TMC (`1500000000+rand`, +100000), ICE (+jittered), M4Y (1,2,3...), PHO | Not sent by TL, JB, ROO, ARL, AGM, DOK on batchexecute. Optional. |
| `authuser` | account index or email | TL, TMC, PHO | Omit for account 0 (TMC `1ef43ba`); pair with header `x-goog-authuser`. |
| `rt=c` | chunked | all | Without it the server returns a plain JSON array (TMC `batchexecute.go:180`). |

**Body.** `f.req=<urlencode([[[rpcId, JSON.stringify(params), null, "generic"]]])>&at=<SNlM0e>&` with trailing `&` (TL `encoder.py:14-47`, JB `base.py:685-701`, TMC `batchexecute.go:196-205`, all others). TMC encodes spaces as `%20` like `encodeURIComponent`. M4Y puts `at` first and adds `f.sid` to the body: tolerated.

**Headers.**

| Header | Senders | Evidence it is needed |
|---|---|---|
| `Content-Type: application/x-www-form-urlencoded;charset=UTF-8` | all | required (streamed chat is stricter, JB `conversation.py:521-523`) |
| `Cookie` | all | required |
| `Origin`, `Referer` = the host in use | JB, TMC, ROO, ICE, MOOD, M4Y, ERS, DOK | AGM, LKA, WAN omit them and still work per their authors; must match the host after the rebrand (KIS). |
| `X-Same-Domain: 1` | JB, TMC, ROO, ARL, ICE, MOOD, M4Y, ERS, DOK, SAH | ERS got 405 until it was added (`6e8f16e`, together with a path fix); AGM omits it. Send it. |
| `X-Goog-Csrf-Token` | JB, ROO | MOOD omits it, works. Optional. |
| `Authorization: SAPISIDHASH <ts>_<sha1(ts SAPISID origin)>` | TMC, ERS (also signaler calls TMC, PHO) | AGM and most others omit it. Optional; origin must be the current host. |
| `x-goog-authuser` | TL (uploads), TMC (non-default account), JAC | |
| Chrome client hints, UA | ICE, M4Y, JUL, MOOD, ROO | No evidence they matter; a fixed fake UA is itself a fingerprint (ICE Avoid). Use one consistent UA. |

**Token acquisition.** `GET https://{host}/` with the cookie jar (do not follow redirects blindly: a 3xx to `accounts.google.com` = signed out, LKA `CheckRedirect`, JB `base.py:1290-1294`). Regex the `WIZ_global_data` HTML for:
- `SNlM0e` -> `at` (CSRF). Tolerate double-quoted, single-quoted, `\x22`-escaped and HTML-escaped forms (TL `extraction.py:86-125`, ERS `auth-headers.ts:178-182`). Never fall back to `FdrFJe` as CSRF (M4Y bug).
- `FdrFJe` -> `f.sid`.
- `cfb2h` -> `bl` (JB, TMC, ROO, ICE, M4Y; JAC adds a generic `"bl"` fallback).
- `oPEP7c` -> account email (JB `cdp.py:1448-1463`).
- `<html lang>` -> `hl` (ICE, MIS).

A signed-out landing page has no tokens; TL v0.8.5 probes `/login` to tell "signed out" from "page drift" (TL CHANGELOG). MIS claims the dashboard GET is bound to a Chrome TLS fingerprint and plain `net/http` gets `302 accounts.google.com/CookieMismatch` [unverified; TMC, LKA, DOK fetch with stdlib HTTP]. Detect `CookieMismatch` explicitly.

## 2. Response framing

- Prefix `)]}'` (XSSI guard) then `rt=c` chunks: a decimal length line, then one JSON line, repeated (TL `decoder.py:272-330`, TMC `chunked.go`).
- **Declared lengths are unreliable for byte slicing.** TL: off by 2 on live rejection bodies, "likely UTF-16 code units" (`rpc-reference.md:3297-3300`). TMC: the real unit is UTF-16 code units (JS string length); its strict parser tries bytes, runes and UTF-16 plus a continuation fallback and requires `json.Valid` (`wrbfr.go:16-148`). ERS tried byte slicing and reverted (`a3abf46`, `4e20855`). performance.md s3: split on structure, not lengths.
- **Recommended Go decoder** (TMC primary design): strip prefix in contiguous or char-per-line form (`wire.go:285-302`); a bare-digits body is a numeric error code (`chunked.go:29-36`); escape raw U+0000-U+001F inside strings because titles/notes carry literal newlines (`unescape.go:101-143`); extract top-level balanced `[...]` with a quote-aware scanner (`chunked.go:138-174`); fall back to the UTF-16-length-aware splitter; count malformed chunks and fail above 10 percent (SAH, AGM).
- **Frames**: `["wrb.fr", rpcId, "<inner JSON string>", null, null, status, "generic"|index]`. Payload at `[2]` is JSON-in-JSON, unescape up to 3 layers (TMC `unescape.go:19-60`). `["er", id, code]` = server error (ROO, AGM, DOK). Trailers `["di", ms]`, `["af.httprm", ...]` (carries rotated `f.sid`, M4Y), `["e",4,null,null,141]` (DOK) are metadata.
- **Several frames for one id**: in `rt=c` the server may send a null placeholder, then the payload; last non-null frame wins; `er` and UserDisplayableError are terminal (SAH `decoder.go:146-185`).
- **Position-5 status = gRPC code**, e.g. `["wrb.fr","cFji9",null,null,null,[3],"generic"]` is INVALID_ARGUMENT with HTTP 200 (DOK `decoder_test.go:7-21` pins this as a bug: it decoded as success). Codes seen: 3 INVALID_ARGUMENT, 4 DEADLINE_EXCEEDED (autolabel, normal), 5 NOT_FOUND, 7 PERMISSION_DENIED, 8 RESOURCE_EXHAUSTED (quota), 9 FAILED_PRECONDITION (polysemic: source cap ~300, oversize, malformed, busy, research re-import), 13 INTERNAL (`fejl7e` success), 16 UNAUTHENTICATED (TL, TMC `errors.go`, JB, ROO, MOOD). Private numbers: 277566/277567 auth, 80620 access denied, 324934 rate limit, 143 not found (TMC `errors.go:10-340`).
- **`google.rpc.Status`** `[code, message, details[]]` at `[5]`; quota rejections carry a `UserDisplayableError` detail and an empty `message`, so the client must author the text (TL `rpc-reference.md:3283, 3302-3315`; SAH `chat.go:461-482`; AGM substring match is the weak form).
- **Missing id but other ids present** = drift (JB `RPCDriftError`, ROO `RpcDriftError`, AGM `collectRPCIds`). **No frames at all** = login or anti-bot page (SAH). **Null with no status** = legitimate null only for RPCs whose null policy allows it (SAH three policies: NullIsError, NullAllowed, NullAllowedUnlessRejected; AGM `allowNull`; MFL `allow_null`).
- Accepted-pending: codes 3 or 9 on add-source and research import may mean the write happened; reconcile by listing (JB `sources.py:431-470`, `research.py:380-405`).

## 3. Catalog by area

### Notebooks

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `wXbhsf` | ListRecentlyViewedProjects | `[null,1,null,[2]]` | `[0]` rows `[title, sources, id, emoji, null, meta]`; meta[0] 1 mine/2 shared, meta[5] modified, meta[8] created `[s,ns]` | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, ERS, JUL, DOK, MFL, PHO | LKA `[]`, JAC `[null,2]` (outliers). TMC 2024 fixture shows `[[id], title, ...]` row layout; MFL titles may carry `thought\n` prefix | consensus |
| `CCqFvf` | CreateProject | `[title,null,null,...tail]` | id at `[2]`; ICE `[11][0][0]` default chat thread | all families | tail FLAT (JB, ROO, ARL, ERS, M4Y) vs `OPTS` (TL, MOOD) vs `[2],[1]` (AGM, DOK, LKA) vs `[title, emoji]` (TMC proto); PHO 8 nulls (bug); ICE sends `""` title | conflicting |
| `rLM1Ne` | GetProject (+ sources, chat settings at `[7]`) | `[nb,null,[2],null,0]` | `[0]=[title, sources, id, ...]`; source `[[sid], title, meta, [null,status]]` | JB, ROO, ARL, AGM, ERS, JUL, DOK, WAN | nested `OPTS` instead of `[2]` (TL, MOOD "safe on both cohorts", SAH "flat rejected"); last arg `1` (ICE, PHO "poll source processing"); TMC `[id,null,ctx,null,include_artifacts,filter]`; LKA `[id]` | conflicting |
| `s0tc2d` | MutateProject | rename `[nb,[[null,null,null,[null,title]]]]`; chat config slot 7 `[nb,[[null*7,[[goal(,prompt)],[len]]]]]`; view level slot 8 `[[0|1]]` | project row | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y | ERS flat `[goal,len,prompt]` (outlier) | consensus |
| `WWINqb` | DeleteProjects | `[[nb],[2]]` | empty | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, ERS, DOK | batch shapes rejected (TL); bare `[nb]` fails code 3 (M4Y live); LKA `[nb]` | consensus |
| `fejl7e` | RemoveRecentlyViewedProject | `[nb]` | none; may be INTERNAL(13) on success | TL, AGM, TMC | | consensus |
| `te3DCe` | CopyProject | `[OPTS, src_nb, title]` | project row | TL, TMC | never retried (no idempotency token) | consensus |
| `VfAZjd` | GenerateNotebookGuide | `[nb,[2]]` | `[[ [summary], [[question,prompt]..] ...]]` | TL, JB, TMC, ARL, AGM, MOOD, ERS | M4Y `[nb,null,[2]]`; LKA `[nb]` | consensus |
| `LQhfEb` | pin / UpdateProjectUserState | `[HDR, nb, [null,[1|0]], [["notebook_lm_state.is_pinned"]]]` | `[0][1][0]` bool | ARL (TMC name only) | | single-source |
| `ub2Bae` | ListFeaturedProjects | `[[ctx]]` | | TMC | | single-source |
| `AUrzMb` | GetProjectAnalytics | | | TMC | | single-source |
| `rG2vCb` | list notebooks, Enterprise web UI | `["projects/P/locations/L",null,null,1]` | wXbhsf layout | JB (not live-validated) | | single-source |

### Sources

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `izAoDd` | AddSources (URL, YouTube, text, Drive; batch of specs) | `[[spec...], nb, ...tail]`; spec is 11 slots: URL `[null,null,[url],null*7,1]`, YouTube URL at slot 7, text `[null,[title,content],null,2,null*6,1]`, Drive slot 0 `[[id,mime,1,title],null*9,1]` | `[[ [[sid], title, ...] ]]`; per-item failures silently omitted unless all fail (TL) | spec slots: TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y, JUL, SAH | tail FLAT (JB, ROO, ARL, ICE, M4Y "works", JUL) vs `OPTS` (TL, MOOD, SAH "flat rejected since Gemini-3.5 rollout", offline audit) vs `[2],null,null` (AGM, DOK) vs none (TMC short form, ERS); Drive keeps FLAT (TL, MOOD, SAH); LKA slot 8 (outlier) | conflicting |
| `X1snv` | AddSourcesAsync | same as `izAoDd` | `[[Source...],null,[[Source,ack]...]]`, ~0.65 s | TL | | single-source |
| `ozz5Z` | disputed | ROO/ARL/JB: `[[[[null,url,627],[null*9,[null,null,1]],1]]]` (nb only in source-path) | ROO: same as izAoDd; ARL: none, ids found by re-listing | ROO, ARL, JB use it as "add URL v2"; JB also as entitlement tier with `"1"` | names: SUBSCRIPTION (M4Y, JAC), GET_UI_CONFIG (ICE, MIS), LogEvent (TMC), "after slide deck" (PHO) | conflicting |
| `o4cbdc` | AddTentativeSources (register upload) | `[[[filename]], nb, ...tail]` | first string = SOURCE_ID | TL, JB, TMC, ICE, AGM, DOK, WAN | tail FLAT (TMC, ICE, JB, AGM, DOK) vs OPTS (TL); PHO `[[[name,13]],...]` | conflicting |
| `tGMBJ` | DeleteSources | notebook id in `source-path` only | empty | id: all | `[[[sid]],[2]]` (ROO, ICE, MOOD, M4Y, ERS, PHO, JB) vs `[[[sid]]]` (TL, DOK) vs `[nb,[sid],[2]]` (AGM) vs TMC `[%ids%, ctx]` | conflicting |
| `hizoJc` | LoadSource | `[[sid],[2],[2]]` text; `[[sid],[3],[3]]` markdown/HTML | `[0]=[[sid],title,meta]`, text blocks `[3][0]`, HTML `[4][1]` | TL, JB, ROO, ARL, AGM, JUL, ERS | M4Y `[[sid]]`; TMC adds ctx | consensus |
| `b7Wfje` | MutateSource (rename) | `[null,[sid],[[[title]]]]` | `[0][1]` title | TL, JB, ICE, ARL, MOOD | JAC (outlier) | consensus |
| `FLmJqe` | RefreshSource | `[null,[sid],[2]]` | null on success; `[3]` = INVALID_ARGUMENT | TL, JB, ICE, MOOD | AGM `[nb,sid,[2]]`; M4Y `[sid,nb]`; DOK `[sid]` | conflicting |
| `yR9Yof` | CheckSourceFreshness | `[null,[sid],[2]]` | URL `[]` = fresh; Drive `[[null,bool,[id]]]` | TL, JB, AGM, MOOD, ERS | M4Y soft-failed to null and dropped the tool | consensus |
| `dtT1F` | BatchRefreshSources | | | TL (works where `FLmJqe` is rejected) | | single-source |
| `QsNTEd` | AppendSource | `[null,[sid],null,[null,[header,body]]]` | status-tagged null | TL | | single-source |
| `R27wvc` | CopySourcesAsync | `[null,null,[[sid]..],target_nb]` | `[[ [[orig], entry]..]]` | TL | | single-source |
| `Z8UXi` | CopySources (sync twin) | | | TL says dead | | stale |
| `ASU5Oe` | RetrieveRelevantChunks | `[nb, query, null, [1], [[[sid]]..]?]` | ranked chunks with ranges | TL | | single-source |
| `mVtEUb` | ListExpertIntelligenceContent (Play Books) | builder in TL | book rows | TL (TMC name only) | | single-source |
| `tr032e` | GenerateDocumentGuides (source guide) | `[[[[sid]]]]` | summary `[0][0][1][0]`, keywords `[0][0][2][0]` | TL, JB, ICE, ARL, AGM, MOOD | M4Y `[sid,nb,[2]]`; LKA `[nb]`; TMC `[%sources%, ctx]` | conflicting |

### Research / discover

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `Es3dTe` | DiscoverSources (sync) | TL `[[query,1],null,mode,nb]` (mode 1-4, 5 rejected) | `[sources, overview, [job]]` | id: TL, TMC, SAH | TMC `[%project_id%,%query%]` without HAR evidence | conflicting |
| `qXyaNe` | "DiscoverSources" | | | DOK, PHO | TMC: not in any JS bundle or HAR | stale |
| `Ljjv0c` | DiscoverSourcesManifold (fast research) | `[[query,1 web/2 drive],null,1,nb]` | `[task_id]` | TL, JB, TMC (HAR 2026-04-17), ICE, ROO, ARL, AGM, MOOD, M4Y | LKA `[nb,query]` | consensus |
| `QA9ei` | DiscoverSourcesAsync (deep research, web only) | `[null,[1],[query,1],5,nb]` | `[a, b]` | TL, JB, TMC, ICE, ARL, AGM, MOOD, M4Y | meaning of `a,b`: TL session id (unpollable), report id; TMC research id, conversation id; ICE task id, report id; JAC inverse | consensus (request) |
| `e3bVqc` | ListDiscoverSourcesJob (poll) | `[null,null,nb]` | `[[task_id, info, updated, created, account]..]`; info `[null,[query,type],mode,[[sources],summary],status,deep?]` | request: TL, JB, ICE, ROO, ARL, AGM, MOOD, M4Y | status: TL 1 running / 2 done / 3 no results / 4 cancelled vs JB, ARL, MOOD "2 or 6 = done", JBc doc 2 in progress (doc wrong per its own code); TMC: polymorphic id (also DeleteChatHistory / PollDeepResearch by args) | conflicting |
| `LBwxtb` | FinishDiscoverSourcesRun (import) | `[null,[1],task,nb,[entries]]`; web entry `[null,null,[url,title],null*7,2]`; Drive `[[id,mime,1,title],null*9,2]`; deep report `[null,[title,md],null,3,null*6,3]` | `[[ [[sid],title]..]]` | TL, JB, TMC, ICE, ROO, ARL, AGM, MOOD, M4Y | ERS sends empty entries (bug); TMC also binds DeleteDeepResearch | consensus |
| `Zbrupe` | CancelDiscoverSourcesJob | `[null,null,run_id]` | `[]` always | TL, TMC, SAH | | consensus |

### Chat (see section 4 for the streamed endpoint)

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `laWbsf` | JS-bundle id of GenerateFreeFormStreamed | not sent via batchexecute by anyone | stream | TMC (`orchestration.proto:1386,1402`) | see section 4 | single-source |
| `Y0vGub` | "QUERY" constant | declared, unused | | MOOD | | stale |
| `ZAnZ8` | "QUERY" constant | declared, unused | | M4Y | | stale |
| `hPTbtc` | ListChatSessions / last conversation id | `[[],null,nb,limit]` (limit 1 or 20; UI sends OPTS first) | `[[[conv_id]]]`; ARL rows `[conv, created, updated]` | TL, JB, TMC, ICE, ARL, AGM, MOOD | DOK `nil` | consensus |
| `khqZz` | ListChatTurns | `[ctx,null,null,conv,limit(,cursor)]` | turns newest first; `t[2]==1` user text `[3]`, `==2` AI text `[4][0][0]`; `[1]` cursor | TL, JB, TMC, ARL, AGM, MOOD | first element `[]` (TL, MOOD, AGM) vs `HDR` (JB, ARL, TMC); DOK `[conv,20]` | consensus |
| `J7Gthc` | DeleteChatTurns | `[[],conv,null,1]` | `[]` | TL, ICE, MOOD, ARL (HDR) | JB `[nb,conv]`; TMC `[options,conv,null,unknown]` | conflicting |
| `oXwmh` | GetChatSessionStatus | `[null,conv]` | `[null,1]` idle, `[token,2]` generating | TL | | single-source |
| `XgrPMd` | CancelGeneration | `[null,conv]` | `[]` | TL, TMC | | consensus |
| `otmP3b` | GeneratePromptSuggestions | `[OPTS, nb, [[sid]..], mode, null, query]` | `[[ [title,prompt]..]]` | TL, TMC, MOOD | | consensus |
| `OcvKNc` | NextStepSuggestions | `[null,nb(,[[[sid]]..])]` | `[[ [question, 9]..]]` | TL | | single-source |
| `uNyJKe` | SubmitFeedback | | | TMC | | single-source |

### Notes and mind maps

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `CYK0Xb` | CreateNote / save mind map / save chat answer | blank `[nb,"",[1],null,"New Note"]`; mind map `[nb,json,[2,null,null,5,[[sid]..]],null,title]`; chat answer 7 slots (TL) | note row | TL, JB, ICE, ROO, AGM, MOOD, ERS | title ignored on create (follow with `cYAfTb`) | consensus |
| `cFji9` | GetNotes (notes + note-backed mind maps) | `[nb]` | `[[ [id, [id,content,meta,null,title]] ]]`; tombstone `[id,null,2]` | TL, JB, ICE, AGM, MOOD | ERS `[nb,[2]]`; JBc second call `[nb,null,[s,us],[2]]` for mind-map delete | consensus |
| `cYAfTb` | MutateNote | `[nb,note,[[[content,title,[],0]]]]` | row | TL, JB, ICE, AGM, MOOD | | consensus |
| `AH0mwd` | DeleteNotes (soft) | `[nb,null,[ids]]` | `["id",null,2]` | TL, JB, ICE, AGM | MOOD and JBc append `[2]`; mind maps need the `cFji9` timestamp step (JBc) | conflicting |
| `yyryJe` | ActOnSources (mind map JSON; legacy transforms) | `[src3,null*4,["interactive_mindmap",[["[CONTEXT]",instr]],lang],null,[2,null,[1]]]` | `[[json,null,[gen_ids]]]` | TL, JB, TMC, ROO, AGM, MOOD, ERS, PHO | M4Y removed after code 3 (payload was wrong); KAF exposes 14 legacy transform actions (dropped by TMC) | consensus |

### Studio artifacts

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `R7cb6c` | CreateArtifact (all Studio types) | `[FIRST, nb, [null,null,TYPE,src3, ...config at slot]]`; TYPE 1 audio @6, 2 report @7, 3 video @8, 4 quiz/flashcards/interactive mind map @9, 7 infographic @14, 8 slides @16, 9 data table @18, 11 interactive report @34 | `[[id,title,type,null,status,...]]`, status 1 = queued | type codes and slots: TL, JB, ROO, ARL, AGM, KIS (live capture), M4Y | FIRST = `[2]` (JB, ROO, ARL, AGM, MOOD, M4Y, ERS) vs `ARTIFACT_OPTS` = OPTS + `[[1,4,8,2,3,6]]` (TL, MOOD "observed on web UI") vs `[2,null,null,[1..[1]],[[2,1,3]]]` (ICE, MIS). Video style enum, slide enums, infographic 6th slot, quiz/flashcard pair order: see feature-matrix. MOOD puts several configs at slot 6 (wrong per KIS). PHO slides @13, ERS @14 (wrong). LKA flat (pre-2026) | conflicting |
| `gArtLc` | ListArtifacts | `[FIRST, nb, 'NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"']` | row: `[0]` id `[1]` title `[2]` type `[4]` status (3 ready, 4 failed, 5 suggested) `[6][5]` audio media `[url,kind,mime]` `[7]` report `[8]` video `[14]` infographic `[15][0]` created `[16]` slides `[18]` data table `[21]` etag `[24]` xlsx | TL, JB, ROO, ARL, AGM, MOOD, LKA, ICE | video `[8][4]` (TL) vs `[8][3]` (JB, ROO); slides `[16][3]/[4]` PDF/PPTX (TL) vs `[16][0]/[3]` (JB, ROO); report `[7]` vs `[7][1][0]`; MOOD reads `[8]` as a string; status 1/2 naming differs; M4Y omits the filter; timestamps vary by type (KIS) | conflicting |
| `v9rmvd` | GetArtifact (interactive HTML, mind-map tree) | `[artifact]` | HTML `[0][9][0]`; tree `[0][9][3]` | TL, JB, TMC, ARL, ICE, AGM, MOOD | JB "full" form adds OPTS + type list | consensus |
| `V5N4be` | DeleteArtifact | | empty | id: all | `[[2],id]` (ROO, JB, ERS, PHO) vs `[[2],nb,id]` (AGM) vs `[[2],nb,[id]]` (MOOD) vs `[DefaultUserConfig,id]` (ICE); M4Y dropped after code 3 | conflicting |
| `rc3d8d` | UpdateArtifact (rename) | `[[id,title],[["title"]]]` | `[1]` title | TL, JB, ARL | ICE `[id,title]`; AGM `[[2],nb,id,title]` | conflicting |
| `Krh3pd` | ExportToDrive | `[null,artifact,content|null,title,1 docs/2 sheets]` | doc URL | TL, JB, AGM, MOOD | | consensus |
| `RGP97b` | ShareAudio / share artifact | `[[1|0],nb(,artifact)]` | row | TL, AGM, TMC | | consensus |
| `KmcKPe` | DeriveArtifact (revise slide) | `[[2],artifact,[[[idx0,prompt]]]]` | artifact row | TL, JB, AGM, TMC | | consensus |
| `Rytqqe` | GenerateArtifact | `[ARTIFACT_OPTS, artifact]` | row, status 1 | TL, TMC (retry failed) | JB uses it to generate suggested interactive-report elements | conflicting |
| `mKDdke` | CopyArtifactsAsync | `[OPTS,[ids],target_nb]` | | TL | | single-source |
| `zVGIdd` | CopyArtifacts (sync twin) | | | TL says dead | | stale |
| `sqTeoe` | GetArtifactCustomizationChoices | | audio, video, slide, report presets | id: TL, TMC, ICE | `[OPTS]` (TL) vs `[ctx,null,mode]` (TMC) vs `[cfg,nb]` (ICE) | conflicting |
| `ciyUvf` | GenerateReportSuggestions | `[[2],nb(,[[sid]..])]` | `[[ [title,desc,null,null,prompt,audience]..]]` | TL, TMC, ARL, AGM | | consensus |
| `sODAg` | GetArtifactDeferredGenerationQuota | | | TL | | single-source |
| `AHyHrd` | CreateAudioOverview (legacy) | `[nb, audio_type, instructions]` | | TMC (`--direct-rpc` default path), PHO | superseded by `R7cb6c` | stale |
| `VUsiyb` | GetAudioOverview (legacy) | `[nb]` | | TMC, PHO | | stale |
| `sJDbic` | DeleteAudioOverview (legacy) | `[nb]` | | TMC | | stale |
| `xpWGLf` | CreateArtifact (older generic) | `[ctx,nb,artifact]` | | TMC, PHO | | stale |
| `DJezBc` | UpdateArtifact (mask form) | `[artifact, mask]` | | TMC | | single-source |
| `WxBZtb` | delete artifact (PHO non-audio path) | `[artifact]` | | PHO | | stale |
| `BnLyuf` | get artifact (PHO) | | | PHO | | stale |
| `Fxmvse` | play progress / artifact user state | | | ICE, MIS, PHO, TMC | ReportPlayProgress (ICE, MIS) vs UpsertArtifactUserState (TMC); unused everywhere | conflicting |

### Labels and collections

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `I3xc3c` | GetLabels | `[OPTS|[2], nb]`; collections `[OPTS',null,3]` | `[[ [name,[[sid]..],label_id,emoji]..]]` | TL, TMC, ROO, ARL | | consensus |
| `agX4Bc` | CreateLabel / autolabel / collections create | auto `[OPTS,nb,null,null,[]]` (`[0]` unlabeled only); manual `[OPTS,nb,null,null,null,[[name,emoji]]]`; collection `[hdr,null,null,null,null,[[name],null,[nbs]],3]` | `[null,[label..]]` | TL, JB, TMC, ROO, ARL | JB also uses it to list (and lists collections by create+delete temp); ROO 3.1.2 fixed the same mistake | conflicting |
| `le8sX` | MutateLabel | `[OPTS,nb,label,[[name_emoji, add, remove]]]` | `[]` | TL, JB, TMC, ROO, ARL | only first id honored per call (TL) | consensus |
| `GyzE7e` | DeleteLabels | `[OPTS,nb,[ids]]`; collections `[hdr,null,[cid],3]` | `[]` | TL, JB, TMC, ROO, ARL | | consensus |

### Sharing

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `QDyure` | ShareProject (LabsTailwindSharingService) | public `[[[nb,null,[v],[v,""]]],1,null,[2]]` (0 restricted, 1 link); users `[[[nb,[[email,null,perm]..],null,[msg_flag,msg]]],notify,null,[2]]` perm 2 editor, 3 viewer, 4 remove | `[]` | TL, JB, ROO, ARL, AGM, MOOD, ICE | PHO `[nb,[[email,null,role]..]]` and link `[nb,null,[1]]` (outlier) | consensus |
| `JFMDGd` | GetProjectDetails (share status) | `[nb,[2]]` | `[[ [email,perm,[],[name,avatar]]..], [isPublic], maxShare, allowed, ...]` | TL, JB, ROO, ARL, AGM, MOOD, ICE | | consensus |
| `ARGkVc`, `EYqtU`, `YJBpHc`, `R6smae`, `LJyzeb`, `OTl0K`, `itA0pc` | Guidebooks service (delete, get, list, publish, details, share, answer) | | | TMC (`sharing.proto:306-330`), PHO (declared) | never exercised | single-source (7 ids) |

### Account, quota, settings

| Id | Purpose | Request | Response | Sources agreeing | Conflicts | Status |
|---|---|---|---|---|---|---|
| `ZwVcOc` | GetOrCreateAccount | `[OPTS]`, `source-path=/` | limits `[0][1]` = `[?,500,300,500000,tier]`, language `[0][2][4][0]` | id: TL, TMC, ICE, ARL, AGM, M4Y | payloads `[OPTS]` (TL), `[null,[1,null*9,[1]]]` (ICE, AGM), `[DefaultUserConfig]` (ICE quota), `[HDR]` (ARL), `[null,1]` (M4Y warm-up), `[[ctx]]` (TMC) | conflicting |
| `hT54vc` | MutateAccount (output language) | TL `[[null,[[null,null,null,null,[lang]]]]]`, `source-path=/` | settings | id: TL, ICE, AGM, ARL | ICE/AGM one more nesting level; ARL appends `HDR`; M4Y/JAC call it PREFERENCES and M4Y's chat-config guess failed | conflicting |
| `SatQRc` | GetAccount (quota meter) | `[]` | | TL | | single-source |
| `EylDcb` | ListQuotaSummary (5-hour and weekly windows, per-action cost) | TL `[null]` | `[1]` windows `[..,[4] type,[5] reset,[6] pct used,[7] pct left]` unordered | id: TL, JB, ARL | `[[2,null,[1],[1,null*9,[1,3]]]]` (JB) vs `[HDR]` (ARL) | conflicting |
| `gzAChb` | GetQuota | | | TL (not modelled) | ICE says quota parser used the wrong RPC; GetOrCreateAccount is the right one | single-source |

### Telemetry and low-value ids (do not implement)

Single-source, named by TMC's proto/bundle mapping (PHO declares the four "outline/section/draft" ids too): `EnujNd` ListModelOptions, `ulBSjf` GetArtifactUserState, `preRPe` GenerateAccessToken, `rtY7md` GetMagicView, `OmVMXc` ReportContent, `HpN0Ub` LogInteractionEvent, `DemIHe` UpdateFeaturedNotebookStatus, `n3dkHd` CreateAccessRequest, `uK8f7c` GenerateMagicView, `lCjAd` GenerateOutline, `BeTrYd` GenerateSection, `exXvGf` StartDraft, `pGC7gf` StartSection, `Of0kDd` FetchInteractivityToken (live voice), `eyWvXc` SDPExchange (live voice). PHO-only declared: `GHsKob`, `rJKx8e` (meaning unclear). Sources: TMC note RPC table rows 100-102; PHO note "declared, unused".

## 4. Chat: `laWbsf` vs GenerateFreeFormStreamed

**What each source says.**
- TMC: chat goes to `POST https://{host}/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed`; the proto comment says "gRPC-Web stream - NOT batchexecute", and the JS bundle binds RPC id `laWbsf` to that method (`client_chat.go:107`, `orchestration.proto:1386-1402`).
- TL, JB, ROO, ARL, ICE, AGM, M4Y, MOOD, SAH, DOK, LKA, MFL, JUL, ERS, PHO, KIS: all POST to the same `.../GenerateFreeFormStreamed` URL with body `f.req=<urlencode([null, "<params JSON>"])>&at=<csrf>&`. None of them sends `rpcids=laWbsf` to batchexecute.
- Declared but unused "QUERY" ids: MOOD `Y0vGub`, M4Y `ZAnZ8`. Neither is called; meaning unknown.
- Enterprise web variant: `.../CloudNotebookLmUi/data/google.cloud.notebooklm.v1main.NotebookService/GenerateFreeFormStreamed` (JB, not live-validated). Enterprise REST has no chat at all (ROB, KD).

**Reconciliation.** These are not two competing transports. `laWbsf` is the name the web bundle uses for the streamed method; the wire call is the dedicated GenerateFreeFormStreamed path with the `[null, "<params>"]` envelope, `rt=c`, plus `bl`, `hl`, `_reqid`, `f.sid` (TL, TMC, JB all send `bl` and `_reqid` here even when they skip them on batchexecute). Evidence that the path form is current: 16 independent-or-derived clients across 5 families use it, including TMC and KIS after the 2026 host move. **Likely current: the GenerateFreeFormStreamed path. Do not implement chat via batchexecute `laWbsf`.** Treat `Y0vGub`/`ZAnZ8` as stale.

**Params: two shapes in the wild.**

| Shape | Layout | Who |
|---|---|---|
| 9-element (newer) | `[sources, question, history|null, [2,null,[1],[1]], conversation_id|null, null, null, notebook_id, seq]` | TL (`chat_stream.py:47-68`), TMC (options `[2,null,[1],[1,null*9,[1,3]]]`), ICE (`seq` = turn counter + 1), AGM, M4Y (reached independently), DOK, SAH, LKA |
| 5-element (older) | `[sources, question, history|null, [2,null,[1]], conversation_id]` | JB, ROO, ARL, MOOD, JUL, ERS (conv id null); PHO puts notebook id in slot 4 |

AGM calls the 9-element form "the newest shape (March 2026)"; TL and TMC (both maintained to Oct 2026) use it. **Prefer 9-element**; keep the 5-element form as a capture-verified fallback only if live testing shows the 9-element form rejected.

Other chat conflicts:
- Source nesting: `[[id]]` per source (ROO, ARL, AGM, ICE) vs `[[[id]]]` (TL depth 2 list of `[[sid]]`, TMC, M4Y, DOK, SAH). Empty list means "all" (TL) but the server then picks a subset (TMC README); LKA sends `nil`, JAC `[]`. Always send explicit ids.
- History: oldest-first `[[answer,null,2],[query,null,1],...]` (JB `conversation.py:71-105`, AGM pairs), chronological `[query,null,1],[answer,null,2]` (M4Y, last 10), newest-first (ICE `history.unshift`). Server-side history via `hPTbtc`/`khqZz` makes local history optional (ARL sends the last 20 turns from `khqZz`; SAH sends null).
- Conversation id: TL and SAH: the id in the stream is a per-stream session id, replaying it creates a ghost turn (TL #659); read the real id with `hPTbtc` after the first ask. JB, AGM, ROO read `first[2][0]` and migrate; M4Y reads `inner[10]`. ROO RPC path sends a fresh UUID each call (single-turn only).
- Header `x-goog-ext-353267353-jspb: [null,null,null,282611]`: sent by TMC ("observed in HAR") and PHO; omitted by everyone else including TL. Unknown if required.
- Response: chunks are cumulative (TL, TMC, ARL); answer vs thinking flag at the type block last element (1 answer, 2 thinking: JB, ROO, ARL, AGM, DOK, SAH, ERS); TL uses `isFinalResponse` at response `[4]`; heartbeats decode to `[]`. TMC diffs snapshots by longest common prefix and keeps `Full` because citations can be rewritten (`[2, 3]` -> `[2-5]`). "Longest text wins" without the type flag (LKA, M4Y, ICE) can return thinking text.
- Citations: type block `[3]` passages; citation N = passage N-1; source UUID at `passage[1][5][0][0][0]`; cited text from `passage[1][4]`; offsets in UTF-16 code units (JB, ROO, ARL, SAH, ICE char offsets); TMC `SourceID` is a chunk handle, resolve titles via `ParentSourceID`. JBc's TECHNICAL_DEEP_DIVE describes a different nesting (`parsed[1]` list): older view.
- Errors: auth failure arrives as HTTP 200 with an error frame (TMC scans ints 16/277566/277567); rejection = null answer frame with `google.rpc.Status` and `UserDisplayableError` detail = rate limit/quota (SAH, TL); oversized query = `[3]` (TL).
- Timeouts: TL 180 s, JB 120 s, TMC first chunk 5 min / idle 5 min / total 15 min with progress measured on parsed chunks, 1 MiB answer cap and repeat guard.

## 5. File upload (`o4cbdc` + `/upload/_/`)

Agreed by TL, JB, TMC, ICE, AGM, DOK (WAN behavior):
1. `o4cbdc` with `[[[filename]], nb, tail]` and `source-path=/notebook/{nb}` returns SOURCE_ID (first string in the result). Never retried (TL). Must precede the upload (TMC `client_upload.go:495-501`).
2. `POST https://{host}/upload/_/?authuser=N` with `x-goog-upload-command: start`, `x-goog-upload-protocol: resumable`, `x-goog-upload-header-content-length: <bytes>`, optional `x-goog-upload-header-content-type`, `x-goog-authuser`, `Origin`, `Referer`, cookies. Body `{"PROJECT_ID":nb,"SOURCE_NAME":file,"SOURCE_ID":sid}` in exactly that key order (TMC: Go map order breaks it). TL's note describes a form-encoded body with the same three keys: **conflict, capture**. Response header `x-goog-upload-url`.
3. `POST <x-goog-upload-url>` with `x-goog-upload-command: upload, finalize`, `x-goog-upload-offset: 0`, raw bytes. JB streams 64 KiB chunks. A 5xx with `X-Goog-Upload-Status: final` means finalized (TMC `:518-555`).
4. Poll `rLM1Ne` until the source status is ready.

Pitfalls: TL's Windows selector-loop bug dropped chunks of large uploads (stream the body, never buffer partial writes); ICE, AGM, ROB read whole files into memory; PHO never sends bytes; MOOD text-extracts PDFs instead. Enterprise REST upload is different: `POST /upload/v1alpha/{parent}/notebooks/{id}/sources:uploadFile?uploadType=media` with `X-Goog-Upload-Protocol: raw`, `X-Goog-Upload-File-Name`, `Content-Type` (KD, ROB).

## 6. Cookie rotation and session keepalive

- **Required cookies**: TL `SID` + `__Secure-1PSIDTS`, plus `OSID` or `APISID+SAPISID+LSID`; JB, M4Y, JUL `SID, HSID, SSID, APISID, SAPISID`; PAV only `SID` (too weak). Domains: `.google.com`, the NotebookLM host, `accounts.google.com` (for rotation), `.googleusercontent.com` (media). Filter by host and dedupe by name or Google answers CookieMismatch (MOOD `e4a32c6`); `.google.com` wins on duplicates (AGM, SAH).
- **RotateCookies**: `POST https://accounts.google.com/RotateCookies`, `Content-Type: application/json`, `Origin: https://accounts.google.com`, body `[000,"-0000000000000000000"]` (TL, JB) or `[0,"-0000000000000000000"]` with the other as fallback (MOOD), 15 s timeout, jar minus `OSID`/`__Secure-OSID`, no redirects (MOOD) or follow (TL). Declares a 600 s next rotation; rate-limit to once per 60 s per profile (TL `keepalive.py:62-73`). Precondition SID + (OSID or APISID+SAPISID) (MOOD). 401/403 = session dead.
- **Conflict**: TL says it returns fresh `__Secure-1PSIDTS/3PSIDTS` and worked for DBSC-bound and unbound profiles; JB issue #316 says from plain httpx it refreshes only `*SIDCC`, not `*PSIDTS`, and aged-out sessions need a real browser (JB `cookie_rotation.py:1-11`, fix for "8-hour session death"). Live test needed.
- **Signaler**: `POST https://signaler-pa.clients6.google.com/punctual/v1/refreshCreds?key=<public web key>[&gsessionid=]`, `Authorization: SAPISIDHASH`, `Content-Type: application/json+protobuf`, body `["tZf5V3ry"]` (TMC) or `["8hcNI0LV"]` (PHO). Does not mint cookies; may keep the server session warm [unverified].
- **Passive rotation**: merge `Set-Cookie` from every response and persist atomically (M4Y, DOK, ICE, MIS); learn `f.sid` from `af.httprm` (M4Y).
- **DBSC**: MOOD documents Device Bound Session Credentials cookies that may not replay outside the issuing browser and detects "rejected (DBSC)" by a real `wXbhsf` after login (`auth.ts:179-220`); PAO measured that injected cookies cannot sign in a fresh profile. performance.md s7: if Google enforces DBSC, cookie replay dies and the browser (in-page fetch) or Enterprise path becomes the only one.

## 7. Host migration

- New host `notebook.google.com` serves the same `/_/LabsTailwindUi/...` paths and ids (KIS `bc32922`, TMC `1ef43ba` 2026-07-27, KAF `8e936ac`, ROO 3.0.4, PAO, ARL, TL default).
- TMC: the legacy origin rejects freshly harvested credentials with 401. SAH WIP: the old host "is still served". JB v0.15.4 still defaults to `notebooklm.google.com` but persists a per-profile `base_host` learned at login (rollout is per account, JB commit `5c772d1`). TL serves both via `NOTEBOOKLM_BASE_URL` with an allow-list.
- Enterprise/Workspace hosts: `notebook.cloud.google.com`, `notebooklm.cloud.google.com`, `vertexaisearch.cloud.google.com` (JB `config.py:68-74`, TL `_env.py:23-25`).
- Rebrand date: MOOD README says 2026-07-16; first code evidence 2026-07-25 (JB). ROO's fallback hid a dead RPC path for 3 releases because the cookie jar was collected for the wrong host (ROO CHANGELOG 3.0.4).
- Rule for us: default `notebook.google.com`; at login record the host the account lands on; on bootstrap failure probe the other host with the cookie jar for that same host; derive `Origin`, `Referer`, SAPISIDHASH origin and cookie URL from the chosen host; one `IsNotebookHost(u)` function (PAN FX-026).

## 8. Enterprise REST (official, Pre-GA)

Base `https://{us|eu|global}-discoveryengine.googleapis.com/v1alpha/projects/{PROJECT_NUMBER}/locations/{location}`; endpoint location may differ from resource location (ROB, KD). Auth `Authorization: Bearer` (gcloud/ADC/env token); optional `x-goog-user-project` (KD). Google page cited by ROB: https://docs.cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/api-notebooks.

| Operation | HTTP | Path | Body / notes | Sources |
|---|---|---|---|---|
| list | GET | `notebooks:listRecentlyViewed?pageSize<=500` | no `nextPageToken` ever (KD) | ROB, KD |
| get | GET | `notebooks/{id}` | includes `sources[]` | ROB (KD: not implemented) |
| create | POST | `notebooks` | `{"title"}` | ROB, KD |
| delete | POST | `notebooks:batchDelete` | `{"names":[...]}`; 2+ names -> 400 (KD 2025-10-31); `{}` even for missing names (ROB) | ROB, KD |
| share | POST | `notebooks/{id}:share` | `{"accountAndRoles":[{"email","role":"PROJECT_ROLE_READER"}]}`; KD: removed from docs 2025-11-07 | ROB |
| add sources | POST | `notebooks/{id}/sources:batchCreate` | `userContents[]` of `webContent{url,sourceName}`, `textContent`, `videoContent{youtubeUrl}`, `googleDriveContent{documentId,mimeType,sourceName}` | ROB, KD |
| upload | POST | `/upload/v1alpha/.../notebooks/{id}/sources:uploadFile` | raw | ROB, KD |
| get source | GET | `notebooks/{id}/sources/{sid}` | ready = `SOURCE_STATUS_COMPLETE` | ROB, KD |
| delete sources | POST | `notebooks/{id}/sources:batchDelete` | names use singular `/source/` | ROB, KD |
| audio create | POST | `notebooks/{id}/audioOverviews` | only `{}` accepted; `sourceIds`, `episodeFocus`, `languageCode` rejected "Unknown name" (KD, ROB March 2026; ROB spike re-test outcome unknown) | ROB, KD |
| audio delete | DELETE | `notebooks/{id}/audioOverviews/default` | | ROB, KD |
| audio status | GET | `audioOverviews/default` | ROB docs: polling "does NOT work"; unresolved | ROB |
| podcast (license-free) | POST | `https://discoveryengine.googleapis.com/v1/projects/{p}/locations/global/podcasts` | stable v1, LRO, `roles/discoveryengine.podcastApiUser`; download `{op}:download?alt=media` | ROB |

Not available in REST: chat, all Studio types except audio, notes, research, labels, rename, summary (ROB `server.py:339-350`, KD table).

## 9. Android gRPC backend (owner option only)

TL v0.8.x ships an alternative transport: HTTP/2 gRPC to `notebooklm-pa.googleapis.com:443`, bearer minted from a Google master token via gpsoauth with the Android app identity, and a full protobuf schema recovered from the Android app (`docs/android/schema.proto`, 326 messages). Web JSON index `i` equals proto tag `i+1`. It is a decompilation of Google's app: legal gray area. We do not plan to vendor or port it; see decisions-needed.md D1. Facts here are from the TL note only.

## 10. Live-capture checklist

Capture each item against a real signed-in session on `notebook.google.com` (and once on `notebooklm.google.com`) before pinning the Go encoder. Record request, response, date, host and account cohort as a sanitized golden fixture.

Transport and framing
1. Minimum header set for batchexecute: with and without `X-Same-Domain`, `Origin`/`Referer`, `X-Goog-Csrf-Token`, SAPISIDHASH.
2. Whether `bl`, `_reqid`, `source-path` are needed on batchexecute; whether a stale `bl` breaks mutations (JBc) or not (TL).
3. Declared chunk length unit (UTF-16 vs bytes) on a response with non-ASCII titles and on a rejection body.
4. Whether plain Go TLS gets `CookieMismatch` on `GET /` (MIS claim).
5. Legacy host behavior with freshly harvested cookies (TMC 401 vs SAH "still served").

Request-options cohort
6. Flat `[2],[1,null*9,[1]]` vs nested `OPTS` vs `HDR` on `CCqFvf`, `rLM1Ne`, `izAoDd` (URL, text, YouTube, Drive), `o4cbdc`, `gArtLc`, `R7cb6c`; record which return code 3/5/9.
7. `rLM1Ne` last arg `0` vs `1`.

Sources
8. `tGMBJ` delete: `[[[sid]],[2]]` vs `[[[sid]]]`.
9. `FLmJqe` refresh shape; when `dtT1F` is needed.
10. `tr032e` guide shape (`[[[[sid]]]]` vs `[sid,nb,[2]]`).
11. `ozz5Z`: what the web UI sends it for (add URL v2, entitlement, UI config, log event).
12. Upload start body: JSON with key order vs form-encoded; `o4cbdc` tail.
13. Batch `izAoDd` with mixed valid/invalid URLs: which items are omitted.

Research
14. `Es3dTe` request shape.
15. `QA9ei` response: meaning of both elements; which one `e3bVqc`, `LBwxtb` and `Zbrupe` take.
16. `e3bVqc` status values for running / done / no results / cancelled (2 vs 6).

Chat
17. 9-element vs 5-element params; options block `[2,null,[1],[1]]` vs TMC's long form.
18. Source nesting `[[id]]` vs `[[[id]]]`; behavior with empty list.
19. History order and whether history is needed when `conversation_id` comes from `hPTbtc`.
20. Where the real conversation id appears (`first[2][0]`, `inner[10]`, only via `hPTbtc`).
21. Need for `x-goog-ext-353267353-jspb`.
22. Citation positions and UTF-16 ranges in the current answer row.
23. `J7Gthc` delete-history shape (`[[],conv,null,1]` vs `[nb,conv]`).

Studio
24. `R7cb6c` first element (`[2]` vs `ARTIFACT_OPTS` vs ICE config).
25. Video format and style enum values (TL vs ICE vs KIS).
26. Slide deck format/length enum values (TL vs KIS); confirm slot 16.
27. Infographic 6th slot (style vs constant 1).
28. Quiz and flashcards quantity/difficulty order.
29. `gArtLc` row positions: video media `[8][3]` vs `[8][4]`, slides PDF/PPTX, report markdown, data table, timestamps per type.
30. `V5N4be` delete shape; `rc3d8d` rename shape; `Rytqqe` semantics.
31. Language: `hl` vs payload language for generation (ROO issue #34).
32. `sqTeoe` payload.

Notes, labels, account
33. `AH0mwd` with and without trailing `[2]`; mind-map ghost cleanup via `cFji9` timestamp.
34. `agX4Bc` vs `I3xc3c` for listing labels and collections.
35. `ZwVcOc`, `hT54vc`, `EylDcb` payloads.

Auth
36. RotateCookies: which cookies come back from a plain Go client (PSIDTS or only SIDCC), body `[000,...]` vs `[0,...]`.
37. DBSC: whether exported cookies replay from Go at all for a fresh login.
38. Signaler `refreshCreds` effect and body token.

Enterprise (needs a licensed project)
39. `audioOverviews` documented fields and status polling.
40. `notebooks:batchDelete` with 2+ names; `listRecentlyViewed` pagination.
