# tenyoru/notebooklm-mcp

- URL: https://github.com/tenyoru/notebooklm-mcp (npm `notebooklm-mcp-firefox`)
- Language / runtime: TypeScript (ESM), Node >= 18, MCP SDK, Patchright (imports both `chromium` and `firefox`, `src/session/shared-context-manager.ts:16`, `src/auth/auth-manager.ts:915`).
- License: MIT, "Copyright (c) 2025 Please Prompto!" (LICENSE:1-3). Snippets allowed.
- Stars / last commit: 0 stars; HEAD `acbc5d5` 2026-03-15. Version 1.2.2 (tag v1.2.2, 2026-03-15).
- Versions reviewed: tag v1.2.2; HEAD acbc5d5. History is PleasePrompto up to `fd0525f` (v1.2.1) then 5 commits (2026-03-15) by the fork author. Clone: `research/upstreams/tenyoru__notebooklm-mcp`.
- Size of the delta: `git diff --stat fd0525f HEAD` = 7 files, +396/-39: `CHANGELOG.md`, `docs/BROWSER_CONFIGURATION.md` (+272), `package.json`, `src/auth/auth-manager.ts` (+45), `src/config.ts` (+6), `src/session/shared-context-manager.ts` (+73).
- REA: `rea analyze-javascript-application ... --artifact-format directory` (259 modules). Module map is the parent v1.2.1 map unchanged (`src/{auth,session,library,tools,resources,utils}`); only `patchright` as browser dependency. No other analysis signal.

## Firefox support approach

The whole approach is a thin switch around `launchPersistentContext`:

1. Config: `BROWSER_TYPE` env (`chrome` default | `chromium` | `firefox` | `zen`) and `BROWSER_EXECUTABLE_PATH` (`src/config.ts:35, 91-92, 176-177`).
2. Pick the Patchright/Playwright BrowserType object: `firefox` for `firefox` and `zen` (Zen is a Firefox fork), otherwise `chromium` (`shared-context-manager.ts:26-37`).
3. Pick channel: `"chrome"` only for `chrome`, nothing for chromium/firefox, and none for zen "because Zen uses executablePath" (`:41-56`).
4. Launch options common to all: `headless`, `viewport`, `locale:"en-US"`, `timezoneId:"Europe/Brussels"` (changed from Berlin, commit 24f64bd), `storageState: statePath`, optional `executablePath` (`:188-212`).
5. **Args**: for Firefox-based browsers the args array is empty ("Chromium flags don't work with Firefox/Zen", `:203-215`, same in auth-manager.ts:946-957); for Chromium-based, the parent's four flags including `--disable-blink-features=AutomationControlled`. Commit acbc5d5 ("Fix Firefox/Zen browser launch - remove Chromium-only args") and 31618c1 ("use Firefox API instead of Chromium") are two bug-fix iterations the same day, suggesting it was tried until it started.
6. Same code path for the interactive login (`auth-manager.ts:915-960`) and runtime (`shared-context-manager.ts:219-228`). The profile directory keeps the name `chrome_profile` for all browsers; switching browser type requires deleting the profile because formats differ (`docs/BROWSER_CONFIGURATION.md` "Profile conflicts", "Session Persistence").
7. Everything else (selectors, answer detection, stealth) is untouched. There is **no Firefox-specific selector or stealth work**; `--disable-blink-features=AutomationControlled`, Patchright's CDP-leak patches and `humanType` have no equivalent on Firefox, so the "stealth" statement in the docs ("Combined with NotebookLM MCP's stealth features, Zen Browser provides an excellent balance", docs/BROWSER_CONFIGURATION.md "Why Use Zen Browser?") is not backed by code. The README (line 383) states that detection by Google cannot be excluded.

Validity caveats (not verified by running anything here):
- Playwright's Firefox driver speaks a patched-Firefox protocol (Juggler) and ships its own Firefox build; pointing `executablePath` at a stock Firefox or Zen install generally does not work [unverified; general Playwright knowledge]. The docs only show log lines and troubleshooting for "Executable doesn't exist", no evidence of an end-to-end NotebookLM session on Firefox/Zen.
- Patchright's anti-detection patches target Chromium [unverified; patchright project statement], so on Firefox it behaves as plain Playwright.
- No tests, no changelog evidence of a successful run.

Implications for our Go fallback: Firefox automation without Playwright would need WebDriver BiDi (Firefox supports BiDi via `--remote-debugging-port`; geckodriver) or `playwright-go`. chromedp/rod/CDP are Chromium-only. Given NotebookLM works in Chromium and the fallback is a fallback, support "any Chromium-family browser by path" (Chrome, Edge, Brave, Chromium: see icebear0828 `detectChromePath`) rather than Firefox.

