# P4 Task 5: DOM Fallback for Chat and Add-Source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When NotebookLM changes its chat or add-source RPC shape (`KindDrift`), answer the question or add the source by driving the signed-in NotebookLM page instead of failing.

**Architecture:** A selector registry (embedded JSON, overridable from `<config>/selectors.json`) names each UI control as an ordered list of candidates. One injected JS helper resolves candidates in the page; Go drives the steps over the existing `page` interface (`Eval`, `Navigate`) and polls for completion. `nlm.Client` gets an optional `DOM` field; `Ask` and `AddSources` call it only after a `KindDrift` from the RPC path. `internal/app` wires the browser implementation in when the browser fallback is enabled.

**Tech Stack:** Go 1.27, chromedp (already in `go.mod`), `embed`, the existing `internal/transport/browser` launcher and `page` abstraction.

**Spec:** [P4 plan, Task 5](../../plans/2026-10-14-p4-browser-and-login.md), [architecture §5, §7](../../plans/2026-10-14-architecture.md), selector research in [PleasePrompto notes](../../../research/notes/PleasePrompto__notebooklm-mcp.md) (§Selectors) and its fork notes (paolodalprato, arlinamid).

## Global Constraints

- All P1 and P4 constraints apply: never the user's default Chrome profile; no `--enable-automation`; no stealth plugins; agents never type credentials.
- Selector order per control: Angular class → Material icon text → ARIA role → locale text. Only EN locale text is added (others are unverified upstream).
- Modal anchor is `[role="dialog"]` excluding promo dialogs (host tag matching `/promo|announcement|onboarding|whats-?new|welcome|tutorial|tour/i`); never `.mdc-dialog--open` or `.cdk-overlay-pane`.
- No human-like typing delays: set the value and dispatch `input`, then click submit.
- A missing control returns `KindDrift` whose message names the registry key, within `selectorWait` (10 s); never a hang.
- Chat completion: `.message-actions` present on the newest answer bubble AND bubble text unchanged for 1 s AND bubble count > count before sending. Hard ceiling = `rpc.Options.StreamTotal` default (15 min).
- Drive sources stay rejected on the DOM path too (same `KindUnsupported` as RPC).
- Test fixture pages are written by hand from the documented structure in research notes; never copied from Google pages.
- Tests never launch a browser unless built with `-tags browser`; `NOTEBOOKLM_BROWSER_FALLBACK=0` stays set in default tests.

## Decision needed before execution (owner)

- **D-T5-1: retry chat on DOM after a drift that happened after the question was sent?** A drift while decoding the stream (`Sent=true`) means Google may have answered and counted it against the daily quota. Recommended: **yes**, because the user got nothing usable. The cost is one extra chat turn and quota unit, and the answer says `transport:"browser-dom"`. With **no**, only pre-send drift falls back. Add-source after send is never retried on DOM (a duplicate source is worse than an error); the RPC path's pending reconciliation handles it.

## Review Focus

1. A promo dialog covering the page when the fallback starts → dismissed, or ignored by the dialog selector; the ask still succeeds. Test in Task 2.
2. The answer is identical to the previous answer in the thread → completed by the bubble count rising, never a hang. Test in Task 2.
3. The daily chat limit is hit on the DOM path (an `[role="alert"]` with rate-limit text) → `KindRateLimited`, not `KindDrift` or a timeout. Test in Task 2.
4. The context is cancelled mid-answer → returns promptly with the caller's error and leaves the tab usable for the next call. Test in Task 2.
5. A user selector override file with invalid JSON → the server starts with the embedded registry and logs one stderr line. Test in Task 1.

---

### Task 1: Selector registry and in-page resolver

**Files:**
- Create: `internal/transport/browser/selectors.go`, `internal/transport/browser/selectors.json` (embedded), `internal/transport/browser/dom.js` (embedded resolver)
- Test: `internal/transport/browser/selectors_test.go` (no build tag; fake `page`)

