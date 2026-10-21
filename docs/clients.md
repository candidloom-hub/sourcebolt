# Connect your AI app

sourcebolt is a local MCP server. Your AI app starts it as a background program with the command `sourcebolt serve` and talks to it over standard input and output ("stdio"). Nothing listens on the network unless you turn on [HTTP mode](#http-mode).

One-click install buttons for several apps: [install-links.md](install-links.md).

## The easy way: `sourcebolt setup`

From the release after v0.2.0, one command does the work on this page for you:

```bash
sourcebolt setup
```

It finds the apps below that are installed for you, asks `Connect sourcebolt to <app>? [Y/n]` for each, and adds the `sourcebolt` entry with the full path to the program, so `PATH` problems cannot happen. Then it signs you in (unless you already are) and runs `sourcebolt doctor`. Restart each app it names at the end.

What it changes, per app:

| App | How setup connects it | File it changes (backup next to it) |
|---|---|---|
| Claude Desktop | edits the file | Windows `%APPDATA%\Claude\claude_desktop_config.json`, macOS `~/Library/Application Support/Claude/claude_desktop_config.json`, Linux `~/.config/Claude/claude_desktop_config.json` |
| Claude Code | runs `claude mcp add --scope user ...` if `claude` is on your `PATH`, otherwise edits the file | `~/.claude.json` |
| Cursor | edits the file | `~/.cursor/mcp.json` |
| VS Code | runs `code --add-mcp ...` if `code` is on your `PATH`, otherwise edits the file | Windows `%APPDATA%\Code\User\mcp.json`, macOS `~/Library/Application Support/Code/User/mcp.json`, Linux `~/.config/Code/User/mcp.json` |
| Windsurf | edits the file | `%APPDATA%\devin\mcp_config.json` (Windows) or `~/.config/devin/mcp_config.json` when that folder exists, otherwise the older `~/.codeium/windsurf/mcp_config.json` |
| Gemini CLI | edits the file | `~/.gemini/settings.json` |

Before any change, setup copies the file to `<file>.sourcebolt-backup-<date>-<time>`. It adds only the `sourcebolt` entry and keeps everything else in the file as it was. It asks before it replaces an existing `sourcebolt` entry. A file with comments in it is left alone; setup tells you to add the entry by hand.

- `sourcebolt setup --dry-run` shows what would change and changes nothing.
- `sourcebolt setup --client cursor` connects only that app (repeat `--client` for more): `claude-desktop`, `claude-code`, `cursor`, `vscode`, `windsurf`, `gemini`.
- `sourcebolt setup --yes` answers yes to every question.
- `sourcebolt setup --undo` puts each file back from its newest backup. For `~/.claude.json`, which Claude Code keeps rewriting, it puts back only the `sourcebolt` entry and leaves the rest of the file alone.

The Linux path for Claude Desktop, the VS Code `mcp.json` path and the older Windsurf path are not stated on the apps' own pages; setup prefers each app's CLI where one exists. If your app keeps its settings elsewhere, use the manual steps below.

## Manual setup

Before you start:

1. Install sourcebolt and sign in ([getting-started.md](getting-started.md)).
2. Run `sourcebolt doctor`. Every check should say `ok`.

### If `sourcebolt` is not on your PATH

Apps do not always see the same `PATH` as your terminal. If the app says it cannot start the server, replace `"sourcebolt"` with the full path to the program. Find it with:

PowerShell:

```powershell
(Get-Command sourcebolt).Source
```

bash:

```bash
command -v sourcebolt
```

In JSON files, write Windows backslashes twice, for example `"C:\\Users\\you\\AppData\\Local\\Programs\\sourcebolt\\sourcebolt.exe"`.

### Fewer tools for small budgets

Some apps limit how many tools they show the model. Add `--tools minimal` to expose only the 8 core tools (`server_info`, `notebook_list`, `notebook_open`, `source_add`, `source_read`, `chat_ask`, `studio_list`, `studio_generate`). `--tools all` adds the opt-in batch tool `chat_ask_many`. Example: `"args": ["serve", "--tools", "minimal"]`.

## Summary

| App | Local server support | Tested by us |
|---|---|---|
| [Claude Code](#claude-code) | Yes | Yes |
| [Claude Desktop](#claude-desktop) | Yes | No, follows the app's documented format |
| [Cursor](#cursor) | Yes | No, follows the app's documented format |
| [VS Code](#vs-code) | Yes (Copilot agent mode) | No, follows the app's documented format |
| [Windsurf](#windsurf) | Yes | No, follows the app's documented format |
| [Codex](#codex) | Yes | No, follows the app's documented format |
| [Gemini CLI](#gemini-cli) | Yes | No, follows the app's documented format |
| [ChatGPT](#chatgpt) | Desktop app only | No |

"Tested by us" means the setup command was run against the real app while writing these docs. The other snippets follow each app's own documentation, linked in its section and checked on 2026-10-20. If one does not work for you, please [open an issue](https://github.com/candidloom-hub/sourcebolt/issues).

## Claude Code

```bash
claude mcp add sourcebolt -- sourcebolt serve
```

This adds sourcebolt to the current project. To make it available in every project, add `--scope user`:

```bash
claude mcp add --scope user sourcebolt -- sourcebolt serve
```

Check it with `claude mcp list`.

**Plugin (after v0.2.0).** Inside Claude Code, the sourcebolt plugin adds the MCP server and the sourcebolt skill in one step. It needs Claude Code 2.1.275 or later, and the `sourcebolt` program must still be on your `PATH`:

```text
/plugin install sourcebolt --marketplace candidloom-hub/sourcebolt
```

Or add the marketplace first, then install: `/plugin marketplace add candidloom-hub/sourcebolt`, then `/plugin install sourcebolt@sourcebolt`. The plugin and marketplace pass `claude plugin validate --strict`; the install itself has not been run yet.

## Claude Desktop

1. Open Claude Desktop, then **Settings**, **Developer**, **Edit Config**.
2. The file is `%APPDATA%\Claude\claude_desktop_config.json` on Windows and `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS.
3. Add sourcebolt. If the file already has an `mcpServers` block, add only the `"sourcebolt"` entry inside it.

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

4. Quit Claude Desktop completely and start it again.

If the server does not appear, check the logs: `%APPDATA%\Claude\logs\mcp-server-sourcebolt.log` (Windows) or `~/Library/Logs/Claude/mcp-server-sourcebolt.log` (macOS). Source: [Connect to local MCP servers](https://modelcontextprotocol.io/docs/develop/connect-local-servers).

## Cursor

Add this to `~/.cursor/mcp.json` (all projects) or `.cursor/mcp.json` in a project:

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

Then open Cursor's MCP settings and check that sourcebolt is enabled.

## VS Code

VS Code uses MCP servers in Copilot agent mode. Add this to `.vscode/mcp.json` in your workspace. Note that VS Code uses the key `servers`, not `mcpServers`:

```json
{
  "servers": {
    "sourcebolt": { "type": "stdio", "command": "sourcebolt", "args": ["serve"] }
  }
}
```

Or add it to your user profile from a bash terminal:

```bash
code --add-mcp '{"name":"sourcebolt","type":"stdio","command":"sourcebolt","args":["serve"]}'
```

From PowerShell, escape the inner quotes:

```powershell
code --add-mcp '{\"name\":\"sourcebolt\",\"type\":\"stdio\",\"command\":\"sourcebolt\",\"args\":[\"serve\"]}'
```

Source: [VS Code MCP configuration reference](https://code.visualstudio.com/docs/agents/reference/mcp-configuration).

## Windsurf

Add sourcebolt to Windsurf's MCP config file in the usual `mcpServers` format:

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

The file location depends on your version. Older Windsurf versions use `~/.codeium/windsurf/mcp_config.json`. On 2026-10-20 Windsurf's MCP docs page redirected to the [Devin desktop MCP docs](https://docs.devin.ai/desktop/cascade/mcp), which name `~/.config/devin/mcp_config.json` (macOS, Linux) and `%APPDATA%\devin\mcp_config.json` (Windows). The safest way is to open the raw config from the app's MCP settings and paste the entry there.

## Codex

Codex CLI and the Codex IDE extension share one config.

```bash
codex mcp add sourcebolt -- sourcebolt serve
```

Or add this to `~/.codex/config.toml`:

```toml
[mcp_servers.sourcebolt]
command = "sourcebolt"
args = ["serve"]
```

Source: [Codex MCP docs](https://learn.chatgpt.com/docs/extend/mcp?surface=cli).

## Gemini CLI

```bash
gemini mcp add -s user sourcebolt sourcebolt serve
```

Or add this to `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

Source: [Gemini CLI MCP servers](https://geminicli.com/docs/tools/mcp-server/). Google's Gemini CLI pages say it was replaced by Antigravity CLI for some account types in 2026; check which one you have.

## ChatGPT

- **ChatGPT desktop app: yes, tested.** Add it under **Settings**, **Plugins**, the **MCPs** tab, **Add** (type STDIO, the full path to `sourcebolt`, argument `serve`). The app shares this configuration with Codex, so a server added to Codex appears there too. Step by step: [ChatGPT desktop guide](guides/chatgpt-desktop.md).
- **ChatGPT on the web: no.** The web version connects only to remote servers on a public HTTPS address. sourcebolt runs on your own computer on purpose: a hosted server would have to hold your Google cookies. There are tunnel tools that expose a local server to the internet, but we do not recommend them for a server that holds your Google session.

Source: [Codex and ChatGPT MCP docs](https://learn.chatgpt.com/docs/extend/mcp?surface=cli).

## Docker

Any app above can start the Docker image instead of the program. Sign in first ([auth.md](auth.md#docker)), then use:

```json
{
  "mcpServers": {
    "sourcebolt": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-v", "sourcebolt-config:/config", "ghcr.io/candidloom-hub/sourcebolt:0.3.3"]
    }
  }
}
```

### Docker without a login step

The container can also sign in from a cookie export you hand it, so you never run `login` ([auth.md](auth.md#sign-in-with-an-environment-variable-containers)). Mount the export read-only as a file and point `NOTEBOOKLM_COOKIES_FILE` at it (use the full path of your file):

```json
{
  "mcpServers": {
    "sourcebolt": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-v", "/home/you/.sourcebolt-secrets/cookies.txt:/run/secrets/notebooklm_cookies:ro",
        "-e", "NOTEBOOKLM_COOKIES_FILE=/run/secrets/notebooklm_cookies",
        "ghcr.io/candidloom-hub/sourcebolt:0.3.3"
      ]
    }
  }
}
```

Or keep the variable in an env file that only you can read and pass `--env-file`:

```json
"args": ["run", "-i", "--rm", "--env-file", "/home/you/.sourcebolt-secrets/sourcebolt.env", "ghcr.io/candidloom-hub/sourcebolt:0.3.3"]
```

where `sourcebolt.env` holds one line, `NOTEBOOKLM_COOKIES=SID=...; __Secure-1PSIDTS=...; OSID=...` (the `Cookie:` header form; an env file cannot hold a multi-line `cookies.txt`).

Treat the export like a password. The file form is safer: a value in `NOTEBOOKLM_COOKIES` (from `-e` or `--env-file`) is visible to `docker inspect`. The cookies stay in memory and are never saved in the container; when Google expires the session, export fresh cookies.

## HTTP mode

Some setups prefer a server that keeps running. sourcebolt can serve MCP over "streamable HTTP" on your own computer only:

1. Create a token file with a long random secret (at least 16 characters).

   PowerShell:

   ```powershell
   $b = New-Object byte[] 32; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
   [Convert]::ToBase64String($b) | Set-Content -Encoding ascii token.txt
   ```

   bash:

   ```bash
   openssl rand -base64 32 > token.txt
   ```

2. Start the server:

   ```bash
   sourcebolt serve --http 127.0.0.1:8787 --token-file token.txt
   ```

3. Point your app at `http://127.0.0.1:8787` and send the token as `Authorization: Bearer <token>`.

Rules: only loopback addresses (`127.0.0.1`, `localhost`) are accepted; requests from other web origins are refused; the connection is plain HTTP, not TLS, so never expose it to a network. Anyone on your computer who can read the token file can use your notebooks.
