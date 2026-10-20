---
name: sourcebolt
description: Use the sourcebolt MCP tools to work with Google NotebookLM notebooks: open a notebook, ask questions with citations, add sources, run research and import results, keep notes. Use when the user mentions NotebookLM, a notebook, or grounding answers in their own sources.
---

# sourcebolt (NotebookLM) skill

sourcebolt is an unofficial MCP server for Google NotebookLM. The user signs in on the command line (`sourcebolt login`); you cannot sign in, and you must never ask for or handle cookies or passwords. If a tool returns `AUTH_REQUIRED`, tell the user to run `sourcebolt login --attach http://127.0.0.1:9222` (see docs/auth.md) and retry.

Tool details and parameters are in docs/tools.md. Most tools take `notebook` as an id, an id prefix of 6 or more characters, or the title. Prefer the id from `notebook_list`.

## Pick the tool

| Goal | Tool |
|---|---|
| Check the server, version, transport | `server_info` |
| Find a notebook | `notebook_list`, then `notebook_open` |
| New notebook | `notebook_create` (can add sources in the same call) |
| Ask about the sources, with citations | `chat_ask` |
| Re-read earlier answers | `chat_history` |
| Add URLs, YouTube links or pasted text | `source_add` |
| See what is in the notebook and whether it finished processing | `source_list` (filter by status), `source_read` |
| Find new sources on the web | `research_start`, `research_status`, `research_import` |
| Keep a note | `note_save`, `note_list`; `chat_ask` with `save_as_note` keeps an answer |
| Check Studio artifacts (audio, reports, ...) | `studio_list` |
| Make an audio or video overview, report, slide deck, mind map, quiz, flashcards, infographic or data table (omitted options take the web app's defaults) | `studio_generate` (only the option values in its schema), then `studio_list` until ready |
| Quota or settings | `account_settings` |
| Sharing | `share_status` first, then `share_update` |

Batch tools (`chat_ask_many`) appear only when the server runs with `--tools all`.

## Recommended flows

### Open a notebook, then ask with citations

1. `notebook_list` (or `notebook_open` with a title).
2. `notebook_open` to see sources and their status. If sources are still processing, wait or use `source_list` with a status filter before asking.
3. `chat_ask` with `notebook` and `question`. The answer includes citations (source, cited text, offsets). Quote the cited text, not your guess.
4. For a follow-up, reuse the returned `conversation_id`. Use `new_conversation: true` for an unrelated topic.
5. For a slow answer, `wait: false` returns a `job_id`; poll `chat_status`, cancel with `job_cancel`.

### Add sources

1. `source_add` with `items`; each item sets exactly one of `url` or `text` (Drive and file items are not available yet). URLs must be public.
2. Read the per-item outcome: `added`, `pending` or `failed`.
3. `source_list` until nothing is pending, then ask.

### Research, then import

1. `research_start` with a descriptive query of at least four words (`mode: fast` by default, `deep` for a report; `corpus: web`).
2. `research_status` with `wait_seconds` (up to 120) until done. Candidates come with indices.
3. Show the candidates to the user, then `research_import` with the chosen indices (or all).
4. `source_list` to confirm processing, then `chat_ask`.

## Safety rules

- `delete` previews unless `confirm: true`. Show the preview to the user and get a clear yes before confirming. Notebooks, sources, notes and chat history can be deleted today.
- `share_update` previews when it widens access or removes people. Same rule.
- Never put cookies, tokens or file contents with secrets into `source_add` text.
- Quota or rate-limit errors come from Google. Report them; do not loop retrying.

## Not available yet

`studio_generate` option values or languages outside its schema enums, and `instructions` where a kind refuses or requires it (see the tool description); Drive and file sources, deleting artifacts, labels and collections, source refresh, `suggest` followups, label create and auto-label, research `discover` mode. Say so instead of retrying.

## Errors

Errors come back as a JSON envelope with a stable kind. Read the kind and the message; `UNSUPPORTED_BY_TRANSPORT` means the feature is not available yet, not that the call is malformed.
