# win4r/notebooklm-py

- URL: https://github.com/win4r/notebooklm-py
- Language / runtime: Python (fork of teng-lin/notebooklm-py at v0.3.4 plus upstream main up to d6cef80)
- License: MIT, "Copyright (c) 2026 Teng Lin" (`LICENSE:1-3`, inherited)
- Stars / last commit: 192 stars (research/upstreams.md); HEAD `204c84c` 2026-04-23 "v0.3.4-hermes.4"
- Versions reviewed: tags `v0.3.4`, `v0.3.4-hermes.1` to `v0.3.4-hermes.4` (`git tag`); merge-base with upstream `d6cef809` (2026-04-19). `git rev-list --left-right --count up/main...HEAD` = 1874 behind, 15 ahead (matches research/upstreams.md).

## Access method

Same as teng-lin at that age: internal `batchexecute` RPC (see `research/notes/teng-lin__notebooklm-py.md` for the wire details). Nothing new on the wire. The fork only touches auth, packaging and docs (`git diff --stat up/main...HEAD`: 9 files, +1217/-24: `.gitignore`, `CHANGELOG.md`, `README.md`, `skills/notebooklm/SECURITY_AUDIT.md`, `skills/notebooklm/SKILL.md`, `skills/notebooklm/import_browser_cookies.py`, `src/notebooklm/auth.py`, `src/notebooklm/cli/session.py`, `tests/unit/test_auth.py`).

## What the 15 commits are

`git log d6cef80..HEAD`:

| Count | Author | Content |
|---|---|---|
| 11 | win4r | Hermes-agent skill layout (`skills/notebooklm/`, 586-line `SKILL.md`), `SECURITY_AUDIT.md` (manual static scan, "LOW RISK"), `import_browser_cookies.py`, README rewrite, install pinned to fork tags `v0.3.4-hermes.N` (`git log`, e.g. `9794cf6`, `6bbdc47`, `d039757`, `4a73cda`) |
| 1 | Jagger.H | `0581faa` feat(auth): auto-refresh cookies via `NOTEBOOKLM_REFRESH_CMD` (upstream PR #298, 3 files +218/-10) |
| 1 | SHIDO, Yuichiro | `21bba1c` use `sys.executable -m playwright` for subprocess (upstream PR #279) |
| 2 | win4r | version bump / doc commits (`6e8f16e`, `204c84c`) that restore "main == tag" |

The two substantive code changes are upstream cherry-picks, not original win4r work. So this fork contains no unique RPC or protocol knowledge.

## Auth lifecycle (the feature of interest)

### `NOTEBOOKLM_REFRESH_CMD` (commit 0581faa, `src/notebooklm/auth.py`)

Behavior, as implemented in that commit (read via `git show 0581faa -- src/notebooklm/auth.py`):

- `fetch_tokens(cookies)` wraps `_fetch_tokens_once`. On `ValueError` whose lower-cased message contains one of `"authentication expired"`, `"redirected to"`, `"run 'notebooklm login'"`, and only if env `NOTEBOOKLM_REFRESH_CMD` is set and env guard `_NOTEBOOKLM_REFRESH_ATTEMPTED` is unset, it runs the command once.
- Runs `subprocess.run(shlex.split(cmd), capture_output=True, text=True, timeout=60)`; non-zero exit or timeout raises `RuntimeError` with stderr.
- The command is expected to rewrite `~/.notebooklm/storage_state.json` (for example a cookie-vault sync script). Then `load_auth_from_storage()` reloads, the caller's cookie dict is cleared and updated in place, and the token fetch retries exactly once.
- The process-scoped env flag prevents refresh loops. 4 unit tests (`tests/unit/test_auth.py`, +123 lines): unset = no refresh, success + retry, no infinite loop, failure surfaces as RuntimeError.
- README suggests `NOTEBOOKLM_REFRESH_CMD="notebooklm login --browser-cookies chrome"` to self-heal (`skills/notebooklm/SECURITY_AUDIT.md`, "Hardening applied").

Upstream status: teng-lin merged the same idea (commit `76829ec7` "feat(auth): auto-refresh cookies via NOTEBOOKLM_REFRESH_CMD (#336)", released v0.4.1) and later hardened it: `shell=False` via shlex by default (`c33f0072`), cross-process lock, mid-session variant, `NOTEBOOKLM_REFRESH_CMD_USE_SHELL`, `..._MIDSESSION`, `..._LOG_OUTPUT` (`teng-lin ... _auth/paths.py:49-61`, ADR-0030 rung L2.5). The win4r version is the obsolete first draft.

### Browser-cookie import (`skills/notebooklm/import_browser_cookies.py`, 128 lines)

- NOT a browser-database reader. It converts a manually exported JSON file (Cookie-Editor or "Get cookies.txt LOCALLY") into Playwright `storage_state.json` (`import_browser_cookies.py:1-12, 104-123`).
- Keeps only cookies for domains `google.com`, `notebooklm.google.com`, `accounts.google.com`, `myaccount.google.com`, `googleusercontent.com`, `youtube.com`, `ytimg.com` (`:24-32`). Requires at least one of `SID`, `__Secure-1PSID`, `__Secure-3PSID` (`:34-35`). Maps sameSite (`:41-50`), converts `expirationDate` to `expires` (`:67-70`), writes `~/.notebooklm/storage_state.json` with chmod 600 (`:110-117`). Supports `--dry-run` and `--out`.
- Motivation recorded in the file: a fresh Playwright Chromium login can trigger Google abuse detection and a ~48 h cooldown, so reuse an existing real-browser session (`:8-10`). README admits the imported session goes stale in 15-30 minutes ("a 15-30min bootstrap, not a long-term solution", commit `bb7eedb`), which is why `NOTEBOOKLM_REFRESH_CMD` was added.
- Upstream later shipped `login --browser-cookies <browser>` via the `rookie-cookies` extra that reads the browser store directly (`teng-lin docs/auth-cookie-lifecycle.md`, "Browser-cookie imports"), superseding this script.

## Skill / security audit pieces worth knowing

- `SECURITY_AUDIT.md` is a Claude-written manual scan listing the only subprocess uses (`playwright install chromium` and `$NOTEBOOKLM_REFRESH_CMD`), outbound hosts (`*.google.com`, `youtube.com`), no telemetry, chmod 600 storage, and an "upgrade guardrail" in `SKILL.md` pinning `notebooklm-py==0.3.4` because the skill holds SID cookies. Good checklist for our own release hygiene, not code.
- The `skills/notebooklm/SKILL.md` is the upstream agent skill copy relocated to the Hermes layout.

## MCP tools
None. CLI/skill only.

## RPC catalog, Streaming/chat
No changes versus the v0.3.4 upstream; see `research/notes/teng-lin__notebooklm-py.md`.

## Versions

`v0.3.4` (upstream 2026-03-12), `v0.3.4-hermes.1` (2026-04-23, src byte-identical to upstream v0.3.4), `.2` (2026-04-23, merge upstream delta + cherry-pick #298/#279), `.3` and `.4` (2026-04-23, docs only). Python package version stays 0.3.4 (`pyproject.toml`; `SECURITY_AUDIT.md:3`). Abandoned snapshot: 1874 commits behind a repo that has reached v0.8.5.

## Adopt

1. Idea of an operator-supplied refresh hook as one rung of an auth recovery ladder (`git show 0581faa`, now upstream L2.5). In Go: `exec.CommandContext` with `shlex`-style split, no shell, 60 s timeout, one attempt per process/per failure window, reload cookie store afterwards.
2. Cookie-export importer accepting both a raw JSON array and `{cookies:[...]}`, filtering to Google domains, requiring a session-anchor cookie, writing 0600 (`import_browser_cookies.py:84-123`). Cheap fallback import path for users who cannot run a browser login.
3. Pin-and-audit discipline for a skill/binary that holds account cookies (`SECURITY_AUDIT.md`).

## Avoid

1. Detecting auth expiry by substring-matching exception text (`"authentication expired"`, `"redirected to"`) (`0581faa` `_AUTH_ERROR_SIGNALS`): brittle; use typed errors.
2. Process-wide env var as the "attempted" latch (`_NOTEBOOKLM_REFRESH_ATTEMPTED`): leaks across calls and cannot express a cooldown.
3. `shlex.split` command from env without an allow-list when running as a long-lived server; the upstream later added opt-in shell mode and logging controls.
4. Forking a fast-moving unofficial-API client at an old tag: the fork is 1874 commits behind and misses every RPC id and shape fix since March 2026.

## Evidence

`git log --format=... d6cef80..HEAD`; `git diff --stat up/main...HEAD`; `git show 0581faa` (full `auth.py` diff read); `skills/notebooklm/import_browser_cookies.py` (read in full); `skills/notebooklm/SECURITY_AUDIT.md` (first 70 lines); upstream clone `git log --all --grep=REFRESH_CMD`. Remote `up` was added only inside the clone's git config for comparison; no code run.
