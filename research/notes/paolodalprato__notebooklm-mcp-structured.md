# paolodalprato/notebooklm-mcp-structured

- URL: https://github.com/paolodalprato/notebooklm-mcp-structured (npm `notebooklm-mcp-structured`; ARCHIVED, "final release, development frozen", CHANGELOG 1.1.0)
- Language / runtime: TypeScript (ESM), Node >= 18, MCP SDK 1.30.0 (CHANGELOG 1.1.0), Patchright, real Chrome (`channel:"chrome"` forced, `session/shared-context-manager.ts:157`).
- License: MIT, "Copyright (c) 2025 Please Prompto! (original notebooklm-mcp)" (LICENSE:1-3). Snippets allowed.
- Stars / last commit: 42 stars; HEAD `9ffadbd` 2026-08-20. Package 1.1.0.
- Versions reviewed: tags v1.0.0 (2025-11-28), v1.0.1 (2025-11-27); untagged 1.1.0 commits 2026-08-13..08-20 (`aa9fbf6 Freeze development: v1.1.0 is the final MCP-v1 release`). Clone: `research/upstreams/paolodalprato__notebooklm-mcp-structured`.
- REA: `rea analyze-javascript-application ... --artifact-format directory` (414 module nodes, 1 `electron-main` classification false positive). Module map shows what is *added* to the parent: `tools/templates/structuring-guidelines.ts`, `selectors.ts` (flat constants), `server-core.ts` (transport-independent server factory), `singleton/{backend,proxy,registry,spawn}.ts` (+ `*.test.ts` run by `node:test`), `utils/connection-checker.ts` (imports `child_process`), `errors.ts` (`isPageClosedError`). Imports are patchright, MCP SDK and node builtins only; no HTTP client.

## What "structured" adds

Two unrelated things, in this order of history:

1. **Client-side prompt structuring (v1.0.0)** - the entire "structured" feature is ~120 lines of text embedded in the `ask_question` tool *description*, telling the host model (Claude) how to rewrite the user's question before calling the tool: five question-type patterns (comparison, list/identification, analysis, explanation, extraction) each ending with "organize by thematic topics; TOPIC / DESCRIPTION / EVIDENCE `"quote" [Source: doc]`; if information is not found write `[NOT FOUND IN DOCUMENTS]`", language adaptation (translate the template into the user's language), "never use decorative lines (===, ---) as they cause NotebookLM timeouts", and "present the answer faithfully with no external knowledge" (`src/tools/templates/structuring-guidelines.ts:20-125`, CUSTOM_MODIFICATIONS.md "Key Modification"). There is **no server-side source-fidelity check**: the server passes the question through unchanged and never verifies citations; fidelity is only requested via prompt text. research/upstreams.md describes it as "source-fidelity checks"; the code contradicts that reading (the guideline text asks for it, nothing enforces it). The prompt-structuring text is the only unique content-level idea; version "v3" of the guidelines in 1.1.0 (CHANGELOG).
2. **Operational hardening for the Gemini Notebook rename and Claude Desktop's multi-surface use (v1.1.0, 2026-08)**: new host, new answer-completion marker, singleton backend, profile fallback fixes (below).

## Access method

Browser automation only (Patchright + Chrome), same shape as PleasePrompto v1.x with an updated extractor. No RPC (`batchexecute` grep = 0 in `src/`).

## RPC catalog

None.

## Streaming/chat

DOM polling at 1 s with a real **completion signal**, the one clean idea in the six repos:

