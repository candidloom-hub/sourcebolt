# Robiton/notebooklm-mcp-cli (PyPI: notebooklm-enterprise-mcp)

- URL: https://github.com/Robiton/notebooklm-mcp-cli
- Language / runtime: Python, FastMCP, httpx (same code base as jacob-bd). Read the jacob-bd note for everything inherited: research/notes/jacob-bd__gemini-notebook-mcp-cli.md
- License: MIT, (c) Jacob Ben David (LICENSE:1-3). Short snippets OK.
- Stars / last commit: 0 stars; HEAD `1eceaf0` 2026-10-13 (Merge PR #47). 1044 commits (includes upstream history).
- Versions reviewed: tags v1.0.0 (2026-04-06) .. v1.0.8 (2026-04-14) plus inherited v0.x tags. HEAD is "Unreleased: Upstream sync v0.5.25 -> v0.15.4" (CHANGELOG.md:10-14).
- Relation: fork of jacob-bd at v0.5.10 (CHANGELOG.md "fork divergence", the 1.0.0 entry); since Oct 2026 it merges upstream by release band (batches 1-6, commits fc4682b, 19e8bd7, f5805aa, 93d36d8, 34e0a5f, c5c4c25). So at HEAD it contains ALL of upstream v0.15.4 PLUS the fork additions below. "96 commits ahead" = mostly merge/sync/scaffold commits, not features.

## What is actually fork-only (everything else is upstream)

1. Official Discovery Engine REST client: `src/notebooklm_tools/core/enterprise_client.py` (543 lines) + `core/enterprise_adapter.py` that makes it look like `NotebookLMClient`.
2. Mode switch: `enterprise.mode = personal|enterprise` in config.toml or env `NOTEBOOKLM_MODE`; `get_client()` returns `EnterpriseAdapter(EnterpriseClient())` in enterprise mode (mcp/tools/_utils.py:300-322). Tool `configure_mode(mode, project_id, location)` (mcp/tools/setup.py:10-45). Config fields mode, project_id, location, endpoint_location (utils/config.py:540-575, 781-787, 849-853).
3. Standalone Podcast API tools `podcast_create`, `podcast_download` (mcp/tools/podcast.py).
4. Paywall/login-wall HEAD pre-check with approved domain list, SSRF private-IP block, URL scheme allowlist, per-URL bulk results (services/sources.py:36, 168-290, 410-422; README "Paywall" section). Param `skip_paywall_check`.
5. Remote MCP OAuth 2.1 layer (Authorization Code + PKCE S256, hard-coded client id/secret, in-memory tokens, TTLs 60 s / 1 h / 30 d), "ported from dizz/notebooklm-mcp-cli" (mcp/oauth.py:1-40, mcp/server.py:247-324).
6. Security hardening: CDP cookie scope narrowed to `Network.getCookies` with `urls=[notebooklm.google.com, accounts.google.com]` instead of `getAllCookies` (utils/cdp.py:1336-1363) [note: does not list `notebook.google.com`; .google.com-domain cookies still match, but verify for rebranded accounts, unverified]; sensitive-dir file blocklist; download path confinement; 0700 dirs (CHANGELOG 1.0.1).
7. Scaffold: `ai/` memory folder, `.claude/`, `.cursorrules`, githooks. Not product code.
8. Private-repo move: v1.5 (hosted enterprise-only weekly publisher MCP) was developed in a spike then moved private (commit 6934697 removed docs/plans/2026-10-12-v1.5-plan.md and scripts/spike_enterprise_v15.py). Both are still readable in history at commit 6617c3c and are my best source for how the author sees the API.

## Access method

Two independent paths selected by mode:
- Personal: unchanged jacob-bd batchexecute RPC (cookies). See upstream note.
- Enterprise REST: `discoveryengine.googleapis.com` v1alpha with a GCP OAuth2 bearer token. Google's own page confirms host, base path, version and the four notebook methods plus share and roles: https://docs.cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/api-notebooks (fetched 2026-10-14): host `https://{us|eu|global}-discoveryengine.googleapis.com`, base `/v1alpha/projects/{PROJECT_NUMBER}/locations/{LOCATION}`, Pre-GA, auth `Authorization: Bearer $(gcloud auth print-access-token)`, roles `PROJECT_ROLE_{OWNER,WRITER,READER,NOT_SHARED}`, recipients need the Cloud NotebookLM User role. That page did not list source/audio endpoints; those come from the fork code below.
- Upstream also has a third Enterprise path (web-UI batchexecute via `CloudNotebookLmUi`, cookies); the fork merged it (batch 4) but its own docs/plan say the official API is the supported route (docs/plans/2026-10-12-v1.5-plan.md in commit 6617c3c: "rules out upstream's enterprise batchexecute route").

## Enterprise REST API catalog (what the official-API mode maps)

Base: `https://{endpointLoc}-discoveryengine.googleapis.com/v1alpha/projects/{project}/locations/{location}` (enterprise_client.py:61-70). `endpointLoc` defaults to `location` but can differ for EU data residency (`NOTEBOOKLM_ENDPOINT_LOCATION`, enterprise_client.py:5-11, 58-61). Upload base: `https://{endpointLoc}-discoveryengine.googleapis.com/upload/v1alpha/projects/{p}/locations/{l}` (:66-70). Web link: `https://notebooklm.cloud.google.com/{location}/notebook/{id}?project={project}` (:200-201). IDs validated `^[a-zA-Z0-9\-]+$` before URL interpolation (:30-36).

| Operation | HTTP | Path (relative to base) | Body / response | file:line | Maps to feature |
|---|---|---|---|---|---|
| list notebooks | GET | `notebooks:listRecentlyViewed?pageSize=500` | `{notebooks:[{notebookId,title,metadata{isShared,isShareable},name}]}`. No list-all, no search (plan doc). No source count. | enterprise_client.py:207-222 | notebook_list |
| get notebook | GET | `notebooks/{id}` | `{notebookId,title,metadata,name,sources:[{sourceId:{id},title,settings:{status},name}]}` | :224-247 | notebook_get; source listing |
| create notebook | POST | `notebooks` | `{"title":..}` -> `{notebookId,title,name}` | :249-259 | notebook_create |
| delete notebook | POST | `notebooks:batchDelete` | `{"names":["projects/P/locations/L/notebooks/ID"]}`; returns `{}` even for missing names (plan doc) | :261-266 | notebook_delete |
| share | POST | `notebooks/{id}:share` | `{"accountAndRoles":[{"email":..,"role":"PROJECT_ROLE_READER"}]}`; no notify flag documented | :268-275 | notebook_share_invite/batch (email only, org scoped; no public link per README table) |
| get source | GET | `notebooks/{id}/sources/{sid}` | `{sourceId,title,settings.status,metadata,name}`; ready value `SOURCE_STATUS_COMPLETE` (spike script) | :281-293 | source polling |
| add URL | POST | `notebooks/{id}/sources:batchCreate` | `{"userContents":[{"webContent":{"url":..,"sourceName":..}}]}` -> `{sources:[...]}` | :295-302 | source_add url |
| add text | POST | same | `{"userContents":[{"textContent":{"sourceName":..,"content":..}}]}` | :304-310 | source_add text |
| add YouTube | POST | same | `{"userContents":[{"videoContent":{"youtubeUrl":..}}]}`; response `metadata.youtubeMetadata{channelName,videoId}` | :312-317, 339-343 | source_add url (youtube) |
| add Drive | POST | same | `{"userContents":[{"googleDriveContent":{"documentId":..,"mimeType":..,"sourceName":..}}]}` | :319-326 | source_add drive |
| upload file | POST | upload base `/notebooks/{id}/sources:uploadFile` | raw body; headers `Authorization`, `Content-Type: <mime>`, `X-Goog-Upload-File-Name`, `X-Goog-Upload-Protocol: raw`; MIME map for pdf/txt/md/epub/docx/pptx/xlsx/png/jpg/mp3/wav/mp4 (reads whole file into memory) | :328-370 | source_add file |
| delete source | POST | `notebooks/{id}/sources:batchDelete` | `{"names":[".../notebooks/ID/source/SID"]}` (resource name uses singular `source`) | :372-378 | source_delete |
| audio overview create | POST | `notebooks/{id}/audioOverviews` | Fork sends `{}` because as of March 2026 the documented fields `sourceIds`, `episodeFocus`, `languageCode` were rejected "Unknown name" (enterprise_client.py:422-437). Spike script (6617c3c) re-tests them with `{"episodeFocus":..,"languageCode":"en","sourceIds":[{"id":..}]}`; outcome not recorded in repo. One audio overview per notebook (plan). | :418-440 | studio_create audio |
| audio overview delete | DELETE | `notebooks/{id}/audioOverviews/default` | - | :442-445 | studio_delete |
| audio status | GET | `notebooks/{id}/audioOverviews/default` or the returned resource `name` | Spike probes this; fork docs claim status polling "does NOT work" (mcp/server.py:78-81). Unresolved. | spike script | studio_status |
| standalone podcast create | POST | `https://discoveryengine.googleapis.com/v1/projects/{p}/locations/global/podcasts` (stable v1, not alpha) | `{"podcastConfig":{"length":"SHORT"|"STANDARD","languageCode":..,"focus":..},"contexts":[{"text":..}],"title":..,"description":..}` -> long-running operation `name`; role `roles/discoveryengine.podcastApiUser`, no NotebookLM license needed | :447-505 (create at 458-495) | podcast_create |
| podcast download | GET | `https://discoveryengine.googleapis.com/v1/{operationName}:download?alt=media` | streams MP3 in 64 KiB chunks | :497-515 | podcast_download |
| chat / query | none | - | `NotImplementedError`: "Chat/query is not available in the Enterprise REST API" | :535-543 | not available |

Not available through the REST API per the fork's own `server_info` list (mcp/tools/server.py:339-350): chat/query, video, reports, flashcards, quizzes, infographics, slides, mind maps, notes, research, notebook rename, notebook summary (adapter stubs enterprise_adapter.py:76-95). Supported list (server.py:323-336): notebook list/create/get/delete, source add (URL, text, YouTube, Drive, file) / delete, audio overview generate/delete, standalone podcast, share.

## Auth lifecycle (enterprise)

- Token order: constructor arg -> env `NBLM_ACCESS_TOKEN` -> `gcloud auth print-access-token` -> `gcloud auth application-default print-access-token` (10 s timeout each) (enterprise_client.py:93-120). A cached token is kept for the process lifetime; HTTP 401 clears it and retries once, not counted as a retry (:134-150).
- No refresh-token handling of its own: relies on gcloud to mint a fresh token each time the cache is cleared. Tokens last ~1 h, so a long-lived server only recovers via the 401 path [inferred].
- Config: `NOTEBOOKLM_PROJECT_ID` (project NUMBER, required), `NOTEBOOKLM_LOCATION` (global|us|eu), `NOTEBOOKLM_ENDPOINT_LOCATION`, `NOTEBOOKLM_MODE` (env wins over config.toml) (enterprise_client.py:1-13; mcp/tools/podcast.py:15-24).
- IAM (docs/AUTHENTICATION.md:778-783): `roles/discoveryengine.admin` for notebooks, sources, audio; `roles/discoveryengine.podcastApiUser` for podcasts; recipients of shares need Cloud NotebookLM User + license (plan doc).
- Enterprise and personal auth are fully separate; server instructions repeat this (mcp/server.py:68-72). Personal side is identical to upstream (cookies, profiles, protected store).
- Multi-account: personal profiles as upstream; enterprise is a single gcloud identity per process.

## RPC catalog

No batchexecute RPC ids are new in this fork. Personal mode uses exactly the upstream catalog (see jacob-bd note; this fork's base.py is the merged v0.15.4 file). For Enterprise-via-web-UI the same ids as upstream apply: `rG2vCb` (list), streamed `GenerateFreeFormStreamed` under `CloudNotebookLmUi`. New ids found: none. The REST catalog above replaces this section for enterprise mode.

## Streaming/chat

Personal: identical to upstream. Enterprise REST: NO chat endpoint; adapter raises NotImplementedError (enterprise_client.py:535-543, enterprise_adapter.py query stub). An official chat API is not documented on the fetched Google page [unverified for other pages].

## MCP tools

56 tools at HEAD (mcp/tool_groups.py:12, CHANGELOG.md "56 MCP tools in total"): upstream's 53 plus fork-only `configure_mode`, `podcast_create`, `podcast_download` (tool_groups.py:104, 131-134). Parameters:

| Tool | Params | Behavior |
|---|---|---|
| configure_mode | mode(personal\|enterprise), project_id, location=global | Pre-checks auth for the target mode (gcloud token or cached cookies), writes config.toml (setup.py:10-80) |
| podcast_create | text(str\|list), title, description, focus, length(SHORT\|STANDARD), language=en | Enterprise only; returns `operation_name` (podcast.py:28-80) |
| podcast_download | operation_name, output_path (default ~/Downloads/podcast.mp3) | Streams MP3 (podcast.py:83-118) |
| source_add (extended) | + skip_paywall_check, repl | HEAD precheck + SSRF guard (services/sources.py:410-422) |
| server_info (extended) | - | per-mode auth_status, `supported_operations` / `unsupported_in_enterprise` lists (tools/server.py:285-350) |
| all others | as upstream | In enterprise mode unsupported ones raise NotImplementedError via adapter |

## Reliability

Enterprise `_request`: retry 429/500/502/503/504 up to 3x, backoff `min(0.5*2^n, 5)` s with +-25% jitter, honors `Retry-After` on 429; 120 s client timeout; error body truncated to 500 chars (enterprise_client.py:122-190). No client-side rate limiter; Pre-GA API so quotas are not documented in this repo.

## Performance notes

Single `httpx.Client` reused (connection reuse). File upload reads whole file into RAM (`f.read()`, :355-358). No batching of source adds in the client (single item per call) though `batchCreate`/`batchDelete` accept arrays (plan doc says batching is "new" work).

## Tests and quality

Own CI "Run tests" gate on Linux, CodeQL, gitleaks config, SHA-pinned workflows (CHANGELOG batch 6 notes). Upstream's Windows leg removed.

## Versions

v1.0.0 2026-04-06 fork release; v1.0.1 2026-04-07 security patch (path traversal, SSRF, CDP cookie scope, fastmcp CVEs); v1.0.2 2026-04-08 endpoint_location, NBLM_ACCESS_TOKEN, retry/backoff, YouTube routing; v1.0.3 2026-04-08 upstream sync v0.5.17; v1.0.4 2026-04-11 OAuth 2.1 remote MCP + sync; v1.0.5 2026-04-12; v1.0.6 2026-04-14 enterprise fixes; v1.0.8 2026-04-14 EPUB, is_shareable, YouTube metadata. No tag after 2026-04-14; Oct 2026 work is "Unreleased" (upstream sync to 0.15.4, 56 tools).

## Adopt

1. Optional "official API" transport for Enterprise: GET/POST map above, resource-name conventions (`.../notebooks/{id}/source/{sid}` singular in delete), `listRecentlyViewed` only, `batchDelete` returns `{}` for missing names (enterprise_client.py:261-266, 372-378; plan doc). Cite Google page https://docs.cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/api-notebooks.
2. Token acquisition ladder (env token -> gcloud -> ADC) and one-shot 401 refresh (enterprise_client.py:93-150). In Go use `golang.org/x/oauth2/google` ADC instead of shelling out [design suggestion].
3. Retry with jitter and `Retry-After` (enterprise_client.py:122-190).
4. Separate `endpoint_location` from resource `location` (:58-61).
5. SSRF guard + URL scheme allowlist before adding web sources; sensitive-dir file blocklist (services/sources.py:197-222, 410-422; CHANGELOG 1.0.1).
6. CDP cookie scoping to only needed URLs (utils/cdp.py:1336-1363), extended to include `notebook.google.com` and `notebook.cloud.google.com`.
7. `server_info` that reports unsupported-in-mode feature lists so the LLM does not try them (tools/server.py:323-350).
8. Standalone podcast endpoint is a distinct, licence-free official feature (enterprise_client.py:447-505); worth exposing when ADC is present.

## Avoid

1. Mode-global switch (`get_client()` returns a different client type for all tools, mcp/tools/_utils.py:300-322) with a `NotImplementedError` adapter: tools appear in the list but fail at call time. Expose capability per backend instead.
2. Shelling out to `gcloud` per token (enterprise_client.py:100-115).
3. Whole-file upload in RAM (:355-358).
4. Hard-coding `{}` for audio overview because of a March 2026 observation; re-test the documented fields (spike script) and keep it feature-flagged.
5. Hard-coded OAuth client id/secret with auto-approve, in-memory tokens (mcp/oauth.py:1-12): only acceptable for single-user.
6. Fork-and-merge drift: this fork needed six sync batches; avoid by owning the transport layer.
7. Assuming REST has parity: no chat, studio types, notes or research, so it cannot replace RPC (mcp/tools/server.py:339-350).

## Evidence

Read: enterprise_client.py (all), enterprise_adapter.py (1-140), mcp/tools/{podcast,setup,server,_utils}.py (relevant), mcp/oauth.py (head), mcp/server.py (59-110, 247-324), utils/cdp.py 1336-1375, services/sources.py (36, 168-290, 410-422 greps), CHANGELOG.md 1-330, README.md 255-335, docs/AUTHENTICATION.md 680-790 (grep), git show 6617c3c (plan + spike script), 6934697. WebFetch of Google docs page above.
