# stratosphereips/strato-mcp-notebooklm

- URL: https://github.com/stratosphereips/strato-mcp-notebooklm
- Language / runtime: Python >=3.10; official `mcp>=1.6.0` SDK (`mcp.server.fastmcp.FastMCP`); `notebooklm-py>=0.3.4` (unbounded upper); python-dotenv; Docker image (`pyproject.toml`, `server.py:10`)
- License: Apache-2.0 (`LICENSE:1-3` Apache License 2.0; `pyproject.toml` license = "Apache-2.0"). GitHub reports NOASSERTION; the file is the standard Apache text. Code may be reused with attribution and NOTICE handling.
- Stars / last commit: 2 stars (research/upstreams.md); HEAD `cb5be81` 2026-04-20
- Versions reviewed: tags v0.1.0 to v0.7.0 (all 2026-04-19/20, `git tag` + log); `pyproject.toml` still says `0.1.0` (never bumped). 22 commits in 2 days (`git log`).

## Access method

Internal `batchexecute` RPC via `notebooklm-py` only (`server.py:90, 97` `NotebookLMClient.from_storage(timeout=...)`; `notebooklm/client.py` wraps `self._client.*`). No RPC ids or wire code of its own. The README's "37 RPC methods" means the public method surface of notebooklm-py (README line 3), not distinct RPC ids; the repository never enumerates them [unverified count]. RPC details: `research/notes/teng-lin__notebooklm-py.md`. Note it was written against notebooklm-py 0.3.x: `research.start()` / `poll()` results are accessed as dicts (`start_result.get("task_id")`, `poll_result.get("status")`, `client.py:933, 940-945`), which newer typed returns may break [unverified against 0.7/0.8].

Code size: `notebooklm/client.py` 1077 lines, `types.py` 207, `resolver.py` 164, `errors.py` 96, five tool files totalling about 700 lines, 101 tests (`wc -l`, commit `32e539e`).

## RPC catalog
None. See teng-lin note.

## Streaming/chat

`ask_notebook` -> `chat.ask(nb_id, question, source_ids=resolved_scope, conversation_id=conversation)` awaiting the whole answer (`client.py:281-330`). Citations enriched client-side: for each reference it calls `sources.get(nb_id, ref.source_id)` to replace ids with titles (N extra RPCs per ask, one per reference, `client.py:306-320`). Tool returns `answer`, `conversation_id`, `follow_up_hint`; references only when `include_references=true`, deduplicated by source id and capped at 10 (`tools/chat.py:70-86`), with commit `95bf8fd` making references opt-in "for faster, lighter responses". `configure_chat` maps modes default, learning_guide, concise, detailed, custom (needs `custom_prompt`) onto the chat-settings RPC (`tools/chat.py:90-123`).

## Auth lifecycle

- notebooklm-py cookies. Local: `~/.notebooklm/storage_state.json` from `notebooklm login`; Docker: env `NOTEBOOKLM_AUTH_JSON` with the file contents (`.env`, chmod 600) (README "Authentication", `config.py:35-58`).
- Documented expiry: "Tokens expire every 2-4 weeks - re-run notebooklm login" (README). No refresh logic, no keepalive.
- Client lifetime fixed by two commits: `dcdca73` use a FastMCP `lifespan` so one `NotebookLMClient` stays open for the server lifetime, `1470b0f` "await from_storage() before using client as context manager" (`server.py:87-109`). Thread-safe singleton accessor `_get_client()` with a lock (`server.py:72-84`).
- Auth errors are mapped to agent guidance: "Run 'notebooklm login' or update NOTEBOOKLM_AUTH_JSON. Cookies expire every 2-4 weeks." (`notebooklm/errors.py:34-43` approx; text also in `README.md`).

## MCP tools: the 37 -> 13 compression

13 tools in 5 "agent stories" (`README.md`, `server.py:22-52`). Every `notebook`, `source`, `scope`, `sources` and `artifact` parameter accepts an id OR a fuzzy title (`resolver.py`). Annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) set on every tool (`tools/*.py` `_tool_annotations`).

