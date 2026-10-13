# K-dash/nblm-rs

- URL: https://github.com/K-dash/nblm-rs
- Language / runtime: Rust 2021 workspace of three crates: `nblm-core` (library), `nblm-cli` (binary `nblm`), `nblm-python` (PyO3 bindings, published as PyPI `nblm`). Core deps: `reqwest 0.12` (rustls, json), `tokio 1.48`, `serde`, `thiserror 2`, `backon 1.6` (retry), `oauth2 5`, `directories 6`, `httpdate`, `parking_lot` (`crates/nblm-core/Cargo.toml:15-40`).
- License: MIT ("Copyright (c) 2025 K..." `LICENSE:3`, workspace crates `license = "MIT"`).
- Stars / last commit: 86 stars (per upstreams.md); HEAD `49b1308` 2026-06-07 (dependabot/renovate chore). Last functional work was 2025-11 (v0.2.3 on 2025-11-20).
- Versions reviewed: tags v0.1.0 (2025-10-19), v0.1.2/v0.1.3 (2025-10-25), v0.1.4/v0.1.5 (2025-10-26), v0.2.0/v0.2.1 (2025-11-01), v0.2.2 (2025-11-11), v0.2.3 (2025-11-20).
- Clone: `research/upstreams/K-dash__nblm-rs/`.

## What this is (and is not)

A client for **Google's official NotebookLM Enterprise API**, which lives on the Discovery Engine (Agentspace / Gemini Enterprise) REST surface in Google Cloud. It is NOT a consumer NotebookLM client: no cookies, no `batchexecute`, no consumer notebooks. It needs a Google Cloud project with the NotebookLM Enterprise license and API enabled (`README.md` "Prerequisite"; `docs/getting-started/authentication.md`). The API is v1alpha and many documented fields are rejected by the server (see Reliability).

## Access method

Official REST/JSON, bearer token auth.

- Base URL: `https://{us-|eu-|global-}discoveryengine.googleapis.com/v1alpha` (`crates/nblm-core/src/env/mod.rs:155-160`; endpoint location normalised from `us|eu|global` by `normalize_endpoint_location`, `:173-185`). Parent path `projects/{project_number}/locations/{location}` (`:159`). So the "endpoint location" (regional host prefix) and "location" (resource location) are separate settings (`NBLM_ENDPOINT_LOCATION`, `NBLM_LOCATION`, `NBLM_PROJECT_NUMBER`; project **number**, not id).
- Upload base: `https://.../upload/v1alpha/projects/.../sources:uploadFile` (path rewritten from `/v1alpha` to `/upload/v1alpha`, `client/url/enterprise.rs:33-42`).
- Headers: `Authorization: Bearer <token>`; optional `x-goog-user-project: <project>` (quota/billing project) (`client/http.rs:199-201,233-235`).
- Profiles: `enterprise` (implemented), `personal` and `workspace` (declared but return "not available yet"; behind `NBLM_PROFILE_EXPERIMENT`) (`env/mod.rs:3-47,143-149`). Suggests the author expects consumer/Workspace APIs; none exists today.

## RPC catalog (REST resource catalog)

| HTTP | Path (relative to base) | Purpose | Request | Response | file:line |
|---|---|---|---|---|---|
| POST | `{parent}/notebooks` | Create notebook | `{"title": "..."}` | `Notebook{name, title, notebookId, emoji?, sources[], metadata{createTime,isShareable,isShared,lastViewed}}` | `backends/enterprise/notebooks.rs:45-57`, `models/notebook.rs` |
| POST | `{parent}/notebooks:batchDelete` | Delete notebooks | `{"names": ["projects/../notebooks/<id>"]}` | `{}` | `notebooks.rs:24-37` |
| GET | `{parent}/notebooks:listRecentlyViewed?pageSize=1..500` | List recent notebooks | none | `{notebooks:[...]}`; no `nextPageToken` ever returned | `notebooks.rs:81-101` |
| POST | `{parent}/notebooks/{id}/sources:batchCreate` | Add sources | `{"userContents":[ {"webContent":{"url","sourceName"}} | {"textContent":{"content","sourceName"}} | {"googleDriveContent":{"documentId","mimeType","sourceName"}} | {"videoContent":{"youtubeUrl"}} ]}` | created sources with `sourceId.id`, `settings.status` | `sources.rs:29-45`, `models/source.rs:65-120`, `requests/source.rs:6-12` |
| POST | `{parent}/notebooks/{id}/sources:batchDelete` | Delete sources | `{"names":[...]}` | `{}` | `sources.rs:45-60` |
| POST | `/upload/v1alpha/{parent}/notebooks/{id}/sources:uploadFile?uploadType=media` | Upload file (raw media upload) | body = bytes; headers `x-goog-upload-protocol: raw`, `x-goog-upload-file-name: <name>`, `Content-Type: <mime>` | `{sourceId...}` | `sources.rs:116-162` |
| GET | `{parent}/notebooks/{id}/sources/{sourceId}` | Get source | none | `NotebookSource{name, title, sourceId{id}, metadata{sourceAddedTimestamp, wordCount, youtubeMetadata{channelName,videoId}}, settings{status}}` | `sources.rs:164-186` |
| POST | `{parent}/notebooks/{id}/audioOverviews` | Create audio overview | **only `{}` accepted** | audio overview resource | `backends/enterprise/audio.rs`, `requests/audio.rs:1-25` |
| DELETE | `{parent}/notebooks/{id}/audioOverviews/default` | Delete audio overview | none | | `audio.rs` |

