# KAFKA2306/nlm

- URL: https://github.com/KAFKA2306/nlm
- Language / runtime: Go 1.24.0 (`go.mod:3`), CLI + stdio MCP server code (not wired). **Module path is still `github.com/tmc/nlm`** (`go.mod:1`).
- License: MIT, "Copyright (c) Travis Cline" (`LICENSE:1-3`) — inherited from tmc/nlm.
- Stars / last commit: 1 star (per upstreams.md); HEAD `a2e47ba` 2026-10-09; 365 commits, of which 149 are by Travis Cline (the upstream), the rest by the fork owner (mostly merge commits of glossary/podcast branches).
- Versions reviewed: no tags (`git tag` empty). Fork point = tmc/nlm as of 2026-03-09 (last upstream commit `297ebc5` "fix audio-create to use direct RPC (AHyHrd)" and `8a647c1` "fix chat request format ..."). That is before tmc's v0.1.0 (2026-07-31).

## What this repo really is (contradicts research/upstreams.md)

research/upstreams.md describes it as "RPC / batchexecute (own implementation)". It is **not** an independent implementation: it is an older **fork of tmc/nlm** (same `internal/batchexecute`, `internal/beprotojson`, `internal/httprr`, `gen/`, `proto`, same go-sdk dependency). `internal/batchexecute/*.go` is a smaller, earlier version of tmc's (796 lines in `batchexecute.go` vs 1054; `chunked.go` 441 vs 357; no `wrbfr.go`, `wire.go`, `unescape.go`). The fork's own additions are unrelated to NotebookLM protocol work:

- `cmd/podcast-backfill/` (RSS merge, Archive.org IAS3 upload, MP4 duration without external tools) — a podcast publishing pipeline built on `audio-list`/`audio-download`.
- `cmd/podcast-ontology/`, `data/glossary/*.yaml`, `cmd/nlm/glossary.go` — a personal technical glossary.
- Security CI: Gitleaks scan, an "actions-safety" ratchet (`.github/workflows/secret-scan-gitleaks.yml`, `actions-safety.yml`).
- `docs/auth-troubleshooting.md` and `AGENTS.md` (agent execution contract).

So there is **nothing new to learn about the wire protocol here beyond tmc/nlm**; read the tmc note instead. What is useful: the MCP wiring pattern at go-sdk v1.2.0, the stdio smoke test, and the auth failure report.

## Access method

Same as tmc/nlm (batchexecute + gRPC-Web chat). See `research/notes/tmc__nlm.md`. The fork owner switched the client to **`notebook.google.com`** (host, `origin`, `referer`) in commit `8e936ac` on 2026-10-09 ("commit pending nlm workspace changes"), i.e. independently of tmc's own change (`1ef43ba`, 2026-07-27) (`internal/notebooklm/rpc/rpc.go:129-136`; `git log -S`). Two unrelated users hitting the same host migration is solid evidence that `notebooklm.google.com` stopped working for freshly harvested credentials. Defaults still hard-code a stale `bl=boq_labs-tailwind-frontend_20260210.19_p0` and the same fake `f.sid` (`rpc.go:120-126`), and `auth.go:433` probes `GET https://notebook.google.com/gen_notebook/notebook` to count notebooks (an endpoint not seen elsewhere; optional `NLM_GOOG_API_KEY`) [unverified purpose].

## RPC catalog

No independent catalog. Superset is `research/notes/tmc__nlm.md`. This fork's `gen/method/` has 53 encoder files vs tmc's 117 (no research, labels, rich-note encoders), so it covers fewer RPCs than tmc.

## Streaming/chat

Same as tmc at the 2026-03 fork point: `client.GenerateFreeFormStreamed(notebookID, prompt, nil)` returning a single `Chunk` string (`cmd/nlm/mcp.go:351-363`). No phase/citation/follow-up handling, no incremental streaming, nil source list means "server chooses". This is the pre-`grpc_arg_format` state; the later tmc streaming parser (thinking/answer phases, cumulative diff, citations) is absent.

## Auth lifecycle

