# asccclass/notebooklm-go

**LICENSE: none** (no LICENSE file; upstreams.md also says none). **BEHAVIOR ONLY. No code copied or to be ported.**

- URL: https://github.com/asccclass/notebooklm-go
- Language / runtime: Go 1.22.2 (`go.mod:3`); deps: `go-rod/rod` + ysmood libs only. Hand-rolled MCP (no SDK). Binary name `notebooklm-mcp-go`.
- Stars / last commit: 1 star (per upstreams.md); HEAD `4358a4b` 2026-03-12; 4 commits (2026-03-11 initial, README zh-TW, SKILL, .gitignore).
- Versions reviewed: no tags. 
- Hygiene finding: **1597 of the repo's ~1616 tracked files are a committed Go build cache `.gocache/`** (`git ls-files | awk -F/ '{print $1}'`). Real source is ~3.3k lines: `internal/session/session.go` 647, `internal/tools/handlers.go` 547, `definitions.go` 371, `internal/library/library.go` 353, `internal/config/config.go` 321, `internal/mcp/server.go` 286, `internal/utils/settings.go` 182, `internal/auth/auth_manager.go` 181, `main.go` 125.
- What it is: a Go rewrite of `PleasePrompto/notebooklm-mcp` (README title "notebooklm-mcp-go": "A Golang rewrite of PleasePrompto/notebooklm-mcp ... All 16 tools from the original are implemented"). Claimed gains over the TypeScript original: ~100 ms startup vs 3-5 s, ~25 MB idle vs ~150 MB, 14 MB binary vs ~100 MB (README table; author's numbers, [unverified]).

## Access method

**Browser automation only** (rod drives the real NotebookLM web UI in Chrome). **No batchexecute, no RPC.** upstreams.md says "RPC / browser (many keywords) [unverified]"; confirmed: browser only. Behavior:

- A "session" = one rod page opened on a notebook URL; waits for a chat textarea (selectors `textarea.query-box-input`, a German aria-label variant `Feld für Anfragen`, then any `textarea`), restores saved sessionStorage, then for each question: clicks the textarea, types with human-like delays (WPM range, random delays; `StealthOptions` in config), presses Enter, polls the DOM for a new response element (selectors `.response-container`, `[data-test-id='chat-response']`, `.model-response`, `message-content`) until it stops growing or `BROWSER_TIMEOUT` (ms) elapses (`internal/session/session.go:67-262`).
- Locale-dependent selectors (a German aria-label) show how fragile UI scraping is; any UI language change breaks it.
- The notebook must be reachable by URL; the error text for a missing input says "ensure the notebook is publicly accessible" (`session.go:109`), i.e. the PleasePrompto flow expects a shared notebook link rather than enumerating the user's library via RPC. The tool keeps a **local notebook library** (name, URL, description, topics, use cases, tags, usage stats) in JSON (`internal/library/library.go`, `types.go`) because the browser flow cannot list notebooks.

## RPC catalog

None.

## Streaming/chat

Not a stream: waits for the DOM answer to stabilize, then returns the text. Progress messages ("Typing question...", "Waiting for answer...") are sent as MCP `notifications/progress` (`internal/mcp/server.go:236-242`). It sets `progressToken` to the **request id**, whereas the MCP spec says the token comes from `_meta.progressToken` supplied by the client — a spec deviation (clients may ignore it).
Citation handling: none (text only).

## Auth lifecycle

- `setup_auth`: launches rod non-headless with `UserDataDir = <data>/chrome_profile`, opens the NotebookLM sign-in URL, polls every 3 s up to 10 min until URL contains `notebooklm.google` and not `accounts.google.com`, then saves cookies for `notebooklm.google.com` + `accounts.google.com` as JSON `state.json` (`session.go:538-600`, `auth_manager.go`).
- State is treated as **expired after 24 hours of file age** regardless of cookie expiry (`auth_manager.go:81-99`); critical cookie names list: SID, HSID, SSID, APISID, SAPISID, OSID, `__Secure-OSID`, `__Secure-1PSID`, `__Secure-3PSID` (`:15-20`). `re_auth` tool switches accounts. `cleanup_data` tool wipes all stored data.
- Config knobs `AutoLoginEnabled`, `LoginEmail`, `LoginPassword`, `AutoLoginTimeoutMs` (`internal/config/config.go:49-53`, default off at `:122`): the design anticipates storing a Google password in config/env for unattended login. [Avoid; also not in our safety envelope.]
- No cookie-store decryption; no DPAPI/ABE.

## Reliability

- Rate-limit handling: the only one — it scans the answer text for phrases like "you've reached your daily limit"/"daily query limit"/"try again tomorrow" and returns `ErrRateLimit` with message "50 queries/day for free accounts" (`session.go:26,152-160`). **This is the only place in this batch that records a concrete consumer quota figure (50 chat queries per day on free accounts).** [Unverified against Google docs; consumer limits have changed over time.] Matters for us: browser fallback and RPC share the same account quota.
- Session manager: max sessions (default via `MAX_SESSIONS`), idle timeout (`SESSION_TIMEOUT` s), cleanup loop every `clamp(timeout/2, 60, 300)` s, evict oldest when full (`session.go:317-380,417-447`).
- No retries.

## Performance notes

Each session holds a browser tab; a shared browser (`ensureBrowser`) is reused across sessions. Startup claims above are unmeasured by us.

## MCP tools

Hand-rolled JSON-RPC 2.0 over stdio (`internal/mcp/server.go`): handles `initialize` (protocolVersion `2024-11-05`), `tools/list`, `tools/call`, `ping`; no SDK. 16 tools (`internal/tools/definitions.go:15-30`):

| Tool | Behavior |
|---|---|
| `ask_question` | Ask the selected/default notebook; optional `session_id` to continue; description is built dynamically from the library (`buildAskQuestionDescription`, `definitions.go:153`) so the agent sees which notebooks exist and what they cover |
| `list_sessions` / `close_session` / `reset_session` | Session stats / close by id / clear chat history keeping id |
| `get_health` | Auth state, active sessions, config |
| `setup_auth` / `re_auth` / `cleanup_data` | Login in a visible browser / switch account / delete all data |
| `add_notebook` | "PERMISSION REQUIRED — only when user explicitly asks" (description steers the LLM to confirm before adding) |
| `list_notebooks` / `get_notebook` / `select_notebook` / `update_notebook` / `remove_notebook` / `search_notebooks` / `get_library_stats` | Local library CRUD/search/stats |

**Tool profiles** (`internal/utils/settings.go:12-150`): `minimal` (ask_question, get_health, setup_auth), `standard` (+re_auth, sessions, list/select/add notebook), `full` (all 16), plus `disabled-tools` list, set with `notebooklm-mcp-go config set profile <p>` (`main.go:82-100`). Idea: let users shrink the tool list to save agent context.

## Tests and quality

No test files. No CI. Committed `.gocache`. Logger uses emoji `slog` messages.

## Versions

4 commits in 2026-03-11..12; no releases.

## Adopt (ideas)

1. **Tool profiles** (`minimal/standard/full`) and a `disabled-tools` list so agents with small context budgets can load fewer tool schemas (`settings.go:136-150`). We have ~40 candidate tools; profiles matter.
2. **Dynamic tool description from local state** (notebook library injected into `ask_question` description) so the model knows what each notebook covers (`definitions.go:153`). With RPC we can inject real notebook titles instead of a manual library.
3. Description wording that forces confirmation for mutating actions ("PERMISSION REQUIRED — only when user explicitly asks to add").
4. Multi-session concept (session id continues a conversation; reset keeps id) — maps to NotebookLM `conversation_id`.
5. Browser fallback shape: persistent profile dir, visible login, wait for URL change, then reuse headless. Keep behind the same Transport interface.
6. Concrete quota detection string list as one input to a rate-limit classifier (but prefer the in-band `UserDisplayableError` signal seen in sahaand's chat parser).

## Avoid

1. Committing build caches (`.gocache/`).
2. Putting login email/password in config (`config.go` AutoLogin fields).
3. UI-selector-based chat, locale-dependent selectors.
4. Progress token = request id (spec deviation).
5. 24-hour wall-clock expiry heuristics for cookies that live for months.
6. Hand-rolled MCP when the official go-sdk exists (protocol-version drift).

## Evidence

Read: `README.md` (head), `go.mod`, `main.go` (grep), `internal/session/session.go` (60-170, 538-600, grep), `internal/mcp/server.go` (grep + head), `internal/tools/definitions.go` (grep), `internal/utils/settings.go` (grep), `internal/auth/auth_manager.go` (grep), `internal/config/config.go` (grep), file listing, `git log`. No code reproduced.
