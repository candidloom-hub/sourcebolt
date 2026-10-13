# PleasePrompto/notebooklm-mcp

- URL: https://github.com/PleasePrompto/notebooklm-mcp (ARCHIVED; README marks it unmaintained, commit 0017998 "docs: mark project as unmaintained and archived", 2026-09-10)
- Language / runtime: TypeScript (ESM), Node >= 18, MCP SDK `@modelcontextprotocol/sdk ^1.0.0`, `patchright ^1.48.2` (package.json:51). Transports: stdio (default) and Streamable HTTP (`--transport http`).
- License: MIT, "Copyright (c) 2025 Please Prompto!" (LICENSE:1-3). Code may be quoted in short snippets.
- Stars / last commit: 3437 stars (research/upstreams.md); HEAD `0017998` 2026-09-10 (docs only). Last code commit `50b3e7f` 2026-05-01 (v2.0.0).
- Versions reviewed: tags v1.0.1 ... v1.2.1, v2.0.0; HEAD 0017998. Clone: `research/upstreams/PleasePrompto__notebooklm-mcp`.
- REA: `rea analyze-javascript-application ... --artifact-format directory` run (41 source files, 383 module nodes). The `--format md` rendering prints `[object Object]` for node details, so the JSON form (`--format json --full-output`) was post-processed with a throwaway script (kept outside the repo) to get a module map: per-file exports and external imports. REA confirmed: only external runtime imports are `patchright`, `@modelcontextprotocol/sdk/*`, `env-paths`, `globby`, node builtins; no `fetch`/HTTP client module exists, and the string `batchexecute` appears nowhere in `src/`. Everything else below is from direct reading.

## Access method

100 % browser automation of the web UI. No RPC, no HTTP calls to Google (grep for `batchexecute` and `wrb.fr` in `src/` returns nothing). A real Google Chrome (Playwright channel `chrome`) is driven through Patchright (a patched Playwright that avoids CDP `Runtime.enable` leaks [unverified here; claim from README line 11 "stealth + persistent fingerprint"]). One persistent browser context per server process; one tab (Page) per MCP "session".

Flow for `ask_question`: open/reuse a tab on the notebook URL, find `textarea.query-box-input`, type the question with human-like typing, press Enter, poll the DOM for the newest `.to-user-container .message-text-content` until its text is stable for 3 polls, strip UI noise, optionally click citation chips to harvest excerpts.

## RPC catalog

None. This server never calls `batchexecute` or any other Google endpoint directly. (Evidence: grep across `src/` for `batchexecute`, `wrb.fr`, `SNlM0e`, `f.sid`, `rpcids` = 0 hits.) The only network surface it opens is its own optional Streamable HTTP listener (`src/transport/http.ts:42-183`, see "MCP tools"/"Reliability").

Use as a negative reference: everything the server can do is bounded by what the Angular UI exposes and is exposed to UI drift (see "Selectors").

## Streaming/chat

Endpoint: none; the answer is read from the DOM.

