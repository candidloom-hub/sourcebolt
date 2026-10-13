# wangjing0/notebooklm-mcp

NO LICENSE FILE (no LICENSE*, none declared in pyproject.toml; verified by `ls LICENSE*` and grep). All rights reserved by default. This note records BEHAVIOR and facts only (names of tools, RPC ids and endpoints are interface facts, not copied code). No code is quoted. Do not port code from this repo.

- URL: https://github.com/wangjing0/notebooklm-mcp
- Language / runtime: Python 3.10+, FastMCP, Playwright (Chrome) plus httpx; uv-managed (README "Requirements"; main.py:8).
- License: none.
- Stars / last commit: 3 stars; HEAD `0eee4c8` 2026-03-03 (merge PR #1). About 30 commits since 2026-02-17. No tags.
- Versions reviewed: no tags/releases; pyproject version 0.1.0 per research/upstreams.md.
- Lineage: README credits the TypeScript project PleasePrompto/notebooklm-mcp as the original and calls this "a Python implementation" (README line 5). The Playwright half (library, sessions, stealth, auth state) is a port of that design; the HTTP-RPC half (commit f53cecc 2026-03-03, "Add HTTP client for NotebookLM API") is new and uses the same batchexecute ids as jacob-bd.

## Access method
Hybrid, per tool (README "Architecture" section):
- Chat (`ask_question`): browser automation. Playwright drives a persistent Chrome profile, types into the chat textarea with simulated human typing (typos, random delays, mouse movement, realistic clicks: src/utils/stealth_utils.py), then waits for the answer by snapshotting existing responses and polling every 1 s with streaming detection (src/session/browser_session.py:194-223). Selector-based (e.g. `textarea.query-box-input`, plus a German-locale aria-label fallback, browser_session.py:105-109) so UI/locale changes break it. Error detection via `.error-message`, `[role=alert]`, `.rate-limit-message` selectors (browser_session.py:271).
- Source and research tools: direct HTTP batchexecute using cookies from the Playwright `storage_state` file (src/http_client/client.py:4, 255-330).
- README states the reason: the chat interface "has no public API" and that the Enterprise API covers only notebooks/sources, not querying (README "Disclaimer" section; links https://docs.cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/api-notebooks).

## RPC catalog (HTTP half only; src/http_client/client.py)
Endpoint `https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute` (client.py:20). Query params used: `rpcids`, `source-path=/notebook/{id}`, `hl=en`, `rt=c`, `f.sid` (no `bl`, no `_reqid`) (client.py:304-330). Body `f.req` plus `at=<SNlM0e>` (client.py:309-312). Headers: form content type and a `Cookie` header only; no Origin/Referer/X-Same-Domain (client.py:317-324) [works for the author; stricter than jacob-bd, risk]. Timeout 60 s. 401/403 raises an auth error and clears the authenticated flag; no retry and no refresh (client.py:326-329). CSRF `SNlM0e` and session `FdrFJe` are regex-scraped from the homepage on first use and cached for the process lifetime (client.py:269-299). Only the FIRST wrb.fr result in the response is taken, regardless of id (client.py:152-165).

| RPC id | Purpose | Request shape (behavioral) | file:line |
|---|---|---|---|
| rLM1Ne | get notebook, used to list sources | `[notebook_id, null, [2], null, 0]` | client.py:334 |
| izAoDd | add URL / YouTube / text source | same layout as jacob-bd (URL list wrapper; YouTube id detection helper at client.py:168) | client.py:340-380 |
| o4cbdc | register file for resumable upload; then the same `x-goog-upload-*` start and finalize steps against the upload endpoint | client.py:381-446 |
| tGMBJ | delete source | client.py:447-456 |
| Ljjv0c | start fast research | client.py:458-480 |
| QA9ei | start deep research | client.py:469 |
| e3bVqc | poll research | client.py:481-520 |
| LBwxtb | import research sources | url+title entries with trailing 2 | client.py:522-550 |

Source status codes: poll `list_sources` until status is 2 (READY) or 3 (ERROR) (client.py:552-569). Source type map includes 10 "media" and 13 "image" (client.py:30-45). No ids that jacob-bd lacks. Nothing for studio, notes, sharing, labels, chat history.

## Streaming/chat
Browser-only; no stream parsing. The answer is whatever the page DOM shows after response stabilizes; no citations structure, no conversation id (a session = an open browser tab/page with its chat thread; follow-ups reuse the same page: README "Stateful sessions"). Every answer ends with a canned reminder text that nudges the agent to keep asking follow-ups (docs/tools.md). Free-tier quota note: about 50 queries/day per account (README FAQ; docs/tools.md `re_auth` says to switch account at the rate limit).

## Auth lifecycle
- `setup_auth`: opens a persistent Chrome profile (Playwright, visible or headless; `HEADLESS` default true) for manual Google login; state is saved as Playwright `storage_state` (src/auth/auth_manager.py:34-58, 355-400). Validity checks: validate state against the context, check cookie expiry, `is_state_expired` (auth_manager.py:83-140).
- Optional auto-login with `LOGIN_EMAIL` and `LOGIN_PASSWORD` environment variables, typed into the Google form including account chooser handling (`AUTO_LOGIN_ENABLED`, auth_manager.py:194-353). This stores a Google password in env/.env: not acceptable for our server.
- HTTP half reads cookies from the same storage_state; requires `SID` present; accepts cookie domains `.google.com`, `notebooklm.google.com`, `.googleusercontent.com`, and any `.google.*` regional domain (client.py:24-28, 97-114).
- Re-auth: `re_auth` tool clears auth data and reopens login (account switch); `cleanup_data` wipes everything; `hard_reset_state` (auth_manager.py:402-470).
- Profile strategies: `NOTEBOOK_PROFILE_STRATEGY` (auto/isolated/...), optional cloning of the profile into per-instance dirs with a max count (src/config.py:56, 120-127; session/shared_context_manager.py).
- Multi-account: multi-tenant mode gives each `X-User-ID` header (1-128 printable ASCII, regex-validated) its own auth, library and browser sessions (src/mcp_server/multi_tenant_server.py:120-161; tenant_manager.py). Tenants are LRU-evicted at `MAX_TENANTS_IN_MEMORY` (default 100) and idle-evicted after `TENANT_IDLE_TIMEOUT_SECONDS` (default 3600) (config.py:190-191, 206-207). No authentication on the `X-User-ID` itself: any caller can claim any tenant [unverified whether a reverse proxy is assumed; README suggests curl with just the header].

## MCP tools (main.py registrations; 28 tools)
Browser/session group: `ask_question(question, session_id?, notebook_id?, notebook_url?, show_browser?)`, `list_sessions`, `close_session(session_id)`, `reset_session(session_id)`, `get_health`, `setup_auth(show_browser?, ...)`, `re_auth(...)`, `cleanup_data(confirm?, ...)` (main.py:27-206; docs/tools.md).
Local notebook library (a JSON catalog of saved notebook URLs with tags/description/topics; not a NotebookLM RPC): `add_notebook` (conversational confirm), `list_notebooks`, `get_notebook(id)`, `select_notebook(id)`, `update_notebook`, `remove_notebook(id)` (library entry only), `search_notebooks(query)`, `get_library_stats` (main.py:107-195; src/library/notebook_library.py). Plus MCP resources `notebooklm://library` and `notebooklm://library/{id}` (docs/tools.md).
HTTP group: `list_sources(notebook_id)`, `add_source_url(notebook_id, url)`, `add_source_text(notebook_id, title, content)`, `add_source_file(notebook_id, file_path)`, `delete_source(notebook_id, source_id)`, `start_research(notebook_id, query, source=web, mode=fast)`, `get_research_status(notebook_id)`, `import_research_sources(notebook_id, task_id, sources[])` (main.py:209-322). `notebook_id` is resolved through the local library (`_resolve_notebook_url`, handlers.py).
Tool profiles: `NOTEBOOKLM_PROFILE=minimal|standard|full` and `NOTEBOOKLM_DISABLED_TOOLS`; minimal = ask_question, get_health, list_notebooks, select_notebook, get_notebook; standard adds setup_auth, list_sessions, add_notebook, update_notebook, search_notebooks (src/utils/settings_manager.py:11-60).
Transports: stdio (default) and streamable HTTP (`--transport http`), optional `--multi-tenant` (README). Docker, docker-compose, Makefile, GitLab CI provided.

## Reliability
Browser layer is acknowledged as the fragile part (README roadmap: no self-healing selectors, no retries). HTTP layer: no retry, no rate-limit handling, no token refresh. Session limits `MAX_SESSIONS` 10, `SESSION_TIMEOUT` 900 s (config.py:33-34). Stealth: README warns Google may detect automation and block the account and advises a dedicated account.

## Performance notes
Per-call new `httpx.AsyncClient` for every RPC (client.py:310). Chat latency = browser typing simulation (random per-key delays) + DOM polling at 1 s, so deliberately slow and heavy (a full Chrome instance per tenant/session, `MAX_SESSIONS` 10).

## Tests and quality
tests/ has unit tests with mocks for config, errors, http_client, library, handlers, settings, tenant manager (tests/*.py); no live tests. `.coverage` file committed.

## Versions
No tags. Timeline: 2026-02-17 start (docs, config); 2026-02-19 multi-tenant HTTP server, CI/CD; 2026-02-20 Docker, deployment docs, tutorial; 2026-03-03 HTTP RPC client for source/research tools.

## Adopt (ideas only, reimplement from behavior)
1. Tool profiles `minimal|standard|full` plus disabled-tools list to control context size (settings_manager.py:11-60); jacob-bd has groups, this has presets: offer both.
2. A local "notebook library" concept (saved notebook aliases with tags/topics and `select_notebook` active default) so an agent can pick a notebook by topic. jacob-bd has aliases and tags; this one adds an "active notebook" default and library resources (docs/tools.md).
3. Per-tenant isolation via `X-User-ID` with LRU and idle eviction for a shared HTTP deployment (tenant_manager.py:22-93), but add real authentication in front.
4. Reminder suffix on answers to encourage follow-up queries is optional UX.
5. Accepting all regional `.google.*` cookie domains when importing cookies (client.py:97-101).

## Avoid
1. Any code reuse: no license.
2. Chat via DOM automation with human-typing simulation (browser_session.py; stealth_utils.py): slow, brittle (locale-specific selectors), and it is exactly the account-risk the README warns about. Use the streamed RPC as primary and keep browser only as fallback.
3. Auto-login with password in environment variables (auth_manager.py:194-353; README config).
4. Unauthenticated `X-User-ID` tenancy and README example binding `0.0.0.0` (README multi-tenant command).
5. HTTP calls without Origin/Referer/X-Same-Domain/UA and taking only the first RPC result (client.py:152-165, 317-324).
6. No retry or token refresh on HTTP half (client.py:326-329).
7. New HTTP client per RPC (client.py:310).

## Evidence
Read: README.md, docs/tools.md, main.py (tool registrations), src/http_client/client.py (RPC ids, auth, flow), src/tools/handlers.py (research/source handlers), src/auth/auth_manager.py (function list), src/config.py (grep), src/session/browser_session.py (grep), src/tenant_manager.py, src/utils/settings_manager.py, src/mcp_server/multi_tenant_server.py (grep), git log and `git show --stat 0eee4c8`. No code reproduced.
