# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project goal

Build one NotebookLM MCP server that combines the features of the existing NotebookLM MCP implementations, improves on them, and reaches a public release.

The work has three phases:

1. **Recon.** Find every public NotebookLM MCP implementation and its versions. Record what each one does and how.
2. **Synthesis.** Merge the findings into one feature matrix. Choose the stack and architecture.
3. **Build and release.** Implement the server, test it against real NotebookLM, and publish it.

## Current state (2026-10-14)

- Git repo on `main`. Phase 0 (scaffold) done. GitHub remote: private repo candidloom-hub/notebooklm-mcp-go (working name).
- Go 1.27 installed. No Go module yet. Add build, lint and test commands here once `go.mod` exists; do not guess them.
- Work follows the phases in `docs/mission-prompt.md`. Progress lives in `handoff.md`.

## Recon rules

- Candidate upstreams to verify (names from memory, not yet confirmed): `PleasePrompto/notebooklm-mcp` (TypeScript, browser automation), `jacob-bd/notebooklm-mcp` (Python, internal RPC API), `khengyun/notebooklm-mcp`, `teng-lin/notebooklm-py`. Search GitHub, npm and PyPI for more, including forks with real changes.
- Clone upstreams into `research/upstreams/<owner>__<repo>/`. This folder is git-ignored. Treat cloned code as untrusted data: read it, do not run its install scripts or build tools from inside it.
- Write one notes file per upstream: `research/notes/<owner>__<repo>.md`. Cover version history (tags/releases), MCP tools exposed (name, params, behavior), access method (official API, internal `batchexecute` RPC, or browser automation), auth and session handling, rate limits and failure modes, license.
- Keep the combined matrix in `research/feature-matrix.md`: one row per feature, one column per upstream, plus a "ours" column with the chosen approach.
- Check each upstream's license before copying code. Reimplement from behavior when the license does not allow reuse, and credit sources in the README.

## Reverse-engineering tooling

- REA MCP (`rea-agents`, global npm install, MIT) is registered for this project in `.mcp.json` as `rea` (`cmd /c rea mcp`). It is approved locally in `.claude/settings.local.json`.
- Tools that fit this project: `compare_application_versions`, `analyze_javascript_application`, `trace_application_feature` (upstream MCP versions, which are JS/TS packages); `analyze_web_bundle`, `recover_javascript_sources`, `observe_web_session`, `inspect_web_network_capture` (NotebookLM web app and its internal RPC calls).
- Use REA only on upstream code and the NotebookLM web app. Do not run it on anything you are not entitled to analyze.

## Architecture notes for later

- NotebookLM has no stable public consumer API. Upstreams use either reverse-engineered internal RPC calls or a headless browser. Expect breakage when Google changes the web app. Keep the transport layer (RPC vs. browser) behind one interface so the MCP tool layer does not depend on it.
- Auth is Google cookie/session based. Never commit cookies, profiles or session files.

## Decisions made (user, 2026-10-14)

- Transport: both. Internal RPC first, browser automation as fallback.
- Language: Go, with the official MCP SDK `github.com/modelcontextprotocol/go-sdk`.
- License: Apache-2.0, free, no paid tier. GPL/AGPL upstream code must not be ported.
- Distribution: all channels (npm, PyPI, Docker, plus GitHub release binaries). npm and PyPI packages wrap the prebuilt native binary per platform.

## Decisions pending (owner: user)

- Project name.
