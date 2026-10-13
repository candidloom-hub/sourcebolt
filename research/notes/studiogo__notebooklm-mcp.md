# studiogo/notebooklm-mcp

- URL: https://github.com/studiogo/notebooklm-mcp (fork of alfredang/notebooklm-mcp)
- Language / runtime: Python >=3.13, FastMCP >=2.14.4, `notebooklm-py[browser]>=0.3.4`, `browser-cookie3>=0.20.1`; optional `playwright` extra (`pyproject.toml`)
- License: MIT (LICENSE: "MIT License / Copyright (c) 2026 Lukasz Hodorowicz"; `pyproject.toml` `license = {text="MIT"}`). GitHub shows NOASSERTION; the file is plain MIT.
- Stars / last commit: 7 stars (research/upstreams.md); HEAD `731e3b8` 2026-08-28
- Versions reviewed: manifest 1.1.0 / CHANGELOG v1.1.0 (2026-05-09). No git tags exist (`git tag` empty). 4 commits ahead of alfredang/main (`git rev-list --left-right --count up/main...HEAD` = 0 behind, 4 ahead): `70e5d16` 2026-04-30 (27 tools + cookie bypass + auto-refresh), `7de0a38` 2026-05-09 (cross-platform, v1.1.0), `4d99b11` and `731e3b8` 2026-08-28 (CONTRIBUTING, code of conduct, remove dead link).

## Access method

Internal `batchexecute` RPC through `notebooklm-py` (`server.py:9` `from notebooklm import NotebookLMClient`; `server.py:49` `NotebookLMClient.from_storage()`). No wire code in this repo. Single file `server.py` (403 lines) + two auth scripts (`scripts/import_chrome_cookies.py` 149 lines, `scripts/login_interactive.py` 118 lines).

The upstream fork parent (alfredang, 2026-01-26/27) had 15 tools; this fork reports 27 total (`CHANGELOG.md` v1.0.0 "12 dodatkowych narzedzi", `grep -c "@mcp.tool()" server.py` = 27).

## RPC catalog

None of its own. notebooklm-py calls used (`server.py`): `notebooks.list/create` (`:122, 130`), `sources.add_url/add_text/add_file/add_drive` (`:139, 148, 161, 170`), `chat.ask` (`:186, 199`), `artifacts.generate_video/audio/slide_deck/mind_map/infographic/quiz/flashcards/report/data_table` (`:208-293`), `artifacts.wait_for_completion` (`:307`), `artifacts.download_*` x9 (`:325-397`). RPC ids: see the teng-lin note.

## Streaming/chat

`ask_notebook` awaits `chat.ask` and returns `{answer, conversation_id, references:[{citation, cited_text, chunk_id}]}` (`server.py:175-192`) built from notebooklm-py `ChatReference` (`citation_number`, `cited_text`, `chunk_id`). `get_notebook_summary` is faked by asking a chat question "Please provide a comprehensive summary and key insights of this notebook." (`:196-201`) instead of calling the summary RPC `VfAZjd`. No conversation id input, no source scoping.

## Auth lifecycle (the "+cookie auto-refresh" claim)

1. Cookie source: local Chrome profiles. `scripts/import_chrome_cookies.py` scans `Default` and `Profile*` dirs under the OS Chrome user-data dir (macOS `~/Library/Application Support/Google/Chrome`, Windows `%LOCALAPPDATA%\Google\Chrome\User Data`, Linux `~/.config/google-chrome|google-chrome-stable|chromium`) (`import_chrome_cookies.py:25-39`), reads each profile's `Cookies` SQLite via `browser_cookie3.chrome(cookie_file=..., domain_name="google.com")` (`:52-61`), keeps profiles with a `SID` cookie on `.google.com`/`notebooklm.google.com` (`:64-69`), picks the profile with the most `*.google.com` cookies (`:110-114`), and writes Playwright-format `~/.notebooklm/storage_state.json` chmod 600 with all `*.google.com` cookies (`:124-141`). Note `cookie_to_playwright` hardcodes `sameSite: "None"` and drops real flag detail (`:71-83`).
2. Fallback: `scripts/login_interactive.py`, Playwright Chromium window, user logs in by hand (including 2FA), script waits for a `notebooklm.google.com/**` URL, validates `SID`, exports cookies (`CHANGELOG.md` v1.1.0 section).
3. Auto-refresh in the server (`server.py:26-114`):
   - `_refresh_cookies_from_chrome()` runs `sys.executable scripts/import_chrome_cookies.py` as an async subprocess (`:26-44`).
   - `get_client()` (guarded by one `asyncio.Lock`) builds the client lazily; if the first build throws, it refreshes cookies and rebuilds once (`:78-96`).
   - `call_with_retry(fn)` wraps every tool; on any exception whose lower-cased text contains one of `auth, expired, 401, 403, redirect, signin, login, null result, null data, rpc`, it drops the client, refreshes cookies from Chrome and retries once (`:99-114`). v1.1.0 widened the filter with `null result`, `null data`, `rpc` (`CHANGELOG.md`).
   - `lifespan` does not crash when auth fails; the client is built on first tool call (`:65-72`).
