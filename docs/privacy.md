# Privacy policy

Last updated: 2026-10-20. Applies to sourcebolt 0.1.0 and later, in every package (release binary, Docker image, MCP bundle, Claude Code plugin, npm and PyPI wrappers).

## Short version

- sourcebolt runs on your own computer. There is no sourcebolt server and no sourcebolt account.
- It talks to Google NotebookLM, with your own Google session, to do what you or your AI client ask.
- Your Google cookies are stored encrypted on your computer.
- There is no telemetry, no analytics, no crash reporting and no update check.
- Nothing is sent to the sourcebolt authors. We never see your notebooks, questions, answers or cookies.

## What sourcebolt does with your data

sourcebolt is a local MCP server. Your MCP client (for example Claude Code, Claude Desktop, Cursor or VS Code) starts it on your machine and sends it tool calls. sourcebolt turns each call into requests to NotebookLM and returns the result to that client.

### Where your data goes

| Destination | When | What is sent |
|---|---|---|
| The NotebookLM web host (default `notebook.google.com`) | Every tool call that needs NotebookLM, and `login`/`doctor` checks | Your Google session cookies and the content of the call: notebook ids, questions, source URLs or pasted text, note text, sharing changes. This is the same kind of traffic the NotebookLM website sends from your browser. |
| Your DNS resolver | `source_add` with URLs | The host names of those URLs, looked up to refuse private-network addresses before the URL is sent to Google. |
| Google's own page resources | Only when the browser fallback runs (a headless Chromium on a dedicated profile), or during `login --browser` | Whatever a normal browser loads to show the NotebookLM page. |
| A browser on your machine | Only during `login --attach <url>` | sourcebolt connects to the local DevTools address you give it and reads the Google cookies of that browser. |

You can change the NotebookLM host with `login --host` or the `NOTEBOOKLM_HOST` environment variable. sourcebolt then sends the same data to the host you chose.

Google processes what NotebookLM receives under Google's own terms and privacy policy: https://policies.google.com/privacy. When you add a URL as a source, Google (not your machine) fetches that URL.

sourcebolt sends nothing anywhere else. The code makes HTTP requests only to the NotebookLM host; you can check this in `internal/transport/rpc` and `internal/session/tokens.go`.

### What is stored on your computer

All under the sourcebolt config directory: `%APPDATA%\sourcebolt` on Windows, `~/Library/Application Support/sourcebolt` on macOS, `~/.config/sourcebolt` on Linux, or `NOTEBOOKLM_CONFIG_DIR` if you set it. In the Docker image it is the `/config` volume.

| Item | Details |
|---|---|
| Sign-in profile | `profiles/<name>.profile`: your Google cookies, the host, a revision and a timestamp, encrypted with AES-256-GCM. |
| Encryption key | In your operating system's keyring (Windows Credential Manager, macOS Keychain, Linux Secret Service). If no keyring is available (for example in Docker), in `profiles/<name>.key` with file mode `0600`. |
| Browser profile | `browser/<profile>`: only if you use `login --browser` or the browser fallback. It is a normal Chromium profile, managed by the browser. |
| Optional overrides | `rpc-ids.json` and `selectors.json`, only if you create them. |

Notebook data, answers and tool results are kept only in memory while the server runs (a short-lived cache and a job table) and are gone when it stops. Log messages go to standard error, which your MCP client may keep in its own logs. sourcebolt masks cookies, CSRF tokens and request bodies in log lines and errors.

`sourcebolt logout` deletes a profile and its key. It does not delete the `browser/` folder (delete it yourself if you used the browser login or fallback), and it does not sign you out of Google; do that from your Google account.

### What the AI client sees

Tool results (notebook titles, source text, answers with citations, notes) go to your MCP client, and from there to whatever AI model it uses. That part is governed by your client's and model provider's privacy terms, not by sourcebolt. No tool accepts or returns cookies or passwords.

## Data we collect

None. The sourcebolt authors run no service that receives data from sourcebolt.

If you open a GitHub issue, discussion or pull request, GitHub's privacy statement applies to what you post there. Please never paste cookies, tokens or HAR files into an issue.

## Children

sourcebolt is a developer tool and is not directed at children.

## Changes

Changes to this policy are made in this file, and its history is public in the repository. A change that affects what data leaves your computer will also be listed in the [changelog](../CHANGELOG.md).

## Contact

Open an issue at https://github.com/candidloom-hub/sourcebolt/issues. For security problems, use private vulnerability reporting as described in [security.md](security.md).

sourcebolt is an independent project. It is not affiliated with, endorsed by or sponsored by Google. "NotebookLM" is a trademark of Google LLC.