## Access method

Browser automation only (v1.2.1 base), no RPC.

## RPC catalog

None.

## Streaming/chat

v1.2.1 logic: gates on `div.thinking-message` visibility then 3-poll stability (`src/utils/page-utils.ts:195-203`; grep PLACEHOLDER finds nothing). That element no longer exists in the current UI (PleasePrompto CHANGELOG 2.0.0 removed it as broken), so answer detection here reduces to the stability poll; `.to-user-container` selector is the parent's. It does not include the v2.0.0 placeholder filter, so "Thinking..." text can be returned as the answer [inferred from absence of PLACEHOLDER_SNIPPETS; CHANGELOG 1.2.0 says they were removed]. No `notebook.google.com` host support: login detection uses `startsWith("https://notebooklm.google.com/")` (`src/auth/auth-manager.ts:325, 349, 532`), so it breaks after the rename.

## Auth lifecycle

Parent's: interactive login in a visible window, `state.json` + `session.json`, 24 h age rule, optional credentials auto-login. Browser selection applies to the login window too (`auth-manager.ts:915-960`). Multi-account: not in this base (added upstream in v2.0.0).

## Selectors

Parent v1.2.1 (aria-label EN/DE; `textarea.query-box-input`, `.to-user-container`). Nothing new.

## Stealth / anti-detection

Chromium path unchanged (persistent profile + `--disable-blink-features=AutomationControlled` + human typing). Firefox path: none beyond human-like typing and delays (`stealth-utils.ts` unchanged).

## Browser / page lifecycle, pooling, timeouts, concurrency

Same as parent v1.2.1: one persistent context, tab per session, maxSessions/timeout, profile strategy auto/single/isolated, no lock. Log strings now say the browser name (`CONFIG.browserType.toUpperCase()`, shared-context-manager.ts:219-228, 303, 340, 353).

## Notebook library / session management

The 16 tools of PleasePrompto v1.2.x: ask_question, add/list/get/select/update/remove/search notebooks, get_library_stats, list/close/reset session, get_health, setup_auth, re_auth, cleanup_data (`src/tools/definitions/*`).

## MCP tools

Same as PleasePrompto v1.2.x minus v2.0.0 additions (no add_source, audio, citations, HTTP transport). Table: see notes/PleasePrompto__notebooklm-mcp.md "MCP tools" rows 1-16; tool profiles minimal/standard/full present.

## Reliability

Parent's. The only new failure mode documented is "Executable doesn't exist" for a wrong `BROWSER_EXECUTABLE_PATH` (docs/BROWSER_CONFIGURATION.md "Troubleshooting").

## Performance notes

No change.

## Tests and quality

None added.

## Versions

| Tag | Date | Notable |
|---|---|---|
| v1.2.2 | 2026-03-15 | multi-browser support (Chrome, Chromium, Firefox, Zen), timezone Europe/Brussels (CHANGELOG 1.2.2) |
Commits: `503c802 Add multi-browser support`, `24f64bd timezone Berlin->Brussels`, `75785c5 Release v1.2.2`, `31618c1 Fix Zen Browser support - use Firefox API instead of Chromium`, `acbc5d5 Fix Firefox/Zen Browser launch - remove Chromium-only args` (all 2026-03-15).

## Adopt

1. Select the browser binary by explicit path + type env (`BROWSER_TYPE`, `BROWSER_EXECUTABLE_PATH`, config.ts:35, 176-177) - for us: `NLM_BROWSER_PATH` and auto-detect list (Chrome, Edge, Brave, Chromium) as in icebear0828 `browser.ts:25-79`.
2. Per-engine arg sets (Chromium-only flags must not be passed to other engines) (`shared-context-manager.ts:203-215`).
3. Keep the profile directory name engine-neutral or per-engine (switching engine invalidates the profile; docs "Profile conflicts") - we should namespace `profile/<engine>/`.

## Avoid

1. Shipping Firefox/Zen as a "supported" engine without evidence of an end-to-end run; the stealth claims are unsupported by code.
2. Hard-coding `Europe/Brussels` (previously Berlin) as timezone regardless of user (`shared-context-manager.ts:198`): inconsistent with real IP/timezone.
3. Built on v1.2.1: inherits the dead `div.thinking-message` gate and the old host.

## Evidence

Read: `git diff fd0525f HEAD` for `src/auth/auth-manager.ts`, `src/session/shared-context-manager.ts`, `src/config.ts`; `docs/BROWSER_CONFIGURATION.md` (all), `CHANGELOG.md` 1.2.2, `src/utils/page-utils.ts:190-205`; `git log`, `git tag`. REA JSON module map.
