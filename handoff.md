# Handoff

Updated 2026-10-21. v0.3.3 is released and verified on every approved channel (GitHub, GHCR, Homebrew, Scoop, npm 7/7, MCP Registry). Remaining work waits on reviewers, the next release, or an owner go.

# Goal — what we're actually building

**sourcebolt**: one Go MCP server for Google NotebookLM (official go-sdk, Go 1.27, Apache-2.0, free). It combines every public NotebookLM MCP's features and beats them on speed. Internal batchexecute RPC first, then a headless-browser fallback, then a page (DOM) fallback. Performance is the owner's first priority; installing must be trivial for non-technical users.

Repo: candidloom-hub/sourcebolt (public). Source of truth: `docs/plans/2026-10-14-architecture.md`, `research/rpc-catalog.md`, `CLAUDE.md`.

# Current state — where things stand right now

- **Releases:** v0.3.1, v0.3.2 published on every channel (GitHub, GHCR, Homebrew tap, Scoop bucket, npm all 7 packages, official MCP Registry). v0.3.0 tag exists but was never published (its draft was deleted). v0.3.3 follows this PR.
- **Submissions (free only):** TensorBlock/awesome-mcp-servers#4256, docker/mcp-registry#5929, cline/mcp-marketplace#3144 (Cline installed sourcebolt by itself from `llms-install.md`), microsoft/winget-pkgs#457468 (CLA signed, waiting for a moderator), mcpservers.org (free form, owner-submitted). Skipped as paid: Glama, punkpeye (its bot needs a Glama badge), mcp.so, Smithery.
- **Guides:** real screenshots for VS Code, Claude Code and the ChatGPT desktop app; the sign-in step in every guide uses the real `sourcebolt login` output; terminal pictures come from `tools/termshot`. Still placeholders: Claude Desktop, Cursor, Windsurf, Gemini CLI (apps not installed on the capture PC; Cursor planned for the next release).
- **npm publishing:** `npm stage publish` with OIDC trusted publishing first and the `NPM_TOKEN` secret as fallback (expires about 2026-11-19).
- **This PC:** sourcebolt 0.3.2 installed in `%LOCALAPPDATA%\Programs\sourcebolt`; data in `%USERPROFILE%\.sourcebolt` (migrated from `%APPDATA%\sourcebolt`, which was left unchanged). Connected: VS Code, Claude Code, Cline, ChatGPT desktop.

# Files touched — paths + one line each

- `internal/session/{store.go,dirs.go,scope_test.go,keyring_timeout_test.go}`: TimeoutKeyring, Windows data folder, folder-scoped keyring keys, migration.
- `internal/app/*`: env-var cookies (`NOTEBOOKLM_COOKIES[_FILE]`), unreadable profile kept as an error instead of failing startup.
- `cmd/sourcebolt/{status.go,login.go,serve.go,setup.go,main.go}`: status --help, login replaces undecryptable profiles, `%` never through `.cmd` CLIs, --help exits 0.
- `internal/transport/rpc/rpc.go`: cancel reported as cancel.
- `install.ps1`, `install.sh`, `scripts/test-installers.*`, `.github/workflows/{ci,release}.yml`, `.goreleaser.yaml`, `tools/distcheck`: installers, real-binary CI, npm staging/OIDC, token form test.
- `docs/**`, `llms-install.md`, `tools/termshot`, `docs/assets/{term,guides}/*`: guides, screenshots, install pages.

# What changed — key edits this session

Bugs found by end-to-end runs of the real binary and fixed failing-test-first: `setup --help` exit 2; `%` expanded by `cmd.exe` through `code.cmd`; macOS keychain hang; cancel reported as AUTH_REQUIRED; GoReleaser token template; `status --help` printing status; Windows MSIX AppData virtualization splitting profiles plus a shared keyring entry; `serve` exiting (and calling Google with no cookies) on an unreadable profile.

# What failed — dead ends, do NOT retry these

- Re-tagging a published version (v0.3.0 stays unpublished; fixes ship as new versions).
- `envOrDefault` / `index .Env` in GoReleaser `token:` fields: only `{{ .Env.NAME }}` works; dry runs never expand tokens.
- Assuming one `%APPDATA%` on Windows: packaged apps (Claude desktop, Store apps) can see a virtualized copy. Data now lives in `%USERPROFILE%\.sourcebolt`.
- Approving only some staged npm packages: always check all 7 versions after the owner approves (Linux packages were missed twice).
- `mcp-publisher` login long before publishing: the registry token expires within minutes; chain login and publish.
- computer-use screenshots of this PC: windows of non-granted apps are masked; the owner's Win+PrtScn captures are clean. IDEs and terminals are click-only.
- Git Bash rewrites `origin/main:path` into a Windows path: set `MSYS_NO_PATHCONV=1`.

# Next steps — exactly what to do next

1. Watch the external submissions and answer reviewers: microsoft/winget-pkgs#457468 (CLA signed; 6 of 7 validation stages passed, installer scan pending), TensorBlock/awesome-mcp-servers#4256, docker/mcp-registry#5929 (its workflows wait for a maintainer), cline/mcp-marketplace#3144, mcpservers.org.
2. Next release (owner decision 2026-10-21): Cursor support and its two screenshots (owner installs Cursor), and PyPI publishing (owner sets up a PyPI trusted publisher first). Claude Desktop, Windsurf and Gemini CLI screenshots only if those apps get installed.
3. Held, needs an explicit go: launch posts.
4. Ignored for now (owner, 2026-10-21): the NPM_TOKEN expiry and the switch to trusted publishing only; housekeeping (capture-3 notebook, old `%APPDATA%\sourcebolt`, upgrading the local install).
