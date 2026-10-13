# julianoczkowski/notebooklm-mcp-2026

- URL: https://github.com/julianoczkowski/notebooklm-mcp-2026
- Language / runtime: Python, FastMCP, httpx, websocket-client, platformdirs (pyproject.toml); package dir `src/notebooklm_mcp_2026/` (2972 lines total).
- License: MIT, (c) 2025 Julian (LICENSE:1-3). Short snippets OK.
- Stars / last commit: 18 stars (research/upstreams.md); HEAD `bb78b41` 2026-07-06 "Add Author section to README"; last code commit `fe034f2` v0.2.1 2026-05-01. 42 commits.
- Versions reviewed: tags v0.1.0, v0.1.1, v0.1.2 (2026-02-14), v0.2.0 (2026-02-15), v0.2.1 (2026-05-01).
- Relation to others: logic is a compact re-implementation of the jacob-bd batchexecute client (same RPC ids, same "type 1 answer / type 2 thinking, longest wins" parser, same history format, same required cookies). Attribution to jacob-bd not seen in the source files [unverified in README]. Parent of Faxtom/notebooklm-mcp (see that note).

## Access method
Internal batchexecute RPC over httpx with Google cookies; Chrome via CDP only for login. No browser at call time.
- `POST https://notebooklm.google.com/_/LabsTailwindUi/data/batchexecute` (src/notebooklm_mcp_2026/config.py:25-27); streaming query at `.../google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed` (config.py:28-33).
- Hard-coded `bl=boq_labs-tailwind-frontend_20260108.06_p0`, override via `NOTEBOOKLM_BL` (config.py:35-39). It does NOT scrape the live `bl` from the page (jacob-bd does), so a rotated build label is a manual fix. Host is hard-coded to notebooklm.google.com (no `notebook.google.com`), so accounts redirected by the Gemini Notebook rebrand would fail [inferred; unverified on a live account].

## RPC catalog
Only 5 ids ("only the subset needed for our 9 tools", config.py:41-48). Envelope identical to jacob-bd (protocol.py:36-86): body `f.req=urlencode([[[rpc, json(params), null, "generic"]]])&at=<csrf>&`, URL params `rpcids, source-path, bl, hl=en, rt=c, f.sid`. Response parser strips `)]}'`, reads byte-count line then JSON line, finds `["wrb.fr", id, "<json>", ...]` (protocol.py:92-168). Auth error: `item[6]=="generic"` and `16 in item[5]` -> `AuthExpiredError` (protocol.py:150-158). No other RPC error codes decoded (no 8/3/9 handling).
Headers: Content-Type form-urlencoded, Origin, Referer, `X-Same-Domain: 1`, UA Chrome/143 Linux (config.py:95-113). `X-Goog-Csrf-Token` header is not set; CSRF goes only in `at=`.

| RPC id | Purpose | Request payload | Response | file:line |
|---|---|---|---|---|
| wXbhsf | list notebooks | `[null,1,null,[2]]` | `[[ [title, sources, id, emoji, null, [meta..]] ]]` (timestamps meta[5], meta[8]) | client.py:311-372 |
| rLM1Ne | get notebook | `[nbid,null,[2],null,0]`, path `/notebook/{id}` | `[[title, sources[[ [sid], title, meta ]], id]]` | client.py:375-428 |
| hizoJc | source full text | `[[sid],[2],[2]]` | content blocks at `r[3][0]`, recursive text collection | client.py:430-475 |
| izAoDd | add URL / YouTube / text | URL `[[ [null,null,[url],null x6,1] ],nbid,[2],[1,null x9,[1]]]`; YouTube url at index 7; text source_data `[null,[title,text],null,2,null x6,1]` | `[[ [[sid], title] ]]` | client.py:477-540 |
| tr032e | source guide | declared only, not called by client.py | - | config.py:48 |
| (stream) GenerateFreeFormStreamed | chat | see below | see below | client.py:541-600; protocol.py:197-232 |

CSRF/session: `GET https://notebooklm.google.com/` with browser-like headers; final URL containing `accounts.google.com` means expired; regex `"SNlM0e":"..."` and `"FdrFJe":"..."` (client.py:147-186; auth.py:364-382). Cookies are set on `.google.com` only (client.py:149-151).

## Streaming/chat
- `POST {BASE}{QUERY_ENDPOINT}?bl&hl=en&_reqid=<n>&rt=c[&f.sid]` body `f.req=urlencode([null, json(params)])&at=` (protocol.py:197-232). `_reqid` += 100000 per query (client.py:581).
- params `[[[[sid]]..], query, history|null, [2,null,[1]], conversation_id]`; history `[[answer,null,2],[query,null,1],...]`; new conversation = fresh uuid4 (client.py:563-588, 606-614). Does not reuse the server-side conversation id (no hPTbtc), so chats do not show in the web UI [inferred].
- Response fully buffered; longest type-1 chunk wins, thinking fallback; chunk text of 20 chars or less is ignored (protocol.py:262-290), which can drop short answers.
- No citations, no sources_used, no error-chunk decoding.
- Local conversation cache is an unbounded dict (client.py:606-628).

