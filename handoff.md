# Handoff

Updated 2026-10-21. The beginner-install batch is code-complete: 4 PRs open, waiting for owner merges.

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

**Open PRs** (none merged; merging is the owner's call). Merge in this order:

1. **#29 `docs/handoff-2026-10-21`**: this handoff and the CLAUDE.md lessons, plus `fix(rpc)`: a call cancelled while its body is read reports `context.Canceled`, not AUTH_REQUIRED (this was the flaky `TestCtxCancel` under -race).
2. **#30 `feat/installers`** (ecef9ef): `install.ps1` and `install.sh` with offline tests and CI jobs; GoReleaser Homebrew cask and Scoop (auto-push on tag, owner approved 2026-10-21); winget generated but `skip_upload: true` always; third-party actions pinned to SHAs; `docs/install.md`. Two security reviews; every finding fixed failing-test-first and mutation-checked. Tests: sh 27/27, PowerShell 5.1 19/19. The release dry run passed on 4a74eaa.
3. **#31 `feat/setup-wizard`** (0ce6bbc): `sourcebolt setup`, no-flags `login`, npx/temp-exe detection (a362634), `--help` exits 0 on every subcommand (the installers need this). **Squash-merge**: WIP commit c7455f9 does not build on its own.
4. **#32 `docs/beginner-guides`** (ba5fb40): install-with-AI prompts, 7 app guides, README block. Client ids are now the real ones (`gemini`, not `gemini-cli`); the uninstall step uses the installer.

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

1. **Owner merges** #29, #30, #31 (squash), #32, in that order. Re-run CI on each PR after the one before it merges.
2. **Screenshots** (after v0.3.0 is out, so the real installer and `setup` output exist):
   - Terminal: render from real command output, with exact text and no personal data.
   - App UI: the owner types one question per app while you capture. Blur notebook titles, email and name; crop to the app window.
3. **v0.3.0**** (owner go):
   - CHANGELOG section and version bumps (`server.json`, the plugin, `gemini-extension.json`, install links via `tools/installlinks`, README).
   - Release dry run, tag, verify, publish the draft promptly (casks and scoop point at it).
   - Then the npm publish run, and tell the owner to approve the staged versions.
4. **Held items, each only on an explicit owner go**: PyPI (trusted publisher), MCP registry (`mcp-publisher publish`), the winget PR, directory submissions (`research/marketplaces.md`), launch posts (`docs/launch-plan.md`), and moving npm to trusted publishing before `NPM_TOKEN` expires.
5. **Owner checks**: delete the capture-3 test notebook; decide whether to keep the VS Code and Cursor sourcebolt entries added for screenshots.