- Credentials in `~/.nlm/env`, mode 0600 (`README.md` "Current boundaries"; `AGENTS.md` invariants).
- `nlm auth login` (profile copy + CDP), `nlm auth -c ws://localhost:9222` (attach to a logged-in Chrome), `NLM_BROWSER_PROFILE`, `-p "<profile>"` (`docs/auth-troubleshooting.md`).
- Platform files present: `internal/auth/chrome_darwin.go`, `chrome_linux.go`, `chrome_windows.go`, `safari_darwin.go`, `safari_other.go`, `browser.go`, `refresh.go` (Signaler `refreshCreds` as in tmc, `internal/auth/refresh.go:35-139`).
- **Operational findings recorded by the fork owner on 2026-10-09** (`docs/auth-troubleshooting.md`):
  - Copying the Chrome profile and launching on the copy fails with repeated `token not found or invalid` because "the session cannot be decrypted in the copied profile". The recommended alternative is CDP attach to a logged-in Chrome. This independently confirms that tmc's old copy-profile approach (stub `Local State`, `auth.go:704`) does not survive current Chrome cookie encryption.
  - `DEFAULT` profile often lacks `notebook.google.com` cookies; the profile list marks the profiles that have them. Good UX detail: show which profile contains the target domain's cookies (tmc `scanProfilesForDomain`, `internal/auth/auth.go:291-338`).
  - A direct read of Chrome's cookie DB was blocked by the agent permission system; "login is done by a human and registered in a credentials manager".
  - An unrelated chromedp log line `could not unmarshal event: unknown PrivateNetworkRequestPolicy value` appears with newer Chrome; it is benign (chromedp/cdproto protocol drift).
  - Unauthenticated alternative: only the NotebookLM Enterprise API (alpha) audio-overview endpoint was confirmed to work without cookies; list API unconfirmed (cites https://docs.cloud.google.com/gemini/enterprise/notebooklm-enterprise/docs/api-audio-overview).

## Reliability

Same retry/error code as tmc's earlier version (`internal/batchexecute/errors.go` 518 lines, `retry_test.go`). No new behavior.

## Performance notes

None.

## MCP tools

State: **implemented but not registered**. README and AGENTS.md say `nlm mcp` "is not a supported command" because `mcp` is not in the dispatcher (`README.md` "Current boundaries"; `AGENTS.md` invariants: "do not decide MCP is supported just from the existence of `cmd/nlm/mcp.go`"). The in-tree `cmd/nlm/mcp_test.go` builds the binary and talks JSON-RPC over stdio (initialize -> tools/list -> tools/call `list_notebooks`) (`mcp_test.go:11-170`). It is the only process-level MCP smoke test among the Go repos I read; the idea is reusable (we should also use go-sdk's in-memory transport, as tmc does).

SDK: **`github.com/modelcontextprotocol/go-sdk v1.2.0`** (`go.mod:11`); pulls `google/jsonschema-go v0.3.0`, `yosida95/uritemplate/v3`, `golang.org/x/oauth2` indirectly (same as tmc). API used: `mcp.NewServer(&mcp.Implementation{Name:"nlm", Version:"0.1.0"}, nil)`, `mcp.AddTool(s, &mcp.Tool{Name, Description}, func(ctx, req *mcp.CallToolRequest, params T) (*mcp.CallToolResult, struct{}, error))`, `s.Run(ctx, &mcp.StdioTransport{})` (`cmd/nlm/mcp.go:15-33`). Errors returned as `&mcp.CallToolResult{IsError:true, Content: TextContent{"Error: "+msg}}` with a nil Go error (`:416-425`). Typed input structs have **no `jsonschema` descriptions** (e.g. `ListSourcesParams{NotebookID string \`json:"notebook_id"\`}`), unlike tmc's later `internal/nlmmcp`.

Tools (30 registered): `list_notebooks, list_sources, list_notes, create_note, add_source_text, delete_note, list_artifacts, create_audio_overview, get_audio_overview, rename_artifact, share_audio, create_notebook, delete_notebook, delete_source, add_source_url, generate_chat` + 14 generated by a loop `generate_{summarize, briefing_doc, faq, study_guide, rephrase, expand, critique, brainstorm, verify, explain, outline, mindmap, timeline, toc}` which call `ActOnSources(notebookID, action, sourceIDs)` (RPC `yyryJe`) and return only "Successfully triggered %s generation. Check your notes for the result." (`mcp.go:372-403`). These 14 are the ActOnSources "transform" actions from the original CLI (`summarize`, `briefing_doc`, ...). tmc dropped them from its MCP server and from the CLI ("deprecated_generation_commands_removed.txt"), so treat them as legacy surface. Loop-variable shadowing `toolName := toolName` is present (Go < 1.22 habit).

## Tests and quality

- CI `ci.yml`; Gitleaks; actions pinning ratchet.
- Inherited testscript suite (`cmd/nlm/testdata/*.txt`) and `internal/batchexecute` tests with `testdata/list_notebooks.txt`.
- Agent-oriented repo discipline (AGENTS.md "Priority of evidence": runtime results > tests > implementation > build files > README > guess; mark `UNVERIFIED`).

## Versions

No tags. 2026-09-15 podcast backfill commands; 2026-10-07/08 security CI; 2026-10-09 auth troubleshooting, glossary merges. See `git log --no-merges`.

## Adopt

1. **Document the auth failure modes the way `docs/auth-troubleshooting.md` does** (symptom -> cause -> action table; "profile list shows which profile has the target-domain cookies"; "CDP attach is the fallback when profile copy fails"). Same structure for our `doctor`/`auth status` command.
2. **Process-level MCP smoke test**: build binary, send `initialize`, `tools/list`, one `tools/call`, assert expected tool names (`cmd/nlm/mcp_test.go:11-170`). Keep alongside go-sdk in-memory tests.
3. **"Do not claim a feature is supported because code exists"** rule (AGENTS.md invariant); mirror in our README/matrix by requiring a test per tool.
4. **Gitleaks + actions-pinning CI** from day one (`.github/workflows/secret-scan-gitleaks.yml`, `actions-safety.yml`), since we handle cookies.
5. Enterprise API as a no-cookie fallback for audio overviews (see nblm-rs note) — the fork owner reached the same conclusion.

## Avoid

1. Forking and keeping `module github.com/tmc/nlm`: import-path collisions, ambiguous provenance.
2. Shipping an MCP implementation that is not registered / not tested through the dispatcher.
3. MCP tools that return "triggered, check your notes" instead of the result id/URL/state (`mcp.go:400-403`); give agents a handle to poll.
4. Untyped tool param descriptions (no `jsonschema` tags) — agents need field docs.
5. Using `~/.nlm/env` plaintext credential file; use OS keystore or at least encrypt at rest and document `auth clear`.

## Evidence

Read: `go.mod`, `README.md`, `AGENTS.md`, `docs/auth-troubleshooting.md`, `cmd/nlm/mcp.go` (full), `cmd/nlm/mcp_test.go` (1-170), `internal/auth/refresh.go` (head), file list, `git log --no-merges`, `diff` of the first 80 lines of `internal/batchexecute/batchexecute.go` against tmc (differs). Nothing executed.