| Story | Tool | Params | Composes (notebooklm-py calls) | file:line |
|---|---|---|---|---|
| 1 Find and open | `list_notebooks` | none | `notebooks.list` | `tools/discover.py:258`, `client.py:133` |
| | `open_notebook` | notebook | `notebooks.get` then concurrently `notebooks.get_description` + `sources.list` via `asyncio.gather(return_exceptions=True)`; top 5 suggested topics; source digests | `discover.py:278`, `client.py:137-165` |
| | `create_notebook` | title, sources? | `notebooks.create` + `add_sources` + same overview as open | `discover.py:303` |
| 2 Understand a source | `inspect_source` | notebook, source, detail guide/fulltext | guide: `sources.get` + `get_guide` + `check_freshness` (URL/Drive); fulltext: `get_fulltext` with `char_count` | `tools/inspect.py:153`, `client.py:188-232` |
| | `manage_source` | notebook, source, action rename/refresh/remove, new_title? | `sources.rename/refresh/delete` + resolver cache invalidation | `inspect.py:194`, `client.py:235-278` |
| 3 Ask | `ask_notebook` | notebook, question, scope[]?, conversation?, include_references=false | `chat.ask` + per-reference `sources.get` | `chat.py:30`, `client.py:281` |
| | `configure_chat` | notebook, mode, custom_prompt? | chat configure | `chat.py:90` |
| 4 Generate | `generate_artifact` | notebook, kind (audio, video, report, quiz, flashcards, mind_map, infographic, slide_deck, data_table), sources[]?, instructions, options (JSON string) | resolves sources then dispatches to `artifacts.generate_*` by kind; returns task_id immediately ("fire-and-return"); mind_map returns completed | `tools/generate.py:35`, `client.py:384-440` |
| | `collect_artifact` | notebook, artifact (id, task_id or fuzzy title), action status/download/export, output_path?, export_target docs/sheets | `artifacts.list` to resolve, then `poll_status` / `download_*` by kind / `export_report` or `export_data_table`; adds `estimated_wait` text per kind | `generate.py:104`, `client.py:663-830` |
| | `list_artifacts` | notebook, kind?, status? | `artifacts.list` | `generate.py:158` |
| 5 Enrich and share | `add_sources` | notebook, sources[] of dicts with one of url, text_title+text_content, file_path, drive_file_id; wait=True | per item: `add_url` (auto web vs YouTube), `add_text`, `add_file`, `add_drive`; sequential; failures per item | `tools/enrich.py:224`, `client.py:852-920` |
| | `research_web` | notebook, query, mode fast/deep, import_results=False | `research.start`, then polls `research.poll` with interval 2.0 s growing x1.5 to 10 s, timeout 60 s fast / 300 s deep, optional `import_sources` | `enrich.py:271`, `client.py:921-992` |
| | `share_notebook` | notebook, action status/set_public/set_private/add_viewer/add_editor/remove_user, email? | `sharing.*` | `enrich.py:321`, `client.py:994-1050` |

Counts per CRUD/verb bundle: sharing 6 operations -> 1 tool (action discriminator), source rename/refresh/remove -> 1, five source-add paths -> 1, artifact 10 generators + 10 downloaders + export + poll -> 3.

## Design lessons (the reason this repo is in the batch)

1. Shape tools by user story, not by endpoint. README "Design Principles": workflow curation over API completeness; fuzzy resolution; human strings instead of integer enums (`"deep_dive"` not `1`); progressive disclosure (`detail="guide"` first, `"fulltext"` second); fire-and-return for slow jobs; actionable errors with "Did you mean" (`README.md` Design Principles).
2. Action discriminator for rare mutations: `manage_source`, `share_notebook`, `collect_artifact` carry `action` rather than 3-6 tools. Keeps tool count at 13 and keeps `destructiveHint` coarse (a true safety weakness: `manage_source` and `share_notebook` are flagged destructive even for read-only `status`, and `collect_artifact` is flagged non-destructive but can write files, see Avoid).
3. Composition inside the server to cut round trips and tokens: `open_notebook` fans out summary + sources in parallel and returns a digest (`client.py:137-165`).
4. Fuzzy resolver with caches: notebooks cached for the session, sources per notebook with TTL 60 s, invalidated on mutation; exact id, exact title, then substring, with `ActionableError(message, guidance, suggestions)` on ambiguity or miss (`resolver.py:1-164`, `errors.py:19-37`). Error response shape `{status:"error", error, guidance, suggestions}` (`errors.py:30-37`).
5. Typed digests instead of raw rows: `NotebookSummary`, `SourceDigest`, `ArtifactDigest`, etc. (`types.py`).
6. Dynamic instructions: `LOCAL_FILES_PATH` set -> a FILE UPLOAD block appended to the server instructions describing the `/files/` mount (`server.py:55-69`, `config.py`).
7. Counterweight from teng-lin ADR-0025: after reaching 38 tools they conclude a lean surface needs client-side deferred loading; strato shows a 13-tool surface is reachable only by dropping capabilities (no notes, labels, search, sharing granularity, chat history, quota, source copy). Our target: about 15-20 tools with all features reachable through `kind`/`action` discriminators and typed enums in the JSON schema (strato passes `options` as a JSON-encoded string, which hides the schema from the model; avoid).

