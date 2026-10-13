# claude-world/notebooklm-skill

- URL: https://github.com/claude-world/notebooklm-skill (PyPI `notebooklm-skill`)
- Language / runtime: Python >=3.10; FastMCP >=2.13.1,<4; `notebooklm-py[browser]>=0.7.3,<0.8`; httpx; feedparser; python-dotenv (`pyproject.toml` dependencies)
- License: MIT, "Copyright (c) 2026 Claude World" (`LICENSE:1-3`, `pyproject.toml` license = "MIT")
- Stars / last commit: 468 stars (research/upstreams.md); HEAD `dc38bd8` 2026-07-18 "ship notebooklm-skill 1.3.0"
- Versions reviewed: 1.3.0 (HEAD). Tags v1.0.0 2026-03-14, v1.1.0 2026-03-15, v1.2.0 2026-03-17, v1.2.1 2026-03-17, v1.2.2 2026-03-22, v1.3.0 2026-07-18 (`git tag` + `git log -1 --format=%ad`).

## Access method (resolves the "[unverified]" in research/upstreams.md)

Internal `batchexecute` RPC, entirely via the third-party library `notebooklm-py` (teng-lin), NOT browser automation and NOT an official API.

Evidence:
- `pyproject.toml` dependency `notebooklm-py[browser]>=0.7.3,<0.8`; the `browser` extra only supplies Playwright for the login command.
- Every operation goes through `NotebookLMClient.from_storage()` (`scripts/common.py:56-72`, which imports `from notebooklm import AuthError, AuthExtractionError, NotebookLMClient`) and typed namespaces: `client.notebooks.list/create/delete/get_summary`, `client.sources.add_url/add_text/add_file/list`, `client.chat.ask`, `client.artifacts.*`, `client.research.start/wait_for_completion/import_sources` (`scripts/common.py:156, 313-330, 563, 639, 664-685`; `mcp_server/tools.py:44, 68, 80, 366`).
- Playwright is only invoked indirectly by `scripts/auth_helper.py`, which shells out to `python -m notebooklm login [--browser X] [--fresh]` (`auth_helper.py:41-63`) and `notebooklm auth logout`.
- No direct HTTP to Google except RSS fetching for the content pipeline (`common.py:238-262`, httpx with 30 s timeout and 5 MB cap).

Therefore it has no RPC ids and no wire code of its own. All RPC detail is in `research/notes/teng-lin__notebooklm-py.md`. Pin `<0.8` means it does NOT use the v0.8 Android backend or MCP server; it re-implements its own thin FastMCP server over notebooklm-py 0.7.x.

What it adds is a "research-to-content" workflow layer: five pipelines (create-and-ingest, trend-to-content via an external `trend-pulse` CLI, RSS/URL research, etc.) in `scripts/pipeline.py` (633 lines), a skill file `SKILL.md`, and a CLI `notebooklm-skill` (`scripts/notebooklm_client.py`, 542 lines).

## RPC catalog
None. See teng-lin note.

## Streaming/chat

`nlm_ask` calls `client.chat.ask(notebook_id, query)` and returns answer plus citations (`mcp_server/tools.py:349-390`, `server.py:116-119`). Answer is awaited whole; no streaming to the MCP client; no conversation id input.

## Auth lifecycle

- Session from notebooklm-py `storage_state.json`, profile aware: `storage_path(profile)` uses `notebooklm.paths.get_storage_path`, fallback `~/.notebooklm/storage_state.json` (`common.py:35-42`). `NOTEBOOKLM_PROFILE` env or `--profile` flag (`server.py:259-268`).
- `notebooklm-auth setup|verify|clear` wraps `python -m notebooklm login` (timeout configurable), verifies by a real `notebooks.list()`, logs out through upstream `auth logout` instead of deleting `~/.notebooklm` (`scripts/auth_helper.py:41-120`).
- Failure handling: `AuthError` and `AuthExtractionError` map to `AuthenticationRequired` with a tagged message `[AUTH_REQUIRED] ... Run uvx --from notebooklm-py notebooklm login` so an AI client can tell the user what to do (`common.py:45-72`; server instructions repeat it `server.py:34-43`). A missing upload file is deliberately NOT turned into an auth error (`common.py:65-72`).
- No cookie refresh, no keepalive, no refresh hook of its own (relies on notebooklm-py 0.7.x built-ins).
- `NOTEBOOKLM_AUTH_JSON` supported through upstream; `SECURITY.md` says treat it as a secret and use a secret manager.

## MCP tools

13 tools, FastMCP, stdio default or `--http` on 127.0.0.1:8765, non-loopback host refused with an explanatory error (`mcp_server/server.py:248-282`). Operational exceptions become `ToolError` (`:62-69`). Input `notebook` accepts an id or title (`common.py:150-175` `resolve_notebook`).

