# ivanovich1071/notebooklm-mcp

- URL: https://github.com/ivanovich1071/notebooklm-mcp
- Language / runtime: TypeScript / Node 20, patchright (browser automation). Package still named `@roomi-fields/notebooklm-mcp`, version 1.5.6 (package.json:2-3)
- License: MIT (inherited LICENSE from roomi-fields). Behavior notes only needed; nothing here is worth copying.
- Stars / last commit: not re-queried; HEAD 4e357d8 2026-02-22 "fix: prevent husky from failing in production by checking command existence"
- Versions reviewed: HEAD only; 0 tags; 134 commits. Merge base with roomi-fields is `aa763f5` (2026-02-15, "fix(security): update @modelcontextprotocol/sdk and qs vulnerabilities", roomi-fields tag area v1.5.6). `git merge-base HEAD aa763f5` returns aa763f5 itself, so every commit after it is the fork's own.

## Verdict on "22 commits ahead of roomi-fields"

The 22 commits (aa763f5..HEAD, 2026-02-19 to 2026-02-22; authors "Sokol Yauheni" and "ivanovich1071"; mostly titled "Update Dockerfile / .dockerignore / types.ts") are deployment hotfixes for a Linux container / Render host, NOT feature work. `git diff --stat aa763f5 HEAD` touches 10 files: `.dockerignore`, `Dockerfile` (rewritten, multi-stage), `package.json`, `tsconfig.json`, `src/http-wrapper.ts`, `src/i18n/index.ts`, `src/tools/index.ts`, `src/types.ts`, and deletes two stray debug PNGs.

It is a fork of the PRE-RPC roomi-fields (v1.5.6, browser/DOM only). There is no `src/rpc/` directory and `grep LabsTailwind|batchexecute|wXbhsf src` finds nothing. So it gives us no RPC information and nothing roomi-fields v3.2.0 lacks. The "RPC primary" behavior exists only in roomi-fields >= v3.0.0 (see roomi-fields__notebooklm-mcp.md).

## Access method

Browser automation only (patchright persistent Chrome profile, DOM selectors localised through `src/i18n/*.json`). No RPC.

## What the fork changed (all evidence is `git diff aa763f5 HEAD`)

- `ToolResult.error` changed from `string` to `{ message: string }` ("to comply with MCP spec recommendations", src/types.ts around line 99-110) and every `return { success:false, error: msg }` in src/tools/index.ts wrapped accordingly (e.g. rate-limit branch). http-wrapper 404 detection changed to `result.error?.message?.includes('not found')`. Commit 590a3d1 "resolve TypeScript errors by wrapping error strings in objects". Effect: breaks the REST error shape that n8n clients of roomi-fields expect; no evidence of tests being updated [unverified].
- `src/i18n/index.ts` rewritten: `LocaleData` collapsed to `Record<string,string>` groups, supported locales shrunk to `'fr' | 'en'`, default locale `'fr'` (i18n/index.ts diff). Upstream at that time had the same two locales; no new language.
- `package.json`: `build` uses `cp src/i18n/*.json dist/i18n/` (POSIX-only), `prepare` becomes `if command -v husky ...; fi` so `npm ci --omit=dev` in Docker does not fail, e2e scripts replaced by `echo "E2E tests disabled in Linux container"`.
- `tsconfig.json`: `module/moduleResolution` Node16 -> NodeNext, source maps/declarations turned off, strictness flags `noUnusedLocals/Parameters` removed (commit bc33d2b "support ESM and import.meta for Render deployment").
- `Dockerfile`: two-stage build on `node:20-bookworm-slim`; installs Chromium runtime libs plus `xvfb x11vnc novnc websockify fluxbox`; non-root user `notebooklm`; `npx patchright install chromium || true` (swallows failure); env `HTTP_PORT=3000 HTTP_HOST=0.0.0.0 HEADLESS=true NOTEBOOKLM_DATA_DIR=/data DISPLAY=:99 NOVNC_PORT=6080`; `HEALTHCHECK ... wget --spider http://localhost:3000/health`; `VOLUME /data`; entrypoint `scripts/docker-entrypoint.sh` starts VNC services then `node dist/http-wrapper.js` (Dockerfile:60-130, scripts/docker-entrypoint.sh).
- DOCKER-FIXES.md (inherited from roomi-fields v1.5.3) lists the Chromium flags needed in a container: `--no-sandbox --disable-setuid-sandbox --disable-gpu --disable-infobars` and the first-login flow: open noVNC at port 6080, `POST /setup-auth {"show_browser":true}`, log into Google inside VNC.

## RPC catalog

None. Not applicable: no RPC code in this fork. For RPC details of the same lineage see roomi-fields__notebooklm-mcp.md.

## Streaming/chat

DOM: types the question in the chat textarea and scrapes the answer; citation extraction was rewritten in roomi-fields v1.5.6 ("citation extraction rewrite", commit 2625867) to use a `page.evaluate` approach. No wire protocol.

## Auth lifecycle

Inherited from roomi-fields v1.5.6: patchright persistent profile + per-account `state.json`, AutoLoginManager (auto-reauth), accounts with rotation. Container variant persists to `/data` volume and does interactive Google login through noVNC.

## MCP tools

Same as roomi-fields v1.5.6 (pre-v2 flat names: ask_question, add_notebook, list_notebooks, get_notebook, select_notebook, update_notebook, remove_notebook, search_notebooks, get_library_stats, list_sessions, close_session, reset_session, get_health, setup_auth, de_auth, re_auth, cleanup_data, plus content/source/notes tools). Not renamed to the v2 `namespace_action` form, and none of the v3 RPC-only tools (share_notebook, generate_study_aid, generate_mind_map, manage_labels, research_sources, source_read, content_delete). Tool list not re-read line by line [unverified beyond the diff].

## Versions

No tags. package.json 1.5.6. Timeline: roomi-fields commits up to 2026-02-15, then fork commits 2026-02-19 .. 2026-02-22.

## Adopt

- Container recipe for headful login in Docker: Xvfb + x11vnc + noVNC + fluxbox, `DISPLAY=:99`, port 6080, then trigger login via the API (Dockerfile:12-40,70-90, scripts/docker-entrypoint.sh, DOCKER-FIXES.md). Relevant for our Docker channel's browser fallback.
- Non-root user, `/data` volume, `HEALTHCHECK` on `/health` (Dockerfile:105-125).
- `prepare` guarded so dev-only git hooks do not break production installs (package.json `prepare`).

## Avoid

- Everything feature-wise: nothing beyond roomi-fields v1.5.6. Do not treat it as a separate RPC implementation.
- Changing the public error shape to `{message}` with no versioned API (types.ts) breaks clients.
- `npx patchright install chromium || true` hides a failed browser download (Dockerfile ~line 100) so the image can build but have no browser.
- Disabling `noUnusedLocals`, declarations and e2e scripts to get a deploy to pass (tsconfig.json, package.json).
- Hardcoded default locale `fr` (i18n/index.ts).

## Evidence

`git log` (first 40), `git merge-base`, `git diff --stat aa763f5 HEAD`, diffs of package.json, tsconfig.json, src/types.ts, src/tools/index.ts, src/http-wrapper.ts, src/i18n/index.ts, Dockerfile, scripts/docker-entrypoint.sh, DOCKER-FIXES.md; grep for RPC strings in src. REA ran (113 KB markdown inventory) but its node output prints `[object Object]`, so it added only a file inventory.
