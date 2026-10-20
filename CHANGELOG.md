# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow [Semantic Versioning](https://semver.org/) once released.

## [Unreleased]

### Added

- Sign in from an environment variable, for containers that cannot run `login` (Docker MCP Toolkit, `docker run`): `NOTEBOOKLM_COOKIES` holds a cookie export (Netscape, JSON or `Cookie:` header, as `login --import`), or `NOTEBOOKLM_COOKIES_FILE` names a file holding one (a Docker secret). The same required-cookie check applies, errors never show values, the cookies stay in memory (never saved, rotations included), the variables are removed from the environment after reading, and a set variable wins over a saved profile. `status` and `doctor` report the source. See `docs/auth.md`.

### Changed

- `sourcebolt status --help` prints usage and exits 0, and `status` with any argument is a usage error (exit 2); before, it ignored its arguments and printed the full status.
- Release workflow: npm packages are staged with `npm stage publish`, run with a pinned npm CLI 11.21.0. Every version still waits for the owner's 2FA approval on npmjs.com. See `docs/registry.md`.

### Security

- npm publishing tries trusted publishing (GitHub OIDC, environment `npm`) before the `NPM_TOKEN` secret. Once each package has a trusted publisher, the long-lived token can be deleted and token publishing turned off.

## [0.3.1] - 2026-10-21

The first published 0.3 release. It has every change listed under 0.3.0: the v0.3.0 tag exists, but its release run failed before anything was published.

### Fixed

- Release packaging: the Homebrew, Scoop and winget `token` fields used `envOrDefault`, which GoReleaser does not allow there (tokens accept exactly `{{ .Env.NAME }}`). Snapshot and dry runs never expand tokens, so only a real tag failed. `go test ./tools/distcheck` now checks the form.

## [0.3.0] - 2026-10-21 (not published)

Installing for people who never used a terminal: one command, a guided setup that connects your AI apps, and sign-in in a normal browser window.

### Added

- One-line installers: `irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex` (Windows) and `curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh` (macOS, Linux). They check the SHA-256 of the download (and the cosign signature when cosign is installed), install per user with no admin rights, add the folder to PATH, offer to run `sourcebolt setup`, upgrade in place and roll back a broken upgrade. `--uninstall` / `-Uninstall` removes exactly what they added; `--purge` / `-Purge` also deletes your data after asking. See `docs/install.md`.
- `sourcebolt setup`: finds Claude Desktop, Claude Code, Cursor, VS Code, Windsurf and Gemini CLI, backs up each config, adds one `sourcebolt` entry, signs you in and runs the doctor checks. `--client <id>`, `--dry-run`, `--undo`, `--yes`, `--no-login`. The command it writes is always a stable path: run from an npx cache or a temp folder, it uses the installed copy, else `npx -y sourcebolt serve`, else it refuses.
- `sourcebolt login` with no flags opens a plain browser window (only a debugging port, no automation flags) and you sign in yourself, so Google does not block it.
- Homebrew cask (`candidloom-hub/homebrew-tap`) and Scoop manifest (`candidloom-hub/scoop-bucket`) published with each release. winget manifests are generated but submitted by hand.
- Beginner guides: install by asking your AI (`docs/install-with-ai.md`), step-by-step guides for 7 apps (`docs/guides/`), and an "Install in 2 minutes" block in the README.

### Changed

- `--help` on any subcommand exits 0 (it exited 2, a usage error).
- AUTH_REQUIRED guidance names `sourcebolt login` first.

### Fixed

- A call cancelled while its answer was being read reported AUTH_REQUIRED (an empty, cut-short body) instead of the cancel, which told the user to sign in again.
- `setup` never sends a value containing `%` through a `.cmd`/`.bat` CLI (`code.cmd`, an npm `claude.cmd`): cmd.exe expanded it, and the VS Code entry failed. It edits the config file instead.

- On a Mac whose keychain is locked or missing with nobody to unlock it (ssh, CI, a headless Mac), `setup`, `doctor`, `login` and `serve` waited forever on the keychain. Keyring calls now give up after 30 s; a new profile's key goes to the 0600 key file, and the doctor check reports the keyring as unavailable.

### Security

- Release workflows pin third-party actions to commit SHAs.

## [0.2.0] - 2026-10-20

Every Studio type, chat settings, saving answers as notes, deletes for sources, notes and chat history, a large performance pass, new user docs, and new distribution channels. Shapes are pinned by live captures #2, #3 and #3b (2026-10-20); see `research/rpc-catalog.md` (rows marked "live").

### Added

