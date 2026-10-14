# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow [Semantic Versioning](https://semver.org/) once released.

## [Unreleased]

Nothing has been released yet. This section summarizes the work on `main` and the release branch so far (P1 core, P2 to P4 tools and transports, P5 packaging and docs).

### Added

- Core (P1): Go module, `batchexecute` request encoder and chunked response decoder with status, drift and null policies, RPC id registry with runtime overrides (`NOTEBOOKLM_RPC_IDS`), TTL cache with singleflight and stale-while-revalidate, notebook id/prefix/title resolver.
- Auth and storage: cookie import (Netscape, JSON, header), AES-256-GCM encrypted profile store with the key in the OS keyring (key file fallback), revision-checked saves, `login --import`, `login --browser`, `login --attach` (CDP), `logout`, `status`, `doctor`.
- CLI: `serve` (stdio, or loopback streamable HTTP with bearer token and Origin check), `call <tool> '<json>'`, tool profiles `default`, `minimal` and `all`.
- Transports: tuned HTTP/2 RPC transport, streamed chat transport with first-chunk, idle, total and size limits, in-page browser fetch transport on a dedicated profile, and a router that falls back from RPC to the headless browser (`--no-browser-fallback` / `NOTEBOOKLM_BROWSER_FALLBACK=0` to disable).
- 31 MCP tools: `server_info`; `notebook_list`, `notebook_open`, `notebook_create`, `notebook_update`; `source_add`, `source_list`, `source_read`, `source_update`; `chat_ask`, `chat_status`, `chat_history`, `chat_configure`, `chat_ask_many`, `suggest`; `research_start`, `research_status`, `research_import`; `note_list`, `note_save`; `share_status`, `share_update`; `label_list`, `label_update`; `studio_list`, `studio_generate` (quota check only); `account_settings`, `auth_login`, `auth_logout` (CLI hints only); `delete` (notebooks); `job_cancel`. Several options return `UNSUPPORTED_BY_TRANSPORT` until a live capture pins them; see [docs/tools.md](docs/tools.md).
- Progress notifications for long calls and a local job table for detached chat (`wait=false`).
- Safety: SSRF URL check, path allow-root check, log redaction, preview-before-delete and preview-before-share-widening.
- Performance: benchmark suite with a regression gate in CI (`bench/`); lazy wire view that cut notebook decode allocations about 111x.
- Packaging: GoReleaser config (archives, checksums, SBOMs, cosign signatures, Docker images including a `-browser` variant), npm launcher with per-platform packages, PyPI per-platform wheels. Nothing is published.
- Documentation: README, generated tool reference (`go run ./tools/gendocs > docs/tools.md`, checked for staleness by a test in CI), auth and security guides, agent `SKILL.md`, MCP registry `server.json`.
- Live capture tooling (`tools/capture`) and sanitized golden fixtures from a real NotebookLM session.

### Fixed

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
- Uncached notebook-list tool calls miss the 2 ms overhead target on the measured machine (see `bench/RESULTS.md`).
