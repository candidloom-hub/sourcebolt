# Launch and adoption plan

Status: draft for the owner. **Nothing in this plan has been posted, submitted or published.** Every step that puts something in public, on a third-party site, or under the owner's name is marked **[owner approval]** and waits for an explicit go.

Sources for directory rules: [research/marketplaces.md](../research/marketplaces.md) (checked 2026-10-20) and the pages linked below. Subreddit rules could not be read: Reddit blocks unauthenticated access, so each Reddit row says "read the rules first" instead of quoting them.

## What we can honestly say

Use only these facts and numbers. They come from `README.md`, `CHANGELOG.md` and `bench/RESULTS.md`.

- Unofficial MCP server for Google NotebookLM, written in Go. Apache-2.0, free, no paid tier.
- One static binary for Linux, macOS and Windows (amd64 and arm64), plus Docker images. Release v0.2.0 is out, on npm (`npx -y sourcebolt serve`) too; PyPI is not published yet.
- 30 tools by default (31 with `--tools all`, 8 with `--tools minimal`): notebooks, sources, chat with citations, research, notes, sharing, labels, Studio list, deletes with preview.
- Measured on one Windows laptop (2026-10-14; high variance, order of magnitude only): cold start to `initialize` p50 10.3 ms; 10.06 MB stripped binary; 12.4 MB idle memory. A live `notebook_list` round trip was p50 848 ms over 10 calls on one account, almost all of it Google.
- Cookies are stored AES-256-GCM encrypted, with the key in the OS keyring. No telemetry. Talks only to the NotebookLM host ([privacy.md](privacy.md)).
- Limits to state every time: it uses NotebookLM's internal web calls, so it can break when Google changes the site; automating a Google web app with your session may violate Google's terms; several options are "coming soon" until a live capture pins them.

Do not claim: "fastest", user counts, star counts, endorsements, or numbers not in the list above.

## Prerequisites before any public post

| # | Step | Who | Status |
|---|---|---|---|
| P1 | Merge `feat/distribution` (this branch) after review | owner | **[owner approval]** |
| P2 | README links `docs/install-links.md`, `docs/sponsor.md`, `docs/privacy.md`, and keeps the `mcp-name` comment (needed for PyPI ownership) | README agent | in progress elsewhere |
| P3 | Enable GitHub Sponsors for `candidloom-hub` and set tiers ([sponsor.md](sponsor.md)) | owner | **[owner approval]** (money) |
| P4 | Enable private vulnerability reporting | owner | **[owner approval]** |
| P5 | Next release tag (for example v0.2.0) so the `.mcpb` bundles exist on a release | owner | **[owner approval]** |
| P6 | Record the demo GIF and a 2-3 minute video (script below), with a throwaway Google account | owner | **[owner approval]** (uses a real account) |
| P7 | Repository topics: `mcp`, `mcp-server`, `notebooklm`, `model-context-protocol`, `golang`, `gemini-cli-extension` | owner | **[owner approval]** |
| P8 | npm token and PyPI trusted publisher, then publish packages and add them to `server.json` ([registry.md](registry.md)) | owner | **[owner approval]**, optional before launch |

## Target list

### Registries and directories

| Target | How | Notes | Approval |
|---|---|---|---|
| Official MCP Registry | `mcp-publisher login github`, `mcp-publisher publish` | `server.json` lists only the GHCR image today; feeds the GitHub/VS Code gallery and later PulseMCP | **[owner approval]** |
| GitHub MCP registry / VS Code `@mcp` | Automatic from the official registry; GitHub's 2025 blog says to email partnerships@github.com to be included | Check whether self-publication shipped before emailing | **[owner approval]** (email) |
| Glama | Claim with `glama.json` (in repo) via glama.ai "Claim ownership" | Check first whether it is already indexed | **[owner approval]** (account) |
| mcp.so | Free web form at https://mcp.so/submit | Do not use the paid fast-track | **[owner approval]** |
| mcpservers.org | Web form at https://mcpservers.org/submit (the wong2 awesome list sends submissions there) | | **[owner approval]** |
| Cline MCP Marketplace | Issue in `cline/mcp-marketplace` with repo URL, 400x400 PNG (`go run ./tools/icon -png 400 -o icon-400.png`), reason | Test that Cline can install from the README first | **[owner approval]** |
| Smithery | `smithery mcp publish ./sourcebolt-mcp-<os>-<arch>.mcpb -n <org>/sourcebolt` | Needs P5 | **[owner approval]** (account) |
| Docker MCP Catalog | PR to `docker/mcp-registry` with `server.yaml` | Confirm non-interactive login (`login --import -` from a secret) first | **[owner approval]** (third-party PR) |
| Gemini CLI gallery | Automatic once the repo has the `gemini-cli-extension` topic and `gemini-extension.json` | Needs P7 | **[owner approval]** (topic) |
| PulseMCP | Submissions paused; picks up registry entries later | Nothing to do | none |
| Anthropic directory, ChatGPT, Grok, Antigravity Marketplace | Not pursued: remote-only or "own the API" rules (see research) | | n/a |