4. Limits: refresh only works while Chrome has a fresh signed-in session, and Chrome must be readable by the server user. Windows DPAPI via browser_cookie3 "partial", Linux needs GNOME Keyring/KWallet (`import_chrome_cookies.py:1-12`). Chrome 127+ app-bound cookie encryption on Windows commonly breaks this approach [unverified]. Does NOT rotate `__Secure-1PSIDTS` (no RotateCookies call), so it re-imports rather than keeps a session alive. Single account (`CHANGELOG`: picks the richest profile).
5. Security: reads the user's real browser cookie store and writes it to disk; the MCP server can spawn that script on any error text match.

## MCP tools

27 tools, FastMCP, stdio (`mcp.run()` `server.py:402-403`). Every tool is wrapped in `call_with_retry`. All take raw `notebook_id`; no name resolution.

| Tool | Params | Behavior (server.py:line) |
|---|---|---|
| `list_notebooks` | none | id, title, sources_count (`:117`) |
| `create_notebook` | title | (`:126`) |
| `add_source_url` | notebook_id, url | (`:135`) |
| `add_source_text` | notebook_id, title, text | (`:144`) |
| `add_source_file` | notebook_id, file_path, mime_type? | abs or ~ path, `sources.add_file` (`:153`) |
| `add_source_drive` | notebook_id, file_id, title, mime_type=Google Doc | (`:166`) |
| `ask_notebook` | notebook_id, question | answer + references (`:182`) |
| `get_notebook_summary` | notebook_id | implemented as a chat prompt (`:195`) |
| `generate_video_overview` | notebook_id, instructions | returns task_id only (`:204`) |
| `generate_audio_overview` | notebook_id, instructions | (`:213`) |
| `generate_slide_deck` | notebook_id, instructions | (`:222`) |
| `generate_mind_map` | notebook_id | note-backed, returns note_id (`:231`) |
| `generate_infographic` | notebook_id, instructions, language="pl", orientation, detail_level | enums via `InfographicOrientation[...]`, default language is Polish (`:240`) |
| `generate_quiz` / `generate_flashcards` | notebook_id, instructions | no difficulty/quantity (`:263, 272`) |
| `generate_summary_report` | notebook_id, instructions | `generate_report(custom_prompt=...)` (`:281`) |
| `generate_data_table` | notebook_id, instructions | (`:290`) |
| `wait_for_completion` | notebook_id, task_id, timeout_seconds=300 | blocks inside the MCP call (`:303`) |
| `download_infographic/audio/video/slide_deck/mind_map/quiz/flashcards/report/data_table` | notebook_id, output_path, artifact_id? | `_expand` forces relative paths to `~/Desktop/<name>` and mkdirs parents (`:312-399`) |

Missing vs teng-lin: notebook delete/rename, source delete/list/rename, notes, research, sharing, chat config, labels, artifact list/status/delete, source scoping, citation text beyond `cited_text`.

## Reliability

Per-call single retry keyed on error text; no backoff; `wait_for_completion` blocks the tool call up to 300 s (can exceed MCP client timeouts); no concurrency control beyond one client lock; no tests in repo (`ls`: no tests dir). Logs to stderr (`server.py:11-15`).

## Versions

No tags. `CHANGELOG.md`: v1.1.0 2026-05-09 (Windows + Linux support, Playwright login script, call_with_retry filter widening), v1.0.0 2026-04-30 (cookies-from-Chrome, lazy server, auto-refresh, +12 tools). alfredang upstream commits 2026-01-26/27 ("premium NotebookLM MCP server with full artifact generation support", `0b39686`).

## Adopt

1. Lazy client + "never crash the server on auth failure" lifespan, then recover per call (`server.py:65-96`).
2. Chrome profile selection heuristic (most Google cookies) and the cross-OS Chrome path table (`import_chrome_cookies.py:25-39, 110-114`), as an optional `login --from-browser` helper; prefer a native Go reader or the `rookie` approach used by teng-lin and decrypt-capable code, and always verify required cookies (`SID` + `__Secure-1PSIDTS`).
3. Return `{citation, cited_text, chunk_id}` for each reference (`server.py:175-179`), but add source id/title (strato does).
4. Download tools that accept `artifact_id?` defaulting to latest (`:322-399`), paired with an explicit write-root restriction (see Avoid).

## Avoid

1. Error-text keyword matching to decide on auth retry (`server.py:107`): the words `rpc` and `null result` also match non-auth failures and will trigger a Chrome cookie re-import loop on ordinary RPC errors.
2. Spawning a Python script that reads the user's Chrome cookie DB from inside an MCP tool path (`server.py:26-44`): privilege and secret-handling hazard; make cookie import an explicit user command.
3. Cookie export that discards cookie attributes (hard-coded `sameSite: "None"`, `import_chrome_cookies.py:80`).
4. `_expand` silently relocating relative output paths to the Desktop and creating arbitrary directories (`server.py:312-318`): path traversal / surprise writes.
5. Summary implemented as a chat question (`server.py:196-201`) consuming daily chat quota; use `VfAZjd`.
6. Hard-coded Polish default language for infographics (`server.py:245`).
7. Python >=3.13 requirement and no tests.

## Evidence

Read in full: `server.py` (403 lines), `scripts/import_chrome_cookies.py`, `CHANGELOG.md` (first 60 lines), `pyproject.toml`; `git log`, `git rev-list` against alfredang remote added only in clone config. Not executed.