- Before sending, `snapshotPriorAnswers(page)` records the text of every existing `.to-user-container .message-text-content` (`src/notebooklm/chat.ts:212-218`; fallback `snapshotAllResponses` in `src/utils/page-utils.ts` via `.to-user-container` :25). Those texts go into `ignoreTexts` so an old identical answer is not returned (`browser-session.ts:388-393`, `chat.ts:241-269`).
- Send: `humanType` into the input, `randomDelay(500,1000)`, `page.keyboard.press("Enter")`, `randomDelay(1000,1500)` (`browser-session.ts:405-419`).
- Completion detection = text stability, not a "done" marker (`chat.ts:227-304`): poll every 750 ms (`pollIntervalMs` default, :199), read `.to-user-container:last-child .message-text-content` `.innerText({timeout:2000})` (`selectors.ts:34`, `chat.ts:310-321`), sanitize, and return when the same non-placeholder text is seen on `stablePolls`=3 consecutive polls (:203, :288-296). Hard ceiling `ANSWER_TIMEOUT_MS` default 600 000 ms (config.ts:98, chat.ts:197). Max poll count `max(8, ceil(timeout/interval)+4)` (:244). Health check `page.evaluate(()=>true)` with 1.5 s budget every 10th poll (:255-257, `browser/watchdog.ts:43-56`).
- Streaming placeholders are filtered so the detector does not lock onto them: ~55 phrases in 8 locales ("answer is being created", "getting the gist", "antwort wird erstellt", ...) plus any text < 50 chars ending in "..." (`chat.ts:28-104`, `:175-182`). Error replies ("the system could not respond", ...) and daily-limit texts are returned immediately without waiting for stability (`:106-173`, `:282-286`).
- Sanitizer (`chat.ts:329-358`) removes isolated lines that are Material icon ligature names that leak into `innerText` (set in `selectors.ts:382-397`: `more_vert`, `content_copy`, `keep_pin`, ...), plus lone digits/punctuation next to them.
- Known weakness (fixed later by arlinamid): "identical answer" hang. If a new answer equals an earlier one, `ignoreSet.has(candidate)` skips it until timeout (`chat.ts:269`). arlinamid counts answer bubbles instead (see notes/arlinamid__notebooklm-mcp.md).
- Conversation: each MCP session = one tab = one NotebookLM chat; follow-ups continue in the same tab (`session_id` re-use, `session-manager.ts:104-114`). `reset_session` = `page.reload()` which only clears the visible DOM chat (`browser-session.ts:741-760`).
- Citations (`source_format` none|inline|footnotes|json): click each `button.citation-marker` in the last answer, wait up to 1.5 s for `.highlighted` text, take the parent `.paragraph` text as excerpt, press Escape (`src/notebooklm/citations.ts:40-168`). Source name comes from the `aria-label` of `span[aria-label]` inside the button, text after the first ": " (:101-107). `[N]` markers are replaced by string `split/join` (:186), so "[1]" inside "[10]" is not at risk but literal "[1]" text in an answer would be.

## Auth lifecycle

- Cookie source: user logs in once in a visible Chrome opened by `setup_auth` (`auth-manager.ts:908-995`). The persistent profile directory `<dataDir>/chrome_profile` (`config.ts:129`, env-paths `notebooklm-mcp` with suffix "") keeps cookies by itself; additionally `context.storageState({path: state.json})` and a `session.json` dump of `sessionStorage` are written to `<dataDir>/browser_state/` (`auth-manager.ts:54-55, 65-100`). Files are written with default permissions (`fs.writeFile` with only an encoding option, :85-87).
- Login detection: poll `page.url()` every 1 s up to 600 times (10 min) until it starts with `https://notebooklm.google.com/` (`auth-manager.ts:283-345`). Entry URL: `https://accounts.google.com/v3/signin/identifier?continue=...notebooklm.google.com...` (`config.ts:27-28`). This host check silently breaks after Google's rename to notebook.google.com (see Problems).
- Required cookies: an expiry check over the names SID, HSID, SSID, APISID, SAPISID, OSID, __Secure-OSID, __Secure-1PSID, __Secure-3PSID (`auth-manager.ts:37-46`, `validateCookiesExpiry` :205-250). Session cookies (expires -1) are treated as valid (:229). Saved state older than 24 h is treated as expired (`isStateExpired` :255-270) and then ignored on next launch (`getValidStatePath` :131-145). That 24 h rule is a heuristic, not a Google cookie lifetime.
- Optional auto-login with `LOGIN_EMAIL`/`LOGIN_PASSWORD` env (`AUTO_LOGIN_ENABLED`, default false): fills `input#identifierId`/`input[name='identifier']`, `input[name='Passwd']`, `#passwordNext`, handles the account chooser, aborts if URL contains "challenge" (`auth-manager.ts:369-460, 570-870`). Credentials live in `CONFIG` plain strings (`config.ts:52-53`).
- Re-auth: `re_auth` tool = clear all auth data then re-run setup (`handlers.ts:529`); `cleanup_data` deletes profile/library (`:843`). `get_health` reports auth status (`:363`).
- Multi-account: `--account <slug>` / `NOTEBOOKLM_ACCOUNT` re-roots `dataDir`, `browserStateDir`, `chromeProfileDir`, `chromeInstancesDir` under `<dataDir>/accounts/<slug>/` (`src/auth/account-switcher.ts`, slug regex `^[a-z0-9][a-z0-9-_]{0,30}$`). One account per process. [CHANGELOG 2.0.0 "Multi-account support"]
- Profile contention: `NOTEBOOK_PROFILE_STRATEGY` auto|single|isolated. On `ProcessSingleton|SingletonLock|profile is already in use` in `auto` mode the manager launches a per-process copy dir `chrome_profile_instances/instance-<pid>-<ts>` (optionally cloning the base profile, `NOTEBOOK_CLONE_PROFILE`) and prunes old ones by TTL (72 h) and count (20) at startup and shutdown (`shared-context-manager.ts:213-246, 321-422`; `config.ts:139-144`). Cloned/isolated profiles carry cookies only if `storageState` is also passed (:171), which is why state.json is kept.

