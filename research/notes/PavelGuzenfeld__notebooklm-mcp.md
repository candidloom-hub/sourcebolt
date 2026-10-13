# PavelGuzenfeld/notebooklm-mcp (PyPI: mcp-notebooklm)

- URL: https://github.com/PavelGuzenfeld/notebooklm-mcp
- Language / runtime: Python >=3.10, official `mcp[cli]` SDK (`mcp.server.fastmcp.FastMCP`) plus `notebooklm-py>=0.3.0` (pyproject.toml:14-17). 1100 lines total (server.py 501, auth_cli.py 336).
- License: MIT, (c) 2026 Pavel Guzenfeld (LICENSE:1-3). Snippets OK.
- Stars / last commit: 2 stars; HEAD `bb90687` 2026-09-17 (merge of a mutation-gate opt-out); 7 commits.
- Versions reviewed: v0.1.0 (2026-03-15), v0.1.1 (2026-04-02). Package was renamed to `mcp-notebooklm` for PyPI availability (commit c865c1a, 2026-03-15).

## Access method
This server owns NO protocol code. It is a thin MCP façade over the `notebooklm-py` SDK (teng-lin/notebooklm-py; see research/notes/teng-lin__notebooklm-py.md for its RPC ids, endpoints and parsing). Every tool is one `await c.<area>.<method>(...)` call on a shared `NotebookLMClient.from_storage(path=NOTEBOOKLM_AUTH_JSON)` (server.py:37-66). Consequently the wire behavior, RPC ids and failure modes are exactly those of notebooklm-py at the installed version.
"No Playwright required" (README.md:194) is true for AUTHENTICATION only: `notebooklm-auth` is a paste-your-cookies CLI (below). `notebooklm-py` itself needs its `[browser]` extra (Playwright) for its own `notebooklm login`; this repo lists that as an optional extra (pyproject.toml:19-20) and its error message still says "pip install notebooklm-py[browser]" (server.py:48-52). So Playwright is avoided at runtime, not eliminated as a possible dependency.

## RPC catalog
None of its own. Auth cookie file format = Playwright `storage_state.json` (auth_cli.py:130-148) consumed by notebooklm-py. See the teng-lin note for ids. No RPC ids originate here.

## Streaming/chat
`c.chat.ask(notebook_id, question, source_ids=...)` and `c.chat.get_history(...)` (server.py:248-273). Streaming format, conversation and citation handling are entirely in notebooklm-py. This wrapper returns the SDK result serialized with `json.dumps(obj.__dict__, default=str)` (server.py:90-111), so citations appear only if the SDK dataclass carries them. No streaming surfaced to the MCP client (single JSON string result).

## Auth lifecycle
- `notebooklm-auth` (auth_cli.py): user opens notebooklm.google.com, copies cookies via (1) "Copy as cURL" from DevTools, (2) raw `Cookie:` header, (3) JSON export of an extension (auth_cli.py:55-93). `parse_input` auto-detects JSON, cURL (`-H 'Cookie:'`, `--header`, `-b`) or raw header (auth_cli.py:96-163).
- Only `document.cookie` (non-httpOnly) is mentioned as a JS option and flagged as insufficient (auth_cli.py:44-47); use Network tab for HttpOnly cookies.
- Filters to a list of Google auth cookies plus any `__Secure-*` (auth_cli.py:27-33, 223-233); minimum required is only `SID` (auth_cli.py:25), whereas jacob-bd requires SID, HSID, SSID, APISID, SAPISID.
- Stores `~/.notebooklm/storage_state.json` (override `NOTEBOOKLM_HOME`) chmod 0600; all cookies set on domain `.google.com`, `expires: -1`, httpOnly, secure (auth_cli.py:150-175). Alternative: env `NOTEBOOKLM_AUTH_JSON` containing the JSON inline (README.md:203-209) or `NOTEBOOKLM_AUTH_JSON` as a path for `from_storage` (server.py:56-57) [the variable is used both ways; unverified which one notebooklm-py accepts].
- `--verify` fetches the CSRF token with urllib against the homepage (auth_cli.py:290-336).
- No refresh, no expiry detection, no rotation, single account: "Cookies ... expire periodically" (README.md). Refresh behavior is whatever notebooklm-py does.

## MCP tools (31; README.md:73, 135; server.py)
All take `notebook_id` first unless noted. Registered with `@mcp.tool()`; results are JSON strings or plain "OK"/"Deleted".

