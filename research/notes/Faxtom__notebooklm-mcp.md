# Faxtom/notebooklm-mcp (PyPI: notebooklm-mcp-faxtom)

- URL: https://github.com/Faxtom/notebooklm-mcp (pyproject URLs still say `Faxtom/notebooklm-mcp-2026`, pyproject.toml:36-39)
- Language / runtime: Python, FastMCP, httpx, websocket-client, rich, questionary, `browser-cookie3>=0.19.1` (pyproject.toml:25-33)
- License: MIT, (c) 2025 Julian (LICENSE), carried over from the parent.
- Stars / last commit: 0 stars; HEAD `e8a5672` 2026-06-18 "Release v0.3.7". 51 commits.
- Versions reviewed: tags v0.3.0 .. v0.3.7 (2026-06-17/18); inherits v0.1.0-v0.2.1.

## Relation to julianoczkowski/notebooklm-mcp-2026 (answer: fork, not rename)
- Evidence: the two clones share 41 commits with identical SHAs back to `eac3522` (2026-02-14). Faxtom has 10 own commits after `fe034f2` (v0.2.1, 2026-05-01), the first being `e764e04` "Add multi-browser auth, Helium support, and one-command login+setup" (2026-06-17). Julian has 1 commit Faxtom lacks (`bb78b41`, README author section, 2026-07-06). CHANGELOG 0.3.0: "Fork by @Faxtom - based on julianoczkowski/notebooklm-mcp-2026". README:17 "This fork improves on julianoczkowski/...". GitHub's fork flag is false (research/upstreams.md) because it was pushed as a standalone repo. The GitHub repo was renamed (notebooklm-mcp-2026 -> notebooklm-mcp), but the Python package and CLI command keep the name `notebooklm-mcp-2026`; PyPI name is `notebooklm-mcp-faxtom` (CHANGELOG 0.3.2). So research/upstreams.md "renamed from notebooklm-mcp-2026" is right for the GitHub repo name only; the project is a fork of Julian's repo, which still exists.
- Delta vs parent (`git diff --stat fe034f2..HEAD -- src`): auth.py +1113 lines, cli.py +606, client.py +84, config.py +49, tools/auth_tools.py +68, version strings. protocol.py and the other tools are untouched. Everything in the julianoczkowski note about RPC, chat and tools applies verbatim (research/notes/julianoczkowski__notebooklm-mcp-2026.md).

## Access method / RPC catalog / Streaming
Identical to the parent: batchexecute with 5 RPC ids (wXbhsf, rLM1Ne, hizoJc, izAoDd, tr032e) and GenerateFreeFormStreamed; hard-coded host and `bl` (src/notebooklm_mcp_2026/config.py). The only client.py change is auth recovery that calls `try_silent_token_refresh()` on CSRF/auth failure (client.py:163, 305). No new RPC ids.

## Auth lifecycle (the fork's real contribution)
`login --method auto|browser|profile|cdp|import` and `--browser` (cli.py:1033, 1051-1055):
1. Import from installed browsers' cookie databases via `browser_cookie3`; order Helium, Chrome, Edge, Brave, Chromium, Opera, Vivaldi, Firefox, LibreWolf, Safari (auth.py:587-612, 462-476, 479-506). Only `ESSENTIAL_COOKIES` kept. On Windows the DB is locked while the browser runs: errors are classified (names/messages) and a hint to use CDP is shown (auth.py:189-211).
2. Helium: user-data dir per OS (Windows `%LOCALAPPDATA%\imput\Helium\User Data`, macOS `~/Library/Application Support/net.imput.helium`, Linux `~/.config/helium`) (auth.py:214-221); profiles listed from `Local State`; picks `profile.last_used`, not always `Default` (auth.py:224-320; CHANGELOG 0.3.7); env `NOTEBOOKLM_HELIUM_PROFILE`; cookie DB `Network/Cookies` or `Cookies` plus the `Local State` key file passed to `browser_cookie3.chrome(cookie_file=, key_file=)` (auth.py:323-401). Helium = imputnet Chromium fork [per README only].
3. CDP import from an ALREADY RUNNING browser started with remote debugging: port candidates 9222, `NOTEBOOKLM_CDP_PORT`, `DevToolsActivePort` files (auth.py:949-1007); enables Network, navigates to NotebookLM, scans all open tabs/profiles and picks the best Google session (auth.py:1224-1268; CHANGELOG 0.3.6, 0.3.7). `enable-cdp` writes a desktop launcher with `--remote-debugging-port=9222`; on Windows it first closes existing Helium processes (auth.py:1091-1164; CHANGELOG 0.3.5).
4. Isolated Chromium profile interactive login (parent behavior) generalized to any Chromium browser, with a headless mode (auth.py:835-936).
5. JSON cookie file import in three formats (auth.py:509-540).
- Silent refresh with cooldown (`NOTEBOOKLM_AUTH_REFRESH_COOLDOWN`, default 300 s): 1) browser import, 2) headless CDP on the persistent profile (auth.py:543-584; config.py:138).
- Storage unchanged: one `auth.json` 0600, single account. Required cookies unchanged (SID, HSID, SSID, APISID, SAPISID).