## Selectors

All UI knowledge is centralised in `src/notebooklm/selectors.ts` (CHANGELOG 2.0.0: "UI changes from Google now require touching exactly one file"). Strategy, stated in the file header (:4-28), is a four-level anchor priority:

1. Angular component CSS classes that are locale-independent: `.to-user-container`, `.message-text-content`, `textarea.query-box-input`, `button.submit-button`, `.add-source-button`, `.single-source-container`, `.cover-subtitle-source-count`, `.create-artifact-button-container`, `.artifact-library-container artifact-library-item` (:32-34, :39, :58, :114-124, :246, :304-309).
2. Material-Symbols icon names as the literal text node of `<mat-icon>`; language-free: `mat-icon:text-is("content_paste")`, `"link"`, `"upload"`, `"audio_magic_eraser"`, `"more_vert"`, `"download"`, `"dock_to_left"` (:151-196, :246, :316, :285).
3. ARIA roles set synchronously by Angular: `[role="dialog"]` as the add-source modal anchor, explicitly chosen over `.mdc-dialog--open` (animation race) and `.cdk-overlay-pane` (matches every menu) (:138-145); `[role="button"]`, `[role="menuitem"]` (:244-263, :329).
4. Locale-bound aria-labels and visible text as last resort for 8 locales EN/DE/FR/ES/PT/IT/NL/JA (`textarea[aria-label*="anfrag" i]`, `button:has-text("Einfügen")`, ...) (:39-50, :202-235).

Each logical control is an array of candidates tried in order or OR-joined with `joinAlt()` (comma = OR in Playwright CSS) (:406-408). Call sites: `locator(joinAlt(sel)).first().click({timeout})`, or loop `for sel of candidates: if await el.isVisible({timeout:1000}) click` (`sources.ts:233-245`, `audio.ts:clickFirstVisible`). The error message on total miss lists all selectors tried and says "NotebookLM Studio UI may have changed" (`audio.ts` end).