Dropped: `share` (Share API removed from official docs on 2025-11-07; code deleted; `docs/cli/share.md`).

### Enterprise API vs consumer batchexecute: what exists where

| Capability | Enterprise REST (nblm-rs) | Consumer RPC (tmc/nlm et al.) |
|---|---|---|
| Create notebook | yes | yes (CCqFvf) |
| List notebooks | recently viewed only, all in one page | yes (wXbhsf) |
| Get notebook | **no** (not implemented; `NotebooksBackend` has only create, batch_delete, delete, list_recently_viewed, `client/api/backends/mod.rs:43-57`) | yes (rLM1Ne), also lists sources |
| Rename / emoji / description | no | yes (s0tc2d) |
| Delete notebook | yes (one at a time; batch of 2+ returns HTTP 400) | yes (WWINqb) |
| Add source url / text / YouTube / Drive / file | yes (web, text, video, Drive, upload) | yes (izAoDd, o4cbdc) |
| List sources | **not directly** (only via notebook body `sources[]`); get one source by id | via rLM1Ne |
| Delete / rename / refresh source, source content or guide | delete only | all |
| Chat / ask question | **no** | yes (laWbsf stream) |
| Notes, mind maps, labels, studio artifacts (report, quiz, slides, video, infographic, data table) | **no** | yes |
| Audio overview | create (no options), delete | create with format/length/language, list, download, share |
| Deep research / discover sources | no | yes |
| Sharing | removed | yes |
| Auth | OAuth2/gcloud/service-account token; official | cookies |

Bottom line: the Enterprise API covers the CRUD skeleton (notebooks, sources, audio) and **none of the AI surface that makes NotebookLM useful for an MCP server (chat, artifacts, research)**. It is a plausible secondary backend for "create notebook + add sources + audio" for users who have Enterprise licenses, not a replacement for the RPC transport.

## Streaming/chat

None. The Enterprise API client has no chat/query endpoint (README features list: notebooks, sources, audio, sharing(removed)).

## Auth lifecycle

Token providers (`crates/nblm-core/src/auth/mod.rs:15-210`, `docs/getting-started/authentication.md`):

1. **gcloud** (default, recommended): spawns `gcloud auth print-access-token` for every token fetch (custom binary path supported) (`auth/mod.rs:123-166`). Token ~1 h; refresh = re-spawn. Drive scope: `ensure_drive_scope` checks the token via tokeninfo for `drive.file` and tells the user to run `gcloud auth login --enable-gdrive-access` (`auth/mod.rs:56-130`).
2. **Env token**: `NBLM_ACCESS_TOKEN` (no refresh; "expire after roughly one hour"; docs recommend a dedicated service account or Workload Identity Federation for unattended use).
3. **User OAuth2 + PKCE** (experimental, hidden `--auth user-oauth`, flag `NBLM_PROFILE_EXPERIMENT`): authorization-code flow with loopback redirect `http://127.0.0.1:4317`, scopes `cloud-platform` and `drive.file`, client id from `NBLM_OAUTH_CLIENT_ID`, endpoints `accounts.google.com/o/oauth2/v2/auth`, `oauth2.googleapis.com/token`, revoke `oauth2.googleapis.com/revoke`; tokens cached in `<ProjectDirs("com","nblm","nblm-rs")>/credentials.json` (config dir chmod 700, file 0600, atomic write via temp file) with `needs_refresh` and refresh tokens (`auth/oauth/mod.rs:40-60,242-343`, `auth/oauth/config.rs:15-41`).
- **No service-account JSON key flow and no Application Default Credentials code.** A service account works only if the user mints an access token themselves (`gcloud auth print-access-token --impersonate-service-account=...` or similar) and passes it via env. We could add ADC cheaply with `golang.org/x/oauth2/google`.
- Multi-account: whatever gcloud's active account is; no explicit switching.
- Python SDK: `GcloudTokenProvider`, env provider, OAuth read-only (`docs/getting-started/authentication.md` table).

## Reliability

