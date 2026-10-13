# tmc/nlm

- URL: https://github.com/tmc/nlm
- Language / runtime: Go 1.25 (`go.mod:3`), single static binary, CLI + library + stdio MCP server
- License: MIT, Copyright (c) 2024 Travis Cline (`LICENSE:1-3`). Short snippets (<15 lines) allowed; credit in README.
- Stars / last commit: 391 stars (per research/upstreams.md); HEAD `7a173b4` 2026-09-22 (674 commits; first commit 2024-11-17 `98e3132`)
- Versions reviewed (tags/commits): tags v0.1.0 (2026-07-31), v0.1.1 (2026-08-04); HEAD `7a173b4`. README mentions a "v0.2.0 candidate" (`docs/releases/v0.2.0.md`), not tagged.
- Clone: `research/upstreams/tmc__nlm/`. Read only; nothing built or run.

This is the most mature Go prior art. Almost everything below is reusable as design, and the batchexecute package is MIT so a port with attribution is allowed.

## Access method

Internal RPC (batchexecute) + one gRPC-Web-style streaming endpoint for chat. No official API. Browser (chromedp) is used **only for auth harvesting**, never for data calls.

- batchexecute endpoint: `POST https://notebook.google.com/_/LabsTailwindUi/data/batchexecute` (`internal/batchexecute/batchexecute.go:159`, host default `internal/notebooklm/rpc/rpc.go:284`). The host was changed from `notebooklm.google.com` to **`notebook.google.com`** on 2026-07-27 (`git show 1ef43ba`: "the legacy origin rejects freshly harvested credentials with 401"). Generated service clients still carry the old host (`gen/service/*_client.go:38`) and `NewWithConfig` rewrites it (`internal/notebooklm/rpc/rpc.go:301-303`).
- Chat: separate endpoint `POST https://<host>/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed` (`notebooklm/client_chat.go:107`). Comment in proto: "gRPC-Web stream - NOT batchexecute" (`proto/notebooklm/v1alpha1/orchestration.proto:1387-1395`). Same RPC id `laWbsf` is bound to it in the JS bundle.
- Uploads: Scotty resumable upload to `https://notebook.google.com/upload/_/` (`notebooklm/client_upload.go:27`).
- `--direct-rpc` flag (README "Selected Flags"; `cmd/nlm/cli_parser.go:296`, `cmd/nlm/main.go:204,414`): it is NOT a transport switch. It sets `UseDirectRPC`, which makes `CreateAudioOverviewWithOptions` use the original single-purpose RPC `AHyHrd` (`createAudioOverviewDirectRPC`, `notebooklm/client_audio.go:38-42,102-103`) instead of the universal artifact RPC `R7cb6c`, but only when the request is the plain default (no source ids, deep-dive, default length, language en); `GetAudioOverview` likewise uses `getAudioOverviewDirectRPC` (`:167-168`). Every other path always uses batchexecute.
- Wire modeling: protobuf definitions under `proto/notebooklm/v1alpha1/*.proto` (5051 lines: `orchestration.proto` 3921, `notebooklm.proto` 693, `sharing.proto` 334, `rpc_extensions.proto` 103). Positional JSON arrays map to proto field numbers (array index = field number - 1). Custom proto options carry the batchexecute metadata (`rpc_id`, `arg_format`, `grpc_endpoint`, `object_encoded`, `omit_trailing_nulls`, `json_bool`) (`proto/notebooklm/v1alpha1/rpc_extensions.proto:9-93`). Codegen with `buf generate` + custom templates in `proto/templates/` writes `gen/method/*_encoder.go` and `gen/service/*_client.go`; hand-verified encoders live in `internal/method/` and must not be regenerated (`docs/dev/codegen.md:1-50`).

### What "lossless-verified" means (answer to the brief)

It is a concrete, test-backed claim, not marketing:

1. `internal/beprotojson` marshals/unmarshals proto messages to/from batchexecute positional arrays (`internal/beprotojson/beprotojson.go:19-80`, object-encoded sparse form at `:44-55`).
2. `nlm betool --verify` decodes a captured wire payload to a proto message, **re-encodes it, and diffs the result against the original wire JSON**. Any wire position the descriptor does not model is reported as `missing_fields` (grouped by normalized path), and `roundtrip_lossless: true` means zero deltas (`internal/betool/betool.go:405-452`).
3. A corpus audit (`internal/betool/corpus.go`) runs this over every recorded request/response; docs claim 3723 lossless corpus records (`docs/dev/rich-types-and-proto-alignment.md:14`) and per-family pass columns in `docs/nlm-proto-migration-ledger.md` (e.g. streaming chat: "231 real payload frames from three responses", ledger row "Streaming chat").
4. Fixtures are real HAR-derived captures: `internal/batchexecute/testdata/{requests,responses}/*.txt`, `notebooklm/testdata/*.json|*.httprr`, `cmd/nlm/testdata/*_wire.json`, recorded by a vendored `httprr` (record/replay RoundTripper; cookies and Authorization redacted) (`internal/httprr/httprr.go`, `docs/dev/http-capture.md`).

Caveats worth knowing: the corpus was kept under `/tmp/nlm-traffic`, which macOS sweeps, so the gate "was never run" at one point (`docs/dev/rich-types-and-proto-alignment.md:137-141`). The corpus itself is not in the repo. Treat the 3723 figure as the author's claim [unverified independently].

