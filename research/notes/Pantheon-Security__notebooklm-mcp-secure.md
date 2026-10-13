# Pantheon-Security/notebooklm-mcp-secure

- URL: https://github.com/Pantheon-Security/notebooklm-mcp-secure (npm `@pan-sec/notebooklm-mcp`; MCP registry id io.github.Pantheon-Security/notebooklm-mcp-secure)
- Language / runtime: TypeScript (ESM), Node >= 20 (Dockerfile `node:20-slim`), MCP SDK, Patchright (real Chrome), `@noble/post-quantum` (crypto.ts:27), official Gemini API client (`gemini/gemini-client.ts`). Tests: vitest, 70 files / 791 tests (CHANGELOG 2026.5.0), Stryker mutation config present.
- License: MIT, "Copyright (c) 2025 Please Prompto!" (LICENSE:1-3). Short snippets allowed.
- Stars / last commit: 85 stars; HEAD `100000b` 2026-09-03 (CI: publish server.json to MCP registry). Package version 2026.5.0.
- Versions reviewed: 55 tags v1.0.1 (2025-10-16) ... v1.10.9 (2026-01-03), v2026.1.0 (2026-01-04) ... v2026.5.0 (2026-09-03); HEAD 100000b. It shares history with PleasePrompto up to ~v1.2.0 (same first tags and dates) then diverges; it does **not** include PleasePrompto v2.0.0 (so no stability-based answer detector).
- Clone: `research/upstreams/Pantheon-Security__notebooklm-mcp-secure`.
- REA: `rea analyze-javascript-application ... --artifact-format directory` (842 module nodes, 184 assets, 298 `unknown`, 1 `endpoint`). Module map shows the surface added over the parent: `compliance/*` (21 files), `auth/mcp-auth.ts`, `utils/{crypto,secure-memory,secrets-scanner,response-validator,security,schema-validator,file-lock,file-permissions,audit-logger}.ts`, `gemini/*`, `notebook-creation/*` (UI automation to create notebooks/sources/audio/video/data-tables), `webhooks/*`, `quota/*`, `logging/query-logger.ts`, `observability/metrics.ts`. External imports confirm: `https`, `dgram` (SIEM syslog UDP) in compliance, `node:dns/promises` in webhooks, `crypto`. No batchexecute anywhere.

## Access method

Browser automation of the web UI (Patchright + real Chrome, persistent profile, same shape as PleasePrompto v1.x) for NotebookLM; **plus** a separate non-NotebookLM path through the official Gemini API for `deep_research`, `gemini_query`, and document upload/query tools (`tools/definitions/gemini.ts`, `gemini/gemini-client.ts`: `client.interactions.create/get/delete` at :416-588, agent `deep-research-pro-preview-12-2025` at `gemini/types.ts:24`, models `gemini-3-flash-preview`/`gemini-3-pro-preview` :18-19). README calls the combination "Hybrid Architecture" (README:246). The Gemini path is an API-key product (GEMINI_API_KEY), not NotebookLM.

Answer path: `session.ask` -> `snapshotAllResponses` -> type -> `waitForLatestAnswer` (`session/browser-session.ts:385-445`; `utils/page-utils.ts:141-250`). It still gates on `div.thinking-message` visibility (`page-utils.ts:198-210`) plus 3-poll text stability at 1 s (`:190-193`), default timeout 120 s (`:165`, `CONFIG.responseTimeout`). That `div.thinking-message` element is the exact selector Google removed and that broke PleasePrompto v1.x (PleasePrompto CHANGELOG 2.0.0); so on today's UI the gate is a no-op and the stability poll is the only thing left. Do not copy this detector.

## RPC catalog

None (grep `batchexecute|wrb.fr` over `src/` = 0). REA's single `endpoint` node is the Gemini API/webhook layer, not NotebookLM.

## Streaming/chat

DOM polling as above. After the answer is read it is passed through the response validator (`tools/handlers/ask-question.ts:270-285`); blocked patterns are replaced inline with `[REDACTED: ...]` and `security_warnings` is added to the result. Default behaviour then appends a "follow-up reminder" to every answer: `followUpEnabled: true` (`config.ts:184`) and text starting "EXTREMELY IMPORTANT: Is that ALL you need to know? ..." (`config.ts:24-25`). This is the same instruction-like text that PleasePrompto made opt-in because host agents' injection guards flagged it (PleasePrompto CHANGELOG 2.0.0, issue #28). A security fork that ships an imperative suffix on tool output by default is self-contradictory.

