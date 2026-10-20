# AI marketplaces and MCP directories: where sourcebolt can be listed

Date checked: 2026-10-20. Sources are the owners' own docs, repos and submission pages unless a line says "secondary". Nothing was submitted, no accounts were created, and no issues or PRs were opened on third-party repos.

What sourcebolt is, for the fit judgements below: a single static Go binary that runs a **local** MCP server (stdio, or loopback streamable HTTP with a bearer token). It talks to Google NotebookLM's **internal, unofficial** web RPC using the **user's own Google session** (cookie import or attach to the user's browser). There is no hosted endpoint and no OAuth server. Released artifacts: GitHub release archives for 6 platforms and GHCR images `ghcr.io/candidloom-hub/sourcebolt:0.1.0` and `:0.1.0-browser`. The npm and PyPI wrappers exist but are not published. `server.json` exists but has not been published to the registry (a registry search for `sourcebolt` returned 0 results on 2026-10-20).

Two facts decide most rows:

1. **Many directories now accept only remote HTTPS servers.** These are ChatGPT, Grok, and Anthropic's connector listings. A local stdio server cannot go into them without a hosted endpoint, and we will not host one: it would hold users' Google cookies on our servers.
2. **Several curated directories require you to own or be authorized for the service you connect to.** Anthropic's directory policy and OpenAI's guidelines both say so. sourcebolt is an unofficial client of a Google service, so a listing there is unlikely even when the packaging fits.

## Summary

| Target | Accepts local stdio? | What we need | Effort | Priority |
|---|---|---|---|---|
| Official MCP Registry | Yes (metadata for npm, PyPI, NuGet, OCI, MCPB, Cargo) | Publish a `server.json` that lists only packages that exist. OCI works today; npm and PyPI need publishing first | S | 1 |
| GitHub MCP Registry / VS Code `@mcp` gallery | Yes (fed from the official registry) | Official registry entry, then possibly an email request to GitHub (secondary, 2025) | S | 2 |
| VS Code install links | Yes | Add a `vscode:mcp/install?...` link to the README | S | 2 |
| Cursor deeplink | Yes | Add a `cursor://anysphere.cursor-deeplink/mcp/install?...` link to the README | S | 2 |
| Claude Code: our own plugin marketplace | Yes | Plugin folder with `.mcp.json` plus `.claude-plugin/marketplace.json` in our repo | S-M | 2 |
| Gemini CLI extensions gallery | Yes | `gemini-extension.json` at repo root, `gemini-cli-extension` topic, release assets named for the gallery. Gemini CLI is now legacy for most users | S | 3 |
| Glama | Yes | Check whether it is indexed already; add `glama.json` to claim it; Dockerfile already exists | S | 3 |
| mcp.so | Yes | Web form (repo URL, name). Paid fast-track exists; not needed | S | 3 |
| awesome-mcp-servers (punkpeye) | Yes (local only; remote has its own list) | One-line PR (owner decision: we do not open PRs on third-party repos without approval) | S | 3 |
| Cline MCP Marketplace | Yes | GitHub issue with repo URL, 400x400 PNG logo, reason; README Cline can follow (`llms-install.md` optional) | S | 3 |
| Docker MCP Catalog | Yes (containerized local) | PR to `docker/mcp-registry` with `server.yaml`; config volume and cookie import story for containers | M | 4 |
| Smithery | Yes, but only as an MCPB bundle | Build `.mcpb` bundles, then `smithery mcp publish ./server.mcpb` | M | 4 |
| Claude Desktop `.mcpb` (direct install) | Yes | Build per-platform `.mcpb` with a `binary` server; attach to GitHub releases | M | 4 |
| PulseMCP | n/a (submissions paused) | Nothing now; says it will pick up official-registry entries later | none | auto |
| Anthropic directory: plugin bundle | Local servers only inside a plugin | Plugin repo, README, LICENSE, privacy policy; but policy 3F ("own or control any API endpoint") likely blocks us | M | 5 (likely no) |
| Anthropic directory: MCP connector | No (remote HTTPS only) | Hosted endpoint + OAuth | L | no |
| Anthropic directory: desktop extension (MCPB) listing | No longer accepted | n/a | n/a | no |
| Cursor Marketplace (plugins) | Plugins hold MCP servers; manual curation | Plugin repo + publisher form; review is manual, not self-serve | M | 5 |
| Google Antigravity Marketplace | Unclear; curated | Interest form only | M | 5 (likely no) |
| ChatGPT plugin/app directory (OpenAI) | No (public HTTPS for MCP) | Hosted endpoint, OAuth, domain verification; guidelines reject unofficial third-party connectors | L | no |
| Codex (OpenAI) | Yes for user config, no directory path found | README snippet for `codex mcp add` / `config.toml` | S | 3 (docs only) |
| xAI Grok (API and grok.com connectors) | No (Streamable HTTP or SSE, public internet) | Hosted endpoint; no third-party listing path found | L | no |
| Windsurf | Unknown (no primary source found) | README config snippet only | S | 4 (docs only) |

Effort: S = under a day, M = a few days, L = a new hosted service.

## 1. Official MCP Registry (registry.modelcontextprotocol.io)

- **What it is.** A metadata registry. The quickstart says "The MCP Registry only hosts metadata, not artifacts" ([quickstart](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/quickstart.mdx)). It is in preview, and breaking changes or data resets may happen before general availability (same page).
- **Local servers.** Accepted. Package types: npm, PyPI, NuGet, OCI, MCPB and Cargo ([package types](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/package-types.mdx)).
- **Ownership checks** ([package types](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/package-types.mdx)):
  - npm: `mcpName` in `package.json` must match `server.json` `name`. Our launcher already has it (`npm/launcher/package.json`).
  - PyPI: an `mcp-name: <name>` token in the README, followed by whitespace, a tag or `-->`. Our README already has `<!-- mcp-name: io.github.candidloom-hub/sourcebolt -->`.
  - OCI: label or annotation `io.modelcontextprotocol.server.name` matching the name. GHCR (`ghcr.io`) is an allowed registry. Our `.goreleaser.yaml` already sets this label on both images.
  - MCPB: the artifact must be on GitHub or GitLab releases, the URL must contain "mcp", and `server.json` must carry `fileSha256`.
- **Auth and namespace.** `mcp-publisher login github` grants `io.github.<username>/*`. Org namespaces need the Owner role ([authentication](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/authentication.mdx)). `candidloom-hub` is a user account, so `io.github.candidloom-hub/sourcebolt` works with plain GitHub login. DNS and HTTP login give reverse-DNS namespaces for a domain we own.
- **Steps.** Install `mcp-publisher`, run `mcp-publisher login github`, then `mcp-publisher publish` ([quickstart](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/quickstart.mdx)). The description limit is 100 characters; ours is 94.
- **Review.** None described. The publish call validates ownership of each listed package.
- **Cost.** None stated.
- **Fit: yes, now.** Our `server.json` lists npm and PyPI packages that are not published, so publishing it as-is should fail the ownership check. Two options:
  - (a) Publish a `0.1.0` entry with only the OCI package now, and add npm and PyPI in a later version.
  - (b) Wait until npm and PyPI are published.

  Option (a) gets us into downstream directories sooner (GitHub, VS Code, and PulseMCP once it reopens). Adding a `github-oidc` publish step to `release.yml` would keep it current. That auth method appears in the registry docs index but was not detailed on the page we fetched.

## 2. GitHub MCP Registry and the VS Code MCP gallery

- `code.visualstudio.com/mcp` redirects to `github.com/mcp` (checked 2026-10-20). VS Code's own page says only that **MCP: Browse MCP Servers** opens the gallery in the Extensions view (`@mcp`, Preview), and does not name the source ([VS Code MCP servers](https://code.visualstudio.com/docs/copilot/customization/mcp-servers)).
- GitHub's blog (October 24, 2025, so possibly outdated) says downstream registries "automatically pick up updates" from the official registry. It also says that to be included you "email partnerships@github.com and request for your server to be included", and that self-publication was coming ([GitHub blog](https://github.blog/ai-and-ml/generative-ai/how-to-find-install-and-manage-mcp-servers-with-the-github-mcp-registry/)). We found no newer primary source saying self-publication shipped. **Conflict/gap**: treat the email step as possibly still required.
- **Install links (works today, no listing needed).** VS Code has a URL handler `vscode:mcp/install?{json-configuration}`, built with `encodeURIComponent(JSON.stringify(obj))`. Insiders uses `vscode-insiders:` ([MCP developer guide](https://code.visualstudio.com/docs/copilot/guides/mcp-developer-guide)). The CLI form is `code --add-mcp '{"name":...,"type":"stdio","command":...,"args":[...]}'` ([MCP configuration reference](https://code.visualstudio.com/docs/copilot/reference/mcp-configuration)). The `https://vscode.dev/redirect/mcp/install` wrapper that GitHub READMEs use (GitHub does not render `vscode:` links) is documented only in secondary sources.
- **Fit: yes.** We need a registry entry and README install links. Until npm is published, the link must call an installed binary (`"command": "sourcebolt"`) or `docker run -i --rm -v sourcebolt-config:/config ghcr.io/candidloom-hub/sourcebolt:0.1.0`.

## 3. Cursor

- **Install deeplink (works today).** `cursor://anysphere.cursor-deeplink/mcp/install?name=$NAME&config=$BASE64_ENCODED_CONFIG`, where `config` is the base64 of the JSON server config in `mcp.json` format ([Cursor install links](https://cursor.com/docs/context/mcp/install-links)). The page shows text and button variants but no `https://` wrapper.
- **Cursor Marketplace.** It lists plugins, which can bundle MCP servers in `mcp.json` and use `${CURSOR_PLUGIN_ROOT}` for paths. "Every plugin is manually reviewed", plugins must be open source, and each update is reviewed. Submit at [cursor.com/marketplace/publish](https://cursor.com/marketplace/publish) ([Cursor plugins](https://cursor.com/docs/plugins)). A forum reply that appears to be from Cursor staff says it is curated and not self-serve, and turnaround varies ([forum, secondary](https://forum.cursor.com/t/unable-to-test-or-publish-the-cursor-plugin-for-scalekit/153341)). We found no Cursor rule on unofficial third-party integrations.
- **Fit: deeplink yes, now. Marketplace yes-after-plugin, low odds and slow.**

## 4. Anthropic

### 4a. Claude directory: MCP connector (remote)

- Remote only: "A local server can't be submitted on its own; include it in a plugin instead". Submission needs an `https://` URL, OAuth 2.0 for authenticated services, a `title` plus `readOnlyHint`/`destructiveHint` on every tool, a privacy policy URL, docs, an icon and a reviewer test account ([submit a connector](https://claude.com/docs/connectors/building/submission)). The Data handling step asks whether the API is "your own, proxied from a partner with permission, or a third party's you don't control" (same page).
- **Fit: no.** We would need a hosted service holding users' Google sessions. That is out of scope and conflicts with our auth model.

### 4b. Claude directory: desktop extensions (MCPB)

- "Desktop extension listings in the directory are deprecated, and the directory no longer accepts local MCP servers packaged as desktop extensions (MCPB)" ([publish to the directory](https://claude.com/docs/directory/publish); same on [MCPB page](https://claude.com/docs/connectors/building/mcpb)). **Conflict**: older support articles and third-party guides still describe a desktop-extension submission form. The current docs win.
- Users can still install a `.mcpb` by double-click, by drag and drop, or via Settings > Extensions > Advanced settings > Install Extension ([MCPB page](https://claude.com/docs/connectors/building/mcpb)). Claude Desktop runs on `darwin` and `win32` only (same page).
- **Fit: direct-install `.mcpb` yes-after-build; directory listing no.** Bundle format notes from the [manifest spec](https://github.com/modelcontextprotocol/mcpb/blob/main/MANIFEST.md):
  - current `manifest_version` 0.3
  - `server.type: "binary"`, with `mcp_config.platform_overrides` for `win32`
  - `compatibility.platforms`
  - `user_config` for settings
  - `privacy_policies` (URLs), required when the extension connects to external services that process user data
  - `icon.png` (512x512 recommended, 256x256 minimum, per the [MCPB page](https://claude.com/docs/connectors/building/mcpb))
  - CLI: `npm install -g @anthropic-ai/mcpb`, then `mcpb init` and `mcpb pack`

  We would ship one `.mcpb` per OS/arch (or one with both Windows and macOS binaries) on GitHub releases. The same artifacts serve Smithery and the registry's MCPB package type.

### 4c. Claude directory: plugin bundle (the only directory path for local servers)

- **Who can submit.** Pro, Max, Team or Enterprise. Free accounts cannot submit. No cost is stated ([publish to the directory](https://claude.com/docs/directory/publish)).
- **Steps.** Submit from a GitHub repo through [claude.ai/directory/manage](https://claude.ai/directory/manage) as **Plugin bundle**. The process ([submit a plugin](https://claude.com/docs/plugins/submit)):
  - validate
  - listing details from `plugin.json` and the README
  - data-handling questions
  - four compliance acknowledgements
  - submit
  - automated scan plus human review for a new listing
- **Review time.** "Review time isn't fixed" ([publish](https://claude.com/docs/directory/publish)).
- **Packaging rules** ([pre-submission checklist](https://claude.com/docs/plugins/pre-submission-checklist)):
  - README of at least 40 words, and a LICENSE
  - no reserved names
  - compiled executables are "Held for a reviewer"
  - launchers (`npx`, `uvx`) must pin an exact version, and even then are held for review
  - credentials only through `userConfig` with `sensitive: true`
  - local server commands must run a file under `${CLAUDE_PLUGIN_ROOT}` without a shell
  - `.mcpb` bundles inside plugins are held
- **Policy blocker.** Software Directory Policy section 3F (updated April 15, 2026): "Developers must verify that they own or control any API endpoint, domain, or user interface their Software connects to". The only exception is that plugins "may connect to any Connector approved in the Software Directory" ([policy](https://support.claude.com/en/articles/13145358-anthropic-software-directory-policy)). sourcebolt connects to NotebookLM, which we do not own, and NotebookLM is not an approved connector. Section 3A also requires a privacy policy link.
- **Fit: likely no.** Packaging is feasible: an `npx sourcebolt@0.1.0 serve` launcher after npm publish. But 3F makes rejection the expected outcome. Only worth trying if the owner accepts a probable rejection. Ask Anthropic (`directory@anthropic.com`) first if in doubt.

### 4d. Claude Code: our own plugin marketplace (no Anthropic review)

- A marketplace is a git repo with `.claude-plugin/marketplace.json` (`name`, `owner`, `plugins[]` with `name` and `source`). Users run `claude plugin marketplace add <owner>/<repo>`, then `claude plugin install <plugin>@<marketplace>`. Names such as `claude-plugins-official` are reserved ([create a marketplace](https://code.claude.com/docs/en/plugin-marketplaces)).
- **Fit: yes, now.** We can add a plugin folder (`.claude-plugin/plugin.json`, `.mcp.json` running `sourcebolt serve`, plus the existing `SKILL.md` as a skill) and a `marketplace.json` to this repo. Validate with `claude plugin validate`. This is distribution, not a listing: users find it through our README.

## 5. OpenAI

- **ChatGPT plugin/app directory.** The submission page now covers "plugins" for ChatGPT and Codex. Remote MCP must be submitted "using a stable, public HTTPS endpoint". Other requirements:
  - website, support, privacy policy and terms URLs
  - auth: none, OAuth, or mixed (no API-key or custom-header auth)
  - domain verification at `/.well-known/openai-apps-challenge`
  - a reviewer test account without MFA
  - 5 positive and 3 negative test cases, and a demo video

  Local or desktop-execution plugins go through a separate "Skills only" path. The review timeline and fees are not stated ([submission](https://developers.openai.com/apps-sdk/deploy/submission)). Connecting from ChatGPT needs a public HTTPS endpoint or OpenAI's Secure MCP Tunnel, and submission needs a public endpoint ([connect from ChatGPT](https://developers.openai.com/apps-sdk/deploy/connect-chatgpt)).
- **Policy blocker.** The guidelines say plugins that mainly act as unofficial connectors to third-party services will not be approved. They also forbid bypassing API restrictions and require compliance with the third party's terms ([submission guidelines](https://developers.openai.com/apps-sdk/app-submission-guidelines)).
- **Codex.** Codex supports STDIO servers configured in `~/.codex/config.toml` or via `codex mcp add` ([Codex MCP](https://developers.openai.com/codex/mcp)). This is user configuration, not a listing.
- **Fit: ChatGPT directory no. Codex: README snippet only.**

## 6. xAI Grok

- **API remote MCP.** "Only Streaming HTTP and SSE transports are supported". You configure `server_url` and `server_label` per request. The docs mention no directory ([xAI remote MCP](https://docs.x.ai/developers/tools/remote-mcp)).
- **grok.com connectors.** A custom connector takes an MCP server URL that must be "reachable over the public internet", with a tunnel for local servers. The page describes no way for third parties to list in the connector catalog ([Grok connectors](https://docs.x.ai/grok/connectors)).
- **Fit: no.** There is no listing path and no stdio support. A user could tunnel `serve --http`, but that exposes a session-bearing server, and we should not recommend it.

## 7. Google: Gemini CLI extensions and Antigravity

- **Gemini CLI gallery.** Listing is automatic, with no submission. Requirements ([releasing extensions](https://geminicli.com/docs/extensions/releasing/)):
  - a public GitHub repo
  - the `gemini-cli-extension` topic
  - `gemini-extension.json` at the repo root or in the release archive
  - passing the daily crawler's validation

  Platform binaries go in release assets named `{platform}.{arch}.{name}.{ext}` (for example `win32.x64.sourcebolt.zip`). The manifest supports `mcpServers` with `command`/`args` and `${extensionPath}`, and `settings` with `sensitive` values ([reference](https://geminicli.com/docs/extensions/reference/)).
- **Caveat.** Gemini CLI's own pages say it "was replaced by Antigravity CLI on June 18th, 2026" for unpaid-tier and Google One users ([reference](https://geminicli.com/docs/extensions/reference/)). Antigravity can convert Gemini extensions with `agy plugin import gemini`, which moves MCP definitions into `mcp_config.json` ([migration guide](https://antigravity.google/docs/cli/gcli-migration/)). Antigravity can also install a plugin straight from GitHub: `agy plugin install https://github.com/<owner>/<repo>` ([marketplace](https://antigravity.google/docs/marketplace), [plugins](https://antigravity.google/docs/plugins)).
- **Antigravity Marketplace.** It is "curated". Listing goes through an interest form ([form](https://forms.gle/2EX5RFYPoJe1UgxR9), linked from the [marketplace page](https://antigravity.google/docs/marketplace)). The page describes no public criteria or timeline.
- **Fit: Gemini gallery yes. It is cheap, but our current release asset names do not match the gallery pattern, so we need extra assets or a manifest-only repo layout using an installed binary. Antigravity listing likely no**: it is a Google-curated marketplace, and we are an unofficial client of a Google product. Documenting `agy plugin install <repo>` is the practical path.

## 8. Smithery

- Two paths: hosted URL ("Bring your own hosting", Streamable HTTP, OAuth if needed), or local stdio, where "Smithery distributes a pre-built MCPB bundle". The CLI command is `smithery mcp publish ./server.mcpb -n <org>/<server>` ([Smithery publish](https://smithery.ai/docs/build/publish)). No review is described beyond an automatic scan and an optional verification checklist. No publishing price is stated.
- **Fit: yes-after-MCPB.** This reuses the Claude Desktop `.mcpb` work (section 4b).

## 9. Glama

- Glama has an "Add Server" button on its listing page, and claimed listings ([servers](https://glama.ai/mcp/servers)). You claim by adding `glama.json` (`$schema`, `maintainers`: GitHub usernames) to the repo root, or by GitHub login for repos on a personal account. Claiming unlocks editing, Docker image config and usage reports ([glama.json post, 2025-07-08](https://glama.ai/blog/2025-07-08-what-is-glamajson)). Glama does not document whether it indexes GitHub repos automatically. Remote connector claims use a different `/.well-known/glama.json` flow (secondary; pages conflict on the schema).
- **Fit: yes.** Check whether the repo is already indexed, then add `glama.json`. Our Dockerfile lets Glama run its inspection.

## 10. mcp.so

- A web form with type (MCP Server / Remote Server / ...), Repository URL and Name. There is an optional $39 one-time fee to "Publish immediately without review" with a badge and featured placement. The free review process is not described ([submit](https://mcp.so/submit)).
- **Fit: yes, free path.** Paying is a money decision for the owner and not needed.

## 11. PulseMCP

- "Not accepting new MCP server or client submissions right now". It recommends the Official MCP Registry and says it will pick entries up automatically when it is back. Last updated September 3, 2026 ([submit](https://www.pulsemcp.com/submit)).
- **Fit: automatic once we are in the official registry.**

## 12. awesome-mcp-servers (punkpeye)

- The list is for servers you install and run yourself, with "a public GitHub repository". Remote-only servers go to a separate list. One server per line, in the right category, matching the existing format, submitted as a PR ([CONTRIBUTING](https://github.com/punkpeye/awesome-mcp-servers/blob/main/CONTRIBUTING.md)).
- **Fit: yes.** The PR needs owner approval, because our working rule is not to open PRs on third-party repos.

## 13. Cline MCP Marketplace

- Submit by opening an issue in `cline/mcp-marketplace` with the repo URL, a 400x400 PNG logo and the reason for adding it. Test that Cline can install the server from the README alone (`llms-install.md` is optional). Review weighs adoption, credibility, maturity and security, with extra scrutiny for sensitive domains. Target review time is "within a couple of days" ([repo README](https://github.com/cline/mcp-marketplace)).
- **Fit: yes.** We need a logo and a Cline-tested README (`llms-install.md` recommended, because our auth setup is non-trivial). Opening the issue is an owner action.

## 14. Docker MCP Catalog

- The catalog takes PRs to `docker/mcp-registry` with a `server.yaml`. Local servers are containerized: `type: server`, `source` project and commit, and `config` with `secrets`/`env`/`parameters`. Remote servers use `type: remote`. Docker can build and sign the image (`mcp/<name>` on Docker Hub), or we can provide our own. The license must allow public use ("MIT or Apache 2 are great, GPL is not"). Every PR is reviewed by Docker, and an approved entry is live "in 24 hours" ([README](https://github.com/docker/mcp-registry), [CONTRIBUTING](https://github.com/docker/mcp-registry/blob/main/CONTRIBUTING.md)).
- **Fit: yes-after-work.** Our image keeps the session in a `/config` volume, and login inside a container is `login --import -`. The Docker MCP Toolkit wires env and secrets, so we must check that a cookie import can be fed non-interactively, for example from a secret, before submitting. The browser fallback image is large and probably out of scope for the catalog.

## 15. Windsurf

- We found no primary Windsurf source for submitting to its built-in MCP marketplace. Secondary guides describe the marketplace UI and the manual config at `~/.codeium/windsurf/mcp_config.json` ([secondary](https://fast.io/resources/windsurf-mcp-setup-guide/)). **Gap**: check docs.windsurf.com again later.
- **Fit: README config snippet only for now.**

## Recommended order

1. **Official MCP Registry**: OCI-only entry now, with npm and PyPI added once published. This unlocks GitHub/VS Code and later PulseMCP.
2. **README install surface**:
   - VS Code `vscode:mcp/install` link and `code --add-mcp` line
   - Cursor deeplink
   - Codex `config.toml` snippet
   - Windsurf snippet
   - Antigravity `agy plugin install` line

   All of these point at the installed binary or the GHCR image until npm is live.
3. **Claude Code plugin marketplace in our repo**: `marketplace.json`, plus a plugin wrapping `sourcebolt serve` and the existing skill.
4. **Free community directories**: Glama claim (`glama.json`), mcp.so free form, awesome-mcp-servers PR, Cline issue. Each external post is an owner action.
5. **Gemini CLI extension**: `gemini-extension.json` and repo topic. Low cost, but shrinking reach.
6. **MCPB bundles** in the release workflow, then Smithery publish and the MCPB package type in `server.json`.
7. **Docker MCP Catalog** PR, after a non-interactive container login path is confirmed.
8. **Do not pursue** unless the strategy changes: the Anthropic connector directory, ChatGPT, Grok and the Antigravity Marketplace (remote-only or own-the-API rules). The Anthropic plugin directory and Cursor Marketplace are low-odds and only worth it with owner sign-off.

## Prerequisites checklist

- [ ] npm packages published (owner `NPM_TOKEN`), and PyPI trusted publisher set up. These are needed before `server.json` can list npm/PyPI and before any `npx`-based one-click link.
- [ ] Decide the registry option: OCI-only `server.json` now, or wait for npm/PyPI.
- [ ] Privacy policy at an HTTPS URL (for example a `PRIVACY.md` rendered on GitHub, or a docs page). It must cover data collection, storage, third-party sharing (Google), retention and contact. Needed by MCPB (`privacy_policies`), the Anthropic plugin directory, ChatGPT, and as good practice everywhere. Must not include personal emails; use a project contact.
- [ ] Icon/logo: 512x512 PNG with transparency (MCPB), 400x400 PNG (Cline). One square master works for both.
- [ ] Tool annotations: every tool has `Title` plus `ReadOnlyHint` or `DestructiveHint`. Partly in place (`internal/mcpserver`); audit all tools.
- [ ] README install section with VS Code, Cursor, Claude Code, Codex, Gemini/Antigravity and Windsurf snippets, and an `llms-install.md` for agent-driven installs.
- [ ] `glama.json` with `maintainers: ["candidloom-hub"]`.
- [ ] `.claude-plugin/marketplace.json` and a plugin folder; run `claude plugin validate`.
- [ ] `gemini-extension.json`, the `gemini-cli-extension` repo topic, and release assets named `{platform}.{arch}.sourcebolt.{zip|tar.gz}` if the extension should download the binary.
- [ ] MCPB: `manifest.json` (`manifest_version` 0.3, `server.type: binary`, `platform_overrides`, `privacy_policies`, icon), `mcpb pack` in CI, per-platform `.mcpb` assets on GitHub releases with "mcp" in the filename or repo name (the repo name `sourcebolt` lacks "mcp", so name the asset for example `sourcebolt-mcp-<os>-<arch>.mcpb`), and `fileSha256` in `server.json`.
- [ ] Docker catalog: confirm `login --import` works from a secret/env without a TTY; draft `server.yaml`.
- [ ] Owner approvals needed before any external action: registry publish, form submissions (mcp.so, Cline issue, Antigravity form), PRs to awesome-mcp-servers and docker/mcp-registry, any paid option (mcp.so fast-track), and any Anthropic or Cursor directory submission.