## Auth lifecycle
- `login` MCP tool / `notebooklm-mcp-2026 login` launches Chrome with `--remote-debugging-port`, dedicated `--user-data-dir`, `--remote-allow-origins=*`, `--disable-extensions`, opens notebooklm.google.com (auth.py:195-205), waits for login (default 300 s), reads `Network.getAllCookies` (auth.py:324-328: all cookies of the isolated profile), keeps `ESSENTIAL_COOKIES` (config.py:75-84), saves JSON. Port scan 9222-9231 (auth.py:39-40, 137-150). Chrome only: macOS /Applications path, Linux which(...), Windows only `C:\Program Files\Google\Chrome` (auth.py:119-134); no Edge/Brave.
- Required cookies SID, HSID, SSID, APISID, SAPISID (config.py:72).
- Storage: `platformdirs.user_data_dir("notebooklm-mcp-2026")/auth.json` 0600, dir 0700 (config.py:14-17; auth.py:75-92). Override `NOTEBOOKLM_MCP_DATA_DIR`. Single account only.
- Expiry: 401/403 or RPC error 16 -> refresh CSRF once via homepage fetch -> retry (client.py:206-309). No headless relogin, no RotateCookies, rotated cookies not persisted.
- Windows launcher exits code 0 while Chrome survives: treat as OK if CDP is live (auth.py:247-262; CHANGELOG 0.2.1, PR #19).

## MCP tools (9; tools/__init__.py:1-18)
| Tool | Params | Behavior |
|---|---|---|
| login | timeout=300 | Interactive Chrome login, blocks |
| check_auth | - | loads saved tokens, fetches homepage |
| list_notebooks | max_results=50 | wXbhsf |
| get_notebook | notebook_id | rLM1Ne plus sources |
| list_sources | notebook_id | sources with type names (config.py:55-66), url |
| get_source_content | source_id | hizoJc full text |
| query_notebook | notebook_id, query, source_ids, conversation_id | streamed chat; returns answer, conversation_id, turn_number, is_follow_up |
| add_source_url | notebook_id, url | izAoDd, http/https only |
| add_source_text | notebook_id, text, title="Pasted Text" | izAoDd |

Transport: stdio only (server.py docstring). CLI subcommands: serve, login, logout, setup, status, doctor, version, help (cli.py).

## Reliability
Retry 429/500/502/503/504, 3 tries, backoff 1,2,4 up to 16 s (config.py:122-128; client.py:236-258). Timeouts 30 s default, 120 s add-source, 120 s query (`NOTEBOOKLM_QUERY_TIMEOUT`) (config.py:88-92). Timeout on add-source returns `{"status":"timeout"}` with "may have succeeded" (client.py:497-507).

## Performance notes
Sync httpx client reused (client.py:129-145); each query first calls get_notebook to enumerate sources (one extra RTT, client.py:563-566).

## Tests and quality
CI matrix ubuntu/macos/windows, ruff, pre-commit, pytest-cov, unit tests for all 9 tools (CHANGELOG 0.2.0). SECURITY.md with a threat model.

## Versions
0.1.0 2026-02-14 first release; 0.1.1/0.1.2 2026-02-14 logout, branded help; 0.2.0 2026-02-15 CI, tests, docs; 0.2.1 2026-05-01 Windows launcher fix. The first commit was named `notebook-julian`, renamed to `notebooklm-mcp-2026` the same day (git 586c297).

## Adopt
1. Clean split: pure `protocol.py` (no I/O) vs `client.py` vs `config.py` constants (config.py:1-5, protocol.py:1-12). Mirror as separate Go packages.
2. Auth security posture (0700 dir, 0600 file, exec without shell, cleanup in finally, store only essential cookies) (auth.py:1-9).
3. Windows launcher-exit-0 handling (auth.py:247-262).
4. Pure-function parser tests with fixtures.

## Avoid
1. `Network.getAllCookies` (auth.py:324-328) and `--remote-allow-origins=*` (auth.py:203).
2. Hard-coded `bl` and host (config.py:25-39); no `notebook.google.com`.
3. Dropping chunks of 20 chars or less (protocol.py:262-290).
4. Ignoring non-16 RPC error codes and rate limiting.
5. Only 9 tools: no studio, research, notes, upload, delete.

## Evidence
Read in full: config.py, protocol.py, tools/*.py, server.py (head), client.py (141-628), auth.py (1-210, 324-420), CHANGELOG.md (head). git log (42 commits), tags.