## Auth lifecycle

- Browser login as parent (visible Chrome, `setup_auth`); state written through `SecureStorage` (encrypted, below) instead of plain JSON: `auth-manager.ts:29, 81, 133, 141, 171, 402` and `session/shared-context-manager.ts:23, 160-175` (decrypts and passes `storageState` object at launch, :195-201). `auth-capture.mjs` captures an authenticated session from the existing Chrome profile and "refuses to write a plaintext fallback" (CHANGELOG 2026.5.0).
- Auto-login credentials: `LOGIN_PASSWORD` is moved into a `SecureCredential` with 30 min TTL and `delete process.env.LOGIN_PASSWORD` (`config.ts:26-31, 223-262`; `utils/secure-memory.ts:115-175`). Same for `GEMINI_API_KEY`.
- Host-change bug fixed late: five auth-detection checks used `startsWith("https://notebooklm.google.com/")`; after Google's 301 to `notebook.google.com` a completed login "went undetected" (CHANGELOG 2026.5.0 FX-026). Replacement `isNotebookLMUrl()` accepts hosts matching `^notebook(?:lm)?\.google\.(tld)` plus an allowlist (`utils/security.ts:30-52, 286-300`).
- MCP-level auth (separate from Google auth): see Security measures #3.
- Multi-session: v2026.1.2 "multi-session support"; per-instance profile strategy same as parent; file locks (`utils/file-lock.ts`) around state saves (`auth-manager.ts` `withLock(this.stateFilePath)`).

## Security measures (the point of this fork) - file:line

Every item below was read in code. "Gap" lines state what the measure does NOT cover, verified or noted [unverified].

### 1. Secret storage at rest
- Hybrid encryption of cookies/state/tokens/keys: ML-KEM-768 (`@noble/post-quantum`, `utils/crypto.ts:27, 180-182`) encapsulates a key, HKDF-SHA256 derives a ChaCha20-Poly1305 key (`crypto.ts:116, 197-244, 209`), key zeroed after use (`:229, 282`). Classical fallback `encryptClassical/decryptClassical` (`:290-318`). `SecureStorage` class (`:411`) writes `<file>.pqenc` / `.enc` (`:634, 679, 734`), PQ key pair stored encrypted as `pq-keys.enc` (`:430`). Defaults on: `NLMCP_ENCRYPTION_ENABLED !== "false"`, post-quantum on, machine key on (`:132-142`); PBKDF2-SHA256 600 000 iterations (`:123, 152-157`).
- Classical key source priority: env `NLMCP_ENCRYPTION_KEY` > key file `NLMCP_ENCRYPTION_KEY_FILE` > persisted random `machine.key` in the config dir written 0600 (`:160-174, 470-511`). The default (machine key) lives on the same disk as the data; the SECURITY.md itself says it protects against "offline theft of individual encrypted files", not against a local attacker (SECURITY.md "What this does and does not protect against").
- Owner-only permissions: `PERMISSION_MODES` 0600/0700 (`utils/file-permissions.ts:57-62`), `chmod` on Unix (:83), `icacls` current-user-only on Windows (:104-230, header :10-11), `writeFileSecure`/`mkdirSecure`/`appendFileSecure` (:236-300).
- Credential isolation: SecureCredential TTL 30 min (`config.ts:26, 229-237`; `secure-memory.ts:115-175`), env vars deleted after read (`config.ts:230, 236`), `SecureString` with wipe (`secure-memory.ts:54-107`), `zeroBuffer` (:29), `secureCompare` timing-safe (:209).
- Gap: the live Chrome profile directory (`chrome_profile/`, holds the real Cookies DB) is created by Chrome in the clear (OS-level protection only); encryption covers only `state.json`/`session.json` copies (`shared-context-manager.ts:195-201` still launches a persistent context on that profile) [profile contents unverified]. Query logs "remain plaintext on disk (mode 0600)" (CHANGELOG 2026.5.0 Known limitations).

