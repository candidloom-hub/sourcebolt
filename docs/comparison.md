# sourcebolt compared with other NotebookLM projects

Several open-source projects connect AI apps or scripts to NotebookLM. This page compares sourcebolt with the six most adopted ones we found, so you can pick the right tool. Several of them do more than sourcebolt today, and the table says so.

## How this comparison was made

- **Which projects.** From our survey of public NotebookLM projects ([research/upstreams.md](../research/upstreams.md)), the six most-starred that are MCP servers, or clients with an MCP server or a CLI an agent can use.
- **Feature facts** come from reading each project's code at the version named below, on 2026-10-14. File-level evidence is in our notes, linked per project. We did not run these projects.
- **Stars, last push and archive status** were checked with the GitHub API on 2026-10-20.
- **sourcebolt** facts are for v0.1.0 unless marked "newer builds" (merged after v0.1.0, not yet in a release).
- Projects change quickly. If something here is out of date, please [open an issue](https://github.com/candidloom-hub/sourcebolt/issues).

Legend: ✅ yes · ⚠️ partly (see the footnote) · ❌ no · ? not covered by our review

## The projects

| Project | What it is | Version reviewed | Stars | Last push | Status | Our notes |
|---|---|---|---|---|---|---|
| **sourcebolt** | MCP server + small CLI, Go | v0.1.0 | new | 2026-10-20 | Active | |
| [teng-lin/notebooklm-py](https://github.com/teng-lin/notebooklm-py) | Python library + CLI + MCP server | v0.8.5 | 19,824 | 2026-10-19 | Active | [notes](../research/notes/teng-lin__notebooklm-py.md) |
| [jacob-bd/gemini-notebook-mcp-cli](https://github.com/jacob-bd/gemini-notebook-mcp-cli) (formerly notebooklm-mcp-cli) | MCP server + CLI (`nlm`), Python | v0.15.4 | 6,362 | 2026-10-17 | Active | [notes](../research/notes/jacob-bd__gemini-notebook-mcp-cli.md) |
| [PleasePrompto/notebooklm-mcp](https://github.com/PleasePrompto/notebooklm-mcp) | MCP server, TypeScript | v2.0.0 | 3,437 | 2026-09-10 | **Archived** | [notes](../research/notes/PleasePrompto__notebooklm-mcp.md) |
| [tmc/nlm](https://github.com/tmc/nlm) | CLI + library + MCP server, Go | v0.1.1 (HEAD 7a173b4) | 392 | 2026-09-23 | Active | [notes](../research/notes/tmc__nlm.md) |
| [roomi-fields/notebooklm-mcp](https://github.com/roomi-fields/notebooklm-mcp) | MCP server + local REST API, TypeScript | v3.2.0 | 202 | 2026-10-18 | Active | [notes](../research/notes/roomi-fields__notebooklm-mcp.md) |
| [Pantheon-Security/notebooklm-mcp-secure](https://github.com/Pantheon-Security/notebooklm-mcp-secure) | Security-focused MCP server (derived from PleasePrompto), TypeScript | 2026.5.0 | 86 | 2026-09-03 | Active | [notes](../research/notes/Pantheon-Security__notebooklm-mcp-secure.md) |

All six are MIT-licensed. sourcebolt is Apache-2.0.

## How each one talks to NotebookLM

| Project | Access method | What you install |
|---|---|---|
| sourcebolt | Internal web requests (RPC) first; headless-browser replay and page (DOM) fallback | One ~10 MB binary. A Chrome-family browser to sign in and for the fallbacks. |
| notebooklm-py | Internal web requests; optional Android API backend; browser only for login | Python 3.10+ and pip packages; Playwright for browser login |
| gemini-notebook-mcp-cli | Internal web requests; opt-in in-page fetch through Chrome | Python 3.11+ and pip packages; Chrome-family browser for login |
| PleasePrompto | Browser automation only (drives the NotebookLM page) | Node.js 18+, npm packages, Google Chrome |
| tmc/nlm | Internal web requests; browser only for login | One Go binary, built with `go install` |
| roomi-fields | Internal web requests first, browser automation fallback | Node.js, npm packages, Chrome (Patchright) |
| Pantheon-Security | Browser automation; separate Gemini API path for some tools | Node.js 20+, npm packages, Chrome; Gemini API key for the Gemini tools |

Direct requests are faster and lighter than driving a web page, but they break when Google changes a request format. Browser automation breaks when Google changes the page layout. A project with both (sourcebolt, roomi-fields) can fall back from one to the other.

## Features

| Feature | sourcebolt | notebooklm-py | gemini-notebook-mcp-cli | PleasePrompto | tmc/nlm | roomi-fields | Pantheon-Security |
|---|---|---|---|---|---|---|---|
| MCP tools | 31 | 38 | 53 | 20 | 25 [^tmc-tools] | ~40 | 48 + 18 compliance |
| List, create, rename notebooks | ✅ | ✅ | ✅ | ⚠️ [^library] | ✅ | ✅ | ⚠️ [^library] |
| Ask with citations | ✅ | ✅ | ✅ | ✅ [^pp-cite] | ✅ | ✅ | ✅ [^pp-cite] |
| Long answers in the background (start, then poll) | ✅ | ✅ | ✅ | ? | ? | ? | ? |
| Add web pages and pasted text | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Add files or Google Drive | ❌ | ✅ | ✅ | ❌ | ⚠️ [^tmc-upload] | ✅ | ⚠️ [^pan-upload] |
| Web research and import | ✅ | ✅ | ✅ | ❌ | ✅ | ⚠️ [^roo-research] | ⚠️ [^pan-research] |
| Studio generation (audio, video, slides, ...) | ✅ 10 kinds [^sb-studio] | ✅ 10 kinds | ✅ 9 kinds | ⚠️ audio only | ✅ [^tmc-studio] | ✅ | ⚠️ [^pan-studio] |
| Download Studio artifacts | ❌ | ✅ | ✅ | ⚠️ audio only | ? | ⚠️ [^roo-dl] | ⚠️ audio only |
| Notes | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ? |
| Sharing | ✅ | ✅ | ✅ | ❌ | ⚠️ audio only | ⚠️ public link | ? |
| Several Google accounts | ✅ profiles | ✅ | ✅ | ✅ | ✅ | ✅ with automatic rotation | ? [^pan-accounts] |
| Fallback when Google changes something | ✅ [^sb-fallback] | ❌ | ⚠️ [^jb-cdp] | ❌ | ❌ | ✅ | ❌ |
| Python library for your own scripts | ❌ | ✅ | ⚠️ | ❌ | ⚠️ Go library | ❌ | ❌ |
| Prebuilt package | ✅ binaries, Docker | ✅ PyPI | ✅ PyPI | ✅ npm | ❌ [^tmc-pkg] | ✅ npm | ✅ npm |

## Safety

| Safety feature | sourcebolt | notebooklm-py | gemini-notebook-mcp-cli | PleasePrompto | tmc/nlm | roomi-fields | Pantheon-Security |
|---|---|---|---|---|---|---|---|
| Session stored encrypted | ✅ AES-256-GCM, key in OS keyring | ? | ⚠️ opt-in [^jb-store] | ❌ [^pp-store] | ❌ [^tmc-store] | ⚠️ [^roo-store] | ⚠️ [^pan-store] |
| Password never handled by the tool | ✅ | ✅ | ✅ | ⚠️ [^autologin] | ✅ | ⚠️ [^autologin] | ⚠️ [^autologin] |
| Preview or confirm before delete | ✅ | ✅ | ✅ | ⚠️ [^pp-confirm] | ? | ? | ⚠️ [^pp-confirm] |
| Blocks private-network source URLs (SSRF) | ✅ | ✅ [^tl-ssrf] | ? | ? | ? | ? | ✅ |
| Local HTTP mode requires a token | ✅ | ✅ [^tl-http] | ? | ❌ [^pp-http] | n/a (stdio only) | ? | ✅ |
| Prompt-injection filter on answers | ❌ | ? | ? | ❌ | ? | ? | ✅ [^pan-inject] |
| Signed release artifacts | ✅ cosign + SBOM | ? | ? | ? | ? | ✅ npm trusted publishing [^roo-oidc] | ? |

## Where others are ahead

- **Studio options and downloads.** notebooklm-py and gemini-notebook-mcp-cli offer more Studio option values and download the results. sourcebolt generates all ten kinds but only with the option values a live capture confirmed, and does not download artifacts yet.
- **Files and Drive.** notebooklm-py, gemini-notebook-mcp-cli and roomi-fields add local files and Drive documents.
- **Breadth.** gemini-notebook-mcp-cli has 53 tools, including collections, cross-notebook queries, pipelines and exports to Google Docs and Sheets.
- **Library use.** notebooklm-py is a full Python library with a large CLI and a REST server.
- **Account rotation.** roomi-fields rotates several accounts automatically when one hits a limit.
- **Hardening extras.** Pantheon-Security adds a prompt-injection filter on answers, audit logging and compliance tools.
- **Community.** notebooklm-py and gemini-notebook-mcp-cli have thousands of stars and long release histories.

## Where sourcebolt is ahead

- **Footprint and speed:** one ~10 MB binary, no runtime to install, about 8 ms start-up ([performance.md](performance.md)).
- **Resilience:** direct requests, plus browser replay and a page fallback for chat and adding sources, with rules that never replay a change twice.
- **Safety by default:** encrypted session with the key in the OS keyring, no password handling, previews before delete and before widening sharing, SSRF checks, a token-protected loopback-only HTTP mode, signed releases.
- **Agent-friendly errors:** stable error codes with a fix and `retry_after_seconds`.

## Which should I pick?

- **You want the most NotebookLM features today, including Studio and downloads:** gemini-notebook-mcp-cli, or notebooklm-py if you also want a Python library.
- **You want a light, fast MCP server that is careful with your account, and your work is mainly notebooks, sources, chat with citations and research:** sourcebolt.
- **You want a Go library or CLI to build on:** tmc/nlm.
- **You need several accounts that rotate automatically, or a local REST API:** roomi-fields.
- **You want answer filtering, audit logs and compliance tooling, and accept browser automation:** Pantheon-Security.
- **PleasePrompto/notebooklm-mcp** is archived; pick an active project instead.

You can also run more than one: for example sourcebolt for daily questions and another server for Studio generation.

## Footnotes

[^tmc-tools]: 25 tools are registered in the code; the project's docs say 39 ([notes](../research/notes/tmc__nlm.md)).
[^library]: These keep a local library of notebook links; creating notebooks works through the page. PleasePrompto cannot create notebooks; Pantheon-Security adds `create_notebook` through the page ([notes](../research/notes/Pantheon-Security__notebooklm-mcp-secure.md)).
[^pp-cite]: Reads the answer from the page; citations are collected by clicking the citation chips, optional ([notes](../research/notes/PleasePrompto__notebooklm-mcp.md)).
[^tmc-upload]: The library and CLI upload files; the MCP server has no file-upload tool ([notes](../research/notes/tmc__nlm.md)).
[^pan-upload]: `upload_document` sends files to the Gemini API, not to NotebookLM ([notes](../research/notes/Pantheon-Security__notebooklm-mcp-secure.md)).
[^roo-research]: Fast web research only; deep research is not wired up ([notes](../research/notes/roomi-fields__notebooklm-mcp.md)).
[^pan-research]: `deep_research` uses the Gemini API, not NotebookLM's research.
[^sb-studio]: v0.1.0 lists Studio artifacts and checks quota. Newer builds generate every kind with the web app's defaults and the option values confirmed by live captures; other values return `UNSUPPORTED_BY_TRANSPORT` ([tools.md](tools.md#studio_generate)).
[^tmc-studio]: MCP tools for audio, video, slide decks and app artifacts (mind map, canvas, prototype).
[^pan-studio]: Audio, video and data tables through the page.
[^roo-dl]: Downloads three media types.
[^sb-fallback]: Browser replay for rejected requests; page (DOM) fallback for chat and adding sources only ([auth.md](auth.md#browser-fallback)).
[^jb-cdp]: An opt-in mode sends the same requests from inside Chrome, which helps when cookies are tied to the browser; it is not a page fallback.
[^jb-store]: Plain files readable only by you by default; an optional "protected" mode encrypts with AES-256-GCM and a key in the OS keystore (since v0.14.0).
[^pp-store]: Cookies live in a Chrome profile and a saved `state.json`, not encrypted by the server.
[^tmc-store]: Plain credential files readable only by you (mode 0600).
[^roo-store]: Optional auto-login credentials are encrypted; cookies live in the Chrome profile.
[^pan-store]: Saved state is encrypted (post-quantum hybrid scheme); the live Chrome profile that holds the real cookies is not.
[^autologin]: Optional auto-login with an email and password from environment variables. Off by default.
[^pp-confirm]: `cleanup_data` requires `confirm`; there is no NotebookLM delete tool.
[^tl-ssrf]: Inferred from the `allow_internal` parameter of `source_add`, which opts out of an internal-address check.
[^tl-http]: Loopback HTTP by default; OAuth for remote deployments.
[^pp-http]: The HTTP listener binds to 127.0.0.1 but has no token or Origin check.
[^pan-inject]: A response validator masks injection-like patterns in answers.
[^roo-oidc]: Publishes to npm with OIDC trusted publishing.
[^pan-accounts]: The project split from PleasePrompto before PleasePrompto added multi-account support; our review did not confirm it.
[^tmc-pkg]: No npm or PyPI package was found, and the GitHub releases list was empty on 2026-10-20 (tags only). Install with `go install`.