How it copes with UI change (and where it did not): the v1.x answer gate waited for `div.thinking-message`; Google removed it and every call timed out (CHANGELOG 2.0.0 "Removed ... Reliance on div.thinking-message", issue #43). v2.0.0 rewrote to the stability detector. The 2.0.0 commit message (50b3e7f) documents further drift: Studio audio entry is now a `<div role="button">`, not a `<button>`; there is no `<audio>` element, only `artifact-library-item`; the add-source modal moved to `[role="dialog"]`. "Last verified: 2026-05 (DE, EN locales)" (`selectors.ts:26-27`) i.e. only two locales were actually tested.

Fallback tricks worth copying:
- Add-source dialog: if the sidebar button click fails, navigate to the same URL with `?addSource=true` (`sources.ts:188-222`).
- Count sources with two anchors, `max(rows, header number)`, because a collapsed sidebar hides rows (`sources.ts:158-180`).
- Detect "pasted text lands in a new Untitled notebook" by comparing the notebook UUID in the URL before and after (`sources.ts:54-102`).
- Wait for the dialog to close before polling for source growth; 90 s poll with `[role="alert"]` scraping (`sources.ts:107-130`).

## Stealth / anti-detection

- Persistent real Chrome profile so the fingerprint (canvas/WebGL/fonts) is constant across restarts. The design rationale is in the class comment: "ONE persistent context = ONE consistent fingerprint = Normal user" (`shared-context-manager.ts:1-36`).
- Launch args `--disable-blink-features=AutomationControlled --disable-dev-shm-usage --no-first-run --no-default-browser-check` (:172-177), `locale:"en-US"`, `timezoneId:"Europe/Berlin"` hard-coded (:162-163), viewport 1920x1080 (`config.ts:103`), headless true by default (`config.ts:96`), real Chrome channel with fallback to bundled Chromium on `Failed to launch.*chrome|executable doesn't exist|code 21|code -?6` (`browser/chromium-fallback.ts:102-123`, used :188-211).
- No `addInitScript` JS patching at all. Everything else relies on Patchright and the real Chrome binary.
- Behavioural stealth (`src/utils/stealth-utils.ts`): Gaussian delays (:54, :72-90), `humanType` at 160-240 WPM with per-char variance and rare typo+backspace (0.3 % in code :153-154; docstring says 2 %, so the docstring is stale), pauses after punctuation (:100-200); `randomMouseMovement` (:209), `realisticClick` (:267), `smoothScroll` (:317), `readingPause` (:361), `simulateReadingPage` (:449). Typing is skipped (`page.fill`) when `CONFIG.stealthEnabled=false` (:123-126). Each of these can be toggled per call through `browser_options.stealth` (ask-question.ts:140-210).
- Limits: no claim of undetectability appears in the PleasePrompto README (grep for "guarantee"/"detect" finds nothing); the tenyoru fork README (line 383) states Google may still detect the automation.

## Browser / page lifecycle, pooling, timeouts, concurrency

- Pool: no real pool. One `BrowserContext` (`SharedContextManager.globalContext`, :39) and a `Map<sessionId, BrowserSession>` of tabs (`session-manager.ts:26`). `maxSessions` default 10 (`config.ts:106`), inactivity timeout 900 s (`:107`), janitor every `clamp(timeout/2, 60, 300)` s with `.unref()` (`session-manager.ts:43-50`). When full, the oldest-created session is evicted (`cleanupOldestSession` :255-284), not the least recently used.
- Context validity check: `await context.cookies()` throws if closed (`shared-context-manager.ts:91-110`); `context.on("close")` nulls the reference (:250-259).
- Headless/visible switch (`show_browser`): if the requested mode differs from the running one, all sessions are closed and the context relaunched (`session-manager.ts:87-100`, `shared-context-manager.ts:473-494`).
- Crash/closed recovery: `isRecoverable` regexes (`Target closed`, `Browser has been closed`, `WebSocket ... closed`, `health check timed out`, ...) (`watchdog.ts:58-78`); `BrowserSession.ask` catches the closed-target message once, nulls the page, `init()`s and retries (`browser-session.ts:456-481`).
- Timeouts: navigation `browserTimeout` 30 s (`config.ts:97`); `waitForSelector` for chat input 10 s then 5 s fallback (`browser-session.ts:180-190`); answer 600 s; audio generation wait 600 s (`audio.ts:62`); source count poll 90 s (`sources.ts:107`); login 10 min; audio download event 60 s (`audio.ts:296`).
- Concurrency: none enforced. There is no per-session lock; two simultaneous `ask_question` calls on the same `session_id` interleave typing in one tab (no mutex in `browser-session.ts`). arlinamid adds `PageLock` (see its note). Different sessions share one browser so they run truly concurrently (and share one Google fingerprint/IP).
- Shutdown: SIGINT/SIGTERM/uncaughtException/unhandledRejection call `requestShutdown` (`index.ts:483-499`) which closes sessions then the context.

## Notebook library / session management

- Library = JSON file `<dataDir>/library.json` (`notebook-library.ts:26`) with `{notebooks[], active_notebook_id, last_modified, version:"1.0.0"}` (`library/types.ts`). Entry: `{id (slug), url, name, description, topics[], content_types[], use_cases[], added_at, last_used, use_count, tags?}` (`types.ts:12-31`). No file locking (`writeFileSync`, :106); two processes can overwrite each other.
- The library is for the *agent*: `ask_question`'s tool description is regenerated from the library (`buildToolDefinitions`, `definitions.ts:18-32`) so the host model sees which notebooks exist and when to use them; `notebooklm://library`, `notebooklm://library/{id}`, `notebooklm://metadata` resources mirror it (`resources/resource-handlers.ts`).
- Resolution order for a call: explicit `notebook_url` > `notebook_id` (looked up, `use_count++`) > active notebook > `NOTEBOOK_URL` env (`handlers.ts:99-120`, `resolveNotebookUrl` :944-958).
- Tool profiles `minimal`(5)/`standard`(10)/`full` filter `tools/list` to save host tokens; settings in `<configDir>/settings.json`, env `NOTEBOOKLM_PROFILE`, `NOTEBOOKLM_DISABLED_TOOLS` (`utils/settings-manager.ts:30-54, 103-140`).

## MCP tools

20 tools in profile "full" (src/tools/definitions/*; the first 16 are the v1.2.x set). `show_browser` (bool) and `browser_options` (object: show, headless, timeout_ms, stealth{enabled, random_delays, human_typing, mouse_movements, typing_wpm_min/max, delay_min_ms/max}, viewport{width,height}) exist on the browser-touching tools.

| Tool | Params | Behavior |
|---|---|---|
| ask_question | question (req), session_id, notebook_id, notebook_url, source_format none/inline/footnotes/json, show_browser, browser_options | Get/create session, type question, wait stable answer, optional citation harvest, wraps answer with AI-generated marker and optional follow-up reminder (`handlers.ts:58-216`; `ask-question.ts:115-250`) |
| add_notebook | url, name, description, topics (req); content_types, use_cases, tags | Add library entry (`notebook-management.ts:11-71`) |
| list_notebooks | - | List library |
| get_notebook | id | One entry |
| select_notebook | id | Set active |
| update_notebook | id; name, description, topics, content_types, use_cases, tags, url | Patch entry |
| remove_notebook | id | Remove entry and close its sessions (`session-manager.ts:186`) |
| search_notebooks | query | Substring search over metadata |
| get_library_stats | - | Counts/usage |
| list_sessions | - | Active sessions with age, message_count |
| close_session | session_id | Close tab |
| reset_session | session_id | Reload tab (clears visible chat only) |
| get_health | - | Auth + sessions + config summary (`handlers.ts:363`) |
| setup_auth | show_browser, browser_options | Open visible Chrome for manual login, 10 min (`system.ts:34`) |
| re_auth | show_browser, browser_options | Clear auth then login (`system.ts:79`) |
| cleanup_data | confirm (req), preserve_library | Delete data dirs (`system.ts:116-150`) |
| add_source | type url/text (req), content (req), title, notebook targeting | UI add-source flow, returns sourceCountBefore/After (`sources.ts:36-92`) |
| generate_audio | custom_prompt, wait_for_completion, timeout_ms | Async by default: returns status started/in_progress/ready (`audio.ts:58-127`) |
| get_audio_status | targeting | ready / in_progress / not_started by DOM (`audio.ts:138-218`) |
| download_audio | destination_dir (req) | Tile 3-dot menu then Download menuitem then `waitForEvent("download")` (`audio.ts:276-317`) |

Server also advertises prompts and progress notifications (`index.ts:163, 216-240`). Streamable HTTP: `POST /mcp`, `GET /healthz`, per-client `Mcp-Session-Id` map, binds `127.0.0.1` by default (`transport/http.ts:8-12, 57`). There is no token, Origin or Host check in `http.ts` (grep for token/origin/auth finds nothing), so a web page could reach a loopback server via DNS rebinding [unverified exploit; absence of checks is verified].

## Reliability

- Rate limit: free tier 50 queries/day assumed; detected by scanning selectors `.error-message, .error-container, [role='alert'], .rate-limit-message, ...` for 10 keywords after each answer (`browser-session.ts:649-700`) and by answer text in 8 languages (`chat.ts:146-173`). Raises `RateLimitError` (`errors.ts`).
- Error mapping: tool handlers return `{success:false, error}`; rate limit gets a special long message (`handlers.ts:194-210`).
- Retries: one retry on closed-target (above); none for selector misses.
- Daily-limit and error texts end the wait early; placeholders reset the stability counter.
- Known quirks recorded in code: pasted-text add_source can redirect to a new "Untitled notebook" (`sources.ts:85-102`); ESC x2 and reload to recover a stuck add-source URL (`browser-session.ts:552-640`).

## Performance notes

- Cold start = Chrome launch (seconds) + goto + `randomDelay(2000,3000)` (`browser-session.ts:115`) + 10 s selector wait. Per ask: 0.5-1 s typing delays + `randomDelay` 1.5-2.5 s + stability needs >= 3 x 750 ms after the last token. Deliberately slow human pacing adds ~5 s per question.
- Connection reuse: one browser for all sessions; tabs persist 15 min idle.
- Memory: one full Chrome (hundreds of MB) regardless of load.
- Poll cost: `innerText` per 750 ms; cheap.

## Tests and quality

No test suite (`"test": "tsx src/index.ts"` in package.json just starts the server). ESLint flat config + Prettier added in 2.0.0. TypeScript strict-ish, DOM lib enabled for `page.evaluate`.

## Versions

From `git tag` and CHANGELOG.md:

| Tag | Date | Notable |
|---|---|---|
| v1.0.1-v1.0.5 | 2025-10-16/17 | Initial server, docs, install guides |
| v1.1.0 | 2025-10-18 | "Major feature update" (notebook library, sessions, stealth) |
| v1.1.1, v1.1.2 | 2025-10-18/19 | bin path fix, Claude Code skill reference |
| v1.2.0 | 2025-11-21 | Tool profiles (minimal/standard/full), LibreChat fix (`prompts`,`logging` capabilities), thinking-message detection |
| v1.2.1 | 2025-12-27 | version bump; completions capability |
| v2.0.0 | 2026-05-01 (CHANGELOG says 2026-04-30) | HTTP transport, add_source, audio tools, citations, `--account`, Chromium fallback, ANSWER_TIMEOUT_MS=600 s, stability-based answer detection, selector registry, AI provenance marker, v1 dropped |
| (no tag) 0017998 | 2026-09-10 | Archived |

`git log --oneline` top: `0017998 docs: mark project as unmaintained and archived`, `50b3e7f feat: v2.0.0 ...`, `fd0525f bump 1.2.1`, `4ad6715 fix: add completions capability`, `4add94b feat: v1.2.0 ...`, `94ec0f6 Release v1.1.2` ... down to `4dbe744 Initial commit` (2025-10-16).

## Adopt

1. Layered selector policy: class > Material-Symbol icon text > ARIA role > locale text, kept in ONE registry file with alternatives per control (`selectors.ts:4-28, 406-408`). Our Go fallback should mirror this as a data table (and ideally an embedded JSON file overridable from disk, so a hotfix needs no rebuild).
2. Stability-based completion + placeholder filter + bubble snapshot instead of a Google-specific "done" element (`chat.ts:227-304`); but combine with the better signals found in other forks (the `.message-actions` marker from paolodalprato, bubble counting from arlinamid).
3. `[role="dialog"]` as the modal anchor, never `.mdc-dialog--open`/`.cdk-overlay-pane` (`selectors.ts:138-145`).
4. Dual-anchor source count and "count before submit, poll after dialog closes" (`sources.ts:158-180, 72-107`).
5. Persistent profile + `launchPersistentContext` + channel fallback chrome -> chromium on known launch-failure regexes (`chromium-fallback.ts:102-138`, `shared-context-manager.ts:188-211`).
6. Profile-lock fallback: isolated per-process profile dir with TTL/count pruning and a safety check that deletion stays inside the instances root (`shared-context-manager.ts:321-422`, esp. :407-412).
7. Zombie-page defense: bounded poll count, periodic 1.5 s liveness probe, Node-side sleep fallback (`watchdog.ts:24-56`, `chat.ts:244-257`).
8. Tool profiles to cut `tools/list` token cost (`settings-manager.ts:30-54`).
9. Library metadata (description/topics/use_cases) injected into the tool description so the host model can route to the right notebook (`definitions.ts:18-32`).
10. Async-by-default long operations (audio): return `started`, poll `get_audio_status` (`audio.ts:58-127`).

## Avoid

1. DOM polling as the primary data path: slow (>= 2.25 s after last token), fragile, and 100 % UI-coupled; `div.thinking-message` removal broke v1.x entirely (CHANGELOG 2.0.0 Removed).
2. No per-session mutex: concurrent calls on one tab corrupt each other (no lock anywhere in `browser-session.ts`).
3. Eviction by oldest *created* session (`session-manager.ts:255-284`), not least-recently-used.
4. Host check `startsWith("https://notebooklm.google.com/")` for login success (`auth-manager.ts:323, 347, 522, 553`); breaks on the notebook.google.com rename (Pantheon and paolodalprato both had to fix this; see their notes).
5. 24 h file-age as the validity rule for saved state (`auth-manager.ts:255-270`) and treating session cookies (expires -1) as valid (:229).
6. Plain-text credentials in env/CONFIG for auto-login (`config.ts:52-53, 204-205`).
7. Cookie/state files written without restrictive permissions (`auth-manager.ts:85-87`).
8. Hard-coded `timezoneId: "Europe/Berlin"` and locale en-US regardless of the user (`shared-context-manager.ts:162-163`): a fingerprint inconsistency with the real IP/timezone.
9. Library JSON written without lock/atomic rename (`notebook-library.ts:106`).
10. HTTP transport with no auth or Origin/Host validation (`transport/http.ts`).
11. Appending an "AI-generated" prefix or follow-up reminder text to answers by default: the reminder tripped prompt-injection guards in host agents (CHANGELOG 2.0.0 "FOLLOW_UP_REMINDER is opt-in").

## Evidence

Read in full or in the cited ranges: `src/notebooklm/selectors.ts`, `chat.ts`, `citations.ts`, `sources.ts`, `audio.ts`; `src/session/{browser-session,session-manager,shared-context-manager}.ts`; `src/auth/{auth-manager,account-switcher}.ts`; `src/browser/{watchdog,chromium-fallback}.ts`; `src/config.ts`; `src/library/*`; `src/tools/definitions/*`; `src/tools/handlers.ts:58-216, 944-1000`; `src/transport/http.ts`; `src/utils/{stealth-utils,settings-manager,page-utils}.ts`; CHANGELOG.md; `git log`, `git tag`; commit message of 50b3e7f. REA module map from `rea analyze-javascript-application` (JSON) used to list exports/imports per file and to confirm the absence of any HTTP client.