### Awesome lists

| List | How | Approval |
|---|---|---|
| punkpeye/awesome-mcp-servers (about 97k stars, active) | One-line PR in the right category, matching the format in CONTRIBUTING | **[owner approval]** (third-party PR) |
| TensorBlock/awesome-mcp-servers (active) | Follow its README's submission steps | **[owner approval]** |
| etewiah/awesome-notebooklm | PR or issue, if the list takes tools | **[owner approval]** |
| appcypher/awesome-mcp-servers | Archived; skip | n/a |

Suggested line for awesome lists (adapt to each list's format):

```markdown
- [candidloom-hub/sourcebolt](https://github.com/candidloom-hub/sourcebolt) - Unofficial NotebookLM server in Go: notebooks, sources, cited chat, research and notes. One static binary, cookies encrypted locally.
```

### Communities and social

| Channel | Rules to respect | Approval |
|---|---|---|
| Show HN | Title starts with "Show HN"; must be something people can try; be there to answer; no asking friends to upvote ([Show HN rules](https://news.ycombinator.com/showhn.html)) | **[owner approval]** |
| r/mcp | Read the sidebar rules and flair requirements first (could not be fetched) | **[owner approval]** |
| r/notebooklm | Read the rules first; many product subreddits restrict tool promotion. If unsure, message the moderators before posting | **[owner approval]** |
| r/ClaudeAI | Read the rules first; use the project/showcase flair if required | **[owner approval]** |
| r/golang | Read the rules first; frame the post around the Go engineering, not the product | **[owner approval]** |
| r/LocalLLaMA, r/SideProject, r/opensource | Optional, later; read each sidebar first | **[owner approval]** |
| Product Hunt | Needs a maker account; prepare gallery images and a first comment | **[owner approval]** (account) |
| X and LinkedIn | Owner's own accounts | **[owner approval]** |
| dev.to (or a blog) | Owner's account; cross-post canonical URL | **[owner approval]** |
| YouTube | Owner's channel; the demo video | **[owner approval]** |

General rule for every community: one post per community, no cross-posting the same text on the same day, answer every question, say clearly that it is unofficial and can break.

## Two-week schedule

Day 0 is the day the owner approves this plan. Times are in the poster's local time; for HN and Reddit, a weekday morning in US Eastern time tends to get more readers.

| Day | Work | Approval |
|---|---|---|
| 0 | Owner reviews this plan and the drafts; picks which channels to use | **[owner approval]** |
| 1 | P1, P3, P4, P7. Run `go test ./...` on main | **[owner approval]** |
| 2 | P5: tag the next release; check that the draft release has the 6 `.mcpb` files and `sourcebolt-mcp-checksums.txt`; publish the draft | **[owner approval]** |
| 3 | Install the `.mcpb` in Claude Desktop on Windows and macOS; install the Claude Code plugin from the marketplace; click the VS Code and Cursor links. Fix anything broken before launch | owner (needs a human and real apps) |
| 4 | P6: record the GIF and video | **[owner approval]** |
| 5 | Official MCP Registry publish; Glama claim; mcp.so and mcpservers.org forms | **[owner approval]** |
| 6 | Awesome-list PRs (punkpeye, TensorBlock); Cline issue | **[owner approval]** |
| 7 | Rest. Check that registry and directory entries show correct data | none |
| 8 | Publish the blog post (dev.to) | **[owner approval]** |
| 9 | Show HN, linking the repo (not the blog). Stay available for the day | **[owner approval]** |
| 10 | r/mcp post; X and LinkedIn posts | **[owner approval]** |
| 11 | r/notebooklm (after checking rules or asking mods) and r/golang | **[owner approval]** |
| 12 | Product Hunt launch (optional) | **[owner approval]** |
| 13 | YouTube video; r/ClaudeAI | **[owner approval]** |
| 14 | Review: issues opened, install problems, which channel brought users (GitHub traffic page). Write down what to fix | none |

## Ready-to-paste drafts

Replace `vX.Y.Z` with the release that has the `.mcpb` files. Keep the numbers as they are unless re-measured.

### Show HN

Title:

```text
Show HN: Sourcebolt - an MCP server for NotebookLM in one Go binary
```

Body (URL field: the GitHub repo; text below as the first comment):

```text
I use NotebookLM to keep answers grounded in my own sources, and I wanted my coding agents (Claude Code, Cursor, Codex) to use it too. NotebookLM has no public API, so sourcebolt talks to the same internal web endpoint the site uses, with your own Google session.

What it does: list and open notebooks, add URL/YouTube/text sources, ask questions and get answers with citations (source and cited passage), run research and import results, keep notes, manage sharing. 30 tools by default, 8 in a minimal profile for small models.

How it is built: a single static Go binary (about 10 MB), stdio or loopback HTTP. It calls the web RPC directly over HTTP/2 and falls back to a headless browser when Google changes a request shape. Cold start to MCP initialize measured about 10 ms on my laptop; a real notebook list takes about 850 ms, almost all of it Google.

Sign-in: you sign in to Google yourself in a browser, and sourcebolt reads the cookies (it never asks for a password). They are stored AES-256-GCM encrypted with the key in your OS keyring. No telemetry; it talks only to NotebookLM.

Caveats, honestly: it is unofficial and will break when Google changes the web app (each fix needs a new capture of the site's traffic). Automating a Google web app with your session may be against Google's terms, so use an account you can afford to lose access to. Several options say "coming soon" until we have captured them.

Apache-2.0. Install: release binaries, Docker, a Claude Code plugin, a Claude Desktop extension (.mcpb), and VS Code/Cursor install links. I'd like feedback on the auth flow and on which NotebookLM features you would use from an agent.
```

### Reddit: r/mcp

Title:

```text
sourcebolt: open-source MCP server for NotebookLM (Go, single binary, cited answers)
```

Body:

```text
I built an MCP server for Google NotebookLM: notebooks, sources, chat with citations, research, notes and sharing. 30 tools by default, a minimal 8-tool profile for small models.

- One static Go binary for Linux/macOS/Windows, or Docker. Also a Claude Code plugin, a Claude Desktop extension (.mcpb), and VS Code/Cursor install links.
- Stdio by default; loopback streamable HTTP with a bearer token if you need it.
- Cookies stay on your machine, encrypted, key in the OS keyring. No telemetry.

It is unofficial: NotebookLM has no public API, so it uses the site's internal calls and can break when Google changes them. When a call fails it falls back to a headless browser, and for chat and adding sources it can drive the NotebookLM page itself.

Repo: https://github.com/candidloom-hub/sourcebolt (Apache-2.0). Feedback on tool design is welcome, especially from people running it with smaller models.
```

### Reddit: r/notebooklm

Check the rules or ask the moderators first. Title:

```text
I made a free, open-source way to use your NotebookLM notebooks from AI coding tools (unofficial)
```

Body:

```text
If you use Claude, Cursor, VS Code or Gemini CLI, sourcebolt lets them work with your own NotebookLM notebooks: ask questions and get answers with citations from your sources, add sources, run research and save notes.

It runs on your computer and uses your own Google sign-in (you sign in yourself in a browser; it never sees your password). It sends nothing to anyone except NotebookLM.

Please read the caveats: it is not from Google, it uses NotebookLM's internal web calls, it can break when the site changes, and automating it may be against Google's terms. Free and open source (Apache-2.0): https://github.com/candidloom-hub/sourcebolt
```

### Reddit: r/golang

Title:

```text
Writing a client for an undocumented batchexecute API in Go: lessons from an MCP server for NotebookLM
```

Body:

```text
sourcebolt is an MCP server for NotebookLM in Go. Some things that may interest Go folks:

- Decoding Google's chunked batchexecute responses with a lazy view instead of full unmarshalling cut notebook decode allocations about 111x.
- A TTL cache with singleflight and stale-while-revalidate for read-only tools.
- Cookies encrypted with AES-256-GCM, key in the OS keyring via go-keyring, 0600 key-file fallback in containers.
- Benchmarks gate CI; base and head run interleaved because sequential runs drifted on a laptop.
- One CGO-free static binary per platform via GoReleaser, with SBOMs and cosign signatures.

Numbers (GitHub Actions runner, 2 CPUs, v0.2.0): cold start to MCP initialize p50 8.2 ms, 16.2 MB idle, 0.40 ms of its own work per cached call; one ~10 MB binary.

Repo: https://github.com/candidloom-hub/sourcebolt. Happy to answer questions about the wire format work or the official MCP Go SDK.
```

### Product Hunt

Name: `sourcebolt`. Tagline (keep it under 60 characters):

```text
Use your NotebookLM notebooks from any AI agent
```

Description:

```text
Free, open-source MCP server that connects Claude, Cursor, VS Code and Gemini CLI to your Google NotebookLM notebooks: cited answers from your sources, adding sources, research and notes. Runs locally, one binary, cookies encrypted on your machine. Unofficial; not affiliated with Google.
```

First comment: reuse the Show HN body.

### X (one post)

```text
sourcebolt: a free, open-source MCP server for Google NotebookLM.

Ask your notebooks from Claude, Cursor, VS Code or Gemini CLI and get answers with citations. One Go binary, ~8 ms cold start, cookies encrypted locally, no telemetry.

Unofficial and can break when Google changes the site.
https://github.com/candidloom-hub/sourcebolt
```

### LinkedIn

```text
I released sourcebolt, an open-source (Apache-2.0) MCP server that lets AI assistants such as Claude, Cursor and VS Code work with Google NotebookLM notebooks.

Why: NotebookLM is good at answers grounded in your own sources, with citations. Agents benefit from the same grounding, but NotebookLM has no public API.

How: a single Go binary that runs on your machine, uses your own Google session (stored encrypted), and talks only to NotebookLM. It starts in about 10 ms on my laptop.

Caveats: it is unofficial, not affiliated with Google, and can break when the site changes.

Repo and install options: https://github.com/candidloom-hub/sourcebolt
```

### dev.to / blog post outline

Title: `Giving AI agents cited answers from NotebookLM: building sourcebolt`

1. The problem: grounded, cited answers from your own sources, inside the tools you code in.
2. No public API: how the NotebookLM web app talks to its backend (batchexecute), and why that is fragile.
3. Design: one Go binary, RPC first, headless-browser fallback, one transport interface.
4. Sign-in without passwords: attach to a browser you signed in yourself; encrypted storage; why `login --browser` gets blocked by Google.
5. Tool design for agents: id-or-title resolution, preview before delete, detached long answers, minimal profile.
6. Measured performance (the numbers above, with the caveats).
7. Install options (link `docs/install-links.md`).
8. Limits and risks, stated plainly.
9. How to help: report breakages, sponsor (`docs/sponsor.md`).

### Demo GIF and video script

Record with a throwaway Google account and a notebook holding public sources only. Do not show cookies, the DevTools URL, or account email. Crop out the browser profile path.

GIF (about 20 seconds, no audio):

1. Claude Code with the plugin installed. Prompt: "List my NotebookLM notebooks."
2. Prompt: "In the 'MCP spec' notebook, what does the spec say about tool annotations? Quote your sources."
3. Show the answer with citation markers and quoted passages.

Video (2-3 minutes):

1. 0:00 What it is, in one sentence, and the unofficial caveat on screen.
2. 0:15 Install: download the release binary (or the `.mcpb` double-click in Claude Desktop).
3. 0:40 Sign in: start the browser with the debugging port, sign in, `sourcebolt login --attach`, `sourcebolt doctor`. Blur the screen while signed-in pages are visible.
4. 1:20 Ask with citations in Claude Code; follow-up question in the same conversation.
5. 1:50 Add a URL source and ask about it.
6. 2:15 Privacy: what is stored and where; nothing sent elsewhere.
7. 2:35 Where to report breakages; link to the repo.

## Final checklist

- [ ] Plan and drafts reviewed and channels chosen **[owner approval]**
- [ ] `feat/distribution` merged **[owner approval]**
- [ ] README links the new docs and keeps `mcp-name`
- [ ] Sponsors enabled, tiers set **[owner approval]**
- [ ] Private vulnerability reporting on **[owner approval]**
- [ ] Release with `.mcpb` files tagged and the draft published **[owner approval]**
- [ ] Manual install checks on real apps (Claude Desktop, Claude Code plugin, VS Code, Cursor): needs a human with those apps
- [ ] Demo GIF and video recorded **[owner approval]**
- [ ] Repository topics set **[owner approval]**
- [ ] MCP Registry published **[owner approval]**
- [ ] Glama claimed; mcp.so and mcpservers.org submitted **[owner approval]**
- [ ] Awesome-list PRs and Cline issue opened **[owner approval]**
- [ ] Smithery and Docker catalog (later) **[owner approval]**
- [ ] Blog post, Show HN, Reddit posts, Product Hunt, X, LinkedIn, YouTube **[owner approval]** each
- [ ] Day-14 review written
