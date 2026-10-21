# Installing sourcebolt (for AI agents such as Cline)

sourcebolt is a local MCP server that lets the user's AI assistant use their Google NotebookLM notebooks. It is one program with no runtime to install. It runs on the user's computer and talks to NotebookLM with the user's own Google sign-in.

Follow these steps in order. Before each command, tell the user in one plain sentence what it does.

## Rules

- **Never ask the user for their Google password, a verification code or cookies, and never type, read or store them.** The user signs in only on Google's own page, in a browser window that `sourcebolt login` opens. If the user pastes something that looks like a password or cookie, tell them to delete it.
- Do not open or read files in the sourcebolt data folder (`%USERPROFILE%\.sourcebolt` on Windows, or `%APPDATA%\sourcebolt` from older versions, `~/Library/Application Support/sourcebolt` on macOS, `~/.config/sourcebolt` on Linux). They hold the encrypted session.
- Do not change anything on the computer other than what these steps describe.

## Step 1: Detect the operating system

Find out whether this is Windows, macOS or Linux, and use the matching commands below. On Windows, run commands in PowerShell.

## Step 2: Install

Use option A unless the user asks for npm.

### Option A: one-line installer (recommended)

It downloads the release from GitHub, checks its SHA-256 checksum, and installs for the current user only (no admin rights). The `NO_SETUP` setting skips the installer's interactive setup question, because you will configure Cline yourself in step 3.

Windows (PowerShell):

```powershell
$env:SOURCEBOLT_NO_SETUP = '1'; irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
```

macOS and Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh -s -- --no-setup
```

The output ends with a line like `Installed: sourcebolt 0.3.1`. The program is installed at:

- Windows: `%LOCALAPPDATA%\Programs\sourcebolt\sourcebolt.exe`, for example `C:\Users\you\AppData\Local\Programs\sourcebolt\sourcebolt.exe`
- macOS and Linux: `~/.local/bin/sourcebolt`, for example `/Users/you/.local/bin/sourcebolt` or `/home/you/.local/bin/sourcebolt`

Work out the **absolute** path (expand `%LOCALAPPDATA%` or `~`) and check it runs:

```bash
"<full path to sourcebolt>" --version
```

It prints `sourcebolt 0.3.1` or newer. Use this full path in the next steps: a new `PATH` entry only reaches terminals and apps started after the install.

### Option B: npm (needs Node.js 18 or newer)

Nothing to install up front; npm downloads the right program the first time:

```bash
npx -y sourcebolt --version
```

It prints `sourcebolt 0.3.1` or newer.

## Step 3: Add sourcebolt to Cline's MCP settings

Open Cline's MCP settings file (`cline_mcp_settings.json`) and add a `sourcebolt` entry under `mcpServers`. Keep every other entry as it is.

With option A, use the full path from step 2:

```json
{
  "mcpServers": {
    "sourcebolt": {
      "command": "<full path to sourcebolt>",
      "args": ["serve"],
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

On Windows, double each backslash inside JSON, for example `"C:\\Users\\you\\AppData\\Local\\Programs\\sourcebolt\\sourcebolt.exe"`.

With option B:

```json
{
  "mcpServers": {
    "sourcebolt": {
      "command": "npx",
      "args": ["-y", "sourcebolt", "serve"],
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

Optional: for a small tool budget, add `"--tools", "minimal"` after `"serve"` to expose only the 8 core tools.

## Step 4: The user signs in to Google

**Do not run this step yourself.** Ask the user to open their own terminal (PowerShell on Windows, Terminal on macOS) and run:

```bash
"<full path to sourcebolt>" login
```

With option B the command is `npx -y sourcebolt login`.

Tell the user what will happen: a normal browser window (Chrome, Edge or Brave) opens on the Google sign-in page; they sign in there and type their password only there; when their notebooks load, the window closes and the terminal prints a line like:

```text
Saved profile "default" for notebook.google.com (3 notebooks visible).
```

The command waits up to 10 minutes. Closing the browser window cancels it and saves nothing. Wait for the user to say they see the `Saved profile` line.

If no supported browser is installed, or Google refuses the sign-in, point the user to https://github.com/candidloom-hub/sourcebolt/blob/main/docs/auth.md for the other sign-in methods. Do not try to obtain cookies for them.

## Step 5: Verify

Run:

```bash
"<full path to sourcebolt>" status
```

Success: a profile line ending in `valid_cookies=true`, for example:

```text
active profile: default
  default      host=notebook.google.com updated=... key=os keyring valid_cookies=true
```

If it prints `no profiles; run: sourcebolt login`, the sign-in in step 4 did not finish: ask the user to run it again.

For a fuller check, run `"<full path to sourcebolt>" doctor`. Every line should start with `ok`. For a line starting with `FAIL`, explain it in plain words and see https://github.com/candidloom-hub/sourcebolt/blob/main/docs/troubleshooting.md.

Then make sure the `sourcebolt` server shows as connected in Cline's MCP servers view (restart it there if needed), and suggest the user asks: "List my NotebookLM notebooks." That calls the `notebook_list` tool.

## Undo

- Remove the `sourcebolt` entry from `cline_mcp_settings.json`.
- Uninstall (option A). Windows: `& ([scriptblock]::Create((irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1))) -Uninstall`. macOS and Linux: `curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh -s -- --uninstall`. Both keep the sign-in data; only the user should decide to delete it (`-Purge` / `--purge`, which asks first).

## More

- Install options: https://github.com/candidloom-hub/sourcebolt/blob/main/docs/install.md
- Tools: https://github.com/candidloom-hub/sourcebolt/blob/main/docs/tools.md
- sourcebolt is an independent project, not made or endorsed by Google. NotebookLM has no public API for personal accounts, so sourcebolt uses the same internal web calls the NotebookLM website makes, and it can break when Google changes the site.
