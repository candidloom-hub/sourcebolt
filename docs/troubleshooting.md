# Troubleshooting

Find your error below by its code or message. Codes such as `AUTH_REQUIRED` appear in the `code` field of the error your AI app shows. Messages from the command line start with `error:`.

## First steps for any problem

1. Run the health check:

   ```bash
   sourcebolt doctor
   ```

   Each line starts with `ok` or `FAIL`. The first `FAIL` usually names the problem. `doctor` exits with code 1 if any check fails.

2. Show where sourcebolt keeps its data and which profiles exist:

   ```bash
   sourcebolt status
   ```

   `valid_cookies=false` means the saved session is incomplete: sign in again.

3. Make sure you have the latest release: `sourcebolt --version`, then compare with the [releases page](https://github.com/candidloom-hub/sourcebolt/releases). Google changes NotebookLM often, and new releases follow.

4. In your AI app, ask it to call `server_info`. It shows the version, the active profile, the NotebookLM host and which path (transport) served the last call.

## How errors look

Every failed tool call returns a JSON error like this:

```json
{
  "status": "error",
  "error": "AUTH_REQUIRED: wXbhsf: session: signed out (signed-out page)",
  "code": "AUTH_REQUIRED",
  "retriable": false,
  "guidance": "The Google session is missing or expired. Run: sourcebolt login (it opens a browser to sign in), or import cookies with: sourcebolt login --import <cookies file>",
  "transport": "rpc"
}
```

- `code` is stable; the sections below are named after it.
- `retriable` says whether trying again later can help.
- `guidance` is a short fix.
- `retry_after_seconds` appears when Google said how long to wait.
- Cookies, tokens and request bodies are masked in every error and log line.

## Sign-in problems

### `AUTH_REQUIRED` (signed out, session expired)

Google no longer accepts the saved session. Google rotates and expires sessions, and signing out of Google in the browser you used also ends it.

Fix: sign in again with `sourcebolt login` ([auth.md](auth.md)), then retry.

```bash
sourcebolt login
```

If that window will not let you sign in, use `sourcebolt login --attach http://127.0.0.1:9222` or `sourcebolt login --import <cookies file>` ([auth.md](auth.md)).

You also see this code when NotebookLM redirects to a Google sign-in page, or when the browser fallback lands on a sign-in page.

### `COOKIE_BINDING`

Google rejected the cookies for this client, usually because they were copied from a different browser session than the one they are tied to. Sign in again with `sourcebolt login`, or import fresh cookies with `sourcebolt login --import <cookies file>`.

### `error: missing required cookies: ...` (cookie import)

```text
error: missing required cookies: SID, __Secure-1PSIDTS, OSID or APISID+SAPISID. Export every google.com cookie (sourcebolt needs SID, __Secure-1PSIDTS, and OSID or APISID+SAPISID), not only the __Secure- ones
```

The message lists only the cookies that are missing; the example above is what you see when all of them are missing.

`login` checks for these cookies before it saves anything:

- `SID`
- `__Secure-1PSIDTS`
- `OSID`, or both `APISID` and `SAPISID`

Many browser extensions export only the `__Secure-` cookies. Without the plain `SID` and `APISID` cookies, NotebookLM returns a signed-out page.

Fix: export **every** cookie for `google.com` (not only for `notebook.google.com`), or use `login --attach`, which reads them all for you.

### `error: signed in, but missing required cookies: ...` (attach or browser login)

The browser was not fully signed in when sourcebolt read it. Open https://notebook.google.com in that window, wait until your notebooks show, then run `login` again. Nothing was saved.

### `error: cookies did not authenticate with notebook.google.com: ...`

The cookies were complete but Google did not accept them (for example they were already expired). sourcebolt saved nothing and kept any working profile. Sign in again in the browser and repeat `login`.

### `error: sign-in timed out after 10m0s; nothing saved.`

`sourcebolt login` (and the sign-in step of `sourcebolt setup`) waits up to 10 minutes for you to finish signing in. Nothing was saved and the browser window was closed. Run `sourcebolt login` again. If you need more time, add `--timeout 20m`.

If the message also says `last problem: the cookies did not work yet`, you were signed in to Google but NotebookLM did not accept the session. Make sure https://notebook.google.com shows your notebooks in that window, or try the attach method below.

### `error: sign-in cancelled: the browser window was closed; nothing saved.`

The browser window was closed before sign-in finished. Run `sourcebolt login` again and leave the window open; it closes by itself when you are done.

If the window closes right after it opens, another sourcebolt may be using the same browser folder (for example the browser fallback of a running AI app). Quit your AI apps and run `sourcebolt login` again.

### Google shows "This browser or app may not be secure"

Google blocks sign-in in browsers that look automated. `sourcebolt login` (no options) starts your browser as a normal program, with only a local debugging port, which is what the attach method does by hand, so Google normally allows it. `login --browser` starts the browser through automation and Google often blocks it.

If Google still refuses, use one of the other ways ([auth.md](auth.md)):

- **Attach:** you start the browser yourself, sign in, then run `sourcebolt login --attach http://127.0.0.1:9222`. See [auth.md](auth.md#attach-flow-in-detail).
- **Import:** export your cookies from a browser where you are signed in, then run `sourcebolt login --import cookies.txt`.

### `error: no Chrome, Edge or Brave browser was found on this computer`

`login --browser` and the browser fallback say it this way: `error: no Chromium-based browser found; install Brave, Chrome or Edge, or pass --browser-path <exe> or set SOURCEBOLT_BROWSER`.

Signing in (`sourcebolt login`, `sourcebolt setup`, `login --browser`) and the browser fallback need Brave, Chrome, Edge or Chromium. Install one, or tell sourcebolt where yours is with `sourcebolt login --browser-path "<path to the browser program>"`, or set it once:

PowerShell:

```powershell
$env:SOURCEBOLT_BROWSER = "C:\Program Files\Google\Chrome\Application\chrome.exe"
```

bash:

```bash
export SOURCEBOLT_BROWSER=/usr/bin/google-chrome
```

## Storage and keyring problems

sourcebolt encrypts your session. The encryption key lives in the OS keyring (Windows Credential Manager, macOS Keychain, Linux Secret Service), or in a private `.key` file when no keyring is available. Details: [auth.md](auth.md#where-data-is-stored).

### `FAIL os keyring` in `doctor`

```text
FAIL  os keyring             <reason> (profiles fall back to a 0600 key file)
```

No usable keyring, which is common on servers, in containers and on Linux without a desktop session. This is not fatal: profiles use a key file readable only by you. Protect the config folder accordingly.

### `Replaced profile "default": the old session could not be read (its encryption key was missing or did not match, or the file was damaged).`

This is a notice, not an error. The profile existed, but it could never be read again: its key was gone from both the keyring and the key file (for example the keyring entry was deleted, or the config folder was copied to another machine), the key no longer matched the file (`session: decrypt profile: ...`), or the file was damaged. So `login` deletes it and saves your new session in its place. Nothing else to do.

Until you log in again, `doctor` fails to load that profile, and the MCP server still starts but every tool call returns `AUTH_REQUIRED` with `the saved profile "default" could not be read (...); run: sourcebolt login to replace it`. Fix: run `login` again. A running server picks up the new login on its next call.

```bash
sourcebolt login --attach http://127.0.0.1:9222
```

### `error: profile "default" exists but cannot be read (...); run: sourcebolt logout --profile default, then log in again`

`login` found a profile it cannot read for a reason that may pass, for example a locked keyring (`session: keyring: ...`) or a file it is not allowed to open. sourcebolt does not delete it on its own, so a locked keyring never costs you a working profile.

If the keyring is only locked, unlock it (sign in to your desktop session) and retry. Otherwise do what the message says:

```bash
sourcebolt logout --profile default
sourcebolt login --attach http://127.0.0.1:9222
```

Use your profile name in place of `default`, and add `--profile <name>` to `login` too. `logout` only deletes the local profile and key; it does not sign you out of Google.

### Signed in, but another app says you are not (Windows)

You ran `sourcebolt login` in a terminal, but Claude Desktop (or another app) still says you are signed out, or reports `session: decrypt profile: cipher: message authentication failed` or `key file not found`.

Versions up to 0.3.1 kept their data in `%APPDATA%\sourcebolt`. Apps installed from the Microsoft Store (packaged MSIX apps, such as Claude Desktop from the Store) get a private copy of `%APPDATA%` for every program they start. So the terminal and the app each saw their own profile file, but both used the same key in Windows Credential Manager, and a login in one replaced the key the other's file needed.

Newer versions fix this:

- The data folder on Windows is now `%USERPROFILE%\.sourcebolt`, which Windows does not redirect.
- On start, a profile that is only in the old `%APPDATA%\sourcebolt` folder is copied over if it can still be read. The old folder is not changed or deleted; remove it yourself once everything works.
- Each data folder has its own Credential Manager entry (`sourcebolt:<profile>@<folder id>`), so two folders never share a key again.

Fix: update sourcebolt, restart the app, and sign in once more if it still says you are signed out:

```bash
sourcebolt login
```

`NOTEBOOKLM_CONFIG_DIR` still wins over the default folder, and no profile is copied when it is set.

## Limits from Google

### `RATE_LIMITED`

Google is slowing this account down. Wait before retrying. If the error has `retry_after_seconds`, wait at least that long. Do not let an agent retry in a loop: that makes it worse.

If the guidance says *"Too many detached jobs are running"*, the limit is sourcebolt's own, not Google's: too many `chat_ask` calls with `wait=false` are still running. Check them with `chat_status`, cancel with `job_cancel`, or ask with `wait=true`.

### `QUOTA`

The account used up a NotebookLM quota (chats per day, Studio generations, and so on). Quotas are set by Google per account and reset later. Ask your app to call `account_settings` to see the quota windows. sourcebolt does not bypass quotas.

### `PERMISSION_DENIED`

This account cannot open that notebook or do that action, for example editing a notebook that was shared with you as viewer.

## NotebookLM changed

### `DRIFT`

NotebookLM changed the format of one of its internal requests or responses, so sourcebolt could not read the answer.

What happens automatically: for **chat** and **adding sources**, sourcebolt can fall back to driving the NotebookLM page in a headless browser (the "DOM fallback"). Such calls report the transport `browser-dom`. Limits of that path:

- Chat: only questions about all sources, in the notebook's default conversation. The answer is text without citation details.
- Add source: only URLs and pasted text, and only when the original request was never sent.

The DOM fallback uses the browser profile at `<config dir>/browser/<profile>` (for the default profile: `browser/default`). It works only if that browser profile is signed in to Google. See [auth.md, Sign in the fallback browser](auth.md#sign-in-the-fallback-browser).

What you can do:

1. Update to the latest release.
2. If a release is not out yet and the maintainers published new request ids, set them with `NOTEBOOKLM_RPC_IDS` or `<config dir>/rpc-ids.json`.
3. If the page layout changed, override the page controls in `<config dir>/selectors.json` ([auth.md](auth.md#browser-fallback)).
4. [Open an issue](https://github.com/candidloom-hub/sourcebolt/issues) with the tool name and the error text (cookies and tokens are already masked).

### Check that the DOM fallback works before you need it

`doctor --dom-check` asks one short question through the page, so you know the fallback works on your machine. (It was added after v0.1.0, so the v0.1.0 binary does not have it.)

```bash
sourcebolt doctor --dom-check "<notebook id or title>"
```

Expected extra lines. This example is from one live run, not a benchmark; the length and time depend on the notebook and on NotebookLM:

```text
      dom answer: 544 chars in 5.2s
ok    dom check
```

If it fails with *"the browser fallback is disabled"*, you started sourcebolt with `--no-browser-fallback` or `NOTEBOOKLM_BROWSER_FALLBACK=0`. If it fails with `AUTH_REQUIRED`, the fallback browser profile is not signed in; see [auth.md, Sign in the fallback browser](auth.md#sign-in-the-fallback-browser).

## Other tool errors

### `UNSUPPORTED_BY_TRANSPORT`

The feature exists in sourcebolt but is not available yet, usually because Google's request format has not been confirmed by a live test. Examples: Drive and file sources, most Studio kinds, source refresh. This is not a mistake in your request. The current list is in [tools.md](tools.md) and in the "Known limitations" of the [changelog](../CHANGELOG.md).

`auth_login` and `auth_logout` always return this code on purpose: sign-in works only from the command line, so cookies never pass through a chat.

### `PENDING`

Google accepted the request but did not confirm it. The item (for example a new source) may still appear. List the items before you retry, or you may add it twice.

### `AMBIGUOUS` or `NOT_FOUND`

The notebook, source or note name matched several items, or none. The error lists `suggestions` with titles and ids. Use the id, or the exact title.

### `INVALID_ARGUMENT`

A parameter is wrong or missing, or Google refused the input. Read the `error` text. For example, `research_start` refuses queries shorter than four words unless `force` is true.

### `TRANSPORT`

A network error or a temporary Google server error. It is marked `retriable`; try again in a moment.

## Setup problems

### Setup did not find my app

`sourcebolt setup` looks for Claude Desktop, Claude Code, Cursor, VS Code, Windsurf and Gemini CLI in their usual places for your user. If your app is not listed, or keeps its settings somewhere else, connect it by hand: [clients.md](clients.md#manual-setup).

### `Could not connect <app>: ... the file is not plain JSON (it may contain comments)`

The app's settings file has comments or another format setup cannot change safely, so setup left it alone. Add the `sourcebolt` entry by hand ([clients.md](clients.md#manual-setup)). Setup prints the file's path.

### I want to undo setup

```bash
sourcebolt setup --undo
```

This puts each app's settings file back from the newest copy setup saved (`<file>.sourcebolt-backup-<date>-<time>`), then restart the app. Run it again to go one more step back. Only one app: add `--client cursor` (or another name). Changes you made to that file after setup are undone too, except in Claude Code's `~/.claude.json`, where only the `sourcebolt` entry is put back.

`--undo` does not sign you out. To delete the saved session as well, run `sourcebolt logout`.

## Connection problems

### My AI app does not show sourcebolt

1. Run `sourcebolt --version` in a terminal. If it is not found, fix your `PATH` or use the full path in the app config ([clients.md](clients.md#if-sourcebolt-is-not-on-your-path)).
2. Check the JSON or TOML syntax of the config file.
3. Restart the app completely.
4. Look at the app's MCP logs (Claude Desktop: [clients.md](clients.md#claude-desktop)).

### `error: --http must be a loopback address, got ...`

HTTP mode only listens on `127.0.0.1`, `::1` or `localhost`. This is deliberate. See [clients.md](clients.md#http-mode).

### `error: token must be at least 16 characters`

The `--token-file` holds a token that is too short. Create a longer random one ([clients.md](clients.md#http-mode)).

## Still stuck

[Open an issue](https://github.com/candidloom-hub/sourcebolt/issues) with:

- `sourcebolt --version`
- your operating system
- the output of `sourcebolt doctor`
- the tool name and the error JSON

Remove any cookies, tokens and notebook content you do not want to share. For security problems, follow [security.md](security.md#reporting-a-vulnerability) instead.
