# P3 Studio, Notes, Labels, Sharing, Account and Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the 33-tool surface: `studio_generate`, `studio_list`, `studio_update`, `studio_export`, `note_list`, `note_save`, `share_status`, `share_update`, `label_list`, `label_update`, `account_settings`, `job_cancel`, `delete`, `notebook_create`, `notebook_update`, `chat_ask_many`, and `server_info include_usage`.

**Architecture:** Domain methods in `internal/nlm`, tools in `internal/mcpserver`. Studio is fire-and-return: `studio_generate` returns an artifact id; `studio_list wait_seconds` polls `gArtLc` with backoff 2 s → 10 s and progress notifications.

**Tech Stack:** P1/P2 stack. No new dependencies.

**Spec:** [architecture](2026-10-14-architecture.md), [rpc-catalog §3 Notebooks, Notes, Studio, Labels, Sharing, Account](../../research/rpc-catalog.md), [feature-matrix rows 1–7, 20–33](../../research/feature-matrix.md).

## Global Constraints

- All P1 and P2 constraints apply.
- `R7cb6c` type codes and config slots: 1 audio @6, 2 report @7, 3 video @8, 4 quiz/flashcards/interactive mind map @9, 7 infographic @14, 8 slides @16, 9 data table @18, 11 interactive report @34. Enum values (video style, slide format/length, infographic slot 6, quiz order) come only from live fixtures (rpc-catalog §10 items 24–32).
- `delete` and `share_update` widening access: return a preview unless `confirm:true` or MCP elicitation accepted.
- Downloads: only into `output_dir` under allowed roots; no overwrite unless `overwrite:true`; no symlink following; media fetched with the profile's `.googleusercontent.com` cookies.

## Review Focus

1. `studio_generate` while quota is exhausted (`EylDcb` shows 0 left or `R7cb6c` returns code 8) → `code:"QUOTA"` with reset time, no retry. Test in Task 1.
2. `gArtLc` row positions that differ per artifact type (video `[8][3]` vs `[8][4]`) → URLs found for every type in fixtures. Test in Task 2.
3. `delete kind=notebook` without `confirm` → preview listing titles, zero destructive calls. Test in Task 6.
4. Deleting a note-backed mind map → ghost cleanup via `cFji9` timestamp step, list no longer shows it. Test in Task 3.
5. `share_update public:true` → requires confirm; result echoes the new public link. Test in Task 4.

---

### Task 1: Studio generate

**Files:** Create `internal/nlm/studio.go`, `internal/nlm/studio_kinds.go`; Test `internal/nlm/studio_test.go`, fixtures `R7cb6c/<kind>.json` per kind, `EylDcb/*`

**Interfaces:**
- Produces: `type StudioKind string` (constants for the 10 kinds); `type StudioRequest struct{ Kind StudioKind; SourceIDs []string; Instructions, Language string; Audio *AudioCfg; Video *VideoCfg; Report *ReportCfg; Quiz *QuizCfg; Slides *SlidesCfg; Infographic *InfographicCfg; MindMap *MindMapCfg }`; `func (c *Client) Generate(ctx, nb string, r StudioRequest) (Artifact, error)`; `func (c *Client) Quota(ctx) (Quota, error)`.
- [ ] **Step 1: Tests:** one table test per kind asserting type code and config slot index; `language` sets `hl` on the call; mind map non-interactive uses `yyryJe` then `CYK0Xb`; quota pre-flight returns `KindQuota` when `pct_left==0`.
- [ ] **Step 2–4:** FAIL → implement → PASS (STOP for capture if a kind lacks a live fixture).
- [ ] **Step 5: Commit** `feat(studio): generate all artifact kinds`.

### Task 2: Studio list, get, update, export

**Files:** Modify `internal/nlm/studio.go`; Create `internal/nlm/download.go`; Test `internal/nlm/studio_list_test.go`, fixtures `gArtLc/*`, `v9rmvd/*`, `rc3d8d/*`, `Rytqqe/*`, `KmcKPe/*`, `Krh3pd/*`

