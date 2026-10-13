# jacob-bd/notebooklm-cli (ARCHIVED predecessor of jacob-bd/gemini-notebook-mcp-cli)

Scope of this note: only what differs from the successor and is worth keeping. Everything else: see research/notes/jacob-bd__gemini-notebook-mcp-cli.md.

- URL: https://github.com/jacob-bd/notebooklm-cli (PyPI `notebooklm-cli` 0.1.12)
- Language / runtime: Python, typer, rich, httpx, websocket-client (CDP). Package `src/nlm/` (about 7.8k lines incl. docs; core/client.py 2178 lines, one monolithic client; the successor split it into mixins).
- License: MIT, (c) 2024 Jacob (LICENSE:1-3).
- Stars / last commit: 256 stars; HEAD `8852560` 2026-06-08 (README banner only). 54 commits. Archived.
- Versions: tags v0.1.0, v0.1.1, v0.1.2 (2026-01-10), v0.1.4, v0.1.5 (2026-01-15), v0.1.7 (2026-01-16), v0.1.8, v0.1.9, v0.1.10 (2026-01-17), v0.1.11, v0.1.12 (2026-01-18). Deprecated 2026-01-29: README banner "merged into NotebookLM MCP CLI" (commits aca813b, c5f503d). No MCP server in this repo (MCP_REFERENCE.md is a 7-line pointer to the other project).

## Access method / RPC catalog
Same batchexecute transport and the same CDP login as the successor, at an earlier stage. RPC ids in `src/nlm/core/client.py:52-84`: wXbhsf, rLM1Ne, CCqFvf, s0tc2d (rename and chat config), WWINqb, izAoDd (URL/text/Drive), hizoJc, yR9Yof, FLmJqe, tGMBJ, VfAZjd, tr032e, Ljjv0c, QA9ei, e3bVqc, LBwxtb, R7cb6c, gArtLc, V5N4be, yyryJe, CYK0Xb, cFji9, AH0mwd. Every one is also in the successor; the successor adds izAoDd's replacement ozz5Z, o4cbdc (file upload), hPTbtc/khqZz/J7Gthc (chat history), rc3d8d, Rytqqe, KmcKPe, v9rmvd, agX4Bc/le8sX/GyzE7e (labels, collections), QDyure/JFMDGd (sharing), Krh3pd (export), EylDcb (usage), cYAfTb (note update), b7Wfje (source rename), rG2vCb (Enterprise). No id here is missing in the successor. Hard-coded `bl=boq_labs-tailwind-frontend_20260108.06_p0` (docs/TECHNICAL_DEEP_DIVE.md "Build Label").

## Streaming/chat, Auth lifecycle, MCP tools
Predecessor is CLI only (no MCP tools). Auth: CDP cookie extraction with port scan 9222-9231 (CHANGELOG 0.1.6), Chrome closed after login so headless recovery works (commit 7c916c2), three-layer recovery (token refresh in client, reload from disk, headless Chrome: src/nlm/core/auth_refresh.py:1-60, with `has_fresher_tokens_on_disk` 5-minute freshness rule). Chat: `nlm notebook query` and an interactive REPL (`nlm chat start`, src/nlm/cli/repl.py, 204 lines). All carried into the successor (cli/commands/repl.py, core/auth.py).

## Reliability / Performance / Tests
Not distinct. `tests/run_e2e_tests.py` and `docs/CLI_TEST_PLAN.md` (708 lines) are a manual e2e test plan; the successor's `docs/MCP_CLI_TEST_PLAN.md` (1394 lines) supersedes it.

## Worth keeping (differences)
1. `docs/TECHNICAL_DEEP_DIVE.md` (133 lines) is the only prose write-up of three protocol quirks; verified against the successor's code:
   - Mind map deletion needs a two-step sequence: `AH0mwd` with `[nb,null,[id],[2]]`, then `cFji9` with `[nb,null,[seconds,micros],[2]]` where the timestamp is that artifact's creation time read from `cFji9` list (`entry[1][2][2]`), else "ghost" entries remain; deleted entries come back as tombstones `[uuid,null,2]` (TECHNICAL_DEEP_DIVE.md section 2; client.py:1607-1640). Successor kept the same logic (successor core/studio.py:667-701). The Go server must implement both steps.
   - Research status: the doc says 2 = IN_PROGRESS and 6 = COMPLETED (section 1), but BOTH repos' code treat status 2 and 6 as completed and 1 as in progress (predecessor client.py:1735, 1900; successor core/research.py status mapping). The doc is wrong; trust the code (still [unverified live]).
   - `bl` staleness: when `bl` is weeks old, mutations (research start, artifact delete) can fail silently or return HTTP 400 (section 3). Reason to scrape `bl` from the page at runtime rather than hard-code.
   - Batch import beats per-source adds for research results; source forms: web `[null,null,[url,title],...,2]`, Drive `[[doc_id,mime,1,title],...,2]` (section 3).
2. Chat citation description (section 4): answer metadata at `parsed[0]` (text `[0]`, source-id master list `[2]`, annotations `[4]`), citation list at `parsed[1]` where entry `i` is citation `[i+1]` and `entry[5][0][0][0]` is the source UUID, offsets at `parsed[2]`; one citation entry per cited PASSAGE, so `[18]` can appear with 9 sources. Note this describes a different nesting than the successor's parser, which reads passages from `first_elem[4][3]` and the source id at `passage[1][5][0][0][0]` (successor core/conversation.py:979-1053). Both may be views of the same payload at different times; a Go parser should be tolerant and verify against a live capture [unverified].
3. `nlm source list --drive --skip-freshness` (0.1.7): skip the N+1 freshness RPCs; successor adds `skip_freshness` and parallelises freshness checks (successor CHANGELOG 0.8.0 batch notes).
4. Output formatter design: table, JSON, compact ("quiet": IDs only) with auto-detection `detect_output_format` (src/nlm/output/formatters.py:12-50, 362-415); useful for a `--json/--quiet` CLI mode if we ship a CLI.
5. `nlm --ai` prints agent-consumable docs plus 12 rules for AI assistants (confirm before delete, check aliases first, do not launch the REPL, prefer `--json`) (CHANGELOG 0.1.11; src/nlm/ai_docs.py). The successor ships an installable skill instead.

## Adopt
- The two-step mind map delete and tombstone filtering (see 1).
- Runtime `bl` discovery (see 1).
- Write protocol quirks into our own docs/ as the predecessor did.

## Avoid
- Treating `docs/TECHNICAL_DEEP_DIVE.md` as authoritative for status codes (it contradicts the code).
- Monolithic 2000-line client file (core/client.py).
- Nothing else of value beyond what the successor has; do not track this repo further.

## Evidence
Read: docs/TECHNICAL_DEEP_DIVE.md (all), CHANGELOG.md (0.1.5-0.1.12), README.md (head), core/client.py (52-84, 1560-1640, 1700-1740, 1900), core/auth_refresh.py (1-60), output/formatters.py (function list), file lists, git log, tags.
