# sourcebolt

A fast MCP server for Google NotebookLM, written in Go. One static binary, no runtime, starts in about 10 ms.

> **Independent project.** sourcebolt is not affiliated with, endorsed by or sponsored by Google. "NotebookLM" is a trademark of Google LLC. NotebookLM has no public consumer API; sourcebolt uses the same internal web calls the NotebookLM site makes. See [Limits and risk](#limits-and-risk) before you rely on it.

Status: pre-release. No version has been published yet (see [Install](#install)).

## What it does

Your MCP client (Claude Code, Claude Desktop, Cursor, Codex, ...) gets tools to work with your own NotebookLM notebooks: list and open notebooks, add sources, ask questions with citations, run research, keep notes, manage sharing, and list Studio artifacts. The full, generated reference with every parameter is in [docs/tools.md](docs/tools.md).

### Tools

31 tools with `--tools all`, 30 by default, 8 with `--tools minimal`.

| Group | Tools | State |
|---|---|---|
| Info | `server_info` | works |
| Notebooks | `notebook_list`, `notebook_open`, `notebook_create`, `notebook_update` | works |
| Sources | `source_add` (URLs, YouTube, pasted text), `source_list`, `source_read`, `source_update` (rename, freshness check) | works; Drive and file upload, source guide, refresh: coming soon |
| Chat | `chat_ask`, `chat_status`, `chat_history`, `chat_configure`, `chat_ask_many` (batch, opt-in) | works; `suggest` followups: coming soon |
| Research | `research_start`, `research_status`, `research_import` | works; `discover` mode: coming soon |
| Notes | `note_list`, `note_save` | works; save a chat turn as a note: coming soon |
| Sharing | `share_status`, `share_update` | works, request shapes not yet live-verified; view level and artifact sharing: coming soon |
| Labels | `label_list`, `label_update` | rename, emoji, add/remove sources; create and auto-label: coming soon |
| Studio | `studio_list`, `studio_generate` | list works; **generation is coming soon** (needs a live capture; it checks quota, then returns `UNSUPPORTED_BY_TRANSPORT`) |
| Account | `account_settings`, `auth_login`, `auth_logout` | quota read works; setting the language is not live-verified; `auth_*` over MCP only point you to the CLI by design |
| Delete | `delete` | notebooks only, preview first, `confirm:true` to act; sources, notes, artifacts, chat history and labels: coming soon |
| Jobs | `job_cancel` | works |

"Coming soon" means the tool or option exists but returns an `UNSUPPORTED_BY_TRANSPORT` error until the wire format is pinned by a live capture (see [docs/live-capture.md](docs/live-capture.md)).

Other behavior: pasted/URL sources are checked against private-network addresses before they are sent; destructive tools preview before they act; long chat answers can run detached (`wait=false`, then `chat_status`); progress notifications are sent for long calls.

## Install

No release exists yet. Every channel below becomes available after the first release; the only way to get sourcebolt today is to build from source.

| Channel | Command | Availability |
|---|---|---|
| GitHub release binary | download the archive for your OS/arch from the Releases page, check `checksums.txt` | available after first release |
| Go | `go install github.com/candidloom-hub/sourcebolt/cmd/sourcebolt@latest` | needs the repo to be public; works from a local checkout today with `go build ./cmd/sourcebolt` |
| Docker | `docker run -i --rm -v sourcebolt-config:/config ghcr.io/candidloom-hub/sourcebolt` (browser fallback image: tag `<version>-browser`) | available after first release |
| npm | `npx sourcebolt serve` | available after first release |
| PyPI | `pipx run sourcebolt serve` or `uvx sourcebolt serve` | available after first release |

Build from source (Go 1.27):

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/sourcebolt ./cmd/sourcebolt
bin/sourcebolt --version
```

The Docker image has no OS keyring, so the profile key is stored in a `0600` file inside the `/config` volume. Sign in on the host and mount the config, or use `login --import -` inside the container.

## Connect your client

Sign in first (next section), then register the server. `sourcebolt` must be on your `PATH`; otherwise use the full path to the binary.

**Claude Code**

```bash
claude mcp add sourcebolt -- sourcebolt serve
```

**Claude Desktop** (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

**Cursor** (`.cursor/mcp.json` or `~/.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

**Codex** (`~/.codex/config.toml`)

```toml
[mcp_servers.sourcebolt]
command = "sourcebolt"
args = ["serve"]
```

Add `"--tools", "minimal"` to `args` for hosts with small tool budgets. Only the Claude Code command was run against a real CLI while writing this; the other snippets follow each client's documented format and were not tested against the clients.

Streamable HTTP (loopback only, bearer token required):

```bash
sourcebolt serve --http 127.0.0.1:8787 --token-file token.txt   # token: 16+ characters
```

## Sign in

sourcebolt needs your Google session cookies. It never asks for your password and no tool types credentials. Details and storage layout: [docs/auth.md](docs/auth.md).

### Recommended: `login --attach`

Start a Chromium-based browser (Brave or Chrome) with a debugging port and a dedicated profile directory, sign in to NotebookLM in that window yourself, then attach. Use a dedicated `--user-data-dir` under the sourcebolt config dir: Chrome refuses remote debugging on its default profile, and a dedicated one keeps your daily browser out of it.

Windows (PowerShell, Brave; for Chrome use `C:\Program Files\Google\Chrome\Application\chrome.exe`):

```powershell
& "C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe" --remote-debugging-port=9222 --user-data-dir="$env:APPDATA\sourcebolt\browser\attach"
```

macOS:

```bash
"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser" --remote-debugging-port=9222 --user-data-dir="$HOME/Library/Application Support/sourcebolt/browser/attach"
```

Linux:

```bash
brave-browser --remote-debugging-port=9222 --user-data-dir="$HOME/.config/sourcebolt/browser/attach"
```

Sign in at https://notebook.google.com in that window, open NotebookLM, then in another terminal:

```bash
sourcebolt login --attach http://127.0.0.1:9222
sourcebolt status
sourcebolt doctor
```

`login` makes one real call (list notebooks) to check the cookies and saves them only if it works. Close the debugging browser afterwards; the debugging port gives control of that browser to anything on your machine that can reach it.

### Alternatives

- `sourcebolt login --browser` opens a browser on a dedicated profile and waits for you to sign in. Google may block it with a "this browser may not be secure" page, because a browser started by automation exposes `navigator.webdriver`. That happened in our own test; prefer `--attach`.
- `sourcebolt login --import cookies.txt` imports a Netscape `cookies.txt`, a JSON export or a `Cookie:` header (`--format netscape|json|header`, `-` reads stdin). Export every google.com cookie, including the plain `SID`, `HSID` and `APISID` ones.

Cookies are stored AES-256-GCM encrypted in `<config dir>/profiles/<name>.profile`. The 32-byte key lives in the OS keyring (Windows Credential Manager, macOS Keychain, Linux Secret Service), or in a `0600` key file when no keyring is available. `sourcebolt logout` deletes a profile.

## How requests are sent

1. **Internal RPC first.** The Go binary calls NotebookLM's web endpoint (`batchexecute`) directly over HTTP/2 with your cookies. This is the fast path.
2. **Headless browser fallback.** If the RPC path fails in a way the fallback policy allows (for example Google rejects the direct request), the call is replayed through a headless Chromium-based browser on a dedicated profile under the config dir, started lazily on first use and stopped when the server exits. It needs a Chromium-based browser installed (or the `-browser` Docker image).
3. `--no-browser-fallback` (or `NOTEBOOKLM_BROWSER_FALLBACK=0`) turns the fallback off. `server_info` reports which transport served the last call.

Environment: `NOTEBOOKLM_PROFILE`, `NOTEBOOKLM_HOST`, `NOTEBOOKLM_CONFIG_DIR`, `NOTEBOOKLM_RPC_IDS`, `NOTEBOOKLM_BROWSER_FALLBACK`, `SOURCEBOLT_BROWSER`.

Command-line summary (`sourcebolt --help`): `serve`, `call <tool> '<json>'` (run any MCP tool in-process, handy for scripts), `login`, `logout`, `status`, `doctor`, `--version`.

## Performance

From [bench/RESULTS.md](bench/RESULTS.md), measured 2026-10-14 on Windows 11, Intel Core i7-12650H (16 threads), 16 GB RAM, Go 1.27.0 windows/amd64, a laptop with other programs running. Variance there is high (20 to 50 percent on millisecond benchmarks); read these as order of magnitude. All data is synthetic and no Google traffic was involved.

| Metric | Result |
|---|---|
| Cold start to `initialize` | p50 10.3 ms, p95 12.6 ms |
| Memory | 12.4 MB idle binary; 19.9 MB peak in-process after listing 100 notebooks (includes a test client and fake server) |
| Decode a 40 KB notebook-list response | 475 us median, 93 allocations |
| Tool call overhead excluding Google (100 notebooks) | cached p50 1.6 ms; uncached p50 9.6 ms (misses the 2 ms target; an optimization is tracked) |
| Connection reuse | 100% after the first request, one TLS handshake, HTTP/2 |
| Binary size | 10.06 MB stripped |

Real calls are dominated by Google: a live `notebook_list` round trip measured p50 848 ms, p95 1.04 s (10 calls, one account). The browser fallback and streamed-chat timings have not been measured.

## Limits and risk

- **Unofficial.** NotebookLM has no stable public API for consumer accounts. sourcebolt uses internal calls that Google can change at any time, which will break tools until the wire format is re-captured.
- **Terms of Service.** Automating a Google web app with your session may violate Google's terms and could lead to rate limiting or account action. You use sourcebolt at your own risk; the authors give no warranty (see the license).
- **Quotas are Google's.** Notebook, source, chat and Studio limits are enforced by Google per account. Quota failures are returned as errors; sourcebolt does not bypass them.
- Requires a Google account that can open NotebookLM. The default host is `notebook.google.com`.
- Use a throwaway notebook with public content while you evaluate it.

## Documentation

- [docs/tools.md](docs/tools.md) - generated tool reference
- [docs/auth.md](docs/auth.md) - sign-in flows, storage, logout, doctor
- [docs/security.md](docs/security.md) - threat model and reporting vulnerabilities
- [SKILL.md](SKILL.md) - agent skill: which tool to use when
- [CHANGELOG.md](CHANGELOG.md)

## Credits

sourcebolt is written from scratch. No source code from other projects has been copied; designs and wire-format knowledge were learned, with thanks, from these MIT- or Apache-2.0-licensed projects (see [NOTICE](NOTICE) and `research/notes/`). Projects with no license or a copyleft license were read for behavior only and nothing from them was used.

| Project | License | What we learned from it |
|---|---|---|
| [teng-lin/notebooklm-py](https://github.com/teng-lin/notebooklm-py) | MIT | RPC ids, payload layouts, source and chat flows |
| [tmc/nlm](https://github.com/tmc/nlm) | MIT | Go wire-protocol modeling and lossless round-trip testing |
| [jacob-bd/gemini-notebook-mcp-cli](https://github.com/jacob-bd/gemini-notebook-mcp-cli) | MIT | MCP tool surface, internal API reference |
| [LocalKinAI/notebooklm-go](https://github.com/LocalKinAI/notebooklm-go) | Apache-2.0 | Go client and auth flow ideas |

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
