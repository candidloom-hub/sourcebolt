# Handoff

Updated 2026-10-20.

# Goal
Build **sourcebolt**: one Go MCP server (official go-sdk v1.8.0, Go 1.27, Apache-2.0, free) for Google NotebookLM. It unites the features of every public NotebookLM MCP upstream and beats them on speed. It uses internal batchexecute RPC first, with a browser fallback. Distribution: GitHub release binaries, GHCR Docker, npm, PyPI and the MCP registry. Repo: candidloom-hub/sourcebolt (private until the release gate). Source of truth: `docs/plans/2026-10-14-architecture.md`, `docs/mission-prompt.md`.

# Current state
- Repo candidloom-hub/sourcebolt is **public**; private vulnerability reporting is on. History was rewritten with git filter-repo before going public; old private repo: candidloom-hub/sourcebolt-archive (local remote `archive`).
- **main** has P1 to P5, the backlog fixes (PR #8), the design-call fixes (PR #13) and the Dependabot bumps (#7, #9, #10, #11). CI green.
- Merged 2026-10-20:
  - PR #12, B3 perf: CI p50 2.26 ms uncached, 1.00 ms cached; allocations −72%.
  - PR #14, DOM fallback (P4 Task 5): reviewed, and all 10 review findings fixed.
- Release: tag v0.1.0 and the GitHub release are approved (2026-10-20). **On hold until the owner approves:** npm, PyPI and the MCP registry listing.
- Capture #2 done 2026-10-20: 40 sanitized fixtures (PR #17, merged). Pinned shapes enabled flashcards generation, source/note/chat-history delete and the source guide, and fixed six request builders, source text nesting and citation decoding. The owner still has to delete the capture notebooks ("sourcebolt capture 2d" and the capture #1 notebook).
- v0.1.0 released 2026-10-20 (GitHub release with binaries, SBOMs and a cosign-signed checksums file; GHCR images `0.1.0` and `0.1.0-browser`).
- Live verified (2026-10-14): `login --attach`, read tools on 9 notebooks (RTT p50 848 ms), notebook_create → source_add → chat_ask with citation (18.4 s).

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
1. Optional, owner-gated: check the DOM fallback against the live NotebookLM page (force drift with NOTEBOOKLM_RPC_IDS).
2. B3: the uncached path is still above 2 ms on CI; a smaller default page is the remaining lever.
3. Next capture (when needed): other Studio types and flashcards options, saving a chat answer as a note, paging of the conversation list, the markdown mode of `source_read`. Method: see CLAUDE.md lessons.
4. Release: in CHANGELOG.md, turn [Unreleased] into [0.1.0], tag v0.1.0 on main, check the draft GitHub release, then publish it. npm, PyPI and the registry stay on hold.
5. Owner-only: the `NPM_TOKEN` secret and the PyPI trusted publisher (when npm and PyPI are approved); delete the capture notebooks.
