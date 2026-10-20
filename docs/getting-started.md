# Getting started

This guide takes you from nothing to your first cited answer from NotebookLM inside an AI app. It takes about ten minutes.

You need:

- A Google account that can open [NotebookLM](https://notebook.google.com).
- Brave, Chrome or Edge, for signing in.
- An AI app that supports MCP (Model Context Protocol), such as Claude Code, Claude Desktop, Cursor, VS Code or Codex.

Tip: while you try sourcebolt, use a test notebook with public content.

## Step 1: Install

Download the program from the [latest release](https://github.com/candidloom-hub/sourcebolt/releases).

**Windows (PowerShell).** For an ARM64 PC, replace `amd64` with `arm64`.

```powershell
$dir = "$env:LOCALAPPDATA\Programs\sourcebolt"
New-Item -ItemType Directory -Force $dir | Out-Null
Invoke-WebRequest https://github.com/candidloom-hub/sourcebolt/releases/download/v0.1.0/sourcebolt_0.1.0_windows_amd64.zip -OutFile "$env:TEMP\sourcebolt.zip"
Expand-Archive "$env:TEMP\sourcebolt.zip" -DestinationPath $dir -Force
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

Close the terminal and open a new one, so it picks up the new `PATH`.

**macOS and Linux (bash).** Set `OS` to `darwin` (macOS) or `linux`, and `ARCH` to `amd64` (Intel/AMD) or `arm64` (Apple silicon, ARM).

```bash
OS=linux ARCH=amd64
curl -fsSLO "https://github.com/candidloom-hub/sourcebolt/releases/download/v0.1.0/sourcebolt_0.1.0_${OS}_${ARCH}.tar.gz"
tar -xzf "sourcebolt_0.1.0_${OS}_${ARCH}.tar.gz" sourcebolt
mkdir -p ~/.local/bin && mv sourcebolt ~/.local/bin/
```

If your shell cannot find `sourcebolt` afterwards, add `~/.local/bin` to your `PATH`.

Other ways to install (Go, Docker): [README, Install](../README.md#install).

**Check it.** On every system:

```bash
sourcebolt --version
```

Expected output:

```text
sourcebolt 0.1.0
```

## Step 2: Sign in

sourcebolt needs your Google session, the same one your browser uses when you open NotebookLM. You sign in yourself, in a real browser window. sourcebolt never sees your password.

The recommended way is **attach**: you start a browser with a "debugging port" (a local door that lets sourcebolt read that browser's cookies), sign in, and sourcebolt copies the session.

**2a. Start the browser on a separate profile.** The separate folder keeps your everyday browser out of this. Chrome also refuses the debugging port on its default profile.

Windows (PowerShell), Brave:

```powershell
& "C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe" --remote-debugging-port=9222 --user-data-dir="$env:APPDATA\sourcebolt\browser\attach"
```

Windows (PowerShell), Chrome:

```powershell
& "C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222 --user-data-dir="$env:APPDATA\sourcebolt\browser\attach"
```

macOS, Brave (for Chrome use `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`):

```bash
"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser" --remote-debugging-port=9222 --user-data-dir="$HOME/Library/Application Support/sourcebolt/browser/attach"
```

Linux, Brave (for Chrome use `google-chrome`):

```bash
brave-browser --remote-debugging-port=9222 --user-data-dir="$HOME/.config/sourcebolt/browser/attach"
```

**2b. Sign in.** In that new window, sign in to Google and open https://notebook.google.com. Wait until you see your notebooks.

**2c. Save the session.** In a second terminal:

```bash
sourcebolt login --attach http://127.0.0.1:9222
```

Expected output (your number of notebooks will differ):

```text
Saved profile "default" for notebook.google.com (9 notebooks visible).
```

sourcebolt made one real call (list notebooks) to prove the session works. It saves nothing if that call fails.

**2d. Close the debugging browser.** While it is open, any program on your computer can control it.

Other sign-in methods (a cookie file, a browser window that sourcebolt opens): [auth.md](auth.md).

## Step 3: Check the setup

```bash
sourcebolt doctor
```

Expected output (times and paths differ):

```text
ok    config dir writable
ok    os keyring
ok    profile default
ok    token bootstrap notebook.google.com (412ms)
profiles dir: <config dir>/profiles
```

Every check line should start with `ok`. If one starts with `FAIL`, see [troubleshooting.md](troubleshooting.md). A `FAIL` on `os keyring` alone is fine: the encryption key is then kept in a private file instead.

You can also try a tool straight from the terminal, without an AI app:

```bash
sourcebolt call notebook_list '{}'
```

It prints JSON with your notebooks: id, title, number of sources and whether you own each one.

## Step 4: Connect your AI app

Pick your app. Each one starts `sourcebolt serve` in the background when it needs it.

**Claude Code:**

```bash
claude mcp add sourcebolt -- sourcebolt serve
```

Check it with `claude mcp list`. The line for `sourcebolt` should say it is connected.

**Claude Desktop, Cursor, Windsurf:** add this to the app's MCP config file, then restart the app.

```json
{
  "mcpServers": {
    "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
  }
}
```

**VS Code, Codex, Gemini CLI, ChatGPT and file locations:** [clients.md](clients.md).

## Step 5: Ask your first question

In your AI app, type:

> Use sourcebolt to list my NotebookLM notebooks. Then ask the first notebook for a one-paragraph summary of its sources, with citations.

What happens:

1. The assistant calls `notebook_list` and shows your notebooks.
2. It calls `chat_ask` with the notebook and your question.
3. The answer comes back with citations: which source, and the exact text that supports each point.

The first answer usually takes a few seconds to a minute. Almost all of that time is NotebookLM thinking, not sourcebolt.

## What to try next

- "Add https://example.com/article to my *Test* notebook." (`source_add`)
- "Research 'home battery recycling methods in Europe' and show me the candidate sources." (`research_start`, then `research_status`)
- "Create a note called *Summary* in my *Test* notebook with the three main points." (`note_save`)
- "What is still processing in my *Test* notebook?" (`source_list`)

Every tool and option: [tools.md](tools.md). How an agent should pick tools: [SKILL.md](../SKILL.md).

## When something goes wrong

- Any error with `AUTH_REQUIRED`: your session expired. Repeat [Step 2](#step-2-sign-in).
- Other errors: [troubleshooting.md](troubleshooting.md).