| Tool | Params | Behavior (server.py:line) |
|---|---|---|
| `nlm_create_notebook` | title, sources?, text_sources?, file_sources? | create + ingest; per-source outcomes, true requested/succeeded/failed counts (`:72-83`) |
| `nlm_list` | none | notebooks with ids, titles, source counts (`:86`) |
| `nlm_delete` | notebook, confirm=False | returns `requires_confirmation` unless `confirm=true` (`:92-101`) |
| `nlm_add_source` | notebook, url?, text?, text_title?, file_path? | exactly one of url/text/file (`:104-113`) |
| `nlm_ask` | notebook, query | grounded answer + citations (`:116`) |
| `nlm_summarize` | notebook | NotebookLM summary (`:122`) |
| `nlm_list_sources` | notebook | kind and processing status (`:128`) |
| `nlm_generate` | notebook, type, lang="en", instructions?, source_ids?, options{}, wait=True, timeout? | types audio, video, cinematic, slides, report, study-guide, quiz, flashcards, mind-map, infographic, data-table; options keys audio_format, audio_length, video_format, video_style, style_prompt, slide_format, slide_length, report_format, custom_prompt, quantity, difficulty, orientation, detail_level, style; `wait=false` returns task id (`:134-165`, `common.py:359-441, 457-505`) |
| `nlm_download` | notebook, type, output_path, output_format?, artifact_id?, force=False | refuses symlinks, directories, existing files unless `force` (`:168-191`, `common.py:573-626`) |
| `nlm_list_artifacts` | notebook, type? | real listing filtered by canonical type (`:194-200`, `common.py:629-640`) |
| `nlm_research` | notebook, query, mode fast/deep, wait=True, import_results=True, max_sources=10, timeout=1800 | start, wait for terminal, require `completed`, import top N (`:203-224`, `common.py:643-695`) |
| `nlm_research_pipeline` | sources[], questions[], output_format article/thread/report, title? | create notebook, ingest URLs, ask each question, assemble content (`:227-235`) |
| `nlm_trend_research` | geo="TW", count=5, platform threads/twitter/instagram/article | runs external `trend-pulse` CLI (`TREND_PULSE_CMD`, argv list, no shell, 60 s timeout, count 1-20), researches each trend, drafts platform text (`:238-245`, `common.py:186-235`) |

Server `instructions` string lists supported artifact types, the auth recovery command, and the two safety rules (delete confirm, no overwrite) (`server.py:32-43`).

## Reliability

- Source ingestion: bounded concurrency semaphore (default 4), per-source `wait=True, wait_timeout=180`, results returned in input order, partial failures reported as `{status:"failed", error}` (`common.py:280-345`).
- Artifact timeouts per type: audio 1800 s, video and cinematic 3600 s, infographic 2400 s, others 1800 s default (`common.py:369-404`); waits through `client.artifacts.wait_for_completion`.
- Research timeout 1800 s; non-`completed` terminal state raises (`common.py:677-681`).
- RSS reader bounded by timeout 30 s, 20 entries, 5 MB (`common.py:238-262`).
- Language alias table (`zh-tw` -> `zh_Hant`, `pt-br` -> `pt_BR`, ...) (`common.py:75-91`).
- Tests: 8 test files, about 1,500 lines, mocked (`tests/`).

## Versions

CHANGELOG `[1.3.0] 2026-07-18`: shared notebooklm-py 0.7.x compatibility layer, profile-aware auth, system Chrome/Edge login, safe skill installer, mixed-source ingestion with bounded concurrency, exact artifact listing and ID downloads for 11 types, full research lifecycle, rebuilt async pipelines with truthful partial failure, JSON-first CLI, loopback-only HTTP MCP, PEP 668-safe installer; fixes for `add_text` argument order, delete success reporting, mind-map generation, infographic download, shell injection in `make_video.sh`; security: downloads reject symlinks and existing files, delete needs confirm. `[1.2.1] 2026-03-17`: `[AUTH_REQUIRED]` error and server instructions. Commit `4dd8c55` 2026-03-22 "disable infographic artifact type (download unreliable)" (re-enabled in 1.3.0). Earlier: `0d4da34` 2026-03-15 raised artifact timeout from 10 min to 60 min.

## Adopt

1. `[AUTH_REQUIRED]` tagged error text plus server `instructions` telling the model exactly how to recover, so the agent can relay the fix to the user (`common.py:45-53`, `server.py:34-43`).
2. Safe download semantics: reject symlinks, directories, existing files unless `force`; validate type/format pairs before any network work (`common.py:573-626`). Add allow-listed write roots on top.
3. `confirm` gate on deletion with a structured `requires_confirmation` response (`server.py:92-101`).
4. Truthful partial-failure reporting for batch ingest (requested/succeeded/failed counts, per-item status, ordered results) (`common.py:280-345`).
5. `wait` flag on generation and research (`wait=false` returns the task id) with per-type timeouts (`server.py:134-165`, `common.py:369-404`).
6. Loopback-only HTTP transport with a hard refusal of other binds (`server.py:272-278`).
7. Shell-free subprocess for external tools with argv list and hard timeout (`common.py:186-215`).

## Avoid

1. Exact dependency window on a volatile unofficial library (`>=0.7.3,<0.8`) so every upstream minor requires a release of this package (`pyproject.toml`).
2. Letting a content-marketing workflow (trend-pulse, Threads/Twitter drafts, RSS) live inside the NotebookLM MCP server: unrelated attack surface (subprocess, network) and tool bloat (`server.py:227-245`). Keep the server to NotebookLM operations; expose pipelines as separate skills/prompts.
3. Python-installed-on-PATH assumptions (`python -m notebooklm login` shell-outs) for auth (`auth_helper.py:41-63`); a Go binary should implement its own login/capture or a documented import path.
4. `nlm_ask` with no conversation id or source scope, so multi-turn and scoped chat are impossible (`server.py:116-119`).
5. 3600 s blocking waits inside a tool call (`common.py:374-375`) vs the 60 s-ish client timeouts most MCP hosts apply; use detached start/poll.

## Evidence

Read: `pyproject.toml`, `mcp_server/server.py` (full), `mcp_server/tools.py` (call sites), `scripts/common.py` (lines 1-120, 150-346, 355-442, 573-695), `scripts/auth_helper.py` (1-120), `SECURITY.md`, `CHANGELOG.md` head, `git tag`, `git log`. Not executed. The Python `scripts/pipeline.py` and `notebooklm_client.py` were only grepped.