- Retry: `backon` exponential, min 500 ms, max 5 s, 3 retries, jitter on; honors `Retry-After` (seconds or HTTP-date, capped at max delay); retries 429, 408 and all 5xx, and connect/timeout errors (`client/retry.rs:11-171`). Applies to every request including POST (create is not idempotent: duplicate-notebook risk). 
- Error type: `thiserror` enum: `TokenProvider, Endpoint, Request, Http{status,message,body}, Json, Url, Validation` with Google-style error body message extraction `error.message` (`error.rs:1-50`).
- Verified API limitations (dated 2025-10-31, `docs/api/limitations.md`): `batchDelete` for notebooks fails with HTTP 400 when more than one name is sent (workaround: loop one at a time); `listRecentlyViewed` accepts `pageSize`/`pageToken` but never returns `nextPageToken`; audio overview request rejects `languageCode`, `sourceIds`, `episodeFocus` ("Unknown name") so only `{}` works. These may have changed since [unverified].
- Debug logging of HTTP bodies truncated to 2048 chars behind `NBLM_DEBUG_HTTP` (`client/http.rs:~110-130`).
- Client timeout default 30 s (`client/mod.rs:20`).

## Performance notes

Async reqwest with connection pooling; nothing measured. Retry delays small. 

## MCP tools

None. (CLI subcommands: `notebooks {create,recent,delete}`, `sources {add,upload,get,delete}`, `audio {create,delete}`, `auth {login,status}`, `doctor`.) `nblm doctor` runs checks for required commands, env vars, Drive-scoped token and API connectivity and returns exit codes (`nblm-core/src/doctor/*`, `docs/cli/doctor.md`) — a pattern worth copying for our own `doctor`.

## Tests and quality

- Wiremock-based integration tests for every CLI command (`crates/nblm-cli/tests/*.rs`: `errors_retry.rs`, `notebooks_create.rs`, `sources_upload.rs`, ...), unit tests for URL building, retry, serde shapes; python tests under `python/tests`.
- CI: build+test on ubuntu and macos (no Windows), codecov, pre-commit, cargo-make, Renovate (`.github/workflows/*`, `codecov.yml`, `renovate.json`).
- Release: Homebrew tap script, prebuilt binaries, PyPI wheels via maturin.

## Versions

v0.1.0 2025-10-19 initial (notebooks, sources); v0.1.2-0.1.5 (2025-10-25/26) audio, docs; v0.2.0 (2025-11-01) Python OAuth/profile work; v0.2.2 (2025-11-11); v0.2.3 (2025-11-20). After that only dependency bumps through 2026-06-07.

## Adopt

1. **Optional "enterprise" backend** behind our transport interface: REST client with the exact resource paths above (`notebooks`, `sources:batchCreate|batchDelete|uploadFile`, `audioOverviews`), selected when `NBLM_PROJECT_NUMBER` + token are configured. Gives a no-cookie, supported path for users with licenses. Keep its capability set small and advertised (`capabilities()` lists unsupported ops) so tools return a clear "not supported by backend" error.
2. **Retry design** with Retry-After parsing, jitter, status set (429, 408, 5xx) (`client/retry.rs`), applied only to idempotent requests in our version.
3. **`doctor` command** with named checks and exit codes (`doctor/checks.rs`).
4. **Documented API limitations file with discovery dates** (`docs/api/limitations.md`). We should keep an equivalent `docs/rpc-quirks.md` for the consumer RPCs.
5. **Credential file hygiene**: chmod 700 dir / 600 file, temp-file + rename write (`auth/oauth/mod.rs:260-343`). (On Windows, ACL equivalent.)
6. Token provider trait (`TokenProvider::access_token()`), pluggable gcloud/env/static providers; mirror in Go with `oauth2.TokenSource`.
7. Input validation errors before network calls (empty notebook id, file name, content type) (`sources.rs:116-135`).

## Avoid

1. Treating alpha docs as truth; keep tests that pin real behavior (audio `{}` only).
2. Spawning `gcloud` for every request (latency ~hundreds of ms); cache the token until expiry, use `oauth2/google` ADC.
3. Auto-retrying POST creates without idempotency keys.
4. Shipping a sharing feature that depends on an API removed from docs.
5. No Windows CI for a tool that spawns external binaries.

## Evidence

Read: `Cargo.toml`, `crates/nblm-core/Cargo.toml`, `src/{env/mod,error,auth/mod,client/{http,retry,mod}}.rs`, `src/client/url/enterprise.rs`, `src/client/api/backends/enterprise/{notebooks,sources,audio}.rs` and `models/*`, `auth/oauth/{mod,config}.rs` (head), CLI `args.rs`, `docs/api/limitations.md`, `docs/getting-started/authentication.md`, `docs/cli/share.md`, `README.md`. `git tag`, `git log`. External URLs referenced inside the repo and not fetched by me: https://cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/share-notebooks (share docs removed 2025-11-07 per the repo) and, from KAFKA2306 notes, https://docs.cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/api-audio-overview. Nothing executed.