**Interfaces:**
- Produces: `type Artifact struct{ ID, Title string; Kind StudioKind; Status ArtifactStatus; MediaURLs map[string]string; Created time.Time }`; `func (c *Client) ListArtifacts(ctx, nb string) ([]Artifact, error)`; `func (c *Client) GetArtifact(ctx, id string) (ArtifactContent, error)`; `func (c *Client) RenameArtifact(ctx, id, title string) error`; `func (c *Client) RetryArtifact(ctx, id string) error`; `func (c *Client) ReviseSlides(ctx, id string, edits []SlideEdit) error`; `func (c *Client) Download(ctx, a Artifact, format, dir string, overwrite bool) (string, error)`; `func (c *Client) ExportToDrive(ctx, id, title string, sheets bool) (string, error)`.
- [ ] **Step 1–4:** tests per type for media URL positions, status mapping (3 ready, 4 failed, 5 suggested filtered), download path safety and no-overwrite; FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(studio): list, content, rename, retry, revise, download, export`.

### Task 3: Notes

**Files:** Create `internal/nlm/notes.go`; Test `internal/nlm/notes_test.go`, fixtures `CYK0Xb/*`, `cFji9/*`, `cYAfTb/*`, `AH0mwd/*`

**Interfaces:**
- Produces: `func (c *Client) ListNotes(ctx, nb string, mindMaps bool) ([]Note, error)` (tombstones `[id,null,2]` filtered); `func (c *Client) SaveNote(ctx, nb string, n NoteInput) (Note, error)` (create then `cYAfTb` to set title); `func (c *Client) DeleteNotes(ctx, nb string, ids []string) error` (mind-map ghost cleanup).
- [ ] **Step 1–5:** tests, FAIL → implement → PASS, commit `feat(nlm): notes and mind-map cleanup`.

### Task 4: Sharing, labels, account, notebook mutations

**Files:** Create `internal/nlm/sharing.go`, `internal/nlm/labels.go`, `internal/nlm/account.go`; Modify `internal/nlm/notebooks.go`; Tests alongside; fixtures `QDyure/*`, `JFMDGd/*`, `RGP97b/*`, `I3xc3c/*`, `agX4Bc/*`, `le8sX/*`, `GyzE7e/*`, `ZwVcOc/*`, `hT54vc/*`, `CCqFvf/*`, `te3DCe/*`, `s0tc2d/*`, `LQhfEb/*`, `fejl7e/*`

**Interfaces:**
- Produces: `ShareStatus`, `ShareUpdate`, `func (c *Client) GetShare / UpdateShare`; `Label`, `func (c *Client) Labels / MutateLabels`; `Account{Tier, Limits, Language}`, `func (c *Client) Account / SetLanguage`; `func (c *Client) CreateNotebook(ctx, title, emoji string) (Notebook, error)` (never retried); `CopyNotebook`; `UpdateNotebook(ctx, id string, u NotebookUpdate) error` with read-back verification.
- [ ] **Step 1–5:** tests (share perms 2 editor / 3 viewer / 4 remove; autolabel code 4 = pending; create never retried; read-back mismatch → `KindInternal`), FAIL → implement → PASS, commit `feat(nlm): sharing, labels, account, notebook mutations`.

### Task 5: Delete and cancel

**Files:** Create `internal/nlm/delete.go`; Test `internal/nlm/delete_test.go`, fixtures `WWINqb/*`, `tGMBJ/*`, `V5N4be/*`, `J7Gthc/*`

**Interfaces:**
- Produces: `func (c *Client) Delete(ctx, kind string, nb string, ids []string) error` dispatching to the per-kind RPC; `func (c *Client) CancelChat(ctx, conv string) error` (`XgrPMd`).
- [ ] **Step 1–5:** tests per kind with pinned shapes, FAIL → implement → PASS, commit `feat(nlm): delete and cancel`.

### Task 6: MCP tools for P3

**Files:** Create `internal/mcpserver/tools_studio.go`, `tools_notes.go`, `tools_share.go`, `tools_labels.go`, `tools_account.go`, `tools_delete.go`, `tools_batch.go`; Modify `tools_info.go`, `tools_notebooks.go`; Test `internal/mcpserver/p3_tools_test.go`

**Interfaces:**
- Produces the remaining tools per feature-matrix (rows 1–7, 20–33) with annotations from that table. `delete`/`share_update` use elicitation when the client supports it, else require `confirm`. `chat_ask_many` registered in group `batch`, `max_parallel` default 3 via `errgroup.SetLimit`. Every mutating tool invalidates the affected cache prefixes.
- [ ] **Step 1: Tests:** `tools/list` count is 32 for `default`, 33 for `all`, 8 for `minimal`; `delete` preview then confirm; `studio_list wait_seconds:30` emits progress and returns when status flips to ready; `server_info include_usage:true` shows quota windows.
- [ ] **Step 2–4:** FAIL → implement → PASS.
- [ ] **Step 5: Commit** `feat(mcpserver): complete tool surface`.