### 2. Input validation
- URL allow-listing: only https, rejects `javascript: data: vbscript: file: about:` (`utils/security.ts:93-100`), notebook host must match `NOTEBOOK_HOST_PATTERN` or the allowlist of notebook(lm).google.* domains (:30-52, 147-182); source URLs additionally reject private/loopback/link-local hosts via `isInternalHost` (127/8, 10/8, 192.168/16, 169.254/16, 172.16/12, localhost, ::1, 0.0.0.0) (:122-140, 184-205). Note: no DNS resolution for source URLs (only the webhook dispatcher resolves, below), so a public name resolving to a private IP passes [verified by absence in security.ts].
- IDs: `validateNotebookId` (:207), `validateSessionId` (:231); questions: non-empty, <= 32 000 chars (:255-272). Applied at `session-manager.ts:101`, `tools/handlers/ask-question.ts:102-114`, `audio-video.ts:41, 82`, `resources/resource-handlers.ts:306-308`.
- Schema validation at dispatch: every tool's declared `inputSchema` is now enforced (`utils/schema-validator.ts:239`, called at `index.ts:466-483`); BREAKING in 2026.5.0 because before that "every schema was decorative" (CHANGELOG FX-015). Validator **fails open** on any keyword it does not understand (schema-validator.ts:231-238).
- File paths: `resolveWithinAllowlist` resolves symlinks then requires the path under `$HOME` or `NLMCP_FOLDER_ALLOWLIST` (colon-separated), and denies sensitive segments (`DENIED_PATH_SEGMENTS`, :389) and system dirs `/etc /root /proc /sys /var/log` (:404) (`security.ts:417-480`); used for `upload_document` (`handlers/gemini.ts:340`) and notebook-creation sources (`handlers/notebook-creation.ts:69, 439`); `resolveWithinBase` for output files (`gemini.ts:722`). Gap: denylist is POSIX-only (Windows system paths not listed).
- Tool bounds: array/length bounds added to query/chat/research/batch tools (commit 6154b73 "schema bounds").

### 3. MCP-level authentication and authorization
- Secure-by-default token auth (`auth/mcp-auth.ts:65-100`): enabled unless `NLMCP_AUTH_DISABLED=true`; token auto-generated on first run, only a salted hash is persisted (`auth-token.hash` + `auth-salt`, :125-137, 175-205); env tokens are hashed then removed from `process.env` (:151-172); optional read-only token (:35, 158-172); lockout after 5 failures with exponential backoff 5 min -> 15 -> 45 min -> 4 h cap (:39-58); optional rotation interval (:207-226).
- Where it is enforced: tool-call handler reads `_meta.authToken` or env `NLMCP_AUTH_TOKEN` and checks admin or read scope (`index.ts:410-427`, `authenticateMCPRequest` `mcp-auth.ts:565`). Admin-only set `TOOLS_REQUIRING_AUTH`: add_folder, cleanup_data, export_library, setup_auth, re_auth, webhook config/remove/test, delete_document, upload_document, download_audio, privacy/compliance operations (`index.ts:149-171`); all research/notebook/session tools are in `TOOLS_EXEMPT_FROM_AUTH` (:136-147), i.e. read scope. Gap: over stdio the token reaches the server via `_meta` or env set by the same client config, so it authenticates the *client config*, not an end user; it adds little against a compromised MCP client.
- Per-session request throttle: `RateLimiter(100 req / 60 s)` keyed by session (`security.ts:506-575`; `handlers/index.ts:~101-104`; check at `handlers/ask-question.ts:120-136`). Plus a daily Google-quota manager that refuses queries near the 50/day free limit (`quota/quota-manager.ts:40-56`, used in ask-question.ts).
- Session timeouts: hard lifetime and inactivity limits (`session/session-timeout.ts:23-65`, check every 5 s :16).

