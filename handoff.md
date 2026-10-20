# Handoff

Updated 2026-10-21. The work was stopped on the owner's request, partway through the "beginner install" batch.

# Goal — what we're actually building

**sourcebolt**: one Go MCP server for Google NotebookLM (official go-sdk, Go 1.27, Apache-2.0, free). It combines every public NotebookLM MCP's features and beats them on speed. Internal batchexecute RPC first, then a headless-browser fallback, then a page (DOM) fallback.

Performance is the owner's first priority.

Distribution: GitHub release binaries, GHCR, npm, PyPI, MCP registry, Homebrew, Scoop and winget.

Current push: make installing it trivial for **non-technical** users. That means one command, a guided setup, a plain sign-in, guides with screenshots, and install-by-asking-your-AI.

Repo: candidloom-hub/sourcebolt (public). Source of truth: `docs/plans/2026-10-14-architecture.md`, `research/rpc-catalog.md`, `CLAUDE.md`.

# Current state — where things stand right now

**Released:**
- v0.1.0 and v0.2.0 on GitHub, with 6 platform binaries, SBOMs, cosign-signed checksums and 6 `.mcpb` bundles.
- GHCR images `0.2.0` and `0.2.0-browser`.
- `go install …@v0.2.0` works.
- **npm 0.2.0 is live** (`npx -y sourcebolt serve`): `sourcebolt` plus 6 `@candidloom-hub/sourcebolt-<os>-<cpu>` packages, each with provenance.

