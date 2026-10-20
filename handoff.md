# Handoff

Updated 2026-10-21. v0.3.1 is tagged; the release run, npm, the MCP registry, winget and directory submissions follow (owner approved all four; PyPI and launch posts stay held).

# Goal — what we're actually building

**sourcebolt**: one Go MCP server for Google NotebookLM (official go-sdk, Go 1.27, Apache-2.0, free). It combines every public NotebookLM MCP's features and beats them on speed. Internal batchexecute RPC first, then a headless-browser fallback, then a page (DOM) fallback.

Performance is the owner's first priority. Current push: installing must be trivial for non-technical users.

Repo: candidloom-hub/sourcebolt (public). Source of truth: `docs/plans/2026-10-14-architecture.md`, `research/rpc-catalog.md`, `CLAUDE.md`.

# Current state — where things stand right now

- **Released before today:** v0.1.0, v0.2.0 (GitHub, GHCR, npm 0.2.0).
- **v0.3.0:** tagged on `b6d4f8b`, but its release run failed at GoReleaser (`envOrDefault` is not allowed in `token:` fields). A **draft** release v0.3.0 with archives only was left on GitHub; it is not public. The owner may delete it (Releases page > v0.3.0 draft > Delete).
- **v0.3.1:** tagged on the PR #34 merge commit. Same content as v0.3.0 plus the token fix.
- Merged today: #29 (handoff, rpc cancel fix), #30 (installers), #31 (setup, squash), #32 (beginner guides), #33 (v0.3.0 prep, real-binary installer CI, macOS keyring timeout), #34 (v0.3.1).
- Repo description and topics updated for directories (`gemini-cli-extension` topic feeds the Gemini gallery).
- Fork `candidloom-hub/winget-pkgs` exists for the winget PR.
- `mcp-publisher` v1.8.1 (checksum-verified) is in the session scratchpad; `mcp-publisher login github` needs a one-time device code from the owner.
- Directory plan: `.superpowers/submissions-v0.3.0.md` (git-ignored).

# Files touched — paths + one line each

- `install.sh`, `install.ps1`, `scripts/test-installers.*`: one-line installers, two security reviews.
- `cmd/sourcebolt/{setup*.go,main.go,status.go,help_test.go}`: setup, `--help` exits 0, no `%` through `.cmd` CLIs, doctor keyring probe with timeout.
- `internal/setup/*`: client detection, stable command (npx/temp detection).
- `internal/session/store.go`, `keyring_timeout_test.go`: `TimeoutKeyring` (30 s) around every OS keyring call.
- `internal/transport/rpc/rpc.go`: a cancelled call reports the cancel, not AUTH_REQUIRED.
- `.goreleaser.yaml`, `.github/workflows/{ci,release}.yml`: casks, scoop, winget (never uploads), SHA-pinned actions, real-binary installer CI.
- `tools/distcheck/goreleaser_token_test.go`: tokens must be `{{ .Env.NAME }}`.
- `CHANGELOG.md`, `README.md`, `docs/*`, `server.json`, `gemini-extension.json`, plugin: 0.3.1.

# What changed — key edits this session

- Bugs found by end-to-end runs of the real binary and fixed failing-test-first: `setup --help` exit 2 (installers never offered setup); `%` in a path expanded by `cmd.exe` in `code.cmd`; macOS keychain hang in setup/doctor/login/serve; flaky cancel reported as AUTH_REQUIRED; GoReleaser token template.
- Installer CI now installs the real built binary and runs `setup --client cursor` in a temp home.

# What failed — dead ends, do NOT retry these

- Re-tagging: never. v0.3.0 failed to release; the fix shipped as v0.3.1.
- `envOrDefault` / `index .Env` in GoReleaser `token:` fields: rejected (`ApplySingleEnvOnly`). Only `{{ .Env.NAME }}`.
- Release dry runs do not expand publish-only templates: a green dry run does not prove a tag will publish.
- Git Bash rewrites `origin/main:path` into a Windows path: set `MSYS_NO_PATHCONV=1` or use the Read tool.
- Everything in the previous handoff's list still applies (capture hook, npm staged publishing, PowerShell 5.1 quoting, computer-use tiers).

# Next steps — exactly what to do next

1. When the v0.3.1 release run finishes: check 6 archives, 6 `.mcpb`, checksums and signature; publish the draft; check GHCR `0.3.1`, the Homebrew tap and Scoop bucket commits.
2. Run `release.yml` from tag v0.3.1 with `publish_npm=true`; the owner approves the staged npm versions with 2FA.
3. MCP registry: `mcp-publisher login github` (owner enters the device code), then `mcp-publisher publish`.
4. winget: open the PR from `candidloom-hub/winget-pkgs` with the v0.3.1 manifests (SHA-256 from `checksums.txt`).
5. Directory submissions per `.superpowers/submissions-v0.3.0.md` (punkpeye waits for Glama; forms need the owner's login).
6. Push `docs/post-v0.3.1` (install.md availability, CLAUDE.md, this handoff) after step 1 confirms the tap and bucket.
7. Held: PyPI (trusted publisher), launch posts. Screenshots for the guides (owner types in each app).
