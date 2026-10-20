# Set up sourcebolt in Gemini CLI

<!-- Vendor facts checked 2026-10-21 at https://geminicli.com/docs/tools/mcp-server/ :
     gemini mcp add [-s user] <name> <command> [args...]; gemini mcp list; gemini mcp remove <name> [-s scope];
     /mcp, /mcp list, /mcp enable|disable <name>; ~/.gemini/settings.json (user), .gemini/settings.json (project);
     stdio servers show "Disconnected" in untrusted folders.
     The page says Gemini CLI was replaced by Antigravity CLI on 2026-06-18 for unpaid and Google One users. -->

This guide connects sourcebolt to Gemini CLI, Google's AI assistant for the terminal. Then you can ask Gemini about your NotebookLM notebooks.

Note: Google's docs say Gemini CLI was replaced by Antigravity CLI on 18 June 2026 for free and Google One accounts. This guide is for people who still use Gemini CLI.

Choose one way:

1. [Easiest: one command](#easiest-one-command) (about 2 minutes)
2. [Let Gemini do it](#let-gemini-do-it)
3. [Manual setup](#manual-setup)

## Easiest: one command

### Step 1: Open a terminal

- **Windows:** click **Start**, type `PowerShell`, then click **Windows PowerShell**.
- **Mac:** press Command and Space together, type `Terminal`, then press Return.
- **Linux:** press Ctrl, Alt and T together on most systems.

You should see a window with a blinking cursor. Use a new window, not one where Gemini CLI is running.

![Step 1: an empty terminal window](../assets/guides/gemini-cli/step1.png)
<!-- screenshot: a fresh Windows PowerShell window opened from the Start menu, prompt visible, nothing typed yet -->

### Step 2: Install sourcebolt

Copy the line for your computer, paste it into the terminal, and press Enter.

Windows:

```powershell
irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
```

Mac and Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
```

The installer downloads sourcebolt, checks that the file is genuine, and installs it for your user only. You should see a line saying sourcebolt was installed, with its version number.

![Step 2: the installer finished and shows the installed version](../assets/guides/gemini-cli/step2.png)
<!-- screenshot: terminal after the one-line installer ran: download and checksum lines, the "Installed: sourcebolt 0.x.y" line, and the question asking whether to run sourcebolt setup now -->

### Step 3: Connect Gemini CLI

The installer asks whether to run setup now. Type `y` and press Enter. (Missed it? Type `sourcebolt setup` and press Enter.)

Setup lists the AI apps it found. When it asks about **Gemini CLI**, answer yes. It saves a backup of Gemini CLI's settings file before it changes it.

![Step 3: sourcebolt setup asking whether to connect Gemini CLI](../assets/guides/gemini-cli/step3.png)
<!-- screenshot: terminal running sourcebolt setup: the list of detected apps with Gemini CLI in it, the yes/no question for Gemini CLI answered "y", and the backup confirmation line -->

### Step 4: Sign in to Google

Setup opens a normal browser window on the Google sign-in page.

1. Sign in to Google in that window. Type your password there, and nowhere else.
2. Wait. When NotebookLM has loaded, the terminal shows a line that starts with `Saved profile`.

![Step 4: the terminal confirms the saved sign-in](../assets/guides/gemini-cli/step4.png)
<!-- screenshot: terminal after sign-in finished, showing 'Saved profile "default" for notebook.google.com (N notebooks visible).'. Blur the account name. -->

### Step 5: Check the connection in Gemini CLI

Start Gemini CLI by typing `gemini` and pressing Enter. (If it was already running, quit it first and start it again.) Then type:

```text
/mcp
```

You should see **sourcebolt** in the list, connected, with its tools.

If it says **Disconnected**, Gemini CLI may not trust the folder you started it in. Start `gemini` in your home folder, or trust the folder when Gemini asks.

![Step 5: /mcp in Gemini CLI shows sourcebolt connected](../assets/guides/gemini-cli/step5.png)
<!-- screenshot: Gemini CLI after typing /mcp: the MCP server list with sourcebolt shown as connected and its tool names -->

Now go to [How to check it works](#how-to-check-it-works).

## Let Gemini do it

Gemini CLI can run the commands above for you and explain each one first. Copy the Gemini CLI prompt from [Let your AI install it](../install-with-ai.md#gemini-cli) and paste it into Gemini CLI.

## Manual setup

1. Install sourcebolt and sign in: do steps 1, 2 and 4 above, but answer `n` when the installer asks about setup. Then run:

   ```bash
   sourcebolt login
   ```

2. Add sourcebolt for your user:

   ```bash
   gemini mcp add -s user sourcebolt sourcebolt serve
   ```

   Or add it to `~/.gemini/settings.json` yourself. If the file already has an `"mcpServers"` section, add only the `"sourcebolt"` line inside it.

   ```json
   {
     "mcpServers": {
       "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
     }
   }
   ```

3. Check with `gemini mcp list`, or `/mcp` inside Gemini CLI.

More options: [Connect your AI app](../clients.md#gemini-cli).

## How to check it works

In Gemini CLI, type:

> List my NotebookLM notebooks.

Gemini asks before it runs a sourcebolt tool. Allow it. You should see your notebook names.

![Check: Gemini CLI listing NotebookLM notebooks through sourcebolt](../assets/guides/gemini-cli/check.png)
<!-- screenshot: Gemini CLI with the prompt "List my NotebookLM notebooks.", the allowed sourcebolt notebook_list tool call, and the reply listing notebooks. Use a test account or blur the titles. -->

You can also type `sourcebolt doctor` in a terminal. Every line should start with `ok`.

## Undo / uninstall

1. **Remove sourcebolt from Gemini CLI.** If you used setup:

   ```bash
   sourcebolt setup --undo
   ```

   If you added it yourself: `gemini mcp remove -s user sourcebolt`.
2. **Delete your saved sign-in:** `sourcebolt logout`. This does not sign you out of Google.
3. **Delete the program.** Windows (PowerShell):

   ```powershell
   Remove-Item (Get-Command sourcebolt).Source
   ```

   Mac and Linux:

   ```bash
   rm "$(command -v sourcebolt)"
   ```

## Something went wrong?

- `sourcebolt` is "not recognized" or "not found": close the terminal, open a new one, and try again.
- `/mcp` shows sourcebolt as disconnected: see step 5, then run `sourcebolt doctor` and read the first `FAIL` line.
- Anything else: [Troubleshooting](../troubleshooting.md).
