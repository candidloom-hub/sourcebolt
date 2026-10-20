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
Invoke-WebRequest https://github.com/candidloom-hub/sourcebolt/releases/download/v0.3.0/sourcebolt_0.3.0_windows_amd64.zip -OutFile "$env:TEMP\sourcebolt.zip"
Expand-Archive "$env:TEMP\sourcebolt.zip" -DestinationPath $dir -Force
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

Close the terminal and open a new one, so it picks up the new `PATH`.

**macOS and Linux (bash).** Set `OS` to `darwin` (macOS) or `linux`, and `ARCH` to `amd64` (Intel/AMD) or `arm64` (Apple silicon, ARM).

```bash
OS=linux ARCH=amd64
curl -fsSLO "https://github.com/candidloom-hub/sourcebolt/releases/download/v0.3.0/sourcebolt_0.3.0_${OS}_${ARCH}.tar.gz"
tar -xzf "sourcebolt_0.3.0_${OS}_${ARCH}.tar.gz" sourcebolt
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
sourcebolt 0.3.0
```

## Step 2: Run setup

> **Version note.** `sourcebolt setup` and `sourcebolt login` without options come in the release after v0.2.0. With v0.2.0, follow [Doing it by hand](#doing-it-by-hand) and sign in with the attach method.

One command connects sourcebolt to your AI apps and signs you in:

```bash
sourcebolt setup
```

Setup does four things, and asks before each change:

1. **Finds your AI apps.** It looks for Claude Desktop, Claude Code, Cursor, VS Code, Windsurf and Gemini CLI.
2. **Connects each app you say yes to.** For every app it asks `Connect sourcebolt to <app>? [Y/n]`. Press Enter for yes, or type `n` for no. Before it changes an app's settings file, it saves a copy next to it (the copy's name ends in `.sourcebolt-backup-` and the date and time). It only adds a `sourcebolt` entry and leaves everything else in the file as it was. If the app already has a `sourcebolt` entry, setup asks before replacing it.
3. **Signs you in.** A browser window opens on the NotebookLM page. Sign in to your Google account there, the normal way. You type your password into Google's own page; sourcebolt never sees it. When your notebooks appear, the window closes by itself. If you are already signed in, setup skips this step.
4. **Checks everything** (the same checks as `sourcebolt doctor`).

At the end it tells you which apps to restart, for example:

```text
Done. Restart Claude Desktop, Cursor and ask: 'List my NotebookLM notebooks'
```

Restart those apps completely (quit, then start again).

Useful options:

| Option | What it does |
|---|---|
| `--dry-run` | Shows what setup would change, and changes nothing. |
| `--yes` | Answers yes to every question. |
| `--client cursor` | Only this app. Repeat it for more apps. Names: `claude-desktop`, `claude-code`, `cursor`, `vscode`, `windsurf`, `gemini`. |
| `--no-login` | Skips the sign-in step. |
| `--undo` | Puts back each app's settings file from the copy setup saved. |

If the browser window says Google will not let you sign in, or setup cannot find a browser, see [troubleshooting.md](troubleshooting.md#sign-in-problems). The other ways to sign in ([auth.md](auth.md)) still work.

To sign in again later (sessions expire), run only:

```bash
sourcebolt login
```

## Step 3: Ask your first question

In your AI app, type:

> Use sourcebolt to list my NotebookLM notebooks. Then ask the first notebook for a one-paragraph summary of its sources, with citations.

What happens:

1. The assistant calls `notebook_list` and shows your notebooks.
2. It calls `chat_ask` with the notebook and your question.
3. The answer comes back with citations: which source, and the exact text that supports each point.

The first answer usually takes a few seconds to a minute. Almost all of that time is NotebookLM thinking, not sourcebolt.

You can also try a tool straight from the terminal, without an AI app:

```bash
sourcebolt call notebook_list '{}'
```

## Doing it by hand

You do not need this if setup worked. It is for apps setup does not know, and for people who prefer to edit settings themselves.

1. **Sign in.** Run `sourcebolt login`. If Google refuses the window it opens (or you have v0.2.0), use the attach method in [auth.md](auth.md#attach-flow-in-detail): you start the browser yourself, sign in, then run `sourcebolt login --attach http://127.0.0.1:9222`.
2. **Check it.** Run `sourcebolt doctor`. Every check line should start with `ok`. A `FAIL` on `os keyring` alone is fine: the encryption key is then kept in a private file instead.
3. **Connect your app.** For Claude Code:

   ```bash
   claude mcp add --scope user sourcebolt -- sourcebolt serve
   ```

   For Claude Desktop, Cursor and Windsurf, add this to the app's MCP settings file, then restart the app:

   ```json
   {
     "mcpServers": {
       "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
     }
   }
   ```

   VS Code, Codex, Gemini CLI, ChatGPT and where each file lives: [clients.md](clients.md).

## What to try next

- "Add https://example.com/article to my *Test* notebook." (`source_add`)
- "Research 'home battery recycling methods in Europe' and show me the candidate sources." (`research_start`, then `research_status`)
- "Create a note called *Summary* in my *Test* notebook with the three main points." (`note_save`)
- "What is still processing in my *Test* notebook?" (`source_list`)

Every tool and option: [tools.md](tools.md). How an agent should pick tools: [SKILL.md](../SKILL.md).

## When something goes wrong

- Any error with `AUTH_REQUIRED`: your session expired. Run `sourcebolt login` and sign in again.
- Other errors: [troubleshooting.md](troubleshooting.md).
