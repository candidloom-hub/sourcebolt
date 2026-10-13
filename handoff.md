# Handoff

## Goal
Build one Go NotebookLM MCP server that unites all upstream features, beats them on speed, and gets released. See CLAUDE.md and docs/mission-prompt.md.

## Current state
- Phase 0 scaffold: done, pushed (candidloom-hub/notebooklm-mcp-go, private, main).
- Phase 1 discovery: done, pushed. research/upstreams.md (67 entries).
- Phase 3 performance research: done, pushed. research/performance.md.
- Phase 2 reverse engineering: in progress. 5 parallel Sonnet agents writing research/notes/<owner>__<repo>.md:
  1 Python notebooklm-py family, 2 jacob-bd lineage + Python seeds, 3 TS browser MCPs, 4 TS RPC MCPs (roomi-fields first), 5 Go/Rust clients.
  Shared rules for those agents: %TEMP%/claude/p2/common.md (read-only static analysis; REA CLI for JS).
- REA MCP tools not loaded in this session (registered after start); REA CLI used instead.

## Files touched
CLAUDE.md, .mcp.json, .gitignore, .gitattributes, handoff.md, docs/mission-prompt.md, research/*

## What changed
See git log.

## What failed
Nothing blocking. Seeds authorityandbrand/notebooklm and idankatz64-commits/notebooklm-mcp-cli are 0-ahead forks (skipped as Tier B).

## Next steps
1. When Phase 2 agents finish: review notes, commit, write research/feature-matrix.md, research/rpc-catalog.md, research/best-practices.md.
2. Phase 4: writing-plans skill -> docs/plans/. Propose 3 names.
3. STOP for owner approval of plan + name before Phase 5.