**`main`** is at `ca650aa` (PR #28 merged: npm docs, and `server.json` lists npm). CI is green.

**Unmerged branches** (pushed, no PRs yet):

- **`feat/installers`** (494ccd7, +5 commits). Contents:
  - `install.ps1`, `install.sh` and their offline tests;
  - CI installer jobs;
  - GoReleaser `homebrew_casks`, `scoops` and `winget`;
  - winget manifests;
  - `docs/install.md`.

  Release dry run passed. The **security review says NOT merge-ready**: 2 bugs, 14 risks (list under Next steps).

- **`feat/setup-wizard`** (origin 684c1ee, +7). Contents:
  - `sourcebolt setup` (detects Claude Desktop, Claude Code, Cursor, VS Code, Windsurf and Gemini CLI; backup, merge, `--undo`, `--dry-run`);
  - `sourcebolt login` with no flags (a plain browser window with a debugging port).

  **Live-tested**: login worked against real Google in 18 s, and `setup --dry-run` detected 5 apps correctly. The local worktree `.claude/worktrees/agent-a4e9190a087ec6574` has 2 **unpushed** commits on top: `7090063` (AUTH_REQUIRED guidance says "Run: sourcebolt login") and `c7455f9` (WIP: npx/temp executable detection, unfinished, tests may fail).

- **`docs/beginner-guides`** (ba262bb, +3). Contents:
  - `docs/install-with-ai.md` (prompts for 7 apps);
  - `docs/guides/*.md` (7 apps, 42 screenshot slots, no images yet);
  - README "Install in 2 minutes";
  - `docs/index.md` links.

  Its `--client` names are guesses until setup merges.

**Owner-side state:**
- npm org `candidloom-hub` exists. The npm account has **staged publishing**: every publish needs a 2FA approval on npmjs.com (platform packages first, `sourcebolt` last).
- Secrets: `NPM_TOKEN` (30-day granular, expires around 2026-11-19) and `HOMEBREW_TAP_TOKEN`.
- Repos created: candidloom-hub/homebrew-tap and candidloom-hub/scoop-bucket (README only).

**This PC** (for screenshots):
- v0.2.0 binary at `%LOCALAPPDATA%\Programs\sourcebolt\sourcebolt.exe`.
- sourcebolt entry added to VS Code `mcp.json` (backup `mcp.json.sourcebolt-backup-20261021`) and to a new `~/.cursor/mcp.json`.
- The sourcebolt profile is signed in.

**On hold until the owner approves:** PyPI, the MCP registry publish, the winget submission, directory and marketplace submissions, launch posts, and donations (option C: none).

**Next release** will be v0.3.0. It needs an owner go, and it is the first release with `setup` and no-flags `login`.

# Files touched — paths + one line each

- `cmd/sourcebolt/{login.go,status.go,setup*.go,doctor_test.go,login_replace_test.go}`: no-flags login, `setup` wizard, `doctor --dom-check`, login replaces a profile whose key was lost, and the cookie hint fix.
- `internal/setup/*` (branch feat/setup-wizard): client detection, JSON merge, backup and undo.
- `internal/transport/browser/{plain*,dom_*,selectors.*}`: plain-browser launch for login, DOM fallback for chat and add-source.
- `internal/nlm/*`: Studio types and defaults, chat_configure, save_as_note, deletes, source guide, add-source reconciliation.
- `internal/wire/{node,decode,stream}.go`: byte scanner, zero-copy decode, citation passage indices, opt-in AnswerBlock.
- `internal/mcpserver/{register.go,tools_*.go,envelope.go}`: raw-handler registration with lazy schemas, has_more, retry_after_seconds.
- `install.ps1`, `install.sh`, `scripts/test-installers.*` (branch feat/installers): one-line installers.
- `.goreleaser.yaml`, `.github/workflows/{release,ci,bench}.yml`: MCPB bundles, casks, scoop, winget, installer CI, bench via env.
- `packaging/{mcpb,winget}/`, `plugins/sourcebolt/`, `.claude-plugin/marketplace.json`, `glama.json`, `gemini-extension.json`, `server.json`: distribution metadata.
- `testdata/fixtures/*/live*_2026-10-20_*.json`: live captures #2, #3 and #3b (sanitized).
- `README.md`, `docs/*.md`, `docs/guides/*`, `docs/install*.md`: user docs rewrite, beginner guides, install pages.
- `CHANGELOG.md`, `CLAUDE.md`, `handoff.md`, `bench/RESULTS.md`, `research/{rpc-catalog,marketplaces,docs-best-practices}.md`.

# What changed — key edits this session

- Released v0.2.0. Published npm 0.2.0 with provenance (after the npm org was created and the staged versions were approved).
- Live captures #2, #3 and #3b pinned the shapes for:
  - all 10 Studio kinds and their defaults;
  - chat settings, including reset to default;
  - save chat answer as note;
  - the source, note and chat-history deletes;
  - the source guide.
- Perf program met every target on CI: notebook_list 0.758 ms uncached and 0.398 ms cached; cold start 8.2 ms; idle memory 16.2 MB; chat first progress +0.30 ms. The default page is 20 with `has_more`. GOGC=200 for `serve`. Transparent huge pages off on Linux.
- `doctor --dom-check`: the DOM fallback was verified live (5.2 s).
- Login fixes: an unreadable profile is replaced, and the cookie hint matches `Jar.Missing`.
- Docs rewrite: getting started, clients, troubleshooting, FAQ, why-sourcebolt, comparison, performance, privacy, install-links.
- Distribution: Claude Code plugin marketplace, MCPB, `glama.json`, a registry-ready `server.json`.

# What failed — dead ends, do NOT retry these

- **Re-tagging a published version**: v0.1.0 is immutable in proxy.golang.org and sum.golang.org. Always use a new version.
- **Capture hook reading `init.body`**: NotebookLM uses `fetch(Request)`, so the bodies came out empty and the first capture had to be redone. Read `input.clone().text()`. Check that bodies are non-empty after the first action.
- **Returning capture data through Claude in Chrome output**: the extension blocks query-string and base64 data. Use a textarea, a real Ctrl+C, then `Get-Clipboard -Raw` into a file.
- **Capturing from a hidden or unfocused tab**: screenshots and the clipboard time out. Bring Chrome forward with computer-use `open_application`.
- **Third-party autofill extensions** inject frames that block the tab until a reload (the reload loses the recording). Ask the owner to disable them before a capture.
- **npm publish under the `@candidloom-hub` scope without an npm org**: `E404 Scope not found`.
- **npm publish without checking staged publishing**: the versions sit as `0.0.0-stage` until the owner approves them with 2FA.
- **`code --add-mcp '<json>'` from Windows PowerShell 5.1**: the quotes get stripped. Use `--%` or edit `mcp.json` directly.
- **Typing into VS Code or Cursor with computer-use**: those apps are click-only. App-chat screenshots need the owner to type.
- **Window focus for app screenshots**: other windows (Brave, the desktop shell) steal the front, and the desktop shows personal file names. Crop to the app window only.
- **Pushing before the full test run**: removing FUNDING.yml broke a distcheck test after the push. Run `go test -short ./...` first.
- **Python `.replace` edits of Go or Markdown with backslashes or `\n`**: they failed silently or asserted. Use the Edit tool for such edits.
- **Removing a worktree while the shell is inside it**: "Permission denied". cd out first, then `git worktree prune`.
- **Byte or alloc assertions under `-race` on CI**: flaky. Skip them under `raceEnabled`.
- **GitHub Sponsors as an individual in India**: Stripe won't take international payments, and a sole proprietorship needs a PAN. The owner chose option C (no donations).

# Next steps — exactly what to do next

1. **feat/installers**: fix the security review findings, then PR and merge.
   - **Bug, `.goreleaser.yaml:194,203`**: the winget block auto-opens a PR to microsoft/winget-pkgs whenever `WINGET_PKGS_TOKEN` is set. Set `skip_upload: true` (winget submission is on hold).
   - **Bug, `install.sh:195`**: the rc-block removal can truncate the rc file when the END marker is missing. Require both markers, write to a mktemp file next to it, then `mv` it into place.
   - **Owner question (raised by the review)**: confirm Homebrew and Scoop may auto-publish on tag. If not, set `skip_upload: true`.
   - **Risks**:
     - Wrap `install.sh` in `main "$@"`.
     - Validate the version against `^v\d+\.\d+\.\d+(-[\w.]+)?$` in both installers.
     - Make both installers refuse non-HTTPS URLs (`--proto-redir '=https'`; same check in the wget branch and in PowerShell).
     - Escape or validate INSTALL_DIR before writing it into the rc file.
     - Guard purge and uninstall paths: never empty, `/`, `$HOME`, USERPROFILE or APPDATA; never `--dir ""`.
     - `install.ps1`: handle a Read-Host failure (non-interactive), preserve the PATH registry value kind, and roll back the binary on a failed upgrade.
     - Pin `sigstore/cosign-installer` and `goreleaser-action` to commit SHAs.
     - Print "integrity only" when cosign is absent.
     - Verify the winget SHA-256 values against `checksums.txt`.
2. **feat/setup-wizard**:
   - In the worktree, finish `c7455f9`: when `os.Executable()` is in an npx/npm cache or a temp dir, write the installed binary path if it exists, else `npx -y sourcebolt serve`, else refuse.
   - Push `7090063` and the fix.
   - Rebase on main after installers merges; mutation-check; PR; merge.
3. **docs/beginner-guides**:
   - Rebase. Replace the guessed `--client` names and installer messages with the real ones (`Installed: sourcebolt <version>`; uninstall lines from `docs/install.md`).
   - Fix the stale `.mcpb` line in `docs/install-links.md`.
   - Terminal screenshots: render them from real command output (exact text, no personal data).
   - App-UI screenshots: the owner types one question per app while you capture. Blur notebook titles, email and name; crop to the app window.
4. **v0.3.0** (owner go):
   - CHANGELOG section and version bumps (`server.json`, the plugin, `gemini-extension.json`, install links via `tools/installlinks`, README).
   - Release dry run, tag, verify, publish the draft promptly (casks and scoop point at it).
   - Then the npm publish run, and tell the owner to approve the staged versions.
5. **Held items, each only on an explicit owner go**: PyPI (trusted publisher), MCP registry (`mcp-publisher publish`), the winget PR, directory submissions (`research/marketplaces.md`), launch posts (`docs/launch-plan.md`), and moving npm to trusted publishing before `NPM_TOKEN` expires.
6. **Owner checks**: delete the notebook "sourcebolt capture 3 (safe to delete)"; decide whether to keep the VS Code and Cursor sourcebolt entries added for screenshots.
