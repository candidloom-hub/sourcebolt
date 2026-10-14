# Handoff

## Goal
Build one Go NotebookLM MCP server that unites all upstream features, beats them on speed, and gets released. See CLAUDE.md and docs/mission-prompt.md.

## Current state
- P1 core on branch feat/p1-core, pushed, CI green, NOT merged to main. Whole-branch Opus review done; all Important findings fixed with RED→GREEN tests.
- Live capture done 2026-10-13 (21 sanitized fixtures); notebook payloads pinned to live shapes (HDR options block).
- P1 Task 13 remaining: live smoke test. Cookie export from the owner lacked SID/HSID/APISID → signed-out page. Owner approved building `sourcebolt login --browser` (P4 Tasks 2+4) to read cookies directly from a dedicated browser profile.
- In flight (3 parallel agents in git worktrees): login --browser (Opus), chat stream decoder (P2 T6 wire part), safety + jobs (P2 T1, T8). Merge each worktree branch into feat/p1-core after review.
- Security follow-up for owner: first cookie file values passed through the chat transcript; owner should sign out other Google sessions after the new login works.

## Files touched
CLAUDE.md, .mcp.json, .gitignore, .gitattributes, handoff.md, docs/mission-prompt.md, research/*

## What changed
See git log.

## What failed
Nothing blocking. Seeds authorityandbrand/notebooklm and idankatz64-commits/notebooklm-mcp-cli are 0-ahead forks (skipped as Tier B).

## Next steps
1. Merge agent branches; run go test ./...; push; CI.
2. Owner runs `sourcebolt login --browser` and signs in; then `go test -tags live ./internal/nlm -run TestLiveListNotebooks -v` → closes P1 Task 13.
3. Finish P1 (finishing-a-development-branch: PR feat/p1-core → main), then P2 remaining tasks, P3, P4 rest, P5.
