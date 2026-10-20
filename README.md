# sourcebolt

<!-- mcp-name: io.github.candidloom-hub/sourcebolt -->

[![Release](https://img.shields.io/github/v/release/candidloom-hub/sourcebolt)](https://github.com/candidloom-hub/sourcebolt/releases)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

**Let your AI assistant use your Google NotebookLM notebooks: one small, fast program, nothing else to install.**

## Install in 2 minutes

| You want to... | Go here |
|---|---|
| Run one command that installs and connects everything | The two commands below |
| Follow pictures for your AI app (Claude, Cursor, VS Code, Windsurf, Gemini CLI, ChatGPT) | [Step-by-step guides](docs/guides/index.md) |
| Let your AI app install it for you | [Let your AI install it](docs/install-with-ai.md) |
| Use npm, Docker or a manual setup (developers) | [Install](#install) and [Connect your AI app](docs/clients.md) |

**Windows** (open PowerShell, paste, press Enter):

```powershell
irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
```

**macOS and Linux** (open Terminal, paste, press Enter):

```bash
curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
```

The installer checks the download, installs sourcebolt for your user only, and then offers `sourcebolt setup`. Setup finds your AI apps, connects the ones you pick (keeping a backup of their settings), and opens a browser window so you can sign in to Google. Your password goes only to Google.

[Quick start](#quick-start) · [Install](#install) · [Connect your app](#connect-your-ai-app) · [Tools](#what-your-assistant-can-do) · [FAQ](#faq) · [Docs](docs/index.md)

> **Independent project.** sourcebolt is not made by, endorsed by or sponsored by Google. "NotebookLM" is a trademark of Google LLC. NotebookLM has no public API for personal accounts, so sourcebolt uses the same internal web calls the NotebookLM website makes. Read [Limits and risk](#limits-and-risk) before you rely on it.

## What it does

sourcebolt is an MCP server. MCP (Model Context Protocol) is the standard way AI apps such as Claude, Cursor, VS Code and Codex call outside tools. Once sourcebolt is connected, you can ask your assistant things like:

- "List my NotebookLM notebooks."
- "Add these three articles to my *Thesis* notebook."
- "Ask the *Thesis* notebook what the sources say about sample size, with citations."
- "Research recent work on battery recycling and import the best sources."

The assistant calls sourcebolt, sourcebolt talks to NotebookLM with your own Google session, and the answer comes back with citations from your sources.

<!-- Demo placeholder: add docs/assets/demo.gif (sourcebolt answering a question from a NotebookLM notebook inside an AI assistant) and show it here. -->

## Key numbers

Measured on the GitHub Actions runner (2 CPUs, AMD EPYC 7763), v0.2.0, medians. Method and raw data: [bench/RESULTS.md](bench/RESULTS.md) and [docs/performance.md](docs/performance.md).

| What | Result |
|---|---|
| Start-up time (start to ready) | 8.2 ms |
| Program size | about 10 MB, one file |
| Memory while idle | 16.2 MB |
| sourcebolt's own work per call, 100 notebooks, cached / not cached | 0.40 ms / 0.76 ms |
| Chat: first progress after Google's first chunk | +0.3 ms |
| A real `notebook_list` call to Google | 848 ms median (Google's time dominates; one laptop, 2026-10-14) |

## Quick start

1. **Install.** Download the file for your system from the [latest release](https://github.com/candidloom-hub/sourcebolt/releases) and put `sourcebolt` on your `PATH`. Step-by-step commands are in [Install](#install).
2. **Sign in.** Start Brave or Chrome with a debugging port, sign in to NotebookLM in that window, then run:

   ```bash
   sourcebolt login --attach http://127.0.0.1:9222
   ```

   You should see `Saved profile "default" for notebook.google.com (N notebooks visible).` The browser commands for each system are in [docs/auth.md](docs/auth.md#attach-flow-in-detail). From the release after v0.2.0, `sourcebolt setup` does steps 2 to 4 for you ([docs/getting-started.md](docs/getting-started.md#step-2-run-setup)).
3. **Check it.**

   ```bash
   sourcebolt doctor
   ```

   Every line should start with `ok`.
4. **Connect your AI app.** For Claude Code:

   ```bash
   claude mcp add sourcebolt -- sourcebolt serve
   ```

   After v0.2.0 you can instead install the Claude Code plugin, which adds the server and the sourcebolt skill (the program must still be on your `PATH`):

   ```text
   /plugin install sourcebolt --marketplace candidloom-hub/sourcebolt
   ```

   Other apps: [Connect your AI app](#connect-your-ai-app). One-click install links: [docs/install-links.md](docs/install-links.md).
5. **Ask a question.** In your app, type: *"Use sourcebolt to list my NotebookLM notebooks, then ask the first one for a one-paragraph summary with citations."*

The full walkthrough with expected output is in [docs/getting-started.md](docs/getting-started.md).

## Install

The current release is [v0.2.0](https://github.com/candidloom-hub/sourcebolt/releases/tag/v0.2.0). Not sure which way to pick? See **[docs/install.md](docs/install.md)**, or use the one-line installer (it verifies the download's checksum, needs no admin rights, and `-Uninstall` / `--uninstall` undoes it):

```powershell
irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex   # Windows PowerShell
```

```bash
curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh   # macOS, Linux
```

| Channel | Status | How |
|---|---|---|
| GitHub release binary | Available | Windows, macOS and Linux, on x64 and ARM64. See below. |
| `go install` | Available | Needs Go 1.27 or newer. |
| Docker (GHCR) | Available | `ghcr.io/candidloom-hub/sourcebolt:0.2.0`. Images are tagged by version only; there is no `latest` tag. |
| npm (`npx`) | Available | `npx -y sourcebolt serve` (needs Node.js 18+). In an app's MCP config: command `npx`, args `["-y", "sourcebolt", "serve"]`. |
| PyPI (`pipx`, `uvx`) | Not published yet | |

### Release binary

**Windows (PowerShell).** This puts `sourcebolt.exe` in a folder in your user profile and adds that folder to your `PATH`. For an ARM64 PC, replace `amd64` with `arm64`.

```powershell
$dir = "$env:LOCALAPPDATA\Programs\sourcebolt"
New-Item -ItemType Directory -Force $dir | Out-Null
Invoke-WebRequest https://github.com/candidloom-hub/sourcebolt/releases/download/v0.2.0/sourcebolt_0.2.0_windows_amd64.zip -OutFile "$env:TEMP\sourcebolt.zip"
Expand-Archive "$env:TEMP\sourcebolt.zip" -DestinationPath $dir -Force
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

Open a new terminal, then run `sourcebolt --version`. It prints `sourcebolt 0.2.0`.

**macOS and Linux (bash).** Set `OS` to `darwin` or `linux` and `ARCH` to `amd64` or `arm64`.

```bash
OS=linux ARCH=amd64
curl -fsSLO "https://github.com/candidloom-hub/sourcebolt/releases/download/v0.2.0/sourcebolt_0.2.0_${OS}_${ARCH}.tar.gz"
tar -xzf "sourcebolt_0.2.0_${OS}_${ARCH}.tar.gz" sourcebolt
mkdir -p ~/.local/bin && mv sourcebolt ~/.local/bin/
sourcebolt --version
```

If `sourcebolt` is not found, add `~/.local/bin` to your `PATH`. Each release has a `checksums.txt` with a keyless cosign signature and a CycloneDX SBOM (software bill of materials) per archive; [docs/security.md](docs/security.md#verify-a-download) shows how to check them.

### Go

```bash
go install github.com/candidloom-hub/sourcebolt/cmd/sourcebolt@latest
```

### Docker

```bash
docker run -i --rm -v sourcebolt-config:/config ghcr.io/candidloom-hub/sourcebolt:0.2.0
```

The image has no OS keyring, so the profile key is kept in a file inside the `/config` volume. Protect that volume like a password. The `0.2.0-browser` tag adds a browser for the fallback path. Signing in inside a container: [docs/auth.md](docs/auth.md#docker).

## Connect your AI app

Sign in first. All apps start the same command: `sourcebolt serve`. If `sourcebolt` is not on your `PATH`, use the full path to the program instead (on Windows, for example `C:\\Users\\you\\AppData\\Local\\Programs\\sourcebolt\\sourcebolt.exe` inside JSON).

| App | Works locally? | Setup |
|---|---|---|
| Claude Code | Yes | `claude mcp add sourcebolt -- sourcebolt serve`, or after v0.2.0 the plugin: `/plugin install sourcebolt --marketplace candidloom-hub/sourcebolt` |
| Claude Desktop | Yes | Add to `claude_desktop_config.json` (below) |
| Cursor | Yes | Add to `~/.cursor/mcp.json` (below) |
| VS Code (Copilot agent mode) | Yes | `code --add-mcp '{"name":"sourcebolt","type":"stdio","command":"sourcebolt","args":["serve"]}'` |
| Windsurf | Yes | Add to its `mcp_config.json` (below) |
| Codex CLI and IDE | Yes | `codex mcp add sourcebolt -- sourcebolt serve` |
| Gemini CLI | Yes | `gemini mcp add -s user sourcebolt sourcebolt serve` |
| ChatGPT desktop app | Yes, as a local (STDIO) server | Settings, MCP servers; it shares the Codex configuration |
| ChatGPT on the web | No | Needs a public HTTPS server. sourcebolt runs only on your computer, by design. |

The JSON format used by Claude Desktop, Cursor and Windsurf:

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

File locations, VS Code JSON, Codex TOML, the HTTP mode and which snippets were tested: [docs/clients.md](docs/clients.md).

Small tool budget? Add `"--tools", "minimal"` to `args` to expose only the 8 core tools.

## What your assistant can do

Grouped by task. Every tool, option and state is in the generated reference, [docs/tools.md](docs/tools.md).

| Task | Tools |
|---|---|
| Find and open notebooks | `notebook_list`, `notebook_open`, `notebook_create`, `notebook_update` |
| Add and read sources (web pages, YouTube, pasted text) | `source_add`, `source_list`, `source_read`, `source_update` |
| Ask questions with citations | `chat_ask`, `chat_status`, `chat_history`, `chat_configure`, `suggest` |
| Find new sources on the web | `research_start`, `research_status`, `research_import` |
| Keep notes | `note_list`, `note_save` |
| Studio (audio, reports, flashcards, ...) | `studio_list`, `studio_generate` |
| Sharing, labels, account | `share_status`, `share_update`, `label_list`, `label_update`, `account_settings` |
| Clean up | `delete` (always previews first), `job_cancel` |

Some options are not available yet, for example Drive and file uploads and the Studio option values no live capture has confirmed. They return a clear `UNSUPPORTED_BY_TRANSPORT` error instead of failing silently. [SKILL.md](SKILL.md) tells an agent which tool to use when.

## How it works

1. **Direct calls first.** sourcebolt sends NotebookLM's own internal web requests (called RPC, remote procedure calls) over HTTP/2 with your session cookies. This is the fast path.
2. **Browser fallback.** If Google rejects the direct request, sourcebolt can repeat it inside a headless Chrome-family browser on a separate profile. It needs Brave, Chrome or Edge installed (or the `-browser` Docker image).
3. **Page fallback.** If Google changes the format of the chat or add-source request, sourcebolt can type into the NotebookLM page itself and read the answer (the "DOM" fallback, named after the page structure it drives).

Turn the browser paths off with `--no-browser-fallback`. `server_info` shows which path served the last call. Details: [docs/auth.md](docs/auth.md#browser-fallback).

## FAQ

**Is this an official Google product?** No. It is an independent open-source project.

**Does sourcebolt see my Google password?** No. You sign in yourself, in a normal browser window. sourcebolt keeps only the session cookies, encrypted on your disk.

**Can the AI read my cookies?** No tool accepts or returns cookies or passwords. Sign-in works only from the command line.

**Does it send my data anywhere else?** No. It talks only to NotebookLM. URLs you add as sources are fetched by Google, not by your computer.

**Is it free?** Yes. Apache-2.0, no paid tier.

More answers: [docs/faq.md](docs/faq.md). Something broken? [docs/troubleshooting.md](docs/troubleshooting.md).

## Limits and risk

- **Unofficial.** Google can change NotebookLM's internal calls at any time. Tools may break until sourcebolt is updated.
- **Terms of Service.** Automating a Google web app with your session may break Google's terms and could lead to rate limits or account action. You use sourcebolt at your own risk; there is no warranty (see the license).
- **Google's quotas apply.** Limits on notebooks, sources, chats and Studio are set by Google per account. sourcebolt reports them as errors and does not bypass them.
- **Try it safely first.** Use a test notebook with public content while you evaluate it.

## Documentation

Start at [docs/index.md](docs/index.md). Highlights:

- [Step-by-step guides per app](docs/guides/index.md) · [Let your AI install it](docs/install-with-ai.md)
- [Getting started](docs/getting-started.md) · [Connect your AI app](docs/clients.md) · [Troubleshooting](docs/troubleshooting.md) · [FAQ](docs/faq.md)
- [Tool reference](docs/tools.md) · [Sign-in and storage](docs/auth.md) · [Security](docs/security.md) · [Privacy](docs/privacy.md) · [Performance](docs/performance.md)
- [One-click install links](docs/install-links.md) · [Registries and directories](docs/registry.md)
- [Why sourcebolt](docs/why-sourcebolt.md) · [Comparison with other NotebookLM servers](docs/comparison.md) · [Changelog](CHANGELOG.md)

## Contributing

Bug reports and pull requests are welcome on [GitHub Issues](https://github.com/candidloom-hub/sourcebolt/issues). Before you open one:

- Run `sourcebolt --version` and `sourcebolt doctor` and include the output. Remove any cookies or tokens first.
- For code changes, run `go vet ./...` and `go test ./...`.
- `docs/tools.md` is generated. Change the tool descriptions in code, then run `go run ./tools/gendocs > docs/tools.md`.
- Report security problems privately, as described in [docs/security.md](docs/security.md#reporting-a-vulnerability).

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

## Credits

sourcebolt is written from scratch. No source code from other projects was copied. Designs and knowledge of the NotebookLM wire format were learned, with thanks, from these MIT- or Apache-2.0-licensed projects (see [NOTICE](NOTICE) and `research/notes/`). Projects with no license or a copyleft license were read for behavior only, and nothing from them was used.

| Project | License | What we learned from it |
|---|---|---|
| [teng-lin/notebooklm-py](https://github.com/teng-lin/notebooklm-py) | MIT | RPC ids, payload layouts, source and chat flows |
| [tmc/nlm](https://github.com/tmc/nlm) | MIT | Go wire-protocol modeling and lossless round-trip testing |
| [jacob-bd/gemini-notebook-mcp-cli](https://github.com/jacob-bd/gemini-notebook-mcp-cli) | MIT | MCP tool surface, internal API reference |
| [LocalKinAI/notebooklm-go](https://github.com/LocalKinAI/notebooklm-go) | Apache-2.0 | Go client and auth flow ideas |
