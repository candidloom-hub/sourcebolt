# Sign-in and storage

NotebookLM has no API key. sourcebolt acts as your signed-in browser session, so it needs your Google session cookies for the NotebookLM host (default `notebook.google.com`).

- You sign in yourself, in a browser. sourcebolt never asks for or types your password.
- Sign-in works only from the command line. The MCP tools `auth_login` and `auth_logout` only tell you which command to run, so cookies never pass through a chat.

Every command on this page runs in a terminal.

## Choose a login method

| Method | Command | When to use it |
|---|---|---|
| **Plain login** (easiest, after v0.2.0) | `sourcebolt login` | sourcebolt finds Chrome, Edge or Brave and starts it as a normal program with only a local debugging port, on the folder `<config dir>/browser/<profile>`, opening NotebookLM. You sign in; when your notebooks load, sourcebolt saves the session, closes the window and prints how many notebooks it sees. It waits 10 minutes (`--timeout` changes this). Closing the window cancels and saves nothing. This is the attach method below, done for you. `sourcebolt setup` uses it too. If Google still refuses to sign you in, use attach or import. |
| **Attach** | `sourcebolt login --attach http://127.0.0.1:9222` | You start a normal browser with a debugging port and sign in. Google does not block it, because it is a normal browser that you started. The only method in v0.2.0 that Google reliably allows. |
| Browser window | `sourcebolt login --browser` | sourcebolt opens a browser on its own profile and waits for you to sign in (300 seconds by default; change with `--timeout`). Google may refuse with "This browser or app may not be secure", because browsers started by automation expose `navigator.webdriver`. This happened in our own test. |
| Import | `sourcebolt login --import cookies.txt` | You already have a cookie export. Accepts a Netscape `cookies.txt`, a JSON export or a `Cookie:` header. `-` reads from standard input. `--format netscape\|json\|header` overrides the automatic detection. |

All methods accept:

- `--profile <name>` to keep several accounts apart (default: `$NOTEBOOKLM_PROFILE`, or `default`).
- `--host <host>` for a different NotebookLM host.

Before saving, `login` checks that these cookies are present: `SID`, `__Secure-1PSIDTS`, and either `OSID` or both `APISID` and `SAPISID`. Then it makes one real call (list notebooks) with the new cookies. If either step fails, nothing is saved, and an existing working profile is kept.

If the existing profile's encryption key is gone (from both the keyring and the key file), that profile can never be read again; `login` replaces it and says so. Other unreadable profiles are left alone, and `login` tells you to run `sourcebolt logout --profile <name>` first.

## Attach flow in detail

The browser must run on its own profile folder (`--user-data-dir`). Chrome 136 and newer ignore `--remote-debugging-port` on the default profile, and a separate folder keeps your everyday browser out of it.

`<config dir>` below means:

| System | Config dir |
|---|---|
| Windows | `%USERPROFILE%\.sourcebolt` |
| macOS | `~/Library/Application Support/sourcebolt` |
| Linux | `~/.config/sourcebolt` |