## MCP tools (9, same names as parent; tools/__init__.py)
login (browser/method options, tools/auth_tools.py), check_auth, list_notebooks, get_notebook, list_sources, get_source_content, query_notebook, add_source_url, add_source_text. CLI adds `login` that verifies the account then writes MCP client configs, `enable-cdp`, interactive `setup` (questionary multi-select of detected clients, cli.py:340-370), `doctor`, `status`, `logout`, `serve`.

## Reliability
Inherited retry (3x, 1-16 s). New: cooldown-guarded silent refresh.

## Performance notes
No change. `browser_cookie3` only loaded at login/refresh.

## Tests and quality
Cross-platform auth tests added for the Linux CI matrix (Helium paths, "Playwright Chromium detection", CHANGELOG 0.3.1) although Playwright is not a dependency (pyproject.toml:25-33).

## Versions
v0.3.0 2026-06-17 multi-browser auth, Helium, login+setup; v0.3.1 2026-06-17 CI fixes; v0.3.2 PyPI rename to notebooklm-mcp-faxtom; v0.3.3 2026-06-18 CDP import while browser open; v0.3.5 Helium launcher fix; v0.3.6 `auto` fallback fix; v0.3.7 Helium multi-profile.

## Adopt
1. Browser discovery as a data table (executable paths, user-data dirs, cookie DB paths, Local State) for Helium/Chrome/Edge/Brave/Chromium/Opera/Vivaldi (auth.py:587-760, 214-345). In Go, reading Chromium cookie DBs needs SQLite plus OS crypto (DPAPI/Keychain/libsecret); recent Chrome on Windows may use app-bound encryption which blocks it [unverified], so CDP import is the robust path.
2. Import through a running browser's CDP so the browser can stay open; scan all tabs, choose the one with a real Google session (auth.py:1224-1268).
3. Port discovery from `DevToolsActivePort` files (auth.py:980-995).
4. Locked-DB error classification with actionable hints (auth.py:189-211).
5. Silent-refresh ladder with cooldown (auth.py:543-584).

## Avoid
1. `browser_cookie3`-style decryption of the user's daily browser profile: reads all `.google.com` cookies; prefer an isolated managed profile or URL-scoped `Network.getCookies`.
2. Killing the user's Helium/Chrome processes to relaunch with debug flags (CHANGELOG 0.3.5).
3. Single-account `auth.json`.
4. Everything under Avoid in the parent note (hard-coded host/bl, 9 tools).
5. Naming drift: package/command `notebooklm-mcp-2026`, PyPI `-faxtom`, repo URLs pointing at the old repo name (pyproject.toml:36-39).

## Evidence
Read: CHANGELOG.md (head), README.md (grep), pyproject.toml, auth.py (189-615, 949-1270 selected), cli.py (grep), config.py:138, git log and SHA-set comparison against the julianoczkowski clone, git diff --stat fe034f2..HEAD.
