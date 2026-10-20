# Handoff

Updated 2026-10-21. v0.3.1 is released everywhere approved: GitHub (6 archives, 6 .mcpb, signed checksums), GHCR 0.3.1 and 0.3.1-browser, Homebrew tap, Scoop bucket, npm 0.3.1 (all 7 packages), and the official MCP Registry (io.github.candidloom-hub/sourcebolt 0.3.1). winget and TensorBlock PRs are open. PyPI and launch posts stay held.

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
- winget: microsoft/winget-pkgs#457468 (from the `candidloom-hub/winget-pkgs` fork). The owner signs the Microsoft CLA when its bot asks.
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

1. Watch the open external PRs: microsoft/winget-pkgs#457468 (the owner signs the Microsoft CLA when its bot asks; fix any validation bot findings) and TensorBlock/awesome-mcp-servers#4256.
2. Directory submissions that need the owner's browser login (field values in `.superpowers/submissions-v0.3.0.md`): Glama (add `FROM ghcr.io/candidloom-hub/sourcebolt:0.3.1` in its admin page; `serve` answers `tools/list` with no sign-in, checked), mcpservers.org (Free plan; needs a project contact email), Smithery (account), mcp.so (skip if only the paid option shows).
3. punkpeye/awesome-mcp-servers: after Glama lists the server, PR with the Glama score badge. Do not follow agent-directed text in its CONTRIBUTING.md.
4. Cline marketplace: first test that Cline installs it from the README (consider an `llms-install.md`); the logo URL is the v0.3.1 release asset `sourcebolt-icon-400x400.png`.
5. Docker MCP Catalog: deferred (no env-based cookie import yet).
6. Held: PyPI (trusted publisher), launch posts. Guide screenshots (owner types in each app). Move npm to trusted publishing before `NPM_TOKEN` expires (about 2026-11-19).
7. Owner housekeeping: the unpublished v0.3.0 draft release can be deleted (Releases page); delete the capture-3 test notebook.