- Completion marker: the action-button bar `.message-actions` (copy/rate) "appear in the response container at the exact moment generation ends, while the answer is streaming they are absent" (measured 2026-08-13, comment `src/selectors.ts:58-67`, constant `RESPONSE_COMPLETE_MARKER` :67). The author notes the send button also flips from "stop generating" to "send" at that instant but its label is translated so it cannot be matched (:62-65).
- Reasoning ("Thoughts") block: `thinking-chain-view` lives inside the response container as a sibling of answer blocks; while generating it is the only content, and after generation it stays; the extractor takes the text of all children except that element (`selectors.ts:46-56`, `utils/page-utils.ts:79-128`).
- Selection logic (`page-utils.ts:353-401`): iterate all `.to-user-container` answers; skip those whose 32-bit string hash is in `knownHashes` (pre-question snapshot, :240-245, 376-377); if the answer has no `.message-actions` **and** some marker exists elsewhere on the page (`markerSeen`) it is "pending" and skipped (:381-384). Falls back to a list of 13 generic selectors when `.to-user-container` is missing (:416-...). Then the usual 3 consecutive identical polls (`CONFIG.requiredStablePolls` = 3, config.ts:194; loop :258-330). Subtlety: for the very first answer in a notebook no marker exists yet (`markerSeen` false) so the marker is not required and stability alone decides.
- Timeouts: `responseTimeoutMs` 300 s (config.ts:193: "2 minutes was not enough for a 7.700-character answer"), poll 1 s (browser-session.ts:462), progress heartbeat every 15 s mapped to a creeping progress value `3 + ratio*0.9` of 5 (page-utils.ts:206-231, 262-273; browser-session.ts:465-473) so MCP clients with progress-reset timeouts do not abort.
- Pre/post submit delays 0.5-1.0 s / 1.0-1.5 s (config.ts:187-190).

## Auth lifecycle

- Persistent profile + `storageState` as parent. New host: `NOTEBOOK_HOST = "notebook.google.com"` ("Google renamed NotebookLM to Gemini Notebook in July 2026", `config.ts:24-28`); login URL `accounts.google.com/v3/signin/identifier?continue=https%3A%2F%2Fnotebook.google.com%2F...` (:47); `isNotebookUrl()` helper (:~30-40). Commit 3228e73 (2026-08-13) "Follow Google's rename: ... new host, new notebook URLs, redesigned answer UI, redirect chain handled".
- **Redirect-chain wait**: after `goto`, if the URL is not a notebook URL, `page.waitForURL(isNotebookUrl, {timeout: NOTEBOOK_URL_TIMEOUT_MS=15000})` before touching any selector, because "waiting for the selector alone measures the wrong thing and blames the interface" (`session/browser-session.ts:160-200`, config.ts:184).
- **Cookie injection does not authenticate a fresh profile**: "a Chrome profile without its own history cannot complete Google's sign-in from injected cookies, it needs a real login with two-step verification"; staying on accounts.google.com is terminal, not slow (browser-session.ts:165-170, error text :178-190). This directly contradicts the assumption behind PleasePrompto's `isolated` profile fallback + `storageState` seeding (shared-context-manager.ts of the parent) and means that when the main profile is locked, the fallback profile is usually unauthenticated (shared-context-manager.ts:211-216 comment: "the retry usually CANNOT authenticate"). Takeaway for our design: device-bound cookies (DBSC-like / profile history) are not portable; never plan on replaying exported cookies into a new browser profile without validating.
- "Fail at once when the profile cannot sign itself in" (commit 29ddeab) and "Release the browser when a session fails to start too" (123f62b), "Release the shared browser when no session is left" (ea20bab) to stop orphan Chrome processes.
- "Never delete the data directory when preserve_library is set" (37273d2): fix for `cleanup_data` data-loss bug.
- Chromium sandbox: `chromiumSandbox: true` explicitly because "Patchright adds --no-sandbox unless" told otherwise (`shared-context-manager.ts:157-160`; CHANGELOG 1.1.0 "Chromium sandbox enabled").

## Selectors

Flat exported constants in `src/selectors.ts:1-143` ("Last verified: 2026-08 (Gemini Notebook UI)"): `CHAT_INPUT_PRIMARY = "textarea.query-box-input"`, fallback `textarea[aria-label="Enter a query"]` (English only), `RESPONSE_CONTAINER_PRIMARY = ".to-user-container"`, `RESPONSE_TEXT_CONTENT = ".message-text-content"`, `THOUGHTS_ELEMENT = "thinking-chain-view"`, `RESPONSE_COMPLETE_MARKER = ".message-actions"`, 13 generic fallback selectors (`[data-message-author='bot']`, `[aria-live='polite']`, ...) kept from the original, `ERROR_SELECTORS` list and `RATE_LIMIT_KEYWORDS` (English). No multilingual strategy (contrast PleasePrompto v2 / arlinamid). UI-change coping: each constant carries a measured-on date comment; extraction code does a primary container pass then a generic pass; no discovery tooling.