| Tool | Params | Behavior (SDK call) | line |
|---|---|---|---|
| notebook_list | - | notebooks.list | 121 |
| notebook_create | title | notebooks.create | 129 |
| notebook_get | notebook_id | notebooks.get | 137 |
| notebook_delete | notebook_id | notebooks.delete (no confirm flag) | 145 |
| notebook_rename | notebook_id, new_title | notebooks.rename | 153 |
| notebook_summary | notebook_id | notebooks.get_summary | 161 |
| source_list | notebook_id | sources.list | 173 |
| source_add_url | notebook_id, url | sources.add_url(wait=True) | 181 |
| source_add_text | notebook_id, title, content | sources.add_text(wait=True) | 189 |
| source_add_file | notebook_id, file_path | sources.add_file(wait=True); path resolved with expanduser, no directory confinement | 199 |
| source_get | notebook_id, source_id | sources.get | 212 |
| source_delete | notebook_id, source_id | sources.delete | 220 |
| source_fulltext | notebook_id, source_id | sources.get_fulltext | 228 |
| source_guide | notebook_id, source_id | sources.get_guide | 236 |
| chat_ask | notebook_id, question, source_ids (comma string) | chat.ask | 248 |
| chat_history | notebook_id | chat.get_history | 268 |
| chat_configure | notebook_id, goal, response_length, custom_prompt | chat.configure | 276 |
| artifact_list | notebook_id | artifacts.list | 308 |
| artifact_generate_audio | notebook_id, source_ids, instructions | artifacts.generate_audio | 316 |
| artifact_generate_report | notebook_id, topic | artifacts.generate_report | 340 |
| artifact_generate_study_guide | notebook_id | artifacts.generate_study_guide | 355 |
| artifact_generate_quiz | notebook_id | artifacts.generate_quiz | 363 |
| artifact_generate_flashcards | notebook_id | artifacts.generate_flashcards | 371 |
| artifact_generate_mind_map | notebook_id | artifacts.generate_mind_map | 379 |
| artifact_download_audio | notebook_id, artifact_id, output_path | artifacts.download_audio; path unconfined | 387 |
| artifact_poll_status | notebook_id, artifact_id | artifacts.poll_status | 398 |
| artifact_delete | notebook_id, artifact_id | artifacts.delete | 408 |
| research_start | notebook_id, query, source(web\|drive), mode(fast\|deep) | research.start | 420 |
| research_poll | notebook_id | research.poll | 441 |
| sharing_status | notebook_id | sharing.get_status | 453 |
| sharing_set_public | notebook_id, public(bool) | sharing.set_public | 461 |

Missing vs jacob-bd: video, infographic, slides, data table, notes, labels, import research results, Drive sources, export to Docs/Sheets, revise slides, download of non-audio artifacts, usage, profiles. Transports: stdio, streamable-http, sse; HTTP defaults to host `0.0.0.0` port 8484 with no auth (server.py:474-501).

## Reliability
No retries, timeouts or error mapping of its own: exceptions propagate from the SDK to FastMCP. Single `asyncio.Lock` guards lazy client creation (server.py:37-66). Source adds always `wait=True`, so a slow source blocks the tool call until the SDK timeout.

## Performance notes
Async client reused for the process lifetime (AppContext, server.py:37-76). No concurrency limit. Nothing measured.

## Tests and quality
`test_auth_parsing.py` runs assertions at module level (zero pytest items); `test_mcp.py` is a live e2e script. The repo opted out of its mutation gate because there is no suite to measure (`.mutation-gate.toml:1-10`, PR #1/#2). CI exists (.github/workflows/ci.yml) [content not reviewed].

## Versions
v0.1.0 2026-03-15 initial; same day rename to `mcp-notebooklm`; v0.1.1 2026-04-02 author URL metadata. Sep 2026 commits are CI-policy only.

## Adopt
1. Cookie-paste ingestion that auto-detects cURL / Cookie header / JSON array / JSON object with `cookies` key (auth_cli.py:96-163) as a fallback `login --manual` in the Go server; it is the only way for HttpOnly cookies without a browser.
2. Filtering input cookies to a known allowlist plus `__Secure-*` prefix (auth_cli.py:223-233), with fallback to keep all if filtering empties.
3. Tool grouping by area (notebook_, source_, chat_, artifact_, research_, sharing_) is easy for LLMs; names are consistent (server.py section headers).
4. Accept `source_ids` as a comma string (works around clients that stringify arrays) (server.py:248-262); jacob-bd later coerced JSON strings too (CHANGELOG 0.5.10).

## Avoid
1. HTTP transport bound to `0.0.0.0` with no auth by default (server.py:487-499).
2. Unconfined `file_path` / `output_path` read-write from LLM-provided strings (server.py:199-210, 387-396).
3. `_serialize` via `obj.__dict__` (server.py:90-111): leaks internals and drops fields on dataclasses with slots or properties.
4. Inheriting all behavior from an SDK; no insulation if notebooklm-py changes RPC ids.
5. Requiring only `SID` cookie (auth_cli.py:25) will pass validation but fail API calls later.
6. Destructive tools without confirmation (notebook_delete, source_delete, artifact_delete).

## Evidence
Read fully: notebooklm_mcp/server.py, notebooklm_mcp/auth_cli.py (to 336), README.md (auth, tools, caveats), pyproject.toml, .mutation-gate.toml (head), git log and tags.
