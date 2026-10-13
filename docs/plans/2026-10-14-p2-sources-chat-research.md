# P2 Sources, Chat and Research Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 12 tools for sources, research and streamed chat with citations: `source_add`, `source_list`, `source_read`, `source_update`, `research_start`, `research_status`, `research_import`, `chat_ask`, `chat_status`, `chat_history`, `chat_configure`, `suggest`.

**Architecture:** Same layers as P1. Chat uses a new streaming path in `transport/rpc` (`GenerateFreeFormStreamed`), decoded incrementally by `wire.StreamDecoder`, surfaced as MCP progress notifications. Long jobs live in `internal/jobs`.

**Tech Stack:** P1 stack plus `golang.org/x/sync/errgroup`.

**Spec:** [architecture](2026-10-14-architecture.md), [rpc-catalog §3 Sources/Research/Chat, §4, §5](../../research/rpc-catalog.md), [feature-matrix tool table rows 8–19](../../research/feature-matrix.md).

## Global Constraints

- All P1 constraints apply.
- Non-idempotent sends (`izAoDd`, `o4cbdc`, `LBwxtb`, stream ask) are never retried after the request body was written.
- Payload shapes marked "conflicting" in rpc-catalog must have a live fixture (`synthetic:false`) before the task's commit. If none exists yet, the task stops at Step 4 and asks the owner for a capture (rpc-catalog §10 items 8–23).
- Chat request: 9-element params `[sources, question, history|null, [2,null,[1],[1]], conversation_id|null, null, null, notebook_id, seq]`; sources as explicit ids, never an empty list.
- Progress notifications at most 1 per second, `progress` strictly increasing.
- Uploads stream from disk; never read a whole file into memory. Allowed upload roots from `NOTEBOOKLM_ALLOWED_ROOTS` (default: user home).

## Review Focus

1. Add-source returns code 3 or 9 although the source was created → reconcile by listing, report success, no duplicate. Test in Task 2.
2. Chat stream where a later cumulative chunk rewrites citation ranges (`[2,3]` → `[2-5]`) → final answer uses the last snapshot, citations re-resolved. Test in Task 6.
3. Thinking chunks (type flag 2) interleaved with answer chunks → only answer text returned. Test in Task 6.
4. URL source pointing to `http://127.0.0.1`, `file://` or a private range → rejected before any Google call. Test in Task 1.
5. Upload of a 200 MB file on Windows → bytes streamed in 64 KiB chunks, peak RSS growth < 20 MB. Test in Task 3.

---

### Task 1: Safety checks

**Files:** Create `internal/safety/url.go`, `internal/safety/path.go`; Test `internal/safety/url_test.go`, `internal/safety/path_test.go`

**Interfaces:**
- Produces: `func CheckPublicURL(ctx context.Context, raw string, r *net.Resolver) (*url.URL, error)`; `func CheckPath(p string, roots []string) (string, error)` (returns cleaned absolute path; rejects symlinks escaping roots, `..`, device paths, UNC on Windows).

