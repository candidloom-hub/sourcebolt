# Handoff

## Goal
Build one Go NotebookLM MCP server that unites all upstream features, beats them on speed, and gets released. See CLAUDE.md and docs/mission-prompt.md.

## Current state
- P1 merged to main via PR #1 (11ddd1f). Live smoke passes (login --attach; 9 notebooks; RTT p50 848 ms).
- Branch feat/p2-tools (pushed, NOT merged): 26 MCP tools live; read tools verified live against the owner account (notebook_open, source_list, note_list, label_list, studio_list, chat_history, share_status, account_settings).
- Merged on feat/p2-tools: chat Ask (streamed), research, notes, notebook delete, studio list/get, source read/update, router + in-page browser fetch wired into app (--no-browser-fallback / NOTEBOOKLM_BROWSER_FALLBACK=0), progress notifications, jobs.
- In flight: notebook_create/update, chat_ask_many, auth tools (agent).
- Blocked on next live capture: studio generate + media positions, deletes of sources/artifacts/chat/notes, source guide/refresh, Drive add, followups, label create/autolabel, GetOrCreateAccount, SetLanguage slot, research status values.
- Known follow-ups: network errors (connection refused) are fallback-eligible and may launch a headless browser needlessly; ResponseHeaderTimeout 30 s vs stream first-chunk 5 min; deferred minors in ledger.
- Security: owner to sign out other Google sessions (first cookie file passed through transcript) and delete notebook.google.com_cookies.txt.

## Files touched
CLAUDE.md, .mcp.json, .gitignore, .gitattributes, handoff.md, docs/mission-prompt.md, research/*

## What changed
See git log.

## What failed
Nothing blocking. Seeds authorityandbrand/notebooklm and idankatz64-commits/notebooklm-mcp-cli are 0-ahead forks (skipped as Tier B).

## Next steps
1. Merge notebook_create agent; live chat_ask test on a fresh test notebook (create via notebook_create, ask, then owner deletes).
2. Live capture session #2 (login --attach + in-page hook) for the blocked list above.
3. PR feat/p2-tools → main; then P4 Task 5 (DOM fallback), P5 docs + release candidate (owner gate).
