# P4 Browser Fallback and Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Browser login (`auth_login method=browser|attach`), the transport router with the fallback policy, an in-page-fetch browser transport for every RPC, and DOM automation for chat and add-source only.

**Architecture:** `transport.Router` wraps `rpc` and `browser` transports. The browser transport launches lazily (first fallback or login), owns one dedicated user-data-dir per profile, keeps one warm NotebookLM tab, and runs RPCs via `fetch` inside the signed-in page. DOM paths use a selector registry. Idle browser reaped after 5 min.

**Tech Stack:** P1 stack + `github.com/chromedp/chromedp` (latest release at execution time; record the version in `go.mod` review).

**Spec:** [architecture §2, §5, §7](2026-10-14-architecture.md), [best-practices: browser automation, fallback policy](../../research/best-practices.md), notes [arlinamid](../../research/notes/arlinamid__notebooklm-mcp.md), [icebear0828](../../research/notes/icebear0828__notebooklm-client.md), [PleasePrompto](../../research/notes/PleasePrompto__notebooklm-mcp.md), [Pantheon](../../research/notes/Pantheon-Security__notebooklm-mcp-secure.md).

## Global Constraints

- All P1 constraints apply.
- Never use the user's default Chrome profile (Chrome 136+ blocks remote debugging there). Profile dir: `<config>/browser/<profile>`.
- Remove `--enable-automation` from chromedp default flags; do not add stealth plugins.
- Fallback only on `KindTransport`, `KindDrift`, `KindCookieBinding`. Result reports `transport:"browser"`.
- Claude/agents never type credentials; `auth_login method=browser` opens a visible window and waits for the human to sign in (timeout default 300 s).
- Browser absent → `KindUnsupported` with guidance to install Chrome/Edge; RPC path keeps working.

## Review Focus

1. RPC returns `KindRateLimited` → no fallback, browser never launched. Test in Task 2.
2. Two concurrent fallbacks on the same profile → one browser, per-tab FIFO lock, both succeed. Test in Task 3.
3. Browser idle 5 min → process tree exits; next fallback relaunches. Test in Task 3.
4. Login window closed by the user before sign-in → `auth_login` returns `code:"CANCELLED"`, no profile saved. Test in Task 4.
5. NotebookLM UI selector missing (DOM drift) → `KindDrift` naming the selector key, never a hang. Test in Task 5.

---

### Task 1: Router

**Files:** Create `internal/transport/router.go`; Test `internal/transport/router_test.go`

**Interfaces:**
- Produces: `func NewRouter(primary Transport, fallback func(ctx context.Context) (Transport, error)) *Router` (fallback constructed lazily); `Router` implements `Transport`; `func (r *Router) Last() string`.
- [ ] **Step 1: Tests:** table over all 12 kinds: fallback invoked only for Transport/Drift/CookieBinding; fallback constructor not called otherwise; `Last()` reflects the transport that answered; fallback error wraps both errors.
- [ ] **Step 2–4:** FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(transport): router with fallback policy`.

### Task 2: Browser launcher and lifecycle

**Files:** Create `internal/transport/browser/launch.go`, `internal/transport/browser/lifecycle.go`; Test `internal/transport/browser/launch_test.go` (`//go:build browser` for tests needing Chrome; unit tests for flag building run always)

**Interfaces:**
- Produces: `type Options struct{ ProfileDir string; Headless bool; ExecPath string; Idle time.Duration }`; `func Launch(ctx context.Context, o Options) (*Browser, error)`; `func (b *Browser) Tab(ctx context.Context) (context.Context, func(), error)` (FIFO per tab); `func (b *Browser) Close() error`; blocked URL patterns for images, fonts, media via `network.SetBlockedURLs`.
- [ ] **Step 1–4:** unit tests for flags (no `enable-automation`, user-data-dir set, headless toggle); browser-tagged tests: launch, open tab, idle reap; FAIL → implement → PASS (`go test -tags browser ./internal/transport/browser -v` locally).
- [ ] **Step 5: Commit** `feat(browser): lazy launcher with idle reaper`.

### Task 3: In-page fetch transport

**Files:** Create `internal/transport/browser/fetch.go`; Test `internal/transport/browser/fetch_test.go`

**Interfaces:**
- Consumes: `wire.EncodeBatch`, `wire.Decode`, `Browser.Tab`.
- Produces: `func NewTransport(b func(ctx) (*Browser, error), host session.Host) transport.Transport` — evaluates a `fetch()` to batchexecute inside the NotebookLM tab using page-scoped tokens (`WIZ_global_data`), returns the body to `wire.Decode`.
- [ ] **Step 1–4:** tests with a local `httptest` page that mimics `WIZ_global_data` and the batchexecute endpoint; concurrency test (2 callers, 1 browser); FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(browser): in-page fetch transport`.

### Task 4: Login flows

**Files:** Create `internal/transport/browser/login.go`, `internal/mcpserver/tools_auth.go`; Modify `cmd/sourcebolt/login.go`; Test `internal/transport/browser/login_test.go`

**Interfaces:**
- Produces: `func LoginInteractive(ctx, o Options, h session.Host) (*session.Jar, session.Host, error)` (waits until a page on a notebook host has `SNlM0e`, records landed host); `func AttachCDP(ctx, url string, h session.Host) (*session.Jar, error)`; tools `auth_login` (methods `browser`, `attach`, `import`) and `auth_logout`; CLI `login --browser`, `login --attach <cdp-url>`. Every path validates with a real `wXbhsf` before `Store.Save`.
- [ ] **Step 1–4:** tests: cancelled window → `CANCELLED`; landed host recorded; validation failure → nothing saved; FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(auth): browser and CDP-attach login`.

### Task 5: DOM automation for chat and add-source

**Files:** Create `internal/transport/browser/selectors.go`, `internal/transport/browser/dom_chat.go`, `internal/transport/browser/dom_sources.go`; Test `internal/transport/browser/dom_test.go` (`browser` tag, local HTML fixtures copied from documented DOM structure, not from Google pages)

**Interfaces:**
- Produces: selector registry `map[string][]string` (ordered fallbacks: class → icon text → ARIA role → locale text); `func (b *Browser) AskDOM(ctx, nb, q string, progress func(string)) (nlm.Answer, error)` with completion = `.message-actions` present + answer-bubble count stable 1 s; `func (b *Browser) AddSourceDOM(ctx, nb string, spec nlm.SourceSpec) error`. `nlm.Client` uses these only when the router fell back and the RPC error was `KindDrift` for chat or add-source.
- [ ] **Step 1–4:** tests: missing selector → `KindDrift` naming key within timeout; completion detection on a fixture page that streams text; FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(browser): DOM fallback for chat and add-source`.