## Reliability

Defaults from `config.py:14-58`: HTTP timeout 30 s, source wait 120 s, fast research 60 s, deep research 300 s (env `NOTEBOOKLM_TIMEOUT`, `NOTEBOOKLM_SOURCE_WAIT_TIMEOUT`, `NOTEBOOKLM_RESEARCH_FAST_TIMEOUT`, `NOTEBOOKLM_RESEARCH_DEEP_TIMEOUT`). Per-wait estimates surfaced to the model: audio about 5 minutes, video about 30 minutes, other about 2 minutes (`server.py:40-43`, `notebooklm/client.py:71`). Rate-limit error text suggests "Free tier allows ~50 queries/day" (`notebooklm/errors.py:57`). No retries, no backoff in this layer. Tests 101 across config, errors, resolver, client, tools with mocks.

## Versions

Tags (git tag, `git log -1` subjects): v0.1.0 2026-04-19 CI chain; v0.2.0 2026-04-19 Docker build fix; v0.3.0 2026-04-19 `LOCAL_FILES_PATH` mount; v0.4.0 2026-04-20 housekeeping merge (#2, bot-synced docs); v0.5.0 2026-04-20 README naming; v0.6.0 2026-04-20 local file path docs; v0.7.0 2026-04-20 CITATION.cff. Notable commits: `9d45e77` initial implementation; `dcdca73`/`1470b0f` lifespan fixes; `95bf8fd` include_references toggle; `f2f78de` structured agent errors from `ActionableError`. Dockerfile + docker-compose, `docker-entrypoint.sh`, `Dockerfile`.

## Adopt

1. Story-shaped tool set and the principle list above (README "Agent Stories", "Design Principles").
2. Resolver: id -> exact title -> substring, with suggestions list and cache TTL 60 s + explicit invalidation (`resolver.py`). Add prefix and Levenshtein near-miss like teng-lin's `Did you mean` (title + id).
3. Structured error envelope `{status, error, guidance, suggestions}` returned as tool result, not exception (`errors.py:30-37`).
4. `open_notebook`-style composite read (summary + topics + source digests in one call, with `return_exceptions` so one failed sub-call degrades gracefully) (`client.py:137-165`).
5. Tool annotations on every tool (`tools/chat.py:18-24`); decide annotations per action, not per tool.
6. `estimated_wait` hints and fire-and-return with a separate collect step (`client.py:663-780`).
7. Progressive-disclosure parameters (`detail`, `include_references`) to keep token use low (`inspect.py`, `chat.py:70`).
8. Docker-first distribution with `serve` entrypoint and read-only `/files` mount for uploads (`README.md`, `docker-compose.yml`).

## Avoid

1. `options` as a JSON-encoded string parameter (`tools/generate.py:40, 79`): model cannot see valid fields in the schema; use typed per-kind objects or enums.
2. Citation enrichment with one `sources.get` per reference (`client.py:306-320`): cache the source list once per ask.
3. Overloaded discriminator tools with mixed side effects under one annotation (`collect_artifact` can write files but is `destructive=False`; `share_notebook` is `destructive=True` even for `status`) (`generate.py:102, enrich.py:319`).
4. Sequential source adds (`client.py:870-890`): use the batch/async add RPCs (`izAoDd` with many specs, `X1snv`).
5. Silent swallowing of import failures (`except Exception: pass` around `import_sources`, `client.py:975-977`).
6. Dict-style access to library results and an unbounded `notebooklm-py>=0.3.4` pin (`pyproject.toml`, `client.py:933-945`).
7. `download` writes into the container filesystem and loses it on exit (their own warning, `generate.py:115-117`); return a resource link or write to an explicit mounted path.
8. Version fields never bumped (`pyproject.toml` 0.1.0 vs tag v0.7.0).

## Evidence

Read: `README.md` (to line 220), `server.py` (full), `tools/chat.py`, `inspect.py`, `discover.py`, `enrich.py`, `generate.py` (full), `notebooklm/client.py` (lines 133-330, 384-440, 663-1000), `resolver.py` (full), `errors.py` (full), `config.py` (full), `pyproject.toml`, `LICENSE` head. Not executed.
