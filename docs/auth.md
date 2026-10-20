# Authentication

NotebookLM has no API key. sourcebolt acts as your signed-in browser session, so it needs your Google cookies for the NotebookLM host (default `notebook.google.com`). You sign in yourself, in a browser; sourcebolt never asks for or types your password, and MCP tools cannot sign in (`auth_login` and `auth_logout` only tell you which CLI command to run).

Everything here is a CLI command run in a terminal, not an MCP tool.

## Choosing a login method

| Method | Command | Notes |
|---|---|---|
| Attach (recommended) | `sourcebolt login --attach http://127.0.0.1:9222` | Reads cookies from a browser you started with a debugging port. Google does not block it because you started and signed in to a normal browser. |
| Browser window | `sourcebolt login --browser` | sourcebolt launches a browser on a dedicated profile and waits for sign-in (default 300 s, `--timeout`). Google may show "this browser may not be secure" and refuse sign-in, because automation-launched browsers expose `navigator.webdriver`. This was observed in our own test. |
| Import | `sourcebolt login --import cookies.txt` | Netscape `cookies.txt`, a JSON export or a `Cookie:` header; `-` reads stdin. `--format netscape\|json\|header` overrides auto-detection. |

All methods take `--profile <name>` (default `$NOTEBOOKLM_PROFILE` or `default`) and `--host <host>`.

Before saving, `login` calls list-notebooks with the new cookies. If that fails, nothing is saved and an existing working profile is not replaced.

## Attach flow in detail

Chrome (since version 136) ignores `--remote-debugging-port` on its default profile, so the browser must run on its own `--user-data-dir`. Use a folder under the sourcebolt config dir. (`<config dir>` is `%APPDATA%\sourcebolt` on Windows, `~/Library/Application Support/sourcebolt` on macOS, `~/.config/sourcebolt` on Linux, unless `NOTEBOOKLM_CONFIG_DIR` is set.) Use a folder name other than your profile name (`attach` below), because `browser/<profile>` is used by `login --browser` and the headless fallback.

Windows (PowerShell):

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

Chrome works the same way with its own executable (`chrome.exe`, `Google Chrome.app/Contents/MacOS/Google Chrome`, `google-chrome`). Then:

1. In that window, sign in to Google and open https://notebook.google.com.
2. In a terminal run `sourcebolt login --attach http://127.0.0.1:9222`.
3. Close the debugging browser. The debugging port lets any local process control that browser while it is open.

## Where data is stored

| Item | Location |
|---|---|
| Encrypted profile | `<config dir>/profiles/<name>.profile` (AES-256-GCM; holds cookies, host, revision, timestamp) |
| Encryption key | OS keyring entry, service `sourcebolt`, user `<name>` (Windows Credential Manager, macOS Keychain, Linux Secret Service). If no keyring is usable: `<config dir>/profiles/<name>.key`, file mode `0600`. |
| Browser profile for `login --browser` and the headless fallback | `<config dir>/browser/<profile>` |
| Optional RPC id overrides | `<config dir>/rpc-ids.json` (or `NOTEBOOKLM_RPC_IDS`) |
| Optional UI selector overrides for the DOM fallback | `<config dir>/selectors.json` |

If the keyring is locked or fails with an error other than "not found", sourcebolt reports it instead of minting a new key, so an existing profile is never orphaned. Saves are revision-checked, so two processes cannot silently overwrite each other.

## Browser fallback

With the browser fallback on (the default; `--no-browser-fallback` or `NOTEBOOKLM_BROWSER_FALLBACK=0` turns it off), the headless browser runs on `<config dir>/browser/<profile>`, so it is signed in only after `login --browser` has run on that profile.

When Google changes the shape of the chat or add-source request (a `DRIFT` error), sourcebolt drives the NotebookLM page in that browser instead: it types the question into the chat box and reads the answer, or opens the add-source dialog and adds the URL or pasted text. Such calls report the transport `browser-dom` (`server_info` shows the last transport). Limits of this path:

- Chat: only questions about all sources in the notebook's default thread. A question scoped to some sources, to another thread or to a new thread returns the original error. The answer is text only, without citation details. A chat request that drifted after it was sent is asked once more on the page (one extra turn against the daily limit).
- Add source: only URLs and pasted text, and only when the request was not sent; an add that may have reached Google is never repeated. Drive sources stay unsupported. The pasted-text title is not set on this path (NotebookLM names the source).

The page controls are found through a selector registry built into sourcebolt. If NotebookLM's page changes before a new release, you can override any control in `<config dir>/selectors.json`. Each key replaces the built-in list for that control, and candidates are tried in order:

```json
{"chat.input": ["textarea.query-box-input", "textarea[aria-label=\"Query box\"]"]}
```

A candidate is a CSS selector, `text:<words>` (a visible button or menu item containing the words) or `icon:<name>` (the button around a Material icon). If the file is not valid JSON, sourcebolt prints one warning on stderr and uses the built-in registry.

## Commands

- `sourcebolt status` shows the config dir, the active profile and, for each saved profile, the host, last update, where the key lives and whether the required cookies are present.
- `sourcebolt doctor` checks that the config dir is writable, that the OS keyring works (otherwise profiles use the key file), that the profile loads, that the cookies are complete, and does a token bootstrap against Google. It exits 1 on any failure. The bootstrap is the only step that touches the network, and only when a valid session exists.
- `sourcebolt logout [--profile p]` deletes the encrypted profile and its key. It does not sign you out of Google; to revoke the session, sign out of Google in the browser or from your Google account's security page.

## Sessions expire

Google rotates and expires cookies. When a call fails with an authentication error, run `login` again. For a long-running setup, `doctor` tells you whether the session still works.
