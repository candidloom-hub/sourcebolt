# FAQ

## About the project

### What is sourcebolt?

A small program that lets AI apps (Claude, Cursor, VS Code, Codex and others) work with your Google NotebookLM notebooks. It is an MCP server. MCP, the Model Context Protocol, is the standard way AI apps call outside tools.

### Is it made by Google?

No. sourcebolt is an independent open-source project. It is not affiliated with, endorsed by or sponsored by Google. "NotebookLM" is a trademark of Google LLC.

### Is it free?

Yes. It is open source under the Apache-2.0 license, with no paid tier. Donations are not open yet. Starring the repository and sharing it helps.

### Google renamed NotebookLM. Does sourcebolt still work?

Yes. sourcebolt uses the current host, `notebook.google.com`, by default. You can change it with `--host` at login or the `NOTEBOOKLM_HOST` environment variable.

## Safety and privacy

### Does sourcebolt see my Google password?

No. You sign in yourself, in a normal browser window. sourcebolt copies only the session cookies after you are signed in. No tool asks for or types a password.

### Can the AI model read my cookies?

No. No tool accepts or returns cookies, tokens or passwords. Signing in works only from the command line. The MCP tools `auth_login` and `auth_logout` only tell you which command to run.

### Where are my cookies stored?

On your computer, encrypted with AES-256-GCM. The key is in your operating system's keyring (Windows Credential Manager, macOS Keychain, Linux Secret Service), or in a file only you can read when there is no keyring. Details: [auth.md](auth.md#where-data-is-stored).

### Does sourcebolt send my data anywhere else?

No. It talks only to NotebookLM. When you add a web page as a source, Google fetches that page, not your computer. sourcebolt has no telemetry and no analytics.

### Can an agent delete my notebooks by mistake?

`delete` first returns a preview of what it would delete and changes nothing. It acts only when called again with `confirm: true`. Names are matched strictly, so a vague title does not match several notebooks. `share_update` also previews before it widens access or removes people. A well-behaved agent shows you the preview and waits for your yes; still, read previews before you approve them.

### Can I get my account in trouble?

Possibly. sourcebolt uses NotebookLM's internal web calls with your session. Automating a Google web app may break Google's terms and could lead to rate limits or account action. Use it at your own risk, ideally with an account you can afford to lose access to while you evaluate it.

## Using it

### Which Google accounts work?

Any account that can open NotebookLM in a browser. sourcebolt does not use the NotebookLM Enterprise API (Google Cloud).

### Can I use more than one Google account?

Yes, with named profiles. Sign in once per account:

```bash
sourcebolt login --attach http://127.0.0.1:9222 --profile work
```

Then start the server for that account with `sourcebolt serve --profile work`, or set `NOTEBOOKLM_PROFILE=work`.

### Can I use it without an AI app?

Yes. `sourcebolt call` runs any tool from the terminal and prints JSON:

```bash
sourcebolt call notebook_list '{}'
```

This is handy for scripts.

### Can it upload PDFs or Google Drive files?

Not yet. Today `source_add` takes public web pages, YouTube links and pasted text. Drive and file sources return `UNSUPPORTED_BY_TRANSPORT` until their request format is confirmed with a live test.

### Can it make audio overviews, videos or slide decks?

Not yet. `studio_list` lists all your Studio artifacts. In v0.1.0, `studio_generate` only checks your quota. Newer builds generate flashcards. Other kinds will follow once their request formats are confirmed. See [tools.md](tools.md#studio_generate).

### Why do some tools say "not available yet"?

NotebookLM has no public API. Every request format must match what the NotebookLM website sends. sourcebolt only enables a request after it has been checked against the real website ("live capture"). Until then the tool returns `UNSUPPORTED_BY_TRANSPORT` rather than guessing and maybe changing the wrong thing.

### My question takes a long time. Is sourcebolt slow?

Almost all of the time is NotebookLM generating the answer. sourcebolt's own work is a few milliseconds per call ([performance.md](performance.md)). For long answers, an agent can ask with `wait: false` and check back with `chat_status`.

### Does it need Node.js, Python or a browser?

No Node.js and no Python. It is one program. A Chrome-family browser (Brave, Chrome, Edge) is needed only to sign in, and optionally for the fallback paths described in [auth.md](auth.md#browser-fallback).

### How do I update?

Download the new release the same way you installed it, and replace the old file. With Go: run the `go install` command again. With Docker: change the image tag. Your sign-in is kept.

### How do I uninstall?

1. Delete your saved sessions: `sourcebolt logout` (add `--profile <name>` for other profiles).
2. Remove sourcebolt from your AI app's config.
3. Delete the program file.
4. Optionally delete the config folder: `%USERPROFILE%\.sourcebolt` (Windows; versions up to 0.3.1 used `%APPDATA%\sourcebolt`), `~/Library/Application Support/sourcebolt` (macOS) or `~/.config/sourcebolt` (Linux).

To end the Google session itself, sign out of Google in the browser you used, or from your Google account's security page.

## More help

- Errors: [troubleshooting.md](troubleshooting.md)
- Setup: [getting-started.md](getting-started.md), [clients.md](clients.md)
- How it compares with other NotebookLM servers: [comparison.md](comparison.md)
