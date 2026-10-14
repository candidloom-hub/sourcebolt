# Handoff

Updated 2026-10-14, after the session context filled up.

# Goal
Build **sourcebolt**: one Go MCP server (official go-sdk v1.8.0, Go 1.27, Apache-2.0, free) for Google NotebookLM. It unites the features of every public NotebookLM MCP upstream and beats them on speed. It uses internal batchexecute RPC first, with a browser fallback. Distribution: GitHub release binaries, GHCR Docker, npm, PyPI and the MCP registry. Repo: candidloom-hub/sourcebolt (private until the release gate). Source of truth: `docs/plans/2026-10-14-architecture.md`, `docs/mission-prompt.md`.

# Current state
- Branches:
  - Repo is a fresh candidloom-hub/sourcebolt (private), built from history rewritten with git filter-repo to remove the owner's local path. Old repo: private candidloom-hub/sourcebolt-archive (old PRs #1-#9; local remote `archive`). All SHAs changed.
  - **main** has P1 and P2 tools.
  - **feat/release** is pushed and NOT merged; PR candidloom-hub/sourcebolt#1. P5 docs, gendocs, registry metadata (`server.json`, npm `mcpName`, OCI label, README `mcp-name` comment).
  - Tests, `goreleaser check` and the npm tests are green.
- Nothing is released. No tag exists. The repo is still private.
- Live verified:
  - `login --attach` works.
  - Read tools work on 9 notebooks (RTT p50 848 ms).
  - End-to-end notebook_create → source_add → chat_ask answered with a citation (18.4 s).
- **Release gate APPROVED by the owner** ("4. approved, go for it"). It covers: make the repo public, merge feat/release → main, tag v0.1.0, publish.
- **Capture #2 is in progress** in a Claude in Chrome tab, on a throwaway capture notebook (id kept out of git).
  - `window.__sbcap` holds 125 calls.
  - `window.__sbhar` holds a skeletonized HAR (123K).
  - The clipboard export timed out because the tab was not focused.
  - We are waiting for the owner to front the tab, do the deletes, rename and chat-history delete, then say "done".
- Secret scan done: gitleaks v8.28.0 over all rewritten history, no leaks; custom grep for owner email, name, cookie values and capture ids, none.
- Dependabot setup-go 7 PR lived in the old repo; Dependabot reopens it in the new repo on its own.

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
1. Update CLAUDE.md "Current state" (main has P1+P2; feat/release pending; release gate approved). Save the lessons to memory (see the session's final message for the list and locations).
2. **Capture #2**:
   - After the owner says "done", export `window.__sbhar` via the clipboard from the focused tab.
   - Save it to the scratchpad and run `go run ./tools/capture --har <file> --out testdata/fixtures`.
   - Leak-scan for the email, real ids and cookie names.
   - Pin the shapes: R7cb6c configs, deletes, tr032e, hizoJc, CYK0Xb chat-save, cYAfTb, mixed izAoDd, EylDcb cost table.
   - Ask the owner to delete the capture notebook.
3. **Release** (approved):
   - Make the repo public, then merge feat/release → main via PR candidloom-hub/sourcebolt#1 once CI is green.
   - Tag v0.1.0. That produces a draft GH release and GHCR images.
   - npm needs the owner's `NPM_TOKEN` repo secret. PyPI needs the owner to configure a trusted publisher. Both are owner-held, so they cannot be automated.
   - Owner enables private vulnerability reporting.
4. Handle the Dependabot setup-go 7 PR when it reappears in the new repo.
5. Engineering backlog:
   - P4 Task 5 DOM fallback.
   - B3 refresh path (3.3 ms on CI vs the 2 ms target).
   - Do not fall back to the browser on connection-refused network errors.
   - ResponseHeaderTimeout 30 s vs stream first chunk.
   - Deferred minors in `.superpowers/sdd/*/progress.md`.
   - Research capture.
6. Owner-only steps:
   - Sign out of other Google sessions.
   - `Remove-Item "$env:USERPROFILE\Workspaces\AI\notebook.google.com_cookies.txt"`
   - `.\bin\sourcebolt.exe call delete '{"kind":"notebook","ids":["sourcebolt test (safe to delete)"],"confirm":true}'`
   - Clean up stale agent worktrees (`git worktree list`).