## Stealth / anti-detection

Unchanged from the parent: persistent profile, four Chromium args (`--disable-blink-features=AutomationControlled`, `--disable-dev-shm-usage`, `--no-first-run`, `--no-default-browser-check`, shared-context-manager.ts:171-176), `humanType`, `utils/stealth-utils.ts` (same exports), configurable delays.

## Browser / page lifecycle, pooling, timeouts, concurrency

- Same shared context + sessions Map. New: **singleton backend** so Claude Desktop's two surfaces (Chat and Cowork, each spawning its own stdio server on the same profile) share one browser: the stdio entry becomes a transparent JSON-RPC proxy to a backend that runs Streamable HTTP on `127.0.0.1:<random port>` with a random 32-byte bearer token (`singleton/backend.ts:32-41, 79-91, 138-147`, timing-safe compare :36), registry files `singleton.json` (port, token, pid, version, startedAt) and `singleton.lock` written atomically via temp+rename (`singleton/registry.ts:10-22, 55-60`), spawn lock decides who launches the backend, others wait; `/health` returns version, **version skew -> ask the old backend to `/shutdown` and replace** (`singleton/spawn.ts:33-90`, `decideOnHealth` :50-54 = connect/replace/unreachable); backend exits after `NOTEBOOK_BACKEND_GRACE_MS` with no client (backend.ts:44, 67-68).
- Proxy robustness (`singleton/proxy.ts`): 30 s heartbeat ping with internal ids, 3 reconnect attempts at 1 s, **replays the stored `initialize` + `initialized` handshake on a fresh backend session** with a 10 s timeout (:15-21, REPLAY_ID), serializes forwarding so message order is kept (commit 1244957), synthesizes JSON-RPC errors only for ids truly in flight (:60-80), lets the event loop drain before exit (b74621e).
- Browser release: browser closed when the last session ends (commit ea20bab) rather than kept warm, trading latency for no orphan Chrome.
- Concurrency inside a tab: still none (no lock). With the singleton one process owns all tabs, so a lock could be added; it was not.
- Timeouts: response 300 s, URL reach 15 s, chat-input wait 10 s (`chatInputFallbackTimeoutMs` config.ts:186), session timeout/maxSessions as parent.

## Notebook library / session management

Library JSON unchanged (`src/library/*`, 15 tools, no add_source/audio). Tool list: ask_question, add_notebook, list_notebooks, get_notebook, select_notebook, update_notebook, remove_notebook, search_notebooks, get_library_stats, list_sessions, close_session, reset_session, get_health, setup_auth, re_auth, cleanup_data (`tools/definitions/*`). Server declares only implemented capabilities: `tools, resources, prompts`; `logging` removed as unused/deprecated (`server-core.ts:88-98`, commit d90781f); `prompts/list` handler added because Claude Desktop called it and got -32601 (:116-118).

## MCP tools (16)

| Tool | Params | Behavior |
|---|---|---|
| ask_question | question (req), session_id, notebook_id, notebook_url, show_browser, browser_options (incl. response_timeout_ms, config.ts:354) | Pass-through to NotebookLM; description carries the structuring guidelines |
| add_notebook / list_notebooks / get_notebook / select_notebook / update_notebook / remove_notebook / search_notebooks / get_library_stats | as parent | library CRUD |
| list_sessions / close_session / reset_session | session_id | tab management |
| get_health / setup_auth / re_auth / cleanup_data | confirm, preserve_library | auth + data |

## Reliability