### 4. Prompt-injection / hostile-output defenses (response validation)
- `ResponseValidator` runs on every NotebookLM answer (`ask-question.ts:270`), Gemini answers (`handlers/gemini.ts:108, 222, 422, 583, 872`), chat extraction and data tables (`data-table-manager.ts:460`, `page-utils.ts:50`). Source: `utils/response-validator.ts`.
- Normalisation before matching: NFKC + strip zero-width/bidi controls (`INVISIBLE_CHARS` :70, `normalizeForDetection` :79).
- 15 regex patterns with severity (:87-172): instruction override ("ignore all previous instructions"), disregard, memory reset, "you are now ... mode", "new instructions:", "system: ...", Llama `[INST]`, ChatML `<|im_start|>`, `### system:`, identity manipulation, jailbreak, safety bypass, rule violation, "human:"/"assistant:" turn injection. critical/high -> replaced by `[REDACTED: prompt injection detected]`; medium -> warning only (:236-249). `detectPromptInjection` reports every occurrence (:331-361).
- Suspicious URLs (shorteners, paste/file-share sites, javascript:/data:/file:, raw IPs, webhooks, Discord webhooks) redacted (:173-196, 252-266); optional allow-domain list `NLMCP_ALLOWED_DOMAINS`.
- Encoded payloads (long base64 with entropy >= 4.0, hex, heavy %-encoding, \u escapes) warned, blocked only if `NLMCP_BLOCK_ENCODED_PAYLOADS=true` (:197-209, 268-282).
- Toggles: `NLMCP_RESPONSE_VALIDATION`, `_BLOCK_PROMPT_INJECTION`, `_BLOCK_SUSPICIOUS_URLS`, `_WARN_SUSPICIOUS` (:30-39).
- Gaps (documented by authors): English-only pattern list; can over-redact notebooks about prompt injection (CHANGELOG 2026.5.0 Known limitations). Returned `sanitized` text is the NFKC-normalised string, so non-matching answers are altered only when something was blocked (answer returned raw otherwise, ask-question.ts:274-283). Regex lists are bypassable by paraphrase, other languages, markdown tricks [expected; unverified by test].
- Untrusted-output labelling is NOT done here (compare arlinamid's AI marker idea in PleasePrompto v2 `utils/disclaimer.ts`).

### 5. Logging and redaction
- Logger writes to stderr only so stdout stays clean for JSON-RPC; optional JSON format `NLMCP_LOG_FORMAT=json` with AsyncLocalStorage correlation ids (`utils/logger.ts:30-66`, `index.ts:~405 correlation_id`). The logger itself does **not** redact; redaction is applied at call sites by `sanitizeForLogging` (`security.ts:304-327`: emails -> masked via `maskEmail` :352, `password/token/api_key/auth =` values -> `[REDACTED]`, `scheme://user:pass@` -> `***:***@`) e.g. `ask-question.ts:98`, `gemini.ts:62, 111, 158, 232, 348, 416`.
- Error text to clients: `sanitizeErrorMessage` strips absolute paths and stack frames (`security.ts:342-348`), applied in `index.ts` import :60 and compliance tool errors (FX-022).
- Audit log: JSONL, hash-chained SHA-256 (`utils/audit-logger.ts:272-285`, `previousHash` starts at "GENESIS" :99), chain continues across days (:195-203), corrupted chain is quarantined rather than restarted (:184-190), serialized critical section so concurrent events cannot fork the chain (:361-374, 422), tamper anchor sidecar with optional HMAC key `NLMCP_AUDIT_CHECKPOINT_KEY` (:100-110, 728-731); details sanitized before write (:324, 454, 548, 601). Authors state the honest limit: a local attacker with broad write access defeats the sidecar unless the HMAC key is kept elsewhere (:100-110).
- Secrets scanner: 31 patterns (AWS, GitHub, Slack, Stripe, OpenAI, Anthropic, private keys, JWT, DB URLs, generic API key, high-entropy) with Shannon entropy (`utils/secrets-scanner.ts:57, 369-465`). Where used: **only** `logging/query-logger.ts:123, 301-302` (redacts question/answer in the query log) and `webhooks/webhook-dispatcher.ts:467` (payloads). It is NOT applied to tool outputs returned to the MCP client or to the audit log [verified by grep].
- Query log: plaintext 0600 (known limitation).

### 6. Sandboxing / process isolation
- Browser flags add `--disable-extensions` and `--site-per-process` to the parent's four (`session/shared-context-manager.ts:203-210`); no `--no-sandbox` anywhere in src (grep). No OS-level sandbox, seccomp, container or separate user for the browser; `Dockerfile` (smithery) is multi-stage, installs Chromium libs, and runs as default user [Dockerfile:1-40; user not verified past line 40].
- Webhook SSRF defence: hostname checked against private ranges, DNS resolved with a 2 s timeout and every returned A/AAAA checked, re-validated per send attempt (anti DNS-rebinding TOCTOU, FX-017); HMAC-SHA256 signature including unix timestamp to prevent replay (`webhooks/webhook-dispatcher.ts:146-168, 445-517, 805`); delivery log persisted (`webhooks.jsonl` change I279).
- Per-page mutex: README/SECURITY claim "Browser page operations serialised per-page". Reality: commit e36b246 only serialises `NotebookCreator.createNotebook` with a per-instance promise queue (`notebook-creation/notebook-creator.ts`); `BrowserSession.ask`/`reset` have no lock (no mutex/queue/lock identifier in `session/browser-session.ts`). Treat the claim as overstated.
- File locks: `FileLock` with stale threshold 15 min (`utils/file-lock.ts:59-110, 261`) around state/quota/audit writes.
- Destructive cleanup guard: `cleanup_data` no longer deletes `optional` categories (incl. `~/.claude/projects/*notebooklm-mcp*`) unless `include_optional` (FX-018).

### 7. Compliance suite (not security per se)
`src/compliance/*`: consent manager, DSAR handler, data erasure/export/inventory/classification, retention engine, incident manager, breach detection, alert manager (https webhooks), SIEM exporter (syslog UDP / https), evidence collector, policy docs, privacy notice, dashboard, report generator. Exposed as MCP tools (`compliance_dashboard`, `submit_dsar`, `export_user_data`, `request_data_erasure`, `grant_consent`, `report_security_incident`, `verify_audit_log_integrity`, ...). Large attack/maintenance surface for a local single-user tool [judgement].

## Selectors

Parent selectors plus `src/notebook-creation/selectors.ts` (264 lines): objects of `{primary, fallbacks[], confirmed, date}` per control, e.g. `newNotebookButton: {primary:'button[aria-label="Create new notebook"]', fallbacks:[...], confirmed:true // December 2025}` (:36-45); `RESPONSE_SELECTORS` fallback list of 13 (including `[aria-live='polite']`) (:15-29); an automated discovery script `scripts/selector-discovery.ts` + `scripts/run-discovery.ts` that walks the UI and prints candidates (header :1-10); deadline-based selector waiting (commit d507b29 I145). Coping with UI change = discovery tool + fallbacks + "confirmed" flag, no multilingual strategy (English labels), so weaker than PleasePrompto v2 on locale.

## Stealth / anti-detection

Inherited: persistent profile, `--disable-blink-features=AutomationControlled`, `humanType`, delays, Europe/Berlin timezone (`shared-context-manager.ts:195`), `utils/stealth-utils.ts` (506 lines). Nothing new.

## Browser / page lifecycle, pooling, timeouts, concurrency

Shared context + Map of sessions as parent; `session-timeout.ts` with `maxLifetimeMs` + `inactivityTimeoutMs`, warnings; `profileStrategy` auto/single/isolated; response timeout `CONFIG.responseTimeout` (`NLMCP_RESPONSE_TIMEOUT_MS`, config.ts:293); quota tracking persisted; webhooks and events (`events/event-emitter.ts`). No per-session lock for ask (see above).

## Notebook library / session management

Library JSON as parent plus `sync_library` (verify library entries against the account's notebooks, `auto_fix`), `export_library`, `add_folder` (upload a folder as sources, recursive, dry_run), `batch_create_notebooks`, `create_notebook` from sources (url/text/file/youtube/drive?) via UI automation, `manage sources` (`list_sources/add_source/remove_source`), `get_query_history` (query log search), `get_notebook_chat_history` (extract chat from browser), quota tiers (`get_quota`, `set_quota_tier`: free 50 q/day, pro 500 [quota-manager.ts:40-56]).

## MCP tools (48 non-compliance incl. re_auth + 18 compliance, names found in definitions)

From definitions (`src/tools/definitions/*.ts`, `compliance/compliance-tools.ts`): ask_question(question req; session_id, notebook_id, notebook_url, show_browser, browser_options), get_notebook_chat_history, generate_data_table, get_data_table, deep_research(query req), gemini_query(query; schema; model...), get_research_status(interaction_id req), upload_document(file_path req), query_document(file_name, query req), list_documents, delete_document(file_name, confirm req), query_chunked_document(file_names, query req), add_notebook(url,name,description,topics req), list_notebooks, get_notebook(id), select_notebook(id), update_notebook(id...), remove_notebook(id), search_notebooks(query), get_library_stats, create_notebook(name, sources req), sync_library(auto_fix), list_sources, add_source(source req), add_folder(folder_path req; recursive, file_types, dry_run, notebook_name_prefix), remove_source(source_id req), export_library(format, output_path), batch_create_notebooks(notebooks req; stop_on_error), generate_audio_overview, get_audio_status, download_audio(output_path), get_query_history, list_sessions, close_session(session_id), reset_session(session_id), get_health(deep_check, notebook_id), setup_auth, re_auth (system.ts:87), cleanup_data(confirm req; preserve_library, include_optional), get_quota(sync), set_quota_tier(tier req), get_project_info, configure_webhook(name,url req; id, enabled, events, format, secret), list_webhooks, test_webhook(id), remove_webhook(id), generate_video_overview, get_video_status, plus 18 compliance tools (compliance_dashboard, compliance_score, generate_compliance_report, collect_audit_evidence, verify_evidence_integrity, list_evidence_packages, submit_dsar, export_user_data, request_data_erasure, get_consent_status, grant_consent, revoke_consent, report_security_incident, get_incident_status, run_health_check, verify_audit_log_integrity, list_policies, get_policy). Tool profiles minimal/standard/full and "advanced tools" gate (`index.ts:~175`). Results carry `structuredContent` and `_errorType` domain|transport, `isError` (commit 3d95b96); annotations corrected (readOnlyHint etc., commit 125755c).

## Reliability

Rate limiter, quota manager (`canMakeQuery` pure, `incrementQueryCountAtomic`), exponential auth lockout, webhook retry with persisted delivery records, shutdown flush of audit/query logs (commit 3d95b96). Answer wait default 120 s (shorter than PleasePrompto 600 s).

## Performance notes

All security layers run synchronously in the Node event loop; they list as known limitation: "audit tamper-anchor writes synchronously per event, `FileLock` uses sync I/O on the quota/audit paths" (CHANGELOG 2026.5.0). ML-KEM key encapsulation per encrypted write is cheap (ms) but each save also does PBKDF2 600k iterations when a passphrase key is used [pbkdf2Sync at crypto.ts:157; per-call cost not measured].

## Tests and quality

791 tests / 70 files (CHANGELOG 2026.5.0), shuffle on, regression tests "proven to fail before the fix" per finding FX-015..027, MEDUSA scanning in CI (README), parallel four-reviewer AI audit of 334 issues (SECURITY.md header). Strongest test culture among the six repos.

## Versions

| Tag | Date | Notable |
|---|---|---|
| v1.0.1-v1.1.2 | 2025-10-16..19 | same as PleasePrompto |
| v1.2.0 | 2025-11-21 | tool profiles (parent) |
| v1.5.0/1 | 2025-12-11 | security layers begin |
| v1.6.0 | 2025-12-18 | compliance-ready architecture |
| v1.8.0 | 2025-12-20 | Gemini Deep Research |
| v1.9.0 | 2025-12-21 | Document API |
| v1.10.0..9 | 2025-12-21..2026-01-03 | PDF chunking, query history |
| v2026.1.0-12 | 2026-01-04..02-16 | calver; chat extraction; multi-session |
| v2026.2.0-11 | 2026-02-17..03-28 | video/data tables, 14 -> 17 layers (2026.2.10 2026-03-15: secure-by-default auth, backoff lockout, credential isolation) |
| v2026.3.0-3 | 2026-04-25..05-11 | Security Audit release, 334 issues; bin shim fix |
| v2026.4.0/1 | 2026-06-01 | reliability + manager refactors |
| v2026.5.0 | 2026-09-03 | FX-015..027; notebook.google.com support; schema validation enforced |

## Adopt

1. Host-agnostic URL check in ONE function, used by auth detection, navigation and tool validation (`security.ts:286-300`; FX-026 lesson). In Go: one `IsNotebookHost(u)` accepting both `notebooklm.google.*` and `notebook.google.*`.
2. https-only + scheme blocklist + private-IP block for user-supplied source URLs, and resolve DNS and re-check at connect time (webhook-dispatcher.ts:146-168 pattern; stricter than security.ts:184-205).
3. Path allowlist with symlink resolution (`security.ts:417-480`) - port with Windows system paths added.
4. Enforce each tool's JSON schema at dispatch (the Go SDK does it for typed tools) and keep it fail-closed, unlike `schema-validator.ts:231-238`.
5. Redact at the sink: single `Redact()` for emails, `key=value` secrets, URL userinfo, absolute paths (`security.ts:304-348`), applied centrally in the logger rather than at call sites.
6. Hash-chained JSONL audit log with cross-day link, serialized writes and external checkpoint (`audit-logger.ts:88-110, 272-285, 361-374`) - optional, only if enterprise mode is wanted.
7. Secret files with 0600 / Windows ACL (`file-permissions.ts`) and keeping the machine key out of the same file as data; prefer OS keychain in Go (not done here).
8. Treat NotebookLM output as untrusted: invisible-char/NFKC normalisation then pattern checks, surfaced as `security_warnings` rather than silent rewriting (`response-validator.ts:70-79, 227-330`); add provenance marker (PleasePrompto v2 idea).
9. Selector discovery tooling and `{primary, fallbacks, confirmed, date}` records (`notebook-creation/selectors.ts:36-45`, `scripts/selector-discovery.ts`) for our browser fallback maintenance.
10. Admin vs read tool classes and lockout-with-backoff for any HTTP transport we expose (`index.ts:136-171`, `mcp-auth.ts:39-58`).
11. Regression-test-per-finding discipline and shuffled test order (CHANGELOG 2026.5.0).

## Avoid

1. Shipping an imperative "EXTREMELY IMPORTANT..." follow-up suffix on by default (`config.ts:24-25, 184`).
2. Claiming a per-page mutex that exists only on notebook creation (SECURITY.md vs `notebook-creator.ts`); real serialization of page access was not implemented for ask.
3. `div.thinking-message` gating (`page-utils.ts:198-210`) - dead selector.
4. Response validator as the main injection defense: regex, English-only, over-redacts, bypassable; keep as defense in depth only. NFKC-normalising returned text silently changes legitimate content.
5. Secrets scanner not applied to returned content or audit log (see #5); "17 layers" marketing count includes cosmetic items (post-quantum for local files whose key is a machine key on the same disk).
6. Token auth transported in `_meta` over stdio: false sense of security when the client is the trust boundary.
7. 21-module compliance suite in a local tool: size/maintenance cost. Keep scope to what NotebookLM users need.
8. Synchronous crypto/IO on hot paths (acknowledged limitation).
9. Windows blind spots: denylist POSIX-only, `icacls` fallback "gracefully if it fails" (`file-permissions.ts:10-12`) means files may stay world-readable on failure.
10. Source-URL SSRF check without DNS resolution (`security.ts:184-205`).

## Evidence

Read: `src/utils/{security,response-validator,secure-memory,crypto,secrets-scanner,file-permissions,schema-validator,file-lock,logger,audit-logger,page-utils}.ts` (function inventories and cited ranges), `src/auth/mcp-auth.ts`, `src/index.ts:130-175, 405-490`, `src/config.ts`, `src/session/{shared-context-manager,session-timeout}.ts`, `src/tools/handlers/{ask-question,index,gemini}.ts`, `src/webhooks/webhook-dispatcher.ts`, `src/quota/quota-manager.ts`, `src/notebook-creation/selectors.ts`, CHANGELOG.md 2026.5.0 and SECURITY.md, Dockerfile; `git log`, `git tag`, `git show e36b246`. REA JSON module map for exports/imports.
