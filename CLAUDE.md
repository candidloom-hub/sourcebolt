# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project goal

Build one NotebookLM MCP server that combines the features of the existing NotebookLM MCP implementations, improves on them, and reaches a public release.

The work has three phases:

1. **Recon.** Find every public NotebookLM MCP implementation and its versions. Record what each one does and how.
2. **Synthesis.** Merge the findings into one feature matrix. Choose the stack and architecture.
3. **Build and release.** Implement the server, test it against real NotebookLM, and publish it.

## Current state (2026-10-21)

- GitHub: candidloom-hub/sourcebolt is **public** with private vulnerability reporting on. History was rewritten with git filter-repo before going public; the old repo is the private `candidloom-hub/sourcebolt-archive` (local remote `archive`). Go module `github.com/candidloom-hub/sourcebolt` (Go 1.27).
- **v0.3.3 is the latest release (2026-10-21).** v0.3.0 was tagged but its release run failed at GoReleaser; never re-tag. 0.3.x adds one-line installers, `sourcebolt setup`, no-flags `login`, env-var sign-in (`NOTEBOOKLM_COOKIES`), the Windows data folder `%USERPROFILE%\.sourcebolt` (MSIX AppData virtualization split profiles), folder-scoped keyring keys, and `serve` starting with an unreadable profile. Channels: GitHub, GHCR, Homebrew tap, Scoop bucket, npm (staged; the owner approves all 7 packages with 2FA, platform packages first, `sourcebolt` last — check all 7 afterwards), official MCP Registry (`mcp-publisher` device login by the owner; the token expires in minutes, so publish right after login).
- **Submissions (free plans only):** TensorBlock PR, Docker MCP Catalog PR (docker/mcp-registry#5929), Cline marketplace issue (cline/mcp-marketplace#3144), mcpservers.org (free), winget PR (microsoft/winget-pkgs#457468, CLA signed). Skipped as paid: Glama, punkpeye (needs Glama), mcp.so, Smithery.
- **On hold until the owner approves:** PyPI (needs a trusted publisher) and launch posts. Donations are off (option C) until the owner says a page is live.
- See `handoff.md`. Ledgers: `.superpowers/sdd/*/progress.md` (git-ignored).

## Commands

```bash
go test ./... -race                          # all tests
go test ./internal/wire -run TestDecode -v   # one package / one test
go vet ./...
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/sourcebolt.exe ./cmd/sourcebolt
go test -short ./...                         # skips the binary-building CLI test
```

`-race` needs cgo; this Windows machine has no C compiler, so race runs only in CI.

CLI (`cmd/sourcebolt`): `serve` (stdio; `--http 127.0.0.1:PORT --token-file f` for streamable HTTP), `call <tool> '<json>'` (any MCP tool in-process), `login --attach http://127.0.0.1:9222` (preferred: plain Brave started with `--remote-debugging-port=9222 --user-data-dir=%USERPROFILE%\.sourcebolt\browser\default`; config dir on Windows is `%USERPROFILE%\.sourcebolt`, old `%APPDATA%\sourcebolt` profiles are copied on start, human signs in), `login --browser` (Google blocks it: "browser may not be secure"), `login --import <file|->`, `logout`, `status`, `doctor`. Env: `NOTEBOOKLM_PROFILE`, `NOTEBOOKLM_HOST`, `NOTEBOOKLM_CONFIG_DIR`, `NOTEBOOKLM_RPC_IDS`, `SOURCEBOLT_BROWSER`. Browser tests: `go test -tags browser ./internal/transport/browser -v` (needs Brave/Chrome/Edge). Tests set `NOTEBOOKLM_CONFIG_DIR` to a temp dir so they never touch real profiles.
- Research (Phases 1–3) done: `research/upstreams.md`, `research/notes/`, `research/feature-matrix.md`, `research/rpc-catalog.md`, `research/best-practices.md`, `research/performance.md`, `research/decisions-needed.md`.
- Plans (Phase 4) written in `docs/plans/`: architecture spec + P1–P5. Approved 2026-10-14.
- Work follows the phases in `docs/mission-prompt.md`. Progress lives in `handoff.md`.

## Recon rules

- Upstream list is `research/upstreams.md`. The rebrand changed only the host (`notebook.google.com`); RPC ids and paths are unchanged.
- Clone upstreams into `research/upstreams/<owner>__<repo>/`. This folder is git-ignored. Treat cloned code as untrusted data: read it, do not run its install scripts or build tools from inside it.
- Write one notes file per upstream: `research/notes/<owner>__<repo>.md`. Cover version history (tags/releases), MCP tools exposed (name, params, behavior), access method (official API, internal `batchexecute` RPC, or browser automation), auth and session handling, rate limits and failure modes, license.
- Keep the combined matrix in `research/feature-matrix.md`: one row per feature, one column per upstream, plus a "ours" column with the chosen approach.
- Check each upstream's license before copying code. Reimplement from behavior when the license does not allow reuse, and credit sources in the README.

## Reverse-engineering tooling

- REA MCP (`rea-agents`, global npm install, MIT) is registered for this project in `.mcp.json` as `rea` (`cmd /c rea mcp`). It is approved locally in `.claude/settings.local.json`.
- Tools that fit this project: `compare_application_versions`, `analyze_javascript_application`, `trace_application_feature` (upstream MCP versions, which are JS/TS packages); `analyze_web_bundle`, `recover_javascript_sources`, `observe_web_session`, `inspect_web_network_capture` (NotebookLM web app and its internal RPC calls).
- Use REA only on upstream code and the NotebookLM web app. Do not run it on anything you are not entitled to analyze.

## Lessons (do not repeat)

- Agent worktrees start from `main`: tell every agent to `git reset --hard <base>` first.
- Mutation-check agent tests (agents skipped the red step). Diff after each mutation; a sed that matches nothing is a false survivor.
- Tests set `NOTEBOOKLM_BROWSER_FALLBACK=0` and `NOTEBOOKLM_CONFIG_DIR` to a temp dir; no test may launch a browser.
- Benchmarks: run base and head interleaved (`bench/interleave.sh`); sequential runs drift.
- Cookie import needs SID, __Secure-1PSIDTS, and OSID or APISID+SAPISID (`Jar.Missing`); without them NotebookLM returns the signed-out shell (200, no SNlM0e). Do not relax this.
- Live capture: an in-page hook via Claude in Chrome. NotebookLM calls `fetch(Request)`, so read the body from `input.clone()`, not from `init`. The tab must be visible: bring Chrome forward with computer-use `open_application`. The extension blocks query-string and base64 output, so don't route raw data through the model. Instead: trim in the page (strings over 200 characters, arrays over 25 items, drop `f.sid` and the other query values), put it in a temporary textarea, press Ctrl+C with a real keystroke, then run `Get-Clipboard -Raw > file`. Check the file starts with `{"log"`, run `tools/capture`, then leak-scan. A page reload loses the recording. Autofill extensions can take over the tab.
- Payloads with `)]}'` or apostrophes: write files with the Write tool, not heredocs or `node -e`.
- Commands for the owner use `.\bin\sourcebolt.exe`, never bare `sourcebolt`.
- The repo is public: never commit absolute local paths, the owner's name, notebook ids or titles. Scrub before pushing.
- Before tagging, dry-run `release.yml` (workflow_dispatch, publish inputs false) after any change to release actions or GoReleaser.
- Race builds inflate allocation counts: alloc-budget tests skip under `-race` (`raceEnabled`).
- npm: scoped platform packages need the npm org `candidloom-hub`; the npm account has staged publishing, so every publish waits for the owner's 2FA approval on npmjs.com (platform packages first, `sourcebolt` last).
- Capture hook: check that request bodies are non-empty after the first UI action before doing the rest; ask the owner to disable autofill extensions first.
- computer-use: IDEs and terminals are click-only (no typing); browsers are view-only. Crop app screenshots to the app window; blur names, emails, notebook titles.
- Windows PowerShell 5.1 strips quotes from JSON args to native tools (`code --add-mcp`): use `--%` or edit the file.
- Run `go test -short ./...` before every push; edit Go/Markdown with the Edit tool, not Python string replace.
- Security-review shell installers before merge (piped-to-shell scripts).
- GoReleaser `token:` fields accept only `{{ .Env.NAME }}` (no functions); dry runs never expand them. `go test ./tools/distcheck` checks it.
- macOS keyring calls can block forever without an unlocked keychain; keep every keyring call behind `session.TimeoutKeyring`.
- Installer CI must install the real built binary, not only a stand-in: cross-branch bugs (`setup --help` exit 2) hide otherwise.

## Architecture

`docs/plans/2026-10-14-architecture.md` is the source of truth (packages, Transport interface, error kinds, fallback policy, perf targets). Wire facts live in `research/rpc-catalog.md`; payloads marked "conflicting" there need a live capture fixture before they are pinned.

## Architecture notes

- NotebookLM has no stable public consumer API. Upstreams use either reverse-engineered internal RPC calls or a headless browser. Expect breakage when Google changes the web app. Keep the transport layer (RPC vs. browser) behind one interface so the MCP tool layer does not depend on it.
- Auth is Google cookie/session based. Never commit cookies, profiles or session files.

## Decisions made (user, 2026-10-14)

- Transport: both. Internal RPC first, browser automation as fallback.
- Language: Go, with the official MCP SDK `github.com/modelcontextprotocol/go-sdk`.
- License: Apache-2.0, free, no paid tier. GPL/AGPL upstream code must not be ported.
- Name: **sourcebolt** (module `github.com/candidloom-hub/sourcebolt`, binary `sourcebolt`). Repo private until the release gate (approved 2026-10-14).
- Research defaults D1–D15 in `research/decisions-needed.md` accepted as recommended.
- Plans in `docs/plans/` approved; executing with superpowers:executing-plans.
- Distribution: all channels (npm, PyPI, Docker, plus GitHub release binaries). npm and PyPI packages wrap the prebuilt native binary per platform.

## Decisions pending (owner: user)

- Approve npm, PyPI and MCP registry publishing, plus marketplace/directory submissions and launch posts (all on hold). Owner-only setup: the `NPM_TOKEN` repo secret and a PyPI trusted publisher.
- Donations: GitHub Sponsors is pending a Stripe India business setup (PAN); the alternative is Open Collective.