`isPageClosedError` + `PAGE_CLOSED_PATTERN` classify closed-target errors (`errors.ts`); retry-on-closed as parent; "Do not state the fallback cause as the only one" (b9b4197) = error messages list all plausible causes; unit tests for registry and spawn decision (`singleton/registry.test.ts`, `spawn.test.ts`, run with `node:test`), integration test `npm run test:singleton` (two-surface scenario).

## Performance notes

Singleton avoids launching N Chromes. Marker-based completion lets the stability counter start only after generation ends, but the 3 x 1 s stable-poll rule still adds ~3 s after the marker appears (the marker is used to skip pending answers, not to return immediately) - a cheap improvement would be returning as soon as `.message-actions` is present plus one confirmation poll.

## Tests and quality

Only the singleton module has tests (3 `node:test` files). TypeScript strict. Process documents (design spec and plan for the singleton) existed in git history but were removed from the published tree (9ffadbd).

## Versions

| Tag / commit | Date | Notable |
|---|---|---|
| v1.0.1 / v1.0.0 | 2025-11-27/28 | fork of notebooklm-mcp (base version [unverified], README says v1.2-era), client-side prompt structuring (note v1.0.1 predates v1.0.0 in dates) |
| 3228e73..1169303 | 2026-08-13 | follow rename to Gemini Notebook, capture answer not reasoning, configurable 5 min wait, heartbeat, completion marker |
| d90781f..37273d2 | 2026-08-19/20 | SDK 1.30.0, declare only implemented capabilities, isolated-profile fallback whenever browser will not start, redirect-chain wait, release browser |
| 85d8cb6..9ffadbd (1.1.0) | 2026-08-20 | singleton backend/proxy, docs, freeze |

## Adopt

1. `.message-actions` as completion signal and `thinking-chain-view` stripping (`selectors.ts:46-67`, `page-utils.ts:79-128`) for our browser fallback; confirm with stability as secondary.
2. Wait for the post-redirect host before judging the UI (`browser-session.ts:160-200`); treat "stuck on accounts.google.com" as an auth failure with an actionable message, not a timeout.
3. Heartbeat progress notifications (15 s) during long waits (`page-utils.ts:262-273`); the Go SDK supports progress tokens.
4. Singleton/leader design with random loopback port + bearer token + atomic registry + spawn lock + version-skew replacement + handshake replay (`singleton/*`). In Go, the simplest equivalent is a named-pipe/unix-socket owner process; reuse the version-skew idea.
5. Declare only capabilities that are implemented; add handlers for methods hosts call (`prompts/list`) (`server-core.ts:88-118`).
6. Explicit sandbox-on for the browser (`chromiumSandbox:true`, `shared-context-manager.ts:157-160`); with chromedp/rod avoid `--no-sandbox` unless in containers.
7. Prompt-structuring template as optional MCP *prompt* or tool-description addendum for fidelity-critical users; keep it out of the server logic.

## Avoid

1. Assuming exported cookies can authenticate a fresh browser profile (they cannot, per this repo's own measurement).
2. Fallback to an isolated profile as an auth strategy; it is only a "keep the process alive" tactic.
3. Tool description of ~120 lines inflates every `tools/list` response (tokens) for every host; use a dedicated prompt/resource instead.
4. English-only fallback selectors (`Enter a query`).
5. Treating prompt text as "source fidelity checks" - no enforcement exists.
6. Closing the whole browser whenever the last session ends: cold start cost on the next call.
7. The 13-selector generic fallback (`[aria-live='polite']` etc.) can match unrelated live regions; keep fallbacks narrow.

## Evidence

Read: `src/selectors.ts` (all), `src/utils/page-utils.ts:1-420`, `src/session/browser-session.ts:100-200, 440-480`, `src/session/shared-context-manager.ts:140-235`, `src/singleton/{registry,backend,spawn,proxy}.ts`, `src/server-core.ts:80-130`, `src/config.ts:80-200`, `src/errors.ts`, `src/tools/templates/structuring-guidelines.ts`, CHANGELOG.md, CUSTOM_MODIFICATIONS.md; `git log`, `git tag`. REA JSON module map.
