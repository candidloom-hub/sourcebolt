# Set up sourcebolt in Windsurf

<!-- Vendor facts checked 2026-10-21. docs.windsurf.com/windsurf/cascade/mcp redirects (307) to
     https://docs.devin.ai/desktop/cascade/mcp, which describes "Devin Desktop" with Cascade as the legacy agent:
     "..." (Actions) menu top right of the Cascade panel > MCPs section > "Open MCP config file" icon; toggle per server;
     ~/.config/devin/mcp_config.json (macOS, Linux) and %APPDATA%\devin\mcp_config.json (Windows).
     Older Windsurf versions used ~/.codeium/windsurf/mcp_config.json (from docs/clients.md, checked 2026-10-20). -->

This guide connects sourcebolt to Windsurf and its Cascade agent. Then you can ask Cascade about your NotebookLM notebooks.

Windsurf's own help pages now live under the name Devin Desktop. The steps below work for both names.

Choose one way:

1. [Easiest: one command](#easiest-one-command) (about 2 minutes)
2. [Let Cascade do it](#let-cascade-do-it)
3. [Manual setup](#manual-setup)

## Easiest: one command

### Step 1: Open a terminal

- **Windows:** click **Start**, type `PowerShell`, then click **Windows PowerShell**.
- **Mac:** press Command and Space together, type `Terminal`, then press Return.
- **Linux:** press Ctrl, Alt and T together on most systems.

You should see a window with a blinking cursor.

![Step 1: an empty terminal window](../assets/guides/windsurf/step1.png)
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

![Step 2: the installer finished and shows the installed version](../assets/guides/windsurf/step2.png)
<!-- screenshot: terminal after the one-line installer ran: download and checksum lines, the "Installed: sourcebolt 0.x.y" line, and the question asking whether to run sourcebolt setup now -->

### Step 3: Connect Windsurf

The installer asks whether to run setup now. Type `y` and press Enter. (Missed it? Type `sourcebolt setup` and press Enter.)

Setup lists the AI apps it found. When it asks about **Windsurf**, answer yes. It saves a backup of Windsurf's MCP settings file before it changes it.

![Step 3: sourcebolt setup asking whether to connect Windsurf](../assets/guides/windsurf/step3.png)
<!-- screenshot: terminal running sourcebolt setup: the list of detected apps with Windsurf in it, the yes/no question for Windsurf answered "y", and the backup confirmation line -->

### Step 4: Sign in to Google

Setup opens a normal browser window on the Google sign-in page.

1. Sign in to Google in that window. Type your password there, and nowhere else.
2. Wait. When NotebookLM has loaded, the terminal shows a line that starts with `Saved profile`.

![Step 4: the terminal confirms the saved sign-in](../assets/guides/windsurf/step4.png)
<!-- screenshot: terminal after sign-in finished, showing 'Saved profile "default" for notebook.google.com (N notebooks visible).'. Blur the account name. -->

### Step 5: Turn sourcebolt on in Cascade

1. Quit Windsurf and open it again.
2. In the Cascade panel, click the **...** menu at the top right.
3. In the **MCPs** section, find **sourcebolt** and make sure its switch is on.

![Step 5: sourcebolt switched on in the MCPs section of Cascade](../assets/guides/windsurf/step5.png)
<!-- screenshot: Windsurf with the Cascade panel's "..." (Actions) menu open, the MCPs section listing sourcebolt with its toggle on and its tool count -->

Now go to [How to check it works](#how-to-check-it-works).

## Let Cascade do it

Cascade can run the commands above for you and explain each one first. Copy the Windsurf prompt from [Let your AI install it](../install-with-ai.md#windsurf) and paste it into Cascade.

## Manual setup

1. Install sourcebolt and sign in: do steps 1, 2 and 4 above, but answer `n` when the installer asks about setup. Then run:

   ```bash
   sourcebolt login
   ```

2. In the Cascade panel, click the **...** menu at the top right. In the **MCPs** section, click the **Open MCP config file** icon. This opens `mcp_config.json`.
3. If the file is empty, paste this. If it already has an `"mcpServers"` section, add only the `"sourcebolt"` line inside it.

   ```json
   {
     "mcpServers": {
       "sourcebolt": { "command": "sourcebolt", "args": ["serve"] }
     }
   }
   ```

4. Save the file, then restart Windsurf and check that sourcebolt is switched on (step 5 above).

More options: [Connect your AI app](../clients.md#windsurf).

## How to check it works

Start a new Cascade chat and type:

> List my NotebookLM notebooks.

Allow the sourcebolt tool if Cascade asks. You should see your notebook names.

![Check: Cascade listing NotebookLM notebooks through sourcebolt](../assets/guides/windsurf/check.png)
<!-- screenshot: Windsurf Cascade chat with the prompt "List my NotebookLM notebooks.", the sourcebolt notebook_list tool call, and the reply listing notebooks. Use a test account or blur the titles. -->

You can also type `sourcebolt doctor` in a terminal. Every line should start with `ok`.

## Undo / uninstall

1. **Remove sourcebolt from Windsurf.** If you used setup:

   ```bash
   sourcebolt setup --undo
   ```

   If you added it yourself, open `mcp_config.json` again (manual step 2) and delete the `"sourcebolt"` line, or switch it off in the **MCPs** section.
2. **Delete your saved sign-in:** `sourcebolt logout`. This does not sign you out of Google.
3. **Delete the program.** If you used the one-line installer, run it again with the uninstall option. Windows (PowerShell):

   ```powershell
   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1))) -Uninstall
   ```

   Mac and Linux:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh -s -- --uninstall
   ```

   This also removes the PATH entry the installer added. Installed it another way (Homebrew, npm, Docker)? See [Install sourcebolt](../install.md) for its undo line.

4. Restart Windsurf.

## Something went wrong?

- `sourcebolt` is "not recognized" or "not found": close the terminal, open a new one, and try again.
- sourcebolt does not appear in the **MCPs** section: check the file for a missing comma or bracket, save it, and restart Windsurf.
- Anything else: [Troubleshooting](../troubleshooting.md).
