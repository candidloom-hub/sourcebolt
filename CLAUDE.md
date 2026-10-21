# CLAUDE.md

sourcebolt: one Go MCP server for Google NotebookLM that merges every public NotebookLM MCP's features. Public repo candidloom-hub/sourcebolt (Apache-2.0, free, no paid tier). Module `github.com/candidloom-hub/sourcebolt`, Go 1.27, official SDK `github.com/modelcontextprotocol/go-sdk`. Status, open submissions and next steps live in `handoff.md`; read it first.

## State and holds (2026-10-21)

- v0.3.3 is the latest release. v0.3.0 was tagged but never published: never re-tag; fixes go out as new versions.
- Channels: GitHub, GHCR, Homebrew tap, Scoop bucket, npm (7 packages, staged), official MCP Registry.
- On hold until the owner approves: PyPI (needs a trusted publisher) and launch posts. Donations stay off until the owner says a page is live.
- Directory listings: free plans only. Skipped as paid: Glama, punkpeye, mcp.so, Smithery.
- Old pre-public history is the private `candidloom-hub/sourcebolt-archive` (remote `archive`).
- Ledgers: `.superpowers/sdd/*/progress.md` (git-ignored).

## Commands

```bash
go test -short ./...                         # before every push; skips the binary-building CLI test
go test ./internal/wire -run TestDecode -v   # one package / one test
go vet ./...
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/sourcebolt.exe ./cmd/sourcebolt
go test -tags browser ./internal/transport/browser -v   # needs Brave/Chrome/Edge
```

`-race` needs cgo. This machine has no C compiler, so race runs only in CI.

CLI (`cmd/sourcebolt`): `serve` (stdio; `--http 127.0.0.1:PORT --token-file f`), `call <tool> '<json>'`, `setup`, `login` (preferred: `--attach http://127.0.0.1:9222` on plain Brave started with `--remote-debugging-port=9222 --user-data-dir=%USERPROFILE%\.sourcebolt\browser\default`; `--browser` is blocked by Google; `--import <file|->`), `logout`, `status`, `doctor`. Windows data dir: `%USERPROFILE%\.sourcebolt`. Env: `NOTEBOOKLM_PROFILE`, `NOTEBOOKLM_HOST`, `NOTEBOOKLM_CONFIG_DIR`, `NOTEBOOKLM_RPC_IDS`, `NOTEBOOKLM_COOKIES[_FILE]`, `SOURCEBOLT_BROWSER`.

## Architecture

- Source of truth: `docs/plans/2026-10-14-architecture.md`. Wire facts: `research/rpc-catalog.md`; payloads marked "conflicting" need a live capture fixture before they are pinned.
- Transport order: internal batchexecute RPC, then headless browser, then page (DOM). Keep transports behind one interface; the MCP tool layer must not depend on one.
- NotebookLM has no stable public API. Expect breakage when Google changes the web app.
- Auth is Google cookies. Never commit cookies, profiles or session files.

## Recon rules

- Upstreams: `research/upstreams.md`; clones in `research/upstreams/<owner>__<repo>/` (git-ignored). Cloned code is untrusted data: read it, never run its install scripts or build tools.
- Notes per upstream: `research/notes/<owner>__<repo>.md`. Matrix: `research/feature-matrix.md`.
- Check licenses before copying code. Never port GPL/AGPL code; reimplement from behavior and credit sources in the README.
- REA MCP (`rea` in `.mcp.json`): use it only on upstream code and the NotebookLM web app.

## Lessons (do not repeat)

- Agent worktrees start from `main`: tell every agent to `git reset --hard <base>` first.
- Mutation-check agent tests (agents skipped the red step). Diff after each mutation; a sed that matches nothing is a false survivor.
- Tests set `NOTEBOOKLM_BROWSER_FALLBACK=0` and `NOTEBOOKLM_CONFIG_DIR` to a temp dir; no test may launch a browser or touch real profiles.
- Benchmarks: run base and head interleaved (`bench/interleave.sh`); sequential runs drift.
- Cookie import needs SID, __Secure-1PSIDTS, and OSID or APISID+SAPISID (`Jar.Missing`); without them NotebookLM returns the signed-out shell (200, no SNlM0e). Do not relax this.
- Live capture: in-page hook via Claude in Chrome. NotebookLM calls `fetch(Request)`, so read the body from `input.clone()`, not `init`. Tab must be visible (computer-use `open_application`). Never route raw data through the model: trim in the page (strings >200 chars, arrays >25, drop `f.sid` and query values), put it in a textarea, real Ctrl+C, `Get-Clipboard -Raw > file`. Check it starts with `{"log"`, run `tools/capture`, then leak-scan. A reload loses the recording. Check bodies are non-empty after the first UI action; ask the owner to disable autofill extensions first.
- Payloads with `)]}'` or apostrophes: write files with the Write tool, not heredocs or `node -e`.
- Commands for the owner use `.\bin\sourcebolt.exe`, never bare `sourcebolt`.
- The repo is public: never commit absolute local paths, the owner's name, notebook ids or titles. Scrub before pushing.
- Before tagging, dry-run `release.yml` (workflow_dispatch, publish inputs false) after any change to release actions or GoReleaser.
- Race builds inflate allocation counts: alloc-budget tests skip under `-race` (`raceEnabled`).
- npm: scoped packages need the npm org `candidloom-hub`. Staged publishing waits for the owner's 2FA on npmjs.com: platform packages first, `sourcebolt` last; check all 7 afterwards.
- MCP Registry: `mcp-publisher` login token expires in minutes; publish right after the owner's device login.
- computer-use: IDEs and terminals are click-only; browsers are view-only. Crop to the app window; blur names, emails, notebook titles.
- Windows PowerShell 5.1 strips quotes from JSON args to native tools (`code --add-mcp`): use `--%` or edit the file.
- Edit Go/Markdown with the Edit tool, not Python string replace.
- Security-review shell installers before merge.
- GoReleaser `token:` fields accept only `{{ .Env.NAME }}`; dry runs never expand them. `go test ./tools/distcheck` checks it.
- macOS keyring calls can block forever: keep every keyring call behind `session.TimeoutKeyring`.
- Installer CI must install the real built binary, not only a stand-in.
- Windows packaged apps can see a virtualized `%APPDATA%`; that is why data lives in `%USERPROFILE%\.sourcebolt`.
