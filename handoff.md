# Handoff

Updated 2026-10-20.

# Goal
Build **sourcebolt**: one Go MCP server (official go-sdk v1.8.0, Go 1.27, Apache-2.0, free) for Google NotebookLM. It unites the features of every public NotebookLM MCP upstream and beats them on speed. It uses internal batchexecute RPC first, with a browser fallback. Distribution: GitHub release binaries, GHCR Docker, npm, PyPI and the MCP registry. Repo: candidloom-hub/sourcebolt (private until the release gate). Source of truth: `docs/plans/2026-10-14-architecture.md`, `docs/mission-prompt.md`.

# Current state
- **v0.2.0 is released (2026-10-20)** and verified: checksums, the cosign signature, the binary's version, the `.mcpb` manifest (30 tools), the GHCR images `0.2.0` and `0.2.0-browser`, and `go install @v0.2.0` through the proxy.
- `main` is at the v0.2.0 tag commit. CI is green. There are no open PRs, agent worktrees or stale branches.
- Live captures #2, #3 and #3b are done. The owner deletes the capture notebooks.
- The DOM fallback was verified live with `doctor --dom-check` (5.2 s).
- Performance (CI, 2 CPUs): `notebook_list` 0.758 ms uncached and 0.398 ms cached; cold start 8.2 ms; idle memory 16.2 MB; chat first progress +0.30 ms.
- On hold until the owner approves: npm, PyPI, the MCP registry, marketplace and directory submissions (see `research/marketplaces.md` and `docs/launch-plan.md`), and launch posts. Donations are off (option C).

# Files touched
- `cmd/sourcebolt/{main,serve,call,login,status}.go`: CLI (serve/call/login/logout/status/doctor); help documents NOTEBOOKLM_BROWSER_FALLBACK=0.
- `internal/wire/*`: batchexecute encode/decode, lazy `wire.Node` with pooled decoders (ListNotebooks(100): 40064 → 361 allocs), stream frames.
- `internal/rpcids/*`: RPC id table plus NOTEBOOKLM_RPC_IDS override.
- `internal/session/*`: AES-256-GCM cookie jar, key in the OS keyring, lock file, import (netscape/json/header), SID required.
- `internal/transport/{transport,errors,router}.go`: Transport interface, error kinds, `Sent` flag, fallback policy.
- `internal/transport/rpc/*`: batchexecute client plus GenerateFreeFormStreamed chat stream.
- `internal/transport/browser/*`: chromedp launch, `login --browser/--attach`, in-page fetch fallback.
- `internal/nlm/*.go`: typed NotebookLM ops (notebooks, chat, sources, research, notes, delete, studio, sharing, labels, account, options).
- `internal/{cache,resolve,jobs,safety,app}`: cache, title→id resolve, async jobs, confirm gates, app wiring.
- `internal/mcpserver/*`: tool registration per group, error envelope in text content, progress, `tools_common.go` shared helpers.
- `tools/capture/*`: HAR → sanitized fixtures (ids, emails, URLs, long numbers, free text replaced).
- `tools/gendocs`: generates `docs/tools.md`.
- `testdata/fixtures/*`: 21 live fixtures from capture #1.
- `bench/*`: coldstart, rss, interleave.sh, compare.sh, RESULTS.md.
- `.github/workflows/{ci,bench,release}.yml`, `.goreleaser.yaml`, `Dockerfile*`, `npm/`, `python/`, `scripts/`: CI, bench gate, release pipeline, wrappers.
- `README.md`, `docs/{tools,auth,security}.md`, `SKILL.md`, `server.json`, `CHANGELOG.md`, `NOTICE`: user docs and registry metadata.
- `research/*`, `docs/plans/*`: recon notes (37 repos), feature matrix, RPC catalog, plans P1–P5.

# What changed
- P1 core merged: wire, session, rpc transport, router, CLI.
- P2 merged: 26+ MCP tools covering chat (streamed), research, notes, delete, studio list/get, sources, sharing, labels, account, notebook create/update, chat_ask_many, plus jobs, progress and the browser fallback.
- Two whole-branch Opus reviews ran. All Important findings were fixed failing-test-first. Agent code was mutation-checked (5/5, 5/5, 4/4).
- Bench gate: base and head run interleaved. Thresholds are alloc 10% and time 25%. `TIME_EXCLUDE ^ToolCallNotebookList` skips that benchmark's time (GC cost of about 30 tool schemas).
- Tests set NOTEBOOKLM_BROWSER_FALLBACK=0 so they never launch Chrome.
- P5 docs and registry metadata are on feat/release.

# What failed
Do NOT retry these.
- **`login --browser`** (chromedp): Google blocks it ("browser may not be secure", `navigator.webdriver`). Use `login --attach http://127.0.0.1:9222` on a plain Brave started with `--remote-debugging-port=9222 --user-data-dir=%APPDATA%\sourcebolt\browser\default`.
- **Partial cookie export** (no SID/HSID/APISID): it returns the signed-out shell (200, no SNlM0e). Do not relax the SID requirement.
- **Clipboard export from an unfocused or minimized tab**: Runtime.evaluate times out, and once the old clipboard (user text) was saved instead. Front the tab first. Check the file starts with `{"log"`. Skeletonize payloads over about 1 MB first.
- **Page-side download or POST to localhost**: page CSP and download blocking stop it.
- **Computer-use clicks in Brave**: browsers are view-only (read tier). Use Claude in Chrome.
- **`-race` on Windows**: no cgo. It runs only in CI.
- **Shell quoting**:
  - bash heredocs break on `)]}'`;
  - `node -e` breaks on apostrophes;
  - a stray `python -` hangs on stdin.
  Use the Write tool or script files instead.
- **sed mutation tests that matched nothing**: they gave false "survivors". Diff after each mutation.
- **Agent worktrees start from main**: always tell the agent to `git reset --hard <base>` first.
- **Base-then-head bench runs**: they drift. Use interleave.sh.

# Next steps
1. Owner-gated publishing, in this order, each only on an explicit go:
   - `NPM_TOKEN` and the PyPI trusted publisher, then run `release.yml` with the publish inputs on;
   - add the npm and PyPI blocks back to `server.json` (`docs/registry.md`), then `mcp-publisher publish`;
   - directory submissions;
   - launch posts.
2. Owner hands-on checks: the `.mcpb` bundle in Claude Desktop, the Claude Code plugin, and the VS Code and Cursor install links.
3. The next capture, if wanted: other Studio option values, Studio media URLs, the conversation-list paging trigger. A demo GIF for the README.
4. Donations: an e-PAN plus sole proprietorship for Stripe India, or Open Collective. Then add `.github/FUNDING.yml`.