- [ ] **Step 1: Tests:** reject `file:`, `javascript:`, `127.0.0.1`, `[::1]`, `10.0.0.1`, `169.254.169.254`, a hostname resolving to `192.168.1.5` (fake resolver); accept `https://example.com/a`. Path: accept file under root; reject `../` escape, symlink to outside root, `\\server\share\x` on Windows.
- [ ] **Step 2: Run** `go test ./internal/safety -v` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(safety): SSRF and path allow-root checks`.

### Task 2: Source add (URL, YouTube, text, Drive) with pending reconciliation

**Files:** Create `internal/nlm/sources.go`; Test `internal/nlm/sources_test.go`, fixtures `testdata/fixtures/izAoDd/*`, `rLM1Ne/after_add.json`

**Interfaces:**
- Consumes: P1 `nlm.Client`, `transport.Call{PendingCodes}`.
- Produces:
  - `type SourceSpec struct{ URL string; Text *TextSpec; Drive *DriveSpec }` (`TextSpec{Title, Content}`, `DriveSpec{DocumentID, MIME, Title}`)
  - `type AddOutcome struct{ Index int; SourceID string; Status string /* added|pending|failed */; Error string }`
  - `func (c *Client) AddSources(ctx, nb string, specs []SourceSpec) ([]AddOutcome, error)` — one `izAoDd` per batch of URL/text specs; YouTube auto-detected (`youtube.com/watch`, `youtu.be/`, `/shorts/`) into slot 7; Drive uses its own call.
  - `func (c *Client) ListSources(ctx, nb string) ([]Source, error)` (via `GetNotebook`).
- [ ] **Step 1: Tests:** spec slot layout per rpc-catalog (URL `[null,null,[url],null*7,1]`, text `[null,[title,content],null,2,null*6,1]`, YouTube slot 7, Drive `[[id,mime,1,title],null*9,1]`); `TestPendingReconciles` (code 9, then list shows the URL → outcome `added`, exactly one `izAoDd` call); `TestPartialFailure` (2 specs sent, response omits one → that one `failed`); `TestNoRetryAfterSend` (network error after write → `KindTransport`, 1 call).
- [ ] **Step 2: Run** `go test ./internal/nlm -run 'TestAdd|TestPending|TestPartial|TestNoRetry' -v` → FAIL.
- [ ] **Step 3: Implement.** Request tail = shape pinned by the live fixture (rpc-catalog §10 item 6).
- [ ] **Step 4: Run** → PASS (or STOP for capture).
- [ ] **Step 5: Commit** `feat(nlm): add sources with pending reconciliation`.

### Task 3: File upload

**Files:** Create `internal/nlm/upload.go`, `internal/transport/rpc/upload.go`; Test `internal/nlm/upload_test.go`

**Interfaces:**
- Produces: `func (c *Client) UploadFile(ctx, nb, path string) (AddOutcome, error)`; transport method `func (t *Transport) Upload(ctx, nb, sourceID, name string, r io.Reader, size int64) error`.
- [ ] **Step 1: Tests (fake upstream):** `o4cbdc` called first with `[[[filename]], nb, ...]`; start POST to `/upload/_/` has `x-goog-upload-command: start`, `x-goog-upload-protocol: resumable`, `x-goog-upload-header-content-length`, body keys in order `PROJECT_ID, SOURCE_NAME, SOURCE_ID` (or the form body if the capture says so); upload POST uses `upload, finalize` and offset 0; 5xx with `X-Goog-Upload-Status: final` = success; `TestUploadStreams` uploads a 200 MB sparse temp file and asserts `runtime.MemStats.HeapInuse` growth < 20 MB.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.** Ordered JSON via `jsontext.Encoder` tokens, never a Go map. Path checked with `safety.CheckPath`.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(nlm): resumable file upload`.

### Task 4: Source read and update

**Files:** Modify `internal/nlm/sources.go`; Test `internal/nlm/sources_read_test.go`, fixtures `hizoJc/*`, `tr032e/*`, `b7Wfje/*`, `FLmJqe/*`, `yR9Yof/*`

**Interfaces:**
- Produces: `func (c *Client) SourceGuide(ctx, nb, sid string) (summary string, keywords []string, err error)`; `func (c *Client) SourceText(ctx, nb, sid string, markdown bool) (string, error)`; `func (c *Client) RenameSource(ctx, nb, sid, title string) error`; `func (c *Client) RefreshSource(ctx, nb, sid string) error` (fallback `dtT1F` when `FLmJqe` returns `[3]`); `func (c *Client) SourceFresh(ctx, nb, sid string) (bool, error)`.
- [ ] **Step 1: Tests:** request shapes `[[sid],[2],[2]]` text / `[[sid],[3],[3]]` markdown; text blocks at `[3][0]`; rename `[null,[sid],[[[title]]]]`; refresh fallback path; freshness `[]` = fresh.
- [ ] **Step 2–4:** run FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(nlm): source guide, text, rename, refresh, freshness`.

### Task 5: Research

**Files:** Create `internal/nlm/research.go`; Test `internal/nlm/research_test.go`, fixtures `Es3dTe/*`, `Ljjv0c/*`, `QA9ei/*`, `e3bVqc/*`, `LBwxtb/*`, `Zbrupe/*`

**Interfaces:**
- Produces: `type ResearchJob struct{ ID, Mode, Query string; Status ResearchStatus; Candidates []Candidate; Report string }`; `func (c *Client) StartResearch(ctx, nb, query, mode, corpus string) (ResearchJob, error)`; `func (c *Client) PollResearch(ctx, nb string) ([]ResearchJob, error)`; `func (c *Client) ImportResearch(ctx, nb, jobID string, picks []Candidate) ([]AddOutcome, error)` (filters URLs already present); `func (c *Client) CancelResearch(ctx, runID string) error`.
- [ ] **Step 1: Tests:** mode mapping discover→`Es3dTe`, fast→`Ljjv0c` `[[q,1|2],null,1,nb]`, deep→`QA9ei` `[null,[1],[q,1],5,nb]`; status values pinned from live fixture (item 16); import entry shapes for web, Drive, deep report; import skips URLs already in the notebook.
- [ ] **Step 2–4:** FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(nlm): discover, fast and deep research`.

### Task 6: Chat stream

**Files:** Create `internal/wire/stream.go`, `internal/transport/rpc/stream.go`, `internal/nlm/chat.go`; Test `internal/wire/stream_test.go`, `internal/nlm/chat_test.go`, fixtures `stream/*`

**Interfaces:**
- Produces:
  - `type StreamDecoder struct{...}`; `func NewStreamDecoder(r io.Reader) *StreamDecoder`; `func (d *StreamDecoder) Next() (Snapshot, error)`; `type Snapshot struct{ Text string; Thinking bool; Final bool; Citations []Citation; ConvHint string; Status *Status }`
  - `type Citation struct{ N int; SourceID string; Text string; Start, End int /* UTF-16 units */ }`
  - transport: `func (t *Transport) Stream(ctx, nb string, body string, onSnap func(wire.Snapshot)) (wire.Snapshot, error)` posting to `/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed` with `bl`, `hl`, `_reqid`, `f.sid`, `rt=c`.
  - `type Answer struct{ Text string; Citations []Citation; ConversationID string; FollowUps []string }`
  - `func (c *Client) Ask(ctx, nb, question string, opts AskOptions, progress func(text string)) (Answer, error)`; `AskOptions{SourceIDs []string; ConversationID string; NewConversation bool}`.
- [ ] **Step 1: Tests:** cumulative snapshots → final text = last answer snapshot; thinking flag 2 ignored; heartbeat `[]` skipped; citation N maps to passage N-1, source id at `passage[1][5][0][0][0]`, offsets in UTF-16 for an emoji answer; rejection frame with `UserDisplayableError` → `KindQuota` with authored message; conversation id resolved through `hPTbtc` after the first ask (not the stream session id); idle timeout 5 min and 1 MiB answer cap enforced (fake clock); `progress` called at most once per second.
- [ ] **Step 2–4:** FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(chat): streamed ask with incremental decode and citations`.

