# One-click install links

Ready-made install links and commands for MCP clients. Sign in first (see [auth.md](auth.md)); none of these steps can sign in for you.

Two ways to run sourcebolt:

- **Binary on PATH.** Install a release binary (or `go install`) so that `sourcebolt` runs from a terminal.
- **Docker.** Uses the pinned image `ghcr.io/candidloom-hub/sourcebolt:0.3.2` with a named volume for the encrypted profile. Sign in into that volume once, for example with a cookie export:

  ```bash
  docker run -i --rm -v sourcebolt-config:/config ghcr.io/candidloom-hub/sourcebolt:0.3.2 login --import - < cookies.txt
  ```

The npm package is published: with Node.js 18+, any app can run sourcebolt as command `npx`, args `["-y", "sourcebolt", "serve"]` instead of the binary. PyPI (`uvx sourcebolt`) is not published yet.

## Notes on the links

- `vscode:` and `cursor:` links open the app when clicked in a browser or passed to `xdg-open`, `open` or `start`. GitHub does not render links with these schemes as clickable, so a README should show them as text to copy, or link a web page that redirects to them.
- Third-party guides use `https://vscode.dev/redirect/mcp/install?name=...&config=...` as a web wrapper for the VS Code link. VS Code's own docs do not describe it, so it is not generated here.
- The PowerShell lines use the stop-parsing token `--%`, so the JSON reaches `code` unchanged in Windows PowerShell 5.1 and PowerShell 7. That quoting was checked with a stand-in for `code.cmd` that prints its arguments; `code --add-mcp` itself was not run, because it changes the VS Code user configuration.
- The links were decoded back to their configs by `go test ./tools/installlinks`, and the VS Code and Cursor URLs were compared with what JavaScript's `encodeURIComponent(JSON.stringify(...))` and base64 produce. They were not clicked in VS Code or Cursor.
- To limit the tool count for a small model, add `"--tools", "minimal"` after `serve`. The links below use the default tool set.

Formats, from each vendor's docs (checked 2026-10-20):

- VS Code: `vscode:mcp/install?{encodeURIComponent(JSON.stringify(config))}`, where the config has `name` plus the server fields ([MCP developer guide](https://code.visualstudio.com/docs/copilot/guides/mcp-developer-guide)); `code --add-mcp '<json>'` ([MCP configuration reference](https://code.visualstudio.com/docs/copilot/reference/mcp-configuration)).
- Cursor: `cursor://anysphere.cursor-deeplink/mcp/install?name=<name>&config=<base64 of the server config JSON>` ([Cursor install links](https://cursor.com/docs/context/mcp/install-links)).

<!-- BEGIN GENERATED: go run ./tools/installlinks -->

### Binary on PATH

Server config: `{"command":"sourcebolt","args":["serve"]}`

VS Code install link:

```text
vscode:mcp/install?%7B%22name%22%3A%22sourcebolt%22%2C%22type%22%3A%22stdio%22%2C%22command%22%3A%22sourcebolt%22%2C%22args%22%3A%5B%22serve%22%5D%7D
```

VS Code Insiders install link:

```text
vscode-insiders:mcp/install?%7B%22name%22%3A%22sourcebolt%22%2C%22type%22%3A%22stdio%22%2C%22command%22%3A%22sourcebolt%22%2C%22args%22%3A%5B%22serve%22%5D%7D
```

VS Code from a terminal (bash, zsh):

```bash
code --add-mcp '{"name":"sourcebolt","type":"stdio","command":"sourcebolt","args":["serve"]}'
```

VS Code from PowerShell:

```powershell
code --% --add-mcp "{\"name\":\"sourcebolt\",\"type\":\"stdio\",\"command\":\"sourcebolt\",\"args\":[\"serve\"]}"
```

Cursor install link:

```text
cursor://anysphere.cursor-deeplink/mcp/install?name=sourcebolt&config=eyJjb21tYW5kIjoic291cmNlYm9sdCIsImFyZ3MiOlsic2VydmUiXX0=
```

### Docker

Server config: `{"command":"docker","args":["run","-i","--rm","-v","sourcebolt-config:/config","ghcr.io/candidloom-hub/sourcebolt:0.3.2","serve"]}`

VS Code install link:

```text
vscode:mcp/install?%7B%22name%22%3A%22sourcebolt%22%2C%22type%22%3A%22stdio%22%2C%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-v%22%2C%22sourcebolt-config%3A%2Fconfig%22%2C%22ghcr.io%2Fcandidloom-hub%2Fsourcebolt%3A0.3.2%22%2C%22serve%22%5D%7D
```

VS Code Insiders install link:

```text
vscode-insiders:mcp/install?%7B%22name%22%3A%22sourcebolt%22%2C%22type%22%3A%22stdio%22%2C%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-v%22%2C%22sourcebolt-config%3A%2Fconfig%22%2C%22ghcr.io%2Fcandidloom-hub%2Fsourcebolt%3A0.3.2%22%2C%22serve%22%5D%7D
```

VS Code from a terminal (bash, zsh):

```bash
code --add-mcp '{"name":"sourcebolt","type":"stdio","command":"docker","args":["run","-i","--rm","-v","sourcebolt-config:/config","ghcr.io/candidloom-hub/sourcebolt:0.3.2","serve"]}'
```

VS Code from PowerShell:

```powershell
code --% --add-mcp "{\"name\":\"sourcebolt\",\"type\":\"stdio\",\"command\":\"docker\",\"args\":[\"run\",\"-i\",\"--rm\",\"-v\",\"sourcebolt-config:/config\",\"ghcr.io/candidloom-hub/sourcebolt:0.3.2\",\"serve\"]}"
```

Cursor install link:

```text
cursor://anysphere.cursor-deeplink/mcp/install?name=sourcebolt&config=eyJjb21tYW5kIjoiZG9ja2VyIiwiYXJncyI6WyJydW4iLCItaSIsIi0tcm0iLCItdiIsInNvdXJjZWJvbHQtY29uZmlnOi9jb25maWciLCJnaGNyLmlvL2NhbmRpZGxvb20taHViL3NvdXJjZWJvbHQ6MC4zLjIiLCJzZXJ2ZSJdfQ==
```

<!-- END GENERATED -->

## Other clients

These are not links, but each is one command or one file. The ones marked "not run" follow the client's documented format and were not tried against the client.

| Client | Install | Checked |
|---|---|---|
| Claude Code (plugin) | `/plugin install sourcebolt --marketplace candidloom-hub/sourcebolt` (Claude Code 2.1.275 or later), or `/plugin marketplace add candidloom-hub/sourcebolt` then `/plugin install sourcebolt@sourcebolt`. Installs the MCP server (binary on PATH) and the sourcebolt skill. | `claude plugin validate --strict` passes on the plugin and the marketplace; install not run |
| Claude Code (server only) | `claude mcp add sourcebolt -- sourcebolt serve` | run |
| Claude Desktop | Download `sourcebolt-mcp-<os>-<arch>.mcpb` from a release and double-click it, or use Settings > Extensions > Advanced settings > Install Extension. Every release from v0.2.0 on has one bundle per platform. | manifest validated against the MCPB v0.3 schema; install not run |
| Gemini CLI | `gemini extensions install https://github.com/candidloom-hub/sourcebolt` (reads `gemini-extension.json`; needs the binary on PATH) | not run |
| Codex | `codex mcp add sourcebolt -- sourcebolt serve`, or the `config.toml` snippet in the README | not run |