**Interfaces:**
- Produces: `type Selectors map[string][]string`; `func LoadSelectors(configDir string, warn io.Writer) Selectors` (embedded defaults merged with an override file `<configDir>/selectors.json`; per-key override replaces that key's list); `func (s Selectors) find(ctx context.Context, p page, key string, wait time.Duration) error` (polls the resolver every 100 ms until a visible match or `wait`, then `KindDrift` with message `selector "<key>" not found (tried N candidates); NotebookLM UI may have changed`).
- Candidate syntax: plain CSS; `text:<substring>` matches visible button/menuitem text, case-insensitive; `icon:<name>` matches a `mat-icon` with that ligature text and resolves to its closest `button,[role=button]`.
- Registry keys and first candidates (from the research notes): `chat.input` (`textarea.query-box-input`, `textarea[aria-label="Query box"]`), `chat.submit` (`query-box .submit-button button`, `button.submit-button`), `chat.bubble` (`.to-user-container`, `.chat-message-pair`), `chat.bubble_text` (`.message-text-content`), `chat.done` (`.message-actions`), `chat.alert` (`[role="alert"]`, `.error-message`, `.rate-limit-message`), `sources.add` (`.add-source-button`, `icon:add`), `sources.dialog` (`mat-dialog-container[role="dialog"]:not([data-sb-promo])`, `[role="dialog"]:not([data-sb-promo])`), `sources.type_link` (`icon:link_2`, `icon:link`, `text:Website`), `sources.type_text` (`icon:content_paste`, `text:Copied text`), `sources.url_input` (`[role="dialog"] input[type="url"]`, `[role="dialog"] textarea`), `sources.text_input` (`[role="dialog"] textarea`), `sources.insert` (`button.mdc-button--unelevated.mat-primary`, `text:Insert`), `sources.row` (`.single-source-container`).

- [ ] **Step 1: Write failing tests** `TestFindMissingKeyIsDrift` (fake page whose resolver result is always `null`; `wait`=200 ms; assert `KindDrift`, message contains `"chat.input"`, returns in < 1 s), `TestOverrideReplacesKey` (temp dir with `{"chat.input":["#mine"]}` → `s["chat.input"]` equals `["#mine"]`, other keys equal the defaults), `TestBadOverrideKeepsDefaults` (invalid JSON → defaults, exactly one line written to `warn`), `TestEmbeddedRegistryHasAllKeys` (every key listed above is present and non-empty).
- [ ] **Step 2: Run** `go test ./internal/transport/browser -run 'Find|Override|Registry' -v`. Expected: FAIL (undefined symbols).
- [ ] **Step 3: Implement** `selectors.go`, `selectors.json` and `dom.js`. `dom.js` defines `window.__sbDom = {find(cands), visible(el), dismissPromos()}` and is injected once per tab (`Eval` of the embedded source when `typeof window.__sbDom === "undefined"`). `dismissPromos` clicks a close button in, or marks with `data-sb-promo`, every dialog whose host tag matches the promo regex.
- [ ] **Step 4: Run** the same command. Expected: PASS.
- [ ] **Step 5: Commit** `feat(browser): selector registry with embedded defaults and override file`.

### Task 2: AskDOM

**Files:**
- Create: `internal/transport/browser/dom_chat.go`, `internal/transport/browser/testdata/dom/chat.html` (fixture: query box, submit button, a script that appends a `.to-user-container` bubble, streams text into it over ~600 ms, then adds `.message-actions`; query param `?same=1` repeats the previous answer text; `?limit=1` shows `[role="alert"]` "You have reached your daily limit")
- Test: `internal/transport/browser/dom_browser_test.go` (`//go:build browser`)

**Interfaces:**
- Consumes: `Selectors.find` (Task 1), the `page` interface, `tabAcquirer` from `fetch.go`.
- Produces: `type DOM struct` built by `func NewDOM(get func(ctx context.Context) (*Browser, error), host session.Host, sel Selectors) *DOM`; `func (d *DOM) Ask(ctx context.Context, nb, question string, progress func(text string)) (text string, err error)`. It navigates to `host.Origin()+"/notebook/"+nb` when the tab is elsewhere, records the bubble count, sets the input, clicks submit, then polls every 250 ms. `progress` gets new text at most once per second. Citations are not harvested (text only; `[N]` markers kept as rendered).

- [ ] **Step 1: Write failing tests** `TestAskDOMAnswers` (answer text equals the fixture's final text; progress called at least once), `TestAskDOMSameAnswerCompletes` (`?same=1`; returns within 5 s), `TestAskDOMRateLimitAlert` (`?limit=1` → `KindRateLimited`), `TestAskDOMMissingInputIsDrift` (fixture without the textarea → `KindDrift` naming `chat.input` within `selectorWait`+1 s), `TestAskDOMCancel` (cancel after 100 ms → returns `context.Canceled` within 1 s; a second `Ask` on the same `DOM` succeeds), `TestAskDOMPromoDialog` (fixture with `<accessibility-promo-dialog role="dialog">` overlaying → still answers).
- [ ] **Step 2: Run** `go test -tags browser ./internal/transport/browser -run AskDOM -v`. Expected: FAIL.
- [ ] **Step 3: Implement** `dom_chat.go` with the completion rule from Global Constraints. Strip Material icon ligature lines (`more_vert`, `content_copy`, `keep_pin`, `thumb_up`, `thumb_down`) from `innerText`.
- [ ] **Step 4: Run** the same command. Expected: PASS.
- [ ] **Step 5: Commit** `feat(browser): DOM chat fallback`.

### Task 3: AddSourceDOM

**Files:**
- Create: `internal/transport/browser/dom_sources.go`, `internal/transport/browser/testdata/dom/sources.html` (fixture: add button opening a `mat-dialog-container[role=dialog]` with link and paste-text type buttons, inputs, and an Insert button that appends a `.single-source-container` row)
- Modify: `internal/transport/browser/dom_browser_test.go`

**Interfaces:**
- Consumes: Task 1 and Task 2's `DOM`.
- Produces: `func (d *DOM) AddURL(ctx context.Context, nb, url string) error`; `func (d *DOM) AddText(ctx context.Context, nb, title, content string) error`. Done = the `sources.row` count rose by 1 within 90 s; the dialog is gone. If `sources.add` is missing, navigate to `.../notebook/<nb>?addSource=true` once before returning drift. `title` is ignored when the dialog has no title field (NotebookLM names pasted text itself).

- [ ] **Step 1: Write failing tests** `TestAddURLDOM` (row count +1, dialog closed), `TestAddTextDOM`, `TestAddSourceMissingInsertIsDrift` (fixture variant without Insert → `KindDrift` naming `sources.insert`), `TestAddSourceAddSourceParamFallback` (fixture hides the add button unless `?addSource=true` → succeeds).
- [ ] **Step 2: Run** `go test -tags browser ./internal/transport/browser -run 'Add.*DOM|AddSource' -v`. Expected: FAIL.
- [ ] **Step 3: Implement** `dom_sources.go`.
- [ ] **Step 4: Run** the same command. Expected: PASS.
- [ ] **Step 5: Commit** `feat(browser): DOM add-source fallback`.

### Task 4: Wire into nlm and app; CI job

**Files:**
- Modify: `internal/nlm/client.go` (field), `internal/nlm/chat.go` (`Ask`), `internal/nlm/sources.go` (`AddSources`), `internal/app/app.go` (wiring), `.github/workflows/ci.yml` (browser-tag job on `ubuntu-latest`, which ships Chrome)
- Test: `internal/nlm/dom_fallback_test.go` (fake streamer and fake DOM; no tag)

**Interfaces:**
- Consumes: Task 2 and Task 3 methods.
- Produces: in `nlm`, `type DOMFallback interface { Ask(ctx context.Context, nb, question string, progress func(string)) (string, error); AddURL(ctx context.Context, nb, url string) error; AddText(ctx context.Context, nb, title, content string) error }` and `Client.DOM DOMFallback` (nil = off). `Answer` gains no fields; the transport name reported through `OnTransport` is `"browser-dom"`.
- Rules: `Ask` falls back only on `KindDrift` (and on `Sent=true` only if D-T5-1 is yes), and only when `o.SourceIDs` is empty and `o.ConversationID` is empty or equals the default thread (the DOM cannot scope sources or pick a thread). Otherwise it returns the original drift error. `AddSources` falls back per spec only on `KindDrift` with `Sent=false`, for URL and Text specs; outcome status `added` after the DOM confirms the row, and the notebook cache is invalidated as on the RPC path.

- [ ] **Step 1: Write failing tests** `TestAskFallsBackToDOMOnDrift` (streamer returns `KindDrift`, fake DOM returns "hi" → `Answer.Text`="hi", `OnTransport` got `"browser-dom"`), `TestAskNoDOMOnRateLimit` (`KindRateLimited` → DOM never called), `TestAskNoDOMWhenSourcesScoped` (drift + `SourceIDs` set → original error, DOM never called), `TestAskSentDriftPolicy` (asserts the D-T5-1 ruling), `TestAddSourcesDOMOnDriftNotSent`, `TestAddSourcesNoDOMAfterSend` (drift with `Sent=true` → error, DOM never called), `TestAddSourcesDOMNeverForDrive`.
- [ ] **Step 2: Run** `go test ./internal/nlm -run 'DOM' -v`. Expected: FAIL.
- [ ] **Step 3: Implement** the fallback in `nlm`; in `app.ForProfile`, when the browser fallback is on, set `rt.Deps.NLM.DOM` to a lazy adapter over `browser.NewDOM(rt.launcher.Get, p.Host, browser.LoadSelectors(o.browserConfigDir, os.Stderr))` that builds the launcher on first use, the same way `rt.newBrowser` does. Add the CI job: `go test -tags browser ./internal/transport/browser -run 'DOM|Find' -v` with `NOTEBOOKLM_BROWSER_FALLBACK=0`.
- [ ] **Step 4: Run** `go test -short ./...` and `go vet ./...`. Expected: PASS.
- [ ] **Step 5: Document** the selector override file and the `browser-dom` transport name in `docs/auth.md` (browser section) and regenerate `docs/tools.md` if any tool text changed (`go run ./tools/gendocs > docs/tools.md`).
- [ ] **Step 6: Commit** `feat(nlm): fall back to DOM chat and add-source on RPC drift`.

## Live verification (owner-gated, after merge)

The fixtures prove the mechanism, not today's NotebookLM DOM. A live check needs the owner's signed-in browser profile. It runs only with owner approval: force drift with `NOTEBOOKLM_RPC_IDS` pointing chat at a bogus id, ask one question in a throwaway notebook, and confirm the answer and `transport:"browser-dom"`.