- `sourcebolt doctor --dom-check <notebook>` asks one question through the browser (DOM) fallback, so you can confirm it works on your machine before you need it.
- Distribution: a Claude Code plugin marketplace in the repo (`/plugin install sourcebolt --marketplace candidloom-hub/sourcebolt`), MCP Bundles (`sourcebolt-mcp-<os>-<arch>.mcpb`) attached to releases, VS Code and Cursor one-click install links (`docs/install-links.md`), `glama.json`, `gemini-extension.json`, and a registry-ready `server.json` (GHCR image only until npm and PyPI are published).
- User docs rewritten: getting started, per-client setup, troubleshooting by error message, FAQ, performance, "Why sourcebolt" and a sourced comparison page; plus a privacy page.
- `studio_generate` generates every Studio type: `audio`, `video`, `report`, `interactive_report`, `slide_deck`, `mind_map`, `quiz`, `flashcards`, `infographic` and `data_table`. Each kind has its own option object whose schema enums list only the values a live capture sent; an omitted option takes the web app's default. A value the web app offers but no capture sent returns `UNSUPPORTED_BY_TRANSPORT` naming the option, before the quota check. Proven values (default first): audio format `deep_dive`, `critique`, length `default`, `short`; video format `cinematic`, `short`; slide deck format `detailed_deck`, `presenter_slides`, length `default`, `short`; quiz quantity `standard`, `more`, difficulty `medium`, `easy`; flashcards quantity `standard`, `fewer`, difficulty `medium`, `hard`, images `include`, `text_only`; infographic orientation `landscape`, `portrait`, detail `standard`, `concise`, style `auto`, `sketch_note`. `instructions` is optional for audio, video, slide deck and quiz, required for report, mind map, flashcards and data table, and refused by infographic and interactive report (each as captured). `report` needs `report.title` and `report.description` plus `instructions` as the prompt; `suggest` surface `report` returns such triples. `language` accepts only the captured codes `en` (default) and `en_in`; cinematic video takes none. `video.generate_later` asks for deferred generation (the web app's "Generate later"; cinematic only; the artifact lists as pending). Sources default to every source of the notebook.
- `chat_ask` option `save_as_note`: saves the answer as a note the way the web app's "Save to note" does (the answer's passages and rich text go along; the server picks the title). Returns `note_id` and `note_title`, or `note_error` with the answer kept.
- `delete` kinds `source`, `note` and `chat_history` (a conversation's whole history; ids are conversation ids). Same preview and `confirm:true` gate as notebooks; refs are matched strictly against a fresh list of the notebook's items. The `chat_history` preview shows each conversation's created and updated times and, when its whole history can be read, the first question; only the newest 20 conversations are reachable (an older id gets `UNSUPPORTED_BY_TRANSPORT`, not `NOT_FOUND`).
- `source_read` mode `guide`: the source guide's summary as `text` plus `keywords`.
- `studio_list` decodes rows of every Studio type with their status (`pending` while queued, generating or deferred).

### Changed

- **Breaking:** `studio_list` reports flashcards, quiz and mind map artifacts as kinds `flashcards`, `quiz` and `mind_map` (all were `interactive`), and slide decks as `slide_deck` (was `slides`), the names `studio_generate` uses. Filters on the old names match nothing.
- **Breaking:** `studio_generate` flashcards default to the web app's defaults (standard, medium, with images), which capture #3 sent, instead of fewer, hard, text only (still available as options).
- **Breaking:** `chat_configure` accepts mode `default`, `learning_guide` or `custom` and response_length `default`, `longer` or `shorter`, the values captures #3 and #3b saved; mode `default` with response_length `default` resets a notebook. Modes `concise` and `detailed` are removed.
- `suggest` surface `report` no longer returns `audience`: the number has no verified meaning.
- `suggest` surface `report` sends the live request (artifact options block and every source when none are given).
- `notebook_list` results carry `has_more`, true when notebooks after the returned page also match; page with `offset`.
- `notebook_list` returns 20 notebooks by default instead of 50. `total` still counts every match; pass `offset` for the next page or a larger `limit` (pages come from the 30 s cache, so paging makes no extra Google call). Each listed notebook is sent twice (structured content and its JSON text fallback) and decoded by the client, so the smaller page cuts the cached call's overhead by about half (B3, see `bench/RESULTS.md`).
- `note_save` creates a note in one call with its title and content, as the web app's "Copied text" flow does; updates send the live note-edit shape.
- Notebook rename, text load-source and artifact-read requests match the live web app (options block added, full `v9rmvd` form). Markdown load-source and chat suggestions were not captured and keep their earlier requests.
- `note_list` (and so `note_save` and the note delete) skips rows that are not notes, such as note-backed mind maps.
- `source_read` reports drift instead of returning empty text when an answer has no text blocks and no HTML.
- `source_add` lists the notebook before adding, so that a source missing from the add answer is only reported added when its row is new; a match with a source that already existed is pending.

### Performance

- Tool-call overhead (B3, 100 notebooks, 2-CPU CI, p50): uncached `notebook_list` 2.20 -> 0.76 ms, cached 0.99 -> 0.40 ms; decoding a 40 KB answer 508 -> 225 us with 9 instead of 93 allocations; cold start and idle memory lower (tool schemas are resolved on first use). See `bench/RESULTS.md`.
- On Linux the server turns off transparent huge pages for its own process (`prctl`), which cut idle RSS by about 2 MB on hosts whose THP mode is "always". The fallback browser inherits the setting.
- `serve` runs with GOGC=200 unless `GOGC` is set: fewer collections per tool call (cached `notebook_list` about 20% faster) at the cost of a somewhat larger heap under load.

### Fixed

- `login` replaces a profile whose encryption key is gone from the OS keyring (it could never be read again) instead of failing with a file-not-found error; other unreadable profiles get a `logout` hint.
- The cookie-import error names exactly the cookies sourcebolt checks (`SID`, `__Secure-1PSIDTS`, and `OSID` or `APISID`+`SAPISID`).

- `chat_configure` sent no options block; the web app sends it. Two changes on one notebook at once could lose one of them; the read and the write now run one at a time per notebook.
- `chat_ask` streams copy the answer's rich block only when `save_as_note` is set.
- `studio_list` with kind `slide_deck` matched nothing.
- `source_read` returned a source's text as one run-on line: text blocks sit one level deeper than the research catalog said.
- Chat citations: a marker lists passage indices, not a range (checked against both live captures). Answers citing a single passage had no citations, and answers whose markers cite several passages lost some. Exact duplicates are dropped and malformed indices skipped.
- `source_add`: an item missing from a successful add answer was reported failed although Google had added it (seen live with YouTube). It is now looked up in the notebook: found is added, not found is failed, a failed lookup is pending.

### Known limitations

- Replaces the 0.1.0 list. Not available yet: Studio option values no capture sent (audio formats brief and debate, long audio, video formats explainer and brief and their visual styles, quiz fewer and hard, flashcards more and easy, square infographics, detailed infographics, other infographic styles), languages other than `en` and `en_in`, deferred generation for anything but cinematic video, free text where a kind was captured without it (infographic, interactive report) or required where only captured with it (report, mind map, flashcards, data table), report templates by name (their texts live in the web app), and notes as Studio sources; artifact, label and collection deletes; Drive and file sources; source refresh; followup suggestions; label create and auto-label; research `discover`.
- Combinations are assumed independent: options proven in different captures (for example a cinematic video without "Generate later", or a quiz topic with the default quantity) are sent together although no capture sent that exact mix.
- `chat_configure` serializes its own changes per notebook; a change made at the same moment in the web app can still be overwritten.
- `studio_list` leaves media URLs empty: no finished video was captured, and the two slide-deck download URLs cannot be told apart (PDF or PPTX) in the sanitized capture.
- Saving a chat answer re-encodes the answer's rich text the way both captured saves did; rich structures neither capture had (tables, images in answers) are sent as the stream had them.
- Not capturable from the web UI: conversation-list paging (the UI keeps one thread per notebook and never pages, so only the newest 20 conversations are listed) and markdown load-source (`hizoJc` markdown mode; no UI path found, so the catalog's request is kept unverified).

## [0.1.0] - 2026-10-20

First public release: GitHub release binaries and GHCR images. npm, PyPI and MCP registry packages are not published yet.

### Added

- Core (P1): Go module, `batchexecute` request encoder and chunked response decoder with status, drift and null policies, RPC id registry with runtime overrides (`NOTEBOOKLM_RPC_IDS`), TTL cache with singleflight and stale-while-revalidate, notebook id/prefix/title resolver.
- Auth and storage: cookie import (Netscape, JSON, header), AES-256-GCM encrypted profile store with the key in the OS keyring (key file fallback), revision-checked saves, `login --import`, `login --browser`, `login --attach` (CDP), `logout`, `status`, `doctor`.
- CLI: `serve` (stdio, or loopback streamable HTTP with bearer token and Origin check), `call <tool> '<json>'`, tool profiles `default`, `minimal` and `all`.
- Transports: tuned HTTP/2 RPC transport, streamed chat transport with first-chunk, idle, total and size limits, in-page browser fetch transport on a dedicated profile, and a router that falls back from RPC to the headless browser (`--no-browser-fallback` / `NOTEBOOKLM_BROWSER_FALLBACK=0` to disable).
- 31 MCP tools: `server_info`; `notebook_list`, `notebook_open`, `notebook_create`, `notebook_update`; `source_add`, `source_list`, `source_read`, `source_update`; `chat_ask`, `chat_status`, `chat_history`, `chat_configure`, `chat_ask_many`, `suggest`; `research_start`, `research_status`, `research_import`; `note_list`, `note_save`; `share_status`, `share_update`; `label_list`, `label_update`; `studio_list`, `studio_generate` (quota check only); `account_settings`, `auth_login`, `auth_logout` (CLI hints only); `delete` (notebooks); `job_cancel`. Several options return `UNSUPPORTED_BY_TRANSPORT` until a live capture pins them; see [docs/tools.md](docs/tools.md).
- DOM fallback: when NotebookLM changes its chat or add-source RPC, sourcebolt drives the signed-in page instead (`browser-dom` transport). Selectors are overridable in `<config>/selectors.json`. Source-scoped asks, non-default threads and Drive sources are not covered.
- Progress notifications for long calls and a local job table for detached chat (`wait=false`).
- Safety: SSRF URL check, path allow-root check, log redaction, preview-before-delete and preview-before-share-widening.
- Performance: benchmark suite with a regression gate in CI (`bench/`); lazy wire view that cut notebook decode allocations about 111x.
- Packaging: GoReleaser config (archives, checksums, SBOMs, cosign signatures, Docker images including a `-browser` variant), npm launcher with per-platform packages, PyPI per-platform wheels. Nothing is published.
- Documentation: README, generated tool reference (`go run ./tools/gendocs > docs/tools.md`, checked for staleness by a test in CI), auth and security guides, agent `SKILL.md`, MCP registry `server.json`.
- Live capture tooling (`tools/capture`) and sanitized golden fixtures from a real NotebookLM session.

### Changed

- **Breaking:** `research_import` selects candidates by `urls`, `report: true` or `all`, matched against a fresh read; the index-based `select` input is removed. `research_status` no longer shows indices, and import results carry `url` instead of `index`.
- `share_update` tries both of its calls and reports each part. The status is `applied` or `partial`; it is an error only when nothing was applied.
- Error envelopes include `retry_after_seconds` when Google sends Retry-After.
- Docker base images are pinned by tag and digest, and Dependabot watches them.

### Performance

- Tool results are marshalled once. The output-schema check runs only in tests, and the advertised schemas are unchanged. Per `notebook_list` call (100 notebooks), on the 2-CPU CI runner: allocations −72%; p50 2.88 → 2.26 ms uncached and 1.67 → 1.00 ms cached.

### Fixed

- Chat answers are no longer cut off by a 30 s response-header timeout. Unary calls bound their wait to the first response byte.
- The browser fallback is skipped after a direct connection refusal. A refused proxy still allows it.
- The HTTP server's Origin check compares the parsed hostname; `http://localhost.evil.com` no longer passes.
- New profile names reject Windows device names, control characters, and leading or trailing dots and spaces. Existing profiles still load, delete and save.
- Chat stream errors after the question was sent are marked as sent.
- Asks that start a new chat thread run one at a time per notebook, so concurrent asks no longer pick up each other's thread.
- Session store keeps keys on transient keyring errors and locks and retries saves; fresh logins win over stale ones.
- No secrets in transport errors; tool errors are redacted.
- Non-idempotent calls are not resent after the request reached the server.
- Source URLs: the validated URL is the one sent.
- Deletes dedupe ids, report per-id outcomes and refuse unsupported kinds before previewing.
- Mutating tools resolve notebooks strictly against a fresh list.
- Headless fallback browser is shut down on exit; failed launches are cached with backoff; detached chat jobs are bounded.
- Chat progress no longer resends the whole answer every second.

### Known limitations

- Studio generation, deletes other than notebooks, Drive and file sources, source guide and refresh, followup suggestions, label create and auto-label, research `discover`, and saving a chat turn as a note are not available yet.
- Uncached `notebook_list` (100 notebooks) measures about 2.3 ms p50 on the 2-CPU CI runner, above the 2 ms overhead target. Most of the remaining time is MCP JSON-RPC encoding and decoding, and the benchmark counts the client's share too (see `bench/RESULTS.md`).

[Unreleased]: https://github.com/candidloom-hub/sourcebolt/compare/v0.3.1...HEAD
[0.3.1]: https://github.com/candidloom-hub/sourcebolt/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/candidloom-hub/sourcebolt/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/candidloom-hub/sourcebolt/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/candidloom-hub/sourcebolt/releases/tag/v0.1.0