Set `NOTEBOOKLM_CONFIG_DIR` to use another folder. Versions up to 0.3.1 used `%APPDATA%\sourcebolt` on Windows; profiles there are copied to the new folder automatically (see [troubleshooting](troubleshooting.md#signed-in-but-another-app-says-you-are-not-windows)).

**1. Start the browser.**

Windows (PowerShell), Brave:

```powershell
& "C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe" --remote-debugging-port=9222 --user-data-dir="$env:USERPROFILE\.sourcebolt\browser\attach"
```

Windows (PowerShell), Chrome:

```powershell
& "C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222 --user-data-dir="$env:USERPROFILE\.sourcebolt\browser\attach"
```

macOS, Brave:

```bash
"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser" --remote-debugging-port=9222 --user-data-dir="$HOME/Library/Application Support/sourcebolt/browser/attach"
```

Linux, Brave:

```bash
brave-browser --remote-debugging-port=9222 --user-data-dir="$HOME/.config/sourcebolt/browser/attach"
```

For Chrome on macOS use `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`; on Linux use `google-chrome`.

**2. Sign in.** In that window, sign in to Google and open https://notebook.google.com.

**3. Attach.** In a terminal:

```bash
sourcebolt login --attach http://127.0.0.1:9222
```

Expected output: `Saved profile "default" for notebook.google.com (N notebooks visible).`

**4. Close the debugging browser.** While it is open, any program on your computer can control it through the debugging port.

## Sign in the fallback browser

The [browser fallback](#browser-fallback) runs a headless browser on the folder `<config dir>/browser/<profile>` (for the default profile: `browser/default`). It can only help if that browser profile is signed in to Google. Two ways:

- **Attach with that folder.** In step 1 above, use `browser\default` (or `browser/default`) instead of `browser\attach` as `--user-data-dir`. Sign in, attach, then **close the browser**: the headless fallback cannot open the folder while another browser uses it. This flow was tested live on 2026-10-20: `login --attach` on that folder, then `sourcebolt doctor --dom-check <notebook>` passed.
- **`sourcebolt login`** (after v0.2.0). This signs in exactly that folder and closes the browser when done, so the fallback can use it right away.
- **`sourcebolt login --browser`.** This signs in the same folder, but Google may block it (see above).

If you never sign in that folder, sourcebolt still works on the fast direct path; only the fallbacks are unavailable. To check the fallback, run `sourcebolt doctor --dom-check "<notebook id or title>"` ([troubleshooting.md](troubleshooting.md#check-that-the-dom-fallback-works-before-you-need-it)).

## Docker

The Docker image stores its config in the `/config` volume. It has no OS keyring, so the profile key is kept in a private file in that volume. Protect the volume like a password.

The simplest way is to import cookies through standard input.

bash:

```bash
docker run -i --rm -v sourcebolt-config:/config ghcr.io/candidloom-hub/sourcebolt:0.1.0 login --import - < cookies.txt
```

PowerShell:

```powershell
Get-Content cookies.txt -Raw | docker run -i --rm -v sourcebolt-config:/config ghcr.io/candidloom-hub/sourcebolt:0.1.0 login --import -
```

Export every `google.com` cookie, not only the `__Secure-` ones: `login` needs `SID`, `__Secure-1PSIDTS`, and `OSID` or `APISID` plus `SAPISID`. Delete the cookie file afterwards.

## Sign in with an environment variable (containers)

A container started by an MCP app (Docker MCP Toolkit, a `docker run` line in a client config) cannot run `login`. Instead, hand it the cookie export itself:

| Variable | Value |
|---|---|
| `NOTEBOOKLM_COOKIES_FILE` (preferred) | Path of a file holding the export, for example a Docker secret at `/run/secrets/notebooklm_cookies`. |
| `NOTEBOOKLM_COOKIES` | The export itself. |

Both accept what `login --import` accepts, detected the same way: a Netscape `cookies.txt`, a JSON cookie array, or a `Cookie:` header (`SID=...; __Secure-1PSIDTS=...; ...`). Set only one; setting both is an error. An empty value counts as not set.

How it behaves:

- `serve`, `call`, `status` and `doctor` read the variable at startup. The same required-cookie check as `login` runs: without `SID`, `__Secure-1PSIDTS`, and `OSID` or `APISID` plus `SAPISID`, sourcebolt stops with an error that names the missing cookies. Errors never show cookie values.
- **The cookies stay in memory.** Nothing is written to `<config dir>/profiles`, and cookies Google rotates during the session are kept in memory only. After a restart sourcebolt starts again from the export, so when Google expires that session (calls fail with `AUTH_REQUIRED`), export fresh cookies and update the secret.
- **When the variable is set, it wins over a saved profile** of the same name. Setting it is an explicit choice for that one process, and a container has no saved profile anyway. The saved profile is not read or changed; remove the variable to use it again. `status` and `doctor` say when the cookies come from the variable.
- sourcebolt removes both variables from its own environment right after reading them, so the fallback browser and any other child process do not inherit them.

**Security.** The export is your Google session: treat it like a password.

- Prefer the file form. A value passed with `-e NOTEBOOKLM_COOKIES=...` is visible to anyone who can run `docker inspect` on the container, and may end up in shell history or in your MCP app's config file.
- Keep the file in a folder only you can open (for example `chmod 700` on the folder). The image runs as user 65532, so on Linux the file itself must be readable by that user (`chmod 644` inside the private folder); Docker Desktop on Windows and macOS does not need this.
- Never commit the file or paste it into a chat.

Example with the file form (bash):

```bash
docker run -i --rm \
  -v "$HOME/.sourcebolt-secrets/cookies.txt:/run/secrets/notebooklm_cookies:ro" \
  -e NOTEBOOKLM_COOKIES_FILE=/run/secrets/notebooklm_cookies \
  ghcr.io/candidloom-hub/sourcebolt:0.3.2 doctor
```

`doctor` should print `ok    cookies from NOTEBOOKLM_COOKIES_FILE (in memory, not saved)` and a working token bootstrap. For client configs, see [clients.md](clients.md#docker).

## Where data is stored

| Item | Location |
|---|---|
| Encrypted profile | `<config dir>/profiles/<name>.profile` (AES-256-GCM; holds cookies, host, revision, timestamp) |
| Encryption key | OS keyring entry, service `sourcebolt`, user `<name>@<folder id>` (Windows Credential Manager, macOS Keychain, Linux Secret Service). The folder id is the first 12 hex digits of the SHA-256 of the profiles folder path, so two config folders never share a key. Entries named just `<name>` come from versions up to 0.3.1: they are still read when they unlock this folder's profile, and copied to the new name on the next save. Without a usable keyring: `<config dir>/profiles/<name>.key`, readable only by you (file mode `0600`). |
| Browser profile for `login --browser` and the fallback | `<config dir>/browser/<profile>` |
| Optional request id overrides | `<config dir>/rpc-ids.json` (or `NOTEBOOKLM_RPC_IDS`) |
| Optional page control overrides for the DOM fallback | `<config dir>/selectors.json` |

If the keyring is locked or fails with an error other than "not found", sourcebolt reports it instead of creating a new key, so an existing profile is never orphaned. Saves are revision-checked, so two sourcebolt processes cannot silently overwrite each other.

## Browser fallback

sourcebolt sends requests directly first. With the browser fallback on (the default), two extra paths can help when the direct path fails:

1. **Browser replay.** If Google rejects the direct request, the same request is sent again from inside the headless browser.
2. **DOM fallback.** If Google changed the format of the chat or add-source request (a `DRIFT` error), sourcebolt drives the NotebookLM page itself: it types the question into the chat box and reads the answer, or opens the add-source dialog and adds the URL or text. These calls report the transport `browser-dom`; `server_info` shows the last transport.

Turn both off with `--no-browser-fallback` or `NOTEBOOKLM_BROWSER_FALLBACK=0`. Quota, sign-in and permission errors never trigger a fallback, because another path cannot fix them. A request that may already have reached Google is not replayed if repeating it could apply a change twice.

Limits of the DOM fallback:

- **Chat:** only questions about all sources, in the notebook's default conversation. A question limited to some sources, or in another or a new conversation, returns the original error. The answer is text only, without citation details. A chat request that drifted after it was sent is asked once more on the page (one extra question against your daily limit).
- **Add source:** only URLs and pasted text, and only when the request was not sent. An add that may have reached Google is never repeated. Drive sources are not supported. The title of pasted text is not set on this path; NotebookLM names the source.

### Overriding page controls

The page controls are found through a list of selectors built into sourcebolt. If the NotebookLM page changes before a new release, you can override any control in `<config dir>/selectors.json`. Each key replaces the built-in list for that control; candidates are tried in order:

```json
{"chat.input": ["textarea.query-box-input", "textarea[aria-label=\"Query box\"]"]}
```

A candidate is a CSS selector, `text:<words>` (a visible button or menu item containing the words) or `icon:<name>` (the button around a Material icon). If the file is not valid JSON, sourcebolt prints one warning and uses the built-in list.

## Commands

- `sourcebolt status` shows the config dir, the active profile, whether the cookies come from `NOTEBOOKLM_COOKIES` or `NOTEBOOKLM_COOKIES_FILE` and, for each saved profile, the host, last update, where the key lives and whether the required cookies are present.
- `sourcebolt doctor --dom-check "<notebook>"` runs the checks below and also asks one short question through the page fallback.
- `sourcebolt doctor` checks that the config dir is writable, that the OS keyring works (otherwise profiles use the key file), that the profile loads, that the cookies are complete, and does a token bootstrap against Google. It exits with code 1 if any check fails. Only the bootstrap uses the network, and only when a valid session exists.
- `sourcebolt logout [--profile p]` deletes the encrypted profile and its key. It does **not** sign you out of Google. To end the session, sign out of Google in that browser or on your Google account's security page.

## Sessions expire

Google rotates and expires session cookies. When a call fails with `AUTH_REQUIRED`, run `login` again. For a long-running setup, `doctor` tells you whether the session still works. More fixes: [troubleshooting.md](troubleshooting.md#sign-in-problems).
