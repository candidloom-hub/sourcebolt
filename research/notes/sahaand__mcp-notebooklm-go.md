# sahaand/mcp-notebooklm-go

**LICENSE: none found** (`ls LICENSE*` -> no file; GitHub API reports none). All rights reserved by default. **BEHAVIOR ONLY: no code was copied or may be copied.** This note describes observable protocol behavior and facts, in prose.

- URL: https://github.com/sahaand/mcp-notebooklm-go
- Language / runtime: Go 1.22 (`go.mod:3`); dependency only `spf13/cobra` (`go.mod:5`). **Go module path is `github.com/saeedata/notebooklm-go`** (a different owner name than the repo, so this is a re-published copy of someone else's project).
- Stars / last commit: 1 star (per upstreams.md); HEAD `e5ee305` 2026-10-11 (merge of PR #2 `fix/rpc-wire-shapes`); 13 commits since 2026-03-27.
- Versions reviewed: no tags. Commits of interest: `0fad659` 2026-03-30 "full feature parity with notebooklm-py reference"; `30b2dea`/`2ef535c` 2026-05-14; **`2d91c5b`, `17b86c5`, `bfe53af`, `c0bcf91`, `6981425` on 2026-10-11** (migration to "current wire shapes").
- Size: 9275 lines total incl. tests; `sources.go` 808, `artifacts.go` 627, `chat.go` 593, `rpc/decoder.go` 398, `rpc/types.go` 341.

## Contradicts research/upstreams.md

upstreams.md lists this as "MCP server (Go)" with "RPC + Playwright [unverified]". Reading the code: **it is not an MCP server**. It is a library plus a cobra CLI (`cmd/notebooklm/*_cmd.go`). The only MCP content is a README section that sketches how one could expose five tool names (`notebooklm_ask, notebooklm_create_notebook, notebooklm_add_source, notebooklm_list_notebooks, notebooklm_generate_audio`) using "your preferred MCP server library" (`README.md:478-495`); no MCP code or dependency exists (`grep -rn -i mcp --include=*.go` found nothing). Playwright is only used by a helper script outside the Go code (`scripts/auth/*.js`, a CDP-websocket cookie exporter, `export_cookies.js`, plus `login.sh`).

Why it still matters: its October 2026 commits are the most recent evidence of what NotebookLM's backend accepts. Its `WIP.md` says: "In October 2026 every RPC was re-audited against notebooklm-py at commit `2aff0fc` (2026-10-09). Google's 'Gemini-3.5' backend rollout had changed most request shapes ... None of the migrated calls have been run against a live account yet." (`WIP.md:6-12,18-34`). So the shapes are *offline-audited against teng-lin/notebooklm-py*, **not live-verified**. Treat as a pointer to the teng-lin diff, not as ground truth.

## Access method

Internal batchexecute RPC (+ streaming chat endpoint + Scotty upload). Host `notebooklm.google.com` (old); WIP #6: "The reference now defaults to `https://notebook.google.com`. The `notebooklm.google.com` host this client uses is still served, but may need to become configurable." (`WIP.md` item 6). **Contradicts tmc/nlm**, which says the legacy origin rejects freshly harvested credentials with 401 (`git show 1ef43ba`). Possibly both true: old origin keeps working for sessions created against it, fails for new ones. [unverified]

## RPC catalog

IDs (`rpc/types.go:5-80`), all consistent with the other repos: ListNotebooks wXbhsf, CreateNotebook CCqFvf, GetNotebook rLM1Ne, RenameNotebook s0tc2d, DeleteNotebook WWINqb, RemoveRecentlyViewed fejl7e, AddSource izAoDd, AddSourceFile o4cbdc, DeleteSource tGMBJ, GetSource hizoJc, RefreshSource FLmJqe, CheckSourceFreshness yR9Yof, UpdateSource b7Wfje, **DiscoverSources Es3dTe** (agrees with tmc; Dokkabei's `qXyaNe` is wrong), Summarize VfAZjd, GetSourceGuide tr032e, GetSuggestedReports ciyUvf, CreateArtifact R7cb6c, ListArtifacts gArtLc, DeleteArtifact V5N4be, RenameArtifact rc3d8d, ExportArtifact Krh3pd, ShareArtifact RGP97b, GetInteractiveHTML v9rmvd, ReviseSlide KmcKPe, StartFastResearch Ljjv0c, StartDeepResearch QA9ei, PollResearch e3bVqc, ImportResearch LBwxtb, **CancelResearch Zbrupe** (tmc: CancelDiscoverSourcesJob), GenerateMindMap yyryJe, CreateNote CYK0Xb, GetNotesAndMindMaps cFji9, UpdateNote cYAfTb, DeleteNote AH0mwd, GetLastConversationID hPTbtc, GetConversationTurns khqZz, ShareNotebook QDyure, GetShareStatus JFMDGd, GetUserSettings ZwVcOc, SetUserSettings hT54vc.

Default `bl` build label: `boq_labs-tailwind-frontend_20260802.02_p0` (`rpc/types.go:11`), with the comment that the chat endpoint "does not currently validate it". Useful data point: a stale `bl` is tolerated by chat as of Oct 2026, but we should still send the live one.

Request facts documented in prose (from `rpc/encoder.go`, `sources.go:354-400`, tests `rpc/encoder_test.go`, `sources_test.go`):

- Envelope identical to tmc: `f.req` = JSON of `[[[id, "<params json string>", null, "generic"]]]`, form body `f.req=...&at=...&` with trailing ampersand (`rpc/encoder.go:1-45`). URL params: `rpcids`, `source-path` (defaults to `/`), `hl=en`, `rt=c`, optional `f.sid` (`:48-62`). No `bl` on batchexecute, no `_reqid`.
- **"Template block"** (`rpc/encoder.go:50-60`): a request-context fragment — the integer 2, two nulls, then an 11-slot array starting with 1, nine nulls, then `[1]` — that "create-notebook, every add-source variant and get-notebook now require. Older payloads sent a flat `[2]` and are rejected with INVALID_ARGUMENT." So after the Gemini-3.5 rollout the flat `[2]` tail is reported as rejected. This differs from tmc's tail (third slot `[1]` and last element `[1,3]`), so at least one of the two context blocks differs by RPC family or by date. We need a live capture to settle which tail each RPC wants.
- Source spec: an 11-slot array with the value placed at one slot and the integer 1 in the last slot. Slot 2 = `[url]` (web), slot 7 = `[url]` (YouTube), slot 1 = `[title, content]` with slot 3 = 2 (text), slot 0 = `[fileId, mime, 1, title]` (Drive; Drive still uses the old flat tail, "TODO #1546" in the reference) (`sources.go:359-391`). GetNotebook params: `[id, null, <template block>, null, 0]` (`:354-356`).
- Source ID lists nest by depth: depth 1 `[[id]]`, depth 2 `[[[id]]]` helper (`rpc/encoder.go:70-84`).

## Streaming/chat

- URL: chat endpoint with `bl`, `hl`, `_reqid` (**starts at 100000, +100000 per call, mutex-guarded**, `client.go:67-75`), `rt=c`, `f.sid`. Body `f.req=[null,"<params>"]&at=<csrf>&` (`chat.go:137-176`).
- Params: `[ nestedSourceIds(depth 2), question, history|null, [2,null,[1],[1]], conversationId|null, null, null, notebookId, 1 ]`. Empty source list -> all notebook sources via a list call. History is sent as null (server keeps it); WIP item 4 notes the reference sends earlier turns from a local cache as `[[answer,null,2],[question,null,1],...]` when context is lost (`WIP.md` item 4).
- Headers: UA, `X-Same-Domain: 1`, Cookie. No `x-goog-ext-353267353-jspb` (tmc says required).
- Retry: on 401/403 it calls `RefreshAuth` (re-fetch of the home page for `SNlM0e/FdrFJe`) and retries **once**; 429 -> "chat rate limit exceeded"; whole stream read with a 3-minute timeout (`chat.go:22,81-135`).
- Parse (`chat.go:321-485`): reads chunk lines; the answer frame carries the text, a **type block at answer[4]** whose last element marks answer vs thinking, and **citations at type_block[3]**. Each citation: first element `[chunk_id,...]`, second element `detail` where `detail[5]` holds the notebook source UUID (found by a depth-limited UUID search) and `detail[4][0]` holds the cited-fragment paragraph elements `[start, end, [[ [start,end,[text,style]], ... ]]]`. Citation number = 1-based position, matching `[N]` markers. **Offsets are UTF-16 code units** (stated in the source comment at `chat.go:523-526`; they re-slice cited text by UTF-16 to trim overlaps). This agrees with tmc's UTF-16 frame-length logic: Google uses JS string indexing everywhere.
- Rejections: a null answer frame with a google.rpc.Status payload `[code, message, details[]]`; if a details entry's type string contains `UserDisplayableError` the client reports "rate limited or rejected by the API; wait a few seconds" plus the server message (`chat.go:461-482`). That is the nearest thing to a rate-limit signal in any Go repo: chat throttling is signaled **in-band** with HTTP 200, not 429.
- Conversation id: after a first ask it re-reads the notebook's last conversation id with `hPTbtc` instead of trusting the id in the stream (`chat.go:60-74`).

## Auth lifecycle

- Source: Playwright `storage_state.json` at `$NOTEBOOKLM_HOME`/`~/.notebooklm/storage_state.json`, or inline JSON `NOTEBOOKLM_AUTH_JSON`, or `--storage` path (`auth.go:37-90`, README "Authentication"). Export via `npx playwright codegen --save-storage ...` or the bundled CDP script `scripts/auth/export_cookies.js` that connects to a Chrome with remote debugging.
- Cookie filter: `.google.com`, `notebooklm.google.com`, `.googleusercontent.com`, and any `.google.<ccTLD>`; when the same name appears twice the `.google.com` one wins (`auth.go:96-125`). (Dokkabei deliberately excludes regional ccTLDs to avoid cross-account mixing; here they are included.)
- Tokens: `GET https://notebooklm.google.com/` with Chrome-131 UA; regexes for `SNlM0e`, `FdrFJe` (`auth.go:129-170`).
- Refresh on 401/403 only inside chat; RPC path has its own policy via `rpcCallPolicy`. Multi-account (`authuser`) is **not** implemented; NOT_FOUND / PERMISSION_DENIED errors append a hint that several signed-in accounts cause account-routing mismatch because requests default to account index 0 (`rpc/decoder.go:104`, `WIP.md` item 5). Good UX line to copy as a concept.

## Reliability

- **Null-result policy** (`rpc/decoder.go:186-250`): three policies per RPC: `NullIsError` (default), `NullAllowed` (e.g. remove-from-recent, which returns a non-OK status on success), `NullAllowedUnlessRejected` (rename/refresh: null is success unless a non-OK status is attached). A response with no RPC frames at all is reported as "possibly a login or anti-bot page"; a response containing other RPC ids but not the requested one is reported as "the method ID may have changed". These two diagnostics are very useful.
- **Multiple frames for one id**: in `rt=c` mode the server may send a null placeholder frame and later the real payload; the **last non-null frame wins**; `er` frames and `UserDisplayableError` markers are terminal (`decoder.go:146-185`).
- Chunk parsing is line-based and ignores declared lengths, but counts malformed lines and fails if more than 10 percent are malformed (`decoder.go:22-72`) — a cheap integrity check.
- gRPC status labels table `grpcStatusLabels` and a 300-character cap on server messages (`decoder.go:74-108`).
- No retry/backoff for RPCs beyond the single auth refresh. 30-second default HTTP timeout (`client.go:18`).

## Performance notes

`sync.Mutex` around auth tokens and request id. Otherwise nothing measured.

## MCP tools

None implemented (see top).

## Tests and quality

`rpc/decoder_test.go` (175), `encoder_test.go` (86), `artifacts_test.go` (196), `chat_test.go` (209), `notes_test.go` (94), `research_test.go` (61), `sharing_test.go` (78), `sources_test.go` (247): unit tests pin each migrated payload to the reference shape (`WIP.md` intro). No live tests, no CI file in the tree, `.idea/` committed.

## Versions

No tags. Timeline in the header.

## Adopt (behavior to reimplement)

1. Per-RPC **null policy** plus status-aware decoding; distinct errors for "login/anti-bot page", "method id changed", "server rejected (label)", "empty result (parameter mismatch)" (`decoder.go:208-250`).
2. **Last non-null frame wins** for repeated ids; stop on `er`/UserDisplayableError (`decoder.go:146-185`).
3. Chat rate-limit detection through the `UserDisplayableError` entry inside the Status details (`chat.go:461-482`).
4. Citations at answer-type-block index 3 with chunk id, source UUID at detail[5], fragment paragraphs at detail[4][0]; offsets in UTF-16 (`chat.go:485-575`).
5. Account-mismatch hint on NOT_FOUND / PERMISSION_DENIED.
6. Malformed-chunk ratio guard (>10 percent) as a cheap parse sanity check.
7. A WIP/audit file that records "audited offline against reference commit X, not live-verified" — same discipline we should apply to our own RPC table (store `verified_at` and `source_commit` per RPC).

## Avoid

1. Relying on this repo as proof the shapes work live; its own WIP says unverified.
2. Name confusion (repo `sahaand/mcp-notebooklm-go`, module `saeedata/notebooklm-go`, not MCP).
3. Hard-coded `bl` and per-call request-id counters starting at 100000 that reset per process.
4. Cookie filter that admits all `.google.<ccTLD>` domains.
5. No retry for transient 5xx.

## Evidence

Read: `WIP.md`, `go.mod`, `README.md` (head, auth, MCP section), `rpc/{encoder,decoder,types}.go` (most), `sources.go:354-400`, `chat.go:1-200,321-575`, `auth.go:1-170`, `client.go:1-80`, `scripts/auth/export_cookies.js` (head), `git log`. Nothing executed. No code copied.