### Task 7: Chat history, configure, suggestions

**Files:** Modify `internal/nlm/chat.go`; Create `internal/nlm/suggest.go`; Test `internal/nlm/chat_history_test.go`, fixtures `hPTbtc/*`, `khqZz/*`, `s0tc2d/chat_config.json`, `otmP3b/*`, `OcvKNc/*`

**Interfaces:**
- Produces: `func (c *Client) History(ctx, nb, conv string, limit int, cursor string) ([]Turn, string, error)`; `func (c *Client) Configure(ctx, nb string, cfg ChatConfig) error` (merges partial settings: reads `rLM1Ne[7]` first); `func (c *Client) Suggest(ctx, nb, surface string, sources []string, query string) ([]Suggestion, error)`.
- [ ] **Step 1–4:** tests for turn ordering (newest first from server, returned oldest first), cursor paging, partial config merge (only `response_length` changes → goal preserved), suggestion parsing; FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(chat): history, configure, suggestions`.

### Task 8: Jobs table

**Files:** Create `internal/jobs/jobs.go`; Test `internal/jobs/jobs_test.go`

**Interfaces:**
- Produces: `type Job struct{ ID, Kind, Status string; Result any; Err error; Started time.Time }`; `func (t *Table) Start(kind string, f func(ctx context.Context) (any, error)) string`; `func (t *Table) Get(id string) (Job, bool)`; `func (t *Table) Cancel(id string) bool`; finished jobs evicted after 1 h.
- [ ] **Step 1–4:** tests for start/get/cancel/evict with fake clock; FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(jobs): local job table`.

### Task 9: MCP tools for P2

**Files:** Create `internal/mcpserver/tools_sources.go`, `tools_research.go`, `tools_chat.go`; Test `internal/mcpserver/p2_tools_test.go`

**Interfaces:**
- Consumes: Tasks 1–8, P1 `register`, `resolve`, `cache`.
- Produces tools exactly as feature-matrix rows 8–19 (names, params, annotations). Cache invalidation: `source_add`, `source_update`, `research_import` invalidate `<profile>/notebook/<id>` and `<profile>/notebooks`. `chat_ask` with `wait=false` returns `{job}`; `chat_status` reads the jobs table. Progress via `req.Session.NotifyProgress` only when the client sent a progress token.
- [ ] **Step 1: Tests:** each tool's happy path through in-memory MCP; `source_add` with an SSRF URL → envelope `code:"INVALID_ARGUMENT"` and zero upstream calls; `chat_ask` emits ≥ 1 progress notification for a 3-snapshot stream; `research_start` rejects queries < 4 words unless `force:true`.
- [ ] **Step 2–4:** FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(mcpserver): source, research and chat tools`.
