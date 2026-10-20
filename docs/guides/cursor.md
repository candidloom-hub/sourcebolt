# Set up sourcebolt in Cursor

<!-- Vendor facts checked 2026-10-21:
     MCP servers are managed from "Customize" in the sidebar; toggle per server; ~/.cursor/mcp.json (global),
     .cursor/mcp.json (project); approval prompt before MCP tool runs; Output panel "MCP Logs" (Cmd/Ctrl+Shift+U):
       https://cursor.com/docs/context/mcp
     Install link format: https://cursor.com/docs/context/mcp/install-links -->

This guide connects sourcebolt to the Cursor code editor. Then you can ask Cursor's agent about your NotebookLM notebooks.

Choose one way:

1. [Easiest: one command](#easiest-one-command) (about 2 minutes)
2. [Let Cursor do it](#let-cursor-do-it)
3. [Manual setup](#manual-setup)

## Easiest: one command

### Step 1: Open a terminal

- **Windows:** click **Start**, type `PowerShell`, then click **Windows PowerShell**.
- **Mac:** press Command and Space together, type `Terminal`, then press Return.
- **Linux:** press Ctrl, Alt and T together on most systems.

You should see a window with a blinking cursor.

![Step 1: an empty terminal window](../assets/guides/cursor/step1.png)
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

![Step 2: the installer finished and shows the installed version](../assets/guides/cursor/step2.png)
<!-- screenshot: terminal after the one-line installer ran: download and checksum lines, the "Installed: sourcebolt 0.x.y" line, and the question asking whether to run sourcebolt setup now -->

### Step 3: Connect Cursor

The installer asks whether to run setup now. Type `y` and press Enter. (Missed it? Type `sourcebolt setup` and press Enter.)

Setup lists the AI apps it found. When it asks about **Cursor**, answer yes. It saves a backup of Cursor's MCP settings file before it changes it.

![Step 3: sourcebolt setup asking whether to connect Cursor](../assets/guides/cursor/step3.png)
<!-- screenshot: terminal running sourcebolt setup: the list of detected apps with Cursor in it, the yes/no question for Cursor answered "y", and the backup confirmation line -->

### Step 4: Sign in to Google

Setup opens a normal browser window on the Google sign-in page.

1. Sign in to Google in that window. Type your password there, and nowhere else.
2. Wait. When NotebookLM has loaded, the terminal shows a line that starts with `Saved profile`.

![Step 4: the terminal confirms the saved sign-in](../assets/guides/cursor/step4.png)
<!-- screenshot: terminal after sign-in finished, showing 'Saved profile "default" for notebook.google.com (N notebooks visible).'. Blur the account name. -->

### Step 5: Turn sourcebolt on in Cursor

1. Quit Cursor and open it again.
2. Open **Customize** in the sidebar and find the MCP servers.
3. Find **sourcebolt** and make sure its switch is on.

![Step 5: sourcebolt switched on in Cursor's MCP list](../assets/guides/cursor/step5.png)
<!-- screenshot: Cursor with Customize open in the sidebar, the MCP servers list showing sourcebolt with its toggle on and its tools available -->

Now go to [How to check it works](#how-to-check-it-works).

## Let Cursor do it

Cursor's agent can run the commands above for you and explain each one first. Copy the Cursor prompt from [Let your AI install it](../install-with-ai.md#cursor) and paste it into Cursor's agent chat.

## Manual setup

1. Install sourcebolt and sign in: do steps 1, 2 and 4 above, but answer `n` when the installer asks about setup. Then run:

   ```bash
   sourcebolt login
   ```

2. Add sourcebolt to Cursor. Either:

   - **Install link:** copy this link into your browser's address bar and press Enter. Your browser asks to open Cursor; allow it, then click **Install** in Cursor.

     ```text
     cursor://anysphere.cursor-deeplink/mcp/install?name=sourcebolt&config=eyJjb21tYW5kIjoic291cmNlYm9sdCIsImFyZ3MiOlsic2VydmUiXX0=
     ```

   - **Settings file:** open `~/.cursor/mcp.json` (in your home folder, inside the `.cursor` folder) in a text editor. If it is empty, paste this. If it already has an `"mcpServers"` section, add only the `"sourcebolt"` line inside it.

     ```json
     {
       "mcpServers": {
         "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
       }
     }
     ```

3. Restart Cursor and check that sourcebolt is switched on (step 5 above).

More options: [Connect your AI app](../clients.md#cursor) and [one-click install links](../install-links.md).

## How to check it works

Open a new agent chat in Cursor and type:

> List my NotebookLM notebooks.

Cursor asks for approval before it runs a sourcebolt tool. Approve it. You should see your notebook names.

![Check: Cursor's agent listing NotebookLM notebooks through sourcebolt](../assets/guides/cursor/check.png)
<!-- screenshot: Cursor agent chat with the prompt "List my NotebookLM notebooks.", the approved sourcebolt notebook_list tool call, and the reply listing notebooks. Use a test account or blur the titles. -->

You can also type `sourcebolt doctor` in a terminal. Every line should start with `ok`.

## Undo / uninstall

1. **Remove sourcebolt from Cursor.** If you used setup:

   ```bash
   sourcebolt setup --undo
   ```

   If you added it yourself, delete the `"sourcebolt"` line from `~/.cursor/mcp.json`, or switch it off in **Customize**.
2. **Delete your saved sign-in:** `sourcebolt logout`. This does not sign you out of Google.
3. **Delete the program.** Windows (PowerShell):

   ```powershell
   Remove-Item (Get-Command sourcebolt).Source
   ```

   Mac and Linux:

   ```bash
   rm "$(command -v sourcebolt)"
   ```

4. Restart Cursor.

## Something went wrong?

- `sourcebolt` is "not recognized" or "not found": close the terminal, open a new one, and try again.
- sourcebolt shows an error in Cursor: open the Output panel (Ctrl+Shift+U, or Cmd+Shift+U on a Mac), choose **MCP Logs**, and read the last lines.
- Anything else: [Troubleshooting](../troubleshooting.md).
