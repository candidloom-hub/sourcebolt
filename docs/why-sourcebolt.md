# Why sourcebolt

There are several ways to connect an AI app to NotebookLM. This page explains what sourcebolt does well, with the evidence for each point, and when another project may suit you better. For a side-by-side table, see [comparison.md](comparison.md).

## Fast, and light on your computer

- **Starts in about 10 ms** (median 10.3 ms from process start to a ready MCP connection). Apps can start it on demand without a noticeable pause.
- **Small:** one 10 MB file, 12.4 MB of memory while idle.
- **Little overhead per call:** about 1.6 ms of sourcebolt's own work for a cached list of 100 notebooks. A real call to NotebookLM takes most of a second (848 ms median for `notebook_list`), so almost all waiting time is Google's.
- **Reuses connections:** after the first request, every request reuses the same HTTP/2 connection (100% in the benchmark), so there is one TLS handshake, not one per call.

All numbers, the machine they were measured on and how to reproduce them: [performance.md](performance.md).

## One program, nothing else to install

sourcebolt is a single static program written in Go. There is no Node.js, Python or package manager to install, and no dependency tree to keep up to date. You download one file for your system (Windows, macOS or Linux, on x64 or ARM64), or use the Docker image. A Chrome-family browser is needed only to sign in, and optionally for the fallback paths below.

## Keeps working when NotebookLM changes

NotebookLM has no public API, so every unofficial client breaks when Google changes its website. sourcebolt has three layers to soften that:

1. **Direct calls first.** sourcebolt sends NotebookLM's internal web requests directly. This is the fast path.
2. **Browser replay.** If Google rejects a direct request, sourcebolt can send it again from inside a headless browser.
3. **Page fallback.** If Google changes the format of the chat or add-source request, sourcebolt can type into the NotebookLM page and read the answer instead.

Fallbacks are careful: they never run for quota, sign-in or permission errors (another path cannot fix those), and a change that may already have reached Google is not replayed. If the page itself changes, you can override the page controls in a JSON file without waiting for a release. Details and limits: [auth.md](auth.md#browser-fallback).

## Built to be safe with your account

- **Your password never touches sourcebolt.** You sign in yourself in a normal browser. No MCP tool accepts, returns or types cookies or passwords.
- **Encrypted session.** Cookies are stored with AES-256-GCM encryption; the key is in your OS keyring (or a private file if there is none).
- **Secrets stay out of logs.** Cookies, tokens and request bodies are masked in every error and log line.
- **Previews before damage.** `delete` shows what it would delete and acts only with `confirm: true`. `share_update` previews before it widens access or removes people. Names are matched strictly.
- **No double changes.** Requests that create or change things (such as asking a question or creating a notebook) are not sent again automatically once they may have reached Google, so a network hiccup does not create duplicates.
- **URL checks.** Source URLs that point to private or internal network addresses are rejected before they are sent.
- **Local only.** The default connection is stdio. The optional HTTP mode listens only on your own computer and requires a token.
- **Signed releases.** Release checksums carry a keyless cosign signature, and each archive has an SBOM. See [security.md](security.md#verify-a-download).

## Clear errors an agent can act on

Every failure returns the same JSON shape with a stable code (`AUTH_REQUIRED`, `RATE_LIMITED`, `QUOTA`, `DRIFT`, ...), a `retriable` flag, a one-line fix and, when Google sends one, `retry_after_seconds`. Features that are not ready return `UNSUPPORTED_BY_TRANSPORT` instead of guessing. See [troubleshooting.md](troubleshooting.md).

## Covers the everyday NotebookLM work

31 tools cover notebooks, sources (web pages, YouTube, pasted text), chat with citations, web research and import, notes, sharing, labels, Studio artifacts, account quotas and cleanup. Helpful extras:

- **Long answers do not block:** ask with `wait: false`, then check with `chat_status`; progress is streamed while the answer is generated.
- **Tool profiles:** `--tools minimal` exposes 8 core tools for apps with small tool budgets; `--tools all` adds asking several notebooks at once.
- **Use from scripts:** `sourcebolt call <tool> '<json>'` runs any tool from the terminal.

Full list: [tools.md](tools.md).

## Free and open source

Apache-2.0, no paid tier, no telemetry. The code, the benchmarks and the research behind every request format are in the repository.

## When another project may suit you better

sourcebolt is young and deliberately enables a feature only after its request format has been checked against the real website. Today that means:

- **Studio generation** (audio overviews, video, slides, infographics, reports) is not available yet; newer builds generate flashcards only. Several other projects generate many Studio kinds today.
- **Drive and file uploads**, and **downloading** Studio artifacts, are not available yet.
- **A Python library or a large CLI**: sourcebolt is an MCP server with a small CLI, not a library.
- **Rotating several accounts automatically** is not supported; you choose one profile per server.
- **Maturity:** other projects have far larger user communities and longer histories.

[comparison.md](comparison.md) names the projects that cover these needs.
