# Let your AI install it

You can ask the AI app you already use to install sourcebolt for you. Copy the prompt for your app below, paste it into a new chat, and send it. The AI runs the commands and tells you what each one does. You only do two things yourself: say "yes" when it asks, and sign in to Google in the browser window that opens.

Prefer to do it yourself? Pick your app in the [step-by-step guides](guides/index.md).

## What the AI will do, and what you do

| Step | The AI does | You do |
|---|---|---|
| 1. Install | Runs the one-line installer. It downloads sourcebolt from GitHub and checks that the file is genuine. | Say yes when the AI asks to run it. |
| 2. Connect | Runs `sourcebolt setup` for your app. It adds sourcebolt to the app's settings and keeps a backup of the old settings. | Answer the setup questions, or say yes when the AI asks. |
| 3. Sign in | Runs `sourcebolt login`. A normal browser window opens. | Sign in to Google in that window. Type your password there, never in the chat. |
| 4. Check | Runs `sourcebolt doctor`. Every line should start with `ok`. | Restart your AI app when the AI tells you to. |
| 5. Try it | Nothing. | Ask: "List my NotebookLM notebooks." |

## Safety first

- **Your Google password never goes into the chat.** You type it only into Google's own sign-in page in the browser window. If an AI asks you for your password, a code from your phone, or cookies, say no.
- **Read before you agree.** Each prompt asks the AI to explain every command before it runs it. If an explanation does not make sense, ask the AI to stop.
- **The installer is public.** You can read it before you run it: [install.ps1](https://github.com/candidloom-hub/sourcebolt/blob/main/install.ps1) (Windows) and [install.sh](https://github.com/candidloom-hub/sourcebolt/blob/main/install.sh) (macOS and Linux). Both download only from this project's GitHub releases.
- **Everything can be undone.** `sourcebolt setup --undo` puts your app's settings back the way they were. See "Undo" in your app's guide.
- sourcebolt is an independent project, not made by Google. Read [Limits and risk](../README.md#limits-and-risk) first.

## The commands the AI will use

You do not need to type these. They are here so you can check what the AI runs.

1. Install, on Windows (PowerShell):

   ```powershell
   irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   ```

   Install, on macOS or Linux:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   ```

2. Connect one app: `sourcebolt setup --client <app>`
3. Sign in: `sourcebolt login`
4. Check: `sourcebolt doctor`

## Claude Code

Claude Code can run commands on your computer, so it can do almost everything. Open Claude Code in any folder and paste:

```text
Please install the sourcebolt MCP server for me and connect it to Claude Code. I am not technical, so before you run each command, tell me in one plain sentence what it does, then wait for me to say yes.

1. Detect my operating system. On Windows, run this in PowerShell:
   irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   On macOS or Linux, run:
   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   If the installer asks whether to run setup now, answer no. You will run it next.
   If "sourcebolt" is not found afterwards, use the full path the installer printed.
2. Run: sourcebolt setup --client claude-code
   If "--client claude-code" is not accepted, run "sourcebolt setup --help" and use the name it lists for Claude Code.
3. Run: sourcebolt login
   It opens a normal browser window. Tell me to sign in to Google there, and wait until the command finishes by itself.
4. Run: sourcebolt doctor
   Every check line should start with "ok". If one says FAIL, explain it in plain words.
5. Tell me to restart Claude Code, then to ask: "List my NotebookLM notebooks."

Rules: never ask me for my Google password, a verification code or cookies, and never type them anywhere. I sign in only in the browser window. If a command asks a question you cannot answer in your terminal, stop and give me the exact command to run in my own terminal. Do not change anything else on my computer.
```

Step-by-step guide without AI: [Claude Code](guides/claude-code.md).

## Claude Desktop

The Claude Desktop chat cannot run commands on your computer. So this prompt asks Claude to walk you through it, one small step at a time. There are two ways:

- **One double-click.** Download the `.mcpb` file for your computer from the [v0.2.0 release](https://github.com/candidloom-hub/sourcebolt/releases/tag/v0.2.0) and double-click it. You still run two commands once, to sign in.
- **Two commands.** Run the installer, then `sourcebolt setup --client claude-desktop`, which also signs you in.

Paste this into a new Claude Desktop chat:

```text
Please guide me, step by step, to install the sourcebolt MCP server and connect it to this Claude Desktop app. I am not technical. Give me one step at a time, tell me exactly what I should see after it, and wait for me to say "done" before the next step. You cannot run commands yourself, so give me each command to copy and paste.

1. Ask me whether I use Windows or a Mac, then tell me how to open PowerShell (Windows) or Terminal (Mac).
2. Give me the installer command for my system and explain in one sentence what it does:
   Windows: irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   Mac: curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   If it asks whether to run setup now, I should answer no.
3. Give me: sourcebolt setup --client claude-desktop
   Explain that it adds sourcebolt to Claude Desktop's settings and keeps a backup. If it does not accept "--client claude-desktop", have me run "sourcebolt setup --help" and pick the Claude Desktop name it lists.
4. Give me: sourcebolt login
   Explain that a browser window opens, that I sign in to Google there, and that the command finishes by itself.
5. Give me: sourcebolt doctor
   Every line should start with "ok". If I paste you a FAIL line, explain it in plain words.
6. Tell me to quit Claude Desktop completely and open it again, then to ask: "List my NotebookLM notebooks."

Rules: never ask me for my Google password, a verification code or cookies. I type my password only on Google's sign-in page in the browser. If I paste something that looks like a password or cookie, tell me to delete it.
```

Step-by-step guide without AI: [Claude Desktop](guides/claude-desktop.md).

## Cursor

Cursor's agent can run commands in its terminal. Open the Agent chat in Cursor and paste:

```text
Please install the sourcebolt MCP server for me and connect it to Cursor. I am not technical, so before you run each command, tell me in one plain sentence what it does, then wait for me to say yes.

1. Detect my operating system. On Windows, run this in PowerShell:
   irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   On macOS or Linux, run:
   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   If the installer asks whether to run setup now, answer no. You will run it next.
   If "sourcebolt" is not found afterwards, use the full path the installer printed.
2. Run: sourcebolt setup --client cursor
   If "--client cursor" is not accepted, run "sourcebolt setup --help" and use the name it lists for Cursor.
3. Run: sourcebolt login
   It opens a normal browser window. Tell me to sign in to Google there, and wait until the command finishes by itself.
4. Run: sourcebolt doctor
   Every check line should start with "ok". If one says FAIL, explain it in plain words.
5. Tell me to check that sourcebolt is switched on in Cursor's MCP settings, then to ask in a new chat: "List my NotebookLM notebooks."

Rules: never ask me for my Google password, a verification code or cookies, and never type them anywhere. I sign in only in the browser window. If a command asks a question you cannot answer in your terminal, stop and give me the exact command to run in my own terminal. Do not change anything else on my computer.
```

Step-by-step guide without AI: [Cursor](guides/cursor.md).

## VS Code

Use GitHub Copilot Chat with the **Agent** selected, so it can run commands. Open the Chat view (the **Chat** menu, then **Open Chat**; or Ctrl+Alt+I on Windows, Ctrl+Cmd+I on a Mac) and paste:

```text
Please install the sourcebolt MCP server for me and connect it to VS Code. I am not technical, so before you run each command, tell me in one plain sentence what it does, then wait for me to say yes.

1. Detect my operating system. On Windows, run this in PowerShell:
   irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   On macOS or Linux, run:
   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   If the installer asks whether to run setup now, answer no. You will run it next.
   If "sourcebolt" is not found afterwards, use the full path the installer printed.
2. Run: sourcebolt setup --client vscode
   If "--client vscode" is not accepted, run "sourcebolt setup --help" and use the name it lists for VS Code.
3. Run: sourcebolt login
   It opens a normal browser window. Tell me to sign in to Google there, and wait until the command finishes by itself.
4. Run: sourcebolt doctor
   Every check line should start with "ok". If one says FAIL, explain it in plain words.
5. Tell me to run "MCP: List Servers" from the Command Palette, check that sourcebolt is there and started, then ask in a new chat: "List my NotebookLM notebooks."

Rules: never ask me for my Google password, a verification code or cookies, and never type them anywhere. I sign in only in the browser window. If a command asks a question you cannot answer in your terminal, stop and give me the exact command to run in my own terminal. Do not change anything else on my computer.
```

Step-by-step guide without AI: [VS Code](guides/vscode.md).

## Windsurf

Windsurf's Cascade agent can run commands. Open Cascade and paste:

```text
Please install the sourcebolt MCP server for me and connect it to Windsurf. I am not technical, so before you run each command, tell me in one plain sentence what it does, then wait for me to say yes.

1. Detect my operating system. On Windows, run this in PowerShell:
   irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   On macOS or Linux, run:
   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   If the installer asks whether to run setup now, answer no. You will run it next.
   If "sourcebolt" is not found afterwards, use the full path the installer printed.
2. Run: sourcebolt setup --client windsurf
   If "--client windsurf" is not accepted, run "sourcebolt setup --help" and use the name it lists for Windsurf.
3. Run: sourcebolt login
   It opens a normal browser window. Tell me to sign in to Google there, and wait until the command finishes by itself.
4. Run: sourcebolt doctor
   Every check line should start with "ok". If one says FAIL, explain it in plain words.
5. Tell me to check that sourcebolt is listed and switched on in the MCPs section of Cascade, then to ask in a new chat: "List my NotebookLM notebooks."

Rules: never ask me for my Google password, a verification code or cookies, and never type them anywhere. I sign in only in the browser window. If a command asks a question you cannot answer in your terminal, stop and give me the exact command to run in my own terminal. Do not change anything else on my computer.
```

Step-by-step guide without AI: [Windsurf](guides/windsurf.md).

## Gemini CLI

Gemini CLI can run commands. Start `gemini` in a terminal and paste:

```text
Please install the sourcebolt MCP server for me and connect it to Gemini CLI. I am not technical, so before you run each command, tell me in one plain sentence what it does, then wait for me to say yes.

1. Detect my operating system. On Windows, run this in PowerShell:
   irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   On macOS or Linux, run:
   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   If the installer asks whether to run setup now, answer no. You will run it next.
   If "sourcebolt" is not found afterwards, use the full path the installer printed.
2. Run: sourcebolt setup --client gemini-cli
   If "--client gemini-cli" is not accepted, run "sourcebolt setup --help" and use the name it lists for Gemini CLI.
3. Run: sourcebolt login
   It opens a normal browser window. Tell me to sign in to Google there, and wait until the command finishes by itself.
4. Run: sourcebolt doctor
   Every check line should start with "ok". If one says FAIL, explain it in plain words.
5. Tell me to quit Gemini CLI, start it again, type /mcp to see that sourcebolt is connected, then ask: "List my NotebookLM notebooks."

Rules: never ask me for my Google password, a verification code or cookies, and never type them anywhere. I sign in only in the browser window. If a command asks a question you cannot answer in your terminal, stop and give me the exact command to run in my own terminal. Do not change anything else on my computer.
```

Step-by-step guide without AI: [Gemini CLI](guides/gemini-cli.md).

## ChatGPT desktop app

The ChatGPT chat cannot run commands on your computer, and `sourcebolt setup` does not know the ChatGPT app yet. So this prompt asks ChatGPT to walk you through it. ChatGPT on the web cannot use sourcebolt at all; you need the desktop app.

Paste this into a new chat in the ChatGPT desktop app:

```text
Please guide me, step by step, to install the sourcebolt MCP server and add it to this ChatGPT desktop app as a local (STDIO) MCP server. I am not technical. Give me one step at a time, tell me exactly what I should see after it, and wait for me to say "done" before the next step. You cannot run commands yourself, so give me each command to copy and paste.

1. Ask me whether I use Windows or a Mac, then tell me how to open PowerShell (Windows) or Terminal (Mac).
2. Give me the installer command for my system and explain in one sentence what it does:
   Windows: irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
   Mac: curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
   If it asks whether to run setup now, I should answer no.
3. Give me: sourcebolt login
   Explain that a browser window opens, that I sign in to Google there, and that the command finishes by itself.
4. Give me: sourcebolt doctor
   Every line should start with "ok". If I paste you a FAIL line, explain it in plain words.
5. Walk me through adding the server in this app: Settings, MCP servers, Add server. Name: sourcebolt. Type: STDIO. Command: sourcebolt. Argument: serve. Then Save and Restart.
6. Tell me to type /mcp in a new chat to see that sourcebolt is connected, then to ask: "List my NotebookLM notebooks."

Rules: never ask me for my Google password, a verification code or cookies. I type my password only on Google's sign-in page in the browser. If I paste something that looks like a password or cookie, tell me to delete it.
```

Step-by-step guide without AI: [ChatGPT desktop](guides/chatgpt-desktop.md).

## If something goes wrong

- The AI says `sourcebolt` is "not found": close the terminal or app and open it again, so it sees the newly installed program. Or ask the AI to use the full path the installer printed.
- `sourcebolt login` never finishes: make sure you finished signing in, and that you can see your notebooks at https://notebook.google.com in that window.
- `sourcebolt doctor` shows `FAIL`: see [Troubleshooting](troubleshooting.md).
- Your app does not show sourcebolt: quit the app completely and open it again.