## RPC catalog

Common request shape (all batchexecute RPCs):

- URL: `https://notebook.google.com/_/LabsTailwindUi/data/batchexecute?rpcids=<ID>&source-path=<path>&bl=<build label>&f.sid=<session id>&hl=en&rt=c&_reqid=<n>[&authuser=<n>]`
  - `rpcids`: first RPC id only, even for multi-RPC envelopes (`batchexecute.go:169`).
  - `source-path`: `/notebook/<id>` when a notebook id is known, else `/` (`internal/notebooklm/rpc/rpc.go:370-374`).
  - `bl` default `boq_labs-tailwind-frontend_20260406.14_p0`, overridden by env `NLM_BL_PARAM` (set from the page's `cfb2h`); `f.sid` default is a hard-coded stale value, overridden by `NLM_SESSION_ID` (page `FdrFJe`) (`rpc.go:292-299`). **Hard-coded fallbacks are a smell**; we must always fetch live values.
  - `hl=en`, `rt=c` (chunked) set in `rpc.go:309-314`; without `rt` the server returns a plain JSON array (`batchexecute.go:180`).
  - `_reqid`: base `1500000000 + rand(1e8)`, then `+= 100000` per call, mutex-guarded (`batchexecute.go:975-1003`). Chat uses `UnixMilli() % 1e6` (`client_chat.go:425`).
  - `authuser` URL param AND `x-goog-authuser` header, only for non-default account; both omitted for account 0 (`internal/authuser/authuser.go:5-12`, `notebooklm/client.go:222-226`).
- Body: `f.req=<url-encoded [[ [id, "<args JSON string>", null, "generic"], ... ]]>&at=<SNlM0e token>&` (trailing `&`, `f.req` first) (`batchexecute.go:196-205`). Percent-encoding: `url.QueryEscape` in the client; the codec in `wire.go:186-188` uses `%20` not `+` to mimic `encodeURIComponent`.
- Headers (`internal/notebooklm/rpc/rpc.go:335-345`): `content-type: application/x-www-form-urlencoded;charset=UTF-8`, `origin: https://<host>`, `referer: https://<host>/`, `x-same-domain: 1`, `accept: */*`, `accept-language`, `cache-control: no-cache`, `pragma: no-cache`, `cookie: <flat header>`. Also `authorization: SAPISIDHASH <ts>_<sha1(ts SAPISID origin)>` computed when SAPISID is in cookies (`batchexecute.go:134-143,264-275`). Chrome client hints (`sec-ch-ua*`, `Sec-Fetch-*`, UA) only on upload paths (`notebooklm/client.go:254-273`).
- Chat extra header: `x-goog-ext-353267353-jspb: [null,null,null,282611]` (`client_chat.go:473,522`).
- Token sources: `at` = `WIZ_global_data.SNlM0e`, `f.sid` = `FdrFJe`, `bl` = `cfb2h`, read via CDP `Evaluate` (`internal/auth/auth.go:1095-1097`) or by regexing the HTML of `GET https://notebook.google.com/` (`nlmauth/refresh.go:243-275`, regex `"<field>"\s*:\s*"((?:\\.|[^"\\])*)"`).

Method table, extracted from `proto/notebooklm/v1alpha1/*.proto` (`rpc_id` and `arg_format` options). `%x%` are request-message placeholders. "ctx" = request-context block (`RequestContext`, version/surface/capabilities). Response shapes are proto messages in the same files; positional details are in the generated types. Rows marked * were seen with different arg formats in two proto files (legacy `notebooklm.proto` vs. current `orchestration.proto`); orchestration wins.

| RPC id | Purpose (proto name) | Request args (`arg_format`) | Response | Source |
|---|---|---|---|---|
| wXbhsf | ListRecentlyViewedProjects | `[null, 1, null, [2]]` (pinned by encoder test, ledger "Projects") | `ListRecentlyViewedProjectsResponse`; fixture `[[[ "nbname2", [[[ [id], title, [null, ...stats, [secs,nanos], [convId,[ts]], type], [null,2] ], ...]]]]]` (`internal/batchexecute/testdata/list_notebooks.txt`) | orchestration.proto:1359 |
| CCqFvf | CreateProject | `[%title%, %emoji%]` | `Project` | orchestration.proto:1343 |
| rLM1Ne | GetProject (also lists sources) | `[%project_id%, null, %context%, null, %include_artifacts%, %filter%]` | `Project` (sources inside) | orchestration.proto:1351 |
| WWINqb | DeleteProjects | `[%project_ids%]` | Empty | :1347 |
| s0tc2d | MutateProject / MutateProjectCover (rename, emoji, description) | `[%project_id%, %updates%]` (+ctx for cover) | `Project` | :1367,1371 |
| fejl7e | RemoveRecentlyViewedProject | `[%project_id%]` | Empty | :1375 |
| ub2Bae | ListFeaturedProjects | `[[ctx_version, null, ctx_surface, ctx_caps]]` | `ListFeaturedProjectsResponse` (fixture `internal/beprotojson/testdata/ub2Bae_list_featured_response.json`) | :1355 |
| izAoDd | AddSources (url/text/youtube/drive/base64) | `[%sources%, %project_id%, [2, null, [1], [1, null*9, [1, 3]]]]` | `AddSourcesResponse` | :1231 |
| o4cbdc | AddFileSource (register upload; JS name AddTentativeSources) | `[[[filename]], project_id, [2], [1, null*9, [1]]]` (hand-built, `notebooklm/client_upload.go:735-750`) | source id | rpc.go:63 |
| tGMBJ | DeleteSources | `[%source_ids%, %context%]` (legacy `[[%ids%]]`; hand code uses `[[[id]]]`, see `notebooklm/client_upload.go:199`) | Empty | :1239 |
| hizoJc | LoadSource (full text, fragments) | `[%source%, %mode%, [2,null,[1],[1,null*9,[1,3]]]]` | `LoadSourceResponse` | :1270 |
| b7Wfje | MutateSource (rename) | `[null, %source_id%, %updates%, [2,...]]` | `Source` (response empty in corpus) | :1274 |
| FLmJqe | RefreshSource | `[null, %source%, %context%]` | `Source` | :1278 |
| yR9Yof | CheckSourceFreshness | `[null, %source%, %context%]` | `CheckSourceFreshnessResponse` | :1235 |
| tr032e | GenerateDocumentGuides / per-source guide | `[%sources%, [2,...]]`; per-source form hand-coded | guide tuple | :1597 |
| VfAZjd | GenerateNotebookGuide | `[%project_id%, [2,null,[1],[1,null*9,[1,3]]]]` | `GenerateNotebookGuideResponse` | :1406 |
| yyryJe | ActOnSources (transform actions, mind-map legacy) | `[%source_groups%, %options%, null*5, %chat_options%, %prompt_history%, %conversation_id%]` | `ActOnSourcesResponse` | :1223 |
| Es3dTe | DiscoverSources | `[%project_id%, %query%]` | `DiscoverSourcesResponse`; no HAR evidence (`rpc.go:66-75`) | :1259 |
| laWbsf | GenerateFreeFormStreamed (chat) | see Streaming/chat | stream | :1386,1402 |
| hPTbtc | GetConversations | `[%context%, null, %project_id%, %limit%]` | `GetConversationsResponse` | :1453 |
| khqZz | GetConversationHistory | `[%context%, null, null, %conversation_id%, %limit%]` | `GetConversationHistoryResponse` (lossless, 5 corpus frames) | :1457 |
| J7Gthc | DeleteChatTurns ("bundle name inferred") | `[%options%, %conversation_id%, null, %unknown_4%]` | `DeleteChatTurnsResponse` | :1495 |
| e3bVqc | DeleteChatHistory / GetDeepResearchSessions / PollDeepResearch (polymorphic by args; JS name ListDiscoverSourcesJob) | `[%context%, null, %project_id%]` / `[null, null, %project_id%]` / `[%context%, null, %job_handle%]` | varies | :1461,1534,1542 |
| Ljjv0c | StartFastResearch (JS DiscoverSourcesManifold) | `[[%query%, 1], null, 1, %project_id%]`; response `[conversation_id]` (HAR-verified 2026-04-17, `rpc.go:116`) | | :1508 |
| QA9ei | StartDeepResearch (JS DiscoverSourcesAsync) | `[null, [1], [%query%, 1], 5, %project_id%]` (wire form `[ctx,null,query,mode,project_id]`); response `[research_id, conversation_id]` | | :1515,1522 |
| LBwxtb | BulkImportFromResearch / DeleteDeepResearch (JS FinishDiscoverSourcesRun) | `[%context%, %marker%, %conversation_id%, %project_id%, %sources%]` ; delete: `[null,[1],%conv_id%,%project_id%]` | | :1554,1574 |
| R7cb6c | CreateUniversalArtifact (audio, video, report, quiz/flashcards, slides, infographic, data table, mind map) | `[%context%, %project_id%, %options%, null, null, %unknown_6%]` | `Artifact` | :1313 |
| AHyHrd | CreateAudioOverviewLegacy | `[%project_id%, %audio_type%, %instructions%]` (commit 297ebc5 "fix audio-create to use direct RPC (AHyHrd)") | `AudioOverview` | :1869 |
| VUsiyb / sJDbic | GetAudioOverview / DeleteAudioOverview | `[%project_id%]` | AudioOverview / Empty | :1295,1299 |
| gArtLc | ListArtifacts / QueryArtifacts | `[%context%, %project_id%, %filter%]` / `[%artifact_types%, %project_id%, %filter%]` | `ListArtifactsResponse` | :1205,1217 |
| v9rmvd | GetArtifact (also interactive HTML/slides) | `[%artifact_id%, %context%]` | `Artifact` (fixture `notebooklm/testdata/v9rmvd_slide_artifact.json`) | :1175 |
| xpWGLf | CreateArtifact | `[%context%, %project_id%, %artifact%]` | `Artifact` | :1165 |
| DJezBc / rc3d8d / V5N4be | UpdateArtifact / RenameArtifact / DeleteArtifact | `[%artifact%, %update_mask%]` / hand-coded / hand-coded | | :1179,1183,1188 |
| KmcKPe | ReviseArtifact (revise slide) | `[%context%, %artifact_id%, %instructions%]` | `Artifact` | :1637 |
| Rytqqe | GenerateArtifact (retry) | `[%context%, %artifact_id%]` | `Artifact` | :1764 |
| XgrPMd | CancelGeneration | `[null, %generation_id%]` | | :1782 |
| Krh3pd | ExportToDrive (Docs/Sheets) | not in arg_format (hand-coded) | `ExportToDriveResponse` | :1790 |
| otmP3b | GenerateArtifactSuggestions (JS GeneratePromptSuggestions) | `[[2,null,[1],[1,null*9,[1,3]]], %project_id%, %source_refs%, %variation%, null, %prompt%]` | | :1650 |
| ciyUvf | GenerateReportSuggestions | `[%context%, %project_id%, %source_ids%]` | `GenerateReportSuggestionsResponse` | :1421 |
| sqTeoe | GetAudioFormats (JS GetArtifactCustomizationChoices) | `[%context%, null, %mode%]` | | :1586 |
| CYK0Xb / cYAfTb / AH0mwd / cFji9 | CreateNote / MutateNote / DeleteNotes / GetNotes | `[%project_id%, ...]` / `[%project_id%, %note_id%, %updates%, [2,...]]` / `[%project_id%, null, %note_ids%, %context%]` / `[%project_id%, null, %since%, %context%]` | Rich notes (`RichDocument`) | :1319-1334 |
| I3xc3c / agX4Bc / le8sX / GyzE7e | GetLabels / CreateLabel+MutateLabelsMode (autolabel) / MutateLabel / DeleteLabels | `[%context%, %project_id%, %scope%]` ... | autolabel RPC can return gRPC code 4 after ~60 s (`errors.go:145-149`) | :1697-1754 |
| ZwVcOc / hT54vc | GetOrCreateAccount (quota, notebook limit 500) / MutateAccount | `[[ctx_version, null, ctx_surface, ctx_caps]]` / `[%update%, %context%]` | `Account` | :1468,1472 |
| AUrzMb | GetProjectAnalytics | `[%project_id%, null, %requested_at%, %mode%]` | | :1443 |
| uNyJKe | SubmitFeedback | `[%turn%, %feedback%, %content%, null, null, %options%]` | | :1447 |
| QDyure / JFMDGd / RGP97b | ShareProject / GetProjectDetails / ShareAudio | `[%project_id%, %settings%]` / `[%share_id%, %context%]` / `[%share_options%, %project_id%]` | | sharing.proto:278-288 |
| te3DCe | CopyProject | `[%context%, %source_project_id%, %new_title%]` | | :1856 |
| Of0kDd / eyWvXc | FetchInteractivityToken (JS GetIceConfig) / SDPExchange (JS SendSdpOffer) — live voice (WebRTC) | | | :1662,1672 |
| EnujNd, LQhfEb, Fxmvse, ulBSjf, mVtEUb, preRPe, rtY7md, OmVMXc, ozz5Z, HpN0Ub, DemIHe, Zbrupe, n3dkHd, uK8f7c, lCjAd, BeTrYd, exXvGf, pGC7gf | ListModelOptions, UpdateProjectUserState, UpsertArtifactUserState, GetArtifactUserState, ListExpertIntelligenceContent, GenerateAccessToken, GetMagicView, ReportContent, LogEvent, LogInteractionEvent (HpN0Ub), UpdateFeaturedNotebookStatus, CancelDiscoverSourcesJob, CreateAccessRequest, GenerateMagicView, GenerateOutline, GenerateSection, StartDraft, StartSection | mostly telemetry/low-value; see proto lines 1805-1882 | | |
| Guidebook ids: ARGkVc, EYqtU, YJBpHc, R6smae, LJyzeb, OTl0K, itA0pc | Delete/Get/ListRecentlyViewed/Publish/GetDetails/Share/GenerateAnswer Guidebook (Guidebooks service, separate bundle slice) | | | sharing.proto:306-330 |

71 orchestration+sharing bindings were observed in the 2026-04-23 JS bundle (`internal/notebooklm/rpc/rpc.go:27-31`). The JS bundle (`www.gstatic.com *.jsonl` in a HAR) is declared the source of truth for rpc_id to service-method names (`rpc.go:19-42`).

### Request payload examples (hand-built, verified in code)

Add text source, add URL source, YouTube, file registration (`notebooklm/client_upload.go`):

- text: `[[[null,[title, content],null,2]], projectID]` (lines 437-451) — note: older/shorter tail; newer proto form appends the `[2,null,[1],[1,null*9,[1,3]]]` block. **Both appear in the repo; sahaand/nblm notes say the short tail is now rejected with INVALID_ARGUMENT** (see sahaand note). Treat the long form as current and re-verify.
- URL: `[[[null,null,[url]]], projectID]` (lines 825-840)
- YouTube: source spec array with `[url]` at index 7 and trailing `1`, then `projectID, [2], [1,null*9,[1]]` (lines 898-916)
- Upload: (1) RPC `o4cbdc` registers filename and returns SOURCE_ID; (2) `POST /upload/_/` with JSON body `{"PROJECT_ID","SOURCE_NAME","SOURCE_ID"}` **in that key order** (Go map order breaks it, `client_upload.go:578-606`) and headers `X-Goog-Upload-Command: start`, `X-Goog-Upload-Protocol: resumable`, `X-Goog-Upload-Header-Content-Length`; response header `X-Goog-Upload-Url`; (3) `POST` bytes to that URL with `X-Goog-Upload-Command: upload, finalize`, `X-Goog-Upload-Offset: 0` (`:682-731`). Register-before-upload ordering is required (`:495-501`). Upload sometimes returns 5xx with `X-Goog-Upload-Status: final`, which is treated as finalized (`:518-555`). Text ceiling `MaxTextSourceBytes = 10 MiB` (`:429`); server fails around 13 MB with code 9 (`notebooklm/errors.go:20-27`).

## Streaming/chat

- Request: `POST <chatEndpoint>?bl&f.sid&hl&rt=c&_reqid=<UnixMilli%1e6>[&authuser]`. Body `f.req=[null, "<inner args JSON string>"]&at=<token>` (`client_chat.go:395-428`). Note the envelope is `[null, "<string>"]`, **not** the batchexecute `[[[id,...]]]`.
- Inner args (positional, = proto `GenerateFreeFormStreamedWireRequest`, `orchestration.proto:790-800`): `[ sources, prompt, history, options, conversation_id, null, null, notebook_id, seq ]` where
  - `sources` = `[[[id]], [[id]]...]` i.e. each id nested 3 levels in a list of `ChatSourceSelection` (`client_chat.go:298-304`; doc comment line ~359: `[[[[source_ids]]],prompt,history,[2,null,[1],[1]],conv_id,null,null,notebook_id,seq]`)
  - `history` = `[[content, null, role]...]` or null
  - `options` = `[2, null, [1], [1, null*9, [1,3]]]`-style: mode=2, citation_modes `[1]`, follow-up `{enabled:1, modes:[1,3]}` at field 11 (`ChatStreamOptions`, `orchestration.proto:803-815`)
  - `conversation_id` = client-generated UUID v4, `seq` starts at 1, optional draft/parent response ids at [5],[6] (`client_chat.go:264-296`)
- Empty source list is resolved to all notebook sources via a fetch first (`resolveSourceIDs`, `client_chat.go:211`) because an unpinned question is answered against a server-chosen subset that changes as the notebook grows (README "Chat" note).
- Response: chunked body (`)]}'`, length line, JSON line, ...). Each frame line starts with `["wrb.fr",...]`; `envelope[2]` is an inner JSON string. The frame's text is the **cumulative** answer, not a delta (`client_chat.go:617-760`). Client diffs against the previous snapshot by longest common prefix (rune-boundary safe) and also carries `Full` because the server can revise earlier text (e.g. citation `[2, 3]` becomes `[2-5]`). Thinking phase frames arrive first (full replacement text; wire phase marker or "starts with `**`" heuristic), then answer frames. Dokkabei notes the same type flag: 1 = answer, 2 = thinking.
- Response proto (`GenerateFreeFormStreamedWireResponse`, `orchestration.proto:812-822`): `answer{chunk, conversation metadata, RichDocument document}`, `citations[Grounding]`, `source_mappings[range, source_indices]`, `follow_up_text`, `is_final`(JSON bool), `follow_ups`, `complete`, `token`.
- Citations: `Citation{SourceIndex, SourceID (chunk handle), ParentSourceID (notebook source UUID), StartChar, EndChar, Confidence, Excerpt, ExcerptRuns, SourceStart/End}` (`client_chat.go:60-79`). SourceID is **not** in the source list; resolve title through ParentSourceID. History frames carry citation data in content block `[4]` (`:1154-1187`).
- Timeouts: first parsed chunk up to 5 min, progress 5 min, idle read 5 min, total 15 min (`client_chat.go:108-125`). Progress is measured on parsed chunks, not socket reads, because keep-alive bytes would otherwise mask a stall (`:546-552`).
- Guards: 1 MiB answer cap and repeated-block detection stop degenerate streams (exit code 10; `NLM_MAX_CHAT_BYTES`, `NLM_CHAT_REPEAT_GUARD`) (README "Chat output guards"). Empty answer is a retryable error (commit `185ed32`).
- Auth failure on chat arrives as HTTP 200 with an error frame and no content; `classifyChatError` scans for bare ints 16 / 277566 / 277567 (`client_chat.go:898-956`).
- Rate limits: none enforced client-side. [unverified whether server returns 429 on chat]

## Auth lifecycle

- Credentials = `NLM_AUTH_TOKEN` (SNlM0e `at`) + `NLM_COOKIES` (flat Cookie header) + optional `NLM_AUTHUSER`; stored in `~/.nlm/env` (mirror of current identity) and `~/.nlm/identities/<name>.env` (mode 0600, dir 0700), current identity in `~/.nlm/current` (`nlmauth/credentials.go:36-45`, `nlmauth/store.go:75-120,200-210`). `NLM_CREDENTIAL_STORE` selects a store kind (`store.go:257`).
- **Cookie source: nlm's own browser profile.** `nlm auth login` launches Chromium (Brave/Chrome/Edge) with `--user-data-dir=~/.nlm/browser/<identity>` and a fixed `--remote-debugging-port` (so Google does not flag automation), waits up to 5 minutes for a human sign-in, then reads `WIZ_global_data.{SNlM0e,FdrFJe,cfb2h}` and `network.GetCookies` over CDP (`internal/auth/auth.go:550-612,1027-1175`, `cmd/nlm/auth.go:183-184`). It does not read the user's personal profile by default (README "Authentication").
- **No cookie decryption anywhere.** There is no DPAPI, AES, Keychain, libsecret or App-Bound Encryption code in tmc/nlm. The "import" path (`-profile <name>`, `-all`) copies `Cookies`, `Login Data`, `Web Data`, `Preferences` into a temp dir, writes a stub `Local State` with `"encrypted_key":""`, launches the browser on the temp dir and reads cookies via CDP (`auth.go:663-710`, files list `:673-682`, stub `:704`). On Windows that copy cannot be decrypted by the new browser instance (DPAPI key is in the real `Local State`), and Chrome 127+ App-Bound Encryption also blocks it. [unverified on Windows; reasoning from the stub and from missdeer's README statement]. Default Windows profile root `%LOCALAPPDATA%\Google\Chrome\User Data`, also Brave and Canary dirs (`internal/auth/chrome_windows.go:11-77`); Linux `~/.config/google-chrome`, `~/.config/BraveSoftware/Brave-Browser` (`chrome_linux.go`); macOS app paths for Chrome/Canary/Chromium/Edge/Brave/Safari (`chrome_darwin.go:22-35`).
- Cookies are not filtered: all cookies returned for the notebook URL are joined. Validity check requires one of `SID/HSID/SSID/APISID`, token length >= 20, cookie string >= 50 chars (`auth.go:1170-1190`).
- Remote CDP attach: `-cdp-url ws://localhost:9222` (`auth.go:134-150`).
- Expiry detection: HTTP 401 (`BatchExecuteError.Unwrap` -> `ErrUnauthorized`, `batchexecute.go:76-81`); batchexecute in-band code 16 / 277566 / 277567; CLI then re-harvests the cached profile and retries once unless `NLM_AUTO_REFRESH=false` (`cmd/nlm/auth.go:663-665`). Exit code 3 = auth required, 9 = login attempted and failed (README "Exit codes").
- Silent credential refresh without a browser: `nlmauth.Refresh()` fetches the app page for `FdrFJe`/`cfb2h`/`gsessionid`, then `POST https://signaler-pa.clients6.google.com/punctual/v1/refreshCreds?key=<public web key>&gsessionid=...` with `Authorization: SAPISIDHASH`, `Content-Type: application/json+protobuf`, body `["tZf5V3ry"]` (`nlmauth/refresh.go:18-136`). This keeps the session warm; it does not mint cookies. [unverified that it extends cookie lifetime]
- Multi-account: `--authuser N` / `NLM_AUTHUSER`, named identities with `--identity` (`auth login --as work`).
- Safety: debug output masks cookies/tokens/Authorization (commit `eb168c8`). Fallback `NLM_HTTP_TRANSPORT=curl` shells out to `curl -4` writing a temp config with the cookie in it (`batchexecute.go:809-933`) — avoid.

## Reliability

- Retry: `MaxRetries` 3, delay 1 s x 2^(n-1), cap 10 s, **no jitter**, no `Retry-After` handling (`batchexecute.go:297-319,707-737`). Retries on HTTP 429/500/502/503/504 and on error-string matches (`connection refused`, `reset`, `i/o timeout`, `TLS handshake timeout`, `EOF`, `broken pipe`, `no such host`, ...) (`batchexecute.go:1012-1053`). It retries **all** RPCs including non-idempotent ones (create notebook/source) — a duplicate-creation risk.
- Error typing: `ErrorType` enum (Auth, Authorization, RateLimit, NotFound, InvalidInput, Server, Network, PermissionDenied, ResourceExhausted, Unavailable, DeadlineExceeded, AlreadyExists), `ErrorCode{Code, Type, Message, Description, Retryable}` table covering gRPC codes 1-16, HTTP 400-504, and private numbers 277566/277567 (auth), 80620 (access denied), 324934 (rate limit), 143 (not found) (`errors.go:10-340`). `APIError.IsRetryable()` (`:89`). Code 9 FAILED_PRECONDITION is polysemic (source cap ~300, oversize, malformed, busy) so call sites add sentinel errors: `ErrSourceCapReached, ErrSourceTooLarge, ErrNotebookCapReached, ErrArtifactGenerating, ErrArtifactNotFound, ErrNoteNotFound, ErrResearchPolling, ErrNotebookNotAccessible, ErrAuthExpired` (`notebooklm/errors.go`). Code 4 DEADLINE_EXCEEDED on autolabel is normal (`errors.go:145-149`).
- In-band status: frame `["wrb.fr",id,null,null,null,[code],"generic"]` means failure with HTTP 200; `Response.Status` records code, and Data must not be unmarshaled as a message (`batchexecute.go:44-48,537-544`, `chunked.go:255-271,316-328`). Known codes seen: 3 INVALID_ARGUMENT, 13 INTERNAL, 16 UNAUTHENTICATED.
- Exit code map for CLI (0 ok, 1 generic, 2 bad-args, 3 auth, 4 not-found, 5 precondition, 6 transient, 7 busy, 8 stale-output, 9 auth-failed, 10 runaway-output, 11 partial-write) (README "Exit codes").
- IPv4 preference: custom dialer resolves A records and dials tcp4 first (sandboxed terminals with broken IPv6) (`batchexecute.go:774-813`).
- No caching, no singleflight, no concurrency limiter in the client. A disk cache exists for source titles only (`cmd/nlm/source_title_cache.go`).

## Performance notes

Not measured by the author. Observations: one `http.Client`, `MaxIdleConns 100`, `IdleConnTimeout 90s` (`batchexecute.go:805-808`) so connection reuse works; `io.ReadAll` of the whole batchexecute body (fine); chat reads via `bufio.Reader.ReadString` to avoid scanner token caps (`client_chat.go:767-780`). A fresh `httpClientWithTimeout` is built per chat call (loses connection pool) (`client_chat.go:477`, `notebooklm/client.go:139-140`). Chat does a source-list fetch before every unpinned ask.

## Response decoding (reusable design)

Pipeline in `internal/batchexecute`:

1. `stripAntiHijackPrefix` removes `)]}'` in contiguous or character-per-line layouts (`wire.go:285-302`).
2. If the body starts with a digit it is chunked; if it is only digits it is a bare numeric error code, mapped to a synthetic `Response{ID:"numeric"}` (`chunked.go:29-36`, `batchexecute.go:473-476`).
3. `sanitizeJSONControlChars` escapes raw U+0000-U+001F inside strings (NotebookLM titles/notes contain literal newlines) (`unescape.go:101-143`).
4. Chunk splitting: the **primary** parser ignores the declared lengths and extracts top-level balanced `[...]` with a quote/escape-aware scanner (`chunked.go:44-56,138-174`). `wrbfr.go` is the strict length-aware variant used for streams: tries the declared count as bytes, as Unicode code points, and as **UTF-16 code units** (the real unit, JS string length), plus a continuation-frame fallback, and requires `json.Valid` plus a newline delimiter (`wrbfr.go:16-148`).
5. Frame `["wrb.fr", id, payload, null, null, status, index]`: payload at [2] is a JSON string, un-escaped up to 3 layers (`unescape.go:19-60`); status at [5]; index at [6] ("generic" or number). Also handles `"er"`/`"e"` error markers. Trailing frames `["di",ms]` and `["af.httprm",...]` are timing metadata and are ignored (visible in `testdata/list_notebooks.txt`).
6. `IsErrorResponse` maps numeric/`[code]`/object payloads to `APIError` via the dictionary.

Codec for tools: `DecodeRequest/EncodeRequest/DecodeResponse/EncodeResponse` round-trip raw wire <-> JSON (`wire.go:61-276`), used by `nlm betool`.

## MCP tools

Server: `internal/nlmmcp/server.go` (52 lines) + `tools.go` (761 lines) on `github.com/modelcontextprotocol/go-sdk v1.2.0` (`go.mod:9`). `mcp.NewServer(impl, &ServerOptions{Instructions: ...})`, `mcp.AddTool(server, &mcp.Tool{Name, Description, Annotations}, typedHandler)` with typed input structs and `jsonschema:"..."` tags, `server.Run(ctx, &mcp.StdioTransport{})`. Tool annotations: read-only / mutating (`DestructiveHint:false`) / destructive (`DestructiveHint:true`), all `OpenWorldHint:false` (`server.go:12-26`). Errors are returned as `IsError` text results (not Go errors). Pagination helper `pageResult[T]{items,total,offset,limit,returned,has_more,next_offset}` with default 50 / max 100 (`tools.go:165-172`). `watch_deep_research` uses `notifications/progress` (`research_watch.go`, tested with `mcp.NewInMemoryTransports()`, `research_watch_test.go:139`).

**25 tools are registered in code** (`grep -c mcp.AddTool internal/nlmmcp/tools.go` = 25); `docs/mcp.md` claims 39 (25 direct + 14 generation) which does not match the code [doc/code drift]. Names: `list_notebooks, list_sources, list_notes, create_note, add_source_text, delete_note, list_artifacts, create_audio_overview, get_audio_overview, rename_artifact, share_audio, create_video_overview, create_app_artifact (prototype|mindmap|canvas), create_slide_deck, read_note, set_instructions, get_instructions, start_deep_research, poll_deep_research, watch_deep_research, create_notebook, delete_notebook, delete_source, add_source_url, generate_chat`. Params of note: every tool takes `notebook_id`; generation tools take `source_ids[]`, `language`, `length` (default|short|long), `audio_type` (deep-dive|brief|critique|debate), video `style` (auto|classic|whiteboard); `watch_deep_research` takes `poll_interval_ms` (default 2000) and `max_wait_seconds` (default 600) (`tools.go:19-155`). The server does not read local files; caller supplies content (`docs/mcp.md`).

## Tests and quality

- `go test ./...`, `gofmt`, `go vet`, `deadcode` workflow (`.github/workflows/ci.yml`, `deadcode.yml`); ubuntu only. 
- testscript (`rsc.io/script`) golden CLI tests in `cmd/nlm/testdata/*.txt`; command-parity golden JSON `cmd/nlm/testdata/command_parity*.golden.json`.
- Record/replay: `internal/httprr` + `notebooklm/testdata/*.httprr`/`.txtar` (real HAR captures with redaction).
- Unit tests for wire codec, retry (`retry_test.go`), errors (`errors_test.go`), chunk framing (`wire_test.go`), many proto-vs-legacy equivalence tests.
- Large surface (682 files, 5 MB of generated protobuf); some drift: `KAFKA2306/nlm` is a stale fork of this repo.

## Versions

`git tag` : v0.1.0 (2026-07-31, `23a4c45`), v0.1.1 (2026-08-04). 674 commits since 2024-11-17. Notable history:

- 2024-11-17 `98e3132` initial NotebookLM CLI; 2024-12-04 audio/generation features, Safari auth.
- 2026-03-09 `297ebc5` AHyHrd direct-RPC for audio; `8a647c1` chat request format aligned to `grpc_arg_format`.
- 2026-07-27 `1ef43ba` switch host/origin to notebook.google.com; omit authuser for default account.
- 2026-09-07..09-22: labels, source selectors, rich rendering, chat live view, structured diagnostics log (`7b4bd1c`), mask Authorization (`eb168c8`), empty-answer retryable (`185ed32`), exit-code help (`6f198a1`).

## Adopt

1. **Proto-described positional wire model + round-trip verification.** Describe each RPC's args/response as a Go struct (or proto) with index=field-1, and run a "decode -> re-encode -> diff" test over recorded fixtures to detect unmodeled slots. Evidence: `rpc_extensions.proto:9-93`, `beprotojson.go:19-80`, `betool.go:405-452`. We can start without protobuf (struct tags + a small reflect codec) but keep the verify idea.
2. **Length-aware frame splitter that counts UTF-16 units**, with bytes/runes/UTF-16/continuation fallbacks, validated by `json.Valid` (`internal/batchexecute/wrbfr.go:16-148`) plus the quote-aware balanced-bracket scanner as the fallback (`chunked.go:138-174`).
3. **Control-char sanitizer and multi-layer unescape** for NotebookLM text with literal newlines (`unescape.go:19-143`).
4. **In-band gRPC status at frame position 5** surfaced as `Status` and mapped through an error dictionary with `Retryable`, plus sentinel errors for the polysemic code 9 (`chunked.go:255-328`, `errors.go:10-340`, `notebooklm/errors.go`).
5. **Chat as a separate streaming endpoint** with cumulative-snapshot diffing, `Full` authoritative text, thinking vs answer phases, citations resolved through `ParentSourceID`, progress-based (not read-based) stall timeout, size/repeat guard (`client_chat.go:108-125,618-765`).
6. **Auth by owning a dedicated browser profile** (`~/.nlm/browser/<identity>`) with a fixed CDP port, harvesting `SNlM0e/FdrFJe/cfb2h` + cookies over CDP, and a cheap HTTP-only page-state refresh (`auth.go:550-612,1027-1175`, `nlmauth/refresh.go:243-275`). This sidesteps DPAPI/ABE completely.
7. **Resumable-upload handshake details** (register id first, metadata key order, `X-Goog-Upload-*`, accept `final` on 5xx) (`client_upload.go:495-731`).
8. **go-sdk usage pattern**: typed `AddTool`, annotations helper, `IsError` text results, pagination envelope, `watch_*` tool with progress, `NewInMemoryTransports` tests (`internal/nlmmcp/`).
9. **Exit-code taxonomy** and structured `slog` diagnostics log opt-in (commit `7b4bd1c`). Map to MCP error `code` fields rather than process exit codes.
10. **Mask secrets in debug output** (`batchexecute.go:88-120`).

## Avoid

1. Hard-coded fallback `bl` and `f.sid` (`rpc.go:292-299`) — they silently go stale. Fail loudly or refetch.
2. Retrying non-idempotent RPCs, no jitter, no `Retry-After` (`batchexecute.go:297-367`). Retry only reads and known-idempotent writes; use jitter.
3. `context.Background()` inside MCP handlers (`internal/nlmmcp/tools.go:203,228,252,272` and others) — cancellation never reaches Google. Pass the handler `ctx`.
4. No `context.Context` on much of the old library surface (41 `context.Background()` calls at one point, `docs/dev/rich-types-and-proto-alignment.md` section 2). Make ctx mandatory from day one.
5. `curl -4` shell-out transport with cookies in a temp file (`batchexecute.go:865-933`).
6. Using only `rpcids=<first id>` for multi-RPC envelopes and returning only `responses[0]` (`batchexecute.go:169,446-460`): batching is effectively unsupported. We want a real multi-RPC `Execute`.
7. Copy-profile "import" with a stub `Local State` (`auth.go:704`) — does not work with DPAPI/ABE.
8. Duplicate hand-rolled and proto decoders (`rich-types-and-proto-alignment.md` section 0-1); pick one decode path.
9. Huge codegen + `gen/` surface (5k-line proto, 3.9k in one file) for a project that only needs ~40 RPCs; adopt the idea, not the volume.
10. Two source-add argument tails coexist (`client_upload.go:437` short vs proto long form); confirm which Google accepts today before copying.

## Evidence

Read in full: `internal/batchexecute/{batchexecute,chunked,wrbfr,wire,errors,unescape}.go`; `internal/notebooklm/rpc/rpc.go:1-380`; `notebooklm/client_chat.go:60-130,260-560,617-790,898-960`; `notebooklm/client_upload.go:425-500,575-625,732-930`; `nlmauth/refresh.go`; `internal/auth/auth.go:481-720,1027-1190`; `internal/auth/chrome_{windows,linux,darwin}.go`; `internal/nlmmcp/{server,research_watch}.go`; `proto/notebooklm/v1alpha1/rpc_extensions.proto`; all `rpc_id/arg_format` options in the three service protos (script-extracted); `docs/mcp.md`; README; `docs/dev/{codegen,rich-types-and-proto-alignment}.md`; `docs/nlm-proto-migration-ledger.md`. Git: `git log`, `git tag`, `git show 1ef43ba`. No code executed.
