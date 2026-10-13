# khengyun/notebooklm-mcp

- URL: https://github.com/khengyun/notebooklm-mcp (PyPI `notebooklm-mcp`)
- Language / runtime: Python >=3.10, FastMCP >=2.0 (`pyproject.toml`)
- License: MIT (LICENSE file text begins "MIT License / Copyright (c) 2025 Kangkhengyun (khengyun)", `LICENSE:1-3`; `pyproject.toml` `license = "MIT"`). GitHub reports NOASSERTION, so research/upstreams.md "NOASSERTION" is a detection artifact, the file is plain MIT.
- Stars / last commit: 83 stars (research/upstreams.md); HEAD `408609f` 2026-06-17 (merge PR #18 video + mind map)
- Versions reviewed: pyproject `2.0.11`; HEAD 408609f. Tags v2.0.1 to v2.0.11 are ALL dated 2025-09-15 and belong to the old browser-automation generation; the 2026-06-17 rewrite has no new tag.

## Access method

Internal `batchexecute` RPC, entirely delegated to `notebooklm-py[browser]>=0.7` (`pyproject.toml` dependencies; `src/notebooklm_mcp/client_rpc.py:1-17, 30-34`). The server owns zero RPC ids or wire code. History: v2.0.x (Sept 2025) drove the DOM with a browser (Selenium/Patchright); commit `4563f2e` (2026-06-17) added the RPC engine, `a8e6bbe` "Remove Patchright engine; RPC-only with deep code reduction". Today `config.py` still carries browser-era fields (`headless`, `profile_dir`, `skip_manual_login`) that are vestigial (`config.py:31-50`).

Code size: `server.py` 485 lines, `client_rpc.py` 426, `cli.py` 521, `config.py` 183 (`wc -l`).

## RPC catalog

None of its own. Maps one-to-one onto notebooklm-py methods (`client_rpc.py`): `notebooks.list/create/rename/delete/get_summary` (`:214-232`), `sources.list/add_url/add_text/delete/get/get_fulltext` (`:237-253, 258-281`), `chat.ask` (`:198`), `artifacts.generate_audio/list_audio/generate_video/list_video` (`:317-351`), `mind_maps.generate/list/get/get_tree` (`:364-387`), `sharing.get_status/set_public/add_user` (`:395-413`). RPC ids: see `research/notes/teng-lin__notebooklm-py.md`.

## Streaming/chat

`send_chat_message` calls `backend.chat.ask(notebook_id, message)`, stores `result.answer`, and `get_chat_response` returns the stored string (`client_rpc.py:194-205`). No citations, no conversation id returned, no source scoping, no streaming to the MCP client: the whole answer is awaited. Conversation continuity relies on notebooklm-py's own cache. State kept in server: `current_notebook_id` (mutable, shared by all callers, `:111, 207-209`) and `_last_answer` (`:115`), so two concurrent chat tool calls can overwrite each other's answer. Read-after-write pairing (`send_chat_message` then `get_chat_response`) is only valid because the call is synchronous.

## Auth lifecycle

- Delegated to notebooklm-py: `NotebookLMClient.from_storage(path=...)` using `auth.storage_state_path`, else `<profile_dir>/storage_state.json`, else notebooklm-py defaults (`NOTEBOOKLM_AUTH_JSON`, `~/.notebooklm/storage_state.json`) (`client_rpc.py:124-167`).
- The server never launches a browser or refreshes cookies; failure to open raises `AuthenticationError("No NotebookLM session found. Run notebooklm login ...")` (`:142-149`). `authenticate()` verifies by `notebooks.list()` and only sets a boolean (`:169-178`); a failed check does not raise, so tools later fail per call.
- Lazy init on first tool call, reset to `None` on failure so the next call retries (`server.py:68-87`). No concurrency lock around `_ensure_client` (two simultaneous first calls can build two clients).
- No cookie auto-refresh, no keepalive, single account.

## MCP tools

28 tools via `@self.app.tool()` (FastMCP v2), plain typed params (flat schema, commit `c7c75e6`), wrapped by `@_tool(prefix)` which logs and re-raises as `NotebookLMError("<prefix>: <exc>")` (`server.py:29-53`). Transports: stdio default, `http` at `/mcp/`, `sse` (`server.py:427-447`).

| Tool | Params | Behavior (server.py:line) |
|---|---|---|
| `healthcheck` | none | status healthy/needs_auth/unhealthy, mode (`:97`) |
| `send_chat_message` | message, wait_for_response=True | ask and store answer (`:116`) |
| `get_chat_response` | none | returns stored last answer (`:131`) |
| `chat_with_notebook` | message, notebook_id? | navigate + ask in one call (`:142`) |
| `navigate_to_notebook` | notebook_id | sets in-process current notebook only (`:161`) |
| `get_default_notebook` / `set_default_notebook` | / notebook_id | config default (`:173, 183`) |
| `list_notebooks` | none | id, title, emoji, source_count (`:200`) |
| `create_notebook` | title | (`:212`) |
| `rename_notebook` | notebook_id, new_title | (`:222`) |
| `delete_notebook` | notebook_id | no confirmation gate (`:230`) |
| `get_notebook_summary` | notebook_id | `notebooks.get_summary` (`:238`) |
| `list_sources` | notebook_id | id, title, type, status (`:246`) |
| `add_source_url` | notebook_id, url | URL or YouTube via `sources.add_url` (`:254`) |
| `add_source_text` | notebook_id, title, text | (`:262`) |
| `delete_source` | notebook_id, source_id | (`:272`) |
| `copy_source` | source_notebook_id, source_id, target_notebook_id | client-side copy: re-add by URL if the source has a URL, else re-add fulltext as text (`:284`, `client_rpc.py:258-281`); does NOT use the server RPC `R27wvc` |
| `create_notebook_from_sources` | title, sources=[{notebook_id, source_id}] | creates notebook, copies each source, returns copied/failed (`:296`, `client_rpc.py:283-303`) |
| `generate_audio_overview` | notebook_id, instructions?, language="en" | returns task status; no format/length options (`:316`) |
| `list_audio_overviews` | notebook_id | `artifacts.list_audio` (`:329`) |
| `generate_video_overview` / `list_video_overviews` | same | (`:340, 353`) |
| `generate_mind_map` | notebook_id, kind=interactive/note_backed | (`:364`) |
| `list_mind_maps` / `get_mind_map` | notebook_id ; notebook_id, mind_map_id | tree via `get_tree` fallback (`:374, 386`) |
| `get_share_status` / `set_notebook_public` / `share_notebook_with_user` | notebook_id ; public ; email, permission viewer/editor | (`:397, 407, 416`) |

Gaps compared with teng-lin's 38 tools: no file upload, no Drive, no research, no quiz/flashcards/report/slide/infographic/data-table generation, no downloads, no status polling by task id, no notes, no labels, no chat config, no citations.

## Reliability

No retries, timeouts or rate-limit handling of its own (inherits notebooklm-py defaults). `ServerConfig.timeout=60` is defined but unused by the RPC engine (`config.py:50, 98` vs `client_rpc.py`). Tests: `tests/test_client_rpc.py` (1029 lines), `test_server.py` (717), `test_cli.py` (758), all with a monkeypatched backend class (`client_rpc.py:30-34` "monkeypatchable"); no live tests.

## CLI

`cli.py` (521 lines): config/profile management and server start (`notebooklm-mcp` entry). Not reviewed line by line [unverified beyond `wc -l` and import list].

## Versions

`git tag`: v2.0.1 to v2.0.11, all 2025-09-15 (browser engine era; 8 commits that day, last `25a1b6c` "Simplify user experience with 2-command workflow"). Post-tag: 2025-09-18 test-coverage commits; 2026-06-17 burst: `4563f2e` RPC engine, `a8e6bbe` drop Patchright, `c7c75e6` flatten schema, `09ace70` copy_source + create_notebook_from_sources, `9c12949` audio + sharing, `0352491` video + mind map. `pyproject` still says 2.0.11, so PyPI 2.0.11 likely predates the RPC rewrite [unverified against PyPI].

## Adopt

1. Tool-per-verb with flat, typed, documented params (`server.py:96-425`); FastMCP-style one-line descriptions that state side effects ("Setting public=true exposes the notebook to anyone with the link", `:408-409`).
2. `create_notebook_from_sources` result shape `{notebook, copied[], failed[]}` so one bad source never aborts a merge (`client_rpc.py:283-303`). In our server prefer the server-side copy RPC `R27wvc` (CopySourcesAsync) and `mKDdke` instead of re-adding by URL.
3. Lazy auth: start the MCP transport first, authenticate on first tool call, reset on failure (`server.py:68-87`, `_ensure_client`), so a stale session degrades one call not the process.
4. `healthcheck` tool reporting `authenticated` and mode (`server.py:97-112`).

## Avoid

1. Process-global mutable "current notebook" and "last answer" (`client_rpc.py:111-115, 194-209`): not safe with concurrent MCP calls; make chat stateless (notebook id + optional conversation id per call).
2. Splitting ask into send/get (`send_chat_message` + `get_chat_response`): a leftover from DOM automation (`client_rpc.py:192-205`); one `ask` call that returns answer + citations is correct.
3. Destructive tools without confirmation (`delete_notebook`, `delete_source`) and a public-link toggle without a gate (`server.py:230, 407`).
4. Dead config from the removed browser engine (`config.py:31-50`) and unused timeout.
5. Copying sources via re-fetching URLs (`client_rpc.py:258-281`): changes content, burns quota, loses uploads; use the copy RPC.
6. Tag/version drift: v2.0.11 tag is browser-era code, HEAD is a different product.

## Evidence

Read in full: `src/notebooklm_mcp/server.py`, `client_rpc.py`, first 60 lines of `config.py`, `pyproject.toml`, `LICENSE` head; `git tag`, `git log`. Not executed.
